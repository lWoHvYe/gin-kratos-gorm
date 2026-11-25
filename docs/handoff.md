# Session handoff

Read this first in a new conversation. It records the current truth, the
remaining work, and local environment pitfalls. Use `git log -1` for the exact
HEAD; running status remains in `docs/review-tracking.md`.

> **过时声明（2026-11-25）**：Snapshot 停在 2026-11-22 12:05，仅作历史背景；当前运行状态以 `docs/review-tracking.md` 为准，契约以 `docs/specs/` 与 `docs/adr/` 当前版本为准，PAY-SETTLE-01 分解以 `docs/lead/PAY-SETTLE-01/decomposition-review.md` 为准。

## Snapshot

- Date: 2026-11-22 12:05 CST.
- Code baseline: `016dd4f`; the handoff update itself is committed after it.
- Branch `main` was clean and in sync with `origin/main` at `016dd4f` before
  this handoff commit; the handoff commit is local until pushed.
- No force push is currently required; the old binary-removal history has
  already been reconciled.
- Repository: `/Users/lWoHvYe/IdeaProjects/gin-kratos-gorm`.
- C is functionally complete and validated in dev + isolated UAT.
- P3-0 contract prerequisites are complete.
- P3 **implementation is not started**; it is blocked on product/AI Spec+ADR,
  not on contract or P0/P1/P2 code.
- Production split remains **off**. Dev and UAT use `WAREHOUSE_MODE=split`.
- There is no large-scale production rollout.

## 2026-11-23 update (supersedes stale statements below)

- C's "functionally complete" verdict was withdrawn (H4). Production split
  remains off; dev/UAT still use `WAREHOUSE_MODE=split`.
- P0 payment settlement & fulfillment admission: SPEC-0002 v1.6 approved,
  SPEC-0001 v1.3 approved (group-refund release boundary). ADR-0007 is next;
  **no protocol code has been written**.
- P0-C landed: order list scan fix (+ regression tests), Helm wiring for the five
  SPEC-0001 flags (defaults false, render-time dependency guard), 0012 long-term
  split-chain decision. Audit: dev/UAT applied only up to `aftersales/0011`;
  no persistent environment applied 0012.
- AI production default is now `false`; `allow-ingress` was narrowed to
  `istio-system`; Pricing write DENY added. Live Mesh UAT is still pending.
- Current truth is `docs/review-tracking.md`; the Snapshot/P3 sections below are
  historical (2026-11-22) and must not be used as current status.

## Current architecture state

Core services: identity, catalog, inventory, cart, pricing, order, payment,
fulfillment, after-sales, seckill; gateway/admin BFFs; optional AI. One schema
per service; gRPC/events only between services; outbox in the business tx.

Recent consolidation:

- `orderActionPolicy` is the single order action authority; `Order.group_actions`
  is typed and `available_actions` is compatibility projection.
- `order_fulfillment_groups.refund_state` is explicit (`NONE/REFUNDING/REFUNDED`),
  projected from permit blocking and `aftersale.completed.v1`.
- A refunded unshipped group is closed for aggregate shipment projection; it no
  longer pins the order in `ALLOCATED`.
- Fulfillment voids `CREATED` parcels on group refund and checks group actions
  before auto-creating a draft.
- `WAREHOUSE_MODE=off|shadow|split` is the single switch; legacy booleans remain
  accepted and conflicting settings fail closed.
- `Payment.GetPaymentByOrder` aggregates all settled intents; `RefundOrder`
  allocates refunds across intents with order-level idempotency.
- Inventory operation replay exists as CLI, gRPC and admin BFF.
- AI mesh is read-only in practice: AI read succeeds, Catalog write returns
  `PermissionDenied`; `scripts/check-ai-mesh.sh`.
- `internal/platform/events/registry.go` holds typed schemas for every current
  core outbox event type; `outbox.Append` rejects unknown/malformed payloads.

## P3-0 contract prerequisites (complete)

- [ADR 0004](adr/0004-p3-0-contract-and-production-scope.md):
  production backup/restore and A/B/C rollback are gates for production
  enablement, not for P3 design/UAT; P3 must remain behind disabled flags.
- `make contract-check` covers:
  - proto evolution baseline;
  - typed event registry tests;
  - bidirectional admin/gateway OpenAPI route parity;
  - TypeScript regeneration with clean-diff check.
- Event producer maps are still used in places, but every payload is validated
  against the typed registry at the outbox boundary. Do not describe this as a
  full producer rewrite.

## C / split readiness

Functional validation is complete on dev and UAT:

- single E2E (`scripts/e2e-phase15.sh`);
- split E2E (`scripts/e2e-warehouse-split.sh`);
- split concurrency (`scripts/check-split-concurrency.sh`);
- split UI Playwright and admin split Playwright;
- Order/NATS/Fulfillment outage injection (`scripts/fault-split-uat.sh`);
- after-sales worker restart injection (`scripts/fault-split-worker-uat.sh`);
- group refund -> origin restock -> G1 remains shippable;
- group refund state reaches `REFUNDED`;
- AI mesh read/write check.

Production still needs, only when a production rollout is requested:

- production backup/restore and old-image smoke;
- production migration/rollback rehearsal;
- production secret/payment profile;
- capacity/chaos beyond the targeted correctness checks.

