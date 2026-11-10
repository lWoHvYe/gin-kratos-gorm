# After-sales service

Owns refund-only and return-refund requests: intake, review decision and the
refund intent/state machine.

- gRPC `:9009`, health/metrics `:9109`, database `mall_aftersales`
- Package `internal/services/aftersales`, entrypoint `cmd/aftersales`
- Migration `migrations/aftersales/0001_init.sql`

## Responsibilities

- Persist after-sale requests, their line items and an append-only audit log.
- Enforce the after-sale state machine.
- Emit `shop.aftersale.*` events through the transactional outbox.
- Record the refund intent, call `Payment.RefundOrder`, then complete the
  request. The two-phase `REFUNDING` state makes retries safe.

## API

`pb.AfterSalesService` from `api/gen/mall/v1`:

| RPC | Request | Behaviour |
|---|---|---|
| `CreateAfterSale` | `order_no, user_id, type, reason, amount_cent, items[]` | Creates a request in `REQUESTED` |
| `GetAfterSale` | `after_sale_no` | Reads the request |
| `Approve` | `after_sale_no, approved, amount_cent, reason` | `REQUESTED -> APPROVED` or `-> REJECTED` |
| `Refund` | `after_sale_no` | `APPROVED -> REFUNDING -> REFUNDED` |

Validation (returns `INVALID_ARGUMENT` via `apperr`):

- `order_no` non-empty, `user_id > 0`;
- `type` is `REFUND_ONLY` or `RETURN_REFUND`;
- `amount_cent > 0`;
- optional items: `sku_id > 0`, `quantity > 0`; duplicate `sku_id` lines are
  merged (quantities summed).

`Approve`/`Refund` on an unknown request return `NOT_FOUND`; illegal transitions
return `FAILED_PRECONDITION`. Approving with `amount_cent > 0` records a partial
approval and must not exceed the requested amount (`INVALID_ARGUMENT` otherwise).

## State machine

```text
REQUESTED --Approve(approved=true)--> APPROVED --Refund--> REFUNDING --> REFUNDED
REQUESTED --Approve(approved=false)--> REJECTED
```

- The aggregate row is locked with `SELECT ... FOR UPDATE` before each decision.
- Repeated identical decisions are idempotent no-ops (`APPROVED` + approve,
  `REJECTED` + reject, refund of `REFUNDED`): the aggregate is returned and no
  extra event is appended.
- Conflicting or out-of-order transitions are rejected (e.g. refunding a
  `REQUESTED` or `REJECTED` request).
- `Refund` commits `REFUNDING` first, calls payment, then commits `REFUNDED`.
  A crash or payment failure leaves the row `REFUNDING`; retrying resumes from
  that state and calls the idempotent payment refund again.

## Audit log

`after_sale_logs` is append-only and written in the same transaction as every
state change:

| action | from -> to | remark |
|---|---|---|
| `CREATE` | `` -> `REQUESTED` | request reason |
| `APPROVE` | `REQUESTED` -> `APPROVED` | decision reason |
| `REJECT` | `REQUESTED` -> `REJECTED` | decision reason |
| `REFUND_INTENT` | `APPROVED` -> `REFUNDING` | JSON refund intent (`amount_cent`, `payment: "pending"`) |
| `REFUND` | `REFUNDING` -> `REFUNDED` | payment refund completed |

The `REFUND_INTENT` row is the durable record of the approved amount before the
payment call; the actual refund is executed by `Payment.RefundOrder`.

## Events

Outbox table `outbox_events`, relayed by `outbox.Relay` to NATS JetStream.
Aggregate type `after_sale`, keyed by `after_sale_no`.

| Topic | Appended when | Payload |
|---|---|---|
| `shop.aftersale.created.v1` | `CreateAfterSale` commits | `after_sale_no, order_no, user_id, type, status, amount_cent, reason` |
| `shop.aftersale.approved.v1` | approve commits | same shape, `status = APPROVED` |
| `shop.aftersale.rejected.v1` | reject commits | same shape, `status = REJECTED` |
| `shop.aftersale.completed.v1` | `Refund` commits | same shape, `status = REFUNDED` |

`created` and `completed` are part of the Phase 1 event set in
`docs/contracts.md`; `approved`/`rejected` follow the same `shop.<domain>.<event>.v1`
naming and need main-agent sign-off if they are to be treated as contract.

## Schema

- `after_sales(id, after_sale_no, order_no, user_id, type, status, amount_cent,
  reason, version, created_at, updated_at)` — `uk_after_sales_no`,
  `idx_after_sales_order`, `idx_after_sales_user`, `idx_after_sales_status`.
  `type` is a MySQL keyword and is back-quoted in queries.
- `after_sale_items(id, after_sale_id, sku_id, quantity, created_at)` —
  `uk_after_sale_items_sku (after_sale_id, sku_id)`.
- `after_sale_logs(id, after_sale_id, action, from_status, to_status, remark,
  created_at)`.
- `outbox_events(...)` — shared outbox schema from `docs/contracts.md`.

`amount_cent` is `BIGINT` per the money convention.

## Configuration

`config.Load("aftersales")`; standard env vars apply (`MYSQL_DSN`, `NATS_URL`,
`ETCD_ENDPOINTS`, `GRPC_ADDR`, `HTTP_ADDR`). `PAYMENT_ADDR` points at the payment
gRPC service. There is no DTM branch in this service; the payment refund is a
normal idempotent gRPC call.

## Build and test

```bash
GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp GOSUMDB=off \
  go build ./cmd/aftersales ./internal/services/aftersales
GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp GOSUMDB=off \
  go test ./internal/services/aftersales/...
```

Unit tests cover request validation, SKU merging and both transition tables.
There are no integration tests, per `docs/contracts.md`.

## Not implemented (deliberately)

- No idempotency key on `CreateAfterSale` (multiple requests per order are
  legal); a client retry creates a second request.
- No item-return logistics or inspection step for `RETURN_REFUND`.
- `GetAfterSale` does not return line items: the generated `AfterSale` message
  has no items field. Items are stored for audit and future contract use.

## Payment integration

`Refund` is two-phase: it commits `REFUNDING`, calls `Payment.RefundOrder`, and
then commits `REFUNDED` with `shop.aftersale.completed.v1`. If payment is down,
the request stays `REFUNDING` and a retry resumes it; the payment refund itself
is idempotent. The service now depends on the payment gRPC service.

## Phase 1.5 return flow

`0002_return_flow.sql` adds return carrier/tracking, refund number, cumulative
refunded amount and per-item received quantity/condition. `RETURN_REFUND` goes
through `Approve -> SubmitReturnTracking -> ReceiveReturn -> Refund`; receiving
calls inventory `Restock` with the after-sale number as the idempotency key.
`SELLABLE` returns restore stock; `DAMAGED` returns are recorded in the ledger
without increasing sellable stock. Create/approve is validated against the
order service (ownership, order status, refundable amount and ordered
quantities). `ListAfterSales` serves user and admin lists.
