# ADR-0007 Amendment 1 Addendum — PAY-OPS-REPLAY-01：AC-61 人工重放 / AC-27-NOWB 人工处置（L2）

| 项 | 值 |
|---|---|
| Status | **DRAFT 0.2 — pending QC round 2（OPS-C2）**（未批准；不构成实现依据；不代表 AC-VERIFIED / merge / Release PASS；含 1 个 `[CONTRACT-IMPACT]` 待人类批准） |
| Change ID | **`PAY-OPS-REPLAY-01`**（Parent `PAY-SETTLE-01` 再次暂停；本 L2 交付后 handback） |
| Parent | `PAY-SETTLE-01`（**暂停**；本 L2 不推进其受管代码/台账） |
| Tier | **L2**：P0、资金/退款、跨服务/持久恢复、运维入口；不适用 L1 |
| Base | `160a45d`（Plan `docs/lead/PAY-OPS-REPLAY-01/plan.md`）；本文件工作树 Base 同上 |
| Spec-Ref | SPEC-0001 **v1.5** §16.2 `AC-27-NOWB-01`（`:863`）/`AC-27-LIVE-01`（`:864`）；SPEC-0002 **v1.11 approved** AC-61（`:320-324`）、AC-62（`:328-344`）、AC-50/69/70（只读边界）；ADR-0007 Rev 2.3 + AC-50 0.6（ACCEPTED） |
| Plan/Tasks | `docs/lead/PAY-OPS-REPLAY-01/plan.md`（`:20-38`）、`tasks.md`（`:5-27`） |
| integration_owner | **System Architect**（本设计）；Order（admission/commitment）；AfterSales（task/lease）；Fulfillment（S6 证据）；ops（执行/审计） |
| Actor | `arch-ops-replay`（System Architect） |
| Reviewed-by | **待 QC round 2**（0.1 被 QC BLOCKED；本文件不代签） |
| 硬边界 | **不新增 RPC**；**新增 1 个 additive 授权表（`[CONTRACT-IMPACT]`，migration 0024，须人类批准）**；不改 SPEC/AC-50/69/70 语义；不交付 BFF/UI；不释放/不恢复 permit/不关闭组（abandon）；不 commit |
| 结论级别 | 设计可执行；**无 `[ARCHITECTURAL_FLAW]`**；**`[CONTRACT-IMPACT]`：授权记录表/迁移**；AC-62 abandon 不释放口径仍待 SPEC-0002 owner 书面确认；`workers.go` guard 在同一 allowed 文件 |

## 0. 决策摘要

- **入口形态：`cmd/ops`（独立运维二进制）**，不新增 admin RPC。原因：现有 RPC 不能直接提供该运维入口（`ResolveLegacyResolution` 与 legacy 支付解析无关且无实现；`ReplayDLQ` 仅 seckill；`AdmitRefund` 不能把 REVIEW 复位）；仅 `TccCancelMarkPaid` 可在 re-arm 后重驱动 TCC；脚本无法可靠承载 CAS/租约/审计/DLQ 解析。
- **两条运维动作**：① `replay`（`aftersale.completed` DLQ / durable task 租约丢失的人工重驱动；永久失败 task 不复位）；② `nowb`（S6 证据 + re-arm；无证据 → 人工放弃，**只停止重试/保留负债，不释放额度/不恢复 permit/不关闭组**）。
- **授权记录（0.2 修正）**：`cmd/ops` 消费**现有 identity JWT**（HS256，`admin` 角色；operator/authorizer/reviewer 三个不同 principal，金额 ≥100000 cent 必须第二复核人），并写 **additive `ops_action_authorizations`**（Order DB，`0024`）；`ticket=RVW` 仍只是复核 id，**不是授权单**。此为 `[CONTRACT-IMPACT]`：需人类批准迁移/Allowlist；零迁移不成立。
- **审计**：授权表（不可抵赖：principal/role/金额/复核人/授权号/有效期/op_id 唯一）+ `after_sale_logs`（AfterSales 本地原子）+ Order 行 `version`/时间戳 + 镜像 `last_error` + 结构化日志 + ops 工单（D-32）；TCC 以授权表 + admission/batch 版本 + 日志 + 工单。
- **失败闭合**：缺授权/证据/工单、DLQ 未找到、租约被占、CAS 不匹配、非 REVIEW 的 task 重放、REVIEW 被普通 replay 绕过——全部非零退出且零状态变化。

## 1. 事实基线（证据链）

