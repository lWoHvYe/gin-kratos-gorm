package mq

import (
	"context"
	"testing"
)

func TestHealthyReportsDegradedSubscriptions(t *testing.T) {
	var nilClient *Client
	if err := nilClient.Healthy(context.Background()); err != nil {
		t.Fatalf("nil client should be healthy: %v", err)
	}
	c := &Client{}
	if err := c.Healthy(context.Background()); err != nil {
		t.Fatalf("fresh client should be healthy: %v", err)
	}
	c.degraded = append(c.degraded, "shop.order.paid.v1: nats: timeout")
	if err := c.Healthy(context.Background()); err == nil {
		t.Fatal("degraded client must fail readiness")
	}
}
