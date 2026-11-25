# ADR-0007 Amendment 1 Addendum — AC-50-FIX-01：`CancelIfCreated` 的“无运单行”additive 结果与 Order 映射

| 项 | 值 |
|---|---|
| Status | **DRAFT — pending QC round 2**（未批准；不构成实现依据；不代表 AC-VERIFIED / merge / Release PASS） |
| Revision | **0.2**（相对 0.1 关闭 QC `QC-PAY-SETTLE-01-AC50-NOWB.md` 的 3 BLOCKER + 13 MAJOR；逐条见 §1.1） |
| Change ID | `PAY-SETTLE-01`（子范围 `AC-50-FIX-01`；实施须由 Lead 另拆独立 `change_id`，见 §0.5） |
| Tier | **L2**（跨服务契约语义、资金/履约准入、既有 RPC 响应语义；按 `docs/collab/CONVENTIONS.md:90` 任一命中即 L2） |
| Tier-Rationale | 修改既有 RPC `Fulfillment.CancelIfCreated` 的响应语义（新增可区分结果值），并改变 Order 准入结果映射（`REFUND_ADMISSION_CONFLICT` vs `PRE_SHIP_REVOKED`），直接影响资金打款/释放门禁与 AC-27A 关闭谓词；跨 Fulfillment/Order 两服务。即使代码增量小，也不得降级 |
| QC 输入 | `docs/collab/qc-reports/QC-PAY-SETTLE-01-AC50-NOWB.md`（BLOCKED；base `da0cdd1`；被审 0.1 blob `53e799e`） |
| Spec-Ref | SPEC-0001 **v1.5**：AC-27（`0001:592-613`）、AC-27A（`0001:615-645`）、AC-27A 注（`0001:645`）、§16.2 `AC-50-FIX-01`（`0001:862`）、`AC-27-NOWB-01`（`0001:863`）、`AC-27-LIVE-01`（`0001:864`）、§16.3 收口前提（`0001:874`）与独立 `change_id` 要求（`0001:879`）、§16 V15-3（`0001:827`）；SPEC-0002 **v1.10**：AC-50（`0002:213-231`，`CONSUMED+无运单`=`0002:223`、部分金额=`0002:217`）、AC-48（`0002:200-206`）、AC-62（`0002:308-323`）、INV-36（`0002:120`）、INV-42（`0002:126`）、INV-43（`0002:127`）、INV-44（`0002:128`）、INV-45（`0002:129`）、INV-50（`0002:134`）、错误表（`0002:517`） |
| Base（仓库） | `da0cdd15d065a503ffa1e6d545e7381f6766737d`（`docs(lead): Phase 0-R checkpoint after Reviewer verdicts`） |
| Base（契约基线） | SPEC-0001 v1.5（approved 2026-11-26，`4215b3c`）、SPEC-0002 v1.10（approved 2026-11-25）、ADR-0007 **Rev 2.3**（accepted 2026-11-25，`docs/adr/0007-payment-settlement-admission.md`） |
| Code head（裁决针对的实现证据） | `c38c54f`（R0.4 实现；`git diff c38c54f -- internal/services/order/refund_admission.go internal/services/fulfillment/cancel_if_created.go api/proto/mall/v1/mall.proto` 为空，本裁决涉及的锚点文件在 base 未变）。**工作树存在其他 agent 的并发改动**（0022/0023 迁移、`r04_checks_integration_test.go`、`verify-r01-migrations.sh`、`review-tracking.md` 等；以 `git status` 为准），均不属于本裁决 |
| Doc head | 本文件由 **Lead 统一提交**；QC/Reviewer 复核时必须记录精确 SHA（本文件不自引 SHA） |
| Integration owner | **System Architect**（本 Addendum 作者/契约）；实现 owner = **Developer**（另拆 `change_id`）；S6 无运单证据 owner = **Fulfillment**；人工队列/告警 owner = **ops** |
| allowed_paths（本文件） | 仅 `docs/adr/0007-amd1-addendum-ac50-nowaybill.md` |
| allowed_paths（本裁决授权的最小实现，须在 Lead 另拆的 `change_id` 下） | `api/proto/mall/v1/mall.proto`（**仅** `CancelIfCreated` 的 RPC/`CancelIfCreatedReply.status` 注释）、`api/gen/mall/v1/mall.pb.go`、`api/gen/mall/v1/mall_grpc.pb.go`（**仅** `make proto` 生成的注释同步，禁止手改；RPC 注释会落在 grpc 生成物）、`internal/services/fulfillment/cancel_if_created.go`、`internal/services/order/refund_admission.go`（`AdmitRefund` 的部分金额前置门禁 + `finishRemoteAdmission` + 零元 `decideZeroRefundResult`/`admitZeroRefund` + Tx2 状态 CAS）、`internal/services/fulfillment/cancel_if_created_test.go`、`internal/services/fulfillment/admission_consumer_nats_integration_test.go`（**仅测试**，不新增生产 seam）、`internal/services/order/refund_admission_integration_test.go`、`internal/services/order/tcc_admission_integration_test.go`（**仅测试**） |
| forbidden_paths | `migrations/**`、`api/gen/**` 手改、`api/proto` 的新字段/新 RPC/枚举变更、`internal/platform/**`、`deploy/**`、`frontend/**`、`internal/services/{inventory,payment,aftersales}/**`、`internal/services/order/tcc_*.go`（**仅生产文件，`tcc_*_test.go` 除外**；TCC 复用同一映射，禁止复制第二套状态判断；测试文件见 allowed_paths）、`internal/services/fulfillment/admission_consumer.go`（生产消费者；现有 `NO_SHIPMENT` 已是 no-op，禁止改语义）、`docs/specs/**`、`docs/collab/**`（含 `ac-coverage.tsv`）、`docs/lead/**`、`docs/implementation/**`、`docs/adr/0007-payment-settlement-admission.md`（已 accepted Rev 2.3 正文）、`docs/adr/0007-payment-settlement-admission-amd1.md`、`scripts/**`、`Makefile` |
| baseline_impact | **涉及**：AC-50 的 `CONSUMED+无运单` 写路径与部分金额前置、AC-27A 关闭谓词的 PRE 来源合法性、AC-27 的收口前提；跨 Fulfillment/Order 的既有 RPC 响应语义。**不涉及**：Payment 金额/退款实现、迁移/schema、事件 subject/payload 结构、permit 状态机、任何新 worker；非回归范围见 §14 |
| INV | INV-36、INV-42、INV-43、INV-44、INV-45、INV-50（逐条见 §14；无新增 INV） |
| Actor | `arch-ac50`（System Architect） |
| Reviewed-by | **待 QC round 2**（本文件不代签；DRAFT 期间不得合入受管代码，不得标 PASS） |
| 硬边界 | 不改 SPEC（SPEC-0001/0002 均归 PO）、不改 accepted ADR Rev 2.3 正文（用本 addendum，未来 Rev 2.4 errata 由 Architect 合并）、无新字段/新 RPC/新事件/新迁移/新状态字段/新 worker；不交付 T2/T3/T4.4；本文件未明确列出的变更一律不做 |

> 本文件是**设计裁决**，不是实现、不是验收、不是台账。代码/测试状态归 Developer/Reviewer；AC 状态归 Reviewer；人工队列终态归 SPEC-0001 §16.2。文中的“MUST/不得”是给 Developer 的施工约束，不免除 QC 与人类批准。

---

## 0. 阅读方式与规范边界

### 0.1 问题陈述（AC-50-FIX-01）与代码事实

SPEC-0001 v1.5 `0001:862` 登记：`CONSUMED+无运单` 写路径把“无运单行”映射为 `CANCELLED`/PRE，零元路径 `decideZeroRefundResult` 同样；ADR §5.3 的 `CancelIfCreatedReply` 没有“无运单”结果。不变量是：**`PRE_SHIP_REVOKED` 只能由 AC-50 合法未离仓分支写入；`CONSUMED+无运单` 必须返回 `REFUND_ADMISSION_CONFLICT`、不得写 PRE**。

| # | 事实 | 证据 |
|---|---|---|
| 1 | Fulfillment 查不到运单行时返回 `CANCELLED`（注释“No parcel exists: nothing to cancel”） | `internal/services/fulfillment/cancel_if_created.go:51-55`（`:54`） |
| 2 | Order 正金额/TCC/legacy 共用的 `finishRemoteAdmission` 把 `CANCELLED` 一律映射为 PRE，并在 Tx2 写回 `result=PRE_SHIP_REVOKED` | `internal/services/order/refund_admission.go:566-608`（`:576-584`、`:590-597`） |
| 3 | 零元 `decideZeroRefundResult` 同样把 `CANCELLED` 映射为 PRE | `refund_admission.go:366-396`（`:384-391`） |
| 4 | **正金额 `CONSUMED` 的 Tx1 在远程探测前已写**：admission（`result=POST,state=PENDING_REMOTE`）、`order_refund_commitments(RESERVED)`、outbox `order.refund_admitted` | `refund_admission.go:253-283`（admission `:254-265`、commitment `:266-272`、outbox `:281`、payload `:610-619`） |
| 5 | **Tx1 前已调用 `Payment.GetIntent`**（只读快照）取权威 captured/refunded | `refund_admission.go:168-175` |
| 6 | TCC Tx1 的 admission 写 `result=PENDING_REMOTE, state=PENDING_REMOTE`（与“result 只允许 PRE/POST”的 SPEC 注存在既有偏差） | `tcc_settlement.go:847-852`；SPEC-0001 `0001:645` |
| 7 | 同 `refund_id` 重放且 `state=PENDING_REMOTE` 时，Order 会再次进入 `finishRemoteAdmission`（重新探测） | `refund_admission.go:179-185`、`:203-211` |
| 8 | AC-27A 第 5/6 条：`CONSUMED` 只有 `result=PRE_SHIP_REVOKED` 才关闭；`state=PENDING_REMOTE`、结果缺失/不可核验 → 未关闭并告警 | SPEC-0001 v1.5 `0001:622-625`、注 `0001:645` |
| 9 | AC-50 的 `NONE→REVOKED` 分支明确“无运单/已取消时幂等成功”→PRE；`CONSUMED+CANCELLED` 也是合法 PRE | SPEC-0002 `0002:219`、`:222` |
| 10 | `order_refund_admissions` 无 CHECK 约束，`result/state` 为 `VARCHAR(32)`，默认 `PENDING_REMOTE` | `migrations/order/0016_settlement_admission.sql:7-19` |
| 11 | 每组至多一条运单行：`UNIQUE KEY (order_no, fulfillment_group_no)` | `migrations/fulfillment/0003_group_shipments.sql:6-9` |
| 12 | 除 Order 与 Fulfillment 自身的 `order.refund_admitted` 消费者外，没有其他 `CancelIfCreated` 调用方（BFF/前端不消费该 status） | 全仓库 grep；`internal/services/fulfillment/admission_consumer.go:43` |

