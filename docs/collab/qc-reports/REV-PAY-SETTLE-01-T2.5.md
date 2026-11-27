# REV-PAY-SETTLE-01-T2.5 — T2.5 / T2.6 / T2.7 实现阶段审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（在途；SPEC-0002 AC-60/AC-63 覆盖范围内） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | `cmd/order-legacy-backfill` + `internal/services/order/legacy_backfill.go`（T2.5）；`internal/services/order/refund_seq.go`（T2.6）；两个集成测试（T2.7）；`docs/lead/PAY-SETTLE-01/t2.5-plan.md`、`tasks.md` T2.5–T2.7。**不审** T2.8/T3/T5、release、其他服务。 |
| base | **`45fe958`**（`git rev-parse HEAD` = `45fe95813ca55f08656d06bf14c5a22b5104b48e`） |
| head | **工作树 @ `45fe958`（未提交）**：新文件全部未跟踪；`tasks.md` 修改 3/3 行（T2.5–T2.7 标 `[x]`，只是进度，不是验收状态）。`legacy_backfill.go` sha256 `f686acee…`；`refund_seq.go` `7b5a340a…`；两个测试 `72fd8179…`/`a22bdb41…`；`cmd/.../main.go` `f888d860…`；`t2.5-plan.md` `7721b90e…`。 |
| final head | **2026-11-27 T2.6 B1 修复后**：`refund_seq.go` sha256 `8c02a7c23ba87058dac0dd0e0200f501e8bfba6287877e5fec226ec0c70bddff`；`refund_seq_integration_test.go` `77d4d54e7476c6d22165cc70a82f824bf62abc79482306746412e2a51b9f5519`；`legacy_backfill_integration_test.go` `dd205dbbf82265ad0c6d4e4624c682a66b0a8ea27c5e478601812844c9005d65`；`ADR-0010` `c5b6643a3b15f16f9313012b8daee9b5c41221dde642d698857535b162bc926a`；plan `77ef33ed71ff6d017c4f3bb3e04317897ffd85939df4a2c5cdd64f26738467e1`；tasks `75abf7a19b9b02f0e6588e1b27c97c19eede7faec8b5da37e6f040bc791e70a6`；`legacy_backfill.go` `f686acee…`；`cmd/.../main.go` `f888d860…`；base `45fe958`。**final：PASS（实现阶段）**；AC-60/63 仍未验证；T2.8/T3 未做。 |
| Spec/ADR | `docs/specs/0002-payment-settlement-admission.md` AC-60/AC-63；`docs/adr/0007-payment-settlement-admission.md` §4.4；**§6.5/§12.5.5 实际在 `docs/adr/0007-payment-settlement-admission-amd1.md:590-610`**（主 ADR 无 §6.5；迁移 `0019` 注释也引 Amendment 1 §6.5）。 |
| INV | 不涉及库存/权限/履约的**新**行为；受影响：AC-60/AC-63、ADR §4.4/§6.5/§12.5.5、宪法 **4/7/20/22**（设计问题回 Spec/ADR；代码不得覆盖契约；退款身份幂等；事务/持久恢复）。 |
| verification | 独立复跑：`go test ./internal/services/order -run 'TestLegacy' -count=1` → `ok`（21.127s）；`-run 'TestLegacyBackfill\|TestLegacyRefundSeq' -v` 5 个测试全 PASS；`go build ./cmd/order-legacy-backfill`、`go vet` → exit 0；边界 diff 0；另用同 SQL 在两段 rollback tx 中取号，**两次都得到 seq=1**（见 §4）。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待裁决；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **第一次：FAIL**（1 阻断 B1：T2.6 同 tx 取号在 rollback 后复用 seq）。**final（2026-11-27，B1 修复后）：PASS**——独立短事务 commit、rollback 不复用、ADR-0010 Accepted；T2.5/T2.7 回归 PASS；**可交人类合入**；**T2.8/T3 未做**；**AC-60/AC-63 仍未验证**（子集）；**NOT RELEASE**。见 §0A。 |

