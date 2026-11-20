package ai

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/id"

	"github.com/redis/go-redis/v9"
)

// RunOpsBriefScheduler generates the deterministic daily brief once per day.
// Delivery is outside business transactions; a failure only skips the brief and
// the date key is released for a retry.
func RunOpsBriefScheduler(ctx context.Context, cache redis.UniversalClient, svc *Service, logger *slog.Logger, interval time.Duration, webhookURL, webhookSecret string) {
	if interval <= 0 {
		interval = time.Hour
	}
	run := func() {
		day := time.Now().Format("2006-01-02")
		doneKey := "ai:ops:brief:" + day
		lockKey := "ai:ops:brief:lock:" + day
		if exists, _ := cache.Exists(ctx, doneKey).Result(); exists > 0 {
			return
		}
		// A crash while generating must not consume the day. The short-lived
		// lock is released by expiry; only a successful generation writes done.
		ok, err := cache.SetNX(ctx, lockKey, "1", 30*time.Minute).Result()
		if err != nil || !ok {
			return
		}
		defer cache.Del(ctx, lockKey)
		brief, err := svc.OpsBrief(ctx, nil)
		if err != nil {
			logger.Warn("ops brief generation failed", "error", err)
			return
		}
		raw, _ := json.Marshal(brief)
		if webhookURL != "" {
			if err := postOpsBriefWebhook(ctx, &http.Client{Timeout: 10 * time.Second}, webhookURL, webhookSecret, raw); err != nil {
				logger.Warn("ops brief webhook failed", "error", err)
				return
			}
		}
		pipe := cache.TxPipeline()
		pipe.Set(ctx, "ai:ops:latest", raw, 48*time.Hour)
		pipe.Set(ctx, doneKey, "1", 48*time.Hour)
		if _, err := pipe.Exec(ctx); err != nil {
			logger.Warn("publish ops brief failed", "day", day, "error", err)
			return
		}
		logger.Info("ops brief generated", "day", day, "orders", brief.GetTotalOrders(), "low_stock", brief.GetLowStock())
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

// webhookTimestampHeader and webhookSignatureHeader authenticate the optional
// ops-brief delivery. The signature covers "<timestamp>.<body>" so a captured
// payload cannot be replayed with a fresh timestamp, and the timestamp lets the
// receiver reject stale deliveries. The secret is required: an unsigned
// webhook is refused instead of silently downgrading to unauthenticated POSTs.
const (
	webhookTimestampHeader = "X-Mall-Timestamp"
	webhookSignatureHeader = "X-Mall-Signature"
	webhookDeliveryHeader  = "X-Mall-Delivery"
	webhookEventHeader     = "X-Mall-Event"
)

func webhookSignature(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func postOpsBriefWebhook(ctx context.Context, client *http.Client, url, secret string, body []byte) error {
	secret = strings.TrimSpace(secret)
	if len(secret) < 16 {
		return fmt.Errorf("OPS_BRIEF_WEBHOOK_SECRET must be at least 16 characters when the webhook URL is set")
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	delivery := id.Event()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "mall-ai-ops-brief/1")
	req.Header.Set(webhookEventHeader, "ops.brief")
	req.Header.Set(webhookDeliveryHeader, delivery)
	req.Header.Set(webhookTimestampHeader, timestamp)
	req.Header.Set(webhookSignatureHeader, webhookSignature(secret, timestamp, body))
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("deliver webhook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook rejected with status %d", resp.StatusCode)
	}
	return nil
}
