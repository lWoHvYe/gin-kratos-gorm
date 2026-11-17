package main

import (
	"context"
	"os"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/grpcx"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/platform/redis"
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/platform/tracing"
	"gin-kratos-gorm/internal/services/ai"

	"google.golang.org/grpc"
)

func main() {
	cfg, err := config.Load("ai")
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

	cache, err := redis.Open(redis.Config{
		Mode: cfg.RedisMode, Addrs: cfg.RedisAddrs, Password: cfg.RedisPassword, MasterName: cfg.RedisMasterName,
	})
	if err != nil {
		logger.Error("connect redis", "error", err)
		os.Exit(1)
	}
	defer cache.Close()

	dial := func(env, fallback string) (*grpc.ClientConn, error) {
		addr := strings.TrimSpace(os.Getenv(env))
		if addr == "" {
			addr = fallback
		}
		return grpcx.Dial(context.Background(), addr)
	}
	catalogConn, err := dial("CATALOG_ADDR", "catalog:9002")
	if err != nil {
		logger.Error("dial catalog", "error", err)
		os.Exit(1)
	}
	defer catalogConn.Close()
	catalog := pb.NewCatalogServiceClient(catalogConn)
	orderConn, err := dial("ORDER_ADDR", "order:9006")
	if err != nil {
		logger.Error("dial order", "error", err)
		os.Exit(1)
	}
	defer orderConn.Close()
	afterSalesConn, err := dial("AFTERSALES_ADDR", "aftersales:9009")
	if err != nil {
		logger.Error("dial aftersales", "error", err)
		os.Exit(1)
	}
	defer afterSalesConn.Close()
	inventoryConn, err := dial("INVENTORY_ADDR", "inventory:9003")
	if err != nil {
		logger.Error("dial inventory", "error", err)
		os.Exit(1)
	}
	defer inventoryConn.Close()
	pricingConn, err := dial("PRICING_ADDR", "pricing:9005")
	if err != nil {
		logger.Error("dial pricing", "error", err)
		os.Exit(1)
	}
	defer pricingConn.Close()

	var rag ai.RAGSearcher
	if cfg.RAGEnabled {
		built, err := ai.OpenRAG(context.Background(), ai.RAGConfig{
			Provider: cfg.EmbeddingProvider,
			BaseURL:  cfg.EmbeddingBaseURL,
			APIKey:   cfg.EmbeddingAPIKey,
			Model:    cfg.EmbeddingModel,
			Dim:      cfg.EmbeddingDim,
		}, catalog, cache, logger)
		if err != nil {
			logger.Warn("rag disabled", "error", err)
		} else {
			rag = built
		}
	}

	svc, err := ai.NewService(context.Background(), ai.Config{
		Enabled:    cfg.AIEnabled,
		Timeout:    cfg.AITimeout,
		RateLimit:  cfg.AIRateLimit,
		RateWindow: cfg.AIRateWindow,

		DailyTokenBudget:       cfg.AIDailyTokenBudget,
		GlobalDailyTokenBudget: cfg.AIGlobalDailyTokenBudget,
		PricePromptPer1K:       cfg.AIPricePromptPer1K,
		PriceCompletionPer1K:   cfg.AIPriceCompletionPer1K,
		CircuitFailures:        cfg.AICircuitFailures,
		CircuitCooldown:        cfg.AICircuitCooldown,
	}, ai.ProviderConfig{
		Provider:   cfg.AIProvider,
		APIKey:     cfg.DeepSeekAPIKey,
		BaseURL:    cfg.DeepSeekBaseURL,
		Model:      cfg.AIModel,
		MaxTokens:  cfg.AIMaxTokens,
		TimeoutSec: int(cfg.AITimeout.Seconds()),
	}, ai.Deps{
		Catalog:    pb.NewCatalogServiceClient(catalogConn),
		Orders:     pb.NewOrderServiceClient(orderConn),
		AfterSales: pb.NewAfterSalesServiceClient(afterSalesConn),
		Inventory:  pb.NewInventoryServiceClient(inventoryConn),
		Promotion:  pb.NewPromotionServiceClient(pricingConn),
		RAG:        rag,
	}, cache, logger)
	if err != nil {
		logger.Error("build ai service", "error", err)
		os.Exit(1)
	}
	if !cfg.AIEnabled {
		logger.Warn("ai assistant is disabled", "AI_ENABLED", "false")
	}

	gs := grpcx.NewServer(cfg, logger)
	pb.RegisterAIServiceServer(gs, svc)
	health := runtime.HealthServer(cfg, nil, logger, func(ctx context.Context) error {
		return cache.Ping(ctx).Err()
	})
	app := runtime.NewApp(cfg, logger, gs, health)
	if err := app.Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}
