# QC-PAY-SETTLE-01-AC50-NOWB — AC-50-FIX-01「无运单」additive amendment 红队审查

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01`（子范围 `AC-50-FIX-01`；实施须由 Lead 另拆独立 `change_id`，见 ADR §0.4） |
| 审查对象 | `docs/adr/0007-amd1-addendum-ac50-nowaybill.md`（**DRAFT 0.1 — pending QC**；460 行） |
| 结论 | **BLOCKED**（3 个 `[BLOCKER]` + 13 个 `[MAJOR]` + 7 个 `[TODO]`；DRAFT 不得进入实现） |
| 日期 | **2026-11-26** |
| Actor | `qc-ac50`（Quality Checker；与 Architect `arch-ac50`、Developer、Reviewer 不同 actor） |
| base | `da0cdd15d065a503ffa1e6d545e7381f6766737d`（`git rev-parse HEAD`；`docs(lead): Phase 0-R checkpoint after Reviewer verdicts`） |
| head | `da0cdd15d065a503ffa1e6d545e7381f6766737d`（`git rev-parse HEAD`，审查期间未变；被审 ADR 为**工作树未跟踪文件**，blob `53e799e14927a0da063c5c399d01a9a907fea31c`，sha256 `a344e9b8e5c296892ae13ea240c10dcb6ac45e33a1a6c85deb04be501df42767`） |
| Tier | **L2（确认）**：既有跨服务 RPC `Fulfillment.CancelIfCreated` 响应语义、资金/履约准入、AC-27A 关闭谓词；按 `docs/collab/CONVENTIONS.md:90` 任一命中即 L2，不得降级 |
| spec_version / adr_revision | SPEC-0001 **v1.5**（approved 2026-11-26，`4215b3c`）；SPEC-0002 **v1.10**（approved 2026-11-25）；ADR-0007 **Rev 2.3**（accepted 2026-11-25） |
| integration_owner | System Architect（契约）；实现 owner = Developer（另拆 `change_id`）；人工队列 = ops + Developer + Fulfillment |
| review_scope | 只审本 Addendum 的 R-1…R-8、§2–§9、§10 测试矩阵、§12 假设、§14 INV/AC 映射，以及其引用的 Order/Fulfillment 代码事实、SPEC-0001 v1.5 AC-27/AC-27A/§16.2/§16.3、SPEC-0002 v1.10 AC-50/AC-62/INV-36/42/43/44/45/50、ADR-0007 Rev 2.3 §5.2/§5.3/§5.7。**不审** R0.4 迁移（`0022/0023`、`r04_checks_integration_test.go`、`scripts/verify-r01-migrations.sh`）及其他工作树并发改动（`docs/ops.md`、`docs/review-tracking.md`）；代码只读，未执行测试/迁移。 |
| baseline_impact | **涉及**：AC-50 `CONSUMED+无运单` 写路径、AC-27A 关闭谓词的 PRE 来源、AC-27 收口前提、Fulfillment/Order 既有 RPC 响应消费。**不涉及**：Payment 金额/退款实现、迁移/schema、事件 subject/payload 结构、permit 状态机、新 worker。 |
| INV | INV-36、INV-42、INV-43、INV-44、INV-45、INV-50；另核对 INV-01/AC-25（不得写订单生命周期/退款态）。 |
| affected_AC | AC-50、AC-27A、AC-27、AC-62、AC-48（非回归）、AC-67/AC-68（零元非回归）；AC-27 仍 **AC-REF**，不是 AC-VERIFIED。 |
| release_status | **NOT RELEASE**；不批准 merge、不声称 AC-VERIFIED / 测试已执行 / Release PASS。 |

---

## 0. 结论

**BLOCKED。**

R-1 的“新增 `NO_SHIPMENT` 字符串值、无字段/无 RPC/无枚举变更”方向正确；R-2/R-3 把 `NO_SHIPMENT` 映射为 `REFUND_ADMISSION_CONFLICT`、禁止 PRE/POST 的方向也正确；R-4 的“两个映射点 + 单一映射表”与当前代码的两个 `CANCELLED` 分支（`refund_admission.go:384-391`、`:577-584`）一致；R-7/R-8 的人工队列与 proto 注释登记面基本对齐 SPEC-0001 §16.2。**但 Addendum 的核心“conflict 路径无写入、无 outbox、Payment 调用 0”与当前 Tx1 事实直接矛盾，O1/O6/O9 三条核心测试按现文本不可实现；W6 的人工修复又与其自身 §2.3 #1 相冲突。** 因此 DRAFT 0.1 不能进入实现。

3 个 `[BLOCKER]`：

1. **B-1**：CONSUMED 的 Tx1 在远程探测前已写 `admission`、`commitment`、`order.refund_admitted` outbox（`refund_admission.go:253-283`，`:281`）。Addendum 却声明 conflict “不写 outbox”（`:148`、`:155`）、W2 “未发生任何写入”（`:264`）、§9 “conflict 不产生 order.refund_admitted”（`:304`）、O1 “无 outbox”（`:320`）——全部为假；O1 按现文本必失败。
2. **B-2**：正金额/TCC 路径在 Tx1 前已调用 `Payment.GetIntent`（`refund_admission.go:168-175`），且 Tx1 必然写 admission/commitment；O1 “Payment 调用 0”（`:320`）、O6 “无写入”（`:325`）、O9 “无 Payment”（`:328`）与“不得调 Payment”（`:148`）的绝对措辞不成立；必须改成“无 `Payment.Refund/RefundBatch`”并区分 fresh / 预置 `PENDING_REMOTE` 两种测试 setup。
3. **B-3**：W6 写“人工修复补出运单行”（`:268`），与本 Addendum §2.3 #1“不得为无行补建一行 `CANCELLED` 运单（会伪造 S6 事实）”（`:127`）、SPEC-0002 S6 ownership（`0002:76`）及 SPEC-0001 `AC-27-NOWB-01` 终态①（`0001:863`，只允许 Fulfillment 提供/修复 S6 证据、Order 不写运单）冲突。若允许“补出 `CREATED` 再取消”，等于用人工制造的 S6 事实把 conflict 洗成 PRE，直接破坏 A3 与 AC-27A 的前提。

其余 `[MAJOR]` 集中在：A3 “PRE 只来自合法分支”在 legacy `NONE` 无许可行 + 已发货场景并不成立；未保留 AC-50 的部分金额前置分支；A4 的“无 admission 终态不得 release”与 AC-62 人工授权释放冲突；TCC Tx1 把 `PENDING_REMOTE` 写进 `result` 与 SPEC-0001 注“result 只允许 PRE/POST”不一致；未处理存量错误 PRE 的一次性审计；TCC 测试路径被 `forbidden_paths` 禁掉；TCC 冲突的人工队列发现口径只写了 AfterSales `last_error`；消息重复/乱序无 W 行；`finishRemoteAdmission` 的 Tx2 无 `state/version` 守卫，重复探测可能重复 outbox/释放；POST 的 commitment“同事务”描述对正金额路径不成立。详见 §3。

---

## 1. 必答崩溃窗口（逐条回答）

> 本报告回答 QC prompt 的 5 个窗口。Addendum §7.2 给了 W1–W8，但**没有独立覆盖“消息重复/乱序”**；下表同时标出 Addendum 的覆盖与缺口。

| # | 窗口 | 预期行为 / 恢复所有者 | 结论与缺口 |
|---|---|---|---|
| 1 | **本地提交后崩溃**（Tx1 已提交 `PENDING_REMOTE`，远程未发出/未受理） | 发起方 durable task（AfterSales 的 `after_sale_refund_tasks` / TCC batch）以同一 `refund_id` 重入 `AdmitRefund`；读 `PENDING_REMOTE` → 重新探测 → `NO_SHIPMENT` → conflict；不写 PRE。Owner：AfterSales/TCC durable task + Order 幂等。 | **方向成立**（W1；`refund_admission.go:179-185,203-211,296-298`；`aftersales/workers.go:399-405`）。**但**Tx1 已写 admission/commitment/outbox（B-1/B-2），W2 的“未发生任何写入”必须改（W1 已承认 Tx1 写入）；重放前必须把 Tx1 的 `order.refund_admitted(result=POST,state=PENDING_REMOTE)` 当作非终态事件，否则观察者会误读。 |
| 2 | **跨服务 RPC 成功后崩溃**（Fulfillment 已返回 `NO_SHIPMENT`/`CANCELLED`，Order 未记录/未返回） | `NO_SHIPMENT`：远程无副作用；重放重新探测，确定性回到 conflict，不写 PRE。`CANCELLED`：运单行已 `CANCELLED`；重放 `CANCELLED` → Tx2 PRE，不丢结果。Owner：发起方 task。 | **W2/W3 方向成立**（W3 正确：`refund_admission.go:566-607`）。**W2 的“未发生任何写入”错误**（B-1/B-2）：Tx1 已写 admission/commitment/outbox。必须改为“Tx1 写入已存在；conflict 不再新增写入”。 |
| 3 | **消息重复/乱序** | 重复 `order.refund_admitted`：Fulfillment consumer 只处理 `result` 为 `PRE_SHIP_REVOKED` 的事件（`admission_consumer.go:40-55`）；重复 PRE 调 `CancelIfCreated`，`CANCELLED` 幂等、`NO_SHIPMENT` 无行 no-op、`ALREADY_SHIPPED/RECEIVED` 只告警，不再取消。乱序：`order.paid` consumer 在 PRE/REVOKED 后通过 `autoShipAllowed` 不再创建 placeholder（`fulfillment/consumer.go:69-75,113-125`）；`aftersale.refund_requested` consumer 的 `CancelCreatedShipment` 在行不存在时 no-op（`refund_consumer.go:35`）。Owner：Fulfillment consumer + Order。 | **Addendum 没有独立 W 行/测试**；现有机制可收敛，但需补 W9 与测试。**另外**：`finishRemoteAdmission` 的 Tx2 `UPDATE ... WHERE refund_id = ?` 无 `state=PENDING_REMOTE`/version 守卫（`:590-594`），并发同 `refund_id` 的两次 Tx2 可能重复写 `order.refund_admitted`/release（`appendRefundReleaseTx` 的 `hasOrderReleaseEventTx` 仅在事务内查重，`refund_release.go:187-193`），见 M-8/M-9。 |
| 4 | **取消/确认竞态** | 运单行 CAS：Ship 胜 → `ALREADY_SHIPPED` → POST；取消胜 → `CANCELLED` → PRE；Ship 后置失败得 `SHIPMENT_CANCELLED`（AC-48）；permit 保持 CONSUMED。Owner：Fulfillment（行 CAS）+ Order（映射）。 | **W4 方向成立**（`cancel_if_created.go:72-91`）。**缺口**：未覆盖“无行探测与 `CreateShipment` 并发”：无行返回后、重试前新行插入，本次返回 conflict（fail-closed），但需要显式说明“重试/人工复核”，并补测试；未覆盖 `AuthorizeShipment` 直接 RPC 造成的 `CONSUMED+无行`（不是正常 `Ship` 路径，见 M-12）。 |
| 5 | **外部重放缺失** | `PENDING_REMOTE` 永久悬挂：Order 无扫描 worker；AfterSales task 达到 `maxTaskAttempts=8` 后 `FAILED_PERMANENT`，由 ops 手工重放（AC-61）或转 AC-27-NOWB-01；TCC batch 停留 `ADMITTING`，由 ops/发起方重放。系统**不会**自行到达终态。Owner：ops（重放）+ Developer（AfterSales task）。 | **W8 方向成立**（W8；`aftersales/service.go:57`、`workers.go:724-748`）。**缺口**：TCC conflict 没有 AfterSales `last_error`；Addendum §8 的 queue 发现口径只写 `last_error`（`:292`），未覆盖 `refund_batches.state='ADMITTING'` + `refund_applications.state='PENDING'`，见 M-7。 |

---

## 2. `[BLOCKER]` 逐条

### `[BLOCKER]` B-1 — `CONSUMED` 的 Tx1 已写 outbox；Addendum 的“conflict 不写 outbox / O1 无 outbox”与代码事实相反

**证据**

- `internal/services/order/refund_admission.go:253-283`：`AdmitRefund` 的 Tx1 对 `permit.state == CONSUMED` 写 `result=POST_SHIP_ALLOWED, state=PENDING_REMOTE`（`:254-265`）、写 `order_refund_commitments(RESERVED)`（`:266-272`）、并在 `:281` **无条件**调用 `appendRefundAdmitted(... result, amount, 1, state)`；对 `NONE` 则写 `result=PRE,state=PRE` 后也写同一 outbox。`appendRefundAdmitted` 的 payload 在 `:610-619`。
- Addendum `:148`：`NO_SHIPMENT` 行写“**不开 Tx2、不 UPDATE admission、不写 outbox、不写 release**”；`:155`：“现有的 `appendRefundAdmitted`（outbox）同样不得触发”；`:264`（W2）：“**未发生任何写入**”；`:304`（§9）：“conflict **不产生** `order.refund_admitted`（不写 PRE/POST 结果）”；`:320`（O1）：“**无 `order.refund_admitted` outbox**”。
- 结论：conflict 是在 Tx1 **提交之后**、远程探测时发生的；Tx1 的 outbox 事件已经存在（`result=POST, state=PENDING_REMOTE`）。O1 若按现文本断言 outbox 数为 0，必失败；W2 “未发生任何写入”为假。这是**文本与代码事实的直接矛盾**，不是措辞问题。

**具体修改（二选一，需 Architect 在 Addendum 明确）**

1. **最小修正（推荐，保持 allowed_paths 不变）**：承认 Tx1 的 outbox 是**非终态占位事件**，新增不变量 A7：
   - `order.refund_admitted` 只有在 `state == result && state ∈ {PRE_SHIP_REVOKED, POST_SHIP_ALLOWED}` 时才是准入事实；`state=PENDING_REMOTE` 的 `result=POST` 占位事件**不是**结果，任何消费者必须忽略（Fulfillment 当前已因 `result != PRE` 忽略，但要在 §2.2/§9 写明）。
   - 将 §3.1 的“不写 outbox”改为“conflict 不新增 **terminal** outbox；Tx1 的占位事件保留”，§9 改为“conflict 不产生 **terminal** PRE/POST 事件”，O1 改为“恰好 1 条 Tx1 占位事件（`result=POST,state=PENDING_REMOTE`），0 条 terminal 事件”；W2 改为“Tx1 写入已存在，conflict 不再新增写入”。
   - 建议同时要求（或另立 L2）把 Tx1/事件消费的 `state=result` 判据变成代码校验；否则“占位 `result=POST`”仍可能被未来消费者误读。
2. **若设计坚持“conflict 完全无 outbox”**：必须把 `AdmitRefund` 的 Tx1 outbox 改为**仅 terminal 时写**（CONSUMED 的 PRE/POST 事件移到 Tx2），并显式扩展 `allowed_paths` 到 `AdmitRefund`（当前 `:17` 只列 `finishRemoteAdmission` + 零元函数），同时补：Tx1 无 outbox 的测试、Tx2 事件与 admission 同事务、W1/W2/W3 的重新分析、TCC/Aftersales 既有测试调整。这不是“不写代码”的 additive 增量，必须由 Architect 另立裁决。

### `[BLOCKER]` B-2 — “Payment 调用 0 / 无写入”的绝对措辞与 `AdmitRefund` 的 Payment 快照 + Tx1 写入冲突

**证据**

- `internal/services/order/refund_admission.go:168-175`：正金额路径在任何 permit 分支之前调用 `s.payments.GetIntent` 取权威 captured/refunded；TCC 路径同样经 `AdmitRefund`。`NO_SHIPMENT` 只能发生在 `GetIntent` 之后。
- `refund_admission.go:253-283`：Tx1 写 `order_refund_admissions`、`order_refund_commitments`、outbox（见 B-1）。
- Addendum `:148`：`NO_SHIPMENT` 的“打款/释放”列写“**不得调 Payment**、不得释放…”；`:320`（O1）：“**Payment 调用 0**”；`:325`（O6）：“**无写入**”；`:328`（O9）：“**无 Payment**”；`:264`（W2）：“未发生任何写入”。
- 结论：`GetIntent` 是 Payment RPC，且在 conflict 前已发生；fresh `AdmitRefund` 的 Tx1 也必然写 admission/commitment/outbox。O1/O6/O9 的“Payment 0 / 无写入”按字面不可实现；O3（预置 `PENDING_REMOTE` 行）才可能“无新写入”。这会让“fail-closed”证据无法落地，也会误导 Reviewer 把 `GetIntent` 当成违规。

**具体修改**

- 将 §3.1 的“不得调 Payment”改为“**不得调用 `Payment.Refund` / `RefundBatch`**（不得产生打款/退款命令）；`GetIntent` 只读快照允许且必要”。
- 将 A4（`:217`）改为“无 admission 终态 ⇒ 自动路径不得调用 `Payment.Refund`/`RefundBatch`、不得写 release/commitment release…”，不要写“不得调用 Payment”。
- O1/O6/O9 的断言改为：
  - `payments.refundCalls == 0`（或 fake 的 `Refund/RefundBatch` 调用为 0），**不**断言 `GetIntent` 为 0；
  - 明确 setup：O1/O6/O9 若走 fresh `AdmitRefund`，断言 Tx1 写入了 `PENDING_REMOTE` + commitment + 占位 outbox；O3 用预置行断言“无新写入”；零元 O7 才能断言“Order 完全不调 Payment”。
- 将 W2 的“未发生任何写入”改为“Tx1 写入已存在；conflict 只新增 Error 日志，不新增 admission/outbox/release”。

### `[BLOCKER]` B-3 — W6“人工修复补出运单行”与 §2.3 #1 / S6 ownership / AC-27-NOWB-01 终态①冲突

**证据**

- Addendum `:268`（W6）：“`NO_SHIPMENT` 后经人工修复**补出运单行**（AC-27-NOWB-01 终态①）→ 重放重新探测：`CREATED`→取消→PRE；`SHIPPED`→POST”。
- 同一文件 `:127`（§2.3 #1）：“**不得为“无行”补建一行 `CANCELLED` 运单**（会伪造 S6 事实并让 AC-27A/审计误判）”；`:128` 还禁止 Fulfillment 写 admission/commitment/outbox。
- SPEC-0002 `:76`：S6 运单/签收证据归 Fulfillment，Order 不得制造；SPEC-0001 `:863`（AC-27-NOWB-01 终态①）：“由 Fulfillment 提供/修复 S6 运单取消证据，ops 凭 Fulfillment 授权核对（Order 不写运单），补齐后按 AC-50 合法分支重放→PRE→组关闭”。
- 结论：如果“无行”是**从未存在**的运单，W6 的“补出运单行再取消”就是人工制造一个 `CREATED`/`CANCELLED` S6 事实，再把 conflict 洗成 PRE；这正是 §2.3 #1 要防的。若原行存在但被丢失，允许“修复”也必须有真实承运/仓证据与审计，不能由 ops 凭“补一行”完成。当前 W6 没有区分“真实证据修复”与“伪造行”，会把 A3 的前提打破。

**具体修改**

- 将 W6 改为：“Fulfillment 提供/修复**既有** S6 取消证据（真实运单记录/承运商取消事实/仓出库撤销凭证；Order 不写运单）；ops 凭 Fulfillment 授权核对。**不得为从未存在的运单新建一行**来制造取消证据；若无法提供真实证据，走终态②（保持可重试/人工队列，不写 PRE、不关闭、不打款）。”
- 补一条测试/演练：`NO_SHIPMENT` 后只允许“真实 `CANCELLED` 行”或“真实 `SHIPPED` 行”重放；若测试用 fixture 直接插入一行 `CANCELLED` 冒充证据，Reviewer 应 FAIL（与 §2.3 #1 一致）。
- 如果人类/SPEC 确实允许“数据修复补行”，必须在 SPEC-0001 §16.2 或 ADR 中写明**证据字段、授权人、审计、只允许修复已丢失行**，并给出 owner/expiry；当前 Addendum 无权自行放宽。

---

## 3. `[MAJOR]` 逐条

### `[MAJOR]` M-1 — A3“PRE 只来自合法分支”在 legacy `NONE` + 已发货场景不成立

**证据**

- SPEC-0002 `0002:218-219`：`NONE` 分支的前提是“**该组未离仓**”；`0002:225` 已离仓应 `POST_SHIP_ALLOWED`。AC-27A `0001:620` 的离仓守卫也说明 `SHIPPED/RECEIVED` 优先。
- `internal/services/order/settlement.go:263-278`（`ensurePermitTx`）：无 permit 行时无条件创建 `NONE`，不检查 group/运单是否 `SHIPPED/RECEIVED`。
- `internal/services/order/refund_admission.go:249-288`：`permit.state != permitConsumed && amount == groupRemaining` 即走 `result=PRE,state=PRE`，对 `NONE` 不做任何运单/离仓探测。
- Addendum `:216`（A3）：“`PRE_SHIP_REVOKED` 只能来自 AC-50 合法分支：`NONE→REVOKED`、`REVOKED` 幂等、`CONSUMED+CANCELLED`（真实取消）”；`:171`（legacy 行）：“legacy 的‘无许可行’是 `NONE`（合法 PRE）”；`:329`（O10）：“legacy 无许可行时仍 `NONE→REVOKED→PRE`”。

**具体场景**：历史 `orders.status=SHIPPED` 的订单（0010 以前/未建 permit），`order_fulfillment_groups` 为 `SHIPPED` 或缺失；AfterSales 普通售后退款调用 `AdmitRefund`（`workers.go:399-405`，不先判断 group 是否离仓）；`ensurePermitTx` 创建 `NONE`，金额等于组剩余 → `NONE→REVOKED` + `result=PRE` + `appendRefundAdmitted` + `appendRefundReleaseTx`。这违反 AC-50 “该组未离仓”的前提：已发货组应 `POST_SHIP_ALLOWED`。若 group 投影为 `SHIPPED`，AC-27A 第 3 条会阻止关闭；但 admission 结果已经错误，且若 group 投影非 SHIPPED（历史数据缺失/不一致），AC-27A 第 4 条（许可 `REVOKED`）会以错误的 PRE/REVOKED 关闭。

**具体修改**

- 不要把 A3/§6.1 写成无条件的“PRE 只来自合法分支”；改为“本 amendment 只消除 `CONSUMED+无运单` 的非法 PRE；`NONE→REVOKED` 的合法性依赖调用方已确认未离仓，见假设/缺口”。
- 在 §12 增加一条 `[不确定]/[DEFERRED]`：legacy 无 permit 且已离仓的订单，`AdmitRefund` 目前无 S6 校验；owner = Architect/PO，expiry 2026-12-31；在关闭前 AC-27A 第 4 条（`REVOKED`）不得对 legacy 已发货组标 AC-VERIFIED/已关闭。
- 若要现在修：在 `NONE` 分支 PRE 前用 Order 自有 group 状态（`SHIPPED/RECEIVED` → POST）或事务外 `ListShipments`（fail-closed）确认未离仓；但这会改变 `NONE` 分支和无远程调用约束，必须由 Architect 另立裁决，不能夹带进 AC-50-FIX-01。
- O10 增加一个“legacy 无 permit + 已发货”反例：应 POST 或拒绝；不得以 `legacy` 名称默认 PRE。

### `[MAJOR]` M-2 — 未保留 AC-50 的“部分金额先 PARTIAL”前置分支

**证据**

- SPEC-0002 `0002:217`：IF 该组未离仓 AND 本次金额 < 整组退款金额 → `PARTIAL_REFUND_REQUIRES_FULL_GROUP`，不得改动许可/运单/打款；SPEC-0001 `0001:827`（V15-3）也明确“且仅在‘未离仓、本次金额 = 整组退款金额’分支；部分金额先按 AC-50 返回 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`”。
- 代码 `refund_admission.go:249-252`：部分金额校验只对 `permit.state != CONSUMED` 生效；`CONSUMED` 完全跳过。`finishRemoteAdmission` `:577-584` 对 `CANCELLED` 一律 PRE，不看金额。
- Addendum `:147`（`CANCELLED` → PRE）、`:148`（`NO_SHIPMENT` → conflict）、`:185-187`（零元表）、O4 `:323`（`CONSUMED+CANCELLED` → PRE）均未写“金额 = 整组”前提。

