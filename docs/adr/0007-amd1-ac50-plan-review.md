# ADR-0007 AC-50 Addendum — Plan Review（L2-P1，Architect）

| 项 | 值 |
|---|---|
| Change ID | `PAY-AC50-REVIEW-01`（Parent `PAY-SETTLE-01` 冻结） |
| 评审对象 | `docs/lead/PAY-AC50-REVIEW-01/plan.md`、`tasks.md`（工作树；Base `d2149c0`，HEAD `752b3f5`） |
| 契约基线 | SPEC-0002 **v1.11 approved**（2026-11-26）；ADR-0007 AC-50 Addendum **0.5 ACCEPTED**（`ADR_APPROVED` 2026-11-26；QC R8 READY）；impact 0.3 |
| 结论 | **READY（无 `[ARCHITECTURAL_FLAW]`）**；**P5 前必须修正 I-1…I-7**（Lead/PO 文本与拆分项，不改架构）。**0.6 errata 后**：CAS 双谓词、`docs/ops.md` 仅 runbook、删除 gateway、AfterSales 收窄 `workers.go` 已由 ADR 0.6 落定，本评审的 gateway 建议以 0.6 为准 |
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

---

## Re-review（2026-11-26，delta：重写 Plan + ADR 0.6 errata）

| 项 | 值 |
|---|---|
| 对象 | `docs/lead/PAY-AC50-REVIEW-01/plan.md`、`tasks.md`（Base `a1cc809`；工作树） |
| 契约 | SPEC v1.11 approved；ADR 0.5 ACCEPTED + **0.6 errata DRAFT（C-5 OPEN）**；impact 0.4 |
| 结论 | **READY（无 `[ARCHITECTURAL_FLAW]`）**；**P5 前必修 R-1/R-3**；R-2/R-4/R-5/R-6 为实施 Brief/QC 前文本项 |

### I-1..I-10 关闭表

| 项 | 判定 | 依据 / 残余 |
|---|---|---|
| I-1 canonical | **CLOSED** | `tasks.md:34` 不再写 `ticket=<id>`；Plan/Tasks 无 stale canonical；ADR 0.6 是唯一格式来源 |
| I-2 T5 边界 | **CLOSED** | `plan.md:53,74` 拆 T5a/T5b；AC-72 三段；gateway/BFF 不在 L2（`plan.md:15`） |
| I-3 T2 拆分 | **CLOSED** | `plan.md:47-48`、`tasks.md:33-34` 拆 T2a/T2b |
| I-4 T5 拆分 | **CLOSED** | `plan.md:52-53`、`tasks.md:38-39` 拆 T5a/T5b |
| I-5 T1 ensureTccApplicationTx/Tx2 | **PARTIAL** | `plan.md:44` T1a 已含 `ensureTccApplicationTx`；但 **T1b `plan.md:45`/`tasks.md:31` 未写“同一 Tx2 原子写 `refund_batches`(MANUAL_HOLD/last_error)+`refund_applications.admission_result`+admission REVIEW”**（ADR §3.6）；见 R-1 |
| I-6 re-arm 审计顺序 | **CLOSED** | `plan.md:52`、`tasks.md:38` T5a“re-arm 先审计后清”；`tasks.md:41` T6b 审计/授权/CAS；ops runbook 在 allowed（`plan.md:14`） |
| I-7 `refund_batch*` 精确文件 | **CLOSED（Plan 侧）** | `plan.md:14` 已无 `refund_batch*/consumer`；但 ADR 0.6 `:24` 仍留条件文件 → R-2 |
| I-8 forbidden 阶段例外 | **CLOSED** | `plan.md:15` 已注“`docs/adr/**`（实现阶段；契约由 Architect 交付）” |
| I-9 状态/QC R8/C-4 | **CLOSED** | `plan.md:5,29-33`；`tasks.md:8-12` C-1..C-4 [x]，C-5 [ ]；C-5 状态与 0.6 一致 |
| I-10 AC-69..72 in-scope | **CLOSED** | `plan.md:73` 明确 in-scope；T7 `plan.md:56` 含 AC-69…72/三段 |

### 0.6 errata 吸收

| 0.6 项 | 判定 | 依据 / 残余 |
|---|---|---|
| CAS 双谓词 | **PARTIAL** | `plan.md:43`/`tasks.md:28` T0c 已写双谓词，但把 `result='PENDING_REMOTE' AND state='PENDING_REMOTE'` 列为可选；正金额 Tx1 `result=POST` → 该选项会 0 行，**必须只用 `result <> 'REVIEW_REQUIRED'`**（R-3） |
| `docs/ops.md` runbook | **CLOSED** | `plan.md:14` allowed（仅 runbook）；forbidden 无 ops |
| proto/生成物 | **PARTIAL** | `plan.md:14` 说明生成 diff 由 `make proto`；未写“提交方式由 Lead 在实施 Brief 记录”（R-4） |
| gateway/BFF | **CLOSED** | `plan.md:15` forbidden gateway；AC-72 入口归 ENTRY-01（`plan.md:74`） |
| 发布顺序 | **CLOSED** | `plan.md:13,41` 原子提交或 Fulfillment 先发、旧 Order fail-closed |
| AfterSales 收窄 | **CLOSED** | `plan.md:14` 精确 `internal/services/aftersales/workers.go` |

