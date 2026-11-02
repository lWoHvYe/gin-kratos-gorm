# gateway service (public BFF)

Public HTTP entry point of the mall. The gateway is a stateless backend for the
frontend: it authenticates the shared JWT, maps JSON onto the generated
protobuf request types, and forwards each call to the service that owns the
data. It has no database and no gRPC server of its own.

- Package: `internal/services/gateway`
- Binary: `cmd/gateway`
- HTTP: `:8080` (fixed, see `docs/contracts.md`)
- Database: none
- Registry: the HTTP endpoint is registered in etcd as `gateway` when
  `ETCD_ENDPOINTS` is set

## Startup

`cmd/gateway` builds a plain `kratos.App`; `runtime.NewApp` is not used because
it requires a gRPC server:

```text
config.Load("gateway") -> log.New -> registry.New(cfg.ETCDEndpoints)
gateway.Dial(ctx, reg) -> gin engine -> khttp.NewServer(khttp.Address(":8080"))
srv.HandlePrefix("/", engine) -> kratos.New(Name, Logger, Server, [Registrar]).Run()
```

The Gin engine owns every route, including `/healthz`; the Kratos HTTP server
only forwards requests to it.

## Downstream clients

`gateway.Dial` opens one lazy gRPC connection per service:

| client | service name | direct fallback |
|---|---|---|
| `identity` | `discovery:///identity` | `IDENTITY_ADDR`, default `127.0.0.1:9001` |
| `catalog` | `discovery:///catalog` | `CATALOG_ADDR`, default `127.0.0.1:9002` |
| `cart` | `discovery:///cart` | `CART_ADDR`, default `127.0.0.1:9004` |
| `pricing` | `discovery:///pricing` | `PRICING_ADDR`, default `127.0.0.1:9005` |
| `order` | `discovery:///order` | `ORDER_ADDR`, default `127.0.0.1:9006` |
| `payment` | `discovery:///payment` | `PAYMENT_ADDR`, default `127.0.0.1:9007` |
| `fulfillment` | `discovery:///fulfillment` | `FULFILLMENT_ADDR`, default `127.0.0.1:9008` |
| `aftersales` | `discovery:///aftersales` | `AFTERSALES_ADDR`, default `127.0.0.1:9009` |

With a registry (non-empty `ETCD_ENDPOINTS`) the discovery targets are used.
Only when `registry.New` returns `nil` does the gateway fall back to the direct
`<SERVICE>_ADDR` addresses. Connections are dialed once at startup and closed
when the app exits; `grpc.NewClient` dials lazily, so an unavailable service
surfaces as a per-request `UNAVAILABLE` rather than a startup failure.

## Middleware chain

Applied to every route, in order:

1. `recovery` — `gin.CustomRecovery`; panics become the JSON error envelope.
2. `httpx.RequestID` — echoes/generates `X-Request-ID`.
3. `httpx.AccessLog` — one structured log line per request (method, path,
   status, latency, request id, client ip).
4. `httpx.CORS` — reflects the origin, answers preflight with `204`.
5. `timeout` — caps the request context at 15s, including downstream calls.

Unknown routes return `404` with `{"code":"NOT_FOUND", ...}` instead of Gin's
plain-text default.

## Authentication

`httpx.Auth(auth.NewManager(cfg))` validates the bearer token on the
authenticated `/api/v1` group; `requireUser` then rejects any token whose `uid`
claim is missing or zero. Handlers always overwrite `user_id` with
`auth.UserID(ctx)`, so a body cannot select another user. `GET /api/v1/cart`
without a token returns `401 UNAUTHENTICATED`.

## Routes

Public:

| method | path | downstream |
|---|---|---|
| POST | `/api/v1/auth/register` | `Identity.Register` (201) |
| POST | `/api/v1/auth/login` | `Identity.Login` |
| GET | `/api/v1/categories` | `Catalog.ListCategories` |
| GET | `/api/v1/products` | `Catalog.ListProducts` (`page`, `page_size`, `keyword`, `category_id`) |
| GET | `/api/v1/products/:id` | `Catalog.GetProduct` |
| GET | `/api/v1/skus/:id` | `Catalog.GetSku` |

Authenticated (`Authorization: Bearer <token>`):

| method | path | downstream / body |
|---|---|---|
| GET | `/api/v1/cart` | `Cart.GetCart` |
| POST | `/api/v1/cart/items` | `Cart.AddItem` `{sku_id, quantity}` |
| PATCH | `/api/v1/cart/items` | `Cart.UpdateItem` `{sku_id, quantity, selected}` |
| DELETE | `/api/v1/cart/items` | `Cart.RemoveItem` `{sku_id}` or `?sku_id=` |
| POST | `/api/v1/cart/merge` | `Cart.AddItem` per line `{items:[{sku_id, quantity}]}` |
| POST | `/api/v1/checkout/preview` | `Pricing.CreateQuote`; `items` is optional and defaults to selected cart lines: `{items:[{sku_id, quantity}], address_id, coupon_id}` |
| POST | `/api/v1/checkout/place` | `Order.CreateOrder` `{quote_id, address_id, receiver_name, receiver_phone, receiver_address, client_request_id}` (201) |
| GET | `/api/v1/orders` | `Order.ListOrders` (`page`, `page_size`, `status`) |
| GET | `/api/v1/orders/:orderNo` | `Order.GetOrder` |
| POST | `/api/v1/orders/:orderNo/cancel` | `Order.CancelOrder` `{reason}` |
| POST | `/api/v1/orders/:orderNo/receive` | `Fulfillment.GetShipmentByOrder` (or `GetShipment` when `{shipment_no}` is supplied) + `Fulfillment.Receive` |
| POST | `/api/v1/payments/:paymentNo/pay` | `Payment.Callback` `{channel, event_id, amount_cent, signature}` |
| POST | `/api/v1/after-sales` | `AfterSales.CreateAfterSale` `{order_no, type, reason, amount_cent, items}` (201) |
| GET | `/api/v1/after-sales/:no` | `AfterSales.GetAfterSale` (owner only) |

