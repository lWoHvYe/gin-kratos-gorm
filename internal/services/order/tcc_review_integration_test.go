package order

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// AC-71 / T1 (ADR-0007 Amd1 AC-50 Addendum 0.6 §3.6/§7.2 W12): a TCC batch
// whose group admission resolves to REVIEW_REQUIRED must read the decision as a
// legal persisted value, park the batch in MANUAL_HOLD, keep the application
// PENDING and pay nothing. Automatic retries return
// TCC_BATCH_REQUIRES_MANUAL_HANDLING with zero writes; only an audited manual
// re-arm resumes the batch.

func tccReviewFixture(t *testing.T, orderNo, paymentNo string) (*Service, *fakePaymentPort, *fakeFulfillmentPort, string) {
	t.Helper()
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	ff := &fakeFulfillmentPort{status: probeStatusNoWaybill}
	svc := tccTestServiceWithFulfillment(t, db, payments, ff)
	seedTccOrder(t, db, orderNo, 10000, 10000, 0, 0, paymentNo)
	seedTccGroup(t, db, orderNo, "G1", 1, 0)
	seedTccLine(t, db, orderNo, 1, "G1", 10000)
	seedTccPermit(t, db, orderNo, "G1", permitConsumed, "SHIP-"+orderNo)
	return svc, payments, ff, tccGroupRefundID(paymentNo, "G1")
}

