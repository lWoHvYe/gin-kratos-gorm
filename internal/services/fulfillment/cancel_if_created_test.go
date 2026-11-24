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
