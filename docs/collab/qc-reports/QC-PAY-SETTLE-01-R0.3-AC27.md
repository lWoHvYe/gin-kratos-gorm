# QC-PAY-SETTLE-01-R0.3-AC27 — R0.3-R1b AC-27 读模型裁决红队审查

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| 任务 | R0.3-R1b QC 复核（Lead 指派；对应 `docs/lead/PAY-SETTLE-01/tasks.md:38`） |
| 审查对象 | `docs/adr/0007-amd1-addendum-r03-ac27-readmodel.md`（Status DRAFT；Revision 0.1；385 行） |
| 结论 | **BLOCKED**（2 个 `[BLOCKER]` + 9 个 `[MAJOR]` + TODO；DRAFT 不得进入实现） |
| 日期 | **2026-11-25** |
| Actor | `qc-r04`（Quality Checker；与 Architect `arch-ac27`、Developer、Reviewer 不同 actor） |
| base | `362b9e21f029976978626952e91f59f3e98532e1` |
| head | `e3998d3ccfc17268a60a46e5928ec496353dfb25`（`git rev-parse HEAD`；`362b9e2..e3998d3` 仅改 R0.4 Addendum，被审 AC-27 ADR 与 order/fulfillment/aftersales 代码均未变） |
| Tier | **L2**（Order 生命周期投影 / permit 门禁 / 既有事件消费效应；确认 ADR 的 Tier-Rationale） |
| review_scope | 只审 R-1…R-5 的裁决、与 SPEC-0001 v1.4 AC-25/26/27/§6.6/§6.6.1/§7.5、SPEC-0002 v1.10 S4/S5/S6/S9/INV-44/45/50/AC-50/62/65、ADR-0007 Rev 2.3 §1.1/§5.2/§5.7 的一致性，以及 ADR 引用的代码事实（`internal/services/order/**`、`internal/services/fulfillment/{cancel_if_created.go,service.go,consumer.go}`、`internal/services/aftersales/workers.go`）。不审 R0.4 Addendum（另见 `QC-PAY-SETTLE-01-R0.4-ADDENDUM.md`）。 |
| release_status | **NOT RELEASE**；不批准 merge；不声称 AC-VERIFIED / 测试已跑 |

## 0. 结论

**BLOCKED。** 两个阻断项都属“裁决越过了既有契约正文”：

1. **B-1（R-2）**：R-2 要求在 `aftersale.completed` consumer 内把 `orders.status` 从 `ALLOCATED` 推进到 `SHIPPED`（ADR:156-166），但 SPEC-0001 **§6.6.1 规范正文**写的是“组退款不改变订单生命周期；退款进度只写 `refund_state`，**不得写回 `orders.status`**”（`0001:289-294`）。ADR §6.1 只分析了 §7.5 示例表（ADR:249-254），漏掉这条规范冲突；“AC-27 EARS-1 的‘推进订单投影’是否等于持久化 `orders.status`”是**业务解释**，不是 Architect 可单方裁决的解释。按 ADR 自己的 §13.6，必须回 PO 升版/解释。
2. **B-2（R-1）**：R-1 把 `order_refund_admissions.result='PRE_SHIP_REVOKED'` 当作“运单已取消”的 Order 侧代理事实（ADR:55、:113-114），但 AC-50 明确 `许可 CONSUMED 且无运单` 必须返回 `REFUND_ADMISSION_CONFLICT` 并告警（`0002:223`）；而 `CancelIfCreated` 对“无运单行”返回 `CANCELLED`（`cancel_if_created.go:50-55`），`finishRemoteAdmission` 把 `CANCELLED` 一律映射为 `PRE`（`refund_admission.go:576-584`）并写回 `result=PRE`（`:590-597`）。ADR:87 枚举 PRE 来源时**漏掉了这个 AC-50 定义为异常的分支**；R-1 会把异常状态静默正规化为“组已关闭”。要么按 AC-50 修正映射（跨服务，超 allowed_paths），要么由 PO/Architect 显式裁决该偏差并留告警，否则 R-1 的关闭判据不成立。

