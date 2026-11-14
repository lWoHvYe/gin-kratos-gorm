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
	"gin-kratos-gorm/internal/services/aftersales"
)

func main() {
	cfg, err := config.Load("aftersales")
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
	paymentAddr := os.Getenv("PAYMENT_ADDR")
	if paymentAddr == "" {
		paymentAddr = "payment:9007"
	}
	conn, err := grpcx.Dial(context.Background(), paymentAddr)
	if err != nil {
		logger.Error("dial payment", "addr", paymentAddr, "error", err)
		os.Exit(1)
	}
	defer conn.Close()
	payments := pb.NewPaymentServiceClient(conn)
	inventoryAddr := os.Getenv("INVENTORY_ADDR")
	if inventoryAddr == "" {
		inventoryAddr = "inventory:9003"
	}
	inventoryConn, err := grpcx.Dial(context.Background(), inventoryAddr)
	if err != nil {
		logger.Error("dial inventory", "addr", inventoryAddr, "error", err)
		os.Exit(1)
	}
	defer inventoryConn.Close()
	inventory := pb.NewInventoryServiceClient(inventoryConn)
	orderAddr := os.Getenv("ORDER_ADDR")
	if orderAddr == "" {
		orderAddr = "order:9006"
	}
	orderConn, err := grpcx.Dial(context.Background(), orderAddr)
	if err != nil {
		logger.Error("dial order", "addr", orderAddr, "error", err)
		os.Exit(1)
	}
	defer orderConn.Close()
	orders := pb.NewOrderServiceClient(orderConn)
	svc := aftersales.New(database, client, payments, inventory, orders, logger)
	workers.Go(func(ctx context.Context) { aftersales.RunRefundRecovery(ctx, database, svc, logger, time.Minute) })
	gs := grpcx.NewServer(cfg, logger)
	pb.RegisterAfterSalesServiceServer(gs, svc)
	app := runtime.NewApp(cfg, logger, gs, runtime.HealthServer(cfg, database, logger, client.Healthy))
	if err := app.Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}
