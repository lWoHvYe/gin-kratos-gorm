package aftersales

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	mysqldriver "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
)

func afterSaleTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run after-sales settlement tests")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil || cfg.DBName == "" {
		t.Fatalf("TEST_MYSQL_DSN must name a database: %v", err)
	}
	adminCfg := *cfg
	adminCfg.DBName = ""
	admin, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec("CREATE DATABASE IF NOT EXISTS `" + cfg.DBName + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	_ = admin.Close()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, table := range []string{"after_sale_items", "after_sale_restock_tasks"} {
		if _, err := db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			t.Fatal(err)
		}
	}
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS after_sales (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, after_sale_no VARCHAR(64) NOT NULL,
			order_no VARCHAR(64) NOT NULL, user_id BIGINT NOT NULL, ` + "`type`" + ` VARCHAR(32) NOT NULL,
			status VARCHAR(24) NOT NULL, amount_cent BIGINT NOT NULL,
			effective_amount_cent BIGINT NOT NULL DEFAULT 0, amount_state VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
			restock_state VARCHAR(24) NOT NULL DEFAULT 'NONE', reason VARCHAR(512) NOT NULL DEFAULT '',
			client_request_id VARCHAR(64) NULL, return_carrier VARCHAR(64) NOT NULL DEFAULT '',
			return_tracking_no VARCHAR(128) NOT NULL DEFAULT '', refund_no VARCHAR(64) NOT NULL DEFAULT '',
			refunded_cent BIGINT NOT NULL DEFAULT 0, version BIGINT NOT NULL DEFAULT 1,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_after_sales_no (after_sale_no))`,
		`CREATE TABLE IF NOT EXISTS after_sale_items (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, after_sale_id BIGINT NOT NULL,
			sku_id BIGINT NOT NULL, quantity INT NOT NULL, warehouse_id BIGINT NOT NULL DEFAULT 1,
			group_no VARCHAR(64) NOT NULL DEFAULT 'G1', received_quantity INT NOT NULL DEFAULT 0,
			condition_code VARCHAR(16) NOT NULL DEFAULT 'SELLABLE', created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_after_sale_items_group (after_sale_id, sku_id, warehouse_id, group_no))`,
		`CREATE TABLE IF NOT EXISTS after_sale_logs (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, after_sale_id BIGINT NOT NULL,
			action VARCHAR(32) NOT NULL, from_status VARCHAR(24) NOT NULL DEFAULT '',
			to_status VARCHAR(24) NOT NULL, remark VARCHAR(512) NOT NULL DEFAULT '',
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3))`,
		`CREATE TABLE IF NOT EXISTS after_sale_amount_versions (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, after_sale_id BIGINT NOT NULL,
			version INT NOT NULL, effective_amount_cent BIGINT NOT NULL, reason VARCHAR(64) NOT NULL DEFAULT '',
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_after_sale_amount_versions (after_sale_id, version))`,
		`CREATE TABLE IF NOT EXISTS after_sale_amount_ledger (
			order_no VARCHAR(64) NOT NULL PRIMARY KEY, captured_cent BIGINT NOT NULL DEFAULT 0,
			consumed_cent BIGINT NOT NULL DEFAULT 0, source VARCHAR(32) NOT NULL DEFAULT 'uninitialized',
			version BIGINT NOT NULL DEFAULT 1,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3))`,
		`CREATE TABLE IF NOT EXISTS after_sale_restock_tasks (
			after_sale_no VARCHAR(64) NOT NULL, sku_id BIGINT NOT NULL,
			warehouse_id BIGINT NOT NULL DEFAULT 1, group_no VARCHAR(64) NOT NULL DEFAULT 'G1',
			quantity BIGINT NOT NULL,
			` + "`condition`" + ` VARCHAR(16) NOT NULL DEFAULT 'SELLABLE', status VARCHAR(24) NOT NULL DEFAULT 'PENDING',
			attempts INT NOT NULL DEFAULT 0, last_error VARCHAR(512) NOT NULL DEFAULT '',
			lease_owner VARCHAR(128) NOT NULL DEFAULT '', lease_until DATETIME(3) NULL,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
			PRIMARY KEY (after_sale_no, sku_id, warehouse_id, group_no))`,
		`CREATE TABLE IF NOT EXISTS after_sale_refund_tasks (
			after_sale_no VARCHAR(64) NOT NULL PRIMARY KEY, status VARCHAR(24) NOT NULL DEFAULT 'PENDING',
			attempts INT NOT NULL DEFAULT 0, last_error VARCHAR(512) NOT NULL DEFAULT '',
			lease_owner VARCHAR(128) NOT NULL DEFAULT '', lease_until DATETIME(3) NULL,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3))`,
		`CREATE TABLE IF NOT EXISTS outbox_events (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, event_id VARCHAR(64) NOT NULL,
			topic VARCHAR(128) NOT NULL, aggregate_type VARCHAR(64) NOT NULL,
			aggregate_id VARCHAR(64) NOT NULL, event_type VARCHAR(64) NOT NULL,
			payload JSON NOT NULL, attempts INT NOT NULL DEFAULT 0,
			last_error VARCHAR(1024) NOT NULL DEFAULT '', next_attempt_at DATETIME(3) NULL,
			published_at DATETIME(3) NULL, dead_lettered_at DATETIME(3) NULL,
			lease_owner VARCHAR(128) NULL, lease_until DATETIME(3) NULL,
			traceparent VARCHAR(256) NULL, created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_outbox_event_id (event_id))`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create after-sales test table: %v", err)
		}
	}
	for _, table := range []string{"after_sale_restock_tasks", "after_sale_refund_tasks", "after_sale_amount_versions",
		"after_sale_amount_ledger", "after_sale_items", "after_sale_logs", "after_sales", "outbox_events"} {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

type fakeOrderService struct {
	pb.OrderServiceClient
	order         *pb.Order
	blocked       []string
	blockedGroups []string
	blockErr      error
	blockCalls    int
}

func (f *fakeOrderService) GetOrder(context.Context, *pb.GetOrderRequest, ...grpc.CallOption) (*pb.Order, error) {
	return f.order, nil
}

func (f *fakeOrderService) BlockShipmentForRefund(_ context.Context, in *pb.ShipmentPermitRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	f.blockCalls++
	f.blocked = append(f.blocked, in.GetAfterSaleNo())
	f.blockedGroups = append(f.blockedGroups, in.GetFulfillmentGroupNo())
	return &pb.Empty{}, f.blockErr
}

type fakePaymentService struct {
	pb.PaymentServiceClient
	captured     int64
	refundErr    error
	refundCalls  int
	refundAmount []int64
}

func (f *fakePaymentService) GetPaymentByOrder(context.Context, *pb.GetPaymentByOrderRequest, ...grpc.CallOption) (*pb.Intent, error) {
	return &pb.Intent{PaymentNo: "PAY-1", CapturedCent: f.captured, Status: "SUCCEEDED"}, nil
}

func (f *fakePaymentService) RefundOrder(_ context.Context, in *pb.RefundOrderRequest, _ ...grpc.CallOption) (*pb.RefundOrderReply, error) {
	f.refundCalls++
	f.refundAmount = append(f.refundAmount, in.GetAmountCent())
	if f.refundErr != nil {
		return nil, f.refundErr
	}
	return &pb.RefundOrderReply{RefundNo: "RFD-1", PaymentNo: "PAY-1", RefundedCent: in.GetAmountCent()}, nil
}

type fakeInventoryService struct {
	pb.InventoryServiceClient
	restockErr   error
	restockCalls int
	restocked    []int64
	warehouses   []int64
}

func (f *fakeInventoryService) Restock(_ context.Context, in *pb.RestockRequest, _ ...grpc.CallOption) (*pb.Stock, error) {
	f.restockCalls++
	f.restocked = append(f.restocked, in.GetSkuId())
	f.warehouses = append(f.warehouses, in.GetWarehouseId())
	if f.restockErr != nil {
		return nil, f.restockErr
	}
	return &pb.Stock{SkuId: in.GetSkuId()}, nil
}

func newAfterSaleTestService(t *testing.T, db *sql.DB, orderStatus string) (*Service, *fakeOrderService, *fakePaymentService, *fakeInventoryService) {
	t.Helper()
	order := &fakeOrderService{order: &pb.Order{
		OrderNo: "ORD-AS", UserId: 1, Status: orderStatus, PaidCent: 1000, RefundState: "NONE",
		Items: []*pb.OrderItem{{SkuId: 1, Quantity: 1}},
	}}
	payment := &fakePaymentService{captured: 1000}
	inventory := &fakeInventoryService{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := New(db, nil, payment, inventory, order, logger)
	return svc, order, payment, inventory
}

func createAfterSale(t *testing.T, svc *Service, typ string, amount int64) *pb.AfterSale {
	t.Helper()
	as, err := svc.CreateAfterSale(context.Background(), &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-AS", UserId: 1, Type: typ, AmountCent: amount, Reason: "test",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("create after sale: %v", err)
	}
	return as
}

func approveAfterSale(t *testing.T, svc *Service, no string, amount int64) {
	t.Helper()
	if _, err := svc.Approve(context.Background(), &pb.ApproveRequest{AfterSaleNo: no, Approved: true, AmountCent: amount}); err != nil {
		t.Fatalf("approve: %v", err)
	}
}

func TestAfterSaleAmountLedgerUsesCaptured(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, _, _, _ := newAfterSaleTestService(t, db, "PAID")
	first := createAfterSale(t, svc, typeRefundOnly, 600)
	if first.GetEffectiveAmountCent() != 600 || first.GetAmountState() != amountActive {
		t.Fatalf("first = %+v", first)
	}
	if _, err := svc.CreateAfterSale(context.Background(), &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-AS", UserId: 1, Type: typeRefundOnly, AmountCent: 500, Reason: "over",
	}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("over-cap create = %v", err)
	}
	var captured, consumed int64
	if err := db.QueryRow(`SELECT captured_cent, consumed_cent FROM after_sale_amount_ledger WHERE order_no = 'ORD-AS'`).
		Scan(&captured, &consumed); err != nil {
		t.Fatal(err)
	}
	if captured != 1000 || consumed != 600 {
		t.Fatalf("ledger = %d/%d", captured, consumed)
	}
}

func TestPartialApprovalReleasesDifference(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, _, _, _ := newAfterSaleTestService(t, db, "PAID")
	as := createAfterSale(t, svc, typeRefundOnly, 600)
	approveAfterSale(t, svc, as.GetAfterSaleNo(), 400)
	var captured, consumed int64
	if err := db.QueryRow(`SELECT captured_cent, consumed_cent FROM after_sale_amount_ledger WHERE order_no = 'ORD-AS'`).
		Scan(&captured, &consumed); err != nil {
		t.Fatal(err)
	}
	if captured != 1000 || consumed != 400 {
		t.Fatalf("ledger = %d/%d", captured, consumed)
	}
	var effective int64
	var state string
	if err := db.QueryRow(`SELECT effective_amount_cent, amount_state FROM after_sales WHERE after_sale_no = ?`, as.GetAfterSaleNo()).
		Scan(&effective, &state); err != nil {
		t.Fatal(err)
	}
	if effective != 400 || state != amountActive {
		t.Fatalf("after sale = %d/%s", effective, state)
	}
	var versions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM after_sale_amount_versions v JOIN after_sales a ON a.id=v.after_sale_id
		WHERE a.after_sale_no = ?`, as.GetAfterSaleNo()).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions < 2 {
		t.Fatalf("amount versions = %d", versions)
	}
	// The released 200 is available for another application.
	if _, err := svc.CreateAfterSale(context.Background(), &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-AS", UserId: 1, Type: typeRefundOnly, AmountCent: 600, Reason: "remaining",
	}); err != nil {
		t.Fatalf("second create after partial release: %v", err)
	}
}

