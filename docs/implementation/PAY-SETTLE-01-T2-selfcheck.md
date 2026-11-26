# PAY-SETTLE-01 T2.1–T2.3 自检（历史 PAID 生产者 + 解析 + 唯一候选 CAS）

| 项 | 值 |
|---|---|
| change_id / Tier | `PAY-SETTLE-01` / **L2** |
| Actor | `dev-r04-fix`（Developer） |
| 契约 | SPEC-0002 v1.11 AC-60/63；ADR-0007 Rev 2.3 §4.4/§5.3/§5.8；`migrations/order/0019_legacy_payment_resolution.sql`；Tasks T2.1–T2.3 |
| **Code head（证据绑定）** | **`1e57c15e7539988e34b12e9f0c3d8222a083e004`** |
| Base | `106e5ac`（T2 重排） |
| 改动 | `internal/services/order/{legacy_resolution.go(新),legacy_resolution_integration_test.go(新),allocation_validation.go,service.go,tcc.go,refund_admission.go,settlement.go}` |
| 未改 | migrations/0019 字节、SPEC/ADR、T3 release/T4 shadow、T2.4 worker、T2.5 回填、T2.6 seq、台账 |
| Status | **WIP**；不声称 AC-60/63 AC-VERIFIED 或 T2 完成 |

## 1. T2.1 生产者修复（先修根因，不靠回填）

- **根因**：`freezeAllocationTx` 是 TCC/直接路径唯一的 `PENDING_PAYMENT→PAID` 写入点，但接受空 `paymentNo`；`MarkPaid`/`TccConfirmMarkPaid` 都未在写前拒绝空 `payment_no`，因此可产生 `status=PAID, settlement_payment_no=''`。
- **修法**：
  - `allocation_validation.go:freezeAllocationTx` 在 UPDATE 前拒绝空 `payment_no`（`PAID_WITHOUT_PAYMENT_NO`），作为最后一道单点防线；
  - `service.go:MarkPaid`、`tcc.go:TccConfirmMarkPaid` 在确认库存/进入事务前提前拒绝空 `payment_no`，避免库存副作用后才发现；
  - 不再用默认值/回填掩盖；旧行留给 T2.5。
- **测试**：`TestLegacyProducerRejectsPAIDWithoutPaymentNo`（两条写路径均拒绝，订单保持 `PENDING_PAYMENT`，`settlement_payment_no` 为空）。

## 2. T2.2/T2.3 解析、入队与 CAS

`ensureLegacyResolution`（`legacy_resolution.go`）：

1. 非 `PAID` 或已有生效支付 → 直接返回（幂等只读，不建队列）。
2. Payment `ListOrderIntents` **事务外**读取；仅保留 `order_no` 匹配的 intent；候选条件：`status=SUCCEEDED` 且 `captured == orders.pay_cent`。
3. **零元且无任何已捕获 intent** → 条件 CAS 写 `settlement_payment_no='zero:<order_no>'`，返回 `EXPIRED_ZERO`，不建队列。
4. **零元但有捕获 intent** → 不写 zero，进入队列（`OPEN`）。
5. **唯一候选** → `orders` 行锁 + `UPDATE ... WHERE settlement_payment_no='' AND status='PAID'` CAS；同候选重放幂等，不同候选入队；其他已捕获 intent 走支付级退款。
6. **0/多候选** → `legacy_payment_resolution` 一行：`OPEN`、`candidate_count`、`pay_cent`、DB 时间 `entered_at` + 24h `expires_at`、`blocked_scope=UNSHIPPED_GROUPS_AND_REFUND`。`ON DUPLICATE` 语义由行锁实现：终态/`COMPENSATING` 不重开、不重置 expiry；`OPEN/RESOLVING` 只刷新候选数/金额。
7. **入口闸门**：`AdmitRefund` 与 `AuthorizeShipment` 对 `PAID` 且 settlement 空的历史单先调用解析；队列/冲突时 fail-closed `LEGACY_PAYMENT_UNRESOLVED`，不写 admission/application/permit。
8. **其他捕获（T2.3）**：唯一候选解析后，对其余 `captured>0` 的 intent 以 `refund_id=pay:<payment_no>:dup`、`non_effective_capture=1` 持久化 `refund_applications(PENDING)` 再调用 `Payment.Refund`；成功后置 `DONE`；失败保留 PENDING+`last_error`，下一次解析重试；歧义订单不退款。
9. 所有远程调用在事务外；Tx 内只写 `orders`/队列/application。

