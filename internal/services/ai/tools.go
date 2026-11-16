package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/metrics"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"google.golang.org/grpc"
)

// The assistant only gets read-only projections of the owning services. User
// scoping comes from the request context, never from model arguments.
type catalogClient interface {
	ListProducts(ctx context.Context, in *pb.ListProductsRequest, opts ...grpc.CallOption) (*pb.ProductList, error)
	GetProduct(ctx context.Context, in *pb.GetProductRequest, opts ...grpc.CallOption) (*pb.Product, error)
}

type orderClient interface {
	ListOrders(ctx context.Context, in *pb.ListOrdersRequest, opts ...grpc.CallOption) (*pb.OrderList, error)
	GetOrder(ctx context.Context, in *pb.GetOrderRequest, opts ...grpc.CallOption) (*pb.Order, error)
	GetOrderStats(ctx context.Context, in *pb.StatsRequest, opts ...grpc.CallOption) (*pb.OrderStats, error)
}

type afterSalesClient interface {
	ListAfterSales(ctx context.Context, in *pb.ListAfterSalesRequest, opts ...grpc.CallOption) (*pb.AfterSaleList, error)
	GetAfterSale(ctx context.Context, in *pb.GetAfterSaleRequest, opts ...grpc.CallOption) (*pb.AfterSale, error)
	GetAfterSaleStats(ctx context.Context, in *pb.StatsRequest, opts ...grpc.CallOption) (*pb.AfterSaleStats, error)
}

type inventoryClient interface {
	ListLowStock(ctx context.Context, in *pb.ListLowStockRequest, opts ...grpc.CallOption) (*pb.StockList, error)
}

// Deps are the read-only clients the agent tools may call.
type Deps struct {
	Catalog    catalogClient
	Orders     orderClient
	AfterSales afterSalesClient
	Inventory  inventoryClient
}

type deps = Deps

const (
	toolTimeout   = 5 * time.Second
	maxToolResult = 10
)

func newTools(d deps) ([]tool.BaseTool, error) {
	search, err := newSearchProductsTool(d.Catalog)
	if err != nil {
		return nil, err
	}
	getProduct, err := newGetProductTool(d.Catalog)
	if err != nil {
		return nil, err
	}
	listOrders, err := newListOrdersTool(d.Orders)
	if err != nil {
		return nil, err
	}
	getOrder, err := newGetOrderTool(d.Orders)
	if err != nil {
		return nil, err
	}
	listAfterSales, err := newListAfterSalesTool(d.AfterSales)
	if err != nil {
		return nil, err
	}
	getAfterSale, err := newGetAfterSaleTool(d.AfterSales)
	if err != nil {
		return nil, err
	}
	policy, err := newPolicyTool()
	if err != nil {
		return nil, err
	}
	return []tool.BaseTool{search, getProduct, listOrders, getOrder, listAfterSales, getAfterSale, policy}, nil
}

// ---------------------------------------------------------------- products

type searchProductsInput struct {
	Query        string `json:"query,omitempty" jsonschema:"description=Search keywords, e.g. 无线耳机"`
	CategoryID   int64  `json:"category_id,omitempty" jsonschema:"description=Optional category id"`
	BrandID      int64  `json:"brand_id,omitempty" jsonschema:"description=Optional brand id"`
	MinPriceCent int64  `json:"min_price_cent,omitempty" jsonschema:"description=Minimum price in cents"`
	MaxPriceCent int64  `json:"max_price_cent,omitempty" jsonschema:"description=Maximum price in cents"`
	Sort         string `json:"sort,omitempty" jsonschema:"description=price_asc, price_desc or newest"`
	PageSize     int32  `json:"page_size,omitempty" jsonschema:"description=Number of products to return, max 10"`
}

type productSummary struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Subtitle     string   `json:"subtitle,omitempty"`
	Description  string   `json:"description,omitempty"`
	MinPriceCent int64    `json:"min_price_cent"`
	MainImage    string   `json:"main_image,omitempty"`
	Attributes   []string `json:"attributes,omitempty"`
}

