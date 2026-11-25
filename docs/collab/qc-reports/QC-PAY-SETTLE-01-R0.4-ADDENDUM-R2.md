# QC-PAY-SETTLE-01-R0.4-ADDENDUM-R2 — R0.4 C-04 Release Addendum DRAFT 0.2（r2）第二轮红队复核

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| 任务 | R0.4a2 复核（第二轮 QC；第一轮 `QC-PAY-SETTLE-01-R0.4-ADDENDUM.md`，commit `362b9e2`） |
| 审查对象 | `docs/adr/0007-amd1-addendum-r0.4-c04-release.md`（Status **DRAFT 0.2**；Revision 0.2-draft r2，commit `e3998d3`） |
| **结论** | **BLOCKED — 不可交人类接受**：2 个 `[BLOCKER]` + 7 个 `[MAJOR]` + TODO；B-1/M-1/M-2/M-3/M-6/M-8/M-10/M-12 已关闭（M-6 契约关闭、ops 告警 ROUTED），B-2/B-3/M-4/M-5/M-7/M-9/M-11 仍 PARTIAL |
| 日期 | **2026-11-25** |
| Actor | `qc-r04-r2`（Quality Checker；与 Architect / Developer / Reviewer 不同 actor） |
| base | `e3998d3ccfc17268a60a46e5928ec496353dfb25`（被审 Addendum r2 落盘） |
| head | `e3998d3ccfc17268a60a46e5928ec496353dfb25`（`git rev-parse HEAD`） |
| code cross-check | `90b502c36a38df9538bbf647ed551f63fb049013`（HEAD 祖先；只读核对契约-实现一致性） |
| Tier | **L2**（跨服务事件 / 库存 / 迁移 / 启动门禁） |
| review_scope | 只审 Addendum r2 的第一轮 B-1/B-2/B-3 + M-1..M-12 关闭情况、新矛盾/死胡同/越界，以及 5 个崩溃窗口；只读核对 `90b502c`/HEAD 的契约-实现。不审 R0.3 台账正文、C-04 RefundBatch 正文、T2/T3/T4.4 实现。 |
| release_status | **NOT RELEASE / NOT RELEASABLE**；不批准 merge；不声称任何测试/迁移已执行；不声称 AC-VERIFIED |

## 0. 结论与必读声明

**BLOCKED。** 第一轮 3 个 `[BLOCKER]` 中：

- **B-1 已关闭（契约）**：shadow 运行态裁决、fail-closed 退出语义、T4.4 收口条件与 owner 已写入 `Addendum:149-162`，与 Director 2026-11-25 裁决（`docs/review-tracking.md:350-358`）及 `cmd/fulfillment/main.go:63-70` 一致；R0.4b 的退出码/日志字段实现仍开（见 M-11），但不再是“运行态裁决缺失”。
- **B-2 仅 PARTIAL**：release event v1 字段表/consumer/DLQ/多组显式缺口已写入 `Addendum:58-124`，但多组与 missing-reservation 的 metric 落在 `internal/services/order/`、`internal/services/inventory/`，均不在 `Addendum:15` 的 `allowed_paths`；且被误挂到 R0.4b 的 Fulfillment 证据（N-2，BLOCKER）。INV-44 canonical evidence 行仍缺（N-8）。
- **B-3 仅 PARTIAL，且新增 BLOCKER**：24h 等值 CHECK 与 writer 推 T2 的方向正确，但 0022/0023 决策门被放在 **R0.4d2 开工前**（`Addendum:236-238`），而 **R0.4c 已就地编辑 0022**（`Addendum:274`）；若任一持久环境已 apply 0022，该顺序先违反“已 apply 文件不可变”，再到 R0.4d2 才发现（N-1，BLOCKER）。

**2 个新 `[BLOCKER]`**：N-1（B-3 的 0022/0023 门顺序）、N-2（M-7 的 metric 越出 `allowed_paths` 且任务/证据错位）。
**7 个 `[MAJOR]`**：N-3（R0.4b/c 任务 ID 与 Lead Plan 冲突）、N-4（M-4 的 `0021` 未进 Plan/验证脚本）、N-5（M-5 的 `release_range <> ''` 无落点/期限）、N-6（M-11 的 config/N5 仍是 panic，无三字段）、N-7（B-2 的 T2 24h 补偿 release source 未定义）、N-8（M-9/INV-44 evidence 行仍未落 canonical 台账）、N-9（窗口 5 的 expiry/compensation worker 未在 Addendum 内显式归属）。

**只读声明**：本报告未执行 `go test`、`-race`、迁移或 Helm 渲染；文中代码/迁移引用是只读事实，**不构成执行证据**；不声称 Release PASS、不批准 merge、不代替 Reviewer 或人类裁决。未在本次核对的内容标 `[不确定]`。

---

## 1. 第一轮问题关闭表

