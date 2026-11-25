# ADR-0007 Amendment 1 Addendum — AC-50-FIX-01：`CancelIfCreated` 的“无运单行”additive 结果与 Order 映射

| 项 | 值 |
|---|---|
| Status | **DRAFT — pending QC round 3**（未批准；不构成实现依据；不代表 AC-VERIFIED / merge / Release PASS） |
| Revision | **0.3**（按人类裁决 **A**：删除 R-2a 预读；接受 Tx1 已写后的人工队列协议；逐条关闭 QC R2，见 §1.1） |
| Change ID | `PAY-SETTLE-01`（子范围 `AC-50-FIX-01`；实施须由 Lead 另拆独立 `change_id`，见 §0.5） |
| Tier | **L2**（跨服务契约语义、资金/履约准入、既有 RPC 响应语义；按 `docs/collab/CONVENTIONS.md:90` 任一命中即 L2） |
| Tier-Rationale | 修改既有 RPC `Fulfillment.CancelIfCreated` 的响应语义（新增可区分结果值），并改变 Order 准入结果映射（`REFUND_ADMISSION_CONFLICT` vs `PRE_SHIP_REVOKED`），直接影响资金打款/释放门禁与 AC-27A 关闭谓词；跨 Fulfillment/Order 两服务。即使代码增量小，也不得降级 |
| Human decision | **2026-11-26 人类裁决 A（Lead 转达）**：①移除 R-2a 与一切 `ListShipments`/事务外预读，不开 ADR §5.2.2 例外；②接受“Tx1 已写 + 人工队列”协议；③`NO_SHIPMENT` 时持久化 `result=REFUND_ADMISSION_CONFLICT`（不写 PRE）、告警、commitment 保持 `RESERVED`、outbox 占位保留、0 打款/0 释放/0 准入终态；④同 `refund_id` 重放返回已持久化 CONFLICT、不再远程探测。**范围**：仅 AC-50-FIX-01；不改 SPEC/Rev 2.3 正文；Lead 须在 `docs/review-tracking.md` 记录该裁决 |
| QC 输入 | `docs/collab/qc-reports/QC-PAY-SETTLE-01-AC50-NOWB.md`：R1（BLOCKED；base `da0cdd1`；0.1 blob `53e799e`）+ **R2（BLOCKED；head `aec9ac8`；0.2 blob `f8b9e67`）** |
| Spec-Ref | SPEC-0001 **v1.5**：AC-27（`0001:592-613`）、AC-27A（`0001:615-645`）、AC-27A 注（`0001:645`）、§16.2 `AC-50-FIX-01`（`0001:862`）、`AC-27-NOWB-01`（`0001:863`）、`AC-27-LIVE-01`（`0001:864`）、§16.3 收口前提（`0001:874`）与独立 `change_id` 要求（`0001:879`）、§16 V15-3（`0001:827`）；SPEC-0002 **v1.10**：AC-50（`0002:213-231`，`CONSUMED+无运单`=`0002:223`、部分金额=`0002:217`、已离仓部分=`0002:226`）、AC-48（`0002:200-206`）、AC-62（`0002:308-323`）、INV-36（`0002:120`）、INV-42（`0002:126`）、INV-43（`0002:127`）、INV-44（`0002:128`）、INV-45（`0002:129`）、INV-50（`0002:134`）、错误表（`0002:517`） |
| Base（仓库） | `47db2e0d35d92520b9f9a55c26af3230fe85c2cc`（`docs(lead): R0.4e-R2 PASS scoped; AC-50-FIX-01 blocked`） |
| Base（契约基线） | SPEC-0001 v1.5（approved 2026-11-26，`4215b3c`）、SPEC-0002 v1.10（approved 2026-11-25）、ADR-0007 **Rev 2.3**（accepted 2026-11-25，`docs/adr/0007-payment-settlement-admission.md`） |
| Code head（裁决针对的实现证据） | `c38c54f`（R0.4 实现；`git diff c38c54f..HEAD -- internal/services/order/refund_admission.go internal/services/fulfillment/cancel_if_created.go api/proto/mall/v1/mall.proto` 为空，本裁决涉及的锚点文件未变）。工作树只应有本文件与 Lead-owned `docs/review-tracking.md` 的改动；其他并发改动以 `git status` 为准，不属于本裁决 |
| Doc head | 本文件由 **Lead 统一提交**；QC/Reviewer 复核时必须记录精确 SHA（本文件不自引 SHA） |
| Integration owner | **System Architect**（本 Addendum 作者/契约）；实现 owner = **Developer**（另拆 `change_id`）；S6 无运单证据 owner = **Fulfillment**；人工队列/告警/重放 owner = **ops** |
| allowed_paths（本文件） | 仅 `docs/adr/0007-amd1-addendum-ac50-nowaybill.md` |
| allowed_paths（本裁决授权的最小实现，须在 Lead 另拆的 `change_id` 下） | `api/proto/mall/v1/mall.proto`（**仅** `CancelIfCreated` 的 RPC/`CancelIfCreatedReply.status` 注释）、`api/gen/mall/v1/mall.pb.go`、`api/gen/mall/v1/mall_grpc.pb.go`（**仅** `make proto` 生成的注释同步，禁止手改）、`internal/services/fulfillment/cancel_if_created.go`、`internal/services/order/refund_admission.go`（`finishRemoteAdmission` 的 `NO_SHIPMENT`/部分金额映射、Tx2 状态 CAS 与 CONFLICT 持久化、`AdmitRefund` 的 existing CONFLICT 重放短路、零元 `decideZeroRefundResult`）、`internal/services/fulfillment/cancel_if_created_test.go`、`internal/services/fulfillment/admission_consumer_nats_integration_test.go`（**仅测试**）、`internal/services/order/refund_admission_integration_test.go`、`internal/services/order/tcc_admission_integration_test.go`（**仅测试**） |
| forbidden_paths | `migrations/**`、`api/gen/**` 手改、`api/proto` 的新字段/新 RPC/枚举变更、`internal/platform/**`、`internal/services/order/ports.go`（**禁止**新增 `ListShipments` 调用或接口变更）、`internal/services/order/settlement.go`、`internal/services/fulfillment/consumer.go`、`internal/services/fulfillment/admission_consumer.go`（生产消费者；现有 `NO_SHIPMENT` no-op 语义不得改）、`internal/services/{inventory,payment,aftersales}/**`、`internal/services/order/tcc_*.go`（**仅生产文件，`tcc_*_test.go` 除外**）、`deploy/**`、`frontend/**`、`docs/specs/**`、`docs/collab/**`（含 `ac-coverage.tsv`）、`docs/lead/**`、`docs/implementation/**`、`docs/adr/0007-payment-settlement-admission.md`（已 accepted Rev 2.3 正文）、`docs/adr/0007-payment-settlement-admission-amd1.md`、`scripts/**`、`Makefile` |
| baseline_impact | **涉及**：AC-50 的 `CONSUMED+无运单`（整组）与 `CONSUMED` 部分金额分支、AC-27A 关闭谓词的 PRE 来源合法性、AC-27 的收口前提；跨 Fulfillment/Order 的既有 RPC 响应语义；`order_refund_admissions.result` 新增冲突值（`state` 保持 `PENDING_REMOTE`、不新增值；人类 A 授权，无 schema 变更）。**不涉及**：Payment 金额/退款实现、迁移/schema、事件 subject/payload 结构、permit 状态机、任何新 worker；非回归范围见 §14 |
| INV | INV-36、INV-42、INV-43、INV-44、INV-45、INV-50（逐条见 §14；无新增 INV） |
| Actor | `arch-ac50`（System Architect） |
| Reviewed-by | **待 QC round 3**（本文件不代签；DRAFT 期间不得合入受管代码，不得标 PASS） |
| 硬边界 | 不改 SPEC（SPEC-0001/0002 均归 PO）、不改 accepted ADR Rev 2.3 正文（用本 addendum，未来 Rev 2.4 errata 由 Architect 合并）、无新字段/新 RPC/新事件/新迁移/新状态字段/新 worker；**无 `ListShipments`/事务外预读**；不交付 T2/T3/T4.4；本文件未明确列出的变更一律不做 |

> 本文件是**设计裁决**，不是实现、不是验收、不是台账。代码/测试状态归 Developer/Reviewer；AC 状态归 Reviewer；人工队列终态归 SPEC-0001 §16.2。文中的“MUST/不得”是给 Developer 的施工约束，不免除 QC 与人类批准。

---

## 0. 阅读方式与规范边界

### 0.1 问题陈述与代码事实

SPEC-0001 v1.5 `0001:862` 登记：`CONSUMED+无运单` 写路径把“无运单行”映射为 `CANCELLED`/PRE，零元路径 `decideZeroRefundResult` 同样；ADR §5.3 的 `CancelIfCreatedReply` 没有“无运单”结果。不变量是：**`PRE_SHIP_REVOKED` 只能由 AC-50 合法未离仓分支写入；`CONSUMED+无运单` 必须返回 `REFUND_ADMISSION_CONFLICT`、不得写 PRE**。

| # | 事实 | 证据 |
|---|---|---|
| 1 | Fulfillment 查不到运单行时返回 `CANCELLED` | `internal/services/fulfillment/cancel_if_created.go:51-55`（`:54`） |
| 2 | Order `finishRemoteAdmission` 把 `CANCELLED` 一律映射为 PRE，并在 Tx2 写回 `result=PRE_SHIP_REVOKED` | `internal/services/order/refund_admission.go:566-608`（`:576-584`、`:590-597`） |
| 3 | 零元 `decideZeroRefundResult` 同样把 `CANCELLED` 映射为 PRE | `refund_admission.go:366-396`（`:384-391`） |
| 4 | **正金额 `CONSUMED` 的 Tx1 在远程探测前已写**：admission（`result=POST,state=PENDING_REMOTE`）、`order_refund_commitments(RESERVED)`、outbox `order.refund_admitted` | `refund_admission.go:253-283`（admission `:254-265`、commitment `:266-272`、outbox `:281`、payload `:610-619`） |
| 5 | **Tx1 前已调用 `Payment.GetIntent`**（只读快照） | `refund_admission.go:168-175` |
| 6 | TCC Tx1 的 admission 写 `result=PENDING_REMOTE,state=PENDING_REMOTE`（既有权宜值） | `tcc_settlement.go:847-852` |
| 7 | 同 `refund_id` 重放且 `state=PENDING_REMOTE` 时会重新进入 `finishRemoteAdmission` | `refund_admission.go:179-185`、`:203-211` |
| 8 | AC-27A 第 5/6 条：`CONSUMED` 只有 `result=PRE_SHIP_REVOKED` 才关闭；`PENDING_REMOTE`、结果缺失/不可核验 → 未关闭并告警 | SPEC-0001 v1.5 `0001:622-625`、注 `0001:645` |
| 9 | AC-50：`NONE→REVOKED` 无运单合法 PRE；`CONSUMED+CANCELLED` 合法 PRE；部分金额未离仓 → `PARTIAL_REFUND_REQUIRES_FULL_GROUP` | SPEC-0002 `0002:217`、`:219`、`:222` |
| 10 | `order_refund_admissions.result/state` 为 `VARCHAR(32)`，无 CHECK 约束；默认 `PENDING_REMOTE` | `migrations/order/0016_settlement_admission.sql:7-19` |
| 11 | 每组至多一条运单行：`UNIQUE KEY (order_no, fulfillment_group_no)` | `migrations/fulfillment/0003_group_shipments.sql:6-9` |
| 12 | 除 Order 与 Fulfillment 自身的 `order.refund_admitted` 消费者外，没有其他 `CancelIfCreated` 调用方 | 全仓库 grep；`internal/services/fulfillment/admission_consumer.go:43` |

**结论**：缺陷在“事实无法区分”：Fulfillment 把“无行”与“真实已取消行”压成同一个 `CANCELLED`，Order 又把它压成同一个 PRE。本 amendment 采用 QC B-2 的**第一种**路径（新增可区分结果），并接受人类 A 的“Tx1 已写 + 人工队列”协议。

### 0.2 `CONSUMED+无行` 的可达性（R2-M12 保持）

| 路径 | 是否可能产生 `CONSUMED+无行` | 证据 |
|---|---|---|
| 正常 `Ship` 路径 | **否**：`CreateShipment` 先插入运单行（`CREATED`），`Ship` 才调 `Order.AuthorizeShipment` 消费 permit | `fulfillment/service.go:154-166`、`:244`；`settlement.go:353-357` |
| 直接调用 `Order.AuthorizeShipment`（Order RPC） | **是**：Order 不校验运单行是否存在 | `api/proto/mall/v1/mall.proto:485-487` |
| 历史数据（0016/0003 之前、无 permit 行、行缺失） | **可能** | `migrations/order/0016_settlement_admission.sql`、`migrations/fulfillment/0003_group_shipments.sql` |
| 人工/数据修复、行被删除 | **可能** | ops；`[不确定]` 生产发生率需 ops 证据 |
| 部分金额 + `CONSUMED` | 不进入 `NO_SHIPMENT` 的 CONFLICT 标记；按 §3.4 返回 PARTIAL | §3.4 |

> O1/O3/O7 的 `CONSUMED+无行` 是**合成异常状态**，用于验证 fail-closed；不据此声称“已发生的 P0 缺陷”或频率。正常路径回归见 F8/O4。

### 0.3 指针澄清（brief 与 canonical 文件不一致处）

任务 brief 指向 `docs/adr/0007-payment-settlement-admission-amd1.md` 的 §5.2/§5.3/§5.7。该文件是 Rev 2.3 的**来源文件**，但其 §5.3（`:375-397`）是 `legacy_payment_resolution` 的并发与事务规则；`Fulfillment.CancelIfCreated` 的规范正文在**合并后的 accepted Rev 2.3** 文件 `docs/adr/0007-payment-settlement-admission.md` §5.3（`:611-623`，合并映射见 `:56`）。本 amendment 的修改对象是**后者**；文件名沿用 Lead 指定的 `0007-amd1-addendum-ac50-nowaybill.md`。

