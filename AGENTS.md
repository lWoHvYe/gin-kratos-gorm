# AGENTS.md

本文件是本仓库多 Agent 协作的执行准则，每次会话自动加载。它位于 `.specify/memory/constitution.md` 之下；优先级为 **宪法 > 本文件 > `docs/collab/CONVENTIONS.md`**，冲突时以宪法为准。这里只保留**宏观协同战略、工程哲学、角色边界、SDD 规范与硬性技术约束**；架构细节与接口契约请以 `docs/` 目录为准。

---

## 0. 核心工程哲学与最高准则 (全员适用)

**本系统中的所有 AI Agent 皆为资深研发，必须摒弃“实习生心态”与“趋易避难本能”，绝对服从以下宏观准则：**

1. **拒绝“屎山雕花”**：发现同一业务逻辑散落大量 `if-else`、状态判断或防御性补丁时，**严禁继续添加条件**，必须立刻停止编码，向人类汇报并提议重构。
2. **拒绝“自我妥协”**：严禁为了完成 Goal，悄悄砍掉复杂逻辑、挑软柿子捏或用 `try-catch` 吞掉异常。遇到极难处理的核心链路，必须停下来出设计，绝不允许“假装完成”。
3. **防御性架构 (Fail-Closed)**：在资金、库存链路中，所有未预期的异常、超时、配置缺失，必须直接报错阻断或进入持久化补偿，绝不允许静默放行。
4. **人类指挥官绝对裁决权 (Supreme Command)**：人类只负责“业务裁决”与“架构方向审批”，严禁把人类当成“人肉语法检查器”。但当人类介入并给出业务 Trade-off 决策时（如：“这次先不管极端的网络脑裂”），AI 必须无条件服从并在文档中记录该豁免。
5. **上下文最小化与隔离原则 (Context Isolation)**：严禁跨角色传递非结构化的聊天记录。所有跨角色信息必须通过精简、版本化的交付物 (`Spec`/`ADR`/`QC Report`/代码/测试报告) 交接；线程只承载结构化问题、裁决和 canonical 指针。下游角色只需要最终契约与明确范围。

---

## 1. 多 Agent 角色矩阵与强制契约

**严禁“既当选手又当裁判”，严禁跨角色越权。** 角色之间必须通过 Artifacts (交付物) 交接。

### 1.0 Senior Agent Lead (Lead) -> 编排、路由与集成
* **职责**：拆分任务、选择 subagent、维护 Plan/Tasks 与证据索引、集成交付、向人类报告；确保每个交付有 owner、change_id、精确 base/head 与证据。
* **防守红线**：
    * 不得代替 PO/Architect/Developer/Reviewer 作者化 Spec、ADR、业务代码、QC 报告或 Reviewer 裁决；治理修订草案仅在人类明确指示下起草，且须人类批准。
    * 必须读取宪法、本文件、CONVENTIONS 与角色 prompt；治理文件变更后必须做 delta audit，不得依赖旧摘要。
    * Reviewer/QC FAIL 即 stop-the-line：冻结该 change_id，不得继续提交受管代码；不得用“加个锁/加个 if”的安全网替代协议修复。
    * 同一时间只推进一个 change_id；不得以 round/commit 数量作为进度。

### 1.1 Product Owner (产品经理) -> `Spec.md`
* **职责**：定义业务边界、验收标准 (AC) 与业务不变量。
* **防守红线**：
    * 不得指定、修改或越权审批技术实现；可以参与业务 Trade-off，但禁止因“技术上难实现”而擅自简化业务规则。
    * **EARS 语法强制约束**：验收标准必须使用 `WHEN/IF/WHILE ... THE SYSTEM SHALL ...` 语法 (见附录 A)，逼迫穷举异常分支。
    * **State Ownership**：明确每个核心状态字段的唯一事实来源，禁止把多个状态揉在一个字段里。

### 1.2 Quality Checker (质量质检员) -> `QC-Report.md`
* **职责**：在 `Spec.md` 和 `ADR.md` 交给下一环节前，进行“红队破坏性审查”。
* **防守红线**：
    * 严禁替 PO/Architect 写文档，只输出漏洞报告。
    * 必须检查：状态机是否有死胡同？并发/超时/崩溃等异常分支是否遗漏？幂等机制是否声明？上下文是否越权（比如 PO 偷写了技术栈要求）？

