# PAY-AC50-REVIEW-01 Tasks (L2)

Status: **APPROVED（2026-11-26，human P5）**；T0 PASS（scoped），T1–T8 pending；运行状态以 `docs/review-tracking.md` 为准。
change_id: `PAY-AC50-REVIEW-01`；Parent `PAY-SETTLE-01` 冻结；Base `a1cc809`。
环境：`BASE='root:root@tcp(127.0.0.1:3306)/mall_integration_test?parseTime=true&loc=UTC'`；`ORDER_DSN=$(scripts/test-dsn.sh "$BASE" order)`、`FF_DSN=... fulfillment`、`AS_DSN=... aftersales`；`<HEAD>` 由 T7 绑定。**T0+T0b+T0c 同一发布单元**（若分批：Fulfillment → T0c → T0b）。

## Phase 0 — 契约门

- [x] **C-1** PO 关闭 R4-B1/B2/B3 + 3 MAJOR（SPEC v1.11 approved）
- [x] **C-2** Architect Addendum 0.5（ACCEPTED）
- [x] **C-3** QC round 8 READY（0 BLOCKER）
- [x] **C-4** 人类 `SPEC_APPROVED`/`ADR_APPROVED`（2026-11-26）
- [x] **C-5** ADR errata 0.6（CAS `state='PENDING_REMOTE' AND result <> 'REVIEW_REQUIRED'`、`docs/ops.md`、proto 生成、gateway 删除、发布顺序）；QC R9 READY，待人类 errata 批准（Owner: Architect + QC + Human）

## Phase 1 — Plan 复审

- [ ] **P1** Architect re-review（P1b：R-1..R-6）
- [ ] **P2** QC re-review（P2b：W4–W10 绑定/T7-T8 证据）
- [ ] **P3** Developer re-review（P3b：命令/环境/ops 任务）
- [ ] **P4** Reviewer re-review（P4b：evidence/HEAD/tsv）
- [x] **P5** 人类批准 Plan（2026-11-26）

## Phase 2 — 实现（原子）

> T0 已 PASS（scoped，`84f5acd`）；wire token 为 `NO_WAYBILL`（ADR `<NO_WAYBILL>` 为占位记法），实施 Brief 记录。T1/T2/T3 未落地前不得单独上生产。

- [x] **T0** Fulfillment `<NO_WAYBILL>` + proto/`make proto`（F1/F1b/F2–F6/F8）
  - `TEST_MYSQL_DSN=$FF_DSN go test ./internal/services/fulfillment -run 'TestCancelIfCreated' -count=1`
  - expected：无行=`<NO_WAYBILL>`；CREATED→CANCELLED；SHIPPED/RECEIVED/EXCEPTION 原值；组隔离；无 outbox
- [x] **T0b** Order 映射 `NO_WAYBILL`→REVIEW；未知 fail-closed（O1/O7/O10）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC50ReviewMapping' -count=1`
  - expected：`result=REVIEW_REQUIRED,state=PENDING_REMOTE`；0 终态 outbox/0 release；未知→Unavailable
- [x] **T0c** Tx2 CAS 双谓词（O11/O11b/O11c/O11d、**W5/W13**）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test -race ./internal/services/order -run 'TestAC50ReviewConcurrent' -count=1`
  - expected：`WHERE refund_id=? AND state='PENDING_REMOTE' AND result <> 'REVIEW_REQUIRED'`；REVIEW 不被洗；0 行→Conflict
- [x] **T1a** TCC 准入读 REVIEW（O1b/O2b/O13b）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestTCCReviewAdmission' -count=1`
- [x] **T1b** TCC 申请单调（O1b）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestTCCReviewApplication' -count=1`
- [x] **T1c** TCC `MANUAL_HOLD`+0 打款+只阻塞本批；**Tx2 同事务写 batch/app/admission**（O1b/W12）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestTCCReviewManualHold' -count=1`
  - expected：`MANUAL_HOLD`；`refundCalls==0`；其他批不受阻；同事务失败整体回滚
