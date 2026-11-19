package inventory

import (
	"context"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/consumer"
)

type fakeCompensation struct {
	calls  int
	failed bool
	order  string
	items  []*pb.OrderItem
}

func (f *fakeCompensation) TccCancelConfirm(_ context.Context, in *pb.TccPaymentRequest) (*pb.Empty, error) {
	f.calls++
	f.order = in.GetOrderNo()
	f.items = in.GetItems()
	if f.failed {
		return nil, context.DeadlineExceeded
	}
	return &pb.Empty{}, nil
}

func TestCompensationHandlerRetriesAndSkipsInvalidItems(t *testing.T) {
	fake := &fakeCompensation{failed: true}
	payload := []byte(`{"order_no":"ORD-1","items":[{"sku_id":1,"quantity":1},{"sku_id":0,"quantity":2}]}`)
	if err := handleCompensation(context.Background(), fake, payload); err == nil {
		t.Fatal("expected retryable error")
	}
	if fake.calls != 1 || fake.order != "ORD-1" || len(fake.items) != 1 || fake.items[0].GetSkuId() != 1 {
		t.Fatalf("call=%d order=%s items=%+v", fake.calls, fake.order, fake.items)
	}

	fake.failed = false
	if err := handleCompensation(context.Background(), fake, payload); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 2 {
		t.Fatalf("calls=%d", fake.calls)
	}

	if err := handleCompensation(context.Background(), fake, []byte(`not-json`)); err == nil {
		t.Fatal("malformed payload accepted")
	}
}

var _ = consumer.Permanent
