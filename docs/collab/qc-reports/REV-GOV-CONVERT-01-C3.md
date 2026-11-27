# REV-GOV-CONVERT-01-C3 — B-1…B-6 状态对齐 + SPEC-0002 验收状态表复审

| 项 | 值 |
|---|---|
| change_id | **`GOV-CONVERT-01`**（C-3 范围） |
| 级别 | **契约变更（状态迁移，不是行为变更）**；不改任何 AC 语义/代码 |
| 阶段 | **Spec** |
| review_scope | ① B-1…B-6 状态对齐（SPEC-0002/ADR 头注、台账、`ac-coverage.tsv`、`tasks.md`）；② C-3：SPEC-0002 **新增 §15 验收状态表/延期清单**；③ 本裁决台账条目。**不审**实现/代码/Release/merge、C-1 宪法与 AGENTS 文本实质、C-4 归档、13 个 addendum 合并、AC-22/28/29/30/35/37 迁回 SPEC-0001 |
| base | **`9e81497`** |
| head | **工作树 @ `9e81497`（未提交）**：评审基线 = `git diff 9e81497` 的 7 个修改文件（评审时 +232/−248；落盘后 +346/−248）+ `docs/convert/**` 未跟踪；本报告、SPEC-0002 §15、台账条目为 Reviewer 依任务书落盘，同样未 commit |
| 契约输入 | 宪法 **v2.0.0**（2026-11-27 生效）；根 `AGENTS.md`；SPEC-0002 v1.11（§2/§9/§11/§12/§14）；`docs/adr/0007-amd1-addendum-ac50-nowaybill.md` 0.6；`docs/convert/DECISIONS.md`（人类裁决转写）；`docs/review-tracking.md`；`docs/collab/ac-coverage.tsv`；`docs/lead/PAY-SETTLE-01/tasks.md` |
| INV | 本次无资金/库存/履约行为变更；状态一致性依据宪法 **27/28/29/30**。领域不变量不变：INV-45（AC-62 仍 `RESERVED`/无 release writer）、INV-51/52（AC-69…72 既有状态）、INV-46（AC-63 仍未验证）。 |
| verification | **静态审查**（Spec 阶段）：逐行核对 `git diff 9e81497`、tsv、台账、tasks.md、DECISIONS；`git show 9e81497:docs/review-tracking.md` 核对被取代行号；证据文件存在性核对；复跑 `GOCACHE=/tmp/gocache-rev-c3 go test ./internal/platform/metrics -run TestRefundReviewGaugesRegistered -count=1` → `ok 0.131s`。**未复跑** MySQL 8.4.13 / NATS / `-race` 集成证据（环境未提供，且本阶段不重开既有 AC 验收）。 |
| 日期 / Actor / Reviewed-by | 2026-11-27 / **`reviewer-c3`**（独立 subagent；与作者 `lead` 不同 actor） / **`human`**（11 项裁决与冻结范围；作者 `lead` 不是 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge / P0 closure / Release |
| overall verdict | **SCOPED-PASS**（B-1…B-6 状态对齐 + §15 表）；**0 BLOCKER**；4 条非阻断 MINOR（NB-1…NB-4）+ 1 条观察（OBS-1）；门禁项 **G-1** 需人类裁决后才能启动 `T2.5+` |

---

## 0. 结论

**SCOPED-PASS。** 在 `9e81497..工作树` 范围内：

1. **B-1…B-6 的状态值与人类裁决/tsv 一致**（逐条见 §2）：AC-60/63=未验证、AC-72 分段、AC-27=有引用、冻结解除范围、11-27 重排、父变更头注均按裁决落地；未发现状态值层面的 BLOCKER。
2. **SPEC-0002 §15 验收状态表/延期清单已建立**，只新增表格与说明，**未改任何 AC 正文**；状态严格按 tsv `AC-VERIFIED/AC-REF/DEFERRED` → `已验证/有引用/未验证` 映射；未新增证据。
3. 发现 **4 条非阻断问题**（NB-1…NB-4）与 **1 条下一阶段门禁**（G-1，裁决 7-B 与 9-A 的范围冲突，作者不可自行修复）。**不把 B-1…B-6 退回作者**：状态值正确，问题属格式/文档漂移/人类裁决范围，见各条"处置"。

> 本文所有台账行号为**本次插入后**的当前行号；`git show 9e81497:docs/review-tracking.md` 的旧行号只在 §2"被取代"处标注。

