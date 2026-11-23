# ADR 0007: 支付结算与履约准入（P0-A / P0-B）

- Status: **accepted**（Director 2026-11-23 批准；QC-ADR-01/02/03 整改）
- Approved: 2026-11-23（含 INV-43 组级适用范围）
- Revision: **1.0**
- Amended by: [0007-payment-settlement-admission-amd1.md](0007-payment-settlement-admission-amd1.md)（accepted 2026-11-24）
- Date: 2026-11-23
- Spec: [SPEC-0002 v1.7](../specs/0002-payment-settlement-admission.md)（approved）
- Integration owner: **Director**（人类）；设计 owner/秘书：System Architect；deputy：待指派
- Related: [architecture-baseline.md](../architecture-baseline.md)（INV-01…16）、[ADR 0005](0005-fulfillment-quantity-split.md)（履约/售后基线）、[ADR 0006](0006-collab-gates.md)、[ext-review-1123-triage.md](../ext-review-1123-triage.md)
- Change ID: `PAY-SETTLE-01`
- Baseline impact: `INV-01/04/05/06/07/14/24/26/30…44`；`AC-39…61`
- Supersedes: 无；本 ADR 修订现有直接 MarkPaid / TCC Confirm / 退款编排的实现协议

---

## 0. 决策摘要（D-01…D-14）

| ID | 决策 | 理由 |
|---|---|---|
| D-01 | **四事实分离**：`PaymentCaptured`（Payment）、`OrderSettled`（Order 的 `PAID`）、`InventoryReady`（Inventory）、`FulfillmentAllowed`（Order 的组级许可谓词）各有唯一所有者；任何一方不得用单一 `order.paid` 推导另外三者 | SPEC-0002 INV-35 |
| D-02 | **捕获后持久结算**：Payment 在写 `SUCCEEDED` 的同一本地事务写 `payment_settlement_tasks`，worker 驱动 DTM/直接结算；W1 由该任务恢复 | SPEC-0002 INV-32/AC-43 |
| D-03 | **首次支付足额**：`CreateIntent` 反查 Order，校验订单存在、用户匹配、`PENDING_PAYMENT`、`amount_cent == pay_cent`；异常捕获不生效并走**支付级退款** | SPEC-0002 INV-30/AC-39 |
| D-04 | **直接/TCC 共用 `settleOrderTx`**：它是唯一把订单置 `PAID` 的写路径；直接路径与 Order TCC 分支都调用它 | SPEC-0002 INV-34 |
| D-05 | **同步就绪校验是 PAID 强制前置**：Order 在结算前调用 `Inventory.CheckOrderReady`（新只读 RPC）；不可用/超时 → fail-closed，不得回退到异步投影；TCC 分支顺序不作为正确性依赖 | INV-06/INV-34；QC-ADR Q-02 |
| D-06 | **CAS=0 必须重读**：只有 `PAID` 且 `settlement_payment_no=本 payment_no` 才幂等成功；`PENDING_PAYMENT` 可重试；`CANCELLED` 返回 `ORDER_CANCELLED` 并退款；其他冲突不触发副作用 | SPEC-0002 INV-33/AC-45 |
| D-07 | **单一生效支付**：`orders.settlement_payment_no` 是唯一生效支付；重复捕获在 CAS 裁决**之后**写 `payment_duplicate_captures` 并走**支付级退款**，不计入订单 captured/refunded | SPEC-0002 INV-31/INV-37/AC-41/AC-53 |
| D-08 | **退款分两类**：**组级退款**（售后/TCC 补偿/缺货）走 `AdmitRefund` + 许可行 CAS；**支付级退款**（非生效捕获/超少付/取消）由 Payment 直接幂等退款，不经组许可，只受 Payment captured 上限约束 | SPEC-0002 INV-36/INV-42/INV-43；QC-ADR Q-06 |
| D-09 | **额度双层 + Payment 权威**：组级退款准入读 Payment `GetPaymentByOrder` 的订单级 captured/refunded；Order 用 `order_refund_commitments` 串行化在途额度；Order 自身退款投影只用于展示 | SPEC-0002 AC-53/AC-55；QC-ADR Q-03 |
| D-10 | **未离仓整组退款**：未离仓组金额必须等于整组退款金额；部分金额返回 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；TCC 按“系统责任、每组全额、不做分摊”，`Σ 活动组系统责任上限 == 订单剩余 captured`，不等则阻断+人工裁决 | SPEC-0002 AC-50/AC-51、D-19/D-21 |
| D-11 | **已授权未离仓窗口**：`permit=CONSUMED` 且运单 `CREATED` 时，退款准入调用 `Fulfillment.CancelIfCreated`；取消成功→`PRE_SHIP_REVOKED`，已离仓→`POST_SHIP_ALLOWED`；Ship 与取消在运单行 CAS | SPEC-0002 §5.2/AC-50 |
| D-12 | **事件契约复用现有 subject**：`shop.payment.succeeded.v1`、`shop.payment.refunded.v1`、`shop.order.paid.v1`、`shop.order.allocation_frozen.v1`、`shop.inventory.confirmed.v1` 继续使用；仅新增 `shop.order.settled.v1`、`shop.order.refund_admitted.v1`；灰度期 `order.paid` 与 `order.settled` 同事务双发 | QC-ADR Q-07；AGENTS §4 渐进兼容 |
| D-13 | **唯一 TCC gid**：复用 `payment-confirm:<payment_no>`；不新增 `payment-settle` gid；worker 启动前查询既有 gid，禁止同一支付两个 TCC 事务 | baseline §2.4；QC-ADR Q-08 |
| D-14 | **零元单不经过 Payment**：Order 提供 `SettleZero(order_no)` 入口，以 `settlement_payment_no=zero:<order_no>` 直接结算；零元退款申请 `DONE(zero_amount)`，不调 Payment | SPEC-0002 AC-40；QC-ADR Q-09 |