| 事实 | 证据 |
|---|---|
| 普通售后 durable task 以 `(after_sale_no)` 唯一，状态 `PENDING/PROCESSING/DONE/FAILED_PERMANENT`，含 `attempts/last_error/lease_owner/lease_until` | `migrations/aftersales/0010_settlement_protocol.sql:88-101` |
| worker 以 5s tick 扫描 `PENDING/PROCESSING` 且租约过期；**REVIEW 任务在 attempts≥8 且 canonical `last_error` 时被排除**（保持 PENDING） | `internal/services/aftersales/workers.go:300-326`；`maxTaskAttempts=8` `service.go:58` |
| 任务 claim：行锁 + `PENDING`/过期 lease + `status=PROCESSING`、`lease_until=NOW+1m`、`attempts+1`；owner 为随机 id | `workers.go:794-827` |
| 任务 release：REVIEW 分支写 canonical 并回 PENDING；**非 REVIEW 且 attempts≥8 走 `FAILED_PERMANENT`，并释放 pending 行/额度** | `workers.go:829-896`（永久释放从 `:858` 起） |
| `ensureRefundTaskTx` 会把 `FAILED_PERMANENT` 的 task 重置为 `PENDING`、清 `last_error`、attempts=0（普通 Refund 重试可绕过 abandon） | `workers.go:915-929` |
| 售后日志表：`action VARCHAR(32) / from_status / to_status / remark VARCHAR(512)`，可按 after_sale_id 审计 | `migrations/aftersales/0001_init.sql:31-41` |
| `order_refund_admissions`：`refund_id PK`、`result/state/amount/version/created_at/updated_at`；**无 source 列** | `migrations/order/0016_settlement_admission.sql:7-18` |
| `refund_batches`：`state` 含 `MANUAL_HOLD`，`last_error VARCHAR(1024)`，`version/lease_owner/lease_until` | `migrations/order/0017_refund_batch_ledger.sql:14-33` |
| `refund_applications`：`state` 含 `FAILED_PERMANENT`，`admission_result`（24），`version` | `0017:45-81` |
| `aftersale.completed` topic=`shop.aftersale.completed.v1`，Order consumer durable=`order-aftersale-completed`、MaxDeliver=10 | `service.go:73`；`internal/services/order/aftersale_consumer.go:23-32` |
| DLQ 由通用 consumer 自动发布到 `<subject>.dlq`，payload=`{subject,data,error,attempts,failed_at}`；MALL stream 收 `shop.>`，MaxAge=7 天 | `internal/platform/consumer/consumer.go:78-80,116-130,157-170`；`internal/platform/mq/mq.go:49-51` |
| 现有 `ResolveLegacyResolution` 是 legacy 支付解析域；**internal/services 无实现**，`api/gen` 只有 Unimplemented 默认 | `mall.proto:491-498`；`api/gen/mall/v1/mall_grpc.pb.go:3485`；grep `internal/services/order` 0 命中 |
| 现有 `ReplayDLQ` 是 seckill 专用（`seckill.order.dlq.*`），不能重放 aftersale | `mall.proto:1324`；`internal/services/seckill/dlq.go:13-16`；`seckill/worker.go:159-170` |
| `AdmitRefund` 是业务准入门（幂等 by refund_id），REVIEW 行返回同一冲突；不能复位 REVIEW | `refund_admission.go:316-360`；`AC-69` |
| `TccCancelMarkPaid` 是既有 TCC 补偿入口：REFUNDING 时重跑 `applyTccCompensation` → `admitTccBatch` → `payTccBatch`；`admitTccBatch` 可续 ADMITTING 批 | `internal/services/order/tcc.go:177-360`；`tcc_admission.go:48-60` |
| 现有 runbook 已有 REVIEW 查询/re-arm SQL，但无 executable 入口、无 abandon、无 DLQ 重放 | `docs/ops.md:829-936` |

## 2. 契约要求（必须满足）

- **AC-61**（`:322`）：退款任务永久失败/准入冲突/预留释放失败 → 告警 + 按 `payment_no/refund_id/after_sale_no/batch_id` 的人工重放入口；用户侧“处理中”。
- **AC-61**（`:323`）：REVIEW → 告警 + 人工队列 + 按 `refund_id`（TCC 加 `batch_id`）的 re-arm 入口；不得只依赖 `last_error` 发现。
- **AC-27-LIVE-01**（`0001:864`）：`aftersale.completed` DLQ / task 租约丢失导致 `PENDING_REMOTE` 永久不收敛 → 提供入口；重放收敛或转 NOWB；不得写 PRE/关闭/打款。
- **AC-27-NOWB-01**（`0001:863`）：①真实 S6 证据 + Fulfillment 授权 + re-arm → 合法重放→PRE→组关闭；②无法补齐 → 人工放弃，记 `FAILED_PERMANENT`，组保持未关闭、**不释放/不恢复 permit/不自动关闭/不自动退款**。
- **AC-62**（`0002:331-332`）：REVIEW 期间承诺 `RESERVED`、permit 原值、组未关闭；人工放弃的 D-24 释放是**独立授权动作**；任何 worker/脚本不得静默释放；同 `refund_id` 幂等/版本 CAS。
- **AC-62**（`:343`）：`RESERVED→RELEASED` 只接受打款前 `REJECTED/CANCELLED` 或 `FAILED_PERMANENT/人工放弃 + D-24 授权`；本 L2 的 abandon **不做** release（见 §5.4 口径）。

## 3. 入口形态与总体架构

### 3.1 选择：`cmd/ops`（推荐）

| 方案 | 结论 | 理由 |
|---|---|---|
| **`cmd/ops` 二进制** | **采用** | 无新网络面/RPC；类型化校验、CAS、DLQ 解析、审计、单元/集成可测；以 K8s Job + ops ServiceAccount 运行；只读默认，`--execute` 才写 |
| shell/SQL 脚本 | 拒绝为主 | 无法保证 CAS/租约/JSON 解析/幂等/结构化审计；仅作为事故时 DBA 手工后备（runbook 保留） |
| 新 admin RPC | 不采用（备选） | 需新 proto RPC/鉴权/mesh/BFF 兼容与人类批准；若 ops 无法获得 DB 凭证，再走该路径（§9） |
| 复用现有 RPC | 部分可行 | `ResolveLegacyResolution` 域不符且未实现；`ReplayDLQ` seckill 专用；`AdmitRefund` 不能复位 REVIEW；仅 `TccCancelMarkPaid` 可作 re-arm 后的 TCC 重驱动 |

