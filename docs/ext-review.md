# 架构调整与实施路线（系统复审决策版）

**快照**：2026-11-21。固定旧 ID 不变，独立新增问题从 `#20` 起；ID 只用于追溯，不再代表执行顺序。
**验证边界**：本轮是静态复审，未运行 Go、前端、真实模型、MySQL/NATS/Redis、Helm/Compose、集群或 live 验证。文中验证均为执行方建议。
**核心判断**：当前不是一批彼此独立的 bug，主线是**两个交易一致性设计缺口**，其二是 **AI 的 Turn/Action 与 RAG 版本发布协议缺失**；其余是安全小修、部署治理和产品门控。继续按 ID 逐个补 `if`、补扫描、补 Redis key，会让状态机更复杂而不收敛。

---

## 一、先做什么：架构决策总览

| 顺序 | 决策 | 合并范围 | 执行方式 |
|---|---|---|---|
| 0. 立即安全小修 | 支付配置 fail-closed；AI mesh 只读最小权限 | #22、#38 | 小范围独立修改，不等待重构，也不扩成平台项目 |
| 1. 交易重构 A | 统一订单、售后、支付、履约的状态所有权与恢复协议 | #20、#21、#24、#36 | 先定状态机和不变量，再一次改后端契约、恢复和前端；禁止分四轮补丁 |
| 1. 交易重构 B | 统一库存整单 Confirm/Cancel 的逐项持久进度与终态事件 | #1、#2、#37，兼收 #3 的版本覆盖 | 建立可重放的库存操作协议；禁止只修某个 NotFound、某个事件字段 |
| 2. 产品决策门 | 明确当前产品是否真正支持多仓 | #23、#3 的仓 1 限制 | 未批准多仓 PRD/ADR 时只维护单仓约束，不默认全面返工 |
| 3. AI 启用时优先 | 建立 Turn/Action 协议，再扩导购和售后 | #4、#26-#31，复用 #38 | 统一会话、草案、预算、取消、反馈与恢复，不逐点新增 Redis key |
| 3. AI/RAG 分别启用时优先 | 建立版本化可变索引与可恢复发布协议 | #17 | 只门控 RAG 检索能力，不与 Turn 状态混改，也不阻塞无 RAG 的只读工具 |
| 4. 局部治理 | 部署、迁移、连接、缓存、cursor、seed、Ops 和工程守卫 | #9、#12、#14、#25、#32-#35、P3 | 独立小修，按风险和实际启用环境安排，不与交易正确性并列 |
| 暂缓 | 跨仓拆单、高级 AI、读写分离、新商城外围模块 | 无新增 ID | 由产品收益、规模数据和对照 eval 决定，不为“完成清单”而做 |

**资源冲突时的取舍**：先完成 0 和两个交易重构；AI、RAG、多仓、HA 深化均不得挤占资金/库存/订单正确性。

---

## 二、交易重构 A：订单—售后—支付—履约必须共用一套状态语义

### 2.1 根因：不是几个状态判断写错，而是事实所有权和跨服务恢复没有闭合
当前同时存在：订单主 `status`、独立 `refund_status`、售后状态、支付事实和 shipment 状态，但守卫、列表和前端各自读取不同字段；退货又把“已收货”“已回库”“可退款”压缩为一个 `RECEIVED`。因此单修 #20 的 SUM、#21 的守卫、#24 的 recovery 或 #36 的按钮，都会在下一个状态组合再次失效。

### 2.2 所有权与不变量

| 所有者 | 唯一负责的事实 | 不得再承担的隐含语义 |
|---|---|---|
| Payment | captured、refunded、退款幂等与累计上限 | 不负责售后商品是否回库 |
| Order | 订单生命周期、退款异步投影、发货许可及竞态裁决 | 不把退款态重新压回主 `status` |
| AfterSales | 申请额度占用、退货进度、Restock 待办、退款准入 | 不拥有实际打款事实，不以状态名暗示跨服务调用已成功 |
| Inventory | 真实库存、回库幂等 | 不拥有售后审批或退款状态 |
| Fulfillment | shipment 和物流履约 | 不自行猜测订单/退款状态 |
| BFF/前端 | 同一组合读模型和可用动作 | 不各自推导另一套状态机 |

