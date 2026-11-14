# Operations guide

This document covers running, resetting and observing the mall stack. API and
schema conventions live in [contracts.md](contracts.md); DTM design lives in
[dtm.md](dtm.md).

## Prerequisites

- Docker Engine with Compose v2.20+ (BuildKit, `docker compose --wait`,
  profiles).
- Go 1.27 for local builds/tests (`go.mod` pins the toolchain).
- Roughly 4 GB of free RAM for the full stack, plus space for MySQL/NATS/
  Prometheus volumes.
- Generated gRPC code under `api/gen` is committed. Regenerate it with the
  `protoc` command from [contracts.md](contracts.md) after editing the proto.

## File map

| Path | Purpose |
|---|---|
| `Dockerfile` | one multi-stage image; `--build-arg SERVICE=<name>` selects `./cmd/<name>` |
| `Makefile` | build/test/fmt/vet, compose orchestration, `db-reset` |
| `deploy/compose/infra.yml` | MySQL, Redis, NATS, etcd, DTM, optional observability |
| `deploy/compose/services.yml` | gateway, admin and the 9 gRPC services |
| `deploy/compose/init/mysql/00-databases.sql` | creates every `mall_*` database plus `dtm`/`dtm_barrier` |
| `deploy/compose/init/mysql/01-schemas.sh` | applies `/migrations/*/*.sql` to `mall_<service>` |
| `deploy/compose/init/mysql/02-dtm.sql` | DTM store tables (`dtm.*`) and the shared branch barrier |
| `deploy/compose/dtm/config.yml` | DTM MySQL store; direct branch DNS, no etcd registration |
| `deploy/compose/observability/` | Prometheus scrape config, Grafana datasource, Tempo config |

## Compose model

`infra.yml` and `services.yml` are two halves of one project (`name: mall`) on
one explicitly named network (`mall`). Always pass both files so cross-file
`depends_on` entries and the shared network resolve:

```bash
docker compose -f deploy/compose/infra.yml -f deploy/compose/services.yml up -d
```

The Makefile builds that command (plus `--env-file deploy/compose/.env` when
the file exists), so prefer the make targets. Starting `services.yml` alone is
not supported: it has no infrastructure definitions by design.

No application container publishes a host port except:

| Container | Host port | Why |
|---|---|---|
| gateway | 8080 | public HTTP BFF |
| admin | 8081 | back-office HTTP BFF |
| traefik | 80 | optional ingress profile |

All internal traffic uses Docker DNS names (`mysql:3306`, `nats:4222`,
`etcd:2379`, `dtm:36790`, `catalog:9002`, …). Do not replace them with
`127.0.0.1` inside compose.

## Quick start

```bash
cp deploy/compose/.env.example deploy/compose/.env
make run-infra
make run-all
```

Check readiness:

```bash
docker compose -f deploy/compose/infra.yml -f deploy/compose/services.yml ps
curl http://localhost:8080/healthz
curl http://localhost:8081/healthz
```

Expected health endpoints:

- gateway: `GET http://localhost:8080/healthz`
- admin: `GET http://localhost:8081/healthz`
- each gRPC service: `GET http://<service>:910<1..9>/healthz`,
  `/readyz` (pings MySQL) and `/metrics` (Prometheus).

## Configuration

All configuration is environment based (`internal/platform/config`).

| Variable | Default | Notes |
|---|---|---|
| `APP_ENV` | `dev` | log field |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `GRPC_ADDR` | `:9000` | per-service compose value is `:900x` |
| `HTTP_ADDR` | `:9100` | health/metrics, per-service `:910x` |
| `MYSQL_DSN` | `root:root@tcp(127.0.0.1:3306)/mall?...` | per-service `mall_<service>` |
| `REDIS_ADDR` | `127.0.0.1:6379` | reserved for cache/idempotency |
| `NATS_URL` | `nats://127.0.0.1:4222` | JetStream is required (`-js`) |
| `ETCD_ENDPOINTS` | empty | optional configuration-center bootstrap; no service discovery |
| `DTM_ADDRESS` | `127.0.0.1:36790` | DTM gRPC endpoint |
| `JWT_SECRET` | dev placeholder | **minimum 16 characters** |
| `JWT_ISSUER` | `mall` | |
| `PAYMENT_SECRET` | empty | empty = mock payments, unsigned callbacks accepted |
| `CATALOG_ADDR` | `catalog:9002` | cart/pricing direct fallback |
| `PRICING_ADDR` | `pricing:9005` | order direct fallback |
| `INVENTORY_ADDR` | `inventory:9003` | order direct fallback |
| `ORDER_ADDR` | `order:9006` | payment direct fallback |
| `INVENTORY_DTM_URL` | `inventory:9003/mall.v1.InventoryService` | direct DTM branch base URL |
| `ORDER_DTM_URL` | `order:9006/mall.v1.OrderService` | direct DTM branch base URL |

