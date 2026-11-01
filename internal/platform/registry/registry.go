package registry

import (
	"fmt"
	"time"

	etcdregistry "github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func New(endpoints []string) (*etcdregistry.Registry, func(), error) {
	if len(endpoints) == 0 {
		return nil, func() {}, nil
	}
	client, err := clientv3.New(clientv3.Config{Endpoints: endpoints, DialTimeout: 5 * time.Second})
	if err != nil {
		return nil, nil, fmt.Errorf("connect etcd: %w", err)
	}
	reg := etcdregistry.New(client)
	return reg, func() { _ = client.Close() }, nil
}
