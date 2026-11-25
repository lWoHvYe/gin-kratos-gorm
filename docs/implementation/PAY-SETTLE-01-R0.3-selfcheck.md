# PAY-SETTLE-01 R0.3-R1/R2 Developer 自检清单

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Role / Actor | Developer / `dev-r03` |
| Tier | **L2**（资金/退款投影 + 既有事件消费语义；不改生产代码） |
| Spec-Ref | SPEC-0001 **v1.4** AC-27；SPEC-0002 **v1.10** INV-45/INV-48/INV-50；SPEC-0001 §6.6/§6.6.1/§7.4/§7.5 |
| Base | `0872983`（Lead Phase 0-R closeout plan；干净工作树中的他人文档改动未纳入本提交） |
| Code head（本次测试证据绑定） | **`fe1600bf41b8ab43ab146f08269bfbf3dc3437d0`** |
| Selfcheck commit | 本文件为 `fe1600b` 之后的 docs-only 提交；`git diff fe1600b..HEAD --stat` 仅本文件 |
| allowed_paths（实际改动） | `internal/services/order/warehouse_quote_test.go`（改）、`internal/services/order/ac27_projection_negative_test.go`、`ac27_projection_cancelled_test.go`、`ac27_projection_integration_test.go`（新增）、本文档 |
| forbidden_paths（未触碰） | 生产代码、契约、migrations、deploy、scripts、台账（`ac-coverage.tsv`/evidence-matrix/tasks/review-tracking 均未改） |
| Status | **WIP**；不声称实现 PASS / Release PASS；**不标 AC-VERIFIED**（归 Reviewer） |

范围：任务 A（AC-27 分支断言）、任务 B（AC-14/21/25/27/31/32 真实 MySQL 回归）、任务 C（INV-48/INV-50 evidence 路径核对）。完整命令与日志见各节；日志原始文件在本地 `.tmp/evidence-r03/final-*.log`（未入库）。

---

## 1. Diff 摘要（`fe1600b`）

```
internal/services/order/ac27_projection_cancelled_test.go    |  62 +++++++++++
internal/services/order/ac27_projection_integration_test.go  | 122 +++++++++++++++++++++
internal/services/order/ac27_projection_negative_test.go     |  68 ++++++++++++
internal/services/order/warehouse_quote_test.go              |  45 +++++++-
4 files changed, 294 insertions(+), 3 deletions(-)
```

- `warehouse_quote_test.go`：**加强**既有 `TestAC27RevokedGroupIsClosedInProjection`（Reviewer 的 evidence 指针 `warehouse_quote_test.go:123` 仍然有效，行号漂移到 `:125`）——fixture 显式使用 `refund_state=REFUNDED` + permit `REVOKED` + `reservation_state=RELEASED`，并通过 `orderRecord.toProto`（order RPC 实际读模型，`repo.go:113-125`）断言投影，而不是只测纯函数。
- `ac27_projection_negative_test.go`：反向用例（已发货组在 `REFUNDING`/`REFUNDED` 下不得变成 `CANCELLED`、不得被当未发组）。
- `ac27_projection_cancelled_test.go`：已持久化 `CANCELLED` 组分支、全部退款未发组（§7.5 行）投影。
- `ac27_projection_integration_test.go`：真实 MySQL 行 + 生产写入函数（`BlockShipmentForRefund`→`REFUNDING`/`REVOKED`，`applyAfterSaleCompleted`→`REFUNDED`）→ `loadFulfillmentGroups` + `toProto` 回读断言。
- 无生产代码、无 schema、无契约改动；新增断言不引入新状态字段、不给读模型加 `if-else`。

---

## 2. 任务 A：AC-27 分支断言

### 2.1 已补齐且真实断言的分支

