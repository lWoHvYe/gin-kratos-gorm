-- ADR-0011: Order-side receipt for the Inventory release ack. The legacy 24h
-- compensation may only call Payment after this row is RELEASED.
CREATE TABLE IF NOT EXISTS refund_release_receipts (
  release_id    VARCHAR(128) NOT NULL,
  refund_id     VARCHAR(128) NOT NULL,
  order_no      VARCHAR(64)  NOT NULL,
  state         VARCHAR(16)  NOT NULL DEFAULT 'PENDING',
  last_error    VARCHAR(512) NOT NULL DEFAULT '',
  version       BIGINT       NOT NULL DEFAULT 1,
  lease_owner   VARCHAR(64)  NOT NULL DEFAULT '',
  lease_until   DATETIME(3)  NULL,
  created_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (release_id),
  UNIQUE KEY uk_refund_release_receipt_refund (refund_id),
  KEY idx_refund_release_receipts_state (state, lease_until),
  CONSTRAINT chk_refund_release_receipt_state CHECK (state IN ('PENDING','RELEASED','FAILED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
