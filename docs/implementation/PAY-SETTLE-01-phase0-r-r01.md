# PAY-SETTLE-01 Phase 0-R R0.1 — 真实迁移 apply、0011 CHECK 与 0018 重入修复

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Role / Actor | Developer / `dev-phase1l` |
| Tier | **L2** |
| Tier-Rationale | 命中 `migrations/**` 触发条；Order/Fulfillment 两个服务的结算与退款 schema；资金链路迁移，恒为 L2 |
| Spec-Ref | SPEC-0002 **v1.10**：INV-45（承诺释放纪律）、INV-47（商品/运费组成）、INV-49（`order.settled` 影子不具准入权威）、AC-65（TCC Σ/逐组扇出）、AC-66（shadow/switch 默认不切换）；ADR-0007 **Rev 2.3** §4.3/§6.2/§6.3/§6.4/§7.2/§10/§12.5.6 |
| Base（任务） | `93380d5`（Lead 派单时的 `main` HEAD） |
| Base（实际父提交） | `29bb54a`（派单后 `main` 前进：`2a05c1d` T1.7 evidence + 若干 docs 提交；均未触及 R0.1 路径。`93380d5` 为派单时 HEAD） |
| Code head | **`42a8db5`**（迁移、脚本、测试 helper 与本文件同提交）；post-commit 证据日志头部固定该 hash |
| Evidence log | `docs/implementation/PAY-SETTLE-01-phase0-r-r01-evidence.txt`（在 code head `42a8db5`、clean worktree 上重跑 `scripts/verify-r01-migrations.sh` 的完整输出） |
| allowed_paths | `migrations/order/0018_refund_commitment_composition.sql`、`migrations/fulfillment/0011_switch_expiry_check.sql`、`scripts/verify-r01-migrations.sh`、`internal/services/order/settlement_integration_test.go`（仅测试 helper）、`docs/implementation/PAY-SETTLE-01-phase0-r-r01.md`、`docs/implementation/PAY-SETTLE-01-phase0-r-r01-evidence.txt` |
| 明确不做 | 业务代码、proto、API 契约、ADR/SPEC 文本、`migrations/order/0017/0019/0020`、`migrations/fulfillment/0010`、`docs/lead/**`、AC 台账 |
| Status | **WIP；未批准**。无 `Reviewed-by`；不声称实现 PASS / Release PASS，不标 AC-VERIFIED |

> 本文件是 Developer 的 R0.1 canonical 交付物：记录真实迁移执行、重入/部分失败修复、CHECK 语义、已应用迁移审计、测试范围与未覆盖范围。Reviewer 的日期化 verdict 由 R0.2 另出。

---

## 0. 结论（R0.1 范围）

1. 在 **fresh MySQL 8.4.13** 上，用**真实 `mall/migrate:dev` runner**（`run.sh` 与仓库字节一致）apply 了仓库真实迁移文件；`order/0017-0021` 与 `fulfillment/0010-0011` 全部成功，ledger 记录的 sha256 与文件逐一相等。
2. 新增 `migrations/fulfillment/0011_switch_expiry_check.sql`：`target_mode='settled'` 必须有 `expires_at`（`chk_switch_settled_expiry`）；`shadow` 允许 NULL；**存量违规行 fail-closed 阻断，不自动补默认值**，必须由 Director 重签（ADR §12.5.6）。
3. 修复 `order/0018_refund_commitment_composition.sql` 的重入/部分失败：列存在性检测 + 单条原子 `ALTER` + 幂等 `UPDATE` + `CREATE TABLE IF NOT EXISTS`；在 fresh DB 上验证了成功重跑、ALTER 后中断、单列部分状态三种场景。runner 的 `FAILED` attempt 仍按设计拒绝自动重放，人工删除 attempt 行后同一真实文件收敛。
4. **不改已应用迁移 checksum 的前提成立**：持久环境审计显示 `order/0018` 从未被 checksum-tracked runner 应用（详见 §5）；因此本次允许修改 0018 内容。若存在未审计到的环境，runner 会以 checksum mismatch **fail-closed**，不会静默分叉。
5. `order/0017/0019/0020` 与 `fulfillment/0010` **未修改**；`fulfillment/0010` 保持已合入内容，新约束由 `0011` 追加。
6. 真实迁移测试脚本 `scripts/verify-r01-migrations.sh` 覆盖上述全链、重入与 CHECK 正反例；不内联 DDL 替代真实迁移。Order 全包测试（含真实 0017/0018/0021 迁移）通过。

