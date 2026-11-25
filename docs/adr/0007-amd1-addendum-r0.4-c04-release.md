# ADR-0007 Amendment 1 Addendum — R0.4 C-04 Release / Schema Ruling

| 项 | 值 |
|---|---|
| Status | **DRAFT 0.2 — pending QC round 2 and human acceptance**；未批准，不构成实现依据，不代表实现 PASS / merge / Release PASS |
| Approved | —（待 QC round 2；人类接受见 R0.4a3） |
| Revision | **0.2-draft（r2）**；修订记录见 §0.3 |
| Tier | **L2**（跨服务事件 / 库存 / 迁移 / 启动门禁） |
| change_id | `PAY-SETTLE-01` |
| Base（仓库） | `362b9e2`（QC-PAY-SETTLE-01-R0.4-ADDENDUM BLOCKED 报告落盘） |
| Base（契约基线） | ADR-0007 **Rev 2.3**（accepted 2026-11-25，`docs/adr/0007-payment-settlement-admission.md`）+ C-04 RefundBatch（accepted 2026-11-24）+ **SPEC-0001 v1.4**（approved 2026-11-24）+ **SPEC-0002 v1.10**（approved 2026-11-25） |
| Spec-Ref | **SPEC-0002 v1.10**：INV-44（`0002:128`）、INV-45（`:129`）、INV-46（`:130`）、INV-49（`:133`）、INV-50（`:134`）、AC-50、AC-62（`:308`）、AC-65（`:371`）、AC-66（`:386`）；**SPEC-0001 v1.4**：AC-27（`0001:571-579`；**属于 SPEC-0001**） |
| AC 状态声明 | AC-27 目前是 **AC-REF，不是 AC-VERIFIED**（`docs/collab/ac-coverage.tsv:41`）；本文件不改变任何 AC 状态 |
| allowed_paths（本文件） | 仅 `docs/adr/0007-amd1-addendum-r0.4-c04-release.md` |
| allowed_paths（本裁决约束的 R0.4 实现，供 Lead Plan 引用） | `internal/services/fulfillment/`、`cmd/fulfillment/`、`internal/platform/config/`、`migrations/order/`、`migrations/fulfillment/`、对应 `_test.go`。**不含** `internal/services/order/`（24h writer 属 T2，见 §2.3.2） |
| forbidden_paths | `docs/specs/**`、`docs/collab/**`、`docs/lead/**`、`docs/implementation/**`、`api/**`、`deploy/**`（R0.4d1 单独变更）、`internal/services/order/**`（本 R0.4 范围） |
| baseline_impact | **涉及**：INV-44（release event/schema/consumer，P0 单组）、INV-45（release 只释放库存预留，不改承诺额度状态）、INV-46（历史队列 24h CHECK 由下限改等值）、INV-47（不改行级/运费语义，仅 0022 约束不改变其判定）、INV-49（shadow 启动硬拒绝）、INV-50（零元与正金额同一 release 路径）。C-04 `RefundBatch` 不变；不涉及 Payment/AfterSales 状态机 |
| Actor | `arch-r04-rev`（System Architect） |
| Reviewed-by | **待 QC round 2**（本文件不代签） |
| Integration owner | System Architect（契约）；实现 owner = Developer（R0.4b/c/d）、ops（重放/告警） |
| Scope | 只裁决 R0.4：① C-04 release event v1 契约（no RPC）与 P0 单组边界；② Fulfillment 启动门禁（含 shadow 运行态、legacy bool、range、到期语义、退出语义）；③ `order/0022` 的 `group_no` 判别与 24h SLA 约束；④ 迁移 apply/重跑/证据口径 |
| 硬边界 | 不写代码/迁移/tests/台账/SPEC；不改已 accepted ADR Rev 2.3（errata 由 Architect 在 Rev 2.4 另做）；不新增 subject/RPC/持久化字段；不把 T2/T3/T4.4 业务塞进 R0.4 |

---

## 0. 阅读方式与规范边界

### 0.1 本文件的效力边界

1. 本文件是**设计约束**，不是迁移文件、不是代码、不是测试证据（QC T-2）。文中 SQL 是 **schema 约束设计**，不是交付的迁移文件。
2. 任何“MUST/裁决”都不免除 QC round 2、Reviewer 与人类接受（`AGENTS.md` §2.3/§2.7）。本文件不批准 merge/Release，也不把任何 AC 标为 AC-VERIFIED。
3. 设计结论只活在 ADR：`docs/collab/CONVENTIONS.md:11`。**已 withdrawn 的 `0007-amd1-addendum-t14b2-inv44-group.md` 与 `docs/lead/PAY-SETTLE-01/t1.4b2-design.md` 不是 C-04 release 的契约来源**（ADR Rev 2.3 `:19` 与 `:1607-1608` 已标记 withdrawn / not merged；Lead 文档不是契约）。本文件不再引用它们作为契约（QC B-2、T-7）。
4. WIP waiver：`docs/review-tracking.md:187-191` 规定 PAY-SETTLE-01 WIP 期间 ADR/SPEC Amendment **免单独 human approve**，但 QC/Architect 复核、merge、Release PASS、AC-VERIFIED **不豁免**；waiver 到期 = P0 closure/merge decision 或 2026-12-31（取早）。本 Addendum 的**人类接受**（R0.4a3）不属于该豁免，仍须记录。

