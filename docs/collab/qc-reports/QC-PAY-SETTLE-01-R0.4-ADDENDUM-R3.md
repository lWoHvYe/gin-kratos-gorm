# QC-PAY-SETTLE-01-R0.4-ADDENDUM-R3 — R0.4 C-04 Release Addendum DRAFT 0.3（r3）第三轮红队复核

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| 任务 | R0.4a2 第三轮复核（QC round 3；第一轮 `362b9e2`、第二轮 `e3998d3`） |
| 审查对象 | `docs/adr/0007-amd1-addendum-r0.4-c04-release.md`（Status **DRAFT 0.3**；Revision 0.3-draft r3；**未 commit，工作树文件**；sha256 `2908df67ce086e73b618ffca4e3f4a8f5b00cb742f0f54eb16ddd9e8eb12055c`） |
| **结论** | **READY — 可交人类接受（R0.4a3）**：**0 BLOCKER、0 未处置 MAJOR**；9 个 `[MINOR]`（均带 owner/落点/建议）+ 6 项 ROUTED。READY **只表示设计可交人类接受**，不表示实现 PASS / merge / Release PASS / AC-VERIFIED |
| 日期 | **2026-11-26** |
| Actor | `qc-r04-r3`（Quality Checker；与 Architect / Developer / Reviewer 不同 actor） |
| base | `06e043e50cdbe05b53991b43e2bc49e1f72d788b` |
| head | `06e043e50cdbe05b53991b43e2bc49e1f72d788b`（`git rev-parse HEAD`）；被审 r3 为工作树文件，故以 sha256 绑定内容（见上） |
| code cross-check | `90b502c36a38df9538bbf647ed551f63fb049013`（HEAD 祖先；只读核对契约-实现一致性） |
| Tier | **L2**（跨服务事件 / 库存 / 迁移 / 启动门禁） |
| review_scope | 只审 Addendum r3 对 round 2 的 `N-1`/`N-2` + `N-3..N-9` 处置、r3 新引入的矛盾/死胡同/越界，5 个崩溃窗口，以及新扫项（状态机/无主状态/幂等/退出码/DLQ/0021-0022 顺序/INV-44 证据归属/与 SPEC-0001 v1.5 AC-27A 一致性）。只读核对 `90b502c` 与 HEAD 的代码/迁移/脚本/台账。**不审** R0.3 台账正文、C-04 RefundBatch 正文、T2/T3/T4.4 实现、0022/0023 的 SQL 正确性执行验证。 |
| release_status | **NOT RELEASE / NOT RELEASABLE**；不批准 merge；不声称任何测试/迁移已执行；不声称 AC-VERIFIED |

## 0. 结论与必读声明

**READY（可交人类接受）。** 第三轮结论依据：

1. **N-1 已关闭**：`0022`/`0023` 可编辑性 preflight 已前移到 `R0.4c-impl` **任何编辑之前**，并给出目标环境口径、两条只读 SQL、checksum 比对、分支 A/B、fail-closed（不可达=已 apply）、stop-the-line 与 `R0.4d2` 只复核的角色边界（`Addendum:279-316`；`Addendum:361`）。
2. **N-2 已关闭**：多组跳过 metric 与 missing-reservation metric 均移出 R0.4 并登记 owner/expiry/停止条件；`allowed_paths` 未扩大，`forbidden_paths` 增 `internal/services/inventory/**`；`R0.4b-impl` 的 PASS 只覆盖 Fulfillment 启动门禁，证据不再错位（`Addendum:15-16,125-127,138,375-376,411`）。
3. **`N-3..N-9` 全部有可执行落点**：CLOSED 7（N-3/N-4/N-5/N-6/N-9 + N-7 契约 + N-8 Addendum 侧），PARTIAL 2（N-7 边界声明、N-8 canonical 行内容），残余均已降级为 `[MINOR]`/ROUTED 并带 owner/expiry/停止条件（§3、§4）。
4. **无新增 BLOCKER**：r3 的 5 个崩溃窗口答案仍成立；无新增 RPC、无新增 subject、未扩大 `allowed_paths`；未把 T2/T3/T4.4 塞回 R0.4。

**READY 的含义边界（必读）**：

- READY 是**设计交付**判断：可以进入 **R0.4a3 人类设计接受**。它**不**代表 R0.4 收口：`0022` 当前仍是 `>=`、仍复制 source 枚举、仍写“re-run safe”（`migrations/order/0022_r04_checks.sql:11,34,55`），`R0.4b-impl/c-impl/d1/d2` 与 `R0.4e` 全部未做；任何“迁移已验证 / INV-44 有证据 / AC-27 已验收”的声明都不成立。
- 本报告不批准 merge/Release，不代签 `Reviewed-by`，不代替人类接受，也不构成 v1.5 SPEC-0001 的 QC delta（见 §6.3）。

**只读声明**：本报告未执行 `go test`、`-race`、迁移或 Helm 渲染；文中代码/迁移/脚本引用是**只读事实**，不构成执行证据。无法核对的内容标 `[不确定]`。

---

