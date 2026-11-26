# PAY-SETTLE-01 T5.0 — AC-39…68 证据审计（只读）

| 项 | 值 |
|---|---|
| change_id / Tier | `PAY-SETTLE-01` / **L2** |
| Actor | `dev-r04-fix`（Developer；只读审计，不写业务/测试代码） |
| 审计 HEAD | **`c56126aac3130403656cbe30ad0aac26190f7972`** |
| 输入 | SPEC-0002 v1.11 AC-39…68（`docs/specs/0002-payment-settlement-admission.md:158-450`）、`docs/collab/ac-coverage.tsv`、`docs/lead/PAY-SETTLE-01/tasks.md` Phase 5、现有 `internal/services/**` 测试 |
| 分类口径 | **EVIDENCE-READY**=现有测试真实覆盖该 AC 的关键断言；**TEST-GAP**=代码路径存在但测试缺失/断言不全；**FEATURE-GAP**=依赖未实现能力（T2/T3/T4/告警/入口），本阶段不能验证 |
| 不写 | `ac-coverage.tsv`、代码、契约、台账；AC-REF 不等于 AC-VERIFIED |

## 0. 结论与统计

**EVIDENCE-READY 3 / TEST-GAP 17 / FEATURE-GAP 10（共 30 条）。**

- **E（3）**：AC-50、AC-67、AC-68。
- **T（17）**：AC-42、43、44、45、46、47、48、49、51、52、53、54、55、56、58、65、66。
- **F（10）**：AC-39、40、41、57、59、60、61、62、63、64。

审计在真实 MySQL 8.4.13 + NATS 2.10 上对代表性子集做了只读复核（见 §4）；未重跑全部矩阵。**未发现“测试绿但把 FEATURE-GAP 冒充 VERIFIED”的情况；发现实现缺口（AC-39 渠道金额退款、AC-41 非生效捕获退款、AC-57 `CREATED` 未拒绝）与 AC-59 行为相反用例（乱序仍结账），需回 Plan/实现。**

## 1. 逐条分类

> `file:line` 均相对仓库根；命令默认需要 `TEST_MYSQL_DSN`（真实 MySQL 8.4），含 NATS 的另需 `TEST_NATS_URL`。`-race` 列只标已有专用竞态/可适用的用例。

### EVIDENCE-READY（3）

| AC | 证据（file:line / test） | 覆盖断言 | 命令 / 竞态 |
|---|---|---|---|
| **AC-50** | `internal/services/order/ac50_review_integration_test.go:102,159,203,347,405,438,537,567,597,640`；`ac50_review_audit_fault_test.go:18,117,184`；`ac50_review_race_test.go:59,102,143,179` | 普通售后额度/PARTIAL/POST/legacy/zero；REVIEW 不写 PRE、不探测、不释放；TCC Tx2 持批 gate；audit/re-arm 保留；Ship-vs-cancel 单终态 | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC50Review\|TestTCCReviewTx2' -count=1`；`-race`：`ac50_review_race_test.go` 四例 |
| **AC-67** | `internal/services/order/refund_admission_integration_test.go:362,449,539,603`；`ac50_review_integration_test.go:159,567,597`；`internal/services/aftersales/zero_refund_integration_test.go:182,221,274,327,366,413,497,562,594`；`workers_source_test.go:5` | 零元短路不调 Payment、S19 DONE、无 PENDING、正金额拒绝、回库前不短路、零元 CONSUMED 无运单不持久化 REVIEW、重复幂等 | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAdmitRefundSystemZero\|TestAC50ReviewZero' -count=1`；`TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run 'TestZero' -count=1` |
| **AC-68** | `internal/services/aftersales/zero_refund_integration_test.go:497`（端到端）、`:182`；断言 helper `:134 assertZeroRefunded` | AfterSales `REFUNDED/effective=0/refunded=0/amount_state=CONSUMED/refund_no=''`、`source=system_zero`、无 Payment、展示 `DONE(zero_amount)` 的事实投影 | 同上 `TestZeroAfterSaleEndToEnd...` / `TestZeroAfterSaleRefundTask...`；无专用 `-race`（含 crash/idempotency 用例） |

