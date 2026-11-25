package order

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// T1.7 evidence (SPEC-0002 v1.10 AC-50/AC-51/AC-65, INV-44/INV-48; ADR-0007
// Amendment 1 §4.1.2/§4.1.3/§6.1/§6.2). Every case below runs against real
// MySQL with the real C-05 refund migrations; Payment and Fulfillment are
// fakes so the failure/race windows can be injected deterministically.
//
// The cases are deliberately end-to-end (TccCancelMarkPaid -> Tx1 -> per-group
// admission -> RefundBatch) because T1.7 is the evidence task for the complete
// compensation protocol, not another unit test of one helper.

// T1.7 §1: a Σ mismatch is a hard stop. The only allowed write is the
// PRECHECK_FAILED audit batch; order, permits, applications, commitments and
// outbox are untouched and Payment is never called.
func TestT17SigmaMismatchWritesAuditAndNeverCallsPayment(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-T17-SIGMA", 10000, 10000, 0, 0, "PAY-T17-SIGMA")
	seedTccGroup(t, db, "ORD-T17-SIGMA", "G1", 1, 0)
	// Σ active group caps = 9000, authoritative remaining captured = 10000.
	seedTccLine(t, db, "ORD-T17-SIGMA", 1, "G1", 9000)
	seedTccPermit(t, db, "ORD-T17-SIGMA", "G1", permitConsumed, "SHIP-T17-SIGMA")
	version := tccVersion(t, db, "ORD-T17-SIGMA")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-T17-SIGMA", PaymentNo: "PAY-T17-SIGMA", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("Σ mismatch error = %v, want FAILED_PRECONDITION", err)
	}

	// Exactly one audit row, with the computed numbers, and it is not resumable.
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-T17-SIGMA'`); got != 1 {
		t.Fatalf("audit batches = %d, want exactly 1", got)
	}
	batch := loadTccBatchRow(t, db, "ORD-T17-SIGMA", "PAY-T17-SIGMA")
	if batch.State != refundBatchFailedPre || batch.AttemptSeq != 1 ||
		batch.BatchID != "tcc:PAY-T17-SIGMA:1" {
		t.Fatalf("audit batch = %+v, want tcc:PAY-T17-SIGMA:1/PRECHECK_FAILED/attempt 1", batch)
	}
	if batch.SumCapCent != 9000 || batch.RemainingCent != 10000 {
		t.Fatalf("audit numbers = sum %d/remaining %d, want 9000/10000", batch.SumCapCent, batch.RemainingCent)
	}

	// R-1: the audit batch is the only write.
	assertNoTccStateWrites(t, db, "ORD-T17-SIGMA", version)
	var permitState string
	var permitVersion int64
	var shipmentNo string
	if err := db.QueryRow(`SELECT state, version, shipment_no FROM order_shipping_permits
		WHERE order_no = 'ORD-T17-SIGMA' AND fulfillment_group_no = 'G1'`).
		Scan(&permitState, &permitVersion, &shipmentNo); err != nil {
		t.Fatal(err)
	}
	if permitState != permitConsumed || permitVersion != 1 || shipmentNo != "SHIP-T17-SIGMA" {
		t.Fatalf("Σ mismatch touched the permit: %s/version %d/shipment %q", permitState, permitVersion, shipmentNo)
	}

	// Zero Payment: no per-group Refund and no atomic RefundBatch.
	if len(payments.refundCalls) != 0 || len(payments.batchCalls) != 0 {
		t.Fatalf("Σ mismatch called Payment: Refund=%d RefundBatch=%d",
			len(payments.refundCalls), len(payments.batchCalls))
	}
}

