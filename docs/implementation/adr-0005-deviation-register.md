# ADR-0005 / SPEC-0001 v1.2 实施偏差登记

| 项 | 值 |
|---|---|
| 记录人 | Developer |
| 日期 | 2026-11-22 |
| 依据 | SPEC-0001 v1.2、ADR-0005 v1.2（§18/§19）、T-001 |
| 状态 | B-01/B-02/B-04/B-05/B-08 已实现；残余项见下表，每项有 owner + expiry |
| 边界 | Developer 未修改 `docs/specs/**`、`docs/adr/**`；`git diff -- docs/specs docs/adr` 为空 |

## A. 已关闭项

| ID | 事项 | 证据 | owner | expiry |
|---|---|---|---|---|
| A-01 | AC-02 以 SPEC v1.2 的“无单仓可满足才拆量”为准 | `TestAllocateOrderQuantitySplit`、`TestAllocateOrderSingleWarehouse` | Product Owner（已升版） | closed |
| B-01 | `AllocateOrderReply.availability` + Order 下单复算 + `availability_json` 非空 | `TestAllocationDecisionRecordsAvailability`、`TestAllocationRevalidationRejectsChangedPlan` | Developer | 2026-12-15（回归窗口） |
| B-02 | `INVENTORY_WAREHOUSE_PRIORITY` 配置与 `(priority,-available,id)` 排序 | `TestAllocateOrderUsesConfiguredWarehousePriority` | Developer | 2026-12-15 |
| B-03 | 责任枚举/原因码/证据、自动落值、168h 运费挂起、判责 RPC | `TestAC14FreightRefundSixScenarios`、`TestAC31To33FreightHoldTimeout` | Developer | 2026-12-15 |
| B-04 | 授权号 + `VerifyRestockAuthorization` + Inventory fail-closed | `TestAC16CrossWarehouseRestockRequiresAuthorization` | Developer | 2026-12-15 |
| B-05 | 缺货 `RESOLVED_REFUND` → AfterSales 系统退款单 + refund task + 可见性 | `TestAC30ShortageCreatesSystemRefundAndPays`、`TestGroupShortageChoiceAndDefaultRefund` | Developer | 2026-12-15 |
| B-08 | `allocation:v1` canonical JSON + 固定 golden hash | `TestAllocationHashV1Golden` | Developer | 2026-12-15 |

## B. 兼容/残余项（到期不清理升级 BLOCKING）

| ID | 事项 | 原因 | owner | expiry |
|---|---|---|---|---|
| B-06a | `CreateShipmentRequest.change_reason` 与 `SubmitReturnTrackingRequest.return_carrier_code` 未落地 | 不是本轮 B-01/B-05 阻断项；换单入口在 Admin BFF 尚未开放 | Developer | 2026-12-15 |
| B-07a | `docs/contracts.md` 尚未同步 §18.8 全部事件必填字段 | typed registry 已 fail-closed；文档属 Architect 契约区 | System Architect | 2026-12-15 |
| C-01 | `QuoteGroup.freight_cent` 只读别名保留 | 旧 admin/UI 读侧未迁移；ADR §18.10 要求 2026-12-31 前移除 | Developer | 2026-12-31 |
| C-02 | `Approve.amount_cent` 仅历史 `line_no=0` 可改额 | 已加 `after_sale_legacy_amount_adjustments` 审计；新单已拒绝；ADR 要求 2026-12-31 硬移除 | Developer | 2026-12-31 |
| C-03 | `Fulfillment.Receive` 保留但 fail-closed | BFF 已改调 `ConfirmReceipt`；ADR 要求 2026-12-31 前删 RPC | Developer | 2026-12-31 |
| D-01 | 缺货选择/退款进度的 C 端按钮与 OpenAPI 路由未接 | 后端 `ShortageCase` 已返回 `refund_state/refund_after_sale_no`；BFF 只读展示尚未加 | Developer | 2026-12-15 |
| D-02 | 日终对账 §11、真实 NATS/DTM/Payment 超时注入未跑 | 非本轮 T-001 顺序 1/2；已列入测试未覆盖范围 | Developer | 2026-12-15 |

## C. 不变量声明

- 未用 `// TODO` 掩盖 E1…E25、B-01、B-05；未用 `try-catch` 吞分布式事务异常。
- B-01 失败时 `stock_changed=true`，BFF 同响应返回完整 `next_preview`；不再只回 `retry_preview`。
- B-05 的退款执行走 AfterSales durable refund task，不在 NATS handler 内直接打款。
- 缺货四小时/48 小时字段和 `AUTO_T7` 文案、60 秒申诉宽限、已确认后 15 天争议不回滚均已实现。