其余 `[MAJOR]`：R-2 的“单一 authority”边界未定义会写错组状态/审计、R-1 的 `LEFT JOIN` 会产生重复组、恢复 owner 误写为“Order worker”（实际是 AfterSales durable task，Order 无扫描 worker）、R-1 关闭与多组不释放叠加使 HELD 预留静默滞留、R-4 把 `REFUNDING` 关闭说成“正确”缺少 PO 授权（SPEC v1.4 记录明确“只覆盖 REFUNDED”）、R-5 越权指示 QC/Reviewer 不得引用 §7.5、R-2 的事件丢失后果被“后续运单事件可收敛”错误淡化、`order.shipped` 复用语义与审计字段未定义、测试矩阵缺关键异常分支。

**只读声明**：未执行任何测试/迁移；未改 ADR/SPEC/代码/台账；文中代码引用是阅读事实，不构成执行证据。

---

## 1. 必答崩溃窗口（本裁决范围）

| # | 窗口 | 预期行为 / 恢复所有者 | 依据与缺口 |
|---|---|---|---|
| 1 | **本地提交后崩溃** | R-1 是纯读派生（每次读重算），崩溃无副作用，N/A。R-2 若按 ADR:156-166 单事务写 `refund_state`+`refund_state` 投影+生命周期+outbox，则崩溃整体回滚、`shop.aftersale.completed.v1` 未 ACK → durable consumer 重投（`MaxDeliver:10`，`aftersale_consumer.go:26`）。Owner：Order（consumer）。 | `aftersale_consumer.go:39-56`（现为两条 **非事务** `db.ExecContext`，R-2 必须改单事务）；`consumer.go:107-143`；ADR:168-175。缺口：10 次后 DLQ，无自动重扫（见 M-7）。 |
| 2 | **跨服务 RPC 成功后崩溃** | **N/A（本裁决不新增 RPC）**。既有 `CancelIfCreated` 窗口属 W2/W7：由**发起方**（AfterSales）durable refund task 以同一 `after_sale_no` 重入 `AdmitRefund` 收敛（`aftersales/workers.go:390-404`、`refund_admission.go:181,207,296`）；Order 侧**没有**扫描 `PENDING_REMOTE` 的 worker。 | ADR:139 写“既有 worker 按 refund_id 重试”但未指认 AfterSales task；ADR:289 又标 `[不确定]`。见 M-3。 |
| 3 | **消息重复/乱序** | 重复 `aftersale.completed`：`refund_state` UPDATE 幂等（`WHERE refund_state <> 'REFUNDED'`，`aftersale_consumer.go:43-48`）+ rank/`from` 守卫（`consumer.go:105-108`）→ 不重复推进/不发 outbox。重复运单事件：group CAS + `from` 集合。乱序：`shipment.created` 在 `PRE` 之后重投会把组 `status` 写回 `ALLOCATED`（`consumer.go:96-101` 无条件写组状态），但 `groupIsClosed` 仍由 permit/admission 兜底，**有效状态仍 CANCELLED**。 | 缺口：没有任何测试覆盖“`PRE` 后重投 `shipment.created`”；建议 T-TODO（见 M-9）。 |
| 4 | **取消/确认竞态** | 关闭判定是纯读，不参与写裁决；写侧是 permit CAS + 运单行 CAS：Ship 先胜 → `ALREADY_SHIPPED`→`POST`；取消先胜 → `advance` 从 `CANCELLED` 得到 `invalid shipment status` 错误（`service.go:497-507`）→ **不产生 `shipment.shipped`**，与 R-1“PRE ⇒ 不会再离仓”一致。 | `settlement.go:326-357`（refund_state/permit 守卫 + CAS）；`refund_admission.go:472-497`（败者重判）。真正缺口是 B-2 的“CONSUMED+无运单”异常映射，而不是此竞态。 |
| 5 | **外部重放缺失** | R-1 无外部渠道依赖（N/A）。R-2 的唯一触发器是 `shop.aftersale.completed.v1`；若该事件 DLQ 后不再重放，**不会再有运单事件**（AC-27 时序是 G1 先发、G2 后关闭），订单会永久停在 `ALLOCATED`，与 ADR:290 “可由后续运单事件收敛”矛盾。 | ADR:290 §8.2 假设错误；无对账/重算 worker（ADR:166 明确不做）。需 DLQ 告警 + ops 重放 runbook + T6 证据（M-7）。 |

