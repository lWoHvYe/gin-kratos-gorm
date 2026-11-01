-- mall_order: orders, order items, status audit log and the transactional outbox.
-- The order service writes orders and outbox events inside a DTM saga branch
-- barrier transaction, so the schema stays append-only friendly.

CREATE TABLE IF NOT EXISTS orders (
  id BIGINT NOT NULL AUTO_INCREMENT,
  order_no VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'PENDING_PAYMENT',
  total_cent BIGINT NOT NULL DEFAULT 0,
  discount_cent BIGINT NOT NULL DEFAULT 0,
  freight_cent BIGINT NOT NULL DEFAULT 0,
  pay_cent BIGINT NOT NULL DEFAULT 0,
  paid_cent BIGINT NOT NULL DEFAULT 0,
  receiver_name VARCHAR(64) NOT NULL DEFAULT '',
  receiver_phone VARCHAR(32) NOT NULL DEFAULT '',
  receiver_address VARCHAR(255) NOT NULL DEFAULT '',
  quote_id VARCHAR(64) NOT NULL DEFAULT '',
  client_request_id VARCHAR(64) NOT NULL DEFAULT '',
  expire_at DATETIME(3) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_orders_order_no (order_no),
  UNIQUE KEY uk_orders_user_client_request (user_id, client_request_id),
  KEY idx_orders_user_created (user_id, created_at),
  KEY idx_orders_status_created (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS order_items (
  id BIGINT NOT NULL AUTO_INCREMENT,
  order_no VARCHAR(64) NOT NULL,
  sku_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL DEFAULT '',
  price_cent BIGINT NOT NULL DEFAULT 0,
  quantity INT NOT NULL DEFAULT 0,
  discount_cent BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_order_items_order (order_no, id),
  KEY idx_order_items_sku (sku_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS order_status_logs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  order_no VARCHAR(64) NOT NULL,
  from_status VARCHAR(32) NOT NULL DEFAULT '',
  to_status VARCHAR(32) NOT NULL,
  reason VARCHAR(255) NOT NULL DEFAULT '',
  operator VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_order_status_logs_order (order_no, id)
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

-- DTM creates this schema lazily; the DDL is repeated here so a fresh database
-- can serve the CreateOrderInner / CancelOrderInner barriers immediately.
CREATE DATABASE IF NOT EXISTS dtm_barrier CHARACTER SET utf8mb4;

CREATE TABLE IF NOT EXISTS dtm_barrier.barrier (
  id BIGINT NOT NULL AUTO_INCREMENT,
  trans_type VARCHAR(45) NOT NULL DEFAULT '',
  gid VARCHAR(128) NOT NULL DEFAULT '',
  branch_id VARCHAR(128) NOT NULL DEFAULT '',
  op VARCHAR(45) NOT NULL DEFAULT '',
  barrier_id VARCHAR(45) NOT NULL DEFAULT '',
  reason VARCHAR(45) NOT NULL DEFAULT '',
  create_time DATETIME(3) NULL,
  update_time DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_barrier (gid, branch_id, op, barrier_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
