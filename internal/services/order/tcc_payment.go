package order

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// This file implements T1.4b1: the per-group Payment.Refund phase of the TCC
// compensation (SPEC-0002 v1.9 AC-51/AC-65, ADR-0007 Amendment 1 §4.1.2
// steps 6-7 + T1.4a Rulings Addendum R-B).
//
// Preconditions enforced before ANY money moves:
//
//	batch state  ADMITTED (the admission phase of T1.4a completed every group),
//	             never FROZEN/ADMITTING and never ADMITTED-as-if-refunded;
//	application  PROCESSING with a resolved PRE_SHIP_REVOKED/POST_SHIP_ALLOWED
//	             admission result;
//	commitment   exactly the group's `tcc:<payment_no>:<group_no>` refund_id,
//	             state RESERVED (R-B: the quota is reserved in admission, not
//	             after payment) and amount == the frozen application amount.
//
// The whole group set is validated before the first refund call, so an
// incomplete/corrupt ledger produces zero refunds (no partial payment). Only a
// downstream Payment failure can leave a partial set; that keeps the batch in
// PAYING (retryable, same refund_ids) instead of pretending the compensation
// completed. The batch is closed to DONE only when every application is DONE
// after the payment.refunded projection (refund_consumer.go).

const tccRefundReasonPrefix = "tcc refund "

// tccRefundReason is a deterministic human-readable reason. Idempotency never
// depends on it: Payment keys the refund on the supplied refund_id.
func tccRefundReason(groupNo string) string {
	return truncate(tccRefundReasonPrefix+groupNo, 255)
}

// tccPaymentCommitment is the quota row a group refund is allowed to settle.
type tccPaymentCommitment struct {
	OrderNo    string
	GroupNo    string
	RefundID   string
	State      string
	AmountCent int64
}

func findTccCommitmentForPayment(ctx context.Context, q querier, refundID string) (*tccPaymentCommitment, error) {
	var commitment tccPaymentCommitment
	err := q.QueryRowContext(ctx, `SELECT order_no, group_no, refund_id, state, amount_cent
		FROM order_refund_commitments WHERE refund_id = ?`, refundID).
		Scan(&commitment.OrderNo, &commitment.GroupNo, &commitment.RefundID,
			&commitment.State, &commitment.AmountCent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperr.Internal("load refund commitment for tcc payment", err)
	}
	return &commitment, nil
}

// tccPaymentPlan is one validated group refund. Skip is set for a group that is
// already refunded (application DONE, or a commitment the payment event already
// settled) so a resume never pays it again.
type tccPaymentPlan struct {
	Application *tccApplicationRef
	Skip        bool
	SkipReason  string
}

