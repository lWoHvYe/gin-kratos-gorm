# Architecture decisions

## Dependency injection: Wire, incrementally

Kratos runtime is v3; see `docs/kratos-v3.md` for migration details. The project
now uses Wire for composition roots where the graph is non-trivial:

- `cmd/gateway`: downstream clients, auth manager, Gin engine, Kratos HTTP
  server and app.
- `cmd/admin`: admin clients, auth manager, Gin engine, Kratos HTTP server and
  app.
- `cmd/order`: DB, NATS, pricing/inventory gRPC connections, DTM client,
  service, gRPC/HTTP servers and app.
- `cmd/payment`: DB, NATS, order/inventory/pricing gRPC connections, DTM/TCC
  configuration, service, gRPC/HTTP servers and app.

Generated files are committed:

```text
cmd/gateway/wire.go       # //go:build wireinject
cmd/gateway/wire_gen.go   # generated, committed
cmd/admin/wire.go
cmd/admin/wire_gen.go
cmd/order/wire.go
cmd/order/wire_gen.go
cmd/payment/wire.go
cmd/payment/wire_gen.go
```

Regenerate with:

```bash
make wire
```

The remaining small services keep the explicit `main.go` composition because
their graph is shallow and the startup/cleanup order is easier to audit there.
Wire can be added per service without changing service constructors; the
provider set is the only new code.

Why not Wire everywhere at once: it adds generated files and provider sets to
every service. The four adopted roots have the highest fan-in (BFF clients,
order/payment clients and DTM), so they get the compile-time graph checks first.

## Data access: database/sql core, GORM read side

The transactional core stays on `database/sql` and explicit SQL:

- DTM Saga barrier requires a `*sql.Tx` that also owns outbox/ledger writes.
- Inventory/order/coupon use conditional `UPDATE ... WHERE ...` and
  `RowsAffected` for compare-and-set.
- Outbox relay uses `FOR UPDATE SKIP LOCKED`.
- Idempotency relies on unique keys and MySQL-specific upserts.

GORM is used where it is a net win without weakening those guarantees
(catalog and cart reads):

- `internal/services/catalog/gorm_repo.go` is the catalog **read side**:
  product/SKU association loading, keyword/category filtering, count + page.
- `internal/services/cart/gorm_repo.go` is the cart **read side**: one
  user → cart → items query with association preload.
- It opens GORM on the existing `*sql.DB` with
  `SkipInitializeWithVersion: true`, so there is one connection pool.
- Catalog writes (`CreateProduct`, `SetProductStatus`) remain explicit
  `*sql.Tx` + `outbox.Append`; the cache invalidation path is unchanged.
- If a GORM reader cannot initialize, catalog/cart fall back to the original
  `database/sql` reads instead of failing startup.

This is deliberately not an ORM-ification of order/inventory/payment/pricing.
Those services are state machines around conditional SQL and DTM; hiding their
transactions behind GORM would increase risk without reducing the hard parts.

Future GORM candidates are low-contention CRUD/reporting surfaces (admin
reporting, warehouse lists), not the transaction core. Payment has now joined
gateway/admin/order on Wire; the next candidates are only services whose graphs
grow beyond the current shallow shape.

## Testing

`make verify` runs fmt, vet, unit tests, build, architecture guard and DTM
contract check. `make test-race` runs the race detector. Wire generation is a
separate explicit step (`make wire`) because it rewrites committed generated
files.
