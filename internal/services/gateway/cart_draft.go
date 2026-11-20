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
	// Validate every line before mutating the cart; one batch stock read keeps
	// request cost proportional to distinct SKUs.
	totals := make(map[int64]int32)
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
		totals[item.SkuID] += item.Quantity
	}
	if err := h.ensureStocks(c.Request.Context(), totals); err != nil {
		httpx.Error(c, apperr.Conflict("stock changed; generate a new draft"))
		return
	}
	// Cart owns the atomic/idempotent boundary; a retry after a partial
	// BFF failure cannot add the draft twice.
	lines := make([]*pb.CartItem, 0, len(draft.Items))
	for _, item := range draft.Items {
		lines = append(lines, &pb.CartItem{SkuId: item.SkuID, Quantity: item.Quantity, PriceCent: item.PriceCent})
	}
	cart, err := h.clients.Cart.ConfirmCartDraft(c.Request.Context(), &pb.ConfirmCartDraftRequest{
		UserId: uid, DraftId: body.DraftID, Items: lines,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	if _, err := h.clients.AI.CompleteCartDraft(c.Request.Context(), &pb.CompleteCartDraftRequest{
		DraftId: body.DraftID, UserId: uid,
	}); err != nil {
		// Cart is already updated and idempotent; retrying is safe.
		h.log.Warn("complete cart draft after cart update", "draft_id", body.DraftID, "error", err)
	}
	httpx.OK(c, gin.H{"confirmed": true, "cart": cart, "total_cent": draft.TotalCent, "message": fmt.Sprintf("已加入 %d 件商品", len(draft.Items))})
}
