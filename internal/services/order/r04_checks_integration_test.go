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

// R0.4c (branch A): real-migration coverage for the frozen
// order/0022_r04_checks.sql and its replacement order/0023_r04_checks_v2.sql.
//
// Branch A keeps 0022 byte-identical to its pre-edit blob and puts every
// semantic correction in 0023: install the `_v2` constraints first, then (only
// after both adds succeeded) drop the old names. 0022's "conditional ALTERs
// make a re-run after cleanup safe" header claim is therefore superseded here,
// not edited in that file.
//
// These scenarios apply the real files in fresh order
// (0017 -> 0019 -> 0021 -> 0022 -> 0023) and prove:
//   * the replacement semantics (v2 present, old names gone, CHECK_CLAUSE is the
//     equality/discriminator, no 0017 enum copy);
//   * the fail-closed boundaries (24h+1s / 25h / 48h and <24h are rejected);
//   * replay of 0023 is idempotent;
//   * 0022's `>=` accepts a 48h row (the WIP regression) and 0023 must stop on
//     it without dropping the old constraints, converging after an audited
//     repair;
//   * the partial state "v2 added, old not dropped" (interrupted 0023) converges
//     when the same file is re-run.

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

// execMigrationSQL runs comment-stripped migration statements on one
// connection (0018/0021/0022/0023 use session-scoped PREPARE statements). It
// returns the first SQL error instead of failing the test so the dirty-row
// scenarios can assert the hard stop.
func execMigrationSQL(db *sql.DB, name, body string) error {
	var cleaned strings.Builder
	for _, line := range strings.Split(body, "\n") {
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

func runOrderMigrationFile(db *sql.DB, name string) error {
	path := filepath.Join("..", "..", "..", "migrations", "order", name)
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}
	return execMigrationSQL(db, name, string(body))
}

// runOrderMigrationPrefixBefore replays the real file up to (excluding) the
// first line containing marker. The scenarios use it to reproduce the crash
// window after the v2 ADDs but before the old-name drops.
func runOrderMigrationPrefixBefore(db *sql.DB, name, marker string) error {
	path := filepath.Join("..", "..", "..", "migrations", "order", name)
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}
	var kept strings.Builder
	found := false
	for _, line := range strings.Split(string(body), "\n") {
		if strings.Contains(line, marker) {
			found = true
			break
		}
		kept.WriteString(line)
		kept.WriteString("\n")
	}
	if !found {
		return fmt.Errorf("marker %q not found in %s", marker, name)
	}
	return execMigrationSQL(db, name+" (prefix)", kept.String())
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

func r04CheckClause(t *testing.T, db *sql.DB, table, name string) string {
	t.Helper()
	var clause string
	if err := db.QueryRow(`SELECT cc.CHECK_CLAUSE
		FROM information_schema.CHECK_CONSTRAINTS cc
		JOIN information_schema.TABLE_CONSTRAINTS tc
		  ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA
		 AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
		WHERE cc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = ? AND cc.CONSTRAINT_NAME = ?`,
		table, name).Scan(&clause); err != nil {
		t.Fatalf("read CHECK_CLAUSE %s.%s: %v", table, name, err)
	}
	return clause
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

// insertLegacyResolution inserts an OPEN legacy resolution whose expires_at is
// the given SQL expression. entered_at and expires_at both use NOW(3) in the
// same statement, which MySQL evaluates to one statement timestamp, so
// "DATE_ADD(NOW(3), INTERVAL 24 HOUR)" is exactly entered_at + 24h.
func insertLegacyResolution(t *testing.T, db *sql.DB, orderNo, expiresSQL string) error {
	t.Helper()
	_, err := db.Exec(fmt.Sprintf(`INSERT INTO legacy_payment_resolution
		(order_no, state, entered_at, expires_at)
		VALUES (?, 'OPEN', NOW(3), %s)`, expiresSQL), orderNo)
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

// requireV2Replacement asserts the end state of a successful 0022 -> 0023
// apply: exactly the v2 constraints, no legacy names.
func requireV2Replacement(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, c := range []struct{ table, name string }{
		{"refund_applications", "chk_refund_app_group_discriminator_v2"},
		{"legacy_payment_resolution", "chk_lpr_expiry_24h_v2"},
	} {
		if !r04ConstraintExists(t, db, c.table, c.name) {
			t.Fatalf("replacement CHECK %s.%s missing", c.table, c.name)
		}
	}
	for _, c := range []struct{ table, name string }{
		{"refund_applications", "chk_refund_app_group_discriminator"},
		{"legacy_payment_resolution", "chk_lpr_expiry_24h"},
	} {
		if r04ConstraintExists(t, db, c.table, c.name) {
			t.Fatalf("legacy CHECK %s.%s must be dropped by 0023", c.table, c.name)
		}
	}
}

func TestR04ChecksFreshSchemaAndValidRows(t *testing.T) {
	db := r04FreshOrderDB(t, "fresh")
	// Branch A order: the frozen 0022 first (it installs the legacy checks),
	// then the replacement 0023.
	mustRunOrderMigration(t, db,
		"0017_refund_batch_ledger.sql",
		"0019_legacy_payment_resolution.sql",
		"0021_zero_application_check.sql",
		"0022_r04_checks.sql",
		"0023_r04_checks_v2.sql",
	)
	requireV2Replacement(t, db)

	// 0017/0019/0021 constraints must still be present (0023 only replaces the
	// two 0022 checks).
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

	// The installed clauses must carry the accepted semantics, not just the
	// names: the v2 discriminator must not copy the 0017 source enum and the
	// v2 24h window must be equality, never a lower bound.
	groupClause := strings.ToLower(r04CheckClause(t, db, "refund_applications", "chk_refund_app_group_discriminator_v2"))
	if !strings.Contains(groupClause, "group_no") || !strings.Contains(groupClause, "is null") {
		t.Fatalf("v2 group discriminator clause = %q, want a group_no NULL discriminator", groupClause)
	}
	for _, copiedSource := range []string{"system_zero", "system_shortage", "aftersale", "tcc"} {
		if strings.Contains(groupClause, copiedSource) {
			t.Fatalf("v2 group discriminator clause copies the 0017 enum (%s): %q", copiedSource, groupClause)
		}
	}
	expiryClause := strings.ToLower(r04CheckClause(t, db, "legacy_payment_resolution", "chk_lpr_expiry_24h_v2"))
	if !strings.Contains(expiryClause, "interval 24 hour") || strings.Contains(expiryClause, ">=") {
		t.Fatalf("v2 24h clause = %q, want equality on entered_at + 24h", expiryClause)
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
		requireCheckViolation(t, err, "chk_refund_app_group_discriminator_v2")
	}

	// 24h SLA boundary: exactly 24h passes; 24h+1s, 25h and 48h are rejected by
	// the v2 equality. A lower bound (>=, 0022's frozen shape) would accept 48h,
	// which the WIP test wrongly asserted was legal.
	if err := insertLegacyResolution(t, db, "ORD-LPR-24H", "DATE_ADD(NOW(3), INTERVAL 24 HOUR)"); err != nil {
		t.Fatalf("expires_at = entered_at + 24h must pass: %v", err)
	}
	for _, tc := range []struct{ orderNo, expr string }{
		{"ORD-LPR-24H1S", "DATE_ADD(NOW(3), INTERVAL 24 HOUR) + INTERVAL 1 SECOND"},
		{"ORD-LPR-25H", "DATE_ADD(NOW(3), INTERVAL 25 HOUR)"},
		{"ORD-LPR-48H", "DATE_ADD(NOW(3), INTERVAL 48 HOUR)"},
	} {
		requireCheckViolation(t, insertLegacyResolution(t, db, tc.orderNo, tc.expr), "chk_lpr_expiry_24h_v2")
	}
	// Shorter windows violate the v2 equality and 0019's existing lower bound;
	// MySQL reports either constraint name, so assert the shared prefix.
	for _, tc := range []struct{ orderNo, expr string }{
		{"ORD-LPR-23H", "DATE_ADD(NOW(3), INTERVAL 23 HOUR)"},
		{"ORD-LPR-24H-1S", "DATE_ADD(NOW(3), INTERVAL 24 HOUR) - INTERVAL 1 SECOND"},
		{"ORD-LPR-1H", "DATE_ADD(NOW(3), INTERVAL 1 HOUR)"},
	} {
		requireCheckViolation(t, insertLegacyResolution(t, db, tc.orderNo, tc.expr), "chk_lpr_expiry")
	}

	// Re-running 0023 is idempotent: conditional add/drop must not duplicate a
	// v2 CHECK and must not resurrect an old name.
	mustRunOrderMigration(t, db, "0023_r04_checks_v2.sql")
	requireV2Replacement(t, db)
	if got := strings.ToLower(r04CheckClause(t, db, "legacy_payment_resolution", "chk_lpr_expiry_24h_v2")); !strings.Contains(got, "interval 24 hour") || strings.Contains(got, ">=") {
		t.Fatalf("0023 re-run changed the v2 24h clause: %q", got)
	}
}

func TestR04ChecksDirtyRowsBlockMigration(t *testing.T) {
	// 0022's frozen `>=` accepts a 48h deadline; 0023's v2 equality must stop on
	// it without dropping the old checks, and a repaired re-run converges.
	t.Run("48h dirty row blocks 0023 then converges", func(t *testing.T) {
		db := r04FreshOrderDB(t, "dirty_48h")
		mustRunOrderMigration(t, db, "0017_refund_batch_ledger.sql", "0019_legacy_payment_resolution.sql", "0022_r04_checks.sql")
		if !r04ConstraintExists(t, db, "legacy_payment_resolution", "chk_lpr_expiry_24h") {
			t.Fatal("frozen 0022 should have installed the legacy 24h CHECK")
		}
		// Legal under 0022 (`>=`), forbidden by the accepted equality contract.
		if err := insertLegacyResolution(t, db, "ORD-PRE-0023-48H", "DATE_ADD(NOW(3), INTERVAL 48 HOUR)"); err != nil {
			t.Fatalf("pre-0023 48h row: %v", err)
		}
		err := runOrderMigrationFile(db, "0023_r04_checks_v2.sql")
		requireCheckViolation(t, err, "chk_lpr_expiry_24h_v2")
		// Both v2 ADDs run before any DROP: the 24h ADD stops the file with the
		// committed v2 discriminator, the legacy checks still in force and no old
		// name dropped. No table is ever left without a constraint.
		if !r04ConstraintExists(t, db, "refund_applications", "chk_refund_app_group_discriminator_v2") {
			t.Fatal("partial 0023 should keep the committed v2 discriminator")
		}
		if !r04ConstraintExists(t, db, "refund_applications", "chk_refund_app_group_discriminator") {
			t.Fatal("failed 0023 must not drop the legacy discriminator before both v2 adds")
		}
		if !r04ConstraintExists(t, db, "legacy_payment_resolution", "chk_lpr_expiry_24h") {
			t.Fatal("failed 0023 must not drop the legacy 24h CHECK")
		}
		if r04ConstraintExists(t, db, "legacy_payment_resolution", "chk_lpr_expiry_24h_v2") {
			t.Fatal("failed ALTER left the v2 24h CHECK installed")
		}
		if _, err := db.Exec("UPDATE legacy_payment_resolution SET expires_at = DATE_ADD(entered_at, INTERVAL 24 HOUR) WHERE order_no = 'ORD-PRE-0023-48H'"); err != nil {
			t.Fatal(err)
		}
		mustRunOrderMigration(t, db, "0023_r04_checks_v2.sql")
		requireV2Replacement(t, db)
	})

	// Interrupted 0023: both v2 ADDs committed, no old-name DROP yet. Re-running
	// the same file must converge (drop the old names) instead of failing.
	t.Run("partial failure v2 added old not dropped converges", func(t *testing.T) {
		db := r04FreshOrderDB(t, "partial_0023")
		mustRunOrderMigration(t, db, "0017_refund_batch_ledger.sql", "0019_legacy_payment_resolution.sql", "0022_r04_checks.sql")
		if err := runOrderMigrationPrefixBefore(db, "0023_r04_checks_v2.sql",
			"-- Only after both v2 CHECKs exist are the old names dropped."); err != nil {
			t.Fatalf("reproduce interrupted 0023: %v", err)
		}
		for _, c := range []struct{ table, name string }{
			{"refund_applications", "chk_refund_app_group_discriminator_v2"},
			{"legacy_payment_resolution", "chk_lpr_expiry_24h_v2"},
			{"refund_applications", "chk_refund_app_group_discriminator"},
			{"legacy_payment_resolution", "chk_lpr_expiry_24h"},
		} {
			if !r04ConstraintExists(t, db, c.table, c.name) {
				t.Fatalf("interrupted 0023 should leave %s.%s installed", c.table, c.name)
			}
		}
		mustRunOrderMigration(t, db, "0023_r04_checks_v2.sql")
		requireV2Replacement(t, db)
	})

	// The discriminating row must stay fail-closed before 0022: a payment-level
	// row that names a fulfillment group is rejected by 0022 itself, so 0023
	// cannot be reached with a dirty discriminator that 0022 accepted.
	t.Run("discriminator dirty row still blocked by 0022", func(t *testing.T) {
		db := r04FreshOrderDB(t, "dirty_group")
		mustRunOrderMigration(t, db, "0017_refund_batch_ledger.sql", "0019_legacy_payment_resolution.sql")
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
		mustRunOrderMigration(t, db, "0022_r04_checks.sql", "0023_r04_checks_v2.sql")
		requireV2Replacement(t, db)
	})
}
