# QC-PAY-AC50-REVIEW-01-PLAN — L2 Plan 红队评审（L2-P2）

| 项 | 值 |
|---|---|
| change_id | `PAY-AC50-REVIEW-01`（Parent `PAY-SETTLE-01` 冻结） |
| 审查对象 | `docs/lead/PAY-AC50-REVIEW-01/plan.md`（69 行）+ `tasks.md`（46 行）；对照 SPEC-0002 v1.11、ADR-0.5、impact 0.3、R8 报告 |
| 结论 | **BLOCKED**（3 个 `[BLOCKER]` + 5 个 `[MAJOR]` + 1 个 `[TODO]`） |
| 日期 | **2026-11-26** |
| Actor | `qc-ac50`（Quality Checker；与 Lead/Architect/Developer 不同 actor） |
| base/head | `752b3f5f3d84d6c9c412ebdb63b169db14c67728`（`git rev-parse HEAD`；Plan `e725a5c` / Tasks `2cca932`） |
| review_scope | 只审 Plan/Tasks 的异常分支、测试矩阵、任务拆分、evidence/DoD、AC/INV/ops 映射；不审 SPEC/ADR 内容、不执行代码/测试。 |
| release_status | **NOT RELEASE**；本报告不批准实现/merge/Release；C-4 人类批准仍有效，但 Plan 未过 L2-P2。 |

## 0. 结论

**BLOCKED。** Plan/Tasks 的**方向与 allowed/forbidden 边界**基本正确（TCC/AfterSales/partial/零元/审计分段、无迁移、C-4 前置、stop conditions），但作为**可执行、可验收的 L2 Plan** 仍有三类硬缺口：

1. **无测试矩阵**：Plan/Tasks 没有把 ADR §10 的 O1–O23/W1–W12/5 崩溃窗口映射到任务与证据；T7 只写“AC 证据矩阵”，没有用例、预期、命令或 `ac-coverage` 行。
2. **任务拆分违反“一任务一不变量”**：T1（TCC 合法值/MANUAL_HOLD/0 Payment/只阻塞本批/re-arm）、T2（existing-first/幂等/AfterSales 镜像/PENDING/8 次）、T5（审计/metrics/告警/BFF）、T6（Tx2 CAS/崩溃/重复/DLQ/re-arm）均同时承载多个不变量；Plan 自述 `plan.md:37` 的约束被自身 T1/T2/T5/T6 违反。
3. **Plan/Tasks 不一致且缺关键载体**：Plan `L2-T2` 没有 AfterSales 镜像/PENDING/8 次（只在 Tasks `tasks.md:25` 出现）；Tasks T2 的 canonical 仍是旧占位 `<code>/ticket=<id>`，与 D-35 `ticket=RVW:<refund_id>` 不一致；T5 仍写“BFF 文案/AC-72 告警”，与 gateway 文案映射/告警另立不一致。

## 1. 异常分支/测试矩阵覆盖（对照 ADR §10）

| 风险 | Plan/Tasks 覆盖 | 缺口 |
|---|---|---|
| TCC `MANUAL_HOLD`/0 Payment/只阻塞本批 | T1（`plan.md:41`/`tasks.md:24`） | 未点名 O1b/O20b/O22；未要求走 `TccCancelMarkPaid`；未列 `TCC_BATCH_REQUIRES_MANUAL_HANDLING` 断言 |
| 售后镜像 PENDING/8 次 | T2（`plan.md:42`/`tasks.md:25`） | Plan T2 未含 AfterSales；未点名 O19/O22/O22b；canonical 仍 `<code>/ticket=<id>` |
| partial 三分类 | T3（`plan.md:43`/`tasks.md:26`） | 未点名 O4b/O4c/O4d；未写“`cancelCalls==0`/未离仓零写入/REVIEW 1 commitment” |
| 零元/legacy | T4（`plan.md:44`/`tasks.md:27`） | 未点名 O7/O10/O21；零元“12 用例”无枚举 |
| 审计 | T5（`plan.md:45`/`tasks.md:28`） | 未点名 O20/O20b；未写 admission 行/version/时间戳/last_error 的断言 |
| CAS/崩溃 | T6（`plan.md:46`/`tasks.md:29`） | 未点名 O11b/O11c/O16；未列 W1–W12/5 窗口 |
| 重复/乱序/DLQ | T6（`plan.md:46`/`tasks.md:29`） | 未点名 O14/O14b/W8/W9；DLQ 只写“测试” |
| 5 崩溃窗口 | T6 仅“崩溃” | 无 W1–W12 逐项任务/证据 |
| 零元不进入 S20/AC-72 计数 | 无 | 未在 T4/T7 断言 O21；AC-72 三段的 `ac-coverage` 行未在 T7 |

