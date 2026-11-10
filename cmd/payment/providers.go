package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/dtm"
	"gin-kratos-gorm/internal/platform/grpcx"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/services/payment"

	"github.com/go-kratos/kratos/v3"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/grpc"
)

type orderConn struct{ *grpc.ClientConn }
type inventoryConn struct{ *grpc.ClientConn }
type pricingConn struct{ *grpc.ClientConn }

type paymentApp struct {
	App     *kratos.App
	Service *payment.Service
	DB      *sql.DB
	MQ      *mq.Client
	Logger  *slog.Logger
}

func provideLogger(cfg *config.Config) *slog.Logger { return log.New(cfg.Service, cfg.Env) }

func provideDB(cfg *config.Config) (*sql.DB, func(), error) {
	database, err := db.Open(cfg.MySQLDSN)
	if err != nil {
		return nil, nil, err
	}
	return database, func() { _ = database.Close() }, nil
}

func provideMQ(cfg *config.Config) (*mq.Client, func(), error) {
	client, err := mq.Connect(cfg.NATSURL)
	if err != nil {
		return nil, nil, err
	}
	return client, func() { client.Close() }, nil
}

func dialNamed(ctx context.Context, key, fallback string) (*grpc.ClientConn, func(), error) {
	conn, err := grpcx.Dial(ctx, envOr(key+"_ADDR", fallback))
	if err != nil {
		return nil, nil, err
	}
	return conn, func() { _ = conn.Close() }, nil
}

func provideOrderConn(ctx context.Context) (*orderConn, func(), error) {
	conn, cleanup, err := dialNamed(ctx, "ORDER", "order:9006")
	if err != nil {
		return nil, nil, err
	}
	return &orderConn{conn}, cleanup, nil
}
func provideInventoryConn(ctx context.Context) (*inventoryConn, func(), error) {
	conn, cleanup, err := dialNamed(ctx, "INVENTORY", "inventory:9003")
	if err != nil {
		return nil, nil, err
	}
	return &inventoryConn{conn}, cleanup, nil
}
func providePricingConn(ctx context.Context) (*pricingConn, func(), error) {
	conn, cleanup, err := dialNamed(ctx, "PRICING", "pricing:9005")
	if err != nil {
		return nil, nil, err
	}
	return &pricingConn{conn}, cleanup, nil
}

func providePaymentService(database *sql.DB, client *mq.Client, order *orderConn, inventory *inventoryConn, pricing *pricingConn, cfg *config.Config, logger *slog.Logger) *payment.Service {
	svc := payment.New(database, pb.NewOrderServiceClient(order.ClientConn), os.Getenv("PAYMENT_SECRET"), logger)
	svc.SetReconciliationClients(pb.NewInventoryServiceClient(inventory.ClientConn), pb.NewPricingServiceClient(pricing.ClientConn))
	svc.EnableTCC(dtm.New(cfg),
		envOr("ORDER_TCC_URL", "order:9006"),
		envOr("INVENTORY_TCC_URL", "inventory:9003"),
		envOr("PRICING_TCC_URL", "pricing:9005"),
		envOr("PAYMENT_TCC_ENABLED", "true") != "false")
	return svc
}

func provideGRPCServer(logger *slog.Logger, svc *payment.Service) *kratosgrpc.Server {
	gs := grpcx.NewServer(logger)
	pb.RegisterPaymentServiceServer(gs, svc)
	return gs
}

func provideHealth(cfg *config.Config, database *sql.DB, logger *slog.Logger) *khttp.Server {
	// DTM is required per transaction, not for process readiness.
	return runtime.HealthServer(cfg, database, logger)
}

func provideApp(cfg *config.Config, logger *slog.Logger, gs *kratosgrpc.Server, hs *khttp.Server) *kratos.App {
	return runtime.NewApp(cfg, logger, gs, hs)
}

func providePaymentApp(app *kratos.App, svc *payment.Service, database *sql.DB, client *mq.Client, logger *slog.Logger) *paymentApp {
	return &paymentApp{App: app, Service: svc, DB: database, MQ: client, Logger: logger}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
