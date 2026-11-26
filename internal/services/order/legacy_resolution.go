package order

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
)

// Legacy PAID payment resolution (SPEC-0002 v1.11 AC-60/AC-63, ADR-0007
// Rev 2.3 §4.4/§5.3). Order is the only writer: Payment is read through
// ListOrderIntents (outside the transaction), the unique candidate is written
// back with a conditional CAS, and 0/multiple candidates enter
// legacy_payment_resolution. The T2.4 SLA worker is deliberately not in this
// file; it owns expiry/COMPENSATING/terminal transitions.
const (
	legacyResolutionStateResolved     = "RESOLVED"
	legacyResolutionStateExpiredZero  = "EXPIRED_ZERO"
	legacyResolutionStateOpen         = "OPEN"
	legacyResolutionStateResolving    = "RESOLVING"
	legacyResolutionStateCompensating = "COMPENSATING"
)

type legacyResolution struct {
	State          string
	PaymentNo      string
	CandidateCount int
}

// ensureLegacyResolution resolves a PAID order whose effective payment is
// empty. It is read-only for already-resolved/non-PAID orders, performs the
// Payment read outside any transaction, then serializes on the orders row.
// A unique SUCCEEDED/captured==pay_cent/order-matching intent is CAS-written;
// zero/no/multiple candidates are queued (or read back) without moving money.
// Other captured intents of a uniquely resolved order are refunded at payment
// level (idempotent `pay:<payment_no>:dup`) and recorded in
// refund_applications; an ambiguous order never refunds anything.
func (s *Service) ensureLegacyResolution(ctx context.Context, orderNo string) (*legacyResolution, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order for legacy resolution", err)
	}
	if record == nil {
		return nil, apperr.NotFound("order not found")
	}
	if record.Status != statusPaid {
		return &legacyResolution{State: "", PaymentNo: strings.TrimSpace(record.SettlementPaymentNo)}, nil
	}
	if settled := strings.TrimSpace(record.SettlementPaymentNo); settled != "" {
		if !strings.HasPrefix(settled, zeroPaymentPrefix) && s.payments != nil {
			if pending, err := s.hasPendingDuplicateRefund(ctx, orderNo); err == nil && pending {
				intents, lerr := s.payments.ListOrderIntents(ctx, &pb.ListOrderIntentsRequest{OrderNo: orderNo})
				if lerr != nil {
					return nil, apperr.Unavailable("list intents to retry duplicate capture refunds", lerr)
				}
				if rerr := s.refundOtherLegacyCaptures(ctx, orderNo, settled, intents.GetIntents()); rerr != nil {
					return nil, rerr
				}
			}
		}
		return &legacyResolution{State: legacySettlementState(settled), PaymentNo: settled}, nil
	}
	if s.payments == nil {
		return nil, apperr.Unavailable("payment client is required to resolve a historical PAID order", nil)
	}
	intents, err := s.payments.ListOrderIntents(ctx, &pb.ListOrderIntentsRequest{OrderNo: orderNo})
	if err != nil {
		return nil, apperr.Unavailable("list order intents for legacy resolution", err)
	}

	anyCaptured := false
	candidates := make([]*pb.Intent, 0, 1)
	for _, in := range s.legacyIntentsForOrder(orderNo, intents.GetIntents()) {
		if in.GetCapturedCent() > 0 {
			anyCaptured = true
		}
		if in.GetStatus() == "SUCCEEDED" && in.GetCapturedCent() == record.PayCent {
			candidates = append(candidates, in)
		}
	}

	// Zero-amount order with no captured intent at all: the zero identity is
	// the only legal resolution. A captured intent (even a zero pay_cent order)
	// must never be turned into zero: it goes to the queue instead.
	if record.PayCent == 0 && !anyCaptured {
		res, err := s.writeZeroResolutionCAS(ctx, orderNo)
		if err != nil {
			return nil, err
		}
		return res, nil
	}
	if len(candidates) == 1 {
		return s.writeUniqueLegacyCandidate(ctx, record, candidates[0], intents.GetIntents())
	}
	return s.enqueueLegacyResolution(ctx, orderNo, int64(len(candidates)), record.PayCent)
}

func legacySettlementState(paymentNo string) string {
	if strings.HasPrefix(paymentNo, zeroPaymentPrefix) {
		return legacyResolutionStateExpiredZero
	}
	return legacyResolutionStateResolved
}

// legacyIntentsForOrder keeps only intents whose order_no matches; a foreign
// intent is evidence of dirty data but never a candidate or a capture of this
// order.
func (s *Service) legacyIntentsForOrder(orderNo string, intents []*pb.Intent) []*pb.Intent {
	out := make([]*pb.Intent, 0, len(intents))
	for _, in := range intents {
		if in == nil || strings.TrimSpace(in.GetOrderNo()) != orderNo {
			continue
		}
		out = append(out, in)
	}
	return out
}

