package aftersales

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"
)

// After-sale request types.
const (
	typeRefundOnly   = "REFUND_ONLY"
	typeReturnRefund = "RETURN_REFUND"
)

// After-sale lifecycle:
//
//	REQUESTED -> APPROVED -> REFUNDING -> REFUNDED
//	REQUESTED -> REJECTED
const (
	statusRequested = "REQUESTED"
	statusApproved  = "APPROVED"
	statusRejected  = "REJECTED"
	statusRefunding = "REFUNDING"
	statusRefunded  = "REFUNDED"
)

// Outbox subjects. created/completed are part of the documented Phase 1 event
// set; approved/rejected are service-local additions on the same naming scheme.
const (
	topicAfterSaleCreated   = "shop.aftersale.created.v1"
	topicAfterSaleApproved  = "shop.aftersale.approved.v1"
	topicAfterSaleRejected  = "shop.aftersale.rejected.v1"
	topicAfterSaleCompleted = "shop.aftersale.completed.v1"
)

const aggregateAfterSale = "after_sale"

type Service struct {
	pb.UnimplementedAfterSalesServiceServer
	db       *sql.DB
	mq       *mq.Client
	payments pb.PaymentServiceClient
	log      *slog.Logger
}

func New(db *sql.DB, client *mq.Client, payments pb.PaymentServiceClient, logger *slog.Logger) *Service {
	return &Service{db: db, mq: client, payments: payments, log: logger}
}

