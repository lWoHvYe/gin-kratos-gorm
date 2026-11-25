# REV-PAY-SETTLE-01-R0.4-R2 — R0.4e 复审（F-1/F-2 修复，分支 A）

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Tier | **L2（确认）**：`migrations/**`、`scripts/**`、`docs/ops.md`；跨服务迁移/运维窗口；不降级 |
| review_scope | **只复审 R0.4e 的 F-1/F-2 关闭与新增风险**：分支 A（0022 冻结 + 0023 替换）、F-1 锁/MDL/维护窗、UAT-as-prod、checksum、replay/partial、既有 open 项是否仍显式登记。**不重审全部 R0.4 行为**（启动门禁/exit 2/3/4/支付/库存语义仍以 `REV-PAY-SETTLE-01-R0.4.md` 的通过项为准） |
| base | **`da0cdd15d065a503ffa1e6d545e7381f6766737d`**（R0.4 WIP + 首轮 Reviewer/Lead docs 后） |
| code head | **`aec9ac86b68c885bfea0de18dc2be4518e075042`**（5 files，+510/−213） |
| head（复审绑定） | **`4549ccb1ce72c044da9a4c589cbec33e652c52a4`**（selfcheck；本报告所有重跑在此 detached worktree 上执行，`WORKTREE clean`） |
| 后续 docs-only delta | 当前 `main` 为 `ba97a86`（branch A/UAT-as-prod/AC-50-FIX-01 决策）；`git diff --stat 4549ccb..ba97a86 -- ':(exclude)docs'` 为空，代码与 4549ccb 一致 |
| 契约输入 | Addendum `c432ed6`（ACCEPTED 0.3 r3）§2.2/§2.3.3（分支 A）/§2.4/§2.5/§2.6；人类 2026-11-26 决策（`ba97a86`：branch A、UAT-as-prod、AC-50-FIX-01 启动）；SPEC-0002 v1.10；SPEC-0001 v1.5 |
| integration_owner | System Architect（契约）；Developer（迁移/脚本/runbook）；ops（维护窗执行/回放） |
| baseline_impact | **涉及** INV-44（多组/missing metric 仍 DEFERRED）、INV-46（24h CHECK 迁移到 0023；writer/worker 仍 T2）、INV-49（shadow 不变）；不涉及 INV-45/47/50 的语义变更 |
| INV | INV-44 AC-REF；INV-45 DEFERRED(T3)；INV-46 DEFERRED(T2)；INV-47 DEFERRED(T3)；INV-49 DEFERRED(T4.4)；INV-50 AC-REF（均未 VERIFIED） |
| affected_AC | 无新增/变更 AC 状态；AC-27 仍 **AC-REF**（R0.3-R2 结论不变）；AC-50-FIX-01/AC-27-NOWB-01/AC-27-LIVE-01 仍 open |
| verification | §3：在 **HEAD 4549ccb** 重跑 `go test TestR04Checks`（plain 4.452s / `-race` 7.232s）与 `scripts/verify-r01-migrations.sh`（EXIT=0，WORKTREE clean）；F-1 原始实验 + exact-head 1845 probe；F-2 preflight |
| 审查日期 | **2026-11-26** |
| Actor / Reviewed-by | `reviewer-r04` / `human (merge pending)` |
| release_status | **NOT RELEASE**；不批准 merge / Release / AC-VERIFIED |
| overall verdict | **PASS（R0.4e-R2，scoped）**：**F-1 CLOSED、F-2 CLOSED**；未发现新阻断。仍有 Lead-owned 文档漂移与既有 open 项，见 §4；不声称 Release/merge/AC-VERIFIED |

> **语义隔离**：PASS 只覆盖本次 F-1/F-2 复审与新增风险；R0.4e 首轮通过项（门禁/exit/0022 原行为）不重审；R0.3-R2 的 AC-27 AC-REF 不变；Phase 0-R 仍不可收口。

---

## 0. 结论