---

## 1. 事实模型与事件契约

### 1.1 事实所有权

| 事实 | 所有者 | 持久化形态 | 消费者 | 禁止 |
|---|---|---|---|---|
| PaymentCaptured | Payment | `payment_intents(SUCCEEDED, captured_cent)` + `payment_callbacks` 去重 | Order、AfterSales、对账 | 不表示库存就绪/可发货 |
| OrderSettled | Order | `orders.status=PAID` + `settlement_payment_no` + `allocation_frozen_at/hash` | Fulfillment、BFF、AfterSales | 不由 Payment 事件顺序推导 |
| InventoryReady | Inventory | 行预留 `CONFIRMED`；以 `CheckOrderReady` 同步返回 | Order | 不由 `PAID`/Catalog 投影推导 |
| FulfillmentAllowed | Order | 组级谓词：`PAID ∧ 行 CONFIRMED ∧ 无活动缺货 ∧ permit=NONE` | Fulfillment | 不是新状态字段 |
| RefundAdmission | Order | `order_refund_admissions(refund_id 唯一)` + 许可行 CAS + `order_refund_commitments` | AfterSales/Payment/Fulfillment | 不是打款事实 |
| Shipment/Receipt | Fulfillment | `shipments` + `shipment_receipts` + `shipped_at` | Order、BFF | Order 不得制造签收事实 |

### 1.2 事件（规范 subject）

| Subject | 状态 | 生产者 → 消费者 | 最小 payload | 备注 |
|---|---|---|---|---|
| `shop.payment.succeeded.v1` | 既有 | Payment → Order/BI | `payment_no, order_no, amount_cent, captured_at, channel` | 即 PaymentCaptured；`payment_no` 唯一 |
| `shop.payment.refunded.v1` | 既有，扩展字段 | Payment → Order/AfterSales/BI | 见 §4.2 | `refund_id` 幂等；Order 只消费订单级字段 |
| `shop.order.paid.v1` | 既有，兼容双发 | Order → BFF/旧消费者 | 既有字段 | 不再作为履约准入依据 |
| `shop.order.allocation_frozen.v1` | 既有 | Order → 审计/BI | 既有字段 | 与结算同事务 |
| `shop.order.settled.v1` | **新增** | Order → Fulfillment/AfterSales/BI | `order_no, payment_no, settled_at, allocation_hash, warehouse_count, package_count, groups[]` | `order_no+payment_no` 唯一 |
| `shop.order.refund_admitted.v1` | **新增** | Order → AfterSales/Payment/Fulfillment | `refund_id, order_no, group_no, result, amount_cent, version` | `refund_id` 唯一 |
| `shop.inventory.confirmed.v1` | 既有 | Inventory → Order | 既有字段 | 仅投影加速；**不得**作为 PAID 依据 |

- registry 必须新增 `order.settled`/`order.refund_admitted` 的必填字段；未知事件 fail-closed。
- P0 只支持单仓 G1；`warehouse_id/group_no` 在多仓事件中的身份升级属 P1（另立 ADR-0005 Amendment），**不是本 ADR 的前置**。
- `order.paid` 与 `order.settled` 在同一结算事务内双发；旧消费者清单见 §12.3。

---

## 2. 支付意图与捕获（Payment）

### 2.1 `CreateIntent`

1. 校验 `order_no`、`amount_cent>0`。
2. 调 `Order.GetOrder(Admin)`（deadline 500ms）；校验订单存在、`user_id` 匹配、`status=PENDING_PAYMENT`、`amount_cent == pay_cent`。
3. 失败返回 `PAYMENT_AMOUNT_MISMATCH`/`ORDER_STATE_CONFLICT`，不创建 intent。
4. 按 `(order_no, amount_cent, channel)` 幂等创建 intent；`user_id` 由 Order 反查。

**零元单不走此入口**：`pay_cent=0` 由 BFF 调 `Order.SettleZero(order_no)`（§3.5），Payment 不创建 intent/任务。
Order 增加零元结算恢复：`zero_settlement` 任务（`order_no` 唯一）或由 Order worker 扫描 `PENDING_PAYMENT AND pay_cent=0`；BFF 调用幂等，崩溃后由 worker 继续。

### 2.2 Callback 本地事务

同一本地事务：

1. `payment_callbacks` 去重（`UNIQUE(channel,event_id)`，复用既有表）。
2. `intent.status=SUCCEEDED` + captured。
3. 写 `payment_settlement_tasks(payment_no PK, state=PENDING, attempts=0, dtm_gid='payment-confirm:<payment_no>', ...)`。
4. `outbox.Append(shop.payment.succeeded.v1)`。
5. 提交后才 ack；本地持久化失败返回可重试错误，不得 ack。

### 2.3 金额/身份裁决（在 Order CAS 之后）

| 输入 | 裁决 |
|---|---|
| 同一 `payment_no` 重放 | 幂等成功，不重复捕获/副作用 |
| 同一 `payment_no` 金额/订单不符 | 拒绝并告警；真实入账则**支付级退款**（`refund_id=pay:<payment_no>:mismatch`） |
| 第二个 `payment_no` 成功捕获 | 由 Order CAS 裁决生效支付；非生效捕获写 `payment_duplicate_captures` + **支付级退款**（`refund_id=pay:<payment_no>:dup`），返回 `DUPLICATE_PAYMENT` |
| 渠道捕获 ≠ pay_cent | 不生效；**支付级退款**；`PAYMENT_AMOUNT_MISMATCH` |
| `pay_cent=0` | 不经过 Payment；Order `SettleZero` |

**禁止在回调时预判“第二个=重复”**：必须等 Order 的 `PAID` CAS 结果；非生效方由 worker 在拿到 `DUPLICATE_PAYMENT` 后写记录/退款。

