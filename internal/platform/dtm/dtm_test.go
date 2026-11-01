package dtm

import (
	"context"
	"testing"
	"time"

	"gin-kratos-gorm/internal/platform/config"

	"github.com/dtm-labs/client/dtmgrpc"
)

func TestNewSagaOptions(t *testing.T) {
	c := New(&config.Config{DTMAddress: "127.0.0.1:36790"})
	s := c.Saga("order-create:TEST")
	if s == nil {
		t.Fatal("nil saga")
	}
	if !s.WaitResult {
		t.Fatal("WaitResult must be true")
	}
	if s.TimeoutToFail != 30 {
		t.Fatalf("TimeoutToFail=%d", s.TimeoutToFail)
	}
	if s.RetryLimit != 3 {
		t.Fatalf("RetryLimit=%d", s.RetryLimit)
	}
}

func TestInBranchOutsideDTM(t *testing.T) {
	if InBranch(context.Background()) {
		t.Fatal("empty context must not be a DTM branch")
	}
}

func TestProbeRejectsUnreachable(t *testing.T) {
	if err := Probe(context.Background(), "127.0.0.1:1", 50*time.Millisecond); err == nil {
		t.Fatal("expected unreachable DTM probe to fail")
	}
	if err := Probe(context.Background(), "", time.Second); err == nil {
		t.Fatal("expected empty address to fail")
	}
}

func TestTCCRejectsEmptyGID(t *testing.T) {
	c := New(&config.Config{DTMAddress: "127.0.0.1:36790"})
	if err := c.TCC("", func(*dtmgrpc.TccGrpc) error { return nil }); err == nil {
		t.Fatal("expected empty gid error")
	}
}
