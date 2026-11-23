# 售后迁移 0012 修复方案（分析，未实施）

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01`（C 部分：确定性发布错误） |
| 范围项 | `docs/ext-review-1123-triage.md` §4.4 5.1-b |
| 角色 | Developer（仅分析与演练；**未修改任何迁移文件或脚本**） |
| 日期 | 2026-11-23 |
| 状态 | 方案待批；`git status -- migrations/` 为空，0012 未改 |
| 约束 | 已应用的迁移文件不可改（checksum 账本）；不改 `docs/specs/**`、`docs/adr/**`、`docs/collab/**`；不引入新依赖 |
| 关联 | `migrations/aftersales/0012_line_ledger.sql`、`deploy/k8s/images/migrate/run.sh`、`deploy/compose/init/mysql/01-schemas.sh`、`scripts/rehearse-c-migration.sh`、ADR-0005 §9.4/§13.3 |

> **2026-11-23 人类决策（覆盖 §0 推荐）**：选择**长期拆分链**（§6.1 方案 D），**不采用** §5 的操作员两阶段桥接。  
> **账本审计结果（2026-11-23）**：dev `mall-data` 与 UAT `mall-uat-data` 的 `mall_aftersales.schema_migrations` 均只应用到 `aftersales/0011_group_items.sql`，无 `0012` 记录、无失败尝试；本地 compose `mysql-unicorn` 的 `mall_aftersales` 无 `schema_migrations` 表且 `after_sale_items` 无 `line_no` 列。结论：0012 尚未在任何持久环境应用，可在下一次迁移前替换为 expand → 操作员回填 → unique 的拆分链。  
> **实施前置**：Architect 签字 + 拆分链设计 + 回填工具 + 一次性库演练；本文件的桥接方案保留为备选对比，不执行。

## 0. 结论（TL;DR）

1. **只“新增一个迁移文件”无法修复未应用环境**：迁移按文件名字典序执行，`0012` 一定先于任何新的 `0015+` 文件运行；0012 自身失败，后面的修复迁移永远到不了。
2. `0012` 的失败是**数据相关**的：同一 `after_sale_id` 的历史多明细行被同一句 `ALTER` 全部赋 `line_no=0`，再加 `UNIQUE(after_sale_id, line_no)` 立即 `ERROR 1062`（已在一次性 MySQL 复现）。
3. 由于 `0012:5-9` 在**同一条 DDL** 里创建 `line_no` 并加唯一键，任何“先回填后加唯一键”的修复都必须在 0012 之外先把列建出来，再由操作员回填，最后加唯一键；这超出了单个 SQL 迁移的能力。
4. **推荐方案（立即）**：不修改任何迁移文件的**操作员两阶段桥接**：
   - Pass 1：用过滤后的迁移树先跑完除 `aftersales/0012-0014` 外的全部迁移（让 `mall_order` 的 `line_no` 可用）；
   - Bridge：操作员脚本按 order 行事实回填 `line_no` → 加唯一键 → 复刻 0012 的其余对象 → 用**固定 sha256** 写 `schema_migrations` 账本行（一次性、有审计、有备份）；
   - Pass 2：正常迁移任务跳过 0012（checksum 匹配），继续 `0013/0014`。
5. **长期方案（需 Architect/人类批准，另行评审）**：审计确认 0012 从未在任何持久环境应用后，用“expand → 操作员回填 → unique”的**拆分链替换** 0012，消除桥接和账本核对；本文件不实施。
6. 已应用 0012 的环境**没有唯一键冲突**，但仍需按 ADR-0005 §9.4 做历史 `line_no` 回填（`line_no=0` 人工队列）。未应用且无多明细历史的环境无需任何处理。

## 1. 事实与证据链

### 1.1 0012 的写法

`migrations/aftersales/0012_line_ledger.sql:4-10`：

```sql
ALTER TABLE after_sale_items
  ADD COLUMN line_no INT NOT NULL DEFAULT 0,
  ADD COLUMN line_paid_cent_snapshot BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN unit_paid_cent_snapshot BIGINT NOT NULL DEFAULT 0,
  DROP INDEX uk_after_sale_items_group,
  ADD UNIQUE KEY uk_after_sale_items_line (after_sale_id, line_no),
  ADD KEY idx_after_sale_items_group (after_sale_id, sku_id, warehouse_id, group_no);
```

历史行在 `ADD COLUMN` 时统一得到 `line_no=0`；同一句 `ALTER` 的后半段立刻要求 `(after_sale_id, line_no)` 唯一。任一 `after_sale_id` 有 ≥2 条历史明细即冲突。0012 自身注释（`:12-14`）也承认历史行归属必须由**独立操作员回填**完成，迁移不能猜。

### 1.2 checksum 账本：已应用文件不可改

- K8s 迁移器：`deploy/k8s/images/migrate/run.sh:103-110` 建 `schema_migrations` 账本；`:129-136` 在文件已应用时比对 sha256，不一致直接 `exit 1`。
- Compose 初始化器：`deploy/compose/init/mysql/01-schemas.sh` 使用同一账本与同一 `key = <service>/<filename>`，`sort -V` 顺序执行；`docs/ops.md:138-140` 明确“Do not edit an already-applied migration”。
- 结论：一旦某环境记录了 `aftersales/0012_line_ledger.sql`，修改文件即 checksum mismatch，迁移任务拒绝继续。

### 1.3 attempt 账本：失败后不会自动重放

`run.sh:146-151`：若 `schema_migration_attempts` 存在该文件的 `FAILED`/`RUNNING` 记录而账本无对应行，直接 `refusing to replay`。所以 0012 失败后必须由操作员显式修复/对账，不能靠“再跑一次 Helm”。

### 1.4 迁移顺序：aftersales 先于 order

`run.sh:98-101` 遍历 `"${MIGRATIONS_ROOT}"/*/`（shell 字典序），`aftersales` 在 `order` 之前。0012 失败会**中断整个任务**，`mall_order` 的 `order/0011_fulfillment_groups.sql`（新增 `order_items.line_no`）与 `order/0014_quantity_split.sql`（新增 `line_paid_cent/unit_paid_cent`）都不会执行。这决定了桥接必须先跑一轮“无 0012”的迁移，才能拿到回填所需的订单行事实。

### 1.5 命名守卫：无法插入“0012 之前”的新文件

`scripts/check-migrations.sh:3-4,16-32`：
- 文件必须匹配 `[0-9][0-9][0-9][0-9]_*.sql`；
- 同一服务内前缀 `>= 0008` 不得重复。

因此 `0011a_*`、`0012_pre_*` 之类的“0012 前置”文件都会被守卫拒绝；唯一能排在 0012 前的办法是替换/改名 0012 本身，而这与“已应用不可改”冲突。

### 1.6 ADR 口径

ADR-0005 §9.4 第 828 行：

> 回填：单组历史售后按 `(after_sale_no, sku, warehouse, group)` 归并到 `line_no`；无法唯一归属的历史数据标 `line_no=0` 并进入人工核对队列，不阻塞新单。

推论：`line_no=0` 是**允许的哨兵值**，但 0012 的 `UNIQUE(after_sale_id, line_no)` 意味着**每个售后最多只能有 1 行未归属明细**。若同一售后有 2 条以上无法唯一归属的明细，必须在加唯一键前人工裁决，桥接必须 fail-closed，不能伪造行号。该“哨兵与唯一键”的组合口径建议由 Architect 在后续 Amendment 中确认（见 §7）。

ADR-0005 §13.3 第 985 行：**不写 down SQL**，生产按 ADR 0003 备份恢复。

## 2. 失败复现（一次性 MySQL，未改仓库）

环境：本机 `mysql:latest` 容器（`127.0.0.1:3306`），一次性库 `mall_scratch_0012`。

构造 0011 之后的 `after_sale_items` 形状，插入历史多明细数据（售后 1001 有 2 条，1002 有 1 条），然后直接执行 0012：

```text
after_sale_id   items
1001            2
1002            1