**必须固化的不变量**：
1. AfterSales 以 Payment 的 captured 或订单成交支付快照为额度基准，持久化每笔申请的 effective_amount 和消费状态。所有未驳回/未取消申请（包括 REFUNDING、REFUNDED）的 SUM(effective_amount) + 本次申请 <= captured；REFUNDED 表示额度已转为实际退款，**不重新释放为可申请额度**。只有未发生退款的驳回/取消才释放。不得再用 Order 的异步 refunded_cent 与 REFUNDED 的 SUM 双重扣减。
2. AfterSales 本地事务序列化本地额度占用，Payment 独立保证实际退款不超过 captured；退款成功后按 after-sale 幂等键对账本地 REFUNDED 与 Payment 事实，不依赖 Order 投影到达先后决定额度。
3. RETURN_REFUND 回库成功前不得进入 REFUNDING、不得调用 Payment 退款；跨服务 Restock 失败必须留下独立、持久、可重试的待办。
4. 退款与发货竞态必须有明确胜者。由 Order 用发货许可和条件更新裁决，必要时同步向 Payment 核验；不得继续读取一个退款投影不会更新的主 `status`。
5. 支付末端始终独立执行 `refunded + amount <= captured`，即使上游售后错误也不放宽。
6. 订单筛选、履约守卫、storefront 和 admin 的动作均由同一状态定义推导。

### 2.3 目标流程，而非新增补丁

- **申请**：在 AfterSales 数据库事务内完成 lifetime 额度消费、幂等键和申请创建；审批调整必须留下新 effective_amount 版本，REFUNDED 仍计入已消费额度。支付仍只接受合法退款指令，退款事实与本地状态按同一 after-sale 键对账。
- **收货**：本地事务同时记录收货事实与 `RESTOCK_PENDING` 待办；worker 调用幂等 Restock，成功后持久化回库事实并开放退款准入。进程重启、重复收货和人工重试都回到同一待办，不再为 REFUNDING/RECEIVED 各写一个 recovery 分支。
- **发货**：Order 对发货许可做条件迁移，Fulfillment 只执行有效许可；退款投影、shipped/delivered 乱序通过版本、暂存或可重试协议收敛。
- **展示**：API 返回生命周期、资金、售后、物流及可用动作，前端不再按单个枚举猜测物流表单或退款入口。

**完成标准**：上述不变量在并发和故障下成立，旧状态能迁移到新协议，前端与后端共用契约；不是四个 ID 各自出现一个新测试文件。

---

## 三、交易重构 B：库存需要整单操作协议，不是循环里的局部修补

### 3.1 根因
`TccConfirm` 和 `TccCancelConfirm` 都逐 SKU 执行并在首个错误返回，而消息层只有整批 Ack/Nak/DLQ。由此产生两类结构后果：中间项持续失败会让后续项饥饿；取消路径的 NotFound 又可能让未处理项随整消息 Ack。关闭桶事件状态错误也是“从尚未落终态的可变结构体序列化事件”的同源问题。

### 3.2 目标设计
1. 由调用方/outbox 为每次 Confirm、部分补偿或整单取消提供确定性的 `operation_id`，并固定、校验该操作的 SKU + 数量集合及 `scope_hash`；重投沿用同一 ID，不同范围不得复用。不能用 `order_no + operation_type` 把支付失败 attempted 子集与后续整单取消误判为同一操作。
2. 持久化该操作每个 SKU 的待处理/完成/失败状态和整体完成条件。每个 SKU 的业务写、台账、事件和进度更新保持本地事务与幂等；**不要求整单一个数据库大事务**，但任何重投都能跳过成功项并继续剩余项。
3. 只有全部必需项达到成功终态才 Ack；永久失败进入可重放的按项 DLQ/恢复视图，不允许整批反复卡在同一项。
4. Confirm 只推进预留终态，Cancel 统一处理 HELD/CONFIRMED；普通与秒杀共用协议，但保留各自库存核算方式。
5. 事件从已持久化的终态构造，不在 helper 中依赖外层稍后才赋值的 `r.status`。
6. Catalog 仍只是版本化展示投影；重建工具必须按版本避免旧快照覆盖，并输出 missing/conflict 报告。

**完成标准**：多 SKU 任意位置失败、重投、重启和 DLQ 重放后，所有项到达正确终态且不重复扣减/释放；不再需要针对单个错误码添加特殊 Ack。

---

## 四、多仓是产品决策门，不是默认返工任务

**【不确定】** 当前文档未证明多仓已经商用，代码则大量固定 `defaultWarehouseID=1`。因此 #23 首先要求产品选择，而不是直接改所有调用点。

| 选择 | 架构处理 |
|---|---|
| A. 当前只支持单仓 | 在边界显式约束仓 1，文档删除多仓支持暗示，重建工具只需完整覆盖该仓；不得让调用方误以为传其他仓库已端到端可用 |
| B. 支持多仓，但每订单单仓 | 先定义分配器、缺货、运费和退货规则，再把已分配仓库持久化到订单明细，贯穿 Reserve/Confirm/Cancel/Shipment/Restock，并制定历史回填 |
| C. 支持跨仓拆单 | 另立产品版本和拆分/运费/部分履约设计，不与本轮正确性重构混合 |

选择 B 后才扩展 #3 为多仓重建；不得先分别把几个常量改成请求参数，形成“看似支持、实际串仓”的假多仓。

