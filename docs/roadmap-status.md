# P0-P3 roadmap status

Audit date: 2026-11-22. This is an honest status snapshot, not a claim that every
P0-P3 item is done.

**Objective status: the necessary P0/P1/P2 slice is complete.** Full legacy
event typing, a complete OpenAPI/proto/TS schema conformance gate and all P3
work remain explicitly deferred, not silently counted as done.

## P0: C production gate

| Item | Status | Evidence / gap |
|---|---|---|
| P0.1 migration/rollback ADR + rehearsal | done | [ADR 0003](adr/0003-c-production-rollout.md), [runbook](runbooks/c-migration-rollback.md), `scripts/rehearse-c-migration.sh`; six C ledger entries and pre-C restore passed on disposable MySQL |
| P0.1b migration runner hardening | done | lease + attempt ledger + failure refusal in `deploy/k8s/images/migrate/run.sh`; `scripts/test-migrate-runner.sh` covers failure, repair, replay, concurrency, stale lease |
| P0.2 split concurrency / fault injection | done | `check-split-concurrency.sh` covers duplicate payment, concurrent G1/G2 ship, draft cancellation, refund/ship race. `fault-split-uat.sh` covers Order outage fail-closed, NATS/Fulfillment outage recovery. `fault-split-worker-uat.sh` stops the after-sales worker with a PROCESSING restock task, restarts it, and verifies restock/refund/group state |
| P0.3 admin group shipment Playwright | done | [admin-split.spec.ts](../frontend/e2e/admin-split.spec.ts) creates a G2 shipment through the admin UI |
| P0.4 group/permit/restock metrics and alerts | done | backlog/oldest gauges exist; four alert rules pass structural + `promtool test rules` firing tests via `make alerts-check` |
| P0.5 production split gate | done (guard) | production values keep split off; dev/UAT use `WAREHOUSE_MODE=split` |

## P1: fulfillment state/action consolidation

| Item | Status | Evidence / gap |
|---|---|---|
| P1.1 `GroupFulfillmentView` | done | `Order.group_actions`, effective `CANCELLED` group status and closed-group projection are the read model; `orderActionPolicy` is the single action path |
| P1.2 typed action policy | done | proto `OrderGroupAction`; `orderActionPolicy.groupActions/legacyActions` replaces the duplicated action functions; UI reads `group_actions` |
| P1.3 explicit group refund state | done | `order_fulfillment_groups.refund_state` is projected from `BlockShipmentForRefund` (`REFUNDING`) and `aftersale.completed.v1` (`REFUNDED`); permit remains the shipping authority and `orders.refund_state` remains the aggregate projection |
| P1.4 shipment void + draft cleanup | done | `CancelCreatedShipment`, `shop.aftersale.refund_requested.v1` consumer, `CREATED -> CANCELLED`, auto-shipment action guard |
| P1.5 `WAREHOUSE_MODE` | done | config + chart single switch; legacy booleans remain compatible; conflicts fail closed |
| P1.6 test DB isolation + split fixture | done | `scripts/test-dsn.sh` gives each integration package its own DB; `scripts/create-split-fixture.sh` creates and publishes two fresh SKUs stocked in different warehouses |

## P2: production/contracts

| Item | Status | Evidence / gap |
|---|---|---|
| Production backup/restore, A/B/C rollback | deferred | C fixture/UAT migration and restore rehearsed; no production-sized backup or A/B production rollback because there is no large-scale production |
| Multi payment intent captured aggregation | done | `GetPaymentByOrder` sums settled intents; `RefundOrder` allocates across captured intents with order-level idempotency; `order_refunded_cent` keeps the Order projection correct; integration tests cover aggregation and retry |
| B inventory replay gRPC/BFF | done | CLI plus `ReplayInventoryOperation` gRPC and admin `POST /inventory/operations/:id/replay`; integration test and live 404 route check |
| #38 AI mesh live 403 | done | live UAT check: AI-sa ListProducts succeeds, CreateProduct returns PermissionDenied; `scripts/check-ai-mesh.sh` |
| Typed outbox event schema | partial | new refund-requested/completed/payment-refunded payloads use typed `internal/platform/events` structs and a contract test; legacy events remain maps. This is the necessary group/refund/payment subset; full legacy migration is deferred |
| OpenAPI/proto/TS consistency | partial | `make contract-check` runs proto guard, typed event tests and admin/gateway OpenAPI route tests; generated TS still has hand-written overlays and no full schema conformance |
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

Under this definition, the necessary P0/P1/P2 slice is complete. Remaining
explicitly deferred: full legacy-event typing, complete OpenAPI/proto/TS schema
conformance and all P3 work.

## Verification snapshot

```text
make verify                 OK, proto baseline 744
make test-integration       per-package DBs, all listed suites ok
go test -race -p 1          order/fulfillment/aftersales/admin ok
dev Playwright              9 passed
UAT                         single E2E, split E2E, split UI, admin split,
                            concurrency and group refund passed
C                           functional dev+UAT; production split off
```

## Recommended next order

1. Add live Prometheus firing coverage in CI (promtool already passes).
2. Migrate remaining legacy outbox events to typed structs and add a complete
   proto/OpenAPI/TS conformance gate.
3. P3 only after those contract/governance items close.
