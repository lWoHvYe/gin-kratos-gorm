# PAY-OPS-REPLAY-01 Tasks (L2)

Status: **DRAFT — pending OPS-C1..C3**；change_id `PAY-OPS-REPLAY-01`；Parent `PAY-SETTLE-01` paused；Base `160a45d`。

## Phase 0 — 设计与批准

- [ ] **OPS-C1** Architect design/impact：运维入口（优先无新 RPC）、幂等键、lease/CAS、授权/审计、DLQ 定位、恢复 owner、崩溃窗口；输出 ADR addendum/impact（Owner: Architect）
- [ ] **OPS-C2** QC 设计审查：异常分支/测试矩阵/越权；0 BLOCKER（Owner: QC）
- [ ] **OPS-C3** 人类批准设计/Plan（Owner: Human）
- [ ] **OPS-C3b** Lead 收窄 allowed_paths（设计定稿后）

## Phase 1 — 实现

- [ ] **OPS-2** NOWB 人工处置：S6 证据登记 + re-arm；无法补齐→人工放弃（AC-62 授权/审计），不释放/不恢复 permit/不关闭组（Owner: Developer）
- [ ] **OPS-3** AC-61 重放：`aftersale.completed` DLQ/租约丢失，仅重驱动未完成 task；幂等/并发/崩溃（Owner: Developer）
- [ ] **OPS-4** 测试/证据：真实 MySQL + NATS；授权拒绝/审计/重复重放/崩溃（Owner: Developer + Reviewer）

## Phase 2 — 复审/交回

- [ ] **OPS-5** Reviewer scoped verdict（NOT RELEASE）
- [ ] **OPS-6** 交回 PAY-SETTLE-01，复审 AC-27/NOWB/LIVE（Owner: Lead/Reviewer）

## Stop

- 新 RPC/迁移/新状态字段未批准 → stop。
- 重复退款/释放/permit 恢复 → stop。
- 未授权人工放弃/无审计 → stop。