**权限模型（0.2）**：
- `cmd/ops` 不监听端口；默认 `--dry-run`，写操作必须 `--execute`；`--execute` **不是认证**。
- **身份**：从文件/环境读取 operator/authorizer/reviewer 的 **identity access JWT**（现有 `internal/platform/auth`，HS256，`JWT_SECRET`/`JWT_ISSUER`）；必须 `roles ⊇ {admin}`、未过期、`sub` 分别为三个不同用户；金额 ≥100000 cent（1000 元）必须有 reviewer 且 `reviewed_by ≠ authorized_by`（D-24）。
- **授权单**：`--authorization-id`（外部授权单号，非 `RVW`）、`--authorization-expires-at`（必须 > now；≤24h）；`--owner-approval-ref` 仅 abandon 必填（SPEC-0002 owner 书面口径）。
- **授权记录**：`ops_action_authorizations`（Order DB）写入 `authorization_id/op_id/action/refund_id/amount/operator_id/authorized_by_id/reviewed_by_id/reason/evidence/s6_authorizer_id/authorized_at/expires_at/token_jti...`；`UNIQUE(op_id)`、`UNIQUE(authorization_id)`；与状态变更同 Order 事务或先写后执行（可重入）。
- **DB 权限**：`OPS_ORDER_DSN`（读写授权表/admission/batch/app）、`OPS_AFTERSALES_DSN`（读写 task/log）、`OPS_FULFILLMENT_DSN`（**只读** S6 证据）；不默认 DSN，生产仅 ops Job 的 Secret；无 `GRANT ALL`。
- **NATS ACL**：`ops` 账号仅 publish `shop.aftersale.completed.v1`、仅 read `shop.aftersale.completed.v1.dlq`（+ 必需 `$JS.API.STREAM.INFO.MALL`/`$JS.API.STREAM.MSG.GET.MALL`）；禁止 publish `.dlq`、禁止订阅其它 `shop.>`。

### 3.2 子命令（稳定 CLI 契约）

```text
# 通用认证（必须；文件或 env）：--operator-token-file / --authorizer-token-file / [--reviewer-token-file]
ops replay dlq  [--after-sale-no AS...] [--refund-id R...] [--payment-no P...] [--batch-id B...] \
                [--stream-seq N] [--from-outbox EVENT_ID] \
                --authorization-id AUTH... --authorization-expires-at T \
                --op-id ... --ticket ... --evidence-ref ... [--execute]
ops replay task [--after-sale-no AS...] [--refund-id R...] [--payment-no P...] [--batch-id B...] --reason LEASE_LOST \
                --authorization-id AUTH... --authorization-expires-at T \
                --op-id ... --ticket ... [--execute]
ops nowb rearm  --refund-id R... --order-no O... --s6-authorizer-token-file F --s6-evidence-ref EVID \
                --authorization-id AUTH... --authorization-expires-at T \
                [--replay-only] [--order-grpc ADDR] --op-id ... [--execute]
ops nowb abandon --refund-id R... --reason NO_EVIDENCE|S6_UNRECOVERABLE \
                --owner-approval-ref OWNER... \
                --authorization-id AUTH... --authorization-expires-at T \
                --op-id ... --ticket RVW:R... [--execute]
ops inspect {dlq|task|review} ...  --operator-token-file ...
```

- `replay dlq`/`replay task` 四个选择器至少一个；解析优先级/唯一性见 §4.2；`--after-sale-no` 兼容旧调用。
- `nowb rearm --replay-only`：跳过 DB re-arm，仅对已 `ADMITTING` 的 TCC 批调用 `TccCancelMarkPaid`；无 `--order-grpc` → exit 6；普通售后/无批拒绝；重复调用幂等。
- `--operator-token-file` 等为 identity JWT；`sub` 不同；`--reviewer` 仅金额 ≥100000 cent 必需；token 过期/角色不符 exit 2。
- `--ticket` 仍是 `RVW:<refund_id>`（AC-69）；`--authorization-id` 是 AC-62 授权单号，二者不得互换。
- `ops inspect` 只读，但需 operator token（admin）；输出 `op_id/authorization_id/version/stream_seq`；不写状态。

- 退出码：`0`=成功/已应用；`2`=参数/授权拒绝；`3`=状态/租约/CAS 冲突；`4`=DLQ/outbox/task 未找到；`5`=依赖不可用；`6`=部分执行需重试（已写审计）。
- 所有写命令要求 `--op-id`（唯一审计键，重试复用）、`--authorization-id`、`--authorization-expires-at`、operator/authorizer JWT；re-arm 另需 Fulfillment token + 可核验 `--s6-evidence-ref`；abandon 另需 `--owner-approval-ref` 与 `--ticket`。
- 金额从 admission 读取；调用方不得覆盖。`--execute` 前完成 JWT/角色/金额/双人/有效期/授权单校验；任一失败 exit 2，零写。

## 4. 操作 A：`replay`（AC-61 / AC-27-LIVE-01）

### 4.1 分支

> `replay task` 只覆盖 **租约丢失/可重试 task**；`FAILED_PERMANENT` 自动路径已经释放 pending/额度（`workers.go:858-888`），同 task 重放会绕过释放事实，故**禁止**；AC-61 的“永久失败”人工入口以新 `refund_id`/AC-62 处置，不由本 CLI 复位。

