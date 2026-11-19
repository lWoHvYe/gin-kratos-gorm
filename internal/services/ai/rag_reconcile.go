package ai

import (
	"context"
	"log/slog"
	"time"
)

// RunRAGReconcile periodically repairs missed catalog events. It is a bounded
// catch-up mechanism, not a replacement for an event high-watermark.
func RunRAGReconcile(ctx context.Context, rag RAGIndex, catalog catalogClient, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := rag.ReconcileProducts(ctx, catalog); err != nil {
				logger.Warn("rag reconcile failed", "error", err)
			} else {
				logger.Info("rag reconcile complete")
			}
		}
	}
}
