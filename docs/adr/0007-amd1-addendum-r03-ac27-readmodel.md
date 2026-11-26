# ADR-0007 Amendment 1 Addendum — R0.3-R1b：AC-27 读模型裁决（运单 CANCELLED / 释放可观察性 / pre-ship REFUNDING / SPEC 漂移）

| 项 | 值 |
|---|---|
| Status | **SUPERSEDED / WITHDRAWN（2026-11-27）**——不再作为实现依据，仅保留历史；R-2 撤回且未实现 |
| Revision | **0.1** |
| Change ID | `PAY-SETTLE-01` |
| Tier | **L2** |
| Tier-Rationale | 裁决对象是资金/履约准入后的 Order 读模型与生命周期投影（P0、资金、状态机语义、既有事件消费效应）；按 `docs/collab/CONVENTIONS.md:90` 任一命中即 L2；后续最小实现虽在单一服务内，仍按 L2 走完整阶梯 |
| Spec-Ref | SPEC-0001 **v1.4**：AC-25（`0001:563-565`）、AC-26（`0001:567-569`）、AC-27（`0001:571-578`）、§6.6（`0001:279-287`）、§6.6.1（`0001:289-295`）、§6.7（`0001:297-303`）、§7.1–§7.5（`0001:364-409`）；SPEC-0002 **v1.10**：S4/S5/S6/S9（`0002:74-79`）、补充约束（`0002:86-104`）、INV-44/INV-45（`0002:128-129`）、INV-50（`0002:134`）、AC-48（`0002:201-206`）、AC-50（`0002:213-231`）、AC-62（`0002:308-323`）、AC-65（`0002:371-379`） |
| Base（仓库） | `34d04664d8564b3d584bef05241f1acc8c5c4ca2`（`docs(dev): PAY-SETTLE-01 R0.3-R1/R2 selfcheck`） |
| Base（契约基线） | SPEC-0001 v1.4（approved 2026-11-24）、SPEC-0002 v1.10（approved 2026-11-25）、ADR-0007 **Rev 2.3**（accepted 2026-11-25，`docs/adr/0007-payment-settlement-admission.md`） |
| Code head（裁决针对的实现证据） | `fe1600bf41b8ab43ab146f08269bfbf3dc3437d0`（Developer R0.3-R1/R2 证据，`docs/implementation/PAY-SETTLE-01-R0.3-selfcheck.md:10`） |
| Doc head | 本文件提交；QC/Reviewer 复核时必须记录精确 SHA（本文件不自引 SHA） |
| Integration owner | System Architect（本 Addendum 作者；Order 读模型裁决）；实现 owner = Developer（R0.3-R1c） |
| allowed_paths（本文件） | 仅 `docs/adr/0007-amd1-addendum-r03-ac27-readmodel.md` |
| allowed_paths（本裁决授权的 R0.3-R1c 最小实现） | `internal/services/order/settlement.go`（关闭谓词/投影语义）、`internal/services/order/fulfillment_groups.go`（读模型加载）、`internal/services/order/aftersale_consumer.go`（退款完成投影触发）、`internal/services/order/consumer.go`、`internal/services/order/repo.go`（如需抽出单一生命周期推进 authority）、`internal/services/order/*_test.go` |
| forbidden_paths | `migrations/**`、`api/**`、`internal/platform/**`、`internal/services/{inventory,fulfillment,payment,aftersales}/**`、`docs/specs/**`、`docs/collab/**`、`docs/lead/**`、`docs/implementation/**` |
| baseline_impact | **涉及**：Order 生命周期投影与组级关闭语义（AC-25/AC-26/AC-27、S5 permit 门禁 AC-48）。不改 permit 状态机、不改退款额度、不改释放执行、不改 Payment 事实；非回归范围见 §9 |
| INV | INV-01、INV-06、INV-24、INV-38、INV-44、INV-45、INV-50（逐条见 §9） |
| Actor | `arch-ac27`（System Architect） |
| Reviewed-by | **待 QC**（本文件不代签；DRAFT 期间不得合入受管代码，不得标 PASS） |
| Scope | 只裁决：BLOCKED-1 运单 `CANCELLED`、BLOCKED-2 `reservation released`、BLOCKED-3 pre-ship `REFUNDING`、SPEC-0001 §7.5:405 描述不一致。不写代码、不改 SPEC、不改已 accepted ADR Rev 2.3 |
| 硬边界 | 不改 SPEC、不改已 accepted ADR Rev 2.3、不新增持久化字段/列/事件字段、无迁移、不新增 subject/RPC、不新增 worker；本文件未明确列出的变更一律不做 |
| Superseded-by | SPEC-0001 **v1.5**（approved，2026-11-27）**AC-27A**；R0.3-R1c 实现（PASS）；AC-27 已按 waiver C+A 升 **AC-VERIFIED** |
| Withdrawal | **R-2 撤回、未实现**（与 SPEC-0001 v1.5 冲突）；R-1/R-3/R-4/R-5 已由 SPEC v1.5 AC-27A 与 R0.3-R1c 取代 |

> **SUPERSEDED / WITHDRAWN（2026-11-27）**：本文件**不再作为实现依据，仅保留历史**。R-2（退款完成写 `orders.status`）与已批准的 SPEC-0001 v1.5 冲突，**撤回、未实现**；R-1/R-3/R-4/R-5 已被 SPEC-0001 v1.5 **AC-27A** 与 **R0.3-R1c** 取代。以下正文为 2026-11-27 之前的历史裁决，不得据此施工或验收；现行契约以 SPEC-0001 v1.5、SPEC-0002 v1.10 与 ADR-0007 Rev 2.3 为准。

> 本文件是**设计裁决**，不是实现、不是验收。所有代码/测试/台账状态归 Developer/Reviewer；§7.5 修正归 Product Owner。本文件中的任何“应当”都不免除 QC 与人类批准。

---

## 0. 阅读方式与规范边界

1. 业务结论只活在 Spec（`docs/collab/CONVENTIONS.md:11`）。本裁决不创造新业务规则，只做两件事：
   - **解释**既有 AC-27 EARS 正文需要哪些 Order 自有事实才能被满足；
   - **指出**哪些表述互相矛盾（SPEC-0001 内部）并指明 owner。
2. SPEC-0001 内部冲突时，**AC 正文（EARS）优先于派生示例表**：§7.5 是“组合投影示例”，AC-27 是可验收条款（`0001:447`：“每条 AC 必须可由测试人员在不了解实现的前提下判定”）。
3. 代码与已批准 SPEC/ADR 冲突时停止交付并由所有者裁决（`CONVENTIONS.md:66`）；描述性文档漂移由 owner 修正，**不得用代码静默迁就描述**（`CONVENTIONS.md:67-68`）。
4. 本裁决不改变事实所有权（`0002:67-104`、ADR Rev 2.3 §1.1 `0007:171-187`）。判据始终是：**谁拥有该事实的写入权**，而不是“读起来方便”。

