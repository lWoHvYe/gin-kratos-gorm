# PAY-SETTLE-01 R0.4-FIX Developer 自检清单（分支 A + F-1/F-2）

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Role / Actor | Developer / `dev-r04-fix` |
| Tier | **L2**（`migrations/**` + `scripts/**` + `docs/ops.md`；命中迁移/工具链触发条） |
| 触发输入 | Reviewer R0.4e **FAIL**（`docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.4.md`：F-1 锁/MDL/维护窗缺失、F-2 preflight 环境覆盖不足）；人类 2026-11-26 裁决**分支 A**、**UAT 暂时按 prod**（`docs/review-tracking.md` 的 2026-11-26 human decisions 行，Lead 落盘） |
| 契约输入 | `docs/adr/0007-amd1-addendum-r0.4-c04-release.md`（ACCEPTED 0.3 r3）§2.2、§2.3.3（分支 A）、§2.4、§2.5、§2.6 |
| Base（本修复父提交） | `da0cdd15d065a503ffa1e6d545e7381f6766737d`（R0.4 WIP `c38c54f` + selfcheck `9322e75` + Lead/Reviewer docs） |
| **Code head（本文全部证据绑定）** | **`aec9ac86b68c885bfea0de18dc2be4518e075042`** |
| 本次改动 | 5 files，+510/−213（§4）；`0023` 新增、`0022` 恢复、迁移测试/脚本/ops 更新 |
| allowed_paths（实际改动） | `migrations/order/0022_r04_checks.sql`（仅恢复字节）、`migrations/order/0023_r04_checks_v2.sql`、`internal/services/order/r04_checks_integration_test.go`（仅迁移测试）、`scripts/verify-r01-migrations.sh`、`docs/ops.md`、本文档 |
| forbidden_paths（未触碰） | `internal/services/order/**` 生产代码、`internal/services/fulfillment/**`、`internal/platform/config/**`、`api/**`、`deploy/**`、`migrations/order/0017–0021`、SPEC/ADR、`docs/lead/**`、`docs/collab/**`、台账（`ac-coverage.tsv`/`evidence-matrix`/`tasks`/`review-tracking`） |
| Status | **WIP**；不声称实现 PASS / R0.4e PASS / merge / Release PASS；**不标 AC-VERIFIED** |

---

## 1. F-2：切换到分支 A（`0022` 逐字节恢复 + `0023` 替换）

### 1.1 `0022` 逐字节恢复（证据）

```bash
git checkout 90b502c -- migrations/order/0022_r04_checks.sql
shasum -a 256 migrations/order/0022_r04_checks.sql
# 8339df0741584ef67c76e7daf417c2d6dee57ed26a7d1cf95ba0ef88cf574626

git rev-parse 90b502c:migrations/order/0022_r04_checks.sql
# 240b45fc44a9be5b266901406357d0945ad6c0c5
git show aec9ac8:migrations/order/0022_r04_checks.sql | shasum -a 256
# 8339df0741584ef67c76e7daf417c2d6dee57ed26a7d1cf95ba0ef88cf574626

git diff 90b502c aec9ac8 -- migrations/order/0022_r04_checks.sql
# (empty)
git diff --stat 90b502c aec9ac8 -- migrations/order/0022_r04_checks.sql migrations/order/0023_r04_checks_v2.sql
#  migrations/order/0023_r04_checks_v2.sql | 120 ++++++++++++++++++++++++++++++++
#  1 file changed, 120 insertions(+)
```

- 分支 A 的 `git diff --stat`（vs `90b502c`）**不含 `0022`**：只新增 `0023`。
- `0023` sha256：`561a57fb366894870c4db4a802f903dc81b19add9238335bca76d17c5122fd7d`（与验证脚本 ledger 断言一致）。
- 相对父提交 `da0cdd1`，`0022` 会显示 74 行变化，因为分支 B 曾就地改过它；本修复把内容**恢复**为 `90b502c` 字节，语义修正全部移入 `0023`。

### 1.2 `0023` 替换契约（`migrations/order/0023_r04_checks_v2.sql`）