### 0.4 与已批准契约的关系

- 本文件**additive**：不删除、不重写 Rev 2.3 任何条款；只给 §5.3 增加一个 `status` 值及其语义，并明确 §5.2.2/§5.2.4 的映射与 Tx1 已写事实的处置。
- 本文件**不批准**任何实现；`ADR_APPROVED`（人类）之前 Developer 不得开工（宪法 Workflow 2）。
- 本文件不改变事实所有权、不改变 permit 状态机、不改变退款额度、不改变 INV-45 的不可逆性；**不新增 `ListShipments`/事务外预读**（人类 A）。
- `order_refund_admissions.result` 的 `REFUND_ADMISSION_CONFLICT` 是**冲突标记值**（不是准入终态；`state` 保持 `PENDING_REMOTE`），由人类 A 授权；与 SPEC-0001 `0001:645` 注的字面枚举（result 只允许 PRE/POST）存在漂移，owner PO 在下次 SPEC 升版同步（§9/§12）。本 ADR 不改 SPEC。

### 0.5 `change_id` 边界（防越权）

SPEC-0001 v1.5 §16.3 `0001:879` 明确：AC-50 修正“需 Lead 拆独立 `change_id` 走 L2（跨 Fulfillment/Order，超 R0.3-R1c `allowed_paths`）”。因此：

- 本 Addendum（契约）挂 `PAY-SETTLE-01`（Lead 指定）；
- **实施**必须由 Lead 另拆独立 `change_id`，并在其实施 Brief 中逐文件落 `allowed_paths`；不得把本文件的 `allowed_paths` 当作 R0.3-R1c 的范围扩展，也不得在同一提交里顺手改 AC-27A 读模型（那是 R0.3-R1c 的独立实施项）。

---

## 1. 裁决摘要

| # | 裁决 | 一句话 | 归属 |
|---|---|---|---|
| **R-1** | `CancelIfCreatedReply.status` 增加 **additive** 值 `NO_SHIPMENT`（string，无新字段/无新 RPC）；Fulfillment 对“该组没有任何运单行”返回它；`CANCELLED` 只表示真实运单已取消 | 先分开事实 | Architect（契约）/ Fulfillment（实现） |
| **R-2** | **整组金额** `NO_SHIPMENT`：Tx2 写 admission `result=REFUND_ADMISSION_CONFLICT`（`state` 保持 `PENDING_REMOTE`；不写 PRE/POST）、Error 告警、commitment 保持 `RESERVED`、outbox 占位保留、0 打款/0 释放/0 准入终态；进入 `AC-27-NOWB-01` 人工队列。同 `refund_id` 重放读已持久化 CONFLICT 并返回同一错误，**不再远程探测** | 持久化冲突 + 人工队列 | Order（实现） |
| **R-2b** | **部分金额**（`amount < 组剩余`）：`permit NONE/REVOKED` 由 Order 自有 permit 事实直接判 pre-ship → `PARTIAL`（Tx1 前，零写入）；`permit CONSUMED` 不预读，Tx1 照旧，Tx2 探测：`CANCELLED`/`NO_SHIPMENT` → `PARTIAL`（不写 PRE、不写 CONFLICT）、保留 Tx1 占位/commitment、告警进人工队列；`ALREADY_SHIPPED/RECEIVED` → POST（部分金额合法）；瞬态/未知 → 可重试 | 不丢 PARTIAL，不静默 POST | Order（实现） |
| **R-3** | 零元 `decideZeroRefundResult`：`NO_SHIPMENT` → conflict + 告警，在写任何 admission/`DONE` 之前返回；`CANCELLED` → PRE；`ALREADY_SHIPPED/RECEIVED` → POST；瞬态/未知 → 可重试。零元无 Tx1，故**不持久化 CONFLICT 标记**，重放重新探测 | 零元不得例外 | Order（实现） |
| **R-4** | 正金额/TCC/legacy/零元共用**唯一映射表**；TCC 经 `AdmitRefund`→`finishRemoteAdmission`，禁止在 `tcc_*.go` 复制第二套状态判断 | 拒绝 if-else 扩散 | Order（实现）/ Reviewer |
| **R-5** | 持久化语义：准入成功终态当且仅当 `state ∈ {PRE,POST}` 且 `result ∈ {PRE,POST}` 且 `state=result`；`PENDING_REMOTE` 不是结果；`REFUND_ADMISSION_CONFLICT` 是**冲突标记**，不是准入终态；Tx1 写入按 §7.4 处置，**不声称“无写入”** | 单一事实载体 + 占位/冲突语义 | Order（实现）/ Reviewer |
| **R-6** | 幂等/并发/崩溃：同 `refund_id` 重放读已持久化结果（PRE/POST/CONFLICT）并返回同一结果；`PENDING_REMOTE` 重新探测；Tx2 以 `state=PENDING_REMOTE` CAS 唯一化写入；崩溃/重复/乱序不得产生 PRE/POST 脏写、重复事件或重复释放 | 可重放、无脏写 | Order/Fulfillment |
| **R-7** | `NO_SHIPMENT`（整组）与部分金额（pre-ship）冲突均进入 **AC-27-NOWB-01** 人工队列（终态/owner/expiry 见 SPEC-0001 §16.2 `0001:863`），组保持未关闭；不得自动关闭/自动退款/恢复 permit；TCC 有独立发现口径 | 人工终态 | ops + Developer + Fulfillment |
| **R-8** | 登记：`mall.proto` 注释必须更新（生成物同步）；`docs/contracts.md` **不改**；错误码不新增（`REFUND_ADMISSION_CONFLICT`/`PARTIAL_REFUND_REQUIRES_FULL_GROUP`）；`order_refund_admissions.result` 新增冲突值（`state` 不新增值）需 PO 同步 SPEC-0001 注（无 schema 变更） | 最小登记面 | Architect / Developer / PO / Reviewer |

**结论级别**：R-1…R-8 均在**不改 SPEC、不改 Rev 2.3 正文、无迁移、无新字段/RPC/事件/worker、无 `ListShipments`/预读**的前提下落地。无 `[ARCHITECTURAL_FLAW]`；人类 A 已接受“Tx1 已写 + 人工队列”的代价。仍登记 6 个**不在本 amendment 修复范围**的既有缺口（M-1 legacy、M-3 人工释放实现、M-4 TCC result 清理、M-5 存量审计执行、M-7 TCC last_error、T-3 多组释放），见 §1.3，owner/expiry 均为 2026-12-31。

### 1.1 QC R2 处置索引（B-1…B-3、R2-B1/B2、M-1…M-13、R2-M1…M5、T-1…T-7、R2-T1…T4）

| QC 项 | R2 判定 | 本 0.3 处置 |
|---|---|---|
| **B-1** Tx1 已写 | CLOSED | 保持 0.2 的 A7/§7.4；**新增 R-2** 明确整组 `NO_SHIPMENT` 写 CONFLICT，不再“保持原样” |
| **B-2** `GetIntent`/Payment 0 | CLOSED | 保持“只禁 `Refund/RefundBatch`，`GetIntent` 允许”；测试 `refundCalls==0` |
| **B-3** W6/正常 consumer | PARTIAL | 人工侧 **CLOSED**（禁止造行）；正常 consumer 侧 **PARTIAL**：按人类要求**删除 0.2 的“迟到 order.paid 不建 placeholder”断言/路径**（生产 `autoShipAllowed` 不能删，见 forbidden）；W9 改为真实行为，迟到行不是人工证据、CONFLICT 后不自动转 PRE；F7b/O14b；阻断该路径另立 `ORDER-PAID-PLACEHOLDER-01`（DEFERRED） |
| **R2-B1** R-2a 门禁破坏幂等 | BLOCKER | **删除 R-2a**；`AdmitRefund` 先读 existing（PRE/POST/CONFLICT 直接返回），不存在才 Tx1；无预读 |
| **R2-B2** R-2a 违反 §5.2.2 | BLOCKER | **删除 R-2a/全部 `ListShipments`**；不开 §5.2.2 例外 |
| **M-1** legacy 已发货 PRE | PARTIAL | 保持 DEFERRED + 审计查询**补 LEFT JOIN/无 group 行**（R2-M2）；per-item stop 见 §1.3 |
| **M-2** 部分金额 | NOT CLOSED | **R-2b**：无预读；CONSUMED 部分金额在 Tx2 探测后判 `CANCELLED/NO_SHIPMENT→PARTIAL`，保留 Tx1 占位/人工队列（人类 A） |
| **M-3** A4/AC-62 | CLOSED 规则/DEFERRED | 保持 A4 人工授权例外；O15 DEFERRED + per-item stop |
| **M-4** TCC `result=PENDING` | CLOSED 读者/DEFERRED | 保持 A2 双条件；`TCC-PENDING-RESULT-01` + per-item stop |
| **M-5** 存量 PRE 审计 | PARTIAL | 审计查询补无 group 行/`state=PENDING_REMOTE` 分支；owner/expiry/per-item stop |
| **M-6** TCC 测试路径 | CLOSED | 保持 `tcc_*_test.go` allowed、生产 `tcc_*.go` forbidden |
| **M-7** TCC 队列发现 | CLOSED 查询/DEFERRED | 保持查询；`TCC-LAST-ERROR-01` + per-item stop |
| **M-8** 重复/乱序 | CLOSED | W9 重写；F7/O14 + F7b/O14b |
| **M-9** Tx2 守卫 | CLOSED 规则 | A8 + **0 行语义**（R2-M5）；O11b/O11c |
| **M-10** POST commitment | CLOSED | 保持分行说明 O5/O5b |
| **M-11** consumer `NO_SHIPMENT` | CLOSED | 保持 no-op success；F7/O14 |
| **M-12** 可达性 | CLOSED | 保持 §0.2/F8；ops 证据登记 |
| **M-13** setup | CLOSED | 保持 setup 列 |
| **R2-M1** late `order.paid` placeholder | 新 MAJOR | **W9 重写 + F7b/O14b**；人工侧仍禁止造行；阻断正常 consumer 路径另立 L2（DEFERRED） |
| **R2-M2** 审计漏无 group 行 | 新 MAJOR | §8 审计查询改 `LEFT JOIN` + `g.group_no IS NULL` + `state IN ('PRE','PENDING_REMOTE')` |
| **R2-M3** 缺 per-item stop | 新 MAJOR | §1.3 逐项 owner/expiry/stop |
| **R2-M4** R-2a 无 deadline | 新 MAJOR | **随 R-2a 删除而消失**；本稿无 ListShipments |
| **R2-M5** A8 0 行未定义 | 新 MAJOR | A8 补 nil/PENDING/terminal/CONFLICT 四分支；O11c |
| **T-1** SPEC-0002 错误表漂移 | TODO | 保持 PO/2026-12-31；加 per-item stop |
| **T-2** proto 工具版本 | FIXED | 保持；两生成文件仅注释 diff |
| **T-3** 多组释放 | CLOSED 单组/DEFERRED | `INV-44-MULTI-01` + per-item stop |
| **T-4** 零元 transient | FIXED | O7b |
| **T-5** `shipment_no` 非空 | FIXED | F1b |
| **T-6** 固定 key/大小写 | FIXED | §8/§9 |
| **T-7** 锁顺序 | FIXED | 选状态 CAS（A8）；不选行锁 |
| **R2-T1** §3.1 CANCELLED 行措辞 | TODO | §3.1 已改为“Order 准入层不写 Payment；`Refund/RefundBatch` 由调用方在准入后决定；本层只写 admission/outbox/release” |
| **R2-T2** O4d/O4e | TODO | **随 R-2a 删除而 N/A**；改为 O4b/O4c/O4d（无预读的部分金额） |
| **R2-T3** map `CANCELLED` 语义 | TODO | **随 R-2a 删除而 N/A** |
| **R2-T4** G1 late `order.paid` 用例 | TODO | F7b/O14b |

### 1.2 13 MAJOR + R2 新增：逐条状态（CLOSED / PARTIAL / OPEN + owner/expiry/stop）

