# PAY-AC50-REVIEW-01 T0+T0b+T0c 自检清单

| 项 | 值 |
|---|---|
| change_id / Parent | `PAY-AC50-REVIEW-01` / `PAY-SETTLE-01`（冻结） |
| Role / Actor | Developer / `dev-r04-fix` |
| Tier | **L2**（跨服务 RPC 响应语义、资金/准入、状态机；Plan 已批准 P5 2026-11-26） |
| 契约输入 | SPEC-0002 **v1.11 approved**（AC-50/AC-69/AC-70/AC-72）；ADR-0007 AC-50 Addendum **0.6 errata**（QC R9 READY、人类 errata 批准 2026-11-26）；impact 0.4；Plan/Tasks（P5 后） |
| Base | `a34c845`（C-5 errata + P5 批准） |
| **Code head（全部证据绑定）** | **`e374384107ca17c266054033c448eb33981445d7`** |
| 范围 | T0（Fulfillment `<NO_WAYBILL>` + proto/生成物）、T0b（Order REVIEW 持久化/幂等/零元/legacy）、T0c（Tx2 双谓词 CAS） |
| allowed_paths（实际改动） | `internal/services/fulfillment/cancel_if_created.go`(+test)、`internal/services/order/refund_admission.go`、`api/proto/mall/v1/mall.proto`、`api/gen/mall/v1/mall.pb.go`（仅 `make proto` 生成）、两个新增 `_test.go`、本文档 |
| forbidden_paths（未触碰） | `migrations/**`、`internal/services/order/{tcc*.go,settlement.go,ports.go}`、AfterSales、`internal/platform`、gateway、deploy、frontend、docs/specs、docs/adr、docs/lead、docs/collab、台账 |
| Status | **WIP**；不声称 AC-VERIFIED / Release PASS / merge；T1–T8 另派 |

## 1. Diff（`e374384`，7 files，+807/−24）

```
api/gen/mall/v1/mall.pb.go                         |  11 +-   (make proto，仅注释)
api/proto/mall/v1/mall.proto                       |  11 +-
internal/services/fulfillment/cancel_if_created.go |  16 +-
.../services/fulfillment/cancel_if_created_test.go |  72 ++++
internal/services/order/ac50_review_integration_test.go | 310 +++++
internal/services/order/ac50_review_race_test.go   | 210 +++++
internal/services/order/refund_admission.go        | 201 ++++--
```

- `git diff e374384^ e374384 -- migrations` 为空；无新字段/RPC/worker。
- `make proto` 可执行且**仅注释 diff**：`protoc 36.2` + `protoc-gen-go v1.36.12` + `protoc-gen-go-grpc v1.5.1`（与已提交生成头一致）；`make proto-guard` → `[proto-guard] OK (1002 baseline entries)`。

## 2. T0（Fulfillment）

- `cancel_if_created.go`：`sql.ErrNoRows` → **`NO_WAYBILL`**（`statusNoWaybill` 常量），不建行、不 commit；真实 `CREATED→CANCELLED`/已 `CANCELLED` 仍 `CANCELLED`；`SHIPPED/RECEIVED/EXCEPTION/TRANSIENT_ERROR` 不变；`shipment_no` 收窄范围与组隔离不变。
- proto：`CancelIfCreatedReply.status` 注释新增 `NO_WAYBILL` 及五值语义（无字段/枚举/RPC 变化）。
- 测试（`cancel_if_created_test.go`）：F1 无行=`NO_WAYBILL` 且无新行；F1b `shipment_no` 无行=`NO_WAYBILL` 且不误取消同组 CREATED；F6 G1 探测不动 G2；F2 CREATED→CANCELLED；F3 幂等 CANCELLED；F4 SHIPPED/RECEIVED；F5 EXCEPTION±`shipped_at`；F8 正常路径。
- 注：ADR/Plan 的 `<NO_WAYBILL>` 是占位记法，wire token 取 **`NO_WAYBILL`**（大写、无尖括号，与既有 status 词形一致）。若 Lead/PO 要字面尖括号，仅需改 Fulfillment 常量 + Order `probeStatusNoWaybill` + proto 注释三处。

## 3. T0b（Order 映射 / 幂等 / 零元 / legacy）

- `finishRemoteAdmission`：`NO_WAYBILL` → Tx2 写 `result=REVIEW_REQUIRED,state=PENDING_REMOTE`；**不写 PRE/POST、不写终态 outbox、不 release、不调 `Payment.Refund`**；permit 保持 `CONSUMED`，commitment 保持 `RESERVED`；未知/未来 status → `Unavailable`（永不 PRE）。
- 裁决错误：`apperr.Failed`（FAILED_PRECONDITION），message 含 `REFUND_ADMISSION_CONFLICT`；**结构化 details**：`refund_id/order_no/group_no/permit_state/probe_status/result/state/reason/ticket=RVW:<refund_id>/manual_queue=AC-27-NOWB-01`（typed，不从 message 解析）。
- existing-first：在任何 `Payment.GetIntent`/远程探测之前读裁决；`review→同一冲突`、`terminal→返回已存结果`、`pending→续跑`、`unknown→Conflict`（单一 `classifyExistingAdmission`）。O2b/O2c：Payment 不可用仍返回同一冲突且 `getCalls==0`、`cancelCalls==0`。
- 零元（AC-67）：`CONSUMED+NO_WAYBILL` → 返回冲突，`review_persisted=false`；**不写 admission/S19/DONE/REVIEW、不计 AC-72、不调 Payment**。
- legacy（AC-70）：无 permit 行→`NONE→REVOKED` PRE（不探测）；CONSUMED+无行→REVIEW；CONSUMED+`ALREADY_SHIPPED`→POST。

## 4. T0c（Tx2 双谓词 CAS）

```sql
UPDATE order_refund_admissions
   SET result = ?, state = ?, version = version + 1, updated_at = NOW(3)
 WHERE refund_id = ? AND state = 'PENDING_REMOTE' AND result <> 'REVIEW_REQUIRED'
```

- `RowsAffected==0` → 只读回读：REVIEW→同一冲突；PRE/POST→该结果；nil→`Conflict("admission disappeared during Tx2")`；PENDING 无复核→`Conflict` retry。**所有 0 行路径不写 outbox/release**（O11c）。
- REVIEW 行因 `result <> 'REVIEW_REQUIRED'` 永不匹配：迟到 `CANCELLED`/并发 Tx2 不能把 REVIEW 洗成 PRE（O11d/W13）；并发 `<NO_WAYBILL>` 只留 1 条 REVIEW（O11/W5）；并发 `CANCELLED` 只写 1 条终态 outbox + 1 条 release（O11b）。

## 5. 命令与结果（真实 MySQL 8.4.13，disposable `mysql:8.4` @127.0.0.1:13335）

```bash
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:13335)/mall_integration_t0_order?parseTime=true&loc=UTC' \
  go test ./internal/services/order -run 'TestAC50Review' -count=1 -v        # PASS 8.465s
TEST_MYSQL_DSN='.../mall_integration_t0_order...' \
  go test -race ./internal/services/order -run 'TestAC50Review' -count=1     # ok 8.974s
TEST_MYSQL_DSN='.../mall_integration_t0_ff...' \
  go test ./internal/services/fulfillment -run 'TestCancelIfCreated' -v      # PASS 1.622s
TEST_MYSQL_DSN='.../mall_integration_t0_order...' go test ./internal/services/order -count=1        # ok 165.180s
TEST_MYSQL_DSN='.../mall_integration_t0_ff...' go test ./internal/services/fulfillment -count=1     # ok 9.464s
make proto-guard                                                            # OK (1002 entries)
```

