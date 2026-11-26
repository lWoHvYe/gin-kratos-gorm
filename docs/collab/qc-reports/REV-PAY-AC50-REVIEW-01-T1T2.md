# REV-PAY-AC50-REVIEW-01-T1T2 — T1/T2 scoped 复审

| 项 | 值 |
|---|---|
| change_id | `PAY-AC50-REVIEW-01`（Parent `PAY-SETTLE-01` 冻结） |
| Tier | **L2（确认）**：TCC 批次 `MANUAL_HOLD`、跨服务 Order/AfterSales 镜像、资金/退款准入；不适用 L1 |
| review_scope | **仅 T1a–T1e、T2a–T2c**（TCC REVIEW 持批/续跑/re-arm、AfterSales canonical/PENDING/8 次、existing-first）；**不审 T3–T8**；不声称 AC-VERIFIED/Release |
| base | **`ff0d912`** |
| code head | **`ff7a847abf07b0e81f7e1bb9d5c0c3f11043c464`**（8 files，+606/−13） |
| head（复审绑定） | **`7b2802450ae1498e0ad8350759c1e369456517c0`**（selfcheck；代码与 ff7a847 一致） |
| 契约输入 | SPEC-0002 v1.11 AC-51/AC-71、AC-69/AC-72；ADR 0.6 §7/§8.5；impact 0.4；Plan/Tasks P5 后 |
| integration_owner | System Architect（契约）；Order（T1）；AfterSales（T2） |
| INV | **INV-45**（permit/commitment 保持）、**INV-51**（REVIEW 有界/不可自动收敛）；INV-52 partial 不在 T1/T2，见 §5 |
| affected_AC | AC-51/AC-71（TCC）、AC-69/AC-72（AfterSales 镜像/任务态）；**不新增 AC-VERIFIED** |
| verification | §1：MySQL 8.4.13 独立 schema 重跑 TCC/AfterSales plain+`-race`、T0 回归；`migrations` diff 空；未改 forbidden 文件 |
| 日期 / Actor / Reviewed-by | 2026-11-26 / `reviewer-r04` / `human (merge pending)` |
| release_status | **NOT RELEASE** |
| overall verdict | **PASS（T1/T2 scoped）**；不可单独上生产（T3–T6 未落地），见 §5 |

---

## 0. 结论

**PASS（有范围）。** 在 `HEAD 7b28024` / `code ff7a847` 上实际重跑：真实 MySQL 8.4.13、每包独立 schema；TCC 与 AfterSales 目标用例及 `-race` 全 PASS；T0 `TestAC50Review` / `TestCancelIfCreated` 回归 PASS；`migrations` diff 为空；`git show --name-only` 未触碰 `tcc.go`/`tcc_payment.go`/`tcc_precheck.go`/`tcc_reconcile.go`/`settlement.go`/`ports.go`/`migrations`，无新字段/RPC。T1 的 batch `MANUAL_HOLD`/0 打款/稳定 token/re-arm 续跑，T2 的 typed 识别/canonical/PENDING@8/非 REVIEW 原样均有真实断言。

**但这是 T1/T2 单元 PASS，不是 AC-50/AC-71/AC-72 全量、不是 Release PASS，也不能单独上生产**：T3（partial/INV-52）、T4（零元终态/legacy 完整矩阵）、T5（审计/metrics/ops）、T6（NATS/DLQ/崩溃）未落地；T1/T2 与 T0 必须作为同一发布单元按序上线。

---

## 1. 实际执行（HEAD 7b28024 / code ff7a847）

```bash
# MySQL 8.4.13，per-package schema
TEST_MYSQL_DSN=.../mall_integration_t1t2_order go test ./internal/services/order -run 'TestTCCReview' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_t1t2_order_race go test -race ./internal/services/order -run 'TestTCCReview' -count=1
TEST_MYSQL_DSN=.../mall_integration_t1t2_as go test ./internal/services/aftersales -run 'TestReview' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_t1t2_as_race go test -race ./internal/services/aftersales -run 'TestReview' -count=1
TEST_MYSQL_DSN=.../mall_integration_t1t2_t0_order go test ./internal/services/order -run 'TestAC50Review' -count=1
TEST_MYSQL_DSN=.../mall_integration_t1t2_t0_ff go test ./internal/services/fulfillment -run 'TestCancelIfCreated' -count=1
git diff --stat ff0d912..HEAD -- migrations
```