## 1. N-1 专项（重点）：preflight 是否真在编辑之前、A/B 是否可执行、是否仍违反“已 apply 不可变”

### 1.1 逐问核对

| 问 | 结论 | 证据 |
|---|---|---|
| preflight 是否真在 `R0.4c-impl` **任何**编辑之前 | **是** | `Addendum:281`（“`R0.4c-impl` 的第一条交付动作不是改 0022，而是 preflight；preflight 证据不存在时 MUST NOT 打开/修改 0022”）；`Addendum:361`（R0.4c-impl 验收 = “**preflight 证据先行**”）；`Addendum:410`（N-1 处置行）；`Addendum:311`（R0.4d2 降为“只复核”） |
| 分支 A 是否可执行 | **是**（带 3 处措辞修正，见 S-1/S-3/S-4） | `Addendum:304`（新增 additive `0023`，替换两个约束，优先新名 + ADD 后 DROP 旧名；同步 fresh-apply、`scripts/verify-r01-migrations.sh` 的 `MIGRATION_FILES`/对象断言；0022 逐字节不变）；`Addendum:323-331` |
| 分支 B 是否可执行 | **是**（见 S-4 的 RUNNING 残记录缺口） | `Addendum:305`（所有目标环境 0 行且无 `APPLIED` attempt 残记录 → 允许就地修订）；`Addendum:347`（清 attempt 再重跑） |
| 是否仍违反“已 apply 不可变” | **否**（默认 fail-closed） | `Addendum:306-310`：①不可达/无权限/无法确认 → 视为已 apply 走 A；②attempt=APPLIED 但账本无行 → 走 A 并登记数据完整性事件；③结论不得就地改判；④stop-the-line + 恢复 preflight 前字节 + 改走 0023 + 冻结。账本校验口径见 `scripts/verify-r01-migrations.sh:117-123`（checksum 逐文件比对） |
| 目标环境清单是否完整 | **功能定义完整、环境名称与仓库实际不符**（S-2） | `Addendum:284`（“至少共享 dev/staging/prod + 任何 durable volume/共享 DB 的预发；一次性容器不算”）；仓库实际：`deploy/compose/**`（dev）、`deploy/k8s/uat/values-mall.yaml`（`mall-uat-data` namespace）、`docs/ops.md:524-525` 的“opt-in production profile”；无 `staging/`、`prod/` 目录 |
| 证据是否完整 | **是** | `Addendum:301`（环境名/查询输出/执行人/时间/HEAD + `sha256sum`）；`Addendum:313`（环境清单可核、`git diff --stat` 不含 0022、时间/HEAD 可核） |
| 停止条件是否完整 | **是** | `Addendum:310`（冻结 + 恢复字节 + 改 0023 + 登记 change freeze）；`Addendum:483`（假设 #1 的停止条件） |

### 1.2 可执行性事实核对（只读）

- `schema_migrations(filename, checksum, applied_at)` 存在：`deploy/k8s/images/migrate/run.sh:105-110`、`deploy/compose/init/mysql/01-schemas.sh:41-46`；`applied_at` 列名与 `Addendum:289-291` 一致。
- `schema_migration_attempts` 由 **migrate runner** 建表并维护 `RUNNING/APPLIED/FAILED`：`deploy/k8s/images/migrate/run.sh:114-122`；主键为 `filename`（每个文件至多一行，故 `Addendum:294-297` 的 `GROUP BY status` 恒为 0/1 行，可执行）。
- runner 对“有账本”与“有 attempt 无账本”的处理：`run.sh:131-152` —— 有账本 + checksum 不符即退出 1；attempt 存在但无账本一律拒重放（含 `RUNNING`）。
- `0022`/`0021` 的现状：`migrations/order/0022_r04_checks.sql:34`（复制完整 source 枚举）、`:55`（`>=`）、`:11`（“re-run after cleanup safe”）、`:28-32`（只按名判存在）；`0021` 已由 `6be5c5a` 合入（`chk_refund_app_system_zero`，`0021_zero_application_check.sql:29-36`）。
- 结论：分支 A/B 都可执行；**默认路径事实上是 A**（`deploy/compose/init/mysql/01-schemas.sh` 对全新持久卷会 apply 全部迁移并把 checksum 写入账本，节 `:41-71`），这不影响 N-1 关闭，但放大了 S-1 的文本矛盾。

### 1.3 N-1 残余（均为 MINOR，不阻断接受）

- **S-1**：分支 A 下仍写着“必须改掉 0022:11/:34/:55”（见 §3）。
- **S-3**：preflight 第 2 条 SQL 假设 `schema_migration_attempts` 存在（见 §3）。
- **S-4**：`RUNNING` 残记录未纳入 A/B 判据，`§2.5` 只删 `FAILED`（见 §3）。

---

## 2. N-2 专项（重点）：metric 是否真的移出 R0.4、allowed/forbidden 是否一致、R0.4b/c 证据是否不再错位

