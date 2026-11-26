# REV-GOV-SDD-HYGIENE-01 — Reviewer 有范围裁决（治理文本）

> **一句话结论**：`PASS`（**有范围**）——只对 `472e64f` → 当前工作树（`AGENTS.md` blob `e7759cd`、`CONVENTIONS.md` blob `d465d50`）的 **5 个治理文本 hunk** 有效。不构成 Spec 全量验收、merge 或 Release。
> 本报告只读审查，只新建本文件；未修改 `AGENTS.md`、`docs/collab/CONVENTIONS.md`、QC 报告或 `docs/review-tracking.md`，未 commit。

| 字段 | 值 |
|---|---|
| change_id | `GOV-SDD-HYGIENE-01` |
| Tier | `L2`（治理文本轻路径；命中 `AGENTS.md:84` 与宪法 `:77`；治理/工具链变更恒为 L2，`docs/collab/CONVENTIONS.md:94`）。**Reviewer 确认 Lead 提议的 Tier 成立。** |
| base | `472e64fec33cbbf91592e2ffc74cbb31abc541db`（= 当前 `HEAD`）；base blobs：`AGENTS.md` `a1b962eec421`、`CONVENTIONS.md` `42fba2173d65` |
| head | **当前工作树（未提交）**：`AGENTS.md` blob `e7759cda8999565206afa36a73e88d722f195d10`、`docs/collab/CONVENTIONS.md` blob `d465d501254eda90320e4cf3790c98f9de723f2d`；`docs/review-tracking.md` blob `d922ce1d56c376c42d7de957fb88fcc14ea90de8`（Lead 台账，不是治理文本） |
| review_scope | 仅审上述 base/head 间 **2 个治理文件、5 个 hunk**：`AGENTS.md:15`、`:77`；`CONVENTIONS.md` §4 删除原协议摘要（`:143-147`）、G-1（`:217`）、§7.2 删除执行器限制历史（`:226-228`）。并核对 `docs/review-tracking.md:350-377` 的 change input/收益/停止条件/delta audit 与 `QC-GOV-SDD-HYGIENE-01.md` 的 READY 证据。**不审**代码、契约、CI、`.specify/**`、`docs/specs/**`、PAY-SETTLE-01/ADR/SPEC 正文；**不审**已删除的 §2.4（现行两文件 0 命中）。纯文档变更，未运行测试，不构成 AC 验收或 Release 证据。 |
| Actor（变更作者） | `arch-gov`（`docs/review-tracking.md:374`） |
| Reviewed-by | `reviewer-gov`（本报告作者；与作者不同 actor） |
| QC | `qc-gov`：`READY`，0 BLOCKER / 0 MAJOR；open MINOR = M1/M2/M6/M7/M8（`QC-GOV-SDD-HYGIENE-01.md:425`、`:460`） |
| human decision | 2026-11-26 Director 批准 P1 清理，明确**不声明** Spec Kit 路径边界；结构对齐另议（`SDD-STRUCT-ALIGN-01` 未开始）——`docs/review-tracking.md:352-354` |
| verdict | **`PASS`（有范围）** |
| release_status | **`NOT RELEASE`** |

---

## 0. 绑定、方法与被审对象

### 0.1 精确绑定

```text
HEAD (git rev-parse HEAD)        = 472e64fec33cbbf91592e2ffc74cbb31abc541db
base AGENTS.md blob              = a1b962eec4211bf95b097c83823a89686757710c
base CONVENTIONS.md blob         = 42fba2173d65c546f2de6c0c505d7cfcc5b6bc76
head AGENTS.md blob (worktree)   = e7759cda8999565206afa36a73e88d722f195d10
head CONVENTIONS.md blob         = d465d501254eda90320e4cf3790c98f9de723f2d
git diff --name-status           = M AGENTS.md / M docs/collab/CONVENTIONS.md / M docs/review-tracking.md
git diff --cached --name-status  = （空）
git diff --numstat (治理文本)    = AGENTS.md 2/2；CONVENTIONS.md 2/6
git diff -U0 hunks (治理文本)    = 5
git diff --check                 = exit 0
```

