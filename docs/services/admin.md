# admin service (back-office BFF)

Back-office HTTP entry point of the mall. Like the gateway it is stateless: it
authenticates the shared JWT, requires the `admin` role, maps JSON onto the
generated protobuf request types, and forwards the call to the owning service.
It has no database and no gRPC server.

- Package: `internal/services/admin`
- Binary: `cmd/admin`
- HTTP: `:8081` (fixed, see `docs/contracts.md`)
- Database: none
- Registry: the HTTP endpoint is registered in etcd as `admin` when
  `ETCD_ENDPOINTS` is set

## Startup

`cmd/admin` builds a plain `kratos.App` (not `runtime.NewApp`, which would
require a gRPC server):

```text
config.Load("admin") -> log.New -> registry.New(cfg.ETCDEndpoints)
admin.Dial(ctx, reg) -> gin engine -> khttp.NewServer(khttp.Address(":8081"))
srv.HandlePrefix("/", engine) -> kratos.New(Name, Logger, Server, [Registrar]).Run()
```

The Gin engine owns every route, including `/healthz`.

## Downstream clients

Only the services the back office needs are dialed:

| client | service name | direct fallback |
|---|---|---|
| `catalog` | `discovery:///catalog` | `CATALOG_ADDR`, default `127.0.0.1:9002` |
| `order` | `discovery:///order` | `ORDER_ADDR`, default `127.0.0.1:9006` |
| `fulfillment` | `discovery:///fulfillment` | `FULFILLMENT_ADDR`, default `127.0.0.1:9008` |
| `aftersales` | `discovery:///aftersales` | `AFTERSALES_ADDR`, default `127.0.0.1:9009` |

With a registry the discovery targets are used; only when `registry.New`
returns `nil` does the BFF fall back to `<SERVICE>_ADDR`. The admin BFF keeps
its own small dial helper instead of importing the gateway package, so the two
BFFs stay independently deployable.

## Authorization

Every `/api/admin/v1` route is behind `httpx.Auth(auth.NewManager(cfg))` and
`httpx.RequireRole("admin")`:

- no/invalid token -> `401 UNAUTHENTICATED`
- valid token without the `admin` role -> `403 PERMISSION_DENIED`

The downstream services do **not** check the admin role themselves (`Catalog`,
`Fulfillment` and `AfterSales` trust their caller). The admin BFF is therefore
the authorization boundary for these write paths and must not be exposed
publicly.

## Routes

| method | path | downstream / body |
|---|---|---|
| POST | `/api/admin/v1/categories` | `Catalog.CreateCategory` `{category:{parent_id, name, level, sort, status}}` (201) |
| POST | `/api/admin/v1/products` | `Catalog.CreateProduct` `{category_id, brand_id, name, subtitle, description, main_image, skus:[{sku_code, specs_json, price_cent, stock}]}` (201) |
| PUT | `/api/admin/v1/products/:id/status` | `Catalog.SetProductStatus` `{status}` |
| GET | `/api/admin/v1/orders` | `Order.ListOrders` with `admin=true` (`page`, `page_size`, `status`, `user_id`) |
| POST | `/api/admin/v1/shipments` | `Fulfillment.CreateShipment` (201), optional `Fulfillment.Ship` |
| POST | `/api/admin/v1/after-sales/:no/approve` | `AfterSales.Approve` `{approved, amount_cent, reason}` |

Details worth knowing:

- `POST /categories` and `POST /products` bind the generated protobuf request
  structs directly, so the category body is nested under `"category"` and the
  SKU list keeps the proto field names.
- `PUT /products/:id/status` takes the id from the path and the status from the
  body; catalog validates the value (`ON_SALE`, `OFF_SALE`, `DRAFT`). It returns
  `{"id": <id>, "status": <status>}`.
- `GET /orders` always sets `admin=true`; the optional `user_id` narrows the
  list to one buyer. Without `user_id` the order service returns all orders.
