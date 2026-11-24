# PAY-SETTLE-01 T1.6 Developer 自检清单

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Role / Actor | Developer / `dev-phase1i` |
| Tier | **L2** |
| Tier-Rationale | 跨服务（Order→Payment 事件命令）、支付级退款、既有 `order.refund_requested` 消费语义；命中 L2 触发条 |
| Design / QC | `docs/adr/0007-amd1-addendum-t16-payment-cancel.md` **accepted r1.0（2026-11-25）**，§4.2 五裁决；QC-28 READY |
| Spec-Ref | SPEC-0002 **v1.9** AC-46/AC-65/AC-67/AC-68、INV-44/INV-46；ADR Amd1 §4.8 |
| Base | `5093a1d`（T1.6 WIP 首个实现提交 `e9643ff` 的父提交；本清单随 metric/selfcheck WIP 提交） |
| Code head（本清单绑定） | **本 WIP 提交**（`git log -1` 即该提交，包含本文件） |
| allowed_paths | `internal/services/order/**`、`internal/services/payment/**`、`internal/services/inventory/**`、`internal/platform/events/**`、`internal/platform/metrics/**`、对应 `_test.go`、`docs/implementation/PAY-SETTLE-01-T1.6-selfcheck.md` |
| baseline_impact | `order.refund_requested` 消费者、`refundCancelledAfterConfirm`、`TccCancelMarkPaid` 的 PENDING+captured 分支、Payment.Refund 幂等；INV-44/INV-46；非回归：order/payment/inventory/events 全包测试。无 proto、无 migrations、无新 cancel 表 |
| Scope | T1.6 payment-level cancel；`pay:<payment_no>:cancel` 命令、PENDING/CANCELLED+captured、consumer command-only、C-04 可选字段、CAS 丢失 metric |
| 不做 | RefundBatch 变更、C-04 release event、Payment cancel 表、proto/apperr 新错误码、最终 observe-only/移除（Director release 记录另定） |
| Status | **WIP**；不批准、不声称实现 PASS / Release PASS、不标 AC-VERIFIED |

> 本文件是 Developer 的 canonical 自检交付物，只记录本次实现事实、证据与残余风险，不替代 Reviewer 裁决。

---

## 1. accepted §4.2 五裁决 → 实现映射

