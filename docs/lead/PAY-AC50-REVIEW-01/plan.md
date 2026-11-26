# PAY-AC50-REVIEW-01 — AC-50 `CONSUMED+无运单` 复核决定（L2）

| 项 | 值 |
|---|---|
| Status | **APPROVED（2026-11-26，human P5）**；ADR 0.6 errata ACCEPTED（C-5）；实现进行中（T0 发布单元 PASS scoped；T1–T8 pending） |
| change_id | `PAY-AC50-REVIEW-01` |
| Parent | `PAY-SETTLE-01`（**保持冻结**；本 L2 交付前不推进其受管代码/台账） |
| Tier | **L2**；命中 P0、资金、跨服务、状态机、既有 RPC 响应语义、共享 metrics |
| Base | `a1cc809`（C-4 + 四角色 Plan 评审落库） |
| Spec-Ref | SPEC-0002 **v1.11 approved（2026-11-26）**：AC-50/51/61/62/65/67 修订、AC-69…72、S20、INV-45/46/51/52、§5.10、D-30…D-35；SPEC-0001 v1.5 AC-27A（只读依赖） |
| ADR | ADR-0007 Rev 2.3 + AC-50 Addendum **0.5 ACCEPTED** + **0.6 errata ACCEPTED（2026-11-26，QC R9 READY + human）**；impact 0.4 |
| integration_owner | System Architect（契约/跨服务）；Order Owner（实现）；ops（runbook/人工队列） |
| 发布单元 | **T0+T0b+T0c 必须同一发布单元**（Fulfillment `<NO_WAYBILL>`、Order 映射、Tx2 双谓词 CAS；否则 T0b 先上的窗口仍可把 REVIEW 洗成 PRE）；若分批，先 Fulfillment，再 T0c，最后 T0b。旧 Order 对未知 status **fail-closed**（`Unavailable`，不得当 PRE）。无迁移 |
| allowed_paths | `internal/services/fulfillment/cancel_if_created.go`；`internal/services/order/refund_admission.go`、`tcc_settlement.go`、`tcc_admission.go`、`worker.go`（仅 gauge）；`internal/services/aftersales/workers.go`（仅 `after_sale_refund_tasks.last_error`/PENDING/REVIEW 8 次例外）；`internal/platform/metrics/metrics.go`；`api/proto/mall/v1/mall.proto`（仅 status 注释/既有 string）；**`api/gen/**` 仅由 `make proto` 生成、同提交、禁手改**；`cmd/*`（仅 wiring，Brief 列明）；`docs/ops.md`（仅 runbook）；`docs/implementation/PAY-AC50-REVIEW-01-selfcheck.md`（证据产物；ADR 0.6 需同步允许）；对应 `_test.go` |
| forbidden_paths | `migrations/**`（无迁移）、`deploy/**`（告警 `REVIEW-REQUIRED-ALERTS-01`）、`frontend/**`（入口 `REVIEW-REQUIRED-ENTRY-01`）、`docs/specs/**`、`docs/adr/**`（实现阶段）、`docs/collab/**`、`docs/lead/**`、`scripts/**`、`Makefile`、`api/gen/**` 手改、accepted ADR Rev 2.3 正文、`internal/services/order/{tcc.go,tcc_payment.go,tcc_precheck.go,tcc_reconcile.go,settlement.go,ports.go}`、`internal/services/fulfillment/{consumer.go,admission_consumer.go}`、`internal/services/{inventory,payment}/**`、`internal/services/gateway/**` |
| baseline_impact | **涉及**：AC-50/51/61/62/65/67、AC-69…72、INV-36/42/43/44/45/50/51/52；Fulfillment/Order RPC 响应语义（新增 string 值）；TCC `MANUAL_HOLD`；共享 metrics。**不涉及**：Payment/Inventory 状态机、C-04 RefundBatch、事件 subject/payload、permit 状态机、迁移/schema、新 worker；**不交付** PAY-SETTLE-01 的 T2/T3/T4.4/T5、BFF/UI、告警规则 |
| 回归基线 | AC-14/21/25/27/31/32；INV-44…50；TCC Σ/逐组；零元 AC-67/68；legacy AC-60/63。**必须跑；不得改 PAY-SETTLE-01 的 AC-27/Phase 0-R 状态** |

## 0. 为什么另立 change_id

- `PAY-SETTLE-01` 已冻结（D-LC-5；Phase 0-R 未闭合）。AC-50 修复升级为新 SPEC 状态 + TCC 行为 + 跨服务写路径 + 审计/指标，无法在父 change_id 内继续。
- 同一时间只推进一个 `change_id`；本 L2 期间父 change_id 只读。
- 本 L2 的 PASS 只覆盖 AC-50/AC-69…72；**不**关闭 AC-27，**不**等于 Phase 0-R/P0/Release。