---

## 0A. Final confirmation（2026-11-27，T2.6 B1 修复后）— PASS

| 项 | 值 |
|---|---|
| 裁决 | **PASS（实现阶段；T2.5/T2.6/T2.7）**；B1 消解；ADR-0010 Accepted；**可交人类合入**；**T2.8/T3 未做**；**AC-60/AC-63 仍未验证**；**NOT RELEASE** |
| final head | `refund_seq.go` `8c02a7c2…`；`refund_seq_integration_test.go` `77d4d54e…`；`legacy_backfill_integration_test.go` `dd205dbb…`；`ADR-0010` `c5b6643a…`；plan `77ef33ed…`；tasks `75abf7a1…`；`legacy_backfill.go` `f686acee…`；`cmd/.../main.go` `f888d860…`；base `45fe958` |
| 测试 | full `go test ./internal/services/order -run 'TestLegacy' -count=1` → **ok 24.361s**；定向 `-v`：`TestLegacyBackfillDryRunApplyIdempotent`、`TestLegacyBackfillPartialDuplicateRefundFailure`、`TestLegacyRefundSeqRollbackDoesNotReuse` 全 PASS（另有 concurrent/empty/requires-db PASS）；`go build`/`go vet` exit 0；边界 diff 0。 |
| 重点 1：rollback 不复用 | **证明**：`refund_seq.go` 改为 `s.db.BeginTx` 独立短事务，读回 `LAST_INSERT_ID()` 后 **commit**；`TestLegacyRefundSeqRollbackDoesNotReuse` 外层 tx rollback 后再取号 = `first+1`（PASS）。 |
| 重点 2：orders 行锁前提/竞态 | 函数注释与 ADR-0010 均写“caller must already hold the orders row lock”；独立 seq tx 不触碰 orders 行，seq 行锁保证编号唯一；同一订单的退款创建由调用方 orders 行锁串行化。**测试的外层 tx 没有实际 `SELECT ... FOR UPDATE` orders 行**，所以它证明的是“seq 独立提交、外层 rollback 不复用”，不是“调用方持锁下的退款流程”；T3.5 集成测试需覆盖。**NB-C，非阻断。** |
| 重点 3：ADR-0010 | **可接受**：Status Accepted（人类 2026-11-27 裁决 A）；“取代 ADR-0007 Amd1 §6.5 中‘取号在 orders 行锁事务内’的**事务范围**；表结构/`legacy:<order_no>:<seq>`/不得复用不变”；备选/后果/核对齐全；未改写已批准 ADR（新建 0010）。 |
| 重点 4：T2.5/T2.7 回归 | **PASS**；NB-1 部分失败测试、NB-2 零元+已捕获、NB-3 `NoCaptured=1`/`Samples=4` 均已落入测试并 PASS。 |
| 重点 5：边界 | `git diff 45fe958 -- migrations api internal/services/{payment,inventory,fulfillment} docs/specs` = **0 行**；无新业务 AC；无手改生成物。 |
| 重点 6：AC-60/63 | **仍未验证**；T2.5/T2.6/T2.7 是子集；T2.8/T3/T5 未做；不升级。 |
| 结论 | **PASS（实现阶段）。实现可交人类合入；T2.8/T3 未做；NOT RELEASE。** |

### 0A.1 非阻断观察