| EARS / 分支 | 测试（文件:行号，行号指 `fe1600b`） | 真实断言 | 结果 |
|---|---|---|---|
| EARS-2：G2 `refund_state=REFUNDED` 且 permit `REVOKED` → 投影 `CANCELLED`；G1 `SHIPPED` 不被阻塞 | `warehouse_quote_test.go:125` `TestAC27RevokedGroupIsClosedInProjection` | `effectiveGroupStatus(G2)=CANCELLED`；`shipmentStatusForGroups=SHIPPED`；`aggregateOrderStatus=SHIPPED`；`toProto` 中 order `status` 仍 `ALLOCATED`、`shipment_status=SHIPPED`、`G2.status=CANCELLED`、`G2.refund_state=REFUNDED`、G2 无任何 group action | **PASS** |
| 同上（真实行/真实写入函数） | `ac27_projection_integration_test.go:16` `TestAC27RefundedGroupProjectionFromRows` | `BlockShipmentForRefund` 后 DB 行 `G2.refund_state=REFUNDING`、permit=`REVOKED`；`applyAfterSaleCompleted` 后 `G2.refund_state=REFUNDED`、持久化 `group.status` 仍 `ALLOCATED`；`orders.status` 仍 `ALLOCATED`；`toProto`：order `ALLOCATED`、`shipment_status=SHIPPED`、G1 `SHIPPED`、G2 `CANCELLED`+`REFUNDED` | **PASS**（`-race`） |
| 运单/组已 `CANCELLED`（按现有读模型字段：持久化 `group.status`） | `ac27_projection_cancelled_test.go:10` `TestAC27CancelledGroupIsClosedInProjection` | `groupIsClosed(CANCELLED)=true`；`effectiveGroupStatus=CANCELLED`；全 CANCELLED 时 `shipmentStatusForGroups=UNSHIPPED`、`aggregateOrderStatus=""`（不推进） | **PASS** |
| reservation released（§6.6.1 组合合法性） | `warehouse_quote_test.go:125` fixture `ReservationState="RELEASED"` + `ac27_projection_cancelled_test.go:10` | **仅证明该组合未被读模型拒绝**，不构成"释放驱动关闭"的断言——`groupIsClosed` 不读 `ReservationState`，见 §2.2 BLOCKED-2 | 组合 PASS / 分支 **BLOCKED** |
| §7.5"全部退款（未发）"：2 组均 REFUNDED+REVOKED → 组 `CANCELLED`、订单 `ALLOCATED`、履约投影 `UNSHIPPED` | `ac27_projection_cancelled_test.go:36` `TestAC27AllRefundedUnshippedGroupsStayAllocated` | `shipmentStatusForGroups=UNSHIPPED`；`aggregateOrderStatus=""`；`toProto` order `ALLOCATED`，两组 `CANCELLED`+`REFUNDED` | **PASS** |
| 反向：已 `SHIPPED` 组 `REFUNDING` 不得投影 `CANCELLED`、不得当未发组 | `ac27_projection_negative_test.go:9` `TestAC27PostShipRefundingGroupStaysShipped` | `effectiveGroupStatus=SHIPPED`；`shipmentStatusForGroups=SHIPPED`；order lifecycle 仍 `SHIPPED`；G2 仍暴露 `receive` | **PASS** |
| 反向：已 `SHIPPED/RECEIVED` 组 `REFUNDED` 不得回退为未发 | `ac27_projection_negative_test.go:42` `TestAC27PostShipRefundedGroupStillCountsAsShipped` | `SHIPPED+REFUNDED` → `SHIPPED`、投影 `PARTIALLY_RECEIVED`、未提前 `COMPLETED`；`RECEIVED+REFUNDED` → `RECEIVED`、订单可 `COMPLETED` | **PASS** |

`REFUNDING + SHIPPED` fixture 的生产可达性：TCC 补偿在已离仓组上写 `order_fulfillment_groups.refund_state=REFUNDING`（`tcc_settlement.go:699-704`），permit 保持 `CONSUMED`。

### 2.2 BLOCKED：读模型无法表达，未加 `if-else`、未改生产语义

按 Brief 边界与 `prompt/Developer.md` 反证义务，以下三条**停手**，交 Architect/PO 裁决：

#### BLOCKED-1 运单 `CANCELLED` 分支（AC-27 EARS-2/EARS-3）

事实：
- `fulfillmentGroup` 读模型只有 `Status/RefundState/ReservationState/PermitState/Freight*`，**没有运单/发货字段**（`fulfillment_groups.go:16-27`）。
- Order **没有运单状态存储**：`ports.go:49-54` 明确 "Order has no shipment/shipped_at storage"，运单事实只能实时调 `Fulfillment.ListShipments`。
- 没有 `shop.shipment.cancelled` 消费：`consumer.go:58-62` 只绑定 created/shipped/received。
- 关闭判定只读 `Status` + `PermitState`（`settlement.go:67-78`），不读 `RefundState/ReservationState`。

