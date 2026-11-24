package order

import (
	"context"
	"database/sql"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// T1.4b1-R evidence (SPEC-0002 v1.9 AC-51/AC-65, C-04 RefundBatch, T1.4b1-R
// Plan §3/§5). Real MySQL: the guarantees under test are the frozen batch
// ledger, the Payment preflight classification and the all-or-none batch
// contract. Payment is the package fake, which mirrors C-04's row-lock
// semantics (atomic items, persisted reply per batch_id, deterministic errors).

func loadCommitmentState(t *testing.T, db *sql.DB, refundID string) (string, int64) {
	t.Helper()
	var state string
	var amount int64
	if err := db.QueryRow(`SELECT state, amount_cent FROM order_refund_commitments WHERE refund_id = ?`,
		refundID).Scan(&state, &amount); err != nil {
		t.Fatal(err)
	}
	return state, amount
}

func batchItemAmount(req *pb.RefundBatchRequest, refundID string) (int64, bool) {
	for _, item := range req.GetItems() {
		if item.GetRefundId() == refundID {
			return item.GetAmountCent(), true
		}
	}
	return 0, false
}

func assertBatchItems(t *testing.T, req *pb.RefundBatchRequest, want map[string]int64) {
	t.Helper()
	if len(req.GetItems()) != len(want) {
		t.Fatalf("batch items = %d, want %d (%v)", len(req.GetItems()), len(want), want)
	}
	for refundID, amount := range want {
		got, ok := batchItemAmount(req, refundID)
		if !ok || got != amount {
			t.Fatalf("batch item %s = %d/%t, want %d", refundID, got, ok, amount)
		}
	}
}

// admittedTccBatch seeds a two-group TCC compensation, runs Tx1 + admission and
// leaves the batch ADMITTED (the T1.4b1-R payment precondition).
func admittedTccBatch(t *testing.T, db *sql.DB, svc *Service, orderNo, paymentNo string, total, g1, g2 int64) tccBatchRow {
	t.Helper()
	ctx := context.Background()
	seedTccOrder(t, db, orderNo, total, total, 0, 0, paymentNo)
	seedTccGroup(t, db, orderNo, "G1", 1, 0)
	seedTccGroup(t, db, orderNo, "G2", 2, 0)
	seedTccLine(t, db, orderNo, 1, "G1", g1)
	seedTccLine(t, db, orderNo, 2, "G2", g2)
	if err := runTccTx1(t, ctx, svc, db, orderNo, paymentNo, total); err != nil {
		t.Fatalf("Tx1: %v", err)
	}
	if err := svc.admitTccBatch(ctx, orderNo, paymentNo); err != nil {
		t.Fatalf("admission: %v", err)
	}
	batch := loadTccBatchRow(t, db, orderNo, paymentNo)
	if batch.State != refundBatchAdmitted {
		t.Fatalf("batch after admission = %s, want ADMITTED", batch.State)
	}
	return batch
}

// T1.4b1-R: exactly one atomic RefundBatch with expected_total_cent=outstanding
// (B), never a per-group Refund. The batch stays PAYING until payment.refunded.
func TestTccRefundBatchPaysEveryAdmittedGroupAtomically(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-PAY-OK", 10000, 10000, 0, 0, "PAY-PAY-OK")
	seedTccGroup(t, db, "ORD-PAY-OK", "G1", 1, 0)
	seedTccGroup(t, db, "ORD-PAY-OK", "G2", 2, 0)
	seedTccLine(t, db, "ORD-PAY-OK", 1, "G1", 6000)
	seedTccLine(t, db, "ORD-PAY-OK", 2, "G2", 4000)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-OK", PaymentNo: "PAY-PAY-OK", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	if len(payments.refundCalls) != 0 {
		t.Fatalf("per-group Refund calls = %d, want 0 (atomic batch only)", len(payments.refundCalls))
	}
	if len(payments.batchCalls) != 1 {
		t.Fatalf("RefundBatch calls = %d, want 1", len(payments.batchCalls))
	}
	batch := loadTccBatchRow(t, db, "ORD-PAY-OK", "PAY-PAY-OK")
	req := payments.batchCalls[0]
	if req.GetPaymentNo() != "PAY-PAY-OK" || req.GetBatchId() != batch.BatchID {
		t.Fatalf("batch request = %s/%s, want PAY-PAY-OK/%s", req.GetPaymentNo(), req.GetBatchId(), batch.BatchID)
	}
	if req.GetExpectedTotalCent() != 10000 {
		t.Fatalf("expected_total_cent = %d, want outstanding 10000", req.GetExpectedTotalCent())
	}
	assertBatchItems(t, req, map[string]int64{
		tccGroupRefundID("PAY-PAY-OK", "G1"): 6000,
		tccGroupRefundID("PAY-PAY-OK", "G2"): 4000,
	})
	if batch.State != refundBatchPaying {
		t.Fatalf("batch after RefundBatch = %s, want PAYING until events", batch.State)
	}
	for _, group := range []string{"G1", "G2"} {
		refundID := tccGroupRefundID("PAY-PAY-OK", group)
		if state, _ := loadCommitmentState(t, db, refundID); state != commitmentReserved {
			t.Fatalf("%s commitment = %s, want RESERVED (reply is not the event)", group, state)
		}
		if appState, _, _, _, _ := loadTccApplication(t, ctx, db, refundID); appState != refundApplicationProcessing {
			t.Fatalf("%s application = %s, want PROCESSING until payment.refunded", group, appState)
		}
	}

	// payment.refunded is the only closure path.
	for _, event := range []paymentRefundedEvent{
		{PaymentNo: "PAY-PAY-OK", OrderNo: "ORD-PAY-OK", RefundID: tccGroupRefundID("PAY-PAY-OK", "G1"),
			AmountCent: 6000, RefundedCent: 6000, OrderRefundedCent: 6000, CapturedCent: 10000},
		{PaymentNo: "PAY-PAY-OK", OrderNo: "ORD-PAY-OK", RefundID: tccGroupRefundID("PAY-PAY-OK", "G2"),
			AmountCent: 4000, RefundedCent: 10000, OrderRefundedCent: 10000, CapturedCent: 10000},
	} {
		if err := applyPaymentRefunded(ctx, svc, event); err != nil {
			t.Fatal(err)
		}
	}
	for _, group := range []string{"G1", "G2"} {
		refundID := tccGroupRefundID("PAY-PAY-OK", group)
		if state, _ := loadCommitmentState(t, db, refundID); state != commitmentSettled {
			t.Fatalf("%s commitment = %s, want SETTLED", group, state)
		}
		if appState, _, _, _, _ := loadTccApplication(t, ctx, db, refundID); appState != refundApplicationDone {
			t.Fatalf("%s application = %s, want DONE", group, appState)
		}
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-OK", "PAY-PAY-OK"); batch.State != refundBatchDone {
		t.Fatalf("batch after payment.refunded = %s, want DONE", batch.State)
	}
	if got := len(payments.batchCalls); got != 1 {
		t.Fatalf("repeat after DONE sent %d batch calls, want 1", got)
	}
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-OK", PaymentNo: "PAY-PAY-OK", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("idempotent repeat: %v", err)
	}
	if got := len(payments.batchCalls); got != 1 {
		t.Fatalf("repeat after DONE sent %d batch calls, want 1", got)
	}
}

