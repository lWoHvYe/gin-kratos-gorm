# PAY-SETTLE-01 T1.4b1-R2 Developer 自检清单

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Role / Actor | Developer / `dev-phase1f` |
| Plan | `docs/lead/PAY-SETTLE-01/t1.4b1-r-plan.md`；**Plan 审批状态由 Lead 维护，本文不写、不代批** |
| 契约输入 | SPEC-0002 **v1.9**（approved 2026-11-24）；ADR-0007 Amd1（accepted）；C-04 RefundBatch（accepted 2026-11-24）；T1.4b1-R Plan |
| Base | `ba8a822`（T1.4b1-R Order 预检 + RefundBatch 接线 WIP） |
| Code head（本清单绑定） | **`6ac218e`**（`wip(order): T1.4b1-R2 durable TCC reconcile and order projection`） |
| Scope | **仅 T1.4b1-R2**：持久 `RunTccReconcileWorker`、`GetRefundByKey` 惰性收敛、order 金额投影；不新增契约/迁移 |
| 不做 | T1.4b2/INV-44 release/回库；零元/Aftersales 持久 task；T1.5；T1.6；T1.7/T5 全矩阵；不改 Spec/ADR/proto/migrations |
| Status | **WIP**；不批准、不宣称实现 PASS / Release PASS，不标 AC-VERIFIED |

> 本文只记录 Developer 的实现事实、测试证据和残余风险，不替代 Reviewer 裁决。Plan 审批状态不在本文职责内。

---

## 1. 反妥协自检清单（三问）

### 1.1 并发冲突 / 网络超时 / 部分成功时会发生什么？处理了吗？

- **Payment 事实已提交、事件丢失**：`RunTccReconcileWorker` 扫描 `ADMITTED/PAYING` 的 active TCC batch，对每个 application 调 `Payment.GetRefundByKey`；`SUCCEEDED/REFUNDED` 时在同一 order-locked 事务内完成：
  - commitment `RESERVED -> SETTLED`（CAS；**唯一金额触发**）；
  - application `PENDING/PROCESSING -> DONE`；
  - 全部 application `DONE` 后 batch `DONE`（`ADMITTED` 先经 `PAYING`）；
  - `orders.refunded_cent/refund_status/refund_state` 单调投影。
  没有 Payment 事实（NotFound/PENDING/PROCESSING/FAILED）时保持原状，不猜测、不打款（`TestTccReconcileWorkerLeavesOutstandingWithoutFact`）。
- **事件 + worker 双写**：两者都锁同一 order 行；投影值取 `max(current, SUM(SETTLED commitments))`，**不是增量**，因此 event 的累计 `OrderRefundedCent` 与 worker 的单组结算不会双计。commitment CAS 成功才视为本次结算的金额触发；CAS 失败（event/另一副本已结算）只补 application DONE，不再加金额（`TestTccReconcileWorkerDoesNotDoubleCountWithEvent`）。
- **并发 worker/多副本**：所有写都在 order 行锁内，commitment 是 CAS，application/batch 是幂等更新；重复扫描/并发扫描不会重复投影。8 并发 worker 断言 `refunded_cent=10000`（不是 20000/80000），且 DONE 后再跑 bump 0 次 order version（`TestTccReconcileWorkerConcurrentIsIdempotent`，`-race`）。
- **部分成功**：有事实的组结算并投影；没有事实的组不动，batch 保持 `PAYING`、order 保持 `REFUNDING`；只有全部组 DONE 且无其它未完成 TCC batch 才 `PARTIAL/FULL`（`TestTccReconcileWorkerKeepsRefundingForPartialBatch`）。已结算的组不会因重扫回滚。
- **GetRefundByKey 抖动**：worker 只记录并跳过该 application（不 park batch、不写账本）；下一轮重试。pay-path 预检仍按 T1.4b1-R 的瞬时/确定性错误语义处理。
- **状态竞争**：worker 不覆盖 `MANUAL_HOLD/FAILED_PERMANENT`；`applyTccReconcileTx` 只接受 `ADMITTED/PAYING/DONE`，`convergeTccBatchTx` 不会把未完成 application 的 batch 置 DONE。
- **order 金额投影**：active TCC batch（状态不是 `DONE/PRECHECK_FAILED`，含 `MANUAL_HOLD/FAILED_PERMANENT`）存在时固定 `refund_state=REFUNDING`；全部完成后按 `refunded_cent vs paid_cent` 派生 `PARTIAL/FULL`，`refund_status` 同步为 `PARTIALLY_REFUNDED/REFUNDED`；已 `FULL` 不回退。
- **pay-path lazy reconcile**：`payTccBatch` 的 `GetRefundByKey` 成功分支复用同一 `settleTccRefundFactTx/convergeTccBatchTx/projectTccReconcileOrderTx`，所以即便 worker 尚未 tick，预检已能收敛 ledger + order 金额；不会出现 batch DONE 而 order 金额永远停在旧值。

