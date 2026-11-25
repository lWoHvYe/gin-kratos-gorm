# REV-PAY-SETTLE-01-T1.4b2-R — order-level deferred release 复审

| 项 | 值 |
|---|---|
| review_scope | `64a2e9d..8acffab` 的 T1.4b2a-R：order-level deferred INV-44 release（last-active、full items、scan、Inventory full Release/TccCancelConfirm）+ B2 source + B3 backfill/cleanup 设计；T1.4b2b/b2 zero 回归。只读；不评审 T1.5/T1.6/T1.7/T5。不构成 Release PASS。 |
| base | `64a2e9d` |
| code head | `8acffab`（`wip(order): T1.4b2a order-level deferred INV-44 release`） |
| docs | `22f7fea`（governance docs）；design `t1.4b2-design.md` Rev 1.1（**工作区 untracked，不在 22f7fea**，见 §6） |
| 契约 | SPEC-0002 v1.9（INV-44/AC-50/AC-67/AC-68）；ADR-0007 Amd1；design Rev 1.1 accepted；C-04 event 增量；C-05 无迁移 |
| Tier / Actor | **L2**（commit `Tier: L2`，`Tier-Rationale` 命中跨服务 inventory/refund）；Actor `dev-phase1g` ≠ Reviewed-by `reviewer-phase1` |
| **verdict** | **FAIL（有范围）** |
| release_status | **NOT RELEASE**；不批准合并 |

> **工作区**：`8acffab` 之上无受管代码改动；design/ADR/QC 为 untracked docs。B1/B2/B3 均按代码事实；B2 source 已修复，旧 group/reservation 冲突已关闭。

---

## 1. BLOCKER（3 条）

### B1 [BLOCKER] TCC group 的 `refund_state` 永远停在 REFUNDING，多组 TCC 永不满足“last active”

- **位置**：`internal/services/order/tcc_settlement.go:697-700` 只把 TCC 组写成 `refund_state=REFUNDING`；Order 代码中写 group `REFUNDED` 的只有 `internal/services/order/aftersale_consumer.go:44-50`（AfterSales completed）。`internal/services/order/refund_release.go:199-205` 的 `refundReleaseGroupIsActive` 只把 `REFUNDED/FULL` 排除，**REFUNDING 保持 active**（注释明确“REFUNDING stays active”）。
- **事实**：两个 TCC 组 G1/G2 均已完成 TCC 退款后，group 仍是 `REFUNDING`；`planRefundRelease` 对每个组都把对方视为 active，均 `emit=false`；scan 反复扫描仍无 last active，**order-level full release 永不发生**，INV-44 库存不释放。TCC 是 T1.4b2a 的主路径；现有 release 测试全部用 after-sale + `markGroupRefunded`（手工写 REFUNDED），**没有 TCC 多组用例**。
- **Reviewer 实际证明**：overlay 两个 TCC 组均 `REFUNDING`、两条 PRE admission，运行 `runRefundReleaseReconcileOnce` 后 release events **0**（`TestReviewerOverlayTccRefundingGroupsNeverEmit` 通过是因为它断言了 0）。
- **判定**：last-active 状态机没有 TCC 完成态，B1 的 deferred 方案对 TCC 多组不成立。需要 TCC 完成时更新 group 状态，或 last-active 判断改读 TCC batch/application 的 DONE；并补“两组 TCC 均完成 → 只发一次 full release”的真实测试。不能只把 `REFUNDING` 改成 inactive（会把未完成退款也当 last）。

### B2 [BLOCKER] deferred scan 是无游标的全量 production backfill，且 `LIMIT 100` 会饿死后续订单

