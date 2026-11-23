package main

import (
	"context"
	"os"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/grpcx"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/platform/tracing"
	"gin-kratos-gorm/internal/services/fulfillment"
)

func main() {
	cfg, err := config.Load("fulfillment")
	if err != nil {
		panic(err)
	}
	logger := log.New(cfg.Service, cfg.Env)
	shutdownTracing, err := tracing.Init(context.Background(), cfg.Service)
	if err != nil {
		logger.Error("init tracing", "error", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	startupCtx, stopStartup := runtime.StartupContext()
	defer stopStartup()
	database, err := db.OpenWithPoolContext(startupCtx, cfg.MySQLDSN, cfg.MySQLMaxOpenConns, cfg.MySQLMaxIdleConns)
	if err != nil {
		logger.Error("open db", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	client, err := mq.Connect(cfg.NATSURL)
	if err != nil {
		logger.Error("connect nats", "error", err)
		os.Exit(1)
	}
	defer client.Close()
	workers := runtime.NewWorkers()
	defer workers.Stop()
	workers.Go(func(ctx context.Context) { outbox.Relay(ctx, database, client, logger, time.Second) })
	orderAddr := os.Getenv("ORDER_ADDR")
	if orderAddr == "" {
		orderAddr = "order:9006"
	}
	orderConn, err := grpcx.Dial(context.Background(), orderAddr)
	if err != nil {
		logger.Error("dial order", "error", err)
		os.Exit(1)
	}
	defer orderConn.Close()
	gs := grpcx.NewServer(cfg, logger)
	svc := fulfillment.NewWithOptions(database, client, logger, pb.NewOrderServiceClient(orderConn), fulfillment.Options{
		CarrierRegistryEnabled:     cfg.CarrierRegistryEnabled,
		DeliveryAutoConfirmEnabled: cfg.DeliveryAutoConfirmEnabled,
	})
	if cfg.FulfillmentConsumeOrderSettled {
		workers.Go(func(ctx context.Context) { fulfillment.RunOrderSettledConsumer(ctx, client, svc, logger) })
	} else {
		workers.Go(func(ctx context.Context) { fulfillment.RunOrderPaidConsumer(ctx, client, svc, logger) })
	}
	workers.Go(func(ctx context.Context) { fulfillment.RunAfterSaleRefundConsumer(ctx, client, svc, logger) })
	workers.Go(func(ctx context.Context) { fulfillment.RunRefundAdmittedConsumer(ctx, client, svc, logger) })
	workers.Go(func(ctx context.Context) {
		fulfillment.RunDeliveryAutoConfirmWorker(ctx, svc, logger, 15*time.Second)
	})
	pb.RegisterFulfillmentServiceServer(gs, svc)
	app := runtime.NewApp(cfg, logger, gs, runtime.HealthServer(cfg, database, logger, client.Healthy))
	if err := app.Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}