---

## 1. HEAD 绑定、环境与迁移校验和

### 1.1 环境

| 项 | 值 |
|---|---|
| MySQL | `mysql:8.4` → **8.4.13**，脚本创建一次性容器并 `--rm`，**未连接/未写入共享 `mysql-unicorn`** |
| Runner | `mall/migrate:dev`（Helm 同款镜像）；镜像内 `/run.sh` sha256 = `48e5376d4ab59d3c904c0f7f8785f4859bf8812bdee44c9190d8f7b731226eb2`，与 `deploy/k8s/images/migrate/run.sh` 一致 |
| 挂载 | `-v <repo>/migrations:/migrations:ro`；runner 读取仓库当前迁移文件，而非镜像内旧快照 |
| 隔离 | 独立容器 + 独立端口；脚本中的 `mall_order`/`mall_fulfillment`/`r01_*` 均在该一次性实例内 |
| 运行命令 | `./scripts/verify-r01-migrations.sh` |

### 1.2 迁移文件校验和（真实文件，非内联）

| 文件 | sha256 |
|---|---|
| `order/0017_refund_batch_ledger.sql` | `64bc9f6afeefda4427810e518e4d5b4aa3a37dd063c4f1b7f4641921eba7c1d6` |
| `order/0018_refund_commitment_composition.sql` | `683f2df86eec78cc4b1c9cbb9426285463efa15e7e4f672edef8ac557e342110` |
| `order/0019_legacy_payment_resolution.sql` | `20ed70a1289421edb01fcff59c588738a2225446549418bf44fbcdc11ac148c9` |
| `order/0020_permit_state_check.sql` | `3762b1794d5d4e8fcf5c00a02adcc6480e82b1b7e50e35d90cb1a8b7e55701d8` |
| `order/0021_zero_application_check.sql` | `29c44ade649ddbafa50847ddb9fe26bc9adfd91e7747729c942f9ad5328c1e82` |
| `fulfillment/0010_settlement_shadow_switch.sql` | `c68fe49beb831ceb0e7634cf76251658119b527b130825bb2920503f9e0888f3` |
| `fulfillment/0011_switch_expiry_check.sql` | `6188ba771db492a5504286fcbdbc24362e292d1652c92784ee23c6d5ddaf220d` |

### 1.3 0018 checksum 变更（旧 → 新）

| 版本 | sha256 | 说明 |
|---|---|---|
| 旧（`2a05c1d` 中的 0018） | `d8ae0b9fb12417f621a37d1ece77efd63344bdd0fbac0c9e103711355eb49e0f` | 无条件 `ALTER`，不可重入 |
| 新（本提交） | `683f2df86eec78cc4b1c9cbb9426285463efa15e7e4f672edef8ac557e342110` | 条件化列添加，可重入 |

> 变更许可依据见 §5：在可审计的持久环境（k8s `mall-data`/`mall-uat-data`、preflight DB、本地共享 MySQL）中，0018 均未进入 `schema_migrations`，也无 0018 列。若该结论对某个未审计环境不成立，runner 的 checksum 校验会**直接失败**（fail-closed），不会静默重放或分叉。

---

## 2. Fresh apply：真实 runner + 真实文件

### 2.1 目标迁移的 apply 日志