**影响**：对一个 `CONSUMED` 但未离仓的组，若 AfterSales 请求部分金额，当前路径会走到 `finishRemoteAdmission`：`CANCELLED` → PRE + release；`NO_SHIPMENT` → conflict。前者违反 AC-50/D-19，可能把部分退款当成 pre-ship 全组关闭；后者虽然是 fail-closed，但错误码应为 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`。

**具体修改**

- 在 Addendum §3.1/§4.1 明确：`NO_SHIPMENT`/`CANCELLED` 的 PRE/conflict 只在“金额 = 整组剩余”时适用；否则先返回 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`（对 `CONSUMED` 也一样）。
- 若实现选择在 `finishRemoteAdmission` 内判断，可从 `groupPaid + groupFreight - committed` 重算 `groupRemaining`（与 `AdmitRefund:228-248` 同口径），在探测结果后、写 Tx2 前返回 `PARTIAL`；这属于 `allowed_paths` 内的 `finishRemoteAdmission`，需 QC/Reviewer 复核。
- O4/O1 增加金额前提；新增 O4b：`CONSUMED+CANCELLED` + 部分金额 → `PARTIAL_REFUND_REQUIRES_FULL_GROUP`，无 PRE、无 release；O1b：`CONSUMED+NO_SHIPMENT` + 部分金额同样先 PARTIAL。

### `[MAJOR]` M-3 — A4 的“无 admission 终态不得 release”与 AC-62 人工授权释放冲突

**证据**

- Addendum `:217`（A4）：“无 admission 终态 ⇒ **不得**调用 Payment、不得写 release/commitment release、不得把申请置 `DONE`/`PROCESSING`”。
- SPEC-0002 `0002:311-322`（AC-62）：`FAILED_PERMANENT` 或运营决定放弃时，承诺保持 `RESERVED`，在 D-24 人工授权矩阵下可 `RESERVED→RELEASED`；INV-45 `0002:129` 同一口径。Addendum §8 终态②（`:289`）只说“仅人工放弃时按 AC-62 记 `FAILED_PERMANENT`”，未写人工释放例外；A4 的绝对措辞会禁止 AC-62 允许的唯一释放路径。

**具体修改**

- A4 改为“**自动路径**在无 admission 终态时不得调用 `Payment.Refund/RefundBatch`、不得写 release/commitment release…；**AC-62 的 D-24 人工授权 `RESERVED→RELEASED` 是唯一例外**，释放只释放额度，不恢复 permit、不当作退款完成、不写 `DONE`。”
- §8 终态②补一句“人工放弃后按 AC-62/D-24 的授权释放承诺；组保持未关闭，permit 保持 CONSUMED；不得自动关闭/自动退款/恢复 permit。”
- 补测试：`NO_SHIPMENT` conflict 后人工授权释放只允许一次、不恢复 permit、组仍不关闭；未授权释放返回 `REFUND_RELEASE_NOT_AUTHORIZED`。

### `[MAJOR]` M-4 — TCC Tx1 把 `PENDING_REMOTE` 写进 `result`，与“result 只允许 PRE/POST”及 A2 的读者口径冲突

**证据**

- `internal/services/order/tcc_settlement.go:847-852`（`ensureTccRemoteAdmissionTx`）：`INSERT INTO order_refund_admissions (... result, state ...) VALUES (?, ?, ?, ?, ?, ?, 1)`，传入 `refundPending, refundPending`；即 `result=PENDING_REMOTE, state=PENDING_REMOTE`。
- SPEC-0001 `0001:645`（AC-27A 注）：“`result` 只允许 `PRE_SHIP_REVOKED`/`POST_SHIP_ALLOWED`，`state=PENDING_REMOTE` 是中间态、不等同于 PRE。”Addendum `:215`（A2）也要求“终态当且仅当 `state ∈ {PRE,POST}` 且 `state=result`”；`:393` 的 INV-50 描述未提 TCC `result=PENDING_REMOTE`。
- 结果：TCC 的 `PENDING_REMOTE` 同时出现在 `result` 和 `state`。若读者实现成“`state == result` 即终态”（少写 `state ∈ {PRE,POST}` 的前半句），会把 TCC 的 PENDING 当成终态；AC-27A 若只读 `result` 也会面对一个 SPEC 注释不允许的值。Addendum 未把这一既有偏差登记为 `[不确定]/[DEFERRED]`。

**具体修改**

- 在 §6/§14 明确：`result` 的**唯一合法值**在标准中是 PRE/POST；TCC Tx1 的 `result=PENDING_REMOTE` 是既有偏差（`tcc_settlement.go:847-852`），AC-27A 读模型必须同时校验 `state ∈ {PRE,POST}`，不得只凭 `state==result` 或只读 `result`。
- 将 TCC `result=PENDING_REMOTE` 登记为独立 L2 清理（owner = Architect/Developer，expiry 2026-12-31）：要么新数据写 `result=''`/不再写中间值，要么在 SPEC-0002 明确允许；不得在 AC-50-FIX-01 中夹带。
- O13 增加 TCC `result=PENDING_REMOTE,state=PENDING_REMOTE` 用例，断言 `groupIsClosed=false`。

### `[MAJOR]` M-5 — 未处理“存量错误 PRE”的一次性审计（AC-27-NOWB-01 只写了未来 conflict）

**证据**

- SPEC-0001 `0001:645`：“存量无法证明由 AC-50 合法分支写入的 PRE 按 §16.2 `AC-27-NOWB-01` 一次性审计，审计结论未出前台账不得把该组标为已关闭/AC-VERIFIED”。
- Addendum `:227` 只写“本 amendment 不修改 AC-27A 文本/不实现读模型”；§8 `:285-292` 只定义**新的** `NO_SHIPMENT` conflict 的终态①/②；没有既有 `result=PRE` 的审计查询、owner、expiry、读模型门禁。
- 影响：修复上线后，过去由“无行→CANCELLED→PRE”写出的存量 PRE 仍会让 AC-27A 第 5 条关闭组；Addendum 的“修复后 PRE 只来自合法分支”只对**新写**成立，无法证明存量。

**具体修改**

- 在 §8 增加“存量审计”行：`SELECT order_no, group_no, refund_id FROM order_refund_admissions WHERE result='PRE_SHIP_REVOKED' AND state='PRE_SHIP_REVOKED'`，与 `order_shipping_permits`/运单/退款申请交叉核对；owner = ops（数据核对）+ Developer（Order 查询），expiry 2026-12-31；审计结论未出前 AC-27A 第 5 条不得对这些组标已关闭/AC-VERIFIED。
- 读模型（R0.3-R1c）必须支持“存量审计白名单/排除”，或审计完成前对 PRE 关闭打 `[DEFERRED]` 告警；不得用本 fix 的“未来合法”替代存量证明。
- 不要求本 Addendum 改台账；只在 §8 明确 owner/expiry 和 AC-27 收口前提。

### `[MAJOR]` M-6 — O9 的 TCC 测试落在 `forbidden_paths`（`tcc_*.go`）内，测试矩阵与 allowed_paths 自相矛盾

**证据**

- Addendum `:17`（allowed_paths）只列 `internal/services/order/refund_admission_integration_test.go` 及“同包必要的 `_test.go`，须在实施 Brief 中逐文件列明”；`:18`（forbidden_paths）明确 `internal/services/order/tcc_*.go`。
- `:328` O9 要求“TCC `source=tcc` + `NO_SHIPMENT`”的 I/R 集成测试。TCC 测试的自然文件是 `internal/services/order/tcc_admission_integration_test.go`（已有 T1.4a 用例，`:144-203`），该路径命中 `tcc_*.go`，被 forbidden。
- 结论：Developer 要么违反 forbidden，要么把 TCC 测试塞进 `refund_admission_integration_test.go`；后者可行但需要在 Brief 明确，不能靠“同包必要 _test.go”自动放行（该句与 `forbidden_paths` 冲突）。

**具体修改**

- 在 `allowed_paths` 明确加 `internal/services/order/tcc_admission_integration_test.go`（**仅测试**；禁止生产 `tcc_*.go` 的状态判断），或把 O9 明确放到 `refund_admission_integration_test.go` 并说明复用 TCC service helper。
- 在 `forbidden_paths` 加例外说明“`tcc_*_test.go` 允许新增测试，但禁止在 `tcc_*.go` 复制 `CANCELLED→PRE` 映射”；否则 R-4 与 O9 无法同时满足。

### `[MAJOR]` M-7 — TCC 冲突的人工队列发现口径缺失；`last_error` 只覆盖 AfterSales

**证据**

- Addendum `:291-292`（§8）：ops 用 Order `s.log.Error` 固定键 + `last_error LIKE '%REFUND_ADMISSION_CONFLICT%'` 发现队列。
- AfterSales `workers.go:724-748` 的 `releaseRefundTask` 确实写 `after_sale_refund_tasks.last_error`；但 TCC 冲突不经过 AfterSales。`tcc_admission.go:97-115` 只 `s.log.Error` 并返回 `Unavailable`，没有写 `refund_batches.last_error`（`migrations/order/0017_refund_batch_ledger.sql:28` 有该列）；`tcc_reconcile.go:340` 反而清空 `last_error`。
- 结果：TCC `NO_SHIPMENT` conflict 的 batch 停在 `ADMITTING`，application 停在 `PENDING`；ops 按 Addendum 的 `last_error` 口径可能找不到。

**具体修改**

- §8 增加 TCC 发现口径：`SELECT batch_id, order_no, payment_no FROM refund_batches WHERE state='ADMITTING'` + `refund_applications.state='PENDING'` + 对应 `s.log.Error` 键；或让 `admitTccBatch` 在失败时写 `refund_batches.last_error`（列已存在，无需迁移；但属于 `tcc_*.go`，须另立 allowed_paths/change_id）。
- 明确 TCC 冲突的 owner/expiry 与 AC-27-NOWB-01/AC-27-LIVE-01 的映射；不得只写 AfterSales `last_error`。

### `[MAJOR]` M-8 — Addendum §7.2 缺“消息重复/乱序”窗口与测试；W5/O11 只覆盖 NO_SHIPMENT

**证据**

- Addendum §7.2 `:261-270` 只有 W1–W8；没有“`order.refund_admitted` 重复/乱序”行，§10 `:312-332` 也没有 consumer 重复/乱序用例。
- 现状：Fulfillment consumer `admission_consumer.go:40-55` 只对 `result=PRE_SHIP_REVOKED` 调 `CancelIfCreated`，对 `NO_SHIPMENT` 走 `return nil`（无日志/无断言）；`order.paid` consumer 的 `autoShipAllowed`（`fulfillment/consumer.go:69-75,113-125`）能避免 PRE 后新建 placeholder；`refund_consumer.go:35` 的 `CancelCreatedShipment` 对无行 no-op。
- 结果：现有机制**可能**收敛，但 Addendum 没有把它写成规范/测试；若 consumer 的未来改动把 `NO_SHIPMENT` 当错误，会与 Addendum 的“additive”承诺冲突。

**具体修改**

- 增加 W9：“`order.refund_admitted` 重复/乱序（PRE 后重投、Tx1 的 POST/PENDING 占位、`order.paid` 迟到）”→ Fulfillment consumer 只处理 terminal PRE，重复 `CancelIfCreated` 幂等；`NO_SHIPMENT` 为 no-op；`order.paid` 迟到不创建 placeholder；owner = Fulfillment。
- §10 增加 `F7`/`O14`：捕获 `NO_SHIPMENT` 的 consumer 无错误/no-op；重复 PRE 不重复写运单；乱序 `order.paid` 不产生新行。
- 明确 `NO_SHIPMENT` 在 consumer 中是“无行，幂等成功”，不是 `TRANSIENT_ERROR`。

### `[MAJOR]` M-9 — 并发同 `refund_id` 的 Tx2 无 `state/version` 守卫，可能重复 outbox/释放

**证据**

- `refund_admission.go:566-608`：`finishRemoteAdmission` 在探测后开 Tx2，`:590-594` 执行 `UPDATE order_refund_admissions SET result=?, state=?, version=version+1 WHERE refund_id=?`，**没有** `AND state='PENDING_REMOTE'` 或 version 条件；`:595` 写 outbox；`:598-602` 写 release。
- `AdmitRefund:199-211` 的两个并发调用：一个赢 Tx1 后调 `finishRemoteAdmission`；败者在 order lock 释放后也读到 `PENDING`，同样调 `finishRemoteAdmission`。两者都在事务外探测，然后各自 Tx2；结果相同但会重复 `order.refund_admitted` 和 release 事件（`refund_release.go:187-193` 的 `hasOrderReleaseEventTx` 只在事务内查，且没有唯一约束兜底）。
- Addendum `:267`（W5）只说“订单行锁串行化；败者重读已持久化行；可能重复探测，但结果相同；admission 至多一行”；未提重复事件/释放；O11 `:330` 只测 `NO_SHIPMENT`（无 Tx2），因此测不到该风险。

