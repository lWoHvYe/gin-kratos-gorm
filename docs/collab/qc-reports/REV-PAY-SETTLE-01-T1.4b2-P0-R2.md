# REV-PAY-SETTLE-01-T1.4b2-P0-R2 — group_count 收口复审

| 项 | 值 |
|---|---|
| review_scope | `60130e1..1de09e3` 的 T1.4b2a P0-R2：event `group_count` 必填、producer 单组=1/多组不发布、consumer 缺失/0/!=1/covered 不一致 → Permanent/DLQ、Inventory 整笔释放、source、重复/并发；以及 Design Rev 1.3 / selfcheck / change-input。只读；不评审 T1.5/T1.6/T1.7/T5。不构成 Release PASS。 |
| base | `60130e1`（P0 单组 release） |
| code head | `1de09e3`（`wip(order): T1.4b2a require event group_count=1`） |
| design | `docs/lead/PAY-SETTLE-01/t1.4b2-design.md` **Revision 1.3**（本 head 已提交） |
| selfcheck | `docs/implementation/PAY-SETTLE-01-T1.4b2-selfcheck.md`（本 head 新增） |
| Tier / Actor | **L2**（commit `Tier: L2`，`Tier-Rationale` 命中跨服务 inventory/refund）；Actor `dev-phase1g` ≠ Reviewed-by `reviewer-phase1` |
| **verdict** | **PASS（有范围）** |
| release_status | **NOT RELEASE**；不批准合并 |

---

## 1. BLOCKER

**无。**

## 2. group_count 契约（B1 关闭）

- **Event schema**：`events.RefundReleaseRequested.GroupCount int32 json:"group_count"` 无 `omitempty`；registry required 列表含 `group_count`（`internal/platform/events/registry.go:313-316`），`validateRefundRelease` 要求 `GroupCount > 0`（`:226-228`）；`covered_group_nos` 若存在，长度必须等于 `group_count` 且包含 `group_no`（`:240-254`）。
- **Producer**：`internal/services/order/refund_release.go:169-196` 仅 `group_count == 1` 时写 event，`GroupCount: 1`；`group_count > 1` 只 Warn、不写 outbox；`orderGroupCount` 从 `order_fulfillment_groups ∪ order_items` 计数，单组=1、多组不发布。测试 `TestMultiGroupRefundDoesNotWriteRelease`、`MultiGroupConcurrent...`、`MultiGroupZero...`、`SingleGroupEmits...` PASS。
- **Consumer**：`internal/services/inventory/refund_release_consumer.go:63-86` 先 `GroupCount != 1` → `consumer.Permanent`；`covered_group_nos` 存在时要求长度 1 且等于 `group_no`；缺失/0/2/其他组 → Permanent/DLQ，不触碰 reservation。
- **测试**：event `missing group_count`、`zero group_count`、`covered other group`、`covered length mismatch` PASS；Inventory `TestRefundReleaseHandlerRejectsInvalidGroupCount/missing|zero|multi|covered_other` PASS；`TestRefundReleaseHandlerAcceptsSingleGroupCoveredAudit` PASS（covered 仅审计）。
- **结论**：上一轮 B1 的“多组防御不可执行”已由必填 `group_count` 关闭；producer/consumer/registry 三处一致。

## 3. 其他 P0 语义

- **单组 HELD/CONFIRMED**：Inventory `Release` 不传 quantity；CONFIRMED 使用 reservation 权威 quantity；`TestRefundReleaseConsumerReleasesHeldOnce`、`RestocksConfirmedOnce`、`MultiWarehouse`、`ConcurrentWorkers` PASS。
- **单组 POST 不写**：`AdmitRefund` 只在 PRE 调 append；`TestAdmitRefundPostDoesNotWriteReleaseOutbox`、`SystemZeroPost...` PASS。
- **多组不写**：多组正金额/零元/并发均 0 outbox、库存 reserved；测试 PASS。
- **source**：SYSTEM → `system_shortage`；payment/legacy/unknown → `REFUND_RELEASE_SOURCE_UNSUPPORTED`；空=aftersale；`TestRefundReleaseSourceMapping`、`TestAdmitRefundSystemShortageSource`、`UnsupportedReleaseSourceRejects` PASS。
- **重复/多 worker**：reservation 状态 CAS + 重复 event no-op；`TestRefundReleaseConsumerConcurrentWorkers`、`RetriesWithoutDoubleRelease` PASS。
- **Inventory 整笔释放**：HELD full `Release`、CONFIRMED full `TccCancelConfirm`；`group_no`/covered 不参与 reservation 身份；partial failure/DLQ/canceled context PASS。
- **无 scan/backfill**：`cmd/order` 不再注册 reconcile worker；`refund_release.go` 无 scan/last-active；P0 无生产 backfill；多组 reserved 为 P1/人工已知行为。
- **zero 回归**：`TestZero...`、`NonZero...`、`CapturedZero...` PASS；task/S19/REFUNDED/0/CONSUMED/无 Payment/return restock 保持。

