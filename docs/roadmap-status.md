# P0-P3 roadmap status

Audit date: 2026-11-22. This is an honest status snapshot, not a claim that every
P0-P3 item is done.

**Objective status: NOT complete.** The goal was previously marked complete for
a "necessary subset"; that was premature and is retracted. The items marked
partial/not done below are real gaps, not optional polish, whenever they border
money, inventory, recovery or production operation.

## P0: C production gate

| Item | Status | Evidence / gap |
|---|---|---|
| P0.1 migration/rollback ADR + rehearsal | done | [ADR 0003](adr/0003-c-production-rollout.md), [runbook](runbooks/c-migration-rollback.md), `scripts/rehearse-c-migration.sh`; six C ledger entries and pre-C restore passed on disposable MySQL |
| P0.1b migration runner hardening | done | lease + attempt ledger + failure refusal in `deploy/k8s/images/migrate/run.sh`; `scripts/test-migrate-runner.sh` covers failure, repair, replay, concurrency, stale lease |
| P0.2 split concurrency / fault injection | partial | `check-split-concurrency.sh` covers duplicate payment, concurrent G1/G2 ship, draft cancellation, refund/ship race. `fault-split-uat.sh` covers Order outage fail-closed, NATS outage and Fulfillment outage recovery. Worker restart remains uncovered |
| P0.3 admin group shipment Playwright | done | [admin-split.spec.ts](../frontend/e2e/admin-split.spec.ts) creates a G2 shipment through the admin UI |
| P0.4 group/permit/restock metrics and alerts | partial | backlog/oldest gauges exist; four alert rules pass `make alerts-check`, but no live Prometheus firing test |
| P0.5 production split gate | done (guard) | production values keep split off; dev/UAT use `WAREHOUSE_MODE=split` |

## P1: fulfillment state/action consolidation

| Item | Status | Evidence / gap |
|---|---|---|
| P1.1 `GroupFulfillmentView` | done | `Order.group_actions`, effective `CANCELLED` group status and closed-group projection are the read model; `orderActionPolicy` is the single action path |
| P1.2 typed action policy | done | proto `OrderGroupAction`; `orderActionPolicy.groupActions/legacyActions` replaces the duplicated action functions; UI reads `group_actions` |
| P1.3 explicit group refund state | rejected by design | `orders.refund_state` is the aggregate projection; group refund admission/isolation uses the authoritative permit state plus the after-sale amount ledger. A second group refund projection would duplicate state without an action consumer |
| P1.4 shipment void + draft cleanup | done | `CancelCreatedShipment`, `shop.aftersale.refund_requested.v1` consumer, `CREATED -> CANCELLED`, auto-shipment action guard |
| P1.5 `WAREHOUSE_MODE` | done | config + chart single switch; legacy booleans remain compatible; conflicts fail closed |
| P1.6 test DB isolation + split fixture | done | `scripts/test-dsn.sh` gives each integration package its own DB; `scripts/create-split-fixture.sh` creates and publishes two fresh SKUs stocked in different warehouses |

## P2: production/contracts

| Item | Status | Evidence / gap |
|---|---|---|
| Production backup/restore, A/B/C rollback | deferred | C fixture/UAT migration and restore rehearsed; no production-sized backup or A/B production rollback because there is no large-scale production |
| Multi payment intent captured aggregation | done | `GetPaymentByOrder` sums settled intents; `RefundOrder` allocates across captured intents with order-level idempotency; `order_refunded_cent` keeps the Order projection correct; integration tests cover aggregation and retry |
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

## Strict definition of done (for reopening)

A task may be marked done only when all of the following hold:

1. The invariant is stated and tested for success, failure, retry, duplicate and
   restart paths; a happy-path E2E is not enough.
2. The code has one authoritative state/action path; compatibility shims are
   explicitly temporary and have removal criteria.
3. Recovery is durable and observable: metrics, alerts, replay/repair procedure.
4. Migration/rollback is rehearsed on a production-like profile, or the item is
   explicitly labelled "not production-ready" and cannot be counted as done.
5. UAT with dev secrets/mock payment is labelled UAT, never production.

Under this definition, P0.2 (worker restart), P0.4 (live alert firing), P2
replay/events/contracts/#38 and all P3 remain open. P1.1/P1.2/P1.6 and P2
payment aggregation are done; P1.3 is rejected by design.

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
2. Add the remaining worker-restart fault injection (P0.2 gap) and a live
   Prometheus alert-firing test (P0.4 gap).
3. Consolidate `availableActions`/`multiGroupActions` behind one action policy and
   decide whether a per-group refund projection is worth persisting.
4. Only then consider P2 payment aggregation/replay and P3 product/AI work.
