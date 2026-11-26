# ADR-0007 Amd1 — AC-50-FIX-01 Technical Impact Analysis（DRAFT）

> 本文件是**技术影响分析**，不是 ADR、不是实现、不是验收、不是台账。契约正文与裁决在
> `docs/adr/0007-amd1-addendum-ac50-nowaybill.md` **DRAFT 0.5**；本文件只回答“PO v1.11 的
> `REVIEW_REQUIRED` 落在哪些文件/表/状态机、有哪些代价与风险”。代码/测试状态归 Developer/Reviewer；
> AC 状态归 Reviewer。

| 项 | 值 |
|---|---|
| Status | **DRAFT 0.3 — pending PO/QC（QC round 5→6）**（未批准；不构成实现依据；不代表 AC-VERIFIED / merge / Release PASS） |
| Change ID | **`PAY-AC50-REVIEW-01`**（本 L2；Parent `PAY-SETTLE-01` 已冻结，仅历史引用；实施须按本 L2 Plan 的 C-4 人类批准） |
| Parent | `PAY-SETTLE-01`（冻结；本 L2 不推进其受管代码/台账） |
| Tier | **L2**（跨 Fulfillment/Order、资金/履约准入、既有 RPC 响应语义、共享 metrics 包；`docs/collab/CONVENTIONS.md:90` 任一命中即 L2） |
| Spec-Ref | SPEC-0002 **v1.11（in-review；工作树含 PO 未 commit 的 R4/R5 delta；行号以 2026-11-26 读取为准）**：S20 唯一载体/分路径镜像/审计（`0002:90,103-106`）、INV-51/52（`0002:147-148`）、AC-50（`0002:227-249`）、AC-51（`0002:251-263`）、AC-61（`0002:320-327`）、AC-62（`0002:328-345`）、AC-65（`0002:392-407`）、AC-67（`0002:424-436`）、**AC-69..72**（`0002:451-499`）、§5.10（`0002:662-681`）、错误码（`0002:605-606` 区间）、**D-30..D-33**（`0002:845-849`）、§11/§12（`0002:882-905`）；人类 2026-11-26 授权（`docs/review-tracking.md:366-373`）；SPEC-0001 **v1.5**：AC-27（`0001:592-613`）、AC-27A（`0001:615-645`）、§16.2（`0001:862-864`）、§16.3（`0001:874-879`）；ADR-0007 **Rev 2.3** §5.2.1/§5.2.2/§5.2.4（`0007:556-583,591-594`）、§5.3（`0007:611-623`）、§5.7（`0007:706-730`）、§12.5.3（`0007:1130`） |
| QC 输入 | `docs/collab/qc-reports/QC-PAY-SETTLE-01-AC50-NOWB.md` R3（`:575-739`）+ R4（`:743-955`）+ **R5（`:959-1062`；R5-B1、R5-M1..M4、R5-T1/T2）** |
| Base（文档） | `d2149c0a3935169206d6a9fa31ca8794b937079c`（Plan `d2149c0`/C-2/0.5 已由 Lead 修正，R6-M1）；SPEC-0002 v1.11 为**工作树 in-review**（未 commit；Reviewer 必须绑定 blob） |
| 代码基线 | 工作树无代码改动；代码事实=本 base 的 `internal/services/**`、`migrations/order/**`（锚点与历史 DRAFT 0.3 记录 `:15` 的 `c38c54f` 一致） |
| Actor | `arch-ac50-impact`（System Architect） |
| Reviewed-by | 待 QC round 6（本文件不代签） |
| Scope | schema、TCC、部分金额、幂等/重放/崩溃、文件级矩阵、兼容/回滚、风险、假设；**不做**实现/迁移/SPEC/台账/测试执行 |
| 只读声明 | 未执行 `go test`/`-race`/真实 MySQL/迁移；行号是阅读事实，不构成执行证据；`[不确定]` 已显式标注 |

---

## 0. 结论摘要

