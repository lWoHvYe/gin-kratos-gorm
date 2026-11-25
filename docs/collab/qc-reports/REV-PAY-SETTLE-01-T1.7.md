# REV-PAY-SETTLE-01-T1.7 — TCC compensation evidence（test-only）

| 项 | 值 |
|---|---|
| review_scope | `93380d5..2a05c1d`：仅 `internal/services/order/t17_evidence_integration_test.go` +510 行。T1.7 六场景 evidence；真实 MySQL + fake Payment/Fulfillment；无生产/契约/迁移。只读；不评审工作区 R0.1 未提交改动。不批准 merge/Release。 |
| base | `93380d5` |
| head | `2a05c1d`（`test(order): T1.7 TCC refund compensation evidence (WIP)`） |
| 文件 | 仅 `internal/services/order/t17_evidence_integration_test.go`；510 insertions；无其他文件 |
| Tier / Actor | **L2**（test-only evidence；`Tier-Rationale`/`Spec-Ref` 在 commit）；Actor `dev-phase1k`；**Status WIP**；无 Reviewed-by/AC-VERIFIED |
| 执行方式 | 用 `2a05c1d` 的 **detached worktree** 运行，未混入工作区 R0.1 改动 |
| **verdict** | **PASS（有范围）** |
| release_status | **NOT RELEASE**；不批准合并 |

---

## 1. BLOCKER

**无。**

## 2. 六场景覆盖与断言

| T1.7 场景 | 测试 | 断言 |
|---|---|---|
| Σ 不等 → 仅 PRECHECK_FAILED 审计、零 Payment | `TestT17SigmaMismatchWritesAuditAndNeverCallsPayment` | 仅 1 个 `refund_batches` PRECHECK_FAILED（sum=9000/remaining=10000/attempt1）；order/permit/application/commitment/outbox 无写；`payments.refundCalls=0`、`batchCalls=0`；permit 保持 CONSUMED/version1/shipment_no |
| REFUNDING vs Ship | `TestT17RefundingRejectsConcurrentAuthorizeShipment` | Tx1 后 G1/G2 permit REVOKED；8 ship + 2 refund 并发，所有 `AuthorizeShipment` FAILED_PRECONDITION，refund 成功，无 shipment_no；强制 group/order `refund_state=NONE` 后 permit 仍拒绝（permit 是权威）；per-group Refund=0；batch PAYING/DONE |
| Tx1 后崩溃续跑 | `TestT17CrashAfterTx1ResumesOnlyUnfinishedGroups` | Tx1 后 batch FROZEN、无 Payment；Payment 不可用返回 UNAVAILABLE、batch PAYING、同一 batch、ledger 行不重复、application 未 DONE；G1 成功事实后重试 `expected_total_cent=4000` 且只发 G2 item，batch identity 不变，per-group Refund=0 |
| 部分组失败 | `TestT17PartialGroupFailureBlocksAllPaymentsAndStaysRecoverable` | G2 CancelIfCreated 注入失败一次：UNAVAILABLE、零 Payment、batch ADMITTING、applications/commitments=2；重试只补 G2（cancelCalls=2）、一次 RefundBatch 覆盖 G1+G2，admissions=2 |
| 无组历史 G1 | `TestT17LegacyNoGroupUsesG1OnlyWithEvidence` | line evidence G1：permit REVOKED、application 8000/PRE、一次 batch G1；无 group 无 line：FAILED_PRECONDITION、仅 PRECHECK_FAILED 审计、无 synthetic permit、零 Payment |
| 已发货 POST | `TestT17ShippedGroupPostWritesNoReleaseAndNoRestock` | application PROCESSING/POST、admission POST、permit CONSUMED/version1 不变；`CancelIfCreated=0`；无 `inventory.refund_release_requested`/`inventory.compensation_requested`/restock/aftersale event；一次 RefundBatch |

- 以上均断言 **DB 状态/协议行**（batch、application、admission、commitment、permit、outbox、Payment 调用），不是只看返回值。
- **`-race`** 下 8 ship + 2 refund 并发场景通过；fake 用于确定性注入，真实 MySQL 保存协议状态。
- **无 AC-VERIFIED**：commit 仅 test 文件，未改 `ac-coverage.tsv`/status；符合用户要求。AC 文本引用不等于 AC-VERIFIED。

## 3. 测试执行（exact `2a05c1d` worktree）

环境：worktree `/tmp/gin-kratos-t17-exact`，detached HEAD `2a05c1d`；`TEST_MYSQL_DSN=root:root@tcp(127.0.0.1:3306)/mall_t17_exact`；`GOPROXY=off`、`GOFLAGS=-mod=readonly`。

| 命令 | 结果 |
|---|---|
| `go test -race ./internal/services/order -run 'TestT17' -count=1 -v` | **PASS**，测试 11.408s（含 build 总 2m45.444s） |
| `go test -race ./internal/services/order -run 'TestTccCancel|TestTccRefundBatch' -count=1` | **PASS**，71.898s |
| `go build ./...`（由上述 test 编译） | **exit 0** |
| `git status` in worktree | 无工作区 R0.1 改动混入 |

**SKIP 未当 PASS**：本 T1.7 测试无 SKIP；NATS/T5 broker e2e 不在本 commit 范围。

## 4. 边界 / 非阻断

- **test-only**：commit 仅 `internal/services/order/t17_evidence_integration_test.go` +510；无生产代码/契约/proto/migrations/scripts/AGENTS/CONVENTIONS。
- **无 T1.7 之外越界**：无 T1.5/T1.6/T5 改动；无组级 RefundBatch 生产改动；无 C-04/C-05 变更。
- **fake 使用**：Payment/Fulfillment 为 fake，用于注入 crash/unavailable/partial/POST；真实 MySQL 保存协议结果。T1.4b1 已有真实 Payment 原子性测试，本测试不替代 Payment 契约验证；作为 T1.7 Order 协议 evidence 可接受。
- **非阻断 [MAJOR] 测试注释不准确**：文件头写 “real C-05 refund migrations”，但 `orderTestDB` 实际使用内联 DDL（既有测试 helper），**没有执行 `migrations/order/*` 的 C-05 迁移**。本测试因此是“真实 MySQL + 内联镜像 schema”，不是“真实迁移”。建议改为“real MySQL (inline test schema)”或在 T5/Release 前补真实 migration rehearsal；**当前 WIP/无 AC-VERIFIED，不阻断 test-only commit**。
- **无 AC-VERIFIED**：正确；不要把本 PASS 当 AC 验收或 Release。

## 5. 声明

- 本报告未修改文件；使用 exact `2a05c1d` worktree 跑测试，未混入工作区 R0.1。
- **PASS 仅覆盖 `93380d5..2a05c1d` 的 T1.7 test-only 六场景 evidence**；不是 Release PASS，不批准 merge，不替代人类 merge 决策。
- 建议后续：真实 C-05 migration rehearsal + Payment 真实 compose evidence 再绑定 AC-VERIFIED；本测试的 fake 分支（Payment unavailable/partial/POST）不得被当作生产实现证明。
- 报告：[REV-PAY-SETTLE-01-T1.7.md](docs/collab/qc-reports/REV-PAY-SETTLE-01-T1.7.md)
