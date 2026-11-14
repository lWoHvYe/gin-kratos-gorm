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
	"gin-kratos-gorm/internal/platform/redis"
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/platform/tracing"
	"gin-kratos-gorm/internal/services/catalog"
)

func main() {
	cfg, err := config.Load("catalog")
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
	cache, err := redis.Open(redis.Config{Mode: cfg.RedisMode, Addrs: cfg.RedisAddrs, Password: cfg.RedisPassword, MasterName: cfg.RedisMasterName})
	if err != nil {
		logger.Error("connect redis", "error", err)
		os.Exit(1)
	}
	defer cache.Close()
	workers := runtime.NewWorkers()
	defer workers.Stop()
	workers.Go(func(ctx context.Context) { outbox.Relay(ctx, database, client, logger, time.Second) })
	svc := catalog.New(database, client, cache, logger)
	workers.Go(func(ctx context.Context) { catalog.RunStockConsumer(ctx, client, svc, logger) })
	gs := grpcx.NewServer(cfg, logger)
	pb.RegisterCatalogServiceServer(gs, svc)
	app := runtime.NewApp(cfg, logger, gs, runtime.HealthServer(cfg, database, logger, client.Healthy))
	if err := app.Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}
