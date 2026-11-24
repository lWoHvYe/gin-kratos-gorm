# REV-PAY-SETTLE-01-T1.6 — Payment-Level Cancel 复审

| 项 | 值 |
|---|---|
| review_scope | `6214146..873550a` 的 T1.6：PENDING_PAYMENT+captured CAS→CANCELLED + order.cancelled + `pay:<payment_no>:cancel`；cancel-after-confirm；Payment consumer command/observe-only；C-04 optional；metric/selfcheck。对照 Addendum `cfa0b0d`（accepted r1.0）、QC-28、T1.6 selfcheck。只读；不评审 T1.7/T5。不构成 Release PASS。 |
| base | `6214146`（T1.5 TCC bypass removed） |
| code heads | `e9643ff`（payment-level cancel）、`873550a`（metric/selfcheck）；Addendum `cfa0b0d` accepted |
| Tier / Actor | **L2**；Actor `dev-phase1i` ≠ Reviewed-by `reviewer-phase1` |
| **verdict** | **FAIL（有范围）** |
| release_status | **NOT RELEASE**；不批准合并 |

---

## 1. BLOCKER（3 条）

### B1 [BLOCKER] `order.cancelled` / Inventory 补偿丢失 `warehouse_id`，多仓 reservation 不释放/不回库

- **位置**：`internal/services/order/service.go:1279-1290` 的 `orderItemsPayload` 只写 sku/name/price/quantity/discount，**没有 `warehouse_id`**；`internal/services/inventory/consumer.go:16-21` 的 `orderCancelledEvent.Items` 只有 sku/quantity；`:47-51` 构造 `pb.OrderItem{SkuId,Quantity}` 无 WarehouseId，`normalizeWarehouse` 回退 W1。`service.go:742-756` 的 `queueInventoryCompensation` 同样只写 sku/quantity；`inventory/compensation_consumer.go:51-56` 亦然。
- **事实**：`cancelPendingCapturedAfterTCC`（`payment_cancel.go:111-153`）成功后在同一事务写 `order.cancelled`，其 payload 使用 `orderItemsPayload`，**不带 warehouse**；多仓订单的 W2 reservation 被 Inventory consumer 按 W1 处理（找不到 W1 行即 skip），W2 保持 HELD/CONFIRMED，INV-44/INV-46 未完成。`refundCancelledWithSnapshot` 的 TccCancelConfirm 本身带 warehouse，但失败后的 `queueInventoryCompensation` 同样丢失 warehouse。
- **Reviewer 实际证明**：overlay 在 W2 放 `stock_reservations(ORD-W2,sku1,W2,HELD,1)`，调用现有 `handleOrderCancelled` 处理只含 `{sku_id,quantity}` 的 order.cancelled payload，W2 reservation 仍 `HELD`（`TestReviewerOverlayOrderCancelledMissingWarehouseLeavesW2Held` 通过是因为它断言了 HELD）。
- **判定**：T1.6 依赖的既有 order.cancelled/compensation 路径在多仓下无法定位 reservation，属于库存数据完整性阻断。需要 event/consumer/compensation payload 显式携带 `warehouse_id`（并补 W2 测试）；不是加本地 if 能解决。**打回 Dev/Architect 的 event payload**。

### B2 [BLOCKER] PENDING+captured 仅以 `paid_cent>0` 触发，`paid_cent=0` 时丢失 authoritative capture 与退款命令