type searchProductsOutput struct {
	Products []productSummary `json:"products"`
	Total    int64            `json:"total"`
}

type getProductInput struct {
	ProductID int64 `json:"product_id" jsonschema:"description=Product id returned by search_products"`
}

func newSearchProductsTool(catalog catalogClient) (tool.BaseTool, error) {
	return utils.InferTool("search_products",
		"Search on-sale products by keyword, category, brand or price range. Use this before recommending anything.",
		func(ctx context.Context, in searchProductsInput) (searchProductsOutput, error) {
			ctx, cancel := context.WithTimeout(ctx, toolTimeout)
			defer cancel()
			pageSize := in.PageSize
			if pageSize <= 0 {
				pageSize = 5
			}
			if pageSize > maxToolResult {
				pageSize = maxToolResult
			}
			list, err := catalog.ListProducts(ctx, &pb.ListProductsRequest{
				Page:         1,
				PageSize:     pageSize,
				Keyword:      strings.TrimSpace(in.Query),
				CategoryId:   in.CategoryID,
				BrandId:      in.BrandID,
				MinPriceCent: in.MinPriceCent,
				MaxPriceCent: in.MaxPriceCent,
				Sort:         strings.TrimSpace(in.Sort),
			})
			if err != nil {
				metrics.AIToolCalls.WithLabelValues("search_products", "error").Inc()
				return searchProductsOutput{}, err
			}
			out := searchProductsOutput{Total: list.GetTotal()}
			for _, p := range list.GetProducts() {
				out.Products = append(out.Products, summarizeProduct(p))
			}
			metrics.AIToolCalls.WithLabelValues("search_products", "ok").Inc()
			return out, nil
		})
}

func newGetProductTool(catalog catalogClient) (tool.BaseTool, error) {
	return utils.InferTool("get_product",
		"Fetch one product's details and attributes by id. Use it when the user asks about a specific product.",
		func(ctx context.Context, in getProductInput) (productSummary, error) {
			ctx, cancel := context.WithTimeout(ctx, toolTimeout)
			defer cancel()
			p, err := catalog.GetProduct(ctx, &pb.GetProductRequest{Id: in.ProductID})
			if err != nil {
				metrics.AIToolCalls.WithLabelValues("get_product", "error").Inc()
				return productSummary{}, err
			}
			metrics.AIToolCalls.WithLabelValues("get_product", "ok").Inc()
			return summarizeProduct(p), nil
		})
}

func summarizeProduct(p *pb.Product) productSummary {
	if p == nil {
		return productSummary{}
	}
	desc := []rune(strings.TrimSpace(p.GetDescription()))
	if len(desc) > 500 {
		desc = append(desc[:500], '…')
	}
	out := productSummary{
		ID:           p.GetId(),
		Name:         p.GetName(),
		Subtitle:     p.GetSubtitle(),
		Description:  string(desc),
		MinPriceCent: p.GetMinPriceCent(),
		MainImage:    p.GetMainImage(),
	}
	for _, a := range p.GetAttributes() {
		if a.GetName() == "" {
			continue
		}
		out.Attributes = append(out.Attributes, a.GetName()+": "+a.GetValue())
	}
	return out
}

// ---------------------------------------------------------------- orders

type listOrdersInput struct {
	Status string `json:"status,omitempty" jsonschema:"description=Optional order status, e.g. PENDING_PAYMENT or PAID"`
	Limit  int32  `json:"limit,omitempty" jsonschema:"description=Number of orders, max 10"`
}

type getOrderInput struct {
	OrderNo string `json:"order_no" jsonschema:"description=Order number shown in the order list"`
}

type orderSummary struct {
	OrderNo   string   `json:"order_no"`
	Status    string   `json:"status"`
	PayCent   int64    `json:"pay_cent"`
	CreatedAt int64    `json:"created_at_unix"`
	Items     []string `json:"items,omitempty"`
}

type orderListOutput struct {
	Orders []orderSummary `json:"orders"`
	Total  int64          `json:"total"`
}

