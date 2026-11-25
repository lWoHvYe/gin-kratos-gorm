
# 多 Agent 协作约定 v3 (执行细则)

> 本文件是 [`AGENTS.md`](../../AGENTS.md) 的执行细则，不改变角色边界。优先级：**宪法 > `AGENTS.md` > 本文件**；冲突时以宪法为准，其次以 `AGENTS.md` 为准。
> 所有 Agent 会话开始必须读取 `AGENTS.md`、本文件和自己的角色 prompt。

---

## 0. 原则

1. **业务结论只活在 Spec，设计结论只活在 ADR**；任何交互通道只记录问题、裁决和指针，不得成为第二契约。
2. **机器只检查客观事实，人判断语义**。正则阀门不能证明断言充分、架构正确或可以发布。
3. **每次结论必须绑定范围**。沟通闭环、实现 PASS、代码合入和发布放行是四种不同状态。
4. **宁可少而可靠的门禁，不要多而虚假的安全感**。新增机器限制必须客观、稳定、低误判，并有明确维护者。

---

## 1. 角色、产物与所有权

| 角色/产物 | 路径 | 唯一作者 | 其他角色 |
|---|---|---|---|
| Lead 编排/计划/证据索引 | `docs/lead/**` | Lead | 只读；不拥有 Spec/ADR/代码/QC/Reviewer 产物 |
| 业务规格 | `docs/specs/**` | Product Owner | 只读；提请 Lead 路由至 owner |
| 质量审查报告 | `docs/collab/qc-reports/**` | Quality Checker | 只读；不代替 PO/Architect 改文档 |
| 设计决策 | `docs/adr/**` | System Architect | 只读；提请 Lead 路由至 owner |
| 角色 prompt | `prompt/**` | System Architect | Reviewer 必须复核；角色边界变化需人类批准 |
| 协作约定 | `docs/collab/CONVENTIONS.md` | System Architect | Reviewer 必须复核；流程原则变化需人类批准 |
| 实现偏差 | `docs/implementation/**` | Developer | 只读；提请 Lead 路由至 owner |
| 代码与测试 | `internal/**`、`cmd/**`、`api/**`、`migrations/**` 等 | Developer | Reviewer 只审查，不直接改 |
| 协作线程 | `docs/collab/<pair>/threads/**` | 已退役 2026-11-25，只读存档 | 不得新开或删除；历史指针保留 |
| AC 引用/延期台账 | `docs/collab/ac-baseline.txt`、`ac-coverage.tsv` | Reviewer | Developer 不得通过改台账规避阀门 |
| 角色记忆 | `docs/<role>/MEMORY.md` | 对应角色 | 只保留稳定经验和 canonical 链接 |

**禁止**直接编辑他人产物。需要修改时提请 Lead 路由至 owner，由所有者回写。

MEMORY 不复制运行状态、PASS、commit、expiry 或发布状态；这些动态事实只认 canonical 产物。

---

## 2. SDD 主流程与批准点

```text
业务想法
 -> PO 输出 Spec Draft
 -> QC 审查 Spec
 -> 人类批准 SPEC_APPROVED
 -> Architect 输出 ADR Draft
 -> QC 审查 ADR
 -> 人类批准 ADR_APPROVED
 -> Developer 实现代码与测试
 -> Reviewer 审查指定 base/head
 -> integration owner 组织基线与组合验证
 -> 发布门禁/人类决定 RELEASE_APPROVED
```

> **路由**：L0/L1 按 §2.3 减免，不走下方完整阶梯；L2 走本流程。任何级别保留人类合入与 Release。

- Spec 未批准，不得开始 ADR；ADR 未批准，不得开始实现。
- QC 只报告遗漏、矛盾和异常分支，不代写 Spec/ADR。
- PO 和 Architect 的批准是业务与架构批准，不等于代码验收。
- 文档错别字、链接修复等非行为变更可走轻量路径，不伪造 Spec/ADR 流程。
- 阻塞项只冻结其影响范围，不机械停止无关工作。

### 2.1 规范性契约与描述性文档

