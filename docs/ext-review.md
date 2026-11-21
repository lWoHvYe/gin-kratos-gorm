# 架构问题与后续路线（系统复审转交版）

**快照**：2026-11-21。只列当前代码仍有证据的剩余缺陷、配置触发风险和后续优化；已完成主体不保留，固定旧 ID 不变，独立新增问题从 `#28` 起。

**验证边界**：本轮是静态复审，未运行 Go、前端、真实模型、MySQL/NATS/Redis、Helm/Compose、集群或 live 验证。下文验证均为执行方建议，不是本 reviewer 的代码完成门槛。

**复审判定**：`review-tracking.md` 的 `done` 不能直接作为验收结论；若只按该表和 `ai-hardening-review.md` 收尾，会遗漏售后、履约、多仓、部署及 AI 状态正确性。下表已逐项回读当前实现并缩小或纠正旧描述，不再转述历史结论。

**回报约定**：每个 ID 回报变更文件、实际执行的检查及结果、未验证项和实现偏差，禁止只报 `done`。

---

## 一、当前缺陷与风险（逐项对照）

### P1: 资金、库存、订单正确性与安全恢复

| ID | 类型 | 当前剩余问题与处理要求 | 建议验证 | 当前代码证据 |
|---|---|---|---|---|
| P1#1 | 确定缺陷 | 关闭桶释放的重复台账主体已修；当前是 mapping helper 在 `r.status` 改为 `RELEASED` 前序列化 `released` 事件，外层随后才改状态且不重发，事件 payload 可仍为 `HELD`。在唯一负责事件的层显式使用终态。 | 关闭桶释放只写一条普通台账和一个事件，事件状态为 `RELEASED`；HELD/CONFIRMED、重放和回滚语义不变。 | `internal/services/inventory/seckill.go:581-601`；`internal/services/inventory/repo.go:292-305` |
| P1#2 | 确定缺陷 | `TccCancelConfirm` 逐 SKU 开独立事务，中间 SKU 出错会提前返回，后续 SKU 未处理；consumer 又把 `NotFound` 转为 `nil`，通用 consumer 随即 Ack，整条取消消息不再重投。按 SKU 持久化/汇总补偿结果，未处理的 SKU 不得整条消息 Ack，第 N 个 SKU 返回 NotFound/临时错误时，前面不重复释放、后面仍被处理；重投、重启和部分成功最终收敛。 | `internal/services/inventory/tcc.go:55-148`；`internal/services/inventory/consumer.go:39-64`；`internal/platform/consumer/consumer.go:91-124` |
| P1#37 | 确定缺陷 | 支付后的 `TccConfirm` 同样逐 SKU 执行并在首个错误返回；confirm consumer 整批重投时，中间 SKU 持续失败会使后续 SKU 在进入 DLQ 前始终未 Confirm。与取消补偿共用按 SKU 进度和剩余项重试协议，不能只重放整批后卡在同一项。 | 多 SKU 第 N 项持续/临时失败时，前项不重复扣减、后项最终被处理；重投、DLQ 重放和重启后所有已支付明细均为 CONFIRMED。 | `internal/services/inventory/tcc.go:37-49`；`internal/services/inventory/confirm_consumer.go:41-59`；`internal/platform/consumer/consumer.go:91-129` |
| P1#3 | 确定缺陷 | 重建工具默认 repair 已修；剩余是只处理仓 1、SKU 不存在时静默跳过，且按 id 无条件覆盖 `stock/stock_version`，重建期间的旧读值可覆盖更新投影。覆盖全部已分配仓库，输出 missing/updated/conflict 报告，并用版本条件或冲突后重读保证收敛。 | 多仓和缺失 SKU 必须出现在计数与报告；重建与实时投影并发时旧快照不能覆盖新版本，重复执行幂等。 | `cmd/catalog-stock-rebuild/main.go:21-82`；多仓契约见 P1#23 |
| P1#4 | 确定缺陷 | HITL 完整 hash/meta/CAS、失败恢复和前端 `checkpoint_closed=false` 主体已存在；剩余是业务 AlreadyExists 只按 code 回读、不比较现有实体与已审草案，保存失败后的 Abort 错误被丢弃，abort/UI 固定展示 `FAILED`，与 SAVED 后拒绝 Abort 的真实状态可不一致。AlreadyExists 必须绑定规范化内容，并展示可查询的真实 checkpoint 状态。 | 同 code 不同内容拒绝或显式走更新流程；重复确认不重复写；保存失败且 Abort 失败仍有恢复入口；SAVED/FAILED/CONFIRMING 展示与后端一致。 | `internal/services/admin/promotions_handlers.go:209-236`；`internal/services/admin/ai_hitl.go:75-118`；`frontend/apps/admin-web/src/views/promotions/CampaignDraftView.vue:95-155` |
| P1#20 | 确定缺陷 | 售后额度把 `paid-refunded` 再减去包含已退款单的售后 SUM，历史退款被重复扣减，可能误拒；SUM、同 key 回读和额度检查又在事务外，同 key 并发可改变幂等返回语义，不同 key 可并发超额申请。按状态只计算未退款占用，并在数据库事务/唯一约束内原子占用额度。支付行锁和累计上限能阻止实际超额打款，但不能修复申请层错误。 | 多笔部分退款后的可申请额度精确；同 key 并发只返回同一业务结果，不同 key 不超总额；支付末端上限回归不变。 | `internal/services/aftersales/service.go:70-160`；`internal/services/payment/service.go:245-330` |
| P1#21 | 确定缺陷 | 退款投影只写 `refunded_cent/refund_status`，履约守卫却检查主 `status` 是否为退款态，无法挡住已退款订单继续履约；订单筛选也只看主状态。另有 `delivered` 早到持续报错并进入通用 DLQ 的乱序风险。明确主状态、退款状态和履约状态的所有权，守卫/查询使用同一读模型，并为可重排事件设计暂存或重试。 | PAID 后全额/部分退款均按规则阻止或允许发货；按退款态查询无遗漏；paid/refunded/shipped/delivered 乱序及 DLQ 重放最终一致。 | `internal/services/order/refund_consumer.go:35-69`；`internal/services/order/consumer.go:35-110`；`internal/services/order/service.go:306-360` |
| P1#22 | 配置触发安全隐患 | 非 dev 默认 `mock=false` 可被非法非空 `PAYMENT_MOCK_ENABLED` 绕过；ParseBool 失败后保留默认 true，同时跳过 secret 启动校验；`verifySignature` 对空 secret 直接成功。配置解析必须 fail-closed，非 dev 禁止 mock/空验签 secret。HTTP mock 路由有登录与 ACTIVE 校验，不是匿名公开；gRPC Callback 的实际外部暴露范围**【不确定】**，需部署核验。 | 非法布尔值、空 secret、非 dev 显式 mock 均启动失败或验签失败；合法 dev mock 不受影响；核验 ingress/Service/mesh 是否暴露 Callback。 | `internal/platform/config/config.go:110-140,248-269,390-430`；`internal/services/payment/service.go:386-397`；`internal/services/gateway/router.go:22-100` |
| P1#23 | 确定缺陷/扩展前置 | 普通订单、自动 shipment 和退货回库仍存在固定 `warehouse_id=1`，但库存和秒杀活动以仓库为业务键；多仓启用后会扣 A 仓、发/退 B 仓。把仓库分配持久化到订单明细并贯穿 reservation、confirm、shipment、return/restock，提供历史数据回填策略。 | 两仓同 SKU 的下单、取消、支付、发货、退货和补货均只影响原仓；历史单回填和跨仓防止串账有验证。 | `internal/services/order/service.go:38,192-200,573-578`；`internal/services/aftersales/service.go:517-520`；`internal/services/fulfillment/service.go:53-143` |
| P1#24 | 确定缺陷 | `ReceiveReturn` 先提交 `RECEIVED` 再跨服务 `Restock`；RECEIVED 时重复调用尚可人工重试，但退货退款在 RECEIVED 已允许进入 REFUNDING，随后 ReceiveReturn 又拒绝该状态，Restock 人工入口也会关闭。自动 recovery 只扫描 REFUNDING，没有独立回库待办；admin UI 同样无置入入口。将 Restock 待办持久化并独立自动/人工重试，回库未成功不得进入退款终态。 | 重试、进程重启、重复收货/重试不重复加库存；RECEIVED->REFUNDING 竞态不能绕开回库，退款后也不存在无法恢复的欠库。 | `internal/services/aftersales/service.go:381-390,468-503`；`internal/services/aftersales/recovery.go:35-59`；`frontend/apps/admin-web/src/views/aftersales/AfterSalesView.vue:46-52` |
| P1#25 | 配置触发恢复风险 | HA 文档只在 `mall` namespace 创建 MySQL Secret，infra migration Job 却安装到 `mall-data` 并引用同名 namespace-local Secret；按文档全新安装可在迁移前失败。让 chart、namespace 和 Secret 指引一致，并新增安装前 preflight。 | 仅按文档在全集群安装 HA，migration Job 能取得凭据；错误 namespace/缺 key 在运行迁移前给出明确错误。 | `docs/production-ha.md:55-119`；`deploy/k8s/charts/infra/templates/migrate-job.yaml:20-56`；`deploy/k8s/charts/infra/values-ha.yaml:15-21` |

