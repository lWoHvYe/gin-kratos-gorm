package grpcx

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"gin-kratos-gorm/internal/platform/apperr"

	kratosmiddleware "github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/logging"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewServer(logger *slog.Logger) *kratosgrpc.Server {
	opts := []kratosgrpc.ServerOption{
		kratosgrpc.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
			errorMapper(),
		),
	}
	if addr := strings.TrimSpace(os.Getenv("GRPC_ADDR")); addr != "" {
		opts = append(opts, kratosgrpc.Address(addr))
	}
	return kratosgrpc.NewServer(opts...)
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
	return grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
}
