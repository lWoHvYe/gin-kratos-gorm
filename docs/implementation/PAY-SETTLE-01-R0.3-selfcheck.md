# PAY-SETTLE-01 R0.3-R1/R2 Developer 自检清单

> **2026-11-27 追加 §9（R0.3-R1c，code head `9a57d01`）**：§2.2 的 BLOCKED-1（运单 `CANCELLED`）已由 AC-27A 第 5 条读模型关闭谓词收口；BLOCKED-2/3 按 SPEC-0001 v1.5 处理；§2.3 的 §7.5 漂移已由 PO v1.5 修正。§1–§8 是 `fe1600b`/`34d0466` 的历史证据，保持原样不改写。

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

---

## 9. 追加：R0.3-R1c — AC-27A 第 5/6 条读模型关闭谓词（2026-11-27）

| 项 | 值 |
|---|---|
| change_id / Tier / Actor | `PAY-SETTLE-01` / **L2** / `dev-r03` |
| Base | `ce0f6b4bb365791ef46098e943f1c25a69ff233d`（Lead resume R0.3-R1c） |
| Code head（本节证据绑定） | **`9a57d012254fcad8c34d9af7c48b2aa398c36ccb`** |
| Selfcheck commit | 本节为 `9a57d01` 之后的 docs-only 提交；`git diff 9a57d01..HEAD --stat` 应仅本文件 |
| Spec-Ref | SPEC-0001 **v1.5** AC-27（读模型口径）/AC-27A（第 5/6 条）；SPEC-0002 **v1.11** AC-50（只读消费，未改） |
| 契约前置 | SPEC-0001 v1.5 approved `4215b3c`；L2 `PAY-AC50-REVIEW-01` scoped PASS（`f0420fc`/`91b7310`，code `3ce0d7d`），`order_refund_admissions.result=PRE_SHIP_REVOKED/REVIEW_REQUIRED` 写路径就绪 |
| allowed_paths（本次改动） | `internal/services/order/fulfillment_groups.go`、`internal/services/order/settlement.go`、`internal/services/order/ac27a_closure_unit_test.go`、`internal/services/order/ac27a_closure_integration_test.go`、本文件 |
| forbidden（未触碰） | L2 代码（`refund_admission.go` 等）、migrations、deploy、specs、adr、台账、BFF/UI |
| Status | **WIP**；不声称实现 PASS / merge / Release / AC-VERIFIED |

### 9.1 Diff 摘要（`9a57d01`）

```
internal/services/order/ac27a_closure_integration_test.go | 261 +++++++++++++
internal/services/order/ac27a_closure_unit_test.go        | 198 +++++++++++
internal/services/order/fulfillment_groups.go             |  24 +++-
internal/services/order/settlement.go                     |  28 +++-
4 files changed, 503 insertions(+), 8 deletions(-)
```

- 无迁移/无 schema/无事件/RPC/worker：`git diff 9a57d01^..9a57d01 -- migrations` 输出为空。
- 无 `orders.status` 写入：生产 diff 只增加读模型派生字段与关闭谓词，没有 `UPDATE orders`。

### 9.2 实现（仅 Order 读模型）

| 文件 | 改动 |
|---|---|
| `fulfillment_groups.go:27-32` | `fulfillmentGroup.PreShipAdmission bool`：**非持久化派生字段**；注释明确来自 Order 自有 `order_refund_admissions.result=PRE_SHIP_REVOKED`，`refund_state` 不参与。 |
| `fulfillment_groups.go:92-126` | `loadFulfillmentGroups` 在同一 SELECT 里用 `EXISTS(SELECT 1 FROM order_refund_admissions a WHERE a.order_no=g.order_no AND a.group_no=g.group_no AND a.result=?)` 派生该 bool；`refundResultPre` 作为参数传入避免字面量漂移。用 EXISTS 而非 LEFT JOIN：同组可能存在多行准入（如保留 PRE 审计后再以新 `refund_id` 准入 POST），JOIN 会放大行数；`0016:18` 已有 `(order_no, group_no)` 索引，无 schema 变化。 |
| `settlement.go:64-96` | `groupIsClosed` 固定顺序：nil→open；`Status=CANCELLED`→closed；`Status∈{SHIPPED,RECEIVED}`→open（离仓守卫先于退款事实）；`PermitState=REVOKED`→closed（S5 终态/INV-45）；`PermitState=CONSUMED && PreShipAdmission`→closed；其他→open。`refund_state`/`reservation_state` **不参与**。它是唯一关闭入口；`effectiveGroupStatus`/`shipmentStatusForGroups`/`aggregateOrderStatus` 调用方式未改。 |