func TestTCCReviewManualHold(t *testing.T) {
	ctx := context.Background()
	svc, payments, ff, refundID := tccReviewFixture(t, "ORD-TCC-R1", "PAY-TCC-R1")
	db := svc.db

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-R1", PaymentNo: "PAY-TCC-R1", PaidCent: 10000,
	})
	if err == nil {
		t.Fatal("TCC no-waybill: expected TCC_BATCH_REQUIRES_MANUAL_HANDLING")
	}
	ae := apperr.From(err)
	if ae == nil || ae.Code != apperr.CodeFailedPrecondition {
		t.Fatalf("error = %v, want FAILED_PRECONDITION manual handling", err)
	}
	if got, _ := ae.Details["tcc_error"].(string); got != tccErrBatchManual {
		t.Fatalf("tcc_error = %q, want %q (details=%v)", got, tccErrBatchManual, ae.Details)
	}
	batch := loadTccBatchRow(t, db, "ORD-TCC-R1", "PAY-TCC-R1")
	if batch.State != refundBatchManualHold {
		t.Fatalf("batch = %s, want MANUAL_HOLD", batch.State)
	}
	var lastError string
	if err := db.QueryRow(`SELECT last_error FROM refund_batches WHERE batch_id = ?`, batch.BatchID).Scan(&lastError); err != nil {
		t.Fatal(err)
	}
	wantError := refundReviewCanonical(refundID, refundReasonConsumedWithoutShipment, refundBatchLastErrorWidth)
	if lastError != wantError {
		t.Fatalf("refund_batches.last_error = %q, want canonical %q", lastError, wantError)
	}
	appState, appResult, _, _, _ := loadTccApplication(t, ctx, db, refundID)
	if appState != refundApplicationPending || appResult != refundReviewRequired {
		t.Fatalf("application = %s/%s, want PENDING/REVIEW_REQUIRED", appState, appResult)
	}
	var admResult, admState string
	if err := db.QueryRow(`SELECT result, state FROM order_refund_admissions WHERE refund_id = ?`, refundID).
		Scan(&admResult, &admState); err != nil {
		t.Fatal(err)
	}
	if admResult != refundReviewRequired || admState != refundPending {
		t.Fatalf("admission = %s/%s, want REVIEW_REQUIRED/PENDING_REMOTE", admResult, admState)
	}
	if got := len(payments.batchCalls); got != 0 {
		t.Fatalf("Payment.RefundBatch calls = %d, want 0", got)
	}
	var commitmentState string
	if err := db.QueryRow(`SELECT state FROM order_refund_commitments WHERE refund_id = ?`, refundID).Scan(&commitmentState); err != nil {
		t.Fatal(err)
	}
	if commitmentState != commitmentReserved {
		t.Fatalf("commitment = %s, want RESERVED", commitmentState)
	}

	// W12: an automatic retry returns the stable token and changes nothing.
	batchVersion := tccVersionOfBatch(t, db, batch.BatchID)
	appVersion := tccVersionOfApplication(t, db, refundID)
	admVersion := tccVersionOfAdmission(t, db, refundID)
	_, err = svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-R1", PaymentNo: "PAY-TCC-R1", PaidCent: 10000,
	})
	if err == nil {
		t.Fatal("MANUAL_HOLD retry: expected the stable manual token")
	}
	if got, _ := apperr.From(err).Details["tcc_error"].(string); got != tccErrBatchManual {
		t.Fatalf("retry tcc_error = %q, want %q", got, tccErrBatchManual)
	}
	if tccVersionOfBatch(t, db, batch.BatchID) != batchVersion ||
		tccVersionOfApplication(t, db, refundID) != appVersion ||
		tccVersionOfAdmission(t, db, refundID) != admVersion {
		t.Fatal("MANUAL_HOLD retry changed persisted state")
	}

	// O2b/O2c: replaying the same refund_id returns the same decision without
	// probing or reading Payment (even when Payment is unavailable).
	cancelCalls := ff.cancelCalls.Load()
	getCalls := payments.getCalls
	req := &pb.AdmitRefundRequest{
		RefundId: refundID, OrderNo: "ORD-TCC-R1", GroupNo: "G1",
		AmountCent: 10000, Source: refundApplicationSourceTCC, BatchId: batch.BatchID,
	}
	if _, replayErr := svc.AdmitRefund(ctx, req); replayErr == nil {
		t.Fatal("review replay: expected REFUND_ADMISSION_CONFLICT")
	} else {
		ac50RequireReviewConflict(t, replayErr, refundID)
	}
	if ff.cancelCalls.Load() != cancelCalls || payments.getCalls != getCalls {
		t.Fatalf("review replay probed: cancel %d->%d get %d->%d",
			cancelCalls, ff.cancelCalls.Load(), getCalls, payments.getCalls)
	}
	downPayments := &fakePaymentPort{getErr: errors.New("payment unavailable")}
	downFF := &fakeFulfillmentPort{status: probeStatusNoWaybill}
	downSvc := tccTestServiceWithFulfillment(t, db, downPayments, downFF)
	if _, err := downSvc.AdmitRefund(ctx, req); err == nil {
		t.Fatal("review replay with Payment down: expected the same conflict")
	} else {
		ac50RequireReviewConflict(t, err, refundID)
	}
	if downPayments.getCalls != 0 || downFF.cancelCalls.Load() != 0 {
		t.Fatalf("review replay with Payment down probed: get=%d cancel=%d",
			downPayments.getCalls, downFF.cancelCalls.Load())
	}
}

