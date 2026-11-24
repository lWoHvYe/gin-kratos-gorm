# PAY-SETTLE-01 T1.4a Developer 自检清单

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Role / Actor | Developer / `dev-phase1c` |
| Plan | `docs/lead/PAY-SETTLE-01/plan.md` + `tasks.md`（Plan approved 2026-11-24；T1.2 已 scoped close） |
| 契约输入 | SPEC-0002 **v1.9**（approved 2026-11-24）；ADR-0007 Amd1（accepted）+ Addendum 1 零元（accepted）+ Addendum 2 T1.2 Rulings（accepted）；C-05 `order/0017`–`0020` |
| Base | `f64434a`（T1.2 scoped close；`wip/pay-settle-01-phase1`） |
| Code head（本清单绑定） | **本 WIP 提交**（`wip(order): T1.4a tcc per-group admission, zero refund short-circuit, 0021`，`git log -1` 即该提交；Base=`f64434a`） |
| Scope | **仅 T1.4a**：TCC batch 逐组准入 + `CancelIfCreated` R-2 语义 + 零元短路 + C-05 `0021` |
| 不做 | Payment.Refund / `payment.refunded` / PAYING/DONE（T1.4b）；`order.refund_requested` consumer（T1.5）；`PENDING_PAYMENT+已捕获`（T1.6）；完整 AC 矩阵/真实迁移全量（T1.7/T5）；T2/T3/T4 |
| Status | **WIP**；不批准、不宣称实现 PASS / Release PASS，不标 AC-VERIFIED |

> 本文件是 Developer 的 canonical 自检交付物。只记录本次实现事实、证据与残余风险，不替代 Reviewer 裁决。

---

## 1. 反妥协自检清单（三问）

### 1.1 并发冲突 / 网络超时 / 部分成功时会发生什么？处理了吗？

