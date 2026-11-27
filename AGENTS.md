# AGENTS.md

> Trial branch `reform/lean-core`（base `6ee05df`，宪法 v2.2.0 不变）。
> 本文件只改流程；不新增角色/批准点。合并前需 reviewer/human；指标不改善则丢弃本分支。

## 1. 谁做什么

| 角色 | 是谁 | 做 | 不做 |
|---|---|---|---|
| 作者 | 主会话 | 写 Spec、ADR、plan、tasks；实现；交证据；记录人类决定 | 审自己的东西；改写审查者裁决；合入 |
| 审查者 | 独立 subagent（有运行时；不能复跑时写明原因/未覆盖并取得人类书面豁免） | 审 Spec/ADR/代码；写裁决；更新验收状态表 | 改代码；改 Spec/ADR 正文（验收表除外）；给补丁建议 |
| 人类 | 人 | 批准 Spec；合入；Release；回答业务裁决题 | 逐条核对文字/语法 |

- 作者和审查者都先读宪法；本文件只写操作细节，不重复原则。
- 作者可用 subagent 做机械子任务；审查者始终独立。
- 修改宪法/本文件仍按宪法 Governance：作者起草、审查者审查、人类批准。

## 2. 流程

### 2.1 契约变更

| 步 | 动作 | 谁 | 产物 |
|---|---|---|---|
| 1 | 写 Spec：宪法附录 A；命中附录 B 写 ADR 并链接 | 作者 | Spec、ADR |
| 2 | 契约审查（任务书见 §3） | 审查者 | 台账 |
| 3 | 批准 Spec 和链接 ADR，回答业务裁决题 | 人类 | 台账 |
| 4 | 写 plan/tasks：`baseline_impact`、`stop_conditions`、AC→检查；实现按阶段分批，批后停下交证据 | 作者 | plan、tasks、代码、日志 |
| 5 | 代码审查（§3）；更新验收状态表 | 审查者 | 台账、Spec |
| 6 | 合入、Release | 人类 | 台账 |

- plan 不省略；金额、库存、权限、履约准入不得只写“不涉及”。
- 第 2 步 FAIL 回第 1 步；第 5 步 FAIL：代码不符合 Spec/ADR 回第 4 步，架构/设计问题或同一问题第二次不过且判为设计问题则冻结回 Spec/ADR，越界按契约变更重走。
- 线上紧急修复不设 hotpath：走最短 Spec/实现流程，当日取得人类批准；不得用 hotfix/新 change_id 绕过 FAIL。

### 2.2 实现变更

- 行为已被批准 Spec 覆盖、且不触及契约主题：走 2.1 的第 4–6 步；不新建 Spec/ADR。
- 无法归入或触及契约主题：按契约变更。
- **bugfix 不设特殊路由**：满足上述条件就是实现变更；否则是契约变更。用现有工具修，不新建 Spec。
- 同一时间只推进一个 change_id；并行需人类批准。
- 任何实现工具不得无人值守连续执行；`speckit-implement` 按阶段分批。

### 2.3 Legacy

- `docs/specs/0001/0002` 冻结：不迁移、不复制；需要修改时只做必要原地修改，迁移另案。
- 新工作默认 `specs/<feature>/spec.md`；0001/0002 保持 `docs/specs/`；不确定先问人类。

## 3. 审查者任务书

作者启动审查者时只用下面这段，不得增删。

