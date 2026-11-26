# QC-PAY-OPS-REPLAY-01-MANUAL — 人工规程红队（OPS-P2）

| 项 | 值 |
|---|---|
| change_id | `PAY-OPS-REPLAY-01`（Parent `PAY-SETTLE-01` 暂停；工具路线 WITHDRAWN） |
| 审查对象 | `docs/adr/0007-amd1-addendum-ops-manual.md` **DRAFT 0.1**（blob `4e8c218`，162 行）；`docs/lead/PAY-OPS-REPLAY-01/plan.md` / `tasks.md`；SPEC-0001 §16.2、SPEC-0002 AC-61/62、`docs/ops.md` |
| 结论 | **BLOCKED**（1 个 `[BLOCKER]` + 4 个 `[MAJOR]` + 2 个 `[TODO]`） |
| 日期 | **2026-11-27** |
| Actor | `qc-ac50`（Quality Checker；与 Architect 不同 actor） |
| base/head | Plan Base `d2ae89b`；当前 head `0809647` |
| release_status | **NOT RELEASE**；不批准生产执行/AC-VERIFIED；manual DRAFT。 |

## 0. 结论

**BLOCKED。** 人工规程总体成立：**工具路线已 WITHDRAWN**（`§0`、Plan/Tasks），无代码/迁移/RPC；DLQ/租约重放只驱动未完成 task，`FAILED_PERMANENT` 拒绝，重复/并发/崩溃有约束（`§3-4`）；re-arm 有 S6 证据/双人/三 CAS/先审计后清（`§5`）；abandon 只置 `FAILED_PERMANENT`，admission 保持 REVIEW、commitment RESERVED、permit 原值、组不关闭、0 Payment/0 release，canonical `last_error` 保留（`§6`）；授权为 `authorization_id` + operator/authorizer/reviewer 三名不同 admin + ≥1000 reviewer + ≤24h + `owner_approval_ref`（`§1,§7`）；D1–D11 覆盖主路径（`§8`）；S6 “曾存在但丢失”只认归档/事件/回执，禁止补造（`§5.1`）。**没有把 abandon 做成 release**；AC-62/INV-45 的 no-release 方向正确。

但仍有 **1 个 BLOCKER**：

- **S6 证据仍缺 group 维度**：`§5.1` 只要求 `shipments.order_no` 匹配 + `status='CANCELLED'`，或 `shipment_events.status='CANCELLED'` 且 shipment/order 匹配。但 `shipments` 只有 `order_no`、无 `group_no`（`migrations/fulfillment/0001_init.sql:1-16`），`shipment_events` 只有 `shipment_no`（`0002_shipment_events.sql:1-9`）。同一订单多组时，A 组的 CANCELLED 运单可用于 B 组的 re-arm；D5/D10 不会发现。必须补“Fulfillment authorizer 在工单中按 `order_no+group_no` 签字/归档证据含 group，或 `shipment_items`/组取消回执能证明 group”；否则 S6 证据不足，re-arm 可能错组，AC-27A 关闭谓词被污染。

另 **4 个 MAJOR**：