func newListOrdersTool(orders orderClient) (tool.BaseTool, error) {
	return utils.InferTool("list_my_orders",
		"List the current user's orders. Use it for questions like 我的订单/发货了吗. Never guess order data.",
		func(ctx context.Context, in listOrdersInput) (orderListOutput, error) {
			uid, err := scopedUser(ctx)
			if err != nil {
				return orderListOutput{}, err
			}
			limit := in.Limit
			if limit <= 0 {
				limit = 5
			}
			if limit > maxToolResult {
				limit = maxToolResult
			}
			ctx, cancel := context.WithTimeout(ctx, toolTimeout)
			defer cancel()
			list, err := orders.ListOrders(ctx, &pb.ListOrdersRequest{
				UserId: uid, Page: 1, PageSize: limit, Status: strings.TrimSpace(in.Status),
			})
			if err != nil {
				metrics.AIToolCalls.WithLabelValues("list_my_orders", "error").Inc()
				return orderListOutput{}, err
			}
			out := orderListOutput{Total: list.GetTotal()}
			for _, o := range list.GetOrders() {
				out.Orders = append(out.Orders, summarizeOrder(o))
			}
			metrics.AIToolCalls.WithLabelValues("list_my_orders", "ok").Inc()
			return out, nil
		})
}

func newGetOrderTool(orders orderClient) (tool.BaseTool, error) {
	return utils.InferTool("get_my_order",
		"Fetch one of the current user's orders by order number. Use it when the user mentions a specific order.",
		func(ctx context.Context, in getOrderInput) (orderSummary, error) {
			uid, err := scopedUser(ctx)
			if err != nil {
				return orderSummary{}, err
			}
			ctx, cancel := context.WithTimeout(ctx, toolTimeout)
			defer cancel()
			o, err := orders.GetOrder(ctx, &pb.GetOrderRequest{OrderNo: strings.TrimSpace(in.OrderNo), UserId: uid})
			if err != nil {
				metrics.AIToolCalls.WithLabelValues("get_my_order", "error").Inc()
				return orderSummary{}, err
			}
			if o.GetUserId() != 0 && o.GetUserId() != uid {
				return orderSummary{}, fmt.Errorf("order does not belong to the current user")
			}
			metrics.AIToolCalls.WithLabelValues("get_my_order", "ok").Inc()
			return summarizeOrder(o), nil
		})
}

func summarizeOrder(o *pb.Order) orderSummary {
	if o == nil {
		return orderSummary{}
	}
	out := orderSummary{
		OrderNo:   o.GetOrderNo(),
		Status:    o.GetStatus(),
		PayCent:   o.GetPayCent(),
		CreatedAt: o.GetCreatedAtUnix(),
	}
	for _, item := range o.GetItems() {
		if item.GetName() == "" {
			continue
		}
		out.Items = append(out.Items, fmt.Sprintf("%s x%d", item.GetName(), item.GetQuantity()))
	}
	return out
}

// ---------------------------------------------------------------- after-sales

type listAfterSalesInput struct {
	Status string `json:"status,omitempty" jsonschema:"description=Optional after-sale status"`
	Limit  int32  `json:"limit,omitempty" jsonschema:"description=Number of records, max 10"`
}

type getAfterSaleInput struct {
	AfterSaleNo string `json:"after_sale_no" jsonschema:"description=After-sale number shown in the after-sale list"`
}

