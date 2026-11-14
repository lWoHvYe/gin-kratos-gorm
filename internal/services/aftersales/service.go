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

	"google.golang.org/grpc/metadata"
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
	statusReturning = "RETURNING"
	statusReceived  = "RECEIVED"
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
	db        *sql.DB
	mq        *mq.Client
	payments  pb.PaymentServiceClient
	inventory pb.InventoryServiceClient
	orders    pb.OrderServiceClient
	log       *slog.Logger
}

func New(db *sql.DB, client *mq.Client, payments pb.PaymentServiceClient, inventory pb.InventoryServiceClient, orders pb.OrderServiceClient, logger *slog.Logger) *Service {
	return &Service{db: db, mq: client, payments: payments, inventory: inventory, orders: orders, log: logger}
}

// validateAgainstOrder prevents after-sales from inventing money or stock. The
// order service is the source of truth for paid/refunded totals and items.
func (s *Service) validateAgainstOrder(ctx context.Context, orderNo string, userID int64, typ string, amountCent int64, items []afterSaleItem) error {
	if s.orders == nil {
		return apperr.Unavailable("order client is not configured", nil)
	}
	order, err := s.orders.GetOrder(ctx, &pb.GetOrderRequest{OrderNo: orderNo, UserId: userID})
	if err != nil {
		return apperr.Unavailable("load order for after-sale validation", err)
	}
	if order.GetOrderNo() == "" || order.GetUserId() != userID {
		return apperr.NotFound("order not found")
	}
	switch order.GetStatus() {
	case "PAID", "ALLOCATED", "SHIPPED", "COMPLETED", "PARTIALLY_REFUNDED":
	default:
		return apperr.Failed(fmt.Sprintf("order is not eligible for after-sales in status %s", order.GetStatus()))
	}
	if typ == typeReturnRefund && order.GetStatus() != "SHIPPED" && order.GetStatus() != "COMPLETED" && order.GetStatus() != "PARTIALLY_REFUNDED" {
		return apperr.Failed("return refund requires a shipped or completed order")
	}
	remaining := order.GetPaidCent() - order.GetRefundedCent()
	if remaining <= 0 {
		return apperr.Failed("order has no refundable amount")
	}
	var requested int64
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cent), 0) FROM after_sales
		WHERE order_no = ? AND status <> 'REJECTED'`, orderNo).Scan(&requested); err != nil {
		return apperr.Internal("sum after-sale amounts", err)
	}
	if requested+amountCent > remaining {
		return apperr.Invalid("after-sale amount %d exceeds remaining refundable amount %d", amountCent, remaining-requested)
	}
	if len(items) == 0 {
		if typ == typeReturnRefund {
			return apperr.Invalid("return refund requires at least one item")
		}
		return nil
	}
	ordered := make(map[int64]int64, len(order.GetItems()))
	for _, item := range order.GetItems() {
		ordered[item.GetSkuId()] += int64(item.GetQuantity())
	}
	for _, item := range items {
		if ordered[item.SkuID] == 0 {
			return apperr.Invalid("sku %d is not part of order %s", item.SkuID, orderNo)
		}
		var existing int64
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(i.quantity), 0)
			FROM after_sale_items i JOIN after_sales a ON a.id = i.after_sale_id
			WHERE a.order_no = ? AND a.status <> 'REJECTED' AND i.sku_id = ?`, orderNo, item.SkuID).Scan(&existing); err != nil {
			return apperr.Internal("sum after-sale item quantity", err)
		}
		if existing+int64(item.Quantity) > ordered[item.SkuID] {
			return apperr.Invalid("return quantity for sku %d exceeds ordered quantity", item.SkuID)
		}
	}
	return nil
}