```text
[migrate] mall_fulfillment <- /migrations/fulfillment/0010_settlement_shadow_switch.sql
[migrate] mall_fulfillment <- /migrations/fulfillment/0011_switch_expiry_check.sql
...
[migrate] mall_order <- /migrations/order/0017_refund_batch_ledger.sql
[migrate] mall_order <- /migrations/order/0018_refund_commitment_composition.sql
[migrate] mall_order <- /migrations/order/0019_legacy_payment_resolution.sql
[migrate] mall_order <- /migrations/order/0020_permit_state_check.sql
[migrate] mall_order <- /migrations/order/0021_zero_application_check.sql
[migrate] complete
```

### 2.2 ledger 校验（runner 记录 = 文件 sha256）

```text
OK   ledger order/0017_refund_batch_ledger.sql = 64bc9f6a...
OK   ledger order/0018_refund_commitment_composition.sql = 683f2df8...
OK   ledger order/0019_legacy_payment_resolution.sql = 20ed70a1...
OK   ledger order/0020_permit_state_check.sql = 3762b179...
OK   ledger fulfillment/0010_settlement_shadow_switch.sql = c68fe49b...
OK   ledger fulfillment/0011_switch_expiry_check.sql = 6188ba77...
```

### 2.3 schema 对象与约束

```text
OK   0017 refund_batches = 1
OK   0017 refund_applications = 1
OK   0018 header columns = 4
OK   0018 commitment lines = 1
OK   0018 releases = 1
OK   0019 legacy_payment_resolution = 1
OK   0019 order_refund_sequences = 1
OK   0020 chk_permit_state = 1
OK   fulfillment/0010 shadow diffs = 1
OK   fulfillment/0010 switch approvals = 1
OK   fulfillment/0011 chk_switch_settled_expiry = 1
OK   0020 rejects GRANTED (rejected: 3819)
OK   0020 accepts NONE = 1
```

### 2.4 runner 重放

```text
[migrate] fulfillment/0010_settlement_shadow_switch.sql already applied
[migrate] fulfillment/0011_switch_expiry_check.sql already applied
[migrate] order/0017_refund_batch_ledger.sql already applied
[migrate] order/0018_refund_commitment_composition.sql already applied
[migrate] order/0019_legacy_payment_resolution.sql already applied
[migrate] order/0020_permit_state_check.sql already applied
[migrate] complete
```

> 说明：runner 对已 apply 文件按 checksum 跳过；本次重放全部 `already applied`，证明 ledger 与文件一致、无 checksum mismatch。

---

## 3. `order/0018` 重入 / 部分失败修复

### 3.1 根因

- 原文件是无条件 `ALTER TABLE ... ADD COLUMN`。MySQL 8.4 **不支持** `ADD COLUMN IF NOT EXISTS`（实测 `ERROR 1064`）。
- 重复执行时 `ERROR 1060 Duplicate column name 'goods_cent'`。
- 部分失败窗口：`ALTER` 是原子 DDL，成功后进程崩溃/超时，`UPDATE` 或两个 `CREATE TABLE` 未执行；此时 runner 把 `schema_migration_attempts` 记为 `FAILED` 且无 `schema_migrations` 行，下一次部署**拒绝自动重放**（设计如此）。原文件即使人工删除 attempt 行也会因重复列再次失败，无法收敛。

### 3.2 修复（只改迁移文件，不改业务代码）

| 机制 | 实现 |
|---|---|
| 列存在性检测 | `information_schema.COLUMNS` 与 4 个期望列做 `NOT EXISTS`，只对缺失列生成 `ADD COLUMN` |
| 单条原子 ALTER | `GROUP_CONCAT` 成一个 `ALTER TABLE ... ADD COLUMN ..., ADD COLUMN ...`，保持 MySQL 原子 DDL 语义 |
| 动态执行 | 会话级 `PREPARE/EXECUTE/DEALLOCATE`；runner 与 `01-schemas.sh` 本来就是单 mysql 会话 |
| 幂等 backfill | `UPDATE ... WHERE composition_source='legacy'` 重复执行结果不变（`goods=amount, freight=0, known=0, source=legacy`） |
| 幂等建表 | 既有 `CREATE TABLE IF NOT EXISTS` 保持不变 |
| 单会话约束 | 文件注释明确要求单会话；Go 测试 helper 改为 `db.Conn` 单连接执行（生产 runner 已满足） |

