package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Service       string
	Env           string
	GRPCAddr      string
	HTTPAddr      string
	MySQLDSN      string
	RedisAddr     string
	RedisPassword string
	NATSURL       string
	ETCDEndpoints []string
	DTMAddress    string
	JWTSecret     string
	JWTIssuer     string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

func Load(service string) (*Config, error) {
	c := &Config{
		Service:       service,
		Env:           env("APP_ENV", "dev"),
		GRPCAddr:      env("GRPC_ADDR", ":9000"),
		HTTPAddr:      env("HTTP_ADDR", ":9100"),
		MySQLDSN:      env("MYSQL_DSN", "root:root@tcp(127.0.0.1:3306)/mall?parseTime=true&loc=Local&charset=utf8mb4"),
		RedisAddr:     env("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: env("REDIS_PASSWORD", ""),
		NATSURL:       env("NATS_URL", "nats://127.0.0.1:4222"),
		DTMAddress:    env("DTM_ADDRESS", "127.0.0.1:36790"),
		JWTSecret:     env("JWT_SECRET", "dev-only-change-me-32-characters"),
		JWTIssuer:     env("JWT_ISSUER", "mall"),
	}
	if v := env("ETCD_ENDPOINTS", "127.0.0.1:2379"); v != "" {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				c.ETCDEndpoints = append(c.ETCDEndpoints, p)
			}
		}
	}
	ttl, err := time.ParseDuration(env("JWT_ACCESS_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("JWT_ACCESS_TTL: %w", err)
	}
	refresh, err := time.ParseDuration(env("JWT_REFRESH_TTL", "720h"))
	if err != nil {
		return nil, fmt.Errorf("JWT_REFRESH_TTL: %w", err)
	}
	c.AccessTTL, c.RefreshTTL = ttl, refresh
	if len(c.JWTSecret) < 16 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 16 characters")
	}
	if c.GRPCAddr == "" || c.HTTPAddr == "" {
		return nil, fmt.Errorf("GRPC_ADDR and HTTP_ADDR are required")
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func EnvInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
