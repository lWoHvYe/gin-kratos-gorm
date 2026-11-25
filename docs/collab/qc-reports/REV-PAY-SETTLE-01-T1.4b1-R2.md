# REV-PAY-SETTLE-01-T1.4b1-R2 — 短复审（B2/B3）

| 项 | 值 |
|---|---|
| review_scope | `198c623..6ac218e` 的 T1.4b1-R2：`RunTccReconcileWorker` 持久收敛、`GetRefundByKey` lazy settle + order 金额投影、worker 注册/config、pay-path lazy 复用；B3 canonical selfcheck/Plan。不评审 T1.4b2/INV-44、zero/Aftersales、T1.5/T1.6、T1.7/T5；不构成 Release PASS。 |
| base | `198c623` |
| code head | `6ac218e`（`wip(order): T1.4b1-R2 durable TCC reconcile and order projection`） |
| docs/selfcheck | `2376f1b`；`docs/implementation/PAY-SETTLE-01-T1.4b1-selfcheck.md` 绑定 code head `6ac218e` |
| 契约 | SPEC-0002 v1.9；C-04 RefundBatch accepted；ADR-0007 Amd1；T1.4b1-R Plan |
| Actor / Reviewed-by | `reviewer-phase1`（Reviewer）≠ `dev-phase1f`（Developer） |
| **verdict** | **PASS（有范围）** |
| release_status | **NOT RELEASE**；不批准合并 |

> 未修改代码/契约/迁移。B2/B3 均在本次范围关闭；B1（外部退款 1000/6000/4000、RefundBatch 原子性）在 T1.4b1-R 已关闭，本次回归仍通过。

---

## 1. BLOCKER

**无。**

## 2. B2 复核（关闭）

| 检查 | 证据 |
|---|---|
| **持久/可注册** | `cmd/order/main.go:38` 注册 `order.RunTccReconcileWorker(ctx, bundle.Service, bundle.Logger, cfg.TccReconcileInterval)`；worker 启动即跑一次再按 ticker（`tcc_reconcile.go:50-77`）；`TCC_RECONCILE_INTERVAL` 默认 1m、非法/非正启动失败（`internal/platform/config/config.go:467-475`，`config_test.go` 覆盖）。 |
| **扫描 ADMITTED/PAYING** | `scanActiveTccBatches`（`tcc_reconcile.go:101-126`）`source=tcc AND state IN (ADMITTED,PAYING) LIMIT 100`；MANUAL_HOLD/FAILED_PERMANENT 不自动推进。 |
| **GetRefundByKey 成功 → 同一 order-locked 事务** | `reconcileTccBatch` 事务外读 fact，`applyTccReconcileTx`（`:220-287`）`findOrderForUpdate` 后 `settleTccRefundFactTx`（commitment CAS）、`markTccApplicationDoneTx`、`convergeTccBatchTx`、`projectTccReconcileOrderTx` 同事务。 |
| **order projection** | `projectTccReconcileOrderTx`（`:439-500`）写 `orders.refunded_cent/refund_status/refund_state`；`refunded_cent=max(current, SUM(SETTLED commitments))`；active batch（含 MANUAL_HOLD/FAILED_PERMANENT）保持 `REFUNDING`；全 done 才按 paid 派生 PARTIAL/FULL；已 FULL 不回退；同事务 append `order.refund_projected` 与 status log。 |
| **无事件测试真的不投递** | `tcc_reconcile_integration_test.go` 的 `commitTccRefundFacts` 只调 fake `RefundBatch` 生成 Payment 事实，**不调用 `applyPaymentRefunded`**；`TestTccReconcileWorkerSettlesLostEventsAndProjectsOrder` 断言 batch DONE、commitment SETTLED、app DONE、order 10000/REFUNDED/FULL、lifecycle PAID、`order.refund_projected=1`。 |
| **event + worker 不双计** | `TestTccReconcileWorkerDoesNotDoubleCountWithEvent`：G1 event 先投影 6000，G2 event 丢失，worker 只结算 G2，最终 10000（非 16000）；late G2 event 幂等。 |
| **部分 batch 保持 REFUNDING** | `TestTccReconcileWorkerKeepsRefundingForPartialBatch`：G1 fact → 6000/PARTIALLY_REFUNDED/REFUNDING，batch PAYING；G2 fact 到齐 → 10000/REFUNDED/FULL；重复 worker 不 bump version。 |
| **并发/幂等** | `TestTccReconcileWorkerConcurrentIsIdempotent`：8 并发 worker 最终 10000，DONE 后再跑 order version 不变；`-race` 通过。 |
| **无事实不写** | `TestTccReconcileWorkerLeavesOutstandingWithoutFact`：PENDING/无成功 fact → 0/REFUNDING，batch ADMITTED，承诺 RESERVED。 |
| **pay-path lazy** | `reconcileTccRefundFact` 改为复用同一 `settleTccRefundFactTx/markTccApplicationDoneTx/convergeTccBatchTx/projectTccReconcileOrderTx`；`tcc_payment_integration_test.go` 增加 order 金额/状态断言。 |

