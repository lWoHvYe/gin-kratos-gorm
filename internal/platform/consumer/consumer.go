// Package consumer runs durable JetStream subscriptions with one retry policy:
// per-message timeout, bounded retries, a dead-letter subject and metrics.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gin-kratos-gorm/internal/platform/metrics"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

// Config describes one durable subscription.
type Config struct {
	Subject        string
	Durable        string
	Queue          string
	DLQSubject     string
	MaxDeliver     int
	AckWait        time.Duration
	MaxAckPending  int
	HandlerTimeout time.Duration
}

// Handler processes one message. Returning an error retries it until MaxDeliver,
// then moves it to the DLQ. Wrap unrecoverable errors with Permanent to skip
// retries.
type Handler func(context.Context, *nats.Msg) error

type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// Permanent marks an error as non-retryable (bad payload, invalid state).
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err: err}
}

// Subscribe registers the durable queue subscription. The setup error is also
// recorded by mq.Client, so a pod that failed to subscribe fails readiness.
func Subscribe(ctx context.Context, client *mq.Client, logger *slog.Logger, cfg Config, handler Handler) error {
	if client == nil || client.JS == nil {
		return fmt.Errorf("consumer %s: jetstream is not configured", cfg.Subject)
	}
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Durable == "" {
		cfg.Durable = cfg.Queue
	}
	if cfg.Queue == "" {
		cfg.Queue = cfg.Durable
	}
	if cfg.MaxDeliver <= 0 {
		cfg.MaxDeliver = 20
	}
	if cfg.AckWait <= 0 {
		cfg.AckWait = 30 * time.Second
	}
	if cfg.HandlerTimeout <= 0 {
		cfg.HandlerTimeout = 10 * time.Second
	}
	dlqSubject := cfg.DLQSubject
	if dlqSubject == "" {
		dlqSubject = cfg.Subject + ".dlq"
	}
	opts := []nats.SubOpt{
		nats.Durable(cfg.Durable), nats.ManualAck(), nats.AckExplicit(),
		nats.MaxDeliver(cfg.MaxDeliver), nats.AckWait(cfg.AckWait),
	}
	if cfg.MaxAckPending > 0 {
		opts = append(opts, nats.MaxAckPending(cfg.MaxAckPending))
	}

	wrapped := func(msg *nats.Msg) {
		// Detach from the worker context so a shutdown drain can finish, and
		// bound each message independently.
		hctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.HandlerTimeout)
		defer cancel()
		err := run(hctx, handler, msg)
		if err == nil {
			metrics.ConsumerOutcomes.WithLabelValues(cfg.Subject, "ok").Inc()
			_ = msg.Ack()
			return
		}
		attempts := delivery(msg)
		if !isPermanent(err) && attempts < uint64(cfg.MaxDeliver) {
			metrics.ConsumerOutcomes.WithLabelValues(cfg.Subject, "retry").Inc()
			logger.Warn("consumer message failed, retrying", "subject", cfg.Subject, "attempt", attempts, "error", err)
			_ = msg.NakWithDelay(5 * time.Second)
			return
		}
		metrics.ConsumerOutcomes.WithLabelValues(cfg.Subject, "dlq").Inc()
		logger.Error("consumer message dead-lettered", "subject", cfg.Subject, "attempts", attempts, "error", err)
		if perr := publishDLQ(client, dlqSubject, cfg.Subject, msg, err, attempts); perr != nil {
			logger.Error("publish consumer DLQ", "subject", cfg.Subject, "error", perr)
			_ = msg.NakWithDelay(5 * time.Second)
			return
		}
		_ = msg.Term()
	}

	_, err := client.QueueSubscribe(cfg.Subject, cfg.Queue, wrapped, opts...)
	return err
}

func run(ctx context.Context, handler Handler, msg *nats.Msg) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("consumer panic: %v", recovered)
		}
	}()
	return handler(ctx, msg)
}

func isPermanent(err error) bool {
	var permanent permanentError
	return errors.As(err, &permanent)
}

func delivery(msg *nats.Msg) uint64 {
	if meta, err := msg.Metadata(); err == nil {
		return meta.NumDelivered
	}
	return 1
}

func publishDLQ(client *mq.Client, dlqSubject, subject string, msg *nats.Msg, cause error, attempts uint64) error {
	payload, err := json.Marshal(map[string]any{
		"subject":   subject,
		"data":      string(msg.Data),
		"error":     cause.Error(),
		"attempts":  attempts,
		"failed_at": time.Now().Unix(),
	})
	if err != nil {
		return err
	}
	pctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return client.Publish(pctx, dlqSubject, payload)
}
