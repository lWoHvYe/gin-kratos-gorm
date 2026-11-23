# gin-kratos-gorm Constitution（项目根本大法）

本文件是本仓库所有 AI 生成物（Spec, Plan, Tasks, Code）必须绝对遵守的最高准则。任何违背本宪法的设计或代码，将被直接拒绝合入。

## Ratification

- v1.2.0 经人类指挥官于 2026-11-24 批准；G1–G4 同日完成，状态 **EFFECTIVE**。
- 层级：**本宪法 > `AGENTS.md` > `docs/collab/CONVENTIONS.md`**；冲突时以本宪法为准。
- G1：AGENTS/CONVENTIONS 已对齐；G2：G-1 已校验 `change_id`/`Actor`/`Reviewed-by`；
  G3：`ac-coverage.tsv` 已含 `test_type/evidence_path/status`；G4：人类裁决 B 已记录在
  `docs/review-tracking.md`。
- 遗留：SPEC-0001 AC-27 在下一次版本升级改为 EARS；PAY-SETTLE-01 WIP 按裁决 B 冻结。

## Core Principles

### I. 架构与工程哲学 (Engineering Philosophy)

1. **拒绝"屎山雕花"（Refactor over Patch）**：严禁用散落的条件分支替代状态机设计。判定线：同一业务路径为修复同一类缺陷已存在 2 处及以上状态判断分支时，MUST 停止编码并提出状态机拆分或领域模型重构方案。修复 MUST NOT 通过新增 `if-else` 默认放行或掩盖冲突状态。
2. **防御性架构（Fail-Closed）**：涉及资金、库存、订单的核心链路，所有未预期的异常、超时、配置缺失必须直接报错阻断（Fail-Closed），或进入持久化补偿队列（Outbox/MQ）。严禁通过 `try-catch` 静默吞掉异常或默认放行。测试替身、开发默认值或 nil 依赖 MUST NOT 弱化生产守卫。资金/库存/履约准入的必需依赖（Inventory、Payment、Fulfillment）MUST 在构造期必填、服务启动期 fail-fast；可选能力（pricing/promotion/AI 等）MUST 用显式 Options 开关，禁止用 nil 表示关闭。测试替身只允许在 `_test.go` 或 build tag 中注入。
3. **契约优先（Contract First）**：修改任何跨服务 RPC 或事件之前，必须先定义 `proto` 或 JSON Schema，严禁在没有契约的情况下修改生产代码。
4. **拒绝自我妥协**：严禁为完成 Goal 而悄悄砍掉复杂逻辑、专挑简单路径或用 `try-catch` 吞掉异常。遇到极难处理的核心链路 MUST 停下出设计，严禁"假装完成"。进度不得以 round 数、commit 数或测试数量衡量；没有合规产出的轮次 MUST 停手，而不是为提交而提交。

### II. 事实所有权与状态机 (State Ownership & Facts)