### 0.2 与 QC 报告的关系

输入为 `docs/collab/qc-reports/QC-PAY-SETTLE-01-R0.4-ADDENDUM.md`（BLOCKED：3 BLOCKER + 12 MAJOR + 7 TODO）。逐条处置见 §4；每条均给出本文件落点、owner 与状态（FIXED / DEFERRED / ROUTED）。本文件不会把“已改文档”表述为“已通过 QC”。

### 0.3 修订记录

| 版本 | 日期 | 内容 |
|---|---|---|
| 0.1-draft（r1） | 2026-11-25 | 初稿：C-04 release no-RPC、启动门禁、`group_no`/24h CHECK。QC round 1 判 **BLOCKED**。 |
| **0.2-draft（r2）** | 2026-11-25 | 按 QC 修订：B-1 shadow 运行态裁决（§2.1.1）、B-2 release event v1 契约与 additive amendment（§1.2-§1.4）、B-3 24h 等值 CHECK 与 writer/迁移口径（§2.3）；M-1…M-12 逐条（§4）；TODO T-2/T-3/T-4/T-7 并入正文，T-1/T-5/T-6 路由 owner |

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

**本 Addendum 是对 C-04 release event v1 的 additive amendment**（只增必填安全字段与消费约束，不改既有字段语义、不改 subject 版本号、不新增 RPC）：

1. 明确 subject / event_type：`shop.inventory.refund_release_requested.v1` / `inventory.refund_release_requested`（代码事实 `internal/platform/events/events.go:42-43`；ADR 只写“组级释放请求”未命名 subject）。
2. 增加并定为必填：`schema_version`（=1）、`release_scope`（=`order_reservation_full`）、`group_count`（P0 必填=1）、`requested_at`（RFC3339）。
3. 增加可选审计字段：`covered_group_nos`（存在时必须恰为单组且等于 `group_no`）。
4. 收窄仓库身份：**`warehouse_id` 属于 `items[]` 且每项必填**；v1 不设 top-level `warehouse_id`（全单释放跨仓时 top-level 语义不唯一，不得缺省 W1）。
5. 明确 consumer 的 fail-closed 字段/消费约束（§1.3 字段表 + §1.4 消费契约）与多组订单不释放的显式缺口（§1.5）。

**为什么必须 amend 而不是“照代码”**：accepted ADR `:204`/`:724` 的字段表**没有** `group_count`/`release_scope`/`covered_group_nos`，而 consumer 把它们当必填安全事实（`internal/services/inventory/refund_release_consumer.go:65-70`；`internal/platform/events/registry.go:320-323` 列出 11 个必填字段、`validateRefundRelease` `:225-302` 校验枚举/派生键/items）。若不在规范文本中固化，契约就停留在 code-only，下一位实现者可以合法地删字段或用 `group_no` 代替，直接触发全量 DLQ 或误释放。

**批准路径**：本 amendment 须经 **QC round 2 + Reviewer** 复核；按 WIP waiver（`docs/review-tracking.md:187-191`）**免单独 human approve**，但 **R0.4a3 人类接受本 Addendum 不豁免**，且必须在 `docs/review-tracking.md` 记录接受事实。接受后由 **Architect** 在 ADR Rev 2.4 把本 §1.2-§1.4 与 §2 的运行态裁决回写进 accepted 正文（owner=Architect，不在本文件内完成）。

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
- `quantity` 是审计输入：全单释放以 Inventory 预留行的权威数量为准（`CONFIRMED` 时用预留数量，不用事件数量），不得用事件数量制造部分释放。

### 1.4 Consumer / producer / DLQ 契约（QC M-7）

- **producer**：`outbox.Append` 与准入写入**同一事务**（`internal/services/order/refund_admission.go:283-287`；零元同路径 `:508-514`）。禁止事务内远程调用 Inventory。
- **subject / DLQ**：`shop.inventory.refund_release_requested.v1`；DLQ = 同 subject 的 `<subject>.dlq`（consumer 默认 `MaxDeliver=20`，`internal/platform/consumer/consumer.go:69-70`；DLQ 口径 `docs/ops.md:261-275`）。
- **producer 重试**：outbox relay 20 次后 dead-letter（`internal/platform/outbox/outbox.go:60-61`、`:264-283`），死信先复制到 `outbox_dead_letters`（`docs/ops.md:261-275`）。
- **重放 owner = ops**（ADR W13 `:967`），步骤 = `docs/ops.md:261-275`；重放前必须确认业务态与 handler 幂等。不得由 Developer 写脚本自动重放。
- **缺失 reservation 行**：是幂等 no-op（`refund_release_consumer.go:92-96`；`internal/services/inventory/service.go:171-186`），但 **MUST NOT 完全静默**：每次必须计数并记录 `release_id/order_no/sku_id/warehouse_id`；新增 metric（如 `inventory_refund_release_missing_reservation_total`）+ 告警。理由：缺行既可能是已释放（正常），也可能是事件早到/数据丢失（异常），静默会把数据完整性故障伪装成成功。
- **重复/乱序**：`RELEASED` no-op；同一订单只发一次 release（`refund_release.go:187-193`）；重复投递只收敛状态，不二次释放。
- **多组订单无 release**：见 §1.5，必须计数/告警，不得只打一行 `log.Warn`。

