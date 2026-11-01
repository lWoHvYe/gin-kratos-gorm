package biz

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-kratos-gorm/internal/biz/model"
	"gin-kratos-gorm/internal/pkg/apperr"
	"gin-kratos-gorm/internal/pkg/idgen"

	"github.com/go-kratos/kratos/v2/log"
)

type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

type fakeCatalog struct {
	snaps      map[int64]*SkuSnapshot
	deducts    map[int64]int32
	restores   map[int64]int32
	failDeduct error
}

func (f *fakeCatalog) GetSkuSnapshot(_ context.Context, skuID int64) (*SkuSnapshot, error) {
	s, ok := f.snaps[skuID]
	if !ok {
		return nil, apperr.NotFound("sku not found")
	}
	return s, nil
}

func (f *fakeCatalog) DeductStock(_ context.Context, _ string, skuID int64, num int32) error {
	if f.failDeduct != nil {
		return f.failDeduct
	}
	f.deducts[skuID] += num
	return nil
}

func (f *fakeCatalog) RestoreStock(_ context.Context, _ string, skuID int64, num int32) error {
	f.restores[skuID] += num
	return nil
}

type fakeOrderRepo struct {
	orders   map[string]*model.Order
	existing *model.Order
	created  int
	events   []*DomainEvent
	expired  []*model.Order
	nextID   int64
}

func (f *fakeOrderRepo) Create(_ context.Context, order *model.Order, items []*model.OrderItem, event *DomainEvent) error {
	f.nextID++
	order.ID = f.nextID
	order.OrderItems = make([]model.OrderItem, 0, len(items))
	for _, item := range items {
		item.OrderID = order.ID
		f2 := *item
		order.OrderItems = append(order.OrderItems, f2)
	}
	f.orders[order.OrderSn] = order
	if event != nil {
		f.events = append(f.events, event)
	}
	f.created++
	return nil
}

func (f *fakeOrderRepo) GetBySn(_ context.Context, orderSN string) (*model.Order, error) {
	if o, ok := f.orders[orderSN]; ok {
		return o, nil
	}
	return nil, apperr.NotFound("order not found")
}

func (f *fakeOrderRepo) GetByClientRequestID(_ context.Context, _ int64, _ string) (*model.Order, error) {
	if f.existing != nil {
		return f.existing, nil
	}
	return nil, apperr.NotFound("order not found")
}

func (f *fakeOrderRepo) List(_ context.Context, _ OrderFilter) ([]*model.Order, int64, error) {
	return nil, 0, nil
}

func (f *fakeOrderRepo) UpdateStatus(_ context.Context, orderSN string, from, to int32, fields map[string]any, event *DomainEvent) (bool, error) {
	o, ok := f.orders[orderSN]
	if !ok || o.Status != from {
		return false, nil
	}
	o.Status = to
	if v, ok := fields["cancel_reason"]; ok {
		o.CancelReason, _ = v.(string)
	}
	if event != nil {
		f.events = append(f.events, event)
	}
	return true, nil
}

func (f *fakeOrderRepo) ListExpired(_ context.Context, _ time.Time, _ int) ([]*model.Order, error) {
	return f.expired, nil
}

type nopTx struct{}

func (nopTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

func newTestOrderUsecase(t *testing.T, catalog *fakeCatalog, repo *fakeOrderRepo) *OrderUsecase {
	t.Helper()
	clock := fakeClock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)}
	logger := log.NewStdLogger(discardWriter{})
	return NewOrderUsecase(repo, catalog, &TxSaga{Tx: nopTx{}, Log: log.NewHelper(logger)}, clock,
		idgen.NewOrderSN("test").WithClock(clock.Now),
		OrderConfig{PaymentTimeout: 30 * time.Minute, ShippingFee: 10, FreeShippingThreshold: 99},
		logger,
	)
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func baseCatalog() *fakeCatalog {
	return &fakeCatalog{
		snaps: map[int64]*SkuSnapshot{
			1: {SkuID: 1, SpuID: 1, SkuName: "keyboard", Price: 399, Stock: 10, OnSale: true},
			2: {SkuID: 2, SpuID: 1, SkuName: "book", Price: 89, Stock: 5, OnSale: true},
		},
		deducts:  map[int64]int32{},
		restores: map[int64]int32{},
	}
}

func baseInput() CreateOrderInput {
	return CreateOrderInput{
		UserID:          7,
		ReceiverName:    "张三",
		ReceiverAddress: "上海市",
		Items:           []CreateOrderItem{{SkuID: 1, Num: 2}},
	}
}