- 代码与已批准 Spec/ADR 冲突：停止相关交付，由所有者判断应修代码还是按流程修契约。
- 描述性文档与代码现状不一致：以代码事实为准并修正文档漂移。
- 不允许用“代码已经这样”静默废止 Spec，也不允许用过期文档描述冒充当前实现。

### 2.2 外部复审/审计输入

外部 Agent、顾问或自动审计的结论**只能作为问题提案**，不得直接成为契约或 AC 依据。

- 引用必须指向当前 HEAD 中存在的文件与行号；找不到的标 `[不确定]`，不得作为需求引用。
- 每条结论按所有者分流：业务口径回 PO，设计回 Architect，实现回 Developer，验收语义回 Reviewer。
- 分流前必须核对是否已被既有 Spec/ADR/INV 裁决；已裁决的只能按文档漂移处理。

### 2.3 变更分级判定

变更分 L0/L1/L2 三级，判据与流程如下；级别随变更记录，Reviewer 逐条核对判据与证据。

| 级别 | 适用范围 | 流程 |
| --- | --- | --- |
| **L0** | 仅非规范性文本（文档/注释/格式）；不改变行为、契约、状态、权限或生成物语义 | 无 Spec/ADR/QC/Plan；运行相关测试；**人类合入** |
| **L1** | 行为变更且仅限单一服务：精确一个 `internal/services/<service>/` + `cmd/<service>/`，且不命中任一 L2 触发条 | 引用既有 Spec 条款（`Spec-Ref`）+ 测试日志 + Reviewer PASS；更新受影响的既有 AC 行（台账由 Reviewer 更新）；无新 Spec/ADR/QC/Plan、无分级台账；**人类合入** |
| **L2** | 命中任一 L2 触发条的变更 | 沿用既有完整阶梯，不减免任何既有批准点；治理文本类 L2 的轻路径见 `AGENTS.md` §2.7 |

**L1 审批 = Reviewer PASS + 人类合入**，取代已撤回的 GOV-COST-01 D-CR-2（QC+Reviewer 联签，从未实施）；见 `docs/review-tracking.md` 2026-11-25 tiering decision。

**L2 触发条（与宪法 Workflow 8 一致，任一命中即 L2）**：P0、跨服务（含既有事件的消费效应）、资金、库存、权限（任一类，含 gateway/BFF 鉴权）、状态机拆分，以及契约/事件/迁移变更；契约含既有 RPC/事件响应语义、错误码映射、事件 payload 语义与消费语义。L1 与任一触发条重叠即 L2：单服务 P0 修复、网关鉴权变更、既有事件消费行为变更均不得判 L1。`frontend/**`、`deploy/**`、共享包、`internal/platform/**` 恒为 L2。规范性 Spec/ADR 修改走修订流程（EARS + QC + 人类批准），不得判 L0。

**判据与防作弊（不可协商）**

1. **默认升级**：不能明确归入 L0/L1 的变更一律 L2；治理/工具链变更（`AGENTS.md`、本文件、宪法、CI、`scripts/**`、spec/ADR 结构）恒为 L2。
2. **禁止自报**：Lead 提议级别并引用命中判据；Reviewer 在 verdict 中确认；Developer 不得设定或更改级别。
3. **误判即 stop-the-line**：冻结该 `change_id`，按 L2 重做，并记入 `docs/review-tracking.md` 的 human decisions/frozen 条目；不得新开 `change_id` 或用 `hotfix:` 绕过；合入后发现误判 MUST 记录，并回退或取得追溯 L2 批准，不得默认保留。
4. **判据即法**：只有经人类批准的 `AGENTS.md`/本文件变更可以修改分级表；Agent 不得改动判据以改判任何工作。
5. **人类检查点保留**：任何级别均保留人类 Release；L0 与 L1 保留人类合入（宪法 Governance 4：merge 是人类检查点，Lead/subagent 不得代行）；L2 保留全部既有批准点。

**变更输入**（随 commit/PR 记录，不新增文件格式）：`change_id`、`Tier`、`Tier-Rationale`、`Spec-Ref`（须实际覆盖改动行为，否则 L2）、`allowed_paths`、base/head、`baseline_impact`/INV、测试证据、Reviewer verdict、人类合入批准人与日期；commit trailer 写 `Tier: L0|L1|L2`。

