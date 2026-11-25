# ADR-0007 Amendment 1 Addendum 1 — 零元售后事实落点

| 项 | 值 |
|---|---|
| Status | **superseded by ADR-0007 Rev 2.3 (2026-11-25)**——原 accepted（Director 2026-11-24 批准；Addendum 1）；条款已并入 Rev 2.3（§5.2.5/§5.7/§8/§13/§15），本文件仅保留历史/审计证据 |
| Superseded by | [ADR-0007 Rev 2.3](0007-payment-settlement-admission.md)（2026-11-25；merge map 见该文件 §Merge map；原 C-1/C-2 冲突已于 2026-11-25 关闭，见该文件 §18） |
| Approved | **2026-11-24** |
| Base | **ADR-0007 Amendment 1（accepted 2026-11-24，Revision 1.0）**；本 Addendum 不修改 Amendment 1 或 ADR-0007 Rev 1.0 正文 |
| Revision | **1.0**（Addendum；原 r3 定稿；QC-17 B-1/B-2/B-3 整改） |
| Target Spec | **SPEC-0002 v1.9（Director approved 2026-11-24；PO 原地升版落库中；AC-67/AC-68、S19、INV-50）**；AC-40 / AC-55 / AC-63 为基线 |
| Change ID | `PAY-SETTLE-01` |
| Design owner | System Architect |
| Integration owner | Director（人类）；deputy: TBD |
| 触发 | Director 授权 T1.2 与 AC-40 零元售后 amendment 并行；T1.2/T1.4 需要明确零元售后的执行账本与完成口径 |
| 生效前置 | ① 本 Addendum **accepted 2026-11-24**；② SPEC-0002 v1.9 落库；③ C-04/C-05 输入评审通过。未满足前不得实现 |
| 禁止 | 不写代码/迁移/测试；不批准 C-04/C-05；不改变正金额退款、Payment 事实与 AC-55 的既有语义 |

---

## 1. 决策摘要

| ID | 决策 |
|---|---|
| **AZ-01** | 零元售后的执行账本放在 **Order**：`refund_applications(source=system_zero, amount=0, state=DONE)` 为**最终**事实；`refund_id=after_sale_no`，`goods_cent=0`、`freight_cent=0`、`non_effective_capture=0`。**`AdmitRefund` 增加零元短路**（不新增 RPC）：`source=system_zero && amount=0 && 订单 pay_cent=0 && 无任何已捕获 intent` 时跳过 Payment 额度/commitment，仍执行 permit/运单/INV-44 准入，同事务返回 DONE；正金额路径不变。**需要回库的退货，回库完成后才调用 `AdmitRefund` 并写 DONE**；REFUND_ONLY/缺货在许可/运单/释放完成后写 DONE。 |
| **AZ-02** | 零元售后**不调用 Payment**，不产生 `refund_no`、不写订单级 `captured/refunded`、不进入 `legacy_payment_resolution`/`EXPIRED_*`/`COMPENSATING`。 |
| **AZ-03** | AfterSales 不新增 `DONE_ZERO` 状态；沿用现有生命周期终态 **`REFUNDED` + `refunded_cent=0`**，`amount_state=CONSUMED`，`refund_no=''`（空字符串，不伪造 Payment 退款号）。 |
| **AZ-04** | 零元售后仍走 `AdmitRefund` 的**零元短路**做组级准入（撤许可/取消未离仓运单/按 INV-44 释放）；短路跳过 Payment 额度与 **0 元 `order_refund_commitments`**；`order_refund_admissions` 照常持久化 `PRE_SHIP_REVOKED/POST_SHIP_ALLOWED`。 |
| **AZ-05** | 这不是新状态机分支：零元短路在 `AdmitRefund` 同一事务直接写 `refund_applications DONE`，**不保留无 worker 的 PENDING 悬挂**；AfterSales 复用 `REQUESTED→APPROVED→REFUNDING→REFUNDED`；`source=system_zero + amount=0 + state=DONE` 是既有状态的值组合，不是新状态。 |
| **AZ-06** | AC-55 的“以 Payment 退款事实为准”仅适用于**有资金退款**；AC-68 已于 **2026-11-24 随零元售后 amendment 批准**，明确完成事实是 Order `refund_applications(DONE, system_zero, 0)` + AfterSales `REFUNDED/0`，不适用 AC-55 的 Payment 优先；实现仍须 SPEC v1.9 落库。 |

---

## 1.1 A-Z1…A-Z8 逐条答复

