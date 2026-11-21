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
		Subject: "shop.order.paid.v1",
		Durable: "fulfillment-order-paid",
		// Legacy queue name must be preserved: the durable consumer already
		// exists with this deliver group.
		Queue:      "fulfillment-auto-shipment",
		MaxDeliver: 5,
	}, func(hctx context.Context, msg *nats.Msg) error {
		var event orderPaidEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		if event.OrderNo == "" {
			return consumer.Permanent(errMissingOrderNo)
		}
		if svc.orders == nil {
			return consumer.Permanent(errMissingOrderClient)
		}
		order, err := svc.orders.GetOrder(hctx, &pb.GetOrderRequest{OrderNo: event.OrderNo, Admin: true})
		if err != nil {
			return err
		}
		groups := order.GetFulfillmentGroups()
		if len(groups) == 0 {
			// Legacy single-group order (or an order created before group
			// backfill): keep the historical event-items path.
			groups = []*pb.OrderFulfillmentGroup{{GroupNo: "G1", WarehouseId: 1}}
		}
		for _, group := range groups {
			items := make([]*pb.ShipmentItem, 0, len(order.GetItems()))
			for _, item := range order.GetItems() {
				if item.GetSkuId() <= 0 || item.GetQuantity() <= 0 {
					continue
				}
				if item.GetGroupNo() == group.GetGroupNo() ||
					(item.GetGroupNo() == "" && group.GetGroupNo() == "G1") {
					items = append(items, &pb.ShipmentItem{SkuId: item.GetSkuId(), Quantity: item.GetQuantity()})
				}
			}
			if len(items) == 0 {
				logger.Warn("order group without items, shipment not auto-created",
					"order_no", event.OrderNo, "group_no", group.GetGroupNo())
				continue
			}
			if _, err := svc.CreateShipment(hctx, &pb.CreateShipmentRequest{
				OrderNo: event.OrderNo, FulfillmentGroupNo: group.GetGroupNo(),
				WarehouseId: group.GetWarehouseId(), Carrier: "AUTO", Items: items,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		logger.Error("subscribe order.paid", "error", err)
	}
}

type missingOrderNoError struct{}

func (missingOrderNoError) Error() string { return "order.paid missing order_no" }

type missingOrderClientError struct{}

func (missingOrderClientError) Error() string { return "order client is not configured" }

var errMissingOrderNo = missingOrderNoError{}
var errMissingOrderClient = missingOrderClientError{}
