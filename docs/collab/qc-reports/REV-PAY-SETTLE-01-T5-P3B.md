# REV-PAY-SETTLE-01-T5-P3B — ADR-0011 Phase 3b（legacy release+receipt → ack → Payment → EXPIRED_REFUND）审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（T5） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | `internal/services/order/legacy_resolution_worker.go`（COMPENSATING scan/lease、release+receipt、ack 屏障、Payment、EXPIRED_REFUND）；`legacy_resolution_worker_test.go`（2 个新测试 + 既有 expiry 加 item）。**不审** T5.2/3/5/6/7。 |
| base | **`098e7fe`** |
| head | **工作树 @ `098e7fe`（未提交）**：worker.go +141/-6；test +101；无 migration/api/inventory 改动。 |
| spec_version | **SPEC-0002 v1.11**（AC-63/INV-46） |
| adr_revision | **ADR-0007 Rev 2.3 / Amd1**；**ADR-0011**（Phase 1/2/3a 已 PASS） |
| baseline_impact | **有**：legacy 24h 补偿在 ack 后才 Payment；release+receipt 同事务；Payment 幂等；EXPIRED_REFUND；多组人工；无 migration/Inventory。 |
| INV | **INV-46**、**AC-63**（COMPENSATING→EXPIRED_REFUND、Payment last、失败保持）；AC-63 可升级。 |
| verification | **命令实测**：`TEST_MYSQL_DSN=... go test ./internal/services/order -run 'TestLegacy\|TestT5RealMigration\|TestAC60\|TestAC63\|TestRefundReleaseAck\|TestReleaseCommitment' -count=1` → **PASS（41.224s）**；`go vet ./internal/services/order ./cmd/order` → 0；`git diff 098e7fe -- migrations api internal/services/inventory` = **0**。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **PASS（Phase 3b）**：PENDING/FAILED 零 Payment、RELEASED 才 Payment；release+receipt 同事务；refund_applications 先写；单组限制；Payment last/幂等；EXPIRED_REFUND；**Phase 3b 可合入；AC-63 已验证（单组自动 + 多组人工 ADR-0011 A）；AC-60 仍部分；T5.2/3/5/6/7 未做**。0 阻断；NB-1…NB-6。 |

---

## 0. 结论

1. **ack 屏障**：`compensateLegacyResolution` 读 `refund_release_receipts`：`PENDING`/`FAILED` → `errLegacyCompensationBlocked`（零 Payment）；`RELEASED` → `payLegacyCompensation`；无 receipt → 单组写 `refund_applications(legacy_expired)` + release 请求（source `legacy_24h`）+ PENDING receipt（同一 tx），返回 blocked。
2. **release+receipt 同事务**：`db.WithTx` 内逐个 `ensureLegacyRefundApplicationTx`、`appendRefundReleaseTx(first.refundID, ..., legacy_24h)`、`insertRefundReleaseReceiptTx`；Payment 前已持久化 intent/release/receipt。
3. **单组限制**：`groupCount != 1` → blocked；无组/多组不默认 G1、不 release，保持 COMPENSATING/人工；shipped 仍优先 `EXPIRED_FINANCE`。
4. **Payment last/幂等**：`payLegacyCompensation` 对每个 captured intent `Payment.Refund(IdempotencyKey=pay:<payment_no>:legacy_expired, NonEffectiveCapture=true, amount=captured-refunded)`；全部成功后 app DONE；再 `terminalLegacyResolution(EXPIRED_REFUND)`；失败 `Unavailable` → COMPENSATING，下一轮重试。
5. **测试**：`TestLegacy24hAckThenPaymentAndExpiredRefund`（ack 前零 Payment、ack 后 1 次、EXPIRED_REFUND、app DONE）；`TestLegacy24hFailedAckKeepsCompensatingAndPaysNothing`（FAILED ack 零 Payment、COMPENSATING）；既有 expiry 测试加 item。全量 PASS。
6. **AC-63**：single-group 全链路（unique/ambiguous/zero/24h/COMPENSATING/EXPIRED_REFUND/Payment）+ 多组人工（ADR-0011 A）可作为 AC-63 的已验证范围；**AC-60 仍部分**（无售后自动分配/Inventory HELD/缺货未接线）；T5.2/3/5/6/7 未做。

