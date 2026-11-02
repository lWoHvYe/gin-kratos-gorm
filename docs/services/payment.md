# Payment service

Mock payment service for the mall. It opens payment intents, applies channel
callbacks, asks the order service to mark the order paid and emits payment
domain events through the transactional outbox. There is no real payment SDK:
`pay_url` is a `mock://` URL and only integer cents ever touch the database.

- gRPC: `:9007` (`GRPC_ADDR`)
- Health/metrics: `:9107` (`HTTP_ADDR`)
- Database: `mall_payment`
- Proto: `PaymentService` in `api/proto/mall/v1/mall.proto`
- Generated package: `pb "gin-kratos-gorm/api/gen/mall/v1"`

## Environment

| Variable | Default | Meaning |
|---|---|---|
| `GRPC_ADDR` | `:9007` (deploy) | gRPC listen address |
| `HTTP_ADDR` | `:9107` (deploy) | `/healthz`, `/readyz`, `/metrics` |
| `MYSQL_DSN` | `root:root@tcp(127.0.0.1:3306)/mall_payment?...` | MySQL DSN |
| `NATS_URL` | `nats://127.0.0.1:4222` | JetStream for outbox relay |
| `ETCD_ENDPOINTS` | `127.0.0.1:2379` | service registration/discovery |
| `DTM_ADDRESS` | `127.0.0.1:36790` | DTM coordinator |
| `ORDER_ADDR` | `order:9006` | order service used by TCC/verification |
| `PAYMENT_SECRET` | *(empty)* | when set, callback HMAC is enforced |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

When `PAYMENT_SECRET` is empty the service runs in mock mode and accepts
unsigned callbacks (a warning is logged at startup). Set it in any shared
environment.

## Schema (`migrations/payment/0001_init.sql`)

| Table | Purpose |
|---|---|
| `payment_intents` | one row per payable order/channel/amount |
| `payment_callbacks` | channel event log, `UNIQUE(channel, event_id)` |
| `refunds` | refund ledger, `UNIQUE(payment_no, amount_cent, reason)` |
| `outbox_events` | transactional outbox drained by `outbox.Relay` |
| `dtm_barrier` | DTM branch protection for branch-context calls |

Intent statuses: `PENDING` → `SUCCEEDED` | `FAILED`; `SUCCEEDED` → `REFUNDED`.
A late success may still settle a `FAILED` intent; a late failure never
downgrades a `SUCCEEDED`/`REFUNDED` one.

## RPCs

### `CreateIntent(CreateIntentRequest) returns (Intent)`

- Validates `order_no != ""` and `amount_cent > 0`; `channel` defaults to
  `mock`.
- Idempotent by **`(order_no, amount_cent, channel)`**: the first call wins, a
  retry returns the same intent (`uk_payment_intents_idem` protects the race).
- Inserts `PENDING`, `payment_no = id.Payment()` (`PAY...`), and
  `pay_url = "mock://pay/" + payment_no`, `expires_at = now + 30m`.

### `GetIntent(GetIntentRequest) returns (Intent)`

- Returns the intent by `payment_no`; empty id is `INVALID_ARGUMENT`, unknown
  id is `NOT_FOUND`.

### `Callback(CallbackRequest) returns (CallbackReply)`

- Optional HMAC verification when `PAYMENT_SECRET` is set. The signature is
  hex HMAC-SHA256 of the literal string
  `payment_no|event_id|amount_cent|status`:

  ```python
  hmac.new(secret, f"{payment_no}|{event_id}|{amount_cent}|{status}".encode(), hashlib.sha256).hexdigest()
  ```

  The status that is signed is the raw request value, before normalization.
  Missing/invalid signatures return `UNAUTHENTICATED`.
- Accepted statuses: `SUCCEEDED`, `SUCCESS`, `PAID` → `SUCCEEDED`;
  `FAILED`, `CLOSED`, `CANCELLED`/`CANCELED` → `FAILED`. Anything else is
  `INVALID_ARGUMENT`.
