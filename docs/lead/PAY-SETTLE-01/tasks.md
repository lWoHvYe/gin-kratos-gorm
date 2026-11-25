# PAY-SETTLE-01 Remediation Tasks (Revision 1)

Status: **APPROVED Plan (2026-11-24)**。本文件是 **Plan 视图**，不拥有运行状态；当前进度/冻结/PASS 以 `docs/review-tracking.md` 为准。change_id: `PAY-SETTLE-01`（原 id）。
依赖：`plan.md`；每条任务必须有 Owner、依赖、验收与证据路径。

## Phase 0 — Contract（批准前不进入 Phase 1）

- [x] **T0.1** D-1..D-5 人类/PO 裁决记录（Owner: PO + Human；依赖: 无；验收: 写入 SPEC/ADR 变更记录）—— Director approved 2026-11-24
- [x] **T0.2** SPEC-0001 v1.4：AC-27 EARS（只禁行首 Gherkin；句内 AND/OR 允许）；登记迁移（Owner: PO；依赖: T0.1）
- [x] **T0.3** SPEC-0002 v1.8：P-04/P-05/P-06、Σ 预检、人工释放、行/运费/责任、AC-62+（Owner: PO；依赖: T0.1）
- [x] **T0.4** ADR-0007 Amendment 1（accepted 2026-11-24）：TCC batch/扇出、Σ 预检、permit 冻结顺序、`refund_applications`、`legacy_payment_resolution`、commitment 行/组成、消费者切换（Owner: Architect；依赖: T0.3）
- [x] **T0.5** proto 增量字段与 RPC（C-04，QC-16 READY FOR MERGE；已合入 main）（Architect owner；Developer 只消费；依赖: T0.4）
- [x] **T0.6** migrations（C-05，QC-16 READY FOR MERGE；已合入 main）：`refund_applications`、commitment lines、`legacy_payment_resolution`、`refund_seq`；真实迁移测试（Owner: Developer；依赖: T0.4）
- [ ] **T0.7 [retro FAIL 2026-11-25]** QC/Reviewer/PO/Architect/Human 批准 Phase 0（C-01…C-05；C-04/C-05 QC-16）（Owner: Lead 组织；依赖: T0.2–T0.6）——原 `[x]` 不成立：无 Reviewer 日期化 verdict；Phase 0 exit 由追溯复核推翻。

## Phase 0-R — 追溯复核整改（2026-11-25，RETRO FAIL；Director 已接受偏差，以下为 P0 收口/Release 前置）

- [ ] **R0.1** 真实迁移 apply（fresh DB）`migrations/order/0017-0020` + `migrations/fulfillment/0010`，绑定精确 HEAD；补 `chk_switch_settled_expiry`；修复 0018 重入/部分失败；记录日志（Owner: Developer；依赖: 无）
- [ ] **R0.2** Reviewer 对 C-01..C-05 出**日期化** verdict（FAIL 2026-11-25：C-04 release RPC 缺；AC-27/INV-45..50 台账缺；见 REV-R0.2）
- [ ] **R0.3** 台账：AC-27、回归 AC-14/21/25/27/31/32、INV-45..50 的覆盖/延期决策；12 orphan 去留（Owner: Reviewer；依赖: R0.2）
- [ ] **R0.4** config fail-closed 测试（`settlement_switch_approvals` 读取）；C-04 release RPC；`group_no` 判别 CHECK；24h SLA CHECK（Owner: Developer + Architect；依赖: R0.2）
- [ ] **R0.5** evidence-matrix 刷新到 SPEC v1.10；migration scope 与 ADR 对齐（Owner: Lead + PO；依赖: R0.3）

## Phase 1 — P-01 TCC 补偿 / P-03 零元回归（Owner: Developer；依赖: T0.7）

- [ ] **T1.0** P-03 回归：Order 侧 `SettleZero`/`AdmitRefund` 已测；**AC-40 零元售后端到端 BLOCKED**（见 Phase 1a checkpoint）
- [x] **T1.1** Σ 预检 helper + 测试（未接线 `TccCancelMarkPaid`；T1.2 接线）
- [x] **T1.2** Tx1：REFUNDING + batch；`NONE→REVOKED`，`CONSUMED` 只写 PENDING_REMOTE + commitment（T1.2-R2 scoped PASS）
- [x] **T1.3** 持久 batch/task；`REFUNDING` 不早退；崩溃重跑只补未完成组
- [x] **T1.4** 逐组 `AdmitRefund`；全部成功后才打款；已发货组 POST（T1.4b1/T1.4b2a scoped PASS；T1.4a 独立 verdict = P0-3）
- [x] **T1.5** TCC bypass removed；consumer 仅保留 cancel-after-confirm，observe-only 归 T1.6/后置
- [x] **T1.6** `PENDING_PAYMENT+已捕获` 分支按 C-03 裁决（T1.6-R scoped PASS）
- [x] **T1.7** 测试：Σ 不等、REFUNDING vs Ship 竞态、崩溃续跑、部分失败、无组 G1、已发货组（test-only PASS；无 AC-VERIFIED）
- [x] **T1.2b** 按 Architect Addendum R-1/R-2/R-3 修复：
  - R-1：Σ 不等写 `refund_batches(state=PRECHECK_FAILED)` 审计，其他零写入；调整“零行”测试。
  - R-2：Tx1 外读 `ListShipments`，`exception_state != 'NONE' && shipped_at_unix>0` 判已离仓；不可用/不明确 fail-closed；T1.4 `CancelIfCreated` 返回 `ALREADY_SHIPPED`。
  - R-3：`PENDING_PAYMENT && captured>0` 在 T1.2b 只加 fail-closed guard，不建 batch；CAS→CANCELLED 归 T1.6。
  - 测试：审计 batch、exception 已离仓、PENDING 捕获 guard、其他零写入。