事实与状态速查（全部为已批准口径）：

| 事实 | Owner | 本裁决用到的持久形态 | 禁止承担 |
|---|---|---|---|
| S4 库存可履约/预留 | **Inventory** | `HELD/CONFIRMED/RELEASED`（`0002:74`） | 不由 Order 状态推导；`RELEASED` 不等于已退款 |
| S5 发货许可 | **Order** | `order_shipping_permits.state ∈ {NONE,CONSUMED,REVOKED}`（`0002:75`） | 不表示运单已离仓；不得由 `orders.status` 推导 |
| S6 运单/签收 | **Fulfillment** | `shipments`；**Order DB 无该事实**（`0002:76`、`ports.go:47-55`） | Order 不得制造签收/运单事实 |
| S9 组退款进度 | **Order（投影）** | `order_fulfillment_groups.refund_state`（`0002:79`） | **不是禁发裁决点**（`0002:79`、`0002:89`） |
| RefundAdmission | **Order** | `order_refund_admissions(refund_id PK, result)`（`0002:95`、ADR §1.1 `0007:181`） | 不是打款事实 |

---

## 1. 裁决摘要

| # | 问题（Developer 证据） | 裁决 | 是否需要生产代码 | 依据锚点 |
|---|---|---|---|---|
| **R-1** | **BLOCKED-1** 运单 `CANCELLED`：pre-ship permit 已 `CONSUMED`，`CancelIfCreated` 取消运单后 permit 仍 `CONSUMED`，退款完成后 `groupIsClosed` 仍 false，组停在 `ALLOCATED`（`selfcheck:58-75`） | **不新增运单投影、不消费 `shipment.cancelled`**。Order 读模型以 **Order 自有的 pre-ship 准入事实 `order_refund_admissions.result=PRE_SHIP_REVOKED`** 作为“该组已按未离仓完成退款准入、不会再产生包裹”的判据；`refund_state` **不进入**关闭谓词 | 是（仅 Order 读模型） | AC-50 `0002:222`；ADR §5.2.2–§5.2.4 `0007:560-596`；INV-45 `0002:129` |
| **R-2** | BLOCKED-1 的**收敛缺口**：只改 `groupIsClosed` 仍可能停在 `ALLOCATED`，因为没有事件在“组关闭”后重新聚合订单生命周期（`selfcheck:71`） | **必须在既有退款完成投影 consumer 内单事务重聚合**（复用既有唯一生命周期推进 authority）；否则 R-1 只修了显示、没修 EARS-1 的“不得停留在 `ALLOCATED`” | 是（Order 投影 consumer） | AC-27 EARS-1 `0001:572-573`；consumer `consumer.go:107-143`；AFTERSALE `aftersale_consumer.go:39-56` |
| **R-3** | **BLOCKED-2** reservation released：退款释放只发 outbox，不回写 `order_fulfillment_groups.reservation_state`（`refund_admission.go:285,599`；写点仅 `fulfillment_groups.go:58-62,71-80`） | **不要求 Order 可观察，且禁止 Order 写 `RELEASED`**。AC-27 EARS-3 是**规范判定**（该组合合法），不是可观察性要求；释放事实 owner 是 **Inventory**（S4/INV-44）。多组订单当前**根本不发释放请求**（`refund_release.go:182-186`，P1/manual），这是独立已知缺口，不得由本裁决放行 | 否（生产代码零改动） | AC-27 EARS-3 `0001:577-578`；§6.6:287；§6.6.1:289-295；INV-44 `0002:128`；ADR §5.7 `0007:706-724` |
| **R-4** | **BLOCKED-3** pre-ship `REFUNDING`：permit `REVOKED` + `refund_state=REFUNDING` 的未发组当前已投影 `CANCELLED`（`settlement.go:67-78`），订单可推进 | **正确，保持**。INV-45 的 permit 终态支持“关闭但退款在途”；Lead 原 Brief“退款未完成不得投影 `CANCELLED`”一句**不成立**（AC-27 正文无此要求，且 Lead 已于 `tasks.md:47` 撤回）；新增负向要求会制造永久死组 | 否（仅补回归断言） | S5 `0002:75,89`；INV-45 `0002:129`；AC-62 `0002:310,314`；AC-27 EARS-2 `0001:574-576` |
| **R-5** | SPEC-0001 §7.5:405 把“G2 退款（未发）”生命周期写成 `ALLOCATED`、履约投影写成 `PARTIALLY_SHIPPED`，与 AC-27 EARS-1 冲突 | **以 AC-27 EARS 正文为准**；§7.5:405 与 :409 注是描述性漂移，由 **PO 在下一次 SPEC 升版修正**；代码/测试跟随 AC-27，**不得迁就 §7.5**。冲突仅限“已有组发货 + G2 关闭”这一行；`0001:406-407`（全部退款未发）不冲突 | 否（文档 owner = PO） | AC-27 `0001:571-578`；§7.5 `0001:396-409`；`CONVENTIONS.md:66-68` |

**结论级别**：R-1/R-2/R-3/R-4/R-5 均可在**不改任何契约、不改已 accepted ADR Rev 2.3、无迁移**的前提下收敛；没有需要 `[ARCHITECTURAL_FLAW]` 的条目。但有一条**必须在台账显式登记的已知缺口**：多组订单的 INV-44 释放请求根本不发出（`refund_release.go:182-186`），见 §4.4，本文件不放行、不掩盖。

---

## 2. R-1：BLOCKED-1 — 运单 `CANCELLED` 的关闭判定（不新增运单投影）

### 2.1 可达时序（已核对的代码事实）

| 步 | 事实 | 代码锚点 |
|---|---|---|
| 1 | G2 `AuthorizeShipment`：`NONE→CONSUMED`，写 `shipment_no` | `settlement.go:347-357`（Ship 门禁 + CAS） |
| 2 | `shop.shipment.created.v1` 把 G2 置 `ALLOCATED` | `consumer.go:58` |
| 3 | 售后全额组退款 `AdmitRefund`：permit 已 `CONSUMED` → 写 admission `PENDING_REMOTE`，**不写 `REVOKED`** | `refund_admission.go:256-259`、`:273-280`（仅 `NONE` 才 REVOKE） |
| 4 | Tx2 调 `CancelIfCreated`：运单 `CREATED→CANCELLED`；结果收敛为 **`PRE_SHIP_REVOKED`**，permit **保持 `CONSUMED`** | `refund_admission.go:566-608`（`:576-584` 映射、`:590-597` 写回）；Fulfillment `cancel_if_created.go:23-88` |
| 5 | 退款完成：组 `refund_state=REFUNDED`；持久化组 `status` 仍 `ALLOCATED` | `aftersale_consumer.go:43-48` |
| 6 | 现读模型：`groupIsClosed` 只看 `Status`/`PermitState` → false → G2 仍计入 `remaining`，订单停在 `ALLOCATED` | `settlement.go:67-78`、`:90-125`、`consumer.go:163-203` |

