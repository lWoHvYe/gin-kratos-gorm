package aftersales

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestClientRequestIDFromMetadata(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-client-request-id", "  as-key  "))
	if got := clientRequestID(ctx); got != "as-key" {
		t.Fatalf("clientRequestID = %q", got)
	}
	if got := clientRequestID(context.Background()); got != "" {
		t.Fatalf("missing metadata = %q", got)
	}
	long := strings.Repeat("k", 100)
	ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-client-request-id", long))
	if got := clientRequestID(ctx); len(got) != 64 {
		t.Fatalf("truncated key length = %d", len(got))
	}
}

func TestIsDuplicateKey(t *testing.T) {
	if !isDuplicateKey(errors.New("Error 1062: Duplicate entry 'x' for key 'uk_after_sales_client_request'")) {
		t.Fatal("MySQL duplicate entry must be detected")
	}
	if isDuplicateKey(errors.New("connection refused")) {
		t.Fatal("unrelated error must not be treated as a duplicate")
	}
}
