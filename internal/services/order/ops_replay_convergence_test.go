package order

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/events"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

// OPS-P5b (test-only): prove the real Order `aftersale.completed` consumer
// converges the group projection (and never invents an admission result), and
// that the existing TCC re-drive path converges a re-armed batch to PRE/POST
// without a duplicate payment. These are the two gaps the Reviewer found in the
// OPS-P4 drill (synthetic DLQ wrapper / CAS-only re-arm).

func opsNATSClient(t *testing.T) *mq.Client {
	t.Helper()
	if os.Getenv("TEST_MYSQL_DSN") == "" || os.Getenv("TEST_NATS_URL") == "" {
		t.Skip("set TEST_MYSQL_DSN and TEST_NATS_URL")
	}
	client, err := mq.Connect(os.Getenv("TEST_NATS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func opsWait(t *testing.T, label string, want func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if want() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("%s did not converge", label)
}

// D1: real `shop.aftersale.completed.v1` consumer over JetStream. The event
// projects the group to REFUNDED but never changes the admission decision
// (PRE/REVIEW stay owned by admission). Duplicate delivery is idempotent; a
// malformed event lands in the DLQ without touching the projection.
func TestOpsReplayAfterSaleCompletedConsumerConvergence(t *testing.T) {
	client := opsNATSClient(t)
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go RunAfterSaleConsumer(ctx, client, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	time.Sleep(500 * time.Millisecond)

	// Unique ids avoid replayed history from earlier runs tainting the case.
	suffix := time.Now().Format("150405.000000")
	orderNo := "ORD-OPSC-" + suffix
	refundID := "REF-OPSC-" + suffix
	ac50SeedOrder(t, db, orderNo, 10000)
	ac50SeedGroup(t, db, orderNo, "SHIPPED", 0)
	if _, err := db.Exec(`UPDATE order_fulfillment_groups SET refund_state = 'REFUNDING' WHERE order_no = ? AND group_no = 'G1'`, orderNo); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO order_refund_admissions
		(refund_id, order_no, group_no, result, state, amount_cent, version)
		VALUES (?, ?, 'G1', ?, ?, 10000, 1)`, refundID, orderNo, refundReviewRequired, refundPending); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO order_refund_commitments
		(order_no,group_no,refund_id,amount_cent,state,version) VALUES (?, 'G1', ?, 10000, 'RESERVED', 1)`, orderNo, refundID); err != nil {
		t.Fatal(err)
	}
	beforeRefunded := countRows(t, db, `SELECT refunded_cent FROM orders WHERE order_no = ?`, orderNo)
	releasesBefore := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = ?`, orderNo, events.RefundReleaseEventType)

	var dlqCount atomic.Int64
	if _, err := client.QueueSubscribe("shop.aftersale.completed.v1.dlq", "", func(*nats.Msg) {
		dlqCount.Add(1)
	}); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(events.AfterSaleCompleted{
		AfterSaleNo: refundID, OrderNo: orderNo, GroupNo: "G1",
		Status: "REFUNDED", AmountCent: 10000, EffectiveAmountCent: 10000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Publish(ctx, "shop.aftersale.completed.v1", payload); err != nil {
		t.Fatal(err)
	}
	opsWait(t, "group REFUNDED", func() bool {
		return countRows(t, db, `SELECT COUNT(*) FROM order_fulfillment_groups WHERE order_no = ? AND group_no = 'G1' AND refund_state = 'REFUNDED'`, orderNo) == 1
	})
	var groupVersion int64
	if err := db.QueryRow(`SELECT version FROM order_fulfillment_groups WHERE order_no = ? AND group_no = 'G1'`, orderNo).Scan(&groupVersion); err != nil {
		t.Fatal(err)
	}
	// Duplicate delivery: idempotent (no version bump, no admission rewrite).
	if err := client.Publish(ctx, "shop.aftersale.completed.v1", payload); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	var afterVersion int64
	if err := db.QueryRow(`SELECT version FROM order_fulfillment_groups WHERE order_no = ? AND group_no = 'G1'`, orderNo).Scan(&afterVersion); err != nil {
		t.Fatal(err)
	}
	if afterVersion != groupVersion {
		t.Fatalf("duplicate event bumped group version: %d -> %d", groupVersion, afterVersion)
	}
	result, state, _ := ac50Admission(t, db, refundID)
	if result != refundReviewRequired || state != refundPending {
		t.Fatalf("consumer changed admission to %s/%s, want REVIEW/PENDING", result, state)
	}
	if got := countRows(t, db, `SELECT refunded_cent FROM orders WHERE order_no = ?`, orderNo); got != beforeRefunded {
		t.Fatalf("consumer changed refunded_cent: %d -> %d", beforeRefunded, got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = ?`, orderNo, events.RefundReleaseEventType); got != releasesBefore {
		t.Fatalf("consumer wrote release events: %d", got)
	}

	// Malformed delivery -> Permanent -> DLQ; projection untouched.
	if err := client.Publish(ctx, "shop.aftersale.completed.v1", []byte(`{`)); err != nil {
		t.Fatal(err)
	}
	opsWait(t, "DLQ", func() bool { return dlqCount.Load() > 0 })
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_fulfillment_groups WHERE order_no = ? AND group_no = 'G1' AND refund_state = 'REFUNDED'`, orderNo); got != 1 {
		t.Fatalf("malformed event changed the projection")
	}
}

// D5: re-arm then the existing TCC re-drive path (`TccCancelMarkPaid`) must
// converge to PRE and pay exactly once; a repeat is idempotent. A shipped group
// converges to POST instead of PRE.
func TestOpsReplayTCCRearmReDriveConvergence(t *testing.T) {
	ctx := context.Background()
	svc, payments, ff, refundID := tccReviewFixture(t, "ORD-OPSR-PRE", "PAY-OPSR-PRE")
	db := svc.db
	req := &pb.TccPaymentRequest{OrderNo: "ORD-OPSR-PRE", PaymentNo: "PAY-OPSR-PRE", PaidCent: 10000}
	if _, err := svc.TccCancelMarkPaid(ctx, req); err == nil {
		t.Fatal("expected MANUAL_HOLD")
	}
	batch := loadTccBatchRow(t, db, "ORD-OPSR-PRE", "PAY-OPSR-PRE")
	if _, err := db.Exec(`UPDATE order_refund_admissions SET result = ?, version = version + 1
		WHERE refund_id = ? AND result = ? AND state = ?`, refundPending, refundID, refundReviewRequired, refundPending); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE refund_batches SET state = ?, version = version + 1
		WHERE batch_id = ? AND state = ?`, refundBatchAdmitting, batch.BatchID, refundBatchManualHold); err != nil {
		t.Fatal(err)
	}
	ff.setStatus("CANCELLED")
	if _, err := svc.TccCancelMarkPaid(ctx, req); err != nil {
		t.Fatalf("re-drive: %v", err)
	}
	result, state, _ := ac50Admission(t, db, refundID)
	if result != refundResultPre || state != refundResultPre {
		t.Fatalf("re-drive admission = %s/%s, want PRE/PRE", result, state)
	}
	if got := len(payments.batchCalls); got != 1 {
		t.Fatalf("payment batches = %d, want 1", got)
	}
	// Repeat is idempotent: no second refund batch.
	if _, err := svc.TccCancelMarkPaid(ctx, req); err != nil {
		t.Fatal(err)
	}
	if got := len(payments.batchCalls); got != 1 {
		t.Fatalf("repeat paid again: %d batches", got)
	}
}

