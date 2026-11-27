# ROLLOUT 裁决记录

| 项 | 值 |
|---|---|
| 基线 | `HEAD 9e81497`（B 阶段改动前）；B 阶段改动后见工作区 |
| 状态 | **11 / 11 已裁决**；B-1…B-6 已执行；**C-1 已落地**；**C-3 SCOPED-PASS**；**C-4 已执行（ac-baseline 残留）**；**D-2 已归档（只移动）**；G-1 已裁决 A；**宪法 v2.2.0（L2/L3 已恢复，已批准（2026-11-27））** |
| 性质 | 人类裁决记录；不替代审查裁决、不替代 C-1 的最终落地批准 |
| 关联 | `ROLLOUT.md` §3、`A1-SELFCHECK.md` §9、`docs/review-tracking.md:458`（状态对齐条目） |

## 一、11 项裁决（2026-11-27，人类）

| # | 问题 | 裁决 | 影响 |
|---|---|---|---|
| 1 | 审查者由谁担任 | **A 外部 subagent，有运行时** | 审查者每次独立启动，可复跑 MySQL/NATS/Redis 与 `-race`；按宪法 29 写台账/验收表，不能写就输出同格式条目 |
| 2 | 合入是否委托作者 | **B 保留人类合入** | 人类批准点 = Spec、合入、Release（契约变更 3 个）；作者不执行 merge |
| 3 | PAY-SETTLE-01 过渡 | **A 走新流程，先做状态对齐** | SPEC-0002 视为已批准；B-1/B-3/B-5/B-6 先做；开放 AC 迁入 SPEC-0002 验收表；13 个 addendum 暂不合并，另起新 ADR 取代 |
| 4 | 停用文件处置 | **B 移到 `docs/archive/`，旧路径留 stub** | `CONVENTIONS.md`、`prompt/` 六个文件、`ac-coverage.tsv`、`handoff.md`、backlog、MEMORY 状态行移入归档；旧路径保留一行指针 |
| 5 | AC-72 状态 | **A 拆分** | 后端段已验证；ALERTS/ENTRY 各记一行延期 |
| 6 | AC-27 / C+A | **B 回到“有引用/未验证”** | AC-27 回 AC-REF；`tasks.md:57` 保持有效；C+A 不追认 |
| 7 | 11-27 重排 | **B 纠正重排** | 以 **T2→T3→T4→T5→P0 closure** 为准；11-27 的“先 Phase 0-R”条目被取代 |
| 8 | 台账整理 | **B 只归档不重排** | 先移旧路线图/GOV-COST/gates/pair/已取代条目；D-1 重排不做 |
| 9 | 冻结解除范围 | **A 追认并放开剩余范围** | 放开 Phase 0-R 收口 + T2.4 剩余；保留 no merge/no release；其他冻结范围不变 |
| 10 | 试点变更 | **A 下一个契约变更** | 选小型非资金变更；建议 SDD-STRUCT-ALIGN-01 的路径/模板对齐 |
| 11 | 紧急修复路径 | **A 不设** | 最短 Spec + 当日人类批准；不得用 hotfix/新 change_id 绕过 FAIL |

## 二、对替换稿的调整（已改，C-1 已落地；以下是落地前调整记录）

- `constitution.draft.md:37`（第 16 条）：人类批准点从“两个”改为“三个”：批准 Spec、合入、Release；合入由人类执行。
- `AGENTS.draft.md` §1：作者“不做合入”；人类“做合入”。
- `AGENTS.draft.md` §2 第 7 步：合入的“谁”从作者改为人类。
- `AGENTS.draft.md` §2：新增“线上紧急修复不设专门路径，最短 Spec + 当日人类批准，不得用 hotfix/新 `change_id` 绕过 FAIL”。

**未做**：没有替换 `.specify/memory/constitution.md`，没有改根 `AGENTS.md`。C-1 落地仍需人类对最终文本的明确批准。

## 三、B 阶段执行结果（2026-11-27）

