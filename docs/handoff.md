# Session handoff

Use this file as the first read in a new conversation. It is intentionally short
and points to the durable design/access/test documents instead of repeating
months of chat history.

## Project

`gin-kratos-gorm` is a Go/Kratos/Gin/GORM/DTM mall with a Vue admin, Nuxt
storefront, a separate seckill service and a single-node k3s + Istio Ambient
deployment for development.

Repository:

```text
/Users/lWoHvYe/IdeaProjects/gin-kratos-gorm
```

Last reviewed baseline (see `git log -1` for the actual HEAD):

```text
3672848 ops: add optional seckill autoscaling and integration target
39010a8 perf: add short-lived sold-out admission cache
526e1be docs: design 100x seckill scale-out
```

## Live development stack

```text
Gateway:  10.211.55.29:80, Host mall.local
Store:    http://mall.local/
Seckill:  http://mall.local/seckill
Admin:    http://mall.local/admin/
Accounts: admin/admin123, demo/user123 (dev only)
```

The ingress host needs a hosts entry:

```text
10.211.55.29 mall.local
```

If the browser cannot use the host, use the full port-forward fallback in
[docs/access.md](access.md):

```bash
kubectl -n mall port-forward svc/storefront-web 13000:3000
kubectl -n mall port-forward svc/admin-web 18082:8080
```

Both frontends proxy `/api` to the in-cluster gateway/admin services.

## What is already implemented

- Identity/catalog/inventory/cart/pricing/order/payment/fulfillment/after-sales.
- Promotion campaigns, rules, budgets, reservations, BEST stacking and simulator.
- Vue 3 admin: catalog, inventory, orders, shipments, after-sales, promotions,
  budget/reservations and seckill activity management.
- Nuxt 3 + Vant storefront: browse/product/cart/checkout/payment/orders/
  after-sales/addresses, including `/seckill` and `/seckill/:id`.
- Seckill S1/S2/S3:
  - inventory bucket allocation and Redis warmup;
  - Redis Lua admission, HMAC token, idempotency and rate limit;
  - durable inventory holds and expiry;
  - NATS JetStream worker, `CreateSeckillOrder`, payment intent and pay flow;
  - cancellation projection, DLQ publish/replay, report-only reconciler;
  - sold-out short cache and Prometheus counters.
- Playwright covers the full seckill UI flow (reserve/poll/pay/order).

## Read next

1. [access.md](access.md) — where the UI is and how to reach it.
2. [seckill-design.md](seckill-design.md) — especially section 19 for the 100x
   scale-out design (1M users, 10 SKUs, 100 units each).
3. [services/seckill.md](services/seckill.md) — current S1-S4 API/behavior.
4. [frontend-plan.md](frontend-plan.md) — frontend architecture and scope.
5. [ops.md](ops.md) — cluster/runtime troubleshooting.

## First commands in a new session

```bash
cd /Users/lWoHvYe/IdeaProjects/gin-kratos-gorm
./scripts/bootstrap-context.sh
git status --short
```

For Go builds/tests in the local sandbox use the cached environment:

```bash
GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath \
GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off \
make verify
```

Frontend:

```bash
cd frontend
pnpm typecheck
pnpm test
pnpm build
PLAYWRIGHT_BROWSERS_PATH=$PWD/../.tmp/playwright-browsers \
  pnpm exec playwright test e2e --config=e2e/playwright.config.ts
```

Integration smokes need the live port-forwards:

```bash
make seckill-integration          # S1 + S2 + S3
```

## Important operational caveats

- The cluster is single-node and dev-only. Do not run a 100k/1M RPS load test
  against this VM; the 100x plan is in the design doc.
- Redis and NATS are single-replica in this profile. Production needs Redis
  Cluster/Sentinel and NATS JetStream 3 replicas.
- Ambient workload certificates expire roughly daily. If ingress returns 503
  after a long idle period, restart ztunnel/istiod, recreate the waypoint and
  restart gateway/app pods; see ops.md.
- Port-forwards are not persistent and die on pod/gateway restarts.
- `kubectl` live state can drift from the last commit. Run the status script
  before assuming a pod/image is current.
- The cluster contains accumulated E2E/seckill test data. Do not treat its
  dashboards/stock as a clean production dataset.
- Demo credentials must be removed/rotated before any real deployment.
- `frontend/pnpm-lock.yaml` is committed; frontend node_modules and pnpm store
  are ignored.

## Next priority

This is the recommended continuation, in order:

1. Production HA manifests/runbook: dedicated Redis Cluster/Sentinel, NATS 3
   replicas, MySQL HA, resource/connection limits.
2. Edge admission: managed WAF/Waiting Room or tested Envoy+rate limit; origin
   cap around the measured target (currently design target ~100k RPS).
3. Pre-scale/HPA/PDB validation for gateway and seckill-api; HPA/PDB templates
   are already opt-in in the Helm chart.
4. Bulk-create/group 10 SKU activities under one campaign; decide whether the
   per-user limit is per activity or per campaign.
5. DLQ/admin operations UI; automatic reconciliation repair only after alerts
   are stable.
6. Distributed k6 in an isolated performance environment when infrastructure
   exists; skip it on the local VM.

Do not redo the S1/S2/S3 schema, Redis Lua flow, JetStream worker or C-end
seckill page from scratch. Extend them.

## Bootstrap prompt for a new conversation

Paste this into a new DSH Web conversation using the same workspace/preset:

```text
继续 gin-kratos-gorm 项目。先阅读仓库里的 docs/handoff.md，
然后运行 ./scripts/bootstrap-context.sh 和 git status 确认当前状态。
不要重新规划已经完成的 S1/S2/S3；按 handoff 的“Next priority”继续，
先做生产 HA 与边缘准入，再补管理/对账/压测基础设施。
修改前先说明本次范围，修改后运行相关集成测试。
```

The goal is to keep the new session small: the durable files are the context,
not the old chat transcript.