**F-1 CLOSED，F-2 CLOSED；R0.4e overall PASS（scoped）。**

- **F-2（分支 A）**：`0022` 已逐字节恢复为 `90b502c` 的 blob `240b45f` / sha256 `8339df07…`；所有语义修正进入 `0023_r04_checks_v2.sql`；fresh apply 为 `0017→…→0022→0023`；preflight 对不可达 staging/prod 按 Addendum §2.3.3 fail-closed 归入分支 A，UAT 只读、未做 DDL；可达环境与 fresh apply 的 0022 ledger 均为 `8339df…`，没有 `85a2fc…`（分支 B）checksum 破坏。
- **F-1（锁/MDL/维护窗）**：真实 MySQL **8.4.13**、500,000 `refund_applications` + 200,000 `legacy_payment_resolution` 行实测：`ADD CHECK ... ALGORITHM=INPLACE, LOCK=NONE` 被 **ERROR 1845** 拒绝；`COPY` 下 `SHARED_NO_WRITE` MDL 持续 43.7s/16.0s，写探针被阻塞 43.8s/16.1s；`DROP CHECK` 在线（0.014–0.022s）。`docs/ops.md` 已写 UAT-as-prod / 生产维护窗（备份、停写、应用、复验、回滚）与失败 attempt 流程；exact-head 脚本也内置 1845 probe。
- **未发现新阻断**。仍有 **Lead-owned 文档漂移**（tasks/evidence-matrix 仍指向 0022 分支 B/旧 evidence path）、F-1 原始日志绑定的小瑕疵（header `da0cdd1`，非 `aec9ac8`）与既有的 INV-44/46、9 MINOR、N-1 标签问题；这些均不改变 F-1/F-2 CLOSED，但应在收口前清理。

---

## 1. F-2：分支 A 与 `0022` 字节冻结

### 1.1 字节与 diff（HEAD 4549ccb）

```text
git rev-parse 90b502c:migrations/order/0022_r04_checks.sql  = 240b45fc44a9be5b266901406357d0945ad6c0c5
git rev-parse 4549ccb:migrations/order/0022_r04_checks.sql  = 240b45fc44a9be5b266901406357d0945ad6c0c5
git diff --stat 90b502c..4549ccb -- migrations/order/0022_r04_checks.sql  (empty)
sha256 worktree 0022 = 8339df0741584ef67c76e7daf417c2d6dee57ed26a7d1cf95ba0ef88cf574626
sha256 0023          = 561a57fb366894870c4db4a802f903dc81b19add9238335bca76d17c5122fd7d
```

| 检查 | 结论 |
|---|---|
| `0022` 逐字节恢复 | **PASS**；`git diff 90b502c..HEAD -- 0022` 为空，blob/sha 与 pre-edit 一致 |
| 语义修正落在 `0023` | **PASS**；`0023` 120 行，含两个 `_v2` 约束、precheck、失败重跑头注释 |
| 先 ADD `_v2` 再 DROP 旧名 | **PASS**；`0023:68-99` 两个 ADD 成功后，`0023:101-120` 才 DROP `chk_refund_app_group_discriminator`/`chk_lpr_expiry_24h`；无“先删后加”的缺失窗 |
| 无缺失约束窗口 | **PASS（可证的 partial 态）**；48h 脏行使第二个 v2 ADD 失败时，第一个 v2 已提交、两个旧约束仍在、旧名未删；脚本/测试断言 |
| 判别式不复制 0017 枚举 | **PASS**；`0023:60-76` 只表达 `(group_no IS NULL) = (source IN ('payment','legacy_expired'))` + 组级非空；CHECK_CLAUSE 断言不含 `system_zero` 等四值 |
| 24h 等值 | **PASS**；`0023:97` 为 `expires_at = DATE_ADD(entered_at, INTERVAL 24 HOUR)`；48h/24h+1s/25h 拒绝 |
| replay | **PASS**；0023 重跑 rc=0，仍只有两个 `_v2`，旧名不复活（脚本/测试） |
| 失败 attempt 流程 | **PASS**；`0023:35-43`、`docs/ops.md:341-360` 与 Addendum §2.5 一致：定位→审计写→删 `FAILED/RUNNING` attempt→重跑→复验；0022 旧头注释的“re-run safe”被明确标记为错误/被取代 |

