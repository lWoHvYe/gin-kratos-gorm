# REV-PAY-SETTLE-01-T1.4b2 — T1.4b2a/b/b2 复审

| 项 | 值 |
|---|---|
| review_scope | `401a67d..3597989` 的 T1.4b2a（INV-44 release event/Inventory consumer）、T1.4b2b（zero durable task）、T1.4b2b2（zero create/approve/system entry）。对照 SPEC-0002 v1.9、ADR-0007 Amd1、T1.4b2 design/QC-27；只读代码/契约/迁移，不评审 T1.5/T1.6/T1.7/T5。不构成 Release PASS。 |
| base | `401a67d` |
| heads | `d5c4a7f`（release outbox/Inventory consumer）、`2a8bc17`（zero task）、`3597989`（zero entry） |
| 契约 | INV-44、AC-50/AC-67/AC-68、AC-65；C-04 release event（无 proto）、C-05 无迁移；design DRAFT（T1.4b2 C-04 release event 不需单独人类 Approve，仍需 QC/Architect） |
| Actor / Reviewed-by | `reviewer-phase1`（Reviewer）≠ `dev-phase1g/h`（Developer） |
| **verdict** | **FAIL（有范围）** |
| release_status | **NOT RELEASE**；不批准合并 |

> **工作区状态**：`3597989` 之上存在未提交的代码/测试改动（`internal/platform/events/registry.go`、`internal/services/order/refund_release.go`、`internal/services/order/refund_release_integration_test.go` 的 source 校验/no-item 测试）。这些**不在** `401a67d..3597989` 的 HEAD 中；本轮 build/test 使用当前工作区，故包含了它们。B1 与这些未提交改动无关；B2 的 head source 行为仍按 `3597989` 判定。

---

## 1. BLOCKER（最多 3；仍阻止 T1.4b2 关闭）

### B1 [ARCHITECTURAL_FLAW] — group 级 release item 与 order/sku/warehouse 级 reservation 的 quantity 不匹配

- **位置**：Order `internal/services/order/refund_release.go:46-90` 按 **fulfillment group** 聚合 item（`order_items WHERE fulfillment_group_no=?`）；Inventory `internal/services/inventory/repo.go:247-253` 的 `releaseTx`、`internal/services/inventory/tcc.go:95-96`、`internal/services/inventory/repo.go:183-184` 的 cancel 均要求请求 `quantity == stock_reservations.quantity`；`internal/services/inventory/refund_release_consumer.go:63-95` 只按 `(order_no, sku_id, warehouse_id)` 找 reservation，然后调用 full-reservation 的 `Release`/`TccCancelConfirm`。
- **事实**：Inventory reservation 的唯一键是 `(order_no, sku_id, warehouse_id)`（`migrations/inventory/0016_reservation_warehouse_keys.sql`），quantity 是该 order/sku/warehouse 的总量；而 Order event 的 item quantity 是 **group** 的数量。一个 group 跨仓/同仓与其他 group 共享 reservation 时（`TestAdmitRefundPreWritesReleaseOutbox` 已显示 G1 可跨 warehouse 1/3；设计也允许同组跨仓），group item quantity 可能小于 reservation quantity。此时 `Release`/`TccCancelConfirm` 返回 `INVALID`，consumer 将其判 `Permanent` 并 DLQ；**INV-44 的物理释放不会执行**，reservation 保持 HELD/CONFIRMED，stock.reserved 不下降。
- **Reviewer 实际证明**：以 overlay 注入（不改仓库）`stock_reservations(ORD-QTY, sku1, wh1, quantity=2, HELD)`，group event item quantity=1；`handleRefundRelease` 返回错误，reservation 仍 `HELD`、`reserved=2`。即“子集 group”根本无法释放；测试名 `TestReviewerOverlaySubsetGroupCannotReleaseReservation` 通过（通过是因为它断言了失败，而不是成功）。
- **测试缺口**：`internal/services/inventory/refund_release_integration_test.go` 的每个 reservation quantity 都等于 event item quantity；`MultiWarehouse` 是两个独立 reservation；Order release test 只有一个 group。没有“两个 group 共享同 (order,sku,warehouse) reservation，只退其中一个 group”的用例。
- **契约判定**：这不是加 if 能修的问题。Event 携带 group，但 Inventory 的释放身份是 order/sku/warehouse 且只接受 full reservation；release 语义无法表达 group 子集。需要 **Architect** 决定：event 是否改为 reservation/full quantity（且只在所有相关 group 都 PRE 时释放），或 Inventory 增加 group/partial release 台账；并补上述用例。**Dev 不得在本分支硬改**。此项直接违背 INV-44/AC-50 的物理释放语义。

### B2 [BLOCKER] — system_shortage source/owner 未产生；head 对未知 source 静默归为 aftersale

