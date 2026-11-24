package aftersales

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

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
	for _, table := range []string{"after_sale_items", "after_sale_restock_tasks", "after_sale_line_ledger",
		"after_sale_refund_breakdowns", "restock_exception_authorizations", "after_sale_freight_holds",
		"after_sale_legacy_amount_adjustments", "after_sale_refund_tasks", "after_sale_amount_versions",
		"after_sale_amount_ledger", "after_sale_logs", "after_sales", "outbox_events"} {
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
			source VARCHAR(24) NOT NULL DEFAULT 'USER',
			responsibility_party VARCHAR(32) NOT NULL DEFAULT 'UNDETERMINED',
			responsibility_source VARCHAR(32) NOT NULL DEFAULT 'SYSTEM',
			responsibility_evidence_ref VARCHAR(128) NOT NULL DEFAULT '',
			responsibility_reason_code VARCHAR(64) NOT NULL DEFAULT '',
			responsibility_determined_by VARCHAR(64) NOT NULL DEFAULT '',
			responsibility_determined_at DATETIME(3) NULL,
			return_carrier_code VARCHAR(32) NOT NULL DEFAULT '',
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_after_sales_no (after_sale_no),
			UNIQUE KEY uk_after_sales_client_request (user_id, client_request_id))`,
		`CREATE TABLE IF NOT EXISTS after_sale_items (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, after_sale_id BIGINT NOT NULL,
			sku_id BIGINT NOT NULL, quantity INT NOT NULL, warehouse_id BIGINT NOT NULL DEFAULT 1,
			group_no VARCHAR(64) NOT NULL DEFAULT 'G1', received_quantity INT NOT NULL DEFAULT 0,
			condition_code VARCHAR(16) NOT NULL DEFAULT 'SELLABLE',
			line_no INT NOT NULL DEFAULT 0, line_paid_cent_snapshot BIGINT NOT NULL DEFAULT 0,
			unit_paid_cent_snapshot BIGINT NOT NULL DEFAULT 0, pending_amount_cent BIGINT NOT NULL DEFAULT 0,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_after_sale_items_line (after_sale_id, line_no),
			KEY idx_after_sale_items_group (after_sale_id, sku_id, warehouse_id, group_no))`,
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
		`CREATE TABLE IF NOT EXISTS after_sale_line_ledger (
			order_no VARCHAR(64) NOT NULL, line_no INT NOT NULL,
			refunded_quantity BIGINT NOT NULL DEFAULT 0, refunded_amount_cent BIGINT NOT NULL DEFAULT 0,
			pending_quantity BIGINT NOT NULL DEFAULT 0, pending_amount_cent BIGINT NOT NULL DEFAULT 0,
			version BIGINT NOT NULL DEFAULT 1,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
			PRIMARY KEY (order_no, line_no))`,
		`CREATE TABLE IF NOT EXISTS after_sale_refund_breakdowns (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, after_sale_no VARCHAR(64) NOT NULL,
			goods_cent BIGINT NOT NULL DEFAULT 0, freight_cent BIGINT NOT NULL DEFAULT 0,
			total_cent BIGINT NOT NULL DEFAULT 0, rule_id VARCHAR(64) NOT NULL DEFAULT '',
			responsibility_party VARCHAR(32) NOT NULL DEFAULT 'UNDETERMINED',
			refund_idempotency_key VARCHAR(96) NOT NULL DEFAULT '',
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_after_sale_refund_breakdown (after_sale_no))`,
		`CREATE TABLE IF NOT EXISTS restock_exception_authorizations (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, after_sale_no VARCHAR(64) NOT NULL,
			sku_id BIGINT NOT NULL, from_warehouse_id BIGINT NOT NULL, to_warehouse_id BIGINT NOT NULL,
			operator VARCHAR(64) NOT NULL, reason VARCHAR(255) NOT NULL,
			auth_no VARCHAR(64) NOT NULL DEFAULT '', expires_at DATETIME(3) NULL, used_at DATETIME(3) NULL,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_restock_exception_authorization (after_sale_no, sku_id),
			UNIQUE KEY uk_restock_exception_auth_no (auth_no))`,
		`CREATE TABLE IF NOT EXISTS after_sale_freight_holds (
			after_sale_no VARCHAR(64) NOT NULL PRIMARY KEY, freight_cent BIGINT NOT NULL DEFAULT 0,
			status VARCHAR(16) NOT NULL DEFAULT 'HELD', deadline_at DATETIME(3) NOT NULL,
			determined_party VARCHAR(32) NOT NULL DEFAULT 'UNDETERMINED', determined_at DATETIME(3) NULL,
			version BIGINT NOT NULL DEFAULT 1,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3))`,
		`CREATE TABLE IF NOT EXISTS after_sale_legacy_amount_adjustments (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, after_sale_no VARCHAR(64) NOT NULL,
			operator VARCHAR(64) NOT NULL, reason VARCHAR(255) NOT NULL, amount_cent BIGINT NOT NULL,
			approved_by VARCHAR(64) NOT NULL DEFAULT '',
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3))`,
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
		"after_sale_amount_ledger", "after_sale_line_ledger", "after_sale_refund_breakdowns",
		"after_sale_freight_holds", "after_sale_legacy_amount_adjustments",
		"restock_exception_authorizations", "after_sale_items", "after_sale_logs", "after_sales", "outbox_events"} {
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
	// db lets a test prove the durable refund task exists before the remote
	// admission is called (T1.4b2b R-C ordering).
	db                *sql.DB
	taskStatusAtAdmit string
	sourceAtAdmit     string
	amountAtAdmit     int64
}

func (f *fakeOrderService) GetOrder(context.Context, *pb.GetOrderRequest, ...grpc.CallOption) (*pb.Order, error) {
	return f.order, nil
}

func (f *fakeOrderService) AdmitRefund(_ context.Context, in *pb.AdmitRefundRequest, _ ...grpc.CallOption) (*pb.AdmitRefundReply, error) {
	f.blockCalls++
	f.blocked = append(f.blocked, in.GetRefundId())
	f.blockedGroups = append(f.blockedGroups, in.GetGroupNo())
	f.sourceAtAdmit = in.GetSource()
	f.amountAtAdmit = in.GetAmountCent()
	if f.db != nil {
		_ = f.db.QueryRow(`SELECT status FROM after_sale_refund_tasks WHERE after_sale_no = ?`, in.GetRefundId()).
			Scan(&f.taskStatusAtAdmit)
	}
	result := "PRE_SHIP_REVOKED"
	if f.blockErr == nil && f.order != nil && f.order.GetStatus() == "SHIPPED" {
		result = "POST_SHIP_ALLOWED"
	}
	if in.GetSource() == zeroAfterSaleSource {
		return &pb.AdmitRefundReply{
			RefundId: in.GetRefundId(), Result: result, State: zeroRefundApplicationDone, AmountCent: 0,
		}, f.blockErr
	}
	return &pb.AdmitRefundReply{RefundId: in.GetRefundId(), Result: result, State: result}, f.blockErr
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
	if _, err := svc.Approve(context.Background(), &pb.ApproveRequest{AfterSaleNo: no, Approved: true, AmountCent: amount, Reason: "test adjustment"}); err != nil {
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

// AC-15: return-refund cannot pay before restock_state=DONE.
func TestAC15ReturnRefundBlocksUntilRestockDone(t *testing.T) {
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
	if order.blockCalls < 1 || payment.refundCalls != 1 || len(payment.refundAmount) != 1 || payment.refundAmount[0] != 600 {
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
	if len(order.blocked) == 0 || order.blocked[0] != as.GetAfterSaleNo() {
		t.Fatalf("blocked = %v", order.blocked)
	}
	if len(order.blockedGroups) == 0 || order.blockedGroups[0] != "G2" {
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

func lineAfterSaleService(t *testing.T, db *sql.DB, captured int64) (*Service, *fakePaymentService) {
	t.Helper()
	order := &fakeOrderService{order: &pb.Order{
		OrderNo: "ORD-LINE", UserId: 1, Status: "PAID", PaidCent: 1000, FreightCent: 0, RefundState: "NONE",
		FulfillmentGroups: []*pb.OrderFulfillmentGroup{
			{GroupNo: "G1", WarehouseId: 1, Status: "ALLOCATED"},
			{GroupNo: "G2", WarehouseId: 2, Status: "ALLOCATED"},
		},
		Items: []*pb.OrderItem{
			{LineNo: 1, SkuId: 1, PriceCent: 1000, Quantity: 3, WarehouseId: 1, GroupNo: "G1",
				LinePaidCent: 1000, UnitPaidCent: 333},
			{LineNo: 2, SkuId: 2, PriceCent: 500, Quantity: 1, WarehouseId: 2, GroupNo: "G2",
				LinePaidCent: 500, UnitPaidCent: 500},
		},
	}}
	payment := &fakePaymentService{captured: captured}
	inventory := &fakeInventoryService{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(db, nil, payment, inventory, order, logger), payment
}

// AC-10/AC-11/AC-13: the line ledger computes 333 then 667, ignores the
// caller's expected amount, and never touches the other warehouse line.
func TestAC10LineRefundCalculation(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, payment := lineAfterSaleService(t, db, 2500)
	ctx := context.Background()
	first, err := svc.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-LINE", UserId: 1, Type: typeRefundOnly, Reason: "partial", ExpectedAmountCent: 999,
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1, WarehouseId: 1, GroupNo: "G1", LineNo: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.GetAmountCent() != 999 || first.GetEffectiveAmountCent() != 333 {
		t.Fatalf("expected amount recorded=%d computed=%d", first.GetAmountCent(), first.GetEffectiveAmountCent())
	}
	if _, err := svc.Approve(ctx, &pb.ApproveRequest{AfterSaleNo: first.GetAfterSaleNo(), Approved: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refund(ctx, &pb.RefundAfterSaleRequest{AfterSaleNo: first.GetAfterSaleNo()}); err != nil {
		t.Fatal(err)
	}
	if len(payment.refundAmount) != 1 || payment.refundAmount[0] != 333 {
		t.Fatalf("first refunds = %v", payment.refundAmount)
	}
	var qty, amount int64
	if err := db.QueryRow(`SELECT refunded_quantity, refunded_amount_cent FROM after_sale_line_ledger
		WHERE order_no = 'ORD-LINE' AND line_no = 1`).Scan(&qty, &amount); err != nil {
		t.Fatal(err)
	}
	if qty != 1 || amount != 333 {
		t.Fatalf("ledger = %d/%d", qty, amount)
	}
	var other int
	if err := db.QueryRow(`SELECT COUNT(*) FROM after_sale_line_ledger WHERE order_no = 'ORD-LINE' AND line_no = 2`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if other != 0 {
		t.Fatalf("line 2 ledger was touched")
	}
	second, err := svc.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-LINE", UserId: 1, Type: typeRefundOnly, Reason: "rest",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 2, WarehouseId: 1, GroupNo: "G1", LineNo: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.GetEffectiveAmountCent() != 667 {
		t.Fatalf("second effective = %d", second.GetEffectiveAmountCent())
	}
	if _, err := svc.Approve(ctx, &pb.ApproveRequest{AfterSaleNo: second.GetAfterSaleNo(), Approved: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refund(ctx, &pb.RefundAfterSaleRequest{AfterSaleNo: second.GetAfterSaleNo()}); err != nil {
		t.Fatal(err)
	}
	if len(payment.refundAmount) != 2 || payment.refundAmount[1] != 667 {
		t.Fatalf("refunds = %v", payment.refundAmount)
	}
	var totalAmount int64
	if err := db.QueryRow(`SELECT refunded_amount_cent FROM after_sale_line_ledger
		WHERE order_no = 'ORD-LINE' AND line_no = 1`).Scan(&totalAmount); err != nil {
		t.Fatal(err)
	}
	if totalAmount != 1000 {
		t.Fatalf("line total refund = %d, want 1000", totalAmount)
	}
}

// AC-12: the order-level captured remainder caps the computed line amount.
func TestAC12RefundCappedByCapturedRemaining(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, payment := lineAfterSaleService(t, db, 300)
	ctx := context.Background()
	as, err := svc.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-LINE", UserId: 1, Type: typeRefundOnly, Reason: "cap",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1, WarehouseId: 1, GroupNo: "G1", LineNo: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if as.GetEffectiveAmountCent() != 300 {
		t.Fatalf("effective = %d, want captured cap 300", as.GetEffectiveAmountCent())
	}
	if _, err := svc.Approve(ctx, &pb.ApproveRequest{AfterSaleNo: as.GetAfterSaleNo(), Approved: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refund(ctx, &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()}); err != nil {
		t.Fatal(err)
	}
	if len(payment.refundAmount) != 1 || payment.refundAmount[0] != 300 {
		t.Fatalf("refunds = %v", payment.refundAmount)
	}
	var ledgerAmount int64
	if err := db.QueryRow(`SELECT refunded_amount_cent FROM after_sale_line_ledger
		WHERE order_no = 'ORD-LINE' AND line_no = 1`).Scan(&ledgerAmount); err != nil {
		t.Fatal(err)
	}
	if ledgerAmount != 300 {
		t.Fatalf("ledger recorded %d but only 300 was paid", ledgerAmount)
	}
}

func shortageOrderService(t *testing.T, db *sql.DB) (*Service, *fakePaymentService) {
	t.Helper()
	order := &fakeOrderService{order: &pb.Order{
		OrderNo: "ORD-SHORT", UserId: 1, Status: "ALLOCATED", PaidCent: 2000, FreightCent: 1000, RefundState: "NONE",
		FulfillmentGroups: []*pb.OrderFulfillmentGroup{
			{GroupNo: "G1", WarehouseId: 1, Status: "ALLOCATED", FreightShareCent: 1000},
		},
		Items: []*pb.OrderItem{
			{LineNo: 1, SkuId: 1, PriceCent: 500, Quantity: 2, WarehouseId: 1, GroupNo: "G1",
				LinePaidCent: 1000, UnitPaidCent: 500},
		},
	}}
	payment := &fakePaymentService{captured: 2000}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(db, nil, payment, &fakeInventoryService{}, order, logger), payment
}

// AC-30/E8: the shortage resolution creates a system refund with a durable task;
// a duplicate event reuses it and the worker actually pays.
func TestAC30ShortageCreatesSystemRefundAndPays(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, payment := shortageOrderService(t, db)
	event := shortageResolvedEvent{
		OrderNo: "ORD-SHORT", GroupNo: "G1", Status: shortageResolved, Resolution: "REFUND",
		ReasonCode: "SHORTAGE_DEFAULT", ResolvedAt: time.Now().Unix(),
		Items: []struct {
			LineNo   int32 `json:"line_no"`
			SkuID    int64 `json:"sku_id"`
			Quantity int32 `json:"quantity"`
		}{{LineNo: 1, SkuID: 1, Quantity: 2}},
	}
	ctx := context.Background()
	if err := svc.createSystemRefund(ctx, event); err != nil {
		t.Fatal(err)
	}
	if err := svc.createSystemRefund(ctx, event); err != nil {
		t.Fatal(err)
	}
	var (
		afterSaleNo, status, source, clientKey string
		effective                              int64
	)
	if err := db.QueryRow(`SELECT after_sale_no, status, source, client_request_id, effective_amount_cent
		FROM after_sales WHERE order_no = 'ORD-SHORT'`).
		Scan(&afterSaleNo, &status, &source, &clientKey, &effective); err != nil {
		t.Fatal(err)
	}
	if status != statusApproved || source != sourceSystem ||
		clientKey != "SYS-SHORTAGE:ORD-SHORT:G1" || effective != 2000 {
		t.Fatalf("system refund = %s/%s/%s/%d", status, source, clientKey, effective)
	}
	var tasks, afterSales int
	if err := db.QueryRow(`SELECT COUNT(*) FROM after_sale_refund_tasks`).Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM after_sales`).Scan(&afterSales); err != nil {
		t.Fatal(err)
	}
	if tasks != 1 || afterSales != 1 {
		t.Fatalf("tasks=%d after_sales=%d", tasks, afterSales)
	}
	if err := svc.processRefundTask(ctx, afterSaleNo); err != nil {
		t.Fatal(err)
	}
	if len(payment.refundAmount) != 1 || payment.refundAmount[0] != 2000 {
		t.Fatalf("payment refunds = %v", payment.refundAmount)
	}
	if err := db.QueryRow(`SELECT status FROM after_sales WHERE after_sale_no = ?`, afterSaleNo).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusRefunded {
		t.Fatalf("status = %s", status)
	}
	var ledgerQty, ledgerAmount, pending int64
	if err := db.QueryRow(`SELECT refunded_quantity, refunded_amount_cent, pending_quantity
		FROM after_sale_line_ledger WHERE order_no = 'ORD-SHORT' AND line_no = 1`).
		Scan(&ledgerQty, &ledgerAmount, &pending); err != nil {
		t.Fatal(err)
	}
	if ledgerQty != 2 || ledgerAmount != 1000 || pending != 0 {
		t.Fatalf("ledger = %d/%d pending=%d", ledgerQty, ledgerAmount, pending)
	}
	if err := svc.createSystemRefund(ctx, event); err != nil {
		t.Fatal(err)
	}
}

