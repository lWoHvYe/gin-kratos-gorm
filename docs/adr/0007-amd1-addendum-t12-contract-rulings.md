# ADR-0007 Amendment 1 Addendum 2 — T1.2/T1.3 Contract Rulings

| 项 | 值 |
|---|---|
| Status | **superseded by ADR-0007 Rev 2.1 (2026-11-25)**——原 accepted（Director 2026-11-24 批准；T1.2 Contract Rulings Addendum）；裁决已并入 Rev 2.1（§0.4/§5.2.1/§5.3/§5.5.1/§3.4.2），本文件仅保留历史/审计证据 |
| Superseded by | [ADR-0007 Rev 2.1](0007-payment-settlement-admission.md)（2026-11-25 合并；merge map 见该文件 §Merge map；原 C-1/C-2 冲突已于 2026-11-25 关闭，见该文件 §18） |
| Approved | **2026-11-24** |
| Base | **ADR-0007 Amendment 1（accepted 2026-11-24，Revision 1.0）**；不修改 Amendment 1 / ADR-0007 Rev 1.0 正文 |
| Revision | **1.0**（Addendum；原 r1 定稿；R-1/R-2/R-3 裁决） |
| WIP | `12858d4`（base `694e6aa`）：T1.2/T1.3 TCC refund batch、permit decision、idempotent resume |
| Target Spec | **SPEC-0002 v1.9（approved 2026-11-24）**；AC-51/AC-65；零元 Addendum 另行 accepted |
| Change ID | `PAY-SETTLE-01` |
| Scope | 仅裁决 T1.2/T1.3 WIP 的 3 个契约偏差；不写代码/迁移/测试 |
| 生效前置 | 本 Addendum **accepted 2026-11-24**；T1.2b 修复完成；Reviewer 按精确 base/head 复审。**不批准 T1.2 实现本身**。 |

> 本文件不改变已批准 ADR 正文；仅记录对 T1.2/T1.3 WIP 的契约裁决与后续任务要求。WIP `12858d4` 不得据此视为实现 PASS。

---

## 1. 裁决表

| # | 偏差 | 裁决 | 依据 | 影响 task | 需 ADR Addendum / Plan 更新 |
|---|---|---|---|---|---|
| **R-1** | Σ 不等时 Dev 按 Plan“零写入”，未写 `refund_batches(PRECHECK_FAILED)` 审计 batch | **以 ADR 为准**：Σ 不等必须写 `PRECHECK_FAILED` 审计 batch；**不写订单/permit/commitment/application/outbox/状态日志**。这是审计事实，不是退款状态变更。 | ADR Amd1 §4.1.2 step 2（`:216`）、§6.1（`:257/470/471`）；AC-65 只禁止改 permit/commitment/订单退款态，未禁止审计 batch | T1.2b 修复；T1.3 保持“PRECHECK_FAILED 不 resume”；T1.7 测试改名/改断言 | **不新增 ADR**；**Plan 必须更新**（T1.2/T1.7 的“零写入”改为“仅审计 batch”）。本 Addendum 记录裁决。 |
| **R-2** | `EXCEPTION` 且 `shipped_at` 的“已离仓”判定：Order DB 无 shipment/`shipped_at`，当前只按组 `SHIPPED/RECEIVED` | **契约缺口，成立**：事实 owner 是 **Fulfillment（S6）**；Order 无该事实。T1.2b 必须在 Tx1 外读 Fulfillment `ListShipments(order_no)`（现有 RPC，`GetShipmentByOrder` 多组会失败），把 `exception_state` 非 `NONE`（当前值 `OPEN`）且 `shipped_at_unix>0` 判为已离仓；不可用/不明确 → fail-closed，不写 batch。T1.4 的 `CancelIfCreated` 必须把 `EXCEPTION+shipped_at` 返回 `ALREADY_SHIPPED`，不得默认 `TRANSIENT_ERROR` 无限重试。 | SPEC-0002 S6/AC-51“已离仓 = SHIPPED/RECEIVED 或 EXCEPTION 且 shipped_at 非空”；`Shipment` 有 `shipped_at_unix/exception_state`（`mall.proto:1021-1040`）；Order `fulfillmentPort` 只有 `CancelIfCreated`（`ports.go:32-34`）；`CancelIfCreated` 只按 `shipments.status` 分支，未读 `exception_state/shipped_at`（`cancel_if_created.go:61-68`）；`GetShipmentByOrder` 多组失败（`fulfillment/service.go:315-317`） | T1.2b 接入读取；T1.4 使用 `ALREADY_SHIPPED`；T1.7 加 EXCEPTION 用例；**T5.4 不是该分支 owner**（见 §4） | **需要本 Addendum**（R-2 已写）；**C-04 无 proto 新增**（复用 `ListShipments`，Order port 增加方法）；**C-05 无迁移**；Plan 更新 T1.2b/T1.4/T1.7 并注明 T5.4 不适用。 |
| **R-3** | `PENDING_PAYMENT+已捕获` 仍保留旧行为（PAID+REFUNDING），与 AC-65/§4.8 的 CAS→CANCELLED + 支付级取消退款不一致 | **语义归 T1.6**，不是 T1.2/T1.3 的语义修复；但 **T1.2b 必须加 fail-closed guard**：`PENDING_PAYMENT && captured>0` 时不得执行 `markOrderRefundingTx` 的 PAID+REFUNDING/批次路径，返回可识别错误、零写入；T1.6 再实现 CAS→CANCELLED + `pay:<payment_no>:cancel`。 | ADR Amd1 §4.8（`:316-323`）；AC-65；Plan T1.6 已存在（`tasks.md:24`）；WIP `markOrderRefundingTx` 当前 `statusPendingPayment` 分支会写 PAID+REFUNDING（`tcc_settlement.go:212-236`），是 P-04 脏数据生产者 | T1.2b guard；T1.6 实现；T1.7 测试；T1.3 不得对 PENDING 建 batch | **不新增 ADR 语义**（§4.8 已定）；**Plan 必须更新 T1.2b/T1.6/T1.7**；本 Addendum 记录裁决。 |

