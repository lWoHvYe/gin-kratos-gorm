# ROLLOUT 阶段 A-1 自查报告

| 项 | 值 |
|---|---|
| 基线 | `HEAD 9e81497`（2026-11-27 07:37 +08），工作区仅 `docs/convert/` 未跟踪 |
| 性质 | 作者侧自查（A-1），逐项交人类；**不是**审查裁决、**不是**批准、**不是** Release 证据 |
| 范围 | `docs/convert/ROLLOUT.md` §6 的 8 项；附带核对 §3 裁决题的引用行 |
| 未改动 | 未改 `AGENTS.md`、宪法、`CONVENTIONS.md`、台账、`ac-coverage.tsv`、任何代码；未 commit |
| 结论 | A-1 完成；A-2 暂缓（无批准记录可登记）；B/C/D 均被人裁决项阻断；另发现 ROLLOUT 自身 8 处与 HEAD 不符 |

> **2026-11-27 更新**：11 项裁决已出（见 `DECISIONS.md`）；B-1…B-6 已执行并记录在 `docs/review-tracking.md:458`。本文件的行号引用仍以改动前 `HEAD 9e81497` 为准；B 阶段后的台账行号已整体后移。

---

## 0. 摘要（先看这里）

1. **Spec Kit 1.4.3 已安装**。`implement` 默认可无人值守跑完 `tasks.md` 的全部阶段，没有“每阶段停下”的内建闸门；`speckit-specify` 生成的 Spec 缺附录 A 的 5 个必备章节，且模板用行首 `Given/When/Then`，与 EARS 要求直接冲突。
2. **`.specify/`（21 个文件）与 `.dsh/skills/`（9 个文件）都被 git 跟踪**。`specify refresh` 在 1.4.3 **不存在**；实测 `specify init --here --force` 会覆盖核心模板，但**保留** `.specify/templates/overrides/` 和宪法。C-2 若只追加章节，应走预设/扩展层的 `append`；项目 override（`.specify/templates/overrides/spec-template.md`）是**整体替换**。
3. **停用文件的引用面很大**：`CONVENTIONS.md` / `ac-coverage.tsv` / `ac-baseline.txt` / `handoff.md` / `backlog.md` 共 **507 处**，`prompt/*.md` **90 处**；其中大量在历史 QC/REV 报告与只读 pair 存档里，现行规则禁止改写存档（`CONVENTIONS.md:145-147`）。删除会造成大量悬空引用。
4. **CI 不覆盖治理文件**：`.github/workflows/go.yml` 的 `paths` 不含 `docs/**`、`prompt/**`、`.specify/**`、`specs/**`；没有 `Tier:` 尾注校验；`.git/hooks` 只有 `.sample`，`core.hooksPath` 未设置。
5. **台账确实自相矛盾**：48 条 AC 行（32 DEFERRED / 9 AC-REF / 7 AC-VERIFIED）；`PAY-SETTLE-01` 有 5 处冻结记录、**没有任何显式解冻**；AC-27 在“AC-REF / AC-VERIFIED”之间反复；AC-72 行状态与 reason 相反；AC-50-FIX-01 仍写 FROZEN 但 AC-50 已 VERIFIED。
6. **替换稿漏掉 12 类现行约束**（见 §7），其中影响最大的是：外部复审只能作问题提案、延期/到期规则、P0/跨服务变更的 Plan 门禁、测试替身与启动 fail-fast、退款 permit/commitment 恢复规则、证据链 `文件:行号`。
7. **ROLLOUT 引用行有 8 处硬性不符**（见 §10），包括台账实际 **1254 行**而非 1268、`SPD-` 应为 `SDD-`、两处把行号范围写反、`docs/Lead` 实际是 `docs/lead`、以及“无 git”与事实不符。

**因此：在人类对裁决 1–4（及 9、11）作出决定前，不执行 B/C/D。** A-2 台账登记也暂缓——它要写“本次简化取代了哪些旧项”，而简化尚未获批。

---

## 1. 自查 1：Spec Kit 行为

只读了已安装的 1.4.3 脚手架（`specify --version` = 1.4.3）；未修改 `.specify/` 或 `.dsh/`。

### 1.1 `implement` 会不会无人值守连续执行 / 多 agent 并行 / 每阶段停下

| 问题 | 事实 | 证据 |
|---|---|---|
| 连续执行 | 会。它解析 `tasks.md` 后按阶段执行，**完成一个阶段就进入下一个**，只在 checklist 有未勾选项时询问；`Done When` 要求“tasks.md 全部任务完成并标 `[X]`” | `.dsh/skills/speckit-implement/SKILL.md:154`（Phase-by-phase execution）、`:224`（All tasks completed） |
| 多 agent 并行 | 不强制。`[P]` 任务被描述为“can run together”，由执行者自行决定；skill 本身不生成 subagent | `:155`、`:169-170` |
| 每阶段停下 | **没有内建停点**。阶段完成只做“验证”，不要求人类确认；`$ARGUMENTS` 可以被用来限定范围，但默认是全量 | `:156`、`:158` |

