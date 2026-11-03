# Kratos v3 upgrade

The Kratos runtime is now on `github.com/go-kratos/kratos/v3 v3.0.0`
(`github.com/go-kratos/kratos/contrib/registry/etcd/v3` for etcd).

## What changed

- Module path: `/v2` -> `/v3`; v3 requires Go 1.25+.
- Logging: Kratos uses standard `log/slog`; the project passes its
  `*slog.Logger` directly to `kratos.Logger` and the gRPC logging middleware.
  No `kratoslog.NewStdLogger` adapter.
- gRPC client: `kratosgrpc.DialInsecure` is gone; discovery dialing uses
  `kratosgrpc.NewClient(ctx, WithEndpoint(...), WithDiscovery(reg))`.
- Optional middleware (JWT/metrics/tracing) is in contrib modules. This project
  uses its own JWT and Prometheus instrumentation, so it does not add those
  Kratos contrib middleware.
- HTTP binding: v3 removed the exported `transport/http/binding` package. The
  project intentionally keeps business REST in Gin; Kratos HTTP is only the
  listener/lifecycle wrapper and health/metrics transport.
- JSON: v3 separates Go `json` and protobuf `protojson` codecs. Gin owns the
  BFF JSON contract, so no Kratos HTTP codec registration is needed.
- OpenTelemetry, streaming HTTP, typed `config.Get[T]`, and validation
  callbacks are available in v3 but are deliberately not enabled yet; they
  change wire contracts or add provider lifecycle that should be introduced
  with integration tests.

## Go 1.27 note

Kratos v3 pins `google.golang.org/grpc v1.81.0`. On Go 1.27,
`golang.org/x/net v0.54.0` removes the legacy HTTP/2 `TrailerPrefix` path and
grpc-go fails to compile. The project pins `golang.org/x/net v0.55.0`, which
restores compatibility. Keep this pin until grpc-go and x/net agree on the
Go 1.27 build tags.

## Deliberate architecture

- Kratos: gRPC transport, etcd registry/discovery, app lifecycle, middleware.
- Gin: C-end/admin REST BFF and aggregation.
- Wire: gateway/admin/order/payment composition roots.
- database/sql: DTM barriers, outbox, conditional updates.
- GORM: catalog/cart read side only.

The DTM driver module still imports the Kratos v2 resolver internally, so
`go.mod` retains `github.com/go-kratos/kratos/v2` as an indirect dependency
while application code uses v3 only.