1. **PO 已定名/定表示（v1.11 D-30）**：持久裁决值 = **`REVIEW_REQUIRED`**；**唯一权威载体** `order_refund_admissions`（`result=REVIEW_REQUIRED`、`state=PENDING_REMOTE`，不新增 state 值）；发起方镜像**分路径**（有 `refund_applications` 行才写；普通售后写 `after_sale_refund_tasks.last_error`，不新建行）；TCC 批次 `MANUAL_HOLD`；对外错误码仍 `REFUND_ADMISSION_CONFLICT`；**无 schema 变更、无迁移、无回填**（D-30 `0002:845`）。
2. **schema 影响 = 0**：`order_refund_admissions.result/state` 均 `VARCHAR(32)`、无 CHECK；`refund_applications.admission_result` 为 `VARCHAR(24)`，`REVIEW_REQUIRED`（15 字符）可容纳；历史稿的 `REFUND_ADMISSION_CONFLICT`（25 字符）**放不进** `VARCHAR(24)`，这正是 D-30 不把它当持久值的事实依据（§2）。**R4-B3 的审计载体（D-32）全部复用既有行/列**：admission 行 + `version`/时间戳 + 发起方 `last_error`（512/1024）+ 日志 + 工单；**不新增表/迁移**（§2.4）。
3. **TCC 影响是本次最大增量**：历史 DRAFT 0.3 的 `result=CONFLICT` 会在 `ensureTccRemoteAdmissionTx` 的 `default` 早失败；v1.11 要求把 `REVIEW_REQUIRED` 当**合法持久裁决**读回、批次置 `MANUAL_HOLD`、自动续跑返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`、`refund_batches.last_error` 记录可查询原因。PO §11 已明确 `tcc_*.go` 须纳入实现 `allowed_paths`（§3）。
4. **部分金额（R3-B2）按 PO D-31 三分法**：**不调用 `CancelIfCreated`**，只用 Order 自有事实——组投影 `SHIPPED/RECEIVED` → POST；组投影未离仓 → `PARTIAL`（在留承诺/占位之前拒绝，零写入）；投影缺失且 `orders.status ∈ {SHIPPED,COMPLETED}` 或不可判定 → `REVIEW_REQUIRED` + 1 条 `RESERVED` commitment（冻结额度，AC-62）。满足 AC-50 `0002:217`（不改运单）、`0002:226`（已离仓部分合法）、INV-52（§4）。
5. **幂等/崩溃**：`AdmitRefund` 必须先读 existing（现状 `GetIntent` 在 `:168`、existing 在 `:179`，须前移）；`REVIEW_REQUIRED` 同 `refund_id` 返回**同一冲突且不重探、不新增写入**；Tx2 必须补 `state` CAS（现状 `:590-594` 无守卫，重复探测可重复 outbox/release）；re-arm 把 `result` 置回 `PENDING_REMOTE`（§5、§8）。
6. **可观测性增量**：AC-72 后端段需 `internal/platform/metrics` 新 gauge + `order/worker.go` 既有 tick 刷新（**不新增 worker**）；**告警规则因 Plan `deploy/**` forbidden 不在本 L2**（`REVIEW-REQUIRED-ALERTS-01`，ops）；用户可见文案/人工入口归 `REVIEW-REQUIRED-ENTRY-01`（D-33）。Plan §allowed_paths 的“metrics”需 Lead 显式确认共享包。
7. **R4 delta 影响（B1/B2/B3）**：B1 普通售后无 `refund_applications` → 不新建行；镜像由 **AfterSales**（allowed，R5-B1）写 `after_sale_refund_tasks.last_error` canonical、任务保持 `PENDING`、**8 次后不得自动 `FAILED_PERMANENT`**；B2 零元不进入 S20/计数；B3 审计按 D-32 四载体（re-arm **先审计后清除**），`REVIEW-AUDIT-01` DEFERRED 且不阻塞。
7a. **R5 delta**：allowed/forbidden 与 Plan `d2149c0` 精确一致（AfterSales/platform metrics/gateway 文案/cmd；deploy/frontend/migrations/docs/api-gen forbidden）；`last_error` unified canonical；AC-72 告警规则归 `REVIEW-REQUIRED-ALERTS-01`；Plan 已对齐（R6-M1）。
8. **结论**：**技术影响已定位，无未解释的资损路径** → 本分析 **READY for QC round 6**；**实现前 BLOCKED**：PO v1.11 仍 `in-review`（未 approved），ADR 须 QC round 6 + 人类 `ADR_APPROVED`（L2 C-4）后才能开工；PO D-30..D-33/§11 ≠ 实现批准。

---

## 1. 事实基线（只读证据）

| # | 事实 | 证据 |
|---|---|---|
| S1 | `order_refund_admissions`：`result/state VARCHAR(32) NOT NULL DEFAULT 'PENDING_REMOTE'`、`version BIGINT`、**无 CHECK** | `migrations/order/0016_settlement_admission.sql:7-19` |
| S2 | `order_refund_commitments`：`(order_no,group_no,refund_id) UNIQUE`、`state(RESERVED/SETTLED/RELEASED)` | `0016:23-35` |
| S3 | `refund_applications.admission_result VARCHAR(24) NOT NULL DEFAULT ''`；`REVIEW_REQUIRED`=15 字符可容纳；`REFUND_ADMISSION_CONFLICT`=25 字符**不可** | `migrations/order/0017_refund_batch_ledger.sql:45-81`（`:56`） |
| S4 | `refund_batches.state` CHECK 已含 `MANUAL_HOLD`；`last_error` 列存在，但 TCC 只 `s.log.Error`，不写列 | `0017:20-33`；`tcc_admission.go:97-110` |
| S5 | 正金额 `CONSUMED` 的 Tx1 在远程探测前已写 admission(`result=POST,state=PENDING_REMOTE`)、commitment、outbox | `refund_admission.go:253-283` |
| S6 | TCC Tx1 写 `result=PENDING_REMOTE,state=PENDING_REMOTE`（既有偏差） | `tcc_settlement.go:844-852` |
| S7 | Tx2 更新 admission **无 `state`/version 守卫**：`UPDATE ... WHERE refund_id=?` | `refund_admission.go:590-594` |
| S8 | 部分金额校验只对 `permit != CONSUMED` 生效；`CONSUMED` 一律进入探测 | `refund_admission.go:249-252`、`:566-608` |
| S9 | `CancelIfCreated` 无行 → 返回 `CANCELLED`；`CREATED→CANCELLED` 真实改行 | `cancel_if_created.go:50-55`、`:70-82` |
| S10 | `order.refund_admitted` consumer 只处理 `result=PRE_SHIP_REVOKED`，其余 no-op | `admission_consumer.go:43-55` |
| S11 | Order 组投影由 `shipment.shipped/received` 推进到 `SHIPPED/RECEIVED`，且“永不降级” | `order/consumer.go:59-62`、`:89-101`；`fulfillment_groups.go:59-99` |
| S12 | TCC 重试顺序：`applyTccCompensation`（内含 `ensureTccRemoteAdmissionTx`）**先于** `admitTccBatch` | `tcc.go:346-361`；`tcc_settlement.go:717` |
| S13 | `ensureTccRemoteAdmissionTx` 的 legal-result switch 只接受 `PENDING/PRE/POST`，其余 `Failed` | `tcc_settlement.go:844-876` |
| S14 | `ensureTccApplicationTx` 的 `admission_result` 单调检查只接受 `""/PRE/POST` 或与本次一致 | `tcc_settlement.go:832-834` |
| S15 | `admitTccApplication` 要求 `AdmitRefund` 返回的 result ∈ `{PRE,POST}`，否则 `Failed` | `tcc_admission.go:151-168` |
| S16 | AfterSales 重试同 `refund_id`；**现有代码** `maxTaskAttempts=8` 后 `FAILED_PERMANENT` 并写 `last_error`（REVIEW 路径必须改为 8 次后仍 `PENDING`，§8.5） | `aftersales/workers.go:399-405`、`:724-748`；`aftersales/service.go:58` |
| S17 | 迁移台账冻结 0016/0017；runner 拒绝 `FAILED/RUNNING` 重放；`ADD CHECK` = COPY + 独占 MDL | `migrations/order/0023_r04_checks_v2.sql:1-12`、`:44-47`；`docs/ops.md:238-275,340-352` |
| S18 | proto `CancelIfCreatedReply.status` 为 `string`，当前 4 值；无新字段/枚举 | `api/proto/mall/v1/mall.proto:1095-1098` |
| S19 | ADR §5.2.2：`CONSUMED` “提交后调用 `CancelIfCreated`（不依赖事务外预读运单状态）”；§5.3 deadline 1s | `0007:573`、`:622` |
| S20 | ADR §5.2.1 的已离仓判定**包含 Order 组状态** `SHIPPED/RECEIVED` | `0007:556` |
| S21 | Order 服务已有既有 worker tick（`RunExpiry`，默认 1 分钟），可挂只读 gauge 刷新 | `internal/services/order/worker.go:11-46`；`aftersales/workers.go:818-834` 同形 |
| S22 | 共享 metrics 包已有 Prometheus gauge/counter 注册模式 | `internal/platform/metrics/metrics.go:1-110` |
| S23 | BFF/frontend/gateway 未引用 `REFUND_ADMISSION_CONFLICT`；用户可见文案无现成投影 | 全仓库 grep（空） |

---

## 2. schema 影响（结论：无迁移）

### 2.1 与 PO D-30 的对齐

| 项 | PO v1.11 决定 | 技术落地 | 迁移 |
|---|---|---|---|
| Order 裁决 | `result=REVIEW_REQUIRED`、`state=PENDING_REMOTE` | 既有 `VARCHAR(32)`；无 CHECK | **无** |
| 发起方镜像（分路径） | 有 `refund_applications` 行（TCC 组级）→ `admission_result=REVIEW_REQUIRED`、`state=PENDING`；普通售后（无该行）→ `after_sale_refund_tasks` 保持 `PENDING` + `last_error`（reason/ticket（`RVW:<refund_id>`）），**不得新建行** | `VARCHAR(24)` 容纳 15 字符；task `last_error VARCHAR(512)` 既有 | **无** |
| TCC 批次 | `state=MANUAL_HOLD` | 0017 CHECK 已包含 | **无** |
| 错误码 | `REFUND_ADMISSION_CONFLICT`（不新增） | `apperr.Failed` → `FAILED_PRECONDITION` | **无** |
| 原因字段 | `reason` 稳定：`CONSUMED_WITHOUT_SHIPMENT`/`PARTIAL_AMOUNT_UNCLASSIFIABLE`/`LEGACY_SHIPMENT_UNKNOWN` | `WithDetails`/日志/`refund_batches.last_error`（列已存在） | **无** |
| 复核审计（D-32） | 不要求行级历史/独立表 | ①admission 行不删除 + `version`/`created_at`/`updated_at`；②镜像 `last_error`（TCC batch 1024；售后 task 512）；③结构化日志；④ops 工单 | **无**（`REVIEW-AUDIT-01` 仅行级历史，DEFERRED） |

### 2.2 为什么 0.3 的 `result=REFUND_ADMISSION_CONFLICT` 必须改

- `refund_applications.admission_result` 是 `VARCHAR(24)`（S3）；`REFUND_ADMISSION_CONFLICT` 为 25 字符 → 严格模式下写入报错 1406/截断风险。
- PO D-30 因而选择独立裁决词 `REVIEW_REQUIRED`（15 字符），并明确“持久值不得用错误码字面量”（`0002:845`）。这同时把**错误码**与**持久裁决**解耦：对外仍 `REFUND_ADMISSION_CONFLICT`，对内是 `REVIEW_REQUIRED`。
- 若有人改回 25 字符 token：必须 `0024` 扩宽 `refund_applications.admission_result` 到 `VARCHAR(32)`，并重走台账/维护窗；本 change **不做**。

### 2.3 CHECK / 注释 / 默认值 / 回填 / 回滚

| 变更 | 是否需要 | 说明 |
|---|---|---|
| `order_refund_admissions.result/state` CHECK | 否 | `ADD CHECK` = COPY + 独占 MDL（S17）；另立 `ADMISSION-RESULT-CHECK-01`，预检 + `_v2` 加后删 |
| `refund_applications.admission_result` CHECK | 否 | 同上；且 CHECK 需枚举 `PRE/POST/PENDING_REMOTE/REVIEW_REQUIRED/''`，风险大于收益 |
| 列注释/默认值 | 否 | MySQL 列无 COMMENT；`0016` 头注释与 ADR `0007:1130` 是文档层；默认保持 `PENDING_REMOTE` |
| 回填 | 否 | `REVIEW_REQUIRED` 只对新写生效；既有 PRE/POST/PENDING_REMOTE 行语义不变 |
| 回滚（down） | 否 | 无 down migration（ADR §10）；旧代码读到 `REVIEW_REQUIRED` 的行为见 §7.2，**禁止无审计回滚** |

### 2.4 复核审计留存（R4-B3 / PO D-32）——不新增表/迁移

| 载体 | 写入 | 保留 | 证据 |
|---|---|---|---|
| `order_refund_admissions` 行 | 进入 REVIEW 时 `result/state/version`；re-arm 时 `version+1` | **不删除**；`created_at`/`updated_at`/`version` | 行查询；O20 |
| 发起方镜像 `last_error` | **canonical** `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>`。TCC：Order Tx2 写 `refund_batches.last_error`（`ticket=RVW:<refund_id>` 系统派生、**必填非空、`RVW:` 前缀**）。售后：**AfterSales** 写 `after_sale_refund_tasks.last_error`（同格式；任务 `PENDING`；8 次后不得自动 `FAILED_PERMANENT`） | **不得以清空该字段作为处置**；后续错误可覆盖“最新”；售后 ≤512、TCC ≤1024；超长保留前缀+截断 `RVW:<refund_id>`+` truncated=1` | `LIKE 'REVIEW_REQUIRED reason=%'`；O20/O22/O23 |
| 结构化日志 | 进入/每次 re-arm/每次处置 | 平台日志策略 | O20 |
| ops 工单 | re-arm/放弃/处置 | 外部工单系统 | 对账交叉 |

- **re-arm 强制顺序**：工单 → 镜像 `last_error` → 日志 → CAS 清 `result`；**禁止先清后补**。
- **`REVIEW-AUDIT-01`（行级历史/独立表）DEFERRED 且不阻塞**；若 QC/PO 坚持行级历史（尤其普通售后的证据/处置人写入 Order 行）→ 需新表/新列，**`[ARCHITECTURAL_FLAW]`/回 Plan**，不在本 L2 夹带。
- **AfterSales 跨库写**：普通售后镜像由 AfterSales 既有错误路径写；Order 不跨库写；ops 以 `RVW:<refund_id>` 建单/映射（不回写）；Order 不跨库写（§10 D-9）。

---

## 3. TCC 影响（最大增量）

### 3.1 现状与 0.3 失败点（R3-B1 复核）

```text
TccCancelMarkPaid（tcc.go:346-361）
  ├─ applyTccCompensation
  │    └─ writeTccBatchGroupsTx（tcc_settlement.go:717）
  │         ├─ ensureTccApplicationTx（:788-840）——admission_result 单调检查
  │         └─ ensureTccRemoteAdmissionTx（:844-876）
  │              switch result：PENDING|PRE|POST → nil；其它 → Failed("not a legal value")
  └─ admitTccBatch（:356）
```

0.3 的 `result=REFUND_ADMISSION_CONFLICT` 在 `ensureTccRemoteAdmissionTx` 的 `default` 直接失败，永远到不了 `AdmitRefund` 的短路；且 `applyTccCompensation` 先于 `admitTccBatch`，所以“停 ADMITTING 等人工”的假设不成立。**v1.11 明确要求把 `REVIEW_REQUIRED` 当合法持久裁决读回（AC-71 `0002:474`）。**

### 3.2 需要改的 TCC 面（PO §11 授权**范围**；实现仍需 L2 C-4 人类批准，≠ merge 授权）

| 文件/函数 | 变更 | 目的 |
|---|---|---|
| `tcc_settlement.go` `ensureTccRemoteAdmissionTx` | legal-result 增加 `refundReviewRequired`；`REVIEW_REQUIRED` 返回 `apperr.Failed(REFUND_ADMISSION_CONFLICT…)`（带 `batch_id/refund_id/group_no/reason`），**不覆盖行、不回滚已提交事务** | AC-71 不得判非法/不得早失败 |
| `tcc_settlement.go` `ensureTccApplicationTx` | 单调检查接受 `REVIEW_REQUIRED`；已复核的 application 不因重算 `PENDING_REMOTE` 判冲突 | 重试可幂等读回 |
| `tcc_settlement.go` `writeTccBatchGroupsTx` | 入口短路：批次已是 `MANUAL_HOLD` → 返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`，**零写入** | AC-71 自动续跑不改状态 |
| `tcc_admission.go` `admitTccBatch`/`tccBatchManualError` | `MANUAL_HOLD` 返回 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`；循环后重读 batch（Tx2 可能已置 `MANUAL_HOLD`），不得走 `markTccBatchAdmitted` 的 generic failure；写可查询原因 | AC-71 |
| `tcc_admission.go` `loadTccApplications`/`admitTccApplication` | `admission_result=REVIEW_REQUIRED` 视为“自动终止”，不当作可重试的 `PENDING` | 不倒灌重探 |
| `refund_admission.go` Tx2 | `source=tcc` 且 `batch_id` 非空时，同一事务 `UPDATE refund_batches SET state='MANUAL_HOLD', last_error='REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>' WHERE batch_id=? AND state IN ('FROZEN','ADMITTING')`；0 行只记录不报错 | 裁决落库即持批，ops 可查；canonical |
| `refund_batches.state` | 复用既有 `MANUAL_HOLD`（0017 CHECK 已含） | 无迁移 |

### 3.3 batch/application 状态机与资金

| 关注点 | 结论 |
|---|---|
| batch | `FROZEN→ADMITTING→(REVIEW_REQUIRED)→MANUAL_HOLD`；禁止 `DONE/ADMITTED/PAYING/FAILED_PERMANENT` 自动迁移 |
| application | 保持 `PENDING`，`admission_result=REVIEW_REQUIRED`；不进入 `PROCESSING/DONE`；不自动 `FAILED_PERMANENT` |
| 打款 | `payTccBatch` 只在 `admitTccBatch` 无失败时执行（`tcc.go:356-361`）→ 复核 → **0 退款命令**（INV-43） |
| 组隔离 | 单组复核只阻塞**本批**与同订单其他组打款；其他订单/`payment_no` 不受影响（AC-71 `0002:477`） |
| 自动续跑 | `MANUAL_HOLD` → `TCC_BATCH_REQUIRES_MANUAL_HANDLING`，0 打款、0 状态变化 |
| re-arm | ops 审计后把 batch `MANUAL_HOLD→ADMITTING` + 组裁决 `REVIEW_REQUIRED→PENDING_REMOTE`，同 `batch_id`/`refund_id` 续跑；不新建退款标识（§8.3） |

---

## 4. 部分金额 + `CancelIfCreated`（R3-B2）

### 4.1 问题

现状 `CONSUMED` 组无论金额都走 `finishRemoteAdmission` → `CancelIfCreated`；`CREATED` 行会被真实取消（S9），随后才判 `PARTIAL`，违反 AC-50 `0002:217`“不得改动许可/运单/打款”。0.3 已无预读，无法在调用前区分 `CREATED` 与 `SHIPPED`。

### 4.2 方案对比（PO D-31 已裁决）

| 选项 | 做法 | AC-50:217 | AC-50:226 | §5.2.2 无预读 | INV-52 不用取消副作用 | 判定 |
|---|---|---|---|---|---|---|
| (i-a) 部分一律人工/整组 | 直接 `PARTIAL` 或 `REVIEW_REQUIRED` | ✅ | ❌ 已离仓部分被误拒 | ✅ | ✅ | PO 否决（误伤合法 post-ship 部分） |
| **(i-b) PO D-31（采用）** | 只用 Order 自有事实三分：投影 SHIPPED/RECEIVED→POST；投影未离仓→PARTIAL（写前拒绝）；不可判定→`REVIEW_REQUIRED` | ✅ | ✅ | ✅ | ✅ | **采用** |
| (ii) 新增只读 S6 查询 | `GetShipmentByOrder`/`ListShipments` 预读 | ✅ | ✅ | ❌ 与 §5.2.2/route A 冲突 | ✅ | PO D-31 明确本版不要求；若引入须修 ADR §5.2.2 + 人类批准 |
| (iii) 先取消再分类 | 0.3 现状 | ❌ | ✅ | ✅ | ❌ | PO D-31 明确否决（INV-52） |

### 4.3 采用方案的精确影响

| 文件/行为 | 变更 | 说明 |
|---|---|---|
| `refund_admission.go` Tx1 部分金额 guard（`:249-252`） | 扩到所有 permit：`amount < groupRemaining` 时按 §3.4 三分法；`PARTIAL` 在 INSERT 前返回（rollback） | **零 admission/commitment/outbox** |
| 组投影读取 | `order_fulfillment_groups.status WHERE order_no=? AND group_no=?`；无组行时取 `orders.status` | Order 自有表，Tx1 内，非跨服务；ADR §5.2.1 已承认组状态为离仓事实（S20） |
| 已离仓部分 | Tx1 写 `result=POST,state=POST` + 终态 outbox + commitment；**不调 `CancelIfCreated`** | 与 `recordTccPostAdmission`（`tcc_admission.go:214`）同形 |
| 不可判定部分 | Tx2/同事务写 `result=REVIEW_REQUIRED,state=PENDING_REMOTE`；`reason=PARTIAL_AMOUNT_UNCLASSIFIABLE` | 需在 Tx1 已有占位后写；或先判后写（见 4.4） |
| 测试 | O4a/O4b/O4c/O4d（addendum §10） | 断言 `cancelCalls==0`、0 写入或指定裁决 |

### 4.4 实现顺序细节（避免“先占位后拒绝”留债）

PO AC-70 `0002:462-463` 要求“未离仓部分在留下承诺/占位之前拒绝”。因此判定必须发生在 **Tx1 插入 admission/commitment 之前**：
- `PARTIAL`：Tx1 直接 rollback，零写入；
- `REVIEW_REQUIRED`：同一 Tx1 写 `result=REVIEW_REQUIRED,state=PENDING_REMOTE` + **1 条 `RESERVED` commitment**（无终态 outbox）——冻结该额度，防同一额度被第二个 `refund_id` 重复承诺/重复退款，并满足 AC-62 “REVIEW 期间承诺保持 RESERVED”。**不得先写 POST/PRE 再改 REVIEW**。
- `POST`（已离仓）：Tx1 正常写 POST + commitment + 终态 outbox；**整组金额若 Order 组投影已 `SHIPPED/RECEIVED`，直接 POST、不调用 `CancelIfCreated`**（PO AC-50 `0002:236`）。

### 4.5 残余风险

1. **投影滞后**（真实已离仓、Order 组投影仍 `ALLOCATED`）→ `PARTIAL`（fail-closed）；用户改整组（整组探测 `ALREADY_SHIPPED` → POST，全额退）或等投影收敛重试。
2. **投影缺失 + `orders.status ∈ {SHIPPED,COMPLETED}`** → `REVIEW_REQUIRED`（AC-70 `0002:464`）；不默认 PRE/POST；**冻结该笔额度**（1 条 RESERVED commitment），用户改整组需先按 AC-62 处置旧承诺，防双花。
3. **legacy 无组行 + 未离仓** → `NONE→REVOKED→PRE`（AC-70 `0002:468`）；已离仓 → POST；不可判定 → REVIEW。

---

## 5. 幂等 / 重放 / 崩溃

### 5.1 同 `refund_id`

| 既有行状态 | 行为 | 依据 |
|---|---|---|
| `state=result ∈ {PRE,POST}` | 返回已持久化结果，不写 | AC-50 `0002:240` |
| `result=REVIEW_REQUIRED` + `state=PENDING_REMOTE` | **返回同一 `REFUND_ADMISSION_CONFLICT`**，不探测、不读 Payment、不写 | AC-69 `0002:451` |
| `state=PENDING_REMOTE` 且无复核标记 | 继续两阶段/重探 | 现状 `:179-185` |
| 未知状态 / 非法组合 | fail-closed `Failed`，不得当成功 | §6 A8 |

### 5.2 `PENDING_REMOTE` 与 `REVIEW_REQUIRED` 都不是成功

- `PENDING_REMOTE` = 未决；`REVIEW_REQUIRED` = 自动路径终止、业务未终的人工持有态（PO §5.10 `0002:652-670`）。
- 成功谓词唯一：`state=result ∈ {PRE,POST}`（A2）。
- 复核一旦写入，普通 replay 不再探测；唯一前进路径：人工 re-arm（同 `refund_id`）或 AC-62 释放 + 新 `refund_id`。

### 5.3 崩溃窗口

| 窗口 | 行为 | 恢复 owner |
|---|---|---|
| W1 Tx1 后崩溃 | 重放 PENDING → 重探 | 发起方 task + Order |
| W2 探测得“无运单”后、Tx2 前崩溃 | Tx1 已写；重放 PENDING → 再探测 → REVIEW | 发起方 task |
| W3 探测 `CANCELLED` 后、Tx2 前崩溃 | 重放探测（幂等）→ PRE | 发起方 task |
| W4 Ship/取消竞态 | Ship 胜 → POST；取消胜 → PRE | Fulfillment 行 CAS |
| W5 同 `refund_id` 并发 | 订单行锁 + Tx2 `state` CAS；败者读回 | Order |
| W6 REVIEW 写入后、返回前崩溃 | 重放读 REVIEW → 同一冲突、不探测 | Order |
| W7 零元远程取消后、单事务前崩溃 | 重放重探（零元无 Tx1） | 发起方 task |
| W8 PENDING 永久悬挂 | 未关闭 + 告警；AC-61 人工重放 | ops |
| W9 消息重复/乱序 + 迟到 `order.paid` | consumer 只处理终态 PRE；无行 no-op；真实 placeholder 允许；REVIEW 后不自动 PRE | Fulfillment/ops |
| W10 部分金额 | 三分法，不探测、不改运单 | Order |
| W11 人工 re-arm | result→PENDING 重评；TCC batch MANUAL_HOLD→ADMITTING | ops |
| W12 TCC 重试撞 REVIEW | batch MANUAL_HOLD；自动续跑稳定错误；0 打款 | Order + ops |

### 5.4 outbox / DLQ

- Tx1 占位保留；REVIEW **不新增终态事件**；consumer 只认 `result=PRE`（S10）。
- `outbox_events.dead_lettered_at`/`outbox_dead_letters`（`internal/platform/outbox/outbox.go:266,324-337`）；REVIEW 不依赖 outbox 收敛。

### 5.5 metrics / 告警 / SLA（AC-72）

- 新增 gauge：`mall_refund_review_backlog`、`mall_refund_review_oldest_seconds`（`internal/platform/metrics`，additive）。
- 刷新：`order/worker.go` 既有 `RunExpiry` tick 内只读查询（S21），**不新增 worker**：
  `SELECT COUNT(*), COALESCE(TIMESTAMPDIFF(SECOND, MIN(updated_at), NOW(3)),0) FROM order_refund_admissions WHERE result='REVIEW_REQUIRED' AND state='PENDING_REMOTE'`
- 告警：规则内容为 `>0`/`oldest>0`（`for` 按 ops），但 **`deploy/**` 在 Plan 中 forbidden** → 落地归 `REVIEW-REQUIRED-ALERTS-01`（ops）；本 L2 不交付规则文件；`reason` 分布走日志/查询。
- **AC-72 分段（D-33/R5-M3）**：本 L2 验收 metrics/查询/`reason`/SLA/对账；**告警规则**归 `REVIEW-REQUIRED-ALERTS-01`（ops，deploy forbidden）；**用户可见完整入口**归 `REVIEW-REQUIRED-ENTRY-01`（gateway 仅文案）；**AC-72 不得整体 AC-VERIFIED**，`ac-coverage` 分段。
- **零元不计数**：gauge 只统计 `order_refund_admissions WHERE result='REVIEW_REQUIRED' AND state='PENDING_REMOTE'`（零元不写 admission），零元冲突单列。
- 用户文案“退款处理中（需人工核对）”与完整人工入口属 `REVIEW-REQUIRED-ENTRY-01`/frontend/ops；本 L2 仅 gateway 文案映射（§6.4）。

---

## 6. 影响矩阵（文件级）

### 6.1 必须改

| 路径 | 动作 | 说明 |
|---|---|---|
| `api/proto/mall/v1/mall.proto` | 仅注释 | `CancelIfCreatedReply.status` 增加新值语义（`mall.proto:1095-1098`）；无字段/RPC/枚举 |
| `api/gen/mall/v1/mall.pb.go`、`mall_grpc.pb.go` | `make proto` 生成 | 禁止手改；`make proto-guard`；diff 只允许注释行；Plan 未显式列生成物 → Lead 实施 Brief 确认 |
| `internal/services/fulfillment/cancel_if_created.go` | `sql.ErrNoRows` 返回新值 | 不 commit、不建行 |
| `internal/services/order/refund_admission.go` | existing 前移；REVIEW 分支；Tx2 `state` CAS；部分金额三分；TCC 批次持批；零元分支 | 核心 |
| `internal/services/order/tcc_settlement.go` | legal 裁决；app 单调；MANUAL_HOLD 短路 | PO §11/AC-71 授权**范围**；实现仍需人类 `ADR_APPROVED` + Plan |
| `internal/services/order/tcc_admission.go` | MANUAL_HOLD 错误/置批；读回 REVIEW | 同上 |
| `internal/platform/metrics/metrics.go` | 2 个 gauge | AC-72 计数/最老 |
| `internal/services/order/worker.go` | 既有 tick 内只读刷新 | 不新增 worker |
| `internal/services/order/{refund_batch*.go,refund_consumer.go,consumer.go}` | **仅当 REVIEW 对账/消费必需** | 实施 Brief 逐条论证；不得改既有语义 |
| `internal/services/aftersales/**`（仅镜像/状态） | canonical `after_sale_refund_tasks.last_error`；任务保持 `PENDING`；8 次后不得自动 `FAILED_PERMANENT` | R5-B1；不改结算/回库/表结构 |
| `internal/services/gateway/**` | 仅显示文案映射 | AC-72 用户可见仅文案；完整入口归 `REVIEW-REQUIRED-ENTRY-01` |
| `cmd/*` | 仅 wiring | 注册 gauge/worker tick（既有） |
| ~~`deploy/compose/observability/alerts.yml`~~ | **不在本 L2**（Plan forbidden `deploy/**`） | 告警规则归 `REVIEW-REQUIRED-ALERTS-01`（ops，2026-12-31）；本 L2 只交付 gauge/查询/SLA 文档 |
| `internal/services/fulfillment/cancel_if_created_test.go` | 测试 F1/F1b/F2–F6 | |
| `internal/services/fulfillment/admission_consumer_nats_integration_test.go` | 测试 F7/F7b/O14 | 生产 consumer 语义不变 |
| `internal/services/aftersales/*_test.go` | 测试 O19/O22（镜像/8 次不 FAILED） | 真实 MySQL |
| `internal/services/order/refund_admission_integration_test.go` | 测试 O1–O11、O16、O18、O20–O23 | 真实 MySQL |
| `internal/services/order/tcc_admission_integration_test.go` | 测试 O1b/O12/O13b，入口 `TccCancelMarkPaid` | 覆盖 MANUAL_HOLD |

### 6.2 只读 / 不改

**AfterSales 除 §6.1 的镜像/PENDING/重试上限外的任何文件/逻辑**、`migrations/order/0016/0017`（frozen；本 change 无迁移）、`docs/contracts.md`、`internal/services/order/ports.go`、`settlement.go`、`fulfillment/consumer.go`、`admission_consumer.go`（生产）、`internal/services/{inventory,payment}/**`、`internal/services/order/tcc.go`、`tcc_payment.go`、`tcc_precheck.go`、`tcc_reconcile.go`、`frontend/**`、`deploy/**`、`docs/specs/**`、`docs/collab/**`、`docs/lead/**`、`docs/implementation/**`、ADR Rev 2.3 正文。

### 6.3 迁移 = 无

`REVIEW_REQUIRED` 两列均容纳；无 `0024`、无回填、无 CHECK。若未来改用 25 字符 token 或加 CHECK，另立 L2（`ADMISSION-RESULT-CHECK-01`）。

### 6.4 API / gateway 文案 / frontend / ops 入口

- 无新 RPC/字段/事件；错误码不新增。
- **用户可见文案**（AC-72）与**人工 re-arm 入口**（AC-61/AC-69）当前无现成 API：本 L2 只给 ops SQL runbook + 查询 + 告警；AC-72 按 **D-33** 分后端段（本 L2 可验收）与入口段（`REVIEW-REQUIRED-ENTRY-01`，DEFERRED，owner Lead/BFF/ops）。交付前沿用“退款处理中”；**AC-72 不得整体 AC-VERIFIED**，`ac-coverage.tsv` 分段登记。
- **gateway**：仅显示文案映射；测试为 **gateway 文案映射测试**（非 BFF 契约测试，R7-M3）。完整用户入口/UI 归 `REVIEW-REQUIRED-ENTRY-01`（DEFERRED）。
- `docs/ops.md` runbook 由 ops 维护；本 ADR 提供查询/工单字段/告警键。

### 6.5 AC / INV / 证据矩阵

| 对象 | 关系 |
|---|---|
| AC-50 `0002:222-245`（v1.11） | 直接修复整组/部分金额；`REVIEW_REQUIRED` 只用于不可判定 |
| AC-69..72 `0002:451-499` | 新增复核决定核心；本 L2 实现后端；AC-69 审计按 D-32；AC-72 按 D-33 分段（metrics/SLA/对账本 L2；告警 `REVIEW-REQUIRED-ALERTS-01`；用户可见/入口 `REVIEW-REQUIRED-ENTRY-01`） |
| AC-69 普通售后镜像 | B1 | `order_refund_admissions` 唯一载体；`after_sale_refund_tasks` 镜像；**不新建 `refund_applications` 行**（O19） |
| AC-69/72 审计 | B3 | D-32 四载体；`REVIEW-AUDIT-01` DEFERRED 且不阻塞（§2.4、O20） |
| AC-27A `0001:615-645` | **不改文本**；REVIEW/PENDING 不关闭；读模型归 R0.3-R1c |
| AC-27 `0001:592-613` | 收口前提；`AC-50-FIX-01` 未关闭前不得 AC-VERIFIED |
| AC-62 `0002:326` | REVIEW 不自动释放；AC-62/D-24 唯一例外 |
| AC-48/51/65/67/68 | 非回归；TCC MANUAL_HOLD 不早失败；零元不持久化 review |
| INV | INV-36/42/43/44/45/50 + **INV-51/52**；INV-01/AC-25 不得触碰 |
| 证据矩阵 | `ac-coverage.tsv` 归 Reviewer；本文件不更新 |

### 6.6 边界：AC-27A / T5 / T2 / T3 / T4.4

| 项 | 边界 |
|---|---|
| AC-27A | 前提保证，不改文本；不实现读模型；不执行存量审计 |
| T5 | 只给测试矩阵与断言；AC-VERIFIED/Release 由 Reviewer/人类 |
| T2 | 不涉及（历史支付解析/回填） |
| T3 | 不实现 release writer/行组成；REVIEW 释放仍走 AC-62/D-24 |
| T4.4 | 不涉及（`order.settled` shadow） |

---

## 7. 兼容性与回滚

### 7.1 行与值

- 旧行 PRE/POST/PENDING_REMOTE 语义不变；`REVIEW_REQUIRED` 只对新写生效。
- 旧 Fulfillment + 新 Order：旧 Fulfillment 对未知 status 仍返回 `CANCELLED`——**注意**：新 Order 不再把 `CANCELLED` 当无运单（因为新 Fulfillment 才返回新值），但若 Fulfillment 未升级，无行仍返回 `CANCELLED` → Order 会写 PRE（缺陷保持）。因此必须**同一原子提交**或先 Fulfillment 后 Order（addendum §2.4）。
- 新 Fulfillment + 旧 Order：旧 Order `default` → `Unavailable`（不写 PRE），fail-closed。

### 7.2 回滚（PO §12 `0002:894` 要求）

- **审计先行**：回滚/隔离前必须确认不存在未处置 `REVIEW_REQUIRED` 行，且 re-arm 的审计（工单/镜像/日志）已写入；**不得**以清空 `last_error`/删除 admission 行作为“回滚”。

- PO：旧代码读到 `REVIEW_REQUIRED` **必须 fail-closed**；当前旧代码 `state=PENDING_REMOTE` → `finishRemoteAdmission` 会**重探**，可能把复核洗成 PRE/POST → **旧代码不是 fail-closed**。
- 结论：**禁止把旧 Order 代码回滚到含 `REVIEW_REQUIRED` 行的库**；如必须回滚，先按 §8.3 反向 re-arm/隔离并审计（PO 同口径：回滚前先审计是否存在 `REVIEW_REQUIRED` 行）。本 change **前向修复**。

### 7.3 发布顺序

- 建议同一原子行为提交（`CONVENTIONS.md:241`）；分批时先 Fulfillment、后 Order；Order 的 REVIEW 写入必须先于 Fulfillment 未知值重试逻辑上线（否则旧 Order 会把新值当 `Unavailable`，仍 fail-closed，可接受）。

---

## 8. 风险与缓解

| # | 风险 | 影响 | 缓解 |
|---|---|---|---|
| A1 | 旧代码/旧路径把 `REVIEW_REQUIRED` 当 `PENDING_REMOTE` 重探 → 洗成 PRE | 资损/误关闭 | 新代码 existing 先行 + 复核短路；O2/O2b；禁止无审计回滚 |
| A2 | AfterSales 收到 nil error 提前打款 | 资损 | `AdmitRefund` 对非终态必须返回错误；O2/O2c |
| A3 | TCC 早失败/非法值 | TCC 不可执行 | 改 switch + app 单调 + MANUAL_HOLD；O1b 走 `TccCancelMarkPaid` |
| A4 | 部分金额先取消运单 | 违 AC-50:217/INV-52 | 三分法、不探测；O4b 断言 `cancelCalls==0` |
| A5 | 未离仓部分留下承诺/占位 | 额度被占、用户改整组受阻 | 判定先于 INSERT；O4b 断言 0 admission/commitment |
| A6 | Tx2 无守卫重复 outbox/release | 重复事件/释放 | A8 `state` CAS + 0 行语义；O11b/O11c |
| A7 | 复核项无 SLA/无发现 | 长期悬挂 | gauge + 告警 + 查询 + ops runbook；`REVIEW-REQUIRED-ENTRY-01` 登记 UI 入口缺口 |
| A8 | `refund_applications` 无法容纳错误码字面量 | 写失败/脏数据 | D-30 用 15 字符裁决；O1b 断言 application 行 |
| A9 | TCC MANUAL_HOLD 无 re-arm 路径 | 批永久持有 | ops runbook（SQL + 审计）；若组织禁止 DB 写 → 另立 RPC L2 |
| A10 | 存量错误 PRE | AC-27A 误关闭 | §8 审计；审计未出前不得关闭 |
| A11 | 普通售后被要求写 `refund_applications`（B1） | 双写/新行 | PO B1 分路径；ADR §3.3/§7.4；O19；不新建行 |
| A12 | 零元被纳入 S20/计数（B2） | 伪造 PENDING/DONE | PO B2；O21；gauge 只查 admission；零元不写 |
| A13 | 原复核记录随 re-arm 丢失（B3） | 审计缺口 | D-32/§8.4：先写工单/镜像/日志再清；O20；行级历史→ `[ARCHITECTURAL_FLAW]`/回 Plan |
| A14 | 以 PO §11/D-30..33 代替人类批准（M3） | 越权开工 | §15.2 #14；人类批准门；Reviewer 核对 |
| A15 | AfterSales 8 次后自动 `FAILED_PERMANENT` 或非 canonical `last_error`（R5-B1/M4） | 违反 AC-69/72；镜像不可解析 | AfterSales allowed 仅镜像/PENDING/重试；§8.5；O22/O23；Reviewer FAIL |

---

## 9. 【假设与不确定性】

1. **[不确定] SPEC-0002 v1.11 仍 `in-review`**（工作树未 commit，`0002:20-23`）：本分析按 2026-11-26 读取的 v1.11 文本；PO/QC 若改名/改表示（例如改回 `state` 或改 token），本文件与 Addendum 需回 Architect 重算（§10 D-1）。
2. **[假设] Order 组投影是部分金额的合格离仓事实**：ADR §5.2.1 `0007:556` + PO v1.11 `0002:98`/AC-70 已采用；若 QC 不承认，退回 (ii) 或 PO 重裁。
3. **普通售后无 `refund_applications` 行（PO B1 已裁决，不是假设）**：唯一权威载体 `order_refund_admissions`；有 `refund_applications` 行才写该行；普通售后写 `after_sale_refund_tasks.last_error`，**不得新建行**（`0002:90,103,456`）。D-2 关闭。
4. **[假设] `refund_batches.last_error` 可由 Order 在 Tx2/re-arm 写**：列已存在（`0017:28`），无需迁移；`tcc_reconcile.go:340` 只清 `refund_applications.last_error`，不清 batch；re-arm **不得清空** batch 审计文本（O20b）。
5. **[假设] metrics gauge 是 AC-72 的最小实现**：共享包 additive；若 QC 认为“度量”必须走独立报表/BI，另立 L2。**未新增 worker**（挂在既有 tick）。
6. **AC-72 分段（PO D-33/R5-M3）**：本 L2 只做 metrics/查询/SLA/对账；告警规则归 `REVIEW-REQUIRED-ALERTS-01`；完整 UI/入口归 `REVIEW-REQUIRED-ENTRY-01`；**AC-72 不得整体 VERIFIED**。
6a. **审计载体（PO D-32）**：不要求行级历史；admission 行 + 镜像 `last_error` + 日志 + 工单；`REVIEW-AUDIT-01` DEFERRED 且不阻塞。若 QC/PO 反向坚持行级历史 → `[ARCHITECTURAL_FLAW]`/回 Plan。
6b. **普通售后镜像写入方（R5-B1）**：AfterSales **在 allowed 范围内**写 canonical `last_error`、保持 `PENDING`、8 次后不得自动 `FAILED_PERMANENT`；Order 不跨库写；ops 以 `RVW:<refund_id>` 建单/映射（不回写）。D-9 关闭（有界降级不再需要：writer 已存在）。
6d. **canonical `last_error`（R5-M4）**：`REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>`；`ticket` 必填非空、`RVW:` 前缀；`reason=` 必需；售后 ≤512、TCC ≤1024；超长保留前缀+截断 `RVW:<refund_id>`+` truncated=1`；解析 key=value，禁止冒号/自由文本。
6c. **SPEC §11（R5-M2）**：SPEC `0002:884` 已改 **DRAFT 0.5**；ADR/impact Base 已 `d2149c0`；Plan `d2149c0`/C-2/0.5 已由 Lead 修正（R6-M1）。Reviewer 绑定 ADR 0.5/impact 0.3 blob。
7. **[不确定] 仓库外 `CancelIfCreated` 调用方**：全仓库只有 Order 与 Fulfillment consumer（S10）；仓库外需 ops 确认。
8. **[不确定] 生产 `CONSUMED+无运单` 发生率**：正常路径不可达（addendum §0.2）；合成状态用于 fail-closed 验证。
9. **未执行**：本文件未运行测试/迁移/压测；行号是阅读事实。

---

## 10. 剩余待决事项（PO v1.11 已解决大部分）

| # | 事项 | 现状 | 需要 |
|---|---|---|---|
| D-1 | 裁决名/表示 | **已定**：`REVIEW_REQUIRED` / `result` / `state` 不变（D-30） | 关闭；改名需重裁 |
| D-2 | 普通售后是否需 `refund_applications` 镜像行 | **PO B1 裁决：不新建**；唯一载体 `order_refund_admissions`，镜像写 task `last_error` | **关闭** |
| D-3 | SPEC-0001 `0001:645` 注同步 | PO §11 item 4 登记 owner PO/2026-12-31 | 跟踪 |
| D-4 | 零元是否持久化 | PO AC-67/AC-69：**不持久化** | 关闭 |
| D-5 | 部分金额判定 | PO D-31 三分法 | 关闭 |
| D-6 | AC-72 用户可见/人工入口 | **PO D-33：分段**；入口归 `REVIEW-REQUIRED-ENTRY-01` | **关闭（分段）** |
| D-7 | `refund_batches.last_error` 写入的 `change_id` 归属 | 本 L2 的 TCC 范围内 | Lead 在实施 Brief 确认 |
| D-8 | 原复核记录载体 | **PO D-32：不要求行级历史**；四载体；`REVIEW-AUDIT-01` DEFERRED 且不阻塞 | **关闭** |
| D-9 | 普通售后镜像的工单号写入方 | **R5-B1 关闭**：AfterSales 在 allowed 范围写 canonical（`ticket=RVW:<refund_id>`，必填非空）；ops 以该 id 建单/映射；Order 不跨库写 | **关闭** |
| D-10 | 引用对齐（R5-M2/R6-M1/R7-M2） | SPEC `0002:884` 已 0.5；ADR/impact Base `d2149c0`；Plan `d2149c0`/C-2/0.5 已由 Lead 修 | **关闭** |

---

## 11. 明确不做

- 不写代码/测试/迁移/SPEC/台账；不 commit。
- 不新增 RPC/proto 字段/事件/worker/状态字段/分布式锁/配置开关；不新增 `ListShipments`/`GetShipmentByOrder`/事务外预读；**不为审计新增表/列**（D-32 复用既有；行级历史 `REVIEW-AUDIT-01` DEFERRED）。
- 不实现 AC-27A 读模型、存量 PRE 审计执行、T3 release、T2 历史解析、T4.4 shadow、完整 BFF/UI/人工入口（gateway 仅文案）。
- 不修改 SPEC-0001/0002（PO 所有）、ADR Rev 2.3 正文、0016/0017。

---

## 12. 变更记录

| Revision | 日期 | 变更 |
|---|---|---|
| 0.1 | 2026-11-26 | 初稿：schema（state/result 两表示）、TCC 早失败、partial 三方案、幂等/回滚、矩阵 |
| **0.3 + R5/R6 delta** | 2026-11-26 | **R5/R6 对齐**：allowed/forbidden 与 Plan `d2149c0` 精确一致（AfterSales 仅镜像/PENDING/重试；platform metrics；gateway 仅文案；cmd；deploy/frontend/migrations/docs/api-gen forbidden）；AfterSales 8 次后不得自动 `FAILED_PERMANENT`；`last_error` 统一 canonical `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>`（512/1024，`ticket` 必填非空）；R5-M1/M2/M3/M4 处置；D-9 关闭；R6：canonical 改 SPEC 定稿 `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>`（无 arm，`truncated=1`，ticket 必填非空）、§7.3 8 次不自动 FAILED、Plan 已对齐。 |
| **0.3** | 2026-11-26 | **按 QC R4-B1/B2/B3 + M1/M2/M3 同步 ADR 0.5 与 PO v1.11 R4 delta**：B1 分路径镜像（`order_refund_admissions` 唯一载体；普通售后不新建 `refund_applications`，写 task `last_error`）；B2 零元不进入 S20/不持久化/不计入 AC-72 计数；B3 审计 D-32（admission 行 + `version`/时间戳 + 镜像 `last_error` + 日志 + 工单；无新表；`REVIEW-AUDIT-01` DEFERRED 且不阻塞；re-arm 先审计后清除）；M1 删除/改正 ADR 自身 stale 0.3；M2 AC-72 分段（D-33）；M3 PO §11≠人类批准；文件级矩阵/测试范围补 O19/O20/O20b/O21；D-2/D-8 关闭，新增 D-9/D-10 |
| **0.2** | 2026-11-26 | **按 SPEC-0002 v1.11（in-review，D-30/D-31）重算**：持久裁决定为 `REVIEW_REQUIRED`（15 字符，`result` 承载、`state` 不变）→ **schema 影响=0、无 0024**；TCC 改为“接受合法裁决 + `MANUAL_HOLD` + `TCC_BATCH_REQUIRES_MANUAL_HANDLING`”（`tcc_settlement.go`/`tcc_admission.go` 进 `allowed_paths`）；partial 改为 PO 三分法（不可判定→REVIEW）；补 AC-72 metrics/告警/SLA 与“用户入口另立 L2”的边界；回滚按 PO §12 改为“审计后前向修复”；剩余待决 D-2/D-6/D-7 |

**Actor**：`arch-ac50-impact`（System Architect）
**Reviewed-by**：待 QC round 5
**Status**：**DRAFT — 不是 ADR，不是实现依据**
