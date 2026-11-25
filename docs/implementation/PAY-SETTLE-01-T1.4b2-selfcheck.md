# PAY-SETTLE-01 T1.4b2a Developer 自检清单

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Role / Actor | Developer / `dev-phase1g` |
| Tier | **L2** |
| Tier-Rationale | 跨服务事件/消费者（Order→Inventory）、订单退款准入、库存预留释放；命中 L2 触发条，不适用 L0/L1 |
| Plan / Design | `docs/lead/PAY-SETTLE-01/t1.4b2-design.md` **Rev 1.3 accepted**（P0 单组 + 必填 `group_count`）；QC-27 READY |
| Spec-Ref | SPEC-0002 **v1.9**：INV-44、AC-50、AC-67、AC-68；SPEC-0001 INV-03 |
| Base | `60130e1`（P0 单组 release 代码；本清单在其上补 `group_count` 契约） |
| Code head（本清单绑定） | **本 WIP 提交**（`git log -1` 即该提交；同提交包含本文件） |
| allowed_paths | `internal/platform/events/**`；`internal/services/order/refund_release.go`、`internal/services/order/refund_admission.go`、`internal/services/order/*_test.go`；`internal/services/inventory/refund_release_consumer*.go`、`internal/services/inventory/*_test.go`；`cmd/order/main.go`、`cmd/inventory/main.go`；`internal/services/aftersales/workers.go`、`internal/services/aftersales/workers_source_test.go`；`docs/implementation/PAY-SETTLE-01-T1.4b2-selfcheck.md` |
| baseline_impact | INV-44（组退款后库存处置有主）、INV-06/INV-17（预留/库存守恒）、AC-50/AC-67/AC-68；非回归：order/inventory/aftersales 全包测试。无 proto、无 migrations、无 Inventory schema 变更 |
| Scope | T1.4b2a：单组判定、`order_reservation_full` outbox、Inventory 整笔 release consumer、B2 `system_shortage` |
| 不做 | 多组库存释放（P0 保持 reserved，P1/人工）；生产回填/scan/receipt；T1.4b2b Zero AfterSales task；Payment/Order 其他路径 |
| Status | **WIP**；不批准、不声称实现 PASS / Release PASS、不标 AC-VERIFIED |

> 本文件是 Developer 的 canonical 自检交付物，只记录本次实现事实、证据与残余风险，不替代 Reviewer 裁决。

---

## 1. 反妥协自检清单（三问）

### 1.1 并发冲突 / 网络超时 / 部分成功时会发生什么？处理了吗？

- **准入与 outbox 原子性**：单组 PRE 准入在同一 `*sql.Tx` 内写 admission/commitment/permit/`order.refund_admitted` 与 `inventory.refund_release_requested`；append 失败整事务回滚，不留 S19/事件。`TestAdmitRefundReleaseOutboxFailureRollsBackAdmission` 用 MySQL trigger 注入 append 失败，断言 admission/refund_admitted/permit/outbox 均无行。
- **并发准入**：`AdmitRefund` 以订单行锁 + admission 锁内重读串行化；同一 `refund_id` 重放返回既有结果，不重复 event。单组订单无第二组可并发触发；多组并发退款只写 admission、不写 outbox（`TestMultiGroupConcurrentRefundsDoNotWriteRelease`，`-race`）。
- **Inventory 超时/部分失败**：逐 item 处理；HELD `Release`、CONFIRMED `TccCancelConfirm`（使用 reservation 权威 quantity）；缺失/RELEASED 幂等 no-op；首个失败返回错误，consumer NAK 重试，不 ACK 半成功；Invalid/NotFound/FailedPrecondition→`Permanent`→DLQ。`TestRefundReleasePartialFailureDoesNotAck`、`TestRefundReleaseConsumerCanceledContextDoesNotRelease`、`TestRefundReleaseHandlerRetriesWithoutDoubleRelease` 覆盖。
- **重复/多 worker**：reservation 状态 CAS + `Release`/`TccCancelConfirm` 幂等；`TestRefundReleaseConsumerConcurrentWorkers` 8 worker 只释放一次，ledger RELEASE=1；重复事件 no-op。
- **契约防御**：`group_count` 缺失/0/!=1 或 `covered_group_nos` 与单组身份不一致时 consumer `Permanent`，不触碰 reservation；producer 多组不写 event。

### 1.2 是否把状态揉进一个字段 / 加 if-else 绕过架构？

- 未新增状态机字段：`group_count` 是事件契约的安全判据（Rev 1.3），不是 Order/Inventory 状态；单组判定只读既有 `order_fulfillment_groups` + `order_items`。
- 未引入 scan/last-active/watermark/manual queue；多组路径只 Warn，不伪造 release。
- 状态所有权不变：Order 只写 admission/outbox；Inventory 只改 reservation/stock；Fulfillment/AfterSales 路径未改。
- 单组/多组分支是契约的显式二选一，不是场景补丁；consumer 对 `group_count`/covered 的防御为独立契约校验。