- `docs/review-tracking.md` 的 `+29/0` 是 Lead 按宪法 `:96`/`AGENTS.md:87` 写下的台账（change input、收益、停止条件、delta audit），**不是治理文本**；`docs/collab/qc-reports/QC-GOV-SDD-HYGIENE-01.md` 为 QC 证据文件（untracked）。二者均在 `docs/review-tracking.md:361-363` 的 `allowed_paths` 内。
- 治理文本 diff **无新增/删除文件、无 rename**；`git status --porcelain -uall` 未出现 `scripts/**`、`.github/**`、`Makefile`、`.specify/**`、`docs/specs/**`、`internal/**`、`api/**`、`migrations/**`。

### 0.2 审查方法

- 只读执行：`git rev-parse`、`git hash-object`、`git diff`、`git show`、`grep`、`ls`；未运行测试（docs-only，无测试可跑，也不伪造测试声明）。
- 所有关键结论给出 `文件:行号` 与实际值；未找到的数据不作推断。

---

## 1. 精确范围核对（必核 1）：**通过**

| # | 文件 | 当前行 | 变更内容 | 是否在声明范围 |
|---|---|---|---|---|
| 1 | `AGENTS.md` | `:15` | §0.5 改为“只经 Lead 路由（§1.0）……`review-tracking.md` 只承载结构化裁决与 canonical 指针（pair 线程已退役，仅存档）” | 是 |
| 2 | `AGENTS.md` | `:77` | §2 第 4 条“线程 CLOSED”改为“历史线程 CLOSED（仅存档）” | 是 |
| 3 | `CONVENTIONS.md` | `:143-147`（旧 `:148-149` 删除） | §4 删除“原协议摘要（只作历史参考…）”两行（T-NNN 路径/状态机/YAML 字段/回写要求） | 是 |
| 4 | `CONVENTIONS.md` | `:217` | G-1 删除“Role/Exception 是否存在、线程是否存在”，改为“§3 中按 Tier 适用字段是否齐备（至少 `change_id`/`Tier`/base/head/`allowed_paths`；L2 另见 §3）” | 是 |
| 5 | `CONVENTIONS.md` | `:226-228` | §7.2 删除执行器限制/债务清单，保留“目标规则不得被描述成已通过机器能力”的口径 | 是 |

结论：**恰好 5 个 hunk，逐项与 change input/QC final scope 一致**（`docs/review-tracking.md:355-359`；`QC-GOV-SDD-HYGIENE-01.md:423`）。治理文本范围之外只有台账与证据文件，无越界修改；`git diff --check` 通过（exit 0）。

---

## 2. 无 Spec Kit 边界（必核 2）：**通过**

- `grep -inE "speckit|SPECIFY_FEATURE|§2\.4" AGENTS.md docs/collab/CONVENTIONS.md` = **0 命中**（exit 1）。
- `§2.4` 已被完整删除；现行两文件没有任何 vendor 路径、`specs/` 副本或 `SPECIFY_FEATURE_DIRECTORY` 规则。
- 人类决定被遵守：`docs/review-tracking.md:352-354` 明写“approved the P1 cleanup **without** a Spec Kit path-boundary declaration…`SDD-STRUCT-ALIGN-01`, not started”；`QC-GOV-SDD-HYGIENE-01.md:422` 同口径。
- 未触碰 `.specify/**` 与 `docs/specs/**`（`git status` 无相关路径）；未新增第二套路径规则。

---

## 3. 无新门禁/新状态/新机制（必核 3）：**通过**

- 治理文本 diff 中**没有新增 `MUST` 句子**（对 `^+` 行提取 `MUST` = 0）；没有新增脚本、CI、Makefile 目标或状态字段。
- G-1 仍只是**人工核对清单**：`CONVENTIONS.md:207`（工具链已退役）、`:213`（下表是 Reviewer 人工核对清单；“原机器检查范围”只记录历史）、`:222`（不再新增 G 系列检查，无脚本实现）。新措辞引用的 `change_id`/`Tier`/base/head/`allowed_paths` 均来自既有 §3（`:114-124`、`:130`），不是新 schema。
- 删除项只减少文本：§4 历史摘要与 §7.2 限制清单；§7 头部“没有替代工具/不得宣称机器门禁”的现行口径保留（`:209`、`:228`）。
- `scripts/**`、`.github/**`、`Makefile`、CI 零改动（`git status --porcelain -uall -- scripts .github Makefile .specify docs/specs` 为空）。
- 未新增批准点：§2 阶梯（`CONVENTIONS.md:40-62`）、§2.3 分级（`:78-103`）、§7 人工清单均未被改写；人类批准点与 Release 保留。

