# QC-PAY-SETTLE-01-R0.4-ADDENDUM — R0.4 C-04 Release Addendum DRAFT 红队审查

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| 任务 | R0.4a1（QC 红队审查；Lead closeout plan `docs/lead/PAY-SETTLE-01/tasks.md:29-36`，本行 `:30`） |
| 审查对象 | `docs/adr/0007-amd1-addendum-r0.4-c04-release.md`（Status DRAFT；Revision 0.1-draft r1） |
| **结论** | **BLOCKED**（Addendum 不可交人类接受：3 个 `[BLOCKER]` + 12 个 `[MAJOR]` + TODO） |
| 日期 | **2026-11-25** |
| Actor | `qc-r04`（Quality Checker；与 Architect / Developer / Reviewer 不同 actor） |
| base | `087298396aa7cb887e6cc9a38c5b033ec2c206b2`（R0.4a1 任务 base；Addendum 在此版本） |
| head | `111516f83147574d898bf4a44194ac2c28548d78`（本报告落盘时 HEAD；`0872983..111516f` 对 Addendum 为空 diff，被审对象未变；tasks/review-tracking/evidence-matrix 的引用行号已按 `111516f` 重钉） |
| code cross-check | `90b502c36a38df9538bbf647ed551f63fb049013`（WIP；base `9111740`）——只读核对契约-实现矛盾 |
| Tier | **L2**（跨服务事件 / 库存 / 迁移 / 启动门禁） |
| review_scope | 仅审 Addendum 的 C-04 release 判定与 R0.4 设计（shadow 启动门禁 / `order/0022` / 24h SLA）；只读核对 `90b502c` 是否与契约矛盾。不审 R0.3 台账、C-04 RefundBatch 正文、T2/T3/T4 实现。 |
| release_status | **NOT RELEASE / NOT RELEASABLE**；不批准 merge；不声称任何测试/迁移已执行 |

## 0. 结论与必读声明

**BLOCKED。** 三个阻断项：

1. **B-1 shadow 运行态与契约相反**：Addendum §2.1 允许 shadow 在存在批准行时启动，但 Director 2026-11-25 已裁决“硬拒绝到 T4.4”，`90b502c` 也确实拒绝启动；Addendum 未写运行态裁决、退出语义、T4.4 收口条件与 owner。
2. **B-2 C-04 release 契约来源失效**：`group_count`/`release_scope` 等**必填安全字段**只存在于已 withdrawn 的 T1.4b2 Addendum 与 Lead design 文档，accepted ADR 的 release 字段表不含 `group_count`；但 consumer 对 `group_count != 1` fail-closed。Addendum 把“契约”指向了不再有效的文件。
3. **B-3 24h SLA 只写了下限**：`expires_at >= entered_at + 24h` 允许 48h/任意更晚；WIP 测试明确把 48h 写成合法（`internal/services/order/r04_checks_integration_test.go:215-219`），与 INV-46“24h 到期必须离开 `OPEN/RESOLVING`、不得无限期 OPEN”不一致。

另有 12 个 `[MAJOR]`：断言已撤回的 B1 仍开、`baseline_impact` 无 INV、AC-27 归错 Spec、fresh-apply 顺序错、shadow range 与 ADR 自相矛盾、approval 状态无主、release DLQ 无契约引用、legacy bool 未入契约、0022 真实迁移证据路径缺失、失败后重跑流程误导、退出码/审计字段缺失、0022 枚举重复；以及 7 个 `[TODO]`。

**只读声明**：本报告未执行 `go test`、迁移或任何写操作；文中引用的 WIP 测试断言仅作为“代码事实”引用，**不构成执行证据**；不声称 Release PASS、不批准 merge、不代替 Reviewer/人类裁决。未在本次验证的内容标 `[不确定]`。

---

## 1. 必答崩溃窗口（5/5 已答）

