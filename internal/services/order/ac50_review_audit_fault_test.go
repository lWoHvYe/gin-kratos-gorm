package order

import (
	"context"
	"sync/atomic"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/events"

	"google.golang.org/grpc"
)

// T5a/T6d (AC-69/AC-72, ADR-0007 Amd1 AC-50 Addendum 0.6 §8.3/§8.4):
// the admission row is never deleted, created_at is the first-review time,
// version is monotonic, and the canonical batch mirror survives reconcile and
// the audited re-arm (audit first, clear last).
func TestAC50ReviewAuditAndRearmRetention(t *testing.T) {
	ctx := context.Background()
	svc, _, ff, refundID := tccReviewFixture(t, "ORD-AUDIT", "PAY-AUDIT")
	db := svc.db
	req := &pb.TccPaymentRequest{OrderNo: "ORD-AUDIT", PaymentNo: "PAY-AUDIT", PaidCent: 10000}
	if _, err := svc.TccCancelMarkPaid(ctx, req); err == nil {
		t.Fatal("expected MANUAL_HOLD")
	}
	batch := loadTccBatchRow(t, db, "ORD-AUDIT", "PAY-AUDIT")
	var createdBefore string
	var versionBefore int64
	if err := db.QueryRow(`SELECT created_at, version FROM order_refund_admissions WHERE refund_id = ?`, refundID).
		Scan(&createdBefore, &versionBefore); err != nil {
		t.Fatal(err)
	}
	var batchVersion int64
	var lastErrorBefore string
	if err := db.QueryRow(`SELECT version, last_error FROM refund_batches WHERE batch_id = ?`, batch.BatchID).
		Scan(&batchVersion, &lastErrorBefore); err != nil {
		t.Fatal(err)
	}

	// O20b: reconcile never clears the batch audit text or advances MANUAL_HOLD.
	if err := svc.runTccReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var state, lastError string
	if err := db.QueryRow(`SELECT state, last_error FROM refund_batches WHERE batch_id = ?`, batch.BatchID).
		Scan(&state, &lastError); err != nil {
		t.Fatal(err)
	}
	if state != refundBatchManualHold || lastError != lastErrorBefore {
		t.Fatalf("reconcile changed the held batch: %s/%q", state, lastError)
	}

	// Re-arm with the enforced order: the mirror already exists and is verified
	// before the versioned CAS clears the result.
	ff.setStatus("CANCELLED")
	res, err := db.Exec(`UPDATE order_refund_admissions
		SET result = ?, version = version + 1, updated_at = NOW(3)
		WHERE refund_id = ? AND result = ? AND state = ? AND version = ?`,
		refundPending, refundID, refundReviewRequired, refundPending, versionBefore)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatal("re-arm CAS did not match the review row")
	}
	res, err = db.Exec(`UPDATE refund_batches
		SET state = ?, version = version + 1, updated_at = NOW(3)
		WHERE batch_id = ? AND state = ? AND version = ?`,
		refundBatchAdmitting, batch.BatchID, refundBatchManualHold, batchVersion)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatal("re-arm CAS did not match the held batch")
	}

	var createdAfter, resultAfter string
	var versionAfter int64
	if err := db.QueryRow(`SELECT created_at, result, version FROM order_refund_admissions WHERE refund_id = ?`, refundID).
		Scan(&createdAfter, &resultAfter, &versionAfter); err != nil {
		t.Fatal(err)
	}
	if createdAfter != createdBefore {
		t.Fatalf("created_at changed by re-arm: %q -> %q", createdBefore, createdAfter)
	}
	if resultAfter != refundPending || versionAfter != versionBefore+1 {
		t.Fatalf("re-armed row = %s v%d, want PENDING v%d", resultAfter, versionAfter, versionBefore+1)
	}
	if err := db.QueryRow(`SELECT last_error FROM refund_batches WHERE batch_id = ?`, batch.BatchID).Scan(&lastError); err != nil {
		t.Fatal(err)
	}
	if lastError != lastErrorBefore {
		t.Fatalf("re-arm cleared the canonical batch mirror: %q -> %q", lastErrorBefore, lastError)
	}

	if _, err := svc.TccCancelMarkPaid(ctx, req); err != nil {
		t.Fatalf("re-armed replay: %v", err)
	}
	var finalResult string
	if err := db.QueryRow(`SELECT result FROM order_refund_admissions WHERE refund_id = ?`, refundID).Scan(&finalResult); err != nil {
		t.Fatal(err)
	}
	if finalResult != refundResultPre {
		t.Fatalf("re-armed admission = %s, want PRE", finalResult)
	}
	var createdFinal string
	if err := db.QueryRow(`SELECT created_at FROM order_refund_admissions WHERE refund_id = ?`, refundID).Scan(&createdFinal); err != nil {
		t.Fatal(err)
	}
	if createdFinal != createdBefore {
		t.Fatalf("created_at changed after resolution: %q", createdFinal)
	}
}

