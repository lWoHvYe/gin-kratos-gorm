# ADR-0007 Amendment 1 Addendum — PAY-SETTLE-01 T2.4 legacy `COMPENSATING` release source 裁决

| 项 | 值 |
|---|---|
| Status | **DRAFT 0.1 — pending QC / human**（未批准；不构成实现依据） |
| Change ID | **`PAY-SETTLE-01`**（T2.4；Parent `PAY-SETTLE-01`） |
| Tier | **L2**：资金/退款、跨服务、状态机、事件契约；不适用 L1 |
| Base（文档） | `2507a86`（T2.4 selfcheck）；代码证据 `e6d8bf9a90117711351f26818ab4cd9c6836f8dc`；当前工作树 `bda51bb` |
| Spec-Ref | SPEC-0002 **v1.11 approved** AC-63（`:346-362`）、INV-46（`:142`）、S16（`:86`）；ADR-0007 Rev 2.3 §4.4.3（`:489-504`）/§5.7（`:706-730`）；`0019` |
| 输入 | `docs/implementation/PAY-SETTLE-01-T2-selfcheck.md` §7.3 `[BLOCKED]`；`refund_release.go:32-48,216`；`events.go:42-57`；`registry.go:200-205,271-277` |
| Actor | `arch-legacy-release`（System Architect） |
| Reviewed-by | 待 QC/human |
| 硬边界 | **本裁决不新增/修改代码、RPC、proto、表、迁移、事件字段**；不改 AC-63/INV-46 文本；不 commit |
| 结论 | **DEFER 到 T3**：不向 INV-44 枚举添加 `legacy_expired`；T2.4 的 `COMPENSATING→EXPIRED_REFUND`/Payment 不实现；保持 `COMPENSATING`；7 天/`REJECTED` 独立于 release source，见 §5 |

## 0. 裁决

**不为 `legacy_expired` 增加 INV-44 release source/reason；将 `COMPENSATING` 成功、`Payment.Refund`、`EXPIRED_REFUND` 延期到 T3（release/commitment）**。

- T2.4 现状（`legacy_resolution_worker.go:180-236`）：已撤 `NONE` permit、对 `CONSUMED+CREATED` 调 `CancelIfCreated`；随后因 `REFUND_RELEASE_SOURCE_UNSUPPORTED` 保持 `COMPENSATING`，**0 release、0 Payment、0 EXPIRED_REFUND**。该行为正确、必须保留。
- **不是“加一个枚举值”就能闭合**：AC-63 要求“撤 permit → CancelIfCreated → INV-44 释放 → Payment.Refund 最后”，且补偿失败保持 `COMPENSATING`（`:360-361`）；现有 INV-44 producer 只写 outbox 请求，**没有 release 完成回执/ack/失败回传**（`refund_release.go:216` B3 deferred：无 receipt table/scan worker）。加 source 只改变标签，不能证明 release 已完成；Payment 若先执行，会违反 AC-63 的顺序/失败语义。
- T3 拥有 release/commitment 的持久任务、授权与人工处置（`docs/lead/PAY-SETTLE-01/tasks.md:93-99` T3.1–T3.7）；T2.4 不得越权改 T3。延期是契约边界，不是实现妥协。

## 1. 事实基线（证据链）

| 事实 | 证据 |
|---|---|
| AC-63 `COMPENSATING` 必须先完成 permit/CancelIfCreated/INV-44 release，Payment 最后；失败保持 `COMPENSATING` | SPEC-0002 `:360-361`；D-22 `:371` |
| INV-44 release 是**订单级全量**、仅单组安全；多组不释放（P1/manual） | `refund_release.go:17-21,174-186` |
| release producer 的 source 是封闭枚举，仅 `tcc/system_zero/system_shortage/aftersale`；`legacy_expired` 被拒绝 | `refund_release.go:32-48`；`events.go:49-57`；`registry.go:200-205,271-277` |
| release 事件由 Order 在准入事务写 outbox；Inventory consumer 异步执行；无 release receipt/ack/scan worker | `refund_release.go:166-210,216`；`inventory/refund_release_consumer.go:47-133` |
| legacy COMPENSATING 已写 `refund_applications(source=legacy_expired, group_no NULL, non_effective_capture=1)`，但尚未 Payment | ADR §4.4.3 `:497`；`legacy_resolution_worker.go:180-236` |
| T2.4 selfcheck 明确 `[BLOCKED]`、未声称完成 | `T2-selfcheck.md` §7.3/§7.6（`2507a86`） |
| `EXPIRED_REFUND` 才表示 Payment 完成；`EXPIRED_FINANCE` 是财务终态（7 天） | SPEC AC-63 `:360-362`；D-23 `:371` |

