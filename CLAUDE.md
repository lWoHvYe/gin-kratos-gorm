# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
this repository.

## Project Overview

`gin-kratos-gorm` is a production-oriented Kratos v2 sample. It ships as:

- `cmd/server`: a modular monolith (user + product + order + background worker)
- `cmd/order`: an optional split order service
- `cmd/product`: an optional split product service

It uses Gin for REST, Kratos for gRPC/lifecycle, GORM (MySQL or SQLite),
Wire, JWT and Casbin. Distributed transactions are **pluggable**: local
transactions/compensation are the default and need no external DTM; DTM is an
opt-in mode for split deployments.

## Essential Commands

```bash
make init          # install protoc plugins, wire and helpers
make api           # regenerate proto/gRPC/HTTP/OpenAPI code
make config        # regenerate internal/conf
make wire          # regenerate Wire code (bootstrap + order + product)
make build         # build ./bin/{server,order,product}
make run-local     # monolith with SQLite, local mode, no infra
make run           # monolith with configs/config.yaml
make run-order     # split order service (:8001/:9001)
make run-product   # split product service (:8002/:9002)
make test          # unit + SQLite integration tests (no DTM/MySQL/etcd)
make test-race
make vet
make migrate       # apply migrations/001_init.sql to MySQL
make compose-up    # MySQL/Redis/etcd only
```

## Layout

```text
api/                 protobuf APIs (user, product, order)
cmd/server           monolith composition root + Wire
cmd/order            split order service + Wire
cmd/product          split product service + Wire
internal/biz         domain use cases, interfaces, sagas, errors
internal/biz/model   GORM models (generated + hand-maintained fields)
internal/data        repositories, DB bootstrap/seed, Casbin, outbox dispatcher
internal/service     proto adapters
internal/server      Gin HTTP, Kratos gRPC, handlers, routers, middleware, worker
internal/pkg         config, auth, apperr, httpx, idgen, metrics, registry, grpcclient
internal/conf        protobuf config
migrations/          MySQL schema + seed
configs/             local/MySQL/split/DTM configurations
```

## Key Design Rules

- Domain (`internal/biz`) must not import Gin, gRPC transports or GORM.
  Repositories/catalogs are interfaces in `biz`, implemented in `internal/data`.
- `internal/data` returns `apperr` errors (NotFound/Conflict/...).
- Services return `apperr`; HTTP handlers map via `internal/pkg/httpx`;
  gRPC uses `GRPCErrorMapper` + `GRPCAuth` middleware.
- Orders are created through a `biz.Saga`:
  - monolith: `TxSaga` (one DB transaction)
  - split order: `CompensationSaga` (remote stock + local order)
  - DTM: `service.DtmCoordinator` (opt-in)
- Stock mutations must stay idempotent through `stock_logs` keys:
  `deduct:<order_sn>:<sku_id>` / `restore:<order_sn>:<sku_id>`.
- Domain events must be written in the same transaction as the state change
  (`outbox_events`), then dispatched by the worker.
- Never use `dtmgrpc.MustGenGid`; DTM GIDs are derived from the order number.
- Never hard-code DB DSNs, JWT secrets or VM IPs. Use config/env overrides.

## Configuration

- `configs/config.local.yaml` — SQLite + local saga, no DTM (default for dev)
- `configs/config.yaml` — alias of local config
- `configs/server.yaml` — MySQL + etcd monolith
- `configs/order.yaml`, `configs/product.yaml` — split services
- `configs/order.dtm.yaml` — split order + DTM

Important env overrides: `DATABASE_DRIVER`, `DATABASE_SOURCE`, `JWT_SECRET`,
`TRANSACTION_MODE`, `DTM_ADDRESS`, `DTM_ENABLED`, `DTM_FALLBACK_TO_LOCAL`,
`ETCD_ENDPOINTS`, `PRODUCT_ENDPOINT`.

## DTM / Integration Tests

- Default `data.transaction.mode: local`; tests and `make run-local` never
  require DTM, MySQL or etcd.
- DTM in a VM Docker network may be unreachable. Keep local mode, or fix
  routing and use `configs/order.dtm.yaml`.
- `fallback_to_local: true` only affects startup; a failed `Submit` returns
  `UNAVAILABLE` rather than silently retrying locally.

## Adding a Service / Provider

1. Add/update `api/<service>/v1/*.proto`, then `make api`.
2. Implement domain interfaces/use cases in `internal/biz`.
3. Implement repositories in `internal/data/persistence`.
4. Add a proto adapter in `internal/service`.
5. Add transports/handlers/routes in `internal/server`.
6. Add providers to the relevant provider sets (`biz.ProviderSet`,
   `data.ProviderSet`, `service.ProviderSet`, `server.TransportSet`) and Wire
   composition roots.
7. Run `make wire`, `make build`, `make test`, `make vet`.

## Important Notes

- All timestamps are stored/returned as RFC3339 strings at the API boundary.
- GORM time tags intentionally omit `type:datetime(3)` so SQLite can scan
  time values; MySQL still defaults to `datetime(3)`.
- The outbox dispatcher and auto-cancel worker are Kratos
  `transport.Server`s, so they participate in graceful shutdown.
- DTM internal endpoints (`CreateOrderInner`, `CancelOrder`, `ReduceStock`,
  `CompensateStock`) are unauthenticated and must be network-isolated.
