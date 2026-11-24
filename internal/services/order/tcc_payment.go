package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// This file implements T1.4b1-R: the atomic Payment.RefundBatch phase of the
// TCC compensation (SPEC-0002 v1.9 AC-51/AC-65, ADR-0007 Amendment 1 §4.1.2
// steps 6-7, C-04 RefundBatch, T1.4b1-R Plan §3).
//
// Preconditions enforced before ANY money moves:
//
//	batch state  ADMITTED (the admission phase of T1.4a completed every group),
//	             never FROZEN/ADMITTING and never ADMITTED-as-if-refunded;
//	application  PROCESSING with a resolved PRE_SHIP_REVOKED/POST_SHIP_ALLOWED
//	             admission result (DONE only after the Payment fact exists);
//	commitment   exactly the group's `tcc:<payment_no>:<group_no>` refund_id,
//	             state RESERVED (R-B: the quota is reserved in admission, not
//	             after payment) and amount == the frozen application amount.
//
// C-04 replaced the per-group Payment.Refund fan-out with ONE atomic
// RefundBatch. Before that call the preflight reads the authoritative Payment
// facts (GetIntent + one GetRefundByKey per group) and proves:
//
//	B = batch.SumGroupCapCent (frozen; never recomputed from current caps)
//	B == done + inFlight + outstanding, unknown == 0
//	R == captured - refunded == B - done          (no external/extra refund)
//
// Any failure parks the batch in MANUAL_HOLD with zero Payment calls. While a
// group refund is in-flight no new group may be started; once all outstanding
// groups are known the batch is sent as a single all-or-none RefundBatch with
// expected_total_cent == outstanding (not B).
//
// A Payment refund fact that exists while the Order ledger is still RESERVED
// (a lost/duplicated payment.refunded event) is lazily reconciled: commitment
// SETTLED, application DONE, batch DONE only when every group is DONE. The
// reconciliation never re-pays and never closes a partly refunded batch.

const tccRefundReasonPrefix = "tcc refund batch "

// Payment refund statuses (payment/refunds.status). A refund row is written as
// SUCCEEDED by Payment today; REFUNDED is accepted as the terminal alias.
const (
	tccRefundStatusSucceeded  = "SUCCEEDED"
	tccRefundStatusRefunded   = "REFUNDED"
	tccRefundStatusPending    = "PENDING"
	tccRefundStatusProcessing = "PROCESSING"
)

const (
	// refundBatchReasonUnavailable is C-04's transient batch failure. Order
	// keeps PAYING and retries; every other deterministic error parks the
	// batch for manual handling.
	refundBatchReasonUnavailable = "BATCH_UNAVAILABLE"

	// RefundBatchReply.status values (C-04 §2.6).
	refundBatchStatusRefunded        = "REFUNDED"
	refundBatchStatusAlreadyRefunded = "ALREADY_REFUNDED"

	// tccRefundInFlightTimeout is a local operational threshold (C-04 §3 only
	// requires "timeout/unknown -> MANUAL_HOLD"; the duration is not a cross
	// service contract). A refund that stays PENDING/PROCESSING longer than
	// this must not keep the automatic retry loop alive forever.
	tccRefundInFlightTimeout = 30 * time.Minute
)

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

// tccPaymentCandidate is one validated group of the frozen batch: its refund
// application and the RESERVED/SETTLED quota commitment.
type tccPaymentCandidate struct {
	Application *tccApplicationRef
	Commitment  *tccPaymentCommitment
}

type tccRefundClass int

const (
	tccRefundUnknown tccRefundClass = iota
	tccRefundDone
	tccRefundInFlight
	tccRefundOutstanding
)

// tccRefundAssessment is the Payment/Order classification of one group.
type tccRefundAssessment struct {
	Candidate *tccPaymentCandidate
	Fact      *pb.Refund
	Class     tccRefundClass
	Reason    string
	// Reconcile is set when the Payment fact is already successful but the
	// Order ledger (commitment/application) still lags behind.
	Reconcile bool
}

