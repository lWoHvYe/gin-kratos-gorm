# REV-PAY-SETTLE-01-R0.4 — R0.4b/c-impl + R0.4d1/d2 实现复审（R0.4e）

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Tier | **L2（确认）**：命中跨服务事件/库存、迁移 `migrations/**`、`internal/platform/**`、`deploy/**`、`scripts/**` 触发条；不降级 |
| 审查范围 | **R0.4b-impl / R0.4c-impl / R0.4d1 / R0.4d2 的实现**（Addendum 0.3 r3，`c432ed6` ACCEPTED） |
| review_scope | 仅上述实现 + preflight 证据链；不审 AC-27/AC-50 语义、T2/T3/T4.4、C-04 producer/consumer 代码；不批准 merge/Release |
| base | `90b502c`（WIP 实现前身）；实现提交为 `c38c54f`（14 files，`git show c38c54f`）。`90b502c..c38c54f` 还含 R0.3 测试提交 `fe1600b` 与 docs/QC 提交，均不在 R0.4e 代码范围；代码 diff 以 `c38c54f` 的 14 文件为准 |
| code head | **`c38c54f899e81a3b33b55f581aaa04a869d74b23`** |
| 仓库 HEAD（重跑绑定） | **`9322e7544abf07f937c3d2adcb00a84b0bef4c26`**；`git diff c38c54f..HEAD --stat` 仅新增 `docs/implementation/PAY-SETTLE-01-R0.4-selfcheck.md`（423 行），**无代码变化** |
| 契约输入 | `docs/adr/0007-amd1-addendum-r0.4-c04-release.md`（ACCEPTED `c432ed6`，QC r3 READY `bd7c239`，9 MINOR 为 TODO）；ADR-0007 **Rev 2.3**；SPEC-0002 **v1.10**；SPEC-0001 **v1.5**（`4215b3c`，仅用于 AC-27 引用，不预判 AC 状态） |
| spec_version / adr_revision | SPEC-0002 **v1.10**；SPEC-0001 **v1.5**；ADR-0007 **Rev 2.3**；Addendum **0.3 r3 ACCEPTED** |
| integration_owner | System Architect（契约）；Developer（R0.4b/c/d 实现）；ops（重放/告警） |
| baseline_impact | **涉及** INV-44/45/46/49/50；不涉及 C-04 RefundBatch、Payment/AfterSales 状态机；INV-47 仅 0022 约束不改判定 |
| INV | INV-44（AC-REF；多组 + missing metric 缺口）；INV-45（DEFERRED T3）；INV-46（DEFERRED T2）；INV-47（DEFERRED T3，不涉及）；INV-49（DEFERRED T4.4）；INV-50（AC-REF） |
| affected_AC | AC-27（AC-REF，状态不变）；AC-50/AC-62/AC-67/AC-68 不在 R0.4；本报告新增 AC-VERIFIED=0 |
| verification | §2：四包 `go test` + `-race`、`verify-r01-migrations.sh`（MySQL 8.4.13，EXIT=0）、compose/Helm 渲染；绑定 HEAD `9322e75` / code `c38c54f` |
| 审查日期 | **2026-11-26** |
| Actor | `reviewer-r04`（Reviewer；与 Developer `dev-r04-impl`、Architect、QC 不同 actor） |
| Reviewed-by | `human (merge pending)` |
| release_status | **NOT RELEASE**；不批准 merge / Release / AC-VERIFIED |
| overall verdict | **FAIL（R0.4e 有范围）**：F-1 锁/MDL 验证缺失；F-2 preflight 未覆盖 prod/staging 的 fail-closed 证明；另有 Lead-owned 的 INV-44 行缺口与若干 MINOR/TODO |

> **语义隔离**：本报告是 R0.4 实现的日期化 verdict，不是 Spec 全量验收，不是 R0.3-R2 verdict，不是 Release PASS，也不替代人类 merge。Addendum 已 ACCEPTED 只表示设计被接受；R0.4e 仍可 FAIL。

---

## 0. 结论

**FAIL（有范围）。** R0.4b/c-impl 的启动门禁与 0022 约束本身实现正确，真实 MySQL 8.4.13 重跑通过；R0.4d1 的默认 `paid` 与 Helm 空值不注入也已独立复核。但 accepted Addendum 有两条**迁移/证据前置**没有满足，不能以“脚本绿/测试绿”替代：

1. **F-1 [BLOCKER]** Addendum §2.4 最后一条与 §2.6 `R0.4d2` 验收明确要求：`ADD CHECK` 的大表 DDL 必须在真实 MySQL 8.4 上验证并**记录锁时长**；不支持在线 DDL 时必须在迁移日志/ops runbook 记录维护窗。仓库中没有任何 `ALGORITHM/LOCK`、MDL、锁等待或时长证据（`scripts/verify-r01-migrations.sh`、`docs/ops.md`、selfcheck 均无）。这是生产迁移可用性/迁移阻断项，不能降级为 TODO。
2. **F-2 [BLOCKER]** Addendum §2.3.3 的目标环境包含 dev/staging/prod，且规定“任一目标环境不可达/无权限/无法确认 → 视为已 apply，走分支 A（0023）”。preflight 只覆盖 k3s `mall-data`、`mall-uat-data`、本机 mysql-unicorn 与 compose dev；对 **staging/prod** 仅记录“本 kube context 无该 namespace / 仓库无 values 目录”，没有 ops/Lead 的持久环境清单或对生产 DB 的只读查询。按 fail-closed 口径，这不足以证明分支 B（就地修改 0022）安全；若存在未查询的持久环境，0022 的 checksum 账本/重放会被破坏。