**结论**：缺陷不在“取消”本身，而在**事实无法区分**：Fulfillment 把“无行”与“真实已取消行”压成同一个 `CANCELLED`，Order 又把它压成同一个 PRE。本 amendment 采用 QC B-2（`QC-PAY-SETTLE-01-R0.3-AC27.md:56-66`）的**第一种**修改路径（新增可区分的“无运单”结果），不采用“no-row 视为已取消 + 已知偏差”的容忍方案。

### 0.2 `CONSUMED+无行` 的可达性（M-12）

| 路径 | 是否可能产生 `CONSUMED+无行` | 证据 |
|---|---|---|
| 正常 `Ship` 路径 | **否**：`CreateShipment` 先插入运单行（`CREATED`），`Ship` 才调 `Order.AuthorizeShipment` 消费 permit | `fulfillment/service.go:154-166`、`:244`；`settlement.go:353-357` |
| 直接调用 `Order.AuthorizeShipment`（Order RPC） | **是**：Order 不校验运单行是否存在 | `api/proto/mall/v1/mall.proto:485-487` |
| 历史数据（0016/0003 之前、无 permit 行、行缺失） | **可能** | `migrations/order/0016_settlement_admission.sql`、`migrations/fulfillment/0003_group_shipments.sql` |
| 人工/数据修复、行被删除 | **可能** | ops；`[不确定]` 生产发生率需 ops 证据 |
| 部分金额 + `CONSUMED` | 由 R-2a 在 Tx1 前拒绝（`PARTIAL`），**不进入 conflict** | §3.4 |

> O1/O7 的 `CONSUMED+无行` 是**合成异常状态**，用于验证 fail-closed；不据此声称“已发生的 P0 缺陷”或频率。正常路径回归见 F8/O5。

### 0.3 指针澄清（brief 与 canonical 文件不一致处）

任务 brief 指向 `docs/adr/0007-payment-settlement-admission-amd1.md` 的 §5.2/§5.3/§5.7。该文件是 Rev 2.3 的**来源文件**，但其 §5.3（`:375-397`）是 `legacy_payment_resolution` 的并发与事务规则；`Fulfillment.CancelIfCreated` 的规范正文在**合并后的 accepted Rev 2.3** 文件 `docs/adr/0007-payment-settlement-admission.md` §5.3（`:611-623`，合并映射见 `:56`）。本 amendment 的修改对象是**后者**；文件名沿用 Lead 指定的 `0007-amd1-addendum-ac50-nowaybill.md`。

### 0.4 与已批准契约的关系

- 本文件**additive**：不删除、不重写 Rev 2.3 任何条款；只给 §5.3 增加一个 `status` 值及其语义，并明确 §5.2.2/§5.2.4 的映射、Tx1 已写事实的处置与持久化边界。
- 本文件**不批准**任何实现；`ADR_APPROVED`（人类）之前 Developer 不得开工（宪法 Workflow 2）。
- 本文件不改变事实所有权、不改变 permit 状态机、不改变退款额度、不改变 INV-45 的不可逆性。

### 0.5 `change_id` 边界（防越权）

SPEC-0001 v1.5 §16.3 `0001:879` 明确：AC-50 修正“需 Lead 拆独立 `change_id` 走 L2（跨 Fulfillment/Order，超 R0.3-R1c `allowed_paths`）”。因此：

- 本 Addendum（契约）挂 `PAY-SETTLE-01`（Lead 指定）；
- **实施**必须由 Lead 另拆独立 `change_id`，并在其实施 Brief 中逐文件落 `allowed_paths`；不得把本文件的 `allowed_paths` 当作 R0.3-R1c 的范围扩展，也不得在同一提交里顺手改 AC-27A 读模型（那是 R0.3-R1c 的独立实施项）。

---

## 1. 裁决摘要

| # | 裁决 | 一句话 | 归属 |
|---|---|---|---|
| **R-1** | `CancelIfCreatedReply.status` 增加 **additive** 值 `NO_SHIPMENT`（string，无新字段/无新 RPC）；Fulfillment 对“该组没有任何运单行”返回它；`CANCELLED` 只表示真实运单已取消 | 先分开事实 | Architect（契约）/ Fulfillment（实现） |
| **R-2** | Order `finishRemoteAdmission`：`NO_SHIPMENT` → `REFUND_ADMISSION_CONFLICT` + 告警，**不新增终态写入、不写 PRE、不开 Tx2 终态更新、不产生退款命令、不释放**；`CANCELLED` → PRE；`ALREADY_SHIPPED/RECEIVED` → POST；`TRANSIENT_ERROR`/缺失/未知 → 不可判定，保持可重试 | 映射 fail-closed | Order（实现） |
| **R-2a** | **部分金额前置门禁**：`CONSUMED` 且 `amount < 组剩余` 时，在 Tx1 前用只读 `ListShipments` 判定是否已离仓；未离仓 → `PARTIAL_REFUND_REQUIRES_FULL_GROUP`（**零 DB 写入**；`GetIntent` 只读已发生、允许）；已离仓 → 继续 Tx1+探测（探测仍权威）；不可判定 → fail-closed 可重试 | 不把部分退款洗成 PRE，也不留 Tx1 残留 | Order（实现） |
| **R-3** | 零元 `decideZeroRefundResult` 使用**同一映射语义**：`NO_SHIPMENT` → conflict + 告警，且在写任何 admission/`DONE` 之前返回；`CANCELLED` → PRE；`ALREADY_SHIPPED/RECEIVED` → POST；瞬态/未知 → 可重试 | 零元不得例外 | Order（实现） |
| **R-4** | 正金额/TCC/legacy/零元共用**唯一映射表**；TCC 已经过 `AdmitRefund`→`finishRemoteAdmission`，禁止在 `tcc_*.go` 复制第二套状态判断 | 拒绝 if-else 扩散 | Order（实现）/ Reviewer |
| **R-5** | 持久化语义：admission **终态当且仅当 `state ∈ {PRE,POST}` 且 `result ∈ {PRE,POST}` 且 `state=result`**；`PENDING_REMOTE` 不是结果、不构成关闭、不构成退款授权；Tx1 的写入按 §7.4 处置，**不声称“无写入”** | 单一事实载体 + 占位语义 | Order（实现）/ Reviewer |
| **R-6** | 幂等/并发/崩溃：同 `refund_id` 重放重新探测并**确定性**回到同一冲突；`PENDING_REMOTE` 不提前关闭、不自动放行；Tx2 以 `state=PENDING_REMOTE` CAS 唯一化终态写入；崩溃窗口不得产生 PRE/POST 或重复事件/释放 | 可重放、无脏写 | Order/Fulfillment |
| **R-7** | `NO_SHIPMENT` 冲突进入 **AC-27-NOWB-01** 人工队列（终态/owner/expiry 见 SPEC-0001 §16.2 `0001:863`），组保持未关闭；不得自动关闭/自动退款/恢复 permit；TCC 有独立发现口径 | 人工终态 | ops + Developer + Fulfillment |
| **R-8** | 登记：**必须**更新 `mall.proto` 的注释（并 `make proto` 生成物同步）；`docs/contracts.md` **不需要**改；错误码**不新增**，继续用 `REFUND_ADMISSION_CONFLICT` / `PARTIAL_REFUND_REQUIRES_FULL_GROUP`（`apperr.Failed`→`FAILED_PRECONDITION`） | 最小登记面 | Architect / Developer / Reviewer |

**结论级别**：R-1…R-8 均可在**不改 SPEC、不改 Rev 2.3 正文、无迁移、无新字段/RPC/事件/worker** 的前提下落地。无 `[ARCHITECTURAL_FLAW]`；但本 revision 显式登记**不在本 amendment 修复范围**的既有缺口（M-1 legacy 已发货、M-3 人工释放实现、M-4 TCC result 清理、M-5 存量 PRE 审计、M-7 TCC last_error 写入、T-3 多组释放），见 §1.1 与 §12，owner/expiry 均在 2026-12-31 前。

### 1.1 QC round 1 处置索引（B-1…B-3、M-1…M-13、T-1…T-7）

| QC 项 | 结论 | 本 revision 处置位置 |
|---|---|---|
| **B-1** Tx1 已写 admission/commitment/outbox | **FIXED（文本/测试口径）** | §6 A7、§7.4 处置协议、§10 O1/O3/O9 断言“Tx1 写入已存在、conflict 不新增终态” |
| **B-2** `GetIntent` 已调 / “Payment 0”绝对措辞 | **FIXED** | §3.1、§4.1、§6 A4、§10 O1/O6/O7/O9 改为 `Refund/RefundBatch` 0，`GetIntent` 允许 |
| **B-3** W6“补出运单行” | **FIXED** | §7.2 W6 改为“仅修复**既有/曾存在但丢失**的真实 S6 证据”；§2.3 #1/§11 明确禁止伪造行；§10 复核判据 |
| **M-1** legacy `NONE`+已发货仍可写 PRE | **DEFERRED**（既有缺口） | §6 A3 收窄；§8 存量审计；§12 `LEGACY-SHIPPED-PRE-01`；owner Architect/PO，expiry 2026-12-31；O10b 标 DEFERRED |
| **M-2** 部分金额丢失 `PARTIAL` 前置 | **FIXED** | §3.4 R-2a 前置门禁；§10 O4b/O4c；§15 施工图 |
| **M-3** A4 与 AC-62 人工释放冲突 | **FIXED（规则）** / 实现 **DEFERRED** | §6 A4 加 AC-62/D-24 唯一例外；§8 终态②；§10 O15 标 DEFERRED（T3 未实现释放写入） |
| **M-4** TCC `result=PENDING_REMOTE` | **FIXED（读者口径）** / 清理 **DEFERRED** | §6 A2 要求 state+result 双条件；§6.2；§12 `TCC-PENDING-RESULT-01`；§10 O13b |
| **M-5** 存量错误 PRE 审计 | **FIXED（登记协议）** | §8 存量审计查询/owner/expiry；AC-27A 关闭门禁 |
| **M-6** TCC 测试路径被 forbidden | **FIXED** | 头部 `allowed_paths` 加 `tcc_admission_integration_test.go`（仅测试）；`forbidden_paths` 明确生产 `tcc_*.go` |
| **M-7** TCC 队列发现口径 | **FIXED（查询/日志）** / `last_error` 写入 **DEFERRED** | §8 TCC 发现查询；§12 `TCC-LAST-ERROR-01` |
| **M-8** 消息重复/乱序窗口 | **FIXED** | §7.2 W9；§10 F7/O14；§2.2 consumer 语义 |
| **M-9** Tx2 无 state/version 守卫 | **FIXED** | §6 A8；§15 施工图；§10 O11b |
| **M-10** POST commitment“同事务”对正金额不成立 | **FIXED** | §3.1 分行说明；§10 O5/O5b |
| **M-11** `admission_consumer` 对 `NO_SHIPMENT` 未明确 | **FIXED** | §2.2/§3.4；§10 F7/O14 |
| **M-12** `CONSUMED+无行` 可达性 | **FIXED（说明）** | §0.2；§10 O1/O7 标“合成异常”，F8 正常路径回归 |
| **M-13** 零元/TCC “Payment 0”与 setup 未区分 | **FIXED** | §10 setup 列；O6/O6b/O7 |
| **T-1** SPEC-0002 错误表漂移 | **`[TODO]` → PO** | §9；owner PO，expiry 由 Lead 登记 2026-12-31 |
| **T-2** proto 工具版本/diff 证据 | **FIXED（要求）** | §9；实施 Brief 固定版本 + 仅注释 diff |
| **T-3** O4 多组释放不成立 | **FIXED（限定单组）** / 多组 **DEFERRED** | §10 O4；§14 `INV-44-MULTI-01` |
| **T-4** 零元 transient 未单列 | **FIXED** | §10 O7b |
| **T-5** `shipment_no` 非空语义 | **FIXED** | §2.1；§10 F1b；§12.3 |
| **T-6** `reason/manual_queue` 键稳定性 | **FIXED（固定 key/大小写）** | §8/§9 |
| **T-7** Tx2 行锁/锁顺序 | **FIXED（选状态 CAS）** | §6 A8；§15；若改用行锁则须写锁顺序（本裁决不选） |

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
- 探测范围：`order_no` 必填；`group_no` 为空时既有实现默认 `G1`（不改）；`shipment_no` 非空时范围收窄到 `(order_no, group_no, shipment_no)`。Order 的调用点必须继续传空 `shipment_no`（现状），使 `NO_SHIPMENT` 表示**整组无行**；T-5 见 §10 F1b。
- `migrations/fulfillment/0003_group_shipments.sql:6-9` 的 `UNIQUE (order_no, fulfillment_group_no)` 保证每组至多一行，故 `NO_SHIPMENT` 无“多行歧义”。

