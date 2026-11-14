package fulfillment

import (
	"context"
	"encoding/json"
	"log/slog"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/consumer"
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
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject:    "shop.order.paid.v1",
		Durable:    "fulfillment-order-paid",
		MaxDeliver: 5,
	}, func(hctx context.Context, msg *nats.Msg) error {
		var event orderPaidEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		if event.OrderNo == "" {
			return consumer.Permanent(errMissingOrderNo)
		}
		items := make([]*pb.ShipmentItem, 0, len(event.Items))
		for _, item := range event.Items {
			if item.SkuID > 0 && item.Quantity > 0 {
				items = append(items, &pb.ShipmentItem{SkuId: item.SkuID, Quantity: item.Quantity})
			}
		}
		if len(items) == 0 {
			logger.Warn("order.paid without items, shipment not auto-created", "order_no", event.OrderNo)
			return nil
		}
		_, err := svc.CreateShipment(hctx, &pb.CreateShipmentRequest{
			OrderNo:     event.OrderNo,
			WarehouseId: 1,
			Carrier:     "AUTO",
			Items:       items,
		})
		return err
	})
	if err != nil {
		logger.Error("subscribe order.paid", "error", err)
	}
}

type missingOrderNoError struct{}

func (missingOrderNoError) Error() string { return "order.paid missing order_no" }

var errMissingOrderNo = missingOrderNoError{}
