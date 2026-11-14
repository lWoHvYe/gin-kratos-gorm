package seckill

import (
	"context"
	"time"
)

type expiredQualification struct {
	activityID int64
	userID     int64
	tokenHash  string
}

// RunQualificationExpiry releases Redis stock held by abandoned qualifications.
// The DB row is the durable state; the Redis compensation is idempotent by
// token hash.
func (s *Service) RunQualificationExpiry(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.expireQualifications(ctx)
		}
	}
}

func (s *Service) expireQualifications(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx, `SELECT activity_id, user_id, token_hash
		FROM seckill_qualifications WHERE status IN ('ISSUED','FAILED') AND expires_at <= NOW(3)
		ORDER BY id LIMIT 200`)
	if err != nil {
		s.log.Warn("query expired seckill qualifications", "error", err)
		return
	}
	var expired []expiredQualification
	for rows.Next() {
		var q expiredQualification
		if err := rows.Scan(&q.activityID, &q.userID, &q.tokenHash); err == nil {
			expired = append(expired, q)
		}
	}
	rows.Close()
	for _, q := range expired {
		if err := s.releaseQualification(ctx, q.activityID, q.tokenHash); err != nil {
			s.log.Warn("skip expired qualification with held stock", "activity_id", q.activityID, "error", err)
			continue
		}
		s.compensate(ctx, q.activityID, q.userID, q.tokenHash)
		// FAILED rows need the same terminal transition: otherwise every scan
		// retries the release and compensation for a row that can never change.
		if _, err := s.db.ExecContext(ctx, `UPDATE seckill_qualifications
			SET status='EXPIRED' WHERE token_hash=? AND status IN ('ISSUED','FAILED')`, q.tokenHash); err != nil {
			s.log.Warn("mark seckill qualification expired", "activity_id", q.activityID, "error", err)
		}
	}
	if len(expired) > 0 {
		s.log.Info("expired seckill qualifications", "count", len(expired))
	}
	if err := rows.Err(); err != nil {
		s.log.Warn("iterate expired seckill qualifications", "error", err)
	}
}