**已核对为正确的部分**（不因 FAIL 而抹去）：

- **C-04 release 无新增 RPC**：`c38c54f` 的 14 个文件不含 `api/**`、`internal/platform/events/**`、`internal/services/inventory/**` 或 Order 生产代码；`git diff --stat` 对这些路径为空。C-04 仍为既有 outbox event → Inventory consumer，未夹带 RPC。
- **shadow 硬拒绝**：门禁通过后仍 `os.Exit(4)`，日志含 `mode`/`approval_id`/`reason`，不注册 consumer、不回退 `paid`（`cmd/fulfillment/main.go:68-84`）。
- **paid/shadow/settled/legacy/range/expiry/N5**：矩阵实现与测试覆盖；默认 `paid`；空 range 不是通配符；expiry 用 DB `NOW(3)`；多行按 `approved_at DESC, approval_id ASC` 取第一条全满足行。
- **exit 1/2/3/4 与三字段**：exit 3 门禁、exit 4 shadow，均在三字段日志后退出；config 拒绝保留 panic（exit 2）且错误文本带稳定 token。
- **0022 分支 B**：编辑前/后 sha256 与 preflight 记录一致；判别式不再复制 0017 完整枚举；24h 为等值 `expires_at = entered_at + 24h`；replay/脏行/partial 在真实 8.4 上通过。
- **R0.4d1**：compose/Helm 默认 `paid`；Helm 空 approval/range 不注入；`docs/ops.md` 有写入模板、0 行校验 SQL、到期告警 SQL 与失败重跑流程。
- **R0.4d2**：`MIGRATION_FILES` 含 `order/0021` + `order/0022`；脚本读 `CHECK_CLAUSE` 断言语义；真实 `mysql:8.4` + `mall/migrate:dev` 运行 `EXIT=0`。

---

## 1. 审查范围与不在范围

**在范围**

> `90b502c..c38c54f` 的中间提交包含 `fe1600b`（R0.3 test-only）与多份 docs/QC 提交；它们不属于 R0.4e。本报告的代码范围严格为 `git show c38c54f` 的 14 个文件，另加 preflight 证据链。

- `cmd/fulfillment/**`：启动门禁、退出码、日志三字段、shadow 硬拒绝、默认 `paid`。
- `internal/platform/config/**`：三态/legacy bool/N5/range/未知 mode 的 fail-closed。
- `internal/services/fulfillment/settlement_switch.go` 与 `_test.go`：批准行只读校验矩阵。
- `migrations/order/0022_r04_checks.sql`：判别式、24h 等值、去枚举复制、重跑口径。
- `internal/services/order/r04_checks_integration_test.go`：0022 真实迁移测试（仅迁移语义）。
- `scripts/verify-r01-migrations.sh`：0017→0022 fresh apply、CHECK 文本/边界/dirty/partial/replay。
- `deploy/compose/**`、`deploy/k8s/charts/mall/**`、`docs/ops.md`：R0.4d1 接线与 runbook。
- preflight 证据（selfcheck §2、`.tmp/evidence-r04/preflight-*.txt`，及 sha256/HEAD/时间链）。

**不在范围 / 不声称**

- 不改 `api/**`、C-04 proto/RPC、Inventory/Order 生产代码；不审 T2/T3/T4.4 实现。
- 不审 AC-27/AC-50 的语义验收（见 R0.3-R2 报告）；本报告只确认 R0.4 没有夹带这些语义。
- 不批准 merge/Release/AC-VERIFIED；不把 Addendum 的 9 MINOR 视为已关闭。
- 不把 `docs/lead/**` 的 INV-44 行视为本报告的产物（owner=Lead）。

---

## 2. 实际执行证据（绑定 HEAD）

环境：`go1.27.1 darwin/amd64`；仓库本地缓存（`docs/development.md` §1：`GOCACHE=$PWD/.tmp/gocache`、`GOMODCACHE=$PWD/.tmp/modcache`、`GOFLAGS=-mod=readonly`、`GOPROXY=off`）；**MySQL 8.4.13**（disposable `mysql:8.4`，`127.0.0.1:13306`）。

### 2.1 Go 测试

命令（每包独立 schema；`-run` 按任务要求覆盖 `TestR04|TestSettlement|TestAC27`；`cmd/fulfillment` 另跑全包以真正执行 `TestStartup*`）：