| 分支 | 触发事实 | 动作 |
|---|---|---|
| **A1 DLQ 事件** | `aftersale.completed` 已进 DLQ；AfterSales task 可能已 DONE，但 Order 投影未收敛 | 定位 DLQ 消息 → 原样重发 `data` 到 `shop.aftersale.completed.v1`；不写 DB |
| **A2 task 租约丢失** | task `PROCESSING` 且 `lease_until < NOW`（worker 本应自动捡起） | worker 下一 tick 自动重试；`replay task` 可清 lease/attempts 让 worker 立即重试；若 lease 有效 → `LEASE_HELD`；`FAILED_PERMANENT` 见 A3 |
| **A3 task 永久失败（非 REVIEW）** | task `FAILED_PERMANENT`（自动永久路径已释放 pending/额度，`workers.go:858-888`） | **拒绝同 task 重放**（`PERMANENT_REQUIRES_NEW_REFUND`）：不复位、不重试、不触碰 admission；由人工按 AC-62/新 `refund_id` 处理 |
| **A4 REVIEW task** | `last_error LIKE 'REVIEW_REQUIRED reason=%'` | **拒绝** A3；必须走 `nowb rearm/abandon`（不得用普通 replay 绕过人工队列） |
| **A5 租约丢失强制重试** | `PROCESSING` 且 `lease_until` 过期/空 | `replay task` 清 lease/attempts（不触碰 admission），worker 立即重试；有效 lease → `LEASE_HELD` |

### 4.2 DLQ 定位与匹配

- Stream `MALL`（`shop.>`，7 天）；DLQ subject `shop.aftersale.completed.v1.dlq`（consumer 默认 `subject+".dlq"`）。
- 消息为 wrapper：`data` 是原始 event JSON；`after_sale_no/order_no` 在 `data` 内（`events.go:24-25`）。**四标识选择器**（AC-61 `0002:322`）：`--after-sale-no` 直接匹配；`--refund-id` 经 `order_refund_admissions`/`refund_applications` 解析到 order/group → after_sale_no；`--payment-no`/`--batch-id` 经 `refund_batches`/`refund_applications` 解析；解析不唯一/0 条 → exit 3/4，不猜。
- 未找到：退出 4，**不发布、不写审计成功**；若显式给 `--from-outbox EVENT_ID`，则从 AfterSales `outbox_events`（`event_type='aftersale.completed'`、`aggregate_id=after_sale_no`、payload JSON）重发；这是**显式**后备，不静默回退（AC-27-LIVE 证据要求）。
- 审计载体：按 `after_sale_no` 查 `after_sales.id` 写 `after_sale_logs`；查不到 → exit 4 `NO_AUDIT_CARRIER`，**不发布**（不得无审计重放）；DLQ 多条匹配且未给 `--stream-seq` → 列出并 exit 3，不任选。
- 重发：原 `data` 字节不变；header `Nats-Msg-Id=ops:<op-id>:<stream_seq|event_id>`；trace 由 `mq.PublishMsg` 注入。
- **NATS ACL（B-5）**：ops 账号仅允许 `publish shop.aftersale.completed.v1`、`subscribe shop.aftersale.completed.v1.dlq`、`$JS.API.STREAM.INFO.MALL`/`MSG.GET.MALL`；**禁止** publish `.dlq`、禁止订阅其他 `shop.>`；ACL 拒绝 → exit 2/5，零写。

### 4.3 幂等/并发/崩溃

- **幂等键**：`op_id + (subject, stream_seq|event_id)`；DB task 复位用 `after_sale_no + status/lease` CAS。
- 重复 replay：Order `applyAfterSaleCompleted` 以 `refund_id/state` 幂等（`aftersale_consumer.go:39`；`refund_consumer` 投影）；重复发布最多重复消费，不得重复取消/占位。审计允许 at-least-once，同一 `op_id` 第二次标记 `duplicate=true`。
- 并发：两个 ops 同时 replay 同一消息 → 都发布，consumer 幂等；task 复位用 `FOR UPDATE` + lease/status，只有一个 RowsAffected=1；另一个返回 `ALREADY_APPLIED`/`CAS_MISMATCH`。
- 崩溃：publish 后、审计前 → 重试同 `op_id` 重发，安全；task 复位事务内同时写 `after_sale_logs`，崩溃无部分状态。

## 5. 操作 B：`nowb`（AC-27-NOWB-01）

### 5.1 通用前置

- 读取 `order_refund_admissions`：必须 `result='REVIEW_REQUIRED' AND state='PENDING_REMOTE'`；否则 `NOT_IN_REVIEW`（terminal/PENDING 不得操作）。
- 授权：operator/authorizer/reviewer 的 **identity JWT**（`admin` 角色；sub 不同；exp 有效）；`authorization_id`（外部授权单）+ `authorization_expires_at`（>now）；金额 ≥100000 cent 必须有 reviewer 且 ≠authorizer；`ticket` 仍是 `RVW:<refund_id>`（复核 id，不是授权单）；`op_id` 唯一；`amount_cent` 从 admission 读取。
- **S6 证据（B-2）**：`s6_evidence_ref` 必须是 Fulfillment 可核验的 `shipment_no` 或 `shipment_events.id`；`cmd/ops` 只读 `OPS_FULFILLMENT_DSN`：`shipments` 行须 `order_no` 匹配且 `status='CANCELLED'`，或 `shipment_events` 行须 `status='CANCELLED'` 且 shipment 行存在并 `order_no` 匹配（历史行不能脱离 shipment 证明 order）；**查不到/不匹配 → exit 2/4，零写**。`s6-authorizer` 为 Fulfillment admin JWT，sub 记入授权表；自由文本/伪造 evidence 拒绝。
- 顺序：**先打开 ops 工单 + 结构化日志 `review_rearm`/`review_abandon`，后 CAS**；Order 与 AfterSales 两库不能一个事务，按 §5.2 的两阶段 + 可重入处理。

