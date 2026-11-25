# REV-PAY-SETTLE-01-T1.4b2-P0 — 单组物理释放复审

| 项 | 值 |
|---|---|
| review_scope | `8acffab..60130e1` 的 T1.4b2a P0：单组 PRE full release、多组零 outbox/库存 reserved、移除 scan、Inventory full Release/TccCancelConfirm + 多组 event 防御、B2 source、B3 deferred。只读；不评审 T1.5/T1.6/T1.7/T5。不构成 Release PASS。 |
| base | `8acffab`（上一轮 T1.4b2-R head） |
| code head | `60130e1`（`wip(order): T1.4b2a P0 single-group release only`） |
| design | `docs/lead/PAY-SETTLE-01/t1.4b2-design.md` Rev 1.2（工作区 **untracked**，见 B2） |
| Tier / Actor | **L2**（commit `Tier: L2`，`Tier-Rationale` 命中跨服务 inventory/refund）；Actor `dev-phase1g` ≠ Reviewed-by `reviewer-phase1` |
| **verdict** | **FAIL（有范围）** |
| release_status | **NOT RELEASE**；不批准合并 |

> P0 的 single/multi、Inventory full release、source、移除 scan 均通过；剩余 BLOCKER 是 **多组 event 防御不可执行** 与 **L2 canonical/selfcheck 缺失**。

---

## 1. BLOCKER（2 条）

### B1 [ARCHITECTURAL_FLAW] — 多组 event 防御没有可执行的 `group_count`，依赖可选 `covered_group_nos`

- **位置**：`internal/services/inventory/refund_release_consumer.go:63-69` 只检查 `len(event.CoveredGroupNos) > 1` 来拒绝“多组 event”；`internal/platform/events/events.go:82-94` 的 `RefundReleaseRequested` 有 `CoveredGroupNos []string omitempty`，**没有 `group_count`**；Order P0 producer `internal/services/order/refund_release.go:169-196` 的单组 event **不设置 `CoveredGroupNos`**；registry `internal/platform/events/registry.go:226-250` 只校验 `release_scope`/source/reason/items，`covered_group_nos` 是可选且没有“必须恰好 1”。
- **事实**：design §2.4 要求“若收到 `group_count>1` 的 event，按契约错误 DLQ”；但 event 没有 `group_count` 字段。Inventory 只能看 optional covered。一个多组 event 若省略/只带 1 个 covered（foreign/旧 producer 的错误/schema 未要求），`len>1` 为 false，consumer 会继续按 full-reservation **释放整单库存**；这正是 P0 说不能发生的场景。现有 `TestRefundReleaseHandlerRejectsMultiGroupEvent` 是**手工**把 covered 改成 `[G1,G2]`，并没有覆盖“多组 event 无 covered/只带一个”的真实 producer 形态。
- **判定**：多组防御是契约/可验证性缺陷，不是加个 if 能解决。需要在 C-04 event 增加 `group_count`（或要求 `covered_group_nos` 必须恰好为 1 且 P0 producer 填充 `[groupNo]`），registry + Inventory 都 fail-closed；并补“无/单 covered 的多组事件 → DLQ、零 Release”的真实用例。该项应打回 **Architect**；Dev 不得在当前分支硬改。

### B2 [BLOCKER] — L2 canonical design/selfcheck 缺失，Tier 输入不完整

- **位置/事实**：`docs/lead/PAY-SETTLE-01/t1.4b2-design.md` 在 `22f7fea` 中不存在，当前为 **untracked**；仓库没有 `PAY-SETTLE-01-T1.4b2-*-selfcheck.md`（只有 `PAY-SETTLE-01-T1.4b1-selfcheck.md`）；commit `60130e1` 只有 `Tier: L2`/`Tier-Rationale`，没有 `Spec-Ref`、`allowed_paths`、`baseline_impact`。
- **契约**：AGENTS §2.7 要求 L2 变更输入包含 `Spec-Ref`、`allowed_paths`、base/head、`baseline_impact`/INV、测试证据、Reviewer verdict；§1.4 要求 Developer 提交反妥协自检清单。当前 Reviewer 无法从 canonical docs commit 审计“Rev 1.2 accepted”，也无法绑定 Developer 自检；这会让 L2 的批准链不可复核。design 文件在 Reviewer 工作区可见不等于已提交的 accepted artifact。
- **判定**：补 design 的 canonical commit（Rev 1.2 accepted）与 T1.4b2 P0 selfcheck，并在 commit/交付输入中写 Spec-Ref/allowed_paths/baseline_impact；否则不能作为 L2 merge 输入。此条不否定 P0 代码本身，但阻止交付关闭。

---

## 2. 已通过 / 关闭

### 生产者（P0）

