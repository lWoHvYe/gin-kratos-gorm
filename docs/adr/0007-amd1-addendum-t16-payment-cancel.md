# ADR-0007 Amendment 1 Addendum — T1.6 Payment-Level Cancel Design

| 项 | 值 |
|---|---|
| Status | **DRAFT — 未批准；T1.6 不进入实现** |
| Approved | — |
| Revision | **0.1-draft（r1）** |
| Tier | **L2**（跨服务/支付级退款/事件/库存补偿） |
| change_id | `PAY-SETTLE-01` |
| Base | ADR-0007 Amd1 + T1.2/T1.4a Rulings Addenda（accepted）+ 零元 Addendum（accepted）+ C-04 RefundBatch（accepted） |
| Spec-Ref | **SPEC-0002 v1.9** AC-46/AC-65/AC-67/AC-68、INV-44/INV-46；ADR Amd1 §4.8 |
| allowed_paths | `internal/services/order/`、`internal/services/payment/`、`internal/services/inventory/`、`internal/platform/events/`、对应 `_test.go` |
| baseline_impact | `order.refund_requested` 消费者、`refundCancelledAfterConfirm`、`TccCancelMarkPaid` 的 PENDING+captured 分支；不改变 C-04 RefundBatch |
| Scope | T1.6 payment-level cancel；不写代码/迁移/测试 |

---

## 1. 结论摘要

1. **PENDING_PAYMENT+captured**：在 TCC 补偿入口先 CAS `PENDING_PAYMENT→CANCELLED`；成功则在**同一事务**写 `order.cancelled` + 支付级取消命令；**不创建组级 application/batch、不进入 AdmitRefund/组扇出、不改 permit**。CAS 失败（已 PAID）→ 进入 §4.1 组扇出。
2. 支付级命令身份：`refund_id=pay:<payment_no>:cancel`；`source=payment`；`non_effective_capture=true`；`amount=captured/paid`；Payment 执行 `Refund`（幂等键同上），**不使用 RefundBatch**（单笔支付级，无组 Σ）。
3. `refundCancelledAfterConfirm` 不再仅写日志：必须写持久 payment-level command（outbox 事务）；失败返回错误/重试，不允许静默丢命令。
4. 保留 `order.refund_requested` 作为**当前命令载体**：T1.5 后 TCC 不再发；T1.6 consumer 只把 `pay:<payment_no>:cancel`/legacy cancel-after-confirm 当命令执行；其余 observe-only。最终 observe-only/移除由 Director release 记录另定。
5. `order.cancelled` 负责 Inventory 补偿；CONFIRMED 预留走 `TccCancelConfirm` 回库，HELD 走 `Release`；Order 不直接改库存。
6. 无 captured/零元：不写 payment command；按 AC-46 本地取消/零元路径；No Payment 不进入本分支。
7. **不可交实现**：本 Addendum DRAFT；先 QC/人类批准 + C-04 event 字段 + T1.6-R 任务与测试。

---

## 2. PENDING_PAYMENT+captured 分支

### 2.1 CAS 与事务

- 入口：TCC 补偿 `TccCancelMarkPaid`/等价裁决点，在 **T1.2 R-3 guard** 位置实现 T1.6。
- 读取权威 Payment `captured/refunded`；仅 `status=PENDING_PAYMENT && captured>0`。
- 事务内：
  1. `SELECT orders FOR UPDATE`；CAS `PENDING_PAYMENT→CANCELLED`（带 version/status）；
  2. 成功 → append `order.cancelled`（库存/优惠补偿，现有协议）；
  3. 同一 tx append payment cancel command（outbox，`refund_id=pay:<payment_no>:cancel`）；
  4. commit；**不写** `refund_batches`/`refund_applications`/`order_refund_admissions`/commitment，不改 permit。
- CAS 0 行/状态已 PAID → rollback；返回 `CANCEL_CAS_LOST`，进入 §4.1 组扇出；不得再发 payment-level cancel。
- outbox append 失败 → 整个 tx rollback；CAS 不落；重试。
- 事务内禁止远程调用；Inventory 由 `order.cancelled` consumer 异步补偿。

### 2.2 Inventory

