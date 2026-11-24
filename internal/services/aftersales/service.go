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
	"gin-kratos-gorm/internal/platform/events"
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
	statusRequested      = "REQUESTED"
	statusApproved       = "APPROVED"
	statusRejected       = "REJECTED"
	statusRefunding      = "REFUNDING"
	statusReturning      = "RETURNING"
	statusReturnReceived = "RETURN_RECEIVED"
	statusRefunded       = "REFUNDED"

	amountActive   = "ACTIVE"
	amountConsumed = "CONSUMED"
	amountReleased = "RELEASED"

	restockNone       = "NONE"
	restockPending    = "PENDING"
	restockProcessing = "PROCESSING"
	restockDone       = "DONE"
	restockFailed     = "FAILED_PERMANENT"

	taskPending    = "PENDING"
	taskProcessing = "PROCESSING"
	taskDone       = "DONE"
	taskFailed     = "FAILED_PERMANENT"

	maxTaskAttempts = 8
)

// Outbox subjects. created/completed are part of the documented Phase 1 event
// set; approved/rejected are service-local additions on the same naming scheme.
const (
	topicAfterSaleCreated         = "shop.aftersale.created.v1"
	topicAfterSaleApproved        = "shop.aftersale.approved.v1"
	topicAfterSaleRejected        = "shop.aftersale.rejected.v1"
	topicAfterSaleReceived        = "shop.aftersale.received.v1"
	topicAfterSaleReturning       = "shop.aftersale.returning.v1"
	topicAfterSaleRestockPending  = "shop.aftersale.restock_pending.v1"
	topicAfterSaleRestocked       = "shop.aftersale.restocked.v1"
	topicAfterSaleRestockFailed   = "shop.aftersale.restock_failed.v1"
	topicAfterSaleRefundRequested = "shop.aftersale.refund_requested.v1"
	topicAfterSaleCompleted       = "shop.aftersale.completed.v1"
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
	// freightResponsibilityEnabled gates responsibility/holds; off keeps the
	// legacy unshipped-full/otherwise-none behavior.
	freightResponsibilityEnabled bool
}

// Options gates ADR 0005 §13.1 behavior.
type Options struct {
	FreightResponsibilityEnabled bool
}

func New(db *sql.DB, client *mq.Client, payments pb.PaymentServiceClient, inventory pb.InventoryServiceClient, orders pb.OrderServiceClient, logger *slog.Logger) *Service {
	return NewWithOptions(db, client, payments, inventory, orders, logger, Options{})
}

func NewWithOptions(db *sql.DB, client *mq.Client, payments pb.PaymentServiceClient, inventory pb.InventoryServiceClient, orders pb.OrderServiceClient, logger *slog.Logger, opts Options) *Service {
	return &Service{
		db: db, mq: client, payments: payments, inventory: inventory, orders: orders, log: logger,
		freightResponsibilityEnabled: opts.FreightResponsibilityEnabled,
	}
}

