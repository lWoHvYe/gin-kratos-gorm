// Package tracing wires OpenTelemetry into HTTP and gRPC. Export is opt-in:
// with OTEL_EXPORTER_OTLP_ENDPOINT empty the SDK stays a no-op, so local runs
// and tests do not depend on a collector.
package tracing

import (
	"context"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Shutdown flushes pending spans; call it after the servers stopped.
type Shutdown func(context.Context) error

// Init configures the global tracer provider and propagator. It returns a no-op
// shutdown when no OTLP endpoint is configured.
func Init(ctx context.Context, service string) (Shutdown, error) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	host := strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(host),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	// NewSchemaless avoids a schema-URL conflict when merging with the SDK's
	// default resource.
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		attribute.String("service.name", service),
	))
	if err != nil {
		return nil, err
	}
	ratio := 1.0
	if raw := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG")); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed >= 0 && parsed <= 1 {
			ratio = parsed
		}
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return provider.Shutdown, nil
}
