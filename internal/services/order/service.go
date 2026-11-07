package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/dtm"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// defaultInventoryDTMURL / defaultOrderDTMURL are direct DNS targets.
	// Kubernetes overrides them with *.mall.svc.cluster.local.
	defaultInventoryDTMURL = "inventory:9003/mall.v1.InventoryService"
	defaultOrderDTMURL     = "order:9006/mall.v1.OrderService"

	inventoryReserveMethod  = "/Reserve"
	inventoryReleaseMethod  = "/Release"
	orderCreateInnerMethod  = "/CreateOrderInner"
	orderCancelInnerMethod  = "/CancelOrderInner"
	defaultWarehouseID      = int64(1)
	reserveTTLSeconds       = int64(900)
	orderExpireWindow       = 15 * time.Minute
	clientRequestIDHeader   = "x-client-request-id"
	maxClientRequestIDBytes = 64
	maxOrderNoBytes         = 64
)

// Topics published by this service.
const (
	topicOrderCreated   = "shop.order.created.v1"
	topicOrderPaid      = "shop.order.paid.v1"
	topicOrderCancelled = "shop.order.cancelled.v1"
	topicOrderAllocated = "shop.order.allocated.v1"
	topicOrderShipped   = "shop.order.shipped.v1"
	topicOrderCompleted = "shop.order.completed.v1"
)

// Service implements pb.OrderServiceServer on top of MySQL (database/sql) and
// DTM sagas.
type Service struct {
	pb.UnimplementedOrderServiceServer

	db        *sql.DB
	mq        *mq.Client
	log       *slog.Logger
	dtm       *dtm.Client
	pricing   pricingPort
	inventory inventoryPort

	inventoryDTM string
	orderDTM     string
}

// New builds the order service. pricing and inventory may be nil in tests that
// only exercise local logic.
func New(database *sql.DB, client *mq.Client, logger *slog.Logger, dtmClient *dtm.Client,
	pricing pricingPort, inventory inventoryPort) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		db:           database,
		mq:           client,
		log:          logger,
		dtm:          dtmClient,
		pricing:      pricing,
		inventory:    inventory,
		inventoryDTM: strings.TrimRight(env("INVENTORY_DTM_URL", defaultInventoryDTMURL), "/"),
		orderDTM:     strings.TrimRight(env("ORDER_DTM_URL", defaultOrderDTMURL), "/"),
	}
}

// ---------------------------------------------------------------- create

// CreateOrder quotes the cart, then runs a DTM saga:
//
//	inventory Reserve (per line) -> order CreateOrderInner
//
// The compensations release the reservations and cancel the order. The saga gid
// is deterministic so retries of the same client request are idempotent.
func (s *Service) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.Order, error) {
	in, err := normalizeCreateOrder(req)
	if err != nil {
		return nil, err
	}
	existing, err := findOrderByClientRequest(ctx, s.db, in.UserID, in.ClientRequestID)
	if err != nil {
		return nil, apperr.Internal("load order by client request id", err)
	}
	if existing != nil {
		return s.replyFor(ctx, s.db, existing)
	}
	if s.pricing == nil {
		return nil, apperr.Unavailable("pricing client is not configured", nil)
	}
	quote, err := s.getQuote(ctx, in.QuoteID)
	if err != nil {
		return nil, err
	}
	orderNo := id.Order()
	inner := buildCreateOrderInner(orderNo, in, quote, quoteExpireAt(quote))
	if err := s.submitCreateSaga(orderNo, in.ClientRequestID, quote.GetLines(), inner); err != nil {
		// A concurrent request with the same idempotency key may have won the
		// saga (the unique index then aborts ours); prefer returning its order.
		if winner, ferr := findOrderByClientRequest(ctx, s.db, in.UserID, in.ClientRequestID); ferr == nil && winner != nil {
			s.log.Info("create order saga lost the idempotency race",
				"order_no", orderNo, "winner_order_no", winner.OrderNo)
			return s.replyFor(ctx, s.db, winner)
		}
		return nil, err
	}
	created, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load created order", err)
	}
	if created == nil {
		// A concurrent request holding the same (user_id, client_request_id)
		// idempotency key may have won the saga; return its order.
		winner, ferr := findOrderByClientRequest(ctx, s.db, in.UserID, in.ClientRequestID)
		if ferr != nil {
			return nil, apperr.Internal("load idempotent order", ferr)
		}
		if winner == nil {
			return nil, apperr.Internal("order was not persisted by the create saga", nil)
		}
		return s.replyFor(ctx, s.db, winner)
	}
	return s.replyFor(ctx, s.db, created)
}