=== run 0012 against historical multi-item data ===
ERROR 1062 (23000) at line 4: Duplicate entry '1001-0' for key 'after_sale_items.uk_after_sale_items_line'
exit=1
```

MySQL 8.0 的 `ALTER TABLE` 是原子 DDL；失败后表保持原状：

```text
line_no_cols=0  rows_intact=3  new_tables=1
```

即失败环境停在 pre-0012 schema，0012 的三张新表也没有创建（第一条语句就失败）。这与 `run.sh` 的 `FAILED` attempt 语义一致：环境被卡在“0011 已应用、0012 未应用、attempt=FAILED”。

单明细历史（每售后 1 行）则 0012 成功，行保留 `line_no=0`、唯一键建立：

```text
after_sale_id  line_no
2001           0
2002           0
uk_after_sale_items_line 索引存在
```

这解释了“为什么有的环境 0012 是已应用状态”：只有无多明细历史的环境才能通过。

## 3. 约束汇总

| # | 约束 | 证据 | 对方案的含义 |
|---|---|---|---|
| C1 | 已应用迁移文件不可改 | `run.sh:103-136`、`docs/ops.md:138-140` | 不能改 0012；方案必须对“已应用/未应用”都安全 |
| C2 | attempt 失败不自恢复 | `run.sh:146-151` | 修复必须显式处理 `FAILED` 记录 |
| C3 | 迁移按文件名顺序执行 | `run.sh:126`、`01-schemas.sh` `sort -V` | 新增 `0015+` 无法先于 0012 |
| C4 | 命名守卫禁止前置/重复前缀 | `check-migrations.sh:16-32` | 不能插 `0011a`/`0012_pre` |
| C5 | aftersales 服务 schema 不得读 order | `0012:12-14`、ADR §9.4 | 回填只能操作员跨库只读，不能写成服务迁移 |
| C6 | 0013/0014 依赖 0012 的列/表 | `0013_responsibility.sql`、`0014_line_pending_amount.sql` | 不能简单“跳过 0012 继续跑 0013” |
| C7 | MySQL DDL 原子但非事务 | 实测 | 桥接必须分步、可重入、失败可重跑，不能假设回滚 |
| C8 | 无 down SQL | ADR §13.3:985 | 回滚=备份恢复 + 镜像回退 |
| C9 | ADR 允许 `line_no=0` 哨兵 | ADR §9.4:828 | 未归属行可以保留 0，但每个售后至多 1 行 |
| C10 | 生产默认关闭开关 | ADR §13.1 | 本项与功能开关无关；迁移必须先于功能启用 |

## 4. 环境分类与影响矩阵

| 环境 | 0012 账本 | 历史多明细 | 0012 结果 | 当前状态 | 需要的动作 |
|---|---|---|---|---|---|
| E1 已应用 | 有，checksum 匹配 | 不可能（否则 0012 失败） | 成功 | 所有历史行 `line_no=0` | 仅按 ADR §9.4 做历史回填（可延后，不阻塞新单）；无 schema 修复 |
| E2 未应用、无冲突 | 无 | 否 | 可正常成功 | 与 E1 同 | 让正常迁移任务跑完 0012；之后可选回填 |
| E3a 未应用、升级环境有多明细 | 无，可能 attempt=FAILED | 是 | `ERROR 1062` | 卡在 pre-0012；order 可能已是 C 后 schema | 两阶段桥接（§5） |
| E3b 未应用、恢复自 pre-C dump 且多明细 | 无 | 是 | `ERROR 1062` | 卡在 pre-0012；`mall_order` 尚未有 `line_no` | 两阶段桥接的 Pass 1 必须先跑 order 迁移 |

**对账本的影响**：

- E1/E2：无变化。
- E3：桥接会在 `schema_migrations` 写入 `aftersales/0012_line_ledger.sql` + 固定 sha256（见 §5.3），并把 `schema_migration_attempts` 的 `FAILED` 记录改写为 `APPLIED` 并附操作员说明；这是**一次性、显式、可审计**的例外，必须绑定发布单号与备份。
- 任何“删账本行强制重放 0012”的做法都会再次撞上同一 `ERROR 1062`，并且丢失审计；明确禁止。

## 5. 推荐方案：操作员两阶段桥接（不改迁移文件）

### 5.1 设计要点

- 0012 文件字节不变（sha256 固定为 `25d2c5e4cc7098daa73e9be50ea1ef9ecf126130d081df9acce4076a25672692`）。
- 桥接只在**受影响的 E3 环境**执行；E1/E2 不执行。
- 所有步骤可重入：每步先查 `information_schema`，已存在则跳过；唯一键最后加。
- 回填只读 `mall_order`，任何 0 匹配/多匹配都 fail-closed 进人工队列；同一售后若仍有 >1 条 `line_no=0`，桥接在加唯一键前中止。
- 桥接必须复刻 0012 的**全部**对象（3 列、索引替换、3 张新表），否则账本记录“已应用”会造成 schema 漂移。

### 5.2 Pass 1：先让 order 迁移跑完

在 E3b（或不确定 order schema 是否就绪）时执行；E3a 若 `mall_order.order_items.line_no` 已存在可跳过。

用同一 `mall/migrate` 镜像起一次性 Job，挂载可写 `emptyDir`，复制迁移树并去掉 `aftersales/0012-0014` 三个文件，再以 `MIGRATIONS_ROOT` 指向副本运行（示意，不是仓库资产）：

```yaml
command: ["/bin/sh", "-c"]
args:
  - |
    set -eu
    cp -a /migrations /work/migrations
    rm -f /work/migrations/aftersales/0012_line_ledger.sql \
          /work/migrations/aftersales/0013_responsibility.sql \
          /work/migrations/aftersales/0014_line_pending_amount.sql
    MIGRATIONS_ROOT=/work/migrations /run.sh
