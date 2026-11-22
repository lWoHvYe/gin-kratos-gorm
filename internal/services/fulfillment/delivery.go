package fulfillment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/outbox"

	"github.com/go-sql-driver/mysql"
)

const (
	trackCreated        = "CREATED"
	trackShipped        = "SHIPPED"
	trackInTransit      = "IN_TRANSIT"
	trackOutForDelivery = "OUT_FOR_DELIVERY"
	trackDelivered      = "DELIVERED"

	exceptionNone = "NONE"
	exceptionOpen = "OPEN"

	sourceCarrierDelivered = "CARRIER_DELIVERED"
	sourceUserConfirm      = "USER_CONFIRM"

	receiptUser             = "USER"
	receiptAutoT1           = "AUTO_T1"
	receiptAutoT7           = "AUTO_T7"
	receiptDisputeResolved  = "DISPUTE_RESOLVED"
	autoConfirmT1           = "T1_DELIVERED"
	autoConfirmT7           = "T7_NO_TRACK"
	autoConfirmPending      = "PENDING"
	autoConfirmDone         = "DONE"
	autoConfirmCancelled    = "CANCELLED"
	disputeOpen             = "OPEN"
	disputeUpheld           = "RESOLVED_UPHELD"
	disputeRejected         = "RESOLVED_REJECTED"
	autoConfirmLease        = time.Minute
	autoConfirmInterval     = 15 * time.Second
	disputeWindow           = 24 * time.Hour
	postReceiptDispute      = 15 * 24 * time.Hour
	appealGrace             = 60 * time.Second
	disputeBusinessDeadline = 3
)

const (
	topicShipmentTracked         = "shop.shipment.tracked.v1"
	topicShipmentException       = "shop.shipment.exception.v1"
	topicShipmentReceived        = "shop.shipment.received.v1"
	topicShipmentDisputeOpened   = "shop.shipment.dispute.opened.v1"
	topicShipmentDisputeResolved = "shop.shipment.dispute.resolved.v1"
)

var trackRank = map[string]int{
	trackCreated: 0, trackShipped: 1, trackInTransit: 2, trackOutForDelivery: 3, trackDelivered: 4,
}

func trackStatusFor(shipmentStatus string) string {
	switch shipmentStatus {
	case statusShipped:
		return trackShipped
	default:
		return trackCreated
	}
}

func isDuplicateTracking(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062 && strings.Contains(me.Message, "uk_shipments_tracking")
}

func (s *Service) validateCarrier(ctx context.Context, code string) (string, error) {
	if code == "" {
		return "", apperr.Failed("carrier_code is required for shipping").
			WithDetails(map[string]any{"available_carriers": s.enabledCarrierCodes(ctx)})
	}
	var name string
	var enabled bool
	err := s.db.QueryRowContext(ctx, `SELECT name, enabled FROM carriers WHERE code = ?`, code).Scan(&name, &enabled)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !enabled) {
		return "", apperr.Failed("carrier is not registered or enabled").
			WithDetails(map[string]any{"available_carriers": s.enabledCarrierCodes(ctx), "carrier_code": code})
	}
	if err != nil {
		return "", apperr.Internal("load carrier", err)
	}
	return name, nil
}

func (s *Service) enabledCarrierCodes(ctx context.Context) []string {
	rows, err := s.db.QueryContext(ctx, `SELECT code FROM carriers WHERE enabled = 1 ORDER BY sort_order, code`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	codes := make([]string, 0, 4)
	for rows.Next() {
		var code string
		if rows.Scan(&code) == nil {
			codes = append(codes, code)
		}
	}
	return codes
}

func (s *Service) upsertAutoConfirmJobTx(ctx context.Context, tx *sql.Tx, shipmentNo, reason string, dueAt time.Time) error {
	var (
		state     string
		oldReason string
		oldDue    time.Time
	)
	err := tx.QueryRowContext(ctx, `SELECT state, reason, due_at FROM delivery_auto_confirm_jobs
		WHERE shipment_no = ? FOR UPDATE`, shipmentNo).Scan(&state, &oldReason, &oldDue)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `INSERT INTO delivery_auto_confirm_jobs (shipment_no, reason, due_at, state)
			VALUES (?, ?, ?, ?)`, shipmentNo, reason, dueAt, autoConfirmPending)
		if err != nil {
			return apperr.Internal("insert auto confirm job", err)
		}
		return nil
	}
	if err != nil {
		return apperr.Internal("load auto confirm job", err)
	}
	// DONE/CANCELLED jobs are terminal for this shipment; a replayed DELIVERED
	// must not resurrect auto confirm. A same-reason replay may only move the
	// deadline later; an older occurred_at must never shrink the 24h window.
	if state != autoConfirmPending {
		return nil
	}
	nextDue := dueAt
	if oldReason == reason && oldDue.After(dueAt) {
		nextDue = oldDue
	}
	if _, err := tx.ExecContext(ctx, `UPDATE delivery_auto_confirm_jobs
		SET reason = ?, due_at = ?, lease_owner = '', lease_until = NULL, updated_at = NOW(3)
		WHERE shipment_no = ? AND state = ?`, reason, nextDue, shipmentNo, autoConfirmPending); err != nil {
		return apperr.Internal("update auto confirm job", err)
	}
	return nil
}

