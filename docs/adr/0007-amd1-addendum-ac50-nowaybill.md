# ADR-0007 Amendment 1 Addendum — AC-50-FIX-01：`CancelIfCreated` 的“无运单行”additive 结果与 `REVIEW_REQUIRED` 复核裁决

| 项 | 值 |
|---|---|
| Status | **DRAFT 0.4 — pending PO/QC**（未批准；不构成实现依据；不代表 AC-VERIFIED / merge / Release PASS） |
| Revision | **0.4**（按 SPEC-0002 **v1.11 in-review** 的 D-30/D-31 与 AC-69..72 重写：持久裁决值 `REVIEW_REQUIRED`、`state` 不新增值、无迁移；部分金额改为 PO 三分法；TCC 接受合法裁决并 `MANUAL_HOLD`；补齐 AC-72 可观测性；删除 0.3 “`result=REFUND_ADMISSION_CONFLICT` 由人类 A 授权”的越权表述） |
| Change ID | `PAY-SETTLE-01`（子范围 `AC-50-FIX-01`；实施须由 Lead 另拆独立 `change_id`，见 §0.5） |
| Tier | **L2**（跨服务契约语义、资金/履约准入、既有 RPC 响应语义、共享 metrics 包；`docs/collab/CONVENTIONS.md:90` 任一命中即 L2） |
| Tier-Rationale | 修改既有 RPC `Fulfillment.CancelIfCreated` 的响应语义（新增可区分结果值），改变 Order 准入裁决（`REVIEW_REQUIRED` vs `PRE_SHIP_REVOKED`/`POST_SHIP_ALLOWED`），并触及 TCC 批次 `MANUAL_HOLD`、部分金额分类与共享 metrics；直接影响资金打款/释放门禁与 AC-27A 关闭谓词；跨 Fulfillment/Order 两服务。不得降级 |
| Human decision | **2026-11-26 Director 授权 PO 升版 SPEC-0002 v1.11**：为 AC-50 `CONSUMED+无运单` 增加**有界、可持久化的 CONFLICT/PENDING 准入结果及其 TCC 处理**；影响分析（业务+技术）须 QC 审阅后方可实施；不构成 merge/Release/AC-VERIFIED（canonical：`docs/review-tracking.md:366-373`）。**PO 已在 v1.11（in-review）按该授权裁决（D-30/D-31）**：持久值 `REVIEW_REQUIRED`、`state` 不新增值、错误码不变、零元不持久化、部分金额三分法 |
| PO 对齐状态 | **已对齐 SPEC-0002 v1.11（in-review，工作树 2026-11-26）**：S20（`0002:87`）、INV-51/52（`0002:142-143`）、AC-50（`0002:222-245`）、AC-69..72（`0002:446-491`）、§5.10（`0002:652-670`）、D-30/D-31（`0002:834-835`）、§11 对 ADR 的修订要求（`0002:868-872`）、§12 兼容/回滚（`0002:881-890`）。**v1.11 仍 in-review（未 approved）**：本 Addendum 与其同步修订，仍须 QC round 4 + 人类 `ADR_APPROVED`；PO 若再改名/改表示，回 Architect 重算 |
| QC 输入 | `docs/collab/qc-reports/QC-PAY-SETTLE-01-AC50-NOWB.md`：R1（BLOCKED；base `da0cdd1`）+ R2（BLOCKED；head `aec9ac8`）+ **R3（BLOCKED；base/head `47db2e0`；R3-B1..B4、M-1/M-2/M-3、R3-T1..T3，见 `:575-739`）** |
| 技术影响分析 | `docs/adr/0007-amd1-ac50-impact-analysis.md` DRAFT 0.2（schema=0 迁移、TCC、partial 三分法、幂等/回滚、文件级矩阵、剩余 D-2/D-6/D-7/D-8） |
| Spec-Ref | SPEC-0002 **v1.11（in-review）**：AC-50/51/61/62/65/67 复核条款、**AC-69..72**、S20、INV-51/52、§5.10、E32..E35、D-30/D-31、§11/§12；SPEC-0001 **v1.5**：AC-27（`0001:592-613`）、AC-27A（`0001:615-645`）、§16.2（`0001:862-864`）、§16.3（`0001:874-879`）；ADR-0007 **Rev 2.3**：§5.2.1/§5.2.2/§5.2.4（`0007:556-583,591-594`）、§5.3（`0007:611-623`）、§5.7（`0007:706-730`）、D-11/A-06（`0007:97,111`） |
| Base（仓库） | `b3f94f6feabdad9257ee085b464a666afd5e1ab7`（`docs(review): authorize PO SPEC-0002 v1.11 for AC-50 outcome`） |
| Base（契约基线） | SPEC-0001 v1.5（approved 2026-11-26，`4215b3c`）；SPEC-0002 **v1.11 in-review**（工作树，未 commit；行号以 2026-11-26 读取为准，Reviewer 必须绑定 blob）；ADR-0007 **Rev 2.3**（accepted 2026-11-25） |
| Code head | 工作树无代码改动；代码事实=本 base 的 `internal/services/**`、`migrations/order/**`（锚点与 0.3 表 `:15` 的 `c38c54f` 一致）。工作树另有 PO 的 `docs/specs/0002-*.md` 改动（in-review），不属于本文件作者 |
| Doc head | 本文件由 **Lead 统一提交**；QC/Reviewer 复核时必须记录精确 SHA（本文件不自引 SHA） |
| Integration owner | **System Architect**（本 Addendum 作者/契约）；实现 owner = **Developer**（另拆 `change_id`）；S6 无运单证据 owner = **Fulfillment**；人工队列/告警/入口 owner = **ops**；TCC/AC-72 边界 owner = **Architect + ops** |
| allowed_paths（本文件） | 仅 `docs/adr/0007-amd1-addendum-ac50-nowaybill.md`、`docs/adr/0007-amd1-ac50-impact-analysis.md` |
| allowed_paths（本裁决授权的最小实现，须在 Lead 另拆的 `change_id` 下） | `api/proto/mall/v1/mall.proto`（**仅** `CancelIfCreated`/`CancelIfCreatedReply.status` 注释）、`api/gen/mall/v1/mall.pb.go`、`api/gen/mall/v1/mall_grpc.pb.go`（**仅** `make proto` 生成，禁止手改）、`internal/services/fulfillment/cancel_if_created.go`、`internal/services/order/refund_admission.go`、`internal/services/order/tcc_settlement.go`（legal 裁决 / app 单调 / `MANUAL_HOLD` 短路）、`internal/services/order/tcc_admission.go`（`MANUAL_HOLD` 错误与置批 / 读回复核）、`internal/platform/metrics/metrics.go`（2 个 gauge，additive）、`internal/services/order/worker.go`（既有 tick 内只读刷新，**不得新增 worker**）、`deploy/compose/observability/alerts.yml`（AC-72 告警规则；ops 复核）、`internal/services/fulfillment/cancel_if_created_test.go`、`internal/services/fulfillment/admission_consumer_nats_integration_test.go`（**仅测试**）、`internal/services/order/refund_admission_integration_test.go`、`internal/services/order/tcc_admission_integration_test.go`（**仅测试**） |
| forbidden_paths | `migrations/**`（**本 change 无迁移**；若需迁移即停止）、`api/gen/**` 手改、`api/proto` 新字段/新 RPC/枚举变更、`internal/platform/**`（`metrics/metrics.go` 除外）、`internal/services/order/ports.go`（**禁止**新增 `ListShipments`/`GetShipmentByOrder` 调用或接口变更）、`internal/services/order/settlement.go`、`internal/services/order/tcc.go`、`tcc_payment.go`、`tcc_precheck.go`、`tcc_reconcile.go`、`internal/services/fulfillment/consumer.go`、`internal/services/fulfillment/admission_consumer.go`（生产消费者；现有“无行 no-op”语义不得改）、`internal/services/{inventory,payment,aftersales}/**`、`frontend/**`、`gateway/**`、`docs/specs/**`、`docs/collab/**`（含 `ac-coverage.tsv`）、`docs/lead/**`、`docs/implementation/**`、`docs/ops.md`（ops owner；本 ADR 只给查询/工单字段）、`docs/adr/0007-payment-settlement-admission.md`（accepted Rev 2.3 正文）、`docs/adr/0007-payment-settlement-admission-amd1.md`、`scripts/**`、`Makefile` |
| baseline_impact | **涉及**：AC-50 的 `CONSUMED+无运单`（整组）与 `CONSUMED` 部分金额、AC-27A 关闭谓词的 PRE 来源合法性、AC-27 的收口前提；Fulfillment/Order 既有 RPC 响应语义；`order_refund_admissions`/`refund_applications` 新增裁决值 `REVIEW_REQUIRED`（**无 schema 变更**）；TCC 批次 `MANUAL_HOLD`；共享 metrics 新 gauge；`deploy` 一条告警。**不涉及**：Payment 金额/退款实现、事件 subject/payload 结构、permit 状态机、迁移/schema、新 worker；非回归范围见 §14 |
| INV | INV-36、INV-42、INV-43、INV-44、INV-45、INV-50，**新增 INV-51/52**（逐条见 §14）；INV-01/AC-25 不得触碰 |
| Actor | `arch-ac50`（System Architect） |
| Reviewed-by | **待 QC round 4**（本文件不代签；DRAFT 期间不得合入受管代码，不得标 PASS） |
| 硬边界 | 不改 SPEC（SPEC-0001/0002 均归 PO）、不改 accepted ADR Rev 2.3 正文；无新字段/新 RPC/新事件/新状态字段/新 worker/新迁移；**无 `ListShipments`/`GetShipmentByOrder`/事务外预读**；不交付 T2/T3/T4.4；不交付 BFF/UI 入口；本文件未明确列出的变更一律不做 |

> 本文件是**设计裁决**，不是实现、不是验收、不是台账。代码/测试状态归 Developer/Reviewer；AC 状态归 Reviewer；人工队列终态归 SPEC-0001 §16.2。文中的“MUST/不得”是给 Developer 的施工约束，不免除 QC 与人类批准。

---

## 0. 阅读方式与规范边界

### 0.1 问题陈述与代码事实

SPEC-0001 v1.5 `0001:862` 登记：`CONSUMED+无运单` 写路径把“无运单行”映射为 `CANCELLED`/PRE，零元路径 `decideZeroRefundResult` 同样；ADR §5.3 的 `CancelIfCreatedReply` 没有“无运单”结果。不变量：**`PRE_SHIP_REVOKED` 只能由 AC-50 合法未离仓分支写入；`CONSUMED+无运单` 必须返回 `REFUND_ADMISSION_CONFLICT`、不得写 PRE**（`0002:223`），并持久化 `REVIEW_REQUIRED`（PO v1.11 D-30/AC-69）。

| # | 事实 | 证据 |
|---|---|---|
| 1 | Fulfillment 查不到运单行时返回 `CANCELLED` | `internal/services/fulfillment/cancel_if_created.go:50-55`（`:54`） |
| 2 | Order `finishRemoteAdmission` 把 `CANCELLED` 一律映射为 PRE，并在 Tx2 写回 `result=PRE_SHIP_REVOKED` | `internal/services/order/refund_admission.go:566-608`（`:576-584`、`:590-597`） |
| 3 | 零元 `decideZeroRefundResult` 同样把 `CANCELLED` 映射为 PRE | `refund_admission.go:366-396`（`:384-391`） |
| 4 | **正金额 `CONSUMED` 的 Tx1 在远程探测前已写**：admission（`result=POST,state=PENDING_REMOTE`）、`order_refund_commitments(RESERVED)`、outbox `order.refund_admitted` | `refund_admission.go:253-283`（admission `:254-265`、commitment `:266-272`、outbox `:281`、payload `:610-619`） |
| 5 | **Tx1 前已调用 `Payment.GetIntent`**（只读快照）；existing 读取在 `:179-185`（**在 GetIntent 之后**，须前移） | `refund_admission.go:168-175`、`:179-185` |
| 6 | TCC Tx1 的 admission 写 `result=PENDING_REMOTE,state=PENDING_REMOTE`（既有权宜值） | `tcc_settlement.go:844-852` |
| 7 | 同 `refund_id` 重放且 `state=PENDING_REMOTE` 时会重新进入 `finishRemoteAdmission` | `refund_admission.go:179-185`、`:203-211` |
| 8 | AC-27A 第 5/6 条：`CONSUMED` 只有 `result=PRE_SHIP_REVOKED` 才关闭；`PENDING_REMOTE`/`REVIEW_REQUIRED`、结果缺失/不可核验 → 未关闭并告警 | SPEC-0001 v1.5 `0001:622-625`、注 `0001:645`；SPEC-0002 v1.11 AC-69 `0002:450` |
| 9 | AC-50 v1.11：`NONE→REVOKED` 无运单合法 PRE；`CONSUMED+CANCELLED` 合法 PRE；`CONSUMED+无运单`（整组）→ `REFUND_ADMISSION_CONFLICT` + `REVIEW_REQUIRED`；部分金额三分法 | SPEC-0002 `0002:222-245` |
| 10 | `order_refund_admissions.result/state` 为 `VARCHAR(32)`，无 CHECK，默认 `PENDING_REMOTE` | `migrations/order/0016_settlement_admission.sql:7-19` |
| 11 | `refund_applications.admission_result` 为 `VARCHAR(24)`；`REVIEW_REQUIRED`(15) 可容纳，`REFUND_ADMISSION_CONFLICT`(25) **不可** | `migrations/order/0017_refund_batch_ledger.sql:45-81`（`:56`）；D-30 `0002:834` |
| 12 | 每组至多一条运单行：`UNIQUE KEY (order_no, fulfillment_group_no)` | `migrations/fulfillment/0003_group_shipments.sql:6-9` |
| 13 | TCC 重试在 `admitTccBatch` 之前先经 `ensureTccRemoteAdmissionTx`/`ensureTccApplicationTx`；前者的 switch 只接受 `PENDING/PRE/POST` | `tcc.go:346-361`；`tcc_settlement.go:717,788-876` |
| 14 | Tx2 更新 admission **无 `state`/version 守卫** | `refund_admission.go:590-594` |
| 15 | Order 组投影由 `shipment.shipped/received` 推进且不降级；ADR §5.2.1 已把组状态列为离仓事实 | `order/consumer.go:59-62,89-101`；`0007:556`；PO v1.11 `0002:98` |
| 16 | 除 Order 与 Fulfillment 自身的 `order.refund_admitted` 消费者外，没有其他 `CancelIfCreated` 调用方 | 全仓库 grep；`admission_consumer.go:43` |
| 17 | `refund_batches.state` CHECK 已含 `MANUAL_HOLD`；`last_error` 列存在 | `0017:20-33`（`:28`） |
| 18 | 既有 metrics 包与 Order worker tick 可挂只读 gauge 刷新 | `internal/platform/metrics/metrics.go:1-110`；`order/worker.go:11-46`；`aftersales/workers.go:818-834` |

**结论**：缺陷在“事实无法区分”：Fulfillment 把“无行”与“真实已取消行”压成同一个 `CANCELLED`，Order 又把它压成同一个 PRE。本 amendment 采用 QC B-2 的第一种路径（新增可区分结果），并按 PO v1.11 持久化 `REVIEW_REQUIRED` 复核裁决。

### 0.2 `CONSUMED+无行` 的可达性（R2-M12 保持）

| 路径 | 是否可能产生 `CONSUMED+无行` | 证据 |
|---|---|---|
| 正常 `Ship` 路径 | **否**：`CreateShipment` 先插入运单行（`CREATED`），`Ship` 才调 `Order.AuthorizeShipment` 消费 permit | `fulfillment/service.go:154-166`、`:244`；`settlement.go:353-357` |
| 直接调用 `Order.AuthorizeShipment`（Order RPC） | **是**：Order 不校验运单行是否存在 | `api/proto/mall/v1/mall.proto:485-487` |
| 历史数据（0016/0003 之前、无 permit 行、行缺失） | **可能** | `migrations/order/0016_settlement_admission.sql`、`migrations/fulfillment/0003_group_shipments.sql` |
| 人工/数据修复、行被删除 | **可能** | ops；`[不确定]` 生产发生率需 ops 证据 |
| 部分金额 + `CONSUMED` | 不进入“整组无运单”裁决；按 §3.4 三分法 | §3.4 |

> 该状态是**合成异常状态**，用于验证 fail-closed；不据此声称“已发生的 P0 缺陷”或频率。正常路径回归见 F8/O4。

### 0.3 指针澄清（brief 与 canonical 文件不一致处）