与 AGENTS 替换稿 §2 的“任何实现工具都不得无人值守连续执行：一个阶段一批，批后停下”（`docs/convert/AGENTS.draft.md:35`）**冲突**：替换稿把“分批停下”写成规则，但工具默认不提供。需要作者每批显式限定 `tasks` 范围或在阶段边界主动中止；这不是工具能保证的。

### 1.2 `plan` / `implement` 的宪法检查读哪个路径

- `speckit-plan`：`Read FEATURE_SPEC and .specify/memory/constitution.md`，并把 `Constitution Check` 填进 plan 模板（`.dsh/skills/speckit-plan/SKILL.md:60,64`）。
- `speckit-implement`：`IF EXISTS: Read .specify/memory/constitution.md`（`.dsh/skills/speckit-implement/SKILL.md:100`）。
- `speckit-specify`：`IF EXISTS: Load .specify/memory/constitution.md`（`.dsh/skills/speckit-specify/SKILL.md:115`）。

结论：三条路径都读**同一份** `.specify/memory/constitution.md`，C-1 替换该文件即可生效，无需改工具配置。

### 1.3 `specify` 生成的 Spec 缺附录 A 的哪些章节

模板：`.specify/templates/spec-template.md`；附录 A：`constitution.draft.md:79-89`。

| 附录 A 必备章节 | 模板现状 | 缺口 |
|---|---|---|
| 业务规则与验收标准（EARS AC） | `Requirements > Functional Requirements` 用 `MUST` + `FR-001`，`User Scenarios` 用行首 `Given/When/Then` | **冲突**：没有 AC 编号，且 Gherkin 行首句式正是现行宪法 `.specify/memory/constitution.md:71` 明令禁止的规避方式 |
| 状态所有权 | 无 | **缺** |
| 契约与异常恢复 | 仅有 `Edge Cases` 占位 | **缺**（幂等、最终一致性、持久恢复所有者、死胡同） |
| 可达性检查（`文件:行号`） | 无 | **缺** |
| 假设与不确定性 | `Assumptions`（`spec-template.md:120`） | **部分**：只有假设，没有“底层依赖风险” |
| 业务裁决题（带编号选项） | 无（`NEEDS CLARIFICATION` 不是裁决题） | **缺** |
| 验收状态表 | 无；只有 `Success Criteria` / `SC-001` | **缺**（无状态、检查类型、证据路径、审查者列） |

### 1.4 与 AGENTS 替换稿 §2 的其它冲突

1. **tasks 勾选是第二状态源**：`implement` 会把完成项标 `[X]`（`SKILL.md:173`），而宪法替换稿第 27 条说“plan、tasks 的勾选…不保存状态”（`constitution.draft.md:54`）。直接冲突。
2. **目录/`change_id` 不对齐**：Spec Kit 用 `specs/NNN-feature/spec.md` 目录；本仓用 `docs/specs/0002-....md` 扁平文件。替换稿第 30 条要求 `change_id` 取“Spec 所在目录的名称”（`constitution.draft.md:57`），但现有 Spec 没有独立目录。这属于 `SDD-STRUCT-ALIGN-01` 未决问题（`docs/review-tracking.md:354,440`），C-2 只加章节标题解决不了。
3. **Gherkin 与 EARS 二选一**：C-2 若只“加七个章节标题”，模板仍保留 `Given/When/Then`；必须同时把 `User Scenarios` 的验收句式改成 EARS，或明确“模板句式不构成 AC，AC 只写在验收状态表”。否则每份 Spec 都会天然违反附录 A。

---

## 2. 自查 2：`.specify/` 与 `.dsh/` 的跟踪与覆盖层

| 问题 | 结果 | 证据 |
|---|---|---|
| 是否被 git 跟踪 | **是**。`.specify/` 21 个文件、`.dsh/skills/` 9 个 `SKILL.md` 均被跟踪 | `git ls-files .specify` / `.dsh`；`.gitignore:66-70` 明确“IS tracked” |
| 机器本地状态 | `.specify/feature.json`、`integrations/packages*`、`extensions/*/local-config.yml` 被 `.specify/.gitignore` 排除 | `.specify/.gitignore` |
| `specify refresh` 是否存在 | **不存在**（1.4.3 无 `refresh` 子命令） | `specify refresh --help` → `No such command` |
| `specify init --here --force` 会不会覆盖 | **会覆盖核心模板**（实测：给 `spec-template.md` 加的 marker 被抹掉）；**保留** `templates/overrides/` 与 `memory/constitution.md` | 在临时 git 仓库复制 `.specify` 后实测 |
| 覆盖层放哪里 | `.specify/templates/overrides/<template-name>.md`（优先级 1） | `.specify/scripts/bash/common.sh:642-650` |
| 覆盖层语义 | **整体 replace**，不是 append：命中 override 就直接 `cat` 并 `return 0` | 同上 |
| 追加式扩展 | 预设层/扩展层支持 `replace/prepend/append/wrap/replace-section`；`speckit-specify` 明确说 resolver 会组合这些层 | `.specify/scripts/bash/common.sh:626-640`；`.dsh/skills/speckit-specify/SKILL.md:95` |