具体时序反证（可达）：
1. G2 先 `AuthorizeShipment` → permit `CONSUMED`，`shop.shipment.created.v1` 把组置 `ALLOCATED`（`consumer.go:58`）。
2. 售后 `AdmitRefund`（全额组退款）：permit=`CONSUMED` → `refund_admission.go:256-257` 记 `POST/PENDING_REMOTE`；`finishRemoteAdmission`（`:566-599`）调 `CancelIfCreated`，运单未发 → 返回 `CANCELLED` → 改记 `PRE` 并追加预留释放；**permit 保持 `CONSUMED`**（只有 `permitNone` 才 `REVOKE`，`:273-277`）。
3. 运单进入 `CANCELLED`（§7.4），库存预留释放（§6.6.1）。
4. `shop.aftersale.completed.v1` → 组 `refund_state=REFUNDED`（`aftersale_consumer.go:43-46`），**组 `status` 仍 `ALLOCATED`、permit 仍 `CONSUMED`**。
5. 读模型：`groupIsClosed=false` → G2 投影 `ALLOCATED`（不是 `CANCELLED`）；`shipmentStatusForGroups` 把 G2 计入 `remaining` → `PARTIALLY_SHIPPED`；`aggregateOrderStatus` 无法推进到 `SHIPPED`。

影响：AC-27 EARS-2 的"运单 `CANCELLED`"关闭条件未生效；AC-27 EARS-1"G2 不得使订单停留在 `ALLOCATED`"在该分支失败；SPEC-0001 §7.5:405（G2 退款未发 → `G2 CANCELLED`）不一致。AC-25/INV-01（不把退款写进 `orders.status`）不受影响。

为什么不能在 `_test.go` 里解决：任何断言"该组已关闭"都要求 `groupIsClosed` 读 `RefundState`/运单事实；写一个 `RefundState=REFUNDED` + permit `CONSUMED` 的 fixture 会**失败**，补 `if` 就是改生产读模型语义。修法只能是（a）`groupIsClosed` 增加 `refund_state=REFUNDED` 判定，或（b）Order 侧新增运单状态投影/`shipment.cancelled` 消费——均属状态所有权/消费协议变更，超出本次 `_test.go` 权限。

#### BLOCKED-2 reservation released（AC-27 EARS-3）

事实：
- `reservation_state` 的写点只有两处：插入 `HELD`（`fulfillment_groups.go:58-62`）、订单取消时 `status=CANCELLED` 同步写 `RELEASED`（`:71-80`）。
- 退款准入的库存释放走 outbox release（`refund_admission.go:285`、`:599`），**不回写** `order_fulfillment_groups.reservation_state`；`groupIsClosed` 不读该字段（`settlement.go:67-78`）。
- 因此"退款已释放预留"在 Order 读模型中不可观察：退款成功后 order 行仍是 `reservation_state='HELD'`。

为什么不在测试里硬断言：fixture 塞 `RELEASED` 对投影结果无影响，断言它等于没证明分支（写伪断言违反反妥协红线）。AC-27 EARS-3 的"预留已释放→组合合法"目前只能由 Inventory 侧（`CancelIfCreated`/release 事实）证据覆盖，而本 change 的 allowed_paths 不含 `internal/services/inventory/`。

裁决点：是否要求 Order 投影库存释放（跨服务读模型/事件）？若否，AC-27 EARS-3 应在 evidence-matrix 明确由 Inventory 侧证据承接。

#### BLOCKED-3 `REFUNDING` 反向（"退款未完成不得投影 CANCELLED"）

生产事实：pre-ship 退款准入会撤销 permit（`refund_admission.go:273-277`；TCC 同事务写组 `REFUNDING`，`tcc_settlement.go:699`），`groupIsClosed` 对 `ALLOCATED+REVOKED` 返回 true（`settlement.go:77`）——因此 `REFUNDING` 组当前被投影为 `CANCELLED`。要满足 Brief 的反向要求，必须让关闭只在 `REFUNDED` 时成立（新增 `refund_state` 负向判断），即改生产语义。