### 1.5 P0 单组边界与多组缺口（QC B-2 第 2 点）

- **P0 路线**：只有单组订单（`group_count=1`）产生 release event；这是 R0.4 的合法单组范围，也是 INV-44 在 P0 的可验证范围。
- **多组订单现状（代码事实）**：`appendRefundReleaseTx` 在 `groupCount != 1` 时直接 `return nil`，只 `log.Warn("multi-group refund keeps inventory reservations (P1/manual)")`（`internal/services/order/refund_release.go:178-186`）：**不发 outbox、不重试、无人工队列**。这是**显式的“不释放”**，不是“释放成功”，也不是可接受的静默成功。
- **裁决**：
  1. 多组不释放作为 **P1/manual 已知缺口**显式登记，不得声称 INV-44 已完成、不得声称 AC-27 的释放腿在 2 组订单上已验证。
  2. 登记字段（由 owner 在 canonical 台账写，不在本文件）：`owner=Developer`、`expiry=2026-12-31`、`tracking=review-tracking/PAY-SETTLE-01`、`reason=multi-group INV-44 release not emitted; P1/manual`、`status=DEFERRED`。
  3. 生产可观测性：多组跳过必须新增 metric + 告警，携带 `order_no/refund_id/group_count`；只 `log.Warn` 不构成可观测性。**R0.4b/c 的 PASS 条件包含该 metric 的测试**（Developer）。
  4. `docs/lead/PAY-SETTLE-01/evidence-matrix.md` 必须新增 **INV-44 行**（现只有 INV-45…50，QC M-9）；owner = Lead，作为 R0.4 收口证据。
- 该缺口**不阻塞** R0.4b/c 的代码正确性，但**阻塞**“INV-44 已完成 / 多组释放已验证”的声明。

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
| 新旧冲突 | mode 与 legacy 映射不一致 | fail-fast（`config.go:736-760`） | 非零退出，不静默择一 |
| 未知/大小写不符/空且 legacy 未设 | — | 不默认 `paid` | 非零退出 |

- paid 是**唯一**允许的默认路径；`SETTLE_RELEASE_APPROVAL_ID` 不得单独生效（N5，ADR `:828`）。
- `release_range` 不得为空串：空串不是通配符（`config.go:775`；`settlement_switch.go:95,148`）。
- 读取表/连接失败 → 启动失败，不得猜 `paid`。
- 多行匹配（QC T-4）：取 `approved_at DESC` 下**第一条全部条件满足**的行（代码 `settlement_switch.go:124`），审计记录该 `approval_id`；不得“任取一行”。

#### 2.1.1 shadow 运行态裁决（QC B-1；Director 2026-11-25）

> **裁决来源**：`docs/review-tracking.md:350-358`——Director 2026-11-25 决定 `FULFILLMENT_SETTLE_CONSUMER_MODE=shadow` **hard-rejected at startup until T4.4**；Addendum 必须记录运行态裁决、fail-closed 退出语义与 T4.4 收口条件；默认保持 `paid`。

**规范条款**：

- **WHEN** 启动请求 `FULFILLMENT_SETTLE_CONSUMER_MODE=shadow`，**THE SYSTEM SHALL** 在注册任何 consumer/worker 之前**拒绝启动**；**无论 §2.1 的批准行门禁是否通过**，都不得进入 shadow 运行态、不得回退 `paid`、不得注册 `order.paid` consumer 顶替。
- **IF** 门禁未通过，**THEN** 以门禁原因拒绝（reason = 具体门禁码，§2.1.4）；**IF** 门禁通过，**THEN** 以稳定 reason **`T4_4_SHADOW_NOT_DELIVERED`** 拒绝。
- 退出必须非零；日志 MUST 含 `mode`、`approval_id`（门禁解析出的行 id；门禁未解析则为空串）与 `reason`。
- **owner**：Developer（T4.4 observe-only 双消费交付）+ Architect（契约/收口条件）；ops 在本裁决解除前 **MUST NOT** 写入 shadow 批准行——写了也不可执行，只会制造“已批准但起不来”的审计噪音。
- **T4.4 收口条件（全部满足才可解除硬拒绝）**：① observe-only 双消费（`order.paid`+`order.settled`，只记录/比对、不建单、不推进组）交付；② Reviewer 对 T4.4 的精确 base/head 出 PASS 证据；③ Director **重新批准** shadow 运行；④ 由 Architect 以 ADR Amendment（Rev 2.4 或新 Addendum）显式删除本硬拒绝条款。**在此之前，任何代码路径不得删除/弱化该拒绝**（删除即违约，Reviewer 应按 `[ARCHITECTURAL_FLAW]` 打回）。
- **默认 `paid` 不变**；本裁决不放行 `90b502c`，也不豁免 Reviewer/人类接受/Release/AC-VERIFIED。
- **ADR 冲突登记**：accepted ADR §6.2 `:822` 与 §6.4 步骤 1 `:856` 仍把 shadow 写成“可启动/可执行”，与本次 Director 裁决及实现（`cmd/fulfillment/main.go:63-70`）冲突。本 Addendum 在 **R0.4/PAY-SETTLE-01 WIP 范围内**以本条款为运行态契约；Architect 必须在接受后于 **Rev 2.4** 加 errata（owner=Architect，接受后执行），否则 accepted ADR 与本文件长期并存矛盾。
- **门禁错误优先于 T4.4 拒绝**：先跑 §2.1 门禁（只读）；门禁失败报门禁原因，门禁通过才报 `T4_4_SHADOW_NOT_DELIVERED`。两种路径都拒绝启动、都非零退出。

