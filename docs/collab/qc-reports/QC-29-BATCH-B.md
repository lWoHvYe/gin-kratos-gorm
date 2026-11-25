# QC-29 — Batch B 治理批次（轻路径）审查

| 项 | 值 |
|---|---|
| review_scope | `docs/phase-legacy-cleanup-and-optimization-plan.md` Batch B 的 B-1…B-5（线程退役、ADR-0007 Rev 2.0 合并、MEMORY 收敛、Lead/QC prompt 维护）；对照 `AGENTS.md` §2.7 新增轻路径与 §2.8、`docs/collab/CONVENTIONS.md`、宪法 v1.2.3、ADR-0007 全部 addendum 头部、`docs/review-tracking.md`、四份 MEMORY、`prompt/{Lead,Quality-Checker}.md`、`prompt/Lead.md` 主责路径 |
| 被审对象 | 工作区未提交变更 19 个文件（全部 `.md`；`git status --porcelain` = ` M`，无新增/删除文件、无脚本/CI） |
| 版本绑定 | **无 commit/head**：变更尚未提交，HEAD 仍为 `104b9276fa2eafd5ac54ad6d9a55a2d701a0dddd`（Batch A）；sha1 均为工作区 blob（`git hash-object`，下同） |
| HEAD（base） | `104b927` |
| 对照基线 | 宪法 v1.2.3（sha1 `466fc1508797`，未改）；plan（未改，行号可直接引用）；review-tracking（未改）；SPEC-0002 v1.9（只读上下文，未改） |
| 审查类型 | 治理文本轻路径 QC：静态、只读、红队。不替代人类批准、不替代 Reviewer verdict、不构成 AC 验收或 Release PASS |
| 结论 | **`BLOCKED`** —— **1 BLOCKER / 4 MAJOR / 5 MINOR** |

## 0. 被审 sha1（工作区 blob，前 12 位）

| 文件 | sha1 | 文件 | sha1 |
|---|---|---|---|
| `AGENTS.md` | `a1b962eec421` | `docs/adr/0007-payment-settlement-admission-amd1.md` | `8f110089d483` |
| `docs/collab/CONVENTIONS.md` | `0f2745f0db21` | `docs/adr/0007-payment-settlement-admission.md` | `ccef0d63d30b` |
| `docs/architect/MEMORY.md` | `44df282f223c` | `...-amd1-c04-c05-handoff.md` | `c6625fbda09d` |
| `docs/developer/MEMORY.md` | `a80bc6bf6b21` | `0007-amd1-addendum-t12-contract-rulings.md` | `ec13fc5462cb` |
| `docs/po/MEMORY.md` | `64210ce7cbb6` | `0007-amd1-addendum-t14a-contract-rulings.md` | `e6dc234f18c8` |
| `docs/reviewer/MEMORY.md` | `fa7163f9b08a` | `0007-amd1-addendum-t14b1-payment-sigma.md` | `12aac3942b59` |
| `prompt/Lead.md` | `7a67d017a3c5` | `0007-amd1-addendum-t14b2-inv44-group.md` | `e2ba47189009` |
| `prompt/Quality-Checker.md` | `0b16c9123382` | `0007-amd1-addendum-t16-payment-cancel.md` | `9768df904777` |
| `0007-amd1-refundbatch-c04-draft.md` | `2fdfa7df6bab` | `0007-amd1-addendum-zero-after-sale.md` | `42e48dd1f6cc` |
| `0007-amd1-refundbatch-c04-rulings.md` | `cd7b0e3311f3` | 被合并前的 ADR-0007（`HEAD:`） | `7f34ca6e8917` |

> 说明：ADR-0007 Rev 1.0 正文被**原地覆盖**（旧 blob `7f34ca6e8917` 只存在于 git 历史），头部 `:13` 的 Supersedes 只写“本文 Rev 1.0 正文（历史版本，条款已并入本版）”，未给归档指针/commit（见 MINOR-4 附注）。

## 1. Must-check 结论矩阵