### 2.4 结算任务 worker

```text
PENDING -> PROCESSING -> DONE
              │
              ├-> PENDING（指数退避）
              └-> FAILED_PERMANENT（告警 + 人工重放）
```

- 认领：`FOR UPDATE SKIP LOCKED` + `lease_owner/lease_until`；同一 `payment_no` 只允许一个 owner。
- **TCC 启用**：`gid=payment-confirm:<payment_no>`（复用既有 gid）；启动前查询 DTM 是否已有该 gid/在途，若有则接管/查询，不得创建第二个；成功后记 `dtm_status=DONE`。
- **TCC 未启用**：调用直接 `MarkPaid`；同一 `payment_no` 幂等。
- `ORDER_CANCELLED`/`DUPLICATE_PAYMENT`/`PAYMENT_AMOUNT_MISMATCH`：任务转 DONE + 创建**支付级退款**申请（§5.4），不重试结算。
- 重启恢复：任务即入口；人工按 `payment_no` 重放；`cmd/payment/main.go` 必须注册 worker。

---

## 3. 订单结算（Order）

### 3.1 `settleOrderTx`（唯一 PAID 写路径）

**事务外**：调用 `Inventory.CheckOrderReady`（§3.2）并拿到逐行 `CONFIRMED` + `reservation_version`；不可用/超时 → `TRANSIENT_ERROR`，不进入事务。

**事务内**（短事务，禁止远程调用）：

1. `SELECT ... FOR UPDATE` 订单。
2. `status != PENDING_PAYMENT` → 进入 §3.3 裁决。
3. `paid_cent == pay_cent`（零元 `0`）；否则 `PAYMENT_AMOUNT_MISMATCH`。
4. `ensureAllocationSnapshotTx`（缺失则补建）。
5. `freezeAllocationTx`（冻结 `allocation_frozen_at/hash`）。
6. `UPDATE orders SET status='PAID', settlement_payment_no=?, paid_cent=?, inventory_ready_version=? WHERE order_no=? AND status='PENDING_PAYMENT'`。
7. 状态日志 + outbox：`shop.order.paid.v1`（兼容）+ `shop.order.settled.v1` + `shop.order.allocation_frozen.v1`。
8. 写 `pricing_confirmation_tasks`（§3.6，持久任务）。
9. 提交。

**禁止**：TCC 分支另写 PAID/冻结；`afterPaid` 在事务内做远程调用。

### 3.2 `Inventory.CheckOrderReady`（新只读 RPC）

```text
rpc CheckOrderReady(CheckOrderReadyRequest) returns (CheckOrderReadyReply)

CheckOrderReadyRequest {
  string order_no = 1;
  repeated LineRef lines = 2;   // line_no/sku_id/warehouse_id/quantity
}
CheckOrderReadyReply {
  bool ready = 1;
  repeated LineStatus lines = 2;
  int64 reservation_version = 3;
}
LineStatus { int32 line_no; int64 sku_id; int64 warehouse_id; int32 quantity;
             string status; int64 version; }
```

- 语义：只读、幂等；`ready=true` 当且仅当所有行 `CONFIRMED`；`HELD/RELEASED/缺失` → `ready=false`。
- 错误：`INVENTORY_NOT_READY`（明确未就绪）、`TRANSIENT_ERROR`（超时/不可用）；deadline 500ms。
- **不可用即阻断**：结算不得回退到 Order 本地就绪投影；本地投影只用于展示。
- owner：Inventory（实现）+ platform（proto/gen）；integration owner 组织契约评审。

### 3.3 CAS=0 / 重读裁决

| 重读状态 | 结论 |
|---|---|
| `PENDING_PAYMENT`（CAS 未生效，事务可重试） | 重试 `settleOrderTx`（有限次，指数退避）；超限 `TRANSIENT_ERROR` |
| `PAID` 且 `settlement_payment_no=本 payment_no` | 幂等成功；不重复副作用 |
| `PAID` 且 `settlement_payment_no≠本 payment_no` | 本笔为重复捕获；支付级退款；`DUPLICATE_PAYMENT` |
| `CANCELLED` | 不结算；支付级退款；`ORDER_CANCELLED` |
| 其他 | `SETTLEMENT_CONFLICT`；不触发副作用；告警 |

### 3.4 取消

- 仅 `PENDING_PAYMENT` 可取消；取消与结算在 `orders.status` 上 CAS，唯一胜者。
- 取消胜出：已捕获资金走**支付级退款**（§5.4）；`HELD` 释放，`CONFIRMED` 走 `CancelConfirm` 补偿释放；失败重试告警。
- 已 `PAID` 不提供取消；TCC 补偿/结算失败走组级退款。

### 3.5 零元结算 `SettleZero`

- 入口：BFF/Order 内部调用 `Order.SettleZero(order_no)`；无 Payment 参与。
- 校验 `pay_cent=0`、`status=PENDING_PAYMENT`；调用 `settleOrderTx`，`payment_no=zero:<order_no>`，`paid_cent=0`。
- 零元单的取消/缺货/售后：`refund_applications` 以 `source=system_zero`、金额 0；仍走 `AdmitRefund` 撤销许可/取消运单，终态 `DONE(zero_amount)`；不调 Payment；只释放/回库。

### 3.6 `pricing_confirmation_tasks`

- 与结算同事务写入（`order_no` 唯一，`state=PENDING`，payload：quote/coupon/items）。
- worker 调 Pricing 核销（`ConsumeCoupon`/`ConfirmPromotions`），幂等；失败重试，`FAILED_PERMANENT` 告警 + 人工入口。
- 业务终态：核销成功/人工关闭；不得因为核销失败把订单退回未支付或重复退款。

---

## 4. 资金口径（Payment/Order）

### 4.1 生效支付与订单级金额

