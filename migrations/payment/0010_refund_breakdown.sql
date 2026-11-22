-- ADR 0005 §6.5/§11: goods and freight refunds are separate facts for the
-- daily equation ② and for auditor reconstruction.

ALTER TABLE refunds
  ADD COLUMN goods_cent BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN freight_cent BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN rule_id VARCHAR(32) NOT NULL DEFAULT '';

UPDATE refunds SET goods_cent = amount_cent WHERE goods_cent = 0;

CREATE TABLE IF NOT EXISTS payment_reconciliation_runs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  service VARCHAR(32) NOT NULL,
  run_date DATE NOT NULL,
  checked BIGINT NOT NULL DEFAULT 0,
  diffs BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_payment_reconciliation_run (service, run_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS payment_reconciliation_diffs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  run_date DATE NOT NULL,
  equation VARCHAR(32) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  expected_cent BIGINT NOT NULL DEFAULT 0,
  actual_cent BIGINT NOT NULL DEFAULT 0,
  diff_cent BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_payment_reconciliation_diff (run_date, equation, order_no),
  KEY idx_payment_reconciliation_diff_run (run_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
