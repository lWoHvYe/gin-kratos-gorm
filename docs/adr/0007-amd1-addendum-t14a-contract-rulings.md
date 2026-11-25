# ADR-0007 Amendment 1 Addendum 3 — T1.4a Contract Rulings

| 项 | 值 |
|---|---|
| Status | **superseded by ADR-0007 Rev 2.1 (2026-11-25)**——原 accepted（Director 2026-11-24 批准；T1.4a Contract Rulings Addendum）；裁决已并入 Rev 2.1（§0.5/§5.2.2/§5.5.1/§5.7/§5.8/§9.2），本文件仅保留历史/审计证据 |
| Superseded by | [ADR-0007 Rev 2.1](0007-payment-settlement-admission.md)（2026-11-25 合并；merge map 见该文件 §Merge map；原 C-1/C-2 冲突已于 2026-11-25 关闭，见该文件 §18） |
| Approved | **2026-11-24** |
| Revision | **1.0**（Addendum；原 r1 定稿；R-A/R-B/R-C 裁决） |
| Base | ADR-0007 Amendment 1（accepted 1.0）+ T1.2 Contract Rulings Addendum（accepted 1.0）+ 零元 Addendum（accepted 1.0） |
| WIP | `6be5c5a`：T1.4a TCC 逐组准入、`CancelIfCreated` R-2 语义、零元短路、`0021` |
| Target Spec | **SPEC-0002 v1.9（approved 2026-11-24）**；INV-44、AC-50/AC-51/AC-65/AC-67/AC-68 |
| Change ID | `PAY-SETTLE-01` |
| Scope | 仅裁决 T1.4a 暴露的 3 个契约/所有权缺口；不写代码/迁移/测试 |
| 生效前置 | 本 Addendum **accepted 2026-11-24**；T1.4b/T3 按 §4 更新；Reviewer 按精确 base/head 复审。**不批准 T1.4a 实现本身**。 |

---

## 1. 裁决表

| # | 缺口 | 裁决 | 依据 | Owner / task | Addendum / Plan |
|---|---|---|---|---|---|
| **R-A** | **INV-44 释放/回库无 owner**：`AdmitRefund`（含零元短路）只做 permit/运单准入；Fulfillment 只取消运单；Inventory 没有 `order.refund_admitted` 消费者；AfterSales 只有退货 restock 任务 | **成立，必须补 owner**。事实 owner/执行者 = **Inventory**；**Order** 负责在准入事务后产生**持久 release 请求**（outbox/lease task），不得在 `AdmitRefund` 事务内直接远程释放；**AfterSales** 只负责退货受理/回库授权，Inventory 执行 restock；**Fulfillment 不负责库存释放**。零元与正金额**同一 owner/同一执行路径**。 | SPEC INV-44（`:128`）、AC-50（`:230`）、零元 AZ-04/INV-50；Inventory 已有 `Release`/`TccCancelConfirm`，但无 `order.refund_admitted` 消费者；`fulfillment/admission_consumer.go` 只 `CancelIfCreated`；T1.4a selfcheck 明示缺口（`:44`、`:167`、`:174`） | **T1.4b**：Order 在准入完成后写持久 release 请求（含 `refund_id/order_no/group_no/items/warehouse_id`）；**T3.3/T3.5**：Inventory release/restock 执行与重试/告警；**T4.1**：Inventory 必需依赖 fail-fast；**T5.2/T5.3** 证据 | **新增本 Addendum**；**Plan 增加 T1.4b owner + T3 子任务**；C-04 需要事件/命令字段（`group_no`/`warehouse_id`/items），C-05 无迁移 |
| **R-B** | **POST 组 commitment 时机**：T1.2b Tx1 对已离仓组不写 commitment；T1.4a `recordTccPostAdmission` 在准入阶段写 `RESERVED` + POST admission | **符合额度意图，需契约澄清**。Tx1 只写 application；POST 组的 `RESERVED` commitment 应在 **T1.4a 准入阶段**与 POST admission 同一事务写入；T1.4b 打款前必须已 RESERVED。不得在 Tx1 写、不得在打款后补。 | ADR §4.1.2 step 3（`:227`）与 Addendum 2 R-2 §3.3（T1.2b 范围）；AC-65/INV-48 额度串行化；T1.4a `tcc_admission.go:200-204` 已有实现；selfcheck `:41`/`:175` | **T1.4a** 保持；**T1.4b** 只从 `ADMITTED` + RESERVED 进入 PAYING；**T1.2b** 明确不写；T3/T5 补并发额度测试 | **需要本 Addendum** 明确“POST commitment 在 admission 写”；Plan 更新 T1.4b/T1.2b 边界 |
| **R-C** | **零元 CONSUMED 先 `CancelIfCreated` 再事务**：失败无 S19，AfterSales 重试 | **有条件接受**：零元无资金，`CancelIfCreated` 幂等；允许“先取消、后单事务写 admission+S19 DONE”。但**前提**是 T1.4b 在调用前已有**持久 AfterSales refund task**（lease/retry）；若没有持久任务，必须先持久意图再远程取消。不得留下无 worker 的 PENDING；不得在取消失败时写 S19。 | 零元 Addendum AZ-05/§3.1；AC-67/AC-68；T1.4a `refund_admission.go:335-344`、`:393-397`；selfcheck `:43` | **T1.4b**：AfterSales 持久 task + 重试 + 终态投影；**T1.4a** 保持现状；T1.7/T5 补崩溃/重试用例 | **需要本 Addendum** 记录条件；Plan 更新 T1.4b 前置 |