结论：C-2 如果只是“加章节标题”，应走**预设/扩展层的 append**（例如 `.specify/presets/` 下一个 `preset.yml` 声明 `spec-template` + `strategy: append`），而不是 `templates/overrides/` 的整体替换；若走 override，必须维护完整模板。`specify refresh` 不存在，不用担心它覆盖；但 `specify init --here --force` 会覆盖核心模板，不会覆盖 override。ROLLOUT C-2 的描述“覆盖层按 append/prepend/wrap 追加”对**项目覆盖层不成立**，对预设/扩展层成立。

---

## 3. 自查 3：停用文件的引用清单

用两组检索：停用文件集合（`CONVENTIONS.md|ac-coverage.tsv|ac-baseline.txt|handoff.md|backlog.md`）命中 **507 处**；`prompt/{Lead,PO,QC,Architect,Developer,Reviewer}.md` 命中 **90 处**。命中最多的是历史审查报告与存档：

| 文件 | 命中数 |
|---|---|
| `docs/collab/qc-reports/QC-GOV-SDD-HYGIENE-01.md` | 83 |
| `docs/collab/qc-reports/REV-GOV-SDD-HYGIENE-01.md` | 42 |
| `docs/review-tracking.md` | 19 |
| `docs/adr/0006-collab-gates.md` | 19（已 superseded/retired） |
| `docs/collab/qc-reports/QC-29-BATCH-B.md` | 18 |
| `docs/collab/developer-reviewer/threads/T-001.md` | 14（只读存档） |
| `docs/specs/0002-payment-settlement-admission.md` | 7（已批准契约） |
| `docs/ext-review-1123-triage.md` | 7（历史） |

**必须处理的活引用**（C-4 的“逐处处理”）：

| 活文件 | 引用 | 处置 |
|---|---|---|
| `README.md:222` | `docs/handoff.md` | 改为指向台账/AGENTS |
| `Spec.md:35,37` | `docs/po/backlog.md`、`docs/collab/CONVENTIONS.md` | 删除/改指 Spec 与台账 |
| `docs/development.md:7,98,111` | `handoff.md`、`ac-coverage.tsv` | 改指 `docs/review-tracking.md` / Spec 验收表 |
| `scripts/bootstrap-context.sh:14,32` | `docs/handoff.md` | 改指向 `docs/review-tracking.md` |
| `docs/handoff.md:86-88` | `docs/lead/PAY-SETTLE-01/{backlog,tasks,evidence-matrix}.md` | 文件本身停用，内部引用随文件处置 |
| `prompt/Product-Owner.md:18` | `ac-baseline.txt`、`ac-coverage.tsv` | 文件停用 |
| `prompt/Developer.md:3-4` | `CONVENTIONS.md`、`handoff.md` | 文件停用 |
| `prompt/Lead.md:20,36` | `CONVENTIONS.md` | 文件停用 |
| `docs/{po,developer,reviewer,architect}/MEMORY.md` | `CONVENTIONS.md`、`ac-coverage.tsv`、`handoff.md` | 按 C-4 清理状态行 |

**约束**：现行规则禁止删除/改写 pair 存档（`docs/collab/CONVENTIONS.md:145-147`）与历史线程；历史 QC/REV 报告属于审查证据，删掉引用目标后会出现大量悬空路径。若选“删除”（裁决 4 A），这些历史文件的链接会永久指向不存在的路径；若选“移到 `docs/archive/`”（B），路径同样改变。**建议**：B + 在旧路径保留一行“已归档，见 docs/archive/…”的 stub，或明确接受历史文档的悬空引用（历史证据不要求可导航）。

---

## 4. 自查 4：CI 触发、Tier 尾注、commit hook

| 检查 | 结果 | 证据 |
|---|---|---|
| `docs/**` 触发 CI | **否** | `.github/workflows/go.yml:5-23`、`frontend.yml:6-13` |
| `prompt/**` 触发 CI | **否** | 同上 |
| `.specify/**` 触发 CI | **否** | 同上 |
| `specs/**` 触发 CI | **否**（`docs/openapi/**` 只触发 frontend） | `frontend.yml:6-13` |
| 写死 `Tier: L0|L1|L2` 的尾注/校验 | **无** | 全仓 `Tier:` 仅出现在文档，不在 CI/脚本 |
| commit hook | **无**（`.git/hooks` 只有 `.sample`；`core.hooksPath` unset） | `ls .git/hooks`；`git config core.hooksPath` |
| 副作用 | `scripts/**` 会触发 Go CI（全量 test/race/build），治理脚本改动会跑昂贵流水线 | `go.yml:8` |

---

## 5. 自查 5：台账未关闭事项

> 台账物理顺序不是时间顺序：`:14-318` 是 11-24/25 混合，`:320-457` 按日期升序，`:458-1003` 按日期降序，`:1044-1165` 是旧路线图。以下“最新”按 `git log`/标题日期判定，最新条目是 `:458`（11-27 T2.4 scoped PASS）。

### 5.1 冻结