| # | 窗口 | 预期行为与恢复所有者 | 证据 | 缺口 |
|---|---|---|---|---|
| 1 | **本地提交后崩溃**（准入事务已提交，release 事件未发出） | release 请求与准入/零元执行事实**同一事务**写 outbox；崩溃后由 Order outbox relay 认领重投；20 次失败 dead-letter 到 `outbox_dead_letters`；ops 按 `refund_id/group_no` 重放。Owner：Order（producer）+ ops（重放）；执行者 Inventory。 | `internal/services/order/refund_admission.go:283-287`（仅 PRE 同事务 append）、`:508-514`（零元同事务）；`cmd/order/main.go:33`（`outbox.Relay`）；`internal/platform/outbox/outbox.go:60-61,202-205,264-283`（lease/SKIP LOCKED/20 次/dead-letter）；`docs/ops.md:258-266` | Addendum 未写 owner、DLQ 重放入口与 7 天归档窗口（M-7）。 |
| 2 | **跨服务 RPC 成功后崩溃** | **N/A（无 RPC）**：C-04 release 的裁决就是“不新增 RPC”（Addendum §1.1），`90b502c` 无 proto/RPC 改动；启动门禁也只读、无远程调用。等价事件窗口（Inventory 已释放、未 ACK 即崩溃）由重投时 `statusReleased` 幂等 no-op 覆盖。 | `git show --stat 90b502c`（仅 config/fulfillment/cmd/migrations/tests，无 `api/**`）；`internal/services/fulfillment/settlement_switch.go:66-78`（只读）；`internal/services/inventory/service.go:171-186`（Release 幂等）；`internal/services/inventory/refund_release_consumer.go:97-99` | 若后续有人新增 release RPC，必须重答本窗口并补 RPC 幂等键；Addendum 应写明“本窗口因无 RPC 不适用”的依据。 |
| 3 | **消息重复/乱序** | 重复：同一订单只发一次 release 事件；重复投递按 reservation 状态机幂等（`RELEASED` no-op）；shadow 重复事件只 `last_seen_at`。乱序：release 事件不可能早于准入提交（同事务）；shadow 无准入权威，乱序不改变结论。 | `internal/services/order/refund_release.go:187-193`（`hasOrderReleaseEventTx` 去重）；`internal/services/inventory/refund_release_consumer.go:97-105`；`docs/adr/0007-payment-settlement-admission.md:843-848`（影子重复只更新） | 缺 reservation 行时**静默成功**且无 metric/告警（M-7）；Addendum 未写去重依据。 |
| 4 | **取消/确认竞态** | permit CAS 与 release append 在**同一事务**：Ship 胜（`CONSUMED`）走 `PENDING_REMOTE` + commitment，**不发 release**；PRE 败者回滚重判。胜者唯一（permit 行 CAS），败者无库存副作用。Owner：Order（admission/permit）；Fulfillment 只 `CancelIfCreated`。 | `internal/services/order/refund_admission.go:257-287`（result 判定 + permit CAS + append）、`:283-287`（仅 PRE append）、`:472-497`（CAS 败者重判）；`docs/adr/0007-payment-settlement-admission.md:712-716`（§5.7 owner/禁止事务内远程调用） | shadow 模式无写路径（B-1），影子期不存在本竞态；Addendum 未声明。 |
| 5 | **外部重放缺失** | release：**N/A + 依据**——Order→Inventory 内部事件，无支付/承运商等外部渠道；恢复由 outbox + consumer DLQ 自驱动。24h 历史队列：Payment 是外部渠道；若 Payment 不再重放，必须由 Order expiry/compensation worker 驱动（ADR §9 W10）；但 T2.2/T2.4 未实现。 | `docs/adr/0007-payment-settlement-admission.md:967`（W13）、`:964`（W10）；`docs/lead/PAY-SETTLE-01/tasks.md:69-71`（T2.2/T2.4 未勾选）；`docs/lead/PAY-SETTLE-01/evidence-matrix.md:63`（INV-46 DEFERRED） | 0022 单独**不能**保证“不无限期 OPEN”；Addendum §2.3 把它写成已定义状态机，但驱动方无实现/无 owner（M-6/M-9）。 |

---

## 2. 逐条漏洞

### `[BLOCKER]` B-1 — shadow 运行态裁决缺失，且与 Director 裁决/实现相反

- **证据**
  - Addendum `docs/adr/0007-amd1-addendum-r0.4-c04-release.md:54`：shadow 只要“存在 `target_mode='shadow' AND status='APPROVED'` 的批准行……release_range 按部署范围校验”即通过；`:61` 只说“未通过不得启动 worker”，**没有“通过后仍拒绝启动”**。
  - 实现 `cmd/fulfillment/main.go:63-70`：门禁通过后，`mode == shadow` **无条件** `os.Exit(1)`，直到 T4.4；日志只有 `mode`，无 `approval_id`/稳定 reason。
  - 人类裁决 `docs/review-tracking.md:350-359`：Director 2026-11-25 决定 shadow **hard-rejected at startup until T4.4**，并要求“**Addendum must record the runtime ruling, the fail-closed exit semantics and the T4.4 exit condition**”。
  - accepted ADR `docs/adr/0007-payment-settlement-admission.md:831` 与 `:856` 仍把 shadow 写成可启动/可执行的 OFF→SHADOW 步骤，因此这是**契约-裁决-实现三方矛盾**，不是单纯文档笔误。