---

## 2. 逐条漏洞

### `[BLOCKER]` B-1 — R-2 写 `orders.status` 与 SPEC-0001 §6.6.1 规范正文冲突，属 PO 裁决

- **证据**
  - ADR:156-166（R-2）：在 `aftersale.completed` 内单事务重聚合，复用 `aggregateOrderStatus` + `updateOrderStatus` CAS，目标 `ALLOCATED→SHIPPED`，并追加 `order.shipped`。
  - SPEC-0001 §6.6.1 最后一条（`0001:289-294`）：**“组退款不改变订单生命周期；退款进度只写 `refund_state`，不得写回 `orders.status`（INV-01/AC-25）”**。
  - ADR:59 / §6.1（`:249-254`）只把 §7.5:405/:409 列为冲突，并称“§7.5 是派生示例表”；**§6.6.1 是规范正文**，不是示例表，ADR 漏检。
  - AC-27 EARS-1（`0001:571-573`）写“推进**订单投影**”，未定义“投影”是 `toProto` 读模型还是持久化 `orders.status`；现有测试（`selfcheck:44-45`）恰好证明 `toProto.shipment_status=SHIPPED` 且 `orders.status` 仍 `ALLOCATED`。
- **具体修改**（任一，需 PO 书面）：
  1. **PO 升版/解释 SPEC-0001**：明确 §6.6.1 的“不得写回 `orders.status`”仅禁止写**退款态**（`REFUNDED/REFUNDING`），并明确 AC-27 EARS-1 要求持久化 `orders.status` 推进；变更记录写“解释性修正，不改 AC 语义”。在此之前 **R-2 不得实现**。
  2. 若 PO 判定“投影”仅指读模型，则 **R-2 取消**，R-1 只修读模型；由 Reviewer 在 AC-27 台账按读模型口径判定（但不得把 `orders.status` 仍 `ALLOCATED` 写成“推进”）。
- **影响**：不修则 R-2 以“Architect 解释”覆盖 §6.6.1 规范文本，违反“业务结论只活在 Spec”（`CONVENTIONS.md:11`）与 ADR 自身 §13.6。

### `[BLOCKER]` B-2 — R-1 的 `PRE_SHIP_REVOKED` 代理把 AC-50 定义为异常的分支正规化为关闭

- **证据**
  - AC-50（`0002:218-225`）：`许可 NONE→REVOKED`→PRE；`许可 REVOKED`→PRE；`CONSUMED+CREATED`→取消→PRE；`CONSUMED+CANCELLED`→PRE；**`许可 CONSUMED 且无运单`→`REFUND_ADMISSION_CONFLICT` 并告警（异常状态，需人工核对）**（`:223`）。
  - `CancelIfCreated`：查不到运单行时直接返回 `statusCancelled`（`cancel_if_created.go:50-55`，注释“No parcel exists: nothing to cancel”）。
  - `finishRemoteAdmission`：把 `CANCELLED` 一律映射为 `refundResultPre`（`refund_admission.go:576-584`），并在 Tx2 写 `result=PRE_SHIP_REVOKED`（`:590-597`）——因此 **CONSUMED+无运单**也会得到 PRE。
  - ADR:87 枚举 PRE 来源只列“NONE→REVOKED、CONSUMED+CREATED→取消、CONSUMED+CANCELLED”，**省略了 `0002:223` 的异常分支**；ADR:113-114 又用 `result=PRE` 做关闭判据。
