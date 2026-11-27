package inventory

import (
	"context"
	"database/sql"
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
	"gin-kratos-gorm/internal/platform/outbox"

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
		return handleRefundReleaseEvent(hctx, svc, msg.Data)
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
func decodeRefundRelease(data []byte) (*events.RefundReleaseRequested, error) {
	if err := events.ValidateJSON(events.RefundReleaseEventType, data); err != nil {
		return nil, err
	}
	var event events.RefundReleaseRequested
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// handleRefundReleaseEvent routes the legacy_24h source through the ack-aware
// transaction; all other sources keep the existing no-ack path.
func handleRefundReleaseEvent(ctx context.Context, svc *Service, data []byte) error {
	event, err := decodeRefundRelease(data)
	if err != nil {
		return consumer.Permanent(err)
	}
	if event.Source == events.RefundReleaseSourceLegacy24h {
		return handleRefundReleaseLegacyAck(ctx, svc, event)
	}
	return handleRefundRelease(ctx, svc, data)
}

func handleRefundRelease(ctx context.Context, svc refundReleaseExecutor, data []byte) error {
	eventp, err := decodeRefundRelease(data)
	if err != nil {
		return consumer.Permanent(err)
	}
	event := *eventp
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

// handleRefundReleaseLegacyAck applies every item and appends the ack outbox in
// one DB transaction (ADR-0011). A retryable error returns without an ack; a
// permanent data error writes a best-effort FAILED ack and then DLQs.
func handleRefundReleaseLegacyAck(ctx context.Context, svc *Service, event *events.RefundReleaseRequested) error {
	err := svc.write(ctx, func(tx *sql.Tx) error {
		for _, item := range event.Items {
			if err := svc.applyLegacyReleaseItemTx(ctx, tx, event.OrderNo, item); err != nil {
				return err
			}
		}
		return appendRefundReleaseAckTx(ctx, tx, event, events.RefundReleaseStatusReleased, "")
	})
	if err == nil {
		return nil
	}
	if !isPermanentReleaseError(err) {
		return err
	}
	// The release transaction rolled back; persist a FAILED ack so Order keeps
	// COMPENSATING instead of waiting forever. Best effort: the DLQ still has
	// the original message.
	appendErr := svc.write(ctx, func(tx *sql.Tx) error {
		return appendRefundReleaseAckTx(ctx, tx, event, events.RefundReleaseStatusFailed, truncateReleaseError(err))
	})
	if appendErr != nil {
		return consumer.Permanent(fmt.Errorf("%w (failed ack: %v)", err, appendErr))
	}
	return consumer.Permanent(err)
}

func (s *Service) applyLegacyReleaseItemTx(ctx context.Context, tx *sql.Tx, orderNo string, item events.RefundReleaseItem) error {
	r, err := s.reservationByOrderSku(ctx, tx, orderNo, item.SkuID, item.WarehouseID, false)
	if apperr.IsCode(err, apperr.CodeNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	switch r.status {
	case statusReleased:
		return nil
	case statusHeld:
		_, err = s.releaseTx(ctx, tx, reserveInput{
			orderNo: orderNo, skuID: item.SkuID, warehouseID: item.WarehouseID,
		})
		return err
	case statusConfirmed:
		if r.quantity <= 0 {
			return apperr.Invalid("confirmed reservation has no quantity")
		}
		_, err = s.cancelReservationTx(ctx, tx, orderNo, item.SkuID, item.WarehouseID, r.quantity)
		return err
	default:
		return apperr.Invalid("unknown reservation status %s", r.status)
	}
}

func appendRefundReleaseAckTx(ctx context.Context, tx *sql.Tx, event *events.RefundReleaseRequested, status, failure string) error {
	var existing int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_events
		WHERE event_type = ? AND aggregate_id = ?`, events.RefundReleaseCompletedEventType, event.ReleaseID).Scan(&existing)
	if err != nil {
		return apperr.Internal("check refund release ack", err)
	}
	if existing > 0 {
		return nil
	}
	payload := events.RefundReleaseCompleted{
		SchemaVersion: 1,
		ReleaseID:     event.ReleaseID,
		RefundID:      event.RefundID,
		OrderNo:       event.OrderNo,
		Status:        status,
		Items:         event.Items,
		ReleasedAt:    time.Now().UTC().Format(time.RFC3339),
		Error:         failure,
	}
	return outbox.Append(ctx, tx, outbox.Event{
		Topic:         events.RefundReleaseCompletedTopic,
		AggregateType: "inventory",
		AggregateID:   event.ReleaseID,
		EventType:     events.RefundReleaseCompletedEventType,
		Payload:       payload,
	})
}

func isPermanentReleaseError(err error) bool {
	return apperr.IsCode(err, apperr.CodeInvalidArgument) ||
		apperr.IsCode(err, apperr.CodeNotFound) ||
		apperr.IsCode(err, apperr.CodeFailedPrecondition)
}

func truncateReleaseError(err error) string {
	text := err.Error()
	if len(text) > 512 {
		return text[:512]
	}
	return text
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
