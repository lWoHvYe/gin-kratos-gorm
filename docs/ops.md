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

## Settlement consumer switch (fulfillment)

`fulfillment` selects its settlement consumer from three environment values
(ADR-0007 Addendum-r0.4 §2.1, accepted 2026-11-26). The process only **reads**
`settlement_switch_approvals` at startup; it never writes approval rows.

| Variable | Default | Contract |
|---|---|---|
| `FULFILLMENT_SETTLE_CONSUMER_MODE` | `paid` | `paid` / `shadow` / `settled`. Unknown or empty (when set) fails startup. |
| `SETTLE_RELEASE_APPROVAL_ID` | empty | Required only for `settled`; must be empty for `paid`/`shadow` (N5). |
| `SETTLE_RELEASE_RANGE` | empty | Required for `shadow`/`settled`; must exactly equal the approval row's `release_range`. Empty is **not** a wildcard. |
| `FULFILLMENT_CONSUME_ORDER_SETTLED` | unset | Legacy input only: `true` maps to a gated `settled` request, `false`/unset maps to `paid`. It can never select `shadow`. |

`shadow` is **hard-rejected** at startup until T4.4 delivers the observe-only
dual consumer (Director ruling 2026-11-25). Do not write shadow approval rows
expecting the pod to run: a valid gate still exits 4 by design. `paid` remains
the only production default.

### Startup exit codes and reason tokens

| Exit | Meaning | Operator action |
|---|---|---|
| `1` | Infrastructure failure (DB pool, NATS, gRPC dial, tracing, run) | Fix the dependency; this is not a switch rejection. |
| `2` | Config rejection (unknown mode, N5, range missing, legacy conflict) | The error text carries a stable token: `MODE_UNKNOWN`, `N5_APPROVAL_ID_MODE_MISMATCH`, `RELEASE_RANGE_MISSING`; legacy/new conflict keeps its own text. |
| `3` | Approval-row gate rejection | Query `settlement_switch_approvals` and fix the row/range; reason is one of `APPROVAL_ROW_MISSING`, `APPROVAL_NOT_APPROVED`, `APPROVAL_EXPIRED`, `RELEASE_RANGE_MISMATCH`, `APPROVAL_DB_UNAVAILABLE`. |
| `4` | Shadow runtime rejection (`T4_4_SHADOW_NOT_DELIVERED`) | Expected until T4.4 closes; do not delete the guard. |

Exit 3/4 logs are JSON records containing `mode`, `approval_id` and `reason`
(the `error` string repeats `mode=... approval_id=... reason=...`), and they are
emitted **before** any consumer/worker is registered. Expiry is evaluated with
the database clock (`NOW(3)`), never the application host clock.

### Writing approval rows (controlled ops/Director action)

`release_range` is mandatory for both shadow and settled rows. Write a new row;
never `UPDATE` an audit row (revoke and re-issue instead). The application never
writes `status='EXPIRED'`; `EXPIRED` is not a reachable state in R0.4, so
dashboards and alerts must use `expires_at <= NOW(3)`, not `status`.

```sql
-- settled example (shadow rows keep both order.paid retirement columns NULL)
INSERT INTO settlement_switch_approvals
  (approval_id, target_mode, approved_by, approved_at, release_range, evidence_ref,
   expires_at, order_paid_admission_retired_at, order_paid_subject_removal_date, status)
VALUES
  ('settled-2026-12-01', 'settled', '<director>', NOW(3), 'prod', '<change/evidence ref>',
   DATE_ADD(NOW(3), INTERVAL 30 DAY), NOW(3), '2027-01-15', 'APPROVED');
```

After writing, both checks below must pass. The first must return **0 rows**;
a non-zero result blocks the deploy and the row must be re-issued by the
Director/ops (audit rows are immutable), with an incident ticket.

```sql
-- invariant: no APPROVED row may carry an empty release_range
SELECT approval_id FROM settlement_switch_approvals
 WHERE status = 'APPROVED' AND release_range = '';

-- sample sanity print for the row just written
SELECT approval_id, target_mode, release_range, status, expires_at
  FROM settlement_switch_approvals
 WHERE approval_id = 'settled-2026-12-01';
```

### Expiry alert and rollout stop

- Expiry is a **startup-window** property: a running pod does not stop when
  `expires_at` passes and there is no hot switch. The only stop path is a
  Director rollback decision plus a restart.
