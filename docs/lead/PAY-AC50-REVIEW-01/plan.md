# PAY-AC50-REVIEW-01 — AC-50 `CONSUMED+无运单` 复核决定（L2）

| 项 | 值 |
|---|---|
| Status | **DRAFT — 未批准；Architect/QC/Developer/Reviewer 评审 + 人类批准前不写业务代码** |
| change_id | `PAY-AC50-REVIEW-01` |
| Tier | **L2** |
| Tier-Rationale | 资金/TCC 批次/跨服务准入/状态机语义；命中 `AGENTS.md` §2.7 的 P0、资金、跨服务、状态机触发条，不适用 L1 减免 |
| Parent | `PAY-SETTLE-01`（**保持冻结**；本 L2 交付前不推进其受管代码/台账） |
| Base | `d2149c0`（PAY-AC50-REVIEW-01 立项；Parent `PAY-SETTLE-01` 冻结检查点 `cd32fb4`） |
| Spec-Ref | SPEC-0002 **v1.11**（`REVIEW_REQUIRED`；AC-50/51/61/62/65/67 修订；AC-69…72、S20、INV-45/46/51/52、§5.10、D-30/31）；SPEC-0001 **v1.5** AC-27A（只读依赖，不改文本） |
| ADR | ADR-0007 **Rev 2.3** + `0007-amd1-addendum-ac50-nowaybill.md` **DRAFT 0.5**（待接受）；`0007-amd1-ac50-impact-analysis.md` DRAFT 0.3 |
| integration_owner | System Architect（契约/跨服务）；Order Owner（实现）；ops（人工队列/告警） |
| allowed_paths（预期） | `internal/services/order/`（仅 `refund_admission.go`、`tcc_settlement.go`、`tcc_admission.go`、必要的 `refund_batch*`/consumer/metrics；不含 `tcc.go`/`tcc_payment.go`/`tcc_precheck.go`/`tcc_reconcile.go`）、`internal/services/fulfillment/cancel_if_created.go`、`internal/services/aftersales/`（仅 `after_sale_refund_tasks.last_error`/任务 PENDING/重试上限行为，不改结算语义）、`internal/platform/metrics/**`（AC-72 gauge，共享包，L2）、`internal/services/gateway/`（仅显示文案映射）、`api/proto/mall/v1/mall.proto`（仅注释/既有 string 值，无新字段）、`cmd/*`（仅 wiring）、对应 `_test.go` |
| forbidden_paths | `migrations/**`（本 L2 设计为无迁移；若需迁移必须回 Plan + 人类批准）、`deploy/**`（告警规则另立 `REVIEW-REQUIRED-ALERTS-01`）、`frontend/**`（入口另立 `REVIEW-REQUIRED-ENTRY-01`）、`docs/specs/**`、`docs/adr/**`、`docs/collab/**`、`docs/lead/**`、`api/gen/**`（手工禁改） |
| baseline_impact | **涉及**：AC-50/51/61/62/65/67、AC-69…72、INV-45/46/51/52；TCC batch `MANUAL_HOLD`；普通售后/零元/缺货/legacy 的准入写路径；PAY-SETTLE-01 的 AC-27/AC-50 基线。**不涉及**：Payment/Inventory 状态机、C-04 RefundBatch、T2/T3/T4.4/T5 |
| affected_AC | AC-50/51/61/62/65/67（修订）、AC-69…72（新增，先 DEFERRED，Reviewer 绑定证据） |
| 回归基线 | AC-14/21/25/27/31/32；INV-44…50；TCC Σ/逐组；零元 AC-67/68；legacy AC-60/63 |

## 0. 为什么另立 change_id

- `PAY-SETTLE-01` 已冻结（D-LC-5；Phase 0-R 未闭合）。AC-50 修复从“R0.3 小任务”升级为：新 SPEC 状态 + TCC 行为 + 跨服务写路径 + 审计/可见性，无法在冻结的父 change_id 内继续提交受管代码。
- 治理要求同一时间只推进一个 `change_id`（`AGENTS.md` §1.0）；本 L2 期间 `PAY-SETTLE-01` 不再产生受管代码/台账变更，只保留只读引用与冻结记录。
- Reviewer scope 独立：本 L2 的 PASS 只覆盖 AC-50/AC-69…72；**不**关闭 AC-27，**不**等于 Phase 0-R/P0 closure/Release。

## 1. 契约前置（P0，未完成不得进入 P1）

| ID | 内容 | Owner | 验收 |
|---|---|---|---|
| C-1 | PO 关闭 QC R4-B1（普通售后无 `refund_applications` 行时 REVIEW 的持久载体）、R4-B2（零元不持久化例外）、R4-B3（复核审计留存与 ADR §8.3 矛盾）+ 3 MAJOR | PO | SPEC-0002 v1.11 修订；EARS；影响节同步 |
| C-2 | Architect 修订 Addendum 0.5（审计留存、`tcc_*.go` allowed_paths、删除 0.3 残留引用） | Architect | DRAFT 0.5；与 v1.11 对齐 |
| C-3 | QC round 5 终审（0 BLOCKER） | QC | QC 报告；READY |
| C-4 | 人类 `SPEC_APPROVED` / `ADR_APPROVED`（Director 2026-11-26 已授权升版；记录本 L2 的实现批准） | Human | review-tracking 决策行 |