- Idempotent by **`(channel, event_id)`**: the event row is inserted in the
  same transaction as the state change, so a replay is a successful no-op (and
  a failed transaction leaves nothing behind). A replay that reuses an
  `event_id` for a different `payment_no` is `CONFLICT`.
- The callback amount must equal the intent amount, otherwise
  `FAILED_PRECONDITION`.
- On `SUCCEEDED` the outbox row `shop.payment.succeeded.v1` is written and,
  after commit, the TCC `payment-confirm:<payment_no>` is started; with
  `PAYMENT_TCC_ENABLED=false` the older `MarkPaid` call is used instead.
  `shop.payment.failed.v1` is written for `FAILED`.
- If the TCC/MarkPaid step fails the callback returns an error so the channel
  retries; the callback row, intent and refund keys are idempotent.

### `Refund(RefundRequest) returns (Empty)`

- Validates `payment_no != ""`, `amount_cent > 0`, and that the intent is
  `SUCCEEDED` with `amount_cent <= intent amount`.
- Idempotent by **`(payment_no, amount_cent, reason)`** through
  `uk_refunds_idem`; a retry returns success without a second refund.
- Inserts the refund row, moves the intent to `REFUNDED` (bumping `version`),
  and writes `shop.payment.refunded.v1` to the outbox in the same transaction.
- Refunds are mocked as instant and settle the whole intent (`REFUNDED`); a
  partial refund is allowed but closes the intent, so a later refund needs a
  new order/payment.

## Events

| Topic | Event type | Aggregate |
|---|---|---|
| `shop.payment.succeeded.v1` | `payment.succeeded` | `payment:<payment_no>` |
| `shop.payment.failed.v1` | `payment.failed` | `payment:<payment_no>` |
| `shop.payment.refunded.v1` | `payment.refunded` | `payment:<payment_no>` |

Payloads carry `payment_no`, `order_no`, `user_id`, `amount_cent`, `channel`
and `status`; the refund event additionally carries `refund_no` and `reason`.
Events are appended with the business write and published by
`outbox.Relay(ctx, db, mq, logger, time.Second)`.

## DTM

`Callback` settles the local intent and outbox in one local `*sql.Tx`. On a
successful callback, payment runs the `payment-confirm:<payment_no>` TCC with
order, inventory and pricing branches. The local unique keys keep callbacks and
refunds idempotent; the TCC branch methods are idempotent on their side.
`payment_intents`/`refunds` writes and `outbox.Append` always share one
`*sql.Tx`.

## Running

```bash
mysql -uroot -p mall_payment < migrations/payment/0001_init.sql
GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp GOSUMDB=off GOFLAGS=-mod=mod \
  go run ./cmd/payment
```

Built with `database/sql` + `github.com/go-sql-driver/mysql` (no GORM), using
the platform helpers `config`, `db`, `log`, `grpcx`, `runtime`, `registry`,
`apperr`, `mq`, `outbox` and `dtm`. `httpx`/`auth` are HTTP/JWT helpers used by
the gateway, so this gRPC-only service does not import them.

## Tests

`go test ./internal/services/payment/...` covers signature verification,
status normalization, the intent state machine, event construction and input
validation. There are no integration tests and no database or network calls in
the tests.

## Limitations

- Mock only; no channel SDK, no asynchronous settlement.
- No refund ledger balance (partial refunds are not accumulated).
- The default success path uses the DTM payment-confirm TCC; `MarkPaid` is only
  the `PAYMENT_TCC_ENABLED=false` fallback.

### `RefundOrder(RefundOrderRequest) returns (Empty)`

Order-keyed refund used by after-sales. It resolves the latest `SUCCEEDED` (or
already `REFUNDED`) intent for the order and calls `Refund`; the refund is
idempotent by `(payment_no, amount_cent, reason)`.

### `VerifyPayment(VerifyPaymentRequest) returns (VerifyPaymentReply)`

Read-only TCC consistency check. It asks order/inventory/pricing for their
`TccQuery*` prepared state and returns the intent status. Exposed to operators
as `POST /api/admin/v1/payments/:paymentNo/verify`.