| 动作 | 结果 | 证据 |
|---|---|---|
| B-1 AC-60/63 | 统一为 **DEFERRED**。全仓无 `TestAC60/63`，只有生产注释；tsv 原本正确，台账 AC-REF 被取代 | `ac-coverage.tsv:29,32` 未改；`review-tracking.md:458` 记录；注释位置 `internal/services/order/tcc_precheck.go:97`、`legacy_resolution.go:14`、`refund_admission.go:679` |
| B-2 AC-72 | 拆成 `AC-72-BACKEND`（AC-VERIFIED）+ 两个 DEFERRED 工作项；两行 reason 加 `AC-72-ALERTS/ENTRY` 前缀 | `ac-coverage.tsv:47-49` |
| B-3 AC-27 | 回 **AC-REF**；reason 注明 ruling 6；`tasks.md:57` 未改 | `ac-coverage.tsv:41` |
| B-4 冻结 | 台账记录解除范围：Phase 0-R 收口 + T2.4 剩余；no merge/no release | `review-tracking.md:458` |
| B-5 重排 | 台账记录取代关系；`tasks.md:26` 改为 T2→T3→T4→T5→P0 | `review-tracking.md:458`；`docs/lead/PAY-SETTLE-01/tasks.md:26` |
| B-6 头部同步 | SPEC-0002 `:9/:24`、ADR nowaybill `:7/:8/:98` 改为“历史父变更；2026-11-27 解除冻结”；历史记录 `0002:1011/:1038` 未动 | 工作区 diff |
| B-7 SDD-STRUCT-ALIGN-01 | 清单已交（A1 §5.4）；按裁决 10 A 作为试点候选，owner/expiry 待定 | `A1-SELFCHECK.md` §5.4 |

**注意**：B-6 改的是已批准 SPEC/ADR 的**状态注记**，不是决策正文；授权来自裁决 3 A（SPEC-0002 视为已批准、父冻结仅历史）与裁决 9 A（解除冻结）。这是过渡期的一次性动作，C-1 后不再有“父冻结”概念。

## 四、C-1 落地（2026-11-27，人类批准）

- `change_id: GOV-CONVERT-01`；`Actor: lead`；`Reviewed-by: human (Director 2026-11-27)`。
- 宪法 v2.0.0 已写入 `.specify/memory/constitution.md`：`Approved/Effective/Last Amended = 2026-11-27`，顶部替换说明已删除。
- 根 `AGENTS.md` 已替换：作者不合入、人类合入（裁决 2-B）；紧急修复不设专门路径（裁决 11-A）。
- 台账 `docs/review-tracking.md:458` 记人类批准 + A-2 取代关系。
- 未 commit、未 merge；按裁决 2-B，合入由人类执行。

## 五、C/D 待办

| 阶段 | 内容 | 依赖 |
|---|---|---|
| C-2 | 可选：给 Spec 模板加附录 A 的七个章节。建议走预设/扩展层 `append`；项目 override 是整体替换 | C-1 已完成 |
| C-3 | **已完成（SCOPED-PASS）**：SPEC-0002 §15 验收状态表（34 行 AC-39…72 + 6 行基线 + 33 行延期清单）；审查报告 `REV-GOV-CONVERT-01-C3.md`；13 个 addendum 合并为新 ADR **未做**，留作后续契约变更 | — |
| C-4 | **已完成（B 方案）**：旧文件移入 `docs/archive/2026-11-27-governance/`，旧路径留 stub；README/Spec.md/development.md/bootstrap-context.sh 已改指新位置；MEMORY 已清除 tsv/gates/38/38 等状态行。**残留**：`docs/collab/ac-baseline.txt` 未在 C-4 清单内，仍留在原路径（新模型无 AC 基线概念） | — |
| D-2 | 台账归档：旧路线图、GOV-COST-01、collab-gates、pair 线程、已被取代的 11-23/11-24 条目移入 `docs/review-tracking-archive.md`；主文件顶部留指针。**不重排** | 人类勾选“已被取代”的条目（见 G-1/D-2 问题） |

## 六、C-3 审查的门禁（需人类裁决）

审查者 `reviewer-c3` 给 **SCOPED-PASS**（0 BLOCKER），但指出：

