# 架构复审与协作治理决策（当前实现版）

**快照**: 2026-11-23。审查对象是当前代码、原始 hardening 要求、后续 Spec/ADR、协作线程和闸门实现，不沿用历史“done/PASS”结论。
**验证边界**: 本轮为静态复审，未运行 Go、前端、MySQL/NATS/Redis/DTM、Helm、集群或浏览器验证。历史测试记录只是交付声明，不等于本轮重新执行。
**核心判断**: 必须把两条交付线分开评价：
1. **非 SDD 基线**: `0 / 1A / 1B / 产品支持多仓`。主体机制已落地，但早期资金 Requirements 中的足额支付、支付后自动恢复和取消退款仍未闭环；取消竞态、履约 readiness 和系统退款准入也存在原始协议缺口。
2. **SDD/multi-agent 增量**: 跨仓拆单、按数量拆行、承运商签收、缺货和争议。Spec/ADR/代码/测试追溯骨架存在，但当前 “AC 引用 + 线程 PASS” 不能证明基线非回归，更不能证明发布就绪。

因此，下一阶段不是继续按问题编号打补丁，而是先收敛**支付结算与履约准入协议**，再统一**事件/恢复身份和多仓读模型**；协作机制保留目录化沟通，但必须把“线程关闭、代码合入、Spec 验收、发布放行”拆成四种不同结论。

---

## 一、验收结论：两条交付线分别判定

| 范围 | 本轮判断 | 证据与剩余边界 |
|---|---|---|
| 0: 支付配置 fail-closed | **配置主体完成** | 非法布尔值和非 dev mock 会拒绝启动；`internal/platform/config/config.go:275-299,433-446` |
| 0: AI Mesh 最小权限 | **静态未闭合，live 未验收** | broad namespace ALLOW 与不完整 DENY 并存；现有 UAT 只验证 Catalog 一个写方法；`deploy/k8s/charts/mall/templates/authorizationpolicy.yaml:12-29,71-90`；`scripts/check-ai-mesh.sh:12-32` |
| 1A: 订单/售后/支付/履约 | **主体存在，原始资金与恢复要求未通过** | 生命周期/退款投影、组织 permit、行台账、durable task 已存在；足额支付、TCC 取消竞态、支付自主结算恢复、统一退款准入和履约 readiness 未闭合 |
| 1B: 库存整单操作协议 | **主协议完成，恢复与履约组合未通过** | `operation_id + scope_hash + per-item progress` 已包含 SKU/仓/数量；取消、confirm/compensation 和 replay 丢仓，Order/Fulfillment 又未等待库存确认终态 |
| 产品支持多仓 (非 SDD 决策及基础能力) | **权威分配主体存在，读到读模型未通过** | Inventory 多仓分配存在；Catalog 在线投影和 rebuild 仍固定仓 1 |
| C: 跨仓拆单 phase 1-6 | **happy path 主体存在，不接受 functionally complete** | 报价分组、金额分摊、组级订单/包裹/售后/UI 已有实现；默认支付、事件身份和恢复链存在阻断问题 |
| Spec/ADR 0005: 按数量拆行、缺货、承运商签收 | **部分实现，不能发布验收** | 缺货默认退款和自动收货框架存在；系统退款履约准入、缺货终态、异常包裹拒收、争议售后和组收货终态未闭合 |
| docs/collab 协作机制 | **规则已升级为 v3，执行器仍有差距** | 状态语义、QC 门禁和 integration owner 已移入 `docs/collab/CONVENTIONS.md`；当前脚本能力边界见 §7.2，未闭环债务见本文 §9.1 |

早期需求不是本轮新发明：已明确规定成交金额与订单应付严格相等、Payment intent 反查订单、MarkPaid 失败且订单取消时自动退款，以及退款状态非空不阻断运单或推进发货。当前偏差说明非 SDD 阶段的整改台账没有持续追踪到代码终态，不能由后续 ADR-0005 的 AC 引用率补签通过。

文档也需纠偏：
- `docs/c-warehouse-review.md:100-106` 的 "functionally complete" 只能代表交付声明，不能覆盖当前代码中的默认支付、事件和恢复缺口。
- `docs/adr/0002-cross-warehouse-split-orders.md:3` 仍写 phase 5/6 未完成，与 readiness 文档不一致。
- `docs/adr/0005-fulfillment-quantity-split.md:1390-1403` 的 `shipment.received` 必填字段没有 `fulfillment_group_no`；`:233-243` 写“用户胜，撤销收货”，`:575-579` 又规定“不回滚 receipt”，需 Architect 先裁决。