### 5.2 Re-arm（终态①）

**Order DB（一个事务）**：

```sql
SELECT result,state,version,order_no,group_no,amount_cent
  FROM order_refund_admissions WHERE refund_id=? FOR UPDATE;  -- 必须 REVIEW/PENDING
UPDATE order_refund_admissions
   SET result='PENDING_REMOTE', version=version+1, updated_at=NOW(3)
 WHERE refund_id=? AND result='REVIEW_REQUIRED' AND state='PENDING_REMOTE' AND version=?; -- RowsAffected=1
-- 有 batch_id 时同一事务：
SELECT state,version,last_error FROM refund_batches WHERE batch_id=? FOR UPDATE; -- 必须 MANUAL_HOLD
UPDATE refund_batches
   SET state='ADMITTING', version=version+1, updated_at=NOW(3)
 WHERE batch_id=? AND state='MANUAL_HOLD' AND version=?; -- RowsAffected=1
```

- **保留** `refund_batches.last_error` canonical 审计文本，不清空；`refund_applications.admission_result` 保持 REVIEW，等待重评（`ensureTccApplicationTx` 认合法值）。
- **abandon 防复位（M-4）**：re-arm 前必须检查 `ops_action_authorizations` 是否存在 `action='NOWB_ABANDON' AND refund_id=?`（存在→`ALREADY_ABANDONED`）；有 `refund_applications.state='FAILED_PERMANENT'` → `TCC_APP_ABANDONED`；有 task 最新 `OPS_NOWB_ABANDON` log → `ALREADY_ABANDONED`；三者任一命中均零写。
- **无 source 列**：普通售后由 task 重驱动（`refundAdmissionSource` 从 after_sales 派生）；TCC 从 `refund_batches.source` 取 `tcc`；legacy 组级若无 task/app，拒绝（`NO_REPLAY_DRIVER`），不得猜 source（`order_refund_admissions` 无 source，`0016:7-18`）。

**AfterSales DB（有 task 时，一个事务）**：

```sql
SELECT status,attempts,lease_owner,lease_until,last_error
  FROM after_sale_refund_tasks WHERE after_sale_no=? FOR UPDATE;
-- 要求 status IN ('PENDING','PROCESSING') 且 lease 空闲/过期；FAILED_PERMANENT（abandoned）拒绝
UPDATE after_sale_refund_tasks
   SET status='PENDING', attempts=0, lease_owner='', lease_until=NULL, updated_at=NOW(3)
 WHERE after_sale_no=?;
INSERT INTO after_sale_logs(after_sale_id,action,from_status,to_status,remark)
  VALUES (?, 'OPS_NOWB_REARM', ?, 'PENDING', ?);  -- remark ≤512：op/ticket/operator/authorizer/evidence/version
```

- 保留 canonical `last_error`（审计）；worker 在下一 tick 因 attempts=0 重新领取（`workers.go:319-326` 的排除条件不再命中）。
- **重驱动**：有 task → worker 自动（无需 CLI 调用）；无 task 且有 batch → CLI 用**既有** `TccCancelMarkPaid` gRPC（`order_no/payment_no`，不新增 RPC），让 `applyTccCompensation→admitTccBatch` 继续 ADMITTING 批；`--order-grpc` 未给 → 仅完成 DB re-arm，退出 6 `REPLAY_PENDING`，ops 重跑 `--replay-only`。
- 若重放仍 `<NO_WAYBILL>`，`admitTccBatch` 内的 `AdmitRefund` 返回 REVIEW，`persistTCCReviewTx` 重写 canonical，批次回 `MANUAL_HOLD`；不写 PRE/不打款。
- **`--replay-only`（B-4）**：不重做 Order/AfterSales DB re-arm；前置 `admission.result=PENDING_REMOTE` 且 `batch.state=ADMITTING`（否则 exit 3）；仅调 `TccCancelMarkPaid`；成功 exit 0，仍 REVIEW 则 batch 回 MANUAL_HOLD（exit 6 可重试）；普通售后/无 batch 拒绝 `NOT_TCC_ADMITTING`。

### 5.3 状态图（re-arm）

```text
Order: REVIEW_REQUIRED/PENDING_REMOTE --CAS(version)--> PENDING_REMOTE/PENDING_REMOTE
TCC:   MANUAL_HOLD --CAS(version)--> ADMITTING --TccCancelMarkPaid(admitTccBatch)--> PRE/POST | REVIEW->MANUAL_HOLD
Task:  PENDING/PROCESSING(lease free/expired) --CAS--> PENDING(attempts=0) --worker--> PROCESSING -> DONE | REVIEW(PENDING)
```

### 5.4 Abandon（终态②；**不释放、不关闭、不恢复**）

**定义**：无法补齐 S6 证据时，停止自动重试并留下人工结论；**不是** AC-62 release，不做 `RESERVED→RELEASED`、不改 permit、不关闭组、不写 `DONE`。