- **具体修改**
  1. 停止条件（ADR §10.2 #2/#3）：Order 侧把“无运单行”与“运单 `CANCELLED`”区分开。要么 Fulfillment `CancelIfCreated` 对 `CONSUMED+no row` 返回新状态（如 `NOT_FOUND/CONFLICT`），Order 映射为 `REFUND_ADMISSION_CONFLICT` + 告警且**不写 PRE**（需跨服务改动，超出 ADR:17-18 allowed_paths → 交 Lead 拆 change_id）；要么由 PO/Architect 明确“no-row 视为已取消”的偏差并留审计，且 ADR §2.1/§2.2 把它写成已知偏差（owner、expiry、告警），不得在枚举里省略。
  2. 新增测试：`T1b`: CONSUMED+no row → 不得 `PRE`/不得关闭（期望 conflict）；`T1c`: CONSUMED+`CANCELLED` row → PRE/关闭。现有 T1 只用 `fakeFulfillmentPort{status:CANCELLED}`（`refund_admission_integration_test.go:231-245`），无法区分两者。
- **影响**：R-1 的关闭谓词把“异常状态”当成“运单已取消”；AC-27 EARS-2 的“运单 `CANCELLED`”条件未真正满足。

### `[MAJOR]` M-1 — R-2 “唯一 authority”边界未定义，直接复用会写错组状态与审计

- **证据**：现有 `applyShipmentStatusGroup` 在事务里**先写组状态**（`UPDATE order_fulfillment_groups SET status=...`，`consumer.go:96-101`），再用硬编码 `insertStatusLog(..., "shipment projection", "fulfillment")`（`consumer.go:126`）和事件 from 集合（`consumer.go:62-64`）。ADR:156-166 只说“抽成唯一 authority 供两处复用”，未定义不得写组状态/不得固定审计字段；§3.2 约束 1 还可能被读成“必须整体复用”。
- **修改**：Addendum 明确 authority 函数签名与边界，例如 `advanceOrderLifecycle(ctx, tx, orderNo, target, from []string, reason, operator, triggerGroupNo)`：**禁止**写 `order_fulfillment_groups.status`；`reason/operator` 由调用方给（退款路径 `"refund closure projection"/"aftersales"`）；`from` 对 SHIPPED = `{PAID, ALLOCATED}`，对 COMPLETED = `{SHIPPED}`；保留 CAS+status log+outbox。T6 增加断言：G2 `group.status` 仍 `ALLOCATED`、status log 的 reason/operator 是退款投影而非 shipment。

### `[MAJOR]` M-2 — R-1 允许 `LEFT JOIN` 会产生重复组（同组多 admission）

- **证据**：ADR:126 写“可 `LEFT JOIN`/子查询”；`order_refund_admissions` 只有 `PRIMARY KEY(refund_id)` 与 `KEY(order_no, group_no)`（`migrations/order/0016_settlement_admission.sql:8-19`），**没有 `(order_no,group_no)` 唯一约束**；同一组可有多条 admission（不同 `refund_id`，例如前一次退款失败/取消后重开）。`LEFT JOIN` 会让 `loadFulfillmentGroups` 返回重复组 → `remaining`/`shipmentStatusForGroups`/`aggregateOrderStatus` 全部错。
- **修改**：§2.5 改为“用 `EXISTS`/`IN (SELECT ...)` 或先 `GROUP BY (order_no,group_no)` 聚合后并入；禁止无去重的 `LEFT JOIN`”；`loadFulfillmentGroups` 加载后断言每组至多一行（fail-closed）；T 用例造两条同组 PRE 行，断言投影不重复、不误推进。

### `[MAJOR]` M-3 — R-1 的恢复 owner 误写为“Order worker”；实际只有 AfterSales durable task

