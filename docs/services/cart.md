# cart service

Shopping cart for the mall. The cart stores a snapshot of sku name and price at
the time the item was added; pricing is re-checked by the order/pricing services
at checkout.

- Package: `internal/services/cart`
- Binary: `cmd/cart`
- gRPC: `:9004` (`GRPC_ADDR`), health/metrics: `:9104` (`HTTP_ADDR`)
- Database: `mall_cart` (`MYSQL_DSN`)

## Data model

`migrations/cart/0001_init.sql`:

- `carts(id, user_id, status)` — one cart per user (`uk_carts_user`), `status`
  defaults to `ACTIVE`.
- `cart_items(id, cart_id, sku_id, name, price_cent, quantity, selected)` with
  `UNIQUE (cart_id, sku_id)`; `selected` defaults to `1`.
- `outbox_events` — the standard platform outbox (relayed by `outbox.Relay`).

## RPCs

All money values are `BIGINT` cents. `user_id` always comes from the request
(the gateway injects the authenticated user); the service performs no auth.

### GetCart

Returns the user's cart with all items and `total_cent`. Reading never creates
rows: a user without a cart gets `{user_id, items: [], total_cent: 0}`.

`total_cent` is the sum of `price_cent * quantity` over **selected items only**.
Unselected lines are listed but do not contribute to the total. This rule is
implemented once in `totalCent` and covered by `service_test.go`.

### AddItem

1. Requires `user_id > 0`, `sku_id > 0`, `1 <= quantity <= 9999`.
2. Calls catalog `GetSku` (endpoint from `CATALOG_ADDR`, default
   `catalog:9002`) to read price; a missing sku maps to `NOT_FOUND`, transport
   failures to `UNAVAILABLE`.
3. The line name comes from the owning product (`GetProduct(spu_id)`); if that
   lookup fails the sku code is stored instead (best effort, logged at warn).
4. Upserts `(cart_id, sku_id)`: existing line quantity is accumulated, name and
   price are refreshed, and the line is selected.
5. Appends `shop.cart.updated.v1` / `cart.item_added` to the outbox in the same
   transaction and returns the full cart.

### UpdateItem

Sets `quantity` and `selected` on an existing line. `quantity <= 0` removes the
line. proto3 has no bool field presence, so `selected` is always applied
(`selected=false` unselects). Unknown sku or missing cart returns `NOT_FOUND`.

### RemoveItem

Deletes the line and returns the cart. Removal is idempotent: removing a sku
that is not present (or for a user without a cart) succeeds.

## Totals and concurrency

Mutations run in a single transaction (`db.WithTx`) that includes the outbox
insert, so a cart change and its event commit atomically. `carts.user_id` is
unique and the first insert uses `ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)`,
so concurrent first adds converge on one cart row.

## Outbox events

| topic | event_type | payload |
|---|---|---|
| `shop.cart.updated.v1` | `cart.item_added`, `cart.item_updated`, `cart.item_removed` | `{"user_id": <int64>, "sku_id": <int64>}` |

A `cart.item_removed` event is written only when a line was actually deleted, so
repeated `RemoveItem` calls on the same sku do not emit duplicates.

Note: cart events are not part of the Phase 1 event list in
`docs/contracts.md`; the topic/event names above need main-agent confirmation.

## Configuration

| env | default | purpose |
|---|---|---|
| `MYSQL_DSN` | local `mall` DSN | cart database |
| `GRPC_ADDR` | `:9000` (set `:9004`) | gRPC listen address |
| `HTTP_ADDR` | `:9100` (set `:9104`) | health/metrics |
| `NATS_URL` | `nats://127.0.0.1:4222` | outbox relay |
| `ETCD_ENDPOINTS` | empty | optional configuration-center bootstrap only |
| `CATALOG_ADDR` | `catalog:9002` | catalog gRPC target (direct address, no discovery) |

`CATALOG_ADDR` is dialed once in `main` with `grpcx.Dial(ctx, addr, nil)` and the
client is passed into the service.

## Build and test

```bash
GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp GOSUMDB=off GOFLAGS=-mod=mod \
  go build ./cmd/cart ./internal/services/cart
go test ./internal/services/cart/...
```

## Known contract gap

`pb.Sku` has no `name` field, so the cart resolves the display name through
`GetProduct` with `sku_code` as fallback. Adding `name` to `Sku` (or a batched
"get sku with product" RPC) would remove the extra catalog round trip.

## Checkout integration

`ClearSelected` removes `selected = 1` lines after checkout and writes
`cart.selected_cleared`. The gateway calls it best-effort after creating the
order and payment intent; the order remains authoritative if the call fails.