### 1.2 是否把状态揉进一个字段 / 加 if-else 绕过架构？

- 未新增状态字段、未新增 proto/迁移、未改 Spec/ADR：沿用 batch `ADMITTED/PAYING/DONE`、application `PENDING/PROCESSING/DONE`、commitment `RESERVED/SETTLED`。
- 未新增第二套退款裁决：worker 只读 Payment 事实，不调用 `RefundBatch`/`Refund`、不改 permit、不调 Inventory。
- 金额投影没有新增“已投影”标记：以 commitment `RESERVED->SETTLED` 的 CAS 作为唯一金额触发；投影用 settled 总和的下界取 `max`，避免双计而不需要迁移。
- 唯一新增的本地策略是 worker 扫描周期与每批 100 条上限；不影响契约状态机。

### 1.3 向现实做了哪些妥协（重大妥协高亮）

1. **【重大】Payment.GetRefundByKey 仍无服务端实现**：proto/generated client 存在（`api/proto/mall/v1/mall.proto:782`、`api/gen/mall/v1/mall_grpc.pb.go:4078`），但 `internal/services/payment` 仍嵌入 `UnimplementedPaymentServiceServer` 且无 `GetRefundByKey` 方法。真实部署下 worker/preflight 会收到 gRPC `Unimplemented`：worker 每轮告警、不收敛；pay-path 预检会 fail-closed。**本 WIP 的 worker 只能用 fake Payment 验证，端到端被该缺口阻塞**（见 §5-1）。
2. **order 金额投影下界是 SETTLED commitments 之和**：TCC 组退款都有 commitment，因此 TCC 场景可完整投影；没有 commitment 的支付级/历史退款仍只依赖 `payment.refunded`（其 `OrderRefundedCent` 是累计值）。本任务不改变支付级退款的事实所有权。
3. **worker 是安全网，不是实时路径**：默认 1 分钟、每批最多 100 条；order 金额在 worker tick、pay-path resume 或事件到达前可能短暂滞后。未实现 metrics/告警/手动触发，只写结构化日志。
4. **无 `payment.refunded` 时订单 lifecycle 不变**：worker 只写 `refund_state/refund_status/refunded_cent`，不改 `orders.status`，也不发 `order.refund_requested`。
5. **`MANUAL_HOLD/FAILED_PERMANENT` 被 worker 排除在自动推进之外**，且投影查询把它们视为“未完成”，因此 order 保持 `REFUNDING` 等待人工；本任务不提供恢复命令。

---

## 2. [修改点] / [架构侵入性] / [副作用评估]

### 2.1 [修改点]

| 文件 | 修改 |
|---|---|
| `internal/services/order/ports.go`（`ba8a822`） | `paymentPort` 增 `GetRefundByKey`、`RefundBatch`；TCC 支付只走原子批。 |
| `internal/services/order/tcc_payment.go`（`ba8a822` + `6ac218e`） | `payTccBatch` 预检 `B/done/inFlight/outstanding/R`、单次 `RefundBatch`、错误映射；`reconcileTccRefundFact` 改为复用 R2 的 settle/converge/project helper，pay-path 也投影 order 金额。 |
| `internal/services/order/tcc_reconcile.go`（新增，`6ac218e`） | `RunTccReconcileWorker` + `runTccReconcileOnce`：扫描 `ADMITTED/PAYING` TCC batch，读 `GetRefundByKey`，在 order 行锁内 CAS commitment/app/batch，并投影 `orders.refunded_cent/refund_status/refund_state`；`max(current, SUM(SETTLED))` 防双计；active batch 保持 `REFUNDING`。 |
| `cmd/order/main.go`（`6ac218e`） | 注册 `order.RunTccReconcileWorker(..., cfg.TccReconcileInterval)`。 |
| `internal/platform/config/config.go`（`6ac218e`） | 新增 `TccReconcileInterval`，默认 1m；`TCC_RECONCILE_INTERVAL` 必须是正 duration，否则启动报错。 |
| `internal/platform/config/config_test.go`（`6ac218e`） | 覆盖 interval 解析/非法值。 |
| `internal/services/order/tcc_reconcile_integration_test.go`（新增） | 5 个真实 MySQL worker 用例：事件丢失收敛+投影、部分批次保持 REFUNDING、无事实无写入、8 并发幂等、event+worker 不双计。 |
| `internal/services/order/tcc_payment_integration_test.go`（`6ac218e`） | pay-path lazy reconcile 增加 order 金额/状态断言。 |
| `internal/services/order/tcc_compensation_integration_test.go`（`6ac218e`） | ALLOCATED resume 在全部 Payment 事实已存在时期望 `ALLOCATED/FULL`（lifecycle 不变）。 |

