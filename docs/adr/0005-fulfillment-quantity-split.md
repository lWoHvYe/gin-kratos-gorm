# ADR 0005: 多仓按数量拆行、选仓优先级与承运商签收

- Status: **accepted**（甲方 2026-11-22 批准；Developer 可开工）
- Date: 2026-11-22
- Approved: 2026-11-22（甲方/人类指挥官；OQ-01…OQ-10 按 §15 暂定口径一并批准）
- Revision: **1.1 / Amendment 1**（2026-11-22；吸收 SPEC-0001 v1.1，回应 [偏差登记](../implementation/adr-0005-deviation-register.md) A/B/C）
- Normative: **§18 是增量契约；与前文冲突时以 §18 为准。**
- Spec: [SPEC-0001 v1.0](../specs/0001-fulfillment-quantity-split.md)（已冻结）
- Amends: [ADR 0002](0002-cross-warehouse-split-orders.md) 的 v1「行级拆分」范围；**废止基线 INV-10**
- Related: [architecture-baseline.md](../architecture-baseline.md)、[ADR 0003](0003-c-production-rollout.md)、[ADR 0004](0004-p3-0-contract-and-production-scope.md)
- Owners: Order / Pricing / Inventory / Fulfillment / AfterSales / Payment / Gateway+Admin BFF

---

## 0. 决策摘要与必答项

### 0.1 架构决策（A-01…A-12）

| ID | 决策 | 理由 |
|---|---|---|
| A-01 | 拆量是**报价事实**：预览算出方案，下单用预留 Saga 重新校验，支付成功瞬间冻结 | 预览不能占库存；支付后必须不可变 |
| A-02 | 冻结点是 **`PAID` 与 `allocation_frozen_at` 同一事务提交**；冻结后行/组/仓只读 | 直接回答 Spec Q1 |
| A-03 | 行实付额、优惠分摊、订单级运费由 **Pricing 唯一计算**；Order 只快照，AfterSales 只按快照计算退款 | 直接回答 Spec Q2 |
| A-04 | 退款金额由 **AfterSales 计算并持久化行台账**，调用方金额只记录为“用户期望”，不参与打款 | INV-23、AC-11 |
| A-05 | 行退款台账 `after_sale_line_ledger` 是 **S14 唯一事实源**；Order 不落任何行已退数量/金额 | INV 铁律 4 |
| A-06 | 承运商为受控清单 `fulfillment.carriers`；运单号唯一性用 **生成列唯一索引**（空值不冲突） | AC-17/18 |
| A-07 | 轨迹是**追加事件 + 单调投影**；异常轨迹单独 `exception_state`，不改运单前向状态 | INV-25、E16/E18 |
| A-08 | **签收（delivery）与收货（receipt）分离**：签收只来自承运商 `DELIVERED` 或用户确认；自动确认只产生 receipt，不伪造 delivery | 解决 S7 与 AC-21 的表面矛盾 |
| A-09 | 自动确认由 **Fulfillment 的 durable job 表** 驱动，Order 只消费事件 | 直接回答 Spec Q4 |
| A-10 | 日终对账由**各事实所有者**各自计算并落差异表，Admin BFF 只读聚合 | 直接回答 Spec Q5 |
| A-11 | 金额分摊使用共享的 **最大余数法**（`internal/platform/money`），Pricing/AfterSales 共用同一实现 | 防止两套舍入逻辑漂移 |
| A-12 | 全部能力默认关闭：`QUANTITY_SPLIT_ENABLED` / `CARRIER_REGISTRY_ENABLED` / `DELIVERY_AUTO_CONFIRM_ENABLED`；生产开启受 ADR 0003 门禁约束 | ADR 0004 §3 |

### 0.2 Spec 第 12 节五个必答问题

| 问题 | 答案 | 详见 |
|---|---|---|
| 拆量快照在哪个时点冻结，冻结后如何拒绝一切变更？ | 支付确认事务内 `status=PENDING_PAYMENT → PAID` 与 `allocation_frozen_at=NOW(3)` 同事务；行/组的写路径全部加 `allocation_frozen_at IS NULL` 守卫，快照表 append-only + 哈希校验 | §5.5、§5.6 |
| 行实付额与优惠分摊在哪个环节唯一计算，客服改价是否被禁止？ | Pricing.CreateQuote 唯一计算；Order 快照；无任何支付后改价 API，客服改价不存在的入口，SQL 守卫拒绝 | §6 |
| 签收事实如何做到“只有两个来源”，争议裁决如何留痕？ | `delivery_evidence` 只接受 `CARRIER_DELIVERED` / `USER_CONFIRM` 两种 source；争议是独立表 + 只允许 `RESOLVED_*`，不是签收来源 | §7.3、§7.4 |
| 无轨迹自动确认的调度由谁负责，如何避免重复确认？ | Fulfillment 的 `delivery_auto_confirm_jobs` + 租约 worker；`shipment_receipts` 主键唯一 + 运单状态守卫 | §7.5 |
| 日终对账的三个等式由谁产出、差异如何暴露？ | Order 产出等式 1/3，Payment 产出等式 2；各自写 `*_reconciliation_diffs`，Admin BFF 聚合只读 + 指标告警 | §11 |

---

## 1. Context

### 1.1 现状与 Spec 的差距