// submitCreateSaga registers every quote line as an inventory reservation and
// finally the order itself, then submits the saga to DTM.
func (s *Service) submitCreateSaga(orderNo, clientRequestID string, lines []*pb.QuoteLine, inner *pb.CreateOrderInnerRequest) error {
	if s.dtm == nil {
		return apperr.Unavailable("dtm client is not configured", nil)
	}
	saga := s.dtm.Saga(createOrderGID(orderNo))
	// Branch headers are forwarded by DTM to every branch; they carry the
	// idempotency key that CreateOrderInnerRequest does not expose.
	if clientRequestID != "" {
		saga.BranchHeaders = map[string]string{clientRequestIDHeader: clientRequestID}
	}
	for _, line := range lines {
		reserve := &pb.ReserveRequest{
			OrderNo:     orderNo,
			SkuId:       line.GetSkuId(),
			WarehouseId: defaultWarehouseID,
			Quantity:    line.GetQuantity(),
			TtlSeconds:  reserveTTLSeconds,
		}
		saga.Add(s.inventoryDTM+inventoryReserveMethod, s.inventoryDTM+inventoryReleaseMethod, reserve)
	}
	saga.Add(s.orderDTM+orderCreateInnerMethod, s.orderDTM+orderCancelInnerMethod, inner)
	if err := saga.Submit(); err != nil {
		return apperr.Unavailable("submit create order saga", err)
	}
	return nil
}

// CreateOrderInner is the DTM branch that persists the order. It only runs
// inside a DTM branch barrier transaction and is idempotent by order_no.
func (s *Service) CreateOrderInner(ctx context.Context, req *pb.CreateOrderInnerRequest) (*pb.Order, error) {
	if err := validateCreateOrderInner(req); err != nil {
		return nil, err
	}
	if !dtm.InBranch(ctx) {
		return nil, apperr.Unavailable("CreateOrderInner is a DTM branch and must be invoked by DTM", nil)
	}
	orderNo := req.GetOrderNo()
	var reply *pb.Order
	err := dtm.Branch(ctx, s.db, func(tx *sql.Tx) error {
		existing, err := findOrder(ctx, tx, orderNo)
		if err != nil {
			return err
		}
		if existing != nil {
			reply, err = s.replyFor(ctx, tx, existing)
			return err
		}
		record := recordFromInner(ctx, req)
		if err := insertOrder(ctx, tx, record, time.Now()); err != nil {
			if isDuplicateKey(err) {
				return errDuplicateOrder
			}
			return err
		}
		if err := insertOrderItems(ctx, tx, orderNo, req.GetItems()); err != nil {
			return err
		}
		if err := insertStatusLog(ctx, tx, orderNo, "", statusPendingPayment, "order created", "system"); err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic:         topicOrderCreated,
			AggregateType: "order",
			AggregateID:   orderNo,
			EventType:     "order.created",
			Payload: map[string]any{
				"order_no":   orderNo,
				"user_id":    req.GetUserId(),
				"quote_id":   req.GetQuoteId(),
				"pay_cent":   record.PayCent,
				"status":     statusPendingPayment,
				"expire_at":  record.ExpireAt.Unix(),
				"items":      orderItemsPayload(req.GetItems()),
				"created_at": time.Now().Unix(),
			},
		})
	})
	if err != nil {
		if errors.Is(err, errDuplicateOrder) {
			existing, ferr := findOrder(ctx, s.db, orderNo)
			if ferr != nil {
				return nil, apperr.Internal("load duplicate order", ferr)
			}
			if existing != nil {
				return s.replyFor(ctx, s.db, existing)
			}
			return nil, apperr.Conflict("order_no already exists")
		}
		return nil, branchError("create order branch", err)
	}
	if reply != nil {
		return reply, nil
	}
	created, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load created order", err)
	}
	if created == nil {
		return nil, apperr.Internal("order not found after branch commit", nil)
	}
	return s.replyFor(ctx, s.db, created)
}