- 裁决 9-A 只放开 “Phase 0-R 收口 + T2.4 剩余”；而 B-5 的下一步是 **T2.5**，T3/T4/T5 也不在放开范围。
- 台账显示 Phase 0-R 已 CLOSED（`:599`）、T2.4 已 scoped PASS 且 T2.5/T2.6 not done。
- 两条人类裁决范围冲突，作者不可自行扩大解冻（宪法 17/30/31）。

**待人类二选一**：扩展解冻到 T2→T3→T4→T5→P0，或维持冻结并重排。

另：审查者 MINOR 指出宪法附录 A 未定义延期清单/owner/expiry 格式；C-3 用 SPEC-0002 §15.3 作为过渡，constitution 的延期规则留作后续修订。

## 七、D-2 归档结果

- 已创建 `docs/review-tracking-archive.md`，移入：2026-11-24 宪法 v1.2.0 / PAY-SETTLE-01 裁决 B、GOV-COST-01、2026-11-25 docs IA + legacy cleanup、2026-11-24 SPEC/ADR approvals 至文件末尾（含 11-24/11-23 决定、P1/P2/P3、external review、AI roadmap、verification、unverified/drift）。
- 主文件顶部留指针；未重排；移动内容逐字保留。
- 归档后主文件 914 行、归档 439 行；标题集合与 base `9e81497` 对比无缺失（只多出 4 条 2026-11-27 新条目）。

## 八、交付验证与合入（2026-11-27）

### 【测试范围】

- 纯文档/状态迁移，未改代码。`git diff --check` exit 0。
- 审查者 `reviewer-c3`（独立 subagent）复跑：
  `GOCACHE=/tmp/gocache-rev-c3 go test ./internal/platform/metrics -run TestRefundReviewGaugesRegistered -count=1` → `ok 0.131s`（Spec 阶段证据，不代表全量测试）。
- SPEC-0002 §15 表与归档 tsv：40 条映射状态零差异（6 条 DEFERRED 基线 AC 在 §15.3，未进 §15.1/§15.2 状态表）。
- 台账归档：主文件 914 行 + 归档 439 行；标题集合与 base `9e81497` 对比无缺失，只多出 4 条 2026-11-27 新条目。
- 旧路径 stub、归档文件、活文档相对链接： touched docs 0 broken links。
- 新治理文本检索 `PAY-SETTLE-01` / `裁决 B` / `过渡条款` / `CONVENTIONS` / `ac-coverage` / `prompt/` / `L0|L1|L2` = 0 命中。

### 【未覆盖范围】

- 未跑 `go test ./...` / `make verify` / MySQL/NATS/Redis 集成 / `-race`（无代码变更）。
- 未重跑 48 条 AC 的证据；§15 表继承既有审查者证据，不新增 AC-VERIFIED。
- 未做 C-2：Spec 模板仍未通过覆盖层加入附录 A 七章节；今后用 `speckit-specify` 生成 Spec 仍需作者手工补齐。
- 未做 13 个 ADR addendum 的合并；这是一次独立契约变更，需新 ADR + 独立审查 + 人类批准，建议作为试点变更。
- `docs/collab/ac-baseline.txt` 未归档（不在 C-4 清单；新模型无 AC 基线概念），仍留在原路径。
- 历史 QC 报告、pair 存档、已批准 SPEC/ADR 里的旧路径未回溯改写；stub 保证可导航。
- §15.2 未列 AC-22/28/29/30/35/37（它们只在 §15.3 延期清单）；审查者已标 MINOR，未阻断。

### 【取舍】

- 裁决 4-B：旧文件**移动+stub**，不删除；历史引用不回改。
- 裁决 3-A：SPEC-0002 视为已批准；只加 §15 状态表，不改 AC 正文，不重写 SPEC-0001。
- 裁决 5/6/7/8/9/10/11 按记录执行；G-1 按人类追加裁决 A 扩展到 T2→T3→T4→T5→P0。
- 未 commit、未 merge；按裁决 2-B，合入由人类执行。

### 合入清单

