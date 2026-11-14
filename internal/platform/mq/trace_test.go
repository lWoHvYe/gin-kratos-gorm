package mq

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestInjectTraceAddsTraceparent(t *testing.T) {
	previous := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(previous)

	provider := sdktrace.NewTracerProvider()
	ctx, span := provider.Tracer("test").Start(context.Background(), "publish")
	defer span.End()

	header := nats.Header{}
	injectTrace(ctx, header)
	if header.Get("Traceparent") == "" {
		t.Fatal("publish did not carry a traceparent header")
	}

	extracted := otel.GetTextMapPropagator().Extract(context.Background(), propagation.HeaderCarrier(header))
	if !extractedSpanValid(extracted) {
		t.Fatal("trace context did not round-trip through NATS headers")
	}
}

func extractedSpanValid(ctx context.Context) bool {
	span := trace.SpanFromContext(ctx)
	return span.SpanContext().IsValid()
}