具体时序反证（说明该反向要求本身有害）：TCC/售后准入先撤 permit 再打款；若退款最终失败停在 `REFUNDING`（INV-45：permit 终态不可恢复；`settlement.go:347-352` 对 `CONSUMED`/`REVOKED` 均拒绝再次发货），按"`REFUNDING` 不关闭"则该组永远计入 `remaining` → G1 已发货但订单永远 `PARTIALLY_SHIPPED/ALLOCATED`，正是 AC-27 EARS-1 禁止的"G2 使订单停留在 `ALLOCATED`"；UI 同时出现 `ALLOCATED` 但无 `ship` action 的死组。故该反向要求需 PO/Architect 明确：`REFUNDING`（pre-ship，permit 已撤）是否算"已关闭"。

已诚实覆盖的部分：post-ship `REFUNDING`/`REFUNDED` 已发货组保持 `SHIPPED`（§2.1 最后两行），满足"不得把已发货组当未发组处理"。

### 2.3 契约观察（不阻塞本次，需 PO/Architect 对齐）

SPEC-0001 内部不一致：AC-27 EARS-1（`:571-573`）要求"G2 不得使订单停留在 `ALLOCATED`"，而 §7.5:405 示例行把"G2 退款（未发）"的生命周期写成 `ALLOCATED`、履约投影写成 `PARTIALLY_SHIPPED`；§7.5 注（`:409`）又写"订单生命周期仍为 ALLOCATED/PAID"。当前实现（`aggregateOrderStatus`/`shipmentStatusForGroups`）与既有单测按 AC-27 正文推进到 `SHIPPED`（G1 已发、G2 关闭 → `remaining=1`）。本次测试断言跟随 AC-27 正文；§7.5 示例行是描述性残留，建议 PO 在对齐 v1.10 时修正（不由 Developer 改契约）。

---

## 3. 任务 B：真实 MySQL 回归

环境：`go1.27.1 darwin/amd64`；仓库本地 Go 环境（`docs/development.md` §1：`PATH/.tmp/bin`、`GOCACHE`、`GOTMPDIR`、`GOMODCACHE`、`GOPATH`、`GOSUMDB=off`、`GOFLAGS=-mod=readonly`、`GOPROXY=off`）；MySQL `mysql-unicorn`（`mysql:latest`，26.10.0）`127.0.0.1:3306`；每包用 `scripts/test-dsn.sh "$BASE" <pkg>` 派生独立 schema（`mall_integration_order` / `mall_integration_aftersales` / `mall_integration_fulfillment`），未共用 schema。

| 包 | 命令（DSN 已按包派生） | 测试 | 耗时 | 结果 |
|---|---|---|---|---|
| aftersales | `TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run '^(TestAC14FreightRefundSixScenarios\|TestAC31FreightHoldTimeout\|TestAC32FreightHoldTimeoutDefaultsToMerchant)$' -count=1 -v` | `settlement_integration_test.go:875` `TestAC14FreightRefundSixScenarios`（6 子场景全 PASS：unshipped_cancel/merchant_full_order/merchant_partial_group/customer_no_reason/partial_quantity_customer/platform_partial_group）、`:1101` `TestAC31`、`:1102` `TestAC32` | ok **1.589s**（wall 4.42s） | **PASS，exit 0** |
| aftersales | `… go test -race … -run '^(TestAC14…\|TestAC31…\|TestAC32…\|TestZero\|TestNonZero\|TestCapturedZero)' -count=1` | 同上 + 12 个零元用例 | ok **8.669s**（wall 13.04s） | **PASS，exit 0** |
| fulfillment | `TEST_MYSQL_DSN=$FF_DSN go test ./internal/services/fulfillment -run '^TestAC21AutoConfirmT7$' -count=1 -v` | `delivery_integration_test.go:397` `TestAC21AutoConfirmT7` | ok **0.459s**（wall 3.24s） | **PASS，exit 0** |
| order | `TEST_MYSQL_DSN=$ORDER_DSN go test -race ./internal/services/order -run '^(TestAC25RefundProjectionKeepsLifecycle|TestAC27(RevokedGroupIsClosedInProjection|RefundedGroupProjectionFromRows|PostShipRefundingGroupStaysShipped|PostShipRefundedGroupStillCountsAsShipped|CancelledGroupIsClosedInProjection|AllRefundedUnshippedGroupsStayAllocated))$' -count=1 -v` | `settlement_integration_test.go:355` `TestAC25RefundProjectionKeepsLifecycle` + `TestAC27RevokedGroupIsClosedInProjection`（`:125`）/`TestAC27RefundedGroupProjectionFromRows`/`TestAC27PostShipRefundingGroupStaysShipped`/`TestAC27PostShipRefundedGroupStillCountsAsShipped`/`TestAC27CancelledGroupIsClosedInProjection`/`TestAC27AllRefundedUnshippedGroupsStayAllocated` | ok **2.226s**（wall 11.40s） | **PASS，exit 0** |
| order | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -count=1`（全包非回归） | 全包 | ok **89.268s**（wall 92.59s） | **PASS，exit 0** |
| build | `go build ./...` | — | wall 32.64s | **PASS，exit 0** |

真实日志摘录（`fe1600b`，节选）：

```
HEAD=fe1600bf41b8ab43ab146f08269bfbf3dc3437d0
=== aftersales AC-14/31/32 ===
--- PASS: TestAC14FreightRefundSixScenarios (0.44s)
--- PASS: TestAC31FreightHoldTimeout (0.59s)
--- PASS: TestAC32FreightHoldTimeoutDefaultsToMerchant (0.47s)
ok  gin-kratos-gorm/internal/services/aftersales  1.589s