- **位置**：`internal/services/order/tcc.go:197-218`：`paid := req.GetPaidCent(); if paid <=0 { paid = record.PaidCent }`；只有 `from == statusPendingPayment && paid > 0` 才 `readTccPaymentSnapshot` 并进入 `cancelPendingCapturedAfterTCC`。`statusPendingPayment` 的 `orders.paid_cent` 在 AC-53 下仍为 0；如果 TCC request 的 `PaidCent` 为 0（或调用方只传 `/cancel` 语义），代码走 `tccCancelLocal`，只本地取消 + order.cancelled，**不读 Payment captured、不写 `pay:<payment_no>:cancel`**，已捕获资金不会退。
- **Reviewer 实际证明**：overlay 用 fake Payment `captured=10000`、PENDING order、`TccCancelMarkPaid(PaidCent:0)`；结果 order CANCELLED、1 个 `order.cancelled`、**0 个 `order.refund_requested` 命令**（`TestReviewerOverlayPendingCapturedPaidZeroLosesCommand` 通过是因为它断言 0）。
- **判定**：Addendum §2.1 要求读取权威 Payment captured/refunded 后决定 PENDING+captured；当前触发条件依赖请求/订单快照的 paid，违反 §4.2 裁决 2/5 的 authoritative 语义。应在 `from==PENDING_PAYMENT` 时**无条件**读取 Payment snapshot（或先读 snapshot 再判断 captured），不能以 `paid_cent` 作为是否进入 T1.6 的判据；补 PaidCent=0 + captured>0 用例。资损阻断。

### B3 [BLOCKER] Inventory 补偿失败被静默吞掉；legacy 金额未取净额

- **位置 A（补偿持久化）**：`internal/services/order/payment_cancel.go:277-285` 在 `s.inventory == nil` 时直接跳过 TccCancelConfirm 并继续写 payment command；`internal/services/order/service.go:742-756` 的 `queueInventoryCompensation` 在 outbox append 失败时只 `s.log.Error`、**返回 void**，`refundCancelledWithSnapshot` 随后仍写 payment command。Addendum §4.2 裁决 3 要求“Inventory 补偿必须持久化 + 告警；命令写入失败则回滚并返回错误，不得静默”。当前 compensation append 失败没有返回/回滚，Stock 无 durable retry，payment 仍退。
- **位置 B（legacy 净额）**：`internal/services/payment/consumer.go:96-105` 对 legacy cancel-after-confirm 在 `AmountCent<=0` 时回退 `PaidCent`；Addendum §4.2 裁决 2 要求 `amount=captured-refunded` 净额。旧事件已存在部分退款时，`PaidCent` 是 captured 全额，Payment.Refund 会按剩余 cap 拒绝（`ALREADY_REFUNDED`），consumer ack，**剩余净额不会被退**；这不是新的 0 金额路径，而是 legacy 兼容的净额缺口。
- **判定**：裁决 3 的 durable compensation 失败不可静默；裁决 2 的 legacy 金额需在 Payment consumer 用权威 net（例如先读 Payment intent 或 event 明确 net；不能用 paid 全额）。两项均影响资金/库存一致性，合并为 B3。

---

## 2. 已通过 / 关闭

- **CAS 成功**：`cancelPendingCapturedAfterTCC` 锁 order、CAS PENDING→CANCELLED、同 tx order.cancelled + payment command；**不建** batch/application/admission/commitment/permit；`TestT1_6PendingCapturedCASWins` 断言 CANCELLED、1 个 order.cancelled、1 个命令、无组 ledger、未直接调 Payment。
- **CAS 丢失/PAID**：`TestT1_6PendingCapturedCASLostToPaymentContinuesGroupFanout` PASS；无 payment-level command/order.cancelled，进入组扇出；log/metric reason `CANCEL_CAS_LOST`。
- **outbox 失败**：`TestT1_6PendingCapturedOutboxFailureRollsBack` PASS；order 仍 PENDING、version 不变、无 order.cancelled/command/status log/batch。
- **重复/并发**：`TestT1_6ConcurrentPendingCapturedCancelSingleWinner`（-race）PASS；同一 `pay:<payment_no>:cancel` identity。
- **CANCELLED+captured / cancel-after-confirm**：`TestT1_6CancelledCapturedWritesCommand`、`TestT1_6RefundCancelledAfterConfirmRestocksAndPersistsCommand` PASS（W1）；权威 captured 800 覆盖请求 1000；命令持久化。
- **无 captured/net 0**：`TestT1_6PendingCapturedAlreadyRefundedWritesNoCommand`、`TestT1_6RefundCancelledAfterConfirmWithoutCaptureWritesNothing` PASS；不写命令。
- **Payment consumer**：command/observe-only、legacy 映射、transient retry、deterministic reject、already-refunded ack、`IdempotencyKey=refund_id`、`NonEffectiveCapture=true` 均 PASS；TCC/未知 observe-only。
- **C-04 optional**：`order.refund_requested` 的 `refund_id/source/non_effective_capture/amount_cent` 可选，legacy/minimal/new payload 兼容；`TestOrderRefundRequestedPaymentCancelFieldsAreOptional` PASS。
- **metric**：`payment_cancel_cas_lost_total{reason,result}` 注册、CAS-lost 分支打点；**无 counter 值单测**（selfcheck 声明，[TODO] 非阻塞）。
- **order.cancelled HELD/CONFIRMED**：W1 用例 `TestOrderCancelledCompensatesHeldAndConfirmed` PASS；多仓见 B1。
- **边界**：无 proto/migrations/新 cancel 表/组级 RefundBatch；Payment 走既有 `Refund`；无 T1.7/T5；Addendum/selfcheck canonical；Tier L2。

