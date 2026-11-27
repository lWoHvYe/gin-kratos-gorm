package order

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestAdmitRefundLineCompositionPersists(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	seedReleaseOrder(t, db, "ORD-LC", 10000, 10000, "PAY-LC", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
		{lineNo: 2, skuID: 2, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
	}, permitConsumed)
	seedReleaseGroup(t, db, "ORD-LC", "G1", 1, "SHIPPED", refundStateNone)

	reply, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-LC", OrderNo: "ORD-LC", GroupNo: "G1", AmountCent: 3000, Source: "aftersale",
		Lines: []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 3000}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPost {
		t.Fatalf("result = %q, want POST", reply.GetResult())
	}
	var goods, freight int64
	var known int
	var source string
	if err := db.QueryRow(`SELECT goods_cent, freight_cent, composition_known, composition_source
		FROM order_refund_commitments WHERE refund_id = 'R-LC'`).Scan(&goods, &freight, &known, &source); err != nil {
		t.Fatal(err)
	}
	if goods != 3000 || freight != 0 || known != 1 || source != "aftersale" {
		t.Fatalf("commitment composition = %d/%d/%d/%q", goods, freight, known, source)
	}
	var quantity, lineGoods, lineFreight int64
	if err := db.QueryRow(`SELECT quantity, goods_cent, freight_cent FROM order_refund_commitment_lines
		WHERE refund_id = 'R-LC' AND line_no = 1`).Scan(&quantity, &lineGoods, &lineFreight); err != nil {
		t.Fatal(err)
	}
	if quantity != 1 || lineGoods != 3000 || lineFreight != 0 {
		t.Fatalf("commitment line = %d/%d/%d", quantity, lineGoods, lineFreight)
	}
	// Same refund_id is idempotent: no duplicate admission/commitment/line.
	repeat, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-LC", OrderNo: "ORD-LC", GroupNo: "G1", AmountCent: 3000, Source: "aftersale",
		Lines: []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 3000}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repeat.GetResult() != refundResultPost {
		t.Fatalf("repeat result = %q, want POST", repeat.GetResult())
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id = 'R-LC'`); got != 1 {
		t.Fatalf("repeat admission rows = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitment_lines WHERE refund_id = 'R-LC'`); got != 1 {
		t.Fatalf("repeat commitment line rows = %d, want 1", got)
	}
}

func TestAdmitRefundLineOverLimitWritesNothing(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	seedReleaseOrder(t, db, "ORD-LC-OVER", 10000, 10000, "PAY-LC-OVER", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
		{lineNo: 2, skuID: 2, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
	}, permitConsumed)
	seedReleaseGroup(t, db, "ORD-LC-OVER", "G1", 1, "SHIPPED", refundStateNone)

	if _, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-LC-OVER", OrderNo: "ORD-LC-OVER", GroupNo: "G1", AmountCent: 6000, Source: "aftersale",
		Lines: []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 6000}},
	}); err == nil || !strings.Contains(err.Error(), "remaining line amount") {
		t.Fatalf("over-limit error = %v, want line-remaining failure", err)
	}
	assertNoAdmissionRows(t, db, "ORD-LC-OVER")
}

func TestAdmitRefundLineSumMismatchWritesNothing(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	seedReleaseOrder(t, db, "ORD-LC-SUM", 10000, 10000, "PAY-LC-SUM", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
	}, permitConsumed)
	seedReleaseGroup(t, db, "ORD-LC-SUM", "G1", 1, "SHIPPED", refundStateNone)

	if _, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-LC-SUM", OrderNo: "ORD-LC-SUM", GroupNo: "G1", AmountCent: 3000, Source: "aftersale",
		FreightRefundableCent: 500,
		Lines:                 []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 3000}},
	}); err == nil {
		t.Fatal("line sum mismatch was accepted")
	}
	assertNoAdmissionRows(t, db, "ORD-LC-SUM")
}

