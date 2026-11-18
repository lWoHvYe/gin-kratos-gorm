package ai

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/metrics"

	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	maxMessageRunes = 2000
	streamTimeout   = 120 * time.Second
	// promptVersion changes whenever systemPrompt or adminSystemPrompt changes.
	promptVersion = "2026-11-19"
)

// Config holds the assistant knobs that are not model-provider specific.
type Config struct {
	Enabled    bool
	Timeout    time.Duration
	RateLimit  int
	RateWindow time.Duration
	Provider   string
	Model      string

	DailyTokenBudget       int64
	GlobalDailyTokenBudget int64
	PricePromptPer1K       int64
	PriceCompletionPer1K   int64
	CircuitFailures        int
	CircuitCooldown        time.Duration
}

// Service streams assistant turns. It never touches business state directly:
// the agent can only call the read-only tools it was built with.
type Service struct {
	pb.UnimplementedAIServiceServer

	cfg        Config
	agent      agentRunner
	adminAgent agentRunner
	sessions   sessionStore
	limiter    limiter
	usage      usageStore
	cache      redis.UniversalClient
	breaker    *circuitBreaker
	// budgets/prices are copied out of cfg for cheap per-request access.
	dailyBudget      int64
	globalBudget     int64
	pricePromptPer1K int64
	priceCompletion  int64
	log              *slog.Logger
}

// NewService wires the agent, Redis session store and per-user limiter. The
// agent is only built when the feature is enabled, so a disabled assistant does
// not dial the model or the catalog.
func NewService(ctx context.Context, cfg Config, provider ProviderConfig, d deps, cache redis.UniversalClient, logger *slog.Logger) (*Service, error) {
	cfg.Provider = provider.Provider
	cfg.Model = provider.Model
	metrics.AIInfo.WithLabelValues(provider.Provider, provider.Model, promptVersion).Set(1)
	var runner, adminRunner agentRunner
	if cfg.Enabled {
		built, err := buildADKAgent(ctx, provider, d, false)
		if err != nil {
			return nil, err
		}
		runner = built
		adminBuilt, err := buildADKAgent(ctx, provider, d, true)
		if err != nil {
			return nil, err
		}
		adminRunner = adminBuilt
	}
	svc := New(cfg, runner, newSessionStore(cache), newLimiter(cache, cfg.RateLimit, cfg.RateWindow), logger)
	svc.cache = cache
	svc.adminAgent = adminRunner
	svc.dailyBudget = cfg.DailyTokenBudget
	svc.globalBudget = cfg.GlobalDailyTokenBudget
	svc.pricePromptPer1K = cfg.PricePromptPer1K
	svc.priceCompletion = cfg.PriceCompletionPer1K
	svc.breaker = newCircuitBreaker(cfg.CircuitFailures, cfg.CircuitCooldown)
	if svc.dailyBudget > 0 || svc.globalBudget > 0 {
		svc.usage = newUsageStore(cache)
	}
	return svc, nil
}

func New(cfg Config, agent agentRunner, sessions sessionStore, limit limiter, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = streamTimeout
	}
	return &Service{cfg: cfg, agent: agent, sessions: sessions, limiter: limit,
		breaker: newCircuitBreaker(5, time.Minute), log: logger}
}