=== fulfillment AC-21 ===
--- PASS: TestAC21AutoConfirmT7 (0.36s)
ok  gin-kratos-gorm/internal/services/fulfillment  0.459s

=== order -race AC-25/AC-27 ===
--- PASS: TestAC27RefundedGroupProjectionFromRows (0.53s)
--- PASS: TestAC25RefundProjectionKeepsLifecycle (0.56s)
--- PASS: TestAC27RevokedGroupIsClosedInProjection (0.00s)
ok  gin-kratos-gorm/internal/services/order  2.226s

=== full order package ===
ok  gin-kratos-gorm/internal/services/order  89.268s
```

注：order 包 stderr 的 `invalid log level: , switching to default: INFO` 为既有噪声（非测试失败；两个 order 运行 exit 0）。未使用 SKIP 作为 PASS：所有目标测试均真实执行；无 DSN 时 `orderTestDB` 会 SKIP，本次均带 DSN。

---

## 4. 任务 C：INV-48 / INV-50 evidence 路径（供 evidence-matrix 使用，状态归 Reviewer）

### 4.1 INV-48：T1.7 六场景（`internal/services/order/t17_evidence_integration_test.go`，HEAD `fe1600b`）

命令：`TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run '^TestT17' -count=1 -v` → ok **5.721s**（wall 9.03s），exit 0。

| 文件:行号 | 测试 | 结果 |
|---|---|---|
| `t17_evidence_integration_test.go:26` | `TestT17SigmaMismatchWritesAuditAndNeverCallsPayment` | PASS 0.61s |
| `:83` | `TestT17RefundingRejectsConcurrentAuthorizeShipment` | PASS 1.06s |
| `:204` | `TestT17CrashAfterTx1ResumesOnlyUnfinishedGroups` | PASS 1.01s |
| `:306` | `TestT17PartialGroupFailureBlocksAllPaymentsAndStaysRecoverable` | PASS 0.88s |
| `:371` | `TestT17LegacyNoGroupUsesG1OnlyWithEvidence`（2 子用例） | PASS 1.33s |
| `:442` | `TestT17ShippedGroupPostWritesNoReleaseAndNoRestock` | PASS 0.73s |

建议 evidence path：`internal/services/order/t17_evidence_integration_test.go:26`（六测试），`test_type=integration`。**状态仍为 AC-REF，不是 AC-VERIFIED**；本次未跑 T5 `-race`/fault 全矩阵。

### 4.2 INV-50：零元售后（`internal/services/aftersales/zero_refund_integration_test.go`，HEAD `fe1600b`）

命令：`TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run '^(TestZero|TestNonZero|TestCapturedZero)' -count=1 -v` → ok **5.133s**（wall 7.96s）；`-race` 合并运行 ok **8.669s**；均 exit 0。

| 文件:行号 | 测试 | 结果 |
|---|---|---|
| `:182` | `TestZeroAfterSaleRefundTaskCompletesWithoutPayment` | PASS 0.43s |
| `:221` | `TestZeroAfterSaleCancelFailureRetriesWithoutS19` | PASS 0.40s |
| `:274` | `TestZeroAfterSaleS19WrittenThenCrashIsIdempotent` | PASS 0.43s |
| `:327` | `TestZeroReturnRefundRequiresRestockBeforeAdmission` | PASS 0.43s |
| `:366` | `TestZeroRefundTaskExhaustionFailsPermanent` | PASS 0.60s |
| `:413` | `TestZeroRefundDoesNotRequirePaymentClient` | PASS 0.35s |
| `:497` | `TestZeroAfterSaleEndToEndCreateApproveRefundWithoutPayment` | PASS 0.53s |
| `:562` | `TestZeroAfterSaleApprovalRejectsPositiveAmount` | PASS 0.36s |
| `:594` | `TestZeroSystemShortageRefundUsesZeroTask` | PASS 0.42s |
| `:652` | `TestNonZeroOrderZeroAmountRequestIsNotZeroAfterSale` | PASS 0.47s |
| `:686` | `TestNonZeroOrderFreeLineZeroAmountFailsClosed` | PASS 0.35s |
| `:711` | `TestCapturedZeroNonZeroIdentityFailsClosed` | PASS 0.27s |

建议 INV-50 evidence path：`internal/services/aftersales/zero_refund_integration_test.go:497`（端到端）+ `:182`（任务无 Payment 完成）；保留 `:562`/`:711` 作为"不得正金额/不得把零捕获当零元"的反向证据。**不是 AC-VERIFIED**（AC-67/68 状态与台账归 Reviewer）。

---

## 5. 【测试范围】/【未覆盖范围】

**【测试范围】**

- AC-27：EARS-2（`REFUNDED`+`REVOKED` → 组 `CANCELLED`、G1 不被阻塞）、EARS-3 的组合合法性（`RELEASED` 不导致拒绝）、`CANCELLED` 组分支、§7.5 全部退款未发组投影、post-ship `REFUNDING`/`REFUNDED` 反向；unit + 真实 MySQL 行回读（`-race`）。
- AC-25：退款投影不写 `orders.status`（真实 MySQL，`-race`）。
- AC-14/31/32：真实 MySQL，6 场景 + 两个挂起用例，plain + `-race`。
- AC-21：真实 MySQL。
- INV-48：T1.7 六场景真实 MySQL（plain）。
- INV-50：aftersales 12 个零元/边界用例（plain + `-race`）。
- 非回归：`internal/services/order` 全包真实 MySQL 通过（89.268s）；`go build ./...` 通过。

**【未覆盖范围】**

- **BLOCKED-1/2/3**（§2.2）：运单 `CANCELLED` 关闭、Order 侧"预留已释放"可观察性、pre-ship `REFUNDING` 负向语义——未写断言、未改生产代码，等 Architect/PO 裁决。
- 未跑 `make verify`、`make test-integration` 全量、T5.2/T5.3 的 `-race`/故障注入全矩阵、迁移/Helm/deploy 验证。
- 未验证 BFF/UI 对 `refund_state`/组 `CANCELLED` 的渲染；未验证 Inventory 侧预留释放事实（allowed_paths 不含 inventory）。
- 12 个 orphan AC 的去留、evidence-matrix 的 INV-45..50 描述补全（R0.3-R3/R4，Lead/Reviewer）。
- aftersales/fulfillment 的真实 MySQL 测试使用测试内联 DDL（既有包 harness），不是 migration 应用证据。
- 24h/168h 定时行为未在真实时间推进下验证（测试用时钟注入/到期字段断言，沿用既有实现）。

---

## 6. 反妥协自检清单

### 6.1 并发冲突 / 网络超时 / 部分成功时会发生什么？处理了吗？

- 本次只改 `_test.go`：新增 3 个测试文件/加强 1 个，无 goroutine、无新并发路径，不改变运行时行为。
- 并发/部分成功证据来自既有测试并在本 HEAD 重跑：`TestT17RefundingRejectsConcurrentAuthorizeShipment`（退款准入 vs 授权发货竞态）、`TestT17CrashAfterTx1ResumesOnlyUnfinishedGroups`（崩溃续跑只补未完成组）、`TestT17PartialGroupFailureBlocksAllPaymentsAndStaysRecoverable`（部分失败不部分打款）——均在 `fe1600b` PASS；order AC-25/AC-27 以 `-race` 执行（2.226s，无 data race 报告）。
- 结论：本次交付未引入新的并发/超时面；受影响的既有并发协议由上述 `-race`/故障场景覆盖。

### 6.2 有没有绕过架构加 `if-else`？

- **没有**。生产代码 0 改动；测试没有为通过而构造"生产不可达"的状态，也没有对不可表达的分支写伪断言。
- 未新增状态字段、未把退款进度写入 `orders.status`、未改 permit 语义；`ReservationState=RELEASED` 只作为 fixture 文档化，未断言其对关闭判定生效。
- 三个不可表达分支按 Brief 停手并给反证（§2.2），而不是在测试里加条件掩盖。

### 6.3 向现实做了哪些妥协（重大妥协高亮）

1. **【重大 / BLOCKED】** AC-27 的"运单 `CANCELLED`"与"预留已释放"分支、以及 Brief 要求的"`REFUNDING` 不得 `CANCELLED`"未交付断言：读模型不可表达 / 与现行 `groupIsClosed`+INV-45 语义冲突。已给具体时序反证，需 Architect/PO 裁决后才能改生产语义。**这是本次唯一未闭合项，不得当作已覆盖。**
2. **【已记录】** `REFUNDING` 组当前投影为 `CANCELLED`（依赖 permit `REVOKED`），本次未改变；post-ship 反向已覆盖。
3. **【已记录】** 新集成测试走测试内联 DDL（既有 harness），不是 runtime migration 证据；迁移证据由 R0.1/`verify-r01-migrations.sh` 负责。
4. **【已记录】** 本次不标 AC-VERIFIED，不更新台账/evidence-matrix（Reviewer/Lead Owner）。

### 6.4 Mock 数据真实性

- 集成测试用真实 MySQL 行，退款状态由生产写入函数产生（`BlockShipmentForRefund` → `REFUNDING`+`REVOKED`；`applyAfterSaleCompleted` → `REFUNDED`），不是手写终态。
- 单测 fixture 使用真实业务边界值（`paid=1000`、`refunded=400/1000`、组 `G1/G2`、`SHIPPED/ALLOCATED/RECEIVED/CANCELLED`），包含异常/反向分支（已退款已发组、全退款未发组、已取消组）。
- 零元用例沿用既有 `aftersales` 真实 MySQL 用例（含 `TestCapturedZeroNonZeroIdentityFailsClosed` 等异常分支），Mock/费用为 0 符合 `pay_cent=0` 合法路径。

---

## 7. 返工影响声明

- **[修改点]**：`internal/services/order/warehouse_quote_test.go`（加强 AC-27 投影测试）；新增 `ac27_projection_negative_test.go`、`ac27_projection_cancelled_test.go`、`ac27_projection_integration_test.go`；新增本 selfcheck。
- **[架构侵入性]**：无。未新增 `if-else`、未改状态所有权、未新增字段/事件/RPC；测试只调用既有读模型函数与生产写入函数。
- **[副作用评估]**：order 包测试数 +5（其中 1 个集成测试在无 `TEST_MYSQL_DSN` 时 SKIP）；既有 `TestAC27RevokedGroupIsClosedInProjection` 的 fixture 增加 `RefundState/ReservationState`，断言更严（仍然是同一读模型行为）；order 全包、aftersales、fulfillment 目标测试均通过，未见副作用。
- **[未处置风险]**：§2.2 三个 BLOCKED 分支 + §2.3 契约观察。

---

## 8. 结论

- 任务 B/C **完成**：目标测试在 `fe1600b` 真实 MySQL 全部 PASS（无 SKIP/FAIL），命令/耗时/行号见上；不标 AC-VERIFIED。
- 任务 A **部分完成**：EARS-2 正向、`CANCELLED` 组、§7.5 全退款未发、post-ship 反向已真实断言；**运单 `CANCELLED`、Order 侧 `reservation released`、pre-ship `REFUNDING` 负向三支 BLOCKED**，附时序反证与影响 AC/INV，交 Architect/PO 裁决。
- 本文件与代码提交均为 **WIP**，不构成 Reviewer PASS / merge / Release。
