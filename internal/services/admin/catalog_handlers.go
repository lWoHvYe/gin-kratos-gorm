package admin

import (
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------- catalog admin

func (h *handlers) listCategoriesAdmin(c *gin.Context) {
	reply, err := h.clients.Catalog.ListCategories(c.Request.Context(), &pb.Empty{})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) updateCategory(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var req pb.UpdateCategoryRequest
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	if req.Category == nil {
		httpx.Error(c, apperr.Invalid("category is required"))
		return
	}
	req.Category.Id = id
	category, err := h.clients.Catalog.UpdateCategory(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, category)
}

func (h *handlers) deleteCategory(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if _, err := h.clients.Catalog.DeleteCategory(c.Request.Context(), &pb.DeleteCategoryRequest{Id: id}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"status": "deleted"})
}

func (h *handlers) listBrands(c *gin.Context) {
	reply, err := h.clients.Catalog.ListBrands(c.Request.Context(), &pb.Empty{})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) createBrand(c *gin.Context) {
	var req pb.CreateBrandRequest
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	brand, err := h.clients.Catalog.CreateBrand(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.Created(c, brand)
}

func (h *handlers) updateBrand(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var req pb.UpdateBrandRequest
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	if req.Brand == nil {
		httpx.Error(c, apperr.Invalid("brand is required"))
		return
	}
	req.Brand.Id = id
	brand, err := h.clients.Catalog.UpdateBrand(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, brand)
}

func (h *handlers) deleteBrand(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if _, err := h.clients.Catalog.DeleteBrand(c.Request.Context(), &pb.DeleteBrandRequest{Id: id}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"status": "deleted"})
}

func (h *handlers) listAdminProducts(c *gin.Context) {
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
	reply, err := h.clients.Catalog.ListAdminProducts(c.Request.Context(), &pb.ListAdminProductsRequest{
		Page: page, PageSize: pageSize, Keyword: strings.TrimSpace(c.Query("keyword")),
		CategoryId: categoryID, BrandId: brandID, Status: strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) getAdminProduct(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	product, err := h.clients.Catalog.GetAdminProduct(c.Request.Context(), &pb.GetProductRequest{Id: id})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, product)
}

func (h *handlers) updateProduct(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var req pb.UpdateProductRequest
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	req.Id = id
	product, err := h.clients.Catalog.UpdateProduct(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, product)
}

func (h *handlers) deleteProduct(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if _, err := h.clients.Catalog.DeleteProduct(c.Request.Context(), &pb.DeleteProductRequest{Id: id}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"status": "deleted"})
}

func (h *handlers) createSku(c *gin.Context) {
	spuID, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var sku pb.SkuInput
	if err := bff.BindJSON(c, &sku); err != nil {
		httpx.Error(c, err)
		return
	}
	created, err := h.clients.Catalog.CreateSku(c.Request.Context(), &pb.CreateSkuRequest{SpuId: spuID, Sku: &sku})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.Created(c, created)
}

func (h *handlers) updateSku(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var req pb.UpdateSkuRequest
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	req.Id = id
	sku, err := h.clients.Catalog.UpdateSku(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, sku)
}

func (h *handlers) deleteSku(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if _, err := h.clients.Catalog.DeleteSku(c.Request.Context(), &pb.DeleteSkuRequest{Id: id}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"status": "deleted"})
}

func (h *handlers) setSkuStatus(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in setStatusRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	sku, err := h.clients.Catalog.SetSkuStatus(c.Request.Context(), &pb.SetSkuStatusRequest{Id: id, Status: in.Status})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, sku)
}

func (h *handlers) listStocks(c *gin.Context) {
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
	skuID, err := bff.QueryInt64(c, "sku_id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	warehouseID, err := bff.QueryInt64(c, "warehouse_id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Inventory.ListStocks(c.Request.Context(), &pb.ListStocksRequest{
		SkuId: skuID, WarehouseId: warehouseID, Page: page, PageSize: pageSize,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) listShipments(c *gin.Context) {
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
	reply, err := h.clients.Fulfillment.ListShipments(c.Request.Context(), &pb.ListShipmentsRequest{
		Page: page, PageSize: pageSize, Status: strings.TrimSpace(c.Query("status")),
		OrderNo: strings.TrimSpace(c.Query("order_no")),
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) getShipment(c *gin.Context) {
	shipmentNo := strings.TrimSpace(c.Param("shipmentNo"))
	shipment, err := h.clients.Fulfillment.GetShipment(c.Request.Context(), &pb.GetShipmentRequest{ShipmentNo: shipmentNo})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, shipment)
}

func (h *handlers) listAfterSales(c *gin.Context) {
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
	userID, err := bff.QueryInt64(c, "user_id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.AfterSales.ListAfterSales(c.Request.Context(), &pb.ListAfterSalesRequest{
		UserId: userID, Admin: true, Page: page, PageSize: pageSize, Status: strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

type receiveReturnRequest struct {
	Condition string `json:"condition"`
}

func (h *handlers) receiveReturn(c *gin.Context) {
	no := strings.TrimSpace(c.Param("no"))
	var in receiveReturnRequest
	if bff.BodyPresent(c) {
		if err := bff.BindJSON(c, &in); err != nil {
			httpx.Error(c, err)
			return
		}
	}
	afterSale, err := h.clients.AfterSales.ReceiveReturn(c.Request.Context(), &pb.ReceiveReturnRequest{AfterSaleNo: no, Condition: in.Condition})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, afterSale)
}

func (h *handlers) listRefunds(c *gin.Context) {
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
	reply, err := h.clients.Payment.ListRefunds(c.Request.Context(), &pb.ListRefundsRequest{
		PaymentNo: strings.TrimSpace(c.Query("payment_no")), OrderNo: strings.TrimSpace(c.Query("order_no")),
		Page: page, PageSize: pageSize,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}
