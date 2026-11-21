# After-sales service

Owns refund-only and return-refund requests: intake, review, lifetime amount
admission, return/restock progress and the durable refund task. Payment remains
the owner of captured/refunded facts; Inventory remains the owner of stock.

- gRPC `:9009`, health/metrics `:9109`, database `mall_aftersales`
- Package `internal/services/aftersales`, entrypoint `cmd/aftersales`
- Migrations `migrations/aftersales/0001_init.sql` … `0010_settlement_protocol.sql`

## Responsibilities

- Persist after-sale requests, line items, amount versions and an audit log.
- Serialize lifetime amount consumption in `after_sale_amount_ledger` using
  Payment `captured_cent` (order snapshot only for legacy rows).
- Record return receipt and queue durable per-SKU restock tasks.
- Record the refund intent and queue a durable refund task; call
  `Payment.RefundOrder` idempotently by `after_sale_no`.
- Emit `shop.aftersale.*` events through the transactional outbox.

## Amount model

`after_sale_amount_ledger` (one row per order):

- `captured_cent` comes from `Payment.GetPaymentByOrder` (source `payment`);
  `order_snapshot` is only the legacy fallback.
- `consumed_cent` is the sum of every non-released request's
  `effective_amount_cent`.
- Creation/approval lock the row `FOR UPDATE` and enforce
  `consumed + effective <= captured`.
- `REFUNDED` stays `CONSUMED`; only a rejected/cancelled request releases.
- `after_sales.amount_cent` is the requested amount;
  `effective_amount_cent` is the approved amount. Every change appends an
  immutable `after_sale_amount_versions` row.
- Payment independently enforces `refunded + amount <= captured`.

## Statuses and return progress

```text
REQUESTED --Approve(true)--> APPROVED --Refund--> REFUNDING --> REFUNDED
REQUESTED --Approve(false)--> REJECTED
APPROVED (RETURN_REFUND) --SubmitReturnTracking--> RETURNING
RETURNING --ReceiveReturn--> RETURN_RECEIVED --restock worker--> restock_state DONE
```

- `RETURN_RECEIVED` means the parcel arrived; it does **not** mean stock was
  restored.
- `restock_state` is `NONE/PENDING/DONE/FAILED_PERMANENT`.
  `RETURN_REFUND` cannot enter `REFUNDING` until `restock_state=DONE`.
- `amount_state` is `ACTIVE/CONSUMED/RELEASED`.

## API

| RPC | Behaviour |
|---|---|
| `CreateAfterSale` | Validates order/items, reserves lifetime amount, creates `REQUESTED` |
| `Approve` | `REQUESTED -> APPROVED/REJECTED`; partial approval releases the difference and appends an amount version |
| `SubmitReturnTracking` | `APPROVED -> RETURNING` for return-refund |
| `ReceiveReturn` | `RETURNING -> RETURN_RECEIVED`, creates per-SKU restock tasks, no inventory call in the request path |
| `Refund` | Calls `Order.BlockShipmentForRefund`, persists `REFUNDING` + refund task, fast-paths Payment |

Validation uses `apperr`: invalid amounts/items are `INVALID_ARGUMENT`, illegal
states are `FAILED_PRECONDITION`, unknown requests are `NOT_FOUND`.

## Durable workers

- `RunRestockWorker` claims `after_sale_restock_tasks` with a lease, calls
  `Inventory.Restock` (`restockNo=after_sale_no`, per-SKU idempotent), marks the
  task `DONE`, and only when every task is `DONE` sets
  `restock_state=DONE` + emits `shop.aftersale.restocked.v1`.
  Transient failures retry; permanent failures emit
  `shop.aftersale.restock_failed.v1` and keep refunds blocked.
- `RunRefundWorker` claims `after_sale_refund_tasks` with a lease, calls
  `Payment.RefundOrder` with `amount=effective_amount_cent` and
  `IdempotencyKey=after_sale_no`, then marks `REFUNDED` + emits
  `shop.aftersale.completed.v1`. Failures retry; a permanently failed task is
  visible and can be re-armed by an explicit `Refund` retry.
- There is no status scanner over `REFUNDING`/`RECEIVED` combinations.

## Audit log

`after_sale_logs` is append-only: `CREATE`, `APPROVE`, `REJECT`,
`RETURN_TRACKING`, `RECEIVE_RETURN`, `REFUND_INTENT`, `REFUND`. Amount changes
also append `after_sale_amount_versions`.

## Events

| Topic | Appended when |
|---|---|
| `shop.aftersale.created.v1` | `CreateAfterSale` commits |
| `shop.aftersale.approved.v1` / `.rejected.v1` | review commits |
| `shop.aftersale.returning.v1` | return tracking commits |
| `shop.aftersale.received.v1` | `RETURN_RECEIVED` commits |
| `shop.aftersale.restock_pending.v1` | restock tasks queued |
| `shop.aftersale.restocked.v1` | all restock tasks done |
| `shop.aftersale.restock_failed.v1` | a restock task is permanently failed |
| `shop.aftersale.refund_requested.v1` | `REFUNDING` + refund task committed |
| `shop.aftersale.completed.v1` | payment refund completed |

All payloads carry `after_sale_no, order_no, user_id, type, status,
amount_cent, effective_amount_cent, amount_state, restock_state, reason`.

## Schema

- `after_sales(after_sale_no, order_no, user_id, type, status, amount_cent,
  effective_amount_cent, amount_state, restock_state, reason, return_*,
  refund_no, refunded_cent, version, client_request_id, ...)`.
- `after_sale_items(after_sale_id, sku_id, quantity, received_quantity,
  condition_code)`.
- `after_sale_logs` — audit trail.
- `after_sale_amount_versions(after_sale_id, version, effective_amount_cent,
  reason)` — immutable amount history.
- `after_sale_amount_ledger(order_no, captured_cent, consumed_cent, source,
  version)` — lifetime cap.
- `after_sale_restock_tasks(after_sale_no, sku_id, quantity, condition, status,
  attempts, lease_owner, lease_until, ...)`.
- `after_sale_refund_tasks(after_sale_no, status, attempts, lease_owner,
  lease_until, ...)`.
- `outbox_events` — shared outbox.

## Configuration

`MYSQL_DSN`, `NATS_URL`, `PAYMENT_ADDR`, `INVENTORY_ADDR`, `ORDER_ADDR`,
`GRPC_ADDR`, `HTTP_ADDR`. `cmd/aftersales/main.go` starts the outbox relay plus
the restock and refund workers.