### TEST-GAP（17）

| AC | 代码路径 / 已有测试 | 缺失分支 | 建议测试文件 / 命令 |
|---|---|---|---|
| **AC-42** | `api/proto/mall/v1/mall.proto:620-645`（status/refund_state/shipment_status/available_actions 分列）；无 handler/BFF 断言 | “捕获未结算只显示处理中”“PAID≠可发货”“单组退款不影响他组”无契约测试 | `internal/services/order` 新增 handler/query 测试；`go test ./internal/services/order -run TestAC42 -count=1` |
| **AC-43** | `internal/services/payment/settlement.go:32,58,89`；`settlement_task_integration_test.go:9` 仅 claim/finish | 捕获已提交+结算未完成崩溃恢复；CANCELLED 全额退款/HELD-CONFIRMED 收敛 | `internal/services/payment/settlement_crash_test.go`；`go test ./internal/services/payment -run 'TestSettlement.*Crash\|TestSettlementResume' -count=1`（fault） |
| **AC-44** | `internal/services/payment/service.go:267-287` 重复 event/identity 冲突 | 同 payment 重复回调副作用一次；金额/订单不一致时真实入账退款；本地失败不 ack | `internal/services/payment/callback_idempotency_test.go`；`go test ./internal/services/payment -run TestCallback -count=1` |
| **AC-45** | `internal/services/order/service.go:604-608` 已 PAID 他 payment 冲突；`duplicate_payment_integration_test.go:12` 仅 TCC 第二支付拒绝 | 同 payment 幂等成功；非生效捕获退款；不触发库存/发货副作用 | `internal/services/order/duplicate_payment_integration_test.go` 补两分支；`-race` |
| **AC-46** | `internal/services/order/service.go:415` CancelOrder；`t16_payment_cancel_integration_test.go:489` 取消胜出单 CAS | 支付胜出时取消返回 `ORDER_ALREADY_PAID`；已捕获全额退款；PAID 无取消动作 | 扩展 `t16_payment_cancel_integration_test.go`；`go test ./internal/services/order -run 'TestT1_6.*Race\|TestAC46' -count=1 -race` |
| **AC-47** | `internal/services/order/service.go:591,635,829` MarkPaid 先 Confirm 再 PAID；`inventory/operations_integration_test.go:173` 部分确认续跑 | Inventory 明确失败/超时/超 D-11 的退款与 CANCELLED；确认成功再缺货交 AC-56 | `internal/services/order/settlement_inventory_fault_test.go`；fault/`-race` |
| **AC-48** | `internal/services/order/settlement.go:307` AuthorizeShipment；`settlement_integration_test.go:257,297,317,332,344`；`fulfillment/settlement_integration_test.go:177` | 行 CONFIRMED/缺货 NOTIFIED 门禁；`CANCELLED` 运单 Ship 拒绝；REVOKED/CONSUMED 不同 shipment_no | 扩展 `settlement_integration_test.go` 与 fulfillment Ship 测试；`-race` |
| **AC-49** | `api/proto/mall/v1/mall.proto:620-645` 四类字段分列；无契约测试 | 支付/结算/库存可履约/发货许可四事实分别输出、不得单状态替代 | BFF/handler 契约测试；`go test ./internal/services/order -run TestFourFacts -count=1` |
| **AC-51** | `t17_evidence_integration_test.go:26`；`tcc_compensation_integration_test.go:251,377,452,512,544,581,660`；`tcc_review_integration_test.go:34` | 逐组终态/部分组已退跳过/每组打款失败只补未完成/Σ 相等后逐组；T3 release | 扩展 TCC 测试矩阵；`TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestTcc\|TestT17' -count=1 -race` |
| **AC-52** | `internal/services/order/shortage.go:34`；`settlement_integration_test.go:856`；`refund_release_integration_test.go:462` | SYSTEM 缺货取消未离仓运单、已离仓只退款、部分组运费；POST 不取消 | `internal/services/order/shortage_refund_test.go`；MySQL+fault |
| **AC-53** | `payment/settlement_integration_test.go:155,172`；`order/settlement_integration_test.go:399` | 非生效捕获单独记账、不计订单 `captured/refunded`；多 intent 不用单 intent 判定 | `internal/services/payment/settlement_funding_test.go`；MySQL |
| **AC-54** | `settlement_integration_test.go:332,344`；`refund_release_integration_test.go:175`；`t17_evidence_integration_test.go:442` | 整单全退阻断所有未发货组并取消运单；单组阻断/其他组许可不变 | 扩展 order/fulfillment 测试；`-race` |
| **AC-55** | `settlement_integration_test.go:355,399`；`tcc_reconcile_integration_test.go` | 退款超时未知保持处理中；零元例外与 Payment 事实优先 | `internal/services/order/refund_projection_test.go`；fault |
| **AC-56** | `shortage.go:34,104,239,260`；`settlement_integration_test.go:856` 仅选择/默认 | 4h 通知、48h 默认 worker、补货/退款并发 CAS、RESTOCKED 后重复/新 event、RESOLVED_REFUND 后补货拒绝 | `internal/services/order/shortage_worker_test.go`；MySQL+fault+`-race` |
| **AC-58** | `fulfillment/delivery_integration_test.go:321` 仅断言不回滚 receipt | 争议成立后进入售后/退款流程、receipt 保留的完整链路 | 扩展 `delivery_integration_test.go`；MySQL |
| **AC-65** | `t17_evidence_integration_test.go:26`；`tcc_payment_integration_test.go:417,475`；`tcc_precheck_integration_test.go:319`；`tcc_reconcile_integration_test.go:190` | 快照变化重检 Σ、逐组打款终态、跳过 SETTLED、崩溃/重放只补未完成 | 扩展 TCC 矩阵；`-race`+fault |
| **AC-66** | `fulfillment/settlement_switch_integration_test.go:157,188,207,308,399` 审批矩阵 | 影子消费不建单/不放行/不改生命周期；未知版本拒绝；ON→SHADOW 回滚恢复兼容 | `internal/services/fulfillment/settlement_shadow_test.go`；MySQL+NATS |