- 逐用例：O1/O2/O2b/O2c/O7/O10/O11/O11b/O11c/O11d 全部 PASS；既有 `TestAdmitRefund*`（18 个）PASS；全包 PASS（无 SKIP）。
- 日志：`.tmp/evidence-ac50-t0/{HEAD.txt,order.log,order-race.log,order-full.log,fulfillment.log,fulfillment-full.log,proto-guard.log}`（HEAD `e374384`）。

## 6. 【测试范围】/【未覆盖范围】

【测试范围】Fulfillment 无行/真实取消/组隔离/proto 注释生成；Order REVIEW 持久化、typed details、无 PRE/POST/释放/Refund、permit/commitment 保持、existing-first 幂等（含 Payment 不可用）、零元不持久化、legacy 三分支、双谓词 CAS 的并发/迟到/0 行路径；全包回归与 `-race`；无迁移与 proto-guard。

【未覆盖范围】

- **T1（TCC）**：`refund_batches.MANUAL_HOLD`/`last_error`/`ensureTcc*` 未改（另派）；本提交遇到 `source=tcc` 时只写 admission REVIEW，**不会置批**。
- **T2（AfterSales）**：canonical `last_error` 镜像、8 次不 `FAILED_PERMANENT` 未改；本提交不会写 task 镜像。**部署提示**：T2 落地前，普通售后 REVIEW 仍走既有 8 次→`FAILED_PERMANENT` 行为，因此本 commit 不应单独发布到生产（Plan P5 已把 T1/T2 列为后续）。
- **T3/T4/T5**：部分金额三分、零元人工队列终态、审计/metrics/ops runbook 未交付。
- **NATS/consumer F7/F7b**（T6b）：未在本单元执行；`admission_consumer` 未改（只认 `result=PRE_SHIP_REVOKED`）。
- UAT/staging/prod 未执行；无真实运维窗口证据。
- `<NO_WAYBILL>` 与 `NO_WAYBILL` 的字面选择见 §2 注。

## 7. 反妥协自检 / 返工影响声明

- 并发/超时/部分成功：Tx2 双谓词 CAS + 0 行只读回读；并发 `<NO_WAYBILL>`/`CANCELLED`/迟到 `CANCELLED`/admission 被删均有测试；REVIEW 不写 outbox/release、不调 Payment；unknown 永远 fail-closed。
- 是否加 if-else 绕过架构：未新增“放行”分支；existing 判定收口为单一 classifier；映射只有一个 `finishRemoteAdmission` 入口；未改 permit/commitment/outbox 既有语义（REVIEW 除外）。
- 妥协（高亮）：① wire token 取 `NO_WAYBILL`（ADR 占位记法，如需字面尖括号改 3 处常量/注释）；② T0 单元先于 T1/T2 交付，**不得单独上生产**（TCC 持批、售后镜像/8 次行为仍未实现）；③ 未跑 NATS consumer 用例（T6b）。

- `[修改点]`：`cancel_if_created.go`、`mall.proto`（注释）、`mall.pb.go`（生成）、`refund_admission.go`、`cancel_if_created_test.go`、新增 `ac50_review_integration_test.go`/`ac50_review_race_test.go`、本文档。
- `[架构侵入性]`：无新状态值/字段/RPC/迁移；REVIEW 复用既有 `result/state` 与 `MANUAL_HOLD`；单一映射权威与单一 existing classifier。
- `[副作用评估]`：旧 Order + 新 Fulfillment → 未知 status `Unavailable` fail-closed；新 Order + 旧 Fulfillment → 无行仍 `CANCELLED` → 仍写 PRE（缺陷保持），因此**同一发布单元**部署；TCC/售后在 T1/T2 前的行为见上。

## 8. 不声称

不声称实现 PASS / R0.4e / merge / Release PASS / AC-VERIFIED；AC-50/AC-69/AC-70 仍待 Reviewer 绑定证据；AC-72 后端段待 T5；不关闭 AC-27/Phase 0-R；`PAY-SETTLE-01` 保持冻结。

---

## 9. T1+T2 自检（追加；code head `ff7a847abf07b0e81f7e1bb9d5c0c3f11043c464`，base `ff0d912`）

| 项 | 值 |
|---|---|
| 范围 | T1a–T1e（TCC）、T2b/T2c（AfterSales）、T2a（existing-first 复核，T0 已实现） |
| Code head | **`ff7a847`**（8 files，+606/−13） |
| 证据 | `.tmp/evidence-ac50-t1t2/`（HEAD 文件、targeted/full/race 日志），真实 MySQL 8.4.13 |

### 9.1 T1（TCC）

- **T1a** `ensureTccRemoteAdmissionTx`：`REVIEW_REQUIRED` 进入合法持久值集合（不再 “not a legal value” 早失败，不覆盖行/回滚）。
- **T1b** `ensureTccApplicationTx`：单调检查接受 `REVIEW_REQUIRED`，已复核 application 不因重算 `PENDING_REMOTE` 判冲突。
- **T1c** `refund_admission.go` 的 REVIEW Tx2 同事务写：`refund_batches.state=MANUAL_HOLD` + canonical `last_error`（`REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:<refund_id>`，1024 列宽 D-35 截断）；**仅当 `refund_applications` 行已存在**时镜像 `admission_result=REVIEW_REQUIRED`（state 保持 PENDING）。`writeTccBatchGroupsTx` 对 `MANUAL_HOLD` 在**任何写入前**短路返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`；`admitTccApplication` 对已持久化 REVIEW 视为自动终止（re-arm 感知：以权威 admission 行为准，admission 被 re-arm 为 PENDING 时继续重评）；`admitTccBatch` 失败后重读 batch，返回稳定 token。
- **T1d/W12**：MANUAL_HOLD 重试零状态变化（batch/application/admission `version` 不变、0 打款）。
- **T1e/O16**：ops 审计 re-arm（`result→PENDING_REMOTE` + `batch MANUAL_HOLD→ADMITTING` 的 version CAS）后同 `batch_id`/`refund_id` 续跑：探测 `CANCELLED→PRE`、admission PRE、application PROCESSING/PRE、恰好 1 次 `RefundBatch`、canonical `last_error` 保留。
- 未改 `tcc.go`/`tcc_payment.go`/`tcc_precheck.go`/`tcc_reconcile.go`；无新状态值/表/迁移/RPC。

### 9.2 T2（AfterSales）

- **T2b/O19/O23**：`workers.go` 从 **typed details** 识别 REVIEW（`apperr.Error` 与 gRPC `ErrorInfo` 两路；禁止 message 解析）；写 `after_sale_refund_tasks.last_error` = canonical `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>`（512 列，D-35 截断 + 单一 parser；未知 key/空 ticket/`arm=`/冒号/尾随空格判不可解析）；测试用 `apperr.GRPC(...)` 证明生产 gRPC 路径。
- **T2c/O22/O22b**：REVIEW 任务始终保持 `PENDING`；8 次后**不自动 `FAILED_PERMANENT`**，扫描器在 `attempts>=8` 且 canonical REVIEW 时不再自动 claim（任务转人工队列）；**非 REVIEW** 8 次仍 `FAILED_PERMANENT`（回归用例通过）。
- **T2a**：existing-first 幂等为 T0 的 `AdmitRefund` 前置裁决读取（Payment 前），O2/O2b/O2c 在 T0 证据与本次 TCC 重放用例中再次覆盖（`cancelCalls/getCalls` 不增、`refundCalls==0`）。

### 9.3 命令与结果（HEAD `ff7a847`）

```bash
TEST_MYSQL_DSN='.../mall_integration_t1_order...' go test ./internal/services/order \
  -run 'TestTCCReview|TestAC50Review' -count=1 -v      # 10 PASS, ok 8.622s
