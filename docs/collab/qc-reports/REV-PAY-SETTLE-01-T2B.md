# REV-PAY-SETTLE-01-T2B — T2.4 scoped 复审（COMPENSATING ops + finance close/REJECTED）

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Tier | **L2（确认）**：资金/退款准入、legacy 队列、财务终态、跨服务 release 边界；不降级 |
| review_scope | **仅 T2.4**：COMPENSATING owner/SLA/queue/expiry、metrics/告警键、7 天 finance close、REJECTED；不审 T2.5/T2.6/T3/T4 |
| base | **`a18e7e0`**（T2.4 blocked/zero gate fix 前） |
| code head | **`4016c150d72c5ec9f7c0f82c00cd1317011693b4`**（4 files，+374/−34） |
| head（复审绑定） | **`deeac4a0479a593065696a3edee386a9e0d506e5`**（ADR/QC docs） |
| 契约输入 | ADR `0007-amd1-addendum-legacy-release.md` DRAFT 0.1（DEFER COMPENSATING success to T3）；QC `QC-PAY-SETTLE-01-T2-LEGACY-RELEASE.md` READY（0 BLOCKER/2 MAJOR/2 TODO）；SPEC-0002 v1.11 AC-63/INV-46；0019 |
| INV | **INV-44/45/46**：不伪造 release source/不恢复 permit/不重复退款；INV-46 仍部分（EXPIRED_REFUND T3） |
| verification | §1：真实 MySQL 8.4.13 + 0019；`TestLegacy` **all PASS 24.734s** / `-race` **25.501s**；`internal/platform/metrics` **0.119s**；0019 diff 空 |
| 日期 / Actor / Reviewed-by | 2026-11-27 / `reviewer-r04` / `human (merge pending)` |
| release_status | **NOT RELEASE** |
| overall verdict | **PASS（T2.4 scoped）**；T2.4 全量未完成：COMPENSATING→`EXPIRED_REFUND` 仍 `[BLOCKED]` T3；AC-63/INV-46 仍 **AC-REF** |

---

## 0. 结论

T2.4 的**可运维/财务收敛部分**成立，scoped PASS：

- **COMPENSATING owner/SLA/queue/expiry**：`docs/ops.md` 新增 24h/7d/COMPENSATING/finance 查询、人工认领、升级、禁止 release/Payment；`legacy_payment_resolution.lease_owner/lease_until` 只做认领，不改 state；worker 只处理 OPEN/RESOLVING 到期，不自动完成 COMPENSATING。
- **metrics**：`mall_legacy_payment_resolution_backlog{state}` / `oldest_seconds{state}` 由既有 worker tick 刷新（无新 worker/scanner）；4 个稳定告警键 `LEGACY_PAYMENT_RESOLUTION_BACKLOG`/`OLDEST`/`COMPENSATING_OLDEST`/`FINANCE_OVERDUE`。
- **7 天 EXPIRED_FINANCE**：既有 `ResolveLegacyResolution` 在 7 天内可写 `closed_at` 关闭；超 7 天返回 `LEGACY_FINANCE_ESCALATION_REQUIRED`，不关闭、不重开。
- **REJECTED**：`OPEN/RESOLVING` + 空 `payment_no` → `REJECTED`，终态不可重开；不写订单 settlement/permit/release/Payment。
- **0 release/0 Payment/0 EXPIRED_REFUND**：测试断言；worker 在 COMPENSATING 前缀后保持，不调用 release/Payment。
- **0019 字节不变；无新迁移/RPC/字段/worker**；`4016c15` 仅 4 files。

**T2.4 仍不是全量完成：** COMPENSATING 成功（INV-44 release + Payment.Refund + `EXPIRED_REFUND`）按 ADR DEFER 到 T3；`EXPIRED_FINANCE` 逾期仅升级、不自动关闭；T2.5/T2.6 未做。**AC-63/INV-46 保持 AC-REF，不升级。**

---

## 1. 实际执行（HEAD deeac4a / code 4016c15）

```bash
# MySQL 8.4.13，0019 真实迁移
TEST_MYSQL_DSN=.../mall_integration_t2b go test ./internal/services/order -run 'TestLegacy|TestT2' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_t2b_race go test -race ./internal/services/order -run 'TestLegacy|TestT2' -count=1
go test ./internal/platform/metrics -count=1
```

| 测试 | 结果 |
|---|---|
| `TestLegacyExpiryBranches` | **PASS**：EXPIRED_ZERO/EXPIRED_FINANCE×2/COMPENSATING 前缀/取消失败保持 |
| `TestLegacyResolutionMetricsAlertKeysAndAge` | **PASS**：backlog/oldest/4 稳定键 |
| `TestLegacyRejectedViaResolveCannotReopen` | **PASS**：REJECTED + 不可重开 |
| `TestLegacyExpiredFinanceSevenDayClose` | **PASS**：7 天内 closing + 不可重开 |
| `TestLegacyExpiredFinanceOverdueEscalatesWithoutClose` | **PASS**：逾期只升级、不关闭 |
| `TestLegacyZeroGate*`（T2a MAJOR 修复） | **PASS**：历史零元 gate、有捕获不 zero、不调 Payment |
| plain / `-race` | **all PASS；24.734s / 25.501s** |
| `internal/platform/metrics` | **PASS 0.119s** |
| `migrations/0019` / code scope | **diff 空**；4 files，无新迁移/RPC/worker |

