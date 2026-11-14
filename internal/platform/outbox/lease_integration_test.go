package outbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"gin-kratos-gorm/internal/platform/mq"

	mysqldriver "github.com/go-sql-driver/mysql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// integrationDB connects to TEST_MYSQL_DSN (e.g.
// root:root@tcp(127.0.0.1:3306)/mall_outbox_test?parseTime=true) and prepares
// an empty outbox table. The database is created if it does not exist.
func integrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run the outbox integration tests")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse TEST_MYSQL_DSN: %v", err)
	}
	if cfg.DBName == "" {
		t.Fatal("TEST_MYSQL_DSN must name a database")
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
	db.SetMaxOpenConns(8)
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM outbox_events")
		_ = db.Close()
	})
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS outbox_events (
		id BIGINT NOT NULL AUTO_INCREMENT,
		event_id VARCHAR(64) NOT NULL,
		topic VARCHAR(128) NOT NULL,
		aggregate_type VARCHAR(64) NOT NULL,
		aggregate_id VARCHAR(64) NOT NULL,
		event_type VARCHAR(64) NOT NULL,
		payload JSON NOT NULL,
		attempts INT NOT NULL DEFAULT 0,
		last_error VARCHAR(1024) NOT NULL DEFAULT '',
		next_attempt_at DATETIME(3) NULL,
		published_at DATETIME(3) NULL,
		dead_lettered_at DATETIME(3) NULL,
		lease_owner VARCHAR(128) NULL,
		lease_until DATETIME(3) NULL,
		traceparent VARCHAR(256) NULL,
		created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		PRIMARY KEY (id),
		UNIQUE KEY uk_outbox_event_id (event_id),
		KEY idx_outbox_lease (published_at, dead_lettered_at, lease_until, next_attempt_at, id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		t.Fatalf("create outbox table: %v", err)
	}
	if _, err := db.Exec("DELETE FROM outbox_events"); err != nil {
		t.Fatal(err)
	}
	return db
}