| 项 | 状态 | 证据 | 是否被显式解冻 |
|---|---|---|---|
| `PAY-SETTLE-01` | 冻结 | `review-tracking.md:175`、`:198`、`:741`、`:789`、`:982` | **没有任何一条显式解冻**；只有 `:622-624` “resume Phase 0-R”、`:643-644` “only for R0.3-R1c”、`:680-681` “rest of Phase 0-R” |
| `AC-50-FIX-01` | FROZEN | `:850` | 无解冻条目；但 `:637` 说“已由 L2 交付”，`ac-coverage.tsv:19` 已 AC-VERIFIED |
| `PAY-AC50-REVIEW-01` | 冻结→解冻 | `:705` 冻结；`:683,692` “freeze lifted / unfrozen for T7/T8 only” | **唯一有明确解冻记录的** |
| `T1.4b2` / `T1.6` / `T1.4b1` 等 | 历史冻结 | `:29,67,115,137` | 多数被后续 PASS 隐含取代，无逐条解冻句 |

### 5.2 豁免

| 范围 | 期限 | 不豁免什么 | 证据 |
|---|---|---|---|
| ADR/SPEC 修订不需单独人类批准（PAY-SETTLE-01 WIP） | P0 收口/合并或 2026-12-31，先到者 | merge、Release、AC-VERIFIED 不豁免 | `:187-191` |
| `PAY-OPS-REPLAY-01` 人工残留风险（4 项） | 2026-12-31 | 不豁免 Release/merge/其他 AC | `:573-586` |
| 人工 ops 流程替代工具入口（C+A） | 2026-12-31 | 不豁免 Release/merge；影响 INV-45、AC-61/62 | `:588-599` |
| 六条 AC（AC-22/28/29/30/35/37） | **2026-12-15**，决定期限 2026-12-08 | — | `:973`；`ac-coverage.tsv:2-7` |
| AC-39…68 批量延期 | 2026-12-31 | 引用≠验收 | `ac-coverage.tsv:8-37` |

### 5.3 AC 状态（`ac-coverage.tsv` 48 条数据行）

- **DEFERRED 32**、**AC-REF 9**、**AC-VERIFIED 7**。
- 非 AC 编号行：`:48` `REVIEW-REQUIRED-ALERTS-01`（owner `ops`）、`:49` `REVIEW-REQUIRED-ENTRY-01`（owner `Lead/BFF/ops`）。
- 矛盾：
  1. **AC-72** `:47` 状态 `AC-VERIFIED`，reason 明说“backend segment only … AC-72 not overall verified”；同时 `:48-49` 又把同一 AC 的未交付段记为 DEFERRED。
  2. **AC-63** `:32` = DEFERRED，台账 `:465,473` 写 AC-REF，`:495` 写 AC-60/63 stay AC-REF。
  3. **AC-27** `:41` = AC-VERIFIED（waiver C+A），但 `:676-681` 最新 OPS-P4 条目仍写 AC-27 remains AC-REF；`tasks.md:57` 写“未关闭前 AC-27 不得 AC-VERIFIED”；且 `:41` 的 thread 写 `PAY-SETTLE-01`，而 C+A 豁免范围是 `PAY-OPS-REPLAY-01`（`:588-595`）。
  4. **AC-50** `:19` = AC-VERIFIED，但其前置 `AC-50-FIX-01` 在 `:850` 仍 FROZEN 且无解冻。
  5. **AC-62** `:31` = AC-REF，reason 是“功能未实现”，本质是延期而非“有引用”。

### 5.4 无 owner / 多 owner

- `SDD-STRUCT-ALIGN-01`：无 owner、无 expiry（`:354,440`）。ROLLOUT B-7 写成 `SPD-STRUCT-ALIGN-01`，拼写不符。
- F7b 残留：owner “Lead/QA”（`:657`），QA 不是矩阵中的角色。
- AC-27 收敛测试：owner “Developer/ops/AfterSales”，三头责任（`:571`）。
- `:244-245` D-CR-6/D-CR-7 “remain undecided”，无 owner，且 GOV-COST-01 撤回时未关闭。
- `:966-968` Developer prompt 指针“not done”，但 `prompt/Developer.md:4` 已存在该指针（QC-34-DEV-POINTER / REV-DEV-POINTER 已记录）——ROLLOUT A-2 的依据成立。
- 六条 2026-12-15 到期的 AC：决定期限 2026-12-08 尚未到（今天 2026-11-27），但台账没有决策记录。

### 5.5 计划视图与台账不一致（选摘）

- `docs/lead/PAY-AC50-REVIEW-01/tasks.md:3` 仍写 “T1–T8 pending”，而 `plan.md:5` 与台账 `:651` 写 COMPLETE（scoped）。
- `docs/lead/PAY-OPS-REPLAY-01/tasks.md:3` 仍 “DRAFT — pending OPS-P1..P3”，台账 `:556` 写 complete。
- `docs/lead/PAY-SETTLE-01/plan.md:5` 仍 “T1.2 FROZEN pending T1.2-R”，而 `:146-150` 已 PASS；T2.4 在 `tasks.md:86` 已 `[x]`，但台账 `:465-467` 仍 AC-REF、legacy ruling 待人类。
- `review-tracking.md:1210-1213` 的 P0 解冻条件仍要求 `make gates-strict` / `--release --range` / G-4——这些工具已在 `:293-299`、`:993-995` 退役/删除，条件**无法满足**。

---

## 6. 自查 6：`docs/` 按角色的目录

