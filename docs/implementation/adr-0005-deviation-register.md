# ADR-0005 / SPEC-0001 实施修订登记

| 项 | 值 |
|---|---|
| 记录人 | Developer |
| 日期 | 2026-11-22 |
| 依据 | SPEC-0001 v1.0、ADR-0005 accepted |
| 状态 | PO 已升版 SPEC-0001 v1.1；Architect 已回写 ADR-0005 §18 Amendment 1；Developer 待按 §18.13 落地 |
| 说明 | 按角色边界，Developer 不修改 `docs/specs/` 与 `docs/adr/`。本文件只登记实施中经人类批准的裁决、契约补充与兼容性解释，供后续 Agent 修订。 |

## A. 已获人类批准的业务口径修订

### A-01 AC-02 场景与 §6.1 / ADR §5.2 冲突

- **原文冲突**
  - SPEC-0001 §6.1：存在单仓可售 ≥ Q 时“不拆量”。
  - SPEC-0001 AC-02：仓 1=2、仓 2=5、购买 3，要求拆成仓 1×2 + 仓 2×1。
  - ADR-0005 §5.2 第 3 步目标函数第 1 项为“不同仓数量最少”，该场景必然选仓 2 单仓。
- **人类裁决（2026-11-22）**：以 §6.1 / ADR 单仓优先为准，修订 AC-02 场景。
- **Developer 实施**：分配器按“单仓足够不拆”；测试改为“两仓各 2、购买 3”验证拆 2 行（`TestAllocateOrderQuantitySplit`）。
- **需要的修订 Agent**：**Product Owner** 将 SPEC-0001 AC-02 场景与期望改为“无单仓满足、两仓合计满足”，升版并在变更记录注明影响的 INV/AC；**Reviewer** 在 PO 升版后按新 AC 重新基线。
- **原因**：不修订则 Reviewer 按原文判定 AC-02 FAIL，而代码遵守 §6.1 与 ADR。

## B. 需要 System Architect 回写的契约/设计

### B-01 分配审计缺少“各仓可用量”契约

- **实施现状**：`order_allocation_decisions.availability_json` 写入 `{}`；`AllocateOrderReply` 没有各仓 `available` 字段，Order 无法落 INV-28 要求的可用量快照。
- **需要的修订 Agent**：**System Architect** 在 ADR §5.1/§5.7/§8.1 定义 allocator 返回可用量的字段（或独立查询契约），再交 Developer 落地。
- **原因**：INV-28 要求可回答“各仓可用量”，当前契约无法提供；不能由 Developer 私自扩业务字段。

### B-02 选仓“业务优先级”无数据来源

- **实施现状**：ADR §5.2 要求候选按 `(business_priority, -available, warehouse_id)` 排序，但 ADR §9.7 又声明 Inventory 无 schema 变更，仓库业务优先级没有配置/表来源。当前实现退化为 `(-available, warehouse_id)`。
- **需要的修订 Agent**：**System Architect** 明确优先级来源（配置、表或固定默认值）并写入 ADR §5.2/§9.7。
- **原因**：没有来源时无法实现第 4 优先级，当前只保证确定性，不等于业务优先级。

### B-03 运费判责缺少结构化输入

- **实施现状**：只实现“未发货整单退款全额退运费”和“退货默认不退运费”；商家责任/错发/破损/丢件/部分数量商家责任无法判定。`rule_id` 仅落 `UNSHIPPED_CANCEL_FULL_FREIGHT` / `LINE_REFUND`。
- **需要的修订 Agent**：**Product Owner** 明确“谁在什么时点以什么字段判定责任”（客服/承运商/用户）；**System Architect** 定义责任字段、判责矩阵与 `rule_id` 枚举，回写 ADR §6.5/§8.1；之后 Developer 实现 AC-14 五场景。
- **原因**：当前售后请求只有自由文本 `reason`，不能把文本当判责依据，否则是发明业务规则。

### B-04 跨仓回库授权的强制校验落点

