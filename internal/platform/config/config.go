package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Redis modes understood by the platform Redis client.
const (
	RedisModeStandalone = "standalone"
	RedisModeSentinel   = "sentinel"
	RedisModeCluster    = "cluster"
)

const (
	defaultMySQLMaxOpenConns = 30
	defaultMySQLMaxIdleConns = 10
	// DefaultDevJWTSecret is the shipped development secret. Production mode
	// must override it; see validateSecrets.
	DefaultDevJWTSecret = "dev-only-change-me-32-characters"
)

type Config struct {
	Service       string
	Env           string
	GRPCAddr      string
	HTTPAddr      string
	MySQLDSN      string
	RedisMode     string
	RedisAddr     string
	RedisAddrs    []string
	RedisPassword string
	// RedisMasterName is the sentinel master set name; empty in the other modes.
	RedisMasterName string
	// PaymentSecret verifies payment channel callbacks. Empty means mock mode.
	PaymentSecret string
	// PaymentMockEnabled allows the BFF mock-pay route and unsigned callbacks.
	PaymentMockEnabled bool
	// WarehouseAllocationShadow enables the checkout allocator dual-read. It
	// never changes the quote or order; phase 2b wires warehouse quote lines.
	WarehouseAllocationShadow bool
	// WarehouseSplitEnabled allows Order/Gateway to accept multi-group quotes.
	// It stays false until the split shipment/permit phases are ready.
	WarehouseSplitEnabled bool
	// WarehouseMode is the single off|shadow|split switch. It is derived from
	// WAREHOUSE_MODE or the legacy booleans and is used for diagnostics.
	WarehouseMode string
	// QuantitySplitEnabled allows one SKU to be split across at most two
	// warehouses (ADR 0005). It requires multi-group warehouse split.
	QuantitySplitEnabled bool
	// CarrierRegistryEnabled makes the controlled carrier list mandatory.
	CarrierRegistryEnabled bool
	// DeliveryAutoConfirmEnabled runs the T+1/T+7 receipt job worker.
	DeliveryAutoConfirmEnabled bool
	// ShortageCaseEnabled exposes the post-payment shortage choice flow.
	ShortageCaseEnabled bool
	// FreightResponsibilityEnabled gates responsibility determination and the
	// 168h freight hold. Off falls back to the legacy "unshipped full freight,
	// otherwise none" rule.
	FreightResponsibilityEnabled bool
	// InventoryWarehousePriority is the configured warehouse business priority
	// order (ADR 0005 §18.3). Empty means warehouse_id ascending.
	InventoryWarehousePriority []int64
	// SeckillTokenSecret signs admission tokens; must differ from JWTSecret.
	SeckillTokenSecret string
	NATSURL            string
	ETCDEndpoints      []string
	DTMAddress         string
	JWTSecret          string
	JWTIssuer          string
	AccessTTL          time.Duration
	RefreshTTL         time.Duration
	// MySQLMaxOpenConns/MySQLMaxIdleConns bound each service's connection pool.
	MySQLMaxOpenConns int
	MySQLMaxIdleConns int
	// AIEnabled gates the assistant. The transaction path never depends on it.
	AIEnabled bool
	// AIProvider is "mock" (deterministic, no network) or "deepseek".
	AIProvider string
	// DeepSeekAPIKey authenticates the DeepSeek API; required for that provider.
	DeepSeekAPIKey  string
	DeepSeekBaseURL string
	AIModel         string
	AIMaxTokens     int
	AITimeout       time.Duration
	// AIRateLimit requests per user per AIRateWindow.
	AIRateLimit  int
	AIRateWindow time.Duration
	// RAGEnabled indexes product/policy documents into Redis Search.
	RAGEnabled bool
	// EmbeddingProvider is "hash" (dev only) or "openai" (compatible API).
	EmbeddingProvider string
	EmbeddingBaseURL  string
	EmbeddingAPIKey   string
	EmbeddingModel    string
	EmbeddingDim      int
	// AIDailyTokenBudget limits one user's prompt+completion tokens per UTC day;
	// 0 disables the check. AIGlobalDailyTokenBudget does the same across users.
	AIDailyTokenBudget       int64
	AIGlobalDailyTokenBudget int64
	// AICircuitFailures consecutive provider failures open the circuit for
	// AICircuitCooldown.
	AICircuitFailures int
	AICircuitCooldown time.Duration
	// Optional list prices in cents per 1k tokens, for cost metrics only.
	AIPricePromptPer1K     int64
	AIPriceCompletionPer1K int64
}

