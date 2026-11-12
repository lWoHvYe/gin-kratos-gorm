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
	"gin-kratos-gorm/internal/platform/redis"
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/services/seckill"
)

func main() {
	cfg, err := config.Load("seckill")
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
	cache, err := redis.Open(cfg.RedisAddr, cfg.RedisPassword)
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	tokenSecret := os.Getenv("SECKILL_TOKEN_SECRET")
	if tokenSecret == "" {
		tokenSecret = cfg.JWTSecret
	}
	svc := seckill.New(database, inventory, cache, logger, tokenSecret)
	go svc.RunScheduler(ctx, 5*time.Second)
	go svc.RunQualificationExpiry(ctx, 30*time.Second)
	gs := grpcx.NewServer(logger)
	pb.RegisterSeckillServiceServer(gs, svc)
	app := runtime.NewApp(cfg, logger, gs, runtime.HealthServer(cfg, database, logger))
	if err := app.Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}
