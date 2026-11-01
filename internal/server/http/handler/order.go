package handler

import (
	"strconv"
	"strings"

	pb "gin-kratos-gorm/api/order/v1"
	"gin-kratos-gorm/internal/pkg/apperr"
	"gin-kratos-gorm/internal/pkg/httpx"
	"gin-kratos-gorm/internal/service"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	svc *service.OrderService
}

func NewOrderHandler(svc *service.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req pb.CreateOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.Invalid("invalid request body: %v", err))
		return
	}
	if idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key")); idempotencyKey != "" {
		req.ClientRequestId = idempotencyKey
	}
	reply, err := h.svc.CreateOrder(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.Created(c, reply)
}

func (h *OrderHandler) GetOrder(c *gin.Context) {
	reply, err := h.svc.GetOrderDetail(c.Request.Context(), &pb.GetOrderDetailReq{OrderSn: c.Param("orderSn")})
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, reply)
}

func (h *OrderHandler) ListOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	req := &pb.ListOrdersReq{
		Page:     int32(page),
		PageSize: int32(pageSize),
		OrderSn:  c.Query("order_sn"),
	}
	if v := c.Query("status"); v != "" {
		status, err := strconv.Atoi(v)
		if err != nil {
			httpx.Error(c, apperr.Invalid("invalid status"))
			return
		}
		s := int32(status)
		req.Status = &s
	}
	reply, err := h.svc.ListOrders(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, reply)
}

func (h *OrderHandler) Pay(c *gin.Context) {
	reply, err := h.svc.PayOrder(c.Request.Context(), &pb.PayOrderReq{OrderSn: c.Param("orderSn")})
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, reply)
}

func (h *OrderHandler) Cancel(c *gin.Context) {
	var body struct {
		Reason string `json:"reason"`
	}
	// Body is optional; ignore EOF.
	_ = c.ShouldBindJSON(&body)
	reply, err := h.svc.CancelOrderByUser(c.Request.Context(), &pb.CancelOrderReq{
		OrderSn: c.Param("orderSn"),
		Reason:  body.Reason,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, reply)
}
