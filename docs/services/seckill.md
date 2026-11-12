# Seckill service

Status: S3 (activity lifecycle, inventory bucket, Redis Lua admission, durable
holds/expiry, JetStream order worker, fixed-price order creation and payment
intent). Production load/HA/reconciliation remain S4.

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

## S2 admission

Customer routes (through gateway):

```text
GET  /api/v1/seckill/activities/:id
POST /api/v1/seckill/activities/:id/reserve
GET  /api/v1/seckill/qualifications/:token
```

- The Redis Lua script checks ACTIVE status, per-user marker and stock, then
  decrements stock once.
- The token is deterministic HMAC(activity,user,version), so an idempotent retry
  returns the same token without touching stock.
- After Redis admission, inventory `HoldSeckillStock` moves one unit from
  `available` to `held`; the qualification row is persisted in `mall_seckill`.
  If the DB step fails, the exact token's Redis stock and DB hold are released.
- `go-redis/redis_rate` limits attempts per user (10/min).
- A 30-second expiry worker releases `ISSUED` qualifications past
  `expires_at`, returning both the Redis unit and the DB held unit.
- S2 intentionally does not publish an order command yet. The qualification
  remains `ISSUED`; S3 adds JetStream and `CreateSeckillOrder`.

The public reserve endpoint returns `202 Accepted`; sold-out is `409`, rate
limit is `429`, and Redis/NATS unavailability is `503`.

## Not yet implemented

- JetStream command publishing/worker
- `CreateSeckillOrder`
- order/payment linkage after qualification
- reconciliation worker
- C-end seckill page
