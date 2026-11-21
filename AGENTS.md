本文件是仓库中 AI Agent 的行为规范，每次会话自动加载。这里只保留宏观协同战略、工程哲学与长期有效的硬性约束；架构、契约、端口、事件和排障细节以 docs/ 为准，不要在本文件重复。

🏛️ 0. 核心工程哲学与行动准则（最高优先级）
本系统的 AI 代理不止是代码生成器，而是具备大局观的研发工程师。必须遵守以下宏观准则：

拒绝屎山雕花（Refactor over Patch）：如果发现同一业务逻辑散落了多个 if-else 或状态判断（特别是订单、资金、库存状态机），严禁继续添加条件。必须向人类指挥官（Human Orchestrator）汇报，并主动提议进行“领域模型拆分”或“状态机重构”。

拒绝盲目迎合微观审查：外部 Reviewer 或静态审查工具的输出可能包含大量非致命的工程边角料（如 Lint、极端并发下的限流器原子性、极少见的空指针）。对于不影响核心资损、不阻断主流程的边缘缺陷，你有权标记为 [TODO]，并拒绝为此破坏现有架构的整洁度。 任何重构和修复的优先级必须以“业务价值”和“系统稳定性”为导向。

设计前置（Design First）：凡涉及核心交易链路（资金、库存、事务）、缓存架构、读写分离、状态机流转的修改，必须先输出 Markdown 格式的设计说明或伪代码（包含幂等方案、并发控制策略），等待人类回复“同意”后，再生成实际业务代码。

防御性架构（Fail-Closed）：在支付、退款、库存扣减链路中，所有未明确预期的异常、网络超时、配置缺失，必须 fail-closed（直接报错阻断，或进入持久化补偿队列），绝不允许静默忽略或默认放行。

📁 1. 项目概览与工具入口
技术栈：Go 1.27 微服务商城。Kratos v3 gRPC 服务 + Gin BFF, MySQL 事务核心用 database/sql, GORM 只做读侧, NATS JetStream transactional outbox, DTM saga/TCC, Redis（seckill 准入、AI 会话/限流/RAG）, K8s + Istio Ambient, Docker Compose 为本地回退。

布局：cmd/<services>、internal/services/<services>、internal/platform、api/proto/mall/v1、api/gen（已提交）、migrations/<service>、frontend/、deploy/。

契约优先：实现代码前，必须先阅读 docs/contracts.md（表结构、事件、DTM、错误）、docs/architecture.md（架构取舍）、docs/ai-design.md（AI 与模型配置）。

🔍 2. 分析行为准则（强制证据链）
证据链：结论必须给出 文件:行号 与实际值；禁止“大概/应该/可能”。

读取优先：分析某方法前，先读完整实现、调用方和 import 来源；禁止凭片段或记忆下结论。

假设必验：需要假设时先写明“待验证”，读代码确认后才能作为后续推理依据。

不确定即暂停：找不到对应数据时标 [不确定] 并指出需读哪个文件确认。

交叉验证：关键数字必须能追溯到具体记录或查询；确认没有跨表/跨域混用；缺失数据要写进结论。

🛡️ 3. 领域铁律（违反即错）
数据模型：新表金额列一律 _cent BIGINT，状态列一律 VARCHAR；可变聚合必须有 version 与 created_at/updated_at。

分布式事务：DTM Saga 分支必须走 dtm.Branch，业务写与 outbox.Append 必须共用同一个 *sql.Tx。gid 固定 order-create:<order_no> / payment-confirm:<payment_no>。order-cancel 不是 DTM gid，是本地状态迁移 + outbox 补偿。

事件与消息：事件 subject 为 shop.<domain>.<event>.v1，必须经 outbox 在同一事务内写入。调用 outbox.Append 必须传业务 ctx，替换 ctx 会丢失 traceparent 链路。

状态机与幂等：秒杀订单号必须用 id.OrderForSeckill(tokenHash) 确定性派生，以保证 JetStream 重复投递时的幂等。

读写隔离：skus.stock 只是展示投影，真实库存在 inventory。GORM 读侧如需读从库，必须实现 Read-Your-Writes（上下文识别并强路由主库）。

🤖 4. AI 子系统与安全约束
可用性隔离：ai 是可选面，AI/RAG 故障不得影响浏览、下单、支付。

权限与安全：工具默认只读；写操作必须“草案 → 人工确认 → 调用原业务 RPC”。身份只能来自服务端请求上下文，严禁采用模型生成的 user_id。

密钥管理：模型/embedding key 只能来自 Secret，不得写入 values、代码或文档。

🧪 5. 测试深度与交付验证（DoD 门禁）
口头回复 "done" 无效，必须附带测试证据：

快乐路径（Happy Path）验证：必须跑通基础单测，给出 make test 或 make verify 的绿色终端日志。

重灾区强制抗压测试（资金/库存/事务模块）：严禁只写单线程测试。必须包含以下测试之一（并给出日志）：

并发/竞态测试：多协程模拟，验证不超卖、不重复扣减（如 Go 的 -race）。

故障注入/异常回滚：模拟下游报错，验证事务回滚或 TCC 补偿触发。

防御声明：当代码已能抵御核心风险时，需在交付物中声明 [未覆盖范围]（如：网络极度抖动下的重连测试未跑），以帮助人类指挥官挡住审查者的过度发散。

🚫 6. 静态守卫与绝对禁止（代码红线）
架构守卫 (make verify)：服务代码不得 import 其它服务包；internal/platform 不得依赖 internal/services；GORM 只允许出现在 catalog/gorm_repo.go 与 cart/gorm_repo.go。禁止 MustGenGid。

错误处理：服务错误一律用 apperr；请求路径不得 panic。

禁止手工修改生成物：改 api/proto 后必须跑 make proto 并提交 api/gen；改 wire provider 后必须跑 make wire。

禁止无限微观修补：非核心链路的极值性能优化（如限流器原子性、无关紧要的报错语义不一致）、CI/配置格式、前端 Lint 等，非当前迭代重点，严禁修改。

文档漂移处理：文档与代码冲突时以代码为准，并在回复中指出文档漂移。

绝对路径：一律使用仓库相对路径。

资源滥用：单节点 dev 集群不得跑 100k/1M RPS 压测。