func freightTestOrder(groups []*pb.OrderFulfillmentGroup) *pb.Order {
	return &pb.Order{
		OrderNo: "ORD-FREIGHT", UserId: 1, Status: "PAID", PaidCent: 2000, FreightCent: 1000,
		FulfillmentGroups: groups,
	}
}

// AC-14: six freight-refund scenarios from SPEC §6.5.1.
func TestAC14FreightRefundSixScenarios(t *testing.T) {
	db := afterSaleTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := New(db, nil, &fakePaymentService{captured: 2000}, &fakeInventoryService{}, nil, logger)
	ctx := context.Background()
	open := []*pb.OrderFulfillmentGroup{
		{GroupNo: "G1", WarehouseId: 1, Status: "ALLOCATED", RefundState: "NONE", FreightShareCent: 300},
		{GroupNo: "G2", WarehouseId: 2, Status: "ALLOCATED", RefundState: "NONE", FreightShareCent: 700},
	}
	closed := []*pb.OrderFulfillmentGroup{
		{GroupNo: "G1", WarehouseId: 1, Status: "ALLOCATED", RefundState: "NONE", FreightShareCent: 300},
		{GroupNo: "G2", WarehouseId: 2, Status: "CANCELLED", RefundState: "REFUNDED", FreightShareCent: 700},
	}
	single := []*pb.OrderFulfillmentGroup{
		{GroupNo: "G1", WarehouseId: 1, Status: "ALLOCATED", RefundState: "NONE", FreightShareCent: 1000},
	}
	cases := []struct {
		name   string
		groups []*pb.OrderFulfillmentGroup
		party  string
		amount int64
		hold   bool
		rule   string
	}{
		{"unshipped cancel", single, partyMerchant, 1000, false, "MERCHANT_FULL_FREIGHT"},
		{"merchant full order", closed, partyMerchant, 1000, false, "MERCHANT_FULL_FREIGHT"},
		{"merchant partial group", open, partyMerchant, 300, false, "SHORTAGE_MERCHANT_FREIGHT_SHARE"},
		{"customer no reason", open, partyCustomer, 0, false, "CUSTOMER_NO_FREIGHT"},
		{"partial quantity customer", single, partyCustomer, 0, false, "CUSTOMER_NO_FREIGHT"},
		{"platform partial group", open, partyPlatform, 300, false, "SHORTAGE_MERCHANT_FREIGHT_SHARE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			order := freightTestOrder(tc.groups)
			amount, hold, rule, err := svc.freightRefundFor(ctx, db, order, "G1", tc.party)
			if err != nil {
				t.Fatal(err)
			}
			if amount != tc.amount || hold != tc.hold || rule != tc.rule {
				t.Fatalf("got amount=%d hold=%v rule=%s", amount, hold, rule)
			}
		})
	}
	// Undetermined is a hold, never an immediate refund.
	amount, hold, rule, err := svc.freightRefundFor(ctx, db, freightTestOrder(open), "G1", partyUndetermined)
	if err != nil || amount != 300 || !hold || rule != "FREIGHT_HOLD" {
		t.Fatalf("undetermined = %d/%v/%s/%v", amount, hold, rule, err)
	}
	// Cumulative cap: a prior refunded breakdown consumes 700 of the 1000
	// order freight; a later full-order refund can only return the remainder.
	if _, err := db.Exec(`INSERT INTO after_sales
		(after_sale_no, order_no, user_id, ` + "`type`" + `, status, amount_cent, effective_amount_cent,
		 amount_state, restock_state, reason, client_request_id, source)
		VALUES ('AS-PRIOR', 'ORD-FREIGHT', 1, 'REFUND_ONLY', 'REFUNDED', 700, 700,
		 'CONSUMED', 'NONE', 'prior', 'prior', 'SYSTEM')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO after_sale_refund_breakdowns
		(after_sale_no, goods_cent, freight_cent, total_cent, rule_id)
		VALUES ('AS-PRIOR', 0, 700, 700, 'MERCHANT_FULL_FREIGHT')`); err != nil {
		t.Fatal(err)
	}
	amount, hold, rule, err = svc.freightRefundFor(ctx, db, freightTestOrder(closed), "G1", partyMerchant)
	if err != nil || amount != 300 || hold || rule != "MERCHANT_FULL_FREIGHT" {
		t.Fatalf("cumulative cap = %d/%v/%s/%v", amount, hold, rule, err)
	}
	// A HELD commitment also consumes the order freight budget before payout.
	if _, err := db.Exec(`DELETE FROM after_sale_refund_breakdowns WHERE after_sale_no = 'AS-PRIOR'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO after_sales
		(after_sale_no, order_no, user_id, ` + "`type`" + `, status, amount_cent, effective_amount_cent,
		 amount_state, restock_state, reason, client_request_id, source)
		VALUES ('AS-HOLD', 'ORD-FREIGHT', 1, 'RETURN_REFUND', 'REFUNDED', 1000, 1000,
		 'CONSUMED', 'DONE', 'hold', 'hold', 'USER')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO after_sale_freight_holds
		(after_sale_no, freight_cent, status, deadline_at) VALUES ('AS-HOLD', 700, 'HELD', NOW(3))`); err != nil {
		t.Fatal(err)
	}
	amount, hold, rule, err = svc.freightRefundFor(ctx, db, freightTestOrder(closed), "G1", partyMerchant)
	if err != nil || amount != 300 || hold || rule != "MERCHANT_FULL_FREIGHT" {
		t.Fatalf("held-freight cap = %d/%v/%s/%v", amount, hold, rule, err)
	}
}