### P2: 可用性、规模、AI 正确性与产品契约

| ID | 类型 | 当前剩余问题与处理要求 | 建议验证 | 当前代码证据 |
|---|---|---|---|---|
| P2#9 | 确定缺陷 | 非法 cursor 的 GORM 路径以普通错误返回并被包成 INTERNAL，SQL fallback 为 INVALID_ARGUMENT；统一输入错误语义，并共用 fixture 固化双读 keyset 排序和分页。 | 两条读路径均返回 INVALID_ARGUMENT；首页/中间页/末页、变页大小和并发数据变化不重不漏。 | `internal/services/catalog/gorm_repo.go:151-175`；`internal/services/catalog/service.go:122-144` |
| P2#12 | 确定缺陷 | catalog smoke 清理的 SPU `OR`/AND 未分组，父商品保护只约束第二分支；brand/category 用 `id IN` + `name IN`，允许 ID/name 交叉误配。修正布尔分组并逐实体配对；已应用环境只能新增向前迁移，不 rewrite 历史。 | 改编 SPU、ID/name 交叉及正常业务相似数据均不误删；全新非 dev 无 smoke，dev seed 可重复。 | `migrations/catalog/0014_remove_smoke_seed.sql:1-18` |
| P2#14 | 规模风险 | strict 高风险路径已直接权威查询；当前剩余是进程内 ACTIVE map 无容量上限，过期 Lookup 只 miss 不删除，唯一用户持续增长会积累。增加容量上限、过期清理和命中指标，不改变 strict 权威校验。 | 长期唯一 ID 压测内存有界，过期项被回收；strict 禁用/版本失效语义保持，普通读路径收益可量化。 | `internal/platform/activecache/cache.go:20-52`；`internal/services/gateway/active_user.go:29-49`；`internal/services/admin/active_user.go:28-43` |
| P2#17 | 确定缺陷 | RAG 剩余范围不只是 cache/rerank：发布先切 alias、后写 current；版本在文档写入前登记且失败版本残留；Upsert 的 prefix hash 提前回退可跳过新构建版本；缓存只清当前 writeIndex，reconcile 删除不清缓存；全量/reconcile 限 180/100，截断后仍可能把未遍历商品当 unseen 删除；build 的 OpenRAG 错误仅记录并正常 return，独立/CronJob 缺少失败状态。重建发布、增量双写、缓存和删除必须使用同一版本协议，不完整采集禁止删除。 | 独立/CronJob 失败应非零并告警重试；Helm post hook 不得让可选 AI 构建失败阻塞核心商城发布，必要时拆分入口。 构建/发布各阶段失败、并发事件、回滚和缓存命中均返回同一版本内容；超过 10,000 商品不丢索引、不误删；CronJob 失败可观测，hook 失败时旧 alias 持续服务且核心发布不被 AI 阻塞。 | `internal/services/ai/rag.go:131-253,351-478,1008-1119`；`cmd/ai/main.go:75-106`；`deploy/k8s/charts/mall/templates/ai-rag-build.yaml:92-130` |
| P2#26 | 确定缺陷 | AI session 的 RPUSH/LTRIM/EXPIRE 本身不会覆盖历史，但 Chat 的 Load 与最终 Append 分离，同 session 并发看不到在途 turn，最终顺序取决于完成顺序；key 仅含 user/session，admin/customer 可共享命名空间。引入 turn ID、同 session 串行化或明确合并协议，并隔离角色命名空间。 | 同 session 并发、乱序完成、同用户 admin/customer 同 session ID 均不串上下文；历史截断和 TTL 回归不变。 | `internal/services/ai/session.go:20-85`；`internal/services/ai/service.go:205-293` |
| P2#27 | 确定缺陷 | recent cart draft key 只有 user ID；当前 turn 无 draft 时 `PopRecent` 会弹出同用户旧 draft，可能串入无关或跨 session 回答。绑定 session/turn 与草案状态，不消费无归属草案；这不是跨用户泄漏，也不会绕过人工确认直接写 cart。 | 同用户双 session、旧草案、过期草案和并发请求不串 draft；每个确认只能消费自己的草案。 | `internal/services/ai/cart_draft_store.go:15-80`；`internal/services/ai/service.go:308-313` |
| P2#28 | 确定缺陷/成本风险 | 日预算请求前 Load、结束后 Add，无原子预授权，并发可超限；usage 用可能已取消的 ctx 写 Redis 且失败仅记日志；Limiter 首次 INCR 后 EXPIRE 失败会留下无 TTL key。采用原子 reserve/settle 与有界计数器，记账失败进入可观测补偿。 | 同用户/全局并发不超预算；断连、超时、Redis 部分失败不造成静默漏账或永久限流。 | `internal/services/ai/service.go:145-341`；`internal/services/ai/limiter.go:15-55` |
| P2#29 | 确定缺陷 | `tool_stream.Stream` 读到上游 EOF 后才返回新 stream，SSE delta 是事后重放，BFF 又使用 `WithoutCancel` + 120 秒，断连不会立即取消上游。实现增量工具事件转发与取消传播，仅对确需完成的事务使用有界脱离上下文。 | 首 token/工具事件在到达时可见；断连及时取消模型和工具调用，120 秒仍是兜底而非常态成本。 | `internal/services/ai/tool_stream.go:20-70`；`internal/services/gateway/ai_handlers.go:25-96` |
| P2#30 | 确定缺陷 | feedback 没有 turn ID、回答存在性校验或幂等键，无法可靠归因；每日 list 写入无专用频控，读取限 5000 并不能限制写入增长。扩展兼容 proto 字段并绑定服务端 turn，增加用户级频控和幂等。 | 重复反馈不重复计数，跨 session/turn 不串评价；伪造 turn 拒绝，持续写入有明确上限。 | `internal/services/ai/feedback.go:15-90`；`internal/services/gateway/ai_handlers.go:75-96`；`api/proto/mall/v1/mall.proto` 的 `AIFeedbackRequest/AIChatEvent` |
| P2#31 | 确定缺陷 | Cart 持久幂等与前端 `draft_closed=false` 已修；BFF 却先读 Redis draft、Catalog、Inventory，最后才到 Cart 持久幂等边界。Cart 已提交后若 Redis draft 丢失或依赖不可用，重试会在查询持久确认前被阻断。提供按 draft ID 的持久恢复/先查确认路径。 | Cart 已提交后分别注入 draft 丢失、Catalog/Inventory 故障、Complete 失败；重试均能恢复原结果且不重复加购。 | `internal/services/gateway/cart_draft.go:35-105`；`internal/services/cart/service.go:336-392` |
| P2#32 | 确定缺陷 | `Ops latest/done` 原子写和 SKU 深链已修；调度锁值固定 `1` 且 defer 无条件 Del，锁过期后旧持有者可删新锁；InventoryView 固定合 1 且不读取 `low_stock=1`。使用 owner token 比较删除并补多仓筛选。 | 锁过期/续约/跨实例竞争不互删；低库存深链在多仓下保留 SKU、仓库和筛选条件，同一天只生成一次。 | `internal/services/ai/ops_brief_scheduler.go:30-60`；`internal/services/ai/ops_brief.go:35-96`；`frontend/apps/admin-web/src/views/catalog/InventoryView.vue:1-55` |
| P2#33 | 恢复风险 | K8s 与 Compose migration runner 都先执行 SQL、后单独写 checksum 台账，且无并发互斥；进程中断或双 Job 可造成“部分已执行但未记账”。按 MySQL DDL 隐式提交语义加数据库锁、执行中/失败标记和启动重核，不能假设所有 DDL 与台账可同事务。 | 在部署/重启/CronJob 交付集中注入 kill、双 Job 并发、checksum 不符和重复启动均不会并发重跑或静默跳过；人工恢复步骤明确。 | `deploy/k8s/images/migrate/run.sh:36-64`；`deploy/compose/init/mysql/01-schemas.sh:41-67` |
| P2#34 | 配置触发 HA 风险 | seckill stream replicas 在 deployment 固定为 `1`，不会继承 HA 的 `streamReplicas=3`；DTM 默认副本并直接引用 values 的 MySQL 密码，HA values 未覆盖。统一副本配置并改 Secret 引用，默认值与生产 profile 分层。 | HA 渲染后所有 JetStream stream（含 seckill）副本符合预期；DTM 副本和凭据来自生产配置/Secret，values 无真实密码。 | `deploy/k8s/charts/mall/templates/dtm.yaml:1-90`；`deploy/k8s/charts/mall/values-ha.yaml:1-95` |
| P2#35 | 韧性缺口 | NATS client `MaxReconnects(10)` 达到上限后不再重连，`Healthy` 只检查 degraded subscription，不检查当前连接状态。按服务期望配置持续重连/退避，并把连接状态纳入健康语义；实际中断时长与副本切换影响**【不确定】**，需故障演练。 | 超过 10 次重连间隔的网络中断后生产/消费自动恢复；健康检查与真实连接一致，不造成重启环或假健康。 | `internal/platform/mq/mq.go:16-120` |
| P2#36 | 产品契约缺陷 | storefront 只在 RETURNING 显示物流表单，但后端首次提交要求 APPROVED；REFUND_ONLY 前端强制商品非空而后端允许为空；待支付订单无继续支付入口；秒杀轮询终止条件一直读首次响应 `q.status`，且支付 mock 404 后进入无继续支付的订单页。统一前后端状态机并补支付恢复闭环。 | APPROVED->RETURNING->RECEIVED 全流程可走通；仅退款无需商品；待支付可恢复；秒杀成功/失败/超时及时停止轮询且支付异常有明确下一步。 | `frontend/apps/storefront-web/pages/after-sales/[no].vue:1-56`；`frontend/apps/storefront-web/pages/after-sales/apply.vue:1-75`；`internal/services/aftersales/service.go:101-105,400-470`；`frontend/apps/storefront-web/pages/orders/[orderNo].vue:1-60`；`frontend/apps/storefront-web/pages/seckill/[id].vue:35-100` |
| P2#37 | 配置触发越权面 | AI code client 虽只读，mesh 却允许 AI 调用 Catalog、Inventory、Pricing/Promotion、Order、AfterSales 的服务级 `/*`，其中包含 Reserve、CancelOrder、CreateAfterSale、Refund 等写 RPC；网络层没有执行“工具默认只读”。将现有有通配和新增 Payment/Fulfillment 统一收缩到实际只读方法白名单，业务 RPC 仍保留自身身份/归属校验。 | 渲染 AuthorizationPolicy 后，AI 对所有依赖服务仅能调用白名单只读方法；写 RPC 在 mesh 层拒绝，gateway/admin 等既有调用方不受影响。 | `deploy/k8s/charts/mall/values.yaml:279-307`；`deploy/k8s/charts/mall/templates/authorizationpolicy.yaml:65-93`；`api/proto/mall/v1/mall.proto:240-249,381-386,720-725` |
| P2#38 | 配置触发越权面 | AI code client 虽只读，mesh 却允许 AI 调用 Catalog、Inventory、Pricing/Promotion、Order、AfterSales 的服务级 `/*`，其中包含 Reserve、CancelOrder、CreateAfterSale、Refund 等写 RPC；网络层没有执行“工具默认只读”。将现有有通配和新增 Payment/Fulfillment 统一收缩到实际只读方法白名单，业务 RPC 仍保留自身身份/归属校验。 | 渲染 AuthorizationPolicy 后，AI 对所有依赖服务仅能调用白名单只读方法；写 RPC 在 mesh 层拒绝，gateway/admin 等既有调用方不受影响。 | `deploy/k8s/charts/mall/values.yaml:279-307`；`deploy/k8s/charts/mall/templates/authorizationpolicy.yaml:65-93`；`api/proto/mall/v1/mall.proto:240-249,381-386,720-725` |