- 先 `ADD` 两个新名：`chk_refund_app_group_discriminator_v2`（判别式只表达 `(group_no IS NULL) = (source IN ('payment','legacy_expired'))` 且组级非空串，**不复制 0017 source 枚举**）与 `chk_lpr_expiry_24h_v2`（`expires_at = DATE_ADD(entered_at, INTERVAL 24 HOUR)` 等值）。
- **两个 `ADD` 都成功之后**才 `DROP` 旧名（`chk_refund_app_group_discriminator`、`chk_lpr_expiry_24h`）——任何时刻每张表都有约束，无“约束缺失窗口”；第二个 `ADD` 失败时旧约束仍在，且下一次重跑会跳过已成功的 `ADD`。
- 头注释明确：**`0022` 保持字节不变，语义修正全在 `0023`**；`0022` 旧头注释的“conditional ALTERs make a re-run after cleanup safe”**说法错误**，由本文件与 `docs/ops.md` 纠正（`0022` 字节不可改）：MySQL DDL 隐式提交，失败/中断会留下“一个 CHECK 已装、另一个缺失 + runner 记 FAILED/RUNNING 并拒绝重放”，必须走“定位→审计写修数据→删 attempt 行→重跑→复验”流程。
- 保留 precheck（打印阻塞行）、按 `information_schema` 条件 ADD/DROP（replay 幂等）、无 down migration；`drop` 只在改名后执行。
- 运维副作用（已写入 `docs/ops.md`）：**不得在 0023 之后重放 0022**（runner ledger 不会重放；手工重放会重新装回旧 `>=`/枚举约束）。

---

## 2. F-1：`ADD CHECK` 锁 / MDL / 维护窗证据

环境：一次性 `mysql:8.4`（**8.4.13**，`127.0.0.1:13333`），造数 **500,000 `refund_applications`**（`DATA_LENGTH` 66,715,648 B）+ **200,000 `legacy_payment_resolution`**（26,804,224 B），会话开启 `events_statements_history_long`；MDL 由独立会话在 DDL 执行期间每 ~30 ms 采样 `performance_schema.metadata_locks`；并发写入探针每 ~20 ms INSERT 一次并记录每次耗时。脚本 `.tmp/evidence-r04/f1-lock-experiment.sh`，原始输出 `.tmp/evidence-r04/f1-lock-experiment.log` / `f1-summary.txt` / `f1-mdl-*.raw`（未入库，绑定 HEAD `aec9ac8`）。

| 尝试 | 结果 |
|---|---|
| `ALTER TABLE ... ADD CHECK ..., ALGORITHM=INPLACE, LOCK=NONE`（两表） | **拒绝**：`ERROR 1845 (0A000): ALGORITHM=INPLACE is not supported for this operation. Try ALGORITHM=COPY.` |
| `ALTER TABLE ... ADD CHECK ..., ALGORITHM=COPY, LOCK=SHARED`（`refund_applications`，500k 行） | 成功；wall **45.0 s**，语句 `TIMER_WAIT` **43.744 s**；MDL `SHARED_NO_WRITE` GRANTED 从 **+0.37 s 到 +43.4 s**，并发写会话 `SHARED_WRITE` **PENDING**；写探针被阻塞 **43.8 s**（max 43,751 ms，0 错误） |
| `ADD CHECK ..., ALGORITHM=COPY, LOCK=SHARED`（`legacy_payment_resolution`，200k 行） | 成功；wall **17.4 s**，`TIMER_WAIT` **16.044 s**；MDL `SHARED_NO_WRITE` +0.35 s→+15.8 s；写探针阻塞 **16.1 s** |
| `ALTER TABLE ... DROP CHECK ..., ALGORITHM=INPLACE, LOCK=NONE`（两表） | **接受**（在线）；语句 **0.014–0.022 s**，无 `SHARED_NO_WRITE` 采样，写探针 max **46–54 ms**（基线 ~21–30 ms），0 错误 |
| `0022` 在本规模上 apply（两个 COPY ADD） | **56.5 s** |
| 真实 `0023` 端到端（同规模、无并发写） | **49.7 s**；再重放 rc=0 且只剩两个 `_v2` 约束 |

