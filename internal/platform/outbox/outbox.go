package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/metrics"
	"gin-kratos-gorm/internal/platform/mq"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
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
	// Persist the producing trace so the relay can link the publish and the
	// consumer's span back to the original request. Store NULL when tracing is
	// off instead of an empty string.
	tp := traceparentFrom(ctx)
	traceparent := sql.NullString{String: tp, Valid: tp != ""}
	_, err = tx.ExecContext(ctx, `INSERT INTO outbox_events (event_id, topic, aggregate_type, aggregate_id, event_type, payload, traceparent, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, e.ID, e.Topic, e.AggregateType, e.AggregateID, e.EventType, string(payload), traceparent, time.Now())
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
	// leaseTTL must exceed publishTimeout so a live relay does not lose rows.
	leaseTTL       = 60 * time.Second
	publishTimeout = 10 * time.Second
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

type claimedEvent struct {
	id          int64
	topic       string
	payload     []byte
	attempts    int
	traceparent string
}

// traceparentFrom serialises the active span context into a W3C traceparent
// header, or returns "" when no span is active.
func traceparentFrom(ctx context.Context) string {
	header := http.Header{}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(header))
	return strings.TrimSpace(header.Get("traceparent"))
}

// relayOwner identifies this process as the lease holder.
var relayOwner = func() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	owner := fmt.Sprintf("%s:%d", host, os.Getpid())
	if len(owner) > 128 {
		owner = owner[:128]
	}
	return owner
}()

// relayOnce claims a batch under a short lease, commits (releasing row locks),
// then publishes outside the transaction. A crashed relay leaves rows leased
// until leaseTTL expires, so another replica retries them (at-least-once).
func relayOnce(ctx context.Context, db *sql.DB, client *mq.Client, logger *slog.Logger) (int, error) {
	events, err := claimBatch(ctx, db)
	if err != nil {
		return 0, err
	}
	published := 0
	for _, event := range events {
		publishCtx, span := publishSpan(event)
		callCtx, cancel := context.WithTimeout(publishCtx, publishTimeout)
		err := client.Publish(callCtx, event.topic, event.payload)
		cancel()
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
		if err != nil {
			metrics.OutboxPublishErrors.Inc()
			if ferr := markFailed(ctx, db, event, err, logger); ferr != nil {
				return published, ferr
			}
			continue
		}
		ok, err := markPublished(ctx, db, event.id)
		if err != nil {
			return published, err
		}
		if ok {
			metrics.OutboxPublished.Inc()
			published++
		}
	}
	return published, nil
}

// claimBatch leases pending rows. The transaction only performs the claim and
// commits before any network call, so a slow NATS never holds MySQL locks.
func claimBatch(ctx context.Context, db *sql.DB) ([]claimedEvent, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT id, topic, payload, attempts, traceparent FROM outbox_events
		WHERE published_at IS NULL AND dead_lettered_at IS NULL
		  AND (next_attempt_at IS NULL OR next_attempt_at <= NOW(3))
		  AND (lease_until IS NULL OR lease_until < NOW(3))
		ORDER BY id LIMIT 100 FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return nil, err
	}
	var events []claimedEvent
	for rows.Next() {
		var event claimedEvent
		var traceparent sql.NullString
		if err := rows.Scan(&event.id, &event.topic, &event.payload, &event.attempts, &traceparent); err != nil {
			rows.Close()
			return nil, err
		}
		event.traceparent = traceparent.String
		events = append(events, event)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, tx.Commit()
	}
	placeholders := make([]string, len(events))
	args := make([]any, 0, len(events)+2)
	args = append(args, relayOwner, int(leaseTTL.Seconds()))
	for i, event := range events {
		placeholders[i] = "?"
		args = append(args, event.id)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE outbox_events
		SET lease_owner = ?, lease_until = DATE_ADD(NOW(3), INTERVAL ? SECOND)
		WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return events, nil
}

// markPublished clears the lease and records success; the lease guard makes a
// reclaimed row (another owner) a no-op instead of a double completion.
func markPublished(ctx context.Context, db *sql.DB, id int64) (bool, error) {
	res, err := db.ExecContext(ctx, `UPDATE outbox_events
		SET published_at = NOW(3), attempts = attempts + 1, next_attempt_at = NULL,
		    lease_owner = NULL, lease_until = NULL
		WHERE id = ? AND lease_owner = ?`, id, relayOwner)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func markFailed(ctx context.Context, db *sql.DB, event claimedEvent, cause error, logger *slog.Logger) error {
	attempts := event.attempts + 1
	if attempts >= maxAttempts {
		if _, err := db.ExecContext(ctx, `UPDATE outbox_events
			SET attempts = ?, last_error = ?, dead_lettered_at = NOW(3),
			    lease_owner = NULL, lease_until = NULL
			WHERE id = ? AND lease_owner = ?`, attempts, truncateError(cause), event.id, relayOwner); err != nil {
			return err
		}
		metrics.OutboxDeadLettered.Inc()
		logger.Error("outbox event dead-lettered", "event_id", event.id, "topic", event.topic, "attempts", attempts, "error", cause)
		return nil
	}
	if _, err := db.ExecContext(ctx, `UPDATE outbox_events
		SET attempts = ?, last_error = ?, lease_owner = NULL, lease_until = NULL,
		    next_attempt_at = DATE_ADD(NOW(3), INTERVAL ? SECOND)
		WHERE id = ? AND lease_owner = ?`,
		attempts, truncateError(cause), int(backoff(attempts).Seconds()), event.id, relayOwner); err != nil {
		return err
	}
	return nil
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

// publishSpan starts a producer span for one event. When the row carries a
// traceparent, the span links to it (the original request already finished, so
// a link is more accurate than a parent/child relationship).
func publishSpan(event claimedEvent) (context.Context, trace.Span) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination", event.topic),
		),
	}
	if event.traceparent != "" {
		header := http.Header{}
		header.Set("traceparent", event.traceparent)
		remote := otel.GetTextMapPropagator().Extract(context.Background(), propagation.HeaderCarrier(header))
		if spanContext := trace.SpanContextFromContext(remote); spanContext.IsValid() {
			opts = append(opts, trace.WithLinks(trace.Link{SpanContext: spanContext}))
		}
	}
	return otel.Tracer("outbox").Start(context.Background(), "publish "+event.topic, opts...)
}
