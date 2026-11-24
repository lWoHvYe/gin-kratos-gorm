# PAY-SETTLE-01 T1.2-R Developer 自检清单

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Role / Actor | Developer / `dev-phase1b` |
| Plan | `docs/lead/PAY-SETTLE-01/t1.2-r-plan.md`（Director approved 2026-11-24） |
| Review input | `docs/collab/qc-reports/REV-PAY-SETTLE-01-T1.2.md`（B1/B2/M1/M2/m1/m2/m6） |
| Base | `4e5d07f`（T1.2b WIP，Reviewer FAIL 范围终点） |
| Code head（本清单绑定的实现 HEAD） | `e7e29e5`（`wip(order): T1.2-R lifecycle gate, legacy composition and no-active refusals`） |
| Docs head | 包含本文件的 WIP commit（`e7e29e5` 之后仅 docs） |
| Scope | T1.2/T1.2b/T1.3 的 B1/M1/M2/m1 修复；**不含** T1.4/T1.5/T1.6/T1.7/T2/T3/T5 |
| Status | **WIP**；不批准、不宣称实现 PASS/Release PASS，不标 AC-VERIFIED |

> 本文件是 Developer 的 canonical 自检交付物（Reviewer B2）。它只记录本实现的事实与残余风险，不替代 Reviewer 裁决，也不构成任何 AC 验收。

---

## 1. 反妥协自检清单（三问，强制）

### 1.1 并发冲突 / 网络超时 / 部分成功时会发生什么？处理了吗？

