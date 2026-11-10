CREATE TABLE IF NOT EXISTS shipment_events (
  id BIGINT NOT NULL AUTO_INCREMENT,
  shipment_no VARCHAR(64) NOT NULL,
  status VARCHAR(16) NOT NULL,
  carrier VARCHAR(64) NOT NULL DEFAULT '',
  tracking_no VARCHAR(128) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_shipment_events_shipment (shipment_no, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