| # | 检查项 | 结果 | 主要证据 |
|---|---|---|---|
| 1 | 轻路径不是漏洞（限治理文本、排除代码/契约/CI/脚本、他人不得用、保留人类+Reviewer） | **不通过** | **BLK-1**（与宪法 L2 全阶梯冲突）；**MAJ-1**（本批次把契约文本并入轻路径）；MIN-5（“台账格式”未限定）；正向：`AGENTS.md:84` 排除“代码、契约、CI、`scripts/**`”，`AGENTS.md:85` 防作弊保留人类 Release |
| 2 | B-1 线程退役 | **部分通过** | 正向：`CONVENTIONS.md:23/25/28/30/34` 改“提请 Lead 路由”；`:138-142` 历史说明；`:188`/`:200` 新延期格式；pair 三目录仍在（`po-architect` T-001..003、`architect-developer` T-001/002、`developer-reviewer` T-001/002）；`ac-coverage.tsv` 仍 36 行、`thread` 列 = `developer-reviewer/T-001` ×6 + `T-002` ×30、**未改动**。负向：**MAJ-2**（6 个 README/索引仍写现行协议 + T-002 仍 OPEN）；**MAJ-4**（新格式未经人类裁定） |
| 3 | B-2 ADR 合并 | **部分通过** | 正向：merge map `ADR:29-77` 覆盖 5 个规范来源（Amd1 + Addendum 1/2/3 + T1.6）**且**逐条标 `superseded`（`:13-19`、各 addendum 头部新增 `Superseded by` 行）；未并入 4 个 + handoff 分类正确（`:20-25`、`:1458-1474` 计数自洽：5 merged / 4 not merged / 2 冲突）；**C-1 `:1425-1435`、C-2 `:1437-1446` 显式记录且未消解**，正文多处回指（`:110`、`:188`、`:227`、`:292`、`:395`、`:611`、`:948`、`:984`、`:1276-1277`、`:1302`）；§18 每项给出 (a)/(b) 两条可执行走向。负向：**MAJ-3**（已退役闸门 G-4/`--release --range` 未记开放项）、MINOR-1、MINOR-4。抽查未发现已批准规则被削弱：`§5.2.1` 行/组/订单三级上限与 Amd1 §4.4 逐句一致；`§5.2.5` 零元短路 8 条与 Zero §3.1 一致；`§2.6`/`§5.5.1` 规则 5 保留 Amd1 §4.1.3(5) 的“每笔前重读 + 停止后续打款 + MANUAL_HOLD”〔**未逐字比对全部 5 个来源文件的每一句**，见 §6 限制〕 |
| 4 | B-3 MEMORY 收敛 | **通过**（作者性见 MINOR-3） | 四份头部行齐备：`architect/MEMORY.md:6`、`developer/MEMORY.md:4`、`po/MEMORY.md:3`、`reviewer/MEMORY.md:6`（“只收跨变更仍成立的稳定经验；运行状态见 review-tracking / ac-coverage”）；“当前基线/当前交付状态/当前残余/审查项状态”表已删（reviewer 原 §2/§3、architect 原“当前基线/未完成项”、developer 原 §2/§4、po 原 §2/§4）；刷新义务句已删（architect 守则第 5 条改为“不写运行状态”，reviewer §7 第 4 条已删）；角色边界/守则/易漂移点/决策索引保留；grep `PASS|commit|expiry|CLOSED|OPEN` 后仅剩“规则/退役日期”，无运行状态表 |
| 5 | B-4/B-5 prompts | **通过** | `prompt/Lead.md:9-10` 分解纪律自查（独占不变量 + 跨服务身份字段契约归属）；`:22-34` Brief 模板覆盖计划 a–i 全部要素（a `:25`；b `:26`；c `:27`；d `:28`；e `:29`；f `:30`；g `:31`；h `:32`；i `:33`）；`:7` delta audit 逐角色核对表（PO/Architect/Developer/QC/Reviewer + “已刷新/未刷新 + owner”）；`docs/reviewer/MEMORY.md:25` 新增第 7 题；`prompt/Quality-Checker.md:13-16` Plan 红队两问。无角色边界扩张：仅 Lead/QC 两个 prompt 变更，与计划 B-4/B-5 逐项对应 |
| 6 | 一致性（宪法 v1.2.3 / AGENTS §2.7 / CONVENTIONS；无新 gate/CI/状态/脚本；人类检查点完整） | **不通过** | **BLK-1**；`AGENTS.md:127`（闸门已删、不得要求运行）vs `ADR:112/:771/:973/:979`（把 G-4/`--release --range` 写成强制前置）→ MAJ-3；无新增 gate/CI/state/script（19 个被改文件全为 `.md`，`scripts/`、`.github/`、`Makefile` 未变）；人类检查点：轻路径文本保留“人类批准”且 `AGENTS.md:85` 保留人类 Release（但见 BLK-1） |
| 7 | Traceability | **基本通过** | 本报告所有结论均带 `文件:行号`；ADR §18 的 6 处外部引用逐条核对成立（`review-tracking.md:118-123` C-04 accepted、`:31-40` T1.5 observe-only 原话、`:75-81` P0 单组简化、`:89` B2 closed）；两处不可验证已标 `[不确定]`（§6） |