// T1.4b1-R: an incomplete admission must produce ZERO RefundBatch calls. The
// retry admits the missing group and then sends exactly one atomic batch.
func TestTccRefundBatchIsBlockedUntilEveryGroupIsAdmitted(t *testing.T) {
	db := orderTestDB(t)
	fulfillment := &fakeFulfillmentPort{status: "CANCELLED"}
	fulfillment.setFailCancels(1)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestServiceWithFulfillment(t, db, payments, fulfillment)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-PAY-ADM", 10000, 10000, 0, 0, "PAY-PAY-ADM")
	seedTccGroup(t, db, "ORD-PAY-ADM", "G1", 1, 0)
	seedTccGroup(t, db, "ORD-PAY-ADM", "G2", 2, 0)
	seedTccLine(t, db, "ORD-PAY-ADM", 1, "G1", 5000)
	seedTccLine(t, db, "ORD-PAY-ADM", 2, "G2", 5000)
	seedTccPermit(t, db, "ORD-PAY-ADM", "G2", permitConsumed, "SHIP-PAY-ADM-G2")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-ADM", PaymentNo: "PAY-PAY-ADM", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("incomplete admission = %v, want retryable UNAVAILABLE", err)
	}
	if len(payments.batchCalls) != 0 {
		t.Fatalf("partial admission sent %d batch calls; want zero", len(payments.batchCalls))
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-ADM", "PAY-PAY-ADM"); batch.State != refundBatchAdmitting {
		t.Fatalf("batch after incomplete admission = %s, want ADMITTING", batch.State)
	}

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-ADM", PaymentNo: "PAY-PAY-ADM", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if len(payments.batchCalls) != 1 {
		t.Fatalf("batch calls after complete admission = %d, want 1", len(payments.batchCalls))
	}
	assertBatchItems(t, payments.batchCalls[0], map[string]int64{
		tccGroupRefundID("PAY-PAY-ADM", "G1"): 5000,
		tccGroupRefundID("PAY-PAY-ADM", "G2"): 5000,
	})
}