**具体修改**

- Tx2 `UPDATE` 加 `WHERE refund_id=? AND state='PENDING_REMOTE'` 并检查 `RowsAffected()==1`；0 行则读回并返回当前结果（或 `Conflict` 让调用方重读），确保只一个执行者写 terminal outbox/release。
- 或在 `finishRemoteAdmission` 开头 `SELECT order_no FOR UPDATE` 串行化 Tx2（注意锁顺序与死锁）。
- 增加 O11b：`CONSUMED+CANCELLED` 同 `refund_id` 并发 → admission 1 行、terminal outbox 1 条、release 至多 1 条；O11 保持 `NO_SHIPMENT` 的“无写入”。

### `[MAJOR]` M-10 — POST 的 `RESERVED commitment`“与 POST admission 同事务”对正金额路径不成立

**证据**

- Addendum `:149`（`ALREADY_SHIPPED/RECEIVED` 行）：“Tx2 写 `result=POST,state=POST`；**POST 的 `RESERVED` commitment 与 POST admission 同事务**（T1.4a/R-B，现状）”；O5 `:324` 同样写“`RESERVED` commitment 与 POST admission 同事务”。
- 代码：正金额 `AdmitRefund` 的 Tx1 已在 `:266-272` 写 `order_refund_commitments(RESERVED)`；`finishRemoteAdmission` 的 Tx2 `:585-607` 只更新 admission/写 outbox，**不写 commitment**。只有 TCC 的 `recordTccPostAdmission`（`tcc_admission.go:280-307`）在 POST admission 同事务写 commitment。
- 结果：O5 若按 Addendum 断言“同事务”，对正金额路径无法通过；正确事实是“Tx1 已有恰好 1 条 RESERVED commitment，Tx2 不重复写”。

**具体修改**

- 把 §3.1 的该句分为两行：正金额/TCC probe 路径“Tx1 已写 RESERVED commitment；Tx2 只更新 admission/outbox，不重复写”；TCC 已离仓（Tx1 未写 commitment）由 `recordTccPostAdmission` 在同一事务写 POST admission + commitment。
- O5 改为“恰好 1 条 RESERVED commitment；admission POST/POST；无重复 commitment；不取消运单”；如要测 TCC 的 same-tx，另加 O5b 用 `recordTccPostAdmission`。

### `[MAJOR]` M-11 — `admission_consumer` 对 `NO_SHIPMENT` 的映射未在 Addendum 明确/测试

**证据**

- Addendum `:48` 事实 #10 已知 `internal/services/fulfillment/admission_consumer.go:43` 是另一个 `CancelIfCreated` 调用方；但 §2.2 的返回矩阵只写 Fulfillment 侧，§3.1 只写 Order 侧，§10 没有 consumer 用例。
- `admission_consumer.go:49-55` 只判断 `ALREADY_SHIPPED/RECEIVED`，其余（包括 `NO_SHIPMENT`）返回 nil；即 `NO_SHIPMENT` 是 no-op success。这个行为符合 AC-50 “无运单/已取消时幂等成功”，但 Addendum 没有把它写成契约/测试，未来 reviewer 或实现可能把 `NO_SHIPMENT` 当异常。
- 对 `NONE→REVOKED` 的合法无行 PRE，no-op 正确；对 `CONSUMED+CANCELLED` 的 PRE，若 Fulfillment 返回 `NO_SHIPMENT` 则意味着行消失，属于不一致，但 consumer 无法区分来源。至少应明确“no-op 是当前唯一安全选择”。

**具体修改**

- 在 §2.2 加一行：“`order.refund_admitted(PRE)` 的 consumer 对 `NO_SHIPMENT` 视为**幂等成功**（无行可取消），不报错、不取消；只有 `ALREADY_SHIPPED/RECEIVED` 告警。”
- §10 加 F7/O14 测试：consumer 收到 `NO_SHIPMENT` 返回 nil、无 outbox/写；重复 PRE 不重复取消。

### `[MAJOR]` M-12 — `CONSUMED+无行` 的可达性未证明；O1 的 fixture 是人工合成状态

**证据**

- 正常 Fulfillment 流程：`CreateShipment` 先插入 `shipments` 行（`fulfillment/service.go:154-166`），`Ship` 再调用 `Order.AuthorizeShipment` 消费 permit（`fulfillment/service.go:244`）；`AuthorizeShipment` 在 `settlement.go:353-357` 把 permit 置 `CONSUMED`。因此**正常路径下 `CONSUMED` 时运单行已存在**，`CONSUMED+无行` 只可能来自：直接调用 `Order.AuthorizeShipment`（Order RPC，`mall.proto:485-487`）、历史无 permit 数据、人工/数据修复、行被删除。
- Addendum `:33-50` 的事实表没有说明这一可达性；O1/O7 直接预置 `CONSUMED` + 无行（`:320`、`:326`），属于合成异常状态，未按宪法 IV.3 说明“真实业务边界”。
- 这不否定 fail-closed 修复的价值（防御性），但影响“该缺陷的实际发生率/优先级”和测试真实性；也可能让 Reviewer 追问“为什么不在 `AuthorizeShipment` 侧校验”。

**具体修改**

- 在 §0/§12 增加可达性表：正常 `Ship`（行先存在，不产生 `CONSUMED+无行`）；直接 `AuthorizeShipment` / 历史数据 / 人工修复（可能）；`[不确定]` 生产 `shipments` 是否有被删除/未应用 `0003` 的情况，需 ops 证据。
- O1/O7 明确“合成异常状态，用于验证 fail-closed”；并增加一条正常路径回归（`CONSUMED+CREATED` → 取消 → PRE；`CONSUMED+SHIPPED` → POST），避免把合成状态当成唯一测试。
- 若可达性无法证明，仍可修（防御性），但不要把它写成“已发生的 P0 缺陷”而无证据；owner/expiry 由 Lead 登记。

### `[MAJOR]` M-13 — 零元/TCC 的“Payment 调用 0”和 O6 “无写入”未区分 setup（与 B-2 同源，单列以防漏测）

**证据**：同上 B-2；零元 `admitZeroRefund` 在 `:154-156` 直接短路，确实不调 Payment；正金额/TCC 在 `:168` 调 `GetIntent`。O6 `:325` 没有说明是 fresh 还是预置 `PENDING_REMOTE`；fresh 会写 Tx1，预置不会。O9 `:328` 的 TCC 也走 `GetIntent`。

**具体修改**：测试矩阵每一行加“setup：fresh / 预置 PENDING / 预置 DONE”列；O6 明确用预置 `PENDING_REMOTE` 证明“无新写入”，另加 O6b fresh 证明“Tx1 写入 + conflict”；O7 保持零元“完全无 Payment”（`payments.refundCalls==0 && getCalls==0`）。这是 B-2 的测试落地细化，Reviewer 必须核对。

---

## 4. `[TODO]`

- **T-1** `docs/specs/0002:517` 的错误表把 `REFUND_ADMISSION_CONFLICT` 描述为“超出剩余额度/已全额退款后的退款申请”，而 AC-50 `0002:223` 把它用于 `CONSUMED+无运单`，ADR §10 `0007:988` 描述更宽（“额度/状态冲突；含 Fulfillment 事实不可用”）。Addendum `:303` 说不改错误码表是对的，但应登记为 SPEC-0002 下次升版的文档同步（owner PO，expiry 由 Lead）；否则 `docs/contracts.md`/错误表与 AC-50 语义漂移。
- **T-2** Proto 注释同步路径（`mall.proto:1096`、`api/gen/mall/v1/mall.pb.go:11203`）可执行，但 `make proto` 未固定 `protoc`/`protoc-gen-go` 版本（`Makefile:152-157`；生成头 `mall.pb.go:1-5` 为 v1.36.12/protoc v7.36.2）。实施 Brief 必须记录版本，并要求 `git diff -- api/gen` 只出现 `CancelIfCreatedReply` 注释行；`mall_grpc.pb.go` 不得出现非注释 diff；禁止手改（宪法 III.4）。
- **T-3** O4 `:323` “释放按 INV-44 触发”对多组订单不成立：`refund_release.go:178-187` 在 `groupCount != 1` 时只 `log.Warn` 并返回 nil，不产生 release。O4 必须限定单组；多组按 `INV-44-MULTI-01` `[DEFERRED]`，不得用 Order fixture `RELEASED` 冒充。
- **T-4** 零元 `TRANSIENT_ERROR`/空/未知未在 §10 单列；`decideZeroRefundResult` 的 `default` 是 `apperr.Unavailable`（`refund_admission.go:384-391`），应加 O7b：零元 `CONSUMED+TRANSIENT_ERROR` → `Unavailable`，不写 admission/DONE、不释放、不调 Payment。
- **T-5** `NO_SHIPMENT` 在 `shipment_no` 非空时的语义只在 §2.1/§12.3 说明；当前 Order 调用点都传空，但 `admission_consumer`/外部调用方可能传 `shipment_no`。补一个 F1b（`shipment_no` 非空、无行）或明确禁止外部传非空；同时按 §12.5 确认仓库外无调用方（Lead/ops 发布门禁）。
- **T-6** `NO_SHIPMENT` 的命名（§12.1）和 `REFUND_ADMISSION_CONFLICT` details 的 `reason/manual_queue` 字符串不是稳定契约；若 ops 查询依赖它们，应在 §8 固定 key/大小写/版本，避免改名导致队列发现失效。
- **T-7** `AdmitRefund` 的 `finishRemoteAdmission` 目前没有 `order` 行锁（`:566-607`）；M-9 的修复可选“Tx2 状态 CAS”或“行锁”。若选行锁，需在 Addendum §7.1/§7.2 写明锁顺序（order → admission）与死锁回退，否则 Reviewer 无法核对。

---

## 5. 已验证成立的部分（正面结论）

1. **R-1 的 additive 形态成立**：`CancelIfCreatedReply.status` 是 `string`（`mall.proto:1095-1098`；生成物 `mall.pb.go:11198-11204`），新增值不需要新字段/RPC/枚举；`scripts/check-proto-evolution.py:1-80` 只比较字段号/RPC 签名/保留号，注释变更不触发 baseline 更新；旧 Order 的 `default` 分支确实 `apperr.Unavailable`（`refund_admission.go:582-584`、`:389-391`），不会写 PRE。
2. **Fulfillment 的二分方向正确**：`sql.ErrNoRows` 在 `cancel_if_created.go:51-55` 是唯一“无行”点；无行时不建行、不 commit（defer rollback），`FOR UPDATE` 锁住已存在行；`migrations/fulfillment/0003_group_shipments.sql:6` 的 `(order_no, fulfillment_group_no)` 唯一键保证每组至多一行，F6 的组隔离有意义；`NO_SHIPMENT` 与 `TRANSIENT_ERROR`/`CANCELLED` 语义可区分。
3. **Order 的映射点确实只有两个**：`refund_admission.go:384-391`（零元）与 `:577-584`（正金额/TCC/legacy）是仅有的 `CancelIfCreatedReply.status` 分支；`tcc_admission.go:152-169` 经 `AdmitRefund`，没有第二套 `CANCELLED→PRE`；`tcc_admission.go:144-150` 的 POST 由 Tx1 Fulfillment 快照决定，不读 `CancelIfCreated`。R-4 的目标正确。
4. **零元路径的顺序正确**：`admitZeroRefund:344-357` 先 `decideZeroRefundResult` 再 `writeZeroRefundTx`；`decideZeroRefundResult:366-396` 在 `CONSUMED` 时调 `CancelIfCreated`；`NO_SHIPMENT` 可在 `writeZeroRefundTx`（`:401` 起）之前返回，不写 admission/`refund_applications(DONE)`；已有 `DONE` 的重复 `refund_id` 在 `:335-339` 先返回 `zeroRefundReply`，符合零元幂等。
5. **NO_SHIPMENT 的 fail-closed 方向正确**：conflict 不写 PRE/POST、不写 release、不改 permit；`CONSUMED` 保持 `CONSUMED`（INV-45）；`PENDING_REMOTE` 不是结果（A2 方向正确）。只要 B-1/B-2 的文本/测试修正，核心映射可以落地。
6. **未越权改 SPEC/Rev 2.3**：Addendum §0.3/§13 `:56-60`、`:371-379` 明确不改 SPEC-0001/0002、不改 accepted Rev 2.3 正文，未来 Rev 2.4 errata 由 Architect 合并；本文件确实是 additive。§0.2 对 `-amd1.md` §5.3 与合并版 §5.3 的指针澄清正确（`-amd1.md:375-397` 是 legacy resolution；merged `0007:611-623` 才是 `CancelIfCreated`）。
7. **AC-27-NOWB-01 的 owner/expiry 文本一致**：Addendum §8 `:290` 的 `Developer（Order 数据核对）/ops（重放、跨服务数据）/Fulfillment（S6 证据）`、`2026-12-31` 与 SPEC-0001 `0001:863`/`:868` 一致；终态①/②的措辞与 SPEC 基本一致（除 B-3 的“补出运单行”误读）。
8. **AC-27A 文本未被改写**：Addendum §6.1 `:223-228` 只做“前提保证”，不修改 AC-27A；`PENDING_REMOTE` 不关闭的方向与 `0001:622-625/645` 一致。

---

## 6. 测试矩阵核对（§10）

| 风险场景 | Addendum 覆盖 | 结论 / 缺口 |
|---|---|---|
| Fulfillment 无行 | F1 `:314` | 覆盖；需补 `shipment_no` 非空（T-5）。 |
| Fulfillment 真实 `CREATED`/`CANCELLED` | F2/F3 `:315-316` | 覆盖；现有 `cancel_if_created_test.go:12-58` 可扩展。 |
| Fulfillment `SHIPPED/RECEIVED/EXCEPTION` | F4/F5 `:317-318` | 覆盖；现有 `:65-128`。 |
| Fulfillment 组隔离 | F6 `:319` | 覆盖。 |
| 正金额 `CONSUMED+NO_SHIPMENT` | O1 `:320` | **按现文本不可实现**（B-1/B-2）：outbox 已存在、GetIntent 已调用；需改断言/setup。 |
| 同 `refund_id` 重放 | O2 `:321` | 覆盖；需明确 O1 的 placeholder outbox 在重放中不新增。 |
| 预置 `PENDING_REMOTE` 崩溃 | O3 `:322` | 覆盖；应同时预置 `RESERVED commitment` 与 Tx1 占位 outbox，才真实模拟 W1/W2。 |
| 正金额 `CANCELLED`/已发 | O4/O5 `:323-324` | O4 未限定金额（M-2）与单组（T-3）；O5 的“同事务”错误（M-10）。 |
| `TRANSIENT_ERROR`/空/未知 | O6 `:325` | 只覆盖正金额；需区分 fresh/预置（B-2/M-13）；零元缺（T-4）。 |
| 零元 `NO_SHIPMENT`/恢复 | O7/O8 `:326-327` | 覆盖；需补零元 transient（T-4）。 |
| TCC | O9 `:328` | 方向覆盖；测试路径被 forbidden（M-6）；“无 Payment”错误（B-2）；queue 发现缺口（M-7）。 |
| legacy | O10 `:329` | 覆盖“legacy 无许可行→PRE”，但缺“legacy 已发货无许可行”反例（M-1）。 |
| 同 `refund_id` 并发 | O11 `:330` | 只覆盖 NO_SHIPMENT；缺 `CANCELLED` 并发 Tx2（M-8/M-9）。 |
| 告警键 | O12 `:331` | 覆盖；需把 TCC 冲突的日志键也纳入（M-7）。 |
| AC-27A 非回归 | O13 `:332` | 覆盖“不变”；需加 TCC `result=PENDING_REMOTE` 用例（M-4）。 |
| 消息重复/乱序 | **无** | 缺 W9/consumer 测试（M-8/M-11）。 |
| 部分金额 | **无** | 缺 O4b/O1b（M-2）。 |
| 存量错误 PRE 审计 | **无** | 缺 §8 行/ops 查询（M-5）。 |

**Mock 真实性**：O1/O7 的零元 fixture 必须 `pay_cent=0` 且 `settlement_payment_no='zero:<order_no>'`；正金额用例必须 `>0`（`0002` 零元边界/宪法 IV.3）；`NO_SHIPMENT` fixture 必须预置 permit `CONSUMED` 和真实 `commitment`/占位 outbox（或明确合成状态，M-12）；不得用 `t.Skip()` 或伪造 `CANCELLED` 行绕过 F1/O1（Addendum `:338` 已禁止）。

---

## 7. 宪法 / 治理逐条核对（本范围）

