CREATE TABLE IF NOT EXISTS categories (
  id BIGINT NOT NULL AUTO_INCREMENT,
  parent_id BIGINT NOT NULL DEFAULT 0,
  name VARCHAR(128) NOT NULL,
  level INT NOT NULL DEFAULT 1,
  sort INT NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'ENABLED',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_categories_parent (parent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS brands (
  id BIGINT NOT NULL AUTO_INCREMENT,
  name VARCHAR(128) NOT NULL,
  logo VARCHAR(512) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'ENABLED',
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS spus (
  id BIGINT NOT NULL AUTO_INCREMENT,
  category_id BIGINT NOT NULL,
  brand_id BIGINT NOT NULL DEFAULT 0,
  name VARCHAR(255) NOT NULL,
  subtitle VARCHAR(255) NOT NULL DEFAULT '',
  description TEXT NOT NULL,
  main_image VARCHAR(512) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'DRAFT',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_spus_category (category_id),
  KEY idx_spus_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS skus (
  id BIGINT NOT NULL AUTO_INCREMENT,
  spu_id BIGINT NOT NULL,
  sku_code VARCHAR(64) NOT NULL,
  specs_json JSON NOT NULL,
  price_cent BIGINT NOT NULL,
  stock BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'ENABLED',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_skus_code (sku_code),
  KEY idx_skus_spu (spu_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS outbox_events (
  id BIGINT NOT NULL AUTO_INCREMENT,
  event_id VARCHAR(64) NOT NULL,
  topic VARCHAR(128) NOT NULL,
  aggregate_type VARCHAR(64) NOT NULL,
  aggregate_id VARCHAR(64) NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  payload JSON NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  last_error VARCHAR(1024) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  published_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_outbox_event_id (event_id),
  KEY idx_outbox_pending (published_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Minimal catalog for local smoke tests.
INSERT IGNORE INTO categories (id, parent_id, name, level, sort, status)
SELECT * FROM (SELECT 1 AS id, 0 AS parent_id, 'Keyboards' AS name, 1 AS level, 1 AS sort, 'ENABLED' AS status
  UNION ALL SELECT 2, 0, 'Books', 1, 2, 'ENABLED') AS seed
WHERE NOT EXISTS (SELECT 1 FROM categories);
INSERT IGNORE INTO brands (id, name, logo, status)
SELECT * FROM (SELECT 1 AS id, 'Kratos' AS name, '' AS logo, 'ENABLED' AS status
  UNION ALL SELECT 2, 'Go Press', '', 'ENABLED') AS seed
WHERE NOT EXISTS (SELECT 1 FROM brands);
INSERT IGNORE INTO spus (id, category_id, brand_id, name, subtitle, description, main_image, status)
SELECT * FROM (SELECT 1 AS id, 1 AS category_id, 1 AS brand_id, 'Kratos Keyboard' AS name,
  '87-key hot-swappable' AS subtitle, 'A mechanical keyboard for developers.' AS description,
  '' AS main_image, 'ON_SALE' AS status
  UNION ALL SELECT 2, 2, 2, 'Go in Action', 'Cloud-native microservices', 'A practical Go book.', '', 'ON_SALE') AS seed
WHERE NOT EXISTS (SELECT 1 FROM spus);
INSERT IGNORE INTO skus (id, spu_id, sku_code, specs_json, price_cent, stock, status)
SELECT * FROM (SELECT 1 AS id, 1 AS spu_id, 'KB-RED-87' AS sku_code, '{"switch":"red"}' AS specs_json, 39900 AS price_cent, 100 AS stock, 'ENABLED' AS status
  UNION ALL SELECT 2, 1, 'KB-BROWN-87', '{"switch":"brown"}', 42900, 80, 'ENABLED'
  UNION ALL SELECT 3, 2, 'BOOK-GO-01', '{"binding":"paperback"}', 8900, 200, 'ENABLED'
  UNION ALL SELECT 4, 2, 'BOOK-GO-02', '{"binding":"hardcover"}', 12900, 50, 'ENABLED') AS seed
WHERE NOT EXISTS (SELECT 1 FROM skus);
