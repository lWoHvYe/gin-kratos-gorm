package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"

	"github.com/nats-io/nats.go"
)

type shipmentStatusEvent struct {
	OrderNo    string `json:"order_no"`
	ShipmentNo string `json:"shipment_no"`
	Status     string `json:"status"`
}

// RunShipmentConsumer projects fulfillment events onto the order aggregate.
// The projection is durable and idempotent: duplicate events may be delivered
// after a crash, and the status update is a compare-and-set.
func RunShipmentConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	if client == nil || client.JS == nil {
		return
	}
	bind := func(subject, durable, target string, from ...string) {
		handler := func(msg *nats.Msg) {
			var event shipmentStatusEvent
			if err := json.Unmarshal(msg.Data, &event); err != nil {
				logger.Error("decode shipment event", "subject", subject, "error", err)
				_ = msg.Term()
				return
			}
			if event.OrderNo == "" {
				_ = msg.Term()
				return
			}
			cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := svc.applyShipmentStatus(cctx, event.OrderNo, target, from...); err != nil {
				logger.Warn("apply shipment status", "order_no", event.OrderNo, "target", target, "error", err)
				_ = msg.NakWithDelay(5 * time.Second)
				return
			}
			_ = msg.Ack()
		}
		if _, err := client.JS.QueueSubscribe(subject, durable, handler,
			nats.Durable(durable), nats.ManualAck(), nats.AckExplicit(), nats.MaxDeliver(20)); err != nil {
			logger.Error("subscribe shipment event", "subject", subject, "error", err)
		}
	}
	bind("shop.shipment.created.v1", "order-shipment-created", statusAllocated, statusPaid)
	bind("shop.shipment.shipped.v1", "order-shipment-shipped", statusShipped, statusPaid, statusAllocated)
	bind("shop.shipment.delivered.v1", "order-shipment-delivered", statusCompleted, statusShipped)
}

func (s *Service) applyShipmentStatus(ctx context.Context, orderNo, target string, from ...string) error {
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return fmt.Errorf("load order: %w", err)
	}
	if record == nil {
		return nil
	}
	if record.Status == target || rankStatus(record.Status) > rankStatus(target) {
		return nil
	}
	allowed := false
	for _, status := range from {
		if record.Status == status {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("cannot move order from %s to %s", record.Status, target)
	}
	topic, eventType := topicOrderShipped, "order.shipped"
	if target == statusCompleted {
		topic, eventType = topicOrderCompleted, "order.completed"
	}
	if target == statusAllocated {
		topic, eventType = topicOrderAllocated, "order.allocated"
	}
	return db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		affected, err := updateOrderStatus(ctx, tx, orderNo, record.Status, target, record.PaidCent)
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		if err := insertStatusLog(ctx, tx, orderNo, record.Status, target, "shipment projection", "fulfillment"); err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic: topic, AggregateType: "order", AggregateID: orderNo, EventType: eventType,
			Payload: map[string]any{"order_no": orderNo, "status": target, "changed_at": time.Now().Unix()},
		})
	})
}

// rankStatus is used only for duplicate/stale event suppression.
func rankStatus(status string) int {
	switch status {
	case statusPendingPayment:
		return 0
	case statusPaid:
		return 1
	case statusAllocated:
		return 2
	case statusShipped:
		return 3
	case statusCompleted:
		return 4
	default:
		return -1
	}
}
