# ADR-0007 Amendment 1 Addendum — PAY-OPS-REPLAY-01 人工运维规程（DLQ/租约重放 + NOWB 人工处置）

| 项 | 值 |
|---|---|
| Status | **DRAFT 0.1 — pending QC / human（manual route）**（未批准；不构成实现依据；不代表 Release PASS） |
| Change ID | **`PAY-OPS-REPLAY-01`**（Parent `PAY-SETTLE-01` 暂停；本规程交付后 handback） |
| Parent | `PAY-SETTLE-01`（暂停；本 L2 不推进其受管代码/台账） |
| Tier | **L2**：P0、资金/退款、跨服务恢复、人工运维；不适用 L1 |
| Base | `160a45d`（Plan）；契约基线 SPEC-0001 v1.5 §16.2、SPEC-0002 v1.11 AC-61/62、ADR-0007 AC-50 0.6 ACCEPTED |
| Human decision | **2026-11-27 C+A**：撤回 `cmd/ops`/`0024`/新 RPC 工具路线；改为**人工运维规程 + disposable MySQL 8.4 演练 + Reviewer**；允许在演练与 Reviewer 证据齐备后标 **AC-27 AC-VERIFIED**（waiver，expiry **2026-12-31**）。本 ADR 只定义规程约束；runbook 正文由 Developer 落 `docs/ops.md` |
| Actor | `arch-ops-replay-manual`（System Architect） |
| Reviewed-by | 待 QC（manual route）；本文件不代签 |
| 硬边界 | **不新增/不修改代码、RPC、proto、表、迁移、worker、feature flag**；不写 `docs/ops.md` 正文；不改 SPEC/AC-50/69/70 语义；不 release/不恢复 permit/不关闭组（abandon）；不 commit |
| 取代 | `0007-amd1-addendum-ops-replay.md` / `0007-amd1-ops-replay-impact.md`（**WITHDRAWN**，仅历史） |

## 0. 决策与范围

- **人工规程**：由 ops/DBA 在**变更单 + 双人授权**下，使用 MySQL 8.4 客户端与 `nats` CLI 执行受控 SQL/NATS 操作；不构造运维二进制/管理 RPC/授权表。
- **不写 runbook 正文**：本 ADR 只定义入口、前置、CAS 条件、执行顺序、审计、禁止项、失败/回滚与演练期望；Developer 据此写 `docs/ops.md` 的具体命令与查询。
- **覆盖**：① `aftersale.completed` DLQ / durable task 租约丢失的人工重放（AC-61/AC-27-LIVE）；② NOWB 人工 re-arm/abandon（AC-27-NOWB/AC-62）。
- **不覆盖**：自动 re-arm、自动 release、D-24 人工释放实现、BFF/UI、TCC 业务规则变更、批量数据修复。
- **工具路线明确关闭**：不得在实现中夹带 `cmd/ops`、`ops_action_authorizations`、`0024` 或新 admin RPC；QC B-1..B-5 的工具缺口由本人工规程 + 补偿控制承接，并由 Reviewer 在 waiver 中记录。

## 1. 角色、授权与不可抵赖（人工控制）

| 角色 | 职责 | 证据 |
|---|---|---|
| **operator** | 执行 SQL/NATS 的 DBA/ops；使用个人命名账号，禁用共享账号 | 变更单/工单、DB 会话审计、命令日志 |
| **authorizer** | 业务授权人（admin 角色）；与 operator 不同人；确认金额/证据/处置 | 变更单签字、admin 账号与角色快照 |
| **reviewer** | 金额 ≥1000 元（100000 cent）的第二复核人；与 authorizer 不同人 | 变更单复核栏、Reviewer 记录 |
| **Fulfillment authorizer** | 对 S6 证据真实性签字；admin 角色 | 变更单 + Fulfillment 证据引用 |
| **Reviewer（独立）** | 演练/生产执行后核验 SQL 输出、DB 快照、无 release/无重复退款；更新 AC 台账 | 评审报告 + 证据路径 |

**授权单最小字段（AC-62 D-24；人工工单）**：`authorization_id`（授权单号，**不得用 `ticket=RVW` 代替**）、`action`（REPLAY/NOWB_REARM/NOWB_ABANDON）、`refund_id/after_sale_no/order_no/group_no/batch_id`、`amount_cent`、`operator/authorizer/reviewer`（姓名+admin user id）、`reason_code`、`evidence_ref`（S6）、`authorized_at`、`expires_at`（≤24h）、`ticket=RVW:<refund_id>`、`owner_approval_ref`（abandon）。任一缺失/过期/越权/≥1000 无 reviewer → **拒绝执行，零变更**。