| 目录/文件 | 性质 | 证据/说明 |
|---|---|---|
| `docs/specs/**` | **现行契约**（唯一业务结论） | `CONVENTIONS.md:23`；`Spec.md:3` |
| `docs/adr/**` | **现行契约**（设计结论） | `CONVENTIONS.md:25`；0006 已 retired，仅历史 |
| `docs/review-tracking.md` | **现行唯一运行状态台账** | `:7` 人类批准 |
| `docs/lead/**` | 在途 Plan/Tasks/证据；新模型下作者写 plan/tasks，**该角色目录将成历史** | `CONVENTIONS.md:22`；`handoff.md:43,86-88` |
| `docs/po/**` | `drafts/` 历史草案；`MEMORY.md` 需清理；`backlog.md` 停用 | `Spec.md:35`；C-4 |
| `docs/architect/**` | 仅 `MEMORY.md` + `README.md`（历史角色目录） | 目录列表 |
| `docs/developer/**` | 仅 `MEMORY.md` | 同上 |
| `docs/reviewer/**` | 仅 `MEMORY.md` | 同上 |
| `docs/collab/**` | `CONVENTIONS.md`/`ac-coverage.tsv` 现行但待停用；`qc-reports/` 历史审查证据；`{pair}/threads/` 只读存档 | `CONVENTIONS.md:143-147` |
| `docs/implementation/**` | Developer selfcheck / deviation register，历史证据；ADR/台账引用 | 目录列表 |
| `docs/handoff.md` | 会话快照（2026-11-22），待停用；`README.md:222`、`docs/development.md:7,111` 引用 | 文件头 |
| `docs/roadmap-status.md` | 2026-11-22 路线图快照；`README.md:219`、`docs/development.md:6,106` 引用 | 文件头 |
| `docs/convert/**` | 本次提案（未跟踪），不属于任一既有角色 | `git status` |

结论：按角色划分的 `docs/lead|po|architect|developer|reviewer` 在新模型下全部转为历史；真正需要保留的只有 `docs/specs/**`、`docs/adr/**`、`docs/review-tracking.md`，以及作为描述性输入的 `docs/architecture-baseline.md`、`docs/contracts.md`、`docs/ops.md`、`docs/development.md`。`docs/handoff.md`、`docs/roadmap-status.md` 是状态副本，按第 27 条应停止维护。

补充事实（清理时会遇到）：

- **`docs/collab/qc-reports/` 实际有 90 份报告**，而 `docs/collab/qc-reports/README.md:5` 仍写“44 份 `.md` 报告”。
- **`prompt/` 六个文件**当前都仍与 v1.3.0 模型一致（`Lead.md`、`Developer.md`、`Product-Owner.md`、`Quality-Checker.md`、`Reviewer.md`、`System-Architect.md`）；其中 `Developer.md:1` 仍写 “`Spec.md` 和 `ADR.md`”，是旧命名；`System-Architect.md` 没有被任何文件按文件名引用，只被 `prompt/**` glob 覆盖（`CONVENTIONS.md:26`）。
- **孤儿文件（0 引用）**：`docs/implementation/aftersales-0012-remediation.md`、`PAY-SETTLE-01-R0.4-FIX-selfcheck.md`、`PAY-SETTLE-01-T1.4a-selfcheck.md`、`PLAN-PAY-AC50-REVIEW-01-dev-review.md`、`docs/adr/0001-internal-trust.md`；`docs/lead/PAY-AC50-REVIEW-01/tasks.md` 与 `PAY-OPS-REPLAY-01/tasks.md` 只有目录级/裸文件名引用。
- **悬空引用（目标已不存在）**：`docs/lead/GOV-COST-01/plan.md`（`review-tracking.md:274`）、`docs/specs/0003-platform-governance-cost-reduction.md`（`:246`）、`docs/adr/0008-governance-cost-reduction-amd1.md`（`:290`）、`docs/collab/qc-reports/QC-R04-ADDENDUM.md`（`lead/PAY-SETTLE-01/tasks.md:32`，实际文件名带 `QC-PAY-SETTLE-01-` 前缀）、`docs/collab/developer-reviewer/threads/T-003.md`（只存在 `po-architect/threads/T-003.md`）。
- **大小写**：`docs/Lead/` 在 `docs/cost-reduction-proposal.md:9`、`ROLLOUT.md:71,97,110` 出现；实际目录是 `docs/lead/`。macOS 忽略大小写，Linux/CI 会断链。
- **契约与快照混放**：`docs/adr/` 里混有 `0007-amd1-ac50-plan-review.md`、`...-ac50-impact-analysis.md`（自称“不是 ADR”）、`...-c04-c05-handoff.md`（自称 handoff）；`docs/implementation/` 里有 `PLAN-...-dev-review.md`（plan review）。这些不是契约，迁移/归档时不能按“ADR 不可改”处理。

---

## 7. 自查 7：现行规则中第 2 节没列到、仍需要的条款

逐条对照 `AGENTS.md`、`CONVENTIONS.md`、宪法与两份替换稿。以下 12 项在替换稿中没有对应或明显弱化：

