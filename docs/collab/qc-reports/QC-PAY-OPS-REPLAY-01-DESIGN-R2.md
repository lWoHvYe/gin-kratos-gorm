# QC-PAY-OPS-REPLAY-01-DESIGN-R2 — OPS-C2 round 2

| 项 | 值 |
|---|---|
| change_id | `PAY-OPS-REPLAY-01`（Parent `PAY-SETTLE-01` 暂停） |
| 审查对象 | `docs/adr/0007-amd1-addendum-ops-replay.md` **DRAFT 0.2**（308 行，blob `04e6bcc`）；`docs/adr/0007-amd1-ops-replay-impact.md`（97 行，blob `cfd3c84`）；设计声明 `migrations/order/0024_ops_action_authorizations.sql`（DRAFT 无 migration 文件，符合“无代码/迁移文件”；但 §7.1 未给 DDL） |
| 结论 | **BLOCKED**（3 个 `[BLOCKER]` + 4 个 `[MAJOR]` + 2 个 `[TODO]`） |
| `[CONTRACT-IMPACT]` | **是，需人类 OPS-C3 + 迁移批准**；当前 DDL/迁移文件未交付，不能批准 |
| 日期 | **2026-11-27** |
| Actor | `qc-ac50`（Quality Checker；与 Architect 不同 actor） |
| base/head | 设计 Base `160a45d`；当前 head `bda51bb` |
| release_status | **NOT RELEASE**；不批准实现/merge/Release。 |

## 0. 结论

**BLOCKED。** 0.2 已实质修复上轮 B-1/B-4/B-5 的大部分：
- **B-1 身份**：使用现有 identity JWT（HS256、`admin`、operator/authorizer/reviewer 三个不同 `sub`、≥100000 cent 必须 reviewer）、外部 `authorization_id`/`authorization_expires_at`、`owner_approval_ref`、JWT `jti`；方向正确。
- **B-3 abandon**：保留 canonical `last_error`，用 `after_sale_logs('OPS_NOWB_ABANDON')`/授权表/`owner_approval_ref` 审计，`ensureRefundTaskTx` 以“最新 log action”拒绝复位；不再丢 reason/ticket。
- **B-4/B-5**：`--replay-only` 已进 CLI；NATS ACL（publish completed/read `.dlq`，禁 publish `.dlq`）与三 DSN 分权（Fulfillment 只读）已写明；四标识选择器、TCC abandon guard、`op_id` 唯一、app/admission/task 三重 guard、remark 截断、inspect 均有设计。

但仍有 **3 个 BLOCKER**：

1. **B-1（DDL 未交付，`[CONTRACT-IMPACT]` 不能批准）**：设计声明 `migrations/order/0024_ops_action_authorizations.sql`；DRAFT 无 migration 文件符合“无代码/迁移文件”，但 §7.1 只有列清单，**没有 `CREATE TABLE` 全文/索引/字符集/各列长度/`CHECK`/rollback 语句**。人类 OPS-C3 的迁移批准必须以 DDL 为对象；当前 `authorization_id PK`/`op_id UNIQUE`/`CHECK` 仅是文字，无法审核/执行。
2. **B-2（S6 证据仍不闭合）**：设计只读 `shipments.status='CANCELLED'` 或 `shipment_events.status='CANCELLED'` 且 order 匹配（`§4.2:142`）；但 `shipments` 只有 `order_no`、无 `group_no`（`migrations/fulfillment/0001_init.sql:1-16`），无法区分多组订单的具体 group；且“曾存在但丢失”的 S6 证据在只读当前行中不存在，设计直接 exit 2/4，未提供 Fulfillment 的“修复/登记曾存在证据”路径，与 `AC-27-NOWB-01` 终态①（Fulfillment 提供/修复证据）不一致。
3. **B-3（外部授权仍是自报）**：`authorization_id`/`owner_approval_ref` 只是 CLI 字符串（`§3.2:82-90`、`§7:218-223`），`cmd/ops` 只校验 JWT 与 `authorization_expires_at`，**没有向外部授权系统核验 `authorization_id` 是否真实/已批准**；`ops_action_authorizations` 只是把自报值写入新表。AC-62 D-24 的“授权单号/授权人/有效期”因此可被同一 operator 伪造；需外部授权核验或人类签发的不可伪造凭证。