**关键结论**：该分支下 “运单 `CANCELLED`” 是 **Fulfillment 侧执行事实**，`CancelIfCreated` **不发布任何事件**（`cancel_if_created.go:76-88` 仅 CAS + commit），Fulfillment 也没有 `shop.shipment.cancelled` producer（`fulfillment/service.go:35-37`、`delivery.go:54-58` 仅 created/shipped/delivered/tracked/exception/received/dispute）。Order 侧**唯一持久化**的收敛证据，就是 AC-50 明确规定的准入结果 `PRE_SHIP_REVOKED`（`0002:222`）——这正是 ADR §5.2.3 的 `CONSUMED+CANCELLED` 分支（`0007:587-589`）。

### 2.2 状态所有权裁决

- 关闭判定属于 **Order 读模型**，其输入**必须全部是 Order 自有持久事实**：S5 permit、RefundAdmission、组投影状态。
- “运单 `CANCELLED`”不进入 Order DB（`ports.go:49-54` 明确 “Order has no shipment/shipped_at storage”），因此**不得**为了让读模型“看见运单”而：
  1. 在 `GetOrder`/`ListOrders` 读路径同步调用 `Fulfillment.ListShipments`（读路径远程依赖、无锁、超时语义不可控，且会破坏读写隔离）；
  2. 新增 `shipment.cancelled` 事件消费（**producer 不存在**，需先改 event registry 契约 + Fulfillment outbox + 历史回填 + 新持久列；属跨服务契约变更，超出“不改契约/无迁移”边界）；
  3. 让 Order 直接写运单状态（违反 `0002:76`、`0007:182` 的 S6 所有权）。
- 因此裁决：**用 `PRE_SHIP_REVOKED` 作为“运单已取消/该组不会再有包裹”的 Order 侧代理事实**。它不是替代运单事实，而是 AC-50/ADR 已经规定的**准入结论**：只有“未离仓”分支才会得出 `PRE_SHIP_REVOKED`（`NONE→REVOKED`、`CONSUMED+CREATED→取消`、`CONSUMED+CANCELLED`；已离仓一律 `POST_SHIP_ALLOWED`，`0002:218-225`、`0007:573-583`）。

### 2.3 为什么**不是** `refund_state=REFUNDED`

Brief 的候选写法是“`refund_state=REFUNDED` 且组无 shipped/received 事实”。裁决**不采用 `refund_state` 作为谓词**，理由（按重要性）：

1. **语义错位**：`refund_state` 是 S9 退款进度投影，SPEC 明确它“**不是禁发裁决点**”（`0002:79`、`0002:89`）。把 S9 放进关闭谓词，等于用资金进度代理履约事实。
2. **缺少 AC-27 EARS-2 要求的那一支**：EARS-2 的条件是“`REFUNDED` **且**（许可 `REVOKED` **或** 运单 `CANCELLED`）”（`0001:574`）。`refund_state` 单独成立时，第二支只能靠组 `status` 兜底——而组 `status` 由事件投影，事件延迟/丢失时并不可靠。
3. **物理等价、投影不一致**：`NONE→REVOKED`（Tx1 即写 `PRE`）在 `REFUNDING` 窗口就关闭；`CONSUMED+CANCELLED`（Tx2 才写 `PRE`）若按 `REFUNDED` 才关闭，则同为“未离仓且已退款准入”的两种情形在 `REFUNDING` 窗口表现不同，违反单一判据（宪法 I.1）。
4. **liveness 有害**：退款永久失败（`REFUNDING` 长期停留，AC-62 人工队列、`0002:311`）时，按 `REFUNDED` 关闭会让该组永远计入 `remaining` → 订单永远 `ALLOCATED`，正是 AC-27 EARS-1 禁止的结果；而 permit 已终态、该组无论如何不可能再发货（INV-45）。
5. `refund_state=REFUNDED` 在 post-ship 场景同样成立（TCC/售后已离仓退款，`tcc_settlement.go:699-704`、`aftersale_consumer.go:43-46`），必须依赖组 `status` 先到才能被守卫挡住——把正确性建立在“事件先到”上不可接受。

> 结论：`refund_state` 只用于**展示退款进度**（`toProto` `repo.go:121` 已如此），**不参与**关闭判定。

### 2.4 关闭谓词（语义定义，非代码）

`groupIsClosed(g)` 的判定**顺序固定**（顺序即安全边界）：

```text
1. g == nil                                   -> false
2. g.Status == CANCELLED                      -> true
3. g.Status ∈ {SHIPPED, RECEIVED}             -> false        // 离仓守卫必须在此，先于任何退款事实
4. g.PermitState == REVOKED                   -> true         // S5 终态（INV-45）
5. preShipRefundAdmitted(g) == true           -> true         // 本裁决新增的那一支
6. 其他                                        -> false

preShipRefundAdmitted(g) := 存在 order_refund_admissions 行，
    order_no = g.order_no 且 group_no = g.group_no 且 result = 'PRE_SHIP_REVOKED'
```

- 第 4 支**保留**（不是冗余）：`order_shipping_permits` 是 S5 自有终态；历史/既有数据与用例存在 `REVOKED` 但无 admission 行（`warehouse_quote_test.go:125-131` 用 fixture 表达），移除会造成回归。
- 第 5 支是**补全**，不是“再加一个 if”：Order 侧只有两类“未离仓且不可再发货”的终态事实——许可 `REVOKED` 与准入 `PRE_SHIP_REVOKED`（`CONSUMED+CANCELLED` 收敛结果）。两者是同一集合的两个来源，而不是场景特判。
- `PENDING_REMOTE` **不构成**关闭事实（崩溃窗口 W2/W7，ADR `0007:951,961`），必须等 Tx2 收敛后重聚合；期间保持“未关闭”是 fail-closed。
- `POST_SHIP_ALLOWED` **永不**被当作关闭依据。

### 2.5 最小实现范围（文件级，不写代码）

