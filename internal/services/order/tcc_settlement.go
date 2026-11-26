package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
)

// This file implements Tx1 of the TCC compensation (SPEC-0002 v1.8 AC-51/AC-65,
// ADR-0007 Amendment 1 §4.1.2 step 3, Plan T1.2) and its idempotent resume
// (Plan T1.3).
//
// Tx1 is deliberately short and local: the order row lock, the Σ recheck with
// the snapshot taken outside the transaction, orders.refund_state=REFUNDING
// (CAS), refund_batches(state=FROZEN), one refund_application per active group,
// the permit decisions and, for CONSUMED groups only, the durable remote
// admission plus the quota commitment and its line composition.
//
// Not in this task (and intentionally not called here): AdmitRefund, Payment,
// Fulfillment.CancelIfCreated, order.refund_requested. Those belong to T1.4+;
// leaving them out keeps Tx1 free of remote calls and lets the durable batch be
// resumed after a crash.

const (
	refundBatchSourceTCC = "tcc"
	refundBatchFrozen    = "FROZEN"
	refundBatchAdmitting = "ADMITTING"
	refundBatchAdmitted  = "ADMITTED"
	refundBatchPaying    = "PAYING"
	refundBatchDone      = "DONE"
	refundBatchFailedPre = "PRECHECK_FAILED"
	// Human-owned batch states: an automatic TCC retry never moves them
	// (Plan T1.2-R R-1/m2, ADR §6.1/AC-62).
	refundBatchManualHold      = "MANUAL_HOLD"
	refundBatchFailedPermanent = "FAILED_PERMANENT"

	refundApplicationSourceTCC  = "tcc"
	refundApplicationPending    = "PENDING"
	refundApplicationProcessing = "PROCESSING"
	refundApplicationDone       = "DONE"

	commitmentReserved = "RESERVED"
	commitmentSettled  = "SETTLED"
)

// tccRefundBatch mirrors one refund_batches row for one TCC compensation.
type tccRefundBatch struct {
	BatchID               string
	OrderNo               string
	PaymentNo             string
	AttemptSeq            int
	State                 string
	CapturedSnapshotCent  int64
	RefundedSnapshotCent  int64
	SumGroupCapCent       int64
	RemainingCapturedCent int64
	Version               int64
}

// tccBatchID / tccGroupRefundID are the deterministic identities of ADR §6.1:
// the batch is `tcc:<payment_no>:<attempt_seq>` (new batch only when a failed
// attempt has to be superseded) and the group refund is
// `tcc:<payment_no>:<group_no>` (stable across resumes, so a crash can never
// create a second refund identity).
func tccBatchID(paymentNo string, attemptSeq int) string {
	return fmt.Sprintf("tcc:%s:%d", paymentNo, attemptSeq)
}

func tccGroupRefundID(paymentNo, groupNo string) string {
	return "tcc:" + paymentNo + ":" + groupNo
}

func tccCommitmentPrefix(paymentNo string) string {
	return "tcc:" + paymentNo + ":"
}

func tccPermitAfterSaleNo(paymentNo string) string {
	return truncate("tcc:"+paymentNo, 64)
}

// tccErrPendingCaptured is the defense-in-depth rejection for a captured
// PENDING_PAYMENT order that somehow reached the group refund path. T1.6
// handles that state at the TccCancelMarkPaid entry (CAS->CANCELLED + payment
// command), so this path must never create a batch/permit/application.
const tccErrPendingCaptured = "TCC_PENDING_CAPTURED_GROUP_PATH"

func tccPendingCapturedError(orderNo, paymentNo string, paidCent int64) error {
	return apperr.Failed("captured PENDING_PAYMENT order reached the group refund path; it must be cancelled at the TCC cancel entry").
		WithDetails(map[string]any{
			"tcc_error": tccErrPendingCaptured,
			"order_no":  orderNo, "payment_no": paymentNo, "paid_cent": paidCent,
		})
}

// tccErrCancelledCaptured is the defense-in-depth rejection for a captured
// CANCELLED order that reached the group refund path. T1.6 owns that branch:
// it restocks the reservation and writes the payment-level cancel command.
const tccErrCancelledCaptured = "TCC_CANCELLED_CAPTURED_GROUP_PATH"

func tccCancelledCapturedError(orderNo, paymentNo string, paidCent int64) error {
	return apperr.Failed("captured CANCELLED order reached the group refund path; the payment-level cancel owns it").
		WithDetails(map[string]any{
			"tcc_error": tccErrCancelledCaptured,
			"order_no":  orderNo, "payment_no": paymentNo, "paid_cent": paidCent,
		})
}

// tccErrStatusUnsupported fails closed for lifecycle states a TCC compensation
// must not silently skip (Plan T1.2-R B1: no silent return).
const tccErrStatusUnsupported = "TCC_ORDER_STATUS_UNSUPPORTED"

func tccUnsupportedStatusError(orderNo, paymentNo, status string) error {
	return apperr.Failed("TCC compensation cannot determine a safe refund branch for the order status").
		WithDetails(map[string]any{
			"tcc_error": tccErrStatusUnsupported,
			"order_no":  orderNo, "payment_no": paymentNo, "status": status,
		})
}

// tccErrBatchManual is returned when the durable batch is parked for manual
// handling. T1.2 must never resume MANUAL_HOLD/FAILED_PERMANENT automatically
// (Plan T1.2-R R-1, ADR §6.1/AC-62).
const tccErrBatchManual = "TCC_BATCH_REQUIRES_MANUAL_HANDLING"

func tccBatchManualError(batch *tccRefundBatch) error {
	return apperr.Failed("TCC refund batch requires manual handling; automatic resume is disabled").
		WithDetails(map[string]any{
			"tcc_error": tccErrBatchManual,
			"batch_id":  batch.BatchID, "state": batch.State,
			"order_no": batch.OrderNo, "payment_no": batch.PaymentNo,
		})
}

// tccSumMismatch carries a failed Σ equation out of Tx1 so the audit batch
// (PRECHECK_FAILED) can be written after the transaction rolled back
// (ADR-0007 Amd1 §4.1.2 step 2; Addendum 2 R-1).
type tccSumMismatch struct {
	result *tccPrecheck
	err    error
}

func (e *tccSumMismatch) Error() string { return e.err.Error() }
func (e *tccSumMismatch) Unwrap() error { return e.err }

// tccShipmentFacts is the Tx1-external snapshot of the Fulfillment-owned S6
// shipment facts. Order has no shipment/shipped_at columns, so the AC-51
// "EXCEPTION and shipped_at" already-left-warehouse rule is evaluated from
// this snapshot; Tx1 itself never calls Fulfillment (Addendum 2 R-2).
type tccShipmentFacts struct {
	byGroup map[string]*pb.Shipment
}