- Ops MUST alert on rows that have expired but are still `APPROVED`
  (owner=ops, expiry 2026-12-31, tracked in
  `review-tracking/PAY-SETTLE-01`):

```sql
SELECT approval_id, target_mode, expires_at
  FROM settlement_switch_approvals
 WHERE target_mode = 'settled' AND status = 'APPROVED' AND expires_at <= NOW(3);
```

- Record the alert rule in the on-call runbook with one real drill/trigger
  output pointer. A verbal "configured" is not acceptance.
- An instance still running on an expired approval is rolled back/restarted by
  the Director, not by the application.

### Migrating `order/0022` + `order/0023` (R0.4 checks, branch A)

Branch A (Director decision 2026-11-26) keeps `order/0022_r04_checks.sql`
**byte-identical** (blob `240b45f`, sha256 `8339df07…`) and moves every semantic
fix into `order/0023_r04_checks_v2.sql`:

| file | constraints | ordering |
|---|---|---|
| `0022` (frozen) | `chk_refund_app_group_discriminator` (copies the 0017 enum), `chk_lpr_expiry_24h` (lower bound `>=`) | applies first on a fresh DB |
| `0023` (replacement) | `chk_refund_app_group_discriminator_v2` (set membership, no enum copy), `chk_lpr_expiry_24h_v2` (exactly `entered_at + 24h`) | adds both `_v2` first, then drops the old names |

Never replay `0022` after `0023`: the runner ledger prevents it, and a manual
replay would reinstall the legacy `>=` / enum-copy checks.

#### DDL lock profile (F-1; measured on disposable MySQL 8.4.13)

| operation | explicit `ALGORITHM=INPLACE, LOCK=NONE` | actual behaviour |
|---|---|---|
| `ADD CHECK` | **rejected** (`ERROR 1845 ... Try ALGORITHM=COPY`) | `COPY`: table rebuild, `SHARED_NO_WRITE` MDL for the whole rebuild, writes blocked |
| `DROP CHECK` | **accepted** | online, metadata only |

Measured with 500,000 `refund_applications` rows + 200,000
`legacy_payment_resolution` rows on a disposable `mysql:8.4.13` (HEAD bound in
the R0.4-FIX selfcheck):

- `0022` apply (both ADDs, 500,000 / 200,000 rows): 56.5 s.
- `ADD CHECK ..., ALGORITHM=COPY, LOCK=SHARED` on `refund_applications`
  (500,000 rows): 45.0 s wall / 43.7 s statement; MDL `SHARED_NO_WRITE`
  GRANTED from +0.37 s to +43.4 s while a concurrent writer stayed
  `SHARED_WRITE PENDING` and its inserts blocked for 43.8 s (0 errors).
- `ADD CHECK ..., ALGORITHM=COPY, LOCK=SHARED` on `legacy_payment_resolution`
  (200,000 rows): 17.4 s wall / 16.0 s statement; the writer was blocked 16.1 s.
- `DROP CHECK ..., ALGORITHM=INPLACE, LOCK=NONE`: statement time 0.014-0.022 s,
  no blocked writer (max 54 ms), 0 errors.
- `0023` end-to-end on the same scale without a concurrent writer: 49.7 s; a
  replay is a no-op and leaves exactly the two `_v2` constraints.

**Consequence:** the two `ADD CHECK` statements of `0023` MUST run inside a
write-frozen maintenance window, not as an online migration. The two
`DROP CHECK` statements are online. The raw F-1 output is attached to the
R0.4-FIX selfcheck (【F-1】 section).

#### Preflight (read-only) — UAT is treated as prod

Director (2026-11-26) treats **UAT as production** for this migration's
preflight, evidence and lock/maintenance-window scope until further notice:
UAT MUST NOT receive the `ADD CHECK` DDL outside its maintenance window, and it
MUST NOT be used as the "we already applied it" proof for any other environment.

Before a window, run the read-only preflight on every persistent environment
(shared dev, UAT-as-prod, prod, any shared/durable pre-prod):

```sql
SELECT filename, checksum, applied_at FROM schema_migrations
 WHERE filename IN ('order/0022_r04_checks.sql','order/0023_r04_checks_v2.sql');
SELECT status, COUNT(*) FROM schema_migration_attempts
 WHERE filename IN ('order/0022_r04_checks.sql','order/0023_r04_checks_v2.sql')
 GROUP BY status;
```

