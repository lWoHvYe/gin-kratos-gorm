package payment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/events"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/outbox"
)

// RefundBatch (C-04 §2, PAY-SETTLE-01 T1.4b1-R) is the atomic group-refund
// batch. It closes the "preflight said 10000, an external refund lands, then
// G1 succeeds and G2 is rejected" partial-payout window: the whole item set is
// validated and written under the payment_intents row lock, and any failure
// rolls back every refund row and every payment.refunded event.
//
// Contract decisions that the code makes explicit:
//
//   - Σ items == expected_total_cent is enforced. expected_total_cent is the
//     caller's outstanding (Order owns B/done/inFlight); Payment never derives
//     or reads the Order batch cap.
//   - Captured capacity is checked against the NEW items only. A batch may
//     include an item that already exists with the same refund_id/amount
//     (ALREADY_REFUNDED, C-04 §2.4); its amount is already part of
//     intent.refunded_cent, so charging it again would double-count. For an
//     all-new batch this is exactly Σ items <= captured - refunded.
//   - The batch idempotency key is (payment_no, batch_id). The persistent
//     payment_refund_batches row stores the canonical item fingerprint and the
//     per-item reply. A replay of the same request returns that stored reply and
//     writes nothing; a different item set is IDEMPOTENCY_CONFLICT with zero
//     writes.
//   - refund_id is stored as refunds.idempotency_key, the single per-refund
//     idempotency identity. A refund_id that already exists with a different
//     amount is ITEM_CONFLICT and the whole batch is rejected before any write.
//
// Error codes are carried as apperr codes plus a machine-readable
// details["reason"] value (C-04 §2.7): INVALID_BATCH, BATCH_SUM_MISMATCH,
// INSUFFICIENT_REMAINING, IDEMPOTENCY_CONFLICT, ITEM_CONFLICT, BATCH_UNAVAILABLE.
// Two deterministic reasons outside C-04's minimum list keep Order from
// retrying a non-convergent state: PAYMENT_NOT_REFUNDABLE (intent was never
// captured) and BATCH_STATE_INVALID (persisted ledger/refund facts diverge).
const (
	refundBatchStatusRefunded        = "REFUNDED"
	refundBatchStatusAlreadyRefunded = "ALREADY_REFUNDED"

	refundBatchReasonInvalid             = "INVALID_BATCH"
	refundBatchReasonSumMismatch         = "BATCH_SUM_MISMATCH"
	refundBatchReasonInsufficient        = "INSUFFICIENT_REMAINING"
	refundBatchReasonIdempotencyConflict = "IDEMPOTENCY_CONFLICT"
	refundBatchReasonItemConflict        = "ITEM_CONFLICT"
	refundBatchReasonUnavailable         = "BATCH_UNAVAILABLE"
	// Deterministic preconditions outside C-04's minimum list. They must not be
	// reported as BATCH_UNAVAILABLE, because Order would retry a state that can
	// never converge.
	refundBatchReasonNotRefundable = "PAYMENT_NOT_REFUNDABLE"
	refundBatchReasonStateInvalid  = "BATCH_STATE_INVALID"

	// paymentNoMax, batchIDMax and refundIDMax mirror migration 0012. Checked
	// before the transaction so an oversized identity is INVALID_BATCH instead
	// of a truncated/failed row.
	paymentNoMax = 64
	batchIDMax   = 96
	refundIDMax  = 128
)

// refundBatchItem is a validated request item.
type refundBatchItem struct {
	refundID   string
	amountCent int64
}

// refundBatchItemResult is both the persisted per-item reply and the wire
// result. It is stored as JSON in payment_refund_batches.item_results.
type refundBatchItemResult struct {
	RefundID   string `json:"refund_id"`
	RefundNo   string `json:"refund_no"`
	Status     string `json:"status"`
	AmountCent int64  `json:"amount_cent"`
}

