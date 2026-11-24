# REV-PAY-SETTLE-01-T1.4b1 — TCC 逐组打款 + payment.refunded 收敛复审

| 项 | 值 |
|---|---|
| review_scope | **实现复审**：`dff267d..1f4f2d1` 的 T1.4b1 —— batch `ADMITTED→PAYING→DONE`、逐组 `Payment.Refund`（`tcc:<payment_no>:<group_no>`）、`payment.refunded` 收敛（commitment/app/batch）、部分失败/重试/人工态；对照 SPEC-0002 v1.9 AC-51/AC-65、INV-48/INV-45、ADR-0007 Amd1 §4.1.2/§4.1.3、T1.4a Rulings Addendum R-B。**不评审** T1.4b2/INV-44 release、零元/Aftersales task、T1.5、T1.6、T1.7/T5；不构成 Release PASS。 |
| base | `dff267d`（T1.4a Addendum approval） |
| head | `1f4f2d1`（`wip(order): T1.4b1 tcc group refunds + payment.refunded convergence`） |
| 契约 | SPEC-0002 v1.9；ADR-0007 Amd1（accepted）+ 零元 Addendum + T1.2/T1.4a Rulings Addendum（accepted） |
| change_id | `PAY-SETTLE-01` |
| INV | INV-48（TCC Σ/逐组扇出/不得部分打款）、INV-45（准入/承诺）、INV-36、INV-37、INV-46 |
| affected_AC | AC-51、AC-65（主）；AC-61/AC-62/AC-64 边界；**不宣称 AC-VERIFIED** |
| Actor / Reviewed-by | `reviewer-phase1`（Reviewer）≠ `dev-phase1d`（Developer） |
| **verdict** | **FAIL（有范围）** |
| release_status | **NOT RELEASE**；不批准合并 |

> 本报告只裁决 T1.4b1；不修改代码/契约，不批准合并，不替代人类 merge/Release 决策。FAIL 按 AGENTS §1.5 / 宪法 Workflow §9 冻结 `PAY-SETTLE-01` 的 T1.4b1 范围。

---

## 1. BLOCKER（仍阻止 T1.4b1 关闭）

| ID | 级别 | 位置 | 事实 / 违背契约 | 必须满足的行为 |
|---|---|---|---|---|
| **B1** | **BLOCKER** | `internal/services/order/tcc_settlement.go:401-418`（`resumeTccRecheckTx` 对 `ADMITTED/PAYING` 只做 `sum==0 && active==0 && remaining>0` 检查，**不比较 Σ 与 remaining**）；`internal/services/order/tcc_payment.go:79-184`（`payTccBatch`）、`:189-266`（`planTccPayments` 只校验每组 application/commitment，不读 Payment 当前 remaining/Σ）、`:270-288`（`Payment.Refund` 用冻结金额）；`internal/services/payment/service.go:378-386`（Payment 只校验 `in.RefundedCent+amount <= captured`，即**单组** cap） | **PAYING/ADMITTED 阶段跳过准入 Σ 后，Payment 的 per-call cap 不能替代 Σ 校验，会在外部退款改变 remaining 时造成部分打款。** 具体时序：订单 captured=10000，batch 组 G1=6000、G2=4000；batch 已 `ADMITTED/PAYING`；期间发生一笔**支付级/外部退款 1000**（无组 commitment，故 `planTccPayments` 看不到差异），Payment 当前 remaining=9000。`resumeTccRecheckTx` 返回 `rechecked`（sum=10000、remaining=9000）但不阻断；`payTccBatch` 依次调用：G1 6000 ≤ 9000 → **Payment 成功、钱已退**；G2 4000 > 3000 → `failTccPayment` 判定性拒绝 → batch `MANUAL_HOLD`。结果 **G1 已部分打款 6000**，违反 AC-65 `IF 准入或打款期间订单级 captured 快照发生变化 THEN 重新执行 Σ；仍不等 → 阻断并转人工，不得沿用旧快照继续打款`（SPEC `:382`）、AC-65 `:375`“不得部分打款”、ADR §4.1.3(5) `:249`“PAYING 期间每次打款前重新读取 Payment 权威；若 remaining captured 变化导致 Σ 不成立，停止后续打款、MANUAL_HOLD”。Payment 的 per-call cap 只保证“单组不超剩余”，**不保证 batch 全部活动组的 Σ == remaining**；`resumeTccRecheckTx` 的现有 `sum==0` 守卫只覆盖“全部组已 SETTLED 但 remaining>0”，不覆盖“部分组可付、部分组超”的差值。 | 在 `ADMITTED/PAYING` 进入/继续打款前，必须以**当前** Payment 权威 `captured/refunded` 重新计算 remaining，并与本批**尚未退款**的组金额之和（或冻结 batch 的 Σ 按已 SETTLED 调整）比较；**Σ != remaining 时在调用任何 Payment.Refund 之前**停止，batch `MANUAL_HOLD`/告警，不得先把能过的组打掉。ADR §4.1.3(5) 要求“每次打款前”重新读取；至少必须在 fan-out 前做一次权威 Σ 校验，并在组间变化时重新校验。当前 `TestTccPaymentRetryToleratesUnprojectedRefund` 只模拟“本批全部已退、事件未投影”（Payment refunded=10000），没有覆盖“外部部分退款 1000、G1/G2 部分可付”的零打款断言；`TestTccPaymentRejectionParksBatchForManualHandling` 是第一组即拒绝，不会暴露“先付 G1 再拒 G2”的部分成功。 |