Record environment, output rows/checksum, operator, time and HEAD, plus
`sha256sum migrations/order/0022_r04_checks.sql`. Branch A is the fail-closed
choice, so this preflight does not change the path; it tells ops which
environment still needs `0023` and whether any environment recorded a 0022
checksum other than `8339df…` (an interim branch-B apply). A different 0022
checksum is a data-integrity incident: do not edit the file; re-issue the
migration through the window and record it in `docs/review-tracking.md`.

#### Maintenance window (UAT-as-prod and production)

Owner: Developer + ops; expiry 2026-12-31.

1. Approve/announce the window. Measure the exact row counts first
   (`SELECT COUNT(*)` on both tables) and scale the estimate from the F-1
   numbers above; if the tables are much larger than the evidence run, extend
   the window instead of assuming online DDL (there is none for `ADD CHECK`).
2. Back up `mall_order` (`mysqldump --single-transaction`) and record its
   sha256 plus the current ledger rows.
3. Freeze Order writes for the window (scale the `order` deployment to zero or
   stop the writers; the `SHARED_NO_WRITE` MDL blocks all DML, so app writes
   during the rebuild would only queue and time out).
4. Run the prechecks from `0022`/`0023`; they must return 0 rows. A non-zero
   result means stop and repair the data with an audited write.
5. Run the migration job (`mall/migrate:dev`): `0022` on a fresh env, then
   `0023` (both COPY adds + the online drops).
6. Verify before resuming writes: the two `_v2` constraints exist, the old
   names are gone, `CHECK_CLAUSE` matches the equality/discriminator semantics
   (`scripts/verify-r01-migrations.sh`), the prechecks return 0 rows, and the
   ledger checksums match.
7. Resume writes and watch the error rate; the `ADD CHECK` validation is
   fail-closed (error 3819) for any row that still violates it.

Rollback / recovery (no down migration, ADR §10):

- Before `0023` commits: kill the job. A `COPY` `ADD CHECK` that is killed rolls
  back. Both v2 adds run before any old-name DROP, so a blocked second ADD
  leaves both legacy checks in force (and the v2 names are skipped on the next
  run). Use the failed-attempt flow below.
- After `0023` commits: restore the `mall_order` dump (or a fresh instance and
  repoint), or write a reviewed forward fix. Never `DROP` the v2 constraint or
  bulk-update rows just to make data fit; never edit `0022`.
- A wrong manual row is corrected by an audited write; the constraint stays.

#### Failed attempt recovery

`0022`/`0023` run against MySQL's implicit DDL commit: a dirty row or an
interruption can leave the first CHECK installed and the next missing while the
runner records `FAILED`/`RUNNING` in `schema_migration_attempts`; the runner
then refuses a replay. The `0022` header's "conditional ALTERs make a re-run
after cleanup safe" claim is superseded and MUST NOT be used.

1. Read the precheck output / error 3819 to find the blocking row(s). Do not
   rerun blindly.
2. Repair data with an audited write only (set the group field or
   `expires_at = DATE_ADD(entered_at, INTERVAL 24 HOUR)`); never drop the
   constraint or bulk-update to hide it.
3. After operator sign-off delete the attempt row for the filename
   (`status IN ('FAILED','RUNNING')`).
4. Re-run the migration job (`0023` converges: the v2 adds are skipped and the
   old names are dropped).
5. Verify both v2 `CHECK_CLAUSE` values and a zero precheck count.
   `scripts/verify-r01-migrations.sh` performs these semantic assertions on a
   real MySQL 8.4, including the partial "v2 added, old not dropped" state.

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
make fault-split-uat # UAT only: Order/NATS/Fulfillment outage recovery
make fault-split-worker-uat # UAT only: after-sales worker restart recovery
make alerts-check # structural check for after-sale backlog rules
# UAT install/validation: docs/runbooks/uat-environment.md
make concurrency-phase15 # opt-in one-unit oversell race check
make create-split-fixture # self-seeding two-SKU split fixture (admin API)
make test-integration # per-package DBs derived from TEST_MYSQL_DSN
# base DSN must contain /mall_integration_test; target uses
# mall_integration_<package> via scripts/test-dsn.sh
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

## Split/after-sale observability

The after-sales worker exports:

- `mall_aftersale_restock_backlog`
- `mall_aftersale_restock_oldest_seconds`
- `mall_aftersale_refund_backlog`
- `mall_aftersale_refund_oldest_seconds`