**结论**：Plan/Tasks 只有“功能任务”，没有可验收的**测试矩阵**；`L2-T7` 只映射 AC，不映射 ADR 的 O 用例/W 窗口。**BLOCKER P-1**。

## 2. 任务拆分（一任务一不变量）

| 任务 | 承载的不变量/分支 | 判定 |
|---|---|---|
| **L2-T1** | TCC 合法裁决 + 不早失败 + `MANUAL_HOLD` + `last_error` + `TCC_BATCH...` + 0 Payment + 只阻塞本批 + re-arm | **至少 5 个不变量**；拆 T1a 合法读/回、T1b `MANUAL_HOLD`+0 Payment、T1c re-arm |
| **L2-T2** | existing-first + 同 `refund_id` 幂等 + AfterSales 镜像 + PENDING + 8 次行为（Tasks 才有） | **至少 3 个不变量**；拆 T2a existing-first、T2b 售后镜像/PENDING/8、T2c 幂等/并发 |
| **L2-T3** | partial 三分支（POST/PARTIAL/REVIEW） | 一个“部分金额分类”不变量，但三分支需独立测试；可保留 T3 + T3a/T3b/T3c 子用例 |
| **L2-T4** | 零元 + legacy 两来源 | **两个来源**；拆 T4a 零元、T4b legacy |
| **L2-T5** | 审计 + metrics + 告警 + BFF 文案 | **至少 3 个不变量**；拆 T5a 审计、T5b metrics/SLA、T5c gateway 文案 |
| **L2-T6** | Tx2 CAS + Tx1/Tx2 崩溃 + 重复/乱序 + DLQ + re-arm | **至少 4 个不变量**；拆 T6a CAS、T6b 崩溃/重复、T6c re-arm |
| **L2-T7/T8** | 证据/回归 | 可保留，但需绑定具体 tests/HEAD |

**BLOCKER P-2**：`plan.md:37` 要求“每任务独占一个不变量/分支”，但 T1/T2/T5/T6 直接违反；需拆包并在每任务声明独占的不变量，否则 Developer 无法按任务验收，Reviewer 无法按不变量判定。

## 3. Evidence/DoD 与 AC/INV/ops

- **T7 只写 AC-50/51/61/62/65/67 + AC-69…72**（`plan.md:47`），**未含 INV-45/46/51/52**；T8 只写 `INV-44…50`（`plan.md:48`），**INV-51/52 完全缺失**——但 Plan 自身 `baseline_impact`/`Spec-Ref` 明确涉及 INV-51/52。**MAJOR P-3**（与 P-1 叠加即 BLOCKER 级证据缺口）。
- **T7** 未把 AC-72 拆成**后端/告警/入口**三段；`tasks.md:28` 把 AC-72 指标/告警/BFF 混在一起；`REVIEW-REQUIRED-ALERTS-01`/`REVIEW-REQUIRED-ENTRY-01` 未进 T7/T8 证据。**MAJOR P-4**。
- **DoD**（`plan.md:65-68`）只写“代码 Diff、测试日志（HEAD）、-race/故障注入、范围、自检”，**没有**：精确命令、期望结果、失败即 FAIL 的断言、`ac-coverage` 行、`test_type/evidence_path/status`、`NOT RELEASE` 绑定。**MAJOR P-5**。
- **C-3/C-4 状态**：`plan.md:32`/`tasks.md:11` 仍是“QC round 5”；实际 QC 已到 R8；`tasks.md:12` C-4 未勾选（人类已批准）。**MAJOR P-6**。
- **T5** `plan.md:45`/`tasks.md:28` 仍写“AC-72 指标/告警 + BFF 文案”，而 ADR/Plan 边界是 `gateway` **文案映射测试**、告警规则另立 `REVIEW-REQUIRED-ALERTS-01`；**MAJOR P-7**。
- **`[TODO]` P-8**：T5 的 ops 证据（告警规则/入口另立）只在文字中；Plan 未见 `REVIEW-REQUIRED-ALERTS-01`/`ENTRY-01` 的 owner/expiry/stop 台账行；建议在 T5/T7 增补。

