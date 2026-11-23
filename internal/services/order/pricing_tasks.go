package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

const (
	pricingTaskPending    = "PENDING"
	pricingTaskProcessing = "PROCESSING"
	pricingTaskDone       = "DONE"
	pricingTaskFailed     = "FAILED_PERMANENT"
	pricingTaskMaxAttempt = 10
)

// ensurePricingConfirmationTask is written in the settlement transaction. It
// makes coupon/promotion confirmation recoverable even if the post-commit
// best-effort call fails.
func ensurePricingConfirmationTask(ctx context.Context, tx *sql.Tx, record *orderRecord) error {
	payload, err := json.Marshal(map[string]any{"quote_id": record.QuoteID, "order_no": record.OrderNo, "user_id": record.UserID})
	if err != nil {
		return apperr.Internal("marshal pricing task", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO pricing_confirmation_tasks
		(order_no, state, attempts, payload, version)
		VALUES (?, ?, 0, ?, 1)
		ON DUPLICATE KEY UPDATE order_no = order_no`,
		record.OrderNo, pricingTaskPending, payload)
	return err
}

func (s *Service) runPricingConfirmationOnce(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var orderNo, payloadRaw string
	err = tx.QueryRowContext(ctx, `SELECT order_no, payload FROM pricing_confirmation_tasks
		WHERE state = ? AND (attempts = 0 OR updated_at < DATE_SUB(NOW(3), INTERVAL 1 MINUTE))
		ORDER BY created_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED`, pricingTaskPending).
		Scan(&orderNo, &payloadRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pricing_confirmation_tasks
		SET state = ?, attempts = attempts + 1, version = version + 1 WHERE order_no = ?`,
		pricingTaskProcessing, orderNo); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	var payload struct {
		QuoteID string `json:"quote_id"`
		UserID  int64  `json:"user_id"`
	}
	_ = json.Unmarshal([]byte(payloadRaw), &payload)
	confirmErr := s.confirmPricing(ctx, orderNo, payload.QuoteID, payload.UserID)
	if confirmErr != nil {
		_, _ = s.db.ExecContext(ctx, `UPDATE pricing_confirmation_tasks
			SET state = IF(attempts >= ?, ?, ?), last_error = ?, version = version + 1 WHERE order_no = ?`,
			pricingTaskMaxAttempt, pricingTaskFailed, pricingTaskPending, truncate(confirmErr.Error(), 512), orderNo)
		return confirmErr
	}
	_, err = s.db.ExecContext(ctx, `UPDATE pricing_confirmation_tasks
		SET state = ?, last_error = '', version = version + 1 WHERE order_no = ?`, pricingTaskDone, orderNo)
	return err
}

func (s *Service) confirmPricing(ctx context.Context, orderNo, quoteID string, userID int64) error {
	if s.pricing == nil || quoteID == "" {
		return nil
	}
	quote, err := s.pricing.GetQuote(ctx, &pb.GetQuoteRequest{QuoteId: quoteID})
	if err != nil {
		return err
	}
	couponID := quote.GetCouponId()
	if couponID != "" {
		if _, err := s.pricing.ConsumeCoupon(ctx, &pb.LockCouponRequest{
			QuoteId: quoteID, CouponId: couponID, UserId: userID,
		}); err != nil {
			return err
		}
	}
	if s.promotion != nil {
		if _, err := s.promotion.ConfirmPromotions(ctx, &pb.PromotionOrderRequest{OrderNo: orderNo}); err != nil {
			return err
		}
	}
	return nil
}

// RunPricingConfirmationWorker is opt-in (PRICING_CONFIRMATION_WORKER_ENABLED).
func RunPricingConfirmationWorker(ctx context.Context, svc *Service, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := svc.runPricingConfirmationOnce(ctx); err != nil && logger != nil {
				logger.Error("pricing confirmation worker", "error", err)
			}
		}
	}
}