**结论：`ADD CHECK` 在 MySQL 8.4 上必须 `COPY`（表重建 + `SHARED_NO_WRITE` MDL，全窗口阻塞写），不支持在线 DDL；`DROP CHECK` 在线。** 因此 `0023` 的两个 `ADD` 必须进**停写维护窗**。`docs/ops.md` 已写入：UAT（按 prod）与生产维护窗步骤（预检/备份/停写/应用/复验/恢复）、回滚与恢复流程（无 down migration；重建/前向修复，禁止 `DROP` v2 或批量 UPDATE 掩盖）、失败 attempt 恢复流程。**未在 UAT/staging/prod 执行任何破坏性 DDL**（UAT 仅只读 preflight，见 §3）。

验证脚本也内置了同一结论的机器检查（绑定 HEAD）：`ADD CHECK ... ALGORITHM=INPLACE, LOCK=NONE` 必须报 1845、`COPY` 可用、`DROP CHECK ... INPLACE, LOCK=NONE` 可用（`verify-r01-migrations-AEC9AC8.log` 的 `F-1 DDL lock profile` 段）。

---

## 3. F-2：分支 A 裁决 + 只读 preflight（UAT 按 prod）

人类 2026-11-26 裁决：**分支 A**；**UAT 暂时按 prod** 对待（preflight/证据/锁与维护窗口径）。按 Addendum §2.3.3 fail-closed：任一环境不可达/无法确认 → 视为“已 apply”，走分支 A（`0023`）。本修复的 preflight 全部**只读**；UAT 未执行任何 DDL。

`bash .tmp/evidence-r04-fix/preflight.sh`（HEAD `aec9ac8`，`captured_at 2026-11-25T22:46:50Z`），原始输出 `.tmp/evidence-r04-fix/preflight.log`：

| 环境 | 版本 | `schema_migrations` | `0022`/`0023` 账本行 | attempt 行 | 最高 order 迁移 | 约束/表 |
|---|---|---|---|---|---|---|
| k3s `mall-data` mysql-0（dev，持久 PVC） | 8.4.13 | 存在 | **0 / 0** | **0 / 0** | `order/0013_group_refund_state.sql` | 0022/0023 约束 0；`refund_applications`/`legacy_payment_resolution` 不存在 |
| k3s `mall-uat-data` mysql-0（**UAT = prod 口径**） | 8.4.13 | 存在 | **0 / 0** | **0 / 0** | `order/0013_group_refund_state.sql` | 0022/0023 约束 0；两表不存在 |
| 本机 `mysql-unicorn`（`docker_mysql-db` 卷） | 26.10.0 | **不存在** | 查询不可用 | 查询不可用 | — | 0022/0023 约束 0；`mall_order` 无 ledger 表 |
| compose dev（project `mall`） | — | — | — | — | — | 无 MySQL 容器/卷 |
| staging/prod | 不可达 | `[不确定]` | `[不确定]` | `[不确定]` | — | 按 fail-closed 视为“已 apply” |

- 可达的持久环境均**没有** `0022` 账本行/attempt 残记录（甚至未 apply 到 `0017`），因此不存在“分支 B 已改字节污染 ledger”的可达环境；不可达环境按 fail-closed 规则并入分支 A（与人类裁决一致）。
- `0022` 文件 sha256 与“分支 B 前”字节一致（§1.1）；若任何环境曾记录分支 B 的 `85a2fc82…` checksum，按 `docs/ops.md` 的“数据完整性事件”路径处理（不改文件、走维护窗重发），本 preflight 未发现该情况。
- **UAT 未被用作破坏性 DDL 的试验场**：只读查询 + 维护窗文档（`docs/ops.md`），未执行任何 `ALTER`。

---

## 4. 变更 Diff（`aec9ac8`）

```
 docs/ops.md                                        | 126 +++++++++-
 .../services/order/r04_checks_integration_test.go  | 259 +++++++++++++--------
 migrations/order/0022_r04_checks.sql               |  74 ++----   (恢复为 90b502c 字节)
 migrations/order/0023_r04_checks_v2.sql            | 120 ++++++++++
 scripts/verify-r01-migrations.sh                   | 144 +++++++-----
 5 files changed, 510 insertions(+), 213 deletions(-)
```

