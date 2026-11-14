-- Persist the producing trace context so the relay can link its publish span
-- and consumers can continue the trace across the asynchronous hop.
SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'outbox_events' AND COLUMN_NAME = 'traceparent');
SET @s := IF(@c = 0, 'ALTER TABLE outbox_events ADD COLUMN traceparent VARCHAR(256) NULL AFTER lease_until', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