### 2.2 返回矩阵（Fulfillment + `order.refund_admitted` consumer，M-11）

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

1. 不得为“无行”补建一行 `CANCELLED`/`CREATED` 运单（会伪造 S6 事实；B-3）。人工修复只能提供**既有/曾存在但丢失**的真实运单证据（含恢复审计），不得为从未存在的运单造行。
2. 不得因 `NO_SHIPMENT` 写 permit、admission、commitment、库存或任何 outbox 事件（Fulfillment 不是这些事实的 owner）。
3. 不得把 `NO_SHIPMENT` 复用为 `TRANSIENT_ERROR` 或 `CANCELLED` 的别名；失败语义必须可区分。
4. 不得新增 `warehouse_id` 缺省、`G1` 缺省或新的请求字段；`group_no` 的既有 `G1` 缺省行为保留，但调用方必须显式传组号（现状）。

### 2.4 兼容与发布顺序

- `NO_SHIPMENT` 是**新增值**。旧 Order 客户端对未知值走 `default` → `apperr.Unavailable`（可重试，见 `refund_admission.go:582-584`、`:389-391`），**不会**写 PRE。
- 若 Fulfillment 先上线：旧 Order 只会重试，状态停在 `PENDING_REMOTE`，fail-closed，可接受（告警噪声，不资损、不误关闭）。
- 若 Order 先上线：在 Fulfillment 仍返回 `CANCELLED` 期间，缺陷保持现状（风险不变，不得宣称修复完成）。
- 按 `CONVENTIONS.md:241`（已批准契约的实现允许一个原子行为提交包含 proto、生成物、服务代码与测试），**建议同一原子行为提交**同时改 Fulfillment 与 Order；若必须分批，按上一条 fail-closed 顺序，并在实施 Brief 记录批次与回滚。

---

## 3. R-2：Order 映射（正金额 / TCC / legacy）

### 3.1 唯一映射表（`finishRemoteAdmission`）

| `CancelIfCreated` 返回 | Order 准入结论 | 持久化动作（**含 Tx1 已写事实**） | 退款/释放 |
|---|---|---|---|
| `CANCELLED` | `PRE_SHIP_REVOKED` | Tx2（状态 CAS）写 `result=PRE,state=PRE` + outbox；**正金额/TCC 的 Tx1 已写恰好 1 条 `RESERVED` commitment**（正金额 `:266-272`；TCC `tcc_settlement.go:898`），Tx2 **不重复写**；release 按 INV-44（单组） | 允许（AC-50 合法 PRE）；**不得** `Payment.Refund/RefundBatch` 之外的写（由调用方在准入后决定） |
| **`NO_SHIPMENT`** | **`REFUND_ADMISSION_CONFLICT`（错误返回）** | **不开 Tx2 终态更新、不改 admission、不写 release**；Tx1 已写的 admission/commitment **保留**（普通正金额另有占位 outbox；TCC 无），处置见 §7.4 | **禁止** `Payment.Refund`/`RefundBatch`、禁止 release/commitment release；`Payment.GetIntent` 已在 Tx1 前只读调用（允许） |
| `ALREADY_SHIPPED` / `RECEIVED` | `POST_SHIP_ALLOWED` | Tx2（状态 CAS）写 `result=POST,state=POST` + outbox；**正金额路径的 Tx1 已有 1 条 `RESERVED` commitment，Tx2 不重复写**；**TCC 已离仓组**由 `recordTccPostAdmission` 在同一事务写 POST admission + commitment（既有，`tcc_admission.go:214`） | 允许（已离仓） |
| `TRANSIENT_ERROR` | **不可判定** | 不动；保持 `PENDING_REMOTE` | 禁止；可重试（`apperr.Unavailable`） |
| 空值 / 未知值 | **不可判定** | 不动；保持 `PENDING_REMOTE` | 禁止；可重试（`apperr.Unavailable`） |

- `NO_SHIPMENT` 的判定与返回必须在**打开 Tx2 之前**完成；不得先写 PRE 再回滚“修正”，也不得把 conflict 写进 `result`。
- 错误对象：`apperr.Failed(...)`（→ gRPC `FAILED_PRECONDITION`），message 含 `REFUND_ADMISSION_CONFLICT` 与 `CONSUMED permit without shipment`；`WithDetails` 固定 key（大小写固定）：`refund_id/order_no/group_no/permit_state="CONSUMED"/probe_status="NO_SHIPMENT"/reason="CONSUMED_WITHOUT_SHIPMENT"/manual_queue="AC-27-NOWB-01"`。不新增错误码、不改错误码表。
- **Payment 口径（B-2）**：本 amendment 只禁止 **`Payment.Refund`/`RefundBatch`**（退款命令/资金写）；`Payment.GetIntent` 是 Tx1 前的只读快照（`refund_admission.go:168-175`），**允许且必要**。不得把 `GetIntent` 写成违规，也不得声称“Payment 调用 0”。
- 正金额 `CONSUMED` 的 Tx1 占位 `result=POST`（`refund_admission.go:254-265`）：**本 amendment 不要求改它**；由 A2/A7 吸收（`state=PENDING_REMOTE` 不是结果；占位事件不是准入事实）。若 QC 认为占位不可接受，应作为**独立 L2 清理**另行裁决，不得夹带进 AC-50-FIX-01。

### 3.2 为什么 `NO_SHIPMENT` 不与 AC-50 的 `NONE→REVOKED` 冲突

- AC-50 `0002:219`：许可 `NONE` 且运单为**无**/`CREATED`/`CANCELLED` → 置 `REVOKED` → `PRE_SHIP_REVOKED`，并通知 Fulfillment 取消（无运单时幂等成功）。
- 因此“无运单”本身**不是**异常；异常是 **`permit=CONSUMED` 且无运单**（`0002:223`）。
- 本 amendment 只改**远程探测调用点**的映射：正金额路径只在 `PENDING_REMOTE`（由 `CONSUMED` 分支写入）时调 `finishRemoteAdmission`；零元路径只在 `permitConsumed` 时调用 `CancelIfCreated`（`refund_admission.go:374-391`）。`NONE/REVOKED` 分支**不得**调用探测，也不得因为本修复被改成“无运单 → conflict”。
- 任何把“无运单一律 conflict”的实现都是**误读**，会破坏 AC-50 `NONE` 分支与 legacy G1 初始化，Reviewer 应直接 FAIL。

### 3.3 legacy / TCC 的归属（不新增映射点）

| 来源 | 现有路径 | 本裁决要求 |
|---|---|---|
| 普通售后（`source=aftersale`） | AfterSales worker → `Order.AdmitRefund` → `finishRemoteAdmission` | 同一映射，无例外 |
| SYSTEM 缺货（`source=system_shortage`） | 同上 | 同一映射，无例外 |
| TCC 补偿（`source=tcc`） | `tcc_admission.go:admitTccApplication` → `Order.AdmitRefund` → `finishRemoteAdmission` | 同一映射；**禁止**在 `tcc_*.go` 再写一份 `CANCELLED→PRE` 的判断；Tx1 的 `result=PENDING_REMOTE` 是既有偏差（§6.2），读者必须看 `state` |
| legacy（历史单/无许可行 G1） | `ensurePermitTx` 初始化后走同一 `AdmitRefund`；legacy 支付解析后的补偿同路径 | 同一映射；legacy 的“无许可行”是 `NONE`（合法 PRE），不是 `CONSUMED+无运单`（conflict）；legacy 已发货缺口见 M-1/§12 |
| 零元（`source=system_zero`） | `admitZeroRefund` → `decideZeroRefundResult` | 同一映射语义（§4） |

### 3.4 R-2a：部分金额前置门禁（M-2）

**问题**：`CONSUMED` 分支在 Tx1 跳过部分金额校验（`refund_admission.go:249-252` 只对 `permit != CONSUMED` 生效）。若对 `CONSUMED` 未离仓组请求部分金额，当前会走到探测：`CANCELLED` → PRE（违反 AC-50 `0002:217`/D-19），`NO_SHIPMENT` → conflict（错误码应为 `PARTIAL`）。V15-3 明确“部分金额先按 AC-50 返回 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`”。

**裁决（Tx1 前门禁，零 DB 写入；`GetIntent` 只读已发生、允许）**：

1. 仅当 `permit=CONSUMED` **且** `amount < 组剩余`（同一口径：`groupPaid + groupFreight − Σ(RESERVED/SETTLED commitment)`）时触发。
2. 在 Tx1 前调用只读 `Fulfillment.ListShipments(order_no)`（page_size=100，`total<=100`），按 `(order_no, group_no)` 建 map；离仓事实 = `SHIPPED/RECEIVED` 或 `EXCEPTION && shipped_at>0`（与 ADR §5.2.1/T1.2-R-2 同口径）。
3. **未离仓**（无该组行 / `CREATED`） → 返回 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`（`apperr.Failed`，message 含 canonical token + details `{"reason":"PARTIAL_REFUND_REQUIRES_FULL_GROUP","group_remaining_cent":...}`）；**不写 admission/commitment/outbox、不探测、不释放、不产生退款命令**（`GetIntent` 只读已发生，允许）。
4. **已离仓** → 继续现有 Tx1+探测（探测仍权威；预期 `ALREADY_SHIPPED` → POST，部分金额合法，AC-50 `0002:226`）。
5. **不可判定**（`ListShipments` 超时/错误/`total>100`/分页不完整/同组多行/`EXCEPTION` 无 `shipped_at`） → `apperr.Unavailable` fail-closed，零 DB 写入，可重试（因此已离仓部分退款暂时依赖 Fulfillment 只读可用性；fail-closed、可重试）。
6. Tx1 在锁内**重新计算**组剩余；若观察到“提交前是整组、锁内变部分”且未走门禁，Tx1 **回滚并返回 PARTIAL**（或重跑门禁），不得带着部分金额进入探测。
7. **不改变 PRE/POST 判定权威**：门禁只用于“拒绝非法部分请求”，SUCCESS 后仍由 `CancelIfCreated` 决定 PRE/POST。ADR §5.2.2“CONSUMED 不依赖事务外预读运单状态”仍成立（预读不作为 PRE/POST 依据）。
8. 零元 `amount=0` 在零元单上等于组剩余 0（全组），不触发门禁；正金额零元来源在更早处已被拒绝。
9. **口径说明**：`组剩余` 沿用 `AdmitRefund:228-248` 的 `groupPaid + groupFreight − Σ(RESERVED/SETTLED)`；若运费责任只部分可退，`整组退款金额` 的精算属 INV-47/T3，本 amendment 不改变该口径。

