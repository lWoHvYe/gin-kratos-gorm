package catalog

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

type stockChangedEvent struct {
	SkuID       int64 `json:"sku_id"`
	WarehouseID int64 `json:"warehouse_id"`
	OnHand      int64 `json:"on_hand"`
	Reserved    int64 `json:"reserved"`
	Available   int64 `json:"available"`
}

// RunStockConsumer keeps catalog.skus.stock as a display projection of the
// inventory service. Inventory remains authoritative; the projection is only
// used for browsing and is revalidated by DTM Reserve at checkout.
func RunStockConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	if client == nil || client.JS == nil {
		return
	}
	handler := func(msg *nats.Msg) {
		var event stockChangedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			logger.Error("decode inventory.stock_changed", "error", err)
			_ = msg.Term()
			return
		}
		if event.SkuID <= 0 {
			_ = msg.Ack()
			return
		}
		// The current deployment is single-warehouse. Ignore other warehouses
		// rather than overwriting the projection with a different location.
		if event.WarehouseID > 0 && event.WarehouseID != 1 {
			_ = msg.Ack()
			return
		}
		available := event.Available
		if available < 0 {
			available = 0
		}
		if _, err := svc.db.ExecContext(ctx, `UPDATE skus SET stock = ? WHERE id = ?`, available, event.SkuID); err != nil {
			logger.Warn("project inventory stock", "sku_id", event.SkuID, "error", err)
			_ = msg.NakWithDelay(2 * time.Second)
			return
		}
		svc.cacheEvictProductBySku(ctx, event.SkuID)
		_ = msg.Ack()
	}
	if _, err := client.JS.QueueSubscribe("shop.inventory.stock.changed.v1", "catalog-stock-projection", handler,
		nats.Durable("catalog-stock-projection"), nats.ManualAck(), nats.AckExplicit(), nats.MaxDeliver(20)); err != nil {
		logger.Error("subscribe inventory.stock_changed", "error", err)
	}
}

func (s *Service) cacheEvictProductBySku(ctx context.Context, skuID int64) {
	var spuID int64
	if err := s.db.QueryRowContext(ctx, `SELECT spu_id FROM skus WHERE id = ?`, skuID).Scan(&spuID); err == nil && spuID > 0 {
		s.cacheEvictProduct(ctx, spuID)
	}
}