### 1.3 向现实做了哪些妥协（重大妥协高亮）

1. **【重大/P0 已知】多组订单不释放库存**：`group_count>1` 的退款准入成功但 reservation 保持 reserved，仅日志/告警，P1/人工；不是遗漏，是 Rev 1.3 的 P0 取舍。
2. **【重大】无生产回填/scan**：不处理历史 PRE admission 无 release 的数据；当前无生产数据；dev/UAT 仅环境清理，不进入代码/迁移。
3. **无 broker e2e**：outbox relay→NATS→consumer 的真实重投/DLQ 由 T5.2/T5.3 补；本次以 producer outbox 行 + handler/DB 集成测试覆盖。
4. **`covered_group_nos` 仅审计**：producer 不写；consumer 若收到则要求长度=1 且等于 `group_no`，否则 DLQ；不作为释放判据。

---

## 2. [修改点] / [架构侵入性] / [副作用评估]

### 2.1 [修改点]

| 文件 | 修改 |
|---|---|
| `internal/platform/events/events.go` | `RefundReleaseRequested` 增加必填 `GroupCount int32`（`group_count`）；保留可选 `CoveredGroupNos` 审计 |
| `internal/platform/events/registry.go` | required 增加 `group_count`；校验 `group_count>0`；`covered_group_nos` 若存在必须长度=`group_count` 且包含 `group_no` |
| `internal/services/order/refund_release.go` | P0：`group_count==1` 且 PRE 才 append；items 整单聚合；`GroupCount=1`；多组 Warn 不写；无 scan/worker |
| `internal/services/order/refund_admission.go` | 三处 PRE 路径调用单组 append（正金额/零元/Tx2 remote admission）；POST 不调用 |
| `internal/services/inventory/refund_release_consumer.go` | 先校验 `group_count==1`；covered 审计一致性；整笔 Release/TccCancelConfirm 忽略 quantity；DLQ 分类 |
| `cmd/order/main.go` | 删除已移除的 reconcile worker 注册（Rev 1.2 清理） |
| `internal/services/aftersales/workers.go` | B2：SYSTEM 缺货准入 source=`system_shortage`；普通售后=`aftersale` |
| tests | order/inventory/events/aftersales 覆盖单组、多组、零元、POST、source、重复/并发、多 SKU/仓、缺字段、group_count 契约 |

### 2.2 [架构侵入性]

- 新增一个事件契约字段 `group_count`，无新 RPC/迁移/表。
- 移除 scan 路径，代码量净减少；没有以 if-else 绕过多组释放，而是显式 P0 行为。
- Inventory 仍只执行既有 `Release`/`TccCancelConfirm`，未新增库存操作。

### 2.3 [副作用评估]

- 多组退款不再有“延迟释放”路径，库存保持 reserved；这是 P0 业务取舍，Reviewer/人类需按 Rev 1.3 接受。
- 单组退款的 release event 现在强制 `group_count=1`；旧格式（缺失）事件会被 consumer DLQ。
- `order_items` 缺 group/warehouse 会让准入 fail-closed（含多组路径），防止猜测 G1/W1。
- 无 Payment/Order 其他路径改动；B2 仅 SYSTEM 缺货 source 改变。

---

## 3. 测试证据（真实 MySQL；绑定本 WIP 提交，Base=`60130e1`）

环境：`go1.27.1 darwin/amd64`；MySQL `mysql-unicorn`（`127.0.0.1:3306`）；`TEST_MYSQL_DSN` 由 `scripts/test-dsn.sh` 按包派生；`TEST_NATS_URL=nats://127.0.0.1:4222`。

| 命令 | 结果 |
|---|---|
| `go build ./...` | OK |
| `go test ./internal/platform/events -count=1` | `ok ... 0.108s` |
| `TEST_MYSQL_DSN=<order> go test ./internal/services/order -count=1` | `ok ... 116.021s` |
| `TEST_MYSQL_DSN=<inventory> go test ./internal/services/inventory -count=1` | `ok ... 12.278s` |
| `TEST_MYSQL_DSN=<aftersales> go test ./internal/services/aftersales -count=1` | `ok ... 27.305s`（B2/基线回归） |
| `TEST_MYSQL_DSN=<order> go test -race ./internal/services/order -run 'TestMultiGroupConcurrentRefundsDoNotWriteRelease\|TestAdmitRefundSingleGroupEmitsImmediately\|TestAdmitRefundUnsupportedReleaseSourceRejects' -count=1` | `ok ... 3.395s` |
| `TEST_MYSQL_DSN=<inventory> go test -race ./internal/services/inventory -run 'TestRefundReleaseConsumerConcurrentWorkers\|TestRefundReleasePartialFailureDoesNotAck\|TestRefundReleaseConsumerRestocksConfirmedOnce' -count=1` | `ok ... 1.824s` |
| `gofmt -l <changed files>` | 仅既有 `internal/services/order/service.go` 漂移；本次改动文件为空 |

