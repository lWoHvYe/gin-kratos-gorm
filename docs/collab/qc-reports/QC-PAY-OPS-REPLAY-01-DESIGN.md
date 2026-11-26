# QC-PAY-OPS-REPLAY-01-DESIGN — AC-61 / AC-27-NOWB 运维设计红队（OPS-C2）

| 项 | 值 |
|---|---|
| change_id | `PAY-OPS-REPLAY-01`（Parent `PAY-SETTLE-01` 暂停） |
| 审查对象 | `docs/adr/0007-amd1-addendum-ops-replay.md` **DRAFT 0.1**（269 行）；`docs/adr/0007-amd1-ops-replay-impact.md`（91 行）；`docs/lead/PAY-OPS-REPLAY-01/plan.md` / `tasks.md`；SPEC-0001 §16.2、SPEC-0002 AC-61/62、`docs/ops.md` |
| 结论 | **BLOCKED**（5 个 `[BLOCKER]` + 4 个 `[MAJOR]` + 2 个 `[TODO]`） |
| 日期 | **2026-11-27** |
| Actor | `qc-ac50`（Quality Checker；与 `arch-ops-replay` 不同 actor） |
| base | `160a45d`（设计 Base；当前 head `bda51bb`） |
| 被审 blob | ADR `ea3fa437039f1f5f095d78fdf2ecbe804e1a7abe`；impact `80efe1f8608bc41f17c106649696dfb82c624d1f` |
| release_status | **NOT RELEASE**；不批准实现/merge/Release；设计 DRAFT，未人类批准。 |

## 0. 结论

**BLOCKED。** `cmd/ops` 的“零新 RPC/字段/表/迁移、只读默认、`--execute` 才写”方向正确；`AC-61` 的 DLQ/lease 定位、`replay` 幂等、`FAILED_PERMANENT` 拒绝、`NO_REPLAY_DRIVER` 基本成立；`abandon` **不 release、不恢复 permit、不关闭组、0 Payment** 与 AC-62/INV-45 不冲突，**没有把人工放弃误做成释放**（`§5.4:173-179`）。

但设计仍有 **5 个 BLOCKER**：

1. **授权/不可抵赖不足**：`operator`/`authorized_by` 是调用方自由文本，`--execute` 不是身份认证；没有 principal/role 校验、没有金额 ≥1000 的双人复核机制、没有授权有效期/授权单号；`ticket=RVW:<refund_id>` 是复核 id，不是 AC-62 D-24 的**授权单号**。`RP-4` 声称测“未授权/金额≥1000 无双人”，但 CLI 契约 `§3.2:72-80` 没有第二授权人/有效期字段，无法执行。
2. **S6 证据未验证**：`s6_authorizer`/`s6_evidence_ref` 只是字符串（`§7:198-199`），`cmd/ops` 没有 Fulfillment 查询/签名校验；re-arm 可用伪造 evidence 执行，违反 AC-27-NOWB 的“Fulfillment 提供/修复真实 S6 证据 + ops 凭授权核对”（SPEC-0001 `:863`）。
3. **abandon 的 canonical/AC-69 审计被破坏**：`§5.4:175` 把 `last_error` 写成 `OPS_ABANDONED ticket=...`，丢失 `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>`（SPEC-0002 AC-69 `:465` 要求复核进入与每次处置的唯一格式）；`AC-27-NOWB` 还要求“取得 SPEC-0002 owner 书面口径”，设计只要求 `authorized_by`。
4. **`--replay-only` 不存在于 CLI 契约**：`§5.2:222`/C3 要求 `--replay-only` 续跑 TCC，但 `§3.2:72-80` 的 `nowb rearm` 没有该 flag；C3 的恢复路径不可执行。
5. **ACL 未定义**：设计只说 `NATS_URL`/ops SA（`:67`），没有 MALL stream/subject 的发布 ACL、DLQ 读取 ACL、`cmd/ops` 的 NATS 权限边界；AC-61/DLQ 的可见/重放安全不可验收。

另 **4 个 MAJOR**：AC-61 入口只按 `after_sale_no`，缺 `payment_no/refund_id/batch_id` 定位（SPEC-0002 `:321-322`）；TCC abandon 复用了 `ensureTccApplicationTx` 的默认拒绝但未在设计与 RP 明确断言；`after_sale_logs.op_id` 无唯一键，审计 at-least-once 不能唯一定位；`OPS_ABANDONED` guard 只覆盖 AfterSales task，需明确 TCC app 的 `FAILED_PERMANENT` 不可复位测试。**2 个 TODO**：`after_sale_logs.remark` 512 的证据引用截断策略；`ops inspect` 的 ACL/审计输出。

## 1. 入口/最小面

