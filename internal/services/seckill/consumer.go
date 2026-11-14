package seckill

import (
	"context"
	"encoding/json"
	"log/slog"

	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

type orderCancelledEvent struct {
	OrderNo string `json:"order_no"`
	UserID  int64  `json:"user_id"`
	Reason  string `json:"reason"`
}

// RunOrderCancelledConsumer projects an order cancellation onto the
// qualification. Inventory release is owned by the inventory consumer; this
// consumer only fixes the customer-visible qualification state.
func RunOrderCancelledConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject: "shop.order.cancelled.v1",
		Durable: "seckill-order-cancelled",
	}, func(hctx context.Context, msg *nats.Msg) error {
		var event orderCancelledEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		if event.OrderNo == "" {
			return consumer.Permanent(errMissingOrderNo)
		}
		_, err := svc.db.ExecContext(hctx, `UPDATE seckill_qualifications
			SET status='CANCELLED' WHERE order_no=? AND status IN ('ORDERED','ORDERING')`, event.OrderNo)
		return err
	})
	if err != nil {
		logger.Error("subscribe seckill order cancellation", "error", err)
	}
}

type missingOrderNoError struct{}

func (missingOrderNoError) Error() string { return "order.cancelled missing order_no" }

var errMissingOrderNo = missingOrderNoError{}
