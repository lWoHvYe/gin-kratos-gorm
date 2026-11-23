-- ADR-0007 P0-A/P0-B: settlement identity, refund admission and durable tasks.
ALTER TABLE orders
  ADD COLUMN settlement_payment_no VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN inventory_ready_version BIGINT NOT NULL DEFAULT 0,
  ADD KEY idx_orders_settlement_payment (settlement_payment_no);

CREATE TABLE IF NOT EXISTS order_refund_admissions (
  refund_id VARCHAR(128) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  group_no VARCHAR(64) NOT NULL,
  result VARCHAR(32) NOT NULL DEFAULT 'PENDING_REMOTE',
  state VARCHAR(32) NOT NULL DEFAULT 'PENDING_REMOTE',
  amount_cent BIGINT NOT NULL DEFAULT 0,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (refund_id),
  KEY idx_order_refund_admissions_order_group (order_no, group_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS order_refund_commitments (
  id BIGINT NOT NULL AUTO_INCREMENT,
  order_no VARCHAR(64) NOT NULL,
  group_no VARCHAR(64) NOT NULL,
  refund_id VARCHAR(128) NOT NULL,
  amount_cent BIGINT NOT NULL,
  state VARCHAR(16) NOT NULL DEFAULT 'RESERVED',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_order_refund_commitment (order_no, group_no, refund_id),
  KEY idx_order_refund_commitments_state (order_no, state)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS pricing_confirmation_tasks (
  order_no VARCHAR(64) NOT NULL,
  state VARCHAR(16) NOT NULL DEFAULT 'PENDING',
  attempts INT NOT NULL DEFAULT 0,
  last_error VARCHAR(1024) NOT NULL DEFAULT '',
  payload JSON NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (order_no),
  KEY idx_pricing_confirmation_state (state)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