// T1.7 §2: once the refund admission has revoked the group permits under
// REFUNDING, a concurrent AuthorizeShipment must lose. The permit is the
// authority: even if the group refund_state projection is forced back to NONE,
// the revoked permit still rejects the shipment, so no shipment is issued.
func TestT17RefundingRejectsConcurrentAuthorizeShipment(t *testing.T) {
	db := orderTestDB(t)
	payments := &countingPaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-T17-SHIP", 10000, 10000, 200, 0, "PAY-T17-SHIP")
	seedTccGroup(t, db, "ORD-T17-SHIP", "G1", 1, 100)
	seedTccGroup(t, db, "ORD-T17-SHIP", "G2", 2, 100)
	seedTccLine(t, db, "ORD-T17-SHIP", 1, "G1", 4900)
	seedTccLine(t, db, "ORD-T17-SHIP", 2, "G2", 4900)

	// Freeze the refund (Tx1): REFUNDING + G1/G2 permits REVOKED + applications.
	if err := runTccTx1(t, ctx, svc, db, "ORD-T17-SHIP", "PAY-T17-SHIP", 10000); err != nil {
		t.Fatalf("Tx1: %v", err)
	}
	for _, groupNo := range []string{"G1", "G2"} {
		var state string
		if err := db.QueryRow(`SELECT state FROM order_shipping_permits
			WHERE order_no = 'ORD-T17-SHIP' AND fulfillment_group_no = ?`, groupNo).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state != permitRevoked {
			t.Fatalf("%s permit after Tx1 = %s, want REVOKED", groupNo, state)
		}
	}
	// Prove the permit, not the display projection, is what blocks shipping.
	if _, err := db.Exec(`UPDATE order_fulfillment_groups SET refund_state = 'NONE'
		WHERE order_no = 'ORD-T17-SHIP'`); err != nil {
		t.Fatal(err)
	}

	const shipWorkers = 8
	const refundWorkers = 2
	shipErrs := make([]error, shipWorkers)
	refundErrs := make([]error, refundWorkers)
	var wg sync.WaitGroup
	wg.Add(shipWorkers + refundWorkers)
	for i := 0; i < shipWorkers; i++ {
		go func(i int) {
			defer wg.Done()
			groupNo := "G1"
			if i%2 == 1 {
				groupNo = "G2"
			}
			_, shipErrs[i] = svc.AuthorizeShipment(ctx, &pb.ShipmentPermitRequest{
				OrderNo: "ORD-T17-SHIP", FulfillmentGroupNo: groupNo,
				ShipmentNo: fmt.Sprintf("SHIP-T17-RACE-%d", i),
			})
		}(i)
	}
	for i := 0; i < refundWorkers; i++ {
		go func(i int) {
			defer wg.Done()
			_, refundErrs[i] = svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
				OrderNo: "ORD-T17-SHIP", PaymentNo: "PAY-T17-SHIP", PaidCent: 10000,
			})
		}(i)
	}
	wg.Wait()

	for i, err := range shipErrs {
		if err == nil {
			t.Fatalf("ship worker %d won the race against the refund admission", i)
		}
		if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
			t.Fatalf("ship worker %d error = %v, want FAILED_PRECONDITION", i, err)
		}
	}
	for i, err := range refundErrs {
		if err != nil {
			t.Fatalf("refund worker %d: %v", i, err)
		}
	}

	// No shipment was issued: every permit stayed REVOKED without a shipment_no.
	for _, groupNo := range []string{"G1", "G2"} {
		var state, shipmentNo string
		var version int64
		if err := db.QueryRow(`SELECT state, shipment_no, version FROM order_shipping_permits
			WHERE order_no = 'ORD-T17-SHIP' AND fulfillment_group_no = ?`, groupNo).
			Scan(&state, &shipmentNo, &version); err != nil {
			t.Fatal(err)
		}
		if state != permitRevoked || shipmentNo != "" {
			t.Fatalf("%s permit after race = %s/shipment %q, want REVOKED/no shipment", groupNo, state, shipmentNo)
		}
	}
	// The refund itself stays on the durable PAYING ledger (no payment.refunded
	// consumer runs here); money was accepted atomically for both groups.
	batch := loadTccBatchRow(t, db, "ORD-T17-SHIP", "PAY-T17-SHIP")
	if batch.State != refundBatchPaying && batch.State != refundBatchDone {
		t.Fatalf("batch after race = %s, want PAYING or DONE", batch.State)
	}
	if payments.refundCalls != 0 {
		t.Fatalf("per-group Refund calls = %d, want 0 (atomic RefundBatch only)", payments.refundCalls)
	}

	// Final direct proof: the permit alone rejects shipping once every other
	// projection is stale (order/group refund_state NONE, permit REVOKED).
	if _, err := db.Exec(`UPDATE order_fulfillment_groups SET refund_state = 'NONE'
		WHERE order_no = 'ORD-T17-SHIP'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE orders SET refund_state = 'NONE', refund_status = ''
		WHERE order_no = 'ORD-T17-SHIP'`); err != nil {
		t.Fatal(err)
	}
	_, err := svc.AuthorizeShipment(ctx, &pb.ShipmentPermitRequest{
		OrderNo: "ORD-T17-SHIP", FulfillmentGroupNo: "G1", ShipmentNo: "SHIP-T17-LATE",
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("post-refund AuthorizeShipment = %v, want FAILED_PRECONDITION", err)
	}
	if msg := apperr.From(err).Message; !strings.Contains(msg, "revoked") {
		t.Fatalf("post-refund AuthorizeShipment = %q, want the revoked permit refusal", msg)
	}
}

