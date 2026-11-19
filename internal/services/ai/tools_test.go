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
func (f *fakeOrders) GetOrderStats(_ context.Context, in *pb.StatsRequest, _ ...grpc.CallOption) (*pb.OrderStats, error) {
	return &pb.OrderStats{FromUnix: in.GetFromUnix(), ToUnix: in.GetToUnix(), TotalOrders: 4, PaidOrders: 3, CompletedOrders: 2, PaidCent: 12345}, nil
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
func (f *fakeAfterSales) GetAfterSaleStats(_ context.Context, in *pb.StatsRequest, _ ...grpc.CallOption) (*pb.AfterSaleStats, error) {
	return &pb.AfterSaleStats{FromUnix: in.GetFromUnix(), ToUnix: in.GetToUnix(), Total: 3, Requested: 1, Approved: 1, Refunded: 1, RefundedCent: 500}, nil
}

type fakeInventory struct{}

func (fakeInventory) ListLowStock(_ context.Context, _ *pb.ListLowStockRequest, _ ...grpc.CallOption) (*pb.StockList, error) {
	return &pb.StockList{Total: 1, Stocks: []*pb.Stock{{SkuId: 9, WarehouseId: 1, OnHand: 8, Reserved: 2, Available: 6}}}, nil
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

func TestAdminToolsAggregateWithoutUserScope(t *testing.T) {
	d := Deps{Orders: &fakeOrders{}, AfterSales: &fakeAfterSales{}, Inventory: fakeInventory{}}
	tools, err := newAdminTools(d)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]tool.BaseTool{}
	for _, tl := range tools {
		info, _ := tl.Info(context.Background())
		byName[info.Name] = tl
	}
	for name, args := range map[string]string{
		"sales_summary":  `{"days":30}`,
		"refund_summary": `{"days":7}`,
		"low_stock":      `{"threshold":10,"limit":5}`,
	} {
		out, err := runTool(t, byName[name], context.Background(), args)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if name == "sales_summary" && !strings.Contains(out, "12345") {
			t.Fatalf("sales out = %s", out)
		}
		if name == "low_stock" && !strings.Contains(out, `"available":6`) {
			t.Fatalf("stock out = %s", out)
		}
	}
}

func TestCampaignDraftIsNeverPersisted(t *testing.T) {
	tl, err := newCampaignDraftTool()
	if err != nil {
		t.Fatal(err)
	}
	out, err := runTool(t, tl, context.Background(), `{"name":"满 300 减 30","type":"THRESHOLD","threshold_cent":30000,"discount_cent":3000,"days":7,"budget_cent":100000}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"status":"DRAFT"`) || !strings.Contains(out, `"stack_mode":"PRIORITY"`) || !strings.Contains(out, "subtotal_cent >= 30000") {
		t.Fatalf("draft = %s", out)
	}
	if !strings.Contains(out, "不会写入数据库") {
		t.Fatalf("missing no-write note: %s", out)
	}
	if _, err := runTool(t, tl, context.Background(), `{"type":"PERCENT","percent":150}`); err == nil {
		t.Fatal("expected percent validation error")
	}
}

type fakeRAG struct{}

func (fakeRAG) Search(context.Context, string, int) ([]ragDoc, error) {
	return []ragDoc{{ID: "product:1002", Kind: "product", Title: "云舟 M3 手机", Content: "6000mAh 大电池，适合学生和重度使用。", ProductID: 1002}}, nil
}

func TestKnowledgeToolReturnsIndexedPassages(t *testing.T) {
	tl, err := newKnowledgeTool(fakeRAG{})
	if err != nil {
		t.Fatal(err)
	}
	out, err := runTool(t, tl, context.Background(), `{"query":"适合大学生的手机"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "云舟 M3") || !strings.Contains(out, "适合学生") {
		t.Fatalf("out = %s", out)
	}
}

type fakePromotion struct{}

func (fakePromotion) EvaluateDraft(context.Context, *pb.EvaluateDraftRequest, ...grpc.CallOption) (*pb.PromotionEvaluation, error) {
	return &pb.PromotionEvaluation{DiscountCent: 3000, Applied: []*pb.AppliedPromotion{{Code: "DRAFT"}}}, nil
}

func TestPreviewCampaignUsesDraftEvaluation(t *testing.T) {
	tl, err := newPreviewCampaignTool(fakePromotion{})
	if err != nil {
		t.Fatal(err)
	}
	out, err := runTool(t, tl, withUserID(context.Background(), 7), `{"type":"THRESHOLD","threshold_cent":30000,"discount_cent":3000,"sample_price_cent":30000}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "3000") || !strings.Contains(out, "DRAFT") {
		t.Fatalf("out = %s", out)
	}
}
