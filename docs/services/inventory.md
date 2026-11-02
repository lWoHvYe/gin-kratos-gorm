# inventory service

Authoritative stock for the mall. Inventory owns the per-warehouse counters and
the reservation state machine used by order creation/cancellation; it never
returns physical stock for a sku it has no row for.

- Package: `internal/services/inventory`
- Binary: `cmd/inventory`
- gRPC: `:9003` (`GRPC_ADDR`), health/metrics: `:9103` (`HTTP_ADDR`)
- Database: `mall_inventory` (`MYSQL_DSN`)
- Events: `shop.inventory.reserved.v1`, `shop.inventory.confirmed.v1`,
  `shop.inventory.released.v1`

## Data model

`migrations/inventory/0001_init.sql`:

- `warehouses(id, name, status)` — seeded with `id = 1`, `Main Warehouse`.
  Requests with `warehouse_id <= 0` default to warehouse 1.
- `inventory_stocks(id, sku_id, warehouse_id, on_hand, reserved, version)` with
  `UNIQUE (sku_id, warehouse_id)` and a check that
  `0 <= reserved <= on_hand`. `available = on_hand - reserved`.
- `stock_reservations(id, reservation_no, order_no, sku_id, warehouse_id,
  quantity, status, expires_at, ...)` with `UNIQUE (order_no, sku_id)` and a
  unique `reservation_no`. Statuses: `HELD`, `CONFIRMED`, `RELEASED`.
- `inventory_ledger(id, reservation_no, order_no, sku_id, warehouse_id,
  quantity, entry_type, on_hand_after, reserved_after)` with
  `UNIQUE (reservation_no, entry_type)`. Entry types: `RESERVE`, `CONFIRM`,
  `RELEASE`.
- `outbox_events` — the standard platform outbox, relayed by `outbox.Relay`.
- `dtm_barrier` — the DTM barrier table (also created by DTM on demand).

Stock rows are provisioned out of band, because the service exposes no
"set stock" RPC. For local development:

```sql
INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand)
VALUES (1001, 1, 100)
ON DUPLICATE KEY UPDATE on_hand = on_hand + VALUES(on_hand);
```

`GetStock` for an unprovisioned sku/warehouse returns `NOT_FOUND`; it does not
fabricate a zero row.

## RPCs

All writes run in one transaction that also contains the outbox insert. When the
call carries DTM branch metadata (`dtm.InBranch(ctx)`), the transaction is the
DTM barrier transaction (`dtm.Branch`); otherwise a plain local transaction
(`db.WithTx`) is used. A DTM branch replay is skipped by the barrier, which is
why `Reserve` re-reads the committed reservation when its callback did not run.

### GetStock

Requires `sku_id > 0`; `warehouse_id` defaults to 1. Returns `on_hand`,
`reserved` and `available = on_hand - reserved`.

### Reserve

Requires a non-empty `order_no` (max 64 chars), `sku_id > 0`, `quantity > 0`,
and `0 <= ttl_seconds <= 86400`.

1. Idempotency check by `(order_no, sku_id)`. A `HELD` or `CONFIRMED`
   reservation is returned unchanged (`reservation_no` and `expires_at_unix`). A
   `RELEASED` reservation is a `CONFLICT`; reservations are never revived under
   the same order line.
2. Conditional decrement:
   `UPDATE inventory_stocks SET reserved = reserved + qty, version = version + 1
   WHERE sku_id = ? AND warehouse_id = ? AND on_hand - reserved >= qty`.
   Zero rows means `FAILED_PRECONDITION: insufficient stock ...` when the stock
   row exists, or `NOT_FOUND` when it does not.
3. Insert the `HELD` reservation with `expires_at = now + ttl` (default 900s),
   write the `RESERVE` ledger row and append
   `shop.inventory.reserved.v1` in the same transaction.
4. If a concurrent request wins the `(order_no, sku_id)` unique key, the
   transaction rolls back and the winner is returned, so concurrent reservations
   of the same order line are idempotent rather than double-counted.

### Confirm

Resolves the reservation by `(order_no, sku_id)`, locking it `FOR UPDATE`.
Repeating a `CONFIRMED` confirm is a no-op. Then:

- `on_hand = on_hand - qty`, `reserved = reserved - qty`, `version + 1`
  (guarded by `on_hand >= qty AND reserved >= qty`);
