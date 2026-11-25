# PAY-SETTLE-01 R0.4b-impl / R0.4c-impl / R0.4d1 / R0.4d2 Developer 自检清单

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Role / Actor | Developer / `dev-r04-impl` |
| Tier | **L2**（启动门禁 + 迁移 + deploy + 脚本；跨服务事件/库存/资金边界） |
| 契约输入 | `docs/adr/0007-amd1-addendum-r0.4-c04-release.md` DRAFT 0.3 r3（commit `d5dda12`），人类 2026-11-26 接受（R0.4a3，`c432ed6`）；QC round 3 READY（`bd7c239`，0 BLOCKER / 0 MAJOR / 9 MINOR） |
| Base | `4215b3c`（`c38c54f^`；`c432ed6` 之后仅并发 docs 提交 `4215b3c`，未触碰代码字节） |
| Code head（本文测试/迁移证据绑定） | **`c38c54f899e81a3b33b55f581aaa04a869d74b23`** |
| 本次改动 | 14 files，+873/−213（见 §1） |
| allowed_paths（实际改动） | `cmd/fulfillment/**`、`internal/services/fulfillment/settlement_switch*`、`internal/platform/config/**`、`migrations/order/0022_r04_checks.sql`（分支 B）、`internal/services/order/r04_checks_integration_test.go`（仅迁移测试）、`scripts/verify-r01-migrations.sh`、`deploy/compose/**`、`deploy/k8s/charts/mall/**`、`docs/ops.md`、本文档 |
| forbidden_paths（未触碰） | `internal/services/order/**` 生产代码、`internal/services/inventory/**`、`api/**`、SPEC/ADR/`docs/lead/**`、`docs/collab/**`、台账（`ac-coverage.tsv` / evidence-matrix / tasks / review-tracking）；`internal/services/order/r04_checks_integration_test.go` 是迁移测试（Addendum §2.3.1 明确要求改 48h 断言），不含 AC-27/AC-50 语义 |
| Status | **WIP**；不声称实现 PASS / R0.4e PASS / Release PASS；**不标 AC-VERIFIED**（归 Reviewer） |

---

## 1. Diff 摘要（`c38c54f`）

```
cmd/fulfillment/main.go                                     |  28 ++-
cmd/fulfillment/main_test.go                                | 252 +++++++++++----
deploy/compose/.env.example                                 |   9 +
deploy/compose/services.yml                                 |   8 +
deploy/k8s/charts/mall/templates/deployments.yaml           |  13 ++
deploy/k8s/charts/mall/values.yaml                          |  14 ++
docs/ops.md                                                 | 106 +++++++++
internal/platform/config/config.go                          |  23 +-
internal/platform/config/settlement_switch_test.go          |  24 +-
internal/services/fulfillment/settlement_switch.go          | 210 +++++++------
internal/services/fulfillment/settlement_switch_integration_test.go | 95 ++++---
internal/services/order/r04_checks_integration_test.go      | 100 ++++++--
migrations/order/0022_r04_checks.sql                        |  74 ++++--
scripts/verify-r01-migrations.sh                            | 130 ++++++++++-
14 files changed, 873 insertions(+), 213 deletions(-)
```

- **R0.4b**：`settlement_switch.go` 把失败原因改为稳定 token（`APPROVAL_ROW_MISSING` / `APPROVAL_NOT_APPROVED` / `APPROVAL_EXPIRED` / `RELEASE_RANGE_MISMATCH` / `APPROVAL_DB_UNAVAILABLE` / `N5_APPROVAL_ID_MODE_MISMATCH` / `RELEASE_RANGE_MISSING` / `MODE_UNKNOWN`），`main.go` 门禁失败 **exit 3**、shadow 运行态拒绝 **exit 4**，并输出 `mode`/`approval_id`/`reason` 三字段（`error` 串同时含 `mode=... approval_id=... reason=...`）。config 拒绝保持 panic（exit 2），错误文本带稳定 token。
- **R0.4c**：`0022` 就地修正（分支 B）：判别式不再复制 0017 枚举；24h 改为等值；删除“无条件 re-run safe”表述，写入失败重跑口径。
- **R0.4d1**：compose/Helm 注入 mode（默认 `paid`）与可选 approval/range；`docs/ops.md` 新增批准行模板、`release_range <> ''` 0 行校验、到期告警与 0022 恢复流程。
- **R0.4d2**：验证脚本增 `order/0021` + `order/0022`、`CHECK_CLAUSE` 文本断言、24h/48h/23h 边界、dirty/partial/replay 场景。

