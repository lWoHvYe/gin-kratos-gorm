# C-04 — Payment.RefundBatch（T1.4b1-R）

| 项 | 值 |
|---|---|
| Status | **accepted**（Director 2026-11-24 批准；C-04 RefundBatch） |
| Approved | **2026-11-24** |
| Revision | **1.0**（C-04；原 r1 定稿；R-1…R-6 并入） |
| Owner | System Architect |
| Change ID | `PAY-SETTLE-01` |
| Scope | 仅 Payment 原子批退款契约；不写服务代码/测试 |
| Base | ADR-0007 Amd1 + T1.4b1 Payment Σ Addendum + `t1.4b1-r-plan.md` |
| Target Spec | **SPEC-0002 v1.9（approved 2026-11-24）**；AC-65/INV-48 |
| 关联 | T1.4b1 Reviewer B1；Order preflight `B/done/inFlight/outstanding/R` |

> 本契约修正 `t1.4b1-r-plan.md` §2 的 `Σ items == B`：Payment 不知道 Order 的 `B`，且 resume/部分组已退时 `items` 只应包含 outstanding；Payment 只校验调用方声明的 `expected_total_cent` 与 `Σ items`，以及 **`Σ NEW items <= remaining`**（既有 item 不重复扣）。`B` 的算法归 Order。

## 1. Proto

```proto
rpc RefundBatch (RefundBatchRequest) returns (RefundBatchReply);
rpc GetRefundByKey (GetRefundByKeyRequest) returns (.mall.v1.Refund); // 已有 C-04；此处仅引用

message RefundBatchItem {
  string refund_id = 1;      // 组级稳定 id；不得复用为其他金额
  int64 amount_cent = 2;     // >0
}

message RefundBatchRequest {
  string payment_no = 1;
  string batch_id = 2;       // 幂等批标识；同一 payment_no 内唯一
  int64 expected_total_cent = 3; // 必须 == Σ items；Order 的 outstanding=B-done-inFlight
  repeated RefundBatchItem items = 4;
  string reason = 5;
}

message RefundBatchItemResult {
  string refund_id = 1;
  string refund_no = 2;      // 既有/新退款号
  string status = 3;         // REFUNDED / ALREADY_REFUNDED / CONFLICT
  int64 amount_cent = 4;
}

message RefundBatchReply {
  string batch_id = 1;
  string status = 2;         // REFUNDED / ALREADY_REFUNDED
  int64 refunded_cent = 3;   // Σ items；不是 intent 总退款额
  repeated RefundBatchItemResult item_results = 4;
}
```

## 2. 语义（强制）

1. **输入校验**：`payment_no/batch_id` 非空；`items` 非空、`refund_id` 唯一、`amount_cent>0`；`expected_total_cent == Σ items.amount_cent`。否则 `INVALID_BATCH`，零写入。
2. **行锁**：在 `payment_intents` 行锁内计算 `remaining = captured - refunded`；外部退款/取消退款必须锁同一 intent 行。
3. **Σ**：
   - `Σ items != expected_total_cent` → `BATCH_SUM_MISMATCH`（`expected_total_cent` 是**全部 items**之和；Order 的 `outstanding`）。
   - 设 `existingSum = Σ(已存在同 refund_id/amount 的 item)`、`newSum = Σ items - existingSum`、`remaining = captured - refunded`；**`newSum > remaining` → `INSUFFICIENT_REMAINING`**。
   - 既有 item 已计入 `intent.refunded_cent`，不得重复扣；全 NEW 时 `newSum == Σ items`，等价于 `Σ items <= remaining`。
   - 两个错误均零写入（不插 refund/outbox、不改 intent）。
   - **不校验 `Σ items == B`**；`B` 不是 Payment 的输入，也不是 Payment 的事实。
4. **幂等**：
   - `(payment_no, batch_id)` 的重复请求必须返回同一 `RefundBatchReply`；若 items/expected_total 与首次不同 → `IDEMPOTENCY_CONFLICT`，零写入。
   - 同一 `refund_id` 已存在且金额一致 → 视为 `ALREADY_REFUNDED`，可包含在成功批次中，不重复插行/事件；金额或 `payment_no` 不一致 → `ITEM_CONFLICT`，整批零写入。
   - 同 `batch_id` 不允许混入新 refund_id（否则 `IDEMPOTENCY_CONFLICT`）。
5. **全成功或全失败**：所有 items 的 refund 行/outbox 在同一事务内写；任一项冲突/超限/失败 → 整个事务 rollback，不产生部分退款。不允许逐项成功后再报错。
6. **Reply（R-1：首次持久化 Reply 固定）**：
   - `status` 在**首次执行**时确定，并持久化到 `payment_refund_batches`；重复请求必须原样返回该 Reply，不得重算/降级。
   - 首次有至少一个 NEW item → `status=REFUNDED`；即使后续重放时全部 item 已存在，仍返回 `REFUNDED`。
   - 首次请求时全部 item 已存在（`newSum=0`）→ `status=ALREADY_REFUNDED`；重放仍返回 `ALREADY_REFUNDED`。
   - `refunded_cent = Σ items`（本批全部 item 金额，含既有），不是 intent 总 `refunded_cent`；`item_results` 每项带 `refund_no/status/amount`。
   - `payment.refunded` 只为**新插入**的 item 发一次；既有 item 不重复发事件，`item_results.status=ALREADY_REFUNDED`。
