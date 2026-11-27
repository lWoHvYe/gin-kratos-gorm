package order

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// t5FreshMigrationDB builds a disposable database by running every real
// migrations/order/*.sql file in order; it does not use orderTestDB's inline
// DDL. This is the T5.4 real-migration evidence.
func t5FreshMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run the T5 migration test")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil || cfg.DBName == "" {
		t.Fatalf("TEST_MYSQL_DSN must name a database: %v", err)
	}
	cfg.DBName = "mall_order_t5_migration"
	adminCfg := *cfg
	adminCfg.DBName = ""
	admin, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec("DROP DATABASE IF EXISTS `" + cfg.DBName + "`"); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	if _, err := admin.Exec("CREATE DATABASE `" + cfg.DBName + "` CHARACTER SET utf8mb4"); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	_ = admin.Close()
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		cleanup, cerr := sql.Open("mysql", adminCfg.FormatDSN())
		if cerr == nil {
			_, _ = cleanup.Exec("DROP DATABASE IF EXISTS `" + cfg.DBName + "`")
			_ = cleanup.Close()
		}
	})
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "migrations", "order", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, file := range files {
		applyOrderTestMigration(t, db, filepath.Base(file))
	}
	return db
}

func TestT5RealMigrationSchemaAndAC60Backfill(t *testing.T) {
	db := t5FreshMigrationDB(t)
	for _, column := range []struct{ table, name string }{
		{"orders", "settlement_payment_no"},
		{"order_refund_commitments", "goods_cent"},
		{"order_refund_commitments", "composition_known"},
		{"order_refund_commitment_lines", "freight_cent"},
		{"refund_commitment_releases", "authorization_id"},
		{"legacy_payment_resolution", "expires_at"},
		{"order_refund_sequences", "next_seq"},
	} {
		if got := countRows(t, db, `SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, column.table, column.name); got != 1 {
			t.Fatalf("migration schema missing %s.%s", column.table, column.name)
		}
	}

	// AC-60: a historical PAID order with a unique captured intent is backfilled
	// to its settlement identity against the real migrated schema.
	seedTccOrder(t, db, "ORD-T5-AC60", 10000, 10000, 0, 0, "")
	payments := &fakePaymentPort{intents: []*pb.Intent{
		legacyIntent("PAY-T5-AC60", "ORD-T5-AC60", "SUCCEEDED", 10000, 0),
	}}
	svc := legacyService(db)
	svc.payments = payments
	report, err := svc.BackfillLegacyPAID(context.Background(), LegacyBackfillOptions{DryRun: false, OrderNo: "ORD-T5-AC60"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Resolved != 1 || report.Errors != 0 {
		t.Fatalf("AC-60 backfill report = %+v", report)
	}
	if got := orderSettlementNo(t, db, "ORD-T5-AC60"); got != "PAY-T5-AC60" {
		t.Fatalf("AC-60 settlement = %q", got)
	}
}