> 状态：**CLOSED** = 本 Addendum 内已形成可执行契约/裁决；**PARTIAL** = 只关闭一部分或落点/证据仍缺；**OPEN** = 未处置。`Addendum` = `docs/adr/0007-amd1-addendum-r0.4-c04-release.md`；`R1` = 第一轮报告 `docs/collab/qc-reports/QC-PAY-SETTLE-01-R0.4-ADDENDUM.md`。

| 第一轮 | 第二轮处置 | 证据（文件:行号） | 残余 / Owner |
|---|---|---|---|
| **B-1** shadow 运行态裁决 | **CLOSED（契约）** | `Addendum:149-162`；`docs/review-tracking.md:350-358`；`cmd/fulfillment/main.go:43-70`；`internal/services/fulfillment/settlement_switch.go:79-115` | R0.4b 仍须补 exit 3/4、`approval_id/reason` 日志（M-11/N-6）；T4.4 收口条件已写 |
| **B-2** release 契约来源/`group_count` | **PARTIAL** | `Addendum:58-124`；`internal/platform/events/registry.go:225-302,320-323`；`internal/services/inventory/refund_release_consumer.go:56-85`；`internal/services/order/refund_release.go:178-213`；`docs/review-tracking.md:187-191,345`；`docs/lead/PAY-SETTLE-01/evidence-matrix.md:65` | 字段表/消费约束可执行；批准路径**合法**（WIP waiver `review-tracking.md:187-191` 免单独 human approve；`Addendum:81` 保留 QC + Reviewer + R0.4a3 human；Reviewer 顺序见 T-11）；但 metric 越界（N-2）、INV-44 evidence 行缺（N-8）、T2 legacy source 未定义（N-7） |
| **B-3** 24h SLA 等值 CHECK | **PARTIAL（N-1 阻断）** | `Addendum:211-240`；`migrations/order/0022_r04_checks.sql:38-57`（当前仍是 `>=`，`:55`）；`internal/services/order/r04_checks_integration_test.go:215-219`（当前 48h 仍期望通过）；`migrations/order/0019_legacy_payment_resolution.sql:51` | 等值/无容忍带/writer 推 T2 方向正确；0022/0023 决策门顺序错（N-1）；R0.4c 尚未改 0022 |
| **M-1** 撤回 B1 话术 | **CLOSED** | `Addendum:54-55`；`docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.2-R2.md:9,31,42,66`；`docs/lead/PAY-SETTLE-01/tasks.md:21` | 不再要求 Reviewer 撤回 B1 |
| **M-2** `baseline_impact` 无 INV | **CLOSED** | `Addendum:17`；`docs/collab/CONVENTIONS.md:137` | 已列 INV-44/45/46/47/49/50 |
| **M-3** AC-27 归属 | **CLOSED** | `Addendum:12-13`；`docs/specs/0001-fulfillment-quantity-split.md:571-579`；`docs/collab/ac-coverage.tsv:41` | 已明确 AC-27 属 SPEC-0001 且为 AC-REF |
| **M-4** fresh apply 顺序 | **PARTIAL** | `Addendum:242-254`；`scripts/verify-r01-migrations.sh:28-35`（仍只到 `0020`）；`docs/lead/PAY-SETTLE-01/tasks.md:35`（只写 `0022`）；`migrations/order/0021_zero_application_check.sql` | Addendum 要求 `0021/0022`；Plan/脚本只有 `0022`（N-4） |
| **M-5** shadow range 矛盾 | **PARTIAL** | `Addendum:172-177`；`migrations/fulfillment/0010_settlement_shadow_switch.sql:42-45`；`internal/platform/config/config.go:774-776`；`internal/services/fulfillment/settlement_switch.go:94-96,147-148` | shadow 要 range 的裁决已写；`release_range <> ''` 仍只写“迁移 CHECK 或 ops 模板”，无迁移号/owner/期限（N-5） |
| **M-6** approval `EXPIRED` 无主 | **CLOSED（契约）/ ROUTED（ops）** | `Addendum:164-170`；`docs/adr/0007-payment-settlement-admission.md:832,836`；`migrations/fulfillment/0010_settlement_shadow_switch.sql:23-45` | 已声明 `EXPIRED` 不可达、看板按 `expires_at`；ops 告警在 R0.4d1，无期限（TODO） |
| **M-7** DLQ/缺 reservation 静默 | **PARTIAL（N-2 阻断）** | `Addendum:105-113,122,273`；`internal/platform/consumer/consumer.go:69-70`；`internal/platform/outbox/outbox.go:60-61,264-283`；`docs/ops.md:259-275`；`internal/services/inventory/refund_release_consumer.go:92-96`；`internal/services/order/refund_release.go:182-186` | DLQ/重放契约已写；metric 代码路径不在 `allowed_paths`，且 R0.4b 证据错位（N-2） |
| **M-8** legacy bool | **CLOSED** | `Addendum:139-141`；`internal/platform/config/config.go:736-760`；`docs/adr/0007-payment-settlement-admission.md:834` | 三态与 legacy 冲突 fail-fast 已覆盖 |
| **M-9** 0022 证据/INV-44 | **PARTIAL（DEFERRED）** | `Addendum:123,242-254`；`scripts/verify-r01-migrations.sh:28-35`；`docs/lead/PAY-SETTLE-01/evidence-matrix.md:65`（只有 INV-46，无 INV-44）；`docs/lead/PAY-SETTLE-01/tasks.md:35` | evidence-matrix 无 INV-44 行；0022 真实迁移日志未落；owner/期限已写（2026-12-31），但未真正关闭（N-8） |
| **M-10** 失败后重跑 | **CLOSED（契约）** | `Addendum:256-266`；`migrations/order/0022_r04_checks.sql:11`；`scripts/test-migrate-runner.sh:49-56` | 流程正确；0022 头注释仍写“re-run after cleanup safe”，R0.4d2 必须删（TODO） |
| **M-11** 退出码/审计字段 | **PARTIAL** | `Addendum:179-192`；`cmd/fulfillment/main.go:22-24,30,38,57,63-70,74,87,110` | exit 3/4 已定义；config/N5 仍是 panic，不满足“所有拒绝日志含 mode/approval_id/reason”（N-6）；shadow 日志仍只 `mode` |
| **M-12** 0022 枚举重复 | **CLOSED（契约）** | `Addendum:194-209`；`migrations/order/0022_r04_checks.sql:28-36,34`；`migrations/order/0017_refund_batch_ledger.sql:73-74` | 设计不复制完整枚举；当前 0022 仍复制，R0.4c 必须改（TODO）；`CHECK_CLAUSE` 文本校验已写进 R0.4d2 |

