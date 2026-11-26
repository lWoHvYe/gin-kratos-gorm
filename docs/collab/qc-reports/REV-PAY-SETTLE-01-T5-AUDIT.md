# REV-PAY-SETTLE-01-T5-AUDIT — T5.0 证据审计复审

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Tier | **L2（确认）**：跨服务/退款/履约/资金证据与 AC 分类；不降级 |
| review_scope | **只审 T5.0 审计**（AC-39…68 分类、4 个代码/契约缺口、T5 基线可接受性）；不写码/不改 tsv；不重开 Phase 0-R |
| audit base | **`c56126a`**（审计绑定 HEAD） |
| audit commit / current | **`e2d56c9`**（T5.0 audit；`git diff c56126a..e2d56c9 -- internal migrations` 为空，docs-only） |
| 契约输入 | SPEC-0002 v1.11 AC-39…68；`docs/collab/ac-coverage.tsv`；`tasks.md` Phase 5；R0.3-R2/R0.4e-R2/Phase 0-R closeout；OPS-P5b AC-27 |
| 日期 / Actor / Reviewed-by | 2026-11-27 / `reviewer-r04` / `human (merge pending)` |
| release_status | **NOT RELEASE** |
| overall verdict | **PASS（审计可作 T5 执行基线，需修正 2 处分类/证据）；FAIL（T5 readiness：4 个 `[BLOCKER]`）**；不修改 tsv、不声称 AC-VERIFIED/Release |

---

## 0. 结论

审计的总体方向正确：EVIDENCE-READY 3（AC-50/67/68）、TEST-GAP 17、FEATURE-GAP 10 的盘点，以及 AC-39/41/57/59 四个缺口的发现，均能作为 T5.1–T5.5 的输入；**可接受为 T5 执行基线**。

但必须做两类修正后才能用于 T5 计划：

1. **FEATURE-GAP 10 不是 10 条都依赖 T2/T3/T4**。真依赖 T2/T3/T4 的是 **5 条**：AC-40/60/63（T2）、AC-62/64（T3）；AC-61 是 T5.3/ops；**AC-39/41/57/59 是 T5 代码/契约缺口，不得归 T2/T3/T4 延期**。
2. **AC-57/59 应作为 `[BLOCKER]` 实现/契约缺口**，不是普通 TEST-GAP/FEATURE-GAP；AC-39/41 同样为 `[BLOCKER]`（T5.3/T5.5）。审计中“AC-59 现有乱序用例反而结账”的引用不准确：`refund_consumer_integration_test.go:125` 是“已捕获 10000、累计退款 2000 的旧事件仍收敛 ledger”，不是 AC-59 “退款投影先于捕获事实”的分支；AC-59 仍无实现。

**T5 不能在四缺口修复/补测前 PASS；不得把这些 AC 标 AC-VERIFIED。**

---

## 1. 分类抽查

### 1.1 EVIDENCE-READY 3

| AC | 审计结论 | 复审 | 说明 |
|---|---|---|---|
| AC-50 | E | **PASS** | 已 AC-VERIFIED（OPS-P5b/AC-50 证据）；关键分支（partial/zero/legacy/REVIEW）有测试 |
| AC-67 | E | **PASS** | 零元短路、不调 Payment、S19 DONE、正金额拒绝、回库 gate、无 REVIEW 持久化；`zero_refund_integration_test.go` + `ac50_review_*` |
| AC-68 | E | **PASS（有边界）** | `assertZeroRefunded` 断言 `REFUNDED/effective=0/refunded=0/amount_state=CONSUMED/refund_no=''`；`DONE(zero_amount)` 是展示事实，未新增 DB 值；审计已说明 |

**E3 可接受；AC-67/68 仍需 Reviewer 按精确 HEAD 做最终 AC-VERIFIED，不由审计自动升级。**

### 1.2 TEST-GAP 17

