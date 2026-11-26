# REV-PAY-AC50-REVIEW-01-T3T6-R2 — T3–T6 re-review（fresh TCC REVIEW gate）

| 项 | 值 |
|---|---|
| change_id | `PAY-AC50-REVIEW-01`（Parent `PAY-SETTLE-01` 冻结） |
| Tier | **L2（确认）**：TCC/退款准入、跨服务 Order/AfterSales、资金门禁；不适用 L1 |
| review_scope | **T3-R1/T3-R2 修复**（fresh Tx1 与 Tx2 共用 TCC REVIEW 持批门禁、`source=tcc` partial/legacy 用例）+ **T5/T6 非回归**；不审 T7/T8；不声称 AC-VERIFIED/Release |
| base | **`72575f6`**（T3–T6 FAIL + freeze/fix plan） |
| code head | **`c097e981d0fa45f166e5efc95b2fd783cfc5f8b7`**（2 files） |
| head（复审绑定） | **`9d9f79d8b411c3e1aa27bcc1cda7f3d5ab5fd1c1`**（selfcheck） |
| 契约输入 | SPEC-0002 v1.11 AC-71（`:483-486`）、ADR 0.6 §3.1/W12（`:373-377`）、Plan T3-R1/R2、selfcheck §12 |
| integration_owner | Order（TCC review gate）；AfterSales（T2/T4 不变）；ops（runbook/T7 registration） |
| INV | **INV-45**（permit/commitment 保持）、**INV-51**（REVIEW 有界/不可自动收敛）、**INV-52**（partial 不取消运单）；T5/T6 非回归 |
| affected_AC | AC-50/51/67/69/70/71/72；**不新增 AC-VERIFIED** |
| verification | §1：MySQL 8.4.13 独立 schema；fresh/Tx2/TCC/T0/T5/T6 plain+`-race`；真实 NATS 2.10.29；`migrations` diff 空；禁改文件未动 |
| 日期 / Actor / Reviewed-by | 2026-11-27 / `reviewer-r04` / `human (merge pending)` |
| release_status | **NOT RELEASE** |
| overall verdict | **PASS（T3–T6 scoped）**；T3-R1 BLOCKER CLOSED；T3-R2 Plan pointer/T6b 仍为 Lead-owned 残余，见 §5 |

---

## 0. 结论

**PASS（T3–T6 scoped）。** 上一轮 T3/T4 `[BLOCKER]`（fresh Tx1 TCC REVIEW 绕过 T1c 持批）已关闭：

- `refund_admission.go:537-544` 的 fresh Tx1 REVIEW 现在调用 `persistTCCReviewTx`；`writeAdmissionTx2` 的 Tx2 路径在 `:979-985` 调用同一 helper；helper `:913-941` 同时写 `refund_batches.MANUAL_HOLD` + canonical `last_error`、`refund_applications.admission_result=REVIEW_REQUIRED`，两处 `RowsAffected==1`，0 行即回滚整个事务。
- `TestTCCReviewFreshSourceGate`（`tcc_review_integration_test.go:228`）覆盖 `source=tcc` partial-unclassifiable/legacy-unclassifiable：batch `MANUAL_HOLD`、canonical `last_error`、app `PENDING/REVIEW`、0 probe/0 payment、自动续跑 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`；batch 预置 `PAYING` 的 0 行门禁回滚 admission/commitment。
- 真实 MySQL 8.4.13：order `TestTCCReviewFreshSourceGate|TestTCCReview|TestAC50Review` PASS 35.503s、`-race` 33.779s；T5/T6 的 audit/crash/ship/consumer 用例同轮 PASS；`TestRefundReviewGauges` 0.731s、metrics 0.077s；真实 JetStream `TestRefundAdmittedConsumerReviewAndDuplicate` PASS 2.029s。`migrations` diff 空；`c097e98` 仅改 `refund_admission.go`/`tcc_review_integration_test.go`。

**T5/T6 未被改坏。** 剩余为 **T3-R2 的 Plan/Tasks 指针未改**（T6b 仍写 `order/review_consumer`/`TestReviewConsumer`，该命令 0 tests；实际测试在 fulfillment 包）与 F7b/W8 的既知残余；不构成本次 T3-R1 blocker，但 T7/T8 前必须清理。**T7/T8 未落地，不得单独上线。**

---

## 1. 实际执行（HEAD 9d9f79d / code c097e98）

```bash
# MySQL 8.4.13，独立 schema；NATS 2.10.29
TEST_MYSQL_DSN=.../mall_integration_t3r2_order \
  go test ./internal/services/order -run 'TestTCCReviewFreshSourceGate|TestTCCReview|TestAC50Review' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_t3r2_order_race \
  go test -race ./internal/services/order -run 'TestTCCReviewFreshSourceGate|TestTCCReview|TestAC50Review' -count=1
