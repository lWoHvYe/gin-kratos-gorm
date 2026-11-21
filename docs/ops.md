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
| `deploy/compose/infra.yml` | MySQL, Redis, NATS, DTM, optional Prometheus/Grafana/Tempo/Alertmanager |
| `deploy/compose/services.yml` | gateway, admin and the 9 gRPC services |
| `deploy/compose/init/mysql/00-databases.sql` | creates every `mall_*` database plus `dtm`/`dtm_barrier` |
| `deploy/compose/init/mysql/01-schemas.sh` | applies `/migrations/*/*.sql` to `mall_<service>` |
| `deploy/compose/init/mysql/02-dtm.sql` | DTM store tables (`dtm.*`) and the shared branch barrier |
| `deploy/compose/dtm/config.yml` | DTM MySQL store; direct branch DNS, no etcd registration |
| `deploy/compose/observability/` | Prometheus scrape/alerts, Alertmanager, Grafana datasource, Tempo config |

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
`dtm:36790`, `catalog:9002`, …). Do not replace them with
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
   version order (`sort -V`) to that database. Each database has a
   `schema_migrations(filename, checksum, applied_at)` ledger: applied files are
   skipped, and editing an applied file fails the job on its checksum. The
   Kubernetes migration image (`deploy/k8s/images/migrate/run.sh`) uses the same
   ledger, so Helm upgrades no longer replay DDL or data backfills.
3. `02-dtm.sql` creates DTM's own store tables (`dtm.trans_global`,
   `dtm.trans_branch_op`, `dtm.kv`) and the shared `dtm_barrier.barrier`
   idempotency table.

Adding a migration:

```bash
# create migrations/<service>/0002_<change>.sql, then re-apply
docker compose -f deploy/compose/infra.yml exec mysql \
  bash /docker-entrypoint-initdb.d/01-schemas.sh
```

The ledger covers both runners. Do not edit an already-applied migration; add a
new numbered file (the ledger fails the job when a checksum changes). A database
that predates the ledger replays the existing files once (they are idempotent)
and then records them.

Reset everything (destroys all named volumes, including NATS/Prometheus):

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
mysql, redis, nats, dtm
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

Order creation uses DTM saga; payment confirmation uses DTM TCC when enabled.
Cancellation is a local status transition plus `shop.order.cancelled.v1`
compensation, not a DTM gid; direct MarkPaid is only used when TCC is disabled or
unconfigured. DTM calls branches by direct service DNS.

## Observability

Core metrics are exposed by each service on `/metrics`; Prometheus scrapes:

- the gRPC services on `9101`–`9111` (including seckill and ai);
- the gateway/admin BFFs on `8080`/`8081`;
- DTM on `dtm:36789/api/metrics`;
- NATS on `nats:8222/metrics`;
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

The Go services now export OTLP spans when `OTEL_EXPORTER_OTLP_ENDPOINT` is set
(for example `tempo:4317` with the tracing profile above). Sampling defaults to
100%; set `OTEL_TRACES_SAMPLER_ARG` (0–1) to reduce it. Instrumented hops:

- HTTP (both BFFs) and gRPC servers/clients;
- SQL statements through `otelsql` (each query is a child span);
- NATS publishes carry `traceparent` in the message headers and durable
  consumers continue the trace.

Asynchronous events keep the trace: `outbox.Append` stores the producing
`traceparent` on the row, the relay opens a producer span linked to it, and the
consumer continues from the message header. DTM transactions carry the context
in `BranchHeaders`, which DTM forwards to every saga/TCC branch call.

With the endpoint empty the tracer stays a no-op, so local runs need no
collector. The database pool is exported regardless of tracing:

```text
mall_db_pool_open_connections / in_use / idle / max_open
mall_db_pool_wait_count_total / wait_duration_seconds_total
```

CORS only reflects origins listed in `CORS_ALLOWED_ORIGINS`; unset means no
cross-origin browser access (same-origin and server-side calls are unaffected).
Request bodies are capped at 1 MiB and `page` is capped at 1000 per call.

