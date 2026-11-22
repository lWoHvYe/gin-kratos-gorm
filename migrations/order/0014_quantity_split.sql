-- ADR 0005 phase A/C: quantity-split allocation snapshot, line money and
-- order-level freeze. All columns are additive; old code keeps reading the
-- pre-existing columns.

ALTER TABLE orders
  ADD COLUMN allocation_frozen_at DATETIME(3) NULL,
  ADD COLUMN allocation_hash CHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN package_count INT NOT NULL DEFAULT 1,
  ADD COLUMN warehouse_count INT NOT NULL DEFAULT 1;

ALTER TABLE order_items
  ADD COLUMN line_paid_cent BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN unit_paid_cent BIGINT NOT NULL DEFAULT 0;

ALTER TABLE order_fulfillment_groups
  ADD COLUMN freight_share_cent BIGINT NOT NULL DEFAULT 0;

-- Backfill: line_paid = price*qty - line discount; unit_paid floors the line
-- paid amount (the last refund adds the remainder, ADR 0005 §6.3).
UPDATE order_items
SET line_paid_cent = price_cent * quantity - discount_cent,
    unit_paid_cent = FLOOR((price_cent * quantity - discount_cent) / quantity)
WHERE line_paid_cent = 0 AND quantity > 0;

UPDATE order_fulfillment_groups
SET freight_share_cent = freight_cent
WHERE freight_share_cent = 0;

UPDATE orders o
JOIN (
  SELECT order_no, COUNT(*) AS group_count, COUNT(DISTINCT warehouse_id) AS warehouse_count
  FROM order_fulfillment_groups
  GROUP BY order_no
) g ON g.order_no = o.order_no
SET o.package_count = g.group_count,
    o.warehouse_count = g.warehouse_count
WHERE o.allocation_frozen_at IS NULL;

-- Existing paid orders are conservatively frozen at their last update. A
-- pending/cancelled order was never frozen.
UPDATE orders
SET allocation_frozen_at = updated_at
WHERE allocation_frozen_at IS NULL
  AND status IN ('PAID', 'ALLOCATED', 'SHIPPED', 'COMPLETED');

CREATE TABLE IF NOT EXISTS order_allocation_snapshots (
  order_no VARCHAR(64) NOT NULL,
  allocation_hash CHAR(64) NOT NULL DEFAULT '',
  lines_json JSON NOT NULL,
  groups_json JSON NOT NULL,
  frozen_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (order_no),
  KEY idx_order_allocation_snapshots_frozen (frozen_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS order_allocation_decisions (
  id BIGINT NOT NULL AUTO_INCREMENT,
  order_no VARCHAR(64) NOT NULL DEFAULT '',
  quote_id VARCHAR(64) NOT NULL DEFAULT '',
  request_json JSON NOT NULL,
  availability_json JSON NOT NULL,
  chosen_json JSON NOT NULL,
  rule_hits_json JSON NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_order_allocation_decisions_order (order_no),
  KEY idx_order_allocation_decisions_quote (quote_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS order_shortage_cases (
  id BIGINT NOT NULL AUTO_INCREMENT,
  order_no VARCHAR(64) NOT NULL,
  group_no VARCHAR(64) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'OPEN',
  resolution VARCHAR(24) NOT NULL DEFAULT '',
  reason VARCHAR(255) NOT NULL DEFAULT '',
  restock_eta DATE NULL,
  notified_at DATETIME(3) NULL,
  choose_deadline_at DATETIME(3) NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_order_shortage_group (order_no, group_no),
  KEY idx_order_shortage_open (status, choose_deadline_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS order_reconciliation_diffs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  run_date DATE NOT NULL,
  equation VARCHAR(32) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  expected_cent BIGINT NOT NULL DEFAULT 0,
  actual_cent BIGINT NOT NULL DEFAULT 0,
  diff_cent BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_order_reconciliation_diff (run_date, equation, order_no),
  KEY idx_order_reconciliation_run (run_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- SQL guards (ADR 0005 §5.6): once an order is frozen, its allocation columns
-- are immutable even if a future caller forgets the repository guard. The
-- error is a hard failure, never a silent no-op.
DELIMITER $$
DROP TRIGGER IF EXISTS trg_order_items_frozen_update$$
CREATE TRIGGER trg_order_items_frozen_update
BEFORE UPDATE ON order_items
FOR EACH ROW
BEGIN
  IF EXISTS (SELECT 1 FROM orders o WHERE o.order_no = OLD.order_no AND o.allocation_frozen_at IS NOT NULL)
     AND (NOT (NEW.line_no <=> OLD.line_no)
       OR NOT (NEW.sku_id <=> OLD.sku_id)
       OR NOT (NEW.quantity <=> OLD.quantity)
       OR NOT (NEW.warehouse_id <=> OLD.warehouse_id)
       OR NOT (NEW.fulfillment_group_no <=> OLD.fulfillment_group_no)
       OR NOT (NEW.price_cent <=> OLD.price_cent)
       OR NOT (NEW.discount_cent <=> OLD.discount_cent)
       OR NOT (NEW.line_paid_cent <=> OLD.line_paid_cent)
       OR NOT (NEW.unit_paid_cent <=> OLD.unit_paid_cent)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'allocation_frozen: order item allocation is immutable';
  END IF;
END$$
DROP TRIGGER IF EXISTS trg_order_groups_frozen_update$$
CREATE TRIGGER trg_order_groups_frozen_update
BEFORE UPDATE ON order_fulfillment_groups
FOR EACH ROW
BEGIN
  IF EXISTS (SELECT 1 FROM orders o WHERE o.order_no = OLD.order_no AND o.allocation_frozen_at IS NOT NULL)
     AND (NOT (NEW.group_no <=> OLD.group_no)
       OR NOT (NEW.order_no <=> OLD.order_no)
       OR NOT (NEW.seq <=> OLD.seq)
       OR NOT (NEW.warehouse_id <=> OLD.warehouse_id)
       OR NOT (NEW.freight_share_cent <=> OLD.freight_share_cent)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'allocation_frozen: fulfillment group allocation is immutable';
  END IF;
END$$
DROP TRIGGER IF EXISTS trg_order_allocation_snapshot_content_update$$
CREATE TRIGGER trg_order_allocation_snapshot_content_update
BEFORE UPDATE ON order_allocation_snapshots
FOR EACH ROW
BEGIN
  IF NOT (NEW.allocation_hash <=> OLD.allocation_hash)
     OR NOT (NEW.lines_json <=> OLD.lines_json)
     OR NOT (NEW.groups_json <=> OLD.groups_json)
     OR NOT (NEW.created_at <=> OLD.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'allocation_frozen: snapshot content is append-only';
  END IF;
END$$
DELIMITER ;
