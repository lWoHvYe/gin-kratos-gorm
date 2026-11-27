# T5 AC-63 release ack Plan（依据 ADR-0011）

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01`（T5） |
| 状态 | Approved A（人类 2026-11-27）；实现中 |
| 目标 | `COMPENSATING → release request → ack → Payment → EXPIRED_REFUND`；失败保持 `COMPENSATING` |

## Phase 1 — 契约/事件（无 Payment）

- `events`：`RefundReleaseSourceLegacy24h` + `RefundReleaseReasonLegacy24hExpiry`；registry/validator 测试；**先**让 Inventory consumer 接受。
- 新 ack 事件 `shop.inventory.refund_release_completed.v1`：schema/registry/validator + consumer 契约测试。
- 不发送 producer，直到 consumer 就绪。

## Phase 2 — Inventory ack（同事务）

- Inventory release consumer 成功/永久失败时，在同一 DB tx 写 outbox ack；`release_id` 幂等；部分失败不 ack。
- 测试：成功 ack、DLQ/失败 ack、重复释放幂等、ack outbox 与 reservation 状态同事务。

## Phase 3 — Order 回执与补偿

- additive migration：`refund_release_receipts`；`legacy_resolution_worker` 写 release + PENDING 回执；ack consumer 更新；`RELEASED` 才 Payment，成功 `EXPIRED_REFUND`，`FAILED`/超时 `COMPENSATING`。
- lease/CAS 防并发；重复 ack/Payment 幂等。
- 测试：ack 前零 Payment；ack 后 Payment 最后；FAILED 零 Payment；并发只一个；多组不释放。

## Phase 4 — 证据

- `TestAC63` 全链路 + T5.4 真实迁移；reviewer base/head；AC-60/63 验收表。
- `TestAC60` 的 no-after-sale 自动分配/Inventory HELD/缺货分支另记（不阻断 AC-63）。

## stop_conditions

- 旧 consumer 未接受新 source → 不发送 producer。
- ack 不能与 release 同事务 → 停止，回 Architect。
- Payment 前未读到 RELEASED → 停止，不降级为“假设成功”。
- 多组/无组不能安全释放 → 保持 COMPENSATING/人工，不默认 G1。
