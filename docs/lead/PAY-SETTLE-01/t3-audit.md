# T3 缺口审计（PAY-SETTLE-01，2026-11-27）

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01`（在途；SPEC-0002 / ADR-0007） |
| 基线 | `7d103a5`（T2.5/T2.6/T2.7 已提交） |
| 范围 | T3.1–T3.7 的现状、证据、缺口；不改契约/代码 |
| 结论 | T3.1/T3.3 **已由现有代码满足**并补不变量测试；T3.2/T3.4/T3.5/T3.6 需契约级实现，另立 Plan/审批 |

## 1. T3.1 permit 不可恢复 — 已满足

现有写路径只有三类：

- `ensurePermitTx` 新建 permit 为 `NONE`（`settlement.go:281-298`）。
- `AuthorizeShipment` 只允许 `NONE/GRANTED` → `CONSUMED`；`REVOKED` 直接失败（`settlement.go:394-408`）。
- `BlockShipmentForRefund` 仅 `NONE→REVOKED`；`GRANTED→REVOKED`；`CONSUMED` 保持 `CONSUMED`（`settlement.go:429-443`）。
- `AdmitRefund` 仅 `NONE→REVOKED`（`:553-578`、zero 路径 `:829`）；`REVOKED` 走 PRE 但不改状态（`:742-753`）。
- `tcc_settlement.decideTccPermitTx` 仅 `NONE→REVOKED`（`:744-758`）。
- `legacy_resolution_worker` 仅 `NONE→REVOKED`（`:246-252`）。

**没有代码路径把 `REVOKED` 写回 `NONE/GRANTED`。**
测试：`TestRevokedPermitCannotBeRestored`（AdmitRefund + BlockShipmentForRefund + AuthorizeShipment 后仍 REVOKED）。

## 2. T3.3 准入前拒绝/取消不建 commitment — 已满足

`AdmitRefund` 在 Tx1 内、写 admission/commitment/outbox 前完成全部金额/组/partial 校验（permit 缺失时可能先插 NONE，但错误会回滚）：

- 超 captured/group → 直接 error（`refund_admission.go:470-500`）。
- 非 CONSUMED 的 partial unshipped → `partialRefundError`，零 admission/commitment/outbox（`:500-520`）。
- zero short-circuit 不建 commitment（既有 `TestAdmitRefundSystemZeroShortCircuit`）。
- payment-level cancel 不建 group commitment（`payment_cancel.go:61-85`）。
- 准入后失败：`PENDING_REMOTE`/`REVIEW_REQUIRED`/`MANUAL_HOLD` 持久化，不静默释放。

测试：`TestRejectedAdmissionWritesNoCommitment`（over-amount + partial unshipped → 零行、permit 仍 NONE）。

**测试边界**：TCC/legacy worker 的 permit 写路径由 grep 审计覆盖，未在本次测试中执行；CONSUMED + partial 分支未单独执行。

## 3. 缺口

| 任务 | 现状 | 缺口/所需 |
|---|---|---|
| **T3.2** 人工放弃/FAILED_PERMANENT → `RESERVED→RELEASED`（只释放额度） | **未实现**。`order_refund_commitments` 没有 `RELEASED` 写入路径；proto 没有 release RPC；无人工授权/审计/lease。 | 需要契约变更：release 命令（RPC 或 ops 工具）、授权/审计字段、`RELEASED` 状态、CAS/lease、幂等；先 Plan + 人类批准。 |
| **T3.4** proto/存储：行、商品/运费组成、责任、`UNDETERMINED/168h` | **proto/列已存在**：`RefundLine`、`AdmitRefundRequest.lines/responsibility/reason_code/freight_refundable_cent`；`0018` 已有 `goods_cent/freight_cent/composition_known` 和 lines 表。 | **实现未使用**：`AdmitRefund` 不读 `GetLines()/GetResponsibility()/GetFreightRefundableCent()`；没有 line-level INV-24、Σ 校验、responsibility 封闭枚举校验、168h。 |
| **T3.5** `AdmitRefund` 行层 INV-24 + 组层 + Payment 三层；未离仓整组、已离仓部分 | 当前只有 amount/group 级；commitment 是 group header，`lines` 未落。 | 需实现 line composition 校验/落库、组层 Σ、Payment captured/refunded 三层、责任/运费；是 T3 最大块。 |
| **T3.6** AfterSales/缺货责任 → 可退运费唯一 Owner；Order 只校验 | 未接线；`refund_admission` 不读 responsibility。 | 需 AfterSales 产出 responsibility 的契约/调用；Order 只校验，不推断。 |
| **T3.7** 测试：行超限、责任运费、168h、未离仓整组、RELEASED 边界、AC-14/31/32 回归 | T3.1/T3.3 已补；T3.2/T3.4/T3.5/T3.6 无测试。 | 随各实现补齐；AC-60/63 仍需 T3.5/T5。 |

## 4. 建议的下一刀

1. **T3.4+T3.5（行级准入 + responsibility/运费 + Payment 三层）**：最大、最核心；建议先写一份实现 Plan（一页）+ 逐条 AC/INV 证据矩阵，再动代码。涉及行为/契约语义（虽 proto/列已有），需要人类批准 Plan。
2. **T3.2（人工释放）**：独立契约变更；release 命令/授权/审计/lease 需要新 RPC 或 ops 工具，建议单独 Plan。
3. **T3.6（AfterSales 责任）**：依赖 T3.4/T3.5 的 responsibility 字段；在 T3.4/T3.5 后接。

**不建议**在未批准 Plan 的情况下直接改 `AdmitRefund` 的金额/行语义或新增 release RPC。

## 5. 本轮变更（仅测试/文档）

- 新增 `internal/services/order/t3_invariants_integration_test.go`：
  - `TestRevokedPermitCannotBeRestored`
  - `TestRejectedAdmissionWritesNoCommitment`
- `tasks.md`：T3.1/T3.3 标 `[x]`；T3.2/T3.4/T3.5/T3.6 保持 `[ ]`；T3.7 保持 `[ ]`（部分）。
- 无代码/契约/迁移改动；无 AC 状态升级。
