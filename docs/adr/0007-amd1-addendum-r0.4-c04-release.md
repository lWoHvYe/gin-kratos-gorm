# ADR-0007 Amendment 1 Addendum — R0.4 C-04 Release / Schema Ruling

| 项 | 值 |
|---|---|
| Status | **DRAFT 0.3 — pending QC round 3 and human acceptance**；未批准，不构成实现依据，不代表实现 PASS / merge / Release PASS |
| Approved | —（待 QC round 3；人类接受见 R0.4a3） |
| Revision | **0.3-draft（r3）**；修订记录见 §0.3 |
| Tier | **L2**（跨服务事件 / 库存 / 迁移 / 启动门禁） |
| change_id | `PAY-SETTLE-01` |
| Base（仓库） | `e3998d3`（本文件 DRAFT 0.2 r2 落盘；QC round 2 BLOCKED 报告 `QC-PAY-SETTLE-01-R0.4-ADDENDUM-R2.md` 以此为被审对象） |
| Base（契约基线） | ADR-0007 **Rev 2.3**（accepted 2026-11-25，`docs/adr/0007-payment-settlement-admission.md`）+ C-04 RefundBatch（accepted 2026-11-24）+ **SPEC-0001 v1.4**（approved 2026-11-24）+ **SPEC-0002 v1.10**（approved 2026-11-25） |
| Spec-Ref | **SPEC-0002 v1.10**：INV-44（`0002:128`）、INV-45（`:129`）、INV-46（`:130`）、INV-49（`:133`）、INV-50（`:134`）、AC-50、AC-62（`:308`）、AC-65（`:371`）、AC-66（`:386`）；**SPEC-0001 v1.4**：AC-27（`0001:571-579`；**属于 SPEC-0001**）。**v1.5 待生效声明**：Director 2026-11-26 已授权 PO 出 **SPEC-0001 v1.5**（`docs/review-tracking.md:350-356`，解决 AC-27 与 §6.6.1/AC-25 冲突、§7.5 `:405`/`:409` 漂移、AC-50 偏差），**v1.5 仍须 QC delta 复核**。在 v1.5 经 QC 并生效前，本文件对 AC-27 的文本引用仍是 v1.4；**v1.5 生效后以 v1.5 为准**（本文件不预判 v1.5 结论，也不因此改变任何 AC 状态） |
| AC 状态声明 | AC-27 目前是 **AC-REF，不是 AC-VERIFIED**（`docs/collab/ac-coverage.tsv:41`）；本文件不改变任何 AC 状态；v1.5 待 QC 不构成 AC 关闭 |
| allowed_paths（本文件） | 仅 `docs/adr/0007-amd1-addendum-r0.4-c04-release.md` |
| allowed_paths（本裁决约束的 R0.4 实现，供 Lead Plan 引用） | `internal/services/fulfillment/`、`cmd/fulfillment/`、`internal/platform/config/`、`migrations/order/`、`migrations/fulfillment/`、`scripts/verify-r01-migrations.sh`（仅 R0.4d2）、`docs/ops.md`（仅 R0.4d1 模板/校验段）、对应 `_test.go`。**不含** `internal/services/order/`（24h writer 属 T2，见 §2.3.2）、**不含** `internal/services/inventory/`（missing-reservation metric 已移出 R0.4，见 §2.7） |
| forbidden_paths | `docs/specs/**`、`docs/collab/**`、`docs/lead/**`、`docs/implementation/**`、`api/**`、`deploy/**`（R0.4d1 单独 L2 变更，不在本裁决的 allowed_paths 内）、`internal/services/order/**`、`internal/services/inventory/**`（本 R0.4 范围） |
| baseline_impact | **涉及**：INV-44（release event/schema/consumer，P0 单组）、INV-45（release 只释放库存预留，不改承诺额度状态）、INV-46（历史队列 24h CHECK 由下限改等值）、INV-47（不改行级/运费语义，仅 0022 约束不改变其判定）、INV-49（shadow 启动硬拒绝）、INV-50（零元与正金额同一 release 路径）。C-04 `RefundBatch` 不变；不涉及 Payment/AfterSales 状态机；**INV-44 的可观测性缺口（多组 metric / missing-reservation metric）在本 r3 明确移出 R0.4 范围并登记延期，见 §2.7** |
| Actor | `arch-r04-r3`（System Architect） |
| Reviewed-by | **待 QC round 3**（本文件不代签）；实现复核见 R0.4e |
| Integration owner | System Architect（契约）；实现 owner = Developer（R0.4b-impl / R0.4c-impl / R0.4d1 / R0.4d2）、ops（重放/告警/批准行写入） |
| Scope | 只裁决 R0.4：① C-04 release event v1 契约（no RPC）与 P0 单组边界；② Fulfillment 启动门禁（含 shadow 运行态、legacy bool、range、到期语义、退出语义）；③ `order/0022`（或 `0023`）的 `group_no` 判别与 24h SLA 约束，含**编辑前 preflight 决策门**；④ 迁移 apply/重跑/证据口径；⑤ R0.4 **不交付**项的显式延期登记 |
| 硬边界 | 不写代码/迁移/tests/台账/SPEC；不改已 accepted ADR Rev 2.3（errata 由 Architect 在 Rev 2.4 另做）；不新增 subject/RPC/持久化字段；不把 T2/T3/T4.4 业务塞进 R0.4；**不扩大 R0.4 的 allowed_paths 来"顺手"满足验收**（QC N-2） |

---

## 0. 阅读方式与规范边界

### 0.1 本文件的效力边界

1. 本文件是**设计约束**，不是迁移文件、不是代码、不是测试证据（QC T-2）。文中 SQL 是 **schema 约束设计**，不是交付的迁移文件。
2. 任何“MUST/裁决”都不免除 QC round 3、Reviewer 与人类接受（`AGENTS.md` §2.3/§2.7）。本文件不批准 merge/Release，也不把任何 AC 标为 AC-VERIFIED。
3. 设计结论只活在 ADR：`docs/collab/CONVENTIONS.md:11`。**已 withdrawn 的 `0007-amd1-addendum-t14b2-inv44-group.md` 与 `docs/lead/PAY-SETTLE-01/t1.4b2-design.md` 不是 C-04 release 的契约来源**（ADR Rev 2.3 `:19` 与 `:1607-1608` 已标记 withdrawn / not merged；Lead 文档不是契约）。本文件不再引用它们作为契约（QC B-2、T-7）。
4. WIP waiver：`docs/review-tracking.md:187-192` 规定 PAY-SETTLE-01 WIP 期间 ADR/SPEC Amendment **免单独 human approve**，但 QC/Architect 复核、merge、Release PASS、AC-VERIFIED **不豁免**；waiver 到期 = P0 closure/merge decision 或 2026-12-31（取早）。本 Addendum 的**人类接受**（R0.4a3）不属于该豁免，仍须记录。
5. 本文件**不写台账**：INV-44 evidence 行、延期行、AC 状态一律由对应 owner 写 canonical 产物（`docs/lead/PAY-SETTLE-01/evidence-matrix.md`、`docs/review-tracking.md`、`docs/collab/ac-coverage.tsv`）。本文件只给要求与验收口径（QC N-8）。

### 0.2 与 QC 报告的关系

- **输入 1**：`docs/collab/qc-reports/QC-PAY-SETTLE-01-R0.4-ADDENDUM.md`（round 1：3 BLOCKER + 12 MAJOR + 7 TODO）。
- **输入 2**：`docs/collab/qc-reports/QC-PAY-SETTLE-01-R0.4-ADDENDUM-R2.md`（round 2：**BLOCKED**，2 BLOCKER `N-1`/`N-2` + 7 MAJOR `N-3..N-9` + 11 TODO `T-1..T-11`；B-2/B-3/M-4/M-5/M-7/M-9/M-11 仍 PARTIAL）。
- 本 r3 是**按 round 2 BLOCKED 的修订稿**，不是对 QC 结论的申诉。逐条处置见 §4；每条给出落点、owner、状态（处置完成 / DEFERRED / ROUTED）。本文件**不会**把“已改文档”表述为“已通过 QC”；round 3 仍有否决权。

### 0.3 修订记录

| 版本 | 日期 | 内容 |
|---|---|---|
| 0.1-draft（r1） | 2026-11-25 | 初稿：C-04 release no-RPC、启动门禁、`group_no`/24h CHECK。QC round 1 判 **BLOCKED**。 |
| 0.2-draft（r2） | 2026-11-25 | 按 QC round 1 修订：B-1 shadow 运行态裁决（§2.1.1）、B-2 release event v1 契约（§1.2-§1.4）、B-3 24h 等值 CHECK 与 writer/迁移口径（§2.3）；M-1…M-12（§4）。QC round 2 判 **BLOCKED**（2 BLOCKER + 7 MAJOR）。 |
| **0.3-draft（r3）** | **2026-11-26** | 按 QC round 2 修订：**N-1** 把 0022/0023 可编辑性决策门前移到 `R0.4c-impl` 任何编辑之前，含命令/证据/停止条件（§2.3.3）；**N-2** 把多组与 missing-reservation metric 移出 R0.4 并登记 owner/expiry，同时把证据挂回正确任务（§1.4/§1.5/§2.7）；**N-3** 任务 ID 改为 `R0.4b-impl`/`R0.4c-impl` 并与 Lead Plan 的 `R0.4b/c`（Reviewer 复审）分离（§2.6）；**N-4** `0021` 进 fresh-apply 与验证脚本/对象断言（§2.4）；**N-5** `release_range <> ''` 选定唯一落点（ops 模板 + `docs/ops.md` 校验）（§2.1.3）；**N-6** 收窄退出日志 MUST 并给 config 稳定 reason token（§2.1.4）；**N-7** T2 24h release source 定为 T2 前置契约增量（§2.3.2）；**N-8** INV-44 evidence 行交 Lead/Reviewer，Addendum 不写台账（§2.4/§4）；**N-9** 24h expiry/compensation worker 显式归属（§2.3.2）。补 M-4/M-5/M-7/M-9/M-11 的可执行落点；补 §5 反模式自省与 §7 合规自检。状态仍为 DRAFT 0.3。 |

---

## 1. C-04 release：event/outbox，不新增 RPC

### 1.1 裁决

**C-04 release 不新增 RPC。** release 契约是 **Order 产出的 durable release event（outbox）→ Inventory consumer 执行既有 `Release`/`TccCancelConfirm`**。