| ID | 答复 | 与 PO v1.9（approved 2026-11-24）的确认/差异 |
|---|---|---|
| **A-Z1** | **采用 `AdmitRefund` 零元短路（不新增 RPC）**：`source=system_zero && amount=0 && 订单 pay_cent=0 && 无任何已捕获 intent` 时跳过 Payment 额度/commitment，仍执行 permit/运单/INV-44 准入，返回 DONE；正金额路径不变。**0 金额不创建 `order_refund_commitments`**；`order_refund_admissions` 仍写 PRE/POST。 | QC-17 B-1 已按“零元短路”关闭；PO A-Z1 的“复用是否创建 0 commitment”不再成立——本 Addendum 明确不创建。C-04 proto 无新增字段/RPC；**Order 实现需零元短路分支**；C-05 需新增 CHECK 迁移（见 §7）。 |
| **A-Z2** | **Order 推进**：`AdmitRefund` 在同事务直接写 `refund_applications DONE`；AfterSales 只读。需回库的退货，先回库再调用 `AdmitRefund`，DONE 不早于回库；**不保留无 worker 的 PENDING 悬挂**。 | QC-17 B-1 要求零元分支；本 Addendum 确认 Order 唯一写入、同事务 DONE；PO S19 的 `PENDING→DONE` 不再作为零元必经路径（如保留 PENDING 必须有持久重试/告警，不得悬挂）。 |
| **A-Z3** | **`amount_state=CONSUMED` 为最终投影**；不新增 `ZERO_DONE`/`ZERO_REFUNDED`；`REFUNDED+refunded_cent=0` 已是终态。 | 与 PO A-Z3 一致。 |
| **A-Z4** | **`refund_no=''` 合法**；不用合成/伪造 Payment refund_no；展示由 `source=system_zero + amount=0 + status=REFUNDED` 推导 `DONE(zero_amount)`。 | 与 PO A-Z4 一致。 |
| **A-Z5** | **未 `PAID` 不产生 S19**：`PENDING_PAYMENT` 取消只按 AC-46 进入 `CANCELLED`/释放，不创建售后终态；非取消的未 PAID 请求拒绝；有已捕获 intent 时按 AC-40/AC-63 的非零/支付级路径，不走零元。 | 与 PO A-Z5/AC-67 一致；本 Addendum 把“有捕获即不适用零元”写成硬边界。 |
| **A-Z6** | **需要回库的退货：回库完成 → `AdmitRefund`（amount=0）→ `refund_applications DONE` → AfterSales `REFUNDED`**；REFUND_ONLY/缺货无回库，许可/运单/释放完成即可 DONE。不得在回库前标 DONE/REFUNDED。 | 与 PO A-Z6 一致；本 Addendum 进一步明确 **`AdmitRefund` 的回库后调用顺序**，避免 DONE 早于回库。 |
| **A-Z7** | **Order 0 金额 commitment：不创建**；保留 `order_refund_admissions` 与 `refund_applications(DONE)`；AfterSales 的 0 元金额台账可保留为审计（`effective=0/refunded=0/amount_state=CONSUMED`），但不得占用正金额额度。 | 与 PO A-Z7 的“保留 S19/准入结果、不产生正金额承诺”一致；本 Addendum 进一步明确 **0 元 commitment 也不创建**（见 A-Z1）。 |
| **A-Z8** | **幂等键 `refund_id`（after_sale_no）**；Order 的 `AdmitRefund`/application DONE 以 `refund_id` 幂等，重复调用返回同一结果；任务 lease/retry 不得重复释放/回库；AfterSales `REFUNDED/0` 重复读取幂等。 | 与 PO A-Z8 一致；不新增零元专用 lease 表。 |

### 1.2 与 PO v1.9（approved 2026-11-24）的差异与冲突处理