// ---------------------------------------------------------------- read

func (s *Service) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.Order, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order", err)
	}
	if record == nil {
		return nil, apperr.NotFound("order not found")
	}
	if !req.GetAdmin() && (req.GetUserId() <= 0 || req.GetUserId() != record.UserID) {
		return nil, apperr.Forbidden("order belongs to another user")
	}
	return s.replyFor(ctx, s.db, record)
}

func (s *Service) ListOrders(ctx context.Context, req *pb.ListOrdersRequest) (*pb.OrderList, error) {
	page := req.GetPage()
	if page <= 0 {
		page = 1
	}
	pageSize := req.GetPageSize()
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	statusFilter := strings.TrimSpace(req.GetStatus())
	if statusFilter != "" {
		if _, ok := knownStatuses[statusFilter]; !ok {
			return nil, apperr.Invalid("unknown status: %s", statusFilter)
		}
	}
	where := "WHERE 1 = 1"
	args := make([]any, 0, 3)
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
	if statusFilter != "" {
		where += " AND status = ?"
		args = append(args, statusFilter)
	}
	total, err := countOrders(ctx, s.db, where, args)
	if err != nil {
		return nil, apperr.Internal("count orders", err)
	}
	records, err := listOrders(ctx, s.db, where, args, "id DESC", pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, apperr.Internal("list orders", err)
	}
	orderNos := make([]string, 0, len(records))
	for _, record := range records {
		orderNos = append(orderNos, record.OrderNo)
	}
	grouped, err := loadOrderItemsByOrder(ctx, s.db, orderNos)
	if err != nil {
		return nil, apperr.Internal("list order items", err)
	}
	reply := &pb.OrderList{Total: total, Orders: make([]*pb.Order, 0, len(records))}
	for _, record := range records {
		reply.Orders = append(reply.Orders, record.toProto(grouped[record.OrderNo]))
	}
	return reply, nil
}

// ---------------------------------------------------------------- cancel