---

## 4. 无活引用/悬空（必核 4）：**通过**

现行治理文本（`AGENTS.md` + `CONVENTIONS.md`）命中情况：

| 检索项 | 命中 | 判定 |
|---|---|---|
| `线程只承载` | 0 | 旧“线程作为现行通道”的表述已清除 |
| `线程是否存在` | 0 | G-1 不再要求线程存在性证据 |
| `Exception` | 0 | `T-NNN` 例外口径不再出现在现行治理文本 |
| `原协议摘要` | 0 | §4 被删摘要无残留标题/指针 |
| `T-NNN` | 仅 `CONVENTIONS.md:191` | 明确是“历史 36 行 `pair/T-NNN` 指针，不回改”，属历史台账说明，不是活协议 |

- §4 删除后仍自足：退役说明与“目录不得删除/历史 36 行不回改”保留（`:145-147`）；§4.1 Reviewer 附加元数据保留（`:149-166`）；§5 的 `thread_status=CLOSED` 自带“历史通道/线程已退役”限定（`:174`）；§6.1/§6.3 的 `thread` 列规则自带历史/新行区分（`:191`、`:203`）。
- §7.2 删除后仍自足：只有“退役后仍成立的只有 §7 的口径要求”一段（`:228`），且 §7 头部（`:209`）已完整保留“无替代工具/不恢复脚本/不得要求运行已删命令”。
- `§4/§7.2` 的入站引用清点：`docs/ext-review-1123.md:24/:240`、`docs/ext-review-1123-triage.md:129/:134/:160` 是 2026-11-23 外部复审快照（`CONVENTIONS.md:72-76` 只作问题提案）；`docs/cost-reduction-proposal.md:20/:36/:43/:64-65` 自称已撤回/归档（`:3`）；`docs/adr/0006-collab-gates.md` 状态为 `superseded / retired`（`:3`、`:9-19`），原文自称只作设计历史；pair 索引/归档线程为只读存档（`:145-147`）。**没有任何现行规范条款依赖被删文本**。
- 本报告未发现本次 diff 新增的悬空治理引用；既有归档指针变旧属 MINOR-2（非阻断）。

---

## 5. 宪法逐条核对（必核 5）：**通过**

### 5.1 Article I–V

| 条款 | 结论 | 依据 |
|---|---|---|
| **I. 架构与工程哲学** | 不适用/不违反 | 无代码、注释或实现改动；无 `if-else`/补丁/防御逻辑；Fail-Closed 与契约优先均无对象；不涉及“自我妥协”。 |
| **II. 事实所有权与状态机** | 不适用/不违反 | 无状态字段、事实、幂等、退款/履约准入、读写隔离变更；`baseline_impact: 不涉及 INV`（`docs/review-tracking.md:376-377`）成立，因为无行为/契约/状态语义改动。 |
| **III. 微服务技术栈铁律** | 不适用/不违反 | 无 DTM/Outbox/数据库/GORM/多仓/静态守卫/技术栈改动。 |
| **IV. 测试与交付标准** | 通过（docs-only 声明） | 纯治理文本，无行为变更，故无测试/并发/故障注入要求；未把 AC-REF 说成 AC-VERIFIED，未伪造测试日志（`AGENTS.md:122`、宪法 `:49`）。 |
| **V. AI 子系统与安全约束** | 不适用/不违反 | 无 `ai`/Turn-Action/RAG/Mesh/权限改动。 |

### 5.2 Governance / Workflow 相关条款

