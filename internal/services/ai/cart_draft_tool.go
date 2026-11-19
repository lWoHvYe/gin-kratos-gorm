package ai

import (
	"context"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/metrics"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type draftCartInput struct {
	Items []draftCartLine `json:"items" jsonschema:"description=SKU lines to put in the draft, max 10"`
}

type draftCartLine struct {
	SkuID    int64 `json:"sku_id"`
	Quantity int32 `json:"quantity"`
}

type cartDraftItem struct {
	SkuID          int64  `json:"sku_id"`
	ProductID      int64  `json:"product_id"`
	Name           string `json:"name"`
	PriceCent      int64  `json:"price_cent"`
	Quantity       int32  `json:"quantity"`
	Available      int64  `json:"available"`
	StockTimestamp int64  `json:"stock_timestamp"`
}

type cartDraft struct {
	Type       string          `json:"type"`
	DraftID    string          `json:"draft_id"`
	Items      []cartDraftItem `json:"items"`
	TotalCent  int64           `json:"total_cent"`
	ExpiresAt  int64           `json:"expires_at"`
	ReadOnly   bool            `json:"read_only"`
	NextAction string          `json:"next_action"`
}

func newDraftCartTool(catalog catalogClient, inventory inventoryClient, drafts CartDraftWriter) (tool.BaseTool, error) {
	return utils.InferTool("draft_cart",
		"Build a read-only cart draft from explicit SKU ids and quantities. Revalidates price and stock. Never writes the cart; the user must confirm in the UI.",
		func(ctx context.Context, in draftCartInput) (cartDraft, error) {
			if len(in.Items) == 0 || len(in.Items) > 10 {
				return cartDraft{}, errInvalidDraft("items must contain 1-10 lines")
			}
			out := cartDraft{
				Type: "cart_draft", DraftID: id.Event(),
				ExpiresAt: time.Now().Add(10 * time.Minute).Unix(), ReadOnly: true,
				NextAction: "confirm_in_ui",
			}
			for _, line := range in.Items {
				if line.SkuID <= 0 || line.Quantity <= 0 || line.Quantity > 99 {
					return cartDraft{}, errInvalidDraft("invalid sku or quantity")
				}
				sku, err := catalog.GetSku(ctx, &pb.GetSkuRequest{SkuId: line.SkuID})
				if err != nil {
					metrics.AIToolCalls.WithLabelValues("draft_cart", "error").Inc()
					return cartDraft{}, err
				}
				if sku.GetStatus() != "ENABLED" || sku.GetPriceCent() <= 0 {
					return cartDraft{}, errInvalidDraft("sku is not purchasable")
				}
				stock, err := inventory.GetStock(ctx, &pb.GetStockRequest{SkuId: line.SkuID})
				if err != nil {
					metrics.AIToolCalls.WithLabelValues("draft_cart", "error").Inc()
					return cartDraft{}, err
				}
				if stock.GetAvailable() < int64(line.Quantity) {
					return cartDraft{}, errInvalidDraft("insufficient stock")
				}
				item := cartDraftItem{
					SkuID: line.SkuID, ProductID: sku.GetSpuId(), PriceCent: sku.GetPriceCent(),
					Quantity: line.Quantity, Available: stock.GetAvailable(),
					StockTimestamp: time.Now().Unix(),
				}
				if p, err := catalog.GetProduct(ctx, &pb.GetProductRequest{Id: sku.GetSpuId()}); err == nil {
					item.Name = p.GetName()
				}
				out.Items = append(out.Items, item)
				out.TotalCent += item.PriceCent * int64(item.Quantity)
			}
			publishDraft(ctx, out)
			if drafts != nil {
				uid, ok := userIDFromContext(ctx)
				if !ok || uid == 0 {
					return cartDraft{}, errInvalidDraft("identity is required")
				}
				if err := drafts.Save(ctx, out.DraftID, uid, out); err != nil {
					return cartDraft{}, err
				}
			}
			metrics.AIToolCalls.WithLabelValues("draft_cart", "ok").Inc()
			return out, nil
		})
}

type invalidDraft string

func (e invalidDraft) Error() string   { return string(e) }
func errInvalidDraft(msg string) error { return invalidDraft(msg) }
