-- Cross-warehouse split orders, phase 1: fulfillment groups + line allocation
-- snapshot. Existing orders become a single G1/warehouse-1 group; no read or
-- write path splits them yet. See docs/adr/0002-cross-warehouse-split-orders.md.

CREATE TABLE IF NOT EXISTS order_fulfillment_groups (
  id BIGINT NOT NULL AUTO_INCREMENT,
  group_no VARCHAR(64) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  seq INT NOT NULL DEFAULT 1,
  warehouse_id BIGINT NOT NULL DEFAULT 1,
  status VARCHAR(24) NOT NULL DEFAULT 'PENDING',
  reservation_state VARCHAR(16) NOT NULL DEFAULT 'HELD',
  freight_cent BIGINT NOT NULL DEFAULT 0,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_order_fulfillment_group_no (order_no, group_no),
  UNIQUE KEY uk_order_fulfillment_group_seq (order_no, seq),
  KEY idx_order_fulfillment_groups_order (order_no, seq),
  KEY idx_order_fulfillment_groups_warehouse (warehouse_id, status),
  CONSTRAINT chk_order_fulfillment_group_seq CHECK (seq > 0),
  CONSTRAINT chk_order_fulfillment_group_freight CHECK (freight_cent >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE order_items
  ADD COLUMN line_no INT NOT NULL DEFAULT 0 AFTER order_no,
  ADD COLUMN warehouse_id BIGINT NOT NULL DEFAULT 1 AFTER quantity,
  ADD COLUMN fulfillment_group_no VARCHAR(64) NOT NULL DEFAULT 'G1' AFTER warehouse_id;

UPDATE order_items
SET line_no = id
WHERE line_no = 0;

ALTER TABLE order_items
  ADD UNIQUE KEY uk_order_items_line (order_no, line_no),
  ADD KEY idx_order_items_group (order_no, fulfillment_group_no);

INSERT INTO order_fulfillment_groups
  (group_no, order_no, seq, warehouse_id, status, reservation_state, freight_cent, version, created_at, updated_at)
SELECT 'G1', o.order_no, 1, 1,
       CASE o.status
         WHEN 'PENDING_PAYMENT' THEN 'PENDING'
         WHEN 'PAID' THEN 'ALLOCATED'
         WHEN 'ALLOCATED' THEN 'ALLOCATED'
         WHEN 'SHIPPED' THEN 'SHIPPED'
         WHEN 'COMPLETED' THEN 'RECEIVED'
         ELSE 'CANCELLED'
       END,
       CASE
         WHEN o.status IN ('PENDING_PAYMENT', 'PAID', 'ALLOCATED', 'SHIPPED', 'COMPLETED') THEN 'HELD'
         ELSE 'RELEASED'
       END,
       o.freight_cent, 1, o.created_at, o.updated_at
FROM orders o
WHERE NOT EXISTS (
  SELECT 1 FROM order_fulfillment_groups g WHERE g.order_no = o.order_no AND g.group_no = 'G1'
);
