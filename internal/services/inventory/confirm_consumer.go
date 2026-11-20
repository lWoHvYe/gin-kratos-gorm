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

type confirmService interface {
	TccConfirm(ctx context.Context, in *pb.TccPaymentRequest) (*pb.Empty, error)
}

type confirmEvent struct {
	OrderNo string `json:"order_no"`
	Items   []struct {
		SkuID    int64 `json:"sku_id"`
		Quantity int32 `json:"quantity"`
	} `json:"items"`
}

// RunConfirmConsumer retries confirmation for an already-paid order whose
// direct Confirm failed. It must never release stock, unlike compensation.
func RunConfirmConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject: "shop.inventory.confirm.requested.v1",
		Durable: "inventory-confirm",
	}, func(hctx context.Context, msg *nats.Msg) error {
		return handleConfirmRequest(hctx, svc, msg.Data)
	})
	if err != nil {
		logger.Error("subscribe inventory confirm", "error", err)
	}
}

func handleConfirmRequest(ctx context.Context, svc confirmService, data []byte) error {
	var event confirmEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return consumer.Permanent(err)
	}
	if event.OrderNo == "" || len(event.Items) == 0 {
		return nil
	}
	items := make([]*pb.OrderItem, 0, len(event.Items))
	for _, item := range event.Items {
		if item.SkuID > 0 && item.Quantity > 0 {
			items = append(items, &pb.OrderItem{SkuId: item.SkuID, Quantity: item.Quantity})
		}
	}
	if len(items) == 0 {
		return nil
	}
	_, err := svc.TccConfirm(ctx, &pb.TccPaymentRequest{OrderNo: event.OrderNo, Items: items})
	return err
}