---

## 2. R0.4c-impl：preflight 先于任何 0022 编辑（Addendum §2.3.3）

### 2.1 执行信息

- 时间：**2026-11-26T03:07:00+0800**（`2026-11-25T19:07:00Z`）
- HEAD：`c432ed613b24461756fdfc75ac6da461684e4afd`
- 执行人：`dev-r04-impl`
- 编辑前 `0022` sha256：**`8339df0741584ef67c76e7daf417c2d6dee57ed26a7d1cf95ba0ef88cf574626`**
- 编辑后 sha256：**`85a2fc8269e093bccc86a44bb6a457c0112bdd7da2fedc800e2c0f4b9dbbfe55`**（绑定 `c38c54f`；验证脚本日志的 ledger 断言即此值）

### 2.2 环境清单与只读查询结果

按 Addendum §2.3.3 的目标环境（所有可达持久环境；一次性容器不算）：

| # | 环境 | 版本 | `schema_migrations` 0022 行 | attempt 记录 | 事实 |
|---|---|---|---|---|---|
| 1 | k3s `mall-data` / `mysql-0`（dev 持久） | 8.4.13 | **0** | **0** | ledger 最大只到 `order/0013_group_refund_state.sql`；`order/0017` 起 0 行；0022 两个 CHECK 均不存在 |
| 2 | k3s `mall-uat-data` / `mysql-0`（UAT 持久） | 8.4.13 | **0** | **0** | 同上（ledger 最大 `order/0013`；0017+ 0 行；0022 CHECK 不存在） |
| 3 | 本机 `mysql-unicorn`（`docker_mysql-db` 持久卷，集成测试/scratch） | 26.10.0 | 无 `schema_migrations` 表 | 无 attempt 表 | `mall_order` 无 `refund_applications`/`legacy_payment_resolution`；全库无 `chk_refund_app_group_discriminator`/`chk_lpr_expiry_24h`；0022 不可能成功 apply |
| 4 | compose dev（`deploy/compose`） | — | 无 MySQL 容器、无 `mall_mysql-data` 卷 | — | 从未初始化；不构成“已 apply” |
| — | staging / prod | — | — | — | 本 kube context 无 staging/prod namespace（仅 `mall`、`mall-data`、`mall-uat`、`mall-uat-data`）与仓库内无对应 values 目录 |

只读 SQL（原样执行，attempt 表按 QC S-3 先判 `information_schema.tables` 是否存在，不存在记为“无 attempt 记录”）：

```sql
SELECT filename, checksum, applied_at FROM schema_migrations
 WHERE filename = 'order/0022_r04_checks.sql';                       -- 环境 1/2：0 行
SELECT status, COUNT(*) FROM schema_migration_attempts
 WHERE filename = 'order/0022_r04_checks.sql' GROUP BY status;       -- 环境 1/2：0 行
```

k3s dev/UAT 实测输出（节选）：

```
8.4.13
ledger_table	1
attempt_table	1
0022_ledger_row	0
0022_attempt_rows	0
ledger_rows_from_0017	0
max_order_migration	order/0013_group_refund_state.sql
0022_group_check	0
0022_24h_check	0
```

mysql-unicorn 实测输出（节选）：

```
26.10.0
ledger_table	0
attempt_table	0
refund_applications	0
0022_group_check	0
0022_24h_check	0
any_0022_check_globally	0
```

**结论：预检时前后，0022 在任何可达持久环境均无账本行、无 attempt 残记录（含 `RUNNING`）、无 0022 对象，且其依赖的 0017/0019 尚未 apply。** 因此走 **分支 B**：允许就地修正 0022（Addendum §2.3.3 分支 B 条件满足）。

### 2.3 分支 B 的 checksum/ledger 影响

- 0022 从未写入任何 `schema_migrations`，不存在任何“已 apply 文件的 checksum 变化 → runner `checksum mismatch`”路径；dev/UAT 下次迁移会把修正后的 0022 作为**首次 apply**记录其 sha256 `85a2fc…`。
- 0022 的依赖顺序不变：`0017 → 0018 → 0019 → 0020 → 0021 → 0022`；0017/0018/0019/0020/0021 字节未改（脚本逐文件 ledger 断言通过）。
- **stop-the-line**：若 R0.4d2 或后续在任何环境发现 0022 已 apply，立即冻结 R0.4、把 0022 恢复为编辑前字节（`8339df…`）并改走 `0023`，登记 `review-tracking.md` freeze 区（Addendum §2.3.3.4）。本次未触发。

