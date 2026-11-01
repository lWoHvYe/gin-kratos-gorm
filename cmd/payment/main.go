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
	"gin-kratos-gorm/internal/platform/dtm"
	"gin-kratos-gorm/internal/platform/grpcx"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"
	"gin-kratos-gorm/internal/platform/registry"
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/services/payment"

	_ "github.com/dtm-labs/driver-kratos"
	"github.com/go-kratos/kratos/v2/transport/grpc/resolver/discovery"
	"google.golang.org/grpc/resolver"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	cfg, err := config.Load("payment")
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

	orderAddr := os.Getenv("ORDER_ADDR")
	if orderAddr == "" {
		orderAddr = "order:9006"
	}
	conn, err := grpcx.Dial(context.Background(), orderAddr, reg)
	if err != nil {
		logger.Error("dial order", "addr", orderAddr, "error", err)
		os.Exit(1)
	}
	defer conn.Close()
	orders := pb.NewOrderServiceClient(conn)

	secret := os.Getenv("PAYMENT_SECRET")
	if secret == "" {
		logger.Warn("PAYMENT_SECRET is not set; callback signatures are not verified (mock mode)")
	}

	if reg != nil {
		resolver.Register(discovery.NewBuilder(reg, discovery.WithInsecure(true)))
	}
	gs := grpcx.NewServer(logger)
	svc := payment.New(database, orders, secret, logger)
	go payment.RunRefundConsumer(ctx, client, svc, logger)
	svc.EnableTCC(dtm.New(cfg),
		envOr("ORDER_TCC_URL", "discovery:///order/mall.v1.OrderService"),
		envOr("INVENTORY_TCC_URL", "discovery:///inventory/mall.v1.InventoryService"),
		envOr("PRICING_TCC_URL", "discovery:///pricing/mall.v1.PricingService"),
		envOr("PAYMENT_TCC_ENABLED", "true") != "false")
	pb.RegisterPaymentServiceServer(gs, svc)
	app := runtime.NewApp(cfg, logger, gs, runtime.HealthServerWithChecks(cfg, database, logger, func(ctx context.Context) error {
		return dtm.Probe(ctx, cfg.DTMAddress, 2*time.Second)
	}), reg)
	if err := app.Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}
