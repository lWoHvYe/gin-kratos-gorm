# Order service

Owns the order aggregate: creation, cancellation, payment confirmation and the
read models for customers and admin. MySQL access is plain `database/sql` with
`github.com/go-sql-driver/mysql` (no ORM). Cross-service consistency uses DTM
sagas.

- gRPC: `:9006` (`GRPC_ADDR`)
- health/metrics: `:9106` (`HTTP_ADDR`)
- database: `mall_order`
- generated API: `api/gen/mall/v1` (`pb.OrderServiceServer`)

## Files

```text
cmd/order/main.go
internal/services/order/service.go   # gRPC handlers, saga orchestration, helpers
internal/services/order/repo.go      # database/sql queries
internal/services/order/service_test.go
migrations/order/0001_init.sql
```

## Configuration

| Env | Default | Purpose |
|---|---|---|
| `MYSQL_DSN` | `root:root@tcp(127.0.0.1:3306)/mall?...` | must point at the `mall_order` schema |
| `NATS_URL` | `nats://127.0.0.1:4222` | JetStream for the outbox relay |
| `ETCD_ENDPOINTS` | `127.0.0.1:2379` | kratos registry + `discovery:///` resolver |
| `DTM_ADDRESS` | `127.0.0.1:36790` | DTM server used to submit sagas |
| `PRICING_ADDR` | `pricing:9005` | pricing gRPC target (`GetQuote`, `UnlockCoupon`, `ConsumeCoupon`) |
| `INVENTORY_ADDR` | `inventory:9003` | inventory gRPC target (`Release`, `Confirm`) |
| `INVENTORY_DTM_URL` | `discovery:///inventory/mall.v1.InventoryService` | DTM branch base URL (`+ /Reserve`, `/Release`) |
| `ORDER_DTM_URL` | `discovery:///order/mall.v1.OrderService` | DTM branch base URL (`+ /CreateOrderInner`, `/CancelOrderInner`) |
| `GRPC_ADDR` / `HTTP_ADDR` | `:9000` / `:9100` | set to `:9006` / `:9106` in compose |

`_ "github.com/dtm-labs/driver-kratos"` is imported in `cmd/order` so DTM can
parse `discovery:///` URLs. The service registers the kratos etcd resolver
(`discovery.NewBuilder`) for the `discovery` scheme so the same URLs also
resolve inside this process.

## Schema (`migrations/order/0001_init.sql`)

- `orders` – money in `_cent BIGINT`, `status` as `VARCHAR`, `version` for
  optimistic concurrency, `expire_at` for the payment window,
  `UNIQUE (order_no)` and `UNIQUE (user_id, client_request_id)`.
- `order_items` – item snapshot taken from the quote (name/price/quantity/discount).
- `order_status_logs` – append-only audit trail of every transition
  (`from_status`, `to_status`, `reason`, `operator`).
- `outbox_events` – transactional outbox used by the relay worker.
- `dtm_barrier.barrier` – saga barrier table (DTM also creates it lazily).

## Statuses

`PENDING_PAYMENT` → `PAID` → `SHIPPED` → `COMPLETED`, plus `CANCELLED` and
`CLOSED`. Only `PENDING_PAYMENT` orders can be paid or cancelled.

## Methods

### `CreateOrder`

1. Validates the request and trims receiver fields; a missing
   `client_request_id` is generated so `UNIQUE (user_id, client_request_id)`
   never collides on empty strings.
2. Idempotency: an existing row for `(user_id, client_request_id)` is returned
   as-is, without touching pricing or DTM.
3. `PRICING_ADDR` `GetQuote(quote_id)` supplies the lines and amounts
   (`total/discount/freight/pay`; the total falls back to the sum of lines and
   `pay = total - discount + freight` when pricing leaves them empty).
4. `order_no = id.Order()`.
5. DTM saga `gid = order-create:<order_no>`:
   - one `Reserve` action per quote line (`ReserveRequest{OrderNo, SkuId,
     WarehouseId: 1, Quantity, TtlSeconds: 900}`) with `Release` as
     compensation;
   - `CreateOrderInner` action carrying the full `CreateOrderInnerRequest`
     (order data + quote lines) with `CancelOrderInner` as compensation.
6. `Submit()` failure returns `UNAVAILABLE` (never a local-transaction
   fallback). Success re-reads the order by `order_no`; if a concurrent request
   won the idempotency key, that order is returned instead.

### `CreateOrderInner` (DTM branch)

Runs inside `dtm.Branch(ctx, db, func(tx *sql.Tx) error)`. Returns
`UNAVAILABLE` when the call has no DTM branch context. Idempotent by
`order_no`; inserts the `PENDING_PAYMENT` order, its items, the status log and
the `shop.order.created.v1` outbox event in the barrier transaction.

The idempotency key is forwarded through DTM saga branch headers
(`x-client-request-id`) because `CreateOrderInnerRequest` has no
`client_request_id` field; without the header the branch falls back to
`order_no`.

### `CancelOrder`

Owner (or `admin`) loads the order; `CANCELLED` is returned idempotently,
`PAID`/`SHIPPED`/`COMPLETED`/`CLOSED` return `FAILED_PRECONDITION`. A
`PENDING_PAYMENT` order is cancelled in a local transaction (guarded update +
status log + `shop.order.cancelled.v1`), then inventory `Release` and pricing
`UnlockCoupon` run best-effort (failures are logged, the cancellation stands).

### `CancelOrderInner` (DTM compensation)

`dtm.Branch` transaction: missing or already cancelled orders are a no-op;
otherwise `PENDING_PAYMENT` → `CANCELLED` with status log and
`shop.order.cancelled.v1`.

### `MarkPaid`

Called by the payment service callback; no TCC lives here. Idempotent:
already `PAID` returns the stored order (and replays the best-effort side
effects), `PENDING_PAYMENT` is updated with `paid_cent`
(default `pay_cent`, rejected when it exceeds `pay_cent`), status log and
`shop.order.paid.v1` outbox event in one transaction. Afterwards inventory
`Confirm` is called per item and pricing `ConsumeCoupon` for the quote coupon
(best-effort, retried by the callback replay).

### `GetOrder`

Requires `user_id` matching the order owner unless `admin` is set.

### `ListOrders`

Filters by `user_id` (always for non-admin, optional for admin) and `status`,
paginates with `page`/`page_size` (default 20, max 100) ordered by `id DESC`,
and returns `total` plus items grouped in one extra query.

## Events

Published through the outbox relay (`outbox.Relay`, 1s tick):
`shop.order.created.v1`, `shop.order.paid.v1`, `shop.order.cancelled.v1`
(aggregate `order`, aggregate id = `order_no`).

## Notes / known gaps

- `CreateOrderInnerRequest` (proto) does not carry `client_request_id`; the
  value is transported via DTM branch headers. Adding the field to the proto
  would remove that workaround (contract change owned by the platform agent).
- Coupon ids are not duplicated in `orders`; `ConsumeCoupon`/`UnlockCoupon`
  resolve the coupon with a best-effort `GetQuote`.
- `MarkPaid` writes in a local transaction on purpose: the payment service owns
  the payment callback and calls `MarkPaid`; no DTM TCC is implemented yet.
