package main

import (
	"context"

	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/tracing"
)

func main() {
	cfg, err := config.Load("gateway")
	if err != nil {
		panic(err)
	}
	shutdownTracing, err := tracing.Init(context.Background(), cfg.Service)
	if err != nil {
		panic(err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	app, cleanup, err := initialize(context.Background(), cfg)
	if err != nil {
		panic(err)
	}
	defer cleanup()
	if err := app.Run(); err != nil {
		panic(err)
	}
}