- Reviewer 的 B1（“C-04 release RPC 缺”）**已于 2026-11-25 由 `REV-PAY-SETTLE-01-R0.2-R2` 撤回/降级为 Plan 文案债务，C-04 contract = PASS**（`REV-PAY-SETTLE-01-R0.2-R2.md:9`、`:31`、`:42`、`:66`）。
- 本 Addendum **不要求 Reviewer 再次撤回任何条目**，也不把 `tasks.md:21` 的旧 Plan 文案称为“越界”；该文案已由 Lead 改为 “C-04 release event/schema/consumer 验证（无 RPC）”（`docs/lead/PAY-SETTLE-01/tasks.md:21`）。R0.4a 只固化契约，不再重开 C-04（QC M-1）。
- 任何未来新增 release RPC 都是**新契约**，须另立 ADR/SPEC + 人类批准，不得在 R0.4 夹带。

### 1.2 契约来源与 additive amendment（QC B-2）

**权威来源**（accepted ADR Rev 2.3）：

| 条款 | 既有裁决 |
|---|---|
| §1.2 `:204` | `inventory 释放请求（组级）` 为既有事件（C-04 增量字段），Order → Inventory；`refund_id, order_no, group_no, items[], warehouse_id`（+ `source`, `reason`）；既有 `inventory.compensation_requested` 不足以按组释放 |
| §5.7 `:712-716` | 触发释放 = Order 在准入事务后产生 **durable release 请求（outbox/lease task）**，禁止事务内远程调用；执行 = Inventory |
| §5.7 `:723-724` | 零元与正金额走**同一** release event/consumer；C-04 扩展组字段、**不新增 RPC**、C-05 无迁移 |
| §8 `:940` | 幂等键 `release_id=rr:<refund_id>`；Order 产出、Inventory 执行 |
| §9 W13 `:967` | 恢复窗口：release 请求未送达/失败 → Order producer + Inventory 组级幂等；人工入口 `refund_id`/`group_no` |
| SPEC-0002 INV-44 `:128` | 未离仓组预留由 Inventory 释放；失败重试告警；释放后守恒 |

**本 Addendum 是对 C-04 release event v1 的 additive amendment**（只增必填安全字段与消费约束；不改既有字段语义；对 `warehouse_id` 位置只做**澄清**而非收窄；不改 subject 版本号；不新增 RPC）：

1. 明确 subject / event_type：`shop.inventory.refund_release_requested.v1` / `inventory.refund_release_requested`（代码事实 `internal/platform/events/events.go:42-43`；ADR 只写“组级释放请求”未命名 subject）。
2. 增加并定为必填：`schema_version`（=1）、`release_scope`（=`order_reservation_full`）、`group_count`（P0 必填=1）、`requested_at`（RFC3339）。
3. 增加可选审计字段：`covered_group_nos`（存在时必须恰为单组且等于 `group_no`）。
4. **澄清** accepted ADR `:204` 的 `items[]`/`warehouse_id` 歧义：`warehouse_id` 属于 `items[]` 且每项必填；v1 **不设** top-level `warehouse_id`（代码无该顶层字段；全单释放跨仓时 top-level 语义不唯一，不得缺省 W1）。这不是“收窄 accepted ADR 字段表”，而是消除文本歧义（QC R2 T-5）。
5. 明确 consumer 的 fail-closed 字段/消费约束（§1.3 字段表 + §1.4 消费契约）与多组订单不释放的显式缺口（§1.5）。

**为什么必须 amend 而不是“照代码”**：accepted ADR `:204`/`:724` 的字段表**没有** `group_count`/`release_scope`/`covered_group_nos`，而 consumer 把它们当必填安全事实（`internal/services/inventory/refund_release_consumer.go:65-70`；`internal/platform/events/registry.go:320-323` 列出 11 个必填字段、`validateRefundRelease` `:225-302` 校验枚举/派生键/items）。若不在规范文本中固化，契约就停留在 code-only，下一位实现者可以合法地删字段或用 `group_no` 代替，直接触发全量 DLQ 或误释放。

**批准路径（QC R2 T-11 顺序澄清）**：

1. 本 Addendum 交 **QC round 3**；
2. **R0.4a3 = 人类接受设计**（记录于 `docs/review-tracking.md`；WIP waiver 不免此步）；
3. 进入实现（`R0.4b-impl` / `R0.4c-impl` / `R0.4d1` / `R0.4d2`）；
4. **R0.4e = Reviewer 对实现的日期化 verdict**，仍可 FAIL 并冻结本 `change_id`；
5. 接受后由 **Architect** 在 ADR Rev 2.4 把本 §1.2-§1.4 与 §2 的运行态裁决回写进 accepted 正文（owner=Architect，不在本文件内完成）。

- `R0.4b/c` 在 Lead Plan 中是 **Reviewer 复审 `90b502c`**，属独立的实现复审，**不替代** R0.4a3 人类接受，也不能把“设计被接受”当作“实现 PASS”。三者语义互不替代（宪法 Workflow 3）。

**关于 QC B-2 证据的一处校正**：QC 称 `registry.go:243-245` 的 validator “只校验 `release_id`”，与代码不符——`:243-245` 是 `release_id` 派生校验；同函数还校验 `schema_version`/`release_scope`/`group_count`/`order_no`/`group_no`/`covered_group_nos`/`source-reason`/`requested_at`/`items`（`registry.go:225-302`），schema 必填表见 `:320-323`。真实缺口是**这些字段不在 accepted ADR 文本**，不是“validator 只校验一个字段”。

### 1.3 Release event v1 字段表（本 Addendum 定义）

| 字段 | 必填 | 类型 / 语义 | 非法时 |
|---|---|---|---|
| `schema_version` | 是 | 整数，只接受 `1` | 永久失败 → DLQ |
| `release_scope` | 是 | 枚举，只接受 `order_reservation_full` | 永久失败 → DLQ |
| `group_count` | 是 | 正整数；**P0 只接受 1** | `!= 1` → 永久失败 → DLQ，**不释放任何预留** |
| `release_id` | 是 | 必须等于 `rr:<refund_id>` | 不匹配 → 永久失败 → DLQ |
| `refund_id` | 是 | 触发本次释放的准入/退款身份 | 空 → 永久失败 → DLQ |
| `order_no` | 是 | 订单号；又是 outbox aggregate_id（每单最多一条 release） | 空 → 永久失败 → DLQ |
| `group_no` | 是 | 触发组；审计用途，不标识 Inventory 预留行 | 空 → 永久失败 → DLQ |
| `covered_group_nos` | 否 | 被本次全单释放覆盖的组；出现时长度必须=1 且等于 `group_no` | 违约 → 永久失败 → DLQ |
| `source` | 是 | 封闭枚举 `tcc`/`system_zero`/`system_shortage`/`aftersale` | 其他 → 永久失败 → DLQ |
| `reason` | 是 | 与 `source` 唯一绑定：`TCC_COMPENSATION`/`ZERO_AFTER_SALE`/`SHORTAGE_REFUND`/`AFTER_SALE_REFUND` | 不匹配 → 永久失败 → DLQ |
| `items[]` | 是 | 非空；每项 `sku_id > 0`、`quantity > 0`、`warehouse_id > 0`；`(sku_id, warehouse_id)` 唯一 | 违约 → 永久失败 → DLQ，**不得部分释放** |
| `requested_at` | 是 | RFC3339 时间戳（审计） | 解析失败 → 永久失败 → DLQ |

- 上表是**完整必填集**（`registry.go:320-323` 的 11 个顶层字段 + item 级约束）；registry 已按此 fail-closed，任何新增字段必须先改本表再改代码。
- **`group_count` 是 registry + consumer 联合 fail-closed**（QC R2 T-4）：registry 只校验 `group_count > 0`（`registry.go:236-238`），`!= 1` 由 consumer 强制永久失败（`refund_release_consumer.go:67-70`）；`covered_group_nos` 的“长度=1 且等于 group_no”由 consumer 校验（`:71-85`）。任何一处被删都会扩大可释放面。
- `quantity` 是审计输入：全单释放以 Inventory 预留行的权威数量为准（`CONFIRMED` 时用预留数量，不用事件数量），不得用事件数量制造部分释放。

### 1.4 Consumer / producer / DLQ 契约（QC M-7）

- **producer**：`outbox.Append` 与准入写入**同一事务**（`internal/services/order/refund_admission.go:283-287`；零元同路径 `:508-514`）。禁止事务内远程调用 Inventory。
- **subject / DLQ**：`shop.inventory.refund_release_requested.v1`；DLQ = 同 subject 的 `<subject>.dlq`（consumer 默认 `MaxDeliver=20`，`internal/platform/consumer/consumer.go:69-70`；DLQ 口径 `docs/ops.md:265-275`）。
- **producer 重试**：outbox relay 20 次后 dead-letter（`internal/platform/outbox/outbox.go:60-61`、`:264-283`），死信先复制到 `outbox_dead_letters`（`docs/ops.md:261-266`）。
- **重放 owner = ops**（ADR W13 `:967`），步骤 = `docs/ops.md:265-275`；重放前必须确认业务态与 handler 幂等。不得由 Developer 写脚本自动重放。
- **缺失 reservation 行（代码事实）**：是幂等 no-op（`refund_release_consumer.go:92-96`；`internal/services/inventory/service.go:171-186`），当前**无任何计数/日志/告警**——只有 `continue`。
  - **本 r3 裁决（QC N-2）**：`internal/services/inventory/**` 不在 R0.4 的 `allowed_paths`，因此**“新增 metric + 告警”不再是 R0.4 交付项**，改为显式延期（owner=Developer，expiry=2026-12-31，tracking=`review-tracking/PAY-SETTLE-01`，reason=`missing-reservation release is silent; no metric/alert`，status=DEFERRED），登记见 §2.7。
  - **不得**声称该路径“可观测性问题已解决”。在延期关闭前，已知保护只有：`RELEASED` no-op 幂等、consumer 重投/DLQ、上游 `refund_applications` 与 reservation 行的事实存在性；**“缺行静默成功”仍是已登记风险**。
  - 若人类/Reviewer 要求它在 R0.4 关闭，唯一合法路径是**人类扩大 `allowed_paths` 并改 Lead Plan**，不得由 Developer 越界实现（§2.7 停止条件）。
- **重复/乱序**：`RELEASED` no-op；同一订单只发一次 release（`refund_release.go:187-193`）；重复投递只收敛状态，不二次释放。
- **多组订单无 release**：见 §1.5，metric 同属延期项，不得只打一行 `log.Warn` 就被当作“已可观测”。

### 1.5 P0 单组边界与多组缺口（QC B-2 第 2 点 / N-2）

