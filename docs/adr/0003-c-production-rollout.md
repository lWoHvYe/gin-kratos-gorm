# ADR 0003: Production rollout and rollback for cross-warehouse orders

- Status: accepted; migration-runner hardening pending
- Date: 2026-11-22
- Related: [ADR 0002](0002-cross-warehouse-split-orders.md),
  [C review](../c-warehouse-review.md)

## Context

Cross-warehouse C adds schema and data backfills across five services:

| Service | Migrations |
|---|---|
| Order | `0010_settlement_protocol.sql`, `0011_fulfillment_groups.sql`, `0012_group_shipping_permits.sql` |
| Inventory | `0016_reservation_warehouse_keys.sql` |
| Pricing | `0010_quote_warehouse.sql` |
| After-sales | `0010_settlement_protocol.sql`, `0011_group_items.sql` |
| Fulfillment | `0003_group_shipments.sql` |

Some changes are one-way data renames (`RECEIVED -> RETURN_RECEIVED`) and
column/primary-key replacements. Down SQL is not a supported rollback: MySQL
DDL commits implicitly and old code cannot safely resume a half-migrated
after-sales/fulfillment dataset.

The current Helm migration job applies files with a checksum ledger but has no
cross-pod advisory lock and no persisted in-flight/failed attempt state. A
crash after DDL but before the ledger insert would replay a non-idempotent
migration.

## Decision

1. **Forward-only migrations.** Production rollback is restore-from-backup plus
   the previous Helm/image revision. We do not write or run down SQL.
2. **Rehearse on a disposable MySQL clone first.** Use the real pre-C dumps,
   run the same `mall-migrate` image, and verify backfills/invariants and the
   restore path.
3. **Harden the migration runner before production:**
   - acquire `GET_LOCK('mall_migrate', 300)` and release it in a trap;
   - persist an attempt row (`filename`, checksum, `RUNNING/APPLIED/FAILED`);
   - mark `RUNNING` before executing, `APPLIED` after, `FAILED` on error;
   - a `FAILED` attempt fails closed and requires operator inspection/restore;
   - keep the existing checksum refusal for edited applied files.
4. **Keep split disabled through migration and application rollout.** The
   production value is `WAREHOUSE_SPLIT_ENABLED=false`; after migration and a
   canary, enable split only with an explicit change window.
5. **Deployment order:**
   1. production backup and clone rehearsal;
   2. Helm migration hook;
   3. Order, Inventory, After-sales, Fulfillment, Payment (compatibility order);
   4. Gateway, Admin;
   5. storefront/admin frontends;
   6. health/E2E checks;
   7. optional split flag enablement after the gate.
6. **Rollback triggers** (any one):
   - migration job fails or leaves `FAILED`;
   - order group/item/warehouse invariant non-zero;
   - fulfillment group shipment duplicate/orphan;
   - after-sales restock/refund task cannot converge;
   - split E2E or payment/refund regression in canary;
   - error rate or outbox lag above the release threshold.
7. **Do not roll back application code alone** after `RETURN_RECEIVED` has been
   written. Restore the database backup to a compatible point, or roll forward.

## Acceptance

- Clone preflight: zero missing groups, zero item/group/warehouse mismatches,
  zero orphan permits, zero duplicate group shipments.
- Same migration image runs cleanly on the clone; ledger has all C files.
- Restore into a second clone returns the pre-C schema and ledger; old image
  starts and passes a smoke test.
- Production backup is restorable and its checksum/size is recorded.
- Split E2E, single E2E, refund isolation and admin group shipment pass.

Dev rehearsal on 2026-11-22 used pre-C dumps in `/tmp/mall-predeploy`, the
current `mall/migrate:dev` image and disposable MySQL 8.4 containers. All six
C ledger entries and invariants passed; a second clone restored to the pre-C
schema (no `order_items.fulfillment_group_no`, no `order_fulfillment_groups`,
no C ledger rows). This is fixture-level evidence, not a production-volume
backup or an old-image smoke test.

## Consequences

- Production rollout takes a maintenance/change window; restore rollback is
  slower than a down migration but is the only safe option for one-way data.
- The migration-runner hardening is a prerequisite, not an optimization.
- Split remains a product flag, not a migration side effect.