---

## 2. R-A：INV-44 owner 与任务

### 2.1 责任边界

| 事实/动作 | 唯一 owner | 说明 |
|---|---|---|
| 未离仓组预留释放（`HELD`/`CONFIRMED`） | **Inventory** | `Release` / `TccCancelConfirm` 的既有执行者；释放失败持久重试/告警 |
| 触发释放 | **Order** | `AdmitRefund`/TCC admission 只返回准入结果；由 Order 在准入事务/outbox 产生 durable release 请求，禁止在 DB 事务内远程调用 Inventory |
| 已离仓退货回库 | **AfterSales**（受理/授权）；**Inventory**（执行 Restock） | `after_sale_restock_tasks` 已有 worker；不得由 Fulfillment 直接改库存 |
| 运单取消 | **Fulfillment** | 只 `CancelIfCreated`，不释放库存 |
| 零元 | **同 R-A** | 不新增零元专用 release owner；同一 Inventory 执行、同一 Order/AfterSales 触发 |

### 2.2 任务落点

- **T1.4b**：Order 在 TCC/零元准入完成后写持久 release 请求（`refund_id/order_no/group_no/items/warehouse_id`），并保证重试；不得在 `AdmitRefund` 内直接调 Inventory。
- **T3.3/T3.5**：Inventory release/restock 执行、lease/重试/告警、组级幂等；AC-50/INV-44 测试。
- **T4.1**：Inventory 为必需依赖，构造期 fail-fast；不得 nil 跳过。
- **T5.4 不适用**：T5.4 是迁移/AC-60 回填；INV-44 证据归 T5.2/T5.3/T5.5。
- **C-04**：新增/扩展 release 事件（至少 `refund_id/order_no/group_no/items/warehouse_id`）；当前 `inventory.compensation_requested` 只有 `order_no/items`，不足以按组释放；**C-05 无迁移**。

### 2.3 零元与正金额

- **同一 owner、同一 Inventory 执行路径**；零元只跳过 Payment，不跳过 INV-44 释放/回库。
- 零元 `REFUND_ONLY` 由 Order/T1.4b 触发未离仓释放；零元 `RETURN_REFUND` 由 AfterSales restock task 触发回库；不得把零元当作“无库存处置”。

---

## 3. R-B：POST 组 commitment 时机

### 3.1 结论

