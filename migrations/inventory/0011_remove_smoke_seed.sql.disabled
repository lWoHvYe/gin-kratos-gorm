-- The 0001 smoke inventory rows are dev fixtures; remove only the exact initial
-- quantities and only when the stock is still untouched.
DELETE FROM inventory_stocks
WHERE warehouse_id = 1 AND reserved = 0
  AND ((sku_id = 1 AND on_hand = 100) OR (sku_id = 2 AND on_hand = 80)
    OR (sku_id = 3 AND on_hand = 200) OR (sku_id = 4 AND on_hand = 50));
