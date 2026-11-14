package main

import (
	"context"
	"time"

	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/outbox"
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/platform/tracing"
	"gin-kratos-gorm/internal/services/order"
)

func main() {
	cfg, err := config.Load("order")
	if err != nil {
		panic(err)
	}
	ctx, stopStartup := runtime.StartupContext()
	defer stopStartup()
	shutdownTracing, err := tracing.Init(ctx, cfg.Service)
	if err != nil {
		panic(err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	bundle, cleanup, err := initialize(ctx, cfg)
	if err != nil {
		panic(err)
	}
	defer cleanup()
	workers := runtime.NewWorkers()
	defer workers.Stop()
	workers.Go(func(ctx context.Context) { outbox.Relay(ctx, bundle.DB, bundle.MQ, bundle.Logger, time.Second) })
	workers.Go(func(ctx context.Context) { order.RunExpiry(ctx, bundle.DB, bundle.Service, bundle.Logger, time.Minute) })
	workers.Go(func(ctx context.Context) { order.RunShipmentConsumer(ctx, bundle.MQ, bundle.Service, bundle.Logger) })
	workers.Go(func(ctx context.Context) { order.RunRefundConsumer(ctx, bundle.MQ, bundle.Service, bundle.Logger) })
	if err := bundle.App.Run(); err != nil {
		panic(err)
	}
}