Secrets live in `deploy/compose/.env` (git-ignored). Copy
`.env.example` and rotate `JWT_SECRET`/`PAYMENT_SECRET` for any shared
environment. MySQL credentials are intentionally `root`/`root` for local
development only.

## Database lifecycle

MySQL runs `deploy/compose/init/mysql/*` **only when `/var/lib/mysql` is empty**.
Order of execution:

1. `00-databases.sql` creates:
   `mall_identity`, `mall_catalog`, `mall_inventory`, `mall_cart`,
   `mall_pricing`, `mall_order`, `mall_payment`, `mall_fulfillment`,
   `mall_aftersales`, `mall_admin`, `dtm`, `dtm_barrier`.
2. `01-schemas.sh` walks `/migrations/<service>/*.sql` (mounted read-only from
   the repository), ensures `mall_<service>` exists, and applies files in
   version order (`sort -V`) to that database.
3. `02-dtm.sql` creates DTM's own store tables (`dtm.trans_global`,
   `dtm.trans_branch_op`, `dtm.kv`) and the shared `dtm_barrier.barrier`
   idempotency table.

Adding a migration:

```bash
# create migrations/<service>/0002_<change>.sql, then re-apply
docker compose -f deploy/compose/infra.yml exec mysql \
  bash /docker-entrypoint-initdb.d/01-schemas.sh
```

The script is idempotent for `CREATE TABLE IF NOT EXISTS` migrations. Do not
edit an already-applied migration; add a new numbered file.

Reset everything (destroys all named volumes, including NATS/etcd/Prometheus):

```bash
make db-reset          # down -v, then start MySQL and re-run init
make run-all           # start the application again
```

MySQL health becomes green only after the init scripts complete: during init
the entrypoint uses a socket-only temporary server, while the healthcheck
probes TCP.

## Startup order

The compose dependency graph is health based:

```text
mysql, redis, nats, etcd, dtm
        │
        ├── identity, catalog, inventory, fulfillment, aftersales
        ├── catalog ──> cart, pricing
        ├── pricing + inventory + dtm ──> order
        ├── order ──> payment
        ├── catalog/order/fulfillment/aftersales ──> admin
        └── all services ──> gateway
```

Because `make run-gateway` starts the gateway's dependencies, it effectively
brings up the full backend graph. Use `make run-service SERVICE=<name>` for a
smaller loop; its dependencies (core infra plus upstream services) are started
as needed.

## DTM operations

DTM uses `yedf/dtm:latest` with `deploy/compose/dtm/config.yml` mounted at
`/app/dtm/conf.yml`:

- store: MySQL `dtm` database on host `mysql`;
- HTTP API `:36789`, gRPC `:36790`;
- DTM is addressed directly by the application; branch URLs use direct
  Kubernetes/Docker DNS names, not etcd discovery.

DTM state and branch barriers:

- `dtm.trans_global`, `dtm.trans_branch_op`, `dtm.kv` — global transaction
  state, created by `02-dtm.sql` during MySQL initialization;
- `dtm_barrier.barrier` — branch idempotency barrier, also created by
  `02-dtm.sql` (the order migration creates it defensively as well).

The DTM server never drops these tables on restart. Only `dtm -r` resets them,
which the compose stack does not invoke.

Useful checks:

```bash
curl http://localhost:36789/api/dtmsvr/version
curl 'http://localhost:36789/api/dtmsvr/query?gid=order-create:<order_no>'
curl http://localhost:36789/api/metrics       # Prometheus exposition
```

Order creation, cancellation, payment confirmation and refunds require DTM.
If DTM is down, the services return `UNAVAILABLE`; they deliberately do not
fall back to local transactions. DTM calls branches by direct service DNS.

## Observability

Core metrics are exposed by each service on `/metrics`; Prometheus scrapes:

- all ten gRPC services on `9101`–`9110` (including seckill);
- the gateway/admin BFFs on `8080`/`8081`;
- DTM on `dtm:36789/api/metrics`;
- NATS on `nats:8222/metrics`;
- etcd on `etcd:2379/metrics`;
- itself on `localhost:9090`.

Gateway/admin expose `/metrics` on their HTTP port next to `/healthz`.

```bash
make run-observability
# Prometheus http://localhost:9090
# Grafana    http://localhost:3000  (admin/admin by default)
```