### 2.4 0022 修正内容（Addendum §2.2/§2.3.1/§2.5）

1. **判别式去枚举复制**（不再抄 0017 的 6 值枚举，只表达集合归属）：
   ```sql
   CHECK (((group_no IS NULL) = (source IN ('payment','legacy_expired')))
          AND (source IN ('payment','legacy_expired') OR group_no <> ''))
   ```
2. **24h 等值**：`CHECK (expires_at = DATE_ADD(entered_at, INTERVAL 24 HOUR))`；precheck 同步改为 `<>`。
3. **失败重跑口径**：删除“无条件 re-run safe”；头注释写明 MySQL DDL 隐式提交、`FAILED`/`RUNNING` attempt 的 5 步人工恢复（审计写修数据 → 删 attempt → 重跑 → 验 `CHECK_CLAUSE`/0 脏行）。
4. **同名不同义**：头注释声明本文件只按名判存在，语义由 R0.4d2 脚本的 `CHECK_CLAUSE` 断言兜底（不把“同名”当“同义”）。

### 2.5 测试（真实 MySQL，非 inline DDL）

`internal/services/order/r04_checks_integration_test.go`（真实迁移文件 + disposable schema）：

- `TestR04ChecksFreshSchemaAndValidRows`：fresh apply 0017/0019/0021/0022；断言 `CHECK_CLAUSE`（判别式含 `group_no is null` 且**不含** `system_zero/system_shortage/aftersale/tcc`；24h 含 `interval 24 hour` 且无 `>=`）；**24h 等值通过**、**24h+1s / 25h / 48h 拒绝**（`chk_lpr_expiry_24h`）、**23h / 24h-1s / 1h 拒绝**；0022 replay 后约束与 clause 不变。
- `TestR04ChecksDirtyRowsBlockMigration`：`payment+group` 脏行阻塞首个 ALTER；1h 脏行让首个 ALTER 落地、第二个失败（partial），修数据后重跑收敛；**新增 48h 脏行**（WIP 曾视为合法）阻塞 0022，修复后收敛。
- 48h 断言已从“合法”改为**拒绝**（WIP `:213-221` 的错误断言已删除）。

真实日志（`c38c54f`，`TEST_MYSQL_DSN=…/mall_integration_order_r04h`）：

```
=== RUN   TestR04ChecksFreshSchemaAndValidRows
--- PASS: TestR04ChecksFreshSchemaAndValidRows (0.65s)
=== RUN   TestR04ChecksDirtyRowsBlockMigration
    --- PASS: TestR04ChecksDirtyRowsBlockMigration/group_discriminator_dirty_row
    --- PASS: TestR04ChecksDirtyRowsBlockMigration/24h_SLA_dirty_row
    --- PASS: TestR04ChecksDirtyRowsBlockMigration/48h_dirty_row_blocks_0022
PASS
ok  	gin-kratos-gorm/internal/services/order	1.318s
```

（原始日志：`.tmp/evidence-r04/unit-tests-c38c54f.log`；未入库。）

---

## 3. R0.4b-impl：启动门禁 / 退出码 / 稳定 reason / 三字段日志

### 3.1 矩阵实现（Addendum §2.1/§2.1.1/§2.1.4）

| 请求 | 实现 | 失败退出 |
|---|---|---|
| `paid`（默认/显式/legacy=false） | 不读批准行；校验 approval_id 为空（N5） | config/门禁拒绝 |
| `shadow` | approval_id 空 + range 非空 + 存在 `APPROVED`、未过期、range 精确相等的 shadow 行；**门禁全过仍拒绝运行**（T4.4 未交付） | 门禁失败 exit 3；门禁通过 exit 4 |
| `settled` | approval_id 非空 + range 非空 + 按 id 取行且 `target_mode=settled`、`APPROVED`、`expires_at NOT NULL 且 > NOW(3)`、range 精确相等 | 门禁失败 exit 3 |
| legacy `true`（mode 未设） | config 映射为 settled 请求，同样要求 approval+range+行校验；永远不能选 shadow | 同 settled |
| 新旧冲突 / 未知/大小写/空 mode / N5 / range 缺失 | config 拒绝（panic → exit 2），错误文本带 token | exit 2 |

