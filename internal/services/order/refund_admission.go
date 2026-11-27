package order

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/outbox"
)

const (
	refundResultPre  = "PRE_SHIP_REVOKED"
	refundResultPost = "POST_SHIP_ALLOWED"
	refundPending    = "PENDING_REMOTE"
	topicRefundAdmit = "shop.order.refund_admitted.v1"

	// refundReviewRequired is the bounded review decision (SPEC-0002 v1.11
	// AC-69/AC-70; ADR-0007 Amd1 AC-50 Addendum 0.6 §3.1). It is NOT an
	// admission result and NOT a terminal state: result=REVIEW_REQUIRED keeps
	// state=PENDING_REMOTE.
	refundReviewRequired = "REVIEW_REQUIRED"
	// probeStatusNoWaybill mirrors the Fulfillment CancelIfCreated value. The
	// Order caller must distinguish "no waybill row" from a real CANCELLED row;
	// it never imports the fulfillment package (cross-service boundary).
	probeStatusNoWaybill = "NO_WAYBILL"
	// refundReasonConsumedWithoutShipment is one of the fixed AC-72 reasons.
	refundReasonConsumedWithoutShipment = "CONSUMED_WITHOUT_SHIPMENT"
	refundReasonPartialUnclassifiable   = "PARTIAL_AMOUNT_UNCLASSIFIABLE"
	refundReasonLegacyShipmentUnknown   = "LEGACY_SHIPMENT_UNKNOWN"
	manualQueueNowb                     = "AC-27-NOWB-01"
	// refundBatchLastErrorWidth is the TCC refund_batches.last_error column
	// width (characters); the after-sales task column is 512.
	refundBatchLastErrorWidth = 1024

	// refundSourceSystemZero is the AC-67/AC-68 zero-amount after-sale source.
	// It is the only source that takes the zero short-circuit; a zero-amount
	// request from any other source keeps the pre-existing positive path.
	refundSourceSystemZero = "system_zero"
	// The zero short-circuit writes the refund_applications DONE state in the
	// same transaction; a TCC group admitted and waiting for the T1.4b payment
	// writes PROCESSING (see tcc_settlement.go for the canonical state names).
)

type refundAdmissionRow struct {
	RefundID   string
	OrderNo    string
	GroupNo    string
	Result     string
	State      string
	AmountCent int64
	Version    int64
}

func findRefundAdmission(ctx context.Context, q querier, refundID string) (*refundAdmissionRow, error) {
	return findRefundAdmissionQuery(ctx, q, refundID, false)
}

// findRefundAdmissionLocked serializes concurrent admissions of one refund_id
// on the admission row (the caller already holds the order row lock).
func findRefundAdmissionLocked(ctx context.Context, q querier, refundID string) (*refundAdmissionRow, error) {
	return findRefundAdmissionQuery(ctx, q, refundID, true)
}

