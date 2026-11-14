# Architecture review (2026-11)

Scope: architecture, technology choices and a systematic implementation review of
the whole repository. This file records what was checked, what was fixed in the
Phase 1 hardening pass, and what is deliberately deferred.

## Verdict

The core design holds up: per-service schemas, gRPC-only service boundaries, a
transactional outbox with JetStream, DTM saga/TCC for checkout, conditional SQL
for stock, and an isolated seckill admission tier with Redis Lua. The biggest
risks were not the architecture but **operational weaknesses around it**: secrets
that fail open, logging of request bodies, missing failure compensation, no Go
CI, and thin tests on the money paths.

Technology choices are kept:
- **Kratos v3 + Gin**: retained. Kratos supplies lifecycle/transport/middleware;
  Gin owns the BFF REST surface. The BFF duplication is the next refactor, not a
  reason to replace either framework.
- **database/sql core + GORM read side**: retained; `check-architecture.sh`
  enforces it.
- **DTM + outbox + JetStream**: retained. Gaps are in outbox governance and
  compensation state machines, not in the choice.
- **Ten services / ten schemas**: retained. Do not merge into a monolith; invest
  in shared BFF/consumer/observability infrastructure instead.

## Fixed in Phase 1 (this change set)

| Area | Change |
|---|---|
| Logging | gRPC request bodies are no longer logged (login passwords, refresh tokens, payment signatures). Routing metadata + error cause only. |
| Secrets | Production (`APP_ENV != dev`) refuses the shipped JWT secret, requires a distinct `SECKILL_TOKEN_SECRET` for seckill, and requires `PAYMENT_SECRET` unless mock payments are explicitly enabled. Chart production overlay stops rendering the dev secret. |
| Mock payments | The public `/payments/:paymentNo/pay` route is opt-in (`PAYMENT_MOCK_ENABLED`), the intent amount is authoritative, and a client-supplied amount must match. |
| Demo accounts | Removed from migrations; seeded only by the Compose dev init (`03-dev-seed.sql`). |
| Errors | gRPC/HTTP log the wrapped cause and domain code before redacting the client message; `mall_errors_total{surface,code}` added. |
| Paid-order flow | `MarkPaid` validates amount/status before confirming inventory; a cancel racing a capture emits a refund event instead of silently consuming stock. TCC cancel now cancels unpaid orders locally instead of parking them in a terminal REFUNDING state; zero-amount refund events are acked with a metric. |
| JSON contract | BFF protobuf replies are rendered through a reflection encoder: snake_case, zero values present, int64 as numbers. Fixes NaN/blank money in the storefront. |
| Admin API | Added `GET /api/admin/v1/orders/{orderNo}` (the UI already called it). |
| Frontend session | Admin and storefront refresh the access token once on 401 (single-flight) and replay the request; checkout uses a stable idempotency key per attempt; cart total and money math fixed. |
| Ops docs | Production runbook now documents the three application secrets and the mock-payment switch. |

## Fixed in Phase 2

- Go CI: `.github/workflows/go.yml` (format, vet, unit, race, build, arch, DTM
  contract, protobuf drift, govulncheck); frontend workflow also runs on
  `docs/openapi/**`.
- Seckill: `FAILED` qualifications reach `EXPIRED` instead of being rescanned
  forever; a permanent payment-intent failure now takes the DLQ/fail path
  instead of acking an unpayable ORDERED qualification.
- Outbox: exponential backoff, 20-attempt dead-letter with metric, 7-day
  pruning; migration `0005_outbox_retry.sql` in every service DB.
- Shutdown: NATS `Drain()` waits for in-flight handlers before closing.
- Observability: BFF `/metrics`, gRPC duration histogram, Compose Prometheus
  scrapes seckill and both BFFs.
- Contracts: tests assert every documented OpenAPI path is still served
  (gateway + admin).
- Aftersales: stuck `REFUNDING` rows are retried by a recovery worker with a
  metric.
- Config/security: BFFs honour `HTTP_ADDR`, gRPC honours the config centre's
  `grpc_addr`, request bodies are capped at 1 MiB, CORS supports an allowlist.

## Trust model

Internal service authentication is recorded in
[adr/0001-internal-trust.md](adr/0001-internal-trust.md): workload-level trust
through mTLS + AuthorizationPolicy + NetworkPolicy, with the residual risks and
upgrade triggers for a per-user signed context.