// loadTccShipmentFacts reads ListShipments(order_no) OUTSIDE the transaction.
// An unavailable/incomplete/ambiguous answer fails closed before any write:
// guessing "not shipped" would allow a refund that cancels a shipment which
// already left the warehouse.
func (s *Service) loadTccShipmentFacts(ctx context.Context, orderNo string) (*tccShipmentFacts, error) {
	if s.fulfillment == nil {
		return nil, apperr.Unavailable("fulfillment client is required for the TCC refund shipment check", nil)
	}
	reply, err := s.fulfillment.ListShipments(ctx, &pb.ListShipmentsRequest{
		OrderNo: orderNo, Page: 1, PageSize: 100,
	})
	if err != nil {
		return nil, apperr.Unavailable("load shipment facts for TCC refund", err)
	}
	if reply == nil {
		return nil, apperr.Unavailable("shipment facts are missing for TCC refund", nil)
	}
	if reply.GetTotal() > 100 || int64(len(reply.GetShipments())) != reply.GetTotal() {
		return nil, apperr.Failed("shipment facts are incomplete; manual reconciliation required").
			WithDetails(map[string]any{
				"order_no": orderNo, "total": reply.GetTotal(), "returned": len(reply.GetShipments()),
			})
	}
	facts := &tccShipmentFacts{byGroup: make(map[string]*pb.Shipment, len(reply.GetShipments()))}
	for _, shipment := range reply.GetShipments() {
		if shipment == nil {
			return nil, apperr.Failed("shipment facts contain an empty shipment; manual reconciliation required").
				WithDetails(map[string]any{"order_no": orderNo})
		}
		// A cancelled shipment is a dead attempt; it does not describe whether
		// the group left the warehouse.
		if shipment.GetStatus() == "CANCELLED" {
			continue
		}
		groupNo := permitGroup(shipment.GetFulfillmentGroupNo())
		if existing, ok := facts.byGroup[groupNo]; ok {
			return nil, apperr.Failed("multiple live shipments for one fulfillment group; manual reconciliation required").
				WithDetails(map[string]any{
					"order_no": orderNo, "group_no": groupNo,
					"shipment_no": shipment.GetShipmentNo(), "existing_shipment_no": existing.GetShipmentNo(),
				})
		}
		facts.byGroup[groupNo] = shipment
	}
	return facts, nil
}

// shipped applies the AC-51 already-left-warehouse rule (Addendum 2 R-2 §3.2):
// Order group SHIPPED/RECEIVED, or Fulfillment shipment SHIPPED/RECEIVED, or
// exception_state != NONE with shipped_at. Anything ambiguous fails closed.
func (f *tccShipmentFacts) shipped(group tccGroupCap) (bool, error) {
	if isShippedGroupStatus(group.GroupStatus) {
		return true, nil
	}
	if f != nil {
		if shipment, ok := f.byGroup[group.GroupNo]; ok {
			if isShippedGroupStatus(shipment.GetStatus()) {
				return true, nil
			}
			if exception := strings.TrimSpace(shipment.GetExceptionState()); exception != "" && exception != "NONE" {
				if shipment.GetShippedAtUnix() > 0 {
					return true, nil
				}
				return false, apperr.Failed("shipment is in exception without a shipped_at fact; manual reconciliation required").
					WithDetails(map[string]any{
						"group_no": group.GroupNo, "shipment_no": shipment.GetShipmentNo(),
						"status": shipment.GetStatus(), "exception_state": exception,
					})
			}
			return false, nil
		}
	}
	switch group.GroupStatus {
	case "PENDING", statusAllocated:
		return false, nil
	default:
		// No shipment facts and a status we cannot interpret as "not shipped":
		// including "" (unknown projection) this is ambiguous, so fail closed
		// instead of guessing (Review G1). A synthetic legacy G1 derives its
		// status from orders.status, so SHIPPED/COMPLETED is handled above.
		return false, apperr.Failed("order group has no shipment fact and its status is not PENDING/ALLOCATED; manual reconciliation required").
			WithDetails(map[string]any{"group_no": group.GroupNo, "group_status": group.GroupStatus})
	}
}

// applyTccCompensation runs the single Tx1 for a TCC refund. The Payment and
// shipment snapshots were captured outside the transaction; the Σ equation is
// rechecked inside the transaction with that same Payment snapshot
// (ADR §4.1.2 step 3). A Σ mismatch inside Tx1 rolls back first, then records
// the PRECHECK_FAILED audit batch (Addendum 2 R-1).
func (s *Service) applyTccCompensation(ctx context.Context, orderNo string, snapshot *tccPaymentSnapshot, shipments *tccShipmentFacts, paid int64) error {
	if snapshot == nil {
		return apperr.Internal("TCC compensation requires a payment snapshot", nil)
	}
	if shipments == nil {
		return apperr.Internal("TCC compensation requires a shipment snapshot", nil)
	}
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		return s.freezeTccCompensationTx(ctx, tx, orderNo, snapshot, shipments, paid)
	})
	if err == nil {
		return nil
	}
	var mismatch *tccSumMismatch
	if errors.As(err, &mismatch) {
		if auditErr := s.recordTccPrecheckFailure(ctx, orderNo, mismatch.result); auditErr != nil {
			s.log.Error("record tcc precheck failure audit batch", "order_no", orderNo, "error", auditErr)
		}
		return mismatch.err
	}
	return err
}

// freezeTccCompensationTx is Tx1: lock, recheck, write REFUNDING/batch/
// applications/permits, or resume a batch that a previous run already froze.
func (s *Service) freezeTccCompensationTx(ctx context.Context, tx *sql.Tx, orderNo string, snapshot *tccPaymentSnapshot, shipments *tccShipmentFacts, paid int64) error {
	locked, err := findOrderForUpdate(ctx, tx, orderNo)
	if err != nil {
		return apperr.Internal("lock order for tcc compensation", err)
	}
	if locked == nil {
		return apperr.NotFound("order not found")
	}
	if locked.RefundState == refundStateFull {
		return nil
	}
	switch locked.Status {
	case statusPaid, statusAllocated, statusShipped, statusCompleted:
		// Captured money is still refundable after the lifecycle advanced.
	case statusCancelled:
		// T1.6 handles CANCELLED + captured before this ledger starts. If it
		// is reached here anyway, fail closed instead of reporting a completed
		// compensation (defense in depth, Plan T1.2-R B1).
		if snapshot.CapturedCent > 0 {
			return tccCancelledCapturedError(locked.OrderNo, snapshot.PaymentNo, snapshot.CapturedCent)
		}
		return nil
	case statusPendingPayment:
		// T1.6 cancels a captured PENDING_PAYMENT order at the entry (CAS +
		// payment command). This ledger must never write PAID/REFUNDING/batch/
		// permit/application for that state (defense in depth, Addendum 2 R-3).
		if snapshot.CapturedCent > 0 {
			return tccPendingCapturedError(locked.OrderNo, snapshot.PaymentNo, snapshot.CapturedCent)
		}
		// No captured money: the local-cancel branch is decided before Tx1.
		return apperr.Failed("PENDING_PAYMENT has no group refund branch").
			WithDetails(map[string]any{"order_no": locked.OrderNo, "payment_no": snapshot.PaymentNo})
	default:
		return tccUnsupportedStatusError(locked.OrderNo, snapshot.PaymentNo, locked.Status)
	}

	batch, completed, err := findTccBatchForResume(ctx, tx, orderNo, snapshot.PaymentNo)
	if err != nil {
		return err
	}
	if completed {
		return nil
	}

	if batch == nil {
		return s.createTccCompensationTx(ctx, tx, locked, snapshot, shipments, paid)
	}
	return s.resumeTccCompensationTx(ctx, tx, locked, snapshot, shipments, batch)
}

