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
