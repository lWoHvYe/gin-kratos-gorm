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
)

// Config holds the assistant knobs that are not model-provider specific.
type Config struct {
	Enabled    bool
	Timeout    time.Duration
	RateLimit  int
	RateWindow time.Duration
}

// Service streams assistant turns. It never touches business state directly:
// the agent can only call the read-only tools it was built with.
type Service struct {
	pb.UnimplementedAIServiceServer

	cfg      Config
	agent    agentRunner
	sessions sessionStore
	limiter  limiter
	log      *slog.Logger
}

// NewService wires the agent, Redis session store and per-user limiter. The
// agent is only built when the feature is enabled, so a disabled assistant does
// not dial the model or the catalog.
func NewService(ctx context.Context, cfg Config, provider ProviderConfig, catalog catalogClient, cache redis.UniversalClient, logger *slog.Logger) (*Service, error) {
	var runner agentRunner
	if cfg.Enabled {
		built, err := buildAgent(ctx, provider, catalog)
		if err != nil {
			return nil, err
		}
		runner = built
	}
	return New(cfg, runner, newSessionStore(cache), newLimiter(cache, cfg.RateLimit, cfg.RateWindow), logger), nil
}

func New(cfg Config, agent agentRunner, sessions sessionStore, limit limiter, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = streamTimeout
	}
	return &Service{cfg: cfg, agent: agent, sessions: sessions, limiter: limit, log: logger}
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

	history, err := s.sessions.Load(ctx, req.GetUserId(), sessionID)
	if err != nil {
		result = "error"
		s.log.Warn("ai session load failed", "user_id", req.GetUserId(), "error", err)
		return status.Error(codes.Unavailable, "assistant is temporarily unavailable")
	}
	turns := append(history, &schema.Message{Role: schema.User, Content: message})

	out, err := s.agent.Stream(ctx, turns)
	if err != nil {
		result = "error"
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
			s.log.Warn("ai stream recv failed", "user_id", req.GetUserId(), "error", recvErr)
			_ = stream.Send(&pb.AIChatEvent{Type: "error", Error: "assistant stream interrupted"})
			return nil
		}
		if chunk == nil {
			continue
		}
		if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
			u := chunk.ResponseMeta.Usage
			metrics.AITokens.WithLabelValues("prompt").Add(float64(u.PromptTokens))
			metrics.AITokens.WithLabelValues("completion").Add(float64(u.CompletionTokens))
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
		_ = stream.Send(&pb.AIChatEvent{Type: "error", Error: "empty assistant response"})
		return nil
	}
	if err := s.sessions.Append(ctx, req.GetUserId(), sessionID,
		&schema.Message{Role: schema.User, Content: message},
		&schema.Message{Role: schema.Assistant, Content: answer.String()}); err != nil {
		// A failed history write must not fail a delivered answer.
		s.log.Warn("ai session append failed", "user_id", req.GetUserId(), "error", err)
	}
	s.log.Info("ai chat", "user_id", req.GetUserId(), "session_id", sessionID,
		"latency_ms", time.Since(start).Milliseconds(), "chars", answer.Len())
	return stream.Send(&pb.AIChatEvent{Type: "done"})
}
