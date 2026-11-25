# REV-PAY-SETTLE-01-T1.6-R — B1/B2/B3 修复复审

| 项 | 值 |
|---|---|
| review_scope | `8dc1766..20df5b7` 的 T1.6-R：order.cancelled/compensation warehouse、PENDING authoritative Payment snapshot、Inventory compensation 持久化、legacy 净额。对照 Addendum `cfa0b0d` accepted、QC-28、T1.6 selfcheck。只读；不评审 T1.7/T5。不构成 Release PASS。 |
| base | `8dc1766`（T1.6 freeze） |
| code head | `20df5b7`（`wip(order,payment,inventory): T1.6-R B1/B2/B3 fixes`） |
| 契约 | ADR-0007 Amd1 T1.6 Addendum accepted r1.0；SPEC-0002 v1.9 AC-46/AC-65/AC-67/AC-68、INV-44/INV-46 |
| Tier / Actor | **L2**；Actor `dev-phase1i` ≠ Reviewed-by `reviewer-phase1` |
| **verdict** | **PASS（有范围）** |
| release_status | **NOT RELEASE**；不批准合并 |

---

## 1. BLOCKER

**无。**

## 2. B1（warehouse）关闭

- `orderItemsPayload`（`internal/services/order/service.go:1299-1318`）为每个 item 写 `warehouse_id`（>0 才写，缺失由 consumer fail-closed）；`queueInventoryCompensation`（`:742-770`）要求 `warehouse_id>0`，缺失返回 `INVENTORY_COMPENSATION_NO_WAREHOUSE`，不再默认 W1。
- Inventory `orderCancelledEvent` / `compensationEvent` 均增加 `warehouse_id`；`handleOrderCancelled` 和 `handleCompensation` 缺失 warehouse 时 `consumer.Permanent`，不构造无 WarehouseId 的 `pb.OrderItem`，不回退 W1。
- **W2 真实测试**：`TestOrderCancelledCompensatesHeldAndConfirmed` 增加 `held_W2` / `confirmed_W2` 子用例，均 PASS；`TestOrderCancelledMissingWarehouseFailsClosed`、`TestCompensationConsumerUsesWarehouse`、`TestCompensationConsumerMissingWarehouseFailsClosed` PASS。
- T1.6 CAS 成功写的 `order.cancelled` payload 来自 `loadOrderItems`，W2 能定位 reservation；B1 关闭。

## 3. B2（PENDING captured）关闭

- `TccCancelMarkPaid`（`internal/services/order/tcc.go:202-235`）对 `from==PENDING_PAYMENT` **无条件** `readTccPaymentSnapshot`；只有真正的 zero/no-payment identity（paymentNo 空 + PayCent=0 + settlement 空）走本地 cancel；`readTccPaymentSnapshot` 对 `zero:<order>` 身份本身返回 captured=0 快照，不调用 Payment（AC-67）。
- captured>0 → `cancelPendingCapturedAfterTCC`（CAS→CANCELLED + order.cancelled + `pay:<payment_no>:cancel`）；captured=0 → `cancelPendingAfterTCC` 本地取消；Payment 不可用返回错误，不猜金额。
- 测试：`TestT1_6PendingCapturedPaidCentZeroUsesAuthorityAndWarehouse`（PaidCent=0 + captured=10000）、`TestT1_6PendingCapturedPaymentUnavailableFailsClosed`（**PASS**）；`TestTccCancelPendingWithoutCaptureStillCancelsLocally` PASS。B2 关闭。

## 4. B3（补偿持久化/legacy 净额）关闭

- `refundCancelledWithSnapshot` 先要求 `s.inventory != nil`（nil 返回 `Unavailable`，不写 command）；`TccCancelConfirm` 失败时 `queueInventoryCompensation` 返回错误，调用方 `return Internal`，**不写 payment command**；`queueInventoryCompensation` 对 outbox append/warehouse 校验失败返回错误（`internal/services/order/payment_cancel.go:247-275`，`service.go:742-770`）。
- Order `MarkPaid` 的 partial-confirm compensation 也改为检查 `queueInventoryCompensation` 错误并阻断（`service.go:646-655`）。
- legacy cancel-after-confirm：`payment/consumer.go:96-135` 不再用 `PaidCent` 退；`paymentCancelNetCent` 调 `GetIntent` 取 authoritative `captured-refunded`，0/不一致/NotFound 不猜、不 fallback；Unavailable 重试。
- 测试：`TestT1_6CancelledCapturedRequiresInventory`、`TestT1_6QueueInventoryCompensationCarriesWarehouseAndReturnsError`、`TestT1_6CancelAfterConfirmInventoryFailureStillRefunds`、`TestT1_6CancelAfterConfirmCompensationAppendFailureBlocksRefund`、`TestT1_6RefundCancelledAfterConfirmRestocksAndPersistsCommand`、`TestHandleOrderRefundRequestedLegacyUsesNetNotPaidCent`、`...LegacyNetUnavailableRetries`、`...LegacyNetNotFoundDoesNotFallbackPaidCent` 全部 PASS。

