package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/outbox"
)

// CreateSeckillOrder creates an order from a previously held seckill
// qualification. Stock is already held in the seckill bucket, so the inventory
// branch creates an order reservation without incrementing reserved again.
// The token hash is the idempotency key; retries return the same order.
func (s *Service) CreateSeckillOrder(ctx context.Context, req *pb.CreateSeckillOrderRequest) (*pb.Order, error) {
	if req.GetActivityId() <= 0 || req.GetUserId() <= 0 || req.GetSkuId() <= 0 || req.GetPriceCent() <= 0 ||
		req.GetQuantity() <= 0 || strings.TrimSpace(req.GetTokenHash()) == "" {
		return nil, apperr.Invalid("activity_id, user_id, sku_id, price_cent, quantity and token_hash are required")
	}
	tokenHash := strings.TrimSpace(req.GetTokenHash())
	if existing, err := findOrderByClientRequest(ctx, s.db, req.GetUserId(), tokenHash); err != nil {
		return nil, apperr.Internal("load seckill order by token", err)
	} else if existing != nil {
		return s.replyFor(ctx, s.db, existing)
	}
	if s.inventory == nil {
		return nil, apperr.Unavailable("inventory client is not configured", nil)
	}
	orderNo := id.Order()
	ttl := int64(orderExpireWindow.Seconds())
	if _, err := s.inventory.CreateSeckillReservation(ctx, &pb.CreateSeckillReservationRequest{
		ActivityId: req.GetActivityId(), OrderNo: orderNo, SkuId: req.GetSkuId(),
		Quantity: int64(req.GetQuantity()), RefNo: tokenHash, TtlSeconds: ttl,
	}); err != nil {
		return nil, remoteError("create seckill reservation", err)
	}
	expireAt := time.Now().Add(orderExpireWindow)
	if req.GetExpireAtUnix() > 0 {
		expireAt = time.Unix(req.GetExpireAtUnix(), 0)
	}
	record := &orderRecord{
		OrderNo: orderNo, UserID: req.GetUserId(), Status: statusPendingPayment,
		TotalCent:    req.GetPriceCent() * int64(req.GetQuantity()),
		DiscountCent: 0, PromotionDiscountCent: 0, AppliedPromotionsJSON: "[]",
		FreightCent: 0, PayCent: req.GetPriceCent() * int64(req.GetQuantity()),
		ReceiverName: req.GetReceiverName(), ReceiverPhone: req.GetReceiverPhone(),
		ReceiverAddress: req.GetReceiverAddress(), QuoteID: fmt.Sprintf("seckill:%d", req.GetActivityId()),
		ClientRequestID: tokenHash, ExpireAt: expireAt,
	}
	item := &pb.OrderItem{
		SkuId: req.GetSkuId(), Name: fmt.Sprintf("seckill-sku-%d", req.GetSkuId()),
		PriceCent: req.GetPriceCent(), Quantity: req.GetQuantity(), DiscountCent: 0,
	}
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := insertOrder(ctx, tx, record, time.Now()); err != nil {
			if isDuplicateKey(err) {
				return errDuplicateOrder
			}
			return apperr.Internal("insert seckill order", err)
		}
		if err := insertOrderItems(ctx, tx, orderNo, []*pb.OrderItem{item}); err != nil {
			return apperr.Internal("insert seckill order items", err)
		}
		if err := insertStatusLog(ctx, tx, orderNo, "", statusPendingPayment, "seckill order created", "seckill"); err != nil {
			return apperr.Internal("insert seckill order status", err)
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic: topicOrderCreated, AggregateType: "order", AggregateID: orderNo, EventType: "order.created",
			Payload: map[string]any{
				"order_no": orderNo, "user_id": req.GetUserId(), "quote_id": record.QuoteID,
				"pay_cent": record.PayCent, "status": statusPendingPayment, "expire_at": expireAt.Unix(),
				"items": []map[string]any{{"sku_id": item.SkuId, "name": item.Name, "price_cent": item.PriceCent,
					"quantity": item.Quantity, "discount_cent": 0}},
				"source": "SECKILL", "activity_id": req.GetActivityId(),
			},
		})
	})
	if err != nil {
		_, _ = s.inventory.ReleaseSeckillReservation(ctx, &pb.ReleaseSeckillReservationRequest{OrderNo: orderNo, SkuId: req.GetSkuId()})
		if errors.Is(err, errDuplicateOrder) {
			winner, ferr := findOrderByClientRequest(ctx, s.db, req.GetUserId(), tokenHash)
			if ferr == nil && winner != nil {
				return s.replyFor(ctx, s.db, winner)
			}
		}
		return nil, err
	}
	created, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load seckill order", err)
	}
	if created == nil {
		return nil, apperr.Internal("seckill order was not persisted", nil)
	}
	return s.replyFor(ctx, s.db, created)
}
