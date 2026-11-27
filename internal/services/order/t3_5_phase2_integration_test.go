package order

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestAdmitRefundResponsibilityValidation(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()

	seed := func(orderNo string, freight int64) {
		seedReleaseOrder(t, db, orderNo, 5000, 5000, "PAY-"+orderNo, []releaseTestItem{
			{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
		}, permitConsumed)
		seedReleaseGroup(t, db, orderNo, "G1", 1, "SHIPPED", refundStateNone)
		if freight > 0 {
			if _, err := db.Exec(`UPDATE order_fulfillment_groups SET freight_cent = ?, freight_share_cent = ?
				WHERE order_no = ? AND group_no = 'G1'`, freight, freight, orderNo); err != nil {
				t.Fatal(err)
			}
		}
	}
	request := func(refundID, orderNo, responsibility, reason string, freight, amount, goods int64) *pb.AdmitRefundRequest {
		return &pb.AdmitRefundRequest{
			RefundId: refundID, OrderNo: orderNo, GroupNo: "G1", AmountCent: amount, Source: "aftersale",
			Responsibility: responsibility, ReasonCode: reason, FreightRefundableCent: freight,
			Lines: []*pb.RefundLine{{LineNo: 1, Quantity: 0, GoodsCent: goods}},
		}
	}

	seed("ORD-RP-BAD-PARTY", 0)
	if _, err := svc.AdmitRefund(ctx, request("R-RP-BAD-PARTY", "ORD-RP-BAD-PARTY", "MERCHANT", "OUT_OF_STOCK", 0, 1000, 1000)); err == nil || !strings.Contains(err.Error(), "closed enum") {
		t.Fatalf("unknown responsibility error = %v", err)
	}
	assertNoAdmissionRows(t, db, "ORD-RP-BAD-PARTY")

	seed("ORD-RP-BAD-REASON", 0)
	if _, err := svc.AdmitRefund(ctx, request("R-RP-BAD-REASON", "ORD-RP-BAD-REASON", "MERCHANT_WAREHOUSE", "LOST_IN_TRANSIT", 0, 1000, 1000)); err == nil || !strings.Contains(err.Error(), "reason_code does not belong") {
		t.Fatalf("cross-party reason error = %v", err)
	}
	assertNoAdmissionRows(t, db, "ORD-RP-BAD-REASON")

	seed("ORD-RP-ORPHAN", 0)
	if _, err := svc.AdmitRefund(ctx, request("R-RP-ORPHAN", "ORD-RP-ORPHAN", "", "OUT_OF_STOCK", 0, 1000, 1000)); err == nil || !strings.Contains(err.Error(), "reason_code requires responsibility") {
		t.Fatalf("orphan reason error = %v", err)
	}
	assertNoAdmissionRows(t, db, "ORD-RP-ORPHAN")

	seed("ORD-RP-UND-HOLD", 500)
	if _, err := svc.AdmitRefund(ctx, request("R-RP-UND-HOLD", "ORD-RP-UND-HOLD", "UNDETERMINED", "PENDING_REVIEW", 200, 1200, 1000)); err == nil || !strings.Contains(err.Error(), "hold freight") {
		t.Fatalf("UNDETERMINED freight error = %v", err)
	}
	assertNoAdmissionRows(t, db, "ORD-RP-UND-HOLD")

	seed("ORD-RP-UND-OK", 500)
	if _, err := svc.AdmitRefund(ctx, request("R-RP-UND-OK", "ORD-RP-UND-OK", "UNDETERMINED", "PENDING_REVIEW", 0, 1000, 1000)); err != nil {
		t.Fatal(err)
	}

	seed("ORD-RP-MERCHANT", 500)
	if _, err := svc.AdmitRefund(ctx, request("R-RP-MERCHANT", "ORD-RP-MERCHANT", "MERCHANT_WAREHOUSE", "OUT_OF_STOCK", 200, 1200, 1000)); err != nil {
		t.Fatal(err)
	}
	var freight int64
	if err := db.QueryRow(`SELECT freight_cent FROM order_refund_commitments WHERE refund_id = 'R-RP-MERCHANT'`).Scan(&freight); err != nil {
		t.Fatal(err)
	}
	if freight != 200 {
		t.Fatalf("merchant freight = %d, want 200", freight)
	}
}

func TestAdmitRefundQuantityOverLimit(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	seedReleaseOrder(t, db, "ORD-RP-QTY", 5000, 5000, "PAY-RP-QTY", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
	}, permitConsumed)
	seedReleaseGroup(t, db, "ORD-RP-QTY", "G1", 1, "SHIPPED", refundStateNone)
	if _, err := db.Exec(`INSERT INTO order_refund_commitments
		(order_no, group_no, refund_id, amount_cent, goods_cent, freight_cent, composition_known, composition_source, state, version)
		VALUES ('ORD-RP-QTY','G1','R-RP-QTY-PRIOR',1000,1000,0,1,'aftersale','SETTLED',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO order_refund_commitment_lines
		(refund_id, line_no, quantity, goods_cent, freight_cent) VALUES ('R-RP-QTY-PRIOR',1,1,1000,0)`); err != nil {
		t.Fatal(err)
	}
	// The line still has 4000 cent of amount, but its only unit is already
	// committed; quantity>0 must reject even though goods would fit.
	if _, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-RP-QTY", OrderNo: "ORD-RP-QTY", GroupNo: "G1", AmountCent: 1000, Source: "aftersale",
		Lines: []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 1000}},
	}); err == nil || !strings.Contains(err.Error(), "remaining line amount") {
		t.Fatalf("quantity over-limit error = %v", err)
	}
}