- `order.cancelled` consumer 按既有 order-level cancel：
  - HELD reservation → `Release`；
  - CONFIRMED reservation → `TccCancelConfirm` 回库补偿；
  - 失败重试/告警，不静默。
- Order 不直接调用 Inventory；不改 C-04 release event（T1.4b2 的 release scope 不用于本分支）。

### 2.3 No Payment / 零元

- `captured=0` 或 `pay_cent=0`：不发 payment command；按 AC-46 本地取消/零元处理。
- 无 Payment 客户端/超时：fail-closed，不进入本分支；不得猜 captured。
- 本分支不产生 `refund_applications(source=payment)` 组级记录；支付级事实在 Payment。

---

## 3. Payment-level command

### 3.1 Event/命令

`order.refund_requested` 当前 subject，新增/使用字段：

```json
{
  "order_no": "...",
  "payment_no": "...",
  "refund_id": "pay:<payment_no>:cancel",
  "source": "payment",
  "non_effective_capture": true,
  "amount_cent": 1234,
  "paid_cent": 1234,
  "reason": "order cancelled after inventory confirm | pending captured cancel",
  "status": "CANCELLED"
}
```

- `refund_id` 是**支付级幂等键**；Payment `Refund` 传入 `IdempotencyKey=refund_id`、`NonEffectiveCapture=true`。
- `amount_cent` = 权威 captured/paid；不得用 order pay_cent 猜测。
- 命令由 Order 在 outbox 事务写；Payment consumer durable/重试；重复事件 Payment.Refund 幂等。
- **不使用 `RefundBatch`**：本命令是单笔支付级退款，无组 items/Σ；RefundBatch 仅 TCC 组级。

### 3.2 `refundCancelledAfterConfirm`

- 当前只发 legacy event + 日志；T1.6 改为：
  1. Inventory CONFIRMED 预留先 `TccCancelConfirm`（失败 queue/retry）；
  2. 写入 payment cancel command outbox（同一事务/可重试）；
  3. append 失败**返回错误/告警**，不得只 log 后继续；
  4. 无 captured/payment_no → 不写命令，保持告警。
- 同一 order/payment 重复调用：`refund_id` 固定，Payment.Refund 幂等。
- 与 `order.cancelled` 已有事件不重复创建组级 S19；仅支付级 command。

### 3.3 `order.refund_requested` 兼容

- T1.5 已移除 TCC 的 `order.refund_requested`；T1.6 consumer：
  - `refund_id=pay:<payment_no>:cancel` → 执行命令；
  - 旧 cancel-after-confirm 缺 `refund_id` 的消息：按 reason/legacy 签名映射到 `pay:<payment_no>:cancel` 执行（过渡兼容）；
  - 其他 TCC/未知事件 → observe-only/告警，不 Refund；
  - 重复/乱序：Payment.Refund 幂等；不产生组级副作用。
- **最终 observe-only/移除**：前置为 T1.5/T1.6 稳定运行、旧队列 drain、BFF/BI 消费者迁移完成、T5.2/T5.3 证据；具体移除日期由 Director 在 release 记录中设定；本 Addendum 不删除 subject，也不得在 drain 前 observe-only。

### 3.4 错误/重试

| 场景 | 错误/行为 |
|---|---|
| CAS 失败（PAID） | `CANCEL_CAS_LOST`；进入组扇出；不写 payment command |
| captured=0/无 payment_no | `PAYMENT_CANCEL_NO_CAPTURE`；本地取消/告警；不写命令 |
| outbox append 失败 | 事务 rollback；返回错误/重试；不得静默 |
| Payment 超时/不可用 | `PAYMENT_CANCEL_REFUND_RETRY`；consumer 退避重试；幂等 |
| Payment 拒绝（不可退/已全额退） | `PAYMENT_NOT_REFUNDABLE`/`ALREADY_REFUNDED`；告警；不重复 |
| 重复事件 | 同 `refund_id` 返回既有；不重复退款 |
| unknown source/refund_id | observe-only/告警；不调用 Payment |

错误使用 apperr 标准 code + `details.reason`；具体 code 由实现映射，但 reason 必须稳定。

---

## 4. 与 T1.4/T1.6 Addendum、C-04 的关系

