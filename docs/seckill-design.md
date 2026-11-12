# Seckill design

Status: S3 implemented plus S4 hardening in progress (activity lifecycle,
inventory bucket, Redis Lua admission, durable holds/expiry, JetStream order
worker, fixed-price CreateSeckillOrder and payment intent, cancellation
projection, DLQ publish, report-only reconciler, C-end seckill page). S4
HA/load/WAF and automatic reconciliation remain.

This document is deliberately a production design rather than a demo sketch.
Seckill is treated as a small, isolated, failure-aware subsystem that reuses the
existing order, inventory, payment and promotion bounded contexts. The target is
no oversell, bounded load, replay-safe admission, and graceful degradation.

## 1. Problem statement

A seckill campaign combines four hard requirements:

1. **Extreme read/write skew**: a large burst hits one activity/SKU for a few
   seconds, while the normal catalog and checkout traffic continues.
2. **No oversell**: the number of successful orders must never exceed the stock
   allocated to the activity.
3. **Bounded admission**: the number of in-flight requests/orders must be
   bounded by a queue and worker capacity, not by an unbounded goroutine pool.
4. **Fairness/replay safety**: the same user/request must not consume stock
   twice; abandoned qualifications and failed async orders must return stock.

A demo that decrements a Redis key and writes an order synchronously is not
sufficient: it loses messages on process death, leaks stock on worker failure,
allows duplicate orders, and collapses when the database or downstream service
is slow.

## 2. Non-goals

- Multi-region active-active sale.
- Lottery/allocation algorithms.
- Dynamic price discovery during the burst.
- A self-built WAF, bot-detection ML model, distributed queue, or load generator.
- Replacing the existing order/payment/inventory state machines.

## 3. Architecture

```text
                        ┌──────────────────────────────┐
                        │ Edge WAF / Waiting Room      │
                        │ Cloudflare/AWS or Envoy+limit│
                        └──────────────┬───────────────┘
                                       │ HTTPS
                        ┌──────────────▼───────────────┐
                        │ gateway (BFF)                │
                        │ auth, rate limit, routing    │
                        └──────────────┬───────────────┘
                                       │ gRPC
                  ┌────────────────────▼────────────────────┐
                  │ seckill-api (stateless, HPA)            │
                  │ activity read, reserve, qualification   │
                  │ Redis Lua admission, token issuance     │
                  └───────┬───────────────────────┬─────────┘
                          │                       │
                  Redis (HA)                  NATS JetStream
                  stock + token ledger        seckill.order.requested
                          │                       │
                  ┌───────▼────────┐      ┌───────▼────────┐
                  │ seckill-worker │─────▶│ order service  │
                  │ pull consumer  │      │ CreateSeckill  │
                  └───────┬────────┘      └───────┬────────┘
                          │                       │
                  MySQL seckill schema      inventory / order / payment
                          │
                  reconciliation + expiry sweeper
```

Key decisions:

- **Admission is Redis-atomic**, not database-synchronous. The HTTP response is
  a qualification token, not a created order.
- **Order creation is asynchronous** through NATS JetStream and an idempotent
  worker. This decouples the burst from MySQL/order latency.
- **Database stock is carved into a seckill bucket** before the sale. Redis is
  the admission mirror; the database bucket + ledger are the final authority.
- **Existing order/inventory/payment services remain authoritative** for the
  order lifecycle. Seckill does not reimplement order state or payment.
- **No distributed lock**. Atomic Lua scripts and unique constraints are used
  instead of lock/lease protocols.

## 4. Bounded contexts and new service

Add one service, `seckill`, with two workloads from the same image:

- `seckill-api`: public activity read + reserve/qualification APIs.
- `seckill-worker`: NATS pull consumer + compensation/reconciliation/expiry.

Why not put this in gateway:

- admission has a different scaling and failure profile from general BFF reads;
- a bad seckill deploy must not take down normal catalog/login/checkout;
- the worker needs its own queue-depth HPA and connection pool.

Why not put this in pricing:

- pricing is quote/promotion logic and is on the normal order path;
- seckill admission is a high-QPS atomic reservation path with a separate Redis
  and NATS stream;
