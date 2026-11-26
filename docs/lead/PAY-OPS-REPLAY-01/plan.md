# PAY-OPS-REPLAY-01 — 人工运维规程 + DB 演练（L2，waiver C+A）

| 项 | 值 |
|---|---|
| Status | **DRAFT — pending Architect protocol + QC + human runbook approval**；未批准不执行生产/不写代码 |
| change_id | `PAY-OPS-REPLAY-01` |
| Parent | `PAY-SETTLE-01`（暂停） |
| Tier | **L2**（P0、退款/持久恢复、运维规程；人工入口 waiver） |
| Base | `d2ae89b` |
| Spec-Ref | SPEC-0001 v1.5 §16.2 `AC-27-NOWB-01`/`AC-27-LIVE-01`；SPEC-0002 v1.11 AC-61/62；INV-45 |
| 人类豁免 | 2026-11-27：工具入口豁免；人工 runbook + 双人复核 + 审计；disposable MySQL 8.4 演练 + Reviewer 后可标 AC-27 AC-VERIFIED；expiry 2026-12-31 |
| 路线 | 撤回 `cmd/ops`/`ops_action_authorizations`/`0024`/新 RPC；只交付 runbook + drill 脚本/证据 |
| allowed_paths | `docs/ops.md`（仅在既有 ops 文档内新增规程）、`scripts/drill-ops-replay.sh`（一次性演练，不接 CI/生产）、`docs/implementation/PAY-OPS-REPLAY-01-selfcheck.md`（证据） |
| forbidden_paths | `migrations/**`、`deploy/**`、`frontend/**`、`docs/specs/**`、`docs/adr/**`（实现阶段）、`docs/collab/**`、`docs/lead/**`、`internal/**`（不写业务代码）、`api/**`、accepted ADR 正文 |
| baseline_impact | AC-27/27A、AC-61/62、INV-45；不改变任何成功路径/代码；不涉及 T2/T3/T4.4/T5 |
| 回归 | L2 `PAY-AC50-REVIEW-01` T0–T8；AC-27/27A；AC-14/21/25/27/31/32 |

## 1. 交付

1. **OPS-P1 Architect protocol**：`docs/ops.md` 规程（DLQ/租约定位、重放、NOWB re-arm、NOWB abandon、双人授权、审计、失败/回滚、禁止项）。DRAFT 待 QC。
2. **OPS-P2 QC**：规程红队（授权/双人/审计/幂等/重复/越权/与 AC-62/INV-45 一致）。
3. **OPS-P3 人类批准 runbook/Plan**。
4. **OPS-P4 Developer drill**：`scripts/drill-ops-replay.sh` 在 disposable MySQL 8.4 + NATS 上演练：DLQ 重放、租约丢失重放、NOWB re-arm、NOWB abandon、重复/越权/失败；输出日志。
5. **OPS-P5 Reviewer**：核对规程与 drill，必要时将 AC-27 升 AC-VERIFIED（按 waiver C+A）。
6. **OPS-P6 交回** PAY-SETTLE-01。

## 2. Stop

- 规程要求新工具/迁移/RPC → 回人类。
- 演练出现重复退款/释放/permit 恢复 → stop。
- 未授权/无双人/无审计 → stop。
- 不得以本 L2 关闭 T2/T3/T4.4/T5 或 Release。
