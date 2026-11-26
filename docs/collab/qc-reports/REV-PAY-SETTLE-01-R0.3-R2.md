# REV-PAY-SETTLE-01-R0.3-R2 — AC-27 / 回归 / 12 orphan 台账复审

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Tier | **L2（确认）**：涉及退款/履约准入、既有事件消费语义与迁移/台账；不降级 |
| 审查范围 | **R0.3-R2**：AC-27/AC-27A 证据与台账行；回归 AC-14/21/25/27/31/32；12 orphan（AC-39/42/43/47/48/49/56/57/58/59/62/64）reason；INV-44…50 描述/路径/状态核对 |
| review_scope | 仅上述台账/证据与回归；不审 R0.4 实现（另见 R0.4e FAIL）；不批准 merge/Release/AC-27 验收 |
| base | `0872983`（R0.3-R1/R2 实现前；selfcheck 记录） |
| 测试代码 head | **`fe1600bf41b8ab43ab146f08269bfbf3dc3437d0`**（4 files，+294/−3，test-only） |
| selfcheck | **`34d0466`**（`docs/implementation/PAY-SETTLE-01-R0.3-selfcheck.md`，248 行） |
| 本次重跑绑定 HEAD | **`9322e7544abf07f937c3d2adcb00a84b0bef4c26`**；`git diff c38c54f..HEAD` 仅 selfcheck（无代码变化），`fe1600b` 的测试文件在 HEAD 未变 |
| 契约输入 | SPEC-0001 **v1.5**（`4215b3c`；AC-27/AC-27A、§16.2 前置缺陷；QC V15 R2 READY）；SPEC-0002 **v1.10**；ADR-0007 **Rev 2.3**；QC `QC-PAY-SETTLE-01-R0.3-AC27.md`（BLOCKED） |
| spec_version / adr_revision | SPEC-0001 **v1.5**；SPEC-0002 **v1.10**；ADR-0007 **Rev 2.3** |
| integration_owner | System Architect（AC-27A 契约）；Developer（断言/回归）；ops（AC-27-LIVE-01 重放） |
| baseline_impact | **涉及** AC-27/27A、AC-14/21/25/31/32 回归、INV-44..50；不涉及 R0.4 实现与生产代码 |
| INV | INV-44（AC-REF；行内容缺口）；INV-45/46/47/49（DEFERRED）；INV-48/50（AC-REF） |
| affected_AC | AC-27/AC-27A、AC-14/21/25/31/32、12 orphan（AC-39/42/43/47/48/49/56/57/58/59/62/64） |
| verification | §1/§2：AC-27 测试 + 回归在 HEAD `9322e75`（MySQL 8.4.13）重跑；无 SKIP/FAIL |
| 审查日期 | **2026-11-26** |
| Actor | `reviewer-r04`（Reviewer；与 Developer `dev-r03`、Architect、PO、QC 不同 actor） |
| Reviewed-by | `human (merge pending)` |
| release_status | **NOT RELEASE**；不批准 merge / Release / Spec 全量验收 |
| overall verdict | **PASS（R0.3-R2 有范围）**：AC-27 证据/台账核对、回归真实执行、12 orphan 映射、INV 描述诚实性均已完成；**不包含 AC-27 验收**。AC-27 仍为 **AC-REF**（不是 AC-VERIFIED）；Phase 0-R **不可收口** |

> **范围声明**：本报告只审 R0.3-R2 的台账/证据与回归，不审 R0.4 实现（另见 `REV-PAY-SETTLE-01-R0.4.md`：FAIL）。PASS 只覆盖本报告 `review_scope`，不是 Spec 全量验收，不是 Release PASS，也不是 Phase 0-R closeout。

> **2026-11-27 追加**：R0.3-R1c（code `9a57d01`）已实现并验证 AC-27A 第 5/6 条；本报告 §1.2/§2 中“未实现/未覆盖”的表述是 **2026-11-26 历史状态**，已被追加段取代。Phase 0-R closeout 结论见文末：**NOWB-01 OPEN、LIVE-01 BLOCKED、AC-27 仍 AC-REF、NOT RELEASE**。

---

## 0. 结论

**PASS（有范围）。**