func findRefundAdmissionQuery(ctx context.Context, q querier, refundID string, lock bool) (*refundAdmissionRow, error) {
	query := `SELECT refund_id, order_no, group_no, result, state, amount_cent, version
		FROM order_refund_admissions WHERE refund_id = ?`
	if lock {
		query += " FOR UPDATE"
	}
	var row refundAdmissionRow
	err := q.QueryRowContext(ctx, query, refundID).
		Scan(&row.RefundID, &row.OrderNo, &row.GroupNo, &row.Result, &row.State, &row.AmountCent, &row.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// refundApplicationRow mirrors one refund_applications ledger row.
type refundApplicationRow struct {
	RefundID        string
	BatchID         string
	Source          string
	OrderNo         string
	GroupNo         string
	NonEffective    int
	AmountCent      int64
	GoodsCent       int64
	FreightCent     int64
	State           string
	AdmissionResult string
	Version         int64
}

func findRefundApplication(ctx context.Context, q querier, refundID string, lock bool) (*refundApplicationRow, error) {
	query := `SELECT refund_id, COALESCE(batch_id,''), source, order_no, COALESCE(group_no,''),
		non_effective_capture, amount_cent, goods_cent, freight_cent, state, admission_result, version
		FROM refund_applications WHERE refund_id = ?`
	if lock {
		query += " FOR UPDATE"
	}
	var row refundApplicationRow
	err := q.QueryRowContext(ctx, query, refundID).Scan(&row.RefundID, &row.BatchID, &row.Source, &row.OrderNo,
		&row.GroupNo, &row.NonEffective, &row.AmountCent, &row.GoodsCent, &row.FreightCent,
		&row.State, &row.AdmissionResult, &row.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// loadPermitState reads the group permit without creating it. "" means no
// permit row yet (an unshipped group that never requested shipment).
func loadPermitState(ctx context.Context, q querier, orderNo, groupNo string) (string, error) {
	var state string
	err := q.QueryRowContext(ctx, `SELECT state FROM order_shipping_permits
		WHERE order_no = ? AND fulfillment_group_no = ?`, orderNo, permitGroup(groupNo)).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return state, nil
}

func admissionReply(row *refundAdmissionRow) *pb.AdmitRefundReply {
	return &pb.AdmitRefundReply{
		RefundId: row.RefundID, Result: row.Result, State: row.State,
		AmountCent: row.AmountCent, Version: row.Version,
	}
}

// existingAdmissionClass is the single classifier for an already persisted
// admission row (ADR R-4): terminal / review / pending / unknown. Every
// existing-row decision goes through it so no caller role invents its own
// if/else ladder.
type existingAdmissionClass int

const (
	existingAdmissionNone existingAdmissionClass = iota
	existingAdmissionTerminal
	existingAdmissionReview
	existingAdmissionPending
	existingAdmissionUnknown
)

func classifyExistingAdmission(row *refundAdmissionRow) existingAdmissionClass {
	if row == nil {
		return existingAdmissionNone
	}
	if row.Result == refundReviewRequired {
		if row.State == refundPending {
			return existingAdmissionReview
		}
		return existingAdmissionUnknown
	}
	if row.State == row.Result && (row.Result == refundResultPre || row.Result == refundResultPost) {
		return existingAdmissionTerminal
	}
	if row.State == refundPending {
		return existingAdmissionPending
	}
	return existingAdmissionUnknown
}

func refundReviewTicket(refundID string) string { return "RVW:" + refundID }

// refundReviewCanonical builds the AC-72/D-35 canonical mirror text
// `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>`; if it does not fit
// the column it keeps the prefix and truncates <refund_id>, appending
// ` truncated=1`. The reason/ticket keys are never truncated.
func refundReviewCanonical(refundID, code string, width int) string {
	prefix := "REVIEW_REQUIRED reason=" + code + " ticket=RVW:"
	if utf8.RuneCountInString(prefix+refundID) <= width {
		return prefix + refundID
	}
	limit := width - utf8.RuneCountInString(prefix) - utf8.RuneCountInString(" truncated=1")
	if limit < 1 {
		limit = 1
	}
	runes := []rune(refundID)
	if limit < len(runes) {
		runes = runes[:limit]
	}
	return prefix + string(runes) + " truncated=1"
}

// refundReviewConflict is the stable REFUND_ADMISSION_CONFLICT for a
// REVIEW_REQUIRED decision with structured (typed) details; callers must not
// parse the message. The error code table is unchanged (AC-69/AC-72).
func refundReviewConflict(row *refundAdmissionRow, reason, probeStatus string) *apperr.Error {
	details := map[string]any{
		"refund_id": row.RefundID, "order_no": row.OrderNo, "group_no": row.GroupNo,
		"result": refundReviewRequired, "state": refundPending,
		"reason": reason, "ticket": refundReviewTicket(row.RefundID),
		"manual_queue": manualQueueNowb,
	}
	message := "REFUND_ADMISSION_CONFLICT: refund requires manual review"
	if probeStatus != "" {
		details["probe_status"] = probeStatus
		details["permit_state"] = permitConsumed
		message = "REFUND_ADMISSION_CONFLICT: consumed permit without shipment; manual review required"
	}
	return apperr.Failed(message).WithDetails(details)
}

// replayReviewConflict reconstructs the stable details of a persisted review
// from Order-owned facts (the admission row stores no reason column).
func (s *Service) replayReviewConflict(ctx context.Context, row *refundAdmissionRow) *apperr.Error {
	if remaining, err := loadGroupRemainingExcluding(ctx, s.db, row.OrderNo, row.GroupNo, row.RefundID); err == nil && row.AmountCent < remaining {
		return refundReviewConflict(row, refundReasonPartialUnclassifiable, "")
	}
	permit, err := loadPermitState(ctx, s.db, row.OrderNo, row.GroupNo)
	if err == nil && permit == permitConsumed {
		return refundReviewConflict(row, refundReasonConsumedWithoutShipment, probeStatusNoWaybill)
	}
	return refundReviewConflict(row, refundReasonLegacyShipmentUnknown, "")
}

func partialRefundError(groupRemaining, requested int64) *apperr.Error {
	return apperr.Failed("partial refund of an unshipped group requires a full-group refund").
		WithDetails(map[string]any{"group_remaining_cent": groupRemaining, "requested_cent": requested})
}

// orderExitClass is the Order-owned waybill-exit proxy used by AC-70: the
// group projection plus (when the group row is missing) the order status.
type orderExitClass int

const (
	exitUnshipped orderExitClass = iota
	exitShipped
	exitUnclassifiable
)

func classifyGroupExit(ctx context.Context, q querier, orderNo, groupNo, orderStatus string) (orderExitClass, error) {
	var status string
	err := q.QueryRowContext(ctx, `SELECT status FROM order_fulfillment_groups
		WHERE order_no = ? AND group_no = ?`, orderNo, groupNo).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		switch orderStatus {
		case statusPaid, statusAllocated:
			return exitUnshipped, nil
		case statusShipped, statusCompleted:
			return exitUnclassifiable, nil
		default:
			return exitUnclassifiable, nil
		}
	}
	if err != nil {
		return 0, apperr.Unavailable("load group projection for refund admission", err)
	}
	switch status {
	case "SHIPPED", "RECEIVED":
		return exitShipped, nil
	case "PAID", "ALLOCATED":
		return exitUnshipped, nil
	default:
		return exitUnclassifiable, nil
	}
}

func loadGroupRemaining(ctx context.Context, q querier, orderNo, groupNo string) (int64, error) {
	return loadGroupRemainingExcluding(ctx, q, orderNo, groupNo, "")
}

// loadGroupRemainingExcluding excludes one refund's own RESERVED commitment so
// a preseeded partial admission is not mistaken for a whole-group request.
func loadGroupRemainingExcluding(ctx context.Context, q querier, orderNo, groupNo, excludeRefundID string) (int64, error) {
	var paid, freight, consumed int64
	if err := q.QueryRowContext(ctx, `SELECT COALESCE(SUM(line_paid_cent),0) FROM order_items
		WHERE order_no = ? AND fulfillment_group_no = ?`, orderNo, groupNo).Scan(&paid); err != nil {
		return 0, apperr.Internal("sum group paid amount", err)
	}
	if err := q.QueryRowContext(ctx, `SELECT COALESCE(SUM(freight_share_cent),0) FROM order_fulfillment_groups
		WHERE order_no = ? AND group_no = ?`, orderNo, groupNo).Scan(&freight); err != nil {
		return 0, apperr.Internal("load group freight share", err)
	}
	if err := q.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cent),0) FROM order_refund_commitments
		WHERE order_no = ? AND group_no = ? AND state IN ('RESERVED','SETTLED') AND refund_id <> ?`,
		orderNo, groupNo, excludeRefundID).Scan(&consumed); err != nil {
		return 0, apperr.Internal("sum group refund commitments", err)
	}
	remaining := paid + freight - consumed
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

func loadOrderStatus(ctx context.Context, q querier, orderNo string) (string, error) {
	var status string
	if err := q.QueryRowContext(ctx, `SELECT status FROM orders WHERE order_no = ?`, orderNo).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", apperr.NotFound("order not found")
		}
		return "", apperr.Internal("load order status", err)
	}
	return status, nil
}

// AdmitRefund is the group-level refund admission described by ADR-0007. It
// serializes admissions on the order row and the permit row, never moves money,
// and is idempotent by refund_id. CONSUMED permits take the two-phase
// PENDING_REMOTE -> CancelIfCreated -> Tx2 path.
func (s *Service) AdmitRefund(ctx context.Context, req *pb.AdmitRefundRequest) (*pb.AdmitRefundReply, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	if req.GetAmountCent() < 0 {
		return nil, apperr.Invalid("amount_cent must not be negative")
	}
	groupNo := permitGroup(req.GetGroupNo())

	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order for refund admission", err)
	}
	if record == nil {
		return nil, apperr.NotFound("order not found")
	}
	refundID := strings.TrimSpace(req.GetRefundId())
	if refundID == "" {
		refundID, err = s.allocateLegacyRefundID(ctx, orderNo, req.GetClientRequestId())
		if err != nil {
			return nil, err
		}
	}
	if record.Status != statusPaid && record.Status != statusAllocated && record.Status != statusShipped && record.Status != statusCompleted {
		return nil, apperr.Failed("order cannot accept refunds in status " + record.Status)
	}
	// AC-67/AC-68/INV-50: a `system_zero` refund is the zero-amount execution
	// fact. It never consults Payment, quota or commitment; it only performs
	// the group admission (permit / CancelIfCreated) and writes the DONE ledger
	// row in the same transaction. The positive-amount path below is unchanged.
	if strings.TrimSpace(req.GetSource()) == refundSourceSystemZero {
		// T2.4 zero gate (Reviewer MAJOR): a historical zero order whose
		// settlement identity is still empty must resolve to zero:<order_no>
		// (or queued) before the zero short-circuit. A captured intent on a
		// zero-pay order must never take the zero path; do not collapse the
		// unresolved case into REFUND_ADMISSION_CONFLICT.
		if strings.TrimSpace(record.SettlementPaymentNo) == "" {
			resolved, rerr := s.ensureLegacyResolution(ctx, orderNo)
			if rerr != nil {
				return nil, rerr
			}
			if resolved == nil || resolved.State != legacyResolutionStateExpiredZero {
				details := map[string]any{"order_no": orderNo, "reason": "LEGACY_PAYMENT_UNRESOLVED"}
				if resolved != nil {
					details["resolution_state"] = resolved.State
					details["candidate_count"] = resolved.CandidateCount
				}
				return nil, apperr.Failed("zero-amount refund requires a resolved historical payment").
					WithDetails(details)
			}
			reloaded, lerr := findOrder(ctx, s.db, orderNo)
			if lerr != nil {
				return nil, apperr.Internal("reload zero-resolved order", lerr)
			}
			if reloaded == nil {
				return nil, apperr.NotFound("order not found")
			}
			record = reloaded
		}
		return s.admitZeroRefund(ctx, req, record, refundID, orderNo, groupNo)
	}
	// AC-69 boundedness: the persisted decision is read before any Payment read
	// and before any remote probe. A REVIEW_REQUIRED row returns the same
	// conflict; a terminal row returns its result without consulting Payment.
	if existing, err := findRefundAdmission(ctx, s.db, refundID); err != nil {
		return nil, apperr.Internal("load refund admission", err)
	} else {
		switch classifyExistingAdmission(existing) {
		case existingAdmissionReview:
			return nil, s.replayReviewConflict(ctx, existing)
		case existingAdmissionTerminal:
			return admissionReply(existing), nil
		case existingAdmissionPending:
			return s.resumePendingAdmission(ctx, existing, req)
		case existingAdmissionUnknown:
			return nil, apperr.Conflict("refund admission has an illegal result/state combination").
				WithDetails(map[string]any{
					"refund_id": refundID, "result": existing.Result, "state": existing.State,
				})
		}
	}

	var captured, refunded int64
	if strings.HasPrefix(record.SettlementPaymentNo, "zero:") {
		// Zero-amount orders have no Payment intent; their captured amount is
		// zero by definition and the group cap is zero.
	} else {
		if s.payments == nil {
			return nil, apperr.Unavailable("payment client is required for refund admission", nil)
		}
		if record.SettlementPaymentNo == "" {
			// T2.2/T2.3 boundary: resolve the historical payment before any
			// amount read. A unique candidate is CAS-written and continues; a
			// zero/no/multiple candidate order is queued and fails closed.
			resolved, rerr := s.ensureLegacyResolution(ctx, orderNo)
			if rerr != nil {
				return nil, rerr
			}
			if resolved == nil || (resolved.State != legacyResolutionStateResolved &&
				resolved.State != legacyResolutionStateExpiredZero) {
				details := map[string]any{"order_no": orderNo, "reason": "LEGACY_PAYMENT_UNRESOLVED"}
				if resolved != nil {
					details["resolution_state"] = resolved.State
					details["candidate_count"] = resolved.CandidateCount
				}
				return nil, apperr.Failed("order has an unresolved historical payment; refund is blocked").
					WithDetails(details)
			}
			reloaded, lerr := findOrder(ctx, s.db, orderNo)
			if lerr != nil {
				return nil, apperr.Internal("reload legacy-resolved order", lerr)
			}
			if reloaded == nil || strings.TrimSpace(reloaded.SettlementPaymentNo) == "" {
				return nil, apperr.Failed("order has no settlement payment after legacy resolution")
			}
			record = reloaded
		}
		intent, ierr := s.payments.GetIntent(ctx, &pb.GetIntentRequest{PaymentNo: record.SettlementPaymentNo})
		if ierr != nil {
			return nil, apperr.Unavailable("load authoritative captured amount", ierr)
		}
		if intent.GetCapturedCent() <= 0 {
			return nil, apperr.Failed("effective payment has no captured amount")
		}
		captured, refunded = intent.GetCapturedCent(), intent.GetRefundedCent()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin refund admission", err)
	}
	defer func() { _ = tx.Rollback() }()
	locked, err := findOrderForUpdate(ctx, tx, orderNo)
	if err != nil {
		return nil, err
	}
	if locked == nil {
		return nil, apperr.NotFound("order not found")
	}
	// Two concurrent admissions of the same refund_id can both pass the
	// pre-transaction read. The order row lock serializes them, so the loser
	// must converge on the persisted admission instead of failing on the
	// primary key.
	if existing, err := findRefundAdmissionLocked(ctx, tx, refundID); err != nil {
		return nil, apperr.Internal("load refund admission under lock", err)
	} else if existing != nil {
		_ = tx.Rollback()
		switch classifyExistingAdmission(existing) {
		case existingAdmissionReview:
			return nil, s.replayReviewConflict(ctx, existing)
		case existingAdmissionTerminal:
			return admissionReply(existing), nil
		case existingAdmissionPending:
			return s.resumePendingAdmission(ctx, existing, req)
		default:
			return nil, apperr.Conflict("refund admission has an illegal result/state combination").
				WithDetails(map[string]any{
					"refund_id": refundID, "result": existing.Result, "state": existing.State,
				})
		}
	}
	if err := ensureGroupExistsTx(ctx, tx, orderNo, groupNo); err != nil {
		return nil, err
	}
	permit, err := ensurePermitTx(ctx, tx, orderNo, groupNo)
	if err != nil {
		return nil, err
	}
	var reserved int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cent),0) FROM order_refund_commitments
		WHERE order_no = ? AND state = ?`, orderNo, "RESERVED").Scan(&reserved); err != nil {
		return nil, apperr.Internal("sum refund commitments", err)
	}
	if req.GetAmountCent()+reserved > captured-refunded {
		return nil, apperr.Failed("refund exceeds the remaining captured amount").
			WithDetails(map[string]any{"captured_cent": captured, "refunded_cent": refunded, "reserved_cent": reserved})
	}
	groupRemaining, err := loadGroupRemaining(ctx, tx, orderNo, groupNo)
	if err != nil {
		return nil, err
	}
	if req.GetAmountCent() > groupRemaining {
		return nil, apperr.Failed("refund exceeds the group remaining amount").
			WithDetails(map[string]any{"group_remaining_cent": groupRemaining, "requested_cent": req.GetAmountCent()})
	}
	if permit.state != permitConsumed && req.GetAmountCent() != groupRemaining {
		return nil, partialRefundError(groupRemaining, req.GetAmountCent())
	}
	composition, err := validateRefundComposition(ctx, tx, req, orderNo, groupNo)
	if err != nil {
		return nil, err
	}

	// AC-50/AC-70/D-31: a CONSUMED partial amount and a legacy NONE permit use
	// the Order-owned group projection only; CancelIfCreated is never called for
	// them. The decision is made before any Tx1 write so PARTIAL leaves zero
	// admission/commitment/outbox rows.
	partial := permit.state == permitConsumed && req.GetAmountCent() < groupRemaining
	exitClass := exitUnshipped
	if partial || permit.state == permitConsumed || permit.state == permitNone {
		exitClass, err = classifyGroupExit(ctx, tx, orderNo, groupNo, locked.Status)
		if err != nil {
			return nil, err
		}
	}

	result, state := refundResultPre, refundResultPre
	reviewReason := ""
	needsProbe := false
	switch permit.state {
	case permitConsumed:
		switch {
		case partial && exitClass == exitUnshipped:
			return nil, partialRefundError(groupRemaining, req.GetAmountCent())
		case partial && exitClass == exitUnclassifiable:
			result, state = refundReviewRequired, refundPending
			reviewReason = refundReasonPartialUnclassifiable
		case partial:
			// Projection SHIPPED/RECEIVED: partial post-ship refund is legal.
			result, state = refundResultPost, refundResultPost
		case exitClass == exitShipped:
			// Whole-group but the projection already proves the exit: POST
			// without probing or cancelling the waybill (AC-50 §236).
			result, state = refundResultPost, refundResultPost
		default:
			// Tx1 placeholder + remote probe (existing two-phase path).
			result, state = refundResultPost, refundPending
			needsProbe = true
		}
	case permitNone:
		switch exitClass {
		case exitShipped:
			result, state = refundResultPost, refundResultPost
		case exitUnshipped:
			result, state = refundResultPre, refundResultPre
		default:
			result, state = refundReviewRequired, refundPending
			reviewReason = refundReasonLegacyShipmentUnknown
		}
	case permitRevoked:
		result, state = refundResultPre, refundResultPre
	default:
		return nil, apperr.Failed("shipping permit state is not a legal persisted state").
			WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo, "permit_state": permit.state})
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_admissions
		(refund_id, order_no, group_no, result, state, amount_cent, version)
		VALUES (?, ?, ?, ?, ?, ?, 1)`,
		refundID, orderNo, groupNo, result, state, req.GetAmountCent()); err != nil {
		return nil, apperr.Internal("insert refund admission", err)
	}
	if err := insertRefundCommitmentTx(ctx, tx, orderNo, groupNo, refundID, req.GetAmountCent(), composition); err != nil {
		return nil, err
	}
	if permit.state == permitNone && result == refundResultPre {
		res, err := tx.ExecContext(ctx, `UPDATE order_shipping_permits
			SET state = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND fulfillment_group_no = ? AND state = ?`,
			permitRevoked, orderNo, groupNo, permitNone)
		if err != nil {
			return nil, apperr.Internal("revoke permit for refund", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return nil, apperr.Conflict("shipping permit changed during refund admission; retry").
				WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo})
		}
	}
	if result != refundReviewRequired {
		if err := appendRefundAdmitted(ctx, tx, refundID, orderNo, groupNo, result, req.GetAmountCent(), 1, state); err != nil {
			return nil, err
		}
	}
	if result == refundResultPre {
		if err := s.appendRefundReleaseTx(ctx, tx, refundID, orderNo, groupNo, req.GetSource()); err != nil {
			return nil, err
		}
	}
	if result == refundReviewRequired && req.GetSource() == refundApplicationSourceTCC {
		// T1c gate: a fresh Tx1 REVIEW must apply the same human-hold writes as
		// Tx2. If the batch/application mirror matches 0 rows the whole Tx1
		// rolls back (no admission/commitment without MANUAL_HOLD).
		if err := persistTCCReviewTx(ctx, tx, refundID, req.GetBatchId(), reviewReason); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit refund admission", err)
	}
	row := &refundAdmissionRow{
		RefundID: refundID, OrderNo: orderNo, GroupNo: groupNo, Result: result,
		State: state, AmountCent: req.GetAmountCent(), Version: 1,
	}
	if result == refundReviewRequired {
		if s.log != nil {
			s.log.Error("refund admission review required",
				"refund_id", refundID, "order_no", orderNo, "group_no", groupNo,
				"source", req.GetSource(), "result", refundReviewRequired, "state", refundPending,
				"reason", reviewReason, "ticket", refundReviewTicket(refundID), "manual_queue", manualQueueNowb)
		}
		return nil, refundReviewConflict(row, reviewReason, "")
	}
	if needsProbe {
		return s.finishRemoteAdmission(ctx, row, req.GetSource(), req.GetBatchId())
	}
	return admissionReply(row), nil
}

// resumePendingAdmission resumes an existing PENDING_REMOTE admission. A whole
// group keeps the two-phase probe; a partial amount is classified by the
// Order-owned group projection without probing, and the existing placeholder/
// commitment are never re-created for PARTIAL (AC-70/D-31).
func (s *Service) resumePendingAdmission(ctx context.Context, row *refundAdmissionRow, req *pb.AdmitRefundRequest) (*pb.AdmitRefundReply, error) {
	remaining, err := loadGroupRemainingExcluding(ctx, s.db, row.OrderNo, row.GroupNo, row.RefundID)
	if err != nil {
		return nil, err
	}
	orderStatus, err := loadOrderStatus(ctx, s.db, row.OrderNo)
	if err != nil {
		return nil, err
	}
	class, err := classifyGroupExit(ctx, s.db, row.OrderNo, row.GroupNo, orderStatus)
	if err != nil {
		return nil, err
	}
	if row.AmountCent >= remaining {
		if class == exitShipped {
			// AC-50 §236: the projection already proves the exit; never probe.
			return s.writeAdmissionTx2(ctx, row, refundResultPost, refundResultPost, req.GetSource(), req.GetBatchId(), "", "")
		}
		return s.finishRemoteAdmission(ctx, row, req.GetSource(), req.GetBatchId())
	}
	if err != nil {
		return nil, err
	}
	switch class {
	case exitShipped:
		return s.writeAdmissionTx2(ctx, row, refundResultPost, refundResultPost, req.GetSource(), req.GetBatchId(), "", "")
	case exitUnshipped:
		return nil, partialRefundError(remaining, row.AmountCent)
	default:
		return s.writeAdmissionTx2(ctx, row, refundReviewRequired, refundPending, req.GetSource(), req.GetBatchId(), refundReasonPartialUnclassifiable, "")
	}
}

// admitZeroRefund implements SPEC-0002 v1.9 AC-67/AC-68 and the zero-amount
// addendum (AZ-01/AZ-04/AZ-05): `source=system_zero && amount=0` on a
// `pay_cent=0` order whose settlement identity is `zero:<order_no>` (that is
// the Order-side proof that Payment reported no captured intent) skips the
// Payment quota and the commitment, still performs the group admission, and
// writes `refund_applications(source=system_zero, amount=0, state=DONE)` in
// the same transaction that persists the admission result. It never calls
// Payment and never moves money.
func (s *Service) admitZeroRefund(ctx context.Context, req *pb.AdmitRefundRequest, record *orderRecord, refundID, orderNo, groupNo string) (*pb.AdmitRefundReply, error) {
	if req.GetAmountCent() != 0 {
		return nil, apperr.Failed("zero-amount refund requires amount_cent 0 (REFUND_ADMISSION_CONFLICT)").
			WithDetails(map[string]any{
				"refund_id": refundID, "order_no": orderNo, "amount_cent": req.GetAmountCent(),
			})
	}
	if record.PayCent != 0 {
		return nil, apperr.Failed("zero-amount refund requires a zero-amount order (REFUND_ADMISSION_CONFLICT)").
			WithDetails(map[string]any{
				"refund_id": refundID, "order_no": orderNo,
				"pay_cent": record.PayCent, "amount_cent": req.GetAmountCent(),
			})
	}
	if record.SettlementPaymentNo != zeroPaymentPrefix+record.OrderNo {
		// AC-40/AC-63: only the zero settlement identity proves "no captured
		// intent". A positive capture or an unresolved historical payment must
		// not be turned into a zero refund.
		return nil, apperr.Failed("zero-amount refund requires the zero:<order_no> settlement identity (REFUND_ADMISSION_CONFLICT)").
			WithDetails(map[string]any{
				"refund_id": refundID, "order_no": orderNo,
				"settlement_payment_no": record.SettlementPaymentNo, "pay_cent": record.PayCent,
			})
	}
	// Idempotency: the DONE execution fact is final.
	if app, err := findRefundApplication(ctx, s.db, refundID, false); err != nil {
		return nil, apperr.Internal("load zero-amount refund application", err)
	} else if app != nil {
		return s.zeroRefundReply(ctx, app, orderNo, groupNo)
	}
	// The remote cancel cannot run inside a transaction, so the admission
	// result (AC-50: NONE/REVOKED -> PRE, CONSUMED -> CancelIfCreated) is
	// decided first. A permit CAS lost to a concurrent Ship rolls back and
	// re-decides on the CONSUMED branch.
	for attempt := 0; attempt < 3; attempt++ {
		result, remote, err := s.decideZeroRefundResult(ctx, refundID, orderNo, groupNo)
		if err != nil {
			return nil, err
		}
		reply, retry, err := s.writeZeroRefundTx(ctx, refundID, orderNo, groupNo, result, remote)
		if err != nil {
			return nil, err
		}
		if retry {
			continue
		}
		return reply, nil
	}
	return nil, apperr.Conflict("zero-amount refund admission raced with shipment authorization; retry").
		WithDetails(map[string]any{"refund_id": refundID, "order_no": orderNo, "group_no": groupNo})
}

// decideZeroRefundResult applies the AC-50 permit branch for a zero refund.
// remote reports that the result came from CancelIfCreated on a CONSUMED
// permit, so the transaction knows a NONE->CONSUMED race must be re-decided
// instead of being recorded as PRE.
func (s *Service) decideZeroRefundResult(ctx context.Context, refundID, orderNo, groupNo string) (string, bool, error) {
	state, err := loadPermitState(ctx, s.db, orderNo, groupNo)
	if err != nil {
		return "", false, apperr.Internal("load shipping permit for zero refund", err)
	}
	switch state {
	case "", permitNone, permitRevoked:
		return refundResultPre, false, nil
	case permitConsumed:
		if s.fulfillment == nil {
			return "", false, apperr.Unavailable("fulfillment client is required for an authorized zero-amount refund", nil)
		}
		reply, err := s.fulfillment.CancelIfCreated(ctx, &pb.CancelIfCreatedRequest{
			OrderNo: orderNo, GroupNo: groupNo, RefundId: refundID,
		})
		if err != nil {
			return "", false, apperr.Unavailable("cancel created shipment for zero refund", err)
		}
		switch reply.GetStatus() {
		case "CANCELLED":
			return refundResultPre, true, nil
		case "ALREADY_SHIPPED", "RECEIVED":
			return refundResultPost, true, nil
		case probeStatusNoWaybill:
			// AC-67/AC-69: a zero-amount conflict is never persisted. No Tx1,
			// no admission/S19/DONE, no REVIEW_REQUIRED marker and no AC-72
			// review back log entry. The caller returns this conflict before
			// opening the single write transaction.
			return "", false, apperr.Failed("REFUND_ADMISSION_CONFLICT: consumed permit without shipment; manual review required").
				WithDetails(map[string]any{
					"refund_id": refundID, "order_no": orderNo, "group_no": groupNo,
					"permit_state": permitConsumed, "probe_status": probeStatusNoWaybill,
					"review_persisted": false, "manual_queue": manualQueueNowb,
					"reason": refundReasonConsumedWithoutShipment,
					"ticket": refundReviewTicket(refundID),
				})
		default:
			return "", false, apperr.Unavailable("shipment state is unknown; retry zero refund admission", nil)
		}
	default:
		return "", false, apperr.Failed("shipping permit state is not a legal persisted state").
			WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo, "permit_state": state})
	}
}

// writeZeroRefundTx persists the admission result and the DONE execution fact
// atomically. retry=true means the permit changed to CONSUMED after the
// pre-transaction decision; nothing was written.
func (s *Service) writeZeroRefundTx(ctx context.Context, refundID, orderNo, groupNo, result string, remote bool) (*pb.AdmitRefundReply, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, apperr.Internal("begin zero-amount refund admission", err)
	}
	defer func() { _ = tx.Rollback() }()
	locked, err := findOrderForUpdate(ctx, tx, orderNo)
	if err != nil {
		return nil, false, apperr.Internal("lock order for zero-amount refund", err)
	}
	if locked == nil {
		return nil, false, apperr.NotFound("order not found")
	}
	if !zeroRefundableStatus(locked.Status) {
		return nil, false, apperr.Failed("order cannot accept refunds in status " + locked.Status)
	}
	if locked.PayCent != 0 || locked.SettlementPaymentNo != zeroPaymentPrefix+locked.OrderNo {
		return nil, false, apperr.Failed("order facts no longer describe a zero-amount order; manual reconciliation required").
			WithDetails(map[string]any{
				"refund_id": refundID, "order_no": orderNo,
				"pay_cent": locked.PayCent, "settlement_payment_no": locked.SettlementPaymentNo,
			})
	}
	// A concurrent caller may have won the order lock after our pre-check.
	if app, err := findRefundApplication(ctx, tx, refundID, true); err != nil {
		return nil, false, apperr.Internal("load zero-amount refund application under lock", err)
	} else if app != nil {
		_ = tx.Rollback()
		reply, rerr := s.zeroRefundReply(ctx, app, orderNo, groupNo)
		return reply, false, rerr
	}
	admissionResult := result
	if existing, err := findRefundAdmissionLocked(ctx, tx, refundID); err != nil {
		return nil, false, apperr.Internal("load zero-amount refund admission under lock", err)
	} else if existing != nil {
		if existing.State == refundPending {
			return nil, false, apperr.Failed("zero-amount refund admission is PENDING_REMOTE; manual reconciliation required").
				WithDetails(map[string]any{"refund_id": refundID, "order_no": orderNo})
		}
		if existing.Result != refundResultPre && existing.Result != refundResultPost {
			return nil, false, apperr.Failed("zero-amount refund admission has an illegal result").
				WithDetails(map[string]any{"refund_id": refundID, "result": existing.Result})
		}
		if existing.OrderNo != orderNo || existing.GroupNo != groupNo || existing.AmountCent != 0 {
			return nil, false, apperr.Failed("zero-amount refund admission conflicts with the request").
				WithDetails(map[string]any{
					"refund_id": refundID, "order_no": existing.OrderNo, "group_no": existing.GroupNo,
					"amount_cent": existing.AmountCent,
				})
		}
		admissionResult = existing.Result
	} else {
		if err := ensureGroupExistsTx(ctx, tx, orderNo, groupNo); err != nil {
			return nil, false, err
		}
		permit, err := ensurePermitTx(ctx, tx, orderNo, groupNo)
		if err != nil {
			return nil, false, err
		}
		switch permit.state {
		case permitNone:
			res, err := tx.ExecContext(ctx, `UPDATE order_shipping_permits
				SET state = ?, version = version + 1, updated_at = NOW(3)
				WHERE order_no = ? AND fulfillment_group_no = ? AND state = ?`,
				permitRevoked, orderNo, groupNo, permitNone)
			if err != nil {
				return nil, false, apperr.Internal("revoke permit for zero-amount refund", err)
			}
			if n, _ := res.RowsAffected(); n == 0 {
				// Lost the CAS to a concurrent Ship: re-decide on CONSUMED.
				return nil, true, nil
			}
		case permitRevoked:
			// Idempotent PRE.
		case permitConsumed:
			if !remote {
				// A concurrent Ship won the permit CAS after our decision read
				// NONE/REVOKED: roll back and re-decide on the CONSUMED branch
				// so CancelIfCreated (not a blind PRE) resolves the race.
				return nil, true, nil
			}
			if admissionResult != refundResultPre && admissionResult != refundResultPost {
				return nil, false, apperr.Failed("zero-amount refund admission has no remote cancel result").
					WithDetails(map[string]any{"refund_id": refundID, "order_no": orderNo, "group_no": groupNo})
			}
		default:
			return nil, false, apperr.Failed("shipping permit state is not a legal persisted state").
				WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo, "permit_state": permit.state})
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_admissions
			(refund_id, order_no, group_no, result, state, amount_cent, version)
			VALUES (?, ?, ?, ?, ?, 0, 1)`,
			refundID, orderNo, groupNo, admissionResult, admissionResult); err != nil {
			return nil, false, apperr.Internal("insert zero-amount refund admission", err)
		}
	}
	// The zero execution fact: DONE in the same transaction, no commitment.
	if _, err := tx.ExecContext(ctx, `INSERT INTO refund_applications
		(refund_id, batch_id, source, order_no, group_no, non_effective_capture,
		 amount_cent, goods_cent, freight_cent, state, admission_result, version)
		VALUES (?, NULL, ?, ?, ?, 0, 0, 0, 0, ?, ?, 1)`,
		refundID, refundSourceSystemZero, orderNo, groupNo, refundApplicationDone, admissionResult); err != nil {
		return nil, false, apperr.Internal("insert zero-amount refund application", err)
	}
	if err := appendRefundAdmitted(ctx, tx, refundID, orderNo, groupNo, admissionResult, 0, 1, refundApplicationDone); err != nil {
		return nil, false, err
	}
	// Zero and positive amounts share the same INV-44 release path; zero only
	// skips Payment. A POST result left the warehouse and must not release.
	if admissionResult == refundResultPre {
		if err := s.appendRefundReleaseTx(ctx, tx, refundID, orderNo, groupNo, refundSourceSystemZero); err != nil {
			return nil, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, false, apperr.Internal("commit zero-amount refund admission", err)
	}
	return &pb.AdmitRefundReply{
		RefundId: refundID, Result: admissionResult, State: refundApplicationDone, AmountCent: 0, Version: 1,
	}, false, nil
}

