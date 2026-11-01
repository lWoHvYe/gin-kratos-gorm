package server

import (
	"net/url"

	orderV1 "gin-kratos-gorm/api/order/v1"
	productV1 "gin-kratos-gorm/api/product/v1"
	userV1 "gin-kratos-gorm/api/user/v1"
	"gin-kratos-gorm/internal/conf"
	"gin-kratos-gorm/internal/pkg/auth"
	"gin-kratos-gorm/internal/server/middleware"
	"gin-kratos-gorm/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

// NewGRPCServer builds the all-in-one gRPC server.
func NewGRPCServer(cfg *conf.Bootstrap,
	user *service.UserService,
	product *service.ProductService,
	order *service.OrderService,
	jwtManager *auth.Manager,
	logger log.Logger) *grpc.Server {
	return newGRPCServer(cfg, jwtManager, logger, func(srv *grpc.Server) {
		userV1.RegisterUserServer(srv, user)
		productV1.RegisterProductServer(srv, product)
		orderV1.RegisterOrderServer(srv, order)
	})
}

func NewUserGRPCServer(cfg *conf.Bootstrap, user *service.UserService, jwtManager *auth.Manager, logger log.Logger) *grpc.Server {
	return newGRPCServer(cfg, jwtManager, logger, func(srv *grpc.Server) {
		userV1.RegisterUserServer(srv, user)
	})
}

func NewProductGRPCServer(cfg *conf.Bootstrap, product *service.ProductService, jwtManager *auth.Manager, logger log.Logger) *grpc.Server {
	return newGRPCServer(cfg, jwtManager, logger, func(srv *grpc.Server) {
		productV1.RegisterProductServer(srv, product)
	})
}

func NewOrderGRPCServer(cfg *conf.Bootstrap, order *service.OrderService, jwtManager *auth.Manager, logger log.Logger) *grpc.Server {
	return newGRPCServer(cfg, jwtManager, logger, func(srv *grpc.Server) {
		orderV1.RegisterOrderServer(srv, order)
	})
}

func newGRPCServer(cfg *conf.Bootstrap, jwtManager *auth.Manager, logger log.Logger, register func(*grpc.Server)) *grpc.Server {
	opts := []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
			middleware.GRPCErrorMapper(),
			middleware.GRPCAuth(jwtManager),
		),
	}
	if cfg.Server.Grpc.Network != "" {
		opts = append(opts, grpc.Network(cfg.Server.Grpc.Network))
	}
	if cfg.Server.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(cfg.Server.Grpc.Addr))
	}
	if cfg.Server.Grpc.Endpoint != "" {
		if u, err := url.Parse(cfg.Server.Grpc.Endpoint); err == nil {
			opts = append(opts, grpc.Endpoint(u))
		}
	}
	if cfg.Server.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(cfg.Server.Grpc.Timeout.AsDuration()))
	}
	srv := grpc.NewServer(opts...)
	register(srv)
	return srv
}
