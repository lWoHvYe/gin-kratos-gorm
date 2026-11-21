-- Warehouse-C phase 3: reservation and operation identity include the
-- warehouse. Existing rows are all warehouse 1 and remain valid.
ALTER TABLE stock_reservations
  DROP INDEX uk_reservations_order_sku,
  ADD UNIQUE KEY uk_reservations_order_sku_warehouse (order_no, sku_id, warehouse_id);

ALTER TABLE inventory_operation_items
  ADD COLUMN warehouse_id BIGINT NOT NULL DEFAULT 1 AFTER sku_id,
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (operation_id, sku_id, warehouse_id);

ALTER TABLE inventory_operation_items
  ADD KEY idx_inventory_operation_item_warehouse (warehouse_id, status);
