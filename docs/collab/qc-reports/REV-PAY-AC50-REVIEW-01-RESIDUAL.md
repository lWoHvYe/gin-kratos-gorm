# REV-PAY-AC50-REVIEW-01-RESIDUAL — T7-R1..R3 残余硬化复审

| 项 | 值 |
|---|---|
| change_id | `PAY-AC50-REVIEW-01`（Parent `PAY-SETTLE-01` 冻结） |
| Tier | **L2（确认）**：test-only 残余硬化；不改变 T0–T8 scoped PASS |
| review_scope | **仅 T7-R1/T7-R2/T7-R3**（F7b 半链/`[BLOCKED]`、TCC 1024/gate、unknown status/O19/O1b 隔离）；**不重开 T0–T8** |
| base | **`b731a9f`** |
| code head | **`d90daf09c3f391ca207df68f02d57a38b7cf781b`**（4 个 `_test.go`，+305） |
| head（复审绑定） | **`d119894f752c26932a08343f608256b7e2b945b1`**（selfcheck） |
| 契约输入 | SPEC v1.11 AC-50/51/69/70/71/72；ADR 0.6；最终 verdict 残余；selfcheck §14 |
| INV | **INV-45/51/52**（T0–T8 结论不变）；INV-44/46/50 保持 AC-REF/DEFERRED |
| verification | §1：MySQL 8.4.13 独立 schema + NATS 2.10.29；order/after-sales/NATS/metrics；`migrations` diff 空；仅 4 个测试文件 |
| 日期 / Actor / Reviewed-by | 2026-11-27 / `reviewer-r04` / `human (merge pending)` |
| release_status | **NOT RELEASE** |
| overall verdict | **PASS（T7 residual scoped）**；T0–T8 scoped PASS 维持；F7b 全链路 `[BLOCKED]` 接受为残余；无台账状态变化 |

---

## 0. 结论

**PASS（残余硬化 scoped）。** `d90daf0` 只改 4 个 `_test.go`，未改生产代码/迁移/deploy/frontend/specs/adr/台账；真实 MySQL 8.4.13 + NATS 2.10.29 重跑全部 PASS：

- **T7-R2**：TCC 1024 canonical 截断/parser、application mirror 0 行 gate、空 `batch_id` fail-closed；均按现有生产代码的真实行为断言。
- **T7-R3**：unknown/future probe status → `Unavailable` 且 0 outbox/release；普通售后 REVIEW `refund_applications` 0 行（O19）；TCC 双批 A `MANUAL_HOLD` 不阻塞 B（O1b，1 次 `RefundBatch`）。
- **T7-R1**：Fulfillment 半链使用**真实** `RunOrderPaidConsumer` 创建 `CREATED` placeholder，REVIEW/POST no-op、真实 PRE 取消；**全链路 F7b `[BLOCKED]`**——跨 Order/Fulfillment 双服务+双库+NATS 的 harness 不存在，且单测试跨服务 import 违反静态守卫；未用 fixture 冒充。接受为 T7 残余（owner Lead/QA，expiry 2026-12-31）。

---

## 1. 实际执行（HEAD d119894 / code d90daf0）

```bash
# MySQL 8.4.13，独立 schema
TEST_MYSQL_DSN=.../mall_integration_res_order go test ./internal/services/order -run 'TestAC50Review|TestTCCReview' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_res_order_race go test -race ./internal/services/order -run 'TestAC50Review|TestTCCReview' -count=1
TEST_MYSQL_DSN=.../mall_integration_res_as go test ./internal/services/aftersales -run 'TestReview' -count=1 -v
go test ./internal/platform/metrics -count=1
TEST_MYSQL_DSN=.../mall_integration_res_ffnat TEST_NATS_URL='nats://127.0.0.1:4222' \
  go test ./internal/services/fulfillment -run 'TestOrderPaidCreatesRealPlaceholderThenReviewNoop|TestRefundAdmittedConsumerReviewAndDuplicate' -count=1 -v
```

| 命令 | 结果 |
|---|---|
| order 目标（31 tests） | **PASS 23.580s** |
| order `-race` 同 | **PASS 26.259s** |
| aftersales `TestReview` | **PASS 2.395s** |
| `internal/platform/metrics` | **PASS 0.077s** |
| 真实 NATS（2 tests） | **PASS 4.465s**（`TestOrderPaidCreatesRealPlaceholderThenReviewNoop` 2.38s） |
| `migrations` diff | **空** |
| 文件面 | `git show --name-only d90daf0` 仅 4 个 `_test.go` |

原始日志：`.tmp/evidence-ac50-residual/reviewer-residual.log`、`reviewer-residual-nats.log`（本次审查生成，未入库）。

---

## 2. T7-R1（F7b）

