package server

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/biz/model"
	"gin-kratos-gorm/internal/conf"
	"gin-kratos-gorm/internal/data/event"

	"github.com/go-kratos/kratos/v2/log"
)

// Worker hosts background jobs: the outbox dispatcher and the expired-order
// reaper. It implements transport.Server so it participates in Kratos graceful
// shutdown.
type Worker struct {
	dispatcher     *event.Dispatcher
	orderUC        *biz.OrderUsecase
	catalog        biz.ProductCatalog
	interval       time.Duration
	cancelInterval time.Duration
	log            *log.Helper
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

func NewWorker(cfg *conf.Bootstrap, dispatcher *event.Dispatcher, orderUC *biz.OrderUsecase, catalog biz.ProductCatalog, logger log.Logger) *Worker {
	helper := log.NewHelper(logger)
	w := &Worker{
		dispatcher:     dispatcher,
		orderUC:        orderUC,
		catalog:        catalog,
		interval:       2 * time.Second,
		cancelInterval: time.Minute,
		log:            helper,
	}
	if cfg != nil && cfg.Order != nil {
		if cfg.Order.AutoCancelInterval != nil && cfg.Order.AutoCancelInterval.AsDuration() > 0 {
			w.cancelInterval = cfg.Order.AutoCancelInterval.AsDuration()
		}
	}
	w.registerHandlers()
	return w
}

func (w *Worker) registerHandlers() {
	// Retry stock restoration through the outbox. The synchronous restore in
	// OrderUsecase.Cancel is best-effort; this handler makes it eventual.
	w.dispatcher.Register(biz.EventOrderCancelled, func(ctx context.Context, row *model.OutboxEvent) error {
		var payload biz.OrderEventPayload
		if err := json.Unmarshal([]byte(row.Payload), &payload); err != nil {
			return err
		}
		for _, item := range payload.Items {
			if err := w.catalog.RestoreStock(ctx, payload.OrderSN, item.SkuID, item.Num); err != nil {
				return err
			}
		}
		return nil
	})
	// Audit/notification consumers can be added here. With no handler the
	// dispatcher parks events; these no-ops keep the outbox drained.
	noop := func(context.Context, *model.OutboxEvent) error { return nil }
	w.dispatcher.Register(biz.EventOrderCreated, noop)
	w.dispatcher.Register(biz.EventOrderPaid, noop)
}

func (w *Worker) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		if err := w.dispatcher.Run(runCtx); err != nil && err != context.Canceled {
			w.log.Errorf("outbox dispatcher stopped: %v", err)
		}
	}()

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.cancelInterval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				cancelled, err := w.orderUC.AutoCancelExpired(runCtx, 100)
				if err != nil && err != context.Canceled {
					w.log.Errorf("auto cancel expired orders: %v", err)
					continue
				}
				if cancelled > 0 {
					w.log.Infof("auto cancelled %d expired order(s)", cancelled)
				}
			}
		}
	}()
	return nil
}

func (w *Worker) Stop(ctx context.Context) error {
	if w.cancel != nil {
		w.cancel()
	}
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
