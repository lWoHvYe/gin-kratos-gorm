package main

import (
	"context"
	"log/slog"

	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/platform/registry"
	"gin-kratos-gorm/internal/services/admin"

	"github.com/gin-gonic/gin"
	etcdregistry "github.com/go-kratos/kratos/contrib/registry/etcd/v3"
	"github.com/go-kratos/kratos/v3"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

const listenAddr = ":8081"

func provideLogger(cfg *config.Config) *slog.Logger {
	return log.New(cfg.Service, cfg.Env)
}

func provideRegistry(cfg *config.Config) (*etcdregistry.Registry, func(), error) {
	return registry.New(cfg.ETCDEndpoints)
}

func provideClients(ctx context.Context, reg *etcdregistry.Registry) (*admin.Clients, func(), error) {
	clients, err := admin.Dial(ctx, reg)
	if err != nil {
		return nil, nil, err
	}
	return clients, func() { _ = clients.Close() }, nil
}

func provideAuth(cfg *config.Config) *auth.Manager {
	return auth.NewManager(cfg)
}

func provideEngine(logger *slog.Logger, jwt *auth.Manager, clients *admin.Clients) *gin.Engine {
	return admin.NewEngine(logger, jwt, clients)
}

func provideHTTPServer(engine *gin.Engine) *khttp.Server {
	srv := khttp.NewServer(khttp.Address(listenAddr))
	srv.HandlePrefix("/", engine)
	return srv
}

func provideApp(cfg *config.Config, logger *slog.Logger, srv *khttp.Server, reg *etcdregistry.Registry) *kratos.App {
	opts := []kratos.Option{
		kratos.Name(cfg.Service),
		kratos.Logger(logger),
		kratos.Server(srv),
	}
	if reg != nil {
		opts = append(opts, kratos.Registrar(reg))
	}
	return kratos.New(opts...)
}
