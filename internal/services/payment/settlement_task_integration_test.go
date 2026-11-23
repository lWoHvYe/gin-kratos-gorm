package payment

import (
	"context"
	"testing"
	"time"
)

func TestSettlementTaskClaimFinish(t *testing.T) {
	db := paymentTestDB(t)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS payment_settlement_tasks (
		payment_no VARCHAR(64) NOT NULL PRIMARY KEY, order_no VARCHAR(64) NOT NULL,
		state VARCHAR(16) NOT NULL DEFAULT 'PENDING', attempts INT NOT NULL DEFAULT 0,
		lease_owner VARCHAR(64) NOT NULL DEFAULT '', lease_until DATETIME(3) NULL,
		dtm_gid VARCHAR(128) NOT NULL DEFAULT '', dtm_status VARCHAR(32) NOT NULL DEFAULT '',
		last_error VARCHAR(1024) NOT NULL DEFAULT '', version BIGINT NOT NULL DEFAULT 1,
		created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3))`); err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(`DELETE FROM payment_settlement_tasks`)
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := newRepo(db).ensureSettlementTask(ctx, tx, "PAY-ST", "ORD-ST", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	task, err := claimSettlementTask(ctx, db, "test", time.Minute)
	if err != nil || task == nil || task.PaymentNo != "PAY-ST" {
		t.Fatalf("claim = (%+v, %v)", task, err)
	}
	// A second claim must not steal the leased task.
	if second, err := claimSettlementTask(ctx, db, "other", time.Minute); err != nil || second != nil {
		t.Fatalf("second claim = (%+v, %v), want nil", second, err)
	}
	if err := finishSettlementTask(ctx, db, "PAY-ST", settlementDone, ""); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM payment_settlement_tasks WHERE payment_no='PAY-ST'`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != settlementDone {
		t.Fatalf("state = %q, want DONE", state)
	}
}