// tccRefundPreflight is the read-only pre-payment view of the batch.
type tccRefundPreflight struct {
	CapturedCent  int64
	RemainingCent int64
	DoneCent      int64
	InFlightCent  int64
	Outstanding   int64
	UnknownCent   int64

	Done      []*tccRefundAssessment
	InFlight  []*tccRefundAssessment
	Pending   []*tccRefundAssessment
	Uncertain []*tccRefundAssessment
}

// tccManualHold carries a deterministic preflight/ledger failure that must park
// the batch instead of retrying forever. It is never returned by a path that
// moved money.
type tccManualHold struct {
	Reason  string
	Details map[string]any
}

func (e *tccManualHold) Error() string { return e.Reason }

// payTccBatch resumes the atomic payment phase of an existing TCC batch. It is
// idempotent: a batch already DONE returns success, a PAYING batch re-reads the
// Payment facts and only sends the still-outstanding groups, and every retry
// reuses the same batch_id so Payment can replay its persisted reply.
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
		// Validated by the preflight below; only these two states may pay.
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
	candidates, err := s.planTccRefundCandidates(ctx, batch, applications)
	if err != nil {
		return err
	}

	pre, err := s.preflightTccRefund(ctx, batch, candidates)
	if err != nil {
		var manual *tccManualHold
		if errors.As(err, &manual) {
			return s.parkTccBatchManual(ctx, batch, manual.Reason, mergeTccDetails(manual.Details, nil))
		}
		return err
	}

	// inFlight first: a group whose Payment refund is still PENDING/PROCESSING
	// must never be joined by a new group. Only re-reading the same refund_id
	// (the next compensation retry) may converge it.
	if pre.InFlightCent > 0 {
		for _, assessment := range pre.InFlight {
			if s.tccRefundInFlightTimedOut(assessment.Fact) {
				return s.parkTccBatchManual(ctx, batch, "TCC_REFUND_IN_FLIGHT_TIMEOUT",
					pre.details(map[string]any{
						"refund_id":       assessment.Candidate.Application.RefundID,
						"group_no":        assessment.Candidate.Application.GroupNo,
						"refund_status":   assessment.Fact.GetStatus(),
						"created_at_unix": assessment.Fact.GetCreatedAtUnix(),
					}))
			}
		}
		if err := s.markTccBatchPaying(ctx, batch); err != nil {
			if errors.Is(err, errTccBatchDone) {
				return nil
			}
			return err
		}
		s.log.Warn("tcc refund payments are in flight; no new group may be started",
			"order_no", batch.OrderNo, "payment_no", batch.PaymentNo, "batch_id", batch.BatchID,
			"in_flight_cent", pre.InFlightCent)
		return apperr.Unavailable("tcc refund has in-flight group refunds; retry after they converge", nil).
			WithDetails(pre.details(nil))
	}

	if err := s.markTccBatchPaying(ctx, batch); err != nil {
		if errors.Is(err, errTccBatchDone) {
			return nil
		}
		return err
	}

	// A successful GetRefundByKey with a lagging Order ledger is the lost-event
	// case: settle the commitment/application/batch from the Payment fact, but
	// only after the Σ gate above (an external mismatch must win MANUAL_HOLD and
	// must not be masked by reconciliation).
	if err := s.reconcileTccDoneApplications(ctx, batch, pre.Done); err != nil {
		var manual *tccManualHold
		if errors.As(err, &manual) {
			return s.parkTccBatchManual(ctx, batch, manual.Reason, mergeTccDetails(manual.Details, pre.details(nil)))
		}
		return err
	}
	if pre.Outstanding == 0 {
		// Every group already has a successful Payment fact; converge the batch
		// (DONE only if every application is DONE, never earlier).
		return s.convergeTccBatch(ctx, batch.BatchID)
	}

	items := make([]*pb.RefundBatchItem, 0, len(pre.Pending))
	for _, assessment := range pre.Pending {
		application := assessment.Candidate.Application
		if application.AmountCent <= 0 {
			return s.parkTccBatchManual(ctx, batch, "TCC_REFUND_ITEM_AMOUNT_INVALID",
				pre.details(map[string]any{
					"refund_id": application.RefundID, "group_no": application.GroupNo,
					"amount_cent": application.AmountCent,
				}))
		}
		items = append(items, &pb.RefundBatchItem{
			RefundId: application.RefundID, AmountCent: application.AmountCent,
		})
	}
	request := &pb.RefundBatchRequest{
		PaymentNo:         batch.PaymentNo,
		BatchId:           batch.BatchID,
		ExpectedTotalCent: pre.Outstanding,
		Items:             items,
		Reason:            truncate(tccRefundReasonPrefix+batch.BatchID, 255),
	}
	reply, err := s.payments.RefundBatch(ctx, request)
	if err != nil {
		return s.failTccRefundBatch(ctx, batch, pre, request, err)
	}
	if err := validateTccRefundBatchReply(reply, request); err != nil {
		return s.parkTccBatchManual(ctx, batch, "REFUND_BATCH_REPLY_INVALID",
			pre.details(map[string]any{"cause": err.Error()}))
	}
	s.log.Info("tcc refund batch accepted",
		"order_no", batch.OrderNo, "payment_no", batch.PaymentNo, "batch_id", batch.BatchID,
		"expected_total_cent", pre.Outstanding, "items", len(items),
		"reply_status", reply.GetStatus(), "refunded_cent", reply.GetRefundedCent())
	// The payment.refunded events close the ledger; a lost event is repaired by
	// the GetRefundByKey preflight on the next retry. Never mark DONE here.
	return s.convergeTccBatch(ctx, batch.BatchID)
}

