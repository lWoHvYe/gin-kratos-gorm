package catalog

import (
	"context"
	"encoding/json"
	"log/slog"

	"gin-kratos-gorm/internal/platform/consumer"
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
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject: "shop.inventory.stock.changed.v1",
		Durable: "catalog-stock-projection",
	}, func(hctx context.Context, msg *nats.Msg) error {
		var event stockChangedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		if event.SkuID <= 0 {
			return nil
		}
		// The current deployment is single-warehouse. Ignore other warehouses
		// rather than overwriting the projection with a different location.
		if event.WarehouseID > 0 && event.WarehouseID != 1 {
			return nil
		}
		available := event.Available
		if available < 0 {
			available = 0
		}
		if _, err := svc.db.ExecContext(hctx, `UPDATE skus SET stock = ? WHERE id = ?`, available, event.SkuID); err != nil {
			return err
		}
		svc.cacheEvictProductBySku(hctx, event.SkuID)
		return nil
	})
	if err != nil {
		logger.Error("subscribe inventory.stock_changed", "error", err)
	}
}

func (s *Service) cacheEvictProductBySku(ctx context.Context, skuID int64) {
	var spuID int64
	if err := s.db.QueryRowContext(ctx, `SELECT spu_id FROM skus WHERE id = ?`, skuID).Scan(&spuID); err == nil && spuID > 0 {
		s.cacheEvictProduct(ctx, spuID)
	}
}