1. **无技术不可抵赖的残余未被人类显式接受**：`§1` 承认“人工规程没有 JWT/工具的密码学校验”，补偿控制是命名账号/双人/变更单/DBA 审计/Reviewer；但 **runbook 没有一行“人类接受该残余、不得声称技术不可抵赖/AC-62 全量”**。Human C+A 豁免了工具，但需在 `docs/ops.md` 由人类签署该残余；否则 Reviewer 不能把“人工规程”当 AC-62 的充分控制。
2. **`authorization_id`/`owner_approval_ref` 仍是工单自报**：`§1,§7` 只要求字段，没有外部授权系统/预登记；DBA/operator 可填写任意编号。人工路线可以接受，但必须明确“Reviewer 核对授权单原件/编号来源”，并把“仅工单自报可被同人伪造”列为残余；否则 D8 只测字段缺失，不测伪造。
3. **abandon 无代码 guard，用户显式再次 `Refund` 可重开 task**：`§6` 已诚实声明 `ensureRefundTaskTx` 现状 `workers.go:915-929` 会重置，且 `§11` 列为不确定；但 D7/D11 没有“用户再次 Refund → 重开/重新评审”的演练，Reviewer 无法判断 abandon 的实际可逆性。必须把“仅人工不再自动重试；用户新请求=新评审”写进 D7/D11 expected，并由人类接受“非技术不可逆”。
4. **AC-27 waiver 与 AC-27A 的口径不清**：`§8` 写“AC-27 可 AC-VERIFIED”，Plan 写“AC-27/27A”，D5 只覆盖 re-arm→PRE，未显式断言 AC-27A 关闭谓词（`CANCELLED`/permit=REVOKED/`CONSUMED+PRE`）。必须明确 waiver 是否覆盖 AC-27A；若不覆盖，AC-27A 仍需 Reviewer 按 AC-27A 证据单独判定。

**2 个 TODO**：D1–D11 的具体 SQL/`nats` 命令、前后快照与 expected 需 Developer 在 `docs/ops.md`/drill 脚本中落全；`§5.2` 的 task 无 `version`、用 status+lease+attempts 行锁 CAS，需在 runbook 用“SELECT ... FOR UPDATE + RowsAffected”写清，避免 DBA 误用 `version`。

## 1. 关闭表

| 检查 | 判定 | 依据 |
|---|---|---|
| 工具路线 WITHDRAWN/no code/migration/RPC | **PASS** | `§0`、`§9`、Plan/Tasks；允许 `docs/ops.md`/drill 脚本/selfcheck。 |
| DLQ/租约定位与重放（仅未完成 task、FAILED 拒绝） | **PASS** | `§2`、`§3`、`§4`；DLQ 7 天/MALL/stream_seq；task `PENDING/PROCESSING` lease；`FAILED_PERMANENT`/REVIEW 拒绝。 |
| 幂等/重复/并发/崩溃 | **PASS（文本）** | `§3.4/§3.7` 原样 data、`Nats-Msg-Id`、同 `op_id`；`§8 D1/D9`。 |
| re-arm S6 证据 | **FAIL（group）** | `§5.1` order 匹配，无 group；`shipments` 无 group；D5/D10 不覆盖错组。 |
| re-arm 双人/CAS/先审计后清 | **PASS** | `§5.2`；admission version、batch version、task status/lease/attempts；`§6` audit first。 |
| abandon 只 FAILED/不 release/permit/组/0 Payment/canonical | **PASS** | `§6`；task/app FAILED；admission REVIEW；commitment RESERVED；`last_error` canonical；no release。 |
| 授权 `authorization_id`/三 admin/≥1000/≤24h/owner | **PASS（字段）/PARTIAL（伪造）** | `§1,§7`；字段/双人/金额/有效期；**authorization_id/owner 自报**（MAJOR）。 |
| S6 曾存在但丢失只认归档/事件/回执，禁止补造 | **PASS** | `§5.1`；`order_no` group 缺口见 BLOCKER。 |
| D1–D11 可执行/可判定 | **PARTIAL** | `§8` 场景/期望清楚；缺 SQL/命令/快照细节（TODO）；D5/D10 缺 group；D7/D11 缺 second Refund。 |
| 无技术不可抵赖 + 人类接受 | **PARTIAL** | `§1` 声明限制；**runbook 未显式人类签署残余**（MAJOR）。 |
| AC-27 waiver 条件 | **PARTIAL** | `§8` 写 AC-27；AC-27A/AC-61/62 边界未逐项（MAJOR）。 |

## 2. 关键 `[BLOCKER]`

### B-1 S6 证据缺少 `group_no`，可跨组错误 re-arm