### 1.3 System Architect (架构师) -> `ADR.md`
* **职责**：接收 QC 通过且经人类批准的 `Spec.md`，输出领域模型、API 契约、状态机图、异常恢复协议；跨服务变更必须明确 integration owner 与 baseline impact。
* **防守红线**：
    * 绝对禁止写业务代码。涉及跨服务、资金或库存时，必须明确“并发冲突、部分成功、进程崩溃、网络脑裂”时的最终一致性保障 (TCC/Outbox) 与持久恢复所有者。
    * **强制假设声明**：必须专列一节 `【假设与不确定性】`，列出所有底层依赖风险。
    * 对 Developer 拥有技术一票否决权。

### 1.4 Developer (执行工程师) -> `Code & Test Logs`
* **职责**：严格依照 `Spec.md` 和 `ADR.md` 完成实现；局部技术实现可以自主，业务规则、跨服务协议和状态所有权不得自行变更。
* **防守红线**：
    * **契约边界与反证义务**：遇到影响业务不变量、跨服务协议、状态所有权或未定义异常语义的问题，必须停下来提问；**如果你认为 ADR 有严重架构缺陷，必须附上至少一个会导致系统崩溃/资损的具体时序推演或伪代码来反证，禁止只提问题不提供证据。** 
    * **先做难而正确的事**：任务清单必须优先处理状态机拆分、并发控制，严禁先做 CRUD 苟且。
    * **强制提交【反妥协自检清单】**：并发超时处理了吗？有没有绕过架构加 `if-else`？向现实做了哪些妥协（重大妥协需高亮）？

### 1.5 Reviewer (无情审查员) -> 有范围的 PASS / FAIL 裁决
* **职责**：拿着契约核对精确 base/head 的代码与测试日志，写明 review_scope；实现 PASS 不等于 Spec 全量验收或 Release PASS。
* **防守红线**：
    * **拒绝微观完美主义**：不影响本次契约与发布风险的边缘规范（如 Lint）只能标记 `[TODO]`；**但权限、安全、数据完整性和迁移阻断不得因未直接造成资损而降级。**
    * **禁止提供“引诱性建议”**：严禁建议“加个本地锁”这种局部补丁。必须指明违反了哪条不变量。**如果发现是架构缺陷，必须标记 `[ARCHITECTURAL_FLAW]`，并明确要求打回给 Architect，严禁允许 Dev 在当前分支硬改。**
    * **Actor 独立性**：Reviewer MUST 与作者为不同 actor；报告必须记录 `Actor`/`Reviewed-by`、精确 base/head、`review_scope`、`release_status`，并逐条核对宪法 Article I–V 与 Governance/Workflow 相关条款。
    * **FAIL 冻结**：FAIL 后不得继续在同一 `change_id` 提交受管代码；修复必须挂重审 Plan；不得用新 `change_id` 或 `hotfix:` 绕过原范围。

---

## 2. SDD (规格驱动开发) 实战规范

1. **分析行为准则 (强制证据链)**：所有结论必须给出 `文件:行号` 与 实际值；禁止“大概/应该/可能”。找不到数据标 `[不确定]`。
2. **返工与修改法则**：
    * Reviewer 提出 FAIL 时，Dev **绝不能**为了消除报错而打补丁。
    * 如果判定为“架构缺陷”，Dev 必须停止写代码，要求打回给 Architect 修改 ADR。
