package order

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// T1.4b1 evidence (SPEC-0002 v1.9 AC-51/AC-65, ADR-0007 Amendment 1 §4.1.2
// steps 6-7, T1.4a Rulings Addendum R-B). Real MySQL: the guarantees are the
// transactional batch/application/commitment ledger plus Payment idempotency.

func refundCallsFor(payments *fakePaymentPort, refundID string) []*pb.RefundRequest {
	calls := make([]*pb.RefundRequest, 0, 1)
	for _, call := range payments.refundCalls {
		if call.GetIdempotencyKey() == refundID {
			calls = append(calls, call)
		}
	}
	return calls
}

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

// T1.4b1: after every group is admitted the batch enters PAYING and each group
// gets exactly one Payment.Refund with refund_id=tcc:<payment_no>:<group_no>
// and amount=group cap. Commitments stay RESERVED until payment.refunded.
func TestTccPaymentPaysEveryAdmittedGroupOnce(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-PAY-OK", 10000, 10000, 500, 0, "PAY-PAY-OK")
	seedTccGroup(t, db, "ORD-PAY-OK", "G1", 1, 300)
	seedTccGroup(t, db, "ORD-PAY-OK", "G2", 2, 200)
	seedTccLine(t, db, "ORD-PAY-OK", 1, "G1", 5500)
	seedTccLine(t, db, "ORD-PAY-OK", 2, "G2", 4000)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-OK", PaymentNo: "PAY-PAY-OK", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	batch := loadTccBatchRow(t, db, "ORD-PAY-OK", "PAY-PAY-OK")
	if batch.State != refundBatchPaying {
		t.Fatalf("batch state = %s, want PAYING (payment accepted, events pending)", batch.State)
	}
	if len(payments.refundCalls) != 2 {
		t.Fatalf("refund calls = %d, want one per group", len(payments.refundCalls))
	}
	for _, want := range []struct {
		group  string
		amount int64
	}{{"G1", 5800}, {"G2", 4200}} {
		refundID := tccGroupRefundID("PAY-PAY-OK", want.group)
		calls := refundCallsFor(payments, refundID)
		if len(calls) != 1 {
			t.Fatalf("%s refund_id calls = %d, want 1", want.group, len(calls))
		}
		call := calls[0]
		if call.GetPaymentNo() != "PAY-PAY-OK" || call.GetAmountCent() != want.amount {
			t.Fatalf("%s refund = payment %q amount %d, want PAY-PAY-OK/%d",
				want.group, call.GetPaymentNo(), call.GetAmountCent(), want.amount)
		}
		if call.GetNonEffectiveCapture() {
			t.Fatalf("%s group refund must be an effective capture", want.group)
		}
		if call.GetReason() == "" {
			t.Fatalf("%s refund has no reason", want.group)
		}
		state, amount := loadCommitmentState(t, db, refundID)
		if state != commitmentReserved || amount != want.amount {
			t.Fatalf("%s commitment after payment = %s/%d, want RESERVED/%d (ADMITTED is not refunded)",
				want.group, state, amount, want.amount)
		}
		if appState, _, _, _, _ := loadTccApplication(t, ctx, db, refundID); appState != refundApplicationProcessing {
			t.Fatalf("%s application after payment = %s, want PROCESSING until payment.refunded", want.group, appState)
		}
	}

	// Idempotent repeat: same refund_ids, no new ledger rows, still PAYING.
	before := len(payments.refundCalls)
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-OK", PaymentNo: "PAY-PAY-OK", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	if got := len(payments.refundCalls) - before; got != 2 {
		t.Fatalf("repeat refund calls = %d, want 2 idempotent re-calls", got)
	}
	for _, group := range []string{"G1", "G2"} {
		if got := len(refundCallsFor(payments, tccGroupRefundID("PAY-PAY-OK", group))); got != 2 {
			t.Fatalf("%s refund_id calls after repeat = %d, want 2 with the same key", group, got)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE order_no = 'ORD-PAY-OK'`); got != 2 {
		t.Fatalf("applications after repeat = %d, want 2", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-PAY-OK'`); got != 2 {
		t.Fatalf("commitments after repeat = %d, want 2", got)
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-OK", "PAY-PAY-OK"); batch.State != refundBatchPaying {
		t.Fatalf("batch after repeat = %s, want PAYING", batch.State)
	}

	// payment.refunded is the only closure path: commitment SETTLED, app DONE,
	// batch DONE after the last group, order fully refunded, lifecycle untouched.
	for _, event := range []paymentRefundedEvent{
		{PaymentNo: "PAY-PAY-OK", OrderNo: "ORD-PAY-OK", RefundID: tccGroupRefundID("PAY-PAY-OK", "G1"),
			AmountCent: 5800, RefundedCent: 5800, OrderRefundedCent: 5800, CapturedCent: 10000},
		{PaymentNo: "PAY-PAY-OK", OrderNo: "ORD-PAY-OK", RefundID: tccGroupRefundID("PAY-PAY-OK", "G2"),
			AmountCent: 4200, RefundedCent: 10000, OrderRefundedCent: 10000, CapturedCent: 10000},
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
		t.Fatalf("batch after payment.refunded = %s, want DONE (PAID semantics)", batch.State)
	}
	var status, refundState string
	var refunded int64
	if err := db.QueryRow(`SELECT status, refund_state, refunded_cent FROM orders WHERE order_no = 'ORD-PAY-OK'`).
		Scan(&status, &refundState, &refunded); err != nil {
		t.Fatal(err)
	}
	if status != statusPaid || refundState != refundStateFull || refunded != 10000 {
		t.Fatalf("order after refunds = %s/%s/%d, want PAID/FULL/10000", status, refundState, refunded)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-PAY-OK' AND event_type = 'order.refund_projected'`); got != 2 {
		t.Fatalf("order.refund_projected events = %d, want 2", got)
	}
}