TEST_MYSQL_DSN=.../mall_integration_t3r2_gauge go test ./internal/services/order -run 'TestRefundReviewGauges' -count=1
go test ./internal/platform/metrics -count=1
TEST_MYSQL_DSN=.../mall_integration_t3r2_ffnat TEST_NATS_URL='nats://127.0.0.1:4222' \
  go test ./internal/services/fulfillment -run 'TestRefundAdmittedConsumerReviewAndDuplicate' -count=1 -v
git diff --stat 72575f6..HEAD -- migrations
```

| 命令 | 结果 |
|---|---|
| order 目标（fresh/Tx2/T0/T5/T6） | **PASS 35.503s**；`TestTCCReviewFreshSourceGate`、`TestTCCReviewTx2BatchGateRollsBack`、`TestTCCReviewManualHold`、`TestTCCReviewRearmResumes`、全部 T0/T5/T6 用例 PASS |
| order `-race` 同 | **PASS 33.779s** |
| `TestRefundReviewGauges` | **PASS 0.731s** |
| `internal/platform/metrics` | **PASS 0.077s** |
| 真实 JetStream `TestRefundAdmittedConsumerReviewAndDuplicate` | **PASS 2.029s** |
| `migrations` diff | **空** |

原始日志：`.tmp/evidence-ac50-t3t6-r2/reviewer-t3r2-order.log`、`reviewer-t5t6-regression.log`（本次审查生成，未入库）。

---

## 2. T3-R1（FRESH TCC REVIEW GATE）——CLOSED

| 检查 | 结论 | 证据 |
|---|---|---|
| fresh Tx1 与 Tx2 共用唯一 review 持久化入口 | **PASS** | `refund_admission.go:537-544`（fresh）与 `:979-985`（Tx2）均调用 `persistTCCReviewTx`；helper `:913-941` |
| batch `MANUAL_HOLD` + canonical `last_error` | **PASS** | helper `:919-929`；`TestTCCReviewFreshSourceGate`（partial reason / legacy reason；canonical 含 `RVW:`） |
| app 镜像 `PENDING/REVIEW_REQUIRED` | **PASS** | helper `:930-940`；测试 `loadTccApplication` 断言 |
| `RowsAffected==1`，batch/app 0 行回滚 | **PASS** | helper `:926-928,937-939`；`TestTCCReviewFreshSourceGate` batch 预置 `PAYING` → admission/commitment 0；Tx2 gate 用例 `TestTCCReviewTx2BatchGateRollsBack` PASS |
| `source=tcc` partial-unclassifiable | **PASS** | `TestTCCReviewFreshSourceGate:251-284`：MANUAL_HOLD、canonical PARTIAL、0 probe/payment、自动续跑 manual token |
| `source=tcc` legacy-unclassifiable | **PASS** | `:286-299`：MANUAL_HOLD、`LEGACY_SHIPMENT_UNKNOWN` |
| 不再泛化 `Unavailable` | **PASS** | `admitTccBatch` 读回 MANUAL_HOLD 返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`；测试断言 `tcc_error` |
| batch_id 缺失/ app 非 PENDING | **PASS（代码 fail-closed）** | helper `:914-917`；app 0 行 `:937-939`；未单独新增两条用例（见 §5 残余） |
| 无 probe/0 payment | **PASS** | 测试 `ff.cancelCalls==0`、`len(payments.batchCalls)==0` |

**T3-R1 BLOCKER：CLOSED。**

