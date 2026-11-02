package inventory

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

type orderCancelledEvent struct {
	OrderNo string `json:"order_no"`
	Items   []struct {
		SkuID    int64 `json:"sku_id"`
		Quantity int32 `json:"quantity"`
	} `json:"items"`
}

// RunOrderCancelledConsumer releases reservations after an order is cancelled.
// The event is the reliable compensation boundary; repeated delivery is safe
// because Release is idempotent.
func RunOrderCancelledConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	if client == nil || client.JS == nil {
		return
	}
	handler := func(msg *nats.Msg) {
		var event orderCancelledEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			logger.Error("decode order.cancelled", "error", err)
			_ = msg.Term()
			return
		}
		if event.OrderNo == "" || len(event.Items) == 0 {
			_ = msg.Ack()
			return
		}
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		for _, item := range event.Items {
			if item.SkuID <= 0 || item.Quantity <= 0 {
				continue
			}
			if _, err := svc.Release(cctx, &pb.ReserveRequest{OrderNo: event.OrderNo, SkuId: item.SkuID, Quantity: item.Quantity}); err != nil {
				if apperr.IsCode(err, apperr.CodeNotFound) {
					continue
				}
				logger.Warn("release cancelled order reservation", "order_no", event.OrderNo, "sku_id", item.SkuID, "error", err)
				_ = msg.NakWithDelay(5 * time.Second)
				return
			}
		}
		_ = msg.Ack()
	}
	if _, err := client.JS.QueueSubscribe("shop.order.cancelled.v1", "inventory-order-cancelled", handler,
		nats.Durable("inventory-order-cancelled"), nats.ManualAck(), nats.AckExplicit(), nats.MaxDeliver(20)); err != nil {
		logger.Error("subscribe order.cancelled", "error", err)
	}
}
