-- Refund state is orthogonal to fulfilment. Keep the order lifecycle
-- (PAID -> SHIPPED -> COMPLETED) intact while exposing refund progress.
SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'refund_status');
SET @s := IF(@c = 0, 'ALTER TABLE orders ADD COLUMN refund_status VARCHAR(32) NOT NULL DEFAULT '''' AFTER refunded_cent', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE orders
SET refund_status = CASE
  WHEN refunded_cent <= 0 THEN ''
  WHEN refunded_cent >= paid_cent THEN 'REFUNDED'
  ELSE 'PARTIALLY_REFUNDED'
END
WHERE refund_status = '';