- reservation status `CONFIRMED`;
- ledger `CONFIRM`;
- outbox `shop.inventory.confirmed.v1`.

A `RELEASED` reservation returns `CONFLICT`. If the caller sends a non-zero
`quantity`, it must match the reservation quantity.

### Release

Resolves the reservation by `(order_no, sku_id)`, locking it `FOR UPDATE`.
Repeating a `RELEASED` release is a no-op.

- `HELD`: `reserved = reserved - qty`, status `RELEASED`, ledger `RELEASE`,
  outbox `shop.inventory.released.v1`.
- `CONFIRMED`: `CONFLICT` ("use the restock flow") — stock was already consumed,
  so releasing it would silently add availability back.
- Unknown reservation: `NOT_FOUND`.

## Invariants

- `available = on_hand - reserved` and `0 <= reserved <= on_hand` at all times;
  both the conditional updates and the table check enforce it.
- `(order_no, sku_id)` is the idempotency key for the reservation lifecycle.
- Reservation numbers are generated with `id.Reservation()`.
- Ledger rows and outbox rows commit atomically with the stock change; an
  outbox failure fails the branch, never a partial commit.

## Outbox events

| topic | event_type | aggregate |
|---|---|---|
| `shop.inventory.reserved.v1` | `inventory.reserved` | `stock_reservation/<reservation_no>` |
| `shop.inventory.confirmed.v1` | `inventory.confirmed` | `stock_reservation/<reservation_no>` |
| `shop.inventory.released.v1` | `inventory.released` | `stock_reservation/<reservation_no>` |

Payload:

```json
{
  "reservation_no": "RSV...",
  "order_no": "ORD...",
  "sku_id": 1001,
  "warehouse_id": 1,
  "quantity": 2,
  "status": "HELD",
  "expires_at_unix": 1700000900,
  "occurred_at_unix": 1700000000
}
```

`event_id` is deterministic (`inv-<event_type>-<reservation_no>`, capped at 64
chars) so a replayed branch cannot double-publish.

## DTM integration

Create-order saga: `Reserve` is an action branch, `Release` is its compensation.
Payment-confirm saga: `Confirm` is an action branch. Business writes and
`outbox.Append` always use the `*sql.Tx` passed to `dtm.Branch`; the service
never opens a second transaction inside a branch. The barrier makes branch
executions idempotent by `gid + branch_id`.

## Configuration

| env | default | purpose |
|---|---|---|
| `MYSQL_DSN` | local `mall` DSN | inventory database |
| `GRPC_ADDR` | `:9000` (set `:9003`) | gRPC listen address |
| `HTTP_ADDR` | `:9100` (set `:9103`) | health/metrics |
| `NATS_URL` | `nats://127.0.0.1:4222` | outbox relay |
| `ETCD_ENDPOINTS` | `127.0.0.1:2379` | service registration |
| `DTM_ADDRESS` | `127.0.0.1:36790` | DTM server for the orchestrator; branch handling needs no client |

`cmd/inventory/main.go` starts `outbox.Relay` (1s interval), registers the gRPC
server with etcd and serves `/healthz`, `/readyz` and `/metrics`.

## Build and test

```bash
GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp GOSUMDB=off GOFLAGS=-mod=mod \
  go build ./cmd/inventory ./internal/services/inventory
go test ./internal/services/inventory/...
```

Unit tests cover request validation/defaults, status-to-reply mapping, available
math and deterministic event IDs; there are no integration tests.

## Known limitations / follow-ups

- No expiry sweeper: `expires_at` is recorded and returned to the order service,
  but an expired `HELD` reservation keeps its `reserved` quantity until
  `Release` (or the order service releases it). A scheduled
  `SELECT ... FOR UPDATE SKIP LOCKED` sweeper, writing `RELEASE` ledger rows and
  `shop.inventory.released.v1` events, is the natural next step.
- No RPC to provision or adjust stock (`on_hand`); ops currently seeds rows via
  SQL. An admin-only `AdjustStock`/`Restock` RPC is required before the
  "restock flow" referenced by `Release` exists.
- Reservations are per `(order_no, sku_id)`. A single order line can only ever
  have one reservation; a released line cannot be reserved again.

## Cancellation consumer

The service consumes `shop.order.cancelled.v1` durably and releases each order
line via `Release`. This is the reliable compensation path; the order service no
longer calls inventory after committing the cancellation. Duplicate events are
idempotent.