// zeroRefundReply returns the persisted DONE fact for an idempotent repeat.
func (s *Service) zeroRefundReply(ctx context.Context, app *refundApplicationRow, orderNo, groupNo string) (*pb.AdmitRefundReply, error) {
	if app.Source != refundSourceSystemZero || app.AmountCent != 0 || app.GoodsCent != 0 ||
		app.FreightCent != 0 || app.NonEffective != 0 {
		return nil, apperr.Failed("refund_id is already used by a non-zero refund (REFUND_ADMISSION_CONFLICT)").
			WithDetails(map[string]any{
				"refund_id": app.RefundID, "source": app.Source,
				"amount_cent": app.AmountCent, "goods_cent": app.GoodsCent, "freight_cent": app.FreightCent,
			})
	}
	if app.OrderNo != orderNo || app.GroupNo != groupNo {
		return nil, apperr.Failed("zero-amount refund application is bound to another order/group").
			WithDetails(map[string]any{
				"refund_id": app.RefundID, "order_no": app.OrderNo, "group_no": app.GroupNo,
			})
	}
	if app.State != refundApplicationDone {
		return nil, apperr.Failed("zero-amount refund application is not DONE; manual reconciliation required").
			WithDetails(map[string]any{"refund_id": app.RefundID, "state": app.State})
	}
	adm, err := findRefundAdmission(ctx, s.db, app.RefundID)
	if err != nil {
		return nil, apperr.Internal("load zero-amount refund admission", err)
	}
	if adm == nil || adm.State != app.AdmissionResult || adm.Result != app.AdmissionResult || adm.AmountCent != 0 {
		return nil, apperr.Failed("zero-amount refund admission is missing or inconsistent; manual reconciliation required").
			WithDetails(map[string]any{"refund_id": app.RefundID, "admission_result": app.AdmissionResult})
	}
	return &pb.AdmitRefundReply{
		RefundId: app.RefundID, Result: app.AdmissionResult, State: refundApplicationDone,
		AmountCent: 0, Version: app.Version,
	}, nil
}

