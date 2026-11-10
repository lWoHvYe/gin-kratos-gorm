-- Address book defaults and lookup support.
SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_addresses' AND INDEX_NAME = 'idx_addresses_user_default');
SET @s := IF(@i = 0, 'ALTER TABLE user_addresses ADD INDEX idx_addresses_user_default (user_id, is_default)', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
