package seckill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/metrics"

	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RunOrderWorker consumes the durable seckill order queue. The command is
// acknowledged only after the order exists and the qualification is ORDERED.
// RunOrderWorker consumes the durable seckill order queue. The command is
// acknowledged only after the order exists and the qualification is ORDERED.
// It uses the same JetStream push/durable pattern as the other services in this
// repository; MaxAckPending bounds in-flight work.
func (s *Service) RunOrderWorker(ctx context.Context, concurrency int) {
	if s.mq == nil || s.mq.JS == nil {
		s.log.Error("seckill worker has no jetstream client")
		return
	}
	if concurrency <= 0 {
		concurrency = 4
	}
	handler := func(msg *nats.Msg) {
		s.handleCommand(ctx, msg)
	}
	_, err := s.mq.JS.QueueSubscribe("seckill.order.requested.>", "seckill-order-worker", handler,
		nats.Durable("seckill-order-worker"), nats.ManualAck(), nats.AckExplicit(),
		nats.MaxAckPending(concurrency*8), nats.AckWait(30*time.Second), nats.MaxDeliver(5))
	if err != nil {
		s.log.Error("subscribe seckill order queue", "error", err)
	}
}

func (s *Service) handleCommand(ctx context.Context, msg *nats.Msg) {
	var cmd orderCommand
	if err := json.Unmarshal(msg.Data, &cmd); err != nil {
		s.log.Error("decode seckill command", "error", err)
		_ = msg.Term()
		return
	}
	if cmd.TokenHash == "" || cmd.ActivityID <= 0 || cmd.UserID <= 0 {
		s.log.Error("invalid seckill command", "subject", msg.Subject)
		_ = msg.Term()
		return
	}
	order, err := s.orders.CreateSeckillOrder(ctx, &pb.CreateSeckillOrderRequest{
		ActivityId: cmd.ActivityID, UserId: cmd.UserID, SkuId: cmd.SKU, PriceCent: cmd.PriceCent,
		Quantity: cmd.Quantity, TokenHash: cmd.TokenHash, AddressId: cmd.AddressID,
		ReceiverName: cmd.ReceiverName, ReceiverPhone: cmd.ReceiverPhone, ReceiverAddress: cmd.ReceiverAddress,
		ExpireAtUnix: cmd.ExpiresAtUnix,
	})
	if err != nil {
		if transient(err) && delivery(msg) < 5 {
			_ = msg.NakWithDelay(5 * time.Second)
			return
		}
		metrics.SeckillOrderCommands.WithLabelValues("dlq").Inc()
		s.publishDLQ(ctx, cmd, err)
		s.failQualification(ctx, cmd, err)
		_ = msg.Term()
		return
	}
	paymentNo, payErr := s.ensurePayment(ctx, cmd, order)
	if payErr != nil {
		if transient(payErr) && delivery(msg) < 5 {
			_ = msg.NakWithDelay(5 * time.Second)
			return
		}
		s.log.Error("create seckill payment intent", "order_no", order.GetOrderNo(), "error", payErr)
	}
	s.markOrdered(ctx, cmd, order.GetOrderNo(), paymentNo)
	metrics.SeckillOrderCommands.WithLabelValues("succeeded").Inc()
	_ = msg.Ack()
}

func (s *Service) ensurePayment(ctx context.Context, cmd orderCommand, order *pb.Order) (string, error) {
	if s.payment == nil {
		return "", apperr.Unavailable("payment client is not configured", nil)
	}
	amount := order.GetPayCent()
	if amount <= 0 {
		amount = cmd.PriceCent * int64(cmd.Quantity)
	}
	intent, err := s.payment.CreateIntent(ctx, &pb.CreateIntentRequest{
		OrderNo: order.GetOrderNo(), UserId: cmd.UserID, AmountCent: amount, Channel: "MOCK",
	})
	if err != nil {
		return "", err
	}
	return intent.GetPaymentNo(), nil
}

func (s *Service) markOrdered(ctx context.Context, cmd orderCommand, orderNo, paymentNo string) {
	if _, err := s.db.ExecContext(ctx, `UPDATE seckill_qualifications
		SET status='ORDERED', order_no=?, payment_no=? WHERE token_hash=? AND status IN ('ISSUED','ORDERING')`, orderNo, paymentNo, cmd.TokenHash); err != nil {
		s.log.Error("mark qualification ordered", "activity_id", cmd.ActivityID, "error", err)
	}
	if s.cache != nil {
		key := fmt.Sprintf("seckill:{%d}:tokens", cmd.ActivityID)
		if err := s.cache.HSet(ctx, key, cmd.TokenHash, "ORDERED").Err(); err != nil {
			s.log.Warn("mark redis qualification ordered", "activity_id", cmd.ActivityID, "error", err)
		}
	}
}

func (s *Service) failQualification(ctx context.Context, cmd orderCommand, cause error) {
	s.log.Error("seckill order command failed", "activity_id", cmd.ActivityID, "token_hash", cmd.TokenHash, "error", cause)
	if _, err := s.db.ExecContext(ctx, `UPDATE seckill_qualifications
		SET status='FAILED' WHERE token_hash=? AND status IN ('ISSUED','ORDERING')`, cmd.TokenHash); err != nil {
		s.log.Error("mark qualification failed", "error", err)
	}
	// Keep the inventory hold and Redis unit until the qualification expiry
	// worker runs. This lets an operator replay a DLQ command without
	// re-acquiring stock; a permanent failure is released by expiry.

}

func transient(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		var ae *apperr.Error
		if errors.As(err, &ae) {
			return ae.Code == apperr.CodeUnavailable || ae.Code == apperr.CodeInternal
		}
		return false
	}
	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Aborted:
		return true
	default:
		return false
	}
}

func delivery(msg *nats.Msg) uint64 {
	if meta, err := msg.Metadata(); err == nil {
		return meta.NumDelivered
	}
	return 1
}

func (s *Service) publishDLQ(ctx context.Context, cmd orderCommand, cause error) {
	if s.mq == nil || s.mq.JS == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"command": cmd, "error": cause.Error(), "failed_at": time.Now().Unix(),
	})
	if err != nil {
		return
	}
	msg := nats.NewMsg(fmt.Sprintf("seckill.order.dlq.%d", cmd.ActivityID))
	msg.Data = payload
	msg.Header.Set("Nats-Msg-Id", "dlq-"+cmd.TokenHash)
	if _, err := s.mq.JS.PublishMsg(msg, nats.Context(ctx)); err != nil {
		s.log.Error("publish seckill DLQ", "activity_id", cmd.ActivityID, "error", err)
	}
}