- **NB-A**：迁移 `0019:55-58` 注释仍写 “inside the same transaction”“never reused or rolled back”；ADR-0010 取代的是“同一事务”范围，且写明“迁移注释不变”。注释的“same transaction”已过时（“never reused/rolled back”现在成立）；建议后续迁移/文档变更时更新或标注历史。**非阻断。**
- **NB-B**：ADR-0010 标题写“SDD-STRUCT-ALIGN-01 之外的 T2.6 裁决”（应为 `PAY-SETTLE-01` T2.6）；Status 仍带“reviewer 复审待”。**文档 nit，非阻断。**
- **NB-C**：`TestLegacyRefundSeqRollbackDoesNotReuse` 的外层 tx 没有持 `orders` 行锁（只开了空 tx）；`TestLegacyRefundSeqConcurrentUnique` 的 20 并发也绕过了 orders 锁。二者证明 seq 唯一/独立提交，但未验证“调用方持锁下的同一订单退款串行化”；T3.5 集成测试需覆盖。**非阻断（T2.6 是取号器，调用方是 T3.5）。**
- **NB-D**：CLI 配置/连接错误 `exit 2`、运行/逐条错误 `exit 1`；plan 中只写“单条 ensure 失败 exit 1”，没有显式写 exit 2（与“错误 exit 1”的字面差异有意 fail-closed）。**文档缺口，非阻断。**
- **NB-E**：plan 头部“不新开：不新 Spec/ADR”与新增 ADR-0010 冲突；应在 plan 注明“除 ADR-0010 外不新开”。**文档，非阻断。**

---

## 0. 第一次审查结论（FAIL，保留原文）

1. **T2.5、T2.7 通过**：dry-run 零写入/零 Refund、apply 的 unique/zero/ambiguous、只处理 PAID、重入不重复、`remaining` 正确、并发同一订单一次退款；测试全 PASS。
2. **T2.6 并发唯一通过，但同 tx 回滚语义不满足 ADR**：`nextLegacyRefundSeq` 在调用方事务内 `INSERT ... ON DUPLICATE KEY UPDATE next_seq=LAST_INSERT_ID(next_seq+1)`；事务 rollback 会把 `next_seq` 一起回滚，下一次取号复用同一 `seq`。ADR Amendment 1 §6.5 与迁移 `0019:55-58` 明写“不得复用/回退 / never reused or rolled back”；`refund_seq.go` 注释却改成“monotonic for committed allocations”，这是代码侧单方面弱化契约（宪法 7），不是作者可自行解释的。
3. **AC-60/AC-63 未升级**：T2.5/T2.6/T2.7 只是子集；AC-60 的退款/发货准入、AC-63 的 24h/`COMPENSATING`/`EXPIRED_*`/Payment 退款/终端分支仍需 T2.4/T3.5/T5 证据与 `TestAC60/TestAC63`。**不标已验证。**
4. **T2.8/T3 未做**；即使 B1 解决，本裁决也不等于 Release 或全量 AC 验收。

---

## 1. 审查输入与方法

- 读：`legacy_backfill.go`、`refund_seq.go`、两个集成测试、`cmd/order-legacy-backfill/main.go`、`t2.5-plan.md`、`tasks.md` diff、SPEC-0002 AC-60/63、ADR §4.4/§6.5/§12.5.5、迁移 `0019`。
- 跑：用户给的 `go test`（`TestLegacy` 全量 + T2.5/T2.6 定向 `-v`）、`go build`、`go vet`、边界 `git diff`。
- 独立验证 T2.6 回滚：在测试库用**同一 SQL** 在两段 `BEGIN ... ROLLBACK` 中取号；两次均 `seq=1`，回滚后 `order_refund_sequences` 行不存在。临时脚本在 `.tmp/`，已删除；测试库临时行已删除。
- 未改代码/测试/迁移/契约/工具；测试产生的 `order-legacy-backfill` 二进制和 `.tmp` 脚本已清理；未 commit/merge。

---

## 2. 测试与构建

```text
$ TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
    go test ./internal/services/order -run 'TestLegacy' -count=1
ok   gin-kratos-gorm/internal/services/order  21.127s

$ go test ... -run 'TestLegacyBackfill|TestLegacyRefundSeq' -count=1 -v
--- PASS: TestLegacyBackfillDryRunApplyIdempotent (0.99s)
--- PASS: TestLegacyBackfillConcurrentSameOrderOneRefund (0.87s)
--- PASS: TestLegacyRefundSeqConcurrentUnique (0.93s)
--- PASS: TestLegacyRefundSeqRejectsEmptyOrder (0.90s)
--- PASS: TestLegacyRefundSeqRequiresTransaction (0.00s)
PASS

$ go build ./cmd/order-legacy-backfill   # exit 0
$ go vet ./internal/services/order ./cmd/order-legacy-backfill   # exit 0
```