#### 2.1.2 批准行到期语义（QC M-6）

- 到期是**启动窗口语义**：运行中的进程**不会**因 `expires_at` 到期自动停止；运行期不轮询、不热切换（ADR `:836`）。
- 唯一停止路径 = ADR §6.4 的 **Director 批准回滚 + 重启**（`:864-867`）；不得由应用自动改模式。
- `settlement_switch_approvals.status='EXPIRED'` **不由应用写入**（应用只读，`:832`），因此该枚举值在 R0.4 是**不可达状态**。设计裁决：**不新增写者**；看板/告警 MUST 以 `expires_at <= NOW(3)` 判定过期，**不得依赖 `status`**（否则过期行仍显示 `APPROVED`，`docs/review-tracking.md` 式的“有效”误判会复发）。
- **ops 必须配置告警**：`target_mode='settled' AND status='APPROVED' AND expires_at <= NOW(3)`；过期仍运行的实例由 Director 决定回滚；owner=ops。
- 若业务后续要求 `status='EXPIRED'` 可达，另立受控 ops 迁移/任务（owner=ops，expiry=2026-12-31，tracking=review-tracking/PAY-SETTLE-01），不得留给“以后”；在此之前它不是 R0.4 交付。

#### 2.1.3 shadow 的 `release_range`（QC M-5）

- **裁决：shadow 也要求 `release_range` 非空且与部署 range 精确相等。** 该裁决消除 accepted ADR 内部冲突：§6.2 第 4 条（`:831`）只要求存在 `target_mode='shadow' AND status='APPROVED'` 的行，而 §6.3（`:848`）与 §12.5.6（`:1285`）要求 range 校验；代码三处一致要求非空 + 精确匹配（`config.go:775`；`settlement_switch.go:95,148`）。
- 后果：shadow 有**两个独立不可启动原因**——range 缺失/不匹配（门禁拒绝）与 T4.4 未交付（运行态拒绝）；两者都必须 fail-closed，不得互相掩盖。
- 批准行 `release_range` MUST `<> ''`：通过 fulfillment 迁移 CHECK 或 ops 写入模板保证（owner=Developer R0.4d1 / ops 模板；`docs/ops.md` 须写模板）。空 range 永不作通配符。
- Architect 在 Rev 2.4 修 §6.2 第 4 条 errata（owner=Architect）。

#### 2.1.4 退出码与审计字段（QC M-11）

稳定契约（R0.4b 验收项，Developer）：

| 退出码 | 含义 | 当前实现 |
|---|---|---|
| `1` | 基础设施失败（open DB / connect NATS / dial Order / tracing / run） | 已如此（`cmd/fulfillment/main.go:30,38,74,87,110`） |
| `2` | config / N5 / 未知 mode / range 缺失（`config.Load` 返回错误 → panic → Go exit 2） | 已如此（config 层，`config.go:775`） |
| `3` | 批准行门禁拒绝（缺行/过期/非 APPROVED/range 不匹配/DB 不可读） | **当前是 1**，R0.4b MUST 改为 3 |
| `4` | shadow 运行态拒绝（`T4_4_SHADOW_NOT_DELIVERED`） | **当前是 1**，且日志缺 `approval_id/reason`（`main.go:63-70`），R0.4b MUST 补齐 |

- 稳定 reason 码（日志字段 `reason`）：`N5_APPROVAL_ID_MODE_MISMATCH`、`MODE_UNKNOWN`、`RELEASE_RANGE_MISSING`、`APPROVAL_ROW_MISSING`、`APPROVAL_NOT_APPROVED`、`APPROVAL_EXPIRED`、`RELEASE_RANGE_MISMATCH`、`APPROVAL_DB_UNAVAILABLE`、`T4_4_SHADOW_NOT_DELIVERED`。
- 所有拒绝日志 MUST 在注册 consumer 前输出，字段至少 `mode/approval_id/reason`；K8s CrashLoopBackOff 的区分依赖 reason（仅靠退出码 1 无法区分“门禁拒绝”“DB 连不上”）。
- R0.4b 证据：`cmd/fulfillment/main_test.go`（退出码 + 日志字段）与 `internal/services/fulfillment/settlement_switch_test.go`（门禁分支）。

### 2.2 `order/0022`：`group_no` 判别 CHECK（QC M-12）

新增 `migrations/order/0022_r04_checks.sql`（Order DB；不改已合入的 0017/0018/0019/0020/0021）：

```sql
ALTER TABLE refund_applications
  ADD CONSTRAINT chk_refund_app_group_discriminator CHECK (
    ((group_no IS NULL) = (source IN ('payment','legacy_expired')))
    AND (source IN ('payment','legacy_expired') OR group_no <> '')
  );
```