- coupling them would make ordinary quotes and campaign bursts share fate.

The existing `Campaign`/`PromotionRule` remains the campaign metadata source.
A `seckill_activities` row links to a campaign and carries seckill-specific
fields. Do not encode seckill stock semantics into generic promotion rules.

## 5. Domain model

### 5.1 Seckill activity

```text
seckill_activities
  id
  campaign_id
  sku_id
  seckill_price_cent
  stock_total
  per_user_limit          -- normally 1
  start_at, end_at
  status                  -- DRAFT, READY, ACTIVE, PAUSED, SOLD_OUT, ENDED
  version                 -- optimistic admin edits
  created_at, updated_at
```

A seckill activity is single-SKU in the first version. Multi-SKU carts are
explicitly rejected: the hot path must not run the generic cart/quote engine.

### 5.2 DB stock bucket

Do not let the Redis counter be the only stock. The inventory service owns a
dedicated bucket:

```text
seckill_stock
  activity_id (unique)
  sku_id
  warehouse_id
  total
  available
  held
  sold
  version
```

At publish, the inventory service atomically allocates `stock_total` from normal
inventory into the seckill bucket. This is the critical step that prevents
normal sales from consuming seckill stock and prevents Redis and DB from
drifting at the start.

Invariant:

```text
on_hand >= normal_reserved + seckill_allocated
seckill_allocated = seckill_available + seckill_held + seckill_sold
```

`seckill_stock_ledger` is append-only:

```text
id, activity_id, delta, balance_after, reason, ref_no, created_at
UNIQUE (activity_id, reason, ref_no)
```

Reasons include `ALLOCATE`, `HOLD`, `RELEASE`, `SELL`, `EXPIRE`, `RECONCILE`.

### 5.3 Qualification

```text
seckill_qualifications
  id
  activity_id
  user_id
  token_hash
  status                 -- ISSUED, ORDERING, ORDERED, FAILED, EXPIRED
  order_no
  expires_at
  created_at, updated_at
  UNIQUE (activity_id, user_id)   -- first version: one attempt per user
  UNIQUE (token_hash)
```

A qualification is not an order. It is a bounded right to create one.

### 5.4 Order linkage

The order service gets a server-only `CreateSeckillOrder` internal RPC:

```text
CreateSeckillOrderRequest {
  activity_id
  qualification_token
  user_id
  sku_id
  quantity
  price_cent
  idempotency_key
}
```

It creates the order and reserves inventory from the seckill bucket while
bypassing coupons and the promotion engine. The public order API must reject
client-generated seckill orders; only the worker can call this RPC.

## 6. Redis design

Use a dedicated Redis deployment (or at minimum a dedicated logical database)
for seckill. Do not share the catalog cache's Redis with the stock hot key.

### 6.1 Keys

Use a hash tag so a Lua script can touch all keys in one Redis Cluster slot:

```text
seckill:{activity_id}:meta        hash  status, start_at, end_at, price
seckill:{activity_id}:stock       integer available count
seckill:{activity_id}:users       hash  user_id -> token_hash
seckill:{activity_id}:tokens      hash  token_hash -> status|order_no
```

TTL: `end_at + 2h` (or a documented safety window). A missing key after end is
expected; do not resurrect stock after the sale.

### 6.2 Atomic admission Lua

The script is executed with `go-redis` `Script.Run` (EVALSHA with NOSCRIPT
fallback). It performs all of the following in one round trip:

```text
1. read meta/status and server-side start/end
2. if user already has a token:
      return ALREADY_ISSUED with existing token status (idempotent)
3. if per-user marker exists: return ALREADY_PARTICIPATED
4. if stock <= 0: return SOLD_OUT
5. DECR stock
6. create a random token on the application side, store token_hash
7. HSET users[user_id] = token_hash, tokens[token_hash] = ISSUED
8. set TTLs
9. return ISSUED
```

Important:

- The token is generated by crypto/rand in Go, not in Lua.
- Only `token_hash` is stored in Redis/MySQL. The raw token is returned once to
  the client and is bound to the authenticated user.