### FEATURE-GAP（10）

| AC | 现状 | 依赖/不能验证原因 | 处置 |
|---|---|---|---|
| **AC-39** | `payment/service.go:91` CreateIntent 校验请求金额；`service.go:245,297-300` Callback 金额不符直接报错；`order/service.go:1341` paidAmount 拒绝 ≠ pay_cent；**无“渠道已捕获≠pay_cent 全额退款”实现/测试** | T5.3 故障注入（渠道异常/金额不符退款）未实现 | 回 T5.3/实现裁定；不可用现有测试标 VERIFIED |
| **AC-40** | 零元核心 `order/zero.go:28` + `zero_integration_test.go:22,57,104,129`；**有捕获 intent 的 legacy 队列/24h `EXPIRED_ZERO` 未实现** | AC-63/T2（`legacy_payment_resolution` 仅迁移，无 resolver/worker） | 归 T2/AC-63；核心零元 ≠ AC-40 全量 |
| **AC-41** | `order/service.go:604-608` 拒绝第二 payment；**未对非生效捕获全额退款，无 `DUPLICATE_PAYMENT` 实现/测试** | WP-R1（重复支付身份/退款）未实现 | 回 Plan/实现，不得以 AC-45 单测代替 |
| **AC-57** | `fulfillment/delivery.go:334` ConfirmReceipt；`:386` 仅拒绝 `CANCELLED`；**未拒绝 `CREATED`**；无 ConfirmReceipt 的 CREATED/CANCELLED/dispute/幂等测试（`TestAC23DirectReceiveRejected` 只覆盖 direct receive） | 实现缺口（AC 要求 `CREATED/CANCELLED` 都拒绝） | **审计发现：需回 Plan/实现**；本阶段不能验证 |
| **AC-59** | 全仓无 `EVENT_OUT_OF_ORDER` 拒绝实现；`refund_consumer_integration_test.go:125` 反而是“乱序仍结账”用例 | T5.3 乱序拒绝未实现 | 回 Plan；不得用正序/幂等测试冒充 |
| **AC-60** | 无 `legacy:<order_no>:<refund_seq>` 分配器（grep 无实现）；`legacy` 仅 AC-50/AC-63 部分 | T2 历史单回填/分配未实现 | 归 T2；T5.4 真实迁移后再验 |
| **AC-61** | `docs/ops.md` 有 runbook；**30 分钟告警/人工重放入口/每日对账部署证据未实现** | 告警与入口延期（AC-61/AC-72 alerts） | 归 T5.3/运维；runbook≠实现 |
| **AC-62** | `refund_commitment_integration_test.go` 覆盖 SETTLED；**D-24 人工释放授权/审计/额度释放未实现** | T3（AC-62 明确 T3 依赖） | 归 T3；不得以 no-release 测试标 VERIFIED |
| **AC-63** | 仅有 `migrations/order/0019_legacy_payment_resolution.sql`；无 Go resolver/worker/backfill | T2（AC-63 全量） | 归 T2；本阶段不可验证 |
| **AC-64** | 行/运费台账 T3 未实现（现有 `refund_release`/freight 仅 partial）；`AC-14/31/32` 只覆盖既有运费，不是 AC-64 行层全量 | T3（AC-64 明确 T3 依赖） | 归 T3 |

