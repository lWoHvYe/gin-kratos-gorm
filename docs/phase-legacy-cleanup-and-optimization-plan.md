# 阶段三遗留清理与协作体系优化方案（转交执行版）

| 项 | 值 |
|---|---|
| 性质 | 执行方案 + 治理修订提案。Batch A 为 L0 文档清理 (A-2a 需人类批准一句话)；Batch B 为治理文本编辑，按 L2 阶梯执行；Batch C 为日历任务。全文不新增治理机制。 |
| 起草者 | 内部静态审查 Agent，应人类 2026-11-25 指示。按 CONVENTIONS §2.2 只作问题提案。 |
| 基线 | 2026-11-25 本地快照，无 git。行号请在执行前对照当前 HEAD；已做过的跳过。 |

---

## 0. 背景摘要 (为什么有这份方案)

四代协作演变（单 Agent 全包 -> A-B 双 Agent -> multi-chat 分角色 SDD -> Lead+subagent+宪法）中，阶段四继承了阶段三的全部产物，但**至少两套机制未做退役处置**：角色 MEMORY 子系统与 pair 线程协议。它们的设计前提（持久角色会话、两两直连拓扑）在阶段四已不成立，导致文档漂移与僵尸待办。同时 PAY-SETTLE-01 两天高强度运行 (五圈 stop-the-line、60+ 份 QC/REV 报告) 暴露了状态多写、设计碎片化等执行层问题。本方案按“防主错误行为 > 防误导判断 > 降成本”排序处置。

---

## Batch A: L0 文档清理，现在执行

A-1 是 2026-11-25 已批准的闸门退役的漏项，不是新决策 (AGENTS §2.8、§6.6)。

| # | 改哪里 | 改什么 | Owner |
|---|---|---|---|
| A-1a | `docs/architect/MEMORY.md` | 删“当前基线”表中的 ADR-0006 行、“未完成项”中的 T-003/G-4 条目、“闸门 (ADR-0006)”整节、“常用命令”中的 `make gates-strict` 行 (按节各位定位) | Architect |
| A-1b | `docs/collab/po-architect/threads/T-003.md` 及该目录 `threads.md` 索引 | 关闭，注明 `CLOSED - superseded by gates retirement 2026-11-25` | PO 与 Architect 会签 |
| A-2a | `docs/review-tracking.md` 头部 | 加一句：“本文件是运行状态 (裁决、冻结、PASS/FAIL、治理决策) 的唯一台账；tasks.md / plan.md / handoff.md / 线程均为视图，冲突时以本文件为准” (属事实所有权声明，需人类批准) | Lead 提议、人类批准 |
| A-2b | `docs/lead/PAY-SETTLE-01/tasks.md` | 头部加“本文件是 Plan 视图，不拥有运行状态，状态见 review-tracking”；更正头部过时的 “T1.2 FROZEN”；已被后续轮次取代的任务不补勾选，行尾标 `[superseded -> review-tracking]` (T1.2-R 的 R-1..R-5, T1.4b1-R2 的 R2-1..R2-5) | Lead |
| A-2c | `docs/handoff.md` | 头部 | 加过时声明和指针：Snapshot 停在 2026-11-22，以 review-tracking、`docs/specs/`、decomposition-review 为准 | Lead |
| A-2d | `docs/lead/PAY-SETTLE-01/plan.md` §8 (第119-120行) | Phase 0 sign-off 仍是 pending 而 Phase 1 已推进：Lead 先核实是已发生未回写还是确实缺失，再由人类定 (D-LC-5) | Lead 核实 -> 人类 |
| A-3 | `docs/collab/developer-reviewer/threads/T-002.md` | 延期跟踪改为指针 (AC-39..68 的状态见 `ac-coverage.tsv`)；spec 版本改为以 `docs/specs/` 当前版为准 | Reviewer |
| A-4 | 新建 `docs/collab/qc-reports/README.md` | 索引：三类命名 (`QC-nn-*`、`REV-n`、`*-Review-01`) 加按任务号的文件列表 | QC (`REV-n`、部分与 Reviewer 协作) |

约束：T-001、T-002 线程文件的路径不得删除。`ac-coverage.tsv` 全部 36 行的 `thread` 列都指向它们。

---

## Batch B: L2 治理批次

一个 change_id，走 Architect 评审 -> QC -> 人类批准 -> Reviewer 复核。执行时机见 D-LC-3。

§2.8 声明：收益是消除死命令、状态多写、僵尸待办三类已发生的漂移，并补上 Lead 的 brief 模板。停止条件：P0 收口时若 B-1 的四项替代载体 (review-tracking、Lead plan/tasks、REV 报告、ac-coverage) 有未承接住的，恢复对应线程职能。

### B-1 pair 线程协议退役 (D-LC-2)

- 供裁决的事实：阶段四的 subagent 是临时实例，时间上不共存，无法直连。PAY-SETTLE-01 两天协作没有新开线程，线程原有的四项职能已由 review-tracking、Lead plan/tasks、REV 报告、ac-coverage 承接。
- 动作：
  1. CONVENTIONS §4 线程协议节改为历史说明；§3 权限表的“要改就开线程”改为“提请 Lead 路由至 owner”。
  2. 台账规则 (第192/204行)：新延期行的 `thread` 取值改为 `review-tracking/<条目标识>` (格式由人类定)；历史 36 行不回改。
  3. 三个 pair 目录保留为历史存档。宪法 Governance 5 引用的路径因此仍在，宪法不改。
  4. Reviewer MEMORY §1、§7 和 architect MEMORY“协作通道”节里的线程指针，随 B-3 标为存档。