1. **AC-27 的已交付断言是真实的**：`fe1600b` 的 EARS-2（`REFUNDED`+`REVOKED`）、`CANCELLED` 组、§7.5 全部退款未发组、post-ship `REFUNDING/REFUNDED` 反向断言都在真实 MySQL/读模型上执行通过；本次在 HEAD `9322e75` 重跑 `TestAC27*` 全部 PASS。
2. **但 AC-27/AC-27A 不能标 AC-VERIFIED**：v1.5 AC-27A 第 5/6 条（`CONSUMED` + `order_refund_admissions.result=PRE_SHIP_REVOKED`、`PENDING_REMOTE`/结果缺失）没有被实现或断言；`groupIsClosed` 仍只读 `Status` + `PermitState`（`internal/services/order/settlement.go:67-78`），没有读取 `order_refund_admissions.result`。`AC-50-FIX-01`、`AC-27-NOWB-01`、`AC-27-LIVE-01` 仍 open（SPEC-0001 v1.5 §16.2），R0.3-R1c 仍 BLOCKED。因此 AC-27 行保持 **AC-REF**（证据可追溯，未验收）。
3. **回归 AC-14/21/25/27/31/32 的真实日志已核对并在 HEAD 重跑**：AC-14 六场景 + 累计上限由本次真实 MySQL 8.4.13 执行覆盖，标 **AC-VERIFIED**；AC-21/25/31/32 有真实执行但 AC 的部分子句未被断言（“至少提示一次”、`任意`场景、168h 上限/用户可见、补退结果可见），保持 **AC-REF**；AC-27 保持 **AC-REF**。SKIP/FAIL 未计入。
4. **12 orphan** 已按 `evidence-matrix.md` §4 映射到 T5.x / T3 dependency；owner/expiry（Developer/2026-12-31）、thread、行序均未改；未删除、未重编号、未批量续期。
5. **INV-45…50 描述/路径/状态诚实**：没有 VERIFIED；INV-48/50 为 AC-REF；INV-45/46/47/49 为 DEFERRED。INV-44 行存在但缺 missing-reservation metric 延期（Lead-owned，见 §4.1）。

**Phase 0-R 是否可收口：NO。** 还缺 R0.3-R1c（AC-50-FIX-01 / AC-27-NOWB-01 / AC-27-LIVE-01 + AC-27A 第 5/6 条实现）、R0.4e 的 F-1/F-2（见另一报告）、INV-44 行补齐、T2/T3/T4.4、ops 告警演练与 Addendum 9 MINOR。**不声称 P0 closure / Release / merge。**

---

## 1. AC-27 / AC-27A（SPEC-0001 v1.5）

### 1.1 已交付断言（真实执行）

| AC-27 / AC-27A 分支 | 测试（`fe1600b` / HEAD 行号） | 真实断言 | 结果 |
|---|---|---|---|
| EARS-2：G2 `refund_state=REFUNDED` + permit `REVOKED` → 组投影 `CANCELLED`；G1 已发不被阻塞 | `warehouse_quote_test.go:125` `TestAC27RevokedGroupIsClosedInProjection` | `effectiveGroupStatus(G2)=CANCELLED`；`shipmentStatusForGroups=SHIPPED`；`aggregateOrderStatus=SHIPPED`；`toProto` 的 order `status=ALLOCATED`、`shipment_status=SHIPPED`、G2 `CANCELLED`+`REFUNDED`、无 group action | **PASS** |
| 同上，真实 DB 行 + 生产写入函数 | `ac27_projection_integration_test.go:16` `TestAC27RefundedGroupProjectionFromRows` | `BlockShipmentForRefund` → `REFUNDING`+`REVOKED`；`applyAfterSaleCompleted` → `REFUNDED`；持久化 `group.status`/`orders.status` 仍 `ALLOCATED`；`toProto` 投影正确 | **PASS**（`-race`） |
| `CANCELLED` 组分支（AC-27A 第 2 条） | `ac27_projection_cancelled_test.go:10` `TestAC27CancelledGroupIsClosedInProjection` | `groupIsClosed(CANCELLED)=true`；全 CANCELLED → `shipmentStatus=UNSHIPPED`；`aggregateOrderStatus=""`（不推进） | **PASS** |
| §7.5 全部退款（未发）：2 组均 `REFUNDED`+`REVOKED` | `ac27_projection_cancelled_test.go:36` `TestAC27AllRefundedUnshippedGroupsStayAllocated` | 组投影 `CANCELLED`+`REFUNDED`；`shipmentStatus=UNSHIPPED`；order 仍 `ALLOCATED` | **PASS** |
| post-ship `REFUNDING`：不得投影 `CANCELLED`、不得当未发组 | `ac27_projection_negative_test.go:9` `TestAC27PostShipRefundingGroupStaysShipped` | `effectiveGroupStatus=SHIPPED`；`shipmentStatus=SHIPPED`；order `SHIPPED`；保留 `receive` | **PASS** |
| post-ship `REFUNDED`：不得回退为未发/提前完成 | `ac27_projection_negative_test.go:42` `TestAC27PostShipRefundedGroupStillCountsAsShipped` | `SHIPPED+REFUNDED` → `SHIPPED`、`PARTIALLY_RECEIVED`；`RECEIVED+REFUNDED` → `RECEIVED`、可 `COMPLETED` | **PASS** |

本次重跑（HEAD `9322e75`，MySQL 8.4.13，`-race`）：

