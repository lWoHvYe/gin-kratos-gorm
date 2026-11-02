# gin-kratos-gorm

A production-oriented microservice mall written in Go 1.27: Kratos gRPC
services, Gin BFFs, MySQL via `database/sql` (no GORM), NATS JetStream
transactional outbox, etcd discovery and DTM sagas for order/payment flows.

## Architecture

```text
                       ┌───────────────┐        ┌───────────────┐
   HTTP :8080  ───────▶│   gateway     │        │     admin     │◀─────── HTTP :8081
                       │  (Gin BFF)    │        │  (Gin BFF)    │
                       └───────┬───────┘        └───────┬───────┘
                               │ gRPC / etcd discovery  │
        ┌──────────────────────┼────────────────────────┼─────────────────────┐
        ▼          ▼           ▼          ▼             ▼          ▼          ▼
   identity    catalog     inventory    cart        pricing      order      payment
   :9001       :9002       :9003        :9004       :9005        :9006      :9007
        │          │           │          │             │          │          │
        └──────────┴───────────┴──────────┴─────────────┴──────────┴──────────┘
                               one MySQL database per service
                               NATS JetStream outbox (shop.<domain>.<event>.v1)
                               DTM for order create + payment TCC

   fulfillment :9008, aftersales :9009
```

Every gRPC service uses the same layout (`cmd/<service>`, `internal/services/<service>`,
`migrations/<service>`), opens MySQL with `db.Open`, registers its generated
`mall.v1` service, publishes domain events through the transactional outbox and
exposes `/healthz`, `/readyz` and `/metrics` on its health port.

## Services and ports

| Service | gRPC | Health/metrics | Database |
|---|---|---|---|
| gateway | – | 8080 (HTTP BFF) | – |
| admin | – | 8081 (HTTP BFF) | `mall_admin` |
| identity | 9001 | 9101 | `mall_identity` |
| catalog | 9002 | 9102 | `mall_catalog` |
| inventory | 9003 | 9103 | `mall_inventory` |
| cart | 9004 | 9104 | `mall_cart` |
| pricing | 9005 | 9105 | `mall_pricing` |
| order | 9006 | 9106 | `mall_order` |
| payment | 9007 | 9107 | `mall_payment` |
| fulfillment | 9008 | 9108 | `mall_fulfillment` |
| aftersales | 9009 | 9109 | `mall_aftersales` |

Infrastructure: MySQL 8.4 `:3306`, Redis 7 `:6379`, NATS 2.10 + JetStream
`:4222` (`:8222` monitoring), etcd 3.5 `:2379`, DTM `:36789` (HTTP) /
`:36790` (gRPC). Optional Prometheus `:9090`, Grafana `:3000`, Tempo `:3200`.

## Quick start (Docker Compose)

Requirements: Docker with Compose v2.20+ (BuildKit).

```bash
cp deploy/compose/.env.example deploy/compose/.env

make run-infra            # MySQL (databases + migrations), Redis, NATS, etcd, DTM
make run-all              # build and start gateway, admin and all 9 services

# or start pieces individually:
make run-service SERVICE=identity
make run-gateway          # brings up its dependencies as well
make run-admin
```

Endpoints once the stack is up:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8081/healthz
curl http://localhost:8080/api/v1/products
```

The gateway/admin are the only application containers with host ports.
Everything else talks over Docker DNS (`mysql`, `nats`, `etcd`, `dtm`,
`identity`, `catalog`, …).

Optional profiles:

```bash
make run-observability    # Prometheus :9090 + Grafana :3000 (admin/admin)
make run-infra            # core stack only
# Traefik on :80, routes /api/v1 -> gateway and /api/admin/v1 -> admin
docker compose -f deploy/compose/infra.yml -f deploy/compose/services.yml \
  --profile ingress up -d traefik