Grafana provisions Prometheus (default) and Tempo datasources. Tempo is
optional and is not started by `make run-observability`; enable it with:

```bash
docker compose -f deploy/compose/infra.yml -f deploy/compose/services.yml \
  --profile tracing up -d tempo
```

The Go services do not export traces yet; the Tempo OTLP endpoints (`4317`
gRPC, `4318` HTTP) and Grafana datasource are ready for that work.

CORS reflects any origin by default (dev). Set `CORS_ALLOWED_ORIGINS` to a
comma-separated allowlist for shared environments; unknown origins receive no
CORS headers. Request bodies are capped at 1 MiB per call.

The outbox relay backs off exponentially and dead-letters after 20 attempts
(`mall_outbox_dead_lettered_total`); terminal rows are pruned after 7 days.

Logs are JSON (`slog`) on stdout and are capped by the compose `json-file`
driver (10 MB × 3 per container):

```bash
docker compose -f deploy/compose/infra.yml -f deploy/compose/services.yml logs -f --tail=200 order
```

## Ingress (optional)

Traefik is included but disabled unless the `ingress` profile is enabled:

```bash
docker compose -f deploy/compose/infra.yml -f deploy/compose/services.yml \
  --profile ingress up -d traefik

curl http://localhost/api/v1/products
curl http://localhost/api/admin/v1/...   # requires an admin bearer token
```

Routing is declared with labels on the gateway/admin containers
(`/api/v1` → gateway `:8080`, `/api/admin/v1` → admin `:8081`). Traefik mounts
the Docker socket read-only and must join the `mall` network.

## Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| `service "mysql" is not defined` | `services.yml` was started without `infra.yml`; use the make targets |
| `network mall declared as external` / label mismatch | remove a manually created `mall` network or the stale project: `docker network rm mall` |
| service restarts with `dial tcp 127.0.0.1:3306` | an env address was overridden with localhost; use Docker DNS names |
| service restarts with `jetstream` / `no responders` | NATS was started without `-js`; use the provided `nats` service |
| `order` create fails with `UNAVAILABLE` | DTM is not healthy, or DTM cannot resolve `inventory:9003`; check DTM logs and service DNS |
| tables missing | data volume was not empty, so init scripts were skipped; run `make db-reset` |
| MySQL healthcheck red during first boot | init scripts still running; increase `start_period`/`retries` on slow hosts |
| port 8080/8081/80/3306 already in use | stop the conflicting process or remap the host port in compose |
| Grafana has no data | services not started yet, or Prometheus targets are down at `:9090/targets` |
| Traefik returns 404 | ingress profile not enabled, or gateway/admin labels not loaded (`--providers.docker.exposedbydefault=false`) |

Handy commands:

```bash
# status of the whole project
docker compose -f deploy/compose/infra.yml -f deploy/compose/services.yml ps

# validate the merged compose configuration without starting anything
docker compose -f deploy/compose/infra.yml -f deploy/compose/services.yml config

# logs for one service
docker compose -f deploy/compose/infra.yml -f deploy/compose/services.yml logs -f identity

# open a MySQL shell
docker compose -f deploy/compose/infra.yml exec mysql mysql -uroot -proot

# inspect databases/tables
docker compose -f deploy/compose/infra.yml exec mysql mysql -uroot -proot -e 'SHOW DATABASES;'
```

## Build, test and CI

```bash
make build        # ./bin/<service> for every cmd
make test         # unit tests only; no MySQL/NATS/DTM required
make e2e-phase15  # opt-in B-end/C-end/return E2E; running stack required
make concurrency-phase15 # opt-in one-unit oversell race check
make fmt-check    # gofmt gate
make vet          # go vet ./...
```

The Dockerfile is cache-friendly (module layer before source) and builds a
static binary (`CGO_ENABLED=0`) on `golang:1.27-alpine`, running as UID 10001 on
`alpine:3.20` with `ca-certificates`, `tzdata` and `wget` for healthchecks. A
missing `SERVICE` build argument fails the build with a clear message.

## Security notes

- `root`/`root` MySQL and the default `JWT_SECRET` are for local development
  only. Never expose MySQL, Redis, NATS, etcd, DTM, Prometheus or Grafana
  directly to the internet.
- The gateway is the only public application surface; admin requires a JWT
  with the `admin` role.
- `PAYMENT_SECRET` must be set to enforce payment callback signatures;
  otherwise the service logs a warning and runs in mock mode.
- Traefik mounts the Docker socket read-only. Keep the ingress profile off on
  hosts where that is not acceptable.

## Readiness and DTM