**结论**：B2 已关闭。先前“batch DONE 但 order REFUNDING/0”的缺口不再存在；worker 持久注册、同事务投影、event+worker 不双计、partial 保持 REFUNDING 均有真实 MySQL 测试。

## 3. B3 复核（关闭）

- `docs/implementation/PAY-SETTLE-01-T1.4b1-selfcheck.md:10` 绑定 **`6ac218e`**（Base `ba8a822`；Payment.GetRefundByKey 由 `198c623` 提供）；§1/§2/§3/§4/§5 覆盖 RefundBatch、GetRefundByKey、worker、order projection、三问与【测试范围】/【未覆盖范围】。B3 selfcheck 关闭。
- **Plan 状态**：`t1.4b1-r-plan.md` 仍 DRAFT/评审行 TBD；按用户说明归 Lead 维护，不是 Reviewer 本次实现裁决的 BLOCKER。merge 前 Lead 需回写 Plan/approval。
- **已知非阻断**：真实 Order↔Payment 组合链路（Payment.RefundBatch WIP `fc295e5`、Order 测试用 fake Payment）仍未验证；worker 无 metrics/手动触发、1m/100 上限；支付级/无 commitment 退款仍只依赖 `payment.refunded`；`MANUAL_HOLD` 保持 REFUNDING。均在 selfcheck §4.2/§5 声明，不阻止 T1.4b1-R2 关闭，但属于 Release 前置。

## 4. 测试证据（Reviewer 独立复跑）

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0**，38.530s |
| `go test ./internal/services/order -run 'TestTccReconcile|TestTccRefundBatch|TestPaymentRefunded' -count=1 -v` | **PASS**，22.345s（5 个 worker 用例 + RefundBatch/消费回归全绿） |
| `go test -race ./internal/services/order -run 'TestTccReconcile|TestTccRefundBatch|TestPaymentRefunded' -count=1` | **ok**，22.963s |
| `go test ./internal/platform/config -count=1` | **ok**，0.083s |
| `go test ./internal/services/payment -run 'TestRefundBatch|TestGetRefundByKey' -count=1` | **ok**，2.387s |
| `go vet` / `gofmt -l` | exit 0 / 无输出 |

- 未发现 SKIP 被当 PASS；AC-51/AC-65 仍 `DEFERRED`，未标 AC-VERIFIED。
- Developer selfcheck 声明的 full order 121.828s / `-race TestTcc` 90.874s 未在本轮复跑（本轮按用户要求定向）；定向与 `-race` 均通过。

## 5. 边界

`git diff --name-only 198c623..6ac218e` 的代码仅 `tcc_reconcile.go`、`tcc_payment.go`、`cmd/order/main.go`、`internal/platform/config/*` + tests；无 proto/migration、无 T1.4b2/INV-44、zero/Aftersales task、T1.5、T1.6；无 Inventory/逐组 `Refund`；Payment.GetRefundByKey 来自 `198c623`，本 R2 未改 RefundBatch 契约。worker 只读 Payment 事实、只写 order ledger/projection，不改 `orders.status`/lifecycle。

## 6. 声明

- 本报告未修改代码/契约；**PASS** 仅覆盖 `198c623..6ac218e` 的 T1.4b1-R2 B2/B3；不是 Release PASS、不批准合并、不替代人类 merge 决策。
- 人类可据此把 T1.4b1-R2 视为实现级复审通过；main/Release 仍需 Plan 状态回写、真实 Order↔Payment 组合验证与 T1.4b2/T1.5/T1.6/T1.7/T5 门禁。
