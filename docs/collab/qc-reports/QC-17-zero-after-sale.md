# QC-17 — 零元售后 Amendment（v1.9-draft / ADR Addendum r2）审查

| 项 | 值 |
|---|---|
| review_scope | `docs/po/drafts/0002-v1.9-zero-after-sale.md`（v1.9-draft，AC-67/68、S19、INV-50、A-Z1…A-Z8）；`docs/adr/0007-amd1-addendum-zero-after-sale.md`（r2，AZ-01…06、A-Z1…A-Z8 答复）；对照 SPEC-0002 v1.8 AC-40/55/63、ADR Amd1、C-04/C-05 |
| 审查类型 | Spec + ADR Addendum 联合 QC；不替代 Human 批准，不构成实现依据 |
| 结论 | **`CHANGES REQUIRED`** |

> 只读审查；未改文件、未跑测试。

## 1. BLOCKER（3 条）

### B-1 `AdmitRefund(amount=0)` 不可“复用”，现有实现会拒绝零元单（**[ARCHITECTURAL_FLAW]**）

- 位置：`docs/adr/0007-amd1-addendum-zero-after-sale.md:22-27,56,72-81,133`（AZ-01/AZ-04/A-Z1/§3.1/§6）；`docs/po/drafts/0002-v1.9-zero-after-sale.md:41,135`（F-1/A-Z1）。
- 事实：现有 `internal/services/order/refund_admission.go:80-89` 的 `AdmitRefund` 要求 `record.SettlementPaymentNo != ""`，并调 `Payment.GetIntent` 后要求 `intent.CapturedCent > 0`；零元单的 `settlement_payment_no = zero:<order_no>`、无 Payment intent（`internal/services/order/zero.go:34`），`GetIntent(zero:...)` 失败/`captured=0` → 准入直接 `Failed`。
- 风险：Addendum 的“复用现有 `AdmitRefund`、C-04 无需新增字段、C-04/C-05 零影响”只在**正金额**成立；零元单走不到 permit/运单/INV-44，AC-67 的“组级释放”无法实现。`[ARCHITECTURAL_FLAW]`：把“零元分支”隐藏在“复用”下，未定义 `AdmitRefund` 的零元短路（`source=system_zero + pay_cent=0 + 无捕获` 时跳过 Payment 权威与 commitment）或独立 `AdmitZeroRefund`。
- 建议：在 Addendum 明确二选一：①`AdmitRefund` 增加零元分支（Order 代码改造，C-04 的“零影响”改为“需 Order 实现改造，proto 字段不变”）；②新增 `AdmitZeroRefund`/等价命令（C-04 增 RPC）。无论哪种，须写明 permit/运单/INV-44 的同一 CAS 语义与失败语义；不能只声明“复用”。

### B-2 `C-05` 的“零影响”不成立；`refund_applications` 需新增 `system_zero` 约束

- 位置：`docs/adr/0007-amd1-addendum-zero-after-sale.md:145`（§7“仅 C-05 的 `refund_applications` CHECK”）；PO v1.9-draft `:44,111`。
- 问题：Addendum §7 自己要求给 `refund_applications` 增加 `source='system_zero' → amount=0 AND goods=0 AND freight=0 AND non_effective_capture=0` 的 CHECK；这是 C-05 的**新增约束**，不是“零影响”。同时 `migrations/order/0017` 现有的 `chk_refund_app_non_effective_source` 对 `system_zero` 只要求 `non_effective_capture=0`，并未约束 0 金额。
- 风险：若按“零影响”实施，零元申请可写入正金额/非零 goods/freight，INV-50/AC-67 的“0 金额”无 DB 兜底。
- 建议：把该 CHECK 明确列为 C-05 增量（`0017` 追加），并注明“零元 `amount/goods/freight=0` + `non_effective_capture=0`”；C-05 的“零影响”改为“新增 1 个 CHECK，无新表/新列”。

### B-3 `amount_state=CONSUMED + REFUNDED/0 + refund_no=''` 与 AC-55 的例外尚未被 AC-68 正式覆盖

