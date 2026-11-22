package order

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
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/outbox"
)

const (
	topicOrderShortageReported = "shop.order.shortage.reported.v1"
	topicOrderShortageResolved = "shop.order.shortage.resolved.v1"
	shortageNotified           = "NOTIFIED"
	shortageResolvedWait       = "RESOLVED_WAIT"
	shortageResolvedRefund     = "RESOLVED_REFUND"
	shortageWindow             = 48 * time.Hour
	shortageNotifyWindow       = 4 * time.Hour
	shortageReasonDefault      = "SHORTAGE_DEFAULT"
	shortageReasonUserChoice   = "SHORTAGE_USER_CHOICE"
	shortageItemReasonCode     = "OUT_OF_STOCK"
)

// ReportGroupShortage records the explicit post-payment shortage case. It never
// reallocates a warehouse or adds a group (INV-26).
func (s *Service) ReportGroupShortage(ctx context.Context, req *pb.ReportGroupShortageRequest) (*pb.ShortageCase, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	groupNo := permitGroup(req.GetGroupNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order for shortage", err)
	}
	if record == nil {
		return nil, apperr.NotFound("order not found")
	}
	if record.Status != statusPaid && record.Status != statusAllocated {
		return nil, apperr.Failed(fmt.Sprintf("shortage can only be reported before shipping in status %s", record.Status))
	}
	var groupStatus string
	if err := s.db.QueryRowContext(ctx, `SELECT status FROM order_fulfillment_groups WHERE order_no = ? AND group_no = ?`,
		orderNo, groupNo).Scan(&groupStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("fulfillment group not found")
		}
		return nil, apperr.Internal("load group for shortage", err)
	}
	if groupStatus == "SHIPPED" || groupStatus == "RECEIVED" {
		return nil, apperr.Failed("group has already shipped")
	}
	var eta sql.NullTime
	if value := strings.TrimSpace(req.GetRestockEta()); value != "" {
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil || !parsed.After(time.Now()) {
			return nil, apperr.Invalid("restock_eta must be a future date in YYYY-MM-DD")
		}
		eta = sql.NullTime{Time: parsed, Valid: true}
	}
	detectedAt := s.now()
	notifyDeadline := detectedAt.Add(shortageNotifyWindow)
	notifiedAt := detectedAt
	chooseDeadline := notifiedAt.Add(shortageWindow)
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_shortage_cases
			(order_no, group_no, status, reason, restock_eta, detected_at, notify_deadline_at,
			 notified_at, choose_deadline_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE status = ?, reason = ?, restock_eta = ?, detected_at = ?,
				notify_deadline_at = ?, notified_at = ?, choose_deadline_at = ?,
				version = version + 1, updated_at = NOW(3)`,
			orderNo, groupNo, shortageNotified, truncate(req.GetReason(), 255), eta,
			detectedAt, notifyDeadline, notifiedAt, chooseDeadline,
			shortageNotified, truncate(req.GetReason(), 255), eta, detectedAt,
			notifyDeadline, notifiedAt, chooseDeadline); err != nil {
			return apperr.Internal("record shortage case", err)
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic: topicOrderShortageReported, AggregateType: "order", AggregateID: orderNo,
			EventType: "order.shortage_reported",
			Payload: map[string]any{
				"order_no": orderNo, "group_no": groupNo, "status": shortageNotified,
				"detected_at_unix": detectedAt.Unix(), "notify_deadline_unix": notifyDeadline.Unix(),
				"notified_at_unix": notifiedAt.Unix(), "choose_deadline_unix": chooseDeadline.Unix(),
				"reason_code": shortageItemReasonCode, "created_at": detectedAt.Unix(),
			},
		})
	})
	if err != nil {
		return nil, err
	}
	return s.GetShortageCase(ctx, &pb.GetShortageCaseRequest{OrderNo: orderNo, GroupNo: groupNo})
}

func (s *Service) ChooseShortageResolution(ctx context.Context, req *pb.ChooseShortageResolutionRequest) (*pb.ShortageCase, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	groupNo := permitGroup(req.GetGroupNo())
	resolution := strings.ToUpper(strings.TrimSpace(req.GetResolution()))
	if orderNo == "" || req.GetUserId() <= 0 {
		return nil, apperr.Invalid("order_no and user_id are required")
	}
	if resolution != "WAIT" && resolution != "REFUND" {
		return nil, apperr.Invalid("resolution must be WAIT or REFUND")
	}
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order for shortage choice", err)
	}
	if record == nil || record.UserID != req.GetUserId() {
		return nil, apperr.NotFound("order not found")
	}
	var status, restockEta string
	var deadline sql.NullTime
	err = s.db.QueryRowContext(ctx, `SELECT status, COALESCE(DATE_FORMAT(restock_eta, '%Y-%m-%d'), ''), choose_deadline_at
		FROM order_shortage_cases WHERE order_no = ? AND group_no = ?`, orderNo, groupNo).
		Scan(&status, &restockEta, &deadline)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("shortage case not found")
	}
	if err != nil {
		return nil, apperr.Internal("load shortage case", err)
	}
	if status == shortageResolvedWait || status == shortageResolvedRefund {
		return s.GetShortageCase(ctx, &pb.GetShortageCaseRequest{OrderNo: orderNo, GroupNo: groupNo})
	}
	next := shortageResolvedWait
	if resolution == "REFUND" {
		next = shortageResolvedRefund
	} else if restockEta == "" {
		return nil, apperr.Invalid("WAIT requires a restock_eta date")
	}
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE order_shortage_cases
			SET status = ?, resolution = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND group_no = ? AND status = ?`,
			next, resolution, orderNo, groupNo, shortageNotified); err != nil {
			return apperr.Internal("resolve shortage case", err)
		}
		items, ierr := shortageEventItems(ctx, tx, orderNo, groupNo)
		if ierr != nil {
			return ierr
		}
		reasonCode := shortageReasonUserChoice
		return outbox.Append(ctx, tx, outbox.Event{
			Topic: topicOrderShortageResolved, AggregateType: "order", AggregateID: orderNo,
			EventType: "order.shortage_resolved",
			Payload: map[string]any{
				"order_no": orderNo, "group_no": groupNo, "status": next,
				"resolution": resolution, "reason_code": reasonCode,
				"resolved_at": s.now().Unix(), "items": items,
			},
		})
	})
	if err != nil {
		return nil, err
	}
	return s.GetShortageCase(ctx, &pb.GetShortageCaseRequest{OrderNo: orderNo, GroupNo: groupNo})
}

