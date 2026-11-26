# PAY-SETTLE-01 T2.4 legacy release — 影响分析（DRAFT 0.1）

| 项 | 值 |
|---|---|
| Status | **DRAFT 0.1 — pending QC / human**；不构成实现依据 |
| Change ID | `PAY-SETTLE-01`（T2.4）；Parent 同 |
| Base | `2507a86`（selfcheck）；代码证据 `e6d8bf9`；当前 `bda51bb` |
| 契约输入 | SPEC-0002 v1.11 AC-63/INV-46；ADR-0007 Rev 2.3 §4.4.3/§5.7；`0019`；T2 selfcheck §7.3 |
| 裁决 | **DEFER** `COMPENSATING→EXPIRED_REFUND`/Payment 到 T3；**不添加** `legacy_expired` source；无代码/迁移/RPC |
| Actor | `arch-legacy-release` |
| Reviewed-by | 待 QC/human |

## 1. 结论

- **T2.4 现状保持**：进入 `COMPENSATING` 后只完成 permit `NONE→REVOKED`、`CONSUMED+CREATED` 的 `CancelIfCreated`；**0 release、0 Payment、0 `EXPIRED_REFUND`**；行保持 `COMPENSATING`，告警/重试。
- **不加 `legacy_expired` 到 INV-44 封闭枚举**：source/reason 是 additive，但仅改变标签；现有 producer 只写 outbox 请求，**无 release 完成回执/ack**（`refund_release.go:216`），无法满足 AC-63 “INV-44 释放→Payment 最后”与失败保持 `COMPENSATING`（`:360-361`）。
- **T3 拥有 release/commitment**（`docs/lead/PAY-SETTLE-01/tasks.md:93-99`）；T2.4 不越权重写 T3。
- **7 天 `EXPIRED_FINANCE` 关闭 / `REJECTED`** 不依赖 release source，可在 T2.4 独立实现/评审；但不得冒充 `EXPIRED_REFUND`，AC-63 仍 AC-REF。
- **无代码/迁移/RPC/event 字段**；不改变 T3、0019、T2.5/T2.6。

## 2. 影响面

| 对象 | 变更 |
|---|---|
| `internal/services/order/legacy_resolution_worker.go` | **无**（保持 `errLegacyCompensationBlocked`） |
| `internal/services/order/refund_release.go` / `events` / `registry` | **无**（source 枚举不变） |
| `migrations/**` / proto / RPC | **无** |
| `legacy_payment_resolution` | 保持 `COMPENSATING`；不写 `EXPIRED_REFUND`；7d/REJECTED 独立 |
| T2.4 状态 | **不完成**；`COMPENSATING` 前缀 PASS，成功/EXPIRED_REFUND BLOCKED |
| AC-63 / INV-46 | **AC-REF**；不得 AC-VERIFIED；不得 Release PASS |
| T3 | 依赖：release 完成/回执、多组策略、审计/指标 |

## 3. 备选方案

| 方案 | 优点 | 风险/结论 |
|---|---|---|
| **Defer T3（采用）** | 不越权、不伪造 release、符合 AC-63 顺序 | T2.4 延迟；AC-63 AC-REF；COMPENSATING 长期告警 |
| 加 `legacy_expired` source/reason | additive、无字段/迁移 | **仅标签**；无 release ack；Payment 可能先于 release；旧消费者 DLQ；拒绝 |
| source + release receipt/ack 表/RPC | 可闭合 | 属 T3 `[CONTRACT-IMPACT]`；需人类批准；非 T2.4 最小 |
| 用 TCC/aftersale source 伪造 | 不改枚举 | 审计错标、INV-44 语义错误；拒绝 |
| Order 直接 Inventory Release | 同步可知 | 违反 §5.7“Order 只产生持久请求，不得事务内直接远程”；无持久恢复；拒绝 |

## 4. 兼容性

- 当前不新增 enum → producer/consumer 无变化；不会让旧 Inventory 因新 source 进 DLQ。
- 未来 T3 若加 source：**先升级所有 Inventory 消费者**，再升级 Order producer；旧消费者对新 source `ValidateJSON` 永久拒绝（DLQ）；回滚不得先回退消费者。
- 不得用 `tcc/system_zero/system_shortage/aftersale` 代表 legacy release；会导致 source/reason 审计失真。
- `release_id=rr:<refund_id>` 未来保持；`refund_id=pay:<payment_no>:legacy_expired`；单组/`group_count=1`；多组仍 P1/manual。

## 5. 测试与证据（T2.4）

- L-1：有捕获无离仓 24h → `COMPENSATING`；permit REVOKED；CancelIfCreated；0 release/0 Payment/0 EXPIRED_REFUND。
- L-2：source 不可用 → 保持 COMPENSATING；告警；无 release/Payment。
- L-3：tick/失租/并发 → 单 lease 胜者；无重复 permit/cancel。
- L-4：已离仓/不确定 → `EXPIRED_FINANCE`；不取消/退款。
- L-5：7 天财务关闭（独立）→ 终态不重开；审计。
- L-6：`REJECTED` → 终态；不发货/不退款；审计。
- 未来 T3（L-7）：consumer-first；release ack；Payment last；失败保持 COMPENSATING；`release_id=rr:<refund_id>`。
- 真实 MySQL 8.4 + `-race`；证据由 Developer/Reviewer 绑定 HEAD；不得用 fixture 冒充生产。

## 6. 假设/风险

1. **[假设]** T2.4 前缀（permit/cancel）已正确；本裁决不改代码。
2. **[假设]** T3 会提供 release 完成/回执/多组策略；否则 `EXPIRED_REFUND` 无法交付。
3. **[风险]** `COMPENSATING` 长期停留；需告警/SLA/人工；不得自动终态/回 OPEN。
4. **[风险]** 7 天/`REJECTED` 独立实现可能被误读为 AC-63 完成；Reviewer 必须分开登记。
5. **[不确定]** T3 是否需要新表/RPC/字段；若需要，标 `[CONTRACT-IMPACT]` 并经人类批准。

## 7. 建议

接受 **DEFER T3**；T2.4 记录 `COMPENSATING` scoped 证据，不标完成/AC-VERIFIED；T3 设计 release 完成/回执 + legacy source（consumer-first）后再实现 `EXPIRED_REFUND`；7 天/`REJECTED` 可独立交付但不得冒充 AC-63。

**Actor**：`arch-legacy-release`
**Status**：**DRAFT 0.1 — pending QC / human；DEFER**