**限制（必须由 Reviewer 记录）**：人工规程没有 JWT/工具的密码学校验；补偿控制是命名账号、双人、变更单、DBA 审计日志、Reviewer 独立核验与 AC-27 waiver。不得声称“技术不可抵赖”或“工具已实现”。

## 2. DLQ / 租约定位（AC-61 / AC-27-LIVE-01）

- **Stream/Subject**：MALL stream（`shop.>`，7 天）；DLQ `shop.aftersale.completed.v1.dlq`；payload wrapper `{subject,data,error,attempts,failed_at}`，`data` 为原事件（`internal/platform/consumer/consumer.go:157-170`；`mq.go:49-51`）。
- **定位键**（至少一个；不唯一则停止）：`after_sale_no`、`stream_seq`、`refund_id`（经 `order_refund_admissions`/`refund_applications` 映射到 order/group/after_sale）、`batch_id`/`payment_no`（经 `refund_batches` 映射）。
- **只读核对**：先记录 `stream_seq/subject/failed_at/attempts/error`、`order_refund_admissions.result/state/version`、`after_sale_refund_tasks.status/attempts/lease_*`、`after_sale_logs` 最新 action；**不修改** DLQ 消息。
- **窗口**：DLQ 只在 MALL stream 保留 7 天；超期/不存在 → 停止，检查 outbox 归档/工单，不伪造、不从记忆重建。

## 3. `aftersale.completed` 人工重放（AC-61）

**只重驱动未完成投影，不重做退款**；`FAILED_PERMANENT` 一律拒绝。事件重放针对已 `DONE` task 的 Order 投影；task 重试只针对 `PENDING/PROCESSING`（§4）；已完成 task 的退款不得重做。

1. 前置：task `DONE`（事件在退款完成后才产生）；Order 侧 `order_refund_admissions` 仍 `PENDING_REMOTE` 或投影缺失；REVIEW 不得用事件 replay 替代 NOWB；task `PENDING/PROCESSING/FAILED_PERMANENT` → 不 replay（FAILED 拒绝；PROCESSING 租约丢失见 §4）。
2. 从 DLQ 取**原始 `data` 字节**（不改写），记录 `stream_seq`；在变更单登记 `op_id`（人工编号）。
3. 用 `nats` CLI 原样发布到 `shop.aftersale.completed.v1`；header `Nats-Msg-Id=manual:<authorization_id>:<stream_seq>`；**不发布到 `.dlq`**。
4. 观察 Order consumer/投影；同一事件重复发布由 `applyAfterSaleCompleted` 幂等（`aftersale_consumer.go:39`）吸收，不得重复退款/取消/占位。
5. 审计：变更单 + `after_sale_logs`（如可写）或结构化 ops 日志；记录 `op_id/authorization_id/stream_seq/subject/ticket`；Reviewer 核对投影前后。
6. **失败**：DLQ 无消息/超 7 天 → 停止（不猜）；发布失败 → 重试同 `op_id`，不重复改 DB；投影仍不收敛 → 转 NOWB/告警，不写 PRE/关闭/打款。
7. **并发/崩溃**：仅一个变更单执行；重复发布安全；崩溃后同 `op_id` 重试，审计 at-least-once。

## 4. 租约丢失 / 未完成 task（AC-61；不重放 completed）

- task `PENDING` 或 `PROCESSING` 且 `lease_until` 过期/空：worker 会在下一 tick 自动重试（`workers.go:300-326`）；人工动作**优先是重启/确认 worker**，不修改 task。
- 若 worker 不可用且必须立即重试：在变更单 + 双人授权下，对 task 行 `FOR UPDATE`，确认 `status IN ('PENDING','PROCESSING')` 且 lease 空闲/过期，`UPDATE ... SET status='PENDING', attempts=0, lease_owner='', lease_until=NULL`；**不触碰 admission/金额**；保留 `last_error` 供审计（worker 成功会清）。
- `FAILED_PERMANENT`：**拒绝同 task 重放**；自动永久路径已释放 pending/额度（`workers.go:858-888`），同 task 重放可能绕过释放；按 AC-62/新 `refund_id` 人工处置，记 `PERMANENT_REQUIRES_NEW_REFUND`。
- REVIEW task：**不走本节**；只走 §5/§6。

## 5. NOWB re-arm（AC-27-NOWB-01 终态①）