func TestReturnRefundBlocksUntilRestockDone(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, order, payment, inventory := newAfterSaleTestService(t, db, "SHIPPED")
	as := createAfterSale(t, svc, typeReturnRefund, 600)
	approveAfterSale(t, svc, as.GetAfterSaleNo(), 600)
	if _, err := svc.SubmitReturnTracking(context.Background(), &pb.SubmitReturnTrackingRequest{
		AfterSaleNo: as.GetAfterSaleNo(), Carrier: "SF", TrackingNo: "SF1",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveReturn(context.Background(), &pb.ReceiveReturnRequest{
		AfterSaleNo: as.GetAfterSaleNo(), Condition: "SELLABLE",
	}); err != nil {
		t.Fatal(err)
	}
	var status, restockState string
	if err := db.QueryRow(`SELECT status, restock_state FROM after_sales WHERE after_sale_no = ?`, as.GetAfterSaleNo()).
		Scan(&status, &restockState); err != nil {
		t.Fatal(err)
	}
	if status != statusReturnReceived || restockState != restockPending {
		t.Fatalf("receive = %s/%s", status, restockState)
	}
	if inventory.restockCalls != 0 {
		t.Fatalf("receive called inventory %d times", inventory.restockCalls)
	}
	if _, err := svc.Refund(context.Background(), &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()}); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("refund before restock = %v", err)
	}
	if payment.refundCalls != 0 || order.blockCalls != 0 {
		t.Fatalf("refund before restock called payment=%d block=%d", payment.refundCalls, order.blockCalls)
	}
	if err := svc.processRestockTask(context.Background(), as.GetAfterSaleNo(), 1); err != nil {
		t.Fatalf("restock task: %v", err)
	}
	if inventory.restockCalls != 1 {
		t.Fatalf("restock calls = %d", inventory.restockCalls)
	}
	if err := db.QueryRow(`SELECT restock_state FROM after_sales WHERE after_sale_no = ?`, as.GetAfterSaleNo()).Scan(&restockState); err != nil {
		t.Fatal(err)
	}
	if restockState != restockDone {
		t.Fatalf("restock_state = %s", restockState)
	}
	if _, err := svc.Refund(context.Background(), &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()}); err != nil {
		t.Fatalf("refund after restock: %v", err)
	}
	if order.blockCalls != 1 || payment.refundCalls != 1 || len(payment.refundAmount) != 1 || payment.refundAmount[0] != 600 {
		t.Fatalf("refund calls block=%d payment=%d amount=%v", order.blockCalls, payment.refundCalls, payment.refundAmount)
	}
	var refunded int64
	var amountState string
	if err := db.QueryRow(`SELECT status, refunded_cent, amount_state FROM after_sales WHERE after_sale_no = ?`, as.GetAfterSaleNo()).
		Scan(&status, &refunded, &amountState); err != nil {
		t.Fatal(err)
	}
	if status != statusRefunded || refunded != 600 || amountState != amountConsumed {
		t.Fatalf("refunded = %s/%d/%s", status, refunded, amountState)
	}
	// A duplicate refund is a no-op and cannot pay twice.
	if _, err := svc.Refund(context.Background(), &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()}); err != nil {
		t.Fatal(err)
	}
	if payment.refundCalls != 1 {
		t.Fatalf("duplicate refund called payment %d times", payment.refundCalls)
	}
}

