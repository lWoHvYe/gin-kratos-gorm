package order

import (
	"context"
	"database/sql"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
)

// This file implements the T1.4a per-group admission phase of the TCC
// compensation (SPEC-0002 v1.9 AC-51/AC-65, ADR-0007 Amendment 1 §4.1.2
// steps 4-5). It runs AFTER Tx1 froze the batch and the applications, never
// inside a database transaction for the remote CancelIfCreated, and it never
// moves money: payment is T1.4b.
//
// For every active group application of the frozen batch:
//
//	POST (already out of the warehouse, decided by T1.2 from the Fulfillment
//	     facts)  -> persist admission POST + RESERVED commitment, no permit
//	                write and no CancelIfCreated;
//	PRE / PENDING_REMOTE -> call AdmitRefund, which resolves CONSUMED groups
//	     through Fulfillment.CancelIfCreated (CANCELLED -> PRE,
//	     ALREADY_SHIPPED/RECEIVED -> POST) and returns PRE for REVOKED permits.
//
// The batch is moved FROZEN -> ADMITTING before the loop and ADMITTING ->
// ADMITTED only when every active group is admitted. A failure leaves the
// application and the batch resumable (PENDING_REMOTE stays PENDING_REMOTE);
// a rerun fills only the unfinished groups. A PAYING batch (T1.4b already
// started paying) is also repaired here: DONE/PROCESSING applications are
// skipped, so re-admission can never re-pay a group. The batch is not moved
// back to ADMITTED in that case.

// tccApplicationRef is one active group application of a frozen batch.
type tccApplicationRef struct {
	RefundID        string
	GroupNo         string
	AmountCent      int64
	GoodsCent       int64
	FreightCent     int64
	State           string
	AdmissionResult string
	Version         int64
}

// admitTccBatch drives the per-group admission of an existing TCC batch. It is
// idempotent: applications already PROCESSING with a resolved result are
// skipped, so a crash only resumes the unfinished groups.
func (s *Service) admitTccBatch(ctx context.Context, orderNo, paymentNo string) error {
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return apperr.Internal("load order for tcc refund admission", err)
	}
	if record == nil || record.RefundState != refundStateRefunding {
		// Tx1 either did not freeze anything (FULL / no active group) or the
		// compensation already completed; no admission phase to run.
		return nil
	}
	if strings.TrimSpace(paymentNo) == "" {
		paymentNo = strings.TrimSpace(record.SettlementPaymentNo)
	}
	batch, err := findActiveTccBatch(ctx, s.db, orderNo, paymentNo)
	if err != nil {
		return err
	}
	if batch == nil {
		return nil
	}
	switch batch.State {
	case refundBatchFrozen:
		if err := s.casTccBatchState(ctx, batch.BatchID, refundBatchFrozen, refundBatchAdmitting); err != nil {
			return err
		}
		batch.State = refundBatchAdmitting
	case refundBatchAdmitting, refundBatchAdmitted, refundBatchPaying:
		// Resume/self-heal: fill only the groups that are not admitted yet.
		// PAYING is a repair path only (a crash left an application PENDING);
		// DONE/PROCESSING groups are skipped and never paid here.
	case refundBatchDone, refundBatchFailedPre:
		// DONE is complete; a PRECHECK_FAILED row has no applications and is
		// audit-only (ADR §6.1).
		return nil
	case refundBatchManualHold, refundBatchFailedPermanent:
		// Human-owned states are never pushed by an automatic retry.
		return tccBatchManualError(batch)
	default:
		return apperr.Failed("unknown tcc refund batch state").
			WithDetails(map[string]any{"batch_id": batch.BatchID, "state": batch.State})
	}

	applications, err := loadTccApplications(ctx, s.db, batch.BatchID)
	if err != nil {
		return err
	}
	failures := make([]map[string]any, 0, 1)
	for _, application := range applications {
		if err := s.admitTccApplication(ctx, batch, application); err != nil {
			s.log.Error("tcc refund admission failed; group remains retryable",
				"order_no", batch.OrderNo, "payment_no", batch.PaymentNo,
				"batch_id", batch.BatchID, "refund_id", application.RefundID,
				"group_no", application.GroupNo, "state", application.State,
				"admission_result", application.AdmissionResult, "error", err)
			failures = append(failures, map[string]any{
				"refund_id": application.RefundID, "group_no": application.GroupNo, "error": err.Error(),
			})
		}
	}
	if len(failures) > 0 {
		// AC-71: the Tx2 that persisted the review holds the batch in the same
		// transaction. Re-read it so the caller gets the stable
		// TCC_BATCH_REQUIRES_MANUAL_HANDLING instead of a generic retry error.
		var state string
		if err := s.db.QueryRowContext(ctx, `SELECT state FROM refund_batches WHERE batch_id = ?`, batch.BatchID).Scan(&state); err == nil && state == refundBatchManualHold {
			batch.State = state
			return tccBatchManualError(batch)
		}
		return apperr.Unavailable("tcc refund admission is incomplete; retry the compensation", nil).
			WithDetails(map[string]any{
				"order_no": batch.OrderNo, "payment_no": batch.PaymentNo,
				"batch_id": batch.BatchID, "failed_groups": failures,
			})
	}
	if batch.State == refundBatchPaying {
		// Repair of an in-flight payment batch: never move it back to ADMITTED.
		return nil
	}
	return s.markTccBatchAdmitted(ctx, batch.BatchID)
}