1. **事实分离（Fact Separation）**：`PaymentCaptured`（支付成功）、`OrderSettled`（订单结算）、`InventoryReady`（库存就绪）、`FulfillmentAllowed`（允许履约）是四个独立的事实。严禁用一个统一的 `status` 或单一的 `order.paid` 事件来混合表达。唯一例外是 ADR 批准的**版本化兼容期**：灰度期 MAY 双发 `order.paid` 作 BFF/旧消费者兼容，但 `order.paid` MUST NOT 作为履约准入、结算或库存就绪的依据；收口后 MUST 停止其准入用途。兼容期 MUST 写明默认消费者切换、收口条件、移除日期与回滚路径；MUST NOT 以"flag 默认 false"长期保留旧准入路径。灰度影子期 MAY 保留旧默认，但 MUST 有 ADR 批准的切换步骤、owner 与收口日期；切换完成后的生产默认 MUST 消费 `OrderSettled` 或经 ADR 批准的等价事实。
2. **状态唯一所有权**：每个核心状态字段只能有一个微服务拥有写入权（如资金状态属于 Payment，履约状态属于 Fulfillment）。禁止跨服务直接修改其他服务的核心状态字段。
3. **幂等与最终一致性**：所有跨服务写操作（支付回调、TCC 分支、消息消费）必须实现强幂等。必须使用数据库唯一约束或状态机 CAS 作为兜底，严禁仅靠 Redis 缓存做幂等。
4. **渐进式兼容演进**：核心交易链路重构（状态机拆分、事件身份升级）MUST NOT 采用 Break Change 直接替换，MUST 采用双写、版本化事件或影子消费（Shadow Consumption）平滑过渡，并明确收口条件与回滚路径。自本宪法 1.2.0 生效起，`main` MUST NOT 新增半成品**协议路径**；未完成的协议实现 MUST 留在 WIP 分支。**契约完整/协议完整**指事件、消费者、迁移、幂等与回滚路径齐全；缺任一即半成品。Feature flag 可用于契约完整增量的灰度，但 MUST NOT 用于掩盖未实现的协议分支；flags 默认 false 不构成合入理由。既有 WIP 按人类裁决冻结或隔离，不得继续扩展。
5. **退款与履约准入**：普通售后、TCC 补偿与 SYSTEM 退款 MUST 进入同一组幂等准入协议，由 permit 裁决退款与发货竞态。IF 某履约组存在未完成退款且尚未授权发货，THEN THE SYSTEM SHALL 撤销或拒绝该组的发货 permit；IF 已离仓后发生退款，THEN THE SYSTEM SHALL 保留已发货事实并只退款，不得回滚履约。TCC 补偿与 SYSTEM 退款 MUST 逐组准入并满足活动组金额之和等于订单剩余可退金额（Σ 等式）；MUST NOT 用整单退款事件替代逐组准入。退款准入一旦作用于 permit，MUST NOT 由实现自行恢复 `REVOKED` permit；恢复 MUST 由版本化 SPEC/ADR 规定并经人类批准。准入前拒绝/取消 MUST NOT 创建 commitment；准入后执行失败 MUST 进入持久重试或人工队列，不得由实现静默释放。任何 commitment 释放 MUST 由版本化 SPEC/ADR 规定并经人类批准。
6. **读写隔离**：`skus.stock` 是投影，真实库存在 inventory；MUST 实现 Read-Your-Writes。禁止把跨服务异步投影读作准入事实。

### III. 微服务技术栈铁律 (Tech Stack Hard Limits)

1. **分布式事务（DTM & Outbox）**：
   - DTM Saga 分支必须走 `dtm.Branch`，业务写表与 `outbox.Append` 必须共用同一个 `*sql.Tx`。
   - `gid` 必须由业务键确定性派生（如 `order-create:<order_no>`），严禁使用随机 UUID。
   - 跨服务资金、库存写入必须有持久任务（worker）或工作流继续驱动；外部渠道重试或进程崩溃不得作为唯一恢复协议。
2. **数据库访问**：
   - 事务核心必须使用显式 `database/sql` 与条件 `UPDATE`（CAS）。
   - GORM 仅允许出现在 `catalog/gorm_repo.go` 与 `cart/gorm_repo.go` 中，且仅限读侧。
3. **多仓与物理边界**：未获 PRD 批准不得开启多仓行为。已批准的多仓操作、事件、恢复和 replay MUST 显式携带 `warehouse_id`，不得缺省默认 W1；需要租身份的履约事件不得缺省默认 G1。只有明确版本化的历史单仓协议可以默认 W1，且该例外 MUST 在代码中以显式 legacy 分支标注。
4. **静态守卫**：`internal/platform` 不得依赖 `internal/services`；跨服务与对外返回的错误 MUST 使用统一的 `apperr` 包（内部上下文包装可用 `fmt.Errorf("%w")`），并按 `docs/contracts.md` 的错误契约映射；请求路径不得 panic，启动期配置或依赖缺失 MUST fail-fast 并以非零状态码退出（`cmd/*/main.go` 的启动失败 panic 不视为违规）。
5. **技术栈准入**：MUST NOT 引入未批准的技术栈（Kafka、Temporal、新 ORM 等）；新增基础设施依赖 MUST 先经 ADR 与人类批准。

