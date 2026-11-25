package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// TestFulfillmentMainHelper runs main() in a child process so the startup
// failure can be observed as a real non-zero exit code. It is a no-op in the
// parent test process.
func TestFulfillmentMainHelper(t *testing.T) {
	if os.Getenv("GO_WANT_FULFILLMENT_MAIN") != "1" {
		return
	}
	main()
}

// settlementEnvKeys are the four switch variables the helper scrubs from the
// inherited environment unless a case sets them explicitly. This is what lets a
// case exercise the real "unset" semantics (paid default) instead of an empty
// string, which the config rejects as an unknown mode.
var settlementEnvKeys = []string{
	"FULFILLMENT_CONSUME_ORDER_SETTLED",
	"FULFILLMENT_SETTLE_CONSUMER_MODE",
	"SETTLE_RELEASE_APPROVAL_ID",
	"SETTLE_RELEASE_RANGE",
}

func fulfillmentMainTestDB(t *testing.T) (string, func()) {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run the fulfillment startup exit test")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil || cfg.DBName == "" {
		t.Fatalf("TEST_MYSQL_DSN must name a database: %v", err)
	}
	name := fmt.Sprintf("mall_r04_main_%d", time.Now().UnixNano())
	adminCfg := *cfg
	adminCfg.DBName = ""
	admin, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("create startup test database: %v", err)
	}
	dbCfg := *cfg
	dbCfg.DBName = name
	db, err := sql.Open("mysql", dbCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if err := applyFulfillmentMigration(db, "0010_settlement_shadow_switch.sql"); err != nil {
		t.Fatalf("apply settlement switch migration: %v", err)
	}
	cleanup := func() {
		_ = db.Close()
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + name + "`")
		_ = admin.Close()
	}
	return dbCfg.FormatDSN(), cleanup
}

func applyFulfillmentMigration(db *sql.DB, name string) error {
	path := filepath.Join("..", "..", "migrations", "fulfillment", name)
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cleaned strings.Builder
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		cleaned.WriteString(line)
		cleaned.WriteString("\n")
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	for _, statement := range strings.Split(cleaned.String(), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := conn.ExecContext(context.Background(), statement); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func startFulfillmentChild(t *testing.T, dsn string, overrides map[string]string) (string, int) {
	t.Helper()
	env := map[string]string{
		"GO_WANT_FULFILLMENT_MAIN":    "1",
		"APP_ENV":                     "dev",
		"MYSQL_DSN":                   dsn,
		"JWT_SECRET":                  "dev-only-change-me-32-characters",
		"NATS_URL":                    "nats://127.0.0.1:1",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "",
	}
	for k, v := range overrides {
		env[k] = v
	}
	scrub := func(key string) bool {
		if _, ok := env[key]; ok {
			return true
		}
		for _, k := range settlementEnvKeys {
			if key == k {
				return true
			}
		}
		return false
	}
	childEnv := make([]string, 0, len(os.Environ())+len(env))
	for _, kv := range os.Environ() {
		if scrub(strings.SplitN(kv, "=", 2)[0]) {
			continue
		}
		childEnv = append(childEnv, kv)
	}
	for k, v := range env {
		childEnv = append(childEnv, k+"="+v)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestFulfillmentMainHelper$")
	cmd.Env = childEnv
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("startup succeeded, want non-zero exit; output:\n%s", output)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("child did not exit normally: %v\n%s", err, output)
	}
	return string(output), exitErr.ExitCode()
}

func seedFulfillmentApproval(t *testing.T, dsn, id, target, status, releaseRange, expirySQL string) {
	t.Helper()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	retired, removal := "NULL", "NULL"
	if target == "settled" {
		retired, removal = "NOW(3)", "'2026-12-25'"
	}
	query := fmt.Sprintf(`INSERT INTO settlement_switch_approvals
		(approval_id, target_mode, approved_by, approved_at, release_range, evidence_ref,
		 expires_at, order_paid_admission_retired_at, order_paid_subject_removal_date, status)
		VALUES (?, ?, 'director', NOW(3), ?, 'evidence', %s, %s, %s, ?)`, expirySQL, retired, removal)
	if _, err := db.Exec(query, id, target, releaseRange, status); err != nil {
		t.Fatalf("seed approval %s: %v", id, err)
	}
}

// requireExit asserts the exact child exit code (2 config, 3 approval gate,
// 4 shadow runtime) and the refusal log fields. The JSON slog record carries
// structured mode/approval_id/reason fields; the error field repeats
// "mode=... approval_id=... reason=..." so both forms are observable.
func requireExit(t *testing.T, output string, code, wantCode int, wantMode, wantApproval, wantReason string) {
	t.Helper()
	if code != wantCode {
		t.Fatalf("exit code = %d, want %d; output:\n%s", code, wantCode, output)
	}
	for _, want := range []string{
		`"mode":"` + wantMode + `"`,
		`"approval_id":"` + wantApproval + `"`,
		`"reason":"` + wantReason + `"`,
		`mode=\"` + wantMode + `\"`,
		`approval_id=\"` + wantApproval + `\"`,
		`reason=` + wantReason,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	// The gate must run before NATS/dial/consumers: a gate rejection never
	// reaches the infrastructure path.
	if strings.Contains(output, "connect nats") || strings.Contains(output, "dial order") {
		t.Fatalf("startup continued past the settlement gate:\n%s", output)
	}
}

func TestStartupFailsClosedOnMissingSettledApproval(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "settled",
		"SETTLE_RELEASE_APPROVAL_ID":       "missing-approval",
		"SETTLE_RELEASE_RANGE":             "prod",
	})
	requireExit(t, output, code, 3, "settled", "missing-approval", "APPROVAL_ROW_MISSING")
	if !strings.Contains(output, "approval row not found") {
		t.Fatalf("output missing the human detail:\n%s", output)
	}
}

func TestStartupFailsClosedOnMissingShadowApproval(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "shadow",
		"SETTLE_RELEASE_RANGE":             "prod",
	})
	requireExit(t, output, code, 3, "shadow", "", "APPROVAL_ROW_MISSING")
	if !strings.Contains(output, "no shadow approval row exists") {
		t.Fatalf("output missing the human detail:\n%s", output)
	}
}

