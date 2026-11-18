-- Qualification status changes are compare-and-set; track a version.
SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'seckill_qualifications' AND COLUMN_NAME = 'version');
SET @s := IF(@c = 0, 'ALTER TABLE seckill_qualifications ADD COLUMN version BIGINT NOT NULL DEFAULT 1', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