- **普通售后**（有 task）：AfterSales 单事务 `FOR UPDATE`，要求 `status IN ('PENDING','PROCESSING')` 且 lease 空闲/过期；`UPDATE ... SET status='FAILED_PERMANENT', lease='' WHERE ...`；**`last_error` 保持 AC-69 canonical `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>` 不变**（不追加 `OPS_ABANDONED`，避免破坏 D-35 解析）；`INSERT after_sale_logs('OPS_NOWB_ABANDON', remark=op/ticket/authorization/owner_approval/evidence)`。**不得**调用 `releasePendingForAfterSaleTx`/改 `after_sale_amount_ledger`/`amount_state`（对比 `workers.go:858-888` 的自动永久路径）。
- **TCC**（有 `refund_applications`）：Order 单事务 `FOR UPDATE`，要求 app `state='PENDING'`；`UPDATE refund_applications SET state='FAILED_PERMANENT', version=version+1 WHERE refund_id=? AND state='PENDING' AND version=?`；batch 保持 `MANUAL_HOLD`、`last_error` 保留；admission 行保持 REVIEW/PENDING、commitment `RESERVED`、permit 原值。无 app 的 legacy 组级拒绝（`NO_TERMINAL_CARRIER`），不得新建行。
- **admission 行不删除/不改 result**；队列/告警仍可见该 REVIEW 项（它仍是未结业务负债），abandon 只是“停止重试+人工结论”；AC-72 backlog 计入它是正确的，不能把 abandon 当已退款/已关闭。
- **防绕过（B-3/M-4）**：`ensureRefundTaskTx`（`workers.go:915-929`）在 task `FAILED_PERMANENT` 时，必须查 `after_sale_logs` **最新** action；若为 `OPS_NOWB_ABANDON` 则**返回失败**（“refund review abandoned; new refund_id required”），不得重置 PENDING。`last_error` 不再作为 marker，canonical 保留。TCC：`nowb rearm` 与 `ensureTccApplicationTx` 对 `refund_applications.state='FAILED_PERMANENT'` 必须拒绝（`TCC_APP_ABANDONED`）；授权表 `ops_action_authorizations` 的 `action='NOWB_ABANDON' AND refund_id` 作为第二道 guard。
- **AC-62 owner 口径（B-3）**：abandon 必填 `--owner-approval-ref`（SPEC-0002 owner 对 AC-62/D-24 的书面口径），与 `authorization_id` 分开；写入授权表/日志/工单。本设计按 Lead 指令 **不做 release**；D-24 释放是独立授权动作，不宣称已实现。若 PO 要求 abandon 同事务 release，必须回 SPEC/人类。

## 6. 租约 / CAS / 幂等总表

| 对象 | 并发控制 | 幂等键 | 冲突行为 |
|---|---|---|---|
| `ops_action_authorizations`（新） | `UNIQUE(op_id)` + `UNIQUE(authorization_id)` | `op_id` | 同 `op_id` 重试读回同一行；不同参数 → `AUTHORIZATION_CONFLICT` |
| `order_refund_admissions` REARM | `FOR UPDATE` + `version` CAS | `refund_id + version + op_id` | RowsAffected=0 → 读当前；仍 REVIEW → `CAS_MISMATCH`；已 PENDING → `ALREADY_REARMED` |
| `refund_batches` REARM | `FOR UPDATE` + `version` CAS | `batch_id + version` | 0 行 → 非 `MANUAL_HOLD` 时 `CAS_MISMATCH` |
| `after_sale_refund_tasks` REARM/REPLAY | `FOR UPDATE` + status/lease | `after_sale_no + op_id` | valid lease → `LEASE_HELD`；FAILED/unknown → `CAS_MISMATCH` |
| `after_sale_refund_tasks` ABANDON | `FOR UPDATE` + status/lease | `after_sale_no + op_id` | 已 `FAILED_PERMANENT` 且 `after_sale_logs` 最新为 `OPS_NOWB_ABANDON` → `ALREADY_ABANDONED`（exit 0） |
| `refund_applications` ABANDON | `FOR UPDATE` + `version` | `refund_id + version + op_id` | 已 `FAILED_PERMANENT` → `ALREADY_ABANDONED`；`nowb rearm`/TCC retry → `TCC_APP_ABANDONED` |
| DLQ replay | 无 DB lease；NATS `Nats-Msg-Id` | `op_id + stream_seq/event_id` | 重复发布安全；audit `duplicate=true` |

## 7. 授权与审计字段

| 字段 | 必填 | 说明 |
|---|---|---|
| `op_id` | 是 | 唯一审计键；`ops_action_authorizations.UNIQUE(op_id)`；重试复用 |
| `authorization_id` | 是 | AC-62 授权单号（外部）；`UNIQUE`；**不是** `ticket=RVW` |
| `authorization_expires_at` | 是 | > now 且 ≤24h；过期拒绝 |
| `operator_id/authorized_by_id` | 是 | JWT `sub`；admin 角色；两者不同 |
| `reviewed_by_id` | 金额≥100000 cent | 第二复核人；≠authorized_by（D-24） |
| `owner_approval_ref` | abandon | SPEC-0002 owner 对 AC-62/D-24 的书面口径 |
| `s6_authorizer_id` / `s6_evidence_ref` | re-arm | Fulfillment admin JWT sub + 可核验 `shipment_no`/`shipment_events.id`；查不到→拒绝 |
| `ticket` | 是 | `RVW:<refund_id>`；AC-69 复核 id，**不是授权单** |
| `reason` | 是 | `S6_EVIDENCE_RECOVERED`/`NO_EVIDENCE`/`S6_UNRECOVERABLE`/`LEASE_LOST` |
| `refund_id/order_no/group_no/batch_id/amount_cent/version` | 是 | 从 admission/batch 读取；金额不得由调用方覆盖 |
| `token_jti_*` | 是 | 三个 JWT 的 jti，防重放/不可抵赖 |
| `stream_seq/event_id/subject` | replay | DLQ/outbox 定位 |
| 审计载体 | — | 新授权表 + `after_sale_logs`（action `OPS_NOWB_REARM`/`OPS_NOWB_ABANDON`/`OPS_DLQ_REPLAY`/`OPS_TASK_REPLAY`，remark ≤512）+ Order 行 version/时间戳 + 结构化日志 + ops 工单 |

