package inventory

import (
	"context"
	"encoding/json"
	"log/slog"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

type compensationService interface {
	TccCancelConfirm(ctx context.Context, in *pb.TccPaymentRequest) (*pb.Empty, error)
}

type compensationEvent struct {
	OrderNo string `json:"order_no"`
	Items   []struct {
		SkuID    int64 `json:"sku_id"`
		Quantity int32 `json:"quantity"`
	} `json:"items"`
}

// RunCompensationConsumer retries inventory compensation emitted by order when
// a direct TccCancelConfirm failed. TccCancelConfirm is idempotent, so a replay
// after a partial compensation is safe.
func RunCompensationConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject: "shop.inventory.compensation.requested.v1",
		Durable: "inventory-compensation",
	}, func(hctx context.Context, msg *nats.Msg) error {
		return handleCompensation(hctx, svc, msg.Data)
	})
	if err != nil {
		logger.Error("subscribe inventory compensation", "error", err)
	}
}

func handleCompensation(ctx context.Context, svc compensationService, data []byte) error {
	var event compensationEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return consumer.Permanent(err)
	}
	if event.OrderNo == "" || len(event.Items) == 0 {
		return nil
	}
	items := make([]*pb.OrderItem, 0, len(event.Items))
	for _, item := range event.Items {
		if item.SkuID <= 0 || item.Quantity <= 0 {
			continue
		}
		items = append(items, &pb.OrderItem{SkuId: item.SkuID, Quantity: item.Quantity})
	}
	if len(items) == 0 {
		return nil
	}
	_, err := svc.TccCancelConfirm(ctx, &pb.TccPaymentRequest{OrderNo: event.OrderNo, Items: items})
	return err
}