| 文件 | 需要的改动（语义，不写代码） |
|---|---|
| `internal/services/order/fulfillment_groups.go` | 在组读模型加载时**同时**取该订单每组的 `order_refund_admissions.result=PRE_SHIP_REVOKED` 存在性（可 `LEFT JOIN`/子查询，键 `(order_no, group_no)`）。**无 schema 变更**：`migrations/order/0016_settlement_admission.sql:7-19` 已有 `idx_order_refund_admissions_order_group (order_no, group_no)`。允许在 `fulfillmentGroup` 增加一个**非持久化派生字段**（如 `PreShipAdmission bool`）或等价本地 map；两者都不是新增持久化字段/契约字段 |
| `internal/services/order/settlement.go` | 按 §2.4 扩展 `groupIsClosed`；保持它仍是**唯一**关闭判定入口，`effectiveGroupStatus`/`shipmentStatusForGroups`/`aggregateOrderStatus` 不改调用方式、不再各自判断退款态 |
| `internal/services/order/*_test.go` | 见 §10 测试矩阵 |

**明确不做**：不新增 `shipment` 表/列/投影、不消费 `shipment.cancelled`、不加读路径远程 RPC、不改 `order_shipping_permits` 状态机、不把 `CONSUMED` 改 `REVOKED`（ADR `0007:589`）。

### 2.6 幂等 / 并发 / 崩溃窗口

| 维度 | 分析 | 保障 |
|---|---|---|
| 幂等 | 新增的是**纯读派生**（每次读重算），无写、无副作用；`PRE` 行按 `refund_id` 唯一（`0016:17`） | 重复读/重复事件结果相同 |
| 并发（读 vs Tx2） | `GetOrder` 无锁读，可能读到 Tx2 提交前的旧状态 → 暂时仍投影“未关闭”；Tx2 提交后下一次读收敛 | 只可能**滞后**，不可能**提前关闭**（`PRE` 只在 AC-50 未离仓分支写入）；不参与任何写入裁决 |
| 并发（Ship vs 退款准入） | Ship 先胜 → 走 `CONSUMED` 两阶段（`0002:221`）；准入先胜 → `REVOKED`，Ship 被拒（`settlement.go:347-352`、AC-48 `0002:203-204`） | 赢家唯一；关闭判定只在终态之后成立 |
| 崩溃（W2/W7） | `CancelIfCreated` 成功但 Tx2 未提交 → admission 停留在 `PENDING_REMOTE` | 读模型**不**提前关闭；既有 worker 按 `refund_id` 幂等重试并写回 `PRE`（ADR `0007:591-596,951,961`），无需新 worker |
| 已退款后再发货 | permit 终态不可恢复（INV-45 `0002:129`、AC-62 `0002:310`）；Ship 需 `NONE` 且不同 `shipment_no` 一律拒绝（AC-48 `0002:203-204`） | `PRE` 一旦成立，组**物理上**不会再产生包裹；关闭判定不会与实际发货冲突 |

---

## 3. R-2：关闭之后订单生命周期必须能收敛（EARS-1 的必要条件）

### 3.1 只改谓词不够（可复现缺口）

`orders.status` 的 `ALLOCATED→SHIPPED→COMPLETED` 推进**只发生在运单事件消费路径**（`consumer.go:58-62,107-143`；PAID/CANCELLED 由结算/取消路径写入，与本裁决无关）。若 G1 的 `shipped` 事件**先于** G2 关闭被消费（BLOCKED-1 的真实时序：G2 先授权、后取消），当时的聚合结果仍是 `remaining=2, shipped=1` → 不推进；等 G2 退款完成、组关闭后，**没有任何事件**再触发聚合。于是：

- `toProto`/`GetOrder` 的 `shipment_status` 会是 `SHIPPED`（`settlement.go:90-125`），但持久化 `orders.status` 永远停在 `ALLOCATED`（`selfcheck:71` 已实测）；
- AC-27 EARS-1 要求“按 G1 的履约进度**推进订单投影**；G2 不得使订单停留在 `ALLOCATED`”（`0001:572-573`），`SHIPPED` 是 EARS-2 允许的生命周期值（`0001:575`）；
- 现状既有集成测试把 `orders.status=ALLOCATED` 写成了期望（`ac27_projection_integration_test.go:86-92`）——它证明的是“退款进度不写生命周期”（INV-01，正确），但**不能**被当作“订单可以永远停在 ALLOCATED”的契约依据。

### 3.2 裁决

**R-2**：在既有“退款完成投影”consumer（`aftersale_consumer.go:39-56`）里，把“组退款态投影 + 缺货投影 + **订单生命周期重聚合** + outbox”合并到**同一个事务**；重聚合复用 `aggregateOrderStatus`（`consumer.go:163-203`）与 `updateOrderStatus` 的 CAS（`repo.go:282-290`），并把现有 shipment 消费路径里的“状态迁移 + `insertStatusLog` + outbox `order.shipped/completed/allocated`”（`consumer.go:107-143`）抽成**唯一 authority** 供两处复用。

约束：

1. **唯一 authority**：不得在 `applyAfterSaleCompleted` 里另写一份 `if 全发 then SHIPPED`；必须复用同一函数（宪法 I.1：同一业务路径不得出现第二处状态判断分支）。
2. **不写退款进度进生命周期**：推进的唯一理由是“关闭事实 + 其余组已发/已收”，`refund_state` 只决定“何时值得重算”（完成投影），**不得**被翻译成生命周期值（INV-01 `baseline:199`、AC-25 `0001:563-565`）。
3. **outbox 同事务**：状态写与 `outbox.Append` 必须共用同一 `*sql.Tx`（宪法 III.1），复用既有事件，payload 仍满足 registry 必填 `order_no,status`（`internal/platform/events/registry.go:314`）；`group_no` 填“使聚合完成的那个组”，**不新增 subject、不改 payload 语义**。
4. **终态 no-op**：`orders.status ∈ {CANCELLED, COMPLETED, PENDING_PAYMENT}` 或目标为空 → 成功 no-op（不得报错、不得重试风暴）。`PENDING_PAYMENT` 只可能出现在取消/补偿路径，不因退款完成推进。
5. **不可达目标**：若目标状态与当前状态不满足既有 `from` 集合（例如事件缺失导致跳过 `SHIPPED` 直接想 `COMPLETED`），新的退款投影路径只**告警 + no-op**（fail-closed，不伪造迁移），并且**不得改变**现有 shipment 消费路径的错误语义（`consumer.go:118-120` 保持原样）。
6. **幂等**：`refund_state` 已是 `REFUNDED` 时 UPDATE 为 no-op；CAS 0 行 → 读回并仅在有进度时重试（复用 `consumer.go:79-160` 的重读模式），不得重复发 outbox。
7. **不做**：不新增定时重算 worker、不新增事件、不在读路径推进状态。

### 3.3 崩溃/并发窗口

