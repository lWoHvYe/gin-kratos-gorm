
# 多 Agent 协作约定 v3 (执行细则)

> 本文件是 [`AGENTS.md`](../../AGENTS.md) 的执行细则，不改变角色边界；冲突时以 `AGENTS.md` 为准。
> 所有 Agent 会话开始必须读取 `AGENTS.md`、本文件和自己的角色 prompt。

---

## 0. 原则

1. **业务结论只活在 Spec，设计结论只活在 ADR**；线程只记录问题、裁决和指针，不得成为第二契约。
2. **机器只检查客观事实，人判断语义**。正则阀门不能证明断言充分、架构正确或可以发布。
3. **每次结论必须绑定范围**。线程 CLOSED、实现 PASS、代码合入和发布放行是四种不同状态。
4. **宁可少而可靠的门禁，不要多而虚假的安全感**。新增机器限制必须客观、稳定、低误判，并有明确维护者。

---

## 1. 角色、产物与所有权

| 角色/产物 | 路径 | 唯一作者 | 其他角色 |
|---|---|---|---|
| 业务规格 | `docs/specs/**` | Product Owner | 只读；要改就开线程 |
| 质量审查报告 | `docs/collab/qc-reports/**` | Quality Checker | 只读；不代替 PO/Architect 改文档 |
| 设计决策 | `docs/adr/**` | System Architect | 只读；要改就开线程 |
| 角色 prompt | `prompt/**` | System Architect | Reviewer 必须复核；角色边界变化需人类批准 |
| 协作约定 | `docs/collab/CONVENTIONS.md` | System Architect | Reviewer 必须复核；流程原则变化需人类批准 |
| 实现偏差 | `docs/implementation/**` | Developer | 只读；通过线程要求修改 |
| 代码与测试 | `internal/**`、`cmd/**`、`api/**`、`migrations/**` 等 | Developer | Reviewer 只审查，不直接改 |
| 协作线程 | `docs/collab/<pair>/threads/**` | 双方各自追加 | 不改对方已写内容 |
| AC 引用/延期台账 | `docs/collab/ac-baseline.txt`、`ac-coverage.tsv` | Reviewer | Developer 不得通过改台账规避阀门 |
| 角色记忆 | `docs/<role>/MEMORY.md` | 对应角色 | 只保留稳定经验和 canonical 链接 |

**禁止**直接编辑他人产物。需要修改时开线程，由所有者回写。

MEMORY 不复制当前线程状态、PASS、commit、expiry 或发布状态；这些动态事实只认 canonical 产物。

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

- Spec 未批准，不得开始 ADR；ADR 未批准，不得开始实现。
- QC 只报告遗漏、矛盾和异常分支，不代写 Spec/ADR。
- PO 和 Architect 的批准是业务与架构批准，不等于代码验收。
- 文档错别字、链接修复等非行为变更可走轻量路径，不伪造 Spec/ADR 流程。
- 阻塞线程只冻结其影响范围，不机械停止无关工作。

### 2.1 规范性契约与描述性文档

- 代码与已批准 Spec/ADR 冲突：停止相关交付，由所有者判断应修代码还是按流程修契约。
- 描述性文档与代码现状不一致：以代码事实为准并修正文档漂移。
- 不允许用“代码已经这样”静默废止 Spec，也不允许用过期文档描述冒充当前实现。

### 2.2 外部复审/审计输入

外部 Agent、顾问或自动审计的结论**只能作为问题提案**，不得直接成为契约或 AC 依据。

- 引用必须指向当前 HEAD 中存在的文件与行号；找不到的标 `[不确定]`，不得作为需求引用。
- 每条结论按所有者分流：业务口径回 PO，设计回 Architect，实现回 Developer，验收语义回 Reviewer。
- 分流前必须核对是否已被既有 Spec/ADR/INV 裁决；已裁决的只能按文档漂移处理。

---

## 3. 变更输入包与 integration owner

每个跨服务变更必须指定一名 **integration owner**，由现有服务 Owner 或 Architect 兼任，不新增角色。

Developer 开始实现前，必须能确定：

```text
change_id
base
spec_version / adr_revision
integration_owner
baseline_impact
affected_AC
allowed_paths
```

Reviewer 开始审查前再记录精确 `head`。审查期间 HEAD 变化，必须重新确认差异和受影响验证。

`baseline_impact` 只能写：