- **P0 路线**：只有单组订单（`group_count=1`）产生 release event；这是 R0.4 的合法单组范围，也是 INV-44 在 P0 的可验证范围。
- **多组订单现状（代码事实）**：`appendRefundReleaseTx` 在 `groupCount != 1` 时直接 `return nil`，只 `log.Warn("multi-group refund keeps inventory reservations (P1/manual)")`（`internal/services/order/refund_release.go:182-186`）：**不发 outbox、不重试、无人工队列**。这是**显式的“不释放”**，不是“释放成功”，也不是可接受的静默成功。
- **裁决**：
  1. 多组不释放作为 **P1/manual 已知缺口**显式登记，不得声称 INV-44 已完成、不得声称 AC-27 的释放腿在 2 组订单上已验证。
  2. 登记字段（由 owner 在 canonical 台账写，不在本文件）：`owner=Developer`、`expiry=2026-12-31`、`tracking=review-tracking/PAY-SETTLE-01`、`reason=multi-group INV-44 release not emitted; P1/manual`、`status=DEFERRED`。
  3. **可观测性（metric + 告警）由 `internal/services/order/` 实现，走独立变更；R0.4 不交付**（QC N-2）。原 r2 的“R0.4b/c PASS 条件包含该 metric 测试”**撤回**：该句把验收挂在错误任务/错误 `allowed_paths` 上，且会迫使 Developer 越界改 Order。metric 的 owner/expiry/停止条件见 §2.7。**R0.4b-impl 的 PASS 只覆盖 Fulfillment 启动门禁**，不覆盖 Order 侧 metric。
  4. `docs/lead/PAY-SETTLE-01/evidence-matrix.md` 必须新增 **INV-44 行**（现只有 INV-45…50，QC M-9/N-8）；owner = Lead，本 Addendum **不写该行**（§2.4 给出该行必须具备的字段口径，R0.4e 由 Reviewer 核对）。
- 该缺口**不阻塞** R0.4 的代码正确性，但**阻塞**“INV-44 已完成 / 多组释放已验证 / INV-44 已有证据”的声明。

---

## 2. R0.4 设计

### 2.1 Fulfillment 启动门禁

在**注册任何 consumer/worker 之前**，按 ADR Rev 2.3 §6.2（`:821-836`）只读校验 `settlement_switch_approvals`；应用无写权限。矩阵如下（含 legacy bool，QC M-8）：

| 请求 | 条件 | DB/配置校验 | 失败语义 |
|---|---|---|---|
| `paid`（显式或默认） | `FULFILLMENT_SETTLE_CONSUMER_MODE` 未设置/空/`paid`，且 legacy 未设/`false` | `SETTLE_RELEASE_APPROVAL_ID` MUST 为空（N5，ADR `:827-828`）；**不读批准行** | 非零退出；不得以 `paid` 之外的模式继续 |
| `shadow` | mode=`shadow` | ① approval id MUST 为空（N5）；② `SETTLE_RELEASE_RANGE` 非空；③ 存在 `target_mode='shadow'`、`status='APPROVED'`、`expires_at IS NULL OR > DB NOW(3)`、`release_range` 与部署 range **精确相等**的行 | 非零退出（reason=门禁原因）；**即使 ①②③ 全过，仍按 §2.1.1 拒绝启动** |
| `settled` | mode=`settled` | ① approval id 非空；② range 非空；③ 按 approval_id 取行：`target_mode='settled'`、`status='APPROVED'`、`expires_at IS NOT NULL AND > DB NOW(3)`、range 精确相等 | 非零退出；不得回退 `paid` |
| legacy `true`（新 mode 未设） | `FULFILLMENT_CONSUME_ORDER_SETTLED=true` | **视为请求 `settled`**：必须同时有 approval id + 上述 settled 行校验（ADR `:834`）；legacy bool **永远不能选 shadow** | 缺/空/过期/非 APPROVED/range 不匹配/冲突 → 非零退出 |
| legacy `false` | `=false` 或未设置 | 未请求模式 → `paid` 默认 | 不得把 `false` 当作 `shadow`/`settled` |
| 新旧冲突 | mode 与 legacy 映射不一致 | fail-fast（`config.go:754-761`） | 非零退出，不静默择一 |
| 未知/大小写不符/空且 legacy 未设 | — | 不默认 `paid`（`config.go:749-752`） | 非零退出 |

- paid 是**唯一**允许的默认路径；`SETTLE_RELEASE_APPROVAL_ID` 不得单独生效（N5，ADR `:828`）。
- `release_range` 不得为空串：空串不是通配符（`config.go:775`；`settlement_switch.go:95,147-148`）。
- 读取表/连接失败 → 启动失败，不得猜 `paid`。
- 多行匹配（QC T-4）：取 `approved_at DESC` 下**第一条全部条件满足**的行（代码 `settlement_switch.go:121-126`），审计记录该 `approval_id`；不得“任取一行”。shadow 行读取按 `approved_at DESC, approval_id ASC`（`:126`）。

#### 2.1.1 shadow 运行态裁决（QC B-1；Director 2026-11-25）

> **裁决来源**：`docs/review-tracking.md:358-366`——Director 2026-11-25 决定 `FULFILLMENT_SETTLE_CONSUMER_MODE=shadow` **hard-rejected at startup until T4.4**；Addendum 必须记录运行态裁决、fail-closed 退出语义与 T4.4 收口条件；默认保持 `paid`。

**规范条款**：

- **WHEN** 启动请求 `FULFILLMENT_SETTLE_CONSUMER_MODE=shadow`，**THE SYSTEM SHALL** 在注册任何 consumer/worker 之前**拒绝启动**；**无论 §2.1 的批准行门禁是否通过**，都不得进入 shadow 运行态、不得回退 `paid`、不得注册 `order.paid` consumer 顶替。
- **IF** 门禁未通过，**THEN** 以门禁原因拒绝（reason = 具体门禁码，§2.1.4）；**IF** 门禁通过，**THEN** 以稳定 reason **`T4_4_SHADOW_NOT_DELIVERED`** 拒绝。
- 退出必须非零；拒绝日志 MUST 含 `mode`、`approval_id`（门禁解析出的行 id；门禁未解析则为空串）与 `reason`。
- **owner**：Developer（T4.4 observe-only 双消费交付）+ Architect（契约/收口条件）；ops 在本裁决解除前 **MUST NOT** 写入 shadow 批准行——写了也不可执行，只会制造“已批准但起不来”的审计噪音。
- **T4.4 收口条件（全部满足才可解除硬拒绝）**：① observe-only 双消费（`order.paid`+`order.settled`，只记录/比对、不建单、不推进组）交付；② Reviewer 对 T4.4 的精确 base/head 出 PASS 证据；③ Director **重新批准** shadow 运行；④ 由 Architect 以 ADR Amendment（Rev 2.4 或新 Addendum）显式删除本硬拒绝条款。**在此之前，任何代码路径不得删除/弱化该拒绝**（删除即违约，Reviewer 应按 `[ARCHITECTURAL_FLAW]` 打回）。
- **默认 `paid` 不变**；本裁决不放行 `90b502c`，也不豁免 Reviewer/人类接受/Release/AC-VERIFIED。
- **ADR 冲突登记**：accepted ADR §6.2 `:822` 与 §6.4 步骤 1 `:856` 仍把 shadow 写成“可启动/可执行”，与本次 Director 裁决及实现（`cmd/fulfillment/main.go:63-70`）冲突。本 Addendum 在 **R0.4/PAY-SETTLE-01 WIP 范围内**以本条款为运行态契约；Architect 必须在接受后于 **Rev 2.4** 加 errata（owner=Architect，接受后执行），否则 accepted ADR 与本文件长期并存矛盾。
- **门禁错误优先于 T4.4 拒绝**：先跑 §2.1 门禁（只读）；门禁失败报门禁原因，门禁通过才报 `T4_4_SHADOW_NOT_DELIVERED`。两种路径都拒绝启动、都非零退出（exit 3 / exit 4，§2.1.4）。

#### 2.1.2 批准行到期语义（QC M-6）

- 到期是**启动窗口语义**：运行中的进程**不会**因 `expires_at` 到期自动停止；运行期不轮询、不热切换（ADR `:836`）。
- 唯一停止路径 = ADR §6.4 的 **Director 批准回滚 + 重启**（`:864-867`）；不得由应用自动改模式。
- `settlement_switch_approvals.status='EXPIRED'` **不由应用写入**（应用只读，`:832`），因此该枚举值在 R0.4 是**不可达状态**。设计裁决：**不新增写者**；看板/告警 MUST 以 `expires_at <= NOW(3)` 判定过期，**不得依赖 `status`**（否则过期行仍显示 `APPROVED`，`docs/review-tracking.md` 式的“有效”误判会复发）。
- **ops 必须配置告警（可验收）**：`target_mode='settled' AND status='APPROVED' AND expires_at <= NOW(3)`；owner=ops，expiry=2026-12-31，tracking=`review-tracking/PAY-SETTLE-01`。验收 = 告警规则写入 ops runbook + 一条真实触发/演练记录（查询输出或告警截图指针），**不接受**“已口头配置”。过期仍运行的实例由 Director 决定回滚。
- 若业务后续要求 `status='EXPIRED'` 可达，另立受控 ops 迁移/任务（owner=ops，expiry=2026-12-31），不得留给“以后”；在此之前它不是 R0.4 交付。

#### 2.1.3 shadow 的 `release_range`（QC M-5 / N-5：唯一落点）

- **裁决：shadow 也要求 `release_range` 非空且与部署 range 精确相等。** 该裁决消除 accepted ADR 内部冲突：§6.2 第 4 条（`:831`）只要求存在 `target_mode='shadow' AND status='APPROVED'` 的行，而 §6.3（`:848`）与 §12.5.6（`:1285`）要求 range 校验；代码三处一致要求非空 + 精确匹配（`config.go:775`；`settlement_switch.go:95,147-148`）。
- 后果：shadow 有**两个独立不可启动原因**——range 缺失/不匹配（门禁拒绝，exit 3）与 T4.4 未交付（运行态拒绝，exit 4）；两者都必须 fail-closed，不得互相掩盖。
- **`release_range <> ''` 的唯一落点（本 r3 决定）**：**不走新增 fulfillment 迁移 CHECK，改由 ops 写入模板 + `docs/ops.md` 校验保证（R0.4d1）**。理由：
  1. 批准行是**人工/受控写入的审计行**，`release_range` 列默认 `''`（`migrations/fulfillment/0010_settlement_shadow_switch.sql:42`）。若新增 `0012` CHECK，存量空 range 行会让迁移直接失败；若先 UPDATE 审计行去满足约束，则违反“审计行不可改写”。
  2. 运行期已 fail-closed：shadow/settled 的部署 range 必须非空（`config.go:775`）且与行**精确相等**（`settlement_switch.go:95,147-148`），因此 `release_range=''` 的行**永远无法通过启动门禁**——它是惰性行，不是放行路径。
  3. R0.4 不新增 fulfillment 迁移，避免扩大 fresh-apply 顺序与迁移证据面（§2.4 的 `0010→0011` 保持不变）。
