```yaml
id: SPEC-0003
title: 协作治理分级、门禁机器化与成本收敛
status: approved
version: 1.4
updated_at: 2026-11-24
owner: Product Owner
approved_by: Director
approved_at: 2026-11-24
domain: platform
supersedes: []
superseded_by: null
adr: [0008]
```

# SPEC-0003 — 协作治理分级、门禁机器化与成本收敛（v1.4 approved）

| 项 | 值 |
|---|---|
| 版本 | v1.4（**approved** 2026-11-24；基于已批准 v1.3，仅修订 bootstrap 证据口径并并入 QC-05 的 7 项 MINOR 编辑性澄清；v1.3 的 approved 历史保留于 §13/§14） |
| 日期 | 2026-11-24 |
| 状态 | **approved v1.4**（QC-05 READY 0/0/7：0 BLOCKER/0 MAJOR/7 MINOR，7 项 MINOR 已作为编辑性澄清并入本批准稿，见 §14）：依 Director 对 `ADR-0008-QC-01` Q-02 [CONTRACT_GAP] 的书面裁决（`docs/review-tracking.md:117-136`，“2026-11-24 human decisions: GOV-COST-01 bootstrap, IV and commit timing”）把 AC-106/AC-112 的 bootstrap 证据改为 legacy **G-1…G-3 PASS**（显式 `--range=<BASE^>..<BASE>`、scoped AC-01…AC-68）+ 显式 `G-4 NOT-RUN(not-implemented-at-BASE)` 记录（不阻断 P0a/P0b、永不计 PASS）、G-4 于 P0b 落地；`approved_by=Director`、`approved_at=2026-11-24`；v1.3 批准证据（`docs/review-tracking.md:94`，D-CR-1…D-CR-5，approver Director）不变 |
| 位置 | `docs/specs/0003-platform-governance-cost-reduction.md` |
| 作者 | Product Owner |
| 下游 | System Architect（治理落点：`AGENTS.md`、`docs/collab/CONVENTIONS.md`）；Developer（仅门禁执行器实现，受 AC-69…AC-74、AC-99/AC-100/AC-103 约束）；治理落点 ADR：ADR-0008（`docs/adr/0008-governance-cost-reduction.md`，approved 2026-11-24）；不启动业务 ADR |
| 依据 | 宪法 `:61-69`、`:75-84`、`:88-105`；`AGENTS.md:31-36`、`:74-77`、`:109-116`、`:120-126`；`docs/collab/CONVENTIONS.md:140-151`、`:157-163`、`:179-210`、`:225-234`；`docs/specs/README.md:9-55`；SPEC-0002 结构 `:67-104`、`:108-134`、`:583-600`、`:604-640`、`:684-710`；提案 `:60-114`；`docs/review-tracking.md:94`（D-CR-1…D-CR-5）、`:117-136`（GOV-COST-01 bootstrap 裁决）；QC-01/QC-02/QC-03/QC-04 全文与 QC-05（`docs/collab/qc-reports/SPEC-0003-QC-05.md`）；`ADR-0008-QC-01` Q-02/Q-10 |
| 范围 | L0/L1 封闭枚举 + L2 fail-closed 默认、错分停线、L1 审批矩阵、QC 差分审查与强制全量节点、门禁执行器完整性（G-1…G-4 / Review vs Release / 显式 range / 措辞 / CI 覆盖 / 自保护）、G-3 引导、scaffold 主权声明、内容摘要与 change_id 注册表；不改变任何业务 AC/INV |
| 被审版本绑定 | 已批准 v1.3：**689 行、sha1 `75eb8f7d52e0b7d8d1f1c5e43237e61979dca263`**（canonical：`docs/adr/0008-governance-cost-reduction.md:10`、`:797`；`docs/collab/qc-reports/ADR-0008-QC-01.md:7`）；QC-04 所审 v1.3 稿 sha1 `15ad29798f3c07ba6d6ac7571c9f3e5790de3061`、HEAD `f64434a90f21a523de2524676ef5eb228af615fa`；v1.3 批准稿仅应用 QC-04 M-1…M-4（字段/证据格式的编辑性澄清），AC/INV 编号与语义不变。v1.4：QC-05 所审稿 **sha1 `79db93233120fcfce9f5bd4b9a0c125b183d8d28`**（696 行，in-review 稿）；获批 v1.4 正文的 sha1 于落库时由 PO 计算并登记 GV-S20——正文不内嵌自身摘要（同 v1.3 的“批准稿摘要不自引用”口径）；v1.4 仅改 AC-106/AC-112 的 bootstrap 证据、GV-S21 的 `g4_status` 字段、INV-75/AC-100 的括号口径、§5.1/§6/§9/§12 交叉引用与头部/§13/§14 记录，AC 编号与其余 AC/INV 文本不变 |

> 本文件只定义治理业务规则、状态所有权、不变量与验收标准（EARS）。**不定义**脚本内部算法、代码结构、CI YAML 细节或存储实现；把治理产物（如门禁执行器、台账、CI 工作流）作为可观察边界是允许的。提案中的成本数字均为**估算**且未被计时/计量（`docs/cost-reduction-proposal.md:145`），不得作为 AC 阈值。

---

## 0. 一页决策摘要

1. **三级风险分级，L0/L1 是封闭枚举，其余一律 L2**：L0 只列非治理文档/注释/格式且无语义变化；L1 只列单服务内既有契约下的业务行为修正；宪法、AGENTS、CONVENTIONS、任何 Spec/ADR、治理/工具链、`scripts/**`、`.github/**`、`Makefile`、`internal/platform/**`、全部 canonical 载体（含新建）与未知/未列举变更一律 L2。不得新增子级或第四风险等级；判据不足按 L2 + 人类裁决。
2. **错分即停线**：发现 L2 被当 L0/L1 执行，冻结 `change_id`，按 L2 重走全流程，并记入 Reviewer 错分台账；不得以新 `change_id` 或 `hotfix:` 绕过。
3. **仅 L1（封闭业务枚举）阶段批准委托给 QC + Reviewer 联签**（D-CR-2，非 Reviewer 单签）；L1 不要求 Plan。L2（含全部治理/工具链/canonical）必须走 Spec/ADR/QC/人类书面批准，Plan/Tasks 与人类书面 Plan 批准，Release 人类书面放行；merge 仅可人类知会/裁决。
4. **QC 差分审查是有边界豁免**：小版本可只审 delta 与直接关联章节，但报告必须列出未复审章节；主版本升级与 Release 门禁前两次全量复审不可省略。
5. **门禁机器化补全**：执行器必须输出 G-1…G-4、区分 Review/Release 结论、绑定显式 `base..head`、使用 AC-REF/AC-VERIFIED 语义；CI 触发路径必须包含 `docs/**`、`prompt/**`、`.specify/**`；门禁/CI 自身变更必须由基线版本或独立人工 diff 复核保护，并由外部检测者在未运行时发出 `GATE_NOT_RUN`。
6. **六条红线不动**：R-1…R-6 原样保留为 INV-51…INV-56；EARS、fail-closed、stop-the-line 与现有业务保证均不降低。
7. **业务基线不动**：AC-01…AC-68、INV-01…INV-50 的语义、编号与验收要求不受本 Spec 影响；治理优化不得成为绕过门禁或重分级风险的工具。
8. **证据可执行**：治理 AC 走 G-3 的 `test_type/evidence_path/status` 证据路径；G-3 扩展先于任何治理 AC 的 AC-VERIFIED 声明；未扩展前治理 AC 为 NOT-RUN、Release 阻断，不批量延期。L1 联签前必须有真实执行证据或人类书面豁免。
9. **内容绑定**：所有 QC/Review/Release 结论绑定被审内容的版本与摘要；同版本不同内容一律重审。
10. **总函数封闭**：未来新建的 canonical 载体、治理 Spec、模板、台账、注册表或路径，未显式列举即 L2 并交人类裁决。

---

## 1. 范围与非目标

### 1.1 本期范围

- L0/L1 封闭枚举 + 其余一律 L2 的全域总函数、未知/新建载体回退、登记、复核与争议裁决（GOV-TIER-01）。
- L1 的产物×批准者矩阵、联签阶段点、QC 签署对象、Plan/Tasks 适用性与人类批准边界；L2 全流程与人类书面批准（GOV-HUM-01）。
- 错分（尤其 L2→L1）的停线、冻结、按 L2 重走与台账记录（GOV-TIER-01 / D-CR-4）。
- L1 阶段批准委托为 QC + Reviewer 联签；L2 与治理/工具链变更保留人类书面批准域（GOV-HUM-01 / D-CR-2）。
- QC 差分审查的最小范围、未复审章节声明与两个强制全量复审节点（GOV-QC-01 / D-CR-3）。
- 门禁执行器补齐 G-4 与 Release 语义、显式 commit range、AC-REF/AC-VERIFIED 措辞、CI 治理路径覆盖与门禁自保护（GOV-CODE-01 / D-CR-5）。
- G-3 的治理证据路径、change_id 注册表、内容摘要、审计载体与 Release 全量 QC 对象集。
- plan/tasks/evidence-matrix 骨架生成与强制主权声明行（GOV-SCAF-01）。
- 上述规则的异常终态、失败可见性、幂等键、证据与基线影响。

### 1.2 明确不做（Non-goals）

- 不取消或简化 subagent 自读宪法原文（红线 R-1）。
- 不动契约先行与 Phase 0 门禁（红线 R-2）。
- 不合并、移动或复制任何唯一台账与事实所有权（红线 R-3）。
- 不为降成本放宽 EARS 强制、fail-closed 或 stop-the-line。
- 不引入新的机器门禁类型（G-5/G-6）；只补齐已定义的 G-1…G-4（`docs/collab/CONVENTIONS.md:188`）。
- 不通过修改分级判据把高风险任务降级为低风险任务（红线 R-6）。
- 不通过伪造测试数据、假绿或隐瞒未覆盖范围推进流程（红线 R-4）。
- **明确禁止**：用本 Spec 绕过任何门禁，或用“治理优化/分级/封闭枚举”重新分类风险以逃避 Spec/ADR/QC/人类批准/Plan/Reviewer；禁止新增任何子级；禁止把提案估算当成 AC 阈值。
- 不定义脚本算法、代码结构、CI YAML 细节、存储结构或权限实现；这些由对应 owner 在人类批准后设计。

### 1.3 现状复核（提案主张 vs 仓库事实，审计日 2026-11-24）

> 提案是静态快照（`docs/cost-reduction-proposal.md:7`）。按 AGENTS §7 与 CONVENTIONS §2.1（`AGENTS.md:124`；`docs/collab/CONVENTIONS.md:62-66`），代码/仓库事实优先，漂移如下登记。

| # | 提案主张 | 审计事实 | 证据 | 判定 |
|---|---|---|---|---|
| 1 | G-4/`--release` 未实现；默认范围 `HEAD^..HEAD` | 均属实：执行器自述仅 G-1/G-2/G-3 且无 `--release`；未传 `--range` 时取 `HEAD^..HEAD`（无父提交退化为 `HEAD..HEAD`） | `scripts/collab_gates.py:2`、`:61-69`、`:370-377`；`docs/collab/CONVENTIONS.md:202-208` | 一致 |
| 2 | 输出仍写 `AC coverage`（提案称 `:339`） | 措辞属实，但行号为 **`:362`**；提案行号已漂移 | `scripts/collab_gates.py:362`；`docs/cost-reduction-proposal.md:62` | **行号漂移** |
| 3 | CI paths 不含 `docs/**` 与 `prompt/**` | 属实；且 `.specify/**` 与根 `AGENTS.md` 同样不在触发路径；`scripts/**`、`Makefile`、workflow 自身会触发，但执行器是同一 diff 自证 | `.github/workflows/go.yml:4-23`；`scripts/collab_gates.py:17-28`、`:390-397` | 一致（缺口更大；见 AC-100 自保护） |
| 4 | 本工作区无 git 环境；治理 head `3adfe2`；路径 `docs/Lead/...` | 不属实：`.git` 存在且 `git status/log` 可执行（QC-01 复核 `6716929`；QC-02 复核 `f64434a`）；plan 写 `3afdfe2`，目录为 `docs/lead/`（小写） | `docs/collab/qc-reports/SPEC-0003-QC-02.md:6`；`docs/lead/PAY-SETTLE-01/plan.md:8`；`docs/cost-reduction-proposal.md:7` | **过时/哈希/大小写漂移** |
| 5 | 仓库共存 16 份 QC 报告 | QC-01 时点 26 个文件/23 个 QC 命名；QC-02 复核 28 个文件/24 个 QC 命名 | `docs/cost-reduction-proposal.md:31`；`docs/collab/qc-reports/SPEC-0003-QC-02.md:71` | **计数漂移（按各时点口径）** |
| 6 | “仅实现 6-1/6-2/6-3” | 实为 G-1/G-2/G-3；G-3 完整能力为：测试引用（仅 `_test.go`）、过期/未过期延期、baseline 删除、连续性、新增 AC 未入 baseline | `scripts/collab_gates.py:2`、`:299-367`、`:310-322`；`docs/collab/CONVENTIONS.md:204-207` | **编号笔误（能力清单已补全）** |

- 准确表述：`docs/**`、`prompt/**`、`.specify/**`、根 `AGENTS.md` 变更绕过 CI 门禁；`scripts/**`、`Makefile`、workflow 会触发，但由被审 diff 中的执行器自证，构成 Q-04 自保护缺口（`.github/workflows/go.yml:4-23`、`:9-18`；`scripts/collab_gates.py:390-397`）。CI runner 实际触发未在本地实跑，标 **[不确定]**。
- 提案的“5-10 倍 / 2-3 倍 / 节省 40-60%”等均为**估算**（`docs/cost-reduction-proposal.md:16-19`、`:88`、`:145`），本 Spec 只采纳其行为要求，不采纳为阈值。

---

## 2. 状态所有权（State Ownership）

> 治理状态字段编号 `GV-S1…GV-S22`，与业务 S1…S19 分离。**一个状态字段只有一个写入者**；`GV-S20` 是 append-only 共享日志，按行归属，不适用“单字段单写者”，但只能有一个 canonical 指针。读取者不得回写。