**范围守卫**：不恢复 `make gates`、不建立按级别的 AC 台账、不新增状态字段、不引入 QC delta 机制。L1 不写 EARS AC：其无新契约，证据即既有 Spec 条款 + 测试 + 受影响既有 AC 行的更新。

---

## 3. 变更输入包与 integration owner

每个跨服务变更必须指定一名 **integration owner**，由现有服务 Owner 或 Architect 兼任，不新增角色。

Developer 开始实现前，必须能确定：

```text
change_id
Tier / Tier-Rationale（L0/L1 按 §2.3；L1 另需 Spec-Ref）
base
spec_version / adr_revision
integration_owner
baseline_impact
affected_AC
allowed_paths
forbidden_paths
stop_conditions
evidence_and_output
```

派单 Brief 是本节 change input 的**视图**：MUST 由上述字段生成，另补 `assigned_role`（角色 + 开场自读要求）；只传 canonical 指针，不承载结论；chat 回传仅为摘要，完整结论写进对应产物。Brief 字段以本节为准，其他文档不得另立 schema。

Reviewer 开始审查前再记录精确 `head`。审查期间 HEAD 变化，必须重新确认差异和受影响验证。

L0/L1 的 `Tier`/`Tier-Rationale`/`Spec-Ref` 按 §2.3 记录；L2 另需 `spec_version`/`adr_revision`。

`baseline_impact` 只能写：

- `不涉及`：说明判断依据；或
- 列出受影响的既有链路、baseline invariant/AC 和对应回归证据。

无论哪种，都必须列出受影响的 **`INV-xx`**（基线 `docs/architecture-baseline.md` 与 Spec 中的业务不变量）；`不涉及` 也必须写“为什么不影响任何 INV”。金额、库存、权限、履约准入相关变更不得只写“不涉及”。

后续 SDD 不得只验证新增 AC 而忽略旧交付、取消、库存、退款等基线链路。

---

## 4. 线程协议（已退役 2026-11-25，历史说明）

> **历史说明**：pair 线程协议已于 2026-11-25 退役（D-LC-2）；角色间交互改为经 Lead 路由（`AGENTS.md` §1.0），不再新开线程。原四项职能由 `docs/review-tracking.md`、Lead Plan/Tasks、REV 报告与 `ac-coverage.tsv` 承接。
>
> 三个 pair 目录 `docs/collab/{po-architect,architect-developer,developer-reviewer}/threads/` 保留为存档，**不得删除**；`ac-coverage.tsv` 历史 36 行的 `thread` 列仍指向它们、不回改，新延期行改为 `review-tracking/<change_id>`（见 §6.3）。
>
> 原协议摘要（只作历史参考，不具备现行效力）：路径 `docs/collab/<pair>/threads/T-NNN.md`；一问一线程、编号递增不复用；状态 `OPEN -> ANSWERED -> CLOSED`；文首 YAML 记 `id / title / opened_by / opened_at / blocking / status / spec / adr / closed_by`；结论必须回写 Spec/ADR。

### 4.1 Reviewer 审查附加元数据（仍适用；线程本身已存档）

Reviewer 的审查报告（含历史审查线程）必须包含：

```text
change_id / Tier / base / head
spec_version / adr_revision
integration_owner
review_scope
baseline_impact
INV: INV-xx, INV-yy（或“不涉及 + 依据”）
affected_AC
verification
release_status
Actor / Reviewed-by
```

`INV:` 为必填：至少列出本范围可能影响的业务不变量；没有影响时写 `INV: 不涉及（依据）`。

---

## 5. 验收状态语义

| 状态                       | 唯一含义                                      | 不能推出                     |
| -------------------------- | --------------------------------------------- | ---------------------------- |
| `thread_status=CLOSED`（历史通道） | 沟通闭环（线程 2026-11-25 退役，仅存档） | 实现正确或发布通过 |
| `review_verdict=PASS/FAIL` | 对指定 base/head、review_scope 的实现审查结论 | Spec 全量验收或 Release PASS |
| `merge_status=ALLOWED`     | 人类批准合入当前变更（分级不改变人类合入）    | 业务 AC 全部完成             |
| `release_status=PASS`      | 精确 HEAD 满足发布证据要求                    | 后续提交自动继承通过         |

