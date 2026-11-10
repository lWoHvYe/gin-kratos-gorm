-- Partial refunds on the mock payment channel.

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_intents' AND COLUMN_NAME = 'captured_cent');
SET @s := IF(@c = 0, 'ALTER TABLE payment_intents ADD COLUMN captured_cent BIGINT NOT NULL DEFAULT 0 AFTER amount_cent', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_intents' AND COLUMN_NAME = 'refunded_cent');
SET @s := IF(@c = 0, 'ALTER TABLE payment_intents ADD COLUMN refunded_cent BIGINT NOT NULL DEFAULT 0 AFTER captured_cent', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_intents' AND COLUMN_NAME = 'payment_method');
SET @s := IF(@c = 0, 'ALTER TABLE payment_intents ADD COLUMN payment_method VARCHAR(32) NOT NULL DEFAULT '''' AFTER channel', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE payment_intents
SET captured_cent = amount_cent
WHERE captured_cent = 0 AND status IN ('SUCCEEDED', 'REFUNDED');

UPDATE payment_intents
SET refunded_cent = amount_cent
WHERE refunded_cent = 0 AND status = 'REFUNDED';

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refunds' AND COLUMN_NAME = 'idempotency_key');
SET @s := IF(@c = 0, 'ALTER TABLE refunds ADD COLUMN idempotency_key VARCHAR(128) NULL AFTER reason', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refunds' AND COLUMN_NAME = 'completed_at');
SET @s := IF(@c = 0, 'ALTER TABLE refunds ADD COLUMN completed_at DATETIME(3) NULL AFTER status', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refunds' AND INDEX_NAME = 'uk_refunds_idempotency');
SET @s := IF(@i = 0, 'ALTER TABLE refunds ADD UNIQUE KEY uk_refunds_idempotency (payment_no, idempotency_key)', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- The legacy (payment_no, amount_cent, reason) key prevents two legitimate
-- same-amount partial refunds. Idempotency is now explicit.
SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refunds' AND INDEX_NAME = 'uk_refunds_idem');
SET @s := IF(@i > 0, 'ALTER TABLE refunds DROP INDEX uk_refunds_idem', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT CHARACTER_MAXIMUM_LENGTH FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_intents' AND COLUMN_NAME = 'status');
SET @s := IF(@c < 32, 'ALTER TABLE payment_intents MODIFY COLUMN status VARCHAR(32) NOT NULL DEFAULT ''PENDING''', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT CHARACTER_MAXIMUM_LENGTH FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refunds' AND COLUMN_NAME = 'status');
SET @s := IF(@c < 32, 'ALTER TABLE refunds MODIFY COLUMN status VARCHAR(32) NOT NULL DEFAULT ''REFUNDED''', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