| 检查 | 结论 | 证据 |
|---|---|---|
| Fulfillment 半链：正常 `RunOrderPaidConsumer` 创建真实 `CREATED` placeholder（非 fixture） | **PASS** | `fulfillment/review_consumer_integration_test.go` `TestOrderPaidCreatesRealPlaceholderThenReviewNoop`；唯一 order/refund；NATS 2.10.29 |
| REVIEW/POST 事件不改真实 placeholder | **PASS** | 同上（REVIEW no-op；POST no-op） |
| 真实 `PRE_SHIP_REVOKED` 取消真实 placeholder | **PASS** | 同上 |
| Order 半链 PENDING→CANCELLED→PRE / partial no-probe / REVIEW no-auto-PRE | **PASS（模拟既有断言）** | T0/T3/O4e/O11d/no-auto-PRE 用例；本单元未新增跨服务 e2e |
| **F7b 全链路** | **`[BLOCKED]`（接受残余）** | 需要 Order + Fulfillment 两个进程、双 MySQL、gRPC + NATS 同测；无跨服务 harness；单测试 import 两 service 包违反静态守卫；不以 fixture 冒充 |
| `[BLOCKED]` 依赖/owner/expiry | **成立/登记** | owner Lead/QA；expiry 2026-12-31；T7 残余；T8 不据此声称 F7b 全链路 |

**判定：F7b `[BLOCKED]` 理由成立，接受为残余；不得声称 F7b 全链路已验证。**

---

## 3. T7-R2

| 检查 | 结论 | 证据 |
|---|---|---|
| TCC 1024 canonical 截断（≤1024、前缀、` truncated=1`、reason/ticket 不重复） | **PASS** | `tcc_review_integration_test.go` `TestTCCReviewLastErrorTruncation1024` |
| 1024 宽 parser 解析（width-agnostic） | **PASS** | `aftersales/review_task_integration_test.go` `TestReviewCanonicalParseAndTruncate` 新增 1024 用例 |
| application mirror 0 行 → 回滚 admission/commitment，batch 不置 MANUAL_HOLD | **PASS** | `TestTCCReviewGateApplicationMirrorZeroRows`（admission POST/PENDING v1、batch ADMITTING v1） |
| 空 `batch_id` → Tx1 写前 fail-closed，0 admission/commitment | **PASS** | `TestTCCReviewEmptyBatchIDFailsClosed` |

---

## 4. T7-R3

| 检查 | 结论 | 证据 |
|---|---|---|
| unknown/future `CancelIfCreated` status → `Unavailable`；0 PRE/POST/REVIEW/outbox/release | **PASS** | `TestAC50ReviewUnknownProbeStatus` |
| O19 普通售后 REVIEW `refund_applications` 0 行 | **PASS** | `TestAC50ReviewOrdinaryAfterSaleNoApplicationRow` |
| O1b 双批隔离：A REVIEW `MANUAL_HOLD` 不阻塞 B（CANCELLED 继续 PAYING/DONE；1 次 `RefundBatch`） | **PASS** | `TestTCCReviewOtherBatchUnaffected` |

---

## 5. 台账 / L2 状态

- **AC 台账无状态变化**：T7-R2/R3 是已有 AC-50/69/70/71/72 证据的补充断言，不改变既有 AC-VERIFIED/AC-REF 状态。当前仍为：AC-50/69/70/71/72-backend **AC-VERIFIED**；AC-51/61/62/65/67 **AC-REF**；AC-72 `REVIEW-REQUIRED-ALERTS-01`/`REVIEW-REQUIRED-ENTRY-01` **DEFERRED**；AC-27 **AC-REF**；AC-21/25/31/32 **AC-REF**；AC-14 既有 AC-VERIFIED。未删/未重编号/未批量续期。
- **L2 维持 T0–T8 scoped PASS**；`NOT RELEASE`，不声称 merge/P0 closure/Release。
- 残余：F7b 全链路 `[BLOCKED]`（T7 登记，owner Lead/QA，2026-12-31）；W8 归 **AC-27-LIVE-01**；ALERTS/ENTRY 仍 DEFERRED；T6b/T6c Plan 文件路径仍写 `order/review_consumer`（实际测试在 fulfillment，Lead-owned 文档指针，不影响本次测试证据）。
- 交回 `PAY-SETTLE-01`：AC-27 仍 AC-REF；Phase 0-R 未闭合；本 L2 不关闭 Parent/T2/T3/T4.4/T5。

---

## 6. 宪法 / Governance

| 条款 | 结论 | 依据 |
|---|---|---|
| I.1/I.4 | **PASS** | 仅测试硬化；未加放行分支/未改生产代码 |
| I.2 Fail-Closed | **PASS** | unknown status→Unavailable；app 0 行/empty batch→回滚 |
| I.3 契约优先 | **PASS** | 按 SPEC/ADR；无迁移/RPC/字段 |
| II.1/II.3 | **PASS** | REVIEW 不冒充终态；双批隔离、NATS 重复/DLQ 幂等 |
| III.2/III.4 | **PASS** | 真实 MySQL/NATS；无新 worker/迁移 |
| IV.1/IV.2 | **PASS** | 精确 HEAD；真实日志；`-race`/NATS |
| IV.4 | **PASS** | `NOT RELEASE`；不声称 Release |
| Workflow 9 / Governance 4 | **PASS** | 残余 test-only 复审；人类 merge pending |

**`[ARCHITECTURAL_FLAW]`：无。**

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-27
**报告**：`docs/collab/qc-reports/REV-PAY-AC50-REVIEW-01-RESIDUAL.md`