- **证据**：ADR:139 与 `:289` 称“既有 worker 按 `refund_id` 幂等重试并写回 PRE”；ADR W7 写“Order worker + Fulfillment”（`docs/adr/0007-payment-settlement-admission.md:961`）。但代码中 **Order 没有扫描 `PENDING_REMOTE` 的 worker**：`order_refund_admissions` 的读取只有按 `refund_id` 的 `findRefundAdmission`（`refund_admission.go:45-55`），`finishRemoteAdmission` 只在 `AdmitRefund` 重入时触发（`:181,207,296,436`）。真实驱动是 AfterSales 的 durable refund task：claim/lease → 调 `AdmitRefund` → 失败 `releaseRefundTask` 重试（`aftersales/workers.go:390-404`）。
- **修改**：把 §2.6/§8.1 的 owner 改为“发起方（AfterSales）durable task worker；Order 侧仅按 `refund_id` 幂等收敛”；写明若 AfterSales task 进入 DLQ/租约丢失，Order **不会**自动收敛、组保持未关闭；登记该为已知缺口（owner/expiry）或要求告警，不得再用“既有 worker”概括。

### `[MAJOR]` M-4 — R-1 关闭 + 多组不释放：组投影 CANCELLED 但预留仍 HELD、无释放请求

- **证据**：`appendRefundReleaseTx` 在 `groupCount != 1` 时直接 `return nil`，仅 `log.Warn("multi-group refund keeps inventory reservations (P1/manual)")`（`refund_release.go:178-187`）；组插入时 `reservation_state='HELD'`（`fulfillment_groups.go:55-66`），退款路径不回写。ADR §4.4（`:204-210`）只要求“台账不得标 AC-VERIFIED”，没有给“关闭但无释放请求”任何可观察信号/owner；而 AC-27 的前提正是 2 组订单（`0001:571`）。
- **修改**：(1) Addendum 显式写“**AC-27 EARS-1 PASS 不蕴含 INV-44/INV-06 已满足**”；(2) 在 R-1 实现里对“关闭组所属订单从未产生 release event”打点/告警（可用现有 `hasOrderReleaseEventTx`/日志，不新增契约），并由 ops 对账查询跟踪 HELD；(3) R0.3-R1c 开工前，Lead/Reviewer 必须先落多组释放缺口的 deferral 行（owner/expiry/reason），否则 AC-27 证据不得收口。禁止让 Order 写 `RELEASED`（ADR §4.2 正确，保留）。

### `[MAJOR]` M-5 — R-4 把 `REFUNDING` 关闭说成“正确”，缺 PO 授权且与 SPEC v1.4 记录冲突

- **证据**：ADR:214-235 结论“`ALLOCATED + REFUNDING + REVOKED` 投影 `CANCELLED` 正确，保持”，并称“AC-27 正文没有‘退款未完成不得投影 CANCELLED’”。但 SPEC-0001 **v1.4 变更记录**（`0001:696`）明确：AC-27 迁移“**只覆盖 `refund_state=REFUNDED`（不把 `REFUNDING` 写进‘已完成’）**”；AC-27 EARS-2 的 WHILE 条件也要求 `REFUNDED`（`0001:574-576`）。从“AC 未禁止”推不出“AC 授权在 `REFUNDING` 就关闭”——这是业务权衡（退款仍在途/永久失败时订单即可显示 SHIPPED）。
- **修改**：把 R-4 从“**正确，保持**”改为“**保留既有行为，AC-27 未 sanction**”；由 PO 确认“pre-ship `REFUNDING` 关闭”是否成立（或 AC-27 升版），并在 Addendum 记录 owner/日期；在此之前 R-4 只能用于“不改代码”，不得用于否决 Lead Brief 之外的新语义。补 T4 断言时注明“关闭 ≠ 退款完成；该关闭是产品裁决待确认项”。

### `[MAJOR]` M-6 — R-5 越权指示 QC/Reviewer 不得引用 §7.5；且冲突分析漏 §6.6.1

