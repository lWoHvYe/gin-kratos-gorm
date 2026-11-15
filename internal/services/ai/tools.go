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

// catalogClient is the subset of CatalogService the assistant may use. It is
// an interface so tools can be tested without a live gRPC server.
type catalogClient interface {
	ListProducts(ctx context.Context, in *pb.ListProductsRequest, opts ...grpc.CallOption) (*pb.ProductList, error)
	GetProduct(ctx context.Context, in *pb.GetProductRequest, opts ...grpc.CallOption) (*pb.Product, error)
}

const toolTimeout = 5 * time.Second

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

func newCatalogTools(catalog catalogClient) ([]tool.BaseTool, error) {
	search, err := utils.InferTool("search_products",
		"Search on-sale products by keyword, category, brand or price range. Use this before recommending anything.",
		func(ctx context.Context, in searchProductsInput) (searchProductsOutput, error) {
			ctx, cancel := context.WithTimeout(ctx, toolTimeout)
			defer cancel()
			pageSize := in.PageSize
			if pageSize <= 0 {
				pageSize = 5
			}
			if pageSize > 10 {
				pageSize = 10
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
	if err != nil {
		return nil, fmt.Errorf("build search_products tool: %w", err)
	}

	get, err := utils.InferTool("get_product",
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
	if err != nil {
		return nil, fmt.Errorf("build get_product tool: %w", err)
	}
	return []tool.BaseTool{search, get}, nil
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