### 1.2 fresh apply / 脚本 entry

`scripts/verify-r01-migrations.sh:33-42` 的 `MIGRATION_FILES` 含 `order/0021`、`order/0022`、`order/0023`；exact-head 日志显示真实 runner 依次 apply `0017 → … → 0022 → 0023`，ledger：

```text
OK ledger order/0022_r04_checks.sql = 8339df0741584ef67c76e7daf417c2d6dee57ed26a7d1cf95ba0ef88cf574626
OK ledger order/0023_r04_checks_v2.sql = 561a57fb366894870c4db4a802f903dc81b19add9238335bca76d17c5122fd7d
```

### 1.3 checksum 影响（应不破坏）

- 可达持久环境（k3s `mall-data`、`mall-uat-data`、本机 mysql-unicorn、compose dev）在 preflight 中 **0022/0023 ledger 均 0 行**，甚至未 apply 到 0017；不存在“分支 B 的 `85a2fc…` checksum”可被破坏。
- fresh apply 记录的是恢复后的 `8339df…`；0023 是独立新文件 `561a57…`，不会改写 0022 ledger。
- **残余**：若某个不可达的 prod 曾 apply 分支 B 的 0022（`85a2fc…`），runner 会 checksum mismatch。`docs/ops.md:299-304` 已将其定义为数据完整性事件（不改文件、维护窗重发、记录 `review-tracking`）；preflight 对 staging/prod 的 fail-closed 归入分支 A 是人类的 2026-11-26 裁决。该残余不构成本次代码阻断，但必须在维护窗前用 ops 环境清单再次确认。

### 1.4 UAT-as-prod

- 人类决策 `ba97a86`：UAT 暂时按 prod 对待（preflight/证据/锁/维护窗口径）。
- `docs/ops.md:280-305` 明确 UAT-as-prod 规则；`docs/ops.md:306-341` 给维护窗步骤。
- `.tmp/evidence-r04-fix/preflight.sh` 对 `mall-uat-data` 只执行只读 SQL（`SELECT`），preflight 日志标注 `UAT, treated as prod; read-only`；**未执行任何 ALTER/INSERT/UPDATE/DELETE**。UAT 未做 DDL 的口径落实。

---

## 2. F-1：锁 / MDL / 维护窗

### 2.1 真实 8.4 实测（`f1-lock-experiment.log`，500k/200k 行）

环境：disposable `mysql:8.4` **8.4.13**；`refund_applications` 500,000 行（`DATA_LENGTH` 66,715,648 B），`legacy_payment_resolution` 200,000 行（26,804,224 B）；独立会话采样 `performance_schema.metadata_locks`，并发写探针 ~20ms 一次。

| 操作 | 结果 |
|---|---|
| `ADD CHECK ... ALGORITHM=INPLACE, LOCK=NONE`（两表） | **拒绝**：`ERROR 1845 (0A000): ALGORITHM=INPLACE is not supported... Try ALGORITHM=COPY.` |
| `ADD CHECK ... ALGORITHM=COPY, LOCK=SHARED`（refund_applications） | 成功；wall 45.039s / statement **43.744s**；MDL `SHARED_NO_WRITE` GRANTED +0.37s→+43.4s；并发 `SHARED_WRITE` PENDING；写探针 max **43,751ms**，0 错误 |
| `ADD CHECK ... ALGORITHM=COPY, LOCK=SHARED`（legacy_payment_resolution） | 成功；wall 17.364s / statement **16.044s**；MDL `SHARED_NO_WRITE` +0.35s→+15.8s；写探针 max **16,067ms** |
| `DROP CHECK ... ALGORITHM=INPLACE, LOCK=NONE`（两表） | **接受（在线）**；语句 **0.022s / 0.014s**；写探针 max **49ms/54ms**，无 `SHARED_NO_WRITE` 采样 |
| `0022` apply（旧 `>=`/枚举约束，500k/200k） | 56.5s |
| 真实 `0023` 端到端（同规模，无并发写） | 49.741s；再 replay rc=0，仍只剩两个 `_v2` |

