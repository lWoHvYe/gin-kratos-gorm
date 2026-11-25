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
