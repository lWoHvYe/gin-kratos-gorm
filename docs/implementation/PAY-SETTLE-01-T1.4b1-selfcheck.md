# PAY-SETTLE-01 T1.4b1 Developer 自检清单

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Role / Actor | Developer / `dev-phase1d` |
| Plan | `docs/lead/PAY-SETTLE-01/plan.md` + `tasks.md`（Plan approved 2026-11-24） |
| 契约输入 | SPEC-0002 **v1.9**（approved 2026-11-24）；ADR-0007 Amd1（accepted）+ 零元 Addendum（accepted）+ T1.2 Rulings Addendum（accepted）+ **T1.4a Rulings Addendum（accepted）** R-B；C-04 既有 `Refund`/`GetRefundByKey` proto；C-05 `order/0017`–`0021` |
| Base | `dff267d`（T1.4a Addendum approval；`wip/pay-settle-01-phase1`） |
| Code head（本清单绑定） | **本 WIP 提交**（`wip(order): T1.4b1 tcc group refunds + payment.refunded convergence`；Base=`dff267d`） |
| Scope | **仅 T1.4b1**：TCC 逐组 `Payment.Refund` + `payment.refunded` 收敛（commitment `SETTLED`、application `DONE`、batch `DONE`） |
| 不做 | INV-44 release/回库（**T1.4b2**）；零元/Aftersales 持久 refund task（T1.4b2/T3）；`order.refund_requested` 消费者（T1.5）；`PENDING_PAYMENT+已捕获`（T1.6）；T1.7/T5 全矩阵；T2/T3/T4 |
| Status | **WIP**；不批准、不宣称实现 PASS / Release PASS，不标 AC-VERIFIED |

> 本文件是 Developer 的 canonical 自检交付物。只记录本次实现事实、证据与残余风险，不替代 Reviewer 裁决。

---

## 1. 反妥协自检清单（三问）

### 1.1 并发冲突 / 网络超时 / 部分成功时会发生什么？处理了吗？

- **准入未完成绝不打款**：`payTccBatch` 只接受 batch `ADMITTED`/`PAYING`；`FROZEN`/`ADMITTING` 返回可重试 `UNAVAILABLE`。`TccCancelMarkPaid` 先 `admitTccBatch` 后 `payTccBatch`，准入任一失败即返回错误、**零退款调用**（`TestTccPaymentIsBlockedUntilEveryGroupIsAdmitted`、`TestTccPaymentRefusesNonAdmittedBatch`）。
- **整组预检后才进入 PAYING**：`planTccPayments` 在第一次退款前校验每个组（`PROCESSING` + `PRE/POST`、`refund_id=tcc:<pay>:<group>`、金额>0、commitment 属于该组且 `RESERVED` 且金额等于组 CapCent）。任一组不满足 → 整批 fail-closed、零退款、batch 保持 `ADMITTED`（`TestTccPaymentRequiresReservedCommitment`）。
- **并发**：batch 状态用 CAS（`ADMITTED→PAYING`、`PAYING→DONE`、`PAYING→MANUAL_HOLD`）；8 并发 TCC 重试仍只有 1 batch/2 application/2 commitment，batch `PAYING`，退款按 `refund_id` 幂等重放（`TestTccAdmissionConcurrentIsIdempotent`，`-race`）。
- **网络超时/部分成功**：`Payment.Refund` 失败按下游语义分类——瞬时失败保持 `PAYING`、记录 application `attempts/last_error`、返回可重试 `UNAVAILABLE`，重试复用同一 `refund_id`（`TestTccPaymentRetryReusesRefundIDAfterTransientFailure`）；判定性拒绝（金额超过剩余 captured、intent 不可退）→ 停止后续组、`PAYING→MANUAL_HOLD`、返回 `FAILED_PRECONDITION`，自动重试不再推进（`TestTccPaymentRejectionParksBatchForManualHandling`）。**部分组已成功时不会回滚已成功退款**（ADR §4.1.3(4)）；重试只补未完成组，Payment 以 `refund_id` 保证不重复退款。
- **崩溃窗口（Refund 已受理、`payment.refunded` 未消费）**：Order 侧 commitment 仍 `RESERVED` 而 Payment `refunded` 已增加。`ADMITTED/PAYING` 的重跑不再用准入期 Σ 等式（会让重试误报 mismatch），改由 Payment 的剩余 captured 校验兜底；重试幂等重放 `refund_id`（`TestTccPaymentRetryToleratesUnprojectedRefund`）。
- **重复/乱序事件**：`payment.refunded` 先收敛账本再投影金额；累计金额已覆盖（重复/乱序）仍会 `SETTLED`+`DONE`，重复投递不改版本、不再发投影事件（`TestPaymentRefundedOutOfOrderStillSettlesLedger`、`TestPaymentRefundedSettlesCommitmentApplicationAndBatch` 的 replay 段）。
- **未知/非生效 refund_id**：非生效捕获直接忽略（不进入订单 captured/refunded、不碰 commitment）；未知 `refund_id` 不猜测组，只投影订单金额并 `Warn` 告警（`TestPaymentRefundedUnknownRefundIDProjectsOrderOnly`、`TestPaymentRefundedIgnoresNonEffectiveCapture`）。
- **人工态**：`MANUAL_HOLD/FAILED_PERMANENT` 的 batch 不被自动准入/打款推进（`tccBatchManualError`）；commitment `RELEASED`（T3 将来）不会被 `payment.refunded` 覆盖。