| QC 项 | 状态 | 说明 | owner / expiry / stop |
|---|---|---|---|
| **M-1** legacy `NONE`+已发货 PRE | **PARTIAL（可接受 DEFERRED）** | A3 收窄；审计补齐无 group 行；`LEGACY-SHIPPED-PRE-01` | Architect/PO；2026-12-31；审计发现 PRE+已发货/无 S6 证据 → AC-27A 第 4/5 条不得 AC-VERIFIED，冻结收口，转独立 L2 |
| **M-2** 部分金额 | **CLOSED（规则）/ 人工债务 DEFERRED** | R-2b：无预读；CONSUMED 部分金额 `CANCELLED/NO_SHIPMENT→PARTIAL`，Tx1 占位/人工队列；`SHIPPED→POST` | Developer/ops（实现+人工队列）；2026-12-31；停止条件：任何 partial+pre-ship 写 PRE/POST 或静默放行 → FAIL；AC-62 释放（T3）未实现 → 旧 `RESERVED` 保持、组未关闭 |
| **M-3** A4/AC-62 | **CLOSED 规则 / DEFERRED 实现** | A4 自动路径 + AC-62 唯一例外；O15 | Developer/T3；2026-12-31；release writer 未实现 → 不得用 fixture 冒充；conflict 组保持未关闭 |
| **M-4** TCC `result=PENDING` | **CLOSED 读者 / DEFERRED 清理** | A2 双条件；O13b | Architect/Developer；2026-12-31；清理前读者只按 A2；只读 result 的消费方 → FAIL |
| **M-5** 存量 PRE 审计 | **CLOSED 协议 / DEFERRED 执行** | §8 查询（含无 group 行）+ AC-27A 门禁 | ops/Developer；2026-12-31；审计未完成/发现不可核验 → AC-27A 不得标已关闭/AC-VERIFIED |
| **M-6** TCC 测试路径 | **CLOSED** | allowed `tcc_*_test.go`；生产 forbidden | Developer；N/A |
| **M-7** TCC 队列发现 | **CLOSED 查询 / DEFERRED last_error** | §8 查询；`TCC-LAST-ERROR-01` | Developer/Architect；2026-12-31；查询/日志不可用 → 不得声称可发现，升级 ops |
| **M-8** 重复/乱序 | **CLOSED** | W9 重写；F7/O14 + F7b/O14b | Fulfillment consumer（只测试）；N/A |
| **M-9** Tx2 守卫 | **CLOSED 规则** | A8 + 0 行语义；O11b/O11c | Developer；N/A；0 行脏写 → FAIL |
| **M-10** POST commitment | **CLOSED** | §3.1 分行；O5/O5b | Developer；N/A |
| **M-11** consumer `NO_SHIPMENT` | **CLOSED** | §2.2 no-op success；F7/O14 | Fulfillment；N/A |
| **M-12** 可达性 | **CLOSED（说明）** | §0.2/F8；ops 证据 | Lead/ops；2026-12-31；若证明正常路径可达 → 回 Architect |
| **M-13** setup | **CLOSED** | §10 setup 列 | Developer；N/A |
| **R2-M1** late `order.paid` | **CLOSED（协议/测试） / 阻断 DEFERRED** | W9 重写；F7b/O14b；`ORDER-PAID-PLACEHOLDER-01` | Architect/Developer；2026-12-31；若业务要求禁止 → 独立 L2；在此之前不得声称“正常 consumer 不会建行” |
| **R2-M2** 审计漏无 group | **CLOSED** | §8 查询修正 | ops/Developer；2026-12-31；0 行 ≠ 安全（先证明无历史 G1） |
| **R2-M3** per-item stop | **CLOSED** | §1.3 | Lead 台账；到期/提前关闭由 Lead 记录 |
| **R2-M4** R-2a deadline | **CLOSED（删除）** | 无 ListShipments | N/A |
| **R2-M5** A8 0 行 | **CLOSED** | A8 四分支；O11c | Developer；N/A；0 行静默成功 → FAIL |

### 1.3 DEFERRED / 人工项的 per-item 停止条件（R2-M3）

| 项 | owner | expiry | stop / exit |
|---|---|---|---|
| `LEGACY-SHIPPED-PRE-01`（M-1） | Architect/PO | 2026-12-31 | 审计发现“PRE + 已发货/无 S6 证据/无 group 行” → AC-27A 第 4/5 条不得标已关闭/AC-VERIFIED；冻结收口；修复走独立 L2 或 PO 裁决 |
| AC-62 人工释放（M-3/O15） | Developer/T3 | 2026-12-31 | release writer 未实现 → 不得用 fixture/未授权释放冒充；conflict 组保持未关闭；T3 关闭前不得声称 AC-62 可执行 |
| `TCC-PENDING-RESULT-01`（M-4） | Architect/Developer | 2026-12-31 | 清理前任何只读 `result`/只比 `state==result` 的消费方 → FAIL；AC-27A 必须 A2 双条件 |
| 存量 PRE 审计（M-5） | ops/Developer | 2026-12-31 | 审计未执行或存在不可核验 PRE → AC-27A 不得关闭/AC-VERIFIED；0 行不自动等于安全 |
| `TCC-LAST-ERROR-01`（M-7） | Developer/Architect | 2026-12-31 | 查询/日志不可用 → ops 不得声称可发现；转人工升级；不得写 `refund_batches.last_error`（未授权生产改动） |
| `INV-44-MULTI-01`（T-3） | Developer/Lead | 2026-12-31 | 多组 release 未实现 → AC-27 EARS-3/INV-44 不得标 VERIFIED；不得用 Order fixture 冒充 |
| SPEC 错误表/注同步（T-1/冲突值） | PO | 2026-12-31 | SPEC-0001 `0001:645` 注未同步 CONFLICT 值 → 不得以 SPEC 注否定本裁决；PO 拒绝该值 → 实现停止，回 Architect/人类 |
| `ORDER-PAID-PLACEHOLDER-01`（R2-M1 阻断） | Architect/Developer | 2026-12-31 | 业务要求禁止正常 consumer 迟到 placeholder → 独立 L2；在此之前 W9 的真实行为（真实行、非证据、CONFLICT 后不自动 PRE）为准 |
| 人工 re-arm runbook（§8） | ops | 2026-12-31 | runbook/授权/审计未就绪 → `AC-27-NOWB-01` 终态①不得执行；不得用普通 replay 冒充；组保持未关闭 |

---

## 2. R-1：`NO_SHIPMENT` 的契约语义（Fulfillment 侧）

### 2.1 契约文本（additive，仅 string 值）

```text
message CancelIfCreatedReply {
  // CANCELLED | NO_SHIPMENT | ALREADY_SHIPPED | RECEIVED | TRANSIENT_ERROR
  // CANCELLED       = a real shipment row was CREATED->CANCELLED, or was already CANCELLED.
  // NO_SHIPMENT     = this (order_no, group_no[, shipment_no]) has no shipment row at all;
  //                   nothing was cancelled and nothing was rolled back.
  // ALREADY_SHIPPED = the parcel left the warehouse (SHIPPED, or EXCEPTION with shipped_at);
  //                   never cancelled.
  // RECEIVED        = the parcel was received; never cancelled.
  // TRANSIENT_ERROR = ambiguous/unknown (incl. EXCEPTION without shipped_at); retry/manual,
  //                   never treated as cancelled.
  string status = 1;
}
```

- **无新字段、无新 RPC、无枚举类型变更**：`status` 是 `string`，这是既有字符串契约上的新增取值。
- `NO_SHIPMENT` 的语义是**“该探测范围内没有任何运单行”**，不是“未发货”、不是“已取消”、不是“可以 PRE”。它不携带 permit/准入结论；permit 事实仍归 Order（S5）。
- 探测范围：`order_no` 必填；`group_no` 为空时既有实现默认 `G1`（不改）；`shipment_no` 非空时范围收窄到 `(order_no, group_no, shipment_no)`。Order 调用点必须继续传空 `shipment_no`（现状），使 `NO_SHIPMENT` 表示**整组无行**；T-5 见 §10 F1b。
- `migrations/fulfillment/0003_group_shipments.sql:6-9` 的 `UNIQUE (order_no, fulfillment_group_no)` 保证每组至多一行，故 `NO_SHIPMENT` 无“多行歧义”。

### 2.2 返回矩阵（Fulfillment + `order.refund_admitted` consumer）

| 数据库事实 / 输入 | Fulfillment 返回 | consumer（仅 `result=PRE`）行为 | 说明 |
|---|---|---|---|
| 无行 | **`NO_SHIPMENT`** | **幂等成功/no-op**（无行可取消，不报错） | 新增；当前错误返回 `CANCELLED` |
| `CREATED` | `CANCELLED` | 取消幂等；重复 PRE 不重复写 | 不变 |
| `CANCELLED` | `CANCELLED` | 幂等 | 不变 |
| `SHIPPED` | `ALREADY_SHIPPED` | Error 告警（“pre-ship 但已离仓”），不回滚 | 不变 |
| `RECEIVED` | `RECEIVED` | 同 `ALREADY_SHIPPED` 告警 | 不变 |
| `EXCEPTION` 且 `shipped_at>0` | `ALREADY_SHIPPED` | 告警 | 不变（T1.2/R-2） |
| `EXCEPTION` 无 `shipped_at` | `TRANSIENT_ERROR` | 重试；不取消 | 不变 |
| 其他未知状态 | `TRANSIENT_ERROR` | 重试；不取消 | 不变 |

- consumer 判据（现状 `internal/services/fulfillment/admission_consumer.go:40-55`）：只处理 `result=PRE_SHIP_REVOKED`；`NO_SHIPMENT` 是 no-op success，**不是** `TRANSIENT_ERROR`，不得据此报错或重试；重复/乱序见 W9（§7.2）与 F7/O14（§10）。本 amendment **不改**该生产文件。

### 2.3 明确禁止

1. **ops/人工**不得为“无行”补建 `CANCELLED`/`CREATED` 运单作为证据（B-3）；正常 `order.paid` consumer 迟到创建的真实 placeholder **不是**人工证据（W9/§12），CONFLICT 后不得据此自动转 PRE。
2. 不得因 `NO_SHIPMENT` 由 Fulfillment 写 permit、admission、commitment、库存或任何 outbox 事件（Fulfillment 不是这些事实的 owner）。
3. 不得把 `NO_SHIPMENT` 复用为 `TRANSIENT_ERROR` 或 `CANCELLED` 的别名；失败语义必须可区分。
4. 不得新增 `warehouse_id` 缺省、`G1` 缺省或新的请求字段；`group_no` 的既有 `G1` 缺省行为保留，但调用方必须显式传组号（现状）。

### 2.4 兼容与发布顺序

- `NO_SHIPMENT` 是**新增值**。旧 Order 客户端对未知值走 `default` → `apperr.Unavailable`（可重试，见 `refund_admission.go:582-584`、`:389-391`），**不会**写 PRE。
- 若 Fulfillment 先上线：旧 Order 只会重试，状态停在 `PENDING_REMOTE`，fail-closed，可接受（告警噪声，不资损、不误关闭）。
- 若 Order 先上线：在 Fulfillment 仍返回 `CANCELLED` 期间，缺陷保持现状（风险不变，不得宣称修复完成）。
- 按 `CONVENTIONS.md:241`，**建议同一原子行为提交**同时改 Fulfillment 与 Order；若必须分批，按上一条 fail-closed 顺序，并在实施 Brief 记录批次与回滚。

---

## 3. R-2 / R-2b：Order 映射（正金额 / TCC / legacy）

### 3.1 唯一映射表（`finishRemoteAdmission`，整组金额）

| `CancelIfCreated` 返回 | Order 准入结论 | 持久化动作（**含 Tx1 已写事实**） | 退款/释放 |
|---|---|---|---|
| `CANCELLED` | `PRE_SHIP_REVOKED` | Tx2（状态 CAS）写 `result=PRE,state=PRE` + outbox；**正金额/TCC 的 Tx1 已写恰好 1 条 `RESERVED` commitment**（正金额 `:266-272`；TCC `tcc_settlement.go:898`），Tx2 **不重复写**；release 按 INV-44（单组）。Order 准入层不写 Payment；`Payment.Refund/RefundBatch` 由调用方在准入后决定；本层只写 admission/outbox/release | 允许（AC-50 合法 PRE） |
| **`NO_SHIPMENT`（整组）** | **`REFUND_ADMISSION_CONFLICT`（错误返回）** | **Tx2 写 `result=REFUND_ADMISSION_CONFLICT`**（`state` 保持 `PENDING_REMOTE`；不写 PRE/POST）+ Error 告警；**不开 release、不写终态 outbox**；Tx1 的 **commitment 与占位 outbox 保留**；admission 的 `result` 被 Tx2 覆盖为 CONFLICT（普通正金额 Tx1 `result=POST`；TCC Tx1 `result=PENDING_REMOTE`；两者 `state` 均保持 `PENDING_REMOTE`）；处置见 §7.4 | **禁止** `Payment.Refund`/`RefundBatch`、禁止 release/commitment release；`Payment.GetIntent` 只读允许 |
| `ALREADY_SHIPPED` / `RECEIVED` | `POST_SHIP_ALLOWED` | Tx2（状态 CAS）写 `result=POST,state=POST` + outbox；**正金额路径的 Tx1 已有 1 条 `RESERVED` commitment，Tx2 不重复写**；**TCC 已离仓组**由 `recordTccPostAdmission` 在同一事务写 POST admission + commitment（既有，`tcc_admission.go:214`） | 允许（已离仓） |
| `TRANSIENT_ERROR` | **不可判定** | 不动；保持 `PENDING_REMOTE` | 禁止；可重试（`apperr.Unavailable`） |
| 空值 / 未知值 | **不可判定** | 不动；保持 `PENDING_REMOTE` | 禁止；可重试（`apperr.Unavailable`） |

- `NO_SHIPMENT` 的判定与返回必须在**打开 Tx2 之前**完成；不得先写 PRE 再回滚“修正”。
- 错误对象：`apperr.Failed(...)`（→ gRPC `FAILED_PRECONDITION`），message 含 `REFUND_ADMISSION_CONFLICT` 与 `CONSUMED permit without shipment`；`WithDetails` 固定 key（大小写固定）：`refund_id/order_no/group_no/permit_state="CONSUMED"/probe_status="NO_SHIPMENT"/reason="CONSUMED_WITHOUT_SHIPMENT"/manual_queue="AC-27-NOWB-01"`。不新增错误码、不改错误码表。
- **Payment 口径（B-2）**：本 amendment 只禁止 **`Payment.Refund`/`RefundBatch`**（退款命令/资金写）；`Payment.GetIntent` 是 Tx1 前的只读快照（`refund_admission.go:168-175`），**允许且必要**。不得把 `GetIntent` 写成违规，也不得声称“Payment 调用 0”。
- **Tx1 占位**：普通正金额 `CONSUMED` 的 Tx1 写 `result=POST,state=PENDING_REMOTE` 占位；TCC `CONSUMED` 的 Tx1 写 `result=state=PENDING_REMOTE`。正常 `CANCELLED→PRE`/`SHIPPED→POST` 时 Tx2 覆盖为终态；`NO_SHIPMENT` 时 Tx2 把 `result` 覆盖为 CONFLICT、`state` 保持 `PENDING_REMOTE`。**不新增**字段；只新增 `result` 的冲突值（人类 A 授权 CONFLICT），`state` 不新增值。