The outbox relay claims batches under a lease (20 rows; 4-minute lease) and
exports `mall_outbox_pending_events` / `mall_outbox_oldest_pending_seconds` and publishes outside the
database transaction. Failures back off exponentially, dead-letter after 20
attempts (`mall_outbox_dead_lettered_total`), and terminal rows are pruned after
7 days; dead-letter rows are first copied to `outbox_dead_letters` so payloads
remain auditable.

Durable consumers share one policy (`internal/platform/consumer`): a 10s
per-message timeout, bounded retries, and dead-lettering to `<subject>.dlq`
after `MaxDeliver`. Outcomes are counted by
`mall_consumer_messages_total{subject,result}`. Inspect/replay a DLQ only after
confirming the business state and the handler's idempotency:

```bash
nats -s nats://nats:4222 stream info MALL
nats -s nats://nats:4222 consumer info MALL <durable>
nats -s nats://nats:4222 sub shop.order.paid.v1.dlq --limit 1
```

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
make e2e-warehouse-split # opt-in split E2E; two SKUs stocked in two warehouses
make rehearse-c-migration # disposable MySQL: pre-C dumps -> migrate -> restore
make test-migrate-runner # disposable MySQL: FAILED refusal + concurrent lease
make concurrency-phase15 # opt-in one-unit oversell race check
make fmt-check    # gofmt gate
make vet          # go vet ./...
```

The Dockerfile builds a static binary (`CGO_ENABLED=0`) on
`golang:1.27-alpine`, running as UID 10001 on `alpine:3.20` with
`ca-certificates`, `tzdata` and `wget` for healthchecks. A missing `SERVICE`
build argument fails the build with a clear message.

Docker's build does **not** inherit the host `GOPROXY` or HTTP proxy. The
Makefile passes the host `go env GOPROXY` into `k8s-images`; in China the host
usually resolves to `https://goproxy.cn,direct`. Override explicitly if needed:

```bash
GOPROXY=https://goproxy.cn,direct make k8s-images
```

Module and Go-build cache mounts persist across source edits, so an incremental
service rebuild normally reuses dependencies (module download is sub-second;
the changed package and link step remain). `.dockerignore` excludes `.tmp` and
`frontend`, keeping the Go/migrate context small. `docker builder prune` can
reclaim old BuildKit layers if Docker reports a large reclaimable cache.

Building all 14 images in `make k8s-images` is intentionally not the fast path;
rebuild only the services you changed. A first-ever build (or after pruning)
must still download modules and populate the Go build cache.

## Security notes

- `root`/`root` MySQL and the default `JWT_SECRET` are for local development
  only. Never expose MySQL, Redis, NATS, DTM, Prometheus, Alertmanager or Grafana
  directly to the internet.
- The gateway is the only public application surface; admin requires a JWT
  with the `admin` role.
- `PAYMENT_SECRET` must be set to enforce payment callback signatures.
  `PAYMENT_MOCK_ENABLED=true` is dev-only: a non-dev environment refuses to
  start with mock payments, and an invalid boolean fails config loading instead
  of silently falling back.