- **具体修改**（二选一，推荐 A）
  - **A（R0.4 范围内）**：在 Addendum 新增 §2.1.1「shadow 运行态裁决」：
    > “WHEN 启动请求 `FULFILLMENT_SETTLE_CONSUMER_MODE=shadow`，THE SYSTEM SHALL 无论批准行校验是否通过，都在注册任何 consumer 之前**拒绝启动**（非零退出，稳定 reason=`T4_4_SHADOW_NOT_DELIVERED`），不得回退 `paid`；日志 MUST 含 `mode/approval_id/reason`。本拒绝是 Director 2026-11-25 的 R0.4 scoped 裁决，owner=Developer(T4.4)+Architect(contract)；解除条件=T4.4 双消费 observe-only 交付 + Reviewer 证据 + Director 重新批准。解除前运维 MUST NOT 写入 shadow 批准行。”
  - 同时在 §2.4 R0.4b 验收加一条：shadow 启动必须在 consumer 注册前非零退出，并断言日志三字段（测试放 `cmd/fulfillment/main_test.go`）。
  - 路由 Architect：ADR §6.4 步骤 1（`:856`）加“实现约束：T4.4 前不可执行”的 errata（Rev 2.4 或本 Addendum accepted 后回写），否则 accepted ADR 与实现长期冲突。
  - **B（不推荐，超 R0.4 范围）**：T4.4 立即实现 observe-only 双消费；R0.4 只保留门禁。需改范围并重排 Plan。
- **不修复的后果**：Director 按 ADR §6.4 步骤 1 批准 shadow → 生产 Pod crash-loop；Addendum 却写成“可启动”，Reviewer 无法按契约判 PASS。

### `[BLOCKER]` B-2 — C-04 release 的“契约”指向已 withdrawn 文档，`group_count` 无 accepted 契约

- **证据**
  - Addendum `:39`：“这些在 T1.4b2/Release event 契约中”，`:34` 也以 T1.4a 为据。
  - `docs/adr/0007-payment-settlement-admission.md:19` 与 `:1607`：`0007-amd1-addendum-t14b2-inv44-group.md` 已 **superseded / withdrawn（2026-11-25）**，“其 B1/B2/B3 不并入本 ADR”，由 **Lead design** `docs/lead/PAY-SETTLE-01/t1.4b2-design.md` Rev 1.3 取代；而 `docs/collab/CONVENTIONS.md:11` 明确“设计结论只活在 ADR”，Lead 文档不是契约。
  - accepted ADR 的 release 字段表：`docs/adr/0007-payment-settlement-admission.md:204`（`refund_id/order_no/group_no/items[]/warehouse_id + source/reason`）、`:724`（同）、`:940`（`release_id=rr:<refund_id>`）；**没有 `group_count`/`release_scope`/`covered_group_nos`**。
  - 但代码把它们当**必填安全字段**：`internal/platform/events/events.go:79-100`、`internal/services/inventory/refund_release_consumer.go:65-70`（`group_count != 1` → Permanent/DLQ，不释放）；validator `internal/platform/events/registry.go:243-245` 只校验 `release_id`。
  - 多组订单：`internal/services/order/refund_release.go:179-187` 只 `log.Warn("multi-group refund keeps inventory reservations (P1/manual)")`，**不发 release**、无 outbox；Addendum `:39` 只写“P0 单组 `group_count=1`”，未写多组不释放及其对 INV-44 的影响。
- **具体修改**
  1. Addendum §1.2 增一行「Release event v1 契约（本 Addendum 定义）」并给出字段表：
     `schema_version/release_scope=order_reservation_full/group_count(必填=1)/covered_group_nos(可选审计)/release_id=rr:<refund_id>/refund_id/order_no/group_no/source/reason/items[{sku_id,quantity,warehouse_id}]/requested_at`；并写“consumer MUST 对 `group_count != 1`、缺失 `warehouse_id`、`release_id` 不匹配走 DLQ 且不释放”。
  2. 同一节写 P0 边界与 deferral：“多组订单不发 release，reservation 保持（P1/人工）；这是 INV-44 的已知缺口，owner=Developer，expiry=2026-12-31，tracking=`review-tracking/PAY-SETTLE-01`”；并在 `docs/lead/PAY-SETTLE-01/evidence-matrix.md` 补 INV-44 行（现只有 INV-45..50，见 `:62-67`）。
  3. 把 `:39` 的 T1.4b2 引用改为 `ADR §2.4:204 / §5.7:724 / §8:940,967` + 本 Addendum 小节号；如坚持在 ADR 正文出现 `group_count`，由 Architect 在 Rev 2.4 补字段（否则契约仍在 code-only 状态）。
- **不修复的后果**：R0.4a 宣称“C-04 release 契约已裁决”，但可审计契约里没有 consumer 依赖的必填字段；下一个实现者可能删字段或以 `group_no` 替代，直接触发 fail-closed 全量 DLQ 或误释放。

### `[BLOCKER]` B-3 — 24h SLA 的 CHECK 只是下限，48h/任意更晚通过