| # | 现行条款 | 替换稿现状 | 影响 |
|---|---|---|---|
| 1 | **分析必须给 `文件:行号` 与实际值**（`AGENTS.md:72`） | 只有附录 A 的“可达性检查”要求行号；一般分析/审查无此要求 | 审查报告可退回“大概/应该”表述；本次 ROLLOUT 自身的 8 处引用错误就是例证 |
| 2 | **ADR 架构缺陷的反证义务**（`AGENTS.md:54`） | 第 19 条只覆盖“对业务裁决有异议”；第 17 条只给路由 | 作者可以只说“ADR 有问题”而不给崩溃/资损时序 |
| 3 | **P0/跨服务/资金/库存/权限/状态机的 Plan 门禁**（宪法 `:84`；`AGENTS.md:78`） | 实现变更“只做第 5-8 步”（`AGENTS.draft.md:32`），**不要求 plan** | 实现变更可在无 plan、无人类 Plan 批准下直接写代码 |
| 4 | **测试替身 / 依赖注入 / 启动 fail-fast**（宪法 `:21`） | 第 15 条只讲 Fail-Closed | nil 依赖、测试替身弱化生产守卫、可选能力用 nil 关闭等风险失去约束 |
| 5 | **事务核心用 `database/sql` + 条件 UPDATE（CAS）**（宪法 `:41`） | 第 20 条只要求幂等 CAS；第 26 条只保留 GORM 限制 | 事务核心可用 GORM/非 CAS 写，违反现行铁律 |
| 6 | **启动期配置/依赖缺失 fail-fast、非零退出、请求路径不得 panic**（宪法 `:44`） | 无 | 启动失败语义丢失 |
| 7 | **历史 Spec 的 EARS 迁移与“行首 Gherkin 不算 EARS”**（宪法 `:71`） | 附录 A 只要求 EARS，未提历史迁移/句式规避 | Spec Kit 模板的 `Given/When/Then` 会天然违规；AC-27 的历史迁移任务失去落点 |
| 8 | **外部复审/审计只能作为问题提案**（`CONVENTIONS.md:70`） | 完全缺失 | 外部 agent/顾问结论可直接成为契约/AC 依据；本次 ROLLOUT 本身就是“内部静态审查 Agent”的产物 |
| 9 | **延期必须有 owner/expiry/reason/thread/status，到期阻断，不得批量续期**（`CONVENTIONS.md:201-203`；宪法 `:49`） | 附录 A 验收状态表无 owner/expiry/reason；无到期规则 | 48 条 AC 行中的 32 条 DEFERRED（含 6 条 12-15 到期）没有合法落点 |
| 10 | **Reviewer 裁决必须带 `review_scope`/`release_status`/INV/baseline_impact**（`CONVENTIONS.md:149-166`） | AGENTS 替换稿 §3 只要求“写明范围” | 跨服务/资金变更的非回归与 INV 证据失去格式 |
| 11 | **“同一路径 2 处以上同类分支即停”**（`AGENTS.md:11`） | 第 4 条只禁“加条件分支或本地锁绕过协议” | 丢失可判定的量化触发线 |
| 12 | **进度不得以 round/commit/测试数量衡量**（`AGENTS.md:29`；宪法 `:23,97`） | 无 | 以提交数冒充进度的行为失去禁止依据 |

另有 3 项**被有意删除**，但需要人类确认而非默认消失：
- `CONVENTIONS.md` 的**变更输入包 / integration owner / baseline_impact**（`:100,106-108,132`）——替换稿说“不再需要”（`ROLLOUT.md:56`），但宪法第 12 条仍要求跨 Spec 非回归证据；没有 owner 时谁组织？
- 宪法 `constitution.md:27` 的**兼容期细节**（双发 `order.paid` 的收口/移除/回滚）——替换稿第 23 条只保留“事实分离”，删掉了兼容期约束。
- 宪法 `constitution.md:31` 的**退款 permit/commitment 恢复规则**——替换稿第 23 条只保留“同一组准入协议 + permit 裁决竞态”，删掉了“不得自行恢复 REVOKED permit / commitment 释放须版本化 SPEC+人类批准 / Σ 等式”。

---

## 8. 自查 8：MEMORY 中的旧角色/旧分级/旧文件

4 个 `MEMORY.md`，共 264 行；其中 **89 行**命中旧角色/旧分级/旧文件/旧闸门/实例状态，**31 行**属于“把运行状态抄进 MEMORY”（现行 `CONVENTIONS.md:36` 已明确禁止，`CONVENTIONS.md:32` 要求只留稳定经验与 canonical 链接）。

| 文件 | 命中行（示例） | 主要问题 |
|---|---|---|
| `docs/architect/MEMORY.md` | `:6`（`ac-coverage.tsv` + 闸门/pair 退役日期）、`:18`（addendum 已超 5 个）、`:39-40`（ADR-0006 退役、0007 合并待办）、`:45-47`（pair/G-1…G-4）、`:51,65`（Reviewer/PO 旧角色） | 日期公告 + 实例计数 + 闸门 |
| `docs/developer/MEMORY.md` | `:4`（tsv/闸门/pair）、`:13`（E1…E25/B-01/B-05 实例）、`:29,57`（CONVENTIONS）、`:33-34`（scope 词表/记忆路径）、`:47`（`make gates*`）、`:61`（tsv）、`:66`（CONVENTIONS） | 旧分级/闸门/文件 |
| `docs/po/MEMORY.md` | `:3`（tsv/闸门/pair）、`:16`（`Role: PO` trailer）、`:20`（tsv）、`:29,36`（v1.1 已修正）、`:42`（CONVENTIONS）、`:46`（backlog） | 旧角色/trailer/文件 |
| `docs/reviewer/MEMORY.md` | `:6,13,30,36,53,54,65,72-74,84`（tsv/闸门/CONVENTIONS/trailer）、`:31`（硬编码 `seq 1 38`）、`:51-52`（38/38、具体 AC 号）、`:61`（TODO 跟进状态） | 大量运行状态 |

