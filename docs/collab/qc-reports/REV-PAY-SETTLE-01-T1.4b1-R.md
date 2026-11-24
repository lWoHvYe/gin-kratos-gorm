# REV-PAY-SETTLE-01-T1.4b1-R — Atomic RefundBatch 复审

| 项 | 值 |
|---|---|
| review_scope | **实现复审**：`c8fe518..198c623` 的 T1.4b1-R —— Payment `RefundBatch` / `GetRefundByKey`（C-04/C-05）、Order preflight（B/done/inFlight/outstanding/R）、in-flight 30m、lazy reconcile、错误映射、不逐组退化。对照 SPEC-0002 v1.9 AC-65/INV-48、ADR-0007 Amd1 §4.1.3/§4.7、C-04 accepted、`t1.4b1-r-plan.md`。**不评审** T1.4b2/INV-44、zero/Aftersales task、T1.5、T1.6、T1.7/T5、SPEC-0003/GOV-COST-01 等并发治理变更；不构成 Release PASS。 |
| base | `c8fe518`（T1.4b1 FAIL 冻结） |
| head | `198c623`（`wip(payment): implement GetRefundByKey read-only lookup`） |
| 契约 | SPEC-0002 v1.9；C-04 `RefundBatch` **accepted**；C-05 `payment/0012_refund_batch.sql`；T1.4a Rulings Addendum R-B |
| change_id | `PAY-SETTLE-01` |
| Actor / Reviewed-by | `reviewer-phase1`（Reviewer）≠ `dev-phase1e`（Developer） |
| **verdict** | **FAIL（有范围）** |
| release_status | **NOT RELEASE**；不批准合并 |

> 本报告只裁决 T1.4b1-R；未修改代码/契约/迁移。工作区已有非本次审查的 `scripts/collab_gates.py`、`scripts/test-collab-gates.sh` 和 governance docs 变更，不在本报告范围。

---

## 1. BLOCKER（仍阻止 T1.4b1-R 关闭）

| ID | 位置 | 事实 / 违背契约 |
|---|---|---|
| **B2** | `internal/services/order/tcc_payment.go:600-668`（`reconcileTccDoneApplications` / `reconcileTccRefundFact`）；`:624-626` 明确“never touches the order amount projection”；`internal/services/order/tcc.go:319`（`payTccBatch` 唯一调用方）；`cmd/order/main.go:31-45`（workers 中无 TCC lazy worker）；计划 `docs/lead/PAY-SETTLE-01/t1.4b1-r-plan.md:60`（要求“Order lazy reconcile（**持久 task/worker**），batch 不得永久 PAYING”） | **lazy reconcile 既无持久 worker/task，也不收敛 order 金额投影。** ① `payTccBatch` 只在 `TccCancelMarkPaid` 内被调用；`RefundBatch` 成功后 TCC cancel 返回 success，DTM 不再重试，`cmd/order` 没有扫描 PAYING/ADMITTED TCC batch 的 worker。若 `payment.refunded` 事件丢失/消费 dead-letter，commitment 停在 `RESERVED`、application 停在 `PROCESSING`、batch 停在 `PAYING`，`reconcileTccDoneApplications` **永远不会再执行**。② 即使人工/重试再次调用 `payTccBatch`，`reconcileTccRefundFact` 只调用 `settlePaymentRefundLedgerTx`（commitment/app/batch），**不更新 order** `refunded_cent/refund_status/refund_state`；注释明确“never touches the order amount projection”。 |
| **B3（交付治理）** | `docs/lead/PAY-SETTLE-01/t1.4b1-r-plan.md:5`（Status 仍 `DRAFT — pending Architect/QC/Reviewer review`）、`:96-98`（Architect/QC/Reviewer 仍 TBD）；`docs/implementation/PAY-SETTLE-01-T1.4b1-selfcheck.md:10`（Code head 仍写“本 WIP 提交”、Base=`dff267d`；仍是 T1.4b1 而非 T1.4b1-R，未绑定 `198c623`）；`prompt/Reviewer.md:9`、`AGENTS.md:56` | C-04 已 accepted，但 **T1.4b1-R 的 Plan 仍 DRAFT/评审行 TBD，且仓库没有绑定 `198c623` 的 canonical Developer selfcheck**（旧 selfcheck 是 T1.4b1/1f4f2d1，未覆盖 RefundBatch/GetRefundByKey/lazy）。按 Reviewer 角色 prompt，缺 canonical 自检清单/Plan 批准状态不能 PASS；若 Lead 已另行持有 R selfcheck 与 Plan 批准记录，应附 canonical 指针并修正 Plan 状态，否则该交付无法进入 merge 输入。 |

**B2 证据（Reviewer 实际复现）**：单独运行 `TestTccRefundBatchLazyReconcileKeepsPartialBatchPaying`（不投递任何 `payment.refunded` 事件）后查询测试库：