| 窗口 | 结果 | 恢复 |
|---|---|---|
| 组退款态已写、订单未推进时崩溃 | 单事务回滚 → 事件未 ack | durable consumer 重投（`MaxDeliver:10`，`aftersale_consumer.go:26`），重跑幂等 |
| 与 shipment 事件并发 | 两方都经 `updateOrderStatus` 的 `status=from` CAS，只有一个成功；败者重读后收敛 | 现有 rank/`from` 守卫 + 重读循环；`rankStatus` 防回退（`consumer.go:205-219`） |
| 推进成功、outbox 未追加 | 不可能：同一事务 | — |
| consumer 永久失败 | 组退款态回滚为 `REFUNDING`（不出现“已退款但订单态假”），进入 DLQ/告警 | 既有 DLQ + 人工重放；**不**触碰资金事实 |

> **范围纪律**：若 Lead 决定把 R-2 拆成独立 `change_id`，则 R-1 交付**不得**声称 AC-27 EARS-1 收敛，台账必须为“订单生命周期收敛”登记显式延期行（owner/expiry/reason/thread）；不得默认放行。

---

## 4. R-3：BLOCKED-2 — reservation released 不要求 Order 可观察，且禁止 Order 写 `RELEASED`

### 4.1 事实

- `reservation_state` 的**全部**写点：插入 `HELD`（`fulfillment_groups.go:58-62`）、整单取消时随 `status=CANCELLED` 写 `RELEASED`（`fulfillment_groups.go:71-80`）。该列是 phase-1 shadow，“**deliberately does not participate in any guard yet**”（`fulfillment_groups.go:69-71`）。
- 退款准入的释放是 **outbox 请求**（`refund_admission.go:284-288,598-602`），执行 owner 是 Inventory（INV-44 `0002:128`、ADR §5.7 `0007:706-724`）；Order 不写、也无法知道 Inventory 是否已执行。
- 更严重的事实：**多组订单当前根本不发释放请求**——`appendRefundReleaseTx` 在 `groupCount != 1` 时直接 `return nil`，只告警 “multi-group refund keeps inventory reservations (P1/manual)”（`refund_release.go:178-186`）。
- `groupIsClosed` 不读 `reservation_state`（`settlement.go:67-78`），读模型也不消费它。

### 4.2 裁决

1. **AC-27 EARS-3 不要求 Order 可观察“预留已释放”**。原文是“WHEN G2 的预留已释放**或**运单已取消，THE SYSTEM SHALL 按 §6.6/§6.6.1 **判定该组合合法**，不构成 §6.6 的禁止组合”（`0001:577-578`）。这是**规范层面的合法性判定**：§6.6:287 与 §6.6.1:289-295 已在 SPEC-0001 v1.3 把“已支付不得预留已释放”收窄为“**仍计划履约的组**”，并明确组退款导致的释放是合法结果。该判定**不需要任何运行时组件再“判断”一次**，更不需要 Order 读模型持有 Inventory 的事实。
2. **禁止** Order 在退款路径把 `reservation_state` 写成 `RELEASED`：
   - 所有权：预留状态 owner 是 Inventory（S4 `0002:74`、宪法 II.2）；Order 写 `RELEASED` 等于凭“已发出释放请求”伪造“释放已执行”；
   - 失败可见性：释放失败必须重试并告警（INV-44）；Order 提前置 `RELEASED` 会掩盖失败；
   - 无消费者：该 shadow 列不参与任何 guard，写入只增加一个会漂移的第二事实源。
3. **防止“释放后仍可发货”的实际闸门不在 Order 读模型**：新发货唯一裁决点是 S5 permit（`0002:89`）；未离仓退款准入后 permit 为 `REVOKED` 或 `CONSUMED+CANCELLED`，AC-48 拒绝新运单（`0002:203-205`），Ship 由 Fulfillment 调用 `Order.AuthorizeShipment`（`fulfillment/service.go:244`）并被 `settlement.go:347-352` 拒绝。因此安全不依赖 Order 是否能观察 `RELEASED`。

### 4.3 证据归属（供 Lead/Reviewer 台账使用，不由本文件改台账）

- AC-27 EARS-3 的**释放腿**证据必须来自 **Inventory 侧**（释放请求事件 `events.RefundReleaseRequested` + Inventory 的 `Release`/`CancelConfirm` 执行与 `stock_ledger`），或来自 §6.6.1 的规范判定；**不得**用 Order `_test.go` 里“fixture 塞 `RELEASED`”当作该分支的验证（那是伪断言，`selfcheck:47,84` 已诚实拒绝）。
- 若 Reviewer 需要一条 Order 侧断言，**只允许**断言语义边界：“`reservation_state` 不是关闭判据、单独置 `RELEASED` 不改变投影”；不得断言它驱动关闭。

### 4.4 已知缺口（不在本裁决放行范围）

多组订单（AC-27 的前提是 2 组）当前**不会**发出组级释放请求（`refund_release.go:182-186`），与 §6.6.1/INV-44“未离仓组的预留由 Inventory 释放”存在差距。这是既有的 **P1/manual** 已知缺口（T1.4b2 设计按人类 2026-11-25 简化裁决只做 P0 单组释放；见 `docs/adr/0007-amd1-addendum-t14b2-inv44-group.md:5-7`）：

- 本 Addendum **不**修复它（修复需要跨服务释放契约/组级释放范围或迁移，超出“不改契约、无迁移”边界）；
- 台账**不得**把“2 组订单的 G2 预留已释放”标为 AC-VERIFIED；应登记为延期/已知缺口（owner、expiry、reason），或由 PO/Lead 另立 `change_id` 走组级释放设计；
- 若有人主张“由 Order 回写 `RELEASED` 来满足 EARS-3”，**否决**：这既伪造 Inventory 事实，又不产生真正的库存释放，只是让断言变绿（违反宪法 I.4、II.2）。

---

## 5. R-4：BLOCKED-3 — pre-ship `REFUNDING` 投影 `CANCELLED` 是正确的

### 5.1 裁决

**确认正确，不改**：`ALLOCATED + refund_state=REFUNDING + permit=REVOKED` 的未发组，当前经 `groupIsClosed`（`settlement.go:67-78`）投影为 `CANCELLED`，并允许订单按其余组进度推进。这是“**关闭但退款在途**”：组不会再产生包裹，退款进度仍由 `refund_state` 独立表达。

### 5.2 依据