- The script does **not** touch MySQL or NATS. Admission latency stays bounded.
- If Redis is unavailable, fail closed with 503. Do not fall back to MySQL.

### 6.3 Compensation

If publishing the NATS command fails after admission:

```text
seckill:{activity}:compensate
  INCR stock
  HDEL users[user_id]
  HDEL tokens[token_hash]
```

The compensation path is also a Lua script and is idempotent by `token_hash`.
If compensation itself fails, record a reconciliation task; never silently
ignore it.

### 6.4 Hot-key scaling

Start with one activity key. It is the simplest correct design and is usually
enough for a single activity on a well-provisioned Redis. Do **not** shard
prematurely.

If load tests show a single-key ceiling:

- shard stock into N buckets with one hash tag per activity;
- choose a bucket by hash(user_id) or a random slot;
- if a chosen bucket is empty, try a bounded number of other buckets;
- aggregate the admin/read view from the ledger, not from the shards;
- keep per-user atomicity in a single user key.

This is a documented phase-2 optimization, not part of the first implementation.

## 7. NATS JetStream

Add a separate stream:

```text
Stream: SECKILL
Subjects: seckill.order.requested.>
Retention: WorkQueuePolicy
Storage: File
Replicas: 3 in production
MaxAge: 24h
```

Publish:

```text
subject: seckill.order.requested.{activity_id}
Nats-Msg-Id: <token_hash>
```

The worker uses a durable pull consumer:

```text
Durable: seckill-order-worker
AckPolicy: Explicit
MaxAckPending: bounded (worker_concurrency * small factor)
AckWait: 30s
MaxDeliver: 5
```

Rules:

- Publish with `Nats-Msg-Id` for broker-side deduplication.
- Ack only after the order transaction commits and the qualification is marked
  ORDERED.
- On transient DB/inventory errors, `NakWithDelay` with backoff.
- After `MaxDeliver`, move to a dead-letter subject/stream and mark FAILED.
- Never use an in-memory Go channel as the durable queue.

Current repo uses `MALL` stream for `shop.>`; `SECKILL` is separate so a
seckill backlog cannot evict or delay normal order/payment events.

## 8. Request lifecycle

### 8.1 Publish/warmup

1. Admin creates campaign + seckill activity (DRAFT).
2. Admin publishes a snapshot: price, stock, start/end, per-user limit.
3. Inventory service allocates the DB bucket atomically.
4. Seckill prewarm writes Redis meta and stock from the DB bucket.
5. Activity becomes READY, then ACTIVE at start time by server clock.

If prewarm/allocate fails, the activity cannot become ACTIVE.

### 8.2 Reserve

```text
POST /api/v1/seckill/activities/{id}/reserve
Authorization: Bearer ...
Idempotency-Key: <client generated UUID>
```

Flow:

1. Edge WAF/rate limit.
2. BFF auth, user/IP/device rate limit.
3. Activity metadata from local cache/Redis (server time).
4. Redis Lua admission.
5. NATS publish command.
6. Return:

```json
{
  "token": "raw-one-time-token",
  "status": "QUEUED",
  "expires_at_unix": 1790000000,
  "poll_after_ms": 500
}
```

Status codes:

- `202` queued
- `200` already ordered/issued (idempotent)
- `409` already participated
- `410` ended
- `429` rate-limited, with `Retry-After`
- `503` Redis/NATS unavailable, fail closed
- `409` sold out vs `410` ended must be distinguishable to the UI.

### 8.3 Worker creates the order

1. Pull command.
2. Check qualification/token hash/user binding.
3. DB transaction:
   - insert idempotency row `(token_hash)`;
   - call order `CreateSeckillOrder` with fixed price;
   - mark qualification ORDERING before the call, ORDERED after commit.
4. On success publish `seckill.order.created` (outbox or direct event after
   commit) and set order_no in Redis token hash.
5. On failure:
   - transient: NakWithDelay;
   - permanent: release DB bucket if held, compensate Redis, mark FAILED.

Do not hold a database transaction open across the NATS publish. Commit first,
then publish; use the existing outbox pattern if an event is required.