func zeroRefundableStatus(status string) bool {
	switch status {
	case statusPaid, statusAllocated, statusShipped, statusCompleted:
		return true
	default:
		return false
	}
}

func (s *Service) finishRemoteAdmission(ctx context.Context, row *refundAdmissionRow, source, batchID string) (*pb.AdmitRefundReply, error) {
	if s.fulfillment == nil {
		return nil, apperr.Unavailable("fulfillment client is required for consumed permits", nil)
	}
	reply, err := s.fulfillment.CancelIfCreated(ctx, &pb.CancelIfCreatedRequest{
		OrderNo: row.OrderNo, GroupNo: row.GroupNo, RefundId: row.RefundID,
	})
	if err != nil {
		return nil, apperr.Unavailable("cancel created shipment", err)
	}
	switch reply.GetStatus() {
	case "CANCELLED":
		// A real CANCELLED row is the only probe answer that may write PRE.
		return s.writeAdmissionTx2(ctx, row, refundResultPre, refundResultPre, source, batchID, "", "")
	case "ALREADY_SHIPPED", "RECEIVED":
		return s.writeAdmissionTx2(ctx, row, refundResultPost, refundResultPost, source, batchID, "", "")
	case probeStatusNoWaybill:
		// CONSUMED permit + no waybill row is not a legal AC-50 branch: persist
		// the bounded review decision instead of guessing PRE/POST. state stays
		// PENDING_REMOTE; no release, no terminal outbox, no Payment command.
		return s.writeAdmissionTx2(ctx, row, refundReviewRequired, refundPending, source, batchID, refundReasonConsumedWithoutShipment, probeStatusNoWaybill)
	default:
		// Unknown/future values stay retryable and can never become PRE.
		return nil, apperr.Unavailable("shipment state is unknown; retry refund admission", nil)
	}
}