语义要点（供 Reviewer 核对）：
- 可达状态下 `result=PRE_SHIP_REVOKED` 意味着 `state=PRE_SHIP_REVOKED`：Tx1 的 `NONE→PRE` 与 Tx2 的 `CONSUMED+Cancelled` 都在同一条 SQL 同时写 `result/state`；`PENDING_REMOTE` 行只可能是 `result=POST_SHIP_ALLOWED`（Tx1 占位）或 `REVIEW_REQUIRED`，两者都不关闭（AC-27A 第 6 条/注）。
- 第 5 支按 AC-27A 第 5 条原文附加 `permit=CONSUMED`：可达 PRE 只有两种来源——`CONSUMED+CancelIfCreated=CANCELLED`（permit 保持 CONSUMED）与 `NONE→REVOKED`（permit 已被第 4 支短路为 closed）；因此与 Lead Brief 的 `PreShipAdmission→true` 一行写法在**可达状态**等价，而 `NONE/其他 permit + PRE` 按 AC-27A 第 6 条保持 open（fail-safe，不为数据异常新增关闭分支）。
- 迟到 `SHIPPED/RECEIVED`：按 AC-27A 第 3 条离仓守卫，S3 组投影收到真实 `shipment.shipped/received` 事件后读模型转为 open；PRE 准入行只作审计、读模型不写回。**若 S3 仍是 `ALLOCATED`（无事件证据/重复乱序无法判定），PRE 组保持 closed（不重新打开）**——两条都测了。
- AC-27A 第 6 条"告警"由 AC-50 写路径承担（`REVIEW_REQUIRED` Error log + AC-72 metric），本次**不在读路径新增日志/告警**（避免读模型副作用）；不声称新增告警能力。

### 9.3 与 R0.3-R1b 裁决 / SPEC v1.5 的关系

| 裁决 | 处置 | 证据 |
|---|---|---|
| **R-1** BLOCKED-1 运单 `CANCELLED` | **关闭**：以 Order 自有 `order_refund_admissions.result=PRE_SHIP_REVOKED` 作为"pre-ship 已准入、不会再产生包裹"的代理事实；不新增运单投影、不消费 `shipment.cancelled`、不在读路径远程调用 | `TestAC27AConsumedPreAdmissionClosesFromRealRows` |
| **R-2** 关闭后重聚合订单生命周期（ADR DRAFT 0.1 §3.2） | **未实现**：SPEC-0001 v1.5 AC-27 EARS-1 明确把"推进"限定为 S2 读模型 `shipment_status`，并规定**不因组关闭/退款完成写 `orders.status`**（`0001:599-603`）；Lead Brief 与 review-tracking scope 同口径。DRAFT ADR 的 R-2 与 v1.5 冲突，以 SPEC/PO 为准；未新增 aftersale consumer 状态迁移/outbox | 集成测试断言退款完成前后 `orders.status` 均 `ALLOCATED` |
| **R-3** reservation released | 维持不要求 Order 可观察、不写 `RELEASED`；测试只断言语义边界（`RELEASED` 单独不关闭，PRE 关闭不受 `HELD/RELEASED` 影响） | `TestAC27AClosureIgnoresRefundStateAndReservationState` |
| **R-4** pre-ship `REFUNDING` 投影 `CANCELLED` | 保持正确：关闭由 permit/admission 决定，`REFUNDING` 不阻止关闭；post-ship 仍不关闭 | 谓词表 + 既有 `TestAC27PostShip…` 回归 |
| **R-5** §7.5 漂移 | PO v1.5 已修正示例表（生命周期 `ALLOCATED`、履约投影 `SHIPPED`），本次测试按该口径；§2.3 的观察作废 | `0001:405`（v1.5） |
| AC-27A 第 6 条 REVIEW/PENDING | `REVIEW_REQUIRED`、`PENDING_REMOTE` 占位、`POST_SHIP_ALLOWED`、缺行均不关闭 | `TestAC27AAdmissionResultDerivation` 子用例 |