### 3.3 证据

```text
== 0018 re-entry: full replay is a no-op ==
OK   0018 replay keeps 4 columns = 4

== 0018 partial failure: atomic ALTER committed, then replay converges ==
OK   partial run added header columns = 4
OK   replay created lines table = 1
OK   replay created releases table = 1
OK   legacy backfill = 500|0|0|legacy
OK   second replay keeps backfill = 500|0|0|legacy

== 0018 partial failure: one existing column is not duplicated ==
OK   partial column set converges to 4 = 4

== runner-level partial failure: FAILED attempt refuses replay, operator repair converges ==
OK   runner refuses replay after a FAILED 0018 attempt
OK   refused run records no 0018 ledger row = 0
OK   operator repair records 0018 ledger row = 683f2df8...
OK   operator repair created lines table = 1
OK   operator repair created releases table = 1
```

- 场景 A：完整 schema 上重放真实 0018 —— no-op，4 列不重复。
- 场景 B：用真实文件的前缀（到 `DEALLOCATE`，即 ALTER 已提交）模拟崩溃；再执行完整 0018 —— UPDATE/建表补齐，legacy 行 backfill 正确，再次重放值不变。
- 场景 C：只存在 `goods_cent` 一列的偏态状态 —— 只补其余 3 列，不报 duplicate。
- 场景 D：runner 级 `FAILED` attempt —— 拒绝自动重放并打印 `previous FAILED attempt`；按 runner 文档删除该 attempt 行后，同一真实 0018 收敛并写入新 checksum ledger。

### 3.4 fail-closed 语义（明确保留）

- **不会**自动删除 `schema_migration_attempts`，也**不会**自动把 0018 标记为 applied；人工必须先检查实际 schema（本文件可安全重跑，正是因为检查后重跑会收敛）。
- 若人工不修复而直接重跑，runner 以 `previous FAILED attempt` 退出，属于明确 fail-closed，不会带着半套 schema 继续 0019-0021。
- 0018 只补缺失列，不校验既有列类型；若人工改坏列类型，后续业务查询会失败，需另行人工修复（不在 R0.1 自动修复范围）。

---

## 4. `fulfillment/0011`：`chk_switch_settled_expiry`

### 4.1 契约依据

- ADR-0007 Rev 2.3 §12.5.6：`settled` 行 `expires_at` **MUST NOT NULL**（`chk_switch_settled_expiry`）；`shadow` 行可为 NULL；启动校验只读 `status='APPROVED'` 且未过期的行。
- 同节：存量 `settled` 行若 `expires_at IS NULL`，**必须由 Director 重新签发**，不得自动补默认值；否则请求 `settled` 启动失败。
- SPEC-0002 v1.10 INV-49：未获批准的切换不得生效，过期后不得静默回退 `paid`。

### 4.2 迁移设计

- 不修改已合入/已应用的 `fulfillment/0010`；新增 `0011` 只加约束，无新表/新列/新枚举。
- 先打印阻断行（`approval_id/target_mode/status/expires_at/approved_at`），再用存储过程 `SIGNAL SQLSTATE '45000'`，消息固定为 `d27 precheck: ...`，不自动 UPDATE。
- 约束添加用 `information_schema.TABLE_CONSTRAINTS` 条件化 + `PREPARE`，重复执行安全。
- 阻断运行不写任何行、不加约束；修复审批行后同一文件可再次执行。

### 4.3 证据

```text
OK   0011 chk_switch_settled_expiry = 1
OK   settled NULL expires rejected (rejected: 3819)
OK   settled with expires accepted = 1
OK   shadow NULL expires accepted = 1
OK   0011 replay keeps one CHECK = 1
OK   0011 fail-closed diagnostic: d27 precheck
OK   blocked run adds no CHECK = 0
OK   blocked run keeps the row = 1
OK   after re-sign 0011 adds the CHECK = 1
```

