# ADR-0007 Amendment 1 Addendum 5 — RefundBatch C-04 语义裁决

| 项 | 值 |
|---|---|
| Status | **superseded by ADR-0007 Rev 2.3 (2026-11-25)**——原 DRAFT；C-04 RefundBatch 已经 Director **accepted（2026-11-24）**，R-1…R-6 已并入 Rev 2.3 §0.7/§5.9/§12.5.8；本文件仅保留历史/审计证据，不得作为实现依据 |
| Merge note | 已并入 ADR-0007 Rev 2.1（K-1，2026-11-25）；C-1 关闭；原 B-2 的 `draft / not normative` 分类作废。 |
| Base | ADR-0007 Amd1 + T1.4b1 Payment Σ Addendum（DRAFT）+ C-04 Draft |
| WIP | `fc295e5`（Payment `RefundBatch` + C-05 ledger） |
| Target Spec | **SPEC-0002 v1.9（approved 2026-11-24）**；AC-65/INV-48 |
| Change ID | `PAY-SETTLE-01` |
| Scope | 只裁决 6 个 C-04 契约点；不写代码/测试 |
| 前置 | 本 Addendum 与 C-04 已于 2026-11-24 accepted；规范内容已并入 Rev 2.1；`fc295e5` 不构成实现 PASS。 |

---

## 1. 裁决表

| # | 问题 | 裁决 | 依据/参考 |
|---|---|---|---|
| **R-1 重放 status** | 同 batch 重放返回首次持久化 Reply；首次 `REFUNDED` 仍 `REFUNDED`，首次全既有 `ALREADY_REFUNDED` 仍 `ALREADY_REFUNDED`。 | **首次持久化 Reply 是最终语义**；`status` 描述**首次执行**，重放不得重算/降级。只有“首次请求时全部 item 已存在”才是 `ALREADY_REFUNDED`；后续重放即使全部已存在也返回首次 `REFUNDED`。 | WIP `refund_batch.go:121-129`、`:287-346`；C-04 draft §2.6 |
| **R-2 mixed item Σ** | 当前只校验 `Σ(NEW items) <= remaining`；既有 item 不重复扣。全 NEW 等价 `Σ <= captured-refunded`。 | **接受**。既有 item 已在 `intent.refunded_cent` 中，重复扣会 double-count；`expected_total_cent` 仍为**全部 items**之和。C-04 §2.3 必须从“`Σ items <= remaining`”改为“`Σ NEW <= remaining`”，并补 mixed 测试。 | WIP `refund_batch.go:180-184`；C-04 draft §2.3 |
| **R-3 schema** | 新增 `payment_refund_batches`（PK `(payment_no,batch_id)`、`items_fingerprint`、`status`、`refunded_cent`、`reason`、`item_results` JSON）+ `refunds.batch_id` + `uk_refunds_batch_item(payment_no,batch_id,idempotency_key)`，保留 `uk_refunds_idempotency(payment_no,idempotency_key)`。 | **确认该 schema**（C-05 `0012_refund_batch.sql`）。`batch_id` 默认 `''` 用于既有 plain Refund；`refund_id` 即 `idempotency_key`，不另造列。`payment_refund_batches` 的 status 只允许 `REFUNDED/ALREADY_REFUNDED`（应用层校验；可加 CHECK 但非阻塞）。 | WIP `migrations/payment/0012_refund_batch.sql`、`repo.go:213-240` |
| **R-4 错误码** | apperr 无领域码；使用标准 code + `details.reason`，新增 `PAYMENT_NOT_REFUNDABLE`/`BATCH_STATE_INVALID`。 | **确认**。Order 映射：`BATCH_UNAVAILABLE` → 保持 `PAYING` 重试；`INSUFFICIENT_REMAINING/BATCH_SUM_MISMATCH/IDEMPOTENCY_CONFLICT/ITEM_CONFLICT/INVALID_BATCH/PAYMENT_NOT_REFUNDABLE/BATCH_STATE_INVALID` → `MANUAL_HOLD`、零新 Payment、告警；不得把确定性错误重试成部分成功。 | WIP `refund_batch.go:47-67`、`:436-451`；C-04 draft §2.7 |
| **R-5 `refunds.batch_id` 审计** | `batch_id` 只标记本批**新写入**；既有 plain Refund 不改写。mixed 幂等依赖 batch 账本。 | **确认**。既有 item 结果写 `ALREADY_REFUNDED`（含原 `refund_no`），不重写 `refunds` 行、不重发 `payment.refunded`；审计以 `payment_refund_batches.item_results` + `refunds.batch_id` 区分“既有未改写/本批新写”。 | WIP `refund_batch.go:164-168`、`:219-248`、`:259-262`；migration 0012 |
| **R-6 C-04 accepted** | （历史问题）C-04 draft 当时仍 DRAFT；是否需先补语义再 accepted。 | **已执行**：C-04 先纳入 R-1/R-2/R-4/R-5 与 schema R-3，并于 2026-11-24 由 Director accepted；`fc295e5` 不构成实现 PASS。 | C-04 draft `:5`；`docs/review-tracking.md:118` |