// CancelOrder is the user/admin facing cancellation. It does not run a DTM
// saga; reservations are released best-effort after the local state change.
func (s *Service) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.Order, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order", err)
	}
	if record == nil {
		return nil, apperr.NotFound("order not found")
	}
	if !req.GetAdmin() && (req.GetUserId() <= 0 || req.GetUserId() != record.UserID) {
		return nil, apperr.Forbidden("order belongs to another user")
	}
	switch record.Status {
	case statusCancelled:
		return s.replyFor(ctx, s.db, record)
	case statusPendingPayment:
	case statusPaid, statusShipped, statusCompleted, statusClosed:
		return nil, apperr.Failed(fmt.Sprintf("order cannot be cancelled in status %s", record.Status))
	default:
		return nil, apperr.Failed(fmt.Sprintf("order cannot be cancelled in status %s", record.Status))
	}
	reason := strings.TrimSpace(req.GetReason())
	if reason == "" {
		reason = "cancelled by request"
	}
	operator := cancelOperator(req)
	items, err := loadOrderItems(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order items", err)
	}
	cancelled := false
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		affected, err := updateOrderStatus(ctx, tx, orderNo, statusPendingPayment, statusCancelled, record.PaidCent)
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		cancelled = true
		if err := insertStatusLog(ctx, tx, orderNo, statusPendingPayment, statusCancelled, reason, operator); err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic:         topicOrderCancelled,
			AggregateType: "order",
			AggregateID:   orderNo,
			EventType:     "order.cancelled",
			Payload: map[string]any{
				"order_no":     orderNo,
				"user_id":      record.UserID,
				"quote_id":     record.QuoteID,
				"reason":       reason,
				"operator":     operator,
				"status":       statusCancelled,
				"cancelled_at": time.Now().Unix(),
				"items":        orderItemsPayload(items),
			},
		})
	})
	if err != nil {
		return nil, apperr.Internal("cancel order", err)
	}
	updated, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("reload order", err)
	}
	if updated == nil {
		return nil, apperr.NotFound("order not found")
	}
	if !cancelled && updated.Status != statusCancelled {
		return nil, apperr.Failed(fmt.Sprintf("order cannot be cancelled in status %s", updated.Status))
	}
	// Inventory release and coupon unlock are driven by the durable
	// shop.order.cancelled.v1 event, not by best-effort calls after commit.
	reply, err := s.replyFor(ctx, s.db, updated)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// CancelOrderInner is the DTM compensation of CreateOrderInner.
func (s *Service) CancelOrderInner(ctx context.Context, req *pb.CancelOrderInnerRequest) (*pb.Empty, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	if !dtm.InBranch(ctx) {
		return nil, apperr.Unavailable("CancelOrderInner is a DTM branch and must be invoked by DTM", nil)
	}
	reason := strings.TrimSpace(req.GetReason())
	if reason == "" {
		reason = "create order rolled back"
	}
	err := dtm.Branch(ctx, s.db, func(tx *sql.Tx) error {
		record, err := findOrder(ctx, tx, orderNo)
		if err != nil {
			return err
		}
		if record == nil || record.Status == statusCancelled {
			// Nothing to compensate; the branch barrier already deduplicates
			// repeated compensations.
			return nil
		}
		if record.Status != statusPendingPayment {
			return apperr.Failed(fmt.Sprintf("order cannot be cancelled in status %s", record.Status))
		}
		affected, err := updateOrderStatus(ctx, tx, orderNo, statusPendingPayment, statusCancelled, record.PaidCent)
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		if err := insertStatusLog(ctx, tx, orderNo, statusPendingPayment, statusCancelled, reason, "system"); err != nil {
			return err
		}
		items, err := loadOrderItems(ctx, tx, orderNo)
		if err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic:         topicOrderCancelled,
			AggregateType: "order",
			AggregateID:   orderNo,
			EventType:     "order.cancelled",
			Payload: map[string]any{
				"order_no":     orderNo,
				"user_id":      record.UserID,
				"quote_id":     record.QuoteID,
				"reason":       reason,
				"status":       statusCancelled,
				"cancelled_at": time.Now().Unix(),
				"items":        orderItemsPayload(items),
			},
		})
	})
	if err != nil {
		return nil, branchError("cancel order branch", err)
	}
	return &pb.Empty{}, nil
}

// ---------------------------------------------------------------- payment