- 迁移测试（同一文件）：先 `0017→0019→0021→0022→0023` 真实迁移；断言 `_v2` 存在、旧名不存在、`CHECK_CLAUSE` 为等值/判别式（不复制 0017 枚举）；24h+1s/25h/48h 与 23h 拒绝、24h 接受；`0023` replay 幂等；48h 脏行让 `0023` 停在 v2 24h ADD（旧约束仍在）并可修复收敛；模拟中断态“两个 v2 已加、旧名未删”后重跑收敛。
- 脚本：`MIGRATION_FILES` 含 `order/0021`+`0022`+`0023`；fresh apply 顺序 `0017→0018→0019→0020→0021→0022→0023`；新增 0021 `chk_refund_app_system_zero`、0023 两个 `_v2`、旧名不存在、`CHECK_CLAUSE` 文本断言；replay/脏行/partial/repair 全部改为 0023 口径；新增 F-1 算法探测段。

---

## 5. 命令与日志（绑定 HEAD `aec9ac8`）

```bash
# 1) 真实 MySQL 8.4 + mall/migrate:dev（fresh apply 0017→0023）
MYSQL_IMAGE=mysql:8.4 MIGRATE_IMAGE=mall/migrate:dev ./scripts/verify-r01-migrations.sh
# EXIT=0；HEAD aec9ac8；MySQL 8.4.13；88 个 OK
# ledger: order/0022 = 8339df07…  order/0023 = 561a57fb…
# 日志：.tmp/evidence-r04-fix/verify-r01-migrations-AEC9AC8.log

# 2) 订单包 R0.4 迁移测试（真实 MySQL 8.4.13，独立 schema）
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:13334)/mall_integration_r04fix?parseTime=true&loc=UTC' \
  go test ./internal/services/order -run 'TestR04Checks' -count=1 -v
# PASS；TestR04ChecksFreshSchemaAndValidRows 1.71s；TestR04ChecksDirtyRowsBlockMigration 3.43s（3 子用例全 PASS）
# 日志：.tmp/evidence-r04-fix/go-test-order-r04-AEC9AC8.log

TEST_MYSQL_DSN='...' go test -race ./internal/services/order -run 'TestR04Checks' -count=1
# ok 5.784s（-race）
# 日志：.tmp/evidence-r04-fix/go-test-order-r04-race-AEC9AC8.log

# 3) F-1（一次性 MySQL 8.4.13 + 500k/200k 行 + MDL 采样 + 写探针）
./.tmp/evidence-r04/f1-lock-experiment.sh
# F1 EXPERIMENT COMPLETE；0023 total apply 49.7s；replay rc=0
# 日志：.tmp/evidence-r04/f1-lock-experiment.log

# 4) 只读 preflight（dev / UAT-as-prod / mysql-unicorn / compose dev）
bash .tmp/evidence-r04-fix/preflight.sh
# 日志：.tmp/evidence-r04-fix/preflight.log
```

- `verify` 日志中 `WORKTREE dirty` 的条目来自其他角色文件（`docs/review-tracking.md` 的 Lead 决策行、AC-50 的 ADR/QC 未跟踪文件），不属于本修复；HEAD 本身为 `aec9ac8`。

---

## 6. 【测试范围】与【未覆盖范围】

【测试范围】

- `0022` 冻结字节（sha256/blob/`git diff` 为空）与 `0023` 在两个真实 MySQL 8.4 上下文（测试包临时 schema + `mall/migrate:dev` fresh apply）中的替换语义；
- 新约束存在性、旧名不存在、`CHECK_CLAUSE` 等值/判别式文本（脚本 + Go 断言）；
- 24h 边界（接受恰 24h；拒绝 24h+1s/25h/48h；<24h 由 0019/equality 拒绝）、组级/支付级判别、0017/0019/0021 既有约束未丢；
- `0023` replay、48h 脏行 fail-closed（旧约束保留）、模拟中断态（两个 v2 已加、旧名未删）重跑收敛；
- F-1：`ALGORITHM=INPLACE, LOCK=NONE` 对 `ADD CHECK` 的拒绝（1845）、`COPY` 的 MDL/时长/写阻塞、`DROP CHECK` 在线；
- F-2：三个可达持久环境的只读 preflight + 不可达环境按 fail-closed 并入分支 A；
- `-race`：订单包 R0.4 迁移测试。

【未覆盖范围】

