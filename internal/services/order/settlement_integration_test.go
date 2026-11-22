package order

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
)

func orderTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run order settlement integration tests")
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
	for _, table := range []string{"order_shipping_permits", "order_fulfillment_groups", "order_items"} {
		if _, err := db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			t.Fatal(err)
		}
	}
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			order_no VARCHAR(64) NOT NULL, user_id BIGINT NOT NULL,
			status VARCHAR(32) NOT NULL, total_cent BIGINT NOT NULL DEFAULT 0,
			discount_cent BIGINT NOT NULL DEFAULT 0, promotion_discount_cent BIGINT NOT NULL DEFAULT 0,
			applied_promotions_json TEXT NOT NULL, freight_cent BIGINT NOT NULL DEFAULT 0,
			pay_cent BIGINT NOT NULL DEFAULT 0, paid_cent BIGINT NOT NULL DEFAULT 0,
			refunded_cent BIGINT NOT NULL DEFAULT 0, refund_status VARCHAR(32) NOT NULL DEFAULT '',
			refund_state VARCHAR(16) NOT NULL DEFAULT 'NONE',
			receiver_name VARCHAR(64) NOT NULL DEFAULT '', receiver_phone VARCHAR(32) NOT NULL DEFAULT '',
			receiver_address VARCHAR(255) NOT NULL DEFAULT '', quote_id VARCHAR(64) NOT NULL DEFAULT '',
			client_request_id VARCHAR(64) NOT NULL DEFAULT '', expire_at DATETIME(3) NOT NULL,
			version BIGINT NOT NULL DEFAULT 1,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_orders_order_no (order_no))`,
		`CREATE TABLE IF NOT EXISTS order_items (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, order_no VARCHAR(64) NOT NULL,
			line_no INT NOT NULL DEFAULT 0, sku_id BIGINT NOT NULL, name VARCHAR(255) NOT NULL DEFAULT '',
			price_cent BIGINT NOT NULL DEFAULT 0, quantity INT NOT NULL DEFAULT 0,
			warehouse_id BIGINT NOT NULL DEFAULT 1, fulfillment_group_no VARCHAR(64) NOT NULL DEFAULT 'G1',
			discount_cent BIGINT NOT NULL DEFAULT 0,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_order_items_line (order_no, line_no))`,
		`CREATE TABLE IF NOT EXISTS order_fulfillment_groups (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, group_no VARCHAR(64) NOT NULL,
			order_no VARCHAR(64) NOT NULL, seq INT NOT NULL DEFAULT 1, warehouse_id BIGINT NOT NULL DEFAULT 1,
			status VARCHAR(24) NOT NULL DEFAULT 'PENDING', refund_state VARCHAR(16) NOT NULL DEFAULT 'NONE',
			reservation_state VARCHAR(16) NOT NULL DEFAULT 'HELD',
			freight_cent BIGINT NOT NULL DEFAULT 0, version BIGINT NOT NULL DEFAULT 1,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_order_fulfillment_group_no (order_no, group_no),
			UNIQUE KEY uk_order_fulfillment_group_seq (order_no, seq))`,
		`CREATE TABLE IF NOT EXISTS order_status_logs (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, order_no VARCHAR(64) NOT NULL,
			from_status VARCHAR(32) NOT NULL DEFAULT '', to_status VARCHAR(32) NOT NULL,
			reason VARCHAR(255) NOT NULL DEFAULT '', operator VARCHAR(64) NOT NULL DEFAULT '',
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3))`,
		`CREATE TABLE IF NOT EXISTS order_shipping_permits (
			order_no VARCHAR(64) NOT NULL, fulfillment_group_no VARCHAR(64) NOT NULL DEFAULT 'G1',
			state VARCHAR(16) NOT NULL DEFAULT 'NONE',
			shipment_no VARCHAR(64) NOT NULL DEFAULT '', after_sale_no VARCHAR(64) NOT NULL DEFAULT '',
			version BIGINT NOT NULL DEFAULT 1,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
			PRIMARY KEY (order_no, fulfillment_group_no))`,
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
			t.Fatalf("create order test table: %v", err)
		}
	}
	for _, table := range []string{"order_fulfillment_groups", "order_items", "order_shipping_permits", "order_status_logs", "orders", "outbox_events"} {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func seedSettlementOrder(t *testing.T, db *sql.DB, orderNo, status string, paid, refunded int64, refundState string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no, user_id, status, total_cent, discount_cent, promotion_discount_cent,
		 applied_promotions_json, freight_cent, pay_cent, paid_cent, refunded_cent,
		 refund_status, refund_state, receiver_name, receiver_phone, receiver_address,
		 quote_id, client_request_id, expire_at, version)
		VALUES (?, 1, ?, ?, 0, 0, '', 0, ?, ?, ?, '', ?, 'R', '13800000000', 'Shanghai', 'Q', '', DATE_ADD(NOW(3), INTERVAL 1 HOUR), 1)`,
		orderNo, status, paid, paid, paid, refunded, refundState); err != nil {
		t.Fatalf("seed order: %v", err)
	}
}