// MarkPaid is the direct fallback used when PAYMENT_TCC_ENABLED=false. The
// default payment path is the TCC in tcc.go; this method remains idempotent for
// rollback and old callers.
func (s *Service) MarkPaid(ctx context.Context, req *pb.MarkPaidRequest) (*pb.Order, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order", err)
	}
	if record == nil {
		return nil, apperr.NotFound("order not found")
	}
	switch record.Status {
	case statusPaid:
		s.afterPaid(ctx, record)
		return s.replyFor(ctx, s.db, record)
	case statusPendingPayment:
	default:
		return nil, apperr.Failed(fmt.Sprintf("order cannot be paid in status %s", record.Status))
	}
	// Confirm stock before marking the order paid. A paid order must never
	// leave the reservation in HELD state where the expiry sweeper could
	// release it and oversell the SKU. Confirm is idempotent, so a payment
	// callback retry is safe.
	if s.inventory != nil {
		items, err := loadOrderItems(ctx, s.db, orderNo)
		if err != nil {
			return nil, apperr.Internal("load order items before payment", err)
		}
		for _, item := range items {
			if _, err := s.inventory.Confirm(ctx, &pb.ReserveRequest{
				OrderNo:     orderNo,
				SkuId:       item.GetSkuId(),
				WarehouseId: defaultWarehouseID,
				Quantity:    item.GetQuantity(),
			}); err != nil {
				return nil, apperr.Unavailable("confirm inventory before payment: "+err.Error(), err)
			}
		}
	}
	paidCent := req.GetPaidCent()
	if paidCent <= 0 {
		paidCent = record.PayCent
	}
	if paidCent > record.PayCent {
		return nil, apperr.Invalid("paid_cent exceeds pay_cent")
	}
	paymentNo := strings.TrimSpace(req.GetPaymentNo())
	operator := "payment"
	if paymentNo != "" {
		operator = truncate("payment:"+paymentNo, 64)
	}
	paidItems, err := loadOrderItems(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order items for paid event", err)
	}
	paid := false
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		affected, err := updateOrderStatus(ctx, tx, orderNo, statusPendingPayment, statusPaid, paidCent)
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		paid = true
		if err := insertStatusLog(ctx, tx, orderNo, statusPendingPayment, statusPaid, "payment "+paymentNo, operator); err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic:         topicOrderPaid,
			AggregateType: "order",
			AggregateID:   orderNo,
			EventType:     "order.paid",
			Payload: map[string]any{
				"order_no":   orderNo,
				"user_id":    record.UserID,
				"payment_no": paymentNo,
				"paid_cent":  paidCent,
				"pay_cent":   record.PayCent,
				"status":     statusPaid,
				"paid_at":    time.Now().Unix(),
				"items":      orderItemsPayload(paidItems),
			},
		})
	})
	if err != nil {
		return nil, apperr.Internal("mark order paid", err)
	}
	updated, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("reload order", err)
	}
	if updated == nil {
		return nil, apperr.NotFound("order not found")
	}
	if updated.Status != statusPaid {
		return nil, apperr.Failed(fmt.Sprintf("order cannot be paid in status %s", updated.Status))
	}
	if paid {
		s.afterPaid(ctx, updated)
	}
	return s.replyFor(ctx, s.db, updated)
}

// afterPaid confirms the inventory reservations and consumes the quote coupon.
// Both calls are idempotent on the remote side; failures are logged because the
// order is already paid and must not be rolled back.
func (s *Service) afterPaid(ctx context.Context, record *orderRecord) {
	items, err := loadOrderItems(ctx, s.db, record.OrderNo)
	if err != nil {
		s.log.Error("load order items after payment", "order_no", record.OrderNo, "error", err)
		return
	}
	if s.inventory != nil {
		for _, item := range items {
			req := &pb.ReserveRequest{
				OrderNo:     record.OrderNo,
				SkuId:       item.GetSkuId(),
				WarehouseId: defaultWarehouseID,
				Quantity:    item.GetQuantity(),
			}
			if _, err := s.inventory.Confirm(ctx, req); err != nil {
				s.log.Warn("confirm inventory", "order_no", record.OrderNo, "sku_id", item.GetSkuId(), "error", err)
			}
		}
	}
	if s.pricing != nil && record.QuoteID != "" {
		couponID := s.couponForQuote(ctx, record.QuoteID)
		_, err := s.pricing.ConsumeCoupon(ctx, &pb.LockCouponRequest{
			QuoteId:  record.QuoteID,
			CouponId: couponID,
			UserId:   record.UserID,
		})
		if err != nil {
			s.log.Warn("consume coupon", "order_no", record.OrderNo, "quote_id", record.QuoteID, "error", err)
		}
	}
}