- AC-42/43/44/45/46/47/48/49/51/52/53/54/55/56/58/65/66 的代码路径存在、缺少关键断言，分类基本成立。
- **AC-58**：现有 `delivery_integration_test.go:321` 只断言 receipt 不回滚；AC-58 还要求“进入售后/退款流程”，确实 test-gap。
- AC-51/65 的“部分绿测”不能提升为 E；审计保守处理正确。
- **AC-57/59 不应在 TEST-GAP/FEATURE-GAP 里被模糊处理；按 §2 的 `[BLOCKER]`。**

### 1.3 FEATURE-GAP 10

| AC | 审计分类 | 复审 | 真实依赖/处置 |
|---|---|---|---|
| AC-40 | F | **T2** | legacy/有捕获 intent 的 24h 分支；T2/AC-63 |
| AC-60 | F | **T2** | `legacy:<order_no>:<refund_seq>` 分配/回填 |
| AC-63 | F | **T2** | resolver/worker/backfill（0019 仅表） |
| AC-62 | F | **T3** | D-24 人工释放/审计，T3 |
| AC-64 | F | **T3** | 行/运费台账 T3 |
| AC-61 | F | **T5.3/ops** | 30m 告警/人工重放/日对账部署；不是 T2/T3/T4 依赖 |
| AC-39 | F | **T5.3 `[BLOCKER]`** | 渠道捕获≠pay_cent 无全额退款；不是 T2/T3/T4 |
| AC-41 | F | **T5.3 `[BLOCKER]`** | 非生效捕获/第二 payment 无退款；不是 T2/T3/T4 |
| AC-57 | F | **T5.5 `[BLOCKER]`** | `CREATED` 未拒绝，代码违反 AC；不是 T2/T3/T4 |
| AC-59 | F | **T5.3 `[BLOCKER]`** | 无 `EVENT_OUT_OF_ORDER`；不是 T2/T3/T4 |

**修正：FEATURE-GAP 10 = 5 条 T2/T3/T4 + 1 条 T5.3/ops + 4 条 T5 `[BLOCKER]`。**

---

## 2. 四个代码/契约缺口（`[BLOCKER]`）

| AC | 文件:行号 | 实际代码/测试 | 判定 |
|---|---|---|---|
| **AC-39** | `internal/services/payment/service.go:297-300`（Callback 金额不等于 intent 直接报错）；`internal/services/order/service.go:1341`（paidAmount 只拒超额）；`payment/service.go:91`（CreateIntent） | 请求金额≠pay_cent 有拒绝；**渠道已捕获≠pay_cent 时没有“全额退款”**，仅错误返回；无 `PAYMENT_AMOUNT_MISMATCH` 后的退款/审计链路 | **`[BLOCKER]`**：AC-39 第三分支未实现；T5.3 不可用现有测试标 VERIFIED |
| **AC-41** | `internal/services/order/service.go:604-612`（MarkPaid 已 PAID 他 payment 只 `Conflict`）；无 `DUPLICATE_PAYMENT`/非生效捕获退款实现 | 第二 payment 被拒绝，但**非生效捕获未全额退款**；同 payment 幂等/非生效退款未覆盖 | **`[BLOCKER]`**：AC-41 身份维度未实现；不能以 AC-45 单测代替 |
| **AC-57** | `internal/services/fulfillment/delivery.go:386` | `confirmReceiptTx` 仅拒绝 `CANCELLED`，**未拒绝 `CREATED`**；`ConfirmReceipt` 用户路径会走到 receipt；`TestAC23DirectReceiveRejected` 只测 direct receive | **`[BLOCKER]`**：直接违反 AC-57；T5.5 不能“仅引用既有测试并标 AC-REF”，须先修/补测 |
| **AC-59** | 全仓无 `EVENT_OUT_OF_ORDER`（grep 无实现/常量）；`internal/services/order/refund_consumer_integration_test.go:125` `TestPaymentRefundedOutOfOrderStillSettlesLedger` | 该测试是“已捕获 10000、累计退款 2000 的旧事件仍收敛 ledger”，**不是**“退款投影先于捕获事实”的 AC-59 分支；“乱序仍结账”的引用过宽。AC-59 的退款先于捕获/收货先于发货/缺货解决先于上报均无 `EVENT_OUT_OF_ORDER` | **`[BLOCKER]`**：实现/测试均缺；现有用例不能当 AC-59 证据 |