type afterSaleSummary struct {
	AfterSaleNo string `json:"after_sale_no"`
	OrderNo     string `json:"order_no"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	AmountCent  int64  `json:"amount_cent"`
	RefundNo    string `json:"refund_no,omitempty"`
	CreatedAt   int64  `json:"created_at_unix"`
}

type afterSaleListOutput struct {
	AfterSales []afterSaleSummary `json:"after_sales"`
	Total      int64              `json:"total"`
}

func newListAfterSalesTool(afterSales afterSalesClient) (tool.BaseTool, error) {
	return utils.InferTool("list_my_after_sales",
		"List the current user's after-sale/refund records. Use it for 退货/退款进度 questions.",
		func(ctx context.Context, in listAfterSalesInput) (afterSaleListOutput, error) {
			uid, err := scopedUser(ctx)
			if err != nil {
				return afterSaleListOutput{}, err
			}
			limit := in.Limit
			if limit <= 0 {
				limit = 5
			}
			if limit > maxToolResult {
				limit = maxToolResult
			}
			ctx, cancel := context.WithTimeout(ctx, toolTimeout)
			defer cancel()
			list, err := afterSales.ListAfterSales(ctx, &pb.ListAfterSalesRequest{
				UserId: uid, Page: 1, PageSize: limit, Status: strings.TrimSpace(in.Status),
			})
			if err != nil {
				metrics.AIToolCalls.WithLabelValues("list_my_after_sales", "error").Inc()
				return afterSaleListOutput{}, err
			}
			out := afterSaleListOutput{Total: list.GetTotal()}
			for _, a := range list.GetAfterSales() {
				out.AfterSales = append(out.AfterSales, summarizeAfterSale(a))
			}
			metrics.AIToolCalls.WithLabelValues("list_my_after_sales", "ok").Inc()
			return out, nil
		})
}

func newGetAfterSaleTool(afterSales afterSalesClient) (tool.BaseTool, error) {
	return utils.InferTool("get_my_after_sale",
		"Fetch one after-sale record by number. Use it for a specific refund/return question.",
		func(ctx context.Context, in getAfterSaleInput) (afterSaleSummary, error) {
			uid, err := scopedUser(ctx)
			if err != nil {
				return afterSaleSummary{}, err
			}
			ctx, cancel := context.WithTimeout(ctx, toolTimeout)
			defer cancel()
			a, err := afterSales.GetAfterSale(ctx, &pb.GetAfterSaleRequest{AfterSaleNo: strings.TrimSpace(in.AfterSaleNo)})
			if err != nil {
				metrics.AIToolCalls.WithLabelValues("get_my_after_sale", "error").Inc()
				return afterSaleSummary{}, err
			}
			// GetAfterSale has no user filter, so enforce ownership here.
			if a.GetUserId() != uid {
				return afterSaleSummary{}, fmt.Errorf("after-sale does not belong to the current user")
			}
			metrics.AIToolCalls.WithLabelValues("get_my_after_sale", "ok").Inc()
			return summarizeAfterSale(a), nil
		})
}

func summarizeAfterSale(a *pb.AfterSale) afterSaleSummary {
	if a == nil {
		return afterSaleSummary{}
	}
	return afterSaleSummary{
		AfterSaleNo: a.GetAfterSaleNo(),
		OrderNo:     a.GetOrderNo(),
		Type:        a.GetType(),
		Status:      a.GetStatus(),
		AmountCent:  a.GetAmountCent(),
		RefundNo:    a.GetRefundNo(),
		CreatedAt:   a.GetCreatedAtUnix(),
	}
}

// ---------------------------------------------------------------- policy

type policyInput struct {
	Query string `json:"query" jsonschema:"description=Policy question, e.g. 退货政策 or 发货时间"`
}

type policyOutput struct {
	Answer  string   `json:"answer"`
	Sources []string `json:"sources,omitempty"`
}

func newPolicyTool() (tool.BaseTool, error) {
	return utils.InferTool("search_policy",
		"Answer mall policy questions (returns, shipping, cancellation, payment, seckill) from the reviewed FAQ.",
		func(_ context.Context, in policyInput) (policyOutput, error) {
			answer, sources := formatPolicy(searchPolicy(in.Query, 2))
			metrics.AIToolCalls.WithLabelValues("search_policy", "ok").Inc()
			return policyOutput{Answer: answer, Sources: sources}, nil
		})
}

func scopedUser(ctx context.Context) (int64, error) {
	uid, ok := userIDFromContext(ctx)
	if !ok {
		return 0, fmt.Errorf("authenticated user is required")
	}
	return uid, nil
}

// ---------------------------------------------------------------- admin analytics

type daysInput struct {
	Days int32 `json:"days,omitempty" jsonschema:"description=Window in days, default 30, max 366"`
}

type salesSummaryOutput struct {
	FromUnix        int64 `json:"from_unix"`
	ToUnix          int64 `json:"to_unix"`
	TotalOrders     int64 `json:"total_orders"`
	PaidOrders      int64 `json:"paid_orders"`
	CompletedOrders int64 `json:"completed_orders"`
	CancelledOrders int64 `json:"cancelled_orders"`
	PaidCent        int64 `json:"paid_cent"`
}

type refundSummaryOutput struct {
	FromUnix     int64 `json:"from_unix"`
	ToUnix       int64 `json:"to_unix"`
	Total        int64 `json:"total"`
	Requested    int64 `json:"requested"`
	Approved     int64 `json:"approved"`
	Refunded     int64 `json:"refunded"`
	RefundedCent int64 `json:"refunded_cent"`
}

type lowStockInput struct {
	Threshold int64 `json:"threshold,omitempty" jsonschema:"description=Report rows whose available stock is <= threshold, default 10"`
	Limit     int32 `json:"limit,omitempty" jsonschema:"description=Number of rows, max 20"`
}

type stockSummary struct {
	SkuID     int64 `json:"sku_id"`
	Warehouse int64 `json:"warehouse_id"`
	OnHand    int64 `json:"on_hand"`
	Reserved  int64 `json:"reserved"`
	Available int64 `json:"available"`
}

type lowStockOutput struct {
	Stocks []stockSummary `json:"stocks"`
	Total  int64          `json:"total"`
}

func statsRange(days int32) (int64, int64) {
	if days <= 0 {
		days = 30
	}
	if days > 366 {
		days = 366
	}
	to := time.Now()
	from := to.AddDate(0, 0, -int(days))
	return from.Unix(), to.Unix()
}

func newAdminTools(d Deps) ([]tool.BaseTool, error) {
	sales, err := utils.InferTool("sales_summary",
		"Read-only order aggregates (counts and paid amount) for a recent window. Use for 销售/订单/营收 questions.",
		func(ctx context.Context, in daysInput) (salesSummaryOutput, error) {
			from, to := statsRange(in.Days)
			ctx, cancel := context.WithTimeout(ctx, toolTimeout)
			defer cancel()
			stats, err := d.Orders.GetOrderStats(ctx, &pb.StatsRequest{FromUnix: from, ToUnix: to})
			if err != nil {
				metrics.AIToolCalls.WithLabelValues("sales_summary", "error").Inc()
				return salesSummaryOutput{}, err
			}
			metrics.AIToolCalls.WithLabelValues("sales_summary", "ok").Inc()
			return salesSummaryOutput{
				FromUnix: stats.GetFromUnix(), ToUnix: stats.GetToUnix(),
				TotalOrders: stats.GetTotalOrders(), PaidOrders: stats.GetPaidOrders(),
				CompletedOrders: stats.GetCompletedOrders(), CancelledOrders: stats.GetCancelledOrders(),
				PaidCent: stats.GetPaidCent(),
			}, nil
		})
	if err != nil {
		return nil, err
	}

	refunds, err := utils.InferTool("refund_summary",
		"Read-only after-sale/refund aggregates for a recent window. Use for 退款/售后 questions.",
		func(ctx context.Context, in daysInput) (refundSummaryOutput, error) {
			from, to := statsRange(in.Days)
			ctx, cancel := context.WithTimeout(ctx, toolTimeout)
			defer cancel()
			stats, err := d.AfterSales.GetAfterSaleStats(ctx, &pb.StatsRequest{FromUnix: from, ToUnix: to})
			if err != nil {
				metrics.AIToolCalls.WithLabelValues("refund_summary", "error").Inc()
				return refundSummaryOutput{}, err
			}
			metrics.AIToolCalls.WithLabelValues("refund_summary", "ok").Inc()
			return refundSummaryOutput{
				FromUnix: stats.GetFromUnix(), ToUnix: stats.GetToUnix(),
				Total: stats.GetTotal(), Requested: stats.GetRequested(),
				Approved: stats.GetApproved(), Refunded: stats.GetRefunded(),
				RefundedCent: stats.GetRefundedCent(),
			}, nil
		})
	if err != nil {
		return nil, err
	}

	low, err := utils.InferTool("low_stock",
		"List stock rows closest to running out (available <= threshold), for 库存/补货 questions.",
		func(ctx context.Context, in lowStockInput) (lowStockOutput, error) {
			threshold := in.Threshold
			if threshold <= 0 {
				threshold = 10
			}
			limit := in.Limit
			if limit <= 0 {
				limit = 10
			}
			if limit > 20 {
				limit = 20
			}
			ctx, cancel := context.WithTimeout(ctx, toolTimeout)
			defer cancel()
			list, err := d.Inventory.ListLowStock(ctx, &pb.ListLowStockRequest{Threshold: threshold, Limit: limit})
			if err != nil {
				metrics.AIToolCalls.WithLabelValues("low_stock", "error").Inc()
				return lowStockOutput{}, err
			}
			out := lowStockOutput{Total: list.GetTotal()}
			for _, st := range list.GetStocks() {
				out.Stocks = append(out.Stocks, stockSummary{
					SkuID: st.GetSkuId(), Warehouse: st.GetWarehouseId(),
					OnHand: st.GetOnHand(), Reserved: st.GetReserved(), Available: st.GetAvailable(),
				})
			}
			metrics.AIToolCalls.WithLabelValues("low_stock", "ok").Inc()
			return out, nil
		})
	if err != nil {
		return nil, err
	}
	draft, err := newCampaignDraftTool()
	if err != nil {
		return nil, err
	}
	return []tool.BaseTool{sales, refunds, low, draft}, nil
}

type campaignDraftInput struct {
	Name          string `json:"name,omitempty" jsonschema:"description=Campaign name; a default is used when empty"`
	Type          string `json:"type" jsonschema:"description=THRESHOLD, PERCENT, FIXED_AMOUNT or FREE_SHIPPING"`
	ThresholdCent int64  `json:"threshold_cent,omitempty" jsonschema:"description=Minimum cart subtotal in cents"`
	DiscountCent  int64  `json:"discount_cent,omitempty" jsonschema:"description=Fixed reduction in cents (THRESHOLD/FIXED_AMOUNT)"`
	Percent       int32  `json:"percent,omitempty" jsonschema:"description=Discount percent for PERCENT, 1-100"`
	Days          int32  `json:"days,omitempty" jsonschema:"description=Duration in days, default 7, max 90"`
	BudgetCent    int64  `json:"budget_cent,omitempty" jsonschema:"description=Optional campaign budget in cents"`
	PerUserLimit  int32  `json:"per_user_limit,omitempty" jsonschema:"description=Optional per-user limit"`
}

type campaignDraft struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	StartAtUnix  int64  `json:"start_at_unix"`
	EndAtUnix    int64  `json:"end_at_unix"`
	Priority     int32  `json:"priority"`
	BudgetCent   int64  `json:"budget_cent"`
	PerUserLimit int32  `json:"per_user_limit"`
	StackMode    string `json:"stack_mode"`
}

type promotionRuleDraft struct {
	Code            string `json:"code"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Status          string `json:"status"`
	Priority        int32  `json:"priority"`
	Exclusive       bool   `json:"exclusive"`
	StackGroup      string `json:"stack_group"`
	ScopeJSON       string `json:"scope_json"`
	ConditionExpr   string `json:"condition_expr"`
	EffectJSON      string `json:"effect_json"`
	MaxDiscountCent int64  `json:"max_discount_cent"`
	BudgetCent      int64  `json:"budget_cent"`
	PerUserLimit    int32  `json:"per_user_limit"`
	AllowWithCoupon bool   `json:"allow_with_coupon"`
	StartAtUnix     int64  `json:"start_at_unix"`
	EndAtUnix       int64  `json:"end_at_unix"`
}

