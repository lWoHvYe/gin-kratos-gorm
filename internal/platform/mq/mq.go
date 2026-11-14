package mq

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

type Client struct {
	Conn *nats.Conn
	JS   nats.JetStreamContext
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
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	_, err := c.JS.Publish(subject, payload, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("publish %s: %w", subject, err)
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