- **证据**：ADR:261 “在 PO 升版之前，Reviewer/QC **不得**以 `0001:405`/`:409` 作为阻断依据”。§7.5 属于 approved SPEC-0001；Reviewer 的 verdict 范围与阻断依据由 Reviewer/人类按 `CONVENTIONS.md:66-68`、宪法 Amendment 5 判定，Architect 无权预先禁止。另 ADR §6.1 只列 §7.5，漏 §6.6.1（B-1）。
- **修改**：改为“登记为 PO 文档漂移（owner=PO，expiry/thread 由 Lead 登记）；Reviewer 在 verdict 中须记录该冲突并以 AC-27 EARS 正文为准；是否阻断由 Reviewer/人类裁决”。同时把 §6.6.1 加入 §6.1 冲突表，并引用 `0001:696` 的 v1.4 口径。

### `[MAJOR]` M-7 — R-2 对事件丢失的后果评估错误

- **证据**：ADR:290 §8.2 “若该事件永久丢失……接受‘滞后但可由后续运单事件收敛’”。AC-27 的真实时序是 G1 的 `shipment.shipped` **先于** G2 关闭被消费（`selfcheck:58-75`）；G2 关闭后**不会再有运单事件**，因此事件丢失会永久停在 `ALLOCATED`，与 ADR:290 的“可收敛”不符；ADR 又禁止新增对账/重算 worker（`:166`）。
- **修改**：把 §8.2 改成“若 `aftersale.completed` 在 `MaxDeliver` 后进入 DLQ，订单可能永久停在 `ALLOCATED`；恢复 owner=ops 手工重放（现有 DLQ 流程），R0.3-R1c 证据必须包含 DLQ 重放用例或书面残余风险”；如要求自动收敛，必须回 Lead 拆新任务（对账 worker）。

### `[MAJOR]` M-8 — R-2 复用 `order.shipped` 的语义与审计未定义

- **证据**：ADR:162 要求复用既有事件、`group_no` 填“使聚合完成的那个组”。该事件由 `consumer.go:132-139` 发出，本仓库无消费者（只有 registry `internal/platform/events/registry.go:314`），但 BFF/BI 可能订阅；事件名“shipped”由退款关闭触发，可能被误读为有新包裹。另 `insertStatusLog` 的 reason/operator 见 M-1。
- **修改**：在 Addendum 明确“该事件在本路径表示**生命周期聚合完成**，`group_no` 为触发关闭的组，不表示新运单”；要求 BFF/BI 不得据此生成发货通知（或由 Lead 确认无消费者）；若无法接受，回 PO 讨论是否需要独立事件（属契约变更，超边界）。同时明确 **R-2 用 DB 时间还是 `time.Now()`**（现有 `changed_at` 为进程时钟，`consumer.go:138`）。

### `[MAJOR]` M-9 — 测试矩阵缺关键异常分支

- **证据**：§10.1 T1 用 `fakeFulfillmentPort{status:CANCELLED}`（`refund_admission_integration_test.go:231-245`），无法覆盖 B-2 的“CONSUMED+无运单”；没有同组多 admission 用例（M-2）；没有“PRE 后重投 `shipment.created`”用例（窗口 3）；没有 `ListOrders` 批量加载断言（ADR §8.8 自标 `[不确定]`）；没有 `aftersale.completed` DLQ 重放用例（M-7）。
- **修改**：新增 `T1b`（CONSUMED+no row → `REFUND_ADMISSION_CONFLICT`，不写 PRE）、`T1c`（CONSUMED+CANCELLED → PRE）、`T2b`（同组两条 PRE，投影不重复）、`T3b`（PRE 后 `shipment.created` 重投，有效状态仍 CANCELLED、不重复推进/不发 outbox）、`T6b`（组状态/审计字段不被退款路径改写）、`T7b`（DLQ 重放后收敛或残余风险记录）、`T10`（`ListOrders` 批量，不 N+1）。异常分支至少保留 `CancelIfCreated` 超时/`TRANSIENT_ERROR`（宪法 IV.2）。

---

## 3. 状态机死胡同 / 无主（专项）