| 字段 | 唯一事实来源/写入者 | 取值/含义 | 谁消费 | 禁止承担的隐含语义 |
|---|---|---|---|---|
| GV-S1 变更级别 `change_tier` | Lead（写入变更输入包） | L0 / L1 / L2；L0/L1 仅限 AC-76/77 的封闭枚举；其余全部 L2；记录判据、证据与 L2 例外 | Reviewer、人类、门禁 | 不表示批准或风险接受；Dev/实现方不得写入或自报；不得新增子级/第四级；GV-S17 升级后以 effective_tier 为准 |
| GV-S2 分级复核 `tier_review` | Reviewer（review verdict 的一部分） | CONFIRMED / MISMATCH / NEEDS_HUMAN | Lead、人类 | 不改写 `change_tier`；不是人类裁决；不是 Spec 批准 |
| GV-S3 变更输入包 | Lead（base/tier/spec/adr/owner/impact/AC/allowed_paths/rollback）；Reviewer 在 Review 时追加 `head` | canonical 路径 `docs/lead/<change_id>/change-package.md`；表头：`change_id/base/head/tier/spec_version/adr_revision/integration_owner/baseline_impact/affected_AC/allowed_paths/rollback`；实施前写入，Review 后只追加 | Reviewer、人类、门禁 | 不是契约；不是批准；`head` 为空时 AC-71 不得声称范围已绑定 |
| GV-S4 门禁机器结论 `gate_verdict` | 门禁执行器 | G-1…G-4 各自 PASS/FAIL/NOT-RUN，附 range、mode、被审内容摘要、证据 | Reviewer、Release 授权人 | 不证明语义正确、测试已执行或可发布（`docs/collab/CONVENTIONS.md:179-190`） |
| GV-S5 Review 裁决 `review_verdict` | Reviewer | PASS/FAIL + `review_scope` + 精确 base/head + reviewed_digest（`docs/collab/CONVENTIONS.md:140-151`） | Lead、人类 | 不等于 Spec 全量验收、merge 或 Release PASS |
| GV-S6 Release 裁决 `release_verdict` | 人类指挥官 | PASS / BLOCKED + 精确 base/head + release_scope + 内容摘要 + 证据清单 | 全员 | 不得由机器/Reviewer/Lead 代写；不自动继承到后续提交 |
| GV-S7 QC 覆盖声明 `qc_coverage` | Quality Checker（QC 报告） | delta/full、reviewed_digest、已复审章节、未复审章节清单、受影响 INV/AC | Reviewer、人类 | 不是“全量已审”；缺未复审清单时不得视为全量 |
| GV-S8 AC 台账 `ac-coverage.tsv` | Reviewer（`docs/collab/CONVENTIONS.md:31`、`:161`；宪法 `:47`） | 现有 8 列；`status ∈ {REF,PENDING,VERIFIED,DEFERRED}`；新规则只作用于 AC-69…AC-112，业务 AC-01…68 保留原有 AC-REF/延期语义 | 全员 | 不是 Plan 视图；不得有第二台账；AC-REF 不等于 AC-VERIFIED；`VERIFIED` 需 Reviewer 签名与摘要 |
| GV-S9 豁免记录 `waiver` | 人类指挥官（书面；AI 只引用） | 范围、期限、受影响 INV/AC、批准人、日期、替代控制、撤销时间（宪法 `:104`） | 全员 | AI 不得创建、推断或扩大；无记录/过期/撤销即不存在 |
| GV-S10 错分台账 `mis_tier_ledger` | Reviewer | canonical 载体：`docs/review-tracking.md` 的 change freeze 区（不存在时由 Lead 按 AC-109 建立）；追加行：`change_id；actual_tier；discovered_at；discoverer；action；re_review_plan；evidence`；发现时立即写入 | Lead、人类 | 不是新 `change_id` 的依据；不得删除或静默关闭；不与冻结登记分离 |
| GV-S11 门禁输入范围 `gate_range` | 发起门禁者（Review：Reviewer；Release：人类授权人） | 显式 `base..head` + mode；GV-S3 的 base 是计划基线，head 由 Reviewer 在 Review 时追加 | 门禁执行器 | 不得对 Review/Release 默认 `HEAD^..HEAD`；默认范围只作本地提示；与 GV-S3/GV-S5 不一致时必须重跑 |
| GV-S12 `change_id` | Lead | 唯一标识；与 base/head、Spec/ADR 版本及内容摘要绑定；分配前查 GV-S19 | 全员 | 不得复用；不得用于绕过冻结 |
| GV-S13 scaffold 骨架结构 | Lead | 由 `.specify/templates/plan-template.md`、`tasks-template.md`、`evidence-matrix-template.md` 生成；记录 `template_version`；最小字段见 AC-90 | 全员 | 生成不等于批准；不得改变 owner 或门禁；模板缺失即失败 |
| GV-S14 产物内容 | 各产物 owner（PO/Architect/Developer/Reviewer，`docs/collab/CONVENTIONS.md:20-31`） | Spec/ADR/Plan/QC/Review 的实际内容 | 全员 | 生成器/Lead 不得代写；不得制造第二台账 |
| GV-S15 人类批准记录 | 人类指挥官 | 仅书面批准：`approved_by/approved_at/scope/expiry/evidence/digest`；Spec/ADR/Plan（仅在要求时）/Release/bootstrap 适用；bootstrap 的 `expiry` 必填（AC-112），非 bootstrap 批准的 `expiry` 可为空 | 全员 | 不得代签；沉默/默认/口头同意不等于批准；“知会”不写入本字段；`expiry` 不得推断、补填或自动续期 |
| GV-S16 L1 联签记录 | QC 写 `qc_signoff`；Reviewer 写 `reviewer_signoff` | 每方记录 `signed_at`、`stage`（必须为 `post_review_pre_merge`）、`delta_digest`；`SIGNED` 由二者同时存在派生 | 全员 | 不得代签；派生状态不引入第二写入者；tier 升级/冻结即失效；不等于 Spec/ADR/Release 批准 |
| GV-S17 人类分级裁决 `tier_ruling` | 人类指挥官 | `CONFIRM` / `UPGRADE_L2` + 理由 + 日期 + 证据；仅在 MISMATCH/NEEDS_HUMAN 时写入 | Lead、Reviewer、门禁 | 不由 Lead/Reviewer 代写；不改写 `change_tier` 原始记录；是唯一升级事实 |
| GV-S18 门禁阻断原因 `release_block_reason` | 门禁执行器 | `GATE_NOT_RUN` / `GATE_INCOMPLETE` / `GATE_ERROR` / `GATE_TIMEOUT` / `G3_BOOTSTRAP_REQUIRED` / `RELEASE_BLOCKED` + 证据 | 人类、Reviewer | 不是 `release_verdict`；不得由机器写 Release PASS；人类裁决单独写 GV-S6 |
| GV-S19 `change_id` 注册表 | Lead | canonical 路径：`docs/collab/change-id-registry.tsv`；列：`change_id/repo/base/head/tier/status/refs/owner/opened_at/closed_at`；分配前追加，关闭时只追加状态 | 全员 | 不是批准；不得覆盖历史行；不得只查当前分支 |
| GV-S20 治理审计日志 | append-only 共享日志：每行由该结论 owner 写入 | canonical 载体：`docs/review-tracking.md` 的 `Governance audit log` 区（不存在时由 Lead 按 AC-109 建立）；追加行：`timestamp；change_id；actor；base/head；mode；verdict；evidence；digest；supersedes`；`evidence` 对 bootstrap 必须承载 `approver=<actor>；date=<YYYY-MM-DD>；expiry=<YYYY-MM-DD>；scope=<P0a/P0b 范围>`（AC-112）；git 历史为最终事实 | 全员 | **不是状态字段**：按行归属，不适用单字段单写者；不得删除/覆盖他人行；更正只追加；bootstrap 行缺 approver/date/expiry/scope 即无效 |
| GV-S21 独立门禁复核记录 | Independent Verifier（IV，独立验证者；actor ≠ 变更作者/Lead） | canonical 路径：`docs/collab/gate-runs/<change_id>/<base>-<head>.md`；字段：`verifier/designated_by/independence_evidence/timestamp/base/head/baseline_result/post_change_result/baseline_digest/post_change_digest/diff_conclusion/g4_status/GATE_NOT_RUN?`；`g4_status` 枚举 `PASS\|NOT-RUN(not-implemented-at-BASE)\|NA`（AC-106/AC-112）；bootstrap scoped 运行另必填 `ac_scope_evaluated/ac_scope_excluded/scope_filter_command`（AC-112） | 人类、Reviewer、门禁 | 不是发布批准；不得由被审 diff 内执行器自证；无 Director 指定/独立性证据即不存在；两次结果不一致时必须绑定双摘要并阻断 |
| GV-S22 Release 对象集 `release_scope` | release owner（Director；或 Director 书面指定的 integration owner） | canonical 路径：`docs/lead/releases/<release_id>/scope.md`；字段：`release_id/release_owner/designated_by/designation_evidence/specs/adrs/versions/digests/baseline_impact`；全量 QC 前写入 | QC、Reviewer、人类 | 不是 Release 裁决；不得由 QC/Reviewer 代写；无 `designated_by/designation_evidence` 即不得开工；对象集变化必须重写并重审 |

**L0/L1/L2 批准矩阵（产物 × 批准者 × 时机）**

| 产物/阶段 | L0 | L1（仅封闭业务枚举） | L2（默认，含治理/工具链/canonical） |
|---|---|---|---|
| 变更输入包 | 必需（change_id/diff） | 必需；Lead 写，Reviewer 追加 head | 必需；Lead 写，Reviewer 追加 head |
| Plan/Tasks | 不要求 | **不要求** | **要求**；人类书面批准（GV-S15）后才能写业务代码（宪法 `:82`） |
| Spec/ADR | 不要求 | 不新开；引用既有条款 | **要求**；人类书面批准 Spec/ADR（GV-S15） |
| QC | 不要求 | QC+Reviewer 联签：Reviewer verdict PASS 后，QC 签治理声明（无新增契约/无 L2/受影响条款/执行证据），Reviewer 再签；`stage=post_review_pre_merge` | QC 全量/delta（按 D-CR-3）；人类批准；Plan/Reviewer |
| 代码/配置/测试 | 不要求（仅文档/格式） | Reviewer verdict PASS + scope + reviewed_digest | Developer 实现；Reviewer verdict；L2 证据 |
| Merge | 人类知会/裁决 | 人类知会/裁决（非书面批准字段） | 人类知会/裁决 |
| Release | 人类书面批准 | 人类书面批准 | 人类书面批准（GV-S15） |
| 自保护/独立复核 | 不要求 | 不要求（不触碰门禁/CI） | 触碰 `scripts/**`、`.github/**`、`Makefile`、canonical/门禁时，必须 AC-100/AC-107 |

补充约束：

- 同一变更的 `change_tier` 写入者（Lead）与 `tier_review` 写入者（Reviewer）不得为同一 actor；违反时以 Actor 冲突处理（宪法 `:84`、`:92`）。
- **effective_tier 派生**：`effective_tier = (GV-S17 = UPGRADE_L2) ? L2 : change_tier`。`GV-S1` 是历史初判，`GV-S17` 是唯一升级事实；门禁、Reviewer、Lead 必须读 `effective_tier`，不得只读 `GV-S1`（AC-108）。
- `mode=warn` 的零退出码不构成 PASS：当前执行器仅用退出码区分 warn/strict，失败信息照常输出（`scripts/collab_gates.py:390-397`）；Review/Release 结论只接受 strict 且范围绑定的 PASS。
- 门禁执行器只拥有 GV-S4/GV-S18，不拥有 GV-S5/GV-S6；人工语义判断不得写进机器结论，机器结论也不得升级为语义批准（`docs/collab/CONVENTIONS.md:190`）。
- Release 结论拆成两个字段：人类写 `release_verdict`（GV-S6），门禁只写 `release_block_reason`（GV-S18）；同一结论不得出现第二写入者。
- GV-S3/GV-S10/GV-S19/GV-S20/GV-S21/GV-S22 的写入时机分别为：实施前（head 于 Review 时）、发现错分时、分配 `change_id` 前、结论产生时、独立门禁复核时、全量 QC 前；任何补写必须标注补写原因，不得倒填时间。
- **canonical 载体不是 L0/L1（作为变更对象时）**：`docs/review-tracking.md`、`docs/collab/ac-baseline.txt`、`docs/collab/ac-coverage.tsv`、`.specify/templates/**`、`docs/lead/**`（change-package/release scope）、`docs/collab/change-id-registry.tsv`、`docs/specs/README.md`、`docs/adr/**`、任何新建 canonical 工件的**结构/规则/既有内容改写**是 L2；但一次变更自身产生的 per-change 记录按 AC-111 继承父变更 tier，不计为独立 L2。未列入的文件/新建工件按 AC-97 的“未知 ⇒ L2”处理。
- **canonical 所有权**：每个 canonical 载体必须有唯一 owner、路径、写入时机与格式；新建前必须登记到 G-1 `MANAGED` 并在 AC-109 记录，否则不得首次写入。
- **G-3 引导**：G-3 扩展必须先于任何治理 AC 的 AC-VERIFIED 声明；扩展上线前治理 AC 为 NOT-RUN、Release 阻断（AC-106）。
- **review-tracking**：当前 D-CR 记录在 `docs/review-tracking.md:94`；QC-01 时点不存在本记录，不得把当前行号倒填为历史证据（M-01）。
- `docs/review-tracking.md` 当前有既有 human decisions 区（`:60`、`:94`、`:117`）；`Change freeze`/`Governance audit log` 区由 Lead 按 AC-109 建立，建立前以文件末尾追加并显式标注区名，不得用其他文件替代。

---

## 3. 治理不变量（INV-51…INV-81）

> 业务基线 INV-01…INV-50 与 AC-01…AC-68 继续有效（`docs/collab/ac-baseline.txt:68`；SPEC-0002 `:108-134`）。本节只新增治理不变量；冲突时以更严格者为准。

| ID | 不变量 | 反面即违规 |
|---|---|---|
| INV-51 | **R-1 原文自读**：每个 subagent 必须自读宪法、AGENTS、CONVENTIONS 与自身角色 prompt，禁止以 Lead 摘要替代原文；治理文件变更后必须记录 delta audit（宪法 `:90-95`；AGENTS `:27`） | 只读摘要即开工；不记录 delta audit；以“未看到新版本”免责 |
| INV-52 | **R-2 契约先行**：当变更要求 Plan（L2/P0/跨服务/资金/库存/权限/状态机）时，Plan 未批准不得写业务代码；Phase 0 不写业务代码；L1 无 Plan 门但不得据此绕 L2（AGENTS `:76`；宪法 `:82`） | 先写码后补契约；以紧急为由跳过 Plan；用“L1 无 Plan”推进 L2 |
| INV-53 | **R-3 事实所有权与唯一台账**：`docs/collab/ac-coverage.tsv` Owner=Reviewer；Plan/evidence-matrix 只是视图，不产生第二台账（`docs/collab/CONVENTIONS.md:31`、`:161`） | 双台账；Lead/Dev 写 AC 状态 |
| INV-54 | **R-4 测试与验证不可伪造**：严禁假 Mock、假绿、隐瞒异常分支；L1 联签前必须有真实执行证据或人类书面豁免（AGENTS `:113`；宪法 `:47-49`） | 用 AC-REF 冒充 AC-VERIFIED；删除异常用例；G-3 PASS 代替执行证据 |
| INV-55 | **R-5 安全边界与 Fail-Closed**：任何级别变更都不得降低资金/库存/履约/权限的防御标准；由宪法 I.2、AGENTS §0.3 与 ADR/Plan 的 fail-closed 声明承载，本 Spec 不新增 AC（宪法 `:19`；AGENTS `:13`） | L1 关闭鉴权/吞异常/默认放行 |
| INV-56 | **R-6 分级判据与核心契约不可越权篡改**：判据只存在于人类批准的版本化 Spec；任何 Agent 不得私改判据或重分级以逃避审查（提案 `:54`） | 实现里加“本类可走 L1”白名单 |
| INV-57 | **分级全域总函数**：每个变更恰好一个标签 L0/L1/L2；L0/L1 是 AC-76/77 的封闭枚举；其余全部 L2；覆盖业务、治理、工具链、canonical、无服务变更；命中任一 L2 判据即至少 L2；判据不足/有争议按 L2 + 人类裁决 | 未分级先做；L2 当 L1/L0；按“改动行数/只改文档”降级；把治理/canonical 当 L0 |
| INV-58 | **分级立法与执行分离**：Dev 不得自报级别；Lead 只按批准判据初判；判据变更须人类批准升版 | Dev 决定 tier；Lead 私自新增 L1 豁免 |
| INV-59 | **错分停线不可绕过**：错分必须冻结 `change_id`、按 L2 重走、记台账；不得以新 id/hotfix/拆提交绕过（D-CR-4） | 换个 `change_id` 继续；hotfix 扩范围 |
| INV-60 | **四种结论语义不可替代**：AC-REF ≠ AC-VERIFIED；Review PASS ≠ Spec 全量验收 ≠ merge ALLOWED ≠ Release PASS；Release 必须绑定精确范围与内容摘要（`docs/collab/CONVENTIONS.md:140-151`、`:157-163`） | 用 CI 绿/线程 CLOSED 当 Release PASS；同版本不同内容复用结论 |
| INV-61 | **门禁覆盖治理路径且无静默跳过**：`docs/**`、`prompt/**`、`.specify/**` 变更必须触发门禁；未触发即 NOT-RUN，不得当 PASS（D-CR-5） | 只改治理文档不跑门禁；跳过当绿 |
| INV-62 | **L1 委托边界**：仅 L1 封闭业务枚举可委托 QC + Reviewer 联签；L1 不要求 Plan；L2/治理/工具链/canonical 必须人类书面批准 Spec/ADR/Plan/Release（D-CR-2；宪法 `:82`、`:93`） | 仅 Reviewer 单签；机器自动批准；把治理/工具链当 L1；用“知会”替代书面批准 |
| INV-63 | **QC 强制全量节点不可绕过**：主版本升级与 Release 前必须全量复审；delta 报告必须列未复审章节（D-CR-3） | delta 审查直接 Release；省略未复审清单 |
| INV-64 | **治理产物唯一台账与 scaffold 不改变所有权**：骨架必须含主权声明；生成不等于批准；不得产生第二台账（GOV-SCAF-01） | 生成稿当批准；脚手架另立台账 |
| INV-65 | **豁免书面且可核查**：人类书面记录范围、期限、受影响 INV/AC、撤销；缺失/过期/撤销/超范围即不存在；AI 不得推断（宪法 `:104`） | 口头豁免；自动续期；撤销后继续引用；AI 自造豁免 |
| INV-66 | **治理变更可追溯**：分级、联签、门禁、QC、豁免、错分、Release 结论必须绑定 `change_id`、actor、时间、范围、内容摘要与证据；审计载体为 `docs/review-tracking.md` 追加区与 git 历史；不得删除历史（AGENTS `:70`） | 事后补记录；倒填时间；覆盖旧结论 |
| INV-67 | **未知/未列举 ⇒ L2**：任何未在 AC-76/77 显式列举的文件、路径、canonical 载体或新建工件，一律 L2 并交人类裁决；不得以“新文件/模板/台账/只改文档”定 L0/L1 | 新建治理工件未分级；未知路径按 L0；未来载体绕过 L2 |
| INV-68 | **L1 矩阵闭合**：L1 不要求 Plan；联签在 Reviewer verdict PASS 后、merge 前；QC 签治理声明与执行证据，不签代码质量；L2 Plan 必须人类书面批准 | L1 跳过 QC/Reviewer；QC 代签代码审查；L2 无 Plan 开工；L1 扩大到治理/canonical |
| INV-69 | **G-3 承载治理证据且不批量延期**：G-3 扩展读取 `test_type/evidence_path/status`；治理 AC 需 `VERIFIED` + Reviewer 签名 + 摘要；未 `VERIFIED` 即 NOT-RUN/FAIL；不得把 AC-69…AC-112 整批登记为延期 | 把 44 条治理 AC 批量 deferred；用 G-3 PASS 冒充执行证据；扩展未上线就宣称 VERIFIED |
| INV-70 | **门禁/CI 自保护**：修改门禁执行器、CI paths、Makefile 门禁接线或 AGENTS 时，必须基线+变更后两次门禁 + 独立 diff 复核（actor ≠ 作者/Lead）；未运行时由 Independent Verifier 发出 `GATE_NOT_RUN` | 被审 diff 中的执行器把自己改成恒 PASS；两次 PASS 就免 diff；CI 被停用后无人发现 |
| INV-71 | **内容摘要绑定**：所有 QC/Review/Release 结论绑定 `version + digest`；版本号相同但内容摘要变化一律视为新版本重审 | 同版本原地改写；复用旧 QC 结论 |
| INV-72 | **change_id 全 ref 唯一**：注册表覆盖本仓库全部 ref（含未合并/已删除分支的 reflog）；分配前查注册表；G-1 扩展前以人工核对+证据兜底 | 只查当前分支；复用已删除分支的 id；无注册表 |
| INV-73 | **书面批准与知会分离**：Spec/ADR/Plan（仅在要求时）/Release 必须人类书面批准并记录 `approved_by/approved_at/scope`；只有 FAIL 知会与 merge 知会可为“知会” | 用“知会”替代书面批准；L1 无 Plan 却要求/伪造 Plan 批准 |
| INV-74 | **Release 全量 QC 对象集**：Release 全量 QC 覆盖 `release_scope` 的全部 Spec/ADR 及其当前版本+摘要，不是单个 Spec；“一次”指一份覆盖该对象集的 Release QC 记录 | 用单 Spec 全量审冒充整批 Release；对象缺失仍放行 |
| INV-75 | **G-3 引导顺序**：GOV-CODE-01 的 G-3 扩展必须先上线，任何治理 AC 才可标 AC-VERIFIED；扩展上线前治理 AC 为 NOT-RUN、Release 阻断；bootstrap 用 `BASE` 上 legacy G-1…G-4（bootstrap 语境按 AC-106：G-1…G-3 PASS + G-4 NOT-RUN(not-implemented-at-BASE)；scoped AC-01…68）+ Independent Verifier diff + 人类书面批准 + 摘要，不得用未上线的 G-3 自证 | 先宣称 VERIFIED 后补 G-3；以 G-3 未上线/预期失败启动 P0；bootstrap 自证；两个复核者 |
| INV-76 | **独立验证者**：门禁/CI/Makefile 变更的独立复核必须有 actor（Independent Verifier）、canonical 路径、记录格式，且 actor ≠ 变更作者/Lead；两次门禁结果不一致时绑定双摘要并阻断 | “外部检测者”无 actor/路径/格式；作者自审；结果不一致仍放行 |
| INV-77 | **effective_tier 唯一**：`effective_tier = UPGRADE_L2 ? L2 : change_tier`；GV-S17 是唯一升级事实，GV-S1 是历史初判；所有消费者读 effective_tier | 人类升级后仍按 GV-S1 的 L1 执行；两个“实际级别”来源 |
| INV-78 | **canonical 载体有主**：每个 canonical 载体必须有唯一 owner、路径、格式、写入时机，并登记 G-1 MANAGED；新建载体先登记后写入 | 无主台账；新建模板/注册表直接写；G-1 不覆盖 |
| INV-79 | **分级标签封闭**：只有 L0/L1/L2 三个风险标签；不得新增子级或第四级；治理/工具链/canonical 一律 L2 | 新增子级“优化”治理变更；把治理/工具链当 L0/L1 |
| INV-80 | **tier scoping**：per-change 记录（change-package、QC/Review、GV-S20、GV-S21、ledger/baseline 行、release_scope、README 行）继承父变更的 `effective_tier`，不得自我升级；只有修改 carrier 的结构/规则才是 L2 | L1 的 QC 报告被当 L2；审计行/台账行导致父变更升级；carrier 结构变更被当 L0 |
| INV-81 | **bootstrap 阶段封闭**：P0a/P0b 使用 BASE（首个 P0a 提交的前一提交）与 scoped AC 集（AC-01…68 + 已批准 Spec 版本）；治理 AC NOT-RUN、Release 阻断；bootstrap 不得用于 Release/AC-VERIFIED；P0b 后恢复完整 AC-100 | 用自身 NOT-RUN AC 阻断 P0b；bootstrap 无期限/无人类批准；P0b 后仍用 scoped 集 |