- `POST /shipments` defaults to a full shipment: when `items` is omitted the BFF
  loads the order (`Order.GetOrder` with `admin=true`) and copies every order
  line into the shipment. Pass explicit `items` for a partial shipment.
  `"ship": true` additionally calls `Fulfillment.Ship`, returning the shipment
  in `SHIPPED` state. `CreateShipment` is idempotent by `order_no` in
  fulfillment, so retries return the existing parcel.
- `POST /after-sales/:no/approve` defaults `approved` to `true` (the common
  case). Send `{"approved": false, "reason": "..."}` to reject, or
  `{"amount_cent": <n>}` to approve a lower amount; aftersales rejects an
  approval amount above the requested amount and conflicting repeat decisions.

## Middleware chain

Identical to the gateway: `recovery` (`gin.CustomRecovery`, JSON errors),
`httpx.RequestID`, `httpx.AccessLog`, `httpx.CORS`, and a 15s request timeout.
Unknown routes return a `404` JSON envelope instead of Gin's plain-text
default.

## Errors

Same envelope and mapping as the gateway:

```json
{"code":"INVALID_ARGUMENT","message":"order_no is required","request_id":"..."}
```

gRPC `InvalidArgument`, `Unauthenticated`, `PermissionDenied`, `NotFound`,
`AlreadyExists`/`Aborted`, `FailedPrecondition`/`OutOfRange` and
`Unavailable`/timeout codes map to `400`, `401`, `403`, `404`, `409`, `422` and
`503` respectively; everything else becomes a `500` with
`"internal server error"`.

## Health

`GET /healthz` on `:8081` returns `{"status":"ok"}`. No `/readyz` and no
`/metrics`: the BFF owns no database or gRPC server.

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

`config.Load("admin")` also parses `MySQL_DSN`, `NATS_URL`, `DTM_ADDRESS`,
`GRPC_ADDR` and `HTTP_ADDR`, but none are used: the listen address is fixed at
`:8081` by the contract.

## Build

```bash
GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp GOSUMDB=off GOFLAGS=-mod=mod \
  go build ./cmd/admin ./internal/services/admin
```

## Known contract gaps

1. **No refund route.** `AfterSales.Refund` (and `Approve` with a partial
   amount) are exposed only partially: an admin can approve, but triggering the
   refund needs a dedicated `POST /after-sales/:no/refund`. Left out because the
   listed admin route set stops at approve.
2. **Admin cancel/receive are not exposed.** Order cancellation and receipt live
   on the public gateway (as the buyer) and fulfillment has no admin "force"
   variant, so a back-office operator cannot yet cancel or close an order.
3. **Shipment lookup is by order, not reversed.** `POST /shipments` copies order
   lines, but tracking updates (`Fulfillment.Ship`) by `shipment_no` are not
   surfaced as a separate route; pass `"ship": true` at creation or add
   `POST /shipments/:no/ship`.
4. **Role enforcement is BFF-only.** `products`, `shipments` and `after-sales`
   write RPCs accept any caller on the gRPC plane. Do not expose `:9008`/`:9009`
   outside the cluster; moving the check into the services would need contract
   and ownership changes.

## Inventory adjustment

`POST /api/admin/v1/inventory/adjust`

```json
{"sku_id":1,"warehouse_id":1,"delta":50,"reason":"inbound"}
```

The inventory service applies the signed delta (positive may create the stock
row for a newly listed SKU), writes an `inventory_ledger` row and publishes
`shop.inventory.adjusted.v1`. Negative deltas cannot take `on_hand` below
`reserved`.

## Operations routes

- `POST /api/admin/v1/inventory/adjust` — signed stock adjustment.
- `POST /api/admin/v1/orders/:orderNo/cancel` — admin cancel (reason body).
- `POST /api/admin/v1/shipments/:shipmentNo/ship` — move CREATED → SHIPPED.
- `POST /api/admin/v1/after-sales/:no/approve` — approve/reject.
- `POST /api/admin/v1/after-sales/:no/refund` — execute the recorded refund.