// planTccRefundCandidates validates EVERY group before any remote call. A
// partial/incomplete admission or a corrupt ledger fails closed with zero
// payment at all.
func (s *Service) planTccRefundCandidates(ctx context.Context, batch *tccRefundBatch, applications []*tccApplicationRef) ([]*tccPaymentCandidate, error) {
	candidates := make([]*tccPaymentCandidate, 0, len(applications))
	for _, application := range applications {
		if application.State != refundApplicationProcessing && application.State != refundApplicationDone {
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
		case commitmentReserved, commitmentSettled:
		default:
			return nil, apperr.Failed("tcc refund commitment is not payable in its current state").
				WithDetails(map[string]any{
					"batch_id": batch.BatchID, "refund_id": application.RefundID,
					"group_no": application.GroupNo, "commitment_state": commitment.State,
				})
		}
		candidates = append(candidates, &tccPaymentCandidate{Application: application, Commitment: commitment})
	}
	return candidates, nil
}

// preflightTccRefund reads the Payment authority and classifies every group.
// It performs NO writes and NO Payment.RefundBatch: a gate failure returns
// *tccManualHold so the caller parks the batch with zero money movement.
func (s *Service) preflightTccRefund(ctx context.Context, batch *tccRefundBatch, candidates []*tccPaymentCandidate) (*tccRefundPreflight, error) {
	if s.payments == nil {
		return nil, apperr.Unavailable("payment client is required for tcc refund payment", nil)
	}
	intent, err := s.payments.GetIntent(ctx, &pb.GetIntentRequest{PaymentNo: batch.PaymentNo})
	if err != nil {
		if isPaymentReadTransient(err) {
			return nil, apperr.Unavailable("read payment intent for tcc refund", err).
				WithDetails(map[string]any{
					"batch_id": batch.BatchID, "payment_no": batch.PaymentNo,
				})
		}
		return nil, &tccManualHold{
			Reason: "TCC_REFUND_INTENT_READ_REJECTED",
			Details: map[string]any{
				"batch_id": batch.BatchID, "payment_no": batch.PaymentNo, "cause": err.Error(),
			},
		}
	}
	if intent == nil {
		return nil, &tccManualHold{
			Reason:  "TCC_REFUND_INTENT_MISSING",
			Details: map[string]any{"batch_id": batch.BatchID, "payment_no": batch.PaymentNo},
		}
	}
	captured := intent.GetCapturedCent()
	refunded := intent.GetRefundedCent()
	if captured < 0 || refunded < 0 || refunded > captured {
		return nil, &tccManualHold{
			Reason: "TCC_REFUND_INTENT_INVALID",
			Details: map[string]any{
				"batch_id": batch.BatchID, "payment_no": batch.PaymentNo,
				"captured_cent": captured, "refunded_cent": refunded,
			},
		}
	}
	pre := &tccRefundPreflight{CapturedCent: captured, RemainingCent: captured - refunded}
	for _, candidate := range candidates {
		assessment := &tccRefundAssessment{Candidate: candidate}
		fact, err := s.getTccRefundFact(ctx, batch.PaymentNo, candidate.Application.RefundID)
		if err != nil {
			var manual *tccManualHold
			if errors.As(err, &manual) {
				return nil, &tccManualHold{
					Reason:  manual.Reason,
					Details: mergeTccDetails(manual.Details, pre.details(nil)),
				}
			}
			return nil, err
		}
		assessment.Fact = fact
		assessment.Class, assessment.Reason = classifyTccRefund(candidate, fact)
		assessment.Reconcile = assessment.Class == tccRefundDone &&
			(candidate.Commitment.State != commitmentSettled || candidate.Application.State != refundApplicationDone)
		switch assessment.Class {
		case tccRefundDone:
			pre.DoneCent += candidate.Application.AmountCent
			pre.Done = append(pre.Done, assessment)
		case tccRefundInFlight:
			pre.InFlightCent += candidate.Application.AmountCent
			pre.InFlight = append(pre.InFlight, assessment)
		case tccRefundOutstanding:
			pre.Outstanding += candidate.Application.AmountCent
			pre.Pending = append(pre.Pending, assessment)
		default:
			pre.UnknownCent += candidate.Application.AmountCent
			pre.Uncertain = append(pre.Uncertain, assessment)
		}
	}

	// The frozen cap B is the batch's fact: never recompute it from the
	// (possibly partially refunded) current group caps.
	sum := pre.DoneCent + pre.InFlightCent + pre.Outstanding
	if batch.SumGroupCapCent != sum {
		return nil, &tccManualHold{
			Reason: "TCC_REFUND_BATCH_SUM_MISMATCH",
			Details: mergeTccDetails(pre.details(map[string]any{
				"sum_group_cap_cent":  batch.SumGroupCapCent,
				"classified_sum_cent": sum,
			}), uncertainDetails(pre)),
		}
	}
	if pre.UnknownCent != 0 {
		return nil, &tccManualHold{
			Reason:  "TCC_REFUND_UNKNOWN_GROUP_STATE",
			Details: mergeTccDetails(pre.details(nil), uncertainDetails(pre)),
		}
	}
	// R must equal exactly the part of this batch that is not yet done. A
	// smaller R is an external/extra refund; a larger R is extra capture or a
	// ledger mismatch. Both are manual: retrying could pay a group twice.
	if expected := batch.SumGroupCapCent - pre.DoneCent; pre.RemainingCent != expected {
		return nil, &tccManualHold{
			Reason: "TCC_REFUND_REMAINING_MISMATCH",
			Details: pre.details(map[string]any{
				"sum_group_cap_cent":      batch.SumGroupCapCent,
				"expected_remaining_cent": expected,
				"external_delta_cent":     pre.RemainingCent - expected,
			}),
		}
	}
	return pre, nil
}

