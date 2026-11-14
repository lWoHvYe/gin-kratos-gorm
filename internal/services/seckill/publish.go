package seckill

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

const seckillStream = "SECKILL"

type orderCommand struct {
	ActivityID      int64  `json:"activity_id"`
	UserID          int64  `json:"user_id"`
	SKU             int64  `json:"sku_id"`
	PriceCent       int64  `json:"price_cent"`
	Quantity        int32  `json:"quantity"`
	TokenHash       string `json:"token_hash"`
	AddressID       int64  `json:"address_id"`
	ReceiverName    string `json:"receiver_name"`
	ReceiverPhone   string `json:"receiver_phone"`
	ReceiverAddress string `json:"receiver_address"`
	ExpiresAtUnix   int64  `json:"expires_at_unix"`
}

// EnsureStream creates the SECKILL WorkQueue stream if it does not exist.
func EnsureStream(client *mq.Client) error { return ensureSeckillStream(client) }

func ensureSeckillStream(client *mq.Client) error {
	if client == nil || client.JS == nil {
		return fmt.Errorf("nats jetstream is not configured")
	}
	if _, err := client.JS.StreamInfo(seckillStream); err == nil {
		return nil
	}
	replicas := 1
	if v := strings.TrimSpace(os.Getenv("SECKILL_STREAM_REPLICAS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			replicas = n
		}
	}
	_, err := client.JS.AddStream(&nats.StreamConfig{
		Name: seckillStream, Subjects: []string{"seckill.>"}, Retention: nats.WorkQueuePolicy,
		Storage: nats.FileStorage, MaxAge: 24 * time.Hour, Replicas: replicas,
	})
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "already") || strings.Contains(msg, "in use") {
			return nil
		}
		return fmt.Errorf("add seckill stream: %w", err)
	}
	return nil
}

func (s *Service) publishQualification(ctx context.Context, q *qualification, a *activity) error {
	if s.mq == nil || s.mq.JS == nil {
		return apperr.Unavailable("nats is not configured", fmt.Errorf("jetstream client is nil"))
	}
	cmd := orderCommand{
		ActivityID: q.activityID, UserID: q.userID, SKU: a.skuID, PriceCent: a.priceCent, Quantity: 1,
		TokenHash: q.tokenHash, AddressID: q.addressID, ReceiverName: q.receiverName,
		ReceiverPhone: q.receiverPhone, ReceiverAddress: q.receiverAddr, ExpiresAtUnix: q.expiresAt.Unix(),
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return apperr.Internal("encode seckill order command", err)
	}
	msg := nats.NewMsg(fmt.Sprintf("seckill.order.requested.%d", q.activityID))
	msg.Data = payload
	msg.Header.Set("Nats-Msg-Id", q.tokenHash)
	if err := s.mq.PublishMsg(ctx, msg); err != nil {
		return apperr.Unavailable("publish seckill order command", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE seckill_qualifications SET published_at=NOW(3)
		WHERE token_hash=? AND published_at IS NULL`, q.tokenHash); err != nil {
		return apperr.Internal("mark seckill command published", err)
	}
	q.publishedAt = sql.NullTime{Time: time.Now(), Valid: true}
	return nil
}

// RunRepublisher republishes ISSUED qualifications whose first publish was lost
// (for example the seckill pod died after the DB commit). It is a small outbox
// safety net until the stream is fed by a transactional outbox.
func (s *Service) RunRepublisher(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.republishPending(ctx)
		}
	}
}

func (s *Service) republishPending(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx, `SELECT q.activity_id, q.user_id, q.address_id, q.receiver_name,
		q.receiver_phone, q.receiver_address, q.token_hash, q.status, q.order_no, q.expires_at, q.published_at,
		a.sku_id, a.price_cent
		FROM seckill_qualifications q JOIN seckill_activities a ON a.id=q.activity_id
		WHERE q.status='ISSUED' AND q.published_at IS NULL AND q.expires_at > NOW(3) ORDER BY q.id LIMIT 100`)
	if err != nil {
		s.log.Warn("query pending seckill commands", "error", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var (
			q         qualification
			skuID     int64
			priceCent int64
		)
		if err := rows.Scan(&q.activityID, &q.userID, &q.addressID, &q.receiverName, &q.receiverPhone,
			&q.receiverAddr, &q.tokenHash, &q.status, &q.orderNo, &q.expiresAt, &q.publishedAt, &skuID, &priceCent); err != nil {
			continue
		}
		a := &activity{id: q.activityID, skuID: skuID, priceCent: priceCent}
		if err := s.publishQualification(ctx, &q, a); err != nil {
			s.log.Warn("republish seckill command", "activity_id", q.activityID, "error", err)
			continue
		}
	}
}
