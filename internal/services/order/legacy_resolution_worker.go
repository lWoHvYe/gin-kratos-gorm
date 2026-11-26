package order

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
)

// RunLegacyResolutionWorker drives the T2.4 SLA state machine. It is safe on
// multiple replicas: every transition is a versioned CAS after an expiring
// lease claim; a loser reads the winner's state and writes nothing.
func RunLegacyResolutionWorker(ctx context.Context, svc *Service, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := svc.expireLegacyResolutions(ctx); err != nil {
				logger.Warn("legacy payment resolution tick", "error", err)
			}
		}
	}
}

func (s *Service) expireLegacyResolutions(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT order_no FROM legacy_payment_resolution
		WHERE state IN (?, ?) AND expires_at <= NOW(3)
		ORDER BY expires_at LIMIT 100`, legacyResolutionStateOpen, legacyResolutionStateResolving)
	if err != nil {
		return apperr.Internal("scan expired legacy resolutions", err)
	}
	var orders []string
	for rows.Next() {
		var orderNo string
		if err := rows.Scan(&orderNo); err == nil {
			orders = append(orders, orderNo)
		}
	}
	_ = rows.Close()
	for _, orderNo := range orders {
		if !s.claimLegacyResolution(ctx, orderNo) {
			continue
		}
		if err := s.processExpiredLegacyResolution(ctx, orderNo); err != nil && !errors.Is(err, errLegacyCompensationBlocked) {
			s.log.Warn("legacy resolution expiry", "order_no", orderNo, "error", err)
		}
	}
	return nil
}

// claimLegacyResolution takes a short lease without changing the state; the
// terminal CAS still compares the row version, so a stale worker cannot win.
func (s *Service) claimLegacyResolution(ctx context.Context, orderNo string) bool {
	res, err := s.db.ExecContext(ctx, `UPDATE legacy_payment_resolution
		SET lease_owner = ?, lease_until = DATE_ADD(NOW(3), INTERVAL 30 SECOND),
		    version = version + 1, updated_at = NOW(3)
		WHERE order_no = ? AND state IN (?, ?) AND expires_at <= NOW(3)
		  AND (lease_until IS NULL OR lease_until < NOW(3))`,
		"legacy-worker", orderNo, legacyResolutionStateOpen, legacyResolutionStateResolving)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n == 1
}

var errLegacyCompensationBlocked = errors.New("legacy compensation release source unsupported")

func (s *Service) processExpiredLegacyResolution(ctx context.Context, orderNo string) error {
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return apperr.Internal("load order for legacy expiry", err)
	}
	if record == nil {
		return nil
	}
	if settled := strings.TrimSpace(record.SettlementPaymentNo); settled != "" {
		// Another command/resolver won; close only the non-terminal queue view
		// without moving money (terminal states are never reopened).
		return s.terminalLegacyResolution(ctx, orderNo, legacyResolutionStateResolved,
			"MANUAL", "NONE", settled)
	}
	if s.payments == nil {
		return apperr.Unavailable("payment client is required for legacy expiry", nil)
	}
	intents, err := s.payments.ListOrderIntents(ctx, &pb.ListOrderIntentsRequest{OrderNo: orderNo})
	if err != nil {
		return apperr.Unavailable("list intents for legacy expiry", err)
	}
	anyCaptured := false
	for _, in := range s.legacyIntentsForOrder(orderNo, intents.GetIntents()) {
		if in.GetCapturedCent() > 0 {
			anyCaptured = true
			break
		}
	}
	if !anyCaptured {
		if record.PayCent == 0 {
			if _, err := s.writeZeroResolutionCAS(ctx, orderNo); err != nil {
				return err
			}
			return s.terminalLegacyResolution(ctx, orderNo, legacyResolutionStateExpiredZero,
				"EXPIRED_ZERO", "NONE", zeroPaymentPrefix+orderNo)
		}
		return s.terminalLegacyResolution(ctx, orderNo, legacyResolutionStateExpiredFinance,
			"EXPIRED_FINANCE", "FINANCE_HOLD", "")
	}
	shipped, err := s.legacyOrderShipped(ctx, orderNo)
	if err != nil {
		return err
	}
	if shipped {
		return s.terminalLegacyResolution(ctx, orderNo, legacyResolutionStateExpiredFinance,
			"EXPIRED_FINANCE", "FINANCE_HOLD", "")
	}
	if err := s.enterLegacyCompensating(ctx, orderNo); err != nil {
		return err
	}
	return s.compensateLegacyResolution(ctx, record, intents.GetIntents())
}

func (s *Service) legacyOrderShipped(ctx context.Context, orderNo string) (bool, error) {
	var shipped int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM order_fulfillment_groups
		WHERE order_no = ? AND status IN ('SHIPPED','RECEIVED')`, orderNo).Scan(&shipped); err != nil {
		return false, apperr.Internal("check shipped groups for legacy expiry", err)
	}
	if shipped > 0 {
		return true, nil
	}
	if s.fulfillment == nil {
		return false, apperr.Unavailable("fulfillment client is required for legacy shipment facts", nil)
	}
	reply, err := s.fulfillment.ListShipments(ctx, &pb.ListShipmentsRequest{OrderNo: orderNo, Page: 1, PageSize: 100})
	if err != nil {
		return false, apperr.Unavailable("load shipment facts for legacy expiry", err)
	}
	if reply == nil || reply.GetTotal() > 100 || int64(len(reply.GetShipments())) != reply.GetTotal() {
		return false, apperr.Failed("shipment facts are incomplete for legacy expiry").
			WithDetails(map[string]any{"order_no": orderNo})
	}
	for _, shipment := range reply.GetShipments() {
		if shipment == nil {
			return false, apperr.Failed("shipment facts contain an empty row")
		}
		switch shipment.GetStatus() {
		case "SHIPPED", "RECEIVED":
			return true, nil
		case "EXCEPTION":
			if shipment.GetShippedAtUnix() > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *Service) enterLegacyCompensating(ctx context.Context, orderNo string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE legacy_payment_resolution
		SET state = ?, lease_owner = '', lease_until = NULL, version = version + 1, updated_at = NOW(3)
		WHERE order_no = ? AND state IN (?, ?)`, legacyResolutionStateCompensating,
		orderNo, legacyResolutionStateOpen, legacyResolutionStateResolving)
	if err != nil {
		return apperr.Internal("enter legacy COMPENSATING", err)
	}
	return nil
}

// compensateLegacyResolution performs the deterministic prefix of ADR §4.4.3:
// revoke NONE permits and cancel CREATED shipments before any money moves. The
// remaining INV-44 release step is blocked in T2.4: the existing release
// producer only accepts tcc/system_zero/system_shortage/aftersale and rejects
// legacy_expired, while T3 cannot be changed here. The row therefore stays
// COMPENSATING (never OPEN, never EXPIRED_REFUND) until that contract is ruled.
func (s *Service) compensateLegacyResolution(ctx context.Context, record *orderRecord, intents []*pb.Intent) error {
	if s.fulfillment == nil {
		return apperr.Unavailable("fulfillment client is required for legacy compensation", nil)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT group_no, status FROM order_fulfillment_groups WHERE order_no = ?`, record.OrderNo)
	if err != nil {
		return apperr.Internal("load legacy compensation groups", err)
	}
	var groups []string
	for rows.Next() {
		var groupNo, status string
		if err := rows.Scan(&groupNo, &status); err != nil {
			continue
		}
		if status != "SHIPPED" && status != "RECEIVED" {
			groups = append(groups, groupNo)
		}
	}
	_ = rows.Close()
	for _, groupNo := range groups {
		var permit string
		err := s.db.QueryRowContext(ctx, `SELECT state FROM order_shipping_permits
			WHERE order_no = ? AND fulfillment_group_no = ?`, record.OrderNo, groupNo).Scan(&permit)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return apperr.Internal("load legacy compensation permit", err)
		}
		if permit == permitNone {
			if _, err := s.db.ExecContext(ctx, `UPDATE order_shipping_permits
				SET state = ?, version = version + 1, updated_at = NOW(3)
				WHERE order_no = ? AND fulfillment_group_no = ? AND state = ?`,
				permitRevoked, record.OrderNo, groupNo, permitNone); err != nil {
				return apperr.Internal("revoke legacy compensation permit", err)
			}
		}
		if permit == permitConsumed {
			reply, err := s.fulfillment.CancelIfCreated(ctx, &pb.CancelIfCreatedRequest{
				OrderNo: record.OrderNo, GroupNo: groupNo,
				RefundId: "pay:" + strings.TrimSpace(record.SettlementPaymentNo) + ":legacy_expired",
			})
			if err != nil {
				return apperr.Unavailable("cancel legacy compensation shipment", err)
			}
			switch reply.GetStatus() {
			case "ALREADY_SHIPPED", "RECEIVED":
				return s.terminalLegacyResolution(ctx, record.OrderNo, legacyResolutionStateExpiredFinance,
					"EXPIRED_FINANCE", "FINANCE_HOLD", "")
			}
		}
	}
	// Blocked contract edge: see function comment. Do not write a misleading
	// release event with another source, and do not call Payment before it.
	return errLegacyCompensationBlocked
}

