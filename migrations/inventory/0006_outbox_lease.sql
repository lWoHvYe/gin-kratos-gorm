-- Outbox lease columns: claim a batch in a short transaction, publish outside
-- it, and let an expired lease be reclaimed by another replica.
SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'outbox_events' AND COLUMN_NAME = 'lease_owner');
SET @s := IF(@c = 0, 'ALTER TABLE outbox_events ADD COLUMN lease_owner VARCHAR(128) NULL AFTER dead_lettered_at', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'outbox_events' AND COLUMN_NAME = 'lease_until');
SET @s := IF(@c = 0, 'ALTER TABLE outbox_events ADD COLUMN lease_until DATETIME(3) NULL AFTER lease_owner', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'outbox_events' AND INDEX_NAME = 'idx_outbox_lease');
SET @s := IF(@i = 0, 'ALTER TABLE outbox_events ADD INDEX idx_outbox_lease (published_at, dead_lettered_at, lease_until, next_attempt_at, id)', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
