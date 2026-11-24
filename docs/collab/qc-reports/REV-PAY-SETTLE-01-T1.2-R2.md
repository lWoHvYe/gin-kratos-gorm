# REV-PAY-SETTLE-01-T1.2-R2 — G1/g1/g2/g3 短复核

| 项 | 值 |
|---|---|
| review_scope | **短复审**：T1.2-R2 对 `REV-PAY-SETTLE-01-T1.2-R` 的 G1/g1/g2/g3 四项修复；代码范围 `e7e29e5..72b984b`，docs `ad26c41`。不评审 T1.4/T1.5/T1.6/T1.7/T2/T3/T5；不构成 Spec 全量验收/Release PASS。 |
| base | `e7e29e5`（T1.2-R 代码 head） |
| code head | `72b984b`（`wip(order): T1.2-R2 legacy G1 shipped, mixed composition and no-active preflight`） |
| docs | `ad26c41`（T1.2 selfcheck §6 R2 addendum）；`REV-PAY-SETTLE-01-T1.2-R.md` 为前次 PASS 报告 |
| 契约 | SPEC-0002 v1.9 AC-51/AC-65；ADR-0007 Amd1 + 零元 Addendum + T1.2 Rulings Addendum；`docs/lead/PAY-SETTLE-01/t1.2-r-plan.md` |
| Actor / Reviewed-by | `reviewer-phase1`（Reviewer）≠ `dev-phase1b`（Developer） |
| **verdict** | **PASS（有范围）** |
| release_status | **NOT RELEASE**；不批准合并 |

> 工作区存在非本次审查的 governance docs 变更（`docs/review-tracking.md` 的 R PASS 记录、`SPEC-0003` 等 untracked），不属于本报告范围；本报告只绑定代码 head `72b984b`。

---

## 1. 结论

**PASS（有范围）。G1/g1/g2/g3 均已关闭；无仍阻止 T1.2 关闭的 BLOCKER。**

| 项 | 关闭依据 | 测试 |
|---|---|---|
| **G1** | `tcc_precheck.go:243` + `:414-431`（`legacyG1GroupStatus`）的 G1 fallback 合成组状态：`SHIPPED→SHIPPED`、`COMPLETED→RECEIVED`、`PAID/ALLOCATED→ALLOCATED`、`PENDING_PAYMENT→PENDING`、未知→`""`；`tcc_settlement.go:224-234` 的 `shipped()` 对 `""` 改为 fail-closed，不再默认“未离仓”。G1 `SHIPPED/COMPLETED` 无 facts 时走 POST，不初始化/撤销 permit、不写 admission/commitment。 | `TestTccCancelLegacyG1ShippedUsesOrderStatus`（SHIPPED/COMPLETED 子用例）PASS；`TestTccCancelUnknownGroupStatusNoFactsFailsClosed` PASS。 |
| **g1** | `tcc_settlement.go:1009-1149` 的 `tccGroupComposition` 新增 `KnownGoods/KnownFreight`（仅 known=1 头）；最终行合计校验改为 known=1 头 vs known=1 行，不再因同组 known=0 而跳过；新 TCC commitment 对混合组仍 `known=0`/无行。 | `TestTccCancelMixedCompositionValidatesKnownLines`（known=1 缺行→FAIL，补行→整组成功、commitment amount=9700/known=0/0 行）PASS。 |
| **g2** | `tcc.go:274-282` 在外部 Payment 快照 + Σ 校验完成后、`ListShipments` 之前判断 `Σ==remaining==0` → 幂等成功；Σ 不等仍先审计、不查 Fulfillment。 | `TestTccCancelNoActiveReturnsBeforeFulfillment` 两个子用例 PASS（全退 + Fulfillment 超时：`listCalls=0`、Payment 已读、零写入；Σ 不等 + Fulfillment 超时：1 条 PRECHECK_FAILED、`listCalls=0`）。 |
| **g3** | `tcc.go:328-333` 的 `TccQueryMarkPaid` 对 `PENDING_PAYMENT/PAID/ALLOCATED/SHIPPED/COMPLETED` 返回 `Prepared=true`，`CANCELLED/CLOSED` 为 false，与 `TccCancelMarkPaid` 的 B1 gate 对齐。 | `TestTccQueryMarkPaidMatchesLifecycleGate` PASS（7 个状态）。 |