func settlementService(db *sql.DB) *Service {
	return &Service{db: db, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestShippingPermitAndRefundRace(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	ctx := context.Background()
	for i := 0; i < 8; i++ {
		orderNo := "ORD-PERMIT-" + string(rune('A'+i))
		seedSettlementOrder(t, db, orderNo, statusPaid, 1000, 0, refundStateNone)
		var authErr, blockErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, authErr = svc.AuthorizeShipment(ctx, &pb.ShipmentPermitRequest{OrderNo: orderNo, ShipmentNo: "SHIP-" + orderNo})
		}()
		go func() {
			defer wg.Done()
			_, blockErr = svc.BlockShipmentForRefund(ctx, &pb.ShipmentPermitRequest{OrderNo: orderNo, AfterSaleNo: "AS-" + orderNo})
		}()
		wg.Wait()
		if blockErr != nil {
			t.Fatalf("%s refund block failed: %v", orderNo, blockErr)
		}
		var permitState, refundState string
		if err := db.QueryRow(`SELECT p.state, o.refund_state FROM order_shipping_permits p
			JOIN orders o ON o.order_no = p.order_no WHERE p.order_no = ?`, orderNo).
			Scan(&permitState, &refundState); err != nil {
			t.Fatal(err)
		}
		if refundState != refundStateRefunding {
			t.Fatalf("%s refund_state = %s", orderNo, refundState)
		}
		if authErr == nil && permitState != permitConsumed {
			t.Fatalf("%s authorize succeeded but permit = %s", orderNo, permitState)
		}
		if authErr != nil && permitState != permitRevoked {
			t.Fatalf("%s authorize failed but permit = %s", orderNo, permitState)
		}
	}
}

func TestBlockShipmentRevokesUnusedPermit(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	ctx := context.Background()
	seedSettlementOrder(t, db, "ORD-BLOCK", statusPaid, 1000, 0, refundStateNone)
	if _, err := svc.BlockShipmentForRefund(ctx, &pb.ShipmentPermitRequest{OrderNo: "ORD-BLOCK", AfterSaleNo: "AS-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AuthorizeShipment(ctx, &pb.ShipmentPermitRequest{OrderNo: "ORD-BLOCK", ShipmentNo: "SHIP-1"}); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("authorize after block = %v", err)
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits WHERE order_no = 'ORD-BLOCK'`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != permitRevoked {
		t.Fatalf("permit = %s", state)
	}
}

func TestAuthorizeShipmentIsIdempotentPerShipment(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	ctx := context.Background()
	seedSettlementOrder(t, db, "ORD-AUTH", statusPaid, 1000, 0, refundStateNone)
	for i := 0; i < 2; i++ {
		if _, err := svc.AuthorizeShipment(ctx, &pb.ShipmentPermitRequest{OrderNo: "ORD-AUTH", ShipmentNo: "SHIP-1"}); err != nil {
			t.Fatalf("authorize %d: %v", i, err)
		}
	}
	if _, err := svc.AuthorizeShipment(ctx, &pb.ShipmentPermitRequest{OrderNo: "ORD-AUTH", ShipmentNo: "SHIP-2"}); !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("second shipment = %v", err)
	}
}

func TestAuthorizeShipmentAllowsOtherGroupWhileOneRefunding(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	ctx := context.Background()
	seedSettlementOrder(t, db, "ORD-REFUNDING", statusAllocated, 1000, 0, refundStateRefunding)
	if _, err := svc.AuthorizeShipment(ctx, &pb.ShipmentPermitRequest{
		OrderNo: "ORD-REFUNDING", ShipmentNo: "SHIP-OTHER", FulfillmentGroupNo: "G1",
	}); err != nil {
		t.Fatalf("another group must remain shippable while one group refunds: %v", err)
	}
}

func TestAuthorizeShipmentDeniedAfterFullRefund(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	ctx := context.Background()
	seedSettlementOrder(t, db, "ORD-FULL", statusPaid, 1000, 1000, refundStateFull)
	if _, err := svc.AuthorizeShipment(ctx, &pb.ShipmentPermitRequest{OrderNo: "ORD-FULL", ShipmentNo: "SHIP-1"}); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("authorize after full refund = %v", err)
	}
}

func TestRefundProjectionKeepsLifecycle(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	ctx := context.Background()
	seedSettlementOrder(t, db, "ORD-PROJ", statusShipped, 1000, 0, refundStateNone)
	if err := applyPaymentRefunded(ctx, svc, paymentRefundedEvent{
		PaymentNo: "PAY-1", OrderNo: "ORD-PROJ", AmountCent: 400, RefundedCent: 400, CapturedCent: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	var status, refundStatus, refundState string
	var refunded, paid int64
	if err := db.QueryRow(`SELECT status, refund_status, refund_state, refunded_cent, paid_cent FROM orders WHERE order_no = 'ORD-PROJ'`).
		Scan(&status, &refundStatus, &refundState, &refunded, &paid); err != nil {
		t.Fatal(err)
	}
	if status != statusShipped || refundStatus != statusPartiallyRefunded || refundState != refundStatePartial || refunded != 400 || paid != 1000 {
		t.Fatalf("partial projection = %s/%s/%s/%d/%d", status, refundStatus, refundState, refunded, paid)
	}
	if err := applyPaymentRefunded(ctx, svc, paymentRefundedEvent{
		PaymentNo: "PAY-1", OrderNo: "ORD-PROJ", AmountCent: 600, RefundedCent: 1000, CapturedCent: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status, refund_status, refund_state FROM orders WHERE order_no = 'ORD-PROJ'`).
		Scan(&status, &refundStatus, &refundState); err != nil {
		t.Fatal(err)
	}
	if status != statusShipped || refundStatus != statusRefunded || refundState != refundStateFull {
		t.Fatalf("full projection = %s/%s/%s", status, refundStatus, refundState)
	}
	// Shipment projection still works after a refund: lifecycle and refund are
	// orthogonal.
	if err := svc.applyShipmentStatus(ctx, "ORD-PROJ", statusCompleted, statusShipped); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status, refund_state FROM orders WHERE order_no = 'ORD-PROJ'`).Scan(&status, &refundState); err != nil {
		t.Fatal(err)
	}
	if status != statusCompleted || refundState != refundStateFull {
		t.Fatalf("completed = %s refund=%s", status, refundState)
	}
}

func TestRefundProjectionUsesOrderAggregate(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	ctx := context.Background()
	seedSettlementOrder(t, db, "ORD-PROJ-MULTI", statusShipped, 1000, 0, refundStateNone)
	if err := applyPaymentRefunded(ctx, svc, paymentRefundedEvent{
		PaymentNo: "PAY-1", OrderNo: "ORD-PROJ-MULTI", AmountCent: 100,
		RefundedCent: 100, OrderRefundedCent: 300, CapturedCent: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	var refunded int64
	if err := db.QueryRow(`SELECT refunded_cent FROM orders WHERE order_no='ORD-PROJ-MULTI'`).Scan(&refunded); err != nil {
		t.Fatal(err)
	}
	if refunded != 300 {
		t.Fatalf("order aggregate = %d", refunded)
	}
}

func TestAvailableActionsUseRefundState(t *testing.T) {
	paid := &orderRecord{Status: statusPaid, RefundState: refundStateNone}
	if actions := availableActions(paid); !containsAction(actions, actionShip) || !containsAction(actions, actionApplyAfterSale) {
		t.Fatalf("paid actions = %v", actions)
	}
	refunding := &orderRecord{Status: statusPaid, RefundState: refundStateRefunding}
	if actions := availableActions(refunding); containsAction(actions, actionShip) {
		t.Fatalf("refunding actions = %v", actions)
	}
	shipped := &orderRecord{Status: statusShipped, RefundState: refundStatePartial}
	if actions := availableActions(shipped); !containsAction(actions, actionReceive) || !containsAction(actions, actionApplyAfterSale) {
		t.Fatalf("shipped actions = %v", actions)
	}
	if got := shipmentStatus(statusCompleted); got != "RECEIVED" {
		t.Fatalf("shipment status = %s", got)
	}
}

func containsAction(actions []string, want string) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}
	return false
}

func TestTccCancelKeepsLifecycleAndSetsRefundState(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	ctx := context.Background()
	seedSettlementOrder(t, db, "ORD-TCC-PENDING", statusPendingPayment, 1000, 0, refundStateNone)
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-PENDING", PaymentNo: "PAY-1", PaidCent: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	var status, refundState string
	var paid int64
	if err := db.QueryRow(`SELECT status, refund_state, paid_cent FROM orders WHERE order_no = 'ORD-TCC-PENDING'`).
		Scan(&status, &refundState, &paid); err != nil {
		t.Fatal(err)
	}
	if status != statusPaid || refundState != refundStateRefunding || paid != 1000 {
		t.Fatalf("pending tcc cancel = %s/%s/%d", status, refundState, paid)
	}
	var topic string
	if err := db.QueryRow(`SELECT topic FROM outbox_events ORDER BY id DESC LIMIT 1`).Scan(&topic); err != nil {
		t.Fatal(err)
	}
	if topic != topicOrderRefundRequested {
		t.Fatalf("outbox topic = %s", topic)
	}

	seedSettlementOrder(t, db, "ORD-TCC-PAID", statusPaid, 1000, 0, refundStateNone)
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-PAID", PaymentNo: "PAY-2", PaidCent: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status, refund_state FROM orders WHERE order_no = 'ORD-TCC-PAID'`).
		Scan(&status, &refundState); err != nil {
		t.Fatal(err)
	}
	if status != statusPaid || refundState != refundStateRefunding {
		t.Fatalf("paid tcc cancel = %s/%s", status, refundState)
	}
}

func TestFulfillmentGroupShadowTracksOrder(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	ctx := context.Background()
	seedSettlementOrder(t, db, "ORD-GROUP", statusPendingPayment, 1000, 0, refundStateNone)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := insertOrderItems(ctx, tx, "ORD-GROUP", []*pb.OrderItem{
		{SkuId: 1, Name: "a", PriceCent: 500, Quantity: 1, DiscountCent: 0},
		{SkuId: 2, Name: "b", PriceCent: 500, Quantity: 1, DiscountCent: 0},
	}); err != nil {
		t.Fatal(err)
	}
	if err := insertFulfillmentGroups(ctx, tx, "ORD-GROUP", []*pb.OrderFulfillmentGroup{
		{GroupNo: "G1", WarehouseId: 1, FreightCent: 1000},
	}, 1000); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	groups, err := loadFulfillmentGroups(ctx, db, "ORD-GROUP")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].GroupNo != defaultGroupNo || groups[0].WarehouseID != defaultWarehouseID ||
		groups[0].Status != "PENDING" || groups[0].FreightCent != 1000 {
		t.Fatalf("groups = %+v", groups)
	}
	var line, warehouse int64
	var groupNo string
	if err := db.QueryRow(`SELECT line_no, warehouse_id, fulfillment_group_no FROM order_items WHERE order_no = 'ORD-GROUP' ORDER BY line_no LIMIT 1`).
		Scan(&line, &warehouse, &groupNo); err != nil {
		t.Fatal(err)
	}
	if line != 1 || warehouse != 1 || groupNo != defaultGroupNo {
		t.Fatalf("item allocation = %d/%d/%s", line, warehouse, groupNo)
	}
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{OrderNo: "ORD-GROUP", PaymentNo: "PAY-G", PaidCent: 1000}); err != nil {
		t.Fatal(err)
	}
	groups, err = loadFulfillmentGroups(ctx, db, "ORD-GROUP")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Status != statusAllocated {
		t.Fatalf("group after captured cancel = %+v", groups)
	}
}