3. **阶梯门禁**：Spec 与 ADR 均须经过 QC 和人类批准后才能进入下游；具体流程以 `docs/collab/CONVENTIONS.md` 为准。
4. **验收语义**：AC 文本引用不等于 AC 已验收；线程 CLOSED、实现 PASS、代码合入与 Release PASS 不得混用。跨服务变更必须验证受影响的既有基线，不得只验证新增 AC。
5. **Plan/Tasks 与 Stop-the-line**：P0/跨服务/资金/库存/权限/状态机变更必须先有 Plan/Tasks 与 AC 证据矩阵；Plan 未批准不得写业务代码。QC/Reviewer FAIL 后冻结 `change_id`，直到修复 Plan、契约裁决与重审完成。
6. **Actor/change_id**：受管交付 MUST 使用 `change_id`、`Actor`、`Reviewed-by` trailer；同一 `change_id` 的 `Actor` 不得等于 `Reviewed-by`。
7. **变更分级 (L0/L1/L2)**：判据与流程以 `docs/collab/CONVENTIONS.md` §2.3 为准；无法明确归入 L0/L1 的一律 L2，命中任一 L2 触发条即 L2。
    * **L0**：仅非规范性文本（文档/注释/格式）；免 Spec/ADR/QC/Plan，跑相关测试；仍须**人类合入**。规范性 Spec/ADR 修改走修订流程（EARS + QC + 人类批准），不是 L0。
    * **L1**：行为变更仅限单一服务（精确一个 `internal/services/<service>/` + `cmd/<service>/`），且不命中 L2 触发条（P0、跨服务含既有事件消费效应、资金、库存、权限含 gateway/BFF 鉴权、状态机拆分、契约/事件/迁移、`api/proto/**`、`migrations/**`、`internal/platform/**`、`frontend/**`、`deploy/**`、共享包）；引用既有 Spec 条款 + 测试日志 + Reviewer PASS，更新受影响既有 AC 行，无新 Spec/ADR/QC/Plan，**人类合入**。
    * **L2**：命中上述任一触发条的变更；沿用既有完整阶梯，不减免任何批准点。
    * **防作弊**：治理/工具链变更（本文件、`CONVENTIONS.md`、宪法、CI、`scripts/**`、spec/ADR 结构）恒为 L2；Lead 提议级别并引用命中判据，Reviewer 在 verdict 中确认，Developer 不得设定或更改级别；误判即 stop-the-line，冻结 `change_id` 并按 L2 重做、记入 `docs/review-tracking.md` 的 human decisions/frozen 条目；合入后发现误判 MUST 记录并回退或取得追溯 L2 批准；分级表仅人类批准可改，任何级别保留人类 Release，L2 保留全部既有批准点。
    * **变更输入**：`change_id`、`Tier`、`Tier-Rationale`、`Spec-Ref`（须实际覆盖改动行为，否则 L2）、`allowed_paths`、base/head、`baseline_impact`/INV、测试证据、Reviewer verdict、人类合入批准人与日期；commit trailer 写 `Tier: L0|L1|L2`。

---

## 3. 项目上下文与技术栈

* **技术栈**：Go 1.27 微服务商城。Kratos v3 gRPC + Gin BFF，MySQL 事务核心 `database/sql`，GORM 只做读侧，NATS JetStream transactional outbox，DTM saga/TCC，Redis，K8s + Istio Ambient。
* **布局**：`cmd/`、`internal/services/`、`internal/platform/`、`api/proto/mall/v1`、`api/gen/`、`migrations/`、`frontend/`、`deploy/`。

---

## 4. 领域铁律与全局分布式共识 (违反即错)

* **全局幂等性**：所有跨服务写操作 (支付回调、TCC 分支、消息消费) 必须实现强幂等。必须使用数据库唯一约束或状态机 CAS 作为兜底，严禁仅靠 Redis 缓存做幂等。
* **超时与重试**：跨服务调用必须设置合理的 Deadline/Timeout，严禁在无退避 (Backoff) 策略的情况下进行无脑重试；重试必须评估下游的幂等能力。
* **分布式事务与持久恢复**：DTM Saga 必须走 `dtm.Branch`，业务写与 `outbox.Append` 共用 `*sql.Tx`，`gid` 必须确定性派生。跨服务资金、库存和履约写入必须有持久任务或工作流继续驱动；外部渠道重试、消息恰好重放或进程崩溃不得作为唯一恢复协议。具体订单取消协议以架构基线/ADR 为准。
* **事实所有权与履约准入**：`PaymentCaptured`、`OrderSettled`、`InventoryReady`、`FulfillmentAllowed` 是不同事实，不得由单一状态或 `order.paid` 事件合并替代。普通售后、TCC 补偿和 SYSTEM 退款必须进入同一组幂等准入协议，由 permit 裁决退款与发货竞态。
* **渐进式兼容演进**：核心交易链路的重构（如状态机拆分、事件身份升级）严禁采用 Break Change 直接替换。必须采用双写、版本化事件或影子消费（Shadow Consumption）进行平滑过渡。 
* **读写隔离**：`skus.stock` 是投影，真实库存在 inventory。必须实现 Read-Your-Writes。
* **多仓身份与边界**：未获 PRD 批准不得开启多仓行为；已批准的多仓操作、事件、恢复和 replay 必须显式携带 `warehouse_id`，不得缺省默认 W1。需要租身份的履约事件不得缺省默认 G1；只有明确版本化的历史单仓协议可以默认 W1。
* **静态守卫**：跨服务禁走包引用；`internal/platform` 不依赖 `services`；GORM 仅限 `catalog`/`cart`；禁止手工改生成物；错误用 `apperr`。