### 1.2 是否把状态揉进一个字段 / 加 if-else 绕过架构？

- 未新增状态字段、未新增 proto/迁移：batch 使用 ADR §6.1 既有 `ADMITTING/ADMITTED/PAYING/DONE/MANUAL_HOLD`（**PAID 语义由契约终态 `DONE` 承担**，DB CHECK 不允许新增 `PAID`）；application 使用既有 `PENDING/PROCESSING/DONE`；commitment 使用既有 `RESERVED/SETTLED`。
- 未新增第二套退款裁决：TCC 打款只消费 T1.4a 已持久化的 `order_refund_admissions` 结果与 commitment，不再调用 `AdmitRefund`、不改 permit。
- 未把 `ADMITTED` 当已退款：`ADMITTED` 只是“可进入打款”的门；只有 `Payment.Refund` 受理 + `payment.refunded` 才把 commitment 置 `SETTLED`、application `DONE`、batch `DONE`。
- 唯一“金额状态回退”保护是订单级 `refund_state`：当存在 active TCC batch（`FROZEN/ADMITTING/ADMITTED/PAYING`）时，部分退款投影保持 `REFUNDING` 而不是 `PARTIAL`；否则 T1.2/T1.3 的 resume 会以“batch 存在但订单不是 REFUNDING”拒绝剩余组（见 §2.3）。

### 1.3 向现实做了哪些妥协（重大妥协高亮）

1. **【重大】“不得部分打款”的边界**：准入门槛是严格全有或全无（任一未准入 → 零退款）。进入 `PAYING` 后，下游逐组 RPC 无法原子化；某组瞬时失败时，已受理的组保持已退、失败组保留 `RESERVED`，batch 保持 `PAYING` 等重试。**不会把部分成功报告为完成**，且 `refund_id` 幂等保证重试不重退。这是 ADR §4.1.3(4) 的实现口径。
2. **【重大】PAYING 阶段不再重跑准入期 Σ 等式**：`ADMITTED/PAYING` 的 resume 用本地账本重算组视图但不校验 `Σ == remaining` 与冻结快照（准入前已验证）；剩余额度由 Payment 的每次 `Refund` 权威校验，判定性拒绝 → `MANUAL_HOLD`。否则“Refund 已成功、事件未投影”的正常窗口会让重试误报 Σ mismatch 并被审计成 PRECHECK_FAILED（T1.2 strict 校验仍用于 `FROZEN/ADMITTING`）。
3. **T1.2 resume Σ 的排除口径收紧**：`tcc:<pay>:` 前缀只排除本批 **RESERVED** commitment；本批 **SETTLED** commitment 计入已占用（钱已退，remaining 已扣减）。否则部分退款成功后 resume 永远 Σ 不足。
4. **`payment.refunded` 保留 REFUNDING**：见 §1.2；这是跨任务交互的最小修复，避免 T1.2/T1.3 resume 被投影中态破坏（`TestPaymentRefundedKeepsRefundingWhileTccBatchPays`）。
5. **零元/Aftersales/INV-44 未做**：按人类裁决拆到 T1.4b2/T3；本任务不写持久 release 请求、不建 AfterSales refund task、不调用 Inventory。
6. **无 lazy reconciliation API 调用**：未接 `GetRefundByKey`（C-04 已有），重试依赖 Payment 幂等 + 事件消费；日终/惰性对账仍属 ADR §4.7 的其余 owner（见 §5）。

---

## 2. [修改点] / [架构侵入性] / [副作用评估]

### 2.1 [修改点]