## Phase 2 — P-04 历史 PAID（Owner: Developer；依赖: T1.2 生产者修复 + T0.7）

- [ ] **T2.1** 先修生产者：TCC/直接路径不再产生 PAID 无生效支付
- [ ] **T2.2** 解析 `ListOrderIntents`：`SUCCEEDED` + `captured==pay_cent` + order 匹配；零元 `zero:<order_no>`
- [ ] **T2.3** 唯一候选 CAS 写回（`''`/NULL）；其他捕获走支付级退款/重复捕获
- [ ] **T2.4** 0/多候选写 `legacy_payment_resolution`（`OPEN/RESOLVING/RESOLVED/EXPIRED_REFUND/EXPIRED_ZERO/EXPIRED_FINANCE/REJECTED`，SLA 24h，Owner/恢复命令；`EXPIRED_*` 映射到订单/组终态）
- [ ] **T2.5** 回填工具 dry-run/计数/幂等；只处理 PAID；先修生产者后运行
- [ ] **T2.6** `refund_seq` 取号与并发唯一
- [ ] **T2.7** 测试：唯一/歧义/零候选/零元/回填重入/并发写回

## Phase 3 — P-05 / P-06（Owner: Developer；依赖: T0.7、T1.2）

- [ ] **T3.1** permit 不可恢复；禁止 worker 静默释放
- [ ] **T3.2** `FAILED_PERMANENT`/人工放弃：人工授权、审计、幂等、CAS/lease 的 `RESERVED→RELEASED`（只释放额度）
- [ ] **T3.3** 准入前拒绝/取消不创建 commitment；准入后失败进持久任务/人工
- [ ] **T3.4** proto/存储：行、商品/运费组成、责任、`UNDETERMINED/168h`
- [ ] **T3.5** `AdmitRefund`：行层 INV-24 + 组层 + Payment 三层；未离仓整组；已离仓部分
- [ ] **T3.6** AfterSales/缺货责任→可退运费唯一 Owner；Order 只校验
- [ ] **T3.7** 测试：行超限、责任运费、168h、未离仓整组、RELEASED 边界、AC-14/31/32 回归

## Phase 4 — Fail-closed / shadow（Owner: Developer + Architect；依赖: T1/T3）

- [ ] **T4.1** Inventory/Payment/Fulfillment 构造期必填、启动 fail-fast；删 nil 跳过
- [ ] **T4.2** 逐 `cmd/*` 依赖矩阵 + 启动测试 + 运维/回滚说明
- [ ] **T4.3** 测试替身仅 `_test.go`/build tag
- [ ] **T4.4** `order.settled` shadow 第三态（双消费、只记录、不建单）；默认仍 false
- [ ] **T4.5** `AuthorizeShipment` 唯一权威；组 `refund_state` 仅防御/展示
- [ ] **T4.6** 测试：shadow 不双建运单、flags false、启动失败矩阵

## Phase 5 — Evidence & Review（Owner: Reviewer + Developer；依赖: Phase 1–4）

- [ ] **T5.1** 矩阵补 `evidence_path`；统一 `ac-coverage.tsv`（Reviewer Owner）
- [ ] **T5.2** `-race`：AC-41/45/46/50/51/54/56 + 退款准入 vs Ship
- [ ] **T5.3** 故障注入：AC-43/47/50/51/55/56/59/61 + TCC 崩溃续跑 + 惰性对账
- [ ] **T5.4** 真实迁移测试（非内联 DDL）；AC-60 回填
- [ ] **T5.5** AC-57/58 仅引用既有测试并标 AC-REF；不重跑不得标 VERIFIED
- [ ] **T5.6** Reviewer 精确 base/head 复审（Article I–V + Governance/Workflow）
- [ ] **T5.7** integration owner 组合验证；人类决定 WIP/Release（Release 不等于本 Plan）