func TestMultiGroupShadowAllocatedTogether(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	ctx := context.Background()
	seedSettlementOrder(t, db, "ORD-MULTI", statusPendingPayment, 1000, 0, refundStateNone)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := insertOrderItems(ctx, tx, "ORD-MULTI", []*pb.OrderItem{
		{SkuId: 1, Name: "a", PriceCent: 500, Quantity: 1, WarehouseId: 1, GroupNo: "G1"},
		{SkuId: 2, Name: "b", PriceCent: 500, Quantity: 1, WarehouseId: 2, GroupNo: "G2"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := insertFulfillmentGroups(ctx, tx, "ORD-MULTI", []*pb.OrderFulfillmentGroup{
		{GroupNo: "G1", WarehouseId: 1, FreightCent: 500},
		{GroupNo: "G2", WarehouseId: 2, FreightCent: 500},
	}, 1000); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	groups, err := loadFulfillmentGroups(ctx, db, "ORD-MULTI")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || groups[0].GroupNo != "G1" || groups[1].GroupNo != "G2" {
		t.Fatalf("groups = %+v", groups)
	}
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{OrderNo: "ORD-MULTI", PaymentNo: "PAY-M", PaidCent: 1000}); err != nil {
		t.Fatal(err)
	}
	groups, err = loadFulfillmentGroups(ctx, db, "ORD-MULTI")
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		if group.Status != statusAllocated {
			t.Fatalf("group %s = %s", group.GroupNo, group.Status)
		}
	}
}

func TestGroupShipmentProjectionIsPartialUntilAllGroups(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	ctx := context.Background()
	seedSettlementOrder(t, db, "ORD-PARTIAL", statusAllocated, 1000, 0, refundStateNone)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := insertFulfillmentGroups(ctx, tx, "ORD-PARTIAL", []*pb.OrderFulfillmentGroup{
		{GroupNo: "G1", WarehouseId: 1, FreightCent: 500},
		{GroupNo: "G2", WarehouseId: 2, FreightCent: 500},
	}, 1000); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := setFulfillmentGroupStatus(ctx, tx, "ORD-PARTIAL", statusAllocated); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	orderStatus := func() string {
		record, err := findOrder(ctx, db, "ORD-PARTIAL")
		if err != nil || record == nil {
			t.Fatalf("load order: %v", err)
		}
		return record.Status
	}
	if err := svc.applyShipmentStatusGroup(ctx, "ORD-PARTIAL", "G1", statusShipped, statusAllocated); err != nil {
		t.Fatal(err)
	}
	if got := orderStatus(); got != statusAllocated {
		t.Fatalf("after G1 shipped order = %s, want ALLOCATED", got)
	}
	if err := svc.applyShipmentStatusGroup(ctx, "ORD-PARTIAL", "G2", statusShipped, statusAllocated); err != nil {
		t.Fatal(err)
	}
	if got := orderStatus(); got != statusShipped {
		t.Fatalf("after both shipped order = %s, want SHIPPED", got)
	}
	if err := svc.applyShipmentStatusGroup(ctx, "ORD-PARTIAL", "G1", statusCompleted, statusShipped); err != nil {
		t.Fatal(err)
	}
	if got := orderStatus(); got != statusShipped {
		t.Fatalf("after G1 received order = %s, want SHIPPED", got)
	}
	if err := svc.applyShipmentStatusGroup(ctx, "ORD-PARTIAL", "G2", statusCompleted, statusShipped); err != nil {
		t.Fatal(err)
	}
	if got := orderStatus(); got != statusCompleted {
		t.Fatalf("after both received order = %s, want COMPLETED", got)
	}
	groups, err := loadFulfillmentGroups(ctx, db, "ORD-PARTIAL")
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		if group.Status != "RECEIVED" {
			t.Fatalf("group %s = %s", group.GroupNo, group.Status)
		}
	}
}