Every gRPC service exposes `/healthz` (process up) and `/readyz` (MySQL ping)
on its 910x port. `order` and `payment` additionally probe `DTM_ADDRESS` from
`/readyz`; they stay out of rotation when DTM is unreachable. Catalog/cart and
the other read paths do not depend on DTM and remain ready.

## Business metrics

Each service exposes `/metrics` from the default Prometheus registry:

- `mall_http_requests_total{method,path,status}` — BFF traffic.
- `mall_http_request_duration_seconds{method,path}` — BFF latency histogram.
- `mall_outbox_published_total` — successfully published outbox events.
- `mall_outbox_publish_errors_total` — publish failures pending retry.
- Plus Go/process metrics from the Prometheus client.

Suggested alerts: outbox error rate, outbox publish stall, HTTP 5xx ratio, and
order/payment readiness failures.

## etcd configuration overlay

Services can optionally load a startup configuration overlay from etcd. Set
`CONFIG_ETCD_PATH` (and optionally `CONFIG_ETCD_PREFIX=true`); environment
variables still win. See `docs/etcd-config.md` for the schema and precedence.

## Phase 1.5 end-to-end check

The Phase 1.5 flow exercises the B-end product model (category, brand, SPU,
SKU, images, attributes, publish), C-end search, address book, cart stock
validation, DTM checkout, mock payment, shipment, return tracking, inventory
restock and partial refund projection.

With the Kubernetes stack running:

```bash
kubectl -n mall port-forward svc/gateway 18080:8080 &
kubectl -n mall port-forward svc/admin   18081:8081 &
BASE_URL=http://127.0.0.1:18080 \
ADMIN_URL=http://127.0.0.1:18081 \
make e2e-phase15
```

The script creates uniquely named data on every run (`E2E-<epoch>`), initializes
inventory before publishing, and asserts the refund and stock projections. It
leaves its test data in the database; use `make db-reset` for a disposable
Compose environment.

The complementary concurrency check creates a one-unit SKU and starts two
checkout requests at the same time. Exactly one order may succeed, and the
inventory row must end at `on_hand=1`, `reserved=1`, `available=0`:

```bash
BASE_URL=http://127.0.0.1:18080 \
ADMIN_URL=http://127.0.0.1:18081 \
make concurrency-phase15
```

## Ambient certificate rotation on the test VM

Istio workload certificates are short-lived. If the VM is left running past
their TTL, ztunnel logs show `CertificateExpired` and the ingress gateway
returns 503. Restart the data plane so new certificates are fetched, then
restart the application pods so their in-pod ztunnel proxies reconnect:

```bash
kubectl -n istio-system rollout restart ds/ztunnel
kubectl -n istio-system rollout restart deployment/istiod
kubectl -n mall delete pod -l gateway.networking.k8s.io/gateway-name=mall-waypoint
kubectl -n istio-system rollout restart deployment/mall-gateway-istio
kubectl -n mall rollout restart deployment
```

## Promotion/coupon checks

The Phase 1.5 E2E also creates an active campaign and promotion, asserts the
quote's `promotion_discount_cent`, and checks that promotion reservations reach
`CONFIRMED` after payment. Campaigns created by previous E2E runs are paused at
the start of each run and the new rule is scoped to that run's SKU.

Coupon lifecycle is covered manually/against a live stack: placing an order
locks the coupon, a second quote sees it unavailable, cancellation unlocks it,
and paying a later order consumes it. The order create path now calls
`LockCoupon`; the paid path calls `ConsumeCoupon`; the cancelled consumer calls
`UnlockCoupon`.

## Production HA profile

The opt-in production profile (Sentinel, NATS cluster, external MySQL) and its
runbook live in [production-ha.md](production-ha.md). Validate the render with
`make k8s-ha-verify`; it needs no cluster.

## Seckill autoscaling

HPA and PDB manifests are opt-in in `deploy/k8s/charts/mall`:

```yaml
autoscaling:
  seckill:
    enabled: true
    minReplicas: 4
    maxReplicas: 40
    targetCPUUtilizationPercentage: 65
podDisruptionBudgets:
  seckill:
    enabled: true
    minAvailable: 1
```

HPA is a safety net, not a launch plan. Pre-scale the deployment before the
sale; reactive CPU scaling is too slow for a flash crowd. The current
single-node profile leaves these disabled.

## Integration checks

Run the seckill S1/S2/S3 checks against a running stack:

```bash
make seckill-integration
```

These cover activity/stock bucket/warmup, Redis admission/idempotency, and
JetStream order/payment. They are integration checks, not a capacity test.