- **POST 组在 T1.4a 准入阶段写 `order_refund_admissions(POST)` + `order_refund_commitments(RESERVED)` 是符合 AC-65/INV-48 额度意图的**：打款前必须先占用组剩余额度，防止并发售后重复占用。
- Tx1（T1.2b）仍只写 application，不写 commitment；这不矛盾：Tx1 只冻结/准入，POST 组的额度在 admission 阶段落账。
- **T1.4b** 只能在 batch `ADMITTED` 且 POST 组 commitment `RESERVED` 存在时进入 PAYING；不得在打款后补 commitment，也不得重复创建。
- 该行为需要写入契约，因为 ADR §4.1.2 step 3 的“已离仓→仅写 application”只描述 Tx1；本 Addendum 明确 admission 阶段写 commitment。
- 测试要求：POST 组 admission + commitment 同事务；重复/并发不产生第二条；T1.2b Tx1-only 测试仍断言无 commitment。

---

## 4. R-C：零元先取消后事务

- **接受条件**：零元 `CONSUMED` 组允许先调 `Fulfillment.CancelIfCreated`，成功后再开事务写 `order_refund_admissions` + `refund_applications(source=system_zero, amount=0, state=DONE)`；取消幂等，事务失败不写 S19，重试幂等。
- **必须前置**：T1.4b 在调用 `AdmitRefund` 前已持久化 AfterSales refund task（或等价 lease/retry intent），保证进程崩溃/事务失败后仍可重试；仅同步调用、无持久任务时，必须先持久意图再远程取消。
- **禁止**：取消失败写 S19；无 worker 的 PENDING；在事务内远程调用；把取消成功当作退款完成而不写 S19。
- **C-04/C-05**：无新增 proto/迁移；T1.4b 使用既有 AfterSales task。

---

## 5. Plan / 任务更新要求

| 任务 | 更新 |
|---|---|
| **T1.4b** | 增加 INV-44 持久 release 请求（R-A）；POST commitment 已由 T1.4a 写，不得重复（R-B）；AfterSales 零元持久 task 前置（R-C）。 |
| **T3.3/T3.5** | Inventory release/restock 执行、lease/retry、组级幂等与 AC-50/INV-44 测试。 |
| **T4.1** | Inventory 必需依赖 fail-fast；不因 nil 跳过 release。 |
| **T5.4** | 明确不承担 INV-44/AC-51 事实；证据归 T5.2/T5.3/T5.5。 |
| **C-04** | release 事件字段扩展（不新增 proto RPC）；零元/正金额同一事件。 |
| **C-05** | 无迁移。 |

---

## 6. 状态与声明

- **Status: accepted**（Director 2026-11-24 批准；Revision 1.0 / Addendum）。
- **不批准 T1.4a 实现本身**：`6be5c5a` 仍需 T1.4b/T3 更新 + Reviewer 复审；本 Addendum accepted 仅批准契约裁决。
- 不修改 ADR-0007 Amendment 1/Rev 1.0、T1.2 Rulings Addendum、零元 Addendum、SPEC、代码/迁移/测试。
- T1.4a `6be5c5a` 保持 WIP；T1.4b/T3 按本 Addendum 补 owner 与测试后，Reviewer 按精确 base/head 复审。
- 2026-11-24：R-A 是**契约缺口**；R-B 是**契约澄清**；R-C 是**有条件接受**。

## 7. 变更记录

| Revision | 日期 | 变更 | 影响 |
|---|---|---|---|
| 0.1-draft（r1） | 2026-11-24 | 初始 T1.4a Contract Rulings：R-A INV-44 owner、R-B POST commitment 时机、R-C 零元先取消条件；含 T1.4b/T3/T4 任务要求 | 新建，DRAFT |
| **1.0（accepted）** | 2026-11-24 | Director 批准 T1.4a Contract Rulings Addendum：Status accepted、Approved 2026-11-24、Revision 1.0；Target Spec v1.9；保留 R-A/R-B/R-C；不批准 `6be5c5a` 实现 | 仍需 T1.4b/T3 + Reviewer；不构成实现 PASS |
