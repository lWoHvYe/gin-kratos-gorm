# ADR-0011: legacy 24h release source + release 完成回执（T5 / AC-63）

| 项 | 值 |
|---|---|
| Status | **Accepted**（2026-11-27 人类裁决 A；reviewer 复审待） |
| Change ID | `PAY-SETTLE-01`（T5） |
| Tier | L2：资金/退款、跨服务事件、状态机 |
| 取代 | `0007-amd1-addendum-legacy-release.md` 的 DEFER 结论及其 §5 的 `legacy_expired` source 提案；`0007-amd1-addendum-r0.4-c04-release.md` §2.3.2 的“Amendment 待写” |
| 关联 | SPEC-0002 AC-63/INV-46；ADR-0007 Amd1 §4.4.3；`legacy_resolution_worker.go`；`refund_release.go`；`inventory/refund_release_consumer.go` |

## 背景

AC-63 要求 legacy 24h 补偿顺序：撤 permit → `CancelIfCreated` → INV-44 释放预留 → **Payment.Refund 最后** → `EXPIRED_REFUND`；失败保持 `COMPENSATING`。现有 INV-44 producer 只写 outbox **请求**，Inventory consumer 异步执行，Order 没有 release 完成回执，无法证明“释放已完成”后再打款；加一个 source 标签不足以满足 AC-63。

## 决策

1. **新增 source/reason**：`legacy_24h` + `LEGACY_24H_EXPIRY`，只用于 legacy 24h 补偿；禁止复用 `tcc/system_zero/system_shortage/aftersale`，禁止复用 `payment/legacy_expired`。注意：`refund_id=pay:<payment_no>:legacy_expired` 是既有 refund_id 后缀，不是 release source；旧 addendum §5 的 `legacy_expired` source 提案被本 ADR 取代。
2. **release 请求**：`release_id=rr:<refund_id>`，`refund_id=pay:<payment_no>:legacy_expired`，`release_scope=order_reservation_full`，`group_count=1`；仅单组；多组/无组不产生 release，保持 `COMPENSATING`/人工；若已有组离仓，按 AC-63 优先转 `EXPIRED_FINANCE`，不进入 release/Payment。
3. **ack 事件**：Inventory 在完成 release 的**同一 DB 事务**写 outbox `shop.inventory.refund_release_completed.v1`（`release_id/refund_id/order_no/status/items/released_at`）；`status=RELEASED|FAILED`。旧 consumer 先升级，Order producer 后才能发送；未升级的旧 consumer 因 source 不在封闭枚举而 DLQ/告警，绝不静默当作成功。
4. **Order 回执表**（additive）：`refund_release_receipts(release_id PK, refund_id UNIQUE, order_no, state PENDING|RELEASED|FAILED, version, lease_owner, lease_until, last_error, updated_at)`。Order 在写 release 请求的同一事务写 `PENDING` 回执。
5. **补偿推进**：ack consumer 按 `release_id` 幂等更新回执；只有 `RELEASED` 才允许 `Payment.Refund`（按 `refund_id` 幂等），成功后再 `EXPIRED_REFUND`；`FAILED`/超时保持 `COMPENSATING` + 告警，绝不打款。
6. **完成屏障**：Payment 前必须读到 `RELEASED` 回执；不依赖 outbox append 成功、不依赖 consumer 日志、不轮询 Inventory DB。回执/lease 用 CAS 抢占，重复 ack 幂等。
7. **部署顺序**：Inventory ack producer/consumer → Order ack consumer/回执 → Order legacy producer；回滚不得先回退 Inventory consumer。旧 consumer 的 DLQ/告警按共享 consumer 框架处理，不重试为成功。

## 备选与否决

- **只加 source，不加 ack**：否决；无法证明 release 完成，Payment 可能先于 release，违反 AC-63。
- **复用 `aftersale/system_*` source**：否决；伪造业务来源，且仍无 ack。
- **同步 Inventory Release RPC**：不是本裁决；若未来采用，需独立 ADR 并替换 outbox 契约。
- **直接轮询 Inventory reservation**：否决；跨服务读扩散、无法证明全量 release 完成。

## 后果

- 新增一个 Inventory outbox 事件和 Order 回执表/迁移；均 additive（Phase 1/3 实现，本 ADR 本身不提交迁移）。
- legacy 24h 补偿在 ack 前保持 `COMPENSATING`；ack 后才能 Payment；多组仍人工。
- AC-63 可闭环；AC-60/63 的验证仍需 `TestAC63` 全链路与 T5 真实迁移证据。

## 核对（Phase 2/4 planned，尚未存在）

- `TestLegacy24hReleaseAck`：请求→ack→Payment 最后；FAILED/超时零 Payment；重复 ack 幂等。
- `TestLegacy24hReleaseSourceContract`：source/reason 封闭枚举；旧 consumer 兼容。
- `TestAC63` 全链路：permit/CancelIfCreated/release ack/Payment/EXPIRED_REFUND；失败保持 COMPENSATING。
- 当前已存在：T5.4 真实迁移、`TestAC60`、`TestAC63` 前缀（`t5-status.md`）。