- **证据**
  - Addendum `:90-91` 写 `expires_at >= DATE_ADD(entered_at, INTERVAL 24 HOUR)`；`:94` 说“writer 必须**在 DB 时间上写入 24h 到期**”；`:100` 只要求测 `=24h` 通过、`<24h` 失败。
  - `migrations/order/0022_r04_checks.sql:55` 同下限；WIP 测试 `internal/services/order/r04_checks_integration_test.go:215-219` 明确 **“48h passes”**（`ORD-LPR-48H` 期望通过）。
  - INV-46 `docs/specs/0002-payment-settlement-admission.md:130`：“24h 到期必须离开 `OPEN/RESOLVING`……不得无限期 `OPEN`”；worker 只按 `expires_at <= NOW(3)` 到期（Addendum `:96`）。因此一个 48h/100 年的 `expires_at` 会让队列**合法地**长期 `OPEN`。
  - Addendum `:94` 的“writer 必须在 DB 时间写 24h”落在 **Order 服务**（`internal/services/order/`），不在 `:12` 的 `allowed_paths` 内 → 该要求无 owner/无落点。
- **具体修改**
  1. 0022 改成**精确 24h（含容忍带）**，二选一：
     - 精确：`CHECK (expires_at = DATE_ADD(entered_at, INTERVAL 24 HOUR))`，writer 用同一条 `INSERT ... VALUES (..., NOW(3), DATE_ADD(NOW(3), INTERVAL 24 HOUR))`（MySQL 同语句 `NOW(3)` 一致）；或
     - 容忍带：`CHECK (expires_at >= DATE_ADD(entered_at, INTERVAL 24 HOUR) AND expires_at <= DATE_ADD(entered_at, INTERVAL 24 HOUR + INTERVAL 5 MINUTE))`。
  2. 0022 precheck 查询改为 `WHERE expires_at <> DATE_ADD(entered_at, INTERVAL 24 HOUR)`（或超出容忍带），并保留诊断输出。
  3. 测试把 `ORD-LPR-48H` 从“通过”改成“失败”，新增 `24h+1s/25h` 拒绝用例；删掉 `r04_checks_integration_test.go:215` 的 “48h passes” 注释。
  4. Addendum `:94` 的 writer 规则二选一：把 `internal/services/order/` 加入 `allowed_paths`（owner=Developer，T2.4），或把“DB 时间写 24h”移到 T2 契约并声明 R0.4c 只交付 CHECK、不交付 writer。
- **备注**：即使 writer 正确，CHECK 是唯一 DB 兜底；现在兜底对“晚设 deadline”完全放行，属于数据完整性/资金 SLA 阻断项，不得降级。

### `[MAJOR]` M-1 — Addendum 仍要求 Reviewer “撤回 B1”，但 B1 已于 R0.2-R2 撤回、C-04 contract PASS

- **证据**：Addendum `:22`（“Reviewer B1……越过 accepted ADR”）、`:40-41`（“Reviewer 应撤回/降级 B1”“请 Reviewer 在 R0.2 verdict 中撤回 B1”）、`:106`（R0.4a 仍写“依 ADR §2.4/§5.7 撤回 RPC 要求”）；但 `docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.2-R2.md:29-30`（C-04 contract PASS、B1 撤回）与 `:59`；`docs/lead/PAY-SETTLE-01/tasks.md:21` 已把 R0.4 改为“release event/schema/consumer 验证（无 RPC）”。
- **修改**：把 §1.1/§1.3/§2.4 R0.4a 改写为“B1 已于 2026-11-25 的 REV-R0.2-R2 撤回并降级为 Plan 文案；C-04 contract PASS；本 Addendum 只固化 release event 契约与 no-RPC 边界，不要求 Reviewer 再次撤回”。删除“越界”指控；引用 `REV-PAY-SETTLE-01-R0.2-R2.md:29-30,59` 与 `tasks.md:21`。
- **影响**：不修则 accepted Addendum 记录一个不存在的 open 争议，R0.4e 复审会以错误前提重开 C-04。

### `[MAJOR]` M-2 — `baseline_impact` 未列任何 INV，违反 CONVENTIONS §3

- **证据**：Addendum `:13` 只有一句影响描述；`docs/collab/CONVENTIONS.md:137` 要求“无论哪种，都必须列出受影响的 `INV-xx`……金额、库存、权限、履约准入相关变更不得只写‘不涉及’”。本变更触及库存释放（INV-44）、历史队列 24h（INV-46）、shadow 门禁（INV-49）、零元同一 release 路径（INV-50）。
- **修改**：`:13` 改为 `baseline_impact: INV-44（release event/schema，P0 单组）、INV-46（24h CHECK，仅下限→需精确化）、INV-49（shadow 启动门禁）、INV-50（零元与正金额同一 release 路径）；INV-45/47 仅 0022 CHECK 不改变语义；C-04 RefundBatch 不变`。

