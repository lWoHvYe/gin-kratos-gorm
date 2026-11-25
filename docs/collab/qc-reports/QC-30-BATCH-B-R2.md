# QC-30 — Batch B 复评（Director K-1…K-6 后）

| 项 | 值 |
|---|---|
| review_scope | Batch B 第二轮：核对 QC-29 的 1 BLOCKER / 4 MAJOR / 5 MINOR 关闭情况，以及 K-1…K-6 裁决落地（宪法 v1.3.0、ADR-0007 Rev 2.1、pair 归档、review-tracking、CONVENTIONS §4/§6）。只读；不替代人类批准或 Reviewer verdict |
| 被审对象 | 工作区未提交变更 **28 个已跟踪文件 + QC-29 报告**（全部 `.md`；`git status --porcelain` = 28 ` M` + 1 `??`）；无新增/删除文件，无脚本/CI/迁移/Spec |
| 版本绑定 | 仍**无 commit/head**：HEAD `104b9276fa2eafd5ac54ad6d9a55a2d701a0dddd`（同 QC-29）；sha1 = 工作区 blob（`git hash-object`） |
| 对照基线 | 宪法 **v1.3.0**（`f9c4f1fdbfe6`，含轻路径条款）；ADR-0007 **Rev 2.1**（`e7e72023a21c`，1596 行）；`review-tracking` K-1…K-6（`:330-339`）；`CONVENTIONS`（`0f2745f0db21`）；plan 未改 |
| 审查类型 | 治理文本轻路径复评（静态、红队）。本轮未运行测试（纯文档），不构成 AC 验收 |
| 结论 | **`BLOCKED`** —— **0 BLOCKER / 2 MAJOR / 6 MINOR**（QC-29 的 5 项 BLOCKER/MAJOR 中 3 项关闭、1 项部分关闭、1 项关闭；另有 1 项**新 MAJOR** 属轻路径允许清单与 B-1 改动路径不一致） |

## 0. 关键文件 sha1（工作区 blob，前 12 位）

| 文件 | sha1 | 文件 | sha1 |
|---|---|---|---|
| `.specify/memory/constitution.md` | `f9c4f1fdbfe6` | `docs/adr/0007-payment-settlement-admission.md` | `e7e72023a21c` |
| `AGENTS.md` | `a1b962eec421`（未变） | `docs/adr/0007-...-amd1.md` | `8f110089d483`（未变） |
| `docs/collab/CONVENTIONS.md` | `0f2745f0db21`（未变） | `docs/adr/0007-amd1-refundbatch-c04-draft.md` | `aaf33b48d226` |
| `docs/review-tracking.md` | `817935cd21fc` | `docs/adr/0007-amd1-refundbatch-c04-rulings.md` | `ef01579e9e63` |
| `docs/collab/po-architect/README.md` | `45e334b02d9c` | `docs/collab/po-architect/threads.md` | `f56a3410c608` |
| `docs/collab/architect-developer/README.md` | `9d82bc2d3f7b` | `docs/collab/architect-developer/threads.md` | `7c6690bebba1` |
| `docs/collab/developer-reviewer/README.md` | `fd43eca6204e` | `docs/collab/developer-reviewer/threads.md` | `6559c8dc0cb8` |
| `docs/collab/developer-reviewer/threads/T-002.md` | `c6169a1e27d6` | `docs/{architect,developer,po,reviewer}/MEMORY.md` | 同 QC-29（未变） |
| `prompt/Lead.md` / `prompt/Quality-Checker.md` | 同 QC-29（未变） | `docs/collab/qc-reports/QC-29-BATCH-B.md` | `24287377d22e` |

## 1. QC-29 遗留项关闭情况