```
ok gin-kratos-gorm/internal/services/order  2.530s
PASS: TestAC27RevokedGroupIsClosedInProjection / RefundedGroupProjectionFromRows /
      PostShipRefundingGroupStaysShipped / PostShipRefundedGroupStillCountsAsShipped /
      CancelledGroupIsClosedInProjection / AllRefundedUnshippedGroupsStayAllocated
```

### 1.2 AC-27A 谓词边界核对（v1.5 第 1–6 条）

`AC-27A` 的固定顺序（SPEC-0001 v1.5 `:616-625`）与当前实现/测试：

| # | 谓词 | 当前实现（`settlement.go:67-78`） | 测试 | 判定 |
|---|---|---|---|---|
| 1 | 组不存在 → 未关闭 | `groupIsClosed(nil)=false` | 无显式用例 | **部分**（代码可读；无断言） |
| 2 | 组持久化 `CANCELLED` → 已关闭 | `Status == CANCELLED` | `ac27_projection_cancelled_test.go:10` | **PASS** |
| 3 | `SHIPPED/RECEIVED` → 未关闭（离仓守卫优先） | 在 permit 判断前返回 false | 两个 post-ship 反向用例 | **PASS** |
| 4 | permit `REVOKED` → 已关闭 | `PermitState == REVOKED` | `warehouse_quote_test.go:125` | **PASS** |
| 5 | permit `CONSUMED` 且 `order_refund_admissions.result=PRE_SHIP_REVOKED` → 已关闭 | **没有读取** `order_refund_admissions.result`；仍只读 `Status`+`PermitState` | **无** | **FAIL（未实现/未覆盖）** |
| 6 | 其他（`NONE`、`PENDING_REMOTE`、`POST_SHIP_ALLOWED`、结果缺失/不可核验）→ 未关闭 | `groupIsClosed=false`（permit 非 REVOKED），未告警 | 无 `PENDING_REMOTE`/缺失 result 用例 | **部分**（代码方向对；无 AC-27A 第 6 条断言） |
| — | 迟到/重复 `SHIPPED/RECEIVED` 的离仓守卫与告警 | 无 AC-27A 的新读模型分支 | 无 | **未覆盖** |
| — | `refund_state`/`reservation_state` 不参与谓词 | 代码不读这两个字段 | fixture 使用它们但不作为 predicate | **PASS（不参与）** |
| — | 只读 Order 自有事实，不要求 Order 观测运单 | `groupIsClosed` 不读运单 | 无运单依赖 | **PASS（方向）** |

**结论：AC-27A 第 5/6 条（v1.5 的核心关闭谓词）未实现/未断言。** 这不是“测试绿但漏分支”的轻微问题：第 5 条正是 `CONSUMED+无运单`/`PRE_SHIP_REVOKED` 的关闭依据，且与 `AC-50-FIX-01` 绑定。R0.3-R1c 在 `docs/lead/PAY-SETTLE-01/tasks.md` 中仍为 **BLOCKED（contract prerequisites）**。

### 1.3 为什么 AC-27 仍是 AC-REF，而不是 AC-VERIFIED

- SPEC-0001 v1.5 `:645`（AC-27A 注）与 `:866`（§16.3）明确：`AC-50-FIX-01`、`AC-27-NOWB-01`、`AC-27-LIVE-01` 未关闭前，不得声称 AC-27 EARS-1/EARS-2 收敛，不得标 AC-VERIFIED。
- `AC-50-FIX-01`：`CONSUMED+无运单` 写路径/零元路径仍把“无运单行”映射为 `CANCELLED`/PRE；ADR §5.3 `CancelIfCreatedReply` 仍缺“无运单”结果。**未修**。
- `AC-27-NOWB-01`：`CONSUMED+无运单` 人工核对终态/owner 尚未闭环。**未修**。
- `AC-27-LIVE-01`：`aftersale.completed` DLQ/租约丢失使 `PENDING_REMOTE` 永久不收敛的 AC-61 人工重放入口尚未闭环。**未修**。
- 因此本次测试通过只证明“已断言的 v1.5 子集真实”，不证明 AC-27/AC-27A 全量验收。**AC-27 行 = AC-REF**（不是 DEFERRED：证据存在且可追溯；不是 AC-VERIFIED：谓词/前置缺陷未闭合）。

---

## 2. 回归 AC-14/21/25/27/31/32

### 2.1 原始日志（`34d0466` / `fe1600b`）