---

## 4. 业务规则与验收标准（EARS）

> 语法遵循 `AGENTS.md` 附录 A 与宪法附加协议（`AGENTS.md:130-137`；`.specify/memory/constitution.md:61-69`）。每条 AC 必须可由不阅读实现的测试者独立复核；治理 AC 以文档/命令输出/门禁记录为证据，不得伪造 Go 测试。

### 4.1 门禁执行器完整性（GOV-CODE-01）

**AC-69** 门禁四项完整输出
- WHEN 任一受管 commit range 传入治理门禁执行器，THE SYSTEM SHALL 输出 G-1、G-2、G-3、G-4 四项结论，每项带 PASS/FAIL/NOT-RUN 与证据。
- IF 任一门禁未实现、未执行或输出中缺失，THEN THE SYSTEM SHALL 返回 `GATE_INCOMPLETE`；不得以 WARN、空输出或 `mode=warn` 的零退出码代替 PASS。
- IF 任一结论为 NOT-RUN，THEN THE SYSTEM SHALL 把该次运行标为未通过，并阻断 Release 结论。

**AC-70** Review 与 Release 结论分离
- WHEN 以 Review 模式运行，THE SYSTEM SHALL 输出 review 结论，声明 `review_scope` 与精确 base/head，且不得输出 Release PASS。
- WHEN 以 Release 模式运行，THE SYSTEM SHALL 要求精确 `base..head`、阻断项清零、延期项已处置、受影响基线非回归证据与覆盖 Release 对象集的全量 QC（AC-88/AC-105）；IF 任一缺失，THEN THE SYSTEM SHALL 只在 GV-S18 记录 `release_block_reason=RELEASE_BLOCKED`，不得写 `release_verdict`；人类放行由 GV-S6 单独记录。
- IF 模式未声明或无法判定，THEN THE SYSTEM SHALL fail-closed 为 `release_block_reason=RELEASE_BLOCKED`，不得默认按 Review 或 Release PASS。

**AC-71** 显式 commit range
- WHEN Review 或 Release 场景运行门禁，THE SYSTEM SHALL 接受并回显显式 `base..head`，并与变更输入包的 base/head 一致。
- IF 未提供显式范围，THEN THE SYSTEM SHALL 在结论中声明 `RANGE_UNBOUND`，且不得把默认 `HEAD^..HEAD` 的结果用于 Review/Release 结论；默认范围只允许作为本地提示。
- IF 运行期间 HEAD 变化，THEN THE SYSTEM SHALL 重新绑定范围并重跑；旧结论不得覆盖新范围，也不得由后续提交继承。

**AC-72** AC-REF 与 AC-VERIFIED 语义
- WHEN 门禁输出 AC 相关结论，THE SYSTEM SHALL 使用 `AC-REF` 与 `AC-VERIFIED` 两个术语；不得输出 `AC coverage`、覆盖率或验收率。
- IF 仅在测试代码中发现 `TestAC<NN>` 或 `AC-<NN>` 引用，THEN THE SYSTEM SHALL 只标 AC-REF。
- IF 无指定 HEAD 的真实执行证据或 Reviewer 的语义核对，THEN THE SYSTEM SHALL 不得标 AC-VERIFIED；未运行、SKIP、FAIL 一律不得 AC-VERIFIED。
- IF 输出使用 `AC coverage`/覆盖率措辞，或把 AC-REF 标成 AC-VERIFIED，THEN THE SYSTEM SHALL 返回 `AC_SEMANTICS_INVALID` 并阻断该结论。

**AC-73** CI 覆盖治理路径
- WHEN 变更触及 `docs/**`、`prompt/**` 或 `.specify/**`，AND CI 运行，THE SYSTEM SHALL 在门禁触发路径内执行治理门禁。
- IF 变更仅触及上述路径而 CI 未触发门禁，THEN THE SYSTEM SHALL 把门禁状态视为 NOT-RUN 并由独立检测者记录 `GATE_NOT_RUN`，不得以“CI 绿/无失败”作为通过证据。
- IF 触发路径配置缺少上述任一已批准目录，THEN THE SYSTEM SHALL 返回 `GATE_INCOMPLETE`；不得以“路径很少变更”为由排除。

**AC-74** 执行器失败/超时 fail-closed
- IF 门禁执行器崩溃、无法读取 git/台账，或因自身异常（非门禁判定 FAIL）返回非零退出，THEN THE SYSTEM SHALL 记录 `GATE_ERROR` 并 fail-closed；IF 超时，THEN SHALL 记录 `GATE_TIMEOUT`；门禁判定为 FAIL 时 SHALL 记 FAIL，三者都不得当作 PASS，也不得用人工口头确认替代重跑。
- WHEN 执行器恢复，THE SYSTEM SHALL 在同一 `change_id + base + head + mode` 上重跑并保留两次结果；只有最新一次 PASS 可用于结论。
- IF Release 模式执行失败/超时，THEN THE SYSTEM SHALL 在 GV-S18 记录 `GATE_ERROR`/`GATE_TIMEOUT`（不得写 `release_verdict`）并阻断 Release，直至重跑 PASS 且人类放行；不得由执行器写 Release PASS。

### 4.2 变更分级与错分（GOV-TIER-01）

**AC-75** 分级唯一、完备与不可低估
- WHEN 任何变更进入 `change_id` 的变更输入包，THE SYSTEM SHALL 按 AC-97 赋予且仅赋予一个标签 L0/L1/L2：L0/L1 仅限 AC-76/77 的封闭枚举，其余一律 L2；并记录判据、证据与命中的 L2 例外。
- IF 判据证据不足、文件/行为冲突或落入未知/未列举范围，THEN THE SYSTEM SHALL 按 L2 处理并交人类裁决；不得默认低级别。
- IF 命中任一 L2 判据，THEN THE SYSTEM SHALL 至少为 L2；不得以“改动小、紧急、只改一行、只改文档/脚本”降为 L1/L0。
- WHILE 同一变更包含多个子改动，THE SYSTEM SHALL 以最高子改动级别作为 overall tier，不得拆分 `change_id` 规避 L2；但 AC-111 的 per-change 记录不算独立子改动，随父变更继承 tier。

**AC-76** L0 封闭枚举
- WHEN 变更仅属于以下之一，THE SYSTEM SHALL 定为 L0：(a) 非治理文档的错别字/链接/格式，且不改变行为、契约、状态、权限或生成物语义；(b) 代码注释/空白/格式，且不改变语义或生成输出；(c) 测试名称/注释/格式，且不改变断言或覆盖。
- IF 变更触及业务行为、AC/INV、接口/事件/迁移、状态机、权限、资金/库存/履约准入，或触及宪法/AGENTS/CONVENTIONS/任何 Spec/ADR/治理/工具链/canonical/`scripts/**`/`.github/**`/`Makefile`/`internal/platform/**`，THEN THE SYSTEM SHALL 不得定为 L0。
- WHEN L0 变更触及受管路径，THE SYSTEM SHALL 保留 `change_id`、Actor/Reviewed-by 与 diff 范围；不得以“轻量”为由省略可追溯性（`AGENTS.md:77`；`docs/collab/CONVENTIONS.md:216`）。

**AC-77** L1 封闭枚举
- WHEN 变更属于以下之一，THE SYSTEM SHALL 定为 L1：(a) 单服务内、既有已批准 Spec 条款下的业务行为修正；(b) 单服务内既有契约下的输入校验/响应映射/本地化错误文案；(c) 单服务内既有契约下的查询/过滤/排序/读模型正确性修正；(d) 单服务内既有契约下的日志/指标/链路或无语义变更的性能调优。
- IF 变更需要新增/修改契约、跨服务、资金、库存、权限、状态机/安全边界，或触及宪法/AGENTS/CONVENTIONS/任何 Spec/ADR/治理/工具链/canonical/`scripts/**`/`.github/**`/`Makefile`/`internal/platform/**`，THEN THE SYSTEM SHALL 不得定为 L1，按 L2 处理。
- WHEN L1 影响既有 AC，THE SYSTEM SHALL 映射唯一台账并按 AC-101 取得真实执行证据；L1 不要求 Plan/新 Spec/ADR。

**AC-78** L2 默认与全流程
- WHEN 变更不属于 AC-76/77 的封闭枚举，或命中 P0/跨服务/资金/库存/权限/状态机/契约新增修改，或触及宪法/AGENTS/CONVENTIONS/任何 Spec/ADR/治理/工具链/canonical/`scripts/**`/`.github/**`/`Makefile`/`internal/platform/**`/未知路径，THE SYSTEM SHALL 走 L2 全流程：Spec→QC→人类书面批准→（架构/接口设计需要的）ADR→QC→人类书面批准→Plan/Tasks→人类书面 Plan 批准→实现→Reviewer→Release 人类书面放行；不得裁剪。L2 治理/工具链/canonical 若不引入架构或接口设计，可免 ADR，但不得免 Spec/QC/人类书面批准/Plan/Independent Verifier。
- IF L0/L1 执行中发现命中 L2 判据，THEN THE SYSTEM SHALL 立即转 AC-80 停线重分级，不得继续按低级别提交。
- WHEN L2 变更，THE SYSTEM SHALL 保留人类对 Spec/ADR/Plan/Release 的全部书面批准点（宪法 `:75-83`）；Plan 未批准不得写业务代码。

**AC-79** 分级登记与写入者分离
- WHEN Lead 初判级别，THE SYSTEM SHALL 在变更输入包中记录 `change_tier`、判据命中项、证据与 `change_id`；Dev/实现方不得写入或修改该字段。
- WHEN Reviewer 复核，THE SYSTEM SHALL 在 review verdict 中记录 `tier_review=CONFIRMED/MISMATCH/NEEDS_HUMAN`；Reviewer 不得改写 `change_tier`，MISMATCH/NEEDS_HUMAN 交人类按 GV-S17 写 `tier_ruling`。
- IF 同一变更的 tier 写入者与 review 裁决者为同一 actor，THEN THE SYSTEM SHALL 拒绝并记录 Actor 冲突（宪法 `:84`、`:92`）。
- WHEN GV-S17 写 UPGRADE_L2，THE SYSTEM SHALL 按 AC-108 派生 `effective_tier=L2`；不得仅按 GV-S1 的 L1 继续。

**AC-80** 错分停线
- WHEN 发现 L2 被按 L0/L1 执行或任何低估计，THE SYSTEM SHALL 立即 stop-the-line，冻结该 `change_id`，停止受管产物与代码提交。
- IF 变更已提交或已合入，THEN THE SYSTEM SHALL 不得以“已合入”视为免除，必须按 L2 重走（AC-81）。
- IF 有人尝试用新 `change_id`、`hotfix:`、拆小提交或修改判据绕过冻结，THEN THE SYSTEM SHALL 拒绝并记录 `TIER_MISMATCH`；`hotfix:` 仅限人类批准的独立安全修复，且不得扩展原范围（宪法 `:83`）。

**AC-81** 按 L2 重走与错分台账
- WHEN 错分确认，THE SYSTEM SHALL 由人类写 GV-S17 `UPGRADE_L2`，并按 `effective_tier=L2` 重走：补 Spec/ADR/QC/人类书面批准/Plan 重审/Reviewer 重审，并挂重审 Plan。
- WHEN 处置完成，THE SYSTEM SHALL 在 GV-S10 的 canonical 载体 `docs/review-tracking.md` change freeze 区追加错分行（`change_id、实际级别、发现时间、发现者、处置、重审 Plan 指针、证据`），并同步写 GV-S17；记录不得删除。
- IF 重走期间再次发现低估计，THEN THE SYSTEM SHALL 保持冻结并按更高一级重走，不得以“已整改”提前解冻。
- IF 错分源于判据歧义，THEN THE SYSTEM SHALL 由人类按 INV-56 批准判据升版，不得由实现或单个 Agent 私自改写。

**AC-82** L1 的 QC + Reviewer 联签
- WHEN L1 变更到达 `stage=post_review_pre_merge`（Reviewer 已出 `review_verdict` PASS 之后、merge 之前），THE SYSTEM SHALL 要求 QC 与 Reviewer 分别签署，两个签署 actor 不同，分别记录 `signed_at`、`stage`、`delta_digest`；签署范围覆盖本次 delta、受影响既有条款与 AC-101 的执行证据。
- IF 仅 Reviewer 单签、仅 QC 单签、QC 在 Reviewer verdict 之前签署、`stage` 不是 `post_review_pre_merge`、或 `delta_digest` 与最终 delta 不一致，THEN THE SYSTEM SHALL 不得批准，并记录 `SIGNOFF_MISSING`。
- WHEN 联签完成，THE SYSTEM SHALL 只放行 L1 阶段推进；WHILE 联签缺失，THE SYSTEM SHALL 保持等待状态，不得以沉默、默认或口头同意补齐；不得据此声称 Spec 全量验收、代码合入或 Release PASS。

**AC-83** 人类保留批准域（书面批准与知会分离）
- WHEN 请求宪法修订、分级裁决、Spec 批准、ADR 批准、Plan 批准（仅在存在/要求 Plan 时）或 Release 放行，THE SYSTEM SHALL 取得人类书面批准并写入 GV-S15 的 `approved_by/approved_at/scope/evidence`；L1 不得覆盖，也不存在“任何级别 Plan 批准”的隐含门。
- WHEN 出现 `change_id` 的首次/重审 FAIL 或 merge 动作，THE SYSTEM SHALL 取得人类知会/裁决；知会只适用于 FAIL 通知与 merge 通知，不得替代 Spec/ADR/Plan/Release 的书面批准。
- IF 任何 Agent 或机器把上述事项标记为“已委托/自动批准”，或把知会当书面批准，THEN THE SYSTEM SHALL 拒绝并 fail-closed。
- WHEN Release 放行，THE SYSTEM SHALL 由人类写 GV-S6 `release_verdict` 与精确 base/head、release_scope、内容摘要；不得由 Reviewer、Lead 或执行器代写。

**AC-84** 联签无效、失效与重签
- IF L1 联签缺少 QC 或 Reviewer、`signed_at` 早于 Reviewer verdict 或晚于 merge、`stage` 不是 `post_review_pre_merge`、或 `delta_digest` 与最终交付不一致，THEN THE SYSTEM SHALL 视为无效并阻断推进。
- IF 联签后修改的 delta 超出原签署范围、effective_tier 升为 L2、`change_id` 被 AC-80 冻结、或 HEAD/内容摘要变化，THEN THE SYSTEM SHALL 立即作废联签并要求重签；不得沿用旧签字。
- IF QC 在联签中声明的未复审章节属于本次 delta 或受影响条款，THEN THE SYSTEM SHALL 不得批准，须补齐复审。

**AC-85** 委托不得外溢
- WHEN L1 实施中发现命中 L2 判据，或产生跨服务、资金、库存、权限、状态机、治理/canonical 影响，THE SYSTEM SHALL 立即停止推进、按 AC-80 停线重分级，并作废 GV-S16 联签。
- IF 以 L1 联签替代 L2 的 QC 红队、人类书面批准或 Plan，THEN THE SYSTEM SHALL 拒绝并记录 `TIER_MISMATCH`。
- WHEN L1 变更扩大到影响 Release 证据或既有基线，THE SYSTEM SHALL 重新评估并取得人类 Release 书面批准。

**AC-86** delta 审查最小范围与版本摘要
- WHEN Spec 小版本修订提交 QC，THE SYSTEM SHALL 至少复审变更 delta 及其直接关联章节，并覆盖受影响的 INV/AC、State Ownership、异常分支与被审内容摘要（`reviewed_digest`）。
- IF delta 改变状态所有权、业务不变量、异常终态或跨服务协议，THEN THE SYSTEM SHALL 不得仅按 delta 放行，须扩大到相关章节或全量复审。
- WHEN 判断“小版本”，THE SYSTEM SHALL 以主版本号不变（vX.Y→vX.Y+1）为限；主版本递增按 AC-88 全量复审；版本号不变但 `reviewed_digest` 变化一律按新版本重审。