任务 brief 指向 `docs/adr/0007-payment-settlement-admission-amd1.md` 的 §5.2/§5.3/§5.7。该文件是 Rev 2.3 的**来源文件**，但其 §5.3（`:375-397`）是 `legacy_payment_resolution` 的并发与事务规则；`Fulfillment.CancelIfCreated` 的规范正文在**合并后的 accepted Rev 2.3** 文件 `docs/adr/0007-payment-settlement-admission.md` §5.3（`:611-623`，合并映射见 `:56`）。本 amendment 的修改对象是**后者**；文件名沿用 Lead 指定的 `0007-amd1-addendum-ac50-nowaybill.md`。

### 0.4 与已批准契约的关系

- 本文件**additive**：不删除、不重写 Rev 2.3 任何条款；只给 §5.3 增加一个 `status` 值及其语义，并明确 §5.2.2/§5.2.4 的映射与 Tx1 已写事实的处置。
- 本文件**不批准**任何实现；`ADR_APPROVED`（人类）+ SPEC-0002 v1.11 approved 之前 Developer 不得开工（宪法 Workflow 2）。
- 不改变事实所有权、permit 状态机、退款额度、INV-45 的不可逆性；**不新增 `ListShipments`/`GetShipmentByOrder`/事务外预读**（人类 route A 保持）。
- **0.4 撤回 0.3 的两个越权点**：①0.3 把 `result=REFUND_ADMISSION_CONFLICT` 写成“人类 A 授权”，超出 canonical 记录（R3-B3）；②0.3 的 25 字符 token 放不进 `refund_applications.admission_result VARCHAR(24)`。PO v1.11 D-30 已按授权裁决 `REVIEW_REQUIRED`（15 字符、`state` 不进新值、错误码不变、零元不持久化），本文件按此对齐。

### 0.5 `change_id` 边界（防越权）

SPEC-0001 v1.5 §16.3 `0001:879` 明确：AC-50 修正“需 Lead 拆独立 `change_id` 走 L2（跨 Fulfillment/Order，超 R0.3-R1c `allowed_paths`）”。因此：

- 本 Addendum（契约）挂 `PAY-SETTLE-01`（Lead 指定）；
- **实施**必须由 Lead 另拆独立 `change_id`，并在其实施 Brief 中逐文件落 `allowed_paths`；不得把本文件的 `allowed_paths` 当作 R0.3-R1c 的范围扩展，也不得在同一提交里顺手改 AC-27A 读模型（那是 R0.3-R1c 的独立实施项）。

### 0.6 与 PO v1.11 的对齐表

| 语义 | PO v1.11（in-review） | 本 ADR |
|---|---|---|
| 持久裁决值 | `REVIEW_REQUIRED`（D-30） | §3.1/§3.5 使用同值；**不是错误码**，对外仍 `REFUND_ADMISSION_CONFLICT` |
| Order 记录 | `result=REVIEW_REQUIRED`、`state=PENDING_REMOTE`（不新增 state 值） | 同 |
| 发起方记录 | `refund_applications.admission_result=REVIEW_REQUIRED`、申请 `PENDING`（TCC 批次 `MANUAL_HOLD`） | 同；普通售后无该行（§12.3） |
| 部分金额 | D-31 三分法（离仓/未离仓/不可判定） | §3.4 |
| 零元 | 不持久化 `REVIEW_REQUIRED`，重放重探 | §4 |
| TCC | 接受合法裁决、`MANUAL_HOLD`、`TCC_BATCH_REQUIRES_MANUAL_HANDLING` | §3.6/§7.2 |
| 可观测性 | 5 分钟告警、24h/7d SLA、计数/最老时长 | §8/§9 |
| 迁移 | 无（D-30） | §9.2 |
| 回滚 | 回滚前审计是否存在 `REVIEW_REQUIRED` 行 | §7.6 |

---

## 1. 裁决摘要