| # | 裁决 | 实现落点 | 证据 |
|---|---|---|---|
| **1** | `CANCEL_CAS_LOST` 不返回业务错误，继续组扇出；稳定 reason + log + metric | `internal/services/order/payment_cancel.go:188-196`：CAS 丢失到 PAID/ALLOCATED/SHIPPED/COMPLETED 时 `return true, nil`，不写 payment-level command，log `reason=CANCEL_CAS_LOST`，并 `metrics.PaymentCancelCASLost.WithLabelValues("CANCEL_CAS_LOST","group_fanout").Inc()`；`cancelled_converged`/`cas_lost_unexpected` 为另外两个结果标签 | `TestT1_6PendingCapturedCASLostToPaymentContinuesGroupFanout`；metric 注册 `internal/platform/metrics/metrics.go:98-105`，`Name=payment_cancel_cas_lost_total`，labels `reason/result` |
| **2** | amount 用 `captured-refunded` 净额，0 不写命令 | `payment_cancel.go:56` `paymentCancelAmount = captured - refunded`；`payment_cancel.go:80` 与 `:235` 净额 <=0 直接不写命令；命令 payload `amount_cent=净额`、`paid_cent=captured`（`payment_cancel.go:86-97`） | `TestT1_6PendingCapturedAlreadyRefundedWritesNoCommand`；`TestT1_6RefundCancelledAfterConfirmWithoutCaptureWritesNothing` |
| **3** | Inventory 失败仍写退款命令；失败持久化补偿并告警；命令失败回滚返回错误 | `payment_cancel.go:247-255`：`TccCancelConfirm` 失败 → `log.Error` + `queueInventoryCompensation`，随后仍写命令；`payment_cancel.go:257-261` 命令 append 失败返回错误（outbox 与命令同 tx） | `TestT1_6RefundCancelledAfterConfirmRestocksAndPersistsCommand`；`TestT1_6PendingCapturedOutboxFailureRollsBack` |
| **4** | 不扩 apperr/proto；错误用稳定 `details.reason` + log + metric | Order：`PAYMENT_CANCEL_NO_PAYMENT_NO`（`payment_cancel.go:78,231`）、`PAYMENT_CANCEL_NO_CAPTURE`（`:239`）；Payment：`PAYMENT_REFUND_REQUESTED_OBSERVE_ONLY`（`consumer.go:90`）、`PAYMENT_CANCEL_NO_CAPTURE`（`:105`）、`PAYMENT_CANCEL_REFUND_RETRY`（`:129`）、`PAYMENT_NOT_REFUNDABLE`/`ALREADY_REFUNDED`（`:190-192`）；均有 log + metric 分类 | Payment consumer 单测；order T1.6 集成 |
| **5** | `refundCancelledAfterConfirm` Payment 不可用 fail-closed，不猜 amount、不写命令 | `internal/services/order/service.go:783-798`：先读 `readTccPaymentSnapshot`，失败返回 `Unavailable`，不降级用订单金额、不写命令；`payment_cancel.go:231` 无 `payment_no` 只告警不写命令 | `TestT1_6RefundCancelledAfterConfirmWithoutCaptureWritesNothing`；Payment snapshot 失败路径由 `readTccPaymentSnapshot` 既有 fail-closed 覆盖 |

### 1.1 CAS 丢失 metric 契约

- 指标：`payment_cancel_cas_lost_total{reason,result}`（`internal/platform/metrics/metrics.go:102-105`），已注册（`:251`）。
- `reason` 固定为 `CANCEL_CAS_LOST`，与 log 的 `reason`、`apperr.Details["reason"]`（`payment_cancel.go:213`）一致。
- 结果标签：
  - `group_fanout`：order 已 PAID/生命周期推进，继续 §4.1 组扇出；
  - `cancelled_converged`：并发 cancel 已胜出，走幂等 restock+命令修复；
  - `cas_lost_unexpected`：CAS 丢失到不可取消状态，fail-closed 返回 Conflict。
- 本 WIP 未给 metric 值加单测（避免为读取 Counter 引入 `client_model`/`testutil` 直接依赖）；增量点由 CAS-lost 集成测试执行路径覆盖，指标名/标签/注册由代码常量绑定。

---

## 2. 反妥协自检清单（三问）

### 2.1 并发冲突 / 网络超时 / 部分成功时会发生什么？处理了吗？

- **PENDING+captured 取消**：`SELECT orders FOR UPDATE` + `UPDATE ... WHERE status=PENDING_PAYMENT` CAS；CAS 成功才写 `order.cancelled` 与 payment command（同一 `*sql.Tx`）；失败/0 行回滚，重读后按 PAID 组扇出或 CANCELLED 幂等修复。`TestT1_6ConcurrentPendingCapturedCancelSingleWinner`（6 并发，`-race`）证明单 winner、1 个 `order.cancelled`、所有命令同一 `refund_id`。
- **outbox 失败**：`TestT1_6PendingCapturedOutboxFailureRollsBack` 用 MySQL trigger 在 `order.refund_requested` 插入时失败，断言 order 仍 PENDING、version 不变、无 `order.cancelled`/command/status log/refund batch。
- **Payment 超时/拒绝**：consumer 对 `Unavailable/Internal/RateLimited` 返回错误交给 durable retry；对 `Invalid/NotFound/FailedPrecondition/Conflict` ack + Error 告警，避免重退；固定 `IdempotencyKey=pay:<payment_no>:cancel`。`TestHandleOrderRefundRequestedRetriesTransientFailure`、`...AcksRejectedRefund`、`TestOrderRefundRequestedCommandIsIdempotent`（两次投递只 1 refund/1 event）、`TestOrderRefundRequestedAlreadyRefundedIsAcked`。
- **Inventory 部分失败**：`refundCancelledWithSnapshot` 先 `TccCancelConfirm`（HELD release / CONFIRMED restock），失败持久化 `inventory.compensation_requested` 并 Error；不阻塞 Payment 退款。
- **重复/乱序**：固定 `refund_id`；Payment.Refund 按 `refund_id` 幂等；`payment.refunded` 带 `non_effective_capture=true`，Order 投影忽略（既有 `TestPaymentRefundedIgnoresNonEffectiveCapture`）。重复取消只重发同一命令，不产生第二个 `order.cancelled`。

