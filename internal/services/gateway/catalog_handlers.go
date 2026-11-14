package gateway

import (
	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *handlers) listCategories(c *gin.Context) {
	reply, err := h.clients.Catalog.ListCategories(c.Request.Context(), &pb.Empty{})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	filtered := make([]*pb.Category, 0, len(reply.GetCategories()))
	for _, category := range reply.GetCategories() {
		if category.GetStatus() == "ENABLED" {
			filtered = append(filtered, category)
		}
	}
	reply.Categories = filtered
	httpx.OK(c, reply)
}

func (h *handlers) listBrands(c *gin.Context) {
	reply, err := h.clients.Catalog.ListBrands(c.Request.Context(), &pb.Empty{})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	filtered := make([]*pb.Brand, 0, len(reply.GetBrands()))
	for _, brand := range reply.GetBrands() {
		if brand.GetStatus() == "ENABLED" {
			filtered = append(filtered, brand)
		}
	}
	reply.Brands = filtered
	httpx.OK(c, reply)
}

func (h *handlers) listProducts(c *gin.Context) {
	page, err := bff.QueryInt32(c, "page")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	pageSize, err := bff.QueryInt32(c, "page_size")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	categoryID, err := bff.QueryInt64(c, "category_id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	brandID, err := bff.QueryInt64(c, "brand_id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	minPrice, err := bff.QueryInt64(c, "min_price_cent")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	maxPrice, err := bff.QueryInt64(c, "max_price_cent")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Catalog.ListProducts(c.Request.Context(), &pb.ListProductsRequest{
		Page: page, PageSize: pageSize, Keyword: strings.TrimSpace(c.Query("keyword")),
		CategoryId: categoryID, BrandId: brandID, MinPriceCent: minPrice, MaxPriceCent: maxPrice,
		Sort: strings.TrimSpace(c.Query("sort")),
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) getProduct(c *gin.Context) {
	productID, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	product, err := h.clients.Catalog.GetProduct(c.Request.Context(), &pb.GetProductRequest{Id: productID})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, product)
}

func (h *handlers) getSku(c *gin.Context) {
	skuID, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	sku, err := h.clients.Catalog.GetSku(c.Request.Context(), &pb.GetSkuRequest{SkuId: skuID})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, sku)
}

// ensureStock checks the default warehouse before adding to cart or quoting.
// The final authority remains DTM Reserve during order creation.