## 2. AC 文本分支清单（关键分支覆盖）

图例：✓=已有测试覆盖并断言；△=部分/间接；✗=未覆盖或实现缺失；—=不适用。

| AC | 关键分支 | 状态 |
|---|---|---|
| 39 | 请求金额≠pay_cent 拒绝 | ✓ `create_intent_test.go:25` |
| 39 | 渠道捕获≠pay_cent 全额退款 | ✗（无实现） |
| 40 | 零元无捕获→PAID/零元售后 | ✓ |
| 40 | 零元有捕获→legacy 队列/24h | ✗（T2） |
| 41 | 第二 payment 拒绝 | △（AC-45 单测） |
| 41 | 非生效捕获退款/同 payment 幂等 | ✗ |
| 42 | 捕获未结算仅“处理中”；PAID≠可发货；单组退款展示 | ✗ |
| 43 | 捕获+结算同事务/任务 | ✓ 任务持久化 |
| 43 | 崩溃续跑/CANCELLED 补偿 | ✗（T5.3） |
| 44 | 重复回调幂等 | △ payment consumer 命令幂等 |
| 44 | 金额/订单不一致退款、失败不 ack | ✗（T5.3） |
| 45 | 已 PAID 他 payment 拒绝 | ✓ `TestAC45...` |
| 45 | 同 payment 幂等/非生效退款 | ✗ |
| 46 | 取消 vs 支付单 CAS | ✓ 取消胜出 |
| 46 | 支付胜出→取消失败+退款 | ✗ |
| 47 | MarkPaid 前 Confirm 全部行 | ✓ 代码/部分测试 |
| 47 | Inventory 失败/超时退款、CANCELLED | ✗（T5.3） |
| 48 | NONE/CONSUMED/REVOKED、不同 shipment_no | ✓ |
| 48 | 行 CONFIRMED/缺货门禁、CANCELLED Ship | ✗ |
| 49 | 四事实分列输出 | ✗ 契约测试 |
| 50 | 全部主要分支 | ✓（见 §1） |
| 51 | TCC Σ/REVIEW MANUAL_HOLD/逐组准入 | ✓ |
| 51 | 逐组终态、跳过已退、部分失败 | △/✗ |
| 52 | SYSTEM 缺货准入/金额 | △ `TestAdmitRefundSystemShortageSource` |
| 52 | 未离仓取消运单、已离仓只退 | ✗ |
| 53 | captured/refunded 聚合 | ✓ payment 聚合 |
| 53 | 非生效捕获不计入、多 intent | △/✗ |
| 54 | 单组隔离/其他组可发 | ✓ `TestAuthorizeShipmentAllowsOtherGroup...` |
| 54 | 整单全退阻断所有组 | ✗ |
| 55 | Payment 事实优先/投影 | ✓ 部分 |
| 55 | 超时未知保持处理中 | ✗ |
| 56 | 缺货选择/48h 默认 | △ `TestGroupShortageChoiceAndDefaultRefund` |
| 56 | 4h 通知/补货 CAS/重复终态 | ✗ |
| 57 | CANCELLED 拒绝 | ✓ `confirmReceiptTx` |
| 57 | CREATED 拒绝 | ✗ 实现缺口 |
| 57 | 重复幂等/争议 OPEN 拒绝 | △ 单元/AC-22 |
| 58 | 收货后争议不回滚 receipt | ✓ `TestAC36...` |
| 58 | 进入售后/退款流程 | ✗ |
| 59 | 退款先于捕获拒绝 | ✗（无实现） |
| 59 | 收货先于发货/库存失败先于支付/缺货解决先于上报 | ✗ |
| 60 | 历史 PAID 展示/退款/发货 | ✗（T2） |
| 61 | 30m 用户处理中/告警 | ✗ |
| 61 | 人工重放入口/日对账 | △ runbook only |
| 62 | REVIEW 保持 RESERVED/许可原值 | ✓ `ac50_review_integration_test.go:102` |
| 62 | D-24 人工释放/审计/冲突 | ✗（T3） |
| 63 | 唯一候选写回/0 或多候选队列/24h 分支 | ✗（T2） |
| 64 | 行级/运费/UNDETERMINED/168h | ✗（T3） |
| 65 | Σ/REVIEW/部分失败/崩溃续跑 | △ 部分 |
| 66 | 审批矩阵/配置 fail-closed | ✓ switch 测试 |
| 66 | 影子消费不建单/未知版本/回滚 | ✗ |
| 67 | 零元短路/正金额拒绝/回库 gate/无 REVIEW | ✓ |
| 68 | DONE(zero_amount) 字段/展示/AC-55 例外 | ✓ |