边界：

```text
$ git diff 45fe958 -- migrations api internal/services/payment internal/services/inventory internal/services/fulfillment docs/specs | wc -l
0
```

`Spec.md`/契约/其他服务/迁移/生成物未改；无新 Spec/ADR；`tasks.md` 只把 T2.5–T2.7 标 `[x]`（进度，不是验收状态，宪法 27）。

---

## 3. T2.5 回填工具

| 检查 | 结论 |
|---|---|
| 默认 dry-run、`-apply` 唯一写开关 | **通过**：CLI `apply` 默认 false，`DryRun: !*apply`；dry-run 只 `findOrder`/`ListOrderIntents`，不 `Refund`、不写订单/队列/seq。测试断言 dry-run 零 settlement/零 queue/零 refund。 |
| 只处理 PAID | **通过**：SQL `WHERE status = ? AND settlement_payment_no = ''`，`statusPaid`；测试 `ORD-BF-P`（PENDING_PAYMENT）未动。列 `settlement_payment_no` 是 `NOT NULL DEFAULT ''`（`0016`），`''` 足够；AC-63 的 `''/NULL` 是防御性措辞。 |
| unique / zero / ambiguous | **通过**：dry-run 分类与 apply 结果一致（unique=1、zero=1、ambiguous=1）；`remaining=1`。 |
| 重入不重复 | **通过**：第二次 apply 只剩 ambiguous，`Queued=1`、`Remaining=1`、无第二次 refund、queue 仍 1 行。 |
| 并发同一订单一次退款 | **通过**：6 并发同一订单，settlement 唯一、refund 1 次、queue 0 行。 |
| 部分失败（结算已写、重复捕获退款失败） | **代码路径**：`ensureLegacyResolution` 返回 error → `report.Errors++`/`firstErr` → 继续下一条 → 函数返回 `firstErr` → CLI 在 `runErr != nil || report.Errors > 0` 时 `exit 1`；没有静默重试/吞错。**但没有测试覆盖该路径**（NB-1），无法把“如实报错”标为已验证。 |
| CLI 退出码 | 运行/逐条错误 `exit 1`；DSN/Ping/gRPC 等配置错误 `fatal` → `exit 2`。与 T2.5 描述的“错误 exit 1”字面不完全一致（NB-4）；ops 若只判断非零，可接受。 |
| 零元 + 已捕获 | 代码正确：`record.PayCent==0 && !AnyCaptured` 才 zero；有捕获 → candidates=0 → ambiguous。**T2.5 测试没有这个用例**（NB-2）；既有 T2.1–T2.4 测试可能覆盖，但 T2.7 矩阵缺。 |
| `no_captured` 计数 | 代码有；测试只断言 Scanned/Unique/Zero/Ambiguous，未断言 `NoCaptured`（NB-3，非阻断）。 |

**T2.5 结论：功能与测试骨架通过；部分失败/零元+已捕获/`no_captured` 的测试缺口是 NB，不阻断（代码 inspection 未见错误）。**

---

## 4. T2.6 取号器（B1）

### 4.1 已通过

- 20 并发 `nextLegacyRefundSeq`：seq 唯一 1..20，`next_seq=20`。
- `LAST_INSERT_ID` 读回在**同一 `*sql.Tx`**（`tx.ExecContext` 后 `tx.QueryRowContext`），连接绑定正确。
- `orders` 行锁前提在函数注释与 plan 中写清：“caller must already hold the orders row lock”；T3.5 是调用方，T2.6 只交付取号器。
- `legacyRefundID` = `legacy:<order_no>:<seq>`；空 order/nil tx 有测试。