### 8.4 Client waits

```text
GET /api/v1/seckill/qualifications/{token}
```

Returns `QUEUED`, `ORDERING`, `ORDERED` + `order_no`, `FAILED` + reason,
`EXPIRED`.

Client polling:

- 500-1000 ms with jitter;
- exponential backoff after 10 failed polls;
- stop immediately on ORDERED/FAILED/EXPIRED;
- do not poll a stock endpoint at high frequency.

## 9. Consistency and failure handling

### 9.1 Invariants

1. Redis stock never goes negative.
2. `successful_orders <= seckill_stock.sold + seckill_stock.held`.
3. Every ORDERED qualification has exactly one order.
4. Every order for a seckill activity has a qualification and a ledger row.
5. No user can hold more than `per_user_limit` successful/active
   qualifications.
6. Expired/failed qualifications are returned to the bucket at most once.

### 9.2 Failure matrix

| Failure | Behavior |
|---|---|
| Redis down | Admission returns 503; no DB fallback; alert |
| NATS down | Compensate Redis and return 503; do not accept an unqueued token |
| Worker down | JetStream retains commands; queue-depth HPA; admission can be paused by circuit breaker |
| MySQL down | Worker retries with delay; admission may continue until queue threshold, then shed load |
| Publish succeeds, response lost | Client retries with same idempotency key; Redis returns existing token |
| Worker crashes after DB commit before ack | Redelivery; idempotency row/order unique key makes it a no-op |
| Redis restart | Rebuild stock/qualifications from DB bucket + active DB qualifications; never assume a missing key means unlimited stock |
| Redis lost after Lua before publish | Token may be lost; stock was decremented; reconciliation/release or explicit stock reset; no oversell |
| Order expires unpaid | Existing order expiry releases DB inventory; seckill bucket release policy is explicit; do not automatically reopen unless activity is still active |
| Duplicate command | `Nats-Msg-Id` + token/order unique keys |
| Clock skew | Server time only; clients display server offset, never decide eligibility |

### 9.3 Reconciliation

A low-frequency worker:

- compares DB bucket/ledger with Redis stock and active qualifications;
- repairs mismatches with a ledger row and an alert;
- expires stale qualifications (`ISSUED`/`ORDERING` older than TTL);
- moves dead commands to DLQ;
- runs on a single leader via a DB advisory lock or a Kubernetes Lease, not an
  ad-hoc Redis lock.

Reconciliation must be **report-first**: log/alert the mismatch before applying
a repair. Automatic correction without metrics hides bugs.

### 9.4 Order cancellation/payment timeout

Policy decision required:

- Seckill stock is normally **not** returned to the public sale after a
  successful order is created and then cancelled; it is a sold opportunity.
- If the business wants to re-release unpaid orders, do it through the seckill
  bucket with explicit `RELEASE` ledger semantics and Redis increment only when
  the activity is still ACTIVE.

Do not silently reuse a cancelled order's stock for a different user without a
ledger row.

## 10. Load control and anti-bot

Buy mature components; do not build these from scratch:

- **Edge WAF / waiting room**: Cloudflare WAF + Waiting Room, AWS WAF, or an
  Envoy-based edge with a tested global rate-limit service.
- **Distributed rate limit**: `go-redis/redis_rate` (GCRA) or Envoy Redis
  ratelimit; local `golang.org/x/time/rate` only as a fast pre-check.
- **Circuit breaker**: `sony/gobreaker` or Envoy outlier detection; fail fast
  when downstream error rate/latency crosses a threshold.
- **Queue/backpressure**: NATS JetStream consumer bounds; reject earlier with
  `429/503` rather than growing memory.
- **Load test**: Grafana k6; not a custom benchmark loop.
- **Observability**: OpenTelemetry traces/metrics/logs; Prometheus + Grafana.

Layered limits:

```text
edge IP/ASN rate
  -> authenticated user rate
  -> per-activity admission cap
  -> Redis stock
  -> JetStream MaxAckPending
  -> worker DB/inventory concurrency
```

Anti-bot:

