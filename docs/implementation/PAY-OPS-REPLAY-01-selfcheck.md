# PAY-OPS-REPLAY-01 OPS-P4 自检清单（人工规程 + disposable 演练）

| 项 | 值 |
|---|---|
| change_id / Parent | `PAY-OPS-REPLAY-01` / `PAY-SETTLE-01`（暂停） |
| Role / Actor | Developer / `dev-r04-fix` |
| Tier | **L2**（P0、资金/退款、跨服务恢复、人工运维） |
| 契约输入 | `docs/adr/0007-amd1-addendum-ops-manual.md` DRAFT 0.1；`QC-PAY-OPS-REPLAY-01-MANUAL-R2.md` READY（0 BLOCKER/3 TODO）；`docs/review-tracking.md` 2026-11-27 人类 waiver C+A；SPEC-0001 v1.5 §16.2、SPEC-0002 v1.11 AC-61/62；Plan/Tasks OPS-P4 |
| Base | `1e5e7a0`（human acceptance of manual residual） |
| **Code head（演练绑定）** | **`5ef1b357fe6d62fe7dea2c15980ab8149d537d33`** |
| 实际改动 | `docs/ops.md`（+250 行新节，既有语义未改）、`scripts/drill-ops-replay.sh`（+545，可为 0/non-0）、本文档 |
| 未改 | `internal/**`、`api/**`、`migrations/**`、`deploy/**`、`frontend/**`、`docs/specs/**`、`docs/adr/**`、台账 |
| Status | **WIP**；不声称 AC-VERIFIED/Release PASS；AC-27 由 Reviewer 按 C+A 决定 |

## 1. Runbook（docs/ops.md）

新增 “Manual ops runbook (PAY-OPS-REPLAY-01: DLQ replay + NOWB re-arm/abandon)”，覆盖 QC MANUAL-R2 §2 的 7 项：

1. **人类残余签署句**：无密码学不可抵赖、无 ops 工具；命名账号/双人/变更单/DBA 审计/Reviewer 补偿；不得声称 AC-62 技术闭环或工具已实现；waiver 仅 AC-27-NOWB/LIVE（AC-27A 仍 AC-REF，AC-61/62 不全量）。
2. **授权**：`authorization_id`/`op_id`/`action`/identifiers/amount/operator/authorizer/reviewer/reason/evidence_ref/authorized_at/expires_at(≤24h)/ticket=RVW/owner_approval_ref；≥100000 cent 必须有 reviewer 且与 authorizer 不同；任一缺失/过期/同人/越权 → 拒绝零写；ticket ≠ 授权单。
3. **D1/D2 DLQ replay**：定位键、只读预检、7 天窗口、原 `data` 字节 + `Nats-Msg-Id=manual:<auth>:<seq>` 发布、JetStream 去重与投影幂等、审计、失败停止。
4. **D3 租约**：优先 worker；手动 fallback 为 task 行锁 CAS（status+lease，不伪造 version），保留 `last_error`，不碰 admission/金额。
5. **D5/D5b/D6/D10 re-arm**：S6 仅 Fulfillment 的 `shipments`/`shipment_events`/归档/承运商回执；必须 `order_no+group_no` 且由 Fulfillment authorizer 签字/归档；先审计后 CAS：admission `REVIEW_REQUIRED→PENDING_REMOTE`（version CAS）、batch `MANUAL_HOLD→ADMITTING`（保留 canonical last_error）、task `PENDING/attempts=0`（无 version 列，行锁）；只由既有 worker/`TccCancelMarkPaid` 重驱；无 driver → 停；CANCELLED→PRE、SHIPPED→POST、仍无运单→REVIEW；错组/伪造拒绝且零写。
6. **D7/D7b/D11 abandon**：task/app `FAILED_PERMANENT`，canonical `last_error` 保留，写 `after_sale_logs`；admission 仍 REVIEW、commitment RESERVED、permit 原值、组不关闭、0 Payment/0 release；TCC app FAILED → `TCC_APP_ABANDONED`；用户再 Refund 是新评审（无代码 guard，工单拦截，不自动重试/关闭）；legacy 无 carrier 拒绝。
7. **D4/D8/D9 + 失败/回滚/禁止项**：FAILED_PERMANENT 拒绝同 task；授权/证据拒绝；并发/崩溃只补未完成步骤；生产失败不回滚已提交 CAS、不清 last_error/删 admission；禁止 `cmd/ops`/`0024`/授权表/新 RPC/新迁移/自动 re-arm/release/close/恢复 permit/伪造 S6/共享账号。

## 2. Drill 结果（真实 disposable 环境）

```bash
./scripts/drill-ops-replay.sh         # EXIT=0
```