- **不复制 source 枚举**：合法 source 由 `0017` 的封闭枚举 `chk_refund_app_source` 兜底（`migrations/order/0017_refund_batch_ledger.sql:73-74`）。判别式只表达：支付级/legacy ⇔ `group_no IS NULL`；组级必须非空串。未来新增 source：改 0017 的封闭枚举；若新 source 属于组级/支付级集合，还要改 `0022` 的**集合归属**（`payment/legacy_expired` 集合或组级分支），但不得复制完整枚举。若任何人复制枚举，必须加注释“新增 source 同步 0017 与 0022”，并列入迁移 checklist。
- **同名不同义检测**：`0022:28-32` 的 `information_schema.TABLE_CONSTRAINTS` 只按名称/类型判断存在性，不校验约束文本；同名不同义的旧约束会被静默接受。规则：R0.4d2 的验证脚本必须同时读 `information_schema.CHECK_CONSTRAINTS.CHECK_CLAUSE` 并与期望文本/语义比对，不一致即失败（“只允许同名同义”）。owner=Developer（R0.4d2）。
- 与 `0011` 的边界：`order_fulfillment_groups.group_no` 是组身份来源；本 CHECK 只约束 application 行，不复制组生命周期。
- 与 `0018` 的边界：0018 是 commitment 组成；不改 0018，不把组归属塞进 commitment 行。

### 2.3 `legacy_payment_resolution` 24h SLA（QC B-3）

#### 2.3.1 CHECK 改为等值

`0022` 追加（0019 已建表；不改 0019）：

```sql
ALTER TABLE legacy_payment_resolution
  ADD CONSTRAINT chk_lpr_expiry_24h CHECK (
    expires_at = DATE_ADD(entered_at, INTERVAL 24 HOUR)
  );
```

- 语义：**`expires_at` 必须恰为 `entered_at + 24h`（DB 时间）**。下限写法（`>=`）允许 48h/任意更晚，使队列可以“合法地”长期 `OPEN`，直接违反 INV-46（`0002:130`：“24h 到期必须离开 `OPEN/RESOLVING`……不得无限期 `OPEN`”）。WIP 测试把 48h 当合法（`internal/services/order/r04_checks_integration_test.go:215-219`）是错误断言，R0.4c MUST 改成拒绝。
- 不设容忍带：容忍带会把“24h”变成区间值，制造第二事实来源；语句级 `NOW(3)` 在同一 INSERT 内与 `DEFAULT CURRENT_TIMESTAMP(3)` 一致，等值可精确复现（R0.4d2 真实迁移日志验证）。
- 0019 的 `chk_lpr_expiry (expires_at >= entered_at)` 保留（`0019:51`）。

#### 2.3.2 writer 契约与归属

- writer（owner=Developer，T2.2/T2.4）MUST 用**同一条 SQL** 写入 DB 时间：`INSERT ... (entered_at, expires_at) VALUES (NOW(3), DATE_ADD(NOW(3), INTERVAL 24 HOUR))`，或 `expires_at` 显式写 `DATE_ADD(NOW(3), ...)` 而 `entered_at` 用默认（同一语句时间戳）；禁止客户端时间、禁止列默认值、禁止 `expires_at` 由调用方传入任意值。
- **R0.4 不交付 writer**：今天仓库内没有 `legacy_payment_resolution` 的生产 writer（只有测试插入 `r04_checks_integration_test.go:129`），writer 属 T2.2/T2.4。`internal/services/order/` **不在** R0.4 allowed_paths；R0.4c 只交付 CHECK + 拒绝测试。T2 writer 的延期登记：owner=Developer，expiry=2026-12-31，tracking=review-tracking/PAY-SETTLE-01，status=DEFERRED（reason: writer 未实现，R0.4 不交付）。
- 即使 writer 缺失，CHECK 仍是唯一 DB 兜底：它必须对“晚设 deadline”失败，不能只做下限。

#### 2.3.3 修订 0022 还是新增 0023（迁移不可变口径）

- **裁决：在 0022 尚未被任何持久环境首次 apply 前，就地修订 0022。** 当前事实支持该判断：canonical 脚本 `scripts/verify-r01-migrations.sh:20-27` 未包含 0022；R0.4d2 未完成（`docs/lead/PAY-SETTLE-01/tasks.md:35`）；仓库内无 0022 的真实 apply 日志。测试用 scratch DB 是一次性容器，不构成“已 apply”。
- **IF** 任何持久环境的 `schema_migrations` 已记录 `order/0022_r04_checks.sql` 的 checksum，**THEN** 禁止改 0022，必须新增 additive 的 `0023`（先 drop 旧 CONSTRAINT 再 add 等值 CHECK，或新约束名 + 旧约束 drop）；**不得**改已 apply 文件的 checksum（hardened runner 的账本/重放会失效）。
- `[不确定]`：WIP 作者是否在某个非共享本地 DB 跑过 0022。**决策门**：R0.4d2 开工前由 Developer 查询目标环境的 `schema_migrations`，把结果写进迁移日志；有 0022 行 → 0023，无 → 就地改。owner=Developer，expiry=2026-12-31。
- precheck 同步改为 `WHERE expires_at <> DATE_ADD(entered_at, INTERVAL 24 HOUR)`（诊断 `LIMIT 100` 只是样本，必须循环到 0 行，QC T-6）。
- 测试（R0.4c）：`=24h` 通过；`48h`/`24h+1s`/`25h` **必须失败**；`<24h` 失败；删除 `:215` 的“48h passes”注释；真实 MySQL 8.4 日志绑定 HEAD（R0.4d2）。

### 2.4 Fresh apply 顺序与迁移证据（QC M-4 / M-9 / T-6）