// getTccRefundFact reads one Payment refund fact. NotFound is the legal
// "outstanding" answer; a transport failure is retryable; any other
// deterministic rejection is a manual-hold.
func (s *Service) getTccRefundFact(ctx context.Context, paymentNo, refundID string) (*pb.Refund, error) {
	fact, err := s.payments.GetRefundByKey(ctx, &pb.GetRefundByKeyRequest{
		PaymentNo: paymentNo, RefundId: refundID,
	})
	if err == nil {
		if fact == nil {
			return nil, &tccManualHold{
				Reason:  "TCC_REFUND_FACT_MISSING",
				Details: map[string]any{"payment_no": paymentNo, "refund_id": refundID},
			}
		}
		if fact.GetRefundId() != refundID {
			return nil, &tccManualHold{
				Reason: "TCC_REFUND_FACT_KEY_MISMATCH",
				Details: map[string]any{
					"payment_no": paymentNo, "want_refund_id": refundID,
					"refund_id": fact.GetRefundId(),
				},
			}
		}
		if fact.GetAmountCent() <= 0 {
			return nil, &tccManualHold{
				Reason: "TCC_REFUND_FACT_AMOUNT_INVALID",
				Details: map[string]any{
					"payment_no": paymentNo, "refund_id": refundID, "amount_cent": fact.GetAmountCent(),
				},
			}
		}
		return fact, nil
	}
	if apperr.IsCode(err, apperr.CodeNotFound) || grpcCode(err) == codes.NotFound {
		return nil, nil
	}
	if isPaymentReadTransient(err) {
		return nil, apperr.Unavailable("read payment refund fact for tcc refund", err).
			WithDetails(map[string]any{"payment_no": paymentNo, "refund_id": refundID})
	}
	return nil, &tccManualHold{
		Reason: "TCC_REFUND_FACT_READ_REJECTED",
		Details: map[string]any{
			"payment_no": paymentNo, "refund_id": refundID, "cause": err.Error(),
		},
	}
}