| 检查 | 判定 | 依据 |
|---|---|---|
| `cmd/ops` 最小入口（无新 RPC/proto/迁移/字段/表） | **PASS** | 设计 `§3:62-67`、impact `§3`：仅 `cmd/ops/**` + `workers.go` guard；无 proto/migration；`TccCancelMarkPaid` 复用既有 RPC。 |
| 只读默认、`--execute` 才写 | **PASS（机制）** | `§3.1:67`、`§3.2:85-87`；默认 dry-run、参数完整性替代交互确认。 |
| 两库/两阶段/可重入 | **PASS** | `§5.1:125-127`、`§5.2`、C1/C3 `:222`；Order 先、task 后；`ALREADY_*`。 |
| 无 BFF/deploy/迁移 | **PASS** | `§12:251-253`；Plan `forbidden_paths`。 |

## 2. 授权/审计

| 检查 | 判定 | 依据/缺口 |
|---|---|---|
| 谁能执行（principal/role） | **FAIL** | `§7:198`：`operator`/`authorized_by` 只是字符串；“ops namespace Job 读 Secret”只证明 Job 身份，不证明具体 operator/authorizer 角色；`--execute` 不是认证。 |
| AC-62 金额 ≥1000 双人 | **FAIL** | SPEC-0002 `:333` 要求双人复核；CLI `§3.2:76-80` 只有单个 `authorized_by`，无第二人/金额阈值/复核记录；RP-4 `:239` 无法被测。 |
| AC-62 授权单号/有效期/证据字段 | **FAIL** | SPEC-0002 `:333` 要求授权单号、操作人/授权人、时间、金额、原因、证据、有效期；设计 `ticket=RVW:<refund_id>` 是复核 id（`:76-80`），无 `valid_until`/authorization order。 |
| 审计不可抵赖 | **FAIL** | `after_sale_logs.remark` 由 CLI 写入，`operator/authorized_by` 自报；`op_id` 无唯一键（`:204`）；TCC 无本地日志，只有 admission/batch version + 日志 + 工单（`:207`）；不能证明“谁授权”。 |
| `OPS_ABANDONED` guard | **PASS（普通售后）** | `§5.4:178`：`status='FAILED_PERMANENT' AND last_error LIKE 'OPS_ABANDONED%'` 拒绝 `ensureRefundTaskTx`；RP-9。 |
| TCC abandon 不可复位 | **PARTIAL/未证明** | `ensureTccApplicationTx` 对 `FAILED_PERMANENT` 默认拒绝（代码事实），但设计未写断言、RP 无“TCC app FAILED 后普通 TCC 重试”用例；需补。 |

## 3. AC-61 replay / DLQ / lease

| 检查 | 判定 | 依据/缺口 |
|---|---|---|
| DLQ 7 天/stream 定位 | **PASS** | MALL `shop.>`、MaxAge 7 天（`:40`），DLQ subject `shop.aftersale.completed.v1.dlq`（`:105`）；`--stream-seq`/`--from-outbox`（`:72-73`）。 |
| DLQ wrapper/原样 data | **PASS** | `:106`、`:109`；`Nats-Msg-Id=ops:<op-id>:<stream_seq|event_id>`。 |
| 重复/并发/崩溃 | **PASS** | `§4.3:113-116`；`op_id` + subject/seq/event；task CAS；C4。 |
| `FAILED_PERMANENT` 拒绝 | **PASS** | `§4.1:93-100` A3/A4；`PERMANENT_REQUIRES_NEW_REFUND`；REVIEW 只走 nowb。 |
| lease 有效/过期 | **PASS** | `§4.1:97-101` A2/A5；`LEASE_HELD`；`FOR UPDATE` + status/lease（`:187`）。 |
| AC-61 四标识定位 | **FAIL** | SPEC-0002 `:321-322` 要求按 `payment_no/refund_id/after_sale_no/batch_id` 提供人工重放；CLI `§3.2:72-75` 只有 `--after-sale-no`；`refund_id/payment_no/batch_id` 不能直接定位。 |
| ACL | **FAIL** | 设计 `:67` 只给 `NATS_URL`/ops SA，无 MALL 发布 ACL、DLQ 读取 ACL、subject 级限制；RP 无 ACL 拒绝用例。 |

## 4. NOWB re-arm / abandon

| 检查 | 判定 | 依据/缺口 |
|---|---|---|
| admission/batch/lease CAS | **PASS** | `§5.2:131-156`；admission version、batch version、task `FOR UPDATE`+status/lease；C1/C3。 |
| 三版本/两库可重入 | **PASS** | admission/batch version + task lease；C1/C2；`ALREADY_*`。 |
| S6 证据验证 | **FAIL** | `s6_authorizer`/`s6_evidence_ref` 仅字符串（`:123`,`:199`）；无 Fulfillment evidence 查询/签名；可伪造。 |
| `NO_REPLAY_DRIVER`（legacy 无 task/app） | **PASS** | `§5.2:144`、`§11:247` RP-12；不猜 source/不新建行。 |
| abandon 不 release/permit/group/0 Payment | **PASS** | `§5.4:173-179`；task/app FAILED；admission REVIEW；commitment RESERVED；无 `releasePendingForAfterSaleTx`；RP-8。 |
| abandon 与 AC-62/INV-45 | **PASS（no-release）** | AC-62 `:331-332` 允许保留 RESERVED/进入授权队列；INV-45 `:141` 只禁止未授权 release；设计明确不 release。**不是把放弃误做成释放。** |
| abandon canonical/owner 书面 | **FAIL** | `last_error=OPS_ABANDONED ticket=...`（`:175`）丢失 AC-69 canonical `REVIEW_REQUIRED reason=<CODE>`（SPEC-0002 `:465`）；AC-27-NOWB `:863` 要求 SPEC-0002 owner 书面口径，设计只要求 `authorized_by`。 |
| `--replay-only` | **FAIL** | C3 `:222` 要求 `--replay-only`，CLI `nowb rearm` `:76-78` 无此 flag；C3 恢复不可执行。 |