### 2.2 是否把状态揉进一个字段 / 加 if-else 绕过架构？

- 未新增订单状态字段；支付级取消使用独立命令身份 `pay:<payment_no>:cancel`，不建组级 `refund_batches/applications/admissions/commitments`，不改 permit。
- CAS 丢失按协议分流（PAID→组扇出、CANCELLED→幂等修复、其他→fail-closed），不是场景补丁；所有分支共享同一 CAS/command 原语。
- 未把 `captured` 写入 `orders.paid_cent`（测试断言 PENDING 取消后仍为 0），符合 AC-53 “PAID 前捕获只用于退款判断”。

### 2.3 向现实做了哪些妥协（重大妥协高亮）

1. **【重大/已裁决】Inventory 失败仍发退款命令**：§4.2 裁决 3 接受；资金不被回库阻塞，库存靠持久补偿 + `order.cancelled` consumer 双路径恢复。
2. **【已裁决】错误码仅 reason/log/metric**：§4.2 裁决 4 接受；不扩 apperr/proto，Reviewer 需按稳定 reason 审查。
3. **【已裁决】`CANCEL_CAS_LOST` 不返回业务错误**：§4.2 裁决 1 接受；组扇出成功即成功，metric/log 记录竞态。
4. **metric 无 counter 值单测**：为避免新增 `client_model`/`testutil` 直接依赖；指标名/标签/注册代码固定，执行路径由 CAS-lost 测试覆盖。
5. **无 broker e2e**：outbox relay→NATS→consumer 的真实重投/DLQ 由 T5.2/T5.3 补；本次以 producer outbox 行 + consumer handler/DB 集成测试覆盖。

---

## 3. [修改点] / [架构侵入性] / [副作用评估]

### 3.1 [修改点]

| 文件 | 修改 |
|---|---|
| `internal/services/order/payment_cancel.go` | 新增：`paymentCancelRefundID`、`paymentCancelAmount`、`appendPaymentCancelCommandTx`、`cancelPendingCapturedAfterTCC`、`refundCancelledWithSnapshot`；CAS-lost metric increments |
| `internal/services/order/tcc.go` | `TccCancelMarkPaid` PENDING+captured 入口替换 T1.2 R-3 guard；CANCELLED+captured 走幂等命令修复；CAS 丢失继续组扇出 |
| `internal/services/order/service.go` | `refundCancelledAfterConfirm` 改为返回错误、读 Payment 权威快照、Inventory 先补偿、命令 append 失败 `MarkPaid` 返回 INTERNAL |
| `internal/services/order/tcc_settlement.go` | 仅更新 defense-in-depth 错误文案/注释，原 fail-closed 行为保留 |
| `internal/services/payment/consumer.go` | command/observe-only 分类、legacy cancel-after-confirm 映射、`refund_id` 幂等键、`non_effective_capture`、超时重试/拒绝 ack |
| `internal/platform/events/registry.go` | `orderEvent` 可选 `refund_id/source/non_effective_capture/amount_cent`，required 不变 |
| `internal/platform/metrics/metrics.go` | 新增 `payment_cancel_cas_lost_total{reason,result}` 并注册 |
| tests | order T1.6 集成、payment consumer 单测/集成、inventory HELD/CONFIRMED、events 兼容性 |