| 检查点 | 结论 | 证据 |
|---|---|---|
| 多组跳过 metric 是否移出 R0.4 | **是** | `Addendum:138`（“原 r2 的‘R0.4b/c PASS 条件包含该 metric 测试’**撤回**……R0.4b-impl 的 PASS 只覆盖 Fulfillment 启动门禁”）；`:375`（§2.7 DEFERRED + 停止条件）；`:473`（反模式自省第 5 条） |
| missing-reservation metric 是否移出 | **是** | `Addendum:124-127`（不再是 R0.4 交付项；“不得声称该路径可观测性问题已解决”）；`:376` |
| `allowed_paths` 是否被扩大 | **否** | `Addendum:15` 明确不含 `internal/services/order/`、`internal/services/inventory/`；`:16` 把二者列入 `forbidden_paths` |
| R0.4b 证据是否仍错位 | **否** | `Addendum:360`（只 `main_test.go` + `settlement_switch_test.go`）；`:138` 显式排除 Order/Inventory metric |
| R0.4c 证据是否仍错位 | **否** | `Addendum:361`（preflight + 0022/0023 + MySQL 日志；无 metric） |
| 是否存在“越界才能 PASS”的残余 MUST | **否**（全表扫描 r3 的 MUST） | 24h writer/worker（`:268-269`）、T2 source（`:270-276`）、ops 模板（`:196-200`）、INV-44 行（`:333-337`）均已指向 R0.4 外 owner |

**N-2 残余（MINOR）**：**S-5** —— Addendum 对证据台账“现状”的描述已过期，且给出的行内容要求未被（工作树中的）行满足（见 §3）。

---

## 3. 新问题分级（0 BLOCKER / 0 MAJOR / 9 MINOR）

> 分级规则：BLOCKER = 阻断接受或可直接导致不变量违反；MAJOR = 必须修复后才能接受；MINOR = 可交人类接受并随实现/R0.4e 关闭。以下每条均给出 `文件:行号`。