---

## 二、非 SDD 架构缺口 A：支付事实、订单结算与履约 readiness 没有成为同一套协议

### 2.1 足额支付仍未由领域末端保证
- Payment `CreateIntent` 只检查订单号非空和金额为正，没有反查 Order；`internal/services/payment/service.go:91-143`。
- Order TCC Try/Confirm 对正数少付均接受，只拒绝超额；`internal/services/order/tcc.go:39-45,75-81`；直接路径的 `paidAmount` 同样只拒超额。
- 正常 BFF/Seckill 当前使用订单应付金额，因此本结论不是“HTTP 用户可直接少付”；它证明的是 Payment/Order 权威末端没有执行早期硬不变量，内部错误调用方或权限扩大时会被接受。

修复必须落在领域契约：首次支付要求足额，Payment 创建 intent 核验订单身份和应付金额；未来改价或部分支付另立协议，不得用 `paid <= pay_cent` 隐式支持。

### 2.2 Payment 已持久化 SUCCEEDED，但后续结算没有持久所有者
Callback 先在本地事务写 callback、intent 状态和 outbox，提交后才同步 `markPaidTCC`；`internal/services/payment/service.go:225-286`。DTM 全局事务又在 `markPaidTCC` 内才创建；`internal/services/payment/tcc.go:32-72`。

因此在“Payment 本地 SUCCEEDED 已提交、DTM 尚未受理”的窗口：
1. 进程崩溃后没有 DTM 工作流可恢复。
2. Payment 启动项只有 Outbox relay 和 RefundConsumer，没有 settlement worker；`cmd/payment/main.go:31-35`。
3. Order expiry worker 只按本地 PENDING_PAYMENT 取消，不核验 Payment captured；`internal/services/order/worker.go:15-42`；`internal/services/order/service.go:401-469`。
4. 重放 callback 确实能重试结算，但恢复因此依赖外部渠道或人工重放，不是系统自有义务。

在外部不再重放时，代码会留下“Payment captured、Order CANCELLED、没有自动退款驱动”的状态。外部渠道重试策略未读取，实际持续时间标 `[不确定]`，但缺少持久恢复入口是静态事实。

### 2.3 TCC Confirm 把取消竞态的 CAS 失败当成幂等成功
当前 Confirm 的顺序为：
1. 事务外预读订单，要求 PENDING_PAYMENT；`internal/services/order/tcc.go:59-74`。
2. 事务内执行 `PENDING_PAYMENT -> PAID` CAS；`RowsAffected=0` 直接返回成功；`:91-98`。
3. 无论 CAS 是否真实迁移，事务后都把预读记录改成 PAID 并调用 `afterPaid`；`:117-119`。

若人工/过期取消在预读与 CAS 之间提交，CAS=0 表示订单已 CANCELLED，不表示本次支付已幂等完成。Confirm 仍返回成功并执行库存/优惠副作用；Payment 在 TCC 成功时不会再进入 `refundUnpayable`；`internal/services/payment/tcc.go:73-86`，直接 MarkPaid 已在提交后重读并处理 CANCELLED；`internal/services/order/service.go:690-708`，默认 TCC 路径反而丢失了这层裁决。

### 2.4 默认 TCC 未统一分配冻结，订单级资金口径也未统一
- 直接 MarkPaid 已组合快照、冻结、状态迁移和 outbox；`internal/services/order/service.go:649-685`。
- TCC Confirm 只写普通状态和 `order.paid`，没有 `ensureAllocationSnapshotTx/freezeAllocationTx/allocation_frozen`；`internal/services/order/tcc.go:91-119`。已是 PAID 的重试也不会补冻结。
- Payment 退款事件的 refunded 汇总整单 intent，captured 却只取当前 intent；Order 用这两个不同口径判断 FULL；`internal/services/payment/service.go:359-384`；`internal/services/order/refund_consumer.go:47-69`。
- 第二个 captured intent 看到订单 PAID 后直接成功返回，不进入不可支付退款；`internal/services/payment/tcc.go:42-46`。

### 2.5 “order.paid” 不等于库存已经可履约
Order Confirm 在一个 TCC 分支写 PAID 和 `order.paid`；Inventory Confirm 是另一个可回退 Retryable/Failed 的分支；`internal/services/inventory/tcc.go:37-55`。因此 Order 分支成功时，库存不保证已经到达确认终态。