// AC-31/32/33: undetermined responsibility refunds goods, holds freight for
// 168h, then the timeout worker refunds it as merchant responsibility. The
// free-text reason never becomes a carrier determination.
// AC-31/AC-32/AC-33: goods refund is not blocked by an undetermined
// responsibility; freight is held for 168h, then defaults to merchant; the
// free-text reason never becomes a carrier determination.
func testAC31To33FreightHoldTimeout(t *testing.T) {
	db := afterSaleTestDB(t)
	order := &fakeOrderService{order: &pb.Order{
		OrderNo: "ORD-HOLD", UserId: 1, Status: "SHIPPED", PaidCent: 2000, FreightCent: 1000, RefundState: "NONE",
		FulfillmentGroups: []*pb.OrderFulfillmentGroup{
			{GroupNo: "G1", WarehouseId: 1, Status: "SHIPPED", RefundState: "NONE", FreightShareCent: 500},
		},
		Items: []*pb.OrderItem{
			{LineNo: 1, SkuId: 1, PriceCent: 1000, Quantity: 1, WarehouseId: 1, GroupNo: "G1",
				LinePaidCent: 1000, UnitPaidCent: 1000},
		},
	}}
	payment := &fakePaymentService{captured: 2000}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewWithOptions(db, nil, payment, &fakeInventoryService{}, order, logger, Options{FreightResponsibilityEnabled: true})
	ctx := context.Background()
	as, err := svc.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-HOLD", UserId: 1, Type: typeReturnRefund, Reason: "包裹破损",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1, WarehouseId: 1, GroupNo: "G1", LineNo: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if as.GetResponsibilityParty() != partyUndetermined || as.GetResponsibilityReasonCode() != "PENDING_REVIEW" {
		t.Fatalf("free-text reason determined responsibility: %+v", as)
	}
	if as.GetFreightState() != freightStateHeld || as.GetFreightCent() != 500 || as.GetFreightDeadlineUnix() == 0 {
		t.Fatalf("freight hold = %+v", as)
	}
	if _, err := db.Exec(`UPDATE after_sales SET status = ?, restock_state = ? WHERE after_sale_no = ?`,
		statusReturnReceived, restockDone, as.GetAfterSaleNo()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refund(ctx, &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()}); err != nil {
		t.Fatal(err)
	}
	if len(payment.refundAmount) != 1 || payment.refundAmount[0] != 1000 {
		t.Fatalf("goods refunds = %v", payment.refundAmount)
	}
	var state, status string
	if err := db.QueryRow(`SELECT status FROM after_sale_freight_holds WHERE after_sale_no = ?`, as.GetAfterSaleNo()).
		Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != freightStateHeld {
		t.Fatalf("hold state = %s", state)
	}
	if err := db.QueryRow(`SELECT status FROM after_sales WHERE after_sale_no = ?`, as.GetAfterSaleNo()).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusRefunded {
		t.Fatalf("after sale status = %s", status)
	}
	if _, err := db.Exec(`UPDATE after_sale_freight_holds SET deadline_at = DATE_SUB(NOW(3), INTERVAL 1 MINUTE)
		WHERE after_sale_no = ?`, as.GetAfterSaleNo()); err != nil {
		t.Fatal(err)
	}
	svc.processExpiredFreightHolds(ctx, logger)
	if len(payment.refundAmount) != 2 || payment.refundAmount[1] != 500 {
		t.Fatalf("freight refunds = %v", payment.refundAmount)
	}
	var rule, responsibility string
	if err := db.QueryRow(`SELECT status, determined_party FROM after_sale_freight_holds WHERE after_sale_no = ?`,
		as.GetAfterSaleNo()).Scan(&state, &responsibility); err != nil {
		t.Fatal(err)
	}
	if state != freightStateRefunded || responsibility != partyMerchant {
		t.Fatalf("hold final = %s/%s", state, responsibility)
	}
	if err := db.QueryRow(`SELECT rule_id FROM after_sale_refund_breakdowns WHERE after_sale_no = ?`,
		as.GetAfterSaleNo()).Scan(&rule); err != nil {
		t.Fatal(err)
	}
	if rule != "FREIGHT_HOLD_TIMEOUT_MERCHANT" {
		t.Fatalf("rule = %s", rule)
	}
}