- **位置**：`internal/services/aftersales/workers.go:389-392` 对所有正金额售后（包括 `createSystemRefund` 创建的 `after_sales.source=SYSTEM` 缺货退款）固定 `AdmitRefund Source:"aftersale"`；Order `refundReleaseSource`（`3597989:internal/services/order/refund_release.go:29-39`）只映射 tcc/system_zero/system_shortage/aftersale，`default` 返回 aftersale；`system_shortage` 分支存在但没有生产调用方。
- **事实**：缺货 SYSTEM 退款的 release event 实际为 `source=aftersale`、`reason=AFTER_SALE_REFUND`，而不是 design §2.1/registry 的 `system_shortage`/`SHORTAGE_REFUND`。这使 INV-44 的 **owner/source 身份错误**，审计和后续按 source 的归属/告警会误判；`system_shortage` 枚举成为死代码。head 对 `payment`/`legacy_expired`/未知 source 也静默归为 aftersale，未 fail-closed；当前工作区未提交的 source 校验只修了未知 source，**没有**修 SYSTEM 缺货的 source 传递。
- **测试缺口**：`TestRefundReleaseSourceMapping` 只测 `refundReleaseSource` 纯函数；没有 system shortage 端到端断言 event.source/reason；没有未知 source 在 head 上的 admission 回滚测试。当前工作区新增的 `TestAdmitRefundUnsupportedReleaseSourceRejects` 不属于 `3597989`。
- **判定**：source 是 T1.4b2a event 的闭合枚举/owner 字段；已知 SYSTEM 缺货路径长期发错 source，且未知 source 不 fail-closed，属于契约/owner 阻断。需要 Dev 在 after-sales→Order 传递 `system_shortage`（或由 Architect 明确 source 归属），并补端到端测试；不得只改 registry 测试。

### B3 [BLOCKER] — 已存在的 PRE admission/S19 没有 release outbox，且无 backfill/repair

- **位置**：`internal/services/order/refund_admission.go:179-185`（已存在非 PENDING admission 直接 `admissionReply` 返回，不再 append release）；`:203-210` 锁内同样；`writeZeroRefundTx` 已有 application 时走 `zeroRefundReply`，也不补 release。
- **事实**：T1.4a 已写入的 PRE admission（`result=PRE_SHIP_REVOKED`）在 T1.4b2a 升级后重试 `AdmitRefund` 只会幂等返回，**不会补写 `inventory.refund_release_requested`**；零元已 DONE 的 S19 同理。T1.4b2a 无 backfill worker/migration（design C-05 “无迁移”），也没有扫 `order_refund_admissions` 缺失 release 的修复任务。对升级前/升级中已 PRE 的组，INV-44 物理释放永远不会发生。
- **测试缺口**：`TestAdmitRefundPreWritesReleaseOutbox` 只覆盖“新 admission 首次写入”；没有“先插入既有 PRE admission（无 release），再重试”的用例；没有 zero S19 旧记录的用例。
- **判定**：这是升级/数据完整性阻断；需要 Architect 定 backfill 方案（启动/worker 扫 PRE 且无 release 的 admission，幂等补 outbox；或 migration/repair）并补测试。至少在 T1.4b2a 的 rollout 前不能声称 INV-44 完整。

---

## 2. 已通过的部分

### T1.4b2a（除 B1/B2/B3）

- Event registry：`inventory.refund_release_requested` 字段齐全、`schema_version=1`、`release_id=rr:<refund_id>`、`source/reason` 闭合对、`items` 唯一 `(sku,warehouse)`、`warehouse_id>0`、`requested_at` RFC3339；事件测试 13 个子用例 PASS。
- Order：`AdmitRefund` **仅 PRE** 在 admission 事务内 `outbox.Append`；POST 不写；release outbox 失败回滚 admission；并发只写一条；zero PRE 写、zero POST 不写；缺 warehouse/无 item 失败回滚。`TestAdmitRefundPreWritesReleaseOutbox`、`PostDoesNotWrite`、`ReleaseOutboxFailureRollsBack`、`SystemZero*`、`ConcurrentWritesOne`、`MissingWarehouse` 等 PASS。
- Inventory：consumer 注册于 `cmd/inventory/main.go:80`；HELD→`Release`、CONFIRMED→`TccCancelConfirm`、RELEASED/缺失幂等跳过、未知状态 Permanent、部分失败不 ACK、同 event 重放/多 worker/多仓/canceled context/DLQ 路径 PASS（`TestRefundRelease...` 全绿）。
- PRE-only：POST 不写 release；符合 design。**但 B1 的 group/reservation 语义未解决。**

### T1.4b2b / b2（全部通过）

