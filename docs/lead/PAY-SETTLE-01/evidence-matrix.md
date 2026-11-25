# AC Evidence Matrix — PAY-SETTLE-01 Remediation

- Source: SPEC-0002 **v1.10**、ADR-0007 **Rev 2.3**；C-02/C-03 当前版本。
- **唯一 AC 台账**：`docs/collab/ac-coverage.tsv`，Owner 固定为 **Reviewer**。本文件只是 Plan 视图，不拥有 AC 状态。
- `Test owner`：Developer；`AC status owner`：Reviewer。两者不得混写。
- Status: `NOT STARTED` / `AC-REF` / `AC-VERIFIED` / `DEFERRED`。只有真实执行日志（绑定 HEAD）+ Reviewer 核对断言才能标 AC-VERIFIED；SKIP/FAIL 不算。

## 1. Existing AC

| AC | 必须覆盖的 EARS 分支（摘） | Test type | Evidence path / planned test | Test owner | Status |
|---|---|---|---|---|---|
| AC-39 | 意图/确认金额≠pay_cent；渠道异常捕获退款 | unit + MySQL + fault | `internal/services/payment/create_intent_test.go`、capture mismatch test | Developer | NOT STARTED |
| AC-40 | 零元不捕获；正金额捕获退款；取消/缺货/售后只释放不调 Payment | MySQL | `order/zero_integration_test.go`、`order/refund_admission_integration_test.go`、零元退款回归 | Developer | AC-REF（旧测试，需新 HEAD 重跑） |
| AC-41 | 第二支付退款；同 payment 幂等；并发 CAS 唯一胜者 | MySQL + `-race` | `order/duplicate_payment_integration_test.go`、duplicate capture test | Developer | AC-REF（旧测试，需新 HEAD 重跑） |
| AC-42 | 捕获未结算只显示处理中；PAID 不等于可发货；组退款展示 | handler/contract | BFF/order handler test | Developer | NOT STARTED |
| AC-43 | 捕获后崩溃续跑；订单 CANCELLED 时 HELD/CONFIRMED 收敛 | fault injection | settlement worker crash test；inventory compensation test | Developer | NOT STARTED |
| AC-44 | 回调重复；payment_no 金额/订单不一致；本地失败不 ack；已被其他 payment 结算 | MySQL + fault | payment callback/outbox test | Developer | NOT STARTED |
| AC-45 | 非 PENDING 重读；PAID 同 payment 幂等；PAID 不同 payment 退款 | MySQL + `-race` | `order/duplicate_payment_integration_test.go` | Developer | AC-REF（旧测试，需新 HEAD 重跑） |
| AC-46 | 取消 vs 支付 CAS 唯一胜者；已 PAID 不提供取消 | MySQL + `-race` | order cancel/confirm race test | Developer | NOT STARTED |
| AC-47 | Inventory 明确不足；技术失败重试；成功后缺货转 AC-56 | MySQL + fault | readiness timeout/failure test | Developer | AC-REF（CheckOrderReady 旧测试，需重跑） |
| AC-48 | 许可 NONE/CONSUMED/REVOKED；行确认；缺货；Ship 读 CANCELLED/CREATED | MySQL | `order/settlement_integration_test.go`、shortage/permit tests | Developer | NOT STARTED |
| AC-49 | 四事实分别输出 | handler/contract | API/BFF contract test | Developer | NOT STARTED |
| AC-50 | 未离仓整组；已离仓部分；REVOKED/CONSUMED/CANCELLED；CAS 败者；取消失败重试；释放失败告警 | MySQL + `-race` | `order/refund_admission_integration_test.go`、cancel-vs-ship race test | Developer | AC-REF（旧测试，需补分支/重跑） |
| AC-51 | PENDING 捕获后 CAS→CANCELLED；活动组；Σ 预检；部分失败；无组 G1；已全额跳过 | MySQL + `-race` + fault | TCC batch/fan-out test、crash-resume test | Developer | NOT STARTED |
| AC-52 | 缺货退款；已离仓只退不取消 | MySQL + worker | shortage refund test | Developer | NOT STARTED |
| AC-53 | 订单级 captured/refunded；非生效捕获单独记账；多 intent | MySQL | reconciliation/ledger test | Developer | NOT STARTED |
| AC-54 | 单组隔离；整单全退阻断所有未发组 | MySQL | group isolation test | Developer | NOT STARTED |
| AC-55 | Payment 事实为准；超时未知保持处理中；惰性对账 | MySQL + fault | refund projection/reconcile test | Developer | NOT STARTED |
| AC-56 | 4h 通知；48h 默认退款；等待补货；RESTOCKED；并发 CAS | MySQL + time/race | shortage timer/choice/restock tests | Developer | NOT STARTED |
| AC-57 | CREATED/CANCELLED 不可收货；重复收货幂等 | MySQL | ADR-0005 existing receipt tests | Developer | AC-REF（既有基线，需引用/重跑） |
| AC-58 | 收货后争议不回滚 receipt | MySQL | ADR-0005 existing dispute tests | Developer | AC-REF（既有基线，需引用/重跑） |
| AC-59 | 退款/收货/库存失败/缺货解决乱序拒绝 | fault/integration | event out-of-order tests | Developer | NOT STARTED |
| AC-60 | 历史 PAID 展示/退款/发货；唯一候选；人工队列；SLA；`EXPIRED_*` 终态 | MySQL + backfill + fault | `ListOrderIntents` resolve test、backfill test、resolution queue test（含 24h 终态） | Developer | NOT STARTED |
| AC-61 | 30min 处理中告警；永久失败人工重放；每日对账 | fault + ops | reconciliation/alert/manual replay test | Developer | NOT STARTED |