- `不涉及`：说明判断依据；或
- 列出受影响的既有链路、baseline invariant/AC 和对应回归证据。

无论哪种，都必须列出受影响的 **`INV-xx`**（基线 `docs/architecture-baseline.md` 与 Spec 中的业务不变量）；`不涉及` 也必须写“为什么不影响任何 INV”。金额、库存、权限、履约准入相关变更不得只写“不涉及”。

后续 SDD 不得只验证新增 AC 而忽略旧交付、取消、库存、退款等基线链路。

---

## 4. 线程协议

**路径**：`docs/collab/<pair>/threads/T-NNN.md`；**引用**：始终写 `pair/T-NNN`，不使用可能跨目录撞名的裸 `T-NNN`。

- 一问一线程；编号在所属 pair 内递增且永不复用。
- 状态：`OPEN -> ANSWERED -> CLOSED`。
- 文首 YAML 最少包含：`id / title / opened_by / opened_at / blocking / status / spec / adr / closed_by`。
- 正文写清：现象、契约引用、影响、选项、裁决、canonical 回写位置。
- 新业务规则必须回写 Spec，新设计规则必须回写 ADR；线程里的临时结论不是契约。

`CLOSED` 只表示本次沟通已回写且没有待回复事项，不代表代码通过或可以发布。

### 4.1 Reviewer 审查附加元数据

Developer⇄Reviewer 的审查线程还必须包含：

```text
base / head
spec_version / adr_revision
integration_owner
review_scope
baseline_impact
INV: INV-xx, INV-yy（或“不涉及 + 依据”）
affected_AC
verification
release_status
```

`INV:` 为必填：至少列出本范围可能影响的业务不变量；没有影响时写 `INV: 不涉及（依据）`。

---

## 5. 验收状态语义

| 状态                       | 唯一含义                                      | 不能推出                     |
| -------------------------- | --------------------------------------------- | ---------------------------- |
| `thread_status=CLOSED`     | 沟通闭环                                      | 实现正确或发布通过           |
| `review_verdict=PASS/FAIL` | 对指定 base/head、review_scope 的实现审查结论 | Spec 全量验收或 Release PASS |
| `merge_status=ALLOWED`     | 允许合入当前变更                              | 业务 AC 全部完成             |
| `release_status=PASS`      | 精确 HEAD 满足发布证据要求                    | 后续提交自动继承通过         |

- Reviewer 输出 PASS 时必须写明审查类型和范围；存在未处置的阻断 AC，不得写 Spec 全量验收 PASS。
- Release PASS 至少需要：精确 base/head、阻断项清零、延期项已处置、受影响基线非回归、要求的测试/环境证据。
- 非核心 Lint、日志风格等不得与资金、库存、权限、数据迁移阻断项同级。

---

## 6. AC、测试与延期

### 6.1 两种不同证据

- **AC-REF**：测试代码中存在 `TestAC<NN>` 或 `AC-<NN>` 引用。只证明可追溯。
- **AC-VERIFIED**：在指定 HEAD 和环境中实际执行，结果通过，且 Reviewer 核对了断言与 AC 语义。

AC-REF 不得命名为“覆盖率”或“验收率”。Go 测试需要执行结果；未运行、SKIP、FAIL 均不算 AC-VERIFIED。ADR 指定浏览、集群或故障注入的 AC，不能用一条 Go 注释替代。

### 6.2 测试按风险选择

- 所有行为变更必须有与风险相称的测试。
- 资金、库存、权限和跨服务恢复必须包含并发竞态或故障注入等重灾区验证。
- 不为数量建立 AC×环境笛卡尔积；优先覆盖支付×取消×库存失败、多组退款×金额序列等高价值组合。

### 6.3 延期

每条延期必须有 `owner / expiry / reason / thread / status`。延期是显式债务，不是通过证书；到期未关闭必须阻断对应范围，不得删行或批量续期规避。

---

## 7. 机器门禁：只保留少量客观检查

### 7.1 门禁边界

| 阀门            | 机器适合检查                                    | 机器不能证明                             |
| --------------- | ----------------------------------------------- | ---------------------------------------- |
| G-1 所有权/范围 | 受管路径、Role/Exception 是否存在、线程是否存在 | 跨提交 Role 并集能证明实际作者；架构合理 |
| G-2 契约版本    | Spec version、索引、变更记录                    | 业务规则正确                             |
| G-3 AC-REF      | AC 编号、测试引用、延期字段                     | 测试已执行、未 SKIP、断言充分            |
| G-4 到期        | owner/expiry/status、到期和续期记录             | 延期是否合理、残余风险是否可接受         |