---

## 3. T5/T6 非回归（PASS）

| 检查 | 结论 | 证据 |
|---|---|---|
| T5a 审计/version/时间戳/re-arm 保留/O20b | **PASS** | `TestAC50ReviewAuditAndRearmRetention`（同轮 PASS）；`refund_batches.last_error` 保留 |
| T5b O18 gauge/7 天/零元不混入/无新 worker | **PASS** | `TestRefundReviewGauges`、`TestRefundReviewGaugesRegistered`；`order/worker.go` 既有 tick；无 migrations |
| T5c runbook | **PASS** | `docs/ops.md` AC-72 后端段；本次未改 |
| T6a W1–W4/O3 崩溃/竞态 | **PASS** | `TestAC50ReviewCrashReplayConverges`、`TestAC50ReviewShipCancelRace` 同轮 PASS |
| T6b W9/F7b Order no-auto-PRE | **PASS（Order 半侧）** | `TestAC50ReviewConsumerNoAutoPre` 同轮 PASS |
| T6b NATS 重复/DLQ | **PASS（实际测试）** | `internal/services/fulfillment/review_consumer_integration_test.go:22`，真实 JetStream |
| T6c DLQ replay | **PASS** | 同上 |
| T6d W11/W13 re-arm | **PASS（模拟）** | `TestAC50ReviewAuditAndRearmRetention`；ops 工单/日志非 DB |
| 禁改面/迁移/新字段/RPC/worker | **PASS** | `c097e98` 仅 2 files；`migrations` diff 空；T5/T6 未改 |

---

## 4. Invariant / AC 边界

- **INV-45**：TCC review 不改变 permit（`CONSUMED` 保持）/commitment（`RESERVED` 保持）——fresh gate 测试的 admission/commitment 只插入，不 release；0 行回滚。**PASS**。
- **INV-51**：`REVIEW_REQUIRED` 持久化 + batch/app 同事务持批；自动续跑不收敛、不自动打款；T3-R1 测试。**PASS（T3/T4 路径）**。
- **INV-52**：partial 不调用 `CancelIfCreated`；`TestAC50ReviewPartial*` 与 TCC fresh partial 的 `cancelCalls==0`；T3 测试。**PASS（T3 路径）**。
- **AC-71/W12**：MANUAL_HOLD + canonical reason + stable token；**PASS**。
- **AC-72/T5**：后端 gauge/runbook；**PASS（后端段）**；用户入口/告警仍 DEFERRED。

---

## 5. 残余 / T7 登记

1. **`[MAJOR]` T3-R2 Plan/Tasks 指针未改**：`plan.md:59` 仍写 `order/review_consumer_integration_test.go` + `TestReviewConsumer`，`tasks.md:73` 同；该命令实际 `[no tests to run]`。实际测试是 `internal/services/fulfillment/review_consumer_integration_test.go::TestRefundAdmittedConsumerReviewAndDuplicate`（本次 PASS）。Lead 需在 Plan/Tasks 更正（T7/T8 前），否则 T3-R2 不能称完成。
2. **`[MINOR]` 门禁用例缺口**：fresh gate 覆盖 batch 0 行；`app` 0 行（app 非 PENDING）与 `batch_id=""` 只有代码 fail-closed，未有独立测试；建议 T7 补两条（不需改实现）。
3. **F7b/W8 残余（接受）**：
   - F7b 真实迟到 `order.paid` placeholder **未构造**；本 L2 未改 Fulfillment settlement consumer，REVIEW 无终态 outbox/Order 无 scanner；Order 侧 no-auto-PRE 3 次 replay 已覆盖。接受为 **T7 明示残余**，不得声称 F7b 全链路验证。
   - W8 `PENDING_REMOTE` 永久悬挂归 **AC-27-LIVE-01**；本 L2 不实现；接受。
4. **T7/T8 未落地**：AC-72 用户入口/告警、ops 演练、证据矩阵/回归收口未完成；T3–T6 不得单独上线，`NOT RELEASE`。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-27
**报告**：`docs/collab/qc-reports/REV-PAY-AC50-REVIEW-01-T3T6-R2.md`
