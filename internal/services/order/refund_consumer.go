package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/events"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"

	"github.com/nats-io/nats.go"
)

type paymentRefundedEvent = events.PaymentRefunded

// RunRefundConsumer projects payment refunds onto the order aggregate. The
// payment event is the source of truth; the update is monotonic and idempotent.
func RunRefundConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject: "shop.payment.refunded.v1",
		Durable: "order-refund-projection",
	}, func(hctx context.Context, msg *nats.Msg) error {
		var event paymentRefundedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		if event.OrderNo == "" || event.RefundedCent < 0 {
			return nil
		}
		return applyPaymentRefunded(hctx, svc, event)
	})
	if err != nil {
		logger.Error("subscribe payment.refunded", "error", err)
	}
}

// applyPaymentRefunded projects one Payment refund fact onto the order. Its
// only writers are the amount projection (refunded_cent/refund_status/
// refund_state) and the refund execution ledger (commitment SETTLED,
// application DONE, TCC batch DONE); it never writes the lifecycle status.
//
// T1.4b1: the ledger convergence is independent of the amount projection. An
// event whose cumulative amount is already covered (duplicate or out-of-order
// delivery) must still settle its commitment/application, and a replayed event
// must be a no-op.
func applyPaymentRefunded(ctx context.Context, svc *Service, event paymentRefundedEvent) error {
	if event.NonEffectiveCapture {
		// Duplicate/over-under capture money never enters the order's
		// captured/refunded facts, and therefore never enters a commitment
		// (INV-37).
		return nil
	}
	err := db.WithTx(ctx, svc.db, func(tx *sql.Tx) error {
		var current, paidCent int64
		var status, refundStatus, refundState string
		if err := tx.QueryRowContext(ctx, `SELECT refunded_cent, paid_cent, status, refund_status, refund_state
			FROM orders WHERE order_no = ? FOR UPDATE`, event.OrderNo).
			Scan(&current, &paidCent, &status, &refundStatus, &refundState); err != nil {
			return err
		}
		// AC-59: a refund projection that arrives before the capture/paid fact
		// must be rejected, never applied against a zero-paid order. The
		// consumer framework retries it, so a later capture event can converge.
		if event.RefundedCent > 0 && paidCent <= 0 {
			return apperr.Failed("EVENT_OUT_OF_ORDER: refund projection arrived before capture").
				WithDetails(map[string]any{"order_no": event.OrderNo, "paid_cent": paidCent,
					"refunded_cent": event.RefundedCent, "status": status})
		}

		// 1. Converge the durable refund execution ledger first (commitment,
		//    application, batch). This must happen even when the order amount
		//    projection already includes the refund.
		if event.RefundID != "" {
			if err := svc.settlePaymentRefundLedgerTx(ctx, tx, event); err != nil {
				return err
			}
		} else {
			svc.log.Warn("payment.refunded carries no refund_id; group commitments cannot be settled",
				"order_no", event.OrderNo, "payment_no", event.PaymentNo, "amount_cent", event.AmountCent)
		}

		// 2. Monotonic order-level amount projection.
		totalRefunded := event.OrderRefundedCent
		if totalRefunded <= 0 {
			totalRefunded = event.RefundedCent
		}
		if totalRefunded <= current {
			return nil
		}
		nextRefundStatus := statusPartiallyRefunded
		if event.CapturedCent > 0 && totalRefunded >= event.CapturedCent {
			nextRefundStatus = statusRefunded
		}
		nextRefundState := refundStateFor(totalRefunded, event.CapturedCent)
		// A durable TCC batch owns REFUNDING while it is still paying groups.
		// A partial payment.refunded projection must not demote the order to
		// PARTIAL: the T1.2/T1.3 resume requires REFUNDING to continue the
		// remaining groups (T1.4b1).
		if nextRefundState == refundStatePartial && refundState != refundStateFull {
			active, err := activeTccBatchExists(ctx, tx, event.OrderNo)
			if err != nil {
				return err
			}
			if active {
				nextRefundState = refundStateRefunding
			}
		}
		if refundState == refundStateFull {
			// Never downgrade an order that is already fully refunded.
			nextRefundState = refundStateFull
		}
		if _, err := tx.ExecContext(ctx, `UPDATE orders
			SET refunded_cent = ?, refund_status = ?, refund_state = ?, version = version + 1, updated_at = ?
			WHERE order_no = ?`,
			totalRefunded, nextRefundStatus, nextRefundState, time.Now(), event.OrderNo); err != nil {
			return err
		}
		if err := insertStatusLog(ctx, tx, event.OrderNo, status, status, "payment refund "+nextRefundStatus, "payment"); err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic: "shop.order.refund_projected.v1", AggregateType: "order", AggregateID: event.OrderNo,
			EventType: "order.refund_projected",
			Payload: map[string]any{"order_no": event.OrderNo, "payment_no": event.PaymentNo,
				"refunded_cent": totalRefunded, "captured_cent": event.CapturedCent,
				"refund_status": nextRefundStatus, "refund_state": nextRefundState, "order_status": status},
		})
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || apperr.IsCode(err, apperr.CodeNotFound) {
			if svc.log != nil {
				svc.log.Warn("refund projection order missing", "order_no", event.OrderNo)
			}
			return nil
		}
		return err
	}
	return nil
}

