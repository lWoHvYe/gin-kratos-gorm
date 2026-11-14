package runtime

import (
	"context"
	"sync"
)

// Workers tracks background loops so shutdown can stop them and wait before the
// database/NATS handles are closed. The context is intentionally independent of
// the signal context: Kratos stops the HTTP/gRPC servers on SIGTERM, then main
// calls Stop, so in-flight requests and consumer drains keep a live context.
type Workers struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewWorkers() *Workers {
	ctx, cancel := context.WithCancel(context.Background())
	return &Workers{ctx: ctx, cancel: cancel}
}

// Go runs fn in a tracked goroutine. fn is expected to return when ctx is done.
func (w *Workers) Go(fn func(context.Context)) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		fn(w.ctx)
	}()
}

// Stop cancels the workers and waits for them to return.
func (w *Workers) Stop() {
	if w == nil {
		return
	}
	w.cancel()
	w.wg.Wait()
}