**AC-87** 未复审章节声明与摘要绑定
- WHEN 输出 delta QC 报告，THE SYSTEM SHALL 显式列出“本次未复审章节”清单（章节号、未复审理由、风险）、已复审的 delta 与章节，并绑定 `reviewed_digest`；IF 清单缺失、为空但未声明“全量复审”、与 delta 矛盾、或摘要与当前内容不一致，THEN THE SYSTEM SHALL 视为未完成 QC 并返回 `DIGEST_MISMATCH`/`QC_FULL_REVIEW_REQUIRED`，不得以 delta 审查放行。
- WHEN 未复审章节包含 Release 阻断相关条款，THE SYSTEM SHALL 在 Release 前补齐全量复审（AC-88）。
- WHEN 输出声明，THE SYSTEM SHALL 保持声明归 QC（GV-S7），Reviewer 不得代写或改写。

**AC-88** 两个强制全量复审节点（对象集与摘要）
- WHEN Spec 主版本递增（vX.Y→v(X+1).0）或变更进入 Release 门禁，THE SYSTEM SHALL 先按 AC-105 的对象集完成一次全量 QC 复审，输出覆盖全部对象章节与 INV/AC 的结论，并绑定每个对象的 `version + digest`；IF 缺少全量复审、仅有 delta 复审、对象缺失、或摘要与当前内容不一致，THEN THE SYSTEM SHALL 返回 `QC_FULL_REVIEW_REQUIRED` 并阻断 Release。
- WHILE 全量复审未完成，THE SYSTEM SHALL 不得产生 Release PASS；不得以 L0/L1 联签或人类口头催促替代。
- WHEN 需要取消或修改这两个节点，THE SYSTEM SHALL 按 INV-56 由人类批准 Spec 升版，不得由 Agent 私自豁免。

**AC-89** delta 不得扩张或替代
- WHEN 使用 delta 审查，THE SYSTEM SHALL 不得把它用于 L2 新增契约、状态机拆分、资金/库存/权限变更、跨服务协议修订、宪法/AGENTS/CONVENTIONS 控制降低或治理/canonical 变更。
- IF delta 审查试图替代全量 QC，或把未复审章节标为“无风险”而无证据，THEN THE SYSTEM SHALL 拒绝并记录 `QC_FULL_REVIEW_REQUIRED`。
- IF 人类豁免了 delta 之外的范围，THEN THE SYSTEM SHALL 按 AC-94 校验书面豁免；无有效豁免不得扩张。

**AC-90** 骨架生成（模板、字段与版本）
- WHEN Lead 为新 `change_id` 发起 plan/tasks/evidence-matrix，THE SYSTEM SHALL 从 `.specify/templates/plan-template.md`、`.specify/templates/tasks-template.md`、`.specify/templates/evidence-matrix-template.md` 生成结构草稿，并记录 `template_version`（模板 git blob/摘要）；`evidence-matrix-template.md` 的 owner 为 System Architect，缺失时按 AC-109 处理。
- WHEN 生成 plan，THE SYSTEM SHALL 至少包含 `change_id/base/head/tier/spec_version/integration_owner/baseline_impact/affected_AC/allowed_paths/rollback/approvals`；tasks 至少包含 `task_id/owner/AC/evidence/status`；evidence-matrix 至少包含 `AC/test_type/evidence_path/status/owner`。
- IF 模板缺失、生成失败、缺少任一最小字段或 `template_version` 不可追溯，THEN THE SYSTEM SHALL 返回 `SCAFFOLD_INCOMPLETE`，不得以空白手写稿替代。
- WHEN 生成重复执行，THE SYSTEM SHALL 按 `change_id + artifact` 幂等，不产生第二份骨架或第二套台账。

**AC-91** 主权声明行强制
- WHEN 生成 plan/tasks/evidence-matrix 骨架，THE SYSTEM SHALL 包含主权声明：`docs/collab/ac-coverage.tsv` 为唯一 AC 台账且 Owner=Reviewer；plan/evidence-matrix 只是 Plan 视图；各产物 owner 按 CONVENTIONS §1（`docs/collab/CONVENTIONS.md:20-31`）；GV-S3/GV-S10/GV-S19/GV-S20 的 canonical 路径不得被替换。
- IF 主权声明缺失、被删除或声称第二台账，THEN THE SYSTEM SHALL 拒绝该骨架，记录 `SCAFFOLD_OWNERSHIP_MISSING`，不得用于审批或证据。
- WHEN 内容 owner 填写，THE SYSTEM SHALL 保持该声明不变；不得以填写内容为由改写 owner 或台账归属。

**AC-92** 生成边界
- WHEN 使用 scaffold，THE SYSTEM SHALL 只生成结构与主权声明，内容由对应 owner 撰写；生成者不得代写业务结论。
- IF 以“已生成/已通过模板”作为 Spec、ADR、Plan 或 Release 批准证据，THEN THE SYSTEM SHALL 拒绝并记录。
- WHEN 判断 `api/gen` 生成物禁令，THE SYSTEM SHALL 仅适用于代码生成物；治理文档骨架不受该禁令约束，但不得手工修改代码生成物（`AGENTS.md:97`）。

**AC-93** `change_id` 全 ref 复用与注册表
- WHEN 分配 `change_id`，THE SYSTEM SHALL 使用本仓库全 ref（含未合并分支、已删除分支的 reflog）与 GV-S19 注册表中均未使用过的标识（无论其已 FAIL、冻结、已 Release 或已关闭），并与 base/head、Spec/ADR 版本及内容摘要绑定。
- IF 复用已使用/冻结/已 Release/已关闭的 `change_id`，或注册表/全 ref 检查发现冲突，THEN THE SYSTEM SHALL 拒绝并返回 `CHANGE_ID_REUSED`；跨仓库同名 id 必须带 repo 限定并登记，不得直接复用。
- IF G-1 尚未扩展复用校验，THEN THE SYSTEM SHALL 执行人工核对（注册表 + 全 ref + reflog 检索）并把检索证据写入 GV-S20；无证据不得分配。
- IF 需要独立紧急安全修复，THEN THE SYSTEM SHALL 由人类批准新的 `hotfix:` `change_id`，且只限安全范围；不得扩展到原 `allowed_paths`（宪法 `:83`）。

**AC-94** 人类豁免有效性（含撤销）
- WHEN 任何豁免被引用（包括 delta QC 范围、门禁或流程范围），THE SYSTEM SHALL 具备人类书面记录：批准人、日期、范围、期限、受影响 INV/AC、替代控制与撤销状态，并在受影响 Spec/线程中引用。
- IF 豁免缺失、过期、超范围、已撤销或由 AI 自行推断，THEN THE SYSTEM SHALL 视为不存在并 fail-closed（返回 `WAIVER_INVALID`），不得推进。
- WHEN 豁免到期或人类撤销，THE SYSTEM SHALL 立即失效并重新评估受影响结论；已按该豁免推进的工作 SHALL 按更严格规则重走或阻断，并记录 `WAIVER_INVALID`、影响面与证据；不得自动续期，AI 不得创建、扩大或撤销豁免（宪法 `:104`）。

**AC-95** 结论可追溯（摘要、载体与追加）
- WHEN 产生分级、tier 裁决、联签、门禁、QC、豁免、错分或 Release 结论，THE SYSTEM SHALL 记录 `change_id`、`parent_change_id`、继承的 `effective_tier`、actor、时间、base/head、mode、结论、`reviewed_digest`、证据路径与引用版本，并追加到 GV-S20 的 `docs/review-tracking.md` `Governance audit log` 区（按 AC-111 继承父变更 tier）；git 历史为最终事实。
- IF 任一结论缺少范围绑定、内容摘要、证据路径或追加记录，THEN THE SYSTEM SHALL 不得用于 Release，并返回 `EVIDENCE_UNBOUND`。
- WHEN 记录已追加，THE SYSTEM SHALL 不得删除或覆盖以规避审计；更正必须以新增行 + `supersedes` 表达，并保留原行；删除/改写他人行即违规。

**AC-96** 业务基线不变
- WHEN 本 Spec 生效，THE SYSTEM SHALL 不改变任何业务 AC（AC-01…AC-68）与 INV-01…INV-50 的语义、编号或验收要求；IF 治理优化导致需要调整业务 AC/INV，THEN THE SYSTEM SHALL 停止并按对应业务 Spec 升版与人类批准处理，不得借本 Spec 静默改写。
- WHEN 回归验证，THE SYSTEM SHALL 证明受影响的既有治理与业务基线未被降低；不得只验证新增 AC-69…AC-112。
- IF 本 Spec 与宪法、AGENTS 或 CONVENTIONS 冲突，THEN THE SYSTEM SHALL 以更高优先级文件为准并修正本 Spec（宪法 `:99-103`）。

### 4.7 治理/工具链分级、委托、自保护与引导（GOV-TIER-01 / GOV-HUM-01 / GOV-CODE-01）

**AC-97** 全域分级总函数（L0/L1 封闭 + L2 默认）
- WHEN 任何变更进入 `change_id`，THE SYSTEM SHALL 仅当符合 AC-76 封闭枚举时定 L0，仅当符合 AC-77 封闭枚举时定 L1；其余全部定 L2。
- WHEN 变更属于下列任一，THE SYSTEM SHALL 定 L2，不得定 L0/L1：宪法、AGENTS、CONVENTIONS、任何 Spec/ADR、prompt、治理文档、`docs/specs/**` 治理 Spec（含新建）、`docs/adr/**`、`docs/specs/README.md`、`scripts/**`、`.github/**`、`Makefile`、`internal/platform/**`、`docs/review-tracking.md`、`docs/collab/ac-baseline.txt`、`docs/collab/ac-coverage.tsv`、`.specify/templates/**`、`docs/lead/**`（change-package/release scope）、`docs/collab/change-id-registry.tsv`、`docs/collab/gate-runs/**`、`docs/collab/qc-reports/**` 及任何其他 canonical 载体；但把该载体作为**变更对象**（结构/规则/既有内容改写）才 L2，一次变更自身产生的 per-change 记录按 AC-111 继承父变更 tier。
- IF 文件/路径/工件未在 AC-76/77 或上句显式列举，THEN THE SYSTEM SHALL 定 L2 并交人类裁决；不得以“新建/未列入/只改文档/只改模板”定 L0/L1。
- WHEN 新建 canonical 载体（台账、模板、注册表、审计区、release scope、change-package、门禁记录等），THE SYSTEM SHALL 先按 AC-109 指定 owner/路径/格式并登记 G-1 `MANAGED`，再允许首次写入；未登记即 `EVIDENCE_UNBOUND`。
- WHEN 本 Spec 经人类批准后的落点实施（AGENTS/CONVENTIONS/CI/脚本/canonical）首次适用本条，THE SYSTEM SHALL 定 L2，并适用 AC-98/AC-100/AC-107/AC-109；不得当作 L0/L1 文档修正。
- WHEN 变更仅因项目排期 P0/P1/P2 被标记，THE SYSTEM SHALL 不据此改变级别；AGENTS §2.5 的 P0/跨服务/资金/库存/权限/状态机仍是 L2 判据，不得用排期标签豁免。

**AC-98** L1/L2 产物×批准者矩阵
- WHEN L1 推进，THE SYSTEM SHALL 执行：Plan/Tasks **不要求**；Reviewer 先出 `review_verdict` PASS + scope；QC 再签 L1 治理声明（无新增契约、无 L2 判据、受影响既有条款、AC-101 执行证据/书面豁免）；Reviewer 后签联合记录；最后才允许 merge，且 merge 须人类知会/裁决；本 L1 产生的 change-package/QC 报告/review verdict/GV-S20 行/台账行按 AC-111 继承 L1，不另定 L2。
- WHEN L2 推进，THE SYSTEM SHALL 执行：Spec/QC/人类书面批准（架构/接口设计需要时加 ADR）；Plan/Tasks + 人类书面 Plan 批准后才能写业务代码；实现后 Reviewer 出 `review_verdict`；Release 人类书面放行；L2 不得使用 L1 联签替代任何批准点。L2 治理/工具链/canonical 无架构/接口设计时免 ADR，但仍需 Spec/QC/人类书面/Plan/Independent Verifier。
- WHEN L2 触及 `scripts/**`、`.github/**`、`Makefile` 或 canonical/门禁，THE SYSTEM SHALL 追加 AC-100/AC-107/AC-109 的独立复核与载体登记。
- IF 矩阵中任一产物缺失、顺序错误、QC 签代码质量而非治理声明、用“L1 无 Plan”推进 L2、或用 L1 联签替代 L2，THEN THE SYSTEM SHALL 拒绝并记录 `SIGNOFF_MISSING`/`TIER_MISMATCH`。
- WHEN 任何文档再出现“任何级别 Plan 批准”，THE SYSTEM SHALL 解释为“仅在存在/要求 Plan 时”，不得据此给 L1 增加 Plan 门或要求人类逐门批准。

**AC-99** G-3 治理证据路径与状态枚举
- WHEN G-3 扩展运行，THE SYSTEM SHALL 在既有 G-3 内读取 `ac-coverage.tsv` 的 `test_type/evidence_path/status`，且新规则**只作用于 AC-69…AC-112**；业务 AC-01…AC-68 保留既有 AC-REF/延期语义，不得被新规则改为必须 VERIFIED。
- WHEN 登记状态，THE SYSTEM SHALL 仅接受 `status ∈ {REF,PENDING,VERIFIED,DEFERRED}`；只有 `VERIFIED` + 非空 `evidence_path` + Reviewer 签名 + `reviewed_digest` 才算验证；`REF/PENDING/DEFERRED` 不视为验证。
- IF 治理 AC 只有 `REF/PENDING/SKIP/FAIL`、缺失/过期延期、空 `evidence_path` 或 `status!=VERIFIED`，THEN THE SYSTEM SHALL 记 `GOV_AC_EVIDENCE_MISSING`；不得以登记台账或批量 deferred 代替。
- WHEN G-3 扩展尚未上线，THE SYSTEM SHALL 将治理 AC 标为 NOT-RUN（`G3_BOOTSTRAP_REQUIRED`）并阻断 Release，不得宣称 AC-VERIFIED；不得把 44 条治理 AC 整批登记为延期，单条延期必须有独立 owner/expiry/reason/thread 与人类豁免，且在 Release 前处置。
- WHEN Reviewer 填行，THE SYSTEM SHALL 记录 `test_type`（`unit/integration/race/fault-injection/manual/static/script`）、`evidence_path`、`status=VERIFIED`，并保留 owner/thread 与摘要；G-3 PASS 不证明执行，Reviewer 仍须语义核对。

**AC-100** 门禁/CI 自保护与独立复核
- WHEN 变更触及 `scripts/**`、`.github/**`、`Makefile` 门禁接线、GV-S4/GV-S18 生成逻辑或任何门禁/CI 配置，THE SYSTEM SHALL 用**基线版本**与变更后版本各执行一次门禁，并**无论两次是否 PASS**，都由 Independent Verifier（actor ≠ 变更作者/Lead）对门禁/CI diff 逐行复核并记录 GV-S21。
- IF 基线/变更后结果不一致、任一失败、或 diff 复核发现未来路径/条件弱化（即使当前 range 未触发），THEN THE SYSTEM SHALL 绑定两次摘要并阻断 merge/Release，直至修复或人类书面豁免。
- IF CI 被停用、触发路径被删除、workflow 被删改或门禁未运行，THEN THE SYSTEM SHALL 由 Independent Verifier 发出 `GATE_NOT_RUN` 并阻断；不得用被审 diff 中的执行器自证。
- WHEN D-CR-6 尚未关闭且变更触及根 `AGENTS.md`，THE SYSTEM SHALL 执行人类书面批准 + 独立 diff 复核 + QC/Reviewer 审核；不得以“三项 CI 路径已覆盖”或“保守默认”自称完整。
- IF 执行器在同一 diff 中被改成恒 PASS、跳过检查或改写自身退出码，THEN THE SYSTEM SHALL 拒绝并记录 `GATE_INCOMPLETE`/`GATE_NOT_RUN`。
- WHEN bootstrap 阶段（AC-112）尚未完成 P0b，THE SYSTEM SHALL 使用 `BASE` 上 legacy G-1…G-4（bootstrap 语境按 AC-106：G-1…G-3 PASS + G-4 NOT-RUN(not-implemented-at-BASE)；scoped 到 AC-01…68 + 已批准 Spec 版本）+ Independent Verifier 逐行 diff 复核作为 P0a/P0b 的判据；本条前两句的完整两次门禁在 bootstrap 变更中由该 scoped 评估替代，治理 AC-69…112 的 NOT-RUN 不参与 PASS/FAIL，不得用未上线的新 G-3 自证，也不得用 P0b 自身 NOT-RUN 的 AC 阻断 P0b。

**AC-101** L1 真实执行证据
- WHEN 请求 L1 联签，THE SYSTEM SHALL 要求每个受影响 AC 在绑定 head 上有真实执行证据：`test_type` 非空、`evidence_path` 存在、`status=VERIFIED`、摘要绑定，且不是 SKIP/FAIL/仅 AC-REF；或具备 AC-94 有效人类书面豁免。
- IF 证据缺失、仅 G-3 PASS、或未覆盖异常分支，THEN THE SYSTEM SHALL 不得联签并返回 `EVIDENCE_UNBOUND`；G-3 PASS 不得被当作执行证明。
- WHEN 证据提交，THE SYSTEM SHALL 由 Reviewer 核验 `【测试范围】/【未覆盖范围】` 与 Mock 真实性（AGENTS `:113`）；IF 发现假绿/假 Mock/隐瞒异常，THEN SHALL 拒绝并记录。