默认不继续增加 G-5、G-6。新增门禁前必须回答：是否客观、是否低误判、是否有明确 owner、误伤成本是否低于收益。

**不交给机器强制判断**：架构优劣、断言深度、业务折中、commit scope 词表、每个 AC 的测试环境数量，以及是否允许发布。

### 7.2 当前执行器能力

当前 Makefile 提供的统一入口只有：

```bash
make gates
make gates-strict
make gates-test
```

当前 `scripts/collab_gates.py` 仍有以下限制：

- 只实现 G-1/G-2/G-3，尚未实现 G-4 和 `--release`。
- 仍输出 `AC coverage`，目标措辞 `AC-REF` 尚未落地。
- 默认范围为 `HEAD^..HEAD`，不能覆盖整个 PR 或 release range。
- G-1 将 range 内 Role 做并集，结果只能作为提示，不能单独作为发布所有权证明。
- Makefile 尚未透传 `RANGE/RELEASE`，CI 也未覆盖全部 docs/prompt 变化。

因此，**文档中的目标规则不得被描述成已经通过的机器能力**，但在可接受范围内。

---

## 8. Commit 与交付

- 推荐使用 Conventional Commit 和必要 trailer (如 `Spec/ADR/Thread/AC/Role`)，服务/模块 scope 保持简单。
- 不维护庞大的强制 scope 词表，不为满足形式机械拆分提交。
- Spec/ADR 的业务或设计变更必须独立评审批；不要把未批准契约与实现混在一起。
- 已批准契约的实现允许一个原子行为提交包含 proto、生成物、服务代码、迁移和对应测试。
- Developer 交付必须包含代码 Diff、实际测试日志、`【测试范围】`、`【未覆盖范围】` 和返工影响声明。
- Reviewer 对核心契约 FAIL 时必须引用具体 Spec/ADR 条款；架构缺陷标 `[ARCHITECTURAL_FLAW]`，不得诱导局部补丁。

---

## 9. 各角色最小动作

| 角色             | 开场必读                                 | 必须交付                                | 绝对不做                       |
| ---------------- | ---------------------------------------- | --------------------------------------- | ------------------------------ |
| Product Owner    | AGENTS、本文件、角色 prompt、相关线程    | 升级 Spec、EARS AC、State Ownership     | 写代码/ADR、在聊天里定规则     |
| Quality Checker  | AGENTS、本文件、待审 Spec 或 ADR         | QC-Report：遗漏、矛盾、异常分支         | 代写契约、写业务代码           |
| System Architect | AGENTS、本文件、已批准 Spec、QC 报告     | ADR/Amendment、状态机、恢复协议         | 写业务代码、擅自改 Spec        |
| Developer        | AGENTS、本文件、Spec+ADR、变更输入包     | 代码、测试日志、偏差与影响声明          | 私自发明规则、用 TODO 掩盖冲突 |
| Reviewer         | AGENTS、本文件、精确 base/head、Spec+ADR | 有范围的 verdict、AC/基线证据、延期台账 | 用 AC-REF 代替验收、直接改代码 |

通用启动语：

```text
先读 AGENTS.md、docs/collab/CONVENTIONS.md 和你的角色 prompt。
只修改自己拥有的产物；需要他人修改就开线程。
所有结论绑定精确 Spec/ADR 版本和代码范围，不把线程 CLOSED 或 AC-REF 当作发布通过。
```

---

## 10. 反模式清单

- QC 未审就批准 Spec/ADR，或人类批准后被静默改契约。
- 在聊天/线程里定业务规则，不回写 Spec/ADR。
- 用历史线程 PASS、默认 HEAD^..HEAD 或 AC 引用数作为 Release PASS。
- 删除/重编号 AC、删唯一测试、删 deferred 行、批量续期或修改阀门使其恒真。
- 让不同 Agent 在移动 HEAD 上各读各的“当前版本”。
- 用分布式锁、额外状态字段或场景专用 `if` 回避状态所有权和协议设计。
- 为 commit 形式机械拆分一个原子行为变更。
- 用更多文档状态或正则规则替代真实组合验证。