| ID | 级别 | 问题 | 证据（文件:行号） | 建议 / Owner |
|---|---|---|---|---|
| **S-1** | MINOR | **分支 A 与“必须改掉 0022”文本矛盾**：§2.3.3 要求分支 A 下“0022 逐字节不变”，但 §2.2/§2.5 仍要求 `R0.4c-impl` “修掉 `0022:11/:34/:55`”、并要求“迁移头注释”口径一致 | `Addendum:243`（“三条 MUST 在 `R0.4c-impl` 内修掉”）、`:351`（“Addendum/迁移头注释/ops runbook 三处口径必须一致”）vs `:232`（“同样内容落在 0023，0022 逐字节不变”）、`:304`、`:314`（验收要求 `git diff --stat` 不含 `0022_*`） | Architect 加一句：“分支 A 下三条修正落在 0023，`0022` 头注释作为不可变历史保留，不作口径源；分支 B 才改 0022”。在改前，实施者可能编辑已 apply 的 0022，正是 N-1 要防的 checksum 破坏；`Addendum:314` 的验收可检出，故不升级 |
| **S-2** | MINOR | **preflight 目标环境名称与仓库实际不符**：写“至少共享 dev/staging/prod”，仓库只有 compose dev 与 k8s UAT（`mall-uat-data`），prod 仅“opt-in profile” | `Addendum:284` vs `deploy/compose/init/mysql/01-schemas.sh:1-30`、`deploy/k8s/uat/values-mall.yaml:1-16`、`docs/ops.md:524-525` | Architect/Developer：以“仓库实际环境为准（compose 持久卷、`mall-uat-data`；如启用 prod profile 亦含）”替换示例名；`Addendum:313` 的“环境清单可核”由 preflight 证据自己枚举。fail-closed 规则（`:306-308`）已保证误判安全 |
| **S-3** | MINOR | **preflight 第 2 条 SQL 假设 `schema_migration_attempts` 存在**：compose init 只建 `schema_migrations`，不建 attempt 表；查询将报错而非返回 0 行 | `Addendum:293-297` vs `deploy/compose/init/mysql/01-schemas.sh:41-46`（仅 `schema_migrations`）、`deploy/k8s/images/migrate/run.sh:114-122`（attempt 表只由 runner 建） | Developer：查询前先 `information_schema.tables` 判定；表不存在记为“无 attempt 记录”。按 `Addendum:306-308` 本会 fail-closed 到 A（安全），但会把“表不存在”误报为“环境不可达” |
| **S-4** | MINOR | **attempt 残记录判据只排除 `APPLIED`，未覆盖 `RUNNING`；§2.5 修复流程只删 `FAILED`**：MySQL DDL 隐式提交，进程在 0022 中途崩溃会留下 `RUNNING` + 部分约束落地；runner 对任何无账本的 attempt 一律拒重放，而 §2.5 的 DELETE 条件不含 `RUNNING` | `Addendum:305`、`:347`、`:343` vs `deploy/k8s/images/migrate/run.sh:112-113,145-152`（“previous ${attempt_status} attempt”含 RUNNING）、`docs/runbooks/c-migration-rollback.md:96-105`（FAILED/RUNNING 的既有人工恢复流程） | Developer：分支 A/B 判据改为“**任何非空 attempt 残记录（含 RUNNING）→ 视为可能已部分 apply，走分支 A**”，并在 §2.5 引用 `c-migration-rollback.md:96-105`、把 DELETE 条件改为 `status IN ('FAILED','RUNNING')`。风险已被 §3 的 R0.4d2 `CHECK_CLAUSE` 文本断言（`Addendum:244,330`）兜住，故不升级 |
| **S-5** | MINOR | **INV-44 canonical 行的“现状”描述过期且行内容不满足 Addendum 自身 MUST**：Addendum 两处仍写“现只有 INV-45…50 / 现无 INV-44 行”，但工作树已新增 INV-44 行；该行缺 Addendum 要求的“missing-reservation no-op + metric 缺口显式延期”，且行内 Owner=Developer 与 Addendum 的“台账 owner=Lead”口径不同 | `Addendum:139`、`:381` vs `docs/lead/PAY-SETTLE-01/evidence-matrix.md:64`（工作树新增行）；`Addendum:335`（MUST 含“缺 reservation 行 no-op **且 metric 缺口显式登记为延期**”） | Lead：在行内补“missing-reservation metric DEFERRED（owner/expiry/tracking）”并把 Owner 列语义写清；Architect 把 `:139,:381` 的“现无”改为“本文件不写台账，行由 Lead 落”。该行未落地前 R0.4e 必须列 `NOT RELEASE` 缺口（`Addendum:336`） |
| **S-6** | MINOR | **`status='REVOKED'` 无主/无语义**：批准行状态机除 `EXPIRED` 外还有 `REVOKED`，Addendum §2.1.2 只裁决 `EXPIRED` 不可达，未写 `REVOKED` 的写者、看板口径与门禁语义 | `Addendum:180-186` vs `migrations/fulfillment/0010_settlement_shadow_switch.sql:52`（`chk_switch_status CHECK (status IN ('APPROVED','REVOKED','EXPIRED'))`）、`Addendum:153-154`（门禁要求 `status='APPROVED'`） | Architect：一句话——“`REVOKED` 只由 Director/ops 人工写入表示撤销；门禁对非 `APPROVED` 一律拒绝启动；本裁决不新增写者”。无资金/库存副作用，不阻断 |
| **S-7** | MINOR | **R0.4d1 的 `deploy/**` 归属自相矛盾**：`forbidden_paths` 把 `deploy/**` 排除并称“R0.4d1 单独 L2 变更”，但 §2.6 的 R0.4d1 验收包含 compose/Helm 渲染 | `Addendum:16` vs `Addendum:362`（R0.4d1 = compose + Helm + `docs/ops.md`） | Architect：在 §2.6 注明“R0.4d1 的 `deploy/**` 部分必须作为独立 change input（自有 allowed_paths）交付，本 Addendum 的 allowed_paths 不覆盖”；Lead 在 Plan 中保持此边界 |
| **S-8** | MINOR | **T2 释放腿未声明受 §1.5 单组边界约束**：v1 事件固定 `group_count=1`、每单至多一条 release（`hasOrderReleaseEventTx` 按 `aggregate_id=order_no` 去重），而 T2 24h `COMPENSATING` 的 INV-44 释放是**订单级**（全部未离仓组）；`§2.3.2` 只说“若该订单需要 INV-44 释放就按 source 前置产生事件”，未写多组订单的处置 | `Addendum:269`、`:270-276` vs `internal/services/order/refund_release.go:26,150-162,194-205`（`group_count=1` + 每单一次）、`internal/services/inventory/refund_release_consumer.go:67-70`（`!=1` 永久失败）、ADR `docs/adr/0007-payment-settlement-admission.md:964`（W10） | Architect：补一句“T2 24h 释放腿同样受 §1.5 的 P0 单组边界；多组订单的 24h 补偿释放续延 P1/manual，owner/expiry 同 §2.7 第 1 行”。T2 已冻结（`:275`），不构成 R0.4 风险 |
| **S-9** | MINOR | **引用漂移**：①r3 称 48h 断言注释在 `r04_checks_integration_test.go:215`，实际注释在 `:213`、48h 插入在 `:218-220`；②`AC-27（0001:571-579）` 是 v1.4 行号，v1.5 工作树已把 AC-27 移到约 `:585-604`，v1.5 生效后所有 `0001:` 行号失效；③`RELEASE_RANGE_MISSING` 会同时出现在 exit 2（`config.go:775`）与 exit 3（`settlement_switch.go:95,106`），同一 token 跨两个退出码 | `Addendum:261` vs `internal/services/order/r04_checks_integration_test.go:213,218-220`；`Addendum:12` vs `docs/specs/0001-fulfillment-quantity-split.md:585-604`；`Addendum:210-220` vs `internal/platform/config/config.go:775`、`internal/services/fulfillment/settlement_switch.go:95,106` | Architect：把行号改为“以 v1.5 提交后为准”或只引 AC 编号；reason token 表加一列“可能退出码”。纯引用精度，不影响语义 |