- **按 DB 分列，不得混用两个 DB 的编号**：

| DB | 顺序 | 说明 |
|---|---|---|
| `mall_order` | `0017 → 0018 → 0019 → 0020 → 0021 → 0022` | 0022 依赖 0017（source 枚举）、0019（`legacy_payment_resolution`）；0017/0018/0019/0020/0021 已合入，R0.4 **只读不改** |
| `mall_fulfillment` | `0010 → 0011` | `settlement_switch_approvals` + `chk_switch_settled_expiry`；R0.4 **只读不改** |

- 上述是 **R0.4 相关的增量顺序**；各 DB 的 `0001…0016`（order）/`0001…0009`（fulfillment）由既有基线承担，不在本 Addendum 重定义。
- `scripts/verify-r01-migrations.sh` 的 `MIGRATION_FILES` 必须新增 `order/0021_zero_application_check.sql` 与 `order/0022_r04_checks.sql`（R0.4d2，owner=Developer）；对象断言必须包含：两个 CHECK 存在、`chk_lpr_expiry_24h` 文本为等值、`chk_refund_app_group_discriminator` 不复制 source 枚举。
- 在脚本与真实日志落地前，R0.4c 只能算 **“SQL 已写”**，不能算 **“迁移已验证”**；`docs/lead/PAY-SETTLE-01/evidence-matrix.md` 的 **INV-44 行**（owner=Lead）与 R0.4d2 日志共同构成 INV-44 证据；AC-27 仍是 AC-REF。
- 大表 DDL（QC T-6）：MySQL 8.4 `ADD CHECK` 会扫描全表；`ALGORITHM=INPLACE, LOCK=NONE` 对 ADD CHECK 的支持 `[不确定]`，必须由 R0.4d2 在真实 8.4 上验证并记录锁时长；若不支持，必须安排维护窗并在迁移日志/ops runbook 记录（owner=Developer，expiry=2026-12-31）。

### 2.5 失败后重跑流程（QC M-10）

- 代码事实：`0022:11` 声称“conditional ALTERs make a re-run after cleanup safe”，但 hardened runner 对 `status='FAILED'` 的 attempt **拒绝重放**（`scripts/test-migrate-runner.sh:49-53`；`scripts/verify-r01-migrations.sh:260-281` 断言的正是 “previous FAILED attempt”）。脏行导致 ALTER 失败 → attempt=FAILED → 清理数据后直接重跑仍会失败。
- **裁决（运维流程，写入 `0022` 头注释与 ops runbook；owner=Developer R0.4d2）**：
  1. 失败即停，不得盲目重试；从 precheck SELECT / error 3819 定位阻塞行。
  2. 只用**审计写**修数据（补组字段/修 `expires_at`）；禁止 drop 约束、禁止批量 UPDATE 掩盖、禁止改已 apply checksum。
  3. `DELETE FROM schema_migration_attempts WHERE filename='order/0022_r04_checks.sql' AND status='FAILED'`（若走 0023 则用 0023 文件名）。
  4. 重跑迁移。
  5. 复验：约束存在 + `CHECK_CLAUSE` 语义正确 + 脏行已清零。
- `migrations/fulfillment/0011_switch_expiry_check.sql:22-40` 的 `SIGNAL` precheck 模式**可选**采用（让脏行以可操作错误失败）；但它**不能替代**第 3 步——runner 在 precheck 失败后同样把 attempt 记为 FAILED 并拒绝重放。采用 SIGNAL 时，错误文本必须包含“先修数据 → 删 attempt → 重跑”。
- 删除任何“无条件 re-run safe”的表述；Addendum/迁移头注释/ops runbook 三处口径必须一致。

### 2.6 R0.4 任务拆分（owner / 证据）

| ID | 任务 | Owner | 验收 / 证据 |
|---|---|---|---|
| **R0.4a** | 本 Addendum 修订并交 QC round 2；人类接受（R0.4a3） | Architect | 本文件（DRAFT 0.2）+ QC round 2 结论 + `docs/review-tracking.md` 接受行；**不要求 Reviewer 撤回 B1**（B1 已撤回） |
| **R0.4b** | Fulfillment 启动门禁：paid/shadow/settled/legacy bool/range/expiry/approval-id；**shadow 硬拒绝（T4.4）+ 退出码 3/4 + reason 码 + 日志三字段**；多组跳过 metric | Developer + Architect | `cmd/fulfillment/main_test.go`（退出码/日志）、`internal/services/fulfillment/settlement_switch_test.go`（门禁分支）；无 paid fallback；shadow 在 consumer 注册前非零退出 |
| **R0.4c** | `order/0022` 判别 CHECK + 等值 24h CHECK + 重复枚举/文本校验 + 48h 拒绝测试 | Developer + Architect | 真实 MySQL 8.4 日志绑定 HEAD；`48h/24h+1s/25h` 拒绝；0017/0018/0019/0020/0021 不改；R0.4d2 前只能算“SQL 已写” |
| **R0.4d1** | `SETTLE_RELEASE_RANGE` / mode / ops 模板 / 告警接线（compose + Helm + `docs/ops.md`） | Developer | `docker compose config` / Helm 渲染日志；默认仍 `paid`；不改变运行语义 |
| **R0.4d2** | `scripts/verify-r01-migrations.sh` 增 `order/0021`/`0022` + CHECK 文本断言 + 0022/0023 决策 + 真实 MySQL 8.4 运行（replay/脏行/锁时长） | Developer | 脚本日志绑定 HEAD；`schema_migrations` 查询证据；INV-44 evidence 行由 Lead 补 |
| **R0.4e** | Reviewer 对 R0.4a–d 的日期化 verdict | Reviewer | 精确 base/head；`release_status=NOT RELEASE`；不替代人类 merge |

