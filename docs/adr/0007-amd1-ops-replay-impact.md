# PAY-OPS-REPLAY-01 — 运维重放/ NOWB 人工处置 影响分析（DRAFT 0.2）

| 项 | 值 |
|---|---|
| Status | **DRAFT 0.2 — pending QC round 2（OPS-C2）**；不构成实现依据 |
| Change ID | `PAY-OPS-REPLAY-01`（Parent `PAY-SETTLE-01` 暂停） |
| Base | `160a45d`（Plan）；契约基线 SPEC-0001 v1.5 §16.2、SPEC-0002 v1.11 AC-61/62、ADR-0007 AC-50 0.6 ACCEPTED |
| 设计输入 | `docs/lead/PAY-OPS-REPLAY-01/plan.md`（`:20-38`）、`tasks.md`（`:5-27`）；`docs/ops.md:829-936`；QC `QC-PAY-OPS-REPLAY-01-DESIGN.md`（BLOCKED 5B/4M） |
| 推荐 | **`cmd/ops` + 现有 identity JWT + 1 张 additive 授权表**；**零新 RPC**；`[CONTRACT-IMPACT]` 迁移待人类批准 |
| Actor | `arch-ops-replay`（System Architect） |
| Reviewed-by | 待 QC round 2 |

## 1. 结论

- **`cmd/ops` 仍推荐**：`replay dlq|task`、`nowb rearm|abandon`、`inspect`；只读默认，`--execute` 才写；不监听端口。
- **授权模型修正（B-1）**：消费现有 identity JWT（HS256，`admin` 角色；operator/authorizer/reviewer 三个不同 `sub`；金额 ≥100000 cent 必须 reviewer），并写 **`ops_action_authorizations`**（Order DB，additive `0024`）记录 authorization_id/op_id/金额/复核/有效期/JTI；`ticket=RVW` 只是复核 id。**零迁移假设不成立**，标 `[CONTRACT-IMPACT]`，需 OPS-C3 人类批准；未批准不得实现。
- **S6 证据（B-2）**：`s6_evidence_ref` 必须匹配 Fulfillment 只读库的 `shipments.status='CANCELLED'` 或 `shipment_events.status='CANCELLED'`（order/shipment 匹配）；查不到→拒绝；禁止自由文本。Fulfillment admin JWT 的 sub 记入授权表。
- **abandon（B-3）**：不改写 canonical `last_error`；task 置 `FAILED_PERMANENT`，审计写 `after_sale_logs` `OPS_NOWB_ABANDON` + 授权表；`ensureRefundTaskTx` 查最新 log guard。TCC app `FAILED_PERMANENT` 后普通 TCC retry 拒绝（`TCC_APP_ABANDONED`）。
- **`--replay-only`（B-4）**：`nowb rearm` 支持；只对 ADMITTING TCC 批调用既有 `TccCancelMarkPaid`；无 gRPC exit 6。
- **ACL（B-5）**：ops NATS 账号仅 publish `shop.aftersale.completed.v1`、read `.dlq`、`$JS.API.STREAM.INFO/MSG.GET.MALL`；DB 三 DSN 分权（Fulfillment 只读）；越权 exit 2/5。
- **AC-61 四标识**：`--after-sale-no/--refund-id/--payment-no/--batch-id` 选择器，解析不唯一→exit 3/4。
- **op_id 唯一**：授权表 `UNIQUE(op_id)`；`after_sale_logs` 仅补充。
- **TCC abandon 不可复位**：RP-9b 断言；`nowb rearm`/`TccCancelMarkPaid` 拒绝。
- **AC-62 no-release** 仍按 Lead 指令；D-24 release 独立，不宣称已实现。

## 2. 替代方案对比

| 方案 | 优点 | 缺点/风险 | 结论 |
|---|---|---|---|
| **`cmd/ops` + identity JWT + 授权表** | 真实 principal/角色/双人/授权单/有效期；无新 RPC；可测 | 需 1 个 additive migration + 三 DSN；授权表须人类批准 | **推荐** |
| `cmd/ops` + 仅 JWT、无授权表 | 零迁移 | 无法持久证明授权单/金额复核/有效期；B-1 不闭合 | 拒绝 |
| shell/SQL 脚本 | 最快 | 无 CAS/租约/DLQ/审计/认证；不可测 | 仅 DBA 后备 |
| 新 admin RPC + BFF | 鉴权集中 | 新 proto/RPC/mesh/BFF/兼容；需人类批准；面更大 | 备选（若禁止 DB 凭证） |
| 复用 `ResolveLegacyResolution`/`ReplayDLQ` | 已有 RPC | 前者域不符且未实现；后者 seckill 专用 | 不可行 |
| 复用 `TccCancelMarkPaid`（既有） | TCC 续跑入口 | 依赖 Payment snapshot；不能复位 REVIEW/DLQ | 仅作 TCC re-arm 后重驱动 |

## 3. 影响面

| 对象 | 变更 | 说明 |
|---|---|---|
| `cmd/ops/**` | **新增** | CLI；JWT 校验、DLQ 解析、CAS/lease、授权记录；`_test.go` |
| `migrations/order/0024_ops_action_authorizations.sql` | **新增（1 个）** | additive 授权表；`[CONTRACT-IMPACT]`；无回填/无 down；人类批准前不实现 |
| `internal/services/aftersales/workers.go` | **小改** | `ensureRefundTaskTx` 查最新 `after_sale_logs` guard；不写 `OPS_ABANDONED` 到 `last_error` |
| `docs/ops.md` | 更新 | 命令、授权单字段、abandon no-release、DLQ/outbox/inspect |
| `after_sale_logs`/`after_sale_refund_tasks` | 复用 | canonical 保留；`FAILED_PERMANENT` + action 审计 |
| `order_refund_admissions`/`refund_batches`/`refund_applications` | 复用 | 只 CAS；不新增列 |
| NATS MALL/DLQ | 复用 | 不新建 stream；ACL 收紧；7 天 |
| proto/API | **无变更** | 无新 RPC/字段；`api/gen` 不手改 |
| Fulfillment DB | **只读** | `OPS_FULFILLMENT_DSN`；仅 `shipments`/`shipment_events` 证据查询 |