**为什么门禁安全（并发时序）**：

```text
T0 Order ListShipments：组无行/ CREATED（未离仓） → 判定 partial 非法
T1 并发 CreateShipment 插入 CREATED 行 / Ship 推进 SHIPPED
T2 Order 直接返回 PARTIAL（不写任何行）
    → 用户/调用方看到“需整组退款”；若 T1 之后确已离仓，重试会看到 SHIPPED → 允许部分退款
```

最坏后果是“短暂拒绝一个原本合法的部分退款”（fail-closed、可重试），不会把未离仓组的部分退款写成 PRE，也不会留下 Tx1 残留。反向：T0 看到 SHIPPED、T1 之后行被删除 → Tx1+探测 `NO_SHIPMENT` → conflict（fail-closed），不会 PRE。

**为什么不在探测后判断（与 QC 建议的差异）**：探测前 Tx1 已提交 admission/commitment；若在探测后才拒绝部分金额，会留下无终态的 `RESERVED` 占用——自动释放违反 AC-62/INV-45，保留则把一次普通的部分退款请求变成人工队列债务。因此把拒绝前移到 Tx1 前；`ListShipments` 只服务于“拒绝”，不服务于 PRE/POST 判定（§3.4 第 7 条）。

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

- 零元是“先取消、后单事务写 DONE”（T1.4a/R-C、`0002` 零元短路）。`NO_SHIPMENT` 时远程没有副作用（无行可取消），因此“先探测、后事务”的顺序天然安全：直接返回 conflict，单事务不开启。
- **零元不调用 Payment**：`admitZeroRefund` 在 `refund_admission.go:154-156` 直接短路，`GetIntent`/`Refund` 均为 0；本路径的“Payment 0”是字面成立（与正金额/TCC 不同，B-2/M-13）。
- 零元冲突**不得**借用 `writeZeroRefundTx` 的 `PENDING_REMOTE` 分支（零元本来不写中间态）；重放时重新探测即可。
- 零元 `DONE` 的幂等语义不变：`refund_id` 重复且已有 DONE → 返回已持久化结果（`zeroRefundReply`），不得因本修复重复探测后改写。

### 4.2 禁止

1. 不得把零元 conflict 写成 `DONE(zero_amount)` 或任何 `refund_applications` 行。
2. 不得在 conflict 时释放预留/回库（INV-44/INV-50；`appendRefundReleaseTx` 不得被触发）。
3. 不得用“无运单”跳过 permit 分支：`NONE` 的零元仍按 AC-50 置 `REVOKED → PRE`。
4. 不得为零元部分金额触发 R-2a（零元单组剩余为 0，`amount=0` 即全组）。

---

## 5. R-4：单一映射权威（反 if-else 扩散）

- 两个映射点（`finishRemoteAdmission`、`decideZeroRefundResult`）必须使用**同一张**状态→结论表（一个包级 helper/常量表即可）。禁止在 AfterSales/TCC/正金额/零元各写一份 `switch status`。
- 映射函数必须是**穷尽 + fail-closed**：未列出的值（含未来新增值）→ 不可判定/可重试，绝不落到 PRE。
- 代码评审判据：搜索 `"CANCELLED"` 在 Order 中的出现点；除该唯一映射表外，任何新增 `CANCELLED→PRE` 分支即 FAIL（宪法 I.1“拒绝屎山雕花”）。

---

## 6. R-5：持久化与不变量（施工不变量）

| ID | 不变量 | 违规示例 |
|---|---|---|
| **A1** | `NO_SHIPMENT` 只产生 `REFUND_ADMISSION_CONFLICT`；**永不**产生 `PRE_SHIP_REVOKED`/`POST_SHIP_ALLOWED` | 把无行当已取消 → PRE |
| **A2** | admission **终态当且仅当 `state ∈ {PRE,POST}` 且 `result ∈ {PRE,POST}` 且 `state=result`**；`PENDING_REMOTE` 不是结果。TCC 的 `result=PENDING_REMOTE` 是既有偏差（§6.2），读者必须同时校验两个条件 | 只凭 `state==result` 或只读 `result` 判关闭/放款 |
| **A3** | **本 amendment 只消除 `CONSUMED+无运单` 的非法 PRE**。`PRE_SHIP_REVOKED` 的合法来源：`NONE→REVOKED`（其合法性依赖 permit 与 S6 事实一致；当前 Order 不校验）、`REVOKED` 幂等、`CONSUMED+CANCELLED`（真实取消）。legacy 无许可行 + 已发货仍可能写 PRE，是**既有缺口**（M-1），不得用本 amendment 声称已消除 | 把 legacy/无 permit 的 PRE 也写成“已证明合法” |
| **A4** | **自动路径**在无 admission 终态时：不得调用 `Payment.Refund`/`RefundBatch`、不得写 release/commitment release、不得把申请置 `DONE`/`PROCESSING`。**唯一例外**：AC-62/D-24 的人工授权 `RESERVED→RELEASED`（只释放额度，不恢复 permit、不关闭组、不当作退款完成、不写 `DONE`） | 自动释放；或把 AC-62 授权释放也禁掉 |
| **A5** | 冲突不改变 permit：`CONSUMED` 保持 `CONSUMED`，不得写 `REVOKED`，不得恢复/重建 permit（INV-45） | 为“关闭组”把 permit 改成 REVOKED |
| **A6** | 冲突不写 `orders.status`、不写 `refund_state`（INV-01/AC-25/AC-27A） | 用 conflict 推进生命周期 |
| **A7** | 普通正金额 `CONSUMED` 的 Tx1 会写**非终态占位事件**（`state=PENDING_REMOTE,result=POST`）；TCC 的 `CONSUMED` Tx1 不写占位事件（其 `NONE` 分支由 `AdmitRefund` 写终态 PRE 事件）。只有 A2 终态事件才是准入事实；消费者必须忽略非终态事件。conflict **不新增**事件；占位事件保留 | 把占位 `POST/PENDING` 当终态；conflict 再补一条 outbox |
| **A8** | Tx2 的终态更新必须 `UPDATE ... WHERE refund_id=? AND state='PENDING_REMOTE'` 并检查 `RowsAffected()==1`；0 行 → 读回已持久化结果并返回，**不得**再写 outbox/release。只有唯一执行者写终态事件/释放 | 两个并发 Tx2 各写一条 outbox/release |

### 6.1 与 AC-27A 的关系（不修改 AC 文本）

- AC-27A 的关闭谓词**只信** `order_refund_admissions.result=PRE_SHIP_REVOKED`（且 `state` 已终态）；它不使用运单事实、`refund_state`、`reservation_state`（`0001:622-623`、`:645`）。
- 本 amendment 的贡献是**前提保证**：修复后 `PRE_SHIP_REVOKED` **不再**由“无运单行”产生（新写入）；合法来源见 A3。这样 AC-27A 第 5 条在实现时才是可信的。
- 本 amendment **不修改** AC-27A 文本（SPEC 归 PO）、**不实现** AC-27A 第 5/6 条读模型（归 R0.3-R1c）、**不**把 `PENDING_REMOTE` 变成关闭。
- **存量 PRE 不因本修复自动合法**：§8 的一次性审计未完成前，AC-27A 第 5 条不得对无法证明来源的 PRE 组标已关闭/AC-VERIFIED（SPEC-0001 `0001:645`）。
- 在 `AC-50-FIX-01` 关闭前，AC-27 不得 AC-VERIFIED（`0001:874`）；本文件通过不构成本条关闭。

### 6.2 TCC `result=PENDING_REMOTE` 的既有偏差（M-4）

- SPEC-0001 `0001:645` 写“`result` 只允许 `PRE_SHIP_REVOKED`/`POST_SHIP_ALLOWED`”；但 TCC Tx1 `ensureTccRemoteAdmissionTx` 写 `result=PENDING_REMOTE,state=PENDING_REMOTE`（`tcc_settlement.go:847-852`）。这是**既有偏差**，不是本 amendment 引入。
- 本 amendment 的读者口径以 A2 为准：`state ∈ {PRE,POST}` **且** `result ∈ {PRE,POST}` **且** `state=result` 才是终态。AC-27A 读模型（R0.3-R1c）必须按此实现，不能只读 `result` 或只比 `state==result`。
- 清理登记为 `TCC-PENDING-RESULT-01`（owner Architect/Developer，expiry 2026-12-31）：要么不再写中间 `result`，要么由 SPEC-0002 明确允许；**不得**在 AC-50-FIX-01 中夹带。

---

## 7. R-6：幂等 / 并发 / 崩溃窗口（强制）

### 7.1 冲突路径时序（正金额/TCC）

```text
AfterSales/TCC durable task
   │ AdmitRefund(refund_id, group, amount)
   ▼
Order: 部分金额门禁（R-2a，仅 CONSUMED 且 amount < 组剩余）
   ├─ ListShipments 未离仓 → PARTIAL（零写入）
   ├─ 已离仓 → 继续
   └─ 不可判定 → Unavailable（零写入）
   ▼
Order: 读 admission
   ├─ 不存在 → Tx1: 读 permit
   │     ├─ NONE → CAS REVOKED → PRE（合法）
   │     └─ CONSUMED → 写 admission(占位) + commitment + 占位 outbox（Tx1 已写，§7.4）
   └─ 存在且 state=PENDING_REMOTE → 直接进入远程阶段
   ▼
CancelIfCreated(order, group) —— 事务外
   ├─ 无行 → NO_SHIPMENT
   ▼
Order: NO_SHIPMENT 分支（本次新增）
   ├─ 不写 Tx2、不改 admission、不写 release、不新增 outbox
   ├─ Error 告警（manual_queue=AC-27-NOWB-01）
   └─ 返回 REFUND_ADMISSION_CONFLICT
   ▼
组保持未关闭；Tx1 占位保留；申请保持可重试；无退款命令/无释放
```

> `result=POST*`（普通正金额）是既有 Tx1 占位，**不代表终态**；TCC Tx1 是 `result=PENDING_REMOTE`；终态判定只看 A2。本 amendment 不改占位值。