- **订单锁串行化**：Tx1 先 `SELECT orders ... FOR UPDATE`，同一订单的 TCC 补偿、Ship、退款准入只能串行；batch 的唯一活动 attempt 由该锁 + `state` 判定保证（`refund_batches` 无 `(source,order_no,payment_no)` UNIQUE，ADR §6.1）。
- **同一 Payment 快照**：事务外 `Payment.GetIntent` 取权威 captured/refunded；Tx1 锁内用**同一快照**重算 Σ，锁内零远程调用。快照与锁内 `settlement_payment_no` 不一致 → fail-closed（`TestTccSumTxRecheckUsesSameSnapshot`）。
- **并发 TCC cancel**：第二个请求可能在预检后撞上第一个请求已冻结的 batch。实现检测 `refund_state=REFUNDING` 后按 batch 续跑；若严格 Σ 因“本补偿自己的承诺”看起来不足，则收敛到已有 batch（`TestTccCancelConcurrentAndRepeatedIsIdempotent`、`TestTccCancelConcurrentPrecheckConvergesOnFrozenBatch`）。
- **部分成功/崩溃**：Tx1 是单事务，订单态、batch、application、permit、admission、commitment 同进同退，不会留下半写状态；外部人为删行由 resume 幂等补齐（`TestTccCancelResumeFillsMissingApplicationsAndPermits`）。已 `DONE` 的 application 组跳过，不重退。
- **超时/不可用**：Payment 不可用 → `UNAVAILABLE` fail-closed；Fulfillment `ListShipments` 超时/返回不完整/多 live shipment/exception 无 `shipped_at` → fail-closed 且零写入（`TestTccCancelShipmentFactsUnavailableFailsClosed` 等）；Tx1 内没有任何远程调用，因此不会出现“事务持锁等待网络”。
- **审计竞争**：`PRECHECK_FAILED` 的 `MAX(attempt_seq)+1` 在事务外分配，重复键最多重试 3 次后记录错误日志；不会因此产生错误退款（资金路径仍 fail-closed），但极端并发仍可能丢失一条审计（**残余风险，见 §4-m5**）。
- **人工挂起**：`MANUAL_HOLD/FAILED_PERMANENT` 不再进入 resumable，自动重试返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING` 且零写入（`TestTccCancelManualHoldBatchIsNotResumed`），避免 DTM 重试绕过人工处置。

### 1.2 我是不是为了图方便，又把状态揉进一个字段里？或加了大量 `if-else` 绕过架构？

- 没有把生命周期与退款态合并：订单 `status` 与 `refund_state` 仍然正交；TCC 只写 `refund_state=REFUNDING`，**不把订单改成 CANCELLED/REFUNDING 生命周期态**（R-1；`TestTccCancelAllocatedOrderMixedGroups` 断言 `ALLOCATED` 保持）。
- B1 的生命周期 gate 是**显式封闭集合 + 未知状态 fail-closed**，不是“默认 `return nil` 放行”的补丁：`PENDING_PAYMENT/PAID/ALLOCATED/SHIPPED/COMPLETED` 允许；`CANCELLED+paid>0` 返回 `FAILED_PRECONDITION`（T1.6 owner）；`CLOSED` 等未知状态返回 `TCC_ORDER_STATUS_UNSUPPORTED`。Tx1 内有同样 gate（锁内 `status` 可能已推进），不存在“入口放过、锁内静默”。
- M1 不是“多写一个 if 跳过校验”：它在数据模型上区分 `composition_known=1`（严格行校验）与 `composition_known=0`（整组额度、无行归因、新 TCC 承诺 known=0/无行），未知组成的行不再被当作“可归因”。`composition_known=1` 的坏行仍 fail-closed（`TestTccCancelKnownCompositionLineMismatchFailsClosed`）。
- M2 是 Σ 判定之后的显式“无活动组”分支（`Σ=remaining=0` 幂等成功；`active=0 && remaining>0` 仍由 Σ mismatch 写审计），没有“先建空 batch 再说”的兜底。
- 没有新增 GORM/新依赖/新 RPC；未改 proto/migrations/Spec/ADR/consumer；`ListShipments` 复用既有 C-04 RPC（`ports.go`）。

### 1.3 我向现实做了哪些妥协？（重大妥协高亮）

1. **【重大】T1.2 成功 ≠ 钱已退**：`AdmitRefund`、`Fulfillment.CancelIfCreated`、逐组 `Payment.Refund`、`payment.refunded` 收敛、`MANUAL_HOLD/PAYING` 状态推进均属 T1.4/T1.6，本任务不实现。Tx1 提交后的 batch/application 目前**没有 worker 驱动**；WIP 不得合入 main，也不得把 `TccCancelMarkPaid` 的 `return nil` 当退款完成。
2. **【重大】CANCELLED 信任边界**：R-3 明确 guard 不得调用 Payment，因此 `CANCELLED + 请求/订单 paid_cent<=0` 仍按幂等成功处理；若请求谎报 0 而实际 captured>0，本实现无法识别，必须由 T1.6 用 Payment 事实收敛（Review m4，Addendum R-3 已接受该信任边界）。
3. **历史 commit 组件的“商品/运费”拆分是约定，不是归因**：`composition_known=0` 组按 ADR/migration 约定把已占用额度视为商品（`goodsCap = paid - committedTotal`，`freightCap = freight_share`），仅用于满足 `refund_applications` 的 `goods+freight=amount` CHECK；新 TCC commitment 写 `composition_known=0` 且**不写行**，不声称任何行级归属。需要行级部分退款时必须人工/T3.4 补齐组成。
4. **T1.4 的 `CancelIfCreated` 语义未实现**：`exception_state != NONE && shipped_at` 在 Order 侧判 POST；T1.4 必须让 Fulfillment 返回 `ALREADY_SHIPPED`，不得落无限 `TRANSIENT_ERROR`（仅留边界注释，未改 Fulfillment 包）。
5. **审计只有日志、没有 metrics**：`PRECHECK_FAILED` 以 `s.log.Warn` 记录；仓库内 Order 服务没有现成 counter 注册点，本轮未新增基础设施。
6. **测试数据库**：order 集成测试现在用**真实 `migrations/order/0017/0018`** 建 refund 表/列/CHECK；其余表仍是测试内联 DDL（T5.4 负责完整真实迁移/回填，不在本任务）。

---

## 2. 本轮回工影响声明

### 2.1 [修改点]

| 文件 | 修改 |
|---|---|
| `internal/services/order/tcc.go` | 入口生命周期 gate（`CANCELLED` 分支、允许集合、未知 fail-closed）；`tccCancelDecision` 支持 `ALLOCATED/SHIPPED/COMPLETED`；付费/捕获检查位置调整（先 `paid`，再 CANCELLED，再 R-3）。 |
| `internal/services/order/tcc_settlement.go` | Tx1 生命周期 gate；`TCC_CANCELLED_CAPTURED_NOT_IMPLEMENTED`/`TCC_ORDER_STATUS_UNSUPPORTED`/`TCC_BATCH_REQUIRES_MANUAL_HANDLING`；resumable 集合移除 `MANUAL_HOLD/FAILED_PERMANENT`；M1 `tccGroupComposition.UnknownComposition` + 组上限公式 + 新 commitment `known=0`/无行；M2 无活动组幂等成功；R-4 resume 审计（`tccSumMismatch` 包裹）。 |
| `internal/services/order/settlement_integration_test.go` | `orderTestDB` 改为执行真实 `migrations/order/0017`+`0018`（去注释后按 `;` 拆分），移除对应内联 DDL/列。 |
| `internal/services/order/tcc_compensation_integration_test.go` | 新增 R-1/M1/M2/R-4/迁移形状测试；旧的 0/0 legacy fail-closed 测试替换为“migrated known=0 整组成功 + known=1 坏行 fail-closed”。 |

### 2.2 [架构侵入性]

- 新增 1 个生命周期 `switch`（入口 + Tx1 各一处，封闭集合，未知 fail-closed），未新增“默认放行”分支。
- 未改状态所有权：Order 仍唯一写 `orders.refund_state`/permit/commitment/application；Fulfillment shipment 只读；Payment 只读快照。
- 未新增跨服务调用/事件/表/列/枚举；未改 proto/migrations/consumer/Spec/ADR。
- `refund_batches` 仍无业务唯一键（按 ADR 由订单锁保证），未用索引替代锁语义。

### 2.3 [副作用评估]

- **行为扩大**：以前 `ALLOCATED/SHIPPED/COMPLETED` 的 TCC cancel 静默成功且不退款；现在会写 batch/application/permit 并把订单置 `refund_state=REFUNDING`（生命周期不变）。这是 B1 的预期修复，但 WIP 下仍无 worker 打款。
- **行为收紧**：`CANCELLED+paid>0`、未知状态、`MANUAL_HOLD/FAILED_PERMANENT` 的自动重试从“成功/续跑”变为可识别错误；DTM 会重试直到 T1.6/T1.4 落地，属 fail-closed 预期。
- **历史数据**：`composition_known=0`（含 migration 回填 goods=amount、无行）不再阻断整单 TCC；有 `composition_known=1` 坏行的订单仍被阻断。
- **无活动组**：全组已退/Σ=0=remaining 时不再产生空 `FROZEN` batch，也不再写 `REFUNDING`。
- **测试基线**：order 测试库现在带 0017/0018 的 CHECK/索引；任何写入非法枚举/金额的旧测试会立即暴露（本轮已全绿）。
- **回归面**：`TccConfirmMarkPaid`/`TccTryMarkPaid`/`TccQueryMarkPaid` 未改（`TccQuery` 对 `ALLOCATED/SHIPPED/COMPLETED` 仍返回 `Prepared=false`，属运维只读校验，不在 R-1 范围，见 §4）。

---

## 3. 测试证据（绑定 code head `e7e29e5`）

环境：`go1.27.1 darwin/amd64`；MySQL `mysql-unicorn`（`127.0.0.1:3306`）；`TEST_MYSQL_DSN=root:***@tcp(127.0.0.1:3306)/mall_integration_test?parseTime=true`；`GOCACHE=/tmp/dsh-go-cache`。

| 命令 | 结果 | 原始日志 |
|---|---|---|
| `gofmt -l <changed files>` | 空 | `/tmp/t12r-logs/build-vet.log` |
| `go build ./...` | exit 0，`BUILD_OK` | 同上 |
| `go vet ./internal/services/order` | exit 0，`VET_OK` | 同上 |
| `go test ./internal/services/order -count=1` | **ok 40.884s**（完整 order 包） | `/tmp/t12r-logs/order-full.log` |
| `go test ./internal/services/order -run 'TestTcc\|TestOrderTestSchema' -count=1 -v` | **PASS 31.218s**，47 个 TCC/预检/生命周期/迁移用例全绿（46 TCC/Σ + 1 schema） | `/tmp/t12r-logs/tcc-verbose.log` |
| `go test -race ./internal/services/order -run 'TestTcc' -count=1` | **ok 24.074s** | `/tmp/t12r-logs/tcc-race.log` |
| `go test -race ... 'TestTccCancelConcurrent…\|…PrecheckConverges…\|…ManualHold…' -count=2` | **ok 5.582s** | `/tmp/t12r-logs/tcc-race-repeat.log` |

新增/关键用例（均真实 MySQL）：`TestTccCancelAllocatedOrderMixedGroups`（ALLOCATED + G1 SHIPPED + G2 未离仓）、`TestTccCancelShippedLifecycle`、`TestTccCancelCompletedLifecycle`、`TestTccCancelRefundingAllocatedOrderResumesByRefundState`、`TestTccCancelCancelledCapturedFailsClosed`、`TestTccCancelCancelledWithoutCapturedIsIdempotent`、`TestTccCancelManualHoldBatchIsNotResumed`（MANUAL_HOLD/FAILED_PERMANENT）、`TestTccCancelUnsupportedStatusFailsClosed`、`TestTccCancelMigratedLegacyCompositionAllowsWholeGroupRefund`、`TestTccCancelKnownCompositionLineMismatchFailsClosed`、`TestTccCancelNoActiveGroupsIsIdempotentSuccess`、`TestTccCancelNoActiveButRemainingCapturedWritesAudit`、`TestTccCancelResumeBlocksWhenPaymentSnapshotChanges`（含 R-4 审计断言）、`TestOrderTestSchemaUsesC05RefundMigrations`。

【测试范围】
- 生命周期：PAID/ALLOCATED（含单组已 SHIPPED 的真实混合）/SHIPPED/COMPLETED 的补偿入口；`REFUNDING` 按 refund_state 续跑（含 ALLOCATED）；`FULL` 幂等；`CANCELLED+paid>0` fail-closed；`CANCELLED+paid<=0` 幂等；未知 `CLOSED` fail-closed。
- Σ 与审计：外部预检 + Tx1 重检审计；resume Σ/快照变化审计；`active=0 && remaining>0` 审计；重复失败 attempt 递增且不阻塞新 attempt；`PRECHECK_FAILED` 不 resume。
- 历史组成：真实 `order/0018` 形状（goods=amount/freight=0/known=0/无行）整组成功；known=1 行合计错误 fail-closed；旧承诺不被改写。
- permit/application：`NONE→REVOKED`、`CONSUMED` 不写 REVOKED（PENDING_REMOTE + commitment）、已离仓 POST（组状态与 `EXCEPTION+shipped_at` 两条路径）、G1；并发/重复幂等；resume 补缺。
- 迁移形状：真实 `order/0017/0018` 的建表/列/CHECK 由 `TestOrderTestSchemaUsesC05RefundMigrations` 断言。
- 超时/不可用：Payment 超时、Fulfillment `ListShipments` 超时、exception 缺 `shipped_at` 均零写入 fail-closed。

【未覆盖范围】
- T1.4：`AdmitRefund` 逐组准入、`CancelIfCreated`（含 `EXCEPTION+shipped_at → ALREADY_SHIPPED`）、逐组 `Payment.Refund`、`payment.refunded → SETTLED`、`MANUAL_HOLD/PAYING` CAS/worker；本实现只到 `REFUNDING + batch/apps`（CONSUMED 组到 `PENDING_REMOTE + RESERVED commitment`）。
- T1.6：`PENDING_PAYMENT`/`CANCELLED` + captured 的 CAS→CANCELLED + 支付级 `pay:<payment_no>:cancel`；本实现只 fail-closed。
- T1.5：`order.refund_requested` 消费者观察化（TCC 已不再发该事件，消费者未改）。
- T1.7/T5.2/T5.3：真实 Ship vs TCC 并发 `-race`、故障注入、完整 AC 矩阵；T5.4 真实迁移回填/AC-60。
- 真实 Fulfillment `CancelIfCreated`/Payment `Refund` 端到端；BFF/运营展示；`make gates`/全量 `make test-race`。
- `TccQueryMarkPaid` 对 `ALLOCATED/SHIPPED/COMPLETED` 仍返回 `Prepared=false`（只读运维校验，不在 R-1 范围）。
- 审计 metrics 未接入（仅日志）；`MAX(attempt_seq)+1` 高并发丢失审计的残余风险（m5）未消除。

---

## 4. 残余风险 / 待裁决

| # | 风险 | 处置 |
|---|---|---|
| m4 | CANCELLED/ PENDING 的 `paid_cent` 信任边界（请求谎报 0） | Addendum R-3 明确 guard 不调用 Payment；T1.6 用 Payment 事实收敛。 |
| m5 | `PRECHECK_FAILED` 的 attempt 分配高并发可能丢失一条审计 | 资金仍 fail-closed；建议 T1.4/T5 增加序列或告警（未在本任务自创）。 |
| m3 | `exception_state` 空值判定：当前 `""`/`NONE` 都视为非 exception；空值+`shipped_at` 会被当作未离仓 | 本轮未改（Review m3 MINOR）；建议 T1.7/契约补充空值语义。 |
| — | `composition_known=0` 的 goods/freight 拆分为 ADR 回填约定，不是行归因 | 只允许整组；行级部分退款需人工/T3.4。 |
| — | T1.4 前 batch 无 worker | WIP 不合并；不得把 T1.2 `return nil` 当退款完成。 |

## 5. 声明

- 本文件不批准任何实现，不宣称 PASS，不把 AC-REF 写成 AC-VERIFIED；`ac-coverage.tsv` 的 AC-51/AC-65 仍按 Reviewer 台账处理。
- 测试日志在代码 HEAD `e7e29e5` 的树上执行；本文件随后仅新增 docs，不改 `internal/`。
- 冻结状态与重审由 Reviewer/人类按 `t1.2-r-plan.md` 执行。
