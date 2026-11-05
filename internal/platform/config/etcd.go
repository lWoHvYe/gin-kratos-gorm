package config

import (
	"context"
	"fmt"
	"strings"
	"time"

	etcdconfig "github.com/go-kratos/kratos/contrib/config/etcd/v3"
	kratosconfig "github.com/go-kratos/kratos/v3/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdProvider is an optional configuration source backed by etcd. It is kept
// separate from the environment bootstrap so a missing etcd does not prevent
// services that do not opt in from starting.
type EtcdProvider struct {
	client *clientv3.Client
	config kratosconfig.Config
}

func LoadEtcd(ctx context.Context, endpoints []string, path string, prefix bool) (*EtcdProvider, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("etcd config path is empty")
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("etcd config endpoints are empty")
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("connect etcd config: %w", err)
	}
	source, err := etcdconfig.New(client,
		etcdconfig.WithContext(ctx),
		etcdconfig.WithPath(path),
		etcdconfig.WithPrefix(prefix),
	)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("create etcd config source: %w", err)
	}
	c := kratosconfig.New(kratosconfig.WithSource(source))
	if err := c.Load(); err != nil {
		_ = c.Close()
		_ = client.Close()
		return nil, fmt.Errorf("load etcd config: %w", err)
	}
	return &EtcdProvider{client: client, config: c}, nil
}

func (p *EtcdProvider) Scan(v any) error {
	if p == nil || p.config == nil {
		return fmt.Errorf("etcd config provider is nil")
	}
	return p.config.Scan(v)
}

func (p *EtcdProvider) Watch(key string, observer kratosconfig.Observer) error {
	if p == nil || p.config == nil {
		return fmt.Errorf("etcd config provider is nil")
	}
	return p.config.Watch(key, observer)
}

func (p *EtcdProvider) Close() error {
	if p == nil {
		return nil
	}
	var first error
	if p.config != nil {
		if err := p.config.Close(); err != nil {
			first = err
		}
	}
	if p.client != nil {
		if err := p.client.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
