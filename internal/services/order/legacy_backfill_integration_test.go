package order

import (
	"context"
	"sync"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestLegacyBackfillDryRunApplyIdempotent(t *testing.T) {
	db := legacyTestDB(t)
	payments := &fakePaymentPort{intents: []*pb.Intent{
		legacyIntent("PAY-BF-U1", "ORD-BF-U", "SUCCEEDED", 10000, 0),
		legacyIntent("PAY-BF-U2", "ORD-BF-U", "SUCCEEDED", 4000, 0),
		legacyIntent("PAY-BF-M1", "ORD-BF-M", "SUCCEEDED", 10000, 0),
		legacyIntent("PAY-BF-M2", "ORD-BF-M", "SUCCEEDED", 10000, 0),
		legacyIntent("PAY-BF-ZC1", "ORD-BF-ZC", "SUCCEEDED", 500, 0),
	}}
	svc := legacyService(db)
	svc.payments = payments
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-BF-U", 10000, 10000, 0, 0, "")
	seedTccOrder(t, db, "ORD-BF-M", 10000, 10000, 0, 0, "")
	seedTccOrder(t, db, "ORD-BF-Z", 0, 0, 0, 0, "")
	seedTccOrder(t, db, "ORD-BF-ZC", 0, 0, 0, 0, "")
	seedTccPendingOrder(t, db, "ORD-BF-P", 10000, 0, 0)

	dry, err := svc.BackfillLegacyPAID(ctx, LegacyBackfillOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if dry.Scanned != 4 || dry.Unique != 1 || dry.Zero != 1 || dry.Ambiguous != 2 {
		t.Fatalf("dry-run report = %+v", dry)
	}
	if dry.NoCaptured != 1 {
		t.Fatalf("dry-run no_captured = %d, want 1", dry.NoCaptured)
	}
	if len(dry.Samples) != 4 {
		t.Fatalf("dry-run samples = %d, want 4", len(dry.Samples))
	}
	for _, orderNo := range []string{"ORD-BF-U", "ORD-BF-M", "ORD-BF-Z", "ORD-BF-ZC"} {
		if got := orderSettlementNo(t, db, orderNo); got != "" {
			t.Fatalf("dry run wrote settlement for %s: %q", orderNo, got)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM legacy_payment_resolution`); got != 0 {
		t.Fatalf("dry run queued %d rows", got)
	}
	if len(payments.refundCalls) != 0 {
		t.Fatalf("dry run refunded: %+v", payments.refundCalls)
	}

	apply, err := svc.BackfillLegacyPAID(ctx, LegacyBackfillOptions{DryRun: false})
	if err != nil {
		t.Fatal(err)
	}
	if apply.Resolved != 1 || apply.Zero != 1 || apply.Queued != 2 || apply.Errors != 0 {
		t.Fatalf("apply report = %+v", apply)
	}
	if apply.Remaining != 2 {
		t.Fatalf("remaining after apply = %d, want 2 ambiguous", apply.Remaining)
	}
	if got := orderSettlementNo(t, db, "ORD-BF-U"); got != "PAY-BF-U1" {
		t.Fatalf("unique settlement = %q", got)
	}
	if got := orderSettlementNo(t, db, "ORD-BF-Z"); got != "zero:ORD-BF-Z" {
		t.Fatalf("zero settlement = %q", got)
	}
	for _, orderNo := range []string{"ORD-BF-M", "ORD-BF-ZC"} {
		if got := orderSettlementNo(t, db, orderNo); got != "" {
			t.Fatalf("ambiguous settlement for %s = %q", orderNo, got)
		}
	}
	if got := orderSettlementNo(t, db, "ORD-BF-P"); got != "" {
		t.Fatalf("non-PAID settlement = %q", got)
	}
	if got := orderStatus(t, db, "ORD-BF-P"); got != statusPendingPayment {
		t.Fatalf("non-PAID status = %q", got)
	}
	assertQueueRow(t, db, "ORD-BF-M", 2, 10000)
	assertQueueRow(t, db, "ORD-BF-ZC", 0, 0)
	if len(payments.refundCalls) != 1 || payments.refundCalls[0].GetPaymentNo() != "PAY-BF-U2" {
		t.Fatalf("duplicate refund calls = %+v", payments.refundCalls)
	}

	// Re-entry is idempotent: only the ambiguous rows remain and must not
	// create a second queue row or a second refund.
	again, err := svc.BackfillLegacyPAID(ctx, LegacyBackfillOptions{DryRun: false})
	if err != nil {
		t.Fatal(err)
	}
	if again.Scanned != 2 || again.Queued != 2 || again.Errors != 0 || again.Remaining != 2 {
		t.Fatalf("re-entry report = %+v", again)
	}
	if len(payments.refundCalls) != 1 {
		t.Fatalf("re-entry refunded again: %+v", payments.refundCalls)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM legacy_payment_resolution WHERE order_no = 'ORD-BF-M'`); got != 1 {
		t.Fatalf("re-entry queue rows = %d", got)
	}
}

// TestLegacyBackfillPartialDuplicateRefundFailure documents the fail-closed
// residual: the unique settlement CAS can commit before the duplicate-capture
// refund fails. The tool reports the error (exit 1); the order leaves the
// PAID-empty window, so a bulk re-run does not silently retry the refund.
func TestLegacyBackfillPartialDuplicateRefundFailure(t *testing.T) {
	db := legacyTestDB(t)
	payments := &fakePaymentPort{
		intents: []*pb.Intent{
			legacyIntent("PAY-BF-PF1", "ORD-BF-PF", "SUCCEEDED", 10000, 0),
			legacyIntent("PAY-BF-PF2", "ORD-BF-PF", "SUCCEEDED", 3000, 0),
		},
		refundErrs: []error{apperr.Unavailable("payment down", nil)},
	}
	svc := legacyService(db)
	svc.payments = payments
	seedTccOrder(t, db, "ORD-BF-PF", 10000, 10000, 0, 0, "")
	report, err := svc.BackfillLegacyPAID(context.Background(), LegacyBackfillOptions{DryRun: false})
	if err == nil || report.Errors != 1 {
		t.Fatalf("partial failure report = %+v err=%v", report, err)
	}
	if got := orderSettlementNo(t, db, "ORD-BF-PF"); got != "PAY-BF-PF1" {
		t.Fatalf("settlement after partial failure = %q", got)
	}
	if len(payments.refundCalls) != 1 {
		t.Fatalf("refund calls after partial failure = %d", len(payments.refundCalls))
	}
	again, err := svc.BackfillLegacyPAID(context.Background(), LegacyBackfillOptions{DryRun: false})
	if err != nil {
		t.Fatal(err)
	}
	if again.Scanned != 0 || len(payments.refundCalls) != 1 {
		t.Fatalf("bulk re-run retried settled partial failure: report=%+v calls=%d", again, len(payments.refundCalls))
	}
}

func TestLegacyBackfillConcurrentSameOrderOneRefund(t *testing.T) {
	db := legacyTestDB(t)
	payments := &safeLegacyPayment{intents: []*pb.Intent{
		legacyIntent("PAY-BF-C1", "ORD-BF-C", "SUCCEEDED", 10000, 0),
		legacyIntent("PAY-BF-C2", "ORD-BF-C", "SUCCEEDED", 3000, 0),
	}}
	svc := legacyService(db)
	svc.payments = payments
	seedTccOrder(t, db, "ORD-BF-C", 10000, 10000, 0, 0, "")

	const callers = 6
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, errs[index] = svc.BackfillLegacyPAID(context.Background(),
				LegacyBackfillOptions{DryRun: false, OrderNo: "ORD-BF-C"})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
	}
	if got := orderSettlementNo(t, db, "ORD-BF-C"); got != "PAY-BF-C1" {
		t.Fatalf("concurrent settlement = %q", got)
	}
	if got := payments.refundCount(); got != 1 {
		t.Fatalf("concurrent duplicate refunds = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM legacy_payment_resolution WHERE order_no = 'ORD-BF-C'`); got != 0 {
		t.Fatalf("concurrent queue rows = %d", got)
	}
}