- 位置：`docs/po/drafts/0002-v1.9-zero-after-sale.md:72,94-101,153`；`amd1-addendum:24,27,54,120-125`。
- 问题：AC-55 的“以 Payment 退款事实为准”是已批准 AC；Addendum 以“AC-40/AC-68 明示例外”主张零元以 Order `refund_applications(DONE)` + AfterSales `REFUNDED/0` 为完成事实。但 AC-68 目前只是 **v1.9-draft 的新增**，尚未批准；AC-68 的文本把 `amount_state=CONSUMED`/`refund_no=''` 的表示写成“§5.1 推荐；A-Z3 若另选必须回 PO 升版”，不是不可争议的例外条款。
- 风险：在 AC-68 获批前，AC-55 字面仍优先；零元售后可能被 Reviewer 按 AC-55 判为“无 Payment 事实不得标 REFUNDED”。这是**契约层冲突**，不是实现细节。
- 建议：AC-68 改为无歧义条款：“零元售后（`pay_cent=0` 且无任何已捕获 intent）的完成事实是 Order `refund_applications(source=system_zero, amount=0, state=DONE)` + AfterSales `REFUNDED/0`，不适用 AC-55 的 Payment 事实优先；本例外是 AC-55 的版本化例外，仅适用于零元。”并在 AC-68 的 EARS 中明确该例外不改变 AC-55 对正金额退款的效力。

## 2. A-Z1…A-Z8 逐条判定

| ID | 判定 | 说明 |
|---|---|---|
| A-Z1 | **部分接受** | “0 金额不创建 commitment”与 S15“每个 `refund_id` 一条承诺（有金额占用）”可相容（0 元无额度可占），但必须以 B-1 的零元分支为前提；现有 `AdmitRefund` 到不了该分支。若保留 0 元 commitment，必须新增明确规则，不得由实现选 |
| A-Z2 | **接受** | Order 在 `AdmitRefund` 内写 `refund_applications(DONE)`，AfterSales 只读；与 S19/AC-67 的“Order 执行账本”一致 |
| A-Z3 | **有条件接受** | 复用 `CONSUMED` + `REFUNDED/0` + `refund_no=''` 可避免新枚举，但需 B-3 的 AC-68 例外先获批；否则 AC-55 字面冲突 |
| A-Z4 | **接受** | `refund_no=''` 合法；不得伪造 Payment `refund_no`；展示由 `system_zero + amount=0 + REFUNDED` 推导 |
| A-Z5 | **接受** | 未 `PAID` 取消只按 AC-46；非取消未 PAID 拒绝；有捕获按 AC-40/AC-63；与 AC-67 一致 |
| A-Z6 | **接受** | 回库完成 → `AdmitRefund` → `DONE` → `REFUNDED`；REFUND_ONLY/缺货无回库；顺序正确 |
| A-Z7 | **部分接受** | 保留 `order_refund_admissions` + `refund_applications(DONE)`、不建 0 元 commitment，与 A-Z1 一致；但需 B-1 的零元分支先落地 |
| A-Z8 | **接受** | `refund_id=after_sale_no` 幂等、任务 lease/retry 不重复释放/回库；与 AC-67 一致 |

## 3. 其他核对

- **S19 与 `refund_applications`**：S19 是 `refund_applications(source=system_zero)` 的**语义子集**，不是新表/新状态；不重复。但 S19 的“`PENDING→DONE`”与 Addendum 的“Order 在同一事务直接 DONE、PENDING 仅可选”存在**表述差异**，建议统一为“零元准入可同事务 DONE；若保留 PENDING，必须有持久重试与告警”，避免无 worker 的 PENDING 悬挂。
- **AC-67/68 EARS**：结构完整（WHEN/IF/WHILE），覆盖未 PAID、回库、幂等、Payment 0 次；但 AC-68 的“A-Z3 若另选”是**开放条款**，应按 B-3 收敛为确定规则；AC-67 的“S19 `PENDING` 不提前 REFUNDED”需与“同事务 DONE”统一（见上）。
- **C-04**：除 B-1 的 Order 实现分支外，proto 字段确实无需新增；但“C-04 零影响”应改为“proto 字段不变、Order 实现需零元分支”。
- **C-05**：见 B-2，需新增 `system_zero` 的 0 金额 CHECK；其余无新表/新枚举。
- **AfterSales 状态机**：`REFUNDED` + `amount_state=CONSUMED` + `refunded_cent=0` 与现有枚举相容；`refund_no=''` 可用；但 AC-55 例外需 B-3 先定。
- **未发现其他 `[ARCHITECTURAL_FLAW]`**：S19 不新建表、不新建状态，方向正确；唯一架构性缺口是 B-1 的“复用 `AdmitRefund`”假设。

## 4. 声明

- 我**未修改任何文件**（PO 草案、ADR Addendum、SPEC/ADR/C-04/C-05 均只读），**未运行测试/门禁**。
- 本次只做零元售后 Amendment 的 QC，不替代 Human 批准；B-1 的 `[ARCHITECTURAL_FLAW]` 需回 PO/Architect 修订后再批准。
- 不能声称 **AC-VERIFIED**、Spec/ADR 批准或 Release PASS；本报告不构成实现依据。