**分级结论**：**0 BLOCKER、0 MAJOR、9 MINOR**。S-1/S-4 触及迁移完整性，但均已有（或已要求）可核验的兜底断言（`Addendum:244,314,330`）与既有人工恢复流程（`docs/runbooks/c-migration-rollback.md:96-105`），故按“可交人类接受 + 实现期关闭”处理，不升级为阻断项。

---

## 4. N-3..N-9 关闭表（含 round 1 遗留）

| QC | 第三轮状态 | 处置落点 / 证据（文件:行号） | 残余 |
|---|---|---|---|
| **N-1** `[BLOCKER]` 0022/0023 决策门顺序 | **CLOSED** | `Addendum:279-316`（preflight 在编辑前）、`:361`、`:410`；`deploy/k8s/images/migrate/run.sh:131-152`；`scripts/verify-r01-migrations.sh:117-123` | S-1 / S-3 / S-4（MINOR） |
| **N-2** `[BLOCKER]` metric 越界 + 证据错位 | **CLOSED** | `Addendum:15-16,124-127,138,375-376,411`；`refund_release.go:182-186`；`refund_release_consumer.go:92-96` | S-5（台账行内容） |
| **N-3** `[MAJOR]` 任务 ID 冲突 | **CLOSED** | `Addendum:353-366`（`R0.4b-impl`/`R0.4c-impl`；“`R0.4b/c`=Reviewer 复审 `90b502c`”）；`docs/lead/PAY-SETTLE-01/tasks.md:33` | Lead Plan 尚未新增 `-impl` 行（ROUTED，`:366`，expiry 2026-12-31） |
| **N-4** `[MAJOR]` `0021` 未进脚本/Plan | **CLOSED（契约）** | `Addendum:324,328-332`（`MIGRATION_FILES` 增 `0021`+`0022`；对象断言含 `chk_refund_app_system_zero`）；`scripts/verify-r01-migrations.sh:28-35`（现仅 0017-0020 + fulfillment 0010-0011） | `tasks.md:35` 仍只写 `0022`（ROUTED，`:332`） |
| **N-5** `[MAJOR]` `release_range <> ''` 无唯一落点 | **CLOSED** | `Addendum:188-201`：选定 ops 模板 + `docs/ops.md` 校验 SQL（结果必须 0 行），给理由、可执行验收与升级停止条件；事实核对 `0010_settlement_shadow_switch.sql:42`（`DEFAULT ''`）、`config.go:775`、`settlement_switch.go:95,147-148` | 0012 仅作人类要求时的另立路径（`:200`） |
| **N-6** `[MAJOR]` config/N5 仍 panic、无三字段 | **CLOSED（契约）** | `Addendum:203-228`：exit 3/4 三字段、exit 2 保留 panic + 稳定 token、main_test 断言 `==2` 且含 `N5_APPROVAL_ID_MODE_MISMATCH`；事实核对 `config.go:752,760,766,775`、`main.go:22-24,48-57,63-70` | S-9③（token 跨退出码，MINOR） |
| **N-7** `[MAJOR]` T2 无合法 release source | **CLOSED（契约）/ PARTIAL（边界）** | `Addendum:270-276`：新增 `source=legacy_24h`/`reason=LEGACY_24H_EXPIRY`，禁止复用现有/`payment`/`legacy_expired`，consumer-first 上线顺序，无 Amendment 则冻结 T2 | S-8（多组/一单一事件边界未声明，MINOR） |
| **N-8** `[MAJOR]` INV-44 evidence 行未落 | **CLOSED（Addendum 侧）/ PARTIAL（canonical 行）** | `Addendum:333-337`（本文件不写台账；给出必备字段、owner=Lead、expiry、R0.4e 强制核对）；`evidence-matrix.md:64` 工作树已出现 INV-44 行（未 commit） | S-5：行内缺 “missing-reservation metric 延期”；行未 commit → R0.4e 必须列 `NOT RELEASE` 缺口 |
| **N-9** `[MAJOR]` 窗口 5 只写 writer | **CLOSED** | `Addendum:269`（Order expiry/compensation worker，ADR §9 W10 `:964`，T2.2/T2.4；“只拒绝晚设 deadline，不能把行移出 `OPEN/RESOLVING`”；INV-46 保持 DEFERRED） | INV-46 实现 DEFERRED（T2，2026-12-31） |
| B-1 shadow 运行态 | CLOSED（契约） | `Addendum:165-178`；`review-tracking.md:358-366`；`main.go:63-70` | exit 4/日志字段属 `R0.4b-impl` |
| B-2 契约来源/`group_count` | CLOSED（契约） | `Addendum:62-140`；`registry.go:198-215,225-302,320-323`；`refund_release_consumer.go:66-85` | 多组 P1/manual（DEFERRED） |
| B-3 24h SLA 等值 | CLOSED（契约） | `Addendum:250-277`；`0019_legacy_payment_resolution.sql:33-54`（`entered_at/expires_at DATETIME(3)`，等值可复现） | 实现待 `R0.4c-impl`（`0022:55` 仍 `>=`） |
| M-1/M-2/M-3/M-8/M-10/M-12 | CLOSED | `Addendum:54-60,17,12-13,150-158,341-351,230-246` | M-10/M-12 的实现待 `R0.4c-impl`（`0022:11,34`） |