- `settlement_payment_no` = 唯一生效支付；`PAID` 后必填。**历史 `PAID` 单可为空**，按 §4.4 解析。
- 订单级 `captured` = 生效支付的 captured（零元 0）；**以 Payment 事实为准**。
- 订单级 `refunded` = 针对生效支付的退款累计；不含非生效捕获/支付级退款。
- `payment_duplicate_captures` 单独记账；不进入订单 captured/refunded。
- `FULL/REFUNDED`：`order_captured_cent >= pay_cent` 且 `order_refunded_cent >= order_captured_cent`；由 Order 投影计算，但准入用 Payment 权威值（§4.3）。

### 4.2 `shop.payment.refunded.v1` 字段

```text
refund_id, payment_no, order_no, amount_cent,
intent_captured_cent, intent_refunded_cent,
order_captured_cent, order_refunded_cent,
non_effective_capture(bool), source, reason, status
```

- Order 只消费 `order_captured_cent/order_refunded_cent` 且 `non_effective_capture=false`；用于展示/对账，不用于准入。
- `source=payment|group`；组级退款由 `refund_id` 关联 `refund_applications`。

### 4.3 退款额度（准入）

三层：

1. **Payment 权威**：按 `orders.settlement_payment_no` 调 `Payment.GetIntent(payment_no)`（deadline 500ms）取该生效 intent 的 `captured_cent/refunded_cent`；订单级 captured/refunded = 该 intent 的值（非生效捕获不计入）。历史单先按 §4.4 解析生效支付。
2. **Order 在途账本**：`order_refund_commitments(order_no, group_no, refund_id, amount_cent, state)`；`state ∈ RESERVED | SETTLED | RELEASED`。准入事务锁订单行，校验 `SUM(state=RESERVED) + 本次 <= Payment captured - Payment refunded`。`SETTLED` 已进入 Payment refunded，必须排除；`RELEASED` 也排除。
3. **组/行层**：`组剩余商品额度 = Σ行剩余`；`组剩余运费额度 = 普通售后按 0001 §6.5 本次责任是否可退、TCC 系统补偿按全部未退 freight_share`；`组剩余可退上限 = 商品 + 运费`；`整组退款金额 = 商品剩余 + 本次可退运费`。行层沿用 SPEC-0001 INV-24。

任一层不满足 → `REFUND_ADMISSION_CONFLICT`；不得打款。Payment 的 INV-05 仍是最终防线；Order 投影只展示。

**承诺状态迁移**：`RESERVED → SETTLED`（Payment 退款成功，发起方 worker 在完成回调中更新）/ `RESERVED → RELEASED`（申请拒绝/取消）。完成与拒绝必须更新 commitment，否则下一笔会被双重扣减。

**崩溃窗口修复**：若 worker 在 Payment 退款成功后、更新 commitment 前崩溃，准入在返回 `REFUND_ADMISSION_CONFLICT` 前必须对该订单的 `RESERVED` 承诺做**惰性对账**：按 `refund_id` 查 Payment 退款事实，已退则置 `SETTLED` 后重算；仍未退才拒绝。日终对账任务同样扫描 `RESERVED` 且 Payment 已退的承诺并修复，告警计数。

### 4.4 历史 `PAID` 单的生效支付

- `settlement_payment_no` 为空的历史单：调用 `GetPaymentByOrder`（扩展为返回 `intents[]`，或新增 `ListOrderIntents`），取**唯一**的 `SUCCEEDED` intent 且 `captured == orders.paid_cent` 作为生效支付；写回 `settlement_payment_no`（幂等）。
- 若 0 个或多个候选：进入人工队列（`legacy_payment_resolution`），退款准入阻断并告警；不得按 0 额度处理。
- 无法解析的历史单仍可发货（以许可/库存为准），但退款/对账需人工确认。
- 回填脚本 dry-run、计数对账、可回滚；不强制在业务高峰执行。

---

## 5. 退款准入与编排

### 5.1 退款分类

| 类别 | 来源 | `refund_id` | 是否经 `AdmitRefund` | 说明 |
|---|---|---|---|---|
| **组级退款** | 普通售后 / TCC 补偿 / SYSTEM 缺货 | `after_sale_no`、`tcc:<payment_no>:<group_no>`、`after_sale_no` | **是** | 受许可行/组额度/离仓状态约束 |
| **支付级退款** | 非生效捕获 / 超少付 / 取消 | `pay:<payment_no>:<reason>` | **否** | 只受 Payment captured 上限；不涉及履约 |

`refund_applications` 统一记录两类；`source` 枚举 `aftersale|tcc|system_shortage|system_zero|payment`；组级才有 `group_no`。

**边界**：INV-43“先准入后打款”只约束组级退款；支付级退款由 Payment 的 captured 上限与 Order 的取消/重复捕获裁决保证，不经过组许可。

### 5.2 组级准入 `AdmitRefund`

**事务外**：`Payment.GetIntent(effective_payment_no)` 取权威 captured/refunded（历史单先按 §4.4 解析）。

**Tx1（短事务，Order-owned，禁止远程调用）**：

1. `SELECT order FOR UPDATE`；校验订单 `status=PAID`、组存在且属于该订单、金额。
2. 额度校验（§4.3）；未离仓且金额 < 整组退款金额 → `PARTIAL_REFUND_REQUIRES_FULL_GROUP`，不改状态。
3. 读取许可行：
   - `NONE` → CAS `REVOKED`；写 `order_refund_admissions(refund_id 唯一, result=PRE_SHIP_REVOKED)` + `order_refund_commitments(RESERVED)`；outbox `order.refund_admitted`；提交。
     - **CAS 失败（并发 Ship 先 `NONE→CONSUMED`）**：Tx1 回滚后重读许可/运单，按 `CONSUMED` 分支重新执行；不得直接把结果写成 PRE。
     - Fulfillment 消费 `order.refund_admitted` 的 PRE 结果，调用 `CancelIfCreated` 取消 `CREATED` 运单（幂等）；Order 崩溃不影响 outbox 投递（W2）。IF 消费时返回 `ALREADY_SHIPPED/RECEIVED`，THEN 不得静默忽略：写告警并发出 `order.refund_admission_recheck`，Order 若尚未打款则改为 `POST_SHIP_ALLOWED`；若已打款则停止后续退款并人工对账。
   - `REVOKED` → 幂等 `PRE_SHIP_REVOKED`（写/读 admission）。
   - `CONSUMED`：写 `admission(state=PENDING_REMOTE)` + commitment；提交后调用 `CancelIfCreated`（不依赖事务外预读运单状态）。