Alert when backlog is non-zero for longer than the restock/refund SLO or when
the oldest task age grows without draining. The four rules live in
`deploy/compose/observability/alerts.yml` and are structurally checked by
`make alerts-check`; it runs `promtool test rules` in a disposable Prometheus
image to prove firing at the configured durations. These are durable task queues, not status scanners.

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

UAT/dev use the single `WAREHOUSE_MODE=split` switch; the legacy
`WAREHOUSE_SPLIT_ENABLED`/`WAREHOUSE_ALLOCATION_SHADOW` pair remains supported
for older values files. The production values keep split off.
Phase 4/5/6 now have per-group shipments, permits, returns/refunds and
group-aware storefront/admin UI. The dev single-node values enable split; the
production defaults keep split off because there is no large-scale production
rollout. The [migration/rollback runbook](runbooks/c-migration-rollback.md)
remains the gate for a future production change window. The disposable dev rehearsal is
`make rehearse-c-migration`; it uses the pre-C dumps in `/tmp/mall-predeploy`
by default.

The AI RAG build is not a release gate by default. `ai-rag-build-scheduled`
runs as a CronJob; `ai.ragBuild.hook=true` opts into the old blocking
post-upgrade Job. Trigger a build manually with:

```bash
kubectl -n mall create job --from=cronjob/ai-rag-build-scheduled ai-rag-build-manual
```

## Refund review queue (AC-72 backend segment)

> Owner: ops (queue/SLA) + Developer (Order data checks) + AfterSales (task
> mirror) + Fulfillment (S6 evidence). Expiry 2026-12-31. The 5-minute alert
> rule and the end-user entry are **not** part of this runbook:
> `REVIEW-REQUIRED-ALERTS-01` (deploy) and `REVIEW-REQUIRED-ENTRY-01`
> (frontend/BFF) own them; until they land, keep the existing "退款处理中" copy.

A persisted refund review is `order_refund_admissions.result='REVIEW_REQUIRED'`
with `state='PENDING_REMOTE'`. It is **not** an admission result and **not** a
terminal state: the permit stays `CONSUMED`, the commitment stays `RESERVED`,
no Payment command ran, and the group is not closed. Zero-amount conflicts are
**not** persisted and never appear in these queries.

### Discovery and SLA

```sql
-- backlog + oldest (the two AC-72 gauges; per source/reason via logs)
SELECT COUNT(*) AS backlog,
       COALESCE(TIMESTAMPDIFF(SECOND, MIN(updated_at), NOW(3)), 0) AS oldest_seconds
  FROM order_refund_admissions
 WHERE result = 'REVIEW_REQUIRED' AND state = 'PENDING_REMOTE';

-- 7-day overdue rows (must be zero; non-zero is an ops blocker for the daily report)
SELECT refund_id, order_no, group_no, updated_at
  FROM order_refund_admissions
 WHERE result = 'REVIEW_REQUIRED' AND state = 'PENDING_REMOTE'
   AND updated_at < DATE_SUB(NOW(3), INTERVAL 7 DAY);

-- per-item detail for the manual queue
SELECT refund_id, order_no, group_no, amount_cent, version, created_at, updated_at
  FROM order_refund_admissions
 WHERE result = 'REVIEW_REQUIRED' AND state = 'PENDING_REMOTE'
 ORDER BY updated_at;

-- TCC batches held for review (reason is the canonical last_error)
SELECT batch_id, order_no, payment_no, state, last_error, updated_at
  FROM refund_batches WHERE state = 'MANUAL_HOLD' ORDER BY updated_at;

-- after-sales mirror (ordinary after-sale / legacy; no refund_applications row)
SELECT after_sale_no, status, attempts, last_error, updated_at
  FROM after_sale_refund_tasks
 WHERE last_error LIKE 'REVIEW_REQUIRED reason=%' ORDER BY updated_at;
```

SLA: first response within **24 hours**, disposition within **7 calendar
days**; overdue items escalate to finance/ops leads and stay open. Never
auto-close, auto-refund, auto-release or auto-promote to `FAILED_PERMANENT`.

### Re-arm (AC-27-NOWB-01 terminal ① / AC-69)

Only ops, only with a real S6 cancellation evidence (existing or previously
existed, recovered by Fulfillment), an authorization, and an ops ticket
(`ticket=RVW:<refund_id>`). **Audit first, clear last** — never clear before the
mirror/log/ticket exist, and never clear `last_error` as the disposition:

1. Open the ops ticket with the actor, authorizer, evidence, time, amount and
   `RVW:<refund_id>`; confirm the mirror `last_error` is the canonical
   `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>` (or the D-35
   truncated form). Do not edit/rewrite `last_error`.
