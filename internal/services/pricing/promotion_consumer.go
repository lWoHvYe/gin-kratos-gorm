package pricing

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

type orderPaidEvent struct {
	OrderNo string `json:"order_no"`
	UserID  int64  `json:"user_id"`
}

// RunOrderPaidConsumer confirms promotion budget reservations after payment.
// The direct call in order.afterPaid is best-effort; this durable consumer is
// the retry path. Both are idempotent.
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
			_ = msg.Ack()
			return
		}
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := svc.promotion.ConfirmByOrder(cctx, event.OrderNo); err != nil {
			logger.Warn("confirm promotions", "order_no", event.OrderNo, "error", err)
			_ = msg.NakWithDelay(5 * time.Second)
			return
		}
		_ = msg.Ack()
	}
	if _, err := client.QueueSubscribe("shop.order.paid.v1", "pricing-promotion-paid", handler,
		nats.Durable("pricing-promotion-paid"), nats.ManualAck(), nats.AckExplicit(), nats.MaxDeliver(20)); err != nil {
		logger.Error("subscribe order.paid", "error", err)
	}
}

// RunPromotionExpiry releases budget held by abandoned quotes/orders. Order
// cancellation is the primary path; this is the safety net.
func RunPromotionExpiry(ctx context.Context, svc *Service, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			n, err := svc.promotion.ReleaseExpired(cctx, 100)
			cancel()
			if err != nil {
				logger.Warn("release expired promotions", "error", err)
			} else if n > 0 {
				logger.Info("released expired promotions", "orders", n)
			}
		}
	}
}
