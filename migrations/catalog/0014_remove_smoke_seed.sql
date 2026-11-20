-- Exact 0001 smoke fixtures only. Any code, name, price, status or id change
-- makes the row ineligible; business-like data is never removed.
DELETE FROM skus
WHERE (id = 1 AND spu_id = 1 AND sku_code = 'KB-RED-87' AND price_cent = 39900 AND status = 'ENABLED')
   OR (id = 2 AND spu_id = 1 AND sku_code = 'KB-BROWN-87' AND price_cent = 42900 AND status = 'ENABLED')
   OR (id = 3 AND spu_id = 2 AND sku_code = 'BOOK-GO-01' AND price_cent = 8900 AND status = 'ENABLED')
   OR (id = 4 AND spu_id = 2 AND sku_code = 'BOOK-GO-02' AND price_cent = 12900 AND status = 'ENABLED');
DELETE s FROM spus s
WHERE (s.id = 1 AND s.name = 'Kratos Keyboard' AND s.subtitle = '87-key hot-swappable' AND s.status = 'ON_SALE')
   OR (s.id = 2 AND s.name = 'Go in Action' AND s.subtitle = 'Cloud-native microservices' AND s.status = 'ON_SALE')
  AND NOT EXISTS (SELECT 1 FROM skus k WHERE k.spu_id = s.id);
DELETE FROM brands WHERE id IN (1,2) AND name IN ('Kratos','Go Press')
  AND NOT EXISTS (SELECT 1 FROM spus s WHERE s.brand_id = brands.id);
DELETE FROM categories WHERE id IN (1,2) AND name IN ('Keyboards','Books')
  AND NOT EXISTS (SELECT 1 FROM spus s WHERE s.category_id = categories.id);
