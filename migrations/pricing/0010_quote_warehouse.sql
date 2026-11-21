-- Warehouse-C phase 2b: quote lines carry the preview allocation and quotes
-- carry one freight row per fulfillment group. Existing quotes backfill to a
-- single G1/warehouse-1 group, preserving their freight/pay totals.

ALTER TABLE quote_lines
  ADD COLUMN line_no INT NOT NULL DEFAULT 0 AFTER quote_id,
  ADD COLUMN warehouse_id BIGINT NOT NULL DEFAULT 1 AFTER quantity,
  ADD COLUMN group_no VARCHAR(64) NOT NULL DEFAULT 'G1' AFTER warehouse_id;

UPDATE quote_lines q
JOIN (
  SELECT quote_id, sku_id,
         ROW_NUMBER() OVER (PARTITION BY quote_id ORDER BY sku_id) AS rn
  FROM quote_lines
) ranked ON ranked.quote_id = q.quote_id AND ranked.sku_id = q.sku_id
SET q.line_no = ranked.rn
WHERE q.line_no = 0;

ALTER TABLE quote_lines
  ADD UNIQUE KEY uk_quote_lines_line (quote_id, line_no),
  ADD KEY idx_quote_lines_group (quote_id, group_no);

CREATE TABLE IF NOT EXISTS quote_groups (
  id BIGINT NOT NULL AUTO_INCREMENT,
  quote_id VARCHAR(64) NOT NULL,
  group_no VARCHAR(64) NOT NULL,
  warehouse_id BIGINT NOT NULL DEFAULT 1,
  goods_cent BIGINT NOT NULL DEFAULT 0,
  discount_cent BIGINT NOT NULL DEFAULT 0,
  freight_cent BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_quote_groups_no (quote_id, group_no),
  KEY idx_quote_groups_quote (quote_id),
  CONSTRAINT chk_quote_groups_goods CHECK (goods_cent >= 0),
  CONSTRAINT chk_quote_groups_discount CHECK (discount_cent >= 0),
  CONSTRAINT chk_quote_groups_freight CHECK (freight_cent >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO quote_groups (quote_id, group_no, warehouse_id, goods_cent, discount_cent, freight_cent)
SELECT q.quote_id, 'G1', 1, q.total_cent, q.discount_cent, q.freight_cent
FROM quotes q
WHERE NOT EXISTS (
  SELECT 1 FROM quote_groups g WHERE g.quote_id = q.quote_id AND g.group_no = 'G1'
);
