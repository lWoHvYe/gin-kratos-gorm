package main

import (
	"context"
	"os"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/grpcx"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/platform/redis"
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/platform/tracing"
	"gin-kratos-gorm/internal/services/identity"
)

func main() {
	cfg, err := config.Load("identity")
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
	// The login limiter is optional: identity must still start if Redis is
	// unavailable (the limiter then fails open with a warning).
	cache, cacheErr := redis.Open(redis.Config{
		Mode: cfg.RedisMode, Addrs: cfg.RedisAddrs, Password: cfg.RedisPassword, MasterName: cfg.RedisMasterName,
	})
	if cacheErr != nil {
		logger.Warn("redis unavailable; login rate limiting disabled", "error", cacheErr)
	}
	if cache != nil {
		defer cache.Close()
	}
	gs := grpcx.NewServer(cfg, logger)
	svc := identity.New(database, auth.NewManager(cfg), logger, identity.NewLoginLimiter(cache))
	pb.RegisterIdentityServiceServer(gs, svc)
	app := runtime.NewApp(cfg, logger, gs, runtime.HealthServer(cfg, database, logger))
	if err := app.Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}