---

## 五、AI：先统一两个协议，再谈功能扩展

AI 是可选面，不能阻塞核心商城，也不应在交易重构之前消耗主线资源。

### 5.1 Turn/Action 协议：替代 session、draft、budget 的逐点修补
统一 `turn_id + user_id + session_id + role` 作为一次回答及工具调用的关联键：
- session 明确同 session 并发是串行、拒绝还是合并，历史顺序不能取决于模型完成先后；admin/customer 命名空间隔离。
- 草案绑定 session/turn、业务键、快照、有效期和状态，禁止用仅含 user 的 recent key 猜测当前草案。
- **业务确认事实归业务服务持久化**，Redis 只做易失会话、配额和缓存；Cart 的持久 confirmation 是现有先例。BFF 必须能按 draft ID 从业务服务恢复，不被 Redis/Catalog/Inventory 前置读取阻断。
- 预算采用原子 reserve/settle，usage 和 limiter 失败可观测；断连取消模型与工具调用，streaming 真正增量转发。
- feedback 绑定 turn 并幂等；campaign、Cart、取消订单和售后共用“草案→人工确认→原业务 RPC”的 Action 协议，不分别发明 HITL 状态。
- mesh 将所有现有 AI 服务级 `/*` 与新增 Payment/Fulfillment 一起收缩为只读方法白名单；代码 client 只读不等于网络权限只读。

### 5.2 RAG 版本发布协议：停止分别补 alias、缓存和 reconcile
沿当前 active versions 双写，明确采用**版本化可变索引 + 可恢复指针发布**，不宣称发布后不可变，也不另外发明独立向量库：
1. 每个版本有构建、可发布、当前、失败/淘汰生命周期；单一发布者负责完整构建、校验、发布和回滚，alias/current 任一阶段失败都能恢复到同一可用版本。
2. 当前版本与构建版本按 active versions 协议接收增量；hash 是否跳过必须按目标版本判断，失败版本从服务集合移除，不能一边原地双写一边声称快照不可变。
3. 搜索缓存键包含索引版本以避免跨版本串读；同一版本原地 Upsert/Delete 仍必须按目标版本统一失效，回滚切回旧版本前也需校验其缓存与内容一致。版本号不能替代同版本缓存失效，也不得只清当前 writeIndex 而遗漏其他 active versions。
4. 只有完整遍历的全量快照才能计算 unseen 删除；100×100 的截断结果不得用于 destructive reconcile。
5. 独立/CronJob 构建失败非零并可告警；Helm post hook 不得因可选 AI 失败阻塞核心商城发布。

### 5.3 扩展顺序

| 阶段 | 扩展内容 | 前置条件 |
|---|---|---|
| AI-0 | Turn/Action、只读 mesh、恢复、成本与取消 | 本节约定的统一协议，而非 #26-#31 各自修补 |
| AI-1 | 订单/支付/履约只读解释 | GetIntent 增加订单/用户归属；GetShipmentByOrder 先通过 Order 核验 order_no 属于请求上下文用户，或扩展 Fulfillment 服务端授权契约；两者均只做只读 RPC 方法级授权 |
| AI-2 | 商品比较、库存核验、Cart 草案、回到原 checkout | Cart 确认只授权加购；订单基于最终报价/地址另行确认，支付再次显式发起 |
| AI-3 | 取消订单/售后动作草案 | 复用交易重构 A 的状态和额度规则；AI 不直接退款、不直接收货 |
| AI-4 | 低库存、退款异常、履约积压、DLQ 等运营 findings | 确定性查询和证据链接先行，补货/促销仍走人工草案 |
| AI-5 | 偏好、rerank/cache、模型路由、多模态、multi-agent | 多 prompt family 对照 eval 证明质量/成本/转化收益；不建无限向量记忆 |

---

## 六、只做小修或后排的事项

