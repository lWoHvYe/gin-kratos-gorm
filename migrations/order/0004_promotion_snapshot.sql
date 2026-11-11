SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'promotion_discount_cent');
SET @s := IF(@c = 0, 'ALTER TABLE orders ADD COLUMN promotion_discount_cent BIGINT NOT NULL DEFAULT 0 AFTER discount_cent', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'applied_promotions_json');
SET @s := IF(@c = 0, 'ALTER TABLE orders ADD COLUMN applied_promotions_json JSON NULL AFTER promotion_discount_cent', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
