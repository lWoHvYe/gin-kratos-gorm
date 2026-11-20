package inventory

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func seckillTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run seckill inventory integration tests")
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
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS inventory_stocks (
			sku_id BIGINT NOT NULL, warehouse_id BIGINT NOT NULL, on_hand BIGINT NOT NULL DEFAULT 0,
			reserved BIGINT NOT NULL DEFAULT 0, version BIGINT NOT NULL DEFAULT 1,
			PRIMARY KEY (sku_id, warehouse_id))`,
		`CREATE TABLE IF NOT EXISTS stock_reservations (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, reservation_no VARCHAR(64) NOT NULL,
			order_no VARCHAR(64) NOT NULL, sku_id BIGINT NOT NULL, warehouse_id BIGINT NOT NULL,
			quantity BIGINT NOT NULL, status VARCHAR(16) NOT NULL, expires_at DATETIME(3) NOT NULL,
			created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS seckill_stocks (
			activity_id BIGINT NOT NULL PRIMARY KEY, sku_id BIGINT NOT NULL, warehouse_id BIGINT NOT NULL,
			total BIGINT NOT NULL, available BIGINT NOT NULL, held BIGINT NOT NULL, sold BIGINT NOT NULL,
			status VARCHAR(16) NOT NULL, version BIGINT NOT NULL DEFAULT 1)`,
		`CREATE TABLE IF NOT EXISTS seckill_order_stock (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, activity_id BIGINT NOT NULL,
			order_no VARCHAR(64) NOT NULL, sku_id BIGINT NOT NULL, quantity BIGINT NOT NULL,
			ref_no VARCHAR(64) NOT NULL, status VARCHAR(16) NOT NULL,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_seckill_order_stock (order_no, sku_id))`,
		`CREATE TABLE IF NOT EXISTS seckill_stock_ledger (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, activity_id BIGINT NOT NULL,
			delta BIGINT NOT NULL, balance_after BIGINT NOT NULL, reason VARCHAR(32) NOT NULL,
			ref_no VARCHAR(64) NOT NULL, created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3))`,
		`CREATE TABLE IF NOT EXISTS inventory_ledger (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, reservation_no VARCHAR(64) NOT NULL,
			order_no VARCHAR(64) NOT NULL, sku_id BIGINT NOT NULL, warehouse_id BIGINT NOT NULL,
			quantity BIGINT NOT NULL, entry_type VARCHAR(16) NOT NULL,
			on_hand_after BIGINT NOT NULL DEFAULT 0, reserved_after BIGINT NOT NULL DEFAULT 0,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_ledger_reservation_entry (reservation_no, entry_type))`,
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
			t.Fatalf("create test table: %v", err)
		}
	}
	for _, table := range []string{"inventory_stocks", "stock_reservations", "seckill_stocks", "seckill_order_stock", "seckill_stock_ledger", "inventory_ledger", "outbox_events"} {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func seedBucket(t *testing.T, db *sql.DB, activity int64, available, held, sold int64, status string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO seckill_stocks
		(activity_id, sku_id, warehouse_id, total, available, held, sold, status, version)
		VALUES (?, 1, 1, ?, ?, ?, ?, ?, 1)`, activity, available+held+sold, available, held, sold, status); err != nil {
		t.Fatal(err)
	}
}