// validateAgainstOrder prevents after-sales from inventing money or stock. The
// amount cap now lives in the per-order amount ledger, not in the order's
// asynchronous refund projection.
func (s *Service) validateAgainstOrder(ctx context.Context, orderNo string, userID int64, typ string, items []afterSaleItem) (*pb.Order, error) {
	if s.orders == nil {
		return nil, apperr.Unavailable("order client is not configured", nil)
	}
	order, err := s.orders.GetOrder(ctx, &pb.GetOrderRequest{OrderNo: orderNo, UserId: userID})
	if err != nil {
		return nil, apperr.Unavailable("load order for after-sale validation", err)
	}
	if order.GetOrderNo() == "" || order.GetUserId() != userID {
		return nil, apperr.NotFound("order not found")
	}
	switch order.GetStatus() {
	case "PAID", "ALLOCATED", "SHIPPED", "COMPLETED":
	default:
		return nil, apperr.Failed(fmt.Sprintf("order is not eligible for after-sales in status %s", order.GetStatus()))
	}
	if order.GetRefundState() == "FULL" {
		return nil, apperr.Failed("order is already fully refunded")
	}
	if len(order.GetFulfillmentGroups()) > 1 && len(items) == 0 {
		return nil, apperr.Invalid("split orders require after-sale items so the fulfillment group is explicit")
	}
	groupStatuses := make(map[string]string, len(order.GetFulfillmentGroups()))
	for _, group := range order.GetFulfillmentGroups() {
		groupStatuses[group.GetGroupNo()] = group.GetStatus()
	}
	if typ == typeReturnRefund && len(groupStatuses) == 0 &&
		order.GetStatus() != "SHIPPED" && order.GetStatus() != "COMPLETED" {
		return nil, apperr.Failed("return refund requires a shipped or completed order")
	}
	if len(items) == 0 {
		if typ == typeReturnRefund {
			return nil, apperr.Invalid("return refund requires at least one item")
		}
		return order, nil
	}
	type lineKey struct {
		skuID       int64
		warehouseID int64
		groupNo     string
	}
	byLine := make(map[int32]*pb.OrderItem, len(order.GetItems()))
	byComposite := make(map[lineKey][]*pb.OrderItem, len(order.GetItems()))
	for _, item := range order.GetItems() {
		warehouseID := item.GetWarehouseId()
		if warehouseID <= 0 {
			warehouseID = 1
		}
		groupNo := item.GetGroupNo()
		if groupNo == "" {
			groupNo = "G1"
		}
		byLine[item.GetLineNo()] = item
		key := lineKey{skuID: item.GetSkuId(), warehouseID: warehouseID, groupNo: groupNo}
		byComposite[key] = append(byComposite[key], item)
	}
	var groupNo string
	for index := range items {
		item := &items[index]
		key := lineKey{skuID: item.SkuID, warehouseID: item.WarehouseID, groupNo: item.GroupNo}
		var line *pb.OrderItem
		if item.LineNo > 0 {
			line = byLine[item.LineNo]
			lineGroup := "G1"
			if line != nil && line.GetGroupNo() != "" {
				lineGroup = line.GetGroupNo()
			}
			lineWarehouse := int64(1)
			if line != nil && line.GetWarehouseId() > 0 {
				lineWarehouse = line.GetWarehouseId()
			}
			if line == nil || line.GetSkuId() != item.SkuID ||
				lineWarehouse != item.WarehouseID || lineGroup != item.GroupNo {
				return nil, apperr.Invalid("line %d does not match the order item", item.LineNo)
			}
		} else {
			matches := byComposite[key]
			if len(matches) != 1 {
				return nil, apperr.Invalid("line_no is required for sku %d: the SKU has multiple split lines", item.SkuID)
			}
			line = matches[0]
			item.LineNo = line.GetLineNo()
		}
		if typ == typeReturnRefund && len(groupStatuses) > 0 {
			status, ok := groupStatuses[item.GroupNo]
			if !ok || (status != "SHIPPED" && status != "RECEIVED") {
				return nil, apperr.Failed(fmt.Sprintf("return refund requires group %s to be shipped or received", item.GroupNo))
			}
		}
		if groupNo == "" {
			groupNo = item.GroupNo
		} else if groupNo != item.GroupNo {
			return nil, apperr.Invalid("one after-sale must belong to a single fulfillment group")
		}
		var existing int64
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(i.quantity), 0)
			FROM after_sale_items i JOIN after_sales a ON a.id = i.after_sale_id
			WHERE a.order_no = ? AND a.status <> 'REJECTED'
			  AND i.line_no = ?`,
			orderNo, item.LineNo).Scan(&existing); err != nil {
			return nil, apperr.Internal("sum after-sale line quantity", err)
		}
		if existing+int64(item.Quantity) > int64(line.GetQuantity()) {
			return nil, apperr.Invalid("return quantity for line %d exceeds the remaining quantity", item.LineNo).
				WithDetails(map[string]any{"line_no": item.LineNo, "remaining_quantity": int64(line.GetQuantity()) - existing})
		}
		item.LinePaidCent = line.GetLinePaidCent()
		item.UnitPaidCent = line.GetUnitPaidCent()
	}
	return order, nil
}

// CreateAfterSale opens a refund only / return-refund request. The request is
// stored as REQUESTED and an shop.aftersale.created.v1 event is emitted.
func (s *Service) CreateAfterSale(ctx context.Context, req *pb.CreateAfterSaleRequest) (*pb.AfterSale, error) {
	expectedCent := req.GetExpectedAmountCent()
	if expectedCent == 0 {
		expectedCent = req.GetAmountCent()
	}
	input, items, err := validateCreate(req.GetOrderNo(), req.GetUserId(), req.GetType(), req.GetReason(), expectedCent, req.GetItems())
	if err != nil {
		return nil, err
	}
	order, err := s.validateAgainstOrder(ctx, input.orderNo, input.userID, input.typ, items)
	if err != nil {
		return nil, err
	}
	capturedCent, capturedSource, err := s.capturedForOrder(ctx, order)
	if err != nil {
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

	party, responsibilitySource, reasonCode := autoResponsibility(input.typ, order)
	groupNo := "G1"
	if len(items) > 0 && items[0].GroupNo != "" {
		groupNo = items[0].GroupNo
	}
	computedCent := input.amountCent
	refunds, goodsCent, computedFreight, computedRule, err := s.computeLineRefunds(ctx, tx, order, toProtoItems(items), input.typ, true)
	if err != nil {
		return nil, err
	}
	freightCent := int64(0)
	holdFreight := false
	ruleID := "LINE_REFUND"
	if goodsCent > 0 {
		if s.freightResponsibilityEnabled {
			freightCent, holdFreight, ruleID, err = s.freightRefundFor(ctx, tx, order, groupNo, party)
			if err != nil {
				return nil, err
			}
		} else {
			freightCent, holdFreight, ruleID = computedFreight, false, computedRule
		}
		computedCent = goodsCent
		if !holdFreight {
			computedCent += freightCent
		}
	} else {
		refunds = nil
	}
	if goodsCent > 0 && capturedCent > 0 {
		ledgerPreview, err := s.ensureAmountLedgerTx(ctx, tx, input.orderNo, capturedCent, capturedSource)
		if err != nil {
			return nil, err
		}
		if available := ledgerPreview.captured - ledgerPreview.consumed; computedCent > available {
			computedCent = available
		}
	}
	ledger, err := s.ensureAmountLedgerTx(ctx, tx, input.orderNo, capturedCent, capturedSource)
	if err != nil {
		return nil, err
	}
	if err := reserveAmountTx(ctx, tx, ledger, computedCent); err != nil {
		return nil, err
	}
	key := sql.NullString{String: requestID, Valid: requestID != ""}
	res, err := tx.ExecContext(ctx, "INSERT INTO after_sales (after_sale_no, order_no, user_id, `type`, status, amount_cent, effective_amount_cent, amount_state, restock_state, reason, client_request_id, source, responsibility_party, responsibility_source, responsibility_evidence_ref, responsibility_reason_code, responsibility_determined_by, responsibility_determined_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'USER', ?, ?, ?, ?, ?, NOW(3))",
		afterSaleNo, input.orderNo, input.userID, input.typ, statusRequested, input.amountCent, computedCent, amountActive, restockNone, input.reason, key,
		party, responsibilitySource, "system:"+reasonCode, reasonCode, "system")
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
	if err := appendAmountVersionTx(ctx, tx, afterSaleID, 1, computedCent, "CREATE"); err != nil {
		return nil, err
	}
	pendingByLine := make(map[int32]int64, len(refunds))
	for _, refund := range refunds {
		pendingByLine[refund.LineNo] = refund.AmountCent
	}
	for i := range items {
		items[i].PendingAmountCent = pendingByLine[items[i].LineNo]
	}
	for _, it := range items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO after_sale_items
			(after_sale_id, sku_id, quantity, warehouse_id, group_no, line_no,
			 line_paid_cent_snapshot, unit_paid_cent_snapshot, pending_amount_cent)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			afterSaleID, it.SkuID, it.Quantity, it.WarehouseID, it.GroupNo, it.LineNo,
			it.LinePaidCent, it.UnitPaidCent, it.PendingAmountCent); err != nil {
			return nil, apperr.Internal("create after sale item", err)
		}
	}
	for _, refund := range refunds {
		if err := s.reservePendingLineTx(ctx, tx, input.orderNo, refund.LineNo, refund.Quantity, refund.AmountCent); err != nil {
			return nil, err
		}
	}
	if err := appendLog(ctx, tx, afterSaleID, "CREATE", "", statusRequested, input.reason); err != nil {
		return nil, err
	}
	if holdFreight && freightCent > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO after_sale_freight_holds
			(after_sale_no, freight_cent, status, deadline_at) VALUES (?, ?, ?, DATE_ADD(NOW(3), INTERVAL 168 HOUR))`,
			afterSaleNo, freightCent, freightStateHeld); err != nil {
			return nil, apperr.Internal("create freight hold", err)
		}
	}
	_ = refunds
	_ = ruleID
	payload, err := json.Marshal(afterSaleEvent{
		AfterSaleNo: afterSaleNo, OrderNo: input.orderNo, UserID: input.userID,
		Type: input.typ, Status: statusRequested, AmountCent: input.amountCent,
		EffectiveAmountCent: computedCent, AmountState: amountActive, RestockState: restockNone,
		Reason: input.reason,
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
// lower than the requested amount records a new effective-amount version and
// releases the difference back to the order's lifetime ledger.
func (s *Service) Approve(ctx context.Context, req *pb.ApproveRequest) (*pb.AfterSale, error) {
	afterSaleNo := strings.TrimSpace(req.GetAfterSaleNo())
	if afterSaleNo == "" {
		return nil, apperr.Invalid("after_sale_no is required")
	}
	pre, err := s.getAfterSaleRow(ctx, afterSaleNo)
	if err != nil {
		return nil, err
	}
	order, err := s.loadOrder(ctx, pre.orderNo, pre.userID)
	if err != nil {
		return nil, err
	}
	if order.GetRefundState() == "FULL" {
		return nil, apperr.Failed("order is already fully refunded")
	}
	capturedCent, capturedSource, err := s.capturedForOrder(ctx, order)
	if err != nil {
		return nil, err
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
	if !changed {
		if err := tx.Commit(); err != nil {
			return nil, apperr.Internal("commit idempotent approval", err)
		}
		return s.loadAfterSale(ctx, afterSaleNo)
	}
	nextEffective := row.effectiveAmountCent
	nextAmountState := amountActive
	if next == statusRejected {
		nextEffective = row.effectiveAmountCent
		nextAmountState = amountReleased
		if err := s.releasePendingForAfterSaleTx(ctx, tx, afterSaleNo); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE after_sale_freight_holds
			SET status = ?, determined_party = ?, determined_at = NOW(3), version = version + 1
			WHERE after_sale_no = ? AND status = ?`,
			freightStateWaived, partyCustomer, afterSaleNo, freightStateHeld); err != nil {
			return nil, apperr.Internal("waive rejected freight hold", err)
		}
	} else if req.GetAmountCent() > 0 {
		var newLineCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM after_sale_items
			WHERE after_sale_id = ? AND line_no > 0`, row.id).Scan(&newLineCount); err != nil {
			return nil, apperr.Internal("check line snapshot for legacy amount", err)
		}
		if newLineCount > 0 {
			return nil, apperr.Invalid("direct amount adjustment is forbidden; adjust quantity instead")
		}
		if req.GetAmountCent() > row.amountCent {
			return nil, apperr.Invalid("approved amount_cent exceeds the requested amount")
		}
		if strings.TrimSpace(req.GetReason()) == "" {
			return nil, apperr.Invalid("legacy amount adjustment requires a reason")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO after_sale_legacy_amount_adjustments
			(after_sale_no, operator, reason, amount_cent, approved_by) VALUES (?, ?, ?, ?, ?)`,
			afterSaleNo, "admin", cutRunes(req.GetReason(), 255), req.GetAmountCent(), "admin"); err != nil {
			return nil, apperr.Internal("audit legacy amount adjustment", err)
		}
		nextEffective = req.GetAmountCent()
	}
	ledger, err := s.ensureAmountLedgerTx(ctx, tx, row.orderNo, capturedCent, capturedSource)
	if err != nil {
		return nil, err
	}
	delta := nextEffective - row.effectiveAmountCent
	if next == statusRejected {
		delta = -row.effectiveAmountCent
	}
	if delta > 0 {
		if err := reserveAmountTx(ctx, tx, ledger, delta); err != nil {
			return nil, err
		}
	} else if delta < 0 {
		if err := releaseAmountTx(ctx, tx, ledger, -delta); err != nil {
			return nil, err
		}
	}
	action := "APPROVE"
	topic, eventType := topicAfterSaleApproved, "aftersale.approved"
	if next == statusRejected {
		action = "REJECT"
		topic, eventType = topicAfterSaleRejected, "aftersale.rejected"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sales
		SET status = ?, effective_amount_cent = ?, amount_state = ?, version = version + 1
		WHERE id = ?`, next, nextEffective, nextAmountState, row.id); err != nil {
		return nil, apperr.Internal("update after sale status", err)
	}
	if err := appendAmountVersionTx(ctx, tx, row.id, row.version+1, nextEffective, action); err != nil {
		return nil, err
	}
	if err := appendLog(ctx, tx, row.id, action, row.status, next, req.GetReason()); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(afterSaleEvent{
		AfterSaleNo: afterSaleNo, OrderNo: row.orderNo, UserID: row.userID,
		Type: row.typ, Status: next, AmountCent: row.amountCent,
		EffectiveAmountCent: nextEffective, AmountState: nextAmountState,
		Reason: row.reason,
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
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit approve after sale", err)
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
		if row.status != statusReturnReceived && row.status != statusRefunding {
			return apperr.Failed("return-refund after sale must be received before refunding")
		}
		if row.restockState != restockDone {
			return apperr.Failed("return-refund stock must be restocked before refunding")
		}
	default:
		return apperr.Failed("unsupported after sale type")
	}
	return nil
}

// groupForAfterSale resolves the single fulfillment group of the request.
// Phase 5 deliberately requires one group per after-sale so a refund cannot
// silently revoke a permit for another group.
func (s *Service) groupForAfterSale(ctx context.Context, afterSaleNo string) (string, error) {
	items, err := s.loadItems(ctx, afterSaleNo)
	if err != nil {
		return "", err
	}
	groupNo := ""
	for _, item := range items {
		current := strings.TrimSpace(item.GetGroupNo())
		if current == "" {
			current = "G1"
		}
		if groupNo == "" {
			groupNo = current
			continue
		}
		if groupNo != current {
			return "", apperr.Failed("one after-sale must belong to a single fulfillment group")
		}
	}
	if groupNo == "" {
		groupNo = "G1"
	}
	return groupNo, nil
}

// Refund persists REFUNDING plus a durable refund task, then runs the refund
// worker inline. The task/lease is written before any remote Order admission
// (R-C): a zero-amount after-sale can therefore retry CancelIfCreated after a
// crash, and a failed admission never loses the intent. Payment is idempotent
// by after_sale_no; a failed inline attempt is retried by the refund worker,
// never by a status scanner.
func (s *Service) Refund(ctx context.Context, req *pb.RefundAfterSaleRequest) (*pb.AfterSale, error) {
	afterSaleNo := strings.TrimSpace(req.GetAfterSaleNo())
	if afterSaleNo == "" {
		return nil, apperr.Invalid("after_sale_no is required")
	}
	pre, err := s.getAfterSaleRow(ctx, afterSaleNo)
	if err != nil {
		return nil, err
	}
	if _, err := s.loadOrder(ctx, pre.orderNo, pre.userID); err != nil {
		return nil, err
	}
	if pre.status == statusRefunded {
		return s.loadAfterSale(ctx, afterSaleNo)
	}
	if err := refundableStatus(pre); err != nil {
		return nil, err
	}
	groupNo, err := s.groupForAfterSale(ctx, afterSaleNo)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin refund after sale", err)
	}
	defer func() { _ = tx.Rollback() }()
	row, err := lockAfterSale(ctx, tx, afterSaleNo)
	if err != nil {
		return nil, err
	}
	if row.status == statusRefunded {
		if err := tx.Commit(); err != nil {
			return nil, apperr.Internal("commit idempotent refund", err)
		}
		return s.loadAfterSale(ctx, afterSaleNo)
	}
	if row.status == statusRefunding {
		if err := ensureRefundTaskTx(ctx, tx, afterSaleNo); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, apperr.Internal("commit refund task replay", err)
		}
	} else {
		if err := refundableStatus(row); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE after_sales
			SET status = ?, version = version + 1 WHERE id = ?`, statusRefunding, row.id); err != nil {
			return nil, apperr.Internal("mark after sale refunding", err)
		}
		if err := ensureRefundTaskTx(ctx, tx, afterSaleNo); err != nil {
			return nil, err
		}
		intent, _ := json.Marshal(map[string]any{
			"after_sale_no": afterSaleNo, "order_no": row.orderNo, "user_id": row.userID,
			"amount_cent": row.effectiveAmountCent, "payment": "pending",
		})
		if err := appendLog(ctx, tx, row.id, "REFUND_INTENT", row.status, statusRefunding, string(intent)); err != nil {
			return nil, err
		}
		payload, _ := json.Marshal(events.AfterSaleRefundRequested{
			AfterSaleNo: afterSaleNo, OrderNo: row.orderNo, GroupNo: groupNo, UserID: row.userID,
			Type: row.typ, Status: statusRefunding, AmountCent: row.amountCent,
			EffectiveAmountCent: row.effectiveAmountCent, AmountState: row.amountState,
			RestockState: row.restockState, Reason: row.reason, RefundIntent: true,
		})
		if err := outbox.Append(ctx, tx, outbox.Event{
			Topic: topicAfterSaleRefundRequested, AggregateType: aggregateAfterSale,
			AggregateID: afterSaleNo, EventType: "aftersale.refund_requested", Payload: payload,
		}); err != nil {
			return nil, apperr.Internal("append refund requested", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, apperr.Internal("commit refund intent", err)
		}
	}

	if err := s.processRefundTask(ctx, afterSaleNo); err != nil {
		// The intent and task are committed; an inline Payment failure must not
		// lose the refund. The worker retries idempotently.
		if s.log != nil {
			s.log.Warn("after sale refund queued for retry", "after_sale_no", afterSaleNo, "error", err)
		}
		return nil, apperr.Unavailable("refund queued for retry", err)
	}
	return s.loadAfterSale(ctx, afterSaleNo)
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
	if row.status == statusReturning || row.status == statusReturnReceived {
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

// ReceiveReturn records that the parcel arrived and queues durable per-SKU
// restock tasks. RETURN_RECEIVED does not mean stock was restored: refund
// admission requires restock_state=DONE.
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
	items, err := s.loadItems(ctx, afterSaleNo)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin receive return", err)
	}
	defer func() { _ = tx.Rollback() }()
	row, err := lockAfterSale(ctx, tx, afterSaleNo)
	if err != nil {
		return nil, err
	}
	if row.typ != typeReturnRefund {
		return nil, apperr.Failed("only RETURN_REFUND requests can be received")
	}
	if row.status == statusReturnReceived {
		// An explicit Receive retry re-arms permanently failed restock tasks;
		// successful/DONE tasks are never replayed.
		if row.restockState == restockFailed {
			if _, err := tx.ExecContext(ctx, `UPDATE after_sale_restock_tasks
				SET status = ?, attempts = 0, last_error = '', lease_owner = '', lease_until = NULL, updated_at = NOW(3)
				WHERE after_sale_no = ? AND status = ?`, taskPending, afterSaleNo, taskFailed); err != nil {
				return nil, apperr.Internal("re-arm restock tasks", err)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE after_sales SET restock_state = ?, version = version + 1 WHERE id = ?`,
				restockPending, row.id); err != nil {
				return nil, apperr.Internal("re-arm restock state", err)
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, apperr.Internal("commit idempotent receive", err)
		}
		return s.loadAfterSale(ctx, afterSaleNo)
	}
	if row.status != statusReturning {
		return nil, apperr.Failed(fmt.Sprintf("after sale cannot be received in status %s", row.status))
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sales
		SET status = ?, restock_state = ?, version = version + 1 WHERE id = ?`,
		statusReturnReceived, restockPending, row.id); err != nil {
		return nil, apperr.Internal("mark return received", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sale_items
		SET received_quantity = quantity, condition_code = ? WHERE after_sale_id = ?`, condition, row.id); err != nil {
		return nil, apperr.Internal("mark items received", err)
	}
	for _, item := range items {
		if item.GetQuantity() <= 0 {
			continue
		}
		warehouseID := item.GetWarehouseId()
		if warehouseID <= 0 {
			warehouseID = 1
		}
		groupNo := item.GetGroupNo()
		if groupNo == "" {
			groupNo = "G1"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO after_sale_restock_tasks
			(after_sale_no, sku_id, warehouse_id, group_no, quantity, `+"`condition`"+`, status, attempts, last_error, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0, '', NOW(3), NOW(3))
			ON DUPLICATE KEY UPDATE after_sale_no = after_sale_no`,
			afterSaleNo, item.GetSkuId(), warehouseID, groupNo, item.GetQuantity(), condition, taskPending); err != nil {
			return nil, apperr.Internal("queue restock task", err)
		}
	}
	if err := appendLog(ctx, tx, row.id, "RECEIVE_RETURN", row.status, statusReturnReceived, condition); err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(afterSaleEvent{
		AfterSaleNo: afterSaleNo, OrderNo: row.orderNo, UserID: row.userID,
		Type: row.typ, Status: statusReturnReceived, AmountCent: row.amountCent,
		EffectiveAmountCent: row.effectiveAmountCent, AmountState: row.amountState,
		RestockState: restockPending, Reason: row.reason,
	})
	if err := outbox.Append(ctx, tx, outbox.Event{
		Topic: topicAfterSaleReceived, AggregateType: aggregateAfterSale,
		AggregateID: afterSaleNo, EventType: "aftersale.received", Payload: payload,
	}); err != nil {
		return nil, err
	}
	if err := outbox.Append(ctx, tx, outbox.Event{
		Topic: topicAfterSaleRestockPending, AggregateType: aggregateAfterSale,
		AggregateID: afterSaleNo, EventType: "aftersale.restock_pending", Payload: payload,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit receive return", err)
	}
	return s.loadAfterSale(ctx, afterSaleNo)
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
	id                  int64
	orderNo             string
	userID              int64
	typ                 string
	status              string
	amountCent          int64
	effectiveAmountCent int64
	amountState         string
	restockState        string
	version             int64
	reason              string
	returnCarrier       string
	returnTracking      string
	refundNo            string
	refundedCent        int64
	source              string
	responsibilityParty string
	responsibilitySrc   string
	responsibilityRef   string
	responsibilityCode  string
	determinedBy        string
	determinedAt        sql.NullTime
	returnCarrierCode   string
}

func lockAfterSale(ctx context.Context, tx *sql.Tx, afterSaleNo string) (*afterSaleRow, error) {
	var row afterSaleRow
	err := tx.QueryRowContext(ctx, "SELECT id, order_no, user_id, `type`, status, amount_cent, effective_amount_cent, amount_state, restock_state, version, reason, return_carrier, return_tracking_no, refund_no, refunded_cent, source, responsibility_party, responsibility_source, responsibility_evidence_ref, responsibility_reason_code, responsibility_determined_by, responsibility_determined_at, return_carrier_code FROM after_sales WHERE after_sale_no = ? FOR UPDATE", afterSaleNo).
		Scan(&row.id, &row.orderNo, &row.userID, &row.typ, &row.status, &row.amountCent, &row.effectiveAmountCent, &row.amountState, &row.restockState, &row.version, &row.reason, &row.returnCarrier, &row.returnTracking, &row.refundNo, &row.refundedCent, &row.source,
			&row.responsibilityParty, &row.responsibilitySrc, &row.responsibilityRef, &row.responsibilityCode, &row.determinedBy, &row.determinedAt, &row.returnCarrierCode)
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
	var responsibilityDeterminedAt, freightDeadline sql.NullTime
	err := s.db.QueryRowContext(ctx, "SELECT a.after_sale_no, a.order_no, a.user_id, a.`type`, a.status, a.amount_cent, a.effective_amount_cent, a.amount_state, a.restock_state, a.reason, a.return_carrier, a.return_tracking_no, a.refund_no, a.refunded_cent, a.created_at, a.responsibility_party, a.responsibility_source, a.responsibility_evidence_ref, a.responsibility_reason_code, a.responsibility_determined_by, a.responsibility_determined_at, a.return_carrier_code, COALESCE(h.status, 'NONE'), COALESCE(h.freight_cent, 0), h.deadline_at FROM after_sales a LEFT JOIN after_sale_freight_holds h ON h.after_sale_no = a.after_sale_no WHERE a.after_sale_no = ?", afterSaleNo).
		Scan(&as.AfterSaleNo, &as.OrderNo, &as.UserId, &as.Type, &as.Status, &as.AmountCent, &as.EffectiveAmountCent, &as.AmountState, &as.RestockState, &as.Reason, &as.ReturnCarrier, &as.ReturnTrackingNo, &as.RefundNo, &as.RefundedCent, &createdAt,
			&as.ResponsibilityParty, &as.ResponsibilitySource, &as.ResponsibilityEvidenceRef, &as.ResponsibilityReasonCode, &as.ResponsibilityDeterminedBy, &responsibilityDeterminedAt, &as.ReturnCarrierCode,
			&as.FreightState, &as.FreightCent, &freightDeadline)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("after sale not found")
	}
	if err != nil {
		return nil, apperr.Internal("get after sale", err)
	}
	as.CreatedAtUnix = createdAt.Unix()
	if responsibilityDeterminedAt.Valid {
		as.ResponsibilityDeterminedAtUnix = responsibilityDeterminedAt.Time.Unix()
	}
	if freightDeadline.Valid {
		as.FreightDeadlineUnix = freightDeadline.Time.Unix()
	}
	as.Items, err = s.loadItems(ctx, afterSaleNo)
	if err != nil {
		return nil, err
	}
	return &as, nil
}

func (s *Service) loadItems(ctx context.Context, afterSaleNo string) ([]*pb.AfterSaleItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.sku_id, i.quantity, i.warehouse_id, i.group_no,
		i.line_no, i.line_paid_cent_snapshot, i.unit_paid_cent_snapshot
		FROM after_sale_items i JOIN after_sales a ON a.id = i.after_sale_id
		WHERE a.after_sale_no = ? ORDER BY i.line_no, i.id`, afterSaleNo)
	if err != nil {
		return nil, apperr.Internal("load after sale items", err)
	}
	defer rows.Close()
	var items []*pb.AfterSaleItem
	for rows.Next() {
		var item pb.AfterSaleItem
		if err := rows.Scan(&item.SkuId, &item.Quantity, &item.WarehouseId, &item.GroupNo,
			&item.LineNo, &item.LinePaidCent, &item.UnitPaidCent); err != nil {
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
	SkuID             int64  `json:"sku_id"`
	Quantity          int32  `json:"quantity"`
	WarehouseID       int64  `json:"warehouse_id,omitempty"`
	GroupNo           string `json:"group_no,omitempty"`
	LineNo            int32  `json:"line_no,omitempty"`
	LinePaidCent      int64  `json:"line_paid_cent,omitempty"`
	UnitPaidCent      int64  `json:"unit_paid_cent,omitempty"`
	PendingAmountCent int64  `json:"pending_amount_cent,omitempty"`
}

// afterSaleEvent is the outbox payload for shop.aftersale.* events.
type afterSaleEvent struct {
	AfterSaleNo         string `json:"after_sale_no"`
	OrderNo             string `json:"order_no"`
	UserID              int64  `json:"user_id"`
	Type                string `json:"type"`
	Status              string `json:"status"`
	AmountCent          int64  `json:"amount_cent"`
	EffectiveAmountCent int64  `json:"effective_amount_cent,omitempty"`
	AmountState         string `json:"amount_state,omitempty"`
	RestockState        string `json:"restock_state,omitempty"`
	Reason              string `json:"reason"`
	GroupNo             string `json:"group_no,omitempty"`
	RefundIntent        bool   `json:"refund_intent,omitempty"`
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
	if amountCent < 0 {
		return afterSaleInput{}, nil, apperr.Invalid("amount_cent must not be negative")
	}
	if amountCent == 0 && len(items) == 0 {
		return afterSaleInput{}, nil, apperr.Invalid("amount_cent or after-sale items are required")
	}
	in.amountCent = amountCent
	in.reason = cutRunes(strings.TrimSpace(reason), 512)

	merged := make([]afterSaleItem, 0, len(items))
	type itemKey struct {
		lineNo int32
		skuID  int64
	}
	index := make(map[itemKey]int, len(items))
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
		warehouseID := item.GetWarehouseId()
		if warehouseID <= 0 {
			warehouseID = 1
		}
		groupNo := strings.TrimSpace(item.GetGroupNo())
		if groupNo == "" {
			groupNo = "G1"
		}
		lineNo := item.GetLineNo()
		if lineNo < 0 {
			return afterSaleInput{}, nil, apperr.Invalid("line_no must not be negative")
		}
		key := itemKey{lineNo: lineNo, skuID: skuID}
		if lineNo == 0 {
			// Legacy callers did not send line_no; merge by SKU so one line per
			// SKU remains. validateAgainstOrder resolves the actual line.
			key = itemKey{skuID: skuID}
		}
		if pos, ok := index[key]; ok {
			sum := int64(merged[pos].Quantity) + int64(item.GetQuantity())
			if sum > math.MaxInt32 {
				return afterSaleInput{}, nil, apperr.Invalid("quantity overflow for sku %d", skuID)
			}
			merged[pos].Quantity = int32(sum)
			continue
		}
		index[key] = len(merged)
		merged = append(merged, afterSaleItem{SkuID: skuID, Quantity: item.GetQuantity(), WarehouseID: warehouseID, GroupNo: groupNo, LineNo: lineNo})
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

func autoResponsibility(typ string, order *pb.Order) (party, source, code string) {
	if typ == typeRefundOnly && orderUnshipped(order) {
		return partyMerchant, sourceSystemValue, "OUT_OF_STOCK"
	}
	return partyUndetermined, sourceSystemValue, "PENDING_REVIEW"
}
