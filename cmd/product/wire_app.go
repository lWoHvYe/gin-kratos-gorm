package main

import (
	"flag"
	"os"

	"gin-kratos-gorm/internal/conf"

	"github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/grpc/resolver/discovery"
	"github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/grpc/resolver"
)

var (
	Name     = "product"
	Version  string
	flagconf string
	id, _    = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "configs/product.yaml", "config path, eg: -conf configs/product.yaml")
}

func newApp(cfg *conf.Bootstrap, logger log.Logger, gs *grpc.Server, hs *http.Server, dis *etcd.Registry) *kratos.App {
	opts := []kratos.Option{
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{"env": cfg.App.Env}),
		kratos.Logger(logger),
		kratos.Server(gs, hs),
	}
	if dis != nil {
		resolver.Register(discovery.NewBuilder(dis, discovery.WithInsecure(true)))
		opts = append(opts, kratos.Registrar(dis))
	}
	return kratos.New(opts...)
}