2. Write the structured log `review_rearm` (refund_id/order_no/group_no/
   from/to/reason/ticket/version).
3. Versioned CAS on the authoritative row only (keep `state`):
   ```sql
   UPDATE order_refund_admissions
      SET result = 'PENDING_REMOTE', version = version + 1, updated_at = NOW(3)
    WHERE refund_id = ? AND result = 'REVIEW_REQUIRED'
      AND state = 'PENDING_REMOTE' AND version = ?;   -- require RowsAffected=1
   ```
   TCC: also move the held batch, **keeping** the canonical `last_error`:
   ```sql
   UPDATE refund_batches
      SET state = 'ADMITTING', version = version + 1, updated_at = NOW(3)
    WHERE batch_id = ? AND state = 'MANUAL_HOLD' AND version = ?; -- RowsAffected=1
   ```
4. Replay the original `refund_id` (Order admission) and observe: real
   `CANCELLED` → `PRE_SHIP_REVOKED`, `SHIPPED` → `POST_SHIP_ALLOWED`, still no
   waybill → review again (the row stays). The admission row is never deleted;
   `created_at` is the first-review time, `version` is monotonic.
5. A user who changes to a whole-group refund closes the old review only through
   the AC-62/D-24 authorized release and a **new** `refund_id`; never reuse or
   edit the old decision.

### Legacy PRE audit (before claiming AC-27A closure)

```sql
SELECT a.refund_id, a.order_no, a.group_no, a.result, a.state,
       p.state AS permit_state, g.status AS group_status
  FROM order_refund_admissions a
  LEFT JOIN order_shipping_permits p
         ON p.order_no = a.order_no AND p.fulfillment_group_no = a.group_no
  LEFT JOIN order_fulfillment_groups g
         ON g.order_no = a.order_no AND g.group_no = a.group_no
 WHERE a.result = 'PRE_SHIP_REVOKED'
   AND (a.state = 'PRE_SHIP_REVOKED' OR a.state = 'PENDING_REMOTE');
```

List the rows with `g.group_no IS NULL`, `g.status IN ('SHIPPED','RECEIVED')`,
`a.state='PENDING_REMOTE'` (unverifiable) and `p.state IN ('REVOKED','CONSUMED')`.
**0 rows does not prove safety**; the audit result must be recorded before any
AC-27A close/AC-VERIFIED claim for the affected groups (owner ops + Developer).

### Reconciliation (daily)

```sql
-- review rows vs the manual queue / reason distribution (zero-amount excluded)
SELECT COUNT(*) FROM order_refund_admissions
 WHERE result = 'REVIEW_REQUIRED' AND state = 'PENDING_REMOTE';
SELECT batch_id, last_error FROM refund_batches WHERE state = 'MANUAL_HOLD';
```

`REVIEW_REQUIRED` must never be counted as refunded/closed/released/completed.
Alert rules and the 5-minute delivery are owned by `REVIEW-REQUIRED-ALERTS-01`
(ops, expiry 2026-12-31); this runbook only provides the queries and the
backend gauge values (`mall_refund_review_backlog`,
`mall_refund_review_oldest_seconds`) that the alerts consume.

## Manual ops runbook (PAY-OPS-REPLAY-01: DLQ replay + NOWB re-arm/abandon)

> **Human residual signing (2026-11-27 C+A waiver, expiry 2026-12-31).**
> This procedure has **no cryptographic non-repudiation and no ops tool**:
> `cmd/ops`, `0024`, an authorization table and new admin RPCs are withdrawn.
> Compensating controls are personal named DBA accounts, dual control, a
> change ticket, DB audit logs and independent Reviewer verification. Do not
> claim AC-62 “technical closure”, “the tool exists” or non-repudiation.
> Waiver scope is `AC-27-NOWB-01` / `AC-27-LIVE-01` only; **AC-27A stays
> AC-REF**, and AC-61/62 are not fully verified by this runbook. AC-27 may be
> marked AC-VERIFIED only after the D1–D11 drill in
> `scripts/drill-ops-replay.sh` plus the Reviewer's evidence.
>
> Nothing here authorizes automatic re-arm, automatic release, automatic
> group closure, permit restore or a second refund.

### Roles and change authorization

