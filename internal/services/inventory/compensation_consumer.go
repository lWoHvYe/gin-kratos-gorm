package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

type compensationService interface {
	applyInventoryOperation(ctx context.Context, typ, orderNo string, items []*pb.OrderItem) (operationSummary, error)
}

type compensationEvent struct {
	OrderNo string `json:"order_no"`
	Items   []struct {
		SkuID       int64 `json:"sku_id"`
		WarehouseID int64 `json:"warehouse_id"`
		Quantity    int32 `json:"quantity"`
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
	for i, item := range event.Items {
		if item.SkuID <= 0 || item.Quantity <= 0 {
			continue
		}
		// INV-44: no implicit W1. Persisted compensation must carry the exact
		// warehouse of the reservation or it is a contract violation.
		if item.WarehouseID <= 0 {
			return consumer.Permanent(fmt.Errorf("inventory.compensation_requested item[%d] for order %s is missing warehouse_id",
				i, event.OrderNo))
		}
		items = append(items, &pb.OrderItem{
			SkuId: item.SkuID, WarehouseId: item.WarehouseID, Quantity: item.Quantity,
		})
	}
	if len(items) == 0 {
		return nil
	}
	summary, err := svc.applyInventoryOperation(ctx, opTypeCancel, event.OrderNo, items)
	if err != nil {
		return err
	}
	if summary.Retryable > 0 {
		return fmt.Errorf("inventory compensation has %d retryable item(s)", summary.Retryable)
	}
	return nil
}