## 1. 契约门

| ID | 内容 | 状态 |
|---|---|---|
| C-1 | PO 关闭 R4-B1/B2/B3 + 3 MAJOR | **CLOSED**（SPEC v1.11 approved） |
| C-2 | Architect Addendum 0.5 | **CLOSED**（ACCEPTED） |
| C-3 | QC round 8 终审 0 BLOCKER | **CLOSED**（READY） |
| C-4 | 人类 `SPEC_APPROVED`/`ADR_APPROVED` | **CLOSED**（2026-11-26） |
| **C-5** | **ADR errata 0.6**：Tx2 CAS `state='PENDING_REMOTE' AND result <> 'REVIEW_REQUIRED'`、`docs/ops.md` runbook、proto 生成、gateway 删除、发布顺序 | **CLOSED**（QC R9 READY + human 2026-11-26） |

## 2. 任务（原子；每任务独占一个不变量；依赖 C-5 + P5）

> 约定：`ORDER_DSN=$(scripts/test-dsn.sh "$TEST_MYSQL_DSN" order)`、`FF_DSN=... fulfillment`、`AS_DSN=... aftersales`；`<HEAD>` 由 Developer 在 T7 绑定。命令中的包路径按任务列。

| ID | 独占不变量 | 内容 | ADR §10 | 测试文件 | 命令 | expected |
|---|---|---|---|---|---|---|
| **T0** | `NO_WAYBILL` 可区分且不误取消 | Fulfillment 无行返回 `<NO_WAYBILL>`；真实 CANCELLED 不变；proto 注释 + `make proto` 生成同提交 | F1/F1b/F2–F6/F8 | `fulfillment/cancel_if_created_test.go` | `TEST_MYSQL_DSN=$FF_DSN go test ./internal/services/fulfillment -run 'TestCancelIfCreated' -count=1` | 无行=`<NO_WAYBILL>`；CREATED→CANCELLED；SHIPPED/RECEIVED/EXCEPTION 原值；G1/G2 组隔离；无 outbox |
| **T0b** | Order 映射 | `NO_WAYBILL`→REVIEW；未知 status fail-closed；不写 PRE/0 释放 | O1/O7/O10 | `order/ac50_review_integration_test.go` | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC50ReviewMapping' -count=1` | `result=REVIEW_REQUIRED,state=PENDING_REMOTE`；permit 不变；0 终态 outbox/0 release；未知→`Unavailable` |
| **T0c** | REVIEW 不可被覆盖 | Tx2 CAS **`state='PENDING_REMOTE' AND result <> 'REVIEW_REQUIRED'`**；0 行读回同一冲突 | O11/O11b/O11c/O11d、W5/W13 | `order/ac50_review_race_test.go` | `TEST_MYSQL_DSN=$ORDER_DSN go test -race ./internal/services/order -run 'TestAC50ReviewConcurrent' -count=1` | 并发只有一方写；REVIEW 不被洗成 PRE/POST；0 行→`Conflict("admission disappeared")`；迟到 CANCELLED 仍 REVIEW |
| **T1a** | TCC 准入读合法 | `ensureTccRemoteAdmissionTx` 接受 REVIEW；不早失败 | O1b/O2b/O13b | `order/tcc_review_integration_test.go` | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestTCCReviewAdmission' -count=1` | 不出现 “not a legal value”；PENDING 保持 |
| **T1b** | TCC 申请单调 | `ensureTccApplicationTx` 单调接受 REVIEW；不覆盖已持久结果 | O1b | 同上 | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestTCCReviewApplication' -count=1` | application `PENDING`+`admission_result=REVIEW_REQUIRED`；幂等 |
| **T1c** | TCC 停批 | batch `MANUAL_HOLD`+`last_error`；0 打款；只阻塞本批；**Tx2 同事务写 batch/app/admission** | O1b/W12 | 同上 | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestTCCReviewManualHold' -count=1` | `MANUAL_HOLD`；`refundCalls==0`；其他批不受阻；batch/app/admission 同事务，失败整体回滚 |
| **T1d** | TCC 自动续跑拒绝 | `TCC_BATCH_REQUIRES_MANUAL_HANDLING`；不自动 FAILED | W12 | 同上 | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestTCCReviewResumeToken' -count=1` | 返回 token；0 状态变化 |
| **T1e** | TCC re-arm | `MANUAL_HOLD→ADMITTING`；审计/授权；不自动 | O16 | 同上 | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestTCCReviewRearm' -count=1` | 审计先写；CAS version 生效；不自动 |
| **T2a** | existing-first 幂等 | 先于 `Payment.GetIntent`；同 `refund_id` 同裁决、不重探/不写/不读 Payment；错误 details 带**结构化** `result/reason/ticket`（禁止字符串嗅探） | O2/O2b/O2c/W6 | `order/ac50_review_integration_test.go` | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC50ReviewReplay' -count=1` | 同错；`cancelCalls/getCalls` 不增；`refundCalls==0` |
| **T2b** | 售后镜像 canonical | AfterSales 写 `after_sale_refund_tasks.last_error` = `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>`（无 arm；超长 ` truncated=1`） | O19/O23 | `aftersales/review_task_integration_test.go` | `TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run 'TestReviewTaskMirror'` | 恰为 canonical；`refund_applications` 0 行；解析 reason/ticket/truncated；只按 typed details 识别 REVIEW |
| **T2c** | 售后任务态 | REVIEW 保持 PENDING；8 次不自动 FAILED；非 REVIEW 保留 | O22/O22b | 同上 | `TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run 'TestReviewTaskRetry' -count=1` | REVIEW→PENDING；非 REVIEW→FAILED_PERMANENT；无自动 DONE/释放 |
| **T3** | 部分金额分类 | fresh/preseed：未离仓→PARTIAL 零写入；已离仓→POST；不可判定→REVIEW+1 RESERVED；`cancelCalls==0` | O4a/O4b/O4c/O4d/O4e/O5c/O10b/W10 | `order/ac50_review_integration_test.go` | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC50ReviewPartial' -count=1` | 三分支 expected；preseed 不新增写入；`refundCalls==0` |
| **T4a** | 零元 | 不写 admission/S19/REVIEW、不计 AC-72、不调 Payment；真实取消后收敛 | O7/O7b/O8/O21/W7 | `aftersales/zero_refund_integration_test.go` + order | `TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run 'TestZero.*Review' -count=1`；`TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestZeroReview' -count=1` | 0 行/0 写入；`refundCalls==0 && getCalls==0`；重放后 PRE+DONE；**不计入 REVIEW backlog（单列），仍进入零元人工队列**；无 S19 |
| **T4b** | legacy | 不猜 payment；无 permit+投影缺失+SHIPPED→REVIEW；未离仓→PRE；已离仓→POST | O10/O10b | `order/ac50_review_integration_test.go` | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC50ReviewLegacy' -count=1` | REVIEW/PRE/POST 按矩阵；不默认 PRE |
| **T5a** | 审计留存 | admission 行不删、`version`/时间戳、镜像 `last_error` 不清、re-arm 先审计后清 | O20/O20b/W6/W11 | `order/ac50_review_integration_test.go` | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC50ReviewAudit' -count=1` | 行保留/version 递增/时间戳；`refund_batches.last_error` 保留 |
| **T5b** | 指标/SLA | `mall_refund_review_backlog`/`oldest`；7 天查询；零元不混入 | O18 | `platform/metrics/metrics_test.go` + `order/worker_test.go` | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/platform/metrics ./internal/services/order -run 'TestAC50ReviewMetrics|TestRefundReviewGauge'` | gauge 计数；zero 不计；7 天可查 |
| **T5c** | ops runbook | AC-72 查询/SLA/对账、§8.3 re-arm、存量 PRE 审计写入 `docs/ops.md`（owner ops；expiry 2026-12-31；证据=runbook 链接/演练记录） | O18/O20 | `docs/ops.md`（runbook） | 人工审阅 | 查询 SQL/告警 key/SLA/对账/re-arm/存量审计步骤齐备 |
| **T6a** | Tx1/Tx2 崩溃 | W1–W3：Tx1 后崩溃重放、Tx2 前/后崩溃；不提前终态 | O3/W1–W4 | `order/ac50_review_fault_test.go` | `TEST_MYSQL_DSN=$ORDER_DSN go test -race ./internal/services/order -run 'TestAC50ReviewCrash|TestAC50ReviewRace' -count=1` | 重放收敛；无终态/释放；W4：Ship 胜→POST、取消胜→PRE |
| **T6b** | 重复/乱序 | W8/W9/W13：重复 PRE、迟到 `order.paid`、REVIEW 后不自动 PRE | O14/O14b/W13/F7b | `order/review_consumer_integration_test.go` | `TEST_MYSQL_DSN=$ORDER_DSN TEST_NATS_URL=nats://127.0.0.1:4222 go test -race ./internal/services/order -run 'TestReviewConsumer' -count=1` | 不重复取消/不建重复 placeholder/REVIEW 不自动 PRE |
| **T6c** | DLQ | DLQ/ACL 重放；0 资金副作用 | F7/O14/W8/W11（W8 仅验证不重复；永久悬挂归 AC-27-LIVE-01） | `order/review_consumer_integration_test.go` | `TEST_MYSQL_DSN=$ORDER_DSN TEST_NATS_URL=nats://127.0.0.1:4222 go test ./internal/services/order -run 'TestReviewConsumerDLQ' -count=1` | 重放幂等；无重复退款 |
| **T6d** | re-arm 顺序 | 先写审计/镜像，再 CAS 清 `result`；不自动 | O16/O20/W11/W13 | `order/ac50_review_integration_test.go` | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC50ReviewRearm' -count=1` | 审计先于清；version CAS；不自动 |
| **T7** | 证据矩阵 | 修订 AC-50/51/61/62/65/67 + AC-69…72（AC-72 三段）+ INV-51/52；Reviewer 写 tsv；`NOT RELEASE` | — | `ac-coverage.tsv`（Reviewer）+ `docs/implementation/PAY-AC50-REVIEW-01-selfcheck.md` | `git rev-parse HEAD` + Reviewer 逐行核对（无独立测试） | 每行 test_type/evidence/status；AC-72 三段；HEAD `<HEAD>` |
| **T8** | 回归 | AC-14/21/25/27/31/32、INV-44…50、TCC/零元/legacy；不得改父状态 | — | 既有测试 | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC25|TestAC27|TestT17' -count=1`；`TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run 'TestAC14|TestAC31|TestAC32|TestZero' -count=1`；`TEST_MYSQL_DSN=$FF_DSN go test ./internal/services/fulfillment -run 'TestAC21' -count=1` | 全 PASS；无 SKIP |