| 项 | 关系 |
|---|---|
| T1.4 Addendum R-3 | 本 Addendum **替代其 PENDING+captured fail-closed guard**；T1.2 不建 batch；T1.6 实现 CAS+cancel command。 |
| T1.4 组级 AdmitRefund | **不调用**；PENDING+captured 不进入组 application/permit/commitment。 |
| C-04 RefundBatch | **不改变**；本分支单笔支付级 Refund，不使用 RefundBatch。 |
| C-04 release event | **不使用/不改变**；T1.6 的 Inventory 补偿走 `order.cancelled`，不是 refund release。 |
| Payment.Refund | 使用既有 `Refund` 的 `idempotency_key`/`non_effective_capture`；不需要新 proto。 |
| `order.refund_requested` | 保留 subject 作命令载体；T1.5 已移除 TCC 生产者；最终 observe-only 由 release 决定。 |

---

## 4.1 C-04 / C-05

| 项 | 变更 |
|---|---|
| C-04 | `order.refund_requested` payload 增/使用 `refund_id`、`source=payment`、`non_effective_capture=true`、`amount_cent`；registry 保持兼容；**无新 RPC**；`Payment.Refund` 使用既有 `idempotency_key`，**不使用 RefundBatch**。 |
| C-04 release event | 不改变；T1.6 的 Inventory 补偿走 `order.cancelled`，不使用 `inventory.refund_release_requested`。 |
| C-05 | **无迁移**；不新增 payment cancel 表；outbox 事务持久化，Payment 幂等。 |
| 目标 | SPEC-0002 v1.9 AC-46/AC-65/AC-67/AC-68、INV-44/INV-46；ADR Amd1 §4.8。 |

## 5. T1.6-R 任务

| ID | 任务 | Owner | 验收 |
|---|---|---|---|
| R1 | PENDING+captured CAS + order.cancelled + payment command 同 tx | Developer | CAS 成功无组状态；CAS 失败组扇出；outbox 失败 rollback |
| R2 | `refundCancelledAfterConfirm` 持久命令 + Inventory CONFIRMED/HELD | Developer | 命令可重试；Payment 幂等；失败不静默 |
| R3 | `order.refund_requested` consumer command-only/legacy 兼容 | Developer | 只执行 `pay:...:cancel`；其他 observe-only；重复/乱序安全 |
| R4 | Payment.Refund idempotency/non_effective + error mapping | Developer | `pay:<...>:cancel` 幂等；超时重试；No Payment 不写 |
| R5 | QC/Architect + Reviewer | QC/Architect/Reviewer | L2；精确 base/head；测试矩阵 |

---

## 6. 测试矩阵

| 场景 | 期望 |
|---|---|
| PENDING+captured，CAS 成功 | order CANCELLED；order.cancelled；payment command；无组 batch/application/permit |
| PENDING+captured，CAS 失败已 PAID | 无 payment-level；进入组扇出 |
| PENDING+captured，outbox 失败 | 事务 rollback；无 CANCELLED 半状态 |
| cancel-after-confirm | Inventory CONFIRMED 回库；payment command；无组 S19 |
| HELD reservation | Release；payment command |
| CONFIRMED reservation | TccCancelConfirm；不回滚已发货事实 |
| 重复 event / Payment 重试 | 同 refund_id 只退一次；无组副作用 |
| 乱序 order.cancelled / payment.refunded | 保持幂等；不重复 |
| Payment 超时/不可用 | 命令重试；不丢；告警 |
| Payment 拒绝/已退 | ALREADY_REFUNDED/PAYMENT_NOT_REFUNDABLE；不重复 |
| 无 captured/零元 | 不发 payment command；本地取消/零元路径 |
| TCC legacy/未知 event | observe-only/告警；不 Refund |
| legacy cancel-after-confirm 缺 refund_id | 映射到 `pay:<...>:cancel` 执行（过渡） |
| 最终 observe-only | 命令事件仍执行；其他忽略；不删除 subject |
| consumer 重复/并发 | Payment idempotency；无重复退款 |

---

## 7. 状态与实现门禁

- **Status: DRAFT**；T1.6 **不可交实现**；先 QC/人类批准 + C-04 event 字段 + T1.6-R。
- 本 Addendum 不修改 C-04 RefundBatch；C-04 release event 不影响本分支。
- 不写代码/迁移/测试；Tier L2；AC 证据由 Reviewer/T5 绑定。
