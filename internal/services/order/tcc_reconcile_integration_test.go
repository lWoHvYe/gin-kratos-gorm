package order

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

// T1.4b1-R2 evidence: the durable TCC reconcile worker settles Payment facts
// that payment.refunded never projected, and projects the order amount in the
// same order-locked transaction. Every test uses real MySQL and never delivers
// a payment.refunded event for the worker path.

func commitTccRefundFacts(t *testing.T, payments *fakePaymentPort, paymentNo, batchID string, items map[string]int64) {
	t.Helper()
	var total int64
	request := &pb.RefundBatchRequest{PaymentNo: paymentNo, BatchId: batchID}
	for refundID, amount := range items {
		total += amount
		request.Items = append(request.Items, &pb.RefundBatchItem{RefundId: refundID, AmountCent: amount})
	}
	request.ExpectedTotalCent = total
	if _, err := payments.RefundBatch(context.Background(), request); err != nil {
		t.Fatalf("seed Payment refund batch: %v", err)
	}
}

func loadOrderRefundProjection(t *testing.T, db *sql.DB, orderNo string) (refunded int64, refundStatus, refundState, status string) {
	t.Helper()
	if err := db.QueryRow(`SELECT refunded_cent, refund_status, refund_state, status
		FROM orders WHERE order_no = ?`, orderNo).
		Scan(&refunded, &refundStatus, &refundState, &status); err != nil {
		t.Fatal(err)
	}
	return refunded, refundStatus, refundState, status
}

