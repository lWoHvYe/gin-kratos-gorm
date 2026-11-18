package inventory

import (
	"context"
	"database/sql"
	"os"
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
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}
	for _, table := range []string{"inventory_stocks", "stock_reservations", "seckill_stocks", "seckill_order_stock", "seckill_stock_ledger"} {
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
	released, err := svc.releaseSeckillMapping(context.Background(), tx, &reservation{orderNo: "ORD-LATE", skuID: 1, warehouseID: 1, quantity: 1})
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
	if _, err := svc.releaseSeckillMapping(context.Background(), tx, &reservation{orderNo: "ORD-DOUBLE", skuID: 1, warehouseID: 1, quantity: 1}); err != nil {
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