| 现状 | 证据 | SPEC-0001 要求 |
|---|---|---|
| 分配器整行选一个仓，放不下就拒绝 | [allocation.go:12-16](../../internal/services/inventory/allocation.go#L12-L16) | 按数量拆到最多 2 仓（INV-19） |
| 每条 quote line 只有一个 warehouse/group | [QuoteLine](../../api/proto/mall/v1/mall.proto) | 同一 SKU 可两行两仓 |
| 每组各算一笔运费，组运费之和即用户运费 | [quote_groups.go:67-114](../../internal/services/pricing/quote_groups.go#L67-L114) | 订单级只收一次，组运费只是内部分摊（INV-20） |
| 售后金额由请求方传入 `amount_cent` | [service.go:866-884](../../internal/services/aftersales/service.go#L866-L884) | 系统按行计算，不接受自填（INV-23） |
| 承运商是自由文本，运单号无唯一约束 | [fulfillment/service.go:59-163](../../internal/services/fulfillment/service.go#L59-L163) | 受控清单 + 承运商内运单号唯一 |
| 运单只有 `CREATED/SHIPPED/RECEIVED/CANCELLED` | [fulfillment/service.go:23-31](../../internal/services/fulfillment/service.go#L23-L31) | 增加轨迹状态、异常、签收证据、争议、自动确认 |
| 收货由运单 `delivered` 直接推进 | [order/consumer.go:58-60](../../internal/services/order/consumer.go#L58-L60) | 签收/收货分离，自动确认有独立调度 |
| 无拆量决策审计、无日终对账 | — | INV-28、§10 审计与日终对账 |

### 1.2 非目标（Spec 已排除，ADR 同样排除）

多仓秒杀；仓间调拨/自动跨仓回库；支付后重新分配；已支付未发货的“按组取消”；区域/时效选仓；换货/补发/维修；短信/邮件触达；选仓成本模型优化。

---

## 2. 领域模型

### 2.1 聚合与所有权（映射 Spec S1–S14）

| Spec | 事实 | 聚合/表 | 服务 | 本 ADR 的写入口 |
|---|---|---|---|---|
| S1 | 订单生命周期 | `orders.status` | Order | 支付确认、运单/收货投影、取消 |
| S2 | 订单履约投影 | `orders.shipment_status`（派生） | Order | 由组状态聚合，不落库 |
| S3 | 履约组状态 | `order_fulfillment_groups.status` | Order | 收款、发货、收货、退款撤许可 |
| S4 | 组退款态 | `order_fulfillment_groups.refund_state` | Order | 许可拦截、`aftersale.completed.v1` |
| S5 | 发货许可 | `order_shipping_permits.state` | Order | `AuthorizeShipment` / `BlockShipmentForRefund` |
| S6 | 运单状态 | `shipments.status` | Fulfillment | `Ship` / `Receive` / `CancelCreatedShipment` |
| S7 | 签收事实 | `delivery_evidence` | Fulfillment（Order 只消费） | 承运商事件、用户确认；争议裁决另表 |
| S8 | 物流轨迹 | `shipment_track_events` + `shipments.track_status` | Fulfillment | `RecordTrackingEvent` |
| S9 | 售后流程 | `after_sales.status` | AfterSales | 审核、寄回、回库、退款 |
| S10 | 申请额度 | `after_sale_amount_ledger` | AfterSales | 申请/审批/完成 |
| S11 | 回库状态 | `after_sales.restock_state` | AfterSales | 回库任务 |
| S12 | 打款事实 | `payment_intents.captured/refunded`、`refunds` | Payment | 回调、退款 |
| S13 | 真实库存 | `inventory_stocks` / `stock_reservations` | Inventory | 预留/确认/释放/回库 |
| S14 | 行已退数量/金额 | `after_sale_line_ledger` | **AfterSales 唯一** | 退款完成时累加 |

### 2.2 新增实体

```text
Order
  ├─ order_allocation_snapshots   1:1  冻结快照（append-only, hash）
  ├─ order_allocation_decisions   1:N  拆量决策审计（INV-28）
  ├─ order_shortage_cases         1:N  支付后缺货二选一（E8/AC-30）
  └─ order_reconciliation_diffs   1:N  等式 1/3 的日终差异

Pricing
  ├─ quote_lines（扩展 line_paid_cent / unit_paid_cent / split_seq）
  └─ quote_groups（扩展 freight_share_cent）

Fulfillment
  ├─ carriers                      受控承运商清单
  ├─ shipment_track_events         追加型轨迹事件
  ├─ delivery_evidence             签收证据（CARRIER_DELIVERED | USER_CONFIRM）
  ├─ shipment_receipts             收货事实（USER | AUTO_T1 | AUTO_T7 | DISPUTE_RESOLVED）
  ├─ delivery_auto_confirm_jobs    自动确认调度（durable queue）
  ├─ delivery_disputes             签收争议 + 裁决留痕
  └─ shipment_operation_cases      异常轨迹/判责入口（E18）

AfterSales
  ├─ after_sale_line_ledger        行退款台账（S14 唯一事实源）
  └─ after_sale_refund_breakdowns  商品/运费退款明细（对账用）

Payment
  └─ refund breakdown 字段/表      商品退款与运费退款分列（对账用）
```

### 2.3 行（line）的规范定义

一条行 = `(order_no, line_no, sku_id, warehouse_id, fulfillment_group_no, quantity, price_cent, discount_cent, line_paid_cent, unit_paid_cent)`。

- **行不设独立状态机**（Spec 4.2 铁律 5）：行的取消/退款由组与售后表达。
- 行号在冻结前一次性分配，分配规则见 §5.2；冻结后不可变。
- 同一 SKU 最多 2 行（INV-19）；整单最多 3 组。

---

## 3. 状态机

### 3.1 订单生命周期（不变）

```text
PENDING_PAYMENT --支付确认--> PAID --首组发货--> ALLOCATED --全部发货--> SHIPPED --全部收货--> COMPLETED
       │                        │
       └──取消/超时/补偿──> CANCELLED <──取消（仅未支付/未发货）──┘
```

规则：

- 只有 6 个值；**禁止**写入 `REFUNDING / PARTIALLY_REFUNDED / REFUNDED / CLOSED`（AC-25）。
- “全部退款”只是 `S1 + S4` 的展示结论。
- 已支付未发货不提供“取消”语义（Spec 6.7）。

### 3.2 履约组

```text
PENDING --支付--> ALLOCATED --发货--> SHIPPED --收货确认--> RECEIVED
   │                  │
   └──取消/退款撤许可──┴──> CANCELLED（读模型；持久化可为 ALLOCATED+REVOKED）
```

- 组 `SHIPPED` 只能由 Fulfillment 的 `shipment.shipped.v1` 驱动；组 `RECEIVED` 只能由 `shipment.received.v1` 驱动。
- 退款撤许可后未发货组：持久化 `ALLOCATED`，许可 `REVOKED`，读模型 `CANCELLED`（沿用 [settlement.go:80-88](../../internal/services/order/settlement.go#L80-L88)）。
- 已退款未发组不得阻塞其他组推进（AC-27）。
- 新增动作仍由 `orderActionPolicy` 单一推导：`confirm_receipt`、`open_receipt_dispute`、`choose_shortage_resolution`；前端不得按字段自行判断。

### 3.2.1 订单履约投影（S2）

`orders.shipment_status` 只允许 5 个值（Spec S2）：

```text
UNSHIPPED | PARTIALLY_SHIPPED | SHIPPED | PARTIALLY_RECEIVED | RECEIVED
```

映射（由组状态聚合，不落组）：

| 组状态 | 履约投影 | 生命周期 |
|---|---|---|
| 全部未发 | UNSHIPPED | PAID/ALLOCATED |
| 部分发货 | PARTIALLY_SHIPPED | ALLOCATED |
| 全部发货 | SHIPPED | SHIPPED |
| 部分收货 | PARTIALLY_RECEIVED | SHIPPED |
| 全部收货 | RECEIVED | COMPLETED |
| 全部组关闭（退款/取消） | UNSHIPPED | 不变 |

**迁移要求**：现有 `shipmentStatus()` 返回的 `CREATED` 必须改为 `UNSHIPPED`；`CREATED` 只保留为运单状态，不得再作为订单履约投影（避免 S2/S6 混淆）。

### 3.3 组退款态（正交）

```text
NONE --BlockShipmentForRefund / 售后进入退款--> REFUNDING --aftersale.completed--> REFUNDED
```

不得写入 `orders.status`。`REFUNDED` 是终态，不回退。

### 3.3.1 发货许可（S5）

```text
NONE --AuthorizeShipment--> CONSUMED
  │
  └─BlockShipmentForRefund--> REVOKED
       （已 CONSUMED 时退款不降级，允许“发货后退款”）
```

- 只允许 `NONE / CONSUMED / REVOKED` 三个值；`GRANTED` 自本 ADR 起**废弃**，读模型不得暴露。
- 主键 `(order_no, fulfillment_group_no)`；同一行的发货与退款请求串行化，唯一胜者。
- Fulfillment 发货前必须调用 `AuthorizeShipment`；**不得**自行判断“可发”。

### 3.4 运单状态（前向）

```text
CREATED --Ship(需许可)--> SHIPPED --收货确认--> RECEIVED
   │
   └──组退款作废--> CANCELLED
```

### 3.5 轨迹状态（单调）+ 异常（正交）

```text
CREATED(0) -> SHIPPED(1) -> IN_TRANSIT(2) -> OUT_FOR_DELIVERY(3) -> DELIVERED(4)

exception_state: NONE -> OPEN -> RESOLVED
exception_type:  REJECTED | RETURNED | LOST | DAMAGED
```

- 轨迹只接受 rank 更大的事件；`DELIVERED` 后拒绝任何非争议轨迹回退（INV-25）。
- 异常事件**不改变** `track_status`，只置 `exception_state=OPEN` 并告警；不自动退款（E18/AC-24）。

### 3.6 签收 / 收货

```text
delivery:  NONE --CARRIER_DELIVERED | USER_CONFIRM--> DELIVERED
receipt:   PENDING --USER | AUTO_T1 | AUTO_T7 | DISPUTE_RESOLVED--> CONFIRMED
```

- `delivery` 与 `receipt` 分离：`AUTO_T7` 只在**无任何轨迹**时产生 receipt，不产生 delivery 证据；页面只能显示“已自动确认收货”，不得显示“已签收”。
- `AUTO_T1` 只对已有 `DELIVERED` 证据的运单生效。

### 3.7 争议

```text
NONE --用户申诉(24h 内)--> OPEN --客服裁决--> RESOLVED_UPHELD（用户胜，撤销收货）
                                      └────> RESOLVED_REJECTED（用户败，确认收货）
```

- `OPEN` 冻结自动确认；客服只能裁决，不能直接改签收/收货（E20/AC-23）。
- 裁决必须写 `resolution_reason`、`operator`、`resolved_at`（AC-22）。

### 3.8 售后（沿用 + 行台账）

```text
REFUND_ONLY:   REQUESTED -> APPROVED -> REFUNDING -> REFUNDED
                              └-> REJECTED
RETURN_REFUND: REQUESTED -> APPROVED -> RETURNING -> RETURN_RECEIVED -> REFUNDING -> REFUNDED
                                                       └ restock_state: PENDING -> PROCESSING -> DONE
```

退款完成时：`after_sale_line_ledger` 累加行已退数量/金额（同一事务），并写 `shop.aftersale.completed.v1`。

### 3.9 缺货处置（E8/AC-30）

```text
OPEN --4h 内通知--> NOTIFIED --用户选择--> RESOLVED_WAIT（等待补货，带日期）
                                   └────> RESOLVED_REFUND（按组退款）
NOTIFIED --48h 未选择--> RESOLVED_REFUND（默认，fail-safe）
```

- 禁止换仓/加组/降级拆量（INV-26）。
- 退款走组退款协议，不写生命周期。
- “通知”在本期指：订单详情显著提示 + `shop.order.shortage.reported.v1` 事件；不做短信/邮件（D-07）。

### 3.10 自动确认任务

```text
PENDING --到期且校验通过--> DONE
PENDING --用户确认/争议/作废--> CANCELLED
```

---

## 4. 不变量与执行点

### 4.1 基线不变量

INV-01…INV-09、INV-11…INV-16 继续有效；**INV-10 废止**（单行数量可跨仓，但必须表现为多行）。

### 4.2 新增不变量（INV-17…INV-28）

| ID | 执行点（唯一） | 验证 |
|---|---|---|
| INV-17 数量守恒 | Pricing 生成行时校验 `Σ line.quantity = requested`；Order 建单再校验一次 | 单测 + 集成 |
| INV-18 单行单仓 | `order_items` 一行一个 `warehouse_id`/`group_no`；DB 约束 + Order 校验 | schema + 集成 |
| INV-19 拆量上限 | Inventory 分配器硬上限（每 SKU 2 行、整单 3 组）；Pricing/Order 二次校验 | 单测 + AC-07 |
| INV-20 运费只收一次 | Pricing 只写订单级 `Quote.freight_cent`；组 `freight_share_cent` 之和必须等于它 | 单测 + 对账等式 3 |
| INV-21 金额守恒 | Pricing 生成后校验 `Σ line_paid + freight = pay`；Order 建单复核 | 单测 + 对账等式 1 |
| INV-22 支付后冻结 | `orders.allocation_frozen_at` + 写守卫 + 快照表 append-only | 集成 + AC-09 |
| INV-23 退款基准唯一 | AfterSales 计算；`amount_cent` 仅记录为期望 | AC-11 |
| INV-24 双层额度 | AfterSales 行台账 + Payment captured 双校验 | AC-12 |
| INV-25 签收单调 | 轨迹 rank 比较 + `DELIVERED` 终态守卫 | AC-19 |
| INV-26 缺货不静默 | `order_shortage_cases` 状态机 + 禁止换仓写路径 | AC-30 |
| INV-27 售后单单组 | AfterSales `groupForAfterSale` 强校验 | AC-24 |
| INV-28 可追溯 | `order_allocation_decisions` + `order_status_logs` + 争议留痕 | 审计查询 |

---

## 5. 拆量与冻结协议

### 5.1 预览流水线

```text
BFF POST /checkout/preview
  -> Inventory.AllocateOrder(lines[])            只读，返回 split allocations（含 quantity）
  -> Pricing.CreateQuote(items, allocations)     唯一计算行金额/优惠/运费
  -> BFF 返回 Quote + package_count + warehouse_count + "将分 N 个包裹"
```

- 分配是**报价事实**，不写预留（Spec 3/6.6）。
- `AllocateOrder` 增加 `quantity` 与 `split_seq` 字段；`QuoteAllocation` 同步。
- 预览必须 2 秒内返回或明确失败（Spec §10）。分配器预算：候选仓 ≤8/SKU、方案枚举 ≤36/SKU、DFS 节点 ≤20000、总计算 ≤1.5s；超预算返回 `UNAVAILABLE` 并提示重试，**禁止**猜一个方案返回。

### 5.2 分配器（确定性）

输入：按 SKU 合并后的 `(sku_id, quantity)`；每个 SKU 的候选仓 `available = on_hand - reserved`。

**第 1 步：候选裁剪**

- 每个 SKU 只保留 `available > 0` 的仓，按 `(business_priority, -available, warehouse_id)` 排序，最多 8 个。
- 若某 SKU 无候选，返回 `FAILED_PRECONDITION`，附 `max_available=0`。

**第 2 步：单 SKU 方案枚举**

- 单仓方案：`available >= q`。
- 双仓方案：`w1 != w2`、`avail1 + avail2 >= q`；分配量为：先按优先级把 `w1` 填到 `min(avail1, q)`，余量给 `w2`（保证“集中度最高”）。
- 最多 2 仓；不生成 3 仓方案。
- **禁止**：按运费最低选仓、客服手工指定选仓、跨仓调拨补货（Spec 6.2）。

**第 3 步：全局选优（整单 ≤3 组）**

- 目标函数（字典序）：
  1. 不同仓数量最少；
  2. 发生拆量的 SKU 数最少；
  3. 集中度最高：同等仓数下，优先填满优先级更高的仓（**填满仓数多者胜**；仍相同则最高优先级仓的分配量大者胜）；
  4. 仓库业务优先级向量最优；
  5. 可用量降序；
  6. 仓库 `warehouse_id` 升序（稳定）。
- 用有界 DFS + 记忆化（键 = 已用仓集合 + SKU 下标）；已用仓超过 3 立即剪枝。
- 无解：返回 `FAILED_PRECONDITION`，附 `max_orderable_quantity`、`min_packages_needed`、`suggestion=reduce_quantity|split_orders`。

**第 4 步：行号/组号生成（冻结前一次）**

- 输出行按 `(原始 item 顺序, split_seq)` 排序，`line_no = 1..N`。
- 组号按仓首次出现顺序 `G1..Gn`；组内仓 = 该行 `warehouse_id`。
- 同一请求重复预览，结果必须逐字节一致（INV-28 可追溯）。

### 5.3 下单校验

`Order.CreateOrder`：

1. 载入 quote；校验未过期、`quote.user_id` 匹配、行/组完整。
2. 校验 INV-17/18/19/20/21（行数量、单行单仓、上限、金额）。
3. 校验 quote 的 `allocation_hash`：由 Pricing 在生成 quote 时按规范序列化（行号、SKU、仓库、组号、数量、金额）计算并写入；Order 用同法复算，不一致即拒绝整单。
4. 提交 Saga：每行（按 `(sku, warehouse)` 合并）预留 → `CreateOrderInner` 写入行/组/快照。
5. 任一预留失败：Saga 回滚；返回 `FAILED_PRECONDITION` + `retry_preview=true`；前端重新预览（E4/E5）。

### 5.4 快照内容

`order_allocation_snapshots`（append-only）：

```text
order_no            PK
allocation_hash     CHAR(64)   -- 行/组规范序列化的 SHA-256
lines_json          JSON       -- 冻结行快照
groups_json         JSON       -- 冻结组快照
frozen_at           DATETIME(3)
created_at
```

同时 `orders` 冗余 `allocation_hash`、`package_count`、`warehouse_count`、`allocation_frozen_at`，供快速读取与守卫。

快照表的不变性：`lines_json / groups_json / allocation_hash / created_at` 内容列**只插入、永不更新**；唯一允许的更新是 `frozen_at` 从 `NULL` 置为时间一次。冻结后任何内容列更新都必须被 repo 层拒绝。

### 5.5 冻结点（Q1）

- 在 `TccConfirmMarkPaid` / `MarkPaid` 的事务里：`UPDATE orders SET status='PAID', allocation_frozen_at=NOW(3) WHERE order_no=? AND status='PENDING_PAYMENT' AND allocation_frozen_at IS NULL`。
- 同一事务写 `order_allocation_snapshots`（若尚未写）与 `shop.order.allocation_frozen.v1`。
- 冻结后允许的写：`status`（发货/收货投影）、`refund_state`、组 `status`、许可状态、缺货案状态；**禁止**改行数量、行仓、组仓、行金额、组运费分摊。

### 5.6 冻结后的拒绝方式

所有可能改分配的服务方法必须带守卫：

```sql
-- 示意：行/组分配字段的写入一律附带
UPDATE order_items SET ... WHERE order_no = ?
  AND EXISTS (SELECT 1 FROM orders o WHERE o.order_no = ? AND o.allocation_frozen_at IS NULL)
```

并增加集成测试：对已支付订单直接调用内部写路径必须得到 `FAILED_PRECONDITION`。**禁止**用“调用方自觉不调”替代 SQL 守卫。

### 5.7 拆量决策审计（INV-28）

`order_allocation_decisions` 每次预览/下单各写一条：

```text
id, order_no/quote_id, request_json, availability_json,
chosen_json, rule_hits_json (命中 6.2 的规则编号), created_at
```

客服/运营查询“为什么分两个包裹”时读这张表，不重新跑算法。

---

## 6. 金额协议（Q2）

### 6.1 计算顺序（唯一）

```text
1. goods_total   = Σ (price_cent × quantity)                 -- 按行
2. order_discount= 券/满减总额（Pricing 计算）
3. line_discount = 最大余数法按行商品金额分摊 order_discount
4. line_paid     = price×qty − line_discount
5. freight       = freightFor(Σ line_paid)                   -- 订单级一次
6. pay           = Σ line_paid + freight
7. group_share   = 最大余数法按组商品金额分摊 freight         -- 内部分摊，不上用户账
```

运费优惠（免邮券/包邮活动）先作用于订单级 `freight`，再按组商品金额分摊；用户侧只看到一行运费（Spec 6.4.3）。

约束：

- `discount_cent` 是券 + 促销的合计；`promotion_discount_cent` 只是其中促销展示，**不得二次扣减**。
- `order_discount <= goods_total`；超出由 Pricing 截断并告警。
- `Σ line_discount = order_discount`，最后一行吸收舍入差额（Spec 6.4.4）。
- `Σ line_paid + freight = pay`（INV-21）。
- `Σ group_share = freight`（INV-20）。
- 包邮门槛按 `Σ line_paid` 判定，不按组（AC-05）。
- 组运费只写 `QuoteGroup.freight_share_cent` / `OrderFulfillmentGroup.freight_share_cent`，用户应付只读订单级 `freight_cent`。

### 6.2 共享算法

`internal/platform/money` 提供纯函数：

- `AllocateProportional(total int64, weights []int64) []int64`（最大余数法，确定性，末项吸收差额）。
- 必须单测覆盖：零权重、负数、总和守恒、大数不溢出、顺序稳定。
- Pricing 与 AfterSales 只允许调用它，**禁止**各自写一份取整逻辑。

### 6.3 行实付单价与末次补差

```text
unit_paid = floor(line_paid / quantity)
remainder = line_paid - unit_paid × quantity
本次退款额 = unit_paid × 本次数量
若 (已退数量 + 本次数量) == quantity: 本次退款额 += remainder
```

不得四舍五入；不得把 `remainder` 提前摊进单价。

### 6.4 退款计算（AfterSales 唯一）

输入：`order_no, line_no, quantity`（RETURN_REFUND 用已收货数量）。

```text
line        = order_items 快照（冻结后不可变）
ledger      = after_sale_line_ledger(order_no, line_no) FOR UPDATE
remaining_q = line.quantity - ledger.refunded_quantity
remaining_a = line.line_paid_cent - ledger.refunded_amount_cent

拒绝: quantity <= 0 | quantity > remaining_q
金额: unit = floor(line.line_paid_cent / line.quantity)
      amount = unit * quantity
      if ledger.refunded_quantity + quantity == line.quantity:
          amount += line.line_paid_cent - unit * line.quantity
amount = min(amount, remaining_a)
amount = min(amount, payment captured remaining)   -- 第二层
```

- 两层都通过才写 `REFUNDING`；任一层失败返回明确错误（E12/E15/AC-12）。
- `amount_cent` 入参只存 `expected_amount_cent`，不参与计算；返回 `computed_amount_cent` 与明细（AC-11）。
- 退款执行成功后，同一本地事务更新 `after_sale_line_ledger`（`refunded_quantity += q`，`refunded_amount_cent += amount`），并写 `shop.aftersale.line_refunded.v1`。

### 6.5 运费退还

- 运费退款是**独立明细**，不混入行金额：`after_sale_refund_breakdowns(goods_cent, freight_cent, total_cent, rule_id)`。
- 判定按 Spec 6.5.1；`rule_id` 必须落库，便于对账与客服解释。
- Payment 只接受 `goods_cent + freight_cent` 的总额，且总额 ≤ captured 剩余。

### 6.6 客服改价

- 不存在支付后改价 API。
- 审批只能按**数量**调整（重新计算金额），不能直接写金额；直接金额调整返回 `INVALID_ARGUMENT`。
- 冻结后任何金额字段写入被 SQL 守卫拒绝。

### 6.7 优惠额度按退款比例释放（Spec 6.7）

- 退款**不得回滚**已消耗的优惠额度；只按实际退款比例释放，释放值 ≤ 已占用值。
- 释放由 Pricing 在消费 `shop.aftersale.completed.v1` 时执行，幂等键 = `after_sale_no`（同一售后重复投递只释放一次）。
- 计算：`release = floor(occupied × refunded_amount / captured)`；累计释放不得超过 `occupied`；订单最后一次退款释放剩余值。
- 释放的是**促销/券的活动额度**，不是用户看到的退款金额；不得把释放额加进退款。
- 禁止用 Order 的异步 `refunded_cent` 作为释放依据（脏读投影）；以 AfterSales 的本次退款事实为准。

---

## 7. 承运商、轨迹与签收（Q3/Q4）

### 7.1 承运商受控清单

`fulfillment.carriers`：

```text
code VARCHAR(32) PK, name VARCHAR(64), enabled BOOL, sort_order INT,
webhook_secret_ref VARCHAR(128),   -- K8s Secret 的键名，禁止存明文
created_at, updated_at, version
```

- `CreateShipment` / `Ship` 必须传 `carrier_code` 且 `enabled=1`；未登记或停用 → `FAILED_PRECONDITION` + 可选清单（AC-17）。
- 退货承运商复用同一清单；`SubmitReturnTracking` 必须校验 `enabled=1`（Spec 6.8）。
- `carrier` 名称作为展示冗余，不允许作为准入依据。
- 管理端维护；变更留痕（`carrier_audit_logs`）。webhook 签名密钥从 K8s Secret 读取，不落库。

### 7.2 运单号唯一

`shipments` 增加生成列：

```sql
tracking_key VARCHAR(192) GENERATED ALWAYS AS (
  IF(tracking_no = '', NULL, CONCAT(carrier_code, ':', tracking_no))
) STORED,
UNIQUE KEY uk_shipments_tracking (tracking_key)
```

- 空运单号不冲突；同一承运商下重复运单号直接阻断发货并告警（E19/AC-18）。
- 换承运商/运单号只允许在 `CREATED` 或 `CANCELLED` 运单上，写 `shipment_change_logs`。

### 7.3 轨迹事件与单调投影

`shipment_track_events`：

```text
id, shipment_no, carrier_code, tracking_no, event_id,
status, exception_type, occurred_at, received_at, raw_json,
UNIQUE (carrier_code, tracking_no, event_id)
```

- 入口：`Fulfillment.RecordTrackingEvent`，由承运商 webhook 适配器调用；签名按承运商密钥校验。
- 写入事务内：`INSERT IGNORE` 事件（幂等）→ 比较 rank → `UPDATE shipments SET track_status=? WHERE rank < ?` → 若 `DELIVERED` 写 `delivery_evidence` + upsert 自动确认任务 → outbox 事件。
- `DELIVERED` 是终态；后续非争议事件一律丢弃并计数（AC-19）。
- 异常事件只写 `exception_type`/`exception_state=OPEN`，不改 `track_status`（E18/AC-24）。

### 7.4 签收证据与争议（Q3）

`delivery_evidence`：

```text
id, shipment_no, source ENUM('CARRIER_DELIVERED','USER_CONFIRM'),
evidence_ref (event_id 或 user_id), occurred_at, created_at,
UNIQUE (shipment_no, source, evidence_ref)
```

- 只有这两种 source 能写。客服没有写入口。
- 用户确认：`ConfirmReceipt(shipment_no, user_id)`；必须校验订单归属；写 `USER_CONFIRM` 证据 + `shipment_receipts`。存在 `OPEN` 争议时返回 `FAILED_PRECONDITION`（先等裁决，避免自相矛盾的证据）。
- 争议：`delivery_disputes`，只允许用户 24h 内发起；`OPEN` 冻结自动确认；客服裁决写 `RESOLVED_UPHELD/REJECTED` + 依据 + 操作人（AC-22/23）。若 `shipment_receipts` 已确认，争议入口关闭并返回 `receipt_already_confirmed`；是否支持回滚见 OQ-08。
- 争议裁决**不是**签收来源；它只决定 receipt 结果。

### 7.5 自动确认调度（Q4）

`delivery_auto_confirm_jobs`：

```text
shipment_no PK, reason ENUM('T1_DELIVERED','T7_NO_TRACK'),
due_at, state ENUM('PENDING','DONE','CANCELLED'),
lease_owner, lease_until, attempts, last_error, created_at, updated_at
```

- 运单 `SHIPPED`：`Ship` 同事务写 `shipped_at`，并建 `T7_NO_TRACK`，`due_at = shipped_at + 7d`。
- `DELIVERED`：upsert `T1_DELIVERED`，`due_at = delivered_at + 1d`。
- 用户确认/争议/运单作废：`CANCELLED`。
- Worker（Fulfillment）：`FOR UPDATE SKIP LOCKED` 领取到期任务，租约 1 分钟；执行前重读运单与争议：
  - `T1`：`delivered_at` 存在、无 OPEN 争议、无 receipt → 写 `shipment_receipts(source=AUTO_T1)`。
  - `T7`：`track_status` 仍为 `CREATED/SHIPPED` 且无任何 `DELIVERED`、无 OPEN 争议、无 receipt → 写 `shipment_receipts(source=AUTO_T7)`。
  - `T7` 到期但已有轨迹（`IN_TRANSIT/OUT_FOR_DELIVERY`）且未 `DELIVERED`：任务置 `CANCELLED`，**不得**自动确认；等待 `DELIVERED` 后按 T1 处理。
- 重复确认防护：`shipment_receipts.shipment_no` 主键；worker 幂等；事件消费端按 `(shipment_no, source)` 去重。
- Order 只消费 `shop.shipment.received.v1` 推进组 `RECEIVED`。

### 7.6 展示口径

- 有 `DELIVERED` 证据：展示“已签收”；有 OPEN 争议：展示“争议中”。
- 无 `DELIVERED` 但 `AUTO_T7` 已确认：展示“已自动确认收货（无物流轨迹）”，**不得**展示“已签收”。
- 异常：展示“异常待处理”，不得展示“已签收/已退款”。
- 自动确认前：BFF 必须返回 `auto_confirm_at`，详情页至少展示一次“将于 X 自动确认收货”；`AUTO_T7` 不得展示为“已签收”。

---

## 8. API 契约

### 8.1 proto 变更（只增不改，字段号取当前最大值 +1）

| 消息 | 新字段 | 说明 |
|---|---|---|
| `QuoteAllocation` | `int32 quantity = 5; int32 split_seq = 6;` | 拆量分配数量与序号 |
| `QuoteLine` | `int64 line_paid_cent = 9; int64 unit_paid_cent = 10; int32 split_seq = 11;` | 行实付/单价 |
| `Quote` | `int32 package_count = 14; int32 warehouse_count = 15; string allocation_hash = 16;` | 付款前知情 |
| `QuoteGroup` | `int64 freight_share_cent = 6;` | 内部分摊 |
| `OrderItem` | `int32 line_no = 8; int64 line_paid_cent = 9; int64 unit_paid_cent = 10;` | 行快照；`refunded_*` 不落 Order |
| `OrderFulfillmentGroup` | `int64 freight_share_cent = 6;` | 仅 Order 拥有的内部分摊；物流字段**不得**放在这里 |
| `Order` | `int32 package_count = 25; int32 warehouse_count = 26; int64 allocation_frozen_at_unix = 27;` | 展示/审计 |
| `Shipment` | `string carrier_code = 10; string track_status = 11; int64 shipped_at_unix = 12; int64 delivered_at_unix = 13; string delivery_source = 14; string exception_state = 15; string receipt_state = 16;` | 物流事实（Fulfillment 独有） |
| `AfterSaleItem` | `int32 line_no = 5; int64 unit_paid_cent = 6; int64 line_paid_cent = 7;` | 按行售后 |
| `CreateAfterSaleRequest` | `int64 amount_cent = 5 [deprecated = true]; int64 expected_amount_cent = 7;` | 期望值只记录 |
| `RefundOrderRequest` | `int64 goods_cent = 7; int64 freight_cent = 8; string rule_id = 9;` | 商品/运费退款分列对账 |

所有权约束：订单详情页的 `carrier_code / tracking_no / track_status / delivery_state / receipt_state` 由 BFF 组合 `Order.GetOrder` + `Fulfillment.ListShipments(order_no)` 得到，**不得**写入 Order 聚合（S6/S8 属于 Fulfillment）。行已退数量/金额同理，由 BFF 调 `AfterSales.ListLineRefunds` 组合，**不得**在 Order 落第二份事实（S14）。

### 8.2 新增 RPC

| 服务 | RPC | 用途 |
|---|---|---|
| Inventory | `AllocateOrder`（扩展 reply） | 拆量只读分配 |
| Fulfillment | `RecordTrackingEvent` | 承运商轨迹回传 |
| Fulfillment | `ConfirmReceipt` | 用户主动确认收货 |
| Fulfillment | `OpenReceiptDispute` / `ResolveReceiptDispute` | 争议 |
| Fulfillment | `ListTrackingEvents` | 详情页轨迹 |
| Fulfillment | `CreateCarrier` / `UpdateCarrier` / `SetCarrierStatus` / `ListCarriers` | 受控清单 |
| Order | `ReportGroupShortage` / `ChooseShortageResolution` / `GetShortageCase` | 支付后缺货二选一 |
| AfterSales | `PreviewRefundQuote` | 只读计算可退数量/金额 |
| AfterSales | `ListLineRefunds` | BFF 组合订单详情（S14 只读） |
| AfterSales | `AuthorizeCrossWarehouseRestock` | 跨仓回库人工例外（Admin 角色 + 原因 + 留痕） |

### 8.3 BFF 路由

C 端：

- `POST /api/v1/checkout/preview` 返回 `package_count/warehouse_count`
- `POST /api/v1/shipments/:shipmentNo/confirm-receipt`
- `POST /api/v1/shipments/:shipmentNo/dispute`
- `GET /api/v1/after-sales/refund-preview?order_no=&line_no=&quantity=`
- `POST /api/v1/orders/:orderNo/shortage/:groupNo/choice`

Admin：

- `GET/POST/PATCH /api/admin/v1/carriers`
- `POST /api/admin/v1/disputes/:id/resolve`
- `POST /api/admin/v1/orders/:orderNo/shortage`
- `GET /api/admin/v1/reconciliation/:date`

Webhook（仅集群内，签名校验，不经用户 JWT）：

- `POST /api/v1/webhooks/carriers/:code`

### 8.4 错误语义

| 场景 | 错误 | 附加信息 |
|---|---|---|
| 总量不足 | `FAILED_PRECONDITION` | `max_orderable_quantity` |
| 需 >2 仓 / >3 组 | `FAILED_PRECONDITION` | `min_packages_needed`、`suggestion` |
| 预览后库存变化 | `FAILED_PRECONDITION` | `retry_preview=true` |
| 已冻结仍改分配 | `FAILED_PRECONDITION` | `allocation_frozen` |
| 承运商未登记/停用 | `FAILED_PRECONDITION` | `available_carriers` |
| 运单号重复 | `CONFLICT` | `carrier_code`、`tracking_no` |
| 轨迹回退 | 幂等丢弃 + 指标 | 不报错 |
| 收货重复确认 | 幂等成功 | `already_confirmed` |
| 争议窗口已过 | `FAILED_PRECONDITION` | `dispute_deadline` |
| 售后超额/超量 | `INVALID_ARGUMENT` | `remaining_quantity`、`remaining_amount_cent` |

`apperr` 扩展 `Details map[string]any` 并映射到 gRPC `ErrorInfo` metadata；HTTP 原样返回 `details`。

### 8.5 事件（必须进 typed registry，ADR 0004）

| 事件 | 生产者 | 消费者 |
|---|---|---|
| `shop.order.allocation_frozen.v1` | Order | 审计/BI |
| `shop.order.shortage.reported.v1` / `.resolved.v1` | Order | 通知/BI |
| `shop.shipment.tracked.v1` | Fulfillment | Order 投影/BI |
| `shop.shipment.exception.v1` | Fulfillment | 运营告警 |
| `shop.shipment.delivered.v1`（扩展 source） | Fulfillment | Order 展示 |
| `shop.shipment.received.v1` | Fulfillment | Order 组 `RECEIVED` |
| `shop.shipment.dispute.opened.v1` / `.resolved.v1` | Fulfillment | Order/运营 |
| `shop.aftersale.line_refunded.v1` | AfterSales | BI/对账 |
| `shop.aftersale.completed.v1`（既有，扩展明细） | AfterSales | Order 组退款态、Pricing 优惠额度释放、Fulfillment 作废运单 |

要求：payload 为显式 struct（禁止 `map[string]any`）；未知事件类型在 `outbox.Append` fail-closed。

---

## 9. 持久化与迁移

> 以下为 DDL **轮廓**：表名、列语义、唯一键与约束是契约；具体列类型/字符集/索引名由 Developer 按仓库迁移规范展开，不得改变语义。迁移编号取各服务当前最大号 +1。

### 9.1 Order（`migrations/order/0014_quantity_split.sql`）

```sql
ALTER TABLE orders
  ADD COLUMN allocation_frozen_at DATETIME(3) NULL,
  ADD COLUMN allocation_hash CHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN package_count INT NOT NULL DEFAULT 1,
  ADD COLUMN warehouse_count INT NOT NULL DEFAULT 1;

ALTER TABLE order_items
  ADD COLUMN line_paid_cent BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN unit_paid_cent BIGINT NOT NULL DEFAULT 0;

ALTER TABLE order_fulfillment_groups
  ADD COLUMN freight_share_cent BIGINT NOT NULL DEFAULT 0;

CREATE TABLE order_allocation_snapshots (...);
CREATE TABLE order_allocation_decisions (...);
CREATE TABLE order_shortage_cases (...);
CREATE TABLE order_reconciliation_diffs (...);
```

回填：`line_paid_cent = price_cent*quantity - discount_cent`；`freight_share_cent = freight_cent`；`package_count/warehouse_count` 按现有组数；`allocation_frozen_at` 对已支付订单回填 `updated_at`（保守：视为已冻结）。

### 9.2 Pricing（`migrations/pricing/0011_split_money.sql`）

```sql
ALTER TABLE quote_lines
  ADD COLUMN line_paid_cent BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN unit_paid_cent BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN split_seq INT NOT NULL DEFAULT 0;
ALTER TABLE quote_groups ADD COLUMN freight_share_cent BIGINT NOT NULL DEFAULT 0;
```

### 9.3 Fulfillment（`migrations/fulfillment/0004_carrier_delivery.sql`）

```sql
ALTER TABLE shipments
  ADD COLUMN carrier_code VARCHAR(32) NOT NULL DEFAULT '',
  ADD COLUMN track_status VARCHAR(24) NOT NULL DEFAULT 'CREATED',
  ADD COLUMN shipped_at DATETIME(3) NULL,
  ADD COLUMN delivered_at DATETIME(3) NULL,
  ADD COLUMN delivery_source VARCHAR(24) NOT NULL DEFAULT '',
  ADD COLUMN exception_state VARCHAR(16) NOT NULL DEFAULT 'NONE',
  ADD COLUMN tracking_key VARCHAR(192) GENERATED ALWAYS AS (
    IF(tracking_no = '', NULL, CONCAT(carrier_code, ':', tracking_no))
  ) STORED,
  ADD UNIQUE KEY uk_shipments_tracking (tracking_key);

CREATE TABLE carriers (code PK, name, enabled, sort_order, webhook_secret_ref, version, created_at, updated_at);
CREATE TABLE shipment_track_events (... UNIQUE (carrier_code, tracking_no, event_id));
CREATE TABLE delivery_evidence (id PK, shipment_no, source, evidence_ref, occurred_at,
                                UNIQUE (shipment_no, source, evidence_ref));
CREATE TABLE shipment_receipts (shipment_no PK, source, evidence_ref, confirmed_at, created_at);
CREATE TABLE delivery_auto_confirm_jobs (shipment_no PK, reason, due_at, state,
                                         lease_owner, lease_until, attempts, last_error,
                                         version, created_at, updated_at);
CREATE TABLE delivery_disputes (id PK, shipment_no, order_no, user_id, status, reason,
                                resolution_reason, resolved_by, deadline_at, opened_at, resolved_at, version);
CREATE TABLE shipment_operation_cases (id PK, shipment_no, exception_type, status,
                                       responsibility, resolved_by, resolution,
                                       version, created_at, resolved_at);
CREATE TABLE shipment_change_logs (id PK, shipment_no, field, old_value, new_value, reason, operator, created_at);
CREATE TABLE carrier_audit_logs (id PK, carrier_code, action, operator, reason, created_at);
```

`shipment_receipts.shipment_no` 主键是“只确认一次”的最终防线；任何 source 的重复确认都是幂等 no-op。

### 9.4 AfterSales（`migrations/aftersales/0012_line_ledger.sql`）

```sql
ALTER TABLE after_sale_items
  ADD COLUMN line_no INT NOT NULL DEFAULT 0,
  ADD COLUMN line_paid_cent_snapshot BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN unit_paid_cent_snapshot BIGINT NOT NULL DEFAULT 0,
  DROP INDEX uk_after_sale_items_group,
  ADD UNIQUE KEY uk_after_sale_items_line (after_sale_id, line_no),
  ADD KEY idx_after_sale_items_group (after_sale_id, sku_id, warehouse_id, group_no);
CREATE TABLE after_sale_line_ledger (
  order_no, line_no, refunded_quantity, refunded_amount_cent,
  version, created_at, updated_at, UNIQUE (order_no, line_no)
);
CREATE TABLE after_sale_refund_breakdowns (...);
CREATE TABLE restock_exception_authorizations (
  id PK, after_sale_no, sku_id, from_warehouse_id, to_warehouse_id,
  operator, reason, created_at, UNIQUE (after_sale_no, sku_id)
);
```

`Inventory.Restock` 在 `warehouse_id != 行原仓` 时必须存在匹配的 `restock_exception_authorizations`，否则拒绝（E14/AC-16）。授权动作只允许 Admin 角色，必须写 operator/reason/time。

回填：单组历史售后按 `(after_sale_no, sku, warehouse, group)` 归并到 `line_no`；无法唯一归属的历史数据标 `line_no=0` 并进入人工核对队列，不阻塞新单。

### 9.5 Payment（`migrations/payment/0010_refund_breakdown.sql`）

`refunds` 增加 `goods_cent`、`freight_cent`、`rule_id`；回填 `goods_cent=amount_cent`。新增 `payment_reconciliation_runs` / `payment_reconciliation_diffs`。

### 9.6 通用约束

- 所有**可变**表（`order_shortage_cases`、`delivery_disputes`、`delivery_auto_confirm_jobs`、`after_sale_line_ledger`、`shipment_operation_cases`、`carriers`）必须带 `version BIGINT` 与 `created_at/updated_at`。
- 所有**追加型**表（`order_allocation_decisions`、`shipment_track_events`、`delivery_evidence`、`shipment_receipts`、`shipment_change_logs`）只允许 INSERT，不允许 UPDATE/DELETE。
- 金额列一律 `_cent BIGINT`；状态列一律 `VARCHAR`；时间列一律 `DATETIME(3)`。

### 9.7 Inventory

无 schema 变更。分配器只读；预留键已支持 `(order_no, sku, warehouse)`。

---

## 10. 异常与恢复协议（E1–E25 全覆盖）

### 10.1 通用恢复原语

| 原语 | 机制 |
|---|---|
| 崩溃恢复 | outbox 租约、整单操作逐项进度、售后 durable task、自动确认 job 租约 |
| 部分成功 | Saga 补偿；操作逐项 `DONE/FAILED_PERMANENT`；退款按行台账续跑 |
| 并发冲突 | 条件更新 + `version`；唯一键；`FOR UPDATE`；许可行串行化 |
| 网络脑裂 | 资金/库存 fail-closed；下游超时入 durable 队列；禁止静默放行 |
| 重复投递 | 业务幂等键 + 事件去重 + 状态守卫 |

### 10.2 E1–E25 映射

| # | 协议 | 落点 |
|---|---|---|
| E1 | 分配器返回 `FAILED_PRECONDITION` + `max_orderable_quantity`；不建单、不收款 | Inventory.AllocateOrder |
| E2 | 拆 2 行 2 组，订单级 1 笔运费；预览展示包裹数 | Pricing/Order |
| E3 | 每 SKU >2 仓或整单 >3 组 → 拒绝 + 建议减量/分单 | Allocator |
| E4 | 预留条件更新 `reserved + q <= on_hand`；失败者收到 `retry_preview`；库存不为负 | Inventory.Reserve |
| E5 | quote 过期/哈希不符/预留失败 → 拒绝 + 新预览，禁止静默换仓 | Order.CreateOrder |
| E6 | 回调 `(channel,event_id)` 去重；TCC gid 确定；库存操作 id 确定；只建一组行、只扣一次 | Payment/Inventory |
| E7 | TCC 失败：未捕获则整单取消；已捕获则退款，订单保留生命周期并展示“已关闭（全部退款）”；禁止 `PAID` 无组/半成立 | Payment/Order |
| E8 | `order_shortage_cases`：4h 通知、48h 默认退款；禁止换仓 | Order |
| E9 | 发货后少发：同仓补发或该件退款；禁止跨仓 | AfterSales/Fulfillment |
| E10 | 行台账按 `line_no` 隔离 | AfterSales |
| E11 | 同 SKU 两行独立计价/独立台账 | AfterSales |
| E12 | `quantity > remaining_q` 拒绝，返回剩余 | AfterSales |
| E13 | `restock_state != DONE` 阻断退款 | AfterSales |
| E14 | 回库仓 = 行 `warehouse_id`；跨仓默认拒绝，仅 `restock_exception_authorizations` 授权后可入指定仓（留痕，不自动流转） | Inventory.Restock/AfterSales |
| E15 | 行剩余额度为 0 拒绝 | AfterSales |
| E16 | 轨迹 rank 单调；`DELIVERED` 终态 | Fulfillment |
| E17 | 争议 `OPEN` 冻结自动确认；3 工作日内裁决 | Fulfillment |
| E18 | 异常轨迹进 `shipment_operation_cases`，不自动退款 | Fulfillment |
| E19 | `uk_shipments_tracking` 阻断 + 告警 | Fulfillment |
| E20 | 无客服改签收/收货 API；只能 `ResolveReceiptDispute` | Fulfillment |
| E21 | 未发货整单退款：组全 CANCELLED、退运费、展示已关闭；不写 `status` | Order/AfterSales |
| E22 | 已发组只走售后；未发组退款；订单不整体取消 | Order/AfterSales |
| E23 | 秒杀绕过分配器，单仓，不足即失败 | Seckill/Order |
| E24 | 售后 items 必须同一 `group_no`，混合拒绝 | AfterSales |
| E25 | 不合并发货；预览/详情明示分开发货 | Pricing/UI |

---

## 11. 日终对账（Q5）

### 11.1 等式与所有者

| 等式 | 所有者 | 计算 | 差异表 |
|---|---|---|---|
| ① `Σ line_paid + freight = pay` | Order | 按订单聚合 | `order_reconciliation_diffs` |
| ② `Σ refunded <= captured`（订单/意图） | Payment | 按 intent/order 聚合 | `payment_reconciliation_diffs` |
| ③ `Σ group.freight_share = order.freight` | Order | 按订单聚合 | `order_reconciliation_diffs` |
| ④ `RESOLVED_REFUND` 缺货案 24h 内必须有 `after_sale`（Amendment 1） | Order | 按 case 聚合，联查本地 `refund_after_sale_no` 投影 | `order_reconciliation_diffs` |

### 11.2 调度与暴露

- 各服务每日 `00:10 Asia/Shanghai` 跑一次只读对账 job，写 `*_reconciliation_runs`（日期、总数、差异数）与 `*_reconciliation_diffs`（业务键、期望、实际、差额）。
- 调度：各服务用 `runtime.Workers` 注册进程内日循环，按 `run_date` 幂等（`UNIQUE(service, run_date)`），重跑只 upsert 差异，不重复累加。
- Payment 写 `payment_reconciliation_diffs`；Order 写 `order_reconciliation_diffs`。
- 差异**不改数据**，只暴露：
  - Admin BFF `GET /api/admin/v1/reconciliation/:date` 聚合只读；
  - 指标 `mall_reconciliation_diffs_total{service,equation}`；
  - 差异 >0 告警。
- 修复必须走人工/补偿任务，禁止对账 job 自动改账。

### 11.3 审计

- 拆量决策读 `order_allocation_decisions`。
- 退款对账读 `after_sale_refund_breakdowns` + `refunds`。
- 签收证据读 `delivery_evidence`；争议读 `delivery_disputes`。
- 例外回库读 `restock_exception_authorizations`；承运商变更读 `carrier_audit_logs`。
- **超额度退款在本设计中被禁止**（INV-24 无例外），因此不设授权入口；审计表不为越权预留。未来若要开例外，必须升版 Spec 并同步修改 INV-24，不得由代码私自开口。
- 保留 ≥2 年（Spec §10）。

---

## 12. 反模式自省（强制章节）

### 12.1 本设计如何防止旧反模式借尸还魂

| 旧反模式 | 本设计的结构性防线 | 一旦出现的判定 |
|---|---|---|
| 为状态组合加 `if-else` | 6 个生命周期值 + 4 个正交维度（组、组退款、许可、签收/收货）；动作由 `orderActionPolicy` 单一推导 | 出现“部分完成/部分退款后…”分支即打回 |
| 脏读投影当事实 | 行已退只用 `after_sale_line_ledger`；Order 不落行退款；Payment 是 captured/refunded 唯一源 | Order 新增行已退字段即打回 |
| 状态压缩 | S1 生命周期 / S4 组退款 / S7 签收 / S11 回库分离；禁止 `CLOSED/REFUNDED` 写入 `status` | 新增组合状态即打回 |
| 批次首错返回 | 库存整单操作逐项进度；售后退款/回库 durable task | 新增“整批失败重试”即打回 |
| 调用方填金额 | `expected_amount_cent` 只记录；计算在 AfterSales | 任何 `amount_cent` 参与打款即打回 |
| 手工改签收 | 只有 `delivery_evidence` 两种 source；客服只有争议裁决入口 | 出现 admin 直接置 RECEIVED 即打回 |
| 状态扫描器 | 全部用 durable job 表 + 租约（自动确认、回库、退款） | 新增“扫描所有 REFUNDING”即打回 |
| 假多仓 | 分配器/行/组/仓全链路；支付后冻结 | 只改 `defaultWarehouseID` 常量即打回 |
| 双份金额算法 | `internal/platform/money` 单一实现 | 第二处取整逻辑即打回 |
| “参数可配置”偏离 D-01…D-07 | 配置校验：`max_split_per_sku <= 2`、`max_groups <= 3`、`freight_charged_once=true`；越界 fail-closed | 配置默认值偏离即打回 |

### 12.2 给 Developer 的否决清单（Reviewer 直接引用）

1. 在 `orders.status` 写入任何退款/关闭值 → 打回。
2. 在 Order 侧新增行已退数量/金额持久列 → 打回。
3. 用调用方 `amount_cent` 计算或打款 → 打回。
4. 支付后存在任何改行/组/仓的写路径 → 打回。
5. 承运商仍按自由文本准入 → 打回。
6. 客服可直接改签收/收货 → 打回。
7. 自动确认用 cron 扫业务状态而非 job 表 → 打回。
8. 为轨迹乱序写 `if status == ...` 分支而非 rank → 打回。
9. 新增第二套取整/分摊实现 → 打回。
10. 用 `// TODO` 掩盖 E1–E25 中任一条 → 打回。

---

## 13. 上线、开关与回滚

### 13.1 开关

| 开关 | 默认 | 作用 |
|---|---|---|
| `QUANTITY_SPLIT_ENABLED` | false | 分配器拆量 + 多组订单 |
| `CARRIER_REGISTRY_ENABLED` | false | 受控承运商准入 |
| `DELIVERY_AUTO_CONFIRM_ENABLED` | false | 自动确认 worker |
| `SHORTAGE_CASE_ENABLED` | false | 缺货二选一流程 |

- 配置加载时校验：与 Spec D-01…D-07 冲突的值 fail-closed（ADR 0004 §3）。
- 生产保持关闭，直到 ADR 0003 的生产门禁被显式执行。

### 13.2 阶段

1. **A 金额快照**：schema + Pricing 行金额 + Order 快照（不拆量，单组），对比旧值。
2. **B 分配器影子**：`AllocateOrder` 返回拆量方案，BFF shadow 比对，不下单。
3. **C 拆量写路径**：`QUANTITY_SPLIT_ENABLED=true` 灰度；并发/故障注入。
4. **D 承运商/轨迹/签收**：`CARRIER_REGISTRY_ENABLED`；webhook 适配器。
5. **E 售后行台账 + 退款计算**：历史数据回填 + 双跑对账。
6. **F UI/知情**：预览包裹数、组级物流、争议入口。

每阶段 DoD：`make verify` + `make contract-check` + 目标集成 + 并发/故障注入日志；未跑项必须标 `[未覆盖范围]`。

### 13.3 回滚

- 代码回滚：关闭开关即回到单仓单组路径；schema 只增不删，旧代码可读。
- 数据回滚：不写 down SQL；生产按 ADR 0003 备份恢复。
- 售后历史行台账回填失败：保留 `line_no=0` 人工队列，不阻塞新单；禁止用近似值伪造。

---

## 14. AC 验证映射

| AC | 验证方式 | 负责 |
|---|---|---|
| AC-01 单仓可满足 | Allocator/Pricing 单测 + 预览集成 | Inventory/Pricing |
| AC-02 拆量成交 | Allocator 单测 + 预览集成（2 行 2 组） | Inventory/Pricing |
| AC-03 拆量不改价 | Pricing 金额单测（Σ 优惠/实付守恒） | Pricing |
| AC-04 运费只收一次 | Pricing 单测 + 对账等式 3 | Pricing/Order |
| AC-05 包邮门槛按单 | Pricing 单测（99 元拆 2 组） | Pricing |
| AC-06 总量不足 | Allocator 单测 + BFF 错误契约 | Inventory/Gateway |
| AC-07 拆量上限 | Allocator 单测（>2 仓/>3 组） | Inventory |
| AC-08 并发抢最后一件 | MySQL 并发测试（`-race`，条件更新） | Inventory/Order |
| AC-09 支付后不重分配 | 冻结守卫集成（改行/组/仓必失败） | Order |
| AC-10 行单价末次补差 | AfterSales 金额单测（333+667） | AfterSales |
| AC-11 退款不可自填 | API 契约测试（期望值不影响打款） | AfterSales |
| AC-12 双层额度 | AfterSales+Payment 集成（行额度/captured） | AfterSales/Payment |
| AC-13 跨仓同 SKU 分开退 | 行台账集成（两行互不挤占） | AfterSales |
| AC-14 运费退还判定 | AfterSales 单测（5 场景） | AfterSales |
| AC-15 回库先于退款 | 状态守卫集成 | AfterSales |
| AC-16 回库归属 | 集成（G2→仓 2；跨仓默认拒绝） | AfterSales/Inventory |
| AC-17 承运商受控 | Fulfillment 单测（未登记/停用拒绝） | Fulfillment |
| AC-18 运单号唯一 | DB 唯一索引 + 集成（重复阻断） | Fulfillment |
| AC-19 轨迹单调 | 轨迹 rank 单测（DELIVERED 后不回退） | Fulfillment |
| AC-20 签收自动确认 | 自动确认 job worker 测试（T+1） | Fulfillment |
| AC-21 无轨迹自动确认 | 自动确认 job worker 测试（T+7） | Fulfillment |
| AC-22 签收争议 | 争议集成 + SLA 到期告警测试 | Fulfillment |
| AC-23 客服不可改签收 | 权限/API 契约测试（仅裁决入口） | Fulfillment/Admin |
| AC-24 异常不自动退款 | 事件测试（异常仅 OPEN） | Fulfillment |
| AC-25 生命周期不含退款态 | 状态机单测 + 代码扫描（禁止写入） | Order |
| AC-26 组级投影 | 投影集成（G1 发/G2 未发） | Order |
| AC-27 已退款未发组不卡单 | 投影集成（G2 退款、G1 发货） | Order |
| AC-28 付款前告知拆单 | Playwright（预览页包裹数） | Gateway |
| AC-29 付款后组级物流 | Playwright + BFF 组合（Order+Shipments） | Gateway/Admin |
| AC-30 缺货显式二选一 | 缺货流程集成 + Playwright（48h 默认退款） | Order/Gateway |

每条 AC 必须在测试名或测试日志中可追溯到编号；仅快乐路径不算通过。

---

## 15. Open Questions（必须由 PO/甲方回答，架构师不代答）

| ID | 问题 | 现状假设 | 阻塞范围 |
|---|---|---|---|
| OQ-01 | “T+1 天 / T+7 天”按**自然日 00:00**还是**24 小时**计？时区是否固定 Asia/Shanghai？ | 暂按 24h 实现，展示按自然日 | AC-20/21 |
| OQ-02 | “3 个工作日”是否排除法定节假日？依据哪份日历？ | 暂按周一至周五，不排节假日 | AC-22 |
| OQ-03 | 包邮门槛的“商品实付总额”是否 = `Σ line_paid`（扣优惠后、不含运费）？ | 是 | AC-05 |
| OQ-04 | 6.4.2 组运费分摊的“组商品金额”是**优惠前**还是**优惠后**？ | 暂按优惠前 `price×qty` | INV-20 |
| OQ-05 | 6.5.1“部分数量退货，除非该组为商家责任”：商家责任时退**全额运费**还是**按比例**？ | 暂按全额 | AC-14 |
| OQ-06 | 承运商清单由谁审批、谁可启停？是否要求双人复核？ | 暂定 Admin 角色单人 + 留痕 | AC-17 |
| OQ-07 | 支付后缺货的“等待补货日期”由谁填写、是否必须为未来日期？ | 暂定运营填写，校验 > now | AC-30 |
| OQ-08 | 争议裁决“用户胜”后，已自动确认的收货是否回滚？订单如何展示？ | 暂定撤回 receipt，组退回 SHIPPED，订单不回滚生命周期 | AC-22 |
| OQ-09 | Spec 2.2 把“补发”列为非目标，但 6.6/E9 要求缺货/少发可“同仓补发”。本期是否实现有限补发？ | 暂定**只做部分退款**，补发开关默认关闭，等甲方裁决 | E9/AC-30 |
| OQ-10 | 部分组退款且商家责任时，“全额退运费”是**该组分摊**还是整单运费？ | 暂定**该组分摊**；最后一组关闭时退剩余订单运费，保证 INV-20 不被突破 | AC-14/AC-30 |

> 甲方于 2026-11-22 批准 ADR 0005 时，上述 OQ-01…OQ-10 按本节“现状假设”一并批准为**暂定执行口径**。任何业务口径变更必须升版 Spec（并同步本 ADR），**不得**在代码或配置默认值里私改；OQ-09 未升版前，补发能力保持关闭，只实现部分退款。

---

## 16. Consequences

**正面**

- 拆量可成交，且运费/优惠/退款三套金额只有一个计算源。
- 签收事实可举证，客服无法手工改状态。
- 所有异步失败都有 durable 恢复点；对账可定位差异。

**代价**

- 契约面扩大：proto 新增字段/RPC、事件 8 个、schema 5 个服务。
- 分配器有组合搜索，需要预算与降级策略。
- 历史售后数据需要回填与人工核对队列。
- 运营新增承运商/争议/缺货三类工作面。

**明确不做**

- 不为拆量新增生命周期状态；不引入第二套库存/金额事实源；不引入新中间件。

---

## 17. 批准

| 角色 | 结论 | 日期 |
|---|---|---|
| System Architect | 提交 ADR 0005（proposed） | 2026-11-22 |
| 甲方/人类指挥官 | **批准 ADR 0005（accepted）**；OQ-01…OQ-10 按 §15 暂定口径一并批准 | 2026-11-22 |
| 甲方/PO | 裁决 B-01、B-05 成立且未完成；B-03 由 Spec v1.1 定义 | 2026-11-22 |
| System Architect | 回写 **Amendment 1（§18）**，补齐 B-01…B-08 / C-01…C-03 | 2026-11-22 |
| Developer | **可开工**；按 §13 阶段 A→F，且遵守 §12.2 否决清单 | — |

已完成：回填 `docs/specs/0001-fulfillment-quantity-split.md` 的 `adr: [0005]`，并登记 `docs/specs/README.md` 关联 ADR。

---

## 18. Amendment 1 — 实施回写（2026-11-22）

> 触发：SPEC-0001 v1.1（AC-02 修正、6.5.2 责任判定）+ [偏差登记](../implementation/adr-0005-deviation-register.md)。
> 效力：本节是增量契约；与前文冲突以本节为准；未提及处继续有效。
> 结论：**B-01、B-05 成立且未完成**。在 §18.2 / §18.6 落地并通过测试前，**INV-28、AC-30 不得判 done**。

### 18.1 裁决总表

| 项 | 裁决 | ADR 落点 | Developer 动作 |
|---|---|---|---|
| A-01 AC-02 冲突 | 以 Spec v1.1 修正后的 AC-02 为准（单仓优先） | §18.11 | 测试改为 2+2/3，并补“单仓足够不拆”反向用例 |
| B-01 可用量缺失 | **成立，未完成**；补 availability 契约 + Order 下单复算并落审计 | §18.2 | 实现 proto/Order 复算/`availability_json`，补 AC |
| B-02 优先级无来源 | 用配置 `INVENTORY_WAREHOUSE_PRIORITY`，默认按 warehouse_id | §18.3 | 解析配置、排序与 `Priority/Rank` 赋值 |
| B-03 运费判责无输入 | Spec v1.1 §6.5.2 已定义；本 ADR 落字段、状态与 7 天挂起 | §18.4 | 责任字段、运费挂起 worker、AC-31…33 |
| B-04 跨仓回库校验落点 | AfterSales 出具授权号，Inventory 调 `VerifyRestockAuthorization` 校验，失败关闭 | §18.5 | 授权号字段、校验 RPC、worker 传参 |
| B-05 缺货未打款 | **成立，未完成**；AfterSales 消费 `shortage.resolved` 创建系统退款并走 durable task | §18.6 | 消费者、系统退款、幂等、可见性、AC-30 |
| B-06 超 ADR 字段 | 全部批准（只增不改），见 §18.7 | §18.7 | 保留现有字段号，补契约测试 |
| B-07 事件 payload | 见 §18.8 必填字段；registry 收紧后 fail-closed | §18.8 | 更新 producer + registry + contracts |
| B-08 hash 规范 | 定义 `allocation:v1` canonical JSON 与字段顺序 | §18.9 | 加版本字段、golden test |
| C-01 `freight_cent` 冗余 | 保留只读别名，2026-12-31 前移除 | §18.10 | 新读侧只用 `freight_share_cent` |
| C-02 legacy 改额 | 仅历史 `line_no=0`，必须授权留痕，2026-12-31 硬移除 | §18.10 | 加审计表/指标，新单拒绝 |
| C-03 `Receive` | 标记 deprecated，保持 fail-closed，2026-12-31 前移除 | §18.10 | BFF 不再调用，proto 标 deprecated |

### 18.2 B-01：分配可用量契约（INV-28）

**问题**：`order_allocation_decisions.availability_json` 写 `{}`，无法回答“各仓可用量是多少”。这不是可接受的实现细节，是 INV-28 未完成。

**契约（proto，只增）**

```text
message AllocateOrderAvailability {
  int64 sku_id = 1;
  int64 warehouse_id = 2;
  int64 available = 3;   // on_hand - reserved，决策时快照
  int32 priority = 4;    // 0 = 业务优先级最高
  int32 rank = 5;        // 候选排序后的位置，0 起
}
message AllocateOrderReply {
  repeated AllocateOrderAllocation allocations = 1;
  repeated AllocateOrderAvailability availability = 2;   // 新增
}
```

**Order 下单复算（唯一落审计的路径）**

1. `CreateOrder` 在 `getQuote` 之后、锁定优惠之前，调用 `Inventory.AllocateOrder`（`sku+qty` 来自 quote lines，超时 1.5s）。
2. 将返回的 `allocations` 与 quote lines 做**多重集比较**：`(sku_id, warehouse_id, quantity)` 必须完全一致；不一致返回 `FAILED_PRECONDITION` + `retry_preview=true`（E5）。
3. 用返回的 `availability` 生成审计 JSON，写入 `order_allocation_decisions.availability_json`。`availability` 必须包含该 SKU 在 `inventory_stocks` 中的**所有仓（含 available=0）**；选仓只用 `available>0` 的候选。这样才能回答“为什么没选某仓”。

```json
{
  "captured_at_unix": 1760000000,
  "source": "ORDER_REVALIDATION",
  "request": [{"sku_id": 1, "quantity": 3}],
  "warehouses": [
    {"sku_id": 1, "warehouse_id": 1, "available": 2, "priority": 0, "rank": 0},
    {"sku_id": 1, "warehouse_id": 2, "available": 2, "priority": 1, "rank": 1}
  ]
}
```

4. `rule_hits_json` 必须来自实际命中的规则，至少区分：`6.2:single_warehouse`、`6.2:min_warehouses`、`6.2:concentration`、`6.2:priority`、`6.2:available_desc`、`6.2:stable`。禁止硬编码一个固定数组冒充。
5. `AllocateOrder` 失败时：`FAILED_PRECONDITION` 原样返回并补 `retry_preview`；`UNAVAILABLE`（预算超限）原样返回；**禁止**用 `{}`、空数组或过期快照继续建单。

**接口改动**：`order.inventoryPort` 增加 `AllocateOrder`；`recordAllocationDecision` 增加 availability 入参。

**验收**：

- 任一拆量订单的 `availability_json` 必须非空且包含每个候选仓的 `available/priority/rank`。
- 下单时库存变化导致方案不一致 → 不建单、返回 `retry_preview`，且不写“成功”的审计。
- 单测/集成：`TestAllocationDecisionRecordsAvailability`、`TestAllocationRevalidationRejectsChangedPlan`。

### 18.3 B-02：选仓业务优先级来源

**决定**：用配置，不新增表。

- 配置键：`INVENTORY_WAREHOUSE_PRIORITY`，逗号分隔的正整数仓库 ID，例如 `2,1`。
- 空值默认：按 `warehouse_id` 升序，等价于 `1,2,3...`。
- 未列入的仓库排在已列仓库之后，按 `warehouse_id` 升序；配置里未知仓库 ID 允许（尚无库存），启动时 warning。
- 重复 ID、非正整数 → 配置加载失败（fail-closed）。
- `inventory.New` 增加 `Options.WarehousePriority []int64`；`cmd/inventory` 解析。
- `loadCandidates` 排序改为 `(priorityRank, -available, warehouse_id)`，`Priority = priorityRank`、`Rank = 排序下标`，候选上限仍为 8。
- 运行期变更需重启；若甲方要求热更新，另立 ADR 加表，不在本 Amendment 内。

**验收**：仓 2 配置为最高优先级且两仓都够时，选仓 2；同优先级时才按 `-available, warehouse_id`。

### 18.4 B-03：责任判定与运费挂起（Spec v1.1 §6.5.2）

**责任枚举**（全局唯一）：`MERCHANT_WAREHOUSE | CARRIER | CUSTOMER | PLATFORM | UNDETERMINED`。

**`after_sales` 新列**（迁移 `aftersales/0013_responsibility.sql`）：

```sql
ALTER TABLE after_sales
  ADD COLUMN responsibility_party VARCHAR(32) NOT NULL DEFAULT 'UNDETERMINED',
  ADD COLUMN responsibility_source VARCHAR(32) NOT NULL DEFAULT 'SYSTEM',
  ADD COLUMN responsibility_evidence_ref VARCHAR(128) NOT NULL DEFAULT '',
  ADD COLUMN responsibility_reason_code VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN responsibility_determined_by VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN responsibility_determined_at DATETIME(3) NULL;
```

**自动落值（只允许这四类，其余 `UNDETERMINED`）**

| 场景 | 责任 | 来源 |
|---|---|---|
| 未发货整单取消/退款 | `MERCHANT_WAREHOUSE` | `SYSTEM` |
| 支付后缺货（6.6） | `MERCHANT_WAREHOUSE` | `SYSTEM` |
| 用户主动无理由退货 | `CUSTOMER` | `USER_DECLARATION` |
| 承运商丢失/破损 | **不自动落值**，只写 `CARRIER` 建议 | `CARRIER_EVIDENCE` |
| 拒收/错发/少发/地址 | `UNDETERMINED` | — |

自由文本 `reason` **不得**参与判责（INV-29/AC-33）。

**判定入口**：新增 `AfterSales.DetermineResponsibility`（Admin 角色）：

```text
message DetermineResponsibilityRequest {
  string after_sale_no = 1;
  string party = 2;             // 五个枚举之一
  string source = 3;            // AGENT | CARRIER_EVIDENCE | USER_DECLARATION | SYSTEM
  string evidence_ref = 4;      // 运单号/轨迹节点/照片编号
  string reason_code = 5;       // 枚举
  string operator = 6;
}
```

必须写 `responsibility_determined_by/at` 与审计日志；`party=UNDETERMINED` 只允许系统初始值，不允许客服回退。

**运费挂起（强制）**

```sql
CREATE TABLE after_sale_freight_holds (
  after_sale_no VARCHAR(64) PRIMARY KEY,
  freight_cent BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'HELD',   -- HELD/REFUNDED/WAIVED
  deadline_at DATETIME(3) NOT NULL,
  determined_party VARCHAR(32) NOT NULL DEFAULT 'UNDETERMINED',
  determined_at DATETIME(3) NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
);
```

- 商品退款**不等待**责任判定：按行台账正常退商品；运费单独进入 `after_sale_freight_holds`。
- 责任为 `MERCHANT_WAREHOUSE/CARRIER/PLATFORM`：退运费；`CUSTOMER`：不退（`WAIVED`）。
- 责任为 `UNDETERMINED`：`HELD`，`deadline_at = NOW + 7d`；用户可见“运费待判定”。
- 7 天到期 worker（AfterSales，durable job）把 `HELD` 置为 `MERCHANT_WAREHOUSE` 并退运费；超期不得继续拖。
- 运费退款金额：`min(group.freight_share_cent, order.freight_cent - 已退运费)`；若该单已无其他未关闭组，取剩余订单运费。**这是对 6.5.1“全额”的分组解释，见 OQ-10；在甲方澄清前按此默认，保证 INV-20 不被突破。**
- 运费退款使用独立 Payment 幂等键 `after_sale_no + ":freight"`，避免与商品退款互斥。
- `after_sale_refund_breakdowns` 增列 `responsibility_party`、`rule_id`、`refund_idempotency_key`。

**`rule_id` 枚举**：`UNSHIPPED_CANCEL_FULL_FREIGHT`、`MERCHANT_FULL_FREIGHT`、`CUSTOMER_NO_FREIGHT`、`FREIGHT_HOLD`、`FREIGHT_HOLD_TIMEOUT_MERCHANT`、`SHORTAGE_MERCHANT_FREIGHT_SHARE`、`LINE_REFUND`。

**`AfterSale` proto 增列**：17 `responsibility_party`、18 `responsibility_source`、19 `responsibility_evidence_ref`、20 `responsibility_reason_code`、21 `responsibility_determined_by`、22 `responsibility_determined_at_unix`、23 `freight_state`、24 `freight_cent`、25 `freight_deadline_unix`。

**AC-31/32/33**：

- AC-31：回库完成、责任 `UNDETERMINED` → 商品退款成功；运费 `freight_state=HELD` 且 `deadline <= 7d`。
- AC-32：挂起满 7 天 → 自动 `MERCHANT_WAREHOUSE` 补退运费，用户可见。
- AC-33：`reason` 写“破损” → 责任仍 `UNDETERMINED`（或客服判定值），不得自动 `CARRIER`。

### 18.5 B-04：跨仓回库授权凭证

**问题**：授权表在 AfterSales，Inventory 无法 JOIN；必须定义跨服务凭证。

**契约**：

- `RestockRequest` 增列：8 `restock_auth_no`、9 `origin_warehouse_id`。
- `restock_exception_authorizations` 增列：`auth_no VARCHAR(64) UNIQUE`、`expires_at DATETIME(3)`、`used_at DATETIME(3) NULL`。
- AfterSales 新增 `VerifyRestockAuthorization`：

```text
message VerifyRestockAuthorizationRequest {
  string restock_auth_no = 1;
  string after_sale_no = 2;
  int64 sku_id = 3;
  int64 from_warehouse_id = 4;
  int64 to_warehouse_id = 5;
  int64 quantity = 6;
}
message VerifyRestockAuthorizationReply { bool valid = 1; string reason = 2; }
```

**校验规则**：

1. `warehouse_id == origin_warehouse_id` → 不需要授权，直接回原仓。
2. 跨仓：必须带 `restock_auth_no`；Inventory 调 `AfterSales.VerifyRestockAuthorization`，字段全部匹配、未过期（24h）才允许。
3. AfterSales 不可用 / 校验失败 / 过期 → **拒绝**（fail-closed），不得“先收进任意仓”。
4. 同一 restock key 重试允许重复校验（幂等）；`used_at` 只记录首次成功，不阻止重试。
5. 所有跨仓成功必须写 `restock_exception_authorizations.used_at` 与审计日志。

**验收**：AC-16 增加“无授权跨仓必失败；有授权且字段匹配才成功；AfterSales 超时必失败”。

### 18.6 B-05：缺货默认退款必须真正发起

**问题**：当前只把 `order_shortage_cases.status` 置 `RESOLVED_REFUND` 并写事件，没有创建售后/退款任务/打款。AC-30 承诺未兑现，**不得判 done**。

**事件契约**：`shop.order.shortage.resolved.v1` 必填：

```json
{
  "order_no": "ORD...",
  "group_no": "G2",
  "status": "RESOLVED_REFUND",
  "resolution": "REFUND",
  "reason_code": "SHORTAGE_DEFAULT | SHORTAGE_USER_CHOICE",
  "resolved_at": 1760000000,
  "items": [{"line_no": 1, "sku_id": 10, "quantity": 2}]
}
```

`items` 来自 Order 的该组未发货行；不得让 AfterSales 猜行。

**消费者（AfterSales）**：

- subject `shop.order.shortage.resolved.v1`，durable/queue `aftersales-shortage-refund`，`MaxDeliver=10`。
- `resolution != REFUND` → Ack。
- 调内部 `createSystemRefund`，成功后 Ack；失败返回错误 → NAK 重试；DLQ 必须告警。

**系统退款单**：

- `after_sales`：`type=REFUND_ONLY`、`status=APPROVED`、`source=SYSTEM`、
  `client_request_id = "SYS-SHORTAGE:<order_no>:<group_no>"`（唯一键 `(user_id, client_request_id)` 防重）、
  `responsibility_party=MERCHANT_WAREHOUSE`、`responsibility_source=SYSTEM`、`reason_code=SHORTAGE_*`。
- 行：该组所有未发货行，数量 = 行数量 − `after_sale_line_ledger.refunded_quantity`；`items` 只用于**定位行**，系统退款按“行剩余全退”执行，不信任事件里的数量。若全部为 0，则不再创建退款，只把 case 标记完成。
- 金额：商品按 §6.4 计算；运费按 §18.4 的组分摊规则（`SHORTAGE_MERCHANT_FREIGHT_SHARE`）。
- 额度：`after_sale_amount_ledger` 正常占用；`after_sale_line_ledger` 在退款成功时累加。
- 任务：`ensureRefundTaskTx` 创建 durable refund task；由现有退款 worker 执行，不在 NATS handler 里直接打款。
- 执行顺序：先退商品（Payment key = `after_sale_no`），再退运费（key = `after_sale_no:freight`）；运费为 `HELD` 时只退商品并保留 hold；两次调用都幂等，部分成功由 task 重试续跑。
- 幂等：售后单唯一键 + refund task 主键 + Payment `after_sale_no` 幂等键；重复事件只重放已有任务。
- 失败：refund task `FAILED_PERMANENT` 必须告警；Admin 可调 `CreateSystemRefund`（`source=ADMIN_REPAIR`）重开，仍走同一幂等键。
- 组已发货（竞态）：返回永久错误，不退款，告警；缺货上报本身应在发货前，竞态需人工处理。

**可见性**：

- `order_shortage_cases` 增列 `refund_after_sale_no`、`refunded_at`。
- Order 消费 `shop.aftersale.completed.v1` 时更新对应 case（按 order_no + group_no）。
- `ShortageCase` proto 增列：8 `refund_after_sale_no`、9 `refunded_at_unix`、10 `refund_state`。
- BFF 展示：`RESOLVED_REFUND` 且无 `refund_after_sale_no` → “退款处理中”；有 → “已退款”。**不得只显示“已决议”而不显示退款进度。**

**AC-30 通过条件（新）**：

1. 48h 未选择后，`after_sales` 行存在且 `client_request_id=SYS-SHORTAGE:<order>:<group>`；
2. refund task 存在且被 worker 推进（成功或可重试失败）；
3. 用户详情页可见“处理中/已退款”；
4. 集成测试覆盖“事件重复不重复退款”。

**对账**：§11 增加等式 4：`RESOLVED_REFUND` 的 shortage case 在 24h 内必须有对应 `after_sale`；缺失写 `order_reconciliation_diffs` 并告警。

### 18.7 B-06：proto 字段批准

全部为**只增不改**，批准如下：

| 消息 | 字段 | 语义 |
|---|---|---|
| `CreateOrderInnerRequest` | 17 `allocation_hash`、18 `package_count`、19 `warehouse_count` | DTM inner 分支携带冻结校验所需快照元数据 |
| `CreateShipmentRequest` | 7 `carrier_code` | 受控承运商 code |
| `PreviewRefundQuoteRequest` | 4 `user_id`、5 `type` | 归属校验 + 运费规则 |
| `ListLineRefundsRequest` | 2 `user_id`、3 `admin` | S14 读模型鉴权 |
| `AllocateOrderReply` | 2 `availability` | §18.2 |
| `RestockRequest` | 8 `restock_auth_no`、9 `origin_warehouse_id` | §18.5 |
| `AfterSale` | 17–25 | §18.4 |
| `ShortageCase` | 8 `refund_after_sale_no`、9 `refunded_at_unix`、10 `refund_state` | §18.6 |
| 新 `CreateSystemRefundRequest` | 1 `order_no`、2 `group_no`、3 `source`、4 `idempotency_key`、5 `items`、6 `reason_code` | Admin/系统补退款 |
| 新 `VerifyRestockAuthorizationRequest/Reply` | 见 §18.5 | 跨仓回库校验 |
| 新 `DetermineResponsibilityRequest/Reply` | 见 §18.4 | 客服判责 |

约束：改完必须 `make proto`、提交 `api/gen`、更新 baseline，`make contract-check` 通过。

### 18.8 B-07：事件 payload 必填字段

registry 的 `required` 收紧后，缺字段会在 `outbox.Append` fail-closed；`docs/contracts.md` 同步。

| 事件 | 必填字段（除既有） | 生产者 → 消费者 |
|---|---|---|
| `order.allocation_frozen` | `order_no, allocation_hash, package_count, warehouse_count, frozen_at` | Order → 审计/BI |
| `order.shortage_reported` | `order_no, group_no, status, choose_deadline_unix, reason_code` | Order → 通知/BI |
| `order.shortage_resolved` | `order_no, group_no, status, resolution, reason_code, resolved_at, items[]` | Order → AfterSales（§18.6） |
| `shipment.tracked` | `shipment_no, order_no, carrier_code, tracking_no, track_status, event_id, occurred_at_unix` | Fulfillment → Order/BI |
| `shipment.exception` | `shipment_no, order_no, exception_type, occurred_at_unix` | Fulfillment → 运营 |
| `shipment.received` | `shipment_no, order_no, source, confirmed_at_unix` | Fulfillment → Order |
| `shipment.dispute_opened` | `shipment_no, order_no, dispute_id, opened_at_unix` | Fulfillment → Order/运营 |
| `shipment.dispute_resolved` | `shipment_no, order_no, dispute_id, status, resolved_at_unix` | Fulfillment → Order/运营 |
| `aftersale.line_refunded` | `after_sale_no, order_no, line_no, quantity, refunded_amount_cent` | AfterSales → BI/对账 |
| `aftersale.completed` | 既有 + `group_no, effective_amount_cent, freight_cent, rule_id` | AfterSales → Order/Pricing/Fulfillment |

### 18.9 B-08：allocation hash v1 canonical 契约

**唯一实现**：`internal/platform/allocation`。Pricing 与 Order 必须调用同一函数。

**canonical JSON**（字段顺序固定，禁止 `omitempty`，禁止额外字段）：

```json
{
  "v": 1,
  "lines": [
    {"line_no":1,"sku_id":10,"warehouse_id":1,"group_no":"G1","quantity":2,
     "price_cent":1000,"discount_cent":100,"line_paid_cent":1900,"unit_paid_cent":950}
  ],
  "groups": [
    {"group_no":"G1","warehouse_id":1,"goods_cent":2000,"discount_cent":100,"freight_share_cent":1000}
  ]
}
```

规则：

1. `lines` 按 `line_no` 升序；`groups` 按 `group_no` 升序；不得重复。
2. 字段集合与顺序固定为上面的结构；`line_no/sku_id/warehouse_id/group_no/quantity/price_cent/discount_cent/line_paid_cent/unit_paid_cent`，组为 `group_no/warehouse_id/goods_cent/discount_cent/freight_share_cent`。
3. 不含 `name`、`package_count`、`warehouse_count`、时间戳、随机数。
4. `SHA-256` 小写 hex。
5. 任何字段/顺序变更 → `v+1`，并在 ADR 记录；旧 quote 一律 `retry_preview`。
6. 必须加 golden test（固定输入 → 固定 hash）；Pricing/Order 各引同一向量。

### 18.10 C-01 / C-02 / C-03 退役标准

| 项 | 保留条件 | 移除标准 |
|---|---|---|
| C-01 `QuoteGroup.freight_cent` | 只读别名，与 `freight_share_cent` 同值；旧 admin/UI/TS 未迁移 | 2026-12-31 前完成读侧迁移；之后新代码禁止写/读 `freight_cent`，下一迁移删列 |
| C-02 `Approve.amount_cent` legacy | 仅 `line_no=0` 的历史售后单；必须写 `after_sale_legacy_amount_adjustments`（operator/reason/amount/approved_by/时间）并打指标 | **2026-12-31 硬移除**；新单（`line_no>0`）一律 `INVALID_ARGUMENT`；历史单未回填则人工处理 |
| C-03 `Receive` | 保留 RPC 但 fail-closed；BFF 只调 `ConfirmReceipt` | 2026-12-31 前确认无调用方后删除；proto 标 `deprecated` |

C-02 的审计表与指标必须在移除前上线；不得用日志代替审计。

### 18.11 A-01：AC-02 以 Spec v1.1 为准

- Spec v1.1 AC-02：仓 1=2、仓 2=2、买 3 → 拆 2 行；并新增反向判据：任一仓可单独满足 3 件时必须单仓不拆。
- ADR §5.2“单仓优先”不变；`TestAllocateOrderQuantitySplit` 必须同时覆盖正向与反向。
- 本项不需要新的 ADR 条款，仅以 Spec v1.1 为验收基线。

### 18.12 影响清单与 AC 映射

- **INV-28**：由 §18.2 闭环；`availability_json` 非空才可判过。
- **AC-30**：由 §18.6 闭环；只到 `RESOLVED_REFUND` 不算通过。
- **INV-29 / AC-31/32/33**：由 §18.4 闭环。
- **AC-16**：由 §18.5 闭环（无授权跨仓必失败）。
- **E8**：增加“AfterSales 消费者 + 系统退款 + 可见性”。
- **E14**：改为“授权号 + Verify RPC + fail-closed”。
- **Order 收货投影改绑**：`COMPLETED` 只由 `shop.shipment.received.v1` 推进；`shop.shipment.delivered.v1` 只作为签收证据/展示，**不得**再推进 `COMPLETED`（否则 AUTO_T7 与争议语义会被绕过）。

AC-31…33 验证映射：

| AC | 验证 |
|---|---|
| AC-31 | AfterSales 集成：回库完成、责任 UNDETERMINED → 商品退款成功、运费 HELD |
| AC-32 | 把 `deadline_at` 拨到过去，跑 worker → 责任转 MERCHANT、运费退款、用户可见 |
| AC-33 | 创建售后 reason 含“破损” → `responsibility_party` 仍为 UNDETERMINED 或客服值 |

### 18.13 Developer 交付检查表（Amendment 1）

1. §18.2 availability 契约与 Order 复算审计；`availability_json` 不得为 `{}`。
2. §18.3 优先级配置与排序；补“仓 2 优先”单测。
3. §18.4 责任字段、判定 RPC、7 天挂起 worker、AC-31/32/33。
4. §18.5 跨仓授权号、Verify RPC、fail-closed、AC-16。
5. §18.6 缺货消费者、系统退款单、幂等、可见性、AC-30；重复事件不重复退款。
6. §18.7 proto 字段与 `make contract-check`。
7. §18.8 registry required 收紧 + `docs/contracts.md`。
8. §18.9 hash v1 + golden test。
9. §18.10 退役计划与审计/指标。
10. 以上每条附测试日志；未跑项写 `[未覆盖范围]`。

**否决条件**：`availability_json` 仍为 `{}`、缺货只写事件不打款、自由文本判责、跨仓回库无授权校验 —— 任一条出现，Amendment 1 不通过。