- Zero task **先持久**：`Refund()` 在 `ensureRefundTaskTx` + commit 后才调 `processRefundTask`；system `createSystemRefund` 创建 after-sale 时同事务建 task；`TestPositiveRefundAdmitsAfterTaskPersisted` PASS。
- `processRefundTask` 在 `s.payments` 校验前用 `isZeroSettlementOrder(order)` 短路到 `processZeroRefundTask`；`AdmitRefund(source=system_zero, amount=0)` 在 `admitZeroRefund` 校验 `pay_cent=0` + `zero:<order_no>` 身份后走 permit/S19，不查 Payment quota/commitment。`TestZeroAfterSaleRefundTaskCompletesWithoutPayment`、`TestZeroRefundDoesNotRequirePaymentClient` PASS。
- 失败无 S19：`AdmitRefund` 失败/非 DONE 时 release task，不写本地 REFUNDED；`TestZeroAfterSaleCancelFailureRetriesWithoutS19`、`S19WrittenThenCrashIsIdempotent`、`TaskExhaustionFailsPermanent` PASS。
- `REFUNDED/refunded_cent=0/amount_state=CONSUMED/refund_no=''` + task DONE 同事务；`aftersale.completed` 发出。`TestZeroAfterSaleRefundTaskCompletesWithoutPayment` PASS。
- 创建/审批/system 入口：`CreateAfterSale`/`Approve`/`createSystemRefund` 对 zero-settled order 可达；`capturedForOrder` 对 zero identity 直接返回 0/zero，不调 Payment；`Approve` 拒绝正金额；`TestZeroAfterSaleEndToEndCreateApproveRefundWithoutPayment`、`ApprovalRejectsPositiveAmount`、`ZeroSystemShortageRefundUsesZeroTask` PASS。
- 非 zero 身份 fail-closed：`pay_cent=0` 但 settlement 非 `zero:<order_no>`、captured=0、非 zero 订单 amount0/free line 均不被标成 zero；`TestNonZeroOrderZeroAmountRequestIsNotZeroAfterSale`、`NonZeroOrderFreeLineZeroAmountFailsClosed`、`CapturedZeroNonZeroIdentityFailsClosed`、`AdmitRefundSystemZeroRejectsMismatchedFacts` PASS。
- return 先 restock：`processRefundTask` 在 zero 分支前检查 `typeReturnRefund && restockState != DONE`；`TestZeroReturnRefundRequiresRestockBeforeAdmission` PASS。

---

## 3. 测试证据（Reviewer 独立复跑）

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0**，60.153s |
| `go test ./internal/platform/events -run 'TestRefundRelease' -count=1 -v` | **PASS**，0.166s |
| `go test ./internal/services/order -run 'TestAdmitRefund|TestRefundRelease' -count=1 -v` | **PASS**，12.263s（含工作区未提交 source/no-item 用例） |
| `go test ./internal/services/inventory -run 'TestRefundRelease' -count=1 -v` | **PASS**，1.993s |
| `go test ./internal/services/aftersales -run 'TestZero|TestPositiveRefund|TestNonZero|TestCapturedZero' -count=1 -v` | **PASS**，7.207s |
| `go test -race ./internal/services/inventory -run 'TestRefundReleaseConsumerConcurrentWorkers|TestRefundReleasePartialFailureDoesNotAck' -count=1` | **ok**，1.704s |
| `go test -race ./internal/services/order -run 'TestAdmitRefundConcurrentWritesOneReleaseOutbox|TestAdmitRefundPreWritesReleaseOutbox' -count=1` | **ok**，2.321s |
| Reviewer overlay：reservation qty=2 + group item qty=1 | **handler 返回错误，reservation HELD/reserved=2**；证明 B1 子集 group 无法释放（不是通过） |
| `go vet` / `gofmt -l` | exit 0 / 无输出 |

- 未发现 SKIP 被当 PASS；AC-50/AC-67/AC-68 未标 AC-VERIFIED。
- 本轮测试使用当前工作区，包含不在 `3597989` 的未提交 source/no-item 改动；B1 的 overlay 与 Inventory 测试不受该改动影响。

---

## 4. 边界 / 范围

- **未越界**：无 T1.5 `order.refund_requested`、T1.6 PENDING_REMOTE、T1.4b2c/INV-44 物理回库扩展、T1.7/T5；无 Inventory restock 新路径、无 Payment/Order 状态机扩展；无 T1.4b2 之外的迁移/proto（C-05 无迁移）。
- **Owner**：Order 生产 release outbox、Inventory 执行、AfterSales 负责 zero task/restock；B2/B3 的 source/backfill owner 仍缺失。
- **C-04/C-05**：event registry 增量、无 proto；design DRAFT；QC-27 `READY` 不等于实现 PASS。
- **测试真实性**：真实 MySQL、并发/回滚/DLQ/zero 等；但 **B1/B2/B3 的关键组合用例缺失**（见上）。

---

## 5. 声明

- 本报告未修改代码/契约/迁移；只新增报告；不批准合并。
- **verdict FAIL（有范围）**：T1.4b2b/b2 zero 路径通过；T1.4b2a release 的 B1（架构/物理释放）、B2（source/owner）、B3（既有 PRE backfill）仍阻断 T1.4b2 关闭。**不是 Release PASS，不替代人类 merge 决策。**
- B1 为 `[ARCHITECTURAL_FLAW]`，应打回 Architect 决定 event/reservation 身份与 partial/full release 语义，Dev 不得在本分支硬改；B2/B3 需在 Architect 方案后补实现与测试，重新按精确 head 复审。
