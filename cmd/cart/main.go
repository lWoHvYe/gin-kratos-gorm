package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/grpcx"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"
	"gin-kratos-gorm/internal/platform/registry"
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/services/cart"
)

// defaultCatalogAddr is the direct (non-discovery) catalog endpoint used when
// CATALOG_ADDR is unset.
const defaultCatalogAddr = "catalog:9002"

func main() {
	cfg, err := config.Load("cart")
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
	reg, closeReg, err := registry.New(cfg.ETCDEndpoints)
	if err != nil {
		logger.Error("registry", "error", err)
		os.Exit(1)
	}
	defer closeReg()
	// Cart talks to catalog directly by address; discovery is not used here.
	catalogAddr := envOr("CATALOG_ADDR", defaultCatalogAddr)
	catalogConn, err := grpcx.Dial(ctx, catalogAddr, nil)
	if err != nil {
		logger.Error("dial catalog", "addr", catalogAddr, "error", err)
		os.Exit(1)
	}
	defer func() { _ = catalogConn.Close() }()
	gs := grpcx.NewServer(logger)
	svc := cart.New(database, pb.NewCatalogServiceClient(catalogConn), logger)
	pb.RegisterCartServiceServer(gs, svc)
	app := runtime.NewApp(cfg, logger, gs, runtime.HealthServer(cfg, database, logger), reg)
	if err := app.Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
