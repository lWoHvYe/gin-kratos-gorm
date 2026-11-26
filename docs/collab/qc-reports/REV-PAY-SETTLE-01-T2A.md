# REV-PAY-SETTLE-01-T2A — T2.1–T2.3 scoped 复审

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Tier | **L2（确认）**：历史 PAID/支付身份/退款边界/跨服务 ListOrderIntents；不降级 |
| review_scope | **仅 T2.1/T2.2/T2.3**；不审 T2.4–T2.6/T3/T4；不声称 AC-60/63 VERIFIED、T2 完成或 Release |
| base | **`106e5ac`**（T2 重排） |
| code head | **`1e57c15e7539988e34b12e9f0c3d8222a083e004`**（7 files，+822/−1） |
| head（复审绑定） | **`b73ac5163982c75146b7a7ad57a07fb34db51491`**（selfcheck） |
| 契约输入 | SPEC-0002 v1.11 AC-60/63；ADR-0007 Rev 2.3 §4.4/§5.3/§5.8；Tasks T2.1–T2.3；`migrations/order/0019_legacy_payment_resolution.sql` |
| INV | **INV-45**（payment/refund 身份不可伪造）、**INV-46**（历史解析收敛，worker 属 T2.4）、INV-50（零元边界） |
| verification | §1：真实 MySQL 8.4.13 + 0019；`TestLegacy` 7 PASS 8.846s / `-race` 8.787s；`migrations/0019` 字节未改；仅 7 个 order 文件 |
| 日期 / Actor / Reviewed-by | 2026-11-27 / `reviewer-r04` / `human (merge pending)` |
| release_status | **NOT RELEASE** |
| overall verdict | **PASS（T2a scoped）**，带一个 `[MAJOR]` 零元入口缺口；AC-60/63 仍 AC-REF；T2.4–T2.6 未做 |

---

## 0. 结论

T2.1–T2.3 的主链实现与测试成立：

- **生产者**：`freezeAllocationTx` 单点、`MarkPaid`、`TccConfirmMarkPaid` 均在写 PAID 前拒绝空 `payment_no`；不再产生 `PAID + settlement_payment_no=''`。
- **解析**：`ListOrderIntents` 事务外读取；过滤 `order_no`；候选 `SUCCEEDED && captured==pay_cent`；零元无捕获写 `zero:<order_no>`；零元有捕获不写 zero；0/多候选写一条 `legacy_payment_resolution(OPEN)`，24h DB-time expiry，终态/COMPENSATING 不重开。
- **CAS / 其他捕获**：唯一候选 `orders` 行锁 + `WHERE settlement_payment_no='' AND status='PAID'` CAS，单胜；其他 captured intent 以 `pay:<payment_no>:dup` 持久化 `refund_applications(PENDING)` 后 Payment.Refund，重复解析不重退；歧义订单不退。
- **入口**：`AdmitRefund`（正金额路径）与 `AuthorizeShipment` 对历史空 settlement 先解析，未解析/冲突 fail-closed `LEGACY_PAYMENT_UNRESOLVED`。

但发现一个 **`[MAJOR]` 零元入口缺口**：`AdmitRefund` 的 `source=system_zero` 分支在 legacy gate **之前**直接进入 `admitZeroRefund`（`refund_admission.go:341`），而 `admitZeroRefund` 要求 settlement 已是 `zero:<order_no>`（`:650`）；历史零元 PAID 单（settlement 空）不会调用 `ensureLegacyResolution`，零元退款会直接 `REFUND_ADMISSION_CONFLICT`。这不是资损（fail-closed），但 AC-60 的“历史零元单退款”路径未闭环，需在 T2.4/T2.5 前修/补测。故 **AC-60/63 仍 AC-REF，不升级**。

---

## 1. 实际执行（HEAD b73ac51 / code 1e57c15）

```bash
# MySQL 8.4.13；测试应用真实 migrations/order/0019_legacy_payment_resolution.sql
TEST_MYSQL_DSN=.../mall_integration_t2a go test ./internal/services/order -run 'TestLegacy|TestT2' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_t2a_race go test -race ./internal/services/order -run 'TestLegacy|TestT2' -count=1
```

| 测试 | 结果 |
|---|---|
| `TestLegacyResolutionUniqueCASRefundsOtherCaptures` | **PASS**：唯一候选、错单/非 SUCCEEDED 排除、其他捕获 1 次 `pay:...:dup`、重复不重退 |
| `TestLegacyResolutionZeroNoMultiCandidatesQueue` | **PASS**：0/2 候选不写 settlement、队列 count/pay、24h expiry、不重置 |
| `TestLegacyResolutionZeroIdentityAndCapturedZeroConflict` | **PASS**：零元无捕获→`zero:`；零元有捕获→队列、不 zero |
| `TestLegacyResolutionConcurrentCASOneWriteOneRefund` | **PASS**：8 并发单写、胜者只退一次 |
| `TestLegacyProducerRejectsPAIDWithoutPaymentNo` | **PASS**：MarkPaid/TCC confirm 空 `payment_no` 拒绝，订单不 PAID |
| `TestLegacyResolutionListFailureFailsClosed` | **PASS**：ListOrderIntents 失败不写 |
| `TestLegacyResolutionBlocksAdmissionAndShipping` | **PASS**：未解析订单退款/发货 `LEGACY_PAYMENT_UNRESOLVED`，0 admission |
| plain / `-race` | **7/7 PASS；8.846s / 8.787s** |
| `migrations/0019` / forbidden | **diff 空**；未改 SPEC/ADR/tsv；仅 7 个 order 文件 |

原始日志：`.tmp/evidence-t2a/reviewer-t2a.log`（本次审查生成，未入库）。

---