**AC-102** 内容摘要绑定
- WHEN 任何 Spec/ADR/QC/Review/Release 结论产生，THE SYSTEM SHALL 绑定被审内容的 `version + digest`（sha1 或 git blob）；delta/full QC 报告记录 `reviewed_digest`，Review/Release 记录 `reviewed_digest`。
- IF 版本号相同但内容摘要变化，THEN THE SYSTEM SHALL 视为新版本并重审；旧 delta/full QC、Review PASS、Release PASS 全部失效。
- IF 缺少 `reviewed_digest` 或摘要与当前内容不一致，THEN THE SYSTEM SHALL 返回 `DIGEST_MISMATCH`，不得用于联签/Release。

**AC-103** `change_id` 注册表与人工兜底
- WHEN 分配/关闭 `change_id`，THE SYSTEM SHALL 在 GV-S19 `docs/collab/change-id-registry.tsv` 追加一行（分配）或只追加关闭状态；不得覆盖历史行。
- IF 注册表缺失、写入者不是 Lead、或行缺少 `repo/base/head/tier/status/refs`，THEN THE SYSTEM SHALL 拒绝分配并返回 `CHANGE_ID_REUSED`/`EVIDENCE_UNBOUND`。
- WHILE G-1 未扩展复用校验，THE SYSTEM SHALL 以“注册表 + 全部 ref + reflog”人工核对作为过渡控制，并把检索证据写入 GV-S20；不得只查当前分支或只凭记忆。

**AC-104** 治理基线 delta audit
- WHEN `AGENTS.md`、`docs/collab/CONVENTIONS.md`、宪法、`docs/specs/**` 治理 Spec、`prompt/**`、`scripts/**`、`.github/**` 或 `Makefile` 发生治理性变更，THE SYSTEM SHALL 在下次交付前记录 delta audit：日期、旧/新版本、变更文件与差异摘要、Lead 与所有受影响 subagent 的重读确认；载体为 `docs/review-tracking.md` 或对应 `docs/collab/<pair>/threads/`。
- IF delta audit 缺失、未覆盖受影响 subagent、或未记录重读版本，THEN THE SYSTEM SHALL 不得进入下一交付/merge，并记录 `EVIDENCE_UNBOUND`。
- WHEN 变更涉及 `docs/collab/CONVENTIONS.md` 或 `prompt/**` 的流程/角色边界，THE SYSTEM SHALL 由 Reviewer 复核 delta audit；不得只由 Lead 自证。

**AC-105** Release 全量 QC 对象集
- WHEN 进入 Release 门禁，THE SYSTEM SHALL 由 GV-S22 release owner 定义 `release_scope` = 本次 Release 涉及的全部 Spec/ADR（当前版本 + digest）及受影响基线；全量 QC SHALL 覆盖该对象集的全部对象，并产出一份 Release QC 记录；该记录按 AC-111 继承 release 的 tier，不单独定 L2。
- IF `release_scope` 缺失、任一对象只有 delta 复审、对象版本/摘要与当前不一致、或只有单个 Spec 全量审，THEN THE SYSTEM SHALL 返回 `QC_FULL_REVIEW_REQUIRED` 并阻断 Release；不得用单 Spec 全审冒充整批 Release。
- WHEN `release_scope` 增加/变更对象，THE SYSTEM SHALL 重新执行该对象的全量 QC 并更新同一份 Release QC 记录；不得沿用旧结论。

**AC-106** G-3 引导顺序与 bootstrap 验证
- WHEN GOV-CODE-01 首次部署，THE SYSTEM SHALL 按 AC-112 的 P0a→P0b 顺序执行；G-3 扩展上线前，任何治理 AC 都不得标 AC-VERIFIED，治理 AC 为 NOT-RUN（`G3_BOOTSTRAP_REQUIRED`）、Release 阻断。
- WHEN 验证 G-3 扩展自身，THE SYSTEM SHALL 以 `BASE` 树上 legacy **G-1…G-3 PASS**（scoped 到该 `BASE` 上已批准 Spec 版本的 AC-01…AC-68）为门禁证据，且 SHALL 以显式 `--range=<BASE^>..<BASE>` 运行 legacy 执行器、在 GV-S21 回显该 range 与完整命令；不得退化为默认 `HEAD^..HEAD`；`BASE` = 第一个 P0a 提交的前一提交，AC-69…AC-112 不在其 gate 范围；不得用未上线的新 G-3 或 NOT-RUN 治理 AC 自证。
- WHEN `BASE` 上 G-4 尚不存在（`scripts/collab_gates.py:2` 只实现 G-1/G-2/G-3），THE SYSTEM SHALL 由 Independent Verifier 在 GV-S21 的 `g4_status` 字段（枚举 `PASS|NOT-RUN(not-implemented-at-BASE)|NA`）显式记录 **`g4_status=NOT-RUN(not-implemented-at-BASE)`**，并附 legacy 执行器的 `SUMMARY`（仅 G-1…G-3）与该缺失事实；该记录 SHALL 不阻断 P0a/P0b，但 SHALL NOT 计为 PASS、SHALL NOT 用于 Release 或 AC-VERIFIED，也不构成 AC-99/INV-69 的 `DEFERRED` 批量延期登记。
- WHEN 组成 bootstrap 证据，THE SYSTEM SHALL 使用 G-1…G-3 PASS（显式 `--range=<BASE^>..<BASE>`）+ `G-4 NOT-RUN(not-implemented-at-BASE)` 记录 + Independent Verifier（GV-S21，actor ≠ 作者/Lead）逐行 diff 复核 + 人类书面批准（GV-S15/GV-S20，含 approver/date/expiry）+ `BASE`/diff 摘要；本条是该证据组合的唯一口径，AC-100 与 INV-75 中 bootstrap 语境的“legacy G-1…G-4”按本条解释。
- WHILE bootstrap 窗口（P0a/P0b），THE SYSTEM SHALL 不把 AC-69 的 **G-1…G-4 四项完整输出要求**与 `GATE_INCOMPLETE` 规则适用于“G-4 尚未实现”这一事实（仅此一条事实被豁免，其余缺口仍按 AC-69 处理）；该豁免仅限 bootstrap 且随 P0b 落地即终止，P0b 之后 G-4 缺失/未执行/未输出仍按 AC-69 返回 `GATE_INCOMPLETE`。
- IF bootstrap 验证失败、legacy G-1…G-3 失败、缺显式 `--range` 或 range 与 `BASE^..BASE` 不符、缺 `G-4 NOT-RUN(not-implemented-at-BASE)` 记录、把该记录当 PASS、无独立复核/人类书面批准/摘要缺失、或 `BASE` 落在 AC-69…登记之后，THEN THE SYSTEM SHALL 阻断扩展上线并返回 `G3_BOOTSTRAP_REQUIRED`/`EVIDENCE_UNBOUND`；不得以 G-3 未上线、预期失败或批量延期启动 P0。
- WHEN P0b 完成，THE SYSTEM SHALL 结束 bootstrap 的 scoped 评估与上述 G-4 例外并恢复 AC-99/AC-100 的完整 AC 集；G-4 自 P0b 起为必需门禁；若扩展回滚或失效，治理 AC 回到 NOT-RUN 并阻断 Release；bootstrap 不得用于 Release 或 AC-VERIFIED 声明。


**AC-107** Independent Verifier（唯一定义：actor、指定、证据、格式）
- WHEN 需要独立复核，THE SYSTEM SHALL 使用由 Director 指定并在 GV-S21 记录 `designated_by` + `independence_evidence` 的 Independent Verifier（IV）；IV 必须 actor ≠ 变更作者/Lead，且其唯一证据载体是 GV-S21；无指定/无独立性证据即视为无 IV。
- WHEN 门禁/CI/Makefile/canonical 门禁变更需要独立复核，THE SYSTEM SHALL 由 Independent Verifier 执行，并在 GV-S21 `docs/collab/gate-runs/<change_id>/<base>-<head>.md` 记录：verifier、designated_by、independence_evidence、timestamp、base/head、baseline_result、post_change_result、baseline_digest、post_change_digest、diff_conclusion、`g4_status`（枚举 `PASS|NOT-RUN(not-implemented-at-BASE)|NA`）、是否 `GATE_NOT_RUN`；bootstrap scoped 运行另按 AC-112 记录 `ac_scope_evaluated/ac_scope_excluded/scope_filter_command`，并把“G-4 尚未实现”这一事实只写入 `g4_status`（不写入 `ac_scope_excluded`）。
- IF 缺少 GV-S21 记录、verifier 与作者/Lead 相同、或两次结果不一致未绑定双摘要，THEN THE SYSTEM SHALL 阻断 merge/Release 并返回 `GATE_NOT_RUN`/`EVIDENCE_UNBOUND`。
- WHEN 外部检测者发现门禁未运行，THE SYSTEM SHALL 由同一 Independent Verifier 角色发出 `GATE_NOT_RUN`；该角色不得由被审 diff 内的执行器或作者充当。

**AC-108** effective_tier 派生
- WHEN GV-S17 写 `UPGRADE_L2`，THE SYSTEM SHALL 派生 `effective_tier=L2`；门禁、Reviewer、Lead、Release 授权人必须读 `effective_tier`，不得只读 GV-S1 的原始 L1。
- IF GV-S1 与 GV-S17 冲突，THEN THE SYSTEM SHALL 以 GV-S17 为唯一升级事实，保留 GV-S1 历史值并记录派生依据；不得把 effective_tier 降回 L1。
- IF 任一消费者使用错误 tier 推进，THEN THE SYSTEM SHALL 停线并按 AC-80 重走。

**AC-109** canonical 载体登记、owner 与 G-1 覆盖
- WHEN 新建或替换 canonical 载体（`docs/review-tracking.md`、`ac-baseline.txt`、`ac-coverage.tsv`、`.specify/templates/**`、`docs/lead/**` change-package/release scope、`change-id-registry.tsv`、`docs/collab/gate-runs/**`、README 索引、任何新建 canonical 工件），THE SYSTEM SHALL 先指定唯一 owner、路径、格式、写入时机，并登记到 G-1 `MANAGED`；G-1 未覆盖即不得首次写入。
- WHEN 分配 owner，THE SYSTEM SHALL 至少遵循：change-package/registry 与 audit/freeze 区的建立/指针为 Lead（其中 GV-S10 错分行由 Reviewer 写、GV-S20 各行由结论 owner 写）；`docs/collab/gate-runs/**` 由 Independent Verifier 写、Lead 登记/指针；release_scope 为 Director 或其书面指定的 release owner（`designated_by/designation_evidence` 记录在 GV-S22）；AC 台账为 Reviewer；`evidence-matrix-template` 与 `.specify/templates/**` 为 System Architect；README 索引为 PO；QC 报告为 QC；不得由生成器或单一 Agent 代写全部。
- IF 载体未登记、owner 缺失、或 G-1 不覆盖该路径，THEN THE SYSTEM SHALL 拒绝首次写入并返回 `SCAFFOLD_INCOMPLETE`/`EVIDENCE_UNBOUND`；不得以“新文件”当 L0。
- WHEN 新建 canonical 工件，THE SYSTEM SHALL 按 AC-97 定 L2；不得把模板/台账/注册表/审计区当作文档轻量变更。

**AC-110** release owner 与 release_scope
- WHEN 进入 Release 准备，THE SYSTEM SHALL 由 Director 本人或 Director 书面指定的 integration owner 担任 release owner，并在 GV-S22 `docs/lead/releases/<release_id>/scope.md` 写 `release_id/release_owner/designated_by/designation_evidence/specs/adrs/versions/digests/baseline_impact`；Director 本人时 `designated_by='Director'` 且 `designation_evidence='direct'`。
- IF release owner 未指定、`release_scope` 缺失/过期/摘要不符，THEN THE SYSTEM SHALL 阻断全量 QC 与 Release，并返回 `QC_FULL_REVIEW_REQUIRED`；QC/Reviewer 不得代写 release_scope。
- WHEN `release_scope` 变更，THE SYSTEM SHALL 由 release owner 重写并重新执行全量 QC；不得沿用旧对象集。


**AC-111** tier scoping：per-change 记录随父变更，不自我升级
- WHEN 判定 tier，THE SYSTEM SHALL 以被变更的对象/行为定级，不以该变更产生的 per-change 记录定级。
- WHEN 一次变更产生 per-change 记录（change-package、QC 报告、Review verdict、GV-S20 审计行、GV-S21 门禁记录、该变更的 baseline/ledger 行、`release_scope`、README 索引行），THE SYSTEM SHALL 在这些记录中写 `parent_change_id` 与父变更的 `effective_tier`，并继承该 tier；不得对这些记录另行定级或升级为 L2。
- WHEN 变更对象是既有 canonical 载体的**结构/规则**（schema、模板、G-1…G-4 逻辑、gate paths、CI triggers、ledger/audit format、canonical 路径/owner），THE SYSTEM SHALL 把该载体变更本身定为 L2；不得以“只是更新一行”降级。
- WHEN 只是在既有载体中为某次变更按既有格式**追加新行或只追加 append-only 的状态/关闭行**（不改 schema/表头/格式/规则/路径/owner，不改写既有行内容），THE SYSTEM SHALL 视为该变更的 per-change 记录，按父变更继承 tier，不构成独立 L2 变更；例：GV-S19 `change-id-registry.tsv` 中本次 `change_id` 的关闭，按既有格式追加 `status=CLOSED`、`closed_at` 非空的行，既有行原样保留。
- IF 改写既有行的内容（修改已写入字段值、删除/覆盖历史行、改表头或 schema），THEN THE SYSTEM SHALL 按 AC-97 把该载体变更本身定为 L2（carrier 结构/规则/既有内容改写），不得援引本条继承；append-only 的“更新”只允许追加状态/关闭行，不包含原地改写。
- IF per-change 记录缺少 `parent_change_id`/继承 tier、或它被独立定级/升级，THEN THE SYSTEM SHALL 拒绝该记录并返回 `EVIDENCE_UNBOUND`。
- WHEN 判断 L0/L1 是否被 per-change 记录升级，THE SYSTEM SHALL 按本条继承规则执行；没有本条，AC-97 会把每次 L0/L1 变更产生的输入包、QC 报告、审计行、台账行都当 L2，从而自我升级。

**AC-112** first-deployment bootstrap 的阶段、BASE 与解除
- WHEN 首次部署 GOV-CODE-01，THE SYSTEM SHALL 按 P0a→P0b 顺序执行：P0a 扩展 G-3 接受治理证据（`test_type/evidence_path/status=VERIFIED` + digest + Reviewer 签名），P0b 落地 Release mode/显式 range/CI paths/自保护/完整 G-4（四门禁输出、`SUMMARY` 与退出码）；P0a 的 `BASE` = 第一个 P0a 提交的**前一个提交**，gate 只评估该 `BASE` 上 **已批准 Spec 版本的 AC 集（AC-01…AC-68）**，AC-69…AC-112 不在该 `BASE` 的 gate 范围内。
- WHEN P0a 验证，THE SYSTEM SHALL 由 `BASE` 树上 legacy **G-1…G-3 PASS**（scoped 到 AC-01…68 + 已批准 Spec 版本，显式 `--range=<BASE^>..<BASE>`）+ 显式 **`G-4 NOT-RUN(not-implemented-at-BASE)`** 记录 + Independent Verifier 逐行 diff 复核（actor ≠ 作者/Lead）+ 人类书面批准 + `BASE`/diff 摘要组成证据；该 G-4 记录 SHALL 不阻断 P0a/P0b、SHALL NOT 计为 PASS，G-4 在 P0b 落地并自 P0b 起为必需门禁；IV SHALL 在 GV-S21 的 `g4_status` 字段记录 `g4_status=NOT-RUN(not-implemented-at-BASE)`（枚举 `PASS|NOT-RUN(not-implemented-at-BASE)|NA`），并记录 `ac_scope_evaluated=AC-01…AC-68`、`ac_scope_excluded=AC-69…AC-112`（排除集只列 AC 编号，G-4 状态不入该字段）与实际执行的命令/临时过滤方式（`scope_filter_command`，含 `--range=<BASE^>..<BASE>`、path/glob 或人工过滤口径与完整命令），使 scoped PASS 可由第三方复现；不要求修改 legacy 执行器，也不得用未上线的新 G-3 或 AC-69…112 的 NOT-RUN 自证。
- WHEN P0a/P0b 期间，THE SYSTEM SHALL 把治理 AC-69…AC-112 标为 NOT-RUN、保持 Release 阻断，且不得据此宣称 Release 或 AC-VERIFIED；bootstrap 是有期限、人类书面批准的 scoped exemption：GV-S15 记 `approved_by/approved_at/scope/expiry`（`expiry` 必填），GV-S20 的该行 `evidence` 以 `approver=<actor>；date=<YYYY-MM-DD>；expiry=<YYYY-MM-DD>；scope=<P0a/P0b 范围>` 承载同一批准，到期自动恢复严格门禁；该例外仅限 bootstrap，不授权 Release 或 AC-VERIFIED。
- WHEN P0a/P0b 的 gate/CI 变更被 AC-100 的两次门禁检查时，THE SYSTEM SHALL 使用同一 scoped AC 集（AC-01…68 + 已批准 Spec 版本）与同一 `g4_status=NOT-RUN(not-implemented-at-BASE)` 记录评估，排除 AC-69…112 的 NOT-RUN，并在 GV-S21 复用同一 `g4_status`/`ac_scope_evaluated/ac_scope_excluded/scope_filter_command` 记录；不得让 P0b 被其自身尚未 VERIFIED 的治理 AC 阻断。
- WHILE bootstrap 窗口内 G-4 尚未实现，THE SYSTEM SHALL 按 AC-106 的 bootstrap 例外执行：AC-69 的 **G-1…G-4 四项完整输出要求**与 `GATE_INCOMPLETE` 规则**均不适用于“G-4 尚未实现”这一事实**（豁免仅限这一条事实，其他缺口仍按 AC-69 返回 `GATE_INCOMPLETE`）；该豁免随 P0b 落地终止，P0b 之后 G-4 缺失/未执行/未输出仍按 AC-69 返回 `GATE_INCOMPLETE`。
- WHEN P0b 完成，THE SYSTEM SHALL 结束 bootstrap 的 scoped 评估与该 G-4 例外，之后 AC-100 的两次门禁对完整 AC 集（AC-01…AC-112）且对 G-1…G-4 全部适用；Release 仍须治理 AC 完成 VERIFIED 且全量 QC 通过，不得以 bootstrap 代替 Release。
- IF bootstrap 缺少人类书面批准/期限（GV-S15 `expiry` 或 GV-S20 `evidence` 的 expiry 缺失）、`BASE` 不在 AC-69…登记前、范围超出 P0a/P0b、把 `G-4 NOT-RUN` 当 PASS 或 Release/AC-VERIFIED 证据、或在 P0b 之后仍援引该 G-4 例外，THEN THE SYSTEM SHALL 拒绝 bootstrap 并返回 `G3_BOOTSTRAP_REQUIRED`/`EVIDENCE_UNBOUND`；不得批量延期，也不得把 G-4 缺失登记为 `DEFERRED`。


