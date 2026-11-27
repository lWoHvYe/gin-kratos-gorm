# REV-PAY-SETTLE-01-T5-AC59 — T5.3 AC-59 第一条（退款投影先于捕获事实）审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（T5） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | `internal/services/order/refund_consumer.go` 的 `applyPaymentRefunded` 乱序守卫；`refund_consumer_integration_test.go` 新测试。**不审** AC-59 其余 3 分支、T5.3 其他故障注入。 |
| base | **`ec36339`** |
| head | **工作树 @ `ec36339`（未提交）**：refund_consumer.go +14/-3；test +22；无 proto/migration。 |
| spec_version | **SPEC-0002 v1.11**（AC-59；AC-60/63 不变） |
| adr_revision | **ADR-0007 Rev 2.3 / Amd1** |
| baseline_impact | **有**：退款投影乱序 fail-closed；仅 order consumer/test；无 proto/migration。 |
| INV | **AC-59**（退款投影先于捕获事实）、AC-60/63（不变）；资金投影安全。 |
| verification | **命令实测**：full `TestLegacy\|TestT5\|TestAC60\|TestAC63\|TestRefundReleaseAck\|TestReleaseCommitment\|TestAdmitRefund\|TestPaymentRefunded` → **PASS（100.678s）**；targeted `TestPaymentRefundedBeforeCaptureRejected`/`...OutOfOrderStillSettlesLedger` → **PASS（2.597s）**；`go vet` 0；`git diff ec36339 -- api migrations` = **0**。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **PASS（AC-59 第一条）**：仅 `RefundedCent>0 && paid_cent<=0` 拒绝；拒绝在任何 ledger/金额/commitment 写入前；`EVENT_OUT_OF_ORDER` 可识别；既有序测试 PASS。**AC-59 部分（1/4）；T5.3 仍部分；AC-60/63 不变**。0 阻断；NB-1…NB-4。 |

---

## 0. 结论

1. **守卫条件**：`applyPaymentRefunded` 读取 `paid_cent`；仅 `event.RefundedCent > 0 && paidCent <= 0` 拒绝；`RefundedCent==0`（零元）、`paid_cent>0`（已捕获/PAID）、累计较老的 refunded 事件不受影响（既有 `TestPaymentRefundedOutOfOrderStillSettlesLedger` PASS）。
2. **写入前**：守卫在 `SELECT ... FOR UPDATE` 后、ledger/金额投影/commitment 写入前返回；测试断言 `refunded_cent` 仍 0。
3. **错误**：`apperr.Failed("EVENT_OUT_OF_ORDER: refund projection arrived before capture")`，带 order/paid/refunded/status details；consumer 框架可重试，捕获事件到达后收敛。
4. **历史 PAID `paid_cent=0`**：守卫按 `paid<=0` 而非 status，历史 PAID 但 paid=0（AC-60 未回填）的 refund 事件也会被拒；**fail-closed 是安全的**，但需 ops/回填/人工确认；见 NB-1。
5. **AC-59 仍部分**：只完成“退款投影先于捕获事实”；收货先于发货、库存失败先于支付成功、缺货解决先于上报未做；AC-59 不能标已验证；T5.3 仍部分。
6. **AC-60/63 不变**：AC-60 已验证、AC-63 已验证，本轮无回退。

---

## 1. 审查输入与方法

- 读：`applyPaymentRefunded` diff、新测试、既有 `TestPaymentRefundedOutOfOrderStillSettlesLedger`、consumer 错误分类。
- 跑：full/targeted、vet、边界。
- 未改代码/契约/迁移；未 commit/merge。

---

## 2. 测试

```text
$ TEST_MYSQL_DSN=... go test ./internal/services/order -run 'TestLegacy|TestT5|TestAC60|TestAC63|TestRefundReleaseAck|TestReleaseCommitment|TestAdmitRefund|TestPaymentRefunded' -count=1
ok   ...order  100.678s

$ go test ... -run 'TestPaymentRefundedBeforeCaptureRejected|TestPaymentRefundedOutOfOrderStillSettlesLedger' -v
--- PASS: TestPaymentRefundedOutOfOrderStillSettlesLedger (1.23s)
--- PASS: TestPaymentRefundedBeforeCaptureRejected (1.15s)
```

