-- ADR 0005 §18.4/§18.5/§18.6/§19.1: responsibility, freight holds, pending
-- line quota, system refund source and cross-warehouse authorization voucher.

ALTER TABLE after_sales
  ADD COLUMN source VARCHAR(24) NOT NULL DEFAULT 'USER',
  ADD COLUMN responsibility_party VARCHAR(32) NOT NULL DEFAULT 'UNDETERMINED',
  ADD COLUMN responsibility_source VARCHAR(32) NOT NULL DEFAULT 'SYSTEM',
  ADD COLUMN responsibility_evidence_ref VARCHAR(128) NOT NULL DEFAULT '',
  ADD COLUMN responsibility_reason_code VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN responsibility_determined_by VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN responsibility_determined_at DATETIME(3) NULL,
  ADD COLUMN return_carrier_code VARCHAR(32) NOT NULL DEFAULT '';

ALTER TABLE after_sale_line_ledger
  ADD COLUMN pending_quantity BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN pending_amount_cent BIGINT NOT NULL DEFAULT 0,
  ADD CONSTRAINT chk_after_sale_line_ledger_pending_quantity CHECK (pending_quantity >= 0),
  ADD CONSTRAINT chk_after_sale_line_ledger_pending_amount CHECK (pending_amount_cent >= 0);

ALTER TABLE after_sale_refund_breakdowns
  ADD COLUMN responsibility_party VARCHAR(32) NOT NULL DEFAULT 'UNDETERMINED',
  ADD COLUMN refund_idempotency_key VARCHAR(96) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS after_sale_freight_holds (
  after_sale_no VARCHAR(64) NOT NULL,
  freight_cent BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'HELD',
  deadline_at DATETIME(3) NOT NULL,
  determined_party VARCHAR(32) NOT NULL DEFAULT 'UNDETERMINED',
  determined_at DATETIME(3) NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (after_sale_no),
  KEY idx_after_sale_freight_holds_due (status, deadline_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE restock_exception_authorizations
  ADD COLUMN auth_no VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN expires_at DATETIME(3) NULL,
  ADD COLUMN used_at DATETIME(3) NULL,
  ADD UNIQUE KEY uk_restock_exception_auth_no (auth_no);

CREATE TABLE IF NOT EXISTS after_sale_legacy_amount_adjustments (
  id BIGINT NOT NULL AUTO_INCREMENT,
  after_sale_no VARCHAR(64) NOT NULL,
  operator VARCHAR(64) NOT NULL,
  reason VARCHAR(255) NOT NULL,
  amount_cent BIGINT NOT NULL,
  approved_by VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_after_sale_legacy_amount_adjustments (after_sale_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
