# ADR-0007 Amendment 1 Addendum — T1.4b2 INV-44 Group Reservation Ruling

| 项 | 值 |
|---|---|
| Status | **superseded / withdrawn**（2026-11-25 人类接受简化；不再作为实现依据） |
| Superseded by | `docs/lead/PAY-SETTLE-01/t1.4b2-design.md`（Revision 1.3，P0 single-group release + group_count） |
| Base | ADR-0007 Amd1 + T1.4b2 Design（accepted）+ C-04 RefundBatch（accepted） |
| WIP | T1.4b2a/b/b2；Reviewer FAIL B1 `[ARCHITECTURAL_FLAW]` |
| Target Spec | **SPEC-0002 v1.9（approved 2026-11-24）**；INV-44、AC-50/AC-67/AC-68；SPEC-0001 INV-03 |
| Change ID | `PAY-SETTLE-01` |
| Scope | 历史裁决，已撤回；仅保留历史，不写实现 |
| 前置 | 历史记录；已由 design Rev 1.3 取代，不再要求 C-05/group reservation |

---

## 1. 裁决摘要

| ID | 结论 |
|---|---|
| **B1** | **采用 Option A：Inventory reservation 增加 group 归属（group-scoped reservation）**。B（等所有组都退款才 full release）不能处理“部分组已发货 + 部分组退款”，C（group 部分释放）若无 group 归属仍无法判定并发/状态，故一并否决。 |
| **B2** | SYSTEM 缺货 worker 必须传 `source=system_shortage`，release reason `SHORTAGE_REFUND`；当前传 `aftersale` 是错误映射，T1.4b2a 修复。 |
| **B3** | T1.4b2a 上线前的 PRE admission/S19 无 release：**必须一次性回填 + 后续 drift scan**（T1.4b2c）；不得把“无回填”作为可接受 WIP。 |
| **C-04** | 新增 `ReserveRequest.fulfillment_group_no`、`TccPaymentRequest.fulfillment_group_no`（或等价 group 字段）；release event 已有 group。C-04 release event 按用户 2026-11-25 不需单独人类 Approve，但需 QC/Architect。 |
| **C-05** | **需要迁移**：`stock_reservations.fulfillment_group_no`、唯一键 `(order_no,group_no,sku_id,warehouse_id)`、`stock_ledger.group_no`；历史数据回填/拆组。旧“无 C-05”设计作废。 |
| **实现** | **不可进入实现**；先完成本 Addendum accepted + C-05 迁移评审 + T1.4b2-R。 |

---

## 2. B1：为什么必须 group-scoped reservation

### 2.1 事实冲突

- Order 的 group 是**履约拆组**：`order_fulfillment_groups(group_no, warehouse_id)`；`order_items.fulfillment_group_no` + `quantity` 是 group 的真实数量来源。
- Inventory 当前 `stock_reservations` 唯一键是 `(order_no, sku_id, warehouse_id)`，quantity 是**订单总量**；`Release`/`Confirm`/`TccCancelConfirm` 都要求 `quantity == reservation.quantity`（`repo.go:183/252`、`tcc.go:95`）。
- 两个 group 共享同一 `(order_no,sku,warehouse)` 时，单组 release 的 group qty < reservation qty → `INVALID`/DLQ；INV-44 不执行。B1 成立。

### 2.2 方案比较

| 方案 | 结论 | 原因 |
|---|---|---|
| **A. group-scoped reservation** | **采用** | Inventory 直接拥有 group 归属；每组可独立 Release/Confirm/Cancel；并发锁粒度正确；零元/正金额同路径。代价是 C-05 迁移 + 历史拆组。 |
| B. 等所有共享组退款后 full release | **否决** | 部分组已发货时 reservation 已 Confirm/扣减，不能 full release；延迟到“最后组”会把未退组库存一起释放或造成已退组不释放；无法满足 AC-51 的已发货/未发货混合。 |
| C. 新命令 partial/group release，无 group 归属 | **否决** | 没有 group 归属就无法判定某数量属于哪个组、无法处理并发 Confirm/Release、无法审计；最终仍需 A 的 group 归属。 |

### 2.3 目标模型

- `stock_reservations` 增 `fulfillment_group_no`；每个 group × sku × warehouse 一行：
  - `quantity = Σ(order_items where group_no, sku_id, warehouse_id).quantity`
  - 唯一键 `(order_no, fulfillment_group_no, sku_id, warehouse_id)`；删除旧 `(order_no, sku_id)` 唯一键。