- `34d0466` 是 selfcheck 的 docs-only 提交（`git show --stat 34d0466` 仅 `docs/implementation/PAY-SETTLE-01-R0.3-selfcheck.md`）。
- selfcheck 记录的原始日志在 `.tmp/evidence-r03/final-*.log`，均绑定 `HEAD=fe1600bf…`，真实 MySQL（mysql-unicorn 26.10.0），无 SKIP：
  - `final-aftersales.log`：AC-14 六子场景 + AC-31 + AC-32，`ok 1.589s`；
  - `final-fulfillment.log`：AC-21，`ok 0.459s`；
  - `final-order-race.log`：AC-25 + 6 个 AC-27，`-race`，`ok 2.226s`；
  - `final-order-full.log`：order 全包 `ok 89.268s`；
  - `final-build.log`：`go build ./...` exit 0。
- 命令、耗时、行号见 selfcheck §3（`:100-139`）；日志与 selfcheck 一致，未见伪造/替换。

### 2.2 本次在 HEAD 重跑（MySQL 8.4.13）

```bash
# HEAD=9322e75；每包独立 schema
TEST_MYSQL_DSN=.../mall_integration_reviewer_as \
  go test ./internal/services/aftersales -run '^(TestAC14FreightRefundSixScenarios|TestAC31FreightHoldTimeout|TestAC32FreightHoldTimeoutDefaultsToMerchant)$' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_reviewer_ff2 \
  go test ./internal/services/fulfillment -run '^TestAC21AutoConfirmT7$' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_reviewer_order2 \
  go test -race ./internal/services/order \
  -run '^(TestAC25RefundProjectionKeepsLifecycle|TestAC27(RevokedGroupIsClosedInProjection|RefundedGroupProjectionFromRows|PostShipRefundingGroupStaysShipped|PostShipRefundedGroupStillCountsAsShipped|CancelledGroupIsClosedInProjection|AllRefundedUnshippedGroupsStayAllocated))$' \
  -count=1 -v
```

| 命令 | 结果 |
|---|---|
| aftersales AC-14/31/32 | **PASS，1.759s**；6 子场景全 PASS |
| fulfillment AC-21 | **PASS，0.391s** |
| order `-race` AC-25/27 | **PASS，2.530s** |
| 整体 | **EXIT=0，无 SKIP/FAIL** |

原始日志：`.tmp/evidence-r03/reviewer-regression-HEAD.log`。

### 2.3 台账判定（`ac-coverage.tsv`）

| AC | 判定 | 依据 / 缺口 |
|---|---|---|
| **AC-14** | **AC-VERIFIED** | 六场景与 AC-14 的 6.5.1 六种场景一一对应；累计上限在测试尾部断言；真实 MySQL 8.4.13 at HEAD PASS |
| **AC-21** | **AC-REF** | 自动确认 T7 的真实断言有；AC-21 的“此前订单详情至少提示过 1 次”未断言（测试只做 job/收货断言） |
| **AC-25** | **AC-REF** | 部分/全额退款不写 `orders.status` 的真实断言有；AC 的“任意退款场景”未穷举（代表性用例，不冒充全量） |
| **AC-27** | **AC-REF** | 见 §1；v1.5 AC-27A 第 5/6 条未覆盖，前置缺陷 open |
| **AC-31** | **AC-REF** | 商品退款 + 运费 hold 有真实断言；168h 上限与“待判定展示”未断言 |
| **AC-32** | **AC-REF** | 超时 → `MERCHANT_WAREHOUSE`/`MERCHANT_FULL_FREIGHT` 补退、规则/金额断言有；用户可见补退结果未断言 |

> 本轮新增 **AC-VERIFIED 仅 AC-14**。其余按证据保持 AC-REF；未把 SKIP/FAIL 计入，也未把 AC-REF 写成验收通过。

---

## 3. 12 orphan 映射（`evidence-matrix.md` §4）

`docs/lead/PAY-SETTLE-01/evidence-matrix.md` §4 的映射已写入 `ac-coverage.tsv` 的 `reason`（owner/expiry/thread/status 不变；未删除、未重编号、未批量续期）：

| AC | 新 reason（摘要） | 映射 |
|---|---|---|
| AC-39 | `T5.3 fault injection: channel exception and amount != pay_cent refund (was orphan)` | T5.3 |
| AC-42 | `T5.1 contract/handler: captured-unsettled shows processing, PAID != shippable (was orphan)` | T5.1 |
| AC-43 | `T5.3 fault injection: crash-resume after capture; HELD/CONFIRMED converge on CANCELLED (was orphan)` | T5.3 |
| AC-47 | `T5.3 fault injection: inventory shortage/technical retry; shortage hands to AC-56 (was orphan)` | T5.3 |
| AC-48 | `T5.2 -race: permit NONE/CONSUMED/REVOKED, line confirm, Ship reads state (was orphan)` | T5.2 |
| AC-49 | `T5.1 contract/handler: four facts emitted separately (was orphan)` | T5.1 |
| AC-56 | `T5.2/T5.3 -race+fault: 4h/48h, restock, concurrent CAS (was orphan)` | T5.2/T5.3 |
| AC-57 | `T5.5 baseline ADR-0005 receipt tests: cite/rerun then AC-REF (was orphan)` | T5.5 |
| AC-58 | `T5.5 baseline ADR-0005 dispute tests: cite/rerun then AC-REF (was orphan)` | T5.5 |
| AC-59 | `T5.3 fault injection: refund/receipt/stock-failure/shortage out-of-order rejection (was orphan)` | T5.3 |
| AC-62 | `T3 dependency: P1-T3 commitment release; not Phase 0-R/T5 (was orphan)` | T3.1–T3.3 |
| AC-64 | `T3 dependency: P1-T3 line/freight ledger; not Phase 0-R/T5 (was orphan)` | T3.4–T3.6 |

