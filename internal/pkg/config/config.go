package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gin-kratos-gorm/internal/conf"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	TransactionModeLocal = "local"
	TransactionModeDTM   = "dtm"
)

// NewConfig loads configuration from a YAML file and applies environment
// overrides for the values that commonly differ between environments.
//
// Environment variables (all optional):
//
//	APP_ENV, LOG_LEVEL, HTTP_ADDR, GRPC_ADDR,
//	DATABASE_DRIVER, DATABASE_SOURCE, DATABASE_AUTO_MIGRATE, DATABASE_SEED,
//	JWT_SECRET, JWT_ISSUER, JWT_TTL,
//	TRANSACTION_MODE, DTM_ADDRESS, DTM_ENABLED, DTM_FALLBACK_TO_LOCAL,
//	ETCD_ENDPOINTS, PRODUCT_ENDPOINT, ORDER_ENDPOINT,
//	ORDER_PAYMENT_TIMEOUT, ORDER_SHIPPING_FEE
func NewConfig(flagconf string) (*conf.Bootstrap, error) {
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		return nil, fmt.Errorf("load config %q: %w", flagconf, err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		return nil, fmt.Errorf("scan config %q: %w", flagconf, err)
	}

	applyDefaults(&bc)
	applyEnvOverrides(&bc)

	if err := Validate(&bc); err != nil {
		return nil, err
	}
	return &bc, nil
}

// MustNewConfig is a convenience helper for commands and tests.
func MustNewConfig(path string) *conf.Bootstrap {
	cfg, err := NewConfig(path)
	if err != nil {
		panic(err)
	}
	return cfg
}

// NewAuth exposes the auth sub-config to Wire.
func NewAuth(c *conf.Bootstrap) *conf.Auth {
	if c == nil {
		return &conf.Auth{}
	}
	return c.Auth
}

// Validate performs fail-fast validation. A misconfigured service must not
// start and silently fall back to insecure/incomplete behavior.
func Validate(c *conf.Bootstrap) error {
	if c == nil {
		return errors.New("config is nil")
	}
	if c.Server == nil || c.Server.Http == nil || c.Server.Grpc == nil {
		return errors.New("config: server.http and server.grpc are required")
	}
	if c.Server.Http.Addr == "" {
		return errors.New("config: server.http.addr is required")
	}
	if c.Server.Grpc.Addr == "" {
		return errors.New("config: server.grpc.addr is required")
	}
	if c.Data == nil || c.Data.Database == nil {
		return errors.New("config: data.database is required")
	}
	db := c.Data.Database
	switch strings.ToLower(db.Driver) {
	case "mysql", "sqlite":
	default:
		return fmt.Errorf("config: unsupported data.database.driver %q (want mysql or sqlite)", db.Driver)
	}
	if strings.TrimSpace(db.Source) == "" {
		return errors.New("config: data.database.source is required")
	}
	if c.Data.Transaction != nil {
		switch strings.ToLower(c.Data.Transaction.Mode) {
		case "", TransactionModeLocal, TransactionModeDTM:
		default:
			return fmt.Errorf("config: unsupported data.transaction.mode %q", c.Data.Transaction.Mode)
		}
	}
	if c.Data.Dtm != nil && c.Data.Dtm.Enabled && strings.TrimSpace(c.Data.Dtm.Address) == "" {
		return errors.New("config: data.dtm.address is required when dtm is enabled")
	}
	if c.Auth == nil || strings.TrimSpace(c.Auth.JwtSecret) == "" {
		return errors.New("config: auth.jwt_secret is required")
	}
	if len(c.Auth.JwtSecret) < 16 {
		return errors.New("config: auth.jwt_secret must be at least 16 characters")
	}
	if c.Auth.AccessTokenTtl == nil || c.Auth.AccessTokenTtl.AsDuration() <= 0 {
		return errors.New("config: auth.access_token_ttl must be positive")
	}
	return nil
}

