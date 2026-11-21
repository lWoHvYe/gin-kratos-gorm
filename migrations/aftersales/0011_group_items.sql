-- Phase 5: after-sale lines and restock tasks carry the order fulfillment
-- group/warehouse. Existing rows backfill to G1/warehouse 1.
ALTER TABLE after_sale_items
  ADD COLUMN warehouse_id BIGINT NOT NULL DEFAULT 1 AFTER quantity,
  ADD COLUMN group_no VARCHAR(64) NOT NULL DEFAULT 'G1' AFTER warehouse_id,
  DROP INDEX uk_after_sale_items_sku,
  ADD UNIQUE KEY uk_after_sale_items_group (after_sale_id, sku_id, warehouse_id, group_no);

ALTER TABLE after_sale_restock_tasks
  ADD COLUMN warehouse_id BIGINT NOT NULL DEFAULT 1 AFTER sku_id,
  ADD COLUMN group_no VARCHAR(64) NOT NULL DEFAULT 'G1' AFTER warehouse_id,
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (after_sale_no, sku_id, warehouse_id, group_no);
