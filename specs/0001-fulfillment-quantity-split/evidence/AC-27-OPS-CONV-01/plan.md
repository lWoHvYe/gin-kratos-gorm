# plan — AC-27-OPS-CONV-01（test-only 收敛断言）

| 项 | 值 |
|---|---|
| change_id | `AC-27-OPS-CONV-01` |
| 级别 | **实现变更（test-only，无生产代码）** |
| 授权 Spec | `specs/0001-fulfillment-quantity-split/spec.md` v1.6 AC-27A IF / §16.2 `AC-27-NOWB-01`、`AC-27-LIVE-01`；`specs/0002-payment-settlement-admission/spec.md` v1.11 AC-50/AC-69/AC-71 |
| 关联 ADR | `docs/adr/0005-…` Rev 1.3；`docs/adr/0007-…` Rev 2.3 |
| 人类豁免 | 2026-11-27：`PAY-OPS-REPLAY-01`/`AC-27-NOWB-01`/`AC-27-LIVE-01` 以 **manual runbook + disposable drill** 代替 `cmd/ops` 工具入口 |
| base | `cd34f70` |
| 阶段 | 代码（test-only） |

## 1. 目标

补齐台账 2026-11-27 记录的“minimal remaining: test-only convergence with existing consumer/worker (no production code)”，使 `AC-27-NOWB-01` 终态 ① 与 `AC-27-LIVE-01` 的重放收敛有**真实断言**，而不是 runbook 文字或合成 wrapper。

## 2. 改动面

| 项 | 影响 |
|---|---|
| 测试 | `internal/services/order/ops_replay_convergence_test.go` 新增 `TestOpsReplayNowbRearmConvergesToPre` |
| 生产代码 | **无** |
| schema/迁移 | 无 |
| 依赖 | 需要 `TEST_MYSQL_DSN` + `TEST_NATS_URL`；缺任一则 SKIP（不是 PASS） |

## 3. AC → 检查

| 条款 | 检查 |
|---|---|
| AC-27A IF / `AC-27-NOWB-01` 终态 ①（真实 S6 证据 → re-arm → PRE → 组关闭） | 新用例：CONSUMED+无运单 → REVIEW_REQUIRED；runbook re-arm（只清复核标记）；Fulfillment 返回 `CANCELLED`；`AdmitRefund` → PRE/PRE；`groupIsClosed`=true、`effectiveGroupStatus`=CANCELLED；`orders.status` 不变；无 Payment 调用 |
| `AC-27-LIVE-01`（DLQ/租约丢失 → 重放收敛） | 复用 OPS-P5b：`TestOpsReplayAfterSaleCompletedConsumerConvergence`（真 consumer + DLQ）、`TestOpsReplayTCCRearmReDriveConvergence`（re-arm 后 worker 重驱到 PRE 且只打款一次）、`TestOpsReplayTCCRearmPostConvergence`（POST） |
| AC-62 人工放弃 | 不在本 change；归 §16.2 `AC-27-NOWB-01` 终态 ② / AC-62 D-24（人工 runbook） |

## 4. stop_conditions

1. 需要改生产代码（例如新增 AC-61 replay 入口）→ 停止，回 Spec/ADR + 人类批准。
2. 测试需要放宽断言/fixture 冒充（如用 `RELEASED` fixture 冒充释放）→ 停止。
3. 无 MySQL/NATS 时 → 记录 SKIP/未覆盖，不得 PASS。

## 5. 交付

- 证据：`evidence.md`（绑定 HEAD、命令、结果、未覆盖）。
- 提交尾注：`change_id: AC-27-OPS-CONV-01` / `Actor: lead` / `Reviewed-by: pending`。
- 独立 reviewer 代码阶段审查；`AC-27-NOWB-01`/`AC-27-LIVE-01` 是否可关闭由审查者裁决，不由作者自标。