原始日志：`.tmp/evidence-t2b/reviewer-t2b.log`（本次审查生成，未入库）。

---

## 2. 逐项核对

| 检查 | 结论 | 证据 |
|---|---|---|
| COMPENSATING owner/SLA/queue/expiry | **PASS** | `docs/ops.md` T2.4 段；owner ops+finance，24h/7d，认领/升级；`legacy_resolution_worker.go` 只处理 OPEN/RESOLVING |
| metrics backlog/oldest + 4 alert keys | **PASS** | `metrics.go` GaugeVec + constants；`refreshLegacyResolutionGauges` 既有 tick；测试 |
| 7 天 EXPIRED_FINANCE 关闭 | **PASS** | `ResolveLegacyResolution` `closedAt`/`resolvedAt` 分支；测试 |
| 逾期升级不关闭 | **PASS** | `LEGACY_FINANCE_ESCALATION_REQUIRED`；测试 |
| REJECTED 不可重开 | **PASS** | `ResolveLegacyResolution` default/REJECTED 分支；测试 |
| 0 release/0 Payment/0 EXPIRED_REFUND | **PASS** | worker 前缀保持 COMPENSATING；测试断言 |
| 无新 worker/迁移/RPC | **PASS** | `4016c15` 仅 metrics/worker/docs/tests；`migrations/0019` diff 空；`ResolveLegacyResolution` 既有 RPC |
| T2a 零元 MAJOR | **PASS（已修）** | `refund_admission.go` system_zero 先解析；`TestLegacyZeroGateHistoricalOrder` |
| COMPENSATING→EXPIRED_REFUND | **`[BLOCKED]`（T3）** | ADR §0/§4；release source 封闭枚举；不写 release/Payment |
| T2.4 全量完成 | **NO** | ADR DRAFT/DEFER；AC-63/INV-46 AC-REF |

---

## 3. ADR / 残余

- **ADR 状态 DRAFT 0.1 — pending QC/human**：QC `READY（0 BLOCKER/2 MAJOR/2 TODO）`；T2.4 代码已按裁决保持 COMPENSATING 前缀、未新增 `legacy_expired` source。**ADR 人类接受仍未记录**；T3 前需人类批准/errata，不能把 DRAFT 当 accepted。
- **QC M-1/M-2**：ops owner/SLA/queue 与 4 告警键已补；但告警规则部署、真实 ticker 1 分钟、UAT/prod 未执行；metrics 只有进程内值。
- **COMPENSATING 成功/`EXPIRED_REFUND`**：`[BLOCKED]`，需 T3 的 release 完成/回执/多组策略；本步不碰 T3/INV-44 source。
- **7 天逾期**：只返回升级要求，无更高授权角色字段/RPC；人工线下升级；不是自动关闭。
- **T2.5/T2.6**：历史回填/dry-run/`legacy:<order_no>:<seq>` 取号未做；本步不声称 T2 完成。
- **AC-63/INV-46**：仍 **AC-REF**；`ac-coverage.tsv` 不改；AC-63 不得 VERIFIED，`EXPIRED_REFUND` 归 T3。

---

## 4. 宪法 / Governance

| 项 | 结论 | 依据 |
|---|---|---|
| I.1/I.4 | **PASS** | 不新增 release source；不用告警/指标冒充 EXPIRED_REFUND |
| I.2 Fail-Closed | **PASS** | COMPENSATING 保持；逾期不关闭；REJECTED 终态 |
| I.3 契约优先 | **PASS（有 DRAFT）** | ADR DEFER T3；未改 SPEC/AC-63 |
| II.1/II.3 | **PASS** | 7 天/REJECTED 与 release 解耦；终态不可重开 |
| III.2/III.4 | **PASS** | 无新表/RPC/worker；metrics 既有 tick |
| IV.1/IV.2 | **PASS（有范围）** | 真实 MySQL/0019、`-race`；未跑 UAT/prod/告警部署 |
| IV.4 | **PASS** | `NOT RELEASE`；不批准 merge/Release |
| Workflow 9 / Governance | **PASS** | T2.4 scoped；人类 merge pending；ADR DRAFT 显式登记 |

**`[ARCHITECTURAL_FLAW]`：无。** COMPENSATING 成功属于 T3 release 合约缺口，不应在 T2.4 用 source 标签/提前 Payment 绕过。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-27
**报告**：`docs/collab/qc-reports/REV-PAY-SETTLE-01-T2B.md`