// T1.4b1-R: BATCH_UNAVAILABLE keeps PAYING and the retry reuses the SAME
// batch_id/item set, so Payment can replay its persisted reply instead of
// treating the retry as a new batch.
func TestTccRefundBatchUnavailableKeepsPayingAndReplaysSameBatch(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{
		captured: 10000,
		batchErrs: []error{
			apperr.Unavailable("BATCH_UNAVAILABLE: payment db is unavailable", nil).
				WithDetails(map[string]any{"reason": refundBatchReasonUnavailable}),
		},
	}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	admittedTccBatch(t, db, svc, "ORD-PAY-RETRY", "PAY-PAY-RETRY", 10000, 6000, 4000)

	err := svc.payTccBatch(ctx, "ORD-PAY-RETRY", "PAY-PAY-RETRY")
	if err == nil || !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("BATCH_UNAVAILABLE = %v, want retryable UNAVAILABLE", err)
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-RETRY", "PAY-PAY-RETRY"); batch.State != refundBatchPaying {
		t.Fatalf("batch after BATCH_UNAVAILABLE = %s, want PAYING", batch.State)
	}
	if len(payments.batchCalls) != 1 {
		t.Fatalf("batch calls after failure = %d, want 1", len(payments.batchCalls))
	}

	if err := svc.payTccBatch(ctx, "ORD-PAY-RETRY", "PAY-PAY-RETRY"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if len(payments.batchCalls) != 2 {
		t.Fatalf("batch calls after retry = %d, want 2", len(payments.batchCalls))
	}
	first, second := payments.batchCalls[0], payments.batchCalls[1]
	if first.GetBatchId() != second.GetBatchId() || first.GetExpectedTotalCent() != second.GetExpectedTotalCent() {
		t.Fatalf("retry changed batch identity: %s/%d -> %s/%d",
			first.GetBatchId(), first.GetExpectedTotalCent(), second.GetBatchId(), second.GetExpectedTotalCent())
	}
	assertBatchItems(t, first, map[string]int64{
		tccGroupRefundID("PAY-PAY-RETRY", "G1"): 6000,
		tccGroupRefundID("PAY-PAY-RETRY", "G2"): 4000,
	})
	assertBatchItems(t, second, map[string]int64{
		tccGroupRefundID("PAY-PAY-RETRY", "G1"): 6000,
		tccGroupRefundID("PAY-PAY-RETRY", "G2"): 4000,
	})
}

