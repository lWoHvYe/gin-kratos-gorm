package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/metrics"
	"gin-kratos-gorm/internal/platform/mq"
)

type Event struct {
	ID            string
	Topic         string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       any
}

func Append(ctx context.Context, tx *sql.Tx, e Event) error {
	payload, err := encodePayload(e.Payload)
	if err != nil {
		return err
	}
	if e.ID == "" {
		e.ID = id.Event()
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO outbox_events (event_id, topic, aggregate_type, aggregate_id, event_type, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, e.ID, e.Topic, e.AggregateType, e.AggregateID, e.EventType, string(payload), time.Now())
	if err != nil {
		return fmt.Errorf("append outbox: %w", err)
	}
	return nil
}

// Relay claims pending rows with SKIP LOCKED, publishes them and marks them done.
// It is safe to run on every replica.
func Relay(ctx context.Context, db *sql.DB, client *mq.Client, logger *slog.Logger, interval time.Duration) {
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
			if n, err := relayOnce(ctx, db, client); err != nil {
				logger.Error("outbox relay", "error", err)
			} else if n > 0 {
				logger.Info("outbox relay", "published", n)
			}
		}
	}
}

func relayOnce(ctx context.Context, db *sql.DB, client *mq.Client) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT id, topic, payload, attempts FROM outbox_events
		WHERE published_at IS NULL ORDER BY id LIMIT 100 FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return 0, err
	}
	type row struct {
		id       int64
		topic    string
		payload  []byte
		attempts int
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.topic, &r.payload, &r.attempts); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	n := 0
	for _, r := range list {
		if err := client.Publish(ctx, r.topic, r.payload); err != nil {
			metrics.OutboxPublishErrors.Inc()
			_, _ = tx.ExecContext(ctx, `UPDATE outbox_events SET attempts = attempts + 1, last_error = ? WHERE id = ?`, err.Error(), r.id)
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE outbox_events SET published_at = ?, attempts = attempts + 1 WHERE id = ?`, time.Now(), r.id); err != nil {
			return n, err
		}
		metrics.OutboxPublished.Inc()
		n++
	}
	return n, tx.Commit()
}

// encodePayload accepts already-marshalled JSON as well as Go values. Passing a
// []byte to json.Marshal would base64-encode it, so callers that pre-marshal an
// event must not silently turn the outbox row into a JSON string.
func encodePayload(v any) ([]byte, error) {
	var payload []byte
	switch value := v.(type) {
	case json.RawMessage:
		payload = value
	case []byte:
		payload = value
	default:
		var err error
		payload, err = json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal event: %w", err)
		}
	}
	if !json.Valid(payload) {
		return nil, fmt.Errorf("marshal event: payload is not valid JSON")
	}
	return payload, nil
}