| # | 裁决 | 一句话 | 归属 |
|---|---|---|---|
| **R-1** | `CancelIfCreatedReply.status` 增加 **additive** 值 `<NO_WAYBILL>`（string，无新字段/无新 RPC；0.3 曾名 `NO_SHIPMENT`，**PO 定名**）；Fulfillment 对“该组没有任何运单行”返回它；`CANCELLED` 只表示真实运单已取消 | 先分开事实 | Architect（契约）/ Fulfillment（实现） |
| **R-2** | **整组金额** + `<NO_WAYBILL>`：Tx2 持久化 `result=REVIEW_REQUIRED,state=PENDING_REMOTE`（**不新增 state 值**）；Error 告警、commitment 保持 `RESERVED`、outbox 占位保留、0 打款/0 释放/0 准入终态；进入 `AC-27-NOWB-01` 人工队列。同 `refund_id` 重放返回**同一 `REFUND_ADMISSION_CONFLICT`**，不探测/不读 Payment/不新增写入 | 有界持久化 + 人工队列 | Order（实现） |
| **R-2b** | **部分金额**（`amount < 组剩余`）：**一律不调用 `CancelIfCreated`**（不改运单、无预读）。只用 Order 自有事实（S3 组投影/S5/S9）三分：投影 `SHIPPED/RECEIVED` → `POST_SHIP_ALLOWED`；投影未离仓 → `PARTIAL_REFUND_REQUIRES_FULL_GROUP`（**写前拒绝**）；不可判定（投影缺失且 `orders.status ∈ {SHIPPED,COMPLETED}` 等）→ `REVIEW_REQUIRED` + 人工队列 | 不丢 PARTIAL、不改运单、不静默 POST | Order（实现） |
| **R-3** | 零元 `decideZeroRefundResult`：`<NO_WAYBILL>` → `REFUND_ADMISSION_CONFLICT` + 告警，在写任何 admission/`DONE` 之前返回；`CANCELLED` → PRE；`ALREADY_SHIPPED/RECEIVED` → POST；瞬态/未知 → 可重试。零元无 Tx1，**不持久化** `REVIEW_REQUIRED`，重放重新探测（AC-67/AC-69） | 零元不得例外打款/造行 | Order（实现） |
| **R-4** | 正金额/TCC/legacy/零元共用**唯一映射表**；TCC 经 `AdmitRefund`→`finishRemoteAdmission`，禁止在 `tcc_*.go` 复制第二套状态判断；`REVIEW_REQUIRED` 只从该表产生 | 拒绝 if-else 扩散 | Order（实现）/ Reviewer |
| **R-5** | 持久化语义：准入成功终态当且仅当 `state=result` 且 `state ∈ {PRE,POST}`；`PENDING_REMOTE` 与 `REVIEW_REQUIRED` 都不是结果；`REVIEW_REQUIRED` 是“自动路径终止、业务未终止”的非终态标记；Tx1 写入按 §7.4 处置，**不声称“无写入”** | 单一事实载体 + 有界语义 | Order（实现）/ Reviewer |
| **R-6** | 幂等/并发/崩溃：同 `refund_id` 重放读已持久化裁决（PRE/POST/REVIEW）并返回同一结果/错误；`PENDING_REMOTE`（无复核）重新探测；Tx2 以 `state=PENDING_REMOTE` CAS 唯一化；`AdmitRefund` 对任何非终态 existing **必须返回错误**，不得回 success reply | 可重放、无脏写、无提前打款 | Order/Fulfillment |
| **R-7** | `REVIEW_REQUIRED`（整组/部分不可判定/legacy 不可判定）进入 **AC-27-NOWB-01** 人工队列（终态/owner/expiry/SLA 见 SPEC-0001 §16.2 与 AC-72），组保持未关闭；不得自动关闭/自动退款/恢复 permit；TCC 批次 `MANUAL_HOLD` | 人工终态 | ops + Developer + Fulfillment |
| **R-8** | 登记：`mall.proto` 注释必须更新（生成物同步）；`docs/contracts.md` **不改**；错误码不新增；`REVIEW_REQUIRED` 为持久裁决值（D-30），`REFUND_ADMISSION_CONFLICT` 为对外错误码；**无迁移**；AC-72 需 metrics/告警（见 R-10） | 最小登记面 | Architect / Developer / PO / Reviewer |
| **R-9** | TCC：把 `REVIEW_REQUIRED` 当**合法持久裁决**读回（`ensureTccRemoteAdmissionTx`/`ensureTccApplicationTx`），批次置 `MANUAL_HOLD`、application `PENDING`+`admission_result=REVIEW_REQUIRED`、`refund_batches.last_error` 记录原因；自动续跑返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`；人工 re-arm 后同 `batch_id`/`refund_id` 续跑 | 不早失败、不阻塞其他批 | Order（实现）/ Architect |
| **R-10** | AC-72 可观测性：新增 `mall_refund_review_backlog`/`mall_refund_review_oldest_seconds`（共享 metrics，additive）+ Order 既有 tick 只读刷新 + 1 条告警规则；**不新增 worker**；用户可见文案/人工入口不在本 change（另立 L2） | 5 分钟告警 + SLA 可见 | Architect / Developer / ops |

**结论级别**：R-1…R-10 均在**不改 SPEC（PO 已改 v1.11）、不改 Rev 2.3 正文、无新字段/RPC/事件/新状态值/新 worker、无迁移、无 `ListShipments`/预读**的前提下可落地。无 `[ARCHITECTURAL_FLAW]`。仍登记 DEFERRED 缺口（M-1 legacy、M-3 人工释放实现、M-4 TCC result 清理、M-5 存量审计执行、T-3 多组释放、BFF/入口），见 §1.3，owner/expiry 均为 2026-12-31。

### 1.1 QC R3 处置索引（R3-B1..B4、R3-M1/M2、R3-T1..T3）

> 下表是**作者处置**，不是 QC 判定；“CLOSED”仅表示作者已给出可审阅的闭合方案，最终状态由 QC round 4 给出。凡依赖 PO v1.11 approved 或人类批准的项，在对应前置完成前仍属**条件闭合**。

| QC 项 | 0.3 问题 | 0.4 处置 | 判定 |
|---|---|---|---|
| **R3-B1** TCC 早失败 | `result=CONFLICT` 命中 `ensureTccRemoteAdmissionTx` 的 `default` → “非法 result”；生产 `tcc_*.go` 被禁改 | PO D-30 改名 `REVIEW_REQUIRED`；R-9 要求 switch/app 单调接受合法裁决；PO §11 明确 `tcc_*.go` 纳入 `allowed_paths`；批次 `MANUAL_HOLD` + 稳定错误 | **条件 CLOSED**（PO v1.11 approved + QC） |
| **R3-B2** 部分金额先取消运单 | `CONSUMED+部分+CANCELLED` 在返回 PARTIAL 前已取消 `CREATED`，违 AC-50 `0002:217` | PO D-31 三分法：**不探测**，只用 Order 自有事实；fresh 未离仓在写前拒绝（零写入）；已存在 PENDING 部分按 AC-70 保持既有承诺并返回 PARTIAL；不可判定→`REVIEW_REQUIRED`+冻结额度 | **条件 CLOSED** |
| **R3-B3** canonical 未授权 `result=CONFLICT` 持久化 | 0.3 把具体表示写成人类 A 授权，超出 `review-tracking` 记录 | PO v1.11 D-30 在 Director 授权范围内裁决；本 ADR 撤回 0.3 表述并按 D-30 对齐；SPEC-0001 注按 D-3 同步 | **条件 CLOSED**（等 v1.11 approved） |
| **R3-B4** “可重试” vs “不再探测” | §8 写“保持可重试”，§3.5 写“不再探测”，自相矛盾 | 采用 PO §5.10 分界表（`0002:652-670`）：REVIEW 自动重试只返回同一冲突、不收敛；人工 re-arm 或 AC-62 后新 `refund_id` 才继续 | **CLOSED（协议）** |
| **R3-M1** 迟到 placeholder 取消 owner | W9 写“由 Fulfillment/ops 取消”但无 owner/runbook | §7.2 W9 明确：正常 consumer 创建的真实 placeholder 由 Fulfillment 取消/ops 审计；人工侧仍禁止造行；`ORDER-PAID-PLACEHOLDER-01` | **CLOSED（协议）** |
| **R3-M2** W9 partial 断言不准 | “重探可取消→PRE”未区分整组/部分 | W9 改为“整组→PRE；部分→§3.4（不探测）” | **CLOSED** |
| **R3-T1** §8 与 §3.5 矛盾 | 同 R3-B4 | 已按 PO §5.10 口径改写 | CLOSED |
| **R3-T2** 非法组合未定义 | `state=PRE/POST` + `result=REVIEW_REQUIRED` 等 | A8 补：`REVIEW_REQUIRED` 只允许与 `state=PENDING_REMOTE` 组合；其他组合 fail-closed | CLOSED |
| **R3-T3** re-arm 过渡表 | re-arm 后组合未成表 | §8.3 给出“唯一合法过渡”表 | CLOSED |

### 1.2 13 MAJOR + R2/R3 新增：逐条状态

| QC 项 | 状态 | 说明 | owner / expiry / stop |
|---|---|---|---|
| **M-1** legacy `NONE`+已发货 PRE | **PARTIAL（可接受 DEFERRED）** | A3 收窄；审计补齐无 group 行；`LEGACY-SHIPPED-PRE-01`；AC-70 已给 legacy 三分法 | Architect/PO；2026-12-31；审计发现 PRE+已发货/无 S6 证据 → AC-27A 第 4/5 条不得 AC-VERIFIED |
| **M-2** 部分金额 | **CLOSED（规则）** | R-2b 三分法；未离仓写前拒绝；不可判定→REVIEW；无预读 | Developer；2026-12-31；任何 partial 调 `CancelIfCreated`/写 PRE/静默 POST → FAIL |
| **M-3** A4/AC-62 | **CLOSED 规则 / DEFERRED 实现** | A4 自动路径 + AC-62 唯一例外；O15 | Developer/T3；2026-12-31；release writer 未实现 → 不得 fixture 冒充 |
| **M-4** TCC `result=PENDING` | **CLOSED 读者 / DEFERRED 清理** | A2 双条件；O13b | Architect/Developer；2026-12-31；只读 result 的消费方 → FAIL |
| **M-5** 存量 PRE 审计 | **CLOSED 协议 / DEFERRED 执行** | §8 查询 | ops/Developer；2026-12-31；审计未完成 → AC-27A 不得关闭 |
| **M-6** TCC 测试路径 | **CLOSED** | allowed `tcc_*_test.go` + 指定生产 `tcc_*.go` | Developer；N/A |
| **M-7** TCC 队列发现 | **CLOSED（本路径写 last_error）/ DEFERRED 通用** | R-9 要求 `refund_batches.last_error='REVIEW_REQUIRED:<reason>'`；`TCC-LAST-ERROR-01` 剩余部分保持 DEFERRED | Developer/Architect；2026-12-31 |
| **M-8** 重复/乱序 | **CLOSED** | W9；F7/O14 + F7b/O14b | Fulfillment consumer（只测试）；N/A |
| **M-9** Tx2 守卫 | **CLOSED 规则** | A8 `state` CAS + 0 行语义；O11b/O11c | Developer；N/A；0 行脏写 → FAIL |
| **M-10** POST commitment | **CLOSED** | §3.1 分行；O5/O5b | Developer；N/A |
| **M-11** consumer `<NO_WAYBILL>` | **CLOSED** | §2.2 no-op success；F7/O14 | Fulfillment；N/A |
| **M-12** 可达性 | **CLOSED（说明）** | §0.2/F8；ops 证据 | Lead/ops；2026-12-31 |
| **M-13** setup | **CLOSED** | §10 setup 列 | Developer；N/A |
| **R2-B1/B2** | CLOSED | 无 `ListShipments`；existing 前移；部分金额用 Order 投影 | Developer；N/A |
| **R2-M1** late `order.paid` | **CLOSED（协议）/DEFERRED** | W9 真实行为；`ORDER-PAID-PLACEHOLDER-01` | Architect/Developer；2026-12-31 |
| **R2-M2/M3/M4/M5** | CLOSED | §8 审计；§1.3；无预读；A8 | — |
| **R3-B1..B4 / R3-M1/M2 / R3-T1..T3** | 条件 CLOSED | §1.1 | 见各行 PO/Developer 依赖 |

### 1.3 DEFERRED / 人工项的 per-item 停止条件

| 项 | owner | expiry | stop / exit |
|---|---|---|---|
| `LEGACY-SHIPPED-PRE-01`（M-1） | Architect/PO | 2026-12-31 | 审计发现“PRE + 已发货/无 S6 证据/无 group 行” → AC-27A 第 4/5 条不得标已关闭/AC-VERIFIED；冻结收口；修复走独立 L2 或 PO 裁决 |
| AC-62 人工释放（M-3/O15） | Developer/T3 | 2026-12-31 | release writer 未实现 → 不得用 fixture/未授权释放冒充；复核组保持未关闭；T3 关闭前不得声称 AC-62 可执行 |
| `TCC-PENDING-RESULT-01`（M-4） | Architect/Developer | 2026-12-31 | 清理前任何只读 `result`/只比 `state==result` 的消费方 → FAIL；AC-27A 必须 A2 双条件 |
| 存量 PRE 审计（M-5） | ops/Developer | 2026-12-31 | 审计未执行或存在不可核验 PRE → AC-27A 不得关闭/AC-VERIFIED；0 行不自动等于安全 |
| `TCC-LAST-ERROR-01`（M-7 剩余） | Developer/Architect | 2026-12-31 | `REVIEW_REQUIRED` 路径外仍无 `last_error` → ops 不得声称可发现；转人工升级 |
| `INV-44-MULTI-01`（T-3） | Developer/Lead | 2026-12-31 | 多组 release 未实现 → AC-27 EARS-3/INV-44 不得标 VERIFIED；不得 fixture 冒充 |
| SPEC 同步 `AC50-OUTCOME-SPEC-SYNC`（R3-B3/T-1） | PO | 2026-12-31 | SPEC-0001 `0001:645` 注未同步（`REVIEW_REQUIRED` 非结果）→ 以 PO v1.11 §11 item 4 的“更严格且更晚批准者为准”处理；不得据旧注否定本裁决 |
| `ORDER-PAID-PLACEHOLDER-01`（R2-M1） | Architect/Developer | 2026-12-31 | 业务要求禁止正常 consumer 迟到 placeholder → 独立 L2；在此之前 W9 真实行为为准 |
| 人工 re-arm runbook（§8.3） | ops | 2026-12-31 | runbook/授权/审计未就绪 → `AC-27-NOWB-01` 终态①不得执行；不得用普通 replay 冒充；组保持未关闭 |
| `REVIEW-REQUIRED-ENTRY-01`（AC-72 用户可见/人工入口） | Lead/BFF/ops | 2026-12-31 | BFF/UI/入口未实现 → AC-72 的“用户可见/入口”不得 AC-VERIFIED；后端裁决/告警不受影响 |
| `ADMISSION-RESULT-CHECK-01`（§9.2） | Architect/Developer | 2026-12-31 | 未加 CHECK 不得声称“DB 已兜底枚举”；加 CHECK 必须预检+维护窗 |
| `REVIEW-AUDIT-01`（原复核记录载体） | Architect/PO | 2026-12-31 | PO 要求行级历史/独立审计表 → 另立 L2；在此之前以 ops 工单 + 日志 + `last_error` 为审计依据，不得声称“行级历史已保留” |

---

## 2. R-1：`<NO_WAYBILL>` 的契约语义（Fulfillment 侧）

### 2.1 契约文本（additive，仅 string 值；名称由 PO 定）

以下 `<NO_WAYBILL>` 为占位名（0.3 用 `NO_SHIPMENT`）。PO v1.11 未对该 Fulfillment 侧值命名（D-30 只定 Order 持久裁决），因此实施 Brief 必须由 Lead/PO 确认；**改名不改语义/矩阵**，但需同步 proto/测试/日志 key。

```text
message CancelIfCreatedReply {
  // CANCELLED | <NO_WAYBILL> | ALREADY_SHIPPED | RECEIVED | TRANSIENT_ERROR
  // CANCELLED       = a real shipment row was CREATED->CANCELLED, or was already CANCELLED.
  // <NO_WAYBILL>    = this (order_no, group_no[, shipment_no]) has no shipment row at all;
  //                   nothing was cancelled and nothing was rolled back.
  // ALREADY_SHIPPED = the parcel left the warehouse (SHIPPED, or EXCEPTION with shipped_at);
  //                   never cancelled.
  // RECEIVED        = the parcel was received; never cancelled.
  // TRANSIENT_ERROR = ambiguous/unknown (incl. EXCEPTION without shipped_at); retry/manual,
  //                   never treated as cancelled.
  string status = 1;
}
```

- **无新字段、无新 RPC、无枚举类型变更**：`status` 是 `string`，这是既有字符串契约上的新增取值（`mall.proto:1089-1098`）。
- 语义是**“该探测范围内没有任何运单行”**，不是“未发货”、不是“已取消”、不是“可以 PRE”。它不携带 permit/准入结论；permit 事实仍归 Order（S5）。
- 探测范围：`order_no` 必填；`group_no` 为空时既有实现默认 `G1`（不改）；`shipment_no` 非空时范围收窄到 `(order_no, group_no, shipment_no)`。Order 调用点必须继续传空 `shipment_no`（现状），使 `<NO_WAYBILL>` 表示**整组无行**；T-5 见 §10 F1b。
- `migrations/fulfillment/0003_group_shipments.sql:6-9` 的唯一键保证每组至多一行，故 `<NO_WAYBILL>` 无“多行歧义”。

### 2.2 返回矩阵（Fulfillment + `order.refund_admitted` consumer）

| 数据库事实 / 输入 | Fulfillment 返回 | consumer（仅 `result=PRE`）行为 | 说明 |
|---|---|---|---|
| 无行 | **`<NO_WAYBILL>`** | **幂等成功/no-op**（无行可取消，不报错） | 新增；当前错误返回 `CANCELLED` |
| `CREATED` | `CANCELLED` | 取消幂等；重复 PRE 不重复写 | 不变 |
| `CANCELLED` | `CANCELLED` | 幂等 | 不变 |
| `SHIPPED` | `ALREADY_SHIPPED` | Error 告警（“pre-ship 但已离仓”），不回滚 | 不变 |
| `RECEIVED` | `RECEIVED` | 同 `ALREADY_SHIPPED` 告警 | 不变 |
| `EXCEPTION` 且 `shipped_at>0` | `ALREADY_SHIPPED` | 告警 | 不变（T1.2/R-2） |
| `EXCEPTION` 无 `shipped_at` | `TRANSIENT_ERROR` | 重试；不取消 | 不变 |
| 其他未知状态 | `TRANSIENT_ERROR` | 重试；不取消 | 不变 |

- consumer 判据（现状 `internal/services/fulfillment/admission_consumer.go:40-55`）：只处理 `result=PRE_SHIP_REVOKED`；`<NO_WAYBILL>` 是 no-op success，**不是** `TRANSIENT_ERROR`，不得据此报错或重试；重复/乱序见 W9（§7.2）与 F7/O14（§10）。本 amendment **不改**该生产文件。
- 部分金额**不再调用** `CancelIfCreated`（§3.4），因此 `<NO_WAYBILL>` 只在整组金额路径出现。

### 2.3 明确禁止

1. **ops/人工**不得为“无行”补建 `CANCELLED`/`CREATED` 运单作为证据（B-3）；正常 `order.paid` consumer 迟到创建的真实 placeholder **不是**人工证据（W9/§12），`REVIEW_REQUIRED` 后不得据此自动转 PRE。
2. 不得因 `<NO_WAYBILL>` 由 Fulfillment 写 permit、admission、commitment、库存或任何 outbox 事件（Fulfillment 不是这些事实的 owner）。
3. 不得把 `<NO_WAYBILL>` 复用为 `TRANSIENT_ERROR` 或 `CANCELLED` 的别名；失败语义必须可区分。
4. 不得新增 `warehouse_id` 缺省、`G1` 缺省或新的请求字段；`group_no` 的既有 `G1` 缺省行为保留，但调用方必须显式传组号（现状）。

### 2.4 兼容与发布顺序

- `<NO_WAYBILL>` 是**新增值**。旧 Order 客户端对未知值走 `default` → `apperr.Unavailable`（可重试，见 `refund_admission.go:580-584`、`:389-391`），**不会**写 PRE。
- 若 Fulfillment 先上线：旧 Order 只会重试，状态停在 `PENDING_REMOTE`，fail-closed，可接受（告警噪声，不资损、不误关闭）。
- 若 Order 先上线：在 Fulfillment 仍返回 `CANCELLED` 期间，缺陷保持现状（风险不变，不得宣称修复完成）。
- 按 `CONVENTIONS.md:241`，**建议同一原子行为提交**同时改 Fulfillment 与 Order；若必须分批，按上一条 fail-closed 顺序，并在实施 Brief 记录批次与回滚（回滚见 §7.6）。

---

## 3. R-2 / R-2b：Order 映射（整组 / 部分 / TCC / legacy）

### 3.1 唯一映射表（`finishRemoteAdmission`，整组金额）

| `CancelIfCreated` 返回 | Order 准入裁决 | 持久化动作（**含 Tx1 已写事实**） | 退款/释放 |
|---|---|---|---|
| `CANCELLED` | `PRE_SHIP_REVOKED` | Tx2（`state` CAS）写 `result=PRE,state=PRE` + outbox；**正金额/TCC 的 Tx1 已写恰好 1 条 `RESERVED` commitment**（正金额 `:266-272`；TCC `tcc_settlement.go:898`），Tx2 **不重复写**；release 按 INV-44（单组）。Order 准入层不写 Payment；本层只写 admission/outbox/release | 允许（AC-50 合法 PRE） |
| **`<NO_WAYBILL>`（整组）** | **`REFUND_ADMISSION_CONFLICT` + `REVIEW_REQUIRED`** | **Tx2 写 `result=REVIEW_REQUIRED,state=PENDING_REMOTE`**（`version+1`；CAS `state='PENDING_REMOTE'`）+ Error 告警；**不开 release、不写终态 outbox**；Tx1 的 **commitment 与占位 outbox 保留**；`source=tcc` 且 `batch_id` 非空时同事务置 `refund_batches.MANUAL_HOLD` + `last_error`，并把 `refund_applications.admission_result` 置 `REVIEW_REQUIRED`（state 保持 `PENDING`）；处置见 §7.4 | **禁止** `Payment.Refund/RefundBatch`、禁止 release/commitment release；`Payment.GetIntent` 只读允许（重放不读） |
| `ALREADY_SHIPPED` / `RECEIVED` | `POST_SHIP_ALLOWED` | Tx2（`state` CAS）写 `result=POST,state=POST` + outbox；**正金额路径的 Tx1 已有 1 条 `RESERVED` commitment，Tx2 不重复写**；**TCC 已离仓组**由 `recordTccPostAdmission` 在同一事务写 POST admission + commitment（既有，`tcc_admission.go:214`） | 允许（已离仓） |
| **（整组）Order 组投影已是 `SHIPPED/RECEIVED`** | `POST_SHIP_ALLOWED` | **不调用 `CancelIfCreated`**（PO AC-50 `0002:236`）；Tx1 直接写 `result=POST,state=POST` + outbox + commitment（或 Tx2 CAS POST）；**不得再探测/取消** | 允许（已离仓） |
| `TRANSIENT_ERROR` | **不可判定（未决）** | 不动；保持 `PENDING_REMOTE` | 禁止；可重试（`apperr.Unavailable`）；**不得写 `REVIEW_REQUIRED`**（AC-70 `0002:469`） |
| 空值 / 未知值 | **不可判定（未决）** | 不动；保持 `PENDING_REMOTE` | 禁止；可重试（`apperr.Unavailable`） |

- `<NO_WAYBILL>` 的判定与返回必须在**打开 Tx2 之前**完成；不得先写 PRE 再回滚“修正”。
- 错误对象：`apperr.Failed(...)`（→ gRPC `FAILED_PRECONDITION`），message 含 `REFUND_ADMISSION_CONFLICT` 与 `CONSUMED permit without shipment`；`WithDetails` 固定 key（大小写固定）：`refund_id/order_no/group_no/permit_state="CONSUMED"/probe_status="<NO_WAYBILL>"/result="REVIEW_REQUIRED"/state="PENDING_REMOTE"/reason="CONSUMED_WITHOUT_SHIPMENT"/manual_queue="AC-27-NOWB-01"`（TCC 另加 `batch_id`）。不新增错误码、不改错误码表。
- **Payment 口径（B-2）**：本 amendment 只禁止 **`Payment.Refund`/`RefundBatch`**（退款命令/资金写）；`Payment.GetIntent` 是只读快照，**允许且必要**；`REVIEW_REQUIRED` 重放时**不读 Payment**（existing 先行）。
- **Tx1 占位**：普通正金额 `CONSUMED` 的 Tx1 写 `result=POST,state=PENDING_REMOTE` 占位；TCC `CONSUMED` 的 Tx1 写 `result=state=PENDING_REMOTE`。正常 `CANCELLED→PRE`/`SHIPPED→POST` 时 Tx2 覆盖为终态；`<NO_WAYBILL>` 时 Tx2 把 `result` 覆盖为 `REVIEW_REQUIRED`、`state` 保持 `PENDING_REMOTE`。**不新增字段、不新增 `state` 值**。
- **`NONE→REVOKED` CAS 败者的“其他”分支**（PO AC-50 `0002:237`）：重读后若既非 `SHIPPED/RECEIVED`、也非 `CREATED/CANCELLED`（许可/运单/组投影不可判定），必须写 `REVIEW_REQUIRED` + `reason=PROBE_UNCLASSIFIABLE`（新增稳定 reason，追加在 PO AC-72 的三个值之后；实施 Brief 固定，不得改名），**不得**降级为 `Unavailable` 无限重试，也不得默认 PRE/POST。

### 3.2 为什么 `<NO_WAYBILL>` 不与 AC-50 的 `NONE→REVOKED` 冲突

- AC-50 `0002:219`：许可 `NONE` 且运单为**无**/`CREATED`/`CANCELLED` → 置 `REVOKED` → `PRE_SHIP_REVOKED`（无运单时幂等成功）。
- 因此“无运单”本身**不是**异常；异常是 **`permit=CONSUMED` 且无运单、且本次为整组金额**（`0002:223`/AC-70）。
- 本 amendment 只改**远程探测调用点**的映射：正金额路径只在 `PENDING_REMOTE`（由 `CONSUMED` 分支写入）时调 `finishRemoteAdmission`；零元路径只在 `permitConsumed` 时调用 `CancelIfCreated`。`NONE/REVOKED` 分支**不得**调用探测，也不得因为本修复被改成“无运单 → `REVIEW_REQUIRED`”。
- 任何把“无运单一律 `REVIEW_REQUIRED`”的实现都是**误读**，会破坏 AC-50 `NONE` 分支与 legacy G1 初始化，Reviewer 应直接 FAIL。

### 3.3 legacy / TCC 的归属（不新增映射点）

| 来源 | 现有路径 | 本裁决要求 |
|---|---|---|
| 普通售后（`source=aftersale`） | AfterSales worker → `Order.AdmitRefund` → `finishRemoteAdmission` | 同一映射，无例外；Order 裁决落 `order_refund_admissions`（普通售后当前无 `refund_applications` 行，§12.3） |
| SYSTEM 缺货（`source=system_shortage`） | 同上 | 同上 |
| TCC 补偿（`source=tcc`） | `tcc_admission.go:admitTccApplication` → `Order.AdmitRefund` → `finishRemoteAdmission` | 同一映射；`admission_result=REVIEW_REQUIRED`、批次 `MANUAL_HOLD`；**禁止**在 `tcc_*.go` 再写一份 `CANCELLED→PRE`；Tx1 `result=PENDING_REMOTE` 是既有偏差（§6.2） |
| legacy（历史单/无许可行 G1） | `ensurePermitTx` 初始化后走同一 `AdmitRefund` | AC-70 `0002:468` 三分法：组投影 `SHIPPED/RECEIVED`→`POST_SHIP_ALLOWED`（不得 PRE）；组投影未离仓，或无组行且 `orders.status ∈ {PAID,ALLOCATED}`→视为未离仓，走 `NONE→REVOKED→PRE_SHIP_REVOKED`；组投影缺失且 `orders.status ∈ {SHIPPED,COMPLETED}`→`REVIEW_REQUIRED`；不得默认 PRE、不得猜 `payment_no`、不得按 0 额度 |
| 零元（`source=system_zero`） | `admitZeroRefund` → `decideZeroRefundResult` | 同一映射语义（§4）；无 Tx1，**不持久化** `REVIEW_REQUIRED` |

### 3.4 R-2b：部分金额协议（PO D-31 三分法；不探测，无预读）

**约束**：人类 route A 删除一切 `ListShipments`/事务外预读；不得开 ADR §5.2.2 例外；不得丢 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；不得把部分金额静默当 POST/PRE；**不得改动运单**（AC-50 `0002:217`/INV-52）。

| permit | 金额 | 探测 | 结论 | 写入/队列 |
|---|---|---|---|---|
| 非 `CONSUMED` | `< 组剩余` | 不探测（Order 自有 permit 事实即 pre-ship） | `PARTIAL_REFUND_REQUIRES_FULL_GROUP` | Tx1 前拒绝，**零 admission/commitment/outbox**（现有 `:249-252` 语义扩展） |
| `CONSUMED` | `== 组剩余`（整组） | 见 §3.1（`CancelIfCreated`） | PRE / POST / `REVIEW_REQUIRED` | 见 §3.1 |
| `CONSUMED` | `< 组剩余` 且组投影 ∈ `{SHIPPED, RECEIVED}` | **不探测** | `POST_SHIP_ALLOWED`（部分金额合法，`0002:226`） | Tx1 写 `result=POST,state=POST` + 终态 outbox + 1 条 commitment；**不取消运单、不写 release** |
| `CONSUMED` | `< 组剩余` 且组投影为未离仓（`PAID/ALLOCATED`；或无组行且 `orders.status ∈ {PAID,ALLOCATED}`） | **不探测** | `PARTIAL_REFUND_REQUIRES_FULL_GROUP` | **写前拒绝**：零 admission/commitment/outbox；alert `PARTIAL_REFUND_ON_UNSHIPPED_GROUP`；组保持未关闭 |
| `CONSUMED` | `< 组剩余` 且组投影缺失 + `orders.status ∈ {SHIPPED,COMPLETED}`，或其他不可判定组合 | **不探测** | `REVIEW_REQUIRED` + `REFUND_ADMISSION_CONFLICT` | Tx1 写 `result=REVIEW_REQUIRED,state=PENDING_REMOTE` + **1 条 `RESERVED` commitment**（冻结该额度，防同一额度被第二个 `refund_id` 重复占用/重复退款）；不写终态 outbox；`reason=PARTIAL_AMOUNT_UNCLASSIFIABLE`；人工队列 |
| 任意 | `< 组剩余` 且组投影读取失败 | **不探测** | `Unavailable`（fail-closed） | 零写入；可重试 |
| 已有 `PENDING_REMOTE` 的部分金额申请（重评） | `< 组剩余` 且组投影未离仓 | **不探测** | `PARTIAL_REFUND_REQUIRES_FULL_GROUP`（PO AC-70 `0002:465`） | **保持既有 admission/commitment 原样**（不自动释放、不写 PRE/POST、不删除行）；由人工按 AC-62 处置 |
| 已有 `PENDING_REMOTE` 的部分金额申请（重评） | `< 组剩余` 且不可判定 | **不探测** | `REVIEW_REQUIRED` | 在既有占位/commitment 基础上写 `result=REVIEW_REQUIRED`；不新增 commitment |

- **写序**：三分判定必须发生在 Tx1 写入之前（PO AC-70 `0002:462-463`）。`PARTIAL` → rollback **零写入**；`REVIEW_REQUIRED` → 同一 Tx1 写裁决行 + **1 条 `RESERVED` commitment**（无终态 outbox；冻结额度，符合 AC-62 “REVIEW 期间承诺保持 RESERVED”）；`POST` → 正常写 POST + commitment + 终态 outbox。**不得先写 POST/PRE 再改 REVIEW**；也不得在 PARTIAL 分支留下承诺。
- **为什么不用 Order 投影决定“整组”？** 整组金额才需要“取消运单/判定离仓”的远程事实；部分金额的合法性问题（未离仓不得退部分）由**本地 permit + 组投影**足够回答，且投影是 ADR §5.2.1 已承认的 Order 自有已离仓事实（`0007:556`；PO `0002:98`），不是跨服务预读。
- **为什么部分金额不探测**：`CancelIfCreated` 对 `CREATED` 行有真实副作用（`cancel_if_created.go:70-82`），而 AC-50 `0002:217` 明确部分金额“不得改动许可/运单/打款”，PO D-31/INV-52 明确禁止用取消副作用分类。
- **为什么 `CANCELLED` 真实行也算未离仓**：无法在不探测的前提下区分“已取消”与“新建”；二者都属于“未离仓”，AC-50 对未离仓部分一律要求 PARTIAL。用户改整组后，整组探测会得到 `CANCELLED→PRE`。
- **投影滞后**：真实已离仓但投影未到 → PARTIAL（fail-closed）；用户改整组（全额退）或等投影重试；不自动改运单、不丢钱。测试 O4d。
- **不得用复核掩盖业务拒绝**：若请求本身违反额度/已全额退款/部分金额规则（AC-50/AC-64），必须返回既有业务错误，**不得**写 `REVIEW_REQUIRED`（AC-70 `0002:470`）。
- **判定口径**：`groupRemaining` 沿用 `AdmitRefund:228-248` 的本地重算（`groupPaid + groupFreight − Σ(RESERVED/SETTLED)`）。判定发生在 **Tx1 插入本笔 commitment 之前**，故不存在自计入；若未来改到 Tx2 重算，Σ 必须显式排除本 `refund_id`。组投影读取 `order_fulfillment_groups.status` + 必要时 `orders.status`，在 Tx1 内（Order 自有表，非跨服务）。
- 口径说明：若运费责任只部分可退，`整组退款金额` 的精算属 INV-47/T3，本 amendment 不改变该口径。

### 3.5 `REVIEW_REQUIRED` 的持久化与重放

1. Tx1 照旧（admission 占位 + commitment + 普通正金额占位 outbox）。
2. Tx2 调 `CancelIfCreated`；`<NO_WAYBILL>` → **同一 Tx2** 以 `WHERE refund_id=? AND state='PENDING_REMOTE'` CAS 写 `result=REVIEW_REQUIRED`（`state` 不变）、`version+1`；不写 release/终态 outbox；Error 告警（§8）；若 `source=tcc`，同事务置 `refund_batches.MANUAL_HOLD` + `last_error='REVIEW_REQUIRED:<reason>'`（`WHERE batch_id=? AND state IN ('FROZEN','ADMITTING')`）并更新 `refund_applications SET admission_result='REVIEW_REQUIRED' WHERE refund_id=? AND state='PENDING'`。
3. commitment 保持 `RESERVED`；permit 保持 `CONSUMED`；`orders.status`/`refund_state` 不写；组未关闭。
4. 同 `refund_id` 重放：`AdmitRefund` **必须在 `Payment.GetIntent` 与任何远程调用之前**先读 existing；`state=result ∈ {PRE,POST}` → 返回已持久化结果；`result=REVIEW_REQUIRED` → **返回同一 `REFUND_ADMISSION_CONFLICT`**（同一 details），**不再调用 `CancelIfCreated`/`GetIntent`**、不再写任何行；`state=PENDING_REMOTE` 且 `result≠REVIEW_REQUIRED` → 继续既有流程。
5. 并发：两个 Tx2 都拿到 `<NO_WAYBILL>`；一个 CAS 成功，另一个 0 行（`state` 已非 `PENDING_REMOTE`）→ 读回 `REVIEW_REQUIRED` → 返回同一错误；不重复写/不重复告警（告警可每请求一条，但不得重复写）。
6. 崩溃：Tx1 后崩溃 → 重放 PENDING → 探测 → REVIEW；Tx2 写 REVIEW 后崩溃 → 重放读 REVIEW → 同一错误；`CancelIfCreated` 返回 `<NO_WAYBILL>` 后、Tx2 前崩溃 → 重放 PENDING → 再探测 → REVIEW。
7. **人工重放**：正常 replay 不探测；`AC-27-NOWB-01` 终态①要求“补齐后按 AC-50 合法分支重放→PRE”。在持久化下，人工重放 = ops 在**真实既有/曾存在但丢失的 S6 证据**与授权/审计下执行**受控 re-arm**（§8.3），把 `result` 置回 `PENDING_REMOTE`（`state` 已是 `PENDING_REMOTE`），再走正常 replay → 探测 → CANCELLED→PRE；若仍 `<NO_WAYBILL>` → 再次 `REVIEW_REQUIRED`。**不得**用普通 replay 冒充人工重放，不得自动 re-arm。
8. `REVIEW_REQUIRED` 是**非准入结果、非终态**的复核标记；AC-27A 第 6 条按 `state=PENDING_REMOTE`/`result` 非 PRE → 未关闭 + 告警。


### 3.6 TCC 执行点（PO AC-71 / R-9）

| 文件/函数 | 变更 | 目的 |
|---|---|---|
| `tcc_settlement.go` `ensureTccRemoteAdmissionTx`（`:844-876`） | legal-result 增加 `REVIEW_REQUIRED`；读到该值返回 `apperr.Failed(REFUND_ADMISSION_CONFLICT…)`（含 `batch_id/refund_id/group_no/reason`），**不覆盖行、不回滚已提交事务** | 不得判非法/不得早失败 |
| `tcc_settlement.go` `ensureTccApplicationTx`（`:788-840`，单调检查 `:832-834`） | 单调检查接受 `REVIEW_REQUIRED`；已复核 application 不因重算 `PENDING_REMOTE` 判冲突 | 重试可幂等读回 |
| `tcc_settlement.go` `writeTccBatchGroupsTx`（`:628-720`） | 入口短路：批次 `MANUAL_HOLD` → 返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`，**零写入** | 自动续跑不改状态 |
| `tcc_admission.go` `admitTccBatch`/`tccBatchManualError`/`casTccBatchState` | `MANUAL_HOLD` 返回稳定错误；循环内其他组继续准入；**循环后重读 batch**：若已被 Tx2 置 `MANUAL_HOLD`，返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`（不得走 `markTccBatchAdmitted` 的 generic failure）；人工 re-arm 后允许 `MANUAL_HOLD→ADMITTING` | AC-71 |
| `tcc_admission.go` `loadTccApplications`/`admitTccApplication` | `admission_result=REVIEW_REQUIRED` 视为“自动终止”，不当作可重试 `PENDING` | 不倒灌重探 |
| `refund_admission.go` Tx2 | `source=tcc` 且 `batch_id` 非空时同事务置批 + `refund_applications.admission_result`（§3.1/§3.5） | 裁决落库即持批/持申请 |
| `refund_batches.state` | 复用既有 `MANUAL_HOLD`（0017 CHECK 已含） | 无迁移 |


---

## 4. R-3：零元路径（`source=system_zero`）

### 4.1 裁决

`decideZeroRefundResult`（`refund_admission.go:366-396`）必须与 §3.1 同表：

| permit 状态 | `CancelIfCreated` 返回 | 零元结论 | 写入 |
|---|---|---|---|
| `""`（无许可行）/`NONE`/`REVOKED` | 不调用探测 | PRE | admission PRE + `refund_applications(DONE, admission_result=PRE)` + INV-44 释放（现状） |
| `CONSUMED` | `CANCELLED` | PRE | 同上（现状） |
| `CONSUMED` | `ALREADY_SHIPPED`/`RECEIVED` | POST | admission POST + `DONE(POST)`；不释放（现状） |
| **`CONSUMED`** | **`<NO_WAYBILL>`** | **`REFUND_ADMISSION_CONFLICT`（错误）** | **不写 admission、不写 `refund_applications`、不写 `DONE`、不释放、不调 `Payment.Refund/RefundBatch`、不持久化 `REVIEW_REQUIRED`** |
| `CONSUMED` | `TRANSIENT_ERROR`/空/未知 | 不可判定 | 不写；`apperr.Unavailable` 可重试（现状） |
| `CONSUMED` | 组投影 `SHIPPED/RECEIVED`（PO AC-67 `0002:430`） | 已离仓 | POST + `DONE`；不探测/不取消 |

- 零元是“先取消、后单事务写 DONE”（T1.4a/R-C）。`<NO_WAYBILL>` 时远程没有副作用（无行可取消），直接返回错误，单事务不开启。
- **零元不持久化 `REVIEW_REQUIRED`**（PO AC-67/AC-69；D-4 已关闭）：同 `refund_id` 重放重新探测（无写入），直到真实 `CANCELLED` 行出现 → PRE + DONE，或人工放弃（AC-62 授权/`FAILED_PERMANENT`）；有限重试后进人工队列（AC-72）。
- **零元不调用 Payment**：`admitZeroRefund` 在 `refund_admission.go:154-156` 直接短路，`GetIntent`/`Refund` 均为 0；本路径“Payment 0”字面成立。
- 零元错误**不得**借用 `writeZeroRefundTx` 的 `PENDING_REMOTE` 分支；不得为凑“可持久化”伪造 `PENDING` 行或 `DONE(zero_amount)`。

### 4.2 禁止

1. 不得把零元错误写成 `DONE(zero_amount)` 或任何 `refund_applications` 行。
2. 不得在复核/有界错误时释放预留/回库（INV-44/INV-50）。
3. 不得用“无运单”跳过 permit 分支：`NONE` 的零元仍按 AC-50 置 `REVOKED → PRE`。
4. 零元 `amount=0` 在零元单上即整组，不适用部分金额表。

---

## 5. R-4：单一映射权威（反 if-else 扩散）

- 两个映射点（`finishRemoteAdmission`、`decideZeroRefundResult`）必须使用**同一张**状态→结论表（一个包级 helper/常量表）。禁止在 AfterSales/TCC/正金额/零元各写一份 `switch status`。
- 映射函数必须**穷尽 + fail-closed**：未列出值（含未来新增值）→ 不可判定/可重试，绝不落到 PRE。
- 代码评审判据：搜索 `"CANCELLED"` 在 Order 中的出现点；除该唯一映射表外，任何新增 `CANCELLED→PRE` 分支即 FAIL（宪法 I.1）。
- **禁止**为部分金额重新引入 `ListShipments`/预读；部分金额的决定只用 permit 事实 + Order 组投影（§3.4）。
- `AdmitRefund` 的 existing 判定必须收口为**一个函数**（`classifyExisting`）：`terminal`（state=result∈PRE/POST）/ `review`（`result=REVIEW_REQUIRED`）/ `pending`（`state=PENDING_REMOTE` 且无复核）/ `unknown`（fail-closed）。禁止散落 `if state == ...`。
- `REVIEW_REQUIRED` 的产生只能来自该映射表；TCC/AfterSales/零元**不得**各自造一个“复核”分支。

---

## 6. R-5：持久化与不变量

| ID | 不变量 | 违规示例 |
|---|---|---|
| **A1** | `<NO_WAYBILL>`（整组）只产生 `REVIEW_REQUIRED`；**永不**产生 `PRE_SHIP_REVOKED`/`POST_SHIP_ALLOWED` | 把无行当已取消 → PRE |
| **A2** | 准入成功终态当且仅当 `state=result` 且 `state ∈ {PRE,POST}`；`PENDING_REMOTE` 不是结果；`REVIEW_REQUIRED` 是**非结果、非终态**的复核标记；TCC 的 `result=PENDING_REMOTE` 是既有偏差（§6.2）；**`AdmitRefund` 对任何非终态 existing 必须返回错误，不得回 success reply** | 只凭 `state==result` 或只读 `result` 判关闭/放款；把 REVIEW 当成功 |
| **A3** | **本 amendment 只消除 `CONSUMED+无运单`（整组）的非法 PRE**。`PRE_SHIP_REVOKED` 的合法来源：`NONE→REVOKED`（依赖 permit 与 S6 一致；当前 Order 不校验）、`REVOKED` 幂等、`CONSUMED+CANCELLED`（真实取消）。legacy 无许可行 + 已发货仍可能写 PRE，是既有缺口（M-1），不得声称已消除 | 把 legacy/无 permit 的 PRE 也写成“已证明合法” |
| **A4** | **自动路径**在无准入成功终态时：不得调用 `Payment.Refund`/`RefundBatch`、不得写 release/commitment release、不得把申请置 `DONE`/`PROCESSING`/`FAILED_PERMANENT`（人工放弃除外）。**唯一例外**：AC-62/D-24 人工授权 `RESERVED→RELEASED`（只释放额度，不恢复 permit、不关闭组、不当作退款完成、不写 `DONE`） | 自动释放；或把 AC-62 授权释放也禁掉 |
| **A5** | `REVIEW_REQUIRED` 不改变 permit：`CONSUMED` 保持 `CONSUMED`，不得写 `REVOKED`，不得恢复/重建 permit（INV-45） | 为“关闭组”把 permit 改成 REVOKED |
| **A6** | `REVIEW_REQUIRED` 不写 `orders.status`、不写 `refund_state`（INV-01/AC-25/AC-27A） | 用复核推进生命周期 |
| **A7** | Tx1 的 `order.refund_admitted` 是非终态**占位事件**（普通正金额 `state=PENDING_REMOTE,result=POST`；TCC 的 CONSUMED Tx1 不写占位）；`REVIEW_REQUIRED` 不新增终态事件；占位保留，消费者必须忽略非终态事件 | 把占位 `POST/PENDING` 当终态；REVIEW 再补一条 outbox |
| **A8** | Tx2 更新必须 `UPDATE ... WHERE refund_id=? AND state='PENDING_REMOTE'` 并检查 `RowsAffected()==1`；0 行 → 重读：nil → `Internal/Conflict("admission disappeared during Tx2")`；`result=REVIEW_REQUIRED` → 返回同一错误；PRE/POST → 返回该结果；`state=PENDING_REMOTE` 且无复核 → `Conflict` 让调用方重读；**所有 0 行路径都不写 outbox/release**。非法组合（`result=REVIEW_REQUIRED` 且 `state≠PENDING_REMOTE`；或未知值）→ `Conflict`，不得自动修复 | 两个并发 Tx2 各写 outbox/release；0 行当成功；覆盖复核 |
| **A9** | `REVIEW_REQUIRED` 是**持久化非终态标记**，不是准入终态：不写 PRE/POST、不打款、不释放、不关闭；AC-27A 按非 PRE → 未关闭；普通 replay 返回同一错误、不再探测；只有 ops 受控 re-arm（§8.3，真实既有/曾存在但丢失的 S6 证据 + 授权 + 审计）才能清除并重探 | 把 REVIEW 当 PRE/关闭；自动 re-arm；用迟到 placeholder 当证据 |
| **A10** | 部分金额（`amount < 组剩余`）**不得调用 `CancelIfCreated`**；未离仓在 Tx1 写入前拒绝（零 admission/commitment/outbox）；已离仓只按 Order 组投影写 POST，不改运单、不 release；不可判定写 `REVIEW_REQUIRED` + 1 条 `RESERVED` commitment（冻结额度，AC-62） | 部分金额先取消运单再返回 PARTIAL；用远程探测区分部分金额；未离仓 PARTIAL 留下承诺；REVIEW 不留承诺导致同额度被二次承诺 |
| **A11** | TCC 遇到 `REVIEW_REQUIRED`：**不得**判为非法 result、不得回滚已提交的准入/承诺/占位、不得打款；批次必须 `MANUAL_HOLD`、application `PENDING`+`admission_result=REVIEW_REQUIRED`、`last_error` 可查询；自动续跑返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING` 且 0 状态变化 | TCC 把 REVIEW 当 `PENDING` 重探；batch 停在 `ADMITTING` 无人发现；自动打款 |

