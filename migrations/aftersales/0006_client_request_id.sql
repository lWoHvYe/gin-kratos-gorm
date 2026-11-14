-- Client idempotency key for after-sale creation. NULL keeps old rows valid and
-- allows many NULLs under the unique index.
SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'after_sales' AND COLUMN_NAME = 'client_request_id');
SET @s := IF(@c = 0, 'ALTER TABLE after_sales ADD COLUMN client_request_id VARCHAR(64) NULL AFTER reason', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'after_sales' AND INDEX_NAME = 'uk_after_sales_client_request');
SET @s := IF(@i = 0, 'ALTER TABLE after_sales ADD UNIQUE KEY uk_after_sales_client_request (user_id, client_request_id)', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
