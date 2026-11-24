package order

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/outbox"
)

// This file implements T1.6: the payment-level cancel of a captured payment.
// It is deliberately separate from the per-group TCC refund ledger: a
// PENDING_PAYMENT order with captured money is cancelled, not refunded through
// refund_batches/refund_applications/commitments, and no shipping permit is
// touched (ADR-0007 Amd1 T1.6 Addendum §2.1/§3.1).
//
// The durable command is written to the existing order.refund_requested subject
// with a payment-level identity `pay:<payment_no>:cancel`. Payment consumes it
// and executes one idempotent Refund; Order never calls Payment.Refund itself.

const (
	// paymentCancelSource identifies a payment-level (not group-level) refund.
	paymentCancelSource = "payment"
	// paymentCancelReasonPending is the reason written by the PENDING_PAYMENT
	// CAS->CANCELLED branch.
	paymentCancelReasonPending = "pending captured cancel"
	// paymentCancelReasonAfterConfirm is the reason written when a payment
	// confirmation lost the cancel CAS after inventory had been confirmed.
	paymentCancelReasonAfterConfirm = "order cancelled after inventory confirm"

	// tccErrCancelCASLost is the stable reason for a lost PENDING_PAYMENT ->
	// CANCELLED CAS. The caller continues the paid group fan-out; it must never
	// emit the payment-level cancel command for a PAID order.
	tccErrCancelCASLost = "CANCEL_CAS_LOST"
)

// errTccCancelCASLost is the private transaction sentinel. It is translated
// into either the paid group fan-out (order became PAID) or an idempotent
// success/command repair (order was cancelled concurrently) after rollback.
var errTccCancelCASLost = errors.New("tcc pending cancel CAS lost")

// paymentCancelRefundID is the deterministic payment-level refund identity.
// It is stable across retries, so Payment.Refund is idempotent by this key.
func paymentCancelRefundID(paymentNo string) string {
	return "pay:" + strings.TrimSpace(paymentNo) + ":cancel"
}

// paymentCancelAmount is the remaining refundable amount of the authoritative
// Payment snapshot. A fully refunded capture has nothing left to command.
func paymentCancelAmount(snapshot *tccPaymentSnapshot) int64 {
	if snapshot == nil {
		return 0
	}
	remaining := snapshot.CapturedCent - snapshot.RefundedCent
	if remaining < 0 {
		return 0
	}
	return remaining
}

// appendPaymentCancelCommandTx writes the durable payment-level cancel command
// in the caller's transaction. It writes no group/batch/application/admission/
// commitment state and never calls Payment (ADR T1.6 §2.1/§3.1).
func appendPaymentCancelCommandTx(ctx context.Context, tx *sql.Tx, record *orderRecord, paymentNo string,
	amountCent, capturedCent int64, reason string) error {
	if record == nil {
		return apperr.Internal("payment cancel command requires an order", nil)
	}
	paymentNo = strings.TrimSpace(paymentNo)
	if paymentNo == "" {
		return apperr.Failed("payment cancel command requires payment_no").
			WithDetails(map[string]any{"order_no": record.OrderNo, "reason": "PAYMENT_CANCEL_NO_PAYMENT_NO"})
	}
	if amountCent <= 0 {
		return nil
	}
	return outbox.Append(ctx, tx, outbox.Event{
		Topic:         topicOrderRefundRequested,
		AggregateType: "order",
		AggregateID:   record.OrderNo,
		EventType:     "order.refund_requested",
		Payload: map[string]any{
			"order_no":              record.OrderNo,
			"payment_no":            paymentNo,
			"refund_id":             paymentCancelRefundID(paymentNo),
			"source":                paymentCancelSource,
			"non_effective_capture": true,
			"amount_cent":           amountCent,
			"paid_cent":             capturedCent,
			"reason":                reason,
			"status":                statusCancelled,
			"created_at":            time.Now().Unix(),
		},
	})
}

