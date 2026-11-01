package handler

import (
	"strconv"

	pb "gin-kratos-gorm/api/product/v1"
	"gin-kratos-gorm/internal/pkg/apperr"
	"gin-kratos-gorm/internal/pkg/httpx"
	"gin-kratos-gorm/internal/service"

	"github.com/gin-gonic/gin"
)

type ProductHandler struct {
	svc *service.ProductService
}

func NewProductHandler(svc *service.ProductService) *ProductHandler {
	return &ProductHandler{svc: svc}
}

func (h *ProductHandler) GetProductDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		httpx.Error(c, apperr.Invalid("invalid product id"))
		return
	}
	reply, svcErr := h.svc.GetProductDetail(c.Request.Context(), &pb.GetProductDetailReq{Id: uint32(id)})
	if svcErr != nil {
		httpx.Error(c, svcErr)
		return
	}
	httpx.OK(c, reply)
}

func (h *ProductHandler) ListProducts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	req := &pb.ListProductsReq{
		Page:     int32(page),
		PageSize: int32(pageSize),
		Keyword:  c.Query("keyword"),
	}
	if v := c.Query("category_id"); v != "" {
		if categoryID, err := strconv.ParseInt(v, 10, 64); err == nil {
			req.CategoryId = categoryID
		}
	}
	if v := c.Query("on_sale"); v != "" {
		if onSale, err := strconv.ParseBool(v); err == nil {
			req.OnSale = &onSale
		}
	}
	reply, err := h.svc.ListProducts(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, reply)
}

func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var req pb.CreateProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.Invalid("invalid request body: %v", err))
		return
	}
	reply, err := h.svc.CreateProduct(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.Created(c, reply)
}

func (h *ProductHandler) UpdateProduct(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		httpx.Error(c, apperr.Invalid("invalid product id"))
		return
	}
	var req pb.UpdateProductReq
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		httpx.Error(c, apperr.Invalid("invalid request body: %v", bindErr))
		return
	}
	req.Id = uint32(id)
	reply, svcErr := h.svc.UpdateProduct(c.Request.Context(), &req)
	if svcErr != nil {
		httpx.Error(c, svcErr)
		return
	}
	httpx.OK(c, reply)
}

func (h *ProductHandler) SetOnSale(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		httpx.Error(c, apperr.Invalid("invalid product id"))
		return
	}
	var body struct {
		OnSale bool `json:"on_sale"`
	}
	if bindErr := c.ShouldBindJSON(&body); bindErr != nil {
		httpx.Error(c, apperr.Invalid("invalid request body: %v", bindErr))
		return
	}
	reply, svcErr := h.svc.SetOnSale(c.Request.Context(), &pb.SetOnSaleReq{Id: uint32(id), OnSale: body.OnSale})
	if svcErr != nil {
		httpx.Error(c, svcErr)
		return
	}
	httpx.OK(c, reply)
}

func (h *ProductHandler) AdjustStock(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		httpx.Error(c, apperr.Invalid("invalid sku id"))
		return
	}
	var body struct {
		Delta int32 `json:"delta"`
	}
	if bindErr := c.ShouldBindJSON(&body); bindErr != nil {
		httpx.Error(c, apperr.Invalid("invalid request body: %v", bindErr))
		return
	}
	reply, svcErr := h.svc.AdjustStock(c.Request.Context(), &pb.AdjustStockReq{SkuId: uint32(id), Delta: body.Delta})
	if svcErr != nil {
		httpx.Error(c, svcErr)
		return
	}
	httpx.OK(c, reply)
}