- **可执行验收（R0.4d1；owner=Developer；expiry=2026-12-31）**：
  1. `docs/ops.md` 必须给出 shadow/settled 批准行写入模板：`release_range` 必填、示例值、`<> ''` 校验 SQL；
  2. 校验 SQL（示例）：`SELECT approval_id FROM settlement_switch_approvals WHERE status='APPROVED' AND release_range='';` **结果必须 0 行**；发现非 0 行 → 由 Director/ops **重新签发**批准行（不得 UPDATE 审计行），记录事件单；
  3. R0.4d1 证据 = 模板段落指针 + 样例行 + 校验 SQL 的 0 行输出；缺任一即 R0.4d1 不 PASS。
- **停止条件**：若 Director 要求 DB 级强制，另立 additive `migrations/fulfillment/0012`（owner=Architect+Developer，expiry=2026-12-31），必须同时更新 §2.4 fresh-apply 顺序、`scripts/verify-r01-migrations.sh` 的 `MIGRATION_FILES`/对象断言，**并先证明存量无非空 range 违规行**；不得在 R0.4 内“顺手加”，也不得由 Developer 自行选择方案。
- Architect 在 Rev 2.4 修 §6.2 第 4 条 errata（owner=Architect）。

#### 2.1.4 退出码与审计字段（QC M-11 / N-6）

稳定契约（`R0.4b-impl` 验收项，Developer）：

| 退出码 | 含义 | 当前实现 |
|---|---|---|
| `1` | 基础设施失败（open DB / connect NATS / dial Order / tracing / run） | 已如此（`cmd/fulfillment/main.go:30,38,74,87,110`） |
| `2` | config / N5 / 未知 mode / range 缺失（`config.Load` 返回错误 → panic → Go exit 2） | 已如此（`config.go:749-776`；`main.go:22-24` 是 panic） |
| `3` | 批准行门禁拒绝（缺行/过期/非 APPROVED/range 不匹配/DB 不可读） | **当前是 1**（`main.go:48-57`），`R0.4b-impl` MUST 改为 3 |
| `4` | shadow 运行态拒绝（`T4_4_SHADOW_NOT_DELIVERED`） | **当前是 1**，且日志缺 `approval_id/reason`（`main.go:63-70`），`R0.4b-impl` MUST 补齐 |

- 稳定 reason 码（日志字段 `reason`）：`N5_APPROVAL_ID_MODE_MISMATCH`、`MODE_UNKNOWN`、`RELEASE_RANGE_MISSING`、`APPROVAL_ROW_MISSING`、`APPROVAL_NOT_APPROVED`、`APPROVAL_EXPIRED`、`RELEASE_RANGE_MISMATCH`、`APPROVAL_DB_UNAVAILABLE`、`T4_4_SHADOW_NOT_DELIVERED`。
- **日志三字段的适用范围（本 r3 收窄，QC N-6 二选一已定）**：
  - **exit 3 / exit 4（门禁 / shadow 拒绝）**：拒绝日志 MUST 在注册 consumer 前输出，字段至少 `mode`/`approval_id`/`reason`。理由：K8s CrashLoopBackOff 必须能区分“门禁拒绝”与“基础设施失败”。
  - **exit 2（config / N5 / 未知 mode / range 缺失）**：**保留 panic 语义**（宪法 Article III.4 允许 `cmd/*` 启动期 panic），**不要求** logger 三字段——因为 `config.Load` 失败时返回 `nil`（`config.go:227-229` 等处均为 `return nil, err`），在 config 有效前构造结构化 logger 会把“配置错误”变成“日志初始化错误”。替代要求：**错误文本 MUST 含稳定 reason token**，且该 token 由 `R0.4b-impl` 加入 config 错误文本：
    - N5 → `N5_APPROVAL_ID_MODE_MISMATCH`（现文本 `config.go:766`）
    - 未知/大小写不符 mode → `MODE_UNKNOWN`（现文本 `:752`）
    - range 缺失 → `RELEASE_RANGE_MISSING`（现文本 `:775`）
    - 新旧冲突 → 保留 `config.go:760` 文本（归入 exit 2，不新增 token）
  - 因此本文件**不再写“所有拒绝日志三字段”**；准确表述是“门禁/shadow 拒绝日志三字段 + config 拒绝 exit 2 且带稳定 reason token”。（原 §2.1.4 的“已如此（config 层）”与“所有拒绝日志”矛盾，随本修订删除。）
- `R0.4b-impl` 证据：`cmd/fulfillment/main_test.go`（子进程真实退出码 + stderr 断言）与 `internal/services/fulfillment/settlement_switch_test.go`（门禁分支）。具体断言：
  - exit 3：缺行（现断言 `code==1` 必须改 3）、过期、非 APPROVED、range 不匹配、DB 不可读；
  - exit 4：门禁通过后的 shadow（现 `TestStartupRefusesShadowWithoutConsumerImplementation`）；
  - exit 2：N5/未知 mode/range 缺失 + stderr 含对应 token（现 `TestStartupRejectsApprovalIDOutsideSettled` 只断言 `!=0`，必须收紧为 `==2` 且含 `N5_APPROVAL_ID_MODE_MISMATCH`）；
  - 三字段：exit 3/4 的 stderr 必须同时含 `mode=`、`approval_id=`、`reason=`（当前 `main.go:51-53` 已有该结构，但退出码/字段覆盖不全）。
- 注：`main_test.go` 需要 `TEST_MYSQL_DSN`（否则 `t.Skip`，`main_test.go:35-37`）；**Skip 不算证据**。真实执行日志须绑定 HEAD，缺真实 DB 日志即 `[不确定]`。

### 2.2 `order/0022`：`group_no` 判别 CHECK（QC M-12）

新增 `migrations/order/0022_r04_checks.sql`（Order DB；不改已合入的 0017/0018/0019/0020/0021）；**若 §2.3.3 preflight 走分支 A，则同样内容落在 `0023`，0022 逐字节不变**：

```sql
ALTER TABLE refund_applications
  ADD CONSTRAINT chk_refund_app_group_discriminator CHECK (
    ((group_no IS NULL) = (source IN ('payment','legacy_expired')))
    AND (source IN ('payment','legacy_expired') OR group_no <> '')
  );
```

- **不复制 source 枚举**：合法 source 由 `0017` 的封闭枚举 `chk_refund_app_source` 兜底（`migrations/order/0017_refund_batch_ledger.sql:73-74`）。判别式只表达：支付级/legacy ⇔ `group_no IS NULL`；组级必须非空串。未来新增 source：改 0017 的封闭枚举；若新 source 属于组级/支付级集合，还要改 `0022`（或 `0023`）的**集合归属**（`payment/legacy_expired` 集合或组级分支），但不得复制完整枚举。若任何人复制枚举，必须加注释“新增 source 同步 0017 与 0022/0023”，并列入迁移 checklist。
- **当前代码事实（R0.4c-impl 必须改掉）**：`0022:34` 仍复制完整 source 枚举，`0022:55` 仍是 `>=` 下限，`0022:11` 仍写“conditional ALTERs make a re-run after cleanup safe”（QC R2 T-8）。三条 MUST 在 `R0.4c-impl` 内修掉，不得只靠本文件文字关闭。
- **同名不同义检测**：`0022:28-32` 的 `information_schema.TABLE_CONSTRAINTS` 只按名称/类型判断存在性，不校验约束文本；同名不同义的旧约束会被静默接受。规则：R0.4d2 的验证脚本必须同时读 `information_schema.CHECK_CONSTRAINTS.CHECK_CLAUSE` 并与期望文本/语义比对，不一致即失败（“只允许同名同义”）。owner=Developer（R0.4d2）。
- 与 `0011` 的边界：`order_fulfillment_groups.group_no` 是组身份来源；本 CHECK 只约束 application 行，不复制组生命周期。
- 与 `0018` 的边界：0018 是 commitment 组成；不改 0018，不把组归属塞进 commitment 行。

### 2.3 `legacy_payment_resolution` 24h SLA（QC B-3）

#### 2.3.1 CHECK 改为等值

`0022`（或 `0023`）追加（0019 已建表；不改 0019）：

```sql
ALTER TABLE legacy_payment_resolution
  ADD CONSTRAINT chk_lpr_expiry_24h CHECK (
    expires_at = DATE_ADD(entered_at, INTERVAL 24 HOUR)
  );
```

- 语义：**`expires_at` 必须恰为 `entered_at + 24h`（DB 时间）**。下限写法（`>=`）允许 48h/任意更晚，使队列可以“合法地”长期 `OPEN`，直接违反 INV-46（`0002:130`：“24h 到期必须离开 `OPEN/RESOLVING`……不得无限期 `OPEN`”）。WIP 测试把 48h 当合法（`internal/services/order/r04_checks_integration_test.go:213-221`，注释在 `:215`）是错误断言，`R0.4c-impl` MUST 改成拒绝。
- 不设容忍带：容忍带会把“24h”变成区间值，制造第二事实来源；语句级 `NOW(3)` 在同一 INSERT 内与 `DEFAULT CURRENT_TIMESTAMP(3)` 一致，等值可精确复现（R0.4d2 真实迁移日志验证）。
- 0019 的 `chk_lpr_expiry (expires_at >= entered_at)` 保留（`0019:51`）。

#### 2.3.2 writer 契约、24h worker 归属与 T2 release source（QC N-7 / N-9）