**计数**：CLOSED 8（B-1、M-1、M-2、M-3、M-6、M-8、M-10、M-12；其中 M-6/M-10/M-12 为契约关闭，M-6 的 ops 告警另 ROUTED）；PARTIAL 7（B-2、B-3、M-4、M-5、M-7、M-9、M-11）；OPEN 0。B-3 的 PARTIAL 内含 N-1 阻断。

---

## 2. 新发现

### `[BLOCKER]` N-1 — 0022/0023 决策门放在 R0.4c 就地编辑之后，不能保证“已 apply 文件不可变”

- **证据**
  - `Addendum:236-238`：裁决“在 0022 尚未被任何持久环境首次 apply 前，就地修订 0022”；决策门明确写 **“R0.4d2 开工前由 Developer 查询目标环境的 `schema_migrations`”**。
  - `Addendum:274`：R0.4c 交付 `order/0022` 的判别 CHECK + 等值 24h CHECK；`Addendum:276`：R0.4d2 才做“0022/0023 决策 + 真实 MySQL 运行”。
  - 因此若 `schema_migrations` 已有 `order/0022_r04_checks.sql`，R0.4c 的编辑已经先发生；R0.4d2 的查询只能事后发现，无法满足“已 apply 文件不可变”。
  - runner 正是用 `schema_migrations.checksum` 做账本（`scripts/verify-r01-migrations.sh:122`），FAILED attempt 还拒绝重放（`scripts/test-migrate-runner.sh:49-56`）；改已 apply 文件会直接使账本/重放失效。
- **具体修改**
  1. 把决策门移到 **R0.4c 开工前**（preflight）：Developer 先查目标环境 `schema_migrations WHERE filename='order/0022_r04_checks.sql'`，把结果写进迁移日志；有行 → R0.4c 不得编辑 0022，直接建 `0023`；无行 → 才允许就地修订。
  2. Addendum `:236-238` 的“R0.4d2 开工前”改为“R0.4c 开工前”；R0.4d2 只复核 preflight 结果，不再作为首次决策点。
  3. Lead Plan `R0.4d2` 与 `R0.4c` 的依赖顺序同步改写；`Addendum:274/:276` 的验收文字同步。
- **不修复的后果**：在任一已 apply 0022 的环境里，R0.4c 会先改文件、R0.4d2 才发现，导致 checksum 账本与迁移重放失效；这是迁移完整性阻断，不能降级为 TODO。

### `[BLOCKER]` N-2 — M-7 的 metric 代码路径不在 `allowed_paths`，且 R0.4b 任务/证据错位

