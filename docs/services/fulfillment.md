# Fulfillment service

Owns parcels for paid orders: shipment creation, hand-over to the carrier and
delivery confirmation.

- gRPC `:9008`, health/metrics `:9108`, database `mall_fulfillment`
- Package `internal/services/fulfillment`, entrypoint `cmd/fulfillment`
- Migration `migrations/fulfillment/0001_init.sql`

## Responsibilities

- Keep the authoritative per-order shipment record and its line items.
- Drive the forward-only shipment lifecycle.
- Emit domain events through the transactional outbox (`shop.shipment.*`).
- No payment, inventory or order-table access; the service is self-contained.

## API

`pb.FulfillmentService` from `api/gen/mall/v1`:

| RPC | Request | Behaviour |
|---|---|---|
| `CreateShipment` | `order_no, warehouse_id, carrier, tracking_no, items[]` | Creates a shipment in `CREATED`; idempotent per `order_no` |
| `Ship` | `shipment_no` | `CREATED -> SHIPPED` |
| `Receive` | `shipment_no` | `SHIPPED -> RECEIVED` |
| `GetShipment` | `shipment_no` | Reads the shipment and its items |

Validation (returns `INVALID_ARGUMENT` via `apperr`):

- `order_no` must be non-empty (trimmed);
- at least one item is required;
- `sku_id > 0`, `quantity > 0`;
- duplicate `sku_id` lines are merged (quantities summed) before persisting.

`Ship`/`Receive` on an unknown shipment return `NOT_FOUND`. Illegal transitions
return `FAILED_PRECONDITION`.

## State machine

```text
CREATED --Ship--> SHIPPED --Receive--> RECEIVED
```

- Transitions are forward-only; the row is locked with `SELECT ... FOR UPDATE`.
- A repeated `Ship` on `SHIPPED` and a repeated `Receive` on `RECEIVED` are
  idempotent no-ops: the aggregate is returned and no extra event is appended.
- `Ship` on `RECEIVED` and `Receive` on `CREATED` are rejected.
- Every successful transition bumps `shipments.version`.

## Events

Outbox table `outbox_events`, relayed by `outbox.Relay` to NATS JetStream
subject `shop.>`. Aggregates `shipment` are keyed by `shipment_no`.

| Topic | Appended when | Payload |
|---|---|---|
| `shop.shipment.created.v1` | `CreateShipment` commits | `shipment_no, order_no, warehouse_id, carrier, tracking_no, status, items[]` |
| `shop.shipment.shipped.v1` | `Ship` commits | same shape, `status = SHIPPED` |
| `shop.shipment.delivered.v1` | `Receive` commits | same shape, `status = RECEIVED` |

Business write and `outbox.Append` share one `*sql.Tx`, so an event exists iff
the state change committed.

## Idempotency

`shipments` has `UNIQUE KEY uk_shipments_no (shipment_no)` (generated with
`id.Shipment()`) and `UNIQUE KEY uk_shipments_order (order_no)`.

`CreateShipment` first looks the order up; if a shipment already exists it is
returned unchanged. If two callers race, the loser of the unique-key insert
rolls back, re-reads the winner's shipment and returns it. Phase 1 therefore
allows one shipment per order (no split parcels); supporting partial shipments
later requires dropping `uk_shipments_order` and keying idempotency on
`order_no + shipment sequence`.

## Schema

- `shipments(id, shipment_no, order_no, warehouse_id, carrier, tracking_no,
  status, version, created_at, updated_at)` — `uk_shipments_no`,
  `uk_shipments_order`, `idx_shipments_status`.
- `shipment_items(id, shipment_id, sku_id, quantity, created_at)` —
  `uk_shipment_items_sku (shipment_id, sku_id)`, `idx_shipment_items_shipment`.
- `outbox_events(...)` — shared outbox schema from `docs/contracts.md`.

Money is not handled by this service, so no `_cent` columns exist.

## Configuration

`config.Load("fulfillment")`; the standard env vars apply (`MYSQL_DSN`,
`NATS_URL`, `ETCD_ENDPOINTS`, `GRPC_ADDR`, `HTTP_ADDR`). The service is not a
DTM branch service: it opens plain local transactions and exposes no branch
endpoints.

## Build and test

```bash
GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp GOSUMDB=off \
  go build ./cmd/fulfillment ./internal/services/fulfillment
GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp GOSUMDB=off \
  go test ./internal/services/fulfillment/...
```

Unit tests cover the pure parts only (request validation, SKU merging,
transition table). There are no integration tests, per `docs/contracts.md`.

## Not implemented (deliberately)

- No carrier API integration: `carrier`/`tracking_no` are supplied by the
  caller (admin/ops).
- No shipment cancellation or return-to-sender transition.
- No event consumption (order/payment services publish; this service does not
  subscribe yet).

## Automatic shipment on payment

The service runs a durable JetStream consumer on `shop.order.paid.v1`
(`fulfillment-order-paid`). It creates a `CREATED` shipment with carrier
`AUTO` from the paid order items, idempotently by `order_no`. An operator later
assigns a real carrier/tracking number and calls `Ship`. Delivery failures are
retried via `NakWithDelay`; duplicate events are safe because shipment creation
is keyed by order.