// T6a W1-W3 (O3): Tx1 committed then a crash/replay must converge on the same
// persisted decision, never on an early terminal/release.
func TestAC50ReviewCrashReplayConverges(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	ff := &fakeFulfillmentPort{status: "TRANSIENT_ERROR"}
	svc := ac50Service(db, payments, ff)
	ctx := context.Background()
	ac50SeedOrder(t, db, "ORD-CRASH", 10000)
	ac50SeedPermit(t, db, "ORD-CRASH", permitConsumed)
	req := &pb.AdmitRefundRequest{
		RefundId: "R-CRASH", OrderNo: "ORD-CRASH", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	}
	// W1/W2: the first probe is transient -> PENDING_REMOTE with the Tx1
	// placeholder; a replay after the crash re-probes and persists the review.
	if _, err := svc.AdmitRefund(ctx, req); err == nil {
		t.Fatal("transient probe: expected Unavailable")
	}
	result, state, _ := ac50Admission(t, db, "R-CRASH")
	if result != refundResultPost || state != refundPending {
		t.Fatalf("after W1 = %s/%s, want placeholder POST/PENDING", result, state)
	}
	ff.setStatus(probeStatusNoWaybill)
	if _, replayErr := svc.AdmitRefund(ctx, req); replayErr == nil {
		t.Fatal("replay: expected REVIEW conflict")
	} else {
		ac50RequireReviewConflict(t, replayErr, "R-CRASH")
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = 'R-CRASH' AND state = 'RESERVED'`); got != 1 {
		t.Fatalf("commitments = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-CRASH' AND event_type = 'order.refund_admitted'`); got != 1 {
		t.Fatalf("events = %d, want the single Tx1 placeholder", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-CRASH' AND event_type = ?`, events.RefundReleaseEventType); got != 0 {
		t.Fatalf("release events = %d, want 0", got)
	}

	// W3: a real CANCELLED probe converges to PRE on replay, once.
	ac50SeedOrder(t, db, "ORD-CRASH2", 10000)
	ac50SeedPermit(t, db, "ORD-CRASH2", permitConsumed)
	ff.setStatus("TRANSIENT_ERROR")
	req2 := &pb.AdmitRefundRequest{
		RefundId: "R-CRASH2", OrderNo: "ORD-CRASH2", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	}
	if _, err := svc.AdmitRefund(ctx, req2); err == nil {
		t.Fatal("transient probe: expected Unavailable")
	}
	ff.setStatus("CANCELLED")
	if _, err := svc.AdmitRefund(ctx, req2); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-CRASH2' AND event_type = 'order.refund_admitted'`); got != 2 {
		t.Fatalf("PRE events = %d, want placeholder+terminal", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-CRASH2' AND event_type = ?`, events.RefundReleaseEventType); got != 1 {
		t.Fatalf("release events = %d, want 1", got)
	}
	if _, err := svc.AdmitRefund(ctx, req2); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-CRASH2' AND event_type = ?`, events.RefundReleaseEventType); got != 1 {
		t.Fatalf("replay duplicated the release: %d", got)
	}
}