```

结果：`aftersales/0001-0011` 已应用，`order/0011`/`order/0014` 已应用（`order_items.line_no`、`line_paid_cent`、`unit_paid_cent` 可用），其余服务不受影响。0012/0013/0014 仍待处理。

> 注意：不能只去掉 0012 而保留 0013/0014——它们依赖 0012 创建的列/表，会再次失败（C6）。

### 5.3 Bridge：回填 + 唯一键 + 复刻对象 + 账本核对

以下为 SQL 骨架；**脚本本身尚未实现**（本次只做分析与一次性库演练）。执行用户需要对 `mall_aftersales` 有 DDL 权限、对 `mall_order` 只读权限。

```sql
-- 0) 前置断言（任一不满足即中止）
--    0011 已应用；0012 未有账本行；after_sale_items 存在；
--    不存在半成品 line_no 唯一键。
SELECT COUNT(*) FROM mall_aftersales.schema_migrations
 WHERE filename='aftersales/0012_line_ledger.sql';            -- 必须 = 0
SELECT COUNT(*) FROM information_schema.statistics
 WHERE table_schema='mall_aftersales' AND table_name='after_sale_items'
   AND index_name='uk_after_sale_items_line';                  -- 必须 = 0

-- 1) 加列（与 0012:5-7 完全一致），先不加唯一键
ALTER TABLE mall_aftersales.after_sale_items
  ADD COLUMN line_no INT NOT NULL DEFAULT 0,
  ADD COLUMN line_paid_cent_snapshot BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN unit_paid_cent_snapshot BIGINT NOT NULL DEFAULT 0;