**结论**：`resumeTccRecheckTx` 在 `ADMITTED/PAYING` 跳过准入 Σ、仅靠 Payment per-call cap，**不可接受**。Payment per-call 是单笔上限，不是 AC-65/INV-48 的 Σ 等式；必须在任何 Payment.Refund 之前恢复 Σ/remaining 校验（或由 Payment 提供等价的全批 remaining 查询）。不能以“Payment 会拒绝某一组”事后兜底，因为此前已支付的组不可回滚。

---

## 2. 非阻断问题（不列入本次 BLOCKER，但 Release 前需处置）

| ID | 级别 | 位置 | 事实 |
|---|---|---|---|
| N1 | MAJOR/已知缺口 | `docs/implementation/PAY-SETTLE-01-T1.4b1-selfcheck.md:§5-2`；`internal/services/payment/service.go:357-359`（idempotent Refund 命中现存 refund 时**不再 append `payment.refunded`**）；Order 无 `GetRefundByKey` 调用 | 若 `payment.refunded` 事件丢失/消费 dead-letter，commitment 保持 `RESERVED`、application `PROCESSING`、batch `PAYING`；重试 `Payment.Refund` 命中现存 refund 返回成功但不再发事件，无 lazy 查询可收敛。selfcheck 将其归 T5.3/后续；本报告不把它算作 T1.4b1 blocker，但 Release 前必须有日终/惰性对账 owner。 |
| N2 | MINOR | `internal/services/order/refund_consumer.go:150-158`、`:216-243` | `payment.refunded` 找不到 commitment 时只 `Warn` 并 ACK；application 为 `CANCELLED/REJECTED/FAILED_PERMANENT` 时只 `log.Error` 并返回，batch 仍 PAYING；没有错误重试/MANUAL_HOLD。正常 T1.4b1 路径不产生这些状态，但数据损坏/未来 T3 终态会留下不可自动收敛的账本。 |
| N3 | MINOR | `internal/services/order/refund_consumer.go:94-105` | 仅在 `nextRefundState == PARTIAL` 时保持 `REFUNDING`；若一个 **full** projection（`total>=captured`）在 batch 仍 `PAYING` 时到达（乱序/外部退款），order 置 `FULL`；之后 `TccCancelMarkPaid` 入口 `FULL` 早退，batch ledger 可能仍 PAYING。金额不超退，但收敛依赖后续事件。 |
| N4 | MINOR | `internal/services/order/tcc_payment.go:189-266` | `planTccPayments` 只遍历已存在的 application；未校验 application 集合与 `refund_batches.sum_group_cap_cent`/冻结组集合一致。若单条 application 被删（人工损坏），可能只付剩余行并最终 DONE。正常 Tx1/ T1.2 resume 会补行；属防御性数据完整性缺口。 |
| N5 | MINOR/范围 | `internal/services/order/refund_consumer.go:144-243` | `payment.refunded` 消费段现在对**任意** `refund_id` 的 commitment/application 收敛（含非 TCC 来源），并校验 event amount == commitment amount。旧 consumer 只按 refund_id 结算 commitment、无金额校验；正常 aftersale/零元路径未改变，但需 T3/T1.5 回归确认不改变既有消费者语义。 |
| N6 | MINOR | `tcc_payment.go:294-331`；`tcc_admission.go:71-125` | `MANUAL_HOLD` 只在 Payment.Refund 返回 `InvalidArgument`/`FailedPrecondition` 时设置，符合“只对判定性拒绝”；缺 commitment/terminal application 等 `plan`/admission 失败只保持 `ADMITTED`/返回 `UNAVAILABLE`，不会自动 MANUAL_HOLD（fail-closed，但需人工日志）。 |

