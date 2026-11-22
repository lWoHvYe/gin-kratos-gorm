# 架构现状基线（Spec 接收前置）

- 角色：System Architect
- 用途：在收到 Product Owner 的 `Spec.md` 之前，先把**当前系统的真实状态**固化为可复核基线。它不是新设计，不是 ADR，不改变任何行为。
- 规则：所有结论必须带 `文件:行号`；读不到或未验证的标 `[待验证]`。文档与代码冲突时以代码为准。
- 快照：`HEAD = 4e0b8b4`（2026-11-22 10:40 +08，`feat(order): project explicit group refund state`）；工作区仅 `AGENTS.md` 修改与 `prompt/` 未跟踪。

---

## 1. 系统拓扑与事实所有权

10 个领域服务 + 2 个 BFF + 1 个可选 AI 面，每服务独立 schema，服务间只走 gRPC/事件：

```text
storefront/admin  ──HTTP──>  gateway(BFF) / admin(BFF)
                                   │ gRPC
        ┌──────────────┬───────────┼───────────┬──────────────┐
        v              v           v           v              v
    identity        catalog     cart/pricing  order        payment
                                                   │            │
                                                   v            v
                                              fulfillment   aftersales
                                                   \          /
                                                    inventory
        seckill ──(Redis Lua 准入 + JetStream worker)──> inventory/order/payment
        ai（可选面，故障不得影响交易）
```

所有权矩阵（唯一事实来源，与 [docs/ext-review.md](ext-review.md) §2.2 一致）：

| 所有者 | 唯一负责的事实 | 不得承担的隐含语义 |
|---|---|---|
| Payment | captured / refunded / 回调去重 / 退款累计上限 | 不判断售后商品是否回库 |
| Order | 订单生命周期、退款异步投影、发货许可与竞态裁决、可用动作 | 不把退款态压回主 `status` |
| AfterSales | 申请额度占用、退货进度、回库待办、退款准入 | 不拥有实际打款事实 |
| Inventory | 真实库存、预留、整单操作进度、回库幂等 | 不拥有审批/退款状态 |
| Fulfillment | 运单与物流履约 | 不猜测订单/退款状态 |
| BFF/前端 | 同一组合读模型与 `available_actions` | 不各自推导第二套状态机 |

---

## 2. 运行时原语

### 2.1 Transactional Outbox