| QC-29 项 | 判定 | 证据 | 残余 |
|---|---|---|---|
| **BLK-1** 轻路径与宪法冲突 | **CLOSED** | 宪法 `:77`（Workflow 1）新增轻路径并明写“L2 MUST 走本条完整阶梯；**治理文本类 L2** … MAY 走轻路径 … 不建 Spec/ADR/Plan；**ADR 修订不属于轻路径**”；`:109` `Version: 1.3.0`；`:9` 版本历史；`AGENTS.md:84` 与宪法措辞同源（“仅改本文件、CONVENTIONS、MEMORY、`prompt/**`、台账格式；不改代码、契约、CI、`scripts/**`”）；`review-tracking.md:334-335` K-3 记录。两处排除项（代码/契约/CI/scripts）与“ADR 修订不属于轻路径”互补，无冲突 | 允许清单缺 `docs/collab/**`（见 **MAJ-2**） |
| **MAJ-1** B-2 契约文本并入轻路径 | **CLOSED** | `review-tracking.md:324-329` 已拆分：B-1/B-3/B-4/B-5 走轻路径，**“B-2 的 ADR-0007 合并是独立 ADR 修订”**；“no Spec/ADR/Plan”只对治理文本项成立；`ADR:1500-1501`/`:1533` 与 §17.1 批准记录 记 Rev 2.0（K-4）+ Rev 2.1（K-1/K-2/K-4/K-6）批准 | — |
| **MAJ-2** 线程退役未收口 | **PARTIAL** | 正向：3 个 pair `README.md` + 3 个 `threads.md` 均加【历史存档·已退役】横幅（如 `po-architect/threads.md:3`、`architect-developer/README.md:3`）；`threads.md` 协议引用改为 §4；两处维护规则改为“不再新增线程”（`po-architect/threads.md:20-23`、`architect-developer/threads.md:18-21`）；T-002 `status: CLOSED`（`T-002.md:7`）；7 个历史线程文件全在（`po-architect` 3、`architect-developer` 2、`developer-reviewer` 2）；`ac-coverage.tsv` 36 行与 `thread` 列未动 | **仍 3 处命令式文本**，见下 |
| **MAJ-3** 退役闸门/已删命令 | **CLOSED** | `grep -n "G-4\|--release\|--range"` ADR-0007 = **0 命中**；替换措辞就位：`ADR:115`（A-10）、`:854`（§6.4 步骤 3）、`:1061`（§12.3）、`:1067` 统一改为“`SPEC-0002 §7 矩阵 + 五方终态 + Reviewer 人工核对 + review-tracking 记录（协作闸门已退役，无命令可跑）`”；`review-tracking.md:338-339` K-6 记录 | — |
| **MAJ-4** 新延期 `thread` 格式 | **CLOSED** | `review-tracking.md:337` K-5 明确“format = `review-tracking/<change_id>`”；`CONVENTIONS.md:188`（§6.1 台账列）、`:200`（§6.3 延期规则）、`:142`（§4 历史说明）、`docs/architect/MEMORY.md:46` 四处措辞一致；历史 36 行未回改 | 无 change_id 延期的兜底未定义（观察，见 §6） |
| MIN-1 状态先于批准 | **CLOSED** | `ADR:3` `accepted（Revision 2.1）` 有 `:4` 批准行 + `:1500-1501` §17.1 逐条批准记录（`:1503` Status）（K-1/K-2/K-4/K-6）+ `:1533` 版本历史，三处一致 | — |
| MIN-2 轻路径保留约束 / 变更输入 | **OPEN** | `AGENTS.md:84` 仍未声明“轻路径不减免 §2.7 变更输入（`:86`）/§2.8（`:87`）/Actor 独立性”；Batch B 仍无 `change_id`/`Tier`/`base`/`head`/`allowed_paths`：`review-tracking.md:324-339` 只有路径与 K 编号，`plan:38` 要求“一个 change_id” | 见 MINOR-3 |
| MIN-3 MEMORY owner 作者性 | **OPEN `[不确定]`** | 无提交、无 Actor trailer，无法证明四份 MEMORY 由各 owner 回写 | 见 MINOR-4 |
| MIN-4 “未裁决不进正文”自相矛盾 | **CLOSED** | `ADR:27` 改为“未裁决事项不进入正文规范条款；Rev 2.0 记录的两项冲突已于 2026-11-25 裁决并在 §18 关闭，正文按裁决结果收敛”；§18 标题改为“冲突裁决与关闭记录”（`:1543`），`:1596` “未裁决冲突 0 项” | 归档头部指针见 MINOR-2 |
| MIN-5 “台账格式”未限定 | **OPEN** | `AGENTS.md:84` 与宪法 `:77` 仍只写“台账格式”，未限定“不得删/改义既有列” | 见 MINOR-5 |