---

## 5. 状态机与非法迁移

### 5.1 治理状态机（合法迁移与非法迁移）

| 状态机 | 合法迁移 | 非法/阻断 |
|---|---|---|
| 变更级别 | 未判定→L0/L1/L2（Lead 初判）→CONFIRMED/MISMATCH/NEEDS_HUMAN（Reviewer 复核）；MISMATCH/NEEDS_HUMAN→GV-S17 人类裁决（CONFIRM/UPGRADE_L2）；L0/L1 发现 L2 判据→FROZEN→按 L2 重新分级；effective_tier 按 AC-108 派生 | 未判定直接实施；L2→L1 降级；Dev 写 tier；tier 写入者与复核者同一 actor；人类裁决覆盖原始 tier 记录；只读 GV-S1 忽略 GV-S17 |
| L1 联签 | WAITING→REVIEW_VERDICT_PASS→QC_SIGNED→REVIEWER_COUNTERSIGNED→SIGNED（post_review_pre_merge）；SIGNED→WAITING（delta/HEAD/摘要变化、effective_tier 升级、AC-80 冻结） | QC 先于 Reviewer verdict；单签；同一 actor；超范围；缺 signed_at/delta_digest；升级后沿用旧签字 |
| QC 审查 | NOT_REVIEWED→DELTA_REVIEWED（带未复审清单+reviewed_digest）→FULL_REVIEWED（主版本/Release 对象集）；FULL_REVIEWED→NOT_REVIEWED（version/digest 变化） | DELTA_REVIEWED 直接 Release；无未复审清单；主版本跳过全量；旧全量结论复用到新版本/新摘要 |
| 门禁运行 | NOT_RUN→RUNNING（显式 range+mode）→PASS/FAIL/GATE_ERROR/GATE_TIMEOUT；ERROR/TIMEOUT→RUNNING（重跑）；PASS→NOT_RUN（HEAD/范围/摘要变化） | NOT_RUN/ERROR/TIMEOUT→PASS；默认 HEAD^..HEAD→Release；warn 零退出→PASS；旧 PASS 继承到新提交 |
| Release 结论 | GV-S18 门禁写 `release_block_reason`；GV-S6 人类写 `release_verdict`（PASS/BLOCKED） | 门禁写 `release_verdict`；人类写 `release_block_reason`；同一结论双写 |
| G-3 引导 | `BASE`（首个 P0a 前一提交）+ legacy G-1…G-3 PASS（显式 `--range=<BASE^>..<BASE>`）scoped 到 AC-01…68 + 显式 `g4_status=NOT-RUN(not-implemented-at-BASE)` 记录（不阻断 P0a/P0b、不计 PASS；G-4 在 P0b 落地，自 P0b 起必需）+ IV diff→EXTENSION_LIVE；扩展上线前治理 AC=NOT_RUN（`G3_BOOTSTRAP_REQUIRED`） | 扩展未上线宣称 VERIFIED；用未上线 G-3 自证 bootstrap；跳过 legacy gate/独立复核；用 P0b 自身 NOT-RUN AC 阻断；把 G-4 NOT-RUN 当 PASS 或作 Release 证据；P0b 后仍缺 G-4 |
| 人类豁免 | NONE→GRANTED（书面范围+期限）→EXPIRED（到期）/REVOKED（撤销/超范围）→NONE 或新 GRANTED | AI 创建/推断/扩大/撤销；无记录视为 GRANTED；自动续期；EXPIRED/REVOKED 继续引用 |

### 5.2 治理阻塞码（唯一语义）

| 阻塞码 | 返回方 | 含义 |
|---|---|---|
| `GATE_INCOMPLETE` | 门禁执行器 | G-1…G-4 未全部输出/未全部执行，或执行器被改成跳过检查 |
| `GATE_NOT_RUN` | Independent Verifier/CI | 治理路径变更未触发门禁、CI 被停用、触发路径被删 |
| `GATE_ERROR` | 门禁执行器 | 执行器崩溃/无法读取 git/台账/非判定 FAIL 的非零退出 |
| `GATE_TIMEOUT` | 门禁执行器 | 执行超时 |
| `RANGE_UNBOUND` | 门禁执行器 | Review/Release 未绑定显式 base..head |
| `AC_SEMANTICS_INVALID` | 门禁/Reviewer | AC-REF 被冒称 AC-VERIFIED 或使用“覆盖率”措辞 |
| `TIER_MISMATCH` | Lead/Reviewer | 级别与实际判据不符，或错分/绕过分级 |
| `SIGNOFF_MISSING` | Reviewer | L1 联签缺少 QC/Reviewer、顺序错误、actor 冲突或阶段错误 |
| `QC_FULL_REVIEW_REQUIRED` | QC/Reviewer | 主版本或 Release 对象集缺少全量复审 |
| `WAIVER_INVALID` | Reviewer/人类 | 豁免缺失、过期、撤销、超范围或 AI 推断 |
| `CHANGE_ID_REUSED` | Lead | 复用已使用/冻结/已 Release/已关闭的 `change_id`，或注册表冲突 |
| `SCAFFOLD_INCOMPLETE` | Lead | 模板/结构字段/template_version 缺失 |
| `SCAFFOLD_OWNERSHIP_MISSING` | Lead/Reviewer | 骨架缺少主权声明或声称第二台账 |
| `GOV_AC_EVIDENCE_MISSING` | G-3/Reviewer | 治理 AC 无 `status=VERIFIED` 的真实证据行 |
| `G3_BOOTSTRAP_REQUIRED` | 门禁/Independent Verifier | G-3 扩展未上线或 bootstrap 未完成；治理 AC 为 NOT-RUN，Release 阻断 |
| `DIGEST_MISMATCH` | QC/Reviewer/门禁 | `reviewed_digest` 缺失或与当前内容不一致 |
| `RELEASE_BLOCKED` | 门禁执行器（GV-S18） | 发布证据不足或存在阻断项；只写阻断原因，不写 Release PASS |
| `EVIDENCE_UNBOUND` | Reviewer | 结论缺少范围、摘要、证据路径或审计行 |

### 5.3 AC ↔ 阻塞码双向索引

| 阻塞码 | 触发 AC/场景 | 下游动作 |
|---|---|---|
| `GATE_INCOMPLETE` | AC-69、AC-73、AC-100 | 不签 PASS；补 G-1…G-4 或修复触发路径 |
| `GATE_NOT_RUN` | AC-73、AC-100、AC-107 | 视为未运行；补跑/人工 diff 复核前不 merge |
| `GATE_ERROR` | AC-74 | fail-closed；重跑并保留日志 |
| `GATE_TIMEOUT` | AC-74 | fail-closed；重跑/缩小范围 |
| `RANGE_UNBOUND` | AC-71 | 不用于 Review/Release；绑定显式范围后重跑 |
| `AC_SEMANTICS_INVALID` | AC-72 | 改回 AC-REF/AC-VERIFIED；补真实证据 |
| `TIER_MISMATCH` | AC-80、AC-85、AC-97、AC-98、AC-111 | 冻结重分级；按 L2 重走 |
| `SIGNOFF_MISSING` | AC-82、AC-84、AC-98 | 不推进；按矩阵补签/重签 |
| `QC_FULL_REVIEW_REQUIRED` | AC-88、AC-89、AC-105、AC-110 | 阻断 Release；补全量/对象集复审 |
| `WAIVER_INVALID` | AC-89、AC-94 | 视为无豁免；停止/重走 |
| `CHANGE_ID_REUSED` | AC-93、AC-103 | 拒绝分配；换新 id/hotfix |
| `SCAFFOLD_INCOMPLETE` | AC-90、AC-109 | 重新生成；不得手写替代 |
| `SCAFFOLD_OWNERSHIP_MISSING` | AC-91 | 拒收；恢复主权声明 |
| `GOV_AC_EVIDENCE_MISSING` | AC-99 | G-3 FAIL；补证据或单条豁免，不得批量延期 |
| `G3_BOOTSTRAP_REQUIRED` | AC-99、AC-106、AC-112 | 先完成 G-3 扩展/bootstrap；不得宣称 VERIFIED |
| `DIGEST_MISMATCH` | AC-87、AC-88、AC-95、AC-102 | 视为新版本；旧结论作废 |
| `RELEASE_BLOCKED` | AC-70、AC-105、AC-110 | 阻断 Release；人类不得基于该状态写 PASS |
| `EVIDENCE_UNBOUND` | AC-95、AC-101、AC-103、AC-104、AC-107、AC-109、AC-111、AC-112 | 不联签/不 Release；补证据行与审计 |

---

## 6. 幂等键（谁生成、作用域、重试语义）

| 对象 | 业务键 | 生成方 | 重试语义 |
|---|---|---|---|
| 变更级别 | `change_id` | Lead | 同键复用同一级别与判据；改级必须留痕并按 AC-81/AC-97 处理 |
| 分级复核/裁决 | `change_id + tier + reviewed_digest` | Reviewer/人类 | 同键幂等；MISMATCH 不得被后续 CONFIRMED 静默覆盖；人类裁决另写 GV-S17 |
| `change_id` 注册表 | `change_id + repo` | Lead | 同键只追加一次分配；关闭只追加状态；不得覆盖 |
| 门禁运行 | `change_id + base + head + mode + digest` | 门禁执行器 | 同键重跑保留记录；以最新 PASS 为结论；摘要变化即新键 |
| Review 裁决 | `change_id + base + head + review_scope + reviewed_digest` | Reviewer | 同键幂等；范围/摘要变化必须新裁决 |
| Release 裁决 | `change_id + base..head + release_scope + digests` | 人类 | 同键幂等；不得继承到不同 head/对象集 |
| QC delta 报告 | `spec_id + from_version + from_digest + to_version + to_digest` | QC | 同键幂等；版本/摘要变化须新报告 |
| 全量 QC | `spec_id + version + digest`（Release 时为对象集） | QC | 同键幂等；主版本/Release 对象集各需一次 |
| L1 联签 | `change_id + artifact + delta_digest + stage + signed_at` | QC/Reviewer | 同键幂等；delta/HEAD/effective_tier 变化必须重签 |
| 豁免 | 人类给定 `waiver_id`（或 `change_id + scope`） | 人类 | 同键幂等；不得自动续期；过期/撤销不可复活 |
| 错分台账 | `change_id + 发现序号` | Reviewer | 同键幂等；追加不覆盖；与 change freeze 同载体 |
| scaffold | `change_id + artifact + template_version` | Lead | 同键只生成一份；重复执行返回同一结构 |
| 治理审计行 | `timestamp + change_id + actor + base/head + mode` | 结论 owner | 追加；更正以新行 + `supersedes`，不覆盖 |
| 独立门禁复核 | `change_id + base + head + verifier + baseline_digest + post_change_digest` | Independent Verifier | 同键幂等；结果不一致必须双摘要阻断 |
| G-3 引导 | `extension_version + extension_digest + legacy_gate_digest + reviewer` | GOV-CODE-01 owner + Independent Verifier | 同键只引导一次；扩展变更需新键 |
| Release 对象集 | `release_id + owner + specs/adrs + versions/digests` | release owner | 同键幂等；对象集变化重写并重审 |

---

## 7. 异常与期望终态（规范性）

> 本矩阵与第 4 节 AC 同等规范；治理角色 = Lead / 门禁执行器 / Independent Verifier / QC / Reviewer / release owner / 人类（Director）。冲突时以更严格条款为准。

| # | 场景 | Lead | 门禁执行器 | QC | Reviewer | 人类 | 期望终态 |
|---|---|---|---|---|---|---|---|
| E32 | L2 被当 L1 执行 | 冻结 `change_id` | 不出 PASS | 按 L2 补审 | 记 MISMATCH + 台账 | 写 GV-S17 | 按 L2 重走 |
| E33 | 执行器崩溃/超时 | 暂停推进 | `GATE_ERROR`/`GATE_TIMEOUT` | — | 不得签 | 不放行 | `release_block_reason=GATE_ERROR/GATE_TIMEOUT` |
| E34 | CI 静默跳过治理路径 | 标 NOT-RUN | Independent Verifier 发 `GATE_NOT_RUN` | — | 不得以 CI 绿签 | 不放行 | 补跑/独立 diff 复核 |
| E35 | delta QC 漏未复审清单/摘要 | 退回 QC | — | 报告无效 | 拒收 | — | 补清单/摘要或全量复审 |
| E36 | Release 无全量 QC/对象集缺失 | 暂停 | `RELEASE_BLOCKED` | 补全量 | 不签 | 不放行 | `QC_FULL_REVIEW_REQUIRED` |
| E37 | 豁免缺失/过期/撤销 | fail-closed | — | 不按豁免审 | 拒绝引用 | 书面补/不批 | `WAIVER_INVALID` |
| E38 | scaffold 缺主权行/模板字段 | 重新生成 | — | — | 拒收 | — | `SCAFFOLD_OWNERSHIP_MISSING`/`SCAFFOLD_INCOMPLETE` |
| E39 | 复用 `change_id`（含全 ref/reflog） | 拒绝新 id | —（G-1 当前不校验复用） | — | 拒审 | 批准新 id/hotfix | `CHANGE_ID_REUSED` |
| E40 | 仅 Reviewer 签 L1 | 不推进 | — | 缺签 | 单签无效 | — | `SIGNOFF_MISSING` |
| E41 | L1 实施中发现 L2 | 停线 | 重跑 | 按 L2 审 | 记 MISMATCH + 作废联签 | 裁决 | 按 L2 重走 |
| E42 | AC-REF 被当 AC-VERIFIED | 不提交 Release | `AC_SEMANTICS_INVALID` | — | 只标 AC-REF | 不接受 | 补真实执行证据 |
| E43 | Release 用默认 `HEAD^..HEAD` | 不提交 | `RANGE_UNBOUND` | 全量复审 | 不签 | 不放行 | 显式 base..head 重跑 |
| E44 | 门禁/CI 被同一 diff 改成恒 PASS | 阻止提交 | 基线版本不得 PASS | 按未审处理 | 拒签 | 不放行 | `GATE_INCOMPLETE`/`GATE_NOT_RUN` |
| E45 | 基线/变更后结果不一致或无独立 diff 复核 | 阻止 merge | 绑定双摘要，不得自证 | — | 不签 | 不放行 | `GATE_NOT_RUN`/`EVIDENCE_UNBOUND`，阻断至复核 |
| E46 | 治理 AC 无 G-3 证据行 | 不宣称完成 | G-3 FAIL | 补证据/单条豁免 | 记 `GOV_AC_EVIDENCE_MISSING` | 不放行 | 补 `status=VERIFIED` 行 |
| E47 | 同版本不同内容摘要 | 重新登记 | 旧 PASS 无效 | 重新审 | 作废旧 verdict | 不放行 | `DIGEST_MISMATCH`，按新版本重审 |
| E48 | 豁免被撤销 | 立即失效 | — | 重新评估 | 拒绝引用 | 记录撤销 | `WAIVER_INVALID`；已推进工作按更严格规则重走 |
| E49 | 未列举/新建 canonical 被当 L0/L1 | 停线 | 重跑 | 按 L2 审 | 记 MISMATCH | 裁决 | 按 AC-97/AC-109 改判 L2 |
| E50 | Release 对象集缺少 Spec/ADR | 暂停 | `RELEASE_BLOCKED` | 补对象集 | 不签 | 不放行 | `QC_FULL_REVIEW_REQUIRED` |
| E51 | 治理文件变更缺 delta audit | 阻止下一交付 | — | — | 拒签 | 不放行 | `EVIDENCE_UNBOUND`，补 delta audit |
| E52 | L1 联签无真实执行证据 | 不提交 | G-3 不等于执行 | 不签 | 不联签 | 不接受 | `EVIDENCE_UNBOUND`，补执行证据/书面豁免 |
| E53 | G-3 扩展未上线即宣称 VERIFIED | 阻止 | `G3_BOOTSTRAP_REQUIRED` | 不签 | 拒收 | 不放行 | 先 bootstrap，治理 AC=NOT-RUN |
| E54 | 独立验证者缺失/actor=作者或 Lead | 阻止 | 记录 `GATE_NOT_RUN` | — | 拒签 | 不放行 | 补 GV-S21 记录/换独立 actor |
| E55 | 基线/变更后门禁结果不一致 | 暂停 | 双摘要绑定 | — | 不签 | 不放行 | 修复/书面豁免后才能推进 |
| E56 | release owner 未指定/release_scope 缺失 | 暂停 | `RELEASE_BLOCKED` | 不启动全量 | 不签 | 不放行 | `QC_FULL_REVIEW_REQUIRED` |
| E57 | 新 canonical 载体未登记/无 owner | 阻止首次写入 | 不得自证 | — | 拒收 | — | `SCAFFOLD_INCOMPLETE`/`EVIDENCE_UNBOUND`，按 AC-109 |
| E58 | P0b 被自身 NOT-RUN 治理 AC 阻断 | 使用 scoped AC-01…68 + 已批准 Spec | scoped 评估 | — | 按 scoped 签 | 批准 bootstrap | 按 AC-112 继续 P0b，不阻断、不 Release |

---

## 8. 失败可见性（人类 / Lead / Reviewer）

