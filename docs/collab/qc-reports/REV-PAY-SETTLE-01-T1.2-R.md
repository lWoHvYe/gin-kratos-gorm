# REV-PAY-SETTLE-01-T1.2-R — T1.2-R 复审（B1/B2/M1/M2/M3/m1）

| 项 | 值 |
|---|---|
| review_scope | **实现复审**：T1.2-R 对 `REV-PAY-SETTLE-01-T1.2` 的 B1/B2/M1/M2/M3/m1 修复；代码范围 `a3f080f..e7e29e5`（T1.2-R 代码 head），docs head `face7a6`（canonical selfcheck）。对照 SPEC-0002 v1.9 AC-51/AC-65、INV-48/INV-45/INV-36/INV-06、ADR-0007 Amd1 + 零元 Addendum + T1.2 Contract Rulings Addendum（R-1/R-2/R-3）、`docs/lead/PAY-SETTLE-01/t1.2-r-plan.md`。**不审查** T1.4/T1.5/T1.6/T1.7/T2/T3/T5 实现；不构成 Spec 全量验收、不构成 Release PASS。 |
| base | `a3f080f`（T1.2-R Plan approved；代码基线为 `4e5d07f`） |
| head | `e7e29e5`（`wip(order): T1.2-R lifecycle gate, legacy composition and no-active refusals`） |
| docs head | `face7a6`（selfcheck `docs/implementation/PAY-SETTLE-01-T1.2-selfcheck.md`） |
| spec_version | SPEC-0002 **v1.9** |
| adr_revision | ADR-0007 Amendment 1（accepted）+ 零元 Addendum（accepted）+ T1.2 Contract Rulings Addendum（accepted，R-1/R-2/R-3） |
| change_id | `PAY-SETTLE-01` |
| integration_owner | **Director**（`docs/lead/PAY-SETTLE-01/plan.md`） |
| INV | INV-06、INV-24、INV-36、INV-44、INV-45、INV-48、INV-50（本范围主要 INV-06/36/48/45） |
| affected_AC | AC-51、AC-65（主）；AC-62/AC-64/AC-66/AC-67/AC-68 仅边界；**不宣称任何 AC-VERIFIED** |
| Actor / Reviewed-by | `reviewer-phase1`（Reviewer）；代码作者 `dev-phase1b`（Developer），不同 actor |
| **verdict** | **PASS（有范围）** |
| release_status | **NOT RELEASE**（实现级 PASS；不批准合并，不构成 Release PASS） |

> 本报告只裁决 T1.2-R 的修复范围。`PASS` 表示：在本报告 `review_scope` 内，前次 B1/B2/M1/M2/M3/m1 已修复且测试证据真实；**不**表示 T1.2 已可合入 main、**不**表示 T1.4/T1.6 已完成、**不**表示 Release PASS。merge/Release 仍由人类按 T5.7 决定。

---

## 1. 结论

**PASS（有范围）。**

前次报告 `REV-PAY-SETTLE-01-T1.2` 的 6 个问题已修复并可复核：

- **B1**：`TccCancelMarkPaid` 不再对 `ALLOCATED/SHIPPED/COMPLETED` 静默返回；入口与 Tx1 都使用显式封闭状态集；`CANCELLED+paid>0` fail-closed（`TCC_CANCELLED_CAPTURED_NOT_IMPLEMENTED`）；未知状态 fail-closed；`MANUAL_HOLD/FAILED_PERMANENT` 移出 resumable。真实 `ALLOCATED`（G1 SHIPPED + G2 未离仓）、`SHIPPED`、`COMPLETED` 测试通过，订单生命周期不被改写。
- **B2**：canonical selfcheck 已落 `docs/implementation/PAY-SETTLE-01-T1.2-selfcheck.md`：三问、`【测试范围】/【未覆盖范围】`、返工影响声明、绑定 `e7e29e5`；`/tmp/t12r-logs/*` 真实存在且与自述一致。
- **M1**：迁移后 `goods=amount/freight=0/known=0` 无行的历史组可按整组退款；组上限用 `CapCent = groupPaid + freight_share - RESERVED/SETTLED`，不是旧 commitment amount；新 TCC commitment 写 `composition_known=0`/无行；旧行不被改写；纯 `known=1` 行合计错误仍 fail-closed。
- **M2**：Σ 校验先于 no-active；`Σ=remaining=0` 幂等成功、不写 `REFUNDING`/空 batch；`active=0 && remaining>0` 写 `PRECHECK_FAILED` 审计。
- **M3**：Plan 已批准（`a3f080f`），`plan.md`/`tasks.md` 回写 APPROVED/FROZEN，`integration_owner=Director`，`review-tracking.md` 记录授权。
- **m1**：resume Σ/快照变化现在写 `PRECHECK_FAILED` 审计（attempt_seq+1、sum/remaining 正确），订单/permit/commitment/application 零写入。
- **m2/m6**：`MANUAL_HOLD/FAILED_PERMANENT` 不自动 resume；order 测试改为执行真实 `migrations/order/0017`+`0018`，`TestOrderTestSchemaUsesC05RefundMigrations` 校验表/列/CHECK。