-- 2) 生成审计映射（放在 mall_migrate 运维库，不污染服务 schema；人工队列关闭后删除）
CREATE TABLE IF NOT EXISTS mall_migrate.after_sale_line_backfill (
  after_sale_id BIGINT NOT NULL, sku_id BIGINT NOT NULL,
  warehouse_id BIGINT NOT NULL, group_no VARCHAR(64) NOT NULL,
  line_no INT NOT NULL, source_order_no VARCHAR(64) NOT NULL,
  source_order_item_id BIGINT NOT NULL,
  resolved_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (after_sale_id, sku_id, warehouse_id, group_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 3) 只把“唯一匹配”的订单行写入映射；0/多匹配不写（人工队列）
INSERT INTO mall_migrate.after_sale_line_backfill
  (after_sale_id, sku_id, warehouse_id, group_no, line_no, source_order_no, source_order_item_id)
SELECT i.after_sale_id, i.sku_id, i.warehouse_id, i.group_no,
       MIN(oi.line_no), s.order_no, MIN(oi.id)
  FROM mall_aftersales.after_sale_items i
  JOIN mall_aftersales.after_sales s ON s.id = i.after_sale_id
  JOIN mall_order.order_items oi
    ON oi.order_no = s.order_no
   AND oi.sku_id = i.sku_id
   AND oi.warehouse_id = i.warehouse_id
   AND oi.fulfillment_group_no = i.group_no
 GROUP BY i.after_sale_id, i.sku_id, i.warehouse_id, i.group_no
HAVING COUNT(*) = 1;   -- 多匹配一律不进映射，交人工

-- 4) 回填行号与金额快照（快照取 order 行事实，不重算）
UPDATE mall_aftersales.after_sale_items i
  JOIN mall_migrate.after_sale_line_backfill b
    ON b.after_sale_id=i.after_sale_id AND b.sku_id=i.sku_id
   AND b.warehouse_id=i.warehouse_id AND b.group_no=i.group_no
  JOIN mall_order.order_items oi ON oi.id = b.source_order_item_id
   SET i.line_no = b.line_no,
       i.line_paid_cent_snapshot = oi.line_paid_cent,
       i.unit_paid_cent_snapshot = oi.unit_paid_cent;