| 条款 | 结论 | 依据 |
|---|---|---|
| Article I.1 拒绝“屎山雕花” | **部分 PASS** | R-4 单一映射方向正确；但 B-1/B-2 的文本把 Tx1 写路径“写成没有”，会造成实现者用 if/特判掩盖；M-2/M-4 需在映射表/测试中穷尽，而不是新增分支。 |
| I.2 Fail-Closed | **部分 PASS** | `NO_SHIPMENT→conflict`、`TRANSIENT/unknown→Unavailable` 方向正确；M-1/M-2 的 legacy/partial 仍可能写 PRE；B-3 的“补行”会放行伪造 PRE。 |
| I.3 契约优先 / 静态守卫 | **PASS（方向）** | 只改 proto 注释，不新增字段/RPC；`api/gen` 由 `make proto` 生成；`docs/contracts.md` 无需改。T-2 要求固定工具版本与 diff 证据。 |
| II.1 事实分离 | **部分 PASS** | `NO_SHIPMENT` 把 Fulfillment 事实分开；A2 不把 `PENDING_REMOTE` 当结果；但 TCC `result=PENDING_REMOTE`（M-4）仍混用 result/state 列。 |
| II.3 幂等/最终一致 | **部分 PASS** | 同 `refund_id` 重探/同键幂等方向正确；M-9 的 Tx2 无状态守卫会重复 outbox/释放；W5 需补 CANCELLED 并发。 |
| II.5 退款/履约准入 | **部分 PASS** | conflict 不写 permit/不恢复 REVOKED；A5 正确；M-1/M-2/M-3 仍涉及 PRE 合法性与释放纪律。 |
| III.1 DTM/Outbox | **FAIL（文本）** | B-1：Tx1/outbox 事实与 Addendum “不写 outbox”矛盾；必须二选一（占位事件命名/移到 Tx2）。 |
| III.4 静态守卫/生成物 | **PASS（方向）** | 不手改 `api/gen`；`make proto` 同步；T-2 要求版本/仅注释 diff。 |
| IV.1 证据不可妥协 | **FAIL（测试矩阵）** | O1/O6/O9 按现文本无法通过；B-1/B-2；未执行任何测试（本报告只读）。 |
| IV.2 重灾区抗压 | **部分** | O9/O11 有 TCC/并发意图；缺 `CANCELLED` 并发 Tx2（M-9）、重复/乱序（M-8）、零元 transient（T-4）。 |
| IV.3 Mock 真实 | **部分** | F1/O1 的零元/正金额规则有；M-12 的 `CONSUMED+无行` 合成状态需说明；O3 需预置 commitment/占位 outbox。 |
| V AI 安全 | **N/A** | 不涉及 AI。 |
| Workflow 8 / Plan 门禁 | **未满足** | 本 Addendum 是 DRAFT；实施须 Lead 另拆 `change_id` + Plan/Tasks + AC 证据矩阵；本报告不代行。 |
| Governance 4 人类检查点 | **保留** | ADR_APPROVED 仍需人类；本报告不批准 merge/Release。 |
| Governance 10 Actor 独立 | **PASS** | `qc-ac50` ≠ `arch-ac50`；只写 QC 报告。 |
| 范围守卫 | **未越过** | 未改 ADR/SPEC/代码/台账/迁移；只新建本报告；未 commit。 |

**`[ARCHITECTURAL_FLAW]`**：本范围**未**新增代码架构缺陷；B-1/B-2/B-3 是 Addendum/测试矩阵/人工处置的契约缺陷，应回 Architect 在本 DRAFT 修正；M-1/M-2/M-4 是既有 SPEC/代码的前置缺口，应在 R0.3-R1c/独立 L2 登记，不得由 Developer 在本 change_id 内用局部 `if` 掩盖。

---

## 8. 边界与后续

- **只读声明**：未执行 `go test`/`-race`/真实 MySQL/迁移；文中代码行号是阅读事实，不构成执行证据；未改 ADR/SPEC/代码/测试/迁移/台账；只新建本文件；未 commit。
- **不审**：R0.4 的 `0022/0023` 迁移、`r04_checks_integration_test.go`、`scripts/verify-r01-migrations.sh`、`docs/ops.md`、`docs/review-tracking.md` 及其并发改动；这些不属于 AC-50-FIX-01。
- **下游**：本 DRAFT 回 Architect 处理 B-1/B-2/B-3 与 M 项 → QC 复核 → 人类 `ADR_APPROVED` → Lead 另拆独立实施 `change_id` + Plan/Tasks/AC 证据矩阵 → Developer 实现 → Reviewer 按精确 base/head 复核。任何“AC-27 已收敛/AC-VERIFIED/Release PASS”均不可由本报告或本 DRAFT 推出。
- **AC 状态**：AC-50 仍 **AC-REF**（不是 AC-VERIFIED）；AC-27A 第 5/6 条未实现；AC-27 仍 AC-REF；`AC-50-FIX-01`/`AC-27-NOWB-01`/`AC-27-LIVE-01` 仍 open，expiry 2026-12-31。

**Actor**：`qc-ac50`（Quality Checker）
**Reviewed-by**：待 Architect 修订 / 人类批准
**Status**：**BLOCKED — DRAFT 不得进入实现**

---

# R2 复核（DRAFT 0.2 delta）

| 项 | 值 |
|---|---|
| 审查对象 | `docs/adr/0007-amd1-addendum-ac50-nowaybill.md` **DRAFT 0.2**（工作树，未 commit；615 行） |
| 结论 | **BLOCKED**（2 个 `[BLOCKER]` + 5 个 `[MAJOR]` + 4 个 `[TODO]`；0.2 不得进入实现） |
| 日期 | **2026-11-26** |
| Actor | `qc-ac50`（Quality Checker；与 `arch-ac50` 不同 actor） |
| base | `da0cdd15d065a503ffa1e6d545e7381f6766737d` |
| head | `aec9ac86b68c885bfea0de18dc2be4518e075042`（`git rev-parse HEAD`；被审文件为工作树未跟踪文件，blob `f8b9e67b258b4b6a34b9ed40d375c0b689aaeec2`，sha256 `57aea961b21fab1219b5d9ecd959ab2a6eb359e8cc6679fe0508b4c0432308d4`）。**注**：QC 读取 0.2 时 HEAD 为 `da0cdd1`；审查期间 Lead 提交 `aec9ac8`（仅 R0.4 branch A：`0023`/`0022`/`r04_checks_integration_test.go`/`scripts`/`docs/ops.md`），`git diff da0cdd1..aec9ac8 -- internal/services/order/refund_admission.go internal/services/fulfillment/cancel_if_created.go api/proto/mall/v1/mall.proto` 为空，ADR blob 未变，故本 R2 结论不受 head 移动影响。 |
| review_scope | **只审 0.2 的 delta**：B-1/B-2/B-3 的处置、M-1…M-13 的关闭/延期、R-2a、A7/A8、§7.4/§7.4.1、W9、§8 人工队列、§10 setup、§12/§15 停止条件；不重审 0.1 已通过部分、不审代码/测试执行。 |
| release_status | **NOT RELEASE**；不批准 merge/Release/AC-VERIFIED；0.2 仍为 DRAFT。 |

## R2.0 结论

**BLOCKED。** 0.2 已实质修正 B-1（A7/A8/§7.4 承认 Tx1 写入并规定保留 admission/commitment/占位 outbox、0 终态/0 释放、人工队列）与 B-2（`GetIntent` 只读允许，O1/O6/O7/O9 改为 `refundCalls==0`），这 2 项在本报告范围内**可以判 CLOSED**；B-3 的人工 W6 文本已修正，但正常 consumer 侧仍见 R2-M1，故只能判 **PARTIAL**。但 **R-2a 的部分金额前置门禁引入了两个新的阻塞问题**：

1. **R2-B1**：R-2a 的门禁被放在“读 admission”之前（§7.1 `:323-326`；§15.1 `:568`）。对同 `refund_id` 的已持久化部分 POST 重放，门禁会在读取持久化结果前调用 `ListShipments`；该只读调用一旦超时/不可用（或异常数据使其返回未离仓），就会返回 `Unavailable`/`PARTIAL`，而不是 AC-50 `0002:227` 要求的“读取已持久化结果并返回同一结果”。这是**同 `refund_id` 幂等回归**，必须把门禁移到 existing admission 读取之后，并且只对 fresh 请求执行；Tx1 锁内重读仍要先收敛 existing。
2. **R2-B2**：R-2a 在 `CONSUMED` 分支 Tx1 前新增 `ListShipments` 预读，和 accepted ADR Rev 2.3 §5.2.2 的“`CONSUMED` … 提交后调用 `CancelIfCreated`（**不依赖事务外预读运单状态**）”（`docs/adr/0007-payment-settlement-admission.md:573`）冲突。Addendum §3.4.7 `:231` 说“仍成立（预读不作为 PRE/POST 依据）”，但门禁确实用预读决定“拒绝/继续”；这不是纯 §5.3 additive，必须显式修改 §5.2.2（人类批准）或另立 L2，不能由 Addendum 自行解释。

此外，B-3 的 W6 文本禁止**人工/ops** 造行，但**没有阻止正常 `order.paid` consumer** 在迟到时创建 placeholder；W9 `:359` 声称不会创建，按 `autoShipAllowed` 的 fallback（`fulfillment/consumer.go:113-125`；`settlement.go:216-230`）并不成立。这是 B-3 的**边界回落**（见 R2-M1），不是 0.2 可以 CLOSED 的完整结论。

## R2.1 3 个 BLOCKER 的关闭情况

| QC 项 | 0.2 处置 | R2 判定 | 依据/缺口 |
|---|---|---|---|
| **B-1** Tx1 已写 admission/commitment/outbox | A7/A8、§7.4 处置协议、O1/O3/O9 断言 | **CLOSED（文本/测试口径）** | `:296-297`、`:369-389`、`:452-454`；承认 Tx1 写入、commitment 保持 `RESERVED`、占位 outbox 保留、0 终态/0 释放、人工队列。与 INV-45/AC-62 一致（A4 已加人工授权例外 `:293`、`§8:414`）。**但 A8 的 0 行读回语义未定义（R2-M5）**。 |
| **B-2** `GetIntent` / “Payment 0” | §3.1/§4.1/A4/O1/O6/O7/O9 改为 `Refund/RefundBatch` 0，`GetIntent` 允许 | **CLOSED** | `:199`、`:293`、`:452/460/462/463/465`；`refundCalls==0` 与 `GetIntent` 只读不矛盾。O7 零元 `getCalls==0` 仍然字面成立（`:265`）。 |
| **B-3** W6“补出运单行” | W6 改为“既有或曾存在但丢失”的真实 S6 证据，禁止为从未存在造行 | **PARTIAL**（人工侧 CLOSED；正常 consumer 侧未关闭） | `:356`、`:171`、`:413`；W6 文本已修。**但** W9 `:359` 的“`order.paid` 迟到不建 placeholder”在 G1 + `CONSUMED` + `refund_state=NONE` 下可被 `autoShipAllowed` 的 `availableActions` fallback 推翻（`fulfillment/consumer.go:113-125`；`settlement.go:216-230`）：迟到 `order.paid` 仍可创建 `CREATED` placeholder，重放 `CancelIfCreated` 得到 `CANCELLED→PRE`。这不算“从未存在的运单”吗？见 R2-M1。 |

## R2.2 13 MAJOR 的关闭/延期与 owner/expiry/停止条件

> 结论：**M-2 未关闭**（新 R2-B1/R2-B2）；**M-1/M-5 部分关闭**（审计查询缺口）；其余 M-3/M-4/M-6/M-7/M-8/M-9/M-10/M-11/M-12/M-13 在 0.2 文本层面可接受为 CLOSED 或“规则 CLOSED + 实现 DEFERRED”。**“停止条件”只有全局 §15.2，不是每条 DEFERRED 都有 per-item exit**（见 R2-M3）。

| QC 项 | 0.2 处置 | R2 判定 | owner / expiry / 停止条件 |
|---|---|---|---|
| **M-1** legacy `NONE`+已发货 PRE | A3 收窄；`LEGACY-SHIPPED-PRE-01`；§8 审计；O10b DEFERRED | **PARTIAL（可接受 DEFERRED）** | Architect/PO；2026-12-31；`§12.10:514`、`§15.2:579`（通用）。**审计查询漏“无 `order_fulfillment_groups` 行”的 legacy G1**（R2-M2）。 |
| **M-2** 部分金额 PARTIAL 前置 | R-2a 前置门禁；O4b/O4c；§15 | **NOT CLOSED** | Developer（实现）；停止条件 `§15.2 #3:578` 已命中（见 R2-B2）。新增 **R2-B1** 幂等回归。 |
| **M-3** A4 与 AC-62 | A4 加人工授权例外；O15 DEFERRED | **CLOSED（规则）/实现 DEFERRED** | Developer/T3；2026-12-31；`O15:474`“T3 未实现/不得冒充”。停止条件可读为 O15 + `§15.2 #2`。 |
| **M-4** TCC `result=PENDING_REMOTE` | A2 双条件；`TCC-PENDING-RESULT-01`；O13b | **CLOSED（读者口径）/清理 DEFERRED** | Architect/Developer；2026-12-31；`§15.2 #5:580`。 |
| **M-5** 存量 PRE 审计 | §8 查询 + AC-27A 门禁 | **PARTIAL** | ops/Developer；2026-12-31；审计查询本身漏无 group 行（R2-M2）；停止条件 = 审计结论/`§15.2 #4`。 |
| **M-6** TCC 测试路径 | allowed_paths 加 `tcc_admission_integration_test.go`；生产 `tcc_*.go` 仍 forbidden | **CLOSED** | Developer；N/A（实施 Brief 列文件）。 |
| **M-7** TCC 队列发现 | §8 TCC 查询；`TCC-LAST-ERROR-01` DEFERRED | **CLOSED（查询）/last_error DEFERRED** | Developer/Architect；2026-12-31；`§15.2 #2:577`。 |
| **M-8** 消息重复/乱序 | W9、F7/O14 | **CLOSED（测试口径）** | Fulfillment consumer；N/A；但 W9 的 `order.paid` 断言有 R2-M1 反例。 |
| **M-9** Tx2 无状态守卫 | A8、O11b | **CLOSED（规则）/实现待测** | Developer；N/A；A8 0 行读回语义见 R2-M5。 |
| **M-10** POST commitment“同事务” | §3.1 分行；O5/O5b | **CLOSED** | Developer；N/A；O5/O5b 已区分正金额 Tx1 与 TCC `recordTccPostAdmission`。 |
| **M-11** consumer `NO_SHIPMENT` | §2.2、F7/O14 | **CLOSED** | Fulfillment consumer；N/A；`admission_consumer.go` 生产语义不变。 |
| **M-12** `CONSUMED+无行` 可达性 | §0.2、F8、O1/O7 合成说明 | **CLOSED（说明）** | Lead 登记 ops 证据；2026-12-31（`§12.9:513`）；无独立 stop，但 `§15.2 #7` 覆盖证据不成立即停。 |
| **M-13** 零元/TCC “Payment 0”/setup | setup 列；O6/O6b/O7 | **CLOSED** | Developer；N/A；O6b preseed、O7 零元分离。 |
| **T-1** 错误表漂移 | §9 `[TODO]` → PO | **TODO（接受）** | PO；2026-12-31；**无 per-item stop**（R2-M3）。 |
| **T-3** O4 多组释放 | O4 限定单组；`INV-44-MULTI-01` DEFERRED | **CLOSED（单组）/DEFERRED** | Developer/Lead；2026-12-31；`§14.1:542`；**无 per-item stop**（R2-M3）。 |

## R2.3 新增 `[BLOCKER]`

### `[BLOCKER]` R2-B1 — R-2a 门禁放在 existing admission 读取之前，破坏同 `refund_id` 幂等

**证据**

- Addendum §7.1 `:323-326`：时序先“部分金额门禁（R-2a）”，再“读 admission”；§15.1 `:568` 也说 `AdmitRefund` 在 Tx1 前调 `ListShipments`。
- 当前 `refund_admission.go:179-185` 先 `findRefundAdmission`，有 terminal 结果直接返回；`:203-211` 在 Tx1 内锁重读并返回 existing。R-2a 若按 §7.1 放在这之前，**不会先返回已持久化结果**。
- 反例（正常数据、非数据修复）：组 `PAID/ALLOCATED`、`permit=CONSUMED`、已发货；`refund_id=R1` 部分 30 已 `POST`，`order_refund_commitments` 已有 30，组剩余 70。调用方重放 `R1` 30：R-2a 条件 `30 < 70` 成立；`ListShipments` **超时** → `Unavailable`（`:229`），或异常返回未离仓 → `PARTIAL`（`:227`）。调用方拿不到已持久化的 `POST_SHIP_ALLOWED`，违反 AC-50 `0002:227`“同 `refund_id` 先读已持久化结果并返回同一结果”。
- 并发变体：两个同 `refund_id` 请求同时进入；一个先提交 terminal POST，另一个在 gate 里因 `ListShipments` 不可用返回 `Unavailable`，同样不收敛到持久化结果。
- 这不是 B-1 的“无新增写入”问题，而是**R-2a 把只读依赖插到了幂等读取之前**；B-1 的 A7/A8 不能覆盖。

**具体修改**

1. 在 `AdmitRefund` 中**先读 existing admission**（保留 `:179-185`、`:203-211` 的顺序）；existing 为 `PENDING_REMOTE` 时走重新探测，terminal 时直接 `admissionReply`。**R-2a 只对“无 existing admission 的 fresh 请求”执行**。
2. 若 R-2a 结果要在 Tx1 内使用，仍应在 Tx1 锁内**再次检查 existing**；若锁内发现 existing（并发同 `refund_id` 已提交），返回 existing，不得返回 PARTIAL/Unavailable。
3. 增加 O2b/O2c：已持久化部分 POST + `ListShipments` 超时/不可用 → 必须返回持久化 POST（不重新准入、不返回 PARTIAL/Unavailable）；同 `refund_id` 并发 terminal + gate → 两路返回同一结果。
4. 在 §7.1 时序图中把 gate 移到“读 admission（existing 返回）”之后；§3.4 第 1 条补“仅 fresh”。

### `[BLOCKER]` R2-B2 — R-2a 的 `ListShipments` 预读改变 ADR Rev 2.3 §5.2.2 的“CONSUMED 不预读”语义

**证据**

- accepted ADR Rev 2.3 §5.2.2 `docs/adr/0007-payment-settlement-admission.md:573`：`CONSUMED` 分支“写 admission(state=PENDING_REMOTE)+commitment；提交后调用 `CancelIfCreated`（**不依赖事务外预读运单状态**）”；`:579-583` 的 Tx1 后远程调用同样以 `CancelIfCreated` 为准。
- Addendum §3.4.2 `:226` 在 **Tx1 前**调用只读 `Fulfillment.ListShipments`；§3.4.7 `:231` 声称“ADR §5.2.2‘CONSUMED 不依赖事务外预读’仍成立（预读不作为 PRE/POST 依据）”。但 R-2a 的预读**决定是否拒绝**（未离仓 → PARTIAL），已经影响准入结果；这不是“不依赖”。
- Addendum 自己在 §12.13 `:517` 与 §15.2 #3 `:578` 承认“若 QC 认为门禁改变 §5.2.2 语义，需回 Architect 裁决”。QC R2 认为**确实改变**：partial `CONSUMED` 的路径从“不预读、Tx1+探测”变成“先预读、未离仓直接拒绝”。这不是 §5.3 的纯 additive，必须显式修订 §5.2.2 或取得人类对 `CONSUMED` 预读的书面豁免；不能只以“预读只用于拒绝”解释为“仍成立”。

**具体修改（二选一）**

