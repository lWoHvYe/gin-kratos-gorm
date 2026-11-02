# Pricing service

Prices a basket and owns coupon lifecycle for the mall.

- gRPC: `:9005` (`GRPC_ADDR`)
- health/metrics: `:9105` (`HTTP_ADDR`)
- database: `mall_pricing`
- entrypoint: `cmd/pricing`
- implementation: `internal/services/pricing`

## Responsibilities

- `CreateQuote` prices a basket from catalog SKU prices and stores it for 15 minutes.
- `GetQuote` returns a stored, unexpired quote.
- `LockCoupon` / `UnlockCoupon` / `ConsumeCoupon` own the coupon state machine.
- No inventory reservation and no order creation: these belong to inventory and order.

## Money rules

All amounts are integer cents (`BIGINT`), never floats.

```text
total_cent    = sum(sku.price_cent * quantity)
freight_cent  = 0          if total_cent >= 9900
                1000       otherwise
discount_cent = coupon.amount_cent, capped at total_cent
pay_cent      = total_cent - discount_cent + freight_cent
```

- Freight thresholds are computed on the pre-discount goods total.
- The quote-level discount is allocated over the lines proportionally to
  `price*quantity` with cumulative rounding, so the sum of
  `quote_lines.discount_cent` equals `quotes.discount_cent` exactly.
- Quantities must be `1..999`; duplicate `sku_id` entries in one request are
  merged. Prices come from catalog, never from the client.

## Coupons

`coupons.status` is one of `AVAILABLE`, `LOCKED`, `USED`.

- `user_id = 0` means the coupon is public and usable by any logged-in user.
- `CreateQuote` validates a coupon (availability, ownership, expiry, minimum
  amount) and records its code in `quotes.coupon_id`; it does not lock it.
- `LockCoupon` moves `AVAILABLE -> LOCKED` and stores `order_no = quote_id`.
- `UnlockCoupon` moves `LOCKED -> AVAILABLE`.
- `ConsumeCoupon` moves `LOCKED -> USED`; consuming an already `USED` coupon for
  the same quote is a no-op.
- All three transitions bump `version` and are keyed by `order_no = quote_id`, so
  repeated DTM branches with the same payload are idempotent.
- Locking a coupon locked by another order returns `CONFLICT`; unlocking or
  consuming an `AVAILABLE` coupon returns `FAILED_PRECONDITION`.

Seeded coupon (public, min spend 50.00):

| code | user_id | amount_cent | min_amount_cent |
|---|---|---|---|
| WELCOME10 | 0 | 1000 | 5000 |

## DTM

`LockCoupon`, `UnlockCoupon` and `ConsumeCoupon` are the DTM branch endpoints and
take the same `LockCouponRequest` payload, so the action and the compensation
receive identical input. When the call carries a DTM branch context they run in
`dtm.Branch` (barrier transaction); otherwise they run in `db.WithTx`. Coupon
writes and the outbox row share the same `*sql.Tx`.

## Data model (`migrations/pricing/0001_init.sql`)

- `coupons(id, code, user_id, amount_cent, min_amount_cent, status, expire_at, order_no, version)`
- `quotes(quote_id, user_id, total_cent, discount_cent, freight_cent, pay_cent, coupon_id, expires_at)`
  where `coupon_id` stores the coupon **code** (`''` when unused).
- `quote_lines(quote_id, sku_id, name, price_cent, quantity, discount_cent)`,
  primary key `(quote_id, sku_id)`; `name` is the catalog `sku_code` because the
  catalog contract exposes no SKU display name.
- `outbox_events` (standard layout)
- `dtm_barrier` (created by DTM on demand, included for fresh databases)

## Events

Published through the outbox relay on `shop.>` JetStream subjects:

- `shop.pricing.coupon.locked.v1`
- `shop.pricing.coupon.unlocked.v1`
- `shop.pricing.coupon.used.v1`

Payload: `{quote_id, coupon_id, code, user_id, amount_cent}`.

## Configuration

| env | default |
|---|---|
| `MYSQL_DSN` | `root:root@tcp(127.0.0.1:3306)/mall?parseTime=true&loc=Local&charset=utf8mb4` |
| `NATS_URL` | `nats://127.0.0.1:4222` |
| `ETCD_ENDPOINTS` | `127.0.0.1:2379` |
| `DTM_ADDRESS` | `127.0.0.1:36790` |
| `GRPC_ADDR` | `:9000` (set `:9005` for pricing) |
| `HTTP_ADDR` | `:9100` (set `:9105` for pricing) |
| `CATALOG_ADDR` | `catalog:9002` (plain `host:port` or `discovery:///catalog`) |

## Develop

```bash
mysql mall_pricing < migrations/pricing/0001_init.sql
go build ./cmd/pricing ./internal/services/pricing
go test ./internal/services/pricing/...
```

## Cancellation consumer

The service consumes `shop.order.cancelled.v1` durably and returns the quote's
locked coupon to `AVAILABLE`. This is the reliable compensation path; duplicate
events are idempotent.
