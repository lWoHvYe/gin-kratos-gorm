package bootstrap

import (
	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/conf"
	"gin-kratos-gorm/internal/server"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewSaga uses a single database transaction: the monolith owns both the order
// and product tables, so saga compensation is unnecessary.
func NewSaga(tx biz.Transactor, logger log.Logger) biz.Saga {
	return &biz.TxSaga{Tx: tx, Log: log.NewHelper(logger)}
}

func NewApp(
	cfg *conf.Bootstrap,
	logger log.Logger,
	grpcSrv *grpc.Server,
	httpSrv *http.Server,
	worker *server.Worker,
) *kratos.App {
	return kratos.New(
		kratos.Name("gin-kratos-gorm"),
		kratos.Metadata(map[string]string{"env": cfg.App.Env}),
		kratos.Logger(logger),
		kratos.Server(
			grpcSrv,
			httpSrv,
			worker,
		),
	)
}