- Reviewer 输出 PASS 时必须写明审查类型和范围；存在未处置的阻断 AC，不得写 Spec 全量验收 PASS。
- Release PASS 至少需要：精确 base/head、阻断项清零、延期项已处置、受影响基线非回归、要求的测试/环境证据。
- 非核心 Lint、日志风格等不得与资金、库存、权限、数据迁移阻断项同级。

---

## 6. AC、测试与延期

### 6.1 两种不同证据

- **AC-REF**：测试代码中存在 `TestAC<NN>` 或 `AC-<NN>` 引用。只证明可追溯。
- **AC-VERIFIED**：在指定 HEAD 和环境中实际执行，结果通过，且 Reviewer 核对了断言与 AC 语义。
- **唯一台账**：`docs/collab/ac-coverage.tsv`，Owner 固定为 Reviewer；列为 `ac / owner / expiry / reason / thread / test_type / evidence_path / status`。Lead 只维护 Plan/Tasks 与链接，不拥有 AC 状态。新延期行的 `thread` 写 `review-tracking/<change_id>`；历史 36 行保留原 `pair/T-NNN` 指针，不回改。

AC-REF 不得命名为“覆盖率”或“验收率”。Go 测试需要执行结果；未运行、SKIP、FAIL 均不算 AC-VERIFIED。ADR 指定浏览、集群或故障注入的 AC，不能用一条 Go 注释替代。

### 6.2 测试按风险选择

- 所有行为变更必须有与风险相称的测试。
- 资金、库存、权限和跨服务恢复必须包含并发竞态或故障注入等重灾区验证。
- 不为数量建立 AC×环境笛卡尔积；优先覆盖支付×取消×库存失败、多组退款×金额序列等高价值组合。

### 6.3 延期

每条延期必须有 `owner / expiry / reason / thread / status`；`thread` 写 `review-tracking/<change_id>`（历史行保留原 pair 指针）。延期是显式债务，不是通过证书；到期未关闭必须阻断对应范围，不得删行或批量续期规避。

---

## 7. 门禁（已退役 2026-11-25，仅保留人工检查清单）

> **Retired 2026-11-25**：协作闸门工具链已整体退役，由 Lead 删除 `scripts/collab_gates.py`、`scripts/check-collab-gates.sh`、`scripts/test-collab-gates.sh`、Makefile `gates`/`gates-strict`/`gates-test` 目标与 CI `collab-gates` job；`make verify` 不再运行 `gates`，**没有替代工具**，也不恢复脚本/CI/Makefile 集成。§7 以下内容只作**人工检查清单与判断原则**：不得再指示任何 agent 运行已删除的命令，也不得把某次提交描述为“已通过 G-1…G-4 机器门禁”。人工审查与常规 test/build 门禁不变。

### 7.1 门禁边界（人工审查清单）

下表是 **Reviewer 人工核对清单**（G-1…G-4 退役后仅存的形态），“原机器检查范围”列只记录历史，不代表现在有可执行检查：

| 阀门            | 人工核对要点（原机器检查范围）                  | 不能证明                                 |
| --------------- | ----------------------------------------------- | ---------------------------------------- |
| G-1 所有权/范围 | 受管路径、Role/Exception 是否存在、线程是否存在、HEAD 的 `change_id`/`Actor`/`Reviewed-by` 是否存在且 `Actor != Reviewed-by` | 跨提交 Role 并集能证明实际作者；架构合理 |
| G-2 契约版本    | Spec version、索引、变更记录                    | 业务规则正确                             |
| G-3 AC-REF      | AC 编号、测试引用、延期字段                     | 测试已执行、未 SKIP、断言充分            |
| G-4 到期        | owner/expiry/status、到期和续期记录             | 延期是否合理、残余风险是否可接受         |

不再新增 G 系列检查；G-1…G-4 仅作人工清单，无脚本实现。新增检查前必须回答：是否客观、是否低误判、是否有明确 owner、误伤成本是否低于收益。

**不交给机器强制判断**：架构优劣、断言深度、业务折中、commit scope 词表、每个 AC 的测试环境数量，以及是否允许发布。