1. **显式修订**：将本 Addendum 的修改对象扩到 §5.2.2，写“仅对 `CONSUMED && amount < 组剩余` 的 partial 请求，Tx1 前允许一次只读 `ListShipments` 用于**拒绝**；该预读不得决定 PRE/POST；SUCCESS 后仍以 `CancelIfCreated` 为准”，并由人类按 L2 批准；更新 Rev 2.4 errata 计划。
2. **不在本 Addendum 做 R-2a**：保留 AC-50 的 partial 缺口为独立 L2（例如 `PARTIAL-CONSUMED-01`），在 `finishRemoteAdmission` 之后以显式 Tx2 拒绝（接受 Tx1 占位/commitment 的已知代价），或在 `AdmitRefund` 中先探测再决定；两者都要另做崩溃/幂等分析。

## R2.4 新增 `[MAJOR]`

### `[MAJOR]` R2-M1 — W9 的“`order.paid` 迟到不建 placeholder”不成立；B-3 的“从未存在不得建行”有正常 consumer 反例

**证据**

- W9 `:359`：“`order.paid` 迟到由 `autoShipAllowed`（`fulfillment/consumer.go:66-70,111-123`）不创建 placeholder”；W6 `:356`/§2.3 #1 `:171` 禁止为从未存在的运单新建行。
- `autoShipAllowed`（`fulfillment/consumer.go:113-125`）先看 `GroupActions`，再看 `AvailableActions`：对 `groupNo=="G1"` 且 action 为 `"ship"` 也返回 true。`settlement.go:216-230` 的 `availableActions` 在 `orders.status=PAID/ALLOCATED` 且 `refund_state != REFUNDING/FULL` 时返回 `"ship"`。
- `AdmitRefund` 的 `NO_SHIPMENT` conflict **不写** `orders.refund_state`（A6/`:295`），所以迟到 `order.paid` 仍可能通过 `autoShipAllowed` 为 G1 创建 `CREATED` placeholder（`fulfillment/consumer.go` 的 `CreateShipment`）。重放 `CancelIfCreated` 得 `CANCELLED→PRE`。这既不是 ops 伪造，也不是 W6 的“既有 S6 证据”，却会造出一个“从未存在”的运单行并把它取消成 PRE。
- 影响：B-3 的“不得为从未存在的运单建行”在**正常 consumer** 上没有闭环；W9 的断言也不准确。若认为这是合法恢复，必须显式写成终态①的例外（正常 `order.paid` placeholder 允许取消）；若不允许，必须在 `autoShipAllowed`/`order.paid`/`CreateShipment` 侧阻断——而 `fulfillment/consumer.go` 不在 allowed_paths，需另立 L2 或把该路径纳入。

**具体修改**

- 在 §7.2 W9/§8 终态① 明确区分：
  - **ops/人工**：只为既有/曾存在但丢失的真实 S6 证据恢复，禁止新建行（W6 保持）；
  - **正常 `order.paid` consumer**：若迟到创建的是真实 `CREATED` placeholder，允许 `CancelIfCreated` 取消→PRE；必须记录“placeholder 由正常 consumer 创建”而非“人工证据”，并补 F7/O14 的 G1 late `order.paid` 用例；
  - 或若业务不接受这种恢复，则把 `autoShipAllowed`/`order.paid` 的阻断列为独立 L2（owner Architect/Developer，expiry 2026-12-31），本 amendment 不得声称已禁止“从未存在”行。
- 在 B-3 的关闭表中把状态改为 **PARTIAL（人工侧 CLOSED，正常 consumer 侧未定义）**，不要写“完全禁止”。

### `[MAJOR]` R2-M2 — M-5 存量审计查询漏“无 `order_fulfillment_groups` 行”的 legacy G1

**证据**

- §8 审计 `:419`：`permit IN ('REVOKED','CONSUMED') AND group.status IN ('SHIPPED','RECEIVED') AND result='PRE'`；`group.status` 来自 `order_fulfillment_groups`。
- `ensurePermitTx`（`settlement.go:263-278`）对无 permit 的历史 G1 创建 `NONE`；`ensureGroupExistsTx`（`:235-248`）对 G1 **直接返回 nil**，不会创建 group 行。历史 G1 可能没有 `order_fulfillment_groups` 行；此时审计查询**一行都匹配不到**，但 `NONE→REVOKED` 已写 PRE（M-1 场景）。M-5 的“审计结论未出前不得关闭”因此无法覆盖该子集。
- 另外，查询要求 `state='PRE'`；若存量有 `result=PRE,state=PENDING` 的异常（TCC/旧数据），也会漏。M-5 只列了 `result/state=PRE`。

**具体修改**

- 审计查询改用 `LEFT JOIN order_fulfillment_groups`，并增加 `OR g.group_no IS NULL` / `group_no NOT FOUND` 分支；对 `result='PRE'` 且 `state IN ('PRE','PENDING_REMOTE')` 分别列出（后者按“不可核验”处理）。
- 明确“无 group 行 + permit REVOKED/CONSUMED + PRE”进入人工队列，不得因 AC-27A“组不存在→未关闭”就跳过审计。
- owner/expiry 保持 ops/Developer + 2026-12-31；在 §8 证据中记录查询返回 0 行不等于安全（需先证明无历史 G1）。

### `[MAJOR]` R2-M3 — DEFERRED 项有 owner/expiry，但缺 per-item 停止/退出条件

**证据**

- M-1 `§12.10:514` / M-3 `O15:474` / M-4 `§12.12:516` / M-5 `§8:419` / M-7 `§12.11:515` / T-3 `§14.1:542` / T-1 `§9:433` 都有 owner/expiry（或 TODO owner），但**只有全局 §15.2**，没有每项的“何时必须停手/关闭”的明确条件。
- 例如 M-1 的停止条件只有 `§15.2 #4`（发现 `NONE/legacy` 误读才回 Architect），不是“审计未完成/审计发现错误 PRE 时 AC-27A 不得收口”的 per-item exit；T-3 多组释放只有 `§14.1` 的 DEFERRED 行，没有“多组 AC-27 证据不得标记已关闭”的停止句。
- 这不影响 0.2 的文本正确性，但影响 **AC-27 收口/Release 的审计可执行性**；用户/Reviewer 无法仅凭表格判断某 DEFERRED 到期/失败时停在哪里。

**具体修改**

- 在 §1.1 或新增 §15.4 增加列：`QC 项 | owner | expiry | stop/exit`；对 M-1/M-3/M-5/M-7/T-3/T-1 写明确 exit（例如“审计发现 PRE 但无 S6 证据 → AC-27A 第 4/5 条不得 AC-VERIFIED，冻结收口”“T3 多组 release 未实现 → AC-27 EARS-3 不得标 VERIFIED”）。
- 到期/提前关闭时由 Lead 在台账记录；不得只靠全局 §15.2 的“回 Architect”概括。

### `[MAJOR]` R2-M4 — R-2a 新增的 `ListShipments` 远程读没有 deadline/退避约束

**证据**

- §3.4.2 `:226` 只写 `page_size=100,total<=100`；§3.4.5 `:229` 写不可用 → `Unavailable`，但未写 deadline/timeout/backoff。
- `AGENTS.md` §4 要求跨服务调用设置 Deadline/Timeout，禁止无退避重试；R-2a 在 `AdmitRefund` 的同步请求路径新增一个远程读，且 **partial refund 的成功/失败依赖它**。若 Fulfillment 慢，Order 的 `AdmitRefund` 会被拖住；若调用方重试，没有退避要求。
- 现有 `CancelIfCreated` 在 ADR §5.3 有 deadline 1s（`0007:622`），R-2a 没有对应约束。

**具体修改**

- 明确 `ListShipments` 使用调用方 context + 明确 deadline（建议 ≤1s，或与 Fulfillment 既有读超时一致），不可用/超时 → `Unavailable`，不重试或按现有退避；不得在 Order 事务内调用。
- 在 §10 加 O4d：`ListShipments` 超时 → `Unavailable`、零写入；调用方重试；不降级为“允许部分退款”。
- 若需重试，说明退避/上限（AGENTS §4），不得无脑重试。

### `[MAJOR]` R2-M5 — A8 的“0 行读回”未定义 row 缺失/非法状态

**证据**

- A8 `:297`：“0 行 → 读回已持久化结果并返回，不得再写 outbox/release”；`finishRemoteAdmission` 当前 `:590-594` 是 `UPDATE ... WHERE refund_id=?`（A8 计划加 state CAS），然后 `:595` 写 outbox。
- 0 行可能是：① 另一执行者已写 terminal（正常，读回）；② 行被删除/不存在；③ `state` 不是 `PENDING_REMOTE`（例如 TCC `result/state` 异常）；④ 更新影响 0 行但 `state` 仍是 PENDING（理论上不应）。A8 只写“读回已持久化结果”，未定义 ②③④ 的 error/Conflict/告警；若读回 `nil`，实现可能 panic 或返回空结果。
- 对 ②/③，R-6 的“无脏写”要求 fail-closed，不得静默写 outbox/release，也不得把 nil 当成功。

**具体修改**

- A8 补：0 行后 `findRefundAdmission` 重读；若 nil → `apperr.Internal/Conflict("admission disappeared during Tx2")`；若 `state=PENDING_REMOTE`（CAS 未生效）→ 重试/Conflict；若 terminal → `admissionReply`；所有 0 行路径都不写 outbox/release。
- 增加 O11c：手工删除/篡改 admission 后 Tx2 0 行 → fail-closed，无 outbox/release。
- 在 §7.2 W5 说明 A8 的 0 行分支。

## R2.5 `[TODO]`

- **R2-T1** `§3.1:191`（`CANCELLED` 行）写“**不得** `Payment.Refund/RefundBatch` 之外的写（由调用方在准入后决定）”，语义可被读成“除退款命令外不得写任何东西”；建议改为“Order 准入层不写 Payment；`Payment.Refund/RefundBatch` 由调用方在准入后决定；本层只写 admission/outbox/release”。
- **R2-T2** §10 缺 O4d（R-2a `ListShipments` 超时/不可用）与 O4e（`EXCEPTION` 无 `shipped_at` → `Unavailable`）；`§3.4.5` 已写行为，测试矩阵未列。
- **R2-T3** `§3.4.2` 未明确 map 中 `CANCELLED` 行按“未离仓”处理（应忽略其 `status` 或明确 `CANCELLED` 不是离仓）；建议与 TCC `loadTccShipmentFacts`（`tcc_settlement.go:190-193`）同口径，并加 O4b 的 `CANCELLED` 行用例。
- **R2-T4** W9/F7 的 G1 late `order.paid` 用例缺；若按 R2-M1 选择“正常 placeholder 允许”，补 `order.paid` 迟到 → `CREATED` → 重放取消 → PRE 的集成用例，并注明不是人工证据。

## R2.6 崩溃窗口 / B-1 协议 / §7.4.1 的 delta 结论

| 窗口 | 0.2 结论 | R2 判断 |
|---|---|---|
| 1 本地提交后崩溃 | W1 承认 Tx1 写入；重放 re-probe | **PASS**；但 R2-B1 的 gate 不能覆盖 existing terminal（新问题） |
| 2 RPC 成功后崩溃 | W2 改为“Tx1 已写、conflict 不新增” | **PASS** |
| 3 消息重复/乱序 | W9 + F7/O14 | **PARTIAL**：consumer no-op 正确；`order.paid` late 断言有 R2-M1 反例 |
| 4 取消/确认竞态 | W4；R-2a 的 gate 在正常 `SHIPPED→CANCELLED` 不可逆的前提下安全 | **PASS（正常状态机）**；但同 `refund_id` 幂等被 R2-B1 破坏 |
| 5 外部重放缺失 | W8 + TCC `ADMITTING` 查询；ops | **PASS（有发现口径）**；M-1/M-5 审计仍需 R2-M2 补全 |

**B-1 的 A7/A8/§7.4 与 INV-45/AC-62/AC-50 同 `refund_id` 幂等**：

- `RESERVED` 保留、不自动释放、不写 permit、0 终态/0 release、AC-62 人工授权例外：**与 INV-45/AC-62 一致，PASS**。
- 同 `refund_id` 冲突重放：**B-1 本身 PASS**（re-probe + 无新增）；**但 R-2a gate 放在 existing 读取之前会破坏它（R2-B1）**。
- `§7.4.1` 的“Tx1 前存在性预检不能消除 conflict”时序**成立**（T0 无行 → T1 建行 → T3 探测）；它讨论的是 existence pre-check，不是 R-2a 的 partial gate；两者不能混用。**PASS**。

## R2.7 最终判定

**BLOCKED。**

- **B-1/B-2**：文本/测试口径 **CLOSED**；A8 0 行语义仍需补（R2-M5）。
- **B-3**：人工 W6 **CLOSED**；正常 `order.paid` 侧 **PARTIAL**（R2-M1）。
- **M-2**：**NOT CLOSED**；R-2a 需先解决 R2-B1（幂等）与 R2-B2（§5.2.2 预读）。
- **M-1/M-5**：DEFERRED/审计查询需补无 group 行（R2-M2）。
- **其余 M-3/M-4/M-6/M-7/M-8/M-9/M-10/M-11/M-12/M-13**：文本层面可接受；M-8 的 W9 受 R2-M1 影响；M-9 受 R2-M5 影响。
- **DEFERRED owner/expiry**：M-1/M-3/M-4/M-5/M-7 均有 owner/expiry（或 TODO owner）；**停止条件只有全局 §15.2**，需补 per-item exit（R2-M3）。

**只读声明**：未执行 `go test`/`-race`/MySQL/迁移；只审 0.2 delta；未改 ADR/SPEC/代码/测试/台账；只追加本 R2 小节；未 commit。**AC-27 仍 AC-REF**，AC-50/AC-27A 未 AC-VERIFIED。

**R2 Actor**：`qc-ac50`
**R2 Reviewed-by**：待 Architect 修订 / 人类批准
**R2 Status**：**BLOCKED — DRAFT 0.2 不得进入实现**

---

# R3 终审（DRAFT 0.3）

| 项 | 值 |
|---|---|
| 审查对象 | `docs/adr/0007-amd1-addendum-ac50-nowaybill.md` **DRAFT 0.3**（工作树，未 commit；693 行；删除 R-2a/全部 `ListShipments`） |
| 结论 | **BLOCKED**（4 个 `[BLOCKER]` + 2 个 `[MAJOR]` + 3 个 `[TODO]`；0 BLOCKER 才 READY，不满足） |
| 日期 | **2026-11-26** |
| Actor | `qc-ac50`（Quality Checker；与 `arch-ac50` 不同 actor） |
| base | `47db2e0d35d92520b9f9a55c26af3230fe85c2cc` |
| head | `47db2e0d35d92520b9f9a55c26af3230fe85c2cc`（`git rev-parse HEAD`；被审 ADR 为工作树修改，blob `b5129b51371180ee6ce247a8c3143ee3ed73cd27`，sha256 `013a65d5bea4296f0d031bd13e248d04d764ed18340e42987f5b8a7fb17258d2`） |
| review_scope | 只审 0.3 的 A 协议 delta：`NO_SHIPMENT` 的 Tx2 `result=CONFLICT/state=PENDING_REMOTE`、R-2b 部分金额、R-2a 删除、B-3/W9、M-1…M-13/R2 项的关闭/延期、§1.3/§8.2/§8.3、5 个崩溃窗口 + 重复/乱序。不重审全文、不执行代码/测试。 |
| release_status | **NOT RELEASE**；不批准 merge/Release/AC-VERIFIED；0.3 仍为 DRAFT。 |

## R3.0 结论

**BLOCKED。**

- **B-1/B-2** 保持 CLOSED；**B-3** 的人工侧协议在本稿是诚实的：W9 不再声称“迟到 `order.paid` 不建 placeholder”，而是承认真实 placeholder、CONFLICT 后不自动 PRE，阻断另立 `ORDER-PAID-PLACEHOLDER-01`（DEFERRED）。**B-3 可判 CLOSED（人工侧）/正常 consumer DEFERRED**。
- **R-2a 与全部 `ListShipments`/事务外预读已删除**：R2-B1（幂等）与 R2-B2（ADR §5.2.2）在本稿消失；`AdmitRefund` 先读 existing、CONFLICT 直接返回、`A8` 四分支 0 行语义已补；**R2-B1/B2 CLOSED**。
- **但 A 协议本身仍不满足“0 BLOCKER”**：
  1. **R3-B1（TCC）**：`result=REFUND_ADMISSION_CONFLICT` 被 TCC 侧 `ensureTccRemoteAdmissionTx` 判为“非法 result”，TCC 重试在到达 `AdmitRefund` 的 CONFLICT 短路之前就失败；TCC 的 A 协议不可执行，且 `allowed_paths` 禁止改生产 `tcc_*.go`。
  2. **R3-B2（部分金额 CANCELLED）**：无预读后，`CONSUMED` 部分金额在 Tx2 调 `CancelIfCreated`；一个真实 `CREATED` 行会先被取消，然后才返回 `PARTIAL`。AC-50 `0002:217` 明确部分金额“不得改动许可/运单/打款”；这与 R-2b 直接冲突。
  3. **R3-B3（A 记录/SPEC）**：`docs/review-tracking.md:382-387` 记录的 route A 只授权“`NO_SHIPMENT` → `REFUND_ADMISSION_CONFLICT` + 人工队列”，**没有**授权 `result=REFUND_ADMISSION_CONFLICT` 持久化，也没有授权“同 `refund_id` 不再探测”；0.3 `:10`/`:75` 把这两项加进“人类 A”，但 canonical 记录未同步。另 `SPEC-0001:645` 仍写 `result` 只允许 PRE/POST；`CONFLICT-SPEC-SYNC` 只是 stop，不是已关闭。
  4. **R3-B4（AC-50 `0002:228`）**：`§8:460` 仍写“申请保持可重试（AC-50 `0002:228`）”，而 `§3.5.4:295` 规定同 `refund_id` 读 CONFLICT 后**不再探测**、不再写；`AfterSales` 重试只会得到同一错误，不能自动收敛。这是“保持可重试”与“不再探测”的**自相矛盾**，需把“可重试”限定为人工 re-arm 后，或取得 AC-50 的书面豁免。

## R3.1 A 协议逐项核对（0016 / AC-50 / INV-45 / AC-62）