- [x] **T1d** TCC 自动续跑 token（W12）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestTCCReviewResumeToken' -count=1`
- [x] **T1e** TCC re-arm（O16）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestTCCReviewRearm' -count=1`
- [x] **T2a** existing-first 幂等 + typed details（O2/O2b/O2c/**W6**）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC50ReviewReplay' -count=1`
  - expected：同错；`cancelCalls/getCalls` 不增；`refundCalls==0`；details 带结构化 `result/reason/ticket`
- [x] **T2b** 售后镜像 canonical + typed details（O19/O23）
  - `TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run 'TestReviewTaskMirror' -count=1`
  - expected：`after_sale_refund_tasks.last_error` 恰为 `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>`；`refund_applications` 0 行；无字符串嗅探
- [x] **T2c** 售后任务态（O22/O22b）
  - `TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run 'TestReviewTaskRetry' -count=1`
  - expected：REVIEW 保持 PENDING；非 REVIEW 8 次 FAILED_PERMANENT；无自动 DONE/释放
- [ ] **T3** 部分金额三分类（O4a–O4e/O5c/O10b/**W10**）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC50ReviewPartial' -count=1`
  - expected：未离仓→PARTIAL 零写入；已离仓→POST；不可判定→REVIEW+1 RESERVED；`cancelCalls==0`；`refundCalls==0`
- [ ] **T4a** 零元（O7/O7b/O8/O21/**W7**）
  - `TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run 'TestZero.*Review' -count=1`
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestZeroReview' -count=1`
  - expected：0 admission/S19/REVIEW；`refundCalls==0 && getCalls==0`；真实取消后 PRE+DONE；不计入 REVIEW backlog（单列），仍进入零元人工队列
- [ ] **T4b** legacy（O10/O10b/**W10**）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC50ReviewLegacy' -count=1`
- [ ] **T5a** 审计留存（O20/O20b/**W6/W11**）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC50ReviewAudit' -count=1`
- [ ] **T5b** 指标/SLA（O18）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/platform/metrics ./internal/services/order -run 'TestAC50ReviewMetrics|TestRefundReviewGauge' -count=1`
- [ ] **T5c** ops runbook：AC-72 查询/SLA/对账、§8.3 re-arm、存量 PRE 审计写入 `docs/ops.md`（owner ops；expiry 2026-12-31；证据=runbook 链接 + 演练记录；人工审阅）
- [ ] **T6a** Tx1/Tx2 崩溃/竞态（O3/**W1–W4**）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test -race ./internal/services/order -run 'TestAC50ReviewCrash|TestAC50ReviewRace' -count=1`
- [ ] **T6b** 重复/乱序/迟到 placeholder（O14/O14b/**W8/W9/W13/F7b**）
  - `TEST_MYSQL_DSN=$FF_DSN TEST_NATS_URL=nats://127.0.0.1:4222 go test -race ./internal/services/fulfillment -run 'TestRefundAdmittedConsumerReviewAndDuplicate' -count=1`
- [ ] **T6c** DLQ/ACL 重放（F7/O14/**W8/W11**；W8 永久悬挂归 AC-27-LIVE-01，本 L2 只验证不重复）
  - `TEST_MYSQL_DSN=$FF_DSN TEST_NATS_URL=nats://127.0.0.1:4222 go test ./internal/services/fulfillment -run 'TestRefundAdmittedConsumerReviewAndDuplicate' -count=1`
- [ ] **T6d** re-arm 审计先于清（O16/O20/**W11/W13**）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC50ReviewRearm' -count=1`
- [ ] **T7** 证据矩阵：修订 AC + AC-69…72（AC-72 三段）+ INV-51/52；Reviewer 写 tsv；`NOT RELEASE`；绑定 `<HEAD>`（Owner: Developer + Reviewer）
  - 证据：`docs/collab/ac-coverage.tsv` + `docs/implementation/PAY-AC50-REVIEW-01-selfcheck.md`；`git rev-parse HEAD`
- [ ] **T8** 回归：AC-14/21/25/27/31/32、INV-44…50、TCC/零元/legacy；不改父状态（Owner: Developer + Reviewer）
  - `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC25|TestAC27|TestT17' -count=1`
  - `TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run 'TestAC14|TestAC31|TestAC32|TestZero' -count=1`
  - `TEST_MYSQL_DSN=$FF_DSN go test ./internal/services/fulfillment -run 'TestAC21' -count=1`

- [x] **T3-R1** fresh Tx1 与 Tx2 共用**唯一** REVIEW 持久化入口，应用 T1c 门禁（batch/app `RowsAffected`）；不得提交无人工持有的 REVIEW（Owner: Developer；证据: `source=tcc` partial/legacy + O4d/O10 + batch MANUAL_HOLD/app 镜像/last_error/token）
- [ ] **T3-R2** 补 `source=tcc` partial/legacy 用例（O4d/O10、W12）；F7b/W8 残余在 T7 明示（Owner: Developer + Reviewer）

## Phase 3 — 复审/交回

- [ ] **R1** Reviewer 精确 base/head 复审（NOT RELEASE）
- [ ] **R2** integration owner 组合验证
- [ ] **R3** 交回 PAY-SETTLE-01（AC-27A 5/6、R0.3-R1c、Phase 0-R）
- [ ] **R4** 人类 merge/Release 决策

## DoD

- Diff、HEAD、真实日志（MySQL 8.4/NATS/`-race`/故障注入）、`【测试范围】/【未覆盖范围】`、反妥协自检。
- `git diff --stat -- migrations` 为空；0016/0017 字节不变；无新 RPC/字段/worker。
- `ac-coverage.tsv` 仅 Reviewer 写；AC-72 三段；`NOT RELEASE`；不关 AC-27/Phase 0-R。

## Stop

- C-5 未关、QC/Reviewer FAIL、契约缺口 → 冻结。
- CAS 非 `state='PENDING_REMOTE' AND result <> 'REVIEW_REQUIRED'`、REVIEW 洗成 PRE、部分金额取消副作用、审计被清、TCC 非法 result、`NO_WAYBILL` 未区分 → stop-the-line。
- 需迁移/新字段/新 RPC/新 subject/新 worker → 回 Plan + 人类批准。