### P3: 工程治理

- **proto guard（确定缺陷）**：RPC key 只有 `rpc.<Method>`，跨 service 的 `TccConfirm/TccCancelConfirm` 等重名会覆盖；reserved 多行同 message 相互覆盖，字段名 reserved 和带字段选项的声明未被完整解析。以 `service+rpc` 为键并解析完整签名、reserved/deprecated，接入统一 CI。证据：`scripts/check-proto-evolution.py:1-87`；`api/proto/mall/v1/mall.proto` 的 Inventory/Pricing TCC RPC。
- **偏好评估（质量缺口）**：baseline 命中偏好品牌已会 FAIL，前端清除也已检查响应；剩余是仅一个 prompt family，最终 `clearPrefs` 错误被忽略，不能据此证明跨场景收益或状态隔离。证据：`cmd/ai-pref-eval/main.go:21-150`。
- **文档漂移**：`review-tracking.md:27-28` 把原 P2#14/#15 错写为 P2#13/#14，本文沿用转交前固定 ID；`docs/ai-design.md:300-306` 仍称 ADK 非流式，但当前代码已启用 streaming 和 tool-aware adapter，应以代码为准并同步设计说明。证据：`internal/services/ai/adk_agent.go:26-90`。

---

## 二、架构与设计优化（独立重点）