- AC-62/AC-64 只关闭“orphan 无归属”问题，**不关闭 AC 本身**；仍是 DEFERRED。
- AC-57/58 仍是 DEFERRED（本轮未重跑既有 ADR-0005 测试；T5.5 要求 cite/rerun 后再标 AC-REF，不能提前）。
- 行数保持 42 数据行；header/列序未变；owner 列仍为测试 owner Developer，台账 owner 为 Reviewer。

---

## 4. INV-44…50 核对

### 4.1 INV-44（evidence-matrix 行）

`docs/lead/PAY-SETTLE-01/evidence-matrix.md:64` 行存在，状态 **AC-REF**（未 VERIFIED），描述覆盖了组退款库存处置、单组 P0、多组 P1/manual。但相对 Addendum §2.4（`:333-337`）仍缺：

- **missing-reservation metric/告警 DEFERRED** 的显式登记（行内只在状态写“多组释放缺口 DEFERRED”）；
- “缺 reservation 行 no-op”分支；
- 重复投递/DLQ 重放的 evidence 分支（行内有幂等描述，但未列重放/DLQ 证据）。

这是 **Lead-owned**（不是 Developer 代码）；在补齐前 **不得声称 INV-44 有证据/已收口**。R0.4e 复核会把它列为 R0.4 收口的 Lead-owned 缺口（见 R0.4e 报告 N-1）。

### 4.2 INV-45…50

| INV | matrix 状态 | 诚实性核对 |
|---|---|---|
| INV-45 | **DEFERRED（T3）** | 描述覆盖“打款前 REJECTED/CANCELLED”“FAILED_PERMANENT/人工放弃 + 授权”“只释放额度/不恢复 permit/不当作退款完成/只回同组”；证据是 `0018` 表/约束 + T3 planned，未冒充行为测试；**不得 VERIFIED** |
| INV-46 | **DEFERRED（T2）** | 描述覆盖 24h 必须离开 `OPEN/RESOLVING`、`COMPENSATING`/`EXPIRED_*`、Payment.Refund 最后；证据是 T2 planned + 0022 CHECK；**writer/worker 未实现，不得称闭环** |
| INV-47 | **DEFERRED（T3）** | 描述覆盖行级/双层额度、商品+运费、责任唯一、`UNDETERMINED` 运费 ≤168h、后置补退同组；证据 T3 planned；**不得 VERIFIED** |
| INV-48 | **AC-REF（未 AC-VERIFIED）** | T17 六测试（`:26,83,204,306,371,442`）在 `fe1600b` PASS；本次 HEAD 重跑 `^TestT17` **PASS 6.150s**；仍缺 T5 `-race`/fault 全矩阵；状态诚实 |
| INV-49 | **DEFERRED（T4.4）** | 描述覆盖 shadow 不具准入权威、默认关闭、切换门禁/人类批准、切换后 `order.paid` 不得作准入依据；证据 T4.4 planned + ADR §12.3；**不得 VERIFIED** |
| INV-50 | **AC-REF（未 AC-VERIFIED）** | 证据指向 `aftersales/zero_refund_integration_test.go:497/:182` + 反向 `:562/:711`；本次 HEAD 重跑 12 个 zero/boundary 用例 **PASS 6.649s**；仍是 AC-REF（AC-67/68 未验收）；状态诚实 |

- **没有任何 INV 被写成 VERIFIED**；INV-48/50 保持 AC-REF；INV-45/46/47/49 保持 DEFERRED。
- INV-44 的缺口见 §4.1；其余行描述/路径/状态与 SPEC-0002 v1.10 一致，未发现“把 T2/T3/T4.4 计划写成已完成”的表述。

---

## 5. Phase 0-R 是否可收口

**NO。** 按 Lead Plan 与 SPEC v1.5 §16.2/§16.3，至少还缺：