另 **4 个 MAJOR**：JWT 三个 principal 全部是 `admin`，没有 operator/authorizer/reviewer 的**角色分离**（只有 `sub` 不同）；`OPS_ORDER_DSN`/`OPS_AFTERSALES_DSN` 是整库写权限，未定义表级最小授权（`§3.1:72` 只说无 GRANT ALL）；`ops_action_authorizations` 的 `action/refund_id/after_sale_no/batch_id` 无一致性约束（可写 `NOWB_REARM` 配空 `refund_id`）；TCC abandon 虽有 `ensureTccApplicationTx` 默认拒绝，但 RP 无“TCC FAILED 后普通重试”用例。**2 个 TODO**：`remark` 512 的 evidence 截断保留策略；`ops inspect` 的 ACL/审计输出。

## 1. 关闭表

| 上轮项 | R2 判定 | 依据 |
|---|---|---|
| **B-1** 授权/不可抵赖 | **PARTIAL** | JWT/sub/reviewer/expiry 已有（`:69,98,141`）；但 **0024 只有列名、无 DDL**（B-1a）且 **authorization_id/owner_approval_ref 未外部核验**（B-1b）。 |
| **B-2** S6 证据 | **NOT CLOSED** | 只读 shipments/events 可验证当前 CANCELLED（`:142`）；**group 维度缺失**、**曾存在但丢失窗口未覆盖**（B-2）。 |
| **B-3** abandon canonical/owner | **CLOSED（文本）** | `:196` 保留 canonical；`:199` 最新 `OPS_NOWB_ABANDON` guard；`:200` owner_approval_ref；`:218-224` 授权表。 |
| **B-4** `--replay-only` | **CLOSED** | CLI `:88`；语义 `:97,:182`；RP-13 `:283`。 |
| **B-5** NATS ACL/DSN | **CLOSED（文本）** | `:72-73,:127`；publish completed/read `.dlq`；Fulfillment 只读。 |
| **MAJOR** 四标识 | **CLOSED** | `:79-83,:96,:123`；四选择器至少一个，解析不唯一 exit 3/4。 |
| **MAJOR** TCC abandon guard | **PARTIAL** | `:163,:199` 有 guard；RP-8 只有状态，无“TCC FAILED 后普通 TCC 重试”用例。 |
| **MAJOR** `op_id` 唯一 | **CLOSED（设计）** | `:71,:206,:218`；`UNIQUE(op_id)`；DDL 未交付。 |
| **MAJOR** app/admission/task guard | **CLOSED（文本）** | `:163,:199,:210-211`。 |
| **MAJOR** remark/inspect | **PARTIAL** | `:232,:248`；T-1/T-2 仍是 TODO。 |

## 2. `[BLOCKER]`

### B-1 **迁移 0024 未交付/DDL 不可批准**

- 设计 `:15,:22,:237` 明确 `migrations/order/0024_ops_action_authorizations.sql`；工作树 **无此文件**（`migrations/order` 仅到 `0023`）。
- §7.1 `:239` 只有列名，无：
  - `CREATE TABLE`/`PRIMARY KEY`/`UNIQUE(op_id)`/`UNIQUE(authorization_id)` 的实际 DDL；
  - 各列长度（`authorization_id`、`operator_id`、`token_jti_*`、`owner_approval_ref`、`s6_evidence_ref`）；
  - `CHECK` 在 MySQL 8.4 的 enforcement/字符集/引擎；
  - additive 的前向执行顺序、回滚（设计说保留表/数据，无 down，`§10:263`）与无回填声明。
- 因 `[CONTRACT-IMPACT]` 需人类批准迁移，**没有 DDL 文件**时无法完成 OPS-C3；`ops_action_authorizations` 的 `UNIQUE(op_id)`/`CHECK` 不能只停留在文字。**必须先交付 0024 DDL + 影响/回滚说明，再 OPS-C2 复审。**

### B-2 **S6 证据：order 匹配无法区分 group；“曾存在但丢失”窗口无路径**