- **exit 1** 仍是基础设施失败；**exit 2** 是 config/N5/未知 mode/range 缺失（保留 panic 语义，Addendum §2.1.4 允许并明确不要求 logger 三字段，但必须有稳定 token）；**exit 3** 是批准行门禁；**exit 4** 是 shadow 运行态。
- 门禁在 NATS/DB 之外的任何 consumer/worker 注册前执行；`main_test` 断言拒绝路径没有 `connect nats`/`dial order`。
- 无 paid fallback：每个拒绝分支都是 `os.Exit(3/4)` 或 config 返回错误。
- 多行匹配：shadow 按 `approved_at DESC, approval_id ASC` 读取，取第一条全部条件满足的行；失败时给出确定性的首个失败原因 + 全部行的 detail。
- reason token 定义在 `internal/platform/config`（config 错误与进程日志共用同一常量，避免漂移）；`target_mode` 不符（settled 请求指向 shadow 行）按 `APPROVAL_NOT_APPROVED` 记录并在 detail 写明 `target_mode`（契约 token 表没有单独的 target-mode token，未自创）。

### 3.2 测试

- `internal/services/fulfillment/settlement_switch_integration_test.go`：paid/shadow/settled 矩阵、缺行/过期/非 APPROVED/target_mode 不符/range 不符/DB 不可读（表被 DROP）/nil DB、N5、未知 mode、多行中取有效行。每个负例同时断言稳定 `Reason`。
- `internal/platform/config/settlement_switch_test.go`：默认 paid、显式 paid、N5、shadow/settled 缺 range、settled 缺 approval、未知/空/大写 mode、legacy true/false/冲突；断言 `MODE_UNKNOWN` / `N5_APPROVAL_ID_MODE_MISMATCH` / `RELEASE_RANGE_MISSING` token。
- `cmd/fulfillment/main_test.go`（子进程真实退出码，`TEST_MYSQL_DSN`）：
  - **exit 3**：缺 settled 行、缺 shadow 行、`REVOKED`、过期、range 不符、approval 表不可读；
  - **exit 4**：有效 shadow 行 → `T4_4_SHADOW_NOT_DELIVERED`（含 `mode`/`approval_id`/`reason`）；
  - **exit 2**：paid + approval_id（`N5_APPROVAL_ID_MODE_MISMATCH`）、未知 mode（`MODE_UNKNOWN`）、shadow 缺 range（`RELEASE_RANGE_MISSING`）、legacy true 缺 approval；
  - **默认 paid**：无 settlement env 时门禁通过后止于 NATS 连接失败（exit 1），证明默认没有被门禁拒绝。
  - 所有拒绝用例断言 JSON 三字段与 `error` 串中的 `mode=... approval_id=... reason=...`，且未进入 NATS/dial。

真实日志（`c38c54f`，节选）：

```
=== RUN   TestStartupFailsClosedOnMissingSettledApproval   --- PASS
=== RUN   TestStartupFailsClosedOnMissingShadowApproval    --- PASS
=== RUN   TestStartupRejectsSettledNonApprovedRow           --- PASS
=== RUN   TestStartupRejectsExpiredSettledRow               --- PASS
=== RUN   TestStartupRejectsReleaseRangeMismatch            --- PASS
=== RUN   TestStartupFailsClosedWhenApprovalTableIsUnreadable --- PASS
=== RUN   TestStartupRejectsApprovalIDOutsideSettled        --- PASS
=== RUN   TestStartupRejectsUnknownModeWithToken            --- PASS
=== RUN   TestStartupRejectsShadowWithoutRange              --- PASS
=== RUN   TestStartupLegacySettledBoolRequiresApproval      --- PASS
=== RUN   TestStartupDefaultPaidPassesGate                  --- PASS
=== RUN   TestStartupRefusesShadowWithoutConsumerImplementation --- PASS
ok  	gin-kratos-gorm/cmd/fulfillment	11.945s
```

一条 exit 3 的真实输出（节选，`c38c54f`）：

```json
{"level":"ERROR","msg":"settlement switch verification failed","mode":"settled",
 "approval_id":"missing-approval","reason":"APPROVAL_ROW_MISSING",
 "detail":"approval row not found",
 "error":"settlement switch rejected: mode=\"settled\" approval_id=\"missing-approval\" reason=APPROVAL_ROW_MISSING detail=approval row not found"}
```

---

## 4. R0.4d1：部署/运维接线

### 4.1 改动

- `deploy/compose/services.yml`：fulfillment 增加
  `FULFILLMENT_SETTLE_CONSUMER_MODE=${…:-paid}`、`SETTLE_RELEASE_APPROVAL_ID`、`SETTLE_RELEASE_RANGE`；默认仍是 paid。