### 3.2 为什么 `NO_SHIPMENT` 不与 AC-50 的 `NONE→REVOKED` 冲突

- AC-50 `0002:219`：许可 `NONE` 且运单为**无**/`CREATED`/`CANCELLED` → 置 `REVOKED` → `PRE_SHIP_REVOKED`（无运单时幂等成功）。
- 因此“无运单”本身**不是**异常；异常是 **`permit=CONSUMED` 且无运单、且本次为整组金额**（`0002:223`）。
- 本 amendment 只改**远程探测调用点**的映射：正金额路径只在 `PENDING_REMOTE`（由 `CONSUMED` 分支写入）时调 `finishRemoteAdmission`；零元路径只在 `permitConsumed` 时调用 `CancelIfCreated`。`NONE/REVOKED` 分支**不得**调用探测，也不得因为本修复被改成“无运单 → conflict”。
- 任何把“无运单一律 conflict”的实现都是**误读**，会破坏 AC-50 `NONE` 分支与 legacy G1 初始化，Reviewer 应直接 FAIL。

### 3.3 legacy / TCC 的归属（不新增映射点）

| 来源 | 现有路径 | 本裁决要求 |
|---|---|---|
| 普通售后（`source=aftersale`） | AfterSales worker → `Order.AdmitRefund` → `finishRemoteAdmission` | 同一映射，无例外 |
| SYSTEM 缺货（`source=system_shortage`） | 同上 | 同一映射，无例外 |
| TCC 补偿（`source=tcc`） | `tcc_admission.go:admitTccApplication` → `Order.AdmitRefund` → `finishRemoteAdmission` | 同一映射；**禁止**在 `tcc_*.go` 再写一份 `CANCELLED→PRE`；Tx1 `result=PENDING_REMOTE` 是既有偏差（§6.2），读者必须看 `state` |
| legacy（历史单/无许可行 G1） | `ensurePermitTx` 初始化后走同一 `AdmitRefund` | 同一映射；legacy “无许可行”是 `NONE`（合法 PRE），不是 `CONSUMED+无运单`；legacy 已发货缺口见 M-1/§12 |
| 零元（`source=system_zero`） | `admitZeroRefund` → `decideZeroRefundResult` | 同一映射语义（§4）；无 Tx1，故无 CONFLICT 持久化 |

### 3.4 R-2b：部分金额协议（A 路线，无预读，M-2）

**约束**：人类 A 删除 R-2a 与一切 `ListShipments`/事务外预读；不得开 ADR §5.2.2 例外；不得丢 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`，不得把部分金额静默当 POST/PRE。

| permit | 金额 | Tx2 探测 | 结论 | 写入/队列 |
|---|---|---|---|---|
| `NONE`/`REVOKED`/无行 | `< 组剩余` | 不探测（Order 自有 permit 事实即 pre-ship） | `PARTIAL_REFUND_REQUIRES_FULL_GROUP` | Tx1 前拒绝，**零 DB 写入**（现有 `:249-252`） |
| `CONSUMED` | `== 组剩余`（整组） | 见 §3.1 | PRE / POST / CONFLICT | 见 §3.1 |
| `CONSUMED` | `< 组剩余` | `CANCELLED`（pre-ship） | **`PARTIAL_REFUND_REQUIRES_FULL_GROUP`**；不写 PRE | Tx1 占位/commitment 保留；alert `reason=PARTIAL_REFUND_ON_PRE_SHIP_CANCELLED`；人工队列；0 释放/0 退款 |
| `CONSUMED` | `< 组剩余` | `ALREADY_SHIPPED`/`RECEIVED` | `POST_SHIP_ALLOWED`（部分金额合法，AC-50 `0002:226`） | Tx2 POST；commitment 已在 Tx1；不释放 |
| `CONSUMED` | `< 组剩余` | `NO_SHIPMENT` | **`PARTIAL_REFUND_REQUIRES_FULL_GROUP`**（V15-3 部分优先）；**不写 CONFLICT** | Tx1 占位/commitment 保留；alert `reason=PARTIAL_REFUND_ON_NO_SHIPMENT`；人工队列；0 释放/0 退款 |
| `CONSUMED` | `< 组剩余` | `TRANSIENT_ERROR`/未知 | `Unavailable` | PENDING 保持；可重试 |

- **优先级**：`0002:217`/V15-3 的部分金额前置优先于 `NO_SHIPMENT` 的 CONFLICT 标记；CONFLICT 标记只用于**整组金额**（`amount == 组剩余`）。V15-3 明确“部分金额先按 AC-50 返回 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`”，因此部分金额 + `NO_SHIPMENT` **不写 CONFLICT**；若人类要求部分金额也标 CONFLICT，会与 V15-3 冲突，需另裁决。
- **为什么不在 Tx1 前用远程预读取舍**：人类 A 明确禁止预读；**为什么不在 Tx2 把部分金额标成 CONFLICT/PRE**：会丢 PARTIAL 语义或静默放行。因此接受“Tx1 占位 + 人工队列”的代价（人类 A）。
- **部分金额的人工终态**：`CANCELLED`/`NO_SHIPMENT` 后组已无可发运单（取消或本来无行），部分退款被拒；人工队列处置：①用户改整组退款 → 旧 `RESERVED` 先经 AC-62/D-24 授权释放（旧申请 `REJECTED/CANCELLED` 或人工放弃），再以**新 `refund_id`** 走整组准入；②运维按 AC-62 授权释放并记录；③若已离仓，按 POST 重新准入。**不得**自动释放、不得自动把旧申请改金额、不得用同一 `refund_id` 改金额重试。
- 重放同 `refund_id`：admission 仍为 `PENDING_REMOTE`（未写 CONFLICT），重新探测，确定性回到同一 PARTIAL；无新增写入。
- 零元：`amount=0` 在零元单上等于组剩余 0（整组），不适用本表；正金额零元来源在更早处被拒绝。
- **判定位置/口径**：Tx2 在拿到探测结果后、写 Tx2 之前，用**本地 Order 事实**重算 `组剩余`（`groupPaid + groupFreight − Σ(RESERVED/SETTLED)`，与 `AdmitRefund:228-248` 同口径；**Σ 必须排除本 `refund_id`**——Tx1 已为本笔写 commitment，重算时排除或加回，否则会把本笔误判为部分），比较 `row.AmountCent` 与 `组剩余`：相等=整组，走 §3.1；小于=部分，走本表。**不调用 Fulfillment/Payment 读、不预读**；TX1 锁内与 Tx2 重算不一致时以 Tx2 锁内/读到的当前值为准，若分类变化按更严格者处理（部分优先，宁可 PARTIAL 不误 PRE）。
- 口径说明：若运费责任只部分可退，`整组退款金额` 的精算属 INV-47/T3，本 amendment 不改变该口径。

### 3.5 R-2c：整组 `NO_SHIPMENT` 的 CONFLICT 持久化与重放

1. Tx1 照旧（admission 占位 + commitment + 普通正金额占位 outbox）。
2. Tx2 调 `CancelIfCreated`；`NO_SHIPMENT` → **同一 Tx2** 以 `WHERE refund_id=? AND state='PENDING_REMOTE' AND result <> 'REFUND_ADMISSION_CONFLICT'` CAS 写 `result=REFUND_ADMISSION_CONFLICT`（`state` 不变）、`version+1`；不写 release/终态 outbox；Error 告警（§8）。
3. commitment 保持 `RESERVED`；permit 保持 `CONSUMED`；`orders.status`/`refund_state` 不写；组未关闭。
4. 同 `refund_id` 重放：`AdmitRefund` **必须在 `Payment.GetIntent` 与任何远程调用之前**先读 existing；`result/state ∈ {PRE,POST}` → 返回已持久化结果；`result=CONFLICT`（`state=PENDING_REMOTE`） → **返回同一 `REFUND_ADMISSION_CONFLICT` 错误**（同一 details），**不再调用 `CancelIfCreated`/`GetIntent`**、不再写任何行；`state=PENDING_REMOTE` 且 result 非 CONFLICT → 继续既有流程（GetIntent + `finishRemoteAdmission`）。
5. 并发：两个 Tx2 都拿到 `NO_SHIPMENT`；一个 CAS 成功写 `result=CONFLICT`，另一个 0 行（`result` 已为 CONFLICT）→ 读回 CONFLICT → 返回同一错误；不重复写/不重复告警（告警可每请求一条，但不得重复写）。
6. 崩溃：Tx1 后崩溃 → 重放 PENDING（result 非 CONFLICT）→ 探测 → CONFLICT；Tx2 写 CONFLICT 后崩溃 → 重放读 `result=CONFLICT` → 同一错误；`CancelIfCreated` 返回 `NO_SHIPMENT` 后、Tx2 前崩溃 → 重放 PENDING（result 非 CONFLICT）→ 再探测 → CONFLICT。
7. **人工重放**：正常 replay 不探测；`AC-27-NOWB-01` 终态①要求“补齐后按 AC-50 合法分支重放→PRE”。在 CONFLICT 持久化下，人工重放 = ops 在**真实既有/曾存在但丢失的 S6 证据**与授权/审计下执行**受控 re-arm**（§8.3），把 `result=CONFLICT` 清除为 Tx1 占位值（普通正金额 `POST_SHIP_ALLOWED`；TCC `PENDING_REMOTE`；`state` 保持 `PENDING_REMOTE`），再走正常 replay → 探测 → CANCELLED→PRE；若仍 `NO_SHIPMENT` → 再次 CONFLICT。**不得**用普通 replay 冒充人工重放，不得自动 re-arm。
8. CONFLICT 是 **`result` 列的持久化冲突标记**（`state` 仍 `PENDING_REMOTE`），不是准入终态；AC-27A 第 6 条按 `state=PENDING_REMOTE` / result 非 PRE → 未关闭 + 告警。

---

## 4. R-3：零元路径（`source=system_zero`）

### 4.1 裁决

`decideZeroRefundResult`（`refund_admission.go:366-396`）必须与 §3.1 同表：

| permit 状态 | `CancelIfCreated` 返回 | 零元结论 | 写入 |
|---|---|---|---|
| `""`（无许可行）/`NONE`/`REVOKED` | 不调用探测 | PRE | admission PRE + `refund_applications(DONE, admission_result=PRE)` + INV-44 释放（现状） |
| `CONSUMED` | `CANCELLED` | PRE | 同上（现状） |
| `CONSUMED` | `ALREADY_SHIPPED`/`RECEIVED` | POST | admission POST + `DONE(POST)`；不释放（现状） |
| **`CONSUMED`** | **`NO_SHIPMENT`** | **`REFUND_ADMISSION_CONFLICT`（错误）** | **不写 admission、不写 `refund_applications`、不写 `DONE`、不释放、不调 `Payment.Refund/RefundBatch`** |
| `CONSUMED` | `TRANSIENT_ERROR`/空/未知 | 不可判定 | 不写；`apperr.Unavailable` 可重试（现状） |

- 零元是“先取消、后单事务写 DONE”（T1.4a/R-C）。`NO_SHIPMENT` 时远程没有副作用（无行可取消），直接返回 conflict，单事务不开启。
- **零元无 Tx1，故不持久化 CONFLICT 标记**；同 `refund_id` 重放会重新探测（无写入），直到真实 `CANCELLED` 行出现 → PRE + DONE，或人工放弃（AC-62 授权/`FAILED_PERMANENT`）。
- **零元不调用 Payment**：`admitZeroRefund` 在 `refund_admission.go:154-156` 直接短路，`GetIntent`/`Refund` 均为 0；本路径“Payment 0”字面成立。
- 零元冲突**不得**借用 `writeZeroRefundTx` 的 `PENDING_REMOTE` 分支；`DONE` 的幂等语义不变（`zeroRefundReply`）。

### 4.2 禁止

1. 不得把零元 conflict 写成 `DONE(zero_amount)` 或任何 `refund_applications` 行。
2. 不得在 conflict 时释放预留/回库（INV-44/INV-50）。
3. 不得用“无运单”跳过 permit 分支：`NONE` 的零元仍按 AC-50 置 `REVOKED → PRE`。
4. 零元 `amount=0` 在零元单上即整组，不适用部分金额表。

---

## 5. R-4：单一映射权威（反 if-else 扩散）

- 两个映射点（`finishRemoteAdmission`、`decideZeroRefundResult`）必须使用**同一张**状态→结论表（一个包级 helper/常量表）。禁止在 AfterSales/TCC/正金额/零元各写一份 `switch status`。
- 映射函数必须**穷尽 + fail-closed**：未列出值（含未来新增值）→ 不可判定/可重试，绝不落到 PRE。
- 代码评审判据：搜索 `"CANCELLED"` 在 Order 中的出现点；除该唯一映射表外，任何新增 `CANCELLED→PRE` 分支即 FAIL（宪法 I.1）。
- **禁止**为了部分金额重新引入 `ListShipments`/事务外预读；部分金额的决定只用 permit 事实 + Tx2 探测结果（§3.4）。

---

## 6. R-5：持久化与不变量