---

## 2. BLOCKER

### BLK-1（P0）轻路径与宪法 Workflow 1 的“L2 MUST 走完整阶梯”直接冲突，且无合宪豁免记录

**证据**

- 新增规则：`AGENTS.md:84`“治理文本类 L2 … MAY 走轻路径：Architect 起草 → QC → 人类批准 → Reviewer 复核，**不建 Spec/ADR/Plan**”。
- 最高准则：`.specify/memory/constitution.md:76`（Workflow 1）“L0/L1 的适用与减免以 `CONVENTIONS.md` §2.3 为准；**L2 MUST 走本条完整阶梯**。任何级别保留人类合入与 Release”——只对 L0/L1 开减免，对 L2 无任何减免出口。
- 冲突后果条款：宪法 `:100`（Amendment & Compliance 1）“冲突时本宪法胜出，**冲突产物 MUST 被拒绝合入**”；`:105`（人类豁免）“豁免 MUST 以书面形式记录于 `docs/` … **MUST 注明范围、期限与受影响不变量**；豁免不修改本宪法文本”。
- 本批次未动宪法：`:108` 仍 `**Version**: 1.2.3`（工作区 sha1 `466fc1508797` 与 HEAD 一致）。
- 仅有“决策”而无“豁免”：`docs/review-tracking.md:324-326` 只写“D-LC-2 / D-LC-3: Batch B uses the **light governance path** (Architect -> QC -> human -> Reviewer; no Spec/ADR/Plan); ADR-0007 (7 addenda) merges to Rev 2.0 in this batch”——**未**按 `:105` 写范围/期限/受影响条款，也未引用 `AGENTS.md:84`。
- 对照先例：L0/L1 分级进入宪法时是**同步修宪**的（宪法 `:76` 明写 L0/L1 减免；`docs/review-tracking.md:310-313` 记“宪法 v1.2.3 … v1.2.2 L0/L1/L2 分级路由”）。本次轻路径跳过了同一程序。

**为什么这是漏洞而不是措辞**：`AGENTS.md:85`（防作弊）已明确治理/工具链变更（含本文件、CONVENTIONS、宪法）**恒为 L2**；而 `prompt/Lead.md:20` 又要求“治理/工具链变更恒为 L2；误判即冻结并按 L2 重做”。一边定义治理文本为 L2，一边给它一个绕开宪法 L2 阶梯的出口，等于把最高准则的约束交给一份优先级更低的文件改掉——正是本批次要消灭的“文档漂移”里最贵的一类（`AGENTS.md:3` 优先级：宪法 > 本文件 > CONVENTIONS）。

**最小修复（二选一，均须人类）**

1. **修宪（建议）**：按宪法 `:101`（`/speckit-constitution`）出 PATCH v1.2.4，在 Workflow 1 增一句：“治理文本类 L2（范围与排除项同 `AGENTS.md` §2.7）MAY 走 Architect → QC → 人类批准 → Reviewer 复核，不建 Spec/ADR/Plan；范围变更须人类批准。” 同批提交，由宪法批准覆盖轻路径本身。
2. **书面豁免**：在 `docs/review-tracking.md` 按 `:105` 记一条人类豁免——范围=治理文本（`AGENTS.md`/CONVENTIONS/MEMORY/`prompt/**`/台账格式，排除契约与代码）、期限=至下次宪法修订、受影响条款=Workflow 1 的 L2 阶梯与 `SPEC_APPROVED`/`ADR_APPROVED` 检查点，并在 `AGENTS.md:84` 引用该豁免。仅“决策记录”不足以代替该格式（`:105`“无记录的豁免视为不存在”）。

> 在 BLK-1 关闭前，任何角色不得以“已按轻路径送审”为由把本批次视为合宪；Reviewer 也不得据现有记录出具 PASS。

---

## 3. MAJOR

### MAJ-1（P1）B-2 把**契约文本**并入轻路径，越出轻路径自身的排除项