TEST_MYSQL_DSN='.../mall_integration_t1_order...' go test -race ./internal/services/order \
  -run 'TestTCCReview' -count=1                        # ok 3.122s
TEST_MYSQL_DSN='.../mall_integration_t1_as...' go test ./internal/services/aftersales \
  -run 'TestReview' -count=1 -v                        # 4 PASS, ok 2.977s
TEST_MYSQL_DSN='.../mall_integration_t1_as...' go test -race ./internal/services/aftersales \
  -run 'TestReview' -count=1                           # ok 3.476s
TEST_MYSQL_DSN='.../mall_integration_t1_order...' go test ./internal/services/order -count=1    # ok 160.306s
TEST_MYSQL_DSN='.../mall_integration_t1_as...' go test ./internal/services/aftersales -count=1  # ok 26.002s
```

- 日志：`.tmp/evidence-ac50-t1t2/{HEAD.txt,order-targeted.log,tcc-race.log,aftersales.log,aftersales-race.log,order-full.log,aftersales-full.log}`。

### 9.4 【测试范围】/【未覆盖范围】

【测试范围】TCC REVIEW 合法读回/持批/0 打款/只阻塞本批/自动续跑 token/re-arm 续跑/重放 typed O2b/O2c；AfterSales typed gRPC+apperr 识别、canonical/截断/parser、PENDING@8、scanner 停止、非 REVIEW 8 次不变；全包回归与 `-race`。

【未覆盖范围】

- T3（部分金额）、T4（零元人工队列终态/legacy 完整矩阵）、T5（审计/metrics/ops runbook）、T6（NATS 消费/崩溃/DLQ）按 Plan 另派。
- re-arm 仅 ops 审计 SQL（无 RPC/CLI；`REVIEW-REQUIRED-ENTRY-01` 另立）；无自动 re-arm。
- TCC `last_error` 的 1024 列宽截断分支未在 DB 超限场景触发（`refund_id ≤128` 正常不触发；同一 builder 已在 512 列宽单测覆盖截断规则）。
- 零元 REVIEW 不持久化已由 T0 覆盖；本次未扩展零元人工队列。

### 9.5 反妥协自检 / 返工影响声明

- 并发/超时/部分成功：REVIEW Tx2 与 batch/app 同事务（失败整体回滚）；MANUAL_HOLD 短路在写入前；0 行 CAS 只读回读；重试幂等；re-arm 感知避免“复核洗白/永久 conflict”；非 REVIEW 路径行为不变。
- 是否加 if-else 绕过：未新增放行分支；REVIEW 判定收口于既有 `classifyExistingAdmission`/`admitTccApplication` 单一入口；AfterSales 只做 typed 检测 + 镜像，不解释结算语义。
- 妥协（高亮）：① re-arm 仍是 ops SQL（Plan/ENTRY-01 边界）；② TCC 1024 截断未做 DB 级触发测试（见上）；③ T4/T5/T6 未交付。
- `[修改点]`：`tcc_settlement.go`（合法值/单调/短路）、`tcc_admission.go`（REVIEW 自动终止+重读 batch）、`refund_admission.go`（T1c 同事务 batch/app + canonical）、`aftersales/workers.go`（typed 识别/canonical/parser/PENDING/扫描过滤）、`ac50_review_race_test.go`（签名）、新增 `tcc_review_integration_test.go`、`review_task_integration_test.go`、`ac50_review_integration_test.go` 的 O19 断言。
- `[架构侵入性]`：无新状态/字段/表/迁移/RPC；复用 `MANUAL_HOLD` 与既有 `last_error`；批/申请/裁决同事务。
- `[副作用评估]`：REVIEW 路径不再 8 次自动 FAILED；非 REVIEW 不变；TCC 自动续跑返回稳定 token；`refund_batches.last_error` 保留 canonical（reconcile 不清）。

### 9.6 不声称

不声称实现 PASS / R0.4e / merge / Release PASS / AC-VERIFIED；AC-50/51/69/70/71/72 待 Reviewer 绑定证据；T3–T6、ENTRY-01、ALERTS-01 未交付；`PAY-SETTLE-01` 仍冻结。

---

## 10. T3+T4+T1c 自检（追加；code head `13a586a6df5e3bebcb2e26f7e5516b2593bb119c`，base `7b28024`）

| 项 | 值 |
|---|---|
| 范围 | T3 partial 三分类、T4a 零元、T4b legacy、T1c 复审 `RowsAffected` 生产门禁 |
| Code head | **`13a586a`**（6 files，+653/−59） |
| 证据 | `.tmp/evidence-ac50-t1t2/`（HEAD-t3t4.txt、t3t4-targeted/race/full 日志），真实 MySQL 8.4.13 |

### 10.1 T3（partial 三分类，AC-50/AC-70、ADR 0.6 §3.4）

- CONSUMED + 部分金额只用 Order 自有事实（`order_fulfillment_groups.status`，组行缺失时用 `orders.status`）分类；**不调用 `CancelIfCreated`**（O4b/O4c/O4d/O4e 断言 `cancelCalls==0`）、`refundCalls==0`。
- 未离仓 → `PARTIAL_REFUND_REQUIRES_FULL_GROUP`：在 Tx1 任何写入前拒绝（fresh 0 admission/commitment/outbox，O4a/O4b）；preseed 用自身 commitment 排除后重算，PARTIAL 不新增写入、保留既有 admission/commitment（O4e）。
- 已离仓（投影 `SHIPPED/RECEIVED`）→ `POST_SHIP_ALLOWED`：1 条 `RESERVED` commitment、终态 outbox、无探测/无 release（O4c；O5c 为整组投影已离仓，直接 POST）。
- 不可判定（组投影缺失 + `orders.status=SHIPPED`，或未知组合）→ `REVIEW_REQUIRED` + 1 条 `RESERVED` commitment、`reason=PARTIAL_AMOUNT_UNCLASSIFIABLE`、无 outbox/无探测（O4d/W10）；不得默认 PRE/POST。
- 整组 CONSUMED 且投影已 `SHIPPED`：fresh 与 resume 都直接 POST（AC-50 §236），不探测。

### 10.2 T4a（零元，AC-67）

- `CONSUMED + NO_WAYBILL` → 冲突、`review_persisted=false`、0 admission/S19/REVIEW、不计 AC-72 backlog、0 Payment（O7/O21）。
- `TRANSIENT_ERROR` → `Unavailable`、0 写入、0 Payment（O7b）。
- 真实 `CANCELLED` 后同 `refund_id` → `PRE` + S19 `DONE`，重放幂等（O8）。
- 未新增 `ListShipments`/预读，未改零元短路的既有语义。

### 10.3 T4b（legacy，AC-70）

- 无 permit 行：先按 Order 投影分类再决定 PRE——投影 `SHIPPED/RECEIVED` → POST（permit 不变）；未离仓/无组行且 `orders PAID/ALLOCATED` → `NONE→REVOKED` PRE；投影缺失且 `orders SHIPPED/COMPLETED` → REVIEW（`reason=LEGACY_SHIPMENT_UNKNOWN`、1 条 RESERVED、0 探测）。
- 不猜 `payment_no`、不按 0 额度；O10/O10b 覆盖。

### 10.4 T1c hardening

- TCC REVIEW Tx2 的 `refund_batches`/`refund_applications` 镜像 UPDATE 均检查 `RowsAffected==1`；0 行即报错并回滚，**不会提交没有人工持有的 REVIEW admission**（`TestTCCReviewTx2BatchGateRollsBack`：batch 预置 `PAYING` + admission 重置 `PENDING` → 失败后 admission 仍 `PENDING`、batch 仍 `PAYING`）。

### 10.5 命令与结果（HEAD `13a586a`）

```bash
TEST_MYSQL_DSN='.../mall_integration_t1_order...' go test ./internal/services/order \
  -run 'TestAC50ReviewPartial|TestAC50ReviewWhole|TestAC50ReviewLegacy|TestAC50ReviewZero|TestTCCReview|TestAdmitRefund' \
  -count=1 -v                                    # 32 PASS, ok 26.017s