| Field | Rule |
|---|---|
| `authorization_id` | Change-ticket id. **Not** `ticket=RVW:<id>`; ticket alone is never authorization. |
| `action` | `REPLAY`, `NOWB_REARM`, `NOWB_ABANDON`. |
| identifiers | `refund_id` / `after_sale_no` / `order_no` / `group_no` / `batch_id` where applicable; all must match the located rows. |
| `amount_cent` | Positive integer. `>= 100000` (¥1000) requires a second reviewer. |
| `operator` | Named DBA/ops admin user id (never a shared account). |
| `authorizer` | Business admin; **different person from operator**. |
| `reviewer` | Second reviewer for `>= 100000`; **different from authorizer** (and operator). |
| `reason_code` | Structured reason (e.g. `CONSUMED_WITHOUT_SHIPMENT`). |
| `evidence_ref` | S6 evidence (see re-arm); required for `NOWB_REARM`. |
| `authorized_at` / `expires_at` | `expires_at` is **≤ 24h** after `authorized_at`; expired → reject. |
| `ticket` | Must equal `RVW:<refund_id>`. |
| `owner_approval_ref` | Required for `NOWB_ABANDON` (SPEC-0002 owner AC-62/D-24 written approval). |
| `op_id` | Unique manual operation id; one change ticket, one executor. |

Any missing/expired/unauthorized/same-person field → **stop, zero writes**. The
Reviewer verifies the original ticket/approval source and the identity admin
role snapshot; the ticket-reported fields are only auditable, not technically
verified.

### D1/D2 — locate and replay `aftersale.completed` (AC-61 / AC-27-LIVE-01)

Replay only re-drives an unfinished projection; it never redoes a refund.

1. Locate with at least one key (`after_sale_no`, `stream_seq`, `refund_id`,
   `batch_id`/`payment_no`); if the key is not unique, stop.
   ```bash
   nats stream info MALL                      # 7-day retention
   nats consumer info MALL fulfillment-refund-admitted
   ```
   DLQ subject: `shop.aftersale.completed.v1.dlq`; wrapper is
   `{subject,data,error,attempts,failed_at}`.
2. Read-only precheck: DLQ `stream_seq/subject/attempts/error`; Order
   `order_refund_admissions.result/state/version`; AfterSales
   `after_sale_refund_tasks.status/attempts/lease_*`; newest
   `after_sale_logs.action`. `FAILED_PERMANENT` →
   **`PERMANENT_REQUIRES_NEW_REFUND`**, never replay. `PENDING/PROCESSING`
   with an expired lease → lease recovery (D3), not event replay. No DLQ
   message / older than 7 days → stop; do not reconstruct from memory.
   ```sql
   SELECT refund_id, order_no, group_no, result, state, version
     FROM order_refund_admissions WHERE refund_id = '<refund_id>';
   SELECT after_sale_no, status, attempts, lease_owner, lease_until, last_error
     FROM after_sale_refund_tasks WHERE after_sale_no = '<after_sale_no>';
   ```
3. Publish the original `data` bytes unchanged (never to `.dlq`):
   ```bash
   nats pub shop.aftersale.completed.v1 \
     --header "Nats-Msg-Id: manual:<authorization_id>:<stream_seq>" \
     --file <original-data.json>
   ```
4. The same `Nats-Msg-Id` is deduped by JetStream; re-publishing the same
   `op_id` is safe and `applyAfterSaleCompleted` is idempotent. Observe the
   Order projection; never write PRE/close/pay from this step.
5. Audit: change ticket + structured ops log with
   `op_id/authorization_id/stream_seq/subject/ticket`; if a task log row is
   writable, `after_sale_logs('OPS_REPLAY', remark≤512)`. Reviewer compares
   before/after snapshots.
6. Failure: missing message → stop; publish failure → retry the same `op_id`
   (no DB edits); still not converged → escalate to NOWB, do not fake closure.

### D3 — lease lost / unfinished task (AC-61)

The normal recovery is the worker tick (it re-claims `PENDING`/expired
`PROCESSING`); prefer restarting/confirming the worker. Manual fallback only
under the same authorization: lock the task, require
`status IN ('PENDING','PROCESSING')` and a free/expired lease, then

```sql
START TRANSACTION;
SELECT status, attempts, lease_owner, lease_until
  FROM after_sale_refund_tasks WHERE after_sale_no = '<after_sale_no>' FOR UPDATE;
UPDATE after_sale_refund_tasks
   SET status = 'PENDING', attempts = 0, lease_owner = '', lease_until = NULL
 WHERE after_sale_no = '<after_sale_no>'
   AND status IN ('PENDING','PROCESSING')
   AND (lease_until IS NULL OR lease_until < NOW(3));
COMMIT;
```

