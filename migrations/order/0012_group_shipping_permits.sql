-- Phase 4: shipping permits are per fulfillment group. Existing permits
-- backfill to G1.
ALTER TABLE order_shipping_permits
  ADD COLUMN fulfillment_group_no VARCHAR(64) NOT NULL DEFAULT 'G1' AFTER order_no,
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (order_no, fulfillment_group_no);