TEST_MYSQL_DSN='.../mall_integration_t1_order...' go test -race ./internal/services/order \
  -run 'TestAC50ReviewPartial|TestAC50ReviewWhole|TestAC50ReviewLegacyUnclassifiable|TestAC50ReviewZero|TestTCCReview' \
  -count=1                                       # ok 10.976s
TEST_MYSQL_DSN='.../mall_integration_t1_order...' go test ./internal/services/order -count=1  # ok 154.848s
```

- 契约变更导致的既有测试更新：`TestAdmitRefundConsumedPermitTwoPhase`、`TestAdmitRefundPostDoesNotWriteReleaseOutbox`、`TestAdmitRefundRemoteAdmissionPreWritesOrderLevelRelease` 由 partial 金额改为整组金额（AC-70 下 partial CONSUMED 未离仓不再探测）；新增 partial/legacy/zero/gate 用例；既有 `TestAdmitRefund*` 全绿。
- 日志：`.tmp/evidence-ac50-t1t2/{HEAD-t3t4.txt,t3t4-targeted.log,t3t4-race.log,t3t4-order-full.log}`。

### 10.6 【测试范围】/【未覆盖范围】

【测试范围】partial 三分类（fresh/preseed、shipped/unshipped/unclassifiable）、整组投影已离仓直 POST、O4a–O4e/O5c/O10b/W10、O7/O7b/O8/O21、O10/O10b、T1c 0 行门禁；真实 MySQL 8.4.13、`-race`、全包回归。

【未覆盖范围】

- T5（审计/metrics/ops runbook）、T6（NATS 消费/DLQ/崩溃窗口）按 Plan 另派；AC-72 的 gauge 尚未实现，O21 以“无 REVIEW admission 行”作为 backlog 代理断言。
- 零元冲突的人工队列终态/AC-72 零元单列计数归 T5/ENTRY-01；本次只保证不持久化、不计数来源行。
- 组投影读取失败返回 `Unavailable` 的分支未单测（代码路径存在，未构造 DB 故障）。
- TCC 1024 列宽截断仍未在 DB 超限场景触发（见 §9.4）。

### 10.7 反妥协自检 / 返工影响声明

- 并发/部分成功：PARTIAL 在写前拒绝；REVIEW 走 Tx2 同事务且镜像 0 行 fail-closed；preseed 重算排除自身 commitment，避免自计入；整组投影已离仓不探测/不取消；不默认 PRE/POST。
- 是否加 if-else 绕过：分类收敛在 `classifyGroupExit`/`loadGroupRemaining` 单一 helper；未按调用方复制状态判断；未新增“放行”分支。
- 妥协（高亮）：① O21 以 DB 行代理 gauge（T5 前）；② 既有 partial-CONSUMED 测试按 AC-70 改为整组金额；③ 投影读取失败分支未做故障注入；④ T1c 门禁会使“batch 已被人工他路持有”时的 REVIEW 提交失败（fail-closed，符合 AC-71）。
- `[修改点]`：`refund_admission.go`（T3 分类/预读/REVIEW 原因/整组投影 POST/resume 排除自身/T1c 镜像门禁）、`tcc_admission.go`（冲突签名）、`ac50_review_integration_test.go`（新增 O4/O5c/O10b/O7b/O8/O21/T1c 用例）、`ac50_review_race_test.go`（permit 前置）、两个既有测试的整组金额修正。
- `[架构侵入性]`：无新字段/表/迁移/RPC/状态值；复用 permit/commitment/admission/outbox；REVIEW 仍为 `result` 承载、`state=PENDING_REMOTE`。
- `[副作用评估]`：partial CONSUMED 未离仓现在返回 PARTIAL（不再远程探测），已离仓不再取消运单；legacy 无 permit 的 POST/REVIEW 不再一律 PRE；既有整组路径不变。

### 10.8 不声称

不声称实现 PASS / merge / Release PASS / AC-VERIFIED；AC-50/69/70/71/72 待 Reviewer 绑定证据；T5/T6、ENTRY-01、ALERTS-01 未交付；`PAY-SETTLE-01` 仍冻结。

---

## 11. T5a/T5b/T5c + T6a–T6d 自检（追加；code head `f1946e74db7629157d9415ec29163dc8c1de8cfd`，base `e4ac80e`）

| 项 | 值 |
|---|---|
| 范围 | T5a 审计、T5b metrics/SLA、T5c ops runbook、T6a 崩溃/竞态、T6b NATS 重复/乱序、T6c DLQ、T6d re-arm |
| Code head | **`f1946e7`**（7 files，+654/−1） |
| 证据 | `.tmp/evidence-ac50-t1t2/`（HEAD-t5t6.txt、t5t6-order-full、t5t6-ff-full、t5b-metrics、t5t6-race），真实 MySQL 8.4.13 + `nats:2.10-alpine` |

### 11.1 T5a/T6d（审计留存 + re-arm）

- `TestAC50ReviewAuditAndRearmRetention`：REVIEW 行**不删除**、`created_at` 保持首次进入、`version` 单调；`runTccReconcileOnce` 不清 `refund_batches.last_error`、不推进 `MANUAL_HOLD`（O20b）；re-arm 按“镜像已在 → 版本 CAS 清 result → batch `MANUAL_HOLD→ADMITTING`”执行后，`last_error` 保留、`created_at` 不变、`version+1`；replay 收敛 PRE，仍不删行（O20/O16/W11/W13）。
- 代码不提供自动 re-arm；只有 ops 带真实 S6 证据/授权/工单的版本化 CAS（§8.3），清空 `last_error` 不作为处置。

### 11.2 T5b（AC-72 后端段）

- `internal/platform/metrics/metrics.go`：新增 `mall_refund_review_backlog`、`mall_refund_review_oldest_seconds`（名称按 ADR §10 O18），在 `init` 注册。
- `internal/services/order/worker.go`：`refreshRefundReviewGauges` 在既有 `RunExpiry` tick 内只读刷新（**不新增 worker/scanner**）；查询 `result='REVIEW_REQUIRED' AND state='PENDING_REMOTE'` 的 count + `MIN(updated_at)` 年龄。零元冲突不写 admission 行，天然不混入。
- `TestRefundReviewGauges`（order）+ `TestRefundReviewGaugesRegistered`（metrics）：backlog=2、oldest≥10 天、7 天逾期查询=1、非 REVIEW 行不计。

### 11.3 T5c（runbook）

- `docs/ops.md` 新增 “Refund review queue (AC-72 backend segment)”：发现/backlog/oldest、7 天逾期、逐项明细、TCC `MANUAL_HOLD` 与售后 `last_error` 查询；24h/7d SLA 与升级；日对账；§8.3 re-arm（授权/审计先于清除/保留 canonical/版本 CAS/同 `refund_id` replay）；存量 PRE 审计查询与“0 行≠安全”。owner ops，expiry 2026-12-31；5 分钟告警规则与用户入口仍归 `REVIEW-REQUIRED-ALERTS-01` / `REVIEW-REQUIRED-ENTRY-01`。

### 11.4 T6a（W1–W4/O3）

`TestAC50ReviewCrashReplayConverges`：Tx1 后崩溃（transient 保持 PENDING + 占位）→ replay 收敛 REVIEW，无提前终态/释放；真实 CANCELLED 后 replay 收敛 PRE，release 恰 1 次且重复 replay 不重复。`TestAC50ReviewShipCancelRace`：Ship/取消竞态只产生一个终态（POST 或 PRE），1 条终态 outbox，release 与结果一致（PRE=1 / POST=0）。

### 11.5 T6b/T6c（NATS + DLQ）

- `internal/services/fulfillment/review_consumer_integration_test.go`（真实 JetStream；消费实现位于 fulfillment，Plan 的 `order/review_consumer` 路径按实际所有者落位）：重复 PRE 只取消一次、不重复建行；`REVIEW_REQUIRED`/`POST_SHIP_ALLOWED` 事件均 no-op（不改 placeholder）；非法 JSON 进 DLQ，DLQ payload replay 仍 no-op。
- Order 侧 `TestAC50ReviewConsumerNoAutoPre`：REVIEW 后连续 3 次 replay（含迟到真实 CANCELLED）不写任何 outbox/release，commitment 仍 RESERVED（无 auto-PRE，F7b/W9 的 Order 半侧）。
- `TEST_NATS_URL` 未设置时测试显式 `t.Skip`；本次证据在 NATS JetStream 上真实执行（见日志）。

### 11.6 命令与结果（HEAD `f1946e7`）

```bash
TEST_MYSQL_DSN='.../mall_integration_t1_order...' go test ./internal/services/order -count=1        # ok 260.677s
TEST_MYSQL_DSN='.../mall_integration_t1_ff...' TEST_NATS_URL='nats://127.0.0.1:14222' \
  go test ./internal/services/fulfillment -count=1                                                   # ok 17.879s