// T1.7 §3: a crash after Tx1 (batch FROZEN, no payment) must resume the same
// batch. A payment-side failure keeps the batch recoverable, and the retry
// sends only the groups that have no successful Payment fact yet.
func TestT17CrashAfterTx1ResumesOnlyUnfinishedGroups(t *testing.T) {
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
	seedTccOrder(t, db, "ORD-T17-CRASH", 10000, 10000, 0, 0, "PAY-T17-CRASH")
	seedTccGroup(t, db, "ORD-T17-CRASH", "G1", 1, 0)
	seedTccGroup(t, db, "ORD-T17-CRASH", "G2", 2, 0)
	seedTccLine(t, db, "ORD-T17-CRASH", 1, "G1", 6000)
	seedTccLine(t, db, "ORD-T17-CRASH", 2, "G2", 4000)

	// Crash point: Tx1 committed, the process died before any Payment call.
	if err := runTccTx1(t, ctx, svc, db, "ORD-T17-CRASH", "PAY-T17-CRASH", 10000); err != nil {
		t.Fatalf("Tx1: %v", err)
	}
	crashed := loadTccBatchRow(t, db, "ORD-T17-CRASH", "PAY-T17-CRASH")
	if crashed.State != refundBatchFrozen {
		t.Fatalf("batch after simulated crash = %s, want FROZEN", crashed.State)
	}
	if len(payments.batchCalls) != 0 || len(payments.refundCalls) != 0 {
		t.Fatalf("Tx1-only crash already called Payment")
	}

	// New process resumes, but Payment is transiently unavailable: admission
	// may finish, no money may move, and the batch stays recoverable.
	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-T17-CRASH", PaymentNo: "PAY-T17-CRASH", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("resume with Payment unavailable = %v, want UNAVAILABLE", err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-T17-CRASH'`); got != 1 {
		t.Fatalf("resume created a second batch: %d", got)
	}
	resumed := loadTccBatchRow(t, db, "ORD-T17-CRASH", "PAY-T17-CRASH")
	if resumed.BatchID != crashed.BatchID || resumed.State != refundBatchPaying {
		t.Fatalf("batch after failed payment = %s/%s, want %s/PAYING", resumed.BatchID, resumed.State, crashed.BatchID)
	}
	if len(payments.batchCalls) != 1 {
		t.Fatalf("RefundBatch calls after failed payment = %d, want 1", len(payments.batchCalls))
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM refund_applications WHERE order_no = 'ORD-T17-CRASH'`,
		`SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-T17-CRASH'`,
		`SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-T17-CRASH'`,
	} {
		if got := countRows(t, db, query); got != 2 {
			t.Fatalf("resume duplicated ledger rows (%s -> %d, want 2)", query, got)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications
		WHERE order_no = 'ORD-T17-CRASH' AND state = ?`, refundApplicationDone); got != 0 {
		t.Fatalf("failed payment marked %d application(s) DONE", got)
	}

	// Payment reports G1 succeeded (event lost): the retry must deduct it and
	// send only G2. G1 is never refunded twice.
	if payments.refundFacts == nil {
		payments.refundFacts = make(map[string]*pb.Refund)
	}
	payments.refundFacts[tccGroupRefundID("PAY-T17-CRASH", "G1")] = &pb.Refund{
		RefundNo: "RFD-T17-G1", RefundId: tccGroupRefundID("PAY-T17-CRASH", "G1"),
		PaymentNo: "PAY-T17-CRASH", AmountCent: 6000, Status: tccRefundStatusSucceeded,
	}
	payments.refunded = 6000

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-T17-CRASH", PaymentNo: "PAY-T17-CRASH", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("retry after G1 fact: %v", err)
	}
	if len(payments.batchCalls) != 2 {
		t.Fatalf("RefundBatch calls after retry = %d, want 2", len(payments.batchCalls))
	}
	second := payments.batchCalls[1]
	if second.GetBatchId() != resumed.BatchID {
		t.Fatalf("retry changed batch identity: %s -> %s", resumed.BatchID, second.GetBatchId())
	}
	if second.GetExpectedTotalCent() != 4000 {
		t.Fatalf("retry expected_total_cent = %d, want only G2's 4000", second.GetExpectedTotalCent())
	}
	assertBatchItems(t, second, map[string]int64{
		tccGroupRefundID("PAY-T17-CRASH", "G2"): 4000,
	})
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-T17-CRASH'`); got != 1 {
		t.Fatalf("retry created another batch: %d", got)
	}
	if len(payments.refundCalls) != 0 {
		t.Fatalf("per-group Refund calls = %d, want 0", len(payments.refundCalls))
	}
}