-- 5) fail-closed 检查（必须在回填之后）：同一售后最多 1 条未映射行
--    （>1 行未映射时，唯一键不可能建立，必须人工裁决后再继续）
SELECT after_sale_id, COUNT(*) FROM mall_aftersales.after_sale_items
 WHERE line_no = 0 GROUP BY after_sale_id HAVING COUNT(*) > 1;   -- 必须为空

-- 6) 索引替换（与 0012:8-10 一致）
ALTER TABLE mall_aftersales.after_sale_items
  DROP INDEX uk_after_sale_items_group,
  ADD UNIQUE KEY uk_after_sale_items_line (after_sale_id, line_no),
  ADD KEY idx_after_sale_items_group (after_sale_id, sku_id, warehouse_id, group_no);

-- 7) 复刻 0012 的其余对象（直接执行 0012 的 CREATE TABLE 段）
--    after_sale_line_ledger / after_sale_refund_breakdowns / restock_exception_authorizations
```

```sql
-- 8) 账本核对：写入 0012 的固定 checksum（一次性）
START TRANSACTION;
INSERT INTO mall_aftersales.schema_migrations (filename, checksum)
VALUES ('aftersales/0012_line_ledger.sql',
        '25d2c5e4cc7098daa73e9be50ea1ef9ecf126130d081df9acce4076a25672692');
-- attempt 行可能不存在（E3b 从未失败过），用 upsert 保证审计落库
INSERT INTO mall_aftersales.schema_migration_attempts
  (filename, checksum, status, owner, error, started_at, updated_at)