4. `order_refund_commitments` 与许可 CAS 同事务。

**Tx1 后（远程，禁止在事务内）**：

- `CONSUMED`：调用 `Fulfillment.CancelIfCreated(order_no, group_no, shipment_no)`：
  - `CANCELLED`（含已取消）→ Tx2 写 `result=PRE_SHIP_REVOKED`。
  - `ALREADY_SHIPPED/RECEIVED` → Tx2 写 `result=POST_SHIP_ALLOWED`。
  - `TRANSIENT_ERROR` → 保持 `PENDING_REMOTE`，worker 重试；**不调 Payment**。
- 已授权未离仓窗口的 `CANCELLED` 即 `CONSUMED+CANCELLED` 分支；Tx2 结果为 PRE。

**Tx2**：按 `refund_id` 更新 admission（唯一键），outbox `order.refund_admitted`；重复调用返回同一结果。

**`CONSUMED + CANCELLED` 分支**：Tx2 结果 `PRE_SHIP_REVOKED`；许可保持 `CONSUMED`（终态），该组无运单可发；新运单因不同 `shipment_no` 被 `FULFILLMENT_NOT_ALLOWED`。W2 重试按运单状态收敛。

### 5.3 `Fulfillment.CancelIfCreated`

```text
rpc CancelIfCreated(CancelIfCreatedRequest) returns (CancelIfCreatedReply)
CancelIfCreatedRequest { order_no; group_no; shipment_no; refund_id }
CancelIfCreatedReply { status: CANCELLED | ALREADY_SHIPPED | RECEIVED | TRANSIENT_ERROR }
```

- CAS `CREATED→CANCELLED`；已 `CANCELLED` 幂等；`SHIPPED/RECEIVED` 返回对应状态；不取消已离仓运单。
- 与 Ship 的 `CREATED→SHIPPED` CAS 竞争，唯一胜者；返回码决定 admission 结果。
- deadline 1s；owner Fulfillment。

### 5.4 支付级退款

- 触发：`PAYMENT_AMOUNT_MISMATCH`、`DUPLICATE_PAYMENT`、取消胜出。**零元售后/缺货不属于支付级**，走组级 `system_zero`。
- 申请：`refund_id=pay:<payment_no>:<reason>`；`source=payment`；金额 ≤ intent captured；无 group。
- 执行：Payment.Refund 直接幂等（`refund_id` 作为 idempotency key）；Order 不写订单级 refunded（非生效）或仅写取消投影（取消）；不调用 `AdmitRefund`。
- 失败：`refund_applications` 持久重试；用户显示“退款处理中”。

### 5.5 三个组级来源

| 来源 | 申请创建 | 准入 | 打款 | 恢复 |
|---|---|---|---|---|
| 普通售后 | AfterSales 本地事务写 `refund_applications` + task | `AdmitRefund` | AfterSales worker → Payment | task 持久重试 |
| TCC 补偿 | Order 为每个**活动组**创建 `tcc:<payment_no>:<group_no>` | 逐组 `AdmitRefund` | 全部活动组准入完成后逐组打款 | 组级 task；只补剩余 |
| SYSTEM 缺货 | AfterSales 系统售后 + 申请 | `AdmitRefund` | 同普通售后 | 同普通售后 |
| SYSTEM 零元 | AfterSales 系统售后 + 申请（金额 0） | `AdmitRefund` | 不调 Payment；`DONE(zero_amount)` | 同普通售后 |

TCC 额外规则：
- 活动组 = 系统责任上限（商品剩余 + 全部未退运费）> 0 且未全退。
- **Σ 活动组系统责任上限必须 == 订单剩余 captured**；不等 → 阻断、告警、人工裁决（owner：Director + Architect；动作：按 `payment_no` 对账/补录，不部分打款）。
- 逐组全额，不做最大余数分摊；全退组跳过；部分成功只补未 DONE 的组。

### 5.6 准入结果持久化与重试

- `order_refund_admissions(refund_id PK, result, state, amount, version)`；结果只允许 `PRE_SHIP_REVOKED/POST_SHIP_ALLOWED`，中间态 `PENDING_REMOTE`。
- 同 `refund_id`：读结果返回；缺失则重跑；`PENDING_REMOTE` 继续远程取消。
- 不同 `refund_id`：组级按额度/离仓规则；支付级按 Payment captured。
- 所有 worker 带 lease owner；失租不得写 admission/commitment。

### 5.7 组退款后的库存

- 未离仓：许可 `REVOKED`/运单 `CANCELLED` 生效后释放（`HELD` 直接，`CONFIRMED` 走 `CancelConfirm`）；失败重试告警，不阻断退款完成，但禁止再发货。
- 已离仓：退货回库按 SPEC-0001 INV-03；回库成功后退款。
- 释放后 `available <= on_hand`、行数量守恒；违反告警并阻断该组。

---

## 6. 履约准入（Order/Fulfillment）

### 6.1 Ship 顺序