- `settled + NULL expires_at` → `ERROR 3819 Check constraint 'chk_switch_settled_expiry' is violated.`
- `settled + expires_at` 且有 `order_paid_admission_retired_at`/`order_paid_subject_removal_date` → 接受。
- `shadow + NULL expires_at` → 接受。
- 存量违规行：先打印该行，再 `ERROR 1644 (45000) ... d27 precheck`；约束未添加，行保留；更新 `expires_at` 后重跑成功。

> 约束对所有 `target_mode='settled'` 行生效（含历史 `REVOKED/EXPIRED` 行），与 ADR “settled 行 MUST NOT NULL”一致；不按 `status` 放宽，避免历史违规行绕过约束。

---

## 5. 已应用迁移审计（为何可以修改 0018 内容）

**结论：在可审计的持久环境中，`order/0018`、`order/0017/0019/0020`、`fulfillment/0010` 均未被 checksum-tracked runner 应用；0018 的 checksum 变更不会造成已应用环境分叉。**

| 环境 | 证据 | 结果 |
|---|---|---|
| k8s `mall-data`（context `mall-k3s`，MySQL 8.4.13） | `mall_order.schema_migrations` 最新为 `order/0013_group_refund_state.sql`；`mall_fulfillment.schema_migrations` 最新为 `fulfillment/0009_outbox_dead_letter_archive.sql` | 0017-0020、0010 **未应用** |
| k8s `mall-uat-data`（同集群，MySQL 8.4.13） | 同上：order 最新 `0013`，fulfillment 最新 `0009` | 0017-0020、0010 **未应用** |
| k8s preflight DB（`mall_order_preflight`、`mall_order_c_preflight`、`mall_order_c4_preflight`、`mall_fulfillment_c4_preflight` 等） | 对 `order/0017-0020`、`fulfillment/0010` 的 ledger 查询结果为空 | **未应用** |
| k8s 全库列检查 | `information_schema.columns WHERE table_name='order_refund_commitments' AND column_name IN ('goods_cent','composition_known')` 在 `mall-data`/`mall-uat-data` 均为空 | 0018 列**不存在** |
| 本地共享 `mysql-unicorn`（MySQL 26.10.0） | `information_schema.tables WHERE table_name='schema_migrations'` 计数 **0**；`mall_order` 无 `goods_cent` | 无 checksum ledger；0018 **未应用** |
| 本地测试库（`mall_integration_*`、`mall_t1*` 等） | 部分库有 `goods_cent`，但系 `settlement_integration_test.go` 的内联测试 DDL/真实迁移 helper 循环创建，**没有 `schema_migrations` 记录** | 不属于持久迁移应用；无 checksum 约束 |

**审计命令（只读）**：

```bash
kubectl config current-context                       # mall-k3s
kubectl exec -n mall-data mysql-0 -- mysql -uroot -proot -N -B mall_order \
  -e "SELECT filename FROM schema_migrations ORDER BY filename DESC LIMIT 3;"
kubectl exec -n mall-uat-data mysql-0 -- mysql -uroot -proot -N -B mall_fulfillment \
  -e "SELECT filename FROM schema_migrations ORDER BY filename DESC LIMIT 3;"
kubectl exec -n mall-data mysql-0 -- mysql -uroot -proot -N -B \
  -e "SELECT TABLE_SCHEMA,COLUMN_NAME FROM information_schema.columns \
      WHERE TABLE_NAME='order_refund_commitments' AND COLUMN_NAME IN ('goods_cent','composition_known');"
docker exec mysql-unicorn mysql -uroot -proot -N -B \
  -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_name='schema_migrations';"
```

