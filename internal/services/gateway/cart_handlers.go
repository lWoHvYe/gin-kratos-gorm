package gateway

import (
	"context"
	"fmt"
	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
)

// ensureStock checks the default warehouse before adding to cart or quoting.
// The final authority remains DTM Reserve during order creation.
func (h *handlers) ensureStock(ctx context.Context, skuID int64, quantity int32) error {
	if skuID <= 0 || quantity <= 0 {
		return apperr.Invalid("sku_id and quantity must be positive")
	}
	if h.clients.Inventory == nil {
		return apperr.Unavailable("inventory client is not configured", nil)
	}
	stock, err := h.clients.Inventory.GetStock(ctx, &pb.GetStockRequest{SkuId: skuID, WarehouseId: 1})
	if err != nil {
		return bff.RemoteError(err)
	}
	if stock.GetAvailable() < int64(quantity) {
		return apperr.Failed(fmt.Sprintf("insufficient stock for sku %d: available %d", skuID, stock.GetAvailable()))
	}
	return nil
}

// ensureCartStock accounts for the quantity already accumulated in the cart;
// AddItem is an upsert, so checking only the incoming delta is insufficient.

// ensureCartStock accounts for the quantity already accumulated in the cart;
// AddItem is an upsert, so checking only the incoming delta is insufficient.
func (h *handlers) ensureCartStock(ctx context.Context, userID, skuID int64, delta int32) error {
	cart, err := h.clients.Cart.GetCart(ctx, &pb.GetCartRequest{UserId: userID})
	if err != nil {
		return bff.RemoteError(err)
	}
	var existing int32
	for _, item := range cart.GetItems() {
		if item.GetSkuId() == skuID {
			existing += item.GetQuantity()
		}
	}
	return h.ensureStock(ctx, skuID, existing+delta)
}

// ---------------------------------------------------------------- addresses

func (h *handlers) getCart(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	cart, err := h.clients.Cart.GetCart(c.Request.Context(), &pb.GetCartRequest{UserId: uid})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, cart)
}

func (h *handlers) addCartItem(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var req pb.AddItemRequest
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	req.UserId = uid
	if err := h.ensureCartStock(c.Request.Context(), uid, req.SkuId, req.Quantity); err != nil {
		httpx.Error(c, err)
		return
	}
	cart, err := h.clients.Cart.AddItem(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, cart)
}

func (h *handlers) updateCartItem(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var req pb.UpdateItemRequest
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	req.UserId = uid
	if req.Quantity > 0 {
		if err := h.ensureStock(c.Request.Context(), req.SkuId, req.Quantity); err != nil {
			httpx.Error(c, err)
			return
		}
	}
	cart, err := h.clients.Cart.UpdateItem(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, cart)
}

func (h *handlers) removeCartItem(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var req pb.RemoveItemRequest
	if bff.BodyPresent(c) {
		if err := bff.BindJSON(c, &req); err != nil {
			httpx.Error(c, err)
			return
		}
	}
	if req.SkuId == 0 {
		// DELETE requests often carry no body; allow ?sku_id=<id> as well.
		skuID, err := bff.QueryInt64(c, "sku_id")
		if err != nil {
			httpx.Error(c, err)
			return
		}
		req.SkuId = skuID
	}
	req.UserId = uid
	cart, err := h.clients.Cart.RemoveItem(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, cart)
}

// mergeRequest is the body of POST /cart/merge. A client sends the anonymous
// cart; every line is added to the authenticated user's cart.

// mergeRequest is the body of POST /cart/merge. A client sends the anonymous
// cart; every line is added to the authenticated user's cart.
type mergeRequest struct {
	Items []mergeItem `json:"items"`
}

type mergeItem struct {
	SkuID    int64 `json:"sku_id"`
	Quantity int32 `json:"quantity"`
}

func (h *handlers) mergeCart(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in mergeRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	for _, item := range in.Items {
		if item.SkuID <= 0 || item.Quantity <= 0 {
			httpx.Error(c, apperr.Invalid("merge items require a positive sku_id and quantity"))
			return
		}
	}
	var cart *pb.Cart
	for _, item := range in.Items {
		if err := h.ensureCartStock(c.Request.Context(), uid, item.SkuID, item.Quantity); err != nil {
			httpx.Error(c, err)
			return
		}
		cart, err = h.clients.Cart.AddItem(c.Request.Context(), &pb.AddItemRequest{
			UserId:   uid,
			SkuId:    item.SkuID,
			Quantity: item.Quantity,
		})
		if err != nil {
			httpx.Error(c, bff.RemoteError(err))
			return
		}
	}
	if cart == nil {
		cart, err = h.clients.Cart.GetCart(c.Request.Context(), &pb.GetCartRequest{UserId: uid})
		if err != nil {
			httpx.Error(c, bff.RemoteError(err))
			return
		}
	}
	httpx.OK(c, cart)
}

// ---------------------------------------------------------------- checkout

// previewRequest is the body of POST /checkout/preview. The pb.QuoteRequest
// user_id is always replaced with the authenticated user.