| ID | 不变量 | 违规示例 |
|---|---|---|
| **A1** | `NO_SHIPMENT`（整组）只产生 `REFUND_ADMISSION_CONFLICT`；**永不**产生 `PRE_SHIP_REVOKED`/`POST_SHIP_ALLOWED` | 把无行当已取消 → PRE |
| **A2** | 准入成功终态当且仅当 `state ∈ {PRE,POST}` 且 `result ∈ {PRE,POST}` 且 `state=result`；`PENDING_REMOTE` 不是结果；`result=REFUND_ADMISSION_CONFLICT`（配 `state=PENDING_REMOTE`）是冲突标记、不是准入终态；TCC 的 `result=PENDING_REMOTE` 是既有偏差（§6.2） | 只凭 `state==result` 或只读 `result` 判关闭/放款 |
| **A3** | **本 amendment 只消除 `CONSUMED+无运单`（整组）的非法 PRE**。`PRE_SHIP_REVOKED` 的合法来源：`NONE→REVOKED`（依赖 permit 与 S6 一致；当前 Order 不校验）、`REVOKED` 幂等、`CONSUMED+CANCELLED`（真实取消）。legacy 无许可行 + 已发货仍可能写 PRE，是既有缺口（M-1），不得声称已消除 | 把 legacy/无 permit 的 PRE 也写成“已证明合法” |
| **A4** | **自动路径**在无准入成功终态时：不得调用 `Payment.Refund`/`RefundBatch`、不得写 release/commitment release、不得把申请置 `DONE`/`PROCESSING`。**唯一例外**：AC-62/D-24 人工授权 `RESERVED→RELEASED`（只释放额度，不恢复 permit、不关闭组、不当作退款完成、不写 `DONE`） | 自动释放；或把 AC-62 授权释放也禁掉 |
| **A5** | 冲突不改变 permit：`CONSUMED` 保持 `CONSUMED`，不得写 `REVOKED`，不得恢复/重建 permit（INV-45） | 为“关闭组”把 permit 改成 REVOKED |
| **A6** | 冲突不写 `orders.status`、不写 `refund_state`（INV-01/AC-25/AC-27A） | 用冲突推进生命周期 |
| **A7** | Tx1 的 `order.refund_admitted` 是非终态**占位事件**（普通正金额 `state=PENDING_REMOTE,result=POST`；TCC 的 CONSUMED Tx1 不写占位）；CONFLICT 不新增终态事件；占位保留，消费者必须忽略非终态事件 | 把占位 `POST/PENDING` 当终态；CONFLICT 再补一条 outbox |
| **A8** | Tx2 终态/冲突更新必须 `UPDATE ... WHERE refund_id=? AND state='PENDING_REMOTE' AND result <> 'REFUND_ADMISSION_CONFLICT'` 并检查 `RowsAffected()==1`；0 行 → 重读：nil → `Internal/Conflict("admission disappeared during Tx2")`；`result=CONFLICT` → 返回同一 CONFLICT 错误；PRE/POST → 返回该结果；`state=PENDING_REMOTE` 且 result 非 CONFLICT → `Conflict` 让调用方重读；**所有 0 行路径都不写 outbox/release** | 两个并发 Tx2 各写 outbox/release；0 行当成功；覆盖 CONFLICT |
| **A9** | `REFUND_ADMISSION_CONFLICT` 是 **`result` 列的持久化冲突标记**（`state` 保持 `PENDING_REMOTE`），不是准入终态：不写 PRE/POST、不打款、不释放、不关闭；AC-27A 按 `state=PENDING_REMOTE`/result 非 PRE → 未关闭；普通 replay 返回同一 CONFLICT 不再探测；只有 ops 受控 re-arm（§8.3，真实既有/曾存在但丢失的 S6 证据 + 授权 + 审计）才能清除该标记并重探 | 把 CONFLICT 当 PRE/关闭；自动 re-arm；用迟到 placeholder 当证据 |

### 6.1 与 AC-27A 的关系（不修改 AC 文本）

- AC-27A 的关闭谓词**只信** `result=PRE_SHIP_REVOKED`（且 `state` 已终态）；不使用运单事实、`refund_state`、`reservation_state`（`0001:622-623`、`:645`）。
- 本 amendment 的贡献是**前提保证**：新写入的 `PRE_SHIP_REVOKED` **不再**由“无运单行（整组）”产生（A3）；`REFUND_ADMISSION_CONFLICT` 与 `PENDING_REMOTE` 均**不关闭**。
- 本 amendment **不修改** AC-27A 文本（SPEC 归 PO）、**不实现** AC-27A 第 5/6 条读模型（归 R0.3-R1c）。
- **存量 PRE 不因本修复自动合法**：§8 一次性审计未完成前，AC-27A 第 5 条不得对无法证明来源的 PRE 组标已关闭/AC-VERIFIED。
- 在 `AC-50-FIX-01` 关闭前，AC-27 不得 AC-VERIFIED（`0001:874`）；本文件通过不构成本条关闭。

### 6.2 TCC `result=PENDING_REMOTE` 的既有偏差（M-4）

- SPEC-0001 `0001:645` 写“`result` 只允许 `PRE_SHIP_REVOKED`/`POST_SHIP_ALLOWED`”；但 TCC Tx1 `ensureTccRemoteAdmissionTx` 写 `result=PENDING_REMOTE,state=PENDING_REMOTE`（`tcc_settlement.go:847-852`）。这是既有偏差，不是本 amendment 引入。
- 读者口径以 A2 为准：`state ∈ {PRE,POST}` **且** `result ∈ {PRE,POST}` **且** `state=result` 才是准入成功；AC-27A 读模型（R0.3-R1c）必须按此实现。
- 清理登记 `TCC-PENDING-RESULT-01`（owner Architect/Developer，expiry 2026-12-31）：要么不再写中间 `result`，要么由 SPEC-0002 明确允许；不得在 AC-50-FIX-01 中夹带。

---

## 7. R-6：幂等 / 并发 / 崩溃窗口（强制）

### 7.1 整组冲突路径时序（正金额/TCC）

```text
AfterSales/TCC durable task
   │ AdmitRefund(refund_id, group, amount)
   ▼
Order: 读 existing admission（先于任何其他逻辑）
   ├─ result/state ∈ {PRE,POST} → 返回已持久化结果
   ├─ result=CONFLICT（state=PENDING_REMOTE）→ 返回同一 REFUND_ADMISSION_CONFLICT（不再探测）
   ├─ state = PENDING_REMOTE → finishRemoteAdmission（重探/收敛）
   └─ 不存在 → Tx1: 读 permit
         ├─ NONE → CAS REVOKED → PRE（合法）
         └─ CONSUMED → 写 admission 占位 + commitment + 占位 outbox
   ▼
 CancelIfCreated(order, group) —— 事务外（无 ListShipments/预读）
   ├─ CANCELLED（整组）→ Tx2 PRE
   ├─ ALREADY_SHIPPED/RECEIVED → Tx2 POST
   ├─ NO_SHIPMENT（整组）→ Tx2 CONFLICT + 告警
   └─ 部分金额 → 按 §3.4（CANCELLED/NO_SHIPMENT→PARTIAL；SHIPPED→POST）
   ▼
CONFLICT: result=REFUND_ADMISSION_CONFLICT（state 仍 PENDING_REMOTE）；组未关闭；commitment RESERVED；占位 outbox 保留；0 打款/0 释放/0 准入终态；人工队列
```

### 7.2 崩溃/并发/重复窗口

| # | 窗口 | 行为 | 恢复 owner |
|---|---|---|---|
| W1 | Tx1 提交后、远程前崩溃 | 重放读 PENDING → 重探；按结果 PRE/POST/CONFLICT/PARTIAL。Tx1 写入已存在 | 发起方 task + Order |
| W2 | `CancelIfCreated` 返回 `NO_SHIPMENT`、Tx2 前崩溃 | Tx1 已写；重放 PENDING → 再探测 → CONFLICT；不写 PRE | 发起方 task |
| W3 | `CancelIfCreated` 返回 `CANCELLED`、Tx2 前崩溃 | 重放探测 `CANCELLED`（幂等）→ Tx2 PRE | 发起方 task |
| W4 | Ship 与取消竞态 | Ship 胜 → ALREADY_SHIPPED → POST；取消胜 → CANCELLED → PRE；Ship 后置失败得 SHIPMENT_CANCELLED（AC-48） | Fulfillment CAS + Order |
| W5 | 同 `refund_id` 并发 | 订单行锁串行化；Tx2 A8 CAS 唯一化；败者读回同一结果；admission 至多一行 | Order |
| W6 | CONFLICT 后 Tx2 写成功、返回前崩溃 | 重放读 `result=CONFLICT` → 同一错误；**不再探测**（尽管 state 仍 PENDING_REMOTE） | 发起方 task + Order |
| W7 | 零元：远程 `CANCELLED` 后、单事务前崩溃 | 重放重探 → CANCELLED → PRE + DONE；零元无 commitment/Payment | 发起方 task |
| W8 | `PENDING_REMOTE` 永久悬挂（task FAILED/DLQ/租约丢失） | 保持未关闭并告警；人工重放（AC-61）；重放仍 `NO_SHIPMENT` → CONFLICT（整组）/PARTIAL（部分）→ AC-27-NOWB-01；**不是**本 amendment 的修复范围（AC-27-LIVE-01）。TCC 停 ADMITTING | ops（重放）+ Developer（AfterSales/TCC task） |
| W9 | `order.refund_admitted` 重复/乱序；迟到 `order.paid` | consumer 只处理终态 PRE；`NO_SHIPMENT` no-op success；重复 PRE 的 `CancelIfCreated` 幂等；**迟到 `order.paid` 可能经正常 consumer 创建真实 `CREATED` placeholder（`autoShipAllowed` fallback，`fulfillment/consumer.go:111-123`；`settlement.go:210-225`）**；该行是真实 S6 行、不是人工证据。若 admission 仍 PENDING → 重探可取消→PRE；若已 CONFLICT → **不自动探测/不自动转 PRE**，由 Fulfillment/ops 取消并走人工队列。业务若要求阻断该路径 → `ORDER-PAID-PLACEHOLDER-01` 独立 L2 | Fulfillment consumer + ops |
| W10 | 部分金额 + `CONSUMED` + `CANCELLED`/`NO_SHIPMENT` | 返回 PARTIAL；不写 PRE/CONFLICT；Tx1 占位/commitment 保留；告警进人工队列；重放重探确定性同一结果 | 发起方 task + ops |
| W11 | CONFLICT 后人工 re-arm | ops 在真实既有/曾存在但丢失的 S6 证据 + 授权 + 审计下清除 `result=CONFLICT`（state 仍 PENDING_REMOTE）；重放探测 → CANCELLED→PRE / SHIPPED→POST / NO_SHIPMENT→CONFLICT；不得自动 re-arm、不得用迟到 placeholder 当证据 | ops（§8.3） |

### 7.3 重试与“已持久化结果”的口径

- AC-50 `0002:227`：同 `refund_id` 重复申请先读**已持久化结果**并返回同一结果。
- 本裁决：`state=PENDING_REMOTE` 且 `result` 非 CONFLICT = 结果尚未产生 → 重探；`state/result ∈ {PRE,POST}` = 准入成功终态 → 直接返回；`result=CONFLICT`（`state` 仍 `PENDING_REMOTE`）= 冲突标记 → 直接返回同一 `REFUND_ADMISSION_CONFLICT`，**不再探测**（人类 A）。
- conflict **不是**准入结果，不得写成 PRE/POST；不得为了“幂等返回”新增 `result=CONFLICT` 之外的 enum（CONFLICT 值本身由人类 A 授权）。
- 发起方重试上限/终态不在本 amendment 内：AfterSales `maxTaskAttempts=8`（`aftersales/service.go:58`）耗尽后 `FAILED`，即人工队列信号；错误文本必须含 `REFUND_ADMISSION_CONFLICT` 以便落 `last_error`（`workers.go:724-748`）。禁止在 AfterSales 增加“conflict 当成功”分支。
- TCC 冲突不经过 AfterSales：batch 停 `ADMITTING`、application 停 `PENDING`；发现见 §8；重放由 ops/发起方。

### 7.4 Tx1 已写事实的处置协议（B-1 保持）

**前提**：`CONSUMED` 的 Tx1 在远程探测前提交（`refund_admission.go:253-283`）。普通正金额写 admission 占位 + commitment + 占位 outbox；TCC 的 `CONSUMED` Tx1 写 admission(`result=state=PENDING_REMOTE`)+commitment（`tcc_settlement.go:847-852,898`），**不写** outbox。**不得声称“无写入/无 outbox”**。

| 对象 | `NO_SHIPMENT`（整组）时状态/处置 | 理由 |
|---|---|---|
| `order_refund_admissions` | Tx2 **写 `result=REFUND_ADMISSION_CONFLICT`，`state` 保持 `PENDING_REMOTE`，version+1**（CAS `state=PENDING_REMOTE AND result <> 'REFUND_ADMISSION_CONFLICT'`）；不删行 | 持久化冲突标记，供重放返回同一错误（人类 A）；删除会丢幂等锚点/证据 |
| `order_refund_commitments` | `RESERVED`（Tx1 已写 1 条）**保留，不释放** | INV-45；AC-62 只有授权释放/打款才离开 RESERVED |
| outbox `order.refund_admitted` | Tx1 占位保留（普通：`result=POST,state=PENDING_REMOTE`；TCC：无占位）；**不新增终态事件** | A7；占位不是准入事实 |
| INV-44 release | **不写** | 只有 PRE 才发 release |
| commitment release | **自动路径不写**；只有 AC-62/D-24 人工授权可写 | INV-45/AC-62 |
| Payment | `GetIntent` 已读；**不调用 `Refund/RefundBatch`** | INV-43/A4 |
| `refund_applications`（TCC） | 保持 `PENDING`（不推进 `DONE/PROCESSING`） | 未准入 |
| permit | 保持 `CONSUMED` | INV-45/A5 |
| 终态 | 准入终态 0（无 PRE/POST）；`result=CONFLICT`（state 仍 PENDING_REMOTE）为冲突标记；人工队列 | SPEC-0001 `0001:863` |

- 同 `refund_id` 重放：读 CONFLICT → 同一错误（§3.5/§7.3）。
- 同组新 `refund_id`：旧 `RESERVED` 仍占额度；新申请会被额度校验挡为 `REFUND_ADMISSION_CONFLICT`（组剩余不足）或需先人工处置；不得重复承诺（AC-62/INV-45）。
- 人工放弃：经 AC-62/D-24 授权释放旧 `RESERVED`（只释放额度）后才允许新申请；组仍不关闭、permit 仍 CONSUMED。

