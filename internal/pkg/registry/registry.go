// Package registry builds the optional etcd service registry used by split
// deployments. The monolith and integration tests run without etcd.
package registry

import (
	"fmt"
	"time"

	"gin-kratos-gorm/internal/conf"

	etcdregistry "github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func New(cfg *conf.Bootstrap) (*etcdregistry.Registry, func(), error) {
	if cfg == nil || cfg.Registry == nil || cfg.Registry.Etcd == nil || len(cfg.Registry.Etcd.Endpoints) == 0 {
		return nil, func() {}, nil
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Registry.Etcd.Endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("connect etcd: %w", err)
	}
	reg := etcdregistry.New(client)
	cleanup := func() {
		_ = client.Close()
	}
	return reg, cleanup, nil
}
