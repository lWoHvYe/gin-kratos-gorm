-- Transaction refactor A: lifecycle and refund are separated; shipping is a
-- permit owned by Order. Legacy refund statuses are moved back to the last
-- lifecycle state recorded in order_status_logs (default PAID).

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'refund_state');
SET @s := IF(@c = 0,
  'ALTER TABLE orders ADD COLUMN refund_state VARCHAR(16) NOT NULL DEFAULT ''NONE'' AFTER refund_status',
  'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE orders
SET refund_state = CASE
  WHEN status = 'REFUNDING' THEN 'REFUNDING'
  WHEN status = 'PARTIALLY_REFUNDED' THEN 'PARTIAL'
  WHEN status = 'REFUNDED' THEN 'FULL'
  ELSE 'NONE'
END
WHERE refund_state = 'NONE';

UPDATE orders o
JOIN (
  SELECT l.order_no, l.to_status
  FROM order_status_logs l
  JOIN (
    SELECT order_no, MAX(id) AS id
    FROM order_status_logs
    WHERE to_status NOT IN ('REFUNDING', 'PARTIALLY_REFUNDED', 'REFUNDED')
    GROUP BY order_no
  ) latest ON latest.id = l.id
) last_lifecycle ON last_lifecycle.order_no = o.order_no
SET o.status = last_lifecycle.to_status
WHERE o.status IN ('REFUNDING', 'PARTIALLY_REFUNDED', 'REFUNDED');

-- Orders with no usable status log: treat as paid, the only state from which
-- the legacy refund projection could have been reached.
UPDATE orders SET status = 'PAID'
WHERE status IN ('REFUNDING', 'PARTIALLY_REFUNDED', 'REFUNDED');

SET @c := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_refund_state');
SET @s := IF(@c = 0,
  'ALTER TABLE orders ADD KEY idx_orders_refund_state (refund_state)',
  'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS order_shipping_permits (
  order_no VARCHAR(64) NOT NULL,
  state VARCHAR(16) NOT NULL DEFAULT 'NONE',
  shipment_no VARCHAR(64) NOT NULL DEFAULT '',
  after_sale_no VARCHAR(64) NOT NULL DEFAULT '',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (order_no),
  KEY idx_order_shipping_permits_state (state)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