### IV. 测试与交付标准 (Testing & DoD)

1. **测试证据不可妥协**：口头 `done` 无效。交付物必须包含实际执行日志，并绑定精确 HEAD。AC 文本引用（AC-REF）不等于 AC-VERIFIED；每条**受影响 AC** MUST 映射到 `docs/collab/ac-coverage.tsv`（`test_type`、`evidence_path`、`status`，Owner 固定为 Reviewer）；只有真实执行日志才能标记 AC-VERIFIED。在 G-3/ledger 扩展落地前，以 Reviewer 人工矩阵为准，MUST NOT 宣称 AC-VERIFIED。
2. **重灾区强制抗压**：资金、库存、权限和跨服务恢复 MUST 提供**并发竞态测试**（如 Go `-race`）或**故障注入测试**（模拟下游超时/崩溃）的日志。不适用时 MUST 书面说明依据并经人类确认，严禁以"不适用"静默跳过。仅功能测试通过、仅 Mock 全绿，不构成抗压证据。
3. **Mock 数据真实性**：测试用例中的 Mock 数据必须符合真实业务边界（支付/退款金额 MUST > 0；零元单 `pay_cent=0` 是合法路径，需单独覆盖），且必须包含至少一个异常分支，严禁用假数据糊弄全绿。
4. **组合与发布隔离**：实现测试绿色（Implementation PASS）不等于 Spec 全量验收，更不等于 Release PASS。跨服务变更必须包含受影响基线的非回归证据。

### V. AI 子系统与安全约束 (AI Safety)

1. **可用性隔离**：`ai` 模块是可选项，其任何故障（超时、幻觉、RAG 构建失败）不得影响商城的浏览、下单和支付主链路。
2. **身份与权限防越权**：
   - AI 工具默认只读。写操作必须走"草案 → 人工确认 → 调用原业务 RPC"。
   - AI 身份必须来自服务端请求上下文，严禁信任模型生成的 `user_id`。
3. **RAG 生命周期**：RAG 必须走版本化可变索引与可恢复发布协议，严禁原地删除重建索引（Drop Index）。
4. **协议与网络边界**：AI MUST 走统一的 `Turn/Action` 协议。Mesh MUST 使用显式服务间 ALLOW 与 AI 精确只读 ALLOW；禁止以 broad ALLOW + 不完整 DENY 收敛权限，DENY 仅作第二道防线。

## 附加协议: 需求定义规范 (Specification Standard)

所有**新增或修订**的 Spec 与 AC MUST 强制使用 **EARS 语法** 穷举异常分支：

- `WHEN [触发条件] THE SYSTEM SHALL [系统行为]`
- `IF [异常场景] THEN THE SYSTEM SHALL [报错或补偿行为]`
- `WHILE [并发状态] THE SYSTEM SHALL [锁或隔离机制]`

本要求不追溯改写已批准并冻结的历史 Spec；历史 Spec 在下一次版本升级时 MUST 将受影响的 AC 改写为 EARS，并登记为基线迁移任务。新增或修订的 AC MUST NOT 以**行首** `Given/When/Then/And` 的 Gherkin 步骤规避 EARS；EARS 句内 MAY 使用 `AND/OR` 组合条件。对已批准 Spec 的任何 Amendment/修订同样 MUST 遵守本条。

## 开发工作流与门禁 (Development Workflow & Gates)

本宪法只定义不可谈判的结果约束；执行细则（角色矩阵、产物所有权、线程协议）以 `AGENTS.md` 与 `docs/collab/CONVENTIONS.md` 为准。优先级 MUST 为 **本宪法 > `AGENTS.md` > `CONVENTIONS.md`**；二者不得与本宪法冲突。`AGENTS.md` §4/§5 为领域铁律，§1/§2/§6 为角色、SDD 与 DoD 执行细则，均与本宪法并行有效：本宪法未显式覆盖处仍受其约束，不得以"宪法未禁止"为由放行。