## 2. 本轮验证矩阵（按 Lead 的 7 项）

| # | 验证项 | 结果 | 证据 |
|---|---|---|---|
| 1 | BLK-1 关闭、轻路径受限 | **通过** | 宪法 `:77` + `AGENTS.md:84` 同源；排除代码/契约/CI/`scripts/**`；宪法另排除 ADR 修订；人类批准 + Reviewer 复核保留（`:77`“任何级别保留人类合入与 Release”） |
| 2 | MAJ-1 关闭、B-2 独立 ADR 轨 | **通过** | `review-tracking.md:324-329`；ADR Rev 2.1 批准记录 `:1500-1501`；`review-tracking` 不再声称“no ADR” |
| 3 | MAJ-2 关闭、无“必须开线程” | **未通过（PARTIAL）** | 见 MAJ-1（下文） |
| 4 | MAJ-3 关闭、G-4/`--release` 0 命中 | **通过** | ADR 全文 0 命中，替换措辞 4 处；`review-tracking.md:338-339` K-6 |
| 5 | MAJ-4 关闭、格式一致 | **通过** | `review-tracking.md:337`；`CONVENTIONS.md:188/:200/:142`；`architect/MEMORY.md:46` |
| 6 | Rev 2.1 增量正确、无规则削弱 | **通过（抽查）** | C-04 并入：§0.7 `:158`、§5.9 `:731-793`、§12.5.8 `:1321-1350`、§8 状态机新增 `PaymentRefundBatch` 两态（`:896`）、merge map 覆盖（`:40-41`）；与来源 `0007-amd1-refundbatch-c04-draft.md:81-83`（drop `uk_refunds_idem` + 保留 `uk_refunds_idempotency` + 新增 `uk_refunds_batch_item`）、`:96-98`（Order 预检 `B/done/inFlight/outstanding/R`、`expected_total_cent=outstanding`）逐条一致；C-2 按 K-2 收窄 A-08（`:113`、`:202`、`:241`、`:1072`、`:1396`），§18.2 `:1559-1568` 保留原冲突证据审计；`§5.9.2` 第 5 条要求“批内 all-or-none”**加强**而非削弱原语义 |
| 7 | 无新 BLOCKER/MAJOR；无新 gate/CI/state/spec | **部分** | 无新增 gate/CI/状态字段/脚本/迁移（28 个被改为 `.md`；`docs/specs/` 干净；`scripts/`、`.github/`、`Makefile` 未动）；但新增 1 项 MAJOR（MAJ-2，轻路径允许清单与 B-1 路径不一致） |

## 3. MAJOR

### MAJ-1（P1，承 QC-29 MAJ-2，PARTIAL）三处命令式“开线程/新增线程”仍在，退役未完全收口

| 文件:行 | 现存文本 | 冲突 |
|---|---|---|
| `docs/collab/developer-reviewer/threads.md:21` | “维护规则 **1. 新增审查线程：在 `threads/` 建 `T-NNN.md`**（带 YAML front-matter），再在本表加一行。” | 同文件 `:3` 横幅（终 3 行）“不得新增线程”；维护规则整节**未**像另外两个索引那样冻结 |
| `docs/collab/developer-reviewer/README.md:21` / `:25` | 规则 2“`T-NNN` 为 `OPEN + BLOCKING` 时不开新审查线程…Reviewer 复跑再改 `ANSWERED/CLOSED`”、规则 6“关闭由 Reviewer 执行…改 `status: CLOSED`、填 `closed_by`、同步索引表” | 仍以**现在时**命令状态变更，与 `architect-developer/README.md:18-26`、`po-architect/README.md:18-26` 已改为“（历史）”不一致；`threads.md:3` 已冻结历史状态 |
| `docs/architect/README.md:6-7` | “需要改设计结论：**开线程** → 改 ADR（Amendment）”、”需要改业务口径：**开线程给 PO**” | 该文件不在本次改动集（`git status` 无此项），仍教角色开线程；QC-29 MAJ-2 修复第 4 条已点名 |