// couponForQuote resolves the coupon attached to a quote. The order table does
// not duplicate pricing state, so the coupon is looked up best-effort.
func (s *Service) couponForQuote(ctx context.Context, quoteID string) string {
	if s.pricing == nil || quoteID == "" {
		return ""
	}
	quote, err := s.pricing.GetQuote(ctx, &pb.GetQuoteRequest{QuoteId: quoteID})
	if err != nil {
		s.log.Warn("get quote for coupon", "quote_id", quoteID, "error", err)
		return ""
	}
	return quote.GetCouponId()
}

// ---------------------------------------------------------------- helpers

func (s *Service) getQuote(ctx context.Context, quoteID string) (*pb.Quote, error) {
	quote, err := s.pricing.GetQuote(ctx, &pb.GetQuoteRequest{QuoteId: quoteID})
	if err != nil {
		return nil, remoteError("get quote", err)
	}
	if quote == nil || quote.GetQuoteId() == "" {
		return nil, apperr.Internal("pricing returned an empty quote", nil)
	}
	if len(quote.GetLines()) == 0 {
		return nil, apperr.Failed("quote has no lines")
	}
	for _, line := range quote.GetLines() {
		if line.GetSkuId() <= 0 {
			return nil, apperr.Internal("pricing returned a quote line without sku_id", nil)
		}
		if line.GetQuantity() <= 0 {
			return nil, apperr.Internal("pricing returned a quote line with a non positive quantity", nil)
		}
		if line.GetPriceCent() < 0 || line.GetDiscountCent() < 0 {
			return nil, apperr.Internal("pricing returned negative quote amounts", nil)
		}
	}
	return quote, nil
}

// replyFor loads the order items and converts a record to its protobuf form.
func (s *Service) replyFor(ctx context.Context, q querier, record *orderRecord) (*pb.Order, error) {
	items, err := loadOrderItems(ctx, q, record.OrderNo)
	if err != nil {
		return nil, apperr.Internal("load order items", err)
	}
	return record.toProto(items), nil
}

// createOrderInput is the validated CreateOrder request.
type createOrderInput struct {
	UserID          int64
	QuoteID         string
	AddressID       int64
	ClientRequestID string
	ReceiverName    string
	ReceiverPhone   string
	ReceiverAddress string
}

func normalizeCreateOrder(req *pb.CreateOrderRequest) (createOrderInput, error) {
	in := createOrderInput{
		UserID:          req.GetUserId(),
		QuoteID:         strings.TrimSpace(req.GetQuoteId()),
		AddressID:       req.GetAddressId(),
		ClientRequestID: strings.TrimSpace(req.GetClientRequestId()),
		ReceiverName:    strings.TrimSpace(req.GetReceiverName()),
		ReceiverPhone:   strings.TrimSpace(req.GetReceiverPhone()),
		ReceiverAddress: strings.TrimSpace(req.GetReceiverAddress()),
	}
	if in.UserID <= 0 {
		return in, apperr.Invalid("user_id is required")
	}
	if in.QuoteID == "" {
		return in, apperr.Invalid("quote_id is required")
	}
	if in.ReceiverName == "" {
		return in, apperr.Invalid("receiver_name is required")
	}
	if in.ReceiverPhone == "" {
		return in, apperr.Invalid("receiver_phone is required")
	}
	if in.ReceiverAddress == "" {
		return in, apperr.Invalid("receiver_address is required")
	}
	if len(in.ReceiverName) > 64 || len(in.ReceiverPhone) > 32 || len(in.ReceiverAddress) > 255 {
		return in, apperr.Invalid("receiver fields exceed the maximum length")
	}
	if in.ClientRequestID == "" {
		// Without a client key the unique (user_id, client_request_id) index
		// would collapse every request of the user; generate one instead.
		in.ClientRequestID = "auto-" + id.Event()
	}
	if len(in.ClientRequestID) > maxClientRequestIDBytes {
		return in, apperr.Invalid("client_request_id must be at most 64 characters")
	}
	if !printableASCII(in.ClientRequestID) {
		// The key travels as a DTM branch header (gRPC metadata), which only
		// accepts printable ASCII.
		return in, apperr.Invalid("client_request_id must be printable ASCII")
	}
	return in, nil
}