## Stop conditions

- 任何契约缺口 / Plan-ADR-SPEC 冲突 → 回到 Phase 0；不得代码自创。
- Σ 预检不等 → 任何状态变更前停止。
- Reviewer FAIL → 冻结 `PAY-SETTLE-01`；不得新 id/hotfix 绕过。
- AC 矩阵无 AC 编号/owner/evidence → 不得进入 Phase 5。
- 默认开关未过 Director release 门禁 → 保持 false。

## Phase 1a checkpoint (2026-11-24)

- Implemented: `internal/services/order/tcc_precheck.go` (pure precheck, 13 tests) and AC-40 `SettleZero` capture check; commit `053896b`; build/gates OK.
- **Contract gap (stop condition)**: zero after-sales end-to-end cannot be represented:
  - `internal/services/aftersales/settlement.go` rejects zero-pay orders with no captured payment;
  - `workers.go` rejects `refundCent <= 0`;
  - `after_sales.amount_state` has no `DONE(zero_amount)` value.
- No code was invented. Needs PO/Architect amendment: define where `DONE(zero_amount)` lives (recommended: Order `refund_applications(source=system_zero, amount=0, DONE)` as execution ledger; AfterSales projects `REFUNDED` with zero amount).
- T1.2 (batch/permit) paused until this gap is ruled, per Plan stop condition.

## T1.2-R（Reviewer FAIL 后）

- [ ] R-1 B1 真实 ALLOCATED/SHIPPED/COMPLETED（Developer） [superseded -> review-tracking]
- [ ] R-2 M1 historical composition（Developer + Architect） [superseded -> review-tracking]
- [ ] R-3 M2 no active groups（Developer） [superseded -> review-tracking]
- [ ] R-4 m1 resume audit（Developer） [superseded -> review-tracking]
- [ ] R-5 B2 self-check canonical（Developer） [superseded -> review-tracking]
- [x] R-6 M3 Plan 状态（Lead）
- [ ] R-7 Reviewer 复审（Reviewer） [superseded -> review-tracking]

## T1.4b1-R2 (Reviewer B2/B3)

- [ ] R2-1 persistent TCC reconcile worker: scan ADMITTED/PAYING batches, GetRefundByKey, update commitment/app/batch + order amount projection in one tx; no event required; config interval. [superseded -> review-tracking]
- [ ] R2-2 order projection: refunded_cent/refund_status/refund_state monotonic; active batch REFUNDING; all done PARTIAL/FULL. [superseded -> review-tracking]
- [ ] R2-3 no-event fault-injection test: batch DONE + order FULL/PARTIAL without payment.refunded. [superseded -> review-tracking]
- [ ] R2-4 canonical selfcheck bound to new HEAD; Plan approval rows. [superseded -> review-tracking]
- [ ] R2-5 Reviewer re-review. [superseded -> review-tracking]

## Human decisions 2026-11-25

- T1.4b2 C-04 release event：不需单独人类批准，按 Amendment 流程。
- T1.5/T1.6：若触及 `order.refund_requested` 消费者/支付级退款，先出 Amendment。
- P0 收尾后 merge：必须人类批准。
- 临时豁免：PAY-SETTLE-01 WIP 期间 ADR/SPEC Amendment 不需单独人类 Approve；QC/Architect 仍需；merge/Release/AC-VERIFIED 不豁免。

## T1.4 closure (2026-11-25)

- [x] T1.4b2a P0-R2 Reviewer PASS (60130e1..1de09e3): single-group release only.
- [x] T1.5 TCC bypass removed; consumer retained only for cancelled-after-confirm; observe-only deferred to T1.6.
- [ ] T1.6 payment-level cancel: `pay:<payment_no>:cancel`, PENDING/CANCELLED+captured, then consumer observe-only (Amendment first).
- [ ] T1.7/T5 evidence.

## T1.6-R (Reviewer FAIL)

- [x] B1: order.cancelled/compensation payload warehouse_id; Inventory multi-WH release/restock; W2 test.
- [x] B2: PENDING_PAYMENT always read Payment snapshot, independent of req.PaidCent.
- [x] B3: Inventory nil/queue failure returns error/persists; legacy net captured-refunded.
- [x] Reviewer re-review (T1.6-R PASS).

## Remaining after T1.6

- T1.7/T5 evidence matrix, real migrations, -race/fault.
- T2 historical PAID resolution.
- T3 line/freight responsibility.
- T4 fail-closed/shadow (default paid).
- Separate: inventory.confirm.requested warehouse_id (paid-confirm path).
- P0 merge only after human approve.

## P0 merge (2026-11-25)

- [x] Human approved merge; `main` merge commit `d7c2f30`.
- [ ] T1.7/T5 evidence, T2/T3/T4, AC-VERIFIED, release gates.
- [ ] Separate: inventory.confirm.requested warehouse_id.
