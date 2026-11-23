package fulfillment

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

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

// RunOrderPaidConsumer is the legacy settlement consumer. During the ADR-0007
// rollout it can be replaced by RunOrderSettledConsumer via
// FULFILLMENT_CONSUME_ORDER_SETTLED.
func RunOrderPaidConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	runOrderSettlementConsumer(ctx, client, svc, logger, "shop.order.paid.v1", "fulfillment-order-paid", "fulfillment-auto-shipment")
}

// RunOrderSettledConsumer creates a CREATED shipment from the new settlement
// fact. It still checks FulfillmentAllowed before creating anything.
func RunOrderSettledConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	runOrderSettlementConsumer(ctx, client, svc, logger, "shop.order.settled.v1", "fulfillment-order-settled", "")
}

func runOrderSettlementConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger, subject, durable, queue string) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject:    subject,
		Durable:    durable,
		Queue:      queue,
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
			if !autoShipAllowed(order, group.GetGroupNo()) {
				logger.Info("order group is not shippable; auto shipment skipped",
					"order_no", event.OrderNo, "group_no", group.GetGroupNo())
				continue
			}
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

func autoShipAllowed(order *pb.Order, groupNo string) bool {
	for _, action := range order.GetGroupActions() {
		if action.GetGroupNo() == groupNo && action.GetAction() == "ship" {
			return true
		}
	}
	for _, action := range order.GetAvailableActions() {
		if action == "ship_group:"+groupNo || (groupNo == "G1" && strings.TrimSpace(action) == "ship") {
			return true
		}
	}
	return false
}