// T1.4b1: an incomplete admission (one group's CancelIfCreated fails) must
// produce ZERO Payment.Refund calls. The retry admits the missing group and
// only then pays both.
func TestTccPaymentIsBlockedUntilEveryGroupIsAdmitted(t *testing.T) {
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
	if len(payments.refundCalls) != 0 {
		t.Fatalf("partial admission refunded %d groups; want zero", len(payments.refundCalls))
	}
	batch := loadTccBatchRow(t, db, "ORD-PAY-ADM", "PAY-PAY-ADM")
	if batch.State != refundBatchAdmitting {
		t.Fatalf("batch after incomplete admission = %s, want ADMITTING", batch.State)
	}

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-ADM", PaymentNo: "PAY-PAY-ADM", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if len(payments.refundCalls) != 2 {
		t.Fatalf("refund calls after complete admission = %d, want 2", len(payments.refundCalls))
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-ADM", "PAY-PAY-ADM"); batch.State != refundBatchPaying {
		t.Fatalf("batch after retry = %s, want PAYING", batch.State)
	}
}

// T1.4b1: a transient Payment failure keeps the batch PAYING (retryable) and a
// retry reuses exactly the same refund_id, so Payment cannot double-refund.
func TestTccPaymentRetryReusesRefundIDAfterTransientFailure(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{
		captured: 10000,
		// call 0: G1 succeeds; call 1: G2 times out; retries succeed.
		refundErrs: []error{nil, errors.New("payment refund timeout")},
	}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-PAY-RETRY", 10000, 10000, 0, 0, "PAY-PAY-RETRY")
	seedTccGroup(t, db, "ORD-PAY-RETRY", "G1", 1, 0)
	seedTccGroup(t, db, "ORD-PAY-RETRY", "G2", 2, 0)
	seedTccLine(t, db, "ORD-PAY-RETRY", 1, "G1", 6000)
	seedTccLine(t, db, "ORD-PAY-RETRY", 2, "G2", 4000)

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-RETRY", PaymentNo: "PAY-PAY-RETRY", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("first payment attempt = %v, want retryable UNAVAILABLE", err)
	}
	if len(payments.refundCalls) != 2 {
		t.Fatalf("first attempt refund calls = %d, want 2 (G1 paid, G2 failed)", len(payments.refundCalls))
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-RETRY", "PAY-PAY-RETRY"); batch.State != refundBatchPaying {
		t.Fatalf("batch after transient failure = %s, want PAYING for retry", batch.State)
	}
	for _, group := range []string{"G1", "G2"} {
		if state, _ := loadCommitmentState(t, db, tccGroupRefundID("PAY-PAY-RETRY", group)); state != commitmentReserved {
			t.Fatalf("%s commitment after failure = %s, want RESERVED (no partial refunded state)", group, state)
		}
	}
	var attempts int
	if err := db.QueryRow(`SELECT attempts FROM refund_applications WHERE refund_id = ?`,
		tccGroupRefundID("PAY-PAY-RETRY", "G2")).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Fatalf("failed group attempts = %d, want 1", attempts)
	}

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-RETRY", PaymentNo: "PAY-PAY-RETRY", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	g1 := refundCallsFor(payments, tccGroupRefundID("PAY-PAY-RETRY", "G1"))
	g2 := refundCallsFor(payments, tccGroupRefundID("PAY-PAY-RETRY", "G2"))
	if len(g1) != 2 || len(g2) != 2 {
		t.Fatalf("refund calls G1=%d G2=%d, want 2 each (same refund_id re-used)", len(g1), len(g2))
	}
	if g1[0].GetIdempotencyKey() != g1[1].GetIdempotencyKey() || g1[0].GetAmountCent() != g1[1].GetAmountCent() {
		t.Fatal("G1 retry changed refund_id/amount")
	}
	if g2[0].GetIdempotencyKey() != g2[1].GetIdempotencyKey() || g2[0].GetAmountCent() != g2[1].GetAmountCent() {
		t.Fatal("G2 retry changed refund_id/amount")
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-PAY-RETRY'`); got != 1 {
		t.Fatalf("batches after retry = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-PAY-RETRY'`); got != 2 {
		t.Fatalf("commitments after retry = %d, want 2", got)
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-RETRY", "PAY-PAY-RETRY"); batch.State != refundBatchPaying {
		t.Fatalf("batch after successful retry = %s, want PAYING until events", batch.State)
	}
}