## 2. 为什么“只加 source/reason”不够

1. **顺序/完成度**：AC-63 的“INV-44 释放”在 Payment 前；outbox append 只是**请求**，不是 Inventory 执行成功。consumer 失败会进 DLQ/告警，但 Order 无 ack；若继续 Payment→`EXPIRED_REFUND`，会出现“已退款未释放/释放失败”。
2. **失败保持 COMPENSATING 不可实现**：没有回执/租约任务，Order 无法区分“release pending/succeeded/failed”；加 source 后仍无法满足 AC-63 `:361`。
3. **覆盖范围**：INV-44 只支持单组订单级全量（`group_count=1`）；legacy 可能是多组/无组投影，source 不解决多组释放策略（T3/P1）。
4. **消费者兼容**：source/reason 是 closed enum；旧 Inventory 会在 `ValidateJSON` 永久拒绝（DLQ）。即使按“先 consumer 后 producer”部署，仍只解决枚举，不解决完成度。
5. **T3 边界**：T3 才拥有 release 持久任务/人工处置/授权；T2.4 不应重写 T3。

## 3. 若不延期：T3 的最小前置（未来设计，不是本步实现）

> 仅当 T3 决定实现时，才允许加 source；且必须先有 release 完成/失败回执。

- **source/reason（additive，不改字段）**：`legacy_expired` + `LEGACY_EXPIRED_REFUND`；`release_scope=order_reservation_full`、`group_count=1`、`release_id=rr:<refund_id>`（`refund_id=pay:<payment_no>:legacy_expired`）；事件仍用现有 `source/reason/release_id` 字段。
- **生产者**：legacy COMPENSATING 在 permit/CancelIfCreated 后，**仅单组**写 release 请求；多组/无组不写、保持 COMPENSATING 或转人工，不得默认 G1。
- **完成屏障**：T3 必须提供 release 执行结果（receipt/lease task/ack）；Order 只有在 release 完成后才允许 `Payment.Refund`；失败/超时保持 COMPENSATING。
- **consumer 兼容**：先升级所有 Inventory 消费者接受新 source，再升级 Order producer；旧 producer 不产生新事件；回滚不得先回退消费者。
- **幂等**：`release_id=rr:<refund_id>`；Inventory reservation 状态机幂等；Payment 按 `refund_id` 幂等；重复事件 no-op。
- **审计/指标**：事件 source/reason、release_id、refund_id/payment_no、订单/组、release 结果/耗时；release 失败告警；Payment 仅在完成屏障后。
- `[CONTRACT-IMPACT]`：若 T3 需要 receipt/ack 新表/RPC/字段，必须列影响并经人类批准；**本 ADR 不批准任何迁移/RPC**。

## 4. 当前裁决下的状态与行为（T2.4）

| 场景 | 行为 |
|---|---|
| 有捕获、无离仓、24h 到期 | 进入 `COMPENSATING`；撤 `NONE` permit；`CONSUMED+CREATED` 调 `CancelIfCreated`；**不写 release event、不调 Payment、不置 `EXPIRED_REFUND`** |
| release source 不可用（现状） | 保持 `COMPENSATING`；告警/指标；租约内重试；不得回 `OPEN/RESOLVING` |
| `COMPENSATING` 中任一步失败/超时/失租 | 保持 `COMPENSATING`；不提前终态；审计 |
| 已离仓/不确定 | `EXPIRED_FINANCE`（现有分支）；不取消运单/不退款 |
| 7 天 `EXPIRED_FINANCE` 关闭 | **独立于 release source**；按 AC-63/D-23 财务/授权角色 7 天内关闭；不重开；可与 T3 解耦 |
| `REJECTED`（脏数据/不可收敛） | **独立于 release source**；人工终态；不发货/不退款；保留审计 |
| `EXPIRED_REFUND` | **不实现**，等 T3 release 完成屏障 |

