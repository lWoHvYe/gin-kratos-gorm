# PAY-SETTLE-01 Remediation Tasks (Revision 1)

Status: **DRAFT — not approved**. change_id: `PAY-SETTLE-01`（原 id）。未批准前不得写业务代码。
依赖：`plan.md`；每条任务必须有 Owner、依赖、验收与证据路径。

## Phase 0 — Contract（批准前不进入 Phase 1）

- [x] **T0.1** D-1..D-5 人类/PO 裁决记录（Owner: PO + Human；依赖: 无；验收: 写入 SPEC/ADR 变更记录）—— Director approved 2026-11-24
- [ ] **T0.2** SPEC-0001 v1.4：AC-27 EARS（只禁行首 Gherkin；句内 AND/OR 允许）；登记迁移（Owner: PO；依赖: T0.1）
- [ ] **T0.3** SPEC-0002 v1.8：P-04/P-05/P-06、Σ 预检、人工释放、行/运费/责任、AC-62+（Owner: PO；依赖: T0.1）
- [ ] **T0.4** ADR-0007 Amendment 1：TCC batch/扇出、Σ 预检、permit 冻结顺序、`refund_applications`、`legacy_payment_resolution`、commitment 行/组成、消费者切换（Owner: Architect；依赖: T0.3）
- [ ] **T0.5** proto 增量字段与 RPC（Architect owner；Developer 只消费；依赖: T0.4）
- [ ] **T0.6** migrations：`refund_applications`、commitment lines、`legacy_payment_resolution`、`refund_seq`；真实迁移测试（Owner: Developer；依赖: T0.4）
- [ ] **T0.7** QC/Reviewer/PO/Architect/Human 批准 Phase 0（Owner: Lead 组织；依赖: T0.2–T0.6）

## Phase 1 — P-01 TCC 补偿 / P-03 零元回归（Owner: Developer；依赖: T0.7）

- [ ] **T1.0** P-03 回归：零元 `SettleZero`/`AdmitRefund`/0 元售后；确认不调 Payment，AC-40 证据
- [ ] **T1.1** Σ 预检：事务外 Payment 快照；Tx1 订单锁下用同一快照复核；不等不改状态，告警+人工
- [ ] **T1.2** Tx1：REFUNDING + batch；`NONE→REVOKED`，`CONSUMED` 只写 PENDING_REMOTE + commitment（不得写 REVOKED）
- [ ] **T1.3** 持久 batch/task；`REFUNDING` 不早退；崩溃重跑只补未完成组
- [ ] **T1.4** 逐组 `AdmitRefund`；全部成功后才打款；已发货组 POST
- [ ] **T1.5** `order.refund_requested` 消费者改为观察/告警；不直接 Refund
- [ ] **T1.6** `PENDING_PAYMENT+已捕获` 分支按 C-03 裁决（CAS→CANCELLED 或 PAID+写生效支付）
- [ ] **T1.7** 测试：Σ 不等、REFUNDING vs Ship 竞态、崩溃续跑、部分失败、无组 G1、已发货组

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