---

## 2. BLOCKER 表

| ID | 级别 | 结论 |
|---|---|---|
| — | — | **无仍阻止 T1.2 关闭的 BLOCKER。** G1/g1/g2/g3 均已关闭；未发现资金静默不退款、错误打款、越界 T1.4/T1.6/consumer/proto/migrations。 |

---

## 3. 测试与环境

- 代码 head：`72b984b`；`go.mod` 未改动（`GOFLAGS=-mod=readonly`）；MySQL `127.0.0.1:3306`；DSN `mall_integration_t12r2` / `_full` / `_race`（`./scripts/test-dsn.sh`）；`GOCACHE=$PWD/.tmp/gocache`。
- 独立复跑：

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0**，52.771s |
| `go test ./internal/services/order -run 'TestTcc|TestOrderTestSchema' -count=1 -v` | **PASS**，53.527s（52 个 TCC/Σ/schema 用例，含 G1/g1/g2/g3 新增用例） |
| `go test -race ./internal/services/order -run 'TestTcc' -count=1` | **ok**，53.727s |
| `go test ./internal/services/order -count=1`（附加） | **ok**，64.579s |
| `go vet ./internal/services/order` / `gofmt -l` | exit 0 / 无输出 |

- Developer 自检日志 `/tmp/t12r2-logs` 已核对：`build-vet.log`（BUILD_OK/VET_OK）、`order-full.log`（69.878s）、`tcc-verbose.log`（PASS 51.078s）、`tcc-race.log`（51.648s）、`tcc-race-repeat.log`（12.791s）；与复跑一致（耗时差异来自机器/并行负载）。**未发现 SKIP/FAIL 被当作 PASS**。
- AC-51/AC-65 仍 `DEFERRED`；本报告**未标 AC-VERIFIED**。

---

## 4. 边界与已知非阻塞残余

- **越界检查**：`git diff --name-only e7e29e5..72b984b` 的代码仅 `internal/services/order/{tcc.go,tcc_precheck.go,tcc_settlement.go}` + TCC 测试；无 proto/gen/migrations/consumer/T1.4/T1.6 生产代码；TCC 仍不调用 `AdmitRefund`/`Payment.Refund`/`CancelIfCreated`，不 append `order.refund_requested`。
- **g2 边界（非 blocker）**：新补偿入口的无活动组已在 `ListShipments` 前幂等成功；**已存在 batch 的 `REFUNDING` 续跑**仍先读 `ListShipments`（续跑需要 shipment facts 补组，selfcheck §6.2 已声明）。Fulfillment 不可用时该续跑 fail-closed、零写入，不产生错误退款。
- **R-4 审计边界（非 blocker）**：`REFUNDING` 续跑的 Σ 变化在 Tx1 内审计；若 `ListShipments` 在进入 Tx1 前不可用，续跑 fail-closed 但不写审计。属同一续跑依赖，不影响新补偿入口的 g2 结论。
- **G-1 元数据（合并前处理）**：`72b984b` 受管提交仍无 `Reviewed-by` trailer；G-1 在 HEAD 上会 FAIL。merge 输入必须补，不能在未补前合并。
- **T1.2 仍是 WIP**：T1.4（AdmitRefund/CancelIfCreated/Payment.Refund/worker）、T1.6、T1.7、T5.7 未完成；AC-51/AC-65 仍 DEFERRED；本报告 PASS 仅覆盖 T1.2-R2 的四项修复，不构成 main/Release merge 许可。

---

## 5. 声明

- 本报告未修改代码/契约；只新增报告。工作区另有非本次审查的 governance docs 变更，未纳入。
- `PASS` 仅指 `e7e29e5..72b984b` 的 G1/g1/g2/g3 修复；**不批准合并、不构成 Release PASS、不替代人类 merge/Release 决策**。
- 人类可据此把 T1.2 视为“实现级复审通过”，继续进入 T1.4/T1.6/T1.7；合入 main/Release 仍需 T5.7、G-1 trailer 与人类批准（本报告不批准）。
