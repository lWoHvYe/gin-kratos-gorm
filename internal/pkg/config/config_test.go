package config

import (
	"strings"
	"testing"
	"time"

	"gin-kratos-gorm/internal/conf"

	"google.golang.org/protobuf/types/known/durationpb"
)

func TestNewConfigLocalFile(t *testing.T) {
	cfg, err := NewConfig("../../../configs/config.local.yaml")
	if err != nil {
		t.Fatalf("load local config: %v", err)
	}
	if cfg.Data.Database.Driver != "sqlite" {
		t.Fatalf("expected sqlite driver, got %q", cfg.Data.Database.Driver)
	}
	if TransactionMode(cfg) != TransactionModeLocal {
		t.Fatalf("expected local transaction mode, got %q", TransactionMode(cfg))
	}
	if DTMEnabled(cfg) {
		t.Fatal("DTM must be disabled in the local config")
	}
	if cfg.Auth.JwtSecret == "" || cfg.Auth.AccessTokenTtl.AsDuration() <= 0 {
		t.Fatal("expected auth defaults")
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("JWT_SECRET", "env-secret-at-least-16-chars")
	t.Setenv("TRANSACTION_MODE", "dtm")
	t.Setenv("DTM_ENABLED", "true")
	t.Setenv("DTM_ADDRESS", "127.0.0.1:36789")
	t.Setenv("DATABASE_DRIVER", "sqlite")
	t.Setenv("DATABASE_SOURCE", ":memory:")
	cfg, err := NewConfig("../../../configs/config.local.yaml")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Auth.JwtSecret != "env-secret-at-least-16-chars" {
		t.Fatalf("env secret not applied: %q", cfg.Auth.JwtSecret)
	}
	if TransactionMode(cfg) != TransactionModeDTM || !DTMEnabled(cfg) {
		t.Fatalf("env transaction mode not applied")
	}
	if cfg.Data.Dtm.Address != "127.0.0.1:36789" {
		t.Fatalf("env DTM address not applied: %q", cfg.Data.Dtm.Address)
	}
}

func TestValidate(t *testing.T) {
	base := func() *conf.Bootstrap {
		return &conf.Bootstrap{
			Server: &conf.Server{
				Http: &conf.Server_HTTP{Addr: ":8000"},
				Grpc: &conf.Server_GRPC{Addr: ":9000"},
			},
			Data: &conf.Data{Database: &conf.Data_Database{Driver: "sqlite", Source: ":memory:"}},
			Auth: &conf.Auth{JwtSecret: "0123456789abcdef", AccessTokenTtl: durationpb.New(time.Hour)},
		}
	}
	if err := Validate(base()); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*conf.Bootstrap)
		want   string
	}{
		{"no server", func(c *conf.Bootstrap) { c.Server.Http = nil }, "server.http"},
		{"bad driver", func(c *conf.Bootstrap) { c.Data.Database.Driver = "oracle" }, "unsupported"},
		{"no source", func(c *conf.Bootstrap) { c.Data.Database.Source = "" }, "source"},
		{"short secret", func(c *conf.Bootstrap) { c.Auth.JwtSecret = "short" }, "at least"},
		{"no ttl", func(c *conf.Bootstrap) { c.Auth.AccessTokenTtl = nil }, "access_token_ttl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base()
			tc.mutate(c)
			err := Validate(c)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}