| 主题 | PO v1.9（approved） | 本 Addendum | 结论 |
|---|---|---|---|
| 0 元 commitment | A-Z1 未决：复用 `AdmitRefund` 可能创建 0 金额 commitment，需确认 | **零元短路；明确不创建**；`order_refund_admissions`/`refund_applications` 仍保留 | QC-17 B-1 以零元短路为前提；仍需 QC/PO 批准；若 PO 要求保留 0 元 commitment，必须新增明确状态/规则并回 PO 升版 |
| S19 状态推进 | `PENDING→DONE`，Order 在准入/释放/回库完成后推进 | Order 在 `AdmitRefund` 零元短路内**同事务直接 DONE**；回库退款先回库后调用；**不保留无 worker 的 PENDING 悬挂** | QC-17 §3 要求统一；r3 采用直接 DONE；若保留 PENDING 必须有持久重试/告警，不得悬挂 |
| `amount_state` | 复用 `CONSUMED`，不新增 `ZERO_DONE` | 同 | 一致 |
| `refund_no` | 允许空串 | 同 | 一致 |
| 未 PAID | 取消只按 AC-46；不创建售后终态 | 同；有捕获按 AC-40/AC-63 | 一致 |
| 回库顺序 | 回库完成前不得 DONE | 回库 → `AdmitRefund` → DONE → REFUNDED | **无冲突**，但明确调用顺序 |
| AC-55 | AC-68 已批准 2026-11-24，明确零元例外（仅 `pay_cent=0` 且无已捕获 intent；正金额 Payment 优先不变） | 本 Addendum 与该例外一致；不以实现自行放宽 | **QC-17 B-3 已闭合（AC-68 approved）**；实现仍待 SPEC v1.9 落库 |
| AC-63 | 零元售后不进入 legacy queue | 同；`EXPIRED_ZERO` 仅历史空支付解析 | 一致 |
| `AdmitRefund` 0 元 | 复用现有能力 | **零元短路**（不新增 RPC）：跳过 Payment/0 commitment，仍走 permit/运单/INV-44，返回 DONE；正金额不变 | **QC-17 B-1 已关闭**；C-04 proto 不变；C-05 新增 `0021_zero_application_check.sql` |


### 1.3 QC-17 B-1/B-2/B-3 逐条回应

| QC | 回应 | 落地位置 |
|---|---|---|
| **B-1** `AdmitRefund(amount=0)` 不可复用 `[ARCHITECTURAL_FLAW]` | **已关闭**：不新增 RPC，采用 `AdmitRefund` **零元短路**——仅当 `source=system_zero && amount=0 && 订单 pay_cent=0 && 无任何已捕获 intent` 时，跳过 Payment 权威额度与 commitment，仍执行 permit/运单/INV-44 准入，同事务写 `refund_applications(DONE)` 并返回 `DONE`；正金额路径不变。C-04 proto 不变；Order 实现需增加该分支。 | §1.1 A-Z1、AZ-01、§3.1、§6 |
| **B-2** C-05 “零影响”不成立 | **已修正**：C-05 需新增 **1 个迁移** `migrations/order/0021_zero_application_check.sql`（建议名；下一个可用号），只加 `refund_applications` 的 `system_zero` 0 金额 CHECK；**不得修改已合入的 `0017`**；无新表/新列/新枚举。 | §7 |
| **B-3** AC-55/AC-68 例外 | **已闭合**：AC-68 于 2026-11-24 随零元售后 amendment 批准，明确零元售后（`pay_cent=0` 且无任何已捕获 intent）的完成事实是 Order `refund_applications(source=system_zero, amount=0, state=DONE)` + AfterSales `REFUNDED`/`refunded_cent=0`/`amount_state=CONSUMED`/`refund_no=''`，不适用 AC-55 的 Payment 事实优先；正金额退款仍按 AC-55。实现仍待 SPEC v1.9 落库。 | §5、§9 |
| **S19 PENDING 悬挂** | **已统一**：零元短路在 `AdmitRefund` 同事务直接 `DONE`，不保留无 worker 的 PENDING；若未来保留 PENDING，必须有持久重试/告警，不得悬挂。 | AZ-05、A-Z2、§3.1、§4 |

---

## 2. 事实与所有权

| 事实 | 唯一所有者 | 落点 | 禁止 |
|---|---|---|---|
| 零元售后申请/执行 | Order（准入与执行账本）；AfterSales（业务申请/售后生命周期） | Order `refund_applications(source=system_zero, amount=0, state=DONE)`；AfterSales `after_sales.status=REFUNDED` | 不得写订单 `captured/refunded`；不得创建 Payment refund |
| 零元准入结果 | Order | `order_refund_admissions(refund_id=after_sale_no, result=PRE/POST)` | 不得用 AfterSales 本地状态替代准入结果 |
| 零元额度 | Order | 无 commitment（amount=0）；不占订单/组可退额度 | 不得创建 0 元 commitment 后再用 `RELEASED/SETTLED` 表示完成 |
| 售后业务状态 | AfterSales | `status=REFUNDED`、`refunded_cent=0`、`amount_state=CONSUMED`、`refund_no=''` | 不得新增 `DONE_ZERO` 或伪造 Payment `refund_no` |
| 库存处置 | Inventory/AfterSales | 原 `HELD`/`CONFIRMED` 按 INV-44 释放/回库；不得因零元跳过 | 不得因无 Payment 而不释放/不回库 |

