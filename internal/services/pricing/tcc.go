package pricing

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/outbox"
)

func (s *Service) TccTryConsume(ctx context.Context, req *pb.TccPaymentRequest) (*pb.Empty, error) {
	target, err := s.couponTarget(ctx, &pb.LockCouponRequest{QuoteId: req.GetQuoteId(), UserId: req.GetUserId()})
	if err != nil {
		return nil, err
	}
	if target == nil {
		return &pb.Empty{}, nil
	}
	switch target.coupon.Status {
	case couponLocked, couponUsed:
		return &pb.Empty{}, nil
	default:
		return nil, apperr.Failed(fmt.Sprintf("coupon is %s", target.coupon.Status))
	}
}

func (s *Service) TccConfirmConsume(ctx context.Context, req *pb.TccPaymentRequest) (*pb.Empty, error) {
	_, err := s.ConsumeCoupon(ctx, &pb.LockCouponRequest{QuoteId: req.GetQuoteId(), UserId: req.GetUserId()})
	return &pb.Empty{}, err
}

func (s *Service) TccCancelConsume(ctx context.Context, req *pb.TccPaymentRequest) (*pb.Empty, error) {
	target, err := s.couponTarget(ctx, &pb.LockCouponRequest{QuoteId: req.GetQuoteId(), UserId: req.GetUserId()})
	if err != nil {
		return nil, err
	}
	if target == nil {
		return &pb.Empty{}, nil
	}
	if err := s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE coupons SET status = ?, order_no = '', version = version + 1
			WHERE id = ? AND status IN (?, ?) AND order_no = ?`,
			couponAvailable, target.coupon.ID, couponLocked, couponUsed, target.orderNo)
		if err != nil {
			return apperr.Internal("revert coupon", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			cur, err := getCouponForUpdate(ctx, tx, target.coupon.ID)
			if err != nil {
				return err
			}
			if cur.Status == couponAvailable {
				return nil
			}
			return apperr.Failed(fmt.Sprintf("coupon cannot be reverted from status %s", cur.Status))
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic: topicCouponUnlocked, AggregateType: "coupon", AggregateID: fmt.Sprintf("%d", target.coupon.ID),
			EventType: "coupon.unlocked", Payload: target.eventPayload(),
		})
	}); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *Service) TccQueryConsume(ctx context.Context, req *pb.TccPaymentRequest) (*pb.TccQueryReply, error) {
	if strings.TrimSpace(req.GetQuoteId()) == "" {
		return &pb.TccQueryReply{Prepared: false}, nil
	}
	target, err := s.couponTarget(ctx, &pb.LockCouponRequest{QuoteId: req.GetQuoteId(), UserId: req.GetUserId()})
	if err != nil {
		return nil, err
	}
	if target == nil {
		return &pb.TccQueryReply{Prepared: true}, nil
	}
	return &pb.TccQueryReply{Prepared: target.coupon.Status == couponLocked || target.coupon.Status == couponUsed}, nil
}