| 文件 | 修改 |
|---|---|
| `internal/services/order/ports.go` | `paymentPort` 增加 `Refund`（既有 proto `RefundRequest`；`idempotency_key` 承载 `refund_id`）。 |
| `internal/services/order/tcc_payment.go`（新增） | T1.4b1 打款阶段：`ADMITTED→PAYING`；整组 `planTccPayments` 预检（PROCESSING+PRE/POST、`refund_id=tcc:<pay>:<group>`、`RESERVED` commitment 且金额=CapCent）；逐组 `Payment.Refund`；瞬时失败保持 PAYING/可重试，判定性拒绝 `MANUAL_HOLD`；`PAYING→DONE` 收敛；`activeTccBatchExists` 供投影使用。 |
| `internal/services/order/tcc.go` | `TccCancelMarkPaid` 在 `admitTccBatch` 成功后调用 `payTccBatch`；失败返回可重试/人工错误，不报成功。 |
| `internal/services/order/tcc_admission.go` | `PAYING` 纳入 repair 分支（只补未准入组，DONE/PROCESSING 跳过）；`markTccBatchAdmitted` 接受并发 worker 已推进到 `ADMITTED/PAYING/DONE`；PAYING 下不回退 `ADMITTED`。 |
| `internal/services/order/tcc_settlement.go` | 新增 `resumeTccRecheckTx`：`FROZEN/ADMITTING` 保留 strict Σ/快照等式；`ADMITTED/PAYING` 只重算并补缺行，额度由 Payment 权威校验；commitment 组成只排除本批 `RESERVED`（`SETTLED` 计入）。 |
| `internal/services/order/tcc_precheck.go` | `loadGroupCommittedCentFiltered` 仅排除本批 `RESERVED`；文档更新（`SETTLED` 必须计入 remaining 等式）。 |
| `internal/services/order/refund_consumer.go` | `payment.refunded`：先收敛账本（commitment `RESERVED→SETTLED`、application `PENDING/PROCESSING→DONE`、全部 DONE 后 batch `PAYING→DONE`），再单调投影订单 `refunded_cent/refund_status/refund_state`；未知 refund_id 告警不猜测；非生效忽略；重复/乱序幂等；active TCC batch 时保持 `REFUNDING`；不改 `orders.status`。 |
| `internal/services/order/{refund_admission,tcc_compensation,tcc_admission}_integration_test.go` | fake/counting/gated Payment 补 `Refund`；既有“成功后 batch=ADMITTED”断言更新为 `PAYING`（T1.4b1 后真实端到端状态）；T1.2 snapshot 变化用例改为 `FROZEN`（strict 门禁仍覆盖准入前）。 |
| `internal/services/order/tcc_payment_integration_test.go`（新增） | 7 个真实 MySQL 用例（见 §3.1）。 |
| `internal/services/order/refund_consumer_integration_test.go`（新增） | 5 个真实 MySQL 用例（见 §3.1）。 |

### 2.2 [架构侵入性]

- 新增 1 个 payment 阶段与 1 个消费收敛段；无“默认放行”分支：所有未知/不满足前置条件的组合都返回错误或保持可重试状态。
- 状态所有权不变：Order 仍唯一写 `refund_batches/refund_applications/order_refund_commitments`；Payment 仍是退款事实 owner；Order 只按事件投影。
- 未改订单 lifecycle（`orders.status`）；未发 `order.refund_requested`；未改 proto/迁移；未新增跨服务调用（只用既有 `Payment.Refund`）。
- 对 T1.2/T1.3 的既有 strict Σ 语义只在“已准入/已开始打款”的 batch 上放行，准入前行为不变。

### 2.3 [副作用评估]

- **`ADMITTED` 不再是 `TccCancelMarkPaid` 成功后的终态**：成功路径结束于 `PAYING`（退款已受理、事件待消费）或 `DONE`（事件已消费）。T1.4a 测试已按此更新；`ADMITTED` 仅表示“已准入、未打款”。
- **部分退款投影期间订单 `refund_state` 保持 `REFUNDING`**：`refunded_cent/refund_status` 仍按金额单调投影；全部组 `DONE` 后才变 `FULL`。非 TCC 退款（无 active batch）仍得到 `PARTIAL`，AC-25 既有测试不变。
- **T1.2 的 “snapshot 变化阻止 resume” 覆盖范围收窄到 `FROZEN/ADMITTING`**；`ADMITTED/PAYING` 的等价保护移到 Payment 调用（拒绝 → `MANUAL_HOLD`）。
- **`MANUAL_HOLD` 新入口**：退款被 Payment 判定性拒绝时 batch 进入人工态，自动重试不再推进；需要人工核对/修数（本任务不提供恢复命令）。
- **`refund_applications.attempts/last_error` 会被打款失败更新**（不含 version），供运维观察。
- **测试库**：`orderTestDB` 仍执行真实 `0017/0018/0021`；新表/约束无变化。