## 5. 问题清单

### `[BLOCKER]`

- **B-1 授权/不可抵赖**：`docs/adr/0007-amd1-addendum-ops-replay.md:67,86,198`。`operator/authorized_by` 自由文本；`--execute` 非认证；无 principal/role/双人/有效期；`ticket=RVW:<refund_id>` 不是授权单号。**必须**引入 ops 身份（K8s SA + 签名 token 或 DB per-user）和 AC-62 D-24 授权字段/阈值/双人；拒绝时审计。
- **B-2 S6 证据**：`:123,199`。`s6_authorizer/s6_evidence_ref` 无 Fulfillment 校验；re-arm 可伪造。**必须**由 Fulfillment 证据查询/签名或预登记证据 ID 校验，缺失→exit 2/4，零写。
- **B-3 abandon canonical/owner**：`:175`；SPEC-0002 `:465`、SPEC-0001 `:863`。`OPS_ABANDONED` 覆盖 canonical，丢 reason；无 SPEC-0002 owner 书面口径。**必须**保留 `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>` + `ops=ABANDONED`/op_id 后缀，并把 owner 书面批准作为 `authorized_by`/审计字段。
- **B-4 `--replay-only`**：`:222` vs `:76-78`。CLI 无该 flag；C3 后无法重驱动 TCC。**必须**在 `nowb rearm` 增加 `--replay-only` 或独立 `nowb replay`，并定义退出码/幂等。
- **B-5 ACL**：`:67`。NATS MALL 发布/DLQ 读取/`cmd/ops` subject 权限未定义；DLQ replay 可能越权或跨 subject。**必须**定义 NATS ACL（`shop.aftersale.completed.v1` 发布、`.dlq` 读取、MALL stream 限制）和 DB 权限边界；RP 增 ACL 拒绝。

### `[MAJOR]`

- **M-1 AC-61 四标识**：`:72-75` 只有 `--after-sale-no`；SPEC-0002 `:321-322` 要求 `payment_no/refund_id/batch_id`。补 `--refund-id`/`--payment-no`/`--batch-id`，无法唯一定位→exit 3/4。
- **M-2 TCC abandon 不可复位未测**：`:175` 设 `refund_applications FAILED_PERMANENT`；代码 `ensureTccApplicationTx` 默认拒绝，但设计/RP 无断言；补 RP：TCC app FAILED 后普通 TCC 重试/`TccCancelMarkPaid` 拒绝且 0 Payment。
- **M-3 `op_id` 无唯一审计键**：`:204` `op_id` 是主键但 `after_sale_logs` 无唯一约束；at-least-once 审计无法唯一定位；加 `(op_id, action)` 查询/幂等判定，或说明 at-least-once 的可接受边界。
- **M-4 `OPS_ABANDONED` guard 仅 task**：`:178`；TCC app 靠 `ensureTccApplicationTx` 默认拒绝但无显式 `OPS_ABANDONED` marker/guard；补 TCC 断言，确保普通 TCC retry 不重置。

### `[TODO]`

- **T-1** `after_sale_logs.remark` 512 的 evidence_ref 截断策略：保留 op/ticket/reason，截断 evidence；审计完整 evidence 放工单/结构化日志。
- **T-2** `ops inspect` 输出字段/ACL/审计格式；RP-12 的 legacy 组级无 task/app 的 exit 3 文案与 `NO_TERMINAL_CARRIER` 需 runbook 对齐。

## 6. 结论

**BLOCKED。** `cmd/ops` 最小面、replay 幂等、CAS/lease、`NO_REPLAY_DRIVER`、abandon no-release 方向成立；但授权不可抵赖、S6 证据验证、abandon canonical/owner、`--replay-only`、ACL、AC-61 四标识是硬缺口。**0 BLOCKER 才 READY**：当前 **5 BLOCKER**，不得交 OPS-C3/实现。`AC-62/INV-45` 与 no-release **不冲突**，且**没有**把人工放弃做成释放；这是设计少数可直接保留的部分。

**只读声明**：未执行代码/测试；只审设计/Plan/SPEC 引用；未改设计/代码/SPEC/ADR；只新建本报告；未 commit。

**Actor**：`qc-ac50`
**Reviewed-by**：待 Architect 修订 / QC OPS-C2 复审
**Status**：**BLOCKED — 5 BLOCKER**
