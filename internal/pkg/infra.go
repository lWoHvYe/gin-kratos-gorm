package pkg

import (
	"gin-kratos-gorm/internal/data/db"
	"gin-kratos-gorm/internal/pkg/auth"
	"gin-kratos-gorm/internal/pkg/config"

	"github.com/google/wire"
)

// InfraSet provides configuration, the database handle and the JWT manager.
var InfraSet = wire.NewSet(
	config.NewConfig,
	config.NewAuth,
	db.NewDB,
	auth.NewManager,
)
