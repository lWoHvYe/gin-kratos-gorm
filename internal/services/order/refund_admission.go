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
	var row refundAdmissionRow
	err := q.QueryRowContext(ctx, `SELECT refund_id, order_no, group_no, result, state, amount_cent, version
		FROM order_refund_admissions WHERE refund_id = ?`, refundID).
		Scan(&row.RefundID, &row.OrderNo, &row.GroupNo, &row.Result, &row.State, &row.AmountCent, &row.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
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
	if s.payments == nil {
		return nil, apperr.Unavailable("payment client is required for refund admission", nil)
	}
	if record.SettlementPaymentNo == "" {
		return nil, apperr.Failed("order has no settlement payment; resolve historical payment first")
	}
	intent, err := s.payments.GetIntent(ctx, &pb.GetIntentRequest{PaymentNo: record.SettlementPaymentNo})
	if err != nil {
		return nil, apperr.Unavailable("load authoritative captured amount", err)
	}
	if intent.GetCapturedCent() <= 0 {
		return nil, apperr.Failed("effective payment has no captured amount")
	}

	// Resume a two-phase admission after a crash before touching the permit.
	if existing, err := findRefundAdmission(ctx, s.db, refundID); err != nil {
		return nil, apperr.Internal("load refund admission", err)
	} else if existing != nil && existing.State == refundPending {
		return s.finishRemoteAdmission(ctx, existing)
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
	if req.GetAmountCent()+reserved > intent.GetCapturedCent()-intent.GetRefundedCent() {
		return nil, apperr.Failed("refund exceeds the remaining captured amount").
			WithDetails(map[string]any{"captured_cent": intent.GetCapturedCent(), "refunded_cent": intent.GetRefundedCent(), "reserved_cent": reserved})
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
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit refund admission", err)
	}
	row := &refundAdmissionRow{
		RefundID: refundID, OrderNo: orderNo, GroupNo: groupNo, Result: result,
		State: state, AmountCent: req.GetAmountCent(), Version: 1,
	}
	if state == refundPending {
		return s.finishRemoteAdmission(ctx, row)
	}
	return admissionReply(row), nil
}

func (s *Service) finishRemoteAdmission(ctx context.Context, row *refundAdmissionRow) (*pb.AdmitRefundReply, error) {
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

