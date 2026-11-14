package pricing

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

type orderCancelledEvent struct {
	OrderNo string `json:"order_no"`
	UserID  int64  `json:"user_id"`
	QuoteID string `json:"quote_id"`
}

func RunOrderCancelledConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	if client == nil || client.JS == nil {
		return
	}
	handler := func(msg *nats.Msg) {
		var event orderCancelledEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			logger.Error("decode order.cancelled", "error", err)
			_ = msg.Term()
			return
		}
		if event.QuoteID == "" {
			_ = msg.Ack()
			return
		}
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, err := svc.UnlockCoupon(cctx, &pb.LockCouponRequest{QuoteId: event.QuoteID, UserId: event.UserID}); err != nil {
			if !apperr.IsCode(err, apperr.CodeNotFound) {
				logger.Warn("unlock cancelled order coupon", "order_no", event.OrderNo, "quote_id", event.QuoteID, "error", err)
				_ = msg.NakWithDelay(5 * time.Second)
				return
			}
		}
		if err := svc.promotion.ReleaseByOrder(cctx, event.OrderNo); err != nil {
			logger.Warn("release promotions", "order_no", event.OrderNo, "error", err)
			_ = msg.NakWithDelay(5 * time.Second)
			return
		}
		_ = msg.Ack()
	}
	if _, err := client.QueueSubscribe("shop.order.cancelled.v1", "pricing-order-cancelled", handler,
		nats.Durable("pricing-order-cancelled"), nats.ManualAck(), nats.AckExplicit(), nats.MaxDeliver(20)); err != nil {
		logger.Error("subscribe order.cancelled", "error", err)
	}
}
