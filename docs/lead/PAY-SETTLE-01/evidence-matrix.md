# AC Evidence Matrix — PAY-SETTLE-01 Remediation

- Source: SPEC-0002 v1.7；C-02 升 v1.8 后刷新本矩阵。
- **唯一 AC 台账**：`docs/collab/ac-coverage.tsv`，Owner 固定为 **Reviewer**。本文件只是 Plan 视图，不拥有 AC 状态。
- `Test owner`：Developer；`AC status owner`：Reviewer。两者不得混写。
- Status: `NOT STARTED` / `AC-REF` / `AC-VERIFIED` / `DEFERRED`。只有真实执行日志（绑定 HEAD）+ Reviewer 核对断言才能标 AC-VERIFIED；SKIP/FAIL 不算。

## 1. Existing AC

| AC | 必须覆盖的 EARS 分支（摘） | Test type | Evidence path / planned test | Test owner | Status |
|---|---|---|---|---|---|
| AC-39 | 意图/确认金额≠pay_cent；渠道异常捕获退款 | unit + MySQL + fault | `internal/services/payment/create_intent_test.go`、capture mismatch test | Developer | NOT STARTED |
| AC-40 | 零元不捕获；正金额捕获退款；取消/缺货/售后只释放不调 Payment | MySQL | `order/zero_integration_test.go`、`order/refund_admission_integration_test.go`、零元退款回归 | Developer | AC-REF（旧测试，需新 HEAD 重跑） |
| AC-41 | 第二支付退款；同 payment 幂等；并发 CAS 唯一胜者 | MySQL + `-race` | `order/duplicate_payment_integration_test.go`、duplicate capture test | Developer | AC-REF（旧测试，需新 HEAD 重跑） |
| AC-42 | 捕获未结算只显示处理中；PAID 不等于可发货；组退款展示 | handler/contract | BFF/order handler test | Developer | NOT STARTED |
| AC-43 | 捕获后崩溃续跑；订单 CANCELLED 时 HELD/CONFIRMED 收敛 | fault injection | settlement worker crash test；inventory compensation test | Developer | NOT STARTED |
| AC-44 | 回调重复；payment_no 金额/订单不一致；本地失败不 ack；已被其他 payment 结算 | MySQL + fault | payment callback/outbox test | Developer | NOT STARTED |
| AC-45 | 非 PENDING 重读；PAID 同 payment 幂等；PAID 不同 payment 退款 | MySQL + `-race` | `order/duplicate_payment_integration_test.go` | Developer | AC-REF（旧测试，需新 HEAD 重跑） |
| AC-46 | 取消 vs 支付 CAS 唯一胜者；已 PAID 不提供取消 | MySQL + `-race` | order cancel/confirm race test | Developer | NOT STARTED |
| AC-47 | Inventory 明确不足；技术失败重试；成功后缺货转 AC-56 | MySQL + fault | readiness timeout/failure test | Developer | AC-REF（CheckOrderReady 旧测试，需重跑） |
| AC-48 | 许可 NONE/CONSUMED/REVOKED；行确认；缺货；Ship 读 CANCELLED/CREATED | MySQL | `order/settlement_integration_test.go`、shortage/permit tests | Developer | NOT STARTED |
| AC-49 | 四事实分别输出 | handler/contract | API/BFF contract test | Developer | NOT STARTED |
| AC-50 | 未离仓整组；已离仓部分；REVOKED/CONSUMED/CANCELLED；CAS 败者；取消失败重试；释放失败告警 | MySQL + `-race` | `order/refund_admission_integration_test.go`、cancel-vs-ship race test | Developer | AC-REF（旧测试，需补分支/重跑） |
| AC-51 | PENDING 捕获后 CAS→CANCELLED；活动组；Σ 预检；部分失败；无组 G1；已全额跳过 | MySQL + `-race` + fault | TCC batch/fan-out test、crash-resume test | Developer | NOT STARTED |
| AC-52 | 缺货退款；已离仓只退不取消 | MySQL + worker | shortage refund test | Developer | NOT STARTED |
| AC-53 | 订单级 captured/refunded；非生效捕获单独记账；多 intent | MySQL | reconciliation/ledger test | Developer | NOT STARTED |
| AC-54 | 单组隔离；整单全退阻断所有未发组 | MySQL | group isolation test | Developer | NOT STARTED |
| AC-55 | Payment 事实为准；超时未知保持处理中；惰性对账 | MySQL + fault | refund projection/reconcile test | Developer | NOT STARTED |
| AC-56 | 4h 通知；48h 默认退款；等待补货；RESTOCKED；并发 CAS | MySQL + time/race | shortage timer/choice/restock tests | Developer | NOT STARTED |
| AC-57 | CREATED/CANCELLED 不可收货；重复收货幂等 | MySQL | ADR-0005 existing receipt tests | Developer | AC-REF（既有基线，需引用/重跑） |
| AC-58 | 收货后争议不回滚 receipt | MySQL | ADR-0005 existing dispute tests | Developer | AC-REF（既有基线，需引用/重跑） |
| AC-59 | 退款/收货/库存失败/缺货解决乱序拒绝 | fault/integration | event out-of-order tests | Developer | NOT STARTED |
| AC-60 | 历史 PAID 展示/退款/发货；唯一候选；人工队列；SLA；`EXPIRED_*` 终态 | MySQL + backfill + fault | `ListOrderIntents` resolve test、backfill test、resolution queue test（含 24h 终态） | Developer | NOT STARTED |
| AC-61 | 30min 处理中告警；永久失败人工重放；每日对账 | fault + ops | reconciliation/alert/manual replay test | Developer | NOT STARTED |

## 2. New semantics requiring AC (C-02)

| 新语义 | 建议 AC | Test type | Evidence path / planned test | Test owner | Status |
|---|---|---|---|---|---|
| P-05 人工授权释放（`FAILED_PERMANENT`/放弃） | AC-62（或 AC-50 新 EARS 条） | MySQL + fault | `order/refund_commitment_release_test.go` | Developer | NOT STARTED |
| P-04 `legacy_payment_resolution` 人工队列/SLA/恢复 | AC-63（或 AC-60 新 EARS 条） | MySQL + fault | `order/legacy_payment_resolution_test.go` | Developer | NOT STARTED |
| P-06 行级/商品运费组成/责任 | AC-64（或 AC-50/53 新 EARS 条） | MySQL + `-race` | `order/refund_line_ledger_test.go` | Developer | NOT STARTED |
| TCC Σ 预检（不改状态） | AC-65（或 AC-51 新 EARS 条） | MySQL + fault | `order/tcc_sum_precheck_test.go` | Developer | NOT STARTED |
| `order.settled` shadow 不双建运单 | AC-66（或 AC-48 新 EARS 条） | MySQL + fault | `fulfillment/shadow_consumer_test.go` | Developer | NOT STARTED |

C-02 必须为每行分配正式 AC 编号并写入 `ac-baseline.txt`；未分配前不得实现对应语义。

## 3. Required logs

- `-race`：AC-41/45/46/50/51/54/56 + 退款准入 vs Ship + TCC batch 并发。
- 故障注入：AC-43/47/50/51/55/56/59/61 + TCC 崩溃续跑 + payment.refunded 丢失/惰性对账。
- 迁移：`refund_applications`、commitment lines、`legacy_payment_resolution`、`refund_seq` 必须跑真实 `migrations/`。
- 每条证据：HEAD、命令、结果、环境；SKIP/FAIL 不计。
- `ac-coverage.tsv` 统一状态；本矩阵不替代它。