**结论**：MySQL 8.4 的 `ADD CHECK` 只能 `COPY`（表重建 + `SHARED_NO_WRITE` MDL，窗口内阻塞写）；`DROP CHECK` 在线。`0023` 的两个 ADD 必须进停写维护窗；`docs/ops.md` 已按此写，未声称在线 DDL。

### 2.2 exact-head 复现

在 **HEAD 4549ccb**、真实 `mysql:8.4.13` + `mall/migrate:dev` 的 exact-head 脚本日志中：

```text
== F-1 DDL lock profile: is ADD CHECK online on this MySQL? ==
OK   ADD CHECK rejects ALGORITHM=INPLACE, LOCK=NONE (rejected: 1845)
OK   ADD CHECK is COPY-only on 8.4.13 -> write-frozen maintenance window required
OK   DROP CHECK accepts ALGORITHM=INPLACE, LOCK=NONE (online)
OK   DDL probe left no probe CHECK = 0
```

- exact-head 脚本复现 1845/COPY-only/在线 DROP；详细 MDL/时长来自 `f1-lock-experiment.log`。
- **绑定小瑕疵（不阻断）**：`f1-lock-experiment.log` 头写 `HEAD da0cdd1`（commit 前工作树），而 selfcheck 声称绑定 `aec9ac8`。exact-head 脚本已复现 1845，`0023` 的 sha `561a57…` 与实验后的约束文本一致；建议在收口前重跑该 500k 实验于 `aec9ac8`/`4549ccb` 或修正日志头，避免“绑定 aec9ac8”与原始 header 不一致。

### 2.3 维护窗 / 回滚 / UAT

- `docs/ops.md:306-341`：批准/公告窗口 → 先测行数并按 F-1 放大 → `mysqldump` + sha → 停 Order 写 → 跑 0022/0023 precheck（0 行）→ 跑迁移 → 复验 v2/旧名/CHECK_CLAUSE/ledger → 恢复写入。
- 回滚：0023 前 kill 可回滚 COPY ADD；0023 后 restore dump 或前向修复；禁止 DROP v2 / 批量 UPDATE 掩盖 / 编辑 0022。
- 失败 attempt：定位 → 审计写 → 删 `FAILED/RUNNING` → 重跑 0023（v2 ADD 跳过，旧名 DROP 收敛）→ 复验。
- UAT-as-prod：只读 preflight，未做 DDL；维护窗执行归 ops/Director。
- **结论：F-1 CLOSED**（锁/MDL/时长/维护窗/UAT 口径均有可核验证据；绑定小瑕疵见 §4）。

---

## 3. 重跑证据（HEAD 4549ccb，WORKTREE clean）

在 `.tmp/wt-4549ccb`（detached HEAD `4549ccb`）执行：

```bash
# MySQL 8.4.13，per-package schema
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:13306)/mall_integration_reviewer_r2_exact?...' \
  go test ./internal/services/order -run 'TestR04Checks' -count=1 -v
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:13306)/mall_integration_reviewer_r2_exact_race?...' \
  go test -race ./internal/services/order -run 'TestR04Checks' -count=1
MYSQL_IMAGE=mysql:8.4 MIGRATE_IMAGE=mall/migrate:dev ./scripts/verify-r01-migrations.sh
```

