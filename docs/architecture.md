# Architecture decisions

## Dependency injection: Wire, incrementally

The project now uses Wire for composition roots where the graph is non-trivial:

- `cmd/gateway`: registry, downstream clients, auth manager, Gin engine, Kratos
  HTTP server and app.
- `cmd/order`: DB, NATS, registry, pricing/inventory gRPC connections, DTM
  client, service, gRPC/HTTP servers and app.

Generated files are committed:

```text
cmd/gateway/wire.go       # //go:build wireinject
cmd/gateway/wire_gen.go   # generated, committed
cmd/order/wire.go
cmd/order/wire_gen.go
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
every service. The two adopted roots have the highest fan-in (gateway clients,
order clients/DTM), so they get the compile-time graph checks first.

## Data access: database/sql core, GORM read side

The transactional core stays on `database/sql` and explicit SQL:

- DTM Saga barrier requires a `*sql.Tx` that also owns outbox/ledger writes.
- Inventory/order/coupon use conditional `UPDATE ... WHERE ...` and
  `RowsAffected` for compare-and-set.
- Outbox relay uses `FOR UPDATE SKIP LOCKED`.
- Idempotency relies on unique keys and MySQL-specific upserts.

GORM is used where it is a net win without weakening those guarantees:

- `internal/services/catalog/gorm_repo.go` is the catalog **read side**:
  product/SKU association loading, keyword/category filtering, count + page.
- It opens GORM on the existing `*sql.DB` with
  `SkipInitializeWithVersion: true`, so there is one connection pool.
- Catalog writes (`CreateProduct`, `SetProductStatus`) remain explicit
  `*sql.Tx` + `outbox.Append`; the cache invalidation path is unchanged.
- If the GORM reader cannot initialize, catalog falls back to the original
  `database/sql` reads instead of failing startup.

This is deliberately not an ORM-ification of order/inventory/payment/pricing.
Those services are state machines around conditional SQL and DTM; hiding their
transactions behind GORM would increase risk without reducing the hard parts.

Future GORM candidates are low-contention CRUD/reporting surfaces (admin
reporting, warehouse lists), not the transaction core. Future Wire candidates
are payment/admin once their graphs grow.

## Testing

`make verify` runs fmt, vet, unit tests, build, architecture guard and DTM
contract check. `make test-race` runs the race detector. Wire generation is a
separate explicit step (`make wire`) because it rewrites committed generated
files.