**若审计结论对某个未覆盖环境不成立**：旧 0018 已带旧 checksum 写入该环境 ledger，新文件会触发 runner 的 `changed after it was applied (checksum mismatch)` 硬失败。该失败是 fail-closed，不会静默重放；处置必须是为该环境提供前向修复迁移/人工审计，而不是本次在本仓库强行兼容（已应用迁移不可回写）。

---

## 6. 测试范围

### 6.1 真实迁移测试（不内联 DDL）

`scripts/verify-r01-migrations.sh`：

1. `HEAD`/worktree/镜像与目标文件 sha256 绑定；fresh `mysql:8.4` 容器。
2. 用真实 `mall/migrate:dev` runner + 仓库 `migrations/` 全量 fresh apply；断言目标文件 ledger checksum。
3. 断言 0017 表、0018 4 列/2 表、0019 2 表、0020 CHECK、0010 2 表、0011 CHECK。
4. 0020 CHECK 反例：`GRANTED` 被 `3819` 拒绝；`NONE` 接受。
5. 0018：完整重放；ALTER 后中断重放；单列偏态重放；runner `FAILED` attempt 拒绝 + 人工删除 attempt 后收敛；legacy backfill 值断言。
6. 0011：`settled NULL` 拒绝、`settled + expires` 接受、`shadow NULL` 接受、重放安全；存量违规行 fail-closed（诊断、无约束、行保留）+ Director 重签后约束成功。
7. runner 全量重放：全部 `already applied`。

### 6.2 Go 集成测试（真实迁移 helper）

- `internal/services/order/settlement_integration_test.go` 的 `applyOrderTestMigration` 读取并执行真实 `migrations/order/0017/0018/0021`；本次只改执行连接（`db.Conn` 单会话），不改变断言或业务代码。
- Order 全包：**通过**，`ok gin-kratos-gorm/internal/services/order 237.355s`（真实 MySQL，`TEST_MYSQL_DSN` 指向一次性 8.4 容器；包含 0017/0018/0021 的 CHECK/列/索引与 T1.7 证据测试）。
- 其他静态检查：`bash -n scripts/verify-r01-migrations.sh`、`./scripts/check-migrations.sh`、`gofmt -l`（无输出）。

### 6.3 受影响契约/INV 映射（仅引用，不标 VERIFIED）

| 契约 | 本 R0.1 覆盖点 | 证据 |
|---|---|---|
| INV-45（承诺释放纪律） | 0018 `refund_commitment_releases` 表/唯一键/金额复核 CHECK 真实创建 | §2.3 |
| INV-47（商品/运费组成） | 0018 列/行表/组成 CHECK 真实创建；legacy backfill 形状 | §2、§3.3 |
| INV-49（switch 不具准入权威） | 0011 强制 `settled.expires_at` 非空；违规 fail-closed | §4.3 |
| AC-65（Σ/逐组） | 0017 batch/application schema 真实创建（不重测 T1.7 业务逻辑） | §2.3 |
| AC-66（shadow/switch） | 0010/0011 switch 审批表与约束真实创建 | §2.3、§4.3 |

> AC-REF 不等于 AC-VERIFIED；本文件只提供 R0.1 迁移实现证据，AC 状态由 Reviewer 在 R0.2/R0.3 维护。

---

## 7. 未覆盖范围