- 轻路径范围：`AGENTS.md:84`“仅改本文件、`CONVENTIONS.md`、MEMORY、`prompt/**`、台账格式；**不改代码、契约**、CI、`scripts/**`”。
- 实际改动：`docs/adr/0007-payment-settlement-admission.md`（+1145/−…；sha1 `ccef0d63d30b`）、`...-amd1.md` 与 5 个 addendum 头部、3 个未并入文件头部、handoff 头部——全部是 `docs/adr/**`，即规范性契约文本。
- 记录冲突：`docs/review-tracking.md:324-326` 明写“**no Spec/ADR/Plan**”且“ADR-0007 (7 addenda) merges to Rev 2.0 **in this batch**”；`ADR:3` 与 `ADR:1381` 也把 Rev 2.0 合并记为“按 D-LC-2/D-LC-3 轻路径送审”。即：**契约文本正沿一条明文排除契约的路径送审**。
- 适用规则：`AGENTS.md:85`“误判即 stop-the-line，冻结 `change_id` 并按 L2 重做”；`AGENTS.md:83`“L2 … 不减免任何批准点”。

**最小修复**：在 Batch B 变更输入中**拆两路**并各记 `Tier-Rationale`——

1. B-1/B-3/B-4/B-5：治理文本轻路径，`allowed_paths` = `AGENTS.md`、`CONVENTIONS.md`、`docs/{architect,developer,po,reviewer}/MEMORY.md`、`prompt/**`；
2. B-2：**ADR 规范文本修订（L2）**，`allowed_paths` = `docs/adr/0007-**`，流程 = Architect 起草 → QC → 人类批准（ADR_APPROVED）→ Reviewer 复核，不建 Plan；并在 `review-tracking.md:324-326` 把“no Spec/ADR”改为“no Spec/Plan；B-2 是 ADR 修订”。

不要用“给轻路径加一条 ADR 例外”来解决——那会把“合并/编辑性重述”变成契约文本的通用快捷通道，轻路径随即成为漏洞。

### MAJ-2（P1）B-1 未收口：pair 目录与角色 README 仍把线程协议写成现行强制（僵尸待办 + 死命令）

- `CONVENTIONS.md:138-142` 宣布线程退役、只读存档、不得新开；`:30` 权限表亦改为存档。
- 但以下文件仍写**现行强制**：
  - `docs/collab/architect-developer/README.md:20`“Developer 遇到 ADR 未定义的异常，**必须开 `T-NNN` 问**”；`:21`“commit trailer 用 `Thread: architect-developer/T-NNN`”；
  - `docs/collab/architect-developer/threads.md:4`（“协议见 … §3”）及末尾“维护规则 1. 新增审查线程…”；`developer-reviewer/threads.md:4`、`po-architect/threads.md:4` 同；
  - `docs/collab/developer-reviewer/README.md:19`（`T-NNN` 为 `OPEN + BLOCKING` 时不开新线程）、`:22`（“**开线程**给反证”）；
  - `docs/collab/po-architect/README.md:18-19`（谁写线程段、不开新线程的排他规则）；
  - `docs/architect/README.md:6-7`“开线程 → 改 ADR”“开线程给 PO”；
  - `docs/collab/developer-reviewer/threads.md:9` T-002 仍标 **`OPEN`**，`:13`“`OPEN`：1（T-002，待 ADR-0007）”——僵尸待办。
- 计划已把该项划入 B-1：plan`:117`“三个 pair 目录的 README 未读，其过时描述**随 B-1 处理**”。
- 适用规则：`AGENTS.md:87`（§2.8）“MUST NOT 留下死契约、死命令或死台账”；本批次自陈收益为“消除死命令、状态多写、僵尸待办三类已发生的漂移”（plan`:40`）。

**最小修复**（不改历史内容、不伪造关闭）：

1. 6 个文件（3 个 pair `README.md` + 3 个 `threads.md`）顶部加统一存档横幅：“pair 线程协议 2026-11-25 退役，本目录只读存档；角色间交互经 Lead 路由（`CONVENTIONS.md` §4）”。
2. 把“必须开线程 / `Thread:` trailer / 一次只推进一个阻塞线程 / 新增线程规则”等**命令式**语句改为过去时存档说明；`threads.md` 的“协议见 §3”改为“§4（历史说明）”。
3. T-002 的 `OPEN` **不得**改写为 CLOSED（历史不回改），但索引加一行：“历史状态；AC-39…61 延期跟踪见 `ac-coverage.tsv`（Batch A A-3）”。
4. `docs/architect/README.md:6-7` 改为“需要改设计结论：提请 Lead 路由 → Architect 以 Amendment 落地”。