- **writer 契约**（owner=Developer，T2.2/T2.4）：MUST 用**同一条 SQL** 写入 DB 时间：`INSERT ... (entered_at, expires_at) VALUES (NOW(3), DATE_ADD(NOW(3), INTERVAL 24 HOUR))`，或 `expires_at` 显式写 `DATE_ADD(NOW(3), ...)` 而 `entered_at` 用默认（同一语句时间戳）；禁止客户端时间、禁止列默认值、禁止 `expires_at` 由调用方传入任意值。
- **R0.4 不交付 writer**：今天仓库内没有 `legacy_payment_resolution` 的生产 writer（只有测试插入 `r04_checks_integration_test.go:129`），writer 属 T2.2/T2.4。`internal/services/order/` **不在** R0.4 allowed_paths；`R0.4c-impl` 只交付 CHECK + 拒绝测试。延期登记：owner=Developer，expiry=2026-12-31，tracking=`review-tracking/PAY-SETTLE-01`，status=DEFERRED（reason: writer 未实现，R0.4 不交付）。
- **24h 到期后的状态驱动（QC N-9，显式归属）**：`OPEN/RESOLVING` → `COMPENSATING` / `EXPIRED_REFUND` / `EXPIRED_FINANCE` / `REJECTED` 的推进由 **Order expiry/compensation worker** 负责（ADR §9 W10 `:964`，T2.2/T2.4）；该 worker 按 `expires_at <= NOW(3)` 扫描并推进，若该订单需要 INV-44 释放，则按 §2.3.2 的 source 前置产生 release 事件。该 worker **不在 R0.4 交付**；`0022`/`0023` 的等值 CHECK **只拒绝晚设 deadline，不能把行移出 `OPEN/RESOLVING`**（`r04_checks_integration_test.go` 也不驱动状态）。**不得声称 24h 状态机已闭环**；INV-46 保持 **DEFERRED(T2)**（`docs/lead/PAY-SETTLE-01/evidence-matrix.md` 的 INV-46 行），owner=Developer，expiry=2026-12-31。
- **T2 24h 补偿的 release source（QC N-7，已裁定，不再留给 T2 自选）**：
  - 现状封闭枚举 = `tcc`/`system_zero`/`system_shortage`/`aftersale`（`refund_release.go:29-51`；`0017` 另有 `payment`/`legacy_expired` 但被 release 生产端显式拒绝并返回 `REFUND_RELEASE_SOURCE_UNSUPPORTED`）。因此 T2 的 24h `COMPENSATING` 若要触发 INV-44 释放，**当前没有合法 source**。
  - **裁决（选定 A：新增版本化 source，作为 T2 的前置契约）**：T2 24h 补偿的释放事件 MUST 使用**新的** `source=legacy_24h` + `reason=LEGACY_24H_EXPIRY`；**禁止**复用 `tcc`/`system_zero`/`system_shortage`/`aftersale`（会伪造业务来源），**禁止**复用 `payment`/`legacy_expired`（生产端已拒绝）。
  - 该枚举增加属**契约增量**，必须在 T2 开工前的**独立 Amendment** 中定义（owner=Architect，expiry=2026-12-31，tracking=`review-tracking/PAY-SETTLE-01`）。本 Addendum **不写该枚举**、不改 `registry.go`/consumer。
  - **上线顺序（写入该 Amendment 的硬约束）**：consumer（registry 枚举 + reason 映射 + inventory handler）先发布并验证，再允许 producer 发送；旧 consumer 对未知 source 会 DLQ（`registry.go:refundReleaseReasons` 封闭映射 + `refund_release_consumer.go:56-60`），producer 提前发送 = 全量 DLQ。
  - **在 Amendment 落地前，T2 MUST NOT 产生任何 release 事件**，即 INV-46 的 `COMPENSATING` 释放腿不可执行；INV-46 保持 DEFERRED(T2)。
  - **停止条件**：T2 开工时若无该 Amendment，T2 冻结并回 Architect；不得以现有 source 临时顶替，也不得在 T2 内顺手扩枚举。
- 即使 writer/worker 缺失，CHECK 仍是唯一 DB 兜底：它必须对“晚设 deadline”失败，不能只做下限。

#### 2.3.3 `0022` 可编辑性 preflight（QC N-1，BLOCKER）

**裁决：迁移文件“可编辑性决策门”前移到 `R0.4c-impl` 的任何编辑之前。** `R0.4c-impl` 的第一条交付动作不是改 0022，而是 preflight；**preflight 证据不存在时，MUST NOT 打开/修改 `migrations/order/0022_r04_checks.sql`**。

- **owner**：Developer 执行；Lead 收存证据（Plan 指定的迁移证据文件）；Reviewer 在 R0.4e 复核；Architect 在分支 A 时**不需要**改契约（约束语义不变，只换文件名）。
- **目标环境范围**：所有**持久**环境——至少共享 dev/staging/prod，以及任何带 durable volume / 共享 DB 的预发环境。一次性容器（`scripts/verify-r01-migrations.sh` 的 disposable MySQL、`test-migrate-runner.sh` 的 scratch DB）**不构成“已 apply”**。
- **每个环境执行（只读；给命令与证据）**：

```sql
-- 1) 账本：是否已成功 apply
SELECT filename, checksum, applied_at
FROM schema_migrations
WHERE filename = 'order/0022_r04_checks.sql';

-- 2) attempt：是否有 FAILED/APPLIED 残记录（表名/用法见 scripts/test-migrate-runner.sh:45-56）
SELECT status, COUNT(*)
FROM schema_migration_attempts
WHERE filename = 'order/0022_r04_checks.sql'
GROUP BY status;
```

  - 账本列 `checksum` 的用法见 `scripts/verify-r01-migrations.sh:122`（`SELECT checksum FROM schema_migrations WHERE filename=...`）。
  - 证据必须记录：**环境名、查询输出（行数/checksum 原文）、执行人、时间、HEAD**，并附 `sha256sum migrations/order/0022_r04_checks.sql`（用于与账本 checksum 比对）。

- **分支（唯一允许的两条路，不得自创第三条）**：
  - **A. 任一目标环境存在 `schema_migrations` 行（或账本 checksum ≠ 当前文件 sha）→ 禁止编辑 0022。** 新增 additive `migrations/order/0023_r04_checks_v2.sql`，对 0022 的两个约束做**替换**（`chk_refund_app_group_discriminator`、`chk_lpr_expiry_24h`）：**优先新约束名**（如 `..._v2`）+ `ADD` 成功后再 `DROP` 旧名，避免 MySQL DDL 非事务性造成的“约束缺失窗口”；若必须沿用同名，则 `DROP` 后立即 `ADD` 并在迁移日志记录窗口时长。并同步：① §2.4 fresh-apply 顺序由 `…→0022` 改为 `…→0022→0023`；② `scripts/verify-r01-migrations.sh` 的 `MIGRATION_FILES` 与对象断言改为 0023（断言 `CHECK_CLAUSE` 为等值/判别式且旧约束已不存在）；③ `R0.4c-impl` 的 SQL 全落在 0023。**0022 保持逐字节不变。**
  - **B. 所有目标环境都返回 0 行（且无 `APPLIED` 的 attempt 残记录）→ 允许就地修订 0022。** FAILED attempt 不使文件不可变，但 MUST 按 §2.5 先清 attempt 再重跑。
- **fail-closed 补充（本 r3 裁决）**：
  1. 任一目标环境**不可达/无权限/无法确认** → **视为“已 apply”，走分支 A（0023）**。理由：改已 apply 文件的 checksum 会破坏 runner 账本与 FAILED 重放（`verify-r01-migrations.sh:122`；`test-migrate-runner.sh:45-56`），不得用“查不到就当作没跑过”换取便利。
  2. attempt 表出现 `APPLIED` 但账本无行（账本不一致）→ 同样视为“已 apply”，走分支 A，并登记数据完整性事件（owner=Developer，expiry=2026-12-31）。
  3. preflight 结论写入证据后**不得就地改判**。
  4. **stop-the-line**：若 R0.4d2 发现新环境已 apply 0022 而 `R0.4c-impl` 已改过文件 → 立即冻结 R0.4，把 0022 恢复到**preflight 前 HEAD 的字节**，改走 0023，并由 Lead 登记到 `docs/review-tracking.md` 的 change freeze 区（宪法 Workflow 9）。
- **R0.4d2 的角色**：只**复核** preflight 证据并追加到迁移日志，**不再作为首次决策点**（原 r2 的“R0.4d2 开工前查询”作废）。
- **验收（`R0.4c-impl` 入口条件，也是 R0.4e 的 `review_scope` 项）**：
  1. preflight 证据存在且覆盖所有目标环境（环境清单可核）；
  2. 分支 A：`git diff --stat` **不含** `migrations/order/0022_*`，且 0023 同时出现在 fresh-apply 顺序与验证脚本中；
  3. 分支 B：0022 的编辑提交时间/HEAD 在 preflight 证据之后（时间与 base/head 可核）。
- **不修复的后果**：在任一已 apply 0022 的环境里，`R0.4c-impl` 会先改文件、R0.4d2 才发现，导致 checksum 账本与迁移重放失效；这是迁移完整性阻断，不能降级为 TODO。

### 2.4 Fresh apply 顺序与迁移证据（QC M-4 / M-9 / N-4 / N-8 / T-6）

- **按 DB 分列，不得混用两个 DB 的编号**：

| DB | 顺序 | 说明 |
|---|---|---|
| `mall_order` | `0017 → 0018 → 0019 → 0020 → 0021 → 0022`（分支 A：`→ 0023`） | `0022`/`0023` 依赖 0017（source 枚举）、0019（`legacy_payment_resolution`）；0017/0018/0019/0020/0021 已合入，R0.4 **只读不改**（0021 的 `chk_refund_app_system_zero` 亦然） |
| `mall_fulfillment` | `0010 → 0011` | `settlement_switch_approvals` + `chk_switch_settled_expiry`；R0.4 **只读不改**（M-5 的落点不走 0012，见 §2.1.3） |

- 上述是 **R0.4 相关的增量顺序**；各 DB 的 `0001…0016`（order）/`0001…0009`（fulfillment）由既有基线承担，不在本 Addendum 重定义。
- **`0021` 必须进 canonical 验证脚本（QC N-4，`R0.4d2`，owner=Developer）**：
  - `scripts/verify-r01-migrations.sh` 的 `MIGRATION_FILES`（当前 `:28-35` 只有 `order/0017-0020` + `fulfillment/0010-0011`）MUST 新增 **`order/0021_zero_application_check.sql`** 与 **`order/0022_r04_checks.sql`**（分支 A 用 `0023`）；
  - 对象断言 MUST 包含：`chk_refund_app_system_zero`（0021）存在且文本正确、`chk_refund_app_group_discriminator` 存在且**不复制 source 枚举**、`chk_lpr_expiry_24h` 文本为**等值**；
  - **脚本 PASS 前，fresh apply 的 `0017→…→0022` 并未被 canonical 脚本证明**；因此 `R0.4d2` 不得 PASS，且 §2.4 的顺序只算“设计顺序”而非“已验证顺序”。
  - **Lead Plan 同步（ROUTED，owner=Lead，expiry=2026-12-31）**：`docs/lead/PAY-SETTLE-01/tasks.md:35` 的 R0.4d2 只写“增 `order/0022` entry”，MUST 改为 `0021 + 0022`（分支 A 为 `0021 + 0023`）并补对象断言口径。本 Addendum 不改 Plan；Plan 未改前 `R0.4d2` 的验收不完整（QC N-4 的残余）。