### 4.2 B1：同 tx rollback 复用 seq，违反“不得复用/回退”

- ADR **Amendment 1 §6.5**（`docs/adr/0007-payment-settlement-admission-amd1.md:590-610`）：取号在 orders 行锁事务内，`INSERT ... ON DUPLICATE KEY UPDATE next_seq = LAST_INSERT_ID(next_seq + 1)`；**“不得复用/回退”**。
- 迁移 `migrations/order/0019_legacy_payment_resolution.sql:55-58`：Numbers are allocated inside the same transaction …；**“they are never reused or rolled back (ADR §6.5)”**。
- 实现 `refund_seq.go:16-20` 的注释却写：**“monotonic for committed allocations and is never shared between two committed refunds”**——把“不得复用/回退”弱化成“提交过的分配不复用”。这是代码侧自行解释契约（宪法 7：代码不得覆盖契约；宪法 4：设计问题回 Spec/ADR），不是实现者可单方面决定。
- 独立验证：用同 SQL 在两段 `BEGIN ... ROLLBACK` 中取号：

```text
allocation 1 -> seq=1
allocation 2 -> seq=1
next_seq row absent after rollbacks
```

  说明 rollback 后 `next_seq` 回滚，下一次分配复用同一 `seq`；`legacy:<order_no>:<seq>` 身份可被下一次尝试复用。
- 测试缺口：`refund_seq_integration_test.go` 只覆盖 commit 路径，没有 rollback 后取号的测试。
- 这是 **设计/契约冲突**，不是局部补丁：ADR 同时要求“同 tx 取号”和“不得复用/回退”；要同时满足，需在两者之间做人类/架构裁决：
  1. 独立提交取号（autonomous sequence / 单独事务提交）使 seq 跨 refund rollback 持久化；或
  2. 修正 ADR/迁移注释为“不重用**已提交** refund_id；rollback 的分配可复用”，并给人类批准（合同变更）。
- 在裁决前，T2.6 **不能**按“不得复用/回退”验收；`tasks.md` 的 `[x]` 只是进度。

---

## 5. T2.7 测试覆盖

| 用例 | 覆盖 |
|---|---|
| dry-run 零写入/零 Refund | 有（`TestLegacyBackfillDryRunApplyIdempotent`） |
| unique / zero / ambiguous | 有（同一测试） |
| non-PAID 不碰 | 有（`ORD-BF-P`） |
| 重入不重复 refund/queue | 有 |
| 并发同一订单一次退款 | 有（6 并发） |
| 20 并发 seq 唯一 | 有（`TestLegacyRefundSeqConcurrentUnique`） |
| 部分失败（settlement 已写、refund 失败） | **无**（NB-1） |
| 零元 + 已捕获（不得 zero） | **无**（NB-2） |
| seq rollback 后复用 | **无**（B1 的测试缺口） |

不是“只断言 happy path”（已覆盖 ambiguous/non-PAID/重入/并发），但**关键边界**（部分失败、零元+已捕获、seq rollback）缺失；这些缺口与 B1/NB-1/NB-2 对应。

---

## 6. AC-60 / AC-63 状态（不升级）

- 本次**不改** `docs/specs/0002` 的 AC 状态；AC-60/AC-63 **保持未验证**。
- T2.5/T2.6/T2.7 是子集：
  - **AC-60**：历史 PAID 的读取/展示、退款准入（无售后单 `legacy:<order_no>:<seq>`）、发货时 Inventory 状态收敛。T2.5 回填、T2.6 取号是其中一部分；退款准入调用方（T3.5）、发货准入（T3）、T5 证据未完成。
  - **AC-63**：完整队列/24h/`COMPENSATING`/`EXPIRED_*`/Payment 退款/终端分支。T2.4 worker + T2.5 回填 + T2.6 取号覆盖一部分；T3.5、T5、`TestAC63` 未完成。