### MAJ-3（P1）Rev 2.0 保留**已退役闸门 G-4** 与**已删命令 `--release --range`** 作为 SHADOW→ON 的强制前置，却未在 §18 记为开放项

- `ADR:112`（A-10）：“SHADOW→ON 需 SPEC §7/**G-4**/五方/`--release --range` + Director”；
- `ADR:771`（§6.4 步骤 3）、`ADR:973`（§12.3 步骤 3）、`ADR:979`（“生产开关默认 false，直到 … **G-4**、`--release --range` … 通过”）同义重复。
- 现状：`AGENTS.md:127`“协作闸门已退役（2026-11-25）：`make gates*`、`scripts/collab_gates.py` 与 CI `collab-gates` job 均已删除；**不得要求运行已删命令**”；`docs/review-tracking.md` 2026-11-25 gate retirement 节同。
- 合并处理不完整：既未改写（正确地：不擅自改已批准 A-10），也未在 §18/§13 记为开放项——§18 只有 C-1/C-2（`:1425`/`:1437`），§18.3 的六类“不构成冲突”也未提闸门退役；而本批次自陈收益包含“消除死命令”（plan`:40`）。
- 后果：未来的 Architect/Developer 按 Rev 2.0 执行 `settled` 切换时会要求一个**不存在的闸门**与一个**已删脚本参数**，属 `AGENTS.md:87`“死命令”。

**最小修复**：§18 增 `18.5`（或 §13 第 36 条，与 C-1/C-2 同级）：“A-10/§6.4/§12.3 中的 `G-4` 与 `--release --range` 已于 2026-11-25 随协作闸门退役失效；SHADOW→ON 的替代前置（SPEC-0002 §7 矩阵 + 五方终态核对 + Reviewer/QC 证据 + Director 批准，按人工清单执行）需 Director 裁决；不得要求运行已删命令。” §18.4 计数同步“未裁决冲突 3 项”。

### MAJ-4（P1）新延期 `thread` 格式未经人类裁定即落地，且指针目标不可解析

- 计划明写由人类定：plan`:47`“新延期行的 `thread` 取值改为 `review-tracking/<条目标识>`（**格式由人类定**）；历史 36 行不回改”。
- 实际落地为 `review-tracking/<change_id>`，且在 **4 处**副本中各写一遍：`CONVENTIONS.md:142`、`:188`、`:200`、`docs/architect/MEMORY.md:46`。
- 可解析性问题：`review-tracking.md` 是**按日期分节**的台账（`:1-6` 只声明 canonicity；示例 `:118`“2026-11-24 C-04 RefundBatch accepted”），没有 `change_id` 主键与索引；`review-tracking/<change_id>` 无法唯一定位节。计划原本的“条目标识”与实现取 `<change_id>` 也不一致。
- 无 change_id 的情形：延期可在无变更时产生（例如 AC-22…37 在 Reviewer 审查中续期，plan Batch C C-1.1），此时 `<change_id>` 为空 → 死指针。
- 未破坏的部分（正向）：`ac-coverage.tsv` 未被改（`git status` 无该文件）、仍 36 行、`thread` 列仍为 `developer-reviewer/T-001`×6 + `T-002`×30；列 schema 未变（`CONVENTIONS.md:188` 列清单与 HEAD 一致）；无解析脚本（`scripts/`、`Makefile` 均无 `thread` 引用；闸门已删），故**不构成门禁破坏**，但构成语义死指针。

**最小修复**：人类裁定格式后**单点定义**（`CONVENTIONS.md` §6.3 定义，其余 3 处只引用）；建议 `<节标题或日期>`（如 `review-tracking#2026-12-15-expiry`），并写明“无 `change_id` 时用最近裁决节的日期/标题”；不要求回改历史 36 行。

---

## 4. MINOR

### MINOR-1 Rev 2.0 自宣 `accepted` 而批准记录仍“待回填”（状态先于批准，与 C-1 同类漂移）