// classifyTccRefund maps one (application, commitment, Payment refund) triple
// to exactly one class. A successful Payment fact is done even when the Order
// ledger lags (lost event); application DONE without a Payment fact is unknown
// and never guessed as done (C-04 §3).
func classifyTccRefund(candidate *tccPaymentCandidate, fact *pb.Refund) (tccRefundClass, string) {
	application, commitment := candidate.Application, candidate.Commitment
	if fact != nil {
		if fact.GetAmountCent() != application.AmountCent {
			return tccRefundUnknown, "REFUND_AMOUNT_MISMATCH"
		}
		switch fact.GetStatus() {
		case tccRefundStatusSucceeded, tccRefundStatusRefunded:
			return tccRefundDone, ""
		case tccRefundStatusPending, tccRefundStatusProcessing:
			if commitment.State == commitmentSettled {
				// A settled commitment can only follow a successful refund;
				// a PENDING fact contradicts it, so never treat it as in-flight
				// (that would allow a retry of an already settled group).
				return tccRefundUnknown, "REFUND_IN_FLIGHT_BUT_COMMITMENT_SETTLED"
			}
			return tccRefundInFlight, ""
		default:
			return tccRefundUnknown, "REFUND_STATUS_UNRECOGNIZED"
		}
	}
	if commitment.State == commitmentSettled {
		// The payment fact may already be projected (event) even when the
		// GetRefundByKey read cannot see it; SETTLED is only written after a
		// successful Payment refund.
		return tccRefundDone, ""
	}
	if application.State == refundApplicationDone {
		return tccRefundUnknown, "APPLICATION_DONE_WITHOUT_PAYMENT_FACT"
	}
	if commitment.State == commitmentReserved && application.State == refundApplicationProcessing {
		return tccRefundOutstanding, ""
	}
	return tccRefundUnknown, "REFUND_APPLICATION_STATE_INCONSISTENT"
}

// reconcileTccDoneApplications repairs the Order ledger from successful Payment
// facts (lost payment.refunded events). It is idempotent and never moves money.
func (s *Service) reconcileTccDoneApplications(ctx context.Context, batch *tccRefundBatch, done []*tccRefundAssessment) error {
	for _, assessment := range done {
		if !assessment.Reconcile {
			continue
		}
		amount := assessment.Candidate.Application.AmountCent
		if assessment.Fact != nil {
			amount = assessment.Fact.GetAmountCent()
		}
		if err := s.reconcileTccRefundFact(ctx, batch, assessment.Candidate.Application, amount); err != nil {
			return err
		}
		s.log.Warn("tcc refund fact was not projected; reconciled the durable ledger",
			"order_no", batch.OrderNo, "payment_no", batch.PaymentNo, "batch_id", batch.BatchID,
			"refund_id", assessment.Candidate.Application.RefundID,
			"amount_cent", amount)
	}
	return nil
}