## 2. 逐项核对

| 检查 | 结论 | 证据 |
|---|---|---|
| `freezeAllocationTx` 拒绝空 `payment_no` | **PASS** | `allocation_validation.go:362-371`；单点 UPDATE 前返回 `PAID_WITHOUT_PAYMENT_NO` |
| `MarkPaid` 拒绝空 | **PASS** | `service.go:606-613`（库存在确认前） |
| `TccConfirmMarkPaid` 拒绝空 | **PASS** | `tcc.go:69-75` |
| `ListOrderIntents` 过滤 order_no | **PASS** | `legacy_resolution.go:126-139`；测试错单排除 |
| 候选 `SUCCEEDED && captured==pay_cent` | **PASS** | `legacy_resolution.go:84-95` |
| 零元无捕获→`zero:<order_no>` | **PASS** | `writeZeroResolutionCAS`；测试 |
| 零元有捕获→不 zero、入队 | **PASS** | `ensureLegacyResolution` 的 `anyCaptured`；测试 |
| 0/多候选 `legacy_payment_resolution(OPEN)` + 24h | **PASS** | `enqueueLegacyResolutionTx`；测试 expiry |
| 终态/COMPENSATING 不重开、不重置 expiry | **PASS** | `:240-270` 只更新 OPEN/RESOLVING；测试 |
| 唯一候选 CAS 单胜 | **PASS** | `writeUniqueLegacyCandidate` 行锁+条件 UPDATE；8 并发测试 |
| 同候选 replay 幂等、不同候选入队 | **PASS** | `current` 分支；测试/代码 |
| 其他 captured intent `pay:<payment_no>:dup` + PENDING→DONE | **PASS** | `refundOtherLegacyCaptures`；测试 |
| 歧义订单不退任何款 | **PASS** | 仅唯一 CAS 胜者调用退款；0/multi 路径无 Refund |
| `AdmitRefund` 正金额 gate | **PASS** | `refund_admission.go:371-401`；`TestLegacyResolutionBlocksAdmissionAndShipping` |
| `AuthorizeShipment` gate | **PASS** | `settlement.go:317-343`；同测试 |
| **零元 `AdmitRefund` gate** | **`[MAJOR]` 缺口** | `refund_admission.go:341` 先走 `admitZeroRefund`，后者 `:650` 要求 `zero:`；历史零元空 settlement 不解析、直接冲突；无测试 |
| 无回填掩盖生产者 | **PASS** | 旧 PAID 行保留；T2.5 未做 |
| 无新字段/迁移/RPC/T3/T4 | **PASS** | `migrations/0019` diff 空；仅 7 files；无新表/字段 |

---

## 3. 残余 / 不声称

- **`[MAJOR]` 零元历史入口**：需让 `source=system_zero` 在 `admitZeroRefund` 前也走 `ensureLegacyResolution`（或在入口统一解析），并补“历史零元 PAID 空 settlement → zero → 零元退款/发货”用例；否则 AC-60 的零元历史退款路径未闭环。非资损（fail-closed），但必须在 T2.4/T2.5/AC-60 验收前修。
- **T2.4–T2.6 未做**：24h worker/SLA/`COMPENSATING`/`EXPIRED_*`、回填 dry-run/计数/重入、`legacy:<order_no>:<seq>` 取号并发；本步只建 OPEN 行并 fail-closed，不声称 SLA/回填闭环。
- **Payment 重复捕获登记**：本步直接 `Payment.Refund`；`payment_duplicate_captures` 登记归 Payment owner，未在真实 Payment 服务验证。
- **0023 CHECK**：测试只应用 0019；`chk_lpr_expiry_24h_v2` 由 R0.4 证据覆盖，本步断言 `TIMESTAMPDIFF=24h`。
- **AC-60/63 不升级**：仍 AC-REF；T2a PASS scoped 不是 AC-VERIFIED/T2 完成/Release。
- 未跑全仓 `make verify`/UAT/prod；无告警/BFF。

---

## 4. INV / 宪法 / Governance

| 项 | 结论 | 依据 |
|---|---|---|
| INV-45 | **PASS** | 不伪造支付身份；空 payment_no 拒绝；其他捕获走支付级退款 |
| INV-46 | **PASS（T2.2/T2.3 部分）** | 唯一候选 CAS、0/多候选 OPEN 24h；worker/终态仍 T2.4 |
| INV-50 | **PASS（边界）** | 零元无捕获 zero；有捕获不 zero；零元 `AdmitRefund` 缺口已列 MAJOR |
| I.1/I.4 | **PASS** | 修生产者根因，不用回填掩盖；无放行分支 |
| I.2 Fail-Closed | **PASS** | List 失败/歧义/未解析均 fail-closed；零元缺口也是 fail-closed |
| I.3 契约优先 | **PASS（有 MAJOR）** | 按 AC-60/63；零元入口未闭环 |
| II.1/II.3 | **PASS** | Payment 读事务外；orders/queue/app 同事务；其他捕获 PENDING 先于远程 |
| III.2/III.4 | **PASS** | database/sql；无新表/RPC/worker |
| IV.1/IV.2 | **PASS（有范围）** | 真实 MySQL/0019、`-race`；T2.4/UAT 未跑已声明 |
| IV.4 | **PASS** | `NOT RELEASE`；不批准 merge/Release |
| Workflow 9 / Governance | **PASS** | T2.1–T2.3 scoped；人类 merge pending |

**`[ARCHITECTURAL_FLAW]`：无。** 零元入口是入口顺序/分支缺口，不是需要推翻解析/CAS 架构；修一次入口即可，不得再散落 if。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-27
**报告**：`docs/collab/qc-reports/REV-PAY-SETTLE-01-T2A.md`
