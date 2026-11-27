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

	db          *sql.DB
	mq          *mq.Client
	log         *slog.Logger
	dtm         *dtm.Client
	pricing     pricingPort
	inventory   inventoryPort
	payments    paymentPort
	fulfillment fulfillmentPort
	promotion   pb.PromotionServiceClient

	inventoryDTM          string
	orderDTM              string
	requireInventory      bool
	warehouseSplitEnabled bool
	shortageCaseEnabled   bool
	now                   func() time.Time
}

// EnableSettlementClients wires the two clients required by refund admission.
// They are separate from the constructor so existing tests can omit them.
func (s *Service) EnableSettlementClients(payments paymentPort, fulfillment fulfillmentPort) {
	s.payments = payments
	s.fulfillment = fulfillment
}

// New builds the order service. pricing and inventory may be nil in tests that
// only exercise local logic.
type Options struct {
	// WarehouseSplitEnabled allows multi-group quotes and per-warehouse
	// reservations. It stays false until split shipments/permits are ready.
	WarehouseSplitEnabled bool
	// ShortageCaseEnabled runs the 48h fail-safe shortage worker.
	ShortageCaseEnabled bool
	// RequireInventory makes nil inventory a hard configuration error instead
	// of a unit-test convenience. cmd/order sets it to true.
	RequireInventory bool
}

func New(database *sql.DB, client *mq.Client, logger *slog.Logger, dtmClient *dtm.Client,
	pricing pricingPort, inventory inventoryPort, promotion pb.PromotionServiceClient) *Service {
	return NewWithOptions(database, client, logger, dtmClient, pricing, inventory, promotion, Options{})
}

