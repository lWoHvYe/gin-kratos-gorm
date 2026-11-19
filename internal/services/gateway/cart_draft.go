package gateway

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"
)

type cartDraftBody struct {
	DraftID string `json:"draft_id"`
}

type storedDraft struct {
	Type    string `json:"type"`
	DraftID string `json:"draft_id"`
	Items   []struct {
		SkuID     int64 `json:"sku_id"`
		PriceCent int64 `json:"price_cent"`
		Quantity  int32 `json:"quantity"`
	} `json:"items"`
	TotalCent int64 `json:"total_cent"`
}

// confirmCartDraft is the only write boundary for an AI cart draft. The model
// never called Cart; the user's click reaches this handler, which revalidates
// price/stock and then calls the ordinary Cart RPC.
func (h *handlers) confirmCartDraft(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var body cartDraftBody
	if err := bff.BindJSON(c, &body); err != nil {
		httpx.Error(c, err)
		return
	}
	if body.DraftID == "" {
		httpx.Error(c, apperr.Invalid("draft_id is required"))
		return
	}
	reply, err := h.clients.AI.GetCartDraft(c.Request.Context(), &pb.GetCartDraftRequest{
		DraftId: body.DraftID, UserId: uid,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	if reply.GetConfirmed() {
		httpx.OK(c, gin.H{"confirmed": true, "idempotent": true})
		return
	}
	var draft storedDraft
	if err := json.Unmarshal([]byte(reply.GetDraftJson()), &draft); err != nil || len(draft.Items) == 0 {
		httpx.Error(c, apperr.Invalid("invalid cart draft"))
		return
	}
	// Validate every line before mutating the cart.
	for _, item := range draft.Items {
		sku, err := h.clients.Catalog.GetSku(c.Request.Context(), &pb.GetSkuRequest{SkuId: item.SkuID})
		if err != nil {
			httpx.Error(c, bff.RemoteError(err))
			return
		}
		if sku.GetStatus() != "ENABLED" || sku.GetPriceCent() != item.PriceCent {
			httpx.Error(c, apperr.Conflict("price or status changed; generate a new draft"))
			return
		}
		stock, err := h.clients.Inventory.GetStock(c.Request.Context(), &pb.GetStockRequest{SkuId: item.SkuID})
		if err != nil {
			httpx.Error(c, bff.RemoteError(err))
			return
		}
		if stock.GetAvailable() < int64(item.Quantity) {
			httpx.Error(c, apperr.Conflict("stock changed; generate a new draft"))
			return
		}
	}
	var cart *pb.Cart
	for _, item := range draft.Items {
		cart, err = h.clients.Cart.AddItem(c.Request.Context(), &pb.AddItemRequest{
			UserId: uid, SkuId: item.SkuID, Quantity: item.Quantity,
		})
		if err != nil {
			httpx.Error(c, bff.RemoteError(err))
			return
		}
	}
	if _, err := h.clients.AI.CompleteCartDraft(c.Request.Context(), &pb.CompleteCartDraftRequest{
		DraftId: body.DraftID, UserId: uid,
	}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"confirmed": true, "cart": cart, "total_cent": draft.TotalCent, "message": fmt.Sprintf("已加入 %d 件商品", len(draft.Items))})
}