### 7.4.1 为什么不用“Tx1 前只读存在性检查”（假设性论证，不采用；非 R-2a）

| 时序 | 事件 |
|---|---|
| T0 | 假设 Order 先做一次只读存在性探测（**本稿不采用、不实现**）：该组无行 |
| T1 | Fulfillment `CreateShipment` 插入 `CREATED` 行（`fulfillment/service.go:154-166`；不经过 `AuthorizeShipment`，permit 仍 `CONSUMED`） |
| T2 | 若 Order 以 T0 结果跳过 Tx1：无准入持久化；T1 的行真实存在，后续仍需 Tx1+探测，T0 已过期 |
| T3 | 真正的线性化点只能是 Tx1+探测：T1 的行被取消 → PRE；Ship 胜出 → POST；行再次消失 → CONFLICT |

- 人类 A 已裁定：**不引入任何 Tx1 前预读**（包括部分金额），接受 Tx1 已写 + 人工队列的代价。
- 本稿不再有 R-2a，也不调用任何事务外只读预读；§3.4 的部分金额决定只用 permit 事实 + Tx2 探测。
- 若未来要把 `CONSUMED` 的 Tx1 outbox 移到 Tx2，必须改 `AdmitRefund`/TCC 既有行为、扩 `allowed_paths` 并重做 W1–W3；本 amendment 不做（§13）。

---

## 8. R-7：人工队列（AC-27-NOWB-01）与告警

| 项 | 裁决 |
|---|---|
| 队列载体 | **复用** SPEC-0001 §16.2 `AC-27-NOWB-01`（`0001:863`）；**不新增**状态字段/表/队列模型；冲突用 `result=REFUND_ADMISSION_CONFLICT`（state 仍 `PENDING_REMOTE`）标记 |
| 组状态 | 保持未关闭（AC-27A：CONFLICT/PENDING_REMOTE/结果缺失 → 未关闭 + 告警）；permit 保持 `CONSUMED` |
| 申请状态 | 保持可重试（AC-50 `0002:228`）；不得打款、不得自动释放、不得置 `FAILED_PERMANENT`（除人工放弃，且需 SPEC-0002 owner 书面口径与 AC-62 授权） |
| 终态①（整组 CONFLICT） | Fulfillment 提供/修复**既有或曾存在但丢失**的真实 S6 取消证据（真实记录/承运商取消/仓出库撤销 + 恢复审计）→ ops 凭 Fulfillment 授权核对 → **受控 re-arm（§8.3）** → 重放探测 → CANCELLED → PRE → 组关闭。**不得为从未存在的运单新建行**；**不得把 CONFLICT 后由正常 `order.paid` 迟到的 placeholder 当证据**（W9） |
| 终态② | 无法补齐 → 申请保持可重试/人工队列；仅人工放弃时按 AC-62 记 `FAILED_PERMANENT` 并取得 SPEC-0002 owner 书面口径；**AC-62/D-24 授权释放旧 `RESERVED` 是唯一释放路径**（只释放额度、不恢复 permit、不关闭组、不写 `DONE`）；组保持未关闭并告警；不得自动关闭/自动退款/恢复 permit |
| 部分金额（R-2b） | `CANCELLED`/`NO_SHIPMENT` → PARTIAL + 告警 `reason=PARTIAL_REFUND_ON_PRE_SHIP_CANCELLED`/`PARTIAL_REFUND_ON_NO_SHIPMENT`；Tx1 占位/commitment 保留；人工队列；处置见 §3.4（AC-62 授权释放后新 `refund_id` 整组退款，或已离仓按 POST 重新准入） |
| owner / expiry | `Developer（Order 数据核对）/ops（重放、跨服务数据）/Fulfillment（S6 证据）`；**2026-12-31**（与 WIP waiver 同日；延期需重新裁决，`0001:856`） |
| 告警机制（固定 key/大小写） | Order `s.log.Error`，msg=`refund admission conflict: consumed permit without shipment`，字段 `refund_id/order_no/group_no/source/probe_status=NO_SHIPMENT/admission_result=REFUND_ADMISSION_CONFLICT/manual_queue=AC-27-NOWB-01`；部分金额 msg 按 reason 区分。`WithDetails` 的 `reason` 固定：`CONSUMED_WITHOUT_SHIPMENT`/`PARTIAL_REFUND_ON_PRE_SHIP_CANCELLED`/`PARTIAL_REFUND_ON_NO_SHIPMENT`。不得改名（T-6）。`deploy/**` 告警规则/runbook 不在本 amendment（单独 L2） |
| 队列发现（普通售后台） | `SELECT refund_id,order_no,group_no,result,state,updated_at FROM order_refund_admissions WHERE result='REFUND_ADMISSION_CONFLICT' AND state='PENDING_REMOTE'` + AfterSales `after_sale_refund_tasks.last_error LIKE '%REFUND_ADMISSION_CONFLICT%'` |
| 部分金额债务发现（R-2b） | `SELECT refund_id,order_no,group_no,result,state,updated_at FROM order_refund_admissions WHERE state='PENDING_REMOTE' AND result IN ('POST_SHIP_ALLOWED','PENDING_REMOTE') AND updated_at < NOW() - INTERVAL 10 MINUTE`，并必须与 Order 告警 `reason=PARTIAL_REFUND_ON_*` / AfterSales `last_error LIKE '%PARTIAL_REFUND_REQUIRES_FULL_GROUP%'` 交叉；**不得仅凭 PENDING 判定**（正常在途也会 PENDING）。owner=ops+Developer，expiry 2026-12-31 |
| 队列发现（TCC，M-7） | `SELECT b.batch_id,b.order_no,b.payment_no,a.refund_id,a.group_no FROM refund_batches b JOIN refund_applications a ON a.batch_id=b.batch_id WHERE b.state='ADMITTING' AND a.state='PENDING' AND b.updated_at < NOW() - INTERVAL 10 MINUTE` + Order `s.log.Error`（`tcc_admission.go:97-110`）。`refund_batches.last_error`/`refund_applications.last_error` 列存在（`migrations/order/0017_refund_batch_ledger.sql:28`），写入需改生产 `tcc_*.go`，登记 `TCC-LAST-ERROR-01`（DEFERRED） |
| 存量 PRE 审计（M-5，R2-M2 修正） | `SELECT a.refund_id,a.order_no,a.group_no,a.result,a.state,p.state AS permit_state,g.status AS group_status FROM order_refund_admissions a LEFT JOIN order_shipping_permits p ON p.order_no=a.order_no AND p.fulfillment_group_no=a.group_no LEFT JOIN order_fulfillment_groups g ON g.order_no=a.order_no AND g.group_no=a.group_no WHERE a.result='PRE_SHIP_REVOKED' AND (a.state='PRE_SHIP_REVOKED' OR a.state='PENDING_REMOTE')`；分别列出：`g.group_no IS NULL`（legacy G1 无 group 行）、`g.status IN ('SHIPPED','RECEIVED')`、`a.state='PENDING_REMOTE'`（不可核验）、`p.state IN ('REVOKED','CONSUMED')`。owner=ops+Developer，expiry 2026-12-31；**0 行 ≠ 安全**（先证明无历史 G1）。审计结论未出前，AC-27A 第 4/5 条不得对相关组标已关闭/AC-VERIFIED |
| 证据 | 单测捕获日志（注入 `slog.Handler`）+ 真实 MySQL 集成断言 CONFLICT 行/无 release/无退款命令；ops 用上述查询 |

### 8.2 与 SPEC-0001 §16.2 的终态对齐

| SPEC-0001 §16.2 | 本 ADR 的对应 |
|---|---|
| **AC-27-NOWB-01 阻断** | 整组 `CONFLICT`（或部分金额 `PENDING`）：组保持未关闭；退款申请 `REFUND_ADMISSION_CONFLICT`/`PARTIAL_REFUND_REQUIRES_FULL_GROUP`；不写 PRE/不打款/不自动释放；其他组不受影响 |
| **AC-27-NOWB-01 终态①** | Fulfillment 提供/修复**既有或曾存在但丢失**的真实 S6 取消证据 → ops 凭 Fulfillment 授权核对（Order 不写运单）→ §8.3 re-arm → 探测 `CANCELLED` → PRE → 组关闭；不得新建行、不得用迟到 placeholder 当证据 |
| **AC-27-NOWB-01 终态②** | 无法补齐 → 申请保持可重试并由人工队列持有；仅人工放弃时按 AC-62 记 `FAILED_PERMANENT` 并取得 SPEC-0002 owner 书面口径；AC-62/D-24 授权释放 `RESERVED` 是唯一释放路径；组保持未关闭并告警；不得自动关闭/自动退款/恢复 permit |
| **AC-27-LIVE-01** | `PENDING_REMOTE` 永久悬挂（DLQ/租约丢失）→ 保持未关闭并告警；AC-61 人工重放；重放仍 `NO_SHIPMENT` → 整组 CONFLICT / 部分 PARTIAL → 转 AC-27-NOWB-01；不写 PRE/不关闭/不打款 |

### 8.3 人工 re-arm runbook（AC-27-NOWB-01 终态①）

- **仅 ops**，在 Fulfillment 提供**真实既有/曾存在但丢失**的 S6 证据、ops 授权核对、审计工单齐备后执行；不得自动、不得由 worker/重试触发。
- 操作：`UPDATE order_refund_admissions SET result=<placeholder>,version=version+1,updated_at=NOW(3) WHERE refund_id=? AND result='REFUND_ADMISSION_CONFLICT' AND state='PENDING_REMOTE'`；`placeholder` 普通正金额=`POST_SHIP_ALLOWED`（与 Tx1 占位一致），TCC=`PENDING_REMOTE`；`state` 保持 `PENDING_REMOTE`。**不动 commitment/permit/outbox**；审计工单记录 refund_id/操作人/授权人/证据/时间。
- 随后以同 `refund_id` 重放 `AdmitRefund`：探测 `CANCELLED` → PRE → 组关闭；`SHIPPED` → POST；仍 `NO_SHIPMENT` → CONFLICT（保持人工队列）。
- **禁止**：用迟到 `order.paid` placeholder 当证据；用普通 replay 代替 re-arm；re-arm 后自动打款/释放（仍走 AC-50/AC-62 原流程）。
- 若组织不允许人工 DB 写：终态①改走“AC-62 授权释放旧 RESERVED + 新 `refund_id` 整组准入”（依赖 T3 release 实现）；在此之前 term① 保持未关闭（§1.3 stop）。

---

## 9. R-8：登记面