- 本表不替代 Lead 的 Plan/Tasks；`R0.4d1/d2` 与 `docs/lead/PAY-SETTLE-01/tasks.md:29-36` 对齐。
- 任何 R0.4 项都不授权 T3 commitment release、T2 历史队列 worker 或 T4.4 shadow 实现。

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

状态：**FIXED** = 本文件已裁决；**DEFERRED** = 显式延期（owner/expiry/tracking）；**ROUTED** = 属其他 owner 的交付物，本文件只要求、不代写。

| QC | 处置 | 落点 / 依据 | Owner / 期限 | 状态 |
|---|---|---|---|---|
| B-1 shadow 运行态 | 新增 §2.1.1：无论门禁是否通过都拒绝启动；reason=`T4_4_SHADOW_NOT_DELIVERED`；日志三字段；owner/T4.4 收口条件；默认 paid | §2.1.1；`docs/review-tracking.md:350-358`；`main.go:63-70` | Dev(T4.4)+Architect；解除=T4.4+Reviewer+Director 重批+契约修订 | FIXED（契约）/ DEFERRED（实现，T4.4） |
| B-2 契约来源/`group_count` | §1.2 权威来源 + additive amendment + 字段表；§1.4 fail-closed；§1.5 多组缺口 | §1.2-§1.5；ADR `:204/:712-716/:723-724/:940/:967` | Architect（QC round 2 + Reviewer）；多组缺口 Dev/2026-12-31 | FIXED（契约）/ DEFERRED（多组 P1） |
| B-3 24h SLA | §2.3 等值 CHECK；writer=DB 时间同语句；0022 就地修订 vs 0023 决策门；WIP 48h 断言改拒绝 | §2.3；`0022:55`；`r04_checks_integration_test.go:215-219` | Dev(R0.4c/d2)；writer=T2/2026-12-31 | FIXED |
| M-1 撤回 B1 话术 | 删除“应撤回/越界”表述；写明 B1 已于 REV-R0.2-R2 撤回、C-04 PASS | §1.1；`REV-...R0.2-R2.md:9,31,42,66`；`tasks.md:21` | Architect | FIXED |
| M-2 baseline_impact 无 INV | 头部列出 INV-44/45/46/47/49/50 及不变项 | 头部 `baseline_impact` | Architect | FIXED |
| M-3 AC-27 归属 | Spec-Ref 改为 SPEC-0001 v1.4 AC-27，并声明 AC-REF 非 AC-VERIFIED | 头部；`0001:571-579`；`ac-coverage.tsv:41` | Architect | FIXED |
| M-4 fresh apply 顺序 | §2.4 按 DB 分列 `0017→…→0022` / `0010→0011`；0017-0021 只读 | §2.4 | Dev(R0.4d2) | FIXED |
| M-5 shadow range 矛盾 | §2.1.3 裁决 shadow 要 range；ADR §6.2 第 4 条 errata 交 Rev 2.4 | §2.1.3；ADR `:831/:848/:1285` | Architect(Rev 2.4)+Dev | FIXED（契约）/ ROUTED（errata） |
| M-6 approval EXPIRED 无主 | §2.1.2 启动窗口语义；应用不写 EXPIRED；看板按 `expires_at`；ops 告警；无运行期轮询 | §2.1.2；ADR `:832/:836` | ops（告警）；Architect（errata） | FIXED（裁决）/ ROUTED（ops 告警） |
| M-7 DLQ/reservation 静默 | §1.4 subject/DLQ/20 次/重放 owner；缺 reservation 计数+告警；多组 metric | §1.4-§1.5 | Dev + ops | FIXED（契约）/ ROUTED（metric 实现） |
| M-8 legacy bool | §2.1 矩阵新增 legacy 行；`true`→settled 且需 approval id；冲突 fail-fast | §2.1；ADR `:834` | Dev(R0.4b) | FIXED |
| M-9 0022 证据/INV-44 | §2.4 canonical 脚本增 0021/0022 + CHECK 文本；INV-44 evidence 行 | §2.4；`verify-r01-migrations.sh:20-27` | Dev(R0.4d2)+Lead(evidence) | DEFERRED（R0.4d2，2026-12-31） |
| M-10 失败后重跑 | §2.5 修复流程 + 删 attempt；SIGNAL 可选但不能替代 | §2.5；`test-migrate-runner.sh:49-53` | Dev(R0.4d2) | FIXED |
| M-11 退出码/日志 | §2.1.4 退出码 1/2/3/4 + 稳定 reason + 三字段；当前 WIP 不满足，R0.4b 验收 | §2.1.4；`main.go:30,38,57,63-70,74,87,110` | Dev(R0.4b) | FIXED（契约）/ 阻塞 R0.4b |
| M-12 0022 重复枚举 | §2.2 判别式不复制枚举；0017 兜底；CHECK 文本校验 | §2.2；`0017:73-74`；`0022:34` | Dev(R0.4c/d2) | FIXED |
| T-1 报告文件名 | Lead Plan 指针不一致（`tasks.md:30` vs 实际 QC 文件名） | — | Lead | ROUTED |
| T-2 “不写代码/迁移/测试”与 DDL 冲突 | §0.1 改为“只给设计约束，不交付迁移文件/代码/tests” | §0.1 | Architect | FIXED |
| T-3 “C-05 0022 草案”含义 | 删除；0022 由 R0.4c 交付（Order DB），C-05 的 0017/0018/0019/0020/0021 只读 | §2.2/§2.4 | Architect | FIXED |
| T-4 多行匹配 | §2.1：`approved_at DESC` 第一条全条件满足行，审计 approval_id | §2.1；`settlement_switch.go:124` | Dev(R0.4b) | FIXED |
| T-5 deploy/ops 无变量 | §2.6 R0.4d1 接线；阻塞激活，不阻塞本 Addendum | §2.6 | Dev(R0.4d1) | ROUTED |
| T-6 大表 DDL/锁 | §2.4 维护窗 + `ALGORITHM/LOCK` 验证；precheck 循环到 0 | §2.4-§2.5 | Dev(R0.4d2) | ROUTED（验证，2026-12-31） |
| T-7 Base 引 T1.4a Rulings | 头部 Base 改为 Rev 2.3 + 已 accepted 基线；withdrawn 文件不作契约 | 头部/§0.1 | Architect | FIXED |