func seedMapping(t *testing.T, db *sql.DB, activity int64, orderNo, status string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO seckill_order_stock
		(activity_id, order_no, sku_id, quantity, ref_no, status) VALUES (?, ?, 1, 1, ?, ?)`,
		activity, orderNo, orderNo+"-ref", status); err != nil {
		t.Fatal(err)
	}
}

func TestCancelConfirmedReturnsUnitToOpenBucket(t *testing.T) {
	db := seckillTestDB(t)
	seedBucket(t, db, 900, 2, 0, 2, seckillStatusAllocated)
	seedMapping(t, db, 900, "ORD-CANCEL", statusConfirmed)
	svc := &Service{db: db}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.cancelSeckillConfirmed(context.Background(), tx, &reservation{orderNo: "ORD-CANCEL", skuID: 1, warehouseID: 1, quantity: 1}); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var available, sold, held int64
	if err := db.QueryRow(`SELECT available, sold, held FROM seckill_stocks WHERE activity_id = 900`).Scan(&available, &sold, &held); err != nil {
		t.Fatal(err)
	}
	if available != 3 || sold != 1 || held != 0 {
		t.Fatalf("available=%d sold=%d held=%d, want 3/1/0", available, sold, held)
	}
	var mappingStatus string
	if err := db.QueryRow(`SELECT status FROM seckill_order_stock WHERE order_no = 'ORD-CANCEL'`).Scan(&mappingStatus); err != nil {
		t.Fatal(err)
	}
	if mappingStatus != statusReleased {
		t.Fatalf("mapping = %s", mappingStatus)
	}
}

func TestReleaseHeldAfterBucketClosedReturnsNormalReserved(t *testing.T) {
	db := seckillTestDB(t)
	if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version) VALUES (1, 1, 10, 3, 1)`); err != nil {
		t.Fatal(err)
	}
	seedBucket(t, db, 901, 0, 1, 0, seckillStatusReleased)
	seedMapping(t, db, 901, "ORD-LATE", statusHeld)
	svc := &Service{db: db}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	released, _, err := svc.releaseSeckillMapping(context.Background(), tx, &reservation{orderNo: "ORD-LATE", skuID: 1, warehouseID: 1, quantity: 1})
	if err != nil || !released {
		_ = tx.Rollback()
		t.Fatalf("released=%v err=%v", released, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var reserved, held, available int64
	if err := db.QueryRow(`SELECT reserved FROM inventory_stocks WHERE sku_id = 1`).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT held, available FROM seckill_stocks WHERE activity_id = 901`).Scan(&held, &available); err != nil {
		t.Fatal(err)
	}
	if reserved != 2 || held != 0 || available != 0 {
		t.Fatalf("reserved=%d held=%d available=%d, want 2/0/0", reserved, held, available)
	}
	var ledgerRows, outboxRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_ledger`).Scan(&ledgerRows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox_events`).Scan(&outboxRows); err != nil {
		t.Fatal(err)
	}
	if ledgerRows == 0 || outboxRows == 0 {
		t.Fatalf("closed-bucket release ledger=%d outbox=%d, want both > 0", ledgerRows, outboxRows)
	}
}

func TestQualificationReleaseDoesNotDoubleCreditConvertedOrder(t *testing.T) {
	db := seckillTestDB(t)
	seedBucket(t, db, 902, 0, 1, 0, seckillStatusAllocated)
	seedMapping(t, db, 902, "ORD-DOUBLE", statusHeld)
	svc := &Service{db: db}
	if _, err := svc.ReleaseSeckillHold(context.Background(), &pb.ReleaseSeckillHoldRequest{
		ActivityId: 902, Quantity: 1, RefNo: "ORD-DOUBLE-ref",
	}); err != nil {
		t.Fatal(err)
	}
	var available, held int64
	if err := db.QueryRow(`SELECT available, held FROM seckill_stocks WHERE activity_id = 902`).Scan(&available, &held); err != nil {
		t.Fatal(err)
	}
	if available != 0 || held != 1 {
		t.Fatalf("converted hold released twice: available=%d held=%d", available, held)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.releaseSeckillMapping(context.Background(), tx, &reservation{orderNo: "ORD-DOUBLE", skuID: 1, warehouseID: 1, quantity: 1}); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT available, held FROM seckill_stocks WHERE activity_id = 902`).Scan(&available, &held); err != nil {
		t.Fatal(err)
	}
	if available != 1 || held != 0 {
		t.Fatalf("order release available=%d held=%d, want 1/0", available, held)
	}
}