// etcdFile is the optional YAML/JSON overlay stored in etcd. Pointers preserve
// "not set" so environment variables remain the higher-priority bootstrap.
type etcdFile struct {
	Service         *string `json:"service" yaml:"service"`
	Env             *string `json:"env" yaml:"env"`
	GRPCAddr        *string `json:"grpc_addr" yaml:"grpc_addr"`
	HTTPAddr        *string `json:"http_addr" yaml:"http_addr"`
	MySQLDSN        *string `json:"mysql_dsn" yaml:"mysql_dsn"`
	RedisMode       *string `json:"redis_mode" yaml:"redis_mode"`
	RedisAddr       *string `json:"redis_addr" yaml:"redis_addr"`
	RedisAddrs      *string `json:"redis_addrs" yaml:"redis_addrs"`
	RedisMasterName *string `json:"redis_master_name" yaml:"redis_master_name"`
	RedisPassword   *string `json:"redis_password" yaml:"redis_password"`
	NATSURL         *string `json:"nats_url" yaml:"nats_url"`
	DTMAddress      *string `json:"dtm_address" yaml:"dtm_address"`
	JWTIssuer       *string `json:"jwt_issuer" yaml:"jwt_issuer"`
	AccessTTL       *string `json:"jwt_access_ttl" yaml:"jwt_access_ttl"`
	RefreshTTL      *string `json:"jwt_refresh_ttl" yaml:"jwt_refresh_ttl"`
}

func Load(service string) (*Config, error) {
	c := &Config{
		Service:            service,
		Env:                "dev",
		GRPCAddr:           ":9000",
		HTTPAddr:           ":9100",
		MySQLDSN:           "root:root@tcp(127.0.0.1:3306)/mall?parseTime=true&loc=Local&charset=utf8mb4",
		RedisMode:          RedisModeStandalone,
		RedisAddr:          "127.0.0.1:6379",
		NATSURL:            "nats://127.0.0.1:4222",
		DTMAddress:         "127.0.0.1:36790",
		JWTSecret:          DefaultDevJWTSecret,
		PaymentMockEnabled: true,
		JWTIssuer:          "mall",
		AccessTTL:          15 * time.Minute,
		RefreshTTL:         720 * time.Hour,
		MySQLMaxOpenConns:  defaultMySQLMaxOpenConns,
		MySQLMaxIdleConns:  defaultMySQLMaxIdleConns,
		AIProvider:         "mock",
		DeepSeekBaseURL:    "https://api.deepseek.com",
		AIModel:            "deepseek-chat",
		AIMaxTokens:        1024,
		AITimeout:          30 * time.Second,
		AIRateLimit:        20,
		AIRateWindow:       10 * time.Minute,
		EmbeddingProvider:  "hash",
		EmbeddingBaseURL:   "https://api.openai.com/v1",
		EmbeddingModel:     "text-embedding-3-small",
		EmbeddingDim:       256,
		AICircuitFailures:  5,
		AICircuitCooldown:  60 * time.Second,
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
	if err := applyEnv(c); err != nil {
		return nil, err
	}
	if len(c.JWTSecret) < 16 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 16 characters")
	}
	if c.GRPCAddr == "" || c.HTTPAddr == "" {
		return nil, fmt.Errorf("GRPC_ADDR and HTTP_ADDR are required")
	}
	if err := c.normalizeRedis(); err != nil {
		return nil, err
	}
	c.normalizePool()
	if err := c.validateSecrets(); err != nil {
		return nil, err
	}
	if err := c.validateAI(); err != nil {
		return nil, err
	}
	return c, nil
}