func NewWithOptions(database *sql.DB, client *mq.Client, logger *slog.Logger, dtmClient *dtm.Client,
	pricing pricingPort, inventory inventoryPort, promotion pb.PromotionServiceClient, opts Options) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		warehouseSplitEnabled: opts.WarehouseSplitEnabled,
		shortageCaseEnabled:   opts.ShortageCaseEnabled,
		requireInventory:      opts.RequireInventory,
		db:                    database,
		mq:                    client,
		log:                   logger,
		dtm:                   dtmClient,
		pricing:               pricing,
		inventory:             inventory,
		promotion:             promotion,
		inventoryDTM:          strings.TrimRight(env("INVENTORY_DTM_URL", defaultInventoryDTMURL), "/"),
		orderDTM:              strings.TrimRight(env("ORDER_DTM_URL", defaultOrderDTMURL), "/"),
		now:                   time.Now,
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
	allocation, err := s.revalidateAllocation(ctx, quote)
	if err != nil {
		return nil, err
	}
	if err := s.recordAllocationDecision(ctx, orderNo, quote, allocation); err != nil {
		return nil, err
	}
	couponLocked := false
	if quote.GetCouponId() != "" {
		if _, err := s.pricing.LockCoupon(ctx, &pb.LockCouponRequest{QuoteId: quote.GetQuoteId(), CouponId: quote.GetCouponId(), UserId: in.UserID}); err != nil {
			return nil, remoteError("lock quote coupon", err)
		}
		couponLocked = true
	}
	promotionReserved := false
	if s.promotion != nil {
		if _, err := s.promotion.ReservePromotions(ctx, &pb.PromotionReserveRequest{
			QuoteId: quote.GetQuoteId(), OrderNo: orderNo, UserId: in.UserID,
		}); err != nil {
			if couponLocked {
				_, _ = s.pricing.UnlockCoupon(ctx, &pb.LockCouponRequest{QuoteId: quote.GetQuoteId(), CouponId: quote.GetCouponId(), UserId: in.UserID})
			}
			return nil, err
		}
		promotionReserved = true
	}
	inner := buildCreateOrderInner(orderNo, in, quote, quoteExpireAt(quote))
	if err := s.submitCreateSaga(ctx, orderNo, in.ClientRequestID, quote.GetLines(), inner); err != nil {
		if promotionReserved {
			_, _ = s.promotion.ReleasePromotions(ctx, &pb.PromotionOrderRequest{OrderNo: orderNo})
		}
		if couponLocked {
			_, _ = s.pricing.UnlockCoupon(ctx, &pb.LockCouponRequest{QuoteId: quote.GetQuoteId(), CouponId: quote.GetCouponId(), UserId: in.UserID})
		}
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
func (s *Service) submitCreateSaga(ctx context.Context, orderNo, clientRequestID string, lines []*pb.QuoteLine, inner *pb.CreateOrderInnerRequest) error {
	if s.dtm == nil {
		return apperr.Unavailable("dtm client is not configured", nil)
	}
	saga := s.dtm.Saga(ctx, createOrderGID(orderNo))
	// Branch headers are forwarded by DTM to every branch; they carry the
	// idempotency key that CreateOrderInnerRequest does not expose. Merge instead
	// of replacing: Saga() already put traceparent there.
	if clientRequestID != "" {
		if saga.BranchHeaders == nil {
			saga.BranchHeaders = map[string]string{}
		}
		saga.BranchHeaders[clientRequestIDHeader] = clientRequestID
	}
	for _, line := range mergeReserveLines(lines) {
		reserve := &pb.ReserveRequest{
			OrderNo:     orderNo,
			SkuId:       line.GetSkuId(),
			WarehouseId: line.GetWarehouseId(),
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
		if err := insertFulfillmentGroups(ctx, tx, orderNo, req.GetFulfillmentGroups(), record.FreightCent); err != nil {
			return err
		}
		if err := insertAllocationSnapshotTx(ctx, tx, req); err != nil {
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
		s.log.Error("create order branch failed", "order_no", orderNo, "error", err)
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
		reply.Orders = append(reply.Orders, record.toProto(grouped[record.OrderNo], nil))
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
	if record.RefundState == refundStateRefunding || record.RefundState == refundStateFull {
		return nil, apperr.Failed("order refund is in progress")
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
		if err := setFulfillmentGroupStatus(ctx, tx, orderNo, statusCancelled); err != nil {
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
	// Promotion release is also idempotent here for latency-sensitive callers.
	if s.promotion != nil && cancelled {
		if _, err := s.promotion.ReleasePromotions(ctx, &pb.PromotionOrderRequest{OrderNo: orderNo}); err != nil {
			s.log.Warn("release promotions on cancel", "order_no", orderNo, "error", err)
		}
	}
	reply, err := s.replyFor(ctx, s.db, updated)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// CancelOrderInner is the DTM compensation of CreateOrderInner.
func (s *Service) CancelOrderInner(ctx context.Context, req *pb.CancelOrderInnerRequest) (*pb.Empty, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	// DTM saga compensation reuses the action payload (CreateOrderInnerRequest).
	// Decoding that as CancelOrderInnerRequest puts the real order number in the
	// reason field. Tolerate both shapes so a failed create can be compensated.
	if !strings.HasPrefix(orderNo, "ORD") && strings.HasPrefix(strings.TrimSpace(req.GetReason()), "ORD") {
		orderNo = strings.TrimSpace(req.GetReason())
	}
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	if !dtm.InBranch(ctx) {
		return nil, apperr.Unavailable("CancelOrderInner is a DTM branch and must be invoked by DTM", nil)
	}
	reason := strings.TrimSpace(req.GetReason())
	if strings.HasPrefix(reason, "ORD") {
		reason = "create order rolled back"
	}
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
		if err := setFulfillmentGroupStatus(ctx, tx, orderNo, statusCancelled); err != nil {
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
	if record.RefundState == refundStateRefunding || record.RefundState == refundStateFull {
		return nil, apperr.Failed("order refund is in progress")
	}
	paymentNo := strings.TrimSpace(req.GetPaymentNo())
	// T2.1 producer: a direct PAID write always carries the effective payment
	// identity. Reject before confirming inventory so a caller cannot leave a
	// PAID row with settlement_payment_no='' (historical-order bug).
	if paymentNo == "" {
		return nil, apperr.Invalid("payment_no is required to mark an order PAID").
			WithDetails(map[string]any{"order_no": orderNo, "reason": "PAID_WITHOUT_PAYMENT_NO"})
	}
	switch record.Status {
	case statusPaid:
		if paymentNo == "" || record.SettlementPaymentNo == paymentNo {
			return s.replyFor(ctx, s.db, record)
		}
		return nil, apperr.Conflict("order was already settled by another payment")
	case statusPendingPayment:
	default:
		return nil, apperr.Failed(fmt.Sprintf("order cannot be paid in status %s", record.Status))
	}
	// Validate the payment before touching inventory: confirming first and
	// rejecting the amount afterwards leaves stock consumed for an order that
	// stays PENDING (and is later cancelled by the expiry sweeper).
	paidCent, err := paidAmount(record, req.GetPaidCent())
	if err != nil {
		return nil, err
	}
	// Confirm stock before marking the order paid. A paid order must never
	// leave the reservation in HELD state where the expiry sweeper could
	// release it and oversell the SKU. Confirm is idempotent, so a payment
	// callback retry is safe.
	items, err := loadOrderItems(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order items before payment", err)
	}
	if s.inventory != nil {
		confirmed := make([]*pb.OrderItem, 0, len(items))
		for _, item := range items {
			if _, err := s.inventory.Confirm(ctx, &pb.ReserveRequest{
				OrderNo:     orderNo,
				SkuId:       item.GetSkuId(),
				WarehouseId: item.GetWarehouseId(),
				Quantity:    item.GetQuantity(),
			}); err != nil {
				// Include the current SKU: Confirm may have committed remotely
				// even though the RPC returned an error. TccCancelConfirm is
				// idempotent and handles HELD, CONFIRMED and seckill.
				attempted := append(append([]*pb.OrderItem{}, confirmed...), item)
				if _, cerr := s.inventory.TccCancelConfirm(ctx, &pb.TccPaymentRequest{
					OrderNo: orderNo, Items: attempted,
				}); cerr != nil {
					s.log.Error("compensate partially confirmed inventory", "order_no", orderNo, "error", cerr)
					if qerr := s.queueInventoryCompensation(ctx, orderNo, attempted); qerr != nil {
						return nil, apperr.Internal("confirm inventory before payment and persist its compensation",
							errors.Join(err, cerr, qerr))
					}
				}
				return nil, apperr.Unavailable("confirm inventory before payment: "+err.Error(), err)
			}
			confirmed = append(confirmed, item)
		}
	}
	if err := s.checkOrderReady(ctx, orderNo, items); err != nil {
		return nil, err
	}
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
		if err := ensureAllocationSnapshotTx(ctx, tx, record); err != nil {
			return err
		}
		affected, err := freezeAllocationTx(ctx, tx, orderNo, paidCent, paymentNo)
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
		if err := setFulfillmentGroupStatus(ctx, tx, orderNo, statusAllocated); err != nil {
			return err
		}
		if err := appendAllocationFrozenEvent(ctx, tx, orderNo, record.AllocationHash, record.PackageCount, record.WarehouseCount); err != nil {
			return err
		}
		if err := appendOrderSettledEvent(ctx, tx, record, paymentNo); err != nil {
			return err
		}
		if err := ensurePricingConfirmationTask(ctx, tx, record); err != nil {
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
		if updated.Status == statusCancelled {
			// Stock was already confirmed for an order that a concurrent cancel
			// closed. Return the captured money instead of leaving the caller
			// with a silently consumed reservation. A command persistence
			// failure is an error, not a log-only event: the capture must not be
			// reported as settled while no durable refund exists.
			if rerr := s.refundCancelledAfterConfirm(ctx, updated, paymentNo); rerr != nil {
				s.log.Error("emit payment-level refund for cancelled order", "order_no", orderNo, "error", rerr)
				return nil, apperr.Internal("order cancelled after confirm and the payment refund command was not persisted", rerr)
			}
		}
		return nil, apperr.Conflict(fmt.Sprintf("order cannot be paid in status %s", updated.Status))
	}
	if paid {
		s.afterPaid(ctx, updated)
	}
	return s.replyFor(ctx, s.db, updated)
}

// queueInventoryCompensation persists a durable Inventory retry. The caller
// must treat a returned error as a real failure: compensating stock must never
// be dropped just because the outbox append failed (T1.6-R B3).
func (s *Service) queueInventoryCompensation(ctx context.Context, orderNo string, items []*pb.OrderItem) error {
	if orderNo == "" || len(items) == 0 {
		return apperr.Invalid("inventory compensation requires order_no and items")
	}
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item.GetWarehouseId() <= 0 {
			return apperr.Failed("inventory compensation item is missing warehouse_id").
				WithDetails(map[string]any{
					"order_no": orderNo, "sku_id": item.GetSkuId(),
					"reason": "INVENTORY_COMPENSATION_NO_WAREHOUSE",
				})
		}
		payload = append(payload, map[string]any{
			"sku_id": item.GetSkuId(), "warehouse_id": item.GetWarehouseId(),
			"quantity": item.GetQuantity(),
		})
	}
	if err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		return outbox.Append(ctx, tx, outbox.Event{
			Topic: "shop.inventory.compensation.requested.v1", AggregateType: "order",
			AggregateID: orderNo, EventType: "inventory.compensation_requested",
			Payload: map[string]any{"order_no": orderNo, "items": payload, "created_at": time.Now().Unix()},
		})
	}); err != nil {
		return apperr.Internal("queue inventory compensation", err)
	}
	return nil
}

// queueInventoryConfirm emits a durable retry when an already-paid order still
// has a HELD reservation. Inventory consumes it with TccConfirm, which is
// idempotent; unlike compensation it must never release the reservation.
func (s *Service) queueInventoryConfirm(ctx context.Context, orderNo string, items []*pb.OrderItem) {
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload = append(payload, map[string]any{"sku_id": item.GetSkuId(), "quantity": item.GetQuantity()})
	}
	if err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		return outbox.Append(ctx, tx, outbox.Event{
			Topic: "shop.inventory.confirm.requested.v1", AggregateType: "order",
			AggregateID: orderNo, EventType: "inventory.confirm_requested",
			Payload: map[string]any{"order_no": orderNo, "items": payload, "created_at": time.Now().Unix()},
		})
	}); err != nil {
		s.log.Error("queue inventory confirm", "order_no", orderNo, "error", err)
	}
}

// refundCancelledAfterConfirm compensates a capture whose order was cancelled
// between the inventory confirm and the status CAS. The authoritative captured
// amount is read from Payment (never guessed from the order), the reservation is
// restocked, and a durable payment-level cancel command is written. Repeat calls
// are safe: TccCancelConfirm and the fixed `pay:<payment_no>:cancel` identity are
// both idempotent.
func (s *Service) refundCancelledAfterConfirm(ctx context.Context, record *orderRecord, paymentNo string) error {
	if record == nil {
		return apperr.Internal("cancel-after-confirm refund requires an order", nil)
	}
	paymentNo = strings.TrimSpace(paymentNo)
	if paymentNo == "" {
		s.log.Error("payment captured for cancelled order without payment_no; no refund command written",
			"order_no", record.OrderNo, "reason", "PAYMENT_CANCEL_NO_PAYMENT_NO")
		return nil
	}
	snapshot, err := s.readTccPaymentSnapshot(ctx, record, paymentNo)
	if err != nil {
		// Fail closed: an unavailable authoritative capture must not become a
		// guessed order-amount refund.
		return apperr.Unavailable("load authoritative captured amount for cancelled order", err)
	}
	return s.refundCancelledWithSnapshot(ctx, record, snapshot, paymentNo, paymentCancelReasonAfterConfirm)
}

// afterPaid confirms the inventory reservations and consumes the quote coupon.
// Both calls are idempotent on the remote side; failures are logged because the
// order is already paid and must not be rolled back.
// checkOrderReady is the ADR-0007 readiness gate. It is a synchronous,
// fail-closed check: if Inventory is configured and not ready, settlement must
// not proceed. A nil client is only possible in unit tests that never wire
// Inventory; production cmd/order always wires it.
func (s *Service) checkOrderReady(ctx context.Context, orderNo string, items []*pb.OrderItem) error {
	if s.inventory == nil {
		if s.requireInventory {
			return apperr.Unavailable("inventory client is required for settlement", nil)
		}
		return nil
	}
	req := &pb.CheckOrderReadyRequest{OrderNo: orderNo}
	for _, item := range items {
		req.Lines = append(req.Lines, &pb.CheckOrderReadyLineRef{
			LineNo: item.GetLineNo(), SkuId: item.GetSkuId(),
			WarehouseId: item.GetWarehouseId(), Quantity: item.GetQuantity(),
		})
	}
	checkCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	reply, err := s.inventory.CheckOrderReady(checkCtx, req)
	if err != nil {
		return apperr.Unavailable("inventory readiness check failed", err)
	}
	if len(reply.GetLines()) == 0 {
		if !reply.GetReady() {
			return apperr.Failed("inventory is not ready for settlement")
		}
		return nil
	}
	// AC-60: historical orders may start from HELD reservations. Confirm HELD
	// before requesting the shipping permit; MISSING/other states are a
	// shortage/refund decision, never a silent pass or a permanent block.
	for _, line := range reply.GetLines() {
		switch line.GetStatus() {
		case "CONFIRMED":
			continue
		case "HELD":
			if _, cerr := s.inventory.Confirm(ctx, &pb.ReserveRequest{
				OrderNo: orderNo, SkuId: line.GetSkuId(), WarehouseId: line.GetWarehouseId(),
				Quantity: line.GetQuantity(),
			}); cerr != nil {
				return apperr.Failed("inventory confirmation failed; shortage/refund required").
					WithDetails(map[string]any{"order_no": orderNo, "sku_id": line.GetSkuId(),
						"warehouse_id": line.GetWarehouseId(), "status": line.GetStatus()})
			}
		default:
			return apperr.Failed("inventory cannot be confirmed; shortage/refund required").
				WithDetails(map[string]any{"order_no": orderNo, "sku_id": line.GetSkuId(),
					"warehouse_id": line.GetWarehouseId(), "status": line.GetStatus()})
		}
	}
	return nil
}

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
				WarehouseId: item.GetWarehouseId(),
				Quantity:    item.GetQuantity(),
			}
			if _, err := s.inventory.Confirm(ctx, req); err != nil {
				s.log.Warn("confirm inventory", "order_no", record.OrderNo, "sku_id", item.GetSkuId(), "error", err)
				s.queueInventoryConfirm(ctx, record.OrderNo, items)
				return
			}
		}
	}
	if s.promotion != nil {
		if _, err := s.promotion.ConfirmPromotions(ctx, &pb.PromotionOrderRequest{OrderNo: record.OrderNo}); err != nil {
			s.log.Warn("confirm promotions", "order_no", record.OrderNo, "error", err)
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
	if err := validateQuoteForOrder(quote, s.warehouseSplitEnabled); err != nil {
		return nil, err
	}
	return quote, nil
}

// replyFor loads the order items and converts a record to its protobuf form.
func (s *Service) replyFor(ctx context.Context, q querier, record *orderRecord) (*pb.Order, error) {
	items, err := loadOrderItems(ctx, q, record.OrderNo)
	if err != nil {
		return nil, apperr.Internal("load order items", err)
	}
	groups, err := loadFulfillmentGroups(ctx, q, record.OrderNo)
	if err != nil {
		return nil, apperr.Internal("load fulfillment groups", err)
	}
	return record.toProto(items, groups), nil
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
	groups := make(map[string]int64, len(req.GetFulfillmentGroups()))
	warehouses := make(map[int64]struct{}, len(req.GetFulfillmentGroups()))
	var groupFreight int64
	for _, group := range req.GetFulfillmentGroups() {
		if group.GetGroupNo() == "" || group.GetWarehouseId() <= 0 {
			return apperr.Invalid("fulfillment groups need group_no and warehouse_id")
		}
		if _, exists := groups[group.GetGroupNo()]; exists {
			return apperr.Invalid("duplicate fulfillment group %s", group.GetGroupNo())
		}
		if _, exists := warehouses[group.GetWarehouseId()]; exists {
			return apperr.Invalid("one warehouse cannot have two fulfillment groups")
		}
		groups[group.GetGroupNo()] = group.GetWarehouseId()
		warehouses[group.GetWarehouseId()] = struct{}{}
		groupFreight += group.GetFreightCent()
	}
	if len(groups) == 0 || len(groups) > maxOrderGroups {
		return apperr.Invalid("order must contain between one and three fulfillment groups")
	}
	lineNos := make(map[int32]struct{}, len(req.GetItems()))
	perSKU := make(map[int64]int, len(req.GetItems()))
	var sumPaid int64
	for _, item := range req.GetItems() {
		if item.GetSkuId() <= 0 {
			return apperr.Invalid("order item sku_id is required")
		}
		if item.GetQuantity() <= 0 {
			return apperr.Invalid("order item quantity must be positive")
		}
		if item.GetPriceCent() < 0 || item.GetDiscountCent() < 0 ||
			item.GetLinePaidCent() < 0 || item.GetUnitPaidCent() < 0 {
			return apperr.Invalid("order item amounts must not be negative")
		}
		lineNo := item.GetLineNo()
		if lineNo <= 0 {
			return apperr.Invalid("order item line_no is required")
		}
		if _, exists := lineNos[lineNo]; exists {
			return apperr.Invalid("duplicate order item line_no %d", lineNo)
		}
		lineNos[lineNo] = struct{}{}
		perSKU[item.GetSkuId()]++
		if perSKU[item.GetSkuId()] > maxLinesPerSKU {
			return apperr.Invalid("sku %d has more than %d order lines", item.GetSkuId(), maxLinesPerSKU)
		}
		if _, ok := groups[item.GetGroupNo()]; !ok {
			return apperr.Invalid("order item references unknown group %s", item.GetGroupNo())
		}
		if groups[item.GetGroupNo()] != item.GetWarehouseId() {
			return apperr.Invalid("order item warehouse does not match its group")
		}
		goods := item.GetPriceCent() * int64(item.GetQuantity())
		if item.GetLinePaidCent() != goods-item.GetDiscountCent() {
			return apperr.Invalid("order item line_paid_cent does not match price minus discount")
		}
		if item.GetUnitPaidCent() != item.GetLinePaidCent()/int64(item.GetQuantity()) {
			return apperr.Invalid("order item unit_paid_cent does not match the floor rule")
		}
		sumPaid += item.GetLinePaidCent()
	}
	if sumPaid+req.GetFreightCent() != req.GetPayCent() {
		return apperr.Invalid("order pay_cent does not equal line paid plus freight")
	}
	if groupFreight != req.GetFreightCent() {
		return apperr.Invalid("group freight does not add up to the order freight")
	}
	if req.GetTotalCent() < 0 || req.GetDiscountCent() < 0 || req.GetFreightCent() < 0 || req.GetPayCent() < 0 {
		return apperr.Invalid("order amounts must not be negative")
	}
	if req.GetPackageCount() != 0 && int(req.GetPackageCount()) != len(groups) {
		return apperr.Invalid("package_count does not match fulfillment groups")
	}
	if req.GetWarehouseCount() != 0 && int(req.GetWarehouseCount()) != len(warehouses) {
		return apperr.Invalid("warehouse_count does not match fulfillment groups")
	}
	return nil
}

// buildCreateOrderInner assembles the DTM payload for the CreateOrderInner
// branch from the quote and the receiver snapshot.
func buildCreateOrderInner(orderNo string, in createOrderInput, quote *pb.Quote, expireAt time.Time) *pb.CreateOrderInnerRequest {
	total, discount, freight, pay := quoteAmounts(quote)
	items := make([]*pb.OrderItem, 0, len(quote.GetLines()))
	for _, line := range quote.GetLines() {
		warehouseID := line.GetWarehouseId()
		if warehouseID <= 0 {
			warehouseID = defaultWarehouseID
		}
		groupNo := line.GetGroupNo()
		if groupNo == "" {
			groupNo = defaultGroupNo
		}
		lineNo := line.GetLineNo()
		if lineNo <= 0 {
			lineNo = int32(len(items) + 1)
		}
		items = append(items, &pb.OrderItem{
			SkuId:        line.GetSkuId(),
			Name:         line.GetName(),
			PriceCent:    line.GetPriceCent(),
			Quantity:     line.GetQuantity(),
			DiscountCent: line.GetDiscountCent(),
			WarehouseId:  warehouseID,
			GroupNo:      groupNo,
			LineNo:       lineNo,
			LinePaidCent: line.GetLinePaidCent(),
			UnitPaidCent: line.GetUnitPaidCent(),
		})
	}
	groups := make([]*pb.OrderFulfillmentGroup, 0, len(quote.GetGroups()))
	for _, group := range quote.GetGroups() {
		groups = append(groups, &pb.OrderFulfillmentGroup{
			GroupNo: group.GetGroupNo(), WarehouseId: group.GetWarehouseId(),
			FreightCent: group.GetFreightCent(), FreightShareCent: group.GetFreightShareCent(),
		})
	}
	return &pb.CreateOrderInnerRequest{
		UserId:                in.UserID,
		OrderNo:               orderNo,
		QuoteId:               in.QuoteID,
		AddressId:             in.AddressID,
		TotalCent:             total,
		DiscountCent:          discount,
		PromotionDiscountCent: quote.GetPromotionDiscountCent(),
		AppliedPromotionsJson: quote.GetAppliedPromotionsJson(),
		FreightCent:           freight,
		PayCent:               pay,
		ReceiverName:          in.ReceiverName,
		ReceiverPhone:         in.ReceiverPhone,
		ReceiverAddress:       in.ReceiverAddress,
		Items:                 items,
		ExpireAtUnix:          expireAt.Unix(),
		FulfillmentGroups:     groups,
		AllocationHash:        quote.GetAllocationHash(),
		PackageCount:          quote.GetPackageCount(),
		WarehouseCount:        quote.GetWarehouseCount(),
	}
}

func recordFromInner(ctx context.Context, req *pb.CreateOrderInnerRequest) *orderRecord {
	expireAt := time.Now().Add(orderExpireWindow)
	if unix := req.GetExpireAtUnix(); unix > 0 {
		expireAt = time.Unix(unix, 0)
	}
	return &orderRecord{
		OrderNo:               req.GetOrderNo(),
		UserID:                req.GetUserId(),
		Status:                statusPendingPayment,
		TotalCent:             req.GetTotalCent(),
		DiscountCent:          req.GetDiscountCent(),
		PromotionDiscountCent: req.GetPromotionDiscountCent(),
		AppliedPromotionsJSON: req.GetAppliedPromotionsJson(),
		FreightCent:           req.GetFreightCent(),
		PayCent:               req.GetPayCent(),
		PaidCent:              0,
		ReceiverName:          req.GetReceiverName(),
		ReceiverPhone:         req.GetReceiverPhone(),
		ReceiverAddress:       req.GetReceiverAddress(),
		QuoteID:               strings.TrimSpace(req.GetQuoteId()),
		ClientRequestID:       clientRequestIDFromContext(ctx, req.GetOrderNo()),
		AllocationHash:        req.GetAllocationHash(),
		PackageCount:          req.GetPackageCount(),
		WarehouseCount:        req.GetWarehouseCount(),
		ExpireAt:              expireAt,
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
		payload := map[string]any{
			"sku_id":        item.GetSkuId(),
			"name":          item.GetName(),
			"price_cent":    item.GetPriceCent(),
			"quantity":      item.GetQuantity(),
			"discount_cent": item.GetDiscountCent(),
		}
		// INV-44/AC-46: Inventory must locate the exact reservation. Never emit
		// an implicit W1 item; an absent warehouse makes the consumer fail
		// closed instead of compensating the wrong warehouse.
		if item.GetWarehouseId() > 0 {
			payload["warehouse_id"] = item.GetWarehouseId()
		}
		out = append(out, payload)
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

// paidAmount validates the captured amount against the order before any side
// effect runs. Keeping it pure makes the "validate first" contract testable.
func paidAmount(record *orderRecord, paidCent int64) (int64, error) {
	if paidCent <= 0 {
		paidCent = record.PayCent
	}
	if paidCent > record.PayCent {
		return 0, apperr.Invalid("paid_cent exceeds pay_cent")
	}
	return paidCent, nil
}