Do not touch the admission/amount; keep `last_error` for audit (the worker
clears it on success). `FAILED_PERMANENT` tasks are never replayed; use a new
`refund_id` under AC-62. REVIEW tasks take the NOWB path below.

### D5/D5b/D6/D10 — NOWB re-arm (AC-27-NOWB-01 terminal ①)

**S6 evidence first.** Accept only Fulfillment-owned records for the exact
`order_no` **and** `group_no`: a `shipments` row with `status='CANCELLED'`, a
`shipment_events` CANCELLED row whose shipment belongs to the order, or an
archived `shipment.cancelled`/carrier cancellation receipt. The Fulfillment
authorizer signs the `order_no+group_no` evidence on the ticket and archives
its id/query output; `evidence_ref` must name it. A `shipments` row alone is
not group proof, and “it existed but was lost” is accepted only from those
archives/events/receipts. No record → **stop and go to abandon**; never create
a shipment row/event to manufacture evidence. A group mismatch (D5b/D10b) or
a forged `authorization_id` → reject, zero writes.

**Audit first, then two/three CAS.** Log the structured `review_rearm` event
(authorization/op/ticket/refund/order/group/version/evidence/operator/
authorizer/reviewer) before executing. Then, in one Order transaction:

```sql
SELECT result, state, version FROM order_refund_admissions
 WHERE refund_id = '<refund_id>' FOR UPDATE;   -- must be REVIEW_REQUIRED/PENDING_REMOTE
UPDATE order_refund_admissions
   SET result = 'PENDING_REMOTE', version = version + 1, updated_at = NOW(3)
 WHERE refund_id = '<refund_id>'
   AND result = 'REVIEW_REQUIRED' AND state = 'PENDING_REMOTE'
   AND version = <version_before>;               -- RowsAffected must be 1

SELECT state, version, last_error FROM refund_batches
 WHERE batch_id = '<batch_id>' FOR UPDATE;      -- must be MANUAL_HOLD
UPDATE refund_batches
   SET state = 'ADMITTING', version = version + 1, updated_at = NOW(3)
 WHERE batch_id = '<batch_id>' AND state = 'MANUAL_HOLD'
   AND version = <batch_version_before>;         -- RowsAffected must be 1
```

Keep `refund_batches.last_error` canonical; keep
`refund_applications.admission_result='REVIEW_REQUIRED'` (waiting for
re-evaluation) and do not create rows. With an AfterSales task, in one
AfterSales transaction lock the task (must be `PENDING`/`PROCESSING` with a
free/expired lease; `FAILED_PERMANENT` → reject), then
`status='PENDING', attempts=0, lease_owner='', lease_until=NULL` while
**keeping** the canonical `last_error`, and insert
`after_sale_logs('OPS_NOWB_REARM', remark≤512)`. The task table has **no
`version` column**; its third dimension is the row lock on
`status+lease_owner+lease_until+attempts` — never invent a version.

Drive the review again only through the existing entry: a task wakes the
worker (5s tick), a TCC batch uses the existing `TccCancelMarkPaid`/DTM retry
(no new RPC). No task and no application → `NO_REPLAY_DRIVER`; stop, never
guess the source (`order_refund_admissions` has no source column). Still no
waybill → `REVIEW_REQUIRED` again (`MANUAL_HOLD` batch, canonical task
`last_error`); never write PRE/pay. Real `CANCELLED` converges to
`PRE_SHIP_REVOKED`; real `SHIPPED` converges to `POST_SHIP_ALLOWED` (D6):
a re-armed shipped group is never “PRE”.
Crash recovery: if Order committed but the task was not reset, re-run and
only do the task; if the task was reset but the worker did not run, wait or
restart it; if TCC is `ADMITTING` but not driven, re-run the existing TCC
entry; never repeat a CAS.

### D7/D7b/D11 — NOWB abandon (AC-27-NOWB-01 terminal ②)

This is **not** an AC-62 release: it stops automatic retries and leaves the
liability, releases nothing, restores no permit, closes no group, writes no
`DONE` and pays nothing.

Ordinary after-sale (has task), one AfterSales transaction:

