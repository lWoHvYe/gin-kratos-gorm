package biz

import (
	"time"

	"gin-kratos-gorm/internal/conf"
	"gin-kratos-gorm/internal/pkg/idgen"

	"github.com/google/wire"
)

// ProviderSet wires domain use cases and the small amount of configuration they
// need. Transaction/Saga implementations are provided by each composition root
// because they differ between the monolith and split deployments.
var ProviderSet = wire.NewSet(
	NewUserUsecase,
	NewProductUsecase,
	NewOrderUsecase,
	NewClock,
	NewOrderSNGenerator,
	NewOrderConfig,
)

// Transaction is kept as an alias for the original repository API.
type Transaction = Transactor

func NewClock() Clock { return RealClock{} }

func NewOrderSNGenerator() *idgen.OrderSN { return idgen.NewOrderSN("") }

func NewOrderConfig(c *conf.Bootstrap) OrderConfig {
	cfg := OrderConfig{
		PaymentTimeout:        30 * time.Minute,
		ShippingFee:           10,
		FreeShippingThreshold: 99,
	}
	if c != nil && c.Order != nil {
		if c.Order.PaymentTimeout != nil && c.Order.PaymentTimeout.AsDuration() > 0 {
			cfg.PaymentTimeout = c.Order.PaymentTimeout.AsDuration()
		}
		cfg.ShippingFee = c.Order.ShippingFee
		cfg.FreeShippingThreshold = c.Order.FreeShippingThreshold
	}
	return cfg
}