// T1.4b1: a definitive Payment rejection (for example a changed captured amount)
// stops the fan-out and parks the batch in MANUAL_HOLD; it never reports success
// and never auto-retries over a human decision.
func TestTccPaymentRejectionParksBatchForManualHandling(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{
		captured:   10000,
		refundErrs: []error{apperr.Invalid("refund amount exceeds remaining refundable amount 0")},
	}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-PAY-REJ", 10000, 10000, 0, 0, "PAY-PAY-REJ")
	seedTccGroup(t, db, "ORD-PAY-REJ", "G1", 1, 0)
	seedTccGroup(t, db, "ORD-PAY-REJ", "G2", 2, 0)
	seedTccLine(t, db, "ORD-PAY-REJ", 1, "G1", 6000)
	seedTccLine(t, db, "ORD-PAY-REJ", 2, "G2", 4000)

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-REJ", PaymentNo: "PAY-PAY-REJ", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("definitive rejection = %v, want FAILED_PRECONDITION", err)
	}
	if len(payments.refundCalls) != 1 {
		t.Fatalf("refund calls after rejection = %d, want 1 (fan-out stopped)", len(payments.refundCalls))
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-REJ", "PAY-PAY-REJ"); batch.State != refundBatchManualHold {
		t.Fatalf("batch after rejection = %s, want MANUAL_HOLD", batch.State)
	}
	if state, _ := loadCommitmentState(t, db, tccGroupRefundID("PAY-PAY-REJ", "G1")); state != commitmentReserved {
		t.Fatalf("G1 commitment = %s, want RESERVED (rejection is not a refund)", state)
	}

	before := len(payments.refundCalls)
	_, err = svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-REJ", PaymentNo: "PAY-PAY-REJ", PaidCent: 10000,
	})
	if err == nil || apperr.From(err).Details["tcc_error"] != tccErrBatchManual {
		t.Fatalf("automatic retry on MANUAL_HOLD = %v, want TCC_BATCH_REQUIRES_MANUAL_HANDLING", err)
	}
	if len(payments.refundCalls) != before {
		t.Fatalf("MANUAL_HOLD batch paid again (%d -> %d)", before, len(payments.refundCalls))
	}
}