### B-2 ADR addendum 收敛

`docs/architect/MEMORY.md` 工作守则第 4 条追加：“单个 ADR 的 addendum > 5 或 P0 收口时，MUST 合出 Rev 2.0 并 supersede 全部 addendum”。首例是 ADR-0007 (现有 7 个 addendum) 于 P0 收口时合并。

### B-3 MEMORY 只留稳定经验

- 现状：CONVENTIONS 第32/36行已禁止 MEMORY 记录运行状态，但四份 MEMORY (architect、developer、po、reviewer) 都停在 2026-11-22 的状态。
- 动作 (各角色 owner 执行，Reviewer 复核)：
  1. 删动态状态节：architect、developer、po 的“当前基线/当前交付状态/当前残余”；reviewer 的 §2、§3。
  2. 删刷新义务：architect 守则第 5 条、reviewer §7 第 4 条里“每轮刷新状态节”的句子。
  3. 保留角色边界、工作守则、易漂移点、决策位置索引。
  4. 各 MEMORY 头部加一句：“只收跨变更仍成立的稳定经验；运行状态见 review-tracking / ac-coverage”。
- 禁止：把当前进度跟进 MEMORY；建自动同步。

### B-4、B-5 角色 prompt 维护

| # | 文件 | 填补内容 |
|---|---|---|
| B-4a | `prompt/Lead.md` | 分解纪律自查：“每个工作包是否声明独占的不变量与分支？跨服务身份字段在 Plan 里是否有契约归属？” |
| B-4e | `prompt/Lead.md` | 内联，不另建文件】【Subagent Brief 模板】。对应宪法 Governance 2 的七要素：a Role 加开场自读；b Change_id、Tier、base、head；c 允许与禁止路径；d 输入契约只传 canonical 指针，不转述；e 验收标准指向 AC 行或 evidence-matrix；f 停止条件 (契约缺口停、无 owner 停、FAIL 停)；g 证据绑定精确 HEAD，附反妥协自检；h 产出落点：结论写进指定产物，brief 不承载结论；i 收尾时把稳定经验回写本角色 MEMORY，运行状态不写 |
| B-4c | `docs/reviewer/MEMORY.md` §4 红队题库 | 追加：“本 FAIL 是否有语义更简的等价方案？被砍掉的范围是否需要人类显式接受？” |
| B-4d | `prompt/Quality-Checker.md` | Plan 红队题库追加：“是否有任务同时承载两个不变量？是否有跨服务身份只在代码里存在、契约里不存在？” |
| B-5 | `prompt/Lead.md` 职责第 4 条 | delta audit 细化为逐角色核对表：各角色的 MEMORY 和产物是否已刷新 |

---

## Batch C: AC 台账 (Reviewer)

- 现状：`ac-coverage.tsv` 36 行全部 DEFERRED。2026-12-15 到期 6 条 (AC-22/28/29/30/35/37)，2026-12-31 到期 30 条 (AC-39..68)。到期未关闭会阻塞对应范围，且禁止批量续期 (CONVENTIONS §6.3)。
- C-1.1 (D-LC-4)：建议 12-08 前对 6 条 12-15 到期的 AC 逐条裁决：关闭、单条续期，或放弃并记录。
- C-1.2：现在可做。AC-39..68 的 `reason` 从模板句改为对应的 WP 编号；没有对应 WP 的就是孤儿债。

---

## 人类裁决清单

| ID | 裁决什么 | 建议 |
|---|---|---|
| D-LC-1 | Batch A 全部执行，含 A-2a 那句话 | 批准 |
| D-LC-2 | pair 线程退役，角色间交互经 Lead 路由 | 批准 |
| D-LC-3 | Batch B 何时执行 | 二选一：P0 收口时；或现在，与 Batch A 同次 (A 先合入，B 另起 change_id)。选现在需一并定两件事：B-1 的停止条件是否改为 P0 收口时追溯；ADR-0007 已超过 5 个 addendum，是设过渡期豁免还是这次就合并 |
| D-LC-4 | C-1.1 的处置日期 | 12-08 前 |
| D-LC-5 | Phase 0 sign-off 空缺如何处置 | Lead 核实后再定，不补签 |

---

## 执行顺序

现在做 A -> 12-08 前做 C-1.1 -> B (时机见 D-LC-3) -> C-1.2 随时。除 D-LC-3 外各项互不依赖。B-4e 无论 B-1 是否获批都执行。

---

## 不做的事

1. 不新增治理机制、脚本、CI、状态字段。
2. 不把运行状态跟进 MEMORY。
3. 不删除 pair 目录下的历史线程。
4. 不改 SPEC、ADR、宪法的契约内容；CONVENTIONS 只改协作流程节。
5. 不给 Lead 建 MEMORY。

---

## 未覆盖

- 行号基于本地快照。qc-reports 和 review-tracking 的后续条目可能已做了部分动作，执行前查重。
- B-3 的前提是临时 subagent 没有既定的 MEMORY 回写流程；若外部环境有，B-3 需修正。
- 三个 pair 目录的 README 未读，其过时描述随 B-1 处理。
- 纯文档变更，无测试。