## 3. 测试矩阵/命令（T7 逐条绑定）

- 全量用例码：ADR §10（F1–F8、O1–O23、W1–W13）；每个任务至少绑定其列出码；不得 `t.Skip()`、不得 fixture 冒充生产证据。
- 环境：`TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_integration_test?parseTime=true&loc=UTC'`；`scripts/test-dsn.sh` 每包独立 schema；NATS `TEST_NATS_URL=nats://127.0.0.1:4222`。
- 全量命令：`go test ./internal/services/{order,fulfillment,aftersales} ./internal/platform/metrics -count=1`；`-race` 至少 T0c/O11/T6a/T6b；故障注入/崩溃用例；T7 记录实际 HEAD/日志。
- 每条证据：`test_type`、`evidence_path`、`expected`、`HEAD`；FAIL/SKIP 不算。

## 4. AC/INV/台账

- 修订 AC-50/51/61/62/65/67；AC-69…72 **in-scope**；INV-51/52。
- AC-72 三段：后端（O18）/ `REVIEW-REQUIRED-ALERTS-01`（DEFERRED，deploy）/ `REVIEW-REQUIRED-ENTRY-01`（DEFERRED，frontend）；`ac-coverage.tsv` 三段登记，不得整体 VERIFIED。
- `ac-coverage.tsv` 仅 Reviewer 写；Developer 提供路径；存量 PRE 审计 `AC-27-NOWB-01` 登记 open，不在本 L2。