- **证据**
  - `Addendum:15` 的 R0.4 实现 `allowed_paths` 仅含 `internal/services/fulfillment/`、`cmd/fulfillment/`、`internal/platform/config/`、`migrations/order/`、`migrations/fulfillment/`、`_test.go`，并明确 **不含 `internal/services/order/`**。
  - 但 `Addendum:122` 要求“多组跳过必须新增 metric + 告警”，且 `Addendum:273` 把 **“多组跳过 metric”** 挂到 **R0.4b**；该逻辑实际在 `internal/services/order/refund_release.go:182-186`（`log.Warn`，`internal/services/order/`），不在 `allowed_paths`。
  - `Addendum:111` 又要求“缺失 reservation 行”新增 metric + 告警；该逻辑在 `internal/services/inventory/refund_release_consumer.go:92-96`（`continue`，`internal/services/inventory/`），同样不在 `allowed_paths`，且未进 R0.4b/c 验收。
  - `Addendum:273` 的 R0.4b 证据是 `cmd/fulfillment/main_test.go` / `internal/services/fulfillment/settlement_switch_test.go`，无法验证 Order/Inventory 的 metric。
- **具体修改**
  1. 二选一：把 `internal/services/order/` 与 `internal/services/inventory/` 加入本裁决的 `allowed_paths`（并为 24h writer 的排除加限定），或把两个 metric 拆到独立变更（owner=Developer，expiry=2026-12-31，tracking=`review-tracking/PAY-SETTLE-01`）。
  2. 从 R0.4b 的 PASS 条件中移除“多组跳过 metric”，改挂到实际代码 owner/task；R0.4b 只保留 Fulfillment 启动门禁证据。
  3. `Addendum:111` 的 missing-reservation metric 必须给 task/owner/验收；否则删除“新增 metric”的 MUST，改为显式 DEFERRED。
- **不修复的后果**：R0.4b/c 的验收在自身 `allowed_paths` 下无法完成；Developer 只能在“越界改 Order/Inventory”与“验收失败”之间二选一。

### `[MAJOR]` N-3 — Addendum 的 R0.4b/c 任务 ID 与 Lead Plan 冲突（Developer vs Reviewer）

- **证据**：`Addendum:273-274` 把 **R0.4b/R0.4c** 定义为 Developer 的 Fulfillment 门禁/0022 迁移；`docs/lead/PAY-SETTLE-01/tasks.md:33` 把 **R0.4b/c** 定义为 Reviewer 复审 `90b502c`。`Addendum:279` 还声称“R0.4d1/d2 与 tasks.md:29-36 对齐”，但 `R0.4b/c` 的 owner/内容并不对齐。
- **修改**：改用不冲突的 ID（如 `R0.4b-impl` / `R0.4c-impl`），或统一 Lead Plan 的 R0.4b/c；在 Addendum §2.6 明确“R0.4b/c = 实现，R0.4e = Reviewer 复审”。
- **影响**：同 ID 不同 owner 会使实现空缺或把 Reviewer 误派去写代码。

### `[MAJOR]` N-4 — M-4 的 `0021` 未进 Lead Plan 与 canonical 验证脚本

- **证据**：`Addendum:252` 要求 `scripts/verify-r01-migrations.sh` 的 `MIGRATION_FILES` 新增 `order/0021_zero_application_check.sql` 与 `order/0022_r04_checks.sql`；实际 `scripts/verify-r01-migrations.sh:28-35` 仍只列 `0017-0020` + fulfillment `0010-0011`；`docs/lead/PAY-SETTLE-01/tasks.md:35` 的 R0.4d2 只写“增 `order/0022` entry”。
- **修改**：Lead 更新 R0.4d2 为 `0021 + 0022`；脚本对象断言加入 0021 的 `chk_refund_app_system_zero`；否则 fresh apply 的 `0017→…→0022` 并未被 canonical 脚本证明。
- **影响**：0021 的零元 CHECK 从未进入 R0.1/R0.4 的真实迁移证据，Addendum 的 M-4 只停留在文档。

### `[MAJOR]` N-5 — M-5 的 `release_range <> ''` 只有“迁移 CHECK 或 ops 模板”，无落点/期限

- **证据**：`Addendum:174-176` 写“批准行 `release_range` MUST `<> ''`：通过 fulfillment 迁移 CHECK 或 ops 写入模板保证（owner=Developer R0.4d1 / ops 模板）”。但 fulfillment 已合入的 `0010`/`0011` 在 `Addendum:249` 是只读；新增 CHECK 需要 `0012`，未出现在 fresh apply 顺序/脚本；`0010:42-45` 仍是 `VARCHAR(128) NOT NULL DEFAULT ''`，无 CHECK。ops 模板在 `Addendum:275` R0.4d1 有 owner，但没有 expiry/验收断言。
- **修改**：明确二选一：①R0.4d1 的 ops 写入模板 + `docs/ops.md` 校验（owner=Developer，expiry=2026-12-31，验收为模板渲染/样例行）；或 ②新增 `migrations/fulfillment/0012` CHECK，并加入 fresh apply 顺序、`scripts/verify-r01-migrations.sh` 与对象断言。不得保留“或”让实现自选。
- **影响**：`release_range=''` 的批准行虽被代码的精确匹配拒绝，但“MUST `<> ''`”没有可审计的落点，属于未处置的数据完整性 MUST。