### 9.4 命令与真实日志（HEAD `9a57d01`，MySQL **8.4.13**）

环境：`docker mysql:8.4`（disposable，`127.0.0.1:13336`）；每包独立 schema，经 `scripts/test-dsn.sh` 派生：`mall_integration_order` / `mall_integration_aftersales` / `mall_integration_fulfillment`（未共用 schema）；`go1.27.1 darwin/amd64`；仓库本地 Go 环境（`GOFLAGS=-mod=readonly GOPROXY=off` 等）。日志原始文件：`.tmp/evidence-r1c/final-*.log`（未入库）。

| # | 命令 | 结果 |
|---|---|---|
| 1 | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -count=1` | **ok 116.705s**（wall 119.94s），exit 0 |
| 2 | `TEST_MYSQL_DSN=$ORDER_DSN go test -race ./internal/services/order -count=1` | **ok 121.013s**（wall 124.45s），exit 0，无 data race |
| 3 | `… go test -race ./internal/services/order -run '^(TestAC25RefundProjectionKeepsLifecycle\|TestAC27\|TestT17\|TestTCCReview\|TestAC50Review)' -count=1 -v` | **51 PASS / 0 FAIL / 0 SKIP**，ok 26.344s（wall 29.78s） |
| 4 | `… go test -race ./internal/services/order -run '^(TestAdmitRefund\|TestZeroApplicationCheckConstraint\|TestRefundRelease)' -count=1 -v` | **20 PASS / 0 FAIL**（INV-44/45/50 回归），ok 13.531s（wall 16.92s） |
| 5 | `TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run '^(TestAC14\|TestAC31\|TestAC32\|TestZero\|TestReview)' -count=1` | **ok 7.116s**，exit 0 |
| 6 | 同 5 + `-race` | **ok 8.277s**，exit 0 |
| 7 | `TEST_MYSQL_DSN=$FF_DSN go test ./internal/services/fulfillment -run '^(TestAC21\|TestCancelIfCreated)' -count=1` | **ok 1.331s**，exit 0 |
| 8 | `go build ./...` / `go vet ./internal/services/order/` / `gofmt -l`（4 个改动文件） | build exit 0（wall 30.30s）；vet OK；gofmt 无输出 |

关键日志摘录（`9a57d01`）：

```
HEAD=9a57d012254fcad8c34d9af7c48b2aa398c36ccb
server=VERSION: 8.4.13
=== full order package ===
ok  gin-kratos-gorm/internal/services/order  116.705s

=== full order package -race ===
ok  gin-kratos-gorm/internal/services/order  121.013s

=== AC-25/27/27A + T17 + TCCReview + AC50Review (-race) ===
--- PASS: TestAC27AConsumedPreAdmissionClosesFromRealRows (0.53s)
--- PASS: TestAC27AAdmissionResultDerivation (0.60s)
--- PASS: TestAC27AClosurePredicateOrder (0.00s)
ok  gin-kratos-gorm/internal/services/order  26.344s

=== INV-44/45/50 regression (-race) ===
ok  gin-kratos-gorm/internal/services/order  13.531s

