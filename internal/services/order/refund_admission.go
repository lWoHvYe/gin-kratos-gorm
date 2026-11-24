package order

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/outbox"
)

const (
	refundResultPre  = "PRE_SHIP_REVOKED"
	refundResultPost = "POST_SHIP_ALLOWED"
	refundPending    = "PENDING_REMOTE"
	topicRefundAdmit = "shop.order.refund_admitted.v1"

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

// AdmitRefund is the group-level refund admission described by ADR-0007. It
// serializes admissions on the order row and the permit row, never moves money,
// and is idempotent by refund_id. CONSUMED permits take the two-phase
// PENDING_REMOTE -> CancelIfCreated -> Tx2 path.
func (s *Service) AdmitRefund(ctx context.Context, req *pb.AdmitRefundRequest) (*pb.AdmitRefundReply, error) {
	refundID := strings.TrimSpace(req.GetRefundId())
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if refundID == "" || orderNo == "" {
		return nil, apperr.Invalid("refund_id and order_no are required")
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
	if record.Status != statusPaid && record.Status != statusAllocated && record.Status != statusShipped && record.Status != statusCompleted {
		return nil, apperr.Failed("order cannot accept refunds in status " + record.Status)
	}
	// AC-67/AC-68/INV-50: a `system_zero` refund is the zero-amount execution
	// fact. It never consults Payment, quota or commitment; it only performs
	// the group admission (permit / CancelIfCreated) and writes the DONE ledger
	// row in the same transaction. The positive-amount path below is unchanged.
	if strings.TrimSpace(req.GetSource()) == refundSourceSystemZero {
		return s.admitZeroRefund(ctx, req, record, refundID, orderNo, groupNo)
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
			return nil, apperr.Failed("order has no settlement payment; resolve historical payment first")
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

	// Resume a two-phase admission after a crash before touching the permit.
	if existing, err := findRefundAdmission(ctx, s.db, refundID); err != nil {
		return nil, apperr.Internal("load refund admission", err)
	} else if existing != nil && existing.State == refundPending {
		return s.finishRemoteAdmission(ctx, existing, req.GetSource())
	} else if existing != nil {
		return admissionReply(existing), nil
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
		if existing.State == refundPending {
			return s.finishRemoteAdmission(ctx, existing, req.GetSource())
		}
		return admissionReply(existing), nil
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
	var groupPaid, groupFreight, groupConsumed int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(line_paid_cent),0) FROM order_items
		WHERE order_no = ? AND fulfillment_group_no = ?`, orderNo, groupNo).Scan(&groupPaid); err != nil {
		return nil, apperr.Internal("sum group paid amount", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(freight_share_cent),0) FROM order_fulfillment_groups
		WHERE order_no = ? AND group_no = ?`, orderNo, groupNo).Scan(&groupFreight); err != nil {
		return nil, apperr.Internal("load group freight share", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cent),0) FROM order_refund_commitments
		WHERE order_no = ? AND group_no = ? AND state IN ('RESERVED','SETTLED')`, orderNo, groupNo).Scan(&groupConsumed); err != nil {
		return nil, apperr.Internal("sum group refund commitments", err)
	}
	groupRemaining := groupPaid + groupFreight - groupConsumed
	if groupRemaining < 0 {
		groupRemaining = 0
	}
	if req.GetAmountCent() > groupRemaining {
		return nil, apperr.Failed("refund exceeds the group remaining amount").
			WithDetails(map[string]any{"group_remaining_cent": groupRemaining, "requested_cent": req.GetAmountCent()})
	}
	if permit.state != permitConsumed && req.GetAmountCent() != groupRemaining {
		return nil, apperr.Failed("partial refund of an unshipped group requires a full-group refund").
			WithDetails(map[string]any{"group_remaining_cent": groupRemaining, "requested_cent": req.GetAmountCent()})
	}

	result := refundResultPre
	state := refundResultPre
	if permit.state == permitConsumed {
		result = refundResultPost
		state = refundPending
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_admissions
		(refund_id, order_no, group_no, result, state, amount_cent, version)
		VALUES (?, ?, ?, ?, ?, ?, 1)`,
		refundID, orderNo, groupNo, result, state, req.GetAmountCent()); err != nil {
		return nil, apperr.Internal("insert refund admission", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_commitments
		(order_no, group_no, refund_id, amount_cent, state, version)
		VALUES (?, ?, ?, ?, ?, 1)
		ON DUPLICATE KEY UPDATE refund_id = refund_id`,
		orderNo, groupNo, refundID, req.GetAmountCent(), "RESERVED"); err != nil {
		return nil, apperr.Internal("insert refund commitment", err)
	}
	if permit.state == permitNone {
		if _, err := tx.ExecContext(ctx, `UPDATE order_shipping_permits
			SET state = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND fulfillment_group_no = ? AND state = ?`,
			permitRevoked, orderNo, groupNo, permitNone); err != nil {
			return nil, apperr.Internal("revoke permit for refund", err)
		}
	}
	if err := appendRefundAdmitted(ctx, tx, refundID, orderNo, groupNo, result, req.GetAmountCent(), 1, state); err != nil {
		return nil, err
	}
	if result == refundResultPre {
		if err := appendRefundReleaseTx(ctx, tx, refundID, orderNo, groupNo, req.GetSource()); err != nil {
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
	if state == refundPending {
		return s.finishRemoteAdmission(ctx, row, req.GetSource())
	}
	return admissionReply(row), nil
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
		if err := appendRefundReleaseTx(ctx, tx, refundID, orderNo, groupNo, refundSourceSystemZero); err != nil {
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

func (s *Service) finishRemoteAdmission(ctx context.Context, row *refundAdmissionRow, source string) (*pb.AdmitRefundReply, error) {
	if s.fulfillment == nil {
		return nil, apperr.Unavailable("fulfillment client is required for consumed permits", nil)
	}
	reply, err := s.fulfillment.CancelIfCreated(ctx, &pb.CancelIfCreatedRequest{
		OrderNo: row.OrderNo, GroupNo: row.GroupNo, RefundId: row.RefundID,
	})
	if err != nil {
		return nil, apperr.Unavailable("cancel created shipment", err)
	}
	result := refundResultPre
	switch reply.GetStatus() {
	case "CANCELLED":
		result = refundResultPre
	case "ALREADY_SHIPPED", "RECEIVED":
		result = refundResultPost
	default:
		return nil, apperr.Unavailable("shipment state is unknown; retry refund admission", nil)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin refund admission tx2", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE order_refund_admissions
		SET result = ?, state = ?, version = version + 1, updated_at = NOW(3)
		WHERE refund_id = ?`, result, result, row.RefundID); err != nil {
		return nil, apperr.Internal("update refund admission", err)
	}
	if err := appendRefundAdmitted(ctx, tx, row.RefundID, row.OrderNo, row.GroupNo, result, row.AmountCent, row.Version+1, result); err != nil {
		return nil, err
	}
	if result == refundResultPre {
		if err := appendRefundReleaseTx(ctx, tx, row.RefundID, row.OrderNo, row.GroupNo, source); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit refund admission tx2", err)
	}
	row.Result, row.State, row.Version = result, result, row.Version+1
	return admissionReply(row), nil
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