### `[MAJOR]` M-3 — Spec-Ref 把 AC-27 归到 SPEC-0002 v1.10

- **证据**：Addendum `:11` 写 “SPEC-0002 v1.10 ... AC-27/AC-50/…”。AC-27 属于 **SPEC-0001**：`docs/specs/0001-fulfillment-quantity-split.md:571-579`（v1.4 EARS，`:21`/`:696` 变更记录）；SPEC-0002 只在 D-17/§14 引用 AC-27（`docs/specs/0002-payment-settlement-admission.md:694`、`:724`），不定义它。
- **修改**：`:11` 改为 `Spec-Ref: SPEC-0002 v1.10 INV-44…50、AC-50/AC-62/AC-65/AC-66；SPEC-0001 v1.4 AC-27`。并注明 AC-27 目前是 **AC-REF 不是 AC-VERIFIED**（`docs/collab/ac-coverage.tsv:41`）。

### `[MAJOR]` M-4 — §2.2 的 fresh-apply 顺序错误/不完整，且混用两个 DB 的迁移号

- **证据**：Addendum `:80` 写“fresh apply 顺序必须为 `0011`→`0017`→`0018`→`0019`→`0022`”；但 order DB 实际还有 `0020_permit_state_check.sql`、`0021_zero_application_check.sql`（`migrations/order/`），fulfillment DB 另有 `0010_settlement_shadow_switch.sql`、`0011_switch_expiry_check.sql`（`migrations/fulfillment/`）。R0.1 脚本只列 order `0017-0020` + fulfillment `0010-0011`（`scripts/verify-r01-migrations.sh:20-27`），且**尚无 0021/0022**（`tasks.md:35` R0.4d2 未做）。
- **修改**：`:80` 改成按 DB 分列：order `0017→0018→0019→0020→0021→0022`；fulfillment `0010→0011`；并把 `:65`“不改已合入 0017/0018/0019”扩为 `0017/0018/0019/0020/0021`。同时把 0022 加入 `scripts/verify-r01-migrations.sh` 的 `MIGRATION_FILES` 与对象断言（R0.4d2）。

### `[MAJOR]` M-5 — shadow 的 `release_range` 要求与 accepted ADR 自相矛盾，且 approval 默认为空串

- **证据**：Addendum `:54` 要求 shadow 校验 range；ADR §6.2 第 4 条（`docs/adr/0007-payment-settlement-admission.md:831`）**不要求** shadow range；ADR §6.3（`:848`）与 §12.5.6（`:1285`）又要求 range；schema 默认 `release_range=''`（ADR `:1261`、`migrations/fulfillment/0010_settlement_shadow_switch.sql:42`）。代码要求部署 range 非空（`internal/platform/config/config.go:775`）且与批准行**精确相等**（`internal/services/fulfillment/settlement_switch.go:95,148`）。
- **后果**：Director 若按 ADR §6.2 第 4 条创建不带 range 的 shadow 行（ADR-valid），会被代码拒绝；shadow 因此有**两个**独立不可启动原因（range + T4.4）。
- **修改**：Addendum §2.1 明确裁决“shadow 也要求 `release_range` 非空且匹配”，并注明该裁决解决 ADR §6.2 第 4 条 vs §6.3 的冲突，由 Architect 在 Rev 2.4 修 errata；同时要求 shadow/settled 批准行 `release_range <> ''`（新增 fulfillment 迁移 CHECK 或 ops 写入模板），并在 `docs/ops.md` 写写入模板。补一条“门禁校验错误优先于 T4.4 拒绝”的语义（当前代码顺序如此：`main.go:43-70`）。

### `[MAJOR]` M-6 — 批准行状态机死胡同/无主：`EXPIRED` 永不写入，运行中的 settled 不受到期约束

- **证据**：Addendum `:58` 只写“无运行期热切换；到期只使下次启动失败”；`:60` 写应用只读；`settlement_switch_approvals.status` 枚举含 `EXPIRED`（ADR `:1268-1271` 区域，`migrations/fulfillment/0010_settlement_shadow_switch.sql` 同），但**没有任何写者**（应用无写权限、无 job）；ADR §6.2 第 9 条（`:836`）也明确运行期不轮询 `expires_at`。
- **后果**：过期的批准行仍 `status='APPROVED'`，按 status 过滤的看板会显示“有效”；运行中的 settled 实例在 `expires_at` 后继续运行，唯一的停止手段是 Director 回滚 + 重启，但 Addendum 未写该 owner/补偿控制。
- **修改**：在 §2.1 增加：
  > “到期是**启动窗口**语义：运行中的进程不会因 `expires_at` 到期自动停止；唯一停止路径是 ADR §6.4 的 Director 批准回滚 + 重启。`status='EXPIRED'` 不由应用写入（应用只读），看板/告警 MUST 以 `expires_at <= NOW(3)` 判定过期，不得依赖 `status`。ops MUST 配置 `target_mode='settled' AND status='APPROVED' AND expires_at <= NOW(3)` 告警；过期仍运行的实例由 Director 决定回滚，owner=ops。”
  > 若要求 `status=EXPIRED` 可达，另立受控 ops 迁移/任务并写 owner，不得留给“以后”。

