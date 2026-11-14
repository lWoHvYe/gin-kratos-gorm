package grpcx

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestAccessLogNeverLogsPayloads(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	handler := accessLog(logger)(func(ctx context.Context, req any) (any, error) {
		return &pb.LoginReply{AccessToken: "access-token-value", RefreshToken: "refresh-token-value"}, nil
	})
	req := &pb.LoginRequest{Username: "admin", Password: "super-secret-password"}
	if _, err := handler(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, secret := range []string{"super-secret-password", "access-token-value", "refresh-token-value"} {
		if strings.Contains(out, secret) {
			t.Fatalf("access log leaked %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, "grpc request") {
		t.Fatalf("expected a routing log line, got %s", out)
	}
}

func TestAccessLogKeepsErrorCause(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	handler := accessLog(logger)(func(ctx context.Context, req any) (any, error) {
		return nil, apperr.Internal("load order", errors.New("sql: connection refused"))
	})
	if _, err := handler(context.Background(), &pb.GetOrderRequest{OrderNo: "ORD1"}); err == nil {
		t.Fatal("expected an error")
	}
	out := buf.String()
	if !strings.Contains(out, "sql: connection refused") {
		t.Fatalf("error cause was dropped: %s", out)
	}
	if !strings.Contains(out, `"code":"INTERNAL"`) {
		t.Fatalf("domain code missing: %s", out)
	}
}