### 6.1 与 AC-27A 的关系（不修改 AC 文本）

- AC-27A 的关闭谓词**只信** `result=PRE_SHIP_REVOKED`（且 `state` 已终态）；不使用运单事实、`refund_state`、`reservation_state`（`0001:622-623`、`:645`）。
- 本 amendment 的贡献是**前提保证**：新写入的 `PRE_SHIP_REVOKED` **不再**由“无运单行（整组）”产生（A3）；`REVIEW_REQUIRED` 与 `PENDING_REMOTE` 均**不关闭**。
- AC-27A 第 6 条“其他（含结果缺失/不可核验）→ 未关闭”天然覆盖 `REVIEW_REQUIRED`；SPEC-0001 `0001:645` 注仍写“`result` 只允许 PRE/POST”，PO v1.11 §11 item 4 已登记“更严格且更晚批准者为准”并承诺下次升版同步（D-3）。
- 本 amendment **不修改** AC-27A 文本（SPEC 归 PO）、**不实现** AC-27A 第 5/6 条读模型（归 R0.3-R1c）。
- **存量 PRE 不因本修复自动合法**：§8 一次性审计未完成前，AC-27A 第 5 条不得对无法证明来源的 PRE 组标已关闭/AC-VERIFIED。
- 在 `AC-50-FIX-01` 关闭前，AC-27 不得 AC-VERIFIED（`0001:874`）。