### `[MAJOR]` N-6 — M-11 的 config/N5 仍是 panic，不满足“所有拒绝日志含 mode/approval_id/reason”

- **证据**：`Addendum:185-192` 定义 exit 1/2/3/4，并要求“**所有拒绝日志 MUST** 在注册 consumer 前输出，字段至少 `mode/approval_id/reason`”。但 exit 2 的 config/N5 路径在 `cmd/fulfillment/main.go:22-24` 是 `config.Load` 返回错误后 `panic(err)`，没有结构化 `mode/approval_id/reason`；`Addendum:188` 又写“已如此（config 层）”，`Addendum:192` 的 R0.4b 证据只覆盖 exit 3/4。
- **修改**：二选一：①把“所有拒绝日志”收窄为“门禁/shadow 拒绝日志”，并明确 config/N5 的 panic 只保证 exit 2 + 错误文本；或 ②R0.4b 捕获 `config.Load` 错误，在 logger 初始化后输出 `mode/approval_id/reason`，并加 `main_test` 断言。必须在 Addendum 中择一并写验收。
- **影响**：K8s CrashLoopBackOff 的 exit 2 与 exit 3/4 无法按本文件承诺的三字段审计；运维无法区分 config/N5 与门禁拒绝的上下文。

### `[MAJOR]` N-7 — B-2 的 T2 24h 补偿没有可用的 release source

- **证据**：`Addendum:97-98` 的 `source` 封闭枚举为 `tcc/system_zero/system_shortage/aftersale`；`internal/services/order/refund_release.go:35-51` 对 `payment`/`legacy_expired` 返回 `REFUND_RELEASE_SOURCE_UNSUPPORTED`。但 INV-46/T2 的 24h `COMPENSATING` 明确要求 INV-44 释放（`docs/specs/0002-payment-settlement-admission.md:130`；`docs/lead/PAY-SETTLE-01/evidence-matrix.md:51,65`；ADR §4.4.3），而 `legacy_expired` 是 `refund_applications` 的合法 source（`migrations/order/0017_refund_batch_ledger.sql:73-74`）。当前 release event 没有“legacy 24h system expiry”的 source。
- **修改**：在 Addendum §2.3.2/T2 延期项中显式登记：T2 24h 补偿的 release source/事件是 **T2 设计前置**（owner=Developer，expiry=2026-12-31）；二选一——新增版本化 source（需契约修订），或明确复用现有 source 并证明语义正确。不得让 T2 实现时才发现无合法 source。
- **影响**：M-6/B-3 把 writer 推 T2 后，T2 的 INV-46 补偿仍无法产生合法 release event；INV-44 的 24h 释放腿会被迫走未定义路径。

### `[MAJOR]` N-8 — M-9/INV-44 的 canonical evidence 行仍未落台账

- **证据**：`Addendum:123` 要求 `docs/lead/PAY-SETTLE-01/evidence-matrix.md` 新增 **INV-44 行**（owner=Lead）；实际该文件从 `INV-45` 开始（`evidence-matrix.md:64-69`），没有 INV-44；`docs/review-tracking.md:345` 只记录了 multi-group release gap（`refund_release.go:182-186`，P1/manual），不是 INV-44 evidence 行。
- **修改**：Lead 在 `evidence-matrix.md` 增 INV-44 行：release consumer 测试 + 0022 + 真实迁移日志；owner=Lead/Developer，expiry=2026-12-31；在 R0.4 收口前不得声称 INV-44 有证据。
- **影响**：B-2 的“多组/INV-44 缺口显式登记”只在本 Addendum 内，未进 canonical evidence；R0.4e/Reviewer 无法按台账核对。

### `[MAJOR]` N-9 — 窗口 5：Addendum 只写 writer，未写 24h expiry/compensation worker 的归属

- **证据**：`Addendum:230-232` 只写“writer（owner=Developer，T2.2/T2.4）……R0.4 不交付 writer”；没有点名 **24h 到期状态机**的 expiry/compensation worker（ADR §9 W10 `:964`；`evidence-matrix.md:65` INV-46 DEFERRED(T2)）。0022 的等值 CHECK 只拒绝晚设 deadline，不能把行移出 `OPEN/RESOLVING`；`r04_checks_integration_test.go` 也不驱动状态。
- **修改**：`Addendum:230-232` 增一句：“24h 到期后的 `COMPENSATING/EXPIRED_*` 由 Order expiry/compensation worker（W10，T2.2/T2.4）驱动；R0.4 只交付 CHECK，INV-46 仍 DEFERRED(T2)，owner=Developer，expiry=2026-12-31；不得声称 24h 状态机已闭环。”并引用 `evidence-matrix.md:65`。
- **影响**：窗口 5 的答案不完整；B-3 只关闭了“晚设 deadline”，未关闭“到期无人驱动”。