// payTccBatch resumes the payment phase of an existing TCC batch. It is
// idempotent: a batch already DONE returns success, a PAYING batch continues
// with the groups that are not DONE yet, and every Retry reuses the same
// refund_id (`tcc:<payment_no>:<group_no>`).
func (s *Service) payTccBatch(ctx context.Context, orderNo, paymentNo string) error {
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return apperr.Internal("load order for tcc refund payment", err)
	}
	if record == nil {
		return nil
	}
	switch record.RefundState {
	case refundStateFull:
		// Every captured cent was refunded; nothing to pay.
		return nil
	case refundStateRefunding:
	case refundStatePartial:
		// The payment.refunded projection may have advanced the order while a
		// durable batch is still PAYING; the batch stays authoritative and
		// keeps refund_state REFUNDING while it is active.
	default:
		return apperr.Failed("tcc refund payment requires a REFUNDING order").
			WithDetails(map[string]any{
				"order_no": orderNo, "refund_state": record.RefundState,
			})
	}
	if strings.TrimSpace(paymentNo) == "" {
		paymentNo = strings.TrimSpace(record.SettlementPaymentNo)
	}
	if paymentNo == "" {
		return apperr.Failed("tcc refund payment requires the settlement payment_no").
			WithDetails(map[string]any{"order_no": orderNo})
	}

	batch, err := findActiveTccBatch(ctx, s.db, orderNo, paymentNo)
	if err != nil {
		return err
	}
	if batch == nil {
		// DONE (or audit-only PRECHECK_FAILED): nothing to resume.
		return nil
	}
	switch batch.State {
	case refundBatchAdmitted, refundBatchPaying:
		// Validated by planTccPayments below; only these two states may pay.
	case refundBatchFrozen, refundBatchAdmitting:
		// Admission must finish before money moves ("全部准入成功后才打款").
		return apperr.Unavailable("tcc refund batch is not admitted; admission must finish before payment", nil).
			WithDetails(map[string]any{
				"batch_id": batch.BatchID, "order_no": batch.OrderNo,
				"payment_no": batch.PaymentNo, "state": batch.State,
			})
	case refundBatchManualHold, refundBatchFailedPermanent:
		return tccBatchManualError(batch)
	case refundBatchFailedPre:
		// Audit-only row; no applications.
		return nil
	default:
		return apperr.Failed("unknown tcc refund batch state").
			WithDetails(map[string]any{"batch_id": batch.BatchID, "state": batch.State})
	}

	applications, err := loadTccApplications(ctx, s.db, batch.BatchID)
	if err != nil {
		return err
	}
	if len(applications) == 0 {
		return apperr.Failed("tcc refund batch has no group applications").
			WithDetails(map[string]any{"batch_id": batch.BatchID, "state": batch.State})
	}
	// Validate the whole group set BEFORE entering PAYING or refunding anything:
	// a partial/incomplete admission must produce zero money movement.
	plans, err := s.planTccPayments(ctx, batch, applications)
	if err != nil {
		return err
	}
	if batch.State == refundBatchAdmitted {
		if err := s.casTccBatchState(ctx, batch.BatchID, refundBatchAdmitted, refundBatchPaying); err != nil {
			// A concurrent worker may have completed or parked the batch;
			// converge instead of reporting a spurious failure.
			current, loadErr := findActiveTccBatch(ctx, s.db, orderNo, paymentNo)
			if loadErr != nil {
				return err
			}
			if current == nil {
				return nil
			}
			if current.State != refundBatchPaying {
				return err
			}
			batch = current
		}
	}
	for _, plan := range plans {
		if plan.Skip {
			s.log.Warn("tcc refund group is already refunded; skipping duplicate payment",
				"order_no", batch.OrderNo, "payment_no", batch.PaymentNo, "batch_id", batch.BatchID,
				"refund_id", plan.Application.RefundID, "group_no", plan.Application.GroupNo,
				"reason", plan.SkipReason)
			continue
		}
		if err := s.payTccGroup(ctx, batch, plan.Application); err != nil {
			return err
		}
	}
	if err := s.convergeTccBatch(ctx, batch.BatchID); err != nil {
		return err
	}
	return nil
}

// planTccPayments validates EVERY group before the first refund call. A batch
// that is not fully admitted/payable fails closed with no payment at all.
func (s *Service) planTccPayments(ctx context.Context, batch *tccRefundBatch, applications []*tccApplicationRef) ([]tccPaymentPlan, error) {
	plans := make([]tccPaymentPlan, 0, len(applications))
	for _, application := range applications {
		if application.State == refundApplicationDone {
			plans = append(plans, tccPaymentPlan{Application: application, Skip: true, SkipReason: "application DONE"})
			continue
		}
		if application.State != refundApplicationProcessing {
			return nil, apperr.Failed("tcc refund application is not admitted for payment").
				WithDetails(map[string]any{
					"batch_id": batch.BatchID, "refund_id": application.RefundID,
					"group_no": application.GroupNo, "state": application.State,
				})
		}
		if application.AdmissionResult != refundResultPre && application.AdmissionResult != refundResultPost {
			return nil, apperr.Failed("tcc refund application has no resolved admission result").
				WithDetails(map[string]any{
					"batch_id": batch.BatchID, "refund_id": application.RefundID,
					"group_no": application.GroupNo, "admission_result": application.AdmissionResult,
				})
		}
		// The refund identity is part of the contract: a group refund is always
		// tcc:<payment_no>:<group_no>. Never pay a renamed/fabricated refund_id.
		if want := tccGroupRefundID(batch.PaymentNo, application.GroupNo); application.RefundID != want {
			return nil, apperr.Failed("tcc refund application refund_id does not match the frozen group identity").
				WithDetails(map[string]any{
					"batch_id": batch.BatchID, "refund_id": application.RefundID, "want_refund_id": want,
				})
		}
		if application.AmountCent <= 0 {
			return nil, apperr.Failed("tcc refund group has no refundable amount").
				WithDetails(map[string]any{
					"batch_id": batch.BatchID, "refund_id": application.RefundID,
					"group_no": application.GroupNo, "amount_cent": application.AmountCent,
				})
		}
		commitment, err := findTccCommitmentForPayment(ctx, s.db, application.RefundID)
		if err != nil {
			return nil, err
		}
		if commitment == nil {
			return nil, apperr.Failed("tcc refund group has no RESERVED quota commitment").
				WithDetails(map[string]any{
					"batch_id": batch.BatchID, "refund_id": application.RefundID,
					"group_no": application.GroupNo,
				})
		}
		if commitment.OrderNo != batch.OrderNo || commitment.GroupNo != application.GroupNo {
			return nil, apperr.Failed("tcc refund commitment does not belong to the frozen group").
				WithDetails(map[string]any{
					"batch_id": batch.BatchID, "refund_id": application.RefundID,
					"commitment_order_no": commitment.OrderNo, "commitment_group_no": commitment.GroupNo,
				})
		}
		if commitment.AmountCent != application.AmountCent {
			return nil, apperr.Failed("tcc refund commitment amount does not match the frozen group cap").
				WithDetails(map[string]any{
					"batch_id": batch.BatchID, "refund_id": application.RefundID,
					"commitment_cent": commitment.AmountCent, "group_cap_cent": application.AmountCent,
				})
		}
		switch commitment.State {
		case commitmentReserved:
			plans = append(plans, tccPaymentPlan{Application: application})
		case commitmentSettled:
			// The payment fact already exists; the projection will mark the
			// application DONE. Do not call Payment again.
			plans = append(plans, tccPaymentPlan{Application: application, Skip: true, SkipReason: "commitment already SETTLED"})
		default:
			return nil, apperr.Failed("tcc refund commitment is not payable in its current state").
				WithDetails(map[string]any{
					"batch_id": batch.BatchID, "refund_id": application.RefundID,
					"group_no": application.GroupNo, "commitment_state": commitment.State,
				})
		}
	}
	return plans, nil
}

