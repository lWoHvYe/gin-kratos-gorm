package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// These tests run against a real MySQL (TEST_MYSQL_DSN); they assert the
// transactional guarantees of RefundBatch, not a mock: refund rows, the intent
// aggregate and the payment.refunded outbox events either all commit or none.

func refundBatchCounts(t *testing.T, db *sql.DB, paymentNo string) (refundRows, events, batches int) {
	t.Helper()
	if err := db.QueryRow(`SELECT COUNT(*) FROM refunds WHERE payment_no = ?`, paymentNo).Scan(&refundRows); err != nil {
		t.Fatalf("count refunds: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox_events WHERE event_type = 'payment.refunded'`).Scan(&events); err != nil {
		t.Fatalf("count refund events: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM payment_refund_batches WHERE payment_no = ?`, paymentNo).Scan(&batches); err != nil {
		t.Fatalf("count refund batches: %v", err)
	}
	return refundRows, events, batches
}

type persistedRefundedEvent struct {
	PaymentNo    string `json:"payment_no"`
	OrderNo      string `json:"order_no"`
	RefundNo     string `json:"refund_no"`
	RefundID     string `json:"refund_id"`
	AmountCent   int64  `json:"amount_cent"`
	RefundedCent int64  `json:"refunded_cent"`
	CapturedCent int64  `json:"captured_cent"`
	Status       string `json:"status"`
}

func loadRefundedEvents(t *testing.T, db *sql.DB) []persistedRefundedEvent {
	t.Helper()
	rows, err := db.Query(`SELECT payload FROM outbox_events WHERE event_type = 'payment.refunded' ORDER BY id`)
	if err != nil {
		t.Fatalf("load refund events: %v", err)
	}
	defer rows.Close()
	var out []persistedRefundedEvent
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatalf("scan refund event: %v", err)
		}
		var event persistedRefundedEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatalf("decode refund event: %v", err)
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate refund events: %v", err)
	}
	return out
}

func batchItemResult(t *testing.T, reply *pb.RefundBatchReply, refundID string) *pb.RefundBatchItemResult {
	t.Helper()
	for _, result := range reply.GetItemResults() {
		if result.GetRefundId() == refundID {
			return result
		}
	}
	t.Fatalf("refund_id %s missing from item_results %+v", refundID, reply.GetItemResults())
	return nil
}

func wantRefundBatchError(t *testing.T, err error, code apperr.Code, reason string) {
	t.Helper()
	if err == nil {
		t.Fatalf("RefundBatch succeeded, want %s", reason)
	}
	ae := apperr.From(err)
	if ae.Code != code {
		t.Fatalf("error code = %s, want %s (%v)", ae.Code, code, err)
	}
	if got := ae.Details["reason"]; got != reason {
		t.Fatalf("error reason = %v, want %s (%v)", got, reason, err)
	}
}

func newRefundBatchRequest(paymentNo, batchID string, items ...*pb.RefundBatchItem) *pb.RefundBatchRequest {
	var total int64
	for _, item := range items {
		total += item.GetAmountCent()
	}
	return &pb.RefundBatchRequest{
		PaymentNo: paymentNo, BatchId: batchID, ExpectedTotalCent: total,
		Items: items, Reason: "tcc refund shared reason",
	}
}