**无新增 BLOCKER。** 但仍发现 1 个 **MAJOR（非资金、非 silent-skip）** 和若干 MINOR，建议在 T1.4/T1.7/Release 前关闭（见 §4）。它们不改变本范围的 PASS，但 Release 前不能忽略。

---

## 2. BLOCKER 表

| ID | 级别 | 结论 |
|---|---|---|
| — | — | **无新增 BLOCKER。** 前次 B1/B2 两个 BLOCKER 均已关闭；未发现资金静默不退款、错误打款、越界改 T1.4/T1.6/consumer/proto/migrations。 |

> 说明：本报告把“非资金、非静默跳过、待 T1.4/T1.7 收口的边界”列为 MAJOR/MINOR，不升级为 BLOCKER；若人类要求“零 MAJOR 才可 merge”，则应先关闭 §4 的 G1。

---

## 3. 前次问题逐项复核

| 前次 ID | 复核结果 | 证据 |
|---|---|---|
| **B1** 真实 ALLOCATED/SHIPPED/COMPLETED 静默成功 | **已修复（组投影场景）**：入口允许 `PENDING_PAYMENT/PAID/ALLOCATED/SHIPPED/COMPLETED`；`CANCELLED+paid>0` 返回 `TCC_CANCELLED_CAPTURED_NOT_IMPLEMENTED`；未知 `CLOSED` 返回 `TCC_ORDER_STATUS_UNSUPPORTED`；Tx1 同样封闭并 fail-closed。`tccCancelDecision` 对 `PAID/ALLOCATED/SHIPPED/COMPLETED` 返回 refund，`paid<=0` keep-paid。 | `internal/services/order/tcc.go:202-218`、`:391-410`；`internal/services/order/tcc_settlement.go:275-290`。测试：`TestTccCancelAllocatedOrderMixedGroups`、`TestTccCancelShippedLifecycle`、`TestTccCancelCompletedLifecycle`、`TestTccCancelCancelledCapturedFailsClosed`、`TestTccCancelCancelledWithoutCapturedIsIdempotent`、`TestTccCancelUnsupportedStatusFailsClosed`、`TestTccCancelManualHoldBatchIsNotResumed`、`TestTccCancelRefundingAllocatedOrderResumesByRefundState`（`tcc_compensation_integration_test.go` 新增段）均 PASS。 |
| **B2** 缺 canonical 自检清单/测试范围 | **已修复**：`docs/implementation/PAY-SETTLE-01-T1.2-selfcheck.md` 有三问（并发/超时、if-else/状态、妥协清单）、`【测试范围】/【未覆盖范围】`、返工影响、绑定 code head `e7e29e5`；`/tmp/t12r-logs/{build-vet,order-full,tcc-verbose,tcc-race,tcc-race-repeat}.log` 真实存在，内容与自述一致。 | selfcheck §1–§5；`/tmp/t12r-logs` 实际读取；`face7a6` commit `Role: Developer/change_id/Actor`。 |
| **M1** 历史 composition 阻断整单 | **已修复（纯 legacy / 纯 known=1）**：`loadTccGroupComposition` 读 `composition_known`；`known=0` 标 `UnknownComposition`，只用 `CommittedAmount`，不校验行；`CapCent` 公式使用 precheck 的 `group.CapCent`；`ensureTccCommitmentTx` 对 unknown 写 `goods=amount/freight=0/known=0`、无行；纯 `known=1` 仍校验头/行。 | `tcc_settlement.go:601-671`、`:827-900`、`:1002-1130`；测试 `TestTccCancelMigratedLegacyCompositionAllowsWholeGroupRefund`（app=9900/9900/0，commitment=9900/known=0/0 行，旧行 100/100/known=0 未变）、`TestTccCancelKnownCompositionLineMismatchFailsClosed` 均 PASS。 |
| **M2** 无活动组仍建空 batch | **已修复**：`createTccCompensationTx` 在 Σ 校验后、NoCaptured 分支前判断 `sum==0 && remaining==0` → 幂等成功；`resume` 同条件且 `active==0` → nil。`active=0 && remaining>0` 仍由 Σ mismatch 写审计。 | `tcc_settlement.go:317-331`、`:366-389`；测试 `TestTccCancelNoActiveGroupsIsIdempotentSuccess`（无 REFUNDING/batch/app/outbox/version）、`TestTccCancelNoActiveButRemainingCapturedWritesAudit`（1 条 PRECHECK_FAILED、无 FROZEN）、`TestTccCancelFullRefundStateIsIdempotent` 均 PASS。 |
| **M3** Plan 状态/索引 | **已修复**：`plan.md`/`tasks.md` 为 APPROVED + T1.2 FROZEN pending T1.2-R；`integration_owner=Director`；`review-tracking.md` 记录 Director 批准；T1.2-R Plan §6 Human APPROVED，QC-19/20 与 Architect B-1/B-2 已落。**残余文档漂移**：T1.2-R Plan §6 的 Architect/QC/Reviewer 行仍 TBD（MINOR，不阻断）。 | `docs/lead/PAY-SETTLE-01/plan.md:5-13`、`tasks.md:3`、`t1.2-r-plan.md`、`docs/review-tracking.md:20-26`。 |
| **m1** resume 无审计 | **已修复**：`resumeTccCompensationTx` 对 `rechecked != nil`/batch 快照不等返回 `tccSumMismatch`；`applyTccCompensation` 在 Tx1 回滚后写 `PRECHECK_FAILED`。 | `tcc_settlement.go:306-332`、`:359-379`；`TestTccCancelResumeBlocksWhenPaymentSnapshotChanges` 现断言审计 attempt=2、sum=10000、remaining=9900、无 commitment/admission 写入。 |
| **m2** MANUAL_HOLD 自动 resume | **已修复**：resumable 仅 `FROZEN/ADMITTING/ADMITTED/PAYING`；`MANUAL_HOLD/FAILED_PERMANENT` 返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`。 | `tcc_settlement.go:505-517`；`TestTccCancelManualHoldBatchIsNotResumed/{MANUAL_HOLD,FAILED_PERMANENT}` PASS（零 application/version 不变）。 |
| **m6** 内联 DDL 代替迁移 | **已修复（refund schema）**：`orderTestDB` 删除内联 `refund_batches/refund_applications/commitment-lines`，改为执行真实 `migrations/order/0017`+`0018`；`TestOrderTestSchemaUsesC05RefundMigrations` 校验表/列/CHECK。其余表仍内联（T5.4 负责完整迁移/回填，未变）。 | `settlement_integration_test.go:48-56`、`:116-133`、`:172-223`；测试 PASS。 |

---

## 4. 新发现（MAJOR / MINOR）

| ID | 级别 | 位置 | 事实 / 契约 | 处置建议 |
|---|---|---|---|---|
| **G1** | **MAJOR（非资金，Release 前关闭）** | `internal/services/order/tcc_precheck.go:242-245`（G1 fallback 构造 `tccGroupCap` 时 **不设 `GroupStatus`**）；`internal/services/order/tcc_settlement.go:224-226`（`GroupStatus == ""` 直接返回“未离仓”）；`tcc.go:210-213` 允许 `SHIPPED/COMPLETED` | **历史无履约组**的 `SHIPPED/COMPLETED` 订单走 G1 fallback 时，G1 的 `GroupStatus` 为空；若 `ListShipments` 没有可用 shipment facts（历史/不完整），`shipped()` 走 `case ""` 返回 false → 该组被判“未离仓”，写 `PRE_SHIP_REVOKED` 并初始化/撤销 G1 permit，而 AC-51 要求已离仓组 `POST_SHIP_ALLOWED`；Addendum R-2 也要求“无 shipment 但 Order 组状态非 PENDING/ALLOCATED → fail-closed”。现有 `TestTccCancelLegacyOrderInitializesG1Permit` 只覆盖 `PAID`，`TestTccCancelShippedLifecycle`/`Completed` 都有真实 group 行；**no-group + SHIPPED/COMPLETED 无测试**。退款金额仍按 CapCent，不会多退/少退，但 admission 结果/许可写错，属 AC-51/AC-65 historical G1 分支的契约偏离。 | 在 G1 fallback 用 `orders.status` 推导 synthetic group status（`SHIPPED→SHIPPED`，`COMPLETED→RECEIVED`），或在“order 已 SHIPPED/COMPLETED 且无 shipment fact”时 fail-closed；补 `TestTccCancelLegacyShippedNoGroups`、`...CompletedNoGroups`（断言 POST、permit 不撤、生命周期不变）。建议 T1.7/T1.4 前完成。 |
| **g1** | MINOR | `internal/services/order/tcc_settlement.go:1113-1118`（`if comp.UnknownComposition { continue }`）；`:1083` 行查询仅 `composition_known=1` | **混合组**（同组既有 `known=0` legacy、又有 `known=1` commitment）会把 `UnknownComposition=true`，最终循环 `continue`，跳过该组所有 `known=1` 行合计校验；纯 `known=1` 的 `TestTccCancelKnownCompositionLineMismatchFailsClosed` 通过，但混合组未被拒绝。由于该组只能整组退款、金额仍来自 commitment headers，**不造成多退/少退**；但 M1 的“known=1 行校验”在混合组上有缺口。 | 行校验按 **known=1 行**独立执行（不要因同组存在 known=0 而整体跳过）；补混合组 known=1 坏行用例。可归 T3.4/T1.7。 |
| **g2** | MINOR | `internal/services/order/tcc.go:294-298`（`loadTccShipmentFacts` 在 `applyTccCompensation` 前）；`tcc_settlement.go:327-331`（M2 shortcut 在 Tx1 内） | 无活动组（Σ=remaining=0）时，`TccCancelMarkPaid` 仍先调用 Fulfillment `ListShipments`；若 Fulfillment 不可用，返回 `UNAVAILABLE` 而非 AC-51 的“幂等成功”。**fail-closed，无错误写入/退款**，但把无关依赖带入 no-active 成功分支；现有 no-active 测试使用可用 fake。 | 在外部预检已确认 `sum==0 && remaining==0` 时直接幂等成功，不再要求 shipment facts；补“no-active + Fulfillment 超时”用例。 |
| **g3** | MINOR | `internal/services/order/tcc.go:304-324` | `TccQueryMarkPaid` 仍只对 `PENDING_PAYMENT/PAID/REFUNDING` 返回 `Prepared=true`；`ALLOCATED/SHIPPED/COMPLETED` 返回 false。该 RPC 仅被 `payment.VerifyPayment` 只读运维校验使用（`internal/services/payment/service.go:722-730`），不参与 DTM try/confirm/cancel；但 B1 修复后生命周期已允许，运维视图会显示“未准备”。 | 与 B1 状态集对齐（或明确文档说明仅 PAID/PENDING 有 prepared 语义）；属 MINOR。 |
| **g4** | MINOR（治理） | `git log e7e29e5`；`scripts/collab_gates.py:135-138` | 受管代码提交仍无 `Reviewed-by` trailer；G-1 在 HEAD 上会 FAIL。Reviewer actor 独立，但 merge 输入元数据未齐。 | 重审/merge 提交补 `Reviewed-by`（≠Actor），或按流程在复审后生成带 trailer 的提交；不能带 G-1 FAIL 合并。 |
| **g5** | MINOR（文档） | `docs/lead/PAY-SETTLE-01/t1.2-r-plan.md:6`（§6 Architect/QC/Reviewer TBD） | Plan 已 Human APPROVED，QC-19/20、Architect B-1/B-2 已落，但 Plan §6 的评审行未回写。 | Lead 回写审批记录；不影响实现 PASS。 |
| **g6** | MINOR（证据） | selfcheck §3 `/tmp/t12r-logs/*` | 自检清单引用 `/tmp` 日志；本 Reviewer 已确认这些日志真实存在，并独立复跑。日志不在 repo canonical，长期不可追溯；建议把最终测试日志摘要/命令落 `docs/implementation` 或 CI artifact。 | 文档/证据改进，不阻断。 |

---

## 5. 越界与边界检查

- **代码范围**：`git diff --name-only a3f080f..e7e29e5` 仅 4 个文件：`internal/services/order/tcc.go`、`tcc_settlement.go`、`settlement_integration_test.go`、`tcc_compensation_integration_test.go`；**无** `api/proto`、`api/gen`、`migrations/`、consumer、Payment/Fulfillment 生产代码、`cmd/`。
- **T1.4/T1.6 未实现**：`TccCancelMarkPaid` 仍不调用 `AdmitRefund`/`Payment.Refund`/`Fulfillment.CancelIfCreated`，不 append `order.refund_requested`；selfcheck 与 `tcc_settlement.go:22-30`/`:682` 注释明确 T1.4 边界；CANCELLED+captured、PENDING+captured 仍 fail-closed 归 T1.6。正确。
- **状态所有权**：Order 只写 `orders.refund_state`/permit/application/commitment；Fulfillment shipment 只读；Payment 只读快照。未改 proto/事件/表/枚举。
- **测试 DDL**：`orderTestDB` 现在执行真实 `migrations/order/0017`+`0018`；但 `orders`/`order_items`/`order_fulfillment_groups`/`order_refund_admissions` 等仍为测试内联 DDL（T5.4 负责完整真实迁移/回填）。`TestOrderTestSchemaUsesC05RefundMigrations` 明确校验 0017/0018 的表/列/CHECK。符合 Plan “至少用 0017/0018 后的 schema”。
- **无新依赖/GORM/RPC**；`ports.go` 仅在 T1.2b 已加 `ListShipments`（本范围未再改）。

---

## 6. 测试证据（Reviewer 独立复跑）

环境：`go1.27.1 darwin/amd64`；MySQL `mysql-unicorn` `127.0.0.1:3306`；DSN 由 `./scripts/test-dsn.sh` 生成：

- `root:root@tcp(127.0.0.1:3306)/mall_integration_t12r?parseTime=true`（TCC/ schema）
- `.../mall_integration_t12r_full`（完整 order）
- `.../mall_integration_t12r_race`（TCC -race）

`GOPATH=$PWD/.tmp/gopath`、`GOMODCACHE=/Users/lWoHvYe/go/pkg/mod`、`GOCACHE=$PWD/.tmp/gocache`、`GOPROXY=off`、`GOFLAGS=-mod=readonly`（未修改 go.mod）。

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0**，35.467s |
| `go test ./internal/services/order -run 'TestTcc|TestOrderTestSchema' -count=1 -v` | **PASS**，28.153s；全部 TCC/Σ/schema 用例通过（含新增 B1/M1/M2/R-4 用例） |
| `go test ./internal/services/order -count=1` | **ok**，64.807s（完整 order 包；与 -race 并行，耗时偏高） |
| `go test -race ./internal/services/order -run 'TestTcc' -count=1` | **ok**，45.759s（与完整包并行，耗时偏高） |
| `go vet ./internal/services/order` | exit 0 |
| `gofmt -l`（4 个变更文件） | 无输出 |

Developer 自检日志 `/tmp/t12r-logs` 已核对：`build-vet.log`（BUILD_OK/VET_OK）、`order-full.log`（40.884s）、`tcc-verbose.log`（PASS 31.218s）、`tcc-race.log`（24.074s）、`tcc-race-repeat.log`（5.582s）。本 Reviewer 复跑全部通过；耗时差异来自并行/机器负载。**未发现 SKIP/FAIL 被当作 PASS**。

**AC 台账**：`ac-coverage.tsv` 的 AC-51/AC-65 仍 `DEFERRED`；本报告**未**标任何 AC-VERIFIED。T1.2-R 是 AC-REF/实现证据，不是 Spec 全量验收。

---

## 7. 未覆盖范围

- T1.4（`AdmitRefund` 逐组、`CancelIfCreated`、逐组 `Payment.Refund`、`payment.refunded→SETTLED`、`MANUAL_HOLD/PAYING` worker/CAS）；T1.6（`PENDING_PAYMENT/CANCELLED+captured` 的 CAS→CANCELLED + `pay:<payment_no>:cancel`）；T1.5 consumer；T1.7/T5.2/T5.3 真实 Ship vs TCC 并发/故障注入；T5.4 真实迁移回填/AC-60；T5.7 combo。
- 真实 Fulfillment `ListShipments`/`CancelIfCreated`、Payment `Refund`、DTM/Outbox/NATS 端到端；BFF/运营展示；`make gates`/`make verify`/全量 `make test-race`。
- 本报告发现的 **G1**（legacy no-group SHIPPED/COMPLETED）与 **g1**（mixed known=1 line validation）**没有测试**；`TccQueryMarkPaid` 高级状态仍 false；`Reviewed-by` trailer 未补。
- 不评价 T2/T3、Spec/ADR 修订、Release 证据。

---

## 8. 宪法 / 治理核对（与本次相关）

| 条款 | 结果 |
|---|---|
| Article I.1/I.2 状态机与 fail-closed | B1 主路径已修复；G1 legacy no-group shipped 的 `PRE` 误判是 fail-open 边角（MAJOR，非 blocker）；其余 Payment/Fulfillment 不可用均 fail-closed。 |
| Article I.3 契约优先 | 未改 proto/事件/迁移；复用 existing `ListShipments`。 |
| Article I.4 拒绝自我妥协 | selfcheck 三问 + 重大妥协高亮；T1.4/T1.6 未假装完成。 |
| Article II.1/II.2 事实分离/所有权 | TCC 走 application/commitment；Order 唯一写；Fulfillment 只读。 |
| Article II.3 幂等 | batch/application/admission/commitment 唯一键/CAS/读回校验；并发测试通过。 |
| Article II.5 退款/许可不可逆 | `CONSUMED` 不 `REVOKED`、POST 不撤 permit 的测试通过；G1 边角是 PRE/POST 判定问题（见 G1）。 |
| Article II.6 读写隔离 | Σ 用 Payment 快照；Tx1 无远程调用。 |
| Article III.1/III.2 | Tx1 本地；显式 SQL/CAS；无 GORM。 |
| Article III.3/III.4/III.5 | 无新仓/W1/G1 缺省协议；apperr/静态守卫；无新栈。 |
| Article IV.1/IV.2 | 真实 MySQL 测试 + `-race` 复跑；未标 AC-VERIFIED；真实 Ship vs TCC 未覆盖（T1.7/T5.2）。 |
| Article IV.3 Mock 真实性 | 生命周期测试已用真实 `ALLOCATED+SHIPPED`/`SHIPPED`/`COMPLETED`；refund schema 用真实 0017/0018；G1 缺口与 mixed 组仍无真实数据用例（MINOR/MAJOR）。 |
| Article IV.4 | 本报告实现 PASS ≠ Release PASS。 |
| Workflow §8/§9 | T1.2-R Plan 已 APPROVED；本次复审完成，但冻结解除/下一步由人类决定；仍不得合 main。 |
| Workflow §10 Actor | `reviewer-phase1` ≠ `dev-phase1b`；报告记录 Actor/Reviewed-by/base/head/scope/release_status。 |

---

## 9. 是否可以交人类 merge 决策

- **实现级：可以交人类做“WIP/下一步”裁决**；本 Reviewer 的 `PASS` 仅覆盖 T1.2-R 的 B1/B2/M1/M2/M3/m1 修复范围。
- **不建议/不批准作为 main 或 Release 的 merge 输入**：
  1. T1.2 仍是 WIP；T1.4（逐组 `AdmitRefund`/`CancelIfCreated`/`Payment.Refund`/worker）、T1.6、T1.7 未完成，Tx1 提交后的 batch/application **没有 worker 驱动**；
  2. T5.7 integration owner 组合验证/Release 门禁未做；AC-51/AC-65 仍 `DEFERRED`，未 AC-VERIFIED；
  3. `e7e29e5` 受管提交缺 `Reviewed-by`，G-1 不能通过；
  4. §4 的 **G1**（legacy no-group SHIPPED/COMPLETED 的 PRE/POST 误判）建议在 T1.4/T1.7 前关闭；若人类要求“零 MAJOR”，应先修 G1。
- 人类可裁决：保持 WIP 并进入 T1.4，或先做 T1.2-R2（G1/mixed/no-active 小修）。**本报告不批准合并、不代替人类 merge/Release 决策。**

---

## 10. 声明

- 本报告未修改任何代码、proto、migrations、Spec/ADR；仅新增本报告。
- `verdict=PASS` 仅指 `a3f080f..e7e29e5` 的 T1.2-R 实现复审；不是 Spec 全量验收、不是 Release PASS、不批准合并。
- 前次 `REV-PAY-SETTLE-01-T1.2.md` 保留为历史 FAIL 记录；本报告不覆盖它。
- 所有结论绑定 `e7e29e5`（代码）与 `face7a6`（docs selfcheck）；测试命令与结果见 §6。