// T1.4b1-R: every deterministic RefundBatch rejection parks the batch in
// MANUAL_HOLD with zero new Payment and disables automatic retries.
func TestTccRefundBatchDeterministicFailureParksManualHold(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{
		captured: 10000,
		batchErrs: []error{
			apperr.New(apperr.CodeFailedPrecondition, "INSUFFICIENT_REMAINING: remaining changed").
				WithDetails(map[string]any{"reason": "INSUFFICIENT_REMAINING"}),
		},
	}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	admittedTccBatch(t, db, svc, "ORD-PAY-REJ", "PAY-PAY-REJ", 10000, 6000, 4000)

	err := svc.payTccBatch(ctx, "ORD-PAY-REJ", "PAY-PAY-REJ")
	if err == nil || apperr.From(err).Details["tcc_error"] != tccErrBatchManual {
		t.Fatalf("deterministic rejection = %v, want TCC_BATCH_REQUIRES_MANUAL_HANDLING", err)
	}
	if apperr.From(err).Details["reason"] != "INSUFFICIENT_REMAINING" {
		t.Fatalf("manual hold details = %v, want reason INSUFFICIENT_REMAINING", apperr.From(err).Details)
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-REJ", "PAY-PAY-REJ"); batch.State != refundBatchManualHold {
		t.Fatalf("batch after rejection = %s, want MANUAL_HOLD", batch.State)
	}
	for _, group := range []string{"G1", "G2"} {
		if state, _ := loadCommitmentState(t, db, tccGroupRefundID("PAY-PAY-REJ", group)); state != commitmentReserved {
			t.Fatalf("%s commitment = %s, want RESERVED", group, state)
		}
	}
	before := len(payments.batchCalls)
	_, err = svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-REJ", PaymentNo: "PAY-PAY-REJ", PaidCent: 10000,
	})
	if err == nil || apperr.From(err).Details["tcc_error"] != tccErrBatchManual {
		t.Fatalf("automatic retry on MANUAL_HOLD = %v, want TCC_BATCH_REQUIRES_MANUAL_HANDLING", err)
	}
	if got := len(payments.batchCalls); got != before {
		t.Fatalf("MANUAL_HOLD batch paid again (%d -> %d)", before, got)
	}
}

// T1.4b1-R: same batch_id with a different item set is IDEMPOTENCY_CONFLICT and
// must park the batch, never be retried as a second partial payment.
func TestTccRefundBatchSameBatchDifferentItemsConflicts(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	batch := admittedTccBatch(t, db, svc, "ORD-PAY-IDEM", "PAY-PAY-IDEM", 10000, 6000, 4000)

	// Payment already persisted a reply for this batch_id with the FULL item
	// set; G1's refund fact is visible while the batch call for G2 would now
	// carry a different item set (same batch_id).
	g1 := tccGroupRefundID("PAY-PAY-IDEM", "G1")
	g2 := tccGroupRefundID("PAY-PAY-IDEM", "G2")
	payments.addRefundFact(&pb.Refund{
		RefundNo: "RFD-G1", RefundId: g1, PaymentNo: "PAY-PAY-IDEM",
		AmountCent: 6000, Status: tccRefundStatusSucceeded, CreatedAtUnix: time.Now().Unix(),
	})
	payments.refunded = 6000
	payments.batchReplies = map[string]*pb.RefundBatchReply{
		batch.BatchID: {
			BatchId: batch.BatchID, Status: refundBatchStatusRefunded, RefundedCent: 10000,
			ItemResults: []*pb.RefundBatchItemResult{
				{RefundId: g1, RefundNo: "RFD-G1", Status: refundBatchStatusRefunded, AmountCent: 6000},
				{RefundId: g2, RefundNo: "RFD-G2", Status: refundBatchStatusRefunded, AmountCent: 4000},
			},
		},
	}

	err := svc.payTccBatch(ctx, "ORD-PAY-IDEM", "PAY-PAY-IDEM")
	if err == nil || apperr.From(err).Details["tcc_error"] != tccErrBatchManual {
		t.Fatalf("same batch/different items = %v, want MANUAL_HOLD", err)
	}
	if reason := apperr.From(err).Details["reason"]; reason != "IDEMPOTENCY_CONFLICT" {
		t.Fatalf("reason = %v, want IDEMPOTENCY_CONFLICT", reason)
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-IDEM", "PAY-PAY-IDEM"); batch.State != refundBatchManualHold {
		t.Fatalf("batch = %s, want MANUAL_HOLD", batch.State)
	}
	if len(payments.refundFacts) != 1 {
		t.Fatalf("refund facts = %d, want only the pre-existing G1", len(payments.refundFacts))
	}
	if _, ok := payments.refundFacts[g2]; ok {
		t.Fatal("conflicting batch wrote G2")
	}
}