- 依据：`AGENTS.md:87`（§2.8）“MUST NOT 留下死契约、**死命令**或死台账”；Batch B 自陈收益即“消除死命令”（plan`:40`）；Lead 本轮验证项 3 的“no remaining 'must open thread'”被上述 2 处字面推翻。
- **最小修复（3 处编辑）**：① `developer-reviewer/threads.md:21-24` 维护规则改为与另两个索引相同的冻结版（“不再新增线程；历史 status 保持原状；不得删除或改写”）；② `developer-reviewer/README.md:21/:25` 加“（历史）”前缀并把“改 status/填 closed_by”改为“退役后历史线程状态冻结”，与另两个 README 对齐；③ `docs/architect/README.md:6-7` 改为“提请 Lead 路由至 owner → Architect 以 Amendment 落地”。

### MAJ-2（P1，新）轻路径允许清单不含 `docs/collab/**`，但 B-1 修改了 6 个该类文件

- 规则（穷举式“仅”）：`AGENTS.md:84` 与宪法 `:77` 均写“**仅改** 本文件/`CONVENTIONS.md`/MEMORY/`prompt/**`/台账格式”。
- 实际：本批次修改 `docs/collab/{po-architect,architect-developer,developer-reviewer}/{README.md,threads.md}` **6 个文件**（sha1 见 §0；均为 pair 归档/索引文本），并另需修改 `docs/architect/README.md`（`docs/architect/**` 同样不在清单）。这些文件既不是 `CONVENTIONS.md`，也不是 MEMORY/`prompt/**`/台账。
- 后果：按 `AGENTS.md:85`（防作弊）“误判即 stop-the-line、按 L2 重做”，B-1 的 pair 文件改动在轻路径下**无授权路径**；同时这也解释了 MAJ-1 的修复为何“改不动”`docs/architect/README.md`——不是遗漏，而是范围外。
- **最小修复（二选一，均须人类）**：① 宪法 PATCH v1.3.1 + `AGENTS.md:84` 把清单扩为“…/台账格式/**`docs/collab/**` 的归档与索引文本（`README`、`threads.md`，不得含新契约）**”，随后 `docs/architect/README.md` 仍按“角色目录 README 属角色治理文本”显式列入；或 ② 在 `review-tracking.md` 记一条 K-7 范围授权：“Batch B 的 B-1 允许修改 `docs/collab/**/{README.md,threads.md}` 与 `docs/architect/README.md`（仅归档/指针文本）”，并注明这不改变轻路径的长期清单。

## 4. MINOR

- **MINOR-1（新）** T-002 以 `CLOSED` + `closed_by: "archived — pair threads retired 2026-11-25"` 归档（`T-002.md:7/:10`），但 `CONVENTIONS.md:171` 定义 `thread_status=CLOSED` 的**唯一含义是“沟通闭环”**，而 T-002 的 Review-01 FAIL（P-01…P-06）从未闭环。横幅与落点列已写“archived”，风险有限，但这是“一个状态两种含义”的复发。**修复**：改用 `status: ARCHIVED`（保留原文不动），或在 `CONVENTIONS.md:171` 增一句“历史线程以 archived 关闭者仅表示随协议退役归档，不代表沟通闭环”。
- **MINOR-2（新）** 归档头部仍把当前版写成 Rev 2.0 且指向“未裁决冲突”：`t12/t14a/t16/zero/amd1` 的 `Status`/`Superseded by` 行仍为“superseded by ADR-0007 **Rev 2.0** … 未裁决冲突见 §18”（如 `0007-amd1-addendum-t12-contract-rulings.md:5-6`）；`t14b1` 的 Merge note 仍写“其 R-2 与 C-04 状态冲突相关，见 **Rev 2.0 §18 C-1**”（`:6`），而 C-1 已于 2026-11-25 关闭且 §18 现为“裁决与关闭记录”（`ADR:1543`、`:1596` “未裁决冲突 0 项”）。**修复**：`Superseded by` 文本改“Rev 2.0/Rev 2.1（现版 Rev 2.1）”、“未裁决冲突见 §18”→“冲突裁决见 §18”；`t14b1` 改“C-1 已关闭，见 §18.2/§18.1”。
- **MINOR-3（承 QC-29 MIN-2）** 轻路径未声明保留的约束，且 Batch B 无变更输入：`AGENTS.md:84` 未写“不减免 `:86` 变更输入 / `:87` §2.8 / Actor 独立性”；`review-tracking.md:324-339` 无 `change_id`、`Tier`、base/head、`allowed_paths`。**修复**：`:84` 末补一句；`review-tracking` 记 `change_id: LC-BATCH-B`、`Tier: L2`、base `104b927`、head（提交后）、`allowed_paths`。
- **MINOR-4（承 QC-29 MIN-3）`[不确定]`** 四份 MEMORY 的 owner 作者性仍无法验证（工作区无提交、无 Actor/trailer）。**修复**：变更输入记各 owner 回写确认，或人类接受“Architect 代拟 + owner 追认”。
- **MINOR-5（承 QC-29 MIN-5）** “台账格式”仍是无限制词（`AGENTS.md:84`、宪法 `:77`）：可被解释为改 `ac-coverage.tsv` 列/语义。**修复**（下次 PATCH）：限定“不得删除或改义既有列；列定义变更仍由 Reviewer 执行并人类批准”，或从轻路径移除“台账格式”。
- **MINOR-6（新）** 宪法自身仍留两条退役前口径：`:96`（Governance 5）把 `docs/collab/<pair>/threads/` 列为 delta audit 的记录地，而 `CONVENTIONS.md:30/:142` 已定为只读存档、“不得新开”；`:75` 仍把“线程协议”称为 CONVENTIONS 的执行细则（现为 §4 历史说明）。**修复**：下次宪法 PATCH 把 `:96` 括注改为“（`docs/review-tracking.md`）”，`:75` 的“线程协议”改为“协作路由”。

## 5. 人类/Reviewer 仍需处理

| 项 | 内容 | 建议 |
|---|---|---|
| **H-1（新）** | 轻路径允许清单是否纳入 `docs/collab/**` 归档/索引与 `docs/architect/README.md`：修宪 PATCH 或记 K-7 范围授权（MAJ-2） | 优先 K-7 范围授权（不动规则，成本最低）；下一次修宪时补入清单 |
| **H-2（新）** | T-002 的 `CLOSED(archived)` 语义是否接受，或改 `ARCHIVED`（MINOR-1） | 建议改 distinct 状态，避免与 `CONVENTIONS.md:171` 冲突 |
| **H-3（承 QC-29 MIN-2）** | 记录 Batch B 的 `change_id`/`Tier`/base/head/`allowed_paths`，供 Reviewer 在 verdict 中确认 Tier | Reviewer 无此项不得出 PASS |
| Reviewer | 全文对照 Rev 2.1 与来源（本轮为抽查：§0.7/§5.9/§12.5.8/§8/§2.6/§5.5.1/§18.1-18.2 + A-08 各处） | §4.4/§6.2-§6.4/§12.1-§12.4 仍建议补一次逐条对照 |

## 6. 限制与不计入 findings 的观察

- 本轮为**抽查**，未逐字比对 Rev 2.1 全部 1596 行；已核对段见 §2 第 6 项。未运行任何测试（纯文档变更，按 `AGENTS.md:127` 与宪法 `:81` 不得伪造测试声明）。
- K-5 只定义格式，未定义“无 `change_id` 的延期”（如 AC-22…37 续期）如何填 `thread`；不构成 finding（K-5 已裁决），但建议 Reviewer 在 Batch C 前用一个真实续期行验证可解析性。
- T-002 由 `OPEN` 改 `CLOSED` 属退役归档动作（内容未删、`closed_by` 写明 archived）；QC-29 曾建议不改状态，Director 侧已按退役处理，本报告只就语义冲突记 MINOR-1。
- 未发现的新增风险（正向确认）：无新 gate/CI/状态字段/脚本/迁移；`docs/specs/**` 未被修改；`ac-coverage.tsv` 36 行未动；7 个历史线程文件与 3 个 pair 目录完整；宪法 `:77` 与 `AGENTS.md:84` 的排除项一致；ADR 全文 `G-4`/`--release`/`--range` = 0 命中，无死命令残留。
- 本报告只读：仅新增 `docs/collab/qc-reports/QC-30-BATCH-B-R2.md`；未修改任何被审文件。
