# QC-PAY-OPS-REPLAY-01-MANUAL-R2 — OPS-P2 终审（记录 waiver）

| 项 | 值 |
|---|---|
| change_id | `PAY-OPS-REPLAY-01`（Parent `PAY-SETTLE-01` 暂停；工具路线 WITHDRAWN） |
| 审查对象 | `docs/adr/0007-amd1-addendum-ops-manual.md` **DRAFT 0.1**（blob `4e8c218`）；`docs/review-tracking.md:458-470` 人类 waiver；Plan/Tasks；SPEC-0001 §16.2、SPEC-0002 AC-61/62 |
| 结论 | **READY（with recorded waiver；0 BLOCKER；3 TODO）** |
| 日期 | **2026-11-27** |
| Actor | `qc-ac50`（Quality Checker；与 Architect 不同 actor） |
| base/head | Plan Base `d2ae89b`；当前 head `1e5e7a0` |
| release_status | **NOT RELEASE**；不豁免 Release/merge；AC-VERIFIED 仅按 C+A 范围。 |

## 0. 结论

**READY（with recorded waiver）。** 人类 2026-11-27 waiver（`docs/review-tracking.md:458-470`）明确接受人工规程的四类残余，且**没有不可豁免的技术/安全阻断**：

1. **B-1 S6 group**：人类接受“S6 group proof 由 Fulfillment authorizer 按 `order_no+group_no` 签字/归档”（`:461-463`）；`shipments` 无 `group_no` 的缺口由该人工证据控制承接。**B-1 由 waiver 覆盖**，但 **runbook 必须把 group 签字段/归档格式写实**，否则控制未落地（TODO，不阻断 OPS-P2）。
2. **M1 无技术不可抵赖**：人类接受“no technical non-repudiation; named accounts/dual control/tickets/DBA audit compensate”（`:468-470`）；**不再要求技术不可抵赖**。
3. **M2 authorization_id/owner_approval_ref 自报**：人类接受“may be ticket-reported”（`:466-467`）；**Reviewer 核对原件/来源**仍须写入 runbook。
4. **M3 abandon 无代码 guard、用户再 Refund 可重开**：人类接受“no code guard; ops ticket/named-account procedure must intercept”（`:464-465`）；**不豁免“自动重试/自动关闭”**，只接受用户显式新请求由工单拦截。
5. **M4 AC-27A 边界**：waiver Scope 明确为 **`AC-27-NOWB-01` / `AC-27-LIVE-01` / AC-27**，**不含 AC-27A**（`:459-460,471`）。因此 **AC-27A 不得 AC-VERIFIED**；AC-61/62 也不得借 waiver 全量通过。**M4 由“waiver 范围收窄”关闭，不是 AC-27A 被豁免**。

结论：**0 BLOCKER**；残余风险已由人类书面接受。可进入 OPS-P3 人类批准 runbook/Plan；**AC-27 AC-VERIFIED 仍须 D1–D11 演练 + Reviewer**，且仅 AC-27 范围，不包含 AC-27A/AC-61/62 全量/Release/merge。

## 1. 上轮 B-1/4 MAJOR 关闭表

| 上轮项 | R2 判定 | 依据 |
|---|---|---|
| **B-1** S6 缺 group | **CLOSED by waiver + required control** | `review-tracking.md:461-463`：Fulfillment authorizer `order_no+group_no` 签字/归档；**runbook 必须落**（TODO T-1）。 |
| **M1** 无技术不可抵赖未接受 | **CLOSED** | `:468-470` 明确接受；以命名账号/双人/工单/DBA 审计/Reviewer 补偿；不得声称技术闭环。 |
| **M2** authorization_id/owner 自报 | **CLOSED** | `:466-467` 明确接受工单自报；Reviewer 核对原件/来源写入 runbook（TODO T-2）。 |
| **M3** abandon 无代码 guard | **CLOSED** | `:464-465` 明确接受；ops 工单/命名账号拦截用户再 Refund；**不得**自动重试/自动关闭。 |
| **M4** AC-27/AC-27A 边界 | **CLOSED（范围收窄）** | `:459-460,471`：scope 仅 AC-27-NOWB/LIVE；**AC-27A 不在 waiver**，不得 AC-VERIFIED；AC-61/62 不全量。 |
| **无不可豁免技术/安全阻断** | **PASS** | 人工规程无代码/迁移/RPC；残余为授权自报/无技术不可抵赖/无代码 guard，均已人类书面接受；S6 group 有 authorizer 控制。 |

