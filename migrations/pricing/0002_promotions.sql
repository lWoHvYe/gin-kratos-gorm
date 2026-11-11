-- Promotion engine: campaigns (activity containers), rules (conditions/effects),
-- budget reservations and an auditable budget ledger.

CREATE TABLE IF NOT EXISTS promotion_campaigns (
  id BIGINT NOT NULL AUTO_INCREMENT,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  type VARCHAR(32) NOT NULL DEFAULT 'NORMAL',
  status VARCHAR(16) NOT NULL DEFAULT 'DRAFT',
  start_at DATETIME(3) NOT NULL,
  end_at DATETIME(3) NOT NULL,
  priority INT NOT NULL DEFAULT 0,
  budget_cent BIGINT NOT NULL DEFAULT 0,
  reserved_cent BIGINT NOT NULL DEFAULT 0,
  consumed_cent BIGINT NOT NULL DEFAULT 0,
  per_user_limit INT NOT NULL DEFAULT 0,
  stack_mode VARCHAR(16) NOT NULL DEFAULT 'PRIORITY',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_campaign_code (code),
  KEY idx_campaign_status_time (status, start_at, end_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS promotion_rules (
  id BIGINT NOT NULL AUTO_INCREMENT,
  campaign_id BIGINT NOT NULL,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  type VARCHAR(32) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'DRAFT',
  priority INT NOT NULL DEFAULT 0,
  exclusive TINYINT(1) NOT NULL DEFAULT 0,
  stack_group VARCHAR(64) NOT NULL DEFAULT '',
  scope_json JSON NOT NULL,
  condition_expr VARCHAR(1024) NOT NULL DEFAULT '',
  effect_json JSON NOT NULL,
  max_discount_cent BIGINT NOT NULL DEFAULT 0,
  budget_cent BIGINT NOT NULL DEFAULT 0,
  reserved_cent BIGINT NOT NULL DEFAULT 0,
  consumed_cent BIGINT NOT NULL DEFAULT 0,
  per_user_limit INT NOT NULL DEFAULT 0,
  allow_with_coupon TINYINT(1) NOT NULL DEFAULT 1,
  start_at DATETIME(3) NOT NULL,
  end_at DATETIME(3) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_rule_code (code),
  KEY idx_rule_campaign (campaign_id),
  KEY idx_rule_status_time (status, start_at, end_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS promotion_reservations (
  id BIGINT NOT NULL AUTO_INCREMENT,
  reservation_no VARCHAR(64) NOT NULL,
  quote_id VARCHAR(64) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  rule_id BIGINT NOT NULL,
  campaign_id BIGINT NOT NULL,
  amount_cent BIGINT NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'HELD',
  idempotency_key VARCHAR(160) NOT NULL,
  expires_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_reservation_no (reservation_no),
  UNIQUE KEY uk_reservation_idempotency (idempotency_key),
  KEY idx_reservation_order (order_no),
  KEY idx_reservation_status (status, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS promotion_reservation_items (
  id BIGINT NOT NULL AUTO_INCREMENT,
  reservation_id BIGINT NOT NULL,
  sku_id BIGINT NOT NULL,
  discount_cent BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  KEY idx_reservation_items_reservation (reservation_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS promotion_budget_ledger (
  id BIGINT NOT NULL AUTO_INCREMENT,
  reservation_no VARCHAR(64) NOT NULL,
  rule_id BIGINT NOT NULL,
  campaign_id BIGINT NOT NULL,
  amount_cent BIGINT NOT NULL,
  entry_type VARCHAR(16) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_budget_ledger_reservation_entry (reservation_no, entry_type),
  KEY idx_budget_ledger_campaign (campaign_id),
  KEY idx_budget_ledger_rule (rule_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'quotes' AND COLUMN_NAME = 'promotion_discount_cent');
SET @s := IF(@c = 0, 'ALTER TABLE quotes ADD COLUMN promotion_discount_cent BIGINT NOT NULL DEFAULT 0 AFTER discount_cent', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'quotes' AND COLUMN_NAME = 'coupon_discount_cent');
SET @s := IF(@c = 0, 'ALTER TABLE quotes ADD COLUMN coupon_discount_cent BIGINT NOT NULL DEFAULT 0 AFTER promotion_discount_cent', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'quotes' AND COLUMN_NAME = 'applied_promotions_json');
SET @s := IF(@c = 0, 'ALTER TABLE quotes ADD COLUMN applied_promotions_json JSON NULL AFTER coupon_id', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'quotes' AND COLUMN_NAME = 'promotion_version');
SET @s := IF(@c = 0, 'ALTER TABLE quotes ADD COLUMN promotion_version BIGINT NOT NULL DEFAULT 0 AFTER applied_promotions_json', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;