// T1.7 §4: one group failing admission (CancelIfCreated error) must block every
// payment, including the group that was already admitted; the batch stays in a
// recoverable ADMITTING state and the retry finishes the missing group without
// duplicating the ledger.
func TestT17PartialGroupFailureBlocksAllPaymentsAndStaysRecoverable(t *testing.T) {
	db := orderTestDB(t)
	fulfillment := &fakeFulfillmentPort{status: "CANCELLED"}
	fulfillment.setFailCancels(1)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestServiceWithFulfillment(t, db, payments, fulfillment)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-T17-PARTIAL", 10000, 10000, 0, 0, "PAY-T17-PARTIAL")
	seedTccGroup(t, db, "ORD-T17-PARTIAL", "G1", 1, 0)
	seedTccGroup(t, db, "ORD-T17-PARTIAL", "G2", 2, 0)
	seedTccLine(t, db, "ORD-T17-PARTIAL", 1, "G1", 5000)
	seedTccLine(t, db, "ORD-T17-PARTIAL", 2, "G2", 5000)
	// G2 has an authorized (not yet shipped) shipment: its admission goes
	// through CancelIfCreated, which is injected to fail once.
	seedTccPermit(t, db, "ORD-T17-PARTIAL", "G2", permitConsumed, "SHIP-T17-PARTIAL-G2")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-T17-PARTIAL", PaymentNo: "PAY-T17-PARTIAL", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("partial admission = %v, want retryable UNAVAILABLE", err)
	}
	if len(payments.refundCalls) != 0 || len(payments.batchCalls) != 0 {
		t.Fatalf("partial admission paid something: Refund=%d RefundBatch=%d",
			len(payments.refundCalls), len(payments.batchCalls))
	}
	batch := loadTccBatchRow(t, db, "ORD-T17-PARTIAL", "PAY-T17-PARTIAL")
	if batch.State != refundBatchAdmitting {
		t.Fatalf("batch after partial failure = %s, want recoverable ADMITTING", batch.State)
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM refund_applications WHERE order_no = 'ORD-T17-PARTIAL'`,
		`SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-T17-PARTIAL'`,
	} {
		if got := countRows(t, db, query); got != 2 {
			t.Fatalf("partial failure ledger (%s -> %d, want 2)", query, got)
		}
	}

	// Retry: only G2's CancelIfCreated is repeated, then one atomic RefundBatch
	// covers both groups.
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-T17-PARTIAL", PaymentNo: "PAY-T17-PARTIAL", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got := fulfillment.cancelCalls.Load(); got != 2 {
		t.Fatalf("CancelIfCreated calls = %d, want 2 (one injected failure + one retry)", got)
	}
	if len(payments.batchCalls) != 1 {
		t.Fatalf("RefundBatch calls after retry = %d, want 1", len(payments.batchCalls))
	}
	assertBatchItems(t, payments.batchCalls[0], map[string]int64{
		tccGroupRefundID("PAY-T17-PARTIAL", "G1"): 5000,
		tccGroupRefundID("PAY-T17-PARTIAL", "G2"): 5000,
	})
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-T17-PARTIAL'`); got != 2 {
		t.Fatalf("admissions after retry = %d, want 2", got)
	}
}