# Tempo (OTLP :4317/:4318, UI :3200)
docker compose -f deploy/compose/infra.yml --profile tracing up -d tempo
```

`make db-reset` drops the named volumes and re-applies `00-databases.sql`,
`02-dtm.sql` and every `migrations/<service>/*.sql` file. See
[docs/ops.md](docs/ops.md) for operations, database lifecycle, troubleshooting
and observability details.

## Local development (Go only)

The services are plain Go binaries; run the infrastructure in Docker and the
service you are working on on the host:

```bash
make run-infra

export MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_identity?parseTime=true&loc=Local&charset=utf8mb4'
export ETCD_ENDPOINTS=127.0.0.1:2379 NATS_URL=nats://127.0.0.1:4222 DTM_ADDRESS=127.0.0.1:36790
export JWT_SECRET=dev-only-change-me-32-characters
export GRPC_ADDR=:9001 HTTP_ADDR=:9101

go run ./cmd/identity
```

The gateway falls back to `127.0.0.1` addresses when `ETCD_ENDPOINTS` is empty
(override any of them with `<SERVICE>_ADDR`, e.g. `ORDER_ADDR=127.0.0.1:9006`).

## DTM

DTM runs as a compose service backed by MySQL (`dtmphp/dtm:latest`, store
config in `deploy/compose/dtm/config.yml`). Its kratos driver registers DTM in
etcd and resolves the `discovery:///...` branch URLs used by the order service.

- Order create: saga `order-create:<order_no>` — inventory `Reserve` (per line),
  then `CreateOrderInner`; compensation releases stock and cancels the order.
- Order cancel: local order transition + durable `shop.order.cancelled.v1`
  outbox event; inventory and pricing consume it to release stock/unlock the
  coupon. This is reliable eventual compensation, not a DTM Saga.
- Payment confirm: DTM TCC (`payment-confirm:<payment_no>`) across order,
  inventory and pricing, with a direct idempotent `MarkPaid` fallback when
  `PAYMENT_TCC_ENABLED=false`.
- Refund/after-sale: `AfterSales.Refund` first commits `REFUNDING`, calls
  `Payment.RefundOrder`, then commits `REFUNDED`. Retrying is safe and resumes
  a stuck `REFUNDING` request.

Branch handlers write business rows and outbox events in the same `*sql.Tx`
through `dtm.Branch`. If DTM is unavailable the write paths return
`UNAVAILABLE`; there is no silent local-transaction fallback. The full design
is in [docs/dtm.md](docs/dtm.md).

Verify DTM:

```bash
curl http://localhost:36789/api/dtmsvr/version
curl 'http://localhost:36789/api/dtmsvr/query?gid=order-create:<order_no>'
```

## Service mesh

Kratos and a service mesh are not mutually exclusive, but they overlap:
Kratos already provides discovery, load balancing, middleware, retries,
circuit breaking and tracing; a mesh adds mTLS and traffic policy at the
infrastructure layer. This version deliberately does **not** include Istio or
Linkerd. If mTLS/canary/zero-trust becomes a requirement, add Linkerd first and
disable duplicate retry/timeout/tracing configuration in Kratos.

## Tests

Unit tests only; there are no integration tests and the test suite does not
need MySQL, NATS, etcd or DTM.

```bash
make test        # go test ./...
make test-race   # go test -race over services/platform
make verify      # fmt + vet + test + build + arch + DTM contract
make build       # build every service into ./bin
make fmt         # gofmt
make fmt-check   # fail if gofmt would change files
make vet         # go vet ./...
```

## Layout

```text
cmd/<service>/              service binaries (main.go)
internal/services/<service>/ gRPC implementations, repos, workers
internal/platform/          config, db, log, grpcx, runtime, registry,
                            auth, apperr, httpx, mq, outbox, dtm
api/proto/mall/v1/          contract (all services in one proto package)
api/gen/mall/v1/            generated Go stubs (committed)
migrations/<service>/       MySQL DDL per service
deploy/compose/             infra + services compose files, MySQL init, DTM
Dockerfile                  one multi-stage image, SERVICE build arg
docs/                       contracts, ownership, DTM and ops documentation
```

`api/gen` is committed (the `*.pb.go` ignore rule excludes it) so a clean
checkout builds without protoc. Regenerate it with the protoc command in
[docs/contracts.md](docs/contracts.md) whenever the proto changes.

Contracts and conventions: [docs/contracts.md](docs/contracts.md).
File ownership: [docs/ownership.md](docs/ownership.md).