// payTccGroup executes one group refund. The refund_id is the idempotency key,
// so a retry after a crash/timeout cannot refund the same group twice.
func (s *Service) payTccGroup(ctx context.Context, batch *tccRefundBatch, application *tccApplicationRef) error {
	if s.payments == nil {
		return apperr.Unavailable("payment client is required for tcc refund payment", nil)
	}
	_, err := s.payments.Refund(ctx, &pb.RefundRequest{
		PaymentNo:      batch.PaymentNo,
		AmountCent:     application.AmountCent,
		Reason:         tccRefundReason(application.GroupNo),
		IdempotencyKey: application.RefundID,
	})
	if err != nil {
		return s.failTccPayment(ctx, batch, application, err)
	}
	s.log.Info("tcc refund payment accepted",
		"order_no", batch.OrderNo, "payment_no", batch.PaymentNo, "batch_id", batch.BatchID,
		"refund_id", application.RefundID, "group_no", application.GroupNo,
		"amount_cent", application.AmountCent, "admission_result", application.AdmissionResult)
	return nil
}

// failTccPayment keeps the batch resumable for a transient failure and parks it
// for manual handling when Payment definitively rejected the refund (for
// example the remaining captured amount changed). It never writes a refunded
// state: only the payment.refunded consumer may settle a commitment.
func (s *Service) failTccPayment(ctx context.Context, batch *tccRefundBatch, application *tccApplicationRef, cause error) error {
	mapped := cause
	var domain *apperr.Error
	if !errors.As(cause, &domain) {
		mapped = remoteError("refund payment", cause)
	}
	definitive := apperr.IsCode(mapped, apperr.CodeInvalidArgument) ||
		apperr.IsCode(mapped, apperr.CodeFailedPrecondition)

	lastErr := truncate(mapped.Error(), 1024)
	if _, err := s.db.ExecContext(ctx, `UPDATE refund_applications
		SET attempts = attempts + 1, last_error = ?, updated_at = NOW(3)
		WHERE refund_id = ?`, lastErr, application.RefundID); err != nil {
		s.log.Error("record tcc refund payment failure", "refund_id", application.RefundID, "error", err)
	}
	details := map[string]any{
		"batch_id": batch.BatchID, "order_no": batch.OrderNo, "payment_no": batch.PaymentNo,
		"refund_id": application.RefundID, "group_no": application.GroupNo,
		"amount_cent": application.AmountCent, "cause": lastErr,
	}
	if definitive {
		// ADR §4.1.3(5): a changed/insufficient captured amount stops the
		// remaining payments and parks the batch for a human decision.
		if err := s.casTccBatchState(ctx, batch.BatchID, refundBatchPaying, refundBatchManualHold); err != nil {
			s.log.Error("park tcc refund batch for manual handling", "batch_id", batch.BatchID, "error", err)
		}
		s.log.Error("tcc refund payment was rejected; batch requires manual handling",
			"order_no", batch.OrderNo, "batch_id", batch.BatchID,
			"refund_id", application.RefundID, "error", lastErr)
		return apperr.Failed("tcc refund payment was rejected; batch requires manual handling").
			WithDetails(details)
	}
	s.log.Error("tcc refund payment failed; batch remains PAYING for retry",
		"order_no", batch.OrderNo, "batch_id", batch.BatchID,
		"refund_id", application.RefundID, "error", lastErr)
	return apperr.Unavailable("tcc refund payment failed; retry the compensation", mapped).
		WithDetails(details)
}