### 5.1 S6 证据核验（“曾存在但丢失”窗口）

- **可接受证据**（Fulfillment-owned，只读核验；Order 不写运单）：
  1. `shipments` 行 `order_no` 匹配且 `status='CANCELLED'`（`migrations/fulfillment/0001_init.sql:1-16`）；
  2. `shipment_events` 行 `status='CANCELLED'`，且其 `shipment_no` 对应 `shipments.order_no` 匹配（`0002_shipment_events.sql:1-10`）；
  3. Fulfillment outbox/归档中的真实 `shipment.cancelled` 事件，或承运商取消回执（含单号/order/时间），由 Fulfillment authorizer 在工单中签字。
- **窗口**：证据必须来自原订单/运单时间线（取消事件早于或等于本次复核；归档事件须可追溯原始时间）；**“曾存在但丢失”**只接受上述归档/事件/回执；仅有口述、聊天、临时新建或 Order 侧补写 → **拒绝**（不得伪造）。
- 无任何可核验记录 → 停止 re-arm，转 §6 abandon；**不得**为“曾存在”补造 S6 行/事件。
- 证据 `evidence_ref` 写运单号/事件 id/归档 id；工单记录 Fulfillment authorizer、核对 SQL 输出与时间。

### 5.2 三版本/三条件 CAS 与顺序（先审计、后清除）

1. **先写审计**：变更单 + ops 结构化日志 `review_rearm`（authorization_id/op_id/ticket/refund/order/group/version/evidence/operator/authorizer/reviewer）→ 再执行。`after_sale_logs` 在 AfterSales 事务内补写（有 task 时）。
2. **Order DB（一个事务）**：
   - `SELECT result,state,version,... FROM order_refund_admissions WHERE refund_id=? FOR UPDATE`；必须 `REVIEW_REQUIRED/PENDING_REMOTE`。
   - `UPDATE ... SET result='PENDING_REMOTE', version=version+1 WHERE refund_id=? AND result='REVIEW_REQUIRED' AND state='PENDING_REMOTE' AND version=?`；RowsAffected=1，否则回滚。
   - 有 `batch_id`：`SELECT state,version,last_error FROM refund_batches WHERE batch_id=? FOR UPDATE`；必须 `MANUAL_HOLD`；`UPDATE ... SET state='ADMITTING', version=version+1 WHERE batch_id=? AND state='MANUAL_HOLD' AND version=?`；RowsAffected=1；**保留 `last_error` canonical**。
   - `refund_applications.admission_result` 保持 REVIEW（等待重评）；不新建行。
3. **AfterSales DB（有 task 时，一个事务）**：
   - `SELECT status,attempts,lease_owner,lease_until,last_error FROM after_sale_refund_tasks WHERE after_sale_no=? FOR UPDATE`；要求 `PENDING/PROCESSING` 且 lease 空闲/过期；`FAILED_PERMANENT` 拒绝。
   - `UPDATE ... SET status='PENDING', attempts=0, lease_owner='', lease_until=NULL`；**`last_error` 保持 canonical 不变**；`INSERT after_sale_logs('OPS_NOWB_REARM', remark≤512)`。
   - **task 没有 version 列**（`0010:88-101`），第三维是 `status+lease_owner+lease_until+attempts` 的行锁 CAS；不得伪造 `version`。
4. **重驱动**：有 task → worker 自动（5s tick）；无 task 有 batch → 由既有 TCC 补偿入口重试（`TccCancelMarkPaid`/DTM 重试，非新 RPC）；无 task/app → 拒绝 `NO_REPLAY_DRIVER`，不猜 source（`order_refund_admissions` 无 source，`0016:7-18`）。
5. **崩溃**：Order 已提交、task 未复位 → 重跑，Order 已 PENDING 则只做 task；task 已复位、worker 未跑 → 等待/重启；TCC ADMITTING 未重驱动 → 重跑既有 TCC 入口；不重复 CAS。
6. 仍无运单 → `AdmitRefund` 返回 REVIEW，task/batch 回 canonical/MANUAL_HOLD；**不写 PRE/不打款**。

## 6. NOWB abandon（AC-27-NOWB 终态②）

**定义**：无不可补齐 S6 证据时停止自动重试/留下人工结论；**不是** AC-62 release；不释放额度、不恢复 permit、不关闭组、不写 `DONE`、0 Payment。

