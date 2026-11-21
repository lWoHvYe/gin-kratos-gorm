package inventory

import (
	"context"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

type fakeConfirm struct {
	calls   int
	typ     string
	order   string
	items   []*pb.OrderItem
	summary operationSummary
}

func (f *fakeConfirm) applyInventoryOperation(_ context.Context, typ, orderNo string, items []*pb.OrderItem) (operationSummary, error) {
	f.calls++
	f.typ = typ
	f.order = orderNo
	f.items = items
	if f.summary.Status == "" {
		f.summary.Status = opStatusSucceeded
	}
	return f.summary, nil
}

func TestConfirmHandlerUsesOrderScope(t *testing.T) {
	f := &fakeConfirm{}
	if err := handleConfirmRequest(context.Background(), f, []byte(`{"order_no":"ORD-C","items":[{"sku_id":1,"quantity":2},{"sku_id":0,"quantity":1}]}`)); err != nil {
		t.Fatal(err)
	}
	if f.calls != 1 || f.typ != opTypeConfirm || f.order != "ORD-C" || len(f.items) != 1 || f.items[0].GetQuantity() != 2 {
		t.Fatalf("calls=%d type=%s order=%s items=%+v", f.calls, f.typ, f.order, f.items)
	}
}

func TestConfirmHandlerRetriesPendingItems(t *testing.T) {
	f := &fakeConfirm{summary: operationSummary{Status: opStatusInProgress, Retryable: 1}}
	err := handleConfirmRequest(context.Background(), f,
		[]byte(`{"order_no":"ORD-PENDING","items":[{"sku_id":1,"quantity":1}]}`))
	if err == nil {
		t.Fatal("pending item must cause a retryable handler error")
	}
	if f.calls != 1 {
		t.Fatalf("calls = %d", f.calls)
	}
}

func TestConfirmHandlerAcksTerminalPartial(t *testing.T) {
	f := &fakeConfirm{summary: operationSummary{Status: opStatusPartial, Failed: 1}}
	err := handleConfirmRequest(context.Background(), f,
		[]byte(`{"order_no":"ORD-PARTIAL","items":[{"sku_id":1,"quantity":1}]}`))
	if err != nil {
		t.Fatalf("terminal PARTIAL must be Acked, got %v", err)
	}
}
