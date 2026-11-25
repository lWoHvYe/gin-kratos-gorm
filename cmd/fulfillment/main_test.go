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
		"GO_WANT_FULFILLMENT_MAIN":          "1",
		"APP_ENV":                           "dev",
		"MYSQL_DSN":                         dsn,
		"JWT_SECRET":                        "dev-only-change-me-32-characters",
		"NATS_URL":                          "nats://127.0.0.1:1",
		"OTEL_EXPORTER_OTLP_ENDPOINT":       "",
		"FULFILLMENT_CONSUME_ORDER_SETTLED": "",
		"FULFILLMENT_SETTLE_CONSUMER_MODE":  "",
		"SETTLE_RELEASE_APPROVAL_ID":        "",
		"SETTLE_RELEASE_RANGE":              "",
	}
	for k, v := range overrides {
		env[k] = v
	}
	childEnv := make([]string, 0, len(os.Environ())+len(env))
	for _, kv := range os.Environ() {
		key := strings.SplitN(kv, "=", 2)[0]
		if _, ok := env[key]; ok {
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

func TestStartupFailsClosedOnMissingSettledApproval(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "settled",
		"SETTLE_RELEASE_APPROVAL_ID":       "missing-approval",
		"SETTLE_RELEASE_RANGE":             "prod",
	})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; output:\n%s", code, output)
	}
	for _, want := range []string{"settlement switch verification failed", "approval row not found"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "order.settled") && strings.Contains(output, "consumer started") {
		t.Fatalf("a consumer started despite the failed approval check:\n%s", output)
	}
}

func TestStartupFailsClosedOnMissingShadowApproval(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "shadow",
		"SETTLE_RELEASE_RANGE":             "prod",
	})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; output:\n%s", code, output)
	}
	if !strings.Contains(output, "settlement switch verification failed") ||
		!strings.Contains(output, "no APPROVED, unexpired shadow approval row") {
		t.Fatalf("unexpected output:\n%s", output)
	}
}

func TestStartupRejectsApprovalIDOutsideSettled(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "paid",
		"SETTLE_RELEASE_APPROVAL_ID":       "unexpected-approval",
	})
	if code == 0 {
		t.Fatalf("paid with an approval id exited 0; output:\n%s", output)
	}
	if !strings.Contains(output, "SETTLE_RELEASE_APPROVAL_ID must be empty") {
		t.Fatalf("output missing N5 diagnostic:\n%s", output)
	}
}

func TestStartupRefusesShadowWithoutConsumerImplementation(t *testing.T) {
	dsn, cleanup := fulfillmentMainTestDB(t)
	defer cleanup()
	// A valid shadow approval must pass the R0.4b gate but still not start the
	// paid consumer: T4.4 observation is not implemented yet.
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO settlement_switch_approvals
		(approval_id, target_mode, approved_by, approved_at, release_range, evidence_ref, expires_at, status)
		VALUES ('shadow-ok', 'shadow', 'director', NOW(3), 'prod', 'evidence', DATE_ADD(NOW(3), INTERVAL 1 HOUR), 'APPROVED')`); err != nil {
		t.Fatalf("seed shadow approval: %v", err)
	}
	output, code := startFulfillmentChild(t, dsn, map[string]string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE": "shadow",
		"SETTLE_RELEASE_RANGE":             "prod",
	})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; output:\n%s", code, output)
	}
	if !strings.Contains(output, "settlement switch verified") ||
		!strings.Contains(output, "shadow settlement consumer is not implemented") {
		t.Fatalf("unexpected output:\n%s", output)
	}
}