### `[MAJOR]` M-7 — release 的 DLQ/重放与“缺 reservation 静默成功”无契约、无观测

- **证据**：Addendum `:39` 只写“consumer 重复/DLQ”，没有 topic/DLQ subject/重放 owner；`internal/platform/outbox/outbox.go:60-61,264-283`（20 次后 dead-letter）、`docs/ops.md:267-275`（`<subject>.dlq` 检查/重放）；consumer 对**缺失 reservation** 直接 `continue`（`internal/services/inventory/refund_release_consumer.go:92-96`），无 metric/告警；ADR §9 W13（`:967`）只给人入口 `refund_id/group_no`，无具体命令。
- **修改**：Addendum 增 §1.4：
  > “release topic=`shop.inventory.refund_release_requested.v1`；DLQ=`…v1.dlq`；producer outbox 20 次失败归档 `outbox_dead_letters`（30 天/7 天 prune），consumer `MaxDeliver=20`；重放 owner=ops（ADR §9 W13），步骤=`docs/ops.md:267-275`。缺失 reservation 是幂等 no-op，但 MUST 计数/告警（新增 metric）并记录 `release_id/order_no/sku/warehouse`，不得完全静默；event 不存在时不得 ack 前 CA。”
  P0 单组约束下，多组订单无 release 的告警也必须写进本契约（当前只有 `refund_release.go:179-187` 日志）。

### `[MAJOR]` M-8 — legacy 布尔开关未进入 Addendum §2.1 矩阵，但代码实现了它

- **证据**：代码 `internal/platform/config/config.go:736-760` 把 `FULFILLMENT_CONSUME_ORDER_SETTLED=true` 映射为 `settled`，且与 `FULFILLMENT_SETTLE_CONSUMER_MODE` 冲突时 fail-fast；accepted ADR §6.2 第 7 条（`:834`）明确保留该兼容。Addendum `:53-55` 的三行矩阵**没有** legacy bool 行，`:59` 的 N5 也只覆盖 `approval_id`。
- **修改**：在 §2.1 矩阵加一行“旧布尔 `true`（mode 未设置）→ 视为 `settled`，必须同时有 approval id；缺/空/冲突 → 非零退出（N5）；新旧冲突 → fail-fast”，并引用 ADR `:834`。否则 R0.4b 的 legacy 行为属于“契约外实现”。

### `[MAJOR]` M-9 — 0022 的真实迁移证据路径与 INV-44 证据行缺失

- **证据**：Addendum `:108` 验收写“真实迁移 PASS；错误行失败”；但 canonical 脚本 `scripts/verify-r01-migrations.sh:20-27` 未含 0021/0022（`tasks.md:35` R0.4d2 未做）；WIP `90b502c` 的测试日志只在 commit message，仓库内无 artifact；`docs/lead/PAY-SETTLE-01/evidence-matrix.md:64-69` 只有 INV-45..50，**没有 INV-44 行**（而 C-04 release 就是 INV-44）。
- **修改**：Addendum §2.4 的 R0.4c 验收改为“`scripts/verify-r01-migrations.sh` 含 `order/0021,0022` 且真实 MySQL 8.4 日志绑定 HEAD（owner R0.4d2/Developer）”；并在 evidence-matrix 增 INV-44 行（owner/expiry/evidence=release consumer 测试 + 0022 + 真实迁移日志）。在修复前，R0.4c 只能算“SQL 已写”，不能算“迁移已验证”。

### `[MAJOR]` M-10 — 0022 失败后“重跑安全”的说法与 runner 行为不符

- **证据**：`migrations/order/0022_r04_checks.sql:11-17` 声称“conditional ALTERs make a re-run after cleanup safe”；但 runner 对 `FAILED` attempt **拒绝重放**（`scripts/test-migrate-runner.sh:49-53`，`scripts/verify-r01-migrations.sh:271` 断言的正是 “previous FAILED attempt”）。脏行场景下 0022 的 ALTER 失败 → attempt=FAILED → 清理数据后直接重跑仍会失败，必须先删 `schema_migration_attempts` 行。
- **修改**：二选一：(a) 采用 `migrations/fulfillment/0011_switch_expiry_check.sql:22-40` 的 `SIGNAL` precheck 模式，让“脏行=明确失败”并给出可操作错误；(b) 在 0022 头注释与 ops runbook 明确写“失败后操作步骤：修数据 → `DELETE FROM schema_migration_attempts WHERE filename='order/0022_r04_checks.sql'` → 重跑”，并删除“re-run after cleanup safe”的无条件表述。Addendum `:80`/`:100` 同步写该流程。

