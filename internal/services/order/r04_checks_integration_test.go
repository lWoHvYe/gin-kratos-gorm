package order

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// R0.4c: real-migration coverage for order/0022_r04_checks.sql.
//
// Each scenario creates a disposable database, applies the real migration files
// from migrations/order (never inline replacement DDL for the schema under
// test), and then proves both the positive rows and the fail-closed rejection
// of dirty rows. The dirty-row tests apply 0017/0019 first, insert a row that
// the new CHECK must reject, and assert the real ALTER fails; after cleanup the
// same file must converge on a re-run.

func r04FreshOrderDB(t *testing.T, label string) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run R0.4c migration tests")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil || cfg.DBName == "" {
		t.Fatalf("TEST_MYSQL_DSN must name a database: %v", err)
	}
	name := fmt.Sprintf("mall_r04_%s_%d", label, time.Now().UnixNano())
	adminCfg := *cfg
	adminCfg.DBName = ""
	admin, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("create R0.4c test database: %v", err)
	}
	dbCfg := *cfg
	dbCfg.DBName = name
	db, err := sql.Open("mysql", dbCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		if _, err := admin.Exec("DROP DATABASE IF EXISTS `" + name + "`"); err != nil {
			t.Errorf("drop R0.4c test database: %v", err)
		}
	})
	return db
}