### 3.1 覆盖矩阵

| 场景 | 测试 |
|---|---|
| 单组 PRE/HELD → full release | `TestAdmitRefundSingleGroupEmitsImmediately`；`TestRefundReleaseConsumerReleasesHeldOnce`（event quantity 故意错误仍忽略） |
| 单组 CONFIRMED → TccCancelConfirm | `TestRefundReleaseConsumerRestocksConfirmedOnce`（event quantity 错误，restock 一次） |
| 单组 POST 不写 | `TestAdmitRefundPostDoesNotWriteReleaseOutbox`；零元 `TestAdmitRefundSystemZeroPostWritesNoReleaseOutbox` |
| 多组退款不写 | `TestMultiGroupRefundDoesNotWriteRelease` |
| 多组并发不写 | `TestMultiGroupConcurrentRefundsDoNotWriteRelease`（`-race`） |
| 多组零元不写 | `TestMultiGroupZeroRefundDoesNotWriteRelease`（S19 DONE、无 Payment、0 outbox） |
| 单组零元 full release | `TestAdmitRefundSystemZeroWritesOrderLevelRelease` |
| append 失败回滚 | `TestAdmitRefundReleaseOutboxFailureRollsBackAdmission` |
| group_count=1 正常 | producer 单测断言；consumer `TestRefundReleaseHandlerAcceptsSingleGroupCoveredAudit` |
| group_count 缺失/0/2 | registry 单测 + `TestRefundReleaseHandlerRejectsInvalidGroupCount` |
| group_count=1 + covered 其他组 | registry 单测 + consumer 同上 |
| 重复/多 worker | order 重复重放；inventory 8 worker、重复事件 no-op |
| Inventory 超时/部分失败/DLQ 分类 | `TestRefundReleasePartialFailureDoesNotAck`、canceled ctx、handler retry/permanent |
| 多 SKU/多仓 | 单组 items 全量；inventory wh1/wh2 |
| source 拒绝/system_shortage | source 单测 + order 集成 + B2 helper 单测 |
| 缺 warehouse/group/no items | fail-closed，零写入 |

---

## 4. 【测试范围】与【未覆盖范围】

### 4.1 测试范围

- Order：单组正/零元/远程准入、POST、多组顺序/并发、append 回滚、source 拒绝、缺字段 fail-closed、event 内容（scope/group_count/items/audit）。
- Inventory：整笔 HELD/CONFIRMED、忽略 quantity、重复/多 worker、部分失败、DLQ 分类、多 SKU/仓、group_count/covered 防御。
- Events：registry required/类型/枚举/group_count/covered 校验。
- Aftersales：B2 `system_shortage` helper + 全包回归。

### 4.2 未覆盖范围

1. 多组库存释放（P0 明确不释放；P1/人工）。
2. 生产历史回填/receipt/scan（Rev 1.3 明确 deferred）。
3. NATS broker 端到端与真实 DLQ 投递断言（T5.2/T5.3）。
4. TCC/zero 的 deferred source 不再存在（无 scan）；仅即时路径覆盖 source。
5. POST 路径不校验 source（不产生 release）；payment/legacy 的 POST 语义未新增裁决。
6. `covered_group_nos` 仅审计；producer 不写，consumer 兼容校验。

---

## 5. 残余风险与需 Reviewer 确认点

1. **多组库存长期 reserved**：P0 已知；若业务要求最终释放，需 P1/人工任务，不在本 WIP。
2. **事件 group_count 契约**：旧格式事件（无 group_count）会被 DLQ；上线前需确认无其他 producer 使用该 subject。
3. **无 broker e2e**：outbox relay 与 consumer 订阅已接线，但真实 NATS 重投/DLQ 未在本次运行。
4. **`orderGroupCount` 口径**：取 `order_fulfillment_groups ∪ order_items.group_no` 去重（兼容历史 G1 无 group 行）；若两投影不一致会倾向多组（不释放），属 fail-safe。
5. **无 AC-VERIFIED**：本清单不改变 `docs/collab/ac-coverage.tsv`（Owner=Reviewer）。

---

## 6. 绑定与声明

- **Code head**：本 WIP 提交（`git log -1`）；Base=`60130e1`；测试日志与文件同提交。
- **Tier**：L2；`change_id`/`Actor`/`Status`/`Tier` 写入 commit trailer；无 `Reviewed-by`。
- **不批准**：本清单不构成实现 PASS、Reviewer PASS、Release PASS 或 AC-VERIFIED；不批准合入。