1. Fulfillment 读运单：`CANCELLED` → `SHIPMENT_CANCELLED`；`SHIPPED/RECEIVED` → 幂等；`CREATED` 继续。
2. 调 `Order.AuthorizeShipment`：
   - `PAID` + 行 `CONFIRMED` + 无活动缺货 + permit `NONE` → `NONE→CONSUMED`。
   - `CONSUMED` 同一 `shipment_no` → 幂等；不同/`REVOKED` → `FULFILLMENT_NOT_ALLOWED`。
3. Fulfillment CAS `CREATED→SHIPPED`；与取消竞争，唯一胜者。

### 6.2 自动建单

- 灰度期 Fulfillment 消费 `order.settled`（不是 `order.paid`）创建 `CREATED` 运单；仍须查 `FulfillmentAllowed`。
- 旧 `order.paid` 消费者在灰度期只做影子/停用；不得从 `paid` 推进组或建可发货运单（§12.3）。

---

## 7. 库存就绪与缺货

- 结算前：`CheckOrderReady` 强制同步；不可用 fail-closed，不 PAID。
- **TOCTOU 规则（收窄）**：Inventory **不得自行释放 `CONFIRMED` 预留**；`HELD` 的 TTL 过期释放保留（它是未支付订单的安全网，PAID 要求全行 `CONFIRMED`，因此不会产生 `PAID + RELEASED`）。`CONFIRMED` 的释放只能由 Order 的取消/退款命令触发，且 Order 先在 `orders.status` 上 CAS（PAID/CANCELLED）成功后才发命令。若现有 Inventory 对 `CONFIRMED` 也有过期释放，必须在 P0 关闭并列入前置核查。
- `inventory_ready_version` 只作审计/展示，不作为 PAID 的 CAS 条件；正确性由“释放命令后置 + Order CAS 唯一胜者”保证。
- 支付后缺货：案件 `NOTIFIED/RESOLVED_WAIT`；预留保持 `CONFIRMED`；禁发；`RESTOCKED` 恢复；`RESOLVED_REFUND` 释放 + 组退款。
- 4h/48h/ETA/迟到补货：按 SPEC-0002 AC-56 与 SPEC-0001 §6.6。

---

## 8. 状态机与幂等键

```text
Order:  PENDING_PAYMENT -> PAID -> ALLOCATED -> SHIPPED -> COMPLETED
        PENDING_PAYMENT -> CANCELLED
Permit: NONE -> CONSUMED | REVOKED（终态）
Refund app: PENDING -> PROCESSING -> DONE | FAILED_PERMANENT | CANCELLED | REJECTED
Admission: PENDING_REMOTE -> PRE_SHIP_REVOKED | POST_SHIP_ALLOWED
Case: NOTIFIED -> RESOLVED_WAIT -> RESTOCKED | RESOLVED_REFUND
      NOTIFIED -> RESTOCKED | RESOLVED_REFUND
Task: PENDING -> PROCESSING -> DONE | PENDING(retry) | FAILED_PERMANENT
```

| 对象 | 键 | 生成方 |
|---|---|---|
| 支付意图 | `(order_no, amount_cent, channel)` | Payment |
| 结算 | `payment_no`；TCC gid `payment-confirm:<payment_no>` | Payment |
| 生效支付 | `orders.settlement_payment_no` | Order |
| 发货许可 | `(order_no, group_no)` | Order |
| 组级退款申请 | `refund_id`（after_sale_no / tcc:<payment_no>:<group_no>） | 发起方 |
| 支付级退款 | `refund_id=pay:<payment_no>:<reason>` | Payment/Order |
| 退款执行 | `refund_id` + `payment_no` | Payment |
| 缺货案件 | `(order_no, group_no, case_seq)` + `event_id` | Order |

---

## 9. 恢复所有者（五个崩溃窗口）

| # | 窗口 | 持久意图 | 唯一驱动方 | 人工入口 |
|---|---|---|---|---|
| W1 | 捕获本地提交后崩溃（DTM 未受理） | `payment_settlement_tasks(PENDING)` | Payment settlement worker | 按 `payment_no` 重放 |
| W2 | 跨服务 RPC 成功后本地未记录 | `refund_applications` + `order_refund_admissions(refund_id)` | 发起方 worker；Order 按 `refund_id` 幂等 | 按 `refund_id` 重放；`CancelIfCreated` 幂等 |
| W3 | 消息重复/乱序 | `event_id` 唯一约束 + 状态 CAS | 各消费者 | 死信 + `event_id` 重放 |
| W4 | 取消/确认竞态 | `orders.status` CAS + 支付级退款申请 | Order | 按 `order_no` 状态日志 |
| W5 | 外部渠道/承运商不再重放 | settlement/refund/短缺失效定时器 | Payment/AfterSales/Order worker | 按 `payment_no/refund_id/order_no` |

- 每个 worker 有 lease/owner fencing；失租不得写聚合/事件。
- 任务有最大尝试与告警；永久失败 `FAILED_PERMANENT` + 人工重放。
- 禁止把回调重放/进程存活作为 W1/W2 的唯一恢复。

---

## 10. 错误语义

| 错误码 | 返回方 | 语义 |
|---|---|---|
| `PAYMENT_AMOUNT_MISMATCH` | Payment | 金额 ≠ pay_cent，不生效 |
| `DUPLICATE_PAYMENT` | Order/Payment | 非生效捕获 |
| `ORDER_CANCELLED` | Order | 取消胜出 |
| `ORDER_ALREADY_PAID` | Order | 支付胜出 |
| `ORDER_STATE_CONFLICT` | Order | 非法生命周期迁移 |
| `SETTLEMENT_CONFLICT` | Order | 结算状态冲突 |
| `INVENTORY_NOT_READY` | Order/Inventory | 预留未确认 |
| `FULFILLMENT_NOT_ALLOWED` | Order | 许可被拒 |
| `SHIPMENT_CANCELLED` | Fulfillment | 运单已取消 |
| `REFUND_ADMISSION_CONFLICT` | Order | 额度/状态冲突 |
| `PARTIAL_REFUND_REQUIRES_FULL_GROUP` | Order | 未离仓部分退款被拒 |
| `SHORTAGE_CASE_CLOSED` | Order | 案件终态重复/迟到补货 |
| `SHIPMENT_NOT_SHIPPED` | Fulfillment | 未发货不能确认收货 |
| `RECEIPT_DISPUTE_OPEN` | Fulfillment | 争议未决 |
| `EVENT_OUT_OF_ORDER` | 消费者 | 乱序拒绝 |
| `TRANSIENT_ERROR` | 各服务 | 可重试；不得 ack/标成功 |