```sql
SELECT status, attempts, lease_owner, lease_until, last_error
  FROM after_sale_refund_tasks WHERE after_sale_no = '<after_sale_no>' FOR UPDATE;
UPDATE after_sale_refund_tasks
   SET status = 'FAILED_PERMANENT', lease_owner = '', lease_until = NULL
 WHERE after_sale_no = '<after_sale_no>'
   AND status IN ('PENDING','PROCESSING')
   AND (lease_until IS NULL OR lease_until < NOW(3));
-- last_error stays canonical: REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>
INSERT INTO after_sale_logs (after_sale_id, action, from_status, to_status, remark)
SELECT id, 'OPS_NOWB_ABANDON', 'PENDING', 'FAILED_PERMANENT',
       'authorization=<id> owner=<owner_approval_ref> ticket=RVW:<refund_id>'
  FROM after_sales WHERE after_sale_no = '<after_sale_no>' LIMIT 1;
COMMIT;
```

Never call the automatic permanent-failure path (it releases pending
amount/quota), never edit `after_sale_amount_ledger`/`amount_state`.

TCC (has `refund_applications`), one Order transaction:

```sql
UPDATE refund_applications
   SET state = 'FAILED_PERMANENT', version = version + 1, updated_at = NOW(3)
 WHERE refund_id = '<refund_id>' AND state = 'PENDING' AND version = <version_before>;
```

The batch stays `MANUAL_HOLD` with `last_error`; admission stays
`REVIEW_REQUIRED/PENDING_REMOTE`; commitment stays `RESERVED`; permit keeps
its value; the group is not closed. A normal TCC retry is rejected by
`ensureTccApplicationTx` (`TCC_APP_ABANDONED`).
Legacy with no task/application → `NO_TERMINAL_CARRIER`; do not create rows.

**User initiates Refund again (D7b/D11).** There is no code guard: an explicit
user `Refund` is a **new request** and may reopen a task. Ops must intercept it
via the ticket/named-account procedure and route it through a **new** §5/§6
review with a new/again-authorized change ticket; never auto-retry or
auto-close, and never treat the abandon as “refunded/closed”. Abandon leaves
the admission in REVIEW, so it still counts in the AC-72 backlog (the
liability is real).

### D8 — authorization and evidence rejections

Reject with zero writes when: `authorization_id`/`op_id` missing; operator ==
authorizer; `amount_cent >= 100000` without a reviewer or reviewer ==
authorizer; `expires_at` in the past or >24h after `authorized_at`;
`ticket != RVW:<refund_id>`; abandon without `owner_approval_ref`; re-arm
without an S6 `evidence_ref` for the exact `order_no+group_no`; S6 not
`CANCELLED` (or `SHIPPED` for the D6 POST mapping); forged/unverifiable
authorization id. Record the rejection reason in the ticket; run the SQL
snapshot before/after and show no row changed. The Reviewer must inspect the
original authorization/owner document — ticket-reported ids are audit-only.

### Audit, failure, rollback, forbidden

- Audit: change ticket + structured ops log + `after_sale_logs` (when a task
  exists) + Order admission/batch `version`/timestamps + DBA session log. The
  admission row is never deleted; `created_at`/`version` are the trace.
- Failure in production: stop, keep the current state and alert. Do not
  auto-reverse a committed CAS, do not clear `last_error`, do not delete
  admissions, and never restore a permit or release quota to “undo”. Forward
  fix under a new ticket / new `refund_id`, or abandon under this runbook.
- Concurrency/crash: one ticket, one executor; duplicate replay uses the same
  `op_id`/`Nats-Msg-Id`; CAS losers read back and stop; after a crash re-run
  only the missing step.
- Forbidden: `cmd/ops`, `0024`, an authorization table, new admin RPCs, new
  migrations/workers/automation; automatic re-arm/release/closure; treating
  abandon as release; restoring permits; writing PRE/POST for a rejected
  review; forging S6 records; using `ticket=RVW` as authorization; shared DBA
  accounts; marking AC-VERIFIED without the drill + Reviewer evidence.

### D1–D11 drill acceptance

Run `./scripts/drill-ops-replay.sh` (disposable MySQL 8.4 + NATS 2.10
JetStream + real migrations; never CI/production). It asserts D1–D11,
including D5b/D10b wrong-group evidence, D7/D11 re-Refund non-reopen, D8
forged/expired/same-person authorization, D9 concurrent CAS and the final
invariants (0 duplicate refund rows, 0 release events, permit unchanged,
`refunded_cent` unchanged). Exit 0 = all checks passed; any non-zero exit
means stop and do not use this runbook in production until re-reviewed.
