# P0-P3 roadmap status

Audit date: 2026-11-22. This is an honest status snapshot, not a claim that every
P0-P3 item is done. The "necessary subset" objective is complete; the gaps below
remain optional/deferred unless a production rollout is approved.

## P0: C production gate

| Item | Status | Evidence / gap |
|---|---|---|
| P0.1 migration/rollback ADR + rehearsal | done | [ADR 0003](adr/0003-c-production-rollout.md), [runbook](runbooks/c-migration-rollback.md), `scripts/rehearse-c-migration.sh`; six C ledger entries and pre-C restore passed on disposable MySQL |
| P0.1b migration runner hardening | done | lease + attempt ledger + failure refusal in `deploy/k8s/images/migrate/run.sh`; `scripts/test-migrate-runner.sh` covers failure, repair, replay, concurrency, stale lease |
| P0.2 split concurrency / fault injection | partial | `scripts/check-split-concurrency.sh` covers duplicate payment, concurrent G1/G2 ship, draft cancellation, refund/ship race on dev and UAT. NATS/Order outage and worker-restart fault injection are not covered |
| P0.3 admin group shipment Playwright | done | [admin-split.spec.ts](../frontend/e2e/admin-split.spec.ts) creates a G2 shipment through the admin UI |
| P0.4 group/permit/restock metrics and alerts | partial | backlog/oldest gauges exist for restock/refund and `/metrics` was checked. No Prometheus alert rules for them yet |
| P0.5 production split gate | done (guard) | production values keep split off; dev/UAT use `WAREHOUSE_MODE=split` |

## P1: fulfillment state/action consolidation

| Item | Status | Evidence / gap |
|---|---|---|
| P1.1 `GroupFulfillmentView` | partial | `Order.group_actions`, effective `CANCELLED` status and closed-group projection exist; there is no single named `GroupFulfillmentView` type yet |
| P1.2 typed action policy | partial | proto `OrderGroupAction`, typed `groupAction`, UI reads `group_actions`; `availableActions` and `multiGroupActions` are still two functions |
| P1.3 explicit group refund state | not done | `orders.refund_state` remains order-level. Group isolation is enforced via permit state and effective status; no per-group refund projection column |
| P1.4 shipment void + draft cleanup | done | `CancelCreatedShipment`, `shop.aftersale.refund_requested.v1` consumer, `CREATED -> CANCELLED`, auto-shipment action guard |
| P1.5 `WAREHOUSE_MODE` | done | config + chart single switch; legacy booleans remain compatible; conflicts fail closed |
| P1.6 test DB isolation + split fixture | partial | `scripts/test-dsn.sh` gives each integration package its own DB. Split Playwright/shell still relies on prepared demo SKUs/stock, not a self-seeding fixture |

## P2: production/contracts

| Item | Status | Evidence / gap |
|---|---|---|
| Production backup/restore, A/B/C rollback | deferred | C fixture/UAT migration and restore rehearsed; no production-sized backup or A/B production rollback because there is no large-scale production |
| Multi payment intent captured aggregation | not done | `intentByOrder` returns the latest settled intent (`ORDER BY id DESC LIMIT 1`); multiple captured intents are not summed |
| B inventory replay gRPC/BFF | partial | `cmd/inventory op-replay` CLI exists; no gRPC/BFF replay surface |
| #38 AI mesh live 403 | partial | Helm render has read-only AI policies; live 403 verification was not run |
| Typed outbox event schema | partial | core refund/shipment consumers have local structs including `group_no`; many outbox payloads remain `map[string]any`/JSON |
| OpenAPI/proto/TS consistency | partial | generated TS + hand-written overlays; no automated proto/OpenAPI conformance gate |
| Migration runner lock/failure marking | done | see P0.1b |

## P3: deferred product/AI work

Not started:

- AI Turn/Action protocol
- RAG versioned release/recovery work beyond the existing dev RAG
- quantity-level warehouse split
- warehouse cost/optimization allocator
- carrier integration
- multi-warehouse seckill

## Verification snapshot

```text
make verify                 OK, proto baseline 734
make test-integration       per-package DBs, all listed suites ok
go test -race -p 1          order/fulfillment/aftersales/admin ok
dev Playwright              9 passed
UAT                         single E2E, split E2E, split UI, admin split,
                            concurrency and group refund passed
C                           functional dev+UAT; production split off
```

## Recommended next order

1. Add alerts for the after-sale backlog gauges (small).
2. Add split fault injection for NATS/Order outage and worker restart (P0.2 gap).
3. Consolidate `availableActions`/`multiGroupActions` behind one action policy and
   decide whether a per-group refund projection is worth persisting.
4. Only then consider P2 payment aggregation/replay and P3 product/AI work.
