# PAY-OPS-REPLAY-01 — AC-61 人工重放 / AC-27-NOWB 人工处置（L2）

| 项 | 值 |
|---|---|
| Status | **DRAFT — pending Architect design + QC + human Plan approval**；未批准不写代码 |
| change_id | `PAY-OPS-REPLAY-01` |
| Parent | `PAY-SETTLE-01`（再次暂停；本 L2 交付后 handback） |
| Tier | **L2**：P0、资金/退款、跨服务/持久恢复、运维入口；不适用 L1 |
| Base | `160a45d` |
| Spec-Ref | SPEC-0001 v1.5 §16.2 `AC-27-NOWB-01`/`AC-27-LIVE-01`；SPEC-0002 v1.11 AC-61/62（人工重放/人工放弃/授权/审计）、AC-50/69/70；INV-45 |
| ADR | ADR-0007 Rev 2.3 + 本次 ops/replay design addendum（Architect 出，待 QC/人类） |
| integration_owner | System Architect（设计）；Order/AfterSales（durable task/lease）；ops（执行/审计） |
| 目标 | 提供**真实可执行**的 ① `aftersale.completed` DLQ/租约丢失人工重放；② NOWB 人工放弃/Re-arm（授权+审计）；使 `AC-27-NOWB-01`/`AC-27-LIVE-01` 可关闭，AC-27 可复审 |
| 非目标 | 不新增退款业务规则；不改 AC-50/69/70 语义；不交付 T2/T3/T4.4/T5；不做 BFF/前端 |
| allowed_paths（候选，设计定稿后收窄） | `internal/services/order/**`（仅 replay/lease/admission 读）、`internal/services/aftersales/**`（仅任务 lease/重放）、`cmd/**` 或 `scripts/**`（运维入口）、`api/proto`（仅当设计证明必须新增字段/RPC，需人类批准）、`docs/ops.md`、对应 `_test.go` |
| forbidden_paths | `migrations/**`（默认无迁移；需要即 stop）、`deploy/**`、`frontend/**`、`docs/specs/**`、`docs/adr/**`（实现阶段）、`docs/collab/**`、`docs/lead/**`、accepted ADR 正文、Payment/Inventory 状态机 |
| baseline_impact | AC-27/AC-27A、AC-61/62、INV-45；PAY-AC50-REVIEW-01 的 REVIEW/queue/DLQ 能力；不影响 AC-50/69/70 成功语义 |
| 回归 | AC-27/27A、AC-14/21/25/27/31/32、INV-44…52；L2 `PAY-AC50-REVIEW-01` 的 T0–T8 证据 |

## 0. 范围

- **OPS-1 设计（Architect）**：优先**不新增 RPC**；评估用 durable task/lease 表 + `cmd/ops`/脚本 的运维入口。必须定义：重放幂等键、租约/CAS、授权/审计、DLQ 消息定位、失败可见性、与 `PENDING_REMOTE`/`REVIEW_REQUIRED` 的交互、重复重放/并发、崩溃窗口。
- **OPS-2 NOWB 人工处置**：① S6 证据 + re-arm（复用 L2 runbook）；② 无法补齐 → 人工放弃（AC-62 授权/审计）且不释放额度/不恢复 permit/不关闭组。
- **OPS-3 AC-61 重放**：`aftersale.completed` DLQ/租约丢失的人工重放；只重驱动未完成 task，不重复退款/不重复释放；可审计。
- **OPS-4** 测试/证据：真实 MySQL + NATS；幂等/并发/崩溃；授权拒绝/审计字段。
- **OPS-5** Reviewer scoped；交回 `PAY-SETTLE-01` 复审 AC-27。

## 1. 门禁

| ID | 内容 | Owner |
|---|---|---|
| OPS-C1 | Architect design/impact（接口、幂等、审计、恢复） | Architect |
| OPS-C2 | QC 设计审查（异常分支/测试矩阵） | QC |
| OPS-C3 | 人类批准设计/Plan | Human |
| OPS-C4 | 实现 OPS-2/OPS-3 + 测试 | Developer |
| OPS-C5 | Reviewer 精确 base/head | Reviewer |
| OPS-C6 | 交回 PAY-SETTLE-01（AC-27 复审） | Lead |

## 2. Stop

- 需要新 RPC/迁移/新状态字段 → 回设计 + 人类批准，不得默认。
- 重放造成重复退款/重复释放/恢复 permit → stop-the-line。
- 人工放弃未授权/无审计 → stop-the-line。
- 不得以本 L2 关闭 T2/T3/T4.4/T5 或 Release。
