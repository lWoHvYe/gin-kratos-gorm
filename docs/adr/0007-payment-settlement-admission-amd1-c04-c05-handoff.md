# C-04 / C-05 Contract Handoff — PAY-SETTLE-01 (ADR-0007 Amendment 1)

| 项 | 值 |
|---|---|
| 状态 | **handoff（契约输入清单）— 非 C-04/C-05 批准、非实现依据**；**未并入 ADR-0007 Rev 2.1 (2026-11-25)** |
| Merge note | 本文件不是 ADR 规范文本；其 D-27/D-28/D-29 输入已在 Rev 2.1 正文实现（§4.4/§5.2.5/§6.2/§12），结论以 Rev 2.1 为准。 |
| Base | **ADR-0007 Amendment 1（accepted 2026-11-24，Revision 1.0）**；ADR-0007 Rev 1.0 不被重写 |
| Target Spec | **SPEC-0002 v1.8（approved 2026-11-24）**；已显式写“先持久化 `refund_applications(PENDING)` 再 Payment”，与本 ADR §4.5/§5.3 一致；SPEC-0001 v1.4（approved 2026-11-24，C-01） |
| 评审 | QC-13（Payment/终态顺序）、QC-14（`READY FOR HUMAN SPEC+ADR APPROVAL`）；ADR 与 SPEC v1.8 均已批准，C-04/C-05 仍待各自流程 |
| 输入契约 | ADR-0007 Amendment 1 §3（D-27）、§4（C-03）、§5（D-28）、§7（D-29）、§6（schema）；Plan C-03/C-04/C-05 |
| 交付对象 | C-04：proto/事件字段（Architect owner，Developer 只消费）；C-05：迁移（Developer；schema 由 ADR 定） |
| 禁止 | 本文件不写代码/迁移/测试；不批准 C-04/C-05；在 C-04/C-05 评审通过前不得实现 |

> **顺序约束（强制）**：`COMPENSATING` 内先撤未离仓 permit / `CancelIfCreated` / INV-44 释放 → 幂等持久化 `refund_applications(PENDING)` → `Payment.Refund` → **Payment 全部成功后才 `COMPENSATING→EXPIRED_REFUND`**。打款失败/超时保持 `COMPENSATING`，不得提前终态、不得回 OPEN。

---

## 1. C-04 — proto / 事件 / 配置输入

### 1.1 `AdmitRefund`（增量字段，不重编号）

现有 `AdmitRefundRequest` 字段 1–5（`refund_id`、`order_no`、`group_no`、`amount_cent`、`source`）保持不变；新增：

| 字段 | # | 类型 | 语义 |
|---|---|---|---|
| `lines` | 6 | `repeated RefundLine` | 本次退货行；组级/已离仓部分退款必填；整组未离仓也要与商品剩余一致 |
| `responsibility` | 7 | `string`（封闭枚举） | 责任方，AfterSales/缺货事实产出；Order 只校验 |
| `reason_code` | 8 | `string` | 责任原因码，封闭枚举；不得自由文本判责 |
| `freight_refundable_cent` | 9 | `int64` | 本次可退运费，由责任方产出；不得为负 |
| `batch_id` | 10 | `string` | TCC/批处理归属；非 TCC 可空 |

`RefundLine`：

| 字段 | # | 类型 | 语义 |
|---|---|---|---|
| `line_no` | 1 | `int32` | 订单行号；`0` 不作为商品行 |
| `quantity` | 2 | `int32` | 本次退货数量 |
| `goods_cent` | 3 | `int64` | 本次行商品金额；不得超行剩余 |

`AdmitRefundReply` 追加（可选、兼容旧调用）：

| 字段 | # | 类型 |
|---|---|---|
| `batch_id` | 6 | `string` |
| `goods_cent` | 7 | `int64` |
| `freight_cent` | 8 | `int64` |

### 1.2 Payment 退款事实查询

- 新增 `GetRefundByKey(GetRefundByKeyRequest) returns (Refund)`：
  - `GetRefundByKeyRequest { string payment_no = 1; string refund_id = 2; }`
  - `Refund` 现有字段 1–7 不变，追加 `string refund_id = 8;`（或等价 `idempotency_key` 只读字段）。
- 用途：`COMPENSATING` 打款超时/未知、`payment.refunded` 丢失时的惰性对账；不得用 `refund_no` 猜测 `refund_id`。
- `payment.refunded` 必须携带 `refund_id`；Order 只消费订单级字段做投影，commitment 由 Order 单写。

### 1.3 Order 历史解析管理命令