```bash
# 绑定 HEAD=9322e75（code=c38c54f）
TEST_MYSQL_DSN=.../mall_integration_reviewer_config \
  go test ./internal/platform/config -run 'TestR04|TestSettlement|TestAC27' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_reviewer_ff \
  go test ./internal/services/fulfillment -run 'TestR04|TestSettlement|TestAC27' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_reviewer_order \
  go test ./internal/services/order -run 'TestR04|TestSettlement|TestAC27' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_reviewer_cmd \
  go test ./cmd/fulfillment -count=1 -v
```

| 包 / 命令 | 结果 | 关键用例 |
|---|---|---|
| `internal/platform/config` | **PASS，0.108s** | `TestSettlementSwitchConfigMatrix`（13 子用例）、`TestSettlementSwitchRangeIsTrimmed`、`TestSettlementSwitchLegacyBooleanMatrix` |
| `internal/services/fulfillment` | **PASS，0.861s** | `TestSettlementSwitchConfigFailuresDoNotTouchDB`、`TestSettlementSwitchPaidNeedsNoApprovalRow`、shadow/settled matrix、读失败不回退 |
| `internal/services/order` | **PASS，3.070s** | `TestR04ChecksFreshSchemaAndValidRows`、`TestR04ChecksDirtyRowsBlockMigration`（3 子）、`TestAC27*`（6 个） |
| `cmd/fulfillment` | **PASS，12.833s** | 12 个 `TestStartup*`（+ helper）：exit 2/3/4、三字段、默认 paid 过门禁、shadow 门禁通过仍 exit 4 |

### 2.2 `-race`

| 包 | 结果 |
|---|---|
| `internal/platform/config` | PASS，1.147s |
| `internal/services/fulfillment` | PASS，1.851s |
| `cmd/fulfillment` | PASS，14.279s |
| `internal/services/order` | PASS，3.875s |

原始日志：`.tmp/evidence-r04/reviewer-{config,fulfillment,order-r04-ac27,cmd-fulfillment,race}.log`（本次审查生成，未入库）。

### 2.3 迁移验证脚本（真实 MySQL 8.4 + `mall/migrate:dev`）

```bash
MYSQL_IMAGE=mysql:8.4 MIGRATE_IMAGE=mall/migrate:dev ./scripts/verify-r01-migrations.sh
```

| 项 | 结果 |
|---|---|
| HEAD | `9322e75`（脚本打印；代码与 `c38c54f` 相同） |
| MySQL | **8.4.13** |
| 退出码 | **0** |
| 0021 ledger | `29c44ade…` |
| 0022 ledger | `85a2fc82…` |
| 关键断言 | 0017→0022 fresh apply；`chk_refund_app_system_zero`；判别式不复制枚举；24h 等值；24h/24h+1s/25h/48h 拒绝；23h 被 0019 拒绝；replay 单约束；dirty/partial/repair 收敛；runner 拒绝 FAILED replay 后人工修复 |
| 原始日志 | `.tmp/evidence-r04/verify-r01-migrations-REVIEWER.log` |

### 2.4 preflight 哈希与时间链

| 证据 | 值 |
|---|---|
| preflight 时间/HEAD | `2026-11-25T19:07:00Z` / `c432ed6`（selfcheck §2.1） |
| 编辑前 0022 sha256 | `8339df0741584ef67c76e7daf417c2d6dee57ed26a7d1cf95ba0ef88cf574626`（与 `git show c432ed6:migrations/order/0022_r04_checks.sql` 一致） |
| 编辑后 0022 sha256 | `85a2fc8269e093bccc86a44bb6a457c0112bdd7da2fedc800e2c0f4b9dbbfe55`（与 `git show c38c54f:…` 一致） |
| 提交时间 | `c38c54f` 2026-11-26 03:20:21 +0800，晚于 preflight 03:07 |
| 环境 | k3s `mall-data`、k3s `mall-uat-data`、本机 mysql-unicorn、compose dev（preflight 日志） |

### 2.5 部署渲染（独立重跑）

```bash
docker compose -f deploy/compose/infra.yml -f deploy/compose/services.yml config
helm lint deploy/k8s/charts/mall
helm template mall deploy/k8s/charts/mall -n mall
helm template mall deploy/k8s/charts/mall -n mall --set settlement.consumerMode=settled \
  --set settlement.approvalID=settled-2026-12-01 --set settlement.releaseRange=prod
```

| 项 | 结果 |
|---|---|
| compose | `FULFILLMENT_SETTLE_CONSUMER_MODE: paid`；`SETTLE_RELEASE_APPROVAL_ID: ""`；`SETTLE_RELEASE_RANGE: ""` |
| `helm lint` | `1 chart(s) linted, 0 chart(s) failed` |
| Helm 默认 | 仅注入 `FULFILLMENT_SETTLE_CONSUMER_MODE=paid`；approval/range 不注入 |
| Helm settled override | 注入 `settled` + `settled-2026-12-01` + `prod` |

原始日志：`.tmp/evidence-r04/reviewer-render.log`。

---

