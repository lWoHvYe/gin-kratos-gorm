-- Links an order to a seckill activity hold so Confirm/Release can maintain the
-- seckill bucket without double-reserving inventory_stocks.reserved.
CREATE TABLE IF NOT EXISTS seckill_order_stock (
  id BIGINT NOT NULL AUTO_INCREMENT,
  activity_id BIGINT NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  sku_id BIGINT NOT NULL,
  quantity BIGINT NOT NULL,
  ref_no VARCHAR(64) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'HELD',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_seckill_order_stock (order_no, sku_id),
  KEY idx_seckill_order_activity (activity_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