新增 Order 管理 RPC（仅 ops/Director 授权；Payment 不写 Order 队列）：

| RPC | 请求字段 | 说明 |
|---|---|---|
| `GetLegacyResolution` | `order_no` | 只读；返回状态/候选数/`resolution_id` |
| `ResolveLegacyResolution` | `order_no, resolution_id, payment_no, operator, authorized_by, evidence_ref` | 人工唯一候选写回；`OPEN/RESOLVING→RESOLVED` CAS |
| `ResumeLegacyResolution` | `order_no, resolution_id` | 幂等恢复被阻断的 ship/refund；不重复发货/退款 |

> **C-04 M-1（QC-15 修订）**：三个历史队列 RPC 的 reply 统一为 `LegacyResolution`：`order_no/state/candidate_count/resolution_id/pay_cent/blocked_scope/resolution_kind/expires_at_unix/resolved_payment_no/operator/authorized_by/version`。`state`、`resolution_kind`、`blocked_scope` 的取值以 ADR §5.4 为准；本修订只补只读可见字段，不新增队列状态或业务语义。

### 1.4 事件与配置

- `order.settled`：保持 `shop.order.settled.v1`；必填字段不变；新增可选 `schema_version`（缺省 1，>1 影子拒绝）；不新增必填字段。
- `order.refund_requested`：保留 subject；Payment 消费者改为**只观察/告警**，不得 `Refund`；移除日期由 release 记录定，本 R1 不移除。
- `order.refund_admitted`：现有字段不变；Order 的 admission 结果仍按 `refund_id` 幂等。
- 配置：
  - `FULFILLMENT_SETTLE_CONSUMER_MODE=paid|shadow|settled`；**R1 默认 `paid`**；`settled` 需 release 门禁 + 人类批准。
  - `SETTLE_RELEASE_APPROVAL_ID`：启动时校验 `settlement_switch_approvals`；请求 `settled` 且无/过期/不匹配批准 → **启动失败（非零退出）**，不得按 `paid` 运行。**N5**：approval-id 非空但未请求 `settled` → 启动失败（config error），不得默认 `paid`。
  - 旧 `FULFILLMENT_CONSUME_ORDER_SETTLED`：`false`/未设置→`paid`；`true`→受门禁的 `settled` 请求，不得绕过。
  - **配置契约载体（QC-15 B-1 修订）**：`internal/platform/config/config.go` 属于 C-04 增量，必须随 C-04 评审：
    - `FulfillmentSettleConsumerMode` ← `FULFILLMENT_SETTLE_CONSUMER_MODE`（`paid|shadow|settled`，默认 `paid`；大小写/空值/未知值 fail-fast）。
    - `SettleReleaseApprovalID` ← `SETTLE_RELEASE_APPROVAL_ID`；请求 `settled` 时必填，缺失启动 fail-fast；**N5：非空但未请求 `settled` 也是 config error → 启动失败（非零），不得默认 `paid`**。
    - 旧布尔只映射 `paid` 或受门禁的 `settled`，不得映射 `shadow`；新旧同时存在且冲突 → 启动 fail-fast，不静默择一。
    - `shadow`/`settled` 的批准行内容（`target_mode/status/expires_at/release_range`）由 Fulfillment 启动校验；没有有效批准行时 **启动失败（非零退出，ADR §3.2 Rev 2.3）**，不得回退 `paid`，不由 config 包猜测。

---

## 2. C-05 — 迁移 / 表 / 约束输入

### 2.1 新增表

| 表 | 关键约束（详见 ADR §6） |
|---|---|
| `refund_batches` | PK `batch_id`；`(source, order_no, payment_no)` **非 UNIQUE**（多 attempt）；active batch 由 Tx1 订单锁 + state 保证；state CHECK |
| `refund_applications` | PK `refund_id`；`UNIQUE(batch_id, group_no)`；`goods_cent + freight_cent = amount_cent`；`non_effective_capture` 与 `source` 约束；state CHECK |
| `order_refund_commitment_lines` | PK `(refund_id, line_no)`；`line_no=0` 运费行、`>0` 商品行；goods/freight 非负 |
| `refund_commitment_releases` | PK `release_id`；`UNIQUE(refund_id)`、`UNIQUE(authorization_id)`；≥1000 元双人复核 CHECK |
| `order_refund_sequences` | PK `order_no`；`next_seq >= 1` |
| `legacy_payment_resolution` | PK `order_no`；`UNIQUE(resolution_id)`；状态 `OPEN/RESOLVING/COMPENSATING/RESOLVED/EXPIRED_REFUND/EXPIRED_ZERO/EXPIRED_FINANCE/REJECTED`；`resolution_kind`/`blocked_scope` CHECK；lease/version |
| `settlement_shadow_diffs` | PK `(order_no, payment_no)`；state `MATCH/MISMATCH/ERROR` |
| `settlement_switch_approvals` | PK `approval_id`；`target_mode=shadow|settled`；ON 行必须有 `order_paid_admission_retired_at` + `order_paid_subject_removal_date` |

