# Development environment and test commands

This is the durable entry point for developers running code and tests locally.
It is not a session snapshot. For the current state use
[`review-tracking.md`](review-tracking.md) (the canonical running ledger);
[`roadmap-status.md`](roadmap-status.md) is a 2026-11-22 roadmap snapshot.
For a session boundary use [`review-tracking.md`](review-tracking.md); `handoff.md`
was retired on 2026-11-27.

Operations/deployment (compose, Helm, production runbooks) remain owned by
[`ops.md`](ops.md); UAT is in
[`runbooks/uat-environment.md`](runbooks/uat-environment.md).

## 1. Toolchain and local Go environment

Use the repository-local caches; do not rely on the global Go cache:

```bash
cd /Users/lWoHvYe/IdeaProjects/gin-kratos-gorm
export PATH="$PWD/.tmp/bin:$PATH"
export GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp
export GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath
export GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
```

Go module changes require an explicit decision; `GOFLAGS=-mod=readonly` is
intentional.

## 2. Build and unit tests

```bash
go build ./...
go test ./... -count=1
make verify
```

`make verify` runs the normal build/test/contract checks. Do not describe a
retired machine gate as “passed”; G-1…G-4 are manual checklists only.

## 3. Integration tests (real MySQL / NATS / Redis)

Local compose or port-forwards must be up first; see
[`ops.md`](ops.md). The test DSN is the base; each package derives its own
schema via `scripts/test-dsn.sh`:

```bash
export TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_integration_test?parseTime=true&loc=UTC'
export TEST_NATS_URL='nats://127.0.0.1:4222'
export TEST_REDIS_ADDR='127.0.0.1:6379'
make test-integration
```

Do not point two packages at the same schema when tests DROP/CREATE tables;
use the per-package DB helper. Integration tests skip without
`TEST_MYSQL_DSN`; a SKIP is not a PASS.

For a focused package:

```bash
TEST_MYSQL_DSN="$TEST_MYSQL_DSN" go test ./internal/services/order -count=1
TEST_MYSQL_DSN="$TEST_MYSQL_DSN" go test -race -p 1 ./internal/services/order -count=1
```

## 4. Frontend

```bash
cd frontend
pnpm typecheck
pnpm build
pnpm test:e2e
```

## 5. Migrations

- Applied migration files are immutable (checksum ledger); never edit them.
- Add a **new** migration for schema changes; use `migrations/<service>/NNNN_name.sql`.
- The runner uses a lease/attempt ledger: a failed `0012`-style attempt must be
  repaired explicitly; do not rely on “run Helm again”.
- Validate on a fresh/disposable MySQL 8.4 database, not only inline test DDL.
- The migration scope is per service; check `migrations/<service>/` and the
  service's migration runner.

## 6. Async and environment pitfalls

- Projections are asynchronous (order status, shipment, group refund, stock).
  Poll/await in tests; a single immediate read can be stale.
- Port-forward only after deployments report rolled out.
- Dev/UAT are single-node; do not run 100k/1M RPS.
- Production split stays **off** unless an explicit production rollout decision
  and the ADR-0003 runbook are executed.
- UAT uses the dev secret/mock payment profile; it is not production security.

## 7. Evidence rules

- Bind every test claim to the exact HEAD.
- Money, inventory, permissions and cross-service recovery need `-race` or
  fault-injection evidence; a happy path is not enough.
- AC-REF is not AC-VERIFIED; AC 状态见
  [`SPEC-0002 §15`](specs/0002-payment-settlement-admission/spec.md#15-验收状态表) 与
  [`review-tracking.md`](review-tracking.md)。
- Include `【测试范围】` and `【未覆盖范围】` in the delivery.

## 8. Canonical pointers

| Need | Canonical |
|---|---|
| Current state / gaps | [`review-tracking.md`](review-tracking.md) |
| Roadmap / deferred work | [`roadmap-status.md`](roadmap-status.md) |
| Contracts | [`specs/`](specs/), [`adr/`](adr/) |
| Architecture baseline | [`architecture-baseline.md`](architecture-baseline.md) |
| Operations / compose / Helm | [`ops.md`](ops.md) |
| UAT | [`runbooks/uat-environment.md`](runbooks/uat-environment.md) |
| Session boundary | [`review-tracking.md`](review-tracking.md) — handoff 已于 2026-11-27 停用 |
