# REV-PAY-OPS-REPLAY-01-R2 — OPS-P5b 收敛终审 + AC-27 判定

| 项 | 值 |
|---|---|
| change_id / Parent | `PAY-OPS-REPLAY-01` / `PAY-SETTLE-01`（暂停） |
| Tier | **L2（确认）**：退款/履约准入、跨服务恢复、人工运维验收；不降级 |
| review_scope | **OPS-P5b**：真实 `aftersale.completed` consumer 与 TCC re-drive 收敛；D1/D5；AC-27 判定；不重开 OPS-P1–P4/T0–T8/R0.3-R1c |
| base | **`e3c68a2`**（OPS-P4 runbook/drill + docs/QC） |
| code head | **`7b7beb5bf349865097f03813a34bafb15f034bc8`**（test-only，1 file） |
| head（复审绑定） | **`a8fc1b3064b7cad695155f22dd955d0c96733d65`**（selfcheck §6） |
| 契约输入 | ADR `0007-amd1-addendum-ops-manual.md`；QC `MANUAL-R2`；waiver C+A（`review-tracking.md:458-483`）；SPEC-0001 v1.5 §16.2；SPEC-0002 AC-61/62；OPS-P4 report |
| INV | **INV-45**（无 release/permit restore）、INV-44/50 边界；AC-61/62 不越权 |
| verification | §1：真实 MySQL 8.4.13 + NATS 2.10.29；`TestOpsReplay` plain **3 PASS 3.709s** / `-race` **4.860s**；`migrations` diff 空；仅新增 `_test.go` |
| 日期 / Actor / Reviewed-by | 2026-11-27 / `reviewer-r04` / `human (merge pending)` |
| release_status | **NOT RELEASE** |
| overall verdict | **PASS（OPS-P5b scoped）；AC-27 = AC-VERIFIED（C+A waiver）；AC-61/62 仍 AC-REF**；不升级 AC-72/F7b |

---

## 0. 结论

**PASS（OPS-P5b scoped）。** `7b7beb5` 补齐 OPS-P4 的两个验收缺口，且不改生产代码：

- **D1**：真实 `RunAfterSaleConsumer` 从 `shop.aftersale.completed.v1` 消费真实 payload，组投影 `REFUNDING→REFUNDED`；admission 保持 `REVIEW_REQUIRED/PENDING_REMOTE`（不臆造 PRE）、`refunded_cent` 不变、无 release outbox；重复投递不改 group version；非法 JSON 进 DLQ 且不改投影。
- **D5**：先产生 `MANUAL_HOLD`，按 runbook 做 audited re-arm（admission/batch CAS），再调用**既有** `TccCancelMarkPaid`：真实 `CANCELLED→PRE_SHIP_REVOKED`、真实 `SHIPPED→POST_SHIP_ALLOWED`；`RefundBatch` 恰 1 次，重复调用不再打款。

runbook + OPS-P4 drill 60 OK + 本次真实 convergence 断言齐备；按人类 waiver C+A（到期 2026-12-31），**AC-27 可升 AC-VERIFIED**。AC-27A 仅按 R0.3-R1c 证据验收，不借本 waiver；AC-61/62 仍 **AC-REF**；AC-72 不升级；F7b 仍 `[BLOCKED]`。`NOT RELEASE`。

---

## 1. 实际执行（HEAD a8fc1b3 / code 7b7beb5）

```bash
# MySQL 8.4.13，独立 schema；NATS 2.10.29
TEST_MYSQL_DSN=.../mall_integration_p5b TEST_NATS_URL=nats://127.0.0.1:4222 \
  go test ./internal/services/order -run 'TestOpsReplay' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_p5b_race TEST_NATS_URL=nats://127.0.0.1:4222 \
  go test -race ./internal/services/order -run 'TestOpsReplay' -count=1
```

| 测试 | 结果 | 关键断言 |
|---|---|---|
| `TestOpsReplayAfterSaleCompletedConsumerConvergence` | **PASS 1.66s** | 真实 consumer：`REFUNDING→REFUNDED`；admission `REVIEW_REQUIRED/PENDING_REMOTE`；refunded 不变；0 release；重复不 bump version；非法 JSON→DLQ |
| `TestOpsReplayTCCRearmReDriveConvergence` | **PASS 1.03s** | re-arm 后 `TccCancelMarkPaid`：CANCELLED→PRE；`RefundBatch`=1；重复仍=1 |
| `TestOpsReplayTCCRearmPostConvergence` | **PASS 0.93s** | SHIPPED→POST（非 PRE）；`RefundBatch`=1 |
| plain / `-race` | **PASS 3.709s / 4.860s** | 3/3 PASS，无 race |
| `migrations` / 生产代码 | **空 / 无** | `git diff 7b7beb5..HEAD -- internal migrations` 为空（selfcheck/docs-only） |

原始日志：`.tmp/evidence-ops-p5b/reviewer-p5b.log`（本次审查生成，未入库）。

---

## 2. D1：真实 consumer 收敛

