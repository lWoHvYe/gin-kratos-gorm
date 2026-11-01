package biz

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gin-kratos-gorm/internal/biz/model"
	"gin-kratos-gorm/internal/pkg/apperr"
	"gin-kratos-gorm/internal/pkg/idgen"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	OrderStatusPendingPayment  int32 = 1
	OrderStatusPendingShipment int32 = 2
	OrderStatusShipped         int32 = 3
	OrderStatusCompleted       int32 = 4
	OrderStatusCancelled       int32 = 5
	OrderStatusClosed          int32 = 6
)

var orderStatusText = map[int32]string{
	OrderStatusPendingPayment:  "待支付",
	OrderStatusPendingShipment: "待发货",
	OrderStatusShipped:         "待收货",
	OrderStatusCompleted:       "已完成",
	OrderStatusCancelled:       "已取消",
	OrderStatusClosed:          "已关闭",
}

func OrderStatusText(status int32) string {
	if s, ok := orderStatusText[status]; ok {
		return s
	}
	return "未知"
}

func ValidOrderStatus(status int32) bool {
	_, ok := orderStatusText[status]
	return ok
}

type CreateOrderItem struct {
	SkuID int64
	Num   int32
}

type CreateOrderInput struct {
	UserID          int64
	OrderSN         string
	ReceiverName    string
	ReceiverPhone   string
	ReceiverAddress string
	Remark          string
	ClientRequestID string
	Items           []CreateOrderItem
}

type PrecomputedAmounts struct {
	TotalAmount   float64
	FreightAmount float64
	PayAmount     float64
}

type OrderFilter struct {
	UserID   int64
	Status   *int32
	OrderSN  string
	Page     int
	PageSize int
}

type OrderRepository interface {
	Create(ctx context.Context, order *model.Order, items []*model.OrderItem, event *DomainEvent) error
	GetBySn(ctx context.Context, orderSN string) (*model.Order, error)
	GetByClientRequestID(ctx context.Context, userID int64, requestID string) (*model.Order, error)
	List(ctx context.Context, filter OrderFilter) ([]*model.Order, int64, error)
	UpdateStatus(ctx context.Context, orderSN string, from, to int32, fields map[string]any, event *DomainEvent) (bool, error)
	ListExpired(ctx context.Context, now time.Time, limit int) ([]*model.Order, error)
}

type OrderConfig struct {
	PaymentTimeout        time.Duration
	ShippingFee           float64
	FreeShippingThreshold float64
}

type OrderUsecase struct {
	repo    OrderRepository
	catalog ProductCatalog
	saga    Saga
	clock   Clock
	orderSN *idgen.OrderSN
	cfg     OrderConfig
	log     *log.Helper
}

func NewOrderUsecase(
	repo OrderRepository,
	catalog ProductCatalog,
	saga Saga,
	clock Clock,
	orderSN *idgen.OrderSN,
	cfg OrderConfig,
	logger log.Logger,
) *OrderUsecase {
	if clock == nil {
		clock = RealClock{}
	}
	return &OrderUsecase{
		repo:    repo,
		catalog: catalog,
		saga:    saga,
		clock:   clock,
		orderSN: orderSN,
		cfg:     cfg,
		log:     log.NewHelper(logger),
	}
}

type orderDraft struct {
	order      *model.Order
	items      []*model.OrderItem
	event      *DomainEvent
	eventItems []OrderItemPayload
}

// OrderQuote is the priced order snapshot used by the DTM coordinator. It is
// read-only: actual persistence happens in CreateInner (the DTM order branch).
type OrderQuote struct {
	Order    *model.Order
	Items    []OrderItemPayload
	Existing bool
}

func (u *OrderUsecase) Quote(ctx context.Context, in CreateOrderInput) (*OrderQuote, error) {
	draft, err := u.prepare(ctx, in, nil)
	if err != nil {
		return nil, err
	}
	return &OrderQuote{
		Order:    draft.order,
		Items:    draft.eventItems,
		Existing: draft.event == nil && draft.order != nil && draft.order.ID != 0,
	}, nil
}