### 3.2 [架构侵入性]

- 新增一个 Prometheus CounterVec（无 proto/RPC/migration/表）。
- 支付级取消复用既有 `order.refund_requested` subject 与 `Payment.Refund`，未新增命令 subject。
- 不改变 C-04 RefundBatch、C-04 release event、permit/commitment 语义。

### 3.3 [副作用评估]

- `order.refund_requested` 现在同时承载 payment-level 命令；TCC 组级路径不再发该事件。Consumer 对非 `pay:*`/legacy 事件 observe-only，避免旧/未知事件误退款。
- `MarkPaid` 在“取消已胜出且退款命令持久化失败”时返回 INTERNAL 而非 CONFLICT；这是 fail-closed，调用方需重试。
- 新增 metric 名不带 `mall_` 前缀，按 accepted §4.2 裁决 1 的固定名；不影响既有 `mall_payment_refund_events_total`。
- 重复投递可能产生多条同一 `refund_id` 的 outbox 命令；Payment 幂等吸收，不重复退款。

---

## 4. 测试证据（真实 MySQL；绑定本 WIP 提交）

环境：`go1.27.1 darwin/amd64`；MySQL `mysql-unicorn`（`127.0.0.1:3306`）；`TEST_MYSQL_DSN` 按包派生独立 schema；`GOCACHE=/tmp/dsh-go-cache`。

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空（无漂移） |
| `go build ./...` | OK |
| `go test ./internal/platform/events -count=1` | `ok ... 0.136s` |
| `TEST_MYSQL_DSN=<payment> go test ./internal/services/payment -count=1` | `ok ... 6.714s` |
| `TEST_MYSQL_DSN=<order> go test ./internal/services/order -count=1` | `ok ... 149.637s` |
| `TEST_MYSQL_DSN=<order> go test -race -p 1 ./internal/services/order -run 'TestT1_6\|TestTccCancel' -count=1` | `ok ... 44.513s` |
| `TEST_MYSQL_DSN=<payment> go test -race -p 1 ./internal/services/payment -count=1` | `ok ... 6.315s` |
| `go mod tidy -diff` | 仅既有 `genproto/googleapis/rpc` direct/indirect 漂移（非本次引入；未执行 tidy） |

### 4.1 覆盖矩阵

| 场景 | 测试 |
|---|---|
| PENDING+captured，CAS 成功 | `TestT1_6PendingCapturedCASWins`：CANCELLED、1 个 `order.cancelled`、1 个命令、无组 ledger、不调 Payment.Refund |
| CAS 丢失到已 PAID | `TestT1_6PendingCapturedCASLostToPaymentContinuesGroupFanout`：无 payment-level 命令，`refund_batches=1`，log 含 `CANCEL_CAS_LOST` |
| CAS 失败/并发 | `TestT1_6ConcurrentPendingCapturedCancelSingleWinner`（`-race`）：单 winner、version+1、同一 `refund_id` |
| outbox 失败 | `TestT1_6PendingCapturedOutboxFailureRollsBack`：全回滚、无半状态 |
| 已全额退/净额 0 | `TestT1_6PendingCapturedAlreadyRefundedWritesNoCommand` |
| CANCELLED+captured | `TestT1_6CancelledCapturedWritesCommand`：命令存在、无组 batch |
| cancel-after-confirm CONFIRMED | `TestT1_6RefundCancelledAfterConfirmRestocksAndPersistsCommand`：`TccCancelConfirm=1`、权威 captured 800（请求 1000） |
| HELD/CONFIRMED 库存补偿 | `TestOrderCancelledCompensatesHeldAndConfirmed`（inventory：HELD release、CONFIRMED restock） |
| 无 captured/payment_no | `TestT1_6RefundCancelledAfterConfirmWithoutCaptureWritesNothing`；`TestHandleOrderRefundRequestedNoCapturedDoesNotRefund` |
| Payment 超时/拒绝/已退 | `TestHandleOrderRefundRequestedRetriesTransientFailure`、`...AcksRejectedRefund`、`TestOrderRefundRequestedAlreadyRefundedIsAcked` |
| consumer observe-only | `TestHandleOrderRefundRequestedObservesForeignCommands` |
| idempotency_key/non_effective | `TestHandleOrderRefundRequestedExecutesPaymentCancelCommand`、`TestOrderRefundRequestedCommandIsIdempotent`（2 次投递 = 1 refund/1 event） |
| legacy 缺 refund_id | `TestHandleOrderRefundRequestedLegacyCancelAfterConfirm` |
| registry 可选字段兼容 | `TestOrderRefundRequestedPaymentCancelFieldsAreOptional`（legacy/minimal/new 三种 payload 均通过） |

