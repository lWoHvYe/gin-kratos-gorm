package inventory

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func seedAllocationStock(t *testing.T, db *sql.DB, rows ...[3]int64) {
	t.Helper()
	if _, err := db.Exec(`DELETE FROM inventory_stocks`); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version)
			VALUES (?, ?, ?, 0, 1)`, row[0], row[1], row[2]); err != nil {
			t.Fatal(err)
		}
	}
}

// AC-01: one warehouse can satisfy the whole quantity, so no split happens.
func TestAllocateOrderSingleWarehouse(t *testing.T) {
	db := seckillTestDB(t)
	seedAllocationStock(t, db, [3]int64{1, 1, 5}, [3]int64{1, 2, 5})
	svc := NewWithOptions(db, nil, Options{QuantitySplitEnabled: true})
	reply, err := svc.AllocateOrder(context.Background(), &pb.AllocateOrderRequest{
		Lines: []*pb.AllocateOrderLine{{SkuId: 1, Quantity: 3}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.GetAllocations()) != 1 {
		t.Fatalf("allocations = %+v", reply.GetAllocations())
	}
	a := reply.GetAllocations()[0]
	if a.GetWarehouseId() != 1 || a.GetQuantity() != 3 || a.GetLineNo() != 1 {
		t.Fatalf("allocation = %+v", a)
	}
	var reservations int
	if err := db.QueryRow(`SELECT COUNT(*) FROM stock_reservations`).Scan(&reservations); err != nil {
		t.Fatal(err)
	}
	if reservations != 0 {
		t.Fatalf("preview wrote %d reservations", reservations)
	}
}

// AC-02 (revised after architect ruling): no single warehouse is enough, but
// two together are; the quantity is split into at most two rows.
func TestAllocateOrderQuantitySplit(t *testing.T) {
	db := seckillTestDB(t)
	seedAllocationStock(t, db, [3]int64{1, 1, 2}, [3]int64{1, 2, 2})
	svc := NewWithOptions(db, nil, Options{QuantitySplitEnabled: true})
	reply, err := svc.AllocateOrder(context.Background(), &pb.AllocateOrderRequest{
		Lines: []*pb.AllocateOrderLine{{SkuId: 1, Quantity: 3}},
	})
	if err != nil {
		t.Fatal(err)
	}
	allocations := reply.GetAllocations()
	if len(allocations) != 2 {
		t.Fatalf("allocations = %+v", allocations)
	}
	if allocations[0].GetWarehouseId() != 1 || allocations[0].GetQuantity() != 2 || allocations[0].GetSplitSeq() != 1 {
		t.Fatalf("first = %+v", allocations[0])
	}
	if allocations[1].GetWarehouseId() != 2 || allocations[1].GetQuantity() != 1 || allocations[1].GetSplitSeq() != 2 {
		t.Fatalf("second = %+v", allocations[1])
	}
	if allocations[0].GetLineNo() != 1 || allocations[1].GetLineNo() != 2 {
		t.Fatalf("line numbers = %d/%d", allocations[0].GetLineNo(), allocations[1].GetLineNo())
	}
}

// AC-06: the total is insufficient; the error carries the maximum orderable
// quantity and no order/preview rows are fabricated.
func TestAllocateOrderInsufficientTotal(t *testing.T) {
	db := seckillTestDB(t)
	seedAllocationStock(t, db, [3]int64{1, 1, 1}, [3]int64{1, 2, 1})
	svc := NewWithOptions(db, nil, Options{QuantitySplitEnabled: true})
	_, err := svc.AllocateOrder(context.Background(), &pb.AllocateOrderRequest{
		Lines: []*pb.AllocateOrderLine{{SkuId: 1, Quantity: 3}},
	})
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("err = %v", err)
	}
	ae := apperr.From(err)
	if ae.Details["max_orderable_quantity"] != int64(2) {
		t.Fatalf("details = %+v", ae.Details)
	}
}

// AC-07: three warehouses would be required for one SKU; the two-row rule
// rejects the whole preview instead of returning a 3-package plan.
func TestAllocateOrderRejectsThirdWarehouse(t *testing.T) {
	db := seckillTestDB(t)
	seedAllocationStock(t, db, [3]int64{1, 1, 1}, [3]int64{1, 2, 1}, [3]int64{1, 3, 1})
	svc := NewWithOptions(db, nil, Options{QuantitySplitEnabled: true})
	_, err := svc.AllocateOrder(context.Background(), &pb.AllocateOrderRequest{
		Lines: []*pb.AllocateOrderLine{{SkuId: 1, Quantity: 3}},
	})
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("err = %v", err)
	}
	ae := apperr.From(err)
	if ae.Details["min_packages_needed"] != 3 {
		t.Fatalf("details = %+v", ae.Details)
	}
}

func TestAllocateOrderRejectsMoreThanThreeGroups(t *testing.T) {
	db := seckillTestDB(t)
	seedAllocationStock(t, db,
		[3]int64{1, 1, 1}, [3]int64{2, 2, 1}, [3]int64{3, 3, 1}, [3]int64{4, 4, 1})
	svc := NewWithOptions(db, nil, Options{QuantitySplitEnabled: true})
	_, err := svc.AllocateOrder(context.Background(), &pb.AllocateOrderRequest{
		Lines: []*pb.AllocateOrderLine{
			{SkuId: 1, Quantity: 1}, {SkuId: 2, Quantity: 1}, {SkuId: 3, Quantity: 1}, {SkuId: 4, Quantity: 1},
		},
	})
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("err = %v", err)
	}
	ae := apperr.From(err)
	if ae.Details["suggestion"] != "split_orders" {
		t.Fatalf("details = %+v", ae.Details)
	}
}

func TestAllocateOrderIsDeterministic(t *testing.T) {
	db := seckillTestDB(t)
	seedAllocationStock(t, db, [3]int64{1, 2, 2}, [3]int64{1, 1, 2}, [3]int64{2, 3, 5})
	svc := NewWithOptions(db, nil, Options{QuantitySplitEnabled: true})
	req := &pb.AllocateOrderRequest{Lines: []*pb.AllocateOrderLine{
		{SkuId: 1, Quantity: 3}, {SkuId: 2, Quantity: 2},
	}}
	first, err := svc.AllocateOrder(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		next, err := svc.AllocateOrder(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first.GetAllocations(), next.GetAllocations()) {
			t.Fatalf("allocation changed: %+v -> %+v", first.GetAllocations(), next.GetAllocations())
		}
	}
}

func TestAllocateOrderQuantitySplitDisabledStaysSingleWarehouse(t *testing.T) {
	db := seckillTestDB(t)
	seedAllocationStock(t, db, [3]int64{1, 1, 2}, [3]int64{1, 2, 2})
	svc := NewWithOptions(db, nil, Options{QuantitySplitEnabled: false})
	_, err := svc.AllocateOrder(context.Background(), &pb.AllocateOrderRequest{
		Lines: []*pb.AllocateOrderLine{{SkuId: 1, Quantity: 3}},
	})
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("quantity split leaked while disabled: %v", err)
	}
}

func TestAllocateOrderUsesConfiguredWarehousePriority(t *testing.T) {
	db := seckillTestDB(t)
	seedAllocationStock(t, db, [3]int64{1, 1, 5}, [3]int64{1, 2, 5})
	svc := NewWithOptions(db, nil, Options{QuantitySplitEnabled: true, WarehousePriority: []int64{2, 1}})
	reply, err := svc.AllocateOrder(context.Background(), &pb.AllocateOrderRequest{
		Lines: []*pb.AllocateOrderLine{{SkuId: 1, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.GetAllocations()) != 1 || reply.GetAllocations()[0].GetWarehouseId() != 2 {
		t.Fatalf("allocations = %+v", reply.GetAllocations())
	}
	if len(reply.GetAvailability()) != 2 {
		t.Fatalf("availability = %+v", reply.GetAvailability())
	}
}

func TestAllocateOrderAvailabilityIncludesZeroStockWarehouse(t *testing.T) {
	db := seckillTestDB(t)
	if _, err := db.Exec(`DELETE FROM inventory_stocks`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version)
		VALUES (1, 1, 5, 5, 1), (1, 2, 5, 0, 1)`); err != nil {
		t.Fatal(err)
	}
	svc := NewWithOptions(db, nil, Options{QuantitySplitEnabled: true})
	reply, err := svc.AllocateOrder(context.Background(), &pb.AllocateOrderRequest{
		Lines: []*pb.AllocateOrderLine{{SkuId: 1, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.GetAvailability()) != 2 {
		t.Fatalf("availability = %+v", reply.GetAvailability())
	}
	var zero *pb.AllocateOrderAvailability
	for _, row := range reply.GetAvailability() {
		if row.GetWarehouseId() == 1 {
			zero = row
		}
	}
	if zero == nil || zero.GetAvailable() != 0 {
		t.Fatalf("zero-stock warehouse = %+v", zero)
	}
}