| AC-72 段 | 范围 | Owner | Expiry | 状态 | Evidence |
|---|---|---|---|---|---|
| 后端 | metrics/SLA/查询/对账（O18） | Developer/Reviewer | 2026-12-31 | in-scope | T5b |
| 告警 | `REVIEW-REQUIRED-ALERTS-01`（deploy 规则） | ops | 2026-12-31 | DEFERRED | `deploy/**` |
| 入口 | `REVIEW-REQUIRED-ENTRY-01`（用户/人工入口） | PO/ops | 2026-12-31 | DEFERRED | `frontend/**` |

## 5. DoD

- 代码 Diff、精确 HEAD、真实日志（MySQL 8.4/NATS/`-race`/故障注入）、`【测试范围】/【未覆盖范围】`、反妥协自检。
- `git diff --stat -- migrations` 为空；0016/0017 字节不变；无新 RPC/字段/worker。
- `release_status=NOT RELEASE`；不声称 AC-VERIFIED/Release PASS；不关 AC-27/Phase 0-R。

## 6. Stop

- C-5 未关、QC/Reviewer FAIL、契约缺口 → 冻结。
- CAS 非 `state='PENDING_REMOTE' AND result <> 'REVIEW_REQUIRED'`、REVIEW 洗成 PRE、部分金额取消副作用、审计被清、TCC 非法 result、`NO_WAYBILL` 未区分 → stop-the-line。
- 需迁移/新字段/新 RPC/新 subject/新 worker → 回 Plan + 人类批准。

## 7. 交回

- L2 完成后恢复 `PAY-SETTLE-01`：AC-27A 第 5/6 条、R0.3-R1c、Phase 0-R 其余项。
- 本 L2 PASS 不改变 AC-27 的 AC-REF。
