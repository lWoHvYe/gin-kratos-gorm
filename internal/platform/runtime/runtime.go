package runtime

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"time"

	"gin-kratos-gorm/internal/platform/config"

	etcdregistry "github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2"
	kratoslog "github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewApp(cfg *config.Config, logger *slog.Logger, gs *kratosgrpc.Server, hs *khttp.Server, reg *etcdregistry.Registry) *kratos.App {
	opts := []kratos.Option{
		kratos.Name(cfg.Service),
		kratos.Logger(kratoslog.NewStdLogger(os.Stdout)),
		kratos.Server(gs, hs),
	}
	if reg != nil {
		opts = append(opts, kratos.Registrar(reg))
	}
	return kratos.New(opts...)
}

type Check func(context.Context) error

func HealthServer(cfg *config.Config, db *sql.DB, logger *slog.Logger) *khttp.Server {
	return HealthServerWithChecks(cfg, db, logger)
}

// HealthServerWithChecks adds required dependencies to /readyz. Order and
// payment use it to make DTM availability part of readiness; catalog/cart do
// not, so they keep serving reads when DTM is down.
func HealthServerWithChecks(cfg *config.Config, db *sql.DB, logger *slog.Logger, checks ...Check) *khttp.Server {
	srv := khttp.NewServer(khttp.Address(cfg.HTTPAddr))
	srv.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	srv.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if db != nil {
			if err := db.PingContext(ctx); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"not ready","dependency":"mysql"}`))
				return
			}
		}
		for _, check := range checks {
			if check == nil {
				continue
			}
			if err := check(ctx); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"not ready","dependency":"required"}`))
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
	srv.Handle("/metrics", promhttp.Handler())
	return srv
}