- `§5.1` 可接受证据：`shipments.order_no` + `CANCELLED`；`shipment_events.status=CANCELLED` + shipment/order 匹配；Fulfillment outbox/归档 `shipment.cancelled`/承运商回执。
- 事实：`shipments` 主键/唯一键是 `shipment_no`、`uk_shipments_order(order_no)`，**无 `group_no`**（`migrations/fulfillment/0001_init.sql:1-16`）；`shipment_events` 只有 `shipment_no/status`（`0002_shipment_events.sql:1-9`）。多组订单的 A 组 CANCELLED 运单可以满足 B 组 re-arm 的 order 匹配。
- `AC-27A` 是**组级**关闭谓词；错组 PRE 会错误关闭 B 组。D5 只写“真实 CANCELLED”，D10 只写“S6 伪造/无记录”，没有“A 组证据用于 B 组”的否定用例。
- **修复**：`§5.1` 增加 group 校验：Fulfillment authorizer 在工单按 `order_no+group_no` 签字，或证据归档/`shipment_items`/组取消回执能证明 group；D5b/D10b 增加错组用例（A 证据→B group 必须 exit 2/4、零写）。

## 3. `[MAJOR]`

1. **人类接受残余未落 runbook**：`§1` 只说“必须由 Reviewer 记录”，未写“人类在 `docs/ops.md` 签署接受：无密码学不可抵赖、依赖 DBA 审计/工单/Reviewer；不得声称 AC-62 技术闭环”。Human C+A 是工具豁免，不自动等于接受该残余；需 OPS-P3 明确。
2. **`authorization_id`/`owner_approval_ref` 可自报**：`§1,§7` 无外部授权核验；DBA/operator 可填任意编号。人工路线可接受，但必须要求 Reviewer 核对授权单原件/编号来源，并把“同人可伪造、仅事后审计”列为残余；D8 增加伪造/回填用例。
3. **abandon 可被用户显式 Refund 重开**：`§6/§11` 已声明 `ensureRefundTaskTx` 现状会重置，但 D7/D11 未测“用户再次 Refund → 重新评审/不自动重试”。需在 D7/D11 写 expected 并由人类接受“非技术不可逆”。
4. **AC-27 waiver 与 AC-27A 不清**：`§8` 只写 AC-27；Plan 写 AC-27/27A；D5 只到 PRE，未断言 AC-27A 关闭谓词。明确 waiver 是否覆盖 AC-27A；若不覆盖，AC-27A 需独立证据/Reviewer，AC-61/62 也不得借 waiver 全量通过。

## 4. `[TODO]`

- **T-1** D1–D11 在 `docs/ops.md`/drill 脚本中补具体 MySQL `FOR UPDATE`/CAS SQL、`nats` CLI、前后快照字段与 expected；D5/D10 补 group 错组用例。
- **T-2** `§5.2` 的 task 无 `version`，用 `status+lease_owner+lease_until+attempts` 行锁 CAS；runbook 写清不得 `UPDATE ... WHERE version`，DBA 误用会 0 行/锁超时。

## 5. 结论

**BLOCKED（1 BLOCKER）。** 人工规程的 no-release/abandon、DLQ/lease、幂等、双人授权、canonical 保留、D1–D11 方向正确；**S6 group 证据**是硬缺口，**无技术不可抵赖**需人类在 runbook 中显式接受。修复 B-1 与 4 MAJOR、补齐 TODO 后重提 OPS-P2；**当前不得 OPS-P3/生产执行/AC-27 AC-VERIFIED**。

**只读声明**：未执行代码/测试/生产操作；只审 manual/Plan/SPEC 引用；未改设计/代码/SPEC/ADR；只新建本报告；未 commit。

**Actor**：`qc-ac50`
**Reviewed-by**：待 Architect 修订 / QC OPS-P2 复审
**Status**：**BLOCKED — 1 BLOCKER**
