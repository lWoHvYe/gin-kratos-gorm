-- ADR-0007 P0-A: capture and settlement are separated by a durable task.
CREATE TABLE IF NOT EXISTS payment_settlement_tasks (
  payment_no VARCHAR(64) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  state VARCHAR(16) NOT NULL DEFAULT 'PENDING',
  attempts INT NOT NULL DEFAULT 0,
  lease_owner VARCHAR(64) NOT NULL DEFAULT '',
  lease_until DATETIME(3) NULL,
  dtm_gid VARCHAR(128) NOT NULL DEFAULT '',
  dtm_status VARCHAR(32) NOT NULL DEFAULT '',
  last_error VARCHAR(1024) NOT NULL DEFAULT '',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (payment_no),
  KEY idx_payment_settlement_state (state, lease_until),
  KEY idx_payment_settlement_order (order_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Non-effective captures are refunded but never counted in the order's
-- captured/refunded facts (ADR-0007 D-07).
CREATE TABLE IF NOT EXISTS payment_duplicate_captures (
  payment_no VARCHAR(64) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  amount_cent BIGINT NOT NULL,
  refund_id VARCHAR(128) NOT NULL,
  state VARCHAR(16) NOT NULL DEFAULT 'PENDING',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (payment_no),
  UNIQUE KEY uk_payment_duplicate_refund (refund_id),
  KEY idx_payment_duplicate_order (order_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
