package redis

import (
	"strings"
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

func TestNewClientStandalone(t *testing.T) {
	c, err := newClient(Config{Addrs: []string{" 127.0.0.1:6379 "}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, ok := c.(*goredis.Client); !ok {
		t.Fatalf("client type = %T, want *redis.Client", c)
	}
}

func TestNewClientStandaloneRequiresOneAddr(t *testing.T) {
	if _, err := newClient(Config{}); err == nil {
		t.Fatal("expected an error for a missing address")
	}
	if _, err := newClient(Config{Addrs: []string{"a:6379", "b:6379"}}); err == nil {
		t.Fatal("expected an error for two standalone addresses")
	}
}

func TestNewClientSentinelRequiresMasterName(t *testing.T) {
	_, err := newClient(Config{Mode: "sentinel", Addrs: []string{"s:26379"}})
	if err == nil || !strings.Contains(err.Error(), "master name") {
		t.Fatalf("err = %v", err)
	}
	c, err := newClient(Config{Mode: "sentinel", Addrs: []string{"s0:26379", "s1:26379"}, MasterName: "mallmaster"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	// go-redis implements failover on top of *Client.
	if _, ok := c.(*goredis.Client); !ok {
		t.Fatalf("client type = %T, want *redis.Client", c)
	}
}

func TestNewClientCluster(t *testing.T) {
	c, err := newClient(Config{Mode: "cluster", Addrs: []string{"a:6379", "b:6379", "c:6379"}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, ok := c.(*goredis.ClusterClient); !ok {
		t.Fatalf("client type = %T, want *redis.ClusterClient", c)
	}
}

func TestNewClientRejectsUnknownMode(t *testing.T) {
	_, err := newClient(Config{Mode: "replica", Addrs: []string{"a:6379"}})
	if err == nil || !strings.Contains(err.Error(), "unknown mode") {
		t.Fatalf("err = %v", err)
	}
}