## 4. 具体修改

1. **P-1（BLOCKER）**：在 plan/tasks 增加**测试矩阵表**，逐行绑定 ADR §10 的 `F1–F8`、`O1–O23`、`W1–W12` 与 5 窗口；每行写 `test_type / evidence_path / expected / status`；T7 负责 `ac-coverage`，T8 负责回归，二者都引用同一矩阵。
2. **P-2（BLOCKER）**：拆 T1/T2/T5/T6（以及 T4 两来源），每任务只持一个不变量；在任务表新增“独占不变量”列；Plan 的“每任务独占”才成立。
3. **P-3（BLOCKER）**：Plan T2 补 **AfterSales 镜像/PENDING/8 次**，与 `tasks.md:25` 对齐（不能只有 Tasks 有、Plan 没有）；同时把 `tasks.md:25` 的 `<code>/ticket=<id>` 改为 D-35 `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>`（无 `arm`，`truncated=1`），并补 O19/O22/O22b。
4. **P-4（MAJOR）**：T7/T8 增 **INV-51/52** 证据；AC-72 分三段登记（后端/告警/入口），`ac-coverage` 不得整体 VERIFIED；`REVIEW-REQUIRED-ALERTS-01`/`REVIEW-REQUIRED-ENTRY-01` 各一行。
5. **P-5（MAJOR）**：DoD 增加可执行证据：`go test ./internal/services/{order,fulfillment,aftersales}/...`、`-race`、故障注入、真实 MySQL、精确 HEAD、期望 PASS/FAIL、`【测试范围】/【未覆盖范围】`、`release_status=NOT RELEASE`；T7 绑定 `ac-coverage.tsv`。
6. **P-6（MAJOR）**：`plan.md:32`/`tasks.md:11` 改为 **QC R8**（0 BLOCKER）；`tasks.md:12` C-4 按人类批准记录勾选/注明“HUMAN APPROVED 2026-11-26”并链接 review-tracking。
7. **P-7（MAJOR）**：`plan.md:45`/`tasks.md:28` 改为“gateway 文案映射测试；AC-72 告警规则不在本 L2（`REVIEW-REQUIRED-ALERTS-01`）；完整入口 `REVIEW-REQUIRED-ENTRY-01`”。
8. **P-8（TODO）**：T5/T7 的 ops 告警/入口另立行补 owner/expiry/stop；Plan/Tasks 的 `C-1/C-2` 与 R8 证据绑定。

## 5. 结论与门禁

**BLOCKED。** Plan/Tasks 不能在当前形态交 Developer/进入实现；三处 BLOCKER（无测试矩阵、任务拆分多不变量、Plan/T2 与 D-35 不一致/缺 AfterSales）必须修复并重新提交 QC P2；MAJOR P-4..P-7 需一并处理。C-4 人类批准有效，但不覆盖 Plan 的 P2 缺口；本 L2 仍 **NOT RELEASE**，不得写业务代码。

**只读声明**：未执行代码/测试；只审 Plan/Tasks 与其对 SPEC/ADR/R8 的可验收性；未改 Plan/SPEC/ADR/代码/台账；只新建本报告；未 commit。

**Actor**：`qc-ac50`
**Reviewed-by**：待 Lead 修订 Plan/Tasks / QC R2
**Status**：**BLOCKED — 3 BLOCKER**