- **TCC 逐组准入的并发**：准入阶段以订单行锁 + application `version` CAS + batch 状态 CAS 串行化；`AdmitRefund` 新增“锁内重读 admission”收敛路径（两个同 `refund_id` 的调用不再在 PK 上互相打爆，第二个返回已持久化结果）。`TestTccAdmissionConcurrentIsIdempotent` 8 并发 + `-race`：1 batch、2 application、2 admission、2 commitment、permit 只撤销一次（`version=2`）。
- **远程取消失败/超时**：`CancelIfCreated` 失败时 `AdmitRefund` 返回 `UNAVAILABLE`，application/admission 保持 `PENDING_REMOTE`，batch 保持 `ADMITTING`，`TccCancelMarkPaid` 返回可重试错误（不报成功、不打款）。重跑只补未完成组，commitment/admission 不重复（`TestTccAdmissionCancelIfCreatedFailureKeepsPendingRemote`）。
- **部分成功/崩溃**：Tx1（T1.2）与准入阶段分离；batch `FROZEN→ADMITTING→ADMITTED` 全 CAS。已 `PROCESSING/PRE|POST` 的组在重跑时跳过；`DONE`（T1.4b 将来）永久跳过。`TestTccCancelResumeFillsMissingApplicationsAndPermits` 与 `TestTccAdmissionNonePermitResolvesPre` 的重复调用覆盖。
- **零元短路的并发**：permit 决策在事务外完成（`CONSUMED` 需要远程 `CancelIfCreated`，不能进事务），随后单事务锁订单行写 admission + `refund_applications(DONE)`。`NONE→CONSUMED` 竞态（Ship 抢先）通过重试哨兵重新走 `CONSUMED` 分支，**不会把已授权/已离仓组盲写成 PRE**；同一 `refund_id` 并发由订单锁 + 锁内重读 application/admission 收敛。
- **零元短路失败**：不写任何 S19 行（单事务），不留下无 worker 的 `PENDING` 悬挂；调用方（AfterSales T1.4b）保持非终态重试。
- **人工态**：`MANUAL_HOLD/FAILED_PERMANENT` 的 batch 不进入准入阶段（Tx1 已 fail-closed；`admitTccBatch` 同样返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`）；application 的 `CANCELLED/REJECTED/FAILED_PERMANENT` 不自动推进。
- **未知/歧义**：未知 batch 状态、未知 permit 状态、`CancelIfCreated` 未知返回、application 结果与 batch 事实冲突，全部 fail-closed 返回错误，不默认放行。

### 1.2 是否把状态揉进一个字段 / 加 if-else 绕过架构？

- 没有新增状态字段：batch 使用 ADR §6.1 既有 `ADMITTING/ADMITTED`；application 使用既有 `PENDING/PROCESSING/DONE`；admission 使用既有 `PRE_SHIP_REVOKED/POST_SHIP_ALLOWED/PENDING_REMOTE`。`source=system_zero + amount=0 + DONE` 是值组合，未新增 `DONE_ZERO`。
- 没有第二套许可裁决：TCC 准入对 `PRE/PENDING_REMOTE` 组调用**同一个** `AdmitRefund`；只有 `POST` 组按 T1.2 已持久化的 `admission_result` 直接登记（见 §4-2 的边界说明），避免对 legacy G1 / exception+shipped 组创建 NONE permit 并降级为 PRE。
- 订单 lifecycle 不变：零元短路不写 `orders.status`/`captured`/`refunded`；TCC 仍只写 `refund_state=REFUNDING`（T1.2 行为）。
- 未改 proto、未新增 RPC、未引入新依赖/新表；`0021` 只加 1 个 CHECK，未改 `0017`。

### 1.3 向现实做了哪些妥协（重大妥协高亮）

1. **【重大】T1.4a 成功 ≠ 已退款**：本任务不调用 `Payment.Refund`，不消费 `payment.refunded`，不把 batch 推到 `PAYING/DONE`。`batch=ADMITTED` 只表示“全部活动组已有准入结果”，钱尚未退。WIP 不得合入 main/不发布。
2. **【重大】POST 组的 commitment 在准入阶段创建**：T1.2b 的 Tx1 对已离仓组只写 application（Addendum 2 R-2 §3.3）；本任务在 `recordTccPostAdmission` 写 `order_refund_admissions(POST)` + `RESERVED` commitment，以便 T1.4b 打款前已占用额度、并发售后不能重复占用。R-2 §3.3 的“不写 commitment”按 T1.2b（Tx1）范围理解；若 Reviewer 认为 POST 组也不得创建 commitment，则 T1.4b 的额度门禁需要重新裁决（**需 Reviewer/Architect 确认，见 §5-2**）。
3. **【重大】零元“无捕获”的判据是 Order 侧 `zero:<order_no>`**：AC-67 明确零元分支“不调用 Payment”，因此本实现以 `pay_cent=0 && settlement_payment_no == zero:<order_no>`（SettleZero 在 Payment 确认无捕获后才写）作为“无任何已捕获 intent”的 Order 侧证明，不额外查 Payment。若零元结算后出现迟到的正金额捕获，必须由 AC-40/AC-63 的支付级退款路径收敛（T1.5/T1.6/T2 范围），零元短路不会把它转成正金额退款（**假设，见 §5-3**）。
4. **零元 `CONSUMED` 的远程取消在事务之前**：为了让 S19 `DONE` 与 admission 在同一事务落库（AZ-05“不保留无 worker 的 PENDING”），`CONSUMED` 组先 `CancelIfCreated` 再开事务；取消幂等，事务失败时不存在 S19 行，重试会再次幂等取消。这与正金额路径的 `PENDING_REMOTE` 两阶段不同，是有意的零元简化。
5. **INV-44 释放不在 `AdmitRefund`**：本实现仍只做 permit/运单准入；Order 的 `AdmitRefund` 从未调用 Inventory 释放，`0021`/T1.4a 也未新增该调用。INV-44 的释放/回库仍由 Fulfillment/AfterSales 消费者与 T3 负责（**契约缺口，见 §5-1**）。
6. **无 worker/新任务表**：准入阶段由 TCC 补偿入口（DTM `TccCancelMarkPaid` 重试）驱动，与 T1.3 相同；未新增 lease/worker 基础设施，`refund_batches` 的 `lease_owner/lease_until` 未被本任务使用（T3/T1.4b 如需再启用）。

---

## 2. [修改点] / [架构侵入性] / [副作用评估]

### 2.1 [修改点]

| 文件 | 修改 |
|---|---|
| `internal/services/order/refund_admission.go` | `AdmitRefund` 增加 `source=system_zero && amount=0 && pay_cent=0 && zero:<order_no>` 短路：跳过 Payment/commitment，permit/`CancelIfCreated` 准入，同事务写 `order_refund_admissions` + `refund_applications(source=system_zero, amount=0, state=DONE)`，返回 `DONE`；正金额路径不变。新增锁内 admission 重读（同 `refund_id` 并发幂等）。 |
| `internal/services/order/tcc_admission.go`（新增） | TCC batch 逐组准入：`FROZEN→ADMITTING`、`PRE/PENDING_REMOTE→AdmitRefund`、`POST→recordTccPostAdmission`、全部成功 `ADMITTING→ADMITTED`；失败保持 `ADMITTING` 并可重试；只补未完成组；不调用 Payment。 |
| `internal/services/order/tcc.go` | `TccCancelMarkPaid` 在 Tx1 成功后调用 `admitTccBatch`；失败返回可重试错误。 |
| `internal/services/order/tcc_settlement.go` | batch 状态常量（`ADMITTING/ADMITTED/PAYING/MANUAL_HOLD/FAILED_PERMANENT`）；`findTccBatchForResume` 使用常量；`ensureTccApplicationTx` 接受 `PROCESSING`（T1.4a 已准入、待 T1.4b 打款）。 |
| `internal/services/fulfillment/cancel_if_created.go` | R-2 §3.2.5：读取 `exception_state`/`shipped_at`；exception+shipped → `ALREADY_SHIPPED`（不取消）；exception 无 shipped_at → `TRANSIENT_ERROR` + 告警（不取消）；其余分支不变。 |
| `migrations/order/0021_zero_application_check.sql`（新增） | `refund_applications` 加 `chk_refund_app_system_zero`：`source='system_zero'` 时 `amount=goods=freight=0 AND non_effective_capture=0`；条件式 ALTER，可重跑；未改 `0017`。 |
| `internal/services/order/settlement_integration_test.go` | 测试库执行真实 `order/0021`；注释更新。 |
| `internal/services/order/tcc_compensation_integration_test.go` | 新增 `runTccTx1` 辅助，把 T1.2 的 Tx1-only 断言保留（`FROZEN`/`PENDING_REMOTE`/无 admission+commitment）；批量状态断言改为 `ADMITTED`（T1.4a 后的真实端到端状态）；并发/PENDING 相关断言按 T1.4a 扩展。 |
| `internal/services/order/refund_admission_integration_test.go` | fake fulfillment 增加 `cancelCalls`/`failCancels`；新增零元短路、零元 CONSUMED、负例、`0021` CHECK 测试。 |
| `internal/services/order/tcc_admission_integration_test.go`（新增） | TCC NONE→PRE、CONSUMED+CREATED→PRE、CONSUMED+SHIPPED→POST（两种时序）、取消失败→PENDING_REMOTE 重试、并发幂等、legacy G1 shipped POST。 |
| `internal/services/fulfillment/cancel_if_created_test.go` | exception+shipped、exception 无 shipped_at、RECEIVED 用例。 |

### 2.2 [架构侵入性]

- 新增 1 个准入阶段与 1 个零元分支；无“默认放行”if。TCC POST 组使用专用登记函数（原因见 §1.3-2 / §5-2），未复制许可裁决。
- 状态所有权不变：Order 唯一写 `refund_applications`/`order_refund_admissions`/commitment/permit；Fulfillment 仍是运单事实 owner；Payment 仍只读（零元不调用）。
- 未改订单 lifecycle、未改 `order.refund_requested` consumer、未发该事件、未调用 `Payment.Refund`。
- `0021` 是纯约束迁移；`0017` 未改。

### 2.3 [副作用评估]

- **TCC 端到端行为扩大**：Tx1 之后现在会写 admission/commitment 并把 batch 推到 `ADMITTED`；原先 T1.2 测试中“Tx1 后 admission=0/commitment=0”的断言改为直接测 Tx1（`runTccTx1`）以保留原证据，另由 T1.4a 测试覆盖端到端。
- **`ADMITTED` 不等于可打款**：T1.4b 必须只对 `ADMITTED` batch 进入 `PAYING`；`ADMITTING` 表示仍有未完成组。
- **Fulfillment `CancelIfCreated` 收紧**：exception+shipped 从“可能取消 CREATED 运单”改为 `ALREADY_SHIPPED`；这会让既有 `shop.order.refund_admitted.v1` PRE 消费者在“PRE 但已离仓”时记录 error 日志（原有行为），但不会回滚履约。
- **零元售后**：只有 `source=system_zero + amount=0` 走新路径；`source=aftersales` 的 0 元老路径保持不变（`TestAdmitRefundZeroAmountOrder` 仍绿）。
- **测试库**：`orderTestDB` 现在同时执行 `0017/0018/0021`；任何违反 `0021` 的写入会立即失败。

---

## 3. 测试证据（真实 MySQL；绑定本 WIP 提交，Base=`f64434a`）

环境：`go1.27.1 darwin/amd64`；MySQL `mysql-unicorn`（`127.0.0.1:3306`，`26.10.0`）；`TEST_MYSQL_DSN` 由 `scripts/test-dsn.sh` 按包派生（避免并发测试互相 DROP 共享库）；`GOCACHE=/tmp/dsh-go-cache`。

| 命令 | 结果 |
|---|---|
| `TEST_MYSQL_DSN=<order> go test ./internal/services/order -count=1` | `ok ... 88.819s` |
| `TEST_MYSQL_DSN=<fulfillment> go test ./internal/services/fulfillment -count=1` | `ok ... 9.768s` |
| `TEST_MYSQL_DSN=<aftersales> go test ./internal/services/aftersales -count=1` | `ok ... 19.159s`（跨服务消费面回归） |
| `TEST_MYSQL_DSN=<order> go test -race ./internal/services/order -run 'TestTccAdmissionConcurrent...\|TestTccCancelConcurrent...\|TestAdmitRefundSystemZero...\|TestTccAdmissionCancelIfCreatedFailure...' -count=1` | `ok ... 11.801s` |
| `TEST_MYSQL_DSN=<order> go test -v ./internal/services/order -run 'TestTccAdmission\|TestAdmitRefundSystemZero\|TestZeroApplicationCheck\|TestAdmitRefund*' -count=1` | 全部 PASS（见 §3.1） |
| `TEST_MYSQL_DSN=<fulfillment> go test -v ./internal/services/fulfillment -run TestCancelIfCreated -count=1` | 全部 PASS（见 §3.2） |
| `go build ./...` | OK |
| `go vet ./internal/services/order ./internal/services/fulfillment` | OK |
| `gofmt -l <changed files>` | 空（`internal/services/order/service.go` 的既有漂移未触碰，`make fmt-check` 基线即失败） |
| `make migrations-check` | `[migrations] OK` |

### 3.1 新增/受影响测试（真实 MySQL，逐条 PASS）

```text
--- PASS: TestAdmitRefundPreShipAndIdempotent (0.83s)
--- PASS: TestAdmitRefundConsumedPermitTwoPhase (1.05s)
--- PASS: TestAdmitRefundZeroAmountOrder (0.79s)
--- PASS: TestAdmitRefundSystemZeroShortCircuit (0.79s)
--- PASS: TestAdmitRefundSystemZeroConsumedPermit (1.78s)  # cancelled / already shipped
--- PASS: TestAdmitRefundSystemZeroRejectsMismatchedFacts (0.76s)  # positive / non-zero order / no zero identity
--- PASS: TestZeroApplicationCheckConstraint (0.67s)  # 0021 negative + valid row
--- PASS: TestTccAdmissionNonePermitResolvesPre (1.21s)
--- PASS: TestTccAdmissionConsumedCreatedCancelIfCreatedPre (1.04s)
--- PASS: TestTccAdmissionConsumedShippedPost (2.04s)  # shipped in Tx1 facts / leaves between Tx1 and admission
--- PASS: TestTccAdmissionCancelIfCreatedFailureKeepsPendingRemote (1.01s)
--- PASS: TestTccAdmissionConcurrentIsIdempotent (1.69s)
--- PASS: TestTccAdmissionLegacyG1ShippedPost (1.07s)
```

`TestTccAdmission*` 的断言包含：batch `ADMITTED`；application `PROCESSING/PRE|POST`；`order_refund_admissions` 的 result/state/amount；`order_refund_commitments` 恰 1 条/组；permit `REVOKED`（NONE 组）或保持 `CONSUMED`（CONSUMED 组，version 不变）；`order.refund_requested` = 0；重复/并发不产生第二套行。`paymentPort` 接口没有 `Refund` 方法，T1.4a 不可能调用打款（编译期约束）。

### 3.2 Fulfillment（R-2）

```text
--- PASS: TestCancelIfCreatedOnlyCancelsCreated (0.62s)
--- PASS: TestCancelIfCreatedRefusesExceptionAfterShipment (0.50s)
```

### 3.3 真实迁移 `0021`（非测试内联 DDL）

在全新库 `mall_t14a_migration` 依次执行真实 `migrations/order/0017_refund_batch_ledger.sql`、`migrations/order/0021_zero_application_check.sql`：

```text
== apply migrations/order/0017_refund_batch_ledger.sql ==
== apply migrations/order/0021_zero_application_check.sql ==
== constraint ==
chk_refund_app_system_zero   CHECK
...
== re-run 0021 (idempotent) ==  (成功，无重复约束)
== negative: system_zero amount=100 ==
ERROR 3819 (HY000): Check constraint 'chk_refund_app_system_zero' is violated.
== positive: system_zero amount=0 DONE ==
R-OK  system_zero  0  DONE
```

（`orderTestDB` 也会在**每个** order 集成测试前执行该真实迁移，作为持续回归。）

---

## 4. 【测试范围】与【未覆盖范围】

### 4.1 【测试范围】

- TCC：`NONE→REVOKED/PRE`、`CONSUMED+CREATED` → `CancelIfCreated` → PRE、`CONSUMED+SHIPPED`（含 T1.2 已判 POST 与准入时才发现 ALREADY_SHIPPED 两种时序）→ POST、`CancelIfCreated` 失败 → `PENDING_REMOTE` 重试、并发/重复幂等、legacy G1 shipped POST、Tx1-only 回归。
- 零元：短路 `DONE`、不调 Payment、无 commitment、permit REVOKED、CONSUMED+CANCELLED/ALREADY_SHIPPED、重复幂等、正金额/非零订单/无 zero identity 负例。
- 迁移：`0021` 真实执行、重跑幂等、CHECK 负例/正例、`migrations-check`。
- Fulfillment：R-2 exception+shipped、exception 无 shipped_at、RECEIVED。
- 跨服务非回归：`aftersales` 全包（其 `AdmitRefund` 调用面）。

### 4.2 【未覆盖范围】（显式不在 T1.4a）

1. `Payment.Refund`、`payment.refunded` 收敛 `commitment SETTLED`、batch `PAYING/DONE`、`MANUAL_HOLD/FAILED_PERMANENT` 的人工/自动推进 → **T1.4b**。
2. `order.refund_requested` consumer 改为观察/告警 → **T1.5**。
3. `PENDING_PAYMENT+已捕获` 的 `CAS→CANCELLED` + `pay:<payment_no>:cancel` → **T1.6**（T1.2b guard 仍在）。
4. T1.7 的全矩阵（Σ 不等、Ship 竞态、崩溃续跑、部分失败、无组 G1、故障注入）与 T5 的 AC 证据矩阵/真实迁移全量/回填 → **T1.7/T5**。
5. AfterSales 零元端到端（回库顺序、`REFUNDED/0/CONSUMED/refund_no=''` 投影、任务重试）→ **T1.4b/T3**；本任务只提供 Order 侧 `AdmitRefund` 短路。
6. 行级/责任/运费（INV-47）、`UNDETERMINED/168h`、`order.settled` shadow、Inventory/Fulfillment 依赖 fail-fast、`complement` → T3/T4。
7. **INV-44 释放/回库**：`AdmitRefund`（含零元短路）不做 Inventory 释放；这是既有实现缺口（非本任务引入），见 §5-1。
8. 指标/告警：仅 `s.log` 日志，未新增 metrics（仓库无 Order counter 注册点）。

---

## 5. 契约缺口 / 需裁决事项（不自行扩展）

1. **INV-44 释放 owner 未落在 `AdmitRefund`**：SPEC-0002 AC-50/INV-44/零元 Addendum AZ-04 要求准入后释放预留，但代码事实是 `AdmitRefund` 只写 permit/admission/commitment，从未调用 Inventory；Fulfillment/AfterSales 也没有消费 `order.refund_admitted` 做释放（`admission_consumer.go` 只取消运单）。本任务未新增 Inventory 调用（避免在 T1.4a 自创跨服务协议/依赖）。需 Architect 明确 INV-44 的调用点与 owner（T3.3/T4 或 AfterSales/Fulfillment consumer）。
2. **POST 组 commitment 的创建时机**：Addendum 2 R-2 §3.3 说 T1.2b 不得为已离仓组写 commitment；本任务在准入阶段写 `RESERVED` commitment（否则 T1.4b 打款前额度未占用，并发售后可超退）。若 Reviewer 认为该约束覆盖 T1.4a，则需要重新裁决“POST 组额度占用”由谁在哪一步完成（**本实现按 ADR Rev1 §5.2 + INV-47 的额度要求处理**）。
3. **零元“无捕获”证明的时点**：AC-67 禁 Payment 调用；本实现以 `zero:<order_no>` 作为 Order 侧证明，不重查 Payment。零元结算后的迟到正金额捕获必须由 AC-40 的支付级退款/legacy 路径（T1.5/T1.6/T2）收敛，零元短路不会探测它。
4. **T1.4b 的 batch 门禁**：T1.4b 必须只从 `ADMITTED` 进入 `PAYING` 且只在“全部活动组 `PROCESSING/PRE|POST`（或 `DONE`）”时打款；`ADMITTING` 是未完成态。本任务未实现 `PAYING`，也未把 `MANUAL_HOLD` 的自动推进补上。
5. **Fulfillment `order.refund_admission_recheck`**：ADR Rev1 §5.2 提到 PRE 后 consumer 发现 `ALREADY_SHIPPED` 时应告警并触发 recheck；当前 consumer 只记 error 日志（既有实现），本任务未新增该事件/流程（T1.5/T3 范围）。
6. **`make fmt-check` 基线失败**：`internal/services/order/service.go` 在 base 提交即未 gofmt（非本任务修改）；本任务所有改动文件 `gofmt -l` 为空。若 CI 以 `make fmt-check` 为门禁，需要单独修该文件（避免混入本次 diff）。
7. **迁移环境**：真实迁移在 compose MySQL `26.10.0` 上执行；生产 MySQL 版本未在本任务验证（由 release 门禁负责）。

---

## 6. 结论

- 本次交付 **仅 T1.4a**：TCC 逐组准入、`CancelIfCreated` R-2 语义、零元短路、C-05 `0021`。
- 未调用 `Payment.Refund`、未发 `order.refund_requested`、未改 consumer、未实现 worker/PAYING/DONE。
- **Status: WIP**；不批准、不宣称实现 PASS / Release PASS，不标 AC-VERIFIED；交由 Reviewer 按精确 base/head 复核，T1.4b 再衔接打款。