- **实施现状**：`restock_exception_authorizations` 与 `AuthorizeCrossWarehouseRestock` 已实现审计写入；自动回库始终回原仓，但 `Inventory.Restock` 尚未强制校验授权。
- **需要的修订 Agent**：**System Architect** 定义 AfterSales → Inventory 的授权凭证契约（授权号/签名/内部 RPC），并明确由 Inventory 拒绝未授权跨仓；Developer 再实现。
- **原因**：ADR §9.4 规定由 `Inventory.Restock` 校验，但授权表在 AfterSales schema，跨服务无法直接 JOIN；不能由 Developer 私自把校验留在 AfterSales。

### B-05 缺货默认退款的执行归属

- **实施现状**：`order_shortage_cases` 能在 48h 后置 `RESOLVED_REFUND` 并写事件，但没有实际创建组退款/售后单，也没有消费者执行退款。
- **需要的修订 Agent**：**System Architect** 定义 `shop.order.shortage.resolved.v1` 的消费者、退款发起方、幂等键与失败重试；Developer 实现。
- **原因**：ADR §3.9 要求“退款走组退款协议”，Order 不应直接调 Payment；当前只到决议，未到打款。

### B-06 超出 ADR §8.1 的 proto 字段

以下字段为实现必需但 ADR §8.1 未列出，需 Architect 回写并确认字段语义：

| 消息 | 新增字段 | 原因 |
|---|---|---|
| `CreateOrderInnerRequest` | `allocation_hash=17`、`package_count=18`、`warehouse_count=19` | DTM 分支只收到 inner payload，无法读取 quote；不传 hash 无法在支付冻结时校验/落快照 |
| `CreateShipmentRequest` | `carrier_code=7` | ADR §7.1 要求 Ship/Create 传受控承运商 code，但 §8.1 未列字段 |
| `PreviewRefundQuoteRequest` | `user_id=4`、`type=5` | 预览需校验订单归属；`type` 决定运费规则 |
| `ListLineRefundsRequest` | `user_id=2`、`admin=3` | S14 读模型需鉴权 |

- **需要的修订 Agent**：**System Architect** 更新 ADR §8.1/§8.2；**Developer** 保持 proto 兼容（只增不改）。

### B-07 事件 payload 契约未在 ADR 展开

- **实施现状**：已注册并校验 `shipment.tracked`、`shipment.exception`、`shipment.received`、`shipment.dispute_opened`、`shipment.dispute_resolved`、`aftersale.line_refunded`、`order.shortage_reported`、`order.shortage_resolved`、`order.allocation_frozen`。
- **需要的修订 Agent**：**System Architect** 在 ADR §8.5/`docs/contracts.md` 补 payload 必填字段与生产者/消费者；Developer 已按 registry fail-closed。
- **原因**：ADR §8.5 只列事件名，typed registry 需要显式字段契约。

### B-08 分配 hash 的规范序列化字段集

- **实施现状**：`allocation.Hash` 包含行号、SKU、仓库、组号、数量、单价、行优惠、行实付、行单价，以及组商品/优惠/运费分摊；ADR §5.3 只概括为“行号、SKU、仓库、组号、数量、金额”。
- **需要的修订 Agent**：**System Architect** 明确 canonical 字段集与顺序；否则 Pricing/Order 任一方调整字段都会造成全量 `retry_preview`。
- **原因**：hash 是跨服务契约，不能由 Developer 单方定义为事实标准。

## C. 兼容性/遗留路径（需 Architect 定移除标准）

### C-01 `QuoteGroup.freight_cent` 继续保留并与 `freight_share_cent` 同值

- **原因**：现有 admin/UI 与 `quote_groups.freight_cent` 列仍读取该字段；ADR §6.1 规定“组运费只写 freight_share_cent”。为不破坏旧读侧，当前把两者写同值，用户应付仍只读订单级 `freight_cent`。
- **需要的修订 Agent**：**System Architect** 决定废弃时间与读侧迁移；**Developer** 在旧读侧迁移后删除冗余写入。

### C-02 `Approve.amount_cent` 的 legacy 路径仍可调整金额

- **实施现状**：有行快照的售后按行计算，调用方金额只作展示；但历史单行/无行快照售后仍保留 `Approve.amount_cent` 改额路径。ADR §6.6 要求“审批只能按数量调整，直接金额调整返回 INVALID_ARGUMENT”。
- **需要的修订 Agent**：**System Architect** 给出历史数据迁移/兼容窗口的移除标准；**Developer** 在回填完成后删除该路径。
- **原因**：立即删除会让未回填的历史单无法审批；当前是显式兼容 shim，不是默认路径。