## 5. 影响/兼容/测试

### 5.1 影响

- **无代码/迁移/RPC/事件字段变更**（本裁决不实施；T2.4 保持现状）。
- `EXPIRED_REFUND`/Payment 不交付；AC-63、INV-46 保持 **AC-REF**，不得标 AC-VERIFIED；T2.4 不得声称完成。
- 7 天/`REJECTED` 可独立实现（不依赖 source），但仍受 AC-63 整体未闭合约束；不得用它们冒充 `EXPIRED_REFUND`。
- `COMPENSATING` 可能长期持有（有告警/SLA）；需要 ops 监控/人工；T3 交付前不得自动关闭。
- T3 依赖：release 完成/回执、多组策略、审计/指标；本裁决不改变 T3 范围。

### 5.2 兼容

- 不新增枚举 → 现有 producer/consumer 无兼容变化；不产生旧消费者 DLQ。
- 未来 T3 加 source 时必须 **consumer-first**；旧消费者对新 source 永久拒绝；回滚顺序必须保留新消费者，否则事件 DLQ。
- 不得用其它 source 伪造 legacy（TCC/system_zero/aftersale）→ 审计错标、INV-44 语义错误。

### 5.3 测试（T2.4 现状/未来）

| # | 场景 | 期望 |
|---|---|---|
| L-1 | 有捕获、无离仓、24h | `COMPENSATING`；permit REVOKED；`cancel` 调用；**0 release、0 Payment、0 EXPIRED_REFUND** |
| L-2 | source 不可用 | 行保持 `COMPENSATING`；告警；不写 release/Payment |
| L-3 | 重复 tick/失租/并发 | 只一个 lease 胜者；无重复 permit/cancel；无 Payment |
| L-4 | 已离仓/不确定 | `EXPIRED_FINANCE`；不取消运单/不退款 |
| L-5 | `EXPIRED_FINANCE` 7 天关闭 | 独立流程；不重开；审计 |
| L-6 | `REJECTED` | 终态；不发货/不退款；审计 |
| L-7 | 未来 T3 source | consumer-first；`release_id=rr:<refund_id>`；release 完成/失败回执；Payment last；失败保持 COMPENSATING |

## 6. 备选方案

| 方案 | 结论 |
|---|---|
| **延期 T3（本裁决）** | 保持现状；不越权改 T3；AC-63 继续 AC-REF；最小风险 |
| 加 `legacy_expired` source/reason（不延期） | 只加标签，无 release 完成/失败回执，无法满足 Payment last/失败保持；旧消费者 DLQ；拒绝 |
| 加 source + 新增 receipt/ack 表/RPC | 属 T3 `[CONTRACT-IMPACT]`；需人类批准；不是 T2.4 的最小修复 |
| 用其它 source 伪造 / 跳过 release | 违反 INV-44/审计；拒绝 |
| Order 直接调 Inventory Release（同步） | 违反 ADR §5.7（Order 只产生持久请求，不得在事务内直接远程释放）；且无持久恢复；拒绝 |

## 7. 假设与不确定性

- **[假设]** T2.4 的 `COMPENSATING` 前缀（permit/cancel）已按 selfcheck 正确；本裁决不改代码。
- **[假设]** T3 会拥有 release 完成/回执/多组策略；若 T3 不实现，`EXPIRED_REFUND` 永久不可交付（AC-63 AC-REF）。
- **[不确定]** 7 天/`REJECTED` 的 T2.4 独立实现排期；它们不依赖 source，但不能替代 `EXPIRED_REFUND`。
- **[风险]** `COMPENSATING` 长期停留需告警/人工；不得自动关闭/回 OPEN/提前 EXPIRED_REFUND。

**Actor**：`arch-legacy-release`（System Architect）
**Status**：**DRAFT 0.1 — pending QC / human；DEFER COMPENSATING success to T3**
