package tracing

import (
	"context"
	"testing"
	"time"
)

func TestInitWithoutEndpointIsNoop(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	shutdown, err := Init(context.Background(), "test-service")
	if err != nil {
		t.Fatal(err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestInitWithEndpointBuildsProvider(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "127.0.0.1:4317")
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.25")
	shutdown, err := Init(context.Background(), "test-service")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		// A missing collector is not a configuration error; shutdown must stay
		// best-effort.
		t.Logf("shutdown: %v", err)
	}
}