| 缺口 | 状态 / owner |
|---|---|
| **R0.3-R1c**：AC-27A 第 5/6 条（`CONSUMED+PRE`、`PENDING_REMOTE`）实现/断言 | BLOCKED（contract prerequisites）；Developer |
| **AC-50-FIX-01**：`CONSUMED+无运单` 写路径/零元路径 + ADR §5.3 additive amendment | open；Developer（Architect 先 amendment），2026-12-31 |
| **AC-27-NOWB-01**：人工核对终态/owner | open；Developer/ops/Fulfillment，2026-12-31 |
| **AC-27-LIVE-01**：DLQ/租约丢失 → AC-61 人工重放 | open；ops/Developer，2026-12-31 |
| **R0.4e**：F-1 锁/MDL/维护窗；F-2 preflight 环境覆盖 | FAIL（见 `REV-PAY-SETTLE-01-R0.4.md`）；Developer + ops/Lead |
| **INV-44 行**：missing-reservation metric 延期 | Lead，2026-12-31 |
| T2 24h writer/worker、`legacy_24h` source；T3 commitment release；T4.4 shadow | DEFERRED，2026-12-31 |
| Addendum 9 MINOR（S-1…S-9）与 ops 到期告警演练 | ROUTED；Architect/Lead/ops，2026-12-31 |

因此：**Phase 0-R 未闭合；无 P0 closure、无 Release、无 merge、无 Spec 全量验收。** R0.3-R2 的 PASS 只表示台账/证据核对完成，不表示这些缺口已关闭。

---

## 6. 宪法 Article I–V / Governance 核对

| 条款 | 结论 | 依据 |
|---|---|---|
| I.1 拒绝“屎山雕花” | **PASS** | 本轮 test-only；未新增生产 if-else/状态字段；不可表达分支按反证义务停手（selfcheck §2.2） |
| I.2 Fail-Closed | **PASS（测试）/ 不适用生产** | 断言未把退款进度写入 `orders.status`；AC-27A 未实现部分被明确标 BLOCKED，不伪装放行 |
| I.3 契约优先 | **PASS** | 未改契约；AC-27/27A 以 SPEC v1.5 为准；Developer 未自创业务语义 |
| I.4 拒绝自我妥协 | **PASS** | 三支 BLOCKED 如实登记；未用 fixture 伪断言“已关闭”；AC-27 未标 VERIFIED |
| II.1 事实分离 | **PASS** | 未把 `refund_state`/`reservation_state`/运单事实混入 `orders.status`；测试断言读模型与持久化分离 |
| II.3 幂等/最终一致 | **PASS（未改）** | 只改测试；无新写路径/事件/RPC |
| II.5 退款/履约准入 | **PASS（未改）** | 未改 permit/准入；AC-27A 第 5 条未实现，已列为阻断 |
| III 技术栈/静态 | **PASS** | 无生产代码、无 schema、无 proto；测试用既有 MySQL harness |
| IV.1 证据不可妥协 | **PASS** | 真实 MySQL 日志；SKIP 不计；AC-REF/AC-VERIFIED 区分；AC-27 不 VERIFIED |
| IV.2 重灾区抗压 | **PASS（有范围）** | AC-25/27 `-race`；T17 崩溃/部分失败/竞态 PASS；未跑 T5 全 matrix，已明确 |
| IV.4 实现/发布隔离 | **PASS** | NOT RELEASE；不声称 Spec 全量验收 |
| Workflow 9 stop-the-line | **本报告不触发**（PASS 有范围）；R0.4e FAIL 触发 R0.4 冻结 | 见 R0.4e 报告 |
| Governance 4 人类检查点 | **保留** | 不批准 merge；人类 merge pending |
| Governance 6 人类豁免 | **PASS** | 仅引用 WIP waiver，不扩大 |
| 角色独立性 | **PASS** | Actor `reviewer-r04` ≠ Developer `dev-r03`；只读代码/契约，仅更新 Reviewer-owned `ac-coverage.tsv` |

**`[ARCHITECTURAL_FLAW]`：无（本范围）。** AC-27A 第 5 条未实现是已知的前置缺陷/实现范围问题，不是本轮 test-only 交付引入的架构缺陷；修复必须走 AC-50-FIX-01 的 ADR amendment + 后续 change_id，不得在测试里加 `if` 伪造。

---

## 7. 建议（PASS 最多 3 条）

1. **AC-27 保持 AC-REF，待 `AC-50-FIX-01`/`AC-27-NOWB-01`/`AC-27-LIVE-01` 关闭后**，由 Architect 先做 ADR §5.3 additive amendment，再由 Developer 实现 AC-27A 第 5/6 条（读 `order_refund_admissions.result`，`PENDING_REMOTE` 不关闭/告警），最后重新绑定 HEAD 做 AC-VERIFIED 复核。不得把本次 AC-REF 直接升级。
2. **回归 AC-21/25/31/32 保持 AC-REF**：分别补“至少提示一次”、`任意`退款场景/全生命周期枚举、168h 上限/用户可见 hold、补退结果可见的断言；补齐后再由 Reviewer 复核，不批量续期。
3. **INV-44 行由 Lead 补齐** missing-reservation metric/告警 DEFERRED 与缺行 no-op 分支；补齐前 `INV-44` 不得写成“有证据/已完成”。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-26
**报告**：`docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.3-R2.md`


