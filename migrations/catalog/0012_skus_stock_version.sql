-- Catalog.skus.stock is a projection. Track the inventory version that produced
-- it so an older stock.changed event cannot overwrite a newer projection.
SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'skus' AND COLUMN_NAME = 'stock_version');
SET @s := IF(@c = 0, 'ALTER TABLE skus ADD COLUMN stock_version BIGINT NOT NULL DEFAULT 0', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
