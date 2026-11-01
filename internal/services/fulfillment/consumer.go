package fulfillment

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

type orderPaidEvent struct {
	OrderNo string `json:"order_no"`
	Items   []struct {
		SkuID    int64  `json:"sku_id"`
		Quantity int32  `json:"quantity"`
		Name     string `json:"name"`
	} `json:"items"`
}

// RunOrderPaidConsumer creates a CREATED shipment as soon as an order is paid.
// The consumer is durable and idempotent: CreateShipment is keyed by order_no.
// An operator still assigns carrier/tracking and calls Ship.
func RunOrderPaidConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	if client == nil || client.JS == nil {
		return
	}
	handler := func(msg *nats.Msg) {
		var event orderPaidEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			logger.Error("decode order.paid", "error", err)
			_ = msg.Term()
			return
		}
		if event.OrderNo == "" {
			_ = msg.Term()
			return
		}
		items := make([]*pb.ShipmentItem, 0, len(event.Items))
		for _, item := range event.Items {
			if item.SkuID > 0 && item.Quantity > 0 {
				items = append(items, &pb.ShipmentItem{SkuId: item.SkuID, Quantity: item.Quantity})
			}
		}
		if len(items) == 0 {
			logger.Warn("order.paid without items, shipment not auto-created", "order_no", event.OrderNo)
			_ = msg.Ack()
			return
		}
		if _, err := svc.CreateShipment(ctx, &pb.CreateShipmentRequest{
			OrderNo:     event.OrderNo,
			WarehouseId: 1,
			Carrier:     "AUTO",
			Items:       items,
		}); err != nil {
			logger.Warn("auto-create shipment", "order_no", event.OrderNo, "error", err)
			_ = msg.NakWithDelay(5 * time.Second)
			return
		}
		_ = msg.Ack()
	}
	if _, err := client.JS.QueueSubscribe("shop.order.paid.v1", "fulfillment-auto-shipment", handler,
		nats.Durable("fulfillment-order-paid"), nats.ManualAck(), nats.AckExplicit(), nats.MaxDeliver(5)); err != nil {
		logger.Error("subscribe order.paid", "error", err)
	}
}
