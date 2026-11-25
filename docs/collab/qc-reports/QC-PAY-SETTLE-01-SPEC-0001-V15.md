# QC-PAY-SETTLE-01-SPEC-0001-V15 — SPEC-0001 v1.5 delta 红队复核（AC-27 读模型 / AC-27A 关闭谓词 / AC-50 `CONSUMED+无运单`）

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| 任务 | L2 `PAY-SETTLE-01`：对 SPEC-0001 v1.5（PO 工作树改动，未 commit）做 delta 红队复核 |
| 审查对象 | `docs/specs/0001-fulfillment-quantity-split.md` v1.5（工作树 blob `efad166ffd91d42a9bd95d638945dd6c831a7d2c`；`git diff` vs `06e043e`） |
| 结论 | **BLOCKED**（3 个 `[BLOCKER]` + 9 个 `[MAJOR]` + 5 个 `[TODO]`；v1.5 不得进入实现） |
| 日期 | **2026-11-26** |
| Actor | `qc-po-v15`（Quality Checker；与 PO、Architect `arch-r04-r3`/`arch-ac27`、Developer、Reviewer 不同 actor） |
| base | `06e043e50cdbe05b53991b43e2bc49e1f72d788b` |
| head | `caaf3512d974438cae36457da3130af735d659d6`（`git rev-parse HEAD`；审查期间 HEAD 由 `06e043e` 前进到 `caaf351`，被审 SPEC blob 未变，仍为 `efad166…`；v1.5 为**未提交工作树**，见 M-9） |
| Tier | **L2**（Spec 修订：状态机/退款准入/许可/读模型；跨服务 S5/S6/S9 语义） |
| review_scope | 只审 SPEC-0001 v1.5 的 delta：YAML/§4.2/§6.6.1/§6.7/§7.1/§7.5/§9 AC-25…27A/§13/§14/§16；只读交叉核对 SPEC-0002 v1.10（S4/S5/S6/S9、INV-01/44/45/50、AC-48/50/62/65）、ADR-0007 Rev 2.3 §1.1/§5.2/§5.7、ADR addendum r03、`docs/implementation/PAY-SETTLE-01-R0.3-selfcheck.md` §2.2、代码与迁移。**不审**审查期间由其他 actor 提交的 `docs/adr/0007-amd1-addendum-r0.4-c04-release.md`（Actor `arch-r04-r3`，独立 DRAFT r3，`d5dda12`）、`docs/lead/PAY-SETTLE-01/evidence-matrix.md` 的 INV-44 行（`caaf351`），也不审 SPEC-0002/ADR 正文修订本身。 |
| release_status | **NOT RELEASE / NOT RELEASABLE**；不批准 merge；不声称任何测试已执行；不声称 AC-VERIFIED；本报告不代替人类批准。 |

## 0. 结论与只读声明

**BLOCKED。** v1.5 正确关闭了前序 QC 的核心冲突（`QC-PAY-SETTLE-01-R0.3-AC27` 的 B-1：持久化 `orders.status` 不被组退款写入），并把 AC-27 EARS-1 的“推进/不得停留”明确限定为 S2 读模型；AC-50 `CONSUMED+无运单` 的 CONFLICT 口径、`refund_state`/`reservation_state` 不参与关闭、pre-ship `REFUNDING` 可关闭但≠退款完成，均与 SPEC-0002/ADR 的方向一致。**但 v1.5 仍不能通过：**