func TestStartupRejectsSettledNonApprovedRow(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	seedFulfillmentApproval(t, dsn, "settled-revoked", "settled", "REVOKED", "prod", "DATE_ADD(NOW(3), INTERVAL 1 HOUR)")
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "settled",
		"SETTLE_RELEASE_APPROVAL_ID":       "settled-revoked",
		"SETTLE_RELEASE_RANGE":             "prod",
	})
	requireExit(t, output, code, 3, "settled", "settled-revoked", "APPROVAL_NOT_APPROVED")
}

func TestStartupRejectsExpiredSettledRow(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	seedFulfillmentApproval(t, dsn, "settled-expired", "settled", "APPROVED", "prod", "DATE_SUB(NOW(3), INTERVAL 1 HOUR)")
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "settled",
		"SETTLE_RELEASE_APPROVAL_ID":       "settled-expired",
		"SETTLE_RELEASE_RANGE":             "prod",
	})
	requireExit(t, output, code, 3, "settled", "settled-expired", "APPROVAL_EXPIRED")
}

func TestStartupRejectsReleaseRangeMismatch(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	seedFulfillmentApproval(t, dsn, "settled-canary", "settled", "APPROVED", "canary", "DATE_ADD(NOW(3), INTERVAL 1 HOUR)")
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "settled",
		"SETTLE_RELEASE_APPROVAL_ID":       "settled-canary",
		"SETTLE_RELEASE_RANGE":             "prod",
	})
	requireExit(t, output, code, 3, "settled", "settled-canary", "RELEASE_RANGE_MISMATCH")
}

func TestStartupFailsClosedWhenApprovalTableIsUnreadable(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TABLE settlement_switch_approvals"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "settled",
		"SETTLE_RELEASE_APPROVAL_ID":       "any",
		"SETTLE_RELEASE_RANGE":             "prod",
	})
	requireExit(t, output, code, 3, "settled", "any", "APPROVAL_DB_UNAVAILABLE")
}

func TestStartupRejectsApprovalIDOutsideSettled(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "paid",
		"SETTLE_RELEASE_APPROVAL_ID":       "unexpected-approval",
	})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (config panic); output:\n%s", code, output)
	}
	if !strings.Contains(output, "N5_APPROVAL_ID_MODE_MISMATCH") {
		t.Fatalf("output missing the N5 reason token:\n%s", output)
	}
}

func TestStartupRejectsUnknownModeWithToken(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "settled-prod",
	})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (config panic); output:\n%s", code, output)
	}
	if !strings.Contains(output, "MODE_UNKNOWN") {
		t.Fatalf("output missing the MODE_UNKNOWN token:\n%s", output)
	}
}

func TestStartupRejectsShadowWithoutRange(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "shadow",
	})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (config panic); output:\n%s", code, output)
	}
	if !strings.Contains(output, "RELEASE_RANGE_MISSING") {
		t.Fatalf("output missing the RELEASE_RANGE_MISSING token:\n%s", output)
	}
}

func TestStartupLegacySettledBoolRequiresApproval(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_CONSUME_ORDER_SETTLED": "true",
	})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (config panic); output:\n%s", code, output)
	}
	if !strings.Contains(output, "SETTLE_RELEASE_APPROVAL_ID is required") {
		t.Fatalf("legacy true did not map to a gated settled request:\n%s", output)
	}
}

func TestStartupDefaultPaidPassesGate(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	// No settlement env at all: paid is the only allowed default. The gate must
	// pass and the process must fail later on the unreachable NATS (exit 1),
	// not be rejected by the gate.
	output, code := startFulfillmentChild(t, dsn, nil)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (infrastructure); output:\n%s", code, output)
	}
	if strings.Contains(output, "settlement switch verification failed") {
		t.Fatalf("default paid was rejected by the settlement gate:\n%s", output)
	}
	if !strings.Contains(output, "connect nats") {
		t.Fatalf("default paid did not proceed to the NATS connect:\n%s", output)
	}
}

func TestStartupRefusesShadowWithoutConsumerImplementation(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	// A valid shadow approval must pass the R0.4b gate but still not start the
	// paid consumer: T4.4 observation is not implemented yet.
	seedFulfillmentApproval(t, dsn, "shadow-ok", "shadow", "APPROVED", "prod", "DATE_ADD(NOW(3), INTERVAL 1 HOUR)")
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "shadow",
		"SETTLE_RELEASE_RANGE":             "prod",
	})
	requireExit(t, output, code, 4, "shadow", "shadow-ok", "T4_4_SHADOW_NOT_DELIVERED")
	if !strings.Contains(output, "settlement switch verified") {
		t.Fatalf("shadow gate did not record the verified approval:\n%s", output)
	}
}
