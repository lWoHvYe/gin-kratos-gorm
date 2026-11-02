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
	"gin-kratos-gorm/internal/platform/dtm"
	"gin-kratos-gorm/internal/platform/grpcx"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"
	"gin-kratos-gorm/internal/platform/registry"
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/services/order"

	_ "github.com/dtm-labs/driver-kratos"
	"github.com/go-kratos/kratos/v2/transport/grpc/resolver/discovery"
	"google.golang.org/grpc/resolver"
)

func main() {
	cfg, err := config.Load("order")
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
	// DTM branch URLs use discovery:///... so DTM (and any gRPC client of this
	// process) needs the kratos etcd resolver registered for that scheme.
	if reg != nil {
		resolver.Register(discovery.NewBuilder(reg, discovery.WithInsecure(true)))
	}
	pricingConn, err := grpcx.Dial(ctx, env("PRICING_ADDR", "pricing:9005"), reg)
	if err != nil {
		logger.Error("dial pricing", "error", err)
		os.Exit(1)
	}
	defer pricingConn.Close()
	inventoryConn, err := grpcx.Dial(ctx, env("INVENTORY_ADDR", "inventory:9003"), reg)
	if err != nil {
		logger.Error("dial inventory", "error", err)
		os.Exit(1)
	}
	defer inventoryConn.Close()
	gs := grpcx.NewServer(logger)
	svc := order.New(database, client, logger, dtm.New(cfg),
		pb.NewPricingServiceClient(pricingConn), pb.NewInventoryServiceClient(inventoryConn))
	go order.RunExpiry(ctx, database, svc, logger, time.Minute)
	go order.RunShipmentConsumer(ctx, client, svc, logger)
	pb.RegisterOrderServiceServer(gs, svc)
	app := runtime.NewApp(cfg, logger, gs, runtime.HealthServerWithChecks(cfg, database, logger, func(ctx context.Context) error {
		return dtm.Probe(ctx, cfg.DTMAddress, 2*time.Second)
	}), reg)
	if err := app.Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