// reconcileTccRefundFact applies one already-successful Payment refund fact to
// the ledger: commitment RESERVED->SETTLED, application->DONE and (only when
// every group is DONE) batch PAYING->DONE. It never touches the order amount
// projection and never re-pays.
func (s *Service) reconcileTccRefundFact(ctx context.Context, batch *tccRefundBatch, application *tccApplicationRef, amountCent int64) error {
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		locked, err := findOrderForUpdate(ctx, tx, batch.OrderNo)
		if err != nil {
			return apperr.Internal("lock order for tcc refund reconciliation", err)
		}
		if locked == nil {
			return apperr.NotFound("order not found for tcc refund reconciliation")
		}
		switch locked.RefundState {
		case refundStateRefunding, refundStatePartial, refundStateFull:
			// The amount projection may already be ahead; the ledger write is
			// still independent and idempotent.
		default:
			return &tccManualHold{
				Reason: "TCC_REFUND_RECONCILE_ORDER_STATE",
				Details: map[string]any{
					"order_no": batch.OrderNo, "refund_state": locked.RefundState,
					"refund_id": application.RefundID,
				},
			}
		}
		event := paymentRefundedEvent{
			RefundID: application.RefundID, PaymentNo: batch.PaymentNo,
			OrderNo: batch.OrderNo, AmountCent: amountCent,
		}
		if err := s.settlePaymentRefundLedgerTx(ctx, tx, event); err != nil {
			if apperr.IsCode(err, apperr.CodeFailedPrecondition) ||
				apperr.IsCode(err, apperr.CodeConflict) ||
				apperr.IsCode(err, apperr.CodeNotFound) {
				return &tccManualHold{
					Reason: "TCC_REFUND_RECONCILE_LEDGER_CONFLICT",
					Details: map[string]any{
						"order_no": batch.OrderNo, "refund_id": application.RefundID,
						"amount_cent": amountCent, "cause": err.Error(),
					},
				}
			}
			return apperr.Internal("reconcile tcc refund ledger", err)
		}
		return nil
	})
	return err
}

// markTccBatchPaying moves ADMITTED -> PAYING before the first money-moving
// call. A concurrent completion is reported as errTccBatchDone so the caller
// can return idempotent success.
var errTccBatchDone = errors.New("tcc refund batch already completed")

func (s *Service) markTccBatchPaying(ctx context.Context, batch *tccRefundBatch) error {
	if batch.State == refundBatchPaying {
		return nil
	}
	if batch.State != refundBatchAdmitted {
		return apperr.Failed("tcc refund batch cannot enter PAYING from its current state").
			WithDetails(map[string]any{"batch_id": batch.BatchID, "state": batch.State})
	}
	if err := s.casTccBatchState(ctx, batch.BatchID, refundBatchAdmitted, refundBatchPaying); err != nil {
		current, loadErr := findActiveTccBatch(ctx, s.db, batch.OrderNo, batch.PaymentNo)
		if loadErr != nil {
			return loadErr
		}
		if current == nil {
			return errTccBatchDone
		}
		if current.State != refundBatchPaying {
			return err
		}
		batch.State = current.State
		return nil
	}
	batch.State = refundBatchPaying
	return nil
}

// parkTccBatchManual is the only automatic writer of MANUAL_HOLD. It is
// idempotent, never overwrites DONE, and returns the manual-hold error the
// caller must surface. It is called only before/without moving money.
func (s *Service) parkTccBatchManual(ctx context.Context, batch *tccRefundBatch, reason string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	details["tcc_error"] = tccErrBatchManual
	details["reason"] = reason
	details["batch_id"] = batch.BatchID
	details["order_no"] = batch.OrderNo
	details["payment_no"] = batch.PaymentNo
	details["batch_state"] = batch.State

	if batch.State == refundBatchAdmitted || batch.State == refundBatchPaying {
		if err := s.casTccBatchState(ctx, batch.BatchID, batch.State, refundBatchManualHold); err != nil {
			var state string
			if loadErr := s.db.QueryRowContext(ctx, `SELECT state FROM refund_batches WHERE batch_id = ?`,
				batch.BatchID).Scan(&state); loadErr != nil {
				return apperr.Internal("reload tcc refund batch for manual hold", loadErr)
			}
			switch state {
			case refundBatchManualHold:
				// A concurrent worker already parked it; surface the same error.
			case refundBatchDone:
				// A concurrent completion won; nothing is payable any more.
				return nil
			default:
				return err
			}
		}
		batch.State = refundBatchManualHold
	}
	s.log.Error("tcc refund batch requires manual handling",
		"order_no", batch.OrderNo, "payment_no", batch.PaymentNo,
		"batch_id", batch.BatchID, "reason", reason, "details", details)
	return apperr.Failed("tcc refund batch requires manual handling: " + reason).WithDetails(details)
}