1. **B-1**：AC-27A 的“无运单事实”IF 把**准入写路径**的错误码/告警字段写进了**读模型谓词**，并要求 Order 观测 S6 运单事实——与 AC-27A 自己的“只用 Order 自有事实/不得反推运单状态”直接矛盾；且无条件返回 `REFUND_ADMISSION_CONFLICT` 越过了 AC-50 的“部分金额先返回 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`”分支。没有指明触发操作，AC 不可验收。
2. **B-2**：AC-27 EARS-2 把持久化 `orders.status` 的合法值枚举写成 `PAID/ALLOCATED/SHIPPED/COMPLETED`，**漏了 `CANCELLED`**；而未支付取消与 AC-51/AC-65 的 TCC 路径都会留下“组已关闭 + `orders.status=CANCELLED`”的合法共存态，AC 正文与之直接冲突。
3. **B-3**：AC-27A 第 5 条（`CONSUMED+PRE_SHIP_REVOKED` → 已关闭）与“数据审计/核验”IF 没有可核验的证据载体，也没有定义 `CONSUMED+无运单` 异常的人工纠正动作与终态：合法的 `CONSUMED+CANCELLED` 分支可能**永久不关闭**（功能死胡同），异常分支则可能被 PRE 正规化；v1.5 声称的“修正前读模型不得关闭”无法在 AC 内表达为可执行规则。

**只读声明**：未执行 `go test`/`-race`/MySQL/迁移；文中代码、迁移、SPEC/ADR 引用均为阅读事实，**不构成执行证据**；未修改 SPEC/ADR/代码/台账，只新建本报告；未 commit。数据分布类判断标 `[不确定]`。

---

## 1. 前序问题关闭表

| 前序问题 | v1.5 处置 | 证据 | 残余 |
|---|---|---|---|
| **B-1**（`QC-PAY-SETTLE-01-R0.3-AC27`）：不得由 Architect 单方把“推进订单投影”解释为写 `orders.status` | **CLOSED（契约口径）**：V15-1 明确为 S2 读模型；§6.6.1/§7.5/AC-27 EARS-2 三处一致 | `0001:297-298`、`0001:590-593`、`0001:417`、`0001:808` | ADR r03 的 **R-2**（在退款完成 consumer 内写 `orders.status`）必须撤销/改写，`0001:815` 已指出；本报告不审该 ADR |
| **B-2**：`PRE_SHIP_REVOKED` 被当作“运单已取消”的代理，漏掉 AC-50 的 `CONSUMED+无运单` 异常 | **PARTIAL**：AC-50 CONFLICT 保留、禁止写 PRE、告警字段已写；但读/写路径未拆、证据载体缺失、修复范围不全 | `0001:294`、`0001:613-615`、`0001:810`；`0002:223` | 见 **B-1/B-3/M-4**；AC-27 EARS-1 在本修正落地前不得声称收敛（`0001:816` 已声明，但 AC 正文未声明） |
| **M-4**（多组释放缺口） | 未由 v1.5 处置；§7.5 注 3 反而暗示有“INV-44 释放/告警流程”可跟踪 | `0001:421`；`internal/services/order/refund_release.go:182-186` | 见 **M-5**；2 组订单当前不发释放请求，AC-27/27A ≠ INV-44 已完成 |
| **M-5**：pre-ship `REFUNDING` 关闭缺 PO 授权 | **CLOSED**：AC-27A 明确 `refund_state` 不参与、pre-ship `REFUNDING` 可关闭但≠退款完成；V15-2 说明 v1.4“只覆盖 `REFUNDED`”仅指 EARS-2 条件 | `0001:621-623`、`0001:809` | 与 AC-48/AC-50/INV-45 不冲突（见 §4） |
| **M-6/R-5**：§7.5 示例与 AC-27 冲突；Architect 越权指示 QC/Reviewer 不得引用 §7.5 | **PARTIAL**：§7.5 行已按读模型修正，且 v1.5 未再出现“不得引用”指令；但 §7.1/§7.2 的 S1/S3 口径仍有新矛盾 | `0001:404-423`、`0001:377` | 见 **M-2/M-3** |

---

## 2. 必答崩溃窗口（本 delta 范围）

| # | 窗口 | 预期行为 / 恢复 owner | 依据与缺口 |
|---|---|---|---|
| 1 | **本地提交后崩溃**（本地事务已提交、跨服务调用未发出/未受理） | 关闭谓词本身是**纯读派生**，无本地写 → **N/A**。若实现 AC-50 修复：`AdmitRefund` Tx1 已写 `PENDING_REMOTE`、`CancelIfCreated` 未发出即崩溃 → 读模型按 AC-27A 第 6 条“未关闭”；恢复 owner 是**发起方 AfterSales 的 durable refund task**（按 `refund_id` 重入 `AdmitRefund`），不是 Order（Order 无扫描 `PENDING_REMOTE` 的 worker）。 | 读派生：`internal/services/order/settlement.go:67-77`、`refund_admission.go:566-608`；恢复：`internal/services/aftersales/workers.go:390-404`（只读事实）。缺口：v1.5 未在 AC/§16 指认该 owner；见 **M-8**。 |
| 2 | **跨服务 RPC 成功后崩溃**（`CancelIfCreated` 已成功、Order 未记录结果） | 运单 `CANCELLED` 已提交、Tx2 未提交 → admission 停留 `PENDING_REMOTE`，AC-27A 第 6 条**不关闭**；重试按 `refund_id` 幂等：再次探测到 `CANCELLED` 行 → 写 PRE；若 `CONSUMED+无运单`，修复后应返回可区分的“无运单”结果且**不写 PRE**。释放请求只在 PRE 之后由 `appendRefundReleaseTx` 产生，重复投递由 `release_id`/`RELEASED` no-op 收敛。 | `cancel_if_created.go:32-88`；`refund_admission.go:566-608`；ADR Rev 2.3 §5.2.4/§5.8（`0007:591-608`）。缺口：v1.5 未要求“以重试时实际行状态重判、不得复用首次 reply”；且“无运单”不可区分（**B-1/M-4**）。 |
| 3 | **消息重复/乱序** | 重复 `aftersale.completed`：`refund_state` UPDATE 幂等；关闭谓词纯读，重复/乱序读结果一致。重复 shipment 事件：组状态 CAS 守卫阻止回退（`CANCELLED` 不被覆盖）。**乱序风险**：`PRE` 之后若迟到一条 `shipment.shipped`，目标 `SHIPPED` 的守卫允许 `ALLOCATED→SHIPPED`，会把组重新写成 `SHIPPED`，AC-27A 第 3 条随即判“未关闭”→读数从已关闭翻回未关闭。 | `internal/services/order/consumer.go:96-101`、`aftersale_consumer.go:39-56`。该异常要求“PRE ⇒ 运单从未离仓”（`CancelIfCreated` 在运单行 CAS），正常不会出现；但 v1.5 未定义**读模型是否允许回退、由谁告警**（§7.5 注 3 只覆盖“组 CANCELLED 与运单 SHIPPED 并存”）。见 **M-2** 的缺口说明。 |
| 4 | **取消/确认竞态** | 由 AC-48/AC-50 裁决，不是本 delta 新增机制：Ship 与退款取消在运单行 CAS，唯一胜者。Ship 先胜 → `ALREADY_SHIPPED`/`RECEIVED` → `POST_SHIP_ALLOWED`，不取消、不回滚；取消先胜 → 运单 `CANCELLED` → PRE，且**仅在此后**按 INV-44 释放；许可 `CONSUMED` 保持终态。关闭谓词纯读，不参与写裁决，不新增资金/库存副作用。 | `0002:219-222,230`；`cancel_if_created.go:63-88`；`refund_admission.go:576-584`。**N/A 依据**：AC-27A 只定义“关闭”的读模型判定，实际胜者仍归 AC-48/AC-50；唯一残余是 `CONSUMED+无运单` 异常（**B-1/B-3**）。 |
| 5 | **外部重放缺失** | 关闭谓词本身不依赖外部渠道 → 对谓词是 **N/A**。但 **AC-27 的 liveness 不是 N/A**：若 AfterSales 的 durable refund task 进入 DLQ/租约永久丢失，`CONSUMED+PENDING_REMOTE` 组会永久不关闭，订单/用户侧呈现“组无可发、也未关闭”的死组；v1.5 只写“未关闭并告警”，没有人工队列、owner、SLA/expiry 或“重放仍冲突后怎么办”。 | `0001:613-620`；`0002:303`（AC-61 提供 `payment_no/refund_id/after_sale_no` 人工重放入口，但 v1.5 未引用）；`internal/platform/consumer/consumer.go:69-70`（`MaxDeliver=20`）。见 **M-8/B-3**。 |

---

## 3. 新发现

### 3.1 `[BLOCKER]`

#### B-1 — AC-27A 的“无运单事实”IF 没有可执行的操作边界与事实所有者，且错误码越界

- **证据**
  - `0001:602-603`：AC-27A 开头限定“**只使用 Order 自有事实**（组投影状态、许可、准入结果）”。
  - `0001:613-615`：同一 AC 的 IF 要求“`许可=CONSUMED` 且该组**无运单事实**（无运单行/查无运单）→ 返回 `REFUND_ADMISSION_CONFLICT` 并告警”，告警必须含“分支原因（**无运单行**）、是否已写 PRE”。
  - `0001:626-627`：AC-27A 又要求 Order“**不得制造、回写或据以反推运单状态**；运单 `CANCELLED` 只通过 AC-50 的准入结果代理进入”。
  - 事实/代码：Order 无 S6 存储（`internal/services/order/ports.go:49-54`）；读模型加载只取组/许可（`internal/services/order/fulfillment_groups.go:84-95`）；关闭谓词只读 `Status/PermitState`（`internal/services/order/settlement.go:67-77`）。**只有准入写路径**（`finishRemoteAdmission` 调 `CancelIfCreated`，`refund_admission.go:566-597`）能观测“无运单行”；读路径无法知道，也无法“返回 `REFUND_ADMISSION_CONFLICT`”（这是准入错误码，`0002:223`）。
  - 错误码条件：AC-50 **先**在 `0002:217` 要求“未离仓且本次金额 < 整组 → `PARTIAL_REFUND_REQUIRES_FULL_GROUP`”，**再**在 `0002:218-223` 的“金额 = 整组”分支列 `CONSUMED+无运单 → REFUND_ADMISSION_CONFLICT`。AC-27A 的 IF 无金额条件，等于对部分金额也返回 CONFLICT，覆盖了 AC-50 的前置分支。
- **具体改法（PO）**
  1. 把 `0001:613-615` 拆成两条：**写路径**（引 AC-50 口径）——“WHEN `AdmitRefund` 的 `CONSUMED` 分支且本次金额 = 整组退款金额，IF Fulfillment 的取消探测返回**可区分的“无运单行”结果**（不得复用 `CANCELLED`），THEN Order SHALL 返回 `REFUND_ADMISSION_CONFLICT` 且不写 PRE；告警含 `order_no/group_no/refund_id`、分支原因、是否已写 PRE”；**读路径**（AC-27A）——“IF 某组的 `CONSUMED+PRE` 证据不可核验，THEN 不判关闭并告警；只允许使用 Order 自有事实，不得要求读路径输出 `REFUND_ADMISSION_CONFLICT` 或‘无运单行’原因”。
  2. 明确 `REFUND_ADMISSION_CONFLICT` 只在“本次金额 = 整组退款金额”分支成立；部分金额先按 `0002:217` 返回 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`。
  3. 明确“可区分”是 **Fulfillment 的契约义务**（见 M-4：ADR Rev 2.3 §5.3 的 `CancelIfCreatedReply` 枚举没有该结果，必须 amendment）；否则该 AC 在现有契约下不可实现。