1. **建立订单资金/履约统一读模型，而非继续堆主状态**：主 `status` 只表达订单生命周期，`refund_status` 表达资金，shipment 表达履约；守卫和 BFF 查询基于明确组合规则，不要把三轴重新压成一个枚举，也不新增第二套订单事实源。
2. **把售后改为“额度占用 + 持久恢复”状态机**：申请、退货、回库、退款分别拥有幂等键和可恢复待办；支付仍保留最终金额上限。跨服务继续用原 RPC/outbox 补偿，不把所有步骤强并成一个分布式事务。
3. **以仓库为订单领域键**：在订单明细记录分配仓并贯穿库存、履约和售后；多仓路由先做确定性单仓订单，跨仓拆单需另有产品规则后再做，避免提前复杂化。
4. **统一消费者部分失败协议**：Confirm 与取消补偿等批量 SKU/item 明确“已全部完成、可重试剩余、永久失败”的结果，禁止部分成功后整消息 Ack，也禁止整批重投让后续项永久饥饿；复用当前幂等表、DLQ 和 outbox，不引入新工作流引擎。
5. **配置与发布 fail-closed**：布尔/密钥/namespace/replicas 在启动或安装前校验；迁移、RAG alias/current 和 HA profile 都需要可恢复发布协议。开发默认值与生产 values 分层，避免隐式继承改变安全语义。
6. **按量优化按证据排序**：先去 10,000 商品硬上限、限制 active cache、批量分页与缓存版本一致性，再依据 profiles 做列表缓存、索引和连接池调整；没有真实只读副本前不做读写分离。
7. **产品闭环优先于新增商城模块**：先完成继续支付、售后全状态、退款/物流可解释、异常恢复和通知，再考虑评价、收藏等外围功能；每项补转化/完成率指标，避免只增页面。
8. **保持现有技术边界**：继续 Kratos/Gin、显式 SQL、outbox/DTM、NATS、Redis Search 和 Istio；事务核心不迁 ORM，不引入 Kafka/Debezium、Temporal、独立向量库、第二套网格或自由 multi-agent，单节点不做 100k/1M RPS 压测。