## 3. 已有测试矩阵命令与环境

### 3.1 一键（真实依赖）
```bash
export TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:13339)/mall_integration_test?parseTime=true&loc=UTC'
export TEST_NATS_URL='nats://127.0.0.1:14226'
make test-integration       # 每个包用 scripts/test-dsn.sh 派生独立 schema
make test-race              # 竞态子集
```
- MySQL **8.4**（本审计 8.4.13）、NATS **2.10 JetStream**；未设 `TEST_MYSQL_DSN`/`TEST_NATS_URL` 时集成测试 `t.Skip`，**SKIP 不算证据**。

### 3.2 分包直跑（审计建议）
```bash
# order
TEST_MYSQL_DSN="$(scripts/test-dsn.sh "$TEST_MYSQL_DSN" order)" \
  go test ./internal/services/order -run 'TestAC50Review|TestAdmitRefund|TestTcc|TestT17|TestSettlement|TestZero' -count=1
# payment
TEST_MYSQL_DSN="$(scripts/test-dsn.sh "$TEST_MYSQL_DSN" payment)" \
  go test ./internal/services/payment -run 'TestCreateIntent|TestCallback|TestRefundBatch|TestGetPayment|TestRefundOrder' -count=1
# fulfillment
TEST_MYSQL_DSN="$(scripts/test-dsn.sh "$TEST_MYSQL_DSN" fulfillment)" TEST_NATS_URL="$TEST_NATS_URL" \
  go test ./internal/services/fulfillment -run 'TestAC|TestShip|TestCancelIfCreated|TestSettlementSwitch|TestRefundAdmitted' -count=1
# aftersales
TEST_MYSQL_DSN="$(scripts/test-dsn.sh "$TEST_MYSQL_DSN" aftersales)" \
  go test ./internal/services/aftersales -run 'TestAC14|TestAC31|TestAC32|TestZero|TestReview|TestAfterSale' -count=1
# inventory
TEST_MYSQL_DSN="$(scripts/test-dsn.sh "$TEST_MYSQL_DSN" inventory)" TEST_NATS_URL="$TEST_NATS_URL" \
  go test ./internal/services/inventory -run 'TestRefundRelease|TestCompensation|TestConfirm|TestCancelConfirmed|TestRelease' -count=1
```