### 6.2 TCC `result=PENDING_REMOTE` 的既有偏差（M-4）

- SPEC-0001 `0001:645` 写“`result` 只允许 `PRE_SHIP_REVOKED`/`POST_SHIP_ALLOWED`”；但 TCC Tx1 `ensureTccRemoteAdmissionTx` 写 `result=PENDING_REMOTE,state=PENDING_REMOTE`（`tcc_settlement.go:844-852`）。这是既有偏差，不是本 amendment 引入。
- 读者口径以 A2 为准：`state=result` **且** `state ∈ {PRE,POST}` 才是准入成功；`REVIEW_REQUIRED`/`PENDING_REMOTE` 均不是。
- 清理登记 `TCC-PENDING-RESULT-01`（owner Architect/Developer，expiry 2026-12-31）：要么不再写中间 `result`，要么由 SPEC-0002 明确允许；不得在 AC-50-FIX-01 中夹带。

---

## 7. R-6：幂等 / 并发 / 崩溃窗口（强制）

### 7.1 整组路径时序（正金额/TCC）

```text
AfterSales/TCC durable task
   │ AdmitRefund(refund_id, group, amount, batch_id?)
   ▼
Order: 读 existing admission（先于 GetIntent / 任何远程）
   ├─ state=result ∈ {PRE,POST} → 返回已持久化结果
   ├─ result=REVIEW_REQUIRED → 返回同一 REFUND_ADMISSION_CONFLICT（不探测/不读 Payment）
   ├─ state=PENDING_REMOTE 且 result≠REVIEW → finishRemoteAdmission（重探/收敛）
   └─ 不存在 → Tx1: 读 permit + 金额分类
         ├─ NONE → CAS REVOKED → PRE（合法）
         ├─ CONSUMED && amount < 组剩余 → §3.4（不探测；投影 POST/PARTIAL/REVIEW）
         └─ CONSUMED && amount = 组剩余 → 写 admission 占位 + commitment + 占位 outbox
   ▼
 CancelIfCreated(order, group) —— 仅整组金额，事务外（无预读）
   ├─ CANCELLED（整组）→ Tx2 PRE
   ├─ ALREADY_SHIPPED/RECEIVED → Tx2 POST
   ├─ <NO_WAYBILL>（整组）→ Tx2 REVIEW_REQUIRED + 告警 (+TCC MANUAL_HOLD)
   └─ TRANSIENT/未知 → Unavailable（PENDING 保持）
   ▼
REVIEW_REQUIRED：组未关闭；commitment RESERVED；占位 outbox 保留；0 打款/0 释放/0 准入终态；人工队列
```

### 7.2 崩溃/并发/重复窗口