func TestTCCReviewRearmResumes(t *testing.T) {
	ctx := context.Background()
	svc, payments, ff, refundID := tccReviewFixture(t, "ORD-TCC-R2", "PAY-TCC-R2")
	db := svc.db
	req := &pb.TccPaymentRequest{OrderNo: "ORD-TCC-R2", PaymentNo: "PAY-TCC-R2", PaidCent: 10000}
	if _, err := svc.TccCancelMarkPaid(ctx, req); err == nil {
		t.Fatal("expected MANUAL_HOLD first")
	}
	batch := loadTccBatchRow(t, db, "ORD-TCC-R2", "PAY-TCC-R2")
	if batch.State != refundBatchManualHold {
		t.Fatalf("batch = %s, want MANUAL_HOLD", batch.State)
	}

	// Simulated real S6 evidence: Fulfillment now reports a real CANCELLED row.
	ff.setStatus("CANCELLED")
	// §8.3 ops re-arm: admission result -> PENDING_REMOTE (version CAS), batch
	// MANUAL_HOLD -> ADMITTING (version CAS). The audit/ticket fields are kept.
	res, err := db.Exec(`UPDATE order_refund_admissions
		SET result = ?, version = version + 1, updated_at = NOW(3)
		WHERE refund_id = ? AND result = ? AND state = ?`,
		refundPending, refundID, refundReviewRequired, refundPending)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatal("re-arm did not reset the review admission")
	}
	res, err = db.Exec(`UPDATE refund_batches
		SET state = ?, version = version + 1, updated_at = NOW(3)
		WHERE batch_id = ? AND state = ?`, refundBatchAdmitting, batch.BatchID, refundBatchManualHold)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatal("re-arm did not move the batch to ADMITTING")
	}

	if _, err := svc.TccCancelMarkPaid(ctx, req); err != nil {
		t.Fatalf("re-armed TCC: %v", err)
	}
	admResult, admState, _ := ac50Admission(t, db, refundID)
	if admResult != refundResultPre || admState != refundResultPre {
		t.Fatalf("re-armed admission = %s/%s, want PRE/PRE", admResult, admState)
	}
	appState, appResult, _, _, _ := loadTccApplication(t, ctx, db, refundID)
	if appResult != refundResultPre || (appState != refundApplicationProcessing && appState != refundApplicationDone) {
		t.Fatalf("re-armed application = %s/%s, want PROCESSING|DONE/PRE", appState, appResult)
	}
	if got := len(payments.batchCalls); got != 1 {
		t.Fatalf("Payment.RefundBatch calls = %d, want 1", got)
	}
	var lastError string
	if err := db.QueryRow(`SELECT last_error FROM refund_batches WHERE batch_id = ?`, batch.BatchID).Scan(&lastError); err != nil {
		t.Fatal(err)
	}
	if lastError != refundReviewCanonical(refundID, refundReasonConsumedWithoutShipment, refundBatchLastErrorWidth) {
		t.Fatalf("re-arm lost the canonical audit text: %q", lastError)
	}
}

func tccVersionOfBatch(t *testing.T, db *sql.DB, batchID string) int64 {
	t.Helper()
	var version int64
	if err := db.QueryRow(`SELECT version FROM refund_batches WHERE batch_id = ?`, batchID).Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}

func tccVersionOfApplication(t *testing.T, db *sql.DB, refundID string) int64 {
	t.Helper()
	var version int64
	if err := db.QueryRow(`SELECT version FROM refund_applications WHERE refund_id = ?`, refundID).Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}

func tccVersionOfAdmission(t *testing.T, db *sql.DB, refundID string) int64 {
	t.Helper()
	var version int64
	if err := db.QueryRow(`SELECT version FROM order_refund_admissions WHERE refund_id = ?`, refundID).Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}

