-- ADR 0005 phase E: the line refund ledger is the only S14 fact source.
-- Order never stores refunded quantities or amounts.

ALTER TABLE after_sale_items
  ADD COLUMN line_no INT NOT NULL DEFAULT 0,
  ADD COLUMN line_paid_cent_snapshot BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN unit_paid_cent_snapshot BIGINT NOT NULL DEFAULT 0,
  DROP INDEX uk_after_sale_items_group,
  ADD UNIQUE KEY uk_after_sale_items_line (after_sale_id, line_no),
  ADD KEY idx_after_sale_items_group (after_sale_id, sku_id, warehouse_id, group_no);

-- Historical line association needs the order database and is intentionally a
-- separate operator-run backfill (ADR 0005 §9.4). This schema migration cannot
-- read order_items from another service schema, so it must not guess a line.

CREATE TABLE IF NOT EXISTS after_sale_line_ledger (
  order_no VARCHAR(64) NOT NULL,
  line_no INT NOT NULL,
  refunded_quantity BIGINT NOT NULL DEFAULT 0,
  refunded_amount_cent BIGINT NOT NULL DEFAULT 0,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (order_no, line_no),
  CONSTRAINT chk_after_sale_line_ledger_quantity CHECK (refunded_quantity >= 0),
  CONSTRAINT chk_after_sale_line_ledger_amount CHECK (refunded_amount_cent >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS after_sale_refund_breakdowns (
  id BIGINT NOT NULL AUTO_INCREMENT,
  after_sale_no VARCHAR(64) NOT NULL,
  goods_cent BIGINT NOT NULL DEFAULT 0,
  freight_cent BIGINT NOT NULL DEFAULT 0,
  total_cent BIGINT NOT NULL DEFAULT 0,
  rule_id VARCHAR(32) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_after_sale_refund_breakdown (after_sale_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS restock_exception_authorizations (
  id BIGINT NOT NULL AUTO_INCREMENT,
  after_sale_no VARCHAR(64) NOT NULL,
  sku_id BIGINT NOT NULL,
  from_warehouse_id BIGINT NOT NULL,
  to_warehouse_id BIGINT NOT NULL,
  operator VARCHAR(64) NOT NULL,
  reason VARCHAR(255) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_restock_exception_authorization (after_sale_no, sku_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