- `deploy/compose/.env.example`：注释示例（默认不启用）。
- `deploy/k8s/charts/mall/values.yaml`：新增 `settlement.consumerMode=paid` / `approvalID=""` / `releaseRange=""`。
- `deploy/k8s/charts/mall/templates/deployments.yaml`：仅 fulfillment 注入 mode（默认 `paid`）；approval/range **非空才注入**（空值不注入，避免把“未设置”变成显式空串）。
- `docs/ops.md`：新增 “Settlement consumer switch (fulfillment)” 段：env 契约、exit 2/3/4 与 reason token、批准行写入模板（`release_range` 必填）、`release_range <> ''` 0 行校验、`expires_at <= NOW(3)` 到期告警、0022 `FAILED`/`RUNNING` attempt 恢复流程；明确应用只读批准行、不写 `EXPIRED`、运行期不热切换。

### 4.2 渲染证据（`c38c54f`）

```
=== docker compose config（fulfillment）===
FULFILLMENT_SETTLE_CONSUMER_MODE: paid
SETTLE_RELEASE_APPROVAL_ID: ""
SETTLE_RELEASE_RANGE: ""

=== helm lint ===
1 chart(s) linted, 0 chart(s) failed

=== helm template 默认 ===
- name: FULFILLMENT_SETTLE_CONSUMER_MODE
  value: "paid"
（approval/range 未注入）

=== helm template --set settlement.consumerMode=settled --set settlement.approvalID=settled-2026-12-01 --set settlement.releaseRange=prod ===
- name: FULFILLMENT_SETTLE_CONSUMER_MODE
  value: "settled"
- name: SETTLE_RELEASE_APPROVAL_ID
  value: "settled-2026-12-01"
- name: SETTLE_RELEASE_RANGE
  value: "prod"
```

### 4.3 批准行模板 + 0 行校验（真实 MySQL 8.4.13）

在 disposable `mysql:8.4` 上应用真实 `migrations/fulfillment/0010_settlement_shadow_switch.sql`，按 `docs/ops.md` 模板写入 shadow/settled 样例行：

```
validation SQL: SELECT approval_id ... WHERE status='APPROVED' AND release_range=''
rows=0
sample:
settled-2026-12-01	settled	prod	APPROVED	2026-12-25 19:24:35.055
shadow-2026-12-01	shadow	prod	APPROVED	NULL
negative control (insert an APPROVED empty-range row):
violating_rows=1
```

负向控制证明该 SQL 不是恒 0（能检出违规行）；示例行已删除/容器销毁，未写入任何持久环境。**ops 侧真实到期告警规则与演练记录仍是 ops 的 ROUTED 项**（Addendum §2.1.2，owner=ops，2026-12-31）；本次只交付文档中的规则与验收 SQL，不声称“已配置告警”。

---

## 5. R0.4d2：`scripts/verify-r01-migrations.sh`

### 5.1 脚本改动

- `MIGRATION_FILES`：新增 `order/0021_zero_application_check.sql` + `order/0022_r04_checks.sql`（分支 B 无 0023）。
- 新增 `check_clause` / `assert_clause_has` / `assert_clause_lacks`：读取 `information_schema.CHECK_CONSTRAINTS.CHECK_CLAUSE` 并断言语义（不再只按名判存在）。
- 对象断言：`chk_refund_app_system_zero`（0021）、`chk_refund_app_group_discriminator`（不复制枚举）、`chk_lpr_expiry_24h`（等值、无 `>=`）。
- 行为断言：0021 非零 `system_zero` 拒绝、零额通过；0022 `payment+group` / 组级无 group 拒绝、payment/组级合法行通过；24h 等值通过，48h / 24h+1s / 25h / 23h 拒绝；0022 replay 不重复约束；**0022 partial failure**（首个 ALTER 已提交、第二个被 48h 脏行阻塞 → 修数据后重跑收敛、0 脏行）。
- runner replay grep 增 `order/0021|order/0022`。

### 5.2 真实执行（MySQL 8.4.13 + `mall/migrate:dev`，`c38c54f`）