// T3-R1 (Reviewer BLOCKER): a fresh Tx1 REVIEW reached from a TCC source must
// apply the same T1c human-hold gate as Tx2. The batch goes MANUAL_HOLD, the
// application is mirrored and the automatic resume returns the stable token;
// if the mirror matches 0 rows the whole Tx1 rolls back.
func TestTCCReviewFreshSourceGate(t *testing.T) {
	ctx := context.Background()
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	ff := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := tccTestServiceWithFulfillment(t, db, payments, ff)

	seed := func(orderNo, paymentNo, refundID, batchID string, amount int64, shipped, hasPermit bool) {
		status := statusPaid
		if shipped {
			status = statusShipped
		}
		ac50SeedOrderStatus(t, db, orderNo, 10000, status)
		if hasPermit {
			ac50SeedPermit(t, db, orderNo, permitConsumed)
		}
		seedTccBatchRow(t, db, batchID, orderNo, paymentNo, refundBatchAdmitting)
		seedTccApplicationRow(t, db, refundID, batchID, orderNo, "G1", refundApplicationPending, refundPending, amount, amount, 0)
		if _, err := db.Exec(`UPDATE orders SET refund_state = ? WHERE order_no = ?`, refundStateRefunding, orderNo); err != nil {
			t.Fatal(err)
		}
	}

	// source=tcc partial + unclassifiable projection (OD4/W10 through TCC).
	refundID := tccGroupRefundID("PAY-TCC-FP", "G1")
	seed("ORD-TCC-FP", "PAY-TCC-FP", refundID, "B-TCC-FP", 5000, true, true)
	_, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: refundID, OrderNo: "ORD-TCC-FP", GroupNo: "G1",
		AmountCent: 5000, Source: refundApplicationSourceTCC, BatchId: "B-TCC-FP",
	})
	ae := requireReviewDetails(t, err, refundID, refundReasonPartialUnclassifiable)
	if got, _ := ae.Details["state"].(string); got != refundPending {
		t.Fatalf("state = %q", got)
	}
	var batchState, lastError string
	if err := db.QueryRow(`SELECT state, last_error FROM refund_batches WHERE batch_id = 'B-TCC-FP'`).
		Scan(&batchState, &lastError); err != nil {
		t.Fatal(err)
	}
	if batchState != refundBatchManualHold {
		t.Fatalf("fresh partial TCC batch = %s, want MANUAL_HOLD", batchState)
	}
	if want := refundReviewCanonical(refundID, refundReasonPartialUnclassifiable, refundBatchLastErrorWidth); lastError != want {
		t.Fatalf("batch last_error = %q, want %q", lastError, want)
	}
	appState, appResult, _, _, _ := loadTccApplication(t, ctx, db, refundID)
	if appState != refundApplicationPending || appResult != refundReviewRequired {
		t.Fatalf("application = %s/%s, want PENDING/REVIEW", appState, appResult)
	}
	if err := svc.admitTccBatch(ctx, "ORD-TCC-FP", "PAY-TCC-FP"); err == nil {
		t.Fatal("automatic resume: expected TCC_BATCH_REQUIRES_MANUAL_HANDLING")
	} else if got, _ := apperr.From(err).Details["tcc_error"].(string); got != tccErrBatchManual {
		t.Fatalf("resume token = %q, want %q", got, tccErrBatchManual)
	}
	if ff.cancelCalls.Load() != 0 || len(payments.batchCalls) != 0 {
		t.Fatalf("fresh partial TCC called probe/payment: cancel=%d batch=%d", ff.cancelCalls.Load(), len(payments.batchCalls))
	}

	// source=tcc legacy unclassifiable (no permit, no group row, order SHIPPED).
	legacyID := tccGroupRefundID("PAY-TCC-FL", "G1")
	seed("ORD-TCC-FL", "PAY-TCC-FL", legacyID, "B-TCC-FL", 10000, true, false)
	_, err = svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: legacyID, OrderNo: "ORD-TCC-FL", GroupNo: "G1",
		AmountCent: 10000, Source: refundApplicationSourceTCC, BatchId: "B-TCC-FL",
	})
	requireReviewDetails(t, err, legacyID, refundReasonLegacyShipmentUnknown)
	if err := db.QueryRow(`SELECT state FROM refund_batches WHERE batch_id = 'B-TCC-FL'`).Scan(&batchState); err != nil {
		t.Fatal(err)
	}
	if batchState != refundBatchManualHold {
		t.Fatalf("fresh legacy TCC batch = %s, want MANUAL_HOLD", batchState)
	}

	// 0-row mirror gate: batch not ADMITTING -> no REVIEW admission committed.
	gateID := tccGroupRefundID("PAY-TCC-GATE", "G1")
	seed("ORD-TCC-GATE", "PAY-TCC-GATE", gateID, "B-TCC-GATE", 5000, true, true)
	if _, err := db.Exec(`UPDATE refund_batches SET state = ? WHERE batch_id = 'B-TCC-GATE'`, refundBatchPaying); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: gateID, OrderNo: "ORD-TCC-GATE", GroupNo: "G1",
		AmountCent: 5000, Source: refundApplicationSourceTCC, BatchId: "B-TCC-GATE",
	}); err == nil {
		t.Fatal("0-row batch mirror: expected fail-closed")
	} else if !strings.Contains(err.Error(), "batch state changed while persisting review") {
		t.Fatalf("gate error = %v", err)
	}
	var admissions, commitments int
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id = ?`, gateID).Scan(&admissions); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = ?`, gateID).Scan(&commitments); err != nil {
		t.Fatal(err)
	}
	if admissions != 0 || commitments != 0 {
		t.Fatalf("0-row gate committed admission=%d commitments=%d", admissions, commitments)
	}
}