**计数**：CLOSED 9（N-1、N-2、N-3、N-4、N-5、N-6、N-9 + N-7/N-8 的契约侧），PARTIAL 2（N-7 边界、N-8 行内容，均 MINOR），OPEN 0。

---

## 5. 必答崩溃窗口（5/5 已答；r3 只核变化）

| # | 窗口 | r3 答案 | 恢复所有者 / 证据 | 结论 |
|---|---|---|---|---|
| 1 | **本地提交后崩溃**（准入已提交、release 事件未发出） | 未变：release 请求与准入/零元执行事实**同一事务**写 outbox；崩溃后 Order outbox relay 认领重投，20 次失败 dead-letter 到 `outbox_dead_letters`（7 天 prune），ops 按 `refund_id/group_no` 重放。r3 新增 §1.4 契约（subject/DLQ/owner） | Order（producer）+ ops（重放）；`refund_admission.go:283-287`、`:508-514`；`refund_release.go:207-213`；`outbox.go:58-61,264-283`；`docs/ops.md:259-266`；`Addendum:118-129` | **CLOSED** |
| 2 | **跨服务 RPC 成功后崩溃** | **N/A + 依据**：C-04 release 不新增 RPC（`Addendum:54-60`）；`90b502c` stat 无 `api/**`（8 files，仅 config/fulfillment/order-test/migrations）；Inventory 的 `Release`/`TccCancelConfirm` 是本地接口。等价事件窗口（Inventory 已释放、未 ACK）由重投 `statusReleased` no-op 覆盖 | Order/Inventory；`refund_release_consumer.go:96-99`；`inventory/service.go:171-186` | **N/A（CLOSED）** |
| 3 | **消息重复/乱序** | 未变：同一订单只发一次（`hasOrderReleaseEventTx` 按 `aggregate_id`）；重复投递 `RELEASED` no-op；乱序不可能早于准入（同事务）。r3 未改协议 | Inventory；`refund_release.go:150-162`；`refund_release_consumer.go:96-105`；`Addendum:128` | **CLOSED** |
| 4 | **取消/确认竞态** | 未变：permit CAS 与 release append 同一事务；Ship 胜（`CONSUMED`）走 `PENDING_REMOTE`+commitment 不发 release；PRE 败者回滚重判；胜者唯一 | Order；`refund_admission.go:256-290`（`:284-287` 仅 PRE）、`:508-514`；ADR §5.7 `:712-716` | **CLOSED** |
| 5 | **外部重放缺失** | release：**N/A + 依据**（内部事件，outbox+DLQ 自驱动）。24h 历史队列：Payment 是外部渠道；r3 明确归属 Order expiry/compensation worker（W10，T2.2/T2.4），INV-46 DEFERRED；T2 release source 已裁定但多组边界未声明（S-8） | Developer/T2；`Addendum:269,270-276`；ADR `:964`；`evidence-matrix.md:66`（INV-46 DEFERRED） | **CLOSED（设计）/ DEFERRED（实现）** |

---

## 6. 新扫结果

### 6.1 状态机死胡同 / 无主状态 / 幂等 / 退出码 / DLQ / 迁移顺序

| 项 | 结论 | 证据 |
|---|---|---|
| 状态机死胡同 | 无新增。`settlement_switch_approvals.status='EXPIRED'` 已裁决为 R0.4 不可达、看板按 `expires_at`；`legacy_payment_resolution` 的 24h 推进由 T2 worker（DEFERRED）驱动，Addendum 明写“不得声称闭环” | `Addendum:180-186`、`:269`；`0010:52` |
| 无主状态 | `EXPIRED` 已归属（不新增写者）；**`REVOKED` 仍无主/无语义 → S-6（MINOR）** | `0010:52`；`Addendum:180-186` |
| 幂等/崩溃窗口 | 未变且成立：`release_id=rr:<refund_id>` + 每单一次 + `RELEASED` no-op + 同事务 outbox | `refund_release.go:150-213`；`refund_release_consumer.go:96-105` |
| 退出码 | 契约完整（1/2/3/4 + 稳定 reason token）；实现未做（现全 `os.Exit(1)`） | `Addendum:203-228`；`main.go:30,38,48-57,63-70,74,87,110` |
| DLQ | subject/DLQ/20 次/归档/prune/重放 owner 已写；`MaxDeliver=20` 与 outbox `maxAttempts=20` 与 `docs/ops.md:259-275` 一致 | `Addendum:118-129`；`consumer.go:68-70`；`outbox.go:58-61` |
| `0021`/`0022` 顺序 | 无冲突：`0021` 只约束 `system_zero` 的金额组成（`0021:29-36`），`0022` 只约束 `group_no` 与 source 归属；两表列不交叉，`0017:45-48` 的 `source NOT NULL` + 封闭枚举保证 `0022` 判别式不会被 NULL source 绕过 | `0021_zero_application_check.sql:29-36`；`0017_refund_batch_ledger.sql:45-48,73-80` |
| `0022` 判别式实现细节 | r3 的判别式（`Addendum:234-240`）在 `source NOT NULL` 前提下与 0017 枚举等价；`group_no` 为 NULL 的 `system_zero` 会被拒绝，与现有生产写者（`refund_admission.go:498-502` 传入 group）一致，不会造成 AC-67/68 回归 | `Addendum:236-239`；`0017:48`；`refund_admission.go:498-502` |
| INV-44 证据归属 | Addendum 交 Lead/Reviewer（本文件不写台账）；工作树行已出现但内容不全 | `Addendum:333-337`；`evidence-matrix.md:64` → **S-5** |