// T1.7 §5: a historical order with no fulfillment group row takes the
// versioned G1 protocol only when the frozen lines are evidence for G1. With no
// group row and no line evidence there is nothing to attribute, so the code
// must fail closed instead of guessing G1 (or W1).
func TestT17LegacyNoGroupUsesG1OnlyWithEvidence(t *testing.T) {
	t.Run("line evidence maps the historical order to G1", func(t *testing.T) {
		db := orderTestDB(t)
		payments := &fakePaymentPort{captured: 8000}
		svc := tccTestService(t, db, payments)
		ctx := context.Background()
		seedTccOrder(t, db, "ORD-T17-LEGACY", 8000, 8000, 0, 0, "PAY-T17-LEGACY")
		seedTccLine(t, db, "ORD-T17-LEGACY", 1, "G1", 8000)

		if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
			OrderNo: "ORD-T17-LEGACY", PaymentNo: "PAY-T17-LEGACY", PaidCent: 8000,
		}); err != nil {
			t.Fatalf("legacy G1 compensation: %v", err)
		}
		var permitState string
		if err := db.QueryRow(`SELECT state FROM order_shipping_permits
			WHERE order_no = 'ORD-T17-LEGACY' AND fulfillment_group_no = 'G1'`).Scan(&permitState); err != nil {
			t.Fatal(err)
		}
		if permitState != permitRevoked {
			t.Fatalf("legacy G1 permit = %s, want REVOKED", permitState)
		}
		var amount int64
		var result string
		if err := db.QueryRow(`SELECT amount_cent, admission_result FROM refund_applications
			WHERE refund_id = 'tcc:PAY-T17-LEGACY:G1'`).Scan(&amount, &result); err != nil {
			t.Fatal(err)
		}
		if amount != 8000 || result != refundResultPre {
			t.Fatalf("legacy G1 application = %d/%s, want 8000/PRE", amount, result)
		}
		if len(payments.batchCalls) != 1 {
			t.Fatalf("legacy G1 RefundBatch calls = %d, want 1", len(payments.batchCalls))
		}
		assertBatchItems(t, payments.batchCalls[0], map[string]int64{
			tccGroupRefundID("PAY-T17-LEGACY", "G1"): 8000,
		})
	})

	t.Run("no group and no line evidence fails closed without guessing G1", func(t *testing.T) {
		db := orderTestDB(t)
		payments := &fakePaymentPort{captured: 10000}
		svc := tccTestService(t, db, payments)
		ctx := context.Background()
		seedTccOrder(t, db, "ORD-T17-NOEV", 10000, 10000, 0, 0, "PAY-T17-NOEV")
		version := tccVersion(t, db, "ORD-T17-NOEV")

		_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
			OrderNo: "ORD-T17-NOEV", PaymentNo: "PAY-T17-NOEV", PaidCent: 10000,
		})
		if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
			t.Fatalf("no-evidence legacy order = %v, want FAILED_PRECONDITION", err)
		}
		// A Σ audit row is allowed; no synthetic G1 permit/application exists.
		if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-T17-NOEV' AND state = ?`, refundBatchFailedPre); got != 1 {
			t.Fatalf("no-evidence audit batches = %d, want 1", got)
		}
		assertNoTccStateWrites(t, db, "ORD-T17-NOEV", version)
		if got := countRows(t, db, `SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = 'ORD-T17-NOEV'`); got != 0 {
			t.Fatalf("no-evidence order created %d synthetic permit(s)", got)
		}
		if len(payments.refundCalls) != 0 || len(payments.batchCalls) != 0 {
			t.Fatalf("no-evidence order called Payment: Refund=%d RefundBatch=%d",
				len(payments.refundCalls), len(payments.batchCalls))
		}
	})
}