---

## 1. 审查输入与方法

- 读：worker diff、新测试、`appendRefundReleaseTx`/`insertRefundReleaseReceiptTx`/`applyRefundReleaseAck`、`terminalLegacyResolution`、`orderGroupCount`、`shipped` 分支。
- 跑：全量 order 回归、vet、边界。
- 未改代码/契约/迁移；Spec AC-63 验收表按 PASS 更新；未 commit/merge。

---

## 2. 测试

```text
$ TEST_MYSQL_DSN=... go test ./internal/services/order -run 'TestLegacy|TestT5RealMigration|TestAC60|TestAC63|TestRefundReleaseAck|TestReleaseCommitment' -count=1
ok   ...order  41.224s

$ go vet ./internal/services/order ./cmd/order   # 0
$ git diff 098e7fe -- migrations api internal/services/inventory | wc -l   # 0
```

- 新测试覆盖：ack 前零 Payment；ack 后 1 次 Payment（idempotency key 正确）；EXPIRED_REFUND；app DONE；FAILED ack 零 Payment、COMPENSATING。
- 既有 `TestLegacyExpiryBranches` 的 captured unshipped 加 `seedLegacyItem`，保证 release 路径有 items。

---

## 3. ack 屏障

- PENDING → blocked；FAILED → blocked；RELEASED → Payment；无 receipt → 同事务写 release+receipt，然后 blocked。
- 测试：第一次 `processExpiredLegacyResolution` 后 `refundCalls=0`、receipt=1、COMPENSATING；`applyRefundReleaseAck(RELEASED)` 后第二次 Payment=1。
- FAILED：`applyRefundReleaseAck(FAILED)` 后第二次 Payment=0、COMPENSATING；测试断言。
- **结论**：ack 前不 Payment 是真实 DB receipt 屏障，不是仅事件。

---

## 4. release+receipt 同事务 / intent / 单组

- `db.WithTx`：`ensureLegacyRefundApplicationTx`（source `legacy_expired`、group_no NULL、non_effective=1、PENDING）→ `appendRefundReleaseTx(first.refundID, orderNo, groupNo, legacy_24h)` → `insertRefundReleaseReceiptTx(releaseID, first.refundID, orderNo)`。
- 单组：`orderGroupCount != 1` → blocked；`groupNo=groups[0]`；无组 groupNo="" → blocked；不默认 G1。
- 多组/无组不 release，保持 COMPENSATING/人工。
- **NB-1**：release 请求只对第一个 captured refund 的 `refund_id`；其余 refund 只在 `refund_applications`，Payment 时全付；release 是 order-level full reservation，语义可接受，但“receipt/refund_id 只对应第一个”需在 Phase 3b 文档化。

---

## 5. Payment last / 幂等 / app DONE + EXPIRED_REFUND

- `legacyCapturedRefunds`：`captured-refunded>0`、payment_no 非空；refundID=`pay:<payment_no>:legacy_expired`。
- `Payment.Refund`：amount=captured-refunded、Reason=`legacy_24h_expiry`、IdempotencyKey=refundID、NonEffectiveCapture=true。
- 全部成功后 `db.WithTx` 把 app `PENDING→DONE`；再 `terminalLegacyResolution(EXPIRED_REFUND)`。
- 失败：`apperr.Unavailable` → 返回；worker 保持 COMPENSATING；下一轮重试；Payment 用同一 idempotency key，不会双重打款。
- **NB-2**：app DONE 与 EXPIRED_REFUND 不是同一 tx；若 app DONE 后 terminal 失败/崩溃，下一轮会重跑 Payment（幂等）+ app DONE（no-op）+ terminal；收敛但非原子。
- **NB-3**：Payment 错误一律 `Unavailable`；永久错误（金额/参数）也会无限重试；且未按 ADR §4.4.3 step 7 在重试前用 `GetRefundByKey` 对账；目前依赖 Payment 的 idempotency key。
- **NB-4**：`payLegacyCompensation` 重试会重新调用所有 refund 的 `Refund`；幂等键保证不重复，但没有“部分成功只补未完成”的短路；可接受但需 Payment 幂等成立。

---

## 6. COMPENSATING scan/lease / 重复 Payment

