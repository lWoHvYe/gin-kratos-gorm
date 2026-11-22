package fulfillment

import (
	"context"
	"encoding/json"
	"log/slog"

	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

type afterSaleRefundEvent struct {
	AfterSaleNo string `json:"after_sale_no"`
	OrderNo     string `json:"order_no"`
	GroupNo     string `json:"group_no"`
}

// RunAfterSaleRefundConsumer voids a CREATED parcel when its group refunds
// before shipment. A shipment already SHIPPED/RECEIVED is left untouched; the
// refund protocol allows that case. The order.paid consumer also checks the
// group action, so an event arriving before the draft parcel is harmless.
func RunAfterSaleRefundConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject:    "shop.aftersale.refund_requested.v1",
		Durable:    "fulfillment-aftersale-refund",
		Queue:      "fulfillment-aftersale-refund",
		MaxDeliver: 10,
	}, func(hctx context.Context, msg *nats.Msg) error {
		var event afterSaleRefundEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		if event.OrderNo == "" {
			return consumer.Permanent(errMissingOrderNo)
		}
		return svc.CancelCreatedShipment(hctx, event.OrderNo, event.GroupNo, event.AfterSaleNo)
	})
	if err != nil {
		logger.Error("subscribe aftersale.refund_requested", "error", err)
	}
}
