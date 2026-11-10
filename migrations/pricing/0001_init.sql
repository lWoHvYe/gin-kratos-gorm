-- pricing service schema (mall_pricing)

CREATE TABLE IF NOT EXISTS coupons (
  id BIGINT NOT NULL AUTO_INCREMENT,
  code VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL DEFAULT 0,
  amount_cent BIGINT NOT NULL,
  min_amount_cent BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'AVAILABLE',
  expire_at DATETIME(3) NOT NULL,
  order_no VARCHAR(64) NOT NULL DEFAULT '',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_coupons_code_user (code, user_id),
  KEY idx_coupons_status (status),
  KEY idx_coupons_order (order_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS quotes (
  quote_id VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  total_cent BIGINT NOT NULL,
  discount_cent BIGINT NOT NULL DEFAULT 0,
  freight_cent BIGINT NOT NULL DEFAULT 0,
  pay_cent BIGINT NOT NULL,
  coupon_id VARCHAR(64) NOT NULL DEFAULT '',
  expires_at DATETIME(3) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (quote_id),
  KEY idx_quotes_user (user_id),
  KEY idx_quotes_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS quote_lines (
  quote_id VARCHAR(64) NOT NULL,
  sku_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL DEFAULT '',
  price_cent BIGINT NOT NULL,
  quantity INT NOT NULL,
  discount_cent BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (quote_id, sku_id)
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


-- Public welcome coupon: 10.00 off when the order total reaches 50.00.
INSERT INTO coupons (code, user_id, amount_cent, min_amount_cent, status, expire_at)
SELECT 'WELCOME10', 0, 1000, 5000, 'AVAILABLE', '2037-12-31 23:59:59'
FROM DUAL
WHERE NOT EXISTS (
  SELECT 1 FROM coupons WHERE code = 'WELCOME10' AND user_id = 0
);