| 命令 | 结果 |
|---|---|
| order `TestTCCReview` | **PASS 2.367s**（`TestTCCReviewManualHold`、`TestTCCReviewRearmResumes`） |
| order `-race` 同 | **PASS 3.408s** |
| aftersales `TestReview` | **PASS 2.925s**（Mirror、RetryStaysPending、NonReviewKeepsPermanentFailure、CanonicalParseAndTruncate） |
| aftersales `-race` 同 | **PASS 3.976s** |
| T0 回归 order `TestAC50Review` | **PASS 8.815s** |
| T0 回归 fulfillment `TestCancelIfCreated` | **PASS 1.706s** |
| `migrations` diff | **空** |

原始日志：`.tmp/evidence-ac50-t1t2/reviewer-t1t2.log`、`.tmp/evidence-ac50-t1t2/reviewer-t0-regression.log`（本次审查生成，未入库）。

---

## 2. T1（TCC）

| 检查 | 结论 | 证据 |
|---|---|---|
| T1a `ensureTccRemoteAdmissionTx` 接受 REVIEW，不 “not a legal value” | **PASS** | `tcc_settlement.go:876-880`；`TestTCCReviewManualHold`（返回稳定 manual token，无非法值） |
| T1b `ensureTccApplicationTx` 单调接受 REVIEW，不覆盖已持久结果 | **PASS** | `tcc_settlement.go:839-841`；`TestTCCReviewManualHold`（application `PENDING` + `admission_result=REVIEW_REQUIRED`） |
| T1c batch `MANUAL_HOLD` + canonical `last_error` + app 镜像，同事务 | **PASS（正常路径）** | `refund_admission.go:751-768`；`TestTCCReviewManualHold`（batch MANUAL_HOLD、last_error canonical、app PENDING/REVIEW、0 `Payment.RefundBatch`） |
| T1c 只阻塞本批 | **PASS（代码）/ 测试缺其他批** | `writeTccBatchGroupsTx` 按 batch 短路（`tcc_settlement.go:637-642`）；现有 T1 测试只建 1 个 batch，未断言“其他 batch 不受阻”，见 §5 R2 |
| T1d 自动续跑返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`，0 状态变化 | **PASS** | `tcc_settlement.go:637-642`、`tcc_admission.go:110-123`；`TestTCCReviewManualHold` 二次 `TccCancelMarkPaid`（token、batch/app/admission version 不变、0 打款） |
| T1e re-arm：`MANUAL_HOLD→ADMITTING` + `result→PENDING_REMOTE`，一次 `RefundBatch` | **PASS（CAS/打款）/ 审计仅模拟** | `TestTCCReviewRearmResumes`（CAS、PRE、1 次 `batchCalls`、canonical last_error 保留）；审计工单/日志未在 DB 测试中体现，见 §5 R2 |
| 未改 forbidden `tcc.go`/`tcc_payment.go`/`tcc_precheck.go`/`tcc_reconcile.go`/`settlement.go`/`ports.go` | **PASS** | `git show --name-only ff7a847` 仅 8 files；无这些文件 |
| 无新字段/RPC/迁移 | **PASS** | `migrations` diff 空；无 proto 改动 |

---

## 3. T2（AfterSales）

| 检查 | 结论 | 证据 |
|---|---|---|
| typed 识别（apperr + gRPC ErrorInfo，不嗅探 message） | **PASS** | `aftersales/workers.go:352-398`；`TestReviewTaskMirror` 用 `apperr.GRPC` |
| canonical `after_sale_refund_tasks.last_error`（无 arm、`RVW:`、截断） | **PASS（512）** | `workers.go:404-437,841-861`；`TestReviewTaskMirror`、`TestReviewCanonicalParseAndTruncate` |
| O19 `refund_applications` 0 行 | **PASS（代码）/ 测试缺断言** | AfterSales 只写 task `last_error`，没有 `refund_applications` 插入；`TestReviewTaskMirror` 未显式 `COUNT(refund_applications)==0`，见 §5 R2 |
| O22 REVIEW 任务保持 PENDING；8 次不 `FAILED_PERMANENT` | **PASS** | `workers.go:841-861`、`runRefundBatch` 扫描过滤；`TestReviewTaskRetryStaysPending`（attempts=8、status=PENDING、scanner 不再次 claim） |
| O22b 非 REVIEW 8 次仍 `FAILED_PERMANENT` | **PASS** | `TestReviewTaskNonReviewKeepsPermanentFailure`（status=FAILED、attempts=8、无 REVIEW 镜像） |
| O23 typed/截断/解析拒绝规则 | **PASS（512）** | `TestReviewCanonicalParseAndTruncate`（canonical/truncated/空 ticket/arm/未知 reason/尾随空格）；TCC 1024 列宽未做 DB 级触发，见 §5 R2 |
| O2b/O2c existing-first（Payment 前读裁决，Payment 不可用不重探） | **PASS** | T0 `TestAC50ReviewReplay`；`TestTCCReviewManualHold` 的 replay/down payments 断言 |

---

## 4. INV / AC 与禁止面

- **INV-45**：T1/T2 的 REVIEW 路径不改 permit（`CONSUMED` 保持）、不改 commitment（`RESERVED` 保持）、不 release、不打款；`TestTCCReviewManualHold` 断言。**PASS（T1/T2 路径）**。
- **INV-51**：REVIEW 持久化（admission + batch/application/task 镜像）、自动路径终止、replay 幂等；T1/T2 测试覆盖；T3 partial/INV-52 未交付。**PASS（T1/T2 路径）**。
- **INV-52**：partial 不取消运单**不在 T1/T2**；T3 未落地，T0b 当前对 partial `CONSUMED+NO_WAYBILL` 仍走探测/REVIEW；**不得据 T1/T2 PASS 声称 AC-50/INV-52 全量通过**。
- **禁止面**：`git show --name-only ff7a847` 仅 `refund_admission.go`、`tcc_settlement.go`、`tcc_admission.go`、`aftersales/workers.go`、测试；未改 `migrations/**`、`tcc.go`/`tcc_payment.go`/`tcc_precheck.go`/`tcc_reconcile.go`/`settlement.go`/`ports.go`、proto、gateway、deploy、frontend。**PASS**。

---

## 5. 残余与建议（PASS，最多 3 条）

1. **T1c batch/app 镜像缺 `RowsAffected` fail-closed**：`refund_admission.go:757-768` 对 `refund_batches`/`refund_applications` 的 UPDATE 不检查 0 行；若 batch 不在 `FROZEN/ADMITTING`（或 TCC application 不在 `PENDING`），admission REVIEW 仍会提交，batch 未 `MANUAL_HOLD`/app 未镜像，自动续跑会变成 `Unavailable` 而非稳定 manual token。正常 T1 流程中 batch=ADMITTING/app=PENDING，测试未触发；**生产前**应加 0 行 fail-closed（回滚/Conflict）与 batch 状态不匹配测试。不是当前 scoped PASS 的观测失败，但属 AC-71 fail-closed 缺口。
2. **补 O19/O1b/O23 的缺测**：`TestReviewTaskMirror` 需断言 `refund_applications` 0 行（O19）；`TestTCCReviewManualHold` 需增加“其他 batch 不受阻”用例（O1b）；TCC 1024 列宽截断分支未在 DB 级触发（O23；当前 512 单测覆盖同一 builder）。O16 的“先审计后 CAS”只有模拟 SQL，需在 ops runbook/演练证据中体现。
3. **T3–T6 未落地，禁止生产**：partial/INV-52（T3）、零元人工队列终态（T4）、审计/metrics/ops（T5）、NATS/DLQ/崩溃（T6）未交付；T1/T2 与 T0 必须同一发布单元并按序上线，不得单独发布。

**不声称**：AC-50/51/69/70/71/72 的 AC-VERIFIED（台账仍 AC-REF/DEFERRED）、Release PASS、merge；`PAY-SETTLE-01` 仍冻结，AC-27 仍 AC-REF。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-26
**报告**：`docs/collab/qc-reports/REV-PAY-AC50-REVIEW-01-T1T2.md`
