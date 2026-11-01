package grpcx

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"gin-kratos-gorm/internal/platform/apperr"

	etcdregistry "github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2/log"
	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewServer(logger *slog.Logger) *kratosgrpc.Server {
	opts := []kratosgrpc.ServerOption{
		kratosgrpc.Middleware(
			recovery.Recovery(),
			logging.Server(log.NewStdLogger(os.Stdout)),
			errorMapper(),
		),
	}
	// GRPC_ENDPOINT is the address registered in etcd and dialed by other
	// services (for example grpc://identity:9001). Without it Kratos would
	// register the local listen address, which is not routable from peers.
	if endpoint := strings.TrimSpace(os.Getenv("GRPC_ENDPOINT")); endpoint != "" {
		if u, err := url.Parse(endpoint); err == nil {
			opts = append(opts, kratosgrpc.Endpoint(u))
		}
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

func Dial(ctx context.Context, endpoint string, reg *etcdregistry.Registry) (*grpc.ClientConn, error) {
	if strings.HasPrefix(endpoint, "discovery:///") && reg != nil {
		return kratosgrpc.DialInsecure(ctx, kratosgrpc.WithEndpoint(endpoint), kratosgrpc.WithDiscovery(reg))
	}
	return grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
}
