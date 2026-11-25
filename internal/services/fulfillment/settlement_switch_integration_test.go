package fulfillment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// R0.4b integration matrix: the startup gate reads the real
// settlement_switch_approvals table (fulfillment/0010) on a fresh MySQL schema.
// No inline replacement DDL is used for the table under test.

func settlementSwitchTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run settlement switch integration tests")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil || cfg.DBName == "" {
		t.Fatalf("TEST_MYSQL_DSN must name a database: %v", err)
	}
	name := fmt.Sprintf("mall_r04_switch_%d", time.Now().UnixNano())
	adminCfg := *cfg
	adminCfg.DBName = ""
	admin, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("create switch test database: %v", err)
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
			t.Errorf("drop switch test database: %v", err)
		}
	})
	applyFulfillmentTestMigration(t, db, "0010_settlement_shadow_switch.sql")
	return db
}

// applyFulfillmentTestMigration executes a real migrations/fulfillment/*.sql
// file on one connection. 0010 contains only plain statements; DELIMITER-based
// files (0011) are exercised by the ops migration runner instead.
func applyFulfillmentTestMigration(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "migrations", "fulfillment", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
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
		t.Fatalf("open migration connection: %v", err)
	}
	defer conn.Close()
	for _, statement := range strings.Split(cleaned.String(), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			t.Fatalf("apply migration %s: %v\n%s", name, err, statement)
		}
	}
}

// seedApproval inserts one approval row. expires is "null", "past" or "future"
// and is evaluated on the database clock. A settled row must also carry the
// ON removal fields required by 0010's chk_switch_removal.
func seedApproval(t *testing.T, db *sql.DB, id, target, status, releaseRange, expires string) {
	t.Helper()
	var expiresSQL string
	switch expires {
	case "null":
		expiresSQL = "NULL"
	case "past":
		expiresSQL = "DATE_SUB(NOW(3), INTERVAL 1 HOUR)"
	case "future":
		expiresSQL = "DATE_ADD(NOW(3), INTERVAL 1 HOUR)"
	default:
		t.Fatalf("bad expiry fixture %q", expires)
	}
	var retired, removal string
	if target == SettlementModeSettled {
		retired, removal = "'2026-11-25 00:00:00.000'", "'2026-12-25'"
	} else {
		retired, removal = "NULL", "NULL"
	}
	query := fmt.Sprintf(`INSERT INTO settlement_switch_approvals
		(approval_id, target_mode, approved_by, approved_at, release_range, evidence_ref,
		 expires_at, order_paid_admission_retired_at, order_paid_subject_removal_date, status)
		VALUES (?, ?, 'director', NOW(3), ?, 'evidence', %s, %s, %s, ?)`,
		expiresSQL, retired, removal)
	if _, err := db.Exec(query, id, target, releaseRange, status); err != nil {
		t.Fatalf("seed approval %s: %v", id, err)
	}
}

func clearApprovals(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("DELETE FROM settlement_switch_approvals"); err != nil {
		t.Fatalf("clear approvals: %v", err)
	}
}

func requireSwitchError(t *testing.T, err error, wantMode, wantApproval string, wantSubstrings ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a switch rejection, got nil")
	}
	var switchErr *SettlementSwitchError
	if !errors.As(err, &switchErr) {
		t.Fatalf("error type = %T, want *SettlementSwitchError: %v", err, err)
	}
	if switchErr.Mode != wantMode {
		t.Fatalf("error mode = %q, want %q (%v)", switchErr.Mode, wantMode, err)
	}
	if switchErr.ApprovalID != wantApproval {
		t.Fatalf("error approval = %q, want %q (%v)", switchErr.ApprovalID, wantApproval, err)
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want containing %q", err, want)
		}
	}
}

