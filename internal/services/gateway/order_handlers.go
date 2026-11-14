package gateway

import (
	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *handlers) listOrders(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
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
	orders, err := h.clients.Order.ListOrders(c.Request.Context(), &pb.ListOrdersRequest{
		UserId:   uid,
		Page:     page,
		PageSize: pageSize,
		Status:   strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, orders)
}

func (h *handlers) getOrder(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		httpx.Error(c, apperr.Invalid("orderNo is required"))
		return
	}
	order, err := h.clients.Order.GetOrder(c.Request.Context(), &pb.GetOrderRequest{OrderNo: orderNo, UserId: uid})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, order)
}

func (h *handlers) cancelOrder(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		httpx.Error(c, apperr.Invalid("orderNo is required"))
		return
	}
	req := &pb.CancelOrderRequest{OrderNo: orderNo, UserId: uid}
	if bff.BodyPresent(c) {
		var in pb.CancelOrderRequest
		if err := bff.BindJSON(c, &in); err != nil {
			httpx.Error(c, err)
			return
		}
		req.Reason = in.GetReason()
	}
	order, err := h.clients.Order.CancelOrder(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, order)
}

// receiveRequest is the optional body of POST /orders/:orderNo/receive. The
// shipment is resolved by order number when shipment_no is omitted.

// receiveRequest is the optional body of POST /orders/:orderNo/receive. The
// shipment is resolved by order number when shipment_no is omitted.
type receiveRequest struct {
	ShipmentNo string `json:"shipment_no"`
}

func (h *handlers) receiveOrder(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		httpx.Error(c, apperr.Invalid("orderNo is required"))
		return
	}
	// Ownership check before touching another service's shipment.
	if _, err := h.clients.Order.GetOrder(c.Request.Context(), &pb.GetOrderRequest{OrderNo: orderNo, UserId: uid}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	var in receiveRequest
	if bff.BodyPresent(c) {
		if err := bff.BindJSON(c, &in); err != nil {
			httpx.Error(c, err)
			return
		}
	}
	shipmentNo := strings.TrimSpace(in.ShipmentNo)
	if shipmentNo == "" {
		shipmentNo = strings.TrimSpace(c.Query("shipment_no"))
	}
	var shipment *pb.Shipment
	if shipmentNo == "" {
		shipment, err = h.clients.Fulfillment.GetShipmentByOrder(c.Request.Context(), &pb.GetShipmentByOrderRequest{OrderNo: orderNo})
	} else {
		shipment, err = h.clients.Fulfillment.GetShipment(c.Request.Context(), &pb.GetShipmentRequest{ShipmentNo: shipmentNo})
	}
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	if shipment.GetOrderNo() != orderNo {
		httpx.Error(c, apperr.Forbidden("shipment does not belong to this order"))
		return
	}
	shipmentNo = shipment.GetShipmentNo()
	received, err := h.clients.Fulfillment.Receive(c.Request.Context(), &pb.ReceiveRequest{ShipmentNo: shipmentNo})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, received)
}

// ---------------------------------------------------------------- payment

// payRequest is the body of POST /payments/:paymentNo/pay. It simulates a
// channel callback; the status is always SUCCEEDED.

func (h *handlers) getOrderShipment(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	order, err := h.clients.Order.GetOrder(c.Request.Context(), &pb.GetOrderRequest{OrderNo: orderNo, UserId: uid})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	if order.GetOrderNo() == "" {
		httpx.Error(c, apperr.NotFound("order not found"))
		return
	}
	shipment, err := h.clients.Fulfillment.GetShipmentByOrder(c.Request.Context(), &pb.GetShipmentByOrderRequest{OrderNo: orderNo})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, shipment)
}

// ---------------------------------------------------------------- seckill