---

## 三、AI 扩展路线（独立重点）

| 阶段 | 产品与工程范围 | 退出参考 |
|---|---|---|
| Phase 0: 可信会话底座 | 先完成 P1#4、P2#17、P2#26-#31、#38；统一 turn/session/draft/budget/cancel/feedback ID 和 trace，建立质量与成本分维度观测。 | 并发、断连、重试、恢复不串上下文、不重复写、不静默超预算；质量指标能定位到具体 turn 和工具证据。 |
| Phase 1: 订单/支付/履约只读解释 | 在现有订单/售后工具外增加 Payment、Fulfillment 只读 client，回答“是否付款、退了多少钱、到哪了、为何不能退”；先补 GetIntent 的订单/用户归属契约，禁止只凭 payment_no 跨用户查询。mesh 同步收敛现有 Catalog、Inventory、Pricing/Promotion、Order、AfterSales 适配，并按 GetIntent、GetShipmentByOrder 等实际只读方法授权新增 Payment/Fulfillment；不能只给新服务加白名单而保留旧写权限。 | 所有回答来自归属校验后的 RPC，支付/退款/物流口径一致；网络策略仅放行白名单只读方法，AI 不可用时原页面交易不受影响。证据：`internal/services/ai/tools.go:17-52,250-411`；`internal/services/payment/service.go:144-157`；`deploy/k8s/charts/mall/values.yaml:279-307`；`cmd/ai/main.go:75-106`。 |
| Phase 2: 导购到支付转化 | 比较候选商品 -> GetProduct/BatchGetStock 核验 -> 复用 Cart draft 人工确认 -> 交回原 checkout UI/流程；用户基于最终报价和地址显式确认创建订单，并另行显式发起支付。Cart 确认只授权加购，不是后续写操作的 blanket approval；模型不得生成 user_id、价格或直接改库。 | 推荐均有当前商品与库存证据；草案过期/涨价/缺货可恢复；订单与支付各有独立确认；分别观测推荐点击、确认加购、下单和支付转化，不用回答质量替代商业指标。 |
| Phase 3: 售后动作草案 | 先基于 Phase 1 解释，再实现持久 action draft -> 人工确认 -> 原 `CancelOrder`/`CreateAfterSale`；草案绑定订单快照、金额、商品、原因和有效期，重复确认幂等。AI 不直接退款、不直接确认收货。 | 越权、状态变化、超额、重复/并发确认均 fail-closed；人工能看到差异并恢复失败草案；支付末端上限继续独立生效。 |
| Phase 4: 运营助手闭环 | 用确定性查询生成低库存、退款异常、履约积压、DLQ 等 findings 和证据链接，再生成补货/促销预填草案；促销复用 campaign HITL，其他写操作也走原业务 RPC 和人工确认。 | finding 可追溯到服务查询和时间窗，模型不能捏造异常；多仓、低库存过滤和恢复入口完整；记录采纳率、处理时长和误报率。 |
| Phase 5: 偏好与评估 | 显式偏好优先于隐式记忆，扩充多 prompt family、对抗/多轮/并发 eval；质量（groundedness、工具正确率、拒答）与成本/商业漏斗分开看板，不建无限向量记忆。 | baseline 具有判别力，初始/最终清理失败即失败；多次运行稳定，收益以对照实验而非单次演示判断。 |
| Phase 6: 高级能力证据门 | multi-agent、多模态、语义 rerank/cache、模型路由继续按需评估；先在同版本索引和相同数据集做对照。 | 只有单 Agent/Workflow 被评测证明不足，且质量、延迟或成本收益覆盖新增复杂度时才采用。 |

