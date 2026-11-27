# ADR-0010: legacy refund sequence 独立提交（PAY-SETTLE-01 T2.6）

| 项 | 值 |
|---|---|
| Status | **Accepted**（2026-11-27 人类裁决 A；reviewer PASS） |
| Change ID | `PAY-SETTLE-01`（T2.6） |
| 取代 | ADR-0007 Amendment 1 §6.5 中“取号在 `orders` 行锁事务内”这一句的**事务范围**；其余（表结构、`legacy:<order_no>:<seq>`、不得复用/回退）不变 |
| 关联 | `docs/adr/0007-payment-settlement-admission-amd1.md:590-603`、`migrations/order/0019_legacy_payment_resolution.sql:55-58`、`internal/services/order/refund_seq.go` |

## 背景

ADR-0007 Amendment 1 §6.5 与迁移 0019 规定 `order_refund_sequences` 的取号在 `orders` 行锁事务内，并写明“不得复用/回退”。但若取号与 refund 写入共用同一事务，refund 事务回滚会把 `next_seq` 一起回滚，下一次取号复用同一个 `seq`。这与“不得复用/回退”冲突；reviewer 在 T2.6 复审中以实测 `BEGIN … ROLLBACK` 复现（两次都得到 `seq=1`）。

## 决策

**取号使用独立短事务提交**：

1. 调用方仍必须先持有 `orders` 行锁（同一订单的退款创建被行锁串行化）。
2. `nextLegacyRefundSeq` 在 `s.db` 上开启自己的短事务，执行
   `INSERT ... ON DUPLICATE KEY UPDATE next_seq = LAST_INSERT_ID(next_seq + 1)`，
   读回 `LAST_INSERT_ID()` 后**提交**，再返回 `seq`。
3. refund 事务后续回滚**不会**回滚已提交的 seq；该号被消耗（允许 gap），但不会被第二个已提交 refund 复用。
4. 取号事务失败即 fail-closed；不得在内存里补号或回退。

## 备选与否决

- **同事务取号 + 把语义改成“已提交的 refund_id 不复用；rollback 可复用”**：否决。它削弱了“不得复用/回退”的保守不变量；退款身份应默认不依赖“回滚的退款不算数”的推理。
- **MySQL 自治事务/保存点**：MySQL 没有自治事务；保存点仍随外层回滚。否决。
- **先取号、后开 refund 事务**：与“调用方持 orders 行锁”冲突，且并发下可能让同一订单的两个退款拿到不同号后一个失败，产生更多 gap；不采用。

## 后果

- 允许 seq gap（进程崩溃/refund 回滚会消耗号）；`legacy:<order_no>:<seq>` 只保证**已提交**退款不复用、不回退。
- 取号与 refund 不在同一原子事务；调用方必须：先锁 orders，再取号，再写 refund/ledger；失败时按 fail-closed 报错，不重试同一号。
- T3.5 `AdmitRefund` 是首个调用方；T2.6 只交付取号器与测试。
- 迁移 0019 的表结构/注释不变；本 ADR 只澄清事务范围。

## 核对

- `TestLegacyRefundSeqRollbackDoesNotReuse`：外层 tx rollback 后，下一次取号 = 上一次 + 1，不复用。
- `TestLegacyRefundSeqConcurrentUnique`：20 并发唯一 1..20。
- `TestLegacyBackfill*`：T2.5 回填不因取号变化而改变。
