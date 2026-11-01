//go:build wireinject
// +build wireinject

package main

import (
	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/data/event"
	"gin-kratos-gorm/internal/data/persistence"
	"gin-kratos-gorm/internal/pkg"
	"gin-kratos-gorm/internal/pkg/grpcclient"
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
		grpcclient.NewProductCatalog,
		persistence.NewData,
		persistence.NewOrderRepo,
		persistence.NewOutboxRepo,
		wire.Bind(new(biz.OrderRepository), new(*persistence.OrderRepo)),
		wire.Bind(new(biz.ProductCatalog), new(*grpcclient.ProductCatalog)),
		biz.NewOrderUsecase,
		biz.NewClock,
		biz.NewOrderSNGenerator,
		biz.NewOrderConfig,
		service.NewOrderService,
		service.NewDtmCoordinator,
		httpServerHandler.NewOrderHandler,
		server.NewOrderHTTPServer,
		server.NewOrderGRPCServer,
		server.NewWorker,
		event.NewDispatcher,
		NewSaga,
		newApp,
	))
}
