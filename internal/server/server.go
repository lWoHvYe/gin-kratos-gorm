package server

import (
	"gin-kratos-gorm/internal/data/event"
	httpServerHandler "gin-kratos-gorm/internal/server/http/handler"

	"github.com/google/wire"
)

// TransportSet wires HTTP/gRPC transports, handlers and background workers.
var TransportSet = wire.NewSet(
	NewGRPCServer,
	NewHTTPServer,
	NewWorker,
	event.NewDispatcher,
	httpServerHandler.NewUserHandler,
	httpServerHandler.NewProductHandler,
	httpServerHandler.NewOrderHandler,
)
