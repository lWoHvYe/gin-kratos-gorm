-- Payment service schema (mall_payment).
--
-- Payments are mocked: no channel SDK is involved, pay_url points at the
-- in-process mock handler and callbacks are plain gRPC calls.

CREATE TABLE IF NOT EXISTS payment_intents (
  id BIGINT NOT NULL AUTO_INCREMENT,
  payment_no VARCHAR(64) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL DEFAULT 0,
  amount_cent BIGINT NOT NULL,
  channel VARCHAR(32) NOT NULL DEFAULT 'mock',
  status VARCHAR(16) NOT NULL DEFAULT 'PENDING',
  pay_url VARCHAR(255) NOT NULL DEFAULT '',
  expires_at DATETIME(3) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_payment_intents_no (payment_no),
  -- CreateIntent is idempotent by (order_no, amount_cent, channel).
  UNIQUE KEY uk_payment_intents_idem (order_no, amount_cent, channel),
  KEY idx_payment_intents_user (user_id),
  KEY idx_payment_intents_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS payment_callbacks (
  id BIGINT NOT NULL AUTO_INCREMENT,
  channel VARCHAR(32) NOT NULL,
  event_id VARCHAR(128) NOT NULL,
  payment_no VARCHAR(64) NOT NULL,
  amount_cent BIGINT NOT NULL,
  status VARCHAR(16) NOT NULL,
  processed_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  -- A channel event is delivered at most once to the business.
  UNIQUE KEY uk_payment_callbacks_event (channel, event_id),
  KEY idx_payment_callbacks_payment (payment_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS refunds (
  id BIGINT NOT NULL AUTO_INCREMENT,
  refund_no VARCHAR(64) NOT NULL,
  payment_no VARCHAR(64) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  amount_cent BIGINT NOT NULL,
  reason VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'REFUNDED',
  channel VARCHAR(32) NOT NULL DEFAULT '',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_refunds_no (refund_no),
  -- Refund is idempotent by (payment_no, amount_cent, reason).
  UNIQUE KEY uk_refunds_idem (payment_no, amount_cent, reason),
  KEY idx_refunds_order (order_no)
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

-- DTM creates this on demand; keeping it here makes fresh databases ready.
CREATE TABLE IF NOT EXISTS dtm_barrier (
  id BIGINT NOT NULL AUTO_INCREMENT,
  trans_type VARCHAR(45) NOT NULL DEFAULT '',
  gid VARCHAR(128) NOT NULL DEFAULT '',
  branch_id VARCHAR(128) NOT NULL DEFAULT '',
  op VARCHAR(45) NOT NULL DEFAULT '',
  barrier_id VARCHAR(45) NOT NULL DEFAULT '',
  reason VARCHAR(45) NOT NULL DEFAULT '',
  create_time DATETIME(6) NULL DEFAULT NULL,
  update_time DATETIME(6) NULL DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uid_barrier (trans_type, gid, branch_id, op, barrier_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