// settlePaymentRefundLedgerTx applies one refund fact to the execution ledger:
// the group commitment RESERVED -> SETTLED, the application -> DONE and, when
// every group of the batch is DONE, the batch PAYING -> DONE. It never touches
// the order lifecycle and never releases quota.
func (s *Service) settlePaymentRefundLedgerTx(ctx context.Context, tx *sql.Tx, event paymentRefundedEvent) error {
	commitment, err := findTccCommitmentForPayment(ctx, tx, event.RefundID)
	if err != nil {
		return err
	}
	if commitment == nil {
		// Payment-level/legacy refunds have no group commitment. The order
		// amount projection still applies; warn instead of guessing a group.
		s.log.Warn("payment.refunded refund_id has no order_refund_commitments row",
			"order_no", event.OrderNo, "payment_no", event.PaymentNo,
			"refund_id", event.RefundID, "amount_cent", event.AmountCent)
		return nil
	}
	if commitment.OrderNo != event.OrderNo {
		return apperr.Failed("payment refunded fact does not match the commitment order").
			WithDetails(map[string]any{
				"refund_id": event.RefundID, "event_order_no": event.OrderNo,
				"commitment_order_no": commitment.OrderNo,
			})
	}
	switch commitment.State {
	case commitmentReserved:
		if event.AmountCent > 0 && commitment.AmountCent != event.AmountCent {
			return apperr.Failed("payment refunded amount does not match the reserved commitment").
				WithDetails(map[string]any{
					"refund_id": event.RefundID, "event_amount_cent": event.AmountCent,
					"commitment_amount_cent": commitment.AmountCent,
				})
		}
		res, err := tx.ExecContext(ctx, `UPDATE order_refund_commitments
			SET state = ?, version = version + 1, updated_at = NOW(3)
			WHERE refund_id = ? AND state = ?`, commitmentSettled, event.RefundID, commitmentReserved)
		if err != nil {
			return apperr.Internal("settle refund commitment", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return apperr.Conflict("refund commitment changed during settlement").
				WithDetails(map[string]any{"refund_id": event.RefundID})
		}
	case commitmentSettled:
		// Duplicate/replayed event: idempotent, keep converging the rest.
	default:
		// RELEASED and any future state: the event must never overwrite a
		// human-owned decision; surface the conflict for reconciliation.
		return apperr.Failed("refund commitment is not settleable from payment.refunded").
			WithDetails(map[string]any{
				"refund_id": event.RefundID, "commitment_state": commitment.State,
				"order_no": event.OrderNo,
			})
	}
	batchID, err := s.markRefundApplicationDoneTx(ctx, tx, event.RefundID)
	if err != nil {
		return err
	}
	if batchID == "" {
		return nil
	}
	pending, err := countPendingTccApplications(ctx, tx, batchID)
	if err != nil {
		return err
	}
	if pending > 0 {
		return nil
	}
	return s.completeTccBatch(ctx, tx, batchID)
}

// markRefundApplicationDoneTx closes the execution application of a settled
// refund. A missing application is legal (Payment-level or directly admitted
// after-sale commitments); a terminal non-DONE application is never rewritten,
// it is surfaced for reconciliation. It returns the TCC batch id (empty for
// payment-level rows) so the caller can converge the batch.
func (s *Service) markRefundApplicationDoneTx(ctx context.Context, tx *sql.Tx, refundID string) (string, error) {
	if _, err := tx.ExecContext(ctx, `UPDATE refund_applications
		SET state = ?, last_error = '', version = version + 1, updated_at = NOW(3)
		WHERE refund_id = ? AND state IN (?, ?)`,
		refundApplicationDone, refundID,
		refundApplicationPending, refundApplicationProcessing); err != nil {
		return "", apperr.Internal("mark refund application done", err)
	}
	var batchID sql.NullString
	var state string
	err := tx.QueryRowContext(ctx, `SELECT batch_id, state FROM refund_applications WHERE refund_id = ?`, refundID).
		Scan(&batchID, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", apperr.Internal("load refund application for settlement", err)
	}
	if state != refundApplicationDone {
		// CANCELLED/REJECTED/FAILED_PERMANENT wins; money moved anyway, so an
		// operator must reconcile. Never rewrite the terminal state here.
		s.log.Error("payment.refunded targeted a refund application that is not DONE-able",
			"refund_id", refundID, "state", state)
	}
	return batchID.String, nil
}
