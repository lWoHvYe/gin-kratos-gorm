-- One order-stock mapping per qualification token (ref_no).
--
-- The order service derives a deterministic order number from the same token,
-- so redelivery is already idempotent by (order_no, sku_id). This unique key is
-- the database-level backstop for any other caller. If historical duplicates
-- exist the index is skipped; resolve them and re-run the migration.
SET @d := (SELECT COUNT(*) FROM (
  SELECT activity_id, ref_no FROM seckill_order_stock
  GROUP BY activity_id, ref_no HAVING COUNT(*) > 1
) AS dup);

SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'seckill_order_stock'
    AND INDEX_NAME = 'uk_seckill_order_ref');
SET @s := IF(@d = 0 AND @i = 0,
  'ALTER TABLE seckill_order_stock ADD UNIQUE KEY uk_seckill_order_ref (activity_id, ref_no)',
  'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
