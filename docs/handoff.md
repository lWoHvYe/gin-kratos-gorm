# Session handoff

Read this first in a new conversation. It records the current truth, the
remaining work, and the local environment pitfalls. Do not use an old chat
transcript as the source of truth.

## Project

`gin-kratos-gorm`: Go/Kratos/Gin/GORM/DTM mall with Vue admin, Nuxt storefront,
a separate seckill service, and a single-node k3s + Istio Ambient dev deployment.

Repository:

```text
/Users/lWoHvYe/IdeaProjects/gin-kratos-gorm
```

Latest feature commit at handoff:

```text
ee7a204 feat(settlement): separate order lifecycle from refund and shipping state
```

This handoff file itself is committed immediately after that feature commit. See
`git log -1` for the exact HEAD.

Local `main` is ahead of `origin/main`. History was rewritten once to remove an
accidentally committed `ai-pref-eval` binary, so a force push is required when
the branch is published:

```bash
git push --force origin main
```

## What is actually done

### Core mall

- Identity/catalog/inventory/cart/pricing/order/payment/fulfillment/after-sales.
- Seckill Redis Lua admission, durable holds, NATS worker, DLQ/replay, sold-out
  cache, Playwright flow.
- HA reference profile, PDB/topology spread, Alertmanager, OTLP egress policy.

### P1/P2 handoff fixes

- P1#1 closed-bucket release writes exactly one ledger/event; MySQL test.
- P1#2 `order.cancelled` uses the inventory cancel operation protocol; partial
  MarkPaid compensation includes the failing SKU; durable confirm consumer
  retries the confirm operation; NATS confirm/order.cancelled tests.
- P1#3 `Stock.version` + `cmd/catalog-stock-rebuild`; live dry-run found and
  fixed 3 projection mismatches.
- P1#4 HITL full draft hash, fail-closed meta, close failures surfaced.
- P2#8 public SKU visibility; P2#9 page/cursor semantics + keyset tests on GORM
  and SQL; P2#10 `BatchGetStock`; P2#11 hidden AI entry; P2#12 exact seed
  cleanup migrations; P2#13 strict identity on high-risk routes; P2#14 unified
  integration target; P2#17 RAG versioning/lease/cache/metrics.
- P2#17 is still **partial** only for cache/rerank quality comparison: the
  independent build Job/CronJob and load-only follower Pods now exist.

### Transaction refactor B (inventory whole-order operations)

- New `inventory_operations` / `inventory_operation_items` (migration `0015`)
  with deterministic `(order_no, operation_type, scope_hash)` identity.
- Confirm/Cancel no longer return on the first item error. Non-DTM calls
  process each pending item in its own transaction + savepoint, persist
  per-item `DONE`/`FAILED_PERMANENT`/`MISSING` progress, continue other items,
  and Ack only when every item is terminal. Consumers NAK while any item is
  retryable; permanent failures emit per-item events instead of retrying the
  whole batch.
- DTM confirm/cancel use one `dtm.Branch` transaction for the whole item set.
  After a branch commit (including a barrier replay) the committed operation is
  read back so a previous `PARTIAL` cannot be reported as success.
- Terminal `shop.inventory.operation.completed.v1`/`.failed.v1` events are
  written in the final item's transaction; `ReplayInventoryOperation` is the
  explicit recovery path for failed/missing items and refuses DONE items.
- `cmd/catalog-stock-rebuild` now refuses to overwrite a newer catalog
  projection (`stock_version` CAS) and reports stale/conflict/missing rows.

### Transaction refactor A (order/after-sales/payment/fulfillment)

- `orders.status` is lifecycle-only; `refund_state` and the
  `refund_status/refunded_cent` projection are orthogonal. The payment refund
  consumer no longer writes `PARTIALLY_REFUNDED/REFUNDED` into `status`.
- `order_shipping_permits` + `AuthorizeShipment`/`BlockShipmentForRefund` make
  Order the shipping/refund race arbiter; Fulfillment `Ship` must authorize
  first. A permit already `CONSUMED` still allows refund-only after shipment
  (human decision), while an un-used permit is revoked before refund.
- `pb.Order` exposes `refund_state`, `shipment_status` and server-owned
  `available_actions`; storefront/admin actions use that list.