当前履约又没有独立 readiness：
- Fulfillment 直接消费 `shop.order.paid.v1` 创建 CREATED shipment；`internal/services/fulfillment/consumer.go:25-85`。
- Order 在支付时把组置 ALLOCATED，但 `reservation_state` 仍为 HELD；`internal/services/order/fulfillment_groups.go:69-79`。
- `groupActions` 不检查 reservation_state，PAID/ALLOCATED 且 permit 开放即给出 ship；`internal/services/order/settlement.go:146-177`。
- Ship 只向 Order 请求 permit，没有 Inventory readiness 检查；`internal/services/fulfillment/service.go:211-249`。

这证明当前一个事件同时承担“支付成功、库存已确认、允许履约”三种语义。不能简单给 Fulfillment 再加一个状态 `if`，应把事实拆开。

### 2.6 目标协议与完成门槛
对应的高层铁律已写入 `AGENTS.md` §4（持久恢复、四事实分离、多仓身份）；本节只保留实现顺序与验收矩阵。由 Architect 先修支付/履约 ADR，再统一实现：
1. Payment 在写 SUCCEEDED 的同一本地事务持久化 settlement task；worker 按 payment_no 幂等驱动结算，DTM 接管后记录其身份，CANCELLED/CLOSED 进入确定性的 `refundUnpayable`。
2. Order 的首次支付强制足额；CAS=0 必须重读权威状态，只有“同一 payment identity 已完成”才幂等成功，冲突/取消不得触发支付后副作用。
3. 直接/TCC 共用一个 settlement/freeze 原语，重验证快照、hash、冻结、订单状态和事件是否全部闭合。
4. Payment 明确 `order_captured_cent/order_refunded_cent` 与单 intent 字段；重复捕获必须有唯一胜者与非胜者退款协议。
5. 将 `PaymentCaptured`、`OrderSettled`、`InventoryReady`、`FulfillmentAllowed` 分为不同事实。Order/Fulfillment 的履约准入必须消费库存确认终态，不再从 `order.paid` 推导。

**验收**: 直接/TCC 同等故障注入；重放 Callback 提交后崩溃、Confirm 预读后取消、DTM 未受理、Inventory Confirm 持续失败、多 intent 不同金额/channel、重复回调 and 跨 intent 部分退款。最终核对 Payment、Order、Inventory、Pricing、Fulfillment 五方终态。

---

## 三、非 SDD 架构缺口 B：不同退款来源没有共用同一个履约准入与持久编排

组织 permit 是正确基础，但当前有三条不同入口：
1. **普通售后**: 先 RPC `BlockShipmentForRefund`，再开始 AfterSales 本地事务写 REFUNDING/task/outbox；`internal/services/aftersales/service.go:576-665`。若 Order 已提交，AfterSales 在本地提交前崩溃，durable task 不存在；显式重试可补建，但启动装配没有该中间态的自动扫描恢复。
2. **Payment TCC Cancel**: Order 直接置 REFUNDING 并写 `order.refund_requested`，没有调用或复用组织 Block；`internal/services/order/tcc.go:132-223`。
3. **SYSTEM 缺货退款**: AfterSales 直接创建 APPROVED 系统售后和 refund task，只发 `aftersale.created`；`internal/services/aftersales/shortage_consumer.go:148-205`。它没有普通路径的 Block，也不发 Fulfillment 所消费的 `aftersale.refund_requested`。

`AuthorizeShipment` 只拒绝订单级 FULL 和 REVOKED permit，不检查组级 REFUNDING/REFUNDED；`internal/services/order/settlement.go:289-337`。因此在多组订单中，缺货组系统退款后，若 permit 未撤销自己 有 CREATED 运单，代码仍允许该组取得发货授权。

目标不是“所有 REFUNDING 一律禁止发货”—已授权发货后的退款是合法业务；而是让所有退款来源进入同一协议：
1. AfterSales/系统来源先在本地持久化退款意图与任务。
2. worker 驱动 Order 组织级退款准入：Order 在 permit 行上裁决“未发货撤销”或“已授权后允许退款”。
3. 准入成功后才执行 Payment；Fulfillment 消费统一的组级履约撤销意图。
4. 普通售后、TCC 补偿、缺货系统退款共享同一状态与恢复语义；缺货决议本身仍由 Order 状态机负责。
5. 崩溃恢复从持久任务继续，不再依赖“跨服务 RPC 成功后进程没有退出”。

**验收**: 每个退款来源分别在 Block 前、Block 后、task 提交前、Payment 超时后注入崩溃；对 CREATED/SHIPPED、单组/多组和并发 Ship 验证唯一胜者。

---