### 3.3 本次只读复核（HEAD `c56126a`，真实 MySQL 8.4.13 + NATS 2.10）
```bash
TEST_MYSQL_DSN='.../mall_integration_audit_order...' go test ./internal/services/order \
  -run 'TestAC50Review|TestAC45|TestT1_6ConcurrentPendingCapturedCancelSingleWinner|TestAdmitRefundPreShipAndIdempotent|TestTCCReviewManualHold|TestRefundCommitment' -count=1   # ok 20.453s
TEST_MYSQL_DSN='.../mall_integration_audit_payment...' go test ./internal/services/payment \
  -run 'TestCreateIntentValidatesOrderAmount|TestOrderRefundRequestedCommandIsIdempotent|TestRefundBatchReplayIsIdempotent|TestGetPaymentByOrder' -count=1                        # ok 0.913s
TEST_MYSQL_DSN='.../mall_integration_audit_ff...' go test ./internal/services/fulfillment \
  -run 'TestAC23|TestAC36|TestShipRequiresOrderPermit|TestSettlementSwitchShadowMatrix' -count=1                                                                        # ok 1.496s
TEST_MYSQL_DSN='.../mall_integration_audit_as...' go test ./internal/services/aftersales \
  -run 'TestZeroAfterSale|TestAC31|TestAC32' -count=1                                                                                                                  # ok 4.395s
TEST_MYSQL_DSN='.../mall_integration_audit_inventory...' TEST_NATS_URL=... go test ./internal/services/inventory \
  -run 'TestRefundReleaseConsumerRestocksConfirmedOnce' -count=1                                                                                                        # ok 0.400s
```
- 日志 `.tmp/evidence-t5-audit/{HEAD.txt,order.log,payment.log,ff.log,as.log,inv.log}`；只证明这些用例真实执行，不把未跑 AC 当 VERIFIED。
- 未跑全量 `make verify`、未跑生产/UAT、未部署告警；审计为主。

## 4. 缺口处置建议（按 T5 子任务）

- **T5.1 contract/handler**：AC-42/49；补 BFF/API 断言后再谈 VERIFIED。
- **T5.2 `-race`**：AC-41/45/46/48/51/54/56/65；优先补 AC-46 支付胜出、AC-48 行 CONFIRMED/缺货门禁、AC-56 案件 CAS。
- **T5.3 fault**：AC-39/43/44/47/51/55/56/59/61/65；AC-39 渠道退款、AC-43 崩溃续跑、AC-59 乱序拒绝是硬缺口。
- **T5.4 真实迁移**：AC-60/63（legacy 分配/回填）、AC-66 切换矩阵；现有 `0019` 仅表，不可当实现。
- **T5.5 baseline**：AC-57/58；AC-58 有 AC-36 不回归用例但无退款链路；AC-57 先修 CREATED 拒绝（实现缺口）。
- **T2/T3/T4 依赖**：AC-40 legacy 分支、AC-60/63（T2）；AC-62/64（T3）；AC-61 告警/入口（T5.3/运维）；这些不能在 T5.0 标 EVIDENCE-READY。

## 5. 边界与不声称

- 本审计只读，未改代码/测试/契约/台账；`ac-coverage.tsv` 仍由 Reviewer 更新。
- 分类是 Developer 证据输入，不是 Reviewer 裁决；AC-REF/AC-VERIFIED 的最终状态由 Reviewer 按精确 HEAD 决定。
- AC-50/67/68 的 E 仅表示**现有测试覆盖其关键断言**，不等于 Release PASS 或全量 AC 验收；AC-51/65 等虽有部分绿测，仍按 T 处理。
- 需回 Plan 的项：**AC-39 渠道捕获≠pay_cent 无全额退款**、**AC-41 非生效捕获无退款**、**AC-57 `CREATED` 未拒绝**（见 `fulfillment/delivery.go:386`）、**AC-59 无 `EVENT_OUT_OF_ORDER` 拒绝**（现有乱序用例反而结账）。建议 Lead 在 T5 计划中单独立项，不得由本审计顺手改码。