// CreateAfterSale opens a refund only / return-refund request. The request is
// stored as REQUESTED and an shop.aftersale.created.v1 event is emitted.
func (s *Service) CreateAfterSale(ctx context.Context, req *pb.CreateAfterSaleRequest) (*pb.AfterSale, error) {
	input, items, err := validateCreate(req.GetOrderNo(), req.GetUserId(), req.GetType(), req.GetReason(), req.GetAmountCent(), req.GetItems())
	if err != nil {
		return nil, err
	}
	if err := s.validateAgainstOrder(ctx, input.orderNo, input.userID, input.typ, input.amountCent, items); err != nil {
		return nil, err
	}
	// A retried submit must return the first request instead of creating a
	// second after-sale; the client sends the key as gRPC metadata.
	requestID := clientRequestID(ctx)
	if requestID != "" {
		existing, err := s.findByClientRequest(ctx, input.userID, requestID)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}
	afterSaleNo := id.AfterSale()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin create after sale", err)
	}
	defer func() { _ = tx.Rollback() }()

	key := sql.NullString{String: requestID, Valid: requestID != ""}
	res, err := tx.ExecContext(ctx, "INSERT INTO after_sales (after_sale_no, order_no, user_id, `type`, status, amount_cent, reason, client_request_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		afterSaleNo, input.orderNo, input.userID, input.typ, statusRequested, input.amountCent, input.reason, key)
	if err != nil {
		if isDuplicateKey(err) && requestID != "" {
			// Another replica won the race; return its row.
			_ = tx.Rollback()
			existing, ferr := s.findByClientRequest(ctx, input.userID, requestID)
			if ferr != nil {
				return nil, ferr
			}
			if existing != nil {
				return existing, nil
			}
		}
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
	if row.status == statusRefunded {
		_ = tx.Rollback()
		return s.loadAfterSale(ctx, afterSaleNo)
	}
	if err := refundableStatus(row); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if row.status != statusRefunding {
		if _, err := tx.ExecContext(ctx, "UPDATE after_sales SET status = ?, version = version + 1 WHERE id = ?",
			statusRefunding, row.id); err != nil {
			_ = tx.Rollback()
			return nil, apperr.Internal("mark after sale refunding", err)
		}
		intent, _ := json.Marshal(map[string]any{
			"after_sale_no": afterSaleNo, "order_no": row.orderNo, "user_id": row.userID,
			"amount_cent": row.amountCent, "payment": "pending",
		})
		if err := appendLog(ctx, tx, row.id, "REFUND_INTENT", row.status, statusRefunding, string(intent)); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, apperr.Internal("commit refund intent", err)
		}
	} else {
		_ = tx.Rollback()
	}

	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	refundReply, err := s.payments.RefundOrder(callCtx, &pb.RefundOrderRequest{
		OrderNo: row.orderNo, AmountCent: row.amountCent,
		Reason: "after-sale " + afterSaleNo, IdempotencyKey: afterSaleNo,
	})
	if err != nil {
		s.log.Warn("after sale payment refund failed", "after_sale_no", afterSaleNo, "order_no", row.orderNo, "error", err)
		return nil, apperr.Unavailable("payment refund failed; retry is safe", err)
	}
	refundNo := ""
	if refundReply != nil {
		refundNo = refundReply.GetRefundNo()
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
	if _, err := tx.ExecContext(ctx, "UPDATE after_sales SET status = ?, refund_no = ?, refunded_cent = ?, version = version + 1 WHERE id = ?",
		statusRefunded, refundNo, row.amountCent, row.id); err != nil {
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

func refundableStatus(row *afterSaleRow) error {
	switch row.typ {
	case typeRefundOnly:
		if row.status != statusApproved && row.status != statusRefunding {
			return apperr.Failed("refund-only after sale must be approved before refunding")
		}
	case typeReturnRefund:
		if row.status != statusReceived && row.status != statusRefunding {
			return apperr.Failed("return-refund after sale must be received before refunding")
		}
	default:
		return apperr.Failed("unsupported after sale type")
	}
	return nil
}

// SubmitReturnTracking records the reverse logistics information for a
// return-refund request.
func (s *Service) SubmitReturnTracking(ctx context.Context, req *pb.SubmitReturnTrackingRequest) (*pb.AfterSale, error) {
	afterSaleNo := strings.TrimSpace(req.GetAfterSaleNo())
	carrier := cutRunes(strings.TrimSpace(req.GetCarrier()), 64)
	tracking := cutRunes(strings.TrimSpace(req.GetTrackingNo()), 128)
	if afterSaleNo == "" || carrier == "" || tracking == "" {
		return nil, apperr.Invalid("after_sale_no, carrier and tracking_no are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin return tracking", err)
	}
	defer func() { _ = tx.Rollback() }()
	row, err := lockAfterSale(ctx, tx, afterSaleNo)
	if err != nil {
		return nil, err
	}
	if row.typ != typeReturnRefund {
		return nil, apperr.Failed("only RETURN_REFUND requests have return logistics")
	}
	if row.status == statusReturning || row.status == statusReceived {
		return s.loadAfterSale(ctx, afterSaleNo)
	}
	if row.status != statusApproved {
		return nil, apperr.Failed(fmt.Sprintf("after sale cannot submit return tracking in status %s", row.status))
	}
	if _, err := tx.ExecContext(ctx, "UPDATE after_sales SET status = ?, return_carrier = ?, return_tracking_no = ?, version = version + 1 WHERE id = ?",
		statusReturning, carrier, tracking, row.id); err != nil {
		return nil, apperr.Internal("update return tracking", err)
	}
	if err := appendLog(ctx, tx, row.id, "RETURN_TRACKING", row.status, statusReturning, carrier+" "+tracking); err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(afterSaleEvent{AfterSaleNo: afterSaleNo, OrderNo: row.orderNo, UserID: row.userID, Type: row.typ, Status: statusReturning, AmountCent: row.amountCent, Reason: row.reason})
	if err := outbox.Append(ctx, tx, outbox.Event{Topic: "shop.aftersale.returning.v1", AggregateType: aggregateAfterSale, AggregateID: afterSaleNo, EventType: "aftersale.returning", Payload: payload}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit return tracking", err)
	}
	return s.loadAfterSale(ctx, afterSaleNo)
}

// ReceiveReturn marks the returned parcel as received and restocks sellable
// units. The inventory restock key is the after-sale number, so retries are safe.
func (s *Service) ReceiveReturn(ctx context.Context, req *pb.ReceiveReturnRequest) (*pb.AfterSale, error) {
	afterSaleNo := strings.TrimSpace(req.GetAfterSaleNo())
	if afterSaleNo == "" {
		return nil, apperr.Invalid("after_sale_no is required")
	}
	condition := strings.ToUpper(strings.TrimSpace(req.GetCondition()))
	if condition == "" {
		condition = "SELLABLE"
	}
	if condition != "SELLABLE" && condition != "DAMAGED" {
		return nil, apperr.Invalid("condition must be SELLABLE or DAMAGED")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin receive return", err)
	}
	row, err := lockAfterSale(ctx, tx, afterSaleNo)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if row.typ != typeReturnRefund {
		_ = tx.Rollback()
		return nil, apperr.Failed("only RETURN_REFUND requests can be received")
	}
	if row.status == statusReceived {
		_ = tx.Rollback()
		if err := s.restockReturn(ctx, afterSaleNo, row.orderNo, condition); err != nil {
			return nil, err
		}
		return s.loadAfterSale(ctx, afterSaleNo)
	}
	if row.status != statusReturning {
		_ = tx.Rollback()
		return nil, apperr.Failed(fmt.Sprintf("after sale cannot be received in status %s", row.status))
	}
	if _, err := tx.ExecContext(ctx, "UPDATE after_sales SET status = ?, version = version + 1 WHERE id = ?", statusReceived, row.id); err != nil {
		_ = tx.Rollback()
		return nil, apperr.Internal("mark return received", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE after_sale_items SET received_quantity = quantity, condition_code = ? WHERE after_sale_id = ?", condition, row.id); err != nil {
		_ = tx.Rollback()
		return nil, apperr.Internal("mark items received", err)
	}
	if err := appendLog(ctx, tx, row.id, "RECEIVE_RETURN", row.status, statusReceived, condition); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	payload, _ := json.Marshal(afterSaleEvent{AfterSaleNo: afterSaleNo, OrderNo: row.orderNo, UserID: row.userID, Type: row.typ, Status: statusReceived, AmountCent: row.amountCent, Reason: row.reason})
	if err := outbox.Append(ctx, tx, outbox.Event{Topic: "shop.aftersale.received.v1", AggregateType: aggregateAfterSale, AggregateID: afterSaleNo, EventType: "aftersale.received", Payload: payload}); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit receive return", err)
	}
	if err := s.restockReturn(ctx, afterSaleNo, row.orderNo, condition); err != nil {
		return nil, err
	}
	return s.loadAfterSale(ctx, afterSaleNo)
}

func (s *Service) restockReturn(ctx context.Context, afterSaleNo, orderNo, condition string) error {
	if s.inventory == nil {
		return apperr.Unavailable("inventory client is not configured", nil)
	}
	items, err := s.loadItems(ctx, afterSaleNo)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.GetQuantity() <= 0 {
			continue
		}
		if _, err := s.inventory.Restock(ctx, &pb.RestockRequest{
			OrderNo: orderNo, SkuId: item.GetSkuId(), WarehouseId: 1,
			Quantity: int64(item.GetQuantity()), Condition: condition,
			Reason: "after-sale " + afterSaleNo, RestockNo: afterSaleNo,
		}); err != nil {
			return apperr.Unavailable("restock returned goods", err)
		}
	}
	return nil
}

// ListAfterSales returns the user/admin after-sale list.
func (s *Service) ListAfterSales(ctx context.Context, req *pb.ListAfterSalesRequest) (*pb.AfterSaleList, error) {
	page, size := req.GetPage(), req.GetPageSize()
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	where := "WHERE 1 = 1"
	args := []any{}
	if req.GetAdmin() {
		if req.GetUserId() > 0 {
			where += " AND user_id = ?"
			args = append(args, req.GetUserId())
		}
	} else {
		if req.GetUserId() <= 0 {
			return nil, apperr.Unauthenticated("user_id is required")
		}
		where += " AND user_id = ?"
		args = append(args, req.GetUserId())
	}
	if status := strings.TrimSpace(req.GetStatus()); status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM after_sales "+where, args...).Scan(&total); err != nil {
		return nil, apperr.Internal("count after sales", err)
	}
	args = append(args, size, (page-1)*size)
	rows, err := s.db.QueryContext(ctx, "SELECT after_sale_no FROM after_sales "+where+" ORDER BY id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, apperr.Internal("list after sales", err)
	}
	defer rows.Close()
	reply := &pb.AfterSaleList{Total: total}
	for rows.Next() {
		var no string
		if err := rows.Scan(&no); err != nil {
			return nil, apperr.Internal("scan after sale", err)
		}
		as, err := s.loadAfterSale(ctx, no)
		if err != nil {
			return nil, err
		}
		reply.AfterSales = append(reply.AfterSales, as)
	}
	return reply, rows.Err()
}

// afterSaleRow is the mutable state needed by the state machine.
type afterSaleRow struct {
	id             int64
	orderNo        string
	userID         int64
	typ            string
	status         string
	amountCent     int64
	reason         string
	returnCarrier  string
	returnTracking string
	refundNo       string
	refundedCent   int64
}

func lockAfterSale(ctx context.Context, tx *sql.Tx, afterSaleNo string) (*afterSaleRow, error) {
	var row afterSaleRow
	err := tx.QueryRowContext(ctx, "SELECT id, order_no, user_id, `type`, status, amount_cent, reason, return_carrier, return_tracking_no, refund_no, refunded_cent FROM after_sales WHERE after_sale_no = ? FOR UPDATE", afterSaleNo).
		Scan(&row.id, &row.orderNo, &row.userID, &row.typ, &row.status, &row.amountCent, &row.reason, &row.returnCarrier, &row.returnTracking, &row.refundNo, &row.refundedCent)
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
	err := s.db.QueryRowContext(ctx, "SELECT after_sale_no, order_no, user_id, `type`, status, amount_cent, reason, return_carrier, return_tracking_no, refund_no, refunded_cent, created_at FROM after_sales WHERE after_sale_no = ?", afterSaleNo).
		Scan(&as.AfterSaleNo, &as.OrderNo, &as.UserId, &as.Type, &as.Status, &as.AmountCent, &as.Reason, &as.ReturnCarrier, &as.ReturnTrackingNo, &as.RefundNo, &as.RefundedCent, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("after sale not found")
	}
	if err != nil {
		return nil, apperr.Internal("get after sale", err)
	}
	as.CreatedAtUnix = createdAt.Unix()
	as.Items, err = s.loadItems(ctx, afterSaleNo)
	if err != nil {
		return nil, err
	}
	return &as, nil
}

func (s *Service) loadItems(ctx context.Context, afterSaleNo string) ([]*pb.AfterSaleItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.sku_id, i.quantity FROM after_sale_items i JOIN after_sales a ON a.id = i.after_sale_id WHERE a.after_sale_no = ? ORDER BY i.id`, afterSaleNo)
	if err != nil {
		return nil, apperr.Internal("load after sale items", err)
	}
	defer rows.Close()
	var items []*pb.AfterSaleItem
	for rows.Next() {
		var item pb.AfterSaleItem
		if err := rows.Scan(&item.SkuId, &item.Quantity); err != nil {
			return nil, apperr.Internal("scan after sale item", err)
		}
		items = append(items, &item)
	}
	return items, rows.Err()
}

func appendLog(ctx context.Context, tx *sql.Tx, afterSaleID int64, action, fromStatus, toStatus, remark string) error {
	remark = cutRunes(remark, 512)
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
	in.reason = cutRunes(strings.TrimSpace(reason), 512)

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

func cutRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

const clientRequestIDMetadata = "x-client-request-id"

// clientRequestID reads the idempotency key forwarded by the BFF.
func clientRequestID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(clientRequestIDMetadata)
	if len(values) == 0 {
		return ""
	}
	key := strings.TrimSpace(values[0])
	if len(key) > 64 {
		key = key[:64]
	}
	return key
}

func (s *Service) findByClientRequest(ctx context.Context, userID int64, requestID string) (*pb.AfterSale, error) {
	var afterSaleNo string
	err := s.db.QueryRowContext(ctx, `SELECT after_sale_no FROM after_sales
		WHERE user_id = ? AND client_request_id = ? LIMIT 1`, userID, requestID).Scan(&afterSaleNo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperr.Internal("query after sale by client request", err)
	}
	return s.loadAfterSale(ctx, afterSaleNo)
}

func isDuplicateKey(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate entry")
}
