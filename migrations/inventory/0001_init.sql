-- Inventory service schema (mall_inventory).
-- Managed with plain SQL; the service uses database/sql, not GORM.

CREATE TABLE IF NOT EXISTS warehouses (
  id BIGINT NOT NULL AUTO_INCREMENT,
  name VARCHAR(128) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'ENABLED',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_warehouses_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- The inventory service never invents stock: rows are provisioned by ops/admin
-- as products are listed, for example:
--   INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand) VALUES (1001, 1, 100)
--   ON DUPLICATE KEY UPDATE on_hand = on_hand + VALUES(on_hand);
CREATE TABLE IF NOT EXISTS inventory_stocks (
  id BIGINT NOT NULL AUTO_INCREMENT,
  sku_id BIGINT NOT NULL,
  warehouse_id BIGINT NOT NULL,
  on_hand BIGINT NOT NULL DEFAULT 0,
  reserved BIGINT NOT NULL DEFAULT 0,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_inventory_stock (sku_id, warehouse_id),
  KEY idx_inventory_warehouse (warehouse_id),
  CONSTRAINT chk_inventory_stock_nonneg CHECK (on_hand >= 0 AND reserved >= 0 AND reserved <= on_hand)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS stock_reservations (
  id BIGINT NOT NULL AUTO_INCREMENT,
  reservation_no VARCHAR(64) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  sku_id BIGINT NOT NULL,
  warehouse_id BIGINT NOT NULL,
  quantity BIGINT NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'HELD',
  expires_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_reservations_no (reservation_no),
  UNIQUE KEY uk_reservations_order_sku (order_no, sku_id),
  KEY idx_reservations_status_expiry (status, expires_at),
  KEY idx_reservations_sku_warehouse (sku_id, warehouse_id),
  CONSTRAINT chk_reservations_quantity CHECK (quantity > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS inventory_ledger (
  id BIGINT NOT NULL AUTO_INCREMENT,
  reservation_no VARCHAR(64) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  sku_id BIGINT NOT NULL,
  warehouse_id BIGINT NOT NULL,
  quantity BIGINT NOT NULL,
  entry_type VARCHAR(16) NOT NULL,
  on_hand_after BIGINT NOT NULL DEFAULT 0,
  reserved_after BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_ledger_reservation_entry (reservation_no, entry_type),
  KEY idx_ledger_order (order_no),
  KEY idx_ledger_sku_warehouse (sku_id, warehouse_id)
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

-- DTM creates its barrier table automatically; declaring it keeps a fresh
-- database usable by branch calls before the first DTM transaction.
CREATE TABLE IF NOT EXISTS dtm_barrier (
  id BIGINT NOT NULL AUTO_INCREMENT,
  trans_type VARCHAR(45) DEFAULT '',
  gid VARCHAR(128) DEFAULT '',
  branch_id VARCHAR(128) DEFAULT '',
  op VARCHAR(45) DEFAULT '',
  barrier_id VARCHAR(45) DEFAULT '',
  reason VARCHAR(45) DEFAULT '',
  create_time DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  update_time DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uniq_barrier (trans_type, gid, branch_id, op)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO warehouses (id, name, status) VALUES (1, 'Main Warehouse', 'ENABLED');

INSERT IGNORE INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version) VALUES
  (1, 1, 100, 0, 1), (2, 1, 80, 0, 1), (3, 1, 200, 0, 1), (4, 1, 50, 0, 1);