// TestRefundBatchAtomicSameAmountReason is the QC-23 B-1 regression: two group
// refunds on the same payment with the same amount and reason must both commit
// in one transaction after the legacy uk_refunds_idem key is dropped.
func TestRefundBatchAtomicSameAmountReason(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-RB-SAME", "ORD-RB-SAME", statusSucceeded, 600, 600, 0)
	svc := New(db, nil, "", nil)
	ctx := context.Background()

	reply, err := svc.RefundBatch(ctx, newRefundBatchRequest("PAY-RB-SAME", "BATCH-SAME",
		&pb.RefundBatchItem{RefundId: "tcc:PAY-RB-SAME:G1", AmountCent: 300},
		&pb.RefundBatchItem{RefundId: "tcc:PAY-RB-SAME:G2", AmountCent: 300},
	))
	if err != nil {
		t.Fatalf("RefundBatch: %v", err)
	}
	if reply.GetStatus() != refundBatchStatusRefunded || reply.GetRefundedCent() != 600 {
		t.Fatalf("reply = %+v", reply)
	}
	if len(reply.GetItemResults()) != 2 {
		t.Fatalf("item_results = %+v", reply.GetItemResults())
	}
	nos := map[string]bool{}
	for _, item := range []string{"tcc:PAY-RB-SAME:G1", "tcc:PAY-RB-SAME:G2"} {
		result := batchItemResult(t, reply, item)
		if result.GetStatus() != refundBatchStatusRefunded || result.GetAmountCent() != 300 || result.GetRefundNo() == "" {
			t.Fatalf("item result %s = %+v", item, result)
		}
		if nos[result.GetRefundNo()] {
			t.Fatalf("duplicate refund_no %s", result.GetRefundNo())
		}
		nos[result.GetRefundNo()] = true
	}

	var refunded int64
	var status string
	if err := db.QueryRow(`SELECT refunded_cent, status FROM payment_intents WHERE payment_no = 'PAY-RB-SAME'`).
		Scan(&refunded, &status); err != nil {
		t.Fatal(err)
	}
	if refunded != 600 || status != statusRefunded {
		t.Fatalf("intent = (%d, %s), want (600, REFUNDED)", refunded, status)
	}

	rows, err := db.Query(`SELECT amount_cent, reason, idempotency_key, batch_id
		FROM refunds WHERE payment_no = 'PAY-RB-SAME' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var amount int64
		var reason, key, batch string
		if err := rows.Scan(&amount, &reason, &key, &batch); err != nil {
			t.Fatal(err)
		}
		count++
		if amount != 300 || reason != "tcc refund shared reason" || batch != "BATCH-SAME" {
			t.Fatalf("refund row = (%d, %q, %q, %q)", amount, reason, key, batch)
		}
		if key != "tcc:PAY-RB-SAME:G1" && key != "tcc:PAY-RB-SAME:G2" {
			t.Fatalf("idempotency_key = %q, want refund_id", key)
		}
	}
	if count != 2 {
		t.Fatalf("refund rows = %d, want 2", count)
	}
	if rows, events, batches := refundBatchCounts(t, db, "PAY-RB-SAME"); rows != 2 || events != 2 || batches != 1 {
		t.Fatalf("counts = (%d refunds, %d events, %d batches)", rows, events, batches)
	}
	eventIDs := map[string]bool{}
	for _, event := range loadRefundedEvents(t, db) {
		if event.PaymentNo != "PAY-RB-SAME" || event.RefundID == "" || event.RefundNo == "" ||
			event.AmountCent != 300 || event.RefundedCent != 600 || event.CapturedCent != 600 {
			t.Fatalf("refund event = %+v", event)
		}
		eventIDs[event.RefundID] = true
	}
	if !eventIDs["tcc:PAY-RB-SAME:G1"] || !eventIDs["tcc:PAY-RB-SAME:G2"] {
		t.Fatalf("event refund_ids = %+v", eventIDs)
	}
}

// TestRefundBatchSecondInsertFailureRollsBack injects a duplicate-key failure
// on the second refund insert. The first insert and the intent update must roll
// back with it: no refund row, no event, no batch ledger row.
func TestRefundBatchSecondInsertFailureRollsBack(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-RB-FAIL", "ORD-RB-FAIL", statusSucceeded, 600, 600, 0)
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS trg_refund_batch_forced_fail`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER trg_refund_batch_forced_fail BEFORE INSERT ON refunds
		FOR EACH ROW
		BEGIN
			IF NEW.idempotency_key = 'RB-FAIL-2' THEN
				SIGNAL SQLSTATE '23000' SET MESSAGE_TEXT = 'forced duplicate key', MYSQL_ERRNO = 1062;
			END IF;
		END`); err != nil {
		t.Fatalf("install fault trigger: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP TRIGGER IF EXISTS trg_refund_batch_forced_fail`) })

	svc := New(db, nil, "", nil)
	_, err := svc.RefundBatch(context.Background(), newRefundBatchRequest("PAY-RB-FAIL", "BATCH-FAIL",
		&pb.RefundBatchItem{RefundId: "RB-OK-1", AmountCent: 300},
		&pb.RefundBatchItem{RefundId: "RB-FAIL-2", AmountCent: 300},
	))
	wantRefundBatchError(t, err, apperr.CodeConflict, refundBatchReasonItemConflict)

	if rows, events, batches := refundBatchCounts(t, db, "PAY-RB-FAIL"); rows != 0 || events != 0 || batches != 0 {
		t.Fatalf("counts after rollback = (%d refunds, %d events, %d batches), want all zero", rows, events, batches)
	}
	var refunded int64
	var status string
	if err := db.QueryRow(`SELECT refunded_cent, status FROM payment_intents WHERE payment_no = 'PAY-RB-FAIL'`).
		Scan(&refunded, &status); err != nil {
		t.Fatal(err)
	}
	if refunded != 0 || status != statusSucceeded {
		t.Fatalf("intent after rollback = (%d, %s), want (0, SUCCEEDED)", refunded, status)
	}
}