- **INV-44 evidence 行（QC M-9 / N-8，本文件不写台账）**：
  - owner = **Lead**（写 `docs/lead/PAY-SETTLE-01/evidence-matrix.md`）；Reviewer 在 R0.4e 按该行核对；expiry=2026-12-31。
  - 该行 MUST 至少包含：`INV-44`；分支（单组 release happy / 重复投递 no-op / 缺 reservation 行 no-op **且 metric 缺口显式登记为延期** / DLQ 重放 / 多组不释放 P1 缺口）；`test_type`（MySQL + unit）；`evidence_path`（Inventory consumer 测试 + `0022`/`0023` + 真实迁移日志）；`owner`；`status`（不得写 AC-VERIFIED；应写“部分覆盖 + P1/missing-reservation 延期”）。
  - **在该行落地前，R0.4 收口不得声称 INV-44 有证据**；R0.4e 的 verdict 必须显式列出该缺口（`release_status=NOT RELEASE`）。
  - 本 Addendum 只给要求，**不代写台账**（QC N-8 的要点：canonical 行必须由 Lead 写进 evidence-matrix，不能只在 Addendum 内声明）。
- 在脚本与真实日志落地前，`R0.4c-impl` 只能算 **“SQL 已写”**，不能算 **“迁移已验证”**；AC-27 仍是 AC-REF。
- 大表 DDL（QC T-6/T-7）：MySQL 8.4 `ADD CHECK` 会扫描全表；`ALGORITHM=INPLACE, LOCK=NONE` 对 ADD CHECK 的支持 `[不确定]`，必须由 R0.4d2 在真实 8.4 上验证并记录锁时长；若不支持，必须安排维护窗并在迁移日志/ops runbook 记录（owner=Developer，expiry=2026-12-31）。

### 2.5 失败后重跑流程（QC M-10）

- 代码事实：`0022:11` 声称“conditional ALTERs make a re-run after cleanup safe”，但 hardened runner 对 `status='FAILED'` 的 attempt **拒绝重放**（`scripts/test-migrate-runner.sh:45-53`；`scripts/verify-r01-migrations.sh:260-281` 断言的正是 “previous FAILED attempt”）。脏行导致 ALTER 失败 → attempt=FAILED → 清理数据后直接重跑仍会失败。
- **裁决（运维流程，写入迁移文件头注释与 ops runbook；owner=Developer R0.4d2）**：
  1. 失败即停，不得盲目重试；从 precheck SELECT / error 3819 定位阻塞行。
  2. 只用**审计写**修数据（补组字段/修 `expires_at`）；禁止 drop 约束、禁止批量 UPDATE 掩盖、禁止改已 apply checksum。
  3. `DELETE FROM schema_migration_attempts WHERE filename='order/0022_r04_checks.sql' AND status='FAILED'`（走 0023 则用 0023 文件名）。
  4. 重跑迁移。
  5. 复验：约束存在 + `CHECK_CLAUSE` 语义正确 + 脏行已清零。
- `migrations/fulfillment/0011_switch_expiry_check.sql:22-40` 的 `SIGNAL` precheck 模式**可选**采用（让脏行以可操作错误失败）；但它**不能替代**第 3 步——runner 在 precheck 失败后同样把 attempt 记为 FAILED 并拒绝重放。采用 SIGNAL 时，错误文本必须包含“先修数据 → 删 attempt → 重跑”。
- 删除任何“无条件 re-run safe”的表述；Addendum/迁移头注释/ops runbook 三处口径必须一致。

### 2.6 R0.4 任务拆分（owner / 证据；QC N-3 已对齐）

> **ID 冲突已解决（QC N-3）**：Lead Plan 的 `R0.4b/c` 是 **Reviewer 复审 `90b502c`**（`docs/lead/PAY-SETTLE-01/tasks.md:33`），**不是实现任务**。本 Addendum 的实现任务改用 `R0.4b-impl` / `R0.4c-impl`；r2 文本中把 `R0.4b/c` 当实现用的表述**全部作废**。

| ID | 任务 | Owner | 验收 / 证据 |
|---|---|---|---|
| **R0.4a** | 本 Addendum 修订并交 **QC round 3**；人类接受（R0.4a3） | Architect | 本文件（DRAFT 0.3）+ QC round 3 结论 + `docs/review-tracking.md` 接受行；**不要求 Reviewer 撤回 B1**（B1 已撤回）；R0.4a3 是**设计接受**，Reviewer 的 R0.4e 仍可 FAIL |
| **R0.4b-impl** | Fulfillment 启动门禁：paid/shadow/settled/legacy bool/range/expiry/approval-id；**shadow 硬拒绝（T4.4）+ 退出码 3/4 + reason 码 + 门禁/shadow 日志三字段 + config exit 2 稳定 token** | Developer + Architect | `cmd/fulfillment/main_test.go`（子进程真实退出码 2/3/4 + stderr 三字段/token，需 `TEST_MYSQL_DSN`）、`internal/services/fulfillment/settlement_switch_test.go`（门禁分支）；无 paid fallback；shadow 在 consumer 注册前非零退出。**不含** Order/Inventory metric（§2.7） |
| **R0.4c-impl** | **preflight（§2.3.3）** → `order/0022`（分支 A 为 `0023`）判别 CHECK + 等值 24h CHECK + 去枚举复制 + 48h 拒绝测试 | Developer + Architect | **preflight 证据先行**；真实 MySQL 8.4 日志绑定 HEAD；`48h/24h+1s/25h` 拒绝、`<24h` 拒绝；0017/0018/0019/0020/0021 不改；R0.4d2 前只能算“SQL 已写” |
| **R0.4d1** | `SETTLE_RELEASE_RANGE` / mode / ops 批准行模板（含 `release_range <> ''` 校验 SQL）/ 告警接线（compose + Helm + `docs/ops.md`） | Developer | `docker compose config` / Helm 渲染日志；模板 + 校验 SQL 0 行输出；ops 告警规则落 runbook；默认仍 `paid`；不改变运行语义 |
| **R0.4d2** | `scripts/verify-r01-migrations.sh` 增 `order/0021` + `0022`（或 `0023`）+ CHECK 文本断言 + **复核 preflight 证据** + 真实 MySQL 8.4 运行（replay/脏行/锁时长） | Developer | 脚本日志绑定 HEAD；`schema_migrations` 查询证据；INV-44 evidence 行由 Lead 补（本任务不含写台账） |
| **R0.4e** | Reviewer 对 R0.4a–d 的日期化 verdict | Reviewer | 精确 base/head；`release_status=NOT RELEASE`；逐条核对 preflight 证据/0021 脚本断言/INV-44 行/延期登记；不替代人类 merge |

- 本表不替代 Lead 的 Plan/Tasks；`R0.4d1/d2` 与 `docs/lead/PAY-SETTLE-01/tasks.md:28-36` 对齐；`R0.4b-impl/c-impl` 是 **新增实现 ID**，Lead Plan 若需要可加行（ROUTED，owner=Lead，expiry=2026-12-31）。
- 任何 R0.4 项都不授权 T3 commitment release、T2 历史队列 worker/source、T4.4 shadow 实现。

### 2.7 R0.4 明确不交付项与延期登记（QC N-2 的收敛表）

> 本表是**范围裁决**，不是台账。canonical 延期行仍由对应 owner 写 `docs/review-tracking.md` / `evidence-matrix.md`；本表给 owner/expiry/停止条件，供 Lead/Reviewer 核对“没有被悄悄塞回 R0.4”。

| 项 | 代码落点（只读事实） | 为何不在 R0.4 | Owner / Expiry / Tracking | 状态与停止条件 |
|---|---|---|---|---|
| 多组跳过 metric + 告警 | `internal/services/order/refund_release.go:182-186`（仅 `log.Warn`） | `internal/services/order/**` 不在 `allowed_paths`（§1.5 第 3 点） | Developer / 2026-12-31 / `review-tracking/PAY-SETTLE-01` | DEFERRED（P1/manual）。停止条件：若要在 R0.4 内做，必须**人类扩大 `allowed_paths` + 改 Lead Plan**；Developer 不得自行越界 |
| missing-reservation metric + 告警 | `internal/services/inventory/refund_release_consumer.go:92-96`（`continue`） | `internal/services/inventory/**` 不在 `allowed_paths`（§1.4） | Developer / 2026-12-31 / `review-tracking/PAY-SETTLE-01` | DEFERRED。“缺行静默”仍是已知风险；不得声称已解决。同样的扩路径停止条件 |
| 24h writer（`INSERT ... NOW(3)`） | 仓库内无生产 writer（只有测试 `r04_checks_integration_test.go:129`） | 属 T2.2/T2.4；`internal/services/order/**` 越界 | Developer / 2026-12-31 / `review-tracking/PAY-SETTLE-01` | DEFERRED。CHECK 是唯一 DB 兜底，不得因 writer 缺失而退回下限 |
| 24h expiry/compensation worker（W10） | ADR §9 W10 `:964`；T2.2/T2.4 未实现 | T2 范围；R0.4 只交 CHECK | Developer / 2026-12-31 / `review-tracking/PAY-SETTLE-01` | DEFERRED；INV-46 保持 DEFERRED(T2)。不得声称 24h 状态机闭环（§2.3.2） |
| T2 release source `legacy_24h` | `refund_release.go:29-51` 封闭枚举 | 契约增量，须独立 Amendment（§2.3.2） | Architect / 2026-12-31 / `review-tracking/PAY-SETTLE-01` | DEFERRED（T2 前置）。停止条件：T2 开工无 Amendment → 冻结 T2 回 Architect |
| fulfillment `0012` range CHECK | `0010:42` 默认 `''` | §2.1.3 已选 ops 模板落点 | Architect+Developer / 2026-12-31 | 仅当 Director 要求 DB 级强制时另立；不得在 R0.4 顺手加 |
| INV-44 evidence 行 | `docs/lead/PAY-SETTLE-01/evidence-matrix.md`（现无 INV-44 行） | 台账 owner=Lead，本文件不写台账 | Lead / 2026-12-31 | ROUTED。停止条件：行未落地 → R0.4e 必须列为 NOT RELEASE 缺口 |
| T4.4 observe-only 双消费 | `cmd/fulfillment/main.go:63-70` 硬拒绝 | T4.4 范围；R0.4 只交硬拒绝 | Developer / 2026-12-31（waiver 上限；T4.4 未交付则重新裁决） | DEFERRED；解除须 §2.1.1 四条件全部满足 |

---

## 3. C-04 release 与 T3 commitment release 的边界

