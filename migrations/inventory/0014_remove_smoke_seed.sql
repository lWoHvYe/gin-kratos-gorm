-- Exact 0001 smoke stock only: original quantities, initial version, zero
-- reserved. Any adjustment (including +1/-1) increments version and is kept.
DELETE FROM inventory_stocks
WHERE warehouse_id = 1 AND reserved = 0 AND version = 1
  AND ((sku_id = 1 AND on_hand = 100) OR (sku_id = 2 AND on_hand = 80)
    OR (sku_id = 3 AND on_hand = 200) OR (sku_id = 4 AND on_hand = 50));
