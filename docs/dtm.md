# DTM integration

## Modes

- Create order: Saga
- Cancel order: Saga
- Payment confirm: DTM TCC, GID `payment-confirm:<payment_no>`.
  Branches: order `TccTryMarkPaid/TccConfirmMarkPaid/TccCancelMarkPaid`,
  inventory `TccTryConfirm/TccConfirm/TccCancelConfirm`, pricing
  `TccTryConsume/TccConfirmConsume/TccCancelConsume`.
  `PAYMENT_TCC_ENABLED=false` switches back to the direct idempotent
  `MarkPaid` fallback.
- Refund/after-sale: Saga

## GIDs

Deterministic, derived from business IDs:

```text
order-create:<order_no>
order-cancel:<order_no>
payment-confirm:<payment_no>
aftersale-refund:<after_sale_no>
```

## Branch endpoints

Saga branch endpoints are normal gRPC methods. The service detects a DTM branch
context and executes inside the barrier transaction:

```go
return dtm.Branch(ctx, s.db, func(tx *sql.Tx) error {
    // business writes + outbox.Append(ctx, tx, ...)
})
```

Never call `db.Begin` or another ORM transaction inside the callback.

## Failure behavior

- DTM is required for order/payment write paths.
- If DTM is unavailable, return `UNAVAILABLE`; do not silently fall back to a
  local transaction.
- Branch handlers must be idempotent by `gid + reservation_no/order_no`.
- `dtm.BarrierFromGrpc` errors on non-DTM calls; branch methods should return a
  clear `UNAVAILABLE` error in that case.

## TCC branches

The payment-confirm TCC uses separate Try/Confirm/Cancel endpoints instead of a
barrier. Each method is idempotent by `order_no`/`payment_no`, and each branch
implements its own query method (`TccQuery*`). The interfaces are:

- order: `TccTryMarkPaid`, `TccConfirmMarkPaid`, `TccCancelMarkPaid`
- inventory: `TccTryConfirm`, `TccConfirm`, `TccCancelConfirm`
- pricing: `TccTryConsume`, `TccConfirmConsume`, `TccCancelConsume`

`PAYMENT_TCC_ENABLED=false` keeps the older direct `MarkPaid` path available for
rollback while the TCC path is being validated.

When the TCC coordinator cancels after the order was already marked paid, the
order moves to `REFUNDING` and publishes `shop.order.refund_requested.v1`.
The payment service consumes it and calls `Refund` idempotently, so the
compensation also unwinds the captured payment.