VALUES ('aftersales/0012_line_ledger.sql',
        '25d2c5e4cc7098daa73e9be50ea1ef9ecf126130d081df9acce4076a25672692',
        'APPLIED', 'operator-bridge', 'operator bridge <ticket>; mapped rows=<n>, unmapped=<m>', NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
  status='APPLIED',
  error='operator bridge <ticket>; mapped rows=<n>, unmapped=<m>',
  updated_at=NOW(3);
COMMIT;
```

> 上述 SQL 是“首次执行”形态。脚本实现时必须用 `information_schema` 守卫把每一步包成可重入：列/表/索引已存在则跳过，索引替换拆成“有旧索引才 DROP、无唯一键才 ADD”，避免中途失败后重跑报 `Duplicate column/key`（参考 `migrations/aftersales/0002_return_flow.sql` 的动态 SQL 写法）。

桥接后置断言（全部通过才允许 Pass 2）：

```text
uk_after_sale_items_line 存在；uk_after_sale_items_group 不存在；
idx_after_sale_items_group 存在；3 张 0012 表存在；
SELECT COUNT(*) FROM information_schema.columns ... line_no/快照列 = 3；
SELECT COUNT(*) FROM information_schema.tables ... 0012 表 = 3；
schema_migrations 行的 checksum = 25d2c5e4…；
同一售后 line_no=0 行数 <= 1。
```

### 5.4 Pass 2：正常迁移任务

用**发布版未经修改的** migrate 镜像再跑一次：

```text
[migrate] aftersales/... 0001..0011 already applied
[migrate] aftersales/0012_line_ledger.sql already applied
[migrate] mall_aftersales <- .../0013_responsibility.sql
[migrate] mall_aftersales <- .../0014_line_pending_amount.sql
...
[migrate] complete
```

预期：0012 因账本 checksum 匹配被跳过（`run.sh:142`），`0013/0014` 正常应用，其余服务不受影响。若日志出现 checksum mismatch 或 `refusing to replay`，立即停止并按 §6 回滚。

### 5.5 E1/E2 的历史回填（独立、可延后）

E1/E2 不需要唯一键修复；若业务要把 `line_no=0` 降到最少，可复用 §5.3 的映射 SQL（只做第 2–5 步），更新 `line_no` 0→真实行号。因为 E1 每个售后至多 1 条 `line_no=0` 行，更新不会触发唯一键冲突；快照同样从 `order_items` 取。ADR 允许未归属行留在人工队列，不阻塞新单。

## 6. 可选方案对比

| 方案 | 是否改 0012 | 能否救 E3 | 对 E1 影响 | 风险 | 结论 |
|---|---|---|---|---|---|
| A. 只新增迁移（`0015+`） | 否 | **不能**：0012 先失败（C3） | 无 | 方案无效 | 否决 |
| B. 迁移内“先回填后唯一键” | 否（需要拆分 0012 才能先建列） | 不能：单文件无法分步；跨 schema 读 order 违反 C5 | 需重跑 | 迁移不可重入 | 否决 |
| C. **操作员两阶段桥接 + 账本核对** | 否 | **能** | 执行时 no-op（前置断言跳过） | 账本例外、DDL 复刻漂移、人工映射 | **推荐（立即）** |
| D. 拆分链替换（expand → 回填 → unique） | 是（删除/改名已应用的 0012） | 能 | 需要幂等守卫 + 孤儿账本行 | 改动迁移历史；需 Architect/人类批准 | 长期备选 |
| E. 删除账本行/强改唯一约束绕过 | 是/否 | 表面能，实际丢审计、重复冲突 | 破坏审计 | 高风险 | 禁止 |
| F. 唯一键改成可空/多个 `line_no=0` | 需改 schema 契约 | 能 | 改变 `line_no` 语义 | 契约/代码变更，需 Architect+PO | 不推荐 |

### 6.1 方案 D（长期）细节

若审计证明 0012 **从未在任何持久环境应用**（查询见 §7.1），最干净的长期修法是：

1. 用幂等守卫（`information_schema` 判断，参考 `0002_return_flow.sql` 的动态 SQL 写法）重写为两个文件：
   - `0012_line_ledger_expand.sql`：加列 + 创建 0012 的 3 张表 + 索引替换中的**非唯一**索引，不加唯一键；
   - `0016_line_ledger_unique.sql`：先断言无重复、再 `ADD UNIQUE KEY uk_after_sale_items_line`。
2. 删除旧 `0012_line_ledger.sql`（或改名），避免它继续先执行；已应用环境的旧账本行成为无害孤儿行，新文件必须对已存在列/索引 no-op。
3. 仍需要两阶段发布（Pass 1 到 expand/order，操作员回填，Pass 2 跑 unique），只是不再需要账本核对。
4. 代价：迁移历史被替换，需 Architect/人类批准并在 release note 说明；在 0012 可能已应用的集群上**不能**采用。

## 7. 待裁决与风险

### 7.1 必须先在每个环境执行的审计

```sql
-- 已应用？checksum 是否匹配当前文件？
SELECT filename, checksum, applied_at
  FROM mall_aftersales.schema_migrations
 WHERE filename='aftersales/0012_line_ledger.sql';
-- 失败尝试？
SELECT filename, status, `error`, started_at, updated_at
  FROM mall_aftersales.schema_migration_attempts
 WHERE filename='aftersales/0012_line_ledger.sql';
-- 是否已有多明细历史？（决定是否 E3）
SELECT after_sale_id, COUNT(*) AS items
  FROM mall_aftersales.after_sale_items
 GROUP BY after_sale_id HAVING COUNT(*) > 1;
-- order 行事实是否可用？（决定是否需要 Pass 1）
SELECT COUNT(*) FROM information_schema.columns
 WHERE table_schema='mall_order' AND table_name='order_items'
   AND column_name IN ('line_no','line_paid_cent','unit_paid_cent');
```

### 7.2 需要 Architect 裁决的口径

- ADR-0005 §9.4 的 `line_no=0` 哨兵与 0012 的 `UNIQUE(after_sale_id, line_no)` 组合，隐含“每个售后最多 1 条未归属明细”。若同一售后有 2 条以上无法唯一映射的历史明细，当前契约没有合法表达；桥接只能 fail-closed 等人工裁决。建议后续 Amendment 明确：未归属行是否允许合并到售后级（`line_no=0` 单行）或以独立表承载。
- 桥接写 `schema_migrations` 属于一次性运营例外，是否需要 Architect/运维共同批准与固定 expiry（建议 30 天内删除桥接脚本）。
- 方案 D（替换迁移链）需要确认 0012 的全环境应用状态，并评估 release note 与回滚语义。

### 7.3 残余风险

| 风险 | 缓解 |
|---|---|
| 桥接 DDL 与 0012 漂移 | 固定 sha256；在一次性库用 0012 对空表建参考库，与桥接结果做 `information_schema` diff；CI 增加 schema 对比 |
| 人工映射错误 | 映射表（`mall_migrate.after_sale_line_backfill`）保留审计，人工队列关闭后删除；回填前后对账（行数、金额合计、`line_paid_cent` 与 order 一致） |
| 桥接中途失败 | 所有步骤先查 `information_schema`，可重入；失败不进入 Pass 2；备份可恢复 |
| 账本例外被滥用 | 只允许发布单绑定的 E3 环境；`error` 列写 ticket；发布后删除桥接脚本 |
| 迁移锁/并发 | 复用 runner 的 `mall_migrate` lease；桥接期间禁止并行迁移任务 |

## 8. 回滚与演练

### 8.1 回滚

- 迁移无 down SQL（ADR §13.3）。回滚 = 恢复 **Pass 1 完成后、Bridge 开始前** 的 `mall_aftersales` 备份（含 `schema_migrations`/`schema_migration_attempts`）+ 回退应用镜像；`mall_order` 的 order 迁移只增不删，可保留。若没有该时间点备份，则回退到 pre-C dump 并同时回退预发/生产数据版本。
- 若已进入 Pass 2（0013/0014 已应用），回滚同样恢复到 Pass 1 后快照；不得只删 `line_no` 列（会破坏已写入的售后行）。
- 恢复后用 §7.1 的审计 SQL 确认回到 pre-0012 状态，再重新评估。

### 8.2 演练（一次性 MySQL，复用 `scripts/rehearse-c-migration.sh` 的模式）

建议新增（另行评审，不在本次交付）`scripts/rehearse-aftersales-0012.sh`，步骤：

1. 起两个一次性 MySQL（`BASE_PORT`/`BASE_PORT+1`），恢复同一份 pre-C dump（含 `mall_order`、`mall_aftersales` 历史多明细数据）。
2. 实例 1：Pass 1（过滤 0012-0014）→ Bridge → Pass 2；断言：
   - 0012 日志为 `already applied`；0013/0014 已应用；
   - `uk_after_sale_items_line` 存在、旧 `uk_after_sale_items_group` 不存在；
   - 3 张 0012 表存在；映射行数 > 0；未映射行每售后 ≤ 1；
   - `schema_migrations` checksum = 固定值；
   - 金额快照与 `order_items` 对账一致。
3. 故障注入：在 Bridge 的“加列后、唯一键前”杀掉进程，重跑 Bridge，断言可重入并最终通过；再对“同一售后 2 条未映射行”的副本断言 Bridge 在唯一键前中止。
4. 实例 2：恢复同一 dump 模拟回滚，断言 pre-0012 schema（无 `line_no`、旧索引存在、3 张表不存在、无 0012 账本行）。
5. 仅在此演练通过后，才允许对 E3 生产环境执行；生产必须另行备份并冻结 aftersales 写入（缩容到 0）。

本次已完成的一次性库演练（§2、§5.3 SQL 骨架的核心步骤）：

```text
-- 失败复现
ERROR 1062 (23000) at line 4: Duplicate entry '1001-0' for key 'after_sale_items.uk_after_sale_items_line'

-- 手工桥接序列（加列 → 映射 1001 两行 line_no=1/2 → 索引替换 → 复刻 3 表）
after_sale_id  sku_id  line_no
1001           501     1
1001           502     2
1002           503     0
索引: PRIMARY(id), uk_after_sale_items_line(after_sale_id,line_no),
      idx_after_sale_items_group(after_sale_id,sku_id,warehouse_id,group_no)
表: after_sale_items, after_sale_line_ledger, after_sale_refund_breakdowns,
     restock_exception_authorizations, schema_migrations,
     after_sale_items_line_backfill（演练库中的映射表；正式脚本放 mall_migrate）

-- 残余 fail-closed：同一售后再插一条未映射行
ERROR 1062 (23000) at line 1: Duplicate entry '1002-0' for key 'after_sale_items.uk_after_sale_items_line'
```

## 9. 未实施与后续工作

本文件**只做分析与演练**，以下均未提交：

- [ ] `scripts/bridge-aftersales-0012.sh`（桥接脚本，含前置断言/映射/fail-closed/账本核对）。
- [ ] `scripts/rehearse-aftersales-0012.sh`（一次性库演练与故障注入）。
- [ ] Pass 1 的过滤迁移树 Job 模板（k8s 运营资产）。
- [ ] ADR-0005 Amendment（§9.4 哨兵与唯一键口径）——归 Architect。
- [ ] 任何 `migrations/**` 改动——本项明确禁止。

## 附录 A：关键文件与行号

| 事实 | 位置 |
|---|---|
| 0012 同句加列 + 唯一键 | `migrations/aftersales/0012_line_ledger.sql:4-10` |
| 0012 声明回填归操作员 | `migrations/aftersales/0012_line_ledger.sql:12-14` |
| checksum 账本与不可改 | `deploy/k8s/images/migrate/run.sh:103-136` |
| attempt 拒绝重放 | `deploy/k8s/images/migrate/run.sh:146-151` |
| 迁移目录字典序 | `deploy/k8s/images/migrate/run.sh:98-101` |
| compose 同账本 | `deploy/compose/init/mysql/01-schemas.sh`（`sort -V`） |
| 命名守卫 | `scripts/check-migrations.sh:3-4,16-32` |
| 回填口径 / 哨兵 | `docs/adr/0005-fulfillment-quantity-split.md:828` |
| 无 down SQL | `docs/adr/0005-fulfillment-quantity-split.md:985` |
| 运维账本说明 | `docs/ops.md:122-140` |
| 现有 C 迁移演练模式 | `scripts/rehearse-c-migration.sh` |

## 附录 B：0012 固定 checksum

```text
sha256(migrations/aftersales/0012_line_ledger.sql)
= 25d2c5e4cc7098daa73e9be50ea1ef9ecf126130d081df9acce4076a25672692
```

该值必须同时出现在：桥接脚本常量、演练断言、发布单；任一不匹配即中止。