// printableASCII reports whether every byte of s is printable ASCII, the only
// range accepted for gRPC metadata values.
func printableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

func validateCreateOrderInner(req *pb.CreateOrderInnerRequest) error {
	if strings.TrimSpace(req.GetOrderNo()) == "" {
		return apperr.Invalid("order_no is required")
	}
	if len(req.GetOrderNo()) > maxOrderNoBytes {
		return apperr.Invalid("order_no is too long")
	}
	if req.GetUserId() <= 0 {
		return apperr.Invalid("user_id is required")
	}
	if strings.TrimSpace(req.GetQuoteId()) == "" {
		return apperr.Invalid("quote_id is required")
	}
	if strings.TrimSpace(req.GetReceiverName()) == "" || strings.TrimSpace(req.GetReceiverPhone()) == "" ||
		strings.TrimSpace(req.GetReceiverAddress()) == "" {
		return apperr.Invalid("receiver_name, receiver_phone and receiver_address are required")
	}
	if len(req.GetItems()) == 0 {
		return apperr.Invalid("order must contain at least one item")
	}
	for _, item := range req.GetItems() {
		if item.GetSkuId() <= 0 {
			return apperr.Invalid("order item sku_id is required")
		}
		if item.GetQuantity() <= 0 {
			return apperr.Invalid("order item quantity must be positive")
		}
		if item.GetPriceCent() < 0 || item.GetDiscountCent() < 0 {
			return apperr.Invalid("order item amounts must not be negative")
		}
	}
	if req.GetTotalCent() < 0 || req.GetDiscountCent() < 0 || req.GetFreightCent() < 0 || req.GetPayCent() < 0 {
		return apperr.Invalid("order amounts must not be negative")
	}
	return nil
}

// buildCreateOrderInner assembles the DTM payload for the CreateOrderInner
// branch from the quote and the receiver snapshot.
func buildCreateOrderInner(orderNo string, in createOrderInput, quote *pb.Quote, expireAt time.Time) *pb.CreateOrderInnerRequest {
	total, discount, freight, pay := quoteAmounts(quote)
	items := make([]*pb.OrderItem, 0, len(quote.GetLines()))
	for _, line := range quote.GetLines() {
		items = append(items, &pb.OrderItem{
			SkuId:        line.GetSkuId(),
			Name:         line.GetName(),
			PriceCent:    line.GetPriceCent(),
			Quantity:     line.GetQuantity(),
			DiscountCent: line.GetDiscountCent(),
		})
	}
	return &pb.CreateOrderInnerRequest{
		UserId:          in.UserID,
		OrderNo:         orderNo,
		QuoteId:         in.QuoteID,
		AddressId:       in.AddressID,
		TotalCent:       total,
		DiscountCent:    discount,
		FreightCent:     freight,
		PayCent:         pay,
		ReceiverName:    in.ReceiverName,
		ReceiverPhone:   in.ReceiverPhone,
		ReceiverAddress: in.ReceiverAddress,
		Items:           items,
		ExpireAtUnix:    expireAt.Unix(),
	}
}

