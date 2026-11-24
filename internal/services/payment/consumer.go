package payment

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/metrics"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

// orderRefundRequested is the order.refund_requested payload. Since T1.6 it is
// a payment-level command carrier (`refund_id=pay:<payment_no>:cancel`) and no
// longer a group-refund trigger. All new fields are optional so legacy events
// still decode.
type orderRefundRequested struct {
	OrderNo             string `json:"order_no"`
	PaymentNo           string `json:"payment_no"`
	PaidCent            int64  `json:"paid_cent"`
	RefundID            string `json:"refund_id"`
	Source              string `json:"source"`
	NonEffectiveCapture bool   `json:"non_effective_capture"`
	AmountCent          int64  `json:"amount_cent"`
	Reason              string `json:"reason"`
	Status              string `json:"status"`
}

const (
	// paymentCancelSource is the only source allowed to execute a refund from
	// order.refund_requested. Any other command is observe-only (T1.6 §3.3).
	paymentCancelSource = "payment"
)

// refundCommandExecutor is the single Payment operation a command may execute.
// It lets the command/observe-only classification be unit tested without a
// database while still using the real Service in production.
type refundCommandExecutor interface {
	Refund(ctx context.Context, in *pb.RefundRequest) (*pb.Empty, error)
}

// RunRefundConsumer executes payment-level cancel commands written by Order.
// It deliberately does NOT execute TCC group refunds: those belong to the
// atomic RefundBatch path. Unknown/foreign order.refund_requested events are
// observed and acked, never refunded.
func RunRefundConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject: "shop.order.refund_requested.v1",
		Durable: "payment-order-refund",
		// Legacy queue name must be preserved: the durable consumer already
		// exists with this deliver group.
		Queue:      "payment-refund",
		MaxDeliver: 10,
	}, func(hctx context.Context, msg *nats.Msg) error {
		return handleOrderRefundRequested(hctx, svc, logger, msg.Data)
	})
	if err != nil {
		logger.Error("subscribe order.refund_requested", "error", err)
	}
}

// handleOrderRefundRequested classifies one order.refund_requested message and
// executes at most one idempotent Payment.Refund. The refund identity is fixed
// (`pay:<payment_no>:cancel`), so repeats and retries cannot refund twice.
func handleOrderRefundRequested(ctx context.Context, svc refundCommandExecutor, logger *slog.Logger, data []byte) error {
	var event orderRefundRequested
	if err := json.Unmarshal(data, &event); err != nil {
		return consumer.Permanent(err)
	}
	event.OrderNo = strings.TrimSpace(event.OrderNo)
	event.PaymentNo = strings.TrimSpace(event.PaymentNo)
	if event.PaymentNo == "" {
		metrics.PaymentRefundEvents.WithLabelValues("invalid").Inc()
		return consumer.Permanent(errMissingPaymentNo)
	}

	refundID, isCommand := paymentCancelCommand(event)
	if !isCommand {
		// TCC group events (T1.5 removed their producer) and any unknown
		// refund identity are observe-only: no Payment call, no group ledger.
		metrics.PaymentRefundEvents.WithLabelValues("observed").Inc()
		logger.Warn("order.refund_requested is not a payment-level cancel command; observe-only",
			"order_no", event.OrderNo, "payment_no", event.PaymentNo,
			"refund_id", event.RefundID, "source", event.Source,
			"reason", "PAYMENT_REFUND_REQUESTED_OBSERVE_ONLY")
		return nil
	}

	amount := event.AmountCent
	if amount <= 0 {
		// Legacy cancel-after-confirm events only carried paid_cent.
		amount = event.PaidCent
	}
	if amount <= 0 {
		// No captured money: the producer should not have written a command.
		// Acking avoids a poison loop; the order side already logs the anomaly.
		metrics.PaymentRefundEvents.WithLabelValues("zero_amount").Inc()
		logger.Warn("payment-level cancel command without captured amount",
			"order_no", event.OrderNo, "payment_no", event.PaymentNo,
			"refund_id", refundID, "reason", "PAYMENT_CANCEL_NO_CAPTURE")
		return nil
	}

	reason := strings.TrimSpace(event.Reason)
	if reason == "" {
		reason = "payment-level cancel"
	}
	if _, err := svc.Refund(ctx, &pb.RefundRequest{
		PaymentNo:           event.PaymentNo,
		AmountCent:          amount,
		Reason:              reason,
		IdempotencyKey:      refundID,
		NonEffectiveCapture: true,
	}); err != nil {
		code := apperr.From(err).Code
		if isRetryableRefundError(code) {
			// Payment timeout/unavailable/internal: keep the message and let
			// the durable consumer retry with backoff. The fixed refund_id
			// makes the retry idempotent.
			metrics.PaymentRefundEvents.WithLabelValues("retry").Inc()
			logger.Warn("payment-level cancel refund will be retried",
				"order_no", event.OrderNo, "payment_no", event.PaymentNo,
				"refund_id", refundID, "error", err,
				"reason", "PAYMENT_CANCEL_REFUND_RETRY")
			return err
		}
		// Deterministic rejection (not refundable / already fully refunded):
		// ack and alert instead of retrying forever or refunding twice.
		metrics.PaymentRefundEvents.WithLabelValues("rejected").Inc()
		logger.Error("payment-level cancel refund was rejected",
			"order_no", event.OrderNo, "payment_no", event.PaymentNo,
			"refund_id", refundID, "error", err, "reason", paymentRefundRejectReason(err))
		return nil
	}
	metrics.PaymentRefundEvents.WithLabelValues("refunded").Inc()
	logger.Info("payment-level cancel refunded",
		"order_no", event.OrderNo, "payment_no", event.PaymentNo,
		"refund_id", refundID, "amount_cent", amount)
	return nil
}

// paymentCancelCommand decides whether a message is the single T1.6 command.
// The identity, not the reason, is authoritative. The legacy no-refund_id
// cancel-after-confirm form is mapped to the same deterministic identity so an
// in-flight old message cannot be lost.
func paymentCancelCommand(event orderRefundRequested) (string, bool) {
	refundID := strings.TrimSpace(event.RefundID)
	source := strings.TrimSpace(event.Source)
	wantID := "pay:" + event.PaymentNo + ":cancel"
	if refundID != "" {
		if refundID != wantID {
			return "", false
		}
		if source != "" && source != paymentCancelSource {
			return "", false
		}
		return refundID, true
	}
	if source != "" && source != paymentCancelSource {
		return "", false
	}
	if isLegacyCancelAfterConfirm(event.Reason) {
		return wantID, true
	}
	return "", false
}

func isLegacyCancelAfterConfirm(reason string) bool {
	normalized := strings.ToLower(strings.TrimSpace(reason))
	return strings.Contains(normalized, "cancelled after inventory confirm") ||
		strings.Contains(normalized, "canceled after inventory confirm")
}

func isRetryableRefundError(code apperr.Code) bool {
	switch code {
	case apperr.CodeUnavailable, apperr.CodeInternal, apperr.CodeRateLimited:
		return true
	default:
		return false
	}
}

func paymentRefundRejectReason(err error) string {
	if strings.Contains(strings.ToLower(apperr.From(err).Message), "exceeds remaining refundable") {
		return "ALREADY_REFUNDED"
	}
	return "PAYMENT_NOT_REFUNDABLE"
}

type missingPaymentNoError struct{}

func (missingPaymentNoError) Error() string { return "order.refund_requested missing payment_no" }

var errMissingPaymentNo = missingPaymentNoError{}
