package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/grpcx"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/services/pricing"
)

func main() {
	cfg, err := config.Load("pricing")
	if err != nil {
		panic(err)
	}
	logger := log.New(cfg.Service, cfg.Env)
	database, err := db.Open(cfg.MySQLDSN)
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go outbox.Relay(ctx, database, client, logger, time.Second)

	catalogAddr := os.Getenv("CATALOG_ADDR")
	if catalogAddr == "" {
		catalogAddr = "catalog:9002"
	}
	conn, err := grpcx.Dial(context.Background(), catalogAddr)
	if err != nil {
		logger.Error("dial catalog", "addr", catalogAddr, "error", err)
		os.Exit(1)
	}
	defer conn.Close()
	catalog := pb.NewCatalogServiceClient(conn)

	gs := grpcx.NewServer(logger)
	svc := pricing.New(database, catalog, logger)
	go pricing.RunOrderCancelledConsumer(ctx, client, svc, logger)
	go pricing.RunOrderPaidConsumer(ctx, client, svc, logger)
	pb.RegisterPricingServiceServer(gs, svc)
	pb.RegisterPromotionServiceServer(gs, svc.Promotion())
	app := runtime.NewApp(cfg, logger, gs, runtime.HealthServer(cfg, database, logger))
	if err := app.Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}