| 项 | 结论 | 依据/说明 |
|---|---|---|
| `0016` 表约束 | **PASS** | `order_refund_admissions.result/state` 为 `VARCHAR(32) NOT NULL DEFAULT 'PENDING_REMOTE'`，无 CHECK（`migrations/order/0016_settlement_admission.sql:7-19`）；`REFUND_ADMISSION_CONFLICT` 25 字符，可容纳；`state` 保持 `PENDING_REMOTE`，不新增 state；无迁移。 |
| AC-50 `0002:223`（整组 CONFLICT） | **PASS（整组）** | `CONSUMED+无运单` 返回 `REFUND_ADMISSION_CONFLICT`；0.3 只在整组 `amount == 组剩余` 写 CONFLICT（`:242`、`:282`），部分金额优先 PARTIAL（V15-3/`0002:217`）。 |
| AC-50 `0002:217`（部分金额） | **FAIL（`CANCELLED` 分支）** | `CONSUMED` 部分金额在 Tx2 调 `CancelIfCreated`；`CREATED→CANCELLED`（`cancel_if_created.go:72-82`）后再返回 PARTIAL，**改动运单**，违反“不得改动许可/运单/打款”。`NO_SHIPMENT` 分支无副作用；`ALREADY_SHIPPED` 部分 POST 合法。 |
| AC-50 `0002:227`（同 `refund_id`） | **PARTIAL** | 持久化 CONFLICT 后返回同一错误符合“读已持久化结果”；但需 `AdmitRefund` 在 `GetIntent`/远程前读 existing（0.3 `:295`；当前代码 `:179-185` 在 `GetIntent` 之后，实施须前移，allowed_paths 允许）。 |
| AC-50 `0002:228`（保持可重试） | **FAIL** | `§8:460` 与 `§3.5.4:295` 冲突；冲突行无自动重探；人工 re-arm 才可继续。 |
| INV-45（准入不可逆/释放纪律） | **PASS** | CONFLICT/PARTIAL 不写 permit、不释放、不恢复 `CONSUMED`；commitment 保持 `RESERVED`；AC-62 人工授权是唯一释放例外。 |
| AC-62（人工释放） | **PASS（规则）** | `§8:462`/`§1.3:169` 明确授权释放只释放额度、不恢复 permit、不关闭组、不写 `DONE`；T3 未实现为 DEFERRED。 |
| `PENDING_REMOTE` 收敛 | **PARTIAL** | 正金额由发起方 task 重放/FAILED + ops re-arm；TCC 被 `ensureTccRemoteAdmissionTx` 挡住（R3-B1）；无 Order 扫描 worker，符合 W8/AC-27-LIVE-01，但 TCC 不可按 A 协议重放。 |
| worker 误重试 | **FAIL（TCC）** | TCC `TccCancelMarkPaid` 先 `applyTccCompensation`（`tcc_settlement.go` `writeTccBatchGroupsTx` → `ensureTccRemoteAdmissionTx`），再 `admitTccBatch`（`tcc.go:356`）；`ensureTccRemoteAdmissionTx` 的 switch 只接受 `PENDING/PRE/POST`（`tcc_settlement.go:847-876`），`result=CONFLICT` 走 `default` → `Failed("tcc refund admission result is not a legal value")`。TCC batch 停 `ADMITTING`、application `PENDING`；不会到达 `AdmitRefund` 的 CONFLICT 短路，也不返回同一 `REFUND_ADMISSION_CONFLICT`。 |

## R3.2 新增 `[BLOCKER]`

### `[BLOCKER]` R3-B1 — `result=CONFLICT` 在 TCC 侧是非法 result；TCC A 协议不可执行

**证据**

- 0.3 `:91`/`:242`/`:293`/`:521` O1b：TCC `NO_SHIPMENT` 写 `result=REFUND_ADMISSION_CONFLICT,state=PENDING_REMOTE`，同 `refund_id` 重放返回同一错误。
- `TccCancelMarkPaid` 的重试顺序：`applyTccCompensation`（`tcc_settlement.go:312`/`resumeTccCompensationTx`/`writeTccBatchGroupsTx`）先于 `admitTccBatch`（`tcc.go:356`）。`writeTccBatchGroupsTx` 对 `CONSUMED` 组调用 `ensureTccRemoteAdmissionTx`（`tcc_settlement.go:717`）。
- `ensureTccRemoteAdmissionTx`（`tcc_settlement.go:847-876`）先 `INSERT ... ON DUPLICATE KEY`，再 `SELECT result,state`，`switch result { case refundPending, refundResultPre, refundResultPost: return nil; default: return apperr.Failed("tcc refund admission result is not a legal value") }`。`result=CONFLICT` → **default**，TCC 重试在到达 `AdmitRefund` 的 existing CONFLICT 短路前就失败。
- 0.3 `forbidden_paths:20` 禁止改生产 `internal/services/order/tcc_*.go`；`allowed_paths:19` 只允许 `tcc_admission_integration_test.go`（测试）。因此**按本稿无法修复 TCC**。
- 这不是“TCC 停 ADMITTING 等待人工”的正常状态：`TccCancelMarkPaid` 返回的是“非法 result”错误，不是 `REFUND_ADMISSION_CONFLICT`；TCC 的 manual 查询仍能发现 batch，但**同 `refund_id` 重放不返回同一 CONFLICT**，违反 0.3 自己的 R-2/A9 和 O1b 预期（O1b 若只调 `AdmitRefund` 会绕过此路径）。

**具体修改（二选一）**

1. **允许 TCC 识别 CONFLICT**：把 `ensureTccRemoteAdmissionTx` 的 `switch` 加 `refundResultConflict`（或等价值），返回 `REFUND_ADMISSION_CONFLICT` 同一错误；这需要把生产 `tcc_settlement.go` 加入 `allowed_paths`（或另拆 change_id），并补 `TccCancelMarkPaid` 级 O1b（不是只调 `AdmitRefund`）。
2. **TCC 不持久化 CONFLICT**：TCC `NO_SHIPMENT` 只返回错误、不写 `result=CONFLICT`；同 `refund_id` TCC 重试仍重探（不满足 A 的“不再探测”），需要人类 A/PO 明确 TCC 例外；或 TCC 改用 `refund_applications` 状态标记，但那是 schema/新状态字段，超出本 amendment。

### `[BLOCKER]` R3-B2 — R-2b 的 `CONSUMED + 部分 + CANCELLED` 先取消运单再返回 PARTIAL，违反 AC-50 `0002:217`

**证据**

- 0.3 `§3.4:277`：`CONSUMED` + `< 组剩余` + `CANCELLED`（pre-ship）→ `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；`§3.4:283` 说“接受 Tx1 占位 + 人工队列的代价”；`§8:463` 同样把 `CANCELLED` 部分金额列入人工队列。
- 但 `finishRemoteAdmission` 在 Tx2 中先调 `CancelIfCreated`（`refund_admission.go:570-584`）；`CancelIfCreated` 对 `CREATED` 行执行 `UPDATE ... status=CANCELLED`（`cancel_if_created.go:72-82`），然后 0.3 才在 Tx2 用本地事实判定 partial 并返回 PARTIAL。
- AC-50 `0002:217`：“IF 该组未离仓 AND 本次金额 < 整组退款金额，THEN ... `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；**不得改动许可/运单/打款**。”`CREATED→CANCELLED` 就是改动运单。`CANCELLED` 分支无法区分“真实已取消（无副作用）”和“本次刚取消的 `CREATED`（有副作用）”；0.3 的表把两者混为 `CANCELLED`。
- 0.3 已无 `ListShipments`/预读，无法在调用 `CancelIfCreated` 前知道 `CREATED` 还是 `SHIPPED`；要区分 `ALREADY_SHIPPED`（部分合法 POST）和 `CREATED`（部分 PARTIAL），当前唯一探测就是会改运单的 `CancelIfCreated`。因此 **A 路线与 AC-50 `0002:217` 在“部分金额 + `CREATED`”上不可同时满足**。

**具体修改（需人类/PO 裁决，不是 Developer 可修）**

1. **保留 A 路线**：由人类/PO 书面明确“部分金额 `CONSUMED+CREATED` 的 `CancelIfCreated` 副作用（取消运单）被接受”，并同步 AC-50 `0002:217` 的例外/新版本；在此之前 `R-2b` 不得实现。
2. **或**允许一个**只读**的 `PeekShipment`/`ListShipments`（仅 partial、无副作用），但这违反人类 A 的“无预读”；需人类重新裁决。
3. **或**部分金额 `CONSUMED` 一律不探测、一律返回 PARTIAL（放弃部分 post-ship 的 `0002:226` 合法性），需 PO 裁决；不能让 `CancelIfCreated` 的副作用被“人工队列”掩盖。

### `[BLOCKER]` R3-B3 — canonical 人类 A 未授权 `result=CONFLICT` 持久化/不再探测；与 `SPEC-0001:645` 仍漂移

**证据**

- `docs/review-tracking.md:382-387`（route A）：“drop the `ListShipments` pre-read; accept the Tx1-written artifacts and route `NO_SHIPMENT` to `REFUND_ADMISSION_CONFLICT` + manual `AC-27-NOWB-01` queue (commitment stays RESERVED, outbox placeholder kept, no PRE/refund/release).” **没有** `result=REFUND_ADMISSION_CONFLICT` 持久化，也**没有**“同 `refund_id` 不再探测”。
- 0.3 `Human decision:10` 却写“③`NO_SHIPMENT` 时持久化 `result=REFUND_ADMISSION_CONFLICT`…④同 `refund_id` 重放返回已持久化 CONFLICT、不再远程探测”，并归为“人类 A”；`:75`/`:499` 也把该值写成“人类 A 授权”。
- `SPEC-0001:645` 仍写“`result` 只允许 `PRE_SHIP_REVOKED`/`POST_SHIP_ALLOWED`”；0.3 `:499` 承认漂移，仅登记 `CONFLICT-SPEC-SYNC`（PO，2026-12-31；PO 拒绝 → 实现停止），**不是已关闭**。
- 结论：A 的**记录范围**小于 0.3 的 `Human decision`，且 `result=CONFLICT` 是与 `SPEC-0001` 注冲突的新值；仅凭 Addendum 的转述不能生效。按 `AGENTS.md`/宪法 Governance 4/6/7，人类裁决必须有书面记录；PO 未同步前不能把 `result=CONFLICT` 作为已批准契约。

**具体修改**

- Lead 必须在 `docs/review-tracking.md` 记录**完整 A**（含 `result=CONFLICT`、`state=PENDING_REMOTE`、同 `refund_id` 不再探测、部分金额/人工队列），并由人类确认；PO 必须在 SPEC-0001 下次升版同步/书面接受 `CONFLICT` 值（或删除该持久化）。
- 在 `CONFLICT-SPEC-SYNC` 关闭前，`result=CONFLICT` 只能作为 **DRAFT 设计**，不得进入实施 Brief 的已批准契约；`PO 拒绝 → 实现停止` 的 stop 必须前置为“未同步 → 不实现”，而不是实施时再停。

### `[BLOCKER]` R3-B4 — AC-50 `0002:228`“申请保持可重试”与“同 `refund_id` 不再探测”自相矛盾

**证据**

- 0.3 `§8:460`：“申请状态：**保持可重试（AC-50 `0002:228`）**；不得打款、不得自动释放、不得置 `FAILED_PERMANENT`…”。
- 0.3 `§3.5.4:295`：同 `refund_id` 且 `result=CONFLICT` → **返回同一 `REFUND_ADMISSION_CONFLICT`，不再调用 `CancelIfCreated`/`GetIntent`**、不再写任何行；`§7.3:414` 同。
- `AfterSales` 的 `releaseRefundTask` 以同一 `refund_id` 重试（`workers.go:399-405,724-748`），每次得到同一 CONFLICT，`maxTaskAttempts=8` 后 `FAILED`；这不是 AC-50 的“申请保持可重试”，而是“等人工 re-arm”。0.3 的 `§8` 与 `§3.5/§7.3` 不能同时为真。
- 影响：Reviewer 无法判定 `AC-50 0002:228` 是否满足；`AC-27-NOWB-01` 的“保持可重试”与 A 的“no probe”冲突，且 `INV-42` 的“可重试”被降为人工。

**具体修改**

- 二选一：①在 §8 改为“申请保持**人工可重试**（普通 replay 返回同一 CONFLICT；仅 ops re-arm/AC-62 后可继续）”，并取得 PO/人类对 AC-50 `0002:228` 的书面口径；或②允许同 `refund_id` 在人工 re-arm 前**不探测但可转人工**（仍不是自动重试），并把这写成 AC-50 的例外。
- 在 §1.3 增加 `AC-50-RETRY-01`（owner PO，expiry 2026-12-31，stop：PO 不接受 → 实现停止），与 `CONFLICT-SPEC-SYNC` 并列。

## R3.3 新增 `[MAJOR]` / `[TODO]`

- **`[MAJOR]` R3-M1 — W9“由 Fulfillment/ops 取消”没有 owner/runbook**：`§7.2:407` 说迟到 placeholder “由 Fulfillment/ops 取消并走人工队列”，但 `§1.3` 只登记 `ORDER-PAID-PLACEHOLDER-01`（阻断创建，DEFERRED），没有“取消迟到 placeholder”的 owner/expiry/stop；`§8` 的 TCC/普通队列查询也不含该行。补一个 runbook（Fulfillment 取消 `CREATED`；ops 审计；不得当证据），或明确该行在 CONFLICT 后保持 `CREATED` 直到人工处置；否则 W9 的“由 Fulfillment/ops 取消”无执行主体。
- **`[MAJOR]` R3-M2 — W9 对 partial PENDING 断言“重探可取消→PRE”不准确**：`§7.2:407` 的“若 admission 仍 PENDING → 重探可取消→PRE”未区分整组/部分。若 PENDING 是**部分金额**，重探 `CANCELLED` 应返回 `PARTIAL`（R-2b），不是 PRE；补“整组→PRE / 部分→PARTIAL”，并在 F7b/O14b 加部分金额的 late `order.paid` 用例。
- **`[TODO]` R3-M3 — `§8.3` re-arm 的 version/审计**：`:484` 的 `UPDATE ... WHERE result=CONFLICT AND state=PENDING` 已有 result/state CAS，第二个并发 re-arm 会 0 行，不是阻断；建议（非必须）加 `AND version=?` 或 `SELECT FOR UPDATE` 作为审计强化，并在工单记录版本。`§1.3:176` 已有 stop。
- **`[TODO]` R3-T1** `§8.2:460` 与 `§3.5.4:295` 的“可重试”矛盾见 R3-B4；修复后同步 `§1.3` 的 `AC-50-RETRY-01`。
- **`[TODO]` R3-T2** `A8` 的 0 行分支已补 nil/CONFLICT/PRE-POST/PENDING-non-CONFLICT；但**`state=PRE/POST` 且 `result=CONFLICT`**（非法组合）未定义，补 `Internal/Conflict` 与测试；`result=CONFLICT` 加 `state=PENDING` 以外值应 fail-closed。
- **`[TODO]` R3-T3** `§8.3` re-arm 的 `placeholder` 对正金额写 `POST_SHIP_ALLOWED`，对 TCC 写 `PENDING_REMOTE`；但 `§8.2` 的“申请保持可重试”仍以 `result/state` 判定，补一段“re-arm 后的 result/state 组合是唯一合法过渡”的表，避免 Reviewer 把 `POST/PENDING` 当终态（A2 已排除，但 runbook 未写）。

## R3.4 13 MAJOR / §1.2 / §1.3 / §8.2 / §8.3 关闭表

| QC 项 | 0.3 状态 | R3 判定 | 缺口 |
|---|---|---|---|
| **M-1** legacy | PARTIAL（DEFERRED） | **CLOSED（deferred）** | `§1.2:145`/`§1.3:168` owner/expiry/stop；审计 `LEFT JOIN` 已补；仍需 ops 执行。 |
| **M-2** 部分金额 | CLOSED（规则）/人工债务 | **NOT CLOSED** | R3-B2：`CONSUMED+CANCELLED` 取消运单；AC-50 `0002:217`。 |
| **M-3** A4/AC-62 | CLOSED 规则/DEFERRED | **CLOSED（规则）/DEFERRED** | `§1.3:169` owner/expiry/stop；T3 未实现。 |
| **M-4** TCC result | CLOSED 读者/DEFERRED | **CLOSED（读者）/DEFERRED** | `§1.3:170`；A2 双条件；`TCC-PENDING-RESULT-01`。 |
| **M-5** 存量 PRE | CLOSED 协议/DEFERRED | **CLOSED（协议）/DEFERRED** | `§8:469` `LEFT JOIN`/无 group/`PENDING` 分支；`§1.3:171`。 |
| **M-6** TCC 测试路径 | CLOSED | **CLOSED** | allowed `tcc_*_test.go`；生产 forbidden。 |
| **M-7** TCC 队列 | CLOSED 查询/DEFERRED | **CLOSED（查询）/DEFERRED** | `§8:468`；`§1.3:172`。 |
| **M-8** 重复/乱序 | CLOSED | **CLOSED（测试）** | W9/F7/O14/F7b/O14b；R3-M2 补 partial。 |
| **M-9** Tx2 守卫 | CLOSED 规则 | **CLOSED** | A8 + 0 行；O11b/O11c。 |
| **M-10** POST commitment | CLOSED | **CLOSED** | `§3.1` 分行；O5/O5b。 |
| **M-11** consumer | CLOSED | **CLOSED** | `§2.2` no-op；F7/O14。 |
| **M-12** 可达性 | CLOSED（说明） | **CLOSED（说明）** | `§0.2`/F8；`§1.2:156` stop。 |
| **M-13** setup | CLOSED | **CLOSED** | `§10` setup。 |
| **R2-B1** 幂等 | CLOSED（删除 R-2a） | **CLOSED** | 无 `ListShipments`；`AdmitRefund` 先读 existing（`:295`）。 |
| **R2-B2** §5.2.2 | CLOSED（删除） | **CLOSED** | 0.3 无预读；`§5.2.2` 不例外。 |
| **R2-M1** late order.paid | CLOSED（协议/DEFERRED） | **CLOSED（协议）** | W9 真实；`ORDER-PAID-PLACEHOLDER-01`；R3-M1 owner 待补。 |
| **R2-M2** 审计无 group | CLOSED | **CLOSED** | `§8:469` `LEFT JOIN`/`g.group_no IS NULL`。 |
| **R2-M3** per-item stop | CLOSED | **CLOSED** | `§1.3` 逐项。 |
| **R2-M4** R-2a deadline | CLOSED（删除） | **CLOSED** | 无 `ListShipments`。 |
| **R2-M5** A8 0 行 | CLOSED | **CLOSED** | A8 四分支；O11c。 |
| **T-1 / T-3** | TODO / DEFERRED | **CLOSED（defer）** | `§1.3:174`/`:173`；`T-1` PO 同步。 |