| 范围 | 处理定位 | 证据 |
|---|---|---|
| #22 支付配置 | 立即修：非法布尔值 fail-closed，非 dev 禁止 mock/空验签 secret；HTTP mock 不是匿名，gRPC 暴露面 **【不确定】** | `internal/platform/config/config.go:110-140,248-269,390-430`; `internal/services/payment/service.go:386-397` |
| #38 AI mesh | 立即修：现有与新增 AI 权限统一只读白名单 | `deploy/k8s/charts/mall/values.yaml:279-307`; `deploy/k8s/charts/mall/templates/authorizationpolicy.yaml:65-93` |
| #25、#33 部署小修 | namespace Secret preflight；迁移 runner 加数据库锁、失败标记和启动重核，不假设 DDL 与台账总能同事务 | `deploy/k8s/charts/infra/templates/migrate-job.yaml:20-56`; `deploy/k8s/images/migrate/run.sh:36-64`; `deploy/compose/init/mysql/01-schemas.sh:41-67` |
| #34、#35 生产 profile 小修 | 统一 JetStream/DTM replicas 和 Secret；NATS 重连与健康语义按运行目标调整，实际故障影响未实测 | `deploy/k8s/charts/mall/templates/deployments.yaml:265-294`; `deploy/k8s/charts/mall/templates/dtm.yaml:1-90`; `internal/platform/mq/mq.go:16-120` |
| #9、#12、#14 后排局部修复 | cursor 错误语义、seed 删除谓词、有界 ACTIVE cache；均不升级为架构项目 | `internal/services/catalog/gorm_repo.go:151-175`; `migrations/catalog/0014_remove_smoke_seed.sql:1-18`; `internal/platform/activecache/cache.go:20-52` |
| #32 后排局部修复 | Ops 锁 owner token 和低库存深链筛选；latest/done 原子写与 SKU 深链主体已修 | `internal/services/ai/ops_brief_scheduler.go:30-60`; `frontend/apps/admin-web/src/views/catalog/InventoryView.vue:1-55` |
| P3 | proto guard 修正 service+rpc 键和解析；偏好增加多 prompt family；同步编号与 streaming 文档漂移 | `scripts/check-proto-evolution.py:1-87`; `cmd/ai-pref-eval/main.go:21-150`; `internal/services/ai/adk_agent.go:26-90` |

---

## 七、交付顺序与禁止事项

1. **先交付安全小修 #22、#38**，同时编写交易重构 A/B 的短 ADR：状态所有权、不变量、迁移和回滚，不先散改代码。
2. **两个交易重构可由不同 owner 并行**，但各自一次交付完整协议**：后端、恢复、契约、前端和故障验证一起验收。
3. **多仓等待产品决策**；部署和局部治理穿插进行，不阻塞交易主线。
4. **AI 会话与 Action 扩展只由 Turn/Action 协议门控，RAG 检索能力只由版本发布协议门控**；无 RAG 的只读解释、导购和售后不得被 RAG 重构阻塞。核心交易未收敛时，不以提高 AI 阶段完成度为目标。

**明确禁止**：
- 为每个新状态组合继续增加 `if`、recovery scanner 或前端特判。
- 为每种批量错误继续增加特殊 Ack/Nak，而不记录逐项进度。
- 为每个 AI 问题继续增加无统一归属的 Redis key。
- 为每条 RAG 路径继续增加局部缓存清理，而不统一快照版本。
- 引入 Kafka/Debezium、Temporal、独立向量库、第二套网格或 ORM 重写来解决上述问题。

---

## 八、证据索引与交叉验证

### 核心重构证据

| 范围 | 当前代码证据 |
|---|---|
| #20、#21、#24、#36 | `internal/services/aftersales/service.go:70-160,381-390,468-503`; `internal/services/aftersales/recovery.go:35-59`; `internal/services/order/refund_consumer.go:35-69`; `internal/services/order/consumer.go:35-110`; `internal/services/payment/service.go:245-330`; `frontend/apps/storefront-web/pages/after-sales/[no].vue:1-56` |
| #1、#2、#37、#3 | `internal/services/inventory/seckill.go:581-601`; `internal/services/inventory/repo.go:292-305`; `internal/services/inventory/tcc.go:37-148`; `internal/services/inventory/confirm_consumer.go:41-59`; `internal/services/inventory/consumer.go:39-64`; `cmd/catalog-stock-rebuild/main.go:21-82` |
| #23 | `internal/services/order/service.go:38,192-200,573-578`; `internal/services/fulfillment/consumer.go:35-64`; `internal/services/aftersales/service.go:517-520` |
| #4、#26-#31 | `internal/services/admin/promotions_handlers.go:209-236`; `internal/services/ai/session.go:20-85`; `internal/services/ai/service.go:145-341`; `internal/services/ai/cart_draft_store.go:15-80`; `internal/services/ai/tool_stream.go:20-70`; `internal/services/gateway/cart_draft.go:35-105`; `internal/services/cart/service.go:336-392` |
| #17 | `internal/services/ai/rag.go:131-253,351-478,1008-1119`; `cmd/ai/main.go:75-106`; `deploy/k8s/charts/mall/templates/ai-rag-build.yaml:92-130` |

**交叉验证结果**：
- **数字搬运**：通过**。RAG `10,000` 仅指 `100 页 × 100 条`，未用于其他规模结论。
- **分类边界**：通过**。申请超额不等于实际超额打款；多仓是产品门控；AI/RAG 是可选面；局部小修与结构重构分开。
- **汇总计算**：通过**。不再以 ID 数量或完成率衡量架构进展，也不把未运行验证视为零风险。
- **结论溯源**：通过**。每个重构主题均能追溯到上述当前实现；已完成主体未被重新包装成重构理由。