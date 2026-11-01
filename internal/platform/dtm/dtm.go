package dtm

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/config"

	"github.com/dtm-labs/client/dtmcli"
	"github.com/dtm-labs/client/dtmgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	Address string
}

func New(cfg *config.Config) *Client { return &Client{Address: cfg.DTMAddress} }

func (c *Client) Saga(gid string) *dtmgrpc.SagaGrpc {
	s := dtmgrpc.NewSagaGrpc(c.Address, gid)
	s.WaitResult = true
	s.TimeoutToFail = 30
	s.RetryLimit = 3
	return s
}

// InBranch reports whether the current gRPC call is a DTM branch call.
func InBranch(ctx context.Context) bool {
	_, err := dtmgrpc.BarrierFromGrpc(ctx)
	return err == nil
}

// Branch runs fn inside the DTM branch barrier transaction. All business writes
// and outbox writes must use the provided *sql.Tx.
func Branch(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	barrier, err := dtmgrpc.BarrierFromGrpc(ctx)
	if err != nil {
		return fmt.Errorf("not a dtm branch context: %w", err)
	}
	return barrier.CallWithDB(db, fn)
}

// QueryPrepared lets DTM query whether a branch has already been prepared.
func QueryPrepared(ctx context.Context, db *sql.DB) error {
	barrier, err := dtmgrpc.BarrierFromGrpc(ctx)
	if err != nil {
		return err
	}
	return barrier.QueryPrepared(db)
}

var _ = dtmcli.BarrierFrom

// Probe checks that the DTM gRPC endpoint is reachable. Discovery URIs are
// skipped because the health endpoint has no registry client; those deployments
// rely on the DTM driver's own resolver.
func Probe(ctx context.Context, address string, timeout time.Duration) error {
	if address == "" {
		return fmt.Errorf("dtm address is empty")
	}
	if strings.Contains(address, "://") {
		return nil
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	dctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := grpc.DialContext(dctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("probe dtm %s: %w", address, err)
	}
	return conn.Close()
}

// TCC runs a DTM TCC global transaction. The callback registers branches with
// CallBranch; DTM performs Prepare/Try, then Confirm or Cancel.
func (c *Client) TCC(gid string, fn func(*dtmgrpc.TccGrpc) error) error {
	if gid == "" {
		return fmt.Errorf("empty tcc gid")
	}
	return dtmgrpc.TccGlobalTransaction2(c.Address, gid, func(tg *dtmgrpc.TccGrpc) {}, fn)
}