| 对象 | 人类（Director）看到 | Lead 看到 | Reviewer 看到/动作 |
|---|---|---|---|
| 门禁未运行/不完整/范围未绑定 | “门禁 NOT-RUN/INCOMPLETE 或结论未绑定 base..head，Release 阻断” | 补齐 G-1…G-4 并重新收集精确范围 | 不得签 PASS；记录 `GATE_NOT_RUN`/`GATE_INCOMPLETE`/`RANGE_UNBOUND` |
| 门禁/CI 被自改或基线/变更后不一致 | “执行器/CI 不可自证，需基线与独立复核” | 用基线版本重跑并绑定双摘要 | Independent Verifier 逐行复核；无证据不签 |
| 分级争议/错分 | `change_id`、实际级别、影响面、GV-S17 裁决 | 冻结并重走 L2 | 记 MISMATCH 与错分台账 |
| L1 联签缺失/顺序错误 | “等待 post_review_pre_merge 的 QC+Reviewer 联签” | 不得推进 | 不得单签；记录 `SIGNOFF_MISSING` |
| L1 无执行证据 | “无真实执行证据，不得联签” | 补执行证据或书面豁免 | 记录 `EVIDENCE_UNBOUND`；不得以 G-3 PASS 代替 |
| QC 全量/Release 对象集缺失 | “Release 前缺全量复审/对象集” | 暂停 Release | 不得签 Release；记录 `QC_FULL_REVIEW_REQUIRED` |
| 治理 AC 无 G-3 证据 | “G-3 FAIL/NOT-RUN，治理 AC 未验证” | 不宣称完成 | 补 `status=VERIFIED` 行；未扩展按 `G3_BOOTSTRAP_REQUIRED` |
| G-3 扩展未上线 | “治理 AC NOT-RUN，Release 阻断” | 先完成 AC-106 bootstrap | 不得宣称 VERIFIED；记录 `G3_BOOTSTRAP_REQUIRED` |
| 独立验证者缺失/actor 冲突 | “GV-S21 缺失，不可推进” | 指定 actor ≠ 作者/Lead | 拒签；记录 `GATE_NOT_RUN`/`EVIDENCE_UNBOUND` |
| release owner/scope 缺失 | “无法定义 Release 对象集” | 指向 GV-S22 | 不启动全量 QC |
| 新 canonical 未登记 | “载体无 owner/未入 G-1，禁止首次写入” | 按 AC-109 登记 | 拒收；记录 `SCAFFOLD_INCOMPLETE`/`EVIDENCE_UNBOUND` |
| 同版本不同摘要 | “旧结论已失效，须重审” | 重新登记 | 作废旧 verdict；记录 `DIGEST_MISMATCH` |
| 豁免失效/撤销 | “豁免缺失/过期/撤销，需书面裁决” | fail-closed | 拒绝引用；记录 `WAIVER_INVALID` |
| 执行器错误/超时 | “门禁运行失败，Release 阻断” | 重跑并保留日志 | 不得以人工口头确认替代 |
| scaffold 缺主权行/`change_id` 复用 | “骨架不可用；需要新 id 或人类批准 hotfix” | 重新生成/查注册表后分配 | 拒收/拒审；记录 `SCAFFOLD_OWNERSHIP_MISSING`/`CHANGE_ID_REUSED` |
| 治理文件变更缺 delta audit | “diff 未审计，不得进入下一交付” | 补 delta audit + 重读确认 | 拒签；记录 `EVIDENCE_UNBOUND` |
| 台账/索引未登记 | “G-2（README/changelog，PO）或 G-3（baseline/tsv，Reviewer）失败” | 不宣称完成 | 由 PO/Reviewer 各自登记；未登记前不得 Release |

---

## 9. 验收证据要求

- 每条 AC-69…AC-112 必须有可复核证据（门禁输出、CI 运行记录、文档检查、台账行或人工检查单）；纯治理 AC 不得伪造 Go 测试或伪造执行日志。
- 治理 AC 使用 G-3 证据路径：`ac-coverage.tsv` 行必须带 `test_type`（`unit/integration/race/fault-injection/manual/static/script`）、非空 `evidence_path`、`status=VERIFIED` + Reviewer 签名 + 摘要；`status` 仅接受 `REF/PENDING/VERIFIED/DEFERRED`，且新规则只作用于 AC-69…AC-112（业务 AC-01…68 保留 AC-REF/延期）。只有 `VERIFIED` 才算验证；`REF/PENDING/SKIP/FAIL/缺失/过期延期` 不算（AC-99）。不得把 AC-69…AC-112 整批登记为延期；单条延期须有独立 owner/expiry/reason/thread 与人类豁免，并在 Release 前处置。
- 证据必须绑定精确 `base..head`、mode 与 `reviewed_digest`，并带 `change_id`、`Actor`、`Reviewed-by`，且 Actor ≠ Reviewed-by；Review 与 Release 分别记录，不得用 Review PASS 代替 Release PASS（`docs/collab/CONVENTIONS.md:140-151`；`AGENTS.md:77`；宪法 `:84`）。
- `G-2` 由 **PO** 负责：新增/升版 Spec 必须在 `docs/specs/README.md` 索引含该版本，且 `## 13. 变更记录` 含该版本（`scripts/collab_gates.py:216-223`、`:237-242`）。`G-3` 由 **Reviewer** 负责：`ac-baseline.txt` 连续且含 AC-69…AC-112，`ac-coverage.tsv` 行有效（`scripts/collab_gates.py:299-367`）。二者都通过才算登记完成；G-2 未登记会 FAIL，G-3 在扩展上线前为 NOT-RUN（`G3_BOOTSTRAP_REQUIRED`）、上线后未登记才 FAIL，均不得视为已通过。
- 交付必须附 `【测试范围】` 与 `【未覆盖范围】`；未覆盖范围必须显式声明，不得以“不适用”静默跳过（`AGENTS.md:115`）；L1 联签前按 AC-101 核验真实执行证据；G-3 扩展按 AC-106 bootstrap，扩展上线前治理 AC 为 NOT-RUN、Release 阻断。
- 门禁/CI 变更必须附 GV-S21 Independent Verifier 记录（含两次结果与双摘要）；Release 必须附 GV-S22 `release_scope` 与全量 QC 记录；canonical 载体必须附 AC-109 登记证据。
- **当前登记状态（非豁免）**：`ac-baseline.txt` 止于 AC-68（`docs/collab/ac-baseline.txt:68`）；v1.4 批准同轮已由 PO 在 `docs/specs/README.md` §4 登记 SPEC-0003（版本 1.4，替代原 v1.3 行）；按 Director commit timing（AC-112、`docs/review-tracking.md:117-136`）批准稿保持 untracked 直至 P0a 原子落地；`ac-baseline.txt`/`ac-coverage.tsv` 的 AC-69…AC-112 仍待 Reviewer 登记。G-3 扩展上线前治理 AC 为 NOT-RUN（`G3_BOOTSTRAP_REQUIRED`）；扩展上线且台账完成后才可谈 G-3 PASS，不得以“已登记”宣称通过。

---

## 10. 决策与待裁决问题（D）

> D-CR-1…D-CR-5 的 canonical 批准证据：`docs/review-tracking.md:94`（2026-11-24 human decisions，approver Director，scope=all recommendations，governance/process only）。提案 `docs/cost-reduction-proposal.md:149-157` 只是待裁决表，不是批准记录。

| ID | 问题 | 决策/建议 | 影响 |
|---|---|---|---|
| D-CR-1 | 是否采纳三级分级制及 §3 客观判据 | **DIRECTOR APPROVED 2026-11-24**：仅批准 L0/L1/L2 三级与提案 §3 判据；**“L0/L1 封闭枚举 + 其余全部 L2”是 v1.2/v1.3 的收紧条款，不是 D-CR-1 已批准内容**，随本 Spec 的人类书面批准生效；不新增子级；判据变更须人类批准升版 | AC-75…AC-81、AC-97；INV-57/58/67/79/80 |
| D-CR-2 | L1 阶段批准委托对象 | **DIRECTOR APPROVED 2026-11-24**：QC + Reviewer 联签（非 Reviewer 单签）；仅 L1 封闭业务枚举适用；治理/工具链/canonical 为 L2，不得委托 | AC-82…AC-85、AC-98；INV-62/68 |
| D-CR-3 | QC 差分审查豁免 | **DIRECTOR APPROVED 2026-11-24**：批准小版本 delta 审查，保留主版本升级与 Release 前两次强制全量复审；Release 对象集按 AC-105 | AC-86…AC-89、AC-105；INV-63/74 |
| D-CR-4 | 错分级处置力度 | **DIRECTOR APPROVED 2026-11-24**：stop-the-line + 按 L2 重走 + Reviewer 台账记录 | AC-80/81；INV-59 |
| D-CR-5 | CI 是否纳入 `docs/**`、`prompt/**`、`.specify/**` | **DIRECTOR APPROVED 2026-11-24**：纳入三项，治理变更同受门禁约束 | AC-73；INV-61 |
| D-CR-6 | CI 是否也纳入根 `AGENTS.md`（及全部受管治理路径） | **OPEN**。选项：(a) 仅 D-CR-5 三项，根 `AGENTS.md` 仍可绕过 CI；(b) 追加根 `AGENTS.md`（最小补齐，推荐）；(c) 追加 `scripts/collab_gates.py:17-28` 的全部 MANAGED 路径。关闭前 `AGENTS.md` 按 L2 + 人类书面批准 + AC-100/AC-107 独立复核 | AC-73/AC-100/AC-107；不阻断本 Spec 其余条款 |
| D-CR-7 | L1 联签中 QC 的审查范围 | **OPEN**。选项：(a) 全量 QC 红队（与 GOV-TIER-01 估算收益冲突，`docs/cost-reduction-proposal.md:88`）；(b) delta + 受影响既有条款 + 未复审声明 + 执行证据（与 D-CR-3 精神一致，推荐；AC-82/AC-98 暂按此项编码）；(c) QC 仅签“未新增契约”声明（最省但风险最高） | AC-82/84/98；若选 (a)/(c)，本 Spec 升版 v1.3 |

- D-CR-1…D-CR-5 的批准来源为 Director 2026-11-24 的 canonical 记录（`docs/review-tracking.md:94`）；QC-01 时点不存在本记录；提案本身是未跟踪文档。本 Spec 自身的批准为 `approved_by=Director`/`approved_at=2026-11-24`（§14），与上述 D-CR 批准分开记录。
- D-CR-6 未关闭前，`AGENTS.md` 变更按 L2 + 人类书面批准 + AC-100/AC-107 独立复核执行；**不得**把 D-CR-5 的三项 CI 路径称为“保守默认”或完整覆盖。
- D-CR-7 未关闭前，L1 联签按推荐选项 (b) 编码执行；不得自行扩张为全量 QC 或缩减为仅声明。
- **归因说明**：D-CR-1 只批准“三级 + 提案 §3 判据”；v1.2/v1.3 的“L0/L1 封闭 + 默认 L2 + canonical/治理全覆盖”是更严格的收紧规则，已随本 Spec 2026-11-24 的人类书面批准生效，不得写成 D-CR-1 已批准。

---

## 11. 与既有契约的关系

- **优先级不变**：宪法 > AGENTS > CONVENTIONS（宪法 `:8`、`:73`、`:99`）；本 Spec 不与宪法 Article I–V、EARS 或 Governance 冲突。
- **命名与元数据**：文件名遵循 `NNNN-<domain>-<capability>.md`（`docs/specs/README.md:9-30`），domain=`platform` 属于词表（`:23`）；YAML 字段遵循必填清单（`:34-55`）；本 Spec 现行 `adr: [0008]`（ADR-0008 approved 2026-11-24），历史口径 `adr: []` 表示“尚未启动/绑定 ADR”，已不描述现状。
- **业务基线不动**：AC-01…AC-68、INV-01…INV-50 的语义、编号与验收要求不变（`docs/collab/ac-baseline.txt:68`；SPEC-0002 `:108-134`）；本 Spec 只新增治理 AC-69…AC-112 与 INV-51…INV-81。L0/L1 封闭、其余 L2；不新增子级/第四风险等级。
- **治理落点（批准后由对应 owner 修订，PO 不代改）**：
  1. `AGENTS.md` §2.5（`:76`）增补 L0/L1 封闭 + L2 默认分级表、AC-98 矩阵与错分停线；**§1.2（`:38-42`）同步 QC 职责**：QC 在 L1 联签中签治理声明与执行证据，不签代码质量；§1.5（`:58-64`）保持 Reviewer 代码审查与 Actor 独立；§6（`:109-116`）DoD 增补 G-3 治理证据；§7（`:120-126`）不变。
  2. `docs/collab/CONVENTIONS.md` §2（`:40-60`）增补分级判定；§2.1（`:62-66`）增补 QC delta；§5（`:140-151`）与 §6.1（`:157-163`）保持四种结论语义；§7.2（`:192-210`）债务清单更新；**§9（`:225-234`）角色最小动作更新**：QC 增加 L1 治理声明签署，Reviewer 增加联合记录与执行证据核对；CONVENTIONS/prompt 变更由 Reviewer 复核（`:27`）。
  3. `.github/workflows/go.yml`（`:4-23`）触发路径加入三项；`collab-gates` job（`:26-33`）继续 `make gates-strict`；门禁/CI 自保护按 AC-100，Independent Verifier 由 AC-107 定义且不由被审 diff 修改。
  4. `scripts/collab_gates.py`（G-4/`--release`/显式 range/措辞/G-3 治理证据/AC-106 bootstrap）与 Makefile 入口（`Makefile:162-169`）；不新增 G-5/G-6；GV-S19 注册表、GV-S3/GV-S10/GV-S20/GV-S21/GV-S22 canonical 载体与 `.specify/templates/evidence-matrix-template.md` 按 AC-109 指定 owner 后建立并登记 G-1 `MANAGED`。
  5. `docs/specs/README.md`（`:77-83`）索引登记 SPEC-0003 属 **PO**；`docs/collab/ac-baseline.txt` 与 `ac-coverage.tsv` 登记 AC-69…AC-112 属 **Reviewer**（`docs/collab/CONVENTIONS.md:31`、`:161`）；change-package/registry 与 audit/freeze 区建立/指针为 Lead（GV-S10 行由 Reviewer 写、GV-S20 行由结论 owner 写），release_scope 为 Director 或其指定 release owner，`evidence-matrix-template` 为 System Architect。
- **delta audit（INV-51 的观察证据）**：本 Spec 落点修改 AGENTS/CONVENTIONS/宪法/治理 Spec/prompt/scripts/CI/Makefile 后，Lead 与所有受影响 subagent 必须按宪法 `:94` 重读并在 `docs/review-tracking.md` 或对应 thread 记录 delta audit（日期、旧/新版本、差异摘要、重读确认）；Reviewer 复核 CONVENTIONS/prompt 的 delta audit（AC-104）。INV-55（fail-closed）由宪法 `:19`、AGENTS §0.3 与 ADR/Plan 的 fail-closed 声明承载，本 Spec 不另造 AC。
- **描述性 vs 规范性**：在本 Spec 批准前，脚本/CI 是现状事实，用于 §1.3 的漂移判定；批准后本 Spec 是规范性契约，脚本/CI 与本文冲突时必须修正实现或按流程升版，不得用“代码已经这样”静默废止（`AGENTS.md:124`；`docs/collab/CONVENTIONS.md:62-66`）。
- **ADR 关系**：本 Spec 现行 `adr: [0008]`：治理落点引入实现设计的 ADR-0008 已由 Director 于 2026-11-24 批准，故回填绑定；历史口径 `adr: []` 仅表示“本 Spec 自身未引入架构/接口设计、尚未绑定 ADR”，不再描述现状。治理落点按 L2 若引入新的设计/契约，仍由 Architect 另立 ADR 并回填；若仅落实已批准的治理规则且无设计变更，可依 AC-78 免 ADR，但不得免 Spec/QC/人类书面/Plan/Independent Verifier。
- **豁免关系**：D-CR-3 是对“QC 每版全量复审”的风险接受声明，不是对 fail-closed/stop-the-line 的豁免；其范围仅限小版本 delta，且受 AC-88/AC-105 收口（提案 `:93-103`）。

---

## 12. 生效、上线顺序与回滚

- **生效条件**：本 Spec 经复审 QC 与人类书面批准后生效；生效前不改变任何现行规则（提案 `:5`）。生效只涉及治理文档/CI/脚本/台账，不涉及业务数据迁移或服务上线。
- **实施顺序不可颠倒**：
  1. P0a G-3 bootstrap（AC-106/AC-112）：先扩展 G-3 接受治理证据（`test_type/evidence_path/status=VERIFIED` + digest + Reviewer 签名）。`BASE` = 第一个 P0a 提交的**前一提交**；legacy **G-1…G-3 PASS** 以显式 `--range=<BASE^>..<BASE>` 只评估 `BASE` 上已批准 Spec 版本的 AC-01…AC-68，AC-69…AC-112 不在 scope；G-4 在 `BASE` 上不存在，由 IV 记 `G-4 NOT-RUN(not-implemented-at-BASE)`（不阻断 P0a/P0b、永不计 PASS）。验证 = G-1…G-3 PASS + `G-4 NOT-RUN` 记录 + Independent Verifier 逐行 diff + 人类书面批准 + `BASE`/diff 摘要。P0a 仍属 L2，不得因 bootstrap 免除人类书面 Spec/Plan 批准；此阶段不得宣称任何治理 AC 为 AC-VERIFIED。
  2. P0b 门禁/CI 收口（AC-112）：Release mode、显式 range、CI paths、AC-100 自保护、AC-107 Independent Verifier、完整 G-4（四门禁输出与 `SUMMARY`/退出码）；G-4 在 P0b 落地并自 P0b 起为必需门禁；P0b 使用同一 scoped AC 集（AC-01…68 + 已批准 Spec 版本）与同一 `G-4 NOT-RUN` 记录，不得被 AC-69…112 的 NOT-RUN 阻断；P0b 仍属 L2，需人类书面 Spec/Plan 批准。
  3. P0.5 治理载体（AC-109）：GV-S19 注册表、GV-S3/GV-S10/GV-S20/GV-S21/GV-S22 canonical 区、`.specify/templates/evidence-matrix-template.md` 就绪并登记 G-1 `MANAGED`；未就绪前不得首次写入对应载体。
  4. P1 分级与委托（GOV-TIER-01 + GOV-HUM-01）：L0/L1 封闭 + L2 默认总函数、AC-98 矩阵、AC-101 执行证据接入。
  5. P2 QC delta 与 scaffold（GOV-QC-01 + GOV-SCAF-01）：必须在 Release 全量对象集（AC-105/AC-110）与 G-4 可执行之后。
