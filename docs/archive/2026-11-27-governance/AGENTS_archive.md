# AGENTS.md

## 1. 谁做什么

| 角色 | 是谁 | 做 | 不做 |
|---|---|---|---|
| 作者 | 主会话 | 写 Spec、ADR、plan、tasks；实现；交证据；记录人类决定和状态变化 | 审自己的东西；改写审查者的裁决；合入 |
| 审查者 | 独立的审查 subagent（有运行时，能复跑测试；不能复跑时必须在裁决里写明原因、未覆盖范围，并取得人类书面豁免，否则不构成代码阶段 PASS） | 审 Spec、ADR、代码；写裁决；更新验收状态表 | 改代码；改 Spec 和 ADR 正文 (验收状态表除外)；给补丁建议 |
| 人类 | 人 | 批准 Spec；合入；做 Release；回答业务裁决题 | 逐条核对文字和语法 |

- 作者和审查者都先读 `.specify/memory/constitution.md`。本文件只补充操作细节，规则本身不在这里重复。
- 作者可以用 subagent 做机械性的子任务，它们都算作者这一侧。审查者始终独立。
- 修改宪法或本文件，按宪法 Governance：作者起草，审查者审查，人类批准。

## 2. 流程

契约变更：

| 步 | 动作 | 谁 | 产物 |
|---|---|---|---|
| 1 | 写 Spec：宪法附录 A 的必备问题；用 Spec Kit 时可用其原生章节（User Scenarios/FR/SC/Assumptions），并加“附录 A 对照”；命中附录 B 的情形必须写 ADR，并在 Spec 里链接 | 作者 | Spec、ADR |
| 2 | 契约审查，任务书见 §3 | 审查者 | 台账 |
| 3 | 批准 Spec 和它链接的 ADR，回答业务裁决题 | 人类 | 台账 |
| 4 | 写 plan (本次的做法，含不需要 ADR 的局部设计) 和 tasks；列出遵守的 ADR；做一次自检 | 作者 | plan、tasks |
| 5 | 实现，按阶段分批，每批停下交证据 | 作者 | 代码、测试日志 |
| 6 | 代码审查，任务书见 §3；更新验收状态表 | 审查者 | 台账、Spec |
| 7 | 合入，在审查通过之后 | 人类 | 台账 |
| 8 | Release | 人类 | 台账 |

- 实现变更做第 4-8 步；plan 可以一页以内，但不省略。plan 必须写 `baseline_impact`（“不涉及”也要写依据）与 `stop_conditions`；金额、库存、权限、履约准入变更不得只写“不涉及”。
- 第 2 步 FAIL：回到第 1 步。第 2 步只审 Spec 和 ADR；plan 在第 6 步随代码一起审。
- 第 6 步 FAIL，分三种。代码不符合已批准的 Spec 或 ADR：回第 5 步，改代码。架构缺陷，或同一问题第二次不过且判定为设计问题：冻结，回第 1 步。越出了已批准的 Spec 的范围：按契约变更重新从第 1 步走。
- 任何实现工具都不得无人值守连续执行：一个阶段一批，批后停下。用 `speckit-implement` 时，按 tasks 阶段分批调用或在阶段边界主动中止；工具默认会连续跑完全部任务，不能作为豁免。
- 线上紧急修复不设专门路径：按契约变更或实现变更走最短的 Spec，当日取得人类批准后实施；不得用 hotfix 或新 change_id 绕过 FAIL（宪法 30）。
- 跨服务变更在写 plan 时指定一名 integration owner（由既有角色兼任，作者或人类指定，不新增角色），负责基线与组合验证；没有 owner 的跨服务契约变更不得进入第 5 步。
- 简单变更的 plan 可以很短，但不省略。tasks 里状态机拆分、并发控制排在前面，不得先做 CRUD。

## 3. 审查者的任务书

作者启动审查者时只用下面这段，不得增删内容。

