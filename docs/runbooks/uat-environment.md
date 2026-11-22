# UAT environment in k3s

A second, isolated UAT stack for production-like validation without touching
the dev `mall`/`mall-data` releases.

| Plane | Namespace | Release |
|---|---|---|
| application | `mall-uat` | `uat` |
| data (MySQL/Redis/NATS) | `mall-uat-data` | `uat-infra` |
| ingress | shared `mall-gateway` | HTTPRoute host `uat.mall.local` |

The app namespace is ambient and uses `mall-waypoint`; the existing gateway is
reused (`ingress.createGateway=false`) so no second node-port/LoadBalancer is
required.

## Install

```bash
kubectl create namespace mall-uat-data
kubectl create namespace mall-uat
kubectl label namespace mall-uat \
  istio.io/dataplane-mode=ambient istio.io/use-waypoint=mall-waypoint --overwrite

helm upgrade --install uat-infra deploy/k8s/charts/infra -n mall-uat-data \
  -f deploy/k8s/charts/infra/values-single-node.yaml \
  -f deploy/k8s/uat/values-infra.yaml --wait --timeout 10m

kubectl -n mall-uat-data exec -i mysql-0 -- mysql -uroot -proot \
  --default-character-set=utf8mb4 < deploy/compose/init/mysql/03-dev-seed.sql
kubectl -n mall-uat-data exec -i mysql-0 -- mysql -uroot -proot \
  --default-character-set=utf8mb4 < deploy/seed/catalog-demo.sql

helm upgrade --install uat deploy/k8s/charts/mall -n mall-uat \
  -f deploy/k8s/charts/mall/values-single-node.yaml \
  -f deploy/k8s/uat/values-mall.yaml --wait --timeout 10m
```

`values-mall.yaml` enables split for this UAT only, disables optional AI, and
points MySQL/Redis/NATS at `mall-uat-data`. It uses the dev secret/mock payment
profile; it is **not** a production security profile.

## Access

The shared gateway exposes `uat.mall.local`:

```bash
curl --resolve uat.mall.local:80:10.211.55.29 http://uat.mall.local/healthz
```

For API scripts, port-forward the UAT services:

```bash
kubectl -n mall-uat port-forward svc/gateway 18090:8080 &
kubectl -n mall-uat port-forward svc/admin 18091:8081 &
BASE_URL=http://127.0.0.1:18090 ADMIN_URL=http://127.0.0.1:18091 \
  ./scripts/e2e-phase15.sh
```

## Split validation

Create two fresh SKUs instead of relying on demo seed:

```bash
BASE=http://127.0.0.1:18090 ADMIN=http://127.0.0.1:18091 \
  FIXTURE_ENV=/tmp/uat-split-fixture.env ./scripts/create-split-fixture.sh
. /tmp/uat-split-fixture.env
```

`create-split-fixture.sh` creates product A stocked in warehouse 1 and product B
stocked in warehouse 2, then publishes both. Publishing now checks available
stock across all warehouses, not only warehouse 1.

Then run the split checks:

```bash
BASE=http://127.0.0.1:18090 ADMIN=http://127.0.0.1:18091 \
  SPLIT_SKU_A=$SPLIT_SKU_A SPLIT_SKU_B=$SPLIT_SKU_B \
  ./scripts/e2e-warehouse-split.sh
BASE=http://127.0.0.1:18090 ADMIN=http://127.0.0.1:18091 \
  SPLIT_SKU_A=$SPLIT_SKU_A SPLIT_SKU_B=$SPLIT_SKU_B \
  ./scripts/check-split-concurrency.sh
```

Browser:

```bash
cd frontend
WEB_BASE_URL=http://uat.mall.local \
API_BASE_URL=http://127.0.0.1:18090 \
ADMIN_API_BASE_URL=http://127.0.0.1:18091 \
SPLIT_SKU_A=$SPLIT_SKU_A SPLIT_SKU_B=$SPLIT_SKU_B \
pnpm exec playwright test --config=e2e/playwright.config.ts e2e/warehouse-split.spec.ts
```

## UAT evidence (2026-11-22)

- fresh `uat-infra` migration job completed; all services healthy;
- `e2e-phase15.sh` passed on UAT;
- split E2E reached `COMPLETED/RECEIVED` with G1/G2;
- UAT Playwright split UI passed (groups rendered, G2 receive -> PARTIALLY_RECEIVED);
- UAT group refund: G2 return/restock/refund, then G1 shipped successfully
  -> `SHIPPED/SHIPPED`, `refund=PARTIAL`;
- UAT admin G2 shipment Playwright passed;
- `scripts/check-split-concurrency.sh` passed: duplicate payment, concurrent
  G1/G2 ship, unshipped G2 refund cancels its draft parcel, G2 refund/ship race
  leaves G1 shippable;
- `scripts/fault-split-uat.sh` passed: Order outage fail-closed, NATS outage
  and Fulfillment outage recovered durable order.paid/auto-shipment flow;
- `scripts/fault-split-worker-uat.sh` passed: after-sales restock worker was
  stopped with a PROCESSING task, restarted, restock converged and G2 reached
  REFUNDED.

## Teardown

```bash
helm uninstall uat -n mall-uat
helm uninstall uat-infra -n mall-uat-data
kubectl delete namespace mall-uat mall-uat-data
```

This deletes the UAT PVCs as well; do not run it if the UAT data is needed.
