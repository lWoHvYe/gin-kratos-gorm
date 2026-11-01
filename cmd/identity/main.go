package main

import (
	"os"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/grpcx"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/platform/registry"
	"gin-kratos-gorm/internal/platform/runtime"
	"gin-kratos-gorm/internal/services/identity"
)

func main() {
	cfg, err := config.Load("identity")
	if err != nil {
		panic(err)
	}
	logger := log.New(cfg.Service, cfg.Env)
	database, err := db.Open(cfg.MySQLDSN)
	if err != nil {
		logger.Error("open db", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	reg, closeReg, err := registry.New(cfg.ETCDEndpoints)
	if err != nil {
		logger.Error("registry", "error", err)
		os.Exit(1)
	}
	defer closeReg()
	gs := grpcx.NewServer(logger)
	svc := identity.New(database, auth.NewManager(cfg), logger)
	pb.RegisterIdentityServiceServer(gs, svc)
	app := runtime.NewApp(cfg, logger, gs, runtime.HealthServer(cfg, database, logger), reg)
	if err := app.Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}