func (s *Service) Chat(req *pb.AIChatRequest, stream pb.AIService_ChatServer) error {
	start := time.Now()
	result := "ok"
	defer func() {
		metrics.AIRequests.WithLabelValues(result).Inc()
		metrics.AIDuration.WithLabelValues(result).Observe(time.Since(start).Seconds())
	}()

	if !s.cfg.Enabled {
		result = "disabled"
		return status.Error(codes.Unavailable, "ai assistant is disabled")
	}
	if req == nil || req.GetUserId() <= 0 {
		result = "invalid"
		return status.Error(codes.InvalidArgument, "user id is required")
	}
	message := strings.TrimSpace(req.GetMessage())
	if message == "" {
		result = "invalid"
		return status.Error(codes.InvalidArgument, "message is empty")
	}
	if utf8.RuneCountInString(message) > maxMessageRunes {
		result = "invalid"
		return status.Error(codes.InvalidArgument, "message is too long")
	}
	sessionID := strings.TrimSpace(req.GetSessionId())
	if sessionID == "" {
		sessionID = "default"
	}
	if len(sessionID) > 64 {
		sessionID = sessionID[:64]
	}

	ctx, cancel := context.WithTimeout(stream.Context(), s.cfg.Timeout)
	defer cancel()

	if s.breaker != nil && !s.breaker.Allow() {
		result = "circuit_open"
		return status.Error(codes.Unavailable, "assistant is temporarily unavailable")
	}
	if s.usage != nil {
		used, err := s.usage.Load(ctx, req.GetUserId())
		if err != nil {
			result = "error"
			s.log.Warn("ai usage load failed", "user_id", req.GetUserId(), "error", err)
			return status.Error(codes.Unavailable, "assistant is temporarily unavailable")
		}
		if s.dailyBudget > 0 && used >= s.dailyBudget {
			result = "budget"
			metrics.AIBudgetExceeded.WithLabelValues("user").Inc()
			return status.Error(codes.ResourceExhausted, "daily token budget exceeded")
		}
		if s.globalBudget > 0 {
			global, err := s.usage.LoadGlobal(ctx)
			if err != nil {
				result = "error"
				s.log.Warn("ai global usage load failed", "error", err)
				return status.Error(codes.Unavailable, "assistant is temporarily unavailable")
			}
			if global >= s.globalBudget {
				result = "budget"
				metrics.AIBudgetExceeded.WithLabelValues("global").Inc()
				return status.Error(codes.ResourceExhausted, "global token budget exceeded")
			}
		}
	}

	allowed, err := s.limiter.Allow(ctx, req.GetUserId())
	if err != nil {
		result = "error"
		s.log.Warn("ai rate limiter failed", "user_id", req.GetUserId(), "error", err)
		return status.Error(codes.Unavailable, "assistant is temporarily unavailable")
	}
	if !allowed {
		result = "rate_limited"
		metrics.AIRateLimited.Inc()
		return status.Error(codes.ResourceExhausted, "assistant rate limit exceeded")
	}

	// Tools read the user id from this context; model arguments can never
	// select another user's data.
	ctx = withUserID(ctx, req.GetUserId())

	history, err := s.sessions.Load(ctx, req.GetUserId(), sessionID)
	if err != nil {
		result = "error"
		s.log.Warn("ai session load failed", "user_id", req.GetUserId(), "error", err)
		return status.Error(codes.Unavailable, "assistant is temporarily unavailable")
	}
	turns := append(history, &schema.Message{Role: schema.User, Content: message})

	runner := s.agent
	if req.GetAdmin() {
		runner = s.adminAgent
	}
	if runner == nil {
		result = "disabled"
		return status.Error(codes.Unavailable, "assistant is not available")
	}
	var promptTokens, completionTokens int64
	defer func() {
		s.recordUsage(ctx, req.GetUserId(), promptTokens, completionTokens)
	}()
	out, err := runner.Stream(ctx, turns)
	if err != nil {
		result = "error"
		s.breaker.Failure()
		s.log.Warn("ai stream failed", "user_id", req.GetUserId(), "error", err)
		_ = stream.Send(&pb.AIChatEvent{Type: "error", Error: "assistant is temporarily unavailable"})
		return nil
	}
	defer out.Close()

	var answer strings.Builder
	for {
		chunk, recvErr := out.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			result = "error"
			s.breaker.Failure()
			s.log.Warn("ai stream recv failed", "user_id", req.GetUserId(), "error", recvErr)
			_ = stream.Send(&pb.AIChatEvent{Type: "error", Error: "assistant stream interrupted"})
			return nil
		}
		if chunk == nil {
			continue
		}
		if chunk.Role == schema.Tool {
			if chunk.ToolName != "" {
				_ = stream.Send(&pb.AIChatEvent{Type: "tool", Tool: chunk.ToolName})
			}
			continue
		}
		if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
			promptTokens += int64(chunk.ResponseMeta.Usage.PromptTokens)
			completionTokens += int64(chunk.ResponseMeta.Usage.CompletionTokens)
		}
		if chunk.Content == "" {
			continue
		}
		answer.WriteString(chunk.Content)
		if err := stream.Send(&pb.AIChatEvent{Type: "delta", Delta: chunk.Content}); err != nil {
			result = "canceled"
			return err
		}
	}

	if answer.Len() == 0 {
		result = "error"
		s.breaker.Failure()
		_ = stream.Send(&pb.AIChatEvent{Type: "error", Error: "empty assistant response"})
		return nil
	}
	s.breaker.Success()
	if err := s.sessions.Append(ctx, req.GetUserId(), sessionID,
		&schema.Message{Role: schema.User, Content: message},
		&schema.Message{Role: schema.Assistant, Content: answer.String()}); err != nil {
		// A failed history write must not fail a delivered answer.
		s.log.Warn("ai session append failed", "user_id", req.GetUserId(), "error", err)
	}
	s.log.Info("ai chat", "user_id", req.GetUserId(), "session_id", sessionID, "admin", req.GetAdmin(),
		"provider", s.cfg.Provider, "model", s.cfg.Model, "prompt_version", promptVersion,
		"latency_ms", time.Since(start).Milliseconds(), "chars", answer.Len())
	return stream.Send(&pb.AIChatEvent{Type: "done"})
}

// recordUsage persists the token counters and exposes metrics. It runs after a
// provider stream, whether the answer completed or failed mid-stream.
func (s *Service) recordUsage(ctx context.Context, userID int64, prompt, completion int64) {
	if prompt <= 0 && completion <= 0 {
		return
	}
	metrics.AITokens.WithLabelValues("prompt").Add(float64(prompt))
	metrics.AITokens.WithLabelValues("completion").Add(float64(completion))
	costCents := (prompt*s.pricePromptPer1K + completion*s.priceCompletion) / 1000
	if costCents > 0 {
		metrics.AICostCents.WithLabelValues("prompt").Add(float64(prompt * s.pricePromptPer1K / 1000))
		metrics.AICostCents.WithLabelValues("completion").Add(float64(completion * s.priceCompletion / 1000))
	}
	if s.usage == nil {
		return
	}
	if err := s.usage.Add(ctx, userID, prompt, completion, costCents); err != nil {
		s.log.Warn("ai usage add failed", "user_id", userID, "error", err)
	}
	if s.globalBudget > 0 {
		if err := s.usage.AddGlobal(ctx, prompt, completion, costCents); err != nil {
			s.log.Warn("ai global usage add failed", "error", err)
		}
	}
}
