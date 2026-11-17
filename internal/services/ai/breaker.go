package ai

import (
	"sync"
	"time"

	"gin-kratos-gorm/internal/platform/metrics"
)

// circuitBreaker stops hammering a failing model provider. After threshold
// consecutive failures it opens for cooldown; one probe is allowed afterwards.
type circuitBreaker struct {
	mu        sync.Mutex
	failures  int
	threshold int
	cooldown  time.Duration
	openUntil time.Time
	now       func() time.Time
}

func newCircuitBreaker(threshold int, cooldown time.Duration) *circuitBreaker {
	if threshold <= 0 {
		threshold = 5
	}
	if cooldown <= 0 {
		cooldown = 60 * time.Second
	}
	return &circuitBreaker{threshold: threshold, cooldown: cooldown, now: time.Now}
}

func (b *circuitBreaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openUntil.IsZero() {
		return true
	}
	if b.now().Before(b.openUntil) {
		metrics.AICircuitState.Set(1)
		return false
	}
	b.openUntil = time.Time{}
	metrics.AICircuitState.Set(0)
	return true
}

func (b *circuitBreaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.openUntil = time.Time{}
	metrics.AICircuitState.Set(0)
}

func (b *circuitBreaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.failures >= b.threshold {
		b.openUntil = b.now().Add(b.cooldown)
		b.failures = 0
		metrics.AICircuitState.Set(1)
	}
}
