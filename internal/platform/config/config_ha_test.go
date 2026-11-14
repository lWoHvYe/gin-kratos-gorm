package config

import (
	"strings"
	"testing"
)

// envKeys lists every variable Load reads, so a test starts from a known state
// regardless of the developer's shell.
var envKeys = []string{
	"APP_ENV", "GRPC_ADDR", "HTTP_ADDR", "MYSQL_DSN",
	"REDIS_MODE", "REDIS_ADDR", "REDIS_ADDRS", "REDIS_MASTER_NAME", "REDIS_PASSWORD",
	"MYSQL_MAX_OPEN_CONNS", "MYSQL_MAX_IDLE_CONNS",
	"PAYMENT_SECRET", "PAYMENT_MOCK_ENABLED", "SECKILL_TOKEN_SECRET",
	"NATS_URL", "DTM_ADDRESS", "JWT_SECRET", "JWT_ISSUER",
	"JWT_ACCESS_TTL", "JWT_REFRESH_TTL",
	"ETCD_ENDPOINTS", "CONFIG_ETCD_PATH",
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range envKeys {
		t.Setenv(k, "")
	}
}

func TestLoadRedisStandaloneDefaults(t *testing.T) {
	clearConfigEnv(t)
	c, err := Load("test")
	if err != nil {
		t.Fatal(err)
	}
	if c.RedisMode != RedisModeStandalone {
		t.Fatalf("mode = %q", c.RedisMode)
	}
	if len(c.RedisAddrs) != 1 || c.RedisAddrs[0] != "127.0.0.1:6379" {
		t.Fatalf("addrs = %v", c.RedisAddrs)
	}
	if c.RedisMasterName != "" {
		t.Fatalf("master name = %q", c.RedisMasterName)
	}
	if c.MySQLMaxOpenConns != defaultMySQLMaxOpenConns || c.MySQLMaxIdleConns != defaultMySQLMaxIdleConns {
		t.Fatalf("pool = %d/%d", c.MySQLMaxOpenConns, c.MySQLMaxIdleConns)
	}
}

func TestLoadRedisSentinel(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("REDIS_MODE", "sentinel")
	t.Setenv("REDIS_ADDRS", " redis-sentinel-0:26379 , redis-sentinel-1:26379 ,")
	t.Setenv("REDIS_MASTER_NAME", " mallmaster ")
	t.Setenv("REDIS_PASSWORD", "secret")
	c, err := Load("test")
	if err != nil {
		t.Fatal(err)
	}
	if c.RedisMode != RedisModeSentinel {
		t.Fatalf("mode = %q", c.RedisMode)
	}
	if len(c.RedisAddrs) != 2 || c.RedisAddrs[0] != "redis-sentinel-0:26379" || c.RedisAddrs[1] != "redis-sentinel-1:26379" {
		t.Fatalf("addrs = %v", c.RedisAddrs)
	}
	if c.RedisMasterName != "mallmaster" {
		t.Fatalf("master name = %q", c.RedisMasterName)
	}
	if c.RedisPassword != "secret" {
		t.Fatalf("password not applied")
	}
}