---

## 1. 审查输入与方法（不依据作者自述）

- **只用**宪法、AGENTS、SPEC-0002、它链接的 ADR、base..head 的 diff、台账、tsv、`tasks.md`，以及人类裁决记录 `docs/convert/DECISIONS.md`。`docs/convert/A1-SELFCHECK.md` **只作线索**，未据其任何结论判定（例如"32 条 DEFERRED/48 行"等数字均在本次自行复核）。
- **未采用**作者在 `DECISIONS.md` §3 的执行结果表作为证据；每一 B 项均回到 `git diff`/`git show 9e81497` 与文件原文核对。
- **静态审查声明**：本次只复跑 1 个不依赖 DB 的单元测试（`internal/platform/metrics`，PASS）。AC-50/69/70/71/72-BACKEND 的 `已验证` 沿用 `reviewer-r04` 在 `3ce0d7d` 绑定的既有裁决（`REV-PAY-AC50-REVIEW-01-FINAL` / `-RESIDUAL`），**本次不重新判定其语义**；AC-REF 的"有引用"同理。
- 是否违反已批准 ADR：B-1…B-6 只改状态注记/状态表，ADR 0.6 的决策正文未改。ADR 头注的"解除冻结"措辞问题见 NB-4。

---

## 2. B-1…B-6 逐条结论

| 项 | 结论 | 核对事实 | 关联问题 |
|---|---|---|---|
| **B-1** AC-60/63=DEFERRED | **PASS** | tsv `:29`/`:32` 均为 `DEFERRED`，`git diff` 未改这两行；台账 `:501-504` 记 DEFERRED；`git show 9e81497` 的 `:465/:473/:495` 正是被取代的 AC-REF 表述；代码中无 `TestAC60`/`TestAC63`（grep `internal/` 零命中），只有生产注释（`tcc_precheck.go:97`、`legacy_resolution.go:14`、`refund_admission.go:679`）。 | NB-1（check type 只能推断）、NB-3（取代未点名） |
| **B-2** AC-72 拆分 | **PASS** | tsv `:47` = `AC-72-BACKEND` `AC-VERIFIED`；`:48-49` = `REVIEW-REQUIRED-ALERTS-01`/`ENTRY-01` `DEFERRED`，reason 带 `AC-72-ALERTS/ENTRY` 前缀；**无** AC-72 整体行。SPEC-0002 AC-72/§9 要求"三段登记、AC-72 不得整体标 AC-VERIFIED"，满足；`worker_test.go:14`、`metrics_test.go:11`、`docs/ops.md` 均存在，metrics 单测复跑 PASS。 | — |
| **B-3** AC-27 回 AC-REF | **PASS** | tsv `:41` = `AC-REF`，reason 写明 ruling 6 且"C+A waiver not formalized"；DECISIONS ruling 6-B；`tasks.md:57` 的"未关闭前 AC-27 不得 AC-VERIFIED"保持有效；与 `AC-27-NOWB-01`/`AC-27-LIVE-01` 仍 open 一致。 | NB-3（未点名取代 AC-27 旧 VERIFIED 流水） |
| **B-4** 冻结解除范围 | **PASS（带门禁 G-1）** | 台账 `:511-513`；DECISIONS ruling 9-A"追认并放开剩余范围：Phase 0-R 收口 + T2.4 剩余；no merge/no release"；`git show 9e81497` 的 5 处冻结记录（`:175/:198/:741/:789/:982`）仍留在历史条目中未改写，B-4 声明只按其范围更新，未扩大；no merge/no release 写明。 | **G-1**（放开的"剩余"已消耗，B-5 的下一步落在范围外）、NB-3（未写"追认"） |
| **B-5** 11-27 重排更正 | **PASS** | 台账 `:514-516` 以 `T2→T3→T4→T5→P0 closure` 为准；`:569` 的重排条目支持；`:687`"先 Phase 0-R"被取代；`tasks.md:26` 已改（diff），`:81` 同样写新顺序。 | NB-3（closeout 的"Next: T5"未点名取代）、NB-2（tasks.md:60 反向陈述） |
| **B-6** 父变更头注同步 | **PASS（带 MINOR）** | `git diff` 中 SPEC-0002 仅 `:9`/`:24` 两行、ADR nowaybill 仅 `:7`/`:8`/`:98` 三行；历史记录 `0002:1011`/`:1038` 未动；"历史父变更；2026-11-27 裁决 9 解除冻结（仍 no merge/no release）"与裁决 9-A 的"no merge/no release"一致。 | NB-4（ADR `:99` 残留"冻结 Parent"、运行状态入契约头） |