func TestRefundBatchReplayIsIdempotent(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-RB-REPLAY", "ORD-RB-REPLAY", statusSucceeded, 600, 600, 0)
	svc := New(db, nil, "", nil)
	req := newRefundBatchRequest("PAY-RB-REPLAY", "BATCH-REPLAY",
		&pb.RefundBatchItem{RefundId: "RB-REPLAY-1", AmountCent: 200},
		&pb.RefundBatchItem{RefundId: "RB-REPLAY-2", AmountCent: 400},
	)
	first, err := svc.RefundBatch(context.Background(), req)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	second, err := svc.RefundBatch(context.Background(), req)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if first.GetStatus() != refundBatchStatusRefunded || second.GetStatus() != refundBatchStatusRefunded {
		t.Fatalf("statuses = (%s, %s)", first.GetStatus(), second.GetStatus())
	}
	if second.GetRefundedCent() != first.GetRefundedCent() || len(second.GetItemResults()) != len(first.GetItemResults()) {
		t.Fatalf("replay reply diverged: first=%+v second=%+v", first, second)
	}
	for _, item := range []string{"RB-REPLAY-1", "RB-REPLAY-2"} {
		a, b := batchItemResult(t, first, item), batchItemResult(t, second, item)
		if a.GetRefundNo() != b.GetRefundNo() || a.GetStatus() != b.GetStatus() || a.GetAmountCent() != b.GetAmountCent() {
			t.Fatalf("replay item %s diverged: %+v vs %+v", item, a, b)
		}
	}
	if rows, events, batches := refundBatchCounts(t, db, "PAY-RB-REPLAY"); rows != 2 || events != 2 || batches != 1 {
		t.Fatalf("counts after replay = (%d refunds, %d events, %d batches)", rows, events, batches)
	}
}

func TestRefundBatchSameBatchDifferentItemsConflicts(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-RB-IDEM", "ORD-RB-IDEM", statusSucceeded, 600, 600, 0)
	svc := New(db, nil, "", nil)
	first := newRefundBatchRequest("PAY-RB-IDEM", "BATCH-IDEM",
		&pb.RefundBatchItem{RefundId: "RB-IDEM-1", AmountCent: 300},
		&pb.RefundBatchItem{RefundId: "RB-IDEM-2", AmountCent: 300},
	)
	if _, err := svc.RefundBatch(context.Background(), first); err != nil {
		t.Fatalf("first call: %v", err)
	}

	subset := newRefundBatchRequest("PAY-RB-IDEM", "BATCH-IDEM",
		&pb.RefundBatchItem{RefundId: "RB-IDEM-1", AmountCent: 300},
	)
	_, err := svc.RefundBatch(context.Background(), subset)
	wantRefundBatchError(t, err, apperr.CodeConflict, refundBatchReasonIdempotencyConflict)

	changed := newRefundBatchRequest("PAY-RB-IDEM", "BATCH-IDEM",
		&pb.RefundBatchItem{RefundId: "RB-IDEM-1", AmountCent: 200},
		&pb.RefundBatchItem{RefundId: "RB-IDEM-2", AmountCent: 100},
	)
	_, err = svc.RefundBatch(context.Background(), changed)
	wantRefundBatchError(t, err, apperr.CodeConflict, refundBatchReasonIdempotencyConflict)

	if rows, events, batches := refundBatchCounts(t, db, "PAY-RB-IDEM"); rows != 2 || events != 2 || batches != 1 {
		t.Fatalf("counts after conflicts = (%d refunds, %d events, %d batches), want (2,2,1)", rows, events, batches)
	}
	var refunded int64
	if err := db.QueryRow(`SELECT refunded_cent FROM payment_intents WHERE payment_no = 'PAY-RB-IDEM'`).Scan(&refunded); err != nil {
		t.Fatal(err)
	}
	if refunded != 600 {
		t.Fatalf("refunded = %d, want 600", refunded)
	}
}