- **普通售后（有 task）**：AfterSales 事务 `FOR UPDATE`；要求 `PENDING/PROCESSING` 且 lease 空闲/过期；`UPDATE ... SET status='FAILED_PERMANENT', lease_owner='', lease_until=NULL`；**`last_error` 保持 canonical `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>` 不变**；`INSERT after_sale_logs('OPS_NOWB_ABANDON', remark=authorization/op/ticket/owner_approval/evidence)`；**不调用**自动永久路径（`workers.go:858-888` 会释放 pending/额度），不改 `after_sale_amount_ledger`/`amount_state`。
- **TCC（有 `refund_applications`）**：Order 事务 `FOR UPDATE`；`UPDATE refund_applications SET state='FAILED_PERMANENT', version=version+1 WHERE refund_id=? AND state='PENDING' AND version=?`；batch 保持 `MANUAL_HOLD`、`last_error` 保留；admission 保持 `REVIEW_REQUIRED/PENDING_REMOTE`；commitment `RESERVED`；permit 原值；组不关闭。普通 TCC retry 必须被 `ensureTccApplicationTx` 的 `FAILED_PERMANENT` 拒绝（`TCC_APP_ABANDONED`）。
- **无 task/app 的 legacy**：拒绝 `NO_TERMINAL_CARRIER`；不新建行/不新建 `refund_applications`。
- **审计**：变更单 + `owner_approval_ref`（SPEC-0002 owner 对 AC-62/D-24 的书面口径）+ `after_sale_logs`/Order 行 version+时间戳 + 结构化日志 + 工单；`authorization_id` 与 `ticket` 分开。
- **可见性**：admission 仍 REVIEW，AC-72 backlog 计入是正确的（负债未清）；abandon 不是已退款/已关闭/已释放。
- **限制**：task `FAILED_PERMANENT` 后用户显式再次 `Refund` 可能触发 `ensureRefundTaskTx` 重开 task（代码现状 `workers.go:915-929`）；这是**新的显式请求**，不是自动重试；ops 不得自动重试，且必须在工单记录“如用户重开须重新走 §5/§6”。不得声称 abandon 在无代码 guard 下技术不可逆。

## 7. 授权 / 双人 / 金额 / 有效期（AC-62 D-24 对应）

- **对应关系**：AC-62 的“人工放弃/FAILED_PERMANENT + 人工授权”在本规程对应 **变更单 authorization_id + operator/authorizer/reviewer 三签 + owner_approval_ref**；`ticket=RVW` 只是复核 id，不是授权单。
- 金额 ≥1000 元（100000 cent）必须有第二复核人（reviewer ≠ authorizer）；授权有效期 ≤24h；过期/越权/字段缺失 → 拒绝。
- **release 不在本规程**：本规程的 abandon 只停止重试/保留负债；`RESERVED→RELEASED` 是独立的 AC-62/D-24 授权动作，不得在 abandon 中顺带执行；不得用本规程的授权单替代 D-24 release 授权。
- DBA 用个人命名账号执行；`operator/authorizer/reviewer` 的 admin 用户与角色由 Reviewer 在 identity 只读快照核对；禁止共享账号/自助脚本。

## 8. 失败 / 回滚 / 演练（disposable MySQL 8.4 + NATS）

| # | 演练 | 期望 |
|---|---|---|
| D1 | DLQ 正常 replay | 投影收敛；无重复退款/取消/占位；审计含 stream_seq |
| D2 | DLQ 缺失/超 7 天 | 停止；不发布、不猜；工单记录 |
| D3 | task 租约丢失 | worker 自动恢复；手动仅重启/确认；不改 admission |
| D4 | `FAILED_PERMANENT` task | 拒绝同 task 重放；新 refund_id/AC-62 |
| D5 | re-arm 真实 CANCELLED | 三 CAS 成功；worker/TCC 重驱；PRE；审计完整；`last_error` canonical 保留 |
| D6 | re-arm SHIPPED/仍无运单 | POST 或再次 REVIEW→MANUAL_HOLD；0 Payment |
| D7 | abandon | task/app FAILED；admission REVIEW；commitment RESERVED；permit 原值；组未关闭；0 release/0 Payment |
| D8 | 未授权/缺复核/≥1000 无 reviewer/过期 | 拒绝；零变更；审计拒绝原因 |
| D9 | 并发/崩溃 | 仅一个 CAS 成功；重跑只补未完成步骤；无部分 release/重复退款 |
| D10 | S6 伪造/无记录 | 拒绝 re-arm；转 abandon；不新建 S6 行 |
| D11 | TCC abandon 后 retry | `TCC_APP_ABANDONED`；app 仍 FAILED；0 Payment |