| # | 窗口 | 行为 | 恢复 owner |
|---|---|---|---|
| W1 | Tx1 提交后、远程前崩溃 | 重放读 PENDING → 重探；按结果 PRE/POST/REVIEW/PARTIAL。Tx1 写入已存在 | 发起方 task + Order |
| W2 | `CancelIfCreated` 返回 `<NO_WAYBILL>`、Tx2 前崩溃 | Tx1 已写；重放 PENDING → 再探测 → REVIEW；不写 PRE | 发起方 task |
| W3 | `CancelIfCreated` 返回 `CANCELLED`、Tx2 前崩溃 | 重放探测 `CANCELLED`（幂等）→ Tx2 PRE | 发起方 task |
| W4 | Ship 与取消竞态 | Ship 胜 → ALREADY_SHIPPED → POST；取消胜 → CANCELLED → PRE；Ship 后置失败得 SHIPMENT_CANCELLED（AC-48） | Fulfillment CAS + Order |
| W5 | 同 `refund_id` 并发 | 订单行锁串行化；Tx2 A8 CAS 唯一化；败者读回同一裁决；admission 至多一行 | Order |
| W6 | REVIEW 写入成功、返回前崩溃 | 重放读 REVIEW → 同一错误；**不再探测** | 发起方 task + Order |
| W7 | 零元：远程 `CANCELLED` 后、单事务前崩溃 | 重放重探 → CANCELLED → PRE + DONE；零元无 commitment/Payment | 发起方 task |
| W8 | `PENDING_REMOTE` 永久悬挂（task FAILED/DLQ/租约丢失） | 保持未关闭并告警；人工重放（AC-61）；重放仍 `<NO_WAYBILL>` → REVIEW → AC-27-NOWB-01；**不是**本 amendment 的修复范围（AC-27-LIVE-01）。TCC 停 ADMITTING | ops（重放）+ Developer（AfterSales/TCC task） |
| W9 | `order.refund_admitted` 重复/乱序；迟到 `order.paid` | consumer 只处理终态 PRE；`<NO_WAYBILL>` no-op success；重复 PRE 的 `CancelIfCreated` 幂等；**迟到 `order.paid` 可能经正常 consumer 创建真实 `CREATED` placeholder（`autoShipAllowed` fallback，`fulfillment/consumer.go:111-123`；`settlement.go:210-225`）**；该行是真实 S6 行、不是人工证据。若 admission 仍 PENDING → 整组重探（`CANCELLED→PRE`）；**部分金额不探测、不自动转 PRE**（§3.4）；若已有 REVIEW → **不自动探测/不自动转 PRE**，由 Fulfillment/ops 按 §8 处置。业务若要求阻断该路径 → `ORDER-PAID-PLACEHOLDER-01` 独立 L2 | Fulfillment consumer + ops（R3-M1） |
| W10 | 部分金额 + `CONSUMED` | 投影离仓→POST；未离仓→PARTIAL（写前拒绝，零写入）；不可判定→REVIEW（写 1 条 RESERVED commitment 冻结额度）；**不探测、不改运单** | 发起方 task + ops |
| W11 | REVIEW 后人工 re-arm | ops 在真实既有/曾存在但丢失的 S6 证据 + 授权 + 审计下把 `result` 置回 `PENDING_REMOTE`（TCC 同批 `MANUAL_HOLD→ADMITTING`）；重放探测 → CANCELLED→PRE / SHIPPED→POST / `<NO_WAYBILL>`→再次 REVIEW；不得自动 re-arm、不得用迟到 placeholder 当证据 | ops（§8.3） |
| W12 | TCC 重试撞 REVIEW | `ensureTccRemoteAdmissionTx`/`ensureTccApplicationTx` 认合法裁决；batch `MANUAL_HOLD`、app `PENDING`、0 打款；自动续跑返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`；不自动 `FAILED_PERMANENT`；其他批不受阻 | Order + ops |

### 7.3 “有界”的定义与重试口径（R3-B4 / PO §5.10）

- **有界** = ① 复核裁决在首次 `<NO_WAYBILL>`/不可判定即持久化，不进入无限 `PENDING_REMOTE`；② 同 `refund_id` 重放返回**同一**结果/错误，不再远程探测、不再新增写入；③ 自动重试上限由发起方既有机制给出（AfterSales `maxTaskAttempts=8` 后 `FAILED_PERMANENT`；TCC batch 由 ops 发现），不新增 worker/扫描器；④ 唯一前进路径是人工 re-arm（同 `refund_id`）或 AC-62 授权释放后以新 `refund_id` 重新申请。
- **AC-50 `0002:228` 的“申请保持可重试”**按 PO §5.10 分界表（`0002:652-670`）收窄为“申请不丢失、幂等可重放；REVIEW 不自动收敛”；本 ADR 不再写“自动重试可收敛”。
- AfterSales `maxTaskAttempts=8`（`aftersales/service.go:58`）耗尽后 `FAILED_PERMANENT`，即人工队列信号；错误文本必须含 `REFUND_ADMISSION_CONFLICT` 以便落 `last_error`（`workers.go:724-748`）。禁止在 AfterSales 增加“复核当成功”分支。
- TCC 不经过 AfterSales：batch `MANUAL_HOLD`、application `PENDING`；发现见 §8；重放由 ops/发起方。

### 7.4 Tx1 已写事实的处置协议（B-1 保持）

**前提**：`CONSUMED` 的 Tx1 在远程探测前提交（`refund_admission.go:253-283`）。普通正金额写 admission 占位 + commitment + 占位 outbox；TCC 的 `CONSUMED` Tx1 写 admission(`result=state=PENDING_REMOTE`)+commitment（`tcc_settlement.go:844-852,898`），**不写** outbox。**不得声称“无写入/无 outbox”**。

| 对象 | `<NO_WAYBILL>`（整组）时状态/处置 | 理由 |
|---|---|---|
| `order_refund_admissions` | Tx2 写 `result=REVIEW_REQUIRED`（`state` 保持 `PENDING_REMOTE`），`version+1`（CAS `state='PENDING_REMOTE'`）；不删行 | 持久化复核裁决，供重放返回同一错误；删除会丢幂等锚点/证据 |
| `order_refund_commitments` | `RESERVED` **保留，不释放**（整组由 Tx1 写 1 条；部分不可判定由同一 Tx1 写 1 条；部分未离仓 PARTIAL 不写） | INV-45/AC-62；只有授权释放/打款才离开 RESERVED |
| outbox `order.refund_admitted` | Tx1 占位保留（普通：`result=POST,state=PENDING_REMOTE`；TCC：无占位）；**不新增终态事件** | A7；占位不是准入事实 |
| `refund_applications`（TCC/存在时） | 同 Tx2 置 `admission_result=REVIEW_REQUIRED`、`state=PENDING`；不写 `DONE`/`PROCESSING`/`FAILED_PERMANENT` | PO S20/AC-69；`ensureTccApplicationTx` 必须认该值为合法持久值 |
| `refund_batches`（TCC） | `state=MANUAL_HOLD` + `last_error='REVIEW_REQUIRED:<reason>'`（同 Tx2；`WHERE batch_id=? AND state IN ('FROZEN','ADMITTING')`） | PO AC-71；0017 CHECK 已含 |
| INV-44 release | **不写** | 只有 PRE 才发 release |
| commitment release | **自动路径不写**；只有 AC-62/D-24 人工授权可写 | INV-45/AC-62 |
| Payment | `GetIntent` 已读（首次）；**不调用 `Refund/RefundBatch`**；重放读 REVIEW 时**连 `GetIntent` 都不读** | INV-43/A4 |
| permit | 保持 `CONSUMED` | INV-45/A5 |
| 终态 | 准入终态 0（无 PRE/POST）；`REVIEW_REQUIRED` 为复核标记；人工队列 | SPEC-0001 `0001:863`；PO AC-69 |

- 同 `refund_id` 重放：读 REVIEW → 同一错误（§3.5/§7.3）。
- 同组新 `refund_id`：旧 `RESERVED` 仍占额度；新申请会被额度校验挡为 `REFUND_ADMISSION_CONFLICT`（组剩余不足）或需先人工处置；不得重复承诺（AC-62/INV-45）。
- 人工放弃：经 AC-62/D-24 授权释放旧 `RESERVED`（只释放额度）后才允许新申请；组仍不关闭、permit 仍 `CONSUMED`。

### 7.5 为什么不用“Tx1 前只读存在性检查”（假设性论证，不采用）

| 时序 | 事件 |
|---|---|
| T0 | 假设 Order 先做一次只读存在性探测（**本稿不采用、不实现**）：该组无行 |
| T1 | Fulfillment `CreateShipment` 插入 `CREATED` 行（`fulfillment/service.go:154-166`；不经过 `AuthorizeShipment`，permit 仍 `CONSUMED`） |
| T2 | 若 Order 以 T0 结果跳过 Tx1：无准入持久化；T1 的行真实存在，后续仍需 Tx1+探测，T0 已过期 |
| T3 | 真正的线性化点只能是 Tx1+探测：T1 的行被取消 → PRE；Ship 胜出 → POST；行再次消失 → REVIEW |

- 人类 route A 已裁定：**不引入任何 Tx1 前预读**（含部分金额），也不采用“取消副作用分类”；部分金额只用 Order 自有组投影。
- 若未来要引入只读 S6 查询，属对 ADR §5.2.2 的 trade-off，须修订 ADR + 人类批准，且只读、有 deadline、不用于整组 PRE/POST（PO D-31 `0002:835`）。
- 若未来要把 `CONSUMED` 的 Tx1 outbox 移到 Tx2，必须改 `AdmitRefund`/TCC 既有行为、扩 `allowed_paths` 并重做 W1–W3；本 amendment 不做（§13）。

### 7.6 回滚约束（PO §12 `0002:886`）

- PO 要求：旧代码读到 `REVIEW_REQUIRED` 必须 fail-closed、不得当 PRE/POST、不得打款/关闭；**回滚前必须先审计是否存在 `REVIEW_REQUIRED` 行**。
- 当前旧代码：`state=PENDING_REMOTE` → `finishRemoteAdmission` 会**重探**，可能把复核洗成 PRE/POST → **旧代码不是 fail-closed**。
- 结论：本 change **前向修复**；若必须回滚，ops 先按 §8.3 反向 re-arm/隔离并审计全部 `REVIEW_REQUIRED` 行；实施 Brief 必须写明“回滚前审计 + 不得自动重映射”。

---

## 8. R-7：人工队列（AC-27-NOWB-01）与 AC-72 告警/ SLA

| 项 | 裁决 |
|---|---|
| 队列载体 | **复用** SPEC-0001 §16.2 `AC-27-NOWB-01`（`0001:863`）+ PO v1.11 AC-72（`0002:481-491`）；**不新增**状态字段/表/队列模型；复核裁决即队列标记（`result=REVIEW_REQUIRED,state=PENDING_REMOTE`） |
| 组状态 | 保持未关闭（AC-27A：REVIEW/PENDING/结果缺失 → 未关闭 + 告警）；permit 保持 `CONSUMED` |
| 申请状态 | `PENDING`（TCC batch `MANUAL_HOLD`）；不自动 `DONE`/`PROCESSING`/`FAILED_PERMANENT`（人工放弃除外）；自动重试只幂等返回同一冲突（不收敛） |
| 终态①（整组复核） | Fulfillment 提供/修复**既有或曾存在但丢失**的真实 S6 取消证据（真实记录/承运商取消/仓出库撤销 + 恢复审计）→ ops 凭 Fulfillment 授权核对 → **受控 re-arm（§8.3）** → 重放探测 → CANCELLED → PRE → 组关闭。**不得为从未存在的运单新建行**；**不得把复核后由正常 `order.paid` 迟到的 placeholder 当证据**（W9） |
| 终态② | 无法补齐 → 申请保持 `PENDING`/人工队列；仅人工放弃时按 AC-62 记 `FAILED_PERMANENT` 并取得 SPEC-0002 owner 书面口径；**AC-62/D-24 授权释放旧 `RESERVED` 是唯一释放路径**（只释放额度、不恢复 permit、不关闭组、不写 `DONE`）；组保持未关闭并告警；不得自动关闭/自动退款/恢复 permit |
| 部分金额（R-2b） | 未离仓 → `PARTIAL_REFUND_REQUIRES_FULL_GROUP`（零写入，用户改整组）；不可判定 → `REVIEW_REQUIRED`（+1 条 `RESERVED` commitment 冻结额度，`reason=PARTIAL_AMOUNT_UNCLASSIFIABLE`）；已离仓 → POST。组保持未关闭 |
| owner / SLA | `Developer（Order 数据核对）/ops（队列、重放）/Fulfillment（S6 证据）`；**单笔 24h 首次响应、7 个自然日处置上限，逾期升级财务/运营负责人**（AC-72）；工程缺口 expiry 2026-12-31（`0001:856`） |
| 告警机制（固定 key/大小写） | Order `s.log.Error`，msg=`refund admission review required: consumed permit without shipment`，字段 `refund_id/order_no/group_no/source/result="REVIEW_REQUIRED"/state="PENDING_REMOTE"/reason/manual_queue=AC-27-NOWB-01`（TCC 另加 `batch_id`）；部分金额/legacy 按 `reason` 区分。`reason` 固定三值，**不得改名**：`CONSUMED_WITHOUT_SHIPMENT`/`PARTIAL_AMOUNT_UNCLASSIFIABLE`/`LEGACY_SHIPMENT_UNKNOWN`。不得改名（T-6）；`deploy/**` 规则见 R-10 |
| 队列发现（普通售后） | `SELECT refund_id,order_no,group_no,source,result,state,updated_at FROM order_refund_admissions WHERE result='REVIEW_REQUIRED' AND state='PENDING_REMOTE'`；交叉 AfterSales `after_sale_refund_tasks.last_error LIKE '%REFUND_ADMISSION_CONFLICT%'` |
| 队列发现（TCC，M-7） | `SELECT batch_id,order_no,payment_no,state,last_error,updated_at FROM refund_batches WHERE state='MANUAL_HOLD'` + `SELECT refund_id,group_no,state,admission_result FROM refund_applications WHERE batch_id=?`；原因读 `refund_batches.last_error`（R-9 写入）。**不得只依赖发起方 last_error**（PO AC-61 `0002:318`） |
| 存量 PRE 审计（M-5，R2-M2 修正） | `SELECT a.refund_id,a.order_no,a.group_no,a.result,a.state,p.state AS permit_state,g.status AS group_status FROM order_refund_admissions a LEFT JOIN order_shipping_permits p ON p.order_no=a.order_no AND p.fulfillment_group_no=a.group_no LEFT JOIN order_fulfillment_groups g ON g.order_no=a.order_no AND g.group_no=a.group_no WHERE a.result='PRE_SHIP_REVOKED' AND (a.state='PRE_SHIP_REVOKED' OR a.state='PENDING_REMOTE')`；分别列出：`g.group_no IS NULL`、`g.status IN ('SHIPPED','RECEIVED')`、`a.state='PENDING_REMOTE'`（不可核验）、`p.state IN ('REVOKED','CONSUMED')`。owner=ops+Developer，expiry 2026-12-31；**0 行 ≠ 安全**。审计结论未出前，AC-27A 第 4/5 条不得对相关组标已关闭/AC-VERIFIED |
| metrics（AC-72） | `mall_refund_review_backlog`、`mall_refund_review_oldest_seconds`（`internal/platform/metrics`）；Order 既有 tick 只读刷新（R-10）；7 天未处置 > 0 视为运营阻断并在日报列出 |

### 8.2 与 SPEC-0001 §16.2 的终态对齐

| SPEC-0001 §16.2 | 本 ADR 的对应 |
|---|---|
| **AC-27-NOWB-01 阻断** | 整组复核（或部分不可判定）：组保持未关闭；申请 `REFUND_ADMISSION_CONFLICT`/`PARTIAL_REFUND_REQUIRES_FULL_GROUP`；不写 PRE/不打款/不自动释放；其他组不受影响 |
| **AC-27-NOWB-01 终态①** | Fulfillment 提供/修复**既有或曾存在但丢失**的真实 S6 取消证据 → ops 凭 Fulfillment 授权核对（Order 不写运单）→ §8.3 re-arm → 探测 `CANCELLED` → PRE → 组关闭；不得新建行、不得用迟到 placeholder 当证据 |
| **AC-27-NOWB-01 终态②** | 无法补齐 → 申请保持 PENDING/人工队列；仅人工放弃时按 AC-62 记 `FAILED_PERMANENT` 并取得 SPEC-0002 owner 书面口径；AC-62/D-24 授权释放 `RESERVED` 是唯一释放路径；组保持未关闭并告警；不得自动关闭/自动退款/恢复 permit |
| **AC-27-LIVE-01** | `PENDING_REMOTE` 永久悬挂（DLQ/租约丢失）→ 保持未关闭并告警；AC-61 人工重放；重放仍 `<NO_WAYBILL>` → REVIEW → 转 AC-27-NOWB-01；不写 PRE/不关闭/不打款 |

### 8.3 人工 re-arm runbook（AC-27-NOWB-01 终态① / AC-69）

- **仅 ops**，在 Fulfillment 提供**真实既有/曾存在但丢失**的 S6 证据、ops 授权核对、审计工单齐备后执行；不得自动、不得由 worker/重试触发。
- 组裁决：`UPDATE order_refund_admissions SET result='PENDING_REMOTE', version=version+1, updated_at=NOW(3) WHERE refund_id=? AND result='REVIEW_REQUIRED' AND state='PENDING_REMOTE' AND version=?`；`state` 不动。
- **原复核记录**：admission 行按状态机迁移（`REVIEW_REQUIRED→PENDING_REMOTE→PRE/POST`），行内不保留历史值；审计依据为 ops 工单（操作人/授权人/证据/时间/version）+ 结构化日志 +（TCC）`refund_batches.last_error`。若 PO 要求行级历史/独立审计表，属新表 → 另立 L2（`REVIEW-AUDIT-01`），不得由 Developer 夹带。
- TCC：同一审计流程把 `refund_batches SET state='ADMITTING', last_error='', version=version+1 WHERE batch_id=? AND state='MANUAL_HOLD' AND version=?`；`refund_applications` 的 `admission_result` 保持 `REVIEW_REQUIRED` 以便重评时被 `ensureTccApplicationTx` 认作可重算（若实现选择在 re-arm 时清空，必须在实施 Brief 写明并测试）。
- **不动 commitment/permit/outbox**；审计工单记录 refund_id/batch_id/操作人/授权人/证据/时间/version。
- 随后同 `refund_id` 重放 `AdmitRefund`：探测 `CANCELLED` → PRE → 组关闭；`SHIPPED` → POST；仍 `<NO_WAYBILL>` → 再次 REVIEW（保持人工队列）。
- **唯一合法过渡**：

| 当前 | 允许动作 | 目标 | 禁止 |
|---|---|---|---|
| `PENDING_REMOTE` | 探测收敛 | `PRE` / `POST` / `REVIEW_REQUIRED`（不判定） | 直接写 REVIEW 以外的终态 |
| `REVIEW_REQUIRED` | 人工 re-arm（同 `refund_id`） | `PENDING_REMOTE` → 重评 | 自动 re-arm；直接跳 `PRE`/`POST`；删除记录 |
| `REVIEW_REQUIRED` | 人工放弃（AC-62 授权释放） | 保留 REVIEW + 承诺 `RELEASED` + 申请 `FAILED_PERMANENT` | 自动释放；写 `DONE`；关闭组 |
| `PRE`/`POST` | — | 终态 | 回退为 REVIEW/PENDING |
| TCC `MANUAL_HOLD` | 人工 re-arm | `ADMITTING` → 续跑 | 自动推进；`FAILED_PERMANENT`；删批 |

- **禁止**：用迟到 `order.paid` placeholder 当证据；用普通 replay 代替 re-arm；re-arm 后自动打款/释放（仍走 AC-50/AC-62 原流程）。
- 若组织不允许人工 DB 写：终态①改走“AC-62 授权释放旧 RESERVED + 新 `refund_id` 整组准入”（依赖 T3 release 实现）；在此之前 term① 保持未关闭（§1.3 stop）。

---

## 9. R-8 / R-10：登记面、迁移与可观测性

| 登记项 | 是否需要 | 内容 / owner | 证据 |
|---|---|---|---|
| `api/proto/mall/v1/mall.proto` 注释 | **必须** | `CancelIfCreatedReply.status` 列出 `<NO_WAYBILL>` 及五值语义（名称由 PO/Lead 定）；RPC 注释补“or report the group has no shipment row”。owner Developer；Architect 文本见 §2.1。**不加字段/不加 RPC** | `mall.proto:1043`、`:1089-1098`；`make proto` 后 `mall.pb.go` + `mall_grpc.pb.go` 注释同步 |
| `api/gen/**` | **仅生成** | `make proto`（`Makefile:153-157`）；禁止手改；**T-2** 固定 `protoc`/`protoc-gen-go` 版本；`git diff -- api/gen` 只允许两文件注释行 | `scripts/check-proto-evolution.py`；`make proto-guard` |
| `docs/contracts.md` | **不需要** | 仅规定 `apperr` 映射；ADR §5.3 是契约正文 | `contracts.md:235-239` |
| 错误码 | **不新增** | `REFUND_ADMISSION_CONFLICT`（`0002:517`/v1.11 `0002:601`）与 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；TCC 另用 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`（PO v1.11 `0002:601`）；`apperr.Failed`→`FAILED_PRECONDITION` | `apperr.go:61-64`；`refund_admission.go:312-328` |
| 持久裁决值 `REVIEW_REQUIRED` | **PO v1.11 D-30 已定；无 schema 变更** | `order_refund_admissions.result`（`VARCHAR(32)`）+ `refund_applications.admission_result`（`VARCHAR(24)`，15 字符）；`state` 保持 `PENDING_REMOTE`；TCC batch `MANUAL_HOLD`（0017 CHECK 已含） | `0016:7-19`；`0017:20-33,56`；`0002:834` |
| 迁移 | **无** | 不得新增 `0024`；若实现需要迁移 → 停止（§15.2） | `0002:834`；`0023:1-12` 的 frozen/checksum 合同 |
| 事件/payload | **不需要** | REVIEW 不新增 `order.refund_admitted`；Tx1 占位按 A7 | `refund_admission.go:610-619` |
| ADR §12.5.3 列注释 | **文档同步（无 DDL）** | `0007:1130` 的 `admission_result` 注释在 Rev 2.4 errata 中同步；本 change 不改 accepted 正文 | `0007:1130` |
| metrics（R-10/AC-72） | **必须** | `mall_refund_review_backlog`、`mall_refund_review_oldest_seconds`（additive）；`order/worker.go` 既有 tick 只读刷新；**不新增 worker** | `internal/platform/metrics/metrics.go`；`order/worker.go:11-46` |
| 告警规则（AC-72） | **必须（ops 复核）** | `deploy/compose/observability/alerts.yml` 增规则（`mall_refund_review_backlog > 0` 或 `oldest > 0`，`for` 由 ops 定；PO 要求 5 分钟内） | `alerts.yml:4-63` |
| 用户可见/人工入口（AC-72/AC-61） | **另立 L2** | BFF/UI/RPC 入口不在本 change；登记 `REVIEW-REQUIRED-ENTRY-01`（Lead/BFF/ops，2026-12-31）；未实现前 AC-72 的“用户可见/入口”不得 AC-VERIFIED | §1.3；PO `0002:483-485` |
| **T-1** SPEC-0002 错误表漂移 | **PO v1.11 已修** | v1.11 `0002:601` 已把 `REFUND_ADMISSION_CONFLICT` 扩为“准入状态/额度冲突”，并说明 `REVIEW_REQUIRED` 不是错误码 | `0002:601-602` |

### 9.2 CHECK 判定（不在本 change）

| 变更 | 是否需要 | 说明 |
|---|---|---|
| `order_refund_admissions.result/state` CHECK | 否 | `ADD CHECK` = COPY + 独占 MDL（`docs/ops.md:259-275`）；另立 `ADMISSION-RESULT-CHECK-01`，预检 + `_v2` 加后删 |
| `refund_applications.admission_result` CHECK | 否 | 同上；枚举含 `PRE/POST/PENDING_REMOTE/REVIEW_REQUIRED/''`，风险大于收益 |
| 默认值/回填 | 否 | 默认保持 `PENDING_REMOTE`；REVIEW 只对新写生效 |

---

## 10. 测试矩阵（实施强制最小集）

> 类型：U=单元/fake port；I=真实 MySQL 集成；R=`-race`；L=日志断言；N=NATS 集成。**setup**：fresh=新申请；preseed=预置 Tx1 行/commitment/占位；done=已有终态/复核。绑定精确 HEAD，日志入库由 Developer 提供；不得用全绿 mock 冒充。**无 `ListShipments` 用例**（人类 route A）。

| # | setup | 场景 | 层 | 断言 | 期望 |
|---|---|---|---|---|---|
| **F1** | — | Fulfillment：无运单行 | U/I | `status="<NO_WAYBILL>"`；`shipments` 0 行；无 outbox/事件 | 新增 |
| **F1b** | — | Fulfillment：`shipment_no` 非空 + 无行 | U/I | `<NO_WAYBILL>`；不误取消同组其他行 | T-5 |
| **F2** | — | Fulfillment：真实 `CREATED` 行 | U/I | `CANCELLED` 且行变 `CANCELLED` | 回归 |
| **F3** | — | Fulfillment：已是 `CANCELLED` | U/I | 幂等 `CANCELLED` | 回归 |
| **F4** | — | Fulfillment：`SHIPPED`/`RECEIVED` | U/I | `ALREADY_SHIPPED`/`RECEIVED` | 回归 |
| **F5** | — | Fulfillment：`EXCEPTION`+`shipped_at` / 无 | U/I | `ALREADY_SHIPPED` / `TRANSIENT_ERROR` | 回归 |
| **F6** | — | Fulfillment：G1 无行、G2 有 `CREATED` 行 | U/I | 请求 G1 → `<NO_WAYBILL>`；G2 不被取消 | 组隔离 |
| **F7** | N | consumer 收到 `<NO_WAYBILL>`；重复 PRE | N/I | `<NO_WAYBILL>` no-op success；重复 PRE 不重复取消 | M-8/M-11 |
| **F7b** | N | G1 late `order.paid`（真实 placeholder） | N/I | 允许创建真实 `CREATED`（正常 consumer）；整组 PENDING 下重探可取消→PRE；部分金额不探测→PARTIAL；已有 REVIEW 时不自动 PRE；记录为正常 consumer、非人工证据 | R2-M1/W9/R3-M1 |
| **F8** | — | 正常路径：`CONSUMED+CREATED`→PRE；`CONSUMED+SHIPPED`→POST | I | 真实行/真实 permit；不依赖合成无行 | M-12 |
| **O1** | fresh | 正金额 `CONSUMED` + **整组** + `<NO_WAYBILL>` | I | `FAILED_PRECONDITION` 含 `REFUND_ADMISSION_CONFLICT`；`result=REVIEW_REQUIRED,state=PENDING_REMOTE`；1 条 `RESERVED` commitment；普通占位 outbox 1 条；**0 终态 outbox/0 release**；permit `CONSUMED`；`refundCalls==0` | **核心** |
| **O1b** | fresh | TCC `source=tcc` + **整组** + `<NO_WAYBILL>`，入口走 `TccCancelMarkPaid` | I/R | `result=REVIEW_REQUIRED`；batch `MANUAL_HOLD`（0 打款）；application `PENDING`+`admission_result=REVIEW_REQUIRED`；`refund_batches.last_error` 含 `REVIEW_REQUIRED`；**不出现 “not a legal value”**；自动续跑返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING` 且 0 状态变化 | R3-B1/AC-71 |
| **O2** | fresh | O1 同 `refund_id` 重放 | I | 返回同一 `REFUND_ADMISSION_CONFLICT`；`cancelCalls` **不增加**；`getCalls` 不增加；行数/commitment/outbox 不变 | 有界 |
| **O2b** | preseed | 预置 `result=REVIEW_REQUIRED,state=PENDING_REMOTE` 后调用 | I | 同一错误；不探测；不读 Payment；无新写入 | 重放 |
| **O2c** | preseed | 预置 REVIEW + `Payment.GetIntent` 不可用 | I | 仍返回同一错误（existing 先于 GetIntent） | 幂等（R2-B1） |
| **O3** | fresh | Tx1 后崩溃模拟（预置 PENDING + 占位 + commitment）→ 探测 | I | 写 `result=REVIEW_REQUIRED`（state 不变）；无终态/释放/退款 | W1/W2 |
| **O4** | fresh | 正金额 `CONSUMED` + 整组 + `CANCELLED`（单组） | I | PRE；admission PRE/PRE；1 条 commitment；release 按 INV-44（单组） | 回归 |
| **O4a** | fresh | `NONE`/`REVOKED` + **部分** | I | `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；**0 admission/commitment/outbox**；不探测（`cancelCalls==0`） | R-2b |
| **O4b** | fresh | `CONSUMED` + **部分** + 组投影未离仓 + 真实 `CREATED` 行 | I | `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；**`cancelCalls==0`**（运单仍 `CREATED`）；0 admission/commitment/outbox；0 release；告警 reason 正确 | **R3-B2 核心** |
| **O4c** | fresh | `CONSUMED` + **部分** + 组投影 `SHIPPED` | I | POST；1 条 commitment；`cancelCalls==0`；不取消运单；部分金额合法 | R-2b |
| **O4d** | fresh | `CONSUMED` + **部分** + 组投影缺失 + `orders.status=SHIPPED` | I | `REVIEW_REQUIRED`；`reason=PARTIAL_AMOUNT_UNCLASSIFIABLE`；**1 条 `RESERVED` commitment**（冻结额度）；`cancelCalls==0`；0 release/0 终态 outbox | AC-70/AC-62 |
| **O4e** | preseed | 已有 `PENDING_REMOTE` 部分金额（Tx1 占位/commitment）+ 重评未离仓 | I | `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；**既有 admission/commitment 保留、不新增写入**；`cancelCalls==0`；不删行 | AC-70 `0002:465` |
| **O5** | fresh | 正金额 `CONSUMED` + 整组 + `ALREADY_SHIPPED`/`RECEIVED` | I | POST；恰好 1 条 RESERVED commitment（Tx1 已有，Tx2 不重复）；不取消 | M-10 |
| **O5c** | fresh | 正金额 `CONSUMED` + 整组 + Order 组投影 `SHIPPED` | I | POST；`cancelCalls==0`（不探测/不取消）；1 条 commitment；不 release | AC-50 `0002:236` |
| **O5b** | fresh | TCC 已离仓（`recordTccPostAdmission`） | I | POST + commitment 同事务（既有，不改）；无探测 | M-10 |
| **O6** | fresh | 正金额 `CONSUMED` + `TRANSIENT_ERROR`/空/未知 | I | `Unavailable`；PENDING 保持；**不写 REVIEW**；无终态/释放；`refundCalls==0` | fail-closed |
| **O6b** | preseed | 预置 PENDING + `TRANSIENT_ERROR` | I | `Unavailable`；计数不变；不写 REVIEW | M-13/AC-70 |
| **O7** | fresh | 零元 `CONSUMED` + `<NO_WAYBILL>` | I | 冲突；admission 0 行；application 0 行；无释放；`refundCalls==0 && getCalls==0`；permit 不变；**无 REVIEW 持久化** | **核心** |
| **O7b** | fresh | 零元 `CONSUMED` + `TRANSIENT_ERROR`/空/未知 | I | `Unavailable`；0 admission/application；无释放/无 Payment | T-4 |
| **O8** | fresh | 零元 `<NO_WAYBILL>` 重放；随后真实 `CANCELLED` 行 | I | 第一次错误 0 写入；真实取消后重试 → PRE + DONE | 零元终态① |
| **O9** | fresh | `TRANSIENT_ERROR`/未知重放（普通） | I | 保持 PENDING；下次探测可收敛；不写终态/REVIEW | 重试 |
| **O10** | fresh | legacy + `<NO_WAYBILL>`（整组） | I | 同一 REVIEW 裁决；legacy 无许可行（NONE）未离仓仍 PRE；组投影已离仓 → POST | AC-70 |
| **O10b** | — | legacy 无 permit + 组投影缺失 + `orders.status=SHIPPED` | I | `REVIEW_REQUIRED`；`reason` 可用；不默认 PRE | AC-70 |
| **O11** | fresh | 同 `refund_id` 并发 + `<NO_WAYBILL>` | I/R | 两路同一错误；admission 1 行且 `result=REVIEW_REQUIRED,state=PENDING_REMOTE`；终态/释放 0；不覆盖为 PRE/POST | 并发 |
| **O11b** | fresh | 同 `refund_id` 并发 + `CANCELLED` | I/R | admission 1 行；终态 outbox 1；release ≤1（A8） | M-9 |
| **O11c** | preseed | Tx2 前删除/篡改 admission，0 行 | I | `Internal/Conflict("admission disappeared during Tx2")`；无 outbox/release；不 panic；非法组合 fail-closed | R2-M5/R3-T2 |
| **O12** | fresh | 告警键（普通 + TCC + 部分 + legacy） | L | 固定 key/reason（§8）；TCC 含 `batch_id`；5 分钟告警规则存在 | AC-72 |
| **O13** | preseed | AC-27A：`CONSUMED` + REVIEW / PENDING | I | `groupIsClosed=false`；本修复不写 permit/不关闭 | 非回归 |
| **O13b** | preseed | TCC `result=PENDING_REMOTE,state=PENDING_REMOTE` | I | `groupIsClosed=false`（A2 双条件） | M-4 |
| **O14** | N | consumer 重复/乱序（PRE 重投、迟到 `order.paid`） | N | 不重复取消/不建重复 placeholder/REVIEW 后不自动 PRE | M-8/W9 |
| **O14b** | N | G1 late `order.paid` + 部分金额 PENDING | N | 重探不探测、不转 PRE；记录真实 placeholder 非人工证据 | R3-M2 |
| **O15** | — | AC-62 人工授权释放（REVIEW 后） | — | `[DEFERRED]`（T3；Developer/T3，2026-12-31）：不得用未授权释放/fixture 冒充；实现时断言只释放一次、不恢复 permit、组仍不关闭、未授权 → `REFUND_RELEASE_NOT_AUTHORIZED` | M-3 |
| **O16** | preseed | 人工 re-arm（`result→PENDING_REMOTE`，TCC batch `MANUAL_HOLD→ADMITTING`）后重放 | I | 探测 `CANCELLED`→PRE；审计/授权；不动 commitment/permit；不得自动 re-arm；`version` CAS 生效 | §8.3 |
| **O17** | — | 部分金额旧承诺释放 + 新 `refund_id` 整组退款 | — | `[DEFERRED]`（T3）：依赖 AC-62 release writer；不得改旧 `refund_id` 金额 | R-2b |
| **O18** | I | AC-72 metrics/SLA | I | `mall_refund_review_backlog/oldest` 按 REVIEW 行计数；7 天未处置项可查询；REVIEW 不计入已退款 | AC-72 后端 |

补充要求：

- **金额 Mock 真实性**：正金额 >0；零元 `pay_cent=0`、`settlement_payment_no='zero:<order_no>'`；不得用 0 元冒充正金额（宪法 IV.3）。
- **合成状态**：O1/O3/O7 的 `CONSUMED+无行` 是 fail-closed 合成异常（§0.2）；另有 F8 正常路径。
- **不得** `t.Skip()`、fixture 伪造 `CANCELLED` 行冒充生产证据、或 `if status=="<NO_WAYBILL>" { return nil }` 绕过 O1/O7。O8/O16 的 fixture 是模拟 Fulfillment 真实证据后的状态，须注释，Reviewer 区分 fixture 与生产证据。
- **不得**为部分金额引入 `ListShipments`/预读；O4b/O4c/O4d 只用 permit + Order 组投影 + `CancelIfCreated` fake。
- O4/O4b/O4c 默认单组；多组释放缺口见 §14 `INV-44-MULTI-01`，不得用 Order fixture `RELEASED` 冒充。
- **O1b 必须走 `TccCancelMarkPaid`/`applyTccCompensation`**，只调 `AdmitRefund` 会绕过 `ensureTccRemoteAdmissionTx`，测不到 R3-B1。

---

## 11. 反模式自省（强制）

| 反模式 | 本设计如何阻止 |
|---|---|
| 用“无运单 → 当已取消”的和稀泥映射掩盖异常 | 新增 `<NO_WAYBILL>`；整组写 `REVIEW_REQUIRED`，不写 PRE；O1/O7 |
| 把 Tx1 已写说成“无写入/无 outbox” | A7/§7.4 承认并处置；O1/O3 断言占位 1、终态 0；O2b/O3 preseed 证明“无新增” |
| 把 `GetIntent` 当成退款/违禁调用 | 只禁 `Refund/RefundBatch`；`GetIntent` 只读允许；REVIEW 重放不读 Payment（O2b/O2c） |
| 为“制造取消证据”新建运单行 | 人工侧禁止；正常 consumer 的真实 placeholder 不是证据、REVIEW 后不自动 PRE（W9）；阻断另立 L2 |
| 用 REVIEW 当 PRE/关闭/放款 | A9：非结果/非终态；AC-27A 未关闭；无 release/payment |
| 自动 re-arm / 用普通 replay 冒充人工重放 | §8.3：仅 ops、真实 S6 证据 + 授权 + 审计；普通 replay 返回同一错误不探测 |
| 部分金额用预读或静默当 POST/PRE，或先取消运单 | R-2b：不探测、不改运单、三分法；O4b/O4c/O4d |
| 先写 RESERVED 再写 REVIEW | A10 + §3.4 写序；O4d 断言 0 commitment |
| 在 AfterSales/TCC/零元各写一份 `switch status` | R-4 单一映射权威；Reviewer 搜 `"CANCELLED"` 出现点 |
| TCC 把 REVIEW 判为“非法 result”或当 PENDING 重探 | R-9/A11；O1b 走 `TccCancelMarkPaid`；`TCC_BATCH_REQUIRES_MANUAL_HANDLING` |
| 两个并发 Tx2 各写事件/释放 | A8 `state` CAS + 0 行语义；O11b/O11c |
| 用迁移/DB CHECK 兜底业务语义 | 本 change 无迁移；CHECK 另立 L2 |
| 只做后端裁决就声称 AC-72 完成 | §1.3 `REVIEW-REQUIRED-ENTRY-01`；用户可见/入口另立 L2 |

---

## 12. 【假设与不确定性】（强制）

1. **SPEC-0002 v1.11 仍 `in-review`**（工作树未 commit）：本文件按 2026-11-26 读取的 v1.11 对齐；PO 若改名/改表示，回 Architect 重算。v1.11 approved + QC + 人类 `ADR_APPROVED` 前不得实现。
2. **Fulfillment 侧新值命名**：`<NO_WAYBILL>` 为占位；PO D-30 只定 Order 持久裁决，Fulfillment status 值须 Lead/PO 在实施 Brief 确认；改名需同步 proto/测试/日志。
3. **普通售后无 `refund_applications` 行**：代码只在 Order 的 zero/TCC 路径写该表（`refund_admission.go:498`、`tcc_settlement.go:789`）；AfterSales 用 `after_sale_refund_tasks`。AC-69 的“发起方 `refund_applications.admission_result`”按“该表存在处（TCC/支付级）才写”实现，**不得为普通售后新建该行**（会双写事实）。需 QC/PO 确认（D-2）。
4. **“无行”事实边界**：依赖 `(order_no, fulfillment_group_no)` 唯一约束（`0003:9`）；未来同组多运单需重新裁决。
5. **`shipment_no` 语义**：Order 传空（现状）→ 组级探测；非空时 `<NO_WAYBILL>` 只表示三键无行；发布前确认仓库外无调用方（T-5）。
6. **`REVIEW_REQUIRED` 组合**：只允许 `result=REVIEW_REQUIRED` 且 `state=PENDING_REMOTE`；其他组合 fail-closed（A8）。replay/Tx2 必须先判 `result=REVIEW_REQUIRED`（早于 PENDING 分支），Tx2 CAS 必须 `state='PENDING_REMOTE'`。
7. **部分金额依赖 Order 组投影**：ADR §5.2.1 + PO v1.11 `0002:98`/AC-70 已采用；若 QC 不承认，退回“显式只读 S6 查询”并须修 ADR §5.2.2 + 人类批准（PO D-31 `0002:835`）。
8. **投影滞后**：真实已离仓但投影未到 → PARTIAL（fail-closed）；用户改整组或等投影重试；不自动改运单。
9. **人工 re-arm**：把“REVIEW → `PENDING_REMOTE`”的受控 re-arm 作为终态①的人工重放路径；仅 ops、真实既有/曾存在但丢失的 S6 证据 + 授权 + 审计。若组织禁止人工 DB 写，term① 改走“AC-62 释放 + 新 `refund_id`”，在 T3 前保持未关闭。
10. **TCC `MANUAL_HOLD` re-arm**：本稿用 ops 审计 SQL 把 batch `MANUAL_HOLD→ADMITTING`；若组织要求 RPC/CLI 入口，另立 L2（`REVIEW-REQUIRED-ENTRY-01`）。
11. **迟到 `order.paid` placeholder**：`autoShipAllowed` fallback 可达（G1+`CONSUMED`+`refund_state=NONE`），会创建真实 `CREATED` 行；W9 按真实行为写；阻断另立 `ORDER-PAID-PLACEHOLDER-01`。
12. **`LEGACY-SHIPPED-PRE-01`（M-1）**：`ensurePermitTx`（`settlement.go:263-278`）无 permit 时建 `NONE`；`AdmitRefund` 对 `NONE` 不做 S6 校验。AC-70 已给 legacy 三分法；`NONE+已发货→PRE` 的存量缺口仍需审计；owner Architect/PO，2026-12-31。
13. **`TCC-LAST-ERROR-01`（M-7 剩余）**：本 change 只在 `REVIEW_REQUIRED` 路径写 `refund_batches.last_error`；其他 TCC 失败仍只 `s.log.Error`；剩余部分 DEFERRED。
14. **`TCC-PENDING-RESULT-01`（M-4）**：TCC Tx1 `result=PENDING_REMOTE` 与 SPEC 注不一致；清理归 Architect/Developer，2026-12-31；之前读者按 A2 双条件。
15. **`INV-44-MULTI-01`**：多组 release 未实现（`refund_release.go:178-187` 只在单组发事件）；AC-27 EARS-3/INV-44 不得标 VERIFIED。
16. **回滚**：旧代码读到 REVIEW 会重探（非 fail-closed）；**禁止无审计回滚**（§7.6/PO `0002:886`）；本 change 前向修复。
17. **AC-72 用户可见/入口**：本 change 不含 BFF/UI/RPC 入口；`REVIEW-REQUIRED-ENTRY-01` 未关闭前 AC-72 不得全量 AC-VERIFIED。
18. **`change_id` 拆分**：实施须由 Lead 另拆（`0001:879`）；不得把本文件当 R0.3-R1c 放行。
19. **原复核记录的载体**：admission/application 行随状态机迁移，不保留历史 `REVIEW_REQUIRED` 值；审计依据是 ops 工单 + 结构化日志 + `refund_batches.last_error`。PO AC-69/AC-72 的“保留原复核记录、不得删除”若被解释为行级历史或独立审计表，属新表/迁移，需另立 L2（`REVIEW-AUDIT-01`，D-8）。
20. **部分不可判定 REVIEW 的额度冻结**：为避免同一额度被第二个 `refund_id` 重复承诺/重复退款，fresh 部分不可判定 REVIEW 会写 1 条 `RESERVED` commitment（AC-62 “REVIEW 期间承诺保持 RESERVED”）；用户改整组需先按 AC-62 处置旧承诺。若 PO/QC 认为不应冻结，需明确“不得双花”的替代护栏，回 Architect。
21. **可达性/未执行**：`CONSUMED+无行` 正常路径不可达；生产发生率 `[不确定]`。本文件未运行测试/迁移；代码行号是阅读事实，不构成执行证据。`[不确定]`：仓库外调用方；生产 `shipments` 是否已全部应用 `0003`。

---

## 13. 不交付 / 明确不做（防范围漂移）

- **不交付 T2/T3/T4.4**；仍按 `docs/lead/PAY-SETTLE-01/tasks.md` 的既有 DEFERRED。
- **不改 SPEC-0002**（PO 已改 v1.11）、**不改 SPEC-0001**、**不改 accepted ADR Rev 2.3 正文**（未来 Rev 2.4 errata 由 Architect 合并，需人类批准）。
- **不新增**：RPC、proto 字段、事件 subject/payload、迁移（本 change 无迁移）、permit 状态值、新 `state` 值、worker、分布式锁、配置开关；**不新增 `ListShipments`/`GetShipmentByOrder`/事务外预读**。
- **不改**：Payment 金额/退款实现、AfterSales task 策略、Inventory 释放协议、`orders.status`/`refund_state` 语义、Fulfillment consumer 生产语义、`autoShipAllowed`/`order.paid` 路径（阻断另立 L2）。
- **不做**：AC-27A 读模型实现（R0.3-R1c）；M-1 legacy 修复；M-4 TCC result 清理；T3 release/re-arm 工具化；把 Tx1 outbox 移到 Tx2 的重构；BFF/UI/人工入口；DB CHECK——均另立 `change_id`/L2。

---

## 14. Baseline 影响 / INV / AC 映射

### 14.1 INV

| INV | 影响 | 本裁决如何保持 |
|---|---|---|
| INV-36（退款统一准入；`NONE→REVOKED`；已授权未离仓取消运单；同键幂等） | 涉及 | 只改 `CONSUMED` 探测映射与部分金额；`NONE` 分支不变（A3 收窄）；幂等由“读 existing + 重探 + A8”保证 |
| INV-42（申请不丢失、可重试、同键幂等） | 涉及 | 复核/部分不丢申请；Tx1 admission/commitment 保留；同 `refund_id` 返回持久化裁决/同一错误；不自动收敛（PO §5.10） |
| INV-43（未得 PRE/POST 前不得调 Payment） | 涉及（核心） | REVIEW/PARTIAL 不产生准入成功 → 禁止 `Refund/RefundBatch`（O1/O4b/O7/O9 断言 `refundCalls==0`）；`GetIntent` 只读不在此限 |
| INV-44（库存处置有主；释放失败重试告警） | 涉及 | REVIEW/PARTIAL 不触发释放；PRE 后走既有持久释放；多组缺口 `INV-44-MULTI-01`（DEFERRED） |
| INV-45（准入不可逆；permit 终态不可恢复；释放纪律） | 涉及（核心） | REVIEW/PARTIAL 不写 permit、不自动释放；AC-62 授权释放为唯一例外；AC-69 人工放弃才释放 |
| INV-50（零元不经过 Payment；DONE 为执行事实） | 涉及 | 零元错误不写 DONE、不调 Payment；不持久化 REVIEW；合法 CANCELLED 仍 DONE(PRE) |
| **INV-51**（复核决定有界、自动不可收敛） | **新增/核心** | A1/A9/A11；REVIEW 自动重试同结果不重探；仅 re-arm/新 `refund_id` 继续；SLA/告警 |
| **INV-52**（准入分类不用取消副作用） | **新增/核心** | R-2b 不调 `CancelIfCreated`；三分法只用 Order 自有事实；A10 |
| INV-01 / AC-25 | 不涉及（不得触碰） | REVIEW 不写 `orders.status`/`refund_state` |
| INV-46/47/48/49 | 不涉及 | 不改历史支付/行级/Σ/shadow |

### 14.2 AC 映射（证据归 Reviewer，本文件不更新台账）

| AC | 关系 | 说明 |
|---|---|---|
| AC-50（v1.11 `0002:222-245`） | 直接修复整组 + 部分 | 整组无运单 → `REFUND_ADMISSION_CONFLICT` + `REVIEW_REQUIRED`；部分三分；不写 PRE/不打款/不释放 |
| **AC-69..72**（`0002:446-491`） | **新增核心** | 取值/持久化/幂等（AC-69）、产生条件（AC-70）、TCC `MANUAL_HOLD`（AC-71）、可见性/SLA/对账（AC-72）；本 change 实现后端，AC-72 用户可见/入口另立 |
| AC-27A（`0001:615-645`） | 前提保证，不改文本 | 新写 PRE 只来自 A3 合法分支；REVIEW/PENDING 不关闭；存量审计前不得关闭/AC-VERIFIED；读模型归 R0.3-R1c |
| AC-27（`0001:592-613`） | 收口前提 | `AC-50-FIX-01` 未关闭前 AC-27 不得 AC-VERIFIED（`0001:874`） |
| AC-62（v1.11 `0002:326`） | 约束 | REVIEW 不得自动释放；只有 AC-62/D-24 授权放弃才有 `FAILED_PERMANENT`/释放；释放不恢复 permit、不关闭组 |
| AC-48（`0002:200-206`） | 非回归 | permit `CONSUMED` 不同 `shipment_no` 仍拒绝；取消后不得复活 |
| AC-51/AC-65（v1.11） | 非回归 | TCC 共用映射；REVIEW → `MANUAL_HOLD`，不得进入 `ADMITTED/PAYING`；不得早失败 |
| AC-67/AC-68（v1.11） | 非回归 | 零元错误不写 DONE、不持久化 REVIEW；合法分支不变；零元无 Payment |
| AC-27-NOWB-01 / AC-27-LIVE-01（`0001:863-864`） | 人工队列/不收敛 | 终态见 §8；term① re-arm（§8.3）；term② 人工放弃；LIVE 重放仍冲突 → NOWB；均不自动关闭/退款 |

---

## 15. 实施边界与停止条件

### 15.1 施工图（最小 diff，不写代码）

1. **Fulfillment**：`cancel_if_created.go` 的 `sql.ErrNoRows` 分支返回 `<NO_WAYBILL>`（不 commit、不建行）；其余分支不变；注释。
2. **Order 金额分类（Tx1 内）**：`AdmitRefund` 在 `GetIntent`/远程前先读 existing；Tx1 计算 `groupRemaining` 后按 §3.4 三分：`PARTIAL` → rollback 零写入；`REVIEW` → 写 `result=REVIEW_REQUIRED,state=PENDING_REMOTE` + 1 条 `RESERVED` commitment（无终态 outbox）；`POST` → 写 POST + commitment + 终态 outbox；`CONSUMED+整组` → Tx1 占位后走 `finishRemoteAdmission`。
3. **Order 整组映射**：`CONSUMED+整组` 若 Order 组投影已 `SHIPPED/RECEIVED` → 直接 POST（**不探测**）；`finishRemoteAdmission` 需接收 `batch_id`（来自 `AdmitRefundRequest`，可为空）以便 Tx2 同事务持批；其余映射： `CANCELLED→PRE`、`ALREADY_SHIPPED/RECEIVED→POST`、`<NO_WAYBILL>→Tx2 REVIEW_REQUIRED + 告警 (+TCC MANUAL_HOLD)`、瞬态/未知→`Unavailable`；Tx2 按 A8 `state` CAS；仅 PRE 触发既有释放。
4. **Order 重放**：`AdmitRefund` existing 分类（terminal / review / pending / unknown）：terminal→reply；review→同一错误、不探测/不读 Payment；pending→继续；unknown→fail-closed；`finishRemoteAdmission` 入口也防御 REVIEW。
5. **Order 零元**：`decideZeroRefundResult` 的 `CONSUMED + <NO_WAYBILL>` → 冲突；不进入 `writeZeroRefundTx`；不持久化 REVIEW；组投影 SHIPPED/RECEIVED → POST+DONE（AC-67）。
6. **TCC**：`ensureTccRemoteAdmissionTx`/`ensureTccApplicationTx` 接受 `REVIEW_REQUIRED`；`writeTccBatchGroupsTx` 对 `MANUAL_HOLD` 短路返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`；`admitTccBatch` 识别复核并置 `MANUAL_HOLD`+`last_error`；Tx2 同事务置批（§3.2）。
7. **proto**：只改 `CancelIfCreated`/`CancelIfCreatedReply` 注释；`make proto`；`make proto-guard` 通过。
8. **AC-72**：`internal/platform/metrics` 加 2 gauge；`order/worker.go` 既有 tick 只读刷新；`deploy` 加 1 条告警规则；不新增 worker。
9. **测试**：§10；真实 MySQL + `-race` + 日志/NATS；不含 ListShipments 用例。
10. **迁移**：无。若实现需要迁移 → 停止。

### 15.2 停止条件（命中即停手，回 Architect/Lead）

1. **SPEC-0002 v1.11 未 approved，或 PO 改名/改表示，或人类未 `ADR_APPROVED`** → **不得开工**。
2. 需要新增 proto 字段/RPC/事件字段、迁移、permit 状态、新的 `state` 值、额外的持久裁决值 → 超出本 amendment。
3. 需要修改 AfterSales/Payment/Inventory/Deploy 其他文件、`docs/specs/**`、`tcc.go`/`tcc_payment.go`/`tcc_precheck.go`/`tcc_reconcile.go`、`consumer.go`/`admission_consumer.go`/`settlement.go`/`ports.go` 语义 → 越界，回 Lead 拆范围。
4. 为实现部分金额/整组复核而引入任何 `ListShipments`/`GetShipmentByOrder`/事务外预读，或恢复“取消副作用分类” → **停止**（route A/D-31）。
5. 出现任何 `<NO_WAYBILL>`（整组）写 PRE/POST、或 REVIEW 被当准入成功/关闭/打款/释放 → 停止，Reviewer FAIL。
6. 部分金额触发 `CancelIfCreated`、先写 commitment 再写 REVIEW、或静默放行 → 停止。
7. 普通 replay 对 REVIEW 重新探测、或自动 re-arm、或自动清除 REVIEW → 停止。
8. TCC 把 REVIEW 判为非法/早失败/当 PENDING 重探、或批次未置 `MANUAL_HOLD` → 停止。
9. 用迟到 `order.paid` placeholder 当人工证据、或普通 consumer 建行被写成“人工证据” → 停止（W9）。
10. 测试无法区分 fresh/preseed/REVIEW、或无法证明 Tx1 占位/终态/复核三分 → 证据不成立，停止。
11. 任何 `AdmitRefund` 对非终态 existing 返回 nil error（可能被判成功）→ 停止，Reviewer FAIL。
12. 未做 AC-72 metrics/告警即声称 AC-72 完成，或未做 BFF/入口即声称“用户可见完成” → 停止。

### 15.3 交付证据（实施时，不属本文件）

- 精确 base/head；`go test`/`-race`/真实 MySQL 日志；`【测试范围】`/`【未覆盖范围】`；受影响 AC 行由 Reviewer 更新；O1/O2/O3/O4b/O4c/O4d/O7 的“REVIEW/PARTIAL、0 终态、0 退款命令、0 释放”断言；F1“无行不建行”；O1b 经 `TccCancelMarkPaid` + `MANUAL_HOLD`；O11b/O11c；O16 re-arm；O18 metrics；proto 生成 diff 仅注释；无迁移声明。
- **不**声称 AC-VERIFIED/Release/merge；`AC-27` 在 `AC-50-FIX-01`/`AC-27-NOWB-01`/`AC-27-LIVE-01` 关闭前保持 AC-REF；AC-69..72 在 Reviewer 绑定执行证据前保持 AC-REF。

---

## 16. Consequences 与后续 owner

| 对象 | 动作 |
|---|---|
| **PO** | APPROVED SPEC-0002 v1.11（当前 in-review）；确认 D-2（普通售后是否需 `refund_applications` 镜像行）；同步 SPEC-0001 `0001:645` 注（D-3/2026-12-31） |
| **QC（round 4）** | 复核 0.4：R3-B1..B4 是否闭合；`REVIEW_REQUIRED` 取值/持久化/幂等；TCC `MANUAL_HOLD`；partial 三分法；A8/A10/A11；§10 矩阵；AC-72 后端/入口边界 |
| **人类（Director）** | 批准 L2 ADR（`ADR_APPROVED`）；保留 merge/Release |
| **Lead** | 按 `0001:879` 另拆实施 `change_id`；建 Plan/Tasks + AC 证据矩阵；登记 `LEGACY-SHIPPED-PRE-01`/`TCC-PENDING-RESULT-01`/`TCC-LAST-ERROR-01`/`INV-44-MULTI-01`/`ORDER-PAID-PLACEHOLDER-01`/`AC50-OUTCOME-SPEC-SYNC`/`REVIEW-REQUIRED-ENTRY-01`/`ADMISSION-RESULT-CHECK-01`/T-1 的 owner/expiry/stop |
| **Developer** | 在独立 `change_id` 下实现 §15.1；跑 §10；提交 proto 生成物、metrics/告警、测试；不 commit 本文件 |
| **Fulfillment owner** | 提供 `<NO_WAYBILL>` 与**既有/曾存在但丢失**的 S6 证据；配合 AC-27-NOWB-01 终态①；不得新建行 |
| **ops** | 用 §8 查询/日志发现队列；执行存量 PRE 审计；按 §8.3 授权/审计 re-arm；告警规则复核；SLA/升级 |
| **Reviewer** | 按精确 base/head 校验：无预读；REVIEW/PARTIAL 不写 PRE/POST；部分不调 `CancelIfCreated`；0 行语义；replay 不探测；`AdmitRefund` 非终态必错误；TCC 不早失败；测试真实执行 |
| **Architect** | Rev 2.4 errata 合并本 addendum（需人类批准）；新缺口升 Revision |

---

## 17. 变更记录

| Revision | 日期 | 变更 |
|---|---|---|
| 0.1 | 2026-11-26 | 初稿：R-1…R-8；`NO_SHIPMENT`、映射、不变量、窗口、矩阵、登记面 |
| 0.2 | 2026-11-26 | 按 QC R1 整改：B-1/B-2/B-3；M-1…M-13；R-2a 部分金额预读；W9；setup；T-1…T-7 |
| 0.3 | 2026-11-26 | 按人类裁决 A 与 QC R2：删除 R-2a 与全部 `ListShipments`；整组 `NO_SHIPMENT` 持久化 `result=CONFLICT`；R-2b 部分金额无预读；W9 真实 placeholder；审计补无 group 行；A8 四分支；§1.3 per-item stop；re-arm runbook |
| **0.4** | 2026-11-26 | **按 SPEC-0002 v1.11（in-review，D-30/D-31/AC-69..72）重写**：持久裁决定为 `REVIEW_REQUIRED`（`result` 承载、`state` 不新增值、无迁移；错误码仍 `REFUND_ADMISSION_CONFLICT`）；删除 0.3 的 `result=CONFLICT` 越权表述；partial 改为 PO 三分法（投影离仓→POST、未离仓→PARTIAL 写前拒绝零写入、不可判定→REVIEW + 冻结额度 commitment）；TCC 改 R-9（接受合法裁决、`MANUAL_HOLD`、`refund_batches.last_error`、`TCC_BATCH_REQUIRES_MANUAL_HANDLING`，`tcc_settlement.go`/`tcc_admission.go` 进 `allowed_paths`）；新增 R-10/AC-72（metrics gauge + 既有 tick + 告警规则；用户可见/入口另立 `REVIEW-REQUIRED-ENTRY-01`）；A2/A8/A9/A10/A11 强化；§8.3 re-arm 过渡表；§10 O1/O1b/O4b/O4c/O4d/O18 重写；§12 补 D-2/普通售后镜像行/回滚/入口假设；§15.2 停止条件补 TCC/复核/入口。未写代码、未改 SPEC/Rev 2.3 正文、未 commit |

**Actor**：`arch-ac50`（System Architect）
**Reviewed-by**：待 QC round 4
**Status**：**DRAFT — pending PO/QC**