```
HEAD             c38c54f899e81a3b33b55f581aaa04a869d74b23
MYSQL_IMAGE      mysql:8.4
MIGRATE_IMAGE    mall/migrate:dev
MySQL version: 8.4.13
[clean fresh apply through the real mall/migrate runner]
OK   ledger order/0021_zero_application_check.sql = 29c44ade…
OK   ledger order/0022_r04_checks.sql = 85a2fc82…
OK   0021 chk_refund_app_system_zero = 1
OK   0021 zero-amount semantics (CHECK_CLAUSE contains system_zero)
OK   0022 chk_refund_app_group_discriminator = 1
OK   0022 chk_lpr_expiry_24h = 1
OK   0022 discriminator NULL check (CHECK_CLAUSE contains group_no)
OK   0022 discriminator no enum copy (CHECK_CLAUSE omits system_zero/system_shortage/aftersale/tcc)
OK   0022 24h equality (CHECK_CLAUSE contains interval 24 hour)
OK   0022 24h equality (CHECK_CLAUSE omits >=)
OK   0022 accepts exactly 24h = 1
OK   0022 rejects 48h (rejected: 3819)
OK   0022 rejects 24h+1s (rejected: 3819)
OK   0022 rejects 25h (rejected: 3819)
OK   0022 rejects 23h (rejected: chk_lpr_expiry)
OK   0022 replay keeps one discriminator = 1
OK   0022 replay keeps one 24h CHECK = 1
OK   0022 blocked by the dirty discriminator row
OK   0022 blocked by the 48h dirty row (first ALTER committed)
OK   partial 0022 kept the discriminator = 1
OK   partial 0022 did not add the 24h CHECK = 0
OK   repaired 0022 adds the 24h CHECK = 1
OK   repaired 0022 has 0 blocking rows = 0
OK   0011 fail-closed diagnostic: d27 precheck
OK   runner refuses replay after a FAILED 0018 attempt
OK   operator repair records 0018 ledger row
R0.1 real migration verification passed
EXIT=0
```

原始日志：`.tmp/evidence-r04/verify-r01-migrations-c38c54f.log`（未入库）。`WORKTREE dirty` 仅由他人 `docs/review-tracking.md` 引起，与脚本输入无关。

---

## 6. 命令与日志索引（绑定 `c38c54f`）

环境（`docs/development.md` §1）：`go1.27.1 darwin/amd64`、仓库本地缓存、`GOFLAGS=-mod=readonly GOPROXY=off`；MySQL `mysql-unicorn` 26.10.0（`127.0.0.1:3306`），每包用 `scripts/test-dsn.sh` 派生独立 schema。

```bash
# 目标包（真实 DB）
TEST_MYSQL_DSN=<per-package> go test ./internal/platform/config -count=1
TEST_MYSQL_DSN=<per-package> go test ./internal/services/fulfillment -count=1
TEST_MYSQL_DSN=<per-package> go test ./cmd/fulfillment -count=1
TEST_MYSQL_DSN=<per-package> go test ./internal/services/order -run '^TestR04' -count=1

# -race（资金/门禁链路抗压）
TEST_MYSQL_DSN=<per-package> go test -race ./internal/platform/config -count=1
TEST_MYSQL_DSN=<per-package> go test -race ./internal/services/fulfillment -count=1
TEST_MYSQL_DSN=<per-package> go test -race ./cmd/fulfillment -count=1
TEST_MYSQL_DSN=<per-package> go test -race ./internal/services/order -run '^TestR04' -count=1

# 构建/静态
gofmt -l <changed go files>     # 空
go vet ./cmd/fulfillment ./internal/platform/config ./internal/services/fulfillment ./internal/services/order
go build ./...

# 迁移（真实 MySQL 8.4 + migrate 镜像）
MYSQL_IMAGE=mysql:8.4 MIGRATE_IMAGE=mall/migrate:dev ./scripts/verify-r01-migrations.sh

# 渲染
docker compose -f deploy/compose/infra.yml -f deploy/compose/services.yml config
helm lint deploy/k8s/charts/mall
helm template mall deploy/k8s/charts/mall -n mall
helm template mall deploy/k8s/charts/mall -n mall --set settlement.consumerMode=settled \
  --set settlement.approvalID=settled-2026-12-01 --set settlement.releaseRange=prod
```

结果（`c38c54f`）：

| 命令 | 结果 |
|---|---|
| config / fulfillment / cmd 包测试 | **PASS**（0.083s / 8.083s / 11.945s） |
| order `TestR04*` | **PASS**（1.318s） |
| 同上 `-race` | **PASS**（1.149s / 8.424s / 14.989s / 2.996s） |
| `go build ./...` | **PASS** |
| `gofmt -l` / `go vet` | 无输出 / **PASS** |
| `verify-r01-migrations.sh` | **PASS，EXIT=0**（8.4.13） |
| compose config + helm lint/template | **PASS**（`0 chart(s) failed`） |