---

## 3. 测试证据（真实 MySQL；绑定本 WIP 提交，Base=`dff267d`）

环境：`go1.27.1 darwin/amd64`；MySQL `mysql-unicorn`（`127.0.0.1:3306`，`26.10.0`）；`TEST_MYSQL_DSN` 由 `scripts/test-dsn.sh` 按包派生；`GOCACHE=/tmp/dsh-go-cache`。

| 命令 | 结果 |
|---|---|
| `go build ./...` | `BUILD_OK` |
| `go vet ./internal/services/order` | `VET_OK` |
| `TEST_MYSQL_DSN=<order> go test ./internal/services/order -count=1` | `ok ... 61.086s` |
| `TEST_MYSQL_DSN=<order> go test -race ./internal/services/order -run 'TestTccPayment|TestPaymentRefunded|TestTccAdmission|TestTccCancel|TestAdmitRefund' -count=1` | `ok ... 59.075s` |
| `gofmt -l <changed files>` | 空（`internal/services/order/service.go` 的既有漂移未触碰） |

### 3.1 新增/受影响测试（逐条 PASS）

T1.4b1 打款（`tcc_payment_integration_test.go`）：

```text
--- PASS: TestTccPaymentPaysEveryAdmittedGroupOnce (0.93s)
--- PASS: TestTccPaymentIsBlockedUntilEveryGroupIsAdmitted (0.93s)
--- PASS: TestTccPaymentRetryReusesRefundIDAfterTransientFailure (0.87s)
--- PASS: TestTccPaymentRejectionParksBatchForManualHandling (1.15s)
--- PASS: TestTccPaymentRequiresReservedCommitment (0.75s)
--- PASS: TestTccPaymentRefusesNonAdmittedBatch (0.89s)
--- PASS: TestTccPaymentRetryToleratesUnprojectedRefund (1.05s)
```

`payment.refunded` 收敛（`refund_consumer_integration_test.go`）：

```text
--- PASS: TestPaymentRefundedSettlesCommitmentApplicationAndBatch (0.76s)
--- PASS: TestPaymentRefundedOutOfOrderStillSettlesLedger (0.74s)
--- PASS: TestPaymentRefundedIgnoresNonEffectiveCapture (0.74s)
--- PASS: TestPaymentRefundedUnknownRefundIDProjectsOrderOnly (0.87s)
--- PASS: TestPaymentRefundedKeepsRefundingWhileTccBatchPays (0.84s)
```

既有受影响用例（T1.4a/T1.3/T1.2 端到端状态）：

```text
--- PASS: TestTccAdmissionNonePermitResolvesPre
--- PASS: TestTccAdmissionConsumedCreatedCancelIfCreatedPre
--- PASS: TestTccAdmissionConsumedShippedPost
--- PASS: TestTccAdmissionCancelIfCreatedFailureKeepsPendingRemote
--- PASS: TestTccAdmissionConcurrentIsIdempotent
--- PASS: TestTccAdmissionLegacyG1ShippedPost
--- PASS: TestTccCancelResumeFillsMissingApplicationsAndPermits
--- PASS: TestTccCancelRefundingOrderWithoutBatchCreatesBatch
--- PASS: TestTccCancelResumeBlocksWhenPaymentSnapshotChanges
--- PASS: TestTccCancelRefundingAllocatedOrderResumesByRefundState
```

---

## 4. 【测试范围】与【未覆盖范围】

### 4.1 【测试范围】

- 全部准入后逐组打款：`refund_id=tcc:<pay>:<group>`、金额=组 CapCent、`non_effective_capture=false`；batch `ADMITTED→PAYING`；commitment 保持 `RESERVED`、application 保持 `PROCESSING` 直到事件。
- 部分准入失败：零退款、batch `ADMITTING`；重试补齐后打款。
- 重复打款幂等：同一 `refund_id` 重复调用、无重复账本行。
- 瞬时失败保持 `PAYING` 可重试；判定性拒绝 `MANUAL_HOLD` 且自动重试不推进。
- 非 `ADMITTED`（`FROZEN`）拒绝打款；缺 `RESERVED` commitment 整批 fail-closed。
- `payment.refunded`：commitment `SETTLED` + application `DONE` + 全组 DONE 后 batch `DONE`；重复/乱序幂等；非生效忽略；未知 `refund_id` 只投影订单金额并告警；部分投影保持 `REFUNDING`；lifecycle 不变。
- 真实 MySQL（C-05 真实迁移）+ `-race`（TCC 准入/取消/打款/消费定向集）。