// convergeTccBatch closes a PAYING batch once every application is DONE. The
// payment.refunded consumer calls the Tx variant; this one is the post-payment
// pass of the same convergence (usually a no-op until the events arrive).
func (s *Service) convergeTccBatch(ctx context.Context, batchID string) error {
	pending, err := countPendingTccApplications(ctx, s.db, batchID)
	if err != nil {
		return err
	}
	if pending > 0 {
		return nil
	}
	return s.completeTccBatch(ctx, s.db, batchID)
}

// countPendingTccApplications counts the applications of a batch that are not
// DONE. Callers already hold the order row lock (the consumer transaction) or
// are the single payment-phase caller, so no extra row lock is required.
func countPendingTccApplications(ctx context.Context, q querier, batchID string) (int, error) {
	var pending int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM refund_applications
		WHERE batch_id = ? AND state <> ?`, batchID, refundApplicationDone).Scan(&pending); err != nil {
		return 0, apperr.Internal("count unfinished tcc refund applications", err)
	}
	return pending, nil
}

// statementQuerier is a *sql.DB or *sql.Tx: the convergence write must run in
// the consumer transaction, and on the payment path it runs standalone.
type statementQuerier interface {
	querier
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// completeTccBatch is the only PAYING -> DONE writer. It never runs while an
// application is unfinished, and it never overwrites a human-owned state.
func (s *Service) completeTccBatch(ctx context.Context, q statementQuerier, batchID string) error {
	res, err := q.ExecContext(ctx, `UPDATE refund_batches
		SET state = ?, version = version + 1, updated_at = NOW(3)
		WHERE batch_id = ? AND state = ?`, refundBatchDone, batchID, refundBatchPaying)
	if err != nil {
		return apperr.Internal("mark tcc refund batch done", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		s.log.Info("tcc refund batch done", "batch_id", batchID)
		return nil
	}
	var state string
	if err := q.QueryRowContext(ctx, `SELECT state FROM refund_batches WHERE batch_id = ?`, batchID).Scan(&state); err != nil {
		return apperr.Internal("reload tcc refund batch state", err)
	}
	if state == refundBatchDone {
		return nil
	}
	if state == refundBatchPaying {
		return apperr.Conflict("tcc refund batch convergence lost the state CAS; retry").
			WithDetails(map[string]any{"batch_id": batchID, "state": state})
	}
	// A concurrent human/MANUAL_HOLD decision wins; do not overwrite it.
	s.log.Warn("tcc refund batch was not PAYING at convergence",
		"batch_id", batchID, "state", state)
	return nil
}

// activeTccBatchExists reports whether the order still has an in-flight TCC
// compensation. The payment.refunded projection uses it to keep
// orders.refund_state=REFUNDING while a durable batch is PAYING, so a partial
// refund event can never make the T1.2/T1.3 resume reject the still-unpaid
// groups as "batch exists but order is not REFUNDING".
func activeTccBatchExists(ctx context.Context, q querier, orderNo string) (bool, error) {
	var count int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM refund_batches
		WHERE source = ? AND order_no = ? AND state IN (?, ?, ?, ?)`,
		refundBatchSourceTCC, orderNo,
		refundBatchFrozen, refundBatchAdmitting, refundBatchAdmitted, refundBatchPaying).Scan(&count); err != nil {
		return false, apperr.Internal("load active tcc refund batch", err)
	}
	return count > 0, nil
}