// persistTCCReviewTx is the single AC-71 human-hold entry shared by the fresh
// Tx1 path and the Tx2 path. It writes the batch MANUAL_HOLD + canonical
// last_error and mirrors the already-existing application row; both mirror
// updates must match exactly one row. Any 0-row path aborts the surrounding
// transaction, so a REVIEW admission is never committed without the hold.
func persistTCCReviewTx(ctx context.Context, tx *sql.Tx, refundID, batchID, reason string) error {
	if batchID == "" {
		return apperr.Failed("tcc refund review requires batch_id; no review committed").
			WithDetails(map[string]any{"refund_id": refundID})
	}
	lastError := refundReviewCanonical(refundID, reason, refundBatchLastErrorWidth)
	res, err := tx.ExecContext(ctx, `UPDATE refund_batches
		SET state = ?, last_error = ?, version = version + 1, updated_at = NOW(3)
		WHERE batch_id = ? AND state IN (?, ?)`,
		refundBatchManualHold, lastError, batchID, refundBatchFrozen, refundBatchAdmitting)
	if err != nil {
		return apperr.Internal("hold tcc refund batch for review", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return apperr.Failed("tcc refund batch state changed while persisting review; no review committed").
			WithDetails(map[string]any{"batch_id": batchID, "refund_id": refundID})
	}
	res, err = tx.ExecContext(ctx, `UPDATE refund_applications
		SET admission_result = ?, version = version + 1, updated_at = NOW(3)
		WHERE refund_id = ? AND state = ?`,
		refundReviewRequired, refundID, refundApplicationPending)
	if err != nil {
		return apperr.Internal("mirror tcc review result on refund application", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return apperr.Failed("tcc refund application is not PENDING while persisting review; no review committed").
			WithDetails(map[string]any{"batch_id": batchID, "refund_id": refundID})
	}
	return nil
}

// writeAdmissionTx2 applies one decided admission result with the mandatory
// double-predicate CAS. The predicate makes a REVIEW_REQUIRED row immutable to
// any automatic path (late probe / concurrent Tx2 / replay): such a row never
// matches, so a late CANCELLED cannot wash the review into PRE. A losing Tx2
// re-reads the persisted decision instead of writing its own outbox/release.
// REVIEW_REQUIRED writes no outbox and no release; PRE/POST keep the existing
// event/release behaviour.
func (s *Service) writeAdmissionTx2(ctx context.Context, row *refundAdmissionRow, result, state, source, batchID, reason, probeStatus string) (*pb.AdmitRefundReply, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin refund admission tx2", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE order_refund_admissions
		SET result = ?, state = ?, version = version + 1, updated_at = NOW(3)
		WHERE refund_id = ? AND state = ? AND result <> ?`,
		result, state, row.RefundID, refundPending, refundReviewRequired)
	if err != nil {
		return nil, apperr.Internal("update refund admission", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, apperr.Internal("read refund admission rows affected", err)
	}
	if affected == 0 {
		_ = tx.Rollback()
		return s.readBackAdmissionTx2(ctx, row)
	}
	if result == refundResultPre || result == refundResultPost {
		if err := appendRefundAdmitted(ctx, tx, row.RefundID, row.OrderNo, row.GroupNo, result, row.AmountCent, row.Version+1, state); err != nil {
			return nil, err
		}
	}
	if result == refundResultPre {
		if err := s.appendRefundReleaseTx(ctx, tx, row.RefundID, row.OrderNo, row.GroupNo, source); err != nil {
			return nil, err
		}
	}
	if result == refundReviewRequired && source == refundApplicationSourceTCC {
		if err := persistTCCReviewTx(ctx, tx, row.RefundID, batchID, reason); err != nil {
			return nil, err
		}
	}
	if result == refundReviewRequired && s.log != nil {
		s.log.Error("refund admission review required: consumed permit without shipment",
			"refund_id", row.RefundID, "order_no", row.OrderNo, "group_no", row.GroupNo,
			"source", source, "result", refundReviewRequired, "state", refundPending,
			"reason", reason,
			"ticket", refundReviewTicket(row.RefundID), "manual_queue", manualQueueNowb)
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit refund admission tx2", err)
	}
	row.Result, row.State, row.Version = result, state, row.Version+1
	if result == refundReviewRequired {
		return nil, refundReviewConflict(row, reason, probeStatus)
	}
	return admissionReply(row), nil
}

// readBackAdmissionTx2 converges a Tx2 whose CAS matched no row. All zero-row
// paths are read-only: no outbox and no release may be written (ADR §6 A8).
func (s *Service) readBackAdmissionTx2(ctx context.Context, row *refundAdmissionRow) (*pb.AdmitRefundReply, error) {
	persisted, err := findRefundAdmission(ctx, s.db, row.RefundID)
	if err != nil {
		return nil, apperr.Internal("reload refund admission after tx2", err)
	}
	switch classifyExistingAdmission(persisted) {
	case existingAdmissionNone:
		return nil, apperr.Conflict("admission disappeared during Tx2").
			WithDetails(map[string]any{
				"refund_id": row.RefundID, "order_no": row.OrderNo, "group_no": row.GroupNo,
				"manual_queue": manualQueueNowb,
			})
	case existingAdmissionReview:
		return nil, s.replayReviewConflict(ctx, persisted)
	case existingAdmissionTerminal:
		return admissionReply(persisted), nil
	case existingAdmissionPending:
		return nil, apperr.Conflict("refund admission changed during Tx2; retry").
			WithDetails(map[string]any{"refund_id": row.RefundID})
	default:
		return nil, apperr.Conflict("refund admission has an illegal result/state combination").
			WithDetails(map[string]any{
				"refund_id": row.RefundID, "result": persisted.Result, "state": persisted.State,
			})
	}
}

func appendRefundAdmitted(ctx context.Context, tx *sql.Tx, refundID, orderNo, groupNo, result string, amount, version int64, state string) error {
	return outbox.Append(ctx, tx, outbox.Event{
		Topic: topicRefundAdmit, AggregateType: "order", AggregateID: orderNo,
		EventType: "order.refund_admitted",
		Payload: map[string]any{
			"refund_id": refundID, "order_no": orderNo, "group_no": groupNo,
			"result": result, "amount_cent": amount, "version": version, "state": state,
		},
	})
}