**代码面**：`cmd/ops` 新增 + `workers.go` guard + 1 个迁移。若人类拒绝迁移，B-1 无合规替代；必须改走 admin RPC + 授权记录（更大面，需人类批准），不得退回自由文本。

## 4. 风险与缓解

| # | 风险 | 影响 | 缓解 |
|---|---|---|---|
| R-1 | JWT 泄露/角色过大 | 越权操作 | `admin` 角色 + 三个不同 sub + exp；授权单/金额/复核；JTI 记录；ops Secret 短 TTL |
| R-2 | 授权表迁移未批准/漂移 | 无法实现/审计缺口 | `[CONTRACT-IMPACT]`；OPS-C3 人类批准；additive/checksum；CLI/测试同步 |
| R-3 | S6 evidence 伪造 | 错误 re-arm | 只认 Fulfillment `shipments`/`shipment_events` 的 CANCELLED 行；order/shipment 匹配；查不到 exit 2/4 |
| R-4 | abandon 覆盖 canonical | AC-69 破坏 | `last_error` 保持 canonical；abandon 只写 task status + logs/授权表 |
| R-5 | `Refund` 绕过 abandon | 重复重试 | `ensureRefundTaskTx` 查最新 `OPS_NOWB_ABANDON` log；RP-9 |
| R-6 | TCC app 被 TCC retry 复位 | 重新打款风险 | app FAILED + `TCC_APP_ABANDONED`；RP-9b；batch MANUAL_HOLD |
| R-7 | ACL 过宽 | 跨 subject 发布/读 | 仅 completed publish + `.dlq` read + JS API；RP-4c |
| R-8 | 两库无分布式事务 | 部分执行 | 两阶段可重入；Order 先、task 后；C1/C3；授权行 `op_id` 唯一 |
| R-9 | AC-61 永久失败同 task | 已释放额度再退款 | 拒绝同 task；新 `refund_id`/AC-62；RP-11 |
| R-10 | `after_sale_logs.remark` 512 | evidence 截断 | 保留 op/auth/ticket/reason，截 evidence；完整证据在授权表/日志/工单 |
| R-11 | `ops inspect` 泄露 | 安全 | admin JWT + 只读 + 不打印 token/secret |

## 5. 假设与不确定性

1. **[假设]** ops Job 可获 Order/AfterSales 读写与 Fulfillment 只读 DSN、NATS ACL；生产 Secret/RBAC，不写代码。
2. **[假设]** identity JWT 的 `admin` 角色与有效期可被 `internal/platform/auth` 校验；三个 distinct sub 可满足 D-24 双人。
3. **[假设]** Fulfillment `shipments`/`shipment_events` 保留可核验的 `CANCELLED` 证据；无行/无事件时拒绝（`[不确定]` 历史丢失场景需 PO/ops 确认）。
4. **[假设]** MALL stream 7 天（`mq.go:51`）；DLQ wrapper 稳定（`consumer.go:157-170`）。
5. **[不确定]** AC-62 abandon 是否必须 D-24 release；按 Lead no-release；若 PO 不同，回 SPEC/人类。
6. **[假设]** 普通售后/legacy 有 task、TCC 有 app；无 task/app 的 legacy 拒绝。
7. **[范围]** AC-61 的 `FAILED_PERMANENT` 同 task 重放不在本 L2（已释放额度）；新 `refund_id`/AC-62。
8. **[假设]** 授权表新增是唯一合规路径；若人类拒绝，改 admin RPC + 授权记录，重新估算。

## 6. 若必须新 RPC/字段/迁移（已发生）

- **已发生**：1 个 additive 授权表（`0024`）。路径：Architect DDL/影响 → OPS-C2 QC → **人工 OPS-C3 + 迁移批准**（Lead 收窄 allowed）→ 实现；不得由 Developer 自加。
- 若改 admin RPC：新 proto RPC/字段 + BFF/mesh 鉴权 + 人类 `ADR_APPROVED`；面更大，非首选。
- 当前设计：**0 新 RPC；1 新表；0 回填；0 down migration**。

## 7. 验证/交付（OPS-4）

- 真实 MySQL 8.4 + NATS：RP-1..RP-13（ADR §11），含 JWT 角色/过期/双人、S6 伪造、ACL、`--replay-only`、TCC abandon 不可复位、四标识定位；`-race` 并发/崩溃。
- 证据：CLI 日志/退出码、授权表行、`after_sale_logs`、admission/batch version、DLQ seq；**能力必须是可执行 `cmd/ops`，runbook 不算**。
- Reviewer scoped；AC-27-NOWB/LIVE 仍 AC-REF；不声称 Release。

## 8. 建议

先由人类批准 **1 个授权表迁移**；实现 `cmd/ops` + JWT/S6/ACL + `workers.go` guard；RP-1..RP-13 全过；OPS-C2 复审 0 BLOCKER 后 OPS-3/4；AC-62 no-release 口径由 SPEC-0002 owner 书面确认。

**Actor**：`arch-ops-replay`（System Architect）
**Status**：**DRAFT 0.2 — pending QC round 2**
