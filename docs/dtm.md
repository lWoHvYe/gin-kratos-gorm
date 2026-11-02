# DTM integration

## Modes

- Create order: DTM Saga `order-create:<order_no>`.
- Cancel order: local `PENDING_PAYMENT -> CANCELLED` transaction plus the
  durable `shop.order.cancelled.v1` outbox event. Inventory and pricing consume
  that event to release stock and unlock the coupon. This is eventual
  compensation, not a DTM Saga.
- Payment confirm: DTM TCC `payment-confirm:<payment_no>`.
  Branches: order `TccTryMarkPaid/TccConfirmMarkPaid/TccCancelMarkPaid`,
  inventory `TccTryConfirm/TccConfirm/TccCancelConfirm`, pricing
  `TccTryConsume/TccConfirmConsume/TccCancelConsume`.
  `PAYMENT_TCC_ENABLED=false` switches back to the direct idempotent
  `MarkPaid` fallback.
- After-sale refund: `AfterSales.Refund` commits `REFUNDING`, calls
  `Payment.RefundOrder`, then commits `REFUNDED`. It is not a DTM global
  transaction; the `REFUNDING` state makes retries safe.

## GIDs

Deterministic, derived from business IDs:

```text
order-create:<order_no>
payment-confirm:<payment_no>
```

`order-cancel` and `aftersale-refund` GIDs are not used: those flows are
outbox/payment-call based, not DTM global transactions.

## Saga branch endpoints

Saga branch endpoints are normal gRPC methods. The service detects a DTM branch
context and executes inside the barrier transaction:

```go
return dtm.Branch(ctx, s.db, func(tx *sql.Tx) error {
    // business writes + outbox.Append(ctx, tx, ...)
})
```

Never call `db.Begin` or another ORM transaction inside the callback. A Saga
branch must not silently fall back to a local transaction: if the DTM context is
missing, return `UNAVAILABLE`.

Inventory/pricing direct admin calls and TCC confirm/cancel are different: they
use their local transaction helper because DTM does not invoke them through a
barrier. They are still idempotent and are retried by the caller/consumer.

## TCC branches

The payment-confirm TCC uses separate Try/Confirm/Cancel endpoints instead of a
barrier. Each method is idempotent by `order_no`/`payment_no`, and each branch
implements its own query method (`TccQuery*`). The interfaces are:

- order: `TccTryMarkPaid`, `TccConfirmMarkPaid`, `TccCancelMarkPaid`
- inventory: `TccTryConfirm`, `TccConfirm`, `TccCancelConfirm`
- pricing: `TccTryConsume`, `TccConfirmConsume`, `TccCancelConsume`

`PAYMENT_TCC_ENABLED=false` keeps the older direct `MarkPaid` path available for
rollback while the TCC path is being validated.

The TCC query methods are exposed through `Payment.VerifyPayment` and the admin
`POST /api/admin/v1/payments/:paymentNo/verify` route. DTM's TCC protocol itself
does not use `QueryPrepared`; the query methods are an operator-facing
consistency check.

When the TCC coordinator cancels after the order was already marked paid, the
order moves to `REFUNDING` and publishes `shop.order.refund_requested.v1`.
The payment service consumes it and calls `Refund` idempotently, so the
compensation also unwinds the captured payment.