// T1.4b1-R: an external refund that lands AFTER the preflight (inside the
// RefundBatch window) is rejected by Payment's row-lock capacity check; Order
// must park with zero partial refunds (all-or-none).
func TestTccRefundBatchExternalRefundInWindowIsAllOrNone(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	payments.beforeBatch = func(f *fakePaymentPort) {
		// External 1000 refund committed between the Order preflight and the
		// Payment batch transaction.
		f.refunded += 1000
	}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	admittedTccBatch(t, db, svc, "ORD-PAY-RACE", "PAY-PAY-RACE", 10000, 6000, 4000)

	err := svc.payTccBatch(ctx, "ORD-PAY-RACE", "PAY-PAY-RACE")
	if err == nil || apperr.From(err).Details["tcc_error"] != tccErrBatchManual {
		t.Fatalf("in-window external refund = %v, want MANUAL_HOLD", err)
	}
	if reason := apperr.From(err).Details["reason"]; reason != "INSUFFICIENT_REMAINING" {
		t.Fatalf("reason = %v, want INSUFFICIENT_REMAINING", reason)
	}
	if len(payments.refundFacts) != 0 {
		t.Fatalf("partial batch wrote %d refund facts, want 0", len(payments.refundFacts))
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-RACE", "PAY-PAY-RACE"); batch.State != refundBatchManualHold {
		t.Fatalf("batch = %s, want MANUAL_HOLD", batch.State)
	}
	for _, group := range []string{"G1", "G2"} {
		if state, _ := loadCommitmentState(t, db, tccGroupRefundID("PAY-PAY-RACE", group)); state != commitmentReserved {
			t.Fatalf("%s commitment = %s, want RESERVED", group, state)
		}
	}
}

// T1.4b1-R §5: external refunds of 1000/6000/4000 must always produce
// MANUAL_HOLD and zero Payment, even when a single group alone would still fit
// under Payment's per-call cap.
func TestTccRefundBatchExternalRefundParksWithoutPayment(t *testing.T) {
	for _, tc := range []struct {
		name     string
		refunded int64
	}{
		{"external 1000", 9000},
		{"external 6000", 4000},
		{"external 4000", 6000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := orderTestDB(t)
			payments := &fakePaymentPort{captured: 10000}
			svc := tccTestService(t, db, payments)
			ctx := context.Background()
			admittedTccBatch(t, db, svc, "ORD-EXT", "PAY-EXT", 10000, 6000, 4000)
			// The external refund lands after Tx1/admission, before payment.
			payments.refunded = tc.refunded

			err := svc.payTccBatch(ctx, "ORD-EXT", "PAY-EXT")
			if err == nil || apperr.From(err).Details["tcc_error"] != tccErrBatchManual {
				t.Fatalf("external refund = %v, want MANUAL_HOLD", err)
			}
			if len(payments.batchCalls) != 0 {
				t.Fatalf("external refund sent %d RefundBatch calls, want 0", len(payments.batchCalls))
			}
			if batch := loadTccBatchRow(t, db, "ORD-EXT", "PAY-EXT"); batch.State != refundBatchManualHold {
				t.Fatalf("batch = %s, want MANUAL_HOLD", batch.State)
			}
			for _, group := range []string{"G1", "G2"} {
				if state, _ := loadCommitmentState(t, db, tccGroupRefundID("PAY-EXT", group)); state != commitmentReserved {
					t.Fatalf("%s commitment = %s, want RESERVED", group, state)
				}
			}
		})
	}
}

