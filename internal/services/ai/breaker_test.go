package ai

import (
	"testing"
	"time"
)

func TestCircuitBreakerOpensAndRecovers(t *testing.T) {
	now := time.Unix(0, 0)
	b := newCircuitBreaker(2, time.Minute)
	b.now = func() time.Time { return now }

	if !b.Allow() {
		t.Fatal("fresh breaker should allow")
	}
	b.Failure()
	if !b.Allow() {
		t.Fatal("one failure must not open the circuit")
	}
	b.Failure()
	if b.Allow() {
		t.Fatal("threshold failures must open the circuit")
	}
	now = now.Add(61 * time.Second)
	if !b.Allow() {
		t.Fatal("cooldown should allow a probe")
	}
	b.Success()
	if !b.Allow() {
		t.Fatal("success should close the circuit")
	}
}