**没有发现的矛盾（逐项复核过）**：tsv 与台账在 AC-50/51/61/62/65/67/69/70/71/72 上一致；AC-14/21/25/31/32 未变；B-2 的入口/告警 owner/expiry 与 SPEC AC-72 一致；B-6 的历史记录未被改。

---

## 3. 逐条问题（非阻断）

### NB-1 [MINOR] 宪法附录 A 未定义延期清单/owner/expiry/到期规则

- **违反/依据**：宪法**附录 A** 的验收状态表列只定义 `AC 编号|状态|检查类型|证据路径|审查者`，未定义 `owner/expiry/reason`；旧 `docs/collab/CONVENTIONS.md:201-203` 的"延期必须有 owner/expiry/reason/thread、到期阻断、不得批量续期"已随 v2.0.0 退役。这是**治理缺口**，不是作者违反某条 AC。
- **事实**：tsv 现有 **32** 条 `DEFERRED`（本报告自行清点：AC-22/28/29/30/35/37、AC-39…49、AC-52…60、AC-63/64/66/68、`REVIEW-REQUIRED-ALERTS-01`/`ENTRY-01`）；台账 `:578-587` 把 AC-39/41/57/59 列为 P0 `[BLOCKER]`，但 tsv 的 expiry 统一 2026-12-31，**没有到期阻断规则**可执行。
- **影响**：DEFERRED 行的检查类型只能按 §9（每条 AC 必须有 `TestAC<NN>`）与 tsv reason 推得；其中 14 行 reason 只写 `WP-*`（AC-40/41/44/45/46/52/53/54/55/60/63/64/66/68），具体测试落点 `[不确定]`。
- **处置**：本裁决已按 C-3 建 §15.3（`AC|owner|expiry|reason`，32 行全量 + `REVIEW-AUDIT-01` 补齐）；请人类确认该临时格式或补附录 A。**非阻断**：表已建，DEFERRED 不再无处落；未发明任何到期规则。

### NB-2 [MINOR] `tasks.md` 视图与台账矛盾（宪法 27）

- **违反/依据**：宪法 **27**（plan/tasks 的勾选不保存状态；状态以 Spec 验收表 + 台账为准）；`AGENTS.md` §2（任务视图不拥有运行状态）。
- **事实**：`tasks.md:26` 已写 **Phase 0-R CLOSED（scoped）**、顺序 `T2→T3→T4→T5→P0`；但同一 `tasks.md:60` 仍写 **"Phase 0-R 未闭合；无 merge/Release/AC-VERIFIED"**。台账 `:599` 为 CLOSED（scoped）。
- **影响**：同一 Plan 文件内相互矛盾，读者可能按旧句阻断 T2；不影响台账状态值。
- **处置**：视图漂移，按宪法 27 以台账为准；建议作者下次编辑时改 `:60`（本次不改 `tasks.md`，任务书只允许改 SPEC/台账/报告）。

### NB-3 [MINOR] 台账"取代关系"未按宪法 28 逐条点名

- **违反/依据**：宪法 **28**（台账条目必须写明取代了哪一条，没有就写"无"）；`AGENTS.md` §4 条目格式。
- **事实**：
  - 2026-11-27 `ROLLOUT 11 rulings; B-1..B-6` 条目（`:490-523`）没有 `- 取代:` 字段；B-1 用散文说 `:465/:473/:495` 被取代，**B-3 没有点名**被取代的 AC-27 旧 `AC-VERIFIED` 流水（`:613-625` `OPS-P5b PASS` 与 `:599-611` `Phase 0-R CLOSED`）。
  - B-5 点名了"resume Phase 0-R first"（`:687`），但 `:599-611` closeout 条目里的 **"Next: T5/AC-39…68"** 也是旧顺序，未被点名。
  - B-4 只写"freeze lifted"，未记录裁决 9-A 的**"追认"**（对冻结期内已完成的 Phase 0-R/T2.4 工作的回溯确认）；建议补一句，避免历史读者误判过去的工作未获授权。
  - 本次裁决条目已补 `- 取代:` 字段，逐条点名上述流水；**历史条目原文未改写**。