func (s *Service) writeZeroResolutionCAS(ctx context.Context, orderNo string) (*legacyResolution, error) {
	zero := zeroPaymentPrefix + orderNo
	queuedState := ""
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE orders
			SET settlement_payment_no = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND status = ? AND settlement_payment_no = ''`,
			zero, orderNo, statusPaid); err != nil {
			return apperr.Internal("write zero legacy settlement", err)
		}
		var current string
		if err := tx.QueryRowContext(ctx, `SELECT settlement_payment_no FROM orders
			WHERE order_no = ? FOR UPDATE`, orderNo).Scan(&current); err != nil {
			return apperr.Internal("reload zero legacy settlement", err)
		}
		if strings.TrimSpace(current) == zero {
			return nil
		}
		state, qerr := s.enqueueLegacyResolutionTx(ctx, tx, orderNo, 0, 0)
		if qerr != nil {
			return qerr
		}
		queuedState = state
		return nil
	})
	if err != nil {
		return nil, err
	}
	if queuedState != "" {
		return &legacyResolution{State: queuedState}, nil
	}
	return &legacyResolution{State: legacyResolutionStateExpiredZero, PaymentNo: zero}, nil
}

func (s *Service) writeUniqueLegacyCandidate(ctx context.Context, record *orderRecord, candidate *pb.Intent, all []*pb.Intent) (*legacyResolution, error) {
	paymentNo := strings.TrimSpace(candidate.GetPaymentNo())
	if paymentNo == "" {
		return s.enqueueLegacyResolution(ctx, record.OrderNo, 0, record.PayCent)
	}
	enqueued := false
	queuedState := ""
	won := false
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		locked, err := findOrderForUpdate(ctx, tx, record.OrderNo)
		if err != nil {
			return apperr.Internal("lock order for legacy settlement", err)
		}
		if locked == nil {
			return apperr.NotFound("order not found")
		}
		current := strings.TrimSpace(locked.SettlementPaymentNo)
		if current != "" {
			// Another resolver won. Same winner is idempotent; a different one
			// is a settlement conflict and must be queued, never overwritten.
			if current == paymentNo {
				return nil
			}
			enqueued = true
			state, qerr := s.enqueueLegacyResolutionTx(ctx, tx, record.OrderNo, 1, record.PayCent)
			queuedState = state
			return qerr
		}
		res, err := tx.ExecContext(ctx, `UPDATE orders
			SET settlement_payment_no = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND status = ? AND settlement_payment_no = ''`,
			paymentNo, record.OrderNo, statusPaid)
		if err != nil {
			return apperr.Internal("write unique legacy settlement", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return apperr.Internal("write unique legacy settlement rows", err)
		}
		if affected == 1 {
			won = true
			return nil
		}
		var after string
		if err := tx.QueryRowContext(ctx, `SELECT settlement_payment_no FROM orders
			WHERE order_no = ? FOR UPDATE`, record.OrderNo).Scan(&after); err != nil {
			return apperr.Internal("reload unique legacy settlement", err)
		}
		if strings.TrimSpace(after) == paymentNo {
			return nil
		}
		enqueued = true
		state, qerr := s.enqueueLegacyResolutionTx(ctx, tx, record.OrderNo, 1, record.PayCent)
		queuedState = state
		return qerr
	})
	if err != nil {
		return nil, err
	}
	if enqueued {
		return &legacyResolution{State: queuedState, CandidateCount: 1}, nil
	}
	// Only the CAS winner accounts for the other captured intents. A loser
	// reads the same effective payment and must not issue a second remote call.
	if won {
		if err := s.refundOtherLegacyCaptures(ctx, record.OrderNo, paymentNo, all); err != nil {
			return nil, err
		}
	}
	return &legacyResolution{State: legacyResolutionStateResolved, PaymentNo: paymentNo}, nil
}

func (s *Service) enqueueLegacyResolution(ctx context.Context, orderNo string, candidateCount, payCent int64) (*legacyResolution, error) {
	state := legacyResolutionStateOpen
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := findOrderForUpdate(ctx, tx, orderNo); err != nil {
			return apperr.Internal("lock order for legacy queue", err)
		}
		got, qerr := s.enqueueLegacyResolutionTx(ctx, tx, orderNo, candidateCount, payCent)
		state = got
		return qerr
	})
	if err != nil {
		return nil, err
	}
	return &legacyResolution{State: state, CandidateCount: int(candidateCount)}, nil
}

// enqueueLegacyResolutionTx inserts/updates the one row per order. Terminal and
// COMPENSATING rows are never reopened or have their expiry reset; OPEN/
// RESOLVING only refresh the observed candidate count/pay_cent.
func (s *Service) enqueueLegacyResolutionTx(ctx context.Context, tx *sql.Tx, orderNo string, candidateCount, payCent int64) (string, error) {
	var current string
	err := tx.QueryRowContext(ctx, `SELECT state FROM legacy_payment_resolution
		WHERE order_no = ? FOR UPDATE`, orderNo).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `INSERT INTO legacy_payment_resolution
			(order_no, state, candidate_count, pay_cent, blocked_scope, entered_at, expires_at)
			VALUES (?, ?, ?, ?, 'UNSHIPPED_GROUPS_AND_REFUND', NOW(3), DATE_ADD(NOW(3), INTERVAL 24 HOUR))`,
			orderNo, legacyResolutionStateOpen, candidateCount, payCent)
		if err != nil {
			return "", apperr.Internal("enqueue legacy payment resolution", err)
		}
		return legacyResolutionStateOpen, nil
	}
	if err != nil {
		return "", apperr.Internal("load legacy payment resolution", err)
	}
	switch current {
	case legacyResolutionStateOpen, legacyResolutionStateResolving:
		if _, err := tx.ExecContext(ctx, `UPDATE legacy_payment_resolution
			SET candidate_count = ?, pay_cent = ?, updated_at = NOW(3)
			WHERE order_no = ? AND state IN (?, ?)`,
			candidateCount, payCent, orderNo, legacyResolutionStateOpen, legacyResolutionStateResolving); err != nil {
			return "", apperr.Internal("refresh legacy payment resolution", err)
		}
	}
	return current, nil
}

// refundOtherLegacyCaptures is the T2.3 "other captures" edge: once an
// effective payment is known, every other captured intent is refunded at
// payment level with a deterministic refund_id. The application row is persisted
// before the remote call; a remote failure leaves it PENDING for the T2.4
// worker/retry and fails closed to the caller.
func (s *Service) refundOtherLegacyCaptures(ctx context.Context, orderNo, effectivePaymentNo string, intents []*pb.Intent) error {
	if s.payments == nil {
		return apperr.Unavailable("payment client is required to refund duplicate captures", nil)
	}
	for _, in := range s.legacyIntentsForOrder(orderNo, intents) {
		paymentNo := strings.TrimSpace(in.GetPaymentNo())
		if paymentNo == "" || paymentNo == effectivePaymentNo || in.GetCapturedCent() <= 0 {
			continue
		}
		amount := in.GetCapturedCent() - in.GetRefundedCent()
		if amount <= 0 {
			continue
		}
		refundID := "pay:" + paymentNo + ":dup"
		done, err := s.ensurePaymentRefundApplication(ctx, orderNo, refundID, paymentNo, amount)
		if err != nil {
			return err
		}
		if done {
			continue
		}
		if _, err := s.payments.Refund(ctx, &pb.RefundRequest{
			PaymentNo: paymentNo, AmountCent: amount, Reason: "duplicate capture",
			IdempotencyKey: refundID, NonEffectiveCapture: true,
		}); err != nil {
			_ = s.setRefundApplicationError(ctx, refundID, err.Error())
			return apperr.Unavailable("refund duplicate capture", err)
		}
		if err := s.setRefundApplicationState(ctx, refundID, "DONE"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) hasPendingDuplicateRefund(ctx context.Context, orderNo string) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM refund_applications
		WHERE order_no = ? AND source = 'payment' AND non_effective_capture = 1 AND state = 'PENDING'`, orderNo).Scan(&n); err != nil {
		return false, apperr.Internal("check pending duplicate capture refunds", err)
	}
	return n > 0, nil
}