// createTccCompensationTx is the fresh Tx1 path: strict Σ recheck (no own
// commitments exist yet), REFUNDING CAS, batch, applications and permits.
func (s *Service) createTccCompensationTx(ctx context.Context, tx *sql.Tx, locked *orderRecord, snapshot *tccPaymentSnapshot, shipments *tccShipmentFacts, paid int64) error {
	rechecked, err := s.recheckTccSumLocked(ctx, tx, locked, snapshot)
	if err != nil {
		if rechecked != nil {
			// Σ equation failure before any order/permit write: roll back and
			// let the caller record the PRECHECK_FAILED audit batch.
			return &tccSumMismatch{result: rechecked, err: err}
		}
		return err
	}
	// Plan T1.2-R M2: the Σ equation above must be evaluated BEFORE the
	// no-active shortcut. With Σ == remaining == 0 there is nothing to refund:
	// idempotent success, no REFUNDING write, no empty FROZEN batch.
	if rechecked.SumGroupCapCent == 0 && rechecked.RemainingCapturedCent == 0 {
		return nil
	}
	if rechecked.NoCaptured {
		// AC-65: no captured money is a cancel/zero-converge branch, never a
		// per-group refund. The T1.6 entry cancels that state before this
		// ledger; reaching it here is fail-closed instead of silently
		// skipping the compensation.
		return apperr.Failed("TCC compensation has no captured amount").
			WithDetails(map[string]any{
				"order_no": locked.OrderNo, "payment_no": snapshot.PaymentNo,
				"pay_cent": rechecked.PayCent, "captured_cent": rechecked.CapturedCent,
			})
	}
	if err := markOrderRefundingTx(ctx, tx, locked, snapshot.PaymentNo, paid); err != nil {
		return err
	}
	batch, err := insertTccBatchTx(ctx, tx, locked.OrderNo, snapshot, rechecked)
	if err != nil {
		return err
	}
	return s.writeTccBatchGroupsTx(ctx, tx, batch, snapshot, shipments, rechecked.Groups, "")
}

// resumeTccCompensationTx is Tx1 for a rerun: the batch already froze the
// compensation, so only missing groups are filled. Before admission the Σ
// recheck ignores this compensation's own RESERVED commitments (they are the
// quota being resumed) and must still match the batch snapshot; after admission
// (ADMITTED/PAYING) the payment authority gates the remaining amounts instead
// (see resumeTccRecheckTx).
func (s *Service) resumeTccCompensationTx(ctx context.Context, tx *sql.Tx, locked *orderRecord, snapshot *tccPaymentSnapshot, shipments *tccShipmentFacts, batch *tccRefundBatch) error {
	if locked.RefundState != refundStateRefunding {
		// A batch only exists after Tx1 wrote REFUNDING in the same
		// transaction. Seeing it without REFUNDING is manual damage.
		return apperr.Failed("refund batch exists but the order is not REFUNDING; manual reconciliation required").
			WithDetails(map[string]any{
				"order_no": locked.OrderNo, "batch_id": batch.BatchID,
				"refund_state": locked.RefundState, "batch_state": batch.State,
			})
	}
	rechecked, err := s.resumeTccRecheckTx(ctx, tx, locked, snapshot, batch)
	if err != nil {
		return err
	}
	if rechecked.SumGroupCapCent == 0 && rechecked.RemainingCapturedCent == 0 && rechecked.ActiveGroupCount == 0 {
		// M2: an existing ledger that is now fully refunded/zero-cap has
		// nothing to fill; idempotent success without an empty batch write.
		return nil
	}
	return s.writeTccBatchGroupsTx(ctx, tx, batch, snapshot, shipments, rechecked.Groups, tccCommitmentPrefix(snapshot.PaymentNo))
}

// resumeTccRecheckTx re-derives the Σ view used to fill missing resume rows.
//
// FROZEN/ADMITTING batches still gate on the strict Σ equality and the frozen
// snapshot (nothing has been paid yet). An ADMITTED/PAYING batch is already past
// admission: its group amounts are frozen and Payment is executing them. There
// the strict equality would fail spuriously as soon as Payment has refunded a
// group but payment.refunded has not been projected yet, and it would also fail
// permanently for groups whose commitment already SETTLED. ADR §4.1.3(5) makes
// the Payment authority per-call cap check the gate for this phase: a definitive
// rejection parks the batch in MANUAL_HOLD (tcc_payment.go), so a changed
// snapshot can never silently over-refund.
func (s *Service) resumeTccRecheckTx(ctx context.Context, tx *sql.Tx, locked *orderRecord, snapshot *tccPaymentSnapshot, batch *tccRefundBatch) (*tccPrecheck, error) {
	excludePrefix := tccCommitmentPrefix(snapshot.PaymentNo)
	if batch.State == refundBatchAdmitted || batch.State == refundBatchPaying {
		rechecked, err := computeTccPrecheckExcluding(ctx, tx, locked, snapshot, excludePrefix)
		if err != nil {
			return nil, err
		}
		if rechecked.SumGroupCapCent == 0 && rechecked.ActiveGroupCount == 0 && rechecked.RemainingCapturedCent > 0 {
			return nil, apperr.Failed("tcc refund has no active group but captured money remains; manual reconciliation required").
				WithDetails(map[string]any{
					"order_no": locked.OrderNo, "batch_id": batch.BatchID,
					"remaining_captured_cent": rechecked.RemainingCapturedCent,
				})
		}
		return rechecked, nil
	}
	rechecked, err := s.recheckTccSumTxExcluding(ctx, tx, locked.OrderNo, snapshot, excludePrefix)
	if err != nil {
		if rechecked != nil {
			// Plan T1.2-R R-4 (m1): a resume Σ failure is audited exactly like
			// a fresh precheck failure; the transaction rolls back first.
			return nil, &tccSumMismatch{result: rechecked, err: err}
		}
		return nil, err
	}
	if rechecked.SumGroupCapCent != batch.SumGroupCapCent || rechecked.RemainingCapturedCent != batch.RemainingCapturedCent {
		// AC-65: the authoritative captured/refunded facts changed since the
		// batch froze. Do not extend it by guessing; record the audit batch
		// (R-4) and stop for manual review (T1.4 owns MANUAL_HOLD).
		return nil, &tccSumMismatch{result: rechecked, err: apperr.Failed("TCC compensation snapshot changed since the batch was frozen").
			WithDetails(map[string]any{
				"order_no": locked.OrderNo, "batch_id": batch.BatchID, "payment_no": snapshot.PaymentNo,
				"batch_sum_group_cap_cent":      batch.SumGroupCapCent,
				"recomputed_sum_group_cap_cent": rechecked.SumGroupCapCent,
				"batch_remaining_captured_cent": batch.RemainingCapturedCent,
				"recomputed_remaining_captured": rechecked.RemainingCapturedCent,
				"recomputed_active_group_count": rechecked.ActiveGroupCount,
				"recomputed_refunded_captured":  rechecked.RefundedCent,
			})}
	}
	return rechecked, nil
}