- **影响**：不修则开发者只能二选一——让 Order 读/反推运单（违反 S6 所有权、AC-27A 自身与 `0002:76`），或按 PRE 直接关闭（把 AC-50 异常正规化）。这正是前序 B-2 未闭合的根因。

#### B-2 — AC-27 EARS-2 的持久化生命周期枚举漏了 `CANCELLED`，与 AC-25/§7.1/AC-51 及正常取消冲突

- **证据**
  - `0001:590-593`：EARS-2 “WHILE G2 已关闭 … THE SYSTEM SHALL 保持持久化 `orders.status` 为其既有合法生命周期值（`PAID`/`ALLOCATED`/`SHIPPED`/`COMPLETED` 中其既有值…）”。
  - `0001:112`（S1 允许 6 值）与 `0001:129`（AC-25）明确生命周期含 `CANCELLED`、且“退款态不得落生命周期”。
  - 可达反例：未支付取消把订单与所有组同时置 `CANCELLED`（`internal/services/order/service.go:453-464`、`:547-557`；组写入 `fulfillment_groups.go:71-80`）→ 按 AC-27A 第 2 条组“已关闭”，AC-27 EARS-2 的 WHILE 条件成立，但 `orders.status=CANCELLED` 不在枚举内；AC-51/AC-65 的 `PENDING_PAYMENT` 已捕获路径同样先 CAS `CANCELLED` 再逐组准入（`0002:236`、`0002:377`），随后组进入 `REVOKED/PRE`（关闭）。
