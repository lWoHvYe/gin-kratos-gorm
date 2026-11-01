package main

import (
	"flag"
	"os"

	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/conf"
	"gin-kratos-gorm/internal/server"

	"github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/grpc/resolver/discovery"
	"github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/grpc/resolver"
)

// Name is the compiled service name.
var (
	Name     = "order"
	Version  string
	flagconf string
	id, _    = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "configs/order.yaml", "config path, eg: -conf configs/order.yaml")
}

// NewSaga uses compensation because stock lives in the product service.
func NewSaga(logger log.Logger) biz.Saga {
	return &biz.CompensationSaga{Log: log.NewHelper(logger)}
}

func newApp(cfg *conf.Bootstrap, logger log.Logger, gs *grpc.Server, hs *http.Server, worker *server.Worker, dis *etcd.Registry) *kratos.App {
	opts := []kratos.Option{
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{"env": cfg.App.Env}),
		kratos.Logger(logger),
		kratos.Server(gs, hs, worker),
	}
	if dis != nil {
		resolver.Register(discovery.NewBuilder(dis, discovery.WithInsecure(true)))
		opts = append(opts, kratos.Registrar(dis))
	}
	return kratos.New(opts...)
}