**测试（真实 MySQL 8.4.13；真实 0019 迁移）**：

| 用例 | 覆盖 |
|---|---|
| `TestLegacyResolutionUniqueCASRefundsOtherCaptures` | 唯一候选写回、错单/非 SUCCEEDED 排除、其他捕获 1 次 `PAY-LR-B`/5000/`pay:...:dup`、重复解析不重退 |
| `TestLegacyResolutionZeroNoMultiCandidatesQueue` | 0 候选、2 候选、无 settlement 写入、队列 count/pay、24h expiry、重复解析不重置 expiry |
| `TestLegacyResolutionZeroIdentityAndCapturedZeroConflict` | 零元无捕获→`zero:`；零元有捕获→队列且不 zero |
| `TestLegacyResolutionConcurrentCASOneWriteOneRefund` | 8 并发只写一次 settlement、CAS 胜者只退一次其他捕获、application DONE |
| `TestLegacyResolutionBlocksAdmissionAndShipping` | 未解析订单退款/发货均 `LEGACY_PAYMENT_UNRESOLVED`，0 admission |
| `TestLegacyResolutionListFailureFailsClosed` | Payment 读失败不写 settlement/队列 |
| `TestLegacyProducerRejectsPAIDWithoutPaymentNo` | MarkPaid/TCC confirm 空 payment_no 拒绝，订单不 PAID |

## 3. 命令与结果（HEAD `1e57c15`）

```bash
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:13340)/mall_integration_t2_order?parseTime=true&loc=UTC' \
  go test ./internal/services/order -run 'TestLegacy' -count=1 -v        # 7 PASS, ok 7.347s
TEST_MYSQL_DSN='...' go test -race ./internal/services/order -run 'TestLegacy' -count=1  # ok 8.582s
TEST_MYSQL_DSN='...' go test ./internal/services/order -count=1                        # ok 214.487s
```

- 日志 `.tmp/evidence-t2/{HEAD.txt,t2-tests.log,t2-race.log,t2-order-full.log}`；无 SKIP/FAIL。
- 测试数据库由 `orderTestDB` 建基础 schema 后执行真实 `migrations/order/0019_legacy_payment_resolution.sql`（真实文件，非手写内联 DDL）。

## 4. 【测试范围】/【未覆盖范围】

【测试范围】T2.1 两条 PAID 写路径 + 单点 freeze；T2.2 候选过滤/零元三分支/0/多候选入队/24h 字段/CAS 冲突/读失败；T2.3 唯一 CAS、其他捕获支付级退款、并发单写、重复解析幂等、入口 fail-closed；真实 MySQL 8.4.13 + `-race` + 全 order 包回归。

【未覆盖范围】

- **T2.4 未实现**：队列 worker/SLA 24h 到期、`OPEN/RESOLVING→COMPENSATING/EXPIRED_*`、`EXPIRED_*` 映射、`ResumeLegacyResolution`/管理 RPC、每日对账；本步只建 `OPEN` 行并 fail-closed，不声称 SLA 闭环。
- **T2.5 未实现**：历史存量回填/dry-run/计数/重入；旧 `PAID` 空 settlement 行仍未修复。
- **T2.6 未实现**：`legacy:<order_no>:<seq>` 取号与并发唯一；当前其他捕获用 `pay:<payment_no>:dup`，不是 legacy 组级退款 id。
- 其他捕获的 `payment_duplicate_captures` 登记由 Payment owner；本步直接 `Payment.Refund`，fake 验证调用，未验证 Payment 真实重复捕获表写入。
- T2 测试的 queue 建表只应用 0019；0023 `chk_lpr_expiry_24h_v2` 的等值 CHECK 由 R0.4 证据覆盖，本步只断言 `TIMESTAMPDIFF=24h`。
- 未跑全仓 `make verify`/UAT/prod；无告警/BFF 展示。