---

# 2026-11-27 Phase 0-R closeout 追加：AC-27-NOWB-01 / AC-27-LIVE-01

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Tier | **L2（确认）**：资金/履约准入人工终态与重放入口；不降级 |
| review_scope | **只评估** `AC-27-NOWB-01` / `AC-27-LIVE-01` 能否用现有证据关闭；不新写代码；不重开 R0.3-R1c/T5 |
| base | **`ce0f6b4`**（R0.3-R1c resume） |
| code head | **`9a57d012254fcad8c34d9af7c48b2aa398c36ccb`**（R0.3-R1c；此后无 `internal/`/`migrations/` 变化） |
| head | **`474700d8a71361dc128e2520534650384f8057b1`**（Phase 0-R resume；docs-only after code） |
| 契约输入 | SPEC-0001 v1.5 §16.2 `:863-864`、AC-27A `:632-633`；`REV-PAY-AC50-REVIEW-01-FINAL.md`；`docs/ops.md` AC-72 runbook/re-arm；AC-61/62 台账 |
| verification | §A：现有 `TestAC50ReviewAuditAndRearmRetention|TestAC50ReviewConsumerNoAutoPre`（1.743s）与真实 NATS `TestRefundAdmittedConsumerReviewAndDuplicate`（2.019s）在 HEAD 474700d 复跑 PASS；`migrations` 空；无新代码 |
| release_status | **NOT RELEASE** |
| overall verdict | **FAIL（closeout scoped）**：NOWB-01 **OPEN**，LIVE-01 **BLOCKED**；AC-27 保持 **AC-REF** |

## A. 现有测试复跑（证明“测试/runbook ≠ 运维能力”）

```bash
# MySQL 8.4.13，独立 schema
TEST_MYSQL_DSN=.../mall_integration_nowb go test ./internal/services/order \
  -run 'TestAC50ReviewAuditAndRearmRetention|TestAC50ReviewConsumerNoAutoPre' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_nowb_ff TEST_NATS_URL=nats://127.0.0.1:4222 \
  go test ./internal/services/fulfillment -run 'TestRefundAdmittedConsumerReviewAndDuplicate' -count=1 -v
```

| 测试 | 结果 | 能证明什么/不能证明什么 |
|---|---|---|
| `TestAC50ReviewAuditAndRearmRetention` | **PASS 1.743s** | re-arm CAS/审计/版本/`created_at` 保留；**测试模拟，不是 ops 入口** |
| `TestAC50ReviewConsumerNoAutoPre` | **PASS（同轮）** | REVIEW replay 不自动 PRE；**不是 DLQ 租约恢复** |
| `TestRefundAdmittedConsumerReviewAndDuplicate` | **PASS 2.019s** | 测试内 DLQ/重复 PRE/POST no-op；**不是 AC-61 人工重放入口** |
| `migrations` / 生产代码 | **无变化** | `git diff 9a57d01..HEAD -- internal migrations` 为空 |
| selfcheck R0.3-R1c | AC-27A 5/6 PASS（9a57d01） | 不是 NOWB/LIVE 终态 |

## B. AC-27-NOWB-01：**OPEN（不能关闭）**

SPEC 终态（`docs/specs/0001-fulfillment-quantity-split.md:863`）：
① Fulfillment S6 取消证据 + ops re-arm→PRE；② 无法补齐→申请保持 PENDING/人工队列，不写 PRE/不打款/不释放/不自动关闭，人工放弃须 AC-62 + SPEC-0002 owner 书面口径。

| 检查 | 结论 | 文件:行号 |
|---|---|---|
| O19 普通售后 REVIEW 无 `refund_applications` 行 | **PASS（保持不误持久化）** | `internal/services/order/ac50_review_integration_test.go:751` |
| O22 第 8 次 REVIEW 仍 PENDING、非 REVIEW 才 FAILED_PERMANENT | **PASS（不自动终态）** | `internal/services/aftersales/review_task_integration_test.go:64` |
| O20 审计/re-arm 保留 canonical/version/created_at | **PASS（测试模拟）** | `internal/services/order/ac50_review_audit_fault_test.go:18` |
| runbook 人工队列查询/SLA/不自动关闭 | **PASS（流程文本）** | `docs/ops.md:853`（明细）、`:871`（never auto-close） |
| runbook re-arm 步骤（ticket/audit/CAS/replay） | **PASS（文本步骤）** | `docs/ops.md:895-905` |
| **终态①：真实 S6 证据 + ops re-arm→PRE 的实际入口** | **OPEN/FAIL** | runbook 仅说“only with real S6 evidence ... Replay”（`docs/ops.md:895-905`），没有 S6 证据登记/校验工具、没有 ops replay 入口；`TestAC50ReviewAuditAndRearmRetention` 是测试。 |
| **终态②：无法补齐→人工队列 + 人工放弃 AC-62** | **OPEN/FAIL** | `docs/ops.md:871` 仅禁止自动 `FAILED_PERMANENT`，没有人工放弃流程/AC-62 授权入口；AC-62 台账仍 **AC-REF**（`docs/collab/ac-coverage.tsv:31`）。 |
| 组保持未关闭/不打款/不释放 | **PASS** | `docs/ops.md:863-871`；R1c 读模型保持 open；T5/T6 测试 |
| **NOWB-01 整体** | **OPEN（FAIL closeout）** | ①②均无真正可执行的运维闭环；O19/O22/O20+runbook 只能证明“保持未关闭/不自动收敛/可审计 re-arm”，不能证明终态可执行。 |