func TestConcurrentReleaseDoesNotDoubleCredit(t *testing.T) {
	db := seckillTestDB(t)
	seedBucket(t, db, 903, 0, 1, 0, seckillStatusAllocated)
	seedMapping(t, db, 903, "ORD-RACE", statusHeld)
	svc := &Service{db: db}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := svc.ReleaseSeckillHold(context.Background(), &pb.ReleaseSeckillHoldRequest{
			ActivityId: 903, Quantity: 1, RefNo: "ORD-RACE-ref",
		})
		errs <- err
	}()
	go func() {
		defer wg.Done()
		tx, err := db.BeginTx(context.Background(), nil)
		if err == nil {
			_, _, err = svc.releaseSeckillMapping(context.Background(), tx, &reservation{
				orderNo: "ORD-RACE", skuID: 1, warehouseID: 1, quantity: 1,
			})
			if err == nil {
				err = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
		}
		errs <- err
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent release: %v", err)
		}
	}
	var available, held int64
	if err := db.QueryRow(`SELECT available, held FROM seckill_stocks WHERE activity_id = 903`).Scan(&available, &held); err != nil {
		t.Fatal(err)
	}
	if available != 1 || held != 0 {
		t.Fatalf("double credit: available=%d held=%d, want 1/0", available, held)
	}
}

func TestReleaseTxClosedSeckillWritesOneLedger(t *testing.T) {
	db := seckillTestDB(t)
	if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version) VALUES (1, 1, 10, 2, 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO stock_reservations
		(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
		VALUES ('RES-CLOSED','ORD-CLOSED',1,1,1,'HELD',DATE_ADD(NOW(3), INTERVAL 1 HOUR),NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	seedBucket(t, db, 903, 0, 1, 0, seckillStatusReleased)
	seedMapping(t, db, 903, "ORD-CLOSED", statusHeld)
	svc := &Service{db: db}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.releaseTx(context.Background(), tx, reserveInput{orderNo: "ORD-CLOSED", skuID: 1, warehouseID: 1, quantity: 1}); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var ledgers int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_ledger WHERE order_no = 'ORD-CLOSED'`).Scan(&ledgers); err != nil {
		t.Fatal(err)
	}
	if ledgers != 1 {
		t.Fatalf("ledger rows = %d, want exactly 1", ledgers)
	}
	var reserved int64
	if err := db.QueryRow(`SELECT reserved FROM inventory_stocks WHERE sku_id=1`).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if reserved != 1 {
		t.Fatalf("reserved = %d, want 1", reserved)
	}
}

func TestTccCancelConfirmRestocksConfirmedReservation(t *testing.T) {
	db := seckillTestDB(t)
	if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version) VALUES (1, 1, 10, 0, 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO stock_reservations
		(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
		VALUES ('RES-CONF','ORD-CONF',1,1,1,'CONFIRMED',DATE_ADD(NOW(3), INTERVAL 1 HOUR),NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	svc := &Service{db: db}
	if _, err := svc.TccCancelConfirm(context.Background(), &pb.TccPaymentRequest{
		OrderNo: "ORD-CONF", Items: []*pb.OrderItem{{SkuId: 1, Quantity: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	var onHand, reserved int64
	if err := db.QueryRow(`SELECT on_hand, reserved FROM inventory_stocks WHERE sku_id=1`).Scan(&onHand, &reserved); err != nil {
		t.Fatal(err)
	}
	if onHand != 11 || reserved != 0 {
		t.Fatalf("on_hand=%d reserved=%d, want 11/0", onHand, reserved)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM stock_reservations WHERE order_no='ORD-CONF'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusReleased {
		t.Fatalf("status=%s", status)
	}
}

func TestBatchGetStockDeduplicatesAndOmitsMissing(t *testing.T) {
	db := seckillTestDB(t)
	for _, row := range []struct{ sku, onHand, reserved int64 }{{1, 10, 2}, {2, 5, 0}} {
		if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version) VALUES (?, 1, ?, ?, 3)`,
			row.sku, row.onHand, row.reserved); err != nil {
			t.Fatal(err)
		}
	}
	svc := &Service{db: db}
	reply, err := svc.BatchGetStock(context.Background(), &pb.BatchGetStockRequest{SkuIds: []int64{1, 2, 1, 999}})
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.GetStocks()) != 2 || reply.GetStocks()[0].GetAvailable() != 8 {
		t.Fatalf("stocks = %+v", reply.GetStocks())
	}
}