// CreateAfterSale opens a refund only / return-refund request. The request is
// stored as REQUESTED and an shop.aftersale.created.v1 event is emitted.
func (s *Service) CreateAfterSale(ctx context.Context, req *pb.CreateAfterSaleRequest) (*pb.AfterSale, error) {
	input, items, err := validateCreate(req.GetOrderNo(), req.GetUserId(), req.GetType(), req.GetReason(), req.GetAmountCent(), req.GetItems())
	if err != nil {
		return nil, err
	}
	afterSaleNo := id.AfterSale()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin create after sale", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, "INSERT INTO after_sales (after_sale_no, order_no, user_id, `type`, status, amount_cent, reason) VALUES (?, ?, ?, ?, ?, ?, ?)",
		afterSaleNo, input.orderNo, input.userID, input.typ, statusRequested, input.amountCent, input.reason)
	if err != nil {
		return nil, apperr.Internal("create after sale", err)
	}
	afterSaleID, err := res.LastInsertId()
	if err != nil {
		return nil, apperr.Internal("after sale id", err)
	}
	for _, it := range items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO after_sale_items (after_sale_id, sku_id, quantity) VALUES (?, ?, ?)`,
			afterSaleID, it.SkuID, it.Quantity); err != nil {
			return nil, apperr.Internal("create after sale item", err)
		}
	}
	if err := appendLog(ctx, tx, afterSaleID, "CREATE", "", statusRequested, input.reason); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(afterSaleEvent{
		AfterSaleNo: afterSaleNo, OrderNo: input.orderNo, UserID: input.userID,
		Type: input.typ, Status: statusRequested, AmountCent: input.amountCent, Reason: input.reason,
	})
	if err != nil {
		return nil, apperr.Internal("marshal after sale event", err)
	}
	if err := outbox.Append(ctx, tx, outbox.Event{
		Topic: topicAfterSaleCreated, AggregateType: aggregateAfterSale,
		AggregateID: afterSaleNo, EventType: "aftersale.created", Payload: payload,
	}); err != nil {
		return nil, apperr.Internal("append after sale created", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit create after sale", err)
	}
	return s.loadAfterSale(ctx, afterSaleNo)
}

func (s *Service) GetAfterSale(ctx context.Context, req *pb.GetAfterSaleRequest) (*pb.AfterSale, error) {
	afterSaleNo := strings.TrimSpace(req.GetAfterSaleNo())
	if afterSaleNo == "" {
		return nil, apperr.Invalid("after_sale_no is required")
	}
	return s.loadAfterSale(ctx, afterSaleNo)
}

// Approve accepts or rejects a pending request. Approving with an amount_cent
// lower than the requested amount records a partial approval.
func (s *Service) Approve(ctx context.Context, req *pb.ApproveRequest) (*pb.AfterSale, error) {
	afterSaleNo := strings.TrimSpace(req.GetAfterSaleNo())
	if afterSaleNo == "" {
		return nil, apperr.Invalid("after_sale_no is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin approve after sale", err)
	}
	defer func() { _ = tx.Rollback() }()

	row, err := lockAfterSale(ctx, tx, afterSaleNo)
	if err != nil {
		return nil, err
	}
	next, changed, err := approveOutcome(row.status, req.GetApproved())
	if err != nil {
		return nil, err
	}
	amountCent := row.amountCent
	if req.GetApproved() && req.GetAmountCent() > 0 {
		if req.GetAmountCent() > row.amountCent {
			return nil, apperr.Invalid("approved amount_cent exceeds the requested amount")
		}
		amountCent = req.GetAmountCent()
	}
	topic, eventType, action := topicAfterSaleApproved, "aftersale.approved", "APPROVE"
	if next == statusRejected {
		topic, eventType, action = topicAfterSaleRejected, "aftersale.rejected", "REJECT"
	}
	if changed {
		if _, err := tx.ExecContext(ctx, "UPDATE after_sales SET status = ?, amount_cent = ?, version = version + 1 WHERE id = ?",
			next, amountCent, row.id); err != nil {
			return nil, apperr.Internal("update after sale status", err)
		}
		if err := appendLog(ctx, tx, row.id, action, row.status, next, req.GetReason()); err != nil {
			return nil, err
		}
		payload, err := json.Marshal(afterSaleEvent{
			AfterSaleNo: afterSaleNo, OrderNo: row.orderNo, UserID: row.userID,
			Type: row.typ, Status: next, AmountCent: amountCent, Reason: row.reason,
		})
		if err != nil {
			return nil, apperr.Internal("marshal after sale event", err)
		}
		if err := outbox.Append(ctx, tx, outbox.Event{
			Topic: topic, AggregateType: aggregateAfterSale,
			AggregateID: afterSaleNo, EventType: eventType, Payload: payload,
		}); err != nil {
			return nil, apperr.Internal("append "+eventType, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit approve after sale", err)
	}
	return s.loadAfterSale(ctx, afterSaleNo)
}

// Refund first persists REFUNDING, then calls payment, then marks REFUNDED.
// A failure between the two phases leaves the request REFUNDING so a retry can
// safely call the idempotent payment refund again.
func (s *Service) Refund(ctx context.Context, req *pb.RefundAfterSaleRequest) (*pb.AfterSale, error) {
	afterSaleNo := strings.TrimSpace(req.GetAfterSaleNo())
	if afterSaleNo == "" {
		return nil, apperr.Invalid("after_sale_no is required")
	}
	if s.payments == nil {
		return nil, apperr.Unavailable("payment client is not configured", nil)
	}
	var row *afterSaleRow
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin refund after sale", err)
	}
	row, err = lockAfterSale(ctx, tx, afterSaleNo)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if _, _, err := refundOutcome(row.status); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	switch row.status {
	case statusRefunded:
		_ = tx.Rollback()
		return s.loadAfterSale(ctx, afterSaleNo)
	case statusRefunding:
		_ = tx.Rollback()
	case statusApproved:
		if _, err := tx.ExecContext(ctx, "UPDATE after_sales SET status = ?, version = version + 1 WHERE id = ?",
			statusRefunding, row.id); err != nil {
			_ = tx.Rollback()
			return nil, apperr.Internal("mark after sale refunding", err)
		}
		intent, _ := json.Marshal(map[string]any{
			"after_sale_no": afterSaleNo, "order_no": row.orderNo, "user_id": row.userID,
			"amount_cent": row.amountCent, "payment": "pending",
		})
		if err := appendLog(ctx, tx, row.id, "REFUND_INTENT", statusApproved, statusRefunding, string(intent)); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, apperr.Internal("commit refund intent", err)
		}
	default:
		_ = tx.Rollback()
		return nil, apperr.Failed(fmt.Sprintf("after sale cannot be refunded in status %s", row.status))
	}

	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := s.payments.RefundOrder(callCtx, &pb.RefundOrderRequest{
		OrderNo: row.orderNo, AmountCent: row.amountCent, Reason: "after-sale " + afterSaleNo,
	}); err != nil {
		s.log.Warn("after sale payment refund failed", "after_sale_no", afterSaleNo, "order_no", row.orderNo, "error", err)
		return nil, apperr.Unavailable("payment refund failed; retry is safe", err)
	}

	tx, err = s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin complete refund", err)
	}
	defer func() { _ = tx.Rollback() }()
	row, err = lockAfterSale(ctx, tx, afterSaleNo)
	if err != nil {
		return nil, err
	}
	if row.status == statusRefunded {
		return s.loadAfterSale(ctx, afterSaleNo)
	}
	if row.status != statusRefunding {
		return nil, apperr.Failed(fmt.Sprintf("after sale cannot be completed in status %s", row.status))
	}
	if _, err := tx.ExecContext(ctx, "UPDATE after_sales SET status = ?, version = version + 1 WHERE id = ?",
		statusRefunded, row.id); err != nil {
		return nil, apperr.Internal("mark after sale refunded", err)
	}
	if err := appendLog(ctx, tx, row.id, "REFUND", statusRefunding, statusRefunded, "payment refund completed"); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(afterSaleEvent{
		AfterSaleNo: afterSaleNo, OrderNo: row.orderNo, UserID: row.userID,
		Type: row.typ, Status: statusRefunded, AmountCent: row.amountCent, Reason: row.reason,
	})
	if err != nil {
		return nil, apperr.Internal("marshal after sale event", err)
	}
	if err := outbox.Append(ctx, tx, outbox.Event{
		Topic: topicAfterSaleCompleted, AggregateType: aggregateAfterSale,
		AggregateID: afterSaleNo, EventType: "aftersale.completed", Payload: payload,
	}); err != nil {
		return nil, apperr.Internal("append aftersale.completed", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit refund after sale", err)
	}
	return s.loadAfterSale(ctx, afterSaleNo)
}

// afterSaleRow is the mutable state needed by the state machine.
type afterSaleRow struct {
	id         int64
	orderNo    string
	userID     int64
	typ        string
	status     string
	amountCent int64
	reason     string
}

func lockAfterSale(ctx context.Context, tx *sql.Tx, afterSaleNo string) (*afterSaleRow, error) {
	var row afterSaleRow
	err := tx.QueryRowContext(ctx, "SELECT id, order_no, user_id, `type`, status, amount_cent, reason FROM after_sales WHERE after_sale_no = ? FOR UPDATE", afterSaleNo).
		Scan(&row.id, &row.orderNo, &row.userID, &row.typ, &row.status, &row.amountCent, &row.reason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("after sale not found")
	}
	if err != nil {
		return nil, apperr.Internal("lock after sale", err)
	}
	return &row, nil
}

func (s *Service) loadAfterSale(ctx context.Context, afterSaleNo string) (*pb.AfterSale, error) {
	var (
		as        pb.AfterSale
		createdAt time.Time
	)
	err := s.db.QueryRowContext(ctx, "SELECT after_sale_no, order_no, user_id, `type`, status, amount_cent, reason, created_at FROM after_sales WHERE after_sale_no = ?", afterSaleNo).
		Scan(&as.AfterSaleNo, &as.OrderNo, &as.UserId, &as.Type, &as.Status, &as.AmountCent, &as.Reason, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("after sale not found")
	}
	if err != nil {
		return nil, apperr.Internal("get after sale", err)
	}
	as.CreatedAtUnix = createdAt.Unix()
	return &as, nil
}

func appendLog(ctx context.Context, tx *sql.Tx, afterSaleID int64, action, fromStatus, toStatus, remark string) error {
	if len(remark) > 512 {
		remark = remark[:512]
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO after_sale_logs (after_sale_id, action, from_status, to_status, remark)
		VALUES (?, ?, ?, ?, ?)`, afterSaleID, action, fromStatus, toStatus, remark); err != nil {
		return apperr.Internal("append after sale log", err)
	}
	return nil
}

