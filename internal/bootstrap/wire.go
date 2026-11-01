//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.
package bootstrap

import (
	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/data"
	"gin-kratos-gorm/internal/pkg"
	"gin-kratos-gorm/internal/server"
	"gin-kratos-gorm/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

func InitApp(string, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(
		biz.ProviderSet,
		pkg.InfraSet,
		data.ProviderSet,
		service.ProviderSet,
		server.TransportSet,
		NewSaga,
		NewApp,
	))
}
