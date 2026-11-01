package router

import (
	"gin-kratos-gorm/internal/server/http/handler"

	"github.com/gin-gonic/gin"
)

type OrderRouter struct {
	orderHandler *handler.OrderHandler
}

func NewOrderRouter(h *handler.OrderHandler) *OrderRouter {
	return &OrderRouter{orderHandler: h}
}

func (r *OrderRouter) RegisterRouter(private *gin.RouterGroup) {
	orders := private.Group("/orders")
	{
		orders.POST("", r.orderHandler.CreateOrder)
		orders.GET("", r.orderHandler.ListOrders)
		orders.GET("/:orderSn", r.orderHandler.GetOrder)
		orders.POST("/:orderSn/pay", r.orderHandler.Pay)
		orders.POST("/:orderSn/cancel", r.orderHandler.Cancel)
	}
}