// Create places an order. Stock deduction and order persistence run through a
// Saga: a single database transaction in the monolith (TxSaga) or remote calls
// with compensation in split deployments (CompensationSaga). DTM is optional.
func (u *OrderUsecase) Create(ctx context.Context, in CreateOrderInput) (*model.Order, error) {
	draft, err := u.prepare(ctx, in, nil)
	if err != nil {
		return nil, err
	}
	if draft.event == nil && draft.order != nil && draft.order.ID != 0 {
		// Idempotent replay: the client request id already produced this order.
		return draft.order, nil
	}

	// Use the normalized draft items (duplicate sku ids are merged) so the
	// stock steps match the persisted order items exactly.
	steps := make([]Step, 0, len(draft.items)+1)
	for _, item := range draft.items {
		item := item
		steps = append(steps, Step{
			Name: fmt.Sprintf("deduct-stock-%d", item.SkuID),
			Do: func(ctx context.Context) error {
				return u.catalog.DeductStock(ctx, draft.order.OrderSn, item.SkuID, item.Num)
			},
			Undo: func(ctx context.Context) error {
				return u.catalog.RestoreStock(ctx, draft.order.OrderSn, item.SkuID, item.Num)
			},
		})
	}
	steps = append(steps, Step{
		Name: "persist-order",
		Do: func(ctx context.Context) error {
			return u.repo.Create(ctx, draft.order, draft.items, draft.event)
		},
	})

	if err := u.saga.Run(ctx, steps); err != nil {
		u.log.Errorf("create order failed: %v", err)
		return nil, err
	}
	return u.repo.GetBySn(ctx, draft.order.OrderSn)
}

// CreateInner persists an order that has already been coordinated by DTM. It
// does not deduct stock again; the DTM saga owns that branch.
func (u *OrderUsecase) CreateInner(ctx context.Context, in CreateOrderInput, pre *PrecomputedAmounts) (*model.Order, error) {
	// DTM may retry the inner branch. The order number is the idempotency key.
	if in.OrderSN != "" {
		if existing, err := u.repo.GetBySn(ctx, in.OrderSN); err == nil && existing != nil {
			return existing, nil
		} else if err != nil && !apperr.IsCode(err, apperr.CodeNotFound) {
			return nil, err
		}
	}
	draft, err := u.prepare(ctx, in, pre)
	if err != nil {
		return nil, err
	}
	if err := u.repo.Create(ctx, draft.order, draft.items, draft.event); err != nil {
		return nil, err
	}
	return u.repo.GetBySn(ctx, draft.order.OrderSn)
}