**四个缺口均为 T5 代码/契约 blocker，不是 T2/T3/T4 延期项。**

---

## 3. T5 基线可接受性 / P0 边界

- **审计可作 T5 执行基线：YES**（E3/T17/F 的盘点可用），但 T5 计划必须：
  1. 将 AC-39/41/57/59 列入 **T5.3/T5.5 blocker 修复**，先修后测；
  2. 将 AC-40/60/63 交 T2、AC-62/64 交 T3，AC-61 留 T5.3/ops；不要把四条 blocker 混入 T2/T3/T4；
  3. 修正 AC-59 证据引用与 FEATURE-GAP 计数；AC-57 不得按 T5.5“仅引用既有测试并 AC-REF”处理。
- **必须在 P0 final closure/Release 前修/补测**：AC-39、AC-41、AC-57、AC-59（均为现有契约缺口/实现缺失；AC-57/59 是既有 AC 违反，不能延后）。Phase 0-R 的 scoped closeout 没有覆盖 AC-39…68 的 T5 缺口；本审计不重开 Phase 0-R，但 T5/P0 final 不得在这些缺口未处理时声称通过。
- **可延期**：AC-40/60/63（T2）、AC-62/64（T3）、AC-61（T5.3/ops）；AC-66 的 shadow 测试在 T4 范围，按 T4 计划。
- **T5.0 不是 AC-VERIFIED**；E3 的 AC-50/67/68 仍需 Reviewer 的精确 HEAD 验收；TEST-GAP/feature-gap 不得被审计“预认证”。

---

## 4. 残余

- 审计的分类修正（F10 计数、AC-59 证据引用、AC-57/59 blocker 归属）需 Lead 在 T5.1/T5.3/T5.5 计划中采纳；本报告只做复审，不改 tsv/代码。
- AC-39/41/57/59 修复需要新测试/实现，属于 T5 受管代码；不在本次只读范围。
- AC-61 的 30m 告警/人工重放/日对账部署证据仍缺；归 T5.3/ops。
- 本报告 `NOT RELEASE`，不声称 P0 closure/Release/merge；Phase 0-R scoped closeout 不因本审计改变。

---

## 5. 宪法 / Governance

| 条款 | 结论 | 依据 |
|---|---|---|
| I.1 拒绝“屎山雕花” | **PASS（审计）** | 审计按 AC 分支分类，未用绿测掩盖 |
| I.2 Fail-Closed | **PASS（审计）** | 缺口标 F/T，不把缺失当 VERIFIED |
| I.3 契约优先 | **PASS（有修正）** | AC-57/59 契约违反被指出；F10 计数需修正 |
| I.4 拒绝自我妥协 | **PASS（审计）** | 四缺口明确回 Plan/实现 |
| II.1 事实分离 | **PASS** | 审计区分 AC-REF/AC-VERIFIED，不整合状态 |
| IV.1 证据不可妥协 | **PASS（抽查）** | 审计只读、真实 MySQL/NATS 子集；未跑全矩阵已声明 |
| IV.4 实现/发布隔离 | **PASS** | `NOT RELEASE`；不批准 merge/Release |
| Workflow 9 / Governance | **PASS** | T5.0 只读审计；Reviewer 出 scoped verdict；人类 merge pending |

**`[ARCHITECTURAL_FLAW]`：无（本审计范围）。** 四缺口是实现/契约缺口，不是需要推翻架构；AC-57/59 必须回 T5.3/T5.5 实现/测试，不得用审计或测试口径掩盖。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-27
**报告**：`docs/collab/qc-reports/REV-PAY-SETTLE-01-T5-AUDIT.md`
