# inventory service

Authoritative stock for the mall. Inventory owns the per-warehouse counters and
the reservation state machine used by order creation/cancellation; it never
returns physical stock for a sku it has no row for.

- Package: `internal/services/inventory`
- Binary: `cmd/inventory`
- gRPC: `:9003` (`GRPC_ADDR`), health/metrics: `:9103` (`HTTP_ADDR`)
- Database: `mall_inventory` (`MYSQL_DSN`)
- Events: `shop.inventory.reserved.v1`, `shop.inventory.confirmed.v1`,
  `shop.inventory.released.v1`, `shop.inventory.operation.completed.v1`,
  `shop.inventory.operation.failed.v1`,
  `shop.inventory.operation.item_failed.v1`

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
- `inventory_operations(id, operation_id, operation_type, order_no, scope_hash,
  status, total_items, succeeded_items, failed_items, skipped_items, version,
  ...)` with `UNIQUE (order_no, operation_type, scope_hash)`. Statuses:
  `PENDING`, `IN_PROGRESS`, `SUCCEEDED`, `PARTIAL`.
- `inventory_operation_items(operation_id, sku_id, quantity, status, attempts,
  last_error, ...)` with `PRIMARY KEY (operation_id, sku_id)`. Item statuses:
  `PENDING`, `DONE`, `FAILED_PERMANENT`, `MISSING`.
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

### AllocateOrder (read-only)

`AllocateOrder` is the phase-2 checkout preview used by the warehouse-C shadow
path. It takes `lines[{sku_id, quantity}]` and returns one
`{line_no, sku_id, quantity, warehouse_id}` per input line. It selects the
lowest warehouse id that has enough available stock for the whole line
(`on_hand - reserved`), consuming a local preview pool for repeated SKU lines.
It does **not** create a reservation or ledger row.

A line that cannot fit in one warehouse returns `FAILED_PRECONDITION`; v1 is
line-level split only, so the checkout must split the line before payment
rather than silently changing the quantity. Phase 2b will add quote/warehouse
fields and per-warehouse freight.

### Reserve

Requires a non-empty `order_no` (max 64 chars), `sku_id > 0`, `quantity > 0`,
and `0 <= ttl_seconds <= 86400`.

1. Idempotency check by `(order_no, sku_id, warehouse_id)`. A `HELD` or `CONFIRMED`
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

## Whole-order operations

Multi-SKU Confirm and Cancel do not loop with "first error wins" any more. Each
call is an operation:

1. Canonicalize the item set by `sku_id`, reject duplicate/non-positive items,
   and derive `scope_hash = sha256(sku:qty,...)` and
   `operation_id = "inv:<type>:<order_no>:<scope_hash[:32]>"`. Confirm and
   Cancel, or two different item sets, are different operations.
2. Ensure `inventory_operations` and `inventory_operation_items` exist. A
   replay with the same `(order_no, type, scope_hash)` reuses the same progress;
   a conflicting item set is `CONFLICT` before any stock write.
3. For non-DTM calls (consumers/direct RPC), process each `PENDING` item in its
   own transaction with a savepoint around the business write. Success marks
   `DONE`; a permanent failure marks `FAILED_PERMANENT`; a missing reservation
   on Cancel marks `MISSING` and does **not** drop the remaining items. A
   transient failure increments `attempts`/`last_error` and remains `PENDING`.
   The handler returns an error (NAK) while any item is PENDING, so a
   redelivery skips completed items. Terminal `PARTIAL` is Acked; permanent
   failures are durable per-item records/events, not an endless batch retry.
4. For a DTM branch call, the whole item set is applied in **one**
   `dtm.Branch` transaction; any failure rolls the branch back and DTM owns the
   retry. A later retry skips items a consumer already committed.
5. When every item is terminal the operation becomes `SUCCEEDED` (all `DONE`)
   or `PARTIAL` (any `FAILED_PERMANENT`/`MISSING`), and the terminal outbox
   event is written in the same transaction as the final item/progress update.
   Item retries stop after 8 attempts and become `FAILED_PERMANENT`.

`ReplayInventoryOperation(operation_id, skus)` is the explicit recovery path for
`FAILED_PERMANENT`/`MISSING` items: it decrements the matching counters, resets
selected items to `PENDING` with `attempts = 0`, and re-runs the protocol. It
refuses to replay a `DONE` item because stock already moved, and there is no
background recovery scanner.

The same image exposes it as a one-shot command (not a scanner):

```bash
/app/app op-replay -operation <operation_id>        # all non-DONE items
/app/app op-replay -operation <operation_id> -skus 1001,1002
```

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
| `shop.inventory.operation.completed.v1` | `inventory.operation_completed` | `inventory_operation/<hashed id>` |
| `shop.inventory.operation.failed.v1` | `inventory.operation_failed` | `inventory_operation/<hashed id>` |
| `shop.inventory.operation.item_failed.v1` | `inventory.operation_item_failed` | `inventory_operation/<hashed id>` |

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
Payment-confirm saga: `TccConfirm`/`TccCancelConfirm` are one branch each. All
item writes, operation progress and `outbox.Append` use the single `*sql.Tx`
passed to `dtm.Branch`; the service never opens a nested transaction inside a
branch. The barrier makes branch executions idempotent by `gid + branch_id`.
After a branch commit (including a barrier replay that skips the callback), the
operation summary is read back from MySQL so a previous `PARTIAL` cannot be
reported as success.

## Configuration

| env | default | purpose |
|---|---|---|
| `MYSQL_DSN` | local `mall` DSN | inventory database |
| `GRPC_ADDR` | `:9000` (set `:9003`) | gRPC listen address |
| `HTTP_ADDR` | `:9100` (set `:9103`) | health/metrics |
| `NATS_URL` | `nats://127.0.0.1:4222` | outbox relay |
| `ETCD_ENDPOINTS` | empty | optional configuration-center bootstrap only |
| `DTM_ADDRESS` | `127.0.0.1:36790` | DTM server for the orchestrator; branch handling needs no client |

`cmd/inventory/main.go` starts `outbox.Relay` (1s interval), registers the gRPC
serves `/healthz`, `/readyz` and `/metrics`.

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