go test ./internal/platform/metrics -count=1                                                          # ok 0.100s
TEST_MYSQL_DSN='.../mall_integration_t1_order...' go test -race ./internal/services/order \
  -run 'TestAC50ReviewAudit|TestAC50ReviewCrash|TestAC50ReviewShipCancel|TestAC50ReviewConsumer|TestRefundReviewGauges' \
  -count=1                                                                                            # ok 8.261s
```

- 日志：`.tmp/evidence-ac50-t1t2/{HEAD-t5t6.txt,t5t6-order-full.log,t5t6-ff-full.log,t5b-metrics.log,t5t6-race.log,t6b-nats.log}`。

### 11.7 【测试范围】/【未覆盖范围】

【测试范围】审计行/版本/时间戳/reconcile 不清镜像/审计先行 re-arm；backlog/oldest gauge 与 7 天查询、零元不混入、既有 tick 刷新；runbook 查询与 re-arm 流程；W1–W4 崩溃/竞态；真实 NATS 重复 PRE/REVIEW/POST no-op/DLQ replay；Order 侧 REVIEW 无 auto-PRE；全包回归与 `-race`。

【未覆盖范围】

- **W8 `PENDING_REMOTE` 永久悬挂**只验证“不重复/不自动收敛”，未构造 DLQ 租约永久丢失的运维重放（归 AC-27-LIVE-01，不在本 L2）。
- **迟到 `order.paid` 真实 placeholder**：本 L2 不改 Fulfillment 的 settlement consumer，未在 NATS 中构造该路径；REVIEW 不产生终态事件，Order 无 scanner，故不会自动转 PRE（由 order 侧 no-auto-PRE 测试覆盖，真实 placeholder 路径仍为既有行为）。
- 告警规则/5 分钟部署（`REVIEW-REQUIRED-ALERTS-01`）与用户可见入口（`REVIEW-REQUIRED-ENTRY-01`）按 D-33 另立，未交付。
- gauge 为进程内指标（多副本需按 instance 聚合/求和），runbook 未固化 Prometheus 查询表达式（ops 侧）。
- TCC 1024 列宽截断仍未在 DB 超限场景触发（见 §9.4）。

### 11.8 反妥协自检 / 返工影响声明

- 并发/超时/部分成功：REVIEW 后 replay 不探测、不写事件；审计先于 CAS；re-arm 版本 CAS；Ship/取消竞态单终态；NATS 重复/DLQ replay 幂等；无自动 re-arm/自动关闭。
- 是否加 if-else 绕过：未新增放行分支；gauge 只读挂既有 tick；consumer 语义未改（只测试 no-op/幂等）。
- 妥协（高亮）：① W8 永久悬挂/迟到 placeholder 真实链路未构造（见上）；② NATS 测试按实际 consumer 所有者放在 fulfillment 包（Plan 文件名偏差）；③ gauge 多副本聚合口径留给 ops。
- `[修改点]`：`internal/platform/metrics/metrics.go`（2 gauge+注册）、`order/worker.go`（tick 只读刷新）、`docs/ops.md`（AC-72/re-arm/legacy audit runbook）、新增 `order/worker_test.go`、`order/ac50_review_audit_fault_test.go`、`fulfillment/review_consumer_integration_test.go`、`platform/metrics/metrics_test.go`。
- `[架构侵入性]`：无新 worker/扫描器/字段/迁移/RPC；复用既有 tick 与 admission/outbox 事实。
- `[副作用评估]`：tick 每分钟一次只读聚合查询（有索引 `idx_order_refund_admissions_order_group`，按 result/state 过滤为全表扫描风险需 ops 观察——当前 P0 数据量小，未新增索引因 `migrations/**` 禁止；已在 runbook/未覆盖登记）。

### 11.9 不声称

不声称实现 PASS / merge / Release PASS / AC-VERIFIED；AC-50/51/61/62/65/67/69/70/71/72 待 Reviewer 绑定证据；ALERTS-01/ENTRY-01/T5 完整公告与 T6 之外的 T2–T4 收口仍待 Reviewer；`PAY-SETTLE-01` 仍冻结。

---

## 12. Reviewer T3–T6 FAIL 修复（T3-R1/T3-R2；code head `c097e981d0fa45f166e5efc95b2fd783cfc5f8b7`，base `849cf78`）

### 12.1 T3-R1 BLOCKER：fresh Tx1 REVIEW 绕过 T1c 持批门禁

- **根因**：T1c 门禁只内联在 `writeAdmissionTx2`（Tx2）。fresh Tx1 的 REVIEW 路径（`refund_admission.go` 的 `result == refundReviewRequired` 分支）只写 admission + commitment 就 commit；`source=tcc` 的 partial-unclassifiable / legacy-unclassifiable 因此留下 batch `ADMITTING`、app 未镜像、无 `refund_batches.last_error`，`admitTccBatch` 只能返回泛化 `Unavailable`（违 AC-71/W12）。
- **修法**：抽出唯一入口 `persistTCCReviewTx(ctx, tx, refundID, batchID, reason)`，Tx1 与 Tx2 **共用**：
  - `refund_batches` → `MANUAL_HOLD` + canonical `last_error`（`REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>`，1024 列 D-35 截断）；
  - `refund_applications`（已存在行）→ `admission_result=REVIEW_REQUIRED`，state 保持 `PENDING`；
  - 两条 UPDATE 均要求 `RowsAffected==1`；任一 0 行即报错并回滚整个事务（fresh Tx1 的 admission/commitment 一并回滚），**不得提交无人工持有的 REVIEW**；`source=tcc` 且 `batch_id` 为空同样 fail-closed。
- **测试**（`TestTCCReviewFreshSourceGate`，真实 MySQL 8.4.13）：
  - fresh `source=tcc` partial-unclassifiable：MANUAL_HOLD、canonical last_error（PARTIAL）、app PENDING/REVIEW、0 probe/0 payment；`admitTccBatch` 返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`，0 状态变化（W12）。
  - fresh `source=tcc` legacy-unclassifiable：同上，reason=`LEGACY_SHIPMENT_UNKNOWN`。
  - 0 行 batch 镜像（batch 预置 `PAYING`）：报 “batch state changed while persisting review”，admission/commitment 均未提交。
  - preseed/Tx2 侧既有 O1b、T1c gate、O4d、O10 用例保持绿色（见 §9/§10）。

### 12.2 T3-R2 证据/位置更正

- **T6b 实际位置**：真实 NATS 用例是 `internal/services/fulfillment/review_consumer_integration_test.go::TestRefundAdmittedConsumerReviewAndDuplicate`（消费实现归属 Fulfillment），**不在** `internal/services/order/review_consumer_integration_test.go`；Plan 的文件路径按实际所有者落位，selfcheck §11.5 的说明以此为准。Order 侧无 auto-PRE 的 DB/NATS 无关断言在 `order/ac50_review_audit_fault_test.go::TestAC50ReviewConsumerNoAutoPre`。
- **F7b 迟到 `order.paid` 真实 placeholder**：**[未覆盖]**。构造它需要 Fulfillment 的 `order.paid` settlement consumer 与 Order 双库/NATS 时序，本 L2 未改该生产 consumer；且 REVIEW 不产生终态 outbox 事件、Order 无 scanner，逻辑上不会自动转 PRE（由 `TestAC50ReviewConsumerNoAutoPre` 的 3 次 replay + 迟到 CANCELLED 断言覆盖）。W8 `PENDING_REMOTE` 永久悬挂归 `AC-27-LIVE-01`，不在本 L2。
- **T5/T6 已 PASS 语义未改**：本次只改 `refund_admission.go` 的 review 持久化入口与新增 TCC 测试；metrics/worker/ops runbook/NATS 用例未改。

### 12.3 命令与结果（HEAD `c097e98`）

```bash
TEST_MYSQL_DSN='.../mall_integration_t1_order...' \
  go test ./internal/services/order -run 'TestTCCReview|TestAC50Review' -count=1   # ok 34.369s
TEST_MYSQL_DSN='.../mall_integration_t1_order...' \
  go test -race ./internal/services/order -run 'TestTCCReview' -count=1           # ok 6.606s
TEST_MYSQL_DSN='.../mall_integration_t1_order...' \
  go test ./internal/services/order -count=1                                      # ok 202.838s
```

- 日志：`.tmp/evidence-ac50-t1t2/{HEAD-t3r.txt,t3r-targeted.log,t3r-race.log,t3r-order-full.log,t3r1-fresh.log}`。
- 【测试范围】fresh/preseed TCC REVIEW 的 batch/app/admission 同事务门禁、canonical reason 两种、自动续跑 token、0 行回滚；order 全包与 `-race`。
- 【未覆盖范围】见 §12.2；T5/T6 的 W8、告警/入口、gauge 多副本聚合维持 §11.7 的登记。

### 12.4 反妥协自检 / 影响声明

- 并发/部分成功：Tx1/Tx2 同门禁；0 行 fail-closed 回滚；batch_id 缺失 fail-closed；不写 outbox/release、不探针、不支付。
- 是否绕过架构：未新增分支/状态；只抽公共函数，消除 Tx1/Tx2 两套实现（防止再次漂移）。
- `[修改点]`：`refund_admission.go`（`persistTCCReviewTx` + Tx1/Tx2 调用）、`tcc_review_integration_test.go`（fresh gate 用例）。
- `[架构侵入性]`：无新字段/表/迁移/RPC；复用 `MANUAL_HOLD` 与既有 `last_error`。
- `[副作用评估]`：TCC fresh REVIEW 现在原子持批；若 batch/app 行缺失或状态不符，REVIEW 不落库（fail-closed），调用方得到可诊断错误而非泛化 `Unavailable`；非 TCC 的 aftersale/legacy REVIEW 路径不受影响。

### 12.5 不声称

不声称实现 PASS / 重审 PASS / merge / Release PASS / AC-VERIFIED；T3/T4 BLOCKER 的修复待 Reviewer 重审绑定 HEAD `c097e98`；AC-50/69/70/71/72 仍 AC-REF；`PAY-SETTLE-01` 仍冻结。

---

## 13. T8 回归 + T7 证据输入（code head `3ce0d7d1cdc8bd36eb65db7cbe36e11736fedd62`）

> 真实 MySQL **8.4.13**（disposable `mysql:8.4` @127.0.0.1:13336；每包独立 schema：`mall_integration_t8_{order,ff,as}`）。无 SKIP/FAIL；`ac-coverage.tsv` 仍由 Reviewer 更新，Developer 不写台账。

### 13.1 T8 命令、范围与结果（HEAD `3ce0d7d`）

| # | 包 / 命令 | 覆盖 | 结果 |
|---|---|---|---|
| 1 | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC25\|TestAC27\|TestT17\|TestTCCReview\|TestAC50Review' -count=1 -v` | AC-25/27、T17、TCC review、AC-50/69/70/71/72 主链 | **38 PASS / 0 FAIL / 0 SKIP**，`ok 33.780s` |
| 2 | `TEST_MYSQL_DSN=$ORDER_DSN go test -race ./internal/services/order -run 'TestAC25\|TestAC27\|TestT17\|TestTCCReview\|TestAC50Review' -count=1 -v` | 同 1，`-race` | **38 PASS**，`ok 27.585s` |
| 3 | `TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAdmitRefund\|TestZeroApplicationCheckConstraint\|TestRefundRelease' -count=1 -v` | INV-44/45/50 回归（release/零元/准入）+ TCC 关键 | **20 PASS**，`ok 21.367s` |
| 4 | `TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run 'TestAC14\|TestAC31\|TestAC32\|TestZero\|TestReview' -count=1 -v` | AC-14/31/32、零元、售后复核镜像 | **16 PASS**，`ok 15.067s` |
| 5 | `TEST_MYSQL_DSN=$AS_DSN go test -race ./internal/services/aftersales -run 'TestAC14\|TestAC31\|TestAC32\|TestZero\|TestReview' -count=1` | 同 4，`-race` | **16 PASS**，`ok 9.759s` |
| 6 | `TEST_MYSQL_DSN=$FF_DSN go test ./internal/services/fulfillment -run 'TestAC21\|TestCancelIfCreated' -count=1 -v` | AC-21、CancelIfCreated F1/F1b/F2–F6/F8 | **4 PASS**，`ok 2.899s` |
| 7 | `go test ./internal/platform/metrics -count=1 -v` | AC-72 gauge 注册 | **1 PASS**，`ok 0.086s` |

- 原始日志：`.tmp/evidence-ac50-t8/{HEAD.txt,order-regression.log,order-race.log,order-inv.log,aftersales-regression.log,aftersales-race.log,fulfillment-regression.log,metrics.log}`；`HEAD.txt` = `3ce0d7d...`。
- 关键 PASS 名单：AC-27 六个投影用例、AC-25、T17 六个（Σ/并发/崩溃/部分失败/legacy/shipped）、TCC review 三个（含 T3-R1 fresh gate）、AC-50 partial/zero/legacy/audit/race 全套；售后 4 个 Review + 12 个 AC-14/31/32/Zero；Fulfillment 3 个 CancelIfCreated + AC-21。

### 13.2 T7 证据映射（建议值；Reviewer 定 status；Developer 不写 tsv）

| AC/INV | test_type | evidence_path:line | test name | 建议 status（HEAD `3ce0d7d`） |
|---|---|---|---|---|
| AC-14 | MySQL | `internal/services/aftersales/settlement_integration_test.go:875` | `TestAC14FreightRefundSixScenarios` | AC-VERIFIED（维持；Reviewer 复核） |
| AC-21 | MySQL | `internal/services/fulfillment/delivery_integration_test.go:397` | `TestAC21AutoConfirmT7` | AC-REF（“至少一次通知”仍未断言） |
| AC-25 | MySQL+race | `internal/services/order/settlement_integration_test.go:355` | `TestAC25RefundProjectionKeepsLifecycle` | AC-REF（范围有限，非“任意退款场景”） |
| AC-27 | unit+MySQL | `internal/services/order/warehouse_quote_test.go:125`；`ac27_projection_integration_test.go:16`；`ac27_projection_cancelled_test.go:10,36`；`ac27_projection_negative_test.go:9,42` | `TestAC27*` 6 个 | AC-REF（AC-27A 5/6、NOWB/LIVE 仍开） |
| AC-31 | MySQL | `internal/services/aftersales/settlement_integration_test.go:1101` | `TestAC31FreightHoldTimeout` | AC-REF（168h/用户可见未断言） |
| AC-32 | MySQL | `internal/services/aftersales/settlement_integration_test.go:1102` | `TestAC32FreightHoldTimeoutDefaultsToMerchant` | AC-REF（用户可见补退未断言） |
| AC-50 | MySQL+race | `internal/services/order/ac50_review_integration_test.go:102,347,405,438,537,567`；`ac50_review_audit_fault_test.go:117` | `TestAC50Review*`、`TestAC50ReviewCrashReplayConverges` | AC-VERIFIED（本 L2 分支范围；Reviewer 定） |
| AC-69 | MySQL+race | `ac50_review_integration_test.go:102,159,261`；`ac50_review_audit_fault_test.go:18`；`ac50_review_race_test.go:179`；`aftersales/review_task_integration_test.go:33,64` | `TestAC50ReviewMapping/ZeroNotPersisted/Replay`、`TestAC50ReviewAuditAndRearmRetention`、`TestAC50ReviewLateCancelDoesNotWash`、`TestReviewTask*` | AC-VERIFIED（后端；审计为 D-32 四载体，非行级历史） |
| AC-70 | MySQL+race | `ac50_review_integration_test.go:203,347,374,405,438,469,510,537` | `TestAC50ReviewPartial*`/`Legacy*`/`WholeGroupProjectionShipped` | AC-VERIFIED（O4a–e/O5c/O10/O10b） |
| AC-71 | MySQL+race | `order/tcc_review_integration_test.go:34,137,228`；`ac50_review_integration_test.go:640` | `TestTCCReviewManualHold/RearmResumes/FreshSourceGate`、`TestTCCReviewTx2BatchGateRollsBack` | AC-VERIFIED（O1b/W12/O16 + T3-R1 gate） |
| AC-72 | MySQL+unit | `order/worker_test.go:14`；`platform/metrics/metrics_test.go:11`；`docs/ops.md`（Refund review queue） | `TestRefundReviewGauges`、`TestRefundReviewGaugesRegistered`、runbook | **部分**：后端段（metrics/SLA/对账）建议 AC-VERIFIED；告警/入口 DEFERRED（ALERTS-01/ENTRY-01），不得整体 VERIFIED |
| INV-44 | MySQL+fault | `internal/services/order/refund_release_integration_test.go:142,244,279,303,341,374,395`；`migrations/order/0023_r04_checks_v2.sql`（R0.4 真实迁移） | `TestRefundReleaseSourceMapping`、`TestAdmitRefund*Release*` | AC-REF（多组释放 + missing-reservation metric DEFERRED） |
| INV-45 | MySQL | `ac50_review_integration_test.go:102`（REVIEW 后 commitment RESERVED、无 release）；`refund_release_integration_test.go:279` | `TestAC50ReviewMapping`、`TestAdmitRefundPostDoesNotWriteReleaseOutbox` | AC-REF（T3 release writer 仍 DEFERRED） |
| INV-46 | MySQL | `migrations/order/0022`（冻结）/`0023_r04_checks_v2.sql`；`order/r04_checks_integration_test.go` | `TestR04Checks*`（R0.4 证据） | DEFERRED（T2 writer/worker；CHECK 只是兜底） |
| INV-47 | MySQL+race | `aftersales/settlement_integration_test.go:875,1101,1102`；`order/settlement_integration_test.go:355` | AC-14/31/32/25 | AC-REF（行级责任/补退 DEFERRED T3） |
| INV-48 | MySQL+race+fault | `internal/services/order/t17_evidence_integration_test.go:26,83,204,306,371,442` | `TestT17SigmaMismatchWritesAuditAndNeverCallsPayment` 等 6 个 | AC-REF（未 AC-VERIFIED） |
| INV-49 | — | ADR Rev 2.3 §12.3 / T4.4（未交付） | — | DEFERRED（T4.4，shadow 硬拒绝已证） |
| INV-50 | MySQL | `aftersales/zero_refund_integration_test.go:497,182,562`；`order/refund_admission_integration_test.go:661`；`ac50_review_integration_test.go:159,567,597` | `TestZeroAfterSale*`、`TestAdmitRefundZero*`、`TestAC50ReviewZero*` | AC-REF（零元不调 Payment/DONE 已测；AC-72 零元单列待 T5 告警段） |

- TCC/零元/legacy 回归：TCC review 3 个 + T1 既有 TCC 用例（T17）全绿；零元 aftersales/order 全绿；legacy T17 + AC-50 legacy（O10/O10b）全绿。
- **建议 status 仅为 Developer 证据输入**；AC-VERIFIED、延期行、`ac-coverage.tsv` 由 Reviewer 按精确 HEAD 核对后写入。

### 13.3 未覆盖/边界（T7 输入）

- AC-72 的告警规则/5 分钟部署与用户入口/文案未交付（ALERTS-01/ENTRY-01），不得整体 AC-VERIFIED。
- F7b 迟到 `order.paid` 真实 placeholder 未构造（见 §12.2）；W8 永久悬挂归 AC-27-LIVE-01。
- AC-21/31/32/25 的既有限制（通知/用户可见/全场景）维持原 tsv reason，不因本次回归升级。
- 本次只跑指定回归与 `-race` 子集；未重跑全仓 `make verify`，未跑 UAT/prod。

### 13.4 结论

T8 回归在真实 MySQL 8.4.13、独立 schema、精确 HEAD `3ce0d7d` 上 **0 FAIL / 0 SKIP**（order 38/38、order-inv 20/20、aftersales 16/16、fulfillment 4/4、metrics 1/1；`-race` order 38/38、aftersales 16/16）。T7 证据输入如上；不声称实现/Release PASS，`ac-coverage.tsv` 由 Reviewer 更新。

---

## 14. T7-R1..R3 残余硬化（test-only；code head `d90daf09c3f391ca207df68f02d57a38b7cf781b`，base `b731a9f`）

> 只改 `_test.go`，未改生产代码/迁移/deploy/frontend/specs/adr/台账；真实 MySQL 8.4.13（独立 schema `mall_integration_t7r_{order,ff,as}`）+ `nats:2.10-alpine`。无 SKIP/FAIL。

### 14.1 T7-R1（F7b 迟到 `order.paid` 真实 placeholder）

- **Fulfillment 半链（已测）** `TestOrderPaidCreatesRealPlaceholderThenReviewNoop`（NATS JetStream）：通过正常 `RunOrderPaidConsumer`（`order.paid`）创建**真实 `CREATED` placeholder**（非 fixture INSERT）；随后 `REVIEW_REQUIRED`/`POST` 事件不改动它；真实 `PRE_SHIP_REVOKED` 才取消该真实行。测试使用每次运行的唯一 `order_no/refund_id`，避免 JetStream 重放历史消息污染。
- **Order 半链（模拟已测）**：`PENDING_REMOTE` 整组 → 真实 `CANCELLED` 重放收敛 PRE（§9 W3）；部分金额预置 `PENDING_REMOTE` → 不探测/不转 PRE（§10 O4e）；已有 `REVIEW_REQUIRED` → 迟到 `CANCELLED` 不自动探测/不自动 PRE（§11 O11d/W13、§11.5 no-auto-PRE 三次 replay）。
- **`[BLOCKED]`（全链路 F7b）**：**双库/双服务时序不可构造**。依赖：Order + Fulfillment 两个进程（真实 `AdmitRefund`/`CancelIfCreated` gRPC）与各自 MySQL + NATS `order.paid`/`refund_admitted`，在同一次测试内完成“Fulfillment 建真实 placeholder → Order 准入重探 → Fulfillment consumer 看到结果”。本仓库无跨服务集成 harness；在单个 Go test 中 import 两个 service 包违反 AGENTS 静态守卫（跨服务包引用），启动两个 `cmd/*` 服务与双 DSN 属运行时编排，超出 test-only 范围。按 Lead 指示标 `[BLOCKED]`，**不以 fixture 冒充**；建议另立 Environment/Integration harness change（owner Lead/QA，expiry 2026-12-31）。

### 14.2 T7-R2（TCC 1024 截断 + gate 独立用例）

- `TestTCCReviewLastErrorTruncation1024`：合成 2000 字符 `refund_id`，`refundReviewCanonical(..., 1024)` 长度 ≤1024、保留 `REVIEW_REQUIRED reason=<CODE> ticket=RVW:` 前缀、以 ` truncated=1` 结尾、`reason=/ticket=` 各仅 1 次；`TestReviewCanonicalParseAndTruncate` 增加 1024 宽度截断文本的 parser 解析（宽高无关）。
- `TestTCCReviewGateApplicationMirrorZeroRows`：batch 镜像可更新、application 行缺失 → Tx2 报 “tcc refund application is not PENDING while persisting review”；**admission 仍为原 PENDING、batch 仍 ADMITTING**（整体回滚）。
- `TestTCCReviewEmptyBatchIDFailsClosed`：`source=tcc` 且 `batch_id` 空 → 在 Tx1 写之前失败，`refund_applications`/`order_refund_commitments` 均 0 行。

### 14.3 T7-R3（unknown status / O19 / O1b 隔离）

- `TestAC50ReviewUnknownProbeStatus`：未知/未来 `CancelIfCreated` status → `Unavailable`；permit/commitment/admission 原样，0 outbox/0 release，不写 PRE/POST/REVIEW。
- `TestAC50ReviewOrdinaryAfterSaleNoApplicationRow`（O19）：普通售后 REVIEW 的 `refund_applications` 明确 0 行（镜像归 AfterSales task）。
- `TestTCCReviewOtherBatchUnaffected`（O1b）：两批 TCC，A 收 `NO_WAYBILL` → `MANUAL_HOLD` + `TCC_BATCH_REQUIRES_MANUAL_HANDLING`；B 收真实 `CANCELLED` → 继续按准入/支付推进（`PAYING/DONE`），恰好 1 次 `RefundBatch`；A 不阻塞 B。

### 14.4 命令与结果（HEAD `d90daf0`）

```bash
TEST_MYSQL_DSN='.../mall_integration_t7r_order...' \
  go test ./internal/services/order -run 'TestTCCReview|TestAC50Review' -count=1 -v      # 31 PASS, ok 20.897s
TEST_MYSQL_DSN='.../mall_integration_t7r_order...' \
  go test -race ./internal/services/order -run 'TestTCCReview|TestAC50ReviewUnknown|TestAC50ReviewOrdinaryAfterSale' -count=1  # ok 8.018s
TEST_MYSQL_DSN='.../mall_integration_t7r_order...' go test ./internal/services/order -count=1  # ok 129.495s
TEST_MYSQL_DSN='.../mall_integration_t7r_as...' go test ./internal/services/aftersales -run 'TestReview' -count=1  # ok 2.093s
TEST_MYSQL_DSN='.../mall_integration_t7r_ff...' TEST_NATS_URL='nats://127.0.0.1:14223' \
  go test ./internal/services/fulfillment -run 'TestOrderPaidCreatesRealPlaceholderThenReviewNoop|TestRefundAdmittedConsumerReviewAndDuplicate' -count=1  # ok 4.571s
```

- 日志：`.tmp/evidence-ac50-t7r/{HEAD.txt,order-t7r.log,order-t7r-race.log,order-full.log,aftersales-t7r.log,fulfillment-t7r.log}`。NATS 用例连续重跑 3 次稳定（唯一 order/refund id + 串行 consumer）。

### 14.5 【测试范围】/【未覆盖范围】

【测试范围】T7-R2 1024 截断/parser/空 batch/app 0 行 gate；T7-R3 unknown status/O19 0 行/O1b 双批隔离；F7b 的 Fulfillment 半链（真实 placeholder + REVIEW/POST no-op + PRE 取消）与 Order 半链模拟；真实 MySQL/NATS、`-race` 子集、order 全包。

【未覆盖范围】

- **F7b 全链路 `[BLOCKED]`**（双服务/双库 harness 缺失，见 §14.1）；W8 永久悬挂仍归 AC-27-LIVE-01。
- NATS 测试依赖本地 `TEST_NATS_URL`，未在 CI 固化；CI 未跑。
- T7-R1 的 Order 半链使用 fake `CancelIfCreated` 状态模拟真实 placeholder 事实，不能替代跨服务端到端证据。

### 14.6 不声称

不声称实现/重审 PASS、Release PASS、AC-VERIFIED；F7b 全链路为 `[BLOCKED]`；`ac-coverage.tsv` 仍由 Reviewer 更新；`PAY-SETTLE-01` 冻结状态不变。
