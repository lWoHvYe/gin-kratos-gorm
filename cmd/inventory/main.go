package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
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
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/platform/tracing"
	"gin-kratos-gorm/internal/services/inventory"
)

func main() {
	cfg, err := config.Load("inventory")
	if err != nil {
		panic(err)
	}
	logger := log.New(cfg.Service, cfg.Env)
	if len(os.Args) > 1 && os.Args[1] == "op-replay" {
		runOperationReplay(cfg, logger)
		return
	}
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
	workers := runtime.NewWorkers()
	workers.Go(func(ctx context.Context) { inventory.RunLedgerMetrics(ctx, database, logger, time.Minute) })
	workers.Go(func(ctx context.Context) { inventory.RunLedgerArchive(ctx, database, logger, time.Hour) })
	defer workers.Stop()
	workers.Go(func(ctx context.Context) { outbox.Relay(ctx, database, client, logger, time.Second) })
	gs := grpcx.NewServer(cfg, logger)
	svc := inventory.New(database, logger)
	workers.Go(func(ctx context.Context) { inventory.RunExpiry(ctx, database, svc, logger, time.Minute) })
	workers.Go(func(ctx context.Context) { inventory.RunOrderCancelledConsumer(ctx, client, svc, logger) })
	workers.Go(func(ctx context.Context) { inventory.RunCompensationConsumer(ctx, client, svc, logger) })
	workers.Go(func(ctx context.Context) { inventory.RunConfirmConsumer(ctx, client, svc, logger) })
	pb.RegisterInventoryServiceServer(gs, svc)
	app := runtime.NewApp(cfg, logger, gs, runtime.HealthServer(cfg, database, logger, client.Healthy))
	if err := app.Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}

// runOperationReplay is the explicit recovery entrypoint for a permanently
// failed or missing inventory operation item. It is a one-shot command, not a
// scanner:
//
//	inventory op-replay -operation <operation_id> [-skus 1,2]
//
// Omitting -skus re-arms every non-DONE item; completed items are never
// replayed.
func runOperationReplay(cfg *config.Config, logger *slog.Logger) {
	flags := flag.NewFlagSet("op-replay", flag.ExitOnError)
	operationID := flags.String("operation", "", "inventory operation_id")
	rawSKUs := flags.String("skus", "", "comma-separated sku ids (empty = all non-DONE items)")
	_ = flags.Parse(os.Args[2:])
	if strings.TrimSpace(*operationID) == "" {
		logger.Error("op-replay requires -operation")
		os.Exit(2)
	}
	var skus []int64
	for _, part := range strings.Split(*rawSKUs, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		sku, err := strconv.ParseInt(part, 10, 64)
		if err != nil || sku <= 0 {
			logger.Error("invalid -skus entry", "value", part)
			os.Exit(2)
		}
		skus = append(skus, sku)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	database, err := db.OpenWithPoolContext(ctx, cfg.MySQLDSN, cfg.MySQLMaxOpenConns, cfg.MySQLMaxIdleConns)
	if err != nil {
		logger.Error("open db", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	svc := inventory.New(database, logger)
	if err := svc.ReplayInventoryOperation(ctx, *operationID, skus); err != nil {
		logger.Error("op-replay failed", "operation", *operationID, "error", err)
		os.Exit(1)
	}
	logger.Info("op-replay complete", "operation", *operationID, "skus", skus)
}
