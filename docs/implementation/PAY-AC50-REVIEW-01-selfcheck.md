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
