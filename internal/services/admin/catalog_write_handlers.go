package admin

import (
	"fmt"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
)

func (h *handlers) createCategory(c *gin.Context) {
	var req pb.CreateCategoryRequest
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	if req.Category == nil {
		httpx.Error(c, apperr.Invalid("category is required"))
		return
	}
	category, err := h.clients.Catalog.CreateCategory(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.Created(c, category)
}

func (h *handlers) createProduct(c *gin.Context) {
	var req pb.CreateProductRequest
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	product, err := h.clients.Catalog.CreateProduct(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.Created(c, product)
}

// setStatusRequest is the body of PUT /products/:id/status.
type setStatusRequest struct {
	Status string `json:"status"`
}

func (h *handlers) setProductStatus(c *gin.Context) {
	productID, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in setStatusRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		httpx.Error(c, apperr.Invalid("status is required"))
		return
	}
	if status == "ON_SALE" {
		product, err := h.clients.Catalog.GetAdminProduct(c.Request.Context(), &pb.GetProductRequest{Id: productID})
		if err != nil {
			httpx.Error(c, bff.RemoteError(err))
			return
		}
		for _, sku := range product.GetSkus() {
			if sku.GetStatus() != "ENABLED" {
				continue
			}
			// Publishing must work when the only stock is in a non-default
			// warehouse for a split listing; sum all warehouse rows.
			stocks, err := h.clients.Inventory.ListStocks(c.Request.Context(), &pb.ListStocksRequest{
				SkuId: sku.GetId(), Page: 1, PageSize: 200,
			})
			if err != nil {
				httpx.Error(c, bff.RemoteError(err))
				return
			}
			var available int64
			for _, stock := range stocks.GetStocks() {
				available += stock.GetAvailable()
			}
			if available <= 0 {
				httpx.Error(c, apperr.Failed(fmt.Sprintf("sku %d has no available stock in any warehouse", sku.GetId())))
				return
			}
		}
	}
	if _, err := h.clients.Catalog.SetProductStatus(c.Request.Context(), &pb.SetProductStatusRequest{
		Id:     productID,
		Status: status,
	}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"id": productID, "status": status})
}

// adjustStockRequest is the body of POST /inventory/adjust.
type adjustStockRequest struct {
	SkuID       int64  `json:"sku_id"`
	WarehouseID int64  `json:"warehouse_id"`
	Delta       int64  `json:"delta"`
	Reason      string `json:"reason"`
}

func (h *handlers) adjustStock(c *gin.Context) {
	var in adjustStockRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	stock, err := h.clients.Inventory.AdminAdjustStock(c.Request.Context(), &pb.AdminAdjustStockRequest{
		SkuId:       in.SkuID,
		WarehouseId: in.WarehouseID,
		Delta:       in.Delta,
		Reason:      in.Reason,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, stock)
}

type replayOperationRequest struct {
	SkuIDs []int64 `json:"sku_ids"`
}

// replayInventoryOperation exposes the explicit per-item inventory recovery to
// operators. It is not a scanner: the caller must name the operation.
func (h *handlers) replayInventoryOperation(c *gin.Context) {
	operationID := strings.TrimSpace(c.Param("operationId"))
	if operationID == "" {
		httpx.Error(c, apperr.Invalid("operation_id is required"))
		return
	}
	var in replayOperationRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	if _, err := h.clients.Inventory.ReplayInventoryOperation(c.Request.Context(), &pb.ReplayInventoryOperationRequest{
		OperationId: operationID, SkuIds: in.SkuIDs,
	}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"ok": true})
}