// T1.4b1-R: GetRefundByKey success with a lost payment.refunded event is lazily
// reconciled (commitment SETTLED/app DONE/batch DONE) without paying again.
func TestTccRefundBatchLazyReconcileRepairsLostEvents(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	admittedTccBatch(t, db, svc, "ORD-PAY-LOST", "PAY-PAY-LOST", 10000, 6000, 4000)

	// Payment committed both refunds; the Order projection never consumed the
	// events (commitments RESERVED, applications PROCESSING).
	for _, group := range []struct {
		group  string
		amount int64
	}{{"G1", 6000}, {"G2", 4000}} {
		payments.addRefundFact(&pb.Refund{
			RefundNo: "RFD-" + group.group, RefundId: tccGroupRefundID("PAY-PAY-LOST", group.group),
			PaymentNo: "PAY-PAY-LOST", AmountCent: group.amount,
			Status: tccRefundStatusSucceeded, CreatedAtUnix: time.Now().Unix(),
		})
	}
	payments.refunded = 10000

	if err := svc.payTccBatch(ctx, "ORD-PAY-LOST", "PAY-PAY-LOST"); err != nil {
		t.Fatal(err)
	}
	if len(payments.batchCalls) != 0 {
		t.Fatalf("lost-event reconcile sent %d batch calls, want 0 (no re-pay)", len(payments.batchCalls))
	}
	for _, group := range []string{"G1", "G2"} {
		refundID := tccGroupRefundID("PAY-PAY-LOST", group)
		if state, _ := loadCommitmentState(t, db, refundID); state != commitmentSettled {
			t.Fatalf("%s commitment = %s, want SETTLED", group, state)
		}
		if appState, _, _, _, _ := loadTccApplication(t, ctx, db, refundID); appState != refundApplicationDone {
			t.Fatalf("%s application = %s, want DONE", group, appState)
		}
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-LOST", "PAY-PAY-LOST"); batch.State != refundBatchDone {
		t.Fatalf("batch after reconcile = %s, want DONE", batch.State)
	}
	if refunded, refundStatus, refundState, _ := loadOrderRefundProjection(t, db, "ORD-PAY-LOST"); refunded != 10000 ||
		refundStatus != statusRefunded || refundState != refundStateFull {
		t.Fatalf("pay-path projection = %d/%s/%s, want 10000/REFUNDED/FULL",
			refunded, refundStatus, refundState)
	}
	// A late/duplicate event replay stays idempotent.
	event := paymentRefundedEvent{
		PaymentNo: "PAY-PAY-LOST", OrderNo: "ORD-PAY-LOST",
		RefundID:   tccGroupRefundID("PAY-PAY-LOST", "G2"),
		AmountCent: 4000, RefundedCent: 10000, OrderRefundedCent: 10000, CapturedCent: 10000,
	}
	if err := applyPaymentRefunded(ctx, svc, event); err != nil {
		t.Fatalf("event replay after lazy reconcile: %v", err)
	}
}

// T1.4b1-R: lazy reconciliation must not close a partly refunded batch. Only the
// group with a Payment fact is reconciled; the outstanding group is still paid
// by exactly one RefundBatch item.
func TestTccRefundBatchLazyReconcileKeepsPartialBatchPaying(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	admittedTccBatch(t, db, svc, "ORD-PAY-MIX", "PAY-PAY-MIX", 10000, 6000, 4000)

	g1 := tccGroupRefundID("PAY-PAY-MIX", "G1")
	g2 := tccGroupRefundID("PAY-PAY-MIX", "G2")
	payments.addRefundFact(&pb.Refund{
		RefundNo: "RFD-G1", RefundId: g1, PaymentNo: "PAY-PAY-MIX",
		AmountCent: 6000, Status: tccRefundStatusSucceeded, CreatedAtUnix: time.Now().Unix(),
	})
	payments.refunded = 6000

	if err := svc.payTccBatch(ctx, "ORD-PAY-MIX", "PAY-PAY-MIX"); err != nil {
		t.Fatal(err)
	}
	if len(payments.batchCalls) != 1 {
		t.Fatalf("batch calls = %d, want 1 for the outstanding group only", len(payments.batchCalls))
	}
	assertBatchItems(t, payments.batchCalls[0], map[string]int64{g2: 4000})
	if got := payments.batchCalls[0].GetExpectedTotalCent(); got != 4000 {
		t.Fatalf("expected_total_cent = %d, want outstanding 4000 (not B)", got)
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-MIX", "PAY-PAY-MIX"); batch.State != refundBatchPaying {
		t.Fatalf("batch = %s, want PAYING (G2 still unprojected)", batch.State)
	}
	if state, _ := loadCommitmentState(t, db, g1); state != commitmentSettled {
		t.Fatalf("G1 commitment = %s, want SETTLED", state)
	}
	if state, _ := loadCommitmentState(t, db, g2); state != commitmentReserved {
		t.Fatalf("G2 commitment = %s, want RESERVED", state)
	}
	if appState, _, _, _, _ := loadTccApplication(t, ctx, db, g1); appState != refundApplicationDone {
		t.Fatalf("G1 application = %s, want DONE", appState)
	}
	if refunded, refundStatus, refundState, _ := loadOrderRefundProjection(t, db, "ORD-PAY-MIX"); refunded != 6000 ||
		refundStatus != statusPartiallyRefunded || refundState != refundStateRefunding {
		t.Fatalf("partial pay-path projection = %d/%s/%s, want 6000/PARTIALLY_REFUNDED/REFUNDING",
			refunded, refundStatus, refundState)
	}

	// A second pass sees G2's successful fact and closes the batch without a
	// second payment.
	if err := svc.payTccBatch(ctx, "ORD-PAY-MIX", "PAY-PAY-MIX"); err != nil {
		t.Fatal(err)
	}
	if len(payments.batchCalls) != 1 {
		t.Fatalf("repeat sent %d batch calls, want 1", len(payments.batchCalls))
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-MIX", "PAY-PAY-MIX"); batch.State != refundBatchDone {
		t.Fatalf("batch after G2 fact = %s, want DONE", batch.State)
	}
}