// T1.4b1/R-B: a group can only be paid from ADMITTED+RESERVED. A missing
// commitment makes the whole fan-out fail closed with zero refunds.
func TestTccPaymentRequiresReservedCommitment(t *testing.T) {
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
	if batch := loadTccBatchRow(t, db, "ORD-PAY-NOQ", "PAY-PAY-NOQ"); batch.State != refundBatchAdmitted {
		t.Fatalf("batch after admission = %s, want ADMITTED", batch.State)
	}
	if _, err := db.Exec(`DELETE FROM order_refund_commitments WHERE refund_id = ?`,
		tccGroupRefundID("PAY-PAY-NOQ", "G1")); err != nil {
		t.Fatal(err)
	}

	err := svc.payTccBatch(ctx, "ORD-PAY-NOQ", "PAY-PAY-NOQ")
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("payment without commitment = %v, want FAILED_PRECONDITION", err)
	}
	if len(payments.refundCalls) != 0 {
		t.Fatalf("refund calls without commitment = %d, want 0", len(payments.refundCalls))
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-NOQ", "PAY-PAY-NOQ"); batch.State != refundBatchAdmitted {
		t.Fatalf("batch after failed precheck = %s, want ADMITTED (no money moved)", batch.State)
	}
}

// T1.4b1: money may only move from an ADMITTED batch, never from FROZEN or
// ADMITTING.
func TestTccPaymentRefusesNonAdmittedBatch(t *testing.T) {
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
	if len(payments.refundCalls) != 0 {
		t.Fatalf("FROZEN batch refund calls = %d, want 0", len(payments.refundCalls))
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-FROZEN", "PAY-PAY-FROZEN"); batch.State != refundBatchFrozen {
		t.Fatalf("batch = %s, want FROZEN", batch.State)
	}
}

// T1.4b1: a retry that races the payment.refunded projection must not be
// rejected. Payment may already show the group refunds while the Order ledger
// is still RESERVED (event not consumed yet); the ADMITTED/PAYING resume path
// relies on Payment idempotency and does not re-run the pre-admission Σ gate.
func TestTccPaymentRetryToleratesUnprojectedRefund(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-PAY-LAG", 10000, 10000, 0, 0, "PAY-PAY-LAG")
	seedTccGroup(t, db, "ORD-PAY-LAG", "G1", 1, 0)
	seedTccGroup(t, db, "ORD-PAY-LAG", "G2", 2, 0)
	seedTccLine(t, db, "ORD-PAY-LAG", 1, "G1", 5800)
	seedTccLine(t, db, "ORD-PAY-LAG", 2, "G2", 4200)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-LAG", PaymentNo: "PAY-PAY-LAG", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	// Payment executed both refunds; the Order projection has not consumed the
	// events yet (commitments stay RESERVED).
	payments.refunded = 10000
	before := len(payments.refundCalls)
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-PAY-LAG", PaymentNo: "PAY-PAY-LAG", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("retry during the payment.refunded lag window: %v", err)
	}
	if got := len(payments.refundCalls) - before; got != 2 {
		t.Fatalf("lag-window retry refund calls = %d, want 2 idempotent re-calls", got)
	}
	if batch := loadTccBatchRow(t, db, "ORD-PAY-LAG", "PAY-PAY-LAG"); batch.State != refundBatchPaying {
		t.Fatalf("batch after lag-window retry = %s, want PAYING", batch.State)
	}
	for _, group := range []string{"G1", "G2"} {
		if state, _ := loadCommitmentState(t, db, tccGroupRefundID("PAY-PAY-LAG", group)); state != commitmentReserved {
			t.Fatalf("%s commitment = %s, want RESERVED until payment.refunded", group, state)
		}
	}
}
