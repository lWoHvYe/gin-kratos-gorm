# PAY-OPS-REPLAY-01 Tasks (L2, manual runbook + drill)

Status: **DRAFT — pending OPS-P1..P3**；change_id `PAY-OPS-REPLAY-01`；Parent `PAY-SETTLE-01` paused；Base `d2ae89b`。
人类 waiver C+A：人工入口；disposable MySQL 8.4 演练 + Reviewer 后可 AC-27 AC-VERIFIED。

## Phase 0 — 规程与批准

- [ ] **OPS-P1** Architect 人工规程（`docs/ops.md`）：DLQ/租约定位、重放、NOWB re-arm/abandon、双人授权、审计、失败/回滚、禁止项（Owner: Architect）
- [ ] **OPS-P2** QC 规程审查：授权/双人/审计/幂等/重复/越权/AC-62/INV-45（Owner: QC）
- [ ] **OPS-P3** 人类批准 runbook/Plan（Owner: Human）
- [ ] **OPS-P3b** Lead 收窄 allowed_paths（规程定稿后）

## Phase 1 — 演练

- [ ] **OPS-P4** Developer `scripts/drill-ops-replay.sh` + 证据：DLQ 重放、租约丢失重放、NOWB re-arm、NOWB abandon、重复/越权/失败；真实 MySQL 8.4 + NATS；绑定 HEAD（Owner: Developer）
- [ ] **OPS-P5** Reviewer 核对规程/drill；按 waiver 决定 AC-27 是否 AC-VERIFIED（Owner: Reviewer）
- [ ] **OPS-P6** 交回 PAY-SETTLE-01（Owner: Lead）

## Stop

- 需新工具/迁移/RPC → 回人类。
- 重复退款/释放/permit 恢复 → stop。
- 未授权/无双人/无审计 → stop。