**体验实现顺序**：正确 streaming 与取消先于动画；草案恢复和来源解释先于多 Agent；支付/履约只读解释先于任何售后写工具。

---

## 四、执行顺序与依赖

| 批次 | 范围 | 依赖与并行边界 |
|---|---|---|
| A: 资金与库存正确性 | P1#1、#2、#37、#20、#21 | 库存与售后/订单可并行；统一验证多 SKU Confirm/取消、并发退款、事件乱序，不能用支付上限替代申请层修复。 |
| B: 履约与恢复 | P1#3、#23、#24、#25、P2#33 | #23 先定仓库数据契约再改履约/退货；迁移和 HA 可并行，涉及 proto 时统一一次 `make proto`。 |
| C: 安全与生产 profile | P1#22、P2#14、#34、#35 | 配置 fail-closed 先行；真实暴露面、重连和 HA 行为由执行方在对应环境验证。 |
| D: AI 可信底座 | P1#4、P2#17、#26-#32、#38 | RAG、session/draft、budget/stream 可分 owner；共享 proto 的 turn/feedback 契约集中变更，避免并行重写生成物。 |
| E: 产品契约与 AI 扩展 | P2#36 -> AI Phase 1-5 | 先修前后端售后/支付闭环，再让 AI 复用同一 RPC；导购与只读解释可并行，售后写草案依赖 Phase 1 和 A/B。 |
| F: 治理与规模 | P2#9、#12、P3、第二节其余优化 | 不阻塞 P1；cursor/seed/proto guard 独立交付，规模项需 profile 或对照数据。 |