## 四、1B 与多仓: 主操作身份正确, 恢复事件和展示投影仍是旧协议

### 4.1 恢复链没有沿用主键身份
`newOperationSpec` 已按 `(sku,warehouse,quantity)` 规范化并派生 scope hash；`internal/services/inventory/operations.go:133-282`。但：
- Order 的取消、confirm/compensation 公共 payload 只发送 SKU+数量；`internal/services/order/service.go:712-745,1237-1248`。
- 三类 Inventory consumer 重建为无仓库 item；`inventory/consumer.go:16-22,39-60`、`confirm_consumer.go:28-26,42-60`、`compensation_consumer.go:20-26,43-62`。
- 人工 replay 只接收 SKU，`op.item(sku,0)` 会选同 SKU 的第一个仓；`internal/services/inventory/operations.go:686-693,873-965`。

主 TCC 能识别 W2，持久重试和人工恢复却退回单仓。修复必须是版本化事件契约：新版最小身份包含 `(order_no,line_no/sku_id,warehouse_id,quantity)`；producer、registry、consumer、DLQ、replay 一次升级。多仓上下文缺仓必须 fail-closed，只有明确的历史单仓 v1 才能默认 W1。

### 4.2 组级收货也缺同一身份治理
- 用户、自动和争议驳回三类 received producer 均未赋 `fulfillment_group_no`；`internal/services/fulfillment/delivery.go:160-170,428-435,620-627,757-765`。
- Order 将空组归为 G1；`internal/services/order/settlement.go:251-257`。
- registry 只检查 key 存在，不检查业务非零值和组合身份；`internal/platform/events/registry.go:177-184,212-233`。

G2/G3 收货可推进 G1。历史“G2 receive passed”必须用真实 Fulfillment outbox producer 重验，不能只调用 Order 投影。

### 4.3 Catalog 仍是单仓读模型
Inventory 事件身份已是 `(sku_id,warehouse_id)`，Catalog 却永久忽略非仓 1；`internal/services/catalog/stock_consumer.go:39-43`；rebuild 同样跳过非仓 1。split 模式下 W2 变化不会更新 `skus.stock`，只能确认浏览/Read-Your-Writes 陈旧，不能据此声称权威库存错扣或超卖。

也不能只删除过滤条件：`skus.stock_version` 是 SKU 单值，不同仓版本直接 Last-Write-Wins 会互相覆盖。需要仓级投影或版本向量，再聚合 SKU 展示库存；在线 consumer 与 rebuild 共用同一聚合口径。

---

## 五、SDD 增量实现中的确定性阻断与状态机缺口

### 5.1 小范围、确定性发布错误

| 问题 | 当前事实 | 影响 |
|---|---|---|
| 订单列表读取损坏 | `orderColumns` 24 列，`listOrders` 只有 20 个 Scan 目标；`internal/services/order/repo.go:137-147,281-300` | 列表稳定返回列数不匹配错误 |
| 历史售后迁移可跑断 | 多 item 先统一 `line_no=0`，随后立即建立唯一键；`migrations/aftersales/0012_line_ledger.sql:4-14` | 历史多明细售后唯一键冲突，后续 backfill 无法执行 |
| Helm 开关不闭合 | 只给部分服务注入 split 配置，未注入 Inventory quantity flag；`deploy/k8s/charts/mall/templates/deployments.yaml:147-160` | Order/Gateway 与 Inventory 行为版本不一致 |

这些可以独立修，但必须随组合协议测试一起验收，不得用单测变绿替代发布矩阵。

### 5.2 receipt、异常物流、争议和缺货
- CREATED 可直接 ConfirmReceipt；争议驳回写 receipt 但不更新 shipment/取消 auto-confirm；`internal/services/fulfillment/delivery.go:334-424,718-780`。
- auto-confirm 多分支更新 job 后直接 return，事务未 Commit；`:533-599`；LOST/DAMAGED 只写 exception_state，T7 仍可能自动收货；`:243-267,586-628`。
- 仓库没有 `shipment.dispute_resolved` 的 AfterSales consumer；producer 和 registry 也未满足 ADR 的 dispute_id/status/专用时间字段；`delivery.go:702-710,767-775`；`internal/platform/events/registry.go:183-184`。
- 缺货用户选择 CAS 不检查 RowsAffected，仍发 resolved outbox；不校验选择期限，重复上报可重开已解决案件；`internal/services/order/shortage.go:73-84,104-167`。