- **影响**：当前状态仍唯一（最新在上 + tsv + 裁决 6/7），但历史流水与当前状态并存，依赖排序规则才能读对；属可读性/可审计性缺陷。
- **处置**：非阻断；建议后续台账整理（D-2 归档）时统一补点名，不回改历史正文。

### NB-4 [MINOR] B-6 残留与"运行状态入契约头"

- **违反/依据**：宪法 **27**（运行状态——冻结/解除——只在台账）；宪法 **I.2**（已批准 ADR 只能改错别字/失效引用）。B-6 把"解除冻结"写进 SPEC/ADR 头，性质上是**失效引用更正**，在 I.2 的例外内；但**不是决策正文**，应保持为注记。
- **事实**：ADR nowaybill `:99` 仍写"不得把本文件的 `allowed_paths` 当作**冻结 Parent** 或 R0.3-R1c 的范围扩展"——`:8`/`:98` 已改"历史父变更/解除冻结"，`:99` 残留"冻结 Parent"，与 B-6 目标不一致。SPEC-0002 `:9`/`:24` 已同步，无残留；历史记录 `:1011`/`:1038` 按任务书**不应**改，保留正确。
- **影响**：`解除冻结`未带 B-4 的范围限定（Phase 0-R 收口 + T2.4 剩余），可能被读成全量解冻；与 G-1 叠加会误导下一步。
- **处置**：非阻断；建议下次编辑将 `:99` 改为"不得把本文件的 `allowed_paths` 当作父变更范围或 R0.3-R1c 的扩展"，并在头注加"运行状态以 `docs/review-tracking.md` 为准"。

### 观察 OBS-1（不改，仅记录）`DECISIONS.md` §2 仍写"C-1 未落地"

- `docs/convert/DECISIONS.md` §2 标题"对替换稿的调整（已改，**C-1 未落地**）"与同文件 §4"**C-1 落地（2026-11-27，人类批准）**"及台账 `:467` 的 GOV-CONVERT-01 人类决定矛盾。
- 依据：宪法 27（状态副本）；**非阻断**，C-1 状态以台账 `:467` 为准。`docs/convert/**` 属本次只读范围，未修改。

---

## 4. AC 位移

> 状态词汇：未验证（tsv `DEFERRED`）/有引用（`AC-REF`）/已验证（`AC-VERIFIED`）。**本次没有把任何 AC 从"未验证/有引用"升为"已验证"**；`AC-50/69/70/71/72-BACKEND` 的 `已验证` 是此前 `reviewer-r04` 绑定 `3ce0d7d` 的既有结论，本变更只是把当前值落到 SPEC-0002 §15。

| AC | 旧状态（变更前） | 新状态（本裁决后） | 检查类型 | 证据路径 / 依据 |
|---|---|---|---|---|
| **AC-60** | 有引用（台账旧表述；tsv 原本已是 DEFERRED） | **未验证** | 测试 | `ac-coverage.tsv:29`；无 `TestAC60`（仅生产注释）；台账 `:501-504` |
| **AC-63** | 有引用（同上） | **未验证** | 测试 | `ac-coverage.tsv:32`；无 `TestAC63`（仅生产注释）；台账 `:501-504` |
| **AC-72** | 已验证（单行 `AC-72`，reason 自述"backend only"） | **`AC-72（后端段）` 已验证；`REVIEW-REQUIRED-ALERTS-01`/`ENTRY-01` 未验证** | 测试 / 可观测指标 | `ac-coverage.tsv:47-49`；`worker_test.go:14`；`metrics_test.go:11`；`docs/ops.md` |
| **AC-27**（SPEC-0001） | 已验证（waiver C+A） | **有引用** | 测试 / 可观测指标 | `ac-coverage.tsv:41`；人类裁决 6；`tasks.md:57`；台账 `:509-510` |
| AC-14（SPEC-0001） | 已验证 | 已验证（不变） | 测试 | `ac-coverage.tsv:38`；`settlement_integration_test.go:875` |
| AC-21/25/31/32（SPEC-0001） | 有引用 | 有引用（不变） | 测试 | `ac-coverage.tsv:39/40/42/43` |
| AC-50/69/70/71 | 已验证 | 已验证（不变） | 测试 | `ac-coverage.tsv:19/44-46` |
| AC-51/61/62/65/67 | 有引用 | 有引用（不变） | 测试 / 可观测指标 | `ac-coverage.tsv:20/30/31/34/36` |
| AC-39…49、52…59、64/66/68（AC-60/63 见上） | 未验证 | 未验证（不变） | 测试 / 契约检查 | `ac-coverage.tsv` 对应行 |

