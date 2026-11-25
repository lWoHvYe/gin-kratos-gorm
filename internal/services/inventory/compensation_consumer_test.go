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
	typ    string
	order  string
	items  []*pb.OrderItem
}

func (f *fakeCompensation) applyInventoryOperation(_ context.Context, typ, orderNo string, items []*pb.OrderItem) (operationSummary, error) {
	f.calls++
	f.typ = typ
	f.order = orderNo
	f.items = items
	if f.failed {
		return operationSummary{}, context.DeadlineExceeded
	}
	return operationSummary{Status: opStatusSucceeded}, nil
}

func TestCompensationHandlerRetriesAndSkipsInvalidItems(t *testing.T) {
	fake := &fakeCompensation{failed: true}
	payload := []byte(`{"order_no":"ORD-1","items":[{"sku_id":1,"warehouse_id":1,"quantity":1},{"sku_id":0,"quantity":2}]}`)
	if err := handleCompensation(context.Background(), fake, payload); err == nil {
		t.Fatal("expected retryable error")
	}
	if fake.calls != 1 || fake.typ != opTypeCancel || fake.order != "ORD-1" || len(fake.items) != 1 || fake.items[0].GetSkuId() != 1 {
		t.Fatalf("call=%d type=%s order=%s items=%+v", fake.calls, fake.typ, fake.order, fake.items)
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

func TestBatchGetStockRejectsEmptyList(t *testing.T) {
	if _, err := (&Service{}).BatchGetStock(context.Background(), &pb.BatchGetStockRequest{}); err == nil {
		t.Fatal("empty batch must be rejected")
	}
}