- **单组 PRE → `order_reservation_full`**：`orderGroupCount`（group 行 ∪ item group）==1；`loadOrderReleaseItems` 聚合整单 items；`appendRefundReleaseTx` 在 admission 事务内写 outbox；`TestAdmitRefundSingleGroupEmitsImmediately`、`TestAdmitRefundRemoteAdmissionPreWritesOrderLevelRelease` PASS。
- **单组 POST 不写**：`appendRefundReleaseTx` 只在 `result==PRE` 调用；`TestAdmitRefundPostDoesNotWriteReleaseOutbox`、`TestAdmitRefundSystemZeroPostWritesNoReleaseOutbox` PASS。
- **多组（>1）不写 outbox、库存 reserved**：`orderGroupCount != 1` 直接 `return nil`（日志），admission 仍成功；`TestMultiGroupRefundDoesNotWriteRelease`、`TestMultiGroupConcurrentRefundsDoNotWriteRelease`、`TestMultiGroupZeroRefundDoesNotWriteRelease` PASS。
- **移除 scan/last-active**：`cmd/order` 不再注册 `RunRefundReleaseReconcileWorker`；`refund_release.go` 无 scan/last-active/plan；测试无 scan；`grep` 仅注释。P0 无生产 backfill；B3 按 design deferred。
- **source**：`refundAdmissionSource` 的 SYSTEM → `system_shortage`；`refundReleaseSource` 仅接受 tcc/system_zero/system_shortage/aftersale + 空=aftersale，payment/legacy/unknown 返回 `REFUND_RELEASE_SOURCE_UNSUPPORTED`；`TestRefundReleaseSourceMapping`、`TestAdmitRefundSystemShortageSource`、`UnsupportedReleaseSourceRejects` PASS。
- **缺组/仓 fail-closed**：`loadOrderReleaseItems` 对空 group/warehouse/无 items 返回错误，admission 回滚；`TestAdmitRefundMissingWarehouseRejectsRelease`、`MissingGroupRejectsRelease`、`NoOrderItemsFailsClosed` PASS。

### Inventory

- **单组 HELD full Release**：consumer 调用 `Release` 时**不传 quantity**（`refund_release_consumer.go:78-81`）；`TestRefundReleaseConsumerReleasesHeldOnce` PASS。
- **单组 CONFIRMED full TccCancelConfirm**：使用 reservation 的 authoritative quantity（不是 event quantity）；`TestRefundReleaseConsumerRestocksConfirmedOnce` PASS。
- **重复/多 worker no-op**：reservation 状态 CAS；`TestRefundReleaseConsumerConcurrentWorkers` PASS。
- **多 SKU/多仓**：`TestRefundReleaseConsumerMultiWarehouse` PASS；partial failure/超时/DLQ/canceled context PASS。
- **多组 event 防御**：代码有 `len(CoveredGroupNos)>1` 拒绝，测试 `TestRefundReleaseHandlerRejectsMultiGroupEvent` PASS；但见 B1 的不可执行性。

### zero / 边界

- T1.4b2b/b2 zero 回归：`TestZero...`、`NonZero...`、`CapturedZero...` 通过；task 先持久、zero short-circuit、S19 DONE、REFUNDED/0/CONSUMED、无 Payment、return 先 restock。
- `git diff --name-only 8acffab..60130e1` 无 proto/migrations/Inventory schema；无 T1.5/T1.6/T1.7/T5；Order 只写 outbox，Inventory 只改 reservation/stock。
- **Tier**：commit `60130e1` 明确 `Tier: L2`/`Tier-Rationale`；Reviewer 确认 L2 命中跨服务 inventory/refund 与事件消费。

---

## 3. 测试证据（Reviewer 独立复跑）

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0**，38.802s |
| `go test ./internal/platform/events -run 'TestRefundRelease'` | **PASS**，0.187s |
| `go test ./internal/services/order -run 'TestMultiGroup|TestAdmitRefund|TestRefundReleaseSource'` | **PASS**，12.481s |
| `go test ./internal/services/inventory -run 'TestRefundRelease'` | **PASS**，2.676s |
| `go test ./internal/services/aftersales -run 'TestZero|TestNonZero|TestCapturedZero'` | **PASS**，8.728s |
| `-race order`（MultiGroup/SingleGroup） | **ok**，4.490s |
| `-race inventory`（ConcurrentWorkers/ReleasesHeldOnce） | **ok**，1.894s |
| `go vet` / `gofmt -l` | exit 0 / 无输出 |

未发现 SKIP 被当 PASS；AC-50/AC-67/AC-68 未标 AC-VERIFIED。

---

## 4. 非阻断 / 已知缺口

1. **多组库存不释放**是 P0 已知行为（P1/人工），不是本轮 blocker；但 B1 的“多组 event 防御”必须是契约级，不能靠日志/covered 可选。
2. **多组仅日志**：`appendRefundReleaseTx` 对 group_count>1 只 `s.log.Warn`；P0 接受，但无可查询 manual 状态；P1 另立。
3. **无 broker e2e**：T5.2/T5.3 需要真实 NATS 重投/DLQ/并发；Release 前置。
4. **Design/selfcheck**：Rev 1.2 文件 untracked、无 T1.4b2 P0 selfcheck；见 B2。
5. **`orderGroupCount`** 把取消/空 group 计入 group_count（保守地不释放）；不对 P0 安全构成风险，但需 P1 文档说明。

---

## 5. 结论

- **verdict FAIL（有范围）**：P0 单组物理释放、多组 0 outbox/reserved、无 scan、source、Inventory full release 均通过；**B1 多组 event 防御不可执行（Architectural）**、**B2 L2 design/selfcheck canonical 缺失**仍阻止 T1.4b2 P0 关闭。
- **不批准合并；不是 Release PASS；不替代人类 merge 决策。**
- B1 应打回 Architect 定 event `group_count`/covered 必填语义并补 DLQ 用例；B2 由 Lead/Developer 提交 canonical design + selfcheck/change-input；之后按精确 head 重审。