---

## 3. 执行顺序（T1.2 / T1.4）

### 3.1 T1.2（Order 准入）

**零元短路（QC-17 B-1，强制）**：`AdmitRefund` 在 `source=system_zero && amount=0 && 订单 pay_cent=0 && 无任何已捕获 intent` 时，**跳过 Payment 权威额度与 commitment**，但仍执行 permit/运单/INV-44 准入，并在同一事务返回 `DONE`；正金额路径不变，不新增 RPC。

1. AfterSales 调用 `AdmitRefund`，参数：`refund_id=after_sale_no`、`source=system_zero`、`amount_cent=0`、`group_no`、可选 `lines`（零元商品的 `goods_cent=0`）。
2. Order 校验：订单 `pay_cent=0` 且无任何已捕获 intent（有捕获则**不是零元售后**，按 AC-40/AC-63 走支付级/人工队列）；`amount_cent=0`。
3. 走与正金额相同的组级准入：`NONE→REVOKED` 或 `CONSUMED+CREATED` 的 `CancelIfCreated`；已离仓组按 `POST_SHIP_ALLOWED`，不取消运单。
4. 写 `order_refund_admissions`；**不创建 commitment**（0 元无在途额度）。
5. 写/复用 `refund_applications(refund_id=after_sale_no, source=system_zero, amount=0, goods=0, freight=0, state=DONE, admission_result=PRE/POST, non_effective_capture=0)`；返回 `state=DONE`。
6. 不调用 Payment；不写订单级 `captured/refunded`；不进入 `legacy_payment_resolution`。

### 3.2 T1.4（AfterSales 完成投影）

1. **REFUND_ONLY / 缺货（无回库）**：直接调用 `AdmitRefund(source=system_zero, amount=0)`；Order 在同一事务完成许可撤销/运单取消/INV-44 释放并写 `refund_applications(DONE)`。
2. **RETURN_REFUND（需要回库）**：先按既有 `RETURNING → RETURN_RECEIVED`/回库完成；**回库完成后才调用** `AdmitRefund(source=system_zero, amount=0)`，Order 写 `refund_applications(DONE)`；不得在回库前调用/标 DONE。
3. 准入成功（S19 DONE）后在本地事务：
   - `after_sales.status=REFUNDED`；
   - `after_sales.refunded_cent=0`；
   - `after_sales.amount_state=CONSUMED`；
   - `after_sales.refund_no=''`（不伪造 Payment refund no；业务展示用 `after_sale_no`）；
   - 记录状态日志/审计；不得调用 Payment、不得生成 Payment refunded。
4. 不得发送 Payment 退款请求；`aftersale.refund_requested` 如仍需用于 Fulfillment 取消/回库，只能由 Fulfillment 消费者处理，**不得触发 Payment**。
5. 重复调用幂等：`status=REFUNDED` + `refunded_cent=0` 直接返回；Order `refund_applications` 已 DONE 直接复用，不重复准入/释放/回库。

---

## 4. 为什么不是新状态机分支

- `refund_applications` 现有状态 `PENDING/PROCESSING/DONE/FAILED_PERMANENT/CANCELLED/REJECTED` 已能表达零元：零元准入在同一事务直接 `DONE`，不进入 `PROCESSING`（无 Payment 执行阶段）。
- AfterSales 现有生命周期 `REQUESTED→APPROVED→REFUNDING→REFUNDED` 已能表达零元：`REFUNDING→REFUNDED` 是既有迁移，`refunded_cent=0` 是金额值，不是新状态。
- `source=system_zero + amount=0 + state=DONE` 是**值组合**，不是新状态；新增 `DONE_ZERO` 会制造第二套完成语义，且与 AC-40 的 `DONE(zero_amount)` 重复。
- 不需要 `COMPENSATING`/`EXPIRED_*`：这些是历史 `PAID` 空支付解析分支；零元售后有明确 `pay_cent=0`，不进入 legacy queue。

```text
Order refund_app:  PENDING --零元准入--> DONE（source=system_zero, amount=0）
AfterSales:       REQUESTED -> APPROVED -> REFUNDING -> REFUNDED（refunded_cent=0）
                  REFUNDING -> REFUNDED 不调用 Payment；失败不回滚已释放事实
```

---

## 5. AC-40 / AC-55 / AC-63 / AC-67 / AC-68 一致性