// T1.4b1-R: an in-flight group refund blocks every new group; the retry first
// converges the same refund_id and only then pays the outstanding group.
func TestTccRefundBatchInFlightStopsNewGroupsUntilConverged(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	admittedTccBatch(t, db, svc, "ORD-PAY-FLY", "PAY-PAY-FLY", 10000, 6000, 4000)

	g1 := tccGroupRefundID("PAY-PAY-FLY", "G1")
	g2 := tccGroupRefundID("PAY-PAY-FLY", "G2")
	payments.addRefundFact(&pb.Refund{
		RefundNo: "RFD-G1", RefundId: g1, PaymentNo: "PAY-PAY-FLY",
		AmountCent: 6000, Status: tccRefundStatusProcessing, CreatedAtUnix: time.Now().Unix(),
	})

	err := svc.payTccBatch(ctx, "ORD-PAY-FLY", "PAY-PAY-FLY")
	if err == nil || !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("in-flight group = %v, want retryable UNAVAILABLE", err)
	}
	if len(payments.batchCalls) != 0 {
		t.Fatalf("in-flight group started %d new batches, want 0", len(payments.batchCalls))
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-FLY", "PAY-PAY-FLY"); batch.State != refundBatchPaying {
		t.Fatalf("batch = %s, want PAYING", batch.State)
	}

	// The same refund_id converges; now only G2 is outstanding.
	payments.refundFacts[g1].Status = tccRefundStatusSucceeded
	payments.refunded = 6000
	if err := svc.payTccBatch(ctx, "ORD-PAY-FLY", "PAY-PAY-FLY"); err != nil {
		t.Fatal(err)
	}
	if len(payments.batchCalls) != 1 {
		t.Fatalf("batch calls after convergence = %d, want 1", len(payments.batchCalls))
	}
	assertBatchItems(t, payments.batchCalls[0], map[string]int64{g2: 4000})
}

// T1.4b1-R: an in-flight refund older than the operational timeout is not
// retried forever; the batch is parked for manual handling.
func TestTccRefundBatchInFlightTimeoutParksManualHold(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	admittedTccBatch(t, db, svc, "ORD-PAY-FLY-TO", "PAY-PAY-FLY-TO", 10000, 6000, 4000)

	payments.addRefundFact(&pb.Refund{
		RefundNo: "RFD-G1", RefundId: tccGroupRefundID("PAY-PAY-FLY-TO", "G1"),
		PaymentNo: "PAY-PAY-FLY-TO", AmountCent: 6000, Status: tccRefundStatusPending,
		CreatedAtUnix: time.Now().Add(-2 * time.Hour).Unix(),
	})

	err := svc.payTccBatch(ctx, "ORD-PAY-FLY-TO", "PAY-PAY-FLY-TO")
	if err == nil || apperr.From(err).Details["tcc_error"] != tccErrBatchManual {
		t.Fatalf("timed-out in-flight = %v, want MANUAL_HOLD", err)
	}
	if reason := apperr.From(err).Details["reason"]; reason != "TCC_REFUND_IN_FLIGHT_TIMEOUT" {
		t.Fatalf("reason = %v, want TCC_REFUND_IN_FLIGHT_TIMEOUT", reason)
	}
	if len(payments.batchCalls) != 0 {
		t.Fatalf("timed-out in-flight started %d batch calls, want 0", len(payments.batchCalls))
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-FLY-TO", "PAY-PAY-FLY-TO"); batch.State != refundBatchManualHold {
		t.Fatalf("batch = %s, want MANUAL_HOLD", batch.State)
	}
}

