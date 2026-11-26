package order

import (
	"context"
	"database/sql"
	"errors"
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