// prepare validates input, obtains authoritative SKU data and builds the order
// aggregate. It is shared by the local saga and the DTM inner endpoint.
func (u *OrderUsecase) prepare(ctx context.Context, in CreateOrderInput, pre *PrecomputedAmounts) (*orderDraft, error) {
	if in.UserID <= 0 {
		return nil, apperr.Unauthenticated("login required")
	}
	items, err := normalizeItems(in.Items)
	if err != nil {
		return nil, err
	}
	in.Items = items
	if strings.TrimSpace(in.ReceiverName) == "" {
		return nil, apperr.Invalid("receiver name is required")
	}
	if strings.TrimSpace(in.ReceiverAddress) == "" {
		return nil, apperr.Invalid("receiver address is required")
	}

	if in.ClientRequestID != "" {
		existing, err := u.repo.GetByClientRequestID(ctx, in.UserID, in.ClientRequestID)
		if err == nil && existing != nil {
			return &orderDraft{order: existing, items: orderItemsAsPointers(existing.OrderItems), event: nil}, nil
		}
		if err != nil && !apperr.IsCode(err, apperr.CodeNotFound) {
			return nil, err
		}
	}

	snapshots := make([]*SkuSnapshot, 0, len(items))
	var total float64
	for _, item := range items {
		snapshot, err := u.catalog.GetSkuSnapshot(ctx, item.SkuID)
		if err != nil {
			return nil, err
		}
		if !snapshot.OnSale {
			return nil, apperr.FailedPrecondition(fmt.Sprintf("sku %d is not on sale", item.SkuID))
		}
		// The DTM inner branch runs after the stock branch; checking again would
		// reject an already-deducted order.
		if pre == nil && snapshot.Stock < int64(item.Num) {
			return nil, apperr.FailedPrecondition(fmt.Sprintf("sku %d has insufficient stock", item.SkuID))
		}
		snapshots = append(snapshots, snapshot)
		total += snapshot.Price * float64(item.Num)
	}
	total = roundMoney(total)
	freight := roundMoney(u.freightFor(total))
	payAmount := roundMoney(total + freight)
	if pre != nil {
		if pre.TotalAmount > 0 {
			total = roundMoney(pre.TotalAmount)
		}
		if pre.FreightAmount >= 0 {
			freight = roundMoney(pre.FreightAmount)
		}
		if pre.PayAmount > 0 {
			payAmount = roundMoney(pre.PayAmount)
		}
	}

	now := u.clock.Now()
	expireAt := now.Add(u.cfg.PaymentTimeout)
	orderSN := in.OrderSN
	if orderSN == "" {
		orderSN = u.orderSN.Next()
	}
	order := &model.Order{
		OrderSn:         orderSN,
		UserID:          in.UserID,
		Status:          OrderStatusPendingPayment,
		TotalAmount:     total,
		PayAmount:       payAmount,
		FreightAmount:   freight,
		ReceiverName:    strings.TrimSpace(in.ReceiverName),
		ReceiverPhone:   strings.TrimSpace(in.ReceiverPhone),
		ReceiverAddress: strings.TrimSpace(in.ReceiverAddress),
		Remark:          strings.TrimSpace(in.Remark),
		ExpireAt:        &expireAt,
	}
	if in.ClientRequestID != "" {
		id := in.ClientRequestID
		order.ClientRequestID = &id
	}

	orderItems := make([]*model.OrderItem, 0, len(items))
	eventItems := make([]OrderItemPayload, 0, len(items))
	for i, item := range items {
		orderItems = append(orderItems, &model.OrderItem{
			OrderSn: order.OrderSn,
			SpuID:   snapshots[i].SpuID,
			SkuID:   item.SkuID,
			SkuName: snapshots[i].SkuName,
			SkuPic:  snapshots[i].SkuPic,
			Price:   snapshots[i].Price,
			Num:     item.Num,
		})
		eventItems = append(eventItems, OrderItemPayload{
			SkuID:   item.SkuID,
			SkuName: snapshots[i].SkuName,
			Num:     item.Num,
			Price:   snapshots[i].Price,
		})
	}
	event := NewOrderEvent(EventOrderCreated, order.OrderSn, order.UserID, order.Status, order.PayAmount, "", eventItems, now)
	return &orderDraft{order: order, items: orderItems, event: event, eventItems: eventItems}, nil
}

func (u *OrderUsecase) Get(ctx context.Context, userID int64, orderSN string, admin bool) (*model.Order, error) {
	order, err := u.repo.GetBySn(ctx, orderSN)
	if err != nil {
		return nil, err
	}
	if !admin && order.UserID != userID {
		return nil, apperr.Forbidden("you do not have access to this order")
	}
	return order, nil
}

