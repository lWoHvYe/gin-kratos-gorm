-- Keep audit history without growing the hot ledger forever. Rows older than
-- the retention window are copied here before deletion.
CREATE TABLE IF NOT EXISTS inventory_ledger_archive (
  id BIGINT NOT NULL,
  reservation_no VARCHAR(64) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  sku_id BIGINT NOT NULL,
  warehouse_id BIGINT NOT NULL,
  quantity BIGINT NOT NULL,
  entry_type VARCHAR(16) NOT NULL,
  on_hand_after BIGINT NOT NULL DEFAULT 0,
  reserved_after BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  archived_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_ledger_archive_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