### 7.2 执行器能力（历史记录，retired 2026-11-25）

原 `scripts/collab_gates.py`、`scripts/check-collab-gates.sh`、`scripts/test-collab-gates.sh` 及 `make gates` / `make gates-strict` / `make gates-test` 随整体退役删除，`make verify` 已不含 `gates`，CI 已无 `collab-gates` job。以下限制清单仅为历史记录，**不再要求任何人运行或修复**：只实现 G-1/G-2/G-3 而无 G-4/`--release`；输出 `AC coverage` 而非 `AC-REF`；默认范围 `HEAD^..HEAD`；G-1 的 Role 并集只能提示；Makefile 未透传 `RANGE/RELEASE`、CI 未覆盖全部 docs/prompt 变化。

退役后仍成立的只有 §7 的口径要求：**文档中的目标规则不得被描述成已经通过的机器能力**。

---

## 8. Commit 与交付

- 推荐使用 Conventional Commit 和必要 trailer；受管交付 MUST 包含 `change_id`、`Actor`、`Reviewed-by`、`Tier`、`Role`（人工复核用；协作闸门已退役），服务/模块 scope 保持简单。
- 不维护庞大的强制 scope 词表，不为满足形式机械拆分提交。
- Spec/ADR 的业务或设计变更必须独立评审批；不要把未批准契约与实现混在一起。
- 已批准契约的实现允许一个原子行为提交包含 proto、生成物、服务代码、迁移和对应测试。
- Developer 交付必须包含代码 Diff、实际测试日志、`【测试范围】`、`【未覆盖范围】` 和返工影响声明。
- Reviewer 对核心契约 FAIL 时必须引用具体 Spec/ADR 条款；架构缺陷标 `[ARCHITECTURAL_FLAW]`，不得诱导局部补丁。

---

## 9. 各角色最小动作

| 角色             | 开场必读                                 | 必须交付                                | 绝对不做                       |
| ---------------- | ---------------------------------------- | --------------------------------------- | ------------------------------ |
| Product Owner    | AGENTS、本文件、角色 prompt、相关 canonical 产物 | 升级 Spec、EARS AC、State Ownership     | 写代码/ADR、在聊天里定规则     |
| Quality Checker  | AGENTS、本文件、待审 Spec/ADR/Plan       | QC-Report：遗漏、矛盾、异常分支；L2 Plan 测试矩阵 | 代写契约、写业务代码 |
| System Architect | AGENTS、本文件、已批准 Spec、QC 报告     | ADR/Amendment、状态机、恢复协议         | 写业务代码、擅自改 Spec        |
| Developer        | AGENTS、本文件、Spec+ADR、变更输入包     | 代码、测试日志、偏差与影响声明          | 私自发明规则、用 TODO 掩盖冲突 |
| Reviewer         | AGENTS、本文件、精确 base/head、Spec+ADR | 有范围的 verdict、AC/基线证据、延期台账 | 用 AC-REF 代替验收、直接改代码 |
| Lead             | 宪法、AGENTS、本文件、角色 prompt | Plan/Tasks、证据索引、集成报告 | 写 Spec/ADR/代码/QC/Reviewer 裁决 |

通用启动语：

```text
先读 AGENTS.md、docs/collab/CONVENTIONS.md 和你的角色 prompt。
只修改自己拥有的产物；需要他人修改就提请 Lead 路由至 owner。
所有结论绑定精确 Spec/ADR 版本和代码范围，不把历史线程 CLOSED 或 AC-REF 当作发布通过。
```

---

## 10. 反模式清单

- QC 未审就批准 Spec/ADR，或人类批准后被静默改契约。
- 在聊天/历史线程里定业务规则，不回写 Spec/ADR。
- 用历史线程 PASS、AC 引用数或未绑定精确 base/head 的结论作为 Release PASS。
- 删除/重编号 AC、删唯一测试、删 deferred 行、批量续期或修改阀门使其恒真。
- 让不同 Agent 在移动 HEAD 上各读各的“当前版本”。
- 用分布式锁、额外状态字段或场景专用 `if` 回避状态所有权和协议设计。
- 为 commit 形式机械拆分一个原子行为变更。
- 用更多文档状态或正则规则替代真实组合验证。