---

## 3. 已核对通过项

- **全部准入后才打款**：`TccCancelMarkPaid` 先 `admitTccBatch` 再 `payTccBatch`（`tcc.go:306-323`）；`payTccBatch` 只接受 `ADMITTED/PAYING`，`FROZEN/ADMITTING` 返回可重试 `UNAVAILABLE`；`planTccPayments` 要求每个 application `PROCESSING + PRE/POST + refund_id=tcc:<pay>:<group> + amount>0 + RESERVED commitment 且金额匹配`，任一不满足则整批零退款（`TestTccPaymentIsBlockedUntilEveryGroupIsAdmitted`、`TestTccPaymentRequiresReservedCommitment`、`TestTccPaymentRefusesNonAdmittedBatch`）。
- **batch `ADMITTED→PAYING→DONE`**：`payTccBatch` 在 fan-out 前 CAS `ADMITTED→PAYING`；`completeTccBatch` 只在所有 application DONE 后 CAS `PAYING→DONE`，`ADMITTED` 不当作已退款；`TestTccPaymentPaysEveryAdmittedGroupOnce` 断言付款后仍 PAYING/RESERVED/PROCESSING。
- **refund_id 幂等**：`payTccGroup` 用 application.RefundID 作为 `RefundRequest.idempotency_key`；Payment `refundKey` 原样使用 provided key（`payment/service.go:455-460`），按 `(payment_no,idempotency_key)` 唯一；重复调用返回现有 refund，不重复退；`TestTccPaymentRetryReusesRefundIDAfterTransientFailure`、`:PaysEvery...` 的 repeat 段验证同 key/同金额。
- **部分失败/重试**：首组成功、次组 timeout → batch 保持 PAYING，application attempts/last_error 记录，重试同 refund_id（`TestTccPaymentRetryReusesRefundIDAfterTransientFailure`）；判定性拒绝 → 停止 fan-out、`PAYING→MANUAL_HOLD`，自动重试不推进（`TestTccPaymentRejectionParksBatchForManualHandling`）。
- **payment.refunded 收敛**：commitment RESERVED→SETTLED、application PENDING/PROCESSING→DONE、全部 DONE 后 batch PAYING→DONE；重复 event 幂等（commitment version 不变、无重复投影事件）、乱序 event 仍收敛账本、非生效忽略、未知 refund_id 只投影订单并 Warn（4 个 consumer 测试）。`refund_state` 在 active batch 的部分投影保持 REFUNDING（`TestPaymentRefundedKeepsRefundingWhileTccBatchPays`）；部分退款金额投影单调。
- **边界**：`git diff --name-only dff267d..1f4f2d1` 无 `api/proto`、`api/gen`、`migrations/`、T1.4b2/INV-44 release、Inventory/AfterSales 生产代码、T1.5 `order.refund_requested`、T1.6 PENDING 分支；`ports.go` 只消费既有 `Refund` RPC；Order 不调用 release/restock；`orders.status` 未被改写。

