package fulfillment

import (
	"context"
	"encoding/json"
	"log/slog"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

type refundAdmittedEvent struct {
	RefundID string `json:"refund_id"`
	OrderNo  string `json:"order_no"`
	GroupNo  string `json:"group_no"`
	Result   string `json:"result"`
	State    string `json:"state"`
}

// RunRefundAdmittedConsumer applies Order's refund admission to Fulfillment:
// a PRE_SHIP_REVOKED result voids any CREATED parcel. The call is idempotent;
// a parcel that already left the warehouse is deliberately left alone.
func RunRefundAdmittedConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject:    "shop.order.refund_admitted.v1",
		Durable:    "fulfillment-refund-admitted",
		Queue:      "fulfillment-refund-admitted",
		MaxDeliver: 10,
	}, func(hctx context.Context, msg *nats.Msg) error {
		var event refundAdmittedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		if event.OrderNo == "" || event.RefundID == "" {
			return consumer.Permanent(errMissingOrderNo)
		}
		if event.Result != "PRE_SHIP_REVOKED" {
			return nil
		}
		reply, err := svc.CancelIfCreated(hctx, &pb.CancelIfCreatedRequest{
			OrderNo: event.OrderNo, GroupNo: event.GroupNo, RefundId: event.RefundID,
		})
		if err != nil {
			return err
		}
		if reply.GetStatus() == "ALREADY_SHIPPED" || reply.GetStatus() == "RECEIVED" {
			if logger != nil {
				logger.Error("refund admission says pre-ship but parcel already left",
					"order_no", event.OrderNo, "group_no", event.GroupNo, "refund_id", event.RefundID)
			}
		}
		return nil
	})
	if err != nil {
		logger.Error("subscribe order.refund_admitted", "error", err)
	}
}