1. **阶梯门禁**：业务想法 → PO 输出 Spec Draft → QC 审查 Spec → 人类批准 `SPEC_APPROVED` → Architect 输出 ADR Draft → QC 审查 ADR → 人类批准 `ADR_APPROVED` → Developer 实现 → Reviewer 按精确 base/head 审查 → integration owner 组织基线与组合验证 → 人类决定 `RELEASE_APPROVED`。
2. **批准不得越级**：Spec 未批准 MUST NOT 开始 ADR；ADR 未批准 MUST NOT 开始实现。PO/Architect 的批准是业务与架构批准，不构成代码验收。
3. **验收语义隔离**：线程 CLOSED、实现 PASS、代码合入、Release PASS 是四种不同状态，MUST NOT 互相替代；Reviewer 的 PASS 必须声明 `review_scope`。
4. **反证义务**：Developer 若认为 ADR 存在架构缺陷，MUST 停止编码并附上至少一个会引发崩溃或资损的具体时序推演或伪代码，MUST NOT 只提问题不提供证据。
5. **架构缺陷不得就地打补丁**：Reviewer 判定为架构缺陷时 MUST 标记 `[ARCHITECTURAL_FLAW]` 并要求打回 Architect，MUST NOT 允许 Developer 在当前分支硬改或添加局部锁等诱导性补丁。
6. **证据绑定**：交付必须附带实际执行日志与精确 HEAD；纯文档变更不得伪造测试声明；跨服务变更必须附受影响基线的非回归证据。
7. **人类裁决记录**：人类给出的业务 Trade-off 豁免 MUST 在文档中记录并指明影响范围，AI 无条件服从，但不得据此自行扩展到未裁决范围。
8. **Plan/Tasks 门禁**：涉及 P0、跨服务、资金、库存、权限或状态机拆分的变更 MUST 在实现前产出 Plan/Tasks 与 AC 证据矩阵。Plan owner 为 Lead；Architect 审技术可行性，QC 审异常分支与测试矩阵，Developer 提供可执行性与风险估算，Reviewer 确认可验证性；人类批准 Plan 后 MUST 才能写业务代码。Plan 未批准 MUST NOT 写业务代码。
9. **Stop-the-line**：Reviewer 对某 `change_id` 出具 FAIL 后，该变更 MUST 冻结并登记到 `docs/review-tracking.md` 的 change freeze 区（`change_id`、冻结时间、重审 Plan 链接）；MUST NOT 继续在该 `change_id` 提交受管代码（业务代码、契约、迁移），直到修复 Plan、契约裁决与重审完成。修复 MUST NOT 用新增 `if-else`、临时锁或"安全网"补丁替代协议修复。人类可批准**独立 `hotfix:` change_id** 处理紧急安全问题；hotfix 范围只限安全修复，触及原 `allowed_paths` 视为扩展原变更。FAIL 后的修复提交 MUST 挂重审 Plan。`change_id` MUST 出现在变更输入与提交元数据中。
10. **Actor 独立性**：每个交付 MUST 使用 commit trailer 记录 `change_id: <id>`、`Actor: <agent-id>`、`Reviewed-by: <agent-id|human>`；同一 actor MUST NOT 同时是作者与 Reviewer。Lead 的职责边界与治理草案例外见 `Governance / Actors & Independence` 第 1 条。

## Governance

### Actors & Independence (Lead + SubAgent)