| 路径 | 谁驱动 | 结论 |
|---|---|---|
| `order_refund_admissions(PENDING_REMOTE)` 崩溃后 | 仅 AfterSales durable task 重入 `AdmitRefund`；Order 无扫描 worker | 有恢复但 owner 被 ADR 写成 Order worker（M-3）；task 丢失即永不关闭 |
| G2 关闭而多组 release 从未发出 | 仅 `log.Warn`，无 outbox/无告警指标 | 预留永久 HELD；AC-27 可绿、INV-44/INV-06 静默违约（M-4） |
| `aftersale.completed` 进 DLQ | ops 手工重放（`docs/ops.md:267-275`） | 无自动收敛；ADR:290 的“后续运单事件”不存在（M-7） |
| `REFUNDING`（退款永久失败）且 permit 终态 | 无（permit 不可恢复） | R-4 让它关闭；业务上是否可接受需 PO（M-5） |
| `orders.status` 停在 `ALLOCATED` 但 `shipment_status=SHIPPED` | 无（R-2 才写 `orders.status`） | R-2 若被 PO 否决，则 AC-27 的“推进”只能停留在读模型；需 PO 解释（B-1） |

---

## 4. 阻塞 / 不阻塞

**阻塞本 Addendum 接受（BLOCKED 的直接原因）**：B-1、B-2。
**阻塞 R0.3-R1c 开工前必须补齐**：M-1（authority 边界）、M-2（重复组）、M-3（恢复 owner）、M-4（deferral/告警）、M-5（PO 确认 R-4）、M-6（§7.5 处置方式）、M-7（DLQ 残余风险）、M-9（测试矩阵）。
**不阻塞本裁决、但阻塞 AC-27 收口/后续台账**：多组释放缺口（M-4）在 P1 前无法让 AC-27 EARS-3 的释放腿可验证；`INV-45..50` evidence 行的既有缺口（R0.3-R3/R0.5）不在本文件范围。
**不阻塞**：R-3 的生产代码零改动结论；R-1 的纯读派生与幂等；R-4 不改代码；R-5 的 PO 文档修正（属 PO，不属本 Addendum）。
**明确不做（正确）**：不新增运单投影/不消费 `shipment.cancelled`、不读路径远程 RPC、不让 Order 写 `reservation_state/RELEASED`、不改 permit/额度/迁移、不新增 worker（ADR §2.5/§4.2/§13）——这些边界与 S4/S5/S6/S9 所有权一致，QC 认可。

---

## 5. 已验证成立的部分

- **R-1 的代码事实**：`groupIsClosed` 现按 `Status→CANCELLED / SHIPPED|RECEIVED→false / PermitState==REVOKED` 判定（`settlement.go:65-78`），与 ADR:103-115 的既定顺序一致；PRE 常量与写入点存在（`refund_admission.go:15`、`:256-288`、`:590-597`），`0016:18` 有 `(order_no, group_no)` 索引；`PRE` 只由 `NONE→REVOKED` 与 Tx2 `CANCELLED` 写入（TCC 写 `POST`，`tcc_admission.go:280-286`），不存在“TCC 误写 PRE”的路径。
- **R-1 不违反 S5/S9**：关闭判定读取的是 Order 自有 `order_shipping_permits`、`order_refund_admissions`、`order_fulfillment_groups`；不把 `refund_state` 当禁发点（S9 `0002:79`），不新增运单事实（S6 `0002:76`、`ports.go:47-55`）。
- **R-2 的单事务方向正确**：现有 `applyAfterSaleCompleted` 是两条非事务 SQL（`aftersale_consumer.go:41-56`），改为单事务 + 复用 CAS + outbox 同事务符合宪法 III.1；`updateOrderStatus` 的 `status=from` CAS 存在（`repo.go:282-290`），`aggregateOrderStatus`/`rankStatus` 存在（`consumer.go:163-219`）。
- **R-2 的 `MaxDeliver:10`** 属实（`aftersale_consumer.go:26`）；shipment 路径的 `insertStatusLog`/`outbox.Append` 与状态写同事务（`consumer.go:124-140`）。
- **R-4 不改代码**：`groupIsClosed` 第 4 支（`REVOKED`）现有行为确实让 pre-ship `REFUNDING` 投影 `CANCELLED`；离仓守卫在其之前（`settlement.go:74-76`），post-ship 不被误伤（`ac27_projection_negative_test.go:9,42` 既有测试）。
- **R-5 的 §7.5:405 冲突属实**：`0001:405` 写 `ALLOCATED / PARTIALLY_SHIPPED`，AC-27 EARS-1 写“不得停留在 `ALLOCATED`”；`0001:447` 的“AC 可由测试人员判定”引文准确。
- **R-3 的所有权结论正确**：`reservation_state` 写点只有插入 `HELD` 与整单取消 `RELEASED`（`fulfillment_groups.go:55-80`），退款释放走 outbox（`refund_admission.go:284-288,598-602`）；禁止 Order 写 `RELEASED` 与 S4/INV-44 一致。