| AC | 本 Addendum 的处理 |
|---|---|
| **AC-40** | 零元取消/缺货/售后只释放/回库，不调用 Payment；`refund_applications` 0 元 `DONE`；售后额度 0。本 Addendum 的 `source=system_zero, amount=0, DONE` 即该“退款申请 0 元标记 DONE(zero_amount)”。 |
| **AC-67** | 零元售后/取消/缺货不调用 Payment；S19 执行事实；回库完成前不 DONE；与 PO v1.9（approved）一致；以 **`AdmitRefund` 零元短路**为执行前提；0 元 commitment 不创建（A-Z1）。 |
| **AC-68** | `REFUNDED` + `effective/refunded=0` + `amount_state=CONSUMED` + `refund_no=''`；`DONE(zero_amount)` 由 `source=system_zero + amount=0 + status=REFUNDED` 推导；与 PO v1.9（approved）一致。 |
| **AC-55** | AC-55 的 Payment 完成口径适用于有资金退款；零元无 Payment，按 AC-40 的例外以 Order `refund_applications(DONE, system_zero, 0)` + AfterSales `REFUNDED/0` 作为完成事实。**不得**把零元售后显示为“Payment 已退款”，也不得要求 Payment refund_no。 |
| **AC-63** | 零元售后不进入 `legacy_payment_resolution`；`EXPIRED_ZERO` 仅用于历史 `PAID` 且无 capture 的解析分支，与本 Addendum 的零元售后无关。零元售后不得产生 `EXPIRED_REFUND/EXPIRED_FINANCE/COMPENSATING`。 |
| **AC-53 / INV-37** | 零元售后不写订单级 `captured/refunded`；不得用 Payment 非生效捕获退款替代。 |
| **AC-50 / INV-24** | 零元仍走许可/运单准入；amount=0 不创建 commitment，不占额度；不作为行级/组级正金额退款的先例。 |

> **AC-55 边界（已由 AC-68 确认）**：AC-55 的“以 Payment 退款事实为准”按“有资金退款”适用；零元售后是 AC-40 的专门规则，以 Order `refund_applications(DONE, system_zero, 0)` + AfterSales `REFUNDED/0` 为完成事实，不调用 Payment。**PO v1.9 AC-68 已于 2026-11-24 批准并明确该例外**；SPEC v1.9 落库前不得实现零元售后；正金额退款仍按 AC-55。

---

## 6. C-04 影响

| 项 | 影响 |
|---|---|
| `AdmitRefund` | **proto 无新增字段/RPC**：`source` 已存在，传 `system_zero`；`amount_cent=0`；`AdmitRefundReply.state` 返回 `DONE`。**Order 实现需增加零元短路分支**（QC-17 B-1），正金额路径不变。 |
| `RefundLine`/责任/运费 | 零元可不传 lines（或传 `goods_cent=0` 的零行）；不要求 `freight_refundable_cent`。 |
| Payment API | 不新增；零元不调用 Payment。 |
| AfterSales API | 不新增 `DONE_ZERO`；沿用 `AfterSale.status=REFUNDED`，`refunded_cent=0`，`refund_no=''`。 |
| 事件 | 不新增零元专用事件；如需 Fulfillment 取消/回库，沿用既有 `aftersale.refund_requested` 的 Fulfillment 消费，不得触发 Payment。 |

---

## 7. C-05 增量（1 个 CHECK 迁移）

| 对象 | 影响 |
|---|---|
| `refund_applications` | 无新列/新状态；**C-05 新增迁移 `migrations/order/0021_zero_application_check.sql`** 加 CHECK：`source='system_zero'` 时 `amount_cent=0 AND goods_cent=0 AND freight_cent=0 AND non_effective_capture=0`；`state=DONE` 使用既有枚举。**不得修改已合入的 `0017`**。 |
| `order_refund_commitments` | 零元**不插入** commitment；无 schema 变更。 |
| `order_refund_admissions` | 不改；零元仍写 `PRE_SHIP_REVOKED/POST_SHIP_ALLOWED`，`amount_cent=0`。 |
| `after_sales` | 无新列/新枚举；`status=REFUNDED`、`refunded_cent=0`、`amount_state=CONSUMED`、`refund_no=''` 均合法。 |
| 迁移 | **C-05 新增 1 个迁移（建议 `0021_zero_application_check.sql`，下一个可用号）**；只加 `refund_applications` 的 system_zero CHECK；**不改 `0017`**；无新表/新列/新枚举/无 down migration。 |