// afterSaleInput is the validated create request.
type afterSaleInput struct {
	orderNo    string
	userID     int64
	typ        string
	amountCent int64
	reason     string
}

// afterSaleItem is the persisted view of an after-sale line.
type afterSaleItem struct {
	SkuID    int64 `json:"sku_id"`
	Quantity int32 `json:"quantity"`
}

// afterSaleEvent is the outbox payload for shop.aftersale.* events.
type afterSaleEvent struct {
	AfterSaleNo  string `json:"after_sale_no"`
	OrderNo      string `json:"order_no"`
	UserID       int64  `json:"user_id"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	AmountCent   int64  `json:"amount_cent"`
	Reason       string `json:"reason"`
	RefundIntent bool   `json:"refund_intent,omitempty"`
}

// validateCreate validates and normalises a CreateAfterSale request.
func validateCreate(orderNo string, userID int64, typ, reason string, amountCent int64, items []*pb.AfterSaleItem) (afterSaleInput, []afterSaleItem, error) {
	in := afterSaleInput{
		orderNo: strings.TrimSpace(orderNo),
		userID:  userID,
		typ:     strings.TrimSpace(typ),
	}
	if in.orderNo == "" {
		return afterSaleInput{}, nil, apperr.Invalid("order_no is required")
	}
	if in.userID <= 0 {
		return afterSaleInput{}, nil, apperr.Invalid("user_id is required")
	}
	if in.typ != typeRefundOnly && in.typ != typeReturnRefund {
		return afterSaleInput{}, nil, apperr.Invalid("type must be REFUND_ONLY or RETURN_REFUND")
	}
	if amountCent <= 0 {
		return afterSaleInput{}, nil, apperr.Invalid("amount_cent must be positive")
	}
	in.amountCent = amountCent
	in.reason = strings.TrimSpace(reason)

	merged := make([]afterSaleItem, 0, len(items))
	index := make(map[int64]int, len(items))
	for _, item := range items {
		if item == nil {
			return afterSaleInput{}, nil, apperr.Invalid("after sale item must not be nil")
		}
		skuID := item.GetSkuId()
		if skuID <= 0 {
			return afterSaleInput{}, nil, apperr.Invalid("sku_id must be positive")
		}
		if item.GetQuantity() <= 0 {
			return afterSaleInput{}, nil, apperr.Invalid("quantity must be positive for sku %d", skuID)
		}
		if pos, ok := index[skuID]; ok {
			sum := int64(merged[pos].Quantity) + int64(item.GetQuantity())
			if sum > math.MaxInt32 {
				return afterSaleInput{}, nil, apperr.Invalid("quantity overflow for sku %d", skuID)
			}
			merged[pos].Quantity = int32(sum)
			continue
		}
		index[skuID] = len(merged)
		merged = append(merged, afterSaleItem{SkuID: skuID, Quantity: item.GetQuantity()})
	}
	return in, merged, nil
}

// approveOutcome computes the status after Approve. A repeated identical
// decision is idempotent (changed == false); a conflicting decision is rejected.
func approveOutcome(current string, approved bool) (string, bool, error) {
	switch current {
	case statusRequested:
		if approved {
			return statusApproved, true, nil
		}
		return statusRejected, true, nil
	case statusApproved:
		if approved {
			return statusApproved, false, nil
		}
		return "", false, apperr.Failed("after sale already approved")
	case statusRejected:
		if approved {
			return "", false, apperr.Failed("after sale already rejected")
		}
		return statusRejected, false, nil
	case statusRefunding, statusRefunded:
		return "", false, apperr.Failed(fmt.Sprintf("after sale already %s", strings.ToLower(current)))
	default:
		return "", false, apperr.Failed(fmt.Sprintf("invalid after sale status %q", current))
	}
}

// refundOutcome computes the status after Refund.
func refundOutcome(current string) (string, bool, error) {
	switch current {
	case statusApproved:
		return statusRefunding, true, nil
	case statusRefunding:
		return statusRefunded, true, nil
	case statusRefunded:
		return statusRefunded, false, nil
	case statusRequested:
		return "", false, apperr.Failed("after sale must be approved before refunding")
	case statusRejected:
		return "", false, apperr.Failed("after sale was rejected")
	default:
		return "", false, apperr.Failed(fmt.Sprintf("invalid after sale status %q", current))
	}
}