// validateSecrets makes production mode fail closed. Development keeps the
// historical defaults so compose and unit tests start without configuration.
// validateAI keeps provider selection explicit and fails closed when the
// selected provider cannot be configured.
func (c *Config) validateAI() error {
	provider := strings.ToLower(strings.TrimSpace(c.AIProvider))
	if provider == "" {
		provider = "mock"
	}
	c.AIProvider = provider
	switch provider {
	case "mock", "deepseek":
	default:
		return fmt.Errorf("AI_PROVIDER must be mock or deepseek (got %q)", c.AIProvider)
	}
	if provider == "deepseek" && strings.TrimSpace(c.DeepSeekAPIKey) == "" && c.AIEnabled {
		return fmt.Errorf("DEEPSEEK_API_KEY is required when AI_PROVIDER=deepseek")
	}
	if c.AITimeout <= 0 {
		c.AITimeout = 30 * time.Second
	}
	if c.AIMaxTokens <= 0 {
		c.AIMaxTokens = 1024
	}
	if c.AIRateLimit <= 0 {
		c.AIRateLimit = 20
	}
	if c.AIRateWindow <= 0 {
		c.AIRateWindow = 10 * time.Minute
	}
	embeddingProvider := strings.ToLower(strings.TrimSpace(c.EmbeddingProvider))
	if embeddingProvider == "" {
		embeddingProvider = "hash"
	}
	c.EmbeddingProvider = embeddingProvider
	switch embeddingProvider {
	case "hash":
		if c.EmbeddingDim <= 0 {
			c.EmbeddingDim = 256
		}
	case "openai":
		if strings.TrimSpace(c.EmbeddingModel) == "" {
			return fmt.Errorf("EMBEDDING_MODEL is required for EMBEDDING_PROVIDER=openai")
		}
		if c.EmbeddingDim <= 0 {
			c.EmbeddingDim = 1536
		}
	default:
		return fmt.Errorf("EMBEDDING_PROVIDER must be hash or openai (got %q)", c.EmbeddingProvider)
	}
	if c.AICircuitFailures <= 0 {
		c.AICircuitFailures = 5
	}
	if c.AICircuitCooldown <= 0 {
		c.AICircuitCooldown = 60 * time.Second
	}
	if c.AIDailyTokenBudget < 0 || c.AIGlobalDailyTokenBudget < 0 {
		return fmt.Errorf("AI token budgets must be >= 0")
	}
	return nil
}

func (c *Config) validateSecrets() error {
	// A payment service that accepts real callbacks must never start without a
	// signature secret, regardless of APP_ENV. Mock mode is the explicit
	// development opt-in.
	if c.Service == "payment" && !c.PaymentMockEnabled && strings.TrimSpace(c.PaymentSecret) == "" {
		return fmt.Errorf("PAYMENT_SECRET is required when mock payments are disabled")
	}
	if c.Env == "dev" {
		return nil
	}
	if c.PaymentMockEnabled {
		return fmt.Errorf("PAYMENT_MOCK_ENABLED must be false when APP_ENV=%s", c.Env)
	}
	if c.JWTSecret == DefaultDevJWTSecret || len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be overridden (>=32 chars) when APP_ENV=%s", c.Env)
	}
	if c.Service == "seckill" {
		if strings.TrimSpace(c.SeckillTokenSecret) == "" {
			return fmt.Errorf("SECKILL_TOKEN_SECRET is required when APP_ENV=%s", c.Env)
		}
		if c.SeckillTokenSecret == c.JWTSecret {
			return fmt.Errorf("SECKILL_TOKEN_SECRET must differ from JWT_SECRET")
		}
	}
	return nil
}

