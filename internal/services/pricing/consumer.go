package pricing

import (
	"context"
	"encoding/json"
	"log/slog"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

type orderCancelledEvent struct {
	OrderNo string `json:"order_no"`
	UserID  int64  `json:"user_id"`
	QuoteID string `json:"quote_id"`
}

func RunOrderCancelledConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject: "shop.order.cancelled.v1",
		Durable: "pricing-order-cancelled",
	}, func(hctx context.Context, msg *nats.Msg) error {
		var event orderCancelledEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		if event.QuoteID == "" {
			return nil
		}
		if _, err := svc.UnlockCoupon(hctx, &pb.LockCouponRequest{QuoteId: event.QuoteID, UserId: event.UserID}); err != nil {
			if !apperr.IsCode(err, apperr.CodeNotFound) {
				return err
			}
		}
		return svc.promotion.ReleaseByOrder(hctx, event.OrderNo)
	})
	if err != nil {
		logger.Error("subscribe order.cancelled", "error", err)
	}
}