### 6.2 与 SPEC-0001 v1.5（AC-27 / AC-27A / §7.5）的一致性

- **Addendum 未替 PO 改 AC-27 语义**：只声明“AC-27 属 SPEC-0001、当前 AC-REF、不改变任何 AC 状态”，并对 v1.5 采取“待 QC delta、生效后以其为准”的保守口径（`Addendum:12-13`、`:429`）。
- **与 AC-27A 不冲突**：v1.5 的 AC-27A 明确“释放事实由 Inventory 拥有（INV-44），Order 不得写 `RELEASED`”“`reservation_state` 不参与关闭谓词”，与 Addendum §1.5 的“多组不释放、不得声称 INV-44 已完成”方向一致（`docs/specs/0001-fulfillment-quantity-split.md:585-624`（工作树）、`Addendum:131-140`）。
- **§1.5 的“AC-27 释放腿”措辞**仍可解释（AC-27 EARS-3 保留“预留已释放→组合合法”分支），但 v1.5 已把“释放腿”从 AC-27 主投影中剥离；建议（S-9 同类）改为“AC-27 EARS-3 / INV-44”双引用。
- **v1.5 自身的治理张力（ROUTED，不是本 Addendum 的问题）**：v1.5 工作树已写 `status: approved`、`approved_at: 2026-11-26`，而 `docs/review-tracking.md:349-353` 明写“QC delta review is still required before implementation”。本报告**不**构成 v1.5 的 QC delta，也**不**为 v1.5 背书；v1.5 需独立 QC 复核（owner=QC，门槛=实现前）。

### 6.3 `0022`/`0023` 的 SQL 设计（只读核，不执行）

- 等值 CHECK 可精确复现的前提成立：`legacy_payment_resolution.entered_at/expires_at` 均为 `DATETIME(3)`（`0019:33-34`），`NOW(3)` 与 `DEFAULT CURRENT_TIMESTAMP(3)` 同语句同值；Addendum 已把驱动时区差异列为假设 #2 与停止条件（`Addendum:484`）。
- 不设容忍带、保留 `0019` 的 `chk_lpr_expiry`（`>=`）与等值不冲突（`Addendum:262-263`）。
- 分支 A 的“先 ADD 新名再 DROP 旧名”避免 MySQL DDL 非事务性的约束缺失窗口（`Addendum:304`），设计正确；`0023` 的脏行 precheck 未强制，但 ALTER 会硬停并落入 §2.5 流程（`Addendum:345-349`）。

---

## 7. 宪法 Article I–V 与 Governance 核对

| 条款 | 结论 | 依据 |
|---|---|---|
| I.1 拒绝“屎山雕花” | 通过：门禁每个拒绝分支都是退出，无“降级 paid”的 else | `Addendum:469`；`settlement_switch.go:88-118` |
| I.2 Fail-Closed | 通过：不可读/不可确认 → 失败；preflight“查不到=已 apply” | `Addendum:162,306-308` |
| I.3 契约优先 | 通过：release event v1 字段表 + consumer 约束先在 ADR 文本固化 | `Addendum:97-116` |
| I.4 拒绝自我妥协 | 通过：metric/writer/worker/source 显式延期，不“假装完成” | `Addendum:369-382` |
| II.1 事实分离 | 通过：不新增 RPC、不复用 `order.cancelled`、不把 T3 额度释放混入 | `Addendum:386-398` |
| II.3 幂等与最终一致性 | 通过：`release_id`/每单一次/`RELEASED` no-op/outbox 同事务 | `Addendum:118-129` |
| II.5 退款与履约准入 | 通过：permit CAS 与 release 同事务，Addendum 不改协议 | `Addendum:39,501`；`refund_admission.go:256-290` |
| III.4 启动 fail-fast | 通过：exit 1/2/3/4 + reason token，panic 仅限 `cmd/*` | `Addendum:203-228` |
| IV.1 证据不可妥协 | 通过：AC-REF ≠ AC-VERIFIED；证据须真实日志绑定 HEAD；Skip 不算 | `Addendum:13,228,338` |
| IV.4 实现/发布隔离 | 通过：明确 DRAFT 不构成 PASS/Release | `Addendum:5,510-512` |
| Workflow 9 stop-the-line | 通过：preflight stop-the-line / T2 冻结 / metric 扩路径停止条件 | `Addendum:310,276,375-376` |
| Governance 6 豁免记录 | 通过：仅引用 WIP waiver（`review-tracking.md:187-192`），不扩大；人类接受不豁免 | `Addendum:33,88` |
| Governance 2（subagent 自读） | 满足：本报告由 QC 独立读取宪法/AGENTS/CONVENTIONS/角色 prompt 后出具 | — |
| 角色边界 | 满足：QC 只出报告，未改 Addendum/SPEC/ADR/代码/迁移/台账 | §8 |