---

## 3. 测试证据（Reviewer 独立复跑）

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0**，63.983s |
| `go test ./internal/services/order -run 'TestT1_6|TestTccCancel' -count=1 -v` | **PASS**，50.593s |
| `go test ./internal/services/payment -run 'TestHandleOrderRefundRequested|TestOrderRefundRequested|TestPayment' -count=1 -v` | **PASS**，0.806s |
| `go test ./internal/services/inventory -run 'TestOrderCancelled|TestCompensation|TestCancel' -count=1 -v` | **PASS**，2.622s（`TestOrderCancelledConsumerNATS` **SKIP**：未设 TEST_NATS_URL） |
| `-race order`（TestT1_6/TestTccCancel） | **ok**，54.061s |
| `-race payment`（T1.6 handler） | **ok**，1.482s |
| `go vet` / `gofmt -l` | exit 0 / 无输出 |
| Reviewer overlay：PENDING+captured, PaidCent=0 | CANCELLED + 1 order.cancelled + **0 payment command**（B2） |
| Reviewer overlay：W2 reservation + order.cancelled payload 无 warehouse | W2 仍 HELD（B1） |

- **SKIP 未当 PASS**：NATS broker e2e 因 `TEST_NATS_URL` 缺失跳过，未计入 PASS；T5.2/T5.3 仍需。
- AC-46/AC-65/AC-67/AC-68 未标 AC-VERIFIED。

---

## 4. 非阻断 / 已知

1. **T1.6 selfcheck** 已绑定本 WIP 提交；Addendum accepted r1.0；Tier L2；allowed_paths/baseline_impact/Spec-Ref 齐全。
2. **`payment_cancel_cas_lost_total`** 无 counter 值单测；代码注册/增量点有，属 [TODO]。
3. **legacy `PaidCent` 回退**（B3-B）需补；新 producer 的 `amount_cent=net` 正确。
4. **NATS broker e2e** 未跑；T5.2/T5.3。
5. **`queueInventoryCompensation`** 缺失 warehouse 与 append 错误静默（B1/B3）。
6. **`s.inventory==nil`** 在 `refundCancelledWithSnapshot` 直接跳过补偿；生产 wiring 应 fail-fast，测试/缺失 wiring 不应静默。

---

## 5. 结论

- **verdict FAIL（有范围）**：T1.6 的 CAS/outbox/consumer 主链和多数测试通过；但 **B1 多仓 order.cancelled 补偿丢 warehouse**、**B2 PENDING captured `PaidCent=0` 丢失退款命令**、**B3 补偿失败静默/legacy 金额非净额** 仍阻断关闭。
- **不批准合并；不是 Release PASS；不替代人类 merge 决策。**
- B1 需 event/consumer/compensation payload 加 `warehouse_id`；B2 需 PENDING 无条件读 Payment snapshot；B3 需 queue 失败返回/持久化、legacy 净额。修复后按精确 head 重审。

报告：[REV-PAY-SETTLE-01-T1.6.md](docs/collab/qc-reports/REV-PAY-SETTLE-01-T1.6.md)
