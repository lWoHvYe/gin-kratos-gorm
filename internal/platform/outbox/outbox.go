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

const (
	// maxAttempts bounds retries before a row is dead-lettered for an operator.
	maxAttempts = 20
	// pruneAfter removes terminal rows so the table cannot grow forever.
	pruneAfter = 7 * 24 * time.Hour
)

// Relay claims pending rows with SKIP LOCKED, publishes them and marks them done.
// It is safe to run on every replica. Failed rows back off exponentially and are
// dead-lettered after maxAttempts instead of blocking the head of the queue.
func Relay(ctx context.Context, db *sql.DB, client *mq.Client, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	lastPrune := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := relayOnce(ctx, db, client, logger); err != nil {
				logger.Error("outbox relay", "error", err)
			} else if n > 0 {
				logger.Info("outbox relay", "published", n)
			}
			if time.Since(lastPrune) >= time.Hour {
				if n, err := pruneOnce(ctx, db); err != nil {
					logger.Error("outbox prune", "error", err)
				} else if n > 0 {
					logger.Info("outbox prune", "deleted", n)
				}
				lastPrune = time.Now()
			}
		}
	}
}

// backoff is exponential (2s, 4s, ...) capped at five minutes.
func backoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 8 {
		attempts = 8
	}
	d := time.Duration(1<<uint(attempts)) * time.Second
	if d > 5*time.Minute {
		return 5 * time.Minute
	}
	return d
}

func relayOnce(ctx context.Context, db *sql.DB, client *mq.Client, logger *slog.Logger) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT id, topic, payload, attempts FROM outbox_events
		WHERE published_at IS NULL AND dead_lettered_at IS NULL
		  AND (next_attempt_at IS NULL OR next_attempt_at <= NOW(3))
		ORDER BY id LIMIT 100 FOR UPDATE SKIP LOCKED`)
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
			attempts := r.attempts + 1
			if attempts >= maxAttempts {
				if _, uerr := tx.ExecContext(ctx, `UPDATE outbox_events
					SET attempts = ?, last_error = ?, dead_lettered_at = NOW(3) WHERE id = ?`,
					attempts, truncateError(err), r.id); uerr != nil {
					return n, uerr
				}
				metrics.OutboxDeadLettered.Inc()
				logger.Error("outbox event dead-lettered", "event_id", r.id, "topic", r.topic, "attempts", attempts, "error", err)
				continue
			}
			if _, uerr := tx.ExecContext(ctx, `UPDATE outbox_events
				SET attempts = ?, last_error = ?, next_attempt_at = DATE_ADD(NOW(3), INTERVAL ? SECOND) WHERE id = ?`,
				attempts, truncateError(err), int(backoff(attempts).Seconds()), r.id); uerr != nil {
				return n, uerr
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE outbox_events
			SET published_at = NOW(3), attempts = attempts + 1, next_attempt_at = NULL WHERE id = ?`, r.id); err != nil {
			return n, err
		}
		metrics.OutboxPublished.Inc()
		n++
	}
	return n, tx.Commit()
}

// truncateError keeps last_error within its VARCHAR(1024) column.
func truncateError(err error) string {
	msg := err.Error()
	if len(msg) <= 1024 {
		return msg
	}
	return msg[:1024]
}

// pruneOnce deletes terminal rows older than pruneAfter in bounded batches.
func pruneOnce(ctx context.Context, db *sql.DB) (int64, error) {
	var total int64
	for i := 0; i < 5; i++ {
		// pruneAfter is a constant, so formatting it keeps the statement free of
		// placeholders inside INTERVAL, which some MySQL versions reject.
		res, err := db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM outbox_events
			WHERE (published_at IS NOT NULL OR dead_lettered_at IS NOT NULL)
			  AND COALESCE(published_at, dead_lettered_at) < DATE_SUB(NOW(3), INTERVAL %d DAY)
			LIMIT 1000`, int(pruneAfter.Hours()/24)))
		if err != nil {
			return total, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, err
		}
		total += n
		if n < 1000 {
			break
		}
	}
	return total, nil
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