// T7-R2: the TCC batch last_error column is 1024 characters; a synthetic
// over-long refund_id must be truncated exactly per D-35 (prefix + truncated
// refund_id + ` truncated=1`) without cutting the reason/ticket keys.
func TestTCCReviewLastErrorTruncation1024(t *testing.T) {
	longID := strings.Repeat("r", 2000)
	text := refundReviewCanonical(longID, refundReasonPartialUnclassifiable, refundBatchLastErrorWidth)
	if len([]rune(text)) > refundBatchLastErrorWidth {
		t.Fatalf("canonical length = %d, want <= %d", len([]rune(text)), refundBatchLastErrorWidth)
	}
	if !strings.HasSuffix(text, " truncated=1") {
		t.Fatalf("truncated text = %q, want trailing ' truncated=1'", text)
	}
	prefix := "REVIEW_REQUIRED reason=" + refundReasonPartialUnclassifiable + " ticket=RVW:"
	if !strings.HasPrefix(text, prefix) {
		t.Fatalf("truncated text lost the prefix: %q", text)
	}
	if strings.Count(text, "reason=") != 1 || strings.Count(text, "ticket=") != 1 {
		t.Fatalf("truncation duplicated a key: %q", text)
	}
}

// T7-R2: the Tx2 gate must fail closed when the application mirror matches 0
// rows; the batch mirror update is rolled back with the admission decision.
func TestTCCReviewGateApplicationMirrorZeroRows(t *testing.T) {
	ctx := context.Background()
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	ff := &fakeFulfillmentPort{status: probeStatusNoWaybill}
	svc := tccTestServiceWithFulfillment(t, db, payments, ff)
	refundID := tccGroupRefundID("PAY-TCC-APP0", "G1")
	ac50SeedOrder(t, db, "ORD-TCC-APP0", 10000)
	ac50SeedPermit(t, db, "ORD-TCC-APP0", permitConsumed)
	seedTccBatchRow(t, db, "B-TCC-APP0", "ORD-TCC-APP0", "PAY-TCC-APP0", refundBatchAdmitting)
	// Admission placeholder exists (Tx1), application row deliberately missing.
	if _, err := db.Exec(`INSERT INTO order_refund_admissions
		(refund_id, order_no, group_no, result, state, amount_cent, version)
		VALUES (?, 'ORD-TCC-APP0', 'G1', ?, ?, 10000, 1)`, refundID, refundResultPost, refundPending); err != nil {
		t.Fatal(err)
	}
	_, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: refundID, OrderNo: "ORD-TCC-APP0", GroupNo: "G1",
		AmountCent: 10000, Source: refundApplicationSourceTCC, BatchId: "B-TCC-APP0",
	})
	if err == nil {
		t.Fatal("0-row application mirror: expected fail-closed")
	}
	if !strings.Contains(err.Error(), "tcc refund application is not PENDING while persisting review") {
		t.Fatalf("error = %v, want the application mirror gate", err)
	}
	result, state, version := ac50Admission(t, db, refundID)
	if result != refundResultPost || state != refundPending || version != 1 {
		t.Fatalf("Tx2 committed without the application mirror: %s/%s v%d", result, state, version)
	}
	var batchState string
	var batchVersion int64
	if err := db.QueryRow(`SELECT state, version FROM refund_batches WHERE batch_id = 'B-TCC-APP0'`).
		Scan(&batchState, &batchVersion); err != nil {
		t.Fatal(err)
	}
	if batchState != refundBatchAdmitting || batchVersion != 1 {
		t.Fatalf("batch mirror was committed without the app mirror: %s v%d", batchState, batchVersion)
	}
}