1. **S5 是唯一禁发裁决点**（`0002:89`）；`REVOKED` 是终态、不可恢复（INV-45 `0002:129`；AC-62 `0002:310`“不得把 `REVOKED` 改回 `CONSUMED`，不得为同一组恢复可发货”）。一个永不可发货的组，在生命周期投影里必须算“已关闭”，否则 `remaining` 永远包含它。
2. **S9 明言不是禁发裁决点**（`0002:79`）；用 `refund_state` 反向要求“未 `REFUNDED` 就不得关闭”等于让资金进度决定履约投影，与 S9 冲突。
3. **AC-27 正文没有“退款未完成不得投影 `CANCELLED`”**：EARS-2 只约束“保持生命周期为 `PAID/ALLOCATED/SHIPPED` 之一、退款进度只写 `refund_state`、不得写 `orders.status`”（`0001:574-576`）；EARS-1 的“已退款完成”是**推进的充分条件**，不是“禁止更早收敛”的必要条件。
4. **liveness 反证**：若要求 `REFUNDED` 才关闭，则退款永久失败（`REFUNDING` + 人工队列，AC-62 `0002:311`）时该组永久计入 `remaining`，订单永久 `ALLOCATED`，且 UI 出现“`ALLOCATED` 但无 `ship` action”的死组——正是 AC-27 EARS-1 要避免的。
5. **post-ship 不被误伤**：离仓守卫（`settlement.go:74-76`）在关闭判定之前；已 `SHIPPED/RECEIVED` 的组即使 `REFUNDING/REFUNDED` 也**不**关闭（既有反向测试 `ac27_projection_negative_test.go:9,42`）。

### 5.3 Lead 原 Brief 句子不成立，已撤回

- Lead 原 Brief 的“退款未完成不得投影 `CANCELLED`”是**越权写入的业务语义**，AC-27 正文没有该要求；Lead 已在 `docs/lead/PAY-SETTLE-01/tasks.md:47` 正式更正并撤回该句，将“pre-ship `REFUNDING` 是否算关闭”交本次裁决。
- 因此 Developer 拒绝实现该反向要求（`selfcheck:88-94`）是**正确**的；不得据此判 Developer 未交付。

### 5.4 需要补的（仅测试，无生产改动）

新增一条**回归断言**：`ALLOCATED + REFUNDING + REVOKED`（pre-ship）→ `groupIsClosed=true`、`effectiveGroupStatus=CANCELLED`、组不阻塞其余组推进；并显式注明“关闭 ≠ 退款完成，`refund_state` 仍为 `REFUNDING`”。见 §10。

### 5.5 明确不做

- 不把 `groupIsClosed` 改成“必须 `REFUNDED` 才关闭”（会引入死组、且与 R-1 的 `PRE` 判据冲突）；
- 不在 `effectiveGroupStatus` 里为 `REFUNDING` 造新标签（组状态枚举只有 7.2 的 5 个值，`0001:373-379`）；
- 不改 `refund_state` 语义、不改 AC-64 的“组保持 `REFUNDING`”（`0002:359`）。

---

## 6. R-5：SPEC-0001 §7.5:405 描述不一致 — 以 AC-27 EARS 为准，PO 修描述

### 6.1 冲突事实

| 位置 | 文本（摘要） | 与 AC-27 正文 |
|---|---|---|
| `0001:405` | `G2 退款（未发） \| ALLOCATED \| PARTIALLY_SHIPPED \| G1 SHIPPED / G2 CANCELLED \| G2 REFUNDED` | **冲突**：G2 关闭 + G1 已发 ⇒ 聚合应为 `SHIPPED`（`consumer.go:163-203`），生命周期不应停留 `ALLOCATED`（`0001:572-573`） |
| `0001:409`（注） | “订单生命周期仍为 ALLOCATED/PAID，G2 的退款态由 `refund_state` 表达” | **冲突**（同上）；且把“退款态表达”与“生命周期不推进”混为一谈 |
| `0001:406-407` | 全部退款（未发）：组 `CANCELLED×2`、生命周期 `ALLOCATED`、对外“已关闭（全部退款）” | **不冲突**：没有任何已发货组可推进，订单停在 `ALLOCATED`、`shipment_status=UNSHIPPED` 是正确收敛（`selfcheck:48` 已测） |
| `0001:571-578`（AC-27） | EARS 正文：推进、只写 `refund_state`、组合合法 | **验收依据** |

### 6.2 裁决

1. **AC-27 EARS 正文是唯一验收文本**；§7.5 的示例行是派生投影表，`0001:447` 已声明 AC 是可判定条款。
2. **代码/测试不得迁就 §7.5**：不得为了让 `0001:405` 成立而保留 `ALLOCATED`、也不得把 `shipment_status` 强行改回 `PARTIALLY_SHIPPED`。
3. **§7.5 的修正归 Product Owner**，在 SPEC-0001 下一次升版完成（建议一并修正 `0001:367` 的“全部发货”口径），**不属本 Addendum 范围、不由 Architect 改 SPEC**。
4. 在 PO 升版之前，Reviewer/QC **不得**以 `0001:405`/`:409` 作为阻断依据；应登记为 PO 文档漂移项（`CONVENTIONS.md:66-68`、宪法 Amendment 5 的“文档漂移修正”）。
5. **语义澄清（供 PO 采纳）**：§7.1 的“全部发货 → `SHIPPED`”应按 AC-27 解读为“**所有仍会产生包裹的组均已发货**”；已关闭（退款完成/许可终态）的组不计入。这是 AC-27 的必要推论，不是新业务规则。

### 6.3 建议 PO 修正清单（仅建议，作者不改）

- `0001:367`：`--全部发货-->` 后补“（不含已关闭组；见 AC-27）”
- `0001:405`：生命周期 `ALLOCATED`→`SHIPPED`；履约投影 `PARTIALLY_SHIPPED`→`SHIPPED`
- `0001:409`：删除“仍为 ALLOCATED/PAID”，改为“按其余组进度推进；退款进度只由 `refund_state` 表达”
- 变更记录登记为“描述性修正，不改 AC 语义”，走 PO 升版流程

---

## 7. 反模式自省（强制）

| 反模式 | 本裁决如何防止 |
|---|---|
| 用 `if-else` 堆投影补丁 | 关闭判定保持**单一函数**（§2.4 固定顺序）；R-2 要求把生命周期迁移抽成**单一 authority**；禁止在各 call site 各自判断 `refund_state`/`reservation_state`/运单 |
| 用第二事实源替代所有权 | `reservation_state` 仍归 Inventory；运单仍归 Fulfillment；Order 只用自有 S5/Admission；`refund_state` 只展示 |
| 读路径远程调用/脏读投影 | 禁止 `GetOrder` 调 `ListShipments`；ORDER 读模型只读本库 |
| 读模型推进掩盖资金问题 | 重聚合只依赖履约终态事实；退款失败不阻塞、也不伪造“已退”；outbox 与状态写同事务 |
| 用“fixture 塞状态”让断言变绿 | 明确禁止对 `reservation_state` 写驱动性断言（§4.3）；BLOCKED-1 测试必须经**生产写入函数/真实行回读**（§10） |
| 用描述性文档反向压实现 | §7.5 冲突以 AC-27 为准，且由 PO 改文档，不由代码迁就（§6） |
| 无 owner 的“暂时跳过” | 多组释放缺口必须登记延期行，不得静默（§4.4） |

