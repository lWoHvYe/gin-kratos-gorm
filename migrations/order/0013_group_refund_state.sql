-- Phase 5/P1: explicit per-group refund projection. The Order service owns
-- this read model; refund admission itself remains the permit + after-sale
-- amount ledger.
SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'order_fulfillment_groups'
    AND COLUMN_NAME = 'refund_state');
SET @s := IF(@c = 0,
  'ALTER TABLE order_fulfillment_groups ADD COLUMN refund_state VARCHAR(16) NOT NULL DEFAULT ''NONE'' AFTER status',
  'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE order_fulfillment_groups g
JOIN order_shipping_permits p
  ON p.order_no = g.order_no AND p.fulfillment_group_no = g.group_no
SET g.refund_state = 'REFUNDING'
WHERE p.state = 'REVOKED' AND g.status IN ('PENDING', 'ALLOCATED');
