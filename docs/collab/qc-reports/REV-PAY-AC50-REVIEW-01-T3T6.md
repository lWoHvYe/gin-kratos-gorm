# REV-PAY-AC50-REVIEW-01-T3T6 — T3/T4/T5/T6 scoped 复审

| 项 | 值 |
|---|---|
| change_id | `PAY-AC50-REVIEW-01`（Parent `PAY-SETTLE-01` 冻结） |
| Tier | **L2（确认）**：部分金额/零元/legacy 准入、TCC/AfterSales 镜像、审计/metrics、NATS consumer；不适用 L1 |
| review_scope | **T3a/T3b、T4a/T4b、T5a–T5c、T6a–T6d**；**不审 T7/T8**；不声称 AC-VERIFIED/Release |
| base | **`30c6f16`**（T1/T2 scoped PASS） |
| code heads | **`13a586a`**（T3/T4 + T1c gate，6 files） + **`f1946e7`**（T5/T6，7 files） |
| head（复审绑定） | **`849cf78`**（selfcheck）；审查期间工作树 HEAD 前进到 `9e53755`（`git diff 849cf78..9e53755` 仅 governance docs，代码不变）；重跑在 9e53755 完成，首轮 849cf78 同样 PASS |
| 契约输入 | SPEC-0002 v1.11 AC-50/AC-51/AC-67/AC-69/AC-70/AC-71/AC-72；ADR 0.6 §3.1/§3.4/§6/§7/§8.5；impact 0.4；Plan/Tasks P5 后 |
| integration_owner | System Architect（契约）；Order（T3/T4/T6）；AfterSales（T2/T4）；ops（T5c/runbook） |
| INV | **INV-45**（permit/commitment 保持）、**INV-51**（REVIEW 有界/不可自动收敛）、**INV-52**（partial 不取消运单）；INV-50（零元）见 T4a |
| affected_AC | AC-50/51/67/69/70/71/72；**不新增 AC-VERIFIED** |
| verification | §1：MySQL 8.4.13 + NATS 2.10、独立 schema 重跑 order/AfterSales/metrics/NATS；`migrations` diff 空；未改 forbidden 面 |
| 日期 / Actor / Reviewed-by | 2026-11-27 / `reviewer-r04` / `human (merge pending)` |
| release_status | **NOT RELEASE** |
| overall verdict | **FAIL（T3/T4 有 `[BLOCKER]`）；T5/T6 验证 PASS**；不可单独上线，见 §5 |

---

## 0. 结论

**T3/T4 FAIL（scoped）；T5/T6 PASS。** 在 `HEAD 849cf78` / code `13a586a`+`f1946e7` 上重跑：MySQL 8.4.13 + NATS 2.10；order `TestAC50Review|TestTCCReview`、AfterSales `TestReview|TestZero`、`TestRefundReviewGauges`、`TestRefundAdmittedConsumerReviewAndDuplicate`（真实 JetStream）全 PASS；`migrations` diff 为空；**T3/T4/T5/T6 代码提交**（`13a586a`/`f1946e7`）未改 `deploy/frontend/specs/adr/台账/tcc.go/tcc_payment.go/tcc_precheck.go/tcc_reconcile.go/settlement.go/ports.go`（`docs/review-tracking.md`/tasks 的 HEAD 变更为并发 Lead/governance，不属于本单元）。T5/T6 的审计、gauge、runbook、崩溃/竞态、NATS/DLQ 证据成立。

**但 T3/T4 新引入的 fresh Tx1 REVIEW 路径绕过了 T1c 的人工持批门禁**：`AdmitRefund` 在 `refund_admission.go:544-551` 直接返回 REVIEW conflict，而 `refund_batches.MANUAL_HOLD`/`refund_applications.admission_result` 镜像只在 `writeAdmissionTx2`（`:940-970`，T1c gate）里做。对 `source=tcc` 的 partial-unclassifiable（`:469-471`）或 legacy/no-permit unclassifiable（`:490-492`）fresh 申请，会提交 REVIEW admission + commitment，但 batch 仍 `ADMITTING`、app 未镜像、`last_error` 未写；`admitTccBatch` 随后返回 `Unavailable` 而非 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`，违反 AC-71/ADR §3.1 W12。T3/T4 测试只覆盖 `source=aftersale`，未覆盖 TCC fresh review。

---

## 1. 实际执行（HEAD 849cf78 / 9e53755，code 13a586a + f1946e7）

```bash
# MySQL 8.4.13，独立 schema；NATS 2.10.29
TEST_MYSQL_DSN=.../mall_integration_t3t6_order go test ./internal/services/order -run 'TestAC50Review|TestTCCReview' -count=1
TEST_MYSQL_DSN=.../mall_integration_t3t6_order_race go test -race ./internal/services/order -run 'TestAC50Review|TestTCCReview' -count=1
TEST_MYSQL_DSN=.../mall_integration_t3t6_as go test ./internal/services/aftersales -run 'TestReview|TestZero' -count=1
TEST_MYSQL_DSN=.../mall_integration_t3t6_as_race go test -race ./internal/services/aftersales -run 'TestReview|TestZero' -count=1
TEST_MYSQL_DSN=.../mall_integration_t3t6_gauge go test ./internal/services/order -run 'TestRefundReviewGauges' -count=1
go test ./internal/platform/metrics -count=1
TEST_MYSQL_DSN=.../mall_integration_t3t6_ffnat TEST_NATS_URL='nats://127.0.0.1:4222' \
  go test ./internal/services/fulfillment -run 'TestRefundAdmittedConsumerReviewAndDuplicate' -count=1 -v