---

## 8. 【假设与不确定性】（强制）

1. **Tx2 收敛假设**：`CONSUMED+CANCELLED` 的 `PRE_SHIP_REVOKED` 最终一定会被 Tx2 写回（ADR `0007:591-596`，W2/W7）。若 worker 永久卡住，则组不会被判关闭、订单可能停在 `ALLOCATED`；这是**既有**恢复缺口，本裁决不新增 recovery owner，也不改 worker。`[不确定]`：无真实故障注入证据证明 Tx2 在 DLQ 耗尽后仍可人工重放（依赖既有 `refund_id` 重放入口，AC-61 `0002:303`）。
2. **事件交付假设**：R-2 依赖 `shop.aftersale.completed.v1` 的 durable 交付（`aftersale_consumer.go:21-37`，`MaxDeliver:10`）。若该事件永久丢失，订单生命周期可能滞后；本裁决**不**新增对账/重算 worker，接受“滞后但可由后续运单事件收敛”，并在台账/发布证据中说明。
3. **顺序假设**：AC-50/INV-43 保证“先准入后打款”，因此 `aftersale.completed` 到达时 `PRE` 应已持久化；若出现 `REFUNDED` 早于 `PRE` 的异常顺序（数据/实现缺陷），读模型会保持“未关闭”（fail-safe），由告警/人工发现。
4. **数据一致性假设**：不存在“同一组既有 `PRE` 又已 `SHIPPED/RECEIVED`”的数据；若出现，离仓守卫优先（不关闭、不自动修复），需人工核对。`[不确定]`：未做历史数据审计。
5. **历史行假设**：存在 `permit=REVOKED` 但无 admission 行的历史/测试数据；保留第 4 支判定以兼容。`[不确定]`：生产库中该组合的规模未统计。
6. **`orders.status=SHIPPED` 的口径**：本裁决把它解释为“所有仍会产生包裹的组均已发货”（AC-27 的必要推论）。若 PO 认为 §7.1 的“全部发货”必须是字面全部组，则 AC-27 EARS-1 与 §7.1 存在**业务层**冲突，需 PO 升版裁决——届时本 Addendum 的 R-2 需暂停而不是由实现自行选择。`[不确定]`：属业务解释，非架构可单方决定。
7. **多组释放缺口**（§4.4）：`[不确定]` 的是“何时/是否由 P1 补上”；在补上之前，AC-27 EARS-3 的释放腿在 2 组订单上**没有**产品事实可验证，只能依赖规范判定与单组证据。
8. **性能**：读模型新增一次按 `(order_no, group_no)` 的索引查找（`0016:18`）；未做压测，假设订单组数在 P0 上限（2）内、`GetOrder`/`ListOrders` 的 N+1 风险可接受（`ListOrders` 若批量加载需一次 `IN` 查询，禁止逐单查询）。`[不确定]`：`ListOrders` 当前是否加载 groups 需 Developer 核对，若加载则必须批量取 admission 事实。

---

## 9. Baseline 影响 / INV / AC 映射

| 项 | 影响 | 依据 / 回归要求 |
|---|---|---|
| INV-01（生命周期与退款态正交，`baseline:199`、AC-25 `0001:563-565`） | **必须保持**：重聚合只由关闭+已发/已收驱动，`refund_state` 不写 `orders.status` | AC-25 既有测试 `settlement_integration_test.go:355` 必须继续 PASS |
| INV-06（已支付不得留 `HELD`；§6.6:287 已按 v1.3 收窄为“仍计划履约的组”） | **保持**：本裁决不触碰预留释放/回库；关闭判定不等于释放 | 释放证据仍归 Inventory（§4） |
| INV-24（额度上限） | **不受影响**：不新增退款/额度路径 | 无需新证据；既有 AC-50/AC-64 回归 |
| INV-38（组级隔离） | **保持**：关闭只影响该组的 `remaining` 计数，不锁死整单 | AC-27 两组场景 |
| INV-44（释放有主） | **不改变**：Order 仍只发 outbox、Inventory 执行；本裁决**禁止** Order 写 `RELEASED` | 证据归 Inventory（§4.3）；多组缺口见 §4.4 |
| INV-45（准入不可逆/承诺释放纪律） | **强化遵守**：关闭判据使用 permit 终态与 `PRE` 准入，不恢复 permit、不释放承诺 | R-1/R-4 的测试必须证明“`REVOKED` 不被改写” |
| INV-50（零元售后闭环） | **不改变**：零元退款同样写 `PRE/POST` admission（`refund_admission.go:490-511`）；关闭判定对零元组同样成立（正确：零元未发组退款后组关闭） | aftersales 零元用例回归；不新增 Payment 调用 |
| S5/S6/S9 所有权（`0002:74-79`） | **不改变**：不新增运单投影、不写预留、不把 S9 当禁发点 | Reviewer 逐条核对 |
| AC-48（发货准入） | **不改变**：permit 门禁原样；`CONSUMED+CANCELLED` 由 AC-48 拒绝新运单 | 既有 permit/refund 竞态测试 |
| AC-50 / AC-62 / AC-65 | **不改变**：准入/释放/TCC 编排零改动 | 既有 T1.7 / TCC 用例 |
| 受影响 AC（本裁决要求） | AC-27 EARS-1/EARS-2/EARS-3；回归 AC-25；观察 AC-26 | 证据矩阵由 Reviewer 维护，本文件不标 AC-VERIFIED |

**跨服务非回归**：本裁决实现只改 `internal/services/order/`，但读模型解释跨服务事实，故发布证据必须包含受影响基线（permit/refund 竞态、aftersale 投影、TCC 准入、零元）非回归，见 §10。

---

## 10. 实施边界与验收要求（R0.3-R1c 施工图，不写代码）

### 10.1 测试矩阵（最小强制集；均绑定精确 HEAD，日志入库由 Developer 提供）