- WAF managed rules and IP reputation at the edge;
- CAPTCHA/Turnstile only on risk signals, not every request;
- device/session fingerprint optional and privacy-reviewed;
- velocity limits by user/IP/device;
- never trust client-supplied timestamps or prices;
- one-time tokens bound to user and activity.

## 11. API surface

Public/customer:

```text
GET  /api/v1/seckill/activities/{id}
POST /api/v1/seckill/activities/{id}/reserve
GET  /api/v1/seckill/qualifications/{token}
```

Admin:

```text
POST /api/admin/v1/seckill/activities
PUT  /api/admin/v1/seckill/activities/{id}
POST /api/admin/v1/seckill/activities/{id}/publish
POST /api/admin/v1/seckill/activities/{id}/pause
POST /api/admin/v1/seckill/activities/{id}/warmup
GET  /api/admin/v1/seckill/activities/{id}/stats
```

Every mutation is audited. Admin publish is idempotent by `(activity_id,
version)`.

## 12. Deployment and capacity

- `seckill-api`: stateless, HPA on RPS/CPU, PodDisruptionBudget, read-only
  activity cache.
- `seckill-worker`: HPA on consumer pending messages (KEDA or custom metrics),
  not CPU alone.
- Separate Redis with HA: Sentinel or Cluster; AOF, `appendfsync everysec` at
  minimum; persistence/HA tested. Current single-node dev Redis is not
  production seckill infrastructure.
- NATS JetStream 3 replicas in production; file storage; resource limits.
- MySQL: separate connection pool for seckill, bounded max connections; no
  long transactions in the hot path.
- Network: seckill pods have their own NetworkPolicy; only gateway -> seckill
  API and worker -> order/inventory/payment.
- Capacity is not guessed. Before launch, define expected peak RPS, stock size,
  and worker drain time; prove them with k6 and a realistic DB/Redis topology.

SLO targets (to be confirmed with the business):

- admission API availability 99.9% during the sale;
- admission p99 latency target in the low hundreds of milliseconds;
- zero oversell;
- 99%+ of accepted qualifications either become orders or are explicitly
  released within the documented TTL.

## 13. Testing and acceptance

A production implementation is not accepted until:

1. **Unit tests** for Lua scripts against a real Redis (not only miniredis if
   its Lua semantics differ).
2. **Integration tests** with MySQL + Redis + NATS + order/inventory:
   - duplicate reserve;
   - publish failure compensation;
   - worker crash after commit;
   - NATS redelivery;
   - DB deadlock/retry;
   - expiry and release.
3. **Load test** with k6:
   - burst above expected peak;
   - p50/p95/p99 admission;
   - DB/Redis CPU and connection saturation;
   - queue depth and drain time;
   - no oversell under concurrency.
4. **Chaos tests**:
   - kill Redis during admission;
   - kill worker during order creation;
   - restart NATS;
   - pause MySQL;
   - verify no oversell, bounded error rate and recovery.
5. **Reconciliation test**: deliberately corrupt Redis stock; verify alert and
   repair.
6. **Security test**: replay token, token from another user, forged price,
   direct order API bypass, rate-limit bypass.

The invariant checker must run in CI/integration and fail the test on any
violation.

## 14. Phased delivery

```text
S0  design sign-off, schema/contract, load-test harness, capacity target
S1  activity admin + inventory bucket allocation + Redis warmup
S2  Redis Lua admission + token + rate limiting + unit/integration tests
S3  NATS worker + CreateSeckillOrder + compensation + order/payment E2E
S4  edge WAF/waiting room, observability, HA, k6/chaos, runbook
S5  storefront seckill page and admin stats
```

S0/S1 must be complete before any marketing traffic. Do not launch with S2 only.

## 15. Build/buy decisions

Use:

- Redis 8 + `go-redis` Lua scripts.
- NATS JetStream (already present).
- MySQL and existing order/inventory/payment state machines.
- `redis_rate` or edge ratelimit for distributed limits.
- Cloudflare/AWS WAF and waiting room if available; otherwise Envoy + tested
  rate-limit service.