### 7.2 崩溃窗口

| # | 窗口 | 行为 | 恢复 owner |
|---|---|---|---|
| W1 | Tx1 已提交（`PENDING_REMOTE`）后、远程调用前崩溃 | 重放读 `PENDING_REMOTE` → 重新探测；无行 → conflict；不会 PRE。Tx1 写入已存在（admission/commitment；普通正金额另有占位 outbox，TCC 无占位 outbox） | 发起方 durable task（AfterSales/TCC）+ Order 幂等 |
| W2 | 远程已返回 `NO_SHIPMENT`、Order 返回错误前崩溃 | **Tx1 写入已存在；conflict 不新增任何 admission/outbox/release**；重放重新探测，确定性回到同一 conflict | 发起方 task 重试 |
| W3 | 远程已把 `CREATED→CANCELLED`、Order 写 Tx2 前崩溃 | 重放探测返回 `CANCELLED`（幂等）→ Tx2（CAS）写 PRE；不丢结果 | 发起方 task 重试 |
| W4 | Ship 与取消竞态（运单行 CAS） | Ship 胜 → `ALREADY_SHIPPED` → POST；取消胜 → `CANCELLED` → PRE；Ship 后置失败得 `SHIPMENT_CANCELLED`（AC-48） | Fulfillment（CAS）+ Order（映射） |
| W5 | 同一 `refund_id` 并发 `AdmitRefund` | 订单行锁串行化；败者重读已持久化行；可能重复探测，但 Tx2 由 A8 CAS 唯一化；admission 至多一行（PK） | Order |
| W6 | `NO_SHIPMENT` 后，Fulfillment 提供/修复**既有或曾存在但丢失**的 S6 取消证据（真实运单记录/承运商取消/仓出库撤销凭证 + 恢复审计），ops 凭 Fulfillment 授权核对 | 重放重新探测：既有行 `CREATED`→取消→PRE；既有行 `SHIPPED`→POST。**不得为从未存在的运单新建一行**来制造证据；若无法提供真实证据，走终态②（保持可重试/人工队列，不写 PRE、不关闭、不打款） | Fulfillment（S6 证据）+ ops（核对）+ Order（只读/重放） |
| W7 | 零元：远程 `CANCELLED` 后、单事务前崩溃 | 重放重新探测 → CANCELLED → PRE + `DONE`；零元无 commitment/Payment | 发起方 durable task（R-C 前置：先有持久 task） |
| W8 | `PENDING_REMOTE` 永久悬挂（task `FAILED`/DLQ/租约丢失） | 保持未关闭并告警；人工重放（AC-61）；重放仍冲突 → AC-27-NOWB-01；**不是**本 amendment 的修复范围（AC-27-LIVE-01）。TCC 停在 `ADMITTING`，有独立发现口径（§8） | ops（重放）+ Developer（AfterSales/TCC task） |
| **W9** | `order.refund_admitted` 重复/乱序（PRE 后重投、Tx1 的 `POST/PENDING` 占位、`order.paid` 迟到） | consumer 只处理终态 PRE；`NO_SHIPMENT` no-op success；重复 PRE 的 `CancelIfCreated` 幂等；`order.paid` 迟到由 `autoShipAllowed`（`fulfillment/consumer.go:66-70,111-123`）不创建 placeholder；`refund_consumer.go:35` 对无行 no-op | Fulfillment consumer + Order |

### 7.3 重试与“已持久化结果”的口径

- AC-50 `0002:227`：同 `refund_id` 重复申请先读**已持久化结果**并返回同一结果；结果缺失时重跑准入并收敛。
- 本裁决下，`state=PENDING_REMOTE` 属于“**结果尚未产生**”，不是已持久化结果；因此重放**允许且必须**重新探测，并确定性收敛到 conflict（或 CANCELLED 后的 PRE）。这与 AC-27A 注“`PENDING_REMOTE` 是中间态、不等同于 PRE”（`0001:645`）一致。
- conflict **不是**一个可持久化的 admission 结果；不得为了“幂等返回”而新增 `result=CONFLICT` 之类的值（那需要 SPEC-0002 升版，超出本 amendment）。幂等性由“无新增写入 + 确定性重探 + A8 CAS”保证。
- 发起方的重试上限/终态不在本 amendment 内：AfterSales 现有 `maxTaskAttempts=8`（`internal/services/aftersales/service.go:58`）耗尽后任务 `FAILED`，即人工队列信号；错误文本必须含 `REFUND_ADMISSION_CONFLICT` 以便落 `last_error`（`workers.go:724-748`）。**禁止**为消除重试而在 AfterSales 增加“conflict 当成功”的分支。
- TCC 冲突不经过 AfterSales：batch 停 `ADMITTING`、application 停 `PENDING`，由 §8 的 TCC 查询 + Order `s.log.Error` 发现；重放由 ops/发起方。

### 7.4 Tx1 已写事实的处置协议（B-1）

**前提**：`CONSUMED` 的 Tx1 在远程探测前提交（`refund_admission.go:253-283`）。普通正金额写 admission(`result=POST,state=PENDING_REMOTE`)+commitment+outbox；TCC 的 `CONSUMED` Tx1 写 admission(`result=state=PENDING_REMOTE`)+commitment（`tcc_settlement.go:847-852,898`），**不写** outbox。因此 conflict 时 **不得声称“无写入/无 outbox”**。

| 对象 | conflict 时状态 | 处置 | 理由 |
|---|---|---|---|
| `order_refund_admissions` | 普通：`result=POST,state=PENDING_REMOTE`；TCC：`result=state=PENDING_REMOTE`；**保持原样** | 不 UPDATE、不 DELETE | 删除会丢幂等锚点与证据；无合法终态值可写；AC-27A 第 6 条（`PENDING_REMOTE` 未关闭） |
| `order_refund_commitments` | `RESERVED`（Tx1 已写 1 条） | **保留，不释放**；继续占用额度 | INV-45；AC-62：只有授权释放/打款才能离开 `RESERVED`。这是该退款在途占用的正确表达 |
| outbox `order.refund_admitted` | 普通：Tx1 已写 1 条占位（`state=PENDING_REMOTE`）；TCC：无占位 | **不新增**；占位保留（A7）；**0 条终态事件** | 占位不是准入事实；消费者必须忽略 |
| INV-44 release（库存预留） | 未写 | **不写** | 只有 PRE 才发 release（`appendRefundReleaseTx` 仅 PRE 分支） |
| commitment release（`RESERVED→RELEASED`） | 未写 | **自动路径不写**；只有 AC-62/D-24 人工授权路径可写（不在本冲突路径） | INV-45/AC-62 |
| Payment | `GetIntent` 已读（普通/TCC）；无退款命令 | **不调用 `Refund/RefundBatch`** | INV-43/A4 |
| `refund_applications` | TCC：`PENDING`；普通：不写 | 不置 `DONE`/`PROCESSING` | 未准入 |
| permit | `CONSUMED` | 不写 | INV-45/A5 |
| 终态 | **无终态**：held | 人工队列 AC-27-NOWB-01；解决路径=W6 真实证据 / 人工放弃（AC-62 授权释放后新 `refund_id`） | SPEC-0001 `0001:863` |

**重放/新申请**：

- 同 `refund_id` 重放：读 `PENDING_REMOTE` → 重新探测 → 同一 conflict；不新增写入。
- 同组新 `refund_id`：旧 `RESERVED` 仍占用组剩余，`AdmitRefund` 的额度校验会把新申请挡为 `REFUND_ADMISSION_CONFLICT`（组剩余不足）或要求先人工处置；**不得**在旧承诺未处置时重复承诺（AC-62/INV-45）。
- 终态②人工放弃：经 AC-62/D-24 授权释放旧 `RESERVED`（只释放额度），之后才允许新申请；组仍不关闭、permit 仍 `CONSUMED`。

### 7.4.1 为什么不用“Tx1 前只读存在性检查”避免 conflict 写入

| 时序 | 事件 |
|---|---|
| T0 | Order `ListShipments`：该组无行（只读预检） |
| T1 | Fulfillment `CreateShipment` 插入 `CREATED` 行（`fulfillment/service.go:154-166`；该调用不经过 `AuthorizeShipment`，permit 仍是 `CONSUMED`） |
| T2 | 若 Order 以 T0 的结果跳过 Tx1：无准入持久化；但 T1 已产生运单行，后续仍需 Tx1+探测，T0 结果已过期 |
| T3 | 若 Order 仍以 Tx1+探测为准：T1 的行被取消 → PRE；或 Ship 胜出 → POST；或行再次消失 → conflict |

- 结论：**预检不能消除 conflict 窗口**，只增加一次远程调用和一个过期信号；真正的线性化点是“Tx1 + 远程探测”。因此不采用预检来“避免写入”。
- R-2a 的部分金额门禁是**不同目的**的拒绝决策（不是准入决策）：拒绝是 fail-closed、可重试的，且不承担“PRE/POST 判定”职责；它不能替代 Tx1+探测。
- 若未来要把 `CONSUMED` 的 Tx1 outbox 移到 Tx2（“完全无占位事件”方案），必须改 `AdmitRefund`/TCC 既有行为、扩 `allowed_paths` 并重做 W1–W3 分析；本 amendment **不做**，见 §13。

---

## 8. R-7：人工队列（AC-27-NOWB-01）与告警