// failTccRefundBatch maps a RefundBatch failure. BATCH_UNAVAILABLE (and any
// transport failure) keeps PAYING for an idempotent retry; every deterministic
// C-04 rejection parks the batch for a human.
func (s *Service) failTccRefundBatch(ctx context.Context, batch *tccRefundBatch, pre *tccRefundPreflight, request *pb.RefundBatchRequest, cause error) error {
	reason, deterministic := classifyTccRefundBatchError(cause)
	mapped := cause
	var domain *apperr.Error
	if !errors.As(cause, &domain) {
		mapped = remoteError("refund batch", cause)
	}
	details := pre.details(map[string]any{
		"reason":              reason,
		"cause":               mapped.Error(),
		"expected_total_cent": request.GetExpectedTotalCent(),
		"items":               len(request.GetItems()),
	})
	if !deterministic || reason == refundBatchReasonUnavailable {
		s.log.Error("tcc refund batch failed; batch remains PAYING for retry",
			"order_no", batch.OrderNo, "payment_no", batch.PaymentNo, "batch_id", batch.BatchID,
			"reason", reason, "error", mapped)
		return apperr.Unavailable("tcc refund batch failed; retry the compensation", mapped).
			WithDetails(details)
	}
	return s.parkTccBatchManual(ctx, batch, reason, details)
}

// classifyTccRefundBatchError extracts C-04's details.reason and decides
// transient vs deterministic. A raw gRPC error is classified by its status
// code because remoteError deliberately folds unknown codes into UNAVAILABLE.
func classifyTccRefundBatchError(err error) (string, bool) {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		reason, _ := ae.Details["reason"].(string)
		if reason == refundBatchReasonUnavailable {
			return reason, false
		}
		switch ae.Code {
		case apperr.CodeUnavailable, apperr.CodeInternal, apperr.CodeRateLimited:
			return reason, false
		default:
			return reason, true
		}
	}
	if st, ok := status.FromError(err); ok {
		reason := ""
		for _, detail := range st.Details() {
			if info, ok := detail.(*errdetails.ErrorInfo); ok {
				reason = info.Metadata["reason"]
			}
		}
		if reason == refundBatchReasonUnavailable {
			return reason, false
		}
		switch st.Code() {
		case codes.InvalidArgument, codes.FailedPrecondition, codes.AlreadyExists, codes.Aborted,
			codes.NotFound, codes.PermissionDenied, codes.OutOfRange:
			return reason, true
		}
	}
	return "", false
}

