package payment

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// GetRefundByKey returns the persisted refund fact keyed by
// (payment_no, refund_id). It is the read-only lazy-reconciliation API for
// refund admission (C-04 §1.2): the caller must key on refund_id, never guess
// it from refund_no.
//
// This method performs no writes: no refund row, intent aggregate, batch ledger
// or payment.refunded event is created or changed. A missing key is NOT_FOUND.
func (s *Service) GetRefundByKey(ctx context.Context, req *pb.GetRefundByKeyRequest) (*pb.Refund, error) {
	paymentNo := strings.TrimSpace(req.GetPaymentNo())
	refundID := strings.TrimSpace(req.GetRefundId())
	if paymentNo == "" {
		return nil, apperr.Invalid("payment_no is required")
	}
	if refundID == "" {
		return nil, apperr.Invalid("refund_id is required")
	}
	ref, err := newRepo(s.db).refundByKey(ctx, s.db, paymentNo, refundID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("refund not found")
	}
	if err != nil {
		return nil, apperr.Internal("get refund by key", err)
	}
	return ref.toPB(), nil
}