| 项 | 裁决 |
|---|---|
| 队列载体 | **复用** SPEC-0001 §16.2 `AC-27-NOWB-01`（`0001:863`）；**不新增**状态字段/表/队列模型，不加 `result=CONFLICT` |
| 组状态 | 保持未关闭（AC-27A 第 6 条：`PENDING_REMOTE`/结果缺失 → 未关闭 + 告警）；permit 保持 `CONSUMED` |
| 申请状态 | 保持可重试（AC-50 `0002:228`）；不得打款、不得自动释放、不得置 `FAILED_PERMANENT`（除人工放弃，且需 SPEC-0002 owner 书面口径与 AC-62 授权） |
| 终态① | Fulfillment 提供/修复**既有或曾存在但丢失**的 S6 运单取消证据（真实记录/承运商取消/仓出库撤销 + 恢复审计）→ ops 凭 Fulfillment 授权核对（Order 不写运单）→ 按 AC-50 合法分支重放 → PRE → 组关闭。**不得为从未存在的运单新建行伪造证据** |
| 终态② | 无法补齐 → 申请保持可重试并由人工队列持有；仅人工放弃时按 AC-62 记 `FAILED_PERMANENT` 并取得 SPEC-0002 owner 书面口径；**AC-62/D-24 授权释放旧 `RESERVED` 是唯一释放路径**（只释放额度、不恢复 permit、不关闭组、不写 `DONE`）；组保持未关闭并告警；不得自动关闭/自动退款/恢复 permit |
| owner / expiry | `Developer（Order 数据核对）/ops（重放、跨服务数据）/Fulfillment（S6 证据）`；**2026-12-31**（与 WIP waiver 同日；延期需重新裁决，见 `0001:856`） |
| 告警机制（固定 key/大小写） | Order `s.log.Error`，msg=`refund admission conflict: consumed permit without shipment`，字段 `refund_id/order_no/group_no/source/probe_status=NO_SHIPMENT/manual_queue=AC-27-NOWB-01`。`WithDetails` 的 `reason=CONSUMED_WITHOUT_SHIPMENT` 同义；两者均不得改名（T-6）。`deploy/**` 的 Prometheus 规则/runbook 接线不在本 amendment 范围（单独 L2） |
| 队列发现（普通售后台） | Order Error 日志 + AfterSales `after_sale_refund_tasks.last_error LIKE '%REFUND_ADMISSION_CONFLICT%'`（`workers.go:724-748`） |
| 队列发现（TCC，M-7） | `SELECT b.batch_id,b.order_no,b.payment_no,a.refund_id,a.group_no FROM refund_batches b JOIN refund_applications a ON a.batch_id=b.batch_id WHERE b.state='ADMITTING' AND a.state='PENDING' AND b.updated_at < NOW() - INTERVAL 10 MINUTE`；配合 Order `s.log.Error`（`tcc_admission.go:97-110`）。`refund_batches.last_error`/`refund_applications.last_error` 列存在（`migrations/order/0017_refund_batch_ledger.sql:28`），但写入需改生产 `tcc_*.go`，登记 `TCC-LAST-ERROR-01`（DEFERRED，owner Developer/Architect，expiry 2026-12-31） |
| 存量 PRE 一次性审计（M-5） | 修复上线后执行：`SELECT order_no, group_no, refund_id FROM order_refund_admissions WHERE result='PRE_SHIP_REVOKED' AND state='PRE_SHIP_REVOKED'`，与 `order_shipping_permits`/`order_fulfillment_groups`/`refund_applications` 交叉核对；**同时**查异常组合 `permit IN ('REVOKED','CONSUMED') AND group.status IN ('SHIPPED','RECEIVED') AND result='PRE_SHIP_REVOKED'`（M-1 legacy 已发货）。owner=ops（核对）+Developer（Order 查询），expiry 2026-12-31；审计结论未出前，AC-27A 第 4/5 条不得对相关组标已关闭/AC-VERIFIED，读模型告警。不要求本文件改台账 |
| 证据 | 单测捕获日志（注入 `slog.Handler` 断言 key/value）+ 真实 MySQL 集成测试断言无终态/无释放/无退款命令；ops 用上述查询 + `last_error` 发现 |

---

## 9. R-8：登记面（proto 注释 / `docs/contracts.md` / 错误码映射）

| 登记项 | 是否需要 | 内容 / owner | 证据 |
|---|---|---|---|
| `api/proto/mall/v1/mall.proto` 注释 | **必须** | 在 `CancelIfCreatedReply.status` 注释列出 `NO_SHIPMENT` 及五值语义；RPC 注释补“or report the group has no shipment row”。owner：Developer 按本裁决写入；Architect 已给文本（§2.1）。**不加字段/不加 RPC** | `mall.proto:1043`、`:1089-1098`；`make proto` 后 `api/gen/mall/v1/mall.pb.go` 注释同步（当前注释在 `:11203`） |
| `api/gen/**` | **仅生成** | 由 `make proto`（`Makefile:153-157`）重新生成；**禁止手改**。`scripts/check-proto-evolution.py` 只比较字段号/RPC 签名，注释变更不需要更新 baseline。**T-2**：实施 Brief 固定 `protoc`/`protoc-gen-go` 版本（当前生成头 v1.36.12/protoc v7.36.2），并要求 `git diff -- api/gen` 只出现 `CancelIfCreated`/`CancelIfCreatedReply` 的注释行（`mall.pb.go` 与 `mall_grpc.pb.go` 均只允许注释 diff；任何字段/方法签名/编号 diff 即 FAIL） | `make proto-guard`（`Makefile:147-151`） |
| `docs/contracts.md` | **不需要** | 该文件只规定“用 `apperr` 映射 gRPC status”（`contracts.md:235-239`），没有逐 RPC 状态目录；ADR §5.3 才是契约正文。若人类希望加一行指针，属 L0 文档变更、owner Lead，另行处理 | `docs/contracts.md:235-239` |
| 错误码映射 | **不新增** | 继续用 `REFUND_ADMISSION_CONFLICT`（SPEC-0002 错误表 `0002:517`、ADR §10 `0007:988`）与 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`（AC-50 `0002:217`）；实现用 `apperr.Failed`（`FAILED_PRECONDITION`）。canonical token 必须在 message/details 出现 | `internal/platform/apperr/apperr.go:61-64`；`refund_admission.go:312-328` 既有同类用法 |
| 事件/payload | **不需要** | conflict 不新增 `order.refund_admitted`；Tx1 占位事件按 A7 解释，不新增 subject/字段 | `refund_admission.go:610-619` |
| **T-1** SPEC-0002 错误表漂移 | **`[TODO]` → PO** | `0002:517` 对 `REFUND_ADMISSION_CONFLICT` 的描述未覆盖 `0002:223`；owner PO，expiry 由 Lead 登记 2026-12-31；本文件不改 SPEC | `0002:223`、`:517` |

---

## 10. 测试矩阵（实施强制最小集）

> 类型：U=单元/fake port；I=真实 MySQL 集成；R=`-race`；L=日志断言；N=NATS 集成。**setup 列**（M-13）：fresh=`AdmitRefund` 新申请；preseed=预置 Tx1 行/commitment/占位；done=已有终态。所有用例绑定精确 HEAD，日志入库由 Developer 提供；**不得**用全绿 mock 冒充。测试名可微调，但语义行不得删。

| # | setup | 场景 | 层 | 断言 | 期望 |
|---|---|---|---|---|---|
| **F1** | — | Fulfillment：无运单行 | U/I | `status="NO_SHIPMENT"`；`shipments` 行数 0；无 outbox/事件 | 新增 |
| **F1b** | — | Fulfillment：`shipment_no` 非空 + 无行 | U/I | `NO_SHIPMENT`；不误取消同组其他行 | T-5 |
| **F2** | — | Fulfillment：真实 `CREATED` 行 | U/I | `CANCELLED` 且行变 `CANCELLED` | 回归（`cancel_if_created_test.go:12-58` 扩展） |
| **F3** | — | Fulfillment：已是 `CANCELLED` | U/I | 幂等 `CANCELLED`，无二次写 | 回归 |
| **F4** | — | Fulfillment：`SHIPPED`/`RECEIVED` | U/I | `ALREADY_SHIPPED`/`RECEIVED` | 回归 |
| **F5** | — | Fulfillment：`EXCEPTION`+`shipped_at` / 无 `shipped_at` | U/I | `ALREADY_SHIPPED` / `TRANSIENT_ERROR` | 回归（`:65-128`） |
| **F6** | — | Fulfillment：G1 无行、G2 有 `CREATED` 行 | U/I | 请求 G1 → `NO_SHIPMENT`；G2 行不被取消 | 组隔离 |
| **F7** | N | `order.refund_admitted(PRE)` consumer 收到 `NO_SHIPMENT`；重复 PRE；迟到 `order.paid` | N/I | `NO_SHIPMENT` no-op success（无错误/无新行）；重复 PRE 不重复取消；迟到 `order.paid` 不建 placeholder | M-8/M-11 |
| **F8** | — | 正常业务路径回归：`CONSUMED+CREATED` → PRE；`CONSUMED+SHIPPED` → POST | I | 真实行/真实 permit；不依赖合成 `CONSUMED+无行` | M-12 |
| **O1** | fresh | 正金额 `CONSUMED` + 全额 + `NO_SHIPMENT` | I | conflict（`FAILED_PRECONDITION`、含 token）；**Tx1 已写**：admission `result=POST,state=PENDING_REMOTE`、1 条 `RESERVED` commitment、1 条占位 outbox；**0 条终态 outbox、0 条 release**；permit 仍 `CONSUMED`；`payments.refundCalls==0`（`GetIntent` 允许） | **核心阻断** |
| **O2** | fresh | O1 同 `refund_id` 重放 | I | 再次 conflict；admission 行数 1；commitment 行数 1；占位 outbox 仍 1；**无新增**；探测被再次调用 | 幂等（重探） |
| **O3** | preseed | 预置 `PENDING_REMOTE`（admission+commitment+占位 outbox）后调用 | I | 同 O1 的 conflict；**计数完全不变**（无新写入） | W1/W2（preseed 证明“无新增”） |
| **O4** | fresh | 正金额 `CONSUMED` + 全额 + `CANCELLED`（单组订单） | I | `PRE`；admission `state=result=PRE`；释放按 INV-44（单组）；无重复 commitment | 回归（现有 `TestAdmitRefundConsumedPermitTwoPhase`） |
| **O4b** | fresh | 正金额 `CONSUMED` + **部分金额** + `ListShipments` 未离仓（无行/`CREATED`） | I | `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；**0 条 admission/commitment/outbox**；`cancelCalls==0`；无 release | R-2a/M-2 |
| **O4c** | fresh | 正金额 `CONSUMED` + **部分金额** + `ListShipments` 已离仓（`SHIPPED`） | I | 继续 Tx1+探测 → `POST`；1 条 commitment；部分金额合法 | R-2a 正向 |
| **O5** | fresh | 正金额 `CONSUMED` + `ALREADY_SHIPPED`/`RECEIVED` | I | `POST`；**恰好 1 条 `RESERVED` commitment（Tx1 已有，Tx2 不重复）**；不取消运单 | M-10 |
| **O5b** | fresh | TCC 已离仓组（`recordTccPostAdmission`） | I | POST admission + commitment **同事务**（既有，不改）；无探测 | M-10 |
| **O6** | fresh | 正金额 `CONSUMED` + `TRANSIENT_ERROR` / 空 / 未知状态 | I | `apperr.Unavailable`；Tx1 写入存在但无终态；无 release；`refundCalls==0` | fail-closed |
| **O6b** | preseed | 预置 `PENDING_REMOTE` + `TRANSIENT_ERROR` | I | `Unavailable`；计数不变（无新写入） | M-13 |
| **O7** | fresh | 零元 `CONSUMED` + `NO_SHIPMENT` | I | conflict；`order_refund_admissions` 0 行；`refund_applications` 0 行；无释放；`payments.refundCalls==0 && getCalls==0`；permit 不变 | **核心阻断** |
| **O7b** | fresh | 零元 `CONSUMED` + `TRANSIENT_ERROR`/空/未知 | I | `Unavailable`；0 admission/application；无释放；无 Payment | T-4 |
| **O8** | fresh | 零元 `NO_SHIPMENT` 重放；随后提供真实 `CANCELLED` 行 | I | 第一次 conflict 且 0 写入；改为真实取消后重试 → `PRE`+`DONE` | 终态①恢复 |
| **O9** | fresh | TCC `source=tcc` + `NO_SHIPMENT` | I/R | `AdmitRefund` conflict；batch 停 `ADMITTING`；application 停 `PENDING`；admission `result=state=PENDING_REMOTE`（M-4）；无占位 outbox/无终态 outbox；`refundCalls==0` | 共享映射 |
| **O10** | fresh | legacy 来源 + `NO_SHIPMENT` | I | 同一 conflict（与 source 无关）；legacy 无许可行时仍 `NONE→REVOKED→PRE` | 来源矩阵 |
| **O10b** | — | legacy 无 permit + 组已发货 | — | **`[DEFERRED]`（M-1，owner Architect/PO，expiry 2026-12-31）**：不属于本 change_id；用 §8 审计查询定位；不得以 `legacy` 名称默认 PRE；修复后补反向测试（应 POST 或拒绝），不得把错误 PRE 固化 | 缺口 |
| **O11** | fresh | 同 `refund_id` 并发 `AdmitRefund` + `NO_SHIPMENT` | I/R | 两路均 conflict；admission 1 行；无终态/无释放 | 并发 |
| **O11b** | fresh | 同 `refund_id` 并发 `AdmitRefund` + `CANCELLED` | I/R | admission 1 行；**终态 outbox 恰 1 条；release 至多 1 条**（A8 CAS） | M-9 |
| **O12** | fresh | 告警键（普通 + TCC） | L | Error 日志含固定 key（§8）；TCC 侧含 `batch_id/refund_id` | 可观测 |
| **O13** | preseed | 非回归：AC-27A 读模型 | I | `groupIsClosed(CONSUMED + PENDING_REMOTE)=false`；本修复不开始读 `result`/不写 permit | 只断言“不变” |
| **O13b** | preseed | TCC `result=PENDING_REMOTE,state=PENDING_REMOTE` | I | `groupIsClosed=false`（A2 双条件；M-4） | 读者口径 |
| **O14** | N | consumer 重复/乱序（同 F7，含 `order.paid` 迟到） | N | 不重复取消/不建 placeholder/不报错 | M-8 |
| **O15** | — | AC-62 人工授权释放（conflict 后） | — | **`[DEFERRED]`（T3 未实现 release writer；owner Developer/T3，expiry 2026-12-31）**：不得用未授权释放/Order fixture 冒充；实现时须断言只释放一次、不恢复 permit、组仍不关闭、未授权 → `REFUND_RELEASE_NOT_AUTHORIZED` | M-3 |