// resumeTccCancelledOrder converges a concurrent TCC cancel: if another caller
// already froze the compensation (REFUNDING) between this call's initial read
// and its precheck, resume that batch instead of returning a spurious Σ
// mismatch. It reports converged=false when the order is not REFUNDING, so the
// caller's original precheck error stands.
func (s *Service) resumeTccCancelledOrder(ctx context.Context, orderNo, paymentNo string, paid int64) (bool, error) {
	latest, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return false, err
	}
	if latest == nil || latest.RefundState != refundStateRefunding {
		return false, nil
	}
	snapshot, err := s.readTccPaymentSnapshot(ctx, latest, paymentNo)
	if err != nil {
		return true, err
	}
	shipments, err := s.loadTccShipmentFacts(ctx, orderNo)
	if err != nil {
		return true, err
	}
	return true, s.applyTccCompensation(ctx, orderNo, snapshot, shipments, paid)
}

// recordTccPrecheckFailure writes the PRECHECK_FAILED audit batch after a Σ
// mismatch (ADR Amd1 §4.1.2 step 2; Addendum 2 R-1). It writes ONLY that row:
// no order/permit/commitment/application/outbox/status-log write, and it never
// replaces an existing audit row (a new attempt_seq is allocated).
func (s *Service) recordTccPrecheckFailure(ctx context.Context, orderNo string, result *tccPrecheck) error {
	if result == nil {
		return apperr.Internal("TCC precheck failure audit requires the computed numbers", nil)
	}
	paymentNo := strings.TrimSpace(result.PaymentNo)
	for attempt := 0; attempt < 3; attempt++ {
		var maxSeq int
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(attempt_seq),0) FROM refund_batches
			WHERE source = ? AND order_no = ? AND payment_no = ?`,
			refundBatchSourceTCC, orderNo, paymentNo).Scan(&maxSeq); err != nil {
			return apperr.Internal("allocate tcc precheck failure attempt", err)
		}
		batchID := tccBatchID(paymentNo, maxSeq+1)
		_, err := s.db.ExecContext(ctx, `INSERT INTO refund_batches
			(batch_id, source, order_no, payment_no, attempt_seq, state, captured_snapshot_cent,
			 refunded_snapshot_cent, sum_group_cap_cent, remaining_captured_cent, attempts, version)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 1)`,
			batchID, refundBatchSourceTCC, orderNo, paymentNo, maxSeq+1, refundBatchFailedPre,
			result.CapturedCent, result.RefundedCent, result.SumGroupCapCent, result.RemainingCapturedCent)
		if err == nil {
			s.log.Warn("payment tcc refund precheck failed; audit batch recorded",
				"order_no", orderNo, "payment_no", paymentNo, "batch_id", batchID,
				"sum_group_cap_cent", result.SumGroupCapCent,
				"remaining_captured_cent", result.RemainingCapturedCent,
				"diff_cent", result.DiffCent())
			return nil
		}
		if isDuplicateKey(err) {
			continue
		}
		return apperr.Internal("record tcc precheck failure audit batch", err)
	}
	return apperr.Internal("record tcc precheck failure audit batch: attempt sequence contention", nil)
}

// markOrderRefundingTx is the refund-state CAS of ADR §4.1.2 step 3. A captured
// PENDING_PAYMENT order is cancelled by T1.6 before this function; reaching it
// here is a fail-closed invariant violation. An already REFUNDING order is left
// untouched so a resume never rewrites lifecycle history.
func markOrderRefundingTx(ctx context.Context, tx *sql.Tx, locked *orderRecord, paymentNo string, paid int64) error {
	if locked.RefundState == refundStateRefunding {
		return nil
	}
	if locked.Status == statusPendingPayment {
		return tccPendingCapturedError(locked.OrderNo, paymentNo, paid)
	}
	res, err := tx.ExecContext(ctx, `UPDATE orders
		SET refund_state = ?, version = version + 1, updated_at = ?
		WHERE order_no = ? AND refund_state <> ?`,
		refundStateRefunding, time.Now(), locked.OrderNo, refundStateFull)
	if err != nil {
		return apperr.Internal("mark order refunding for tcc compensation", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.Conflict("order could not be moved to REFUNDING for tcc compensation").
			WithDetails(map[string]any{"order_no": locked.OrderNo, "refund_state": locked.RefundState})
	}
	return nil
}

// findTccBatchForResume returns the batch a rerun must continue (T1.3) and
// whether a previous attempt already completed. Multiple resumable batches for
// one (order, payment) violate ADR §6.1 and fail closed.
func findTccBatchForResume(ctx context.Context, tx *sql.Tx, orderNo, paymentNo string) (*tccRefundBatch, bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT batch_id, order_no, payment_no, attempt_seq, state,
		captured_snapshot_cent, refunded_snapshot_cent, sum_group_cap_cent, remaining_captured_cent, version
		FROM refund_batches
		WHERE source = ? AND order_no = ? AND payment_no = ?
		ORDER BY attempt_seq DESC FOR UPDATE`, refundBatchSourceTCC, orderNo, paymentNo)
	if err != nil {
		return nil, false, apperr.Internal("load tcc refund batches", err)
	}
	var resumable []*tccRefundBatch
	completed := false
	for rows.Next() {
		batch := &tccRefundBatch{}
		if err := rows.Scan(&batch.BatchID, &batch.OrderNo, &batch.PaymentNo, &batch.AttemptSeq,
			&batch.State, &batch.CapturedSnapshotCent, &batch.RefundedSnapshotCent,
			&batch.SumGroupCapCent, &batch.RemainingCapturedCent, &batch.Version); err != nil {
			_ = rows.Close()
			return nil, false, apperr.Internal("scan tcc refund batch", err)
		}
		switch batch.State {
		case refundBatchFrozen, refundBatchAdmitting, refundBatchAdmitted, refundBatchPaying:
			resumable = append(resumable, batch)
		case refundBatchManualHold, refundBatchFailedPermanent:
			// Plan T1.2-R R-1/m2: human-owned states must not be resumed by an
			// automatic TCC retry; T1.4 owns their state CAS.
			_ = rows.Close()
			return nil, false, tccBatchManualError(batch)
		case refundBatchDone:
			completed = true
		case refundBatchFailedPre:
			// Audit only, no applications (ADR §6.1): a later attempt may be
			// created, but a precheck-failed row is never resumed.
		default:
			_ = rows.Close()
			return nil, false, apperr.Failed("unknown tcc refund batch state").
				WithDetails(map[string]any{"batch_id": batch.BatchID, "state": batch.State})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, false, apperr.Internal("iterate tcc refund batches", err)
	}
	if err := rows.Close(); err != nil {
		return nil, false, apperr.Internal("close tcc refund batches", err)
	}
	if len(resumable) > 1 {
		return nil, false, apperr.Failed("multiple active TCC refund batches; manual reconciliation required").
			WithDetails(map[string]any{
				"order_no": orderNo, "payment_no": paymentNo,
				"active_batches": len(resumable), "latest_batch": resumable[0].BatchID,
			})
	}
	if len(resumable) == 1 {
		return resumable[0], false, nil
	}
	return nil, completed, nil
}