- **前置条件**：P0a 前，人类书面批准 bootstrap（GV-S15 的 `approved_by/approved_at/scope/expiry` + GV-S20 的 `evidence` 同字段，`expiry` 必填）且 `BASE` 已记录；同时按 `docs/review-tracking.md:117-136` 的 commit timing，**已批准的 SPEC-0003 保持 untracked 直到 P0a**，P0a 原子落地（批准稿 + G-3 治理证据扩展 + 台账/base 播种），在此之前不得单独提交本 Spec，以保持已提交 HEAD 的门禁为绿；P0b 前，P0a 的 legacy gate/IV/摘要齐备；P1 启动前，G-4、`--release`、显式 range、AC 语义措辞、AC-100/AC-107 必须可用；P2 启动前，两个强制全量节点与 Release 对象集必须可执行并可核查。
- **bootstrap 口径**：P0a/P0b 期间治理 AC-69…AC-112 为 NOT-RUN（`G3_BOOTSTRAP_REQUIRED`）、Release 阻断；bootstrap 是有期限、有人类书面批准的 scoped exemption：GV-S15 记 `approved_by/approved_at/scope/expiry`（`expiry` 必填）、GV-S20 的 `evidence` 记 approver/date/expiry/scope、GV-S21 记 `g4_status=NOT-RUN(not-implemented-at-BASE)`/scoped AC 集/排除集/过滤命令（AC-112）；证据为 legacy G-1…G-3 PASS（显式 `--range=<BASE^>..<BASE>`）+ `g4_status` 记录，后者不阻断 P0a/P0b、永不计 PASS、不得作 Release/AC-VERIFIED 证据；AC-69 的 G-1…G-4 四项完整输出要求与 `GATE_INCOMPLETE` 不适用于 bootstrap 期“G-4 尚未实现”这一事实（仅此一条），该例外随 P0b 落地终止；到期自动恢复严格门禁。P0b 完成后，AC-100 的两次门禁对完整 AC 集且对 G-1…G-4 全部适用；不得用 bootstrap 代替 Release 或 AC-VERIFIED。不得以 G-3 未上线、预期失败或批量延期启动 P0。
- **delta audit**：每次治理落点变更完成后、下一次交付前，Lead 与所有受影响 subagent 必须按宪法 `:94` 完成 delta audit（AC-104）；未完成不得进入下一交付。
- **回滚**：治理规则不得由 Agent 静默回滚；回滚须人类书面批准并升版本 Spec；文档/CI/脚本变更以 git revert 表达，保留原变更记录、错分台账、豁免撤销、GV-S21 记录与审计行；不得删除历史。无业务数据回滚。
- **过渡期**：若人类批准分阶段生效，未生效阶段仍按现行规则执行；不得以“未来会放宽”或“仅文档/新载体已分类”为由提前跳过当前门禁。


---

## 13. 变更记录

| 版本 | 日期 | 变更 | 影响 |
|---|---|---|---|
| v1.0 | 2026-11-24 | 初稿 in-review：将 D-CR-1…D-CR-5 的批准结果编码为 GOV-CODE-01/TIER-01/HUM-01/QC-01/SCAF-01；保留 R-1…R-6 为 INV-51…INV-56；新增 INV-57…INV-66、AC-69…AC-96；记录提案现状漂移与 D-CR-6/D-CR-7 | 不影响业务 AC/INV；新增治理 AC/INV；QC-01 BLOCKED |
| **v1.1** | 2026-11-24 | 按 QC-01 Q-01…Q-21 整改：引入治理子级（v1.2 已移除）；L1 产物×批准者矩阵；G-3 治理证据路径与不批量延期；门禁/CI 自保护与 AGENTS 替代控制；GV-S17 人类分级裁决、GV-S18 门禁阻断原因、GV-S19 注册表、GV-S20 审计日志；release_verdict 与 release_block_reason 拆分；内容摘要绑定；change_id 全 ref 注册表；delta audit；Release 全量 QC 对象集；补全 QC 计数/G-3 能力/CI 绕过口径 | 新增 AC-97…AC-105、INV-67…INV-74；不改变业务 AC/INV；QC-02 判 0/3/8，17/21 CLOSED、4 PARTIAL |
| **v1.2** | 2026-11-24 | 按 QC-02 N-01…N-03、M-01…M-08 整改：**移除治理子级**，L0/L1 改封闭枚举、其余全部 L2（未知/新建/全部 canonical 亦然）；G-3 引导改为“扩展先上线，之前 NOT-RUN、Release 阻断”，bootstrap 用 legacy gate + 独立 diff 复核 + 摘要；Independent Verifier actor/路径/格式与两次门禁不一致阻断；GV-S21/S22、release owner/scope、effective_tier、canonical 登记/G-1 覆盖、status 枚举、GV-S3 head、GV-S20 共享日志例外；修正 review-tracking 行号口径 | 新增 AC-106…AC-110、INV-75…INV-79；更新 AC-75…AC-110/INV-52…INV-79/§2/§5…§12；不改变业务 AC/INV；QC-03 判 0/2/4 |
| **v1.3** | 2026-11-24 | 按 QC-03 N-04/N-05、M-A…M-D 整改：新增 per-change 证据工件随父变更继承 tier 的 scoping 规则（AC-111/INV-80），只有 carrier 结构/规则变更才 L2；新增 first-deployment P0a/P0b、BASE=首个 P0a 前一提交、scoped AC-01…68 与独立验证者/人类批准/到期制 bootstrap（AC-112/INV-81）；统一 Independent Verifier 定义与指定证据；AC-109 只认 G-1 MANAGED 并补 gate-runs owner；GV-S22 补 release owner 书面指定证据；修正 D-CR-1 的收紧规则归因；**approved 2026-11-24（QC-04 M-1…M-4 应用为编辑性澄清，无语义变更）**：GV-S15 `expiry`、GV-S20 `evidence` 承载 approver/date/expiry/scope、GV-S21 bootstrap scoped 运行字段、AC-107 `designated_by/independence_evidence`、AC-112 scoped 可复现证据、AC-111 append-only 边界 | 新增 AC-111/AC-112、INV-80/INV-81；更新 AC-97/98/100/106/107/109/110、§2、§12、§14；不改变业务 AC/INV；QC-04 判 READY（0/0/4） |
| **v1.4** | 2026-11-24 | 依 Director 对 `ADR-0008-QC-01` Q-02 [CONTRACT_GAP] 的书面裁决（`docs/review-tracking.md:117-136`，“2026-11-24 human decisions: GOV-COST-01 bootstrap, IV and commit timing”）修订 **bootstrap 证据口径**：AC-106/AC-112 改为 `BASE` 树 legacy **G-1…G-3 PASS**（scoped AC-01…AC-68、显式 `--range=<BASE^>..<BASE>`）+ 显式 `G-4 NOT-RUN(not-implemented-at-BASE)` 记录（不阻断 P0a/P0b、永不计 PASS、不授权 Release/AC-VERIFIED）；G-4 在 P0b 落地并自 P0b 起为必需门禁；AC-69 的 G-1…G-4 四项完整输出要求与 `GATE_INCOMPLETE` 对 bootstrap 期缺失 G-4 的豁免随 P0b 终止；该例外仅限 bootstrap、有时限、需人类批准（GV-S15/GV-S20 approver/date/expiry）；同步 §6/§12 交叉引用；并校正 review-tracking 行号（D-CR 证据 `:88`→`:94`；human decisions 区 `:54`/`:88`/`:112`→`:60`/`:94`/`:117`，与 `ADR-0008-QC-01` Q-13 同一漂移）。**approved 2026-11-24（QC-05 M-1…M-7 应用为编辑性澄清，无语义变更）**：M-1 在 INV-75/AC-100 的“legacy G-1…G-4”后加 AC-106 括号口径；M-2 把 AC-106/AC-112 的 AC-69 例外扩到四门禁输出要求与 `GATE_INCOMPLETE`；M-3 GV-S21 增 `g4_status`（`PASS\|NOT-RUN(not-implemented-at-BASE)\|NA`）并由 AC-112 引用；M-4 ADR QC 报告统一为 `ADR-0008-QC-01`；M-5 v1.3 批准摘要统一为 `75eb8f7d…`（689 行）并删除冲突的 pre-record 摘要；M-6 §12/§14 记载批准稿 untracked 至 P0a、P0a 原子落地；M-7 变更记录补列 §5.1/§9/头部元数据 | 无其他语义变更：主体为 AC-106/AC-112 的 bootstrap 证据与 §6/§12 交叉引用，另改 §5.1（G-3 引导状态机行）、§9（登记状态）、头部元数据（`status/version/approved_by/approved_at`、版本/状态/被审版本绑定）与 §13/§14 记录（含行号校正）；AC 编号、AC-01…68、其余 AC-69…112 与 INV 文本不变；不弱化 fail-closed、不新增批量延期；QC-05 判 READY（0 BLOCKER/0 MAJOR/7 MINOR），已批准生效 |
| **v1.4** | 2026-11-24 | v1.4（2026-11-24）：ADR-0008 approved；回填 adr: [0008]（元数据，无业务语义变更）。仅回填 YAML `adr` 字段、头部下游行与 `docs/specs/README.md` §4 索引 ADR 链接；版本保持 1.4，`approved_by/approved_at` 不变 | 无业务规则变更：AC/INV 编号与文本均未改动 |

---

## 14. 批准记录

| 角色 | 结论 | 日期 |
|---|---|---|
| Director（人类指挥官） | 批准 D-CR-1…D-CR-5 全部推荐项；canonical 记录 `docs/review-tracking.md:94`（approver Director，scope=all recommendations） | 2026-11-24 |
| Quality Checker | QC-01 对 v1.0：BLOCKED（0/10/11）；QC-02 对 v1.1（sha1 `15dc9be8…`）：BLOCKED（0/3/8，17/21 CLOSED、4 PARTIAL）；QC-03 对 v1.2（sha1 `b7e150dd…`，HEAD `f64434a`）：BLOCKED（0/2/4；N-01/N-02 CLOSED、N-03 PARTIAL、Q-21/M-01…M-08 CLOSED）；QC-04 对 v1.3（sha1 `15ad2979…`，HEAD `f64434a`）：**READY FOR HUMAN SPEC APPROVAL（0 BLOCKER/0 MAJOR/4 MINOR；N-04/N-05、M-A…M-D CLOSED）**；QC-05 对 v1.4（sha1 `79db9323…`，delta 审查）：**READY FOR HUMAN SPEC v1.4 APPROVAL（0 BLOCKER/0 MAJOR/7 MINOR；M-1…M-7 已作为编辑性澄清并入 v1.4 批准稿）**，报告 `docs/collab/qc-reports/SPEC-0003-QC-05.md` | 2026-11-24 |
| Product Owner | 产出 v1.0，按 QC-01/QC-02/QC-03 整改为 v1.1/v1.2/v1.3；将已批准 v1.3 落为 approved 并应用 QC-04 M-1…M-4 编辑性澄清（不升版）；将已批准 v1.4 落为 approved 并应用 QC-05 M-1…M-7 编辑性澄清（不升版）；仅写业务规则/AC/INV 与索引登记，未改代码/治理文件 | 2026-11-24 |
| Director（人类指挥官） | **批准 SPEC-0003 v1.3**（session directive `SPEC_APPROVED`，2026-11-24）：接受 QC-04 的 4 项 MINOR（M-1…M-4）作为批准时编辑性澄清并入，不另升版；QC-04 结论 READY（0/0/4）；`approved_by=Director`、`approved_at=2026-11-24`；批准稿 = **689 行、sha1 `75eb8f7d52e0b7d8d1f1c5e43237e61979dca263`**（canonical，见 `docs/adr/0008-governance-cost-reduction.md:10`、`:797` 与 `docs/collab/qc-reports/ADR-0008-QC-01.md:7`；QC-05 M-5 已删除与 canonical 冲突的 pre-record 摘要，本行不再保留旧值） | 2026-11-24 |
| Director（人类指挥官） | **2026-11-24 裁决 `ADR-0008-QC-01` Q-02 [CONTRACT_GAP]：采用 SPEC-0003 v1.4 amendment**（canonical 记录 `docs/review-tracking.md:117-136`，“2026-11-24 human decisions: GOV-COST-01 bootstrap, IV and commit timing”）：bootstrap 证据 = `BASE` 树 legacy G-1…G-3 PASS（scoped AC-01…68、显式 `--range=<BASE^>..<BASE>`）+ `G-4 NOT-RUN(not-implemented-at-BASE)` 记录（不阻断 P0a/P0b、NOT-RUN 永不计 PASS）；G-4 于 P0b 落地并自 P0b 起必需；该例外仅限 bootstrap、有时限、需人类书面批准（GV-S15/GV-S20），不授权 Release 或 AC-VERIFIED；v1.4 随后于同日经 QC-05 与人类书面批准（见下行） | 2026-11-24 |
| Director（人类指挥官） | **批准 SPEC-0003 v1.4**（2026-11-24）：依上述 canonical 裁决（`docs/review-tracking.md:117-136`，“2026-11-24 human decisions: GOV-COST-01 bootstrap, IV and commit timing”）批准 v1.4；QC-05 结论 **READY FOR HUMAN SPEC v1.4 APPROVAL（0 BLOCKER/0 MAJOR/7 MINOR，`docs/collab/qc-reports/SPEC-0003-QC-05.md`）**，7 项 MINOR（M-1…M-7）作为批准时编辑性澄清并入，不另升版；`approved_by=Director`、`approved_at=2026-11-24`；v1.4 仅修订 bootstrap 证据口径，AC 编号与其余 AC/INV 语义不变；批准稿按 commit timing 保持 untracked 至 P0a（见下行落库依赖） | 2026-11-24 |

- **归因边界**：D-CR-1 只批准 L0/L1/L2 三级与提案 §3 判据；“L0/L1 封闭 + 默认 L2 + canonical/治理全覆盖”是 v1.2/v1.3 的收紧规则，已随本 Spec 2026-11-24 的人类书面批准生效，不是 D-CR-1 已批准内容。
- **被审版本绑定**：QC-03 审 v1.2 的 sha1 为 `b7e150dde6b1ba06992184cf068b88b7564e7bb0`，HEAD 为 `f64434a90f21a523de2524676ef5eb228af615fa`；QC-04 审 v1.3 的 sha1 为 `15ad29798f3c07ba6d6ac7571c9f3e5790de3061`、HEAD 同上，结论 READY（0/0/4）；v1.3 批准稿仅应用 QC-04 M-1…M-4 编辑性澄清，AC/INV 语义不变。QC-05 审 v1.4 的 sha1 为 `79db93233120fcfce9f5bd4b9a0c125b183d8d28`（696 行，in-review 稿），结论 READY（0/0/7）；获批 v1.4 正文 sha1 于落库时计算并登记 GV-S20。
- **摘要血统（digest lineage，元数据，不改变版本）**：v1.4 获批内容摘要 = `58679f03e417b98a591545c84854cda3aaa6b898`（697 行；`approved_by=Director`/`approved_at=2026-11-24`）；获批后仅做 `adr: [0008]` 元数据回填（YAML `adr`、头部下游行、§13 记录、`docs/specs/README.md` §4 链接、§11 `adr` 表述与本条/§14 状态行），回填稿摘要 = `1371eafc62350794d2c50af8ef9a304ac18236bb`（698 行，回填后、本轮 §11/§14 一致性说明前的工作树摘要），AC/INV 编号与文本零变更，版本仍为 1.4；**review/Release 的 canonical 绑定是 P0a 提交中的 Spec blob**（含全部元数据回填的 git blob 摘要），不是未跟踪草案的工作树 sha1；未跟踪草案摘要仅为评审时点记录（point-in-time），不得作 canonical 绑定或 Release 证据。
- **v1.4 状态（approved，2026-11-24）**：v1.4 按 `docs/review-tracking.md:117-136` 的 Director 裁决修订 AC-106/AC-112 的 bootstrap 证据口径，并入 QC-05 M-1…M-7 编辑性澄清；AC 编号与其余 AC/INV 文本不变。v1.4 **已批准**：`approved_by=Director`、`approved_at=2026-11-24`；QC-05 结论 READY（0 BLOCKER/0 MAJOR/7 MINOR，`docs/collab/qc-reports/SPEC-0003-QC-05.md`）。ADR-0008（治理落点实现设计）Rev 1.1 已由 Director 于 2026-11-24 批准（`docs/adr/0008-governance-cost-reduction.md`），其规范绑定为**已批准**的 SPEC-0003 v1.4。
- **落库依赖（非豁免）**：v1.4 批准同轮，`docs/specs/README.md` §4 索引与 §13 版本由 **PO** 更新为 1.4（G-2，`scripts/collab_gates.py:216-223`、`:237-242`）；按 `docs/review-tracking.md:117-136` 的 commit timing，**已批准的 SPEC-0003 保持 untracked 直到 P0a**，P0a 原子落地（批准稿 + G-3 治理证据扩展 + 台账/base 播种），在此之前不得单独提交本 Spec；`ac-baseline.txt`/`ac-coverage.tsv` 的 AC-69…AC-112 仍待 **Reviewer** 登记（G-3，`docs/collab/CONVENTIONS.md:31`、`:161`），GV-S20 审计行与 GV-S21 `g4_status` 记录随 P0a 产生。G-3 扩展按 AC-106 bootstrap；扩展上线前治理 AC 为 NOT-RUN。G-3 登记完成前其 FAIL/NOT-RUN 是预期状态，不得视为已通过。