- **具体改法**：`0001:590-593` 的括号改为“`PAID`/`ALLOCATED`/`SHIPPED`/`COMPLETED`/`CANCELLED` 中其既有值”，并注明“`PENDING_PAYMENT` 不适用：AC-51/AC-65 在组准入前已 CAS 到 `CANCELLED`”；或写成“任一既有合法生命周期值（含 `CANCELLED`），不得写入退款态”。补一条“未支付取消/TCC 已捕获 → `CANCELLED` + 组关闭”的验收用例。
- **影响**：EARS-2 是 AC-27 的核心条款；按字面执行会让合法取消态被判失败，或迫使测试人员跳过该场景，AC-27 不可判定。

#### B-3 — AC-27A 第 5 条与“数据审计/核验”IF 无可核验证据载体，且 `CONSUMED+无运单` 异常无人工终态（死胡同）

- **证据**
  - `0001:608-610`：第 5 条以“存在 `order_refund_admissions.result=PRE_SHIP_REVOKED`，且该结果按 AC-50 **只能由** `CONSUMED+CREATED→CANCELLED` 或 `CONSUMED+CANCELLED` 分支写入”作为关闭条件。
  - `0001:616-618`：IF 数据审计/核验发现 PRE 不是由上述合法分支写入（“含 `CONSUMED+无运单`、历史数据、证据缺失或无法核验”）→ 不据此关闭并告警。
  - 持久载体：`migrations/order/0016_settlement_admission.sql:8-19` 的 `order_refund_admissions` 只有 `refund_id/order_no/group_no/result/state/amount_cent/version/created_at/updated_at`，**没有**“取消探测分支/观测到的运单状态”列；写路径也只写 `result/state`（`refund_admission.go:576-597`）。因此读模型无法区分“合法 PRE”与“`CONSUMED+无运单` 的异常 PRE”。
  - 终态：AC-27A 只说“人工核对后按 AC-50 修正”，未定义队列、owner、期限或允许的纠正动作（补建缺失运单？撤 `CONSUMED` 许可？在证据补齐后写 PRE？退款完成？）。而 `CONSUMED` 许可按 INV-45/AC-62 不可恢复、该组又尚无 PRE，导致组既不能发也不能关。
- **具体改法（PO，二选一或组合）**
  1. **给出可核验证据的业务事实**：要求 `AdmitRefund` 在写 admission 时持久化“取消探测观测”（如 `NO_WAYBILL` / `WAYBILL_CANCELLED` / `CREATED_CANCELLED`）作为 Order 自有的审计事实（属新业务事实，ADR/迁移由 Architect 设计）；或
  2. **给出存量裁定**：明确“修复上线后由 AC-50 合法分支写入的 PRE 视为已核验；存量 PRE 由一次性人工审计（owner、SLA/expiry）处理，审计前不关闭并告警”，并把“人工核对”的队列、owner、期限、允许动作（必须不违反 INV-45）写清楚。
- **影响**：不修则合法的 `CONSUMED+CANCELLED` 分支（`0002:222`）永远无法关闭 → 订单永久 `PARTIALLY_SHIPPED`，正是 AC-27 要消除的“卡住”；或者实现直接信任 PRE，把 `CONSUMED+无运单` 异常静默正规化（前序 B-2 复发）。

### 3.2 `[MAJOR]`

#### M-1 — §4.2 用错跨 Spec 状态编号：`refund_state` 在 SPEC-0001 是 **S4**，不是 S9

- **证据**：`0001:134`“不得用它替代 S5/S9”；但 `0001:115` 的 S4 = 履约组退款态（`NONE/REFUNDING/REFUNDED`，即 `refund_state`），`0001:120` 的 S9 = 售后流程状态（AfterSales）；SPEC-0002 `0002:79` 的 S9 是“组退款进度”，并明确“**沿用 SPEC-0001 S4**”。
- **改法**：`0001:134` 改为“不得用它替代 S5/S4（组退款态 `refund_state`；SPEC-0002 编号为 S9）”；AC-27A/§16 中凡指 `refund_state` 统一写 S4，避免与 AfterSales 的 S9 混淆。
- **影响**：按 SPEC-0001 的字面读，S9 是 AfterSales 的状态，测试/评审会把“不得替代 S9”误读为与售后流程的关系，漏掉真正的“退款态不得作为关闭判据”。