### 残余（P5/实施前）

| # | 位置 | 问题 | 级别 | Owner |
|---|---|---|---|---|
| R-1 | `plan.md:45`、`tasks.md:31` | T1b 缺“Tx2 同事务原子写 batch MANUAL_HOLD/last_error + app admission_result + admission REVIEW”；否则崩溃可留下 admission REVIEW 而 batch 仍 ADMITTING（与 ADR §3.6 不一致） | **P5 前** | Lead |
| R-2 | `plan.md:14` vs ADR 0.6 `:24` | ADR allowed 仍列 `refund_batch*.go/refund_consumer.go/consumer.go`（条件），Plan 已删；Reviewer 会看到范围不一致 | 实施 Brief 前 | Architect（0.6 修订）/Lead |
| R-3 | `plan.md:43`、`tasks.md:28` | T0c 的 `result='PENDING_REMOTE' AND state='PENDING_REMOTE'` 选项对正金额（Tx1 result=POST）无效，会 0 行；**只允许 `result <> 'REVIEW_REQUIRED'`**（ADR A8） | **P5 前** | Lead |
| R-4 | `plan.md:14,41` | proto/`api/gen` 仅写“生成 diff”；未把“生成物提交方式由 Lead 在实施 Brief 记录”写入 T0/DoD | 实施 Brief 前 | Lead |
| R-5 | `plan.md:43`、`tasks.md:28,40` | T0c 未列 **O11d**（REVIEW 后迟到 CANCELLED）；T6a 只到 W1–W9，缺 **W13**（0.6 新增） | P5 前（测试覆盖） | Lead/QC |
| R-6 | `plan.md:16` | “**不交付** T2/T3/T4.4/T5”中 T5 与 L2-T5 冲突；应写“PAY-SETTLE-01 Phase T5”避免歧义 | 实施 Brief 前 | Lead |

**结论**：四份 Plan 评审后，原 I-1/I-2/I-3/I-4/I-6/I-7/I-8/I-9/I-10 已关闭；I-5 与 0.6 CAS/proto 吸收仍为 PARTIAL。**技术可行性/所有权/无迁移结论不变，无 `[ARCHITECTURAL_FLAW]`**；**READY** 进入 P2–P4，但 **R-1/R-3 必须在 P5 前修复**，R-2/R-4/R-5/R-6 在实施 Brief/QC 前修复。

**Actor**：`arch-ac50`（System Architect）
**Status**：**PLAN RE-REVIEW — READY (conditional on R-1/R-3 before P5)**

## P1b focused re-review（2026-11-26，R-1/R-3/R-4/R-5/R-6）

| 项 | 判定 | 依据 / 残余 |
|---|---|---|
| **R-1** T1 Tx2 原子（batch/app/admission） | **PARTIAL** | `tasks.md:38` T1c 已明确“Tx2 同事务写 batch/app/admission”；但 `plan.md:46` T1c 内容列仍只写 `MANUAL_HOLD`+`last_error`/0 打款/只阻塞本批，未写同事务原子性 → **Plan 行需补一句**（P5 前） |
| **R-3** CAS 字面 | **CLOSED** | `plan.md:43`、`tasks.md:31`、`plan.md:86` 均逐字为 `state='PENDING_REMOTE' AND result <> 'REVIEW_REQUIRED'`；不再有 `result='PENDING_REMOTE'` 的错误选项 |
| **R-4** proto/`make proto` | **CLOSED** | `plan.md:41` T0“proto 注释 + `make proto` 生成同提交”；`plan.md:14` allowed 只列 proto、`api/gen` 禁手改 |
| **R-5** O11d/W13 | **CLOSED** | `plan.md:43` T0c 含 O11d/W13；`plan.md:58` T6b 含 W13；`tasks.md:31,66` 同 |
| **R-6** T5 命名 | **PARTIAL** | 任务表已拆 `T5a/T5b`（`plan.md:55-56`、`tasks.md:60-64`）；但 `plan.md:16` baseline 仍写“**不交付** T2/T3/T4.4/T5”，与 L2-T5 同名；应改为“PAY-SETTLE-01 Phase T5”（P5 前一行） |

**0.6 allowed 清理**：已从 ADR 0.6 `allowed_paths` 删除 `refund_batch*.go/refund_consumer.go/consumer.go`，并在 `forbidden_paths` 显式标注“本 L2 不使用”；impact 0.4 §6.1 同步删除。ADR 状态保持 **DRAFT 0.6 — pending QC**。

**结论 READY（conditional）**：R-3/R-4/R-5 已闭合；R-1/R-6 为 Plan 文本一行级残留（`plan.md:46`、`plan.md:16`），**P5 前修**；无 `[ARCHITECTURAL_FLAW]`。修后交 P2–P4、P5。C-5（0.6 QC/人类批准）仍 OPEN，未关闭前不写码。

**Actor**：`arch-ac50`（System Architect）
**Status**：**P1b RE-REVIEW — READY (conditional: R-1/R-6 one-line fixes before P5)**