| 条款 | 结论 | 依据 |
|---|---|---|
| 宪法 `:77`（治理文本 L2 轻路径） | 满足 | 只改 `AGENTS.md`/`CONVENTIONS.md`，无代码/契约/CI/`scripts/**`；流程为 Architect 起草（`arch-gov`）→ QC（`qc-gov` READY）→ 人类 2026-11-26 批准 → Reviewer 复核（本报告）；未建 Spec/ADR/Plan。 |
| 宪法 `:79`（四种状态不得混用） | 满足 | `AGENTS.md:77` 改为“历史线程 CLOSED（仅存档）”，仍把它与实现 PASS、合入、Release PASS 并列区分；本报告亦不把审查 PASS 当 Release。 |
| 宪法 `:82`（证据绑定精确 HEAD、纯文档不得伪造测试） | 满足 | base/head blob 已绑定；明确 docs-only、无测试声明；`git diff --check` 为客观证据。 |
| 宪法 `:83`（人类裁决记录） | 满足 | 2026-11-26 人类批准与“不声明 Spec Kit 边界”记入 `docs/review-tracking.md:352-354`，范围未扩大。 |
| 宪法 `:86`（Actor 独立性/trailer） | 变更级满足；trailer 属合入包装 | 作者 `arch-gov` ≠ Reviewer `reviewer-gov`（`docs/review-tracking.md:374`）；当前**未提交**，commit trailer 尚不存在（不是已发生的违规，属 MINOR-4 `[TODO]`，合入时补）。 |
| 宪法 `:92`（Lead 边界） | 满足 | Lead 只写台账/变更输入（`docs/review-tracking.md:350-377`），未作者化 Reviewer 裁决；未代写 Spec/ADR。 |
| 宪法 `:94`（Reviewer 义务） | 满足 | 本报告声明 `review_scope`、base/head、`release_status`、`Actor/Reviewed-by`，并逐条核对 Article I–V 与 Governance/Workflow。 |
| 宪法 `:96`（治理基线 delta audit） | 正文满足；窄口径张力见 5.3 | delta audit 已写入 `docs/review-tracking.md:372-373`（主记录地 `review-tracking.md` 完整可用）；`AGENTS.md:15` 的“pair 线程已退役，仅存档”与 `:96` 括注存在窄口径张力。 |
| 宪法 `:102`（宪法修订程序） | 满足 | 本变更未改宪法文本；M1 的未来 PATCH 明确另走 `/speckit-constitution`，不在本轻路径内。 |
| 宪法 `:105`（同源表述） | 满足（漂移减少） | `AGENTS.md:15` 与 `CONVENTIONS.md:145-147` 现在都表述“线程退役、仅存档”；本次改动消除了旧的“线程只承载……”活通道措辞，未新增漂移。 |
| 宪法 `:106`（人类豁免） | 不需要 | 本变更未要求豁免；人类批准本身已记录。 |

### 5.3 `AGENTS.md:15` 与宪法 `:96` 的张力：**MINOR/`[TODO]`，不是 BLOCKER**

**结论：不阻断，判 MINOR，按已知漂移处理；不得据此判 FAIL。**

依据：