func (s *Service) GetShortageCase(ctx context.Context, req *pb.GetShortageCaseRequest) (*pb.ShortageCase, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	groupNo := permitGroup(req.GetGroupNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	var out pb.ShortageCase
	var deadline, notified, detected, notifyDeadline, refunded sql.NullTime
	var eta sql.NullTime
	var refundAfterSaleNo string
	err := s.db.QueryRowContext(ctx, `SELECT order_no, group_no, status, resolution, notified_at,
		choose_deadline_at, restock_eta, refund_after_sale_no, refunded_at, refund_state,
		detected_at, notify_deadline_at FROM order_shortage_cases WHERE order_no = ? AND group_no = ?`,
		orderNo, groupNo).Scan(&out.OrderNo, &out.GroupNo, &out.Status, &out.Resolution, &notified, &deadline, &eta,
		&refundAfterSaleNo, &refunded, &out.RefundState, &detected, &notifyDeadline)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("shortage case not found")
	}
	if err != nil {
		return nil, apperr.Internal("load shortage case", err)
	}
	if out.RefundState == "" {
		out.RefundState = "NONE"
	}
	out.RefundAfterSaleNo = refundAfterSaleNo
	if refunded.Valid {
		out.RefundedAtUnix = refunded.Time.Unix()
	}
	if detected.Valid {
		out.DetectedAtUnix = detected.Time.Unix()
	}
	if notifyDeadline.Valid {
		out.NotifyDeadlineUnix = notifyDeadline.Time.Unix()
	}
	if notified.Valid {
		out.NotifiedAtUnix = notified.Time.Unix()
	}
	if deadline.Valid {
		out.ChooseDeadlineUnix = deadline.Time.Unix()
	}
	if eta.Valid {
		out.RestockEta = eta.Time.Format("2006-01-02")
	}
	return &out, nil
}

func shortageEventItems(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, orderNo, groupNo string) ([]map[string]any, error) {
	rows, err := q.QueryContext(ctx, `SELECT line_no, sku_id, quantity FROM order_items
		WHERE order_no = ? AND fulfillment_group_no = ? ORDER BY line_no`, orderNo, groupNo)
	if err != nil {
		return nil, apperr.Internal("load shortage event items", err)
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 2)
	for rows.Next() {
		var lineNo, skuID int64
		var quantity int32
		if err := rows.Scan(&lineNo, &skuID, &quantity); err != nil {
			return nil, apperr.Internal("scan shortage event item", err)
		}
		items = append(items, map[string]any{"line_no": lineNo, "sku_id": skuID, "quantity": quantity})
	}
	return items, rows.Err()
}

// RunShortageWorker applies the 48h fail-safe default (D-05). It writes the
// resolution and an event; the actual group refund is executed by the after-sale
// refund protocol, never by mutating the lifecycle here.
func RunShortageWorker(ctx context.Context, database *sql.DB, svc *Service, logger *slog.Logger, interval time.Duration) {
	if svc == nil || !svc.shortageCaseEnabled {
		return
	}
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := svc.resolveExpiredShortages(ctx); err != nil && logger != nil {
				logger.Error("resolve expired shortages", "error", err)
			}
		}
	}
}

func (s *Service) resolveExpiredShortages(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT order_no, group_no FROM order_shortage_cases
		WHERE status = ? AND choose_deadline_at IS NOT NULL AND choose_deadline_at <= NOW(3) LIMIT 100`, shortageNotified)
	if err != nil {
		return err
	}
	type key struct{ orderNo, groupNo string }
	var keys []key
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.orderNo, &k.groupNo); err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, k)
	}
	rows.Close()
	for _, k := range keys {
		err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
			res, err := tx.ExecContext(ctx, `UPDATE order_shortage_cases
				SET status = ?, resolution = 'REFUND', version = version + 1, updated_at = NOW(3)
				WHERE order_no = ? AND group_no = ? AND status = ?`,
				shortageResolvedRefund, k.orderNo, k.groupNo, shortageNotified)
			if err != nil {
				return apperr.Internal("default shortage resolution", err)
			}
			affected, _ := res.RowsAffected()
			if affected == 0 {
				return nil
			}
			items, ierr := shortageEventItems(ctx, tx, k.orderNo, k.groupNo)
			if ierr != nil {
				return ierr
			}
			payload, _ := json.Marshal(map[string]any{
				"order_no": k.orderNo, "group_no": k.groupNo, "status": shortageResolvedRefund,
				"resolution": "REFUND", "reason_code": shortageReasonDefault,
				"resolved_at": s.now().Unix(), "items": items,
			})
			return outbox.Append(ctx, tx, outbox.Event{
				Topic: topicOrderShortageResolved, AggregateType: "order", AggregateID: k.orderNo,
				EventType: "order.shortage_resolved", Payload: payload,
			})
		})
		if err != nil {
			return err
		}
	}
	return nil
}