> **2026-11-25 R0.3-R2 基线回归**（`fe1600b`，真实 MySQL，无 SKIP）：AC-14 六子场景 1.589s、AC-31/32、AC-21 0.459s、AC-25/AC-27 `-race` 2.226s、order 全包 89.268s、`go build ./...` PASS。命令与日志见 `docs/implementation/PAY-SETTLE-01-R0.3-selfcheck.md:100-125`。台账状态仍以 Reviewer 为准（本次不标 AC-VERIFIED）。

## 1b. AC-27（SPEC-0001 v1.4，R0.3-R1 补分支）

| AC | 必须覆盖的 EARS 分支（摘，canonical SPEC-0001 :571-579） | Test type | Evidence path | Test owner | Status |
|---|---|---|---|---|---|
| AC-27 | ①G2 已退款完成且 G1 正常发货→按 G1 推进、G2 不阻塞、订单不停在 `ALLOCATED`；②WHILE `refund_state=REFUNDED` 且 permit `REVOKED` 或运单 `CANCELLED`→生命周期保持 `PAID/ALLOCATED/SHIPPED` 之一，退款进度只写 `refund_state`，不写 `orders.status`；③预留已释放或运单已取消→§6.6/§6.6.1 判定合法；反向：退款未完成/已发货组不得被投影成 CANCELLED | unit + MySQL | `warehouse_quote_test.go:125`（REFUNDED+REVOKED+RELEASED via `toProto`）、`ac27_projection_integration_test.go`（真实写入行回读）、`ac27_projection_cancelled_test.go`、`ac27_projection_negative_test.go`（post-ship 反向） | Developer | AC-REF（2026-11-25 `fe1600b` 跑 PASS；**3 支 BLOCKED**：waybill CANCELLED / reservation released 不可观察 / pre-ship REFUNDING 语义待 Architect 裁决） |

## 2. New semantics — SPEC-0002 v1.10 已分配 AC-62…68

> 2026-11-25 R0.5：删除旧稿“建议 AC”表述；AC-62…68 已在 `ac-baseline.txt` 与 SPEC-0002 v1.10 正式分配，未分配前不得实现的规定不再适用。状态列只是 Plan 视图，最终以 `ac-coverage.tsv`（Reviewer）为准。