| 项 | C-04 release | T3 commitment release |
|---|---|---|
| 对象 | Inventory 预留/运单/回库（INV-44 物理库存） | `order_refund_commitments` 额度（RESERVED→RELEASED，INV-45/AC-62） |
| 触发 | Order 准入后 outbox event；Inventory consumer | 打款前 `REJECTED/CANCELLED` 或 `FAILED_PERMANENT`/人工放弃 + D-24 授权 |
| 执行 | 既有 `Release`/`TccCancelConfirm` | `refund_commitment_releases` 行 + 授权金额/复核 |
| 幂等 | `release_id=rr:<refund_id>` | `UNIQUE(refund_id)`/`UNIQUE(authorization_id)` |
| RPC | **不新增** | 人工/管理命令（T3），不是 release RPC |
| R0.4 范围 | 只固化 event/schema/consumer + 单组边界 | 不实现；不得把 T3 额度释放/授权塞进 R0.4 |

- R0.4 的 `group_no` CHECK 针对 `refund_applications`，不是承诺释放；24h CHECK 针对 `legacy_payment_resolution`，不是 T3。
- 不得以 C-04 release event 替代 T3 的 commitment release：两者 owner/触发/幂等键不同（ADR §5.7 `:706-724`、§5.6 `:698-704`）。

---

## 4. QC 逐条处置

状态：**处置完成** = 本 r3 已给出可执行裁决/落点，**仍待 QC round 3 复核**；**DEFERRED** = 显式延期（owner/expiry/tracking）；**ROUTED** = 属其他 owner 的交付物，本文件只要求、不代写。**本表不是“已关闭”声明。**

### 4.1 round 2（`QC-PAY-SETTLE-01-R0.4-ADDENDUM-R2.md`）

| QC | 处置 | 落点 / 依据 | Owner / 期限 | 状态 |
|---|---|---|---|---|
| **N-1** `[BLOCKER]` 0022/0023 决策门顺序 | 决策门前移到 `R0.4c-impl` 任何编辑之前；给出目标环境清单、两条只读 SQL、checksum 比对、分支 A/B、fail-closed（不可达=已 apply）、stop-the-line、R0.4d2 只复核 | §2.3.3；`verify-r01-migrations.sh:122`；`test-migrate-runner.sh:45-56` | Dev（执行）/Lead（收证）/Reviewer（R0.4e 复核）；2026-12-31 | 处置完成（待 QC r3） |
| **N-2** `[BLOCKER]` metric 越界 + 证据错位 | metric 移出 R0.4（不改 `allowed_paths`）；从 `R0.4b-impl` PASS 条件删除；两处 metric 登记 DEFERRED owner/expiry + 扩路径停止条件；forbidden_paths 增 `internal/services/inventory/**` | §1.4/§1.5/§2.7；`refund_release.go:182-186`；`refund_release_consumer.go:92-96` | Developer；2026-12-31；`review-tracking/PAY-SETTLE-01` | 处置完成（待 QC r3） |
| **N-3** `[MAJOR]` 任务 ID 冲突 | 实现 ID 改 `R0.4b-impl`/`R0.4c-impl`；明写 `R0.4b/c`=Reviewer 复审 `90b502c`；r2 旧用法作废 | §2.6；`tasks.md:33` | Architect（改稿）/Lead（Plan 加行）；2026-12-31 | 处置完成（待 QC r3）/ ROUTED（Plan 行） |
| **N-4** `[MAJOR]` `0021` 未进脚本/Plan | 脚本 MUST 增 `0021`+`0022`；对象断言含 `chk_refund_app_system_zero`；Plan `tasks.md:35` 同步交 Lead | §2.4；`verify-r01-migrations.sh:28-35` | Dev（R0.4d2）/Lead（Plan）；2026-12-31 | 处置完成（契约）/ ROUTED（Plan） |
| **N-5** `[MAJOR]` `release_range <> ''` 无落点 | 选定唯一落点：ops 模板 + `docs/ops.md` 校验 SQL（不加 0012）；给理由、验收、停止条件/升级路径 | §2.1.3；`0010:42`；`config.go:775`；`settlement_switch.go:95,147-148` | Dev（R0.4d1）；2026-12-31 | 处置完成（待 QC r3） |
| **N-6** `[MAJOR]` config/N5 仍 panic、无三字段 | 收窄 MUST：exit 3/4 三字段；exit 2 保留 panic + 稳定 reason token（`N5_…`/`MODE_UNKNOWN`/`RELEASE_RANGE_MISSING`）；main_test 断言 ==2 且含 token | §2.1.4；`config.go:749-776`；`main.go:22-24,48-57,63-70` | Dev（R0.4b-impl）；2026-12-31 | 处置完成（待 QC r3） |
| **N-7** `[MAJOR]` T2 无合法 release source | 裁定新增 `source=legacy_24h`/`reason=LEGACY_24H_EXPIRY`，作为 T2 前置独立 Amendment；禁止复用现有 source；consumer-first 上线顺序；无 Amendment 则 T2 冻结 | §2.3.2；`refund_release.go:29-51`；`registry.go:refundReleaseReasons` | Architect（Amendment）；2026-12-31 | 处置完成（契约）/ DEFERRED（T2 前置） |
| **N-8** `[MAJOR]` INV-44 evidence 行未落 | 本文件不写台账；给出 evidence-matrix 行的必备字段与 owner/期限；行未落地则 R0.4e 必须列为缺口 | §2.4；`evidence-matrix.md`（现无 INV-44 行） | Lead；2026-12-31 | ROUTED（Lead）/ 待 R0.4e 核对 |
| **N-9** `[MAJOR]` 窗口 5 只写 writer | 显式归属：expiry/compensation worker = Order（ADR §9 W10 `:964`，T2.2/T2.4）；R0.4 只交 CHECK；不得声称 24h 闭环 | §2.3.2；`evidence-matrix.md` INV-46 行 | Developer；2026-12-31 | 处置完成（归属）/ DEFERRED（INV-46） |

### 4.2 round 1（`QC-PAY-SETTLE-01-R0.4-ADDENDUM.md`）关闭情况与残余

| QC | r3 处置 | 落点 / 依据 | Owner / 期限 | 状态 |
|---|---|---|---|---|
| B-1 shadow 运行态 | §2.1.1 保留；补 exit 3/4 与门禁/shadow 三字段的验收点 | §2.1.1/§2.1.4；`review-tracking.md:358-366` | Dev（T4.4 + R0.4b-impl） | 处置完成（契约）/ DEFERRED（实现） |
| B-2 契约来源/`group_count` | §1.2-§1.5 保留；metric 移出 R0.4（N-2）；INV-44 行交 Lead（N-8）；T2 source 已裁定（N-7） | §1.2-§1.5；ADR `:204/:712-716/:723-724/:940/:967` | Architect（QC r3）/Lead（evidence） | 处置完成（契约）/ 残余=延期项 |
| B-3 24h SLA | §2.3 保留等值 CHECK；补 preflight（N-1）、worker（N-9）、source（N-7） | §2.3；`0022:55`；`r04_checks_integration_test.go:213-221` | Dev（R0.4c-impl/d2） | 处置完成（契约）/ 实现待 |
| M-1 撤回 B1 话术 | 保留 | §1.1；`REV-...R0.2-R2.md:9,31,42,66`；`tasks.md:21` | Architect | 处置完成 |
| M-2 `baseline_impact` | 保留 INV 列表；补 metric 缺口说明 | 头部 `baseline_impact` | Architect | 处置完成 |
| M-3 AC-27 归属 | 保留“属 SPEC-0001 + AC-REF”；补 v1.5 待 QC 声明 | 头部 Spec-Ref；`review-tracking.md:350-356` | Architect / PO（v1.5） | 处置完成（v1.5 生效后以 v1.5 为准） |
| M-4 fresh apply 顺序 | 保留按 DB 分列；`0021` 进脚本与对象断言（N-4） | §2.4 | Dev（R0.4d2）/Lead（Plan） | 处置完成（契约）/ ROUTED（Plan） |
| M-5 shadow range 矛盾 | §2.1.3 保留裁决；`<> ''` 落点已定（N-5） | §2.1.3 | Dev（R0.4d1）+ Architect（Rev 2.4 errata） | 处置完成（契约）/ ROUTED（errata） |
| M-6 approval EXPIRED 无主 | §2.1.2 保留；ops 告警补 expiry/可验收断言 | §2.1.2 | ops；2026-12-31 | 处置完成（裁决）/ ROUTED（ops 告警） |
| M-7 DLQ/reservation 静默 | §1.4 保留 DLQ/重放契约；metric 部分 DEFERRED（N-2） | §1.4/§2.7 | Dev + ops；2026-12-31 | 处置完成（契约）/ DEFERRED（metric） |
| M-8 legacy bool | 保留矩阵行；引用行号校正为 `config.go:754-761` | §2.1 | Dev（R0.4b-impl） | 处置完成 |
| M-9 0022 证据/INV-44 | 脚本断言 + INV-44 行交 Lead（N-4/N-8） | §2.4 | Dev（R0.4d2）+ Lead | 处置完成（契约）/ ROUTED（台账） |
| M-10 失败后重跑 | 保留流程；删“无条件 re-run safe”要求 | §2.5；`test-migrate-runner.sh:45-53` | Dev（R0.4d2） | 处置完成 |
| M-11 退出码/日志 | §2.1.4 重写为 2/3/4 + 收窄三字段 + config token（N-6） | §2.1.4 | Dev（R0.4b-impl） | 处置完成（契约）/ 实现待 |
| M-12 0022 重复枚举 | §2.2 保留判别式；补“当前代码仍复制/仍 `>=`”的 R0.4c-impl MUST | §2.2；`0022:11,34,55` | Dev（R0.4c-impl） | 处置完成（契约）/ 实现待 |
| T-1 报告文件名 | Plan 指针不一致 | — | Lead；2026-12-31 | ROUTED |
| T-2 “不写代码”与 DDL 冲突 | §0.1 已改“设计约束，不交付迁移/代码/tests” | §0.1 | Architect | 处置完成 |
| T-3 “C-05 0022 草案” | 0022 由 R0.4c-impl（Order DB）交付；0017-0021 只读 | §2.2/§2.4 | Architect | 处置完成 |
| T-4 多行匹配 | §2.1：`approved_at DESC` 第一条全条件满足行 | §2.1；`settlement_switch.go:121-126` | Dev（R0.4b-impl） | 处置完成 |
| T-5 deploy/ops 无变量 | R0.4d1 接线；不阻塞本 Addendum | §2.6/§2.7 | Dev（R0.4d1）；2026-12-31 | ROUTED |
| T-6 大表 DDL/锁 | R0.4d2 维护窗 + `ALGORITHM/LOCK` 验证；precheck 循环到 0 | §2.4/§2.5 | Dev（R0.4d2）；2026-12-31 | ROUTED（验证） |
| T-7 Base 引 T1.4a Rulings | 头部 Base 已改 Rev 2.3 + accepted 基线 | 头部/§0.1 | Architect | 处置完成 |