func (s *Service) terminalLegacyResolution(ctx context.Context, orderNo, state, kind, scope, paymentNo string) error {
	ledgerState := ""
	switch state {
	case legacyResolutionStateExpiredZero:
		ledgerState = zeroPaymentPrefix + orderNo
	case legacyResolutionStateResolved:
		ledgerState = paymentNo
	}
	return db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		var version int64
		var current string
		if err := tx.QueryRowContext(ctx, `SELECT state, version FROM legacy_payment_resolution
			WHERE order_no = ? FOR UPDATE`, orderNo).Scan(&current, &version); err != nil {
			return apperr.Internal("load legacy terminal row", err)
		}
		if current != legacyResolutionStateOpen && current != legacyResolutionStateResolving &&
			current != legacyResolutionStateCompensating {
			return nil
		}
		if ledgerState != "" {
			if _, err := tx.ExecContext(ctx, `UPDATE orders
				SET settlement_payment_no = ?, version = version + 1, updated_at = NOW(3)
				WHERE order_no = ? AND status = ? AND settlement_payment_no = ''`,
				ledgerState, orderNo, statusPaid); err != nil {
				return apperr.Internal("write legacy terminal settlement", err)
			}
		}
		_, err := tx.ExecContext(ctx, `UPDATE legacy_payment_resolution
			SET state = ?, resolution_kind = ?, resolution_id = ?, blocked_scope = ?,
			    resolved_payment_no = ?, resolved_at = IF(? IN ('RESOLVED','EXPIRED_ZERO','EXPIRED_REFUND'), NOW(3), resolved_at),
			    closed_at = IF(? IN ('EXPIRED_FINANCE','REJECTED'), NOW(3), closed_at),
			    lease_owner = '', lease_until = NULL, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND state IN (?, ?, ?) AND version = ?`,
			state, kind, "lpr:"+orderNo, scope, paymentNo,
			state, state, orderNo,
			legacyResolutionStateOpen, legacyResolutionStateResolving, legacyResolutionStateCompensating, version)
		if err != nil {
			return apperr.Internal("write legacy terminal state", err)
		}
		return nil
	})
}