| 命令 | 结果 |
|---|---|
| `TestR04ChecksFreshSchemaAndValidRows` | **PASS** 1.18s |
| `TestR04ChecksDirtyRowsBlockMigration`（3 子） | **PASS** 3.11s：48h 阻塞/收敛、partial v2 added-old not dropped 收敛、discriminator 仍被 0022 阻塞 |
| `-race` 同 | **PASS** `ok 7.232s` |
| `verify-r01-migrations.sh` | **EXIT=0**；MySQL 8.4.13；`WORKTREE clean`；fresh `0017→0023`；ledger `0022=8339df…`、`0023=561a57…`；F-1 probe 1845；partial/replay/runner 失败 attempt 全 OK |
| `0022` diff | **空** |

原始日志：`.tmp/evidence-r04-fix/reviewer-r2-order-tests-4549ccb.log`、`verify-r01-migrations-REVIEWER-R2-4549ccb.log`（本次审查生成，未入库）。

---

## 4. 新增风险 / 仍 open 的显式登记

**未发现新阻断。** 以下为需在收口前清理的 **[MINOR]/[TODO]（不改变 F-1/F-2 CLOSED）**：

1. **F-1 原始日志绑定漂移**：`f1-lock-experiment.log` header `HEAD da0cdd1`，selfcheck 声称 `aec9ac8`；exact-head 脚本复现 1845，且 `0023` sha 一致。建议重跑 500k 实验于最终 HEAD 或修正日志头。owner=Developer。
2. **`0022` 冻结头仍含“re-run safe”错误句**：分支 A 禁止编辑 0022；`0023:35-43` 与 `docs/ops.md:341-360` 已明确该说法错误并被取代。读者只看 0022 仍可能误读；建议在 `0023`/ops 已有指针基础上，由 Architect 在 Addendum/errata 记录“0022 header 为冻结历史，不作为口径源”。owner=Architect。
3. **Lead-owned 文档漂移**：
   - `docs/lead/PAY-SETTLE-01/tasks.md:52` 仍写“0022 分支 B / Reviewer R0.4e 审查中”，未反映 branch A/`aec9ac8`；
   - `docs/lead/PAY-SETTLE-01/evidence-matrix.md` 的 INV-46/AC-63 evidence path 仍写 `0022 chk_lpr_expiry_24h`，应指向 `0023 chk_lpr_expiry_24h_v2`；
   - INV-44 行已补 missing-reservation metric（N-1 内容关闭），但文字误写“Reviewer R0.4e **F-2** 指出 metric 缺口”（metric 是 N-1；F-2 是 preflight）；
   - selfcheck §8 仍说“未处理 N-1”，与 Lead 已在 `da0cdd1` 更新 INV-44 行不一致。
   owner=Lead/Architect；均为文档漂移，不改变代码/证据结论。
4. **仍 open 的显式登记保留**：
   - INV-44 仍 **AC-REF**（多组 + missing-reservation metric DEFERRED，owner Developer/ops，2026-12-31）；INV-46 仍 **DEFERRED(T2)**（writer/worker/`legacy_24h` source 未交付）；INV-45/47/49 DEFERRED；INV-48/50 AC-REF。
   - QC round 3 的 9 MINOR：S-3/S-4/S-5 已有代码/文档侧处理；S-1/S-2/S-6/S-7/S-8/S-9 仍为 Architect/Lead/ops 的 TODO；selfcheck §8 明确“不因本文档关闭”。保留登记。
   - `AC-50-FIX-01`、`AC-27-NOWB-01`、`AC-27-LIVE-01` 仍 open；AC-27 仍 **AC-REF**，不得 AC-VERIFIED。人类 `ba97a86` 已授权启动 AC-50-FIX-01（ADR §5.3 amendment 先行），但尚未实现。
   - R0.4e 首轮的 N-2（Addendum “空 mode”矛盾）、N-3（compose 空值）、N-4（0022 同名不同义）、N-5（ops 告警演练）、N-6（9 MINOR）、N-7（matrix AC-27 旧表述）仍按原 owner/expiry；本修复未关闭它们。
5. **维护窗未执行**：本次未在 UAT/staging/prod 执行 0023 的 COPY ADD；ops 维护窗是发布前门禁，不是 R0.4e 的实现 PASS 条件。`release_status=NOT RELEASE` 不变。