## 3. accepted Addendum 逐条核对

### 3.1 C-04 release event（no RPC）

| 检查点 | 结论 | 证据 |
|---|---|---|
| 不新增 RPC/proto | **PASS** | `c38c54f` 14 文件无 `api/**`；`git diff --stat 90b502c c38c54f -- api` 为空 |
| 不改既有 release producer/consumer/registry | **PASS** | 无 `internal/platform/events/**`、`internal/services/inventory/**`、`refund_release.go`、`refund_admission.go` 变更 |
| `release_id=rr:<refund_id>`、同事务 outbox、`RELEASED` no-op 等既有事实链 | **未改（只读核对）** | 代码事实与 Addendum §1.4 一致；不在本实现范围内 |
| `group_count=1` / 多组缺口 | **不在 R0.4 交付** | 仍为 P1/manual；不得据此声称 INV-44 完成 |

### 3.2 启动门禁矩阵

| 请求 | 实现 | 判定 |
|---|---|---|
| `paid`（默认/显式/legacy=false） | 不读批准行；approval id 必须为空 | **PASS**（`settlement_switch.go:107-113`、`config.go:739-794`） |
| `shadow` | approval 空 + range 非空 + shadow 行 `APPROVED`、未过期（NULL 或 >`NOW(3)`）、range 精确相等 | **PASS**；门禁全过仍在 `main.go:68-84` exit 4 |
| `settled` | approval 非空 + range 非空 + 按 id 取行且 `target_mode=settled`、`APPROVED`、`expires_at NOT NULL` 且未过期、range 精确相等 | **PASS**（`settlement_switch.go:128-141,256-281`） |
| legacy `true` | config 映射为 gated settled；永远不能选 shadow | **PASS**（`config.go:759-764`；config test） |
| legacy `false`/未设 | 默认 paid | **PASS** |
| 新旧冲突 | config fail-fast | **PASS**（`config.go:771-779`） |
| 未知/大小写/显式空 mode | 不默认 paid，exit 2 | **PASS**（`config.go:766-770`；`TestStartupRejectsUnknownModeWithToken`） |
| 多行匹配 | `approved_at DESC, approval_id ASC` 第一条全满足 | **PASS**（`settlement_switch.go:200-254`） |
| DB 不可读 | 启动失败，不猜 paid | **PASS**（`TestSettlementSwitchReadFailureDoesNotFallBackToPaid`、`TestStartupFailsClosedWhenApprovalTableIsUnreadable`） |

### 3.3 退出码与日志三字段

| 项 | 结论 | 证据 |
|---|---|---|
| exit 1 基础设施 | PASS | `main.go:27-40,85-103,121-125` |
| exit 2 config/N5/未知/range | PASS | panic 语义（Go exit 2）；错误文本含 `N5_APPROVAL_ID_MODE_MISMATCH`/`MODE_UNKNOWN`/`RELEASE_RANGE_MISSING`；`main_test` 断言 `==2` |
| exit 3 门禁 | PASS | `main.go:50-63`；`TestStartup*` 六种缺行/过期/非 APPROVED/range 不符/表不可读 |
| exit 4 shadow | PASS | `main.go:68-84`；`TestStartupRefusesShadowWithoutConsumerImplementation` |
| 三字段 | PASS | exit 3/4 同时含 JSON `"mode"/"approval_id"/"reason"` 与 `mode=... approval_id=... reason=...`；`requireExit` 断言 |
| 门禁在 consumer 注册前 | PASS | 门禁在 NATS/workers/`RegisterFulfillmentServiceServer` 之前；测试断言无 `connect nats`/`dial order` |
| 无 paid fallback | PASS | 每个拒绝分支 `os.Exit`，无 else 降级；shadow 通过门禁仍拒绝 |

### 3.4 `order/0022`

| 检查点 | 结论 | 证据 |
|---|---|---|
| 分支 B preflight 在编辑前 | **PASS（流程链成立）** | 时间 03:07 < 提交 03:20；编辑前/后 sha 与 git blob 一致；preflight 日志存在 |
| 判别式不复制 0017 枚举 | **PASS** | `0022:44-49,57-58` 只表达 `payment/legacy_expired` 集合归属；脚本断言 CHECK_CLAUSE 不含 `system_zero/system_shortage/aftersale/tcc` |
| `expires_at = entered_at + 24h` 等值 | **PASS** | `0022:62-85`；脚本接受 24h、拒绝 24h+1s/25h/48h；23h 由 0019 下限拒绝 |
| 不复制 `source` 完整枚举 | **PASS** | 同判别式；0017 的 `chk_refund_app_source` 仍是唯一封闭枚举 |
| replay | **PASS** | 脚本重放 0022 后约束仍各 1 个 |
| 脏行 fail-closed | **PASS** | payment+group、组级无 group、48h 脏行均被拒（3819） |
| partial failure | **PASS** | 首个 ALTER 提交、第二个被 48h 脏行阻塞；修数据后收敛，0 阻塞行 |
| 同名不同义 | **PASS（脚本侧）** | 脚本读 `CHECK_CLAUSE` 比对；迁移文件本身只按名判存在，已被脚本兜底（残余风险见 N-4） |
| 失败重跑口径 | **PASS** | `0022:17-29` 删除“无条件 re-run safe”，写明审计写→删 attempt→重跑→复验；`docs/ops.md:238-258` 一致 |
| 大表 DDL 锁时长 | **FAIL（F-1）** | 见 §4.1；脚本/ops/selfcheck 无任何锁/MDL/时长证据 |

