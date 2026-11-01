# gin-kratos-gorm

A medium-complexity, production-oriented microservice sample built with
[Kratos v2](https://github.com/go-kratos/kratos), Gin, GORM, Wire, JWT and
Casbin. It ships as a modular monolith plus optional split services
(`order`, `product`), and **does not require DTM for local development or
integration tests**.

> Original demo defects and the design decisions behind this rewrite are
> documented in [docs/ANALYSIS.md](docs/ANALYSIS.md).

## Features

- **Domain-driven layout**: `biz` (domain/use cases) → `data` (repositories) →
  `service` (API adapters) → `server` (HTTP/gRPC transports).
- **Order lifecycle**: create, quote/pricing, pay, cancel, list/detail,
  idempotent creation (`Idempotency-Key`), automatic cancellation of expired
  unpaid orders.
- **Stock safety**: conditional stock deduction, deterministic stock logs for
  idempotency, idempotent compensation/restore.
- **Transactional outbox**: domain events (`order.created/paid/cancelled`) are
  written in the same transaction as the state change and dispatched by a
  background worker with retry/backoff.
- **Pluggable distributed transaction strategy**:
  - `local` (default): a single DB transaction in the monolith, or a local
    compensation saga for split deployments. No DTM/etcd/MySQL needed.
  - `dtm` (optional): remote DTM saga for split deployments, with a startup
    health probe and optional fallback to local mode.
- **AuthN/AuthZ**: bcrypt password hashing, configurable JWT secret/TTL,
  role-aware claims, Casbin RBAC with seeded policies, route-level admin guards.
- **Observability**: request IDs, structured access logs, health/readiness
  endpoints, Prometheus-text `/metrics` (no extra dependency).
- **Config**: file + environment overrides, validation, no hard-coded VM IPs or
  JWT secrets.
- **Databases**: MySQL for production, SQLite for local development and tests.
- **Tests**: unit tests for pricing/saga/state transitions and SQLite-backed
  integration tests for repositories and HTTP APIs.

## Architecture

```
cmd/server    modular monolith: user + product + order + background worker
cmd/order     split order service (local compensation saga or optional DTM)
cmd/product   split product service (stock mutations + product catalog)

internal/
  biz/        domain models, use cases, repository/catalog interfaces, saga
  data/       GORM repositories, db bootstrap/seed, casbin, outbox dispatcher
  service/    proto adapters (request validation + DTO mapping)
  server/     Gin HTTP, Kratos gRPC, middleware, handlers, routers, worker
  pkg/        config, auth, errors, httpx, idgen, metrics, registry, grpcclient
  conf/       protobuf configuration
api/          protobuf APIs (user, product, order)
migrations/   MySQL schema + development seed
```

### Order creation paths

| Deployment | Transaction strategy | Stock | Order |
|---|---|---|---|
| Monolith (`cmd/server`) | `TxSaga` | same DB transaction | same DB transaction |
| Split (`cmd/order`, `local`) | `CompensationSaga` | gRPC `ReduceStock` | order DB; compensation on failure |
| Split (`cmd/order`, `dtm`) | DTM saga | DTM branch + barrier | DTM inner branch |

`order.created/paid/cancelled` events are always written to the outbox inside
the order transaction.

## Quick start (no external infrastructure)

```bash
# Go 1.26+
make run-local
```

This uses `configs/config.local.yaml`:

- SQLite file `./dev.db`
- `data.transaction.mode: local`
- Auto-migration and seed data (admin/demo accounts + two products)
- HTTP `:8000`, gRPC `:9000`

Health check:

```bash
curl http://127.0.0.1:8000/healthz
curl http://127.0.0.1:8000/readyz
curl http://127.0.0.1:8000/metrics
```

### Demo accounts

| Username | Password | Role |
|---|---|---|
| `admin` | `admin123` | admin |
| `demo` | `user123` | user |

Change these via `SEED_ADMIN_PASSWORD` / `SEED_USER_PASSWORD`, and never use
them in production.

## API examples

```bash
# Login
TOKEN=$(curl -s -X POST http://127.0.0.1:8000/api/base/login \
  -H 'Content-Type: application/json' \
  -d '{"name":"demo","password":"user123"}' | jq -r .token)

# Product catalog (public)
curl 'http://127.0.0.1:8000/api/products?page=1&page_size=10&keyword=Go'

# Create an order (idempotent)
curl -X POST http://127.0.0.1:8000/api/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-1' \
  -d '{
    "receiver_name":"张三",
    "receiver_phone":"13800000000",
    "receiver_address":"上海市",
    "items":[{"sku_id":1,"num":2}]
  }'

# Detail / pay / cancel
curl http://127.0.0.1:8000/api/orders/<order_sn> -H "Authorization: Bearer $TOKEN"
curl -X POST http://127.0.0.1:8000/api/orders/<order_sn>/pay -H "Authorization: Bearer $TOKEN"
curl -X POST http://127.0.0.1:8000/api/orders/<order_sn>/cancel \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"reason":"changed mind"}'
```

Admin endpoints: `POST /api/products`, `PUT /api/products/:id`,
`PUT /api/products/:id/status`, `PUT /api/products/:id/stock`,
`POST /api/users`, `GET /api/users/:id`, `PUT /api/users/:id/role`.

Errors use a stable envelope:

```json
{"code":"FAILED_PRECONDITION","message":"insufficient stock","request_id":"..."}
```

## Configuration

Configuration is validated at startup (`internal/pkg/config`). Common
environment overrides:

| Variable | Purpose |
|---|---|
| `APP_ENV` | `dev` / `staging` / `prod` |
| `HTTP_ADDR`, `GRPC_ADDR` | listen addresses |
| `DATABASE_DRIVER`, `DATABASE_SOURCE` | `mysql` or `sqlite` + DSN/path |
| `DATABASE_AUTO_MIGRATE`, `DATABASE_SEED` | dev-only schema/seed |
| `JWT_SECRET`, `JWT_ISSUER`, `JWT_TTL` | auth settings (secret ≥ 16 chars) |
| `TRANSACTION_MODE` | `local` or `dtm` |
| `DTM_ADDRESS`, `DTM_ENABLED`, `DTM_FALLBACK_TO_LOCAL` | optional DTM |
| `ETCD_ENDPOINTS` | comma-separated etcd endpoints (split/discovery) |
| `PRODUCT_ENDPOINT` | order service → product endpoint (`host:port` or `discovery:///product`) |
| `ORDER_PAYMENT_TIMEOUT`, `ORDER_SHIPPING_FEE` | order rules |

Files:

- `configs/config.local.yaml` — SQLite, local mode (no infra)
- `configs/config.yaml` — alias of local config
- `configs/server.yaml` — monolith with MySQL + etcd
- `configs/order.yaml` / `configs/product.yaml` — split services (SQLite, local)
- `configs/order.dtm.yaml` — split order service with DTM enabled

## DTM note (VM/Docker connectivity)

The original demo hard-failed when DTM was unreachable because it used
`dtmgrpc.MustGenGid` (panic) and had no fallback. This project:

- defaults to `transaction.mode: local`, so **no DTM process is needed**;
- generates a deterministic GID from the order number (`order-<order_sn>`)
  instead of calling DTM just to generate one;
- probes direct DTM addresses at startup and, when
  `data.transaction.fallback_to_local: true`, falls back to local mode with a
  warning instead of crashing;
- never silently falls back after a failed Submit (that could double-charge);
  it returns `UNAVAILABLE` and lets the caller retry;
- keeps DTM completely out of unit/integration tests.

If DTM runs in a VM Docker network that this host cannot reach, keep
`transaction.mode: local` (recommended) or fix routing and use
`configs/order.dtm.yaml`. Discovery-based DTM addresses (`discovery:///dtmservice`)
are not probed at startup because they require etcd resolution.

## Split services

```bash
# Terminal 1: product service on :8002/:9002
make run-product

# Terminal 2: order service on :8001/:9001 (calls product via 127.0.0.1:9002)
make run-order
```

Both services use the same JWT secret as the monolith so a token issued by the
monolith can be used against the order service. In split mode, the order
service uses the compensation saga and records stock restoration requests in
its outbox.

## Tests

```bash
make test        # unit + SQLite integration tests
make test-race   # race detector
make vet
```

Tests never require MySQL, etcd, Redis or DTM. They cover:

- order pricing, shipping fee, duplicate SKU merging, insufficient stock,
  idempotency, pay/cancel/auto-cancel, owner checks;
- `TxSaga`/`CompensationSaga` behavior;
- stock deduction/restore idempotency and transaction rollback;
- outbox persistence/dispatch;
- HTTP auth, RBAC fallback, order lifecycle, validation and admin product CRUD;
- JWT/password, ID generation and metrics export.

## Database

For production MySQL:

```bash
mysql -u root -p -e 'CREATE DATABASE gin_kratos CHARACTER SET utf8mb4'
mysql -u root -p gin_kratos < migrations/001_init.sql
mysql -u root -p gin_kratos < migrations/002_seed.sql   # optional dev seed
```

Use `data.database.auto_migrate: false` and `seed: false` in production.

## Docker

```bash
# Infrastructure only (MySQL, Redis, etcd)
make compose-up

# Monolith in Docker (profile app)
docker compose -f docker/docker-compose.yml --profile app up --build
```

DTM is deliberately not in the default stack. Redis is provisioned for future
caching/rate-limiting work; the current sample does not require it.

## Code generation

```bash
make init     # install protoc plugins + wire
make api      # regenerate protobuf/gRPC/HTTP/OpenAPI
make config   # regenerate internal/conf
make wire     # regenerate Wire code
make build    # build ./bin/{server,order,product}
```

## Production checklist

- Replace `JWT_SECRET` with a random 32+ byte value; set `APP_ENV=prod`.
- Use MySQL and disable `auto_migrate`/`seed`; apply versioned migrations.
- Enable TLS at the edge; run gRPC with mTLS or network isolation. DTM internal
  endpoints (`CreateOrderInner`, `CancelOrder`, `ReduceStock`,
  `CompensateStock`) are intentionally unauthenticated and must not be exposed.
- Configure Redis/rate limiting for login if running multiple replicas (the
  included login limiter is process-local).
- Tune DB pool, timeouts, outbox interval and payment timeout.
- Add your alerting on `/metrics`, error rate and outbox backlog.
