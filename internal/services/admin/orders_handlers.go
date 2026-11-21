package admin

import (
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
)

func (h *handlers) listOrders(c *gin.Context) {
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
	orders, err := h.clients.Order.ListOrders(c.Request.Context(), &pb.ListOrdersRequest{
		UserId:   userID,
		Admin:    true,
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

// getOrder returns one order for the back office. The order service requires
// Admin=true to skip the owner check.
func (h *handlers) getOrder(c *gin.Context) {
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		httpx.Error(c, apperr.Invalid("order_no is required"))
		return
	}
	order, err := h.clients.Order.GetOrder(c.Request.Context(), &pb.GetOrderRequest{OrderNo: orderNo, Admin: true})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, order)
}

// createShipmentRequest is the body of POST /shipments. When "items" is omitted
// the parcel is built from every line of the order. "ship" additionally moves
// the new shipment from CREATED to SHIPPED (fulfillment.Ship).
type createShipmentRequest struct {
	OrderNo            string         `json:"order_no"`
	WarehouseID        int64          `json:"warehouse_id"`
	FulfillmentGroupNo string         `json:"fulfillment_group_no"`
	Carrier            string         `json:"carrier"`
	TrackingNo         string         `json:"tracking_no"`
	Items              []shipmentItem `json:"items"`
	Ship               bool           `json:"ship"`
}

type shipmentItem struct {
	SkuID    int64 `json:"sku_id"`
	Quantity int32 `json:"quantity"`
}

func (h *handlers) createShipment(c *gin.Context) {
	var in createShipmentRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	orderNo := strings.TrimSpace(in.OrderNo)
	if orderNo == "" {
		httpx.Error(c, apperr.Invalid("order_no is required"))
		return
	}
	order, err := h.clients.Order.GetOrder(c.Request.Context(), &pb.GetOrderRequest{OrderNo: orderNo, Admin: true})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	groupNo := strings.TrimSpace(in.FulfillmentGroupNo)
	if err := validateShipmentOrder(order, groupNo); err != nil {
		httpx.Error(c, err)
		return
	}
	items := make([]*pb.ShipmentItem, 0, len(in.Items))
	for _, item := range in.Items {
		items = append(items, &pb.ShipmentItem{SkuId: item.SkuID, Quantity: item.Quantity})
	}
	if len(items) == 0 {
		// Default to a full shipment for the selected group (or the whole order
		// for legacy single-group callers).
		for _, line := range order.GetItems() {
			if groupNo != "" && line.GetGroupNo() != groupNo {
				continue
			}
			items = append(items, &pb.ShipmentItem{SkuId: line.GetSkuId(), Quantity: line.GetQuantity()})
		}
		if len(items) == 0 {
			httpx.Error(c, apperr.Failed("no order items belong to the selected fulfillment group"))
			return
		}
	}
	shipment, err := h.clients.Fulfillment.CreateShipment(c.Request.Context(), &pb.CreateShipmentRequest{
		OrderNo:            orderNo,
		WarehouseId:        in.WarehouseID,
		FulfillmentGroupNo: strings.TrimSpace(in.FulfillmentGroupNo),
		Carrier:            strings.TrimSpace(in.Carrier),
		TrackingNo:         strings.TrimSpace(in.TrackingNo),
		Items:              items,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	if in.Ship {
		shipment, err = h.clients.Fulfillment.Ship(c.Request.Context(), &pb.ShipRequest{ShipmentNo: shipment.GetShipmentNo()})
		if err != nil {
			httpx.Error(c, bff.RemoteError(err))
			return
		}
	}
	httpx.Created(c, shipment)
}

// validateShipmentOrder uses Order's server-owned action list instead of
// re-deriving the lifecycle/refund state in the BFF. A refunding/full-refunded
// order has no "ship" action, so no draft shipment is created before the
// fulfillment permit would reject it.
func validateShipmentOrder(order *pb.Order, groupNo string) error {
	for _, action := range order.GetAvailableActions() {
		if groupNo == "" && action == "ship" {
			return nil
		}
		if groupNo != "" && action == "ship_group:"+groupNo {
			return nil
		}
	}
	return apperr.Failed("shipping is not available for this order/group")
}

// approveRequest is the body of POST /after-sales/:no/approve. approved defaults
// to true so a bare approve call accepts the request.
type approveRequest struct {
	Approved   *bool  `json:"approved"`
	AmountCent int64  `json:"amount_cent"`
	Reason     string `json:"reason"`
}

type cancelOrderRequest struct {
	Reason string `json:"reason"`
}

func (h *handlers) cancelOrder(c *gin.Context) {
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		httpx.Error(c, apperr.Invalid("orderNo is required"))
		return
	}
	var in cancelOrderRequest
	if bff.BodyPresent(c) {
		if err := bff.BindJSON(c, &in); err != nil {
			httpx.Error(c, err)
			return
		}
	}
	order, err := h.clients.Order.CancelOrder(c.Request.Context(), &pb.CancelOrderRequest{
		OrderNo: orderNo,
		Admin:   true,
		Reason:  strings.TrimSpace(in.Reason),
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, order)
}

func (h *handlers) verifyPayment(c *gin.Context) {
	paymentNo := strings.TrimSpace(c.Param("paymentNo"))
	if paymentNo == "" {
		httpx.Error(c, apperr.Invalid("paymentNo is required"))
		return
	}
	reply, err := h.clients.Payment.VerifyPayment(c.Request.Context(), &pb.VerifyPaymentRequest{PaymentNo: paymentNo})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) shipShipment(c *gin.Context) {
	shipmentNo := strings.TrimSpace(c.Param("shipmentNo"))
	if shipmentNo == "" {
		httpx.Error(c, apperr.Invalid("shipmentNo is required"))
		return
	}
	shipment, err := h.clients.Fulfillment.Ship(c.Request.Context(), &pb.ShipRequest{ShipmentNo: shipmentNo})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, shipment)
}

func (h *handlers) refundAfterSale(c *gin.Context) {
	no := strings.TrimSpace(c.Param("no"))
	if no == "" {
		httpx.Error(c, apperr.Invalid("after-sale number is required"))
		return
	}
	afterSale, err := h.clients.AfterSales.Refund(c.Request.Context(), &pb.RefundAfterSaleRequest{AfterSaleNo: no})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, afterSale)
}

func (h *handlers) approveAfterSale(c *gin.Context) {
	afterSaleNo := strings.TrimSpace(c.Param("no"))
	if afterSaleNo == "" {
		httpx.Error(c, apperr.Invalid("after sale number is required"))
		return
	}
	var in approveRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	approved := true
	if in.Approved != nil {
		approved = *in.Approved
	}
	afterSale, err := h.clients.AfterSales.Approve(c.Request.Context(), &pb.ApproveRequest{
		AfterSaleNo: afterSaleNo,
		Approved:    approved,
		AmountCent:  in.AmountCent,
		Reason:      in.Reason,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, afterSale)
}
