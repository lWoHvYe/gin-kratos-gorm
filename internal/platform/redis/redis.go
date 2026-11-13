package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Mode selects how the client discovers the server. The mode is deliberately
// explicit: a sentinel or cluster deployment must not silently degrade to a
// single node.
type Mode string

const (
	ModeStandalone Mode = "standalone"
	ModeSentinel   Mode = "sentinel"
	ModeCluster    Mode = "cluster"
)

// Config describes a Redis endpoint.
//
//	standalone: Addrs holds one address
//	sentinel:   Addrs holds the sentinel addresses, MasterName the master set
//	cluster:    Addrs holds the seed nodes
type Config struct {
	Mode       string
	Addrs      []string
	Password   string
	MasterName string
}

// Open builds the client for the requested mode and verifies connectivity.
// Callers must close the returned client.
func Open(cfg Config) (goredis.UniversalClient, error) {
	client, err := newClient(cfg)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}

// newClient performs validation and construction without network I/O so the
// mode handling is unit-testable.
func newClient(cfg Config) (goredis.UniversalClient, error) {
	mode := Mode(strings.ToLower(strings.TrimSpace(cfg.Mode)))
	if mode == "" {
		mode = ModeStandalone
	}
	addrs := cleanAddrs(cfg.Addrs)
	switch mode {
	case ModeStandalone:
		if len(addrs) == 0 {
			return nil, fmt.Errorf("redis: standalone mode requires an address")
		}
		if len(addrs) > 1 {
			return nil, fmt.Errorf("redis: standalone mode accepts exactly one address, got %d", len(addrs))
		}
		return goredis.NewClient(&goredis.Options{Addr: addrs[0], Password: cfg.Password}), nil
	case ModeSentinel:
		if len(addrs) == 0 {
			return nil, fmt.Errorf("redis: sentinel mode requires at least one sentinel address")
		}
		if strings.TrimSpace(cfg.MasterName) == "" {
			return nil, fmt.Errorf("redis: sentinel mode requires a master name")
		}
		return goredis.NewFailoverClient(&goredis.FailoverOptions{
			MasterName:    strings.TrimSpace(cfg.MasterName),
			SentinelAddrs: addrs,
			Password:      cfg.Password,
		}), nil
	case ModeCluster:
		if len(addrs) == 0 {
			return nil, fmt.Errorf("redis: cluster mode requires at least one seed address")
		}
		return goredis.NewClusterClient(&goredis.ClusterOptions{
			Addrs:    addrs,
			Password: cfg.Password,
		}), nil
	default:
		return nil, fmt.Errorf("redis: unknown mode %q (want %s, %s or %s)",
			cfg.Mode, ModeStandalone, ModeSentinel, ModeCluster)
	}
}

func cleanAddrs(addrs []string) []string {
	var out []string
	for _, a := range addrs {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}