```text
你是独立审查者。先读 .specify/memory/constitution.md，再读 Spec 和它链接的 ADR。
change_id: <编号>; 级别: 契约变更 | 实现变更 | 治理变更; Spec 和 ADR: <路径，治理变更写"无">
声称受影响的其他 Spec 的 AC: <编号，没有就写"无">
base: <sha>; head: <sha>; 阶段: Spec | 代码
只依据 Spec、ADR、宪法，以及 base/head 之间的代码、plan、测试日志判断，不依据作者自述；其他输入只作问题提案（宪法 12），不作证据。
输出:
1. 裁决 PASS / SCOPED-PASS / FAIL，写明范围。
2. 逐条问题，指向被违反的 AC/INV/ADR/必备章节/宪法条款；摘不到的记 [TODO]。
3. AC 位移：受影响 AC 的旧/新状态、检查类型、证据路径。
4. 能写台账就按 AGENTS §4 写入 docs/review-tracking.md 并更新 Spec 验收表；不能写就输出同格式条目。
5. 写明 review_scope、release_status、spec_version/adr_revision、verification、受影响 INV、baseline_impact；资金/库存/权限/跨服务/迁移必须逐条列 INV。
一次给全所有 blocking 问题；只追 blocker；作者修完后只复跑失败项+diff；不给补丁；发现设计缺口标 DESIGN-GAP，停代码回 Spec/ADR。
```

- Spec 阶段审：附录 A 是否都有落点、回答到位；是否违反 INV/ADR；AC 是否可达、落到测试/契约检查/可观测指标；跨服务 integration owner/baseline_impact；资金/库存/权限/跨服务必须写 INV。
- 代码阶段：代码是否符合 Spec/ADR；测试/证据是否支持 AC；是否使其他 Spec 已验证 AC 失效；高风险是否按宪法 13 有并发/故障证据。
- 审查期间 HEAD 变化要重新确认差异和受影响验证。
- `analyze`/`checklist` 只作 lint/自检，不构成 PASS；审查者不给补丁，不替作者设计。
- 超过 2 轮仍不收敛：由人类按“实现错误/证据不足/设计问题/契约缺口/环境”分类决定下一步，不自动放行；权限/安全/数据完整性/迁移/资金/库存阻断不受轮次影响。

## 4. 台账与提交

台账 `docs/review-tracking.md`，最新在上，只有一个插入位置；同日条目按插入位置判定，日期只分组。格式：

```text
### YYYY-MM-DD <审查裁决|人类决定|状态变化>: <change_id> - <一句话>
- Actor / Reviewed-by: ...; 级别: 契约变更 | 实现变更 | 治理变更; 阶段: Spec | 代码; base..head: ...
- Spec 和 ADR: <路径>
- 结论: PASS | SCOPED-PASS | FAIL | 批准 | 冻结 | Release; 范围: ...; release_status: NOT RELEASE | PASS
- 取代: <被取代条目的日期和标题> (没有就写"无")
- 阻断项: ...; 待办: ...
- AC 位移: <Spec> <AC 编号> <旧状态> -> <新状态> (证据路径)
```

提交尾注：`change_id:`、`Actor:`、`Reviewed-by:`。作者不合入；人类在合入提交补尾注。尾注必须真实；未审写 `Reviewed-by: pending`，不得冒充。

## 5. 交付要求

每次代码交付满足宪法 12–14，并带对应 AC：

- 证据：diff、测试日志（绑定 `git rev-parse HEAD`）、【测试范围】【未覆盖范围】【取舍】。
- 新 Spec 证据放 Spec 同目录 `evidence/`；在途变更可沿用 `docs/implementation/**`；治理变更放 `docs/collab/qc-reports/`。
- 台账写精确路径和 HEAD；AC 位移写路径。
- 高风险按宪法 13 附并发/故障注入；不适用须人类书面豁免。

缺一项，审查者按证据不足退回。

## 6. 工具（非规则）

- Spec Kit：契约变更可用 specify/clarify/plan/tasks/implement，implement 分批；checklist 是作者自检；converge 只参考。核心模板/脚本不改；扩展用 preset/override 官方机制。
- 按需参考（非规范，冲突以宪法/本文件为准）：`docs/checklists/author.md`、`docs/checklists/design.md`、`docs/checklists/reviewer.md`、`docs/guides/spec-writing.md`、`docs/guides/spec-kit.md`、`docs/guides/review-protocol.md`。
- 不引入新角色/新状态文件；不把非规范 guide 当验收依据。
