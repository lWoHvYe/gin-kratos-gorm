package inventory

import (
	"context"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

type fakeConfirm struct {
	calls int
	order string
	items []*pb.OrderItem
}

func (f *fakeConfirm) TccConfirm(_ context.Context, in *pb.TccPaymentRequest) (*pb.Empty, error) {
	f.calls++
	f.order = in.GetOrderNo()
	f.items = in.GetItems()
	return &pb.Empty{}, nil
}

func TestConfirmHandlerUsesOrderScope(t *testing.T) {
	f := &fakeConfirm{}
	if err := handleConfirmRequest(context.Background(), f, []byte(`{"order_no":"ORD-C","items":[{"sku_id":1,"quantity":2},{"sku_id":0,"quantity":1}]}`)); err != nil {
		t.Fatal(err)
	}
	if f.calls != 1 || f.order != "ORD-C" || len(f.items) != 1 || f.items[0].GetQuantity() != 2 {
		t.Fatalf("calls=%d order=%s items=%+v", f.calls, f.order, f.items)
	}
}