- 设计 `:142`：`shipments` 行须 `order_no` 匹配且 `status='CANCELLED'`，或 `shipment_events` `status='CANCELLED'` 且 shipment 存在、order 匹配。
- 事实：`shipments` 表只有 `order_no`，**无 `group_no`**（`migrations/fulfillment/0001_init.sql:1-16`；`uk_shipments_order` 是 order 唯一）；`shipment_events` 也只有 `shipment_no`（`0002_shipment_events.sql:1-9`）。因此同一订单**多组**时，一个 CANCELLED shipment 不足以证明“该 group”的 S6 证据；可能用 A 组 evidence re-arm B 组。
- `AC-27-NOWB-01`（SPEC-0001 `:863`）终态①允许“Fulfillment 提供/**修复** S6 运单取消证据（真实记录/承运商取消/仓出库撤销 + 恢复审计）”；设计只读当前行，**曾存在但已丢失**的行查不到就 exit 2/4，未覆盖该合法窗口。需 Fulfillment 证据修复/登记接口或明确“丢失即只能 abandon”的 PO 书面口径（后者会缩小 AC-27-NOWB 范围）。
- 结论：**当前 S6 校验 fail-closed（好），但覆盖不足（group/lost），BLOCKER**。

### B-3 **`authorization_id`/`owner_approval_ref` 未外部核验，仍是自报**

- JWT 只证明 **谁调用**（`sub`/admin/jti），不证明 **`authorization_id` 对应真实外部授权**；设计没有授权系统查询/签名校验（`:69-71,:141`），只把字符串写入 `ops_action_authorizations`。
- `owner_approval_ref` 同理：它是 SPEC-0002 owner 的书面口径，但设计只要求字符串（`:200`），无 owner 签发/验证。
- 一个 admin operator 可自造 `authorization_id=...`、`owner_approval_ref=...`、`authorization_expires_at=未来`，再拉两个 admin `sub` 过 JWT 检查；AC-62 D-24 的“授权单号/授权人/有效期”**不能防抵赖**。需外部授权服务/预登记授权 ID/人类签名文件，`cmd/ops` 只读校验；否则 `[CONTRACT-IMPACT]` 的授权表只是记录自报值。

## 3. `[MAJOR]`

- **M-1 角色未分离**：`:69` 三个 principal 都要求 `roles ⊇ {admin}`；operator/authorizer/reviewer **只是 `sub` 不同**，没有 `ops_operator`/`ops_authorizer`/`ops_reviewer` 角色分离。AC-62“仅授权角色”只能靠 admin，权限提升面大；至少 reviewer 应有独立角色，operator 不应能自审。
- **M-2 DB 最小权限**：`:72` `OPS_ORDER_DSN` 读写“授权表/admission/batch/app”，`OPS_AFTERSALES_DSN` 读写“task/log”，但未定义**表级 GRANT**；`cmd/ops` 用整库用户可写其它表。需迁移/部署明确 `GRANT SELECT/INSERT/UPDATE` 到具体表，Fulfillment 仅 `SELECT shipments/shipment_events`。
- **M-3 授权行一致性**：`:239` 列 `action/refund_id/after_sale_no/order_no/group_no/batch_id` 无 CHECK/NOT NULL 约束；可写 `NOWB_REARM` 但 `refund_id` 空，或 `NOWB_ABANDON` 配 `batch_id` 不一致。需按 action 必填字段的 CHECK/应用校验，并纳入 RP。
- **M-4 TCC abandon 普通重试未测**：`:199` 依赖 `ensureTccApplicationTx` 对 `FAILED_PERMANENT` 默认拒绝；RP-8 只断言 abandon 后状态，未测“TCC app FAILED 后 `TccCancelMarkPaid`/普通 TCC 重试”必须 `TCC_APP_ABANDONED` 且 0 Payment。补充 RP-8b。

## 4. `[TODO]`

- **T-1** `after_sale_logs.remark` 512 截断：保留 `op_id/authorization_id/ticket/reason`，截断 `s6_evidence_ref`；完整 evidence 在授权表/结构化日志/工单（`:232`）。
- **T-2** `ops inspect` 的 ACL/审计输出：仅 admin operator JWT、不打印 token/secret，输出 `op_id/authorization_id/version/stream_seq`（`:248`）；RP-12 的 `NO_REPLAY_DRIVER` 文案与 runbook 对齐。

## 5. `[CONTRACT-IMPACT]` 与门禁

- **是 `[CONTRACT-IMPACT]`**：新增 `ops_action_authorizations` 表 + `0024` 迁移，超出原零迁移假设；必须 **人类 OPS-C3 + 迁移批准**，且 Lead 收窄 Plan `allowed_paths`（含 `migrations/order/0024_*.sql`）。
- **当前不可批准**：B-1 迁移文件不存在；B-2 S6 覆盖不足；B-3 外部授权未核验。**OPS-C2 0 BLOCKER 才 READY**，当前 **3 BLOCKER**。
- **不新增 RPC**；`cmd/ops` 仍是最小入口；`--execute`/JWT/授权表方向正确；`abandon` **no-release** 与 AC-62/INV-45 **不冲突**，**没有**误做成释放。

**只读声明**：未执行代码/测试；只审设计/impact/迁移缺失事实；未改设计/代码/SPEC/ADR；只新建本报告；未 commit。

**Actor**：`qc-ac50`
**Status**：**BLOCKED — 3 BLOCKER；[CONTRACT-IMPACT] 需人类批准，当前 DDL 未交付**