// admitTccApplication resolves one active group application.
func (s *Service) admitTccApplication(ctx context.Context, batch *tccRefundBatch, application *tccApplicationRef) error {
	if application.AdmissionResult == refundReviewRequired && application.State != refundApplicationDone {
		// AC-71: a persisted review on the application means the automatic path
		// is already terminated; do not call AdmitRefund/CancelIfCreated again.
		// The manual re-arm resets only the authoritative admission row, so a
		// re-armed decision must fall through and be re-evaluated.
		admission, err := findRefundAdmission(ctx, s.db, application.RefundID)
		if err != nil {
			return apperr.Internal("load refund admission for tcc application", err)
		}
		if classifyExistingAdmission(admission) == existingAdmissionReview {
			row := &refundAdmissionRow{
				RefundID: application.RefundID, OrderNo: batch.OrderNo,
				GroupNo: application.GroupNo, AmountCent: application.AmountCent,
				Result: refundReviewRequired, State: refundPending,
			}
			return refundReviewConflict(row, refundReasonConsumedWithoutShipment, probeStatusNoWaybill)
		}
	}
	switch application.State {
	case refundApplicationDone:
		// Already executed (T1.4b); never refund it twice.
		return nil
	case refundApplicationPending, refundApplicationProcessing:
	default:
		// CANCELLED/REJECTED/FAILED_PERMANENT are human-owned and must not be
		// pushed automatically (INV-45).
		return apperr.Failed("tcc refund application is not resumable in its current state").
			WithDetails(map[string]any{
				"order_no": batch.OrderNo, "batch_id": batch.BatchID,
				"refund_id": application.RefundID, "state": application.State,
			})
	}
	if application.State == refundApplicationProcessing &&
		(application.AdmissionResult == refundResultPre || application.AdmissionResult == refundResultPost) {
		return nil
	}
	if application.AdmissionResult == refundResultPost {
		// The already-left-warehouse fact was decided by Tx1 from the
		// Fulfillment snapshot. A code that still shipped might not have a
		// permit row (legacy G1), so calling CancelIfCreated/AdmitRefund would
		// create a NONE permit and could downgrade POST to PRE. Persist the
		// decided POST admission and reserve the quota instead.
		return s.recordTccPostAdmission(ctx, batch, application)
	}
	reply, err := s.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: application.RefundID, OrderNo: batch.OrderNo, GroupNo: application.GroupNo,
		AmountCent: application.AmountCent, Source: refundApplicationSourceTCC, BatchId: batch.BatchID,
	})
	if err != nil {
		// A CONSUMED group whose CancelIfCreated failed keeps its PENDING_REMOTE
		// admission; the next TCC retry resumes it.
		return err
	}
	result := reply.GetResult()
	if result != refundResultPre && result != refundResultPost {
		return apperr.Failed("tcc refund admission did not resolve to PRE/POST").
			WithDetails(map[string]any{
				"order_no": batch.OrderNo, "refund_id": application.RefundID,
				"result": result, "state": reply.GetState(),
			})
	}
	return s.updateTccApplicationAdmission(ctx, application, result)
}