- **真实 consumer**：`RunAfterSaleConsumer`（`shop.aftersale.completed.v1` durable）在测试进程内启动，真实 NATS JetStream + 真实 MySQL；不是手工 SQL 投影。
- **正确语义**：`order_fulfillment_groups.refund_state` 变为 `REFUNDED`，但 `order_refund_admissions` 仍 `REVIEW_REQUIRED/PENDING_REMOTE`；REFUNDED 投影不会把 REVIEW 写成 PRE，也不会自动关闭组。
- **资金/释放**：`orders.refunded_cent` 不变；`inventory.refund_release_requested` outbox 0；无重复退款。
- **重复/乱序**：同 payload 重复投递，group `version` 不递增；非法 JSON 由 consumer `Permanent` 投 DLQ，投影不变。
- **唯一 id**：每轮唯一 `order_no/refund_id`，避免 JetStream 历史重放；连续 3 次稳定（selfcheck §6.3）。

---

## 3. D5：re-arm 后 TCC re-drive

- **re-arm**：admission `REVIEW_REQUIRED/PENDING_REMOTE→PENDING_REMOTE`（version CAS）、batch `MANUAL_HOLD→ADMITTING`（version CAS）、canonical `last_error` 保留；按 runbook 的 audited re-arm。
- **CANCELLED**：既有 `TccCancelMarkPaid` → admission `PRE_SHIP_REVOKED/PRE_SHIP_REVOKED`；`RefundBatch` 恰 1 次；再次调用 `RefundBatch` 不增加（幂等，不重复打款）。
- **SHIPPED**：既有 `TccCancelMarkPaid` → admission `POST_SHIP_ALLOWED/POST_SHIP_ALLOWED`（不是 PRE）；`RefundBatch` 恰 1 次。
- **不变量**：测试真实 schema；admission/commitment/permit/release 不被错误改写；无自动 release；Payment/Fulfillment 为既有 test double，但 `TccCancelMarkPaid`/Order DB 路径真实。

---

## 4. AC-27 判定

| 项 | 判定 | 依据 |
|---|---|---|
| AC-27-NOWB-01 终态① | **PASS** | D5 CANCELLED→PRE/组可关闭；runbook re-arm + TCC re-drive；`RefundBatch`=1 |
| AC-27-NOWB-01 终态② | **PASS** | OPS-P4 D7/D11 abandon；admission REVIEW、commitment RESERVED、permit 原值、0 release |
| AC-27-LIVE-01 | **PASS（waiver 范围）** | D1 真实 consumer 重放；REVIEW admission 不被事件臆造为 PRE；DLQ/重复幂等；runbook D1/D3/D4 |
| AC-27A 第 5/6 条 | **PASS（R0.3-R1c 独立证据）** | `9a57d01`；R1c report；**不借本 waiver** |
| **AC-27** | **AC-VERIFIED** | runbook + OPS-P4 drill 60 OK + OPS-P5b 真实 convergence + Reviewer 断言；waiver C+A scope 仅 AC-27-NOWB/LIVE，expiry 2026-12-31 |
| AC-61/62 | **AC-REF（不升级）** | manual replay 验收不使 AC-61/62 全量通过；D-24 release 不在本规程 |
| AC-72 / F7b | **不升级 / `[BLOCKED]`** | ALERTS/ENTRY 仍 DEFERRED；F7b 全链路仍无跨服务 harness |

**判定：AC-27 升 AC-VERIFIED；AC-27A 由 R1c 证据独立支撑；AC-61/62 仍 AC-REF；不升级 AC-72。**

---

## 5. 残余 / 边界

1. **测试替代边界**：D1 consumer 在测试进程内，Payment/Fulfillment 为既有 test double；未跑生产 gRPC/NATS 网络时序/UAT/CI。AC-27 的 waiver 接受“disposable MySQL 8.4 + drill + Reviewer”，但不得声称生产全链路/UAT。
2. **AC-61/62 仍 AC-REF**：manual replay 入口/AC-62 D-24 release 未全量；本报告不把 AC-61/62 一并升级。
3. **F7b**：仍 `[BLOCKED]`（AC-50 范围）；W8/AC-27-LIVE 的永久悬挂仅按 runbook/waiver 验收，不额外声称 AC-61 全量。
4. **AC-27A**：仅 R0.3-R1c 证据；不在 C+A waiver；未因本 L2 重开。
5. **AC-72 ALERTS/ENTRY** 仍 DEFERRED；`PAY-SETTLE-01` 仍冻结；`NOT RELEASE`、不 merge/Release/P0 closure。

---

## 6. INV / 宪法 / Governance

| 项 | 结论 | 依据 |
|---|---|---|
| INV-45 | **PASS** | D1 无 release；D5 PRE/POST 不打款重复；permit 不恢复；D7 abandon 不 release |
| I.1/I.4 | **PASS** | test-only；未加生产分支/未新造收敛路径 |
| I.2 Fail-Closed | **PASS** | 非法 JSON→DLQ；REVIEW 保持；重复幂等 |
| I.3 契约优先 | **PASS** | AC-27/27A 按 SPEC v1.5；AC-61/62 不越权 |
| II.1/II.3 | **PASS** | 事件不臆造 PRE；admission 唯一载体；重复幂等 |
| III.2/III.4 | **PASS** | 真实 MySQL/NATS；无 schema/RPC/worker |
| IV.1/IV.2 | **PASS（有范围）** | 真实日志；`-race`；test double/未跑全包已声明 |
| IV.4 | **PASS** | `NOT RELEASE`；不声称 AC-61/62/AC-72 全量 |
| Workflow 9 / Governance | **PASS** | L2；waiver C+A 记录；人类 merge pending |

**`[ARCHITECTURAL_FLAW]`：无。**

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-27
**报告**：`docs/collab/qc-reports/REV-PAY-OPS-REPLAY-01-R2.md`