1. **宪法 `:96` 的可执行路径未被切断。** `:96` 的原文是“在 `docs/review-tracking.md`（或对应 `docs/collab/<pair>/threads/`）记录 delta audit（日期、版本、差异摘要）”。主记录地是 `review-tracking.md`，threads 只是“或”的并列选项；本变更的 delta audit 已落在 `docs/review-tracking.md:372-373`。没有“无法合规记录”的死结。
2. **线程退役是宪法自己委托的执行细则。** 宪法 `:75` 明写“执行细则（角色矩阵、产物所有权、线程协议）以 `AGENTS.md` 与 `CONVENTIONS.md` 为准”。`CONVENTIONS.md:145-147` 退役线程、保留存档，属于被委托的执行细则，不直接违宪；`:96` 的括注因此成为**可选但已退役的落点**。
3. **本次改动没有新增冲突，反而减少漂移。** 旧 `AGENTS.md:15` 把线程描述为“只承载结构化问题、裁决和 canonical 指针”的现行通道；新文本明确“pair 线程已退役，仅存档”。宪法 v1.3.0（2026-11-25）与线程退役同日但未同步 `:96`/`:75`，该漂移先于本变更存在（`CONVENTIONS.md:145`；QC 已登记为 M1）。
4. **新句的“`review-tracking.md` 只承载结构化裁决与 canonical 指针”是窄读风险，但非禁止性规则。** 该句位于 §0.5 的上下文隔离原则，语义是“review-tracking 不是第二契约/聊天通道”，不是对台账内容的白名单。即使按最窄读法（delta audit 不是“裁决”也不是“指针”），宪法 `:96` 优先级更高，且本次台账自身就在 `review-tracking.md:360-373` 记录了 change input、收益、停止条件与 delta audit——实际做法已证明该路径畅通。若追求文本精确，可在未来宪法 PATCH 中一并澄清“review-tracking 承载结构化裁决、canonical 指针与治理 delta audit”；这是文档精确化，不是当前合规阻断。
5. **没有条款被违反。** 不存在“必须写 threads”或“禁止写 review-tracking”的强制要求被本 diff 破坏；因此不满足 BLOCKER/MAJOR 的判定条件。

**残留处置（`[TODO]`）**：下一次宪法修订（`/speckit-constitution`，宪法 `:102`）PATCH `:96`/`:75` 的括注，或在下一份 delta audit 显式写“宪法 `:96`/`:75` 残留已接受”；不得用改 `CONVENTIONS.md` 的方式覆盖宪法。

---

## 6. 台账/QC 证据核对（必核 6）：**通过（有少量包装性缺口）**

`docs/review-tracking.md:350-377` 的 `GOV-SDD-HYGIENE-01` 条目：

| 要求 | 是否存在 | 证据 |
|---|---|---|
| change input：base | 是 | `:360`（`base 472e64f`） |
| change input：allowed_paths | 是 | `:361-363`（含两治理文件、QC/REV 报告、`review-tracking.md`） |
| change input：forbidden | 是 | `:364-365`（code/contracts/migrations/`scripts/**`/CI/`.specify/**`/`docs/specs/**`） |
| change input：tests | 是 | `:365`（docs-only, none claimed） |
| 可验证收益（benefit） | 是 | `:366-367`（grep-checkable，三项） |
| 停止条件（stop condition） | 是 | `:368-371`（只允许两治理文件；需宪法/ADR/脚本/CI 则停止拆分；否则回退） |
| delta audit | 是（正文） | `:372-373`（两文件变更、全员重读、`prompt/**` 未变、MEMORY 无运行态变更） |
| human decision | 是 | `:352-354`（2026-11-26 Director 批准；不声明 Spec Kit 边界） |
| Tier | 是 | `:376`（L2，治理文本轻路径） |
| baseline_impact / INV | 是 | `:376-377`（不涉及 INV，理由为无行为/契约/状态变更） |
| Actor / QC | 是 | `:374-375`（`arch-gov`、`qc-gov` READY） |

QC 报告已记录：人类决定不声明 Spec Kit 边界（`:422`）、§2.4 删除（`:423`、`:425`）、D-1 为 N/A、D-2 为 CLOSED（`:438-440`）、最终 READY 0 BLOCKER/0 MAJOR（`:425`、`:460`）；open MINOR = M1/M2/M6/M7/M8（`:425`）。与本次实际 diff 一致。

**包装性缺口（MINOR-4，`[TODO]`，不阻断 PASS）**：

- 当前未 commit，故无法核对 `change_id`/`Tier`/`Actor`/`Reviewed-by` 的 commit trailer；`head` 只能以 blob 绑定（本报告已记录）。
- `Tier-Rationale` 只写“governance text light path”，未显式引用命中判据（`AGENTS.md:84`、宪法 `:77`、`CONVENTIONS.md:94`）；建议在 change input/commit 中补齐。
- delta audit 未显式写“版本”（`AGENTS.md`/`CONVENTIONS.md` 的版本口径；日期在标题 `:350`，差异摘要见 `:355-373`）；宪法 `:96` 括注含“版本”项，建议补一行。
- 人类**合入**批准人与日期尚不存在（PASS 前是正常的，合入前必须补，宪法 `:95`、`CONVENTIONS.md:98`）。
- 台账 `:355` 的“`AGENTS.md` §0.5/:15 and §2.4/:77”中，`AGENTS.md` 没有 §2.4 小节，`:77` 是 §2 第 4 条；与本次被删除的 `CONVENTIONS.md` §2.4 易混，建议改为“§2 第 4 条 / `:77`”或仅写行号。