// AC-11: the caller's expected amount never changes the computed/payed amount.
func TestAC11ExpectedAmountDoesNotAffectRefund(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, payment := lineAfterSaleService(t, db, 2500)
	ctx := context.Background()
	as, err := svc.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-LINE", UserId: 1, Type: typeRefundOnly, Reason: "expected only",
		ExpectedAmountCent: 999,
		Items:              []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1, WarehouseId: 1, GroupNo: "G1", LineNo: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if as.GetAmountCent() != 999 || as.GetEffectiveAmountCent() != 333 {
		t.Fatalf("expected=%d computed=%d", as.GetAmountCent(), as.GetEffectiveAmountCent())
	}
	if _, err := svc.Approve(ctx, &pb.ApproveRequest{AfterSaleNo: as.GetAfterSaleNo(), Approved: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refund(ctx, &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()}); err != nil {
		t.Fatal(err)
	}
	if len(payment.refundAmount) != 1 || payment.refundAmount[0] != 333 {
		t.Fatalf("paid = %v", payment.refundAmount)
	}
}

// AC-13: refunding warehouse 1's line never changes warehouse 2's line ledger.
func TestAC13CrossWarehouseLineIsolation(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, _ := lineAfterSaleService(t, db, 2500)
	ctx := context.Background()
	as, err := svc.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-LINE", UserId: 1, Type: typeRefundOnly, Reason: "line 1",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1, WarehouseId: 1, GroupNo: "G1", LineNo: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Approve(ctx, &pb.ApproveRequest{AfterSaleNo: as.GetAfterSaleNo(), Approved: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refund(ctx, &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()}); err != nil {
		t.Fatal(err)
	}
	var other int
	if err := db.QueryRow(`SELECT COUNT(*) FROM after_sale_line_ledger
		WHERE order_no = 'ORD-LINE' AND line_no = 2`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if other != 0 {
		t.Fatalf("line 2 ledger was touched")
	}
}

func TestAC31FreightHoldTimeout(t *testing.T)                   { testAC31To33FreightHoldTimeout(t) }
func TestAC32FreightHoldTimeoutDefaultsToMerchant(t *testing.T) { testAC31To33FreightHoldTimeout(t) }
func TestAC33FreeTextDoesNotDetermineCarrier(t *testing.T)      { testAC31To33FreightHoldTimeout(t) }

// P0-2: two concurrent after-sales for 2 units on a 3-unit line cannot both
// reserve the line; pending quantity/amount are cleared on completion.
func TestLinePendingReservationBlocksConcurrentOverRefund(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, _ := lineAfterSaleService(t, db, 2500)
	ctx := context.Background()
	results := make([]error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, results[index] = svc.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
				OrderNo: "ORD-LINE", UserId: 1, Type: typeRefundOnly, Reason: "concurrent",
				Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 2, WarehouseId: 1, GroupNo: "G1", LineNo: 1}},
			})
		}(i)
	}
	wg.Wait()
	success := 0
	for _, err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("concurrent creates succeeded=%d errors=%v", success, results)
	}
	var pendingQty, pendingCent int64
	if err := db.QueryRow(`SELECT pending_quantity, pending_amount_cent FROM after_sale_line_ledger
		WHERE order_no = 'ORD-LINE' AND line_no = 1`).Scan(&pendingQty, &pendingCent); err != nil {
		t.Fatal(err)
	}
	if pendingQty != 2 || pendingCent != 666 {
		t.Fatalf("pending = %d/%d", pendingQty, pendingCent)
	}
	var afterSaleNo string
	if err := db.QueryRow(`SELECT after_sale_no FROM after_sales WHERE order_no = 'ORD-LINE'`).Scan(&afterSaleNo); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Approve(ctx, &pb.ApproveRequest{AfterSaleNo: afterSaleNo, Approved: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refund(ctx, &pb.RefundAfterSaleRequest{AfterSaleNo: afterSaleNo}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT pending_quantity, refunded_quantity FROM after_sale_line_ledger
		WHERE order_no = 'ORD-LINE' AND line_no = 1`).Scan(&pendingQty, &pendingCent); err != nil {
		t.Fatal(err)
	}
	if pendingQty != 0 || pendingCent != 2 {
		t.Fatalf("pending/refunded = %d/%d", pendingQty, pendingCent)
	}
}

// P0-3: with FREIGHT_RESPONSIBILITY_ENABLED=false the legacy caliber applies:
// unshipped REFUND_ONLY returns the full freight and creates no hold.
func TestFreightResponsibilityDisabledUsesLegacyRule(t *testing.T) {
	db := afterSaleTestDB(t)
	order := &fakeOrderService{order: &pb.Order{
		OrderNo: "ORD-FLAG", UserId: 1, Status: "PAID", PaidCent: 2000, FreightCent: 1000, RefundState: "NONE",
		FulfillmentGroups: []*pb.OrderFulfillmentGroup{
			{GroupNo: "G1", WarehouseId: 1, Status: "ALLOCATED", RefundState: "NONE", FreightShareCent: 1000},
		},
		Items: []*pb.OrderItem{
			{LineNo: 1, SkuId: 1, PriceCent: 1000, Quantity: 1, WarehouseId: 1, GroupNo: "G1",
				LinePaidCent: 1000, UnitPaidCent: 1000},
		},
	}}
	svc := New(db, nil, &fakePaymentService{captured: 2000}, &fakeInventoryService{}, order,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	as, err := svc.CreateAfterSale(context.Background(), &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-FLAG", UserId: 1, Type: typeRefundOnly, Reason: "unshipped",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1, WarehouseId: 1, GroupNo: "G1", LineNo: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if as.GetEffectiveAmountCent() != 2000 || as.GetFreightState() != "NONE" || as.GetFreightCent() != 0 {
		t.Fatalf("legacy refund = %+v", as)
	}
}

// P0-2: rejecting an after-sale releases its line pending quantity/amount.
func TestRejectedAfterSaleReleasesLinePending(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, _ := lineAfterSaleService(t, db, 2500)
	ctx := context.Background()
	as, err := svc.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-LINE", UserId: 1, Type: typeRefundOnly, Reason: "reject",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1, WarehouseId: 1, GroupNo: "G1", LineNo: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Approve(ctx, &pb.ApproveRequest{AfterSaleNo: as.GetAfterSaleNo(), Approved: false, Reason: "no"}); err != nil {
		t.Fatal(err)
	}
	var pendingQty, pendingCent int64
	if err := db.QueryRow(`SELECT pending_quantity, pending_amount_cent FROM after_sale_line_ledger
		WHERE order_no = 'ORD-LINE' AND line_no = 1`).Scan(&pendingQty, &pendingCent); err != nil {
		t.Fatal(err)
	}
	if pendingQty != 0 || pendingCent != 0 {
		t.Fatalf("pending after reject = %d/%d", pendingQty, pendingCent)
	}
}

// P0-2: a permanently failed refund task releases line pending and order
// amount reservations so the quota is not leaked.
func TestPermanentRefundFailureReleasesReservations(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, _ := lineAfterSaleService(t, db, 2500)
	ctx := context.Background()
	as, err := svc.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-LINE", UserId: 1, Type: typeRefundOnly, Reason: "permanent failure",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1, WarehouseId: 1, GroupNo: "G1", LineNo: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Approve(ctx, &pb.ApproveRequest{AfterSaleNo: as.GetAfterSaleNo(), Approved: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE after_sales SET status = ? WHERE after_sale_no = ?`, statusRefunding, as.GetAfterSaleNo()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO after_sale_refund_tasks
		(after_sale_no, status, attempts, last_error, lease_owner, lease_until)
		VALUES (?, 'PROCESSING', ?, '', 'owner', DATE_ADD(NOW(3), INTERVAL 1 MINUTE))`,
		as.GetAfterSaleNo(), maxTaskAttempts); err != nil {
		t.Fatal(err)
	}
	svc.releaseRefundTask(ctx, as.GetAfterSaleNo(), "owner", errors.New("permanent"))
	var pendingQty, pendingCent, consumed int64
	if err := db.QueryRow(`SELECT pending_quantity, pending_amount_cent FROM after_sale_line_ledger
		WHERE order_no = 'ORD-LINE' AND line_no = 1`).Scan(&pendingQty, &pendingCent); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT consumed_cent FROM after_sale_amount_ledger WHERE order_no = 'ORD-LINE'`).Scan(&consumed); err != nil {
		t.Fatal(err)
	}
	var amountState string
	if err := db.QueryRow(`SELECT amount_state FROM after_sales WHERE after_sale_no = ?`, as.GetAfterSaleNo()).Scan(&amountState); err != nil {
		t.Fatal(err)
	}
	if pendingQty != 0 || pendingCent != 0 || consumed != 0 || amountState != amountReleased {
		t.Fatalf("leak pending=%d/%d consumed=%d state=%s", pendingQty, pendingCent, consumed, amountState)
	}
}

func holdAfterSaleService(t *testing.T, db *sql.DB) (*Service, *fakePaymentService) {
	t.Helper()
	order := &fakeOrderService{order: &pb.Order{
		OrderNo: "ORD-HOLD7", UserId: 1, Status: "SHIPPED", PaidCent: 2000, FreightCent: 1000, RefundState: "NONE",
		FulfillmentGroups: []*pb.OrderFulfillmentGroup{
			{GroupNo: "G1", WarehouseId: 1, Status: "SHIPPED", RefundState: "NONE", FreightShareCent: 500},
		},
		Items: []*pb.OrderItem{
			{LineNo: 1, SkuId: 1, PriceCent: 1000, Quantity: 1, WarehouseId: 1, GroupNo: "G1",
				LinePaidCent: 1000, UnitPaidCent: 1000},
		},
	}}
	payment := &fakePaymentService{captured: 2000}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewWithOptions(db, nil, payment, &fakeInventoryService{}, order, logger, Options{FreightResponsibilityEnabled: true})
	return svc, payment
}

func createHoldAfterSale(t *testing.T, svc *Service) *pb.AfterSale {
	t.Helper()
	as, err := svc.CreateAfterSale(context.Background(), &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-HOLD7", UserId: 1, Type: typeReturnRefund, Reason: "包裹破损",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1, WarehouseId: 1, GroupNo: "G1", LineNo: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return as
}

// P0-7: a rejected after-sale waives its freight hold and the worker must not
// pay freight without a product refund.
func TestFreightHoldNotPaidForRejectedAfterSale(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, payment := holdAfterSaleService(t, db)
	as := createHoldAfterSale(t, svc)
	if _, err := svc.Approve(context.Background(), &pb.ApproveRequest{
		AfterSaleNo: as.GetAfterSaleNo(), Approved: false, Reason: "rejected",
	}); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRow(`SELECT status FROM after_sale_freight_holds WHERE after_sale_no = ?`, as.GetAfterSaleNo()).
		Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != freightStateWaived {
		t.Fatalf("hold after reject = %s", state)
	}
	if _, err := db.Exec(`UPDATE after_sale_freight_holds SET deadline_at = DATE_SUB(NOW(3), INTERVAL 1 MINUTE)
		WHERE after_sale_no = ?`, as.GetAfterSaleNo()); err != nil {
		t.Fatal(err)
	}
	svc.processExpiredFreightHolds(context.Background(), nil)
	if len(payment.refundAmount) != 0 {
		t.Fatalf("rejected after-sale paid freight: %v", payment.refundAmount)
	}
}

// P0-7: determination before the goods refund must not pay freight early; after
// the goods refund the worker pays it and records it in the breakdown.
func TestFreightHoldBeforeGoodsRefundIsRecordedInBreakdown(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, payment := holdAfterSaleService(t, db)
	as := createHoldAfterSale(t, svc)
	if _, err := svc.DetermineResponsibility(context.Background(), &pb.DetermineResponsibilityRequest{
		AfterSaleNo: as.GetAfterSaleNo(), Party: partyMerchant, Source: sourceAgent,
		EvidenceRef: "photo-1", ReasonCode: "OUT_OF_STOCK", Operator: "cs-1",
	}); err != nil {
		t.Fatal(err)
	}
	if len(payment.refundAmount) != 0 {
		t.Fatalf("freight paid before goods refund: %v", payment.refundAmount)
	}
	if _, err := db.Exec(`UPDATE after_sales SET status = ?, restock_state = ? WHERE after_sale_no = ?`,
		statusReturnReceived, restockDone, as.GetAfterSaleNo()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refund(context.Background(), &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()}); err != nil {
		t.Fatal(err)
	}
	svc.processExpiredFreightHolds(context.Background(), nil)
	if len(payment.refundAmount) != 2 {
		t.Fatalf("refund calls = %v", payment.refundAmount)
	}
	var freight, total int64
	if err := db.QueryRow(`SELECT freight_cent, total_cent FROM after_sale_refund_breakdowns
		WHERE after_sale_no = ?`, as.GetAfterSaleNo()).Scan(&freight, &total); err != nil {
		t.Fatal(err)
	}
	if freight != 500 || total != 1500 {
		t.Fatalf("breakdown = freight %d total %d", freight, total)
	}
	var state string
	if err := db.QueryRow(`SELECT status FROM after_sale_freight_holds WHERE after_sale_no = ?`, as.GetAfterSaleNo()).
		Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != freightStateRefunded {
		t.Fatalf("hold = %s", state)
	}
}