这些应由一个 receipt 状态迁移原语和带 version/deadline 的 shortage CAS 状态机统一处理。没有独立 4h scanner 本身不是缺陷，因为创建案件时已同事务通知；真正要验证的是通知与案件事实可恢复。

### 5.3 售后行台账、任务 fencing 和回库授权
- 跨行余额和剩余额度查回 pending；永久失败又释放 pending/订单预算，人工重算不恢复；`internal/services/aftersales/refund_lines.go:51-100,199-227`；`workers.go:546-624`。
- Restock complete/release 不使用 owner；Refund 正常成功路径也只按 after_sale_no 置 DONE，二者均可能让失租旧 worker 继续写聚合和事件；`workers.go:140-235,365-368,457-508,597-608`。
- Restock Verify 不比较 quantity，请求缺 line_no、used_at 在库存成功前写入；`internal/services/inventory/restock.go:82-95`；`internal/services/aftersales/restock_auth.go:31-53`。

修复方向是统一“预占+已退/明确释放”的台账迁移，统一 lease-owner fencing 和完整行级回库授权，不是给每个 worker 增加特殊恢复分支。售后 worker 在数据库行锁内调用 Payment RPC 的持锁时间优化排在正确性之后。

---

## 六、8 的 AI Mesh: 不是继续扩 DENY 清单，而是收敛默认授权

`allow-ingress` 对 waypoint 使用 ALLOW，来源包含整个当前 namespace，路径含 `/*`；`deploy/k8s/charts/mall/templates/authorizationpolicy.yaml:12-29`。`deny-ai-writes` 对 ai-sa 使用 DENY，优先级正确，但 `aiDenyPaths` 未覆盖 PricingService 的 LockCoupon/UnlockCoupon/ConsumeCoupon/TCC 等写方法；`deploy/k8s/charts/mall/values.yaml:303-377`。

Pricing 的正常服务间 ALLOW 来源不含 ai，并不能抵消前面的 namespace-wide ALLOW。因此静态配置存在 AI 写权限漏洞面；前提是 ai-sa 被滥用或 AI Pod 被控制。**未证明普通 prompt 可直接调用这些 RPC，也未声称已发生安全事件**。

现有 `scripts/check-ai-mesh.sh:12-32` 只验证 AI 可 ListProducts、CreateProduct 返回 PermissionDenied，无法证明所有服务写面关闭。处理方式：
1. 收窄 broad ingress ALLOW 的来源，恢复显式服务间 ALLOW + AI 精确只读 ALLOW；DENY 作为第二道防线，而不是长期维护所有写 RPC 的 denylist。
2. 对 Catalog、Inventory、Pricing、Promotion、Order、AfterSales、Payment、Fulfillment 做全路径静态矩阵和 live 反例 UAT。
3. live UAT 未执行前，只能记为“支付配置主体完成、Mesh 未验收”。

---

## 七、已做正确的部分，不要推倒重来

1. **Payment 退款防线**：行锁、幂等键、captured 上限和同事务 outbox 应保留。`GetPaymentByOrder` 只聚合 SUCCEEDED/PARTIALLY_REFUNDED/REFUNDED intent，早期“PENDING 也计入 captured”的怀疑已被代码否证；`internal/services/payment/repo.go:233-236`。
2. **金额与分配**：优惠/运费最大余数法保持守恒守恒，下单按 `(sku,warehouse,quantity)` 重核；`internal/services/pricing/quote_groups.go:118-200`；`internal/services/order/allocation_validation.go:152-207`。
3. **库存整单操作**：operation ID、scope hash、逐项进度和终态事件方向正确，不需要换消息或事务平台。
4. **组织 permit**：它提供了发货与退款竞态的序列化边界；问题是部分入口绕过，不是 permit 模型本身错误。
5. **AfterSales durable task**：任务化和 Payment 幂等退款方向正确；问题是编排顺序、SYSTEM 准入和 fencing 未统一。
6. **Cart AI 确认**：业务确认、购物车和 outbox 已在 MySQL 本地事务完成，Redis 只是可恢复视图。
7. **RAG 基础方向**：版本化索引、独立构建 Job、follower 和 alias 方向正确，不需要另起向量平台。

---

## 八、AI 与 RAG: 先补协议身份，再扩展能力

AI 是可选面，优先级低于交易阻断项。

### 8.1 Turn/Action
当前 Chat 没有稳定 `turn_id/action_id`，session Load/Append 无 revision，同会话并发会按模型完成顺序写历史；`internal/services/ai/session.go:44-85`；`internal/services/ai/service.go:205-211,293-315`。草案可能从同用户 recent list 取回，feedback 也没有 turn 关联；`internal/services/ai/feedback.go:14-23`。

