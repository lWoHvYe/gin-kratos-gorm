SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='seckill_qualifications' AND COLUMN_NAME='payment_no');
SET @s := IF(@c=0, "ALTER TABLE seckill_qualifications ADD COLUMN payment_no VARCHAR(64) NOT NULL DEFAULT '' AFTER order_no", 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
