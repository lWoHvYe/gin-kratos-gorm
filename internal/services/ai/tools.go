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
}

type afterSalesClient interface {
	ListAfterSales(ctx context.Context, in *pb.ListAfterSalesRequest, opts ...grpc.CallOption) (*pb.AfterSaleList, error)
	GetAfterSale(ctx context.Context, in *pb.GetAfterSaleRequest, opts ...grpc.CallOption) (*pb.AfterSale, error)
}

// Deps are the read-only clients the agent tools may call.
type Deps struct {
	Catalog    catalogClient
	Orders     orderClient
	AfterSales afterSalesClient
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