#### M-2 — 组持久化 `CANCELLED` 的写者/读模型口径未统一（§4.1 S3 vs §7.2 vs §7.5 注 2 vs AC-27A 第 2 条）

- **证据**：`0001:114`（S3“谁可以改变”含“**退款撤许可**”）；`0001:384`（§7.2 把 `CANCELLED` 标为“**读模型**”）；`0001:419`（§7.5 注 2“持久化组 `status` 可以仍为 `ALLOCATED`”，`G2 CANCELLED` 是 `effectiveGroupStatus`）；`0001:605`（AC-27A 第 2 条“组**持久化状态** = `CANCELLED` → 已关闭”）。代码事实：退款路径不写组 `status`（`refund_admission.go:256-288` 只写 admission/permit/commitment）；持久化 `CANCELLED` 只由订单取消 `setFulfillmentGroupStatus` 写入（`service.go:453-464`、`:547-557`、`fulfillment_groups.go:71-80`）。
- **改法**：明确“S3 的持久化 `CANCELLED` 只由整单取消写入；退款撤许可只改 S5，不改 S3；读模型用 `effectiveGroupStatus` 派生 `CANCELLED`”；同步修正 `0001:114` 的“谁可以改变”与 `0001:384` 的“（读模型）”标签，区分“持久化 CANCELLED”与“派生 CANCELLED”。另明确迟到 `shipment.shipped` 覆盖 `ALLOCATED→SHIPPED` 后是否允许把已关闭组“重新打开”（窗口 3）及告警 owner。
- **影响**：AC-27A 第 2 条依赖“持久化 = CANCELLED”却未定义其唯一写者；读者会以为退款会写组状态，违反 S3/S5 所有权。

#### M-3 — §7.1 新增的 S1 迁移集合声明与代码不符，§7.5“全部退款（未发）→ `ALLOCATED`”不是确定结论

- **证据**：`0001:377` 新注称生命周期“**只由真实发货/收货/取消事实迁移**”；但 §7.1 图的 `PAID --首组发货--> ALLOCATED`（`0001:371`）在代码中由 `shop.shipment.created.v1` 触发（`internal/services/order/consumer.go:58`：`bind(..., statusAllocated, statusPaid)`），而 `shipment.created` 只是运单建单（`internal/services/fulfillment/service.go:155-200`，状态 `CREATED`；§7.4 `0001:396` 的“发货”是 `CREATED→SHIPPED`）。因此“组/订单从未实际发货”也可能已是 `ALLOCATED`。`0001:414`“全部退款（未发）→ `ALLOCATED`”只有在曾建过运单时成立；若两组都在任何 `shipment.created` 之前退款，`orders.status` 应为 `PAID`。
- **改法**：二选一并写清（属业务/契约裁决）——(a) 承认 `PAID→ALLOCATED` 的触发是“首组运单创建（发货授权）”，修正 §7.1 图与 `377` 注的“真实发货”措辞；或 (b) 若业务要求“实际离仓才 `ALLOCATED`”，则代码与 §7.1 冲突，必须停下由 owner 裁决（不得用文档迁就代码）。同时把 `0001:414` 的生命周期列改为“`PAID` 或 `ALLOCATED`（取决于是否已创建运单）”或拆行。
- **影响**：v1.5 以“S1 只由真实发货/收货/取消迁移”为立论解释 §7.5，但该立论与实际实现不符；§7.5 的“组合投影”对全部退款行失效。

#### M-4 — AC-50 修复范围不完整：漏了 `system_zero` 路径，且需要 ADR §5.3 契约 amendment（不是纯代码修复）

- **证据**：`0001:810` 把偏差定位在 `cancel_if_created.go:51-55` 与 `refund_admission.go:576-584,590-597`；但**零元路径** `decideZeroRefundResult` 对 `CONSUMED` 的 `CANCELLED` 同样返回 `PRE`（`refund_admission.go:370-390`，`:385-386`），并由 `writeZeroRefundTx` 写 admission（`:430-520`）。ADR §5.2.5 明确零元走“与正金额相同的组级准入”（`docs/adr/0007-payment-settlement-admission.md:598-608`），因此 `CONSUMED+无运单` 的零元退款同样会被写成 PRE。另：ADR Rev 2.3 §5.3 的 `CancelIfCreatedReply` 枚举只有 `CANCELLED/ALREADY_SHIPPED/RECEIVED/TRANSIENT_ERROR`（`0007:611-620`，`:616`），没有“无运单”结果；仅改 Go 代码会让 accepted ADR 文本失真。
- **改法**：§16 V15-3 的证据/范围补上 `decideZeroRefundResult` 与零元 admission 写路径，并写明“修复必须覆盖 `AdmitRefund` 的全部来源（正金额/TCC/`system_zero`/legacy）”；同时要求 Architect 先对 ADR Rev 2.3 §5.3 做 additive amendment（新增可区分的“无运单”结果，或明示复用现有 RPC 的等价字段），再进入实现。
- **影响**：只修正金额路径时，零元单的异常仍会被写 PRE 并据以关闭组（AC-27A 第 5 条），前序 B-2 在零元链路复发。

#### M-5 — AC-27 EARS-3 / §7.5 注 3 仍把“预留释放/运单事实”写成系统可判定/可跟踪的义务，但 2 组订单没有释放请求、Order 不可观测