### `[MAJOR]` M-11 — 启动失败退出码/审计字段不满足 Addendum 自身要求

- **证据**：Addendum `:58` 要求“启动失败必须非零退出、记录 `approval_id/mode/reason`”；实现所有失败都 `os.Exit(1)`（`cmd/fulfillment/main.go:30,38,57,69,74,87,110`），shadow 拒绝日志只有 `mode`（`:65-70`），无 `approval_id/reason` 稳定码。
- **修改**：定义稳定退出码（如 2=config/N5、3=approval rejected、4=shadow-not-implemented）或稳定 `reason` 字段；shadow 拒绝日志补 `approval_id`（门禁已解析出 `approval.ApprovalID`）与 `reason=T4_4_SHADOW_NOT_DELIVERED`；在 `cmd/fulfillment/main_test.go` 断言退出码与日志字段。否则 K8s CrashLoopBackOff 无法区分“门禁拒绝”和“DB 连不上”。

### `[MAJOR]` M-12 — 0022 重复 0017 的 source 枚举，未来改枚举要双改

- **证据**：`migrations/order/0017_refund_batch_ledger.sql:73-74` 已有 `chk_refund_app_source` 封闭枚举；`migrations/order/0022_r04_checks.sql:34` 在判别 CHECK 里**再次复制**同一枚举。
- **修改**：判别式改为不复制枚举：`CHECK ((group_no IS NULL) = (source IN ('payment','legacy_expired')))`，由 0017 的封闭枚举兜底 source 合法性；若保留复制，加注释“新增 source 必须同步 0017 与 0022”，并把它列进迁移 checklist。另：0022 的 `information_schema` 跳过逻辑不校验约束文本，同名不同义的约束会被静默接受——加 `CHECK` 文本校验或注明“只允许同名同义”。

### `[TODO]` 清单

- **T-1** 报告文件名：`tasks.md:30` 写 `QC-R04-ADDENDUM.md`，本文件按 R0.4a1 brief 命名为 `QC-PAY-SETTLE-01-R0.4-ADDENDUM.md`；Lead 需对齐 Plan 指针（不阻塞）。
- **T-2** Addendum `:14` “不写代码/迁移/测试”与 §2.2/§2.3 给 DDL 冲突；改成“本文件只给设计约束，不交付迁移文件/代码”。
- **T-3** Addendum `:129` “C-05 0022 为 R0.4 草案”含义不明；明确 0022 由 R0.4c（Order DB）交付，C-05 的 0017/0018/0019/0021 只读。
- **T-4** `settlement_switch_approvals` 多行匹配规则未写；代码取 `approved_at DESC` 的第一条合格行（`settlement_switch.go:124`），Addendum 应写“多行时以 approved_at 最新且全部条件满足者为准，审计记录该 approval_id”。
- **T-5** `docs/ops.md` 与 `deploy/` 均无 `SETTLE_RELEASE_RANGE`/`FULFILLMENT_SETTLE_CONSUMER_MODE`（grep 为零）；属 R0.4d1，不阻塞 Addendum 接受，但阻塞 shadow/settled 激活。
- **T-6** 0022 的 `ALTER` 未写 `ALGORITHM/LOCK`；MySQL 8.4 加 CHECK 会扫描全表，`refund_applications` 大表需维护窗/显式 `ALGORITHM=INPLACE, LOCK=NONE`（若支持），并写明 precheck 的 `LIMIT 100` 只诊断、需循环到 0 行。
- **T-7** Addendum `:10`/`:31` 的 Base 仍引 T1.4a Rulings；该文件已被 ADR Rev 2.3 取代（`docs/adr/0007-payment-settlement-admission.md:10-12`），改为引用 Rev 2.3 的合并条款。

---

## 3. 状态机死胡同 / 无主（专项）

| 状态/路径 | 进入后由谁驱动 | 结论 |
|---|---|---|
| `settlement_switch_approvals.status=EXPIRED` | 无人写入（应用只读、无 job） | **死状态**；只能靠 `expires_at` 时间比较（M-6） |
| 已 `settled` 运行中、`expires_at` 到期 | 无运行期轮询（ADR §6.2 第 9 条） | 只能 Director 回滚 + 重启；Addendum 未写 owner/告警（M-6） |
| shadow 批准行已存在 | 无 consumer（T4.4 未交付），启动硬拒绝 | 批准行“不可执行”；Addendum 未写（B-1） |
| `legacy_payment_resolution` `OPEN/RESOLVING` 24h 到期 | 设计上 Order expiry/compensation worker；T2.2/T2.4 未实现 | **无主**；0022 只有 CHECK，不驱动状态（W5/M-9） |
| 多组订单退款 → INV-44 释放 | 只有 `log.Warn`，无 outbox、无重试、无人工队列 | **静默缺口**；必须登记 deferral + INV-44 evidence（B-2/M-9） |
| release event 进入 DLQ | ops 手工重放（`docs/ops.md:267-275`） | 有入口但 Addendum 未引用 owner（M-7） |
| 孤儿/账外：0022 约束被同名不同义占用 | 无检测（conditional 只看存在性） | [TODO] M-12 备注 |