建议统一 envelope：
- `turn_id + session_id + expected_revision` 标识回答；`action_id + draft_id + idempotency_key` 标识工具/确认。
- 明确同会话串行或 CAS 冲突语义，逐步移除跨 turn recent-draft fallback。
- BFF 当前用 `WithoutCancel` 脱离 HTTP 取消；`internal/services/gateway/ai_handlers.go:39-54`。增加按 turn 的取消并传播到模型和工具，同时保留独立 deadline。
- 预算先明确软限制或硬上限；只有硬上限才做原子 reserve/settle。
- feedback 关联 turn/action；每日只统计前 5,000 条时显式返回 truncated 或独立计数；`internal/services/ai/feedback.go:65-108`。

### 8.2 RAG 发布生命周期
- 构建失败只记录并 return，进程以成功状态退出；`cmd/ai/main.go:88-103`。
- 构建/reconcile 最多扫描 100×100 条，却用不完整 seen 集删除未出现商品；`internal/services/ai/rag.go:356-379,1078-1114`。
- Upsert/Delete 写多个 active versions，却只清当前 writeIndex 缓存；`:1088-1075`。
- 版本注册表不按 index base 隔离，失败版本未注销，prune 只删注册不删索引文档；`:402-478,1046-1058`。

处理顺序：构建失败非零；扫描不完整禁止 destructive reconcile；缓存失效覆盖实际写版本；再补版本状态、base 隔离和回收。多副本 reconcile 可按 base 加短租约，不需要全局领导者选举。

### 8.3 扩展顺序

| 阶段 | 能力 | 门槛 |
|---|---|---|
| AI-0 | Turn/Action、会话 revision、取消、反馈、预算语义 | 不新增业务写权限 |
| AI-1 | 订单/售后/库存解释、异常证据链接、RAG 质量治理 | 领域 RPC 服务端重校验用户归属 |
| AI-2 | 比较、导购、Cart 草案并回原 checkout | 复用 Cart 唯一键+本地事务确认模式 |
| AI-3 | 取消订单、售后和运营处置草案 | 复用交易状态机；AI 不直接退款/收货 |
| AI-4 | Payment/Fulfillment 用户作用域只读解释 | Mesh 精确方法白名单，管理员和消费者身份分离 |
| AI-5 | 支付/履约写工具 | 单独 PRD/ADR、人工确认、金额上限、超时未知对账 |

多 Agent、模型路由、多模态和长期记忆放在对照 eval 证明质量/成本收益之后。

---

## 九、docs/collab: 规则已外移, 只保留未闭环事实

角色所有权、QC 门禁、线程协议、状态语义、integration owner、AC-REF/AC-VERIFIED 和提交规则已成为 canonical：`[CONVENTIONS.md](collab/CONVENTIONS.md)`；最高准则见 `[AGENTS.md](../AGENTS.md)`。本节不再复述规则、目标状态或改造方案，只保留本轮发现且尚未被上述文档吸收的具体事实。

### 9.1 仍未闭环的具体项

| 项 | 当前事实 | 落点 |
|---|---|---|
| 6 条 AC 未验收 | AC-22/28/29/30/35/37 仍缺 SLA、Playwright 或真实竞态证据；`docs/collab/ac-coverage.tsv:2-7` | 到期应由 G-4 阻断（ADR-0086 Amendment 1），当前脚本未实现 |
| 历史 PASS 措辞 | T-001 写“PASS，最终放行”并称缺口“不影响 PASS”；`docs/collab/developer-reviewer/threads/T-001.md:351-378` | 只能归于该轮代码修复审查，已按 CONVENTIONS §5 重新判定 |
| 浅层 AC 测试 | AC-38 只重复同一 allocation，未验证调整仓库优先级后历史超时不变；拆单并发/故障脚本未进门槛；`Makefile:174-202` | 由 Reviewer 人工判定，AC-REF 不能替代 |
| 被降级的金额验证 | P0-7 只断言 Payment 调用次数与 breakdown，有效多组金额序列被列为非阻断 TODO | 并入支付/退款组合验收 |
| 动态状态双事实源 | Architect MEMORY 同时写 T-003 待实现和“发布门禁已过”；`docs/architect/MEMORY.md:21-32`；Developer MEMORY 仍写线程 OPEN；`docs/developer/MEMORY.md:15-23` | CONVENTIONS §1 已禁止 MEMORY 复制动态状态，待清理 |
| 门禁范围缺陷 | G-1 对整个 range 的 Role 做并集；默认 `HEAD^..HEAD`；CI paths 不含 docs/prompt；`scripts/collab_gates.py:47-53,120-149`、`.github/workflows/go.yml:3-33` | 见 CONVENTIONS §7.2 能力边界 |