- 因此**没有逐条 AC 依据可以升级 AC-60/63**；不得用 T2.5/T2.6/T2.7 的绿测冒充 AC-60/63 已验证（宪法 11/12）。
- T2.8/T3 仍未做；Release 不在本次范围。

---

## 7. 问题

> final confirmation：B1 已消解（独立短事务提交）；见 §0A。本节是第一次审查原文。

### B1（阻断，T2.6）

**同 tx 取号在 rollback 后复用 seq，违反 ADR §6.5 / 迁移 0019 的“不得复用/回退”。**

- **证据**：ADR Amendment 1 §6.5“不得复用/回退”；迁移 `0019:55-58`“never reused or rolled back”；`refund_seq.go` 同 tx + `ON DUPLICATE KEY UPDATE`；代码注释自行弱化为“committed allocations”；独立 rollback 实验两次 seq=1；测试只 commit。
- **条款**：宪法 **4**（设计问题回 Spec/ADR）、**7**（代码不得覆盖契约）、**20/22**（退款身份幂等/持久恢复）；ADR §6.5/§12.5.5。
- **影响**：退款 tx 在取号后失败/回滚，下一次可用同一 `legacy:<order_no>:<seq>`；与“不得复用/回退”不一致；若下游以 refund_id 做幂等键，可能把两次不同的尝试当成同一退款（或反之）。这是资金身份语义，不是风格问题。
- **处置**：需要人类/架构裁决（独立提交取号 vs 修正“不得复用/回退”为“已提交 id 不复用”）；加 rollback 测试；再复审。**不能**用局部补丁掩盖（宪法 4/10）。

### NB

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | T2.5 部分失败（settlement 已写、重复捕获退款失败）无测试；代码路径会 `Errors++`/`firstErr`/`exit 1`，未静默重试，但未验证 | T2.5 计划停止条件；T2.7 |
| NB-2 | T2.5 零元 + 已捕获（不得 zero）无测试；代码分类为 ambiguous，但未验证 | AC-63/AC-40；T2.7 |
| NB-3 | T2.5 测试未断言 `NoCaptured`/`Samples` 计数 | 报告字段；T2.7 |
| NB-4 | CLI 配置/连接错误 `exit 2`，运行/逐条错误 `exit 1`；与“错误 exit 1”字面不完全一致 | T2.5 描述；CLI `fatal` |
| NB-5 | §6.5 实际在 Amendment 1 文件，主 ADR 无 §6.5；迁移/任务书引用需更精确 | 文档引用 |

---

## 8. 裁决与下一步

> final confirmation：**PASS（实现阶段，B1 消解）**；见 §0A。

**FAIL。** T2.5/T2.7 通过；T2.6 并发唯一通过但 B1（rollback 复用 seq）阻断；AC-60/AC-63 未验证；T2.8/T3 未做；NOT RELEASE。

**不能交人类合入**，直到 B1 由人类/架构裁决并落实：
1. 决定“独立提交取号”还是“修正‘不得复用/回退’为已提交 id 不复用”（合同/设计变更）；
2. 按决定改 ADR/迁移/实现之一，并加 seq rollback 测试；
3. 复审 T2.6；随后 T2.5/T2.7 保持通过；AC-60/63 仍等 T3.5/T3/T5 与 `TestAC60/TestAC63`；
4. T2.8/T3 仍未做；Release 由人类，NOT RELEASE。

---

## 9. 复现命令（只读）

```bash
git rev-parse HEAD   # 45fe958...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestLegacy' -count=1
go build ./cmd/order-legacy-backfill && go vet ./internal/services/order ./cmd/order-legacy-backfill
git diff 45fe958 -- migrations api internal/services/payment internal/services/inventory \
  internal/services/fulfillment docs/specs | wc -l   # 0
```

**本次落盘**：本报告（§0A final PASS）+ `docs/review-tracking.md` 最新块顶部 FAIL→PASS 两条裁决。未改代码/测试/迁移/契约/工具；未 commit、未 merge。
