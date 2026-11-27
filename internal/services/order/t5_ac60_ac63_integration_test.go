package order

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"google.golang.org/grpc"
)

func seedAC60Order(t *testing.T, db *sql.DB, orderNo string, pay int64, settle string) {
	t.Helper()
	seedTccOrder(t, db, orderNo, pay, pay, 0, 0, settle)
	seedReleaseGroup(t, db, orderNo, "G1", 1, "ALLOCATED", refundStateNone)
	seedReleasePermit(t, db, orderNo, "G1", permitNone)
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES (?,1,1,'x',1,1,'G1',?,?)`, orderNo, pay, pay); err != nil {
		t.Fatal(err)
	}
}

func TestAC60HistoricalPaidReadBackfillRefundShip(t *testing.T) {
	ctx := context.Background()
	t.Run("read and backfill unique capture", func(t *testing.T) {
		db := legacyTestDB(t)
		seedAC60Order(t, db, "ORD-AC60-BF", 10000, "")
		payments := &fakePaymentPort{intents: []*pb.Intent{
			legacyIntent("PAY-AC60-BF", "ORD-AC60-BF", "SUCCEEDED", 10000, 0),
		}}
		svc := legacyService(db)
		svc.payments = payments
		record, err := findOrder(ctx, db, "ORD-AC60-BF")
		if err != nil || record == nil || record.Status != statusPaid || record.SettlementPaymentNo != "" {
			t.Fatalf("historical read = %+v, %v", record, err)
		}
		report, err := svc.BackfillLegacyPAID(ctx, LegacyBackfillOptions{DryRun: false, OrderNo: "ORD-AC60-BF"})
		if err != nil || report.Resolved != 1 {
			t.Fatalf("backfill = %+v, %v", report, err)
		}
		if got := orderSettlementNo(t, db, "ORD-AC60-BF"); got != "PAY-AC60-BF" {
			t.Fatalf("settlement = %q", got)
		}
	})

	t.Run("refund with after-sale identity and idempotent retry", func(t *testing.T) {
		db := legacyTestDB(t)
		seedAC60Order(t, db, "ORD-AC60-RF", 10000, "")
		payments := &fakePaymentPort{captured: 10000, intents: []*pb.Intent{
			legacyIntent("PAY-AC60-RF", "ORD-AC60-RF", "SUCCEEDED", 10000, 0),
		}}
		svc := legacyService(db)
		svc.payments = payments
		req := &pb.AdmitRefundRequest{RefundId: "AS-AC60", OrderNo: "ORD-AC60-RF", GroupNo: "G1",
			AmountCent: 10000, Source: "aftersale"}
		first, err := svc.AdmitRefund(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if first.GetResult() != refundResultPre {
			t.Fatalf("refund result = %q, want PRE", first.GetResult())
		}
		again, err := svc.AdmitRefund(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if again.GetResult() != first.GetResult() {
			t.Fatalf("idempotent result = %q/%q", first.GetResult(), again.GetResult())
		}
		if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id = 'AS-AC60'`); got != 1 {
			t.Fatalf("admission rows = %d", got)
		}
	})

	t.Run("shipping after legacy resolution", func(t *testing.T) {
		db := legacyTestDB(t)
		seedAC60Order(t, db, "ORD-AC60-SH", 10000, "")
		svc := legacyService(db)
		svc.payments = &fakePaymentPort{intents: []*pb.Intent{
			legacyIntent("PAY-AC60-SH", "ORD-AC60-SH", "SUCCEEDED", 10000, 0),
		}}
		svc.inventory = &t16InventoryPort{db: db}
		if _, err := svc.AuthorizeShipment(ctx, &pb.ShipmentPermitRequest{
			OrderNo: "ORD-AC60-SH", ShipmentNo: "SHIP-AC60", FulfillmentGroupNo: "G1",
		}); err != nil {
			t.Fatal(err)
		}
		if got := orderSettlementNo(t, db, "ORD-AC60-SH"); got != "PAY-AC60-SH" {
			t.Fatalf("shipping settlement = %q", got)
		}
		assertPermitState(t, db, "ORD-AC60-SH", "G1", permitConsumed)
	})
}