- k6, OpenTelemetry, Prometheus/Grafana.
- Kubernetes HPA/KEDA and PodDisruptionBudget.

Do not build:

- a custom message queue;
- a custom distributed lock;
- a custom WAF/bot-detection model;
- a custom load generator;
- a second order/payment state machine.

## 16. Open decisions

1. ~~Expected peak RPS and concurrent users per sale.~~ Resolved for first
   target: 10,000 concurrent users, 100 units; design for 10k-20k admission RPS
   with 2x headroom.
2. Single SKU vs multi-SKU activity; first version assumes single SKU.
3. One attempt per user vs retry after cancellation.
4. Whether unpaid/cancelled stock is re-released while the activity is ACTIVE.
5. Managed edge WAF/waiting room vs self-hosted Envoy.
6. Redis HA topology (Sentinel vs Cluster) and RPO/RTO.
7. Whether seckill order creation reuses `OrderService.CreateOrder` with a
   source flag or a dedicated `CreateSeckillOrder` RPC. Recommendation: a
   dedicated RPC that calls the same order/inventory state machine.

## 17. Current repo gaps

S1 now provides:

- `mall_seckill.seckill_activities` and qualification table placeholder;
- `mall_inventory.seckill_stocks` and stock ledger;
- inventory `AllocateSeckillStock` / `ReleaseSeckillStock` / `GetSeckillStock`;
- seckill activity admin API and scheduler;
- Redis warmup keys and activity/bucket smoke test.

Still missing:

- WAF/OpenTelemetry/k6 execution against a production-like endpoint;
- reconciliation/lease worker;
- frontend seckill page.
- Redis and NATS are still single-replica in the current dev cluster; production
  HA is required before launch.
- Promotion campaigns exist but must not be used as the seckill stock source.

The next implementation step should be **S0 + S1**, not a synchronous
"decrement Redis and insert order" demo.

## 18. Capacity decision: 10k concurrent users, 100 units

The first production target is deliberately asymmetric:

```text
concurrent users      10,000
seckill stock         100
successful orders     <= 100
expected rejections   ~9,900
```

This is a **low-stock, high-contention** sale. It is not a high-throughput stock
problem; it is an admission/load-shedding problem. The design must make the
9,900 losers cheap and fast, and make the 100 winners reliable.

### 18.1 Target envelope

These are design targets to prove with k6, not measured claims:

```text
admission burst        10k-20k requests/second for 5-10 seconds
admission p99          <= 200 ms at the edge/API (excluding client network)
successful orders      100
queue messages         <= 200 (100 expected)
API replicas           min 4, HPA 4-20
worker replicas        min 2, HPA 2-8
Redis                  HA primary/replica, one hot activity key
NATS JetStream         3 nodes, stream replicas 3 in production
MySQL                  only ~100 order writes, normal pool
```

The API must reject sold-out requests in the Redis Lua round trip. It must not
enqueue, write MySQL, or call order/inventory for a request that lost the stock
race.

### 18.2 What this means

- **Do not shard the stock key yet.** One Redis key and one Lua script can
  serialize the 100 decrements. Cluster/sharding adds failure modes without
  helping a 100-unit sale.
- **Do not add Kafka.** Only ~100 order commands are produced; existing NATS
  JetStream is more than sufficient.
- **Do not build a lottery.** First-come-first-served is the business rule.
- **Do not queue all 10k users.** Once stock reaches zero, return the business
  result immediately. A waiting room may smooth the edge, but the origin should
  not store 9,900 useless jobs.
- **Do not write a qualification to MySQL on the hot path.** Redis admission +
  asynchronous worker keeps DB writes at ~100.
- **Do not use a process-local mutex or in-memory queue.** API and worker
  replicas must share Redis/NATS state.

### 18.3 Cluster behavior

All application replicas are stateless:

- JWT auth, no sticky session.
- Activity metadata is cached with a version/TTL; Redis remains authoritative
  for stock.
- Admission always executes the shared Redis script; no replica-local counter.
- Workers use a durable JetStream consumer; duplicate delivery is handled by
  unique keys.
- Reconciliation uses a Kubernetes Lease or DB advisory lock so only one
  replica repairs at a time.
