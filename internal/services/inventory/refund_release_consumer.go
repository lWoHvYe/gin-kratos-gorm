package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/events"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

// INV-44 / AC-50 / T1.4b2a: Inventory is the executor of the durable release request
// produced by Order. It only changes reservations/stock; it never touches
// permits, shipments or refunds. Idempotency is the reservation state machine
// itself (HELD/RELEASED are no-ops when replayed), keyed by the event's
// release_id at the protocol level. No release ledger table is added (C-05).
type refundReleaseExecutor interface {
	refundReleaseReservation(ctx context.Context, orderNo string, skuID, warehouseID int64) (status string, quantity int64, err error)
	Release(ctx context.Context, req *pb.ReserveRequest) (*pb.Empty, error)
	TccCancelConfirm(ctx context.Context, req *pb.TccPaymentRequest) (*pb.Empty, error)
}

// RunRefundReleaseConsumer subscribes the durable release consumer. Retries and
// DLQ are owned by the shared consumer framework: a failed message is retried
// with a delay up to MaxDeliver, then published to the subject's DLQ with an
// Error log and the `dlq` metric. A permanent payload/data error skips retries
// but still goes through the same DLQ + alert path.
func RunRefundReleaseConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject:        events.RefundReleaseTopic,
		Durable:        "inventory-refund-release",
		MaxDeliver:     20,
		AckWait:        30 * time.Second,
		HandlerTimeout: 30 * time.Second,
	}, func(hctx context.Context, msg *nats.Msg) error {
		return handleRefundRelease(hctx, svc, msg.Data)
	})
	if err != nil {
		logger.Error("subscribe inventory refund release", "error", err)
	}
}

// handleRefundRelease applies one event item by item. It never ACKs a partial
// success: the first retryable failure aborts the message, leaving already
// released items as RELEASED so the redelivery skips them. A malformed event
// (missing warehouse, mismatched release_id, wrong enum) is permanent and goes
// to the DLQ instead of being guessed.
func handleRefundRelease(ctx context.Context, svc refundReleaseExecutor, data []byte) error {
	if err := events.ValidateJSON(events.RefundReleaseEventType, data); err != nil {
		return consumer.Permanent(err)
	}
	var event events.RefundReleaseRequested
	if err := json.Unmarshal(data, &event); err != nil {
		return consumer.Permanent(err)
	}
	// group_count is the safety fact for a full-reservation release. P0 only
	// supports single-group orders; missing/0/!=1 is a contract error and must
	// never release stock.
	if event.GroupCount != 1 {
		return consumer.Permanent(fmt.Errorf(
			"refund release %s: group_count must be 1, got %d", event.ReleaseID, event.GroupCount))
	}
	if len(event.CoveredGroupNos) > 0 {
		// covered_group_nos is audit-only, but when present it must describe
		// exactly this one group; a stale/foreign producer listing another group
		// is a contract violation.
		if len(event.CoveredGroupNos) != 1 {
			return consumer.Permanent(fmt.Errorf(
				"refund release %s: covered_group_nos must contain exactly the single group",
				event.ReleaseID))
		}
		if strings.TrimSpace(event.CoveredGroupNos[0]) != strings.TrimSpace(event.GroupNo) {
			return consumer.Permanent(fmt.Errorf(
				"refund release %s: covered_group_nos %v does not match group_no %q",
				event.ReleaseID, event.CoveredGroupNos, event.GroupNo))
		}
	}
	for _, item := range event.Items {
		status, quantity, err := svc.refundReleaseReservation(ctx, event.OrderNo, item.SkuID, item.WarehouseID)
		if err != nil {
			return err
		}
		switch status {
		case "":
			// No reservation row: there is nothing to release. The admission
			// and the reservation are created before this event can exist, so a
			// missing row is an idempotent no-op, not a data guess.
			continue
		case statusReleased:
			continue
		case statusHeld:
			// Full-release semantics: the event quantity is audit input only
			// and is deliberately not compared to the reservation quantity.
			_, err = svc.Release(ctx, &pb.ReserveRequest{
				OrderNo: event.OrderNo, SkuId: item.SkuID, WarehouseId: item.WarehouseID,
			})
		case statusConfirmed:
			// TccCancelConfirm requires a positive quantity, so use the
			// authoritative reservation quantity, never the event quantity.
			if quantity <= 0 {
				return consumer.Permanent(fmt.Errorf(
					"refund release %s: confirmed reservation %s sku %d warehouse %d has no quantity",
					event.ReleaseID, event.OrderNo, item.SkuID, item.WarehouseID))
			}
			_, err = svc.TccCancelConfirm(ctx, &pb.TccPaymentRequest{
				OrderNo: event.OrderNo,
				Items: []*pb.OrderItem{{
					SkuId: item.SkuID, Quantity: int32(quantity), WarehouseId: item.WarehouseID,
				}},
			})
		default:
			return consumer.Permanent(fmt.Errorf(
				"refund release %s: reservation %s sku %d warehouse %d has unknown status %q",
				event.ReleaseID, event.OrderNo, item.SkuID, item.WarehouseID, status))
		}
		if err != nil {
			return classifyRefundReleaseError(err)
		}
	}
	return nil
}

// refundReleaseReservation returns "" when the reservation does not exist. A
// missing reservation is handled as an idempotent success by the caller; any
// other read error is retryable.
func (s *Service) refundReleaseReservation(ctx context.Context, orderNo string, skuID, warehouseID int64) (string, int64, error) {
	r, err := s.reservationByOrderSku(ctx, s.db, orderNo, skuID, warehouseID, false)
	if apperr.IsCode(err, apperr.CodeNotFound) {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	return r.status, r.quantity, nil
}

// classifyRefundReleaseError separates retryable races/timeouts from permanent
// data inconsistencies. A HELD reservation that became CONFIRMED between the
// status read and Release is a CONFLICT: the redelivery re-reads and takes the
// TccCancelConfirm branch. Failed-precondition/not-found/invalid are data
// problems that a retry cannot fix, so they go to the DLQ with an alert.
func classifyRefundReleaseError(err error) error {
	switch {
	case apperr.IsCode(err, apperr.CodeConflict):
		return err
	case apperr.IsCode(err, apperr.CodeInvalidArgument),
		apperr.IsCode(err, apperr.CodeNotFound),
		apperr.IsCode(err, apperr.CodeFailedPrecondition):
		return consumer.Permanent(err)
	default:
		return err
	}
}