// updateTccApplicationAdmission advances the application to PROCESSING (the
// admitted state that still waits for the T1.4b payment) with a version CAS.
func (s *Service) updateTccApplicationAdmission(ctx context.Context, application *tccApplicationRef, result string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE refund_applications
		SET state = ?, admission_result = ?, version = version + 1, updated_at = NOW(3)
		WHERE refund_id = ? AND version = ? AND state IN (?, ?)`,
		refundApplicationProcessing, result, application.RefundID, application.Version,
		refundApplicationPending, refundApplicationProcessing)
	if err != nil {
		return apperr.Internal("update tcc refund application admission", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		return nil
	}
	// Another caller advanced the row. Converge on the persisted result
	// instead of overwriting it.
	current, err := findRefundApplication(ctx, s.db, application.RefundID, false)
	if err != nil {
		return apperr.Internal("reload tcc refund application admission", err)
	}
	if current == nil {
		return apperr.Failed("tcc refund application disappeared during admission").
			WithDetails(map[string]any{"refund_id": application.RefundID})
	}
	if current.State == refundApplicationDone {
		return nil
	}
	if current.State == refundApplicationProcessing && current.AdmissionResult == result {
		return nil
	}
	return apperr.Failed("tcc refund application admission changed concurrently").
		WithDetails(map[string]any{
			"refund_id": application.RefundID, "state": current.State,
			"admission_result": current.AdmissionResult, "want_admission_result": result,
		})
}

// recordTccPostAdmission persists the POST admission and the RESERVED
// commitment of a group that already left the warehouse. Tx1 deliberately
// writes neither for a shipped group (Addendum 2 R-2 §3.3); the admission
// phase writes them before any payment can run, so a concurrent after-sale
// cannot re-use the same quota.
func (s *Service) recordTccPostAdmission(ctx context.Context, batch *tccRefundBatch, application *tccApplicationRef) error {
	if application.AmountCent <= 0 {
		return apperr.Failed("tcc post-ship application has no refundable amount").
			WithDetails(map[string]any{"refund_id": application.RefundID, "amount_cent": application.AmountCent})
	}
	if application.GoodsCent+application.FreightCent != application.AmountCent {
		return apperr.Failed("tcc post-ship application composition is inconsistent").
			WithDetails(map[string]any{
				"refund_id": application.RefundID, "amount_cent": application.AmountCent,
				"goods_cent": application.GoodsCent, "freight_cent": application.FreightCent,
			})
	}
	return db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		locked, err := findOrderForUpdate(ctx, tx, batch.OrderNo)
		if err != nil {
			return apperr.Internal("lock order for tcc post-ship admission", err)
		}
		if locked == nil {
			return apperr.NotFound("order not found")
		}
		current, err := findRefundApplication(ctx, tx, application.RefundID, true)
		if err != nil {
			return apperr.Internal("load tcc refund application for post-ship admission", err)
		}
		if current == nil {
			return apperr.Failed("tcc refund application is missing for post-ship admission").
				WithDetails(map[string]any{"refund_id": application.RefundID})
		}
		if current.OrderNo != batch.OrderNo || current.GroupNo != application.GroupNo ||
			current.AmountCent != application.AmountCent || current.Source != refundApplicationSourceTCC {
			return apperr.Failed("tcc refund application conflicts with the frozen batch").
				WithDetails(map[string]any{
					"refund_id": application.RefundID, "order_no": current.OrderNo,
					"group_no": current.GroupNo, "amount_cent": current.AmountCent, "source": current.Source,
				})
		}
		if current.State == refundApplicationDone {
			return nil
		}
		if current.AdmissionResult != refundResultPost && current.AdmissionResult != "" {
			return apperr.Failed("tcc refund application admission result conflicts with the shipped fact").
				WithDetails(map[string]any{
					"refund_id": application.RefundID, "admission_result": current.AdmissionResult,
				})
		}
		if current.State != refundApplicationPending && current.State != refundApplicationProcessing {
			return apperr.Failed("tcc refund application is not resumable in its current state").
				WithDetails(map[string]any{"refund_id": application.RefundID, "state": current.State})
		}

		admissionInserted := false
		existing, err := findRefundAdmissionLocked(ctx, tx, application.RefundID)
		if err != nil {
			return apperr.Internal("load tcc post-ship admission", err)
		}
		if existing != nil {
			if existing.Result != refundResultPost || existing.State != refundResultPost ||
				existing.AmountCent != current.AmountCent ||
				existing.OrderNo != batch.OrderNo || existing.GroupNo != application.GroupNo {
				return apperr.Failed("tcc post-ship admission conflicts with the frozen batch").
					WithDetails(map[string]any{
						"refund_id": application.RefundID, "result": existing.Result,
						"state": existing.State, "amount_cent": existing.AmountCent,
					})
			}
		} else {
			if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_admissions
				(refund_id, order_no, group_no, result, state, amount_cent, version)
				VALUES (?, ?, ?, ?, ?, ?, 1)`,
				application.RefundID, batch.OrderNo, application.GroupNo,
				refundResultPost, refundResultPost, current.AmountCent); err != nil {
				return apperr.Internal("insert tcc post-ship admission", err)
			}
			admissionInserted = true
		}

		// Reserve the quota with the same composition rules as Tx1. Own TCC
		// commitments are excluded so a resume recomputes the same lines.
		composition, err := loadTccGroupComposition(ctx, tx, batch.OrderNo, tccCommitmentPrefix(batch.PaymentNo))
		if err != nil {
			return err
		}
		lines, err := loadTccLinesByGroup(ctx, tx, batch.OrderNo)
		if err != nil {
			return err
		}
		comp := composition[application.GroupNo]
		if comp == nil {
			comp = &tccGroupComposition{}
		}
		known := !comp.UnknownComposition
		if err := ensureTccCommitmentTx(ctx, tx, batch.OrderNo, application.GroupNo, application.RefundID,
			current.AmountCent, known, current.GoodsCent, current.FreightCent, comp, lines[application.GroupNo]); err != nil {
			return err
		}

		res, err := tx.ExecContext(ctx, `UPDATE refund_applications
			SET state = ?, admission_result = ?, version = version + 1, updated_at = NOW(3)
			WHERE refund_id = ? AND version = ?`,
			refundApplicationProcessing, refundResultPost, application.RefundID, current.Version)
		if err != nil {
			return apperr.Internal("mark tcc application admitted post-ship", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return apperr.Failed("tcc post-ship application changed under the order lock").
				WithDetails(map[string]any{"refund_id": application.RefundID, "version": current.Version})
		}
		if admissionInserted {
			if err := appendRefundAdmitted(ctx, tx, application.RefundID, batch.OrderNo, application.GroupNo,
				refundResultPost, current.AmountCent, 1, refundResultPost); err != nil {
				return err
			}
		}
		return nil
	})
}

