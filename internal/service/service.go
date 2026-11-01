package service

import (
	"github.com/google/wire"
)

// ProviderSet wires the application services.
var ProviderSet = wire.NewSet(NewUserService, NewProductService, NewOrderService, NewDtmCoordinator)
