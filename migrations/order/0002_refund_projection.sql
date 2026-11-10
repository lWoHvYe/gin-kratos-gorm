-- Refund amount projected from payment.refunded.v1.
SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'refunded_cent');
SET @s := IF(@c = 0, 'ALTER TABLE orders ADD COLUMN refunded_cent BIGINT NOT NULL DEFAULT 0 AFTER paid_cent', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @c := (SELECT CHARACTER_MAXIMUM_LENGTH FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'status');
SET @s := IF(@c < 32, 'ALTER TABLE orders MODIFY COLUMN status VARCHAR(32) NOT NULL', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