// insertTccBatchTx creates the FROZEN batch. The audit numbers are the Tx1
// calculation: a new attempt only exists after the Σ equation held.
func insertTccBatchTx(ctx context.Context, tx *sql.Tx, orderNo string, snapshot *tccPaymentSnapshot, pre *tccPrecheck) (*tccRefundBatch, error) {
	var maxSeq int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(attempt_seq),0) FROM refund_batches
		WHERE source = ? AND order_no = ? AND payment_no = ?`,
		refundBatchSourceTCC, orderNo, snapshot.PaymentNo).Scan(&maxSeq); err != nil {
		return nil, apperr.Internal("allocate tcc refund batch attempt", err)
	}
	batch := &tccRefundBatch{
		BatchID:               tccBatchID(snapshot.PaymentNo, maxSeq+1),
		OrderNo:               orderNo,
		PaymentNo:             snapshot.PaymentNo,
		AttemptSeq:            maxSeq + 1,
		State:                 refundBatchFrozen,
		CapturedSnapshotCent:  pre.CapturedCent,
		RefundedSnapshotCent:  pre.RefundedCent,
		SumGroupCapCent:       pre.SumGroupCapCent,
		RemainingCapturedCent: pre.RemainingCapturedCent,
		Version:               1,
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO refund_batches
		(batch_id, source, order_no, payment_no, attempt_seq, state, captured_snapshot_cent,
		 refunded_snapshot_cent, sum_group_cap_cent, remaining_captured_cent, attempts, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 1)`,
		batch.BatchID, refundBatchSourceTCC, orderNo, snapshot.PaymentNo, batch.AttemptSeq, refundBatchFrozen,
		batch.CapturedSnapshotCent, batch.RefundedSnapshotCent, batch.SumGroupCapCent, batch.RemainingCapturedCent); err != nil {
		return nil, apperr.Internal("insert tcc refund batch", err)
	}
	return batch, nil
}

// writeTccBatchGroupsTx materializes the per-group ledger for the batch. Every
// write is idempotent, so a resume fills only what a crash left missing:
//
//	NONE  permit        -> CAS NONE->REVOKED, application(PRE_SHIP_REVOKED)
//	CONSUMED or CREATED -> application(PENDING_REMOTE) + admission(PENDING_REMOTE)
//	                       + RESERVED commitment with line composition
//	shipped / received  -> application(POST_SHIP_ALLOWED), permit untouched
//
// CONSUMED never writes REVOKED (ADR §4.1.3).
func (s *Service) writeTccBatchGroupsTx(ctx context.Context, tx *sql.Tx, batch *tccRefundBatch, snapshot *tccPaymentSnapshot, shipments *tccShipmentFacts, groups []tccGroupCap, excludeRefundPrefix string) error {
	if batch.State == refundBatchManualHold {
		// AC-71/W12: a human-held batch is never pushed by an automatic retry.
		// Return before any write so the resume changes zero rows.
		return tccBatchManualError(batch)
	}
	composition, err := loadTccGroupComposition(ctx, tx, batch.OrderNo, excludeRefundPrefix)
	if err != nil {
		return err
	}
	linesByGroup, err := loadTccLinesByGroup(ctx, tx, batch.OrderNo)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if !group.Active {
			continue
		}
		comp := composition[group.GroupNo]
		if comp == nil {
			comp = &tccGroupComposition{}
		}
		// Plan T1.2-R M1: for historical/pre-line-composition commitments
		// (composition_known=0) the group cap is
		// groupPaid + freight_share - RESERVED/SETTLED (the precheck value),
		// never the old commitment amount. Only a whole-group refund is allowed;
		// line attribution must not be invented.
		var goodsCap, freightCap int64
		if comp.UnknownComposition {
			goodsCap = group.PaidCent - comp.CommittedAmount
			freightCap = group.FreightCent
		} else {
			goodsCap = group.PaidCent - comp.CommittedGoods
			freightCap = group.FreightCent - comp.CommittedFreight
		}
		if goodsCap < 0 || freightCap < 0 || goodsCap+freightCap != group.CapCent {
			return apperr.Failed("tcc group composition does not match the prechecked cap").
				WithDetails(map[string]any{
					"order_no": batch.OrderNo, "group_no": group.GroupNo, "batch_id": batch.BatchID,
					"goods_cap_cent": goodsCap, "freight_cap_cent": freightCap,
					"group_cap_cent":        group.CapCent,
					"composition_known":     !comp.UnknownComposition,
					"committed_amount_cent": comp.CommittedAmount,
					"committed_goods_cent":  comp.CommittedGoods, "committed_freight_cent": comp.CommittedFreight,
				})
		}

		refundID := tccGroupRefundID(snapshot.PaymentNo, group.GroupNo)
		shipped, err := shipments.shipped(group)
		if err != nil {
			return err
		}
		admissionResult := refundResultPre
		consumed := false
		if shipped {
			// Already left the warehouse (including EXCEPTION + shipped_at):
			// refund-only POST path. Never revoke the permit or reserve a new
			// commitment here (Addendum 2 R-2 §3.3).
			admissionResult = refundResultPost
		} else {
			result, remote, err := decideTccPermitTx(ctx, tx, batch.OrderNo, group.GroupNo, snapshot.PaymentNo)
			if err != nil {
				return err
			}
			admissionResult, consumed = result, remote
		}

		if _, err := tx.ExecContext(ctx, `UPDATE order_fulfillment_groups
			SET refund_state = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND group_no = ? AND refund_state <> ?`,
			refundStateRefunding, batch.OrderNo, group.GroupNo, groupRefundStateRefunded); err != nil {
			return apperr.Internal("mark group refunding for tcc compensation", err)
		}

		done, err := ensureTccApplicationTx(ctx, tx, batch, refundID, group, goodsCap, freightCap, admissionResult)
		if err != nil {
			return err
		}
		if done {
			// The group's refund application already completed; never re-open it.
			continue
		}
		if !consumed {
			continue
		}
		if err := ensureTccRemoteAdmissionTx(ctx, tx, batch.OrderNo, group.GroupNo, refundID, group.CapCent); err != nil {
			return err
		}
		if err := ensureTccCommitmentTx(ctx, tx, batch.OrderNo, group.GroupNo, refundID,
			group.CapCent, !comp.UnknownComposition, goodsCap, freightCap, comp, linesByGroup[group.GroupNo]); err != nil {
			return err
		}
	}
	return nil
}

// decideTccPermitTx reads the group permit and applies the AC-50/A-65 permit
// branch. It returns the durable admission result and whether the group takes
// the PENDING_REMOTE (CONSUMED) two-phase path.
//
// T1.4 boundary: the remote Fulfillment.CancelIfCreated call is deliberately NOT
// made here; Tx1 only persists the decision. When T1.4 lands, a shipment with
// exception_state != NONE and shipped_at must be answered as ALREADY_SHIPPED
// (Addendum 2 R-2 §3.2.5), never as an endless TRANSIENT_ERROR retry.
func decideTccPermitTx(ctx context.Context, tx *sql.Tx, orderNo, groupNo, paymentNo string) (string, bool, error) {
	permit, err := ensurePermitTx(ctx, tx, orderNo, groupNo)
	if err != nil {
		return "", false, err
	}
	switch permit.state {
	case permitNone:
		res, err := tx.ExecContext(ctx, `UPDATE order_shipping_permits
			SET state = ?, after_sale_no = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND fulfillment_group_no = ? AND state = ?`,
			permitRevoked, tccPermitAfterSaleNo(paymentNo), orderNo, groupNo, permitNone)
		if err != nil {
			return "", false, apperr.Internal("revoke shipping permit for tcc refund", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return refundResultPre, false, nil
		}
		// Lost the CAS (a concurrent Ship reached the permit). Converge by
		// re-reading, exactly like AC-50: a consumed permit takes the remote
		// path, a revoked one is idempotent.
		permit, err = ensurePermitTx(ctx, tx, orderNo, groupNo)
		if err != nil {
			return "", false, err
		}
		switch permit.state {
		case permitRevoked:
			return refundResultPre, false, nil
		case permitConsumed:
			return refundPending, true, nil
		default:
			return "", false, apperr.Failed("shipping permit state is unknown for tcc refund").
				WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo, "permit_state": permit.state})
		}
	case permitConsumed:
		return refundPending, true, nil
	case permitRevoked:
		return refundResultPre, false, nil
	default:
		// D-29: GRANTED and any other value are not legal persisted states.
		return "", false, apperr.Failed("shipping permit state is not a legal persisted state").
			WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo, "permit_state": permit.state})
	}
}