---

## 11. 可观测性与对账

- 指标：捕获未结算、settlement task 失败、退款申请状态、准入冲突/PENDING_REMOTE 时长、预留释放失败、缺货超时、`PAID` 与 Payment 不一致数。
- 告警：30 分钟未结算；退款 `FAILED_PERMANENT`；准入冲突；释放失败；4h/48h 超时。
- 日终对账：Payment 订单级/intent 级已收未 PAID、非生效/支付级退款、Order 投影、未完成任务、释放结果；差异告警可重放。
- 五方终态核对：Payment/Order/Inventory/Pricing/Fulfillment。

---

## 12. 迁移、上线与回滚

### 12.1 新增/复用结构

| 结构 | 决策 |
|---|---|
| `payment_callbacks` | **复用**（已有 `UNIQUE(channel,event_id)`） |
| `payment_settlement_tasks` | 新增：`payment_no PK, state, attempts, lease_owner, lease_until, dtm_gid, dtm_status, last_error` |
| `payment_duplicate_captures` | 新增：`payment_no PK, order_no, amount_cent, refund_id, state` |
| `orders.settlement_payment_no` | 新增列（NULL 兼容历史）；`inventory_ready_version BIGINT` |
| `order_refund_admissions` | 新增：`refund_id PK, order_no, group_no, result, state, amount_cent, version` |
| `order_refund_commitments` | 新增：`(order_no, group_no, refund_id) UNIQUE, amount_cent, state(RESERVED/SETTLED/RELEASED), version` |
| `refund_applications` | 新增：`refund_id PK, source, order_no, group_no NULL, amount_cent, state, admission_result, attempts, last_error` |
| `pricing_confirmation_tasks` | 新增：`order_no PK, state, attempts, last_error` |
| 既有 `after_sale_*`/`order_shipping_permits`/`shipments` | 复用，不重命名 |

金额一律 `_cent BIGINT`；可变聚合有 `version`；所有新表有 `created_at/updated_at`。

### 12.2 回填

- 历史 `PAID`：按 §4.4 解析/回填 `settlement_payment_no`；无法解析入人工队列；不按 0 额度处理。
- 历史在途退款：`refund_id=legacy:<order_no>:<refund_seq>`，映射既有 after-sales task；dry-run 计数 + 人工队列。
- 所有回填可重跑、可回滚；不在高峰执行。

### 12.3 灰度与消费者切换

0. **影子**：写 settlement task/refund application，仍走旧路径；对账，不改变行为。
1. **Fulfillment 影子**：同时消费 `order.settled` 与 `order.paid`，只记录不建单；比对。
2. **Fulfillment 切换**：`FULFILLMENT_CONSUME_ORDER_SETTLED=true`，停止从 `order.paid` 建单/推进组；`order.paid` 只作 BFF 兼容。
3. **结算 worker**：`PAYMENT_SETTLEMENT_WORKER_ENABLED=true`；旧同步路径保留为人工兜底。
4. **退款准入**：`REFUND_ADMISSION_ENABLED=true`；旧退款路径只读/停写。
- 回滚：关开关 → 任务保留可重放；不做 down migration；数据前向兼容。
- 启动装配必须注册 settlement worker、refund worker、pricing-confirmation worker、zero-settlement worker；漏注册视为未实现。
- 生产开关默认 false，直到 SPEC-0002 §7 矩阵、G-4、`--release --range`、五方终态通过。

### 12.4 旧消费者清单

`shop.order.paid.v1` 旧消费者需在灰度期核对：Fulfillment 自动建单、BFF/通知、BI。Fulfillment 必须切换到 `order.settled`；其他消费者可继续消费 `order.paid`，但不得据此允许发货。

---

## 13. 假设与不确定性（强制）

1. **DTM 确认/补偿顺序不保证**：Order 用同步 `CheckOrderReady` 兜底，不依赖分支顺序；若 DTM 出现非幂等补偿，停止灰度回 ADR。
2. **`CheckOrderReady` 是新 RPC**：需 proto/gen 与 Inventory 实现；未发布前 `PAYMENT_SETTLEMENT_WORKER_ENABLED=false`，不得用异步投影替代。
3. **P0 只支持单仓 G1**：多仓事件身份 v2 是 P1（另立 ADR-0005 Amendment）；多仓开关保持关闭。本 ADR 不依赖该 Amendment。
4. **Pricing 核销**：`pricing_confirmation_tasks` 持久重试；核销失败不退还支付、不重复退款；人工入口关闭。
5. **历史单**：按 §4.4；无法解析的退款阻断并人工处理；发货仍以许可/库存为准。
6. **渠道回调**：以 `(channel,event_id)` 去重；不假设无限重放；支付级退款可重试。
7. **MySQL 8 DDL 原子非事务**：迁移用 hardened runner 的 attempt/checksum 账本 + 备份。
8. **时钟**：业务时间以 MySQL `NOW(3)` 为准；定时器以 DB 时间；服务时钟偏差不影响 CAS。
9. **R-2（取消记账）**：取消仅 `PENDING_PAYMENT`；取消退款是**支付级**，不进入组额度/订单 refunded 投影；`order_captured=0`。
10. **R-3（历史无组单）**：按 G1；额度=订单剩余 captured；许可行无则初始化 `NONE`；释放按 SPEC-0001 v1.3 §6.6.1。
11. **R-4（TCC 运费与后续判责）**：TCC 系统补偿退全部未退运费；后续责任判责不得二次退同一运费，共享 `order_refund_commitments`/组台账。
12. **容量**：单节点 dev/UAT 不做 100k/1M RPS；不引入新队列/事务平台。
13. **`payment.refunded` 扩展字段兼容**：新增字段在灰度期 nullable/optional；旧消费者忽略；Order 只在 `non_effective_capture=false` 时使用订单级字段；回放按 `refund_id` 幂等，重复事件不得二次投影。
14. **历史人工队列**：无法解析生效支付/legacy 退款的历史单进入人工队列，目标 24h 内处理；期间退款阻断、发货仍以许可/库存为准。
15. **INV-43 适用范围（待人类确认）**：本 ADR 将 INV-43“先准入后打款”限定为**组级退款**；支付级退款（非生效捕获/超少付/取消）是 capture correction，不经过组许可。若人类要求所有退款都经组许可，需为支付级退款定义 synthetic admission，并回写 SPEC-0002。

