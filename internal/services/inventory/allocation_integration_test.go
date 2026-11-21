package inventory

import (
	"context"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestAllocateOrderIsReadOnlyAndLineScoped(t *testing.T) {
	db := seckillTestDB(t)
	if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version)
		VALUES (1, 1, 1, 0, 1), (1, 2, 5, 0, 1)`); err != nil {
		t.Fatal(err)
	}
	svc := New(db, nil)
	ctx := context.Background()
	reply, err := svc.AllocateOrder(ctx, &pb.AllocateOrderRequest{Lines: []*pb.AllocateOrderLine{
		{SkuId: 1, Quantity: 1},
		{SkuId: 1, Quantity: 3},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.GetAllocations()) != 2 {
		t.Fatalf("allocations = %+v", reply.GetAllocations())
	}
	if a := reply.GetAllocations()[0]; a.GetWarehouseId() != 1 || a.GetLineNo() != 1 || a.GetQuantity() != 1 {
		t.Fatalf("first allocation = %+v", a)
	}
	if a := reply.GetAllocations()[1]; a.GetWarehouseId() != 2 || a.GetLineNo() != 2 || a.GetQuantity() != 3 {
		t.Fatalf("second allocation = %+v", a)
	}
	var reservations, ledger int
	if err := db.QueryRow(`SELECT COUNT(*) FROM stock_reservations`).Scan(&reservations); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_ledger`).Scan(&ledger); err != nil {
		t.Fatal(err)
	}
	if reservations != 0 || ledger != 0 {
		t.Fatalf("allocator wrote reservations=%d ledger=%d", reservations, ledger)
	}
	if _, err := svc.AllocateOrder(ctx, &pb.AllocateOrderRequest{Lines: []*pb.AllocateOrderLine{{SkuId: 1, Quantity: 10}}}); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("insufficient allocation err = %v", err)
	}
	if _, err := svc.AllocateOrder(ctx, &pb.AllocateOrderRequest{}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("empty allocation err = %v", err)
	}
}