func TestSettlementSwitchConfigFailuresDoNotTouchDB(t *testing.T) {
	ctx := context.Background()
	// No DB at all: paid with an approval id and shadow/settled with missing
	// range/approval must fail before any query.
	_, err := VerifySettlementSwitch(ctx, nil, SettlementSwitchConfig{Mode: SettlementModePaid, ApprovalID: "apr-1"})
	requireSwitchError(t, err, SettlementModePaid, "apr-1", "N5")

	_, err = VerifySettlementSwitch(ctx, nil, SettlementSwitchConfig{Mode: SettlementModeShadow, ReleaseRange: ""})
	requireSwitchError(t, err, SettlementModeShadow, "", "SETTLE_RELEASE_RANGE")

	_, err = VerifySettlementSwitch(ctx, nil, SettlementSwitchConfig{Mode: SettlementModeSettled, ReleaseRange: "prod"})
	requireSwitchError(t, err, SettlementModeSettled, "", "SETTLE_RELEASE_APPROVAL_ID")

	_, err = VerifySettlementSwitch(ctx, nil, SettlementSwitchConfig{Mode: SettlementModeSettled, ApprovalID: "apr-1"})
	requireSwitchError(t, err, SettlementModeSettled, "apr-1", "SETTLE_RELEASE_RANGE")

	_, err = VerifySettlementSwitch(ctx, nil, SettlementSwitchConfig{Mode: "unknown"})
	requireSwitchError(t, err, "unknown", "", "unknown settlement consumer mode")

	_, err = VerifySettlementSwitch(ctx, nil, SettlementSwitchConfig{Mode: ""})
	requireSwitchError(t, err, "", "", "unknown settlement consumer mode")
}

func TestSettlementSwitchPaidNeedsNoApprovalRow(t *testing.T) {
	db := settlementSwitchTestDB(t)
	ctx := context.Background()
	approval, err := VerifySettlementSwitch(ctx, db, SettlementSwitchConfig{Mode: SettlementModePaid})
	if err != nil {
		t.Fatalf("paid must start without an approval row: %v", err)
	}
	if approval != (SettlementApproval{}) {
		t.Fatalf("paid approval = %+v, want zero value", approval)
	}
	// paid must not read the table: dropping it still starts paid.
	if _, err := db.Exec("DROP TABLE settlement_switch_approvals"); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySettlementSwitch(ctx, db, SettlementSwitchConfig{Mode: SettlementModePaid}); err != nil {
		t.Fatalf("paid must not depend on settlement_switch_approvals: %v", err)
	}
}

func TestSettlementSwitchShadowMatrix(t *testing.T) {
	db := settlementSwitchTestDB(t)
	ctx := context.Background()
	base := SettlementSwitchConfig{Mode: SettlementModeShadow, ReleaseRange: "prod"}

	cases := []struct {
		name    string
		seed    func(t *testing.T)
		wantID  string
		wantErr []string
	}{
		{
			name:    "missing row",
			seed:    func(t *testing.T) {},
			wantErr: []string{"no APPROVED, unexpired shadow approval row"},
		},
		{
			name: "approved with NULL expiry",
			seed: func(t *testing.T) {
				seedApproval(t, db, "shadow-null", SettlementModeShadow, "APPROVED", "prod", "null")
			},
			wantID: "shadow-null",
		},
		{
			name: "approved with future expiry",
			seed: func(t *testing.T) {
				seedApproval(t, db, "shadow-future", SettlementModeShadow, "APPROVED", "prod", "future")
			},
			wantID: "shadow-future",
		},
		{
			name: "expired",
			seed: func(t *testing.T) {
				seedApproval(t, db, "shadow-expired", SettlementModeShadow, "APPROVED", "prod", "past")
			},
			wantErr: []string{"expired"},
		},
		{
			name: "not approved",
			seed: func(t *testing.T) {
				seedApproval(t, db, "shadow-revoked", SettlementModeShadow, "REVOKED", "prod", "future")
			},
			wantErr: []string{"status=REVOKED"},
		},
		{
			name: "release range mismatch",
			seed: func(t *testing.T) {
				seedApproval(t, db, "shadow-canary", SettlementModeShadow, "APPROVED", "canary", "future")
			},
			wantErr: []string{"release_range", "canary"},
		},
		{
			name: "settled row does not authorize shadow",
			seed: func(t *testing.T) {
				seedApproval(t, db, "settled-only", SettlementModeSettled, "APPROVED", "prod", "future")
			},
			wantErr: []string{"no APPROVED, unexpired shadow approval row"},
		},
		{
			name: "any valid row among invalid rows exists",
			seed: func(t *testing.T) {
				seedApproval(t, db, "shadow-revoked", SettlementModeShadow, "REVOKED", "prod", "future")
				seedApproval(t, db, "shadow-ok", SettlementModeShadow, "APPROVED", "prod", "future")
			},
			wantID: "shadow-ok",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearApprovals(t, db)
			tc.seed(t)
			approval, err := VerifySettlementSwitch(ctx, db, base)
			if len(tc.wantErr) > 0 {
				requireSwitchError(t, err, SettlementModeShadow, "", tc.wantErr...)
				return
			}
			if err != nil {
				t.Fatalf("unexpected rejection: %v", err)
			}
			if approval.ApprovalID != tc.wantID {
				t.Fatalf("approval = %q, want %q", approval.ApprovalID, tc.wantID)
			}
			if approval.TargetMode != SettlementModeShadow || approval.ReleaseRange != "prod" {
				t.Fatalf("approval = %+v", approval)
			}
		})
	}

	// N5 is a config-level rejection, not a DB fallback.
	_, err := VerifySettlementSwitch(ctx, db, SettlementSwitchConfig{
		Mode: SettlementModeShadow, ApprovalID: "shadow-null", ReleaseRange: "prod",
	})
	requireSwitchError(t, err, SettlementModeShadow, "shadow-null", "N5")
}

