-- Catalog read model for media, attributes and search.
-- MySQL 8 does not support ADD COLUMN IF NOT EXISTS, so these blocks use
-- information_schema checks to keep the migration re-runnable.

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'spus' AND COLUMN_NAME = 'min_price_cent');
SET @s := IF(@c = 0, 'ALTER TABLE spus ADD COLUMN min_price_cent BIGINT NOT NULL DEFAULT 0 AFTER status', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'skus' AND COLUMN_NAME = 'image_url');
SET @s := IF(@c = 0, 'ALTER TABLE skus ADD COLUMN image_url VARCHAR(512) NOT NULL DEFAULT '''' AFTER status', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS spu_images (
  id BIGINT NOT NULL AUTO_INCREMENT,
  spu_id BIGINT NOT NULL,
  url VARCHAR(512) NOT NULL,
  sort INT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_spu_images_spu (spu_id, sort, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS spu_attributes (
  id BIGINT NOT NULL AUTO_INCREMENT,
  spu_id BIGINT NOT NULL,
  name VARCHAR(128) NOT NULL,
  value VARCHAR(512) NOT NULL DEFAULT '',
  sort INT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_spu_attributes_spu (spu_id, sort, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

UPDATE spus s
SET min_price_cent = COALESCE((
  SELECT MIN(price_cent) FROM skus
  WHERE skus.spu_id = s.id AND skus.status = 'ENABLED'
), 0);

SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'spus' AND INDEX_NAME = 'idx_spus_status_category_price');
SET @s := IF(@i = 0, 'ALTER TABLE spus ADD INDEX idx_spus_status_category_price (status, category_id, min_price_cent)', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'skus' AND INDEX_NAME = 'idx_skus_status_price');
SET @s := IF(@i = 0, 'ALTER TABLE skus ADD INDEX idx_skus_status_price (status, price_cent)', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
