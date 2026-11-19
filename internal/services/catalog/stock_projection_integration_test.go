package catalog

import (
	"context"
	"database/sql"
	"os"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func stockTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run stock projection integration tests")
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
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS skus (
		id BIGINT NOT NULL PRIMARY KEY, spu_id BIGINT NOT NULL,
		stock BIGINT NOT NULL DEFAULT 0, stock_version BIGINT NOT NULL DEFAULT 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM skus`); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestStockProjectionRejectsOlderVersion(t *testing.T) {
	db := stockTestDB(t)
	if _, err := db.Exec(`INSERT INTO skus (id, spu_id, stock, stock_version) VALUES (1, 10, 5, 5)`); err != nil {
		t.Fatal(err)
	}
	if changed, err := applyStockChanged(context.Background(), db, stockChangedEvent{SkuID: 1, Available: 99, Version: 4}); err != nil || changed {
		t.Fatalf("older event changed=%v err=%v", changed, err)
	}
	var stock, version int64
	if err := db.QueryRow(`SELECT stock, stock_version FROM skus WHERE id = 1`).Scan(&stock, &version); err != nil {
		t.Fatal(err)
	}
	if stock != 5 || version != 5 {
		t.Fatalf("older event overwrote projection: stock=%d version=%d", stock, version)
	}
	if changed, err := applyStockChanged(context.Background(), db, stockChangedEvent{SkuID: 1, Available: 7, Version: 6}); err != nil || !changed {
		t.Fatalf("newer event changed=%v err=%v", changed, err)
	}
	if err := db.QueryRow(`SELECT stock, stock_version FROM skus WHERE id = 1`).Scan(&stock, &version); err != nil {
		t.Fatal(err)
	}
	if stock != 7 || version != 6 {
		t.Fatalf("newer event not applied: stock=%d version=%d", stock, version)
	}
}
