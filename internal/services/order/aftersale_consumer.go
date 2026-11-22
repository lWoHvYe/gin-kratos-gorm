package order

import (
	"context"
	"encoding/json"
	"log/slog"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/events"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

type afterSaleCompletedEvent = events.AfterSaleCompleted

// RunAfterSaleConsumer projects a completed refund onto the explicit
// per-group refund state. The permit decision remains authoritative; this is
// the read model used by the order/admin UI.
func RunAfterSaleConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject:    "shop.aftersale.completed.v1",
		Durable:    "order-aftersale-completed",
		Queue:      "order-aftersale-completed",
		MaxDeliver: 10,
	}, func(hctx context.Context, msg *nats.Msg) error {
		var event afterSaleCompletedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		return applyAfterSaleCompleted(hctx, svc, event)
	})
	if err != nil {
		logger.Error("subscribe aftersale.completed", "error", err)
	}
}

func applyAfterSaleCompleted(ctx context.Context, svc *Service, event afterSaleCompletedEvent) error {
	if event.OrderNo == "" || event.GroupNo == "" {
		return nil
	}
	if _, err := svc.db.ExecContext(ctx, `UPDATE order_fulfillment_groups
		SET refund_state = ?, version = version + 1, updated_at = NOW(3)
		WHERE order_no = ? AND group_no = ? AND refund_state <> ?`,
		groupRefundStateRefunded, event.OrderNo, event.GroupNo, groupRefundStateRefunded); err != nil {
		return apperr.Internal("project group refund state", err)
	}
	return nil
}