### `[TODO]` 清单（不阻断，但需记录）

- **T-1** Lead Plan `tasks.md:30` 仍写 `QC-R04-ADDENDUM.md`；实际报告名为 `QC-PAY-SETTLE-01-R0.4-ADDENDUM.md` / 本 `-R2`；Lead 对齐指针。
- **T-2** `docs/lead/PAY-SETTLE-01/tasks.md:31` 的 R0.4a2 写“把 Addendum 置 accepted”，但 Addendum 是 DRAFT 0.2、R0.4a3 才是人类接受；Lead 明确“R0.4a2 = 交 QC 的 DRAFT；R0.4a3 = human accept”。
- **T-3** `Addendum:188` 的 exit 2 “已如此”与 `:192` 的三字段要求矛盾（见 N-6）；若不在 R0.4b 修 config 日志，至少在表内注明“config panic 不在此列”。
- **T-4** `Addendum:102` 的“registry 已按此 fail-closed”对 `group_count==1` 不精确：registry 仅校验 `group_count > 0`（`registry.go:236-238`），`==1` 在 consumer（`refund_release_consumer.go:67-70`）；建议写“registry + consumer 联合 fail-closed”。
- **T-5** `Addendum:76` 的“v1 不设 top-level `warehouse_id`”是对 accepted ADR 字段表的**收窄**，但 §1.2 标题写“只增必填字段、不改既有字段语义”；建议改成“澄清 accepted ADR 的 `items[]/warehouse_id` 歧义；代码无 top-level 字段”。
- **T-6** `docs/ops.md`/`deploy/**` 仍无 `SETTLE_RELEASE_RANGE`/`FULFILLMENT_SETTLE_CONSUMER_MODE`（grep 零）；R0.4d1 接线路由，阻塞激活不阻塞本 Addendum。
- **T-7** 0022 大表 `ADD CHECK` 的 `ALGORITHM/LOCK` 与 MDL 时长仍未验证（`Addendum:254` 已标 `[不确定]`）；R0.4d2 需维护窗/锁时长记录。
- **T-8** 0022 当前仍 `>=`（`0022:55`）、仍复制完整 source 枚举（`0022:34`）、头注释仍写“re-run after cleanup safe”（`0022:11`）；R0.4c/d2 必须按 Addendum 修改，不得只靠本报告文字关闭。
- **T-9** `M-6` 的 ops 告警（`target_mode='settled' AND status='APPROVED' AND expires_at <= NOW(3)`）在 `Addendum:169` 是 MUST，但 R0.4d1 无 expiry/验收断言；补 owner/期限。
- **T-10** `M-5` 的 `release_range <> ''` 若选 ops 模板，需在 `docs/ops.md` 写样例行；若选 0012，需加 fresh apply/对象断言（见 N-5）。
- **T-11** `Addendum:81` 的“QC round 2 + Reviewer 复核”与 Plan 的 R0.4a3（人类接受，`tasks.md:32`）/R0.4b/c（Reviewer 复审，`:33`）顺序未写清：当前 human 接受在 Reviewer 复审之前。需明确“R0.4a3 是设计接受，Reviewer R0.4b/c 仍可 FAIL 并冻结”，或把 Reviewer 设计复核前移；否则批准路径的“Reviewer 复核”位置不可审计。

**更正第一轮的一处证据**：`R1` B-2 称 `registry.go:243-245` 的 validator “只校验 `release_id`”，该表述有误；实际 `validateRefundRelease` 在 `registry.go:225-302` 校验 `schema_version/release_scope/group_count(>0)/release_id/refund_id/order_no/group_no/covered_group_nos/source-reason/requested_at/items`，`:243-245` 只是 `release_id` 派生校验。Addendum `:83` 的更正成立；但 B-2 的“这些字段不在 accepted ADR 文本”缺口仍成立，故 B-2 仍 PARTIAL，不因该证据错误而关闭。

---

## 3. 必答崩溃窗口（5/5 已答）