func TestGroupRefundStateProjection(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	ctx := context.Background()
	seedSettlementOrder(t, db, "ORD-GROUP-REFUND", statusAllocated, 1000, 0, refundStateNone)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := insertFulfillmentGroups(ctx, tx, "ORD-GROUP-REFUND", []*pb.OrderFulfillmentGroup{
		{GroupNo: "G1", WarehouseId: 1, FreightCent: 0},
	}, 0); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BlockShipmentForRefund(ctx, &pb.ShipmentPermitRequest{
		OrderNo: "ORD-GROUP-REFUND", AfterSaleNo: "AS-GROUP", FulfillmentGroupNo: "G1",
	}); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRow(`SELECT refund_state FROM order_fulfillment_groups WHERE order_no='ORD-GROUP-REFUND' AND group_no='G1'`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != refundStateRefunding {
		t.Fatalf("blocked group refund state = %s", state)
	}
	if err := applyAfterSaleCompleted(ctx, svc, afterSaleCompletedEvent{
		OrderNo: "ORD-GROUP-REFUND", GroupNo: "G1", AfterSaleNo: "AS-GROUP",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT refund_state FROM order_fulfillment_groups WHERE order_no='ORD-GROUP-REFUND' AND group_no='G1'`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != groupRefundStateRefunded {
		t.Fatalf("completed group refund state = %s", state)
	}
}