1. **Lead 的职责边界**：Lead 负责拆分任务、选择 subagent、维护 Plan/Tasks 与证据索引、集成交付、向人类报告；MUST NOT 编写 Spec、ADR、业务代码、QC 报告、Reviewer 裁决或本宪法文本（人类明确指示的治理修订草案除外，且仍须人类批准并记录指示）。若没有合适的 owner，Lead MUST 停下并向人类请求角色，不得自任。
2. **SubAgent 的治理义务**：每个 subagent MUST 自行读取 `AGENTS.md`、本宪法、`docs/collab/CONVENTIONS.md` 与自身角色 prompt；MUST NOT 仅依赖 Lead 摘要。Brief MUST 包含 role、`change_id`、允许/禁止路径、输入契约、验收标准、停止条件与证据要求。
3. **Reviewer 独立性**：Reviewer MUST 与作者为不同 actor；报告 MUST 逐条核对本宪法 Article I–V 以及 Governance/Workflow 相关条款，写明 `review_scope`、精确 base/head、`release_status` 与 `Actor/Reviewed-by`。Reviewer 的 PASS 只在声明范围内有效；FAIL 触发 Stop-the-line。
4. **人类检查点**：Spec 批准、ADR 批准、Plan 批准、每个 `change_id` 的首次与重审 FAIL、merge、release 均须人类知会/裁决；Lead 与 subagent MUST NOT 代行。
5. **治理基线同步**：`AGENTS.md`、本宪法或 `CONVENTIONS.md` 变更后，Lead 与所有 subagent MUST 重新读取，并在 `docs/review-tracking.md`（或对应 `docs/collab/<pair>/threads/`）记录 delta audit（日期、版本、差异摘要）；MUST NOT 依赖旧摘要或"未看到新版本"作为免责理由。
6. **进度度量**：不得以 round 数、commit 数、测试数量作为合规或交付进度的替代；无合规产出的轮次 MUST 停手并报告阻断。

### Amendment & Compliance

1. **效力与优先级**：本宪法是仓库内所有 AI 生成物的最高准则，优先于 Spec、Plan、Tasks、ADR 与代码；冲突时本宪法胜出，冲突产物 MUST 被拒绝合入。
2. **修订程序**：修订 MUST 通过 `/speckit-constitution` 提出，经人类指挥官批准后生效；涉及资金、库存、履约或跨服务协议的原则修订，MUST 同时记录迁移方案或明确豁免范围。
3. **版本策略**：遵循语义化版本。MAJOR：删除或重定义不兼容的原则/治理条款；MINOR：新增原则或实质性扩展现有指导；PATCH：澄清、措辞、错别字等非语义修订。
4. **合规审查**：任何 Spec/ADR Review（QC 或 Reviewer）MUST 逐条核对本宪法 Article I–V 以及 Governance/Workflow 中与本次变更相关的条款；Reviewer 的 PASS 只在声明的 `review_scope` 内有效；机器阀门只校验客观事实，不构成语义批准。
5. **持续对齐**：本宪法与 `AGENTS.md`、`docs/collab/CONVENTIONS.md` MUST 保持同源表述；发现漂移时 MUST 修正文档。描述性文档（不含本宪法与已批准的 Spec/ADR）与代码冲突时以代码事实为准；本宪法或已批准 Spec/ADR 与代码冲突时，MUST 停止相关交付并交由对应所有者裁决，不得以代码现状静默废止契约。
6. **人类豁免**：人类指挥官可对本宪法条款给出有范围的 Trade-off 豁免。豁免 MUST 以书面形式记录于 `docs/` 并在受影响的 Spec/ADR/线程中引用，MUST 注明范围、期限与受影响不变量；豁免不修改本宪法文本，MUST NOT 由 AI 自行推断或扩大适用范围。无记录的豁免视为不存在。
7. **过渡条款**：本宪法 1.2.0 生效前已存在于 `main` 的 WIP（含 PAY-SETTLE-01）按人类裁决 B 处理：**隔离 = 保留在 `main`、冻结、不合并到 release、不发布**；其 open 项（当前 P-01 安全网、P-04、P-05、P-06）在冻结清单中列明，不得据此新增半成品协议路径，也不得把"已存在"作为继续扩展的理由。B 裁决 MUST 记录在 `docs/review-tracking.md` 的 human decisions 区并被本章引用。

**Version**: 1.2.0 | **Approved**: 2026-11-24 | **Effective**: 2026-11-24 | **Ratified**: 2026-11-24 | **Last Amended**: 2026-11-24

**Status**: EFFECTIVE.