| # | 窗口 | 第二轮答案（在 R1 基础上只核变化） | 恢复所有者 / 证据 | 结论 |
|---|---|---|---|---|
| 1 | **本地提交后崩溃**（准入事务已提交，release event 未发出） | release 请求与准入/零元执行事实**同一事务**写 outbox（`refund_admission.go:284-287`、`:510-514`；`refund_release.go:207-213`）；崩溃后 Order outbox relay 认领重投，20 次失败 dead-letter 到 `outbox_dead_letters`，ops 按 `refund_id/group_no` 重放。Addendum §1.4 已写 producer/DLQ/重放 owner。 | Order（producer）+ ops（重放）；`cmd/order/main.go:33`；`outbox.go:60-61,202-205,264-283`；`ops.md:259-275` | **CLOSED（契约）** |
| 2 | **跨服务 RPC 成功后崩溃** | **N/A（无 RPC）**：C-04 release 不新增 RPC（`Addendum:52-56`）；`git show --stat 90b502c` 无 `api/**`；Inventory consumer 的 `Release`/`TccCancelConfirm` 是本地接口（`refund_release_consumer.go:25-29`），不是跨服务 RPC。等价事件窗口（Inventory 已释放、未 ACK）由重投 `statusReleased` no-op 覆盖（`:97-99`；`service.go:168-183`）。 | Order/Inventory；`refund_release_consumer.go:56-128` | **N/A + 依据（CLOSED）** |
| 3 | **消息重复/乱序** | 同一订单只发一次 release（`refund_release.go:187-193`）；重复投递按 reservation 状态机幂等（`RELEASED` no-op，`refund_release_consumer.go:97-99`）；shadow 重复只更新 `last_seen_at`（ADR §6.3）。Addendum §1.4 已写重复/乱序。 | Inventory；`refund_release_consumer.go:97-105` | **CLOSED** |
| 4 | **取消/确认竞态** | permit CAS 与 release append 在同一事务：Ship 胜（`CONSUMED`）走 `PENDING_REMOTE` + commitment，不发 release；PRE 败者回滚重判；胜者唯一（permit 行 CAS），败者无库存副作用。`refund_admission.go:256-287`（permit CAS `:273-279`；release 仅 PRE `:284-287`；零元 `:508-514`）。Addendum 不改该协议，依赖 ADR §5.7 `:712-716`。 | Order（admission/permit）；Fulfillment 只 `CancelIfCreated` | **CLOSED（ADR/code）** |
| 5 | **外部重放缺失** | release：**N/A + 依据**——Order→Inventory 内部事件，无支付/承运商外部渠道；恢复由 outbox + consumer DLQ 自驱动。24h 历史队列：Payment 是外部渠道；若 Payment 不再重放，必须由 Order expiry/compensation worker（ADR §9 W10 `:964`）驱动；T2.2/T2.4 未实现，`evidence-matrix.md:65` 已登记 INV-46 DEFERRED(T2)。**Addendum §2.3.2 只写了 writer，未写 worker（N-9）；T2 release source 未定义（N-7）。** | Developer/T2；expiry=2026-12-31；`docs/lead/PAY-SETTLE-01/tasks.md:69-71`（T2.2/T2.4 未勾选） | **PARTIAL（N-7/N-9）** |

---

## 4. 状态机 / 无主 / 幂等 / 退出码 / DLQ / legacy / 0022 专项

| 状态/路径 | 进入后由谁驱动 | 第二轮结论 |
|---|---|---|
| `settlement_switch_approvals.status=EXPIRED` | 无应用写者（应用只读，ADR `:832`）；看板按 `expires_at` | **CLOSED（裁决）**：`Addendum:168` 明确不可达、不依赖 `status`；若业务要求可达，另立 ops 任务（owner/expiry 已写） |
| 已 `settled` 运行中、`expires_at` 到期 | 无运行期轮询（ADR `:836`）；Director 批准回滚 + 重启 | **CLOSED**：`Addendum:166-169` 已写停止路径/owner；ops 告警在 R0.4d1（T-9） |
| shadow 批准行已存在 | 无 consumer（T4.4 未交付），启动硬拒绝 | **CLOSED（契约）**：`Addendum:155-159`；ops MUST NOT 写入；`main.go:63-70` |
| `legacy_payment_resolution` `OPEN/RESOLVING` 24h 到期 | 设计上 Order expiry/compensation worker；T2.2/T2.4 未实现 | **PARTIAL**：N-9；Addendum 只写 writer，未写 worker |
| 多组订单退款 → INV-44 释放 | 只有 `log.Warn`，无 outbox、无重试、无人工队列 | **PARTIAL**：Addendum `:118-124` 已显式登记 P1/manual；metric 越界（N-2）、evidence 行缺（N-8） |
| release event 进入 DLQ | ops 手工重放（`docs/ops.md:259-275`） | **CLOSED（契约）**：`Addendum:108-110` 已写 subject/DLQ/20 次/owner |
| 0022 枚举重复 / 同名不同义 | R0.4c 改判别式；R0.4d2 校验 `CHECK_CLAUSE` | **PARTIAL（契约关闭，实现待）**：`Addendum:206-207`；当前 `0022:34` 仍复制 |
| 启动失败退出码 1/2/3/4 | 当前 3/4 仍 exit 1；config exit 2 是 panic | **PARTIAL**：N-6；`Addendum:185-192` 的“所有拒绝日志三字段”与 config panic 矛盾 |
| R0.4b/c 任务 ID | Addendum 与 Lead Plan 同 ID 不同 owner | **OPEN**：N-3；`Addendum:273-274` vs `tasks.md:33` |