- `ADR:3`“Status: **accepted（Revision 2.0 合并版）**”；同一文件 `:1381`（§17.1）“本 Rev 2.0 合并 … **批准记录待回填**”。头部 `:4` 的 `Approved:` 只列 2026-11-23/24/25 的原对象，无 Rev 2.0。
- 这正是 C-1（`:1425-1435`）暴露的“文件自称 accepted、批准事实另有记录”的同一形态；若人类最终改稿或不予批准，仓库将留下一个自称 accepted 的规范性文本。
- **最小修复**：Status 暂写 `proposed（Rev 2.0 合并版，待 Director 批准）`；批准后由 Architect 在 §17.1 补批准行并把 Status 改回 `accepted`（与 `:4` 并列）。

### MINOR-2 轻路径未声明保留的约束；Batch B 自身缺少 §2.7 变更输入

- `AGENTS.md:84` 未写“轻路径仍受 `AGENTS.md:86` 变更输入（`change_id`/`Tier`/`Tier-Rationale`/`base`/`head`/`allowed_paths`/`baseline_impact`）、`AGENTS.md:87` §2.8（最小化/停止条件/撤回）、Actor 独立性（`AGENTS.md:79`）、人类 Release 约束”。
- 实际缺失：Batch B 无 `change_id`——plan`:38` 要求“一个 change_id”，但 `review-tracking.md:324-326` 只记路径与 D-LC 编号；无 `Tier: L2` trailer、无 base/head、无 `allowed_paths`、无 `Tier-Rationale`（`AGENTS.md:86` 要求）。
- **最小修复**：`AGENTS.md:84` 末尾补“轻路径不减免 §2.7 变更输入、Actor 独立性、人类合入/Release 与 §2.8 约束”；`review-tracking.md` 记 `change_id: LC-BATCH-B`、`Tier: L2`、base `104b927`、head（提交后）、`allowed_paths` 清单与 `Tier-Rationale`，供 Reviewer 在 verdict 中确认 Tier（`AGENTS.md:1.5`）。

### MINOR-3 B-3 的“各角色 owner 执行”无法验证 `[不确定]`

- plan`:58`“各角色 owner 执行，Reviewer 复核”；`CONVENTIONS.md:32`“角色记忆 … 唯一作者 = 对应角色”。
- 工作区全部为未提交修改（无 commit、无 Actor trailer、无 `git status` 差异归属），本报告无法证明四份 MEMORY 由各自 owner 回写，而非由单一 agent 代拟。`[不确定]`
- **最小修复**：变更输入记“四份 MEMORY owner 回写确认（架构/开发/PO/Reviewer 各自 actor id）”，或人类明确接受“Architect 代拟 + 各 owner 追认”。

### MINOR-4 “未裁决事项不进入正文规范条款”与正文中的 MUST NOT 注记自相矛盾

- `ADR:27`：“未裁决事项**不进入**正文规范条款，集中记录在 §18；不得由实现自行选择。”
- 但 `ADR:292`（§2.6 正文）“在该冲突关闭前，任何实现**不得不经裁决地依赖或排除** batch 契约…”、`:395`/`:611`（§3.7/§5.4）同义，`:1276-1277`（§13.34/35）亦为“不得据单方文本实现/验收”。这些是**合并时新写的约束**，不是来源条款。
- **最小修复**（择一）：头部 `:27` 改为“未裁决事项以**注记**形式标注、不构成新规则，结论集中在 §18”；或把 `:292` 的禁令移入 §18.1、§2.6 只留指针。
- 附注（同条）：`ADR:13` 的 Supersedes 未给 Rev 1.0 的归档指针；旧正文只能靠 `HEAD:` blob `7f34ca6e8917` 找回。建议补“Rev 1.0 全文 = `104b927:docs/adr/0007-payment-settlement-admission.md`”。

### MINOR-5 轻路径中的“台账格式”未限定，存在弱化 AC 台账的解释空间

- `AGENTS.md:84` 把“台账格式”列为轻路径可改项；`CONVENTIONS.md:188` 的 AC 台账列（`ac / owner / expiry / reason / thread / test_type / evidence_path / status`）是 `AGENTS.md:126`（§6.5 AC 证据矩阵）与 `CONVENTIONS.md:200` 延期规则的依赖面。
- 若“格式”被解释为可删 `expiry`/`thread`/`status` 列或改变其语义，就等于用治理轻路径关掉 AC 债务的可见性——与 `CONVENTIONS.md` §10 反模式“删 deferred 行 / 修改阀门使其恒真”同效。
- **最小修复**：`AGENTS.md:84` 把“台账格式”限定为“**不得删除或改义既有列**；台账内容与列定义变更仍由台账 owner（Reviewer）执行并经人类批准”，或直接从轻路径范围移除“台账格式”。

