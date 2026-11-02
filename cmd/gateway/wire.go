//go:build wireinject

package main

import (
	"context"

	"gin-kratos-gorm/internal/platform/config"

	"github.com/go-kratos/kratos/v2"
	"github.com/google/wire"
)

func initialize(ctx context.Context, cfg *config.Config) (*kratos.App, func(), error) {
	wire.Build(
		provideLogger,
		provideRegistry,
		provideClients,
		provideAuth,
		provideEngine,
		provideHTTPServer,
		provideApp,
	)
	return nil, nil, nil
}