- 演练必须在 **disposable MySQL 8.4**（真实 schema/0003/0016/0017/0010，NATS 测试 stream）执行；Reviewer 记录 SQL 输出/前后快照；不得用 fixture 冒充生产证据。
- **生产失败**：停止，保留现状/告警；不回滚已提交的 CAS（不自动反向改状态）；按工单/新 `refund_id` 前向处置；禁止清空 `last_error`/删除 admission 作为回滚。
- **waiver**：人类 2026-11-27 C+A 允许在 D1–D11 演练 + Reviewer 证据齐备后把 **AC-27** 标 AC-VERIFIED（expiry 2026-12-31）；不包含 AC-61/62 全量、Release/merge；AC-61/62 仍按 Reviewer 证据范围登记。

## 9. 禁止项

- 禁止 `cmd/ops`、`0024`、授权表、新 admin RPC、新迁移、新 worker/脚本自动化。
- 禁止自动 re-arm/自动 release/自动关闭；禁止把 abandon 当 release；禁止恢复 permit、写 PRE/POST 或重复退款。
- 禁止伪造 S6 证据/新建运单行/用聊天或口述当证据；禁止用 `ticket=RVW` 代替授权单。
- 禁止共享 DBA 账号、无工单执行、无 Reviewer 证据标 AC-VERIFIED。

## 10. QC B/M 对应（manual route）

| QC 项 | 人工规程处置 |
|---|---|
| B-1 授权 | §1/§7：变更单 authorization_id + 三个 admin 角色/不同人 + ≥1000 reviewer + 有效期 ≤24h；无工具但以命名账号/工单/DBA 审计/Reviewer 补偿；不声称密码学不可抵赖 |
| B-2 S6 | §5.1：仅 Fulfillment `shipments`/`shipment_events`/outbox/承运商回执；无记录拒绝；不伪造 |
| B-3 canonical | §6：`last_error` 保持 canonical；abandon 审计在 `after_sale_logs`/工单/owner_approval；TCC 用 admission/batch 版本 + 日志 |
| B-4 replay-only | 人工路线无 CLI/flag；TCC re-arm 后由既有 `TccCancelMarkPaid`/DTM 重试驱动（§5.2 步骤 4）；无 driver → 停止，不新建入口 |
| B-5 ACL | NATS：ops/DBA 仅 publish `shop.aftersale.completed.v1`、subscribe `.dlq`、最小 JS API；DB：Order/AfterSales RW、Fulfillment RO；禁止 `.dlq` publish/其他 `shop.>` |
| M-1 四标识 | §2：after_sale_no/stream_seq/refund_id/batch_id/payment_no；不唯一/缺失停止 |
| M-2 TCC abandon | §6 + D11：app FAILED；普通 TCC retry 拒绝（`TCC_APP_ABANDONED`）；0 Payment |
| M-3 op_id 唯一 | 变更单 `op_id` 唯一；Reviewer 以工单/日志核对；无 DB 唯一表（工具路线已撤回） |
| M-4 app/admission guard | §5.2/§6：re-arm 先查 app FAILED/admission REVIEW；abandon 后不得自动复位；用户显式 Refund 重开须重新评审（限制已声明） |
| T-1 remark 截断 | 有 task 时 `after_sale_logs.remark` 优先 op/authorization/ticket/reason，截断 evidence；完整证据在工单 |
| T-2 inspect/ACL | 人工只读查询由 Reviewer 记录；不输出 token/secret；权限同 B-5 |

## 11. 假设与不确定性

- **[假设]** ops/DBA 具备 disposable MySQL 8.4 + NATS 测试环境与生产只读/受控写权限；生产变更单/双人流程可用。
- **[假设]** Fulfillment `shipments`/`shipment_events` 或归档保留可核验 CANCELLED 证据；无记录时按 abandon，不猜。
- **[不确定]** 用户显式再次 `Refund` 会重开 task（现状代码），本规程不修改代码；工单必须记录并重新评审。
- **[不确定]** AC-62 D-24 release 独立流程未实现；本规程不声称 release；若需 release，另立变更/人类批准。
- **[风险]** 人工执行依赖 DBA 纪律；以双人/命名账号/工单/Reviewer/演练补偿；任何越权即停止并回 Plan。

**Actor**：`arch-ops-replay-manual`（System Architect）
**Status**：**DRAFT 0.1 — pending QC / human（manual route；AC-27 waiver expiry 2026-12-31）**