另有一类**独立违规**（不受新模型影响，今天就违反 `CONVENTIONS.md:32,36`）：四个 MEMORY 都抄了业务规则/决策索引（architect `:24-33`、developer `:18-25`、reviewer `:19-25,55-61`、po `:27-38`），而各自头部写“不放业务规则”。这属于 C-4 清理的一部分，但**不应等新模型**。

---

## 9. 裁决题证据核对（§3）

| # | ROLLOUT 证据 | 核对结果 | 备注 |
|---|---|---|---|
| 1 | 审查者 A/B/C | 事实成立：`AGENTS.draft.md:10` 支持 subagent 或静态会话；B 不能复跑测试，宪法 `:12` 要求作者给日志 | 无预设推荐 |
| 2 | 合入委托 A/B | 事实成立：`AGENTS.draft.md:29` 第 7 步“作者合入”；现行宪法 Governance 4 要求人类 merge | 这是本次最大的权力变更 |
| 3 | PAY-SETTLE-01 过渡 A/B | 事实成立：SPEC-0002 `:9,24` 与 ADR addendum `:7-8` 都写 parent 冻结；`docs/adr/` 确有 13 个 `0007-amd1-addendum-*` | 13 个 addendum 待合并 |
| 4 | 删除 vs 归档 | 事实成立：引用面 507 + 90；历史存档禁止改写 | 删除会造成悬空引用 |
| 5 | AC-72 | **成立**：`ac-coverage.tsv:47` 状态与 reason 相反；`:48-49` 非 AC 编号、owner 为角色簇 | 建议 A（拆分），但 tsv 将停用，需迁入 Spec 验收表 |
| 6 | AC-27 / C+A | **成立但不完整**：`:536,549,586,591` 只有标签；`:617-618` 选项无编号；`tasks.md:57` 写“未关闭前不得 AC-VERIFIED”；`ac-coverage.tsv:41` thread 指向 `PAY-SETTLE-01`，而 C+A 范围是 `PAY-OPS-REPLAY-01` | 需要人类认定 C+A 是否覆盖 NOWB/LIVE 两个子项 |
| 7 | 11-27 重排 | **成立**：`:502-509` 写 T2→T3→T4→T5→P0；`:620-625` 写先 Phase 0-R 再 T5，T2/T3/T4 在 P0 之后；`tasks.md:26` 写旧顺序；两者互不引用 | ROLLOUT 把范围写成 `:592-509`，应为 `:502-509` |
| 8 | 台账整理 | **大体成立**：`:14-167` 交错、`:320-457` 升序、`:458-1003` 降序、`:1044-1165` 旧路线图；但实际 **1254 行**不是 1268；`:1044-1165` 内有 2026-11-21 带日期小节；第二处 11-26 条目从 `:709` 开始而非 `:789` | 见 §10 |
| 9 | 冻结解除范围 | **成立**：5 处冻结（`:175,198,741,789,982`），恢复条目（`:620-625`）未引用解除了哪一条；无显式解冻 | 需要人类给出范围与保留约束 |
| 10 | 试点变更 | 无证据可核 | 建议另选小型变更 |
| 11 | 紧急路径 | 事实成立：替换稿无紧急路径；替换稿第 2 条要求批准后才实现（`constitution.draft.md:11`）；`constitution.draft.md:57` 禁 hotfix 绕过 FAIL | 建议 A（不设），用最短 Spec + 当日人类批准 |

---

## 10. ROLLOUT 自身与 HEAD 不符（8 处）

| # | ROLLOUT 位置 | ROLLOUT 写法 | HEAD 事实 |
|---|---|---|---|
| 1 | `ROLLOUT.md:8` | “2026-11-27 本地快照，**无 git**” | 仓库是 git 仓库，HEAD `9e81497`（2026-11-27 07:37） |
| 2 | `ROLLOUT.md:73` | 台账“共 **1268** 行” | **1254 行** |
| 3 | `ROLLOUT.md:99` | “`:592-509` 与 `:629-625` 互不引用” | 范围写反：应为 `:502-509` 与 `:620-625`；`509`/`592`/`629`/`625` 都不是区间端点 |
| 4 | `ROLLOUT.md:99` | `:1044-1165` 是“无日期的旧路线图段” | 该范围含 2026-11-21 带日期小节（`:1067,1078,1085`） |
| 5 | `ROLLOUT.md:100` | `docs/Lead/PAY-SETTLE-01/...` | git 跟踪的是 `docs/lead/...`（macOS 大小写不敏感，Linux 会失败） |
| 6 | `ROLLOUT.md:101` | `SPD-STRUCT-ALIGN-01` | 实际是 `SDD-STRUCT-ALIGN-01`（`:354,440`）；`SPD-` 只出现在 ROLLOUT 自身 |
| 7 | `ROLLOUT.md:100` | SPEC-0002 `:1038` 是“变更记录” | `:1038` 在 §14 批准记录（`:1016` 起），§13 变更记录从 `:995` 起 |
| 8 | `ROLLOUT.md:100` | B-6 依赖写 `8-4` | 裁决 8 只有 A/B/C 三个选项，没有“4”；应为笔误或需人类澄清 |