---

## 5. 人类仍需裁决的清单

| ID | 裁决什么 | 关联 | 建议 |
|---|---|---|---|
| **K-1（C-1，既有）** | C-04 `RefundBatch` 到底是 accepted 还是 DRAFT；若 accepted 需 Rev 2.1 并入 `Σ NEW <= remaining`、首次 Reply 固定、错误映射；若不 accepted 需对“AC-65 不得部分打款”残余竞态给书面豁免或重排 Plan，并把 t14b1 标 rejected | `ADR:1425-1435`、`:18`、`:1378` | 先裁状态，再决定是否 Rev 2.1 |
| **K-2（C-2，既有）** | A-08“消费者不再 `Refund`”与 T1.6“消费者执行 `pay:<payment_no>:cancel`”哪一方为规范；若收窄 A-08 需明确“仅支付级取消命令可执行” | `ADR:1437-1446`、`:110`、`:188`、`:227` | 收窄 A-08 措辞（T1.5 裁决记录 `review-tracking.md:31-40` 已指向该方向），但须人类书面确认 |
| **K-3（新，阻断）** | 轻路径合宪化：修宪 PATCH v1.2.4，或按宪法 `:105` 记书面豁免（范围=治理文本、期限=至下次修宪、受影响=Workflow 1 L2 阶梯与 SPEC/ADR 批准检查点） | `AGENTS.md:84`、宪法 `:76`/`:100`/`:105`/`:108`、`review-tracking.md:324-326` | 修宪（更彻底，消除“低优先级文件改高优先级规则”的先例） |
| **K-4（新）** | B-2 是否作为独立 ADR 修订记录（`docs/adr/**` 不属轻路径范围）；Rev 2.0 合并本体批准 + §17.1 批准行回填 + Status 由 proposed 改 accepted | `AGENTS.md:84`、`ADR:3`/`:1381`、`review-tracking.md:324` | 是：B-2 单列 ADR 修订；批准后回填 |
| **K-5（新）** | 新延期 `thread` 格式最终裁定（是否用 `review-tracking/<change_id>`、无 change_id 时兜底） | plan`:47`、`CONVENTIONS.md:142/188/200`、`architect/MEMORY.md:46` | 改为可定位的 `<节标题或日期>`，单点定义 |
| **K-6（新）** | A-10/§6.4/§12.3 中 G-4 与 `--release --range` 的替代前置（人工清单？还是登记豁免） | `ADR:112/771/973/979`、`AGENTS.md:127` | 记入 §18.5，按人工清单执行，不要求运行已删命令 |

---

## 6. 限制与不计入 findings 的观察

- **未逐字比对** Rev 2.0 正文 1474 行与 5 个来源文件的每一句；本报告对“已批准规则未被削弱”的结论基于：merge map 覆盖 5 个规范来源、4 个未并入文件分类正确、§5.2 三级额度/§5.2.5 零元/§2.6+§5.5.1 规则 5 逐句抽查一致。**建议 Reviewer 在复核时补一次全文对照**（尤其 §4.4、§6.2–§6.4、§8、§12.1–§12.4）。
- `docs/reviewer/MEMORY.md:59`（§6 第 9 条）仍保留 `responsibility.go:312-317`、`:220-230`、`:349-355`、`service.go:283-286`、`workers.go:414-416` 等**硬行号**。属既有内容（本批次未新增），但 B-3 目标是“只留稳定经验”；行号随代码漂移即过期，建议下次触碰时改为符号名。
- 未发现的问题（正向确认）：无新增 gate/CI/状态字段/脚本（19 个被改文件全为 `.md`）；`ac-coverage.tsv` 36 行与 `thread` 列未动；三个 pair 目录及历史线程文件完好（`po-architect` 3 个、`architect-developer` 2 个、`developer-reviewer` 2 个）；宪法未改（`sha1 466fc1508797`）；SPEC-0002 v1.9 未改；`AGENTS.md:127` 的闸门退役声明与 `CONVENTIONS.md:138-142` 的线程退役不冲突。
- 本报告只读：仅写入 `docs/collab/qc-reports/QC-29-BATCH-B.md` 一个文件；未改任何被审文件。`docs/collab/qc-reports/README.md` 的 QC-29 索引行（Batch A A-4 的索引）留待 QC 在下一轮补登记，不属于 Batch B 的审批条件。