- **未在 UAT/staging/prod 执行维护窗 DDL**（UAT 按 prod 只读；窗口执行归 ops/Director，步骤已写入 `docs/ops.md`）；因此“生产规模锁时长”仍是按 500k/200k 行的合成规模外推，**实际行数必须在窗口前用 `COUNT(*)` 重新测量并按 ops 步骤放大窗口**。
- 24h writer 与 expiry/compensation worker 属 T2（INV-46 仍 DEFERRED）；`CHECK` 只拒绝晚设 deadline，**不推进状态机**。
- INV-44 多组释放、missing-reservation metric/告警仍 DEFERRED（`internal/services/order/**`、`internal/services/inventory/**` 不在 R0.4 允许路径）；evidence-matrix 的 INV-44 行由 Lead 维护。
- T4.4 shadow 运行态、T3 commitment release、AC-27/AC-27A/AC-50 语义验收不在本次范围。
- 未关闭 QC round 3 的 9 个 `[MINOR]`（owner 见 QC 报告）；未处理 R0.4e 的 N-1（Lead-owned INV-44 行）与 N-2..N-7。

---

## 7. 反妥协自检（Developer prompt 强制）

- **并发/超时/部分成功**：`ADD CHECK` 是 `COPY` 且全程阻塞写（F-1 已实测），所以不把迁移说成“在线”；两个 `ADD` 成功后才会 `DROP` 旧名，失败/中断不会留下无约束表；失败 attempt 必须显式删除（runner 拒绝自动重放），未用“重跑就好”掩盖。
- **是否用 if-else / 局部补丁绕过架构**：没有新增任何生产代码分支、锁或状态字段；只做“约束替换 + 迁移证据 + 运维窗口”。没有扩大 `allowed_paths`，没有动 0017–0021、Order/Inventory/Config/Fulfillment 代码。
- **向现实做的妥协（须高亮）**：① 代表性规模是 500k/200k 行的合成数据（无 UAT/prod 破坏性试验），真实行数与窗口时长必须由 ops 在窗口前测量；② 维护窗步骤只是文档化，尚未在 UAT/prod 执行（UAT 按 prod，禁止提前演练 DDL）；③ `0023` 之后手工重放 `0022` 会复活旧约束，已在迁移头注释与 `docs/ops.md` 明示为禁止操作（runner ledger 本身不会重放）。

### 返工影响声明

- `[修改点]`：`migrations/order/0022_r04_checks.sql`（恢复字节）、新增 `migrations/order/0023_r04_checks_v2.sql`、`internal/services/order/r04_checks_integration_test.go`（迁移测试）、`scripts/verify-r01-migrations.sh`、`docs/ops.md`（迁移/维护窗/UAT-as-prod 段）。
- `[架构侵入性]`：无新增业务分支或状态所有权变更；约束语义与 Addendum §2.2/§2.3 一致（只换文件名/替换方式）。
- `[副作用评估]`：① 分支 B 的 `0022` 字节（`85a2fc82…`）被恢复为 `8339df07…`；若某环境曾 apply 分支 B 版本，会出现 ledger checksum 不一致，按 ops 的“数据完整性事件”处理（本 preflight 未发现该环境）。② fresh apply 的 ledger 现在记录 `0022=8339df07…`、`0023=561a57fb…`。③ 0023 之后重放 0022 会重新安装旧约束（禁止）。④ 维护窗内 Order 写入会阻塞（`SHARED_NO_WRITE`），必须先停写。

---

## 8. 明确不声称 / 未关闭项

- **不声称**实现 PASS、R0.4e PASS、merge、Release PASS、AC-VERIFIED；R0.4e 需 Reviewer 重新绑定 HEAD 并重跑。
- INV-46 仍 **DEFERRED(T2)**（writer/worker 未交付）；INV-44 仍为 AC-REF（多组/missing-reservation 延期）；`0022`/`0023` 只提供 DB 兜底。
- `0023` 的“迁移已验证”只指 fresh/replay/脏行/partial 在真实 MySQL 8.4 上通过；**不代表 UAT/prod 窗口已验证**。
- 9 个 QC `[MINOR]` 与 R0.4e 的 N-1/AC-50 FIX 仍 open；不因本文档而关闭。