## 5. 五裁决落实

| # | 裁决 | 状态 |
|---|---|---|
| 1 | CAS lost 不返回业务错误、继续组扇出 + reason/log/metric | **PASS**（`TestT1_6PendingCapturedCASLost...`；metric 注册/打点） |
| 2 | amount=captured-refunded 净额 | **PASS**（Order net；Payment legacy `paymentCancelNetCent`） |
| 3 | Inventory 失败仍写命令，补偿必须持久化 + 告警 | **PASS**（TccCancelConfirm 失败→queue；queue 失败→阻断；nil inventory→Unavailable） |
| 4 | 不扩 apperr/proto；reason/log/metric | **PASS** |
| 5 | `refundCancelledAfterConfirm` Payment 不可用 fail-closed | **PASS**（snapshot 失败返回 Unavailable） |

## 6. 边界 / 非阻断

- **无 proto/migrations/新 cancel 表/组级 RefundBatch/scan**：`git diff --name-only 8dc1766..20df5b7` 仅 order/payment/inventory/tests/selfcheck；无 `api/proto`、`migrations`、`refund_batches` 路径。
- **无 T1.7/T5**：diff 无相关路径。
- **已知残余 #5（`inventory.confirm.requested` 无 warehouse）**：**不属于 T1.6 BLOCKER**。T1.6-R 的 cancel 路径走 `order.cancelled`/`TccCancelConfirm`/`inventory.compensation_requested`，均已带 warehouse；`inventory.confirm.requested` 是 **paid confirm** 的既有路径（`afterPaid` 失败时 `queueInventoryConfirm`），不在本次 payment-level cancel 语义内。应**另立任务**（T1.2/T1.4 兼容或 T1.7/T5 收口），不要用 T1.6 补丁混入。
- **NATS broker e2e**：`TestOrderCancelledConsumerNATS` 因未设 `TEST_NATS_URL` **SKIP**，未计入 PASS；T5.2/T5.3 仍需。
- **metric**：`payment_cancel_cas_lost_total` 无 counter 值单测（selfcheck 已声明），[TODO]。
- **旧 order.cancelled/compensation 事件若缺 warehouse**：新 consumer 会 fail-closed/DLQ；当前无生产数据，迁移/重放需另立清理；不是 T1.6-R blocker。

## 7. 测试证据（Reviewer 独立复跑）

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0**，59.942s |
| `go test ./internal/services/order -run 'TestT1_6|TestTccCancel|TestOrderCancelled' -count=1 -v` | **PASS**，58.473s |
| `go test ./internal/services/payment -run 'TestHandleOrderRefundRequested|TestOrderRefundRequested|TestPayment' -count=1 -v` | **PASS**，0.863s |
| `go test ./internal/services/inventory -run 'TestOrderCancelled|TestCompensation|TestCancel' -count=1 -v` | **PASS**，4.643s（NATS 用例 SKIP） |
| `-race order`（TestT1_6/TestTccCancel） | **ok**，60.670s |
| `-race payment`（T1.6 handler） | **ok**，1.890s |
| `-race inventory`（OrderCancelled/Compensation） | **ok**，4.544s |
| `go vet` / `gofmt -l` | exit 0 / 无输出 |

- **W2 覆盖**：order.cancelled HELD/CONFIRMED W2、compensation W2、missing warehouse fail-closed 全部 PASS。
- **B2 覆盖**：PaidCent=0+captured=10000、Payment unavailable fail-closed PASS。
- **B3 覆盖**：nil inventory、queue append failure、TccCancelConfirm failure、legacy net/unavailable/notfound PASS。
- **SKIP 未当 PASS**：NATS 用例跳过，未计 PASS；AC-46/AC-65/AC-67/AC-68 未标 AC-VERIFIED。

## 8. 结论

- **verdict PASS（有范围）**：T1.6-R 的 B1/B2/B3 已关闭；五裁决落实；无 proto/migrations/组级 RefundBatch/scan/T1.7/T5 越界。
- **不是 Release PASS；不批准合并；不替代人类 merge 决策。**
- 残余 #5（`inventory.confirm.requested` 无 warehouse）确认为 **独立任务**，不阻塞 T1.6；建议另立任务补 paid confirm 多仓与测试。
- 报告：[REV-PAY-SETTLE-01-T1.6-R.md](docs/collab/qc-reports/REV-PAY-SETTLE-01-T1.6-R.md)