原始日志：`.tmp/evidence-r04/{unit-tests,race-tests,verify-r01-migrations}-c38c54f.log`、`render-c38c54f.log`、`ops-approval-template-c38c54f.log`、`preflight-*.txt`（本地，未入库）。

---

## 7. 【测试范围】

- Fulfillment 启动门禁：paid/shadow/settled 矩阵、legacy bool 映射、N5、未知 mode、range 缺失、缺行/过期/非 APPROVED/target_mode 不符/range 不符/DB 不可读、默认 paid 不降级；子进程真实退出码 2/3/4 与三字段日志。
- `order/0022`：fresh apply、24h 等值、24h+1s/25h/48h/<24h 拒绝、判别式合法/非法行、replay、脏行与 partial-failure 收敛；`CHECK_CLAUSE` 语义断言（真实 MySQL 8.4.13）。
- 迁移 runner：0017→0022 fresh apply、ledger checksum、runner replay、FAILED attempt 拒绝重放与人工修复；0018/0011 既有场景非回归。
- 部署渲染：compose（默认 paid，approval/range 空）与 Helm（默认 paid；settled override 注入三值）；helm lint。
- 批准行模板/0 行校验：真实 8.4 上样例行 + 负向控制。

## 8. 【未覆盖范围】

- **未实现/未验证**：T2 的 24h writer（`INSERT … NOW(3)`）、24h expiry/compensation worker、`source=legacy_24h` 契约增量 → INV-46 仍 **DEFERRED(T2, 2026-12-31)**；本任务的 CHECK 只拒绝晚设 deadline，不能把行移出 `OPEN/RESOLVING`。
- **未实现**：T4.4 observe-only 双消费；shadow 仍硬拒绝（exit 4），未验证 shadow 运行态。
- **未交付**：多组 release metric、missing-reservation metric（`internal/services/order/**`/`inventory/**`，DEFERRED，Addendum §2.7）；INV-44 canonical evidence 行由 Lead 写，本文件不写台账、不声称 INV-44 有证据。
- **ops 侧**：真实到期告警规则部署/触发演练（owner=ops，ROUTED）；本次只交付 runbook 规则 + 验收 SQL。
- **未执行**：UAT/dev k3s 上的实际 rollout/迁移（0001–0013 之外）、生产环境、`helm upgrade`、集群级 apply；未对 `mall-data`/`mall-uat-data` 写任何数据（preflight 只读）。
- **不覆盖**：AC-27/AC-50 语义与 `internal/services/order/**` 生产代码（另派）；T3 commitment release；C-04 release event v1 的 producer/consumer 行为变更（只读核对既有实现，未改）。
- **不是**：实现 PASS、R0.4e verdict、merge 批准、Release PASS、AC-VERIFIED。

---

## 9. 反妥协自检清单

1. **并发/超时**：门禁用只读查询 + `StartupContext` 超时；拒绝即退出，无重试/无降级。迁移由既有 runner 的 lease 串行化；0022 的 DDL 隐式提交窗口由 precheck + attempt 台账 + 人工恢复流程处理，未用“再跑一次就行”掩盖。
2. **部分成功**：0022 第一个 ALTER 成功、第二个失败 → 记录 `FAILED`/partial，修数据后重跑收敛；集成测试与验证脚本均在真实 DB 上复现该窗口。没有“无条件 re-run safe”的假声明。
3. **崩溃窗口**：启动期在任何 consumer 注册前完成门禁；shadow 在门禁通过后仍拒绝（exit 4）。迁移进程崩溃留下的 `RUNNING` 残记录被 preflight 判为“可能部分 apply”，恢复流程要求显式删除并复验。
4. **是否加 if-else 绕过架构**：没有新增放行分支；门禁每个拒绝分支都是 exit。`target_mode` 不符复用 `APPROVAL_NOT_APPROVED`（契约无独立 token），没有新增契约 token。
5. **状态揉字段**：未改任何状态所有权；`REVOKED`/`EXPIRED` 的写者与告警口径按 Addendum 保持（应用只读）。
6. **妥协**：唯一显式限制是 §8 的 DEFERRED 项（T2 writer/worker/source、T4.4、metric、ops 告警演练）；均已登记 owner/expiry，不声称完成。
7. **默认值**：`paid` 仍是唯一默认；未把默认改为 shadow/settled。

---

## 10. QC round 3 的 9 MINOR 处置