**SPEC-0001 基线**：AC-14 保持已验证；AC-21/25/27/31/32 为有引用（AC-27 由已验证回调为有引用，裁决 6）。SPEC-0001 正文未改；其验收状态表尚未建立，当前值暂镜像在 SPEC-0002 §15.2，**SPEC-0001 建表后应以其表为唯一当前值**（本报告 NB-1 的一部分）。

---

## 5. 阻断项 / 门禁项 / 打回点

- **阻断项（本裁决范围内）：无。** 0 BLOCKER；B-1…B-6 无一项需要打回作者。
- **门禁项 G-1（下一阶段启动前必须人类裁决）**：
  - 裁决 9-A 只放开 **Phase 0-R 收口 + T2.4 剩余**；台账 `:599` 已把 Phase 0-R 记为 CLOSED（scoped），`:525-535` 已把 T2.4 记为 scoped PASS，并明确 **`T2.5/T2.6` not done**（`tasks.md:87-89` 的 T2.5/T2.6/T2.7 未勾选）。
  - 裁决 7-B/B-5 的下一步顺序是 `T2→T3→T4→T5`，即下一步是 **T2.5**——它**不在**裁决 9 已放开的"T2.4 剩余"内，T3/T4/T5 同样不在。
  - 结论：两条人类裁决就"现在能做什么"存在范围冲突。**不是作者可修复的问题**（不得自行扩大解冻范围，宪法 17/31；不得用新 change_id 绕过，宪法 30）。请人类二选一：(a) 把解冻范围扩展到 `T2→T3→T4→T5→P0 closure`；(b) 维持只放开 T2.4 剩余，则 T2.5+ 继续冻结并重排。
  - 影响：G-1 不阻断本 Spec/状态裁决，**阻断 T2.5+ 的实现启动**。
- **打回点**：无（本次不 FAIL）。

---

## 6. 未覆盖范围（SCOPED-PASS 的边界）

1. **未复跑** MySQL 8.4.13 / NATS 2.10.29 / `-race` 集成证据；AC-`已验证`/`有引用` 的状态沿用既有 `reviewer-r04` 绑定 HEAD 的裁决，本报告只核对证据路径存在性与 1 个 unit test。
2. **未审** C-1 的宪法/AGENTS 文本实质（人类 2026-11-27 已批准生效；仅核对与状态对齐相关的一致性）。
3. **未审** C-4 归档、D-2 台账归档、13 个 addendum 合并为新 ADR（C-3 后续，需独立审查/批准）。
4. **未审** 任何实现/代码/迁移变更（本阶段 Spec）。
5. AC-22/28/29/30/35/37 只迁入 §15.3 延期清单，**未**判定其 SPEC-0001 归属与迁回时点（`[不确定]`，见 NB-1）。
6. AC-72 后端段的"24h 首次响应/7 天升级"只能由 runbook/`worker_test` 的 7 天查询证据体现；本次未重新执行其集成断言（沿用 `reviewer-r04` T0–T8 结论）。
7. **人类裁决的原始记录**：仓库内只有 `docs/convert/DECISIONS.md` 与台账的转写，无独立签署件；本次核对的是"执行是否与记录一致"，**不验证转写是否逐字忠实**（宪法 29 要求作者按原话记录；G-1 即是由转写可判定的两条裁决范围冲突）。

---

## 7. 本次落盘的文件（审查者改动）

| 文件 | 改动 | 说明 |
|---|---|---|
| `docs/specs/0002-payment-settlement-admission.md` | **新增 §15 验收状态表/延期清单**（+102 行） | 只加表与说明，**未改 AC 正文**、未改 §1–§14 |
| `docs/collab/qc-reports/REV-GOV-CONVERT-01-C3.md` | 新建（本文件） | 审查裁决报告 |
| `docs/review-tracking.md` | 顶部新增 1 条审查裁决（`:458`） | 按 AGENTS §4；历史条目未改 |

未 commit、未 merge、未改代码、未改 `.specify/memory/constitution.md`、根 `AGENTS.md`、`docs/convert/**`、`docs/collab/ac-coverage.tsv`。

---

**Actor: `reviewer-c3`（独立 subagent，非作者）｜Reviewed-by: `human`（11 项裁决与冻结范围）｜作者 `lead` 是 Actor 侧，不是 Reviewed-by。**