func (u *OrderUsecase) List(ctx context.Context, userID int64, filter OrderFilter, admin bool) ([]*model.Order, int64, error) {
	if filter.Status != nil && !ValidOrderStatus(*filter.Status) {
		return nil, 0, apperr.Invalid("invalid order status")
	}
	if !admin {
		filter.UserID = userID
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	return u.repo.List(ctx, filter)
}

// Pay marks a pending order as paid. Repeating the call is idempotent.
func (u *OrderUsecase) Pay(ctx context.Context, userID int64, orderSN string, admin bool) (*model.Order, error) {
	order, err := u.Get(ctx, userID, orderSN, admin)
	if err != nil {
		return nil, err
	}
	switch order.Status {
	case OrderStatusPendingShipment, OrderStatusShipped, OrderStatusCompleted:
		return order, nil
	case OrderStatusPendingPayment:
		// continue
	default:
		return nil, apperr.FailedPrecondition("order cannot be paid in status " + OrderStatusText(order.Status))
	}

	now := u.clock.Now()
	event := NewOrderEvent(EventOrderPaid, order.OrderSn, order.UserID, OrderStatusPendingShipment, order.PayAmount, "", orderItemsPayload(order), now)
	ok, err := u.repo.UpdateStatus(ctx, order.OrderSn, OrderStatusPendingPayment, OrderStatusPendingShipment, map[string]any{
		"pay_time": now,
	}, event)
	if err != nil {
		return nil, err
	}
	if !ok {
		latest, getErr := u.repo.GetBySn(ctx, order.OrderSn)
		if getErr != nil {
			return nil, getErr
		}
		return latest, nil
	}
	return u.repo.GetBySn(ctx, order.OrderSn)
}

// Cancel cancels an unpaid order and restores stock. The stock restore is
// idempotent and also recorded in the outbox for retry if the product service is
// temporarily unavailable.
func (u *OrderUsecase) Cancel(ctx context.Context, userID int64, orderSN, reason string, admin bool) (*model.Order, error) {
	order, err := u.Get(ctx, userID, orderSN, admin)
	if err != nil {
		return nil, err
	}
	if order.Status == OrderStatusCancelled {
		return order, nil
	}
	if order.Status != OrderStatusPendingPayment && order.Status != OrderStatusPendingShipment {
		return nil, apperr.FailedPrecondition("order cannot be cancelled in status " + OrderStatusText(order.Status))
	}
	if !admin && order.Status != OrderStatusPendingPayment {
		return nil, apperr.FailedPrecondition("only unpaid orders can be cancelled by the buyer")
	}

	now := u.clock.Now()
	event := NewOrderEvent(EventOrderCancelled, order.OrderSn, order.UserID, OrderStatusCancelled, order.PayAmount, reason, orderItemsPayload(order), now)
	ok, err := u.repo.UpdateStatus(ctx, order.OrderSn, order.Status, OrderStatusCancelled, map[string]any{
		"cancel_reason": reason,
	}, event)
	if err != nil {
		return nil, err
	}
	if !ok {
		latest, getErr := u.repo.GetBySn(ctx, order.OrderSn)
		if getErr != nil {
			return nil, getErr
		}
		return latest, nil
	}

	for _, item := range order.OrderItems {
		if err := u.catalog.RestoreStock(ctx, order.OrderSn, item.SkuID, item.Num); err != nil {
			u.log.Errorf("restore stock for order %s sku %d failed, outbox retry pending: %v", order.OrderSn, item.SkuID, err)
		}
	}
	return u.repo.GetBySn(ctx, order.OrderSn)
}

// AutoCancelExpired cancels unpaid orders whose payment window elapsed. It is
// safe to run concurrently with user-triggered cancellation because the status
// transition is a conditional update.
func (u *OrderUsecase) AutoCancelExpired(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	expired, err := u.repo.ListExpired(ctx, u.clock.Now(), limit)
	if err != nil {
		return 0, err
	}
	cancelled := 0
	for _, order := range expired {
		if _, err := u.Cancel(ctx, order.UserID, order.OrderSn, "payment timeout", true); err != nil {
			if errors.Is(err, context.Canceled) {
				return cancelled, err
			}
			u.log.Warnf("auto cancel order %s: %v", order.OrderSn, err)
			continue
		}
		cancelled++
	}
	return cancelled, nil
}

func (u *OrderUsecase) freightFor(total float64) float64 {
	if u.cfg.ShippingFee <= 0 || total >= u.cfg.FreeShippingThreshold {
		return 0
	}
	return u.cfg.ShippingFee
}

func normalizeItems(items []CreateOrderItem) ([]CreateOrderItem, error) {
	if len(items) == 0 {
		return nil, apperr.Invalid("at least one order item is required")
	}
	if len(items) > 100 {
		return nil, apperr.Invalid("too many order items (max 100)")
	}
	merged := make(map[int64]int32, len(items))
	order := make([]int64, 0, len(items))
	for _, item := range items {
		if item.SkuID <= 0 {
			return nil, apperr.Invalid("sku id must be positive")
		}
		if item.Num <= 0 {
			return nil, apperr.Invalid("item quantity must be positive")
		}
		if _, ok := merged[item.SkuID]; !ok {
			order = append(order, item.SkuID)
		}
		merged[item.SkuID] += item.Num
	}
	result := make([]CreateOrderItem, 0, len(order))
	for _, skuID := range order {
		result = append(result, CreateOrderItem{SkuID: skuID, Num: merged[skuID]})
	}
	return result, nil
}

func orderItemsPayload(order *model.Order) []OrderItemPayload {
	items := make([]OrderItemPayload, 0, len(order.OrderItems))
	for _, item := range order.OrderItems {
		items = append(items, OrderItemPayload{
			SkuID:   item.SkuID,
			SkuName: item.SkuName,
			Num:     item.Num,
			Price:   item.Price,
		})
	}
	return items
}

func orderItemsAsPointers(items []model.OrderItem) []*model.OrderItem {
	result := make([]*model.OrderItem, 0, len(items))
	for i := range items {
		result = append(result, &items[i])
	}
	return result
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}