- **幂等/崩溃窗口**：窗口 1-4 的幂等键/同一事务/`RELEASED` no-op/一次 release 仍成立（`refund_release.go:187-193`、`refund_release_consumer.go:97-99`、`service.go:168-183`）。
- **legacy bool**：矩阵行已补（`Addendum:139-141`）；与 `config.go:736-760` 的 `true→settled`/冲突 fail-fast 一致（M-8 CLOSED）。
- **DLQ**：release subject/DLQ/20 次/重放 owner 已写（`Addendum:108-110`）；missing reservation 的计数/告警仍未实现（N-2）。

---

## 5. 已验证成立的部分（避免只列漏洞）

- **B-1 运行态裁决**：`Addendum:149-162` 与 Director 2026-11-25 裁决（`review-tracking.md:350-358`）逐项一致：无论门禁是否通过都拒绝启动、稳定 reason `T4_4_SHADOW_NOT_DELIVERED`、日志三字段、T4.4 四项收口、默认 `paid`、不注册 `order.paid` 顶替；`main.go:63-70` 当前确在 consumer 注册前退出（exit 1，R0.4b 改 4）。
- **B-2 字段表/consumer**：`Addendum:85-103` 的 11 个顶层字段与 `registry.go:320-323` 一致；`group_count != 1` 在 `refund_release_consumer.go:67-70` 走 `consumer.Permanent`，不释放；`covered_group_nos` 长度/等于 `group_no`（`:71-85`）；`quantity` 审计输入、CONFIRMED 用预留数量（`:100-118`）；producer 同一事务（`refund_release.go:207-213`）且 `group_count=1`（`:194-205`）。
- **B-3 方向**：`Addendum:217-225` 的等值 CHECK 能拒绝 48h（设计上）；无容忍带；writer 明确推 T2.2/T2.4、`internal/services/order/` 不在 R0.4 `allowed_paths`（`Addendum:228-232`），没有在 R0.4 造半成品 writer；`0019:51` 的 `>=` 约束保留且与等值不冲突。
- **M-1/M-2/M-3**：`Addendum:54-55` 不再要求 Reviewer 撤回 B1；`Addendum:17` 列 INV；`Addendum:12-13` 正确把 AC-27 归 SPEC-0001 且标 AC-REF。
- **M-8/M-10/M-12**：legacy bool 矩阵（`Addendum:139-141`）与 `config.go:736-760` 一致；重跑流程（`Addendum:256-266`）与 `scripts/test-migrate-runner.sh:49-56` 的 FAILED attempt 删除一致；0022 判别式设计（`Addendum:200-203`）不复制完整 source 枚举，由 0017 封闭枚举兜底。
- **提交范围**：`git show --stat e3998d3` 只改 Addendum（1 file，+294/−73）；未改代码/迁移/台账/SPEC；`git status` 仅显示一个与本任务无关的未跟踪 R0.3 报告，本报告未触碰。
- **WIP 事实**：`90b502c` 是 HEAD 祖先；其 stat 仅 `cmd/fulfillment`、`internal/platform/config`、`internal/services/fulfillment`、`internal/services/order/r04_checks_integration_test.go`、`migrations/order/0022`，无 `api/**`，与 no-RPC 裁决一致。

---

## 6. 边界、`[不确定]` 与角色声明

- 本报告为 **QC 红队报告**，不是 Reviewer verdict；不批准 merge/Release，不代替人类接受。
- **未执行**：`go test`、`-race`、迁移、Helm 渲染；因此 WIP 测试/迁移结果**未被本报告复核**，标 `[不确定]`。本报告只读核对代码/迁移/文档。
- `[不确定]`：0022 是否已在某个持久环境 apply（N-1 的 preflight 才能定案）；MySQL 8.4 等值 CHECK 与 `NOW(3)`/`DEFAULT CURRENT_TIMESTAMP(3)` 的逐位行为；`ADD CHECK` 的 MDL/锁时长；T4.4 交付时间；多组 P1 是否/何时补。
- **只改了本文件**：未改 Addendum/ADR/SPEC/迁移/代码/台账；未 commit；未执行 `go test`/迁移。报告落盘路径：`docs/collab/qc-reports/QC-PAY-SETTLE-01-R0.4-ADDENDUM-R2.md`。
- 下游：本报告 **BLOCKED**；N-1/N-2 修复后需 **QC round 3** 或人类按风险接受（不建议）；之后 R0.4a3 人类接受 → R0.4b/c/d 实现/证据 → R0.4e Reviewer 日期化 verdict（`release_status=NOT RELEASE`）。任何一步 FAIL 冻结 `PAY-SETTLE-01` 的 R0.4 范围，不得用新 `change_id` 绕过。