func TestRestockFailureBlocksRefundUntilRetry(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, _, payment, inventory := newAfterSaleTestService(t, db, "SHIPPED")
	as := createAfterSale(t, svc, typeReturnRefund, 600)
	approveAfterSale(t, svc, as.GetAfterSaleNo(), 600)
	if _, err := svc.SubmitReturnTracking(context.Background(), &pb.SubmitReturnTrackingRequest{
		AfterSaleNo: as.GetAfterSaleNo(), Carrier: "SF", TrackingNo: "SF1",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveReturn(context.Background(), &pb.ReceiveReturnRequest{AfterSaleNo: as.GetAfterSaleNo()}); err != nil {
		t.Fatal(err)
	}
	inventory.restockErr = context.DeadlineExceeded
	if err := svc.processRestockTask(context.Background(), as.GetAfterSaleNo(), 1); err == nil {
		t.Fatal("expected restock failure")
	}
	var restockState, taskStatus string
	var attempts int
	if err := db.QueryRow(`SELECT restock_state FROM after_sales WHERE after_sale_no = ?`, as.GetAfterSaleNo()).Scan(&restockState); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status, attempts FROM after_sale_restock_tasks WHERE after_sale_no = ? AND sku_id = 1`, as.GetAfterSaleNo()).
		Scan(&taskStatus, &attempts); err != nil {
		t.Fatal(err)
	}
	if restockState != restockPending || taskStatus != taskPending || attempts != 1 {
		t.Fatalf("failed task = %s/%s/%d", restockState, taskStatus, attempts)
	}
	if _, err := svc.Refund(context.Background(), &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()}); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("refund with failed restock = %v", err)
	}
	if payment.refundCalls != 0 {
		t.Fatal("payment called before restock succeeded")
	}
	inventory.restockErr = nil
	if err := svc.processRestockTask(context.Background(), as.GetAfterSaleNo(), 1); err != nil {
		t.Fatalf("retry restock: %v", err)
	}
	if _, err := svc.Refund(context.Background(), &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()}); err != nil {
		t.Fatalf("refund after successful retry: %v", err)
	}
}

func TestRefundTaskRetriesPaymentIdempotently(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, _, payment, _ := newAfterSaleTestService(t, db, "PAID")
	as := createAfterSale(t, svc, typeRefundOnly, 600)
	approveAfterSale(t, svc, as.GetAfterSaleNo(), 600)
	payment.refundErr = context.DeadlineExceeded
	if _, err := svc.Refund(context.Background(), &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()}); err == nil {
		t.Fatal("expected queued refund error")
	}
	var status, taskStatus string
	var attempts int
	if err := db.QueryRow(`SELECT status FROM after_sales WHERE after_sale_no = ?`, as.GetAfterSaleNo()).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status, attempts FROM after_sale_refund_tasks WHERE after_sale_no = ?`, as.GetAfterSaleNo()).
		Scan(&taskStatus, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != statusRefunding || taskStatus != taskPending || attempts != 1 {
		t.Fatalf("queued refund = %s/%s/%d", status, taskStatus, attempts)
	}
	payment.refundErr = nil
	if err := svc.processRefundTask(context.Background(), as.GetAfterSaleNo()); err != nil {
		t.Fatalf("refund worker retry: %v", err)
	}
	if payment.refundCalls != 2 {
		t.Fatalf("payment calls = %d", payment.refundCalls)
	}
	if err := db.QueryRow(`SELECT status FROM after_sales WHERE after_sale_no = ?`, as.GetAfterSaleNo()).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusRefunded {
		t.Fatalf("status after retry = %s", status)
	}
}

func TestConcurrentAfterSaleCreateRespectsCaptured(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, _, _, _ := newAfterSaleTestService(t, db, "PAID")
	const workers = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.CreateAfterSale(context.Background(), &pb.CreateAfterSaleRequest{
				OrderNo: "ORD-AS", UserId: 1, Type: typeRefundOnly, AmountCent: 200, Reason: "race",
			})
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if success != 5 {
		t.Fatalf("successful creates = %d, want 5 (1000/200)", success)
	}
	var consumed int64
	if err := db.QueryRow(`SELECT consumed_cent FROM after_sale_amount_ledger WHERE order_no = 'ORD-AS'`).Scan(&consumed); err != nil {
		t.Fatal(err)
	}
	if consumed != 1000 {
		t.Fatalf("consumed = %d", consumed)
	}
}

func TestRestockPermanentFailureEmitsEvent(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, _, _, inventory := newAfterSaleTestService(t, db, "SHIPPED")
	as := createAfterSale(t, svc, typeReturnRefund, 600)
	approveAfterSale(t, svc, as.GetAfterSaleNo(), 600)
	if _, err := svc.SubmitReturnTracking(context.Background(), &pb.SubmitReturnTrackingRequest{
		AfterSaleNo: as.GetAfterSaleNo(), Carrier: "SF", TrackingNo: "SF1",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveReturn(context.Background(), &pb.ReceiveReturnRequest{AfterSaleNo: as.GetAfterSaleNo()}); err != nil {
		t.Fatal(err)
	}
	inventory.restockErr = context.DeadlineExceeded
	for attempt := 1; attempt <= maxTaskAttempts; attempt++ {
		_ = svc.processRestockTask(context.Background(), as.GetAfterSaleNo(), 1)
	}
	var taskStatus, restockState string
	if err := db.QueryRow(`SELECT status FROM after_sale_restock_tasks WHERE after_sale_no = ? AND sku_id = 1`, as.GetAfterSaleNo()).Scan(&taskStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT restock_state FROM after_sales WHERE after_sale_no = ?`, as.GetAfterSaleNo()).Scan(&restockState); err != nil {
		t.Fatal(err)
	}
	if taskStatus != taskFailed || restockState != restockFailed {
		t.Fatalf("permanent restock = %s/%s", taskStatus, restockState)
	}
	var events int
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox_events WHERE topic = 'shop.aftersale.restock_failed.v1'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("restock_failed events = %d", events)
	}
}

func TestAfterSaleGroupRefundAndRestock(t *testing.T) {
	db := afterSaleTestDB(t)
	order := &fakeOrderService{order: &pb.Order{
		OrderNo: "ORD-AS", UserId: 1, Status: "SHIPPED", PaidCent: 2000, RefundState: "NONE",
		Items: []*pb.OrderItem{
			{SkuId: 1, Quantity: 1, WarehouseId: 1, GroupNo: "G1"},
			{SkuId: 2, Quantity: 1, WarehouseId: 2, GroupNo: "G2"},
		},
		FulfillmentGroups: []*pb.OrderFulfillmentGroup{
			{GroupNo: "G1", WarehouseId: 1, Status: "SHIPPED"},
			{GroupNo: "G2", WarehouseId: 2, Status: "SHIPPED"},
		},
	}}
	payment := &fakePaymentService{captured: 2000}
	inventory := &fakeInventoryService{}
	svc := New(db, nil, payment, inventory, order, slog.New(slog.NewTextHandler(io.Discard, nil)))
	as, err := svc.CreateAfterSale(context.Background(), &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-AS", UserId: 1, Type: typeReturnRefund, AmountCent: 1000, Reason: "group 2",
		Items: []*pb.AfterSaleItem{{SkuId: 2, Quantity: 1, WarehouseId: 2, GroupNo: "G2"}},
	})
	if err != nil {
		t.Fatalf("create group 2 after-sale: %v", err)
	}
	if _, err := svc.Approve(context.Background(), &pb.ApproveRequest{AfterSaleNo: as.GetAfterSaleNo(), Approved: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitReturnTracking(context.Background(), &pb.SubmitReturnTrackingRequest{
		AfterSaleNo: as.GetAfterSaleNo(), Carrier: "SF", TrackingNo: "SF2",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveReturn(context.Background(), &pb.ReceiveReturnRequest{AfterSaleNo: as.GetAfterSaleNo()}); err != nil {
		t.Fatal(err)
	}
	var warehouseID int64
	var groupNo string
	if err := db.QueryRow(`SELECT warehouse_id, group_no FROM after_sale_restock_tasks WHERE after_sale_no = ? AND sku_id = 2`,
		as.GetAfterSaleNo()).Scan(&warehouseID, &groupNo); err != nil {
		t.Fatal(err)
	}
	if warehouseID != 2 || groupNo != "G2" {
		t.Fatalf("restock task = %d/%s", warehouseID, groupNo)
	}
	if err := svc.processRestockTaskForGroup(context.Background(), as.GetAfterSaleNo(), 2, 2, "G2"); err != nil {
		t.Fatal(err)
	}
	if len(inventory.warehouses) != 1 || inventory.warehouses[0] != 2 {
		t.Fatalf("restock warehouses = %v", inventory.warehouses)
	}
	if _, err := svc.Refund(context.Background(), &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()}); err != nil {
		t.Fatalf("refund group 2: %v", err)
	}
	if len(order.blocked) != 1 || order.blocked[0] != as.GetAfterSaleNo() {
		t.Fatalf("blocked = %v", order.blocked)
	}
	if len(order.blockedGroups) != 1 || order.blockedGroups[0] != "G2" {
		t.Fatalf("blocked groups = %v", order.blockedGroups)
	}
}

func TestRefundOnlyBeforeGroupShipment(t *testing.T) {
	db := afterSaleTestDB(t)
	order := &fakeOrderService{order: &pb.Order{
		OrderNo: "ORD-REFUND-EARLY", UserId: 1, Status: "ALLOCATED", PaidCent: 2000, RefundState: "NONE",
		Items: []*pb.OrderItem{
			{SkuId: 1, Quantity: 1, WarehouseId: 1, GroupNo: "G1"},
			{SkuId: 2, Quantity: 1, WarehouseId: 2, GroupNo: "G2"},
		},
		FulfillmentGroups: []*pb.OrderFulfillmentGroup{
			{GroupNo: "G1", WarehouseId: 1, Status: "ALLOCATED"},
			{GroupNo: "G2", WarehouseId: 2, Status: "ALLOCATED"},
		},
	}}
	svc := New(db, nil, &fakePaymentService{captured: 2000}, &fakeInventoryService{}, order,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := svc.CreateAfterSale(context.Background(), &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-REFUND-EARLY", UserId: 1, Type: typeRefundOnly, AmountCent: 1000, Reason: "unshipped",
		Items: []*pb.AfterSaleItem{{SkuId: 2, Quantity: 1, WarehouseId: 2, GroupNo: "G2"}},
	}); err != nil {
		t.Fatalf("refund-only before shipment must be accepted: %v", err)
	}
}
