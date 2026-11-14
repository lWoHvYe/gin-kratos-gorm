package runtime

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkersStopWaitsForLoops(t *testing.T) {
	workers := NewWorkers()
	var finished atomic.Bool
	workers.Go(func(ctx context.Context) {
		<-ctx.Done()
		time.Sleep(10 * time.Millisecond)
		finished.Store(true)
	})
	workers.Stop()
	if !finished.Load() {
		t.Fatal("Stop returned before the worker finished")
	}
}

func TestWorkersContextIsIndependentOfCaller(t *testing.T) {
	workers := NewWorkers()
	defer workers.Stop()
	var live atomic.Bool
	workers.Go(func(ctx context.Context) {
		// The worker context must not be cancelled just because the caller's
		// context (e.g. a request) is done.
		select {
		case <-ctx.Done():
		case <-time.After(20 * time.Millisecond):
			live.Store(true)
		}
	})
	time.Sleep(40 * time.Millisecond)
	if !live.Load() {
		t.Fatal("worker context was cancelled prematurely")
	}
}
