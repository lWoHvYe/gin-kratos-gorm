package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/outbox"

	"google.golang.org/grpc/codes"
)

// This file implements the durable T1.4b1-R2 lazy reconciliation of TCC group
// refunds:
//
//	Payment.GetRefundByKey(SUCCEEDED/REFUNDED)
//	  -> commitment RESERVED -> SETTLED (CAS; the single amount-projection trigger)
//	  -> application PROCESSING -> DONE
//	  -> batch DONE when every application is DONE
//	  -> orders.refunded_cent/refund_status/refund_state monotonic projection
//
// It never moves money and never calls RefundBatch. The worker is the durable
// alternative to relying on payment.refunded delivery: a committed Payment
// refund survives a lost/duplicated event, and the Order ledger converges on the
// next scan. The event consumer and the worker serialize on the same order row,
// the commitment CAS decides which writer owns the amount, and the projection is
// always max(current, SUM(SETTLED commitments)) instead of an increment, so an
// event cumulative total and a worker settlement can never double count.
//
// Active batches (including a parked MANUAL_HOLD/FAILED_PERMANENT row) keep
// orders.refund_state=REFUNDING. Only when no TCC batch is outstanding does the
// worker derive PARTIAL/FULL from refunded_cent vs paid_cent.

const (
	tccReconcileScanLimit = 100
	// tccReconcileDefaultInterval is intentionally conservative: the worker is a
	// safety net, not the fast path (payment.refunded remains the normal path).
	tccReconcileDefaultInterval = time.Minute
)

// RunTccReconcileWorker periodically settles TCC refund facts that exist in
// Payment but were never projected by payment.refunded. It is safe on multiple
// replicas: every write is a CAS/idempotent and all writers lock the order row.
func RunTccReconcileWorker(ctx context.Context, svc *Service, logger *slog.Logger, interval time.Duration) {
	if svc == nil || svc.payments == nil {
		if logger != nil {
			logger.Warn("tcc reconcile worker disabled: payment client is required")
		}
		return
	}
	if interval <= 0 {
		interval = tccReconcileDefaultInterval
	}
	// Run once at startup so a restart promptly repairs a lost event instead of
	// waiting for the first tick.
	if err := svc.runTccReconcileOnce(ctx); err != nil && logger != nil {
		logger.Error("tcc reconcile worker", "error", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := svc.runTccReconcileOnce(ctx); err != nil && logger != nil {
				logger.Error("tcc reconcile worker", "error", err)
			}
		}
	}
}