- **证据**：`0001:594-596`（EARS-3“WHEN G2 的预留已释放或运单已取消 … 判定该组合合法”）与 `0001:421`（注 3“`reservation_state=HELD` 不阻止关闭，但**必须由 INV-44 的释放/告警流程跟踪**”）。代码事实：多组订单 `appendRefundReleaseTx` 在 `groupCount != 1` 时直接 `return nil`，仅 `log.Warn`（`internal/services/order/refund_release.go:182-186`）；Order 不读 `reservation_state` 于关闭谓词（`settlement.go:67-77`），也不写 `RELEASED`（`refund_admission.go:256-288`）。ADR r03 §4.4 与 evidence-matrix 的 INV-44 行均已把多组释放登记为 **DEFERRED**。
- **改法**：把 EARS-3 明确为“**规范层面的合法性判定，不要求 Order 观测/断言预留或运单事实**”（与 ADR r03 R-3 一致），证据归 Inventory；把 §7.5 注 3 的“必须由 INV-44 流程跟踪”改为“多组订单当前不产生释放请求（P1/manual 已知缺口，台账登记）；AC-27/27A 不以 INV-44 完成为前提，释放证据归 Inventory 对账/台账，不得用 Order fixture 的 `RELEASED` 冒充”。补一条：不得以 AC-27/27A PASS 反推 INV-44 已完成。
- **影响**：不修则测试人员会写“fixture 塞 `RELEASED`”的伪断言（前序 selfcheck 已拒绝），或误以为释放已被跟踪。

#### M-6 — AC-27 EARS-1 未限定“2 组订单/只剩一个剩余组”，对 3 组订单会给出错误期望

- **证据**：`0001:586-589` 的 WHEN 只写“G2 已关闭且 G1 正常发货”，没有“2 组”或“其余组均已关闭”；但 INV-19 允许最多 3 个履约组（`0001:146`），§7.5 表是 2 组示例。对 3 组订单（G1 已发、G2 关闭、G3 未发），代码 `shipmentStatusForGroups` 正确返回 `PARTIALLY_SHIPPED`（`settlement.go:90-125`），而 AC-27 EARS-1 按字面要求 `SHIPPED`。
- **改法**：在 WHEN 中加“该订单为 2 组，或除 G1 外其余组均已关闭/已收货”；或把期望写成“按剩余组聚合：仅剩 G1 且已发 → `SHIPPED`”。同步在 §7.5 注 1 声明“本表以 2 组为例”。
- **影响**：AC 可判定性；3 组订单（INV-19 合法）会被误判。

#### M-7 — AC-27 EARS-3/AC-27A 对“运单事实”的处理仍有措辞级越权，且部分术语不是业务事实

- **证据**：`0001:626-627`“运单 `CANCELLED` 只通过 AC-50 的准入结果代理进入”与 `0001:608-610` 第 5 条一致；但 `0001:594` EARS-3 的 WHEN 仍以“运单已取消”为触发，`0001:611`/`:619` 用“准入结果 = `PENDING_REMOTE`”（`PENDING_REMOTE` 在 ADR §5.2.4 是**中间态/`state`**，`result` 枚举为 PRE/POST）。此外 `0001:608` 直接写表列 `order_refund_admissions.result`，`0001:419` 直接写函数名 `effectiveGroupStatus`，违反本 Spec 第 28 行“数据结构/实现不在本文件范围”的自我边界。
- **改法**：引入/引用业务术语“组退款准入结果（RefundAdmission，owner=Order）”并统一使用；把“准入结果 = `PENDING_REMOTE`”改为“准入尚未收敛（`state=PENDING_REMOTE`）”；把表列/函数名移到 §16 的“依据/证据”列（该节可引代码），规范性 AC 只写业务事实；EARS-3 的 WHEN 改为“已由 Inventory/Fulfillment 确认的释放/取消事实（本 Spec 不要求 Order 持有）”。
- **影响**：PO 越权写数据结构/实现名，且 `result`/`state` 混用会造成“PENDING_REMOTE 是否算 PRE”的验收歧义。

#### M-8 — 未定义“准入永久不收敛”的 liveness 与人工队列 owner/期限；AC-27 的“不得阻塞”没有前提

- **证据**：`0001:619-620` 只要求“取消结果未知/超时 → 未关闭并告警”；`0001:613-615` 只要求 CONFLICT+告警；没有 owner/SLA/expiry/重放后仍冲突的终态。若 `shop.aftersale.completed.v1` 或 AfterSales refund task 进入 DLQ 后不再重放（`internal/platform/consumer/consumer.go:69-70`，`MaxDeliver=20`；Order 无扫描 `PENDING_REMOTE` 的 worker），组会永久不关闭；AC-27 EARS-1 的“G2 不得阻塞 G1 发货/不得停留”在“G2 未关闭”时本不适用，但用户侧会看到“组无可发、也未关闭”的死组。SPEC-0002 AC-61 有人工重放入口（`0002:303`），v1.5 未引用。
- **改法**：在 AC-27A 增加 IF：“IF 准入/取消探测在 DLQ 或租约丢失后仍无法收敛，THEN 系统 SHALL 告警并提供 AC-61 的人工重放入口；owner=ops（重放）+ 发起方 AfterSales（重试）；在人工处置前保持未关闭，不得写 PRE/不得关闭”；并登记一条 owner/expiry 的延期或残余风险（若 PO 认为 AC-61 已足够，须在 AC 中引用并声明 AC-27 的收敛前提）。
- **影响**：否则“G2 不卡住订单”只对已收敛的准入成立，异常窗口无 owner，违反 fail-closed/可恢复原则。

