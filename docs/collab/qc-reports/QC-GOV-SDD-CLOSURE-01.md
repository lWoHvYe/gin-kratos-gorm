# QC-GOV-SDD-CLOSURE-01 — PO prompt 单行「批准回写」规则红队审查（r2）

> 本文件仅为 QC 报告；`READY` 不构成人类批准、不构成 merge/Release，也不替代 Reviewer verdict。只读审查：未改 `prompt/Product-Owner.md` 或其他文件，未 commit。

| 项 | 值 |
|---|---|
| change_id | `GOV-SDD-CLOSURE-01` |
| Tier | `L2`（治理文本轻路径：`AGENTS.md:84`、宪法 `:77`；仅 `prompt/**`，无代码/契约/CI/`scripts/**`；治理文本恒 L2，`AGENTS.md:85`） |
| base | `f1946e74db7629157d9415ec29163dc8c1de8cfd`（HEAD 未变；`prompt/Product-Owner.md` blob `cd3b56e4e5b6`） |
| head | 未提交工作树 blob **`453eb3dad91d`**（r1 为 `63cd28f7b84d`）；`git diff` 仍 `@@ -15,6 +15,7 @@`，`+1/-0`（仅 `:18`） |
| review_scope | 仅该一行 diff（r2 文本）。对照：宪法 v1.3.0、`AGENTS.md`、`CONVENTIONS.md`、`prompt/Reviewer.md`、`docs/specs/README.md`、`Spec.md`、`ac-coverage.tsv`、`ac-baseline.txt`、`docs/review-tracking.md`（`66331aa`、GOV-SDD-HYGIENE-01、`:420-423`、`:963-969`）、`.specify/.gitignore`。不审代码/契约；纯文档，未跑测试，不构成 AC 验收或 Release 证据 |
| Actor | `qc-closure` |
| verdict | **`READY`**（r1 为 BLOCKED）— Round 2：**0 BLOCKER / 0 MAJOR / 5 MINOR**，`A-1/C-1/D-1` 均 CLOSED，无新增问题 |
| release_status | **`NOT RELEASE`** |

## Delta 复核 r2（仅比对 `:18`，base 未变）

- **C-1 CLOSED**：r2 已删除 `docs/po/drafts/speckit-*` 与 `.specify/feature.json` 整个分支；`grep -inE "speckit|SPECIFY_|feature\.json" prompt/` → 0 命中（`EXIT=1`）。不再存在 Spec Kit 路径/命名边界声明，与人类 2026-11-26 裁决（`docs/review-tracking.md:352-354`、`:370`）一致；canonical 仍由 `docs/specs/README.md:3`、`Spec.md:3` 承载，本行不重述属正确最小化。
- **A-1 CLOSED**：r2 已点名 `ac-baseline.txt` 与 `ac-coverage.tsv` 两处登记，且"台账 owner 恒为 Reviewer，PO 不写台账"，与 `CONVENTIONS.md:31`、`:191`、宪法 `:49` 及历史协议 `docs/review-tracking.md:966-967` 一致；"登记台账"未被写成 PO 职责。
- **D-1 CLOSED**：r2 已无 gitignored 本地指针条款，不再有不可核对项；仅剩 D-2 MINOR。
- **A-2 N/A**：r2 未再要求删除草稿，"同轮先删导致映射证据丢失"的时序风险随该子句消失。
- **新问题：无**。角色边界（编号由 PO 在 Spec 内给出、注册归 Reviewer）、EARS（"按本文件 EARS 重写"＋`:20-21`／宪法 `:65-71`）、路径（无新路径约定）、范围（仍 1 文件 1 行、`:6-7`/`:20-21` 未动）均无新增缺陷。历史裁决记录 `docs/review-tracking.md:353/:370` 仍含 "Spec Kit" 字样，属人类决策留档，不得删除（`AGENTS.md:87`），非违规。

## A. 角色边界（PASS）

- 台账 owner 恒为 Reviewer，PO 不写台账；编号仅作 Spec 内 AC 标签，注册由 Reviewer 完成——均在上限内。
- **MINOR A-3**："给出编号"未说明与 `ac-baseline.txt` 现有序列衔接（现止于 `AC-68`）；建议补"编号须接续 `ac-baseline.txt` 序列，Reviewer 登记后生效"，避免重号（`CONVENTIONS.md:269`）。

## B. EARS（PASS）

- "新增 AC 必须按本文件 EARS 重写"配合同文件 `:20-21` 与宪法 `:65-71`，已阻断 Gherkin 直接入库。
- **MINOR B-1（open）**：建议显式写"不得以行首 `Given/When/Then` 步骤替代 EARS"（宪法 `:71`）。

## C. 路径/canonical（PASS）

- 无 Spec Kit 路径边界、无 `feature.json` 工具条款；未把 `specs/` 声明为 canonical（`docs/specs/README.md:3`、`Spec.md:3`）。

## D. 可执行性（PASS）

- 两处索引同步可逐格核对：`Spec.md:7-10` 与 `docs/specs/README.md:79-82` 均有「状态/版本」列；"当轮"与 `docs/specs/README.md:88`、`Spec.md:22`"approved 即入库"一致。未新增工具/门禁/状态字段（`CONVENTIONS.md:102`）。
- **MINOR D-2（open）**：未含该 Spec 自身 YAML `status/version/approved_by/approved_at` 与「变更记录」（`docs/specs/README.md:36-48`、`:70-72`、`:98`）；建议句末补"并回填该 Spec 文首 YAML 与变更记录"。

## E. 范围（PASS）

- `git status --porcelain` 只有 `prompt/Product-Owner.md`（M）与本 QC 报告（??）；`git diff --numstat` = `1 0 prompt/Product-Owner.md`；未动 EARS 段 `:20-21`、绝对边界 `:6-7` 或其他 prompt 文件；属 `prompt/**` 治理文本轻路径（`AGENTS.md:84`、宪法 `:77`）。
- HEAD 仍为 `f1946e7`（r1 审查后未再前进），base/范围未漂移（`CONVENTIONS.md:128`）。
- **MINOR E-1（open）**：`prompt/**` 唯一作者为 System Architect（`CONVENTIONS.md:26`），Brief 仍未记录起草 actor；若 PO 自改则违规。人类批准时须在 `docs/review-tracking.md` 记录批准范围与 delta audit（宪法 `:96`）。

## F. 与既有条款冲突（无硬冲突）

- 未破 §2.3 范围守卫、未改 §6.3 延期字段；台账 owner 仍为 Reviewer；Reviewer 独立性无影响（宪法 `:94`、`AGENTS.md:63`）。
- **MINOR F-1（open）**：该行仍复述 `docs/specs/README.md:86-99`、`Spec.md:22`（源头为 `docs/review-tracking.md:963-969` 已标 superseded 的历史 gate note）；存在第二真源漂移风险（`CONVENTIONS.md:11`、`:273`）。建议改为指针引用。

## 结论

- **verdict：`READY`（Round 2）** — 0 BLOCKER / 0 MAJOR；BLOCKER C-1、MAJOR A-1/D-1 已 CLOSED，无新增问题；剩余 MINOR 5：`A-3`、`B-1`、`D-2`、`E-1`、`F-1`（均建议项，不阻断人类批准）。
- 最小增量仍限 `prompt/Product-Owner.md` 一行，不新增工具/字段/门禁；**`NOT RELEASE`**。
- 本报告不构成人类批准、不构成 merge 或 Release 批准；人类批准与 Reviewer 复核检查点仍须各自完成。