---

## 2. R-1：`PRECHECK_FAILED` 审计 batch（ADR 优先）

### 2.1 裁决

- **ADR Amd1 §4.1.2 step 2 是有效契约**：Σ 不等时写 `refund_batches(state=PRECHECK_FAILED)` 仅作审计 + 告警 + 人工；**不写**订单 `refund_state/status`、permit、commitment、commitment line、`refund_applications`、outbox、status log。
- Plan 的“Σ 不等零写入”是更严格但**与 ADR 冲突**的表述；以 ADR 为准，Plan 需修订。AC-65 的“任何状态变更前阻断”不禁止审计 batch：审计 batch 不是 permit/commitment/订单退款态，也不触发退款。
- `PRECHECK_FAILED` 不进入 `findTccBatchForResume` 的可续跑集合；后续人工修正后创建新 attempt（`attempt_seq+1`），不得 resume 该审计 batch（ADR §6.1）。

### 2.2 T1.2b 修复要求

1. Σ 不等（含 `validateTccSum` 失败）必须在**任何订单/permit 写入之前**插入一条 `refund_batches`：
   - `source='tcc'`、`order_no`、`payment_no`、`attempt_seq`（下一个可用）、`state='PRECHECK_FAILED'`；
   - 记录 `captured_snapshot_cent/refunded_snapshot_cent/sum_group_cap_cent/remaining_captured_cent`、`version=1`；
   - 不写 `refund_applications`/admission/commitment/permit/outbox/status log。
2. 返回原 `FAILED_PRECONDITION` 错误，保留 Σ 数字 details；同时告警/指标。
3. 重复 Σ 失败的审计：允许多个 `PRECHECK_FAILED` attempt（ADR §6.1），不得覆盖/复用旧审计行；不得让审计行阻塞后续成功 attempt。
4. 测试：把 `TestTccCancelSigmaMismatchWritesNothing`（`tcc_compensation_integration_test.go:125-165`）改为“写 1 条 `PRECHECK_FAILED` batch + 其他表/outbox/status 零写入”；保留 permit 未变、订单 version 未变断言。
5. T1.3：`findTccBatchForResume` 对 `PRECHECK_FAILED` 继续视为“不 resume”，但不得把“存在审计行”误判为 active batch（`tcc_settlement.go:279` 已如此；补测试）。

**是否需要 ADR Addendum**：不需要修改 ADR；本 Addendum 记录“ADR 优先、Plan 修订”的裁决。**Plan 更新**：T1.2 和 T1.7 的“零写入”措辞改为“仅审计 batch”。

---

## 3. R-2：`EXCEPTION + shipped_at` 的已离仓判定（契约缺口）

### 3.1 事实所有权

- **Fulfillment** 拥有 S6 运单/签收事实：`shipments.status`、`shipped_at`、`exception_state`；Order 只消费投影，Order DB 没有 `shipment/shipped_at`。
- AC-51 的“已离仓”定义是 `SHIPPED/RECEIVED` **或** `EXCEPTION && shipped_at 非空`；当前 schema 的 AC“EXCEPTION”对应 `shipments.exception_state != 'NONE'`（实际值 `OPEN`），不是 `shipments.status='EXCEPTION'`。仅看 Order 组 `SHIPPED/RECEIVED` **不能覆盖 exception_state+shipped_at**。因此当前 WIP 只按组状态判定是**契约缺口**。
- `GetShipmentByOrder` 在订单有多个 shipment 时直接失败（`fulfillment/service.go:315-317`），不适合多组订单；Order 应读取 `ListShipments(order_no)`（现有 RPC，`mall.proto:1009`）并返回全部 shipment facts。