// T1.7 §6: an already shipped group is POST_SHIP_ALLOWED. Refund must not
// cancel its shipment, must not write an inventory release, and must not enter
// the AfterSales restock path; only the group refund is paid.
func TestT17ShippedGroupPostWritesNoReleaseAndNoRestock(t *testing.T) {
	db := orderTestDB(t)
	fulfillment := &fakeFulfillmentPort{status: "CANCELLED", shipments: []*pb.Shipment{{
		ShipmentNo: "SHIP-T17-POST", OrderNo: "ORD-T17-POST", FulfillmentGroupNo: "G1",
		Status: "SHIPPED",
	}}}
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestServiceWithFulfillment(t, db, payments, fulfillment)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-T17-POST", 10000, 10000, 0, 0, "PAY-T17-POST")
	seedTccGroup(t, db, "ORD-T17-POST", "G1", 1, 0)
	seedTccLine(t, db, "ORD-T17-POST", 1, "G1", 10000)
	seedTccPermit(t, db, "ORD-T17-POST", "G1", permitConsumed, "SHIP-T17-POST")

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-T17-POST", PaymentNo: "PAY-T17-POST", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("shipped POST compensation: %v", err)
	}
	var state, result string
	if err := db.QueryRow(`SELECT state, admission_result FROM refund_applications
		WHERE refund_id = 'tcc:PAY-T17-POST:G1'`).Scan(&state, &result); err != nil {
		t.Fatal(err)
	}
	if state != refundApplicationProcessing || result != refundResultPost {
		t.Fatalf("shipped application = %s/%s, want PROCESSING/POST", state, result)
	}
	var admissionResult string
	if err := db.QueryRow(`SELECT result FROM order_refund_admissions
		WHERE refund_id = 'tcc:PAY-T17-POST:G1'`).Scan(&admissionResult); err != nil {
		t.Fatal(err)
	}
	if admissionResult != refundResultPost {
		t.Fatalf("shipped admission = %s, want POST", admissionResult)
	}
	// No CancelIfCreated for a group that already left the warehouse.
	if got := fulfillment.cancelCalls.Load(); got != 0 {
		t.Fatalf("shipped group called CancelIfCreated %d times", got)
	}
	// Permit stays CONSUMED and untouched; no release/restock/compensation path.
	var permitState string
	var permitVersion int64
	if err := db.QueryRow(`SELECT state, version FROM order_shipping_permits
		WHERE order_no = 'ORD-T17-POST' AND fulfillment_group_no = 'G1'`).Scan(&permitState, &permitVersion); err != nil {
		t.Fatal(err)
	}
	if permitState != permitConsumed || permitVersion != 1 {
		t.Fatalf("shipped permit = %s/version %d, want CONSUMED/version 1", permitState, permitVersion)
	}
	if releases := loadRefundReleaseEvents(t, db, "ORD-T17-POST"); len(releases) != 0 {
		t.Fatalf("POST shipped refund wrote %d inventory release event(s)", len(releases))
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-T17-POST'
			AND (event_type LIKE '%restock%' OR topic LIKE '%restock%' OR topic LIKE '%aftersale%')`,
		`SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-T17-POST'
			AND event_type IN ('inventory.compensation_requested', 'inventory.refund_release_requested')`,
	} {
		if got := countRows(t, db, query); got != 0 {
			t.Fatalf("shipped POST refund triggered a release/restock path (%s -> %d)", query, got)
		}
	}
	if len(payments.batchCalls) != 1 {
		t.Fatalf("shipped POST RefundBatch calls = %d, want 1", len(payments.batchCalls))
	}
	assertBatchItems(t, payments.batchCalls[0], map[string]int64{
		tccGroupRefundID("PAY-T17-POST", "G1"): 10000,
	})
}
