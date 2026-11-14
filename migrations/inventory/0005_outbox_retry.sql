-- Outbox retry governance: exponential backoff, dead-lettering and pruning.
-- Idempotent like the rest of the migrations; every service database has the
-- same outbox_events shape.

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'outbox_events' AND COLUMN_NAME = 'next_attempt_at');
SET @s := IF(@c = 0, 'ALTER TABLE outbox_events ADD COLUMN next_attempt_at DATETIME(3) NULL AFTER attempts', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'outbox_events' AND COLUMN_NAME = 'dead_lettered_at');
SET @s := IF(@c = 0, 'ALTER TABLE outbox_events ADD COLUMN dead_lettered_at DATETIME(3) NULL AFTER published_at', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'outbox_events' AND INDEX_NAME = 'idx_outbox_retry');
SET @s := IF(@i = 0, 'ALTER TABLE outbox_events ADD INDEX idx_outbox_retry (published_at, dead_lettered_at, next_attempt_at, id)', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