func insertEvent(t *testing.T, db *sql.DB, eventID string) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO outbox_events
		(event_id, topic, aggregate_type, aggregate_id, event_type, payload)
		VALUES (?, 'shop.test.v1', 'test', '1', 'test.created', '{"ok":true}')`, eventID)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestClaimLeasesRowsAndSkipsLeased(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	for _, id := range []string{"e1", "e2", "e3"} {
		insertEvent(t, db, id)
	}
	claimed, err := claimBatch(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 3 {
		t.Fatalf("claimed %d rows, want 3", len(claimed))
	}
	var owner string
	var until time.Time
	if err := db.QueryRow("SELECT lease_owner, lease_until FROM outbox_events WHERE event_id = 'e1'").Scan(&owner, &until); err != nil {
		t.Fatal(err)
	}
	if owner != relayOwner || until.Before(time.Now()) {
		t.Fatalf("lease = %q until %s", owner, until)
	}

	// A second relay must not steal a live lease.
	again, err := claimBatch(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("second claim got %d rows, want 0", len(again))
	}

	// An expired lease is reclaimable.
	if _, err := db.Exec("UPDATE outbox_events SET lease_until = DATE_SUB(NOW(3), INTERVAL 1 MINUTE) WHERE event_id = 'e1'"); err != nil {
		t.Fatal(err)
	}
	reclaimed, err := claimBatch(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(reclaimed) != 1 || reclaimed[0].id == 0 {
		t.Fatalf("reclaimed %d rows, want 1", len(reclaimed))
	}
}

func TestMarkPublishedRequiresTheSameLeaseOwner(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	id := insertEvent(t, db, "e1")
	claimed, err := claimBatch(ctx, db)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v (%d)", err, len(claimed))
	}

	previous := relayOwner
	relayOwner = "someone-else"
	ok, err := markPublished(ctx, db, id)
	relayOwner = previous
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("a foreign owner must not complete a leased row")
	}
	ok, err = markPublished(ctx, db, id)
	if err != nil || !ok {
		t.Fatalf("owner completion = %v, %v", ok, err)
	}
	var published sql.NullTime
	if err := db.QueryRow("SELECT published_at FROM outbox_events WHERE id = ?", id).Scan(&published); err != nil {
		t.Fatal(err)
	}
	if !published.Valid {
		t.Fatal("published_at was not set")
	}
}

func TestMarkFailedBacksOffThenDeadLetters(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	id := insertEvent(t, db, "e1")
	claimed, err := claimBatch(ctx, db)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v", err)
	}
	if err := markFailed(ctx, db, claimed[0], errBoom, testLogger()); err != nil {
		t.Fatal(err)
	}
	var attempts int
	var next sql.NullTime
	var lease sql.NullString
	if err := db.QueryRow("SELECT attempts, next_attempt_at, lease_owner FROM outbox_events WHERE id = ?", id).Scan(&attempts, &next, &lease); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || !next.Valid || lease.Valid {
		t.Fatalf("after failure: attempts=%d next=%v lease=%v", attempts, next.Valid, lease.Valid)
	}

	// Exhaust the retry budget.
	if _, err := db.Exec("UPDATE outbox_events SET attempts = 19, lease_owner = ?, lease_until = DATE_ADD(NOW(3), INTERVAL 1 MINUTE) WHERE id = ?", relayOwner, id); err != nil {
		t.Fatal(err)
	}
	if err := markFailed(ctx, db, claimedEvent{id: id, topic: "shop.test.v1", attempts: 19}, errBoom, testLogger()); err != nil {
		t.Fatal(err)
	}
	var dead sql.NullTime
	if err := db.QueryRow("SELECT dead_lettered_at FROM outbox_events WHERE id = ?", id).Scan(&dead); err != nil {
		t.Fatal(err)
	}
	if !dead.Valid {
		t.Fatal("row was not dead-lettered after max attempts")
	}
}

func TestPruneRemovesOnlyOldTerminalRows(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	published := insertEvent(t, db, "old-published")
	pending := insertEvent(t, db, "pending")
	if _, err := db.Exec("UPDATE outbox_events SET published_at = DATE_SUB(NOW(3), INTERVAL 30 DAY) WHERE id = ?", published); err != nil {
		t.Fatal(err)
	}
	n, err := pruneOnce(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pruned %d rows, want 1", n)
	}
	var remaining int
	if err := db.QueryRow("SELECT COUNT(*) FROM outbox_events WHERE id = ?", pending).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatal("pending row was pruned")
	}
}

var errBoom = errors.New("boom")

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestAppendPersistsTraceparentForTheRelay(t *testing.T) {
	db := integrationDB(t)
	previous := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(previous)

	provider := sdktrace.NewTracerProvider()
	defer func() { _ = provider.Shutdown(context.Background()) }()
	ctx, span := provider.Tracer("test").Start(context.Background(), "request")
	defer span.End()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Append(ctx, tx, Event{Topic: "shop.test.v1", AggregateType: "test", AggregateID: "1", EventType: "test.created", Payload: map[string]any{"ok": true}}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	var stored sql.NullString
	if err := db.QueryRow("SELECT traceparent FROM outbox_events WHERE topic = 'shop.test.v1'").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !stored.Valid || stored.String == "" {
		t.Fatal("Append did not persist the producing traceparent")
	}
	claimed, err := claimBatch(context.Background(), db)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v (%d)", err, len(claimed))
	}
	if claimed[0].traceparent != stored.String {
		t.Fatalf("claimed traceparent = %q, want %q", claimed[0].traceparent, stored.String)
	}
	// Without an active span the column stays NULL.
	tx, err = db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Append(context.Background(), tx, Event{Topic: "shop.test.v2", AggregateType: "test", AggregateID: "2", EventType: "test.created", Payload: map[string]any{"ok": true}}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var nullCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM outbox_events WHERE topic = 'shop.test.v2' AND traceparent IS NULL").Scan(&nullCount); err != nil {
		t.Fatal(err)
	}
	if nullCount != 1 {
		t.Fatalf("expected a NULL traceparent without a span, got %d", nullCount)
	}
}

func natsClient(t *testing.T) *mq.Client {
	t.Helper()
	url := os.Getenv("TEST_NATS_URL")
	if url == "" {
		t.Skip("set TEST_NATS_URL to run relay integration tests")
	}
	client, err := mq.Connect(url)
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

func TestRelayPublishesAndMarks(t *testing.T) {
	db := integrationDB(t)
	client := natsClient(t)
	ctx := context.Background()
	subject := fmt.Sprintf("shop.test.outbox.%d", time.Now().UnixNano())
	id := insertEvent(t, db, "relay-"+subject)
	if _, err := db.Exec("UPDATE outbox_events SET topic = ? WHERE id = ?", subject, id); err != nil {
		t.Fatal(err)
	}
	n, err := relayOnce(ctx, db, client, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("published %d, want 1", n)
	}
	var published sql.NullTime
	if err := db.QueryRow("SELECT published_at FROM outbox_events WHERE id = ?", id).Scan(&published); err != nil {
		t.Fatal(err)
	}
	if !published.Valid {
		t.Fatal("row was not marked published")
	}
	msg, err := client.JS.GetLastMsg("MALL", subject)
	if err != nil {
		t.Fatalf("message missing from stream: %v", err)
	}
	if len(msg.Data) == 0 {
		t.Fatal("published message has no payload")
	}
}

func TestRelayPublishFailureBacksOff(t *testing.T) {
	db := integrationDB(t)
	client := natsClient(t)
	// A closed client makes PublishMsg fail without touching the server.
	client.Close()
	id := insertEvent(t, db, "relay-failure")
	n, err := relayOnce(context.Background(), db, client, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("published %d on a closed client", n)
	}
	var attempts int
	var next sql.NullTime
	var lease sql.NullString
	if err := db.QueryRow("SELECT attempts, next_attempt_at, lease_owner FROM outbox_events WHERE id = ?", id).Scan(&attempts, &next, &lease); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || !next.Valid || lease.Valid {
		t.Fatalf("attempts=%d next=%v lease=%v", attempts, next.Valid, lease.Valid)
	}
}