**最小剩余工作（不新写代码，登记）**：① 由 Fulfillment/ops 提供真实 S6 证据的登记/校验与授权工单，ops 用现有 re-arm CAS 后以 `AdmitRefund`/等价 replay 入口重放；② 定义 AC-62 人工放弃/`FAILED_PERMANENT` 的授权与审计流程（AC-62 实现或书面口径）；owner Developer/ops/Fulfillment，expiry 2026-12-31。未完成前 AC-27 不得 AC-VERIFIED。

## C. AC-27-LIVE-01：**BLOCKED（不能关闭）**

SPEC 终态（`docs/specs/0001-fulfillment-quantity-split.md:864`）：DLQ/租约丢失导致 `PENDING_REMOTE` 永久不收敛时，提供 **AC-61 人工重放入口**；重放收敛→PRE/关闭，仍冲突→NOWB 人工队列。

| 检查 | 结论 | 文件:行号 |
|---|---|---|
| T6c NATS DLQ/重放用例 | **PASS（测试范围）** | `internal/services/fulfillment/review_consumer_integration_test.go:24`；仅消费者测试 |
| AC-61 实际人工重放入口（按 payment_no/refund_id/after_sale_no/batch_id） | **不存在** | `api/proto/mall/v1/mall.proto:493` 只有内部 `AdmitRefund`；`:1324` 的 `ReplayDLQ` 是秒杀，不是退款；无 ops/BFF/CLI 入口 |
| runbook “Replay the original refund_id” | **仅文本** | `docs/ops.md:899`（无命令/工具/权限入口） |
| AC-61 台账 | **AC-REF** | `docs/collab/ac-coverage.tsv:30`（manual replay deferred） |
| **LIVE-01 整体** | **BLOCKED** | T6c 是测试/runbook，不是运维能力；租约丢失/DLQ 无实际 replay 入口，故不可关闭。 |

**最小剩余工作（BLOCKED）**：实现 AC-61 人工 replay 入口（ops 鉴权 + 审计 + 幂等，按 `payment_no/refund_id/after_sale_no/batch_id`），覆盖 DLQ 重放与 AfterSales durable task 租约恢复；owner ops/Developer/AfterSales，expiry 2026-12-31；需新 change_id/Plan（超出本次只读范围）。未完成前不得用测试冒充运维能力。

## D. AC-27 台账结论

- AC-27A 5/6 的 R0.3-R1c **PASS** 仍成立（`9a57d01`；`REV-PAY-SETTLE-01-R0.3-R1c.md`）。
- **AC-50-FIX-01**：写路径已由 L2 `PAY-AC50-REVIEW-01` 交付（AC-50 **AC-VERIFIED**）。
- **AC-27-NOWB-01**：**OPEN**；**AC-27-LIVE-01**：**BLOCKED**。
- 因此 **AC-27 保持 AC-REF**，不升级 AC-VERIFIED；`ac-coverage.tsv` 的 AC-27 reason 已更新为上述缺口，status 仍 AC-REF。
- **Phase 0-R 不可收口**；`NOT RELEASE`；T2/T3/T4.4/T5/Release 继续 gated。

## E. 残余 / 边界

1. NOWB 终态①的 S6 证据工作流/ops replay 入口，终态②的 AC-62 人工放弃；owner Developer/ops/Fulfillment，2026-12-31。
2. LIVE AC-61 人工 replay 入口缺失；owner ops/Developer/AfterSales，2026-12-31；`[BLOCKED]`。
3. AC-61/AC-62 台账仍 AC-REF；AC-72 ALERTS/ENTRY 仍 DEFERRED；F7b/W8 残余不变。
4. 本报告只做评估，未改生产代码/SPEC/ADR/migrations；`NOT RELEASE`。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-27
**报告**：`docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.3-R2.md`（追加 Phase 0-R closeout 段）