**Stop**：C-1..C-4 任一未闭合，不得写代码；不得用“代码先跑通”倒逼契约。

## 2. 实施任务（P2，每任务独占一个不变量/分支；依赖 C-4 + Plan 批准）

| ID | 任务 | Owner | 依赖 | 验收 / 证据 |
|---|---|---|---|---|
| L2-T1 | TCC：`REVIEW_REQUIRED` 为合法持久裁决；`ensureTccRemoteAdmissionTx` 不早失败；批次 `MANUAL_HOLD` + `last_error` + `TCC_BATCH_REQUIRES_MANUAL_HANDLING`；0 Payment；只阻塞本批；re-arm 续跑 | Developer | C-4 | TCC 集成（`MANUAL_HOLD`/0 打款/不判非法）；崩溃/重放；`-race` |
| L2-T2 | 普通售后：REVIEW 持久载体（按 C-1 裁决）；`AdmitRefund` 在 `Payment.GetIntent` 前读已持久化裁决；同 `refund_id` 不重探/不写/不读 Payment | Developer | C-4 | 准入集成：existing-first、Payment 不可用仍同一裁决、重放/并发 |
| L2-T3 | 部分金额：`CONSUMED` 不调 `CancelIfCreated`；Order 自有事实三分类（已离仓→POST / 未离仓→PARTIAL 零写入 / 不可判定→REVIEW+冻结 commitment） | Developer | C-4 | 三分支集成；投影滞后；禁止取消副作用；`-race` |
| L2-T4 | 零元/legacy：零元不写 PRE、不持久化；legacy 不猜 `payment_no`；组投影缺失且订单已离仓→REVIEW | Developer | C-4 | 零元 12 用例回归 + legacy 矩阵 |
| L2-T5 | 复核审计留存 + 指标/告警（AC-72）；BFF 文案“退款处理中（需人工核对）”；人工入口另立 `REVIEW-REQUIRED-ENTRY-01`；`deploy/**` 告警规则另立 `REVIEW-REQUIRED-ALERTS-01` | Developer + Architect | C-4 | 审计查询；`internal/platform/metrics` 2 gauge + tick；gateway 文案映射测试；告警规则不在本 L2 交付 |
| L2-T6 | Tx2 `state='PENDING_REMOTE'` CAS；Tx1/Tx2 崩溃、重复/乱序、DLQ/人工 re-arm | Developer | C-4 | 故障注入 + `-race`；状态审计 |
| L2-T7 | 证据矩阵：AC-50/51/61/62/65/67 修订行 + AC-69…72；Reviewer 绑定精确 HEAD | Developer + Reviewer | T1–T6 | `ac-coverage.tsv`（Reviewer）；无 AC-VERIFIED 除真实执行 |
| L2-T8 | 回归：AC-14/21/25/27/31/32、INV-44…50、TCC Σ/零元/legacy；不回归 PAY-SETTLE-01 基线 | Developer + Reviewer | T1–T7 | 真实 MySQL；`-race`/fault；Reviewer scoped PASS |

## 3. Plan 评审（P1，C-4 后）

- Architect：技术可行性/状态机/跨服务边界；
- QC：异常分支/测试矩阵/并发与崩溃窗口；
- Developer：可执行性/风险估算/是否需拆包；
- Reviewer：可验证性/证据路径/Tier 确认；
- Human：批准 Plan 后才能写代码（宪法 Workflow 8）。

## 4. 停止条件

- 任何契约缺口/QC|Reviewer FAIL → 冻结本 L2，不得用 if-else/局部锁/新状态绕过。
- TCC 出现非法 result、部分金额产生取消副作用、REVIEW 被自动洗成 PRE、审计缺失 → stop-the-line。
- 需要迁移/新字段/新 RPC/新 subject → 回 Plan + 人类批准，不得默认。
- 不得以本 L2 关闭 AC-27、Phase 0-R、P0 或 Release。

## 5. 交付与回归

- 交付：代码 Diff、真实测试日志（HEAD）、`-race`/故障注入、`【测试范围】/【未覆盖范围】`、反妥协自检。
- 回归：PAY-SETTLE-01 的 AC-27 仍 AC-REF；AC-14 等基线不得回退；INV-44/46 的 open 项不得被本 L2 静默关闭。
- 完成后交回 `PAY-SETTLE-01`：AC-27A 第 5/6 条实现与 R0.3-R1c 才能继续；Phase 0-R 仍需 T2/T3/T4.4/T5。