## 2. Runbook 必须写明的残余/人工补偿（Developer 落 `docs/ops.md`）

1. **S6 group（B-1）**：`evidence_ref` 必须含 `order_no+group_no`；Fulfillment authorizer 在变更单/归档中按 `order_no+group_no` 签字；D5b/D10b 增加“A 组证据用于 B 组 → 拒绝、零写”；不得仅凭 `shipments.order_no` re-arm。
2. **用户再 Refund（M3）**：写明“abandon 后无代码 guard；用户显式再次 Refund 是**新请求**，必须重新走 §5/§6 评审；ops 以命名账号/工单拦截，禁止自动重试/自动关闭”；D7/D11 expected 必须覆盖。
3. **authorization_id/owner 自报（M2）**：Reviewer 必须核对授权单原件/编号来源和 owner 书面口径；D8 增加“自报编号/同人伪造/伪造 owner”用例；工单记录“仅事后审计，不是技术校验”。
4. **无技术不可抵赖（M1）**：runbook 首段必须有人类签署句：“本规程无密码学不可抵赖；以命名账号、双人、变更单、DBA 审计、Reviewer 补偿；不得声称 AC-62 技术闭环/工具已实现。”
5. **AC-27A/AC-61/62 范围（M4）**：runbook 明示 waiver 只覆盖 AC-27-NOWB/LIVE；AC-27A 仍 AC-REF，AC-61/62 不得全量 AC-VERIFIED；Reviewer 只能按 C+A 标 AC-27。
6. **不变式**：abandon 只 `FAILED_PERMANENT`，admission REVIEW、commitment RESERVED、permit 原值、组不关闭、0 Payment/0 release、canonical `last_error` 保留；re-arm 先审计后 CAS；task 无 `version`，用 status+lease+attempts 行锁。
7. **D1–D11**：补具体 SQL/`nats` 命令、前后快照、expected；D5b/D10b（错组）、D7/D11（再 Refund）、D8（伪造授权/owner）、D9（并发/崩溃）必须可判定。

## 3. TODO（不阻断 OPS-P2；OPS-P4 前完成）

- **T-1** S6 group/D5b/D10b + authorizer 签字/归档格式。
- **T-2** authorization 原件核对/D8 伪造 + runbook 人类残余签署句。
- **T-3** D1–D11 具体命令/快照/expected（含 task 无 `version` 的 CAS 写法）。

## 4. 结论

**READY（with recorded waiver；0 BLOCKER）。** 上轮 B-1/4 MAJOR 均由人类 2026-11-27 waiver 覆盖或按 Scope 收窄（AC-27A 不在）；无不可豁免技术/安全阻断。**可进入 OPS-P3**；**AC-27 AC-VERIFIED** 只在 D1–D11+Reviewer 后、且仅 AC-27 范围；**不豁免 Release/merge/AC-27A/AC-61/62 全量**。Developer 必须按 §2 落 `docs/ops.md`，否则 OPS-P4 退回。

**只读声明**：未执行代码/测试/生产操作；只审 manual/Plan/waiver/SPEC 引用；未改设计/代码/SPEC/ADR；只新建本报告；未 commit。

**Actor**：`qc-ac50`
**Status**：**READY（with recorded waiver；0 BLOCKER；3 TODO）**
