-- Access tokens carry the user's token_version. Bumping it on a status change
-- invalidates already-issued tokens at the BFFs' high-risk checks.
SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'token_version');
SET @s := IF(@c = 0, 'ALTER TABLE users ADD COLUMN token_version BIGINT NOT NULL DEFAULT 0', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
