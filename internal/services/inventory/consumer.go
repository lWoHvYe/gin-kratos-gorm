package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	pb "gin-kratos-gorm/api/gen/mall/v1"
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
		return handleOrderCancelled(hctx, svc, msg.Data)
	})
	if err != nil {
		logger.Error("subscribe order.cancelled", "error", err)
	}
}

func handleOrderCancelled(ctx context.Context, svc *Service, data []byte) error {
	var event orderCancelledEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return consumer.Permanent(err)
	}
	if event.OrderNo == "" || len(event.Items) == 0 {
		return nil
	}
	items := make([]*pb.OrderItem, 0, len(event.Items))
	for _, item := range event.Items {
		if item.SkuID > 0 && item.Quantity > 0 {
			items = append(items, &pb.OrderItem{SkuId: item.SkuID, Quantity: item.Quantity})
		}
	}
	if len(items) == 0 {
		return nil
	}
	// Use the same HELD/CONFIRMED/seckill compensation path as TCC cancel.
	// A missing reservation is a terminal MISSING item, not an early return:
	// the remaining items must still be processed before this message is Acked.
	summary, err := svc.applyInventoryOperation(ctx, opTypeCancel, event.OrderNo, items)
	if err != nil {
		return err
	}
	if summary.Retryable > 0 {
		return fmt.Errorf("inventory cancel has %d retryable item(s)", summary.Retryable)
	}
	return nil
}