7. **错误码（R-4）**：
   - 使用 apperr 标准 code + `details.reason`；reason 至少：
     `INVALID_BATCH`、`BATCH_SUM_MISMATCH`、`INSUFFICIENT_REMAINING`、`IDEMPOTENCY_CONFLICT`、`ITEM_CONFLICT`、`BATCH_UNAVAILABLE`、`PAYMENT_NOT_REFUNDABLE`（intent 从未 captured）、`BATCH_STATE_INVALID`（ledger/refund 事实不一致）。
   - 所有错误零写入；不得返回半成功。
   - **Order 映射**：
     - `BATCH_UNAVAILABLE` → 保持 `PAYING`，退避重试；
     - `INSUFFICIENT_REMAINING/BATCH_SUM_MISMATCH/IDEMPOTENCY_CONFLICT/ITEM_CONFLICT/INVALID_BATCH/PAYMENT_NOT_REFUNDABLE/BATCH_STATE_INVALID` → `MANUAL_HOLD`、零新 Payment、告警；不得把确定性错误重试成部分成功。
8. **存储（R-3/R-5，定稿 schema）**：C-05 必须新增/收敛以下对象；不再是“建议/或”：
   - `refunds.batch_id VARCHAR(96) NOT NULL DEFAULT ''`；`batch_id=''` 表示既有/plain Refund，**RefundBatch 不得改写既有行**；只有本批新写入行赋本批 `batch_id`。
   - 删除 legacy `uk_refunds_idem(payment_no, amount_cent, reason)`（允许同 payment 同金额/reason 的多个组）。
   - 保留 `uk_refunds_idempotency(payment_no, idempotency_key)`；`refund_id` 即 `idempotency_key`，不另造列。
   - 新增 `uk_refunds_batch_item(payment_no, batch_id, idempotency_key)`，即 `UNIQUE(payment_no,batch_id,refund_id)`。
   - 新增 `payment_refund_batches`：`PRIMARY KEY(payment_no,batch_id)`、`items_fingerprint CHAR(64)`、`status VARCHAR(32)`（仅 `REFUNDED/ALREADY_REFUNDED`）、`refunded_cent BIGINT`、`reason`、`item_results JSON`、`created_at/updated_at`。
   - `item_results` 持久化每项 `refund_id/refund_no/status/amount`；mixed 的既有项记 `ALREADY_REFUNDED` 且保留原 `refund_no`，审计以该 JSON + `refunds.batch_id` 区分“既有未改写/本批新写”。

## 3. 与 Order preflight 的边界

Order 负责：
- `B = batch.sum_group_cap_cent`；
- `done`（commitment SETTLED 或 GetRefundByKey 成功）；
- `inFlight`（PENDING/PROCESSING）；
- `outstanding = B - done - inFlight`；
- `unknown`（application DONE 但无 Payment 事实）；
- `R = captured - refunded`；
- 门禁：`B == done+inFlight+outstanding`、`unknown==0`、`R == B-done`；否则 MANUAL_HOLD 零 Payment；
- `inFlight>0` 不调用 RefundBatch；
- 调用时 `expected_total_cent = outstanding`（不是 B），items 只含 outstanding 组。

Payment 不重复 Order 的 `B` 计算；只保证 `Σ items == expected_total_cent`、**`Σ NEW items <= remaining`** 且 all-or-none。

## 4. R-1…R-6 对照与 Plan 更新

| Ruling | 本文落点 |
|---|---|
| R-1 重放 status | §2.6：首次持久化 Reply 固定 |
| R-2 mixed Σ | §2.3：`newSum <= remaining`，既有不重复扣 |
| R-3 schema | §2.8：定稿 schema，C-05 必需 |
| R-4 错误码/Order 映射 | §2.7：新增两码 + MANUAL_HOLD 映射 |
| R-5 `refunds.batch_id` 审计 | §2.8：只标记新写入，既有不改写 |
| R-6 C-04 accepted | **accepted 2026-11-24**；C-04 已并入 R-1..R-6；`fc295e5` 仍需 Reviewer/Order 预检，不构成实现 PASS |

Plan/C-05 更新：
- `t1.4b1-r-plan.md` §2 的 `Σ items == B` 必须删除，改为 `expected_total_cent`；mixed 用 `newSum`。
- C-05 按 §2.8 定稿：`refunds.batch_id`、drop `uk_refunds_idem`、`uk_refunds_idempotency`、`uk_refunds_batch_item`、`payment_refund_batches`。
- 事件丢失/in-flight 的 durable lazy reconcile owner（T1.4b2/T5.3）需在 Plan 明确；`GetRefundByKey` 成功但事件丢失时 batch 不得永久 PAYING。
- 测试矩阵见 `t1.4b1-r-plan.md` §5；需补 `ADMITTED/PAYING` resume、GetRefundByKey 超时、同 batch_id 不同 items、既有 item + 新 item 混合。

## 5. 状态与声明

- **Status: accepted**（Director 2026-11-24 批准；Revision 1.0）。
- `fc295e5` 是 Payment WIP，**不因 C-04 accepted 而成为实现 PASS**；由 Reviewer/Order 预检按本契约复审。
- 本 C-04 只定契约，不含 Payment/Order 代码、迁移或测试。

## 6. 变更记录

| Revision | 日期 | 变更 | 影响 |
|---|---|---|---|
| 0.1-draft（r1） | 2026-11-24 | 初始 C-04 RefundBatch：expected_total_cent、per-item results、C-05 batch ledger；并入 R-1…R-6 | 新建，DRAFT |
| **1.0（accepted）** | 2026-11-24 | Director 批准 C-04：Status accepted、Approved 2026-11-24、Revision 1.0；Target Spec v1.9；保留 R-1…R-6；不批准 `fc295e5` 实现 | Reviewer/Order 预检仍需；不构成实现 PASS |