补充要求：

- **金额 Mock 真实性**：正金额用例必须 >0；零元 `pay_cent=0`、`settlement_payment_no='zero:<order_no>'` 合法单列；不得用 0 元冒充正金额（宪法 IV.3）。
- **合成状态说明**：O1/O7 的 `CONSUMED+无行` 是 fail-closed 合成异常（§0.2），必须另有 F8 正常路径回归。
- **不得**用 `t.Skip()`、fixture 伪造 `CANCELLED` 行冒充**生产证据**、或 `if status=="NO_SHIPMENT" { return nil }` 绕过 O1/O7。O8 中插入 `CANCELLED` 行是“模拟 Fulfillment 已确认真实行后的状态”的测试 fixture，必须在测试注释中说明，Reviewer 区分 fixture 与生产证据。
- O4/O4b/O4c 默认单组订单；多组释放缺口见 §14 `INV-44-MULTI-01`，不得用 Order fixture 的 `RELEASED` 冒充。

---

## 11. 反模式自省（强制）

| 反模式 | 本设计如何阻止 |
|---|---|
| 用“无运单 → 当已取消”的和稀泥映射掩盖异常 | 新增可区分事实 `NO_SHIPMENT`；Order 侧 conflict 是**显式错误**，不写终态；O1/O7 断言无 PRE、无终态事件、无退款命令、无释放 |
| 把 Tx1 已写说成“无写入/无 outbox” | A7/§7.4 承认 Tx1 写入并给出处置；O1 断言占位 1 条、终态 0 条；O3 用 preseed 证明“无新增” |
| 把 `GetIntent` 当成退款/违禁调用 | B-2 口径：只禁 `Refund/RefundBatch`；`GetIntent` 只读允许；测试只断言 `refundCalls==0` |
| 为“制造取消证据”新建运单行 | §2.3 #1/W6 明确禁止；人工只能修复既有 S6 证据；Reviewer 对伪造 fixture 直接 FAIL |
| 在 AfterSales/TCC/零元各写一份 `switch status` | R-4 单一映射权威；Reviewer 以“Order 中 `"CANCELLED"` 出现点”为审查判据 |
| 用 `result=CONFLICT`/新状态字段/新表“记住冲突” | 禁止；冲突不是 admission 结果；`PENDING_REMOTE` + 告警 + 人工队列已足够（A2/A7） |
| 为消除告警而在调用方把 conflict 当成功/当可忽略 | 明确禁止（§7.3）；AfterSales 只能重试/最终 `FAILED` 人工队列；TCC 停 ADMITTING/人工 |
| 顺手把 AC-27A 读模型、legacy 已发货、TCC result 清理一起改 | 硬边界/§13：AC-27A 实现归 R0.3-R1c；M-1/M-4 另立；本 amendment 不夹带 |
| 用迁移/DB CHECK 兜底业务语义 | 无迁移；语义由唯一映射 + 测试保证；若未来要 CHECK，另立 L2 |
| 用“旧客户端会忽略新值”赌兼容 | 已验证旧 Order 对未知值 fail-closed（`refund_admission.go:582-584`），且发布顺序有明确回滚 |
| 两个并发 Tx2 各写一条终态事件/释放 | A8 状态 CAS + `RowsAffected==1`；O11b 断言终态 outbox 1 条、release ≤1 |

---

## 12. 【假设与不确定性】（强制）

1. **`NO_SHIPMENT` 的命名**：brief 建议、本裁决采用。若人类/QC 更倾向 `SHIPMENT_NOT_FOUND` 等，仅改名，不改变语义与矩阵；改名后 proto 注释与测试需同步。
2. **“无行”的事实边界**：依赖 `shipments` 的 `(order_no, fulfillment_group_no)` 唯一约束（`0003:9`）。若未来允许同组多运单（拆包裹），`CancelIfCreated` 的 `QueryRow` 语义与本裁决的“整组无行”都需要重新裁决；**当前不成立**。
3. **`shipment_no` 语义**：Order 传空（现状）→ 组级探测；若未来调用方传具体 `shipment_no`，`NO_SHIPMENT` 只表示“该三键无行”，不等于整组无行。本裁决不改变调用约定；发布前需确认仓库外无调用方（T-5）。
4. **`result=POST`/`PENDING_REMOTE` 占位**：普通正金额 Tx1 写 `result=POST,state=PENDING_REMOTE`；TCC Tx1 写 `result=state=PENDING_REMOTE`。本裁决不改占位，要求 A2/A7 读者口径。若 QC 要求清理，另立 L2（`TCC-PENDING-RESULT-01` + 普通占位清理）。
5. **旧客户端行为**：本裁决假设除 Order 与本仓库 `order.refund_admitted` 消费者外无其他 `CancelIfCreated` 调用方（全仓库 grep 支持）；仓库外客户端若存在且把未知值当 `CANCELLED`，会误判。发布前需由 Lead/ops 确认。
6. **AfterSales 重试上限**：依赖现有 `maxTaskAttempts=8`（`aftersales/service.go:58`）把长期冲突收敛到 `FAILED` 人工队列；若该值/策略变更，人工队列的“可发现性”需重新评估。
7. **告警接线**：本裁决只要求 Order 结构化 Error 日志；`deploy/**` 的告警规则/runbook 不在 allowed_paths。若 ops 要求 Prometheus 规则，需另立 L2；在此之前“告警”= 日志键 + `last_error`/TCC 查询。
8. **`change_id` 拆分**：实施须由 Lead 另拆（`0001:879`）。若 Lead 决定复用 `PAY-SETTLE-01` 的子任务编号，仍需独立实施范围与证据索引；不得把本文件当成 R0.3-R1c 的放行。
9. **§0.2 可达性**：`CONSUMED+无行` 在正常 `Ship` 路径不可达（先建行后授权）；可达来自直接 `AuthorizeShipment` RPC/历史数据/人工修复。生产发生率 `[不确定]`，不据此声称 P0 频率；Lead 需登记证据需求。
10. **`LEGACY-SHIPPED-PRE-01`（M-1，DEFERRED）**：`ensurePermitTx`（`settlement.go:263-278`）无 permit 时无条件建 `NONE`，`AdmitRefund` 对 `NONE` 不做 S6 校验（`refund_admission.go:249-288`）；历史已发货组可能被写 PRE。修复需在 `NONE` 分支引入 S6 预读/组状态校验，改变 AC-50 `NONE` 分支语义，**超出本 amendment**；owner=Architect/PO，expiry 2026-12-31；关闭前 AC-27A 第 4 条不得对 legacy 已发货组标 AC-VERIFIED（§8 审计）。
11. **`TCC-LAST-ERROR-01`（M-7，DEFERRED）**：TCC 失败只 `s.log.Error`（`tcc_admission.go:97-110`），未写 `refund_batches.last_error`；写入需改生产 `tcc_*.go`，另立 `change_id`；当前用 §8 查询 + 日志发现。
12. **`TCC-PENDING-RESULT-01`（M-4，DEFERRED）**：TCC Tx1 `result=PENDING_REMOTE` 与 SPEC 注不一致；清理归 Architect/Developer，expiry 2026-12-31；之前读者按 A2 双条件。
13. **R-2a 门禁依赖**：依赖 `Fulfillment.ListShipments`（`ports.go:47-55`；ADR §5.2.1 已要求）与 `s.fulfillment != nil`；不可用/歧义必须 fail-closed，不得退化为“允许部分退款”。若 QC 认为门禁改变 ADR §5.2.2 的“不预读”语义，需回 Architect 裁决；本裁决的解释是“预读只用于拒绝，不作为 PRE/POST 依据”。
14. **未执行**：本文件未运行任何测试/迁移；代码行号是阅读事实，不构成执行证据。`[不确定]`：仓库外调用方；生产 `shipments` 是否已全部应用 `0003` 的每组唯一约束（未应用则多行会引入探测歧义）——均需 ops/迁移证据确认。

---

## 13. 不交付 / 明确不做（防范围漂移）

- **不交付 T2**（历史支付 24h writer/worker、`legacy_24h` source）、**T3**（commitment release/行级台账）、**T4.4**（`order.settled` shadow 切换）；仍按 `docs/lead/PAY-SETTLE-01/tasks.md` 的既有 DEFERRED 处置。
- **不改 SPEC-0002**（AC-50 文本保持；`REFUND_ADMISSION_CONFLICT`/`PARTIAL_REFUND_REQUIRES_FULL_GROUP` 语义不变）；**不改 SPEC-0001**（AC-27/AC-27A 文本归 PO）。
- **不改已 accepted ADR Rev 2.3 正文**：本 addendum 单独存在；未来 Rev 2.4 由 Architect 做 errata 合并（需人类批准）。
- **不新增**：RPC、proto 字段、事件 subject/payload 字段、迁移/schema/CHECK、permit 状态值、`order_refund_admissions.result/state` 取值、worker、分布式锁、配置开关。
- **不改**：Payment 金额/退款实现、AfterSales task 策略、Inventory 释放协议、`orders.status` 生命周期、`refund_state` 语义、Fulfillment consumer 的生产语义。
- **不做**：AC-27A 读模型实现（`groupIsClosed` 读取 admission）——归 R0.3-R1c；M-1 legacy 已发货修复；M-4 TCC `result` 清理；T3 commitment 人工释放实现；把 Tx1 outbox 全部移到 Tx2 的重构（§7.4.1）——均另立 `change_id`/L2。

