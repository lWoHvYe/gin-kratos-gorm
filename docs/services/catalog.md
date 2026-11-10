# Catalog service

Owns category/brand/SPU/SKU metadata, media, attributes, publication state and
the C-end browse read model.

## Storage

- MySQL `mall_catalog`: `categories`, `brands`, `spus`, `skus`, `spu_images`,
  `spu_attributes`, `outbox_events`.
- `skus.stock` is a **display projection**, not the inventory source of truth.
  Inventory emits `shop.inventory.stock.changed.v1`; the catalog durable
  consumer `catalog-stock-projection` updates it.
- Product writes add `min_price_cent` and `spu_images`/`spu_attributes` through
  `0002_product_media_and_search.sql`.

## Behaviour

- New SPUs are `DRAFT`; SKUs start with zero stock. An operator must initialize
  inventory and then publish (`ON_SALE`).
- Publishing requires at least one enabled, positively priced SKU and available
  stock for every enabled SKU.
- Disabling/deleting the last enabled SKU atomically moves an `ON_SALE` SPU to
  `OFF_SALE`.
- C-end `GetSku` requires both the SKU and its parent SPU to be sellable.
- C-end search supports category subtree, brand, price range, keyword and
  `price_asc`/`price_desc`/`newest`; it is MySQL-backed by design for Phase 1.5.
- Admin routes are exposed by the `admin` BFF; C-end routes by `gateway`.

## Ports

gRPC `9002`, HTTP health `9102` (`/healthz`, `/readyz`, `/metrics`).