- `expireLegacyResolutions` 扫描 `state=COMPENSATING`（不限 expires_at）；`claimLegacyResolution` 也接受 COMPENSATING；lease 30s；每轮重试。
- PENDING/FAILED 的 COMPENSATING 会每轮被扫描并 blocked；FAILED 一直保持 COMPENSATING（人工/告警），不会 Payment；这是 ADR 的失败语义，但需要 ops 告警。
- Payment 失败重试靠 idempotency key，不会重复打款；release 请求/ receipt 无重复（同事务 + receipt PK）。
- shipped 优先：既有 `shipped`/`EXPIRED_FINANCE` 分支在 release 之前，测试覆盖。
- **NB-5**：FAILED/PENDING 的 COMPENSATING 没有专门告警/计数（scan 会反复 block）；建议加指标/告警，避免静默循环。

---

## 7. AC-63 / AC-60

- **AC-63 可标已验证（单组自动 + 多组人工，ADR-0011 A）**：unique/ambiguous/zero/24h/COMPENSATING/EXPIRED_REFUND/Payment 全链路有测试；多组/无组按 ADR-0011 A 保持人工，属已批准范围；ship 优先 `EXPIRED_FINANCE`。
- **AC-60 仍部分**：无售后单自动分配 `legacy:<order_no>:<refund_seq>`、Inventory HELD/缺货完整发货流程未接线；T5 `TestAC60...` 仅覆盖读取/回填/有售后退款/发货。
- `TestAC63LegacyResolutionAndQueue` 仍是前缀（unique/ambiguous/zero/COMPENSATING）；Phase 3b 新测试补 ack→Payment→EXPIRED_REFUND 全链路；两者区分清楚，没有把前缀冒充全量。

---

## 8. 边界

- `git diff 098e7fe -- migrations api internal/services/inventory` = **0**；无 proto/migration/Inventory 改动。
- 只改 worker + test；无 Payment 协议/迁移。
- Spec AC-63 验收表更新为已验证（见下）；AC-60 保持未验证/部分。

---

## 9. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | release 请求/receipt 只对应第一个 refund；其余 refund 只在 refund_applications、Payment 全付；需文档化 | `compensateLegacyResolution` |
| NB-2 | app DONE 与 EXPIRED_REFUND 非同一 tx；收敛但非原子 | `payLegacyCompensation` |
| NB-3 | Payment 错误一律 Unavailable；永久错误无限重试；未按 ADR step 7 用 `GetRefundByKey` | `payLegacyCompensation` |
| NB-4 | 重试重调所有 refund；依赖 Payment idempotency key | `payLegacyCompensation` |
| NB-5 | FAILED/PENDING COMPENSATING 无专门告警/计数，scan 反复 block | `expireLegacyResolutions` |
| NB-6 | 无 Payment 失败/重试/多组/shipped 的专门新测试 | 新测试覆盖 |

---

## 10. 裁决

**PASS（Phase 3b）。** ack 屏障（PENDING/FAILED 零 Payment，RELEASED 才 Payment，Payment 失败保持 COMPENSATING）；release+receipt 同事务；refund_applications 先写；单组限制/多组不默认 G1；Payment last/幂等；app DONE + EXPIRED_REFUND；全量测试 PASS。

**Phase 3b 可合入；AC-63 已验证（单组自动 + 多组人工 ADR-0011 A）；AC-60 仍部分；T5.2/3/5/6/7 未做；NOT RELEASE。**

AC 位移：`docs/specs/0002` **AC-63 未验证 → 已验证**（`TestLegacy24hAckThenPaymentAndExpiredRefund`/`...FailedAck` + T5 AC-63 prefix；多组人工 ADR-0011 A）；AC-60 保持未验证（部分）。

---

## 11. 复现命令（只读）

```bash
git rev-parse HEAD   # 098e7fe...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestLegacy|TestT5RealMigration|TestAC60|TestAC63|TestRefundReleaseAck|TestReleaseCommitment' -count=1
go vet ./internal/services/order ./cmd/order
git diff 098e7fe -- migrations api internal/services/inventory | wc -l   # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 PASS 裁决 + `docs/specs/0002` AC-63 验收状态。未改代码/契约/迁移；未 commit、未 merge。