### 4.2 【未覆盖范围】（显式不在 T1.4b1）

1. **INV-44 持久 release 请求 / Inventory 释放/回库**（R-A）→ **T1.4b2/T3.3/T3.5**。
2. **AfterSales 零元持久 refund task / 终态投影**（R-C 前置）→ **T1.4b2**。
3. `order.refund_requested` 消费者改为观察/告警 → **T1.5**。
4. `PENDING_PAYMENT+已捕获` 的 CAS→CANCELLED + 支付级取消退款 → **T1.6**。
5. `GetRefundByKey` 惰性对账、日终 worker、`AdmitRefund` 事务外退款事实查询 → ADR §4.7 其余部分（**T5.3** / 后续任务；见 §5-2）。
6. T1.7/T5 全矩阵（Σ 不等全量、Ship 竞态全矩阵、故障注入、真实迁移全量、回填、AC 证据矩阵）→ **T1.7/T5**。
7. 运维恢复命令（`MANUAL_HOLD` 的自助恢复/重放工具）与 metrics：本任务只有 `s.log` 日志。
8. `composition_known=0` 的行级退款、责任运费/168h、`order.settled` shadow、依赖 fail-fast → T3/T4。
9. 生产 MySQL 版本与生产集群验证：本任务只在 compose MySQL `26.10.0` 验证（release 门禁负责）。

---

## 5. 契约缺口 / 需裁决事项（不自行扩展）

1. **INV-44 释放/回库仍未接线**（T1.4b2）：本任务只打款，不写 `release` 请求、不调 Inventory；`order.refund_admitted` 事件字段（`group_no/warehouse_id/items`）扩展仍是 C-04 缺口，需 T1.4b2 落实。
2. **`GetRefundByKey` 惰性对账未接线**（ADR §4.7/A-07）：本任务的重试依赖 Payment `refund_id` 幂等 + 事件消费；若 `payment.refunded` 丢失（超出 outbox 假设），commitment 会停留 `RESERVED`、batch 停留 `PAYING`，需要日终 worker 或人工修复。归属待 Lead/Architect 明确（T5.3 或 T1.4b2）。
3. **PAYING 阶段的 Σ/快照复核口径**：本实现不逐笔预读 `Payment.GetIntent`，而把“remaining captured 已变化”的裁决交给 `Payment.Refund` 的权威校验（判定性拒绝 → `MANUAL_HOLD`）。这与 ADR §4.1.3(5) 的目标一致，但字面“每次打款前重新读取 Payment 权威”未逐字实现；若 Reviewer 要求逐字实现，需要显式定义“已 Refund 未投影”状态下的等式（建议随 `GetRefundByKey` 一并裁决）。
4. **PRE 组的缺失 commitment 无法自愈**：准入阶段（T1.4a）才写 PRE 组 commitment；若数据被破坏删除，`payTccBatch` 会整批 fail-closed（零退款、batch 保持 `ADMITTED`），但现有 `AdmitRefund` 幂等路径不会重建它，需要人工修数。该边界是 fail-closed，不是自动放行。
5. **`refund_applications` 的 `PROCESSING` 语义**：本任务沿用“已准入、待/正在打款”，未新增 `PAYING` 值（避免与 batch 状态重复表达）；`payment.refunded` 到达前无法从 application 区分“已受理退款”和“未受理”，对账需结合 commitment（`RESERVED`）与 Payment。若 T5 对账需要更强可观测性，需契约增量。
6. **batch `PAID`**：按 ADR §6.1 契约终态使用 `DONE`，未新增 `PAID`（DB CHECK 只允许既有 9 值）。
7. **`make fmt-check` 基线失败**：`internal/services/order/service.go` 在 base 即未 gofmt（非本任务修改）；本任务所有改动文件 `gofmt -l` 为空。

---

## 6. 结论

- 本次交付 **仅 T1.4b1**：TCC 逐组 `Payment.Refund`（`tcc:<pay>:<group>`、组 CapCent、全部准入后才打款、幂等/可重试/人工态）+ `payment.refunded` 收敛（commitment `SETTLED`、application `DONE`、batch `DONE`）。
- 未做 INV-44 release/回库、零元售后持久 task、T1.5/T1.6/T1.7；未改 order lifecycle、未发 `order.refund_requested`、未新增 proto/迁移。
- **Status: WIP**；不批准、不宣称实现 PASS / Release PASS，不标 AC-VERIFIED；交由 Reviewer 按精确 base/head 复核。
