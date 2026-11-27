package order

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

// TestAdmitRefundConcurrentSameLine pins the order-row-lock serialization:
// two different refund_ids racing for the same line may create only one
// admission/commitment; the loser sees the reduced group/line quota.
func TestAdmitRefundConcurrentSameLine(t *testing.T) {
	db := orderTestDB(t)
	payments := &safeLegacyPayment{}
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	seedReleaseOrder(t, db, "ORD-CONC-LINE", 10000, 10000, "PAY-CONC-LINE", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 10000, groupNo: "G1"},
	}, permitConsumed)
	seedReleaseGroup(t, db, "ORD-CONC-LINE", "G1", 1, "SHIPPED", refundStateNone)

	requests := []*pb.AdmitRefundRequest{
		{RefundId: "R-CONC-LINE-A", OrderNo: "ORD-CONC-LINE", GroupNo: "G1", AmountCent: 6000, Source: "aftersale",
			Lines: []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 6000}}},
		{RefundId: "R-CONC-LINE-B", OrderNo: "ORD-CONC-LINE", GroupNo: "G1", AmountCent: 6000, Source: "aftersale",
			Lines: []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 6000}}},
	}
	errs := make([]error, len(requests))
	var wg sync.WaitGroup
	for i, req := range requests {
		wg.Add(1)
		go func(index int, request *pb.AdmitRefundRequest) {
			defer wg.Done()
			_, errs[index] = svc.AdmitRefund(context.Background(), request)
		}(i, req)
	}
	wg.Wait()
	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent successes = %d, errs = %v", successes, errs)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-CONC-LINE'`); got != 1 {
		t.Fatalf("concurrent admissions = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-CONC-LINE'`); got != 1 {
		t.Fatalf("concurrent commitments = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitment_lines l
		JOIN order_refund_commitments c ON c.refund_id = l.refund_id WHERE c.order_no = 'ORD-CONC-LINE'`); got != 1 {
		t.Fatalf("concurrent commitment lines = %d, want 1", got)
	}
}

// TestAdmitRefundLegacyCompositionAmountOnlyAllowed pins A-03: a group whose
// prior commitment is composition_known=0 may still take a whole-group
// amount-only refund; only line-level partials are forbidden.
func TestAdmitRefundLegacyCompositionAmountOnlyAllowed(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	seedReleaseOrder(t, db, "ORD-LEGACY-AMT", 5000, 5000, "PAY-LEGACY-AMT", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
	}, permitConsumed)
	seedReleaseGroup(t, db, "ORD-LEGACY-AMT", "G1", 1, "SHIPPED", refundStateNone)
	if _, err := db.Exec(`INSERT INTO order_refund_commitments
		(order_no, group_no, refund_id, amount_cent, goods_cent, freight_cent,
		 composition_known, composition_source, state, version)
		VALUES ('ORD-LEGACY-AMT','G1','R-LEGACY-PRIOR',1000,1000,0,0,'legacy','SETTLED',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-LEGACY-AMT", OrderNo: "ORD-LEGACY-AMT", GroupNo: "G1", AmountCent: 4000, Source: "aftersale",
	}); err != nil {
		t.Fatal(err)
	}
	var amount int64
	var known int
	if err := db.QueryRow(`SELECT amount_cent, composition_known FROM order_refund_commitments
		WHERE refund_id = 'R-LEGACY-AMT'`).Scan(&amount, &known); err != nil {
		t.Fatal(err)
	}
	if amount != 4000 || known != 0 {
		t.Fatalf("amount-only commitment = %d/%d, want 4000/0", amount, known)
	}
}

// TestAdmitRefundMixedLegacyCompositionRejectsLines pins the mixed group rule:
// even when another commitment has line composition, a known=0 row forces the
// group to whole-group-only and line requests fail closed.
func TestAdmitRefundMixedLegacyCompositionRejectsLines(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	seedReleaseOrder(t, db, "ORD-MIXED", 10000, 10000, "PAY-MIXED", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
		{lineNo: 2, skuID: 2, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
	}, permitConsumed)
	seedReleaseGroup(t, db, "ORD-MIXED", "G1", 1, "SHIPPED", refundStateNone)
	for _, row := range []struct {
		refundID string
		known    int
	}{
		{"R-MIXED-LEGACY", 0},
		{"R-MIXED-KNOWN", 1},
	} {
		if _, err := db.Exec(`INSERT INTO order_refund_commitments
			(order_no, group_no, refund_id, amount_cent, goods_cent, freight_cent,
			 composition_known, composition_source, state, version)
			VALUES ('ORD-MIXED','G1',?,1000,1000,0,?,'aftersale','RESERVED',1)`, row.refundID, row.known); err != nil {
			t.Fatal(err)
		}
		if row.known == 1 {
			if _, err := db.Exec(`INSERT INTO order_refund_commitment_lines
				(refund_id, line_no, quantity, goods_cent, freight_cent) VALUES (?,1,1,1000,0)`, row.refundID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-MIXED-NEW", OrderNo: "ORD-MIXED", GroupNo: "G1", AmountCent: 3000, Source: "aftersale",
		Lines: []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 3000}},
	}); err == nil {
		t.Fatal("line request was accepted for a mixed known=0+1 group")
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-MIXED'`); got != 0 {
		t.Fatalf("mixed group wrote %d admissions", got)
	}
}