### 3.2 设计要求

1. **Order 读取**：T1.2b 在 Tx1 外按 `order_no` 调 `ListShipments`（page_size=100，`total<=100`；超过或分页不完整 → fail-closed/人工），按 `fulfillment_group_no` 建 map；只读，不写 Order。
2. **判定**：组已离仓 iff：
   - Order 组 `status=SHIPPED/RECEIVED`，或
   - Fulfillment shipment `status=SHIPPED/RECEIVED`，或
   - Fulfillment `exception_state != 'NONE'`（当前 `OPEN`）且 `shipped_at_unix>0`。
3. **不可用/不明确**：Fulfillment 超时、无 shipment 但 Order 组状态非 `PENDING/ALLOCATED`、多 shipment 冲突、`exception_state` 非 `NONE` 但无 `shipped_at`：fail-closed，**不写 batch/permit/admission/application**，告警+人工；不得猜“未离仓”或“已离仓”。
4. **Tx1**：不得在事务内远程调用；shipment facts 作为 precheck 输入传入 Tx1；Tx1 只使用快照。
5. **T1.4 `CancelIfCreated`**：Fulfillment 对 `exception_state != 'NONE' && shipped_at` 必须返回 `ALREADY_SHIPPED`（不得落 `TRANSIENT_ERROR` 无限重试）；`exception_state` 非 `NONE` 但无 `shipped_at` 返回 `TRANSIENT_ERROR` + 告警/人工，不取消。当前实现只按 `shipments.status` 分支，需补该语义（无 proto 字段）。
6. **C-04**：**无 proto 新增字段/RPC**；Order 的 `fulfillmentPort` 增加 `ListShipments` 方法（existing RPC），Fulfillment `CancelIfCreated` 语义按 §3.2.5 调整（无 proto 字段）。**C-05**：无迁移。
7. **T5.4 边界**：`T5.4` 是“真实迁移测试；AC-60 回填”，**不是 AC-51 EXCEPTION 分支的 owner**。AC-51 的证据落在 **T1.7**（用例）+ **T5.2/T5.3**（竞态/故障注入）；Plan 应更新任务边界，不能把该分支塞进 T5.4。

### 3.3 T1.2b / T1.4 / T1.7 要求

| task | 要求 |
|---|---|
| **T1.2b** | 在 Tx1 前读 `ListShipments`；按 §3.2 判定；不可用 fail-closed；把 `shipped` 标志传入 Tx1；exception_state+shipped 组不得 `NONE→REVOKED`/写 commitment，按 POST 组处理 |
| **T1.4** | `AdmitRefund` 对 `CONSUMED` 组调 `CancelIfCreated`；EXCEPTION+shipped 期望 `ALREADY_SHIPPED`；TRANSIENT/unknown 不取消、不退款，保持 PENDING_REMOTE/人工 |
| **T1.7** | 增加 `exception_state+shipped_at`（已离仓→POST）、`exception_state` 非 NONE 无 `shipped_at`（fail-closed）、Fulfillment 超时（fail-closed）三条用例 |
| **T5.4** | 不适用；不要在该任务中伪造/回填 AC-51 事实 |

**是否需要 ADR Addendum**：**需要**（本节即 Addendum）。**Plan 更新**：新增 T1.2b 读取要求、T1.4 期望、T1.7 用例，并注明 T5.4 不是 owner。

---

## 4. R-3：`PENDING_PAYMENT+已捕获` 归 T1.6

### 4.1 裁决

- **语义归 T1.6**：AC-65/§4.8 的 `PENDING_PAYMENT→CANCELLED` + 支付级 `pay:<payment_no>:cancel` 由 T1.6 实现；T1.2/T1.3 不实现该分支，也不得把旧行为作为最终实现。
- **T1.2b 必须加 fail-closed guard**：当前 `markOrderRefundingTx` 在 `statusPendingPayment && paid>0` 时写 `PAID+REFUNDING`（`tcc_settlement.go:212-236`），并继续建 batch；这会产生“PAID 但无 `settlement_payment_no`”的 P-04 脏数据，且与 AC-65/§4.8 相反。T1.2b 必须在该分支**返回可识别错误（如 `TCC_PENDING_CAPTURED_NOT_IMPLEMENTED`/`FAILED_PRECONDITION`）且零写入**，直到 T1.6 实现 CAS→CANCELLED + 支付级取消退款。
- 如果 T1.6 先于 T1.2b 合入，则 T1.2b 可直接依赖 T1.6 的分支；在 T1.6 未合入前，guard 是唯一允许的行为。
- **不需要新的 ADR 语义**：§4.8 已定义；本 Addendum 只要求 T1.2b 不保留旧行为。

