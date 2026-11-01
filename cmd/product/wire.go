//go:build wireinject
// +build wireinject

package main

import (
	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/data"
	"gin-kratos-gorm/internal/data/persistence"
	"gin-kratos-gorm/internal/pkg"
	"gin-kratos-gorm/internal/pkg/registry"
	"gin-kratos-gorm/internal/server"
	httpServerHandler "gin-kratos-gorm/internal/server/http/handler"
	"gin-kratos-gorm/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

func wireApp(string, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(
		pkg.InfraSet,
		registry.New,
		persistence.NewData,
		persistence.NewProductRepo,
		data.NewEnforcer,
		wire.Bind(new(biz.ProductRepository), new(*persistence.ProductRepo)),
		biz.NewProductUsecase,
		service.NewProductService,
		httpServerHandler.NewProductHandler,
		server.NewProductHTTPServer,
		server.NewProductGRPCServer,
		newApp,
	))
}