| 登记项 | 是否需要 | 内容 / owner | 证据 |
|---|---|---|---|
| `api/proto/mall/v1/mall.proto` 注释 | **必须** | `CancelIfCreatedReply.status` 列出 `NO_SHIPMENT` 及五值语义；RPC 注释补“or report the group has no shipment row”。owner Developer；Architect 文本见 §2.1。**不加字段/不加 RPC** | `mall.proto:1043`、`:1089-1098`；`make proto` 后 `api/gen/mall/v1/mall.pb.go` + `mall_grpc.pb.go` 注释同步 |
| `api/gen/**` | **仅生成** | `make proto`（`Makefile:153-157`）生成；禁止手改。**T-2**：固定 `protoc`/`protoc-gen-go` 版本（当前生成头 v1.36.12/protoc v7.36.2）；`git diff -- api/gen` 只允许两文件注释行；任何字段/方法签名/编号 diff 即 FAIL | `scripts/check-proto-evolution.py`；`make proto-guard`（`Makefile:147-151`） |
| `docs/contracts.md` | **不需要** | 仅规定“用 `apperr` 映射 gRPC status”（`contracts.md:235-239`），无逐 RPC 状态目录；ADR §5.3 是契约正文 | `docs/contracts.md:235-239` |
| 错误码 | **不新增** | `REFUND_ADMISSION_CONFLICT`（`0002:517`、`0007:988`）与 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`（`0002:217`）；`apperr.Failed`→`FAILED_PRECONDITION`。canonical token 在 message/details | `apperr.go:61-64`；`refund_admission.go:312-328` |
| `order_refund_admissions.result` 新值 | **人类 A 授权；无 schema 变更** | `result=REFUND_ADMISSION_CONFLICT`（VARCHAR(32) 可容纳）；**`state` 不新增值**（保持 `PENDING_REMOTE`）。不是新字段/迁移。与 SPEC-0001 `0001:645` 注（result 只允许 PRE/POST）漂移：**owner PO** 在下次 SPEC-0001 升版同步（`CONFLICT-SPEC-SYNC`，2026-12-31）；PO 拒绝 → 实现停止 | `migrations/order/0016_settlement_admission.sql:7-19`；`0001:645` |
| 事件/payload | **不需要** | CONFLICT 不新增 `order.refund_admitted`；Tx1 占位按 A7 解释 | `refund_admission.go:610-619` |
| **T-1** SPEC-0002 错误表漂移 | `[TODO]` → PO | `0002:517` 未覆盖 `0002:223`；owner PO，2026-12-31；无 per-item stop 已补 §1.3 | `0002:223`、`:517` |

---

## 10. 测试矩阵（实施强制最小集）

> 类型：U=单元/fake port；I=真实 MySQL 集成；R=`-race`；L=日志断言；N=NATS 集成。**setup**：fresh=新申请；preseed=预置 Tx1 行/commitment/占位；done=已有终态/冲突。绑定精确 HEAD，日志入库由 Developer 提供；不得用全绿 mock 冒充。**无 `ListShipments` 用例**（R-2a 已删）。

| # | setup | 场景 | 层 | 断言 | 期望 |
|---|---|---|---|---|---|
| **F1** | — | Fulfillment：无运单行 | U/I | `status="NO_SHIPMENT"`；`shipments` 0 行；无 outbox/事件 | 新增 |
| **F1b** | — | Fulfillment：`shipment_no` 非空 + 无行 | U/I | `NO_SHIPMENT`；不误取消同组其他行 | T-5 |
| **F2** | — | Fulfillment：真实 `CREATED` 行 | U/I | `CANCELLED` 且行变 `CANCELLED` | 回归 |
| **F3** | — | Fulfillment：已是 `CANCELLED` | U/I | 幂等 `CANCELLED` | 回归 |
| **F4** | — | Fulfillment：`SHIPPED`/`RECEIVED` | U/I | `ALREADY_SHIPPED`/`RECEIVED` | 回归 |
| **F5** | — | Fulfillment：`EXCEPTION`+`shipped_at` / 无 | U/I | `ALREADY_SHIPPED` / `TRANSIENT_ERROR` | 回归 |
| **F6** | — | Fulfillment：G1 无行、G2 有 `CREATED` 行 | U/I | 请求 G1 → `NO_SHIPMENT`；G2 不被取消 | 组隔离 |
| **F7** | N | consumer 收到 `NO_SHIPMENT`；重复 PRE | N/I | `NO_SHIPMENT` no-op success；重复 PRE 不重复取消 | M-8/M-11 |
| **F7b** | N | G1 late `order.paid`（真实 placeholder） | N/I | 允许创建真实 `CREATED`（正常 consumer）；PENDING 下重探可取消→PRE；已 CONFLICT 时不自动 PRE；记录为正常 consumer、非人工证据 | R2-M1/W9 |
| **F8** | — | 正常路径：`CONSUMED+CREATED`→PRE；`CONSUMED+SHIPPED`→POST | I | 真实行/真实 permit；不依赖合成无行 | M-12 |
| **O1** | fresh | 正金额 `CONSUMED` + **整组** + `NO_SHIPMENT` | I | `FAILED_PRECONDITION` 含 token；admission `result=REFUND_ADMISSION_CONFLICT,state=PENDING_REMOTE`；1 条 `RESERVED` commitment；普通占位 outbox 1 条；**0 终态 outbox/0 release**；permit `CONSUMED`；`refundCalls==0` | **核心** |
| **O1b** | fresh | TCC `source=tcc` + **整组** + `NO_SHIPMENT` | I/R | admission `result=REFUND_ADMISSION_CONFLICT,state=PENDING_REMOTE`；batch `ADMITTING`；application `PENDING`；无占位/终态 outbox；`refundCalls==0` | TCC 共享 |
| **O2** | fresh | O1 同 `refund_id` 重放 | I | 返回同一 CONFLICT 错误；`cancelCalls` **不增加**（不再探测）；行数/commitment/outbox 不变 | 人类 A |
| **O2b** | preseed | 预置 `result=CONFLICT,state=PENDING_REMOTE` 后调用 | I | 同一错误；不探测；无新写入 | 重放 |
| **O2c** | preseed | 预置 `result=CONFLICT` + `Payment.GetIntent` 不可用 | I | 仍返回同一 CONFLICT；**不依赖 Payment 读**（existing 先于 GetIntent）；无探测/无新写入 | 幂等（R2-B1 对应） |
| **O3** | fresh | Tx1 后崩溃模拟（预置 PENDING + 占位 + commitment）→ 探测 | I | 写 `result=CONFLICT`（state 保持 PENDING_REMOTE）；无终态/释放/退款；计数仅新增该 result 更新 | W1/W2 |
| **O4** | fresh | 正金额 `CONSUMED` + 整组 + `CANCELLED`（单组） | I | PRE；admission PRE/PRE；1 条 commitment；release 按 INV-44（单组） | 回归 |
| **O4a** | fresh | `NONE`/`REVOKED` + **部分**（Order 自有 permit 事实） | I | `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；**0 admission/commitment/outbox**；不探测（`cancelCalls==0`）；无 release | R-2b |
| **O4b** | fresh | `CONSUMED` + **部分** + `CANCELLED` | I | `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；**不写 PRE/CONFLICT**；Tx1 占位/commitment 保留；`cancelCalls==1`；0 release/0 退款；告警 reason 正确 | R-2b |
| **O4c** | fresh | `CONSUMED` + **部分** + `ALREADY_SHIPPED` | I | POST；1 条 commitment；部分金额合法；不取消运单 | R-2b |
| **O4d** | fresh | `CONSUMED` + **部分** + `NO_SHIPMENT` | I | `PARTIAL_REFUND_REQUIRES_FULL_GROUP`（V15-3 优先）；**不写 CONFLICT**；PENDING/占位保留；告警 reason 正确；0 release/0 退款 | R-2b |
| **O5** | fresh | 正金额 `CONSUMED` + 整组 + `ALREADY_SHIPPED`/`RECEIVED` | I | POST；恰好 1 条 RESERVED commitment（Tx1 已有，Tx2 不重复）；不取消 | M-10 |
| **O5b** | fresh | TCC 已离仓（`recordTccPostAdmission`） | I | POST + commitment 同事务（既有，不改）；无探测 | M-10 |
| **O6** | fresh | 正金额 `CONSUMED` + `TRANSIENT_ERROR`/空/未知 | I | `Unavailable`；PENDING 保持；无终态/释放；`refundCalls==0` | fail-closed |
| **O6b** | preseed | 预置 PENDING + `TRANSIENT_ERROR` | I | `Unavailable`；计数不变 | M-13 |
| **O7** | fresh | 零元 `CONSUMED` + `NO_SHIPMENT` | I | conflict；admission 0 行；application 0 行；无释放；`refundCalls==0 && getCalls==0`；permit 不变 | **核心** |
| **O7b** | fresh | 零元 `CONSUMED` + `TRANSIENT_ERROR`/空/未知 | I | `Unavailable`；0 admission/application；无释放/无 Payment | T-4 |
| **O8** | fresh | 零元 `NO_SHIPMENT` 重放；随后真实 `CANCELLED` 行 | I | 第一次 conflict 0 写入；真实取消后重试 → PRE + DONE | 零元终态① |
| **O9** | fresh | `TRANSIENT_ERROR`/未知重放（普通） | I | 保持 PENDING；下次探测可收敛；不写终态 | 重试 |
| **O10** | fresh | legacy 来源 + `NO_SHIPMENT`（整组） | I | 同一 CONFLICT；legacy 无许可行（NONE）仍 PRE | 来源矩阵 |
| **O10b** | — | legacy 无 permit + 组已发货 | — | `[DEFERRED]`（M-1，Architect/PO，2026-12-31）：用 §8 审计定位；不得默认 PRE；修复后补反向测试 | 缺口 |
| **O11** | fresh | 同 `refund_id` 并发 + `NO_SHIPMENT` | I/R | 两路同一 CONFLICT；admission 1 行且 `result=CONFLICT,state=PENDING_REMOTE`；终态/释放 0；不覆盖为 PRE/POST | 并发 |
| **O11b** | fresh | 同 `refund_id` 并发 + `CANCELLED` | I/R | admission 1 行；终态 outbox 1；release ≤1（A8） | M-9 |
| **O11c** | preseed | Tx2 前删除/篡改 admission，0 行 | I | `Internal/Conflict("admission disappeared during Tx2")`；无 outbox/release；不 panic | R2-M5 |
| **O12** | fresh | 告警键（普通 + TCC + 部分） | L | 固定 key/reason（§8）；TCC 含 `batch_id/refund_id` | 可观测 |
| **O13** | preseed | AC-27A：CONSUMED + CONFLICT / PENDING | I | `groupIsClosed=false`；本修复不写 permit/不关闭 | 非回归 |
| **O13b** | preseed | TCC `result=PENDING_REMOTE,state=PENDING_REMOTE` | I | `groupIsClosed=false`（A2 双条件） | M-4 |
| **O14** | N | consumer 重复/乱序（PRE 重投、迟到 `order.paid`） | N | 不重复取消/不建重复 placeholder/不自动 PRE（CONFLICT 后） | M-8/W9 |
| **O15** | — | AC-62 人工授权释放（CONFLICT 后） | — | `[DEFERRED]`（T3；Developer/T3，2026-12-31）：不得用未授权释放/fixture 冒充；实现时断言只释放一次、不恢复 permit、组仍不关闭、未授权 → `REFUND_RELEASE_NOT_AUTHORIZED` | M-3 |
| **O16** | preseed | 人工 re-arm（清除 `result=CONFLICT`，state 保持 PENDING）后重放 | I | 探测 `CANCELLED`→PRE；审计/授权；不动 commitment/permit；不得自动 re-arm | §8.3 |
| **O17** | — | 部分金额旧承诺释放 + 新 `refund_id` 整组退款 | — | `[DEFERRED]`（T3）：依赖 AC-62 release writer；释放前不得新整组退款；不得改旧 refund_id 金额 | R-2b |

补充要求：

- **金额 Mock 真实性**：正金额 >0；零元 `pay_cent=0`、`settlement_payment_no='zero:<order_no>'`；不得用 0 元冒充正金额（宪法 IV.3）。
- **合成状态**：O1/O3/O7 的 `CONSUMED+无行` 是 fail-closed 合成异常（§0.2）；另有 F8 正常路径。
- **不得** `t.Skip()`、fixture 伪造 `CANCELLED` 行冒充生产证据、或 `if status=="NO_SHIPMENT" { return nil }` 绕过 O1/O7。O8/O16 的 fixture 是模拟 Fulfillment 真实证据后的状态，须注释，Reviewer 区分 fixture 与生产证据。
- **不得**为部分金额重新引入 `ListShipments`/预读；O4b/O4c/O4d 只用 permit + `CancelIfCreated` fake。
- O4/O4b/O4c 默认单组；多组释放缺口见 §14 `INV-44-MULTI-01`，不得用 Order fixture `RELEASED` 冒充。

---

## 11. 反模式自省（强制）

| 反模式 | 本设计如何阻止 |
|---|---|
| 用“无运单 → 当已取消”的和稀泥映射掩盖异常 | 新增 `NO_SHIPMENT`；整组写 CONFLICT，不写 PRE；O1/O7 断言 |
| 把 Tx1 已写说成“无写入/无 outbox” | A7/§7.4 承认并处置；O1/O3 断言占位 1、终态 0；O2b/O3 preseed 证明“无新增” |
| 把 `GetIntent` 当成退款/违禁调用 | 只禁 `Refund/RefundBatch`；`GetIntent` 只读允许；测试只断言 `refundCalls==0` |
| 为“制造取消证据”新建运单行 | 人工侧禁止；正常 consumer 的真实 placeholder 不是证据、CONFLICT 后不自动 PRE（W9）；阻断另立 L2 |
| 用 CONFLICT 当 PRE/关闭/放款 | A9：冲突标记不是准入终态；AC-27A 未关闭；无 release/payment |
| 自动 re-arm / 用普通 replay 冒充人工重放 | §8.3：仅 ops、真实 S6 证据 + 授权 + 审计；普通 replay 返回 CONFLICT 不探测 |
| 部分金额用预读或静默当 POST/PRE | R-2b：无预读；PARTIAL 语义保留；O4b/O4c/O4d |
| 在 AfterSales/TCC/零元各写一份 `switch status` | R-4 单一映射权威；Reviewer 搜 `"CANCELLED"` 出现点 |
| 两个并发 Tx2 各写事件/释放 | A8 CAS + 0 行语义；O11b/O11c |
| 用迁移/DB CHECK 兜底业务语义 | 无迁移；CONFLICT 是既有列值；未来 CHECK 另立 L2 |

---

## 12. 【假设与不确定性】（强制）

1. **`NO_SHIPMENT` 命名**：brief 建议、采用；改名不改语义/矩阵，需同步 proto/测试。
2. **“无行”事实边界**：依赖 `(order_no, fulfillment_group_no)` 唯一约束（`0003:9`）；未来同组多运单需重新裁决；当前不成立。
3. **`shipment_no` 语义**：Order 传空（现状）→ 组级探测；非空时 `NO_SHIPMENT` 只表示三键无行；发布前确认仓库外无调用方（T-5）。
4. **CONFLICT 值/状态**：人类 A 授权 `result=REFUND_ADMISSION_CONFLICT`；本稿 **`state` 保持 `PENDING_REMOTE`**（不新增 state 值），冲突以 result 标记；因此 replay/Tx2 必须**先判 `result=CONFLICT`**（早于 PENDING 分支），且 Tx2 CAS 必须带 `result <> CONFLICT`，否则并发会覆盖标记（A8/A9）。AC-27A 因 `state=PENDING_REMOTE`/result 非 PRE → 未关闭。SPEC-0001 `0001:645` 注漂移由 PO 同步（`CONFLICT-SPEC-SYNC`，2026-12-31）；PO 拒绝 → 实现停止。
5. **部分金额债务**：`CONSUMED` 部分金额在 pre-ship 结果下保留 Tx1 占位/commitment 并进人工队列（人类 A）；人工处置依赖 AC-62 授权释放（T3 未实现）或已离仓 POST 重新准入。**不得**自动释放/改金额。
6. **人工 re-arm**：把 CONFLICT→PENDING_REMOTE 的受控 re-arm 作为终态①的人工重放路径；仅 ops、真实既有/曾存在但丢失的 S6 证据 + 授权 + 审计。若组织禁止人工 DB 写，term① 改走“AC-62 释放 + 新 refund_id”，在 T3 前保持未关闭。
7. **迟到 `order.paid` placeholder**：`autoShipAllowed` fallback 可达（G1+CONSUMED+refund_state=NONE），会创建真实 `CREATED` 行；按人类 B-3 要求，本稿**删除** 0.2 的“不建 placeholder”断言/路径，W9 按真实行为重写，**不声称不可达**；生产 `autoShipAllowed`/`consumer.go` 不在 allowed_paths，若要阻断 → `ORDER-PAID-PLACEHOLDER-01` 独立 L2（2026-12-31）；阻断前不得声称“正常 consumer 不会建行”。
8. **`LEGACY-SHIPPED-PRE-01`（M-1）**：`ensurePermitTx`（`settlement.go:263-278`）无 permit 时建 `NONE`；`AdmitRefund` 对 `NONE` 不做 S6 校验（`refund_admission.go:249-288`）；历史已发货组可能被写 PRE。修复需改 `NONE` 分支语义，超出本 amendment；owner Architect/PO，2026-12-31；审计见 §8。
9. **`TCC-LAST-ERROR-01`（M-7）**：TCC 失败只 `s.log.Error`（`tcc_admission.go:97-110`），未写 `refund_batches.last_error`；写入需改生产 `tcc_*.go`，另立 `change_id`；当前用 §8 查询 + 日志。
10. **`TCC-PENDING-RESULT-01`（M-4）**：TCC Tx1 `result=PENDING_REMOTE` 与 SPEC 注不一致；清理归 Architect/Developer，2026-12-31；之前读者按 A2 双条件。
11. **`INV-44-MULTI-01`**：多组 release 未实现（`refund_release.go:178-187` 只在单组发事件）；AC-27 EARS-3/INV-44 不得标 VERIFIED；不得用 Order fixture 冒充。
12. **`ORDER-PAID-PLACEHOLDER-01`**：若业务不接受迟到 placeholder，需要在 `fulfillment/consumer.go`/`settlement.go` 侧阻断，属独立 L2；本 amendment 不声称已禁止“正常 consumer 建行”。
13. **`change_id` 拆分**：实施须由 Lead 另拆（`0001:879`）；不得把本文件当 R0.3-R1c 放行。
14. **可达性/未执行**：`CONSUMED+无行` 正常路径不可达（先建行后授权）；生产发生率 `[不确定]`。本文件未运行测试/迁移；代码行号是阅读事实，不构成执行证据。`[不确定]`：仓库外调用方；生产 `shipments` 是否已全部应用 `0003`；需 ops/迁移证据。

---

## 13. 不交付 / 明确不做（防范围漂移）

- **不交付 T2/T3/T4.4**；仍按 `docs/lead/PAY-SETTLE-01/tasks.md` 的既有 DEFERRED。
- **不改 SPEC-0002**、**不改 SPEC-0001**、**不改 accepted ADR Rev 2.3 正文**（未来 Rev 2.4 errata 由 Architect 合并，需人类批准）。
- **不新增**：RPC、proto 字段、事件 subject/payload、迁移/schema/CHECK、permit 状态值、worker、分布式锁、配置开关；**不新增 `ListShipments`/事务外预读**。
- **不改**：Payment 金额/退款实现、AfterSales task 策略、Inventory 释放协议、`orders.status`/`refund_state` 语义、Fulfillment consumer 生产语义、`autoShipAllowed`/`order.paid` 路径（阻断另立 L2）。
- **不做**：AC-27A 读模型实现（R0.3-R1c）；M-1 legacy 修复；M-4 TCC result 清理；T3 release/re-arm 工具化；把 Tx1 outbox 移到 Tx2 的重构——均另立 `change_id`/L2。

---

## 14. Baseline 影响 / INV / AC 映射

### 14.1 INV

| INV | 影响 | 本裁决如何保持 |
|---|---|---|
| INV-36（退款统一准入；`NONE→REVOKED`；已授权未离仓取消运单；同键幂等） | 涉及 | 只改 `CONSUMED` 探测映射与部分金额；`NONE` 分支不变（A3 收窄）；幂等由“读 existing + 重探 + A8”保证 |
| INV-42（申请不丢失、可重试、同键幂等） | 涉及 | conflict/partial 不丢申请；Tx1 admission/commitment 保留；同 `refund_id` 返回持久化结果 |
| INV-43（未得 PRE/POST 前不得调 Payment） | 涉及（核心） | CONFLICT/PARTIAL 不产生准入成功 → 禁止 `Refund/RefundBatch`（O1/O4b/O4d/O7/O9 断言 `refundCalls==0`）；`GetIntent` 只读不在此限 |
| INV-44（库存处置有主；释放失败重试告警） | 涉及 | CONFLICT/PARTIAL 不触发释放；PRE 后走既有持久释放；多组缺口 `INV-44-MULTI-01`（DEFERRED） |
| INV-45（准入不可逆；permit 终态不可恢复；释放纪律） | 涉及（核心） | CONFLICT/PARTIAL 不写 permit、不自动释放；AC-62 授权释放为唯一例外 |
| INV-50（零元不经过 Payment；DONE 为执行事实） | 涉及 | 零元 conflict 不写 DONE、不调 Payment；合法 CANCELLED 仍 DONE(PRE) |
| INV-01 / AC-25 | 不涉及（不得触碰） | 冲突不写 `orders.status`/`refund_state` |
| INV-46/47/48/49 | 不涉及 | 不改历史支付/行级/Σ/shadow |

### 14.2 AC 映射（证据归 Reviewer，本文件不更新台账）

| AC | 关系 | 说明 |
|---|---|---|
| AC-50（`0002:213-231`） | 直接修复 `0002:223` + 保留 `0002:217` | 整组 `CONSUMED+无运单` → `REFUND_ADMISSION_CONFLICT`（CONFLICT 标记）；部分金额 → `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；不写 PRE/不打款/不释放 |
| AC-27A（`0001:615-645`） | 前提保证，不改文本 | 新写 PRE 只来自 A3 合法分支；CONFLICT/PENDING 不关闭；存量审计前不得关闭/AC-VERIFIED；读模型归 R0.3-R1c |
| AC-27（`0001:592-613`） | 收口前提 | `AC-50-FIX-01` 未关闭前 AC-27 不得 AC-VERIFIED（`0001:874`） |
| AC-62 / INV-45（`0002:308-323`） | 约束 | CONFLICT/PARTIAL 不得自动释放；只有 AC-62/D-24 授权放弃才有 `FAILED_PERMANENT`/释放；释放不恢复 permit、不关闭组 |
| AC-48（`0002:200-206`） | 非回归 | permit `CONSUMED` 不同 `shipment_no` 仍拒绝；取消后不得复活 |
| AC-51/AC-65（TCC） | 非回归 | TCC 共用映射；batch 不得在 CONFLICT 时进入 ADMITTED/PAYING；发现见 §8 |
| AC-67/AC-68（零元） | 非回归 | 零元 conflict 不写 DONE；合法分支不变；零元无 Payment |
| AC-27-NOWB-01 / AC-27-LIVE-01（`0001:863-864`） | 人工队列/不收敛 | 终态见 §8；term① re-arm（§8.3）；term② 人工放弃；LIVE 重放仍冲突 → NOWB；均不自动关闭/退款 |

