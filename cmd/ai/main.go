package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/grpcx"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/platform/mq"
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
	// The same image runs the one-shot RAG build Job with `args: ["rag-build"]`.
	// AI Pods never build the index; they only follow the published alias.
	if len(os.Args) > 1 && os.Args[1] == "rag-build" {
		runRAGBuild(cfg, logger)
		return
	}
	runService(cfg, logger)
}

func runRAGBuild(cfg *config.Config, logger *slog.Logger) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cache, err := redis.Open(redis.Config{
		Mode: cfg.RedisMode, Addrs: cfg.RedisAddrs, Password: cfg.RedisPassword, MasterName: cfg.RedisMasterName,
	})
	if err != nil {
		logger.Error("connect redis", "error", err)
		os.Exit(1)
	}
	defer cache.Close()

	catalogAddr := strings.TrimSpace(os.Getenv("CATALOG_ADDR"))
	if catalogAddr == "" {
		catalogAddr = "catalog:9002"
	}
	catalogConn, err := grpcx.Dial(ctx, catalogAddr)
	if err != nil {
		logger.Error("dial catalog", "error", err)
		os.Exit(1)
	}
	defer catalogConn.Close()

	maxAge := 24 * time.Hour
	if raw := strings.TrimSpace(os.Getenv("RAG_BUILD_MAX_AGE")); raw != "" {
		parsed, parseErr := time.ParseDuration(raw)
		if parseErr != nil || parsed < 0 {
			logger.Error("invalid RAG_BUILD_MAX_AGE (use 0 to always rebuild)", "value", raw, "error", parseErr)
			os.Exit(1)
		}
		maxAge = parsed
	}
	force := strings.EqualFold(strings.TrimSpace(os.Getenv("RAG_BUILD_FORCE")), "true")
	ragIndex := strings.TrimSpace(os.Getenv("RAG_INDEX"))
	due, current, err := ai.RAGBuildDue(ctx, cache, ragIndex, maxAge, force)
	if err != nil {
		logger.Error("read rag current", "error", err)
		os.Exit(1)
	}
	if !due {
		logger.Info("rag build skipped: current index is fresh",
			"index", current.Index, "prefix", current.Prefix, "max_age", maxAge.String())
		return
	}

	buildCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	built, err := ai.OpenRAG(buildCtx, ai.RAGConfig{
		Index:    ragIndex,
		Provider: cfg.EmbeddingProvider,
		BaseURL:  cfg.EmbeddingBaseURL,
		APIKey:   cfg.EmbeddingAPIKey,
		Model:    cfg.EmbeddingModel,
		Dim:      cfg.EmbeddingDim,
	}, pb.NewCatalogServiceClient(catalogConn), cache, logger)
	if err != nil {
		// AI is optional: a failed build must not block the mall release. The
		// previous `_current` alias keeps serving, the scheduled Job retries,
		// and the error is visible in this Job's log. Redis/config errors above
		// still fail fast because they indicate a broken deployment.
		logger.Error("rag build failed; leaving the previous alias serving", "error", err)
		return
	}
	index, prefix := built.ServingVersion()
	logger.Info("rag build complete", "index", index, "prefix", prefix, "max_age", maxAge.String())
}

func runService(cfg *config.Config, logger *slog.Logger) {
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

	rag := ai.NewLazyRAG()
	workers := runtime.NewWorkers()
	defer workers.Stop()
	if cfg.RAGEnabled {
		refresh := time.Minute
		if raw := strings.TrimSpace(os.Getenv("RAG_ALIAS_REFRESH")); raw != "" {
			parsed, parseErr := time.ParseDuration(raw)
			if parseErr != nil || parsed <= 0 {
				logger.Warn("invalid RAG_ALIAS_REFRESH; using default", "value", raw, "error", parseErr)
			} else {
				refresh = parsed
			}
		}
		// Load only the version published by the build Job. The embedding
		// provider being unreachable no longer turns a Pod into a builder.
		workers.Go(func(ctx context.Context) {
			ai.RunRAGFollower(ctx, ai.RAGConfig{
				Index:    strings.TrimSpace(os.Getenv("RAG_INDEX")),
				Provider: cfg.EmbeddingProvider,
				BaseURL:  cfg.EmbeddingBaseURL,
				APIKey:   cfg.EmbeddingAPIKey,
				Model:    cfg.EmbeddingModel,
				Dim:      cfg.EmbeddingDim,
			}, cache, logger, refresh, rag.Set)
		})
	}
	if cfg.RAGEnabled {
		natsClient, err := mq.Connect(cfg.NATSURL)
		if err != nil {
			logger.Warn("ai catalog index consumer disabled", "error", err)
		} else {
			defer natsClient.Close()
			workers.Go(func(ctx context.Context) {
				ai.RunCatalogIndexConsumer(ctx, natsClient, catalog, rag, logger)
			})
			workers.Go(func(ctx context.Context) {
				ai.RunRAGReconcile(ctx, rag, catalog, logger, 15*time.Minute)
			})
		}
	}

	var ragDep ai.RAGSearcher
	if cfg.RAGEnabled {
		ragDep = rag
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
		RAG:        ragDep,
		Drafts:     ai.NewCartDraftWriter(cache),
	}, cache, logger)
	if err != nil {
		logger.Error("build ai service", "error", err)
		os.Exit(1)
	}
	if !cfg.AIEnabled {
		logger.Warn("ai assistant is disabled", "AI_ENABLED", "false")
	}

	if cfg.AIEnabled {
		workers.Go(func(ctx context.Context) {
			ai.RunOpsBriefScheduler(ctx, cache, svc, logger, time.Hour,
				os.Getenv("OPS_BRIEF_WEBHOOK_URL"), os.Getenv("OPS_BRIEF_WEBHOOK_SECRET"))
		})
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