#### M-9 — 证据/范围：v1.5 未 commit，且审查期间 HEAD 前进；`base` 与被审字节不能靠 head 定位

- **证据**：审查开始时 `git rev-parse HEAD` = `06e043e`，v1.5 为工作树修改（blob `efad166…`），同一工作树还含其他 actor 的 `docs/adr/0007-amd1-addendum-r0.4-c04-release.md`（Actor `arch-r04-r3`）与 `docs/lead/PAY-SETTLE-01/evidence-matrix.md` 改动；审查期间这些改动已分别提交为 `d5dda12`、`caaf351`，HEAD 前进到 `caaf351`，而 SPEC v1.5 **仍未 commit**（`git status` 只剩 `M docs/specs/0001-fulfillment-quantity-split.md`）。被审 blob 未变（`efad166…`），但 `head=caaf351` 已不指向被审 SPEC 字节；`git diff 06e043e..caaf351` 还混入 R0.4 文档提交。
- **改法**：实现前由 PO 单独 commit v1.5（带 `change_id/Actor/Reviewed-by/Tier` trailer），或由 Lead 在 tasks/evidence 中记录 `blob efad166…` 作为审查绑定；不得把 SPEC v1.5 与 ADR r0.4 r3 混在同一 change_id 交付。本报告仅对 `efad166…` 的 SPEC 字节负责。
- **影响**：交付证据绑定（宪法 Workflow 6 / AGENTS §2.6）；否则后续 Reviewer 无法证明审的是哪一版。

### 3.3 `[TODO]`

- **T-1** §16 的行号引用陈旧：`0001:811` 仍写“`AC-26:567-569`；`AC-27:571-579`”（v1.4 行号），v1.5 实际为 `581-583`/`585-599`；`§7.5:396-409` 已标“v1.4 行号”，但 AC-26/AC-27 未标。建议统一标 `(v1.4 行号)` 或改为 v1.5 行号；`AC-48:201-206` 建议核对（AC-48 起于 `0002:200`）。
- **T-2** `0001:818` 只要求 SPEC-0002 §11 同步 v1.4→v1.5；实际引用还有 `0002:26`（依据）、`:716`、`:724`、`:730`、`:778`（批准记录）。建议列全，或写成“SPEC-0002 全部 `v1.4` 引用”。
- **T-3** AC-25/AC-26 在本轮仍是行首 Gherkin（`0001:577-583`），而 v1.5 是版本升级且 AC-27 显式重述了 AC-26 的适用边界；按宪法《Specification Standard》，受影响 AC 应在升级时迁移为 EARS。若 PO 判定二者“未受影响”，请在 §13/§16 写明依据（否则登记为 EARS 迁移债）。
- **T-4** §8“异常分支穷举（每条都是验收项）”（`0001:427-455`）未新增本轮分支：`CONSUMED+无运单`、PRE 不可核验、`PENDING_REMOTE` 永久悬挂、PRE 后迟到 `shipment.shipped`。建议补 E26…E29 或在 §8 首行注明“状态/退款异常见 §9 AC-27A”。
- **T-5** §6.7 第 2 条（`0001:303`）“退款完成后该组呈现‘已关闭’”仍暗示关闭以退款完成为前提，与 AC-27A 的“pre-ship `REFUNDING` 可关闭”措辞不一致；建议改为“退款完成或 AC-27A 关闭谓词满足后，该组呈现‘已关闭’”。

---

## 4. 已验证成立 / 不构成冲突的部分

