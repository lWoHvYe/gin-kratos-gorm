package gateway

import (
	"context"
	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"
	"gin-kratos-gorm/internal/platform/id"
	"strings"

	"github.com/gin-gonic/gin"
)

// previewRequest is the body of POST /checkout/preview. The pb.QuoteRequest
// user_id is always replaced with the authenticated user.
type previewRequest struct {
	Items     []checkoutItem `json:"items"`
	AddressID int64          `json:"address_id"`
	CouponID  string         `json:"coupon_id"`
}

type checkoutItem struct {
	SkuID    int64 `json:"sku_id"`
	Quantity int32 `json:"quantity"`
}

func (h *handlers) previewCheckout(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in previewRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	items := make([]*pb.QuoteItem, 0, len(in.Items))
	for _, item := range in.Items {
		items = append(items, &pb.QuoteItem{SkuId: item.SkuID, Quantity: item.Quantity})
	}
	if len(items) == 0 {
		// No explicit items: price the selected cart lines. This keeps the
		// server cart as the source of truth for the normal C-end checkout.
		cart, err := h.clients.Cart.GetCart(c.Request.Context(), &pb.GetCartRequest{UserId: uid})
		if err != nil {
			httpx.Error(c, bff.RemoteError(err))
			return
		}
		for _, item := range cart.GetItems() {
			if item.GetSelected() && item.GetQuantity() > 0 {
				items = append(items, &pb.QuoteItem{SkuId: item.GetSkuId(), Quantity: item.GetQuantity()})
			}
		}
		if len(items) == 0 {
			httpx.Error(c, apperr.Invalid("cart has no selected items"))
			return
		}
	}
	totals := make(map[int64]int32, len(items))
	for _, item := range items {
		totals[item.GetSkuId()] += item.GetQuantity()
	}
	for skuID, quantity := range totals {
		if err := h.ensureStock(c.Request.Context(), skuID, quantity); err != nil {
			httpx.Error(c, err)
			return
		}
	}
	quote, err := h.clients.Pricing.CreateQuote(c.Request.Context(), &pb.QuoteRequest{
		UserId:    uid,
		Items:     items,
		AddressId: in.AddressID,
		CouponId:  strings.TrimSpace(in.CouponID),
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, quote)
}

// placeOrderRequest is the body of POST /checkout/place. The order service
// rebuilds the lines from the quote, so only the quote and receiver are needed.

// placeOrderRequest is the body of POST /checkout/place. The order service
// rebuilds the lines from the quote, so only the quote and receiver are needed.
type placeOrderRequest struct {
	QuoteID         string `json:"quote_id"`
	AddressID       int64  `json:"address_id"`
	ReceiverName    string `json:"receiver_name"`
	ReceiverPhone   string `json:"receiver_phone"`
	ReceiverAddress string `json:"receiver_address"`
	ClientRequestID string `json:"client_request_id"`
}

func (h *handlers) placeOrder(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in placeOrderRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	clientRequestID := strings.TrimSpace(in.ClientRequestID)
	if clientRequestID == "" {
		clientRequestID = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	}
	if clientRequestID == "" {
		clientRequestID = id.Event()
	}
	receiverName, receiverPhone, receiverAddress := strings.TrimSpace(in.ReceiverName), strings.TrimSpace(in.ReceiverPhone), strings.TrimSpace(in.ReceiverAddress)
	if receiverName == "" || receiverPhone == "" || receiverAddress == "" {
		address, err := h.resolveAddress(c.Request.Context(), uid, in.AddressID)
		if err != nil {
			httpx.Error(c, err)
			return
		}
		if receiverName == "" {
			receiverName = address.GetReceiver()
		}
		if receiverPhone == "" {
			receiverPhone = address.GetPhone()
		}
		if receiverAddress == "" {
			receiverAddress = joinAddress(address)
		}
	}
	if receiverName == "" || receiverPhone == "" || receiverAddress == "" {
		httpx.Error(c, apperr.Invalid("receiver_name, receiver_phone and receiver_address are required"))
		return
	}
	order, err := h.clients.Order.CreateOrder(c.Request.Context(), &pb.CreateOrderRequest{
		UserId:          uid,
		QuoteId:         strings.TrimSpace(in.QuoteID),
		AddressId:       in.AddressID,
		ClientRequestId: clientRequestID,
		ReceiverName:    receiverName,
		ReceiverPhone:   receiverPhone,
		ReceiverAddress: receiverAddress,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	intent, err := h.clients.Payment.CreateIntent(c.Request.Context(), &pb.CreateIntentRequest{
		OrderNo:    order.GetOrderNo(),
		UserId:     uid,
		AmountCent: order.GetPayCent(),
		Channel:    defaultPaymentChannel,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	// The order is authoritative even if clearing the cart fails; keep the
	// stale cart rather than failing a successful order.
	if _, err := h.clients.Cart.ClearSelected(c.Request.Context(), &pb.ClearCartRequest{UserId: uid}); err != nil {
		if h.log != nil {
			h.log.Warn("clear selected cart after order", "order_no", order.GetOrderNo(), "error", err)
		}
	}
	httpx.Created(c, gin.H{"order": order, "payment": intent})
}

// ---------------------------------------------------------------- orders

func (h *handlers) resolveAddress(ctx context.Context, userID, addressID int64) (*pb.Address, error) {
	if addressID <= 0 {
		return nil, apperr.Invalid("address_id is required when receiver fields are omitted")
	}
	list, err := h.clients.Identity.ListAddresses(ctx, &pb.GetUserRequest{UserId: userID})
	if err != nil {
		return nil, bff.RemoteError(err)
	}
	for _, address := range list.GetAddresses() {
		if address.GetId() == addressID {
			return address, nil
		}
	}
	return nil, apperr.NotFound("address not found")
}

func joinAddress(address *pb.Address) string {
	parts := []string{address.GetProvince(), address.GetCity(), address.GetDistrict(), address.GetDetail()}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, " ")
}
