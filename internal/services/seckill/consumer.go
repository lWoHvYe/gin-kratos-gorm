package seckill

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

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
	if client == nil || client.JS == nil {
		return
	}
	handler := func(msg *nats.Msg) {
		var event orderCancelledEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil || event.OrderNo == "" {
			_ = msg.Term()
			return
		}
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, err := svc.db.ExecContext(cctx, `UPDATE seckill_qualifications
			SET status='CANCELLED' WHERE order_no=? AND status IN ('ORDERED','ORDERING')`, event.OrderNo); err != nil {
			logger.Warn("project seckill cancellation", "order_no", event.OrderNo, "error", err)
			_ = msg.NakWithDelay(5 * time.Second)
			return
		}
		_ = msg.Ack()
	}
	if _, err := client.QueueSubscribe("shop.order.cancelled.v1", "seckill-order-cancelled", handler,
		nats.Durable("seckill-order-cancelled"), nats.ManualAck(), nats.AckExplicit(), nats.MaxDeliver(20)); err != nil {
		logger.Error("subscribe seckill order cancellation", "error", err)
	}
}