### 3.5 R0.4d1 部署接线

| 检查点 | 结论 | 证据 |
|---|---|---|
| 默认 `paid` | PASS | compose `${…:-paid}`；Helm `default "paid"`；values `consumerMode: paid` |
| 不改变生产默认 | PASS | 未改既有 production values；Helm 默认渲染仅 `paid` |
| Helm 空值不注入 | PASS | `{{- with ... }}`；默认模板无 approval/range |
| compose 空值 | **PASS（语义）/MINOR** | compose 注入空串；config 对这些字段 trim 后等价 unset；见 N-3 |
| 批准行模板 | PASS | `docs/ops.md:186-216` 含 settled/shadow 模板、`release_range <> ''` 0 行校验、负向控制 |
| 到期告警 | **按契约 ROUTED** | `docs/ops.md:218-236` 有 SQL/owner/expiry；真实 ops 告警规则与演练是 owner=ops，不在 R0.4d1 代码范围 |
| 不写审批行/不热切换 | PASS | 只读 `VerifySettlementSwitch`；无 UPDATE/INSERT；`docs/ops.md:218-222` |

### 3.6 R0.4d2 脚本

| 检查点 | 结论 | 证据 |
|---|---|---|
| `MIGRATION_FILES` 含 0021+0022 | PASS | `scripts/verify-r01-migrations.sh:28-37` |
| fresh apply 0017→0022 | PASS | 真实 `mall/migrate:dev`；ledger 逐文件 checksum |
| 对象断言 | PASS | `chk_refund_app_system_zero`、判别式、`chk_lpr_expiry_24h` |
| CHECK_CLAUSE 语义 | PASS | 读 `information_schema.CHECK_CONSTRAINTS`；不复制枚举、等值、无 `>=` |
| 真实 MySQL 8.4 | PASS | 8.4.13，EXIT=0 |
| schema_migrations 证据 | PASS | 脚本逐文件 ledger checksum |
| preflight 复核 | PASS（文本侧） | selfcheck §2.3 记录分支 B 与 stop-the-line；脚本不重跑 preflight（设计如此） |
| replay/脏行/partial | PASS | 见 §3.4 |
| **锁时长** | **FAIL（F-1）** | 脚本无 `ALGORITHM/LOCK`、无 MDL/耗时测量；`docs/ops.md` 无维护窗记录；selfcheck §5/§6 未提 |

---

## 4. FAIL 项（具体契约条款 + 修复 Plan 要求）

### F-1 [BLOCKER] `ADD CHECK` 大表 DDL 的锁/MDL/时长验证与维护窗记录缺失

**违反条款**