---

## 4. 哪些阻塞、哪些不阻塞 R0.4 收口

**阻塞 Addendum 接受（必须改完才可 READY）**：B-1、B-2、B-3、M-1、M-2、M-3、M-4、M-5、M-6、M-8、M-11。
**阻塞 R0.4 收口（可并行、已有 Plan 任务）**：M-9（R0.4d2 迁移脚本/日志 + INV-44 evidence 行）、M-10（失败重跑流程）、M-12（0022 枚举）、T-5（R0.4d1 部署接线）、T-6（大表 DDL 维护窗）。
**不阻塞 R0.4 收口（属 R0.3/R0.5 或后续 release 门禁）**：AC-27 仍 AC-REF、回归 AC-14/21/25/27/31/32 deferred、12 orphan、INV-45..50 evidence 描述缺口（R0.3 已记录，`REV-PAY-SETTLE-01-R0.3.md:1-96`）；多组 P1 人工处理（登记 deferral 后不阻塞 R0.4b/c 代码，但阻塞“INV-44 已完成”声明）；`docs/ops.md`/`deploy/**` 接线（R0.4d1，阻塞激活不阻塞代码正确性）。
**不阻塞且不应在 R0.4 处理**：T3 commitment release、T4.4 shadow 实现、T2 历史队列 worker——Addendum 已正确划界（`:112-124`），不得把 T3/T2 塞进 R0.4。

---

## 5. 已验证成立的部分（避免只列漏洞）

- **C-04 release = event/outbox、无新增 RPC** 的实质裁决正确：ADR `:204`、`:724`、`:940`、`:967`；SPEC-0002 `:128`（INV-44）；`90b502c` 无 `api/**` 改动（`git show --stat`）。Addendum 的结论方向对，问题在契约来源与 P0 边界（B-2）。
- **0022 是 additive**：`git show --stat 90b502c` 只新增 `migrations/order/0022_r04_checks.sql`，未改 0017/0018/0019；0019 的 `chk_lpr_expiry` 保留（`internal/services/order/r04_checks_integration_test.go:164-172`）。无 down migration 与 ADR §10 一致。
- **脏行 fail-closed 成立**：0022 的 `ALTER` 是硬停（error 3819），runner 还会把 attempt 标 FAILED 并拒绝重放（M-10 只要求补流程文字）。
- **启动门禁语义基本正确**：只读（`settlement_switch.go:66-78`）、用 DB `NOW(3)` 判到期（`:123`、`:176`）、无 `paid` fallback、N5 在 config（`config.go:775`）与 service（`settlement_switch.go:95,106`）双层；paid/settled 两行与 Addendum 一致。
- **窗口 4 无副作用**：release 只在 PRE 分支、与 permit CAS 同事务（`refund_admission.go:272-287`）；POST 不 append（`:283-287`）；零元同路径（`:508-514`）。
- **release 幂等**：事件每 order 一次（`refund_release.go:187-193`）+ reservation 状态机幂等（`service.go:171-186`、`refund_release_consumer.go:97-99`），重复/乱序收敛。

---

## 6. 边界、`[不确定]` 与角色声明

- 本报告为 **QC 红队审查**，不是 Reviewer verdict；不批准 merge/Release，不代替人类决定。
- **未执行**：`go test`、`-race`、迁移、Helm 渲染；因此 WIP commit message 中的测试/迁移结果**未被本报告复核**，标 `[不确定]`。
- `[不确定]`：0022 在真实 MySQL 8.4.13 上的 MDL/锁时长与 add-CHECK 行为（需 R0.4d2 日志）；`90b502c` 的 integration test 是否在 CI 环境实际可跑（需 Reviewer 绑定 HEAD 复核）。
- 只改了本文件；未改 Addendum/ADR/SPEC/tasks/台账/代码；未 commit。
- 下游：R0.4a2（Architect 按 B-1/B-2/B-3 + M 项改 Addendum 并置 accepted）、R0.4a3（人类接受）、R0.4b/c（Reviewer 复审 `90b502c`）、R0.4d1/d2（部署/迁移证据）、R0.4e（Reviewer 日期化 verdict，release_status=NOT RELEASE）。
