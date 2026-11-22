package inventory

import (
	"context"
	"sync"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"google.golang.org/grpc"
)

type fakeRestockAuth struct {
	pb.AfterSalesServiceClient
	valid bool
	calls int
}

func (f *fakeRestockAuth) VerifyRestockAuthorization(context.Context, *pb.VerifyRestockAuthorizationRequest, ...grpc.CallOption) (*pb.VerifyRestockAuthorizationReply, error) {
	f.calls++
	return &pb.VerifyRestockAuthorizationReply{Valid: f.valid}, nil
}

// AC-16: cross-warehouse restock fails closed without a matching authorization
// and succeeds only after AfterSales verifies it.
func TestAC16CrossWarehouseRestockRequiresAuthorization(t *testing.T) {
	db := seckillTestDB(t)
	if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version)
		VALUES (1, 1, 1, 0, 1), (1, 2, 0, 0, 1)`); err != nil {
		t.Fatal(err)
	}
	svc := New(db, nil)
	ctx := context.Background()
	req := &pb.RestockRequest{
		OrderNo: "ORD-RST", SkuId: 1, WarehouseId: 2, Quantity: 1,
		RestockNo: "ORD-RST", OriginWarehouseId: 1,
	}
	if _, err := svc.Restock(ctx, req); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("cross-warehouse restock without auth = %v", err)
	}
	auth := &fakeRestockAuth{valid: false}
	svc.SetAfterSalesClient(auth)
	req.RestockAuthNo = "RST:ORD-RST:1"
	if _, err := svc.Restock(ctx, req); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("rejected auth = %v", err)
	}
	auth.valid = true
	if _, err := svc.Restock(ctx, req); err != nil {
		t.Fatal(err)
	}
	var onHand int64
	if err := db.QueryRow(`SELECT on_hand FROM inventory_stocks WHERE sku_id = 1 AND warehouse_id = 2`).Scan(&onHand); err != nil {
		t.Fatal(err)
	}
	if onHand != 1 || auth.calls != 2 {
		t.Fatalf("on_hand=%d calls=%d", onHand, auth.calls)
	}
}

// AC-08: two concurrent buyers of the last unit; exactly one reservation wins
// and stock never goes negative.
func TestAC08ConcurrentReserveLastItem(t *testing.T) {
	db := seckillTestDB(t)
	if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version)
		VALUES (1, 1, 1, 0, 1)`); err != nil {
		t.Fatal(err)
	}
	svc := New(db, nil)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, errs[index] = svc.Reserve(ctx, &pb.ReserveRequest{
				OrderNo: []string{"ORD-A", "ORD-B"}[index], SkuId: 1, WarehouseId: 1, Quantity: 1,
			})
		}(i)
	}
	wg.Wait()
	success := 0
	for _, err := range errs {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successes = %d, errors=%v", success, errs)
	}
	var onHand, reserved int64
	if err := db.QueryRow(`SELECT on_hand, reserved FROM inventory_stocks WHERE sku_id = 1 AND warehouse_id = 1`).
		Scan(&onHand, &reserved); err != nil {
		t.Fatal(err)
	}
	if onHand != 1 || reserved != 1 {
		t.Fatalf("stock = on_hand %d reserved %d", onHand, reserved)
	}
}
