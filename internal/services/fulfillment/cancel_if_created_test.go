package fulfillment

import (
	"context"
	"io"
	"log/slog"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestCancelIfCreatedOnlyCancelsCreated(t *testing.T) {
	db := fulfillmentTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	ctx := context.Background()

	if _, err := db.Exec(`INSERT INTO shipments (shipment_no, order_no, fulfillment_group_no, status)
		VALUES ('SHIP-C1', 'ORD-C1', 'G1', 'CREATED')`); err != nil {
		t.Fatal(err)
	}
	reply, err := svc.CancelIfCreated(ctx, &pb.CancelIfCreatedRequest{
		OrderNo: "ORD-C1", GroupNo: "G1", ShipmentNo: "SHIP-C1", RefundId: "R-C1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetStatus() != statusCancelled {
		t.Fatalf("status = %q, want %q", reply.GetStatus(), statusCancelled)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM shipments WHERE shipment_no = 'SHIP-C1'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusCancelled {
		t.Fatalf("db status = %q, want %q", status, statusCancelled)
	}

	// Idempotent repeat.
	if reply, err = svc.CancelIfCreated(ctx, &pb.CancelIfCreatedRequest{
		OrderNo: "ORD-C1", GroupNo: "G1", ShipmentNo: "SHIP-C1", RefundId: "R-C1",
	}); err != nil || reply.GetStatus() != statusCancelled {
		t.Fatalf("repeat = (%v, %v), want CANCELLED", reply, err)
	}

	if _, err := db.Exec(`INSERT INTO shipments (shipment_no, order_no, fulfillment_group_no, status)
		VALUES ('SHIP-C2', 'ORD-C2', 'G1', 'SHIPPED')`); err != nil {
		t.Fatal(err)
	}
	reply, err = svc.CancelIfCreated(ctx, &pb.CancelIfCreatedRequest{
		OrderNo: "ORD-C2", GroupNo: "G1", ShipmentNo: "SHIP-C2", RefundId: "R-C2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetStatus() != "ALREADY_SHIPPED" {
		t.Fatalf("shipped status = %q, want ALREADY_SHIPPED", reply.GetStatus())
	}
}

// R-2 (ADR-0007 Amd1 Addendum 2 §3.2.5): a carrier exception after pickup
// (`exception_state != NONE` and `shipped_at`) is an already-left-warehouse
// fact. It must answer ALREADY_SHIPPED and never cancel or roll back the
// shipment. An exception without `shipped_at` is ambiguous: no cancel and a
// TRANSIENT_ERROR for retry/manual reconciliation.
func TestCancelIfCreatedRefusesExceptionAfterShipment(t *testing.T) {
	db := fulfillmentTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	ctx := context.Background()

	if _, err := db.Exec(`INSERT INTO shipments
		(shipment_no, order_no, fulfillment_group_no, status, exception_state, shipped_at)
		VALUES ('SHIP-E1', 'ORD-E1', 'G1', 'CREATED', 'OPEN', NOW(3))`); err != nil {
		t.Fatal(err)
	}
	reply, err := svc.CancelIfCreated(ctx, &pb.CancelIfCreatedRequest{
		OrderNo: "ORD-E1", GroupNo: "G1", RefundId: "R-E1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetStatus() != "ALREADY_SHIPPED" {
		t.Fatalf("exception+shipped status = %q, want ALREADY_SHIPPED", reply.GetStatus())
	}
	var status, exception string
	if err := db.QueryRow(`SELECT status, exception_state FROM shipments WHERE shipment_no = 'SHIP-E1'`).
		Scan(&status, &exception); err != nil {
		t.Fatal(err)
	}
	if status != statusCreated || exception != "OPEN" {
		t.Fatalf("exception+shipped shipment mutated to %s/%s", status, exception)
	}

	if _, err := db.Exec(`INSERT INTO shipments
		(shipment_no, order_no, fulfillment_group_no, status, exception_state, shipped_at)
		VALUES ('SHIP-E2', 'ORD-E2', 'G1', 'CREATED', 'OPEN', NULL)`); err != nil {
		t.Fatal(err)
	}
	reply, err = svc.CancelIfCreated(ctx, &pb.CancelIfCreatedRequest{
		OrderNo: "ORD-E2", GroupNo: "G1", RefundId: "R-E2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetStatus() != "TRANSIENT_ERROR" {
		t.Fatalf("exception without shipped_at status = %q, want TRANSIENT_ERROR", reply.GetStatus())
	}
	if err := db.QueryRow(`SELECT status FROM shipments WHERE shipment_no = 'SHIP-E2'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusCreated {
		t.Fatalf("ambiguous exception cancelled the shipment: %s", status)
	}

	if _, err := db.Exec(`INSERT INTO shipments
		(shipment_no, order_no, fulfillment_group_no, status, exception_state)
		VALUES ('SHIP-E3', 'ORD-E3', 'G1', 'RECEIVED', 'NONE')`); err != nil {
		t.Fatal(err)
	}
	reply, err = svc.CancelIfCreated(ctx, &pb.CancelIfCreatedRequest{
		OrderNo: "ORD-E3", GroupNo: "G1", RefundId: "R-E3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetStatus() != "RECEIVED" {
		t.Fatalf("received status = %q, want RECEIVED", reply.GetStatus())
	}
}

// AC-50 Addendum 0.6 R-1 (§2.1/§2.2): "no shipment row in this probe scope" is
// a distinct answer, not CANCELLED. A real CANCELLED row keeps answering
// CANCELLED, and the probe must never create a row for the missing case.
func TestCancelIfCreatedNoWaybill(t *testing.T) {
	db := fulfillmentTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	ctx := context.Background()

	// F6: G2 has a real CREATED row; the G1 probe must not touch it.
	if _, err := db.Exec(`INSERT INTO shipments (shipment_no, order_no, fulfillment_group_no, status)
		VALUES ('SHIP-N1', 'ORD-N1', 'G2', 'CREATED')`); err != nil {
		t.Fatal(err)
	}

	// F1: no row in the whole-group scope.
	reply, err := svc.CancelIfCreated(ctx, &pb.CancelIfCreatedRequest{
		OrderNo: "ORD-N1", GroupNo: "G1", RefundId: "R-N1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetStatus() != statusNoWaybill {
		t.Fatalf("no-row status = %q, want %q", reply.GetStatus(), statusNoWaybill)
	}
	var g1 int
	if err := db.QueryRow(`SELECT COUNT(*) FROM shipments WHERE order_no = 'ORD-N1' AND fulfillment_group_no = 'G1'`).Scan(&g1); err != nil {
		t.Fatal(err)
	}
	if g1 != 0 {
		t.Fatalf("no-row probe created %d shipment rows", g1)
	}
	var g2 string
	if err := db.QueryRow(`SELECT status FROM shipments WHERE shipment_no = 'SHIP-N1'`).Scan(&g2); err != nil {
		t.Fatal(err)
	}
	if g2 != statusCreated {
		t.Fatalf("G1 probe mutated G2: %q", g2)
	}

	// F1b: shipment_no narrows the scope; a missing row is still NO_WAYBILL and
	// must not cancel the sibling row of the same group.
	reply, err = svc.CancelIfCreated(ctx, &pb.CancelIfCreatedRequest{
		OrderNo: "ORD-N1", GroupNo: "G2", ShipmentNo: "SHIP-MISSING", RefundId: "R-N1b",
	})
	if err != nil || reply.GetStatus() != statusNoWaybill {
		t.Fatalf("missing shipment_no = (%v, %v), want NO_WAYBILL", reply, err)
	}
	if err := db.QueryRow(`SELECT status FROM shipments WHERE shipment_no = 'SHIP-N1'`).Scan(&g2); err != nil {
		t.Fatal(err)
	}
	if g2 != statusCreated {
		t.Fatalf("narrow-scope probe mutated the sibling row: %q", g2)
	}

	// F2: the same row probed exactly is still CANCELLED (real fact).
	reply, err = svc.CancelIfCreated(ctx, &pb.CancelIfCreatedRequest{
		OrderNo: "ORD-N1", GroupNo: "G2", RefundId: "R-N1c",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetStatus() != statusCancelled {
		t.Fatalf("created row status = %q, want %q", reply.GetStatus(), statusCancelled)
	}
	if err := db.QueryRow(`SELECT status FROM shipments WHERE shipment_no = 'SHIP-N1'`).Scan(&g2); err != nil {
		t.Fatal(err)
	}
	if g2 != statusCancelled {
		t.Fatalf("created row not cancelled: %q", g2)
	}
}