```text
orders:            ORD-PAY-MIX  refunded_cent=0  refund_status=''  refund_state=REFUNDING status=PAID
refund_batches:    tcc:PAY-PAY-MIX:1  DONE
refund_applications: tcc:PAY-PAY-MIX:G1 DONE; tcc:PAY-PAY-MIX:G2 DONE
```

即 **batch/ledger 已 DONE，但 order 仍是 REFUNDING、refunded_cent=0**。这正是 `payment.refunded` 丢失后 lazy 应当收敛的 projection；当前 lazy 不写 order，order 会永久停在 REFUNDING。`TestTccRefundBatchLazyReconcileRepairsLostEvents` 之所以显示 FULL/10000，是因为测试在 lazy 之后又**手工投递了一条 `payment.refunded` 事件**；它没有证明 lazy 自身能修复 order，也没有证明存在持久 worker。**测试用 late event 掩盖了 B2**。

**修复要求**：增加持久扫描/worker（或 durable task）对 active TCC batch（`FROZEN/ADMITTING/ADMITTED/PAYING`）按 `GetRefundByKey`/Payment 事实执行 lazy reconcile；lazy 必须在同一事务内同时更新 **order 金额投影**（`refunded_cent/refund_status/refund_state`，单调、active batch 保持 REFUNDING、全 done 后 FULL/PARTIAL）与 commitment/app/batch；补“不投递事件”的 fault-injection 测试，断言 order 也收敛。不得只修 batch ledger 或只依赖 DTE 重试。

---

## 2. B1 外部退款与原子性（已验证关闭，不计 BLOCKER）

- **外部退款 1000/6000/4000**：`TestTccRefundBatchExternalRefundParksWithoutPayment` 三个子用例 PASS；preflight `R = captured - refunded` 与 `B - done` 不等 → `tccManualHold` → `parkTccBatchManual`，**0 个 RefundBatch、承诺保持 RESERVED**。
- **preflight 后窗口**：`TestTccRefundBatchExternalRefundInWindowIsAllOrNone` PASS；Payment `RefundBatch` 在 `payment_intents` 行锁内校验 `newSum <= remaining`，全失败/零 refunds/零 outbox；Order 将 `INSUFFICIENT_REMAINING` 映射 `MANUAL_HOLD`。C-04 原子批关闭“先付 G1 再拒 G2”的窗口。
- **Payment 原子性/幂等/冲突**：`TestRefundBatchAtomicSameAmountReason`、`SecondInsertFailureRollsBack`、`ReplayIsIdempotent`、`SameBatchDifferentItemsConflicts`、`SameRefundIDDifferentAmountConflicts`、`InsufficientRemainingAndSumMismatchWriteNothing`、`MixedExistingAndNewItems`、`ConcurrentSameBatch`、`ConcurrentDifferentBatchesCannotOverRefund` 全部 PASS；同一 payment/amount/reason 多 item 可成功（旧 `uk_refunds_idem` 已由 `0012` 删除）。
- **`GetRefundByKey`**：`refund_lookup` 测试 PASS；只读 `refundByKey`，缺失 NOT_FOUND，无写入/无事件；与 `refunds.idempotency_key` 对齐。
- **Order 不逐组退化**：`TestTccRefundBatchPaysEveryAdmittedGroupAtomically` 断言 `refundCalls=0`，只调用一次 `RefundBatch`；无 per-group fallback。

---

## 3. Order preflight 其他项

- **B/done/inFlight/outstanding/R**：`preflightTccRefund` 计算冻结 `B=batch.SumGroupCapCent`、`done`（fact 成功或 commitment SETTLED）、`inFlight`（PENDING/PROCESSING）、`outstanding`，校验 `B==sum`、`unknown==0`、`R==B-done`；外部退款/unknown → `MANUAL_HOLD`，零 RefundBatch。**通过**（外部 1000/6000/4000 测试）。
- **in-flight 30m**：`TestTccRefundBatchInFlightStopsNewGroupsUntilConverged`、`InFlightTimeoutParksManualHold` PASS；超时 `MANUAL_HOLD`，未超时不允许新组。
- **GetRefundByKey 超时**：`TestTccRefundBatchGetRefundByKeyTimeoutFailsClosed` PASS；`UNAVAILABLE`、零 RefundBatch、batch 保持 ADMITTED，可重试。
- **错误映射**：`BATCH_UNAVAILABLE` → `PAYING` 重试；其余确定性 reason → `MANUAL_HOLD`；`TestTccRefundBatchUnavailableKeepsPayingAndReplaysSameBatch`、`DeterministicFailureParksManualHold` PASS。
- **不部分打款**：外部 1000/6000/4000 + in-window 原子批测试 PASS；Order 没有 per-group `Refund` fallback。
- **`inFlight`/`done`/lazy 的 order projection**：见 B2；ledger 收敛有测试，order 收敛缺失。

---

## 4. 测试证据（Reviewer 独立复跑）