// T7-R2: an empty batch_id on a TCC review fails closed before any Tx1 write.
func TestTCCReviewEmptyBatchIDFailsClosed(t *testing.T) {
	ctx := context.Background()
	db := orderTestDB(t)
	svc := tccTestServiceWithFulfillment(t, db, &fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ac50SeedOrderStatus(t, db, "ORD-TCC-NOB", 10000, statusShipped)
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-TCC-NOB", OrderNo: "ORD-TCC-NOB", GroupNo: "G1",
		AmountCent: 10000, Source: refundApplicationSourceTCC, BatchId: "",
	}); err == nil {
		t.Fatal("empty batch_id: expected fail-closed")
	} else if !strings.Contains(err.Error(), "requires batch_id") {
		t.Fatalf("error = %v, want batch_id gate", err)
	}
	for _, table := range []string{"order_refund_admissions", "order_refund_commitments"} {
		if got := countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE refund_id = 'R-TCC-NOB'`); got != 0 {
			t.Fatalf("empty batch_id wrote %s: %d", table, got)
		}
	}
}

// T7-R3/O1b: two TCC batches; one REVIEW parks in MANUAL_HOLD, the other
// continues independently (no cross-batch blocking).
func TestTCCReviewOtherBatchUnaffected(t *testing.T) {
	ctx := context.Background()
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	ff := &fakeFulfillmentPort{status: probeStatusNoWaybill}
	svc := tccTestServiceWithFulfillment(t, db, payments, ff)
	for _, order := range []struct{ no, pay string }{
		{"ORD-TCC-ISO-A", "PAY-TCC-ISO-A"},
		{"ORD-TCC-ISO-B", "PAY-TCC-ISO-B"},
	} {
		seedTccOrder(t, db, order.no, 10000, 10000, 0, 0, order.pay)
		seedTccGroup(t, db, order.no, "G1", 1, 0)
		seedTccLine(t, db, order.no, 1, "G1", 10000)
		seedTccPermit(t, db, order.no, "G1", permitConsumed, "SHIP-"+order.no)
	}
	// Batch A: REVIEW -> MANUAL_HOLD.
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-ISO-A", PaymentNo: "PAY-TCC-ISO-A", PaidCent: 10000,
	}); err == nil {
		t.Fatal("batch A: expected MANUAL_HOLD token")
	} else if got, _ := apperr.From(err).Details["tcc_error"].(string); got != tccErrBatchManual {
		t.Fatalf("batch A token = %q (err=%v details=%v)", got, err, apperr.From(err).Details)
	}
	// Batch B: real CANCELLED continues to admission/payment.
	ff.setStatus("CANCELLED")
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-ISO-B", PaymentNo: "PAY-TCC-ISO-B", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("batch B was blocked by batch A: %v", err)
	}
	batchA := loadTccBatchRow(t, db, "ORD-TCC-ISO-A", "PAY-TCC-ISO-A")
	batchB := loadTccBatchRow(t, db, "ORD-TCC-ISO-B", "PAY-TCC-ISO-B")
	if batchA.State != refundBatchManualHold {
		t.Fatalf("batch A = %s, want MANUAL_HOLD", batchA.State)
	}
	if batchB.State != refundBatchPaying && batchB.State != refundBatchDone {
		t.Fatalf("batch B = %s, want PAYING/DONE", batchB.State)
	}
	if got := len(payments.batchCalls); got != 1 {
		t.Fatalf("payment batches = %d, want 1 (batch A must pay nothing)", got)
	}
}
