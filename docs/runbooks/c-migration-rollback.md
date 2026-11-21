# C production migration and rollback rehearsal

Use this with [ADR 0003](../adr/0003-c-production-rollout.md). The automated
dev rehearsal is `scripts/rehearse-c-migration.sh`; it uses disposable MySQL
containers and never touches the live database.

## 1. Freeze and inventory

- Freeze checkout/fulfillment writes for the change window.
- Record image digests for every service and frontend.
- Record current `schema_migrations` rows and row counts for the five affected
  databases.
- Confirm the production split flag is false.

## 2. Backup

Use a consistent dump per database:

```bash
for db in mall_order mall_aftersales mall_inventory mall_pricing mall_fulfillment; do
  mysqldump --single-transaction --routines --triggers --no-create-db "$db" > "backup/$db.sql"
  sha256sum "backup/$db.sql" >> backup/SHA256SUMS
done
```

Store the backup outside the cluster and verify one table can be read from it.

## 3. Clone rehearsal

- Restore the five dumps into a disposable MySQL instance.
- Build the current migration image (`mall/migrate:dev` in dev).
- Run the image with `MYSQL_HOST`/`MYSQL_PORT` pointing at the clone, not the
  production service.
- Verify the C ledger entries and these invariants:

```sql
-- order groups/items
SELECT COUNT(*) FROM orders o
LEFT JOIN order_fulfillment_groups g ON g.order_no = o.order_no
WHERE g.id IS NULL;                                            -- expect 0
SELECT COUNT(*) FROM order_items i
LEFT JOIN order_fulfillment_groups g
  ON g.order_no = i.order_no AND g.group_no = i.fulfillment_group_no
WHERE g.id IS NULL OR g.warehouse_id <> i.warehouse_id;        -- expect 0
SELECT COUNT(*) FROM order_shipping_permits p
LEFT JOIN order_fulfillment_groups g
  ON g.order_no = p.order_no AND g.group_no = p.fulfillment_group_no
WHERE g.id IS NULL;                                            -- expect 0

-- after-sales group/restock
SELECT COUNT(*) FROM after_sale_items WHERE warehouse_id IS NULL OR group_no = ''; -- expect 0
SELECT COUNT(*) FROM after_sale_restock_tasks
WHERE warehouse_id IS NULL OR group_no = '';                   -- expect 0

-- pricing/fulfillment
SELECT COUNT(*) FROM quote_lines l
LEFT JOIN quote_groups g ON g.quote_id = l.quote_id AND g.group_no = l.group_no
WHERE g.id IS NULL;                                            -- expect 0
SELECT COUNT(*) FROM (
  SELECT order_no, fulfillment_group_no FROM shipments
  GROUP BY order_no, fulfillment_group_no HAVING COUNT(*) > 1
) d;                                                           -- expect 0
```

- Run `scripts/e2e-warehouse-split.sh` against a canary stack using the clone
  data, not production traffic.
- Run the refund isolation script/manual case: ship G2, refund G2, then ship G1.

## 4. Production apply

1. Run the hardened `mall-migrate` job with the advisory lock and attempt
   ledger.
2. Verify the job is complete and every C file is `APPLIED`.
3. Roll services in the ADR order; keep split false.
4. Wait for outbox lag and error rate to return to baseline.
5. Run single E2E plus a read-only split smoke (no flag change).
6. Only after sign-off, enable split in a separate change window.

## 5. Rollback

- Stop new writes and scale application deployments to zero if data correctness
  is in doubt.
- Restore the five pre-migration dumps into a fresh MySQL instance/clone first;
  validate ledger/schema.
- Point the previous image revision at the restored database (or restore in
  place with the database stopped).
- Do **not** run `ALTER TABLE ... DROP` down scripts; they are not maintained.
- Verify old code starts and passes order/payment/after-sales smoke.
- If the restore point is after `RETURN_RECEIVED`, keep the forward code and
  roll forward instead of forcing old code.

## 6. Evidence to record

- Backup paths and SHA256.
- Clone migration log and ledger count.
- Invariant query results.
- Split/single/Playwright results.
- Image digests before/after.
- Any manual data repair, with order/group IDs.

## 7. Dev fixture rehearsal result

The first rehearsal (2026-11-22) used `/tmp/mall-predeploy` pre-C dumps, the
current `mall/migrate:dev` image and disposable MySQL 8.4 containers:

```bash
BACKUP_DIR=/tmp/mall-predeploy MIGRATE_IMAGE=mall/migrate:dev \
  ./scripts/rehearse-c-migration.sh
```

Results:

- all six C ledger entries applied;
- zero missing order groups, item/group/warehouse mismatches, orphan permits,
  duplicate group shipments, or missing after-sale/restock group;
- inventory reservation and fulfillment shipment unique indexes present;
- a second clone restored to the pre-C schema and had zero C ledger rows.

This proves the SQL/backfill and restore path on fixture data. It does **not**
prove production-volume timing, a production backup, or an old-image smoke run;
those remain required before production split is enabled.
