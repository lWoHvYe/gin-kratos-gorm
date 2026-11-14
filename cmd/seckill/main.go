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
	"gin-kratos-gorm/internal/platform/redis"
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/platform/tracing"
	"gin-kratos-gorm/internal/services/seckill"
)

func main() {
	cfg, err := config.Load("seckill")
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
	cache, err := redis.Open(redis.Config{Mode: cfg.RedisMode, Addrs: cfg.RedisAddrs, Password: cfg.RedisPassword, MasterName: cfg.RedisMasterName})
	if err != nil {
		logger.Error("open redis", "error", err)
		os.Exit(1)
	}
	defer cache.Close()
	inventoryAddr := os.Getenv("INVENTORY_ADDR")
	if inventoryAddr == "" {
		inventoryAddr = "inventory:9003"
	}
	conn, err := grpcx.Dial(context.Background(), inventoryAddr)
	if err != nil {
		logger.Error("dial inventory", "addr", inventoryAddr, "error", err)
		os.Exit(1)
	}
	defer conn.Close()
	inventory := pb.NewInventoryServiceClient(conn)
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
	paymentAddr := os.Getenv("PAYMENT_ADDR")
	if paymentAddr == "" {
		paymentAddr = "payment:9007"
	}
	paymentConn, err := grpcx.Dial(context.Background(), paymentAddr)
	if err != nil {
		logger.Error("dial payment", "addr", paymentAddr, "error", err)
		os.Exit(1)
	}
	defer paymentConn.Close()
	payment := pb.NewPaymentServiceClient(paymentConn)
	client, err := mq.Connect(cfg.NATSURL)
	if err != nil {
		logger.Error("connect nats", "error", err)
		os.Exit(1)
	}
	defer client.Close()
	if err := seckill.EnsureStream(client); err != nil {
		logger.Error("ensure seckill stream", "error", err)
		os.Exit(1)
	}

	workers := runtime.NewWorkers()
	defer workers.Stop()
	tokenSecret := cfg.SeckillTokenSecret
	if tokenSecret == "" {
		// Dev fallback only; production validation rejects an empty value.
		tokenSecret = cfg.JWTSecret
		logger.Warn("SECKILL_TOKEN_SECRET is unset; falling back to the JWT secret (dev only)")
	}
	svc := seckill.New(database, inventory, orders, payment, client, cache, logger, tokenSecret)
	workers.Go(func(ctx context.Context) { svc.RunScheduler(ctx, 5*time.Second) })
	workers.Go(func(ctx context.Context) { svc.RunQualificationExpiry(ctx, 30*time.Second) })
	workers.Go(func(ctx context.Context) { svc.RunRepublisher(ctx, 10*time.Second) })
	workers.Go(func(ctx context.Context) { svc.RunOrderWorker(ctx, 4) })
	workers.Go(func(ctx context.Context) { seckill.RunOrderCancelledConsumer(ctx, client, svc, logger) })
	workers.Go(func(ctx context.Context) { svc.RunReconciler(ctx, 30*time.Second) })
	gs := grpcx.NewServer(cfg, logger)
	pb.RegisterSeckillServiceServer(gs, svc)
	app := runtime.NewApp(cfg, logger, gs, runtime.HealthServer(cfg, database, logger, client.Healthy))
	if err := app.Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}
