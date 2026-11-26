# ADR-0007 AC-50 Addendum — Plan Review（L2-P1，Architect）

| 项 | 值 |
|---|---|
| Change ID | `PAY-AC50-REVIEW-01`（Parent `PAY-SETTLE-01` 冻结） |
| 评审对象 | `docs/lead/PAY-AC50-REVIEW-01/plan.md`、`tasks.md`（工作树；Base `d2149c0`，HEAD `752b3f5`） |
| 契约基线 | SPEC-0002 **v1.11 approved**（2026-11-26）；ADR-0007 AC-50 Addendum **0.5 ACCEPTED**（`ADR_APPROVED` 2026-11-26；QC R8 READY）；impact 0.3 |
| 结论 | **READY（无 `[ARCHITECTURAL_FLAW]`）**；**P5 前必须修正 I-1…I-7**（Lead/PO 文本与拆分项，不改架构） |
| 只读声明 | 未执行代码/测试/迁移；行号为工作树阅读事实 |

## 1. 技术可行性

- **表示/无迁移**：`REVIEW_REQUIRED` 15 字符进 `order_refund_admissions.result VARCHAR(32)` 与 `refund_applications.admission_result VARCHAR(24)`；`state` 不新增值；`refund_batches.MANUAL_HOLD` 已在 0017 CHECK；**无迁移/回填/CHECK**（`0016:7-19`、`0017:20-33,56`）。可行。
- **TCC**：`ensureTccRemoteAdmissionTx` 接受合法裁决、`ensureTccApplicationTx` 单调接受 REVIEW、`writeTccBatchGroupsTx` 对 `MANUAL_HOLD` 短路、`admitTccBatch` 置批/返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`、0 Payment；`tcc_settlement.go`/`tcc_admission.go` 在 allowed，`tcc.go`/`tcc_payment.go`/`tcc_precheck.go`/`tcc_reconcile.go` forbidden。可行（`plan.md:14,41`）。
- **Order/AfterSales**：Order 唯一权威裁决 `order_refund_admissions`；AfterSales 仅 canonical `last_error`、保持 `PENDING`、**仅 REVIEW** 8 次不自动 FAILED；非 REVIEW 保留既有 8 次/FAILED。边界可执行（`plan.md:14,42,45`；ADR §8.5）。
- **部分金额/零元/legacy**：D-31 三分法（不调 `CancelIfCreated`）、零元不写 S20/不持久化、legacy 不猜 `payment_no`；与 SPEC/ADR 一致（`plan.md:43,44`）。

## 2. 状态机 / 所有权 / 恢复

- 所有权：S20/准入 = Order；S6 = Fulfillment；task 镜像/状态 = AfterSales；TCC 批次 = Order。无跨服务 DB 写（AfterSales 写自己的 task，Order 不跨库）。
- 状态机：`PENDING_REMOTE → REVIEW_REQUIRED`（`state` 不变）→ 人工 re-arm 回 `PENDING_REMOTE` → PRE/POST；TCC `ADMITTING→MANUAL_HOLD→ADMITTING`。无新增 state 值/状态字段。
- 恢复/幂等窗口：T6 覆盖 Tx1/Tx2 崩溃、重复/乱序、DLQ、re-arm；T1/T2 覆盖 TCC 重放与 existing-first。**缺口**：aftersale re-arm 的跨服务“先审计后清除”顺序只在 ADR §8.3/§8.5，任务未点名（见 I-6）。

## 3. 任务拆分（一任务一不变量）

- T1（TCC 合法裁决/MANUAL_HOLD）、T3（partial 三分）、T4（零元/legacy）、T6（CAS/崩溃）、T7（证据）、T8（回归）粒度可接受。
- **T2 混合 3 个不变量**（existing-first 幂等、AfterSales 镜像、任务 8 次状态）→ 建议拆 T2a（Order existing-first/同键）与 T2b（AfterSales canonical/ PENDING/8 次）。
- **T5 混合**审计留存、metrics、gateway 文案、告警边界（告警另立）→ 建议拆 T5a（审计/admission 行/镜像）、T5b（gauge+tick）、T5c（gateway 文案映射测试）。
- **T1 未点名** `ensureTccApplicationTx` 单调接受 REVIEW、Tx2 同事务写 `refund_batches`/`refund_applications`（ADR §3.6）；需补显式子项，否则易只改 `ensureTccRemoteAdmissionTx`。

## 4. allowed_paths / 迁移

- Order 精确文件 + AfterSales（仅镜像/PENDING/重试）+ platform metrics + gateway（仅文案）+ proto（仅注释/既有 string）+ cmd（wiring）+ `_test.go`；deploy/frontend/migrations/docs/api-gen forbidden。**足够且最小**。
- `api/gen/**` 由 `make proto` 生成、Plan 未显式列 → 实施 Brief 必须写明生成/提交方式。
- **无迁移结论成立**：本 L2 不新增表/列/索引；若实现需迁移即 stop（`plan.md:15`）。

## 5. 问题清单（P5 前处理）

| # | 位置 | 问题 | Owner | 阻断 P5 |
|---|---|---|---|---|
| I-1 | `tasks.md:25` | canonical 仍写 `ticket=<id>`；SPEC 定稿为 `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>`（非空必填、无 arm、`truncated=1`） | Lead | **是** |
| I-2 | `tasks.md:28` | 写“AC-72 指标/告警 + BFF 文案”；Plan `plan.md:45` 已改为 gateway 文案映射测试、告警归 `REVIEW-REQUIRED-ALERTS-01` | Lead | **是** |
| I-3 | `tasks.md:25` | T2 混合 existing-first/镜像/8 次三个不变量 → 拆 T2a/T2b | Lead | 是 |
| I-4 | `tasks.md:28` | T5 混合审计/metrics/文案 → 拆 T5a/T5b/T5c | Lead | 是 |
| I-5 | `tasks.md:24` | T1 漏 `ensureTccApplicationTx` 单调、Tx2 同事务写 batch/app；补子项 | Lead/Developer | **是** |
| I-6 | `tasks.md:29` | T6 未点名 aftersale re-arm“先审计后清除”跨服务顺序与 ops runbook owner | Lead/ops | 是 |
| I-7 | `plan.md:14` | `refund_batch*` 在本仓库无对应文件；`consumer/metrics` 需精确文件，避免范围漂移 | Lead | 否（实施 Brief 前） |
| I-8 | `plan.md:15` | forbidden `docs/adr/**` 与 C-2 Architect 修订冲突；应注明“实现阶段禁止，契约阶段由 Architect 交付” | Lead | 否 |
| I-9 | `plan.md:5,12,32`；`tasks.md:3,9-12` | 状态/ADR 0.5/QC round 5/C-1..C-4 仍 DRAFT/未勾选；C-4 已批准、QC R8 READY，需同步 | Lead | 否（台账卫生） |
| I-10 | `plan.md:17` | AC-69..72 仍写“先 DEFERRED”；批准后应为 in-scope + 证据，非 DEFERRED | Lead/Reviewer | 否 |

## 6. 结论

**READY**：技术可行、所有权与状态机自洽、TCC/AfterSales/Order 边界可执行、无迁移结论成立、恢复/幂等窗口有覆盖；**无 `[ARCHITECTURAL_FLAW]`**。I-1…I-6 为 P5 前必修（文本/拆分），I-7…I-10 为实施 Brief/台账卫生项；修正后交 QC L2-P2、Developer L2-P3、Reviewer L2-P4，再由人类 P5 批准写码。

**Actor**：`arch-ac50`（System Architect）
**Status**：**PLAN REVIEW — READY (conditional on I-1..I-6 before P5)**