---

## 五、复审记录与交叉验证

**数据读取记录**：第一节每行“当前代码证据”即本轮实际读取范围；分析覆盖对应完整方法及 consumer/BFF/配置/前端调用方。历史文档仅作线索，未作为代码事实来源。

**分析步骤**：先按旧 ID 回读剩余范围，再沿售后->支付->订单/履约、订单->库存->仓库、AI 请求->session/draft/budget->BFF、RAG 构建->发布->增量->删除、部署 values/chart->运行脚本做开放调用链扫描，最后去重排序。

**逐项对照**：`done` 声明只与当前代码剩余范围对照；已完成主体已删除，旧 ID 不重新编号，新发现不与旧项强行合并。

交叉验证结果：
- **数字留证**：通过。`10,000` 仅来自 RAG 的 `100 页 x 100 条`，`120 秒`、`MaxReconnects(10)`、副本 `1/3` 均只用于其对应证据行。
- **分类边界**：通过。售后“可超额申请”未被写成“必然超额打款”；HTTP mock 未写成匿名公开；draft 串话未写成跨用户泄漏或直接写 cart；运行暴露与故障时长标为「不确定」。
- **汇总统计**：通过。本轮不以完成率或缺陷总数支撑结论，也未把未运行验证计为零缺陷；缺失运行证据已集中声明。
- **结论溯源**：通过。每个当前缺陷均有仓库相对 `文件:行号`；优化和扩展均以前述缺陷或现有 `client/proto` 边界为依据，没有沿用已排除的旧描述。