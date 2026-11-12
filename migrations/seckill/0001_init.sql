-- Seckill activity metadata. The actual stock bucket lives in mall_inventory;
-- this database owns the activity lifecycle and, in S2, qualifications.
CREATE TABLE IF NOT EXISTS seckill_activities (
  id BIGINT NOT NULL AUTO_INCREMENT,
  campaign_id BIGINT NOT NULL,
  sku_id BIGINT NOT NULL,
  warehouse_id BIGINT NOT NULL DEFAULT 1,
  name VARCHAR(128) NOT NULL,
  price_cent BIGINT NOT NULL,
  stock_total BIGINT NOT NULL,
  per_user_limit INT NOT NULL DEFAULT 1,
  start_at DATETIME(3) NOT NULL,
  end_at DATETIME(3) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'DRAFT',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_seckill_activity_sku (campaign_id, sku_id),
  KEY idx_seckill_activity_status (status, start_at, end_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reserved for S2. A qualification is a bounded right to create one order.
CREATE TABLE IF NOT EXISTS seckill_qualifications (
  id BIGINT NOT NULL AUTO_INCREMENT,
  activity_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  token_hash CHAR(64) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'ISSUED',
  order_no VARCHAR(64) NOT NULL DEFAULT '',
  expires_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_seckill_qualification_user (activity_id, user_id),
  UNIQUE KEY uk_seckill_qualification_token (token_hash),
  KEY idx_seckill_qualification_expiry (status, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
