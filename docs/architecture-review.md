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

## Verified findings deferred to later phases

- **Seckill lifecycle** (Phase 2): `FAILED` qualifications are re-scanned forever
  because the expiry update only matches `ISSUED`; JetStream `MaxDeliver`
  exhaustion bypasses the application DLQ and metrics; the order-redelivery
  window can create two reservations (needs a failing test first).
- **Outbox** (Phase 2): no attempts cap/backoff/dead-letter/pruning; publishing
  happens while the claiming transaction holds row locks.
- **Worker lifecycle** (Phase 2): consumer subscription failures are logged but
  the pod stays Ready; SIGTERM does not drain consumers before DB/NATS close.
- **CI** (Phase 2): only the frontend workflow exists. `make wire` cannot run
  from a clean checkout because the Wire tool dependencies are absent from
  `go.sum`; the Phase 1 gateway provider change was mirrored into
  `wire_gen.go` manually. Add a `tools.go` and run `make wire` in CI.
- **Observability** (Phase 2/3): no tracing despite a provisioned Tempo; BFF
  `/metrics` is not exposed; the Compose Prometheus job omits `seckill:9110`.
- **Contracts** (Phase 2): OpenAPI covers 18 of 32 gateway routes; generated TS
  types are unused; no route-contract test exists yet.
- **Migration ledger** (Phase 3): every deploy replays all migrations
  (idempotent DDL + data backfills). Needs version+checksum tooling.
- **Aftersales** (Phase 2): no idempotency key, no automatic retry for stuck
  `REFUNDING`.
- **Security hardening** (Phase 2): no login rate limiting, reflective CORS,
  unbounded request bodies, internal gRPC trusts `user_id`/`admin` request
  fields (mesh-only trust).

## Needs a failing test before changing

- Seckill order creation under JetStream redelivery (two reservations for one
  held unit).
- Checkout crash window between coupon/promotion reservation and saga submit.
