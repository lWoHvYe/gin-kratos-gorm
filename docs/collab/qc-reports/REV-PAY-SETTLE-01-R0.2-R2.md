# REV-PAY-SETTLE-01-R0.2-R2 — B1 C-04 release RPC 争议复审

| 项 | 值 |
|---|---|
| review_scope | R0.2 B1 争议：`docs/adr/0007-amd1-addendum-r0.4-c04-release.md` DRAFT vs ADR-0007 Rev 2.3 §2.4/§5.7、C-04/C-05、T1.4a R-A；B2 AC-27 台账、B3 INV-45..50 evidence 仅记录不关闭。只读；不改代码/契约；不批准 merge/Release。 |
| **verdict date** | **2026-11-25**（不将 2026-11-24 的 C-01 原始批准倒签为本轮） |
| R0.1 evidence | code `42a8db5`、evidence `aaef9f9`（C-05） |
| Tier / Actor | **L2**；Reviewer `reviewer-phase1`；与 C-04 Architect/Director、C-05 Developer/QC 不同 |
| **overall verdict** | **FAIL（R0.2-R2 有范围）**：B1 **撤回/降级为 Plan 债务**，C-04 contract **PASS（2026-11-25）**；**B2/B3 仍开**（R0.3/R0.5） |
| release_status | **NOT RELEASE**；不批准合并 |

---

## 1. B1 裁决：C-04 release **不需要 RPC**

**接受 Architect DRAFT 的实质裁决**；但依据是已 accepted 的 **ADR-0007 Rev 2.3**，不是 DRAFT 本身：

| 条款 | 证据 | 结论 |
|---|---|---|
| ADR Rev 2.3 §2.4 `:204` | `inventory 释放请求（组级）` 为**既有事件（C-04 增量字段）**；`refund_id/order_no/group_no/items/warehouse_id`；既有 `inventory.compensation_requested` 只有 `order_no/items`，**C-04 必须补组字段，C-05 无迁移** | event/outbox；无 RPC |
| ADR Rev 2.3 §5.7 `:724` | “C-04 必须扩展组字段（**不新增 RPC**），C-05 无迁移” | 明确 no RPC |
| ADR Rev 2.3 §5.7 `:712-716` | Order 产出 **durable release 请求**（outbox/lease task）；Inventory 执行 `Release`/`TccCancelConfirm`；禁止 DB 事务内远程调用 | owner/执行 API 既有；无新 RPC |
| ADR Rev 2.3 §8 `:940`、`:967` | `release_id=rr:<refund_id>`；W13 Order durable release 请求 → Inventory 组级幂等 | event/outbox；无 RPC |
| ADR Rev 2.3 A-12 `:117` | C-04 API 增量为 `AdmitRefund` 字段、`GetRefundByKey`、历史队列命令、配置；**不含 Inventory release RPC** | C-04 无 release RPC |
| SPEC-0002 v1.10 INV-44 `:129`、§5.7 `:723` | 组退款库存处置 owner/event；零元与正金额同一 release event/consumer；未要求 gRPC | event；无 RPC |
| T1.4a R-A | Order 准入后持久 release 请求；Inventory 执行；不得事务内远程调用 | event/outbox；无 RPC |
| C-04 RefundBatch | 仅 Payment `RefundBatch`；不覆盖 Inventory release；不能把 accepted 当 release RPC | 与 release 解耦 |

- **`tasks.md:21` 的 “C-04 release RPC” 是 Plan 文案**，不是 Spec/ADR；Plan 不得覆盖已 accepted 的 ADR Rev 2.3。R0.4 应改为 **C-04 release event/schema/consumer 验证（no RPC）**；这是 Lead/Plan 债务，不是 Dev 实现缺口。
- **DRAFT Addendum** 本身 **未批准**；本报告不把 DRAFT 当契约。C-04 PASS 依据是 **ADR Rev 2.3 的已 accepted 条款** + 现有 C-04 event/RefundBatch 契约。
- **C-04 contract verdict（2026-11-25）**：**PASS**。`RefundBatch`/`GetRefundByKey` 存在；release 是 event/outbox（`inventory.refund_release_requested` / C-04 组字段）；无 release RPC 需求。**B1 撤回**；仅保留 **Plan 文案修正 [MAJOR/TODO]**。

---

## 2. C-04 / C-05 verdict 更新

| 契约 | 2026-11-25 verdict | 说明 |
|---|---|---|
| **C-01** SPEC-0001 v1.4 | **PASS（contract）** | AC-27 EARS；原始 2026-11-24 批准仅历史；AC-27 台账 B2 仍开 |
| **C-02** SPEC-0002 v1.10 | **PASS（contract）** | K-8；INV-45..50 正文；B3 仍开 |
| **C-03** ADR-0007 Rev 2.3 | **PASS** | accepted 2026-11-25；N5 |
| **C-04** proto/release | **PASS（contract）** | B1 撤回；RefundBatch/GetRefundByKey + release event/outbox；无需 RPC |
| **C-05** migrations | **PASS** | R0.1 `42a8db5`/`aaef9f9` 真实 MySQL 8.4.13 证据充分 |

**R0.2-R2 总体仍 FAIL**：不是 C-04，而是 **B2/B3 台账/evidence 未关闭**；不能因 B1 争议阻塞这两项记录。

---

## 3. B2（仍开，R0.3）

- `docs/collab/ac-coverage.tsv` 37 行 **无 AC-27 行**；`ac-baseline.txt:27` 有 AC-27。
- C-01 v1.4 的 AC-27 只有正文/EARS，无 `test_type/evidence_path/status`；**AC-27 不能 AC-VERIFIED**。
- **R0.3**（Owner=Reviewer）需写覆盖/延期决策；本报告只记录，不在 R0.2-R2 关闭。

## 4. B3（仍开，R0.3/R0.5）

- `docs/lead/PAY-SETTLE-01/evidence-matrix.md` 仍 **v1.7**，无 INV-45..50 行；`ac-coverage.tsv` 是 AC 台账，无 INV 行。
- SPEC-0002 v1.10 已定义 INV-45..50（`:129-134`），但无证据/延期决策；**R0.3/R0.5** 未闭合。
- 本报告只记录；不因 B1 撤回而把 B3 关闭。

---

## 5. 边界 / 声明

- 本报告未修改代码/契约/迁移；未跑新测试/迁移；C-05 采用 R0.1 canonical 证据。
- **B1 撤回/降级**：C-04 contract PASS；`tasks.md:21` 的 “release RPC” 是 Plan 文案，改为 **release event/schema/consumer 验证（no RPC）**；不得据此要求新增 RPC。
- **B2/B3 仍开**：AC-27 台账、INV-45..50 evidence 由 R0.3/R0.5 处理；R0.2-R2 **overall FAIL**。
- **日期 2026-11-25；未倒签 2026-11-24**。C-01 的 2026-11-24 仅作原始批准历史；C-04 的 2026-11-24 仅作 RefundBatch accepted 历史。
- **不是 Release PASS；不批准 merge；不替代人类 merge 决策。**

报告：[REV-PAY-SETTLE-01-R0.2-R2.md](docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.2-R2.md)