func applyDefaults(c *conf.Bootstrap) {
	if c.Server == nil {
		c.Server = &conf.Server{}
	}
	if c.Server.Http == nil {
		c.Server.Http = &conf.Server_HTTP{}
	}
	if c.Server.Grpc == nil {
		c.Server.Grpc = &conf.Server_GRPC{}
	}
	if c.Server.Http.Addr == "" {
		c.Server.Http.Addr = "0.0.0.0:8000"
	}
	if c.Server.Grpc.Addr == "" {
		c.Server.Grpc.Addr = "0.0.0.0:9000"
	}
	if c.Server.Http.Timeout == nil {
		c.Server.Http.Timeout = durationpb.New(10 * time.Second)
	}
	if c.Server.Grpc.Timeout == nil {
		c.Server.Grpc.Timeout = durationpb.New(10 * time.Second)
	}

	if c.Data == nil {
		c.Data = &conf.Data{}
	}
	if c.Data.Database == nil {
		c.Data.Database = &conf.Data_Database{}
	}
	db := c.Data.Database
	if db.Driver == "" {
		db.Driver = "mysql"
	}
	if db.MaxOpenConns == 0 {
		db.MaxOpenConns = 50
	}
	if db.MaxIdleConns == 0 {
		db.MaxIdleConns = 10
	}
	if db.ConnMaxLifetime == nil {
		db.ConnMaxLifetime = durationpb.New(time.Hour)
	}
	if c.Data.Transaction == nil {
		c.Data.Transaction = &conf.Data_Transaction{}
	}
	if c.Data.Transaction.Mode == "" {
		c.Data.Transaction.Mode = TransactionModeLocal
	}
	if c.Data.Dtm == nil {
		c.Data.Dtm = &conf.Data_DTM{}
	}
	if c.Data.Dtm.Timeout == nil {
		c.Data.Dtm.Timeout = durationpb.New(5 * time.Second)
	}

	if c.Auth == nil {
		c.Auth = &conf.Auth{}
	}
	if c.Auth.JwtIssuer == "" {
		c.Auth.JwtIssuer = "gin-kratos-gorm"
	}
	if c.Auth.AccessTokenTtl == nil {
		c.Auth.AccessTokenTtl = durationpb.New(24 * time.Hour)
	}
	if c.Auth.CasbinModelPath == "" {
		c.Auth.CasbinModelPath = "configs/rbac_model"
	}

	if c.Order == nil {
		c.Order = &conf.Order{}
	}
	if c.Order.PaymentTimeout == nil {
		c.Order.PaymentTimeout = durationpb.New(30 * time.Minute)
	}
	if c.Order.AutoCancelInterval == nil {
		c.Order.AutoCancelInterval = durationpb.New(time.Minute)
	}
	if c.Order.ShippingFee == 0 {
		c.Order.ShippingFee = 10
	}
	if c.Order.FreeShippingThreshold == 0 {
		c.Order.FreeShippingThreshold = 99
	}

	if c.App == nil {
		c.App = &conf.App{}
	}
	if c.App.Env == "" {
		c.App.Env = "dev"
	}
	if c.App.LogLevel == "" {
		c.App.LogLevel = "info"
	}
	if c.Services == nil {
		c.Services = &conf.Services{}
	}
	if c.Registry == nil {
		c.Registry = &conf.Registry{}
	}
}

func applyEnvOverrides(c *conf.Bootstrap) {
	if v := os.Getenv("APP_ENV"); v != "" {
		c.App.Env = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		c.App.LogLevel = v
	}
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		c.Server.Http.Addr = v
	}
	if v := os.Getenv("GRPC_ADDR"); v != "" {
		c.Server.Grpc.Addr = v
	}
	if v := os.Getenv("DATABASE_DRIVER"); v != "" {
		c.Data.Database.Driver = v
	}
	if v := os.Getenv("DATABASE_SOURCE"); v != "" {
		c.Data.Database.Source = v
	}
	if v := os.Getenv("DATABASE_AUTO_MIGRATE"); v != "" {
		c.Data.Database.AutoMigrate = parseBool(v, c.Data.Database.AutoMigrate)
	}
	if v := os.Getenv("DATABASE_SEED"); v != "" {
		c.Data.Database.Seed = parseBool(v, c.Data.Database.Seed)
	}
	if v := os.Getenv("DATABASE_MAX_OPEN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Data.Database.MaxOpenConns = int32(n)
		}
	}
	if v := os.Getenv("DATABASE_MAX_IDLE_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Data.Database.MaxIdleConns = int32(n)
		}
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		c.Auth.JwtSecret = v
	}
	if v := os.Getenv("JWT_ISSUER"); v != "" {
		c.Auth.JwtIssuer = v
	}
	if v := os.Getenv("JWT_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Auth.AccessTokenTtl = durationpb.New(d)
		}
	}
	if v := os.Getenv("TRANSACTION_MODE"); v != "" {
		c.Data.Transaction.Mode = v
	}
	if v := os.Getenv("DTM_ADDRESS"); v != "" {
		c.Data.Dtm.Address = v
	}
	if v := os.Getenv("DTM_ENABLED"); v != "" {
		c.Data.Dtm.Enabled = parseBool(v, c.Data.Dtm.Enabled)
	}
	if v := os.Getenv("DTM_FALLBACK_TO_LOCAL"); v != "" {
		c.Data.Transaction.FallbackToLocal = parseBool(v, c.Data.Transaction.FallbackToLocal)
	}
	if v := os.Getenv("ETCD_ENDPOINTS"); v != "" {
		parts := strings.Split(v, ",")
		c.Registry.Etcd = &conf.Etcd{}
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				c.Registry.Etcd.Endpoints = append(c.Registry.Etcd.Endpoints, p)
			}
		}
	}
	if v := os.Getenv("PRODUCT_ENDPOINT"); v != "" {
		c.Services.ProductEndpoint = v
	}
	if v := os.Getenv("ORDER_ENDPOINT"); v != "" {
		c.Services.OrderEndpoint = v
	}
	if v := os.Getenv("ORDER_PAYMENT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Order.PaymentTimeout = durationpb.New(d)
		}
	}
	if v := os.Getenv("ORDER_SHIPPING_FEE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.Order.ShippingFee = f
		}
	}
}

func parseBool(v string, fallback bool) bool {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

// TransactionMode returns the normalized transaction mode.
func TransactionMode(c *conf.Bootstrap) string {
	if c == nil || c.Data == nil || c.Data.Transaction == nil {
		return TransactionModeLocal
	}
	mode := strings.ToLower(strings.TrimSpace(c.Data.Transaction.Mode))
	if mode == "" {
		return TransactionModeLocal
	}
	return mode
}

// DTMEnabled reports whether remote DTM should be used.
func DTMEnabled(c *conf.Bootstrap) bool {
	return TransactionMode(c) == TransactionModeDTM && c.Data.Dtm != nil && c.Data.Dtm.Enabled
}