---

## 5. 宪法 Article I–V / Governance 核对

| 条款 | 结论 | 依据 |
|---|---|---|
| I.1 拒绝“屎山雕花” | **PASS** | 没有用局部 if-else/“重跑就好”掩盖；分支 A 用独立 0023 替换约束，先 ADD 后 DROP，无缺失窗 |
| I.2 Fail-Closed | **PASS** | 48h 脏行让 0023 停在第二个 ADD，旧约束保留；不可达 staging/prod 按 fail-closed 归入分支 A；UAT 只读 |
| I.3 契约优先 | **PASS** | 0022 冻结；语义修正与 Addendum §2.2/§2.3 一致；未改 0017-0021、proto/RPC |
| I.4 拒绝自我妥协 | **PASS** | 维护窗明确为“停写 + COPY”，未把 ADD CHECK 说成在线；F-1 原始日志/维护窗步骤可核 |
| II.1 事实分离 | **PASS** | 未改 C-04 release 事实链；0023 只替换两个 CHECK，不引入状态字段/RPC |
| II.3 幂等/最终一致 | **PASS** | 0023 replay 幂等；失败 attempt 需人工删行；runner 拒绝自动重放 |
| III.2 DB 访问 | **PASS** | 显式 SQL/迁移文件；无 GORM/生产写入代码变更 |
| III.4 静态/启动 | **PASS** | 不改 `cmd/**`/`internal/platform/**`；无新 panic/包依赖 |
| IV.1 证据不可妥协 | **PASS** | exact-head `WORKTREE clean`、四类日志、ledger、F-1 probe；F-1 原始 500k 日志绑定小瑕疵已列 MINOR |
| IV.2 重灾区抗压 | **PASS（有范围）** | 迁移 partial/dirty/replay/失败 attempt + 并发写探针（F-1）；未执行生产维护窗，已明确 |
| IV.4 实现/发布隔离 | **PASS** | NOT RELEASE；不声称生产迁移已验证/Release PASS |
| Workflow 9 stop-the-line | **PASS（修复路径）** | 首轮 FAIL 后未新开 change_id；分支 A 修复挂 R0.4e-R2；仍保留人类 merge |
| Governance 4 人类检查点 | **PASS** | 不批准 merge；人类 merge pending |
| Governance 6 人类豁免 | **PASS** | 仅引用既有 WIP waiver；不扩大 |
| 角色独立性 | **PASS** | Actor `reviewer-r04` ≠ Developer `dev-r04-fix`；只读代码/契约，只建本报告 |

**`[ARCHITECTURAL_FLAW]`：无。** 分支 A 是 Addendum 预设的两条合法路径之一；0023 替换方式符合“先 ADD 后 DROP”的契约要求。F-1 的维护窗是运维执行项，不是架构缺陷。

---

## 6. 后续建议（PASS，最多 3 条）

1. **清理 Lead-owned 文档漂移**：`tasks.md` 改 branch A/`aec9ac8`；`evidence-matrix.md` 的 INV-46/AC-63 path 改 `0023 chk_lpr_expiry_24h_v2`；修正 INV-44 行的“F-2”标签与 selfcheck §8 的 N-1 描述。**在收口前完成，不作为 R0.4e-R2 阻断。**
2. **补 F-1 绑定**：在最终 HEAD 重跑 500k/200k 的 MDL/时长实验，或把 `f1-lock-experiment.log` 头改为 `da0cdd1` 工作树 + 说明 `0023` sha `561a57…` 与 exact-head 复现关系；避免 selfcheck“绑定 aec9ac8”与 raw header 不一致。
3. **维护窗作为发布门禁**：UAT-as-prod/prod 执行前按 `docs/ops.md:306-341` 重新测行数、备份、停写、复验；`0022` 若出现 `85a2fc…` checksum 必须按数据完整性事件处理。**不得**以本次 R0.4e-R2 PASS 代替 ops 窗口/Release 批准。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-26
**报告**：`docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.4-R2.md`