---

## 8. 已验证成立的部分（避免只列漏洞）

1. **N-1 的顺序修复是实质性的**：preflight 从 `R0.4d2` 前移到 `R0.4c-impl` 编辑前，并把“不可达/无权限”也归入“视为已 apply”，这是 r2 缺口的正确修补（`Addendum:281,306-308`）。
2. **N-2 的收敛方式正确**：没有为过验收而扩大 `allowed_paths`，而是把 metric 移出并登记，同时撤回错误的 PASS 条件（`Addendum:138,411`）。
3. **N-5 的落点选择有理由且可验收**：新增 fulfillment `0012` 会让存量空 range 行迁移失败、UPDATE 审计行又违反不可改写，ops 模板 + 0 行校验是可行解（`Addendum:192-200`）。
4. **N-6 的二选一已定**：不再声称“所有拒绝日志三字段”，config 保留 panic + 稳定 token，与 `cmd/*` 启动期 panic 的宪法允许一致（`Addendum:215-222`）。
5. **N-7 的裁定方向安全**：新 source、consumer-first、无 Amendment 则冻结，避免 producer 提前发送造成全量 DLQ（`Addendum:270-276`）。
6. **N-9 的归属明确**：24h worker 归 Order（W10/T2.2/T2.4），不假装 CHECK 能推进状态（`Addendum:269`）。
7. **r3 未越界**：`git diff` 只含 3 个文件（Addendum + Lead evidence-matrix + PO SPEC-0001 v1.5，后两者非 Architect 产物）；Architect 只改了 Addendum；未改 ADR Rev 2.3/迁移/代码/台账。
8. **引用抽查全部命中**：ADR `:19/:204/:712/:723-724/:821-822/:827/:831-832/:834/:836/:848/:856/:864/:940/:964/:967/:1285/:1607`、`registry.go:198-215/225-302/320-323`、`consumer.go:68-70`、`outbox.go:58-61`、`config.go:752,760,766,775`、`settlement_switch.go:95,121-126,147-148,176,195,200`、`0010:42,52`、`0019:33-34,51`、`0017:45-48,73-80`、`0021:29-36`、`verify-r01-migrations.sh:28-35,117-123`、`test-migrate-runner.sh:45-56`、`01-schemas.sh:41-71`、`run.sh:105-152` 均与文中所引一致（S-9 的三处除外）。

---

## 9. 边界、`[不确定]` 与角色声明

- 本报告是 **QC 红队报告**，不是 Reviewer verdict；不批准 merge/Release，不代替人类接受 Addendum 或 v1.5。
- **未执行**：`go test`、`-race`、迁移、`docker compose config`、Helm 渲染。因此 WIP commit message 与 `r04_checks_integration_test.go` 的“通过”不作为证据。
- `[不确定]`：0022 是否已在某个持久环境 apply（preflight 才能定案；按 `01-schemas.sh` 逻辑，全新持久卷会 apply 0022）；MySQL 8.4 `ADD CHECK` 的 `ALGORITHM/LOCK` 与 MDL 时长；`NOW(3)` 与 `DEFAULT CURRENT_TIMESTAMP(3)` 在特定驱动/时区下的逐位一致性（Addendum 假设 #2 已登记）；`main_test.go` 的 `TEST_MYSQL_DSN` 在 CI 是否可用。
- **只改了本文件**：`docs/collab/qc-reports/QC-PAY-SETTLE-01-R0.4-ADDENDUM-R3.md`。未改 Addendum/ADR/SPEC/tasks/evidence-matrix/ac-coverage/迁移/代码；未 commit。
- **ROUTED（非本角色产物，需 owner 处理）**：Lead——Plan 增加 `-impl` 行、`tasks.md:35` 改 `0021+0022`、INV-44 行补 metric 延期并 commit、Plan 报告文件名指针；Architect——S-1/S-2/S-3/S-4/S-6/S-7/S-8/S-9 的文本修正、ADR Rev 2.4 errata、T2 `legacy_24h` Amendment；PO/QC——SPEC-0001 v1.5 的独立 QC delta；ops——批准行模板/0 行校验/到期告警。
- **下游**：本报告 **READY**（可交人类接受）→ R0.4a3 人类设计接受（记录于 `docs/review-tracking.md`）→ `R0.4b-impl`/`R0.4c-impl`（preflight 先行）/`R0.4d1`/`R0.4d2` → `R0.4e` Reviewer 日期化 verdict（`release_status=NOT RELEASE`）。任何一步 FAIL 冻结 `PAY-SETTLE-01` 的 R0.4 范围，不得用新 `change_id` 绕过。
