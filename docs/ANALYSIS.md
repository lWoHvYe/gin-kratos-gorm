# Project analysis and upgrade notes

This document records what the original demo actually did, the concrete
problems found, and how the current code addresses them.

## 1. Original shape

The repository mixed four commands and three overlapping implementations:

| Command | What it really was |
|---|---|
| `cmd/user-go` | A monolith that registered user + product + order + greeter gRPC services and Gin routes against the root `internal/{biz,data,service,server}` tree. |
| `cmd/order` | A separate order service with its **own** copy of models/queries/repositories/transports and a DTM SAGA. |
| `cmd/product` | A separate product service with another copy of product models/queries/repositories. |
| `cmd/helloworld-go` | Kratos template demo. |

The root tree and the per-service trees duplicated Order/Product models,
queries and repositories. The `internal/order` and `internal/product` trees also
had their own config loaders with `10.211.55.29` hard-coded.

## 2. Defects found

### DTM / distributed transaction

- `dtmgrpc.MustGenGid(s.dtmC.Address)` panics (or blocks) when DTM is not
  reachable. A `defer recover()` only logged the panic, then the handler
  returned `(nil, nil)` — a successful-looking response with a nil order.
- DTM was the only path for order creation in `cmd/order`; there was no local
  fallback. The VM Docker DTM instance was therefore a hard dependency for any
  integration test or local run.
- The order payload was mutated (`req.OrderSn`, `req.UserId`) and then reused
  for the DTM inner call; prices were hard-coded (`319.0`, `99.0`).
- Product `ReduceStock` called `barrier.CallWithDB(sqlDB, ...)` but then used
  `gormDB.Transaction(...)` inside the callback, so the business update did not
  run on the barrier's `*sql.Tx`. The DTM barrier and the business transaction
  were not atomic.
- No `WaitResult`, retry limit or idempotency key; retries could create
  duplicate orders or deduct stock twice.
- `CancelOrder` used the same `CreateOrderReq` payload (required by DTM) but
  there was no user-facing cancel reason/state machine.

### Domain and data correctness

- Prices and receiver data were hard-coded; the product catalog was never used
  to price an order.
- No stock idempotency in the root implementation; retries could double-deduct.
- `q.ProductSku.Stock.Gte(...)` then `Update` ignored the case where
  `RowsAffected == 0` in some paths; insufficient stock could be treated as
  success.
- Order status was not used as a compare-and-set guard, so concurrent
  pay/cancel requests could overwrite each other.
- GORM association tags used `foreignKey:OrderId`/`SpuId` but the Go fields are
  `OrderID`/`SpuID`; preloads were fragile.
- SQLite (intended for tests) could not scan `datetime(3)` columns and
  `bigint unsigned` primary keys were not auto-incrementing. Tests did not
  exist to catch either.

### AuthN / AuthZ

- JWT signing key `"lWoHvYe"` was hard-coded in multiple packages.
- Passwords were neither stored nor checked; `Login` only checked that the
  password field was non-empty.
- `utils.GetLoginUserId` did `ctx.Value("userID").(uint64)` and panicked for
  gRPC requests, which have no Gin context.
- The root gRPC server had no JWT middleware, while the order gRPC server did.
- Casbin was initialized but the middleware was only attached to a subgroup;
  admin routes were not consistently protected. The policy matcher required
  grouping only, while the code checked role codes.
- No rate limiting on login, no password length/hash validation.

### APIs and operations

- No product create/update/on-sale/stock-adjust/list APIs.
- No order list/pay/cancel-by-user APIs; order status was not exposed with a
  human-readable label.
- No pagination conventions, no validation layer, inconsistent error responses
  (`{"error": ...}` vs `{"msg": ...}`).
- No health/readiness endpoints, metrics, request IDs or access logs.
- No background worker for expired-order cancellation or outbox dispatch.
- No tests, migrations, or reproducible local setup.
- `configs/config.yaml` hard-coded a VM IP; `make build`/Docker used a Go 1.25
  image while `go.mod` requires 1.26.

### Code hygiene

- Duplicate `internal/order` and `internal/product` trees diverged from the
  monolith.
- `internal/data/utils/search.applyPublic` called `db.Where(...)` without
  assigning the result, so conditions silently did nothing.
- `internal/biz/repository.go` was an unused generic repository layer.
- Generated GORM query packages existed for only some tables, and the root
  query package did not know about `stock_logs`/`outbox_events`.

## 3. What changed

