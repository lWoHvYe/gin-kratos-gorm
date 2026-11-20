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

Current HEAD at handoff:

```text
0201db7 feat: expose HITL state and abort in admin UI
```

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
- P1#2 `order.cancelled` uses `TccCancelConfirm`; partial MarkPaid compensation
  includes the failing SKU; durable confirm consumer retries `TccConfirm`;
  NATS confirm/order.cancelled tests.
- P1#3 `Stock.version` + `cmd/catalog-stock-rebuild`; live dry-run found and
  fixed 3 projection mismatches.
- P1#4 HITL full draft hash, fail-closed meta, close failures surfaced.
- P2#8 public SKU visibility; P2#9 page/cursor semantics + keyset tests on GORM
  and SQL; P2#10 `BatchGetStock`; P2#11 hidden AI entry; P2#12 exact seed
  cleanup migrations; P2#13 strict identity on high-risk routes; P2#14 unified
  integration target; P2#17 RAG versioning/lease/cache/metrics.
- P2#17 is still **partial**: no independent RAG Job/elected leader.

### AI

- ADK tool-aware streaming is working: cumulative DeepSeek chunks are
  normalized and tool-call fragments merge by `ToolCall.Index`. Real eval
  15/15, tool-selection 3/3, normal answer emitted 246 SSE deltas.
- RAG: versioned index/alias, dual-write, post-build/15m reconciliation,
  exact/semantic cache, rollback runbook, 3m lease with 1m renewal, lazy proxy
  with 2m background retry.
- HITL: `INTERRUPTED -> CONFIRMING -> SAVED`, `FAILED` via Abort, admin UI state
  display, abort button.
- Cart: draft tool -> Redis draft -> SSE draft -> storefront card -> Cart
  `ConfirmCartDraft` transaction + `(user_id,draft_id)` idempotency. Handler
  tests cover expired/price/stock/Complete-failure.
- Preferences: user-scoped Redis profile, 180d retention, UI, system injection,
  per-session attribution key, Redis user-isolation test, A/B 3/3.
- Ops: deterministic daily brief, atomic latest/done, low-stock links, feedback,
  optional webhook (only 2xx marks done).

## Remaining work, in priority order

1. **RAG Job / elected leader**
   - Current lazy proxy removes permanent startup disable, but there is still no
     independent Job and a first build >3m can leave followers without alias.
   - Goal: a dedicated build Job or leader with lease renewal; AI pods only load
     the current alias.

2. **HITL atomic CAS + save-failure live**
   - `FAILED`/Abort/UI exist, but `CONFIRMING` transition is read/modify/write,
     not atomic CAS.
   - Add a Lua/CAS transition and a live save-failure -> FAILED -> retry test.

3. **Phase 8 UI attribution / regression suite**
   - Redis/log attribution and isolation test exist; no UI to show which
     preferences influenced a recommendation, and the benchmark uses one prompt
     family.

4. **Phase 9 webhook auth / richer anomalies**
   - Optional webhook exists with no signature/auth.
   - Exception explanation is mostly low-stock links; cancellation/refund
     anomaly explanations are not built.

5. **Real GitHub CI** — local `make test-integration` has passed; Actions was
   not actually run.

6. **P3 cleanup** — Shipment/Refund/ListReply are still hand-written types; admin
   split can continue; RAG Job remains the main operational gap.

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
  ./internal/services/catalog ./internal/services/order -count=1
```

Integration needs port-forwards to MySQL/NATS/Redis and `TEST_*` envs:

```bash
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:13306)/mall_integration_test?parseTime=true&loc=UTC' \
TEST_NATS_URL='nats://127.0.0.1:14222' \
TEST_REDIS_ADDR='127.0.0.1:16379' make test-integration
```

The Makefile target includes outbox/consumer, AI RAG + lease + preference
isolation, seckill, inventory seckill/compensation/ledger/NATS, catalog
projection + keyset, and pricing cleanup.

Live AI:

```bash
make ai-eval       # 15/15 expected
make ai-rag-eval   # hit@5 5/5 expected
make ai-pref-eval  # A/B 3/3 expected
```

## Environment caveats

- Cluster is single-node dev only. Do not run 100k/1M RPS.
- Ambient certificates expire roughly daily; use `make ambient-recover` if
  ingress/waypoint returns 503.
- CoreDNS was restored to `forward . /etc/resolv.conf`; host resolver has been
  flaky. If DNS fails, check CoreDNS upstream before changing config.
- The embedding/provider endpoint is flaky; AI now starts before RAG and retries
  in the background. `rag disabled` at startup is not automatically fatal.
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
不要重做已完成的主链路。按 handoff 的 Remaining work 顺序，先做 RAG Job/elected
leader，再做 HITL 原子 CAS + save-failure live，然后 Phase8 UI 归因和 Phase9
webhook 鉴权。每个改动报告文件、检查、未验证项；不要在没有证据时标 done。
```

The durable files are the context; keep the new session small.