## P3 start gates

P3 should start with design artifacts, not code:

1. **AI Turn/Action**: Spec + ADR. Current AI has session/draft but no canonical
   `turn_id`/Action lifecycle. Need turn identity, session concurrency,
   draft/confirm/cancel, recovery, mesh read-only allowlist.
2. **Quantity split/allocator optimization**: product Spec + ADR. Current
   allocator (`internal/services/inventory/allocation.go`) deliberately picks
   one warehouse for the whole line and lowest warehouse id; no quantity split or
   cost model.
3. **Carrier integration**: provider/webhook/idempotency contract. Current
   Fulfillment only stores `carrier`/`tracking_no` strings.
4. **Multi-warehouse seckill**: blocked on production split and the quantity
   inventory model.

RAG hardening can be scoped independently, but existing RAG is still partial for
cache/rerank quality and release/recovery depth.

## Verification commands

Use the project-local Go caches in this sandbox; do not use the global cache:

```bash
cd /Users/lWoHvYe/IdeaProjects/gin-kratos-gorm
export PATH="$PWD/.tmp/bin:$PATH"
export GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp
export GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath
export GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
make verify
```

Contract/alert gates:

```bash
make contract-check   # proto + event registry + route parity + TS freshness
make alerts-check     # structural + promtool firing (Docker/prometheus image)
```

Integration needs port-forwards and env:

```bash
kubectl -n mall-data port-forward pod/mysql-0 13306:3306 &
kubectl -n mall-data port-forward pod/nats-0 14222:4222 &
kubectl -n mall-data port-forward pod/redis-0 16379:6379 &

TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:13306)/mall_integration_test?parseTime=true&loc=UTC' \
TEST_NATS_URL='nats://127.0.0.1:14222' \
TEST_REDIS_ADDR='127.0.0.1:16379' \
make test-integration
```

`make test-integration` derives `mall_integration_<package>` via
`scripts/test-dsn.sh`; packages no longer share one DB.

Frontend:

```bash
cd frontend
pnpm typecheck
pnpm build
pnpm test:e2e   # 9 tests at last run
```

## UAT environment

Currently running:

- app namespace `mall-uat`, release `uat`;
- data namespace `mall-uat-data`, release `uat-infra`;
- host `uat.mall.local`, shared `mall-gateway`;
- split enabled, optional AI not part of split validation;
- dev secret/mock payment profile — **not production security**.

Install/access/teardown: [UAT runbook](runbooks/uat-environment.md).

Port-forward examples:

```bash
kubectl -n mall-uat port-forward svc/gateway 18090:8080 &
kubectl -n mall-uat port-forward svc/admin 18091:8081 &
```

UAT-only checks:

```bash
BASE=http://127.0.0.1:18090 ADMIN=http://127.0.0.1:18091 \
  ./scripts/create-split-fixture.sh
. /tmp/split-fixture.env
BASE=http://127.0.0.1:18090 ADMIN=http://127.0.0.1:18091 \
  SPLIT_SKU_A=$SPLIT_SKU_A SPLIT_SKU_B=$SPLIT_SKU_B \
  ./scripts/check-split-concurrency.sh
```

`check-ai-mesh.sh` needs the grpcurl image imported into k3s; the UAT check
creates short-lived `ai-mesh-read`/`ai-mesh-write` Pods.

## Environment caveats

- Single-node dev/UAT only; do not run 100k/1M RPS.
- E2E projections are asynchronous. Wait/poll for promotion reservation
  `CONFIRMED`, order `ALLOCATED`, shipment projection and group refund state;
  the tests already do this.
- Start port-forwards only after deployments report rolled out; an early
  port-forward can return empty replies.
- Helm `mall-migrate` uses the hardened lease/attempt ledger in
  `deploy/k8s/images/migrate/run.sh`.
- `AGENTS.md` is now the multi-agent role charter; `docs/architecture-baseline.md`
  and `prompt/` are role artifacts. Review whether they should be committed
  according to the team workflow.
- Prometheus is not running in the k3s stack; `promtool` firing tests are local
  Docker tests, not a live alerting stack.

## Useful artifacts

- `docs/roadmap-status.md` — P0/P1/P2/P3-0 status and deferred scope.
- `docs/review-tracking.md` — canonical running ledger.
- `docs/c-warehouse-review.md` — C dev/UAT review.
- `docs/adr/0002-cross-warehouse-split-orders.md` — C design.
- `docs/adr/0003-c-production-rollout.md` / `runbooks/c-migration-rollback.md`.
- `docs/adr/0004-p3-0-contract-and-production-scope.md`.
- `scripts/check-split-concurrency.sh`, `fault-split-uat.sh`,
  `fault-split-worker-uat.sh`, `create-split-fixture.sh`.
- `scripts/check-alerts.sh`, `check-contracts.sh`, `check-ai-mesh.sh`.
- `frontend/e2e/warehouse-split.spec.ts`, `admin-split.spec.ts`.

## Next session

Do not resume by implementing P3. First choose one of:

- write the AI Turn/Action Spec + ADR;
- write the quantity split/allocator/carrier product Spec + ADR;
- harden RAG release/recovery as an isolated project.

Only then start code. Keep production split off until an explicit production
rollout decision and the ADR 0003 runbook are executed.