func TestRefundBatchSameRefundIDDifferentAmountConflicts(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-RB-ITEM", "ORD-RB-ITEM", statusSucceeded, 600, 600, 0)
	svc := New(db, nil, "", nil)
	if _, err := svc.RefundBatch(context.Background(), newRefundBatchRequest("PAY-RB-ITEM", "BATCH-ITEM-A",
		&pb.RefundBatchItem{RefundId: "RB-ITEM-1", AmountCent: 300},
	)); err != nil {
		t.Fatalf("first call: %v", err)
	}

	_, err := svc.RefundBatch(context.Background(), newRefundBatchRequest("PAY-RB-ITEM", "BATCH-ITEM-B",
		&pb.RefundBatchItem{RefundId: "RB-ITEM-1", AmountCent: 200},
		&pb.RefundBatchItem{RefundId: "RB-ITEM-2", AmountCent: 100},
	))
	wantRefundBatchError(t, err, apperr.CodeConflict, refundBatchReasonItemConflict)

	if rows, events, batches := refundBatchCounts(t, db, "PAY-RB-ITEM"); rows != 1 || events != 1 || batches != 1 {
		t.Fatalf("counts after item conflict = (%d refunds, %d events, %d batches), want (1,1,1)", rows, events, batches)
	}
	var secondBatch int
	if err := db.QueryRow(`SELECT COUNT(*) FROM payment_refund_batches
		WHERE payment_no = 'PAY-RB-ITEM' AND batch_id = 'BATCH-ITEM-B'`).Scan(&secondBatch); err != nil {
		t.Fatal(err)
	}
	if secondBatch != 0 {
		t.Fatal("a conflicting batch must not persist a ledger row")
	}
	var newItemRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM refunds WHERE payment_no = 'PAY-RB-ITEM' AND idempotency_key = 'RB-ITEM-2'`).Scan(&newItemRows); err != nil {
		t.Fatal(err)
	}
	if newItemRows != 0 {
		t.Fatal("a conflicting batch must not insert its later items")
	}
}

func TestRefundBatchInsufficientRemainingAndSumMismatchWriteNothing(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-RB-REM", "ORD-RB-REM", statusPartiallyRefunded, 1000, 1000, 200)
	svc := New(db, nil, "", nil)
	ctx := context.Background()

	_, err := svc.RefundBatch(ctx, newRefundBatchRequest("PAY-RB-REM", "BATCH-REM",
		&pb.RefundBatchItem{RefundId: "RB-REM-1", AmountCent: 500},
		&pb.RefundBatchItem{RefundId: "RB-REM-2", AmountCent: 400},
	))
	wantRefundBatchError(t, err, apperr.CodeFailedPrecondition, refundBatchReasonInsufficient)

	badSum := &pb.RefundBatchRequest{
		PaymentNo: "PAY-RB-REM", BatchId: "BATCH-SUM", ExpectedTotalCent: 800,
		Items: []*pb.RefundBatchItem{
			{RefundId: "RB-SUM-1", AmountCent: 500},
			{RefundId: "RB-SUM-2", AmountCent: 400},
		},
		Reason: "sum mismatch",
	}
	_, err = svc.RefundBatch(ctx, badSum)
	wantRefundBatchError(t, err, apperr.CodeInvalidArgument, refundBatchReasonSumMismatch)

	if rows, events, batches := refundBatchCounts(t, db, "PAY-RB-REM"); rows != 0 || events != 0 || batches != 0 {
		t.Fatalf("counts after rejected batches = (%d refunds, %d events, %d batches), want all zero", rows, events, batches)
	}
	var refunded int64
	if err := db.QueryRow(`SELECT refunded_cent FROM payment_intents WHERE payment_no = 'PAY-RB-REM'`).Scan(&refunded); err != nil {
		t.Fatal(err)
	}
	if refunded != 200 {
		t.Fatalf("refunded = %d, want 200 (unchanged)", refunded)
	}
}

func TestRefundBatchMixedExistingAndNewItems(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-RB-MIX", "ORD-RB-MIX", statusSucceeded, 1000, 1000, 0)
	svc := New(db, nil, "", nil)
	ctx := context.Background()
	if _, err := svc.Refund(ctx, &pb.RefundRequest{
		PaymentNo: "PAY-RB-MIX", AmountCent: 300, Reason: "earlier plain refund",
		IdempotencyKey: "RB-MIX-1",
	}); err != nil {
		t.Fatalf("plain Refund: %v", err)
	}

	reply, err := svc.RefundBatch(ctx, newRefundBatchRequest("PAY-RB-MIX", "BATCH-MIX",
		&pb.RefundBatchItem{RefundId: "RB-MIX-1", AmountCent: 300},
		&pb.RefundBatchItem{RefundId: "RB-MIX-2", AmountCent: 700},
	))
	if err != nil {
		t.Fatalf("mixed RefundBatch: %v", err)
	}
	if reply.GetStatus() != refundBatchStatusRefunded || reply.GetRefundedCent() != 1000 {
		t.Fatalf("reply = %+v", reply)
	}
	if got := batchItemResult(t, reply, "RB-MIX-1").GetStatus(); got != refundBatchStatusAlreadyRefunded {
		t.Fatalf("existing item status = %s, want ALREADY_REFUNDED", got)
	}
	if got := batchItemResult(t, reply, "RB-MIX-2").GetStatus(); got != refundBatchStatusRefunded {
		t.Fatalf("new item status = %s, want REFUNDED", got)
	}

	var refunded int64
	if err := db.QueryRow(`SELECT refunded_cent FROM payment_intents WHERE payment_no = 'PAY-RB-MIX'`).Scan(&refunded); err != nil {
		t.Fatal(err)
	}
	if refunded != 1000 {
		t.Fatalf("intent refunded = %d, want 1000", refunded)
	}
	// The pre-existing plain refund is not adopted into the batch: its
	// attribution must not be rewritten by a later batch.
	var existingBatch, newBatch string
	if err := db.QueryRow(`SELECT batch_id FROM refunds WHERE payment_no = 'PAY-RB-MIX' AND idempotency_key = 'RB-MIX-1'`).Scan(&existingBatch); err != nil {
		t.Fatal(err)
	}
	if existingBatch != "" {
		t.Fatalf("existing refund batch_id = %q, want empty", existingBatch)
	}
	if err := db.QueryRow(`SELECT batch_id FROM refunds WHERE payment_no = 'PAY-RB-MIX' AND idempotency_key = 'RB-MIX-2'`).Scan(&newBatch); err != nil {
		t.Fatal(err)
	}
	if newBatch != "BATCH-MIX" {
		t.Fatalf("new refund batch_id = %q, want BATCH-MIX", newBatch)
	}
	// One plain-refund event plus one event for the new item; the existing item
	// must not emit a second payment.refunded.
	if rows, events, batches := refundBatchCounts(t, db, "PAY-RB-MIX"); rows != 2 || events != 2 || batches != 1 {
		t.Fatalf("counts = (%d refunds, %d events, %d batches), want (2,2,1)", rows, events, batches)
	}
	// Replaying the mixed batch returns the same stored reply and writes nothing.
	replay, err := svc.RefundBatch(ctx, newRefundBatchRequest("PAY-RB-MIX", "BATCH-MIX",
		&pb.RefundBatchItem{RefundId: "RB-MIX-1", AmountCent: 300},
		&pb.RefundBatchItem{RefundId: "RB-MIX-2", AmountCent: 700},
	))
	if err != nil {
		t.Fatalf("mixed replay: %v", err)
	}
	if replay.GetStatus() != reply.GetStatus() || replay.GetRefundedCent() != reply.GetRefundedCent() {
		t.Fatalf("mixed replay = %+v", replay)
	}
}

// TestRefundBatchReplyRefundedCentIsBatchSum pins the C-04 §2.6 rule that
// refunded_cent is Σ items, never the intent's cumulative refunded total.
func TestRefundBatchReplyRefundedCentIsBatchSum(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-RB-SUM", "ORD-RB-SUM", statusSucceeded, 1000, 1000, 0)
	svc := New(db, nil, "", nil)
	ctx := context.Background()
	if _, err := svc.Refund(ctx, &pb.RefundRequest{
		PaymentNo: "PAY-RB-SUM", AmountCent: 300, Reason: "earlier plain refund",
		IdempotencyKey: "RB-SUM-EXISTING",
	}); err != nil {
		t.Fatalf("plain Refund: %v", err)
	}
	reply, err := svc.RefundBatch(ctx, newRefundBatchRequest("PAY-RB-SUM", "BATCH-SUM-ONLY",
		&pb.RefundBatchItem{RefundId: "RB-SUM-NEW", AmountCent: 700},
	))
	if err != nil {
		t.Fatalf("RefundBatch: %v", err)
	}
	if reply.GetRefundedCent() != 700 {
		t.Fatalf("refunded_cent = %d, want 700 (Σ items, not intent total 1000)", reply.GetRefundedCent())
	}
	var intentTotal int64
	if err := db.QueryRow(`SELECT refunded_cent FROM payment_intents WHERE payment_no = 'PAY-RB-SUM'`).Scan(&intentTotal); err != nil {
		t.Fatal(err)
	}
	if intentTotal != 1000 {
		t.Fatalf("intent refunded total = %d, want 1000", intentTotal)
	}
}

type concurrentRefundBatchCall struct {
	reply *pb.RefundBatchReply
	err   error
}

func runConcurrentRefundBatch(svc *Service, reqs []*pb.RefundBatchRequest) []concurrentRefundBatchCall {
	calls := make([]concurrentRefundBatchCall, len(reqs))
	var wg sync.WaitGroup
	for i, req := range reqs {
		wg.Add(1)
		go func(i int, req *pb.RefundBatchRequest) {
			defer wg.Done()
			calls[i].reply, calls[i].err = svc.RefundBatch(context.Background(), req)
		}(i, req)
	}
	wg.Wait()
	return calls
}

func TestRefundBatchConcurrentSameBatch(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-RB-CONC", "ORD-RB-CONC", statusSucceeded, 600, 600, 0)
	svc := New(db, nil, "", nil)
	req := newRefundBatchRequest("PAY-RB-CONC", "BATCH-CONC",
		&pb.RefundBatchItem{RefundId: "RB-CONC-1", AmountCent: 300},
		&pb.RefundBatchItem{RefundId: "RB-CONC-2", AmountCent: 300},
	)
	calls := runConcurrentRefundBatch(svc, []*pb.RefundBatchRequest{req, req})
	for i, call := range calls {
		if call.err != nil {
			t.Fatalf("concurrent call %d: %v", i, call.err)
		}
		if call.reply.GetStatus() != refundBatchStatusRefunded || call.reply.GetRefundedCent() != 600 {
			t.Fatalf("concurrent call %d reply = %+v", i, call.reply)
		}
	}
	if rows, events, batches := refundBatchCounts(t, db, "PAY-RB-CONC"); rows != 2 || events != 2 || batches != 1 {
		t.Fatalf("counts = (%d refunds, %d events, %d batches), want (2,2,1)", rows, events, batches)
	}
}

func TestRefundBatchConcurrentDifferentBatchesCannotOverRefund(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-RB-OVER", "ORD-RB-OVER", statusSucceeded, 1000, 1000, 0)
	svc := New(db, nil, "", nil)
	reqA := newRefundBatchRequest("PAY-RB-OVER", "BATCH-OVER-A",
		&pb.RefundBatchItem{RefundId: "RB-OVER-A", AmountCent: 600},
	)
	reqB := newRefundBatchRequest("PAY-RB-OVER", "BATCH-OVER-B",
		&pb.RefundBatchItem{RefundId: "RB-OVER-B", AmountCent: 600},
	)
	calls := runConcurrentRefundBatch(svc, []*pb.RefundBatchRequest{reqA, reqB})
	success, insufficient := 0, 0
	for _, call := range calls {
		switch {
		case call.err == nil:
			success++
		case apperr.IsCode(call.err, apperr.CodeFailedPrecondition) &&
			apperr.From(call.err).Details["reason"] == refundBatchReasonInsufficient:
			insufficient++
		default:
			t.Fatalf("unexpected concurrent error: %v", call.err)
		}
	}
	if success != 1 || insufficient != 1 {
		t.Fatalf("success = %d, insufficient = %d, want 1/1", success, insufficient)
	}
	var refunded int64
	if err := db.QueryRow(`SELECT refunded_cent FROM payment_intents WHERE payment_no = 'PAY-RB-OVER'`).Scan(&refunded); err != nil {
		t.Fatal(err)
	}
	if refunded != 600 {
		t.Fatalf("refunded = %d, want 600 (only one batch)", refunded)
	}
	if rows, events, batches := refundBatchCounts(t, db, "PAY-RB-OVER"); rows != 1 || events != 1 || batches != 1 {
		t.Fatalf("counts = (%d refunds, %d events, %d batches), want (1,1,1)", rows, events, batches)
	}
}

func TestRefundBatchAllExistingReturnsAlreadyRefunded(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-RB-ALREADY", "ORD-RB-ALREADY", statusSucceeded, 600, 600, 0)
	svc := New(db, nil, "", nil)
	ctx := context.Background()
	if _, err := svc.Refund(ctx, &pb.RefundRequest{
		PaymentNo: "PAY-RB-ALREADY", AmountCent: 300, Reason: "earlier plain refund",
		IdempotencyKey: "RB-ALREADY-1",
	}); err != nil {
		t.Fatalf("plain Refund: %v", err)
	}
	var plainRefundNo string
	if err := db.QueryRow(`SELECT refund_no FROM refunds
		WHERE payment_no = 'PAY-RB-ALREADY' AND idempotency_key = 'RB-ALREADY-1'`).Scan(&plainRefundNo); err != nil {
		t.Fatal(err)
	}

	req := newRefundBatchRequest("PAY-RB-ALREADY", "BATCH-ALREADY",
		&pb.RefundBatchItem{RefundId: "RB-ALREADY-1", AmountCent: 300},
	)
	reply, err := svc.RefundBatch(ctx, req)
	if err != nil {
		t.Fatalf("all-existing RefundBatch: %v", err)
	}
	if reply.GetStatus() != refundBatchStatusAlreadyRefunded || reply.GetRefundedCent() != 300 {
		t.Fatalf("reply = %+v, want ALREADY_REFUNDED/300", reply)
	}
	result := batchItemResult(t, reply, "RB-ALREADY-1")
	if result.GetStatus() != refundBatchStatusAlreadyRefunded || result.GetRefundNo() != plainRefundNo {
		t.Fatalf("item result = %+v", result)
	}
	if rows, events, batches := refundBatchCounts(t, db, "PAY-RB-ALREADY"); rows != 1 || events != 1 || batches != 1 {
		t.Fatalf("counts = (%d refunds, %d events, %d batches), want (1,1,1)", rows, events, batches)
	}
	// The replay must return the same ALREADY_REFUNDED reply, not downgrade or
	// emit a second event.
	replay, err := svc.RefundBatch(ctx, req)
	if err != nil {
		t.Fatalf("all-existing replay: %v", err)
	}
	if replay.GetStatus() != refundBatchStatusAlreadyRefunded || replay.GetRefundedCent() != 300 {
		t.Fatalf("replay = %+v", replay)
	}
	if rows, events, batches := refundBatchCounts(t, db, "PAY-RB-ALREADY"); rows != 1 || events != 1 || batches != 1 {
		t.Fatalf("counts after replay = (%d refunds, %d events, %d batches)", rows, events, batches)
	}
}

func TestRefundBatchUncapturedIntentFailsClosed(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-RB-PENDING", "ORD-RB-PENDING", statusPending, 600, 0, 0)
	svc := New(db, nil, "", nil)
	_, err := svc.RefundBatch(context.Background(), newRefundBatchRequest("PAY-RB-PENDING", "BATCH-PENDING",
		&pb.RefundBatchItem{RefundId: "RB-PENDING-1", AmountCent: 300},
	))
	wantRefundBatchError(t, err, apperr.CodeFailedPrecondition, refundBatchReasonNotRefundable)
	if rows, events, batches := refundBatchCounts(t, db, "PAY-RB-PENDING"); rows != 0 || events != 0 || batches != 0 {
		t.Fatalf("counts = (%d refunds, %d events, %d batches), want all zero", rows, events, batches)
	}
}