另：`ROLLOUT.md:95` B-1 说 AC-60/63 在台账是 AC-REF、在 tsv 是 DEFERRED，成立；但 `:465/:473` 只涉及 AC-63，AC-60 只在 `:495`。

---

## 11. 阶段状态与建议

| 阶段 | 状态 | 原因 |
|---|---|---|
| A-1 自查 | **完成**（本文件） | 8 项均有证据 |
| A-2 台账登记 | **暂缓** | 要写“本次简化取代了哪些旧项”；简化未获批，且 A-2 要求“批准记录只登记人类确认过的内容”，当前没有可登记的批准 |
| B-1…B-7 | **未做** | B-2/3/4/5/6 依赖裁决 5/6/9/7/8-4；B-1 虽无依赖，但 AC 状态写入属 Reviewer，Lead 不得代写；B-7 的清点已由本文件 §5 覆盖，决定仍归人类 |
| C-1…C-4 | **未做** | 依赖裁决 1–4（C-1 另依赖 11；C-2 依赖自查 2） |
| D-1…D-2 | **未做** | 依赖裁决 6；D-1 必须是最后一个动作 |

**我的建议（供裁决参考，不是决定）：**

1. **审查者**：选 **A**（外部 subagent，有运行时）。本仓重灾区证据需要真实 MySQL/NATS/Redis 与 `-race`；B 无法复跑，只能依赖作者日志，独立性最弱。C 可作为过渡，但会增加第三种审查形态。
2. **合入**：建议 **B（保留人类合入）**，至少对契约变更与资金/库存/权限/跨服务变更保留；实现变更可先试点 A。理由：当前台账已显示多处“计划说完成、台账说未完成”“冻结未解冻却继续做”的漂移，人类合入是最后一道低成本控制。若选 A，建议把“人类合入”至少保留给 `api/proto/**`、`migrations/**`、`internal/platform/**`、`deploy/**`。
3. **PAY-SETTLE-01**：建议 **A + 明确边界**：先把 B-1/B-3/B-5/B-6 做完，把仍开放的 AC（AC-27/60/61/62/63/65/67 等）按“已验证/有引用/延期”迁入 SPEC-0002 的验收状态表；13 个 addendum 先不合并，等新 ADR 起草并经审查/批准；在迁移完成前冻结不解除、不合并。若不愿承担迁移成本，选 B（旧流程走完），但 C-4 必须推迟到旧变更关闭之后。
4. **停用文件**：建议 **B（移到 `docs/archive/`）+ 旧路径一行 stub**，避免 500+ 历史引用全部悬空；历史 QC/REV/线程不改写。若选 A（删除），需明确“历史文档悬空引用可接受”。
5. **AC-72**：建议 **A（拆分）**，但落点是 SPEC-0002 验收状态表，不是已停用的 tsv。
6. **AC-27 / C+A**：建议 **B（回到 AC-REF）**，除非人类明确认定 C+A 手工流程同时覆盖 `AC-27-NOWB-01`/`AC-27-LIVE-01` 两个仍 open 的子项；若选 A，必须改 `tasks.md:57` 的禁止句并写明覆盖范围。
7. **重排**：建议 **A（以后来的 11-27 人类重排为准）**，在台账补一条“取代 `:502-509`”的记录；同时修 `tasks.md:26`。
8. **台账整理**：建议 **B（先只归档，不重排）**；D-1 重排留到所有行号依赖项完成后作为最后一个动作。一次重排+归档（A）在还有 B/C 未落地时会让所有引用再次漂移。
9. **冻结解除**：需要人类明确：是“追认已完成工作 + 放开 Phase 0-R/T2.4 剩余范围”，还是“维持冻结，把 T2.4 视为越界”。建议前者，但保留 no merge/no release，并写一条台账。
10. **试点**：建议 **A**，但选一个**小而非资金**的契约变更（例如 SDD-STRUCT-ALIGN-01 的路径/模板对齐），不要拿 PAY-SETTLE-01 做试点。
11. **紧急路径**：建议 **A（不设）**，用“最短 Spec + 当日人类批准 + 事后审查”处理；B 的 hotfix 绕过与替换稿第 30 条冲突。

---

## 12. 复现命令（只读）

```bash
git rev-parse --short HEAD                 # 9e81497
wc -l docs/review-tracking.md              # 1254
wc -m AGENTS.md .specify/memory/constitution.md docs/collab/CONVENTIONS.md
git ls-files .specify | wc -l              # 21
git ls-files .dsh | wc -l                  # 9
specify --version                          # 1.4.3
specify refresh --help                     # No such command
git config core.hooksPath                  # (unset)
```
