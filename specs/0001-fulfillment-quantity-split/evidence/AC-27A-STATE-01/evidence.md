# evidence — AC-27A-STATE-01

| 项 | 值 |
|---|---|
| change_id | `AC-27A-STATE-01` |
| 级别 | 实现变更 |
| 授权 Spec | `specs/0001-fulfillment-quantity-split/spec.md` v1.6（2026-11-28 批准）AC-27A 第 5/6 条与 IF |
| 关联 ADR | `docs/adr/0005-fulfillment-quantity-split.md` Rev 1.3 §21（本变更不改 ADR） |
| base | `dee75b0` |
| head | `16e472e`（本证据绑定的实现提交；`git rev-parse HEAD`） |
| 阶段 | 代码 |
| 结论 | 实现完成，**待独立代码审查**；不声称 AC-VERIFIED、不是 Release |

## 1. diff 范围（`dee75b0..16e472e`）

| 文件 | blob | 变更 |
|---|---|---|
| `internal/services/order/fulfillment_groups.go` | `5881df4` | `PreShipAdmission` 的 EXISTS 加 `AND a.state = a.result`；注释改为 v1.6“有效 PRE” |
| `internal/services/order/settlement.go` | `f98f996` | `groupIsClosed` 第 5 条注释补 `state=result`（逻辑不变） |
| `internal/services/order/ac27a_closure_integration_test.go` | `9043b6c` | 新增负例：`result=PRE, state=PENDING_REMOTE` → `PreShipAdmission=false`、`groupIsClosed=false`、`effectiveGroupStatus` 保持原状态 |
| `.../evidence/AC-27A-STATE-01/plan.md`、`tasks.md` | 新增 | 本 change 的 plan/tasks |

## 2. 测试范围（实际执行）

环境：MySQL 8（容器 `mysql-unicorn`，127.0.0.1:3306）、Go 1.27.1、仓库本地 GOCACHE/GOMODCACHE、`GOFLAGS=-mod=readonly`。

| 命令 | 结果 |
|---|---|
| `go build ./...` | PASS |
| `make fmt-check vet`（`go vet ./...`） | PASS |
| `TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_integration_test?parseTime=true&loc=UTC' go test ./internal/services/order -count=1` | `ok gin-kratos-gorm/internal/services/order 255.698s` |
| `TEST_MYSQL_DSN=... go test -race -p 1 ./internal/services/order -run 'TestAC27A|TestGroupRefundStateProjection|TestMultiGroupShadow' -count=1` | `ok ... 4.388s` |

新增用例（`TestAC27AAdmissionResultDerivation`）：
- `pre_ship_revoked closes`：`result=PRE,state=PRE` → closed（回归）
- **`pre_ship_revoked with mismatched state stays open`：`result=PRE,state=PENDING_REMOTE` → 未关闭（新增）**
- `review_required stays open`、`pending_remote placeholder stays open`、`post_ship_allowed stays open`、`late shipped status wins`、`no admission row stays open`：保持

## 3. 未覆盖范围

- **未跑 `make verify` 全套**（全仓 `test-race`、`arch`、`dtm-contract`、`migrations-check`、`proto-guard`）：未覆盖，不计 PASS。
- 未跑其它包集成测试（共享 schema，按 `docs/development.md` 不并行）。
- 未做生产/存量数据审计：是否存在 `result=PRE 且 state≠result` 的行属 `AC-27-NOWB-01` 一次性审计（§16.2），本 change 不做数据修复。
- 未触及 T5.2/T5.3 证据；PAY-SETTLE T5 仍冻结。
- 未验证 AC-27A 的完整端到端（Fulfillment 侧事件/告警）；本 change 只改 Order 读模型派生。

## 4. 取舍

- **fail-closed**：异常/存量 `result=PRE,state≠result` 行不关闭组（与 AC-27A v1.6、SPEC-0002 v1.11 AC-69 一致），而不是“按 result 关闭 + 告警”。
- **只改读模型派生，不改写路径**：所有 PRE 写入点均为 `result=state=PRE_SHIP_REVOKED`（`refund_admission.go:523/551/557/952`），无需改写入逻辑。
- **不做数据回填**：避免在本 change 混入迁移；存量异常行交 `AC-27-NOWB-01` 审计。
- 未在查询中加 `state = ?` 常量，而用 `a.state = a.result`：直接表达“state 落在 result 上”的契约，且对 PRE/POST 同样成立。

## 5. 风险与回滚

- **行为差异**：仅 `result=PRE 且 state≠result` 的行由“已关闭”变为“未关闭”；当前写入路径不产生该组合。若生产存量存在，方向为“不误关组”，但订单展示/可操作项可能变化，需按 §16.2 `AC-27-NOWB-01` 审计。
- **回滚**：`git revert 16e472e` 即恢复原谓词；无 schema/数据变化。

## 6. AC 映射（待审查者裁决）

| AC | 检查 | 本证据可背书 |
|---|---|---|
| AC-27A 第 5 条 | 新增负例 + 既有正例 | 有引用 |
| AC-27A 第 6 条 | 既有用例（REVIEW/PENDING/POST/missing） | 有引用 |
| AC-27A IF（告警） | 不新增告警路径；异常行归第 6 条未关闭 | 有引用 |
| AC-27（S2 聚合） | `TestGroupRefundStateProjection`/`TestMultiGroupShadow` | 有引用 |