### 2.2 [架构侵入性]

- 新增 1 个后台 worker 和 1 组共享 settle/project helper；没有“默认放行”分支：未知/不匹配的 commitment、application、fact 都跳过并告警，不写金额。
- 状态所有权不变：Order 仍唯一写 `refund_batches/refund_applications/order_refund_commitments/orders`；Payment 仍是退款事实 owner；worker 只读 Payment。
- 未改订单 lifecycle、未新增跨服务写、未改 proto/迁移、未碰 Inventory。
- order 金额投影新增一个写入者（worker/pay-path），但通过 order 行锁 + commitment CAS + `max` 下界与 event 消费保持同一事实口径；event 的累计投影行为未改。

### 2.3 [副作用评估]

- `ADMITTED` batch 在 worker 看到 Payment 事实后会先转 `PAYING`，全部组 DONE 后 `DONE`；`ADMITTED` 不再是“有事实但未推进”的悬挂态。
- order `refund_state` 在 active batch 期间保持 `REFUNDING`（包括 `MANUAL_HOLD`）；全部完成才 `PARTIAL/FULL`。已 `FULL` 不回退。
- 新增 `order.refund_projected` outbox 事件（worker/pay-path 投影时）与 `order_status_logs(operator='tcc-reconcile')`；重复投影不重复写（金额/状态/状态机均未变化时直接返回）。
- `TCC_RECONCILE_INTERVAL` 非法值导致 `config.Load` 失败（fail-closed）；默认 1m。
- 每轮扫描最多 100 个 active batch；大量积压时下一轮继续，未做分页游标。

---

## 3. 测试证据（真实 MySQL；绑定 `6ac218e`，Base=`ba8a822`）

环境：`go1.27.1 darwin/amd64`；MySQL `mysql-unicorn`（`127.0.0.1:3306`，`26.10.0`）；`TEST_MYSQL_DSN=root:root@tcp(127.0.0.1:3306)/mall_integration_test?parseTime=true`；`GOCACHE=<workspace>/.gocache`（默认缓存被环境 sandbox 拒绝，非仓库变更）。

| 命令 | 结果 |
|---|---|
| `go build ./...` | `BUILD_OK` |
| `go test ./internal/platform/config -count=1` | `ok ... 0.112s` |
| `go test ./internal/services/order -count=1` | `ok ... 121.828s` |
| `go test -race -p 1 ./internal/services/order -run 'TestTcc' -count=1` | `ok ... 90.874s` |
| `gofmt -l <changed files>` | 空（`internal/services/order/service.go` 的既有漂移未触碰） |

### 3.1 新增 worker 用例（不投递 `payment.refunded`）

```text
--- PASS: TestTccReconcileWorkerSettlesLostEventsAndProjectsOrder (1.28s)
--- PASS: TestTccReconcileWorkerKeepsRefundingForPartialBatch (1.46s)
--- PASS: TestTccReconcileWorkerLeavesOutstandingWithoutFact (1.32s)
--- PASS: TestTccReconcileWorkerConcurrentIsIdempotent (1.67s)
--- PASS: TestTccReconcileWorkerDoesNotDoubleCountWithEvent (1.22s)
ok  gin-kratos-gorm/internal/services/order  7.054s
```

### 3.2 既有 T1.4b1-R 用例（回归）

`TestTccRefundBatch*` 14 个顶层用例 + external 1000/6000/4000 子用例全部 PASS（见 §3.3 命令输出），包括：原子单批、BATCH_UNAVAILABLE 重试、MANUAL_HOLD、同 batch 不同 items、in-flight、GetRefundByKey 超时、外部退款零 Payment、部分/全量 lazy reconcile。pay-path lazy reconcile 现在同时断言 `orders.refunded_cent/refund_status/refund_state`。

```text
ok  gin-kratos-gorm/internal/services/order  121.828s   # full order package
ok  gin-kratos-gorm/internal/services/order  90.874s    # -race TestTcc
```

---

## 4. 【测试范围】与【未覆盖范围】

