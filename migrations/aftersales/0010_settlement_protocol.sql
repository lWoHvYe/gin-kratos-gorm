-- Transaction refactor A: lifetime amount ledger, effective-amount versions,
-- durable restock/refund tasks, and RETURN_RECEIVED semantics.
--
-- RETURN_RECEIVED means the parcel was received, not that stock was restored.
-- restock_state gates refund admission; existing RETURN_REFUND rows are queued
-- for an idempotent Inventory.Restock replay.

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'after_sales' AND COLUMN_NAME = 'effective_amount_cent');
SET @s := IF(@c = 0,
  'ALTER TABLE after_sales ADD COLUMN effective_amount_cent BIGINT NOT NULL DEFAULT 0 AFTER amount_cent',
  'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'after_sales' AND COLUMN_NAME = 'amount_state');
SET @s := IF(@c = 0,
  'ALTER TABLE after_sales ADD COLUMN amount_state VARCHAR(16) NOT NULL DEFAULT ''ACTIVE'' AFTER effective_amount_cent',
  'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'after_sales' AND COLUMN_NAME = 'restock_state');
SET @s := IF(@c = 0,
  'ALTER TABLE after_sales ADD COLUMN restock_state VARCHAR(24) NOT NULL DEFAULT ''NONE'' AFTER amount_state',
  'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE after_sales SET effective_amount_cent = amount_cent WHERE effective_amount_cent = 0;

UPDATE after_sales
SET amount_state = CASE
  WHEN status = 'REJECTED' THEN 'RELEASED'
  WHEN status = 'REFUNDED' THEN 'CONSUMED'
  ELSE 'ACTIVE'
END
WHERE amount_state = 'ACTIVE';

UPDATE after_sales SET status = 'RETURN_RECEIVED' WHERE status = 'RECEIVED';

UPDATE after_sales
SET restock_state = 'PENDING'
WHERE `type` = 'RETURN_REFUND'
  AND status IN ('RETURN_RECEIVED', 'REFUNDED', 'REFUNDING')
  AND restock_state = 'NONE';

CREATE TABLE IF NOT EXISTS after_sale_amount_versions (
  id BIGINT NOT NULL AUTO_INCREMENT,
  after_sale_id BIGINT NOT NULL,
  version INT NOT NULL,
  effective_amount_cent BIGINT NOT NULL,
  reason VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_after_sale_amount_versions (after_sale_id, version),
  KEY idx_after_sale_amount_versions (after_sale_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS after_sale_amount_ledger (
  order_no VARCHAR(64) NOT NULL,
  captured_cent BIGINT NOT NULL DEFAULT 0,
  consumed_cent BIGINT NOT NULL DEFAULT 0,
  source VARCHAR(32) NOT NULL DEFAULT 'uninitialized',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (order_no),
  CONSTRAINT chk_after_sale_amount_ledger_nonneg CHECK (captured_cent >= 0 AND consumed_cent >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS after_sale_restock_tasks (
  after_sale_no VARCHAR(64) NOT NULL,
  sku_id BIGINT NOT NULL,
  quantity BIGINT NOT NULL,
  `condition` VARCHAR(16) NOT NULL DEFAULT 'SELLABLE',
  status VARCHAR(24) NOT NULL DEFAULT 'PENDING',
  attempts INT NOT NULL DEFAULT 0,
  last_error VARCHAR(512) NOT NULL DEFAULT '',
  lease_owner VARCHAR(128) NOT NULL DEFAULT '',
  lease_until DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (after_sale_no, sku_id),
  KEY idx_after_sale_restock_status (status, lease_until),
  CONSTRAINT chk_after_sale_restock_quantity CHECK (quantity > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS after_sale_refund_tasks (
  after_sale_no VARCHAR(64) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'PENDING',
  attempts INT NOT NULL DEFAULT 0,
  last_error VARCHAR(512) NOT NULL DEFAULT '',
  lease_owner VARCHAR(128) NOT NULL DEFAULT '',
  lease_until DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (after_sale_no),
  KEY idx_after_sale_refund_status (status, lease_until)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO after_sale_amount_versions (after_sale_id, version, effective_amount_cent, reason)
SELECT id, 1, effective_amount_cent, 'MIGRATED' FROM after_sales
ON DUPLICATE KEY UPDATE after_sale_id = after_sale_id;

INSERT INTO after_sale_amount_ledger (order_no, captured_cent, consumed_cent, source)
SELECT order_no, 0, COALESCE(SUM(effective_amount_cent), 0), 'uninitialized'
FROM after_sales
WHERE amount_state <> 'RELEASED'
GROUP BY order_no
ON DUPLICATE KEY UPDATE order_no = order_no;

INSERT INTO after_sale_restock_tasks (after_sale_no, sku_id, quantity, `condition`, status, attempts, last_error)
SELECT a.after_sale_no, i.sku_id, i.quantity,
       COALESCE(NULLIF(i.condition_code, ''), 'SELLABLE'), 'PENDING', 0, ''
FROM after_sales a
JOIN after_sale_items i ON i.after_sale_id = a.id
WHERE a.`type` = 'RETURN_REFUND' AND a.restock_state = 'PENDING'
  AND NOT EXISTS (
    SELECT 1 FROM after_sale_restock_tasks t
    WHERE t.after_sale_no = a.after_sale_no AND t.sku_id = i.sku_id
  );

INSERT INTO after_sale_refund_tasks (after_sale_no, status, attempts, last_error)
SELECT a.after_sale_no, 'PENDING', 0, ''
FROM after_sales a
WHERE a.status = 'REFUNDING'
  AND (a.`type` <> 'RETURN_REFUND' OR a.restock_state = 'DONE')
  AND NOT EXISTS (
    SELECT 1 FROM after_sale_refund_tasks t WHERE t.after_sale_no = a.after_sale_no
  );