func isShippedGroupStatus(status string) bool {
	return status == "SHIPPED" || status == "RECEIVED"
}

// ensureTccApplicationTx inserts the durable per-group refund intent and
// verifies an existing one matches the batch (idempotent resume). It reports
// done=true when the application already completed (DONE), so the group is
// skipped and never refunded twice (AC-65).
func ensureTccApplicationTx(ctx context.Context, tx *sql.Tx, batch *tccRefundBatch, refundID string, group tccGroupCap, goodsCap, freightCap int64, admissionResult string) (bool, error) {
	if _, err := tx.ExecContext(ctx, `INSERT INTO refund_applications
		(refund_id, batch_id, source, order_no, group_no, non_effective_capture,
		 amount_cent, goods_cent, freight_cent, state, admission_result, version)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, 1)
		ON DUPLICATE KEY UPDATE refund_id = refund_id`,
		refundID, batch.BatchID, refundApplicationSourceTCC, batch.OrderNo, group.GroupNo,
		group.CapCent, goodsCap, freightCap, refundApplicationPending, admissionResult); err != nil {
		return false, apperr.Internal("insert tcc refund application", err)
	}
	var (
		existingBatch, existingGroup, existingState, existingResult string
		amount, goods, freight                                      int64
		nonEffective                                                int
	)
	if err := tx.QueryRowContext(ctx, `SELECT batch_id, group_no, amount_cent, goods_cent, freight_cent,
		state, admission_result, non_effective_capture
		FROM refund_applications WHERE refund_id = ? FOR UPDATE`, refundID).
		Scan(&existingBatch, &existingGroup, &amount, &goods, &freight,
			&existingState, &existingResult, &nonEffective); err != nil {
		return false, apperr.Internal("load tcc refund application", err)
	}
	if existingBatch != batch.BatchID || existingGroup != group.GroupNo ||
		amount != group.CapCent || goods != goodsCap || freight != freightCap || nonEffective != 0 {
		return false, apperr.Failed("tcc refund application conflicts with the prechecked group").
			WithDetails(map[string]any{
				"order_no": batch.OrderNo, "group_no": group.GroupNo, "refund_id": refundID,
				"batch_id": existingBatch, "amount_cent": amount, "goods_cent": goods, "freight_cent": freight,
				"want_amount_cent": group.CapCent, "want_goods_cent": goodsCap, "want_freight_cent": freightCap,
			})
	}
	switch existingState {
	case refundApplicationDone:
		return true, nil
	case refundApplicationPending, refundApplicationProcessing:
		// PENDING is the Tx1 state; PROCESSING is the T1.4a admitted state that
		// waits for the T1.4b payment. Both are idempotently resumable.
	default:
		return false, apperr.Failed("tcc refund application is not resumable in its current state").
			WithDetails(map[string]any{
				"order_no": batch.OrderNo, "group_no": group.GroupNo,
				"refund_id": refundID, "state": existingState,
			})
	}
	// admission_result is monotonic: a later resume may observe the resolved
	// PRE/POST result written by the admission phase; it must never go back.
	if existingResult != "" && existingResult != admissionResult &&
		existingResult != refundResultPre && existingResult != refundResultPost &&
		existingResult != refundReviewRequired {
		return false, apperr.Failed("tcc refund application admission result conflicts with the permit decision").
			WithDetails(map[string]any{
				"order_no": batch.OrderNo, "group_no": group.GroupNo, "refund_id": refundID,
				"admission_result": existingResult, "want_admission_result": admissionResult,
			})
	}
	return false, nil
}