// T6a W4: Ship and cancel race on the same refund_id. The Tx2 CAS elects one
// outcome; the loser reads it back and neither duplicates events nor writes a
// release for a POST admission.
func TestAC50ReviewShipCancelRace(t *testing.T) {
	db := orderTestDB(t)
	var calls atomic.Int64
	svc := ac50Service(db, &fakePaymentPort{captured: 10000}, &ac50AlternatingProbe{first: "ALREADY_SHIPPED", second: "CANCELLED", calls: &calls})
	ac50SeedOrder(t, db, "ORD-RACE-W4", 10000)
	ac50SeedPermit(t, db, "ORD-RACE-W4", permitConsumed)
	req := &pb.AdmitRefundRequest{
		RefundId: "R-RACE-W4", OrderNo: "ORD-RACE-W4", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	}
	replies, errs := ac50RunConcurrent(t, svc, req, 2)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
		if replies[i].GetResult() != refundResultPost && replies[i].GetResult() != refundResultPre {
			t.Fatalf("goroutine %d result = %q", i, replies[i].GetResult())
		}
	}
	final, state, _ := ac50Admission(t, db, "R-RACE-W4")
	if final != state || (final != refundResultPost && final != refundResultPre) {
		t.Fatalf("admission = %s/%s", final, state)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id = 'R-RACE-W4'`); got != 1 {
		t.Fatalf("admission rows = %d", got)
	}
	terminals := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-RACE-W4' AND event_type = 'order.refund_admitted'
		  AND JSON_UNQUOTE(JSON_EXTRACT(payload, '$.state')) IN (?, ?)`, refundResultPre, refundResultPost)
	if terminals != 1 {
		t.Fatalf("terminal events = %d, want 1", terminals)
	}
	releases := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-RACE-W4' AND event_type = ?`, events.RefundReleaseEventType)
	wantRelease := 0
	if final == refundResultPre {
		wantRelease = 1
	}
	if releases != wantRelease {
		t.Fatalf("release events = %d, want %d for %s", releases, wantRelease, final)
	}
}

// ac50AlternatingProbe returns a different status per call to exercise the
// Ship-vs-cancel race deterministically (both callers still race on the CAS).
type ac50AlternatingProbe struct {
	first, second string
	calls         *atomic.Int64
}

func (p *ac50AlternatingProbe) CancelIfCreated(context.Context, *pb.CancelIfCreatedRequest, ...grpc.CallOption) (*pb.CancelIfCreatedReply, error) {
	n := p.calls.Add(1)
	if n == 1 {
		return &pb.CancelIfCreatedReply{Status: p.first}, nil
	}
	return &pb.CancelIfCreatedReply{Status: p.second}, nil
}

func (p *ac50AlternatingProbe) ListShipments(context.Context, *pb.ListShipmentsRequest, ...grpc.CallOption) (*pb.ShipmentList, error) {
	return &pb.ShipmentList{}, nil
}

// T6b/T6c (F7b/O14, DB-level): a REVIEW emits no terminal event, so duplicate
// replays and a late real cancellation cannot auto-PRE or duplicate cancel/
// refund work; the Tx1 placeholder stays the only event.
func TestAC50ReviewConsumerNoAutoPre(t *testing.T) {
	db := orderTestDB(t)
	ff := &fakeFulfillmentPort{status: probeStatusNoWaybill}
	svc := ac50Service(db, &fakePaymentPort{captured: 10000}, ff)
	ctx := context.Background()
	ac50SeedOrder(t, db, "ORD-CONS", 10000)
	ac50SeedPermit(t, db, "ORD-CONS", permitConsumed)
	req := &pb.AdmitRefundRequest{
		RefundId: "R-CONS", OrderNo: "ORD-CONS", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	}
	if _, err := svc.AdmitRefund(ctx, req); err == nil {
		t.Fatal("expected REVIEW")
	}
	eventsBefore := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-CONS'`)
	// A late real cancellation is a Fulfillment fact; Order has no scanner and
	// the persisted review stays sticky until an audited re-arm.
	ff.setStatus("CANCELLED")
	for i := 0; i < 3; i++ {
		if _, err := svc.AdmitRefund(ctx, req); err == nil {
			t.Fatal("replay after late cancel: expected the same review conflict")
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-CONS'`); got != eventsBefore {
		t.Fatalf("review replays wrote outbox events: %d -> %d", eventsBefore, got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-CONS'
		AND JSON_UNQUOTE(JSON_EXTRACT(payload, '$.state')) IN (?, ?)`, refundResultPre, refundResultPost); got != 0 {
		t.Fatalf("terminal events = %d, want 0", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-CONS' AND event_type = ?`, events.RefundReleaseEventType); got != 0 {
		t.Fatalf("release events = %d, want 0", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = 'R-CONS' AND state = 'RESERVED'`); got != 1 {
		t.Fatalf("commitments = %d, want 1", got)
	}
}