---

## 14. Baseline 影响 / INV / AC 映射

### 14.1 INV

| INV | 影响 | 本裁决如何保持 |
|---|---|---|
| INV-36（退款统一准入；未发货组 `NONE→REVOKED`、已授权未离仓取消运单；同 `refund_id` 幂等） | 涉及 | 只改 `CONSUMED` 探测结果映射与部分金额前置；`NONE` 分支不变（A3 收窄，legacy 缺口另登）；幂等由重探+A8 保证 |
| INV-42（退款申请不丢失、可重试、同键幂等） | 涉及 | conflict 不丢申请；Tx1 admission/commitment 保留；同 `refund_id` 重放确定 |
| INV-43（未得 PRE/POST 前不得调 Payment） | 涉及（核心） | conflict 不产生终态 → 禁止 `Refund/RefundBatch`（O1/O7/O9 断言 `refundCalls==0`）；`GetIntent` 只读不在此限 |
| INV-44（组退款库存处置有主；释放失败重试告警） | 涉及 | conflict 不触发释放；PRE 后仍走既有持久释放；**多组释放缺口**登记 `INV-44-MULTI-01`（DEFERRED，owner Developer/Lead，expiry 2026-12-31；`refund_release.go:178-187` 只在单组发事件） |
| INV-45（准入不可逆；permit 终态不可恢复；释放纪律） | 涉及（核心） | conflict 不写 permit、不恢复/撤销 `CONSUMED`；自动路径不释放；AC-62 授权释放为唯一例外 |
| INV-50（零元闭环不经过 Payment；`DONE` 为执行事实） | 涉及 | 零元 conflict 不写 `DONE`、不调 Payment；合法 CANCELLED 仍 `DONE(PRE)` |
| INV-01 / AC-25（生命周期与退款态正交） | 不涉及（不得触碰） | conflict 不写 `orders.status`/`refund_state` |
| INV-46/47/48/49 | 不涉及 | 不改历史支付/行级/Σ/shadow |

### 14.2 AC 映射（证据归 Reviewer，本文件不更新台账）

| AC | 关系 | 说明 |
|---|---|---|
| AC-50（SPEC-0002 `0002:213-231`） | 直接修复 `0002:223` 分支 + 保留 `0002:217` 部分前置 | `CONSUMED+无运单` → `REFUND_ADMISSION_CONFLICT`；部分金额 → `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；不写 PRE/不打款/不释放 |
| AC-27A（SPEC-0001 `0001:615-645`） | 前提保证，**不改文本** | 新写 PRE 只来自 A3 合法分支；`PENDING_REMOTE` 不关闭；存量 PRE 审计完成前不得关闭/AC-VERIFIED；读模型实现仍归 R0.3-R1c |
| AC-27（`0001:592-613`） | 收口前提 | `AC-50-FIX-01` 未关闭前 AC-27 不得 AC-VERIFIED（`0001:874`） |
| AC-62 / INV-45（`0002:308-323`） | 约束 | conflict 不得自动走人工释放；只有 AC-62/D-24 授权放弃才有 `FAILED_PERMANENT`/释放路径；释放不恢复 permit、不关闭组 |
| AC-48（`0002:200-206`） | 非回归 | permit `CONSUMED` 不同 `shipment_no` 仍拒绝；取消后不得复活运单 |
| AC-51/AC-65（TCC） | 非回归 | TCC 经 `AdmitRefund` 共用映射；batch 不得在 conflict 时进入 ADMITTED/PAYING；发现口径见 §8 |
| AC-67/AC-68（零元） | 非回归 | 零元 conflict 不写 `DONE`；合法分支不变；零元无 Payment |
| AC-27-NOWB-01 / AC-27-LIVE-01（`0001:863-864`） | 人工队列/不收敛 | 本文件只提供冲突信号；终态与重放归 ops/Developer；TCC 映射同队列 |

---

## 15. 实施边界与停止条件

### 15.1 施工图（最小 diff，不写代码）

1. **Fulfillment**：`cancel_if_created.go` 的 `sql.ErrNoRows` 分支返回 `NO_SHIPMENT`（不 commit、不建行）；其余分支不变；新增/更新注释。
2. **Order 部分金额门禁（R-2a）**：`AdmitRefund` 在 Tx1 前对 `CONSUMED && amount < 组剩余` 调只读 `ListShipments`；未离仓 → `PARTIAL`（零写入）；已离仓 → 继续；不可判定 → `Unavailable`；Tx1 锁内复核金额。
3. **Order 正金额/TCC/legacy 映射**：`finishRemoteAdmission` 在打开 Tx2 前对 `NO_SHIPMENT` 返回 conflict（含 details + Error 告警）；`CANCELLED→PRE`、`ALREADY_SHIPPED/RECEIVED→POST`、其余 `Unavailable`；Tx2 按 A8 CAS；仅 PRE 触发既有释放。
4. **Order 零元**：`decideZeroRefundResult` 在 `permitConsumed` 分支对 `NO_SHIPMENT` 返回 conflict；不进入 `writeZeroRefundTx`；其余分支不变。
5. **proto**：只改 `CancelIfCreated`/`CancelIfCreatedReply` 注释；`make proto` 生成；`make proto-guard` 通过（注释不改变 baseline）。
6. **测试**：§10 矩阵；真实 MySQL 集成 + `-race` + 日志/NATS 用例；不新增生产 consumer seam。

### 15.2 停止条件（命中即停手，回 Architect/Lead）

1. 需要新增 proto 字段/RPC/事件字段、迁移、permit 状态、`result/state` 新值 → 超出本 amendment。
2. 需要修改 AfterSales/Payment/Inventory/Deploy/`docs/specs/**`、生产 `tcc_*.go`、`admission_consumer.go` 语义 → 越界，回 Lead 拆范围。
3. R-2a 的 `ListShipments` 预读若被判定违反 ADR §5.2.2 → 回 Architect（不得自行加 `if` 绕过）。
4. 发现“无运单 → conflict”会破坏 `NONE→REVOKED` 分支或 legacy G1 → 设计误读，回 Architect。
5. 任何消费方必须读 `result` 而不看 `state` 才能工作 → 说明 A2 被破坏，停止并回 Architect（可能是 AC-27A 实现问题）。
6. 旧客户端/仓库外调用方把 `NO_SHIPMENT` 当 `CANCELLED` → 兼容性阻断，回 Lead/ops 确认发布策略。
7. 测试无法区分 fresh 与 preseed（B-2/M-13）或无法证明 Tx1 占位/终态二分（B-1） → 证据不成立，停止。

### 15.3 交付证据（实施时，不属本文件）

- 精确 base/head；`go test`/`-race`/真实 MySQL 日志；`【测试范围】`/`【未覆盖范围】`；受影响 AC 行由 Reviewer 更新；O1/O3/O7 的“终态 0、退款命令 0、释放 0”断言日志；F1“无行不建行”证据；O4b“预检零写入”证据；O11b“终态事件 1 条”证据；proto 生成 diff 由 `make proto` 产生（非手改，仅注释）。
- **不**声称 AC-VERIFIED/Release/merge；`AC-27` 在三个前置缺陷（`AC-50-FIX-01`/`AC-27-NOWB-01`/`AC-27-LIVE-01`）关闭前保持 AC-REF。

---

## 16. Consequences 与后续 owner

| 对象 | 动作 |
|---|---|
| **QC（round 2）** | 复核本 0.2：B-1/B-2/B-3 是否真正关闭；M-1/M-3/M-4/M-5/M-7 的 DEFERRED owner/expiry 是否可接受；R-2a 是否确实不改变 PRE/POST 判定权威；测试矩阵 setup 是否可执行 |
| **人类（Director）** | L2 ADR 批准（`ADR_APPROVED`），然后才允许实施；保留 merge/Release 检查点 |
| **Lead** | 按 `0001:879` 另拆独立实施 `change_id`；建 Plan/Tasks + AC 证据矩阵；登记 `LEGACY-SHIPPED-PRE-01`/`TCC-PENDING-RESULT-01`/`TCC-LAST-ERROR-01`/`INV-44-MULTI-01`/T-1 的 owner/expiry；不把本文件当 R0.3-R1c 放行 |
| **Developer** | 在独立 `change_id` 下实现 §15.1；跑 §10；提交 proto 生成物与测试；不 commit 本文件（Lead 统一提交） |
| **Fulfillment owner** | 提供“无行”可区分结果与**既有** S6 证据；配合 AC-27-NOWB-01 终态①；不得新建行 |
| **ops** | 用 Order 日志键 + AfterSales `last_error` + TCC `ADMITTING/PENDING` 查询发现人工队列；执行 §8 存量 PRE 审计；告警接线另立 L2 |
| **Reviewer** | 按精确 base/head 校验映射穷尽性、Tx1 占位/终态二分、无退款命令/释放、proto 仅注释变更、测试真实执行（fresh/preseed 区分）；AC-27 仍 AC-REF |
| **Architect** | 后续 Rev 2.4 errata 合并本 addendum（需人类批准）；如 QC 发现新缺口，修订本文件并升 Revision |

---

## 17. 变更记录

| Revision | 日期 | 变更 |
|---|---|---|
| 0.1 | 2026-11-26 | 初稿：R-1…R-8；新增 `NO_SHIPMENT`、Order/零元映射、持久化不变量、崩溃窗口、测试矩阵、登记面 |
| **0.2** | 2026-11-26 | 按 `QC-PAY-SETTLE-01-AC50-NOWB.md`（BLOCKED）整改：**B-1** 承认 Tx1 已写并新增 A7/A8 与 §7.4 处置协议（含“为何不用 Tx1 前预检”时序）；**B-2** 改为只禁 `Refund/RefundBatch`、`GetIntent` 允许，并重写 O1/O3/O6/O7/O9 断言；**B-3** W6 改为只修复**既有/曾存在但丢失**的真实 S6 证据、禁止新建行；**M-1…M-13** 逐条修复或登记 DEFERRED（`LEGACY-SHIPPED-PRE-01`/`TCC-PENDING-RESULT-01`/`TCC-LAST-ERROR-01`/`INV-44-MULTI-01`）；新增 **R-2a 部分金额前置门禁**、W9 重复/乱序、TCC 发现口径、存量 PRE 审计、setup 列、T-1…T-7 处置。未写代码、未改 SPEC/Rev 2.3 正文、未 commit |

**Actor**：`arch-ac50`（System Architect）
**Reviewed-by**：待 QC round 2
**Status**：**DRAFT — pending QC round 2**