---

## 7. Actor 独立性与 Tier 确认（必核 7）：**通过**

- 作者：`arch-gov`（Architect 起草；`docs/review-tracking.md:374`）。
- Reviewer：`reviewer-gov`（本报告作者）。两者不同 actor，满足宪法 `:86`/`:94` 与 `AGENTS.md:63`。
- QC：`qc-gov`，与作者、Reviewer 均不同；QC `READY` 不替代本 verdict（`QC-GOV-SDD-HYGIENE-01.md:326`）。
- **Tier：`L2` 确认成立。** 治理文本变更（`AGENTS.md`/`CONVENTIONS.md`）恒为 L2（`CONVENTIONS.md:94`）；本次未改代码/契约/CI/`scripts/**`，符合 `AGENTS.md:84` 与宪法 `:77` 的轻路径范围，因此可走轻路径、不建 Spec/ADR/Plan。
- 本报告头部已声明 `review_scope`、base/head、`Tier: L2`、`release_status: NOT RELEASE`；提交时须补 `change_id`/`Actor`/`Reviewed-by`/`Tier` trailer（`AGENTS.md:79`、`CONVENTIONS.md:243`）。

---

## 8. M6：`ac-coverage.tsv` 的 `T-003` 悬空指针（必核 8）：**存在，属 Reviewer `[TODO]`，不是本变更 FAIL 理由**

- `docs/collab/ac-coverage.tsv:38-43` 共 6 行（AC-14/21/25/27/31/32）`thread` 列均为 `developer-reviewer/T-003`。
- 该线程文件不存在：`ls docs/collab/developer-reviewer/threads/` 仅 `T-001.md`、`T-002.md`；`git log --all -- docs/collab/developer-reviewer/threads/T-003.md` 无结果——不是“被删除”，而是台账指向从未存在的线程。
- 这 6 行不在本变更的 5 个 hunk/`allowed_paths` 内（`docs/review-tracking.md:361-363`），且 `ac-coverage.tsv` 属 Reviewer 台账；按 `AGENTS.md:61` 的“拒绝微观完美主义”，列为 `[TODO]`，**不作为 GOV-SDD-HYGIENE-01 的 FAIL 理由**。
- 历史 36 行（`:2-37`）指向 `T-001`/`T-002` 的指针不受影响；修复时不得回改这 36 行（`CONVENTIONS.md:191`、`:203`）。

---

## 9. 问题分级

| 级别 | 数量 | 明细 |
|---|---|---|
| **BLOCKER** | **0** | — |
| **MAJOR** | **0** | — |
| **MINOR** | **4** | MINOR-1（宪法 `:96`/`:75` 窄口径张力，含“review-tracking 只承载”窄读）、MINOR-2（归档入站指针变旧）、MINOR-3（`ac-coverage.tsv:38-43` 的 `T-003` 悬空）、MINOR-4（提交包装/台账精确性缺口） |

| ID | 级别 | 位置 | 说明 | Owner | 阻断? |
|---|---|---|---|---|---|
| MINOR-1 | MINOR | `AGENTS.md:15` vs 宪法 `:96`/`:75` | 已知、先于本变更存在的窄口径张力；本 diff 减少漂移而非新增冲突；“review-tracking 只承载…”若按闭集读法可由未来宪法 PATCH 澄清 | Architect（未来 `/speckit-constitution`） | 否 (`[TODO]`) |
| MINOR-2 | MINOR | `CONVENTIONS.md` §4/§7.2 删除处 | 归档文档（cost proposal / ext-review / ADR-0006 / pair 索引）中的旧指针变旧；无现行规则依赖 | 无需本次修 | 否 (`[TODO]`，可选) |
| MINOR-3 | MINOR | `docs/collab/ac-coverage.tsv:38-43` | 6 行指向不存在的 `developer-reviewer/T-003`；与本次治理文本无关 | Reviewer（本 actor） | 否 (`[TODO]`) |
| MINOR-4 | MINOR | `docs/review-tracking.md:355-377` / commit 包装 | 未提交导致 trailer 不可核；`Tier-Rationale` 未引判据；delta audit 未显式写版本；人类合入批准待补；`:355` 的 `§2.4/:77` 记号易与被删的 CONVENTIONS §2.4 混淆 | Lead | 否 (`[TODO]`) |