// normalizeRedis resolves the endpoint list for the selected mode and rejects
// incomplete sentinel/cluster configurations instead of silently degrading to a
// single node. Standalone keeps the historical REDIS_ADDR behavior.
func (c *Config) normalizeRedis() error {
	mode := strings.ToLower(strings.TrimSpace(c.RedisMode))
	if mode == "" {
		mode = RedisModeStandalone
	}
	switch mode {
	case RedisModeStandalone, RedisModeSentinel, RedisModeCluster:
	default:
		return fmt.Errorf("REDIS_MODE must be %s, %s or %s (got %q)",
			RedisModeStandalone, RedisModeSentinel, RedisModeCluster, c.RedisMode)
	}
	c.RedisMode = mode
	if mode == RedisModeStandalone {
		if len(c.RedisAddrs) > 1 {
			return fmt.Errorf("REDIS_ADDRS must contain exactly one address in standalone mode (got %d)", len(c.RedisAddrs))
		}
		if len(c.RedisAddrs) == 0 {
			if strings.TrimSpace(c.RedisAddr) == "" {
				return fmt.Errorf("REDIS_ADDR is required in standalone mode")
			}
			c.RedisAddrs = []string{strings.TrimSpace(c.RedisAddr)}
		}
		c.RedisAddr = c.RedisAddrs[0]
		c.RedisMasterName = ""
		return nil
	}
	if len(c.RedisAddrs) == 0 {
		if strings.TrimSpace(c.RedisAddr) == "" {
			return fmt.Errorf("REDIS_ADDRS is required in %s mode", mode)
		}
		c.RedisAddrs = []string{strings.TrimSpace(c.RedisAddr)}
	}
	if mode == RedisModeSentinel {
		c.RedisMasterName = strings.TrimSpace(c.RedisMasterName)
		if c.RedisMasterName == "" {
			return fmt.Errorf("REDIS_MASTER_NAME is required in sentinel mode")
		}
	} else {
		c.RedisMasterName = ""
	}
	return nil
}