// T1.4b1-R: a GetRefundByKey timeout is fail-closed but retryable: no Payment,
// no MANUAL_HOLD, the batch stays ADMITTED for the next attempt.
func TestTccRefundBatchGetRefundByKeyTimeoutFailsClosed(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	admittedTccBatch(t, db, svc, "ORD-PAY-TO", "PAY-PAY-TO", 10000, 6000, 4000)

	payments.getRefundErr = apperr.Unavailable("payment refund read timeout", nil)
	err := svc.payTccBatch(ctx, "ORD-PAY-TO", "PAY-PAY-TO")
	if err == nil || !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("GetRefundByKey timeout = %v, want retryable UNAVAILABLE", err)
	}
	if len(payments.batchCalls) != 0 {
		t.Fatalf("timeout sent %d batch calls, want 0", len(payments.batchCalls))
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-TO", "PAY-PAY-TO"); batch.State != refundBatchAdmitted {
		t.Fatalf("batch after read timeout = %s, want ADMITTED", batch.State)
	}

	payments.getRefundErr = nil
	if err := svc.payTccBatch(ctx, "ORD-PAY-TO", "PAY-PAY-TO"); err != nil {
		t.Fatalf("retry after timeout: %v", err)
	}
	if len(payments.batchCalls) != 1 {
		t.Fatalf("batch calls after retry = %d, want 1", len(payments.batchCalls))
	}
}

// T1.4b1-R: a group can only be paid from ADMITTED+RESERVED. A missing
// commitment makes the whole batch fail closed with zero RefundBatch.
func TestTccRefundBatchRequiresReservedCommitment(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-PAY-NOQ", 10000, 10000, 0, 0, "PAY-PAY-NOQ")
	seedTccGroup(t, db, "ORD-PAY-NOQ", "G1", 1, 0)
	seedTccLine(t, db, "ORD-PAY-NOQ", 1, "G1", 10000)

	if err := runTccTx1(t, ctx, svc, db, "ORD-PAY-NOQ", "PAY-PAY-NOQ", 10000); err != nil {
		t.Fatal(err)
	}
	if err := svc.admitTccBatch(ctx, "ORD-PAY-NOQ", "PAY-PAY-NOQ"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM order_refund_commitments WHERE refund_id = ?`,
		tccGroupRefundID("PAY-PAY-NOQ", "G1")); err != nil {
		t.Fatal(err)
	}

	err := svc.payTccBatch(ctx, "ORD-PAY-NOQ", "PAY-PAY-NOQ")
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("payment without commitment = %v, want FAILED_PRECONDITION", err)
	}
	if len(payments.batchCalls) != 0 {
		t.Fatalf("batch calls without commitment = %d, want 0", len(payments.batchCalls))
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-NOQ", "PAY-PAY-NOQ"); batch.State != refundBatchAdmitted {
		t.Fatalf("batch after failed precheck = %s, want ADMITTED (no money moved)", batch.State)
	}
}

// T1.4b1-R: money may only move from an ADMITTED batch, never from FROZEN or
// ADMITTING.
func TestTccRefundBatchRefusesNonAdmittedBatch(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-PAY-FROZEN", 10000, 10000, 0, 0, "PAY-PAY-FROZEN")
	seedTccGroup(t, db, "ORD-PAY-FROZEN", "G1", 1, 0)
	seedTccLine(t, db, "ORD-PAY-FROZEN", 1, "G1", 10000)

	if err := runTccTx1(t, ctx, svc, db, "ORD-PAY-FROZEN", "PAY-PAY-FROZEN", 10000); err != nil {
		t.Fatal(err)
	}
	err := svc.payTccBatch(ctx, "ORD-PAY-FROZEN", "PAY-PAY-FROZEN")
	if err == nil || !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("payment on FROZEN batch = %v, want retryable UNAVAILABLE", err)
	}
	if len(payments.batchCalls) != 0 {
		t.Fatalf("FROZEN batch calls = %d, want 0", len(payments.batchCalls))
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-FROZEN", "PAY-PAY-FROZEN"); batch.State != refundBatchFrozen {
		t.Fatalf("batch = %s, want FROZEN", batch.State)
	}
}

// Compile-time guard: the fake used by this file must satisfy the C-04 port.
var _ paymentPort = (*fakePaymentPort)(nil)
