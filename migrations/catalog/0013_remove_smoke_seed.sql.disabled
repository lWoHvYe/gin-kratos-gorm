-- 0001 seeded a tiny smoke catalog. New environments should get data from
-- deploy/seed explicitly, not from schema migrations. Remove only exact seed
-- signatures; never touch rows that no longer match.
DELETE FROM skus
WHERE sku_code IN ('KB-RED-87','KB-BROWN-87','BOOK-GO-01','BOOK-GO-02')
  AND spu_id IN (1,2);
DELETE s FROM spus s
WHERE s.id IN (1,2)
  AND ((s.id = 1 AND s.name = 'Kratos Keyboard') OR (s.id = 2 AND s.name = 'Go in Action'))
  AND NOT EXISTS (SELECT 1 FROM skus k WHERE k.spu_id = s.id);
DELETE FROM brands WHERE id IN (1,2) AND name IN ('Kratos','Go Press')
  AND NOT EXISTS (SELECT 1 FROM spus s WHERE s.brand_id = brands.id);
DELETE FROM categories WHERE id IN (1,2) AND name IN ('Keyboards','Books')
  AND NOT EXISTS (SELECT 1 FROM spus s WHERE s.category_id = categories.id);
