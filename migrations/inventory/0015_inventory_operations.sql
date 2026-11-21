-- Whole-order inventory operations. A reservation remains the per-SKU unit of
-- truth; these tables add the operation aggregate and per-item progress needed
-- to resume a multi-SKU confirm/cancel without starving later items.
--
-- See docs/services/inventory.md "Whole-order operations" for the protocol.

CREATE TABLE IF NOT EXISTS inventory_operations (
  operation_id VARCHAR(128) NOT NULL,
  operation_type VARCHAR(16) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  scope_hash CHAR(64) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'PENDING',
  total_items INT NOT NULL,
  succeeded_items INT NOT NULL DEFAULT 0,
  failed_items INT NOT NULL DEFAULT 0,
  skipped_items INT NOT NULL DEFAULT 0,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (operation_id),
  UNIQUE KEY uk_inventory_operation_scope (order_no, operation_type, scope_hash),
  KEY idx_inventory_operation_status (status, updated_at),
  CONSTRAINT chk_inventory_operation_counts
    CHECK (total_items > 0 AND succeeded_items + failed_items + skipped_items <= total_items)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS inventory_operation_items (
  operation_id VARCHAR(128) NOT NULL,
  sku_id BIGINT NOT NULL,
  quantity BIGINT NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'PENDING',
  attempts INT NOT NULL DEFAULT 0,
  last_error VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (operation_id, sku_id),
  KEY idx_inventory_operation_item_status (operation_id, status),
  CONSTRAINT chk_inventory_operation_item_quantity CHECK (quantity > 0),
  CONSTRAINT chk_inventory_operation_item_attempts CHECK (attempts >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
