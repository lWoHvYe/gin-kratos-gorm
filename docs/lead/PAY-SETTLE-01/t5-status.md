# T5 状态：AC-60 / AC-63（2026-11-27）

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01`（T5） |
| 基线 | `0416bc6`（T5 测试已提交）；ADR-0011 已 PASS（`4f...` 待记） |
| 结论 | AC-60 主要路径可测；AC-63 的 `COMPENSATING→EXPIRED_REFUND` **被未批准契约阻断**；AC-63 不能标已验证 |

## 1. 已通过（本轮 T5 证据）

- **T5.4 真实迁移**：`TestT5RealMigrationSchemaAndAC60Backfill`
  - 新建 disposable DB，按序执行 `migrations/order/*.sql` 全部 22 个真实文件（非内联 DDL）；
  - 断言 `orders.settlement_payment_no`、commitment composition/lines、release、legacy queue/sequences 列存在；
  - 历史 PAID + 唯一捕获 intent → `BackfillLegacyPAID` 回填 `settlement_payment_no`。
- **AC-60**：`TestAC60HistoricalPaidReadBackfillRefundShip`
  - 读取历史 PAID 为已支付（不要求四事实）；
  - 唯一候选回填；
  - 有售后单 `refund_id=after_sale_no` 的退款准入 + 幂等重试；
  - 历史单发货：先解析生效支付，再以 Inventory ready 消费 permit。
- **AC-63 已实现分支**：`TestAC63LegacyResolutionAndQueue`
  - unique → RESOLVED；ambiguous → OPEN（候选 2、不写 settlement）；
  - no capture zero → EXPIRED_ZERO；
  - captured unshipped 24h → COMPENSATING（撤 NONE permit、不调用 Payment）。

## 2. 阻断：AC-63 完整补偿

AC-63 要求：撤 permit → `CancelIfCreated` → **INV-44 释放预留** → **Payment.Refund 最后** → `EXPIRED_REFUND`；失败保持 `COMPENSATING`。

现状（`legacy_resolution_worker.go:215-292`）：
- 已完成撤 `NONE` permit、`CONSUMED+CREATED` 的 `CancelIfCreated`；
- 到 INV-44 释放时返回 `errLegacyCompensationBlocked`，**保持 COMPENSATING，不写 release、不调 Payment、不置 EXPIRED_REFUND**；
- 原因是 release source 封闭枚举不含 legacy，且 release producer 只有 outbox **请求**，没有完成回执/ack/lease；Payment 若先执行会违反 AC-63 顺序。

两份 ADR addendum 冲突：
- `docs/adr/0007-amd1-addendum-legacy-release.md`：**DEFER** 到 T3；不加 `legacy_expired`；要求 release receipt/ack 才能 Payment。
- `docs/adr/0007-amd1-addendum-r0.4-c04-release.md` §2.3.2：**选定 A**：新增 `source=legacy_24h` + `reason=LEGACY_24H_EXPIRY`；但明确要求**独立 Amendment**（owner Architect）先落地，且 consumer 先发布，producer 后才能发；当前该 Amendment 不存在。

因此 AC-63 不能由实现自创 source/claim release 完成。

## 3. AC-60 剩余缺口（非阻断本轮）

- 无售后单时由 Order 自动分配 `legacy:<order_no>:<refund_seq>`：`nextLegacyRefundSeq`/`legacyRefundID` 已交付，但 `AdmitRefund` 仍要求调用方传 `refund_id`；没有“空 refund_id 自动分配”的入口。
- 发货 Inventory 分支：`checkOrderReady` 对 HELD/未 ready 已 fail-closed，但“HELD 先确认/补偿、无法确认走缺货/退款”的完整流程未在 `AuthorizeShipment` 接线；需要单独的缺货/补偿路径。

## 4. 需要人类裁决

**AC-63 的 release 契约选哪条：**

- **A. 批准 `legacy_24h` + release 完成回执/lease/ack**
  - 新增独立 ADR/Amendment；
  - 新增 release receipt/task（或等价 ack）与 lease，Inventory consumer 先发布；
  - Order 在 ack 后才 `Payment.Refund`，失败保持 COMPENSATING；多组仍 manual；
  - 可闭合 AC-63，但范围最大。
- **B. 保持 AC-63 DEFERRED**
  - COMPENSATING 作为人工/财务终态；修改 SPEC AC-63 验收状态/说明（契约变更）；
  - T5 聚焦 AC-60/其他 AC；不声称 AC-63 已验证。
- **C. 直接 Inventory Release RPC + ack**
  - 不走 outbox source；新增同步释放接口与回执；
  - 改变现有 INV-44 异步契约，需独立 ADR。

**推荐**：若目标是关闭 SPEC-0002 全量，选 **A**；若本轮要可发布，选 **B** 并把 AC-63 明确 DEFERRED。无论选哪条，AC-63 在裁决前保持**未验证**。

## 5. 本轮工作树（已提交 `0416bc6`）

- `internal/services/order/t5_migration_integration_test.go`
- `internal/services/order/t5_ac60_ac63_integration_test.go`
- `internal/services/order/settlement_integration_test.go`（迁移 runner 支持 `DELIMITER`）
- 测试均 PASS；无生产代码/迁移/契约改动。