| # | 用例（语义） | 层级 | 断言要点 | 对应裁决 |
|---|---|---|---|---|
| T1 | `NONE→REVOKED`（既有）+ `CONSUMED` + `CancelIfCreated=CANCELLED` → `PRE_SHIP_REVOKED`，组 `ALLOCATED`+`REFUNDED` → **closed**；G1 `SHIPPED` → `shipment_status=SHIPPED`、生命周期推进 | 真实 MySQL + 生产写入函数（`AdmitRefund` + `fakeFulfillmentPort{status:CANCELLED}`，`refund_admission_integration_test.go:231-245`） | G2 投影 `CANCELLED`；`orders.status` 由聚合推进为 `SHIPPED`，且**不是**因 `refund_state` 直接写入 | R-1/R-2 |
| T2 | `CONSUMED` + `refund_state=REFUNDED` 但**无** `PRE` admission（模拟 post-ship 退款） | 单元 | 组 **不** closed（证明不以 `refund_state` 为判据） | R-1 |
| T3 | admission 停留 `PENDING_REMOTE` | 单元/真实行 | 组 **不** closed（崩溃窗口不提前关闭） | R-1 |
| T4 | `ALLOCATED + REFUNDING + REVOKED`（pre-ship） | 单元 | closed=true、`effectiveGroupStatus=CANCELLED`、不阻塞 G1 推进；`refund_state` 仍 `REFUNDING` | R-4 |
| T5 | `SHIPPED + REFUNDED`、`RECEIVED + REFUNDED`（post-ship） | 既有 | 仍 open/仍计 shipped；不得回退（`ac27_projection_negative_test.go:9,42` 保持） | R-4 |
| T6 | 退款完成投影触发重聚合：G1 已发 + G2 先关闭后 `aftersale.completed` | 真实 MySQL | 生命周期收敛 `ALLOCATED→SHIPPED`；重复事件/并发不重复 outbox、不回退 | R-2 |
| T7 | 终态 no-op：`CANCELLED`/`COMPLETED` 订单收到 `aftersale.completed` | 真实 MySQL | 无状态迁移、无 outbox、返回成功 | R-2 |
| T8 | `reservation_state` 边界 | 单元 | 单独 `RELEASED` **不**改变关闭判定；测试注释说明该分支证据归 Inventory | R-3 |
| T9 | 全部退款未发（2 组均 closed） | 既有 | 生命周期停 `ALLOCATED`、`shipment_status=UNSHIPPED`（§7.5:406 一致性） | R-5 |

补充要求：

- 所有涉及资金/准入的用例必须至少一个**异常分支**（`CancelIfCreated` 超时/`TRANSIENT_ERROR`、`ALREADY_SHIPPED` 收敛 POST），符合宪法 IV.2/IV.3；
- 并发证据：`-race` + 至少一条“shipment 事件 vs 退款完成事件并发”的竞态用例；不适用必须书面说明并经人类确认；
- 既有被修正的期望（`ac27_projection_integration_test.go:86-92` 的 `orders.status=ALLOCATED`）必须改为“生命周期只能由聚合推进；断言 `SHIPPED` 或 `∈{ALLOCATED,SHIPPED}` 且**永不是** `REFUNDED`”，并附说明，防止被误读为放宽断言。

### 10.2 停止条件（stop conditions）

1. QC 对本 Addendum 未出结论 / 人类未批准 → **不得**动生产代码。
2. 实现需要新增持久化列/迁移/事件/subject/RPC → 停止，回到 Architect/PO（超本裁决边界）。
3. 发现 `PRE` 与 `POST` 语义无法覆盖的第三种收敛（如新的准入结果）→ 停止，不得用默认分支兜底。
4. 发现多组释放缺口必须与 AC-27 同批解决 → 停止，交 Lead/PO 拆 `change_id`。
5. 任何测试必须靠改生产语义（例如把 `REVOKED` 恢复、把 `refund_state` 写进 `orders.status`）才能变绿 → 停止并标记 `[ARCHITECTURAL_FLAW]` 回 Architect。

---

## 11. Consequences 与后续 owner

| owner | 后续动作 |
|---|---|
| **QC** | 对本文件做红队审查（异常分支、幂等、状态死胡同、越权）；DRAFT 期间不得进入实现 |
| **Product Owner** | 下一次 SPEC-0001 升版修正 §7.1“全部发货”口径与 §7.5:405/:409（§6.3 建议）；**不改 AC-27 语义** |
| **Developer（R0.3-R1c）** | 按 §2.5/§3.2/§10 实现最小修复 + 测试；精确 HEAD；不自行改状态所有权；重大妥协高亮 |
| **Reviewer** | 在本裁决范围内核对 AC-27 EARS-1/2/3 证据；台账登记 §4.4 延期行；确认 Tier L2；不得以 §7.5 阻断 |
| **Lead** | 决定 R-2 是否与 R-1 同批；更新 Plan/Tasks 与 evidence-matrix 的 EARS-3 证据归属 |

**本文件不声称**：Release PASS、AC-VERIFIED、merge、Reviewer PASS。DRAFT 的效力仅限“供 QC 复核与人类批准的技术裁决草案”。

---

## 12. 变更记录

| Revision | 日期 | 变更 | 影响 |
|---|---|---|---|
| **0.1** | 2026-11-25 | 初稿：R-1（BLOCKED-1：以 Order 自有 `PRE_SHIP_REVOKED` 准入事实判关闭，否决运单投影/`shipment.cancelled` 消费）、R-2（关闭后生命周期重聚合）、R-3（BLOCKED-2：不要求 Order 观察释放，禁止写 `RELEASED`，多组释放缺口显式登记）、R-4（BLOCKED-3：pre-ship `REFUNDING` 投影 `CANCELLED` 正确，Lead Brief 句撤回）、R-5（§7.5:405 以 AC-27 为准，PO 修描述） | 状态 **DRAFT — pending QC**；不改 SPEC、不改 ADR Rev 2.3、无迁移、无代码 |

---

## 13. 与已批准契约的关系（声明）

1. **不修改** ADR-0007 Rev 2.3 正文、Status（accepted）或任何已 accepted Addendum；本文件是其**增量解释**，冲突时以 Rev 2.3 与 SPEC 正文为准。
2. **不修改** SPEC-0001 / SPEC-0002 任何内容（含 AC、INV、§7.5）；对 SPEC 的建议仅出现在 §6.3，且由 PO 执行。
3. **不新增**持久化字段/列/表/索引、事件 subject/payload 字段、RPC、worker、迁移。
4. **不改变**事实所有权：S4→Inventory、S5/S9/RefundAdmission→Order、S6→Fulfillment。
5. **不改变** permit 状态机与 INV-45 的不可逆性；不做任何“恢复许可/重开退款”的动作。
6. 若 QC 认为 R-1/R-2 超出了“解释既有 AC-27”的范围而构成新业务规则，则必须回到 PO 升版 SPEC（本文件不默认放行）。

---

**Status: DRAFT — pending QC** ｜ Revision 0.1 ｜ Change ID `PAY-SETTLE-01` ｜ Tier L2 ｜ Actor `arch-ac27` ｜ Reviewed-by: 待 QC
