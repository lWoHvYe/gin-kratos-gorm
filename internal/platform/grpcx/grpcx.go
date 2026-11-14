package grpcx

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/metrics"

	kratosmiddleware "github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/transport"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewServer(cfg *config.Config, logger *slog.Logger) *kratosgrpc.Server {
	opts := []kratosgrpc.ServerOption{
		kratosgrpc.Middleware(
			recovery.Recovery(),
			accessLog(logger),
			errorMapper(),
		),
		kratosgrpc.Options(grpc.StatsHandler(otelgrpc.NewServerHandler())),
	}
	if cfg != nil {
		if addr := strings.TrimSpace(cfg.GRPCAddr); addr != "" {
			opts = append(opts, kratosgrpc.Address(addr))
		}
	}
	return kratosgrpc.NewServer(opts...)
}

// accessLog records routing metadata only. Request and response payloads are
// never logged: LoginRequest/RefreshRequest/CallbackRequest carry passwords,
// refresh tokens and payment signatures.
func accessLog(logger *slog.Logger) kratosmiddleware.Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(handler kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			start := time.Now()
			reply, err := handler(ctx, req)
			operation := "unknown"
			if tr, ok := transport.FromServerContext(ctx); ok {
				operation = tr.Operation()
			}
			attrs := []any{
				slog.String("kind", "server"),
				slog.String("operation", operation),
				slog.Float64("latency_ms", float64(time.Since(start).Milliseconds())),
			}
			if err != nil {
				ae := apperr.From(err)
				code := string(ae.Code)
				metrics.GRPCDuration.WithLabelValues(operation, code).Observe(time.Since(start).Seconds())
				attrs = append(attrs, slog.String("code", code))
				metrics.Errors.WithLabelValues("grpc", code).Inc()
				// The wrapped cause is logged here, before errorMapper replaces
				// it with a bare gRPC status; otherwise 5xx root causes vanish.
				attrs = append(attrs, slog.Any("error", err))
				level := slog.LevelWarn
				if ae.Code == apperr.CodeInternal || ae.Code == apperr.CodeUnavailable {
					level = slog.LevelError
				}
				logger.Log(ctx, level, "grpc request", attrs...)
				return reply, err
			}
			metrics.GRPCDuration.WithLabelValues(operation, "OK").Observe(time.Since(start).Seconds())
			attrs = append(attrs, slog.String("code", "OK"))
			logger.Log(ctx, slog.LevelInfo, "grpc request", attrs...)
			return reply, nil
		}
	}
}

func errorMapper() kratosmiddleware.Middleware {
	return func(handler kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			reply, err := handler(ctx, req)
			if err != nil {
				return reply, apperr.GRPC(err)
			}
			return reply, nil
		}
	}
}

// Dial connects to a Kubernetes Service DNS name or a direct host:port. Service
// discovery is owned by Kubernetes/Ambient, not by the application.
func Dial(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	return grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
}