---

## 3. 守卫条件 / 历史 PAID

- 条件只看 `RefundedCent>0 && paidCent<=0`；不区分 status。
- PENDING_PAYMENT/paid=0 的测试匹配。
- **NB-1（历史 PAID paid=0）**：AC-60 的历史 PAID 若 `paid_cent=0`（未回填），其退款事件会被判 `EVENT_OUT_OF_ORDER`；fail-closed 不会写错投影，但该历史退款可能一直重试/DLQ，直到 `paid_cent` 被回填/人工处理。AC-60 回填只写 `settlement_payment_no`，不写 `paid_cent`；需要 ops/后续回填确认。**不阻断本守卫**（fail-closed 优于错投影），但需记录/告警。
- **NB-2**：测试未覆盖历史 PAID paid=0 的 fail-closed 分支；也未有“捕获事件到达后同一事件重试收敛”的测试。

---

## 4. 写入前 / 错误 / 重试

- 守卫在 `SELECT ... FOR UPDATE` 后、`1. Converge the durable refund execution ledger first` 之前；不会写 ledger/commitment/金额投影。
- 测试只断言 `refunded_cent` 仍 0；未断言 ledger/commitment 零行（NB-3）。
- 错误 `apperr.Failed("EVENT_OUT_OF_ORDER...")`；`strings.Contains` 测试可识别。
- **NB-4**：未测试 consumer 层对 `EVENT_OUT_OF_ORDER` 的重试/DLQ 分类；只直接调 `applyPaymentRefunded`。

---

## 5. AC-59 / T5.3 / AC-60/63

- **AC-59 部分（1/4）**：本实现只覆盖“退款投影先于捕获事实”；其余三条（收货先于发货、库存失败先于支付成功、缺货解决先于上报）未做；AC-59 不能标已验证。
- **T5.3 仍部分**：本轮只是 AC-59 第一条；T5.3 的其余故障注入未做。
- **AC-60/63 不变**：AC-60 已验证、AC-63 已验证，本轮无回退。

---

## 6. 边界

- `git diff ec36339 -- api migrations` = **0**；无 proto/migration。
- 只改 `internal/services/order/refund_consumer.go` + test；无 Inventory/Payment 协议改动；无其他契约越界。

---

## 7. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | 历史 PAID `paid_cent=0` 的退款事件也会 fail-closed；需回填/人工/告警；不阻断守卫 | `applyPaymentRefunded` |
| NB-2 | 无历史 PAID paid=0、无“捕获后同一事件重试收敛”测试 | 新测试 |
| NB-3 | 测试只断言 refunded_cent=0，未断言 ledger/commitment 零行 | 新测试 |
| NB-4 | 未测试 consumer 层 `EVENT_OUT_OF_ORDER` 重试/DLQ 分类 | 新测试 |

---

## 8. 裁决

**PASS（AC-59 第一条）。** 仅 `RefundedCent>0 && paid_cent<=0` 拒绝；拒绝在 ledger/金额/commitment 写入前；`EVENT_OUT_OF_ORDER` 可识别；零元/已捕获/累计较老事件不受影响；既有序测试 PASS；无 proto/migration。

**AC-59 部分（1/4）；T5.3 仍部分；AC-60/63 不变；NOT RELEASE。**

---

## 9. 复现命令（只读）

```bash
git rev-parse HEAD   # ec36339...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestLegacy|TestT5|TestAC60|TestAC63|TestRefundReleaseAck|TestReleaseCommitment|TestAdmitRefund|TestPaymentRefunded' -count=1
go test ... -run 'TestPaymentRefundedBeforeCaptureRejected|TestPaymentRefundedOutOfOrderStillSettlesLedger' -count=1 -v
go vet ./internal/services/order
git diff ec36339 -- api migrations | wc -l   # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 PASS 裁决。未改代码/契约/迁移；未 commit、未 merge。