- `stock_ledger` 增 `fulfillment_group_no`（审计/对账）；旧行按 G1/可证明 group 回填。
- `inventory_stocks.reserved/on_hand` 仍按 `(sku,warehouse)` 汇总；拆分 reservation 不改变总量。
- Order 是 group 归属事实来源；Inventory 不自行推断 group。

### 2.4 C-04

- `ReserveRequest` 增 `fulfillment_group_no`；`Confirm`/`Release` 使用该字段（空值拒绝，不默认 G1；历史迁移行已显式 G1）。
- `TccPaymentRequest` 增 `fulfillment_group_no`（或 `TccCancelConfirmRequest`）；release event 已有 `group_no`。
- `Order` 的 Reserve/Confirm/Compensation 调用按 `order_items.fulfillment_group_no` 传组；不得再用 order-level line 数量去匹配整单 reservation。
- `inventory.compensation_requested` 旧 order-level cancel 事件：由 consumer 按 order 下所有 group fan-out；若保留旧事件，需补 group 归属/兼容；不得把 group release 复用旧 payload。

### 2.5 C-05 迁移与历史

1. 加 `fulfillment_group_no`（先 nullable/默认 ''）；加 `stock_ledger.group_no`。
2. 旧单组订单（0011 前/G1）：回填 `G1`；reservation 原 quantity 即 group quantity。
3. 已有 `order_items.fulfillment_group_no` 的多组订单：
   - 按 `(order_no,group_no,sku,warehouse)` 汇总 group quantity；
   - 若 Σ group quantity == reservation quantity：拆成多行 group reservation（同 reservation_no 不可复用；生成新 reservation_no）；
   - 若不等/无法证明：标记 `LEGACY_SHARED`/人工，**不回填、不自动 release**；DLQ/告警。
4. 建新唯一键、删旧唯一键；迁移可重入；旧 ledger 行补 group。
5. 历史无法拆分的 group：T2/人工处理，不阻塞新订单，但该组 release fail-closed。

### 2.6 幂等/并发/边界

- **幂等**：`release_id=rr:<refund_id>`；group reservation 行唯一；`Release`/`TccCancelConfirm` 重复 no-op；`Confirm` 重复 no-op。
- **并发**：按 group reservation 行锁；G1 release 不影响 G2；Release vs Confirm 由状态 CAS 唯一胜者；失败方重读/转 POST/人工。
- **零元/正金额**：同一 group reservation、同一 release event/consumer；零元只跳过 Payment。
- **已发货组**：不写 release event；reservation 已 Confirm，AfterSales restock 任务回库；不得 `Release`。
- **历史**：见 §2.5；无法拆分的共享 reservation 不得猜测释放。

---

## 3. B2：SYSTEM 缺货源

| 路径 | AdmitRefund source | release source/reason |
|---|---|---|
| 普通售后 | `aftersale`（空串兼容） | `aftersale` / `AFTER_SALE_REFUND` |
| 零元售后 | `system_zero` | `system_zero` / `ZERO_AFTER_SALE` |
| TCC | `tcc` | `tcc` / `TCC_COMPENSATION` |
| SYSTEM 缺货 | **`system_shortage`** | `system_shortage` / `SHORTAGE_REFUND` |
| `payment`/`legacy_expired` | **拒绝** `REFUND_RELEASE_SOURCE_UNSUPPORTED` | T2 再扩展，不得默认 aftersale |

- 当前 `internal/services/aftersales/workers.go:389-391` 的 SYSTEM 缺货 worker 传 `aftersale`，是 **B2**；T1.4b2a/b 必须改为 `system_shortage`。
- 空 source 仅兼容普通售后；显式 `payment/legacy_expired/unknown` 拒绝，准入 rollback、零 outbox/S19。

---

## 4. B3：上线前 PRE admission/S19 backfill

T1.4b2a 上线前已经产生的 `PRE_SHIP_REVOKED` admission / S19 没有 release event；不回填会永久缺 INV-44。

**方案：一次性回填 + drift scan（T1.4b2c）**

1. 在 C-05 group reservation 迁移/历史拆组后运行。
2. 扫描：
   - `order_refund_admissions.admission_result=PRE_SHIP_REVOKED`（或等价 PRE 终态）且 `refund_applications` 已 DONE/有效；
   - 不存在对应 `inventory.refund_release_requested` 事件（按 `release_id=rr:<refund_id>`）；
   - `source ∈ {tcc,system_zero,system_shortage,aftersale}`；`payment/legacy_expired` 跳过并报告。