// ---- RPC: Get/Resolve/Resume (existing proto, no new RPC) ----

func (s *Service) GetLegacyResolution(ctx context.Context, req *pb.GetLegacyResolutionRequest) (*pb.LegacyResolution, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	return s.loadLegacyResolution(ctx, s.db, orderNo)
}

func (s *Service) ResolveLegacyResolution(ctx context.Context, req *pb.ResolveLegacyResolutionRequest) (*pb.LegacyResolution, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	paymentNo := strings.TrimSpace(req.GetPaymentNo())
	operator := strings.TrimSpace(req.GetOperator())
	authorizedBy := strings.TrimSpace(req.GetAuthorizedBy())
	evidence := strings.TrimSpace(req.GetEvidenceRef())
	resolutionID := strings.TrimSpace(req.GetResolutionId())
	if orderNo == "" || paymentNo == "" || operator == "" || authorizedBy == "" || evidence == "" || resolutionID == "" {
		return nil, apperr.Invalid("order_no, resolution_id, payment_no, operator, authorized_by and evidence_ref are required")
	}
	if resolutionID != "lpr:"+orderNo {
		return nil, apperr.Failed("resolution_id does not match the order").
			WithDetails(map[string]any{"order_no": orderNo, "resolution_id": resolutionID})
	}
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		var state string
		if err := tx.QueryRowContext(ctx, `SELECT state FROM legacy_payment_resolution
			WHERE order_no = ? FOR UPDATE`, orderNo).Scan(&state); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apperr.NotFound("legacy resolution not found")
			}
			return apperr.Internal("load legacy resolution for resolve", err)
		}
		if state != legacyResolutionStateOpen && state != legacyResolutionStateResolving {
			return apperr.Failed("legacy resolution is not OPEN/RESOLVING").
				WithDetails(map[string]any{"order_no": orderNo, "state": state})
		}
		var current string
		if err := tx.QueryRowContext(ctx, `SELECT settlement_payment_no FROM orders
			WHERE order_no = ? FOR UPDATE`, orderNo).Scan(&current); err != nil {
			return apperr.Internal("lock order for legacy resolve", err)
		}
		if strings.TrimSpace(current) != "" {
			return apperr.Conflict("order already has an effective payment")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE orders
			SET settlement_payment_no = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND status = ? AND settlement_payment_no = ''`,
			paymentNo, orderNo, statusPaid); err != nil {
			return apperr.Internal("write resolved legacy payment", err)
		}
		_, err := tx.ExecContext(ctx, `UPDATE legacy_payment_resolution
			SET state = ?, resolution_kind = 'MANUAL', resolution_id = ?, blocked_scope = 'NONE',
			    resolved_payment_no = ?, operator = ?, authorized_by = ?, evidence_ref = ?,
			    resolved_at = NOW(3), lease_owner = '', lease_until = NULL,
			    version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND state IN (?, ?)`,
			legacyResolutionStateResolved, resolutionID, paymentNo, operator, authorizedBy, evidence,
			orderNo, legacyResolutionStateOpen, legacyResolutionStateResolving)
		if err != nil {
			return apperr.Internal("write legacy RESOLVED", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.loadLegacyResolution(ctx, s.db, orderNo)
}

func (s *Service) ResumeLegacyResolution(ctx context.Context, req *pb.ResumeLegacyResolutionRequest) (*pb.LegacyResolution, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	return s.loadLegacyResolution(ctx, s.db, orderNo)
}

func (s *Service) loadLegacyResolution(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, orderNo string) (*pb.LegacyResolution, error) {
	var out pb.LegacyResolution
	var expires time.Time
	err := q.QueryRowContext(ctx, `SELECT order_no, state, candidate_count, pay_cent, blocked_scope,
		resolution_kind, resolution_id, resolved_payment_no, operator, authorized_by, version, expires_at
		FROM legacy_payment_resolution WHERE order_no = ?`, orderNo).
		Scan(&out.OrderNo, &out.State, &out.CandidateCount, &out.PayCent, &out.BlockedScope,
			&out.ResolutionKind, &out.ResolutionId, &out.ResolvedPaymentNo, &out.Operator,
			&out.AuthorizedBy, &out.Version, &expires)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("legacy resolution not found")
		}
		return nil, apperr.Internal("load legacy resolution view", err)
	}
	out.ExpiresAtUnix = expires.Unix()
	return &out, nil
}
