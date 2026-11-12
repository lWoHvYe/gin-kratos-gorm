# Seckill service

Status: S1 (activity lifecycle, inventory bucket allocation, warmup). Admission
and asynchronous order creation are S2/S3, documented in
[../seckill-design.md](../seckill-design.md).

## Ports

- gRPC: `9010`
- health: `9110`
- database: `mall_seckill`
- Redis: shared Redis today; production should use a dedicated HA Redis
- inventory dependency: `inventory:9003`

## S1 API

Admin BFF routes:

```text
GET  /api/admin/v1/seckill/activities
POST /api/admin/v1/seckill/activities
GET  /api/admin/v1/seckill/activities/:id
PUT  /api/admin/v1/seckill/activities/:id
POST /api/admin/v1/seckill/activities/:id/publish
POST /api/admin/v1/seckill/activities/:id/pause
POST /api/admin/v1/seckill/activities/:id/warmup
GET  /api/admin/v1/seckill/activities/:id/stock
```

`publish` is deliberately idempotent:

1. allocate the DB bucket in inventory;
2. set status `READY` or `ACTIVE` depending on server time;
3. write `seckill:{id}:meta` and `seckill:{id}:stock` to Redis.

`pause` releases only the unsold (`available`) bucket. `held`/`sold` are not
silently returned; that policy belongs to the order/refund flow.

The scheduler activates `READY` activities at `start_at` and ends/releases
`READY`/`ACTIVE` activities at `end_at`. It runs in every replica; the status
update is a conditional compare-and-set, so duplicate work is harmless.

## S1 data

- `mall_seckill.seckill_activities`
- `mall_seckill.seckill_qualifications` (reserved for S2)
- `mall_inventory.seckill_stocks`
- `mall_inventory.seckill_stock_ledger`

Inventory allocation increments `inventory_stocks.reserved` so normal sales
cannot consume the seckill bucket. Release decrements only the unsold part.

## Not yet implemented

- `POST /api/v1/seckill/activities/:id/reserve`
- Redis Lua admission/token issuance
- JetStream command publishing
- `CreateSeckillOrder`
- compensation/reconciliation worker
- frontend seckill page
