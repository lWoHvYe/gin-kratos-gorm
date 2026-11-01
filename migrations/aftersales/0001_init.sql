CREATE TABLE IF NOT EXISTS after_sales (
  id BIGINT NOT NULL AUTO_INCREMENT,
  after_sale_no VARCHAR(64) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  `type` VARCHAR(32) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'REQUESTED',
  amount_cent BIGINT NOT NULL,
  reason VARCHAR(512) NOT NULL DEFAULT '',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_after_sales_no (after_sale_no),
  KEY idx_after_sales_order (order_no),
  KEY idx_after_sales_user (user_id),
  KEY idx_after_sales_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS after_sale_items (
  id BIGINT NOT NULL AUTO_INCREMENT,
  after_sale_id BIGINT NOT NULL,
  sku_id BIGINT NOT NULL,
  quantity INT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_after_sale_items_sku (after_sale_id, sku_id),
  KEY idx_after_sale_items_after_sale (after_sale_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS after_sale_logs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  after_sale_id BIGINT NOT NULL,
  action VARCHAR(32) NOT NULL,
  from_status VARCHAR(16) NOT NULL DEFAULT '',
  to_status VARCHAR(16) NOT NULL,
  remark VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_after_sale_logs_after_sale (after_sale_id)
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