### 4.3 round 2 TODO（T-1…T-11）处置

| TODO | 处置 |
|---|---|
| T-1 Plan 报告文件名 | ROUTED（Lead，2026-12-31） |
| T-2 R0.4a2 “置 accepted”措辞 | 本文件明确 `R0.4a2 = 交 QC 的 DRAFT`、`R0.4a3 = human accept`；Plan 措辞 ROUTED（Lead） |
| T-3 exit 2 与三字段矛盾 | 由 N-6 收窄解决（§2.1.4） |
| T-4 “registry 已按此 fail-closed”不精确 | 改为“registry + consumer 联合 fail-closed”（§1.3） |
| T-5 §1.2 标题“只增必填字段”与 `warehouse_id` 收窄冲突 | 改为“澄清 `items[]/warehouse_id` 歧义；代码无 top-level 字段”（§1.2 第 4 条） |
| T-6 `docs/ops.md`/`deploy/**` 无变量 | ROUTED（R0.4d1；阻塞激活，不阻塞本文件） |
| T-7 ADD CHECK 的 `ALGORITHM/LOCK`/MDL 未验证 | ROUTED（R0.4d2 真实 8.4 记录，2026-12-31） |
| T-8 0022 当前仍 `>=`/复制枚举/“re-run safe” | 转为 `R0.4c-impl` 的三条 MUST（§2.2/§2.3.1/§2.5），不得靠文字关闭 |
| T-9 M-6 ops 告警无 expiry/验收 | 已补 owner=ops、expiry、可验收断言（§2.1.2） |
| T-10 M-5 若选 ops 模板需样例行 | 已定选 ops 模板 + 样例行 + 0 行校验（§2.1.3） |
| T-11 批准路径顺序不清 | 已写五步顺序：QC r3 → R0.4a3 人类接受 → 实现 → R0.4e Reviewer（§1.2） |

---

## 5. 【反模式自省】（角色 prompt 强制项）

本设计如何防止旧反模式借尸还魂：

1. **“加个 if 就放行”**：启动门禁的每个拒绝分支都是**退出**，没有“降级 `paid`”的 else 分支；shadow 即使门禁全过也拒绝（§2.1.1），因此不存在“多一个条件就多一条放行路径”。
2. **“用 flag 默认 false 长期掩盖半成品”**：shadow 是硬拒绝而非默认关闭的灰开关；解除需要 T4.4 交付 + Reviewer PASS + Director 重批 + 契约修订四条件（§2.1.1），不是改一个默认值。
3. **“文档写已关闭，代码没变”**：`0022` 当前仍 `>=`、仍复制枚举、仍写“re-run safe”（`0022:11,34,55`）；本文件把它们写成 `R0.4c-impl` 的 MUST + R0.4d2 的对象断言，**不接受**“Addendum 已裁决”作为关闭证据（§2.2/§2.4）。
4. **“先改文件、事后补决策记录”**：preflight 前置于编辑，并规定“不可达 = 视为已 apply”与“改判即 stop-the-line”（§2.3.3）；这条直接针对 r2 的顺序缺陷。
5. **“用越界修改换取验收通过”**：metric 需要 Order/Inventory，本裁决选择**移出 R0.4 并显式延期**，而不是扩大 `allowed_paths`；任何扩路径必须人类批准 + 改 Plan（§2.7）。
6. **“用一个状态/一个事件冒充多个事实”**：C-04 release 仍是 Order(durable event) → Inventory(executor) 的单一事实链，不引入 release RPC、不复用 `order.cancelled`、不把 T3 额度释放混入（§1.1/§3）。
7. **“失败当作成功”**：DLQ/重投/`RELEASED` no-op/checksum 账本都是显式失败或显式幂等；缺失 reservation 的静默被**登记为风险**而非“已解决”（§1.4）。

---

## 6. 【假设与不确定性】（强制）

| # | 假设 / 不确定 | 影响 | Owner | Expiry | 停止条件 |
|---|---|---|---|---|---|
| 1 | **0022 是否已在某个持久环境 apply**：本文件不预设结论，改由 §2.3.3 preflight 定案。`[不确定]` | 决定分支 A（0023）或 B（就地改 0022） | Developer（执行）/Lead（收证） | 2026-12-31 | 任一环境有行/不可达 → 强制分支 A；若已改 0022 才发现 → stop-the-line + 恢复字节 + 0023（§2.3.3） |
| 2 | **MySQL 8.4 等值 CHECK 与语句时间**：假设同一 INSERT 内 `NOW(3)` 与 `DEFAULT CURRENT_TIMESTAMP(3)` 取同一语句时间。`[不确定]`：不同驱动/连接时区或显式 `SET timestamp` | 等值 CHECK 可能误拒合法写入 | Developer（R0.4d2） | 2026-12-31 | 真实 8.4 日志不能复现等值 → 停 R0.4d2，回 Architect 改契约（不得私自加容忍带） |
| 3 | **ADD CHECK 锁行为**：`ALGORITHM=INPLACE, LOCK=NONE` 对 MySQL 8.4 ADD CHECK 可用性与 MDL 时长 `[不确定]` | 大表 DDL 可能阻塞生产 | Developer（R0.4d2） | 2026-12-31 | 不支持在线 → 安排维护窗并记录；未验证前不得宣称“在线迁移” |
| 4 | **多组 / missing-reservation 可观测性**：两处 metric 已 DEFERRED，当前生产只有 `log.Warn` / `continue` | INV-44 的“失败可见性”在 P1 关闭前不完整 | Developer | 2026-12-31 | P1/multi-group 变更启动前必须带上 metric；或人类扩 R0.4 范围（§2.7） |
| 5 | **T2 release source**：`legacy_24h` 尚未在契约/registry 中定义 | INV-46 的 `COMPENSATING` 释放腿不可执行 | Architect（Amendment） | 2026-12-31 | T2 开工无 Amendment → 冻结 T2（§2.3.2） |
| 6 | **24h worker 未实现**：W10 属 T2.2/T2.4，仓库无实现 | `0022` 只能拒绝晚设 deadline，不能推进状态 | Developer | 2026-12-31 | 不得声称 24h 状态机闭环；INV-46 保持 DEFERRED |
| 7 | **T4.4 交付时间** `[不确定]` | shadow 硬拒绝没有“到期自动解除” | Developer / Director | 2026-12-31（waiver 上限） | 解除须 §2.1.1 四条件；单方面删除=违约，Reviewer 按 `[ARCHITECTURAL_FLAW]` 打回 |
| 8 | **ADR Rev 2.4 errata 未完成**：与 accepted ADR §6.2 `:822`、§6.4 `:856`、§6.2 第 4 条 `:831` 仍冲突 | 契约文本与运行态短期并存矛盾 | Architect | 本文件被接受后（不晚于 2026-12-31） | 接受后未做 Rev 2.4 → 不得关闭 R0.4 的契约收口 |
| 9 | **SPEC-0001 v1.5 未生效**：PO 已获授权，QC delta 未完成 | AC-27 文本/§7.5 漂移仍是 v1.4 口径 | PO / QC | v1.5 发布时 | v1.5 生效后本文件引用以 v1.5 为准；未生效前不得按 v1.5 预判 AC-27 |
| 10 | **WIP waiver 期限**：免单独 human approve 于 P0 closure/merge decision 或 2026-12-31（取早）失效 | 过期后未接受的裁决需重走批准 | Lead / Human | 2026-12-31 | 本文件人类接受不豁免；waiver 过期 ≠ 接受 |
| 11 | **`main_test` 依赖 `TEST_MYSQL_DSN`**（否则 `t.Skip`，`main_test.go:35-37`） | 无真实 DB 时“退出码 PASS”不是证据 | Developer | 2026-12-31 | R0.4b-impl 证据必须含真实 MySQL 日志绑定 HEAD；Skip 不算 PASS |
| 12 | **本文件未执行**任何 `go test`、`-race`、迁移或 Helm 渲染 | 文中代码/迁移引用只是只读事实，不是执行证据 | Architect | — | 任何“已验证”声明必须由 R0.4b-impl/c-impl/d1/d2 的真实日志支撑 |

---

## 7. 合规自检（供 QC round 3 / Reviewer 核对）

- **宪法 Article I（拒绝补丁/拒绝妥协）**：不新增 if-else 放行路径（§5）；metric 缺口不“假装解决”而是显式延期（§2.7）。
- **Article II（事实/幂等）**：release 事实链不变（§1.1）；`release_id`/outbox 同事务/`RELEASED` no-op 保留（§1.4）。
- **Article III.4（启动 fail-fast）**：exit 2/3/4 + panic 语义与 reason token（§2.1.4）。
- **Article IV（证据/DoD）**：AC-REF ≠ AC-VERIFIED；AC-27 仅 AC-REF（头部）；测试证据必须真实日志绑定 HEAD（§2.1.4/§2.4/§6 第 12 行）。
- **Article IV.2（重灾区抗压）**：C-04 释放的幂等/竞态窗口由既有 ADR §5.7/§9 W13 承担，本 R0.4 不改协议；R0.4e 须核对受影响基线非回归（§2.6）。
- **Workflow 9（stop-the-line）**：preflight stop-the-line（§2.3.3）、T2 冻结（§2.3.2）、metric 扩路径停止条件（§2.7）。
- **Governance 6（人类豁免记录）**：WIP waiver 引用 `docs/review-tracking.md:187-192`；本文件不自行扩大豁免（§0.1.4）。

## 8. 状态

- **Status: DRAFT 0.3 — pending QC round 3 and human acceptance**。未批准；不进入实现；不构成 AC-VERIFIED / merge / Release PASS。
- 本文件只改自己；未改 ADR Rev 2.3 / SPEC / 迁移 / 代码 / tasks / 台账。`0022`/`0023` 的 SQL 是约束设计，不是本文件交付的迁移。
- 下游顺序：**QC round 3** → **R0.4a3 人类设计接受**（记录于 `docs/review-tracking.md`）→ `R0.4b-impl` / `R0.4c-impl`（preflight 先行）/ `R0.4d1` / `R0.4d2` → **R0.4e Reviewer 日期化 verdict**（`release_status=NOT RELEASE`）。任何一步 FAIL 都冻结 `PAY-SETTLE-01` 的 R0.4 范围，不得用新 `change_id` 绕过。
- 本文件的 r2 已被本 r3 取代；r3 仍未通过 QC，**不得**被引用为“已关闭 QC round 2”。