func TestLoadRedisSentinelRequiresMasterName(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("REDIS_MODE", "sentinel")
	t.Setenv("REDIS_ADDRS", "127.0.0.1:26379")
	_, err := Load("test")
	if err == nil || !strings.Contains(err.Error(), "REDIS_MASTER_NAME") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadRedisCluster(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("REDIS_MODE", "cluster")
	t.Setenv("REDIS_ADDRS", "redis-0:6379,redis-1:6379,redis-2:6379")
	t.Setenv("REDIS_MASTER_NAME", "ignored")
	c, err := Load("test")
	if err != nil {
		t.Fatal(err)
	}
	if c.RedisMode != RedisModeCluster || len(c.RedisAddrs) != 3 {
		t.Fatalf("mode/addrs = %q %v", c.RedisMode, c.RedisAddrs)
	}
	if c.RedisMasterName != "" {
		t.Fatalf("cluster mode should clear master name, got %q", c.RedisMasterName)
	}
}

func TestLoadRedisRejectsBadMode(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("REDIS_MODE", "replica")
	if _, err := Load("test"); err == nil || !strings.Contains(err.Error(), "REDIS_MODE") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadRedisStandaloneRejectsMultipleAddrs(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("REDIS_ADDRS", "a:6379,b:6379")
	if _, err := Load("test"); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadMySQLPoolLimits(t *testing.T) {
	cases := []struct {
		name               string
		open, idle         string
		wantOpen, wantIdle int
	}{
		{"explicit", "80", "20", 80, 20},
		{"idle-clamped", "80", "200", 80, 80},
		{"negative-open", "-5", "5", defaultMySQLMaxOpenConns, 5},
		{"garbage", "many", "few", defaultMySQLMaxOpenConns, defaultMySQLMaxIdleConns},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv("MYSQL_MAX_OPEN_CONNS", tc.open)
			t.Setenv("MYSQL_MAX_IDLE_CONNS", tc.idle)
			c, err := Load("test")
			if err != nil {
				t.Fatal(err)
			}
			if c.MySQLMaxOpenConns != tc.wantOpen || c.MySQLMaxIdleConns != tc.wantIdle {
				t.Fatalf("pool = %d/%d, want %d/%d", c.MySQLMaxOpenConns, c.MySQLMaxIdleConns, tc.wantOpen, tc.wantIdle)
			}
		})
	}
}

func TestEtcdOverlayRedisAddrs(t *testing.T) {
	mode, addrs := "cluster", "a:6379, b:6379"
	c := &Config{RedisMode: RedisModeStandalone, RedisAddr: "127.0.0.1:6379"}
	if err := applyEtcd(c, &etcdFile{RedisMode: &mode, RedisAddrs: &addrs}); err != nil {
		t.Fatal(err)
	}
	if err := c.normalizeRedis(); err != nil {
		t.Fatal(err)
	}
	if c.RedisMode != RedisModeCluster || len(c.RedisAddrs) != 2 {
		t.Fatalf("mode/addrs = %q %v", c.RedisMode, c.RedisAddrs)
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" a , ,b,")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("splitCSV = %v", got)
	}
}

func TestProductionRequiresRealJWTSecret(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "production")
	_, err := Load("order")
	if err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("err = %v", err)
	}
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	if _, err := Load("order"); err != nil {
		t.Fatalf("valid secret rejected: %v", err)
	}
}

func TestProductionSeckillNeedsOwnTokenSecret(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	if _, err := Load("seckill"); err == nil || !strings.Contains(err.Error(), "SECKILL_TOKEN_SECRET") {
		t.Fatalf("err = %v", err)
	}
	t.Setenv("SECKILL_TOKEN_SECRET", "0123456789abcdef0123456789abcdef")
	if _, err := Load("seckill"); err == nil || !strings.Contains(err.Error(), "must differ") {
		t.Fatalf("err = %v", err)
	}
	t.Setenv("SECKILL_TOKEN_SECRET", "fedcba9876543210fedcba9876543210")
	if _, err := Load("seckill"); err != nil {
		t.Fatalf("valid seckill secret rejected: %v", err)
	}
}

func TestProductionPaymentRequiresSecretUnlessMock(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	if _, err := Load("payment"); err == nil || !strings.Contains(err.Error(), "PAYMENT_SECRET") {
		t.Fatalf("err = %v", err)
	}
	t.Setenv("PAYMENT_SECRET", "callback-secret")
	if _, err := Load("payment"); err != nil {
		t.Fatalf("valid payment secret rejected: %v", err)
	}
	// Explicit mock mode in production is allowed but must not be the default.
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("PAYMENT_MOCK_ENABLED", "true")
	if c, err := Load("payment"); err != nil || !c.PaymentMockEnabled {
		t.Fatalf("mock opt-in failed: %v %+v", err, c)
	}
}

func TestDevKeepsMockPaymentsAndSharedSeckillSecret(t *testing.T) {
	clearConfigEnv(t)
	c, err := Load("seckill")
	if err != nil {
		t.Fatal(err)
	}
	if !c.PaymentMockEnabled {
		t.Fatal("dev should default to mock payments")
	}
	if c.SeckillTokenSecret != "" {
		t.Fatal("dev may leave SECKILL_TOKEN_SECRET empty (falls back to JWT)")
	}
}

func TestProductionDefaultsMockPaymentsOff(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("PAYMENT_SECRET", "callback-secret")
	c, err := Load("gateway")
	if err != nil {
		t.Fatal(err)
	}
	if c.PaymentMockEnabled {
		t.Fatal("production must default mock payments off")
	}
}