- 新增（必须一并 `git add`）：`docs/archive/**`、`docs/review-tracking-archive.md`、`docs/collab/qc-reports/REV-GOV-CONVERT-01-C3.md`、`docs/convert/**`。
- 修改：`.specify/memory/constitution.md`、`AGENTS.md`、`README.md`、`Spec.md`、`docs/review-tracking.md`、`docs/specs/0002-...md`、`docs/adr/0007-amd1-addendum-ac50-nowaybill.md`、`docs/lead/PAY-SETTLE-01/{tasks,backlog}.md`、`docs/collab/{CONVENTIONS.md,ac-coverage.tsv}`、`docs/handoff.md`、`prompt/*.md`、`docs/{architect,developer,po,reviewer}/MEMORY.md`、`docs/development.md`、`scripts/bootstrap-context.sh`。
- 注意：如果只提交修改而不提交 `docs/archive/**`，旧文件在工作树中等于被删除、归档副本未入库；合入前必须检查 `git status` 包含 archive。

## 九、核心文件复核（2026-11-27，宪法 v2.1.0）

- 人类指示：Spec Kit 生成的 Spec 应允许其原生语法，不再强制 EARS。
- 已调整：附录 A 允许 EARS 或 Spec Kit 原生语法（Given/When/Then + FR/SC）；新增“Spec Kit 兼容”注（Acceptance Scenarios 逐条映射 AC；Spec Kit 目录名/扁平布局的 change_id 规则）；Spec 回答附录 A 问题而非固定章节；新增延期清单；change_id 与 Spec 身份分开；tasks `[X]` 只是执行进度；审查者有运行时/静态回退需人类豁免；`speckit-implement` 分批；台账同日按插入位置；尾注在合入提交；evidence 路径分流；模板覆盖层 replace vs append；实现变更第 4-8 步、plan 一页以内；新增“治理变更”级别。
- 审查：`reviewer-syntax` 首轮 **FAIL**（B1：change_id 与 SPEC-0002 已登记 id 冲突）→ 修复 → **re-review PASS**；报告 `REV-GOV-SPEC-SYNTAX-01.md`。
- 版本：v2.0.0 -> **v2.1.0**（MINOR；非 MAJOR）。人类批准最终文本后生效。
- L2/L3 已在 **v2.2.0** 恢复（`GOV-CORE-RESTORE-01`）：审查者之外的其他输入、integration owner、reviewer 元数据；permit/commitment/Σ、`database/sql`+CAS、测试替身/启动 fail-fast、`order.paid` 兼容期两层。详见 §十。
- 详细复核与建议见 [CORE-REVIEW.md](CORE-REVIEW.md)。

## 十、v2.2.0 第二批（L2 + L3，`GOV-CORE-RESTORE-01`）

- 宪法 v2.1.0 -> **v2.2.0**（MINOR：新增/恢复规则，非删除/重定义原则）。
- **L2**：宪法 12 明确审查者（独立 subagent）裁决是唯一审查结论；其他输入只作问题提案、引用当前 HEAD、`[不确定]`/文档漂移、不削弱第 9 条；AGENTS §2 integration owner（既有角色兼任）；AGENTS §3 `review_scope`/`release_status`/`spec_version`/`adr_revision`/`verification`/INV/`baseline_impact`、HEAD 变化复核、plan `baseline_impact`/`stop_conditions`。
- **L3**：宪法 15 订单/开发默认值/依赖 fail-fast/Options/测试替身/启动失败；宪法 23 II.5 两条 IF/THEN、`REVOKED` permit、commitment、Σ、`order.paid` 兼容期两层；宪法 26 `database/sql`+CAS、GORM 文件级读侧、`apperr`/`fmt.Errorf("%w")`/`docs/contracts.md`。
- 审查：SCOPED-PASS -> NB-11 逐字化 FAIL（把四项兼容要求错误限定到“灰度影子期”）-> 恢复两层结构 -> **final PASS**；报告 `REV-GOV-CORE-RESTORE-01.md`。
- 收益/停止条件已写入台账 `GOV-CORE-RESTORE-01` 条目。
- **人类已批准 v2.2.0（2026-11-27）；未 commit/未 merge**。
- 仍非阻断：宪法 23 段落较长（可读性）；“ADR 批准的版本化兼容期”未逐字出现，可由附录 B + 影子期 ADR 要求推出。

> 人类更正（2026-11-27）：不存在“外部复审”；reviewer 就是独立 subagent；其他输入才作问题提案。
