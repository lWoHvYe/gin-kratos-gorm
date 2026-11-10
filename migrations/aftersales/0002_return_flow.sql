SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'after_sales' AND COLUMN_NAME = 'return_carrier');
SET @s := IF(@c = 0, 'ALTER TABLE after_sales ADD COLUMN return_carrier VARCHAR(64) NOT NULL DEFAULT '''' AFTER reason', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'after_sales' AND COLUMN_NAME = 'return_tracking_no');
SET @s := IF(@c = 0, 'ALTER TABLE after_sales ADD COLUMN return_tracking_no VARCHAR(128) NOT NULL DEFAULT '''' AFTER return_carrier', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'after_sales' AND COLUMN_NAME = 'refund_no');
SET @s := IF(@c = 0, 'ALTER TABLE after_sales ADD COLUMN refund_no VARCHAR(64) NOT NULL DEFAULT '''' AFTER return_tracking_no', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'after_sales' AND COLUMN_NAME = 'refunded_cent');
SET @s := IF(@c = 0, 'ALTER TABLE after_sales ADD COLUMN refunded_cent BIGINT NOT NULL DEFAULT 0 AFTER refund_no', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'after_sale_items' AND COLUMN_NAME = 'received_quantity');
SET @s := IF(@c = 0, 'ALTER TABLE after_sale_items ADD COLUMN received_quantity INT NOT NULL DEFAULT 0 AFTER quantity', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'after_sale_items' AND COLUMN_NAME = 'condition_code');
SET @s := IF(@c = 0, 'ALTER TABLE after_sale_items ADD COLUMN condition_code VARCHAR(16) NOT NULL DEFAULT ''SELLABLE'' AFTER received_quantity', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