// loadTccApplications reads the active group applications of one batch.
func loadTccApplications(ctx context.Context, q querier, batchID string) ([]*tccApplicationRef, error) {
	rows, err := q.QueryContext(ctx, `SELECT refund_id, COALESCE(group_no,''), amount_cent, goods_cent,
		freight_cent, state, admission_result, version
		FROM refund_applications WHERE batch_id = ? ORDER BY group_no`, batchID)
	if err != nil {
		return nil, apperr.Internal("load tcc refund applications", err)
	}
	defer rows.Close()
	applications := make([]*tccApplicationRef, 0, 2)
	for rows.Next() {
		var application tccApplicationRef
		if err := rows.Scan(&application.RefundID, &application.GroupNo, &application.AmountCent,
			&application.GoodsCent, &application.FreightCent, &application.State,
			&application.AdmissionResult, &application.Version); err != nil {
			return nil, apperr.Internal("scan tcc refund application", err)
		}
		applications = append(applications, &application)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate tcc refund applications", err)
	}
	return applications, nil
}

// findActiveTccBatch returns the one resumable batch of a compensation, or nil
// when only audit/completed batches exist. Multiple active batches violate ADR
// §6.1 and fail closed.
func findActiveTccBatch(ctx context.Context, q querier, orderNo, paymentNo string) (*tccRefundBatch, error) {
	rows, err := q.QueryContext(ctx, `SELECT batch_id, order_no, payment_no, attempt_seq, state,
		captured_snapshot_cent, refunded_snapshot_cent, sum_group_cap_cent, remaining_captured_cent, version
		FROM refund_batches WHERE source = ? AND order_no = ? AND payment_no = ? ORDER BY attempt_seq DESC`,
		refundBatchSourceTCC, orderNo, paymentNo)
	if err != nil {
		return nil, apperr.Internal("load tcc refund batches for admission", err)
	}
	defer rows.Close()
	var active *tccRefundBatch
	var manual *tccRefundBatch
	for rows.Next() {
		batch := &tccRefundBatch{}
		if err := rows.Scan(&batch.BatchID, &batch.OrderNo, &batch.PaymentNo, &batch.AttemptSeq,
			&batch.State, &batch.CapturedSnapshotCent, &batch.RefundedSnapshotCent,
			&batch.SumGroupCapCent, &batch.RemainingCapturedCent, &batch.Version); err != nil {
			return nil, apperr.Internal("scan tcc refund batch for admission", err)
		}
		switch batch.State {
		case refundBatchFrozen, refundBatchAdmitting, refundBatchAdmitted, refundBatchPaying:
			if active != nil {
				return nil, apperr.Failed("multiple active TCC refund batches; manual reconciliation required").
					WithDetails(map[string]any{
						"order_no": orderNo, "payment_no": paymentNo,
						"latest_batch": advancedTccBatch(active, batch).BatchID,
					})
			}
			active = batch
		case refundBatchManualHold, refundBatchFailedPermanent:
			manual = batch
		case refundBatchDone, refundBatchFailedPre:
			// Not resumable for admission: DONE is complete, PRECHECK_FAILED
			// is audit-only (ADR §6.1).
		default:
			return nil, apperr.Failed("unknown tcc refund batch state").
				WithDetails(map[string]any{"batch_id": batch.BatchID, "state": batch.State})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate tcc refund batches for admission", err)
	}
	if active != nil {
		return active, nil
	}
	if manual != nil {
		return nil, tccBatchManualError(manual)
	}
	return nil, nil
}

func advancedTccBatch(current, candidate *tccRefundBatch) *tccRefundBatch {
	if candidate.AttemptSeq > current.AttemptSeq {
		return candidate
	}
	return current
}

// casTccBatchState moves the batch with a version CAS. A concurrent worker
// that already reached the target (or a later admission state) is success.
func (s *Service) casTccBatchState(ctx context.Context, batchID, from, to string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE refund_batches
		SET state = ?, version = version + 1, updated_at = NOW(3)
		WHERE batch_id = ? AND state = ?`, to, batchID, from)
	if err != nil {
		return apperr.Internal("move tcc refund batch state", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		return nil
	}
	var state string
	if err := s.db.QueryRowContext(ctx, `SELECT state FROM refund_batches WHERE batch_id = ?`, batchID).Scan(&state); err != nil {
		return apperr.Internal("reload tcc refund batch state", err)
	}
	if state == to || (to == refundBatchAdmitting && (state == refundBatchAdmitting || state == refundBatchAdmitted)) {
		return nil
	}
	return apperr.Failed("tcc refund batch state is not the expected state").
		WithDetails(map[string]any{"batch_id": batchID, "state": state, "want_from": from, "want_to": to})
}

// markTccBatchAdmitted is the final CAS of the admission phase. It never moves
// the batch to PAYING/DONE: the payment phase is T1.4b.
func (s *Service) markTccBatchAdmitted(ctx context.Context, batchID string) error {
	for attempt := 0; attempt < 3; attempt++ {
		res, err := s.db.ExecContext(ctx, `UPDATE refund_batches
			SET state = ?, version = version + 1, updated_at = NOW(3)
			WHERE batch_id = ? AND state = ?`, refundBatchAdmitted, batchID, refundBatchAdmitting)
		if err != nil {
			return apperr.Internal("mark tcc refund batch admitted", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return nil
		}
		var state string
		if err := s.db.QueryRowContext(ctx, `SELECT state FROM refund_batches WHERE batch_id = ?`, batchID).Scan(&state); err != nil {
			return apperr.Internal("reload tcc refund batch after admission", err)
		}
		switch state {
		case refundBatchAdmitted, refundBatchPaying, refundBatchDone:
			// A concurrent worker already completed admission and may have
			// started/finished the T1.4b1 payment phase; admission is complete.
			return nil
		case refundBatchAdmitting:
			continue
		default:
			return apperr.Failed("tcc refund batch left ADMITTING during admission").
				WithDetails(map[string]any{"batch_id": batchID, "state": state})
		}
	}
	return apperr.Conflict("tcc refund batch admission CAS contention; retry").
		WithDetails(map[string]any{"batch_id": batchID})
}