- **位置**：`internal/services/order/refund_release.go:374-383` 的 `scanPendingRefundReleases` 扫 **所有** `order_refund_admissions.result=PRE` 且无 release 的订单，`ORDER BY a.updated_at, a.refund_id LIMIT 100`，**无 cursor/游标/watermark**；`cmd/order/main.go:41` 以 `time.Minute` 无条件启动。
- **事实 A（生产 backfill 未 deferred）**：该 scan 不只覆盖新 admission，也覆盖 T1.4a 的历史 PRE（含 upgrade 前数据）；会直接补写 full release 并由 Inventory 物理释放。design §4.2 写明“生产回填 deferred（当前无生产数据）”，但代码没有 env/watermark/dry-run 门，**生产扫描即 backfill**，与 human decision 不一致；也没有独立的 dev/UAT 一次性 cleanup。
- **事实 B（starvation）**：pending 的 deferred（非 last）和 manual（共享 POST）订单不会写 event、也不会从查询消失；它们占据 `ORDER BY updated_at, refund_id` 的前 100，后续 ready 订单永远进不了扫描页。`LIMIT 100` 没有分页/游标/scan marker，属于**持久饿死**，不是瞬时延迟。
- **Reviewer 实际证明**：overlay 先放 100 个 shared-POST manual 订单（refund_id `R-000..R-099`），再放一个两组均 REFUNDED、应释放的 `ORD-TARGET`（`R-ZZZ-TARGET`）；执行一次 scan 后 target release events **0**（`TestReviewerOverlayScanStarvesAfterOneHundredPending` 通过是因为它断言了 0）。
- **判定**：deferred scan 同时违反“生产 backfill deferred”和可收敛性；需要 cursor/keyset 分页或 durable scan marker（design 说不新增 receipt，但可用 watermark/游标），并把 production backfill 与 new-admission deferred release 分开；dev/UAT cleanup 需 dry-run/幂等入口。**不是加一个 `LIMIT` 的局部补丁**。

### B3 [BLOCKER] 共享 POST/manual 仅写日志；未知当前 group 还会 fail-open 发 full release

- **位置**：`internal/services/order/refund_release.go:288-305`（`appendRefundReleaseTx`）在 `plan.manual` 时只 `s.log.Warn(...)` 后 `return nil`；`internal/services/order/refund_release.go:150-170` 的 `loadRefundReleaseGroups` 把只出现在 `order_items`、没有 group 行的组补为 `status=UNKNOWN`；`planRefundRelease`（`:213-285`）对 `current` 为 UNKNOWN 时 **不判 POST/不判 manual**，若无其他 active group 则 `emit=true`。
- **事实 A（共享 POST 无 durable manual）**：`TestSharedPOSTGroupBlocksAutomaticRelease` 仅断言“不写 event”；`plan.manual` 只留下 warning，没有持久 manual 标记、receipt、alert metric 或可操作队列；admission/S19 仍提交，scan 每分钟重复日志，库存一直保留，**没有 owner 能可靠发现/执行人工释放**。design §2.2.4 要求“标记人工/告警”，当前只有一条非持久日志。
- **事实 B（未知当前 group fail-open）**：如果当前触发组没有 `order_fulfillment_groups` 行（历史/数据缺陷），`current=UNKNOWN`、`refundReleaseGroupIsPOST` 为 false；其他组无 active 时 `emit=true`，会按订单全量释放。该组可能实际已 SHIPPED/POST，释放会触碰已离仓/已 restock 的库存，违背 design §2.2.4 的“POST 不释放”和 §7“缺组 fail-closed”。现有 `TestAdmitRefundMissingGroupRejectsRelease` 只覆盖 `order_items.fulfillment_group_no` 为空，不覆盖“有 item 组但无 group 行”。
- **判定**：共享 POST 的 manual 未被持久化/告警；未知当前 group 的 fail-open 可造成库存错误，属数据完整性阻断。需要：未知 current group 强制 manual/defer；共享 POST 建立持久人工标记/告警（或明确 ops 入口）；并补“有 item 无 group 行 → 不释放”和“共享 POST → 有可查询 manual 状态”测试。

---

## 2. 已关闭 / 通过