type campaignDraftOutput struct {
	Campaign campaignDraft      `json:"campaign"`
	Rule     promotionRuleDraft `json:"rule"`
	Notes    []string           `json:"notes"`
}

func newCampaignDraftTool() (tool.BaseTool, error) {
	return utils.InferTool("draft_campaign",
		"Build a DRAFT campaign + promotion rule from a few numbers. This never writes to the database; an admin must review and save it.",
		func(_ context.Context, in campaignDraftInput) (campaignDraftOutput, error) {
			typ := strings.ToUpper(strings.TrimSpace(in.Type))
			if typ == "" {
				typ = "THRESHOLD"
			}
			switch typ {
			case "THRESHOLD", "PERCENT", "FIXED_AMOUNT", "FREE_SHIPPING":
			default:
				return campaignDraftOutput{}, fmt.Errorf("unsupported campaign type %q", in.Type)
			}
			now := time.Now()
			days := in.Days
			if days <= 0 {
				days = 7
			}
			if days > 90 {
				days = 90
			}
			end := now.AddDate(0, 0, int(days))
			name := strings.TrimSpace(in.Name)
			if name == "" {
				name = "AI 活动草案"
			}
			stamp := now.Unix()
			notes := []string{
				"草案不会写入数据库；确认后才可通过管理端活动接口保存。",
				"预览需要先保存活动，当前版本不提供草稿模拟。",
			}
			ruleType := typ
			condition := ""
			effect := ""
			maxDiscount := int64(0)
			switch typ {
			case "THRESHOLD":
				if in.ThresholdCent <= 0 || in.DiscountCent <= 0 {
					return campaignDraftOutput{}, fmt.Errorf("threshold_cent and discount_cent are required for THRESHOLD")
				}
				condition = fmt.Sprintf("subtotal_cent >= %d", in.ThresholdCent)
				effect = fmt.Sprintf(`{"type":"THRESHOLD","tiers":[{"min_cent":%d,"amount_cent":%d}]}`, in.ThresholdCent, in.DiscountCent)
				maxDiscount = in.DiscountCent
			case "PERCENT":
				if in.Percent <= 0 || in.Percent > 100 {
					return campaignDraftOutput{}, fmt.Errorf("percent must be between 1 and 100")
				}
				if in.ThresholdCent > 0 {
					condition = fmt.Sprintf("subtotal_cent >= %d", in.ThresholdCent)
				}
				effect = fmt.Sprintf(`{"type":"PERCENT","percent":%d,"max_discount_cent":%d}`, in.Percent, in.DiscountCent)
				maxDiscount = in.DiscountCent
			case "FIXED_AMOUNT":
				if in.DiscountCent <= 0 {
					return campaignDraftOutput{}, fmt.Errorf("discount_cent is required for FIXED_AMOUNT")
				}
				if in.ThresholdCent > 0 {
					condition = fmt.Sprintf("subtotal_cent >= %d", in.ThresholdCent)
				}
				effect = fmt.Sprintf(`{"type":"FIXED_AMOUNT","amount_cent":%d}`, in.DiscountCent)
				maxDiscount = in.DiscountCent
			case "FREE_SHIPPING":
				if in.ThresholdCent > 0 {
					condition = fmt.Sprintf("subtotal_cent >= %d", in.ThresholdCent)
				}
				effect = `{"type":"FREE_SHIPPING","shipping_cent":1000}`
			}
			out := campaignDraftOutput{
				Campaign: campaignDraft{
					Code: fmt.Sprintf("AI-%d", stamp), Name: name, Type: "PROMOTION", Status: "DRAFT",
					StartAtUnix: now.Unix(), EndAtUnix: end.Unix(), Priority: 10,
					BudgetCent: in.BudgetCent, PerUserLimit: in.PerUserLimit, StackMode: "STACK",
				},
				Rule: promotionRuleDraft{
					Code: fmt.Sprintf("AI-R-%d", stamp), Name: name, Type: ruleType, Status: "DRAFT",
					Priority: 10, Exclusive: false, StackGroup: "default",
					ScopeJSON: `{"all":true}`, ConditionExpr: condition, EffectJSON: effect,
					MaxDiscountCent: maxDiscount, BudgetCent: in.BudgetCent, PerUserLimit: in.PerUserLimit,
					AllowWithCoupon: true, StartAtUnix: now.Unix(), EndAtUnix: end.Unix(),
				},
				Notes: notes,
			}
			metrics.AIToolCalls.WithLabelValues("draft_campaign", "ok").Inc()
			return out, nil
		})
}