| AC | 必须覆盖的 EARS 分支（摘，canonical SPEC-0002 §4.9/§4.10） | Test type | Evidence path / planned test | Test owner | Status |
|---|---|---|---|---|---|
| AC-62 | permit 终态不可逆；`FAILED_PERMANENT`/放弃保持 `RESERVED` 进人工；D-24 授权矩阵（角色、≥1000 双人、审计、最小证据、过期/撤销→`REFUND_RELEASE_NOT_AUTHORIZED`、冲突→`REFUND_COMMITMENT_CONFLICT`）；幂等一次 `RESERVED→RELEASED`；禁止 worker 静默释放；只回同组额度、不触发退款；仅 `REJECTED/CANCELLED` 或 `FAILED_PERMANENT`+授权两类触发 | MySQL + fault | `order/refund_commitment_release_test.go`（T3，planned） | Developer | DEFERRED（T3，2026-12-31） |
| AC-63 | 生效支付解析：候选 `SUCCEEDED` 且 `captured==pay_cent`；唯一候选 CAS 写回；零元 `zero:<order_no>`；有捕获不得 zero；0/多候选进队列 24h；有捕获无离仓→`COMPENSATING`（撤 permit→`CancelIfCreated`→INV-44 释放→Payment.Refund 最后）→`EXPIRED_REFUND`；有离仓→`EXPIRED_FINANCE`（7 天、不重开）；补偿失败保持；终态 CAS 不被覆盖；`REJECTED` 不做“等待补证” | MySQL + backfill + fault | `order/legacy_payment_resolution_test.go`（T2，planned）；0022 `chk_lpr_expiry_24h` | Developer | DEFERRED（T2，2026-12-31） |
| AC-64 | 行级 + 订单 captured 双层额度；商品/可退运费拆分；`UNDETERMINED` 商品照退、运费挂起 ≤168h；后置补退同组；累计退运费 ≤ 该组实付 ≤ 用户实付；责任唯一来源（AfterSales/缺货/TCC），Order 只校验；未离仓整组仅全额（`PARTIAL_REFUND_REQUIRES_FULL_GROUP`）；重复补退幂等 | MySQL + `-race` | `order/refund_line_ledger_test.go`（T3，planned） | Developer | DEFERRED（T3，2026-12-31） |
| AC-65 | 同一 Payment 快照 Σ 预检（系统责任上限=商品剩余+未退运费）；不等→状态变更前阻断+人工；逐组 `tcc:<payment_no>:<group_no>` 准入；全部准入后才打款；部分失败只补未完成；崩溃续跑不重退；快照变化重跑；已离仓 POST/未离仓 REVOKED；无活动组幂等成功；无组按 G1；`PENDING_PAYMENT` 已捕获先裁决 | MySQL + `-race` + fault | `internal/services/order/t17_evidence_integration_test.go`（T1.7 test-only PASS） | Developer | AC-REF（T5 待复核） |
| AC-66 | 默认 OFF 只记录/比对，不建单不放行；与 `order.paid` 双发；不一致只告警；影子自身失败不影响结算/准入；未知版本/缺字段拒绝；OFF→SHADOW 仅 Director 影子批准；SHADOW→ON 需 §7 矩阵+五方终态+Reviewer 人工核对+review-tracking+K-6；回滚人类批准；切换后 `order.paid` 不得再作准入依据 | MySQL + fault | `fulfillment/shadow_consumer_test.go`（T4.4，planned） | Developer | DEFERRED（T4，2026-12-31） |
| AC-67 | 零元短路同事务写 S19 `DONE`，不建 `PENDING`、不调 Payment；未 `PAID` 取消不建 S19；有捕获不得走零元；金额>0 拒绝 `REFUND_ADMISSION_CONFLICT`；需回库时回库完成后短路；失败不留无 worker 的 `PENDING`；重复提交幂等 | MySQL | `order/zero_refund_integration_test.go`（planned；T1.4b2b 已有零元测试） | Developer | DEFERRED（T5，2026-12-31） |
| AC-68 | AfterSales 最终 `REFUNDED/effective=0/refunded=0/CONSUMED/refund_no=''`；`DONE(zero_amount)` 由 `source=system_zero`+`amount=0`+`REFUNDED` 推导，不加字段；S19 非 `DONE` 不得展示；未 `PAID`/有捕获不得写；对账与 S19 `DONE` 对平；正金额仍按 AC-55 | MySQL | 同上 + `aftersales` 零元投影测试（planned） | Developer | DEFERRED（T5，2026-12-31） |

## 2b. New invariants INV-45…50 (SPEC v1.10，2026-11-25 R0.3-R3 补全)

本表只作 Plan/证据索引，不拥有 AC 状态；AC-VERIFIED 仍只由 Reviewer 写入 `ac-coverage.tsv`。行描述按 SPEC-0002 v1.10 §3 原文收敛，不再省略分支。