### 9.2 结论
上述项不改变本轮对交易链路的判断，但在 G-4、`--release` 和 AC-VERIFIED 落地前，**`make gates-strict` 全绿不能作为 Spec 全量验收或 Release PASS**。规则本身已由 CONVENTIONS v3 定稿，本文不再重复。

---

## 十、实施顺序与禁止事项

### 第一步: 阻断扩散并修正验收口径
- 保持 split/quantity/auto-confirm 等开关关闭，除非完成对应矩阵验证。
- 暂停把当前 `CLOSED/PASS`、38/38 和历史“发布门禁已过”作为发布凭证。
- 修订列表、迁移顺序和 Helm 渲染三项确定性错误。
- 指定支付/库存/履约组合 integration owner。

### 第二步: 先完成支付-订单-库存-履约协议设计
- 持久 settlement task、足额支付、CAS 冲突裁决、统一 freeze、订单级资金口径。
- 分离 PaymentCaptured、OrderSettled、InventoryReady、FulfillmentAllowed。
- 普通/TCC/SYSTEM 退款共用组织准入和持久编排。

这一步必须先改 ADR 和故障验收矩阵，再改代码；不能分给不同 owner 各自补一个条件后宣称完成。

### 第三步: 版本化事件、状态机和恢复收敛
- 统一库存/shipment/dispute 事件身份、registry、consumer、DLQ 和 replay。
- Fulfillment/shortage 统一 receipt 与 shortage 状态机。
- AfterSales 统一 pending 生命周期、lease fencing、人工重放和行级回库授权。
- Catalog 建立多仓投影与 rebuild 一致性。

### 第四步: 流程门禁和其余治理并行落地
- 兑现 ADR-0086、逐提交范围检查、AC-VERIFIED 和 release 证据绑定。
- Mesh 收窄 broad ALLOW 并做全路径 live UAT。
- 用历史多明细售后、已支付单的数据副本做迁移、回滚和旧镜像 smoke。

### 第五步: 再做 AI、HA 和规模优化
核心交易未收敛前，不以 AI 完成度、多副本数量或新运营功能替代正确性进展。

**明确禁止：**
- 给直接 MarkPaid 与 TCC Confirm 分别增加互不一致的冻结或取消补丁。
- 用“所有 REFUNDING 禁止发货”破坏已发货后的合法退款。
- 给每个事件单独补一个可选字段，同时保留多仓缺字段默认 W1/G1。
- 给每个 worker 增加特殊恢复分支，而不统一持久意图、owner/fencing 和状态。
- 用 Kafka、Temporal、独立 Schema Registry、新向量平台或 DTM 包裹所有本地动作来规避协议设计。
- 用更多文档状态、commit trailer 或正则覆盖率替代真实组合验证。

---

## 十一、分析记录与证据交叉验证

### 11.1 数据读取记录

本轮读取并交叉追踪了：
- 非 SDD 原始要求：`./trae/specs/harden-payment-inventory-seckill/spec.md:129-159,175-216`、`docs/ext-review.md`、`docs/architecture-baseline.md`、`docs/review-tracking.md`。
- SDD/交付：`docs/specs/0001-fulfillment-quantity-split.md`、`docs/adr/0002-cross-warehouse-split-orders.md`、`docs/adr/0005-fulfillment-quantity-split.md`、`docs/c-warehouse-review.md`。
- 支付/订单：`payment/service.go`、`payment/tcc.go`、`payment/repo.go`、`cmd/payment/main.go`、`order/service.go`、`order/tcc.go`、`order/worker.go`、`order/settlement.go`、`order/fulfillment_groups.go`。
- 库存/履约：`inventory/operations.go`、`inventory/tcc.go`、三类恢复 consumer、`fulfillment/consumer.go`、`fulfillment/service.go`、`fulfillment/delivery.go`。
- 售后：`aftersales/service.go`、`shortage_consumer.go`、`workers.go`、`refund_lines.go`、`restock_auth.go`。
- 多仓/AI：`catalog/stock_consumer.go`、`AI service/session/feedback/rag`、Mesh values/AuthorizationPolicy 和 live 检查脚本。
- 协作：CONVENTIONS、三组线程及 README、ac-coverage.tsv、四角色 MEMORY、ADR-0086、`scripts/collab_gates.py`、Makefile 和 CI。

