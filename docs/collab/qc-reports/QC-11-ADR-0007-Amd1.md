# QC-11 — ADR-0007 Amendment 1 预审草案复核

| 项 | 值 |
|---|---|
| review_scope | `docs/adr/0007-payment-settlement-admission-amd1.md`（Rev 0.1-draft，base ADR-0007 Rev 1.0）；对照 `docs/po/drafts/0002-v1.8-amendment.md` r3、`docs/adr/0007-payment-settlement-admission.md` Rev 1.0、Plan `docs/lead/PAY-SETTLE-01/{plan,tasks,evidence-matrix}.md` 的 D-1…D-5、C-03/C-05 |
| 审查类型 | ADR Amendment 预审（QC）；不替代 Human `ADR_APPROVED`，不构成实现依据 |
| 结论 | **`CHANGES REQUIRED`** |

> 本报告只审 Amendment 文本与 SPEC r3/Rev 1.0 的一致性；不修改文件、不跑测试。

## 1. BLOCKER（3 条）

### B-1 `EXPIRED_REFUND` 缺少 AC-63 的“撤销/取消未离仓组许可/运单 + INV-44 释放”

- 位置：`docs/adr/0007-payment-settlement-admission-amd1.md:289-294`（§4.5）、`:320-332`（§5.1）。
- 问题：ADR 的 `EXPIRED_REFUND` 只写“为每笔 captured intent 写 `refund_applications(source=legacy_expired, group_no NULL, non_effective_capture=true)` 并调 Payment.Refund”；§5.1 只写“`EXPIRED_REFUND`：禁发，不再新建组退款”。而 SPEC r3 AC-63（`docs/po/drafts/0002-v1.8-amendment.md:145`）要求“撤销/取消**所有**未离仓组的许可/运单并按 INV-44 释放预留”，E25（`:296`）也要求“仅 `EXPIRED_REFUND` 且无离仓组时按 INV-44 释放”。ADR 没有写 permit `REVOKED`/运单 `CANCELLED`/释放动作，`禁发` 只阻断新退款，不阻断已存在的 `NONE` 许可被 Ship 消费 → 存在“已退款又发货”窗口。
- 建议：在 §4.5/§5.1 的 `EXPIRED_REFUND` 终态事务内显式写“未离仓组 permit `NONE→REVOKED`、`CREATED` 运单 `CANCELLED`、按 INV-44 释放；已离仓组保持 `CONSUMED` 只退款”；并复用 `AdmitRefund` 或 `BlockShipmentForRefund` 的同一裁决点；补 AC-63/E25 的测试断言。

### B-2 ADR 自身 schema 与正文不一致（ON 记录/移除日期、支付级标记）

- 位置：`amd1.md:181-199`（§3.4）vs `:556-593`（§6.6）；`:289-294`（§4.5）vs `:442-476`（§6.2）。
- 问题：
  1. §3.4 要求“同一批准行写入 `order.paid_admission_retired_at = ON 时刻`、必须填写 `order.paid_subject_removal_date`”，但 §6.6 的 `settlement_switch_approvals` 没有这两个列；ON 记录/移除日期无落点。
  2. §4.5 写 `refund_applications(... non_effective_capture=true)`，但 §6.2 的 `refund_applications` 没有 `non_effective_capture` 列；支付级/历史退款标记无法持久化，`INV-37` 的“非生效捕获不计入订单 refunded”无法在 Order 账本表达。
- 建议：在 §6.6 的 `settlement_switch_approvals` 增 `order_paid_admission_retired_at`、`order_paid_subject_removal_date`（并说明 ON 时必须非空）；在 §6.2 增 `non_effective_capture TINYINT(1) NOT NULL DEFAULT 0`，并补 `source=legacy_expired/payment` 的约束；同步 `§4.5/§3.4` 与 schema。

### B-3 契约边界 #2：AC-63 的 `pay_cent=0 → zero` 与 AC-40 的“零元单正金额捕获须退款”冲突

- 位置：`docs/po/drafts/0002-v1.8-amendment.md:135`（AC-63）vs `:133`（AC-40）；`amd1.md:321-332`（§5.1）。
- 问题：AC-63 写“IF 订单 `pay_cent=0`，THEN 解析为 `zero:<order_no>`，不调用 Payment”，字面不区分“有无已捕获 intent”；AC-40 要求“零元单收到任何正金额捕获 → 全额退款”。ADR §5.1 采用“无任何已捕获 intent 才 `zero`，有捕获按 D-22 有捕获分支”，这是**安全解释**，但 SPEC 文本仍可被实现按字面 `zero`，吞掉已捕获资金。
- 建议：由 PO 在 v1.8 中把 AC-63 改为“`pay_cent=0` **且无任何已捕获 intent** → `zero:<order_no>`；有捕获 intent 时按 AC-40 全额退款/按 D-22 有捕获分支”，并在 AC-63/AC-40 交叉引用；ADR §5.1 已按此实现，无需改。

## 2. Architect 提出的 4 个边界判定

