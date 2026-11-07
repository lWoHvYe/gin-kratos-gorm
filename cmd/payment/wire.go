//go:build wireinject

package main

import (
	"context"

	"gin-kratos-gorm/internal/platform/config"

	"github.com/google/wire"
)

func initialize(ctx context.Context, cfg *config.Config) (*paymentApp, func(), error) {
	wire.Build(
		provideLogger, provideDB, provideMQ,
		provideOrderConn, provideInventoryConn, providePricingConn,
		providePaymentService, provideGRPCServer, provideHealth, provideApp, providePaymentApp,
	)
	return nil, nil, nil
}