- After-sales has a per-order lifetime `after_sale_amount_ledger` using Payment
  `captured_cent` (order snapshot only for legacy), immutable
  `after_sale_amount_versions`, `RETURN_RECEIVED` semantics, per-SKU
  `after_sale_restock_tasks`, and durable `after_sale_refund_tasks`.
  `RETURN_REFUND` cannot enter `REFUNDING` until every restock task is `DONE`;
  no status scanner remains (old `RunRefundRecovery` deleted).
- `Payment.GetPaymentByOrder` is the read-only settlement fact source for
  after-sales. Migrations: orders `0004`, after-sales `0010`; both backfills
  were exercised on scratch MySQL data.

### AI

- ADK tool-aware streaming is working: cumulative DeepSeek chunks are
  normalized and tool-call fragments merge by `ToolCall.Index`. Real eval
  15/15, tool-selection 3/3, normal answer emitted 246 SSE deltas.
- RAG: versioned index/alias, dual-write, post-build/15m reconciliation,
  exact/semantic cache, rollback runbook, 3m lease with 1m renewal. Builds are
  owned by the `ai-rag-build` Job (Helm hook) and `ai-rag-build-scheduled`
  CronJob; AI Pods are load-only followers of `ai:rag:current`
  (`RAG_ALIAS_REFRESH`, default 1m). The build Job skips a fresh version
  (`RAG_BUILD_MAX_AGE`, default 24h; `RAG_BUILD_FORCE` overrides); a fresh
  pointer whose index is missing is treated as stale. Compose has a one-shot
  `ai-rag-build` service.
- HITL: `INTERRUPTED -> CONFIRMING -> SAVED`, `FAILED` via Abort, admin UI state
  display, abort button. State changes use one Redis Lua CAS
  (`hitlStateTransitionCAS`), so concurrent confirms cannot regress SAVED or
  lose a FAILED reason.
- Cart: draft tool -> Redis draft -> SSE draft -> storefront card -> Cart
  `ConfirmCartDraft` transaction + `(user_id,draft_id)` idempotency. Handler
  tests cover expired/price/stock/Complete-failure.
- Preferences: user-scoped Redis profile, 180d retention, UI, system injection,
  per-session attribution key, Redis user-isolation test, A/B 3/3. Chat emits an
  `attribution` SSE event with the explicit profile fields applied to that
  answer; the storefront renders it with an "调整偏好" link. Only the owner's
  profile is attributed (cross-user test).
- Ops: deterministic daily brief, atomic latest/done, low-stock links, feedback,
  optional webhook. Delivery requires `OPS_BRIEF_WEBHOOK_SECRET` (>=16 chars),
  signs `timestamp.body` with HMAC-SHA256 (`X-Mall-Signature`), sends
  `X-Mall-Timestamp`/`X-Mall-Delivery`/`X-Mall-Event`, and only marks done on
  2xx. Unsigned configuration is refused, not downgraded.

## Remaining work, in priority order

1. **Real GitHub CI** — local `make test-integration` has passed; Actions was
   not actually run.

2. **Phase 8 benchmark breadth** — attribution UI and the owner-scoped event are
   implemented, but the A/B benchmark still uses one prompt family; add a
   cross-user/no-memory regression set.

3. **Phase 9 richer anomaly explanations** — the signed webhook is implemented;
   exception explanations are still mostly low-stock links, not
   cancellation/refund anomalies.

4. **P3 cleanup** — Shipment/Refund/ListReply are still hand-written types; admin
   split can continue.

5. **Live DTM/rollout checks for A/B** — local MySQL/NATS protocol tests and
   `make test-integration` passed, but the new image was not rolled out and
   `make dtm-smoke` was not run against the DTM barrier. A's migrations were
   applied to scratch MySQL data, but the new order/after-sales/fulfillment
   images were not rolled out; no browser E2E for the new actions. Inventory
   B's replay remains an internal/API-pending method.

## Verification commands

Go environment in this sandbox (do not use the global Go cache):

```bash
cd /Users/lWoHvYe/IdeaProjects/gin-kratos-gorm
export PATH="$PWD/.tmp/bin:$PATH"
export GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp
export GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath
export GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
make verify
```

Targeted tests:

```bash
go test ./internal/services/ai ./internal/services/gateway \
  ./internal/services/admin ./internal/services/inventory \
  ./internal/services/catalog ./internal/services/order \
  ./internal/services/aftersales ./internal/services/payment \
  ./internal/services/fulfillment ./internal/platform/bff -count=1
```