1. **EARS 与状态机**：AC-27（`0001:585-599`）与 AC-27A（`0001:601-629`）均使用 `WHEN/IF/WHILE … THE SYSTEM SHALL …`，无行首 Gherkin；AC-27A 的 6 步顺序清楚，`CANCELLED` 先、`SHIPPED/RECEIVED` 守卫次之、`REVOKED`、`CONSUMED+PRE`、其他兜底，与现有 `groupIsClosed`（`settlement.go:67-77`）的 3 步一致并补第 5 支；未新增持久化字段，符合 §4.2 第 5/6 条。
2. **读模型口径与持久化生命周期**：`§6.6.1:297-298`、`§6.7:304`、`AC-27 EARS-2:590-593`、`§7.5:417`、`§7.1:377` 一致地把“组关闭/退款完成”与持久化 `orders.status` 分离；“`shipment_status` 不是写回 `orders.status`”无歧义。**前序 B-1 已关闭。**
3. **AC-27A 的排除项**：`refund_state` 不参与（`0001:607,610,621-623`），`reservation_state` 不参与（`:624-625`），`PENDING_REMOTE`/`POST_SHIP_ALLOWED` 不构成关闭（`:611,619`），Order 不得写 `RELEASED`/制造运单（`:625-627`），与 INV-01/INV-44/INV-45 及 S4（SPEC-0002 编号 S9）/S5/S6 的所有权一致。
4. **pre-ship `REFUNDING` 可关闭 ≠ 退款完成**：与 AC-48（`REVOKED` → `FULFILLMENT_NOT_ALLOWED`，`0002:203-204`）、AC-50（`0002:222-223`）、INV-45（`0002:129`）、AC-62（`0002:308-310`）不冲突；permit 终态不可恢复，关闭只是“不会再产生包裹”，退款进度仍由 S4/`refund_state` 表达。**前序 M-5 已关闭。**
5. **§7.5 示例修正**：`G2 已关闭 + G1 已发 → ALLOCATED / SHIPPED` 与代码 `shipmentStatusForGroups`（`settlement.go:90-125`）、`aggregateOrderStatus`（`consumer.go:163-203`）一致；`全部退款（未发）→ UNSHIPPED` 一致；AC-26/AC-27 的边界（“G2 是否已关闭”而非“是否未发”）已写清（`0001:597-599`、`423`）。**除 M-3（S1 迁移集合/all-refunded 行）外，无新矛盾。**
6. **变更记录/批准记录**：未出现 Release/AC-VERIFIED 误称；`status: approved` + `approved_by: Director` + “QC delta 复核后实现”与 `docs/review-tracking.md:350-356` 的“QC delta review is still required before implementation; no merge/Release/AC-VERIFIED implied”一致；本 SPEC 未改 SPEC-0002/ADR/代码/台账（ADR r3/evidence 行属其他 actor 的独立提交，见 M-9）。
7. **越权**：PO 未修改 SPEC-0002/ADR/代码；`0001:816-818` 对 Architect/Developer/Reviewer/SPEC-0002 owner 的下游影响是**声明**而非代写，符合角色边界（但范围/证据仍见 M-4/M-7）。

---

## 5. 阻塞 / 不阻塞与下游

- **阻塞人类接受 v1.5（必须 PO 改）**：B-1（AC-27A IF 的读/写路径与错误码条件）、B-2（EARS-2 生命周期枚举）、B-3（PRE 证据/存量裁定与 `CONSUMED+无运单` 人工终态）。
- **阻塞实现（即使 B 修完也必须先处置）**：M-4（零元路径 + ADR §5.3 amendment）、M-9（commit/绑定被审字节）、M-7（术语/事实边界）。M-1/M-2/M-3/M-5/M-6/M-8 必须在 v1.5 正文或 §16 内闭合，否则 AC-27/27A 不可判定或会产生伪断言。
- **不阻塞、但阻塞“AC-27 全量收口/台账”**：AC-50 `CONSUMED+无运单` 的实现修正（跨 `internal/services/fulfillment` + `internal/services/order`，超 R0.3-R1c `allowed_paths`，需 Lead 拆独立 `change_id` 走 L2；见 `0001:816`）；多组 INV-44 释放缺口（P1/manual，evidence-matrix DEFERRED）；AC-27/AC-27A 的 `ac-coverage.tsv` 新行与证据（Reviewer，`0001:817`）。
- **下游 owner**：
  - PO：B-1/B-2/B-3 + M-1/M-2/M-3/M-5/M-6/M-7/M-8 + T-1…T-5；改完交 QC delta 复核。
  - Architect：ADR Rev 2.3 §5.3 `CancelIfCreatedReply` amendment（区分“无运单”）；ADR r03 的 R-2 撤销/改写、R-1 按 AC-27A 补齐（`0001:815`）；ADR r0.4 r3 与 SPEC 的 `Spec-Ref` 保持 v1.4→v1.5 待生效声明。
  - Developer：**在本报告转 READY 前不得实现**；修复 `CONSUMED+无运单` 必须覆盖正金额/TCC/`system_zero`，不得用 PRE 关闭。
  - Reviewer：AC-27/AC-27A 的 AC-REF/证据矩阵；AC-27A 是**新增 AC 行**，不得用 AC-27 旧行替代；AC-REF ≠ AC-VERIFIED。
  - Lead：拆独立 `change_id`（AC-50 修正）；在 `review-tracking.md` 记录本次 BLOCKED 与 scope。

## 6. 审查边界、未覆盖与证据声明

- **未执行**：`go test`、`-race`、真实 MySQL、迁移、故障注入；本报告中的代码/迁移/SPEC/ADR 引用是只读事实，**不构成执行证据**；未做生产库 `CONSUMED+PRE`/`CONSUMED+无运单` 的数据审计，相关规模判断标 `[不确定]`。
- **未覆盖**：ADR r0.4 r3 正文、SPEC-0002 正文修订、R0.4 实现、`docs/lead/**` 台账、部署/ops；这些文件在本轮其他提交（`d5dda12`/`caaf351`）中修改，不属本 QC 的审查对象（M-9）。
- **只读声明**：本报告只新建 `docs/collab/qc-reports/QC-PAY-SETTLE-01-SPEC-0001-V15.md`；未改 SPEC/ADR/代码/迁移/台账；未 commit；不批准 merge、Release 或 AC-VERIFIED；不代替人类批准。被审 SPEC 字节：`efad166ffd91d42a9bd95d638945dd6c831a7d2c`（`git hash-object docs/specs/0001-fulfillment-quantity-split.md`）。
- **结论**：**BLOCKED — 不交实现，不交人类验收**；PO 修复 B-1/B-2/B-3 与阻塞级 M 后重交 QC delta 复核。
