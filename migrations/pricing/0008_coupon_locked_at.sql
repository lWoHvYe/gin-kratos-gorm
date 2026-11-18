-- Track when a coupon was locked so abandoned locks can be swept. Without
-- this, a process crash between LOCKED and UNLOCKED/USED leaves the coupon
-- unavailable forever.
SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'coupons' AND COLUMN_NAME = 'locked_at');
SET @s := IF(@c = 0, 'ALTER TABLE coupons ADD COLUMN locked_at DATETIME(3) NULL AFTER order_no', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE coupons SET locked_at = updated_at
WHERE status = 'LOCKED' AND locked_at IS NULL;

SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'coupons' AND INDEX_NAME = 'idx_coupons_locked');
SET @s := IF(@i = 0, 'ALTER TABLE coupons ADD INDEX idx_coupons_locked (status, locked_at)', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
