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

## Fixed in Phase 3 (in progress)

- Worker lifecycle: `runtime.Workers` tracks background loops; shutdown now
  stops servers first, then cancels/waits for workers, then drains NATS and
  closes the database. Consumer subscription failures are recorded and fail
  `/readyz` instead of leaving a silent pod that never projects events.
- Tracing: OpenTelemetry is wired into gRPC servers/clients and both BFFs, is
  opt-in via `OTEL_EXPORTER_OTLP_ENDPOINT` (Tempo is already provisioned in
  Compose), and propagates `traceparent` across service calls.

## Verified findings deferred to later phases

- **Seckill reservation idempotency** (Phase 3): the order-redelivery window can
  create two reservations; needs a failing test plus a unique key on
  `(activity_id, ref_no)`.
- **CI tooling** (Phase 3): `make wire` cannot run from a clean checkout
  because the Wire tool dependencies are absent from `go.sum`; provider changes
  are mirrored into `wire_gen.go` manually. Add a `tools.go` and run it in CI.
- **BFF platformization** (Phase 3): gateway/admin still duplicate ~250 lines
  of client/middleware/error helpers.
- **Internal authentication** (Phase 3): gRPC still trusts `user_id`/`admin`
  request fields (mesh-only trust). Needs an explicit decision.
- **OpenAPI coverage** (Phase 3): 18 of 32 gateway routes are documented; the
  guard only prevents documented routes from disappearing.
- **Migration ledger** (Phase 3): every deploy replays all migrations
  (idempotent DDL + data backfills). Needs version+checksum tooling.
- **Aftersales idempotency** (Phase 3): creation has no client key, so a double
  submit creates two after-sale rows (payment prevents a double payout).
- **Security hardening** (Phase 3): no login rate limiting; internal gRPC still
  trusts `user_id`/`admin` request fields (mesh-only trust).

## Needs a failing test before changing

- Seckill order creation under JetStream redelivery (two reservations for one
  held unit).
- Checkout crash window between coupon/promotion reservation and saga submit.