// normalizePool clamps pool values so a bad override cannot exhaust MySQL or
// configure more idle connections than open ones.
func (c *Config) normalizePool() {
	if c.MySQLMaxOpenConns <= 0 {
		c.MySQLMaxOpenConns = defaultMySQLMaxOpenConns
	}
	if c.MySQLMaxIdleConns < 0 {
		c.MySQLMaxIdleConns = 0
	}
	if c.MySQLMaxIdleConns > c.MySQLMaxOpenConns {
		c.MySQLMaxIdleConns = c.MySQLMaxOpenConns
	}
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
	set(&c.RedisMode, f.RedisMode)
	set(&c.RedisAddr, f.RedisAddr)
	if f.RedisAddrs != nil {
		c.RedisAddrs = splitCSV(*f.RedisAddrs)
	}
	set(&c.RedisMasterName, f.RedisMasterName)
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
func applyEnv(c *Config) error {
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
	if v := os.Getenv("REDIS_MODE"); v != "" {
		c.RedisMode = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		c.RedisAddr = v
	}
	if v := os.Getenv("REDIS_ADDRS"); v != "" {
		c.RedisAddrs = splitCSV(v)
	}
	if v := os.Getenv("REDIS_MASTER_NAME"); v != "" {
		c.RedisMasterName = v
	}
	if v, ok := os.LookupEnv("REDIS_PASSWORD"); ok {
		c.RedisPassword = v
	}
	if v := os.Getenv("MYSQL_MAX_OPEN_CONNS"); v != "" {
		c.MySQLMaxOpenConns = EnvInt("MYSQL_MAX_OPEN_CONNS", c.MySQLMaxOpenConns)
	}
	if v := os.Getenv("MYSQL_MAX_IDLE_CONNS"); v != "" {
		c.MySQLMaxIdleConns = EnvInt("MYSQL_MAX_IDLE_CONNS", c.MySQLMaxIdleConns)
	}
	if v := os.Getenv("PAYMENT_SECRET"); v != "" {
		c.PaymentSecret = v
	}
	if v, ok := os.LookupEnv("PAYMENT_MOCK_ENABLED"); ok && strings.TrimSpace(v) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("PAYMENT_MOCK_ENABLED must be a boolean (got %q)", v)
		}
		c.PaymentMockEnabled = parsed
	} else if c.Env != "dev" {
		// Mock payments are a development capability; a non-dev environment
		// must never default to them.
		c.PaymentMockEnabled = false
	}
	if v := os.Getenv("SECKILL_TOKEN_SECRET"); v != "" {
		c.SeckillTokenSecret = v
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
	splitSet := false
	if v, ok := os.LookupEnv("WAREHOUSE_SPLIT_ENABLED"); ok && strings.TrimSpace(v) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("WAREHOUSE_SPLIT_ENABLED must be a boolean (got %q)", v)
		}
		c.WarehouseSplitEnabled = parsed
		splitSet = true
	}
	shadowSet := false
	if v, ok := os.LookupEnv("WAREHOUSE_ALLOCATION_SHADOW"); ok && strings.TrimSpace(v) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("WAREHOUSE_ALLOCATION_SHADOW must be a boolean (got %q)", v)
		}
		c.WarehouseAllocationShadow = parsed
		shadowSet = true
	}
	if v := strings.TrimSpace(os.Getenv("WAREHOUSE_MODE")); v != "" {
		var shadow, split bool
		switch v {
		case "off":
		case "shadow":
			shadow = true
		case "split":
			shadow, split = true, true
		default:
			return fmt.Errorf("WAREHOUSE_MODE must be off, shadow or split (got %q)", v)
		}
		if splitSet && c.WarehouseSplitEnabled != split {
			return fmt.Errorf("WAREHOUSE_MODE=%s conflicts with WAREHOUSE_SPLIT_ENABLED", v)
		}
		if shadowSet && c.WarehouseAllocationShadow != shadow {
			return fmt.Errorf("WAREHOUSE_MODE=%s conflicts with WAREHOUSE_ALLOCATION_SHADOW", v)
		}
		c.WarehouseAllocationShadow, c.WarehouseSplitEnabled = shadow, split
		c.WarehouseMode = v
	} else {
		switch {
		case c.WarehouseSplitEnabled:
			c.WarehouseMode = "split"
		case c.WarehouseAllocationShadow:
			c.WarehouseMode = "shadow"
		default:
			c.WarehouseMode = "off"
		}
	}
	if v, ok := os.LookupEnv("QUANTITY_SPLIT_ENABLED"); ok && strings.TrimSpace(v) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("QUANTITY_SPLIT_ENABLED must be a boolean (got %q)", v)
		}
		c.QuantitySplitEnabled = parsed
	}
	if c.QuantitySplitEnabled && !c.WarehouseSplitEnabled {
		return fmt.Errorf("QUANTITY_SPLIT_ENABLED requires WAREHOUSE_MODE=split")
	}
	if raw := strings.TrimSpace(os.Getenv("INVENTORY_WAREHOUSE_PRIORITY")); raw != "" {
		seen := make(map[int64]struct{})
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				return fmt.Errorf("INVENTORY_WAREHOUSE_PRIORITY contains an empty entry")
			}
			warehouseID, err := strconv.ParseInt(part, 10, 64)
			if err != nil || warehouseID <= 0 {
				return fmt.Errorf("INVENTORY_WAREHOUSE_PRIORITY must contain positive integers (got %q)", part)
			}
			if _, dup := seen[warehouseID]; dup {
				return fmt.Errorf("INVENTORY_WAREHOUSE_PRIORITY contains duplicate warehouse %d", warehouseID)
			}
			seen[warehouseID] = struct{}{}
			c.InventoryWarehousePriority = append(c.InventoryWarehousePriority, warehouseID)
		}
	}
	if v, ok := os.LookupEnv("CARRIER_REGISTRY_ENABLED"); ok && strings.TrimSpace(v) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("CARRIER_REGISTRY_ENABLED must be a boolean (got %q)", v)
		}
		c.CarrierRegistryEnabled = parsed
	}
	if v, ok := os.LookupEnv("DELIVERY_AUTO_CONFIRM_ENABLED"); ok && strings.TrimSpace(v) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("DELIVERY_AUTO_CONFIRM_ENABLED must be a boolean (got %q)", v)
		}
		c.DeliveryAutoConfirmEnabled = parsed
	}
	if v, ok := os.LookupEnv("FREIGHT_RESPONSIBILITY_ENABLED"); ok && strings.TrimSpace(v) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("FREIGHT_RESPONSIBILITY_ENABLED must be a boolean (got %q)", v)
		}
		c.FreightResponsibilityEnabled = parsed
	}
	if v, ok := os.LookupEnv("SHORTAGE_CASE_ENABLED"); ok && strings.TrimSpace(v) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("SHORTAGE_CASE_ENABLED must be a boolean (got %q)", v)
		}
		c.ShortageCaseEnabled = parsed
	}
	if v, ok := os.LookupEnv("AI_ENABLED"); ok && strings.TrimSpace(v) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("AI_ENABLED must be a boolean (got %q)", v)
		}
		c.AIEnabled = parsed
	}
	if v := os.Getenv("AI_PROVIDER"); v != "" {
		c.AIProvider = v
	}
	if v := os.Getenv("DEEPSEEK_API_KEY"); v != "" {
		c.DeepSeekAPIKey = v
	}
	if v := os.Getenv("DEEPSEEK_BASE_URL"); v != "" {
		c.DeepSeekBaseURL = v
	}
	if v := os.Getenv("AI_MODEL"); v != "" {
		c.AIModel = v
	}
	if v := os.Getenv("AI_MAX_TOKENS"); v != "" {
		c.AIMaxTokens = EnvInt("AI_MAX_TOKENS", c.AIMaxTokens)
	}
	if v := os.Getenv("AI_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.AITimeout = d
		}
	}
	if v := os.Getenv("AI_RATE_LIMIT"); v != "" {
		c.AIRateLimit = EnvInt("AI_RATE_LIMIT", c.AIRateLimit)
	}
	if v := os.Getenv("AI_RATE_WINDOW"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.AIRateWindow = d
		}
	}
	if v, ok := os.LookupEnv("AI_RAG_ENABLED"); ok && strings.TrimSpace(v) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("AI_RAG_ENABLED must be a boolean (got %q)", v)
		}
		c.RAGEnabled = parsed
	}
	if v := os.Getenv("EMBEDDING_PROVIDER"); v != "" {
		c.EmbeddingProvider = v
	}
	if v := os.Getenv("EMBEDDING_BASE_URL"); v != "" {
		c.EmbeddingBaseURL = v
	}
	if v := os.Getenv("EMBEDDING_API_KEY"); v != "" {
		c.EmbeddingAPIKey = v
	}
	if v := os.Getenv("EMBEDDING_MODEL"); v != "" {
		c.EmbeddingModel = v
	}
	if v := os.Getenv("EMBEDDING_DIM"); v != "" {
		c.EmbeddingDim = EnvInt("EMBEDDING_DIM", c.EmbeddingDim)
	}
	if v := os.Getenv("AI_DAILY_TOKEN_BUDGET"); v != "" {
		c.AIDailyTokenBudget = int64(EnvInt("AI_DAILY_TOKEN_BUDGET", int(c.AIDailyTokenBudget)))
	}
	if v := os.Getenv("AI_GLOBAL_DAILY_TOKEN_BUDGET"); v != "" {
		c.AIGlobalDailyTokenBudget = int64(EnvInt("AI_GLOBAL_DAILY_TOKEN_BUDGET", int(c.AIGlobalDailyTokenBudget)))
	}
	if v := os.Getenv("AI_CIRCUIT_FAILURES"); v != "" {
		c.AICircuitFailures = EnvInt("AI_CIRCUIT_FAILURES", c.AICircuitFailures)
	}
	if v := os.Getenv("AI_CIRCUIT_COOLDOWN"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.AICircuitCooldown = d
		}
	}
	if v := os.Getenv("AI_PRICE_PROMPT_CENTS_PER_1K"); v != "" {
		c.AIPricePromptPer1K = int64(EnvInt("AI_PRICE_PROMPT_CENTS_PER_1K", int(c.AIPricePromptPer1K)))
	}
	if v := os.Getenv("AI_PRICE_COMPLETION_CENTS_PER_1K"); v != "" {
		c.AIPriceCompletionPer1K = int64(EnvInt("AI_PRICE_COMPLETION_CENTS_PER_1K", int(c.AIPriceCompletionPer1K)))
	}
	return nil
}

// splitCSV turns a comma-separated endpoint list into trimmed entries, dropping
// blanks so trailing commas do not create unusable addresses.
func splitCSV(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
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