### 4.2 T1.2b / T1.6 / T1.7 要求

| task | 要求 |
|---|---|
| **T1.2b** | `PENDING_PAYMENT && captured>0` 在 Tx1 前/内 guard：不写 PAID/REFUNDING/batch/permit/application；返回明确错误；不得调用 Payment；不得依赖旧 `order.refund_requested` 路径 |
| **T1.6** | 实现 AC-65/§4.8：CAS→CANCELLED + `order.cancelled` + `pay:<payment_no>:cancel`（source=payment、non_effective=true）+ 不创建组级 application/permit 变更；幂等 |
| **T1.7** | 增加 PENDING+captured：T1.6 前 guard 零写入；T1.6 后 CAS→CANCELLED、无组扇出、支付级退款幂等 |

**是否需要 ADR Addendum**：不需要（§4.8 已定）；**Plan 更新**：T1.2b guard、T1.6 依赖、T1.7 用例。

---

## 5. Plan / 任务更新清单

| 任务 | 更新 |
|---|---|
| **T1.2** | 拆出 **T1.2b**：R-1 审计 batch；R-2 `ListShipments` precheck + fail-closed；R-3 PENDING guard。T1.2 的“Σ 不等零写入”改为“仅 `PRECHECK_FAILED` 审计 batch”。 |
| **T1.3** | 保持 `PRECHECK_FAILED` 不 resume；新增“审计 batch 不阻塞新 attempt”测试；PENDING 不得建 batch。 |
| **T1.4** | `CancelIfCreated` 对 exception_state+shipped 期望 `ALREADY_SHIPPED`；unknown/超时不取消、不退款、PENDING_REMOTE/人工。 |
| **T1.6** | 明确为 PENDING+captured 的唯一语义 owner；T1.2b guard 在 T1.6 前生效。 |
| **T1.7** | 增加 R-1/R-2/R-3 三类用例；AC-51 exception_state 用例归此，不归 T5.4。 |
| **T5.4** | 保持“真实迁移/AC-60 回填”；不承担 AC-51 exception_state 事实。 |
| **C-04** | R-2 无 proto 新增；Order `fulfillmentPort` 增 `ListShipments`（existing RPC）；Fulfillment `CancelIfCreated` 语义调整（无 proto 字段）。 |
| **C-05** | R-1/R-2/R-3 均无迁移；不新增表/列/枚举。 |

---

## 6. 状态与声明

- **Status: accepted**（Director 2026-11-24 批准；Revision 1.0 / Addendum）。
- **不批准 T1.2 实现本身**：`12858d4` 仍需 T1.2b 修复 + Reviewer 复审；本 Addendum 的 accepted 仅批准契约裁决，不构成实现 PASS。
- **T1.2b 仍不得合并**：需 Reviewer 按精确 base/head 复审 + 人类另行批准实现；本 Addendum 的 accepted 不构成合并许可。
- 本 Addendum 不修改 ADR-0007 Amendment 1 / Rev 1.0 / SPEC / 代码 / 迁移 / 测试。
- WIP `12858d4` 在 T1.2b 修复与 Reviewer 复审前保持 WIP，不得视为实现 PASS 或 Release 证据。
- R-2 的 Fulfillment `ListShipments`/`CancelIfCreated` 行为变更需 Fulfillment owner 评审；R-3 的 T1.6 是已知范围，不因本 Addendum 提前完成。

## 7. 变更记录

| Revision | 日期 | 变更 | 影响 |
|---|---|---|---|
| 0.1-draft（r1） | 2026-11-24 | 初始 T1.2/T1.3 Contract Rulings：R-1 `PRECHECK_FAILED` 审计、R-2 `EXCEPTION/shipped_at` 契约缺口、R-3 PENDING+captured 归 T1.6；含 T1.2b/Plan 更新要求 | 新建，DRAFT |
| **1.0（accepted）** | 2026-11-24 | Director 批准 T1.2 Contract Rulings Addendum：Status accepted、Approved 2026-11-24、Revision 1.0；Target Spec v1.9；保留 R-1/R-2/R-3；不批准 `12858d4` 实现 | 仍需 T1.2b + Reviewer；不构成实现 PASS |