- MySQL uniqueness enforces `(activity_id, user_id)` and order idempotency.

The Redis key design already uses a hash tag:

```text
seckill:{activity_id}:stock
seckill:{activity_id}:users
seckill:{activity_id}:tokens
```

This works with a single Redis, Sentinel, and Redis Cluster without changing the
Lua contract. The first deployment can use Sentinel for HA; Redis Cluster is a
later infrastructure change, not an application rewrite.

### 18.4 Middleware expansion contract

Even though the first deployment may use single-replica dev Redis/NATS, the
application must be written for the clustered future:

- Redis scripts use only keys with the same `{activity_id}` hash tag and avoid
  cross-slot `MULTI`.
- NATS publish uses `Nats-Msg-Id`; consumers use durable names and explicit ack.
- No local cache is treated as stock truth.
- No background job assumes it is the only running instance; use a Lease.
- MySQL connections are bounded per pod; no long transaction in admission.
- Configuration comes from environment/ConfigMap, not pod-local files.
- Readiness/liveness are independent of Redis/NATS connectivity so a transient
  dependency failure does not remove every API pod and cause a total outage;
  the endpoint returns 503 and metrics/alerts fire.

### 18.5 Load and chaos acceptance for this envelope

k6 scenarios:

1. 10k VUs, stock 100, one activity, all unique users.
2. Duplicate reserve by the same user.
3. Redis fail/restart during the burst.
4. Worker stop/restart while 100 commands are pending.
5. NATS restart after publish before ack.
6. MySQL latency injection during worker order creation.
7. Assert: orders <= 100, stock never negative, no duplicate user order,
   accepted qualification either ORDERED or explicitly RELEASED/FAILED.

The load test must verify the 9,900 losers receive a fast business rejection,
not a timeout or a 5xx.

## 19. Scale-out design: 100x users, 10 SKUs, 100 units each

This is a different regime from the first 10k-user/100-unit target:

```text
concurrent users       1,000,000            (100x)
SKUs                    10
stock per SKU           100
successful orders       <= 1,000
expected rejections     ~999,000
```

The stock is still tiny. The hard problem is **edge admission and fast
rejection**, not order throughput. Do not scale the database or worker for
999,000 losers; scale the edge, gateway and admission tier.

### 19.1 Capacity shape

If the 1M users arrive over 10 seconds, the origin sees roughly 100k RPS. If
they arrive in one second, the origin cannot and should not absorb 1M RPS. The
edge waiting room must shape the burst before it reaches `seckill-api`.

Recommended starting topology (to be proven in a dedicated performance
environment, not on the local k3s VM):

```text
Edge WAF / Waiting Room / CDN        managed or tested Envoy edge
Gateway replicas                    20-40, pre-scaled before the sale
seckill-api replicas                50-100, HPA + pre-scale, stateless
Redis                               dedicated 3-master/3-replica Cluster
NATS JetStream                      3 nodes, stream replicas=3
seckill-worker                      2-4 replicas (only ~1k commands)
MySQL                               ~1k holds/orders, normal HA
Frontend                            CDN/static or cached SSR, not per-user SSR
```

These are design targets. They must be validated with distributed k6 in an
isolated performance cluster; do not run a 1M-user test against the local VM.

### 19.2 Ten SKUs: model them as ten activities under one campaign

The current schema already has `UNIQUE(campaign_id, sku_id)` and one Redis stock
key per activity. The lowest-risk scale change is:

```text
campaign C
  ├─ seckill activity C/SKU-1  stock 100
  ├─ seckill activity C/SKU-2  stock 100
  └─ ... 10 rows
```

Each activity keeps:

```text
seckill:{activity_id}:meta
seckill:{activity_id}:stock
seckill:{activity_id}:users
seckill:{activity_id}:tokens
```

This requires **no schema change**. Add a bulk-create/group-by-campaign admin
operation and show the ten activities as one campaign group in the storefront.

If the product requirement is a single page with a SKU picker and **one
campaign-level per-user limit**, add a multi-SKU child table instead:

```text
seckill_activity_skus(activity_id, sku_id, price_cent, stock_total, ...)
seckill_qualifications(activity_id, sku_id, user_id, ...)
```

Token and reservation keys must include `sku_id`. Do not overload the current
single-SKU qualification with a nullable SKU; make the composite key explicit.
For the first 100x rollout, prefer ten activities under one campaign: it avoids
a schema migration while preserving the proven stock/order path.

### 19.3 Redis

Ten stock keys are not automatically a hot-key problem. At 100k RPS total, each
SKU key sees roughly 10k RPS; a well-provisioned Redis Cluster can handle that.
Do not shard stock into buckets before measuring per-key latency and CPU.

Use:

- dedicated Redis Cluster for seckill, not the catalog cache;
- hash tags per activity so the Lua script stays single-slot;
- 3 masters + 3 replicas, AOF, tested failover;
- a short negative cache for `SOLD_OUT` to avoid 999k Lua calls after stock is
  zero (invalidate on activity version/publish; never use local cache as the
  source of truth before sold out);
- Lua scripts unchanged; all stock mutations remain one round trip.

Only if a single activity key is measured above the Redis shard ceiling should
stock be sharded. Sharding requires per-shard stock, a user marker strategy and
a cross-shard reconciliation design; it is not a first-response optimization.

### 19.4 API and gateway

- Make `seckill-api` stateless and independently scalable; no request affinity.
- Pre-scale before the sale instead of relying on reactive CPU HPA; HPA is a
  safety net, not the launch plan.
- Keep JWT validation local; never call identity per admission.
- Put a bounded admission rate at the edge. The origin should not accept more
  than it can drain; return `429`/`503` with `Retry-After`.
- The general gateway can remain the BFF, but at 100k+ RPS monitor its CPU and
  connection pools and consider a dedicated seckill route; do not add another
  synchronous service hop without a measured need.
- Return `SOLD_OUT` immediately for the 999k losers; do not enqueue or write
  MySQL for them.

### 19.5 Frontend/CDN

At 1M users, the landing page itself must not SSR per request:

- serve storefront/static assets through a CDN;
- cache public activity metadata for a short TTL;
- make the seckill page hydrate from a cached activity snapshot;
- only accepted qualifications poll `GET /qualifications/:token`;
- losers should stop after `SOLD_OUT`/`ENDED`, not poll at high frequency;
- use server time + offset; no client-side eligibility decisions.

### 19.6 Queue and database

The queue only carries about 1,000 commands, so NATS sizing is not the critical
path. Keep:

```text
MaxAckPending bounded
AckWait 30s
MaxDeliver 5
DLQ + replay
```

MySQL writes remain roughly 1,000 qualifications/orders; the 999k losers never
touch it. If the queue or DB is slow, shed at admission; do not let HTTP
requests accumulate.

### 19.7 Multi-AZ / HA

- Redis Cluster with replicas in separate zones.
- NATS JetStream 3 replicas across zones.
- MySQL primary + replicas/backup; bounded per-pod connections.
- Gateway/API PodDisruptionBudget and topology spread.
- Pre-warm connections and verify failover before the sale.
- Fail closed on Redis loss; never fall back to MySQL for admission.

### 19.8 What not to do

- Do not run 1M VUs against the local k3s VM.
- Do not create one Redis key shard per user.
- Do not write a qualification row for every loser.
- Do not build a custom WAF/waiting room or custom load generator.
- Do not add Kafka for 1,000 order commands.
- Do not scale MySQL for traffic that should have been rejected at the edge.

### 19.9 Rollout sequence for this target

1. Bulk-create ten activities under one campaign and group them in the storefront.
2. Move admission behind a managed edge waiting room/WAF; cap origin RPS.
3. Pre-scale gateway/seckill-api; add PDB/HPA and test failover.
4. Move to dedicated HA Redis/NATS; add negative-cache sold-out behavior.
5. Add dashboards/alerts for admission RPS, Redis p99/CPU, queue depth, 429/503.
6. Run distributed k6 from an isolated performance environment; validate the
   100k RPS origin target, not 1M RPS at the origin.