---

## 6. `[TODO]` 清单与边界声明

- **T-1** ADR:10 的 AC-27 行号写 `571-578`，实际 AC-27 正文到 `579`（EARS-3 第 3 行）；§6.6 起始 `281` 而非 `279`。属引用精度，[TODO]。
- **T-2** ADR:126 写“可 `LEFT JOIN`/子查询”，建议直接指定 `EXISTS` 并说明 `idx_order_refund_admissions_order_group` 只覆盖 `(order_no,group_no)` 前缀、`result` 过滤为回表（P0 组数 ≤2，可接受）。
- **T-3** `order_refund_admissions.result` 在 `0016` **没有 CHECK/枚举约束**（`0016:8-19`），R-1 依赖 Go 常量 `refundResultPre`；建议至少在读模型集中定义常量/注释，若加 DB CHECK 需另立迁移（超本裁决边界）。
- **T-4** R-3 §4.3 的证据路径请写具体：Inventory 侧 `refund_release_consumer.go` 的 `Release/TccCancelConfirm` 测试 + `stock_ledger`；多组场景当前无 release event，不得用 Order fixture 冒充。
- **T-5** R-3 的 §4.4 deferral 行需在实际台账落 owner/expiry（Lead/Reviewer），否则 R0.3-R1c 不得开；本报告不代改台账。
- **T-6** ADR:172 “consumer 永久失败 → 组退款态回滚为 REFUNDING” 仅在 R-2 单事务成立后有效；当前非事务实现下第二条 SQL 失败会留下 `refund_state=REFUNDED` 且缺货投影未写——R0.3-R1c 必须先包事务，再加此断言。
- **T-7** `ListOrders` 的组加载路径（`service.go:397` 逐单循环 + `service.go:944`）需 Developer 在实现前确认批量策略（ADR §8.8 自标 `[不确定]`），否则新增 admission 查询会放大 N+1。
- **T-8** `changed_at` 使用 `time.Now().Unix()`（`consumer.go:138`）而非 DB 时间；既有多路径一致，但事件排序/审计口径应在 Addendum 说明（不阻塞）。
- **T-9** ADR:230 引用 `tasks.md:47` 的 Lead 更正准确（已核）；建议同时引用 `REV-PAY-SETTLE-01-R0.3.md` 的 AC-27 AC-REF 状态，避免把“测试 PASS”读成 AC-VERIFIED。

- **未执行**：`go test`、`-race`、真实 MySQL；本报告不构成执行证据；`[不确定]`：`[BLOCKER]`/`[MAJOR]` 中涉及真实数据分布的判断（同组多 admission 的实际规模、DLQ 重放频率）未在本次统计。
- 未改 ADR/SPEC/代码/台账；只新建本文件；未 commit。
- 下游：R0.3-R1b 回 Architect 处理 B-1/B-2 与 M 项 → QC 复核 → 人类接受 → 才能进 R0.3-R1c；R-5 的 §7.5 修正归 PO；AC-27 证据归 Reviewer，任何 AC-VERIFIED 都必须绑定真实执行日志与精确 HEAD。
