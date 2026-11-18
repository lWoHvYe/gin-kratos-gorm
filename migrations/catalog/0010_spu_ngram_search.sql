-- Substring search used leading-wildcard LIKE and could not use an index.
-- MySQL's ngram parser supports CJK and substring queries via MATCH.
SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'spus' AND INDEX_NAME = 'ft_spus_name_subtitle');
SET @s := IF(@i = 0,
  'ALTER TABLE spus ADD FULLTEXT INDEX ft_spus_name_subtitle (name, subtitle) WITH PARSER ngram',
  'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
