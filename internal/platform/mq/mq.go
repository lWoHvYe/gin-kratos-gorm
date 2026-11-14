package mq

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type Client struct {
	Conn *nats.Conn
	JS   nats.JetStreamContext

	mu       sync.Mutex
	degraded []string
}

func Connect(url string) (*Client, error) {
	nc, err := nats.Connect(url, nats.Name("mall"), nats.Timeout(5*time.Second), nats.RetryOnFailedConnect(true), nats.MaxReconnects(10))
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	c := &Client{Conn: nc, JS: js}
	if err := c.ensureStream(); err != nil {
		nc.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) ensureStream() error {
	_, err := c.JS.StreamInfo("MALL")
	if err == nil {
		return nil
	}
	_, err = c.JS.AddStream(&nats.StreamConfig{
		Name: "MALL", Subjects: []string{"shop.>"}, Retention: nats.LimitsPolicy,
		Storage: nats.FileStorage, MaxAge: 7 * 24 * time.Hour, Replicas: streamReplicas(),
	})
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "already in use") || strings.Contains(msg, "already exists") {
			return nil
		}
		return fmt.Errorf("add stream MALL: %w", err)
	}
	return nil
}

func (c *Client) Publish(ctx context.Context, subject string, payload []byte) error {
	msg := nats.NewMsg(subject)
	msg.Data = payload
	return c.PublishMsg(ctx, msg)
}

// PublishMsg publishes with W3C trace context in the message headers so an
// asynchronous consumer can continue the trace.
func (c *Client) PublishMsg(ctx context.Context, msg *nats.Msg) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	injectTrace(ctx, msg.Header)
	if _, err := c.JS.PublishMsg(msg, nats.Context(ctx)); err != nil {
		return fmt.Errorf("publish %s: %w", msg.Subject, err)
	}
	return nil
}

// QueueSubscribe wraps the JetStream subscription and records failures so the
// readiness probe can report a pod that never subscribed to its consumers.
func (c *Client) QueueSubscribe(subject, queue string, handler nats.MsgHandler, opts ...nats.SubOpt) (*nats.Subscription, error) {
	sub, err := c.JS.QueueSubscribe(subject, queue, handler, opts...)
	if err != nil {
		c.mu.Lock()
		c.degraded = append(c.degraded, fmt.Sprintf("%s: %v", subject, err))
		c.mu.Unlock()
	}
	return sub, err
}

// Healthy is a readiness check: a failed consumer subscription keeps failing
// the probe until the pod is replaced.
func (c *Client) Healthy(context.Context) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.degraded) > 0 {
		return fmt.Errorf("nats subscriptions degraded: %s", strings.Join(c.degraded, "; "))
	}
	return nil
}

func (c *Client) Close() {
	if c == nil || c.Conn == nil {
		return
	}
	// Drain waits for in-flight handlers and flushes pending publishes. Bound
	// it so a stuck handler cannot hold a rolling update forever.
	done := make(chan struct{})
	go func() {
		_ = c.Conn.Drain()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
	c.Conn.Close()
}

// streamReplicas is opt-in: NATS_STREAM_REPLICAS=3 creates the durable MALL
// stream with three replicas on a JetStream cluster. Existing streams keep their
// current replica count; migrate them explicitly (see docs/production-ha.md).
func streamReplicas() int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("NATS_STREAM_REPLICAS"))); err == nil && n > 0 {
		return n
	}
	return 1
}

// injectTrace writes the W3C traceparent into NATS headers.
func injectTrace(ctx context.Context, header nats.Header) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(header))
}