3. 为每条生成 release event，`release_id=rr:<refund_id>`；`items` 来自 order group items（group-scoped 后）；group reservation 必须存在；否则人工/DLQ。
4. 幂等：`release_id` + Inventory group operation；`outbox event_id` 使用 `release_id`（或重复事件由 Inventory 幂等，不重复释放）；**不新增 receipt 表**。
5. 跳过 `POST_SHIP_ALLOWED`、已发货组；回库走 AfterSales。
6. 输出 dry-run 计数/差异/跳过原因；一次性回填后保留 periodic drift scan（同一幂等事件）与告警。
7. 回填是 T1.4b2 上线门禁；不得以“旧数据无 release”作为可接受 WIP。

---

## 5. C-04/C-05 影响汇总

| 项 | 变更 |
|---|---|
| C-04 | `ReserveRequest.fulfillment_group_no`；`TccPaymentRequest.fulfillment_group_no`；release event 保持 group_no；旧 `inventory.compensation_requested` 需 group fan-out/兼容，不复用为 group release。 |
| C-05 | `stock_reservations.fulfillment_group_no`、新唯一键、`stock_ledger.fulfillment_group_no`、历史拆组/回填。 |
| 目标 | SPEC-0002 v1.9 INV-44/AC-50/AC-51；ADR Amd1；T1.4b2 design（其 §2/§7 与本 Addendum 冲突处以本 Addendum 为准）。 |
| 人类审批 | C-04 release event 不需单独人类 Approve（用户 2026-11-25）；C-05 迁移仍需正常 QC/Architect/Release 门禁；本 Addendum 需 QC/Architect accepted。 |

---

## 6. T1.4b2-R 任务

| ID | 任务 | Owner | 验收 |
|---|---|---|---|
| **R1** | C-04 group 字段 + C-05 group reservation/ledger 迁移 + 历史拆组 | Architect + Developer | 迁移可重入；单组/多组/不可拆分；旧键删除；无默认 G1（新路径） |
| **R2** | Order Reserve/Confirm/Release 按 group；缺 group fail-closed | Developer | 多组同 SKU/仓；共享 reservation 不再误 release；历史 fail-closed |
| **R3** | Inventory Release/TccCancelConfirm 按 group；group 行锁/幂等/并发 | Developer | 单组独立；Release vs Confirm 唯一胜者；重复 no-op |
| **R4** | SYSTEM 缺货源 `system_shortage`；source 拒绝表 | Developer | 四源映射；payment/legacy/unknown 拒绝；无默认 aftersale |
| **R5** | B3 一次性回填 + drift scan | Developer | dry-run；PRE-only；release_id 幂等；POST/支付级跳过；不新增 receipt |
| **R6** | QC/Architect + Reviewer | QC/Architect/Reviewer | 精确 base/head；测试矩阵；不构成 Release PASS |

---

## 7. 测试矩阵

| 场景 | 期望 |
|---|---|
| 同 SKU/仓两个 group，仅退 G1 | G1 group reservation Release；G2 不受影响；不 DLQ |
| G1 HELD/G2 CONFIRMED | 只 release G1；G2 保持；已发货组走 restock |
| 同 group 多行 quantity-split | group reservation quantity = Σ lines；Release 一次 |
| 并发 G1 Release / G2 Confirm | 各自行锁；互不影响；CAS 唯一胜者 |
| 重复 release event | group reservation no-op；不重复释放 |
| 共享旧 reservation 不可拆 | fail-closed/人工；不自动释放；告警 |
| 历史 G1 | 迁移 G1；Release 正常 |
| source shortage | `system_shortage`/`SHORTAGE_REFUND` |
| source payment/legacy/unknown | `REFUND_RELEASE_SOURCE_UNSUPPORTED`；零 outbox/S19 |
| B3 PRE 无 release | 回填生成 release；重复运行幂等；POST 跳过 |
| B3 回填时 group reservation 缺失 | 人工/DLQ；不猜 G1/W1 |
| 零元/正金额 | 同一 group release 路径；零元仅无 Payment |
| 已发货组 | 不 Release；AfterSales restock |
| order_items 缺 group | fail-closed；不默认 G1 |

---

## 8. 状态与实现门禁

- **Status: superseded / withdrawn**；本 Addendum 不再作为实现依据；T1.4b2 以 design Rev 1.2 为准。
- C-04 group 字段 + C-05 迁移 + B3 回填 + Reviewer 复审完成前，T1.4b2 不能 PASS/Release。
- 本 Addendum 不修改已 accepted 的 C-04 RefundBatch；release event 按用户决定无需单独人类 Approve，但需 QC/Architect。
