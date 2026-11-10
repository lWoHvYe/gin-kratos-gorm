# Implementation contracts

## Generated API

All services use the single generated package:

```go
import pb "gin-kratos-gorm/api/gen/mall/v1"
```

Generated code is committed under `api/gen`; regenerate and commit it whenever
the proto changes:

```bash
protoc --proto_path=api/proto --proto_path=third_party \
  --go_out=api/gen --go_opt=paths=source_relative \
  --go-grpc_out=api/gen --go-grpc_opt=paths=source_relative \
  api/proto/mall/v1/mall.proto
```

## Service layout

```text
cmd/<service>/main.go
internal/services/<service>/
  service.go     # gRPC implementation
  repo.go        # database/sql queries
  worker.go      # optional outbox / scheduled jobs
migrations/<service>/0001_init.sql
```

Each service must:

- open MySQL using `db.Open(cfg.MySQLDSN)`;
- create gRPC server using `grpcx.NewServer(logger)`;
- register its generated service;
- use `runtime.HealthServer(cfg, database, logger)` for internal health/metrics;
- use `runtime.NewApp(cfg, logger, grpcServer, healthServer)` and `app.Run()`;
- use `config.Load("<service>")`;
- use `log.New(cfg.Service, cfg.Env)`;
- use `apperr` for domain errors;
- use NATS/outbox for domain events.

Reference implementation: `internal/services/identity` and `cmd/identity`.

## Ports and env

| Service | gRPC | HTTP/health | DB |
|---|---|---|---|
| identity | 9001 | 9101 | mall_identity |
| catalog | 9002 | 9102 | mall_catalog |
| inventory | 9003 | 9103 | mall_inventory |
| cart | 9004 | 9104 | mall_cart |
| pricing | 9005 | 9105 | mall_pricing |
| order | 9006 | 9106 | mall_order |
| payment | 9007 | 9107 | mall_payment |
| fulfillment | 9008 | 9108 | mall_fulfillment |
| aftersales | 9009 | 9109 | mall_aftersales |
| gateway | - | 8080 | - |
| admin | - | 8081 | mall_admin |

Common env: `MYSQL_DSN`, `REDIS_ADDR`, `NATS_URL`, `ETCD_ENDPOINTS` (optional
configuration center only), `DTM_ADDRESS`, `JWT_SECRET`, `GRPC_ADDR`, `HTTP_ADDR`.
Service-to-service addresses are direct Kubernetes/Docker DNS names.
Payment/after-sales: `PAYMENT_TCC_ENABLED`, `ORDER_TCC_URL`, `INVENTORY_TCC_URL`, `PRICING_TCC_URL`, `INVENTORY_ADDR`, `PRICING_ADDR`, `PAYMENT_ADDR`.

## Database conventions

- MySQL 8.0. Transactional core uses `database/sql` +
  `go-sql-driver/mysql`; catalog read queries use GORM (`docs/architecture.md`).
- All money columns end with `_cent BIGINT`.
- All status columns are VARCHAR strings.
- Every table has `created_at/updated_at`; mutable aggregates have `version BIGINT`.
- Every service DB has `outbox_events`:
  ```sql
  CREATE TABLE IF NOT EXISTS outbox_events (
    id BIGINT NOT NULL AUTO_INCREMENT,
    event_id VARCHAR(64) NOT NULL,
    topic VARCHAR(128) NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    payload JSON NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    last_error VARCHAR(1024) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    published_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_outbox_event_id (event_id),
    KEY idx_outbox_pending (published_at, id)
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
  ```
- DTM branch services also need `dtm_barrier`; DTM creates it automatically, but include it in migration when possible.

## DTM branch rules

