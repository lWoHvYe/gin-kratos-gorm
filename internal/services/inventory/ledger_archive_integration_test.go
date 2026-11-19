package inventory

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func ledgerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run ledger archive tests")
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
		`CREATE TABLE IF NOT EXISTS inventory_ledger (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, reservation_no VARCHAR(64) NOT NULL,
			order_no VARCHAR(64) NOT NULL, sku_id BIGINT NOT NULL, warehouse_id BIGINT NOT NULL,
			quantity BIGINT NOT NULL, entry_type VARCHAR(16) NOT NULL,
			on_hand_after BIGINT NOT NULL DEFAULT 0, reserved_after BIGINT NOT NULL DEFAULT 0,
			created_at DATETIME(3) NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS inventory_ledger_archive (
			id BIGINT NOT NULL PRIMARY KEY, reservation_no VARCHAR(64) NOT NULL,
			order_no VARCHAR(64) NOT NULL, sku_id BIGINT NOT NULL, warehouse_id BIGINT NOT NULL,
			quantity BIGINT NOT NULL, entry_type VARCHAR(16) NOT NULL,
			on_hand_after BIGINT NOT NULL DEFAULT 0, reserved_after BIGINT NOT NULL DEFAULT 0,
			created_at DATETIME(3) NOT NULL, archived_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3))`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	_, _ = db.Exec(`DELETE FROM inventory_ledger`)
	_, _ = db.Exec(`DELETE FROM inventory_ledger_archive`)
	return db
}

func TestArchiveLedgerMovesOnlyOldRows(t *testing.T) {
	db := ledgerTestDB(t)
	old := time.Now().Add(-ledgerRetention - time.Hour)
	_, err := db.Exec(`INSERT INTO inventory_ledger
		(reservation_no,order_no,sku_id,warehouse_id,quantity,entry_type,created_at)
		VALUES ('r-old','o-old',1,1,1,'RESERVE',?), ('r-new','o-new',1,1,1,'RESERVE',?)`, old, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	n, err := archiveLedgerOnce(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("archived=%d, want 1", n)
	}
	var hot, archived int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_ledger`).Scan(&hot); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_ledger_archive`).Scan(&archived); err != nil {
		t.Fatal(err)
	}
	if hot != 1 || archived != 1 {
		t.Fatalf("hot=%d archived=%d, want 1/1", hot, archived)
	}
}