func TestOpsReplayTCCRearmPostConvergence(t *testing.T) {
	ctx := context.Background()
	svc, payments, ff, refundID := tccReviewFixture(t, "ORD-OPSR-POST", "PAY-OPSR-POST")
	db := svc.db
	req := &pb.TccPaymentRequest{OrderNo: "ORD-OPSR-POST", PaymentNo: "PAY-OPSR-POST", PaidCent: 10000}
	if _, err := svc.TccCancelMarkPaid(ctx, req); err == nil {
		t.Fatal("expected MANUAL_HOLD")
	}
	batch := loadTccBatchRow(t, db, "ORD-OPSR-POST", "PAY-OPSR-POST")
	if _, err := db.Exec(`UPDATE order_fulfillment_groups SET status = 'SHIPPED' WHERE order_no = ? AND group_no = 'G1'`, "ORD-OPSR-POST"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE order_refund_admissions SET result = ?, version = version + 1
		WHERE refund_id = ? AND result = ? AND state = ?`, refundPending, refundID, refundReviewRequired, refundPending); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE refund_batches SET state = ?, version = version + 1
		WHERE batch_id = ? AND state = ?`, refundBatchAdmitting, batch.BatchID, refundBatchManualHold); err != nil {
		t.Fatal(err)
	}
	ff.setStatus("ALREADY_SHIPPED")
	if _, err := svc.TccCancelMarkPaid(ctx, req); err != nil {
		t.Fatalf("re-drive POST: %v", err)
	}
	result, state, _ := ac50Admission(t, db, refundID)
	if result != refundResultPost || state != refundResultPost {
		t.Fatalf("re-drive admission = %s/%s, want POST/POST", result, state)
	}
	if got := len(payments.batchCalls); got != 1 {
		t.Fatalf("payment batches = %d, want 1", got)
	}
}