- HEAD **`5ef1b35`**；MySQL **8.4.13**（`mysql:8.4`，127.0.0.1:13401）；NATS **2.10-alpine** JetStream（127.0.0.1:14301）；schema 由 **`mall/migrate:dev`** 真实迁移创建（`mall_order`/`mall_aftersales`/`mall_fulfillment`）。
- 结果：**60 OK / 0 FAIL，DRILL PASSED**（日志 `.tmp/evidence-ops-replay/drill-run.log`；迁移 `.tmp/evidence-ops-replay/migrate.log`；NATS helper stderr `nats.err`）。
- D1–D11 实际断言：
  - **D1**：合成 DLQ wrapper 提取原 `data` 字节（`cmp` 相等）；同一 `Nats-Msg-Id` 重发两次 JetStream 仅 +1；`refunded_cent` 与 ledger 快照不变。
  - **D2**：无 DLQ 消息时 helper 非 0，脚本 fail-closed。
  - **D3**：task 行锁 CAS 复位 `PENDING/0/lease 空`，canonical `last_error` 保留，admission 不变。
  - **D4**：`FAILED_PERMANENT` task 保持，且不产生新 admission（runbook 拒绝口径）。
  - **D5**：CANCELLED 证据 + 授权通过；admission CAS=1、batch CAS=1、`PENDING_REMOTE`、batch `ADMITTING`、canonical last_error/app REVIEW/permit CONSUMED/0 release。
  - **D5b**：错组（G2 证据用于 G1）拒绝 `S6_GROUP_MISMATCH`，快照零变化。
  - **D6**：SHIPPED 证据映射 `POST_SHIP_ALLOWED`（不是 PRE），admission 未变、0 release。
  - **D7**：abandon task `FAILED_PERMANENT`、last_error canonical、admission REVIEW、commitment RESERVED、permit CONSUMED、group refund_state NONE、refunded_cent 0、0 release、`after_sale_logs` 1 行。
  - **D7b/D11**：task 不被自动重开；TCC app `FAILED_PERMANENT`、batch `MANUAL_HOLD` + canonical、commitment RESERVED、admission REVIEW、permit 原值、0 release。
  - **D8**：≥1000 无 reviewer、过期、reviewer=authorizer、abandon 无 owner、伪造 authorization_id 全部拒绝；快照零变化。
  - **D9**：两个并发 CAS 仅 1 个成功，version 只 +1。
  - **D10/D10b**：缺证据/错组拒绝，shipments 行数不变。
  - **final**：0 release、permit CONSUMED、refunded_cent 0、每 refund 1 admission、TCC app 无重复。

## 3. 【测试范围】/【未覆盖范围】

【测试范围】真实 schema 上的 runbook SQL（授权/证据前置、re-arm 双 CAS、task 无 version 的 lease CAS、abandon、日志/审计、D1 DLQ wrapper 提取 + JetStream 去重、D9 并发、拒绝零写）与最终不变式（无重复退款/释放/permit 恢复）。

【未覆盖范围】

- **未启动 Order/AfterSales/TCC 服务/consumer**：D1 验证 DLQ 提取、原字节发布、JetStream 去重与 DB 快照不变，**不**验证真实 consumer 投影收敛；D5 验证 re-arm CAS，**不**验证 worker/TCC re-drive 后的 PRE/POST（manual route 无代码/服务运行）。D7b“用户再 Refund 重开 task”是代码现状（`workers.go:915-929`），drill 只验证不自动重开，不能覆盖真实用户路径。
- 演练数据为 disposable fixture（明确非生产证据）；DLQ wrapper 由脚本合成（无 nats CLI 时用于验证 wrapper→data 字节流），未从真实 DLQ 消费。
- 未执行生产/UAT；未接 CI；`nats` CLI 未安装，NATS 操作经一次性 `go run` helper（仅演练）。
- AC-27A/AC-61/62 全量、Release/merge 不在本自检；AC-27 的 AC-VERIFIED 由 Reviewer 按 C+A waiver + 本 drill/日志决定。

## 4. 反妥协自检 / 返工影响声明

- **并发/失败/部分成功**：D9 CAS 单胜；D5 双 CAS 事务；D7/D11 保留负债；拒绝路径零写；D3 行锁；未自动反向改状态。
- **是否加 if-else/局部补丁**：无代码；规程只使用既有表/任务/入口，未新增工具/表/RPC；授权/证据拒绝是 runbook 前置，不是代码 guard（限制已签署）。
- **妥协（高亮）**：① 无技术不可抵赖/无代码 guard（人类 C+A 已接受）；② drill 不运行服务 consumer，D1/D5 的“收敛/重驱”只到 SQL/NATS 机制与快照层；③ DLQ wrapper 为合成 fixture；④ S6 group 依赖 Fulfillment authorizer 签字/归档（waiver 控制）。
- `[修改点]`：`docs/ops.md` 新节、`scripts/drill-ops-replay.sh`、本文档。
- `[架构侵入性]`：无代码/schema/RPC/worker 变更；只新增文档与一次性脚本。
- `[副作用评估]`：runbook 禁止项明确；drill 仅 disposable 端口/容器（13401/14301），trap 清理；未接 CI/prod。

## 5. 不声称

不声称 OPS-P4 PASS / merge / Release PASS / AC-VERIFIED；AC-27/AC-61/62 的验收由 Reviewer 按 C+A waiver 与本 drill 证据决定；AC-27A 仍 AC-REF；工具路线（`cmd/ops`/`0024`/授权表/新 RPC）仍 WITHDRAWN，不得复活。