---

## 15. 实施边界与停止条件

### 15.1 施工图（最小 diff，不写代码）

1. **Fulfillment**：`cancel_if_created.go` 的 `sql.ErrNoRows` 分支返回 `NO_SHIPMENT`（不 commit、不建行）；其余分支不变；注释。
2. **Order 映射**：`finishRemoteAdmission` 在拿到探测结果后，用本地 Order 事实重算 `组剩余` 判定整组/部分（不远程读）：整组 `NO_SHIPMENT` → Tx2 写 `result=CONFLICT`（CAS `state=PENDING_REMOTE AND result <> 'REFUND_ADMISSION_CONFLICT'`）+ 告警；整组 `CANCELLED→PRE`；`ALREADY_SHIPPED/RECEIVED→POST`；部分金额按 §3.4（`CANCELLED/NO_SHIPMENT→PARTIAL` 不写 PRE/CONFLICT；`SHIPPED→POST`）；瞬态/未知→`Unavailable`。Tx2 按 A8 CAS；仅 PRE 触发既有释放。
3. **Order 重放**：`AdmitRefund` 在 `Payment.GetIntent`/任何远程调用**之前**先读 existing；PRE/POST → 返回；CONFLICT → 返回同一 CONFLICT 错误、不探测/不读 Payment；PENDING → 继续（GetIntent + `finishRemoteAdmission`）；不存在 → Tx1。`finishRemoteAdmission` 入口也必须先判 `result=CONFLICT` 并短路（防御其他调用路径），不得重探。
4. **Order 零元**：`decideZeroRefundResult` 的 CONSUMED + `NO_SHIPMENT` → conflict；不进入 `writeZeroRefundTx`；其余不变。
5. **proto**：只改 `CancelIfCreated`/`CancelIfCreatedReply` 注释；`make proto`；`make proto-guard` 通过。
6. **测试**：§10；真实 MySQL + `-race` + 日志/NATS；不含 ListShipments 用例。

### 15.2 停止条件（命中即停手，回 Architect/Lead）

1. 需要新增 proto 字段/RPC/事件字段、迁移、permit 状态、`result/state` 新值（`result=CONFLICT` 除外；`state` 不新增值） → 超出本 amendment。
2. 需要修改 AfterSales/Payment/Inventory/Deploy/`docs/specs/**`、生产 `tcc_*.go`、`consumer.go`/`admission_consumer.go`/`settlement.go`/`ports.go` 语义 → 越界，回 Lead 拆范围。
3. 为实现部分金额/整组冲突而引入任何 `ListShipments`/事务外预读 → **停止**（人类 A 明确禁止）。
4. 出现任何 `NO_SHIPMENT`（整组）写 PRE/POST、或 `CONFLICT` 被当准入成功/关闭/打款/释放 → 停止，Reviewer FAIL。
5. 部分金额在 `CANCELLED`/`NO_SHIPMENT` 下写 PRE/POST、或静默当 POST → 停止。
6. 普通 replay 对 CONFLICT 重新探测、或自动 re-arm → 停止。
7. 用迟到 `order.paid` placeholder 当人工证据、或普通 consumer 建行被写成“人工证据” → 停止（W9）。
8. 测试无法区分 fresh/preseed/conflict、或无法证明 Tx1 占位/终态/冲突三分 → 证据不成立，停止。

### 15.3 交付证据（实施时，不属本文件）

- 精确 base/head；`go test`/`-race`/真实 MySQL 日志；`【测试范围】`/`【未覆盖范围】`；受影响 AC 行由 Reviewer 更新；O1/O2/O3/O4b/O4d 的“CONFLICT/PARTIAL、0 终态、0 退款命令、0 释放”断言；F1“无行不建行”；O11b/O11c；O16 re-arm；proto 生成 diff 仅注释。
- **不**声称 AC-VERIFIED/Release/merge；`AC-27` 在 `AC-50-FIX-01`/`AC-27-NOWB-01`/`AC-27-LIVE-01` 关闭前保持 AC-REF。

---

## 16. Consequences 与后续 owner

| 对象 | 动作 |
|---|---|
| **QC（round 3）** | 复核 0.3：R-2a/预读是否彻底删除；CONFLICT 持久化/重放/0 行语义；部分金额 R-2b；W9/F7b；审计查询；per-item stop |
| **人类（Director）** | 批准 L2 ADR（`ADR_APPROVED`），并确认 `result=CONFLICT` + `state` 保持 `PENDING_REMOTE` 的口径；保留 merge/Release |
| **Lead** | 记录人类 A；按 `0001:879` 另拆实施 `change_id`；建 Plan/Tasks + AC 证据矩阵；登记 `LEGACY-SHIPPED-PRE-01`/`TCC-PENDING-RESULT-01`/`TCC-LAST-ERROR-01`/`INV-44-MULTI-01`/`ORDER-PAID-PLACEHOLDER-01`/`CONFLICT-SPEC-SYNC`/T-1 的 owner/expiry/stop；不把本文件当 R0.3-R1c 放行 |
| **Developer** | 在独立 `change_id` 下实现 §15.1；跑 §10；提交 proto 生成物与测试；不 commit 本文件 |
| **Fulfillment owner** | 提供 `NO_SHIPMENT` 与**既有/曾存在但丢失**的 S6 证据；配合 AC-27-NOWB-01 终态①；不得新建行 |
| **ops** | 用 §8 查询/日志发现队列；执行存量 PRE 审计；按 §8.3 授权/审计 re-arm；告警接线另立 L2 |
| **Reviewer** | 按精确 base/head 校验：无预读；CONFLICT/PARTIAL 不写 PRE/POST；0 行语义；replay 不探测；测试真实执行 |
| **Architect** | Rev 2.4 errata 合并本 addendum（需人类批准）；新缺口升 Revision |

---

## 17. 变更记录

| Revision | 日期 | 变更 |
|---|---|---|
| 0.1 | 2026-11-26 | 初稿：R-1…R-8；`NO_SHIPMENT`、映射、不变量、窗口、矩阵、登记面 |
| 0.2 | 2026-11-26 | 按 QC R1 整改：B-1/B-2/B-3；M-1…M-13；R-2a 部分金额预读；W9；setup；T-1…T-7 |
| **0.3** | 2026-11-26 | 按**人类裁决 A** 与 QC R2：**删除 R-2a 与一切 `ListShipments`/事务外预读**；`NO_SHIPMENT`（整组）Tx2 持久化 `result=CONFLICT`（state 保持 `PENDING_REMOTE`），同 `refund_id` 重放返回同一错误且不再探测；commitment 保持 `RESERVED`、占位 outbox 保留、0 打款/0 释放/0 准入终态；R-2b 部分金额无预读（`CANCELLED/NO_SHIPMENT→PARTIAL`，保留 Tx1 占位/人工队列；`SHIPPED→POST`）；W9 按迟到 `order.paid` 真实 placeholder 重写（F7b/O14b）；审计查询补无 group 行/`PENDING_REMOTE`；A8 补 0 行四分支（O11c）；§1.3 per-item stop；人工 re-arm runbook（§8.3）。未写代码、未改 SPEC/Rev 2.3 正文、未 commit |

**Actor**：`arch-ac50`（System Architect）
**Reviewed-by**：待 QC round 3
**Status**：**DRAFT — pending QC round 3**