### C-03 `Receive` RPC 保留但已 fail-closed

- **实施现状**：`Fulfillment.Receive` 直接返回 `FAILED_PRECONDITION`，C 端 `POST /orders/:orderNo/receive` 已改调 `ConfirmReceipt`；proto 中 `Receive` 仍存在。
- **需要的修订 Agent**：**System Architect** 在 ADR/proto 标记 `Receive` deprecated 或删除计划；**Developer** 在下一轮移除旧 RPC 调用方。
- **原因**：保留 RPC 但禁止直接改收货，符合 E20；不删是为了避免破坏旧客户端编译。

## D. 不是“修订”，是本轮未完成项（需下轮 Developer）

- BFF/OpenAPI：承运商管理、争议、退款预览、缺货选择、Admin 对账路由与页面。
- §11 日终对账 job、差异表写入、Admin 聚合与指标告警。
- AC-14 五场景、AC-16 强制校验、AC-22/23/29/30 端到端 Playwright。
- 真实 NATS/DTM/Payment 超时、进程崩溃、NATS 重投注入。
- 分配决策 `availability_json` 的真实可用量（依赖 B-01）。

## E. 边界声明

本文件不改变 SPEC-0001 / ADR-0005 的效力；在 PO/Architect 回写前，代码按本文件登记的裁决执行。`git diff -- docs/specs docs/adr` 应为空，证明 Developer 未越权修改契约文档。

---

## F. Architect 回写（2026-11-22）

> 依据 [ADR-0005 §18 Amendment 1](../adr/0005-fulfillment-quantity-split.md)。A-01/B-03 已由 SPEC-0001 v1.1 处理；B-01/B-05 确认**未完成**。

| 项 | 裁决 | ADR 落点 | 状态 |
|---|---|---|---|
| A-01 AC-02 | 以 Spec v1.1 修正后的 AC-02 为准（单仓优先，2+2/3） | §18.11 | 已由 PO 升版，ADR 确认 |
| B-01 availability | **成立，未完成**；补 `AllocateOrderReply.availability`，Order 下单复算并落 `availability_json` | §18.2 | 待 Developer 落地 |
| B-02 优先级 | 配置 `INVENTORY_WAREHOUSE_PRIORITY`，默认 warehouse_id 升序 | §18.3 | 待 Developer 落地 |
| B-03 运费判责 | 采用 Spec v1.1 §6.5.2；责任枚举/字段/7 天挂起 | §18.4 | 字段与 worker 待落地 |
| B-04 跨仓授权 | 授权号 + `VerifyRestockAuthorization`，Inventory fail-closed | §18.5 | 待 Developer 落地 |
| B-05 缺货打款 | **成立，未完成**；AfterSales 消费 `shortage.resolved` 创建系统退款并走 refund task | §18.6 | 待 Developer 落地 |
| B-06 proto 字段 | 全部批准（只增不改） | §18.7 | 可按现有实现保留 |
| B-07 事件 payload | 必填字段见 §18.8；registry 收紧 | §18.8 | 待更新 producer/registry/contracts |
| B-08 hash 规范 | `allocation:v1` canonical JSON，共享实现 | §18.9 | 待加版本 + golden test |
| C-01 freight_cent | 只读别名，2026-12-31 前移除 | §18.10 | 保留兼容 |
| C-02 legacy 改额 | 仅 `line_no=0`，审计+指标，2026-12-31 硬移除 | §18.10 | 需补审计表/指标 |
| C-03 Receive | deprecated + fail-closed，2026-12-31 前移除 | §18.10 | 保留兼容 |

**判定**：在 B-01 与 B-05 的 ADR §18.2 / §18.6 落地并通过测试前，**INV-28 与 AC-30 不得判 done**；Reviewer 可按 §18.13 否决清单直接打回。

**二次审计**：ADR §19 已修正 9 处硬伤，并列出 P-01…P-16 待 PO/运营确认项。P-01…P-04 未裁决前，对应 AC 标 `[待确认]`，不得判 done。