// ensureTccRemoteAdmissionTx persists the PENDING_REMOTE admission before any
// remote cancel/payment is attempted (ADR §4.1.2 step 3). An already resolved
// admission is left untouched and is never overwritten (T1.4 owns Tx2).
func ensureTccRemoteAdmissionTx(ctx context.Context, tx *sql.Tx, orderNo, groupNo, refundID string, amount int64) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_admissions
		(refund_id, order_no, group_no, result, state, amount_cent, version)
		VALUES (?, ?, ?, ?, ?, ?, 1)
		ON DUPLICATE KEY UPDATE refund_id = refund_id`,
		refundID, orderNo, groupNo, refundPending, refundPending, amount); err != nil {
		return apperr.Internal("insert tcc refund admission", err)
	}
	var existingOrder, existingGroup, result, state string
	var existingAmount int64
	if err := tx.QueryRowContext(ctx, `SELECT order_no, group_no, result, state, amount_cent
		FROM order_refund_admissions WHERE refund_id = ? FOR UPDATE`, refundID).
		Scan(&existingOrder, &existingGroup, &result, &state, &existingAmount); err != nil {
		return apperr.Internal("load tcc refund admission", err)
	}
	if existingOrder != orderNo || existingGroup != groupNo || existingAmount != amount {
		return apperr.Failed("tcc refund admission conflicts with the batch").
			WithDetails(map[string]any{
				"order_no": orderNo, "group_no": groupNo, "refund_id": refundID,
				"admission_amount_cent": existingAmount, "want_amount_cent": amount,
			})
	}
	switch result {
	case refundPending, refundResultPre, refundResultPost, refundReviewRequired:
		// REVIEW_REQUIRED is a legal persisted decision (AC-71): it must not be
		// judged illegal or fail before the admission phase can read it back.
		return nil
	default:
		return apperr.Failed("tcc refund admission result is not a legal value").
			WithDetails(map[string]any{"refund_id": refundID, "result": result, "state": state})
	}
}

// ensureTccCommitmentTx reserves the group's quota before the remote admission
// (AC-64/INV-24). A composition_known=1 commitment is written with its line
// composition and verified; a composition_known=0 (historical/pre-line) group
// gets only the quota header with no lines, because Plan T1.2-R M1 forbids
// guessing a line attribution for a whole-group TCC refund.
func ensureTccCommitmentTx(ctx context.Context, tx *sql.Tx, orderNo, groupNo, refundID string,
	amount int64, known bool, goodsCap, freightCap int64, comp *tccGroupComposition, lines []tccOrderLine) error {
	headerGoods, headerFreight, knownValue := goodsCap, freightCap, 1
	var expected []tccCommitmentLine
	if known {
		var err error
		expected, err = buildTccCommitmentLines(comp, lines, headerGoods, headerFreight)
		if err != nil {
			return err
		}
	} else {
		// Migration/ADR convention for an unattributed legacy commitment:
		// goods=amount, freight=0, composition_known=0, no lines.
		headerGoods, headerFreight, knownValue = amount, 0, 0
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_commitments
		(order_no, group_no, refund_id, amount_cent, goods_cent, freight_cent,
		 composition_known, composition_source, state, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON DUPLICATE KEY UPDATE refund_id = refund_id`,
		orderNo, groupNo, refundID, amount, headerGoods, headerFreight,
		knownValue, refundApplicationSourceTCC, commitmentReserved); err != nil {
		return apperr.Internal("insert tcc refund commitment", err)
	}
	var (
		existingAmount, existingGoods, existingFreight int64
		compositionKnown                               int
		state, source                                  string
	)
	if err := tx.QueryRowContext(ctx, `SELECT amount_cent, goods_cent, freight_cent, composition_known, state, composition_source
		FROM order_refund_commitments WHERE refund_id = ? FOR UPDATE`, refundID).
		Scan(&existingAmount, &existingGoods, &existingFreight, &compositionKnown, &state, &source); err != nil {
		return apperr.Internal("load tcc refund commitment", err)
	}
	if existingAmount != amount || compositionKnown != knownValue {
		return apperr.Failed("tcc refund commitment conflicts with the prechecked composition").
			WithDetails(map[string]any{
				"order_no": orderNo, "group_no": groupNo, "refund_id": refundID,
				"amount_cent": existingAmount, "goods_cent": existingGoods, "freight_cent": existingFreight,
				"composition_known": compositionKnown,
				"want_amount_cent":  amount, "want_composition_known": knownValue,
			})
	}
	if known && (existingGoods != headerGoods || existingFreight != headerFreight) {
		return apperr.Failed("tcc refund commitment conflicts with the prechecked composition").
			WithDetails(map[string]any{
				"order_no": orderNo, "group_no": groupNo, "refund_id": refundID,
				"goods_cent": existingGoods, "freight_cent": existingFreight,
				"want_goods_cent": headerGoods, "want_freight_cent": headerFreight,
			})
	}
	if state != commitmentReserved && state != commitmentSettled {
		return apperr.Failed("tcc refund commitment is not resumable in its current state").
			WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo, "refund_id": refundID, "state": state})
	}
	if !known {
		// No line rows by design; a legacy whole-group refund is represented by
		// the quota header only (ADR §4.3).
		return nil
	}
	for _, line := range expected {
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_commitment_lines
			(refund_id, line_no, quantity, goods_cent, freight_cent)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE line_no = line_no`,
			refundID, line.LineNo, line.Quantity, line.GoodsCent, line.FreightCent); err != nil {
			return apperr.Internal("insert tcc commitment line", err)
		}
	}
	stored, err := loadTccCommitmentLines(ctx, tx, refundID)
	if err != nil {
		return err
	}
	if !equalTccCommitmentLines(expected, stored) {
		return apperr.Failed("tcc commitment line composition does not match the commitment header").
			WithDetails(map[string]any{
				"order_no": orderNo, "group_no": groupNo, "refund_id": refundID,
				"expected_lines": expected, "stored_lines": stored,
			})
	}
	return nil
}

type tccCommitmentLine struct {
	LineNo      int
	Quantity    int64
	GoodsCent   int64
	FreightCent int64
}

// buildTccCommitmentLines derives the new commitment's line composition from
// the order lines minus the lines already attributed to RESERVED/SETTLED
// commitments. A legacy commitment without lines cannot be attributed per line,
// so it fails closed instead of guessing (ADR §4.3).
func buildTccCommitmentLines(comp *tccGroupComposition, lines []tccOrderLine, goodsCap, freightCap int64) ([]tccCommitmentLine, error) {
	if comp == nil {
		comp = &tccGroupComposition{}
	}
	expected := make([]tccCommitmentLine, 0, len(lines)+1)
	var goodsTotal int64
	for _, line := range lines {
		lineNo := int(line.LineNo)
		committedGoods := comp.LineGoods[lineNo]
		committedQuantity := comp.LineQuantity[lineNo]
		goodsRemaining := line.LinePaidCent - committedGoods
		quantityRemaining := int64(line.Quantity) - committedQuantity
		if goodsRemaining < 0 || quantityRemaining < 0 {
			return nil, apperr.Failed("tcc commitment line is already over-committed").
				WithDetails(map[string]any{
					"line_no": lineNo, "line_paid_cent": line.LinePaidCent,
					"committed_goods_cent": committedGoods, "quantity": line.Quantity,
					"committed_quantity": committedQuantity,
				})
		}
		if goodsRemaining == 0 && quantityRemaining == 0 {
			continue
		}
		expected = append(expected, tccCommitmentLine{
			LineNo: lineNo, Quantity: quantityRemaining, GoodsCent: goodsRemaining,
		})
		goodsTotal += goodsRemaining
	}
	if goodsTotal != goodsCap {
		return nil, apperr.Failed("tcc commitment line composition cannot be attributed").
			WithDetails(map[string]any{
				"goods_cap_cent": goodsCap, "attributed_goods_cent": goodsTotal,
			})
	}
	if freightCap > 0 {
		expected = append(expected, tccCommitmentLine{LineNo: 0, FreightCent: freightCap})
	}
	sort.Slice(expected, func(i, j int) bool { return expected[i].LineNo < expected[j].LineNo })
	return expected, nil
}

func loadTccCommitmentLines(ctx context.Context, q querier, refundID string) ([]tccCommitmentLine, error) {
	rows, err := q.QueryContext(ctx, `SELECT line_no, quantity, goods_cent, freight_cent
		FROM order_refund_commitment_lines WHERE refund_id = ? ORDER BY line_no`, refundID)
	if err != nil {
		return nil, apperr.Internal("load tcc commitment lines", err)
	}
	defer rows.Close()
	lines := make([]tccCommitmentLine, 0, 4)
	for rows.Next() {
		var line tccCommitmentLine
		if err := rows.Scan(&line.LineNo, &line.Quantity, &line.GoodsCent, &line.FreightCent); err != nil {
			return nil, apperr.Internal("scan tcc commitment line", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate tcc commitment lines", err)
	}
	return lines, nil
}

func equalTccCommitmentLines(want, got []tccCommitmentLine) bool {
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if want[i] != got[i] {
			return false
		}
	}
	return true
}

// tccGroupComposition is the quota already taken by RESERVED/SETTLED
// commitments of one group, with the line attribution of their lines.
type tccGroupComposition struct {
	CommittedAmount  int64
	CommittedGoods   int64
	CommittedFreight int64
	// KnownGoods/KnownFreight are the header sums of the composition_known=1
	// commitments only. They are validated against their line rows even when
	// the group also has a known=0 legacy commitment (Review g1: a mixed group
	// must not skip the known=1 line check).
	KnownGoods   int64
	KnownFreight int64
	// UnknownComposition is true when at least one RESERVED/SETTLED commitment
	// of the group predates line composition (composition_known=0, e.g. the
	// order/0018 migration backfill goods=amount/freight=0 with no lines). Such
	// a group may only be refunded as a whole; Plan T1.2-R M1 forbids inventing
	// a line attribution for it.
	UnknownComposition bool
	LineGoods          map[int]int64
	LineQuantity       map[int]int64
}

// loadTccGroupComposition reads the RESERVED/SETTLED commitment headers and,
// only for groups whose commitments all have composition_known=1, their line
// attribution. A composition_known=0 row is not an error: it marks the group as
// whole-group-only and its stored goods/freight are not used as attribution
// (Plan T1.2-R M1, ADR §4.3).
//
// excludeRefundPrefix (resume) ignores only this compensation's own RESERVED
// commitments: they are the quota being resumed. Own SETTLED commitments stay
// in the composition because that quota was already refunded (T1.4b1).
func loadTccGroupComposition(ctx context.Context, q querier, orderNo, excludeRefundPrefix string) (map[string]*tccGroupComposition, error) {
	out := make(map[string]*tccGroupComposition, 2)
	rows, err := q.QueryContext(ctx, `SELECT group_no, refund_id, state, amount_cent, goods_cent, freight_cent, composition_known
		FROM order_refund_commitments WHERE order_no = ? AND state IN (?, ?)`,
		orderNo, commitmentReserved, commitmentSettled)
	if err != nil {
		return nil, apperr.Internal("load refund commitment composition", err)
	}
	for rows.Next() {
		var groupNo, refundID, state string
		var amount, goods, freight int64
		var compositionKnown int
		if err := rows.Scan(&groupNo, &refundID, &state, &amount, &goods, &freight, &compositionKnown); err != nil {
			_ = rows.Close()
			return nil, apperr.Internal("scan refund commitment composition", err)
		}
		if excludeRefundPrefix != "" && state == commitmentReserved &&
			strings.HasPrefix(strings.TrimSpace(refundID), excludeRefundPrefix) {
			continue
		}
		groupNo = strings.TrimSpace(groupNo)
		comp := out[groupNo]
		if comp == nil {
			comp = &tccGroupComposition{LineGoods: map[int]int64{}, LineQuantity: map[int]int64{}}
			out[groupNo] = comp
		}
		comp.CommittedAmount += amount
		if compositionKnown == 0 {
			// Legacy/pre-line commitment: quota is the only fact we may use.
			comp.UnknownComposition = true
			continue
		}
		if amount != goods+freight {
			_ = rows.Close()
			return nil, apperr.Failed("refund commitment composition is incomplete; manual backfill required").
				WithDetails(map[string]any{
					"order_no": orderNo, "refund_id": refundID, "amount_cent": amount,
					"goods_cent": goods, "freight_cent": freight,
				})
		}
		comp.CommittedGoods += goods
		comp.CommittedFreight += freight
		comp.KnownGoods += goods
		comp.KnownFreight += freight
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, apperr.Internal("iterate refund commitment composition", err)
	}
	if err := rows.Close(); err != nil {
		return nil, apperr.Internal("close refund commitment composition", err)
	}

	type lineKey struct {
		groupNo string
		lineNo  int
	}
	lineGoods := map[lineKey]int64{}
	lineFreight := map[string]int64{}
	lineQuantity := map[lineKey]int64{}
	// Only composition_known=1 commitments have line attribution; unknown
	// whole-group commitments are deliberately excluded from line math.
	lineRows, err := q.QueryContext(ctx, `SELECT c.group_no, c.refund_id, c.state, l.line_no,
		COALESCE(SUM(l.goods_cent),0), COALESCE(SUM(l.freight_cent),0), COALESCE(SUM(l.quantity),0)
		FROM order_refund_commitment_lines l
		JOIN order_refund_commitments c ON c.refund_id = l.refund_id
		WHERE c.order_no = ? AND c.state IN (?, ?) AND c.composition_known = 1
		GROUP BY c.group_no, c.refund_id, c.state, l.line_no`, orderNo, commitmentReserved, commitmentSettled)
	if err != nil {
		return nil, apperr.Internal("load refund commitment lines", err)
	}
	for lineRows.Next() {
		var groupNo, refundID, state string
		var lineNo int
		var goods, freight, quantity int64
		if err := lineRows.Scan(&groupNo, &refundID, &state, &lineNo, &goods, &freight, &quantity); err != nil {
			_ = lineRows.Close()
			return nil, apperr.Internal("scan refund commitment lines", err)
		}
		if excludeRefundPrefix != "" && state == commitmentReserved &&
			strings.HasPrefix(strings.TrimSpace(refundID), excludeRefundPrefix) {
			continue
		}
		groupNo = strings.TrimSpace(groupNo)
		key := lineKey{groupNo: groupNo, lineNo: lineNo}
		if lineNo > 0 {
			lineGoods[key] += goods
			lineQuantity[key] += quantity
		} else {
			lineFreight[groupNo] += freight
		}
	}
	if err := lineRows.Err(); err != nil {
		_ = lineRows.Close()
		return nil, apperr.Internal("iterate refund commitment lines", err)
	}
	if err := lineRows.Close(); err != nil {
		return nil, apperr.Internal("close refund commitment lines", err)
	}
	for groupNo, comp := range out {
		var goods, freight int64
		for key, value := range lineGoods {
			if key.groupNo == groupNo {
				goods += value
				comp.LineGoods[key.lineNo] = value
			}
		}
		for key, value := range lineQuantity {
			if key.groupNo == groupNo {
				comp.LineQuantity[key.lineNo] = value
			}
		}
		freight = lineFreight[groupNo]
		// g1: validate the known=1 portion even for a mixed group that also has
		// a known=0 legacy commitment. For a pure known=0 group both sums are 0
		// and there are no known=1 line rows, so this is a no-op.
		if goods != comp.KnownGoods || freight != comp.KnownFreight {
			return nil, apperr.Failed("refund commitment lines do not match the commitment headers").
				WithDetails(map[string]any{
					"order_no": orderNo, "group_no": groupNo,
					"known_composition": !comp.UnknownComposition,
					"header_goods_cent": comp.KnownGoods, "line_goods_cent": goods,
					"header_freight_cent": comp.KnownFreight, "line_freight_cent": freight,
				})
		}
	}
	return out, nil
}

type tccOrderLine struct {
	LineNo       int32
	Quantity     int32
	LinePaidCent int64
}

// loadTccLinesByGroup groups the order lines, keeping the same empty->G1
// fallback as the Σ precheck.
func loadTccLinesByGroup(ctx context.Context, q querier, orderNo string) (map[string][]tccOrderLine, error) {
	rows, err := q.QueryContext(ctx, `SELECT line_no, quantity, line_paid_cent, fulfillment_group_no
		FROM order_items WHERE order_no = ? ORDER BY line_no`, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order lines for tcc composition", err)
	}
	defer rows.Close()
	out := make(map[string][]tccOrderLine, 2)
	for rows.Next() {
		var line tccOrderLine
		var groupNo string
		if err := rows.Scan(&line.LineNo, &line.Quantity, &line.LinePaidCent, &groupNo); err != nil {
			return nil, apperr.Internal("scan order line for tcc composition", err)
		}
		groupNo = strings.TrimSpace(groupNo)
		if groupNo == "" {
			groupNo = defaultGroupNo
		}
		out[groupNo] = append(out[groupNo], line)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate order lines for tcc composition", err)
	}
	return out, nil
}
