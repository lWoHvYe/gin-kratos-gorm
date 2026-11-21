package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"

	"github.com/nats-io/nats.go"
)

type shipmentStatusEvent struct {
	OrderNo            string `json:"order_no"`
	ShipmentNo         string `json:"shipment_no"`
	Status             string `json:"status"`
	FulfillmentGroupNo string `json:"fulfillment_group_no"`
}

// RunShipmentConsumer projects fulfillment events onto the order aggregate.
// The projection is durable and idempotent: duplicate events may be delivered
// after a crash, and the status update is a compare-and-set.
func RunShipmentConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	if client == nil || client.JS == nil {
		return
	}
	bind := func(subject, durable, target string, from ...string) {
		statusTarget := target
		statusFrom := from
		err := consumer.Subscribe(ctx, client, logger, consumer.Config{
			Subject: subject,
			Durable: durable,
		}, func(hctx context.Context, msg *nats.Msg) error {
			var event shipmentStatusEvent
			if err := json.Unmarshal(msg.Data, &event); err != nil {
				return consumer.Permanent(err)
			}
			if event.OrderNo == "" {
				return consumer.Permanent(fmt.Errorf("shipment event missing order_no"))
			}
			if err := svc.applyShipmentStatusGroup(hctx, event.OrderNo, event.FulfillmentGroupNo, statusTarget, statusFrom...); err != nil {
				return err
			}
			logger.Info("shipment status projected", "order_no", event.OrderNo, "target", statusTarget)
			return nil
		})
		if err != nil {
			logger.Error("subscribe shipment event", "subject", subject, "error", err)
		}
	}
	bind("shop.shipment.created.v1", "order-shipment-created", statusAllocated, statusPaid)
	bind("shop.shipment.shipped.v1", "order-shipment-shipped", statusShipped, statusPaid, statusAllocated)
	bind("shop.shipment.delivered.v1", "order-shipment-delivered", statusCompleted, statusShipped)
}

func (s *Service) applyShipmentStatus(ctx context.Context, orderNo, target string, from ...string) error {
	return s.applyShipmentStatusGroup(ctx, orderNo, defaultGroupNo, target, from...)
}

// applyShipmentStatusGroup advances one fulfillment group and, only when every
// group reaches the same aggregate milestone, advances the order lifecycle.
// Partial shipment/receipt stays in ALLOCATED/SHIPPED plus a derived
// shipment_status so it never becomes a new lifecycle status.
func (s *Service) applyShipmentStatusGroup(ctx context.Context, orderNo, groupNo, target string, from ...string) error {
	groupNo = permitGroup(groupNo)
	groupStatus := target
	if target == statusCompleted {
		groupStatus = "RECEIVED"
	}
	for attempt := 0; attempt < 5; attempt++ {
		record, err := findOrder(ctx, s.db, orderNo)
		if err != nil {
			return fmt.Errorf("load order: %w", err)
		}
		if record == nil {
			return nil
		}
		updated := false
		err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
			// Never downgrade a group. A delivered event may move SHIPPED to
			// RECEIVED, while a shipped event must not move RECEIVED backwards.
			guard := "status NOT IN ('SHIPPED', 'RECEIVED', 'CANCELLED')"
			if target == statusCompleted {
				guard = "status NOT IN ('RECEIVED', 'CANCELLED')"
			} else if target == statusAllocated {
				guard = "status NOT IN ('ALLOCATED', 'SHIPPED', 'RECEIVED', 'CANCELLED')"
			}
			if _, err := tx.ExecContext(ctx, `UPDATE order_fulfillment_groups
				SET status = ?, version = version + 1, updated_at = NOW(3)
				WHERE order_no = ? AND group_no = ? AND `+guard,
				groupStatus, orderNo, groupNo); err != nil {
				return apperr.Internal("update fulfillment group status", err)
			}
			groups, err := loadFulfillmentGroupsTx(ctx, tx, orderNo)
			if err != nil {
				return err
			}
			orderTarget := aggregateOrderStatus(record.Status, target, groups)
			if orderTarget == "" || rankStatus(record.Status) >= rankStatus(orderTarget) {
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
				return fmt.Errorf("cannot move order from %s to %s", record.Status, orderTarget)
			}
			affected, err := updateOrderStatus(ctx, tx, orderNo, record.Status, orderTarget, record.PaidCent)
			if err != nil {
				return err
			}
			if affected == 0 {
				return nil
			}
			updated = true
			if err := insertStatusLog(ctx, tx, orderNo, record.Status, orderTarget, "shipment projection", "fulfillment"); err != nil {
				return err
			}
			topic, eventType := topicOrderShipped, "order.shipped"
			if orderTarget == statusCompleted {
				topic, eventType = topicOrderCompleted, "order.completed"
			}
			if orderTarget == statusAllocated {
				topic, eventType = topicOrderAllocated, "order.allocated"
			}
			return outbox.Append(ctx, tx, outbox.Event{
				Topic: topic, AggregateType: "order", AggregateID: orderNo, EventType: eventType,
				Payload: map[string]any{"order_no": orderNo, "status": orderTarget, "group_no": groupNo, "changed_at": time.Now().Unix()},
			})
		})
		if err != nil {
			return err
		}
		if updated {
			return nil
		}
		// The group update is idempotent; if no order transition was needed we
		// are done. Re-read only when a concurrent transition changed the order.
		current, err := findOrder(ctx, s.db, orderNo)
		if err != nil {
			return err
		}
		if current == nil || rankStatus(current.Status) >= rankStatus(target) || current.Status == record.Status {
			return nil
		}
	}
	return fmt.Errorf("shipment projection did not converge for %s", orderNo)
}

func aggregateOrderStatus(orderStatus, target string, groups []*fulfillmentGroup) string {
	if len(groups) == 0 {
		if target == statusCompleted {
			return statusCompleted
		}
		return target
	}
	shipped, received, cancelled := 0, 0, 0
	for _, group := range groups {
		switch group.Status {
		case "SHIPPED":
			shipped++
		case "RECEIVED":
			shipped++
			received++
		case "CANCELLED":
			cancelled++
		}
	}
	switch target {
	case statusCompleted:
		if received == len(groups) {
			return statusCompleted
		}
	case statusShipped:
		if shipped == len(groups) && received < len(groups) {
			return statusShipped
		}
	case statusAllocated:
		if orderStatus == statusPaid {
			return statusAllocated
		}
	}
	_ = cancelled
	return ""
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
