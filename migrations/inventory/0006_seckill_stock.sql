-- Dedicated seckill stock bucket. The total is carved out of inventory_stocks
-- by increasing inventory_stocks.reserved (quantities are units), so normal
-- sales cannot consume the bucket.
CREATE TABLE IF NOT EXISTS seckill_stocks (
  activity_id BIGINT NOT NULL,
  sku_id BIGINT NOT NULL,
  warehouse_id BIGINT NOT NULL DEFAULT 1,
  total BIGINT NOT NULL,
  available BIGINT NOT NULL,
  held BIGINT NOT NULL DEFAULT 0,
  sold BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'ALLOCATED',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (activity_id),
  KEY idx_seckill_stocks_sku (sku_id, warehouse_id),
  CONSTRAINT ck_seckill_stocks_positive CHECK (total >= 0 AND available >= 0 AND held >= 0 AND sold >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS seckill_stock_ledger (
  id BIGINT NOT NULL AUTO_INCREMENT,
  activity_id BIGINT NOT NULL,
  delta BIGINT NOT NULL,
  balance_after BIGINT NOT NULL,
  reason VARCHAR(32) NOT NULL,
  ref_no VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_seckill_ledger_ref (activity_id, reason, ref_no),
  KEY idx_seckill_ledger_activity (activity_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
