# QC-PAY-SETTLE-01-T2-LEGACY-RELEASE — T2.4 legacy `COMPENSATING` 裁决审查

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01`（T2.4） |
| 审查对象 | `docs/adr/0007-amd1-addendum-legacy-release.md` DRAFT 0.1（114 行）；`docs/adr/0007-amd1-legacy-release-impact.md`（74 行）；SPEC-0002 v1.11 AC-63（`:346-362`）/INV-46（`:142`）/S16（`:86`）；T2 selfcheck §7.3 |
| 结论 | **READY（0 BLOCKER；2 MAJOR；2 TODO）** |
| 日期 | **2026-11-27** |
| Actor | `qc-ac50`（Quality Checker；与 `arch-legacy-release` 不同 actor） |
| base/head | Base `2507a86`；代码证据 `e6d8bf9`；当前 head `a18e7e0` |
| release_status | **NOT RELEASE**；AC-63/INV-46 保持 **AC-REF**，不得 AC-VERIFIED。 |

## 0. 结论

**READY（0 BLOCKER）。** 裁决**守住 AC-63 的顺序**：T2.4 只保留 `COMPENSATING` 前缀（撤 `NONE` permit、`CONSUMED+CREATED` 的 `CancelIfCreated`），**不写 release event、不调 Payment、不置 `EXPIRED_REFUND`**（ADR `§0/§4`；`legacy_resolution_worker.go:180-236`）。这正确：现有 INV-44 producer 只写 outbox 请求，**无 release 完成回执/ack**（`refund_release.go:216`），仅加 `legacy_expired` 只是“加标签”，不能证明 release 完成；Payment 若先执行会违反 AC-63 `:360-361`。**没有把“加标签”误当 release 完成**。

- **不新增 `legacy_expired` source/reason**：正确；source 是封闭枚举（`refund_release.go:32-48`、`events.go:49-57`、`registry.go:200-205`），加标签不解决完成度/多组/旧 consumer DLQ。
- **T3 边界清楚**：T2.4 不越权；T3 拥有 release/commitment 持久任务、回执/ack、多组策略、人工处置（ADR `§2/§3`；T3 tasks `:93-99`）。
- **7 天/`REJECTED` 独立**：不依赖 release source；`EXPIRED_FINANCE` 按 D-23 7 天财务关闭、`REJECTED` 脏数据终态，均不得冒充 `EXPIRED_REFUND`（ADR `§4/§5.1`；SPEC `:364,:367`）。
- **AC-63/INV-46 保持 AC-REF**：ADR `§5.1` 明确；T2.4 不得声称完成；L-1..L-6 证据只能支撑“前缀/独立分支”，不能支撑 `EXPIRED_REFUND`。

**2 MAJOR（不阻断裁决，但阻断 T2.4 的 scoped 验收/运营闭环）：**

1. **`COMPENSATING` 的长期停留缺明确 owner/SLA/manual queue/expiry**。ADR `§4` 说“告警/指标；租约内重试”；`§5.1` 说“可能长期持有（有告警/SLA）；需要 ops 监控/人工”；但**没有**：
   - `legacy_payment_resolution` 的 owner（ops/Developer/Order）、首次响应/最长停留 SLA、升级路径；
   - 人工队列/工单字段（`authorization_id`? 仅查询?）；
   - 何时允许人工置 `EXPIRED_FINANCE`/`REJECTED`（`EXPIRED_FINANCE` 只用于已离仓/不确定，不用于 `COMPENSATING`；`REJECTED` 只脏数据）。
   不补则 `COMPENSATING` 可在 T3 前无限停留（AC-63 要求失败保持并告警，但告警需有 owner/阈值）。
2. **T2.4 的告警/指标未定义具体键**：ADR `§4` 只说“告警/指标”；未给 `legacy_compensating_blocked`/oldest/`REFUND_RELEASE_SOURCE_UNSUPPORTED` 计数、阈值、日志字段。L-2 的“告警”无法验收；Reviewer 只能看到状态不变。

**2 TODO**：L-1..L-6 的 SQL/日志/期望需绑定 T2.4 HEAD；T3 的 `legacy_expired` source 未来必须 consumer-first、release receipt/ack、多组 P1/manual；本 ADR 已写，但需 Reviewer 在 T3 计划复核。

## 1. 契约核对

| AC-63/INV-46 要求 | T2.4 当前 | 判定 |
|---|---|---|
| 撤未离仓组 permit | T2.4 撤 `NONE` permit | **PASS** |
| `CancelIfCreated` | T2.4 对 `CONSUMED+CREATED` 调 | **PASS** |
| INV-44 释放 | **不写 release event**（source unsupported） | **PASS（前缀）/DEFER** |
| Payment.Refund 最后 | **不调 Payment** | **PASS（前缀）** |
| 全部成功才 `EXPIRED_REFUND` | **不置**，保持 `COMPENSATING` | **PASS** |
| 失败/超时保持 `COMPENSATING` 并告警 | 保持；告警未定义键（M2） | **PARTIAL** |
| 7 天 `EXPIRED_FINANCE`/`REJECTED` 独立 | 可独立；不得冒充 EXPIRED_REFUND | **PASS（边界）** |
| AC-63/INV-46 AC-REF | ADR 明确 | **PASS** |

## 2. 关键判断

### 2.1 “只加 source/reason”是否足够

**不够。** `refund_release.go:216` 只有 outbox 请求；Inventory consumer 异步执行，无 receipt/ack/scan worker；Order 无法区分 release pending/succeeded/failed。加 `legacy_expired` 只改变标签，**不能**满足 AC-63 `:360-361` 的“release 完成→Payment 最后/失败保持”。ADR 拒绝该方案正确。

### 2.2 T3 边界

T3 拥有 release/commitment（T3.1–T3.7），需：
- release 完成/失败回执（receipt/lease/ack）；
- 多组策略（当前 INV-44 仅单组 `group_count=1`，`refund_release.go:174-186`；多组 P1/manual）；
- legacy source `legacy_expired` + `LEGACY_EXPIRED_REFUND`，**consumer-first**；
- `release_id=rr:<refund_id>`，Payment 仅在 release 完成后；
- `[CONTRACT-IMPACT]` 若需 receipt/ack 表/RPC 必须人类批准。
T2.4 **不得**越权实现；ADR 边界清楚。

### 2.3 `COMPENSATING` 是否不可收敛

**会长期停留，但不等于“无限不可收敛”**：AC-63 明确失败保持并告警；T3 交付前不得自动 `EXPIRED_REFUND`/回 `OPEN`。需要 **M1/M2** 的 owner/SLA/人工队列/告警键，否则 `COMPENSATING` 无终止路径。`EXPIRED_FINANCE` 7 天只适用于已离仓/不确定，**不能**用于 `COMPENSATING`；`REJECTED` 只用于脏数据，不能当“等待补证”。

### 2.4 7 天/`REJECTED` 独立

**可以做，但边界清楚**：L-5/L-6 只验证独立分支；不得用其 PASS 声称 AC-63 完成；`EXPIRED_FINANCE` 不是 `EXPIRED_REFUND`，`REJECTED` 不是“未查到/等待”。ADR `§4/§5.1` 已写；Reviewer 必须分开登记。

## 3. MAJOR/TODO 清单

- **[MAJOR] M-1** `COMPENSATING` 长期停留：补 owner（ops/Developer/Order）、SLA（首次响应/最长/升级）、`legacy_payment_resolution` 人工队列/工单字段、允许的人工终态（仅 `EXPIRED_FINANCE`/`REJECTED` 的合法条件）与禁止项。
- **[MAJOR] M-2** 告警/指标：定义 `legacy_compensating_blocked`/oldest/`REFUND_RELEASE_SOURCE_UNSUPPORTED` 计数/阈值/日志字段；L-2 的“告警”必须可观测。
- **[TODO] T-1** L-1..L-6 绑定 T2.4 HEAD 的 SQL/日志/期望；`CancelIfCreated`/permit/`0 release/Payment/EXPIRED` 断言写清。
- **[TODO] T-2** T3 复核：consumer-first、release receipt/ack、多组 P1、`[CONTRACT-IMPACT]` 人类批准；本 ADR 不实施。

## 4. 结论

**READY（0 BLOCKER）。** 裁决正确守住 AC-63 顺序，拒绝“加标签当完成”，T3 边界清楚，7 天/`REJECTED` 独立且不冒充 AC-63；AC-63/INV-46 保持 AC-REF。**M-1/M-2 必须在 T2.4 scoped 验收前补**（owner/SLA/告警键），否则 `COMPENSATING` 无运营闭环；**不阻断本契约裁决**。**不得**在 T3 前写 release/Payment/`EXPIRED_REFUND`，不得新增 `legacy_expired` source。

**只读声明**：未执行代码/测试；只审 ADR/impact/SPEC/T2 selfcheck；未改代码/SPEC/ADR/迁移；只新建本报告；未 commit。

**Actor**：`qc-ac50`
**Status**：**READY（0 BLOCKER；2 MAJOR；2 TODO）**