**结论**：13 MAJOR 中 **M-2 未关闭**（R3-B2）；M-1/M-3/M-4/M-5/M-7/T-1/T-3 的 DEFERRED 有 owner/expiry/stop（§1.3）；其余文本可接受。**但 A 协议的两个新 B（TCC、部分 CANCELLED）使 M-2 与 R-2b 不能实施。**

## R3.5 5 个崩溃窗口 + 消息重复/乱序

| 窗口 | 0.3 覆盖 | R3 判断 |
|---|---|---|
| 1 本地提交后崩溃 | W1；Tx1 已写，重放 PENDING → 重探 | **PASS**（R3-B3 的 result 记录仍需先授权） |
| 2 RPC 成功后崩溃 | W2/W3/W6；NO_SHIPMENT/CANCELLED/CONFLICT 重放 | **PASS**（positive）；TCC 重放被 R3-B1 挡 |
| 3 消息重复/乱序 | W9/W5；consumer 终态 PRE、NO_SHIPMENT no-op、order.paid 真实 placeholder | **PASS（协议）**；R3-M1 取消 owner；R3-M2 partial PRE 断言 |
| 4 取消/确认竞态 | W4；Ship/取消行 CAS | **PASS**（正常状态机）；partial `CREATED` 取消副作用见 R3-B2 |
| 5 外部重放缺失 | W8；AfterSales/TCC task + AC-27-LIVE-01 + TCC 查询 | **PASS（发现）**；TCC 被 R3-B1 挡；conflict 无自动收敛，符合 A |
| 重复/乱序（W9） | W9；F7/O14/F7b/O14b | **PARTIAL**；R3-M1/M2 |

**§7.4.1** 的“Tx1 前存在性预检不能消除 conflict”在 0.3 中已明确是**假设性论证**，不是实现；其结论（线性化点在 Tx1+探测）与 A 路线一致，**PASS**。`§8.2/§8.3` 的队列/re-arm runbook 有 owner/expiry/stop（`§1.3`），但 re-arm 的版本审计（R3-M3，TODO）和 AC-50 retry（R3-B4）未闭合。

## R3.6 最终判定

**BLOCKED。**

- **R2-B1/B2**：**CLOSED**（R-2a/`ListShipments` 已删除；`AdmitRefund` existing 先行）。
- **B-1/B-2**：**CLOSED**；**B-3**：**CLOSED（人工侧）/正常 consumer DEFERRED**；W9 不再用“不建 placeholder”掩盖代码行为，诚实。
- **B-3/W9** 的 `ORDER-PAID-PLACEHOLDER-01` 是诚实的 DEFERRED；但 W9 的取消 owner（R3-M1）和 partial PRE 断言（R3-M2）需补。
- **A 协议**：`0016`/`AC-50:223`/`INV-45`/`AC-62` 的“整组 CONFLICT”基本成立；**`AC-50:217`（partial CANCELLED 取消运单）**、**`AC-50:228`（不再探测 vs 可重试）**、**TCC `ensureTccRemoteAdmissionTx`（CONFLICT 非法 result）** 与 **canonical A/SPEC-0001:645** 未闭合 → 4 BLOCKER。
- **M-2**：**NOT CLOSED**（R3-B2）；其余 M-1/M-3/M-4/M-5/M-7 的 DEFERRED 有 owner/expiry/stop。
- **5 窗 + 重复/乱序**：窗口齐备；TCC 重放与 partial 取消仍阻断；conflict 的 `PENDING_REMOTE` 不会自动收敛，符合 A 的人工队列，但必须补齐 TCC/partial 的契约。

**只读声明**：未执行 `go test`/`-race`/MySQL/迁移；只审 0.3 delta；未改 ADR/SPEC/代码/测试/台账；只追加本 R3 小节；未 commit。**AC-27 仍 AC-REF**；AC-50/AC-27A 未 AC-VERIFIED。

**R3 Actor**：`qc-ac50`
**R3 Reviewed-by**：待 Architect 修订 / 人类批准
**R3 Status**：**BLOCKED — DRAFT 0.3 不得进入实现**

---

# R4 终审（SPEC v1.11 + ADR 0.4 + impact）

| 项 | 值 |
|---|---|
| 审查对象 | `docs/specs/0002-payment-settlement-admission.md` **v1.11（in-review / REVIEW_REQUIRED）**；`docs/adr/0007-amd1-addendum-ac50-nowaybill.md` **DRAFT 0.4**；`docs/adr/0007-amd1-ac50-impact-analysis.md` **DRAFT 0.2** |
| 结论 | **BLOCKED**（3 个 `[BLOCKER]` + 3 个 `[MAJOR]` + 2 个 `[TODO]`；0 BLOCKER 才 READY，不满足） |
| 日期 | **2026-11-26** |
| Actor | `qc-ac50`（Quality Checker；与 `arch-ac50`/`arch-ac50-impact` 不同 actor） |
| base | `b3f94f6feabdad9257ee085b464a666afd5e1ab7` |
| head | `b3f94f6feabdad9257ee085b464a666afd5e1ab7`（`git rev-parse HEAD`） |
| 被审 blob | SPEC `3a844b0a137931ac15ef965d015893dbc6c897e3`；ADR `8595c7ca14b1333fe6bba5be8d7f3d593df7d38b`；impact `7265cc9ccf390b37c7bedf78e46447dca2582c6a` |
| review_scope | 只审 v1.11 新增/修订的 AC-50/51/61/62/65/67 + S20/INV-51/52/AC-69..72/§5.10/E32..35/D-30/31/§11/§12，及 ADR 0.4 与 impact 0.2 的对应；不重审 v1.10 全文、不执行代码/测试/迁移。 |
| release_status | **NOT RELEASE**；不批准 merge/Release/AC-VERIFIED；SPEC v1.11 仍 in-review，ADR 0.4 仍 DRAFT。 |

## R4.0 结论

**BLOCKED。** 核心数据结构与 TCC/部分金额方向正确：

- `REVIEW_REQUIRED` 15 字符放进 `order_refund_admissions.result VARCHAR(32)`、`refund_applications.admission_result VARCHAR(24)` 均成立；`state=PENDING_REMOTE` 不新增值；`0016/0017` 无 CHECK 阻断；`refund_batches.state='MANUAL_HOLD'` 已在 0017 CHECK 中；**无迁移/无回填**。现有 `PRE/POST/PENDING_REMOTE` 行语义不变；旧代码回滚需先审计（ADR §7.6/impact §7.2），这一点诚实。
- TCC 方向已按 PO AC-71 修正：`REVIEW_REQUIRED` 为合法持久裁决，batch `MANUAL_HOLD`、app `PENDING`、`refund_batches.last_error`、自动续跑 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`、0 Payment；`tcc_settlement.go`/`tcc_admission.go` 进入 allowed_paths（PO §11 明确要求）。`0017` CHECK 已含 `MANUAL_HOLD`。
- R-2b 部分金额已按 D-31 改为**不调用 `CancelIfCreated`**、只用 Order 组投影三分；`AC-50:217` 的“不改运单”得到满足；`已离仓→POST`、`未离仓→PARTIAL 零写入`、`不可判定→REVIEW+1 commitment` 与 AC-62 冻结原则自洽。
- B-3/W9 已按 R2-M1 诚实降级：W9 不再声称“迟到 order.paid 不建 placeholder”；真实 placeholder 由 Fulfillment 取消/ops 审计，阻断另立 `ORDER-PAID-PLACEHOLDER-01`（DEFERRED）；没有用测试断言改写掩盖代码行为。

但仍有 **3 个合同级 BLOCKER**：

1. **R4-B1（普通售后 `refund_applications`）**：SPEC S20/AC-69 要求“发起方 `refund_applications.admission_result=REVIEW_REQUIRED`、申请 `state=PENDING`”，但普通售后实际使用 `after_sale_refund_tasks`，**没有 `refund_applications` 行**；impact D-2 自己标 `[假设]`“需 QC/PO 确认”，ADR 0.4 §12.3 也称普通售后无该行。SPEC 没有 carve-out，ADR 不能自行缩小 AC；**AC-69/S20 对普通售后不可验收**。
2. **R4-B2（S20/INV-51 的“每个 refund_id”与零元例外）**：S20 写“每个 `refund_id` 一条裁决”，INV-51 写“无法按 AC-50 判定的组级准入 SHALL 以 `REVIEW_REQUIRED` 持久记录”；但 AC-69/AC-67 明确零元路径**不持久化** `REVIEW_REQUIRED`、无 Tx1/无承诺。S20/INV-51 没有“零元除外”，与 AC-67/69 直接矛盾；零元也未被 S20 的“每个 refund_id”覆盖。
3. **R4-B3（原复核记录）**：SPEC AC-69 `0002:456` 与 AC-72 `0002:488` 要求“保留原复核记录、原因、证据、处置人，不得删除（对账/审计依据）”；ADR 0.4 §8.3 `:523` 写“admission 行按状态机迁移，行内不保留历史值；审计依据为 ops 工单 + 日志 + last_error；若 PO 要求行级历史/独立审计表，另立 L2 `REVIEW-AUDIT-01`”。PO 已经在本版 AC-69/72 要求，ADR 却 DEFERRED；**AC-69/72 的保留记录条款没有实现载体**，不能判可验收。

另 3 个 `[MAJOR]`：SPEC §11 `:870` 仍称“ADR 仍为 DRAFT 0.3”（实际 0.4，stale）；AC-72 的“用户可见/人工入口”明确另立 `REVIEW-REQUIRED-ENTRY-01`，故 AC-72 不能整体 AC-VERIFIED（诚实，但必须登记）；`tcc_*.go` 进 allowed_paths 不能仅由 PO §11 授权，仍需人类 `ADR_APPROVED` + Lead 独立 `change_id`/Plan。

## R4.1 `REVIEW_REQUIRED` 表示与兼容（0016/0017/旧行/worker）

| 检查 | 结论 | 依据 |
|---|---|---|
| `result VARCHAR(32)` 容纳 15 字符 | **PASS** | `0016:7-19`，无 CHECK；`REVIEW_REQUIRED` 15。 |
| `admission_result VARCHAR(24)` 容纳 15 | **PASS** | `0017:45-81`（`:56`）；`REVIEW_REQUIRED`=15；`REFUND_ADMISSION_CONFLICT`=25 放不下，D-30 用 REVIEW 正确。 |
| `state=PENDING_REMOTE` 不新增值 | **PASS** | `0016:11-12`；`state` 默认/值不变。 |
| `MANUAL_HOLD` 已存在 | **PASS** | `0017:20-33` CHECK 含 `MANUAL_HOLD`；无迁移。 |
| 既有 PRE/POST/PENDING 行兼容 | **PASS** | 只新增值；ADR §12/impact §2；`REVIEW` 只对新写。 |
| 现有 worker 是否把 `state=PENDING_REMOTE` 当可自动重试 | **PASS（需新代码守住）** | 全仓 grep：`order_refund_admissions` 只按 `refund_id` 被 `AdmitRefund`/`ensureTccRemoteAdmissionTx` 读取，无独立扫描 worker（`refund_admission.go:39-65`、`tcc_settlement.go:844-876`）；TCC batch 由 `MANUAL_HOLD` 短路。**ADR 必须保证** `AdmitRefund` 在任何 `GetIntent`/远程前读 existing（`:295`），否则旧行为会重探；这正是 ADR §3.5.4/§7.6 的要求。**不建议**为此新增独立 state（会引入新值/迁移/Check 风险），但需测试 O2b 及旧代码回滚审计。 |
| 旧代码回滚 | **PASS（诚实）** | impact §7.2：旧代码 `PENDING_REMOTE` 会重探，可能洗成 PRE/POST；ADR §7.6 要求回滚前审计，不得自动重映射。SPEC §12 同口径。 |

## R4.2 TCC（AC-71/ADR §3.6/impact §3）

| 检查 | 结论 | 依据/缺口 |
|---|---|---|
| `ensureTccRemoteAdmissionTx` 接受 REVIEW | **PASS（设计）** | ADR 0.4 `§3.6` 明列；`tcc_settlement.go` 在 allowed_paths。 |
| `ensureTccApplicationTx` 单调接受 REVIEW | **PASS（设计）** | `§3.6`；`tcc_settlement.go:788-840`；allowed。 |
| `writeTccBatchGroupsTx` 遇 MANUAL_HOLD 短路 | **PASS（设计）** | `§3.6`；返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`；`tcc_settlement.go:628-720`；allowed。 |
| `admitTccBatch`/`tccBatchManualError` | **PASS（设计）** | `tcc_admission.go` 在 allowed；`tccBatchManualError` 已有 `tcc_error`/`TCC_BATCH_REQUIRES_MANUAL_HANDLING`；`MANUAL_HOLD` 分支已有（`tcc_admission.go:85-87`）。 |
| `refund_batches.last_error` | **PASS** | `0017:28` 列存在；Tx2 同事务写；`tcc_reconcile.go:340` 只清 **refund_applications.last_error**，不是 batch；ADR §3.5 明确。 |
| 只阻塞本批/其他订单 | **PASS（设计）** | batch 按 `(order_no,payment_no)`；`MANUAL_HOLD` 仅本批；AC-71 `0002:477`。 |
| 0 Payment | **PASS（设计）** | `admitTccBatch` 在 `payTccBatch` 前返回；`MANUAL_HOLD` 分支；AC-71。 |
| `tcc_*.go` allowed 是否越界/PO §11 是否足够 | **PASS（范围）/需人类** | PO §11 明确 TCC 需改；ADR 0.4 allowed 仅 `tcc_settlement.go`/`tcc_admission.go`，不含 `tcc.go`/`tcc_payment.go`/`tcc_precheck.go`/`tcc_reconcile.go`；所需面足够。**但 PO §11 不是人类实现批准**：仍需 `ADR_APPROVED` + Lead 独立 `change_id` + Plan/Tasks；Reviewer 复核。不得把 PO §11 当 merge 授权。 |
| `TCC` 崩溃/重放 | **PASS** | W12；batch `MANUAL_HOLD`、app `PENDING`、0 打款、自动续跑稳定错误、re-arm 同批同 `refund_id`。 |
| **潜在缺口** | **CLOSED（设计）** | `tcc_reconcile` 不在 allowed，但其 `markTccApplicationDoneTx` 只在 Payment 成功事实时把 PENDING/PROCESSING→DONE；REVIEW 无 Payment，不会触发；`refund_consumer` 同理。若未来外部 Payment 事实出现，需 ops 审计。 |

**结论**：TCC 的 A 协议在 **allowed_paths/PO 范围内可执行**；R3-B1 **CLOSED（设计）**。但 **implementation 仍需人类批准**（见 §R4.6）。

## R4.3 部分金额 R-2b（AC-50:217/AC-70/INV-52）

