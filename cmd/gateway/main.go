package main

import (
	"context"
	"os"
	"time"

	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/config"
	"gin-kratos-gorm/internal/platform/log"
	"gin-kratos-gorm/internal/platform/registry"
	"gin-kratos-gorm/internal/services/gateway"

	"github.com/go-kratos/kratos/v2"
	kratoslog "github.com/go-kratos/kratos/v2/log"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

// listenAddr is the public HTTP entry point of the mall (see docs/contracts.md).
const listenAddr = ":8080"

func main() {
	cfg, err := config.Load("gateway")
	if err != nil {
		panic(err)
	}
	logger := log.New(cfg.Service, cfg.Env)
	reg, closeReg, err := registry.New(cfg.ETCDEndpoints)
	if err != nil {
		logger.Error("registry", "error", err)
		os.Exit(1)
	}
	defer closeReg()

	dialCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clients, err := gateway.Dial(dialCtx, reg)
	if err != nil {
		logger.Error("dial downstream services", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := clients.Close(); err != nil {
			logger.Warn("close downstream clients", "error", err)
		}
	}()

	engine := gateway.NewEngine(logger, auth.NewManager(cfg), clients)
	srv := khttp.NewServer(khttp.Address(listenAddr))
	srv.HandlePrefix("/", engine)

	opts := []kratos.Option{
		kratos.Name(cfg.Service),
		kratos.Logger(kratoslog.NewStdLogger(os.Stdout)),
		kratos.Server(srv),
	}
	if reg != nil {
		opts = append(opts, kratos.Registrar(reg))
	}
	if err := kratos.New(opts...).Run(); err != nil {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}