---

## 5. 【假设与不确定性】（强制）

1. **0022 是否已 apply（B-3 关键假设）**：本文件按“0022 未被任何持久环境 apply”裁决就地修订。依据是 `scripts/verify-r01-migrations.sh:20-27` 尚无 0022、R0.4d2 未完成、仓库无真实日志。`[不确定]`：WIP 作者是否在个人/共享 DB 跑过；由 R0.4d2 的 `schema_migrations` 查询定案；若已有记录，**必须**改走 0023，本文件的 0022 修订裁决自动失效。
2. **MySQL 8.4 等值 CHECK 与语句时间**：假设同一 INSERT 内 `NOW(3)` 与 `DEFAULT CURRENT_TIMESTAMP(3)` 取同一语句时间，因此等值可精确成立。`[不确定]`：不同驱动/连接时区或显式 `SET timestamp` 下的行为；由 R0.4d2 真实 8.4 日志验证。
3. **ADD CHECK 锁行为**：`ALGORITHM=INPLACE, LOCK=NONE` 对 MySQL 8.4 ADD CHECK 的可用性与 MDL 时长 `[不确定]`；可能需要维护窗。未验证前不得宣称“在线迁移”。
4. **多组 P1 缺口的时间**：何时/是否由 P1 补上多组 release `[不确定]`；本文件只登记缺口与 owner，不承诺实现。补上之前，2 组订单的 INV-44 释放腿没有产品事实，AC-27 不能因此标 AC-VERIFIED。
5. **T4.4 交付时间**：`[不确定]`。因此 shadow 硬拒绝没有“到期自动解除”；解除必须同时满足 §2.1.1 的四条件，任何单方面删除都违约。
6. **ADR Rev 2.4 errata 未完成**：本文件与 accepted ADR §6.2 `:822`、§6.4 `:856`、§6.2 第 4 条 `:831` 存在文本冲突；在本文件被接受且 Rev 2.4 落地前，R0.4 范围以本文件为运行态契约，但这不是长期状态。
7. **WIP waiver 期限**：ADR/SPEC amendment 的免单独 human approve 于 P0 closure/merge decision 或 2026-12-31（取早）失效（`docs/review-tracking.md:187-191`）；本文件的人类接受不豁免，且过期后所有未接受的裁决需重新走批准。
8. **AC-27 状态**：`[确定]` AC-27 属 SPEC-0001 v1.4 且当前为 AC-REF（`ac-coverage.tsv:41`）；本文件不改变其状态，也不把 R0.4 的 schema 证据当作 AC 验收。
9. **registry 已校验全部字段**：`[确定]`（`registry.go:225-302,320-323`）；QC B-2 关于“validator 只校验 release_id”的描述被本文件校正如 §1.2。若 Reviewer 认为该校正有误，应以精确 HEAD 的代码复核为准。
10. **未执行声明**：本文件未执行 `go test`、`-race`、迁移或 Helm 渲染；文中代码/迁移引用是只读事实，不构成执行证据。未在本次核对的内容标 `[不确定]`。

---

## 6. 状态

- **Status: DRAFT 0.2 — pending QC round 2 and human acceptance**。未批准；不进入实现；不构成 AC-VERIFIED / merge / Release PASS。
- 本文件只改自己；未改 ADR Rev 2.3 / SPEC / 迁移 / 代码 / 台账。`0022` 的 SQL 是约束设计，不是本文件交付的迁移。
- 下游：QC round 2 → 人类接受（R0.4a3，记录于 `docs/review-tracking.md`）→ Reviewer 复审 `90b502c`（R0.4b/c）→ R0.4d1/d2 → R0.4e 日期化 verdict（`release_status=NOT RELEASE`）。任何一步 FAIL 都冻结 `PAY-SETTLE-01` 的 R0.4 范围，不得用新 `change_id` 绕过。