| 分支 | 0.4 行为 | R4 判定 |
|---|---|---|
| `CONSUMED` 部分 + 组投影 `SHIPPED/RECEIVED` | 不探测/不取消；POST + commitment + 终态 outbox | **PASS**；AC-50 `0002:226`。 |
| `CONSUMED` 部分 + 组投影未离仓（`PAID/ALLOCATED` 或无组+`PAID/ALLOCATED`） | **写前拒绝** `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；零 admission/commitment/outbox | **PASS**；AC-50 `0002:217`“不得改动运单/许可/打款”。 |
| `CONSUMED` 部分 + 组投影缺失且 `orders.status ∈ {SHIPPED,COMPLETED}` / 不可判定 | `REVIEW_REQUIRED` + 1 条 `RESERVED` commitment；人工队列 | **PASS**；AC-62 冻结额度，AC-70 `0002:464`。 |
| 已有 PENDING 部分 + 重评未离仓 | 返回 `PARTIAL...`，保持既有 admission/commitment 不删/不释放 | **PASS**；PO AC-70 `0002:465`；AC-62 债务显式。 |
| `CONSUMED` 部分 + `CANCELLED` | **不再调用** `CancelIfCreated`；按局部投影三分 | **PASS**；R3-B2 关闭；O4b `cancelCalls==0`。 |
| `NONE/REVOKED` 部分 | Tx1 前拒绝、零写入 | **PASS**；现有 `:249-252`。 |
| 瞬态/不可判定探测 | `Unavailable`，不写 REVIEW/PRE/POST | **PASS**；AC-70 `0002:469`。 |

**R3-B2 CLOSED**。注意：投影滞后会导致“真实已离仓但投影未到”被临时判 PARTIAL（fail-closed，ADR §3.4 投影滞后），不得自动改整组；O4d 覆盖。

## R4.4 `REVIEW_REQUIRED` 幂等/崩溃/人工 re-arm

- **existing 前移**：SPEC AC-69 `0002:451` / ADR §3.5.4 `:306` 要求 `AdmitRefund` 在读 `Payment.GetIntent`/任何远程前先读 existing；当前代码 `:168` GetIntent 在 `:179` existing 前，**实施必须前移**（allowed `refund_admission.go`）。**PASS（设计）**。
- **同 refund_id REVIEW**：返回同一 `REFUND_ADMISSION_CONFLICT`，不探测、不读 Payment、不新增 outbox/release/commitment；Tx2 CAS `state='PENDING_REMOTE'`；A8 0 行四分支；**PASS（设计）**。
- **并发**：两个 Tx2 都 `<NO_WAYBILL>`，一个 CAS 成功，另一个 0 行读回 REVIEW；**PASS（设计）**；O11b/O11c。
- **re-arm**：同 `refund_id` 版本化把 `result` 置回 `PENDING_REMOTE`，TCC batch `MANUAL_HOLD→ADMITTING`；只 ops/真实 S6 证据/授权/审计；不得自动；`§8.3` 有 CAS；**PASS（设计）**。
- **崩溃**：W1-W12 齐；`REVIEW` 持久化后崩溃/重放；TCC `MANUAL_HOLD`；**PASS**。
- **`REVIEW_REQUIRED` 与 `AC-50:228`**：SPEC §5.10 已把“保持可重试”收窄为“不丢失、幂等可重放；人工 re-arm/新 refund_id 才继续”；ADR §7.3 同；**R3-B4 CLOSED（协议）**。**但仍需 PO/Reviewer 在台账按 AC-69/72 记录，不得声称自动可重试收敛。**

## R4.5 S20 / INV-51 / AC-69..72 的必要性、最小性与 EARS

| 项 | 判定 | 说明 |
|---|---|---|
| **S20** | **PARTIAL** | 作为持久裁决/状态承载必要；但“**每个 refund_id** 一条裁决”与零元不持久化、普通售后无 `refund_applications` 两处冲突（R4-B2/R4-B1）。需改为“持久化路径的每个 refund_id”+“若发起方有 `refund_applications` 行”。 |
| **INV-51** | **PARTIAL** | “有界/自动不可收敛”必要；但同样未排除零元（AC-67/69 不持久化），与 S20 一起需加零元例外。 |
| **INV-52** | **PASS** | 禁止取消副作用分类，直接闭合 R3-B2；必要。 |
| **AC-69** | **PARTIAL** | 取值/持久化/幂等/重放/终态判定必要；但普通售后 `refund_applications`（B1）与“保留原复核记录”（B3）不可验收。其余 EARS 完整。 |
| **AC-70** | **PASS** | 整组/部分/legacy 三分法、写前拒绝、不可判定 REVIEW、探测瞬态可重试，EARS 可验收；与 AC-50 `0002:217/223/226/461` 一致。 |
| **AC-71** | **PASS（设计）** | TCC batch `MANUAL_HOLD`/app PENDING/last_error/只阻塞本批/0 Payment/re-arm 同批；allowed `tcc_*` 足够；EARS 可验收（需真实 `TccCancelMarkPaid` 测试 O1b）。 |
| **AC-72** | **PARTIAL** | 告警/SLA/owner/对账/metrics 必要且后端可实现；但“用户查看展示”与“人工入口”明确另立 `REVIEW-REQUIRED-ENTRY-01`，**AC-72 不能整体 AC-VERIFIED**。影响分析对此诚实；需 PO/Reviewer 在台账登记 AC-72 的分段状态。 |
| **新增是否“为扩而扩”** | **部分** | AC-69/70 可视为 AC-50 的收窄/实现化；AC-71 是 TCC 必需；AC-72 的运营/UI 部分应拆分（已拆 `REVIEW-REQUIRED-ENTRY-01`）。**不建议**再把 AC-72 的指标/SLA 折回 AC-50/62；保留但标注为运维 AC。 |

## R4.6 影响分析（impact DRAFT 0.2）

| 项 | 判定 | 缺口 |
|---|---|---|
| schema/迁移/回填 | **PASS** | 0 迁移、无 0024、无回填；`REVIEW` 两列均容纳；`MANUAL_HOLD` 已有 CHECK；impact §2/S1/S3/S4。 |
| TCC | **PASS** | `§3` 已列 `tcc_settlement.go`/`tcc_admission.go` 的 legal/app/MANUAL_HOLD 改动；`last_error`；`TCC_BATCH...`；与 PO §11 一致。 |
| 部分金额 | **PASS** | `§4` 三分法、无 `CancelIfCreated`、投影滞后 fail-closed；与 D-31/INV-52 一致。 |
| 幂等/回滚 | **PASS** | `§5` existing 前移、REVIEW 不探测、Tx2 CAS；`§7` 旧代码回滚审计；诚实。 |
| 可观测性 | **PASS** | metrics gauge/既有 tick/告警规则；用户可见/入口另立；`§6.1/6.4`。 |
| 测试/证据矩阵 | **PARTIAL** | 文件级矩阵和 O1b 走 `TccCancelMarkPaid` 已列；但 **AC-69 普通售后/AC-69 原复核记录/AC-72 用户可见** 的证据仍缺；T5 未交付（声明）。 |
| BFF/前端/gateway | **PASS（边界）** | 明确不在本 change；`REVIEW-REQUIRED-ENTRY-01` 另立；无遗漏迁移。 |
| **D-2 普通售后无 `refund_applications`** | **OPEN / BLOCKER** | impact `:325` 自标 `[假设]`“需 QC/PO 确认”；ADR §12.3 按“无行”处理，**SPEC AC-69/S20 未 carve-out**；R4-B1。 |
| **D-8 原复核记录** | **OPEN / BLOCKER** | impact `:326`/`REVIEW-AUDIT-01` 把行级历史 DEFERRED；SPEC AC-69 `:456`/AC-72 `:488` 要求保留；R4-B3。 |
| **D-6 用户可见/入口** | **OPEN / MAJOR** | `REVIEW-REQUIRED-ENTRY-01`；诚实，但 AC-72 不能整体验收；需 PO/Reviewer 分段。 |
| **D-7 last_error 归属** | **PASS（本 change 范围）** | `refund_batches.last_error` 列存在；`tcc_reconcile` 不清 batch；TCC-LAST-ERROR-01 剩余 DEFERRED。 |

## R4.7 5 个崩溃窗口 + 重复/乱序

| 窗口 | 0.4 覆盖 | R4 |
|---|---|---|
| 本地提交后崩溃 | W1 | **PASS**；Tx1 写入/REVIEW 重放。 |
| RPC 成功后崩溃 | W2/W6 | **PASS**；`<NO_WAYBILL>` 重放 REVIEW；`CANCELLED`/POST。 |
| 消息重复/乱序 | W9/W5/W12 | **PASS**；consumer no-op、重复 PRE、order.paid 真实 placeholder、TCC MANUAL_HOLD。 |
| 取消/确认竞态 | W4 | **PASS**；部分金额不再调 `CancelIfCreated`。 |
| 外部重放缺失 | W8 | **PASS**；发起方 FAILED + ops + AC-27-LIVE-01；TCC MANUAL_HOLD 可发现。 |
| 部分金额崩溃 | W3/W10 | **PASS**；PARTIAL 零写入；REVIEW 1 commitment。 |
| 人工 re-arm | W11 | **PASS**；版本化 re-arm，不得自动。 |

## R4.8 关闭表（R3 → R4）

| QC 项 | 0.4 判定 | 说明 |
|---|---|---|
| R3-B1 TCC 早失败 | **CLOSED（设计）** | PO D-30/AC-71/R-9；`tcc_settlement.go`/`tcc_admission.go` allowed；O1b 走 `TccCancelMarkPaid`。 |
| R3-B2 部分金额取消运单 | **CLOSED（设计）** | D-31/R-2b；不调 `CancelIfCreated`；O4b `cancelCalls==0`。 |
| R3-B3 canonical 授权/CONFLICT 值 | **CLOSED（值）/OPEN（SPEC 同步）** | PO v1.11 D-30 用 `REVIEW_REQUIRED`；0.3 的 CONFLICT 值撤回；SPEC-0001 注 `AC50-OUTCOME-SPEC-SYNC` DEFERRED；实现前需 v1.11 approved。 |
| R3-B4 可重试 vs 不再探测 | **CLOSED（协议）** | SPEC §5.10/ADR §7.3 明确“不收敛、人工 re-arm/新 refund_id”；AC-50:228 收窄。 |
| R3-M1 W9 取消 owner | **CLOSED（协议）** | Fulfillment 取消/ops 审计；`ORDER-PAID-PLACEHOLDER-01`。 |
| R3-M2 W9 partial 断言 | **CLOSED** | W9 区分整组 PRE/部分 §3.4。 |
| R3-T1/T2/T3 | **CLOSED** | §5.10/A8/§8.3。 |
| M-1 legacy | **PARTIAL（DEFERRED）** | AC-70 legacy 三分法；审计/expiry 2026-12-31。 |
| M-2 部分金额 | **CLOSED（规则）** | D-31；见 R4.3。 |
| M-3/M-4/M-5/M-7 | **CLOSED（规则）/DEFERRED** | AC-62/T3、A2 双条件、审计、TCC last_error；§1.3。 |
| M-6/M-8/M-9/M-10/M-11/M-12/M-13 | **CLOSED** | TCC 测试路径/W9/A8/O5/consumer/可达性/setup。 |
| R2-M1 late order.paid | **CLOSED（协议）/DEFERRED** | W9 真实；`ORDER-PAID-PLACEHOLDER-01`。 |
| R2-M2/M3/M4/M5 | **CLOSED** | 审计 `LEFT JOIN`、§1.3、无预读、A8。 |

## R4.9 新增 `[BLOCKER]`

### `[BLOCKER]` R4-B1 — SPEC AC-69/S20 要求普通售后写 `refund_applications`，但普通售后没有该行；impact D-2 仍未决

**证据**

- SPEC v1.11 S20 `0002:87`：“发起方 `refund_applications.admission_result` 抄同一值、申请 `state` 保持 `PENDING`”；AC-69 `0002:449` 同。
- 实际代码/impact：普通售后使用 `after_sale_refund_tasks`，`refund_applications` 只在 Order 的 zero/TCC/支付级路径写（`refund_admission.go:498`、`tcc_settlement.go:789`）；impact `:325` 明确标 `[假设]`“普通售后没有 `refund_applications` 行……需 QC/PO 确认（D-2）”；ADR 0.4 §12.3 按“无行”处理，**未创建该行**（也不应双写）。
- 结论：SPEC 的“发起方 `refund_applications`”对普通售后不可执行；ADR 0.4 的“无行”是**自行缩小 AC**，没有 PO carve-out。AC-72 的“申请 `REFUND_ADMISSION_CONFLICT`”/队列可读，但 S20/AC-69 的持久镜像条款不成立。

**具体修改**

- PO 必须在 SPEC v1.11 增补：“IF 发起方使用 `refund_applications`（TCC/支付级等），THEN 写 `admission_result=REVIEW_REQUIRED`、`state=PENDING`；IF 普通售后使用 `after_sale_refund_tasks`，THEN 以该 task 的 `PENDING` + `last_error`（含 `REFUND_ADMISSION_CONFLICT`）作为发起方镜像，不得为普通售后新建 `refund_applications` 行”；同步 S20/AC-69/AC-72。
- 在 ADR 0.4 与 impact 删除/替换 `[假设] D-2`，登记 owner/expiry；在 QC/Reviewer 测试矩阵中增加普通售后（无 `refund_applications`）的 `TestAC69` 分支。
- 未完成前，AC-69/S20 不得标 AC-VERIFIED；实现不得新建该行（会双写事实）。

### `[BLOCKER]` R4-B2 — S20/INV-51 未排除零元“不持久化 `REVIEW_REQUIRED`”，与 AC-67/AC-69 直接矛盾

**证据**

- S20 `0002:87`：“**每个 `refund_id` 一条裁决**：`PENDING_REMOTE`/`PRE`/`POST`/`REVIEW_REQUIRED`”。
- INV-51 `0002:142`：“无法按 AC-50 合法分支判定的组级准入 SHALL 以 `REVIEW_REQUIRED` 持久记录”。
- AC-69 `0002:448`/AC-67 `0002:429`：零元路径**不持久化** `REVIEW_REQUIRED`；`refund_admission.go` 零元无 Tx1/无承诺；ADR 0.4 §4/§7.4 同。
- 结论：零元属于“无法判定”的组级退款，但被 S20/INV-51 的字面要求覆盖；SPEC 没有“零元除外”，与 AC-67/69/ADR 的零元例外冲突。EARS 无法同时验收 S20/INV-51 与 AC-67/69。

**具体修改**

- S20/INV-51 增加“**零元路径除外**：零元无 Tx1/无承诺/无 Payment，SHALL NOT 持久化 `REVIEW_REQUIRED`；重放重新探测，按 AC-67/AC-72 有限重试/人工队列”；删除“每个 refund_id”的绝对措辞或明确“持久化路径的每个 refund_id”。
- 在 `E32` 与测试 `TestAC69` 增加零元分支；`AC-72` 的 `REVIEW` 计数查询不得把零元计入；`§5.10` 状态机注明零元不进入 S20。
- 需要 PO 修订 SPEC（B2）；未修订前不能按 S20/INV-51 全量验收。

### `[BLOCKER]` R4-B3 — AC-69/AC-72 要求保留原复核记录，ADR 0.4 却将其 DEFERRED 为 `REVIEW-AUDIT-01`

**证据**

- SPEC AC-69 `0002:456`：“WHEN 复核项收敛… SHALL 保留原复核记录、原因、证据与处置人，不得删除（对账与审计依据）”；AC-72 `0002:488` 同“保留原复核记录用于审计”。
- ADR 0.4 §8.3 `:523`：“admission 行按状态机迁移，**行内不保留历史值**；审计依据为 ops 工单 + 结构化日志 + `last_error`；若 PO 要求行级历史/独立审计表，属新表 → 另立 L2 `REVIEW-AUDIT-01`，不得由 Developer 夹带。”
- impact `:326`/`:344` 把 `REVIEW-AUDIT-01` 列为 DEFERRED（D-8）。**PO 已经在 v1.11 AC-69/72 要求**，不是“若 PO 要求”；ADR 把已批准的 AC 条款降级为 DEFERRED，属于**自行缩小合同**，且 `REVIEW_AUDIT` 记录会随 `result` 回 PENDING 而丢失（`§8.3` 明确行内不保留）。
- 影响：AC-69/AC-72 无法 AC-VERIFIED；审计/对账无“原复核记录”载体；ops 工单/日志不是 AC 文本所要求的持久记录。

**具体修改（二选一，需 PO/人类）**

1. **实现原复核记录**：新增审计表/历史行/事件（需要 schema 或独立表）→ 由 Lead 另拆 L2；在 SPEC v1.11 的 AC-69/72 明确载体；在此之前不得声称 AC-69/72 可验收。
2. **降低 AC 文本**：PO 将 AC-69 `0002:456`/AC-72 `0002:488` 改为“保留复核处置审计（ops 工单/结构化日志/`last_error`；行级历史为 `REVIEW-AUDIT-01`）”，并明确不要求行内历史；ADR/impact 同步。**但这是 SPEC 文案修订，不能由 Architect 自行缩小。**

## R4.10 新增 `[MAJOR]` / `[TODO]`

- **`[MAJOR]` R4-M1 — SPEC §11 `:870` 仍写“ADR 仍为 DRAFT 0.3”**：实际被审 ADR 是 0.4；`docs/specs/0002...:870` 的同步指令 stale，容易让 Reviewer/Lead 审错对象。改为“ADR DRAFT 0.4（`8595c7c`）”并在 SPEC 变更记录登记；不涉及语义，但属证据绑定。
- **`[MAJOR]` R4-M2 — AC-72 的用户可见/人工入口是 DEFERRED**：`REVIEW-REQUIRED-ENTRY-01`（Lead/BFF/ops，2026-12-31）诚实，但 AC-72 `0002:484` 的“用户查看展示”与“人工入口”在本 change 不交付；AC-72 只能部分/后端 AC-VERIFIED，必须在 `ac-coverage.tsv` 分段登记，不得以“AC-72 后端 + metrics”整体通过。SPEC v1.11 已声明 in-review/DEFERRED，但需 PO/Reviewer 明确。
- **`[MAJOR]` R4-M3 — `tcc_*.go` 的 PO §11 授权不等于人类实现批准**：PO §11 要求 TCC 改动；ADR 0.4 allowed `tcc_settlement.go`/`tcc_admission.go` 是正确范围（不含 `tcc.go`/`tcc_payment.go`/`tcc_precheck.go`/`tcc_reconcile.go`）。但 **PO 不能替代** `ADR_APPROVED`、Lead 独立 `change_id`、Plan/Tasks、Reviewer 与人类 merge；实施 Brief 必须显式记录；不得以“PO §11 已授权”启动写码。
- **`[TODO]` R4-T1** `tcc_reconcile.go:340` 只清 `refund_applications.last_error`，不写/不清 `refund_batches.last_error`；ADR 0.4 写 batch 列，正常路径无冲突。补测试断言 `refund_batches.last_error` 在 `MANUAL_HOLD` 后不被 `tcc_reconcile` 清空（若未来 reconcile 触及该列）。
- **`[TODO]` R4-T2** `AC-72` 的 5 分钟告警规则、metrics 刷新频率、`REVIEW_REQUIRED-ENTRY-01` 的 BFF/入口均未实现/验证；impact §6 已列，但 `deploy`/`metrics` 的真实执行证据需 Developer/ops 在实施阶段补，不能以 SQL 演练替代。另需确认 `order/worker.go` 的既有 tick 不会被 `MANUAL_HOLD` 阻塞。

## R4.11 最终判定与实现门禁

**BLOCKED。**

- **0 BLOCKER 才 READY**：本报告有 **3 个 `[BLOCKER]`**（R4-B1/B2/B3），**不能 READY**。
- **是否可开始实现：否。** 即使修复上述 BLOCKER，仍必须先满足：
  1. **SPEC-0002 v1.11 从 in-review 变为 approved**（当前 header/评审状态；Director 授权 PO 升版不等于 approved）；
  2. **ADR 0.4 获人类 `ADR_APPROVED`**；`REVIEW-REQUIRED-ENTRY-01`/`REVIEW-AUDIT-01`/`ORDER-PAID-PLACEHOLDER-01`/`TCC-LAST-ERROR-01`/`LEGACY-SHIPPED-PRE-01`/`INV-44-MULTI-01` 等 DEFERRED 有 owner/expiry/stop；
  3. Lead 按 `0001:879` **另拆独立 `change_id` + Plan/Tasks + AC 证据矩阵**，并明确 `tcc_*.go` 的人类批准；
  4. 本报告的 R4-B1/B2/B3 关闭、R4-M1/M2/M3 处置，QC/Reviewer 再复核。
- **可交人类**：0.4 的技术方向（REVIEW 表示/无迁移/TCC/部分金额/幂等/崩溃）已可交给人类做**原则批准**，但在上述 3 个合同缺口关闭前**不得开始实现**；也不得声称 AC-69..72/AC-72 用户可见/原复核记录已验收。
- **AC 状态**：AC-69..72 仍 `DEFERRED`/AC-REF；AC-27 仍 AC-REF；`REVIEW_REQUIRED` 不构成准入成功；无 Release/merge。

**只读声明**：未执行 `go test`/`-race`/MySQL/迁移；只审 v1.11/0.4/impact；未改 SPEC/ADR/代码/测试/台账；只追加本 R4 小节；未 commit。

**R4 Actor**：`qc-ac50`
**R4 Reviewed-by**：待 PO 修订 SPEC / Architect 修订 ADR / 人类批准
**R4 Status**：**BLOCKED — 3 BLOCKER；不得开始实现**