```

| 命令 | 结果 |
|---|---|
| order `TestAC50Review|TestTCCReview` | **PASS**（首轮 849cf78 32.080s；9e53755 21.701s） |
| order `-race` 同 | **PASS**（22.482s / 20.212s） |
| AfterSales `TestReview|TestZero` | **PASS**（8.132s / 8.089s） |
| AfterSales `-race` 同 | **PASS**（9.309s / 8.497s） |
| `TestRefundReviewGauges`（order） | **PASS 0.840s** |
| `internal/platform/metrics` | **PASS 0.084s** |
| 真实 NATS `TestRefundAdmittedConsumerReviewAndDuplicate` | **PASS 2.370s**（fulfillment 包） |
| `migrations` diff | **空** |

原始日志：`.tmp/evidence-ac50-t3t6/reviewer-t3t6.log`、`reviewer-t6b-nats.log`、`reviewer-t5b-gauge.log`、`reviewer-t6b-command-check.log`（本次审查生成，未入库）。

---

## 2. T3/T4（FAIL）

| 检查 | 结论 | 证据 |
|---|---|---|
| O4a/O4b partial 未离仓 fresh 零写入、`cancelCalls==0` | **PASS（普通售后）** | `TestAC50ReviewPartialUnshipped`、`TestAC50ReviewPartialConsumedUnshipped`；`refund_admission.go:467-468` |
| O4c partial 已离仓 → POST，1 commitment，0 probe/release | **PASS** | `TestAC50ReviewPartialShipped`；`:472-474` |
| O4d partial 不可判定 → REVIEW + 1 RESERVED，`reason=PARTIAL_AMOUNT_UNCLASSIFIABLE` | **PASS（普通售后）** | `TestAC50ReviewPartialUnclassifiable`；`:469-471` |
| O4e preseed 保留既有 admission/commitment，不新增写入 | **PASS** | `TestAC50ReviewPartialPreseedUnshipped`；`resumePendingAdmission:563-600` |
| O5c 整组 CONSUMED + 投影 SHIPPED → POST 不探测 | **PASS** | `TestAC50ReviewWholeGroupProjectionShipped`；`:475-478` |
| O10b legacy 无 permit + 不可判定 → REVIEW + 1 RESERVED | **PASS（普通售后）** | `TestAC50ReviewLegacyUnclassifiable`；`:490-492` |
| W10 部分金额/legacy 不取消运单 | **PASS（普通售后）** | 上述测试 fake CancelIfCreated 从不调用 |
| **TCC fresh REVIEW 的 T1c 持批/镜像** | **FAIL（`[BLOCKER]`）** | `refund_admission.go:544-551` fresh Tx1 REVIEW 直接返回 conflict；T1c batch/app 镜像仅在 `writeAdmissionTx2:940-970`。TCC partial-unclass（`:469-471`）/legacy-unclass（`:490-492`）fresh 路径不写 `refund_batches.MANUAL_HOLD`、不写 app `admission_result`、不写 `last_error`；`admitTccBatch` 读回 batch 非 MANUAL_HOLD → `Unavailable`，非稳定 token |
| T4a 零元 O7/O7b/O8/O21 | **PASS** | `TestAC50ReviewZeroNotPersisted`、`TestAC50ReviewZeroTransientNoWrites`、`TestAC50ReviewZeroNoWaybillThenCancelled`；0 admission/S19/REVIEW、0 Payment；CANCELLED→PRE+DONE |
| T4b legacy O10/O10b | **PASS（普通售后）** | `TestAC50ReviewLegacy`、`TestAC50ReviewLegacyUnclassifiable`；不猜 payment |
| 既有 partial 测试金额调整 | **PASS** | `refund_admission_integration_test.go`/`refund_release_integration_test.go` 改为整组金额；全包/目标 PASS |
| T1c `RowsAffected` gate | **PASS（仅 Tx2 路径）** | `refund_admission.go:940-970`；`TestTCCReviewTx2BatchGateRollsBack`；但 fresh Tx1 未复用此 gate |

**FAIL 条款**：SPEC-0002 v1.11 AC-71（`:483-486`）要求 `REVIEW_REQUIRED` 持久化时 batch `MANUAL_HOLD`、app `PENDING/REVIEW`、可查询原因；ADR 0.6 §3.1 `:373-377` 要求同一 Tx2/Tx1 写 batch/app/admission；当前 fresh Tx1 REVIEW 路径只写 admission+commitment，不满足。修复必须是**一处 review 持久化入口**（fresh Tx1 与 Tx2 共用 T1c gate），不是在 fresh 分支里再加一个散落 if；并补 `source=tcc` partial-unclass / legacy-unclass 测试。

---

## 3. T5（PASS）

| 检查 | 结论 | 证据 |
|---|---|---|
| T5a O20/O20b：admission 行不删、`version`/`created_at`/`updated_at`、re-arm 保留 canonical、reconcile 不清 batch | **PASS** | `TestAC50ReviewAuditAndRearmRetention`；`refund_batches.last_error` 保留 |
| T5a O16/W11 re-arm：版本 CAS、审计先于清除、一次 `RefundBatch` | **PASS（测试模拟 CAS/一次打款；审计为 ops 工单/日志，非 DB 表）** | 同上 |
| T5b O18：2 gauge、7 天查询、零元不混入、无新 worker | **PASS** | `internal/platform/metrics/metrics.go:83-92`；`order/worker.go:56-74`（既有 tick 只读）；`TestRefundReviewGauges`、`TestRefundReviewGaugesRegistered` |
| T5c runbook：发现/backlog/7d/TCC MANUAL_HOLD/售后 last_error/re-arm/存量 PRE | **PASS** | `docs/ops.md` 新增 AC-72 后端段；owner ops/expiry 2026-12-31 |
| 未新增 worker/字段/迁移 | **PASS** | only `worker.go` tick；`migrations` diff 空 |

---

## 4. T6（PASS，含证据路径偏差）

| 检查 | 结论 | 证据 |
|---|---|---|
| T6a W1–W4/O3 崩溃/竞态 | **PASS** | `TestAC50ReviewCrashReplayConverges`、`TestAC50ReviewShipCancelRace`（单终态、release 与结果一致） |
| T6b W9/F7b Order no-auto-PRE | **PASS** | `TestAC50ReviewConsumerNoAutoPre`（REVIEW 后 replay 不写 outbox/release，commitment 仍 RESERVED） |
| T6b NATS 重复 PRE/REVIEW/POST no-op/DLQ replay | **PASS（实际测试）** | `internal/services/fulfillment/review_consumer_integration_test.go:22` `TestRefundAdmittedConsumerReviewAndDuplicate`；真实 JetStream 2.10.29 |
| T6c DLQ | **PASS** | 同上，非法 JSON 进 DLQ，DLQ replay no-op |
| T6d W11/W13 re-arm | **PASS（模拟）** | `TestAC50ReviewAuditAndRearmRetention`；ops 工单/日志非 DB 证据 |
| **任务给定 T6b 命令** | **`[MAJOR]` 证据路径偏差** | `go test ./internal/services/order -run 'TestReviewConsumer'` → `[no tests to run]`（`.tmp/evidence-ac50-t3t6/reviewer-t6b-command-check.log`）；实际测试在 `fulfillment` 包、名为 `TestRefundAdmittedConsumerReviewAndDuplicate`。Plan/T6b 的 `order/review_consumer` 指针需更正 |
| F7b 迟到 placeholder 真实链路 | **未构造（selfcheck 已声明）**；仅 Order no-auto-PRE 覆盖 | 不构成本次 T6 scoped blocker，但不得声称 F7b 全链路已验证 |

---

## 5. 残余 / 修复要求

1. **`[BLOCKER]` T3/T4 fresh Tx1 REVIEW 绕 T1c**（见 §2）：把 review 持久化收口为单一 helper，fresh Tx1 与 `writeAdmissionTx2` 共用；TCC batch `MANUAL_HOLD`/app 镜像必须在同一事务；补 `source=tcc` partial-unclass 与 legacy-unclass 用例。修复后重跑 T3/T4/T1。
2. **`[MAJOR]` T6b evidence path/command**：实际测试 `fulfillment/TestRefundAdmittedConsumerReviewAndDuplicate`；`order -run TestReviewConsumer` 选不到测试，不能作为 PASS 证据。更新 Plan/T6b 指针；F7b 真实 placeholder 路径仍为空。
3. **T5/T6 仍非生产**：T7/T8 未落地；T5c ops 告警/入口、T6b F7b/W8、TCC 1024 截断等仍为 DEFERRED。修复 T3/T4 blocker 前，T3–T6 不得单独上线；`NOT RELEASE`、不关 AC-27/Phase 0-R。

**不声称**：AC-50/51/67/69/70/71/72 的 AC-VERIFIED（仍 AC-REF/DEFERRED）、Release PASS、merge；`PAY-SETTLE-01` 仍冻结。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-27
**报告**：`docs/collab/qc-reports/REV-PAY-AC50-REVIEW-01-T3T6.md`