func TestAC63LegacyResolutionAndQueue(t *testing.T) {
	ctx := context.Background()
	t.Run("unique capture resolves", func(t *testing.T) {
		db := legacyTestDB(t)
		seedTccOrder(t, db, "ORD-AC63-U", 10000, 10000, 0, 0, "")
		svc := legacyService(db)
		svc.payments = &fakePaymentPort{intents: []*pb.Intent{
			legacyIntent("PAY-AC63-U", "ORD-AC63-U", "SUCCEEDED", 10000, 0),
		}}
		res, err := svc.ensureLegacyResolution(ctx, "ORD-AC63-U")
		if err != nil || res.State != legacyResolutionStateResolved || res.PaymentNo != "PAY-AC63-U" {
			t.Fatalf("unique resolve = %+v, %v", res, err)
		}
	})

	t.Run("ambiguous capture queues", func(t *testing.T) {
		db := legacyTestDB(t)
		seedTccOrder(t, db, "ORD-AC63-A", 10000, 10000, 0, 0, "")
		svc := legacyService(db)
		svc.payments = &fakePaymentPort{intents: []*pb.Intent{
			legacyIntent("PAY-AC63-A1", "ORD-AC63-A", "SUCCEEDED", 10000, 0),
			legacyIntent("PAY-AC63-A2", "ORD-AC63-A", "SUCCEEDED", 10000, 0),
		}}
		res, err := svc.ensureLegacyResolution(ctx, "ORD-AC63-A")
		if err != nil || res.State != legacyResolutionStateOpen || res.CandidateCount != 2 {
			t.Fatalf("ambiguous resolve = %+v, %v", res, err)
		}
		if got := orderSettlementNo(t, db, "ORD-AC63-A"); got != "" {
			t.Fatalf("ambiguous wrote settlement %q", got)
		}
	})

	t.Run("no capture zero expires to EXPIRED_ZERO", func(t *testing.T) {
		db := legacyTestDB(t)
		seedTccOrder(t, db, "ORD-AC63-Z", 0, 0, 0, 0, "")
		svc := legacyService(db)
		svc.payments = &fakePaymentPort{}
		res, err := svc.ensureLegacyResolution(ctx, "ORD-AC63-Z")
		if err != nil || res.State != legacyResolutionStateExpiredZero {
			t.Fatalf("zero resolve = %+v, %v", res, err)
		}
	})

	t.Run("captured unshipped enters COMPENSATING and does not release", func(t *testing.T) {
		db := legacyTestDB(t)
		seedTccOrder(t, db, "ORD-AC63-C", 10000, 10000, 0, 0, "")
		seedTccGroup(t, db, "ORD-AC63-C", "G1", 1, 0)
		seedTccPermit(t, db, "ORD-AC63-C", "G1", permitNone, "")
		seedLegacyQueue(t, db, "ORD-AC63-C", legacyResolutionStateOpen)
		svc := legacyService(db)
		svc.payments = &fakePaymentPort{intents: []*pb.Intent{
			legacyIntent("PAY-AC63-C", "ORD-AC63-C", "SUCCEEDED", 10000, 0),
		}}
		svc.fulfillment = &fakeFulfillmentPort{status: probeStatusNoWaybill}
		_ = svc.processExpiredLegacyResolution(ctx, "ORD-AC63-C")
		state, _, _, _ := legacyQueueState(t, db, "ORD-AC63-C")
		if state != legacyResolutionStateCompensating {
			t.Fatalf("compensation state = %s", state)
		}
		if got := countRows(t, db, `SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = 'ORD-AC63-C' AND state = 'REVOKED'`); got != 1 {
			t.Fatalf("permit revocation = %d", got)
		}
		if len(svc.payments.(*fakePaymentPort).refundCalls) != 0 {
			t.Fatal("compensation called Payment before release")
		}
	})
}