1. **未在真实 k8s 集群执行 Helm migrate job**：R0.1 明确要求 fresh DB，避免污染 `mall-data`/`mall-uat-data`；生产/预发演练留给 R0.2/Release 阶段。
2. **未验证大数据量/在线 DDL 的锁与耗时**：fresh 空表 apply 通过；`ALTER TABLE order_refund_commitments` 在真实大表上的 online DDL 行为、元数据锁窗口未测。
3. **未实现/未测试 R0.4 的 config fail-closed 读取**：`SETTLE_RELEASE_APPROVAL_ID`、`settlement_switch_approvals` 启动校验代码不在 R0.1 范围；0011 只保证库层约束。
4. **`order/0021` 零元 CHECK 仅随全链 apply 验证存在**：未在本任务重测 `system_zero` 全部业务语义（T1.0/T1.7/AC-68 既有证据另算）。
5. **其他服务迁移**只做全链 smoke apply；未逐个断言其 schema/CHECK。
6. **无 down migration**：ADR §10 明确只前向、不回滚；本任务未提供回退脚本，回退靠旧代码忽略新结构 + 人工审计修复。
7. **未运行 `-race`**：本任务改动为 DDL/迁移测试，无 Go 并发代码变更；Order 全包集成测试未带 `-race`。重灾区并发证据仍以既有 T1.7/T5.2 为准。
8. **未在 MySQL 8.0/9.x 上验证**：仅 8.4.13（项目目标版本）；`PREPARE`/`SIGNAL`/CHECK 行为在其他版本未测。
9. **0011 存量行策略未覆盖真实业务数据**：只用一条违规行模拟；真实环境若存在多条历史 `settled` 行，需 Director 逐条重签，脚本不代替该裁决。

---

## 8. 反妥协自检清单（Developer 三问）

### 8.1 并发冲突、超时、部分成功时会发生什么？处理了吗？

- **迁移重入**：0018 用列存在性检测 + 原子 ALTER；中断后重跑收敛（§3.3 场景 A/B/C）。
- **runner 部分失败**：`schema_migration_attempts=FAILED` 时 runner 拒绝自动重放（§3.3 场景 D）；人工检查后删除 attempt 行再重跑收敛。没有静默跳过、没有自动改数据。
- **switch 审批行违规**：0011 precheck 先打印再 `SIGNAL`，不自动 UPDATE/不补默认值；阻断运行不改行、不加约束（§4.3）。
- **checksum 分叉**：若某环境已应用旧 0018，runner 直接 checksum mismatch 失败（fail-closed），不静默重放。

### 8.2 是否把状态揉进一个字段 / 加 if-else 绕过架构？

- 未改任何业务状态字段、状态机、事件或 proto；没有为绕过架构新增分支。
- 迁移内的 `CASE`/`NOT EXISTS` 只是 **schema 存在性判断**，不是业务规则分支；唯一业务约束（settled 必须有 expires）来自 ADR，不是实现自创。
- 0011 不按 `status` 放宽约束，避免“历史行绕过”的隐性状态合并。

### 8.3 向现实做了哪些妥协（重大妥协高亮）？

1. **【重大，已给出证据】修改了已合入的 `order/0018` 文件内容（checksum 变化）**：R0.1 明确要求修复重入，而 MySQL 8.4 无 `ADD COLUMN IF NOT EXISTS`，新增迁移无法修复 0018 自身的中断窗口。修改前审计了 k8s 持久环境、preflight、本地共享库与 ledger，均未应用旧 0018；若判断有误，runner checksum 会 fail-closed。**需 Reviewer/Architect 在 R0.2 复核该判断。**
2. **runner `FAILED` attempt 仍需人工删除**：这是 runner 既有 fail-closed 设计，本任务不修改 runner；0018 的可重入性保证“人工检查后重跑能收敛”，但不自动重放。
3. **0011 要求 Director 重签存量违规行**：ADR 禁止自动补默认值，迁移只能阻断；本任务未提供自动修复脚本（正确做法是人工裁决）。
4. **Order 集成测试 helper 改为单连接**：为匹配生产 runner 的单会话语义；这是测试执行模型修正，不改断言/业务逻辑。
5. **未在真实 k8s 执行**：遵守 R0.1 “fresh DB、不污染共享库”的约束；集群演练留待 Release 流程。

---

## 9. [修改点] / [架构侵入性] / [副作用评估]

### 9.1 [修改点]

