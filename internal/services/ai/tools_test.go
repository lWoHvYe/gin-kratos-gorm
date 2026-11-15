package ai

import (
	"context"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"github.com/cloudwego/eino/components/tool"
	"google.golang.org/grpc"
)

type fakeOrders struct {
	lastList *pb.ListOrdersRequest
	order    *pb.Order
}

func (f *fakeOrders) ListOrders(_ context.Context, in *pb.ListOrdersRequest, _ ...grpc.CallOption) (*pb.OrderList, error) {
	f.lastList = in
	return &pb.OrderList{Total: 1, Orders: []*pb.Order{{
		OrderNo: "ORD-1", UserId: in.GetUserId(), Status: "PAID", PayCent: 9900,
		Items: []*pb.OrderItem{{Name: "Widget", Quantity: 1}},
	}}}, nil
}
func (f *fakeOrders) GetOrder(_ context.Context, in *pb.GetOrderRequest, _ ...grpc.CallOption) (*pb.Order, error) {
	if f.order != nil {
		return f.order, nil
	}
	return &pb.Order{OrderNo: in.GetOrderNo(), UserId: in.GetUserId(), Status: "PAID"}, nil
}

type fakeAfterSales struct {
	lastList *pb.ListAfterSalesRequest
	record   *pb.AfterSale
}

func (f *fakeAfterSales) ListAfterSales(_ context.Context, in *pb.ListAfterSalesRequest, _ ...grpc.CallOption) (*pb.AfterSaleList, error) {
	f.lastList = in
	return &pb.AfterSaleList{Total: 1, AfterSales: []*pb.AfterSale{{
		AfterSaleNo: "AS-1", OrderNo: "ORD-1", UserId: in.GetUserId(), Status: "APPROVED", AmountCent: 500,
	}}}, nil
}
func (f *fakeAfterSales) GetAfterSale(_ context.Context, in *pb.GetAfterSaleRequest, _ ...grpc.CallOption) (*pb.AfterSale, error) {
	if f.record != nil {
		return f.record, nil
	}
	return &pb.AfterSale{AfterSaleNo: in.GetAfterSaleNo(), UserId: 7, Status: "APPROVED"}, nil
}

func runTool(t *testing.T, base tool.BaseTool, ctx context.Context, args string) (string, error) {
	t.Helper()
	invokable, ok := base.(tool.InvokableTool)
	if !ok {
		t.Fatalf("tool %T is not invokable", base)
	}
	return invokable.InvokableRun(ctx, args)
}

func TestOrderToolsUseContextUserOnly(t *testing.T) {
	orders := &fakeOrders{}
	list, err := newListOrdersTool(orders)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runTool(t, list, withUserID(context.Background(), 7), `{"limit":3}`)
	if err != nil {
		t.Fatal(err)
	}
	if orders.lastList.GetUserId() != 7 || orders.lastList.GetPageSize() != 3 {
		t.Fatalf("request = %+v", orders.lastList)
	}
	if !strings.Contains(out, "ORD-1") || !strings.Contains(out, "Widget") {
		t.Fatalf("out = %s", out)
	}
	if _, err := runTool(t, list, context.Background(), `{}`); err == nil {
		t.Fatal("expected missing user error")
	}

	get, err := newGetOrderTool(orders)
	if err != nil {
		t.Fatal(err)
	}
	orders.order = &pb.Order{OrderNo: "ORD-2", UserId: 99, Status: "PAID"}
	if _, err := runTool(t, get, withUserID(context.Background(), 7), `{"order_no":"ORD-2"}`); err == nil {
		t.Fatal("expected ownership rejection")
	}
}

func TestAfterSaleToolEnforcesOwnership(t *testing.T) {
	afterSales := &fakeAfterSales{record: &pb.AfterSale{AfterSaleNo: "AS-9", UserId: 99}}
	get, err := newGetAfterSaleTool(afterSales)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runTool(t, get, withUserID(context.Background(), 7), `{"after_sale_no":"AS-9"}`); err == nil {
		t.Fatal("expected ownership rejection")
	}
	afterSales.record = &pb.AfterSale{AfterSaleNo: "AS-9", UserId: 7, Status: "REFUNDED"}
	out, err := runTool(t, get, withUserID(context.Background(), 7), `{"after_sale_no":"AS-9"}`)
	if err != nil || !strings.Contains(out, "REFUNDED") {
		t.Fatalf("out = %s err = %v", out, err)
	}
}

func TestPolicyToolAnswersFromCuratedText(t *testing.T) {
	p, err := newPolicyTool()
	if err != nil {
		t.Fatal(err)
	}
	out, err := runTool(t, p, context.Background(), `{"query":"退货政策"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "退货退款流程") || !strings.Contains(out, "退款") {
		t.Fatalf("out = %s", out)
	}
}