- **旧 B1（group 级 item vs order/sku/warehouse reservation quantity）已关闭**：Order 改为 order-level full items（`loadOrderReleaseItems`，聚合整个订单），Inventory HELD 调用 `Release` 不传 quantity（`refund_release_consumer.go:76-80`），CONFIRMED 用 reservation 的 authoritative quantity 调 `TccCancelConfirm`（`:81-95`）；`TestRefundReleaseConsumerReleasesHeldOnce`、`RestocksConfirmedOnce`、`MultiWarehouse`、`ConcurrentWorkers` 全绿。整单 full release、Inventory 忽略 event quantity 已验证。
- **B2 source 已关闭**：`internal/services/aftersales/workers.go:332-341` 的 `refundAdmissionSource` 对 `sourceSystem` 返回 `system_shortage`；`TestAdmitRefundSystemShortageSource`、`TestDeferredReleaseScanUsesShortageSource` PASS；未知 `payment/legacy_expired/unknown` 由 `refundReleaseSource` 拒绝，空 source 兼容 aftersale。
- **deferred source 重建**：`deferredRefundReleaseSource` 先读 `refund_applications.source`，再读 resolved shortage case；TCC/zero/shortage/aftersale 路径有测试（`TestDeferredReleaseScanUsesShortageSource`）；未发现新的 source 重建 blocker。
- **T1.4b2b/b2 zero 回归**：全部 PASS（task 先持久、AdmitRefund zero short-circuit、失败无 S19、REFUNDED/0/CONSUMED、无 Payment、create/approve/system captured=0、非 zero 身份 fail-closed、return 先 restock）。`TestZero...` 全绿。
- **边界**：`git diff --name-only 64a2e9d..8acffab` 无 proto/migrations/Inventory schema；无 T1.5/T1.6/T1.7/T5；Order 只写 outbox，Inventory 只改 reservation/stock；C-05 无迁移。
- **Tier**：commit `8acffab` 有 `Tier: L2` 与 `Tier-Rationale`；完整 change-input（Spec-Ref、allowed_paths、baseline_impact）未在 commit trailer 出现，属治理完整性 [TODO]，不单独升为 blocker。

---

## 3. 测试证据（Reviewer 独立复跑）

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0**，53.715s |
| `go test ./internal/platform/events -run 'TestRefundRelease'` | **PASS**，0.264s |
| `go test ./internal/services/order -run 'TestOrderLevel|TestDeferredRelease|TestAdmitRefund|TestRefundRelease|TestSharedPOST'` | **PASS**，20.870s（含 order-level/deferred/POST/source 测试） |
| `go test ./internal/services/inventory -run 'TestRefundRelease'` | **PASS**，3.142s |
| `go test ./internal/services/aftersales -run 'TestZero|TestPositiveRefund|TestNonZero|TestCapturedZero'` | **PASS**，11.714s |
| `-race order`（DeferredReleaseScan/OrderLevel） | **ok**，2.885s |
| `-race inventory`（ConcurrentWorkers/ReleasesHeldOnce） | **ok**，1.986s |
| Reviewer overlay：两组 TCC 均 REFUNDING | scan 后 **0 release event**（B1；断言 0 通过） |
| Reviewer overlay：100 个 prior manual + 1 个 ready target | scan 后 **target 0 event**（B2 starvation；断言 0 通过） |
| `go vet` / `gofmt -l` | exit 0 / 无输出 |

- 未发现 SKIP 被当 PASS；AC-50/AC-67/AC-68 未标 AC-VERIFIED。
- 本轮使用当前工作区（代码 head 无额外改动）；overlay 测试不写入仓库。

---

## 4. 非阻断 / 风险

1. **scan 周期硬编码**：`cmd/order/main.go:41` 固定 `time.Minute`，未接 config；属运维策略 [TODO]，但在 B2 的 starvation 未解决前会放大延迟。
2. **无 receipt**：design C-05 明确不新增 release receipt；B2 的 scan cursor/watermark 需要在“不加 receipt”约束下设计，不能用本地锁替代。
3. **design canonical**：`docs/lead/PAY-SETTLE-01/t1.4b2-design.md` 目前 **untracked**，不在 `22f7fea`；没有 T1.4b2 selfcheck。用户称 Rev 1.1 accepted，但仓库没有可审计的 canonical docs commit；merge 前需补。
4. **Manual 语义**：设计允许 shared POST 转人工/AfterSales，但 B3 的日志-only 不是可审计的 manual 状态；T5 告警/ops 入口需另立。
5. **Missing group row** 的 fail-open（B3-B）与 **current group REFUNDING**（B1）均无现有测试；补测试后才能声称 coverage。

---

## 5. 边界 / 结论

- **未越界**：无 T1.5/T1.6/T1.7/T5、无 proto/migrations/Inventory schema、无 Payment/Order 外部协议扩展；Order release event 在 admission outbox，Inventory 只执行。
- **verdict FAIL（有范围）**：旧 group/reservation B1 已关闭；B2 source 已关闭；但 **TCC 多组 last-active（B1）**、**scan starvation/unapproved production backfill（B2）**、**shared POST/unknown group manual/fail-open（B3）** 仍阻止 T1.4b2 关闭。
- **不批准合并；不是 Release PASS；不替代人类 merge 决策。** B1/B2 需要 Architect/Dev 决定状态/扫描/backfill 方案；B3 需要持久 manual/fail-closed 设计；不得用局部 if 或新 change_id 绕过。
