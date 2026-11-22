# Cross-warehouse split orders: dev readiness review

Scope: ADR [0002](adr/0002-cross-warehouse-split-orders.md), phases 1-6,
commits through the phase-6 UI change. This is a **dev** review; production
rollout is still gated.

## Architecture

| Concern | Owner | Evidence |
|---|---|---|
| Warehouse allocation | Inventory | read-only `AllocateOrder`, lowest warehouse with full line stock |
| Quote grouping/freight | Pricing | `quote_groups`, per-group flat freight |
| Order lifecycle/groups | Order | `order_fulfillment_groups`, one `G<n>` per warehouse |
| Stock/reservations | Inventory | reservation/operation keys include `warehouse_id` |
| Shipping permits | Order | `(order_no, fulfillment_group_no)` primary key |
| Parcels | Fulfillment | shipments unique `(order_no, fulfillment_group_no)` |
| Returns/refunds | After-sales | item group/warehouse, origin restock |
| Actions/UI | Gateway/BFF | group-suffixed actions, storefront/admin group views |

- Service code does not import another service package; `make arch` passes.
- Cross-service effects use transactional outbox events; no cross-database
  joins.
- `WAREHOUSE_SPLIT_ENABLED` is false in production values and true only in the
  dev single-node values. Split requires the allocation path; gateway now uses
  the allocator when either shadow or split is enabled.

## Design invariants checked

1. Split is line-level only; a line quantity is never divided across warehouses.
2. Every order line has `(warehouse_id, group_no)`; pricing groups and order
   groups use the same group numbers.
3. Inventory reservation/operation identity is warehouse-aware.
4. One shipping permit and one shipment per group; a permit cannot be consumed
   by a different group.
5. Order status stays aggregate (`ALLOCATED`/`SHIPPED`/`COMPLETED`);
   `shipment_status` is the partial projection.
6. A refund blocks/revokes only its own group permit. Another group remains
   shippable while one group is refunding; only a fully refunded order blocks
   all shipping.
7. A return-refund is validated against the group's `SHIPPED`/`RECEIVED`
   state, not only the aggregate order status.
8. One after-sale request belongs to one group; split orders must send items so
   the group is explicit.
9. Restock tasks carry the origin warehouse and stay idempotent by after-sale
   number.

## Findings fixed during this review

| Finding | Fix |
|---|---|
| Split enabled without shadow skipped allocations and the cart stock check | gateway uses `shadow || split` for allocation and stock precheck |
| `GetShipmentByOrder` silently picked G1 for a multi-parcel order | returns FAILED_PRECONDITION; caller must pass `shipment_no` |
| A permit could be requested for a non-existent group | `ensureGroupExistsTx` before authorize/block (non-G1) |
| A refunding G2 could hide/ship-block G1 through order-level `refund_state` | group actions use permit state; authorize blocks only `FULL` |
| Fulfillment accepted a shipment with the wrong group/warehouse/items | `CreateShipment` validates against Order groups and lines |
| Split after-sale without items defaulted to G1 | split orders require explicit after-sale items |
| `REFUND_ONLY` was rejected before shipment because the group status check was shared with returns | refund-only is allowed for an unshipped group; only RETURN_REFUND requires SHIPPED/RECEIVED |
| An unshipped refunded group kept the order in ALLOCATED | a REVOKED permit makes the group closed for aggregate/projection purposes |
| Group refund progress was only implicit in permits | `order_fulfillment_groups.refund_state` is projected from permit blocking and `aftersale.completed` |
| Refunded draft parcels stayed CREATED | Fulfillment consumes `aftersale.refund_requested` and moves CREATED -> CANCELLED |
| Race tests across packages shared one MySQL database | `test-race -p 1` |
| Split E2E used Bash's readonly `GROUPS` array | renamed to `GROUP_COUNT` |

## Verification executed

- `make verify` -> `verify: OK`, proto baseline 741.
- `make test-integration` -> all listed MySQL/NATS suites `ok`.
- `go test -race -p 1` on order/fulfillment/after-sales/admin -> `ok`.
- `scripts/e2e-phase15.sh` -> `phase15 e2e passed`.
- `scripts/e2e-warehouse-split.sh` -> two quote groups, two parcels,
  `COMPLETED/RECEIVED`.
- `frontend` typecheck/build -> pass.
- Live AI mesh: read allowed, Catalog CreateProduct PermissionDenied.
- Playwright -> 9 tests, including
  [warehouse-split.spec.ts](../frontend/e2e/warehouse-split.spec.ts) and
  [admin-split.spec.ts](../frontend/e2e/admin-split.spec.ts): group UI, G2
  receive, admin G2 shipment.
- `scripts/check-split-concurrency.sh`: duplicate payment, concurrent G1/G2
  ship, draft cancellation, refund/ship race.
- `scripts/fault-split-uat.sh`: NATS and Fulfillment outage recovery.
- `make alerts-check`: four after-sale backlog rules.
- `make test-integration` now derives `mall_integration_<package>`; packages no
  longer share one test database.

## Not covered / production gates

- Dev fixture migration/rollback rehearsal passed (disposable MySQL, pre-C
  dumps, current migrate image). A production backup/restore and old-image
  smoke are still required; production values keep split disabled.
- No high-load/chaos run for split checkout; only targeted correctness races.
- Admin group-shipment UI is covered by Playwright.
- No production-profile secret/payment configuration; UAT uses dev/mock.
- A separate k3s UAT stack was installed on 2026-11-22 with isolated
  `mall-uat-data`; single E2E, split E2E, split Playwright, admin split,
  concurrency and group refund passed. It uses dev secrets/mock payment and is
  not a production profile.
- Payment multi-intent aggregation and the A/B production rollback rehearsal
  remain separate gates.

## Verdict

C is **functionally complete and enabled in dev and UAT**. The isolated UAT
stack passed fresh migration, single/split E2E, admin/storefront split UI,
targeted concurrency, draft cancellation and group refund. Production remains
intentionally off because there is no large-scale production rollout; the
runbook and hardened migration runner are retained for the eventual change
window.