## 5. 反妥协自检 / 影响声明

- 并发/失败：CAS 单胜；`-race` 通过；Payment 读失败 fail-closed；重复解析幂等；其他捕获 app PENDING + error，不静默吞掉。
- 是否绕过：未新增回填掩盖生产者；未改 0019/SPEC/ADR/T3/T4；未自创状态（复用 0019 的 `OPEN`/`EXPIRED_ZERO`）。
- `[修改点]`：见上表 7 文件（2 新）。
- `[架构侵入性]`：无新表/迁移/RPC；复用 `legacy_payment_resolution`、`refund_applications`、outbox 未改。
- `[副作用评估]`：`MarkPaid`/TCC confirm 空 payment_no 从“可能成功”改为拒绝（符合 AC-60）；历史空 settlement 的退款/发货从“直接失败/放行”改为解析或 fail-closed；队列行会在 0/多候选时新增，T2.4 前无 worker 自动推进。

## 6. 不声称

不声称 T2 完成 / AC-60/63 AC-VERIFIED / Release PASS；T2.4/T2.5/T2.6、告警/入口、Payment 重复捕获登记仍待后续；`ac-coverage.tsv` 由 Reviewer 更新。

---

## 7. T2.4 + 零元 gate（code head `e6d8bf9a90117711351f26818ab4cd9c6836f8dc`，base `1e57c15`）

| 项 | 值 |
|---|---|
| 状态 | **零元 gate 已修；T2.4 非 COMPENSATING 分支已实现；COMPENSATING→EXPIRED_REFUND `[BLOCKED]`** |
| 改动 | `refund_admission.go`（零元 gate）、`legacy_resolution.go`（state）、`legacy_resolution_worker.go`（新）、`legacy_resolution_worker_test.go`（新）、`cmd/order/main.go`（wire worker）、`refund_admission_integration_test.go`（旧断言随 gate 修正） |
| 未改 | 0019 字节/字段/迁移、T3/T4、T2.5/T2.6、台账 |

### 7.1 零元 gate（MAJOR）

`source=system_zero` 现在在 `admitZeroRefund` 之前检查 `settlement_payment_no`：
- 历史零元（`pay_cent=0`、无任何 captured）→ 先 `ensureLegacyResolution` 写 `zero:<order_no>`，再走零元短路；
- 零元有 captured → 队列/fail-closed `LEGACY_PAYMENT_UNRESOLVED`，**不写 zero、不写 REVIEW、不返回 `REFUND_ADMISSION_CONFLICT`**；
- 已 `zero:` 的订单不重复调用 Payment list。
测试：`TestLegacyZeroGateHistoricalOrder`、`TestLegacyZeroGateCapturedIntentIsNotZero`、`TestLegacyZeroGateDoesNotCallPaymentRefund`；旧 `TestAdmitRefundSystemZeroRejectsMismatchedFacts` 的 Payment 断言按新 gate 改为“不 get/不 refund”。

### 7.2 T2.4 worker / RPC

- `RunLegacyResolutionWorker`（`cmd/order/main.go` 已 wire，1 分钟）扫描 `OPEN/RESOLVING` 且 `expires_at<=NOW(3)`，先 30s lease claim（不改 state），再按 Payment intents/组投影：
  - 无 captured + `pay_cent=0` → `EXPIRED_ZERO`（CAS 写 zero settlement）；
  - 无 captured + `pay_cent>0` → `EXPIRED_FINANCE`（`FINANCE_HOLD`）；
  - 有 captured + 任一 SHIPPED/RECEIVED/EXCEPTION+shipped_at → `EXPIRED_FINANCE`；
  - 有 captured + 无离仓 → `COMPENSATING`（撤 `NONE` permit、对 `CONSUMED` + CREATED 调 `CancelIfCreated`）。