// runTccReconcileOnce scans a bounded page of active TCC batches. A failure on
// one batch is logged and does not stop the others (the returned error keeps the
// worker observable).
func (s *Service) runTccReconcileOnce(ctx context.Context) error {
	if s.payments == nil {
		return apperr.Unavailable("payment client is required for tcc reconcile", nil)
	}
	batches, err := s.scanActiveTccBatches(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, batch := range batches {
		if err := s.reconcileTccBatch(ctx, batch); err != nil {
			s.log.Error("reconcile tcc refund batch", "batch_id", batch.BatchID,
				"order_no", batch.OrderNo, "payment_no", batch.PaymentNo, "error", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Service) scanActiveTccBatches(ctx context.Context) ([]*tccRefundBatch, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT batch_id, order_no, payment_no, attempt_seq, state,
		captured_snapshot_cent, refunded_snapshot_cent, sum_group_cap_cent, remaining_captured_cent, version
		FROM refund_batches
		WHERE source = ? AND state IN (?, ?)
		ORDER BY updated_at, batch_id
		LIMIT ?`, refundBatchSourceTCC, refundBatchAdmitted, refundBatchPaying, tccReconcileScanLimit)
	if err != nil {
		return nil, apperr.Internal("scan active tcc refund batches", err)
	}
	defer rows.Close()
	batches := make([]*tccRefundBatch, 0, 8)
	for rows.Next() {
		batch := &tccRefundBatch{}
		if err := rows.Scan(&batch.BatchID, &batch.OrderNo, &batch.PaymentNo, &batch.AttemptSeq,
			&batch.State, &batch.CapturedSnapshotCent, &batch.RefundedSnapshotCent,
			&batch.SumGroupCapCent, &batch.RemainingCapturedCent, &batch.Version); err != nil {
			return nil, apperr.Internal("scan active tcc refund batch", err)
		}
		batches = append(batches, batch)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate active tcc refund batches", err)
	}
	return batches, nil
}

// reconcileTccBatch reads the Payment facts outside the transaction, then
// settles every successful refund in one order-locked transaction. A batch with
// unfinished applications and no successful fact is left untouched.
func (s *Service) reconcileTccBatch(ctx context.Context, batch *tccRefundBatch) error {
	applications, err := loadTccApplications(ctx, s.db, batch.BatchID)
	if err != nil {
		return err
	}
	facts := make([]tccReconcileFact, 0, len(applications))
	allDone := len(applications) > 0
	for _, application := range applications {
		if application.State != refundApplicationDone {
			allDone = false
		}
		fact, err := s.readTccReconcileFact(ctx, batch.PaymentNo, application)
		if err != nil {
			s.log.Warn("read tcc refund fact for reconcile", "batch_id", batch.BatchID,
				"refund_id", application.RefundID, "error", err)
			continue
		}
		if fact == nil {
			continue
		}
		commitment, err := findTccCommitmentForPayment(ctx, s.db, application.RefundID)
		if err != nil {
			return err
		}
		if commitment == nil {
			s.log.Warn("tcc refund fact has no commitment; leaving it for manual reconciliation",
				"batch_id", batch.BatchID, "refund_id", application.RefundID)
			continue
		}
		if commitment.OrderNo != batch.OrderNo || commitment.AmountCent != application.AmountCent {
			s.log.Warn("tcc refund commitment does not match the batch; leaving it",
				"batch_id", batch.BatchID, "refund_id", application.RefundID,
				"commitment_order_no", commitment.OrderNo,
				"commitment_cent", commitment.AmountCent, "application_cent", application.AmountCent)
			continue
		}
		if commitment.State != commitmentReserved && commitment.State != commitmentSettled {
			continue
		}
		facts = append(facts, tccReconcileFact{application: application, amountCent: fact.GetAmountCent()})
	}
	if len(facts) == 0 && !allDone {
		// Nothing to settle and the batch is not merely missing its DONE write.
		return nil
	}
	return db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		return s.applyTccReconcileTx(ctx, tx, batch, facts)
	})
}

// readTccReconcileFact returns a successful refund fact, nil for "no refund",
// and an error for a transient/deterministic read problem. The worker never
// parks a batch: a bad applicant is logged and skipped.
func (s *Service) readTccReconcileFact(ctx context.Context, paymentNo string, application *tccApplicationRef) (*pb.Refund, error) {
	fact, err := s.payments.GetRefundByKey(ctx, &pb.GetRefundByKeyRequest{
		PaymentNo: paymentNo, RefundId: application.RefundID,
	})
	if err != nil {
		if apperr.IsCode(err, apperr.CodeNotFound) || grpcCode(err) == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}
	if fact == nil {
		return nil, nil
	}
	if fact.GetRefundId() != application.RefundID {
		return nil, fmt.Errorf("refund fact key %q does not match application %q",
			fact.GetRefundId(), application.RefundID)
	}
	if fact.GetAmountCent() != application.AmountCent {
		return nil, fmt.Errorf("refund fact amount %d does not match application %d",
			fact.GetAmountCent(), application.AmountCent)
	}
	switch fact.GetStatus() {
	case tccRefundStatusSucceeded, tccRefundStatusRefunded:
		return fact, nil
	default:
		return nil, nil
	}
}

// tccReconcileFact is one successful Payment refund to settle in the order-locked
// transaction.
type tccReconcileFact struct {
	application *tccApplicationRef
	amountCent  int64
}

func (s *Service) applyTccReconcileTx(ctx context.Context, tx *sql.Tx, batch *tccRefundBatch, facts []tccReconcileFact) error {
	locked, err := findOrderForUpdate(ctx, tx, batch.OrderNo)
	if err != nil {
		return apperr.Internal("lock order for tcc reconcile", err)
	}
	if locked == nil {
		return apperr.NotFound("order not found for tcc reconcile")
	}
	switch locked.RefundState {
	case refundStateRefunding, refundStatePartial, refundStateFull:
	case refundStateNone:
		// A TCC batch only exists after Tx1 wrote REFUNDING. Seeing the batch
		// without it is manual damage; never invent a projection from it.
		s.log.Warn("tcc reconcile skipped: order is not REFUNDING",
			"order_no", batch.OrderNo, "batch_id", batch.BatchID, "refund_state", locked.RefundState)
		return nil
	default:
		return nil
	}
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT state FROM refund_batches WHERE batch_id = ?`,
		batch.BatchID).Scan(&state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return apperr.Internal("load tcc refund batch for reconcile", err)
	}
	if state != refundBatchAdmitted && state != refundBatchPaying && state != refundBatchDone {
		// Human-owned or terminal state: an automatic reconcile must not push it.
		return nil
	}
	changed := false
	for _, fact := range facts {
		settled, err := s.settleTccRefundFactTx(ctx, tx, batch.OrderNo, fact.application.RefundID, fact.amountCent)
		if err != nil {
			return err
		}
		if settled {
			changed = true
		}
		appDone, err := s.markTccApplicationDoneTx(ctx, tx, fact.application.RefundID)
		if err != nil {
			return err
		}
		if appDone {
			changed = true
		}
	}
	if len(facts) > 0 {
		// At least one Payment refund fact exists: the money-moving phase did
		// start, so an ADMITTED batch is moved to PAYING even while other groups
		// are still outstanding.
		if err := s.markTccBatchPayingTx(ctx, tx, batch.BatchID); err != nil {
			return err
		}
	}
	done, err := s.convergeTccBatchTx(ctx, tx, batch)
	if err != nil {
		return err
	}
	if done {
		changed = true
	}
	if !changed {
		return nil
	}
	return s.projectTccReconcileOrderTx(ctx, tx, locked, batch)
}

// settleTccRefundFactTx is the only lazy-reconcile amount trigger: it returns
// true only when this transaction performed RESERVED -> SETTLED. A concurrent
// event/worker that already settled the row returns false and never projects the
// amount again.
func (s *Service) settleTccRefundFactTx(ctx context.Context, tx *sql.Tx, orderNo, refundID string, amountCent int64) (bool, error) {
	var commitment tccPaymentCommitment
	err := tx.QueryRowContext(ctx, `SELECT order_no, group_no, refund_id, state, amount_cent
		FROM order_refund_commitments WHERE refund_id = ? FOR UPDATE`, refundID).
		Scan(&commitment.OrderNo, &commitment.GroupNo, &commitment.RefundID,
			&commitment.State, &commitment.AmountCent)
	if errors.Is(err, sql.ErrNoRows) {
		return false, apperr.Failed("tcc refund fact has no commitment").
			WithDetails(map[string]any{"refund_id": refundID})
	}
	if err != nil {
		return false, apperr.Internal("load refund commitment for tcc reconcile", err)
	}
	if commitment.OrderNo != orderNo || commitment.AmountCent != amountCent {
		return false, apperr.Failed("tcc refund fact does not match the reserved commitment").
			WithDetails(map[string]any{
				"refund_id": refundID, "event_order_no": orderNo, "amount_cent": amountCent,
				"commitment_order_no": commitment.OrderNo, "commitment_cent": commitment.AmountCent,
			})
	}
	switch commitment.State {
	case commitmentSettled:
		return false, nil
	case commitmentReserved:
		res, err := tx.ExecContext(ctx, `UPDATE order_refund_commitments
			SET state = ?, version = version + 1, updated_at = NOW(3)
			WHERE refund_id = ? AND state = ?`, commitmentSettled, refundID, commitmentReserved)
		if err != nil {
			return false, apperr.Internal("settle refund commitment for tcc reconcile", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return true, nil
		}
		// Lost a race to the event consumer or another replica. The other writer
		// owns the amount projection; this one only converges the application.
		return false, nil
	default:
		return false, apperr.Failed("refund commitment is not settleable for tcc reconcile").
			WithDetails(map[string]any{"refund_id": refundID, "state": commitment.State})
	}
}

// markTccApplicationDoneTx advances PROCESSING/PENDING -> DONE. A terminal
// non-DONE application (human decision) is never rewritten; the pending count
// keeps the batch open for reconciliation.
func (s *Service) markTccApplicationDoneTx(ctx context.Context, tx *sql.Tx, refundID string) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE refund_applications
		SET state = ?, last_error = '', version = version + 1, updated_at = NOW(3)
		WHERE refund_id = ? AND state IN (?, ?)`,
		refundApplicationDone, refundID, refundApplicationPending, refundApplicationProcessing)
	if err != nil {
		return false, apperr.Internal("mark tcc refund application done", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		return true, nil
	}
	var state string
	err = tx.QueryRowContext(ctx, `SELECT state FROM refund_applications WHERE refund_id = ?`, refundID).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, apperr.Internal("load tcc refund application for reconcile", err)
	}
	if state != refundApplicationDone {
		s.log.Error("tcc refund application is terminal non-DONE; leaving the batch open",
			"refund_id", refundID, "state", state)
	}
	return false, nil
}

// markTccBatchPayingTx moves ADMITTED -> PAYING for a batch whose Payment facts
// prove the money-moving phase started. It is idempotent and never overrides a
// human-owned state.
func (s *Service) markTccBatchPayingTx(ctx context.Context, tx *sql.Tx, batchID string) error {
	res, err := tx.ExecContext(ctx, `UPDATE refund_batches
		SET state = ?, version = version + 1, updated_at = NOW(3)
		WHERE batch_id = ? AND state = ?`, refundBatchPaying, batchID, refundBatchAdmitted)
	if err != nil {
		return apperr.Internal("move reconciled tcc batch to paying", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		return nil
	}
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT state FROM refund_batches WHERE batch_id = ?`, batchID).Scan(&state); err != nil {
		return apperr.Internal("reload tcc refund batch state", err)
	}
	switch state {
	case refundBatchPaying, refundBatchDone, refundBatchManualHold, refundBatchFailedPermanent:
		return nil
	default:
		return apperr.Failed("tcc refund batch cannot enter PAYING during reconcile").
			WithDetails(map[string]any{"batch_id": batchID, "state": state})
	}
}

// convergeTccBatchTx closes the batch once every application is DONE. An
// ADMITTED batch that already has successful Payment facts is moved through
// PAYING first (the money-moving phase did start); it is never DONE while an
// application is unfinished.
func (s *Service) convergeTccBatchTx(ctx context.Context, tx *sql.Tx, batch *tccRefundBatch) (bool, error) {
	pending, err := countPendingTccApplications(ctx, tx, batch.BatchID)
	if err != nil {
		return false, err
	}
	if pending > 0 {
		return false, nil
	}
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT state FROM refund_batches WHERE batch_id = ?`,
		batch.BatchID).Scan(&state); err != nil {
		return false, apperr.Internal("load tcc refund batch state for converge", err)
	}
	if state == refundBatchAdmitted {
		res, err := tx.ExecContext(ctx, `UPDATE refund_batches
			SET state = ?, version = version + 1, updated_at = NOW(3)
			WHERE batch_id = ? AND state = ?`,
			refundBatchPaying, batch.BatchID, refundBatchAdmitted)
		if err != nil {
			return false, apperr.Internal("move reconciled tcc batch to paying", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			state = refundBatchPaying
		} else if err := tx.QueryRowContext(ctx, `SELECT state FROM refund_batches WHERE batch_id = ?`,
			batch.BatchID).Scan(&state); err != nil {
			return false, apperr.Internal("reload tcc refund batch state", err)
		}
	}
	if state == refundBatchPaying {
		if err := s.completeTccBatch(ctx, tx, batch.BatchID); err != nil {
			return false, err
		}
	}
	if err := tx.QueryRowContext(ctx, `SELECT state FROM refund_batches WHERE batch_id = ?`,
		batch.BatchID).Scan(&state); err != nil {
		return false, apperr.Internal("reload tcc refund batch after converge", err)
	}
	return state == refundBatchDone, nil
}

// projectTccReconcileOrderTx is the worker/pay-path order amount projection.
// The amount is never incremented: it is max(current, SUM(SETTLED commitments)),
// so an event's cumulative OrderRefundedCent and a worker settlement cannot
// double count. While any TCC batch is not DONE/PRECHECK_FAILED the order stays
// REFUNDING; otherwise it is derived from refunded_cent vs paid_cent.
func (s *Service) projectTccReconcileOrderTx(ctx context.Context, tx *sql.Tx, locked *orderRecord, batch *tccRefundBatch) error {
	var settledCent int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cent), 0)
		FROM order_refund_commitments WHERE order_no = ? AND state = ?`,
		locked.OrderNo, commitmentSettled).Scan(&settledCent); err != nil {
		return apperr.Internal("sum settled tcc commitments", err)
	}
	nextRefunded := locked.RefundedCent
	if settledCent > nextRefunded {
		nextRefunded = settledCent
	}
	var outstanding int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM refund_batches
		WHERE source = ? AND order_no = ? AND state NOT IN (?, ?)`,
		refundBatchSourceTCC, locked.OrderNo, refundBatchDone, refundBatchFailedPre).Scan(&outstanding); err != nil {
		return apperr.Internal("count outstanding tcc refund batches", err)
	}
	nextStatus := locked.RefundStatus
	if nextRefunded > 0 {
		if locked.PaidCent > 0 && nextRefunded >= locked.PaidCent {
			nextStatus = statusRefunded
		} else {
			nextStatus = statusPartiallyRefunded
		}
	}
	nextState := refundStateFor(nextRefunded, locked.PaidCent)
	switch {
	case outstanding > 0:
		nextState = refundStateRefunding
	case nextRefunded <= 0 && locked.RefundState != refundStateNone:
		// No amount was projected yet; do not erase an existing REFUNDING without
		// a monetary fact to justify a terminal state.
		nextState = locked.RefundState
	}
	if locked.RefundState == refundStateFull {
		nextState = refundStateFull
	}
	if nextRefunded == locked.RefundedCent && nextStatus == locked.RefundStatus && nextState == locked.RefundState {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orders
		SET refunded_cent = ?, refund_status = ?, refund_state = ?, version = version + 1, updated_at = NOW(3)
		WHERE order_no = ?`, nextRefunded, nextStatus, nextState, locked.OrderNo); err != nil {
		return apperr.Internal("project tcc reconciled refund", err)
	}
	if err := insertStatusLog(ctx, tx, locked.OrderNo, locked.Status, locked.Status,
		"tcc reconcile refund "+nextStatus, "tcc-reconcile"); err != nil {
		return apperr.Internal("log tcc reconciled refund", err)
	}
	capturedCent := batch.CapturedSnapshotCent
	if capturedCent <= 0 {
		capturedCent = locked.PaidCent
	}
	return outbox.Append(ctx, tx, outbox.Event{
		Topic: "shop.order.refund_projected.v1", AggregateType: "order", AggregateID: locked.OrderNo,
		EventType: "order.refund_projected",
		Payload: map[string]any{
			"order_no": locked.OrderNo, "payment_no": batch.PaymentNo,
			"refunded_cent": nextRefunded, "captured_cent": capturedCent,
			"refund_status": nextStatus, "refund_state": nextState, "order_status": locked.Status,
		},
	})
}