### Consistent architecture

- One shared domain/data/service layer now serves the monolith and the split
  binaries. The duplicate `internal/order/**` and `internal/product/**` trees
  were removed.
- `cmd/user-go` became `cmd/server` (it is a monolith, not a user service).
  The Greeter/helloworld template code was removed.
- `internal/biz` owns repository interfaces and use cases; `internal/data`
  implements them; `internal/service` maps protobuf; `internal/server` owns
  transports and middleware.

### Transactions

- `biz.Transactor` + `TxSaga`: all steps in one DB transaction (monolith).
- `biz.CompensationSaga`: sequential steps with reverse-order undo, used when
  product is a remote service. It is deterministic and fully testable without
  DTM.
- `DtmCoordinator`: opt-in remote saga with:
  - deterministic GID (`order-<order_sn>`) instead of `MustGenGid`;
  - startup probe for direct addresses;
  - `fallback_to_local` startup fallback;
  - `WaitResult`, timeout and retry limit;
  - no silent fallback after Submit failure (returns `UNAVAILABLE`).
- Product stock operations support both local repository transactions and the
  DTM branch barrier (raw SQL executed on the barrier's `*sql.Tx`).

### Ordering

- Authoritative SKU snapshots (name, image, price, stock, on-sale) are loaded
  before pricing; totals are copied into order items.
- Money is rounded to cents; shipping rules are configurable.
- Duplicate SKU lines are merged, so stock steps match persisted items.
- `Idempotency-Key` maps to a unique `(user_id, client_request_id)` index.
- Pay/cancel use `WHERE status = <expected>` compare-and-set transitions.
- Cancel restores stock synchronously and records an `order.cancelled` event;
  the outbox handler retries restoration idempotently.
- A worker auto-cancels expired unpaid orders.

### Stock

- `stock_logs` primary keys are deterministic:
  `deduct:<order_sn>:<sku_id>` and `restore:<order_sn>:<sku_id>`.
- Duplicate deduct/restore requests are successful no-ops.
- Stock updates are conditional (`stock >= ?` or `stock + delta >= 0`).

### Outbox

- `outbox_events` is written in the same transaction as the order.
- A dispatcher polls with batch/backoff, retries up to 5 attempts, and parks
  failed events for inspection.
- Events are at-least-once; handlers are idempotent.

### Auth and RBAC

- bcrypt password hashing and verification; password policy enforced.
- JWT secret/issuer/TTL from configuration; typed claims shared by Gin and gRPC.
- Gin middleware injects claims into `request.Context()`; gRPC middleware
  extracts `authorization` metadata.
- Role codes (`admin`, `user`) are in the token; Casbin policies use
  `r:admin`/`r:user` subjects. Admin routes also have a `RequireRole("admin")`
  fallback when Casbin is disabled.
- Login rate limiting is included as a process-local safeguard.

### Operations

- `/healthz` (liveness), `/readyz` (DB ping), `/metrics` (Prometheus text).
- Request ID propagation, access logs and RED metrics.
- Graceful shutdown includes the outbox/auto-cancel worker via Kratos
  `transport.Server`.
- Startup config validation fails fast on bad driver/secret/mode values.

### Testing and local setup

- SQLite support in `data/db` with auto-migration and idempotent seed.
- Time/ID GORM tags fixed for SQLite (`datetime`, non-unsigned primary keys).
- Unit tests for domain rules and sagas.
- SQLite integration tests for repositories, transactions, outbox, HTTP auth,
  RBAC fallback and admin APIs.
- `make test` requires no MySQL/etcd/DTM.

## 4. Trade-offs / known limitations

- Money uses `float64` because the existing schema uses `decimal` +
  `float64`. For high-precision billing, migrate to a decimal library / integer
  cents end-to-end.
- The login rate limiter and metrics registry are process-local. Use Redis and
  a metrics backend for multi-replica deployments.
- DTM discovery addresses are not probed at startup; a wrong VM network still
  fails at `Submit` (correctly, without panicking).
- DTM internal endpoints are unauthenticated by design for the demo; isolate
  them with network policy/mTLS before exposing them.
- The split services use separate SQLite files by default. Point them at the
  same MySQL database (or different schemas) in a real deployment.
- Casbin role codes are baked into the JWT; role changes take effect on the
  next login/token refresh.
- The outbox dispatcher is a single poller; handlers are idempotent, but a
  production deployment may want `FOR UPDATE SKIP LOCKED` or a queue.