## Fixed in Phase 3 (in progress)

- Worker lifecycle: `runtime.Workers` tracks background loops; shutdown now
  stops servers first, then cancels/waits for workers, then drains NATS and
  closes the database. Consumer subscription failures are recorded and fail
  `/readyz` instead of leaving a silent pod that never projects events.
- Tracing: OpenTelemetry is wired into gRPC servers/clients and both BFFs, is
  opt-in via `OTEL_EXPORTER_OTLP_ENDPOINT` (Tempo is already provisioned in
  Compose), and propagates `traceparent` across service calls.
- Seckill reservation idempotency: the order number is derived from the
  qualification token, so a redelivered command reuses the same order and the
  same `(order_no, sku_id)` reservation instead of creating a second one; a
  unique `(activity_id, ref_no)` index is the database backstop. The duplicate
  order path no longer releases the winner's reservation.
- Migration ledger: both the Compose init script and the Kubernetes migration
  image track `schema_migrations(filename, checksum, applied_at)`, skip applied
  files and fail on a changed checksum.
- After-sale creation idempotency: the BFF forwards `Idempotency-Key` as gRPC
  metadata, `after_sales` has a unique `(user_id, client_request_id)` key, and a
  retried submit returns the first request.
- Login throttling: identity allows 10 attempts per username per 5 minutes
  through Redis, resets on success, and fails open with a warning if Redis is
  unavailable.
- Wire tooling: `tools.go` pins the generator, `make wire` works from a clean
  checkout (it immediately caught a stale order graph: the promotion client was
  missing from `wire.go`), and CI now fails when committed `wire_gen.go` drifts.
- Tracing depth: SQL statements are spanned through `otelsql`, NATS publishes
  carry `traceparent`, durable consumers continue the trace, and outbox rows
  persist the producing context so the relay's publish span links back to the
  original request.
- Pool observability: `sql.DBStats` is exported as `mall_db_pool_*` gauges and
  counters.
- Cancellable startup: all services open MySQL with `runtime.StartupContext`,
  and DTM transactions carry `traceparent` in branch headers.
- Outbox integration tests: real-MySQL tests cover lease claiming, foreign
  owner completion, backoff/dead-letter and pruning; CI runs them against a
  MySQL service container.
- Outbox lease relay: a short transaction claims a batch (`lease_owner`,
  `lease_until`), commits, then publishes outside the transaction; failures back
  off, dead-letter after 20 attempts, and an expired lease is reclaimed.
- BFF readiness: `/healthz` is liveness, `/readyz` reports gRPC dependency
  states, fails only for critical deps (identity/order/payment) and returns
  `degraded` otherwise.
- Consumer DLQ: `internal/platform/consumer` applies one policy to all durable
  subscriptions - 10s per-message timeout, bounded retries, `<subject>.dlq`
  dead-lettering, panic recovery and `mall_consumer_messages_total`.
- Gateway layout: `router.go` now only wires routes; handlers are grouped by
  domain (auth, catalog, address, cart, checkout, order, payment, aftersales,
  seckill) with `clients.go` and `handlers.go` alongside.
- BFF platformization: `internal/platform/bff` now owns recovery, timeouts,
  JSON binding with the 1 MiB cap, path/query parsing and downstream error
  mapping. This removed ~200 duplicated lines and fixed a real drift (admin
  mapped ResourceExhausted to 503, gateway to 429; both now return 429).

## Verified findings deferred to later phases

- **Tracing depth**: the chain is connected end to end (HTTP, gRPC, SQL, NATS,
  outbox, DTM branch headers).
- **Method-level AuthorizationPolicy**: current rules are service-prefixed; a
  DENY for system RPCs must be validated in staging before enabling.
- **Startup retry** is cancellable: `runtime.StartupContext` is wired into
  `db.OpenWithPoolContext`, so SIGTERM aborts a boot-time MySQL wait.
- **Fault-injection tests**: `make chaos` stops a real NATS instance while the
  relay runs (events must recover without dead-lettering) and stops Redis while
  seckill admission runs (must fail closed, then recover with correct stock).
  Slow-downstream and full-stack chaos still need a staging environment.
- **Checkout crash window**: coupon/promotion reservation happens before the
  saga submit; the window needs a failing test before changing.