func TestSettlementSwitchSettledMatrix(t *testing.T) {
	db := settlementSwitchTestDB(t)
	ctx := context.Background()
	base := SettlementSwitchConfig{Mode: SettlementModeSettled, ApprovalID: "settled-1", ReleaseRange: "prod"}

	cases := []struct {
		name    string
		seed    func(t *testing.T)
		wantErr []string
	}{
		{
			name:    "missing row",
			seed:    func(t *testing.T) {},
			wantErr: []string{"not found"},
		},
		{
			name: "valid settled",
			seed: func(t *testing.T) {
				seedApproval(t, db, "settled-1", SettlementModeSettled, "APPROVED", "prod", "future")
			},
		},
		{
			name: "expired",
			seed: func(t *testing.T) {
				seedApproval(t, db, "settled-1", SettlementModeSettled, "APPROVED", "prod", "past")
			},
			wantErr: []string{"expired"},
		},
		{
			name: "NULL expiry",
			seed: func(t *testing.T) {
				seedApproval(t, db, "settled-1", SettlementModeSettled, "APPROVED", "prod", "null")
			},
			wantErr: []string{"expires_at is NULL"},
		},
		{
			name: "not approved",
			seed: func(t *testing.T) {
				seedApproval(t, db, "settled-1", SettlementModeSettled, "REVOKED", "prod", "future")
			},
			wantErr: []string{"status="},
		},
		{
			name: "target mode mismatch",
			seed: func(t *testing.T) {
				seedApproval(t, db, "settled-1", SettlementModeShadow, "APPROVED", "prod", "future")
			},
			wantErr: []string{"target_mode", "shadow"},
		},
		{
			name: "release range mismatch",
			seed: func(t *testing.T) {
				seedApproval(t, db, "settled-1", SettlementModeSettled, "APPROVED", "canary", "future")
			},
			wantErr: []string{"release_range", "canary"},
		},
		{
			name: "different approval id does not match",
			seed: func(t *testing.T) {
				seedApproval(t, db, "settled-other", SettlementModeSettled, "APPROVED", "prod", "future")
			},
			wantErr: []string{"not found"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearApprovals(t, db)
			tc.seed(t)
			approval, err := VerifySettlementSwitch(ctx, db, base)
			if len(tc.wantErr) > 0 {
				requireSwitchError(t, err, SettlementModeSettled, "settled-1", tc.wantErr...)
				return
			}
			if err != nil {
				t.Fatalf("unexpected rejection: %v", err)
			}
			if approval.ApprovalID != "settled-1" || approval.TargetMode != SettlementModeSettled {
				t.Fatalf("approval = %+v", approval)
			}
		})
	}
}

func TestSettlementSwitchReadFailureDoesNotFallBackToPaid(t *testing.T) {
	db := settlementSwitchTestDB(t)
	ctx := context.Background()
	if _, err := db.Exec("DROP TABLE settlement_switch_approvals"); err != nil {
		t.Fatal(err)
	}
	_, err := VerifySettlementSwitch(ctx, db, SettlementSwitchConfig{Mode: SettlementModeShadow, ReleaseRange: "prod"})
	requireSwitchError(t, err, SettlementModeShadow, "", "cannot read settlement_switch_approvals")

	_, err = VerifySettlementSwitch(ctx, db, SettlementSwitchConfig{Mode: SettlementModeSettled, ApprovalID: "settled-1", ReleaseRange: "prod"})
	requireSwitchError(t, err, SettlementModeSettled, "settled-1", "cannot read settlement_switch_approvals")
}
