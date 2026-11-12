-- Address snapshot captured at qualification time and the command publication
-- marker used by the S3 JetStream worker/republisher.
SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='seckill_qualifications' AND COLUMN_NAME='address_id');
SET @s := IF(@c=0, 'ALTER TABLE seckill_qualifications ADD COLUMN address_id BIGINT NOT NULL DEFAULT 0 AFTER user_id', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='seckill_qualifications' AND COLUMN_NAME='receiver_name');
SET @s := IF(@c=0, "ALTER TABLE seckill_qualifications ADD COLUMN receiver_name VARCHAR(128) NOT NULL DEFAULT '' AFTER address_id", 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='seckill_qualifications' AND COLUMN_NAME='receiver_phone');
SET @s := IF(@c=0, "ALTER TABLE seckill_qualifications ADD COLUMN receiver_phone VARCHAR(32) NOT NULL DEFAULT '' AFTER receiver_name", 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='seckill_qualifications' AND COLUMN_NAME='receiver_address');
SET @s := IF(@c=0, "ALTER TABLE seckill_qualifications ADD COLUMN receiver_address VARCHAR(512) NOT NULL DEFAULT '' AFTER receiver_phone", 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='seckill_qualifications' AND COLUMN_NAME='published_at');
SET @s := IF(@c=0, 'ALTER TABLE seckill_qualifications ADD COLUMN published_at DATETIME(3) NULL AFTER updated_at', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