---

## 2. 细化语义

### 2.1 重放

- 首次请求写入 `payment_refund_batches` 的 `status/refunded_cent/item_results`。
- 同 `(payment_no,batch_id)` + 同 fingerprint：直接返回持久化 Reply，零写入；`status` 不改写。
- 同 `(payment_no,batch_id)` + 不同 fingerprint：`IDEMPOTENCY_CONFLICT`，零写入。
- 首次 `REFUNDED` 之后的 Replay 不因“现在全部已存在”变为 `ALREADY_REFUNDED`；否则重放响应不一致，破坏幂等。

### 2.2 mixed Σ

```text
existingSum = Σ(existing item.amount_cent)   // 已在 intent.refunded_cent
newSum      = Σ(new item.amount_cent)
remaining   = captured - intent.refunded_cent
require expected_total == Σ(all items)
require newSum <= remaining
```

- 全 NEW：`newSum == Σ(all)`。
- mixed：只扣 NEW；既有 item 不重复扣，batch ledger 记为 `ALREADY_REFUNDED`。
- C-04 draft 的 `Σ items <= remaining` 对 mixed 过严，必须改为 `Σ NEW <= remaining`。

### 2.3 Schema

```sql
-- refunds
batch_id VARCHAR(96) NOT NULL DEFAULT ''
UNIQUE KEY uk_refunds_idempotency (payment_no, idempotency_key)
UNIQUE KEY uk_refunds_batch_item  (payment_no, batch_id, idempotency_key)

-- payment_refund_batches
PRIMARY KEY (payment_no, batch_id)
items_fingerprint CHAR(64) NOT NULL
status VARCHAR(32) NOT NULL          -- REFUNDED / ALREADY_REFUNDED
refunded_cent BIGINT NOT NULL
reason VARCHAR(255) NOT NULL DEFAULT ''
item_results JSON NOT NULL
created_at/updated_at DATETIME(3)
```

- `refunds.batch_id=''` 表示 legacy/plain Refund，不因 RefundBatch 重写。
- `item_results` 保存每项 `refund_id/refund_no/status/amount`；`ALREADY_REFUNDED` 项的 `refund_no` 必须来自既有 refund 事实。
- `refunds.batch_id` 只对**新写入**行赋值；mixed 的既有行保持原值（通常 `''`）。

### 2.4 错误映射（Order）

| Payment reason | Order 行为 |
|---|---|
| `BATCH_UNAVAILABLE` | 保持 `PAYING`，退避重试；不得打款/部分写 |
| `INSUFFICIENT_REMAINING` | `MANUAL_HOLD`；零新 Payment |
| `BATCH_SUM_MISMATCH` | `MANUAL_HOLD`；调用方 bug/参数错误 |
| `IDEMPOTENCY_CONFLICT` | `MANUAL_HOLD`；同 batch 不同 items |
| `ITEM_CONFLICT` | `MANUAL_HOLD`；同 refund_id 不同 amount |
| `INVALID_BATCH` | `MANUAL_HOLD`（或调用方修正后新 batch） |
| `PAYMENT_NOT_REFUNDABLE` | `MANUAL_HOLD`/终态；不重试 |
| `BATCH_STATE_INVALID` | `MANUAL_HOLD`/终态；不重试 |

---

## 3. Order 预检是否可进入

- **C-04 已于 2026-11-24 accepted；Order 预检可进入实现（仍需 Reviewer/AC 证据）**。
- Order 预检设计本身已具备条件（`B/done/inFlight/outstanding/R/unknown`、`expected_total=outstanding`、in-flight 超时、lazy reconcile）；C-04 已 accepted 且 R-1/R-2/R-4/R-5 + schema 已并入，Order 预检接到 `fc295e5` 仍须 Reviewer 复审，不得把 WIP 当实现 PASS。
- `fc295e5` 的 WIP 行为与 R-1/R-2/R-3/R-5 一致；R-4 的错误码需并入 C-04 后再由 Order 映射。

## 4. 状态

- **Status: superseded by ADR-0007 Rev 2.1 (2026-11-25)**。原 DRAFT 状态随 C-04 accepted（2026-11-24）与 Rev 2.1 合并结束；本文件仅保留历史/审计证据；`fc295e5` 不构成实现 PASS。
- 本文件不修改代码/迁移/测试。