### 4.1 【测试范围】

- 不投递 `payment.refunded`，仅运行 worker：batch DONE、commitment SETTLED、application DONE、order `refunded_cent/refund_status/refund_state` 正确。
- 部分组有事实/部分无事实：有事实的组结算并投影，batch 保持 PAYING、order 保持 REFUNDING；后续事实到齐后 FULL。
- 无 Payment 事实（NotFound/PENDING）：零写入、零投影。
- 重复/并发 worker：金额不双计、DONE 后不再 bump version。
- 事件 + worker：event 先投影累计值，worker 只补剩余 commitment，最终 10000 而非 16000；迟到 event 幂等。
- pay-path lazy reconcile 与 worker 共用投影：batch DONE 时 order 金额同步，不留“batch DONE、order 旧值”。
- 真实 MySQL（真实 order 迁移 0017/0018/0021）+ `-race`。

### 4.2 【未覆盖范围】

1. **真实 Payment 进程**：`GetRefundByKey`/`RefundBatch` 仍由 fake 模拟；Payment 真实服务端 `GetRefundByKey` 实现不在本任务（见 §5-1）。
2. **worker 运维面**：无 metrics、无手动触发、无死信/积压分页游标；只有日志。
3. **多副本真实集群**：只在单进程内 8 并发 goroutine + 行锁/CAS 验证；未做多 Pod 网络分区/时钟实验。
4. **支付级/无 commitment 退款**：worker 的金额下界只来自 SETTLED commitments；这类退款仍只由 `payment.refunded` 投影（事实所有权不变）。
5. **INV-44 release/回库、零元/Aftersales、T1.5/T1.6、T1.7/T5 全矩阵**：按人类裁决不在本任务。
6. **`MANUAL_HOLD` 恢复命令**、跨日对账报表、生产 MySQL/集群验证：不在本任务。

---

## 5. 契约缺口 / 需裁决事项（不自行扩展）

1. **【阻塞】Payment.GetRefundByKey 无服务端实现**：proto/generated client 已 accepted，`internal/services/payment/service.go` 仍嵌入 `UnimplementedPaymentServiceServer`，无 `func (s *Service) GetRefundByKey`。真实部署下 worker 只能告警、pay-path 预检 fail-closed；**T1.4b1-R2 的 worker 端到端不可用，直到 Payment 实现该只读 RPC**。建议 Payment owner 按 C-04 实现 `(payment_no, refund_id)` 查询/NotFound，不改 proto。
2. **Payment.RefundBatch 仍是 WIP `fc295e5`**：C-04 accepted ≠ 实现 PASS；Order 测试使用 fake Payment 模拟行锁/原子/幂等语义。真实组合验证需 Payment 实现 + `0012_refund_batch.sql`。
3. **order 金额投影下界**：`max(current, SUM(SETTLED commitments))` 不覆盖无 commitment 的支付级退款；若事件永久丢失，这类金额仍会滞后。是否需要 Payment 级累计读（`ListOrderIntents`/`GetIntent`）作为第二下界，需 Architect 裁决。
4. **worker 周期/超时**：`TCC_RECONCILE_INTERVAL` 默认 1m 是本任务本地运维值；C-04 未定义。in-flight 30m 阈值仍沿用 `ba8a822` 的本地策略。
5. **`MANUAL_HOLD/FAILED_PERMANENT` 的投影**：worker 不推进这些 batch；投影查询把它们视为未完成以保持 `REFUNDING`。若业务希望人工态也允许金额投影到 `PARTIAL/FULL`，需状态机裁决。
6. **outbox 事件 payload**：worker 的 `order.refund_projected` 用 `batch.CapturedSnapshotCent` 作为 `captured_cent`；多 payment intent 订单是否应改用订单级 captured 汇总，需事件契约 owner 确认。
7. **`make fmt-check` 基线失败**：`internal/services/order/service.go` 在 base 即未 gofmt（非本任务修改）；本任务改动文件 `gofmt -l` 为空。

---

## 6. 结论

- 本提交完成 **T1.4b1-R2**：持久 TCC reconcile worker + `GetRefundByKey` 惰性收敛 + order 金额/状态投影；不依赖 `payment.refunded`，event+worker 不双计，并发/重复幂等。
- 未改 Spec/ADR/proto/migrations；未做 T1.4b2/INV-44/零元/T1.5/T1.6；未调 Inventory。
- **Status: WIP**；不批准、不宣称实现 PASS / Release PASS，不标 AC-VERIFIED。真实端到端被 Payment.GetRefundByKey 服务端缺口阻塞。