func (s *Service) ensurePaymentRefundApplication(ctx context.Context, orderNo, refundID, paymentNo string, amount int64) (bool, error) {
	var state string
	err := s.db.QueryRowContext(ctx, `SELECT state FROM refund_applications WHERE refund_id = ?`, refundID).Scan(&state)
	if err == nil {
		return state == "DONE", nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, apperr.Internal("load duplicate capture refund application", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO refund_applications
		(refund_id, batch_id, source, order_no, group_no, non_effective_capture,
		 amount_cent, goods_cent, freight_cent, state, admission_result, version)
		VALUES (?, NULL, 'payment', ?, NULL, 1, ?, ?, 0, 'PENDING', '', 1)
		ON DUPLICATE KEY UPDATE refund_id = refund_id`,
		refundID, orderNo, amount, amount); err != nil {
		return false, apperr.Internal("persist duplicate capture refund application", err)
	}
	return false, nil
}

func (s *Service) setRefundApplicationState(ctx context.Context, refundID, state string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE refund_applications
		SET state = ?, version = version + 1, updated_at = NOW(3) WHERE refund_id = ?`, state, refundID); err != nil {
		return apperr.Internal("update duplicate capture refund application", err)
	}
	return nil
}

func (s *Service) setRefundApplicationError(ctx context.Context, refundID, message string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE refund_applications
		SET last_error = ?, version = version + 1, updated_at = NOW(3) WHERE refund_id = ?`,
		truncate(message, 1024), refundID); err != nil {
		return apperr.Internal("record duplicate capture refund error", err)
	}
	return nil
}