---

## 5. AI 子系统与安全约束

* **可用性隔离**：`ai` 是可选项，故障不得影响浏览、下单、支付。
* **协议优先**：必须走统一的 `Turn/Action` 协议；RAG 必须走版本化可变索引，严禁原地重建。
* **权限安全**：工具默认只读；写操作必须“草案 -> 人工确认 -> RPC”。身份只能来自服务端。Mesh 必须使用显式服务间 ALLOW 与 AI 精确只读 ALLOW。禁止依赖 broad ALLOW + 不完整 DENY 收敛权限；DENY 只作为第二道防线。

---

## 6. 交付验证与 DoD 门禁

口头回复 `done` 无效，必须附带证据：
1. **Happy Path**：行为变更必须提供受影响测试的实际执行日志 (如 `make test` 或 `make verify`) 并绑定精确 HEAD；纯文档变更不得伪造测试声明。
2. **重灾区抗压**：资金、库存、权限和跨服务恢复必须按 ADR 提供**并发竞态测试**或**故障注入测试**日志，并绑定精确 HEAD；不适用时必须取得人类书面豁免。**Mock 数据必须符合真实业务边界，且必须包含至少一个异常分支的用例，严禁全绿假数据糊弄。**
3. **组合与发布证据**：跨服务变更必须包含受影响基线的非回归证据。实现测试绿色不等于 Release PASS；发布结论必须绑定精确 base/head。阻断项、延期处置及所要求的环境验证。
4. **防御声明**：必须附带 `【测试范围】` 与 `【未覆盖范围】`，帮助人类挡住审查者的过度发散。
5. **AC 证据矩阵**：受影响 AC 必须映射到 `docs/collab/ac-coverage.tsv` 的 `test_type`、`evidence_path`、`status`；AC-REF 不等于 AC-VERIFIED。
6. **协作闸门已退役（2026-11-25）**：`make gates*`、`scripts/collab_gates.py` 与 CI `collab-gates` job 均已删除；G-1…G-4 只是人工检查清单，不得要求运行已删命令，也不得以“已通过机器门禁”作为交付证据。

---

## 7. 绝对禁止与资源滥用

* 禁止引入未批准的技术栈 (Kafka、Temporal、新 ORM 等)。
* 无测量依据不得在非核心链路引入高复杂度的极值性能优化。
* 已批准 Spec/ADR 与代码冲突时，必须停止相关交付并由所有者裁决，严禁代码静默覆盖契约；描述性文档与代码现状冲突时，以代码事实为准并修正文档漂移。
* 单节点 dev 集群不得超过 100k/1M RPS 压测。
* `main` 不得新增半成品协议路径；未完成协议留在 WIP 分支。既有 PAY-SETTLE-01 WIP 按人类裁决 B 冻结，不得继续扩展。

---

## 附录 A：EARS 语法强制示例 (所有 PM 与 Architect 必读)

* **反例 (模糊)**：“用户退款时，如果金额超过实际支付，系统应报错。”
* **正例 (EARS)**：
    * “WHEN 用户发起退款申请 `THE SYSTEM SHALL` 校验退款金额。”
    * “IF 申请金额 > 实际支付金额 (captured)，`THEN THE SYSTEM SHALL` 拒绝申请并抛出 `OVER_REFUND_ERROR`。”
    * “WHILE 某履约组存在未完成退款且尚未授权发货，`THE SYSTEM SHALL` 撤销或拒绝该组的发货 permit。”
    * “IF 履约组已经发货后发生退款，`THEN THE SYSTEM SHALL` 保留已发货事实并进入售后退款流程，不得把整单履约一律锁定。”