func recordFromInner(ctx context.Context, req *pb.CreateOrderInnerRequest) *orderRecord {
	expireAt := time.Now().Add(orderExpireWindow)
	if unix := req.GetExpireAtUnix(); unix > 0 {
		expireAt = time.Unix(unix, 0)
	}
	return &orderRecord{
		OrderNo:         req.GetOrderNo(),
		UserID:          req.GetUserId(),
		Status:          statusPendingPayment,
		TotalCent:       req.GetTotalCent(),
		DiscountCent:    req.GetDiscountCent(),
		FreightCent:     req.GetFreightCent(),
		PayCent:         req.GetPayCent(),
		PaidCent:        0,
		ReceiverName:    req.GetReceiverName(),
		ReceiverPhone:   req.GetReceiverPhone(),
		ReceiverAddress: req.GetReceiverAddress(),
		QuoteID:         strings.TrimSpace(req.GetQuoteId()),
		ClientRequestID: clientRequestIDFromContext(ctx, req.GetOrderNo()),
		ExpireAt:        expireAt,
	}
}

// clientRequestIDFromContext reads the idempotency key forwarded by DTM through
// the saga branch headers. The fallback keeps the (user_id, client_request_id)
// index usable when a branch is invoked outside of CreateOrder.
func clientRequestIDFromContext(ctx context.Context, fallback string) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(clientRequestIDHeader); len(values) > 0 {
			if v := strings.TrimSpace(values[0]); v != "" {
				return truncate(v, maxClientRequestIDBytes)
			}
		}
	}
	return truncate(fallback, maxClientRequestIDBytes)
}

// quoteAmounts returns the order amounts derived from a quote. The total falls
// back to the sum of the lines when pricing did not fill it in.
func quoteAmounts(quote *pb.Quote) (total, discount, freight, pay int64) {
	total = quote.GetTotalCent()
	if total <= 0 {
		for _, line := range quote.GetLines() {
			total += line.GetPriceCent() * int64(line.GetQuantity())
		}
	}
	discount = quote.GetDiscountCent()
	if discount < 0 {
		discount = 0
	}
	if discount > total {
		discount = total
	}
	freight = quote.GetFreightCent()
	if freight < 0 {
		freight = 0
	}
	pay = quote.GetPayCent()
	if pay <= 0 {
		pay = orderPayCent(total, discount, freight)
	}
	return total, discount, freight, pay
}

// orderPayCent is the amount payable: merchandise total minus discount plus
// freight, never negative.
func orderPayCent(totalCent, discountCent, freightCent int64) int64 {
	pay := totalCent - discountCent + freightCent
	if pay < 0 {
		return 0
	}
	return pay
}

func quoteExpireAt(quote *pb.Quote) time.Time {
	now := time.Now()
	if unix := quote.GetExpiresAtUnix(); unix > 0 {
		if at := time.Unix(unix, 0); at.After(now) {
			return at
		}
	}
	return now.Add(orderExpireWindow)
}

func createOrderGID(orderNo string) string { return "order-create:" + orderNo }

func cancelOperator(req *pb.CancelOrderRequest) string {
	if req.GetAdmin() {
		return "admin"
	}
	return truncate("user:"+strconv.FormatInt(req.GetUserId(), 10), 64)
}

// remoteError maps a downstream gRPC error onto the domain error model.
func remoteError(action string, err error) error {
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.NotFound:
			return apperr.NotFound(action + ": " + st.Message())
		case codes.InvalidArgument:
			return apperr.Invalid("%s: %s", action, st.Message())
		case codes.FailedPrecondition, codes.Aborted:
			return apperr.Failed(action + ": " + st.Message())
		}
	}
	return apperr.Unavailable(action, err)
}

// branchError keeps domain errors intact and wraps everything else.
func branchError(action string, err error) error {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae
	}
	return apperr.Internal(action, err)
}

func orderItemsPayload(items []*pb.OrderItem) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"sku_id":        item.GetSkuId(),
			"name":          item.GetName(),
			"price_cent":    item.GetPriceCent(),
			"quantity":      item.GetQuantity(),
			"discount_cent": item.GetDiscountCent(),
		})
	}
	return out
}

// truncate cuts s to at most max bytes without splitting a UTF-8 rune (MySQL
// rejects invalid UTF-8 in VARCHAR columns).
func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max]
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
