package main

import (
	"context"

	"gin-kratos-gorm/internal/platform/config"
)

func main() {
	cfg, err := config.Load("admin")
	if err != nil {
		panic(err)
	}
	app, cleanup, err := initialize(context.Background(), cfg)
	if err != nil {
		panic(err)
	}
	defer cleanup()
	if err := app.Run(); err != nil {
		panic(err)
	}
}
