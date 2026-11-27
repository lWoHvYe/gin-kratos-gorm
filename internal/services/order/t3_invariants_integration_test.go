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

// TestRevokedPermitCannotBeRestored pins T3.1: no public refund/shipping path
// moves a REVOKED permit back to NONE/GRANTED. Admission still returns PRE and
// BlockShipmentForRefund is idempotent, but the permit remains REVOKED.
func TestRevokedPermitCannotBeRestored(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	seedReleaseOrder(t, db, "ORD-T31", 10000, 10000, "PAY-T31", []releaseTestItem{{
		lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 10000, groupNo: "G1",
	}}, permitRevoked)
	ctx := context.Background()

	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-T31", OrderNo: "ORD-T31", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPre {
		t.Fatalf("revoked permit admission = %q, want PRE", reply.GetResult())
	}
	assertPermitState(t, db, "ORD-T31", "G1", permitRevoked)

	if _, err := svc.BlockShipmentForRefund(ctx, &pb.ShipmentPermitRequest{
		OrderNo: "ORD-T31", AfterSaleNo: "AS-T31", FulfillmentGroupNo: "G1",
	}); err != nil {
		t.Fatal(err)
	}
	assertPermitState(t, db, "ORD-T31", "G1", permitRevoked)

	// The shipping path must fail closed on a revoked permit, never re-grant it.
	if _, err := svc.AuthorizeShipment(ctx, &pb.ShipmentPermitRequest{
		OrderNo: "ORD-T31", ShipmentNo: "SHIP-T31", FulfillmentGroupNo: "G1",
	}); err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("AuthorizeShipment error = %v, want revoked-permit failure", err)
	}
	assertPermitState(t, db, "ORD-T31", "G1", permitRevoked)
}

// TestRejectedAdmissionWritesNoCommitment pins T3.3: over-amount and partial
// unshipped rejections happen before Tx1, so no admission/commitment/outbox row
// is written and the permit stays NONE.
func TestRejectedAdmissionWritesNoCommitment(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	seedReleaseOrder(t, db, "ORD-T33", 10000, 10000, "PAY-T33", []releaseTestItem{{
		lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 10000, groupNo: "G1",
	}}, permitNone)
	ctx := context.Background()

	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-T33-OVER", OrderNo: "ORD-T33", GroupNo: "G1", AmountCent: 20000, Source: "aftersale",
	}); err == nil {
		t.Fatal("over-amount refund was accepted")
	}
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-T33-PART", OrderNo: "ORD-T33", GroupNo: "G1", AmountCent: 5000, Source: "aftersale",
	}); err == nil {
		t.Fatal("partial unshipped refund was accepted")
	}
	for _, table := range []string{"order_refund_admissions", "order_refund_commitments"} {
		if got := countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE order_no = 'ORD-T33'`); got != 0 {
			t.Fatalf("rejected admission wrote %d %s rows", got, table)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-T33'`); got != 0 {
		t.Fatalf("rejected admission wrote %d outbox events", got)
	}
	assertPermitState(t, db, "ORD-T33", "G1", permitNone)
}

func assertPermitState(t *testing.T, db *sql.DB, orderNo, groupNo, want string) {
	t.Helper()
	var got string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits
		WHERE order_no = ? AND fulfillment_group_no = ?`, orderNo, groupNo).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("permit %s/%s = %q, want %q", orderNo, groupNo, got, want)
	}
}
