package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/outbox"
	"gin-kratos-gorm/internal/services/payment"
)

func main() {
	cfg, err := config.Load("payment")
	if err != nil {
		panic(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	bundle, cleanup, err := initialize(ctx, cfg)
	if err != nil {
		panic(err)
	}
	defer cleanup()
	go outbox.Relay(ctx, bundle.DB, bundle.MQ, bundle.Logger, time.Second)
	go payment.RunRefundConsumer(ctx, bundle.MQ, bundle.Service, bundle.Logger)
	if err := bundle.App.Run(); err != nil {
		panic(err)
	}
}