func TestAdmitRefundLegacyCompositionRejectsLines(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	seedReleaseOrder(t, db, "ORD-LC-LEGACY", 10000, 10000, "PAY-LC-LEGACY", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
	}, permitConsumed)
	seedReleaseGroup(t, db, "ORD-LC-LEGACY", "G1", 1, "SHIPPED", refundStateNone)
	if _, err := db.Exec(`INSERT INTO order_refund_commitments
		(order_no, group_no, refund_id, amount_cent, goods_cent, freight_cent, composition_known, state, version)
		VALUES ('ORD-LC-LEGACY','G1','R-LEGACY-PRIOR',1000,1000,0,0,'RESERVED',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-LC-LEGACY", OrderNo: "ORD-LC-LEGACY", GroupNo: "G1", AmountCent: 3000, Source: "aftersale",
		Lines: []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 3000}},
	}); err == nil {
		t.Fatal("line composition was accepted for a legacy composition_known=0 group")
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-LC-LEGACY'`); got != 0 {
		t.Fatalf("new admission rows = %d, want 0", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = 'R-LC-LEGACY'`); got != 0 {
		t.Fatalf("new commitment rows = %d, want 0", got)
	}
}

func TestAdmitRefundPositiveFreightWritesFreightLine(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	seedReleaseOrder(t, db, "ORD-LC-F", 5000, 5000, "PAY-LC-F", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
	}, permitConsumed)
	seedReleaseGroup(t, db, "ORD-LC-F", "G1", 1, "SHIPPED", refundStateNone)
	if _, err := db.Exec(`UPDATE order_fulfillment_groups SET freight_cent = 500, freight_share_cent = 500
		WHERE order_no = 'ORD-LC-F' AND group_no = 'G1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-LC-F", OrderNo: "ORD-LC-F", GroupNo: "G1", AmountCent: 3200, Source: "aftersale",
		FreightRefundableCent: 200,
		Lines:                 []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 3000}},
	}); err != nil {
		t.Fatal(err)
	}
	var goods, freight int64
	if err := db.QueryRow(`SELECT goods_cent, freight_cent FROM order_refund_commitments WHERE refund_id = 'R-LC-F'`).
		Scan(&goods, &freight); err != nil {
		t.Fatal(err)
	}
	if goods != 3000 || freight != 200 {
		t.Fatalf("header goods/freight = %d/%d, want 3000/200", goods, freight)
	}
	var freightLine int64
	if err := db.QueryRow(`SELECT freight_cent FROM order_refund_commitment_lines
		WHERE refund_id = 'R-LC-F' AND line_no = 0`).Scan(&freightLine); err != nil {
		t.Fatal(err)
	}
	if freightLine != 200 {
		t.Fatalf("freight line = %d, want 200", freightLine)
	}
}

func TestAdmitRefundLineRemainingIncludesSettled(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	seedReleaseOrder(t, db, "ORD-LC-SETTLED", 10000, 10000, "PAY-LC-SETTLED", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
		{lineNo: 2, skuID: 2, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
	}, permitConsumed)
	seedReleaseGroup(t, db, "ORD-LC-SETTLED", "G1", 1, "SHIPPED", refundStateNone)
	if _, err := db.Exec(`INSERT INTO order_refund_commitments
		(order_no, group_no, refund_id, amount_cent, goods_cent, freight_cent, composition_known, composition_source, state, version)
		VALUES ('ORD-LC-SETTLED','G1','R-LC-PRIOR',4000,4000,0,1,'aftersale','SETTLED',1)`); err != nil {
		t.Fatal(err)
	}
	// quantity 0 models a prior partial-amount refund that did not consume a
	// whole unit; the line still has 1000 cent of amount remaining.
	if _, err := db.Exec(`INSERT INTO order_refund_commitment_lines
		(refund_id, line_no, quantity, goods_cent, freight_cent) VALUES ('R-LC-PRIOR',1,0,4000,0)`); err != nil {
		t.Fatal(err)
	}
	// Group remaining is 6000 but line 1 remaining is 1000; only the line
	// layer can reject 4000 here.
	if _, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-LC-SETTLED-OVER", OrderNo: "ORD-LC-SETTLED", GroupNo: "G1", AmountCent: 4000, Source: "aftersale",
		Lines: []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 4000}},
	}); err == nil || !strings.Contains(err.Error(), "remaining line amount") {
		t.Fatalf("settled line error = %v, want line-remaining failure", err)
	}
	if _, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-LC-SETTLED-OK", OrderNo: "ORD-LC-SETTLED", GroupNo: "G1", AmountCent: 1000, Source: "aftersale",
		// quantity=0: amount-only partial that does not consume a unit.
		Lines: []*pb.RefundLine{{LineNo: 1, Quantity: 0, GoodsCent: 1000}},
	}); err != nil {
		t.Fatal(err)
	}
	var gotQuantity int64
	if err := db.QueryRow(`SELECT quantity FROM order_refund_commitment_lines
		WHERE refund_id = 'R-LC-SETTLED-OK' AND line_no = 1`).Scan(&gotQuantity); err != nil {
		t.Fatal(err)
	}
	if gotQuantity != 0 {
		t.Fatalf("amount-only line quantity = %d, want 0", gotQuantity)
	}
}

func assertNoAdmissionRows(t *testing.T, db *sql.DB, orderNo string) {
	t.Helper()
	for _, table := range []string{"order_refund_admissions", "order_refund_commitments"} {
		if got := countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE order_no = ?`, orderNo); got != 0 {
			t.Fatalf("%s rows = %d, want 0", table, got)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitment_lines l
		JOIN order_refund_commitments c ON c.refund_id = l.refund_id WHERE c.order_no = ?`, orderNo); got != 0 {
		t.Fatalf("commitment line rows = %d, want 0", got)
	}
}