- 终止 CAS 带 version/state 条件，终态不重开；`ResolveLegacyResolution` 实现现有 proto 的 manual `OPEN/RESOLVING→RESOLVED` + `settlement_payment_no` CAS + `resolution_id=lpr:<order>`；`Get/Resume` 返回视图；无新 RPC。
- 测试：`TestLegacyExpiryBranches`（EXPIRED_ZERO/EXPIRED_FINANCE×2/COMPENSATING prefix/取消失败保持 COMPENSATING）、`TestLegacyManualResolveCASAndTerminalLock`、`TestLegacyTimerVsManualResolveCAS`。

### 7.3 `[BLOCKED]` COMPENSATING→EXPIRED_REFUND

ADR §4.4.3 第 4 步要求先完成 INV-44 释放，再 `Payment.Refund`，全部成功才 `EXPIRED_REFUND`。现有释放生产者的 source 是封闭枚举，仅接受 `tcc/system_zero/system_shortage/aftersale`：

- `internal/services/order/refund_release.go:32-48`（`refundReleaseSource`）拒绝 `legacy_expired`（`REFUND_RELEASE_SOURCE_UNSUPPORTED`）；
- `internal/platform/events/events.go:49-57` + `registry.go:200-205` 无 `legacy_expired` source/reason；
- 不能改 T3（release/commitment）或 0019/新增字段；用其它 source 伪造或跳过释放均违反 INV-44 与审计。
- **具体依赖**：Lead/Architect 裁决是否为 `legacy_expired` 增加 INV-44 source/reason（或另立 T3 释放流程）。在裁决前，worker 只完成补偿前缀并**保持 COMPENSATING**（不写 release、不调 Payment、不置 EXPIRED_REFUND），`processExpiredLegacyResolution` 返回 `errLegacyCompensationBlocked`；测试断言该行仍 COMPENSATING、permit 已 REVOKED、0 refund。
- 因此 T2.4 **不能声称完成**；`EXPIRED_REFUND`、补偿成功、7 天 finance close、`REJECTED` 生产入口仍未交付（见 §7.4）。

### 7.4 命令与结果（HEAD `e6d8bf9`）

```bash
TEST_MYSQL_DSN='.../mall_integration_t2_order...' \
  go test ./internal/services/order -run 'TestLegacy|TestAdmitRefundSystemZero|TestZero' -count=1  # ok 27.419s
TEST_MYSQL_DSN='...' go test -race ./internal/services/order -run 'TestLegacy' -count=1             # ok 20.396s
TEST_MYSQL_DSN='...' go test ./internal/services/order -count=1                                     # ok 237.051s
```
- 日志 `.tmp/evidence-t2/{HEAD.txt,t24-all.log,t24-race.log,t24-order-full.log}`；无 SKIP/FAIL。

### 7.5 【测试范围】/【未覆盖范围】

【测试范围】零元历史 gate 两向；24h 的 EXPIRED_ZERO/EXPIRED_FINANCE（无捕获正/有捕获已离仓）；COMPENSATING 入口的 permit 撤销/CancelIfCreated；取消失败保持 COMPENSATING；manual Resolve CAS/终态不重开；timer vs manual 并发；`-race` + order 全包。

【未覆盖范围】

- **COMPENSATING 成功**（INV-44 释放 + Payment.Refund + `EXPIRED_REFUND`）因 source 契约 `[BLOCKED]`。
- `EXPIRED_FINANCE` 7 天财务关闭、`REJECTED` 生产入口（无 RPC/命令，未实现）。
- 真实 ticker 的 1 分钟调度未在测试中等待；直接调用 `processExpiredLegacyResolution`，lease/CAS 单测覆盖。
- 队列 0023 等值 24h CHECK 仍由 R0.4 证据覆盖；T2.4 表由 0019 建。
- T2.5 回填、T2.6 seq、Payment `payment_duplicate_captures` 实际登记仍未交付。

### 7.6 不声称

不声称 T2.4 完成 / AC-63 AC-VERIFIED / Release PASS；零元 gate MAJOR 已修，COMPENSATING 成功与 EXPIRED_REFUND 待 release source 契约裁决；`ac-coverage.tsv` 仍由 Reviewer 更新。