Details worth knowing:

- `POST /checkout/preview` with no `items` reads the selected cart lines. This
  makes the server cart the normal C-end source of truth and avoids trusting a
  client-generated list.
- `POST /checkout/place` fills any missing receiver fields from `address_id`
  (the address must belong to the user) before creating the order, then clears
  selected cart lines after the order and payment intent succeed. Clearing is best-effort: a stale cart is preferable to failing
  an already-created order.
- `POST /checkout/place` resolves the idempotency key from the body
  `client_request_id`, then the `Idempotency-Key` header, then a generated
  `id.Event()`. The order service keys `(user_id, client_request_id)` on it.
- `POST /payments/:paymentNo/pay` is a simulated success callback: it always
  sends `status = SUCCEEDED`, defaults `channel` to `MOCK`, resolves `event_id`
  like the order idempotency key, and forwards `signature` without verifying it.
  `amount_cent` is passed through (0 means "the payment service decides").
- `POST /orders/:orderNo/receive` verifies that the caller owns the order and
  that the shipment's `order_no` matches the path before receiving it.
- `POST /after-sales` verifies that the caller owns `order_no` (via
  `Order.GetOrder`) before opening the request: the after-sales service does not
  consult orders, so ownership is enforced here.
- `POST /cart/merge` is an add-based merge: each submitted line is added to the
  authenticated user's cart, then the resulting cart is returned. There is no
  server-side anonymous cart to merge from.
- All `*_id`, `*_cent` and `int32` query parameters are parsed strictly; a
  malformed value returns `400 INVALID_ARGUMENT` rather than being ignored.

## Errors

Failures use `httpx.Error` and the platform envelope:

```json
{"code":"NOT_FOUND","message":"product not found","request_id":"..."}
```

Downstream `google.golang.org/grpc/status` errors are translated back into
`apperr` values, so the HTTP status matches the domain meaning:

| gRPC code | apperr code | HTTP |
|---|---|---|
| `InvalidArgument` | `INVALID_ARGUMENT` | 400 |
| `Unauthenticated` | `UNAUTHENTICATED` | 401 |
| `PermissionDenied` | `PERMISSION_DENIED` | 403 |
| `NotFound` | `NOT_FOUND` | 404 |
| `AlreadyExists`, `Aborted` | `CONFLICT` | 409 |
| `FailedPrecondition`, `OutOfRange` | `FAILED_PRECONDITION` | 422 |
| `Unavailable`, `DeadlineExceeded`, `ResourceExhausted`, `Canceled` | `UNAVAILABLE` | 503 |
| anything else | `INTERNAL` | 500 |

`INTERNAL` messages are replaced with `"internal server error"` by `httpx.Error`;
the original error is not leaked to the client.

## Health

`GET /healthz` on `:8080` returns `{"status":"ok"}`. There is no `/readyz` or
`/metrics` endpoint because the gateway owns no database or gRPC server; use
etcd service health or the HTTP probe in `deploy/`.

## Configuration

| env | default | purpose |
|---|---|---|
| `JWT_SECRET` | dev placeholder | shared signing secret (>= 16 chars) |
| `JWT_ISSUER` | `mall` | token issuer |
| `JWT_ACCESS_TTL` | `15m` | access token lifetime |
| `ETCD_ENDPOINTS` | `127.0.0.1:2379` | service discovery; leave empty for direct dialing |
| `<SERVICE>_ADDR` | port table | direct target used when no registry is configured |
| `LOG_LEVEL` | `info` | `debug`, `warn`, `error` |
| `GIN_MODE` | `debug` | set `release` in production |

`config.Load("gateway")` still parses `MySQL_DSN`, `NATS_URL`, `DTM_ADDRESS`,
`GRPC_ADDR` and `HTTP_ADDR`, but the gateway uses none of them: the listen
address is fixed at `:8080` by the contract.

## Build

```bash
GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp GOSUMDB=off GOFLAGS=-mod=mod \
  go build ./cmd/gateway ./internal/services/gateway
```

## Known contract gaps

1. **Order receive** resolves the shipment by order number; `shipment_no` is
   optional. The gateway
   therefore requires `{shipment_no}` (body or `?shipment_no=`) and returns
   `400 INVALID_ARGUMENT` when it is missing. A `GetShipmentByOrder` RPC (or an
   order-side `Receive`) would remove this from the public contract.
2. **No payment signature verification.** `/payments/:paymentNo/pay` is a
   development callback simulator; a real channel needs signature validation
   and an intent creation route (`Payment.CreateIntent` is not exposed yet).
3. **Checkout preview trusts the submitted items.** The gateway does not read
   the cart server-side; the client sends the lines it wants priced. A cart-based
   preview would need cart and pricing coupling that belongs in a service.
4. **`/payments/:paymentNo/pay` is not ownership-checked.** `pb.Intent` carries
   no `user_id` and `Payment.Callback` only takes the payment number, so an
   authenticated user cannot be proven to own the intent. This route is a
   development simulator; a production callback must be channel-signed and a
   `GetIntent`-based owner check needs a proto field.
5. **No refresh/logout endpoints.** `LoginReply.refresh_token` is issued by
   identity but the gateway exposes no refresh route; add one when identity
   grows a refresh RPC.
