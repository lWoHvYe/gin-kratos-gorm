package config

import (
	"context"
	"os"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// TestLoadEtcdOverlay is opt-in because it needs a running etcd. Run with:
//
//	TEST_ETCD_ENDPOINTS=127.0.0.1:2379 go test ./internal/platform/config
func TestLoadEtcdOverlay(t *testing.T) {
	endpoint := os.Getenv("TEST_ETCD_ENDPOINTS")
	if endpoint == "" {
		t.Skip("set TEST_ETCD_ENDPOINTS to run the etcd overlay test")
	}
	client, err := clientv3.New(clientv3.Config{Endpoints: []string{endpoint}, DialTimeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	key := "/mall/config/test/config.yaml"
	value := []byte("grpc_addr: \":19000\"\nhttp_addr: \":19100\"\njwt_issuer: \"etcd-test\"\njwt_access_ttl: \"5m\"\n")
	if _, err := client.Put(context.Background(), key, string(value)); err != nil {
		t.Fatal(err)
	}
	defer client.Delete(context.Background(), key)

	t.Setenv("ETCD_ENDPOINTS", endpoint)
	t.Setenv("CONFIG_ETCD_PATH", key)
	t.Setenv("JWT_SECRET", "0123456789abcdef")
	t.Setenv("GRPC_ADDR", "") // empty env must not mask the etcd overlay
	c, err := Load("configtest")
	if err != nil {
		t.Fatal(err)
	}
	if c.GRPCAddr != ":19000" || c.HTTPAddr != ":19100" || c.JWTIssuer != "etcd-test" {
		t.Fatalf("etcd overlay not applied: %+v", c)
	}
	if c.AccessTTL != 5*time.Minute {
		t.Fatalf("access ttl = %s", c.AccessTTL)
	}
}
