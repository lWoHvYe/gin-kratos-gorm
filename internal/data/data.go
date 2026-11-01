package data

import (
	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/data/persistence"

	"github.com/google/wire"
)

// ProviderSet wires repositories and their domain interfaces.
var ProviderSet = wire.NewSet(
	persistence.NewData,
	persistence.NewUserRepo,
	persistence.NewProductRepo,
	persistence.NewOrderRepo,
	persistence.NewOutboxRepo,
	NewEnforcer,
	wire.Bind(new(biz.UserRepository), new(*persistence.UserRepo)),
	wire.Bind(new(biz.ProductRepository), new(*persistence.ProductRepo)),
	wire.Bind(new(biz.ProductCatalog), new(*persistence.ProductRepo)),
	wire.Bind(new(biz.OrderRepository), new(*persistence.OrderRepo)),
	wire.Bind(new(biz.Transactor), new(*persistence.Data)),
)

// RepoSet is kept as an alias for older composition roots.
var RepoSet = ProviderSet