### 2.2 既有表变更

- `order_refund_commitments`：新增 `goods_cent`、`freight_cent`、`composition_known`、`composition_source`；历史行回填 `goods_cent=amount_cent, freight_cent=0, composition_known=0`；新承诺 `composition_known=1` 且行合计等于头金额。
- `order_shipping_permits`：D-29 预检通过后加 `CHECK(state IN ('NONE','CONSUMED','REVOKED'))`；预检若有 `GRANTED`/未知行，**阻断部署**并人工按证据分类，不自动映射。
- `orders.settlement_payment_no`：保持 `NOT NULL DEFAULT ''`；唯一候选 CAS `WHERE settlement_payment_no=''`。
- 不新增 down migration；所有新表/列 additive。

### 2.3 迁移路径与执行

- `migrations/order/**`：`refund_batches`、`refund_applications`、commitment lines/releases/sequences、`legacy_payment_resolution`、commitment 增列、permit CHECK。
- `migrations/payment/**`：如 C-04 需要 `refund_id` 索引/只读列（以 C-04 评审为准）。
- `migrations/fulfillment/**`：`settlement_shadow_diffs`、`settlement_switch_approvals`；**Plan 的 allowed_paths 必须包含该目录**，否则 D-27 无合法路径。
- 真实迁移测试使用 `migrations/`（不得内联 DDL 冒充）；MySQL 8 DDL 非事务，hardened runner + 备份；不做 down migration。

---

## 3. D-27 / D-28 / D-29 输入确认

| ID | 已落入 ADR 的输入 |
|---|---|
| **D-27** | `order.settled` v1 字段/版本/影子比对；`paid|shadow|settled` 三态；R1 默认 `paid`；OFF→SHADOW 影子批准；SHADOW→ON 全量门禁；ON 移除日期；启动用 `settlement_switch_approvals` 校验；旧布尔开关映射 |
| **D-28** | Order 唯一写 `legacy_payment_resolution`；`order_no` PK、`resolution_id` 唯一；24h→`COMPENSATING`；Payment 成功后才 `EXPIRED_REFUND`；打款失败保持 `COMPENSATING`；`EXPIRED_REFUND` 先撤 permit/取消运单/INV-44 释放 |
| **D-29** | `GRANTED` 非合法持久状态；迁移预检有行即阻断；人工按证据分类；代码移除读容忍；未知状态 fail-closed；加 CHECK；无 down migration |

---

## 4. 输入齐备性与门禁

| 项 | 状态 |
|---|---|
| ADR-0007 Amendment 1 | **accepted 2026-11-24**（Revision 1.0） |
| ADR-0007 Rev 1.0 | 正文未改写；头部已加 `Amended by` |
| SPEC-0002 v1.8 | **approved 2026-11-24**；已显式写 Payment 前持久化 `refund_applications(PENDING)`，C-04/C-05 必须与该顺序一致 |
| C-04 | **未批准**；本文件只列输入，交 QC/Developer 评审 |
| C-05 | **未批准**；迁移编号由 Developer 在评审后取下一个可用号 |
| Plan allowed_paths | 需包含 `migrations/fulfillment/**`（QC-12 MAJOR，Plan owner 处理） |

### 4.1 C-04 草案修订记录（QC-15，WIP-DRAFT）

| QC-15 项 | 修订落点 | 状态 |
|---|---|---|
| **B-1** 配置契约 | §1.4 引用 + `internal/platform/config/config.go`（三态/approval id/旧布尔映射/fail-fast） | C-04 草案已补，**待 QC 复审**；仍非批准 |
| **M-1** `LegacyResolution` reply | §1.3 引用 + `api/proto/mall/v1/mall.proto` 的 `LegacyResolution` 字段 5–12 | C-04 草案已补，**待 QC 复审**；仍非批准 |

> 本修订不批准 C-04/C-05，不改变 ADR/SPEC；`GetRefundByKey` 包限定与 `RefundRequest.refund_id` 不新增按 QC-15 裁决保持。

**下一步**：QC 审 C-04/C-05 输入 → Developer 提供可执行性/风险 → C-04/C-05 批准 → 实现。在此之前不得写代码/迁移/测试。
