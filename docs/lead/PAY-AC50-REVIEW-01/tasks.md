# PAY-AC50-REVIEW-01 Tasks (L2)

Status: **DRAFT — 未批准**。Plan 视图；运行状态以 `docs/review-tracking.md` 为准。
change_id: `PAY-AC50-REVIEW-01`；Parent `PAY-SETTLE-01` 保持冻结。
依赖：`plan.md`、SPEC-0002 v1.11、ADR-0007 AC-50 Addendum 0.5。

## Phase 0 — 契约闭合（未完成不得写代码）

- [ ] **L2-C1** PO 关闭 QC R4-B1/B2/B3 + 3 MAJOR：售后无 `refund_applications` 行时 REVIEW 载体、零元不持久化例外、复核审计留存与 ADR §8.3；同步影响节（Owner: PO；证据: SPEC-0002 v1.11 修订）
- [ ] **L2-C2** Architect 修订 Addendum 0.5：审计留存、`tcc_*.go` allowed_paths、删除 0.3 残留（Owner: Architect；证据: DRAFT 0.5）
- [ ] **L2-C3** QC round 5 终审，0 BLOCKER（Owner: QC；证据: `docs/collab/qc-reports/QC-PAY-SETTLE-01-AC50-NOWB.md` R5）
- [ ] **L2-C4** 人类 `SPEC_APPROVED` / `ADR_APPROVED`（Owner: Human；证据: review-tracking 决策行）

## Phase 1 — Plan 评审

- [ ] **L2-P1** Architect 技术可行性/状态机/跨服务边界（Owner: Architect）
- [ ] **L2-P2** QC 异常分支/测试矩阵/5 崩溃窗口（Owner: QC）
- [ ] **L2-P3** Developer 可执行性/风险估算/是否拆包（Owner: Developer）
- [ ] **L2-P4** Reviewer 可验证性/证据路径/Tier 确认（Owner: Reviewer）
- [ ] **L2-P5** 人类批准 Plan（Owner: Human；未批准不得写业务代码）

## Phase 2 — 实现（每任务独占一个不变量/分支）

- [ ] **L2-T1** TCC：`REVIEW_REQUIRED` 合法裁决、`MANUAL_HOLD`、0 Payment、只阻塞本批、re-arm（Owner: Developer；依赖: L2-C4/P5）
- [ ] **L2-T2** 售后 REVIEW 持久载体 + `GetIntent` 前读裁决 + 同 `refund_id` 幂等；AfterSales 镜像 `after_sale_refund_tasks.last_error`（canonical `REVIEW_REQUIRED reason=<code> ticket=<id>`），任务保持 `PENDING`、不得 8 次后自动 `FAILED_PERMANENT`（Owner: Developer）
- [ ] **L2-T3** 部分金额三分类，禁止 `CancelIfCreated` 副作用（Owner: Developer）
- [ ] **L2-T4** 零元不写 PRE/不持久化；legacy 不猜 payment（Owner: Developer）
- [ ] **L2-T5** 审计留存 + AC-72 指标/告警 + BFF 文案；人工入口另立 `REVIEW-REQUIRED-ENTRY-01`（Owner: Developer + Architect）
- [ ] **L2-T6** Tx2 `state='PENDING_REMOTE'` CAS + 崩溃/重放/DLQ 测试（Owner: Developer）
- [ ] **L2-T7** 证据矩阵 AC-50/51/61/62/65/67 + AC-69…72（Owner: Developer + Reviewer）
- [ ] **L2-T8** 回归 AC-14/21/25/27/31/32、INV-44…50、TCC/零元/legacy（Owner: Developer + Reviewer）

## Phase 3 — 复审与交回

- [ ] **L2-R1** Reviewer 精确 base/head 复审（Owner: Reviewer；release_status=NOT RELEASE）
- [ ] **L2-R2** integration owner 组合验证（Owner: Architect/Order Owner）
- [ ] **L2-R3** 交回 `PAY-SETTLE-01`：AC-27A 第 5/6 条实现/R0.3-R1c 才能继续；Phase 0-R 仍需 T2/T3/T4.4/T5（Owner: Lead）
- [ ] **L2-R4** 人类 merge/Release 决策（Owner: Human；本 Plan 不授权）

## Stop conditions

- 契约缺口 / QC|Reviewer FAIL → 冻结 `PAY-AC50-REVIEW-01`，不得新 id/hotfix 绕过。
- TCC 非法 result、部分金额取消副作用、REVIEW 洗成 PRE、审计缺失 → stop-the-line。
- 需迁移/新字段/新 RPC/新 subject → 回 Plan + 人类批准。
- `last_error` 的 AC-72/ADR/AfterSales 三处格式不一致，或 AfterSales 对 REVIEW 自动 `FAILED_PERMANENT` → stop-the-line。
- 不得以本 L2 关闭 AC-27/Phase 0-R/P0/Release。