// cancelPendingCapturedAfterTCC is the T1.6 replacement of the T1.2 R-3
// fail-closed guard. It locks the order, CASes PENDING_PAYMENT -> CANCELLED and
// writes order.cancelled plus the payment-level command in one transaction.
//
// The second return value reports whether the caller must continue the normal
// PAID group fan-out because payment confirmation won the CAS. In that case the
// transaction rolled back and no order.cancelled/payment command was written.
func (s *Service) cancelPendingCapturedAfterTCC(ctx context.Context, record *orderRecord,
	snapshot *tccPaymentSnapshot, paymentNo string) (bool, error) {
	if record == nil || snapshot == nil {
		return false, apperr.Internal("pending captured cancel requires an order and a payment snapshot", nil)
	}
	paymentNo = strings.TrimSpace(paymentNo)
	reason := paymentCancelReasonPending
	if paymentNo != "" {
		reason = truncate(reason+" "+paymentNo, 255)
	}
	items, err := loadOrderItems(ctx, s.db, record.OrderNo)
	if err != nil {
		return false, apperr.Internal("load order items for pending captured cancel", err)
	}
	amount := paymentCancelAmount(snapshot)
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		locked, err := findOrderForUpdate(ctx, tx, record.OrderNo)
		if err != nil {
			return apperr.Internal("lock order for pending captured cancel", err)
		}
		if locked == nil {
			return apperr.NotFound("order not found")
		}
		if locked.Status != statusPendingPayment {
			// Payment confirmation (or another cancel) won between the
			// authoritative Payment snapshot and this lock.
			return errTccCancelCASLost
		}
		affected, err := updateOrderStatus(ctx, tx, record.OrderNo, statusPendingPayment, statusCancelled, locked.PaidCent)
		if err != nil {
			return err
		}
		if affected == 0 {
			return errTccCancelCASLost
		}
		if err := insertStatusLog(ctx, tx, record.OrderNo, statusPendingPayment, statusCancelled, reason, "system"); err != nil {
			return err
		}
		// order.cancelled drives the Inventory compensation (HELD release /
		// CONFIRMED restock) asynchronously; Order never calls Inventory here.
		if err := outbox.Append(ctx, tx, outbox.Event{
			Topic:         topicOrderCancelled,
			AggregateType: "order",
			AggregateID:   record.OrderNo,
			EventType:     "order.cancelled",
			Payload: map[string]any{
				"order_no":     record.OrderNo,
				"user_id":      record.UserID,
				"quote_id":     record.QuoteID,
				"reason":       reason,
				"operator":     "system",
				"status":       statusCancelled,
				"cancelled_at": time.Now().Unix(),
				"items":        orderItemsPayload(items),
			},
		}); err != nil {
			return err
		}
		// A concurrent cancel can already have consumed the capture; only the
		// remaining amount is refundable. Payment.Refund additionally enforces
		// this cap under its intent lock.
		return appendPaymentCancelCommandTx(ctx, tx, locked, paymentNo, amount, snapshot.CapturedCent, reason)
	})
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, errTccCancelCASLost) {
		return false, apperr.Internal("cancel pending captured order", err)
	}

	latest, err := findOrder(ctx, s.db, record.OrderNo)
	if err != nil {
		return false, apperr.Internal("reload order after pending captured cancel CAS", err)
	}
	if latest == nil {
		return false, apperr.NotFound("order not found")
	}
	switch latest.Status {
	case statusPaid, statusAllocated, statusShipped, statusCompleted:
		// CANCEL_CAS_LOST: payment confirmation won (the lifecycle may already
		// have advanced). No payment-level cancel may be emitted; continue the
		// ordinary PAID group refund fan-out.
		s.log.Warn("TCC cancel CAS lost to payment confirmation; continuing group refund",
			"order_no", record.OrderNo, "payment_no", paymentNo, "status", latest.Status,
			"reason", tccErrCancelCASLost)
		return true, nil
	case statusCancelled:
		// A concurrent cancel won. Repair the durable pieces idempotently:
		// restock a confirmed reservation (if any) and persist the payment
		// command. The refund identity is fixed, so a duplicate is harmless.
		// Reuse the snapshot already read for this decision instead of taking a
		// second Payment snapshot that could fail after the cap was proven.
		if err := s.refundCancelledWithSnapshot(ctx, latest, snapshot, paymentNo, paymentCancelReasonAfterConfirm); err != nil {
			return false, err
		}
		return false, nil
	default:
		return false, apperr.Conflict("TCC cancel lost the order CAS and the order is not cancellable").
			WithDetails(map[string]any{
				"order_no": record.OrderNo, "payment_no": paymentNo,
				"status": latest.Status, "reason": tccErrCancelCASLost,
			})
	}
}

// refundCancelledWithSnapshot is the durable cancel-after-confirm repair:
// Inventory is compensated first (HELD release / CONFIRMED restock), then the
// payment-level command is written with the authoritative captured amount.
// Inventory failure is queued durably and alerted; a command append failure is
// returned so the caller does not treat the capture as refunded.
func (s *Service) refundCancelledWithSnapshot(ctx context.Context, record *orderRecord,
	snapshot *tccPaymentSnapshot, paymentNo, reason string) error {
	if record == nil || snapshot == nil {
		return apperr.Internal("cancel-after-confirm refund requires an order and a payment snapshot", nil)
	}
	paymentNo = strings.TrimSpace(paymentNo)
	if paymentNo == "" {
		s.log.Error("cancelled-after-confirm capture has no payment_no; no refund command written",
			"order_no", record.OrderNo, "reason", "PAYMENT_CANCEL_NO_PAYMENT_NO")
		return nil
	}
	amount := paymentCancelAmount(snapshot)
	if amount <= 0 {
		s.log.Warn("cancelled-after-confirm capture has no refundable amount; no refund command written",
			"order_no", record.OrderNo, "payment_no", paymentNo,
			"captured_cent", snapshot.CapturedCent, "refunded_cent", snapshot.RefundedCent,
			"reason", "PAYMENT_CANCEL_NO_CAPTURE")
		return nil
	}
	items, err := loadOrderItems(ctx, s.db, record.OrderNo)
	if err != nil {
		return apperr.Internal("load items for cancelled-after-confirm restock", err)
	}
	if s.inventory != nil {
		if _, err := s.inventory.TccCancelConfirm(ctx, &pb.TccPaymentRequest{
			OrderNo: record.OrderNo, Items: items,
		}); err != nil {
			// The order remains cancelled and the payment must still be
			// refunded. Persist a durable Inventory retry and alert; do not
			// silently swallow the compensation failure.
			s.log.Error("restock cancelled-after-confirm order", "order_no", record.OrderNo, "error", err)
			s.queueInventoryCompensation(ctx, record.OrderNo, items)
		}
	}
	if err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		return appendPaymentCancelCommandTx(ctx, tx, record, paymentNo, amount, snapshot.CapturedCent, reason)
	}); err != nil {
		return apperr.Internal("persist payment-level cancel command", err)
	}
	return nil
}