func (r refundBatchItemResult) toPB() *pb.RefundBatchItemResult {
	return &pb.RefundBatchItemResult{
		RefundId: r.RefundID, RefundNo: r.RefundNo, Status: r.Status, AmountCent: r.AmountCent,
	}
}

// RefundBatch atomically refunds every item in the batch (C-04).
func (s *Service) RefundBatch(ctx context.Context, req *pb.RefundBatchRequest) (*pb.RefundBatchReply, error) {
	paymentNo, batchID, reason, items, err := normalizeRefundBatch(req)
	if err != nil {
		return nil, err
	}
	fingerprint, totalCent := refundBatchFingerprint(items)
	r := newRepo(s.db)
	now := s.now()

	var reply *pb.RefundBatchReply
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		// Lock the intent first. Every money-moving path (Refund, RefundBatch and
		// any future capture/refund writer) must lock the same row before it
		// reads captured/refunded, which serializes competing batches.
		in, err := r.intentByNo(ctx, tx, paymentNo, true)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound("payment intent not found")
		}
		if err != nil {
			return err
		}

		prior, err := r.refundBatchByKey(ctx, tx, paymentNo, batchID, true)
		switch {
		case err == nil:
			rep, err := replayRefundBatch(ctx, tx, r, prior, items, fingerprint, totalCent)
			if err != nil {
				return err
			}
			reply = rep
			return nil
		case !errors.Is(err, sql.ErrNoRows):
			return err
		}

		if in.Status != statusSucceeded && in.Status != statusPartiallyRefunded && in.Status != statusRefunded {
			return refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonNotRefundable,
				"payment intent is %s; only captured payments can be refunded", in.Status)
		}
		captured := capturedCent(in)
		if in.RefundedCent < 0 || in.RefundedCent > captured {
			return refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonStateInvalid,
				"payment intent refunded_cent %d is outside [0, captured_cent %d]", in.RefundedCent, captured)
		}
		if in.Status == statusRefunded && in.RefundedCent < captured {
			return refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonStateInvalid,
				"payment intent is REFUNDED but refunded_cent %d is below captured_cent %d", in.RefundedCent, captured)
		}

		results := make(map[string]refundBatchItemResult, len(items))
		newItems := make([]refundBatchItem, 0, len(items))
		var existingSum int64
		for _, item := range items {
			existing, err := r.refundByKey(ctx, tx, paymentNo, item.refundID)
			switch {
			case err == nil:
				if existing.AmountCent != item.amountCent {
					return refundBatchError(apperr.CodeConflict, refundBatchReasonItemConflict,
						"refund_id %s already exists with amount_cent %d, requested %d",
						item.refundID, existing.AmountCent, item.amountCent)
				}
				if existing.Status != statusSucceeded && existing.Status != statusRefunded {
					return refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonStateInvalid,
						"refund_id %s is in status %s; a batch cannot adopt it", item.refundID, existing.Status)
				}
				existingSum += item.amountCent
				results[item.refundID] = refundBatchItemResult{
					RefundID: item.refundID, RefundNo: existing.RefundNo,
					Status: refundBatchStatusAlreadyRefunded, AmountCent: item.amountCent,
				}
			case errors.Is(err, sql.ErrNoRows):
				newItems = append(newItems, item)
			default:
				return err
			}
		}
		if existingSum > in.RefundedCent {
			return refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonStateInvalid,
				"existing refund facts %d exceed payment intent refunded_cent %d", existingSum, in.RefundedCent)
		}
		remaining := captured - in.RefundedCent
		newSum := totalCent - existingSum
		if newSum > remaining {
			return refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonInsufficient,
				"batch requests %d new cents but only %d remain refundable", newSum, remaining)
		}

		status := refundBatchStatusAlreadyRefunded
		nextStatus := in.Status
		totalRefunded := in.RefundedCent
		if len(newItems) > 0 {
			status = refundBatchStatusRefunded
			totalRefunded = in.RefundedCent + newSum
			nextStatus = refundedIntentStatus(totalRefunded, captured)
			// One aggregate update for the whole batch, never one commit per item.
			if err := r.updateIntentRefund(ctx, tx, in.PaymentNo, totalRefunded, nextStatus, now); err != nil {
				return err
			}
			for _, item := range newItems {
				ref := &refund{
					RefundNo:       id.New("RFD"),
					PaymentNo:      in.PaymentNo,
					OrderNo:        in.OrderNo,
					AmountCent:     item.amountCent,
					Reason:         reason,
					Status:         statusSucceeded,
					Channel:        in.Channel,
					IdempotencyKey: item.refundID,
					BatchID:        batchID,
				}
				if err := r.insertRefund(ctx, tx, ref, now); err != nil {
					if isDuplicateKey(err) {
						// The pre-insert read already saw every committed row under
						// the intent lock, so a duplicate here means an uncoordinated
						// writer or a corrupted state. Fail the whole batch closed.
						return refundBatchError(apperr.CodeConflict, refundBatchReasonItemConflict,
							"refund_id %s conflicted with a concurrent refund", item.refundID)
					}
					return err
				}
				results[item.refundID] = refundBatchItemResult{
					RefundID: item.refundID, RefundNo: ref.RefundNo,
					Status: refundBatchStatusRefunded, AmountCent: item.amountCent,
				}
			}

			var orderRefunded int64
			if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(refunded_cent), 0)
				FROM payment_intents WHERE order_no = ?`, in.OrderNo).Scan(&orderRefunded); err != nil {
				return apperr.Internal("sum order refunds", err)
			}
			for _, item := range newItems {
				result := results[item.refundID]
				if err := outbox.Append(ctx, tx, outbox.Event{
					Topic:         topicPaymentRefunded,
					AggregateType: aggregatePayment,
					AggregateID:   in.PaymentNo,
					EventType:     eventRefunded,
					Payload: events.PaymentRefunded{
						RefundNo: result.RefundNo, RefundID: item.refundID,
						PaymentNo: in.PaymentNo, OrderNo: in.OrderNo,
						UserID: in.UserID, AmountCent: item.amountCent,
						RefundedCent: totalRefunded, OrderRefundedCent: orderRefunded,
						CapturedCent: captured, Reason: reason, Channel: in.Channel,
						Status: nextStatus, IdempotencyKey: item.refundID,
					},
				}); err != nil {
					return err
				}
			}
		}

		ordered := make([]refundBatchItemResult, 0, len(items))
		for _, item := range items {
			ordered = append(ordered, results[item.refundID])
		}
		payload, err := json.Marshal(ordered)
		if err != nil {
			return apperr.Internal("encode batch item results", err)
		}
		record := &refundBatchRecord{
			PaymentNo: paymentNo, BatchID: batchID,
			ItemsFingerprint: fingerprint, Status: status,
			RefundedCent: totalCent, Reason: reason, ItemResults: payload,
		}
		if err := r.insertRefundBatch(ctx, tx, record, now); err != nil {
			if isDuplicateKey(err) {
				return refundBatchError(apperr.CodeConflict, refundBatchReasonIdempotencyConflict,
					"batch_id %s was committed concurrently", batchID)
			}
			return err
		}

		reply = &pb.RefundBatchReply{
			BatchId: batchID, Status: status, RefundedCent: totalCent,
			ItemResults: refundBatchResultsToPB(ordered),
		}
		return nil
	})
	if err != nil {
		s.log.Error("refund batch failed", "payment_no", paymentNo, "batch_id", batchID, "error", err)
		return nil, encodeRefundBatchError(err)
	}
	s.log.Info("payment refund batch committed", "payment_no", paymentNo, "batch_id", batchID,
		"items", len(items), "refunded_cent", totalCent, "status", reply.GetStatus())
	return reply, nil
}

// replayRefundBatch returns the persisted reply for a duplicate
// (payment_no, batch_id) request. It writes nothing. The fingerprint check is
// what turns "same batch_id, different items" into IDEMPOTENCY_CONFLICT instead
// of a second refund; the refund row re-check fails closed if the ledger and the
// refund facts ever diverge.
func replayRefundBatch(ctx context.Context, tx *sql.Tx, r *repo, prior *refundBatchRecord,
	items []refundBatchItem, fingerprint string, totalCent int64) (*pb.RefundBatchReply, error) {
	if prior.ItemsFingerprint != fingerprint {
		return nil, refundBatchError(apperr.CodeConflict, refundBatchReasonIdempotencyConflict,
			"batch_id %s already exists with a different item set", prior.BatchID)
	}
	if prior.Status != refundBatchStatusRefunded && prior.Status != refundBatchStatusAlreadyRefunded {
		return nil, refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonStateInvalid,
			"batch_id %s has unknown persisted status %s", prior.BatchID, prior.Status)
	}
	if prior.RefundedCent != totalCent {
		return nil, refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonStateInvalid,
			"batch_id %s persisted refunded_cent %d does not equal the item sum %d",
			prior.BatchID, prior.RefundedCent, totalCent)
	}
	var stored []refundBatchItemResult
	if err := json.Unmarshal(prior.ItemResults, &stored); err != nil {
		return nil, refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonStateInvalid,
			"decode persisted item results of batch %s: %v", prior.BatchID, err)
	}
	if len(stored) != len(items) {
		return nil, refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonStateInvalid,
			"batch %s persisted %d item results for %d requested items", prior.BatchID, len(stored), len(items))
	}
	byID := make(map[string]refundBatchItemResult, len(stored))
	for _, result := range stored {
		if _, duplicate := byID[result.RefundID]; duplicate {
			return nil, refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonStateInvalid,
				"batch %s persisted duplicate refund_id %s", prior.BatchID, result.RefundID)
		}
		byID[result.RefundID] = result
	}
	for _, item := range items {
		result, ok := byID[item.refundID]
		if !ok || result.AmountCent != item.amountCent {
			return nil, refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonStateInvalid,
				"batch %s persisted item set diverges from its fingerprint", prior.BatchID)
		}
		fact, err := r.refundByKey(ctx, tx, prior.PaymentNo, item.refundID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonStateInvalid,
				"batch %s is persisted but refund_id %s has no refund row", prior.BatchID, item.refundID)
		}
		if err != nil {
			return nil, err
		}
		if fact.AmountCent != result.AmountCent || fact.RefundNo != result.RefundNo {
			return nil, refundBatchError(apperr.CodeFailedPrecondition, refundBatchReasonStateInvalid,
				"batch %s refund_id %s diverges from the persisted result", prior.BatchID, item.refundID)
		}
	}
	return &pb.RefundBatchReply{
		BatchId: prior.BatchID, Status: prior.Status, RefundedCent: prior.RefundedCent,
		ItemResults: refundBatchResultsToPB(stored),
	}, nil
}

// normalizeRefundBatch validates the whole request before any database work.
// A malformed batch must never reach the transaction, and Σ items must equal
// the caller-declared expected_total_cent.
func normalizeRefundBatch(req *pb.RefundBatchRequest) (paymentNo, batchID, reason string, items []refundBatchItem, err error) {
	paymentNo = strings.TrimSpace(req.GetPaymentNo())
	if paymentNo == "" {
		return "", "", "", nil, refundBatchError(apperr.CodeInvalidArgument, refundBatchReasonInvalid,
			"payment_no is required")
	}
	if len(paymentNo) > paymentNoMax {
		return "", "", "", nil, refundBatchError(apperr.CodeInvalidArgument, refundBatchReasonInvalid,
			"payment_no exceeds %d characters", paymentNoMax)
	}
	batchID = strings.TrimSpace(req.GetBatchId())
	if batchID == "" {
		return "", "", "", nil, refundBatchError(apperr.CodeInvalidArgument, refundBatchReasonInvalid,
			"batch_id is required")
	}
	if len(batchID) > batchIDMax {
		return "", "", "", nil, refundBatchError(apperr.CodeInvalidArgument, refundBatchReasonInvalid,
			"batch_id exceeds %d characters", batchIDMax)
	}
	rawItems := req.GetItems()
	if len(rawItems) == 0 {
		return "", "", "", nil, refundBatchError(apperr.CodeInvalidArgument, refundBatchReasonInvalid,
			"items must not be empty")
	}
	items = make([]refundBatchItem, 0, len(rawItems))
	seen := make(map[string]struct{}, len(rawItems))
	var sum int64
	for i, raw := range rawItems {
		refundID := raw.GetRefundId()
		if refundID == "" || strings.TrimSpace(refundID) != refundID {
			return "", "", "", nil, refundBatchError(apperr.CodeInvalidArgument, refundBatchReasonInvalid,
				"items[%d].refund_id must be non-empty and must not have surrounding whitespace", i)
		}
		if len(refundID) > refundIDMax {
			return "", "", "", nil, refundBatchError(apperr.CodeInvalidArgument, refundBatchReasonInvalid,
				"items[%d].refund_id exceeds %d characters", i, refundIDMax)
		}
		if _, duplicate := seen[refundID]; duplicate {
			return "", "", "", nil, refundBatchError(apperr.CodeInvalidArgument, refundBatchReasonInvalid,
				"duplicate refund_id %s", refundID)
		}
		seen[refundID] = struct{}{}
		amount := raw.GetAmountCent()
		if amount <= 0 {
			return "", "", "", nil, refundBatchError(apperr.CodeInvalidArgument, refundBatchReasonInvalid,
				"items[%d].amount_cent must be greater than 0", i)
		}
		if sum > maxInt64-amount {
			return "", "", "", nil, refundBatchError(apperr.CodeInvalidArgument, refundBatchReasonInvalid,
				"item amounts overflow int64")
		}
		sum += amount
		items = append(items, refundBatchItem{refundID: refundID, amountCent: amount})
	}
	if expected := req.GetExpectedTotalCent(); expected != sum {
		return "", "", "", nil, refundBatchError(apperr.CodeInvalidArgument, refundBatchReasonSumMismatch,
			"expected_total_cent %d does not equal the item sum %d", expected, sum)
	}
	return paymentNo, batchID, truncateReason(strings.TrimSpace(req.GetReason())), items, nil
}

// refundBatchFingerprint returns a stable SHA-256 over the item set and its
// total. Order is irrelevant: the same items in a different order replay the
// same batch.
func refundBatchFingerprint(items []refundBatchItem) (string, int64) {
	parts := make([]string, 0, len(items))
	var total int64
	for _, item := range items {
		total += item.amountCent
		parts = append(parts, item.refundID+":"+strconv.FormatInt(item.amountCent, 10))
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:]), total
}

func refundBatchResultsToPB(results []refundBatchItemResult) []*pb.RefundBatchItemResult {
	out := make([]*pb.RefundBatchItemResult, 0, len(results))
	for _, result := range results {
		out = append(out, result.toPB())
	}
	return out
}

func refundBatchError(code apperr.Code, reason, format string, args ...any) *apperr.Error {
	return apperr.New(code, reason+": "+fmt.Sprintf(format, args...)).
		WithDetails(map[string]any{"reason": reason})
}

// encodeRefundBatchError preserves domain errors and maps infrastructure
// failures to BATCH_UNAVAILABLE (C-04 §2.7). A transient DB/commit failure must
// be retried by the caller while the batch stayed PAYING; it must never be
// reported as a partial success.
func encodeRefundBatchError(err error) error {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae
	}
	return apperr.Wrap(apperr.CodeUnavailable, refundBatchReasonUnavailable+": refund batch transaction failed", err).
		WithDetails(map[string]any{"reason": refundBatchReasonUnavailable})
}

const maxInt64 = int64(^uint64(0) >> 1)