| INV | 必须覆盖的分支（摘） | Test type | Evidence path / planned task | Owner | Status |
|---|---|---|---|---|---|
| INV-45 | 准入不可逆；承诺只可因①打款前 `REJECTED/CANCELLED` 或②`FAILED_PERMANENT`/人工放弃 + 人工授权而 `RESERVED→RELEASED`；只释放额度、不恢复 permit、不当作退款完成、只回同订单同组；禁止 worker/重试/脚本静默释放；`SETTLED`/`RELEASED`/金额不等拒绝 | MySQL + fault | `migrations/order/0018_refund_commitment_composition.sql`（表/约束）+ T3 release tests（planned） | Developer | DEFERRED（T3） |
| INV-46 | `PAID` 且生效支付为空先解析；唯一候选 `SUCCEEDED`+`captured==pay_cent`；零元 `zero:<order_no>`；有捕获不得 zero；0/多候选队列 24h 离开 `OPEN/RESOLVING`（`COMPENSATING` 或终态）；有捕获无离仓→`COMPENSATING`（撤 permit/`CancelIfCreated`/INV-44 释放，Payment.Refund 最后）→`EXPIRED_REFUND`，有离仓→`EXPIRED_FINANCE`；补偿失败保持 `COMPENSATING`；不猜 payment_no、不无限期 OPEN | MySQL + backfill + fault | T2 `legacy_payment_resolution`/backfill（planned）；0022 `chk_lpr_expiry_24h` | Developer | DEFERRED（T2） |
| INV-47 | 行级 ≤ 行剩余、合计 ≤ captured 剩余（INV-24 双层）；商品+可退运费；责任唯一（AfterSales/缺货/TCC），Order 只校验；`UNDETERMINED` 商品照退、运费挂起 ≤168h；后置补退同组；累计退运费 ≤ 该组实付 ≤ 用户实付 | MySQL + `-race` | T3 line/freight tests（planned） | Developer | DEFERRED（T3） |
| INV-48 | 改任何状态前 Σ 活动组系统责任上限 == 订单剩余 captured；不等阻断+告警+人工，不部分打款；逐组准入/打款；部分失败只补未完成组；崩溃续跑不重退 | MySQL + `-race` + fault | `internal/services/order/t17_evidence_integration_test.go:26,83,204,306,371,442`（6 测试 2026-11-25 `fe1600b` PASS）；T5 `-race`/fault 待补 | Developer | AC-REF（未 AC-VERIFIED） |
| INV-49 | shadow 不具准入权威；默认关闭；只记录/比对；不一致/失败只告警；切换需独立 release 门禁 + 人类批准；切换后 `order.paid` 不得再作准入依据 | MySQL + fault | T4.4 shadow tests（planned）；ADR Rev 2.3 §12.3 | Developer | DEFERRED（T4） |
| INV-50 | 零元且无任何已捕获 intent：售后/取消/缺货只释放或回库、不调 Payment；`source=system_zero`、`amount=0`、S19 `DONE`；AfterSales `REFUNDED/CONSUMED/effective=0/refunded=0/refund_no=''`；不得正金额、不得写 `orders.status=REFUNDED`、不得在未 `PAID` 时伪造已退款 | MySQL | `internal/services/aftersales/zero_refund_integration_test.go:497`（端到端）+ `:182`（无 Payment 完成）；反向 `:562`（正金额拒绝）/:711（零捕获非零元拒绝）；2026-11-25 `fe1600b` PASS | Developer | AC-REF（未 AC-VERIFIED） |

## 3. Required logs

- `-race`：AC-41/45/46/50/51/54/56 + 退款准入 vs Ship + TCC batch 并发。
- 故障注入：AC-43/47/50/51/55/56/59/61 + TCC 崩溃续跑 + payment.refunded 丢失/惰性对账。
- 迁移：`refund_applications`、commitment lines、`legacy_payment_resolution`、`refund_seq` 必须跑真实 `migrations/`。
- 每条证据：HEAD、命令、结果、环境；SKIP/FAIL 不计。
- `ac-coverage.tsv` 统一状态；本矩阵不替代它。

## 4. 12 orphan disposition（R0.3-R4，Lead 提议；Reviewer 最终写入台账）

旧 `ac-coverage.tsv` 的 12 行 reason=`orphan (no WP)` 不再成立：逐个映射到 T5 任务或显式 T3 依赖。**不删除、不批量续期**；owner/expiry 保持 Developer/2026-12-31，除非 Reviewer 另裁。

| AC | 处置 | 映射任务 / 依据 |
|---|---|---|
| AC-39 | T5 故障注入 | T5.3：渠道异常捕获、金额≠`pay_cent` 的支付级退款 |
| AC-42 | T5 contract/handler | T5.1：捕获未结算只显示处理中、PAID≠可发货 |
| AC-43 | T5 故障注入 | T5.3：捕获后崩溃续跑、CANCELLED 时 HELD/CONFIRMED 收敛 |
| AC-47 | T5 故障注入 | T5.3：Inventory 明确不足/技术失败重试/成功后缺货转 AC-56 |
| AC-48 | T5 `-race` | T5.2：permit NONE/CONSUMED/REVOKED、行确认、Ship 读状态 |
| AC-49 | T5 contract/handler | T5.1：四事实分别输出（支付/结算/库存/准入） |
| AC-56 | T5 `-race`+fault | T5.2/T5.3：4h/48h、补货、并发 CAS |
| AC-57 | 既有基线 AC-REF | T5.5：ADR-0005 收货测试，只引用/重跑后标 AC-REF |
| AC-58 | 既有基线 AC-REF | T5.5：ADR-0005 争议测试，同上 |
| AC-59 | T5 故障注入 | T5.3：退款/收货/库存失败/缺货解决乱序拒绝 |
| AC-62 | 显式 T3 依赖（非 orphan） | T3.1–T3.3/AC-62；保持 DEFERRED，Phase 0-R/T5 不实现 |
| AC-64 | 显式 T3 依赖（非 orphan） | T3.4–T3.6/AC-64；保持 DEFERRED，Phase 0-R/T5 不实现 |

说明：AC-62/AC-64 属 P1-T3（用户明确本阶段不进入 T3），因此仅关闭“orphan 无归属”问题，不关闭 AC 本身；其余 10 条进入 T5 证据计划。台账 reason 由 Reviewer 改写为对应 `T5.x` / `T3 dependency`。

