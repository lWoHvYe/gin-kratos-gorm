//go:build wireinject

package main

import (
	"context"

	"gin-kratos-gorm/internal/platform/config"

	"github.com/google/wire"
)

func initialize(ctx context.Context, cfg *config.Config) (*orderApp, func(), error) {
	wire.Build(
		provideLogger,
		provideDB,
		provideMQ,
		provideRegistry,
		providePricingConn,
		provideInventoryConn,
		providePricingClient,
		provideInventoryClient,
		provideDTMClient,
		provideOrderService,
		provideGRPCServer,
		provideHealthServer,
		provideApp,
		provideOrderApp,
	)
	return nil, nil, nil
}