---

## 5. 【测试范围】与【未覆盖范围】

### 5.1 测试范围

- Order：PENDING CAS 成功/丢失/并发、CANCELLED+captured、cancel-after-confirm、outbox 回滚、净额 0、重复命令、权威金额。
- Payment：command/observe-only、legacy 映射、超时重试、确定性拒绝、已全额退、幂等键、`non_effective_capture`。
- Inventory：`order.cancelled` 对 HELD/CONFIRMED 的复用补偿。
- Events：`order.refund_requested` legacy/minimal/new 可选字段兼容。

### 5.2 未覆盖范围

1. NATS broker 端到端重投/DLQ（T5.2/T5.3）；本次仅 handler + MySQL。
2. 真实外部渠道超时；以 `apperr.Unavailable` 注入。
3. `payment_cancel_cas_lost_total` 计数器值单测（避免新增 test 依赖；指标注册/增量点由代码审查 + 执行路径覆盖）。
4. 用户 `CancelOrder` 与 TCC Cancel 的受控交错；用并发 TccCancelMarkPaid + `beforeGet` 钩子覆盖。
5. `order.cancelled` / `payment.refunded` 跨 broker 乱序；既有 non-effective 投影测试覆盖。
6. AC 台账/AC-VERIFIED（Owner=Reviewer）、Release PASS（人类/集成 owner）。

---

## 6. 残余风险与需 Reviewer 确认点

1. **metric 命名**：`payment_cancel_cas_lost_total` 不带 `mall_` 前缀，按 accepted §4.2 裁决 1 固定名；若仓库要求统一前缀需 Architect 修订。
2. **错误码仅 reason**：Reviewer 需接受 §4.2 裁决 4 的 WIP 口径，不得要求新增 proto/apperr。
3. **Inventory 失败仍退款**：§4.2 裁决 3 已接受；Reviewer 需核对持久补偿 + 告警存在。
4. **`CANCEL_CAS_LOST` 不返回错误**：§4.2 裁决 1 已接受；Reviewer 需核对不写 payment-level command。
5. **net amount**：`captured-refunded`，与 ADR r1.0 一致；部分退款后不会超退。
6. **命令重复**：同 `refund_id` 可能有多条 outbox；由 Payment 幂等吸收，非订单侧去重。
7. **无 AC-VERIFIED**：本清单不改变 `docs/collab/ac-coverage.tsv`。

---

## 7. 绑定与声明

- **Base**：`5093a1d`；**Code head**：本 WIP 提交（`git log -1`）。
- **Tier**：L2；commit trailer 包含 `change_id`/`Role`/`Actor`/`Status`/`Tier`；无 `Reviewed-by`。
- **不批准**：本清单不构成实现 PASS、Reviewer PASS、Release PASS 或 AC-VERIFIED；不批准合入。