| # | 边界 | 判定 | 理由/要求 |
|---|---|---|---|
| 1 | AC-60“不得永久阻断” vs `EXPIRED_FINANCE` 终态禁发/禁退 | **可由 QC 接受（需 PO 确认）** | ADR §11.7 按“不得无限期 `OPEN`”解释，`EXPIRED_FINANCE` 是财务终态+7 天关闭；AC-60 的“不得永久阻断”应指“不得无终态地挂起”，不是“必须最终发货”。PO 在 v1.8 批准记录中确认该解释即可，不需要改 ADR |
| 2 | 零元 + 已捕获 | **契约缺陷（B-3）** | AC-63 的字面 `pay_cent=0 → zero` 与 AC-40 冲突；ADR 的安全解释正确，但须 PO 改 SPEC 文本 |
| 3 | `refund_applications` 所有权（Order 执行账本 vs AfterSales 业务申请） | **可由 QC 接受** | ADR §4.2 明确“AfterSales 业务申请 + Order 执行/准入账本，以 `refund_id` 关联”，S8 的“唯一发起方”仍指业务发起方；不构成第二套申请 |
| 4 | OFF→SHADOW 门禁（AC-66“任何开关变更” vs ADR 的 Director 影子批准） | **契约缺陷（MAJOR，非 BLOCKER）** | AC-66 字面“任何开关变更均需 D-27 全量门禁”，ADR §3.4 对 OFF→SHADOW 只要求 Director 影子批准+清单；ADR 自己写“若按更严格解释，以 AC-66 为准”，等于把冲突留给实现。应由 PO 在 AC-66 明确“OFF→SHADOW=影子批准；SHADOW→ON=全量门禁”，或 ADR 统一按全量门禁 |

## 3. 其他 MAJOR/MINOR

| 级别 | 位置 | 问题 | 建议 |
|---|---|---|---|
| MAJOR | `amd1.md:13`、`:701-717`、`:556-593`；Plan `plan.md:12` | D-27 的 shadow 表在 Fulfillment DB，ADR A-14/§11.11 已要求 C-05 扩展 `migrations/fulfillment/**`，但 Plan 的 `allowed_paths` 仍只有 `migrations/{payment,order}/**`；不更新 Plan，C-05 无合法路径 | 在 Plan 的 `allowed_paths` 增 `migrations/fulfillment/**`，并把该变更写入 QC/Plan 批准记录 |
| MAJOR | `amd1.md:94`（AC-51/AC-65 的 `PENDING_PAYMENT+已捕获`） | ADR §4.1.3/§4.8 规定 `PENDING_PAYMENT+已捕获` 先 CAS→`CANCELLED` 后走支付级取消退款；但 AC-63 的 `EXPIRED_REFUND` 是 **PAID** 历史单的“不写 `CANCELLED`”分支，二者语义不同，ADR 已区分；仅需在 §4.8 明确“只适用于 PENDING，不适用于历史 PAID”，避免实现混用 | 在 §4.8 加一句边界声明 |
| MINOR | `amd1.md:405-440`（§6.1） | §4.1.1 说 `(source, order_no, payment_no)` 唯一，§6.1 的 `idx_refund_batch_lookup` 不是 UNIQUE；`PRECHECK_FAILED` 可多批次，但 active batch 的“唯一”只靠应用检查 | 在 §6.1 注明“唯一由 Tx1 订单锁 + active state 保证；索引非 UNIQUE”或改为 `(source,order_no,payment_no,state)` 的应用约束 |
| MINOR | `amd1.md:364-403`（§5.4） | `legacy_payment_resolution.resolution_kind` 无 CHECK/枚举；`blocked_scope` 也无 CHECK | 补 `CHECK (resolution_kind IN (...))` 与 `blocked_scope` 枚举，避免自由文本 |
| MINOR | `amd1.md:510-540`（§6.4） | `refund_commitment_releases` 的 `authorized_by/reviewed_by` 是否强制“不同人”未在 schema 表达；D-24 的 ≥1000 双人规则只在 §4.3 文本 | 在 §6.4 加 CHECK 或触发器/事务校验说明，确保两人不同 |

## 4. D-27/D-28/D-29 与 P-05/P-04/P-06 的结论

- **D-27**：三态 `paid|shadow|settled`、默认 `paid`、影子只记录/比对、ON 门禁、回滚、移除日期均已定义；**B-2 的 schema 缺列**必须补，否则 ON 记录/移除日期无落点。OFF→SHADOW 门禁见边界 #4。
- **D-28**：Order 唯一写、`order_no` 主键、`resolution_id` 唯一、24h D-22 分支、锁/lease、终态退款意图同事务、`EXPIRED_*` 终态——均可接受；仅 `resolution_kind`/`blocked_scope` 的 CHECK 为 MINOR。
- **D-29**：`GRANTED` 非合法状态、预检阻断、人工分类、CHECK、代码移除、未知状态 fail-closed——可接受；未发现 BLOCKER。
- **P-05**：人工释放的 D-24 矩阵、同订单/同组、`refund_commitment_releases`、幂等/lease——可接受；P-05 的 AC-62 已由 Human 批准默认，ADR 与之一致。
- **P-04**：零元+已捕获见 B-3；`legacy_payment_resolution` 的 24h/唯一写/锁可接受。
- **P-06**：commitment lines（`line_no=0` 运费、`>0` 商品、`SUM` 校验、`composition_known=0` 只整组）可接受。
- **C-05 `migrations/fulfillment/**`**：ADR 已识别（A-14/§6.6/§11.11），但 **Plan 的 allowed_paths 未更新**（MAJOR）。

## 5. 声明

- 我**未修改任何文件**（ADR Amendment、SPEC 草案、Plan、代码均只读），**未运行测试/门禁**。
- 本次只做 ADR Amendment 预审，不替代 Human `ADR_APPROVED`；D-27…D-29 的最终关闭仍由 Architect/Reviewer 完成。
- 本报告不构成实现依据；C-03/C-04/C-05 须在 `SPEC_APPROVED` + `ADR_APPROVED` 后才可开始。