func TestOrderCreatePricingAndStock(t *testing.T) {
	catalog := baseCatalog()
	repo := &fakeOrderRepo{orders: map[string]*model.Order{}}
	uc := newTestOrderUsecase(t, catalog, repo)

	order, err := uc.Create(context.Background(), baseInput())
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if order.TotalAmount != 798 || order.PayAmount != 798 {
		t.Fatalf("unexpected amounts: total=%v pay=%v", order.TotalAmount, order.PayAmount)
	}
	if order.Status != OrderStatusPendingPayment {
		t.Fatalf("unexpected status: %d", order.Status)
	}
	if catalog.deducts[1] != 2 {
		t.Fatalf("expected 2 deducted, got %d", catalog.deducts[1])
	}
	if len(repo.events) != 1 || repo.events[0].Type != EventOrderCreated {
		t.Fatalf("expected order.created event, got %+v", repo.events)
	}
	if order.ExpireAt == nil || !order.ExpireAt.After(time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected expire time in the future")
	}
}

func TestOrderCreateShippingFee(t *testing.T) {
	catalog := baseCatalog()
	catalog.snaps[2].Price = 50
	repo := &fakeOrderRepo{orders: map[string]*model.Order{}}
	uc := newTestOrderUsecase(t, catalog, repo)

	order, err := uc.Create(context.Background(), CreateOrderInput{
		UserID: 7, ReceiverName: "张三", ReceiverAddress: "上海市",
		Items: []CreateOrderItem{{SkuID: 2, Num: 1}},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if order.FreightAmount != 10 || order.PayAmount != 60 {
		t.Fatalf("unexpected freight/pay: %v/%v", order.FreightAmount, order.PayAmount)
	}
}

func TestOrderCreateMergesDuplicateSkus(t *testing.T) {
	catalog := baseCatalog()
	repo := &fakeOrderRepo{orders: map[string]*model.Order{}}
	uc := newTestOrderUsecase(t, catalog, repo)

	order, err := uc.Create(context.Background(), CreateOrderInput{
		UserID: 7, ReceiverName: "张三", ReceiverAddress: "上海市",
		Items: []CreateOrderItem{{SkuID: 1, Num: 1}, {SkuID: 1, Num: 2}},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if len(order.OrderItems) != 1 || order.OrderItems[0].Num != 3 {
		t.Fatalf("expected one merged item with num 3, got %+v", order.OrderItems)
	}
	if catalog.deducts[1] != 3 {
		t.Fatalf("expected 3 deducted, got %d", catalog.deducts[1])
	}
}

func TestOrderCreateInsufficientStock(t *testing.T) {
	catalog := baseCatalog()
	catalog.snaps[1].Stock = 1
	repo := &fakeOrderRepo{orders: map[string]*model.Order{}}
	uc := newTestOrderUsecase(t, catalog, repo)

	_, err := uc.Create(context.Background(), baseInput())
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("expected failed precondition, got %v", err)
	}
	if repo.created != 0 {
		t.Fatalf("order must not be created")
	}
}

func TestOrderCreateIdempotent(t *testing.T) {
	catalog := baseCatalog()
	existing := &model.Order{ID: 99, OrderSn: "existing", UserID: 7, Status: OrderStatusCancelled}
	repo := &fakeOrderRepo{orders: map[string]*model.Order{"existing": existing}, existing: existing}
	uc := newTestOrderUsecase(t, catalog, repo)

	in := baseInput()
	in.ClientRequestID = "key-1"
	order, err := uc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if order.ID != 99 || repo.created != 0 || len(catalog.deducts) != 0 {
		t.Fatalf("expected idempotent replay, got order=%+v created=%d deducts=%v", order, repo.created, catalog.deducts)
	}
}

func TestOrderCreateCompensatesWhenPersistFails(t *testing.T) {
	catalog := baseCatalog()
	repo := &failingRepo{fakeOrderRepo: &fakeOrderRepo{orders: map[string]*model.Order{}}}
	logger := log.NewStdLogger(discardWriter{})
	uc := NewOrderUsecase(repo, catalog, &CompensationSaga{Log: log.NewHelper(logger)},
		fakeClock{now: time.Now()}, idgen.NewOrderSN("test"),
		OrderConfig{PaymentTimeout: time.Minute, ShippingFee: 0, FreeShippingThreshold: 0}, logger)

	_, err := uc.Create(context.Background(), baseInput())
	if err == nil {
		t.Fatal("expected create to fail")
	}
	if catalog.restores[1] != 2 {
		t.Fatalf("expected compensation to restore 2, got %d", catalog.restores[1])
	}
}

type failingRepo struct{ *fakeOrderRepo }

func (f *failingRepo) Create(context.Context, *model.Order, []*model.OrderItem, *DomainEvent) error {
	return errors.New("db down")
}

func TestOrderPayAndCancelLifecycle(t *testing.T) {
	catalog := baseCatalog()
	repo := &fakeOrderRepo{orders: map[string]*model.Order{}}
	uc := newTestOrderUsecase(t, catalog, repo)
	order, err := uc.Create(context.Background(), baseInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	paid, err := uc.Pay(context.Background(), 7, order.OrderSn, false)
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if paid.Status != OrderStatusPendingShipment {
		t.Fatalf("expected pending shipment, got %d", paid.Status)
	}
	// Paying twice is idempotent.
	if _, err := uc.Pay(context.Background(), 7, order.OrderSn, false); err != nil {
		t.Fatalf("pay twice: %v", err)
	}

	cancelled, err := uc.Cancel(context.Background(), 7, order.OrderSn, "changed mind", true)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.Status != OrderStatusCancelled || cancelled.CancelReason != "changed mind" {
		t.Fatalf("unexpected cancelled order: %+v", cancelled)
	}
	if catalog.restores[1] != 2 {
		t.Fatalf("expected stock restore, got %d", catalog.restores[1])
	}
}

func TestOrderOwnerCheck(t *testing.T) {
	catalog := baseCatalog()
	repo := &fakeOrderRepo{orders: map[string]*model.Order{}}
	uc := newTestOrderUsecase(t, catalog, repo)
	order, _ := uc.Create(context.Background(), baseInput())

	if _, err := uc.Get(context.Background(), 8, order.OrderSn, false); err == nil {
		t.Fatal("expected forbidden for another user")
	}
	if _, err := uc.Get(context.Background(), 8, order.OrderSn, true); err != nil {
		t.Fatalf("admin should see order: %v", err)
	}
}

func TestAutoCancelExpired(t *testing.T) {
	catalog := baseCatalog()
	repo := &fakeOrderRepo{orders: map[string]*model.Order{}}
	uc := newTestOrderUsecase(t, catalog, repo)
	order, _ := uc.Create(context.Background(), baseInput())
	// Move the order back in time by marking it expired.
	order.Status = OrderStatusPendingPayment
	repo.expired = []*model.Order{order}

	// Cancel calls Get first; keep it in the map.
	count, err := uc.AutoCancelExpired(context.Background(), 10)
	if err != nil {
		t.Fatalf("auto cancel: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 cancelled, got %d", count)
	}
	if order.Status != OrderStatusCancelled {
		t.Fatalf("expected cancelled status, got %d", order.Status)
	}
}

func TestCompensationSagaRunsUndoInReverse(t *testing.T) {
	var calls []string
	saga := &CompensationSaga{}
	err := saga.Run(context.Background(), []Step{
		{Name: "a", Do: func(context.Context) error { calls = append(calls, "do-a"); return nil }, Undo: func(context.Context) error { calls = append(calls, "undo-a"); return nil }},
		{Name: "b", Do: func(context.Context) error { calls = append(calls, "do-b"); return nil }, Undo: func(context.Context) error { calls = append(calls, "undo-b"); return nil }},
		{Name: "c", Do: func(context.Context) error { calls = append(calls, "do-c"); return errors.New("boom") }},
	})
	if err == nil {
		t.Fatal("expected saga failure")
	}
	want := []string{"do-a", "do-b", "do-c", "undo-b", "undo-a"}
	if len(calls) != len(want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls=%v want=%v", calls, want)
		}
	}
}

func TestCreateInnerSkipsStockCheckAndIsIdempotent(t *testing.T) {
	catalog := baseCatalog()
	catalog.snaps[1].Stock = 0 // stock was already deducted by the DTM branch
	repo := &fakeOrderRepo{orders: map[string]*model.Order{}}
	uc := newTestOrderUsecase(t, catalog, repo)

	in := baseInput()
	in.OrderSN = "DTM-SN-1"
	pre := &PrecomputedAmounts{TotalAmount: 798, FreightAmount: 0, PayAmount: 798}
	order, err := uc.CreateInner(context.Background(), in, pre)
	if err != nil {
		t.Fatalf("create inner: %v", err)
	}
	if order.OrderSn != "DTM-SN-1" || len(catalog.deducts) != 0 {
		t.Fatalf("inner branch must not deduct stock: %+v deducts=%v", order, catalog.deducts)
	}

	// A retry with the same order number returns the existing order.
	again, err := uc.CreateInner(context.Background(), in, pre)
	if err != nil {
		t.Fatalf("create inner retry: %v", err)
	}
	if again.OrderSn != order.OrderSn || repo.created != 1 {
		t.Fatalf("expected idempotent inner retry, created=%d", repo.created)
	}
}