// runOrderMigrationFile executes one real migration file on a single
// connection (0018/0021/0022 use session-scoped PREPARE statements). It
// returns the first SQL error instead of failing the test so the dirty-row
// scenarios can assert the hard stop.
func runOrderMigrationFile(db *sql.DB, name string) error {
	path := filepath.Join("..", "..", "..", "migrations", "order", name)
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}
	var cleaned strings.Builder
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		cleaned.WriteString(line)
		cleaned.WriteString("\n")
	}
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer conn.Close()
	for _, statement := range strings.Split(cleaned.String(), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func mustRunOrderMigration(t *testing.T, db *sql.DB, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := runOrderMigrationFile(db, name); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
}

func r04ConstraintExists(t *testing.T, db *sql.DB, table, name string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
		WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = ? AND CONSTRAINT_NAME = ?
		  AND CONSTRAINT_TYPE = 'CHECK'`, table, name).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count == 1
}

func insertRefundApplication(t *testing.T, db *sql.DB, refundID, source string, groupNo any, amount, goods, freight int64, nonEffective int, state string) error {
	t.Helper()
	_, err := db.Exec(`INSERT INTO refund_applications
		(refund_id, batch_id, source, order_no, group_no, non_effective_capture,
		 amount_cent, goods_cent, freight_cent, state)
		VALUES (?, NULL, ?, 'ORD-R04', ?, ?, ?, ?, ?, ?)`,
		refundID, source, groupNo, nonEffective, amount, goods, freight, state)
	return err
}

func insertLegacyResolution(t *testing.T, db *sql.DB, orderNo, offset string) error {
	t.Helper()
	query := fmt.Sprintf(`INSERT INTO legacy_payment_resolution
		(order_no, state, entered_at, expires_at)
		VALUES (?, 'OPEN', NOW(3), DATE_ADD(NOW(3), INTERVAL %s))`, offset)
	_, err := db.Exec(query, orderNo)
	return err
}

func requireCheckViolation(t *testing.T, err error, constraint string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s to reject the row, statement succeeded", constraint)
	}
	if !strings.Contains(err.Error(), constraint) {
		t.Fatalf("error = %v, want constraint %s", err, constraint)
	}
}

func TestR04ChecksFreshSchemaAndValidRows(t *testing.T) {
	db := r04FreshOrderDB(t, "fresh")
	mustRunOrderMigration(t, db,
		"0017_refund_batch_ledger.sql",
		"0019_legacy_payment_resolution.sql",
		"0021_zero_application_check.sql",
		"0022_r04_checks.sql",
	)
	if !r04ConstraintExists(t, db, "refund_applications", "chk_refund_app_group_discriminator") {
		t.Fatal("group discriminator CHECK missing after fresh 0022")
	}
	if !r04ConstraintExists(t, db, "legacy_payment_resolution", "chk_lpr_expiry_24h") {
		t.Fatal("24h SLA CHECK missing after fresh 0022")
	}
	// 0017/0019/0021 constraints must still be present (0022 is additive).
	for _, c := range []struct{ table, name string }{
		{"refund_applications", "chk_refund_app_source"},
		{"refund_applications", "chk_refund_app_non_effective_source"},
		{"refund_applications", "chk_refund_app_system_zero"},
		{"legacy_payment_resolution", "chk_lpr_expiry"},
	} {
		if !r04ConstraintExists(t, db, c.table, c.name) {
			t.Fatalf("pre-existing CHECK %s.%s was lost", c.table, c.name)
		}
	}

	valid := []struct {
		name    string
		source  string
		groupNo any
		amount  int64
		goods   int64
		freight int64
		nonEff  int
		state   string
	}{
		{"payment-level payment", "payment", nil, 100, 100, 0, 1, "PENDING"},
		{"payment-level legacy_expired", "legacy_expired", nil, 50, 50, 0, 1, "PENDING"},
		{"group aftersale", "aftersale", "G1", 100, 90, 10, 0, "PENDING"},
		{"group tcc", "tcc", "G2", 30, 30, 0, 0, "PROCESSING"},
		{"group system_shortage", "system_shortage", "G3", 20, 20, 0, 0, "PENDING"},
		{"group system_zero", "system_zero", "G4", 0, 0, 0, 0, "DONE"},
	}
	for _, row := range valid {
		if err := insertRefundApplication(t, db, "APP-"+strings.ReplaceAll(row.name, " ", "-"),
			row.source, row.groupNo, row.amount, row.goods, row.freight, row.nonEff, row.state); err != nil {
			t.Fatalf("valid row %q rejected: %v", row.name, err)
		}
	}

	invalid := []struct {
		name    string
		source  string
		groupNo any
		nonEff  int
	}{
		{"payment with group", "payment", "G1", 1},
		{"legacy_expired with group", "legacy_expired", "G9", 1},
		{"aftersale without group", "aftersale", nil, 0},
		{"tcc with empty group", "tcc", "", 0},
		{"system_shortage without group", "system_shortage", nil, 0},
		{"system_zero without group", "system_zero", nil, 0},
	}
	for _, row := range invalid {
		err := insertRefundApplication(t, db, "BAD-"+strings.ReplaceAll(row.name, " ", "-"),
			row.source, row.groupNo, 0, 0, 0, row.nonEff, "PENDING")
		requireCheckViolation(t, err, "chk_refund_app_group_discriminator")
	}

	// 24h SLA boundary: exactly 24h passes, 48h passes, 23h and 1h fail.
	if err := insertLegacyResolution(t, db, "ORD-LPR-24H", "24 HOUR"); err != nil {
		t.Fatalf("expires_at = entered_at + 24h must pass: %v", err)
	}
	if err := insertLegacyResolution(t, db, "ORD-LPR-48H", "48 HOUR"); err != nil {
		t.Fatalf("expires_at = entered_at + 48h must pass: %v", err)
	}
	requireCheckViolation(t, insertLegacyResolution(t, db, "ORD-LPR-23H", "23 HOUR"), "chk_lpr_expiry_24h")
	requireCheckViolation(t, insertLegacyResolution(t, db, "ORD-LPR-1H", "1 HOUR"), "chk_lpr_expiry")

	// Re-running 0022 is safe and does not duplicate the constraints.
	mustRunOrderMigration(t, db, "0022_r04_checks.sql")
	if !r04ConstraintExists(t, db, "refund_applications", "chk_refund_app_group_discriminator") ||
		!r04ConstraintExists(t, db, "legacy_payment_resolution", "chk_lpr_expiry_24h") {
		t.Fatal("0022 re-run lost a CHECK")
	}
}

func TestR04ChecksDirtyRowsBlockMigration(t *testing.T) {
	t.Run("group discriminator dirty row", func(t *testing.T) {
		db := r04FreshOrderDB(t, "dirty_group")
		mustRunOrderMigration(t, db, "0017_refund_batch_ledger.sql", "0019_legacy_payment_resolution.sql")
		// Before 0022 this row is legal; the new discriminator must block it.
		if err := insertRefundApplication(t, db, "PRE-0022-PAYMENT", "payment", "G1", 100, 100, 0, 1, "PENDING"); err != nil {
			t.Fatalf("pre-0022 payment row: %v", err)
		}
		err := runOrderMigrationFile(db, "0022_r04_checks.sql")
		requireCheckViolation(t, err, "chk_refund_app_group_discriminator")
		if r04ConstraintExists(t, db, "refund_applications", "chk_refund_app_group_discriminator") {
			t.Fatal("failed ALTER left the group discriminator installed")
		}
		if _, err := db.Exec("DELETE FROM refund_applications WHERE refund_id = 'PRE-0022-PAYMENT'"); err != nil {
			t.Fatal(err)
		}
		mustRunOrderMigration(t, db, "0022_r04_checks.sql")
		if !r04ConstraintExists(t, db, "refund_applications", "chk_refund_app_group_discriminator") {
			t.Fatal("0022 did not converge after the dirty row was removed")
		}
	})

	t.Run("24h SLA dirty row", func(t *testing.T) {
		db := r04FreshOrderDB(t, "dirty_sla")
		mustRunOrderMigration(t, db, "0017_refund_batch_ledger.sql", "0019_legacy_payment_resolution.sql")
		if err := insertLegacyResolution(t, db, "ORD-PRE-0022-1H", "1 HOUR"); err != nil {
			t.Fatalf("pre-0022 legacy row: %v", err)
		}
		err := runOrderMigrationFile(db, "0022_r04_checks.sql")
		requireCheckViolation(t, err, "chk_lpr_expiry_24h")
		// The first ALTER (group discriminator) is committed before the second
		// fails; the re-run must skip it via information_schema and add the 24h
		// check after the dirty row is fixed.
		if !r04ConstraintExists(t, db, "refund_applications", "chk_refund_app_group_discriminator") {
			t.Fatal("partial 0022 should keep the first committed CHECK")
		}
		if r04ConstraintExists(t, db, "legacy_payment_resolution", "chk_lpr_expiry_24h") {
			t.Fatal("failed ALTER left the 24h CHECK installed")
		}
		if _, err := db.Exec("UPDATE legacy_payment_resolution SET expires_at = DATE_ADD(entered_at, INTERVAL 24 HOUR) WHERE order_no = 'ORD-PRE-0022-1H'"); err != nil {
			t.Fatal(err)
		}
		mustRunOrderMigration(t, db, "0022_r04_checks.sql")
		if !r04ConstraintExists(t, db, "legacy_payment_resolution", "chk_lpr_expiry_24h") {
			t.Fatal("0022 did not converge after the dirty row was fixed")
		}
	})
}
