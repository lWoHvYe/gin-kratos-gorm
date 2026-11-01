// Package db opens the application database. It supports MySQL for production
// and SQLite for local development and tests, so integration tests never need
// external infrastructure (MySQL/etcd/DTM).
package db

import (
	"fmt"
	"strings"
	"time"

	"gin-kratos-gorm/internal/biz/model"
	"gin-kratos-gorm/internal/conf"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// NewDB opens the configured database and applies pool settings.
func NewDB(cfg *conf.Bootstrap) (*gorm.DB, error) {
	if cfg == nil || cfg.Data == nil || cfg.Data.Database == nil {
		return nil, fmt.Errorf("database config is missing")
	}
	dc := cfg.Data.Database

	logLevel := gormlogger.Warn
	if cfg.App != nil && cfg.App.Env == "dev" {
		logLevel = gormlogger.Info
	}

	gormCfg := &gorm.Config{
		Logger:                                   gormlogger.Default.LogMode(logLevel),
		DisableForeignKeyConstraintWhenMigrating: true,
		TranslateError:                           true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: false,
		},
	}

	var dialector gorm.Dialector
	switch strings.ToLower(dc.Driver) {
	case "mysql":
		dialector = mysql.Open(dc.Source)
	case "sqlite":
		dialector = sqlite.Open(dc.Source)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", dc.Driver)
	}

	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, fmt.Errorf("open %s database: %w", dc.Driver, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	maxOpen := int(dc.MaxOpenConns)
	// SQLite in-memory databases are per-connection; a single connection keeps
	// the schema and data visible to every query.
	if strings.EqualFold(dc.Driver, "sqlite") && strings.Contains(dc.Source, "memory") {
		maxOpen = 1
	}
	if maxOpen > 0 {
		sqlDB.SetMaxOpenConns(maxOpen)
	}
	if dc.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(int(dc.MaxIdleConns))
	}
	if dc.ConnMaxLifetime != nil && dc.ConnMaxLifetime.AsDuration() > 0 {
		sqlDB.SetConnMaxLifetime(dc.ConnMaxLifetime.AsDuration())
	}
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	if dc.AutoMigrate {
		if err := Migrate(db); err != nil {
			return nil, fmt.Errorf("auto migrate: %w", err)
		}
	}
	if dc.Seed {
		if err := Seed(db); err != nil {
			return nil, fmt.Errorf("seed: %w", err)
		}
	}
	return db, nil
}

// Migrate creates/updates the schema for all domain tables.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.Role{},
		&model.UserRole{},
		&model.ProductSpu{},
		&model.ProductSku{},
		&model.StockLog{},
		&model.Order{},
		&model.OrderItem{},
		&model.OutboxEvent{},
	)
}

// Close closes the underlying SQL pool.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
