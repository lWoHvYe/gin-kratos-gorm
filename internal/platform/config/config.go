package config

import (
	"context"
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

// etcdFile is the optional YAML/JSON overlay stored in etcd. Pointers preserve
// "not set" so environment variables remain the higher-priority bootstrap.
type etcdFile struct {
	Service       *string `json:"service" yaml:"service"`
	Env           *string `json:"env" yaml:"env"`
	GRPCAddr      *string `json:"grpc_addr" yaml:"grpc_addr"`
	HTTPAddr      *string `json:"http_addr" yaml:"http_addr"`
	MySQLDSN      *string `json:"mysql_dsn" yaml:"mysql_dsn"`
	RedisAddr     *string `json:"redis_addr" yaml:"redis_addr"`
	RedisPassword *string `json:"redis_password" yaml:"redis_password"`
	NATSURL       *string `json:"nats_url" yaml:"nats_url"`
	DTMAddress    *string `json:"dtm_address" yaml:"dtm_address"`
	JWTIssuer     *string `json:"jwt_issuer" yaml:"jwt_issuer"`
	AccessTTL     *string `json:"jwt_access_ttl" yaml:"jwt_access_ttl"`
	RefreshTTL    *string `json:"jwt_refresh_ttl" yaml:"jwt_refresh_ttl"`
}

func Load(service string) (*Config, error) {
	c := &Config{
		Service:    service,
		Env:        "dev",
		GRPCAddr:   ":9000",
		HTTPAddr:   ":9100",
		MySQLDSN:   "root:root@tcp(127.0.0.1:3306)/mall?parseTime=true&loc=Local&charset=utf8mb4",
		RedisAddr:  "127.0.0.1:6379",
		NATSURL:    "nats://127.0.0.1:4222",
		DTMAddress: "127.0.0.1:36790",
		JWTSecret:  "dev-only-change-me-32-characters",
		JWTIssuer:  "mall",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 720 * time.Hour,
	}
	// Bootstrap endpoints are needed before the config center can be read.
	if v := strings.TrimSpace(os.Getenv("ETCD_ENDPOINTS")); v != "" {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				c.ETCDEndpoints = append(c.ETCDEndpoints, p)
			}
		}
	}
	if path := strings.TrimSpace(os.Getenv("CONFIG_ETCD_PATH")); path != "" {
		if len(c.ETCDEndpoints) == 0 {
			return nil, fmt.Errorf("ETCD_ENDPOINTS is required when CONFIG_ETCD_PATH is set")
		}
		provider, err := LoadEtcd(context.Background(), c.ETCDEndpoints, path, envBool("CONFIG_ETCD_PREFIX", false))
		if err != nil {
			return nil, err
		}
		defer provider.Close()
		var file etcdFile
		if err := provider.Scan(&file); err != nil {
			return nil, fmt.Errorf("scan etcd config: %w", err)
		}
		if err := applyEtcd(c, &file); err != nil {
			return nil, err
		}
	}
	applyEnv(c)
	if len(c.JWTSecret) < 16 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 16 characters")
	}
	if c.GRPCAddr == "" || c.HTTPAddr == "" {
		return nil, fmt.Errorf("GRPC_ADDR and HTTP_ADDR are required")
	}
	return c, nil
}

func applyEtcd(c *Config, f *etcdFile) error {
	set := func(dst *string, src *string) {
		if src != nil {
			*dst = *src
		}
	}
	set(&c.Service, f.Service)
	set(&c.Env, f.Env)
	set(&c.GRPCAddr, f.GRPCAddr)
	set(&c.HTTPAddr, f.HTTPAddr)
	set(&c.MySQLDSN, f.MySQLDSN)
	set(&c.RedisAddr, f.RedisAddr)
	set(&c.RedisPassword, f.RedisPassword)
	set(&c.NATSURL, f.NATSURL)
	set(&c.DTMAddress, f.DTMAddress)
	set(&c.JWTIssuer, f.JWTIssuer)
	if f.AccessTTL != nil {
		d, err := time.ParseDuration(*f.AccessTTL)
		if err != nil {
			return fmt.Errorf("etcd jwt_access_ttl: %w", err)
		}
		c.AccessTTL = d
	}
	if f.RefreshTTL != nil {
		d, err := time.ParseDuration(*f.RefreshTTL)
		if err != nil {
			return fmt.Errorf("etcd jwt_refresh_ttl: %w", err)
		}
		c.RefreshTTL = d
	}
	return nil
}
func applyEnv(c *Config) {
	if v := os.Getenv("APP_ENV"); v != "" {
		c.Env = v
	}
	if v := os.Getenv("GRPC_ADDR"); v != "" {
		c.GRPCAddr = v
	}
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		c.HTTPAddr = v
	}
	if v := os.Getenv("MYSQL_DSN"); v != "" {
		c.MySQLDSN = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		c.RedisAddr = v
	}
	if v, ok := os.LookupEnv("REDIS_PASSWORD"); ok {
		c.RedisPassword = v
	}
	if v := os.Getenv("NATS_URL"); v != "" {
		c.NATSURL = v
	}
	if v := os.Getenv("DTM_ADDRESS"); v != "" {
		c.DTMAddress = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		c.JWTSecret = v
	}
	if v := os.Getenv("JWT_ISSUER"); v != "" {
		c.JWTIssuer = v
	}
	if v := os.Getenv("JWT_ACCESS_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.AccessTTL = d
		}
	}
	if v := os.Getenv("JWT_REFRESH_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.RefreshTTL = d
		}
	}
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func EnvInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