---

## 14. 反模式自省（强制）

- 禁止直接/TCC 各写一套 freeze/状态/取消补丁；必须走 `settleOrderTx`。
- 禁止用异步就绪投影决定 `PAID`；`CheckOrderReady` 不可用即阻断。
- 禁止用 Order 自己的退款投影做额度准入；Payment 权威 + Order 在途账本。
- 禁止把 `CAS=0/已存在/RPC 超时` 静默当成功。
- 禁止在 DB 事务内做跨服务调用；`CheckOrderReady` 在事务前，`CancelIfCreated` 在 Tx1 后。
- 禁止给每个 worker 加特殊恢复分支；统一任务 + lease/fencing + 人工重放。
- 禁止新增 Kafka/Temporal/Schema Registry；禁止改事件 subject 造成 Break Change。
- 禁止删除/重编号 AC 或把未执行测试写成 PASS。

---

## 15. Baseline 影响与 AC 映射

| 基线/契约 | 影响 |
|---|---|
| INV-01 | 退款不写回 `orders.status`；`settlement_payment_no` 仅结算写 |
| INV-04 | 许可行仍是组级退款/发货唯一裁决点 |
| INV-05 | Payment 末端 `refunded+amount<=captured` 仍保留 |
| INV-06 | `CheckOrderReady` 强制前置 + `settleOrderTx`；组退款释放按 0001 v1.3 §6.6.1 |
| INV-07/14 | outbox 同事务；DTM barrier 不降级 |
| INV-24/26 | 额度双层；缺货 4h/48h 不变 |
| INV-30…44 | 本 ADR 实现目标 |

AC 映射：
- AC-39/40/41/44 → §2；AC-42/61 → §2/§11；AC-43/45 → §2.4/§3；AC-46 → §3.4；
- AC-47/48 → §3.1/§6；AC-49 → §1；AC-50/51/52/53/54/55 → §4/§5；
- AC-56 → §7；AC-57/58 → ADR-0005（不变）；AC-59 → §9 W3（四对乱序逐条落表）；AC-60 → §4.4/§12.2；
- AC-40 零元 → §3.5/§5.1。

乱序逐对（AC-59）：
| 乱序 | 终态 |
|---|---|
| 退款投影先于捕获 | 拒绝/重试；捕获到达后按订单级字段投影 |
| 收货先于发货 | 拒绝/重试；运单从未发货则拒绝并告警 |
| 库存失败先于支付成功 | 结算时 `INVENTORY_NOT_READY`，退款 |
| 缺货解决先于上报 | 拒绝并告警；不得创建已解决案件 |

---

## 16. Consequences

- **正面**：资金终态可恢复；四事实不混用；三类组退款统一；支付级退款独立；可按 AC 写故障注入。
- **代价**：新增表/worker/RPC；直接/TCC 双路径改造；灰度双发/双读复杂；需 DTM/Inventory/Fulfillment 协同。
- **运营**：新增告警/人工入口；对账口径变化；历史单需人工队列。
- **风险**：`CheckOrderReady`/`CancelIfCreated` 未就绪时只能影子/关闭；不得跳过。

---

## 17. 批准

- Status: **accepted v1.0**；Director 2026-11-23 批准；INV-43 组级范围已由 SPEC-0002 v1.7 确认。
- 批准前置（C-1…C-9）：
  1. `CheckOrderReady` 的 proto/字段/owner 评审通过（Inventory + platform）。
  2. `CancelIfCreated` 的 proto/字段/owner 评审通过（Fulfillment）。
  3. `Payment.GetIntent`/`GetPaymentByOrder(intents[])` 与 `payment.refunded` 扩展字段的契约评审通过（Payment + platform）。
  4. settlement/refund/pricing-confirmation/zero-settlement worker 的启动装配与开关清单确认。
  5. event registry 新增 `order.settled`/`order.refund_admitted` 的必填字段评审通过。
  6. 历史 `PAID` 解析与 legacy refund_id 的回填 dry-run 方案通过；人工队列 24h 目标。
  7. 迁移评审（新表/列/唯一键/金额类型/索引）通过。
  8. Inventory 核查：不得自行释放 `CONFIRMED` 预留；`HELD` TTL 释放保留。
  9. 人类确认 INV-43 适用范围（组级退款；支付级退款为 capture correction）——若不认可，需另立 synthetic admission 并回写 SPEC-0002（PO 升版）。
- 批准（2026-11-23）：Developer 按本 ADR 实现；`AC-39…61` 逐条 AC-VERIFIED；Reviewer 出具有范围 verdict。
- 未批准前：不得写支付/结算/退款/准入协议代码。