```text
你是独立审查者。先读 .specify/memory/constitution.md，再读 Spec 和它链接的 ADR。
change_id: <编号>; 级别: 契约变更 | 实现变更 | 治理变更; Spec 和 ADR: <路径，治理变更写"无">
声称受影响的其他 Spec 的 AC: <编号，没有就写"无">
base: <sha>; head: <sha>; 阶段: Spec | 代码
只依据 Spec、它链接的 ADR、宪法，以及这个 base/head 之间的代码、plan 和测试日志判断，不依据作者的自述；审查者之外的其他输入只作问题提案（宪法 12），不作为证据。
输出:
1. 裁决 PASS / SCOPED-PASS / FAIL，写明范围。
2. 逐条问题，每条注明违反的 AC、领域不变量、已批准的 ADR、必备问题的要求或宪法条款；摘不到的记为 [TODO]。
3. AC 位移: 本次 Spec 和受影响的其他 Spec 里，哪些 AC 从什么状态移到什么状态，附检查类型和证据路径。
4. 能写台账，就把裁决按 AGENTS.md §4 的格式写入 docs/review-tracking.md，并更新 Spec 的验收状态表；不能写，就输出同格式的条目。
5. 写明 review_scope、release_status（NOT RELEASE / PASS；PASS 只表示该范围证据满足，不是 Release PASS）、`spec_version`/`adr_revision`、`verification`、受影响的 INV 与 `baseline_impact`；所有变更至少写 `INV: 不涉及（依据）` 或列可能影响的 INV；资金、库存、权限、跨服务、数据迁移必须逐条列 INV。审查者之外的其他输入按宪法 12 只作问题提案，不作为证据。
第 5 项的字段写入审查报告；台账至少记 review_scope 与 release_status。
```

- Spec 阶段审三件事：必备问题是否都有落点、回答是否到位；是否违反领域不变量或已批准的 ADR；每条 AC 是否可达，是否落到了具体的测试、契约检查或可观测指标上。跨服务变更还要核对 integration owner 与 baseline_impact；资金、库存、权限、跨服务裁决必须写 INV。
- 审查期间 HEAD 变化时，重新确认差异和受影响验证；作者的 plan 必须写 `baseline_impact`（“不涉及”也要写依据）与 `stop_conditions`。
- 代码阶段还要核对：这次改动有没有使其他 Spec 里已验证的 AC 失效。
- SCOPED-PASS: 范围内通过，写明未覆盖的 AC 或路径。
- 审查方式：按所有权、不变量和裁决门来审，先看状态机和并发，不按清单逐条挑边缘问题。
- 能运行测试就复跑；不能复跑时按 §1 的条件写明原因、未覆盖范围并取得人类书面豁免，否则不构成代码阶段 PASS。

## 4. 台账与提交

台账是 `docs/review-tracking.md`，最新在上，只有一个插入位置。历史台账尚未重排时，同日条目按插入位置（块顶为最新）判定，日期只用于分组；重排是独立动作，未完成不改变“最新在上”的语义。条目格式：

```text
### YYYY-MM-DD <审查裁决|人类决定|状态变化>: <change_id> - <一句话>
- Actor / Reviewed-by: ...; 级别: 契约变更 | 实现变更 | 治理变更; 阶段: Spec | 代码; base..head: ...
- Spec 和 ADR: <路径>
- 结论: PASS | SCOPED-PASS | FAIL | 批准 | 冻结 | Release; 范围: ...; release_status: NOT RELEASE | PASS (仅表示该范围证据满足，不是 Release)
- 取代: <被取代条目的日期和标题> (没有就写"无")
- 阻断项: ...; 待办: ...
- AC 位移: <Spec> <AC 编号> <旧状态> -> <新状态> (证据路径)
```

提交尾注：`change_id:`、`Actor:`、`Reviewed-by:`。作者不执行合入，由人类在合入提交上补这三个尾注。

## 5. 交付要求

作者每次代码交付必须满足宪法 12-14，并带对应的 AC 编号。

测试日志和审查报告：新 Spec 放 Spec 同目录的 `evidence/`（扁平 Spec 用 `docs/specs/evidence/<change_id>/`）；在途变更可沿用 `docs/implementation/**`；治理变更的审查报告放 `docs/collab/qc-reports/`。台账必须写精确路径和 `git rev-parse HEAD`。台账的"AC 位移"写它们的路径。

缺任何一项，审查者按证据不足退回，不进入审查。

## 6. 推荐工具

这一节不是规则，和上文冲突时以上文为准。

- 装了 Spec Kit 时，第 1、4 步可以用它的 specify、clarify、plan、tasks，第 5 步可以用 implement，但必须分批；checklist 默认是作者自检，若作为审查产物，`[x]` 表示审查者判定，不是实现完成；converge 的方法可供审查者参考；修宪可以用 constitution 辅助起草。`speckit-implement` 没有阶段参数，分批靠操作者在阶段边界主动中止/重启，不能依赖工具自动停。
- 官方脚手架里的核心模板和脚本不要直接改。要改 Spec 必备问题，走它的模板覆盖层：项目覆盖层 `.specify/templates/overrides/` 是整体替换；只追加章节时用预设/扩展层的 `append`，不要用 override 冒充 append。
```