环境：`go1.27.1`；MySQL `mysql-unicorn 127.0.0.1:3306`；`GOCACHE=$PWD/.tmp/gocache`、`GOPROXY=off`、`GOFLAGS=-mod=readonly`（未改 go.mod）。

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0**，1m11.445s |
| `go test ./internal/services/order -run 'TestTccRefundBatch|TestTccCancel|TestTccAdmission|TestPaymentRefunded' -count=1 -v` | **PASS**，80.883s |
| `go test ./internal/services/payment -run 'TestRefundBatch|TestGetRefundByKey|TestRefund' -count=1 -v` | **PASS**，5.110s |
| `go test -race ./internal/services/order -run 'TestTccRefundBatch|TestPaymentRefunded' -count=1` | **ok**，29.160s |
| `go test -race ./internal/services/payment -run 'TestRefundBatch|TestGetRefundByKey' -count=1` | **ok**，6.456s |
| `go vet ./internal/services/order ./internal/services/payment` / `gofmt -l` | exit 0 / 无输出 |

- **真实迁移**：在 scratch MySQL 依次执行 `migrations/payment/0001_init.sql`、`0002_partial_refunds.sql`、`0012_refund_batch.sql`，全部 exit 0；实际 schema 为 `refunds.batch_id`、`uk_refunds_idempotency(payment_no,idempotency_key)`、`uk_refunds_batch_item(payment_no,batch_id,idempotency_key)`、`payment_refund_batches`，与 C-04/C-05 一致。测试套件本身仍用 `paymentTestDB` 的内联镜像 schema（非真实 migration）；建议补自动化真实迁移测试（RΣ5/T5.4），但不作为本次 blocker。
- **Developer selfcheck/logs**：没有绑定 `198c623` 的 T1.4b1-R selfcheck；旧 `PAY-SETTLE-01-T1.4b1-selfcheck.md` 仍是 T1.4b1/Base=`dff267d`，未绑定 `198c623`。本次以 Reviewer 独立复跑为准；未发现 SKIP 被当 PASS。
- `ac-coverage.tsv` 的 AC-51/AC-65 仍 `DEFERRED`；未标 AC-VERIFIED。

---

## 5. 边界 / 范围

- **C-04**：`api/proto`/gen 新增 `RefundBatch`/`GetRefundByKey` 既有 RPC，`baseline.json` 更新；C-04 accepted。**C-05**：`migrations/payment/0012` 新增 batch_id/unique/batch 表；实际迁移验证通过。**C-04/Plan/C-05 语义一致**；Plan 自身仍 DRAFT/TBD（B3）。
- **未越界**：无 T1.4b2/INV-44 release/回库、zero/Aftersales 持久 task、T1.5 `order.refund_requested`、T1.6 PENDING；未改 Order 生命周期/Inventory；无逐组 `Refund` fallback。`refund_consumer` 的 `payment.refunded` 收敛仍属 T1.4b1 范围。
- **范围污染提示**：`c8fe518..198c623` 还包含 SPEC-0003/GOV-COST-01/ADR-0008、`scripts/collab_gates.py` 等并发治理变更，不属于 T1.4b1-R；本报告未评价，但该 head 不是干净的 T1.4b1-R-only merge 输入。

---

## 6. 非阻断问题

- **N1**：`paymentTestDB` 用内联 `payment_refund_batches`/`refunds` 镜像，而非执行 `0012`；本次 Reviewer 手动真实迁移通过，但 RΣ5“迁移 + 测试”未自动化。
- **N2**：`planTccRefundCandidates` 对缺失 commitment/terminal application 返回普通 `FAILED_PRECONDITION`，batch 保持 `ADMITTED`（测试 `RequiresReservedCommitment` 断言），不会自动 `MANUAL_HOLD`；零 Payment、fail-closed，但无自动人工态/告警。
- **N3**：`planTccRefundCandidates` 未校验 `intent.GetOrderNo()`==batch.OrderNo / `GetRefundByKey` fact 的 order；Payment batch 的 payment_no 在 T1.2/T1.3 已校验，当前为防御性缺口。
- **N4**：`inFlight` 的 `CreatedAtUnix=0` 永不超时；应 fail-closed/人工或使用事件时间。
- **N5**：`payment_refund_batches.status` 无 DB CHECK；由应用枚举兜底（QC 已提示）。
- **N6**：B3 的 Plan/selfcheck 状态如前；C-04 accepted 不等于 T1.4b1-R 实现 PASS。

---

## 7. 声明

- 本报告未修改代码/契约/迁移，只新增报告；未批准合并。
- **verdict FAIL（有范围）**：B1 外部退款/原子性已关闭；**B2 lazy reconcile 不持久且不收敛 order 金额投影**、B3 canonical selfcheck/Plan 状态缺失，仍阻止 T1.4b1-R 关闭。**不是 Release PASS，不替代人类 merge 决策。**
- 修复 B2 需增加持久 TCC lazy worker/task，并在 lazy 中同事务更新 order `refunded_cent/refund_status/refund_state`；补“不投递 payment.refunded”的 fault-injection 断言 order 已收敛。B3 需补 `198c623` 绑定的 R selfcheck 并回写 Plan/approval。修复后再按精确 head 复审。