func (s *Service) cancelAutoConfirmJobsTx(ctx context.Context, tx *sql.Tx, shipmentNo string) error {
	_, err := tx.ExecContext(ctx, `UPDATE delivery_auto_confirm_jobs
		SET state = ?, version = version + 1, updated_at = NOW(3)
		WHERE shipment_no = ? AND state = ?`, autoConfirmCancelled, shipmentNo, autoConfirmPending)
	if err != nil {
		return apperr.Internal("cancel auto confirm job", err)
	}
	return nil
}

type trackingEventPayload struct {
	ShipmentNo    string `json:"shipment_no"`
	OrderNo       string `json:"order_no"`
	GroupNo       string `json:"fulfillment_group_no,omitempty"`
	CarrierCode   string `json:"carrier_code,omitempty"`
	TrackingNo    string `json:"tracking_no,omitempty"`
	Status        string `json:"status,omitempty"`
	ExceptionType string `json:"exception_type,omitempty"`
	EventID       string `json:"event_id,omitempty"`
	OccurredAt    int64  `json:"occurred_at,omitempty"`
	Source        string `json:"source,omitempty"`
}

// RecordTrackingEvent is the only carrier tracking write path. Events are
// append-only and the projection only moves forward by rank (INV-25).
func (s *Service) RecordTrackingEvent(ctx context.Context, req *pb.RecordTrackingEventRequest) (*pb.Shipment, error) {
	shipmentNo := strings.TrimSpace(req.GetShipmentNo())
	eventID := strings.TrimSpace(req.GetEventId())
	status := strings.ToUpper(strings.TrimSpace(req.GetStatus()))
	exceptionType := strings.ToUpper(strings.TrimSpace(req.GetExceptionType()))
	if shipmentNo == "" || eventID == "" {
		return nil, apperr.Invalid("shipment_no and event_id are required")
	}
	if status == "" && exceptionType == "" {
		return nil, apperr.Invalid("a tracking event needs a status or exception_type")
	}
	if status != "" {
		if _, ok := trackRank[status]; !ok {
			return nil, apperr.Invalid("unknown tracking status %q", status)
		}
	}
	if exceptionType != "" && exceptionType != "REJECTED" && exceptionType != "RETURNED" && exceptionType != "LOST" && exceptionType != "DAMAGED" {
		return nil, apperr.Invalid("unknown exception_type %q", exceptionType)
	}
	occurredAt := s.now()
	if unix := req.GetOccurredAtUnix(); unix > 0 {
		occurredAt = time.Unix(unix, 0)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin tracking event", err)
	}
	defer func() { _ = tx.Rollback() }()
	var (
		orderNo, groupNo, carrierCode, trackingNo, currentTrack, currentException string
		deliveredAt                                                               sql.NullTime
	)
	err = tx.QueryRowContext(ctx, `SELECT order_no, fulfillment_group_no, carrier_code, tracking_no,
		track_status, exception_state, delivered_at FROM shipments WHERE shipment_no = ? FOR UPDATE`, shipmentNo).
		Scan(&orderNo, &groupNo, &carrierCode, &trackingNo, &currentTrack, &currentException, &deliveredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("shipment not found")
	}
	if err != nil {
		return nil, apperr.Internal("lock shipment for tracking", err)
	}
	if carrierCode == "" {
		carrierCode = strings.TrimSpace(req.GetCarrierCode())
	}
	if req.GetCarrierCode() != "" && carrierCode != strings.TrimSpace(req.GetCarrierCode()) {
		return nil, apperr.Failed("tracking carrier does not match the shipment")
	}
	if req.GetTrackingNo() != "" && trackingNo != strings.TrimSpace(req.GetTrackingNo()) {
		return nil, apperr.Failed("tracking number does not match the shipment")
	}
	raw := sql.NullString{String: req.GetRawJson(), Valid: strings.TrimSpace(req.GetRawJson()) != ""}
	res, err := tx.ExecContext(ctx, `INSERT IGNORE INTO shipment_track_events
		(shipment_no, carrier_code, tracking_no, event_id, status, exception_type, occurred_at, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		shipmentNo, carrierCode, trackingNo, eventID, status, exceptionType, occurredAt, raw)
	if err != nil {
		return nil, apperr.Internal("append tracking event", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, apperr.Internal("tracking event rows", err)
	}
	if affected == 0 {
		if err := tx.Commit(); err != nil {
			return nil, apperr.Internal("commit duplicate tracking event", err)
		}
		return s.loadShipment(ctx, shipmentNo)
	}
	changed := false
	if exceptionType != "" {
		if currentException != exceptionOpen {
			if _, err := tx.ExecContext(ctx, `UPDATE shipments SET exception_state = ?, version = version + 1 WHERE shipment_no = ?`,
				exceptionOpen, shipmentNo); err != nil {
				return nil, apperr.Internal("open shipment exception", err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO shipment_operation_cases
			(shipment_no, exception_type, status) VALUES (?, ?, 'OPEN')
			ON DUPLICATE KEY UPDATE status = 'OPEN', resolved_at = NULL`,
			shipmentNo, exceptionType); err != nil {
			return nil, apperr.Internal("open shipment operation case", err)
		}
		payload, _ := json.Marshal(trackingEventPayload{
			ShipmentNo: shipmentNo, OrderNo: orderNo, GroupNo: groupNo, CarrierCode: carrierCode,
			TrackingNo: trackingNo, ExceptionType: exceptionType, EventID: eventID, OccurredAt: occurredAt.Unix(),
		})
		if err := outbox.Append(ctx, tx, outbox.Event{
			Topic: topicShipmentException, AggregateType: aggregateShipment, AggregateID: shipmentNo,
			EventType: "shipment.exception", Payload: payload,
		}); err != nil {
			return nil, apperr.Internal("append shipment exception", err)
		}
	}
	if status != "" {
		currentRank := trackRank[currentTrack]
		if currentTrack == "" {
			currentRank = -1
		}
		if currentTrack == trackDelivered {
			// DELIVERED is terminal for non-dispute evidence (INV-25).
			changed = false
		} else if trackRank[status] > currentRank {
			if _, err := tx.ExecContext(ctx, `UPDATE shipments SET track_status = ?, version = version + 1 WHERE shipment_no = ?`,
				status, shipmentNo); err != nil {
				return nil, apperr.Internal("advance tracking status", err)
			}
			changed = true
			currentTrack = status
		}
		if status == trackDelivered && currentTrack == trackDelivered {
			if _, err := tx.ExecContext(ctx, `UPDATE shipments
				SET delivered_at = COALESCE(delivered_at, ?), delivery_source = ?, version = version + 1
				WHERE shipment_no = ?`, occurredAt, sourceCarrierDelivered, shipmentNo); err != nil {
				return nil, apperr.Internal("record delivery evidence time", err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO delivery_evidence
				(shipment_no, source, evidence_ref, occurred_at) VALUES (?, ?, ?, ?)`,
				shipmentNo, sourceCarrierDelivered, eventID, occurredAt); err != nil {
				return nil, apperr.Internal("record carrier delivery evidence", err)
			}
			if err := s.upsertAutoConfirmJobTx(ctx, tx, shipmentNo, autoConfirmT1, occurredAt.Add(24*time.Hour)); err != nil {
				return nil, err
			}
		}
	}
	if changed {
		payload, _ := json.Marshal(trackingEventPayload{
			ShipmentNo: shipmentNo, OrderNo: orderNo, GroupNo: groupNo, CarrierCode: carrierCode,
			TrackingNo: trackingNo, Status: currentTrack, EventID: eventID, OccurredAt: occurredAt.Unix(),
		})
		if err := outbox.Append(ctx, tx, outbox.Event{
			Topic: topicShipmentTracked, AggregateType: aggregateShipment, AggregateID: shipmentNo,
			EventType: "shipment.tracked", Payload: payload,
		}); err != nil {
			return nil, apperr.Internal("append shipment tracked", err)
		}
	}
	if status == trackDelivered {
		payload, _ := json.Marshal(trackingEventPayload{
			ShipmentNo: shipmentNo, OrderNo: orderNo, GroupNo: groupNo, CarrierCode: carrierCode,
			TrackingNo: trackingNo, Status: trackDelivered, Source: sourceCarrierDelivered,
			OccurredAt: occurredAt.Unix(),
		})
		if err := outbox.Append(ctx, tx, outbox.Event{
			Topic: topicShipmentDelivered, AggregateType: aggregateShipment, AggregateID: shipmentNo,
			EventType: "shipment.delivered", Payload: payload,
		}); err != nil {
			return nil, apperr.Internal("append shipment delivered", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit tracking event", err)
	}
	return s.loadShipment(ctx, shipmentNo)
}

// ConfirmReceipt is the user-confirmation path. It writes evidence and a
// receipt in one transaction; a carrier DELIVERED event alone never creates a
// receipt (T+1 does that later).
func (s *Service) ConfirmReceipt(ctx context.Context, req *pb.ConfirmReceiptRequest) (*pb.Shipment, error) {
	shipmentNo := strings.TrimSpace(req.GetShipmentNo())
	if shipmentNo == "" || req.GetUserId() <= 0 {
		return nil, apperr.Invalid("shipment_no and user_id are required")
	}
	if s.orders == nil {
		return nil, apperr.Unavailable("order client is not configured", nil)
	}
	order, err := s.orderForShipment(ctx, shipmentNo)
	if err != nil {
		return nil, err
	}
	if order.GetUserId() != req.GetUserId() {
		return nil, apperr.Forbidden("shipment belongs to another user")
	}
	if _, err := s.confirmReceiptTx(ctx, shipmentNo, receiptUser, fmt.Sprintf("user:%d", req.GetUserId())); err != nil {
		return nil, err
	}
	return s.loadShipment(ctx, shipmentNo)
}

func (s *Service) orderForShipment(ctx context.Context, shipmentNo string) (*pb.Order, error) {
	var orderNo string
	err := s.db.QueryRowContext(ctx, `SELECT order_no FROM shipments WHERE shipment_no = ?`, shipmentNo).Scan(&orderNo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("shipment not found")
	}
	if err != nil {
		return nil, apperr.Internal("load shipment order", err)
	}
	order, err := s.orders.GetOrder(ctx, &pb.GetOrderRequest{OrderNo: orderNo, Admin: true})
	if err != nil {
		return nil, apperr.Unavailable("load order for receipt", err)
	}
	return order, nil
}

func (s *Service) confirmReceiptTx(ctx context.Context, shipmentNo, source, evidenceRef string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, apperr.Internal("begin receipt", err)
	}
	defer func() { _ = tx.Rollback() }()
	var orderNo, status string
	err = tx.QueryRowContext(ctx, `SELECT order_no, status FROM shipments WHERE shipment_no = ? FOR UPDATE`, shipmentNo).
		Scan(&orderNo, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, apperr.NotFound("shipment not found")
	}
	if err != nil {
		return false, apperr.Internal("lock shipment for receipt", err)
	}
	if status == statusCancelled {
		return false, apperr.Failed("shipment was cancelled")
	}
	var openDisputes int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM delivery_disputes
		WHERE shipment_no = ? AND status = ?`, shipmentNo, disputeOpen).Scan(&openDisputes); err != nil {
		return false, apperr.Internal("check open disputes", err)
	}
	if openDisputes > 0 {
		return false, apperr.Failed("shipment has an open receipt dispute").
			WithDetails(map[string]any{"reason": "open_dispute"})
	}
	res, err := tx.ExecContext(ctx, `INSERT IGNORE INTO shipment_receipts
		(shipment_no, source, evidence_ref, confirmed_at) VALUES (?, ?, ?, NOW(3))`,
		shipmentNo, source, evidenceRef)
	if err != nil {
		return false, apperr.Internal("record receipt", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		if err := tx.Commit(); err != nil {
			return false, apperr.Internal("commit idempotent receipt", err)
		}
		return false, nil
	}
	if source == receiptUser || source == receiptDisputeResolved {
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO delivery_evidence
			(shipment_no, source, evidence_ref, occurred_at) VALUES (?, ?, ?, NOW(3))`,
			shipmentNo, sourceUserConfirm, evidenceRef); err != nil {
			return false, apperr.Internal("record user delivery evidence", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shipments
		SET status = ?, track_status = ?, delivered_at = COALESCE(delivered_at, NOW(3)),
		    delivery_source = IF(delivery_source = '', ?, delivery_source), version = version + 1
		WHERE shipment_no = ?`,
		statusReceived, trackDelivered, sourceUserConfirm, shipmentNo); err != nil {
		return false, apperr.Internal("mark shipment received", err)
	}
	if err := s.cancelAutoConfirmJobsTx(ctx, tx, shipmentNo); err != nil {
		return false, err
	}
	payload, _ := json.Marshal(trackingEventPayload{
		ShipmentNo: shipmentNo, OrderNo: orderNo, Status: statusReceived, Source: source,
	})
	if err := outbox.Append(ctx, tx, outbox.Event{
		Topic: topicShipmentReceived, AggregateType: aggregateShipment, AggregateID: shipmentNo,
		EventType: "shipment.received", Payload: payload,
	}); err != nil {
		return false, apperr.Internal("append shipment received", err)
	}
	if err := tx.Commit(); err != nil {
		return false, apperr.Internal("commit receipt", err)
	}
	return true, nil
}

// RunDeliveryAutoConfirmWorker is the durable T+1/T+7 scheduler. It uses the job
// table and a lease, never a business-status scan (ADR 0005 A-09).
func RunDeliveryAutoConfirmWorker(ctx context.Context, svc *Service, logger *slog.Logger, interval time.Duration) {
	if svc == nil || !svc.autoConfirmEnabled {
		return
	}
	if interval <= 0 {
		interval = autoConfirmInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := svc.runAutoConfirmBatch(ctx, logger); err != nil {
				logger.Error("delivery auto confirm batch", "error", err)
			}
		}
	}
}

func (s *Service) runAutoConfirmBatch(ctx context.Context, logger *slog.Logger) error {
	rows, err := s.db.QueryContext(ctx, `SELECT shipment_no FROM delivery_auto_confirm_jobs
		WHERE state = ? AND due_at <= NOW(3) AND (lease_until IS NULL OR lease_until < NOW(3))
		ORDER BY due_at LIMIT 50`, autoConfirmPending)
	if err != nil {
		return err
	}
	var shipmentNos []string
	for rows.Next() {
		var shipmentNo string
		if err := rows.Scan(&shipmentNo); err != nil {
			rows.Close()
			return err
		}
		shipmentNos = append(shipmentNos, shipmentNo)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, shipmentNo := range shipmentNos {
		if err := s.processAutoConfirmJob(ctx, shipmentNo); err != nil && logger != nil {
			logger.Warn("auto confirm job deferred", "shipment_no", shipmentNo, "error", err)
		}
	}
	return nil
}

func (s *Service) processAutoConfirmJob(ctx context.Context, shipmentNo string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var reason, state string
	var dueAt time.Time
	var attempts int
	err = tx.QueryRowContext(ctx, `SELECT reason, state, due_at, attempts FROM delivery_auto_confirm_jobs
		WHERE shipment_no = ? FOR UPDATE`, shipmentNo).Scan(&reason, &state, &dueAt, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if state != autoConfirmPending || dueAt.Add(appealGrace).After(s.now()) {
		return nil
	}
	owner := fmt.Sprintf("worker-%d", s.now().UnixNano())
	if _, err := tx.ExecContext(ctx, `UPDATE delivery_auto_confirm_jobs
		SET lease_owner = ?, lease_until = DATE_ADD(NOW(3), INTERVAL 1 MINUTE), attempts = attempts + 1,
		    version = version + 1, updated_at = NOW(3)
		WHERE shipment_no = ? AND state = ?`, owner, shipmentNo, autoConfirmPending); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := s.evaluateAutoConfirmJob(ctx, shipmentNo, reason); err != nil {
		_, _ = s.db.ExecContext(ctx, `UPDATE delivery_auto_confirm_jobs
			SET lease_owner = '', lease_until = NULL, last_error = ?, updated_at = NOW(3)
			WHERE shipment_no = ? AND state = ?`, truncateFulfillment(err.Error(), 1000), shipmentNo, autoConfirmPending)
		return err
	}
	return nil
}

func (s *Service) evaluateAutoConfirmJob(ctx context.Context, shipmentNo, reason string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var (
		orderNo, status, trackStatus string
		deliveredAt                  sql.NullTime
	)
	err = tx.QueryRowContext(ctx, `SELECT order_no, status, track_status, delivered_at
		FROM shipments WHERE shipment_no = ? FOR UPDATE`, shipmentNo).
		Scan(&orderNo, &status, &trackStatus, &deliveredAt)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = tx.ExecContext(ctx, `UPDATE delivery_auto_confirm_jobs SET state = ?, updated_at = NOW(3) WHERE shipment_no = ?`,
			autoConfirmCancelled, shipmentNo)
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	var receipt int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM shipment_receipts WHERE shipment_no = ?`, shipmentNo).Scan(&receipt); err != nil {
		return err
	}
	if receipt > 0 {
		_, err = tx.ExecContext(ctx, `UPDATE delivery_auto_confirm_jobs SET state = ?, updated_at = NOW(3) WHERE shipment_no = ?`,
			autoConfirmCancelled, shipmentNo)
		return err
	}
	var openDisputes int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM delivery_disputes WHERE shipment_no = ? AND status = ?`,
		shipmentNo, disputeOpen).Scan(&openDisputes); err != nil {
		return err
	}
	if openDisputes > 0 {
		_, err = tx.ExecContext(ctx, `UPDATE delivery_auto_confirm_jobs SET state = ?, updated_at = NOW(3) WHERE shipment_no = ?`,
			autoConfirmCancelled, shipmentNo)
		return err
	}
	confirm := false
	switch reason {
	case autoConfirmT1:
		if deliveredAt.Valid && s.now().Before(deliveredAt.Time.Add(24*time.Hour).Add(appealGrace)) {
			_, err = tx.ExecContext(ctx, `UPDATE delivery_auto_confirm_jobs
				SET state = ?, lease_owner = '', lease_until = NULL, updated_at = NOW(3)
				WHERE shipment_no = ?`, autoConfirmPending, shipmentNo)
			if err != nil {
				return err
			}
			return tx.Commit()
		}
		confirm = deliveredAt.Valid
	case autoConfirmT7:
		if trackStatus == trackCreated || trackStatus == trackShipped {
			confirm = true
		} else if trackStatus == trackInTransit || trackStatus == trackOutForDelivery {
			_, err = tx.ExecContext(ctx, `UPDATE delivery_auto_confirm_jobs SET state = ?, updated_at = NOW(3) WHERE shipment_no = ?`,
				autoConfirmCancelled, shipmentNo)
			return err
		}
	}
	if !confirm || status != statusShipped {
		_, err = tx.ExecContext(ctx, `UPDATE delivery_auto_confirm_jobs SET state = ?, updated_at = NOW(3) WHERE shipment_no = ?`,
			autoConfirmCancelled, shipmentNo)
		return err
	}
	source := receiptAutoT1
	if reason == autoConfirmT7 {
		source = receiptAutoT7
	}
	_, err = tx.ExecContext(ctx, `INSERT IGNORE INTO shipment_receipts
		(shipment_no, source, evidence_ref, confirmed_at) VALUES (?, ?, ?, NOW(3))`,
		shipmentNo, source, reason)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shipments
		SET status = ?, version = version + 1
		WHERE shipment_no = ? AND status = ?`, statusReceived, shipmentNo, statusShipped); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE delivery_auto_confirm_jobs SET state = ?, lease_owner = '', lease_until = NULL,
		version = version + 1, updated_at = NOW(3) WHERE shipment_no = ?`, autoConfirmDone, shipmentNo)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(trackingEventPayload{
		ShipmentNo: shipmentNo, OrderNo: orderNo, Status: statusReceived, Source: source,
	})
	if err := outbox.Append(ctx, tx, outbox.Event{
		Topic: topicShipmentReceived, AggregateType: aggregateShipment, AggregateID: shipmentNo,
		EventType: "shipment.received", Payload: payload,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// OpenReceiptDispute freezes auto confirmation. The window is 24h after the
// carrier DELIVERED evidence; a confirmed receipt closes the entry.
func (s *Service) OpenReceiptDispute(ctx context.Context, req *pb.OpenReceiptDisputeRequest) (*pb.ReceiptDispute, error) {
	shipmentNo := strings.TrimSpace(req.GetShipmentNo())
	if shipmentNo == "" || req.GetUserId() <= 0 {
		return nil, apperr.Invalid("shipment_no and user_id are required")
	}
	order, err := s.orderForShipment(ctx, shipmentNo)
	if err != nil {
		return nil, err
	}
	if order.GetUserId() != req.GetUserId() {
		return nil, apperr.Forbidden("shipment belongs to another user")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin receipt dispute", err)
	}
	defer func() { _ = tx.Rollback() }()
	var orderNo string
	var deliveredAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT order_no, delivered_at FROM shipments WHERE shipment_no = ? FOR UPDATE`, shipmentNo).
		Scan(&orderNo, &deliveredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("shipment not found")
	}
	if err != nil {
		return nil, apperr.Internal("lock shipment for dispute", err)
	}
	var confirmedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT confirmed_at FROM shipment_receipts WHERE shipment_no = ?`, shipmentNo).Scan(&confirmedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.Internal("check receipt", err)
	}
	if !deliveredAt.Valid && !confirmedAt.Valid {
		return nil, apperr.Failed("shipment has no delivery evidence")
	}
	deadline := deliveredAt.Time.Add(disputeWindow)
	if confirmedAt.Valid {
		// Post-receipt disputes are allowed for 15 days but never roll back the
		// receipt fact (P-07/AC-36).
		deadline = confirmedAt.Time.Add(postReceiptDispute)
	}
	if s.now().After(deadline) {
		return nil, apperr.Failed("the dispute window has passed").
			WithDetails(map[string]any{"dispute_deadline": deadline.Unix()})
	}
	var existingID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM delivery_disputes WHERE shipment_no = ? AND status = ? FOR UPDATE`,
		shipmentNo, disputeOpen).Scan(&existingID)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return nil, apperr.Internal("commit existing dispute", err)
		}
		return s.loadDispute(ctx, existingID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.Internal("load open dispute", err)
	}
	businessDeadline := addBusinessDays(s.now(), disputeBusinessDeadline)
	res, err := tx.ExecContext(ctx, `INSERT INTO delivery_disputes
		(shipment_no, order_no, user_id, status, reason, deadline_at) VALUES (?, ?, ?, ?, ?, ?)`,
		shipmentNo, orderNo, req.GetUserId(), disputeOpen, truncateFulfillment(req.GetReason(), 512), businessDeadline)
	if err != nil {
		return nil, apperr.Internal("open receipt dispute", err)
	}
	disputeID, _ := res.LastInsertId()
	if err := s.cancelAutoConfirmJobsTx(ctx, tx, shipmentNo); err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(trackingEventPayload{
		ShipmentNo: shipmentNo, OrderNo: orderNo, Source: sourceUserConfirm,
		OccurredAt: s.now().Unix(),
	})
	if err := outbox.Append(ctx, tx, outbox.Event{
		Topic: topicShipmentDisputeOpened, AggregateType: aggregateShipment, AggregateID: shipmentNo,
		EventType: "shipment.dispute_opened", Payload: payload,
	}); err != nil {
		return nil, apperr.Internal("append dispute opened", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit dispute", err)
	}
	return s.loadDispute(ctx, disputeID)
}

func (s *Service) ResolveReceiptDispute(ctx context.Context, req *pb.ResolveReceiptDisputeRequest) (*pb.ReceiptDispute, error) {
	resolution := strings.ToUpper(strings.TrimSpace(req.GetResolution()))
	if resolution != disputeUpheld && resolution != disputeRejected {
		return nil, apperr.Invalid("resolution must be RESOLVED_UPHELD or RESOLVED_REJECTED")
	}
	if strings.TrimSpace(req.GetReason()) == "" || strings.TrimSpace(req.GetOperator()) == "" {
		return nil, apperr.Invalid("resolution reason and operator are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin dispute resolution", err)
	}
	defer func() { _ = tx.Rollback() }()
	var shipmentNo, orderNo, status string
	err = tx.QueryRowContext(ctx, `SELECT shipment_no, order_no, status FROM delivery_disputes WHERE id = ? FOR UPDATE`,
		req.GetDisputeId()).Scan(&shipmentNo, &orderNo, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("dispute not found")
	}
	if err != nil {
		return nil, apperr.Internal("lock dispute", err)
	}
	if status != disputeOpen {
		return nil, apperr.Failed("dispute is already resolved")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE delivery_disputes
		SET status = ?, resolution_reason = ?, resolved_by = ?, resolved_at = NOW(3), version = version + 1
		WHERE id = ?`, resolution, truncateFulfillment(req.GetReason(), 512),
		truncateFulfillment(req.GetOperator(), 64), req.GetDisputeId()); err != nil {
		return nil, apperr.Internal("resolve dispute", err)
	}
	if resolution == disputeRejected {
		// A rejected dispute confirms the receipt; it does not create a new
		// delivery source.
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO shipment_receipts
			(shipment_no, source, evidence_ref, confirmed_at) VALUES (?, ?, ?, NOW(3))`,
			shipmentNo, receiptDisputeResolved, fmt.Sprintf("dispute:%d", req.GetDisputeId())); err != nil {
			return nil, apperr.Internal("record dispute receipt", err)
		}
		payload, _ := json.Marshal(trackingEventPayload{
			ShipmentNo: shipmentNo, OrderNo: orderNo, Status: statusReceived, Source: receiptDisputeResolved,
		})
		if err := outbox.Append(ctx, tx, outbox.Event{
			Topic: topicShipmentReceived, AggregateType: aggregateShipment, AggregateID: shipmentNo,
			EventType: "shipment.received", Payload: payload,
		}); err != nil {
			return nil, apperr.Internal("append dispute receipt", err)
		}
	}
	payload, _ := json.Marshal(trackingEventPayload{
		ShipmentNo: shipmentNo, OrderNo: orderNo, Source: resolution,
		OccurredAt: s.now().Unix(),
	})
	if err := outbox.Append(ctx, tx, outbox.Event{
		Topic: topicShipmentDisputeResolved, AggregateType: aggregateShipment, AggregateID: shipmentNo,
		EventType: "shipment.dispute_resolved", Payload: payload,
	}); err != nil {
		return nil, apperr.Internal("append dispute resolved", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit dispute resolution", err)
	}
	return s.loadDispute(ctx, req.GetDisputeId())
}

func (s *Service) loadDispute(ctx context.Context, id int64) (*pb.ReceiptDispute, error) {
	var out pb.ReceiptDispute
	var deadline, opened time.Time
	var resolved sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT id, shipment_no, order_no, user_id, status, reason,
		resolution_reason, resolved_by, deadline_at, opened_at, resolved_at FROM delivery_disputes WHERE id = ?`, id).
		Scan(&out.Id, &out.ShipmentNo, &out.OrderNo, &out.UserId, &out.Status, &out.Reason,
			&out.ResolutionReason, &out.ResolvedBy, &deadline, &opened, &resolved)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("dispute not found")
	}
	if err != nil {
		return nil, apperr.Internal("load dispute", err)
	}
	out.DeadlineAtUnix = deadline.Unix()
	out.OpenedAtUnix = opened.Unix()
	if resolved.Valid {
		out.ResolvedAtUnix = resolved.Time.Unix()
	}
	return &out, nil
}

func (s *Service) ListTrackingEvents(ctx context.Context, req *pb.ListTrackingEventsRequest) (*pb.TrackingEventList, error) {
	shipmentNo := strings.TrimSpace(req.GetShipmentNo())
	if shipmentNo == "" {
		return nil, apperr.Invalid("shipment_no is required")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, shipment_no, carrier_code, tracking_no, event_id,
		status, exception_type, occurred_at, received_at FROM shipment_track_events
		WHERE shipment_no = ? ORDER BY occurred_at, id`, shipmentNo)
	if err != nil {
		return nil, apperr.Internal("list tracking events", err)
	}
	defer rows.Close()
	reply := &pb.TrackingEventList{}
	for rows.Next() {
		var event pb.TrackingEvent
		var occurredAt, receivedAt time.Time
		if err := rows.Scan(&event.Id, &event.ShipmentNo, &event.CarrierCode, &event.TrackingNo,
			&event.EventId, &event.Status, &event.ExceptionType, &occurredAt, &receivedAt); err != nil {
			return nil, apperr.Internal("scan tracking event", err)
		}
		event.OccurredAtUnix = occurredAt.Unix()
		event.ReceivedAtUnix = receivedAt.Unix()
		reply.Events = append(reply.Events, &event)
	}
	return reply, rows.Err()
}

func (s *Service) CreateCarrier(ctx context.Context, in *pb.Carrier) (*pb.Carrier, error) {
	code := strings.TrimSpace(in.GetCode())
	if code == "" || strings.TrimSpace(in.GetName()) == "" {
		return nil, apperr.Invalid("carrier code and name are required")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO carriers (code, name, enabled, sort_order)
		VALUES (?, ?, ?, ?)`, code, truncateFulfillment(in.GetName(), 64), in.GetEnabled(), in.GetSortOrder()); err != nil {
		if isDuplicateKey(err) {
			return nil, apperr.Conflict("carrier already exists")
		}
		return nil, apperr.Internal("create carrier", err)
	}
	if err := s.auditCarrier(ctx, code, "CREATE", "admin", ""); err != nil {
		return nil, err
	}
	return s.GetCarrier(ctx, code)
}

func (s *Service) UpdateCarrier(ctx context.Context, in *pb.Carrier) (*pb.Carrier, error) {
	code := strings.TrimSpace(in.GetCode())
	if code == "" {
		return nil, apperr.Invalid("carrier code is required")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE carriers SET name = ?, sort_order = ?, version = version + 1
		WHERE code = ?`, truncateFulfillment(in.GetName(), 64), in.GetSortOrder(), code)
	if err != nil {
		return nil, apperr.Internal("update carrier", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, apperr.NotFound("carrier not found")
	}
	if err := s.auditCarrier(ctx, code, "UPDATE", "admin", ""); err != nil {
		return nil, err
	}
	return s.GetCarrier(ctx, code)
}

func (s *Service) SetCarrierStatus(ctx context.Context, req *pb.SetCarrierStatusRequest) (*pb.Carrier, error) {
	code := strings.TrimSpace(req.GetCode())
	if code == "" {
		return nil, apperr.Invalid("carrier code is required")
	}
	if strings.TrimSpace(req.GetOperator()) == "" {
		return nil, apperr.Invalid("operator is required")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE carriers SET enabled = ?, version = version + 1 WHERE code = ?`,
		req.GetEnabled(), code)
	if err != nil {
		return nil, apperr.Internal("set carrier status", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, apperr.NotFound("carrier not found")
	}
	if err := s.auditCarrier(ctx, code, "STATUS", req.GetOperator(), req.GetReason()); err != nil {
		return nil, err
	}
	return s.GetCarrier(ctx, code)
}

func (s *Service) ListCarriers(ctx context.Context, _ *pb.Empty) (*pb.CarrierList, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT code, name, enabled, sort_order, version FROM carriers ORDER BY sort_order, code`)
	if err != nil {
		return nil, apperr.Internal("list carriers", err)
	}
	defer rows.Close()
	reply := &pb.CarrierList{}
	for rows.Next() {
		var carrier pb.Carrier
		if err := rows.Scan(&carrier.Code, &carrier.Name, &carrier.Enabled, &carrier.SortOrder, &carrier.Version); err != nil {
			return nil, apperr.Internal("scan carrier", err)
		}
		reply.Carriers = append(reply.Carriers, &carrier)
	}
	return reply, rows.Err()
}

func (s *Service) GetCarrier(ctx context.Context, code string) (*pb.Carrier, error) {
	var carrier pb.Carrier
	err := s.db.QueryRowContext(ctx, `SELECT code, name, enabled, sort_order, version FROM carriers WHERE code = ?`, code).
		Scan(&carrier.Code, &carrier.Name, &carrier.Enabled, &carrier.SortOrder, &carrier.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("carrier not found")
	}
	if err != nil {
		return nil, apperr.Internal("get carrier", err)
	}
	return &carrier, nil
}

func (s *Service) auditCarrier(ctx context.Context, code, action, operator, reason string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO carrier_audit_logs (carrier_code, action, operator, reason)
		VALUES (?, ?, ?, ?)`, code, action, truncateFulfillment(operator, 64), truncateFulfillment(reason, 255))
	if err != nil {
		return apperr.Internal("audit carrier", err)
	}
	return nil
}

func isDuplicateKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

func truncateFulfillment(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

// addBusinessDays adds weekdays only, per OQ-02 (holidays are not modeled yet).
func addBusinessDays(from time.Time, days int) time.Time {
	result := from
	for days > 0 {
		result = result.AddDate(0, 0, 1)
		if result.Weekday() != time.Saturday && result.Weekday() != time.Sunday {
			days--
		}
	}
	return result
}