// validateTccRefundBatchReply proves Payment answered exactly the requested
// all-or-none batch: same batch_id, same items/amounts, expected total, valid
// per-item status. Anything else is a state inconsistency, never a success.
func validateTccRefundBatchReply(reply *pb.RefundBatchReply, request *pb.RefundBatchRequest) error {
	if reply == nil {
		return errors.New("refund batch reply is empty")
	}
	if reply.GetBatchId() != request.GetBatchId() {
		return fmt.Errorf("refund batch reply batch_id %q, want %q", reply.GetBatchId(), request.GetBatchId())
	}
	switch reply.GetStatus() {
	case refundBatchStatusRefunded, refundBatchStatusAlreadyRefunded:
	default:
		return fmt.Errorf("refund batch reply status %q is not REFUNDED/ALREADY_REFUNDED", reply.GetStatus())
	}
	if reply.GetRefundedCent() != request.GetExpectedTotalCent() {
		return fmt.Errorf("refund batch reply refunded_cent %d, want %d",
			reply.GetRefundedCent(), request.GetExpectedTotalCent())
	}
	results := reply.GetItemResults()
	if len(results) != len(request.GetItems()) {
		return fmt.Errorf("refund batch reply has %d item results for %d items", len(results), len(request.GetItems()))
	}
	want := make(map[string]int64, len(request.GetItems()))
	for _, item := range request.GetItems() {
		want[item.GetRefundId()] = item.GetAmountCent()
	}
	seen := make(map[string]struct{}, len(results))
	for _, result := range results {
		refundID := result.GetRefundId()
		amount, ok := want[refundID]
		if !ok {
			return fmt.Errorf("refund batch reply contains unrequested refund_id %q", refundID)
		}
		if _, duplicate := seen[refundID]; duplicate {
			return fmt.Errorf("refund batch reply repeats refund_id %q", refundID)
		}
		seen[refundID] = struct{}{}
		if result.GetAmountCent() != amount {
			return fmt.Errorf("refund batch reply refund_id %q amount %d, want %d",
				refundID, result.GetAmountCent(), amount)
		}
		switch result.GetStatus() {
		case refundBatchStatusRefunded, refundBatchStatusAlreadyRefunded:
		default:
			return fmt.Errorf("refund batch reply refund_id %q has status %q", refundID, result.GetStatus())
		}
		if strings.TrimSpace(result.GetRefundNo()) == "" {
			return fmt.Errorf("refund batch reply refund_id %q has no refund_no", refundID)
		}
	}
	for refundID := range want {
		if _, ok := seen[refundID]; !ok {
			return fmt.Errorf("refund batch reply is missing refund_id %q", refundID)
		}
	}
	return nil
}

// tccRefundInFlightTimedOut is the local fail-safe for a refund that never
// leaves PENDING/PROCESSING. A missing created_at is not treated as a timeout
// (it stays retryable rather than guessing age).
func (s *Service) tccRefundInFlightTimedOut(fact *pb.Refund) bool {
	if fact == nil || fact.GetCreatedAtUnix() <= 0 {
		return false
	}
	created := time.Unix(fact.GetCreatedAtUnix(), 0)
	return s.now().Sub(created) > tccRefundInFlightTimeout
}

// isPaymentReadTransient reports whether a Payment read error may be retried
// without changing Order state. Unknown/raw errors are treated as transient:
// fail-closed here means "do not pay now", not "park a human".
func isPaymentReadTransient(err error) bool {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		switch ae.Code {
		case apperr.CodeUnavailable, apperr.CodeInternal, apperr.CodeRateLimited:
			return true
		default:
			return false
		}
	}
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unavailable, codes.Internal, codes.Unknown, codes.DeadlineExceeded,
			codes.ResourceExhausted, codes.Aborted:
			return true
		default:
			return false
		}
	}
	return true
}

func grpcCode(err error) codes.Code {
	if st, ok := status.FromError(err); ok {
		return st.Code()
	}
	return codes.OK
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

// details renders the preflight numbers for logs/errors/manual-hold records.
func (p *tccRefundPreflight) details(extra map[string]any) map[string]any {
	out := map[string]any{
		"captured_cent":    p.CapturedCent,
		"remaining_cent":   p.RemainingCent,
		"done_cent":        p.DoneCent,
		"in_flight_cent":   p.InFlightCent,
		"outstanding_cent": p.Outstanding,
		"unknown_cent":     p.UnknownCent,
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func mergeTccDetails(base, extra map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for key, value := range extra {
		base[key] = value
	}
	return base
}

func uncertainDetails(pre *tccRefundPreflight) map[string]any {
	if len(pre.Uncertain) == 0 {
		return nil
	}
	groups := make([]map[string]any, 0, len(pre.Uncertain))
	for _, assessment := range pre.Uncertain {
		groups = append(groups, map[string]any{
			"refund_id":        assessment.Candidate.Application.RefundID,
			"group_no":         assessment.Candidate.Application.GroupNo,
			"state":            assessment.Candidate.Application.State,
			"commitment_state": assessment.Candidate.Commitment.State,
			"reason":           assessment.Reason,
		})
	}
	return map[string]any{"unknown_groups": groups}
}
