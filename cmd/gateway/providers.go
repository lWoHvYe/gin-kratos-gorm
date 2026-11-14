package main

import (
	"context"
	"log/slog"

	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/services/gateway"

	"github.com/gin-gonic/gin"
	"github.com/go-kratos/kratos/v3"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

func provideLogger(cfg *config.Config) *slog.Logger {
	return log.New(cfg.Service, cfg.Env)
}

func provideClients(ctx context.Context) (*gateway.Clients, func(), error) {
	clients, err := gateway.Dial(ctx)
	if err != nil {
		return nil, nil, err
	}
	return clients, func() { _ = clients.Close() }, nil
}

func provideAuth(cfg *config.Config) *auth.Manager {
	return auth.NewManager(cfg)
}

func provideEngine(cfg *config.Config, logger *slog.Logger, jwt *auth.Manager, clients *gateway.Clients) *gin.Engine {
	return gateway.NewEngine(logger, jwt, clients, gateway.Options{
		PaymentMockEnabled: cfg.PaymentMockEnabled,
	})
}

func provideHTTPServer(cfg *config.Config, engine *gin.Engine) *khttp.Server {
	srv := khttp.NewServer(khttp.Address(cfg.HTTPAddr))
	srv.HandlePrefix("/", engine)
	return srv
}

func provideApp(cfg *config.Config, logger *slog.Logger, srv *khttp.Server) *kratos.App {
	return kratos.New(
		kratos.Name(cfg.Service),
		kratos.Logger(logger),
		kratos.Server(srv),
	)
}
