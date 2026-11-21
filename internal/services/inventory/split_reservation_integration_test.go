package inventory

import (
	"context"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestReservationAndOperationKeysIncludeWarehouse(t *testing.T) {
	db := seckillTestDB(t)
	if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version)
		VALUES (1, 1, 10, 0, 1), (1, 2, 10, 0, 1)`); err != nil {
		t.Fatal(err)
	}
	svc := New(db, nil)
	ctx := context.Background()
	orderNo := "ORD-SPLIT"
	if _, err := svc.Reserve(ctx, &pb.ReserveRequest{OrderNo: orderNo, SkuId: 1, WarehouseId: 1, Quantity: 2, TtlSeconds: 600}); err != nil {
		t.Fatalf("warehouse 1 reserve: %v", err)
	}
	if _, err := svc.Reserve(ctx, &pb.ReserveRequest{OrderNo: orderNo, SkuId: 1, WarehouseId: 2, Quantity: 3, TtlSeconds: 600}); err != nil {
		t.Fatalf("warehouse 2 reserve: %v", err)
	}
	// A replay for the same (sku, warehouse) must return the existing row, not
	// create a second reservation under the same order.
	if _, err := svc.Reserve(ctx, &pb.ReserveRequest{OrderNo: orderNo, SkuId: 1, WarehouseId: 1, Quantity: 2}); err != nil {
		t.Fatalf("warehouse 1 replay: %v", err)
	}
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM stock_reservations WHERE order_no = ?`, orderNo).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("reservations = %d, want 2 (one per warehouse)", rows)
	}
	items := []*pb.OrderItem{
		{SkuId: 1, WarehouseId: 1, Quantity: 2},
		{SkuId: 1, WarehouseId: 2, Quantity: 3},
	}
	if _, err := svc.TccConfirm(ctx, &pb.TccPaymentRequest{OrderNo: orderNo, Items: items}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	var opItems int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_operation_items WHERE warehouse_id IN (1,2)`).Scan(&opItems); err != nil {
		t.Fatal(err)
	}
	if opItems != 2 {
		t.Fatalf("operation items = %d", opItems)
	}
	var onHand int64
	if err := db.QueryRow(`SELECT on_hand FROM inventory_stocks WHERE sku_id=1 AND warehouse_id=1`).Scan(&onHand); err != nil {
		t.Fatal(err)
	}
	if onHand != 8 {
		t.Fatalf("warehouse 1 on_hand = %d", onHand)
	}
	if _, err := svc.TccCancelConfirm(ctx, &pb.TccPaymentRequest{OrderNo: orderNo, Items: items}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	for warehouseID, want := range map[int64]int64{1: 10, 2: 10} {
		if err := db.QueryRow(`SELECT on_hand FROM inventory_stocks WHERE sku_id=1 AND warehouse_id=?`, warehouseID).Scan(&onHand); err != nil {
			t.Fatal(err)
		}
		if onHand != want {
			t.Fatalf("warehouse %d on_hand = %d, want %d", warehouseID, onHand, want)
		}
	}
}