- 业务写与事件写同一 `*sql.Tx`：`outbox.Append(ctx, tx, ...)`（[outbox.go:34](internal/platform/outbox/outbox.go#L34)）。
- 中继用 `FOR UPDATE SKIP LOCKED` 短事务认领，提交后再发 NATS，避免慢 NATS 持锁（[outbox.go:191-201](internal/platform/outbox/outbox.go#L191-L201)）。
- 租约 `lease_owner/lease_until` 防多副本重复完成（[outbox.go:243-256](internal/platform/outbox/outbox.go#L243-L256)）。
- 指数退避，20 次后 dead-letter 并计数（[outbox.go:57](internal/platform/outbox/outbox.go#L57)、[outbox.go:258-278](internal/platform/outbox/outbox.go#L258-L278)）。
- 7 天后归档/清理，先归档 dead-letter 再删（[outbox.go:308-371](internal/platform/outbox/outbox.go#L308-L371)）。
- 持久化 `traceparent`，relay 用 link 而非 child span（[outbox.go:425-445](internal/platform/outbox/outbox.go#L425-L445)）。

### 2.2 DTM

- 下单 Saga：`gid = order-create:<order_no>`，`WaitResult=true`、`TimeoutToFail=30`、`RetryLimit=3`（[dtm.go:37-43](internal/platform/dtm/dtm.go#L37-L43)、[service.go:1152](internal/services/order/service.go#L1152)）。
- 分支必须 `dtm.Branch`，业务写与 outbox 共用该 `*sql.Tx`（[dtm.go:53-59](internal/platform/dtm/dtm.go#L53-L59)）。
- 支付确认 TCC：`gid = payment-confirm:<payment_no>`，三个参与者 order / inventory / pricing（[payment/tcc.go:55-72](internal/services/payment/tcc.go#L55-L72)）。
- 取消不是 DTM：本地 `PENDING_PAYMENT -> CANCELLED` + `shop.order.cancelled.v1` 事件补偿（[docs/dtm.md](dtm.md)、[service.go:467-469](internal/services/order/service.go#L467-L469)）。
- 禁止 `MustGenGid` / `MustGetDtmClient`（[scripts/check-architecture.sh:47-52](scripts/check-architecture.sh#L47-L52)）。

### 2.3 消费端统一策略

`consumer.Subscribe` 提供单一策略：每消息超时、有限重试、`<subject>.dlq`、panic 恢复、指标（[consumer.go:56-134](internal/platform/consumer/consumer.go#L56-L134)）。`Permanent(err)` 跳过重试（[consumer.go:46-52](internal/platform/consumer/consumer.go#L46-L52)）。

### 2.4 幂等身份（数据库唯一键是最后防线）

| 场景 | 幂等键 | 证据 |
|---|---|---|
| 下单 | `(user_id, client_request_id)` | [0001_init.sql:26](migrations/order/0001_init.sql#L26) |
| 支付意图 | `(order_no, amount_cent, channel)` | [payment/0001_init.sql:22](migrations/payment/0001_init.sql#L22) |
| 支付回调 | `(channel, event_id)` | [payment/0001_init.sql:39](migrations/payment/0001_init.sql#L39) |
| 退款 | `(payment_no, idempotency_key)` | [payment/0002_partial_refunds.sql:38](migrations/payment/0002_partial_refunds.sql#L38) |
| 售后申请 | `(user_id, client_request_id)` | [aftersales/0006_client_request_id.sql:10](migrations/aftersales/0006_client_request_id.sql#L10) |
| 库存预留 | `(order_no, sku_id, warehouse_id)` | [inventory/0016_reservation_warehouse_keys.sql:5](migrations/inventory/0016_reservation_warehouse_keys.sql#L5) |
| 库存整单操作 | `inv:v2:<type>:<order_no>:<scope_hash[:32]>` | [operations.go:199-202](internal/services/inventory/operations.go#L199-L202) |
| 秒杀订单号 | `id.OrderForSeckill(tokenHash)` 确定性派生 | [id.go:28-31](internal/platform/id/id.go#L28-L31) |
| 运单 | `(order_no, fulfillment_group_no)` | [fulfillment/0003_group_shipments.sql:6](migrations/fulfillment/0003_group_shipments.sql#L6) |
| 售后回库任务 | `(after_sale_no, sku_id, warehouse_id, group_no)` | [aftersales/0011_group_items.sql:13](migrations/aftersales/0011_group_items.sql#L13) |

### 2.5 错误模型

统一 `apperr`，HTTP/gRPC 映射集中（[apperr.go:14-24](internal/platform/apperr/apperr.go#L14-L24)、[apperr.go:68-121](internal/platform/apperr/apperr.go#L68-L121)）。资金/库存路径未预期异常必须 `FAILED_PRECONDITION`/`UNAVAILABLE` 阻断，不许 try-catch 放行。

---

## 3. 领域状态机

### 3.1 订单生命周期 `orders.status`

```text
PENDING_PAYMENT ──支付确认──> PAID ──首组发货?──> ALLOCATED ──全部发货──> SHIPPED ──全部收货──> COMPLETED
       │                       │                      │
       └──取消/超时/补偿──> CANCELLED <──取消(仅未发货)┘
```

- 写入点：`PENDING_PAYMENT -> PAID`（[service.go:634](internal/services/order/service.go#L634)、[tcc.go:92](internal/services/order/tcc.go#L92)）；`-> CANCELLED`（[service.go:423](internal/services/order/service.go#L423)）；`-> ALLOCATED/SHIPPED/COMPLETED` 由运单投影消费者写（[consumer.go:71-159](internal/services/order/consumer.go#L71-L159)）。
- 所有迁移走条件更新 + `version` 自增（[repo.go:252-260](internal/services/order/repo.go#L252-L260)）。
- `CLOSED`、`REFUNDING`、`PARTIALLY_REFUNDED`、`REFUNDED` 仍是 `knownStatuses` 成员（[repo.go:16-41](internal/services/order/repo.go#L16-L41)），但**不再写入 `orders.status`**（见 §9 漂移）。

### 3.2 资金维度与生命周期正交

- `refund_state`：`NONE -> REFUNDING -> PARTIAL -> FULL`（[settlement.go:17-37](internal/services/order/settlement.go#L17-L37)）。
- `refund_status`：`'' -> PARTIALLY_REFUNDED -> REFUNDED`，由 `shop.payment.refunded.v1` 单调投影（[refund_consumer.go:53-99](internal/services/order/refund_consumer.go#L53-L99)）。
- 退款投影**永不改生命周期状态**（[refund_consumer.go:50-52](internal/services/order/refund_consumer.go#L50-L52)）。

### 3.3 履约组 `order_fulfillment_groups`

```text
PENDING ──支付──> ALLOCATED ──发货──> SHIPPED ──收货──> RECEIVED
   │                  │
   └──取消────────────┴──> CANCELLED
```

- 唯一键 `(order_no, group_no)`、`(order_no, seq)`（[order/0011:18-19](migrations/order/0011_fulfillment_groups.sql#L18-L19)）。
- 退款撤销发货许可后，未发货组**读模型呈现 CANCELLED**，但持久化 `status` 仍是 `ALLOCATED`（[settlement.go:80-88](internal/services/order/settlement.go#L80-L88)）。
- 组退款态 `refund_state`：`NONE -> REFUNDING -> REFUNDED`（[0013_group_refund_state.sql](migrations/order/0013_group_refund_state.sql)、[aftersale_consumer.go](internal/services/order/aftersale_consumer.go)）。

### 3.4 发货许可 `order_shipping_permits`

```text
NONE ──AuthorizeShipment──> CONSUMED
  │
  └──BlockShipmentForRefund──> REVOKED      （CONSUMED 时退款不降级，允许发货后退款）
```

- 主键 `(order_no, fulfillment_group_no)`（[order/0012](migrations/order/0012_group_shipping_permits.sql)）。
- 发货与退款在同一行上串行化（[settlement.go:289-395](internal/services/order/settlement.go#L289-L395)）。
- `GRANTED` 已定义但当前无写入点（[settlement.go:24](internal/services/order/settlement.go#L24)）。

### 3.5 运单 `shipments`

```text
CREATED ──Ship(需许可)──> SHIPPED ──Receive──> RECEIVED
   │
   └──CancelCreatedShipment(组退款)──> CANCELLED
```

- 前向单向，行锁 + 同事务 outbox（[fulfillment/service.go:272-336](internal/services/fulfillment/service.go#L272-L336)）。
- 发货前必须取得 Order 许可（[fulfillment/service.go:185-193](internal/services/fulfillment/service.go#L185-L193)）。

### 3.6 支付意图 `payment_intents`

```text
PENDING ──SUCCEEDED──> SUCCEEDED ──部分退款──> PARTIALLY_REFUNDED ──退满──> REFUNDED
   │                        │
   └──FAILED──> FAILED      └──迟到失败不降级
```

- 迟到成功被接受，迟到失败不降级已支付（[payment/service.go:471-489](internal/services/payment/service.go#L471-L489)）。
- 退款项始终独立执行 `refunded + amount <= captured`（[payment/service.go:334-336](internal/services/payment/service.go#L334-L336)）。

### 3.7 售后 `after_sales`（三个正交维度）

```text
REFUND_ONLY:     REQUESTED -> APPROVED -> REFUNDING -> REFUNDED
                              └─> REJECTED

RETURN_REFUND:   REQUESTED -> APPROVED -> RETURNING -> RETURN_RECEIVED -> REFUNDING -> REFUNDED
                                                             │
                                                             └─ restock_state: PENDING -> PROCESSING -> DONE / FAILED_PERMANENT
```

- `status` 只表达流程；`amount_state`（ACTIVE/CONSUMED/RELEASED）表达额度；`restock_state` 表达回库（[aftersales/service.go:25-50](internal/services/aftersales/service.go#L25-L50)）。
- 回库 `DONE` 前不得进入 `REFUNDING`（[service.go:396-413](internal/services/aftersales/service.go#L396-L413)）。
- 退款任务由 durable task 驱动，非状态扫描（[workers.go:260-406](internal/services/aftersales/workers.go#L260-L406)）。

### 3.8 库存预留 `stock_reservations`

```text
HELD ──Confirm──> CONFIRMED
  │                  │
  └──Release──> RELEASED <──CancelConfirm(HELD 释放 / CONFIRMED 回库)──┘
```

- 确认重复是 no-op；释放已确认是 conflict（[inventory/service.go:128-159](internal/services/inventory/service.go#L128-L159)）。
- 秒杀预留走独立桶与映射，通用路径不得扣减普通 `reserved`（[inventory/tcc.go:104-138](internal/services/inventory/tcc.go#L104-L138)）。

### 3.9 库存整单操作 `inventory_operations`

```text
PENDING -> IN_PROGRESS -> SUCCEEDED
                       └-> PARTIAL（仍有 retryable/永久失败项）
item: PENDING -> DONE | FAILED_PERMANENT | MISSING
```

- 操作身份 = `(type, order_no, scope_hash)`，不同范围不得复用（[operations.go:133-197](internal/services/inventory/operations.go#L133-L197)）。
- DTM 分支路径整单一个事务；非 DTM 路径逐项 savepoint + 进度持久化，可跳过成功项（[operations.go:204-260](internal/services/inventory/operations.go#L204-L260)）。

### 3.10 秒杀（概述）

Redis Lua 准入 + 耐久 hold + JetStream worker + DLQ/replay，库存权威在 inventory 桶而非 Redis 镜像。详见 [seckill-design.md](seckill-design.md)。资格状态含 `ORDERED/FAILED/EXPIRED/CANCELLED`（[seckill/dlq.go:47](internal/services/seckill/dlq.go#L47)）。

---

## 4. 不变量清单（新 Spec 不得违反）

| ID | 不变量 | 证据 |
|---|---|---|
| INV-01 | 订单生命周期与退款态正交，退款不得写回 `orders.status` | [settlement.go:15-16](internal/services/order/settlement.go#L15-L16) |
| INV-02 | 售后额度以 Payment captured 为基准；`SUM(effective_amount)+本次 <= captured`；已退款额度不释放 | [aftersales/settlement.go:101-122](internal/services/aftersales/settlement.go#L101-L122) |
| INV-03 | RETURN_REFUND 回库 `DONE` 前不得退款 | [aftersales/service.go:396-413](internal/services/aftersales/service.go#L396-L413) |
| INV-04 | 退款与发货竞态由许可行裁决，有唯一胜者 | [settlement.go:286-338](internal/services/order/settlement.go#L286-L338) |
| INV-05 | 支付末端始终独立校验 `refunded + amount <= captured` | [payment/service.go:327-336](internal/services/payment/service.go#L327-L336) |
| INV-06 | 已支付订单不得留下 HELD 预留；确认在标记支付之前 | [service.go:584-622](internal/services/order/service.go#L584-L622) |
| INV-07 | 所有库存/金额变更与 outbox 事件同事务 | [outbox.go:34](internal/platform/outbox/outbox.go#L34) |
| INV-08 | 批次部分成功必须记录逐项进度，不得整批反复卡在首错 | [operations.go:24-54](internal/services/inventory/operations.go#L24-L54) |
| INV-09 | `skus.stock` 只是展示投影，权威库存在 inventory | [AGENTS.md](AGENTS.md) 领域铁律 |
| INV-10 | 跨仓拆单是产品版本：订单行级拆分，单行数量不跨仓 | [allocation.go:12-16](internal/services/inventory/allocation.go#L12-L16) |
| INV-11 | 一个售后单只能属于一个履约组 | [aftersales/service.go:415-441](internal/services/aftersales/service.go#L415-L441) |
| INV-12 | 可用动作必须服务端推导，前端不得按枚举猜 | [settlement.go:134-225](internal/services/order/settlement.go#L134-L225) |
| INV-13 | 秒杀订单号确定性派生，重复投递命中同一订单 | [id.go:28-31](internal/platform/id/id.go#L28-L31) |
| INV-14 | DTM 分支必须走 barrier，不得静默降级为本地事务 | [docs/dtm.md](dtm.md) |
| INV-15 | 历史状态迁移只能前向，生产回滚走备份恢复 | [ADR 0003](adr/0003-c-production-rollout.md) |
| INV-16 | 金额一律整数分 `_cent BIGINT`，状态一律 VARCHAR，可变聚合有 `version` | [AGENTS.md](AGENTS.md) 领域铁律 |

---

## 5. 故障恢复协议矩阵

| 故障 | 现有协议 | 关键点 | 证据 |
|---|---|---|---|
| 中继进程崩溃（outbox） | 租约 + 至少一次投递 | 崩溃者持有的行在 `leaseTTL` 后由其他副本重试 | [outbox.go:151-187](internal/platform/outbox/outbox.go#L151-L187) |
| 消费端崩溃/重投 | JetStream durable + 逐项进度 | 重投跳过 `DONE`，只推进剩余项 | [operations.go:215-244](internal/services/inventory/operations.go#L215-L244) |
| 退款 worker 崩溃 | durable task + 1 分钟租约 | 到期可被其他副本认领；Payment 按 after-sale 幂等 | [aftersales/workers.go:115-154](internal/services/aftersales/workers.go#L115-L154) |
| 回库 worker 崩溃 | durable per-SKU task + 租约 | `Restock` 以 `after_sale_no+sku` 幂等 | [aftersales/workers.go:33-52](internal/services/aftersales/workers.go#L33-L52) |
| Saga 分支部分成功 | DTM 补偿 + barrier 去重 | 补偿幂等；`CancelOrderInner` 容忍两种 payload 形状 | [service.go:482-554](internal/services/order/service.go#L482-L554) |
| TCC 取消已支付 | 订单转 `REFUNDING` + `order.refund_requested` | Payment 消费后幂等退款 | [order/tcc.go:132-223](internal/services/order/tcc.go#L132-L223) |
| 支付回调重复 | `(channel,event_id)` 去重 | 重放仍 reconciliation，但不重复记账 | [payment/service.go:199-287](internal/services/payment/service.go#L199-L287) |
| 多支付意图退款 | 订单级幂等键 + 逐意图分摊 | 重试从已退额度续跑，不重复打款 | [payment/service.go:528-612](internal/services/payment/service.go#L528-L612) |
| 事件乱序/降级 | 单调投影 + 状态秩守卫 | 已 RECEIVED 不被 shipped 拉回；退款只增不减 | [consumer.go:86-94](internal/services/order/consumer.go#L86-L94)、[refund_consumer.go:60-66](internal/services/order/refund_consumer.go#L60-L66) |
| 下游超时/不可用 | fail-closed + 持久补偿 | 资金/库存路径阻断或入 outbox 待重试 | [AGENTS.md](AGENTS.md) 防御性架构 |

---

## 6. 契约面

### 6.1 同步 API（gRPC，`api/proto/mall/v1/mall.proto`）

Identity / Catalog / Inventory / Cart / Pricing / Order / Payment / Promotion / Fulfillment / AfterSales / Seckill / AI。关键交易 RPC：

- Order：`CreateOrder`、`CreateOrderInner`、`CancelOrder(Inner)`、`MarkPaid`、`Tcc{Try,Confirm,Cancel,Query}MarkPaid`、`AuthorizeShipment`、`BlockShipmentForRefund`
- Inventory：`Reserve/Confirm/Release`、`AllocateOrder`、`Restock`、`Tcc*Confirm`
- Payment：`CreateIntent`、`Callback`、`Refund`、`RefundOrder`、`GetPaymentByOrder`、`VerifyPayment`
- AfterSales：`CreateAfterSale`、`Approve`、`SubmitReturnTracking`、`ReceiveReturn`、`Refund`
- Fulfillment：`CreateShipment`、`Ship`、`Receive`

### 6.2 异步事件（`shop.<domain>.<event>.v1`，必须经 outbox）

生产者→消费者关键链路（[docs/contracts.md](contracts.md)）：

```text
order.created/paid/cancelled/allocated/shipped/completed
  -> fulfillment(order.paid 建 CREATED 运单)、inventory/pricing(order.cancelled 释放)
payment.succeeded/refunded
  -> order(退款投影)
shipment.created/shipped/delivered
  -> order(履约组/生命周期投影)
inventory.operation.completed/failed/item_failed
  -> 运维/对账
aftersale.* / refund_requested / completed
  -> order(组退款态)、fulfillment(作废 CREATED 运单)
```

### 6.3 BFF 路由

网关 C 端与 admin 端路由见 [gateway/router.go:55-106](internal/services/gateway/router.go#L55-L106)；写操作要求 `admin` 角色与审计。

---

## 7. 质量门禁

`make verify` = `fmt-check + vet + test + test-race + build + arch + dtm-contract + migrations-check + proto-guard`（[Makefile:159](Makefile#L159)）。

架构守卫强制（[scripts/check-architecture.sh](scripts/check-architecture.sh)）：

1. 服务之间禁止互相 import，只能 gRPC/contracts；
2. `internal/platform` 不得依赖 service；
3. GORM 仅允许 `catalog/gorm_repo.go`、`cart/gorm_repo.go`；
4. 禁止非确定性 DTM GID 辅助函数。

测试分层：单测 / `-race` / 真 MySQL 集成 / NATS 集成 / Playwright / chaos（[docs/architecture-review.md](architecture-review.md)）。

---

## 8. 历史反模式与「借尸还魂」位点

来源：[docs/ext-review.md](ext-review.md) §2.1、§三、§七。新 Spec 与 ADR 必须显式封堵：

| 历史反模式 | 曾经的表现 | 现行防线 | 新设计禁止 |
|---|---|---|---|
| 状态组合 if-else 补丁 | 守卫、列表、前端各读不同字段，修一个组合下一个又失效 | 单一 `orderActionPolicy` + 服务端 `available_actions` | 为每个新组合加 `if`/前端特判 |
| 脏读投影当事实 | 用 Order 异步 `refunded_cent` 当售后额度基准 | AfterSales 本地额度台账 + Payment captured | 跨服务读异步投影做准入 |
| 状态压缩 | 售后 `RECEIVED` 同时表示已收货/已回库/可退款 | `status`/`amount_state`/`restock_state` 三正交 | 把多个事实揉进一个枚举 |
| 批次首错返回 | 中间项失败导致后续饥饿，或不处理项被整批 Ack | 整单操作协议 + 逐项进度 + 按项 DLQ | 为每种错误码加特殊 Ack/Nak |
| 可变结构体序列化事件 | 从未落终态的结构体构造事件 | 事件从持久化终态构造 | helper 里依赖外层稍后赋值 |
| 假多仓 | 只改 `defaultWarehouseID` 常量，实际串仓 | 产品版本 + 显式分配/许可/运费协议 | 未获 PRD 前把常量改参数 |
| 局部缓存清理 | 每条 RAG 路径各清各的 | 版本化索引 + 统一失效 | 原地覆盖 alias/无序重建 |
| 技术栈置换式救火 | 引入 Kafka/Temporal/独立向量库/第二套网格/ORM 重写 | 现有 outbox+DTM+JetStream 保留 | 未批准技术栈 |

---

## 9. 已识别漂移与未闭合缺口（Spec 不得假设已解决）

**代码/文档漂移（以代码为准）：**

1. `knownStatuses` 仍接受 `REFUNDING/PARTIALLY_REFUNDED/REFUNDED/CLOSED` 作为 `orders.status` 过滤（[repo.go:30-41](internal/services/order/repo.go#L30-L41)），但这些值只写 `refund_status`/`refund_state`；`0010` 已把历史退款态迁回生命周期（[0010:21-38](migrations/order/0010_settlement_protocol.sql#L21-L38)）。按这些值筛订单会得到空集。
2. `CLOSED` 从未被写入 `orders.status`，仅存在于读取守卫（[service.go:407](internal/services/order/service.go#L407)）。
3. `permitGranted` 已定义但无写入点（[settlement.go:24](internal/services/order/settlement.go#L24)）。
4. `fulfillmentGroup.ReservationState` 代码只写 `HELD/RELEASED`，ADR 0002 声称含 `CONFIRMED`（[fulfillment_groups.go:65-74](internal/services/order/fulfillment_groups.go#L65-L74)）。

**未闭合缺口（来自 roadmap）：**

- P0.2 worker 重启故障注入未覆盖（[docs/roadmap-status.md](roadmap-status.md)）。
- P0.4 告警规则存在但无真实 Prometheus 触发验证。
- P2 typed outbox schema 仅核心退款/运单消费者有本地结构，多数仍 `map[string]any`。
- P2 inventory replay 只有 CLI，无 gRPC/BFF 面。
- #38 AI mesh 只做了 Helm 渲染，live 403 未实测。
- P3 全部未启动：AI Turn/Action、RAG 版本化发布、数量级拆仓、多仓成本分配、承运商、多仓秒杀。

---

## 10. Spec 接收清单（PO 交付时必须覆盖）

收到 `Spec.md` 后，我会先按此清单做完整性审查；缺项将回退给 PO，而不是由架构师补写业务规则：

1. **业务目标与非目标**：本次解决什么、明确不做什么。
2. **角色与入口**：C 端/运营/系统任务分别触发哪些动作。
3. **状态所有权**：每个状态字段的唯一事实来源，禁止一字段多语义。
4. **业务不变量**：金额、库存、履约、售后的硬约束。
5. **状态机**：允许的迁移、非法迁移的返回语义。
6. **验收标准（AC）**：必须含并发、超时、部分成功、进程崩溃、重复投递、乱序六类分支，而非仅快乐路径。
7. **幂等键**：谁生成、作用域、重试语义。
8. **失败可见性**：用户看到什么、运营看到什么、是否需要补偿/重放入口。
9. **数据保留与迁移**：新增列/表、历史回填、是否需要前向迁移。
10. **上线与回滚**：开关、灰度、回滚条件。

---

## 附：本基线的验证方式

```bash
make verify          # fmt + vet + test + race + build + arch + dtm + migrations + proto
git log -1 --format='%h %ad %s' --date=iso
```

如需复核任一结论，按文中 `文件:行号` 直接跳转；本文件只记录事实，不包含尚未批准的新设计。
