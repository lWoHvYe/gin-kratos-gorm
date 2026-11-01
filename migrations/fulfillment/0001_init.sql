CREATE TABLE IF NOT EXISTS shipments (
  id BIGINT NOT NULL AUTO_INCREMENT,
  shipment_no VARCHAR(64) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  warehouse_id BIGINT NOT NULL DEFAULT 0,
  carrier VARCHAR(64) NOT NULL DEFAULT '',
  tracking_no VARCHAR(128) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'CREATED',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_shipments_no (shipment_no),
  UNIQUE KEY uk_shipments_order (order_no),
  KEY idx_shipments_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS shipment_items (
  id BIGINT NOT NULL AUTO_INCREMENT,
  shipment_id BIGINT NOT NULL,
  sku_id BIGINT NOT NULL,
  quantity INT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_shipment_items_sku (shipment_id, sku_id),
  KEY idx_shipment_items_shipment (shipment_id)
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