- `action` 长度 ≤32、`remark` ≤512（`0001_init.sql:34,37`）；**T-1**：remark 优先保留 `op_id/authorization_id/ticket/reason`，截断 `evidence_ref`；完整 evidence 在授权表/结构化日志/工单。
- **B-3**：abandon 不改 canonical `last_error`；处置证据在 `after_sale_logs`/授权表/工单。TCC 以授权表 + `refund_batches`/`refund_applications` version + 日志 + 工单为 D-32 载体。

### 7.1 `[CONTRACT-IMPACT]` 授权记录表（B-1；**需人类批准**）

- **决策**：新增 additive `ops_action_authorizations`（Order DB，`migrations/order/0024_ops_action_authorizations.sql`）。这是对 0.1 “零迁移”假设的**修正**，不是默认。
- **不复用 `refund_commitment_releases`**：该表是 AC-62 **release** 专用（`refund_id` 唯一、金额>0、插入即代表释放；`0018:77-103`），用于 re-arm/abandon 会伪造 release、违反 INV-45。
- **最小列**：`authorization_id PK`、`op_id UNIQUE`、`action`（`NOWB_REARM|NOWB_ABANDON|TASK_REPLAY|DLQ_REPLAY`）、`refund_id/after_sale_no/order_no/group_no/batch_id`、`amount_cent`、`operator_id/authorized_by_id/reviewed_by_id`、`owner_approval_ref`、`s6_authorizer_id`、`s6_evidence_ref`、`authorization_expires_at`、`reason_code`、`token_jti_operator/authorizer/reviewer`、`created_at`；`CHECK (amount_cent < 100000 OR (reviewed_by_id IS NOT NULL AND reviewed_by_id <> authorized_by_id))`。
- **写入**：Order DB 与 admission/batch 状态变更同事务；AfterSales-only（task replay/abandon）先写授权行再执行，`op_id` 唯一保证重试不重复；失败保留授权行（审计）。
- **批准路径**：`[CONTRACT-IMPACT]` → OPS-C2 QC + **人类 OPS-C3 + 迁移批准**（Architect 出 DDL/影响，Lead 收窄 allowed）；未批准前**不得实现/写码**；若人类拒绝迁移，则本设计对 B-1 无合规替代（不接受仅 JWT 自报或 `ticket=RVW` 授权），必须回 Plan 改为 admin RPC/外部授权服务（同样需人类批准）。

## 8. 可见性 / 指标

- 不新增 metric；复用 `mall_refund_review_backlog`/`mall_refund_review_oldest_seconds`（Order）与 `AfterSaleRefundRecovery`（AfterSales）；`cmd/ops` 输出 JSON 摘要 + exit code，结构化日志字段 `op_id/action/result/rows/version/stream_seq`。
- **abandon 不改变 admission**，所以 backlog 仍计入该 REVIEW（正确：负债未清）；runbook 增加“已 abandon 的 task”查询，避免 ops 重复处置。
- DLQ 可见：`ops inspect dlq` 输出 stream seq/失败原因；没有新告警规则（`REVIEW-REQUIRED-ALERTS-01` 仍 DEFERRED）。
- **`ops inspect`（T-2）**：只读；仍需 admin operator JWT；输出 `op_id/authorization_id/refund_id/state/version/stream_seq`；不打印 token/secret；ACL 同 §3.1（只读 DB，不 publish）。

## 9. 崩溃 / 恢复窗口

| 窗口 | 后果 | 恢复 |
|---|---|---|
| C1 Order re-arm 已提交、AfterSales task 未复位 | worker 不会捡 REVIEW task；无自动副作用 | 重跑 `nowb rearm`（Order 已 PENDING → `ALREADY_REARMED`，只做 task 复位） |
| C2 task 复位已提交、worker 未运行 | 只是 PENDING，可等 tick；无重复 | 重启 worker / 等待；不重复 CAS |
| C3 batch ADMITTING、gRPC 未调用 | 批次不再自动推进；0 Payment | `--replay-only` 用既有 `TccCancelMarkPaid`；未完成前不得手工改 DONE |
| C4 DLQ 已发布、审计未写 | 重复消费幂等 | 同 `op_id` 重试；审计 at-least-once |
| C5 abandon 已提交、响应丢失 | task/app FAILED；admission REVIEW 保留 | 重跑返回 `ALREADY_ABANDONED`；不可自动回滚 |
| C6 两 ops 并发 | version/lease CAS 只一个成功 | 另一个读当前；`ALREADY_*`/`CAS_MISMATCH` |

## 10. 回滚 / 前向修复

- 代码回滚：删 `cmd/ops` 即可；新增授权表为 **additive**，回滚保留表/数据，不删/不回填；不提供 down migration（按项目迁移纪律）；回滚不改变业务状态（但已执行的 re-arm/abandon 不可逆）。
- 状态回滚：re-arm 后的 admission 可再次 REVIEW/PRE/POST，**不能**“撤销”为 REVIEW 或回滚已产生的 PRE/POST；abandon 是人工终态，**不得自动 un-abandon**；如需继续，使用新 `refund_id`/AC-62 授权；回滚前先审计（`docs/ops.md` 回滚约束）。
- 禁止把 `last_error` 清空/删除 admission 行作为回滚；禁止直接改 `refund_batches` 到 DONE/PAID。

## 11. 测试矩阵（OPS-4；真实 MySQL 8.4 + NATS）

