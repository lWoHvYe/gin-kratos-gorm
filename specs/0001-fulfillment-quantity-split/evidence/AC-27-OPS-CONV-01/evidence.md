# evidence — AC-27-OPS-CONV-01

| 项 | 值 |
|---|---|
| change_id | `AC-27-OPS-CONV-01` |
| 级别 | 实现变更（test-only，无生产代码） |
| 授权 Spec | `specs/0001-fulfillment-quantity-split/spec.md` v1.6 AC-27A IF、§16.2 `AC-27-NOWB-01`/`AC-27-LIVE-01`；`specs/0002-…/spec.md` v1.11 AC-50/AC-69/AC-71 |
| base | `cd34f70` |
| head | `8040776`（本证据绑定的提交） |
| 阶段 | 代码（test-only） |
| 结论 | 收敛断言已补，**待独立代码审查**；`AC-27-NOWB-01`/`AC-27-LIVE-01` 是否可关闭由审查者裁决，作者不自标 |

## 1. diff 范围（`cd34f70..8040776`）

| 文件 | 变更 |
|---|---|
| `internal/services/order/ops_replay_convergence_test.go` | 新增 `TestOpsReplayNowbRearmConvergesToPre`（NOWB 终态 ①） |
| `.../evidence/AC-27-OPS-CONV-01/plan.md`、`tasks.md` | 本 change 的 plan/tasks |

**无生产代码改动。**

## 2. 测试范围（实际执行；MySQL 8 容器 `mysql-unicorn` + NATS 2.x 127.0.0.1:4222）

| 命令 | 结果 |
|---|---|
| `go build ./...` | PASS |
| `go vet ./internal/services/order` | PASS |
| `TEST_MYSQL_DSN=… TEST_NATS_URL=… go test ./internal/services/order -run 'TestOpsReplay' -count=1 -v` | `ok … 4.581s`；4/4 PASS |
| `TEST_MYSQL_DSN=… TEST_NATS_URL=… go test ./internal/services/order -count=1` | `ok … 206.227s`（全包，非 SKIP） |

4 个对应用例：
- **`TestOpsReplayNowbRearmConvergesToPre`（新增）**：CONSUMED+无运单 → `REVIEW_REQUIRED`/`PENDING_REMOTE`；组保持未关闭；runbook re-arm（只清复核标记、同 `refund_id`、`version+1`）；Fulfillment 返回 `CANCELLED`；`AdmitRefund` → `PRE/PRE`；`loadFulfillmentGroups` → `groupIsClosed=true`、`effectiveGroupStatus=CANCELLED`；`orders.status` 保持原值；`Payment.Refund` 0 次。
- `TestOpsReplayAfterSaleCompletedConsumerConvergence`（OPS-P5b D1）：真 `shop.aftersale.completed.v1` consumer，投影 REFUNDED、不改 admission、重复幂等、坏消息进 DLQ。
- `TestOpsReplayTCCRearmReDriveConvergence`（D5 PRE）：re-arm 后 `TccCancelMarkPaid` 重驱到 PRE 且只打款一次，重复幂等。
- `TestOpsReplayTCCRearmPostConvergence`（D5 POST）：已离仓组重驱到 POST。

独立 oracle：直接种子 `order_refund_admissions`/permit/group 后经**生产 `AdmitRefund` 路径**重驱，断言 DB 事实与读模型派生；不依赖作者自述。

## 3. 未覆盖范围

- **AC-62 人工放弃（NOWB 终态 ②）**：未实现/未断言；归 2026-11-27 manual runbook 与 AC-62/D-24。
- **`AC-27-LIVE-01` 的“工具入口”**：2026-11-27 人类决策以 manual runbook + drill 代替 `cmd/ops`；本 change 不新增工具入口（生产代码为 no-code 范围）。
- 未跑 `make verify` 全套（arch/dtm-contract/migrations-check/proto-guard）；其它包集成未跑。
- 存量 `result=PRE,state≠result` 数据审计归 §16.2 `AC-27-NOWB-01` 一次性审计，本 change 不做数据修复。
- 未覆盖 AfterSales durable task 租约丢失（需 AfterSales 包与故障注入）。

## 4. 取舍

- **test-only**：按台账“minimal remaining”约束，不新增生产代码；re-arm 用 runbook 的 SQL 步骤模拟，不伪造工具。
- **用生产 `AdmitRefund` 重驱**而不是直接 UPDATE 成 PRE：确保断言覆盖真实准入路径与落库。
- **不把 AC-27 标 AC-VERIFIED**：`AC-27-NOWB-01`/`AC-27-LIVE-01` 的关闭与 AC-27 升级归审查者与人类。

## 5. AC 映射（待裁决）

| 条款 | 检查 | 本证据可背书 |
|---|---|---|
| `AC-27-NOWB-01` 终态 ① | 新增用例 | 有引用 |
| `AC-27-LIVE-01` 重放收敛 | OPS-P5b D1/D5 复跑 | 有引用 |
| `AC-27-NOWB-01` 终态 ② / AC-62 | 未覆盖 | 未验证 |
