package main

import (
	"context"
	"log/slog"
	"os"

	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/platform/registry"
	"gin-kratos-gorm/internal/services/gateway"

	"github.com/gin-gonic/gin"
	etcdregistry "github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2"
	kratoslog "github.com/go-kratos/kratos/v2/log"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

const listenAddr = ":8080"

func provideLogger(cfg *config.Config) *slog.Logger {
	return log.New(cfg.Service, cfg.Env)
}

func provideRegistry(cfg *config.Config) (*etcdregistry.Registry, func(), error) {
	return registry.New(cfg.ETCDEndpoints)
}

func provideClients(ctx context.Context, reg *etcdregistry.Registry) (*gateway.Clients, func(), error) {
	clients, err := gateway.Dial(ctx, reg)
	if err != nil {
		return nil, nil, err
	}
	return clients, func() { _ = clients.Close() }, nil
}

func provideAuth(cfg *config.Config) *auth.Manager {
	return auth.NewManager(cfg)
}

func provideEngine(logger *slog.Logger, jwt *auth.Manager, clients *gateway.Clients) *gin.Engine {
	return gateway.NewEngine(logger, jwt, clients)
}

func provideHTTPServer(engine *gin.Engine) *khttp.Server {
	srv := khttp.NewServer(khttp.Address(listenAddr))
	srv.HandlePrefix("/", engine)
	return srv
}

func provideApp(cfg *config.Config, logger *slog.Logger, srv *khttp.Server, reg *etcdregistry.Registry) *kratos.App {
	opts := []kratos.Option{
		kratos.Name(cfg.Service),
		kratos.Logger(kratoslog.NewStdLogger(os.Stdout)),
		kratos.Server(srv),
	}
	if reg != nil {
		opts = append(opts, kratos.Registrar(reg))
	}
	return kratos.New(opts...)
}