- The AI service is authorized only for explicit read methods on catalog,
  inventory, promotion evaluation, orders and after-sales; the mesh rules in
  `deploy/k8s/charts/mall/values.yaml` contain no `/*` wildcard for the `ai`
  service account.
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
make ambient-recover
```

The script restarts ztunnel/istiod, recreates the waypoint, restarts the ingress
gateway and mall deployments, and waits for `Gateway PROGRAMMED=True`.

### AI model egress

The `ai` service calls DeepSeek and Qwen embedding endpoints over public HTTPS.
`allow-ai-egress` permits public addresses for AI pods only; k3s ignored
port-qualified ipBlock rules in this environment, so it has no port restriction.
Production should use a dedicated egress gateway with domain allow-listing
instead. Model keys live in the `mall-ai-secrets` Secret.

### AI RAG build and ops webhook

AI Pods are load-only: they re-read `ai:rag:current` every `RAG_ALIAS_REFRESH`
(default `1m`) and never embed the catalog. The `ai-rag-build` Job (Helm
`post-install,post-upgrade` hook) and `ai-rag-build-scheduled` CronJob own the
build; both run the same `mall/ai` image with `args: ["rag-build"]`. A current
version newer than `RAG_BUILD_MAX_AGE` (default `24h`) is reused, so hook
retries and repeated upgrades are cheap; `RAG_BUILD_FORCE=true` overrides the
check. A fresh pointer whose index was dropped is treated as stale, so the Job
rebuilds and self-heals pointer drift. Compose runs the one-shot
`ai-rag-build` service. A failed build logs the
error and exits 0 so the optional AI build cannot block a mall release; the
previous `_current` alias keeps serving and the 6-hour CronJob retries. Redis or
configuration errors still fail fast, and a failed build is visible in the Job
log.

The optional ops-brief webhook is authenticated: when
`OPS_BRIEF_WEBHOOK_URL` is set, `OPS_BRIEF_WEBHOOK_SECRET` is mandatory
(>=16 characters) and an unsigned request is refused rather than sent. Each
POST carries `X-Mall-Timestamp`, `X-Mall-Delivery`, `X-Mall-Event: ops.brief`
and `X-Mall-Signature: sha256=<hex HMAC-SHA256(secret, timestamp + "." + body)>`.
Receivers should reject timestamps outside their replay window. A non-2xx
response leaves the day pending so the scheduler retries.

### After draining the namespace

`kubectl -n mall scale deployment --all --replicas=0` also scales the Ambient
waypoint to zero; the mesh then reports `unknown waypoint` and gRPC calls reset.
Restore `deployment/mall-waypoint` to one replica, wait for the `Gateway` to be
`PROGRAMMED=True`, restart ztunnel, then roll the application deployments.

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

## A/B settlement migration and production rollout

The dev cluster was migrated on 2026-11-21 with the Helm `mall-migrate` job:
`order/0010_settlement_protocol.sql`, `aftersales/0010_settlement_protocol.sql`,
`inventory/0014_remove_smoke_seed.sql` and `inventory/0015_inventory_operations.sql`.
Before the job, the same migrations were applied to clones of the real
`mall_order`/`mall_aftersales`/`mall_inventory` data and the affected databases
were dumped.

Production order:

1. Take a consistent backup of `mall_order`, `mall_aftersales`,
   `mall_inventory`, `mall_payment` and `mall_fulfillment`.
2. Rehearse the SQL on a clone and record row counts/backfill distributions.
3. Run the migration job before rolling new pods.
4. Roll Order, Payment, Inventory and AfterSales first; then Fulfillment
   (it calls `Order.AuthorizeShipment`); then Gateway/Admin and frontends.
5. Keep the old image digests. The new columns/tables are additive, but
   `after_sales.RECEIVED -> RETURN_RECEIVED` is a one-way data rename: old
   AfterSales code cannot resume new requests after a rollback. The supported
   rollback is restore-from-backup or roll forward; do not invent a down SQL.

`WAREHOUSE_SPLIT_ENABLED` stays `false` in both the dev and production values.
Phase 4/5/6 now have per-group shipments, permits, returns/refunds and
group-aware storefront/admin UI. The dev single-node values enable split; the
production defaults keep `WAREHOUSE_SPLIT_ENABLED=false` until the
[production rollout/rollback rehearsal](runbooks/c-migration-rollback.md)
passes against a production backup. The disposable dev rehearsal is
`make rehearse-c-migration`; it uses the pre-C dumps in `/tmp/mall-predeploy`
by default.

The AI RAG build is not a release gate by default. `ai-rag-build-scheduled`
runs as a CronJob; `ai.ragBuild.hook=true` opts into the old blocking
post-upgrade Job. Trigger a build manually with:

```bash
kubectl -n mall create job --from=cronjob/ai-rag-build-scheduled ai-rag-build-manual
```