---

## 10. Verdict：**PASS（有范围）**

**PASS 范围**：base `472e64f` → 当前工作树（`AGENTS.md` `e7759cd`、`CONVENTIONS.md` `d465d50`）的 5 个治理文本 hunk。在该范围内：

- 精确范围与 change input 一致，无任何越界修改；
- 无 Spec Kit 边界文本（grep 0 命中），人类“不声明路径边界”的决定被遵守；
- 无新机器门禁/新状态字段/新脚本/新批准点，G-1 仍为人工清单；
- 无活引用/悬空规范性依赖，§4/§7.2 删除后 §4.1、§5、§6.1/§6.3、§7 头部自足；
- 宪法 Article I–V 及 Governance/Workflow 相关条款逐条通过；`AGENTS.md:15` 与宪法 `:96` 的张力按 MINOR/`[TODO]` 处理，不构成 BLOCKER；
- 台账/QC 证据与 Actor 独立性满足；Tier L2 确认成立。

**本 verdict 的有效性条件**：只对上述 blob 有效；若提交产生新 head（或工作树内容在提交前变化），必须重新绑定并复核 diff；若提交内容与工作树不一致，本 PASS 不继承。

---

## 11. 明确声明

- **本 verdict 不等于 Spec 全量验收、merge 或 Release。** `release_status = NOT RELEASE`；人类合入/Release 检查点仍保留（宪法 `:77`、`:95`；`CONVENTIONS.md:98`）。
- **no AC-VERIFIED**：本变更没有受影响 AC，未运行测试、未产生任何 AC-VERIFIED/AC-REF 结论；纯文档变更不伪造测试声明（`AGENTS.md:122`、宪法 `:49`）。
- **docs-only 无测试声明**：审查证据限于 `git diff`/`grep`/blob 哈希/`git diff --check`。
- **未修改任何治理文本/QC 报告/台账，未 commit**：本报告是唯一写入；`AGENTS.md`、`CONVENTIONS.md`、`QC-GOV-SDD-HYGIENE-01.md`、`docs/review-tracking.md` 保持本审查前状态。

---

## 12. 后续建议（PASS，最多 3 条）

1. **合入前补齐提交包装（Lead）**：commit/PR 写 `change_id: GOV-SDD-HYGIENE-01`、`Tier: L2`、`Actor: arch-gov`、`Reviewed-by: reviewer-gov`，绑定精确 head，并补 `Tier-Rationale` 的命中判据（`AGENTS.md:84`/宪法 `:77`）、delta audit 的版本项、人类合入批准人与日期；同时把台账 `:355` 的 `§2.4/:77` 记号改为“§2 第 4 条 / `:77`”，避免与被删除的 CONVENTIONS §2.4 混淆。
2. **M1 的宪法收口（Architect，独立变更）**：未来经 `/speckit-constitution` 对 `:96`/`:75` 做 PATCH，或在下一份 delta audit 明写“宪法 `:96`/`:75` 残留已接受”；不在本次轻路径内，也不得用改 `CONVENTIONS.md` 覆盖宪法。
3. **M6 的台账修复（Reviewer，本 actor，另行处置）**：把 `docs/collab/ac-coverage.tsv:38-43` 的 `developer-reviewer/T-003` 改为 `review-tracking/<change_id>`（保留 `test_type`/`evidence_path`/`status` 与证据列），不回改 `:2-37` 的 36 行历史指针；该修复不属于 GOV-SDD-HYGIENE-01 的本轮范围。