func TestAC60LegacyRefundAllocationIdempotent(t *testing.T) {
	db := legacyTestDB(t)
	seedAC60Order(t, db, "ORD-AC60-ALLOC", 10000, "")
	payments := &fakePaymentPort{captured: 10000, intents: []*pb.Intent{
		legacyIntent("PAY-AC60-ALLOC", "ORD-AC60-ALLOC", "SUCCEEDED", 10000, 0),
	}}
	svc := legacyService(db)
	svc.payments = payments
	ctx := context.Background()
	req := &pb.AdmitRefundRequest{
		OrderNo: "ORD-AC60-ALLOC", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
		ClientRequestId: "REQ-AC60-1",
	}
	if _, err := svc.AdmitRefund(ctx, req); err != nil {
		t.Fatal(err)
	}
	var refundID string
	if err := db.QueryRow(`SELECT refund_id FROM order_refund_admissions WHERE order_no = 'ORD-AC60-ALLOC'`).Scan(&refundID); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(refundID, "legacy:ORD-AC60-ALLOC:") {
		t.Fatalf("allocated refund_id = %q", refundID)
	}
	if _, err := svc.AdmitRefund(ctx, req); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-AC60-ALLOC'`); got != 1 {
		t.Fatalf("admissions = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM legacy_refund_allocations WHERE order_no = 'ORD-AC60-ALLOC'`); got != 1 {
		t.Fatalf("allocations = %d, want 1", got)
	}
	var again string
	if err := db.QueryRow(`SELECT refund_id FROM legacy_refund_allocations WHERE order_no = 'ORD-AC60-ALLOC' AND request_id = 'REQ-AC60-1'`).Scan(&again); err != nil {
		t.Fatal(err)
	}
	if again != refundID {
		t.Fatalf("retry allocated %q, want %q", again, refundID)
	}
}

type ac60Inventory struct {
	pb.InventoryServiceClient
	lines        []*pb.CheckOrderReadyLineStatus
	confirmCalls []*pb.ReserveRequest
}

func (f *ac60Inventory) CheckOrderReady(context.Context, *pb.CheckOrderReadyRequest, ...grpc.CallOption) (*pb.CheckOrderReadyReply, error) {
	return &pb.CheckOrderReadyReply{Ready: false, Lines: f.lines}, nil
}

func (f *ac60Inventory) Confirm(_ context.Context, in *pb.ReserveRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	f.confirmCalls = append(f.confirmCalls, in)
	return &pb.Empty{}, nil
}

func TestAC60ShippingConfirmsHeldAndBlocksMissing(t *testing.T) {
	ctx := context.Background()
	t.Run("HELD is confirmed before the permit", func(t *testing.T) {
		db := legacyTestDB(t)
		seedAC60Order(t, db, "ORD-AC60-HELD", 10000, "")
		svc := legacyService(db)
		svc.payments = &fakePaymentPort{intents: []*pb.Intent{
			legacyIntent("PAY-AC60-HELD", "ORD-AC60-HELD", "SUCCEEDED", 10000, 0),
		}}
		inventory := &ac60Inventory{lines: []*pb.CheckOrderReadyLineStatus{{
			LineNo: 1, SkuId: 1, WarehouseId: 1, Quantity: 1, Status: "HELD",
		}}}
		svc.inventory = inventory
		if _, err := svc.AuthorizeShipment(ctx, &pb.ShipmentPermitRequest{
			OrderNo: "ORD-AC60-HELD", ShipmentNo: "SHIP-AC60-HELD", FulfillmentGroupNo: "G1",
		}); err != nil {
			t.Fatal(err)
		}
		if len(inventory.confirmCalls) != 1 || inventory.confirmCalls[0].GetOrderNo() != "ORD-AC60-HELD" {
			t.Fatalf("confirm calls = %+v", inventory.confirmCalls)
		}
		assertPermitState(t, db, "ORD-AC60-HELD", "G1", permitConsumed)
	})

	t.Run("MISSING fails closed and keeps the permit", func(t *testing.T) {
		db := legacyTestDB(t)
		seedAC60Order(t, db, "ORD-AC60-MISS", 10000, "")
		svc := legacyService(db)
		svc.payments = &fakePaymentPort{intents: []*pb.Intent{
			legacyIntent("PAY-AC60-MISS", "ORD-AC60-MISS", "SUCCEEDED", 10000, 0),
		}}
		inventory := &ac60Inventory{lines: []*pb.CheckOrderReadyLineStatus{{
			LineNo: 1, SkuId: 1, WarehouseId: 1, Quantity: 1, Status: "MISSING",
		}}}
		svc.inventory = inventory
		if _, err := svc.AuthorizeShipment(ctx, &pb.ShipmentPermitRequest{
			OrderNo: "ORD-AC60-MISS", ShipmentNo: "SHIP-AC60-MISS", FulfillmentGroupNo: "G1",
		}); err == nil {
			t.Fatal("missing reservation was accepted")
		}
		if len(inventory.confirmCalls) != 0 {
			t.Fatalf("missing confirm calls = %d", len(inventory.confirmCalls))
		}
		assertPermitState(t, db, "ORD-AC60-MISS", "G1", permitNone)
	})
}