=== aftersales non-regression ===
ok  gin-kratos-gorm/internal/services/aftersales  7.116s   (+ -race ok 8.277s)
```

（#1/#2 的原始日志同一文件 `.tmp/evidence-r1c/final-order-plain-race.log`（先 plain 后 `-race`）；#3/#4/#5/#6/#7 各自独立日志。order 包 stderr 的 `invalid log level` 为既有噪声，非失败。）

### 9.5 新增/变更测试

| 文件:行号（`9a57d01`） | 测试 | 断言要点 |
|---|---|---|
| `ac27a_closure_unit_test.go:9` | `TestAC27AClosurePredicateOrder` | 11 个顺序用例：nil/CANCELLED/离仓守卫/REVOKED/CONSUMED+PRE/NONE+PRE/NONE/GRANTED |
| `:76` | `TestAC27AClosureIgnoresRefundStateAndReservationState` | `refund_state∈{NONE,REFUNDING,REFUNDED}`×`reservation∈{HELD,RELEASED}` 均不改变 PRE 关闭/不关闭；`RELEASED`/`REFUNDED` 单独不关闭 |
| `:108` | `TestAC27ALateShippedStatusWinsOverPreAdmission` | S3 确认 `SHIPPED/RECEIVED` 后 PRE 组为 open、投影保持 `SHIPPED/RECEIVED`、仍计已发 |
| `:132` | `TestAC27AClosedGroupKeepsCancelledLifecycle` | `orders.status=CANCELLED` 与关闭组共存且保持 `CANCELLED`（AC-27 v1.5 EARS-3） |
| `:153` | `TestAC27AThreeGroupBoundary` | G3 未关闭 → `PARTIALLY_SHIPPED` 且不推进；G3 关闭 → 读模型 `SHIPPED`，`orders.status` 仍 `ALLOCATED` |
| `ac27a_closure_integration_test.go:20` | `TestAC27AConsumedPreAdmissionClosesFromRealRows` | 真实行 + 生产写入（`AdmitRefund`+`CancelIfCreated=CANCELLED`）→ admission `PRE/PRE`、permit 保持 `CONSUMED`、G2 closed/`CANCELLED`、G1 open；`orders.status` 前后均 `ALLOCATED`、`shipment_status=SHIPPED`；退款完成事件后 admission 未改写 |
| `:156` | `TestAC27AAdmissionResultDerivation` | 真实行派生：PRE（CONSUMED）关闭；PRE 但 permit `NONE`、REVIEW/PENDING 占位/POST/缺行均不关闭；迟到 `SHIPPED` 不因 PRE 重新关闭 |

既有 AC-27 测试（`warehouse_quote_test.go:125`、`ac27_projection_negative_test.go:9/42`、`ac27_projection_cancelled_test.go:10/36`、`ac27_projection_integration_test.go:16`）**未改期望**：它们本就断言 `orders.status` 保持 `ALLOCATED`、读模型 `shipment_status` 按 G1 推进（v1.5 口径），本次在 `-race`/真实 MySQL 下全部继续 PASS。

### 9.6 【测试范围】/【未覆盖范围】

**【测试范围】**
- AC-27A 第 5/6 条关闭谓词：nil/CANCELLED/离仓守卫/REVOKED/PRE/REVIEW/PENDING/POST/缺行；`refund_state`/`reservation_state` 不参与；3 组边界；CANCELLED 生命周期共存。
- AC-27 EARS-1/2 读模型口径：关闭组从剩余组排除、`shipment_status` 按 G1 推进、`orders.status` 不写；退款完成前后不变。
- AC-25 生命周期不含退款态；T17 六场景（Σ/竞态/崩溃/部分失败/legacy/shipped）`-race` 回归。
- AC-50/AC-69/70 相关 TCC/AC50 REVIEW 测试（12 个）`-race` 回归；INV-44/45/50 20 个 `-race` 回归。
- Order 全包 plain+`-race`；aftersales 目标集 plain+`-race`；fulfillment `AC-21`+`CancelIfCreated`；`go build ./...`。
- 真实 MySQL 8.4.13、独立 schema；无 SKIP/FAIL（SKIP 未计入 PASS）。

**【未覆盖范围】**
- `AC-27-NOWB-01`（存量 PRE 一次性审计/`CONSUMED+无运单` 写路径前置缺陷）与 `AC-27-LIVE-01`（`PENDING_REMOTE` 永久悬挂/w8）仍按 SPEC-0001 v1.5 §16.2 打开 → **AC-27 仍是 AC-REF，不得 AC-VERIFIED**。
- `AC-50-FIX-01` 的 L2 写路径不在本次范围（只读消费其 `result`）。
- 多组订单的 INV-44 释放请求缺口（P1/manual）未修；`reservation_state` 不可观察不变。
- 读路径"告警"未新增（由 AC-50 写路径 log/metric 承担）；BFF/UI 未验证；UAT/prod、迁移 apply、Helm/deploy 未涉及（无变更）。
- 未做历史数据审计（是否存在 PRE 与 SHIPPED/RECEIVED 并存）；出现时按离仓守卫保持 open，需人工核对。
- `ListOrders` 批量场景未单独压测（新增 EXISTS 走既有 `(order_no, group_no)` 索引，未改变查询次数）。

### 9.7 反妥协自检清单

**并发/超时/部分成功**
- 本次是纯读派生（每次读重算、无写、无副作用）；读与 Tx2 并发只可能"滞后关闭"，不可能"提前关闭"（PRE 只在未离仓分支写入）。
- 崩溃窗口 `PENDING_REMOTE`：读模型不关闭；既有 worker 按 `refund_id` 幂等重写 PRE，无新 worker。
- `-race` 覆盖 order 全包（121.013s）与目标集（51 PASS）；T17 退款准入 vs 授权发货竞态、崩溃续跑、部分失败回归 PASS。
- 未新增 goroutine/超时/远程调用（读模型只读本库）。

**是否加 if-else 绕过架构**
- 没有。关闭判定仍是**单一函数**、固定顺序；没有在 call site 另判 `refund_state`/`reservation_state`/运单；没有新增持久字段/第二事实源；没有写 `orders.status`。
- 派生字段用 EXISTS 而非 JOIN，避免多准入行放大；无 schema/迁移。

**妥协项（重大高亮）**
1. **【已裁决/高亮】R-2 未实现**：DRAFT ADR 0.1 §3.2 要求退款完成 consumer 重聚合 `orders.status`，与 PO SPEC v1.5（只读模型、不写 `orders.status`）冲突；按 SPEC/Lead Brief 不实现。若 Reviewer 认为仍需要 R-2，必须回 Architect/PO 先改契约，不能由实现自行加状态写入。
2. **【已记录】第 5 支附加 `permit=CONSUMED`（AC-27A 第 5 条原文）**：与 Lead Brief 的 `PreShipAdmission→true` 一行写法在可达状态等价（见 §9.2）；`NONE/其他 permit + PRE` 按第 6 条保持 open，属契约要求而非防御性分支。
3. **【已记录】** 读模型不做 AC-27A 第 6 条告警（写路径已有 log/metric）。
4. **【已记录】** 未做存量审计（AC-27-NOWB-01 独立项）。

**Mock/数据真实性**
- 生产路径用例用真实 MySQL 行 + `AdmitRefund`/`applyAfterSaleCompleted`/`CancelIfCreated`（fake 仅替代跨服务端口），金额 1000/500 符合业务边界；含异常分支（REVIEW/PENDING/POST/缺行、迟到离仓、3 组边界）。
- 未用 fixture 塞 `RELEASED` 驱动关闭（只断言语义边界）。

### 9.8 返工影响声明

- **[修改点]**：`fulfillment_groups.go`（派生 `PreShipAdmission` + EXISTS 查询）；`settlement.go`（`groupIsClosed` 顺序/第 5 支）；新增 `ac27a_closure_unit_test.go`、`ac27a_closure_integration_test.go`；本 selfcheck 追加。
- **[架构侵入性]**：低。未新增持久字段/事件/RPC/worker/migration；未改状态所有权；未写 `orders.status`；关闭谓词仍是唯一 authority，未在 call site 加分支。
- **[副作用评估]**：所有 `loadFulfillmentGroups` 调用方（GetOrder/ListOrders、shipment 投影、allocation validation、tcc precheck）多读一个 EXISTS，语义不变（无 PRE 行时结果与旧行为一致）；permit `REVOKED`/离仓/CANCELLED 排序回归已覆盖。order 全包、aftersales/fulfillment 目标集、`-race` 全绿。
- **[未处置风险]**：AC-27-NOWB-01/AC-27-LIVE-01/AC-50-FIX-01、多组释放缺口、读路径告警缺失；AC-27 仍 AC-REF。

### 9.9 结论

- R0.3-R1c 的 AC-27A 第 5/6 条读模型关闭谓词已实现，并在真实 MySQL 8.4.13（独立 schema、`-race`）验证；`orders.status` 无写入；`refund_state`/`reservation_state` 不参与。
- **不标 AC-VERIFIED / 不声称实现 PASS / Release**；AC-27 仍 AC-REF（前置项未关闭），由 Reviewer 出有范围 verdict。