- Saga branch handlers must call `dtm.Branch(ctx, db, func(tx *sql.Tx) error { ... })`.
- Business writes and `outbox.Append(ctx, tx, ...)` must use the same `*sql.Tx`.
- A Saga branch invoked outside a DTM context must return `UNAVAILABLE`; do not open a separate transaction.
- Direct admin calls and TCC Confirm/Cancel are not DTM barrier calls; they use the service-local transaction helper and must be idempotent.
- Branch IDs must be deterministic from `order_no` / `reservation_no`.
- Use `gid = order-create:<order_no>`, `payment-confirm:<payment_no>`. `order-cancel` is not a DTM GID: order cancellation is local + `shop.order.cancelled.v1` outbox compensation.
- Payment-confirm TCC branch methods live in `order/tcc.go`, `inventory/tcc.go`
  and `pricing/tcc.go`; URLs are configurable with `ORDER_TCC_URL`,
  `INVENTORY_TCC_URL`, `PRICING_TCC_URL`.

## Events

Publish through NATS JetStream subjects `shop.<domain>.<event>.v1`.
Use `outbox.Append` inside the state-change transaction. The service worker runs
`outbox.Relay(ctx, db, mqClient, logger, time.Second)`.

Events for Phase 1:

- `shop.order.created.v1`
- `shop.order.paid.v1`
- `shop.order.allocated.v1`
- `shop.order.shipped.v1`
- `shop.order.completed.v1`
- `shop.order.cancelled.v1`
- `shop.order.refund_requested.v1`
- `shop.inventory.reserved.v1`
- `shop.inventory.confirmed.v1`
- `shop.inventory.released.v1`
- `shop.payment.succeeded.v1`
- `shop.payment.refunded.v1`
- `shop.shipment.created.v1`
- `shop.shipment.shipped.v1`
- `shop.shipment.delivered.v1`
- `shop.aftersale.completed.v1`

Consumers:

- inventory/pricing consume `shop.order.cancelled.v1` for durable compensation.
- order consumes shipment created/shipped/delivered to project `ALLOCATED` /
  `SHIPPED` / `COMPLETED`.
- payment consumes `shop.order.refund_requested.v1`.
- fulfillment consumes `shop.order.paid.v1` to create a shipment.

## API / errors

- Return `apperr.*` from services; `grpcx` maps to gRPC status.
- Validation happens in the service; reject empty IDs, negative quantities, invalid states.
- No panics in request paths.

## Testing

- Unit tests plus the opt-in Phase 1.5 E2E script (`make e2e-phase15`).
- Focus on money math, state transitions, reservation/refund idempotency.
- `go build ./cmd/<service>` and `go test ./internal/services/<service>/...` must pass.

## Event consumers

- `fulfillment` consumes `shop.order.paid.v1` and creates a `CREATED` shipment
  from the order items. The consumer is durable and idempotent by `order_no`.
- `order.paid` payload includes `items` (`sku_id`, `name`, `price_cent`,
  `quantity`, `discount_cent`).

## Phase 1.5 additions

- Catalog admin now covers category/brand CRUD, SPU/SKU CRUD, image and
  attribute lists, draft/publish status, and `GetAdminProduct`/
  `ListAdminProducts`.
- `skus.stock` is a display projection. Inventory owns the counters and emits
  `shop.inventory.stock.changed.v1`; catalog consumes it with a durable
  `catalog-stock-projection` consumer.
- C-end product search filters in MySQL by category subtree, brand, price and
  keyword, with `price_asc`/`price_desc`/`newest` ordering.
- Identity exposes `UpdateAddress`, `DeleteAddress`, `SetDefaultAddress`.
- Payment supports idempotent partial refunds, `captured_cent`, `refunded_cent`
  and `ListRefunds`. Mock channel only; no provider SDK.
- `order.refunded_cent` follows `shop.payment.refunded.v1`; partial/full refund
  statuses are projected asynchronously.
- After-sales supports `RETURN_REFUND`: approve, submit return tracking, receive
  the parcel, restock (`SELLABLE`) or record `DAMAGED`, then refund. Restock is
  idempotent by after-sale number.
- Fulfillment exposes `ListShipments` for the back office.
- Phase 1.5 validation is covered by `scripts/e2e-phase15.sh` against a running
  stack in addition to unit tests. It is intentionally not run by `make test`.

Events added:

- `shop.inventory.stock.changed.v1`
- `shop.aftersale.returning.v1`
- `shop.aftersale.received.v1`
- `shop.order.refund_projected.v1`

Consumer added:

- catalog `catalog-stock-projection`.
- order `order-refund-projection`.