| 文件 | 修改 |
|---|---|
| `migrations/order/0018_refund_commitment_composition.sql` | 无条件 ALTER → 条件化单条 ALTER（information_schema + PREPARE）；更新 header 到 SPEC v1.10/ADR Rev 2.3；UPDATE/建表语义不变 |
| `migrations/fulfillment/0011_switch_expiry_check.sql` | **新增**：precheck + `chk_switch_settled_expiry` + 条件化添加 |
| `scripts/verify-r01-migrations.sh` | **新增**：fresh 8.4 + 真实 runner + 真实文件的全链/重入/CHECK 验证 |
| `internal/services/order/settlement_integration_test.go` | 仅 `applyOrderTestMigration`：每条语句固定在同一 `*sql.Conn` 执行（0018 的 PREPARE 需要单会话） |
| `docs/implementation/PAY-SETTLE-01-phase0-r-r01.md` | 本文件 |
| `docs/implementation/PAY-SETTLE-01-phase0-r-r01-evidence.txt` | post-commit 证据日志（同一 code head 重跑） |

### 9.2 [架构侵入性]

- **无业务代码/契约/proto 变更**；无新状态、无新表（除 0018 既有契约表）、无新列（0011 只加 CHECK）。
- 0018 只是把同一组列/表改成可重入表达，最终 schema 与 ADR §12.5.2 定义一致。
- 0011 约束与 ADR §12.5.6 逐字对应，未扩大/缩小语义。
- 测试 helper 仅改变语句执行连接，不改变测试数据/断言。

### 9.3 [副作用评估]

- 0018 checksum 变化：只影响“已应用旧 0018 的环境”；审计结论为不存在；若有遗漏则 fail-closed 阻断部署（可见、可修复）。
- 0011 可能在存在历史违规 `settled` 行时**阻断部署**：这是 ADR 要求的 fail-closed；处置是 Director 重签，不是放宽约束。
- 0018 重跑对已存在列/表为 no-op；legacy backfill 仍只命中 `composition_source='legacy'` 行，不覆盖新 writer 的组成。
- 0011 不改变 `shadow` 行行为；`settled` 新写入必须带 `expires_at`，已有应用写入路径若缺该字段会在 DB 层失败（预期，R0.4 另补启动/config 测试）。

---

## 10. 残余风险与后续

1. **R0.2 Reviewer**：需对“修改 0018 checksum”的审计结论、0011 fail-closed 语义、以及本 WIP 的 base/head 出日期化 verdict；不得以本文件替代裁决。
2. **R0.4**：`settlement_switch_approvals` 的启动读取 fail-closed、C-04 release RPC、`group_no` 判别 CHECK、24h SLA CHECK 仍待实现/测试；0011 只提供库层 `expires_at` 约束。
3. **生产演练**：建议在预发用真实 Helm migrate job + 备份恢复演练 0018/0011；本任务未执行。
4. **0011 存量数据**：若预发/生产存在 `settled` 且 `expires_at IS NULL` 的历史行，部署会阻断；需 Director 提前盘点并重签，不能临时补默认值。
5. **0018 列类型漂移**：条件化逻辑只判断列名是否存在，不校验类型；如人工曾以错误类型建列，需要单独审计（本任务未自动修复）。
6. **未纳入 0021 业务语义**：0021 只做全链 apply smoke；零元售后端到端仍按既有 T1.0/T1.7 证据与后续任务。

---

## 11. 证据日志

- 脚本：`scripts/verify-r01-migrations.sh`
- 运行：`./scripts/verify-r01-migrations.sh 2>&1 | tee /tmp/r01-verify.log`
- 提交后完整日志：`docs/implementation/PAY-SETTLE-01-phase0-r-r01-evidence.txt`（头部含 code head `42a8db5`、`WORKTREE clean`、镜像与文件 sha256；本次证据提交只新增该日志并修正 Base/Code head 元数据，不改代码）
- 关键摘要：§2（fresh apply/ledger）、§3.3（0018 重入/部分失败/runner）、§4.3（0011 CHECK/fail-closed）、§6.2（Order 全包测试）。

> 说明：本文件中的日志摘要是真实执行输出的截取；未执行/未覆盖项已在 §7 显式列出，不以“预期通过”代替日志。