## 4. Design / selfcheck / change input

- **Design**：`docs/lead/PAY-SETTLE-01/t1.4b2-design.md` **Revision 1.3** 已随 head 提交；`Status accepted`、`Approved 2026-11-25`、`Tier L2`、`Spec-Ref`、`group_count` 必填/consumer 语义、P0 multi reserved、B2/B3 说明齐全。
- **Selfcheck**：`docs/implementation/PAY-SETTLE-01-T1.4b2-selfcheck.md` 已随 head 提交；含 `Tier`/`Tier-Rationale`、`Spec-Ref`、`Base=60130e1`、`allowed_paths`、`baseline_impact`、三问、测试范围/未覆盖范围与 WIP 声明。
- **Change input**：commit `1de09e3` 含 `change_id`/`Actor`/`Status`/`Tier`/`Tier-Rationale`；`Spec-Ref`/`allowed_paths`/`baseline_impact` 在 selfcheck/design 中可审计。唯一 [TODO]：commit trailer 未重复 `Spec-Ref`，不影响 canonical 记录。
- **No proto/migrations/Inventory schema**：`git diff --name-only 60130e1..1de09e3` 仅 events/code/tests/docs；无 `api/proto`、`api/gen`、`migrations`、Inventory schema。
- **No T1.5/T1.6/T1.7/T5**：diff 无相关路径。

## 5. 测试证据（Reviewer 独立复跑）

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0**，51.109s |
| `go test ./internal/platform/events -run 'TestRefundRelease' -count=1 -v` | **PASS**，0.208s（含 missing/0/covered 用例） |
| `go test ./internal/services/order -run 'TestMultiGroup|TestAdmitRefund|TestRefundReleaseSource' -count=1 -v` | **PASS**，16.784s |
| `go test ./internal/services/inventory -run 'TestRefundRelease' -count=1 -v` | **PASS**，2.408s（含 invalid group_count/covered） |
| `go test ./internal/services/aftersales -run 'TestZero|TestNonZero|TestCapturedZero' -count=1` | **PASS**，10.307s |
| `-race order`（MultiGroupConcurrent/SingleGroup） | **ok**，2.714s |
| `-race inventory`（ConcurrentWorkers/ReleasesHeldOnce） | **ok**，2.399s |
| `go vet` / `gofmt -l` | exit 0 / 无输出 |

未发现 SKIP 被当 PASS；AC-50/AC-67/AC-68 未标 AC-VERIFIED。

## 6. 非阻断 / 已知缺口

1. **TCC 单组端到端**：现有单组测试用 `Source:"aftersale"`/zero；`TestRefundReleaseSourceMapping` 覆盖 tcc 映射，但没有单组 TCC admission→event→Inventory 用例；建议补一条，不阻塞 P0（source 路径共享）。
2. **多组库存不释放**：P0 已知行为（P1/人工）；不是本轮缺口。
3. **no broker e2e**：T5.2/T5.3 需真实 NATS 重投/DLQ/并发；Release 前。
4. **commit trailer** 未重复 `Spec-Ref`/`allowed_paths`；selfcheck/design 有，[TODO]。
5. **旧 60130e1 event（无 group_count）**会被新 consumer DLQ；当前无生产数据，P0 不 backfill；升级环境需按 design 丢弃/重发。

## 7. 声明

- 本报告未修改代码/契约；**PASS** 仅覆盖 `60130e1..1de09e3` 的 T1.4b2a P0-R2（group_count、单组释放、multi reserved、source、Inventory、design/selfcheck）。
- **不是 Release PASS，不批准合并，不替代人类 merge 决策。**
- Tier 确认：**L2**；无 proto/migrations/Inventory schema；无 T1.5/T1.6 越界。