| QC | 处置 | 证据/落点 |
|---|---|---|
| **S-1** 分支 A 与“必须改 0022”矛盾 | **本任务走分支 B**，0022 就地修正正是分支 B 路径；没有 0023，也没有违反“已 apply 不可变”。Addendum 文本矛盾仍由 Architect 修正（ROUTED）。 | §2.2/§2.3；0022 头注释 |
| **S-2** 环境名与仓库实际不符 | preflight 按实际环境枚举（compose dev / k3s `mall-data` / k3s `mall-uat-data` / 本机 mysql-unicorn），未使用 staging/prod 示例名。 | §2.2 |
| **S-3** attempt 表可能不存在 | preflight 先查 `information_schema.tables`；mysql-unicorn 无 attempt 表记为“无 attempt 记录”，不误报为“环境不可达”。 | §2.2 |
| **S-4** RUNNING 残记录/§2.5 只删 FAILED | preflight 采用更严格判据：**任何非空 attempt 残记录（含 RUNNING）→ 可能部分 apply**；0022 头注释、`docs/ops.md` 的 DELETE 条件均为 `status IN ('FAILED','RUNNING')`。`CANCELLED` 不是本仓库 runner 写入的状态（仅 RUNNING/APPLIED/FAILED）；若出现非这三类的残记录，按“任何残记录”规则人工审查后处理，不声称已自动化覆盖。 | §2.2/§2.4；`0022` 头注释；`docs/ops.md` |
| **S-5** INV-44 canonical 行内容 | **ROUTED / TODO（Lead）**：本任务不写 `evidence-matrix.md`，不声称 INV-44 有证据；行内“missing-reservation metric DEFERRED”与 owner 语义由 Lead 处理。 | §8；Addendum §2.4 |
| **S-6** `REVOKED` 无主 | **ROUTED（Architect）**：实现按“非 APPROVED 一律拒绝启动”处理（测试含 REVOKED）；未新增写者/未改 0010。 | `settlement_switch.go`；main_test |
| **S-7** `deploy/**` 归属 | 本次 R0.4d1 的 deploy 改动按 Lead Brief 的 allowed_paths 交付，已在 §1 列明；Addendum 的 allowed_paths 文本修正 ROUTED（Architect/Lead）。 | §1 |
| **S-8** T2 释放腿多组边界 | **ROUTED / DEFERRED（Architect）**：未改 T2 source/registry/consumer，也未声称多组 24h 释放可执行；仅交付 24h CHECK。 | §8 |
| **S-9** 引用漂移 | 测试行号以本次改动后的文件为准；`RELEASE_RANGE_MISSING` 同时用于 exit 2（config）与 exit 3（门禁）是契约 token 复用的既定事实，`docs/ops.md` 已按退出码分列。Addendum 行号漂移 ROUTED（Architect）。 | §3/§4 |

---

## 11. 反模式自省（对照 Addendum §5）

1. **“加个 if 就放行”**：无；门禁失败全部 exit，shadow 门禁全过仍 exit 4。
2. **“默认 false 掩盖半成品”**：shadow 是硬拒绝而非灰开关；解除条件仍是 T4.4 四条件。
3. **“文档写已关闭，代码没变”**：0022 的 `>=`/枚举复制/“re-run safe”已在代码与迁移中实际改掉，并由真实 8.4 脚本断言。
4. **“先改文件、事后补决策记录”**：preflight 在编辑前，证据含环境/时间/HEAD/编辑前 sha。
5. **“越界换验收”**：未改 `internal/services/order/**` 生产代码 / inventory / 台账；metric、T2、T4.4 显式延期。
6. **“一个状态/事件冒充事实”**：未新增 RPC/subject；只做门禁与 schema 约束。
7. **“失败当成功”**：DLQ/重投/`RELEASED` no-op 等既有协议未改；missing-reservation 静默仍是已登记风险，未声称解决。

---

## 12. 声明与边界

- 本文件是 WIP 自检，不构成实现 PASS；`R0.4e` Reviewer verdict、人类 merge、Release PASS、AC-VERIFIED 均未发生。
- 未改 SPEC/ADR/台账/Lead 文档；工作树中他人对 `docs/review-tracking.md`、`docs/specs/0001-fulfillment-quantity-split.md` 的修改未被纳入 `c38c54f`（仅 `git add` 精确路径）。
- 若 R0.4d2 之后在任一环境发现 0022 已 apply，按 Addendum §2.3.3 stop-the-line 处理（恢复 `8339df…` 字节 + 改走 0023 + freeze 登记），不得继续本分支。