---

## 4. 测试证据（Reviewer 独立复跑）

环境：`go1.27.1 darwin/amd64`；MySQL `mysql-unicorn` `127.0.0.1:3306`；DSN 由 `./scripts/test-dsn.sh` 生成 `mall_integration_t14b1` / `_race` / `_full`；`GOPATH=$PWD/.tmp/gopath`、`GOMODCACHE=/Users/lWoHvYe/go/pkg/mod`、`GOCACHE=$PWD/.tmp/gocache`、`GOPROXY=off`、`GOFLAGS=-mod=readonly`（未修改 go.mod）。

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0**，59.866s |
| `go test ./internal/services/order -run 'TestTccPayment|TestPaymentRefunded' -count=1 -v` | **PASS**，13.869s（13 个 TCC payment/consumer 用例） |
| `go test -race ./internal/services/order -run 'TestTccPayment|TestPaymentRefunded' -count=1` | **ok**，14.647s |
| `go test ./internal/services/order -count=1`（附加回归） | **ok**，101.557s |
| `go vet ./internal/services/order` / `gofmt -l` | exit 0 / 无输出 |

- Developer selfcheck 声明的 `order-full 61.086s`、`-race 59.075s` 与本机复跑（101.557s/14.647s）存在机器/负载差异；所有目标用例通过。selfcheck 未提供可复核的原始测试日志路径（仅 `GOCACHE=/tmp/dsh-go-cache`），本报告以 Reviewer 独立复跑为准。
- **BLOCKER B1 无测试覆盖**：没有“外部部分退款 1000 + G1/G2 6000/4000 → 期望零 Payment.Refund / MANUAL_HOLD 且无组已退”的用例；现有测试只覆盖本批全部已退（`payments.refunded=10000`）和首组即拒绝，未覆盖“先付 G1 再拒 G2”的部分成功。
- `ac-coverage.tsv` 的 AC-51/AC-65 仍 `DEFERRED`；本报告未标 AC-VERIFIED。

---

## 5. 未覆盖范围（本报告不评价）

T1.4b2/INV-44 持久 release/回库、零元/Aftersales 持久 task（T1.4b2/T3）、`order.refund_requested` 消费者（T1.5）、`PENDING_PAYMENT+captured`（T1.6）、真实 Payment `GetRefundByKey` 惰性对账/日终 worker（T5.3）、T1.7/T5 全矩阵、真实 Fulfillment `CancelIfCreated`/Payment 端到端、BFF/运营展示、生产集群/迁移；本报告不替代这些任务。

---

## 6. 声明

- 本报告未修改代码/契约，仅新增报告；工作区存在非本次审查的 governance docs 变更，不在本报告范围。
- **verdict FAIL（有范围）**：T1.4b1 实现未通过 AC-65/INV-48/ADR §4.1.3(5) 的 Σ/不得部分打款审查；**不批准合并、不构成 Release PASS、不替代人类 merge 决策**。
- 修复 B1 后需重新提交精确 head、自检清单/测试日志与【测试范围】/【未覆盖范围】；Reviewer 按新 head 复审。FAIL 后 `PAY-SETTLE-01` 的 T1.4b1 范围保持冻结，修复必须挂重审 Plan，不得用新 change_id 或局部的“加个 if”绕过 Σ 协议。
