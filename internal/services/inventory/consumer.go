package inventory

import (
	"context"
	"encoding/json"
	"log/slog"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/consumer"
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
// because Release is idempotent. Failures retry and eventually dead-letter.
func RunOrderCancelledConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject: "shop.order.cancelled.v1",
		Durable: "inventory-order-cancelled",
	}, func(hctx context.Context, msg *nats.Msg) error {
		var event orderCancelledEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		if event.OrderNo == "" || len(event.Items) == 0 {
			return nil
		}
		for _, item := range event.Items {
			if item.SkuID <= 0 || item.Quantity <= 0 {
				continue
			}
			if _, err := svc.Release(hctx, &pb.ReserveRequest{OrderNo: event.OrderNo, SkuId: item.SkuID, Quantity: item.Quantity}); err != nil {
				if apperr.IsCode(err, apperr.CodeNotFound) {
					continue
				}
				return err
			}
		}
		return nil
	})
	if err != nil {
		logger.Error("subscribe order.cancelled", "error", err)
	}
}