### 11.2 逐项对照

| 契约/不变量 | 主键实现 | 另一入口/恢复/组合 | 判定 |
|---|---|---|---|
| 首次支付足额且 intent 反查订单 | BFF 使用订单金额 | Payment/Order 末端只拒超额 | 不闭合 |
| Payment captured 后由系统持久恢复 | 同事务保存资金事实 | DTN 在本地提交后才创建，无 settlement worker | 不闭合 |
| 支付与取消唯一胜者 | 直接 MarkPaid 重读并补偿 | TCC Confirm CAS=0 当成功并继续副作用 | 不闭合 |
| PAID 与 allocation freeze 同事务 | 直接 MarkPaid 已满足 | 默认 TCC Confirm 未执行 | 不闭合 |
| 库存 ready 后才允许履约 | Inventory Confirm 可重试/失败 | order.paid 迟发单，HELD 组仍可获 ship | 不闭合 |
| 所有退款先经组织 permit | 普通售后有 Block | TCC Cancel 与 SYSTEM 缺货绕过 | 不闭合 |
| 库存身份为 SKU+仓+数量 | operation spec 已满足 | cancel/confirm/compensation/replay 丢仓 | 不闭合 |
| 多仓展示库存一致 | Inventory 权威多仓 | Catalog/rebuild 忽略非仓 1 | 不闭合 |
| 组 RECEIVED 由对应组事件驱动 | consumer 支持 group | 三类 received producer 未赋值组号 | 不闭合 |
| 整单全退比较同一 captured/refunded 口径 | refunded 为整单汇总 | captured 为单 intent | 不闭合 |
| AC 有真实验收证据 | CI 有测试执行基础 | 闸门只数文本引用，六条 deferred 未验收，却被纳入最终放行表述 | 不闭合 |
| 发布审查绑定精确范围 | ADR-0086 已批准显式 range | 脚本无 `--release`，CI 默认 HEAD^..HEAD | 不闭合 |

### 11.3 分析步骤

1. 将非 SDD 原始 Requirements 与后续 Spec/ADR 分开，避免用新增 AC 倒推旧链路已正确。
2. 沿 Callback→Payment settlement→Order CAS→Inventory Confirm→Fulfillment permit 追踪完整时序。
3. 对直接/TCC、普通/TCC/SYSTEM 退款、主满意/持久重试、在线投影/rebuild、单仓/多仓做平行路径对照。
4. 独立审查 collab 契约、实际脚本、CI 线程和 MEMORY，区分文档承诺、机器能力和人工结论。
5. 将问题按支付结算、履约准入、事件身份、读模型和验收语义归并，删除 captured 混入 PENDING 等被代码否证的假设。

### 11.4 交叉验证结果

- **数字搬运**：通过**。24 列/20 Scan、100 页×100 条、5,000 条反馈、38 条 AC、6 条 deferred 均来自对应实现或台账；未把构造时序当成线上事故记录。
- **分类边界**：通过**。Payment captured、Order 生命周期、Inventory ready、Catalog 展示投影分开；普通/TCC/SYSTEM 退款分开；静态 Mesh 风险与 prompt 可利用性分开。
- **汇总计算**：通过**。38/38 只表示引用，6 条 deferred 未到期但仍未验收；Payment 整单 refunded 与单 intent captured 未被阅同一口径。
- **结论溯源**：通过**。每个阻断主题均有当前代码和原始要求或 ADR 对照；未将“未运行”写成“运行失败”，也未将历史测试声明写成伪造。

### 11.5 最终结论

当前实现应判定为：**0 仅支付配置主体完成；1A/1B 与产品多仓的主体机制存在，但原始资金、恢复、履约 readiness 和多仓读模型未验收；跨仓拆单和 Spec-0085 不具备发布条件；docs/collab 规则已升级为 v3，执行落差与未闭环债务见本文 §9.1。**

下一阶段优先完成支付结算与履约准入的统一协议，再处理事件/恢复与多仓投影；协作侧用最少元数据和状态语义把每次审查绑定到精确代码范围、基线影响和真实执行证据。完成这些以后，再扩 AI、HA 和运营能力，系统复杂度才会下降而不是继续上升。

【测试范围】当前源码、原始 Requirements、Spec/ADR、迁移、部署、协作线程与闸门实现的静态审查；多路独立只读复核。
【未覆盖范围】所有真实测试执行、历史数据分布、线上开关、集群配置、外部支付渠道重试、模型/承运商行为、Istio 实际转发与生产故障频率。