| # | 场景 | 断言 |
|---|---|---|
| RP-1 | 重复 DLQ replay 同 `op_id` | 事件只投影一次；无重复取消/占位；第二次 `duplicate=true`/幂等 |
| RP-2 | task 有效 lease 被 worker 占用 | `LEASE_HELD`；无状态变化、无审计成功 |
| RP-3 | DLQ 缺失/过期 | exit 4；0 publish/0 DB 写；不猜 |
| RP-4 | JWT 角色/过期/operator=authorizer/金额≥1000 无 reviewer/authorization_expires_at 过期/授权单重放 | exit 2；0 写；审计拒绝；`op_id` 唯一 |
| RP-4b | S6 evidence 伪造/不存在/order 不匹配 | exit 2/4；re-arm 不执行；0 admission/batch/log 写 |
| RP-4c | NATS ACL 越权（publish 非 completed、subscribe 非 DLQ） | 连接/发布拒绝；exit 5；0 写 |
| RP-5 | re-arm 审计 | `after_sale_logs` action/remark 含 op/ticket/evidence；Order version+1；TCC `last_error` 未清 |
| RP-6 | re-arm 崩溃 C1/C3 | 重跑幂等；无重复 PRE/Payment；TCC 可 replay-only |
| RP-7 | NOWB 终态① | 真实 `CANCELLED` → PRE；`SHIPPED` → POST；仍无运单 → REVIEW 回 MANUAL_HOLD |
| RP-8 | NOWB 终态② abandon | task/app FAILED；admission 仍 REVIEW；commitment RESERVED；permit 原值；组未关闭；0 Payment/0 release |
| RP-9 | abandon 后普通 `Refund` | 被 `after_sale_logs` 最新 `OPS_NOWB_ABANDON` guard 拒绝；task 不重置；canonical `last_error` 保留；无重试 |
| RP-9b | TCC abandon 后 `TccCancelMarkPaid`/普通 TCC retry | `TCC_APP_ABANDONED`；app 仍 FAILED；0 Payment；batch 仍 MANUAL_HOLD |
| RP-13 | `--replay-only`（C3） | batch ADMITTING 时仅重驱动、不重复 DB re-arm；无 gRPC exit 6；重复调用幂等 |
| RP-10 | 并发 re-arm/abandon | 仅一个 RowsAffected=1；另一个 `ALREADY_*`/`CAS_MISMATCH`；无部分状态 |
| RP-11 | task 永久失败非 REVIEW（已释放） | `replay task` **拒绝** `PERMANENT_REQUIRES_NEW_REFUND`；task/admission/额度无变化；不重复退款/释放 |
| RP-12 | 无 task/app 的 legacy 组级 | exit 3 `NO_REPLAY_DRIVER`/`NO_TERMINAL_CARRIER`；不猜 source/不新建行；inspect 输出对齐 runbook |

## 12. 不做 / 假设与不确定性

- **不做**：新 RPC；BFF/UI；自动 release；自动 re-arm；修改 AC-50/69/70 成功语义；TCC 新业务规则；生产数据批量修复。**新增 1 张授权表/迁移（§7.1）是 `[CONTRACT-IMPACT]`，须人类批准**。
- **AC-61 永久失败同 task 重放不在本 L2**：自动永久路径已释放 pending/额度（`workers.go:858-888`），同 task 重放会绕过释放事实；以新 `refund_id`/AC-62 处置。若 PO 要求同 task 重放，必须重新预留/重新准入设计并回人类批准。
- **假设**：ops Job 可获 Order/AfterSales DB 只读写权限与 NATS 发布权限；MALL stream 7 天保留（`mq.go:51`）；DLQ wrapper 格式不变（`consumer.go:157-170`）；普通售后/legacy 有 task、TCC 有 app；`after_sale_logs.action/remark` 长度足够（`0001_init.sql:34,37`）。
- **[不确定]**：TCC 无 task 时走既有 `TccCancelMarkPaid`，依赖 Payment snapshot/order/Σ（`tcc.go:177-360`）；不可用则 `REPLAY_PENDING`，不得伪造 DONE；legacy 组级无 task/app 的处置需 PO/ops 确认；AC-62 的 abandon 是否必须伴随 D-24 release 需 PO 文字口径（本设计按 Lead 指令 no-release）。
- **风险**：`cmd/ops` 是特权 DB 写入面；以只读默认、`--execute`、ops RBAC、审计字段、无网络监听收敛；若未来 schema 变更，必须同步 CLI SQL 与测试。

## 13. Owner / AC 映射 / 登记

| 项 | Owner | 证据 |
|---|---|---|
| `cmd/ops` + tests | Developer | `cmd/ops/**`、`_test.go`；RP-1..RP-12 |
| `after_sale_logs` audit / task guard | AfterSales/Developer | `workers.go` guard；RP-5/RP-9 |
| runbook 更新 | ops | `docs/ops.md`（replay/rearm/abandon/inspect） |
| S6 证据/授权 | Fulfillment | admin JWT sub + `shipment_no`/`shipment_events.id` 可核验证据 |
| TCC replay | Order | 既有 `TccCancelMarkPaid`（经 `admitTccBatch`）；batch CAS |
| AC-27-NOWB/LIVE 复审 | Lead/Reviewer | `ac-coverage.tsv` 仅 Reviewer；AC-27 仍 AC-REF |

**Actor**：`arch-ops-replay`（System Architect）
**Status**：**DRAFT 0.2 — pending QC round 2（OPS-C2；含 `[CONTRACT-IMPACT]` 授权表）**