// T1.4b1-R2: Payment committed RefundBatch, no event is delivered, and the
// worker settles commitment/application/batch AND projects the order amount.
func TestTccReconcileWorkerSettlesLostEventsAndProjectsOrder(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	batch := admittedTccBatch(t, db, svc, "ORD-REC-LOST", "PAY-REC-LOST", 10000, 6000, 4000)
	commitTccRefundFacts(t, payments, "PAY-REC-LOST", batch.BatchID, map[string]int64{
		tccGroupRefundID("PAY-REC-LOST", "G1"): 6000,
		tccGroupRefundID("PAY-REC-LOST", "G2"): 4000,
	})

	if err := svc.runTccReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if batch := loadTccBatchRow(t, db, "ORD-REC-LOST", "PAY-REC-LOST"); batch.State != refundBatchDone {
		t.Fatalf("batch = %s, want DONE", batch.State)
	}
	for _, group := range []string{"G1", "G2"} {
		refundID := tccGroupRefundID("PAY-REC-LOST", group)
		if state, _ := loadCommitmentState(t, db, refundID); state != commitmentSettled {
			t.Fatalf("%s commitment = %s, want SETTLED", group, state)
		}
		if appState, _, _, _, _ := loadTccApplication(t, ctx, db, refundID); appState != refundApplicationDone {
			t.Fatalf("%s application = %s, want DONE", group, appState)
		}
	}
	refunded, refundStatus, refundState, status := loadOrderRefundProjection(t, db, "ORD-REC-LOST")
	if refunded != 10000 || refundStatus != statusRefunded || refundState != refundStateFull {
		t.Fatalf("order projection = %d/%s/%s, want 10000/REFUNDED/FULL", refunded, refundStatus, refundState)
	}
	if status != statusPaid {
		t.Fatalf("order lifecycle changed to %s, want PAID", status)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-REC-LOST' AND event_type = 'order.refund_projected'`); got != 1 {
		t.Fatalf("order.refund_projected = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_status_logs
		WHERE order_no = 'ORD-REC-LOST' AND operator = 'tcc-reconcile'`); got != 1 {
		t.Fatalf("reconcile status logs = %d, want 1", got)
	}
}

// T1.4b1-R2: a partly refunded active batch stays REFUNDING and PARTIAL; the
// worker does not close the batch or the order before every group is DONE.
func TestTccReconcileWorkerKeepsRefundingForPartialBatch(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	admittedTccBatch(t, db, svc, "ORD-REC-PART", "PAY-REC-PART", 10000, 6000, 4000)

	g1 := tccGroupRefundID("PAY-REC-PART", "G1")
	g2 := tccGroupRefundID("PAY-REC-PART", "G2")
	payments.addRefundFact(&pb.Refund{
		RefundNo: "RFD-G1", RefundId: g1, PaymentNo: "PAY-REC-PART",
		AmountCent: 6000, Status: tccRefundStatusSucceeded, CreatedAtUnix: time.Now().Unix(),
	})
	payments.refunded = 6000

	if err := svc.runTccReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if batch := loadTccBatchRow(t, db, "ORD-REC-PART", "PAY-REC-PART"); batch.State != refundBatchPaying {
		t.Fatalf("batch = %s, want PAYING while G2 is outstanding", batch.State)
	}
	if state, _ := loadCommitmentState(t, db, g1); state != commitmentSettled {
		t.Fatalf("G1 commitment = %s, want SETTLED", state)
	}
	if state, _ := loadCommitmentState(t, db, g2); state != commitmentReserved {
		t.Fatalf("G2 commitment = %s, want RESERVED", state)
	}
	if appState, _, _, _, _ := loadTccApplication(t, ctx, db, g2); appState != refundApplicationProcessing {
		t.Fatalf("G2 application = %s, want PROCESSING", appState)
	}
	refunded, refundStatus, refundState, _ := loadOrderRefundProjection(t, db, "ORD-REC-PART")
	if refunded != 6000 || refundStatus != statusPartiallyRefunded || refundState != refundStateRefunding {
		t.Fatalf("partial projection = %d/%s/%s, want 6000/PARTIALLY_REFUNDED/REFUNDING",
			refunded, refundStatus, refundState)
	}
	// A second pass with no new fact changes nothing.
	var versionBefore int64
	if err := db.QueryRow(`SELECT version FROM orders WHERE order_no = 'ORD-REC-PART'`).Scan(&versionBefore); err != nil {
		t.Fatal(err)
	}
	if err := svc.runTccReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var versionAfter int64
	if err := db.QueryRow(`SELECT version FROM orders WHERE order_no = 'ORD-REC-PART'`).Scan(&versionAfter); err != nil {
		t.Fatal(err)
	}
	if versionAfter != versionBefore {
		t.Fatalf("no-op worker bumped order version %d -> %d", versionBefore, versionAfter)
	}

	payments.addRefundFact(&pb.Refund{
		RefundNo: "RFD-G2", RefundId: g2, PaymentNo: "PAY-REC-PART",
		AmountCent: 4000, Status: tccRefundStatusSucceeded, CreatedAtUnix: time.Now().Unix(),
	})
	payments.refunded = 10000
	if err := svc.runTccReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if batch := loadTccBatchRow(t, db, "ORD-REC-PART", "PAY-REC-PART"); batch.State != refundBatchDone {
		t.Fatalf("batch = %s, want DONE after the last fact", batch.State)
	}
	refunded, refundStatus, refundState, _ = loadOrderRefundProjection(t, db, "ORD-REC-PART")
	if refunded != 10000 || refundStatus != statusRefunded || refundState != refundStateFull {
		t.Fatalf("final projection = %d/%s/%s, want 10000/REFUNDED/FULL", refunded, refundStatus, refundState)
	}
}

// T1.4b1-R2: no Payment fact means no settlement and no projection; the active
// batch stays REFUNDING/PAYING and the worker is a no-op.
func TestTccReconcileWorkerLeavesOutstandingWithoutFact(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	admittedTccBatch(t, db, svc, "ORD-REC-NONE", "PAY-REC-NONE", 10000, 6000, 4000)
	payments.addRefundFact(&pb.Refund{
		RefundNo: "RFD-G1", RefundId: tccGroupRefundID("PAY-REC-NONE", "G1"),
		PaymentNo: "PAY-REC-NONE", AmountCent: 6000, Status: tccRefundStatusPending,
		CreatedAtUnix: time.Now().Unix(),
	})

	if err := svc.runTccReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if batch := loadTccBatchRow(t, db, "ORD-REC-NONE", "PAY-REC-NONE"); batch.State != refundBatchAdmitted {
		t.Fatalf("batch = %s, want ADMITTED (no successful fact)", batch.State)
	}
	for _, group := range []string{"G1", "G2"} {
		if state, _ := loadCommitmentState(t, db, tccGroupRefundID("PAY-REC-NONE", group)); state != commitmentReserved {
			t.Fatalf("%s commitment = %s, want RESERVED", group, state)
		}
	}
	refunded, _, refundState, _ := loadOrderRefundProjection(t, db, "ORD-REC-NONE")
	if refunded != 0 || refundState != refundStateRefunding {
		t.Fatalf("order = %d/%s, want 0/REFUNDING", refunded, refundState)
	}
}

// T1.4b1-R2: concurrent workers are idempotent and never double count the
// amount; a later pass is a no-op.
func TestTccReconcileWorkerConcurrentIsIdempotent(t *testing.T) {
	db := orderTestDB(t)
	payments := &countingPaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	batch := admittedTccBatch(t, db, svc, "ORD-REC-RACE", "PAY-REC-RACE", 10000, 6000, 4000)
	if _, err := payments.RefundBatch(ctx, &pb.RefundBatchRequest{
		PaymentNo: "PAY-REC-RACE", BatchId: batch.BatchID, ExpectedTotalCent: 10000,
		Items: []*pb.RefundBatchItem{
			{RefundId: tccGroupRefundID("PAY-REC-RACE", "G1"), AmountCent: 6000},
			{RefundId: tccGroupRefundID("PAY-REC-RACE", "G2"), AmountCent: 4000},
		},
	}); err != nil {
		t.Fatal(err)
	}

	const workers = 8
	errs := make([]error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			errs[i] = svc.runTccReconcileOnce(ctx)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
	}
	refunded, refundStatus, refundState, _ := loadOrderRefundProjection(t, db, "ORD-REC-RACE")
	if refunded != 10000 || refundStatus != statusRefunded || refundState != refundStateFull {
		t.Fatalf("concurrent projection = %d/%s/%s, want 10000/REFUNDED/FULL",
			refunded, refundStatus, refundState)
	}
	if batch := loadTccBatchRow(t, db, "ORD-REC-RACE", "PAY-REC-RACE"); batch.State != refundBatchDone {
		t.Fatalf("batch = %s, want DONE", batch.State)
	}
	var versionBefore, versionAfter int64
	if err := db.QueryRow(`SELECT version FROM orders WHERE order_no = 'ORD-REC-RACE'`).Scan(&versionBefore); err != nil {
		t.Fatal(err)
	}
	if err := svc.runTccReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT version FROM orders WHERE order_no = 'ORD-REC-RACE'`).Scan(&versionAfter); err != nil {
		t.Fatal(err)
	}
	if versionAfter != versionBefore {
		t.Fatalf("post-DONE worker bumped order version %d -> %d", versionBefore, versionAfter)
	}
}

// T1.4b1-R2: event + worker must not double count. The event projects its own
// cumulative total first; the worker settles the remaining commitment and the
// projection is max(current, SUM(SETTLED commitments)).
func TestTccReconcileWorkerDoesNotDoubleCountWithEvent(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	batch := admittedTccBatch(t, db, svc, "ORD-REC-EVT", "PAY-REC-EVT", 10000, 6000, 4000)
	commitTccRefundFacts(t, payments, "PAY-REC-EVT", batch.BatchID, map[string]int64{
		tccGroupRefundID("PAY-REC-EVT", "G1"): 6000,
		tccGroupRefundID("PAY-REC-EVT", "G2"): 4000,
	})
	// Only G1's event arrives; G2's event is lost.
	if err := applyPaymentRefunded(ctx, svc, paymentRefundedEvent{
		PaymentNo: "PAY-REC-EVT", OrderNo: "ORD-REC-EVT",
		RefundID: tccGroupRefundID("PAY-REC-EVT", "G1"), AmountCent: 6000,
		RefundedCent: 6000, OrderRefundedCent: 6000, CapturedCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	if refunded, _, _, _ := loadOrderRefundProjection(t, db, "ORD-REC-EVT"); refunded != 6000 {
		t.Fatalf("after G1 event = %d, want 6000", refunded)
	}
	if err := svc.runTccReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	refunded, refundStatus, refundState, _ := loadOrderRefundProjection(t, db, "ORD-REC-EVT")
	if refunded != 10000 || refundStatus != statusRefunded || refundState != refundStateFull {
		t.Fatalf("after worker = %d/%s/%s, want 10000/REFUNDED/FULL (no double count)",
			refunded, refundStatus, refundState)
	}
	// A late G2 event is idempotent and does not add the amount again.
	if err := applyPaymentRefunded(ctx, svc, paymentRefundedEvent{
		PaymentNo: "PAY-REC-EVT", OrderNo: "ORD-REC-EVT",
		RefundID: tccGroupRefundID("PAY-REC-EVT", "G2"), AmountCent: 4000,
		RefundedCent: 10000, OrderRefundedCent: 10000, CapturedCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	if refunded, _, _, _ := loadOrderRefundProjection(t, db, "ORD-REC-EVT"); refunded != 10000 {
		t.Fatalf("late event changed refunded to %d, want 10000", refunded)
	}
}