- `docs/adr/0007-amd1-addendum-r0.4-c04-release.md:339`（§2.4 最后一条）：“MySQL 8.4 `ADD CHECK` 会扫描全表；`ALGORITHM=INPLACE, LOCK=NONE` 对 ADD CHECK 的支持 `[不确定]`，**必须由 R0.4d2 在真实 8.4 上验证并记录锁时长**；若不支持，必须安排维护窗并在迁移日志/ops runbook 记录（owner=Developer，expiry=2026-12-31）。”
- 同文件 `:363`（§2.6 `R0.4d2` 验收）：“真实 MySQL 8.4 运行（replay/脏行/**锁时长**）”。

**事实**

- `scripts/verify-r01-migrations.sh` 全文没有 `ALGORITHM=`、`LOCK=NONE`、`performance_schema.metadata_locks`、MDL/耗时测量或 `EXPLAIN`；只有 0017→0022 的功能/边界断言。脚本里的 `MIGRATION_LOCK_WAIT_SECONDS` 是 **migration runner 的 lease 等待**，不是 `ADD CHECK` 的 DDL 锁时长。
- `docs/ops.md:238-258` 只写失败重跑流程，没有“在线 DDL 支持结论/维护窗/锁时长”；
- `docs/implementation/PAY-SETTLE-01-R0.4-selfcheck.md` §5（258-306）与 §6（310-355）未提锁时长；
- 脚本在空的 disposable 表上执行 DDL，无法代表生产大表；`[不确定]` 仍未转为可核验结论。

**为什么不能降级为 TODO**

`ADD CHECK` 在 MySQL 8.4 上的 MDL/表扫描会阻塞生产写入；Addendum 把该项写成 R0.4d2 的验收项，不是风格问题。没有该证据，R0.4d2 不能 PASS，也不能声称“迁移已验证/可在线执行”。

**修复 Plan 要求（Reviewer 不接受局部补丁式文字声明）**

1. 在真实 MySQL 8.4 + 代表性行数（至少覆盖 `refund_applications`、`legacy_payment_resolution` 的预期量级；可在 disposable clone 上造数）执行 0022 的两个 `ALTER`，记录：
   - `ALGORITHM=INPLACE, LOCK=NONE` 是否被接受（或明确报错/回退）；
   - `performance_schema.metadata_locks` / `performance_schema.events_statements_history_long` 或等价方式测得的 MDL/锁等待/DDL 时长；
   - 测试表行数、MySQL 版本、HEAD。
2. 若在线 DDL 不支持或时长不可接受：在 `docs/ops.md`（或 `docs/runbooks/c-migration-rollback.md`）写明**维护窗、停写/滚动步骤、回滚路径**，并在 selfcheck 迁移日志记录结论；不得写“已在线验证”这类未测声明。
3. 把上述结论与原始输出加入 R0.4d2 证据（绑定 HEAD），再提交重审 Plan；在重审前 R0.4 保持冻结（Workflow 9）。
4. `0022` 的 SQL 本身不需要为“锁时长”改语义；若为在线 DDL 需要改写（例如拆约束/加 `ALGORITHM`），必须回到 Architect 确认是否仍是同一契约。

### F-2 [BLOCKER] preflight 未对 staging/prod 证明“未 apply”，分支 B 的 fail-closed 前提不完整

**违反条款**

- Addendum `:284`（§2.3.3 目标环境）：“所有**持久**环境——至少共享 dev/staging/prod，以及任何带 durable volume / 共享 DB 的预发环境。一次性容器……不构成‘已 apply’。”
- Addendum `:306-308`（fail-closed）：“任一目标环境**不可达/无权限/无法确认 → 视为‘已 apply’，走分支 A（0023）**……不得用‘查不到就当作没跑过’换取便利。”
- Addendum `:312-315`（R0.4c-impl 验收）：preflight 证据必须覆盖**所有目标环境（环境清单可核）**；分支 B 还要求 0022 编辑提交时间/HEAD 在 preflight 之后。

**事实**

- selfcheck §2.2 表第 5 行（`docs/implementation/PAY-SETTLE-01-R0.4-selfcheck.md:65`）把 staging/prod 记为“本 kube context 无 staging/prod namespace（仅 mall、mall-data、mall-uat、mall-uat-data）与仓库内无对应 values 目录”；preflight-3 日志同。
- 这只证明“当前 kube context 没有该 namespace / 仓库没有 values 目录”，不等于“不存在生产/预发持久 DB”，也没有 ops/Lead 的持久环境清单或对生产 DB 的只读查询。
- `docs/ops.md:628-631` 明确存在 opt-in production HA profile（external MySQL）；仓库没有 production values 不等于生产 DB 不存在或 0022 从未被 apply。
- 在 fail-closed 口径下，“无法确认”必须视为已 apply。现有证据不足以排除“某持久环境已 apply 0022”；若存在，就地修改会破坏 `schema_migrations` checksum 与 runner 重放（Addendum `:306-310` 的 stop-the-line 场景）。

**为什么不能降级为 TODO**

Addendum 明确把该场景称为“迁移完整性阻断，不能降级为 TODO”（`:316`）。分支 B 是“就地改已存在文件”的高风险路径；没有完整环境清单，就不能把“0 行”当作全局结论。

**修复 Plan 要求**

1. 由 **ops/Lead** 出具书面持久环境清单（环境名、DB endpoint、是否生产/预发、是否共享/durable、负责人、查询时间），并对每个环境执行 preflight 的两条只读 SQL + `sha256sum` 比对；或由 ops 明确书面确认“不存在 staging/prod 持久 DB，0022 不可能被 apply”。
2. 若任一环境有 `schema_migrations` 0022 行、attempt 残记录（含 `RUNNING`），或无法确认：立即按 §2.3.3 走 **分支 A**：
   - 把 `0022` 恢复到 preflight 前字节 `8339df…`（`git show c432ed6:…`）；
   - 新增 `migrations/order/0023_r04_checks_v2.sql`（替换两个约束，优先新名 + ADD 后 DROP 旧名）；
   - 同步 fresh-apply 顺序、`scripts/verify-r01-migrations.sh` 的 `MIGRATION_FILES` 与对象断言；
   - 按 §2.3.3.4 登记 `docs/review-tracking.md` 的 change freeze。
3. 若 ops 确认无生产/预发持久 DB：把该书面确认加入 preflight 证据（绑定 HEAD），并在 selfcheck 里写明“分支 B 的环境清单依据”；仍不得把“当前 kube context 无 namespace”单独当作全局证明。
4. 在 F-1/F-2 关闭前，R0.4 冻结；不得用新 `change_id` 或 `hotfix:` 绕过。

> **不是 `[ARCHITECTURAL_FLAW]`**：F-1/F-2 是迁移证据/环境清单的缺失，不是状态机或事件协议缺陷；不需要打回 Architect 改模型（Addendum 已 accepted）。但 F-2 若改走分支 A，需按 Addendum 由 Architect 确认约束语义不变（只换文件名）。

---

## 5. 非阻断发现与 TODO

### N-1 [MAJOR，Lead-owned] INV-44 canonical 行缺 missing-reservation metric 延期

- Addendum `:333-337`（§2.4）要求 `evidence-matrix.md` 的 INV-44 行至少覆盖：单组 release happy / 重复投递 no-op / 缺 reservation 行 no-op **且 metric 缺口显式登记为延期** / DLQ 重放 / 多组不释放 P1。
- 现有行 `docs/lead/PAY-SETTLE-01/evidence-matrix.md:64` 的状态只写“多组释放缺口 DEFERRED”，未写 missing-reservation metric/告警的延期；描述列也未覆盖“缺行 no-op”。
- owner=**Lead**（不是 Developer 代码）；在补齐前不得声称“INV-44 有证据/已收口”（Addendum `:336`）。本报告按该条将其列为 R0.4 收口的 Lead-owned 缺口，不计入 F-1/F-2 的 Developer 修复项。

### N-2 [MINOR，Architect errata] Addendum §2.1 的“空 mode”自相矛盾

- 同一张表第 1 行把“未设置/空/`paid`”都算 paid（`Addendum:152`），最后一行又把“空且 legacy 未设”判为“不默认 paid、非零退出”（`:158`）。
- 实现与测试选择后者：显式空 `FULFILLMENT_SETTLE_CONSUMER_MODE` = `MODE_UNKNOWN`/exit 2；未设置 = paid。该选择符合 `:158` 的 fail-closed 与 Addendum 引用的 `config.go:749-752` 口径（当前实现 `config.go:765-770`），也已被测试固定。
- 需 Architect 在 Rev 2.4 errata 中把“空”表述收窄为“未设置 = paid；显式空 = 未知”。不影响本次实现正确性。

### N-3 [MINOR] compose 注入空 approval/range

- Helm 用 `with` 不注入空值；compose 显式注入 `SETTLE_RELEASE_APPROVAL_ID=""`、`SETTLE_RELEASE_RANGE=""`（`deploy/compose/services.yml`、render 日志）。
- 对 config 语义等价于 unset（这两个字段 trim 后为空；只有 mode 区分 unset/empty），因此不改变默认/不放行。若“空值不注入”按字面要求，compose 也应改为不设置这两个 key；属 [TODO]，请 Dev 忽略此项，不影响本次 PASS/FAIL 结论。

### N-4 [TODO] 0022 的“同名不同义”只由脚本兜底

- 迁移文件只按 `information_schema.TABLE_CONSTRAINTS` 名称判存在（`0022:52-60`），同名不同义的旧约束会被跳过；R0.4d2 脚本的 `CHECK_CLAUSE` 断言能检出（已验证），但**裸 runner 直接 apply**不会。
- Addendum 已接受“脚本必须比对 CHECK_CLAUSE”的方案；残余风险保留为 [TODO]/已知边界，不新增迁移逻辑。

### N-5 [TODO/ROUTED] ops 到期告警演练

- `docs/ops.md:218-236` 已写规则/SQL/owner/expiry，但真实告警规则落 runbook 与一条触发/演练记录仍是 owner=ops（expiry 2026-12-31），不在 R0.4d1 代码范围；不得声称“已配置”。

### N-6 [TODO] QC round 3 的 9 MINOR 仍有未关闭项

- S-2/S-6/S-7/S-8/S-9 的文本/归属修正仍属 Architect/Lead；Developer 已处理 S-3/S-4/S-5 的代码侧（preflight attempt 表判定、RUNNING、INV 行不代写）。这些不改变本 verdict，但 Addendum 的 9 MINOR 不得被表述为“已全部关闭”。

### N-7 [TODO，Lead] evidence-matrix 的 AC-27 行仍写“v1.5 待 QC”

- `docs/lead/PAY-SETTLE-01/evidence-matrix.md:42` 仍写“SPEC v1.5 AC-27A 待 QC；AC-27A 谓词待实现/重跑”。v1.5 已 `4215b3c` approved、QC V15 R2 READY；该行应刷新为“AC-27A 谓词/前置缺陷状态”，否则与 canonical 状态漂移。owner=Lead，非本次实现阻断。

---

## 6. INV 与 affected AC

| INV | 结论 | 说明 |
|---|---|---|
| **INV-44** | **AC-REF；不得称已完成** | C-04 release 无 RPC、单组 P0 边界不变；多组 P1/manual、missing-reservation metric 仍 DEFERRED；evidence-matrix 行不完整（N-1） |
| **INV-45** | **DEFERRED（T3）** | R0.4 不实现 commitment release；0022 只约束 application/24h，不改 0018 组成；无 VERIFIED |
| **INV-46** | **DEFERRED（T2）；仅 CHECK 落地** | 24h 等值 CHECK 已实现并验证；writer/expiry-compensation worker/T2 `legacy_24h` source 未交付；不得声称 24h 状态机闭环 |
| **INV-47** | **DEFERRED（T3）** | 行级/运费/责任语义不在 R0.4；0022 不改其判定 |
| **INV-49** | **DEFERRED（T4.4）；门禁侧 PASS** | shadow 硬拒绝与默认 paid 已实现；shadow 运行态/双消费未交付，T4.4 四条件未满足 |
| **INV-50** | **AC-REF（R0.3-R2）** | 零元路径无 R0.4 改动；仍 AC-REF，不是 AC-VERIFIED |

**affected_AC**：AC-27（SPEC-0001 v1.5；本报告不改变其 **AC-REF** 状态，见 R0.3-R2）；AC-50/AC-62/AC-67/AC-68 不在 R0.4 实现范围，未改变状态。**本报告新增 AC-VERIFIED：0。**

---

## 7. 宪法 Article I–V / Governance 核对

| 条款 | 结论 | 依据 |
|---|---|---|
| I.1 拒绝“屎山雕花” | **PASS**（实现层） | 门禁每个拒绝分支 exit；无“默认放行/降级 paid”的 else；无新增状态字段 |
| I.2 Fail-Closed | **PASS（代码）/部分 FAIL（证据）** | 门禁/配置/DB 不可读均失败退出；但 F-2 的 preflight 未把“无法确认”全部按已 apply 处理 |
| I.3 契约优先 | **PASS** | C-04 不改 proto/RPC；Addendum §1.2-§1.4 已 accepted；实现未夹带新契约 |
| I.4 拒绝自我妥协 | **FAIL（有范围）** | F-1 的锁/MDL 验收项缺失；不能以脚本绿替代 |
| II.1 事实分离 | **PASS** | 不改 release fact 链；不把 T3 commitment release 混入；无 RPC |
| II.3 幂等/最终一致 | **PASS（未改）** | release_id/outbox 同事务/RELEASED no-op 未改；0022 约束 additive |
| II.5 退款与履约准入 | **PASS（未改）** | permit/准入代码未改；shadow 不运行；T4.4 仍拒绝 |
| III.1 DTM/Outbox | **PASS（未改）** | 无 Order/Inventory 生产写入变更 |
| III.2 DB 访问 | **PASS** | 启动门禁用 `database/sql` 参数化查询；无 GORM；DDL 由迁移文件 |
| III.4 静态/启动 fail-fast | **PASS** | exit 2/3/4；`cmd/*` panic 允许；无跨服务包引用 |
| IV.1 证据不可妥协 | **FAIL（有范围）** | 真实 Go/迁移日志齐全，但 F-1 的锁时长证据缺失；AC-REF 未写成 VERIFIED |
| IV.2 重灾区抗压 | **PASS（已有）/-race** | 四个包 `-race` PASS；release/结算的既有故障证据未改；R0.4 不改资金写入 |
| IV.4 实现/发布隔离 | **PASS** | release_status=NOT RELEASE；不声称 Spec 全量验收 |
| Workflow 9 stop-the-line | **触发** | 本 FAIL 冻结 R0.4 范围；修复须挂重审 Plan，不得新 id/hotfix 绕过 |
| Governance 4 人类检查点 | **保留** | 本报告不批准 merge；人类 merge pending |
| Governance 6 人类豁免 | **PASS** | 仅引用 WIP waiver；Addendum 接受不豁免 Reviewer/merge/Release/AC-VERIFIED |
| 角色独立性 | **PASS** | Actor `reviewer-r04` ≠ Developer `dev-r04-impl`；只读代码/契约，仅改 Reviewer-owned 台账 + 本报告 |

**`[ARCHITECTURAL_FLAW]`：无。** F-1/F-2 是验收证据与环境清单的缺失；不需要把 R0.4 打回 Architect 重设计。若 F-2 改走分支 A，Architect 只需确认“约束语义不变、只换文件名”。

---

## 8. 结论与后续

- **R0.4e verdict：FAIL（有范围）**，release_status **NOT RELEASE**。R0.4b/c/d 的实现主体质量可接受，但 **F-1（锁/MDL/维护窗）** 与 **F-2（preflight 环境覆盖）** 关闭前，R0.4 不得 PASS、不得 merge、不得声称“迁移已验证”。
- **PASS 的前置**：F-1/F-2 均有可核验证据；N-1 由 Lead 补齐；9 MINOR 的 ROUTED 项按 owner 处理。重审时必须重新绑定 HEAD，并重跑 `go test`（至少本次四包）+ `verify-r01-migrations.sh`。
- **不声称**：Release PASS、merge、Spec 全量验收、INV-44 已完成、AC-27 AC-VERIFIED、shadow 可运行。
- **下一步**：Lead 建 R0.4e 修复 Plan（F-1/F-2 owner=Developer；F-2 环境清单 owner=ops/Lead；N-1 owner=Lead），完成后走重审；冻结期间不得在同一 `change_id` 提交受管代码（Workflow 9）。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-26
**报告**：`docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.4.md`
