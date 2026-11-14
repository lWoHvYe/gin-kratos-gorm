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
	"gin-kratos-gorm/internal/services/order"

	"github.com/go-kratos/kratos/v3"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/grpc"
)

const (
	defaultPricingAddr   = "pricing:9005"
	defaultInventoryAddr = "inventory:9003"
)

type pricingConn struct{ *grpc.ClientConn }
type inventoryConn struct{ *grpc.ClientConn }

type orderApp struct {
	App     *kratos.App
	Service *order.Service
	DB      *sql.DB
	MQ      *mq.Client
	Logger  *slog.Logger
}

func provideLogger(cfg *config.Config) *slog.Logger {
	return log.New(cfg.Service, cfg.Env)
}

func provideDB(ctx context.Context, cfg *config.Config) (*sql.DB, func(), error) {
	database, err := db.OpenWithPoolContext(ctx, cfg.MySQLDSN, cfg.MySQLMaxOpenConns, cfg.MySQLMaxIdleConns)
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

func providePricingConn(ctx context.Context) (*pricingConn, func(), error) {
	conn, err := grpcx.Dial(ctx, env("PRICING_ADDR", defaultPricingAddr))
	if err != nil {
		return nil, nil, err
	}
	return &pricingConn{conn}, func() { _ = conn.Close() }, nil
}

func provideInventoryConn(ctx context.Context) (*inventoryConn, func(), error) {
	conn, err := grpcx.Dial(ctx, env("INVENTORY_ADDR", defaultInventoryAddr))
	if err != nil {
		return nil, nil, err
	}
	return &inventoryConn{conn}, func() { _ = conn.Close() }, nil
}

func providePricingClient(conn *pricingConn) pb.PricingServiceClient {
	return pb.NewPricingServiceClient(conn.ClientConn)
}

func provideInventoryClient(conn *inventoryConn) pb.InventoryServiceClient {
	return pb.NewInventoryServiceClient(conn.ClientConn)
}

func providePromotionClient(conn *pricingConn) pb.PromotionServiceClient {
	return pb.NewPromotionServiceClient(conn.ClientConn)
}

func provideDTMClient(cfg *config.Config) *dtm.Client {
	return dtm.New(cfg)
}

func provideOrderService(database *sql.DB, client *mq.Client, logger *slog.Logger, dtmClient *dtm.Client, pricing pb.PricingServiceClient, inventory pb.InventoryServiceClient, promotion pb.PromotionServiceClient) *order.Service {
	return order.New(database, client, logger, dtmClient, pricing, inventory, promotion)
}

func provideGRPCServer(cfg *config.Config, logger *slog.Logger, svc *order.Service) *kratosgrpc.Server {
	gs := grpcx.NewServer(cfg, logger)
	pb.RegisterOrderServiceServer(gs, svc)
	return gs
}

func provideHealthServer(cfg *config.Config, database *sql.DB, logger *slog.Logger) *khttp.Server {
	// DTM is required per transaction, not for process readiness. A DTM
	// readiness probe would flap the whole order/payment service on transient
	// control-plane or waypoint issues.
	return runtime.HealthServer(cfg, database, logger)
}

func provideApp(cfg *config.Config, logger *slog.Logger, gs *kratosgrpc.Server, hs *khttp.Server) *kratos.App {
	return runtime.NewApp(cfg, logger, gs, hs)
}

func provideOrderApp(app *kratos.App, svc *order.Service, database *sql.DB, client *mq.Client, logger *slog.Logger) *orderApp {
	return &orderApp{App: app, Service: svc, DB: database, MQ: client, Logger: logger}
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