The HITL CAS, RAG follower/build-due and attribution tests run only with a
Redis 8 (with Search) address:

```bash
TEST_REDIS_ADDR=127.0.0.1:16379 go test ./internal/services/ai \
  ./internal/services/admin ./internal/platform/bff -count=1
```

Frontend changes need `pnpm -C frontend typecheck && pnpm -C frontend build`.

Integration needs port-forwards to MySQL/NATS/Redis and `TEST_*` envs:

```bash
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:13306)/mall_integration_test?parseTime=true&loc=UTC' \
TEST_NATS_URL='nats://127.0.0.1:14222' \
TEST_REDIS_ADDR='127.0.0.1:16379' make test-integration
```

The Makefile target includes outbox/consumer, AI RAG + lease + preference
isolation + RAG follower + HITL CAS + attribution, seckill, inventory
seckill/compensation/ledger/NATS + whole-order operation protocol, order
settlement/shipping permits, payment settlement read, after-sales
amount/restock/refund protocol, fulfillment permit, catalog projection +
keyset, and pricing cleanup.

Live AI:

```bash
make ai-eval       # 15/15 expected
make ai-rag-eval   # hit@5 5/5 expected
make ai-pref-eval  # A/B 3/3 expected
```

## Live checks this session (2026-11-21)

- `bin/ai rag-build` against the dev catalog and Redis built 64 documents into a
  throwaway `ai_kb_livecheck` index; a second run skipped on freshness
  (`RAG_BUILD_MAX_AGE=24h`) and `RAG_BUILD_FORCE=true` built v2. The throwaway
  index, pointer and registry entries were removed afterwards.
- `ai:rag:current` had drifted to a dropped `ai_kb_test_...` index, written by an
  older RAG test before pointer keys were namespaced. It was restored to
  `ai_kb_current` (`ai_kb_v1795208929702651081`, 64 docs). The RAG test now
  deletes its pointer and dual-write registry entry, and non-default index bases
  use `<base>:current`.

## Environment caveats

- Cluster is single-node dev only. Do not run 100k/1M RPS.
- Ambient certificates expire roughly daily; use `make ambient-recover` if
  ingress/waypoint returns 503.
- CoreDNS was restored to `forward . /etc/resolv.conf`; host resolver has been
  flaky. If DNS fails, check CoreDNS upstream before changing config.
- The embedding/provider endpoint is flaky; AI Pods now start without RAG and
  wait for the build Job to publish `ai:rag:current`. `rag disabled` at startup
  is not fatal, and a failed build leaves the previous alias serving.
- Port-forwards die on restarts; recreate before live tests.
- Cluster contains accumulated E2E data; do not treat dashboards/stock as clean.
- Demo credentials are dev-only and must be rotated for real deployments.
- Migration state: cart `0004` was applied via a manual migrate Job; rebuild the
  migrate image and rerun the Job when deploying to a new environment.
- `frontend/pnpm-lock.yaml` is committed; node_modules/.nuxt/.output are ignored.

## First steps in a new chat

```bash
cd /Users/lWoHvYe/IdeaProjects/gin-kratos-gorm
./scripts/bootstrap-context.sh
git status --short
git log --oneline -5
```

Then read:

1. [review-tracking.md](review-tracking.md) — current P1/P2/P3/AI truth table.
2. [ai-hardening-review.md](ai-hardening-review.md) — AI remaining risks.
3. [ai-roadmap.md](ai-roadmap.md) — Phase 5-10 status.
4. [ai-rag-rollback.md](ai-rag-rollback.md) — RAG rollback steps.

## Bootstrap prompt for a new conversation

```text
继续 gin-kratos-gorm。先读 docs/handoff.md、docs/review-tracking.md 和
docs/ai-hardening-review.md，运行 ./scripts/bootstrap-context.sh 与 git status。
不要重做已完成的主链路。按 handoff 的 Remaining work 顺序，先做真实 GitHub CI，
再处理 Phase8/9 与 P3，并补 A/B 的 live DTM/rollout 验证。设计先行，禁止为状态
组合继续加 if、recovery scanner 或前端特判。每个改动报告文件、检查、未验证项；
不要在没有证据时标 done。
```

The durable files are the context; keep the new session small.
