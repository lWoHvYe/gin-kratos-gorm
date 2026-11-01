package router

import (
	"gin-kratos-gorm/internal/server/http/handler"

	"github.com/gin-gonic/gin"
)

type ProductRouter struct {
	productHandler *handler.ProductHandler
}

func NewProductRouter(h *handler.ProductHandler) *ProductRouter {
	return &ProductRouter{productHandler: h}
}

func (r *ProductRouter) RegisterProductRouter(public, admin *gin.RouterGroup) {
	public.GET("/products", r.productHandler.ListProducts)
	public.GET("/products/:id", r.productHandler.GetProductDetail)

	admin.POST("/products", r.productHandler.CreateProduct)
	admin.PUT("/products/:id", r.productHandler.UpdateProduct)
	admin.PUT("/products/:id/status", r.productHandler.SetOnSale)
	admin.PUT("/products/:id/stock", r.productHandler.AdjustStock)
}