---

## 8. T1.2 / T1.4 接口边界

| 任务 | 必须做 | 不得做 |
|---|---|---|
| **T1.2（Order）** | 识别 `source=system_zero + amount=0 + pay_cent=0`；校验无已捕获 intent；走 permit/运单准入；写 `order_refund_admissions` + `refund_applications(DONE)`；返回 `state=DONE` | 不得创建 0 元 commitment；不得调用 Payment；不得写订单 captured/refunded；不得进入 legacy queue |
| **T1.4（AfterSales）** | REFUND_ONLY/缺货：调用 `AdmitRefund`（Order 完成许可/释放）→ 置 `REFUNDED/0/CONSUMED/refund_no=''`；RETURN_REFUND：先完成回库 → 再调用 `AdmitRefund` → 置终态；幂等 | 不得调用 `Payment.RefundOrder`/`processRefundTask`；不得伪造 refund_no；不得把 `DONE_ZERO` 作为新状态；不得因零元跳过库存处置；不得在回库前调 `AdmitRefund` |

---

## 9. 假设与不确定性（强制）

1. **SPEC 已批准**：本 Addendum 以 SPEC-0002 v1.8（approved 2026-11-24）AC-40 为业务依据；若后续 SPEC 改变零元售后语义，本 Addendum 需重映射。
2. **AfterSales 零元创建路径**：现有普通用户售后对“金额 0”有校验；零元订单的售后必须由服务端识别 `pay_cent=0` 后走零元路径，不能用“用户手填 0 元”绕过正金额订单的售后额度。该识别属于 T1.4 实现，不改 Spec。
3. **`refund_no` 为空**：UI/对账不得把空 `refund_no` 当作“未退款”；以 `status=REFUNDED` + `refunded_cent=0` + Order `refund_applications(DONE)` 为准。
4. **0 元 commitment 例外**：本 Addendum 明确 0 元不创建 commitment；S15 的“每个 refund_id 一条承诺”按“有金额占用”解释，0 元无额度可占。若 QC/PO 认为必须保留 0 元承诺，应新增明确状态/规则，不得由实现自行选择。
5. **不新增基础设施**：不引入新表/新队列；复用 Order/AfterSales 现有结构。
6. **审批**：本 Addendum **accepted 2026-11-24**；实现前需 SPEC-0002 v1.9 落库 + C-04/C-05 评审通过。

---

## 10. 批准

- **Status: accepted**（Director 2026-11-24 批准；Revision 1.0 / Addendum 1）。
- **生效前置**：SPEC-0002 v1.9 落库 + C-04/C-05 评审通过。
- 实现前：不得写代码/迁移/测试，不得作为实现依据。
- 后续：C-04/C-05 按本 Addendum 与 ADR-0007 Amendment 1 实现；Reviewer 按精确 base/head 复审。

## 11. 变更记录

| Revision | 日期 | 变更 | 影响 |
|---|---|---|---|
| 0.1-draft（r1） | 2026-11-24 | 初始零元售后 Addendum：AZ-01…AZ-06、T1.2/T1.4 边界、C-04/C-05 影响 | 新建，DRAFT |
| **0.2-draft（r2）** | 2026-11-24 | 对照 PO v1.9-draft AC-67/68/S19/INV-50，逐条回答 A-Z1…A-Z8；确认 `CONSUMED`/空 `refund_no`/未 PAID 边界/回库顺序/幂等；记录 0 元 commitment 不创建为与 PO A-Z1 的决策差异；Target 升 v1.9-draft | Status 仍 DRAFT；未改 Spec/代码/测试 |
| **0.3-draft（r3）** | 2026-11-24 | 按 QC-17 B-1/B-2/B-3 整改：采用 `AdmitRefund` **零元短路**（不新增 RPC，跳过 Payment/0 commitment，仍走 permit/运单/INV-44，同事务 DONE）；C-05 新增 `migrations/order/0021_zero_application_check.sql`（不改 0017）；要求 PO 将 AC-68 改为无歧义零元例外；删除无 worker 的 PENDING 悬挂 | Status 仍 DRAFT；未改 Spec/代码/测试 |
| **1.0（accepted）** | 2026-11-24 | Director 批准零元售后 Addendum：Status accepted、Approved 2026-11-24、Revision 1.0；删除 current draft/final candidate 状态；Target Spec v1.9（approved 2026-11-24） | 生效前置：SPEC v1.9 落库 + C-04/C-05 评审；保留 r1–r3 变更记录 |
