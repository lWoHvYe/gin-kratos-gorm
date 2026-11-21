-- Phase 4: one shipment per order fulfillment group. Existing single-warehouse
-- shipments backfill to G1.
ALTER TABLE shipments
  ADD COLUMN fulfillment_group_no VARCHAR(64) NOT NULL DEFAULT 'G1' AFTER order_no,
  DROP INDEX uk_shipments_order,
  ADD UNIQUE KEY uk_shipments_order_group (order_no, fulfillment_group_no);
