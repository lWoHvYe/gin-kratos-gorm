# Demo catalog data

`deploy/seed/catalog-demo.sql` replaces the single "E2E Widget" catalog with a
realistic demo assortment for AI and UI testing:

- 6 top-level categories with 13 children: 手机数码, 电脑办公, 家用电器,
  食品饮料, 美妆个护, 家居日用;
- 9 fictional brands (星环, 云舟, 极光, 声谷, 远峰, 清风, 山野, 素研, 眠川):
  fictional names avoid wrong claims about real products while keeping the
  naming, specs and pricing realistic;
- 18 SPUs / 25 SKUs across phones, laptops, tablets, monitors, headphones,
  watches, large appliances, coffee, nuts, skincare and bedding;
- attributes, gallery images (picsum placeholders), descriptions and
  `min_price_cent` populated, plus matching `inventory_stocks`.

Apply it explicitly (never as a migration):

```bash
make seed-demo              # k3s: kubectl -n mall-data exec mysql-0
scripts/seed-demo-catalog.sh --compose   # docker compose infra
```

The script uses `utf8mb4` on the client connection and deletes
`catalog:product:*` Redis keys afterwards, because product detail is cached and
a reseed does not invalidate it otherwise. IDs are fixed (categories 100+,
brands 200+, SPUs 1000+, SKUs 10000+) so reruns update the demo rows instead of
creating duplicates; existing E2E rows are untouched.

The data is intentionally version-controlled as SQL, not a migration: any
environment that should not carry demo products simply does not run it.
