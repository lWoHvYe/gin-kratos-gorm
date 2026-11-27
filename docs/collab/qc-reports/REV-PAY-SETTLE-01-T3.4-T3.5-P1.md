# REV-PAY-SETTLE-01-T3.4-T3.5-P1 — T3.4/T3.5 Phase 1（行组成 + commitment lines）审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（在途；SPEC-0002 AC-60/AC-63 范围内） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | `internal/services/order/refund_composition.go`（新增）；`refund_admission.go` 的 composition/commitment 插入改动；`t3_5_line_composition_integration_test.go`；`t3.4-t3.5-plan.md`；`tasks.md` T3.4/T3.5 状态。**只审 Phase 1**；responsibility/reason_code 封闭枚举、168h、T3.6、post-departure partial 强制 lines、T3.2 release **不在本轮**。 |
| base | **`0740286`**（`git rev-parse HEAD` = `0740286d78cda6530eb2b7f2371cee67d1f7f2b8`） |
| head | **工作树 @ `0740286`（未提交）**：`refund_admission.go` 改 6/6 行；新增 `refund_composition.go` sha256 `6115b48fef6916cb6aed7503e9c4b3cd820d856bd0fba6234ce69e7d2aef9da9`、`t3_5_line_composition_integration_test.go` `c5d4b9d1aaf62d0c12ae64722f7578f5074a55f2adf3c8cd70a80fc5df159c51`、plan `316b1dcedbea7eb7978787c80f63fdd87ec501c2b1340dc9979fbcf3da81cb9f`；`tasks.md` 未改，T3.4/T3.5 仍 `[ ]`。 |
| final head | **2026-11-27 delta 后**：`refund_composition.go` sha256 `2a2468d470976720b508c119bf56ed0bbcc0f967b5c8a034a53057a77d74cea0`；`t3_5_line_composition_integration_test.go` `f1a7086dc079a1844b7c7ce1aa6d71f0edb9e86ba17014a8ca8eca57a99d22be`；plan `3abc51c7a3163779d61081b381907b57657828a95a112360b9304f6293d8e33e`；`refund_admission.go` `55da0fae…`（6/6）；base `0740286`。 |
| spec_version | **SPEC-0002 v1.11**（approved 2026-11-26；AC-60/AC-63） |
| adr_revision | **ADR-0007 Rev 2.3 / Amendment 1 §4.4**（行级/组级/Payment 三层；S14/S15） |
| baseline_impact | **有**：`AdmitRefund` 对带 `lines` 的请求新增行组成校验与 commitment lines 落库；无 lines 的旧请求保持 amount-only 路径；不改 proto/migration/其他服务；影响 INV-24/INV-47、AC-60/63（**仍不升级**）。 |
| INV | **INV-24**（行剩余 ≤ 实付 − S14 − S15；无授权超退）、**INV-47**（行组成/header 一致）、AC-60/63、宪法 20/22/23。 |
| verification | **命令实测**：`go test ./internal/services/order -run 'TestAdmitRefund\|TestTccAdmission\|TestLegacy' -count=1` → **ok 33.985s**；4 个 Phase 1 新测试 `-v` 全 PASS；`go build ./internal/services/order/...`、`go vet` exit 0；`git diff 0740286 -- migrations api docs/specs` = 0；`refund_admission.go` 只 6/6 行（composition 调用 + commitment helper）。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **SCOPED-PASS（Phase 1）；delta 后仍成立**：NB-1/2/4/6 已修，6 个 Phase 1 测试 PASS；T3.4/T3.5 其余（responsibility/reason_code/168h/T3.6、post-departure partial 强制 lines）未做；**AC-60/63 仍未验证**。0 阻断；NB-3 部分剩余、NB-5 仍在、NB-7（quantity=0 产品语义）。 |

---

## 0A. Delta confirmation（2026-11-27，NB 加固后）— SCOPED-PASS 仍成立

| 项 | 值 |
|---|---|
| 裁决 | **SCOPED-PASS（Phase 1）仍成立**；NB-1/2/4/6 已修；新增 NB-7（quantity=0 产品语义，不阻断） |
| final head | `refund_composition.go` `2a2468d4…`；test `f1a7086d…`；plan `3abc51c7…`；`refund_admission.go` `55da0fae…`（6/6）；base `0740286` |
| 测试 | full `go test ./internal/services/order -run 'TestAdmitRefund\|TestTccAdmission\|TestLegacy' -count=1` → **ok 51.969s**（本机）；6 个 Phase 1 测试 `-v` 全 PASS；`go build`/`go vet` exit 0；`git diff 0740286 -- migrations api docs/specs` = 0 |
| NB-1 | **已修**：plan 状态 **Approved（2026-11-27 人类批准）**，写明本轮只做 Phase 1；并发措辞改为“orders 行锁串行化 + 行剩余校验；commitment lines PK 不是跨 refund 唯一键”。 |
| NB-2 | **已修**：plan 的“唯一键兜底”已按 `(refund_id,line_no)` PK 的实际语义改写。 |
| NB-4 | **已修**：header 回读比较 `order_no/group_no/composition_source`；同 refund_id 换 order/group/source 同金额 → fail-closed。 |
| NB-3/NB-6 | **部分已修**：新增 `TestAdmitRefundPositiveFreightWritesFreightLine`、`TestAdmitRefundLineRemainingIncludesSettled`（SETTLED 4000 行剩余 1000；4000 拒绝、1000 通过）；正常落库测试增加同 refund_id 幂等（1 admission/1 line）；over-limit 断言错误文本 `remaining line amount`。仍缺并发同一行、known=0 amount-only 允许、mixed known=0+1 的测试（NB-3 剩余）。 |
| NB-5 | **仍在**：known=1 但无 lines 的畸形组，amount-only 请求不加载 composition，不发现（有 lines 请求会 fail-closed）。 |
| NB-7（新） | `TestAdmitRefundLineRemainingIncludesSettled` 用 prior SETTLED line `quantity=0, goods=4000` 模拟“部分金额不消耗整件”；请求 quantity 仍必须 >0。**“prior 部分金额 + quantity=0”的语义需产品/架构确认**；不阻断 Phase 1（本轮不改变请求 quantity>0 的校验）。 |
| 其他 | `.gitignore` 新增 `/order-legacy-backfill`（构建产物忽略；无行为；delta 未声明，属 housekeeping，非阻断）。 |
| 结论 | **SCOPED-PASS（Phase 1）仍成立。Phase 1 行组成可交人类合入；T3.4/T3.5 其余（responsibility/168h/T3.6）未做；AC-60/63 仍未验证；NOT RELEASE。** |

---

## 0. 第一次审查结论（Phase 1 SCOPED-PASS，保留原文）

1. **Phase 1 行组成可用**：`validateRefundComposition` 校验 line_no/quantity/goods、行归属组、行剩余（`LinePaid - RESERVED/SETTLED commitment lines`）、`Σ goods + freight = amount`、freight ≤ 组剩余；`insertRefundCommitmentTx` 写 header composition + `order_refund_commitment_lines`（line 0 运费、>0 商品）并回读一致。
2. **S14/S15 都算**：`loadTccGroupComposition` 的 header 与 lines 查询都是 `state IN (RESERVED, SETTLED)`；行剩余同时扣 RESERVED 与 SETTLED，不是只算 RESERVED（检查 2 通过；SETTLED 场景没有专门测试，NB-3）。
3. **`composition_known=0` 组拒绝行级**：loader 标记 `UnknownComposition`，有 lines 的请求 fail-closed；无 lines 的 amount-only 旧路径不变；混合组（known=0 + known=1）也因 `UnknownComposition` 拒绝行级（检查 3 通过；mixed 场景无测试，NB-3）。
4. **并发**：`AdmitRefund` 先 `findOrderForUpdate` 锁 orders 行，再在 Tx 内读 commitment lines；同一订单两笔不同 refund 抢同一行由 orders 行锁串行化 + 行剩余校验拦截。`order_refund_commitment_lines` 的 PK 是 `(refund_id,line_no)`，**不**提供跨 refund 的同一行唯一兜底（plan 的“唯一键兜底”措辞不准确，NB-2）；无新增并发测试（本任务允许代码审查）。
5. **幂等/冲突**：同 `refund_id` 重试 header/lines 用 `ON DUPLICATE KEY UPDATE` + 回读比对，不重复；amount/goods/freight/known 不同 → fail-closed。未比较 `order_no/group_no/composition_source`（NB-4）；无幂等/冲突测试（NB-3）。
6. **旧 amount-only 未变**：`composition == nil` 时 helper 用原 SQL（无 goods/freight/known 列），`TestTccAdmission`/`TestLegacy`/既有 `TestAdmitRefund` 全 PASS（检查 6 通过）。
7. **未越权**：`tasks.md` T3.4/T3.5 仍 `[ ]`；plan 状态仍写“Draft，待人类批准”与用户“已批准”不一致（NB-1），但代码/交付没有声称 T3.4/T3.5 完成。

---

## 1. 审查输入与方法

- 读：`refund_composition.go`、`refund_admission.go` diff、`t3_5_line_composition_integration_test.go`、`t3.4-t3.5-plan.md`、`tasks.md`、`loadTccGroupComposition`/`loadTccCommitmentLines`/`loadTccLinesByGroup`、`0018` 的 `order_refund_commitment_lines`。
- 跑：全量 `TestAdmitRefund|TestTccAdmission|TestLegacy`（MySQL `mall_order_test`）→ PASS；4 个 Phase 1 新测试 `-v` → PASS；build/vet。
- 边界：`git diff 0740286 -- migrations api docs/specs` = 0；`refund_admission.go` diff 6/6 行，只有 composition 调用 + commitment helper 替换。
- 未改代码/测试/契约/迁移；未 commit/merge。

---

## 2. 测试

```text
$ go test ./internal/services/order -run 'TestAdmitRefund|TestTccAdmission|TestLegacy' -count=1
ok   gin-kratos-gorm/internal/services/order  33.985s

=== RUN   TestAdmitRefundLineCompositionPersists        --- PASS (0.53s)
=== RUN   TestAdmitRefundLineOverLimitWritesNothing     --- PASS (0.53s)
=== RUN   TestAdmitRefundLineSumMismatchWritesNothing   --- PASS (0.64s)
=== RUN   TestAdmitRefundLegacyCompositionRejectsLines  --- PASS (0.63s)
```

- 正常落库：header 3000/0/known=1/aftersale + line 1 3000；PASS。
- 行超限/Σ 不匹配/known=0 拒绝：都断言 admission/commitment/commitment_lines 零行（known=0 只断言新 refund_id 零行）。
- **局限**：行超限测试只断言 `err != nil`，未断言错误是行层而非 group/amount 层；无 SETTLED/known=1 prior 行剩余、positive freight/line 0、幂等/冲突、known=0 amount-only、mixed known=0+1、concurrency 测试（NB-3）。

---

## 3. 行剩余是否含 S14 + S15（检查 2）

`loadTccGroupComposition`（`tcc_settlement.go:1091+`）：

- header：`WHERE order_no = ? AND state IN (RESERVED, SETTLED)`；
- lines：同样 `state IN (RESERVED, SETTLED) AND composition_known = 1`；
- `comp.LineGoods`/`LineQuantity` 汇总到 `validateRefundComposition` 的 `remainingGoods := linePaid - comp.LineGoods[lineNo]`、`remainingQuantity := quantity - comp.LineQuantity[lineNo]`。

**结论：RESERVED（S15）与 SETTLED（S14）都扣；没有只算 RESERVED 的漏洞。** 但 **没有测试** 先塞一笔 SETTLED commitment lines 再请求行退款（NB-3）；代码路径正确。

---

## 4. `composition_known=0` 与混合组（检查 3）

- `loadTccGroupComposition` 对 `composition_known=0` 的 RESERVED/SETTLED header 设 `UnknownComposition=true`，不把其 goods/freight 作为行归属。
- `validateRefundComposition` 先检查 `comp.UnknownComposition`，有 lines → `Failed("... only a whole-group amount-only refund is allowed")`；无 lines → 返回 nil，旧 amount-only 路径继续。
- 混合组（known=0 + known=1）：`UnknownComposition=true` → 行级拒绝；loader 的 `g1` 仍校验 known=1 部分 header/lines 一致。
- **已有 header 但无 lines 的 known=1 组**：`loadTccGroupComposition` 回读 known=1 header 与 line 行，若 header goods/freight ≠ line sums → fail-closed。但该 loader **只在有 lines 的请求**里调用；amount-only 请求不触发，因此 known=1 header 无 lines 的畸形状态在 amount-only 路径不被发现（NB-5，非本轮新写入的状态）。
- 新测试只覆盖 known=0 拒绝行级；混合组/known=1 无 lines/known=0 amount-only 允许均无测试（NB-3）。

---

## 5. 并发（检查 4）

- `AdmitRefund` 在 Tx 开始后 `findOrderForUpdate` 锁 orders 行；`validateRefundComposition`/`insertRefundCommitmentTx` 都在同一 Tx 内、锁之后。
- 同一订单两笔不同 refund 抢同一行：第二笔等 orders 行锁；锁后读 SETTLED/RESERVED lines，行剩余不足即拒绝。**orders 行锁 + 行剩余校验是实际守卫。**
- `order_refund_commitment_lines` PK = `(refund_id, line_no)`；`order_refund_commitments` unique = `refund_id`。它们只保证**同一 refund_id** 不重复，**不是**跨 refund 的同一行唯一约束；plan 的“orders 行锁 + 唯一键兜底”表述不准确（NB-2）。
- 无新增并发测试；用户允许代码审查。代码路径未发现明显竞态（锁顺序一致，未在锁内调远程 Payment）。

---

## 6. 幂等/冲突（检查 5）

- 同 `refund_id` 重试：header `INSERT ... ON DUPLICATE KEY UPDATE refund_id = refund_id`；lines 同样；随后 `SELECT ... FOR UPDATE` + `loadTccCommitmentLines` + `equalTccCommitmentLines`，同组成不重复、成功。
- 不同 amount/goods/freight/known → `Failed("refund commitment conflicts...")`；不同 lines 但 header 相同 → 回读比对失败 → fail-closed。
- **未检查** `order_no/group_no/composition_source`：同一 refund_id 换个 order/group/source 但 amount/goods/freight 相同会被当作幂等（admission 既有检查可能先拦截 order/group；source 无检查，NB-4）。
- `state` 允许 `RESERVED` 与 `SETTLED`；SETTLED 无 admission 的畸形行可被“resume”式处理（NB-4，非本轮新写入）。
- 无幂等/冲突测试（NB-3）。

---

## 7. 旧 amount-only 未变（检查 6）

- `composition == nil` 时 `insertRefundCommitmentTx` 走原 SQL：
  `INSERT INTO order_refund_commitments (order_no,group_no,refund_id,amount_cent,state,version) ... ON DUPLICATE KEY UPDATE refund_id = refund_id`；
  不写 goods/freight/known/lines。
- `TestTccAdmission`/`TestLegacy`/既有 `TestAdmitRefund` 全 PASS；TCC/T1.4 旧请求未因 Phase 1 改变。
- 新测试 `TestAdmitRefundLineSumMismatchWritesNothing` 也证明 freight≠0 且无 lines 时被拒绝（`validateRefundComposition` 的 `freight != 0` 分支）。

---

## 8. 边界与“Phase 1 越权”（检查 7/8）

- `git diff 0740286 -- migrations api docs/specs` = **0**；无 proto/migration/契约改动。
- `refund_admission.go` = **6 added / 6 deleted**，只有 composition 调用 + commitment helper 替换；无其他行为改动。
- `tasks.md` **未改**，T3.4/T3.5 仍 `[ ]`；没有“T3.4/T3.5 完成”的声称。plan 状态仍写 **“Draft，待人类批准 Plan 后实现”**，与用户“人类已批准”不一致（NB-1）；应更新为 Approved 并注明只做 Phase 1。
- 新增文件均未跟踪；无生成物/二进制残留。

---

## 9. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | plan 状态仍 “Draft，待人类批准”，与“人类已批准 T3.4+T3.5 Plan”不一致；应更新状态并写清本轮只做 Phase 1 | `t3.4-t3.5-plan.md` 头部 |
| NB-2 | plan“并发由 orders 行锁 + 唯一键兜底”不准确：`order_refund_commitment_lines` PK 是 `(refund_id,line_no)`，不提供跨 refund 同一行唯一；实际是 orders 行锁 + 行剩余校验 | `0018` PK；`refund_admission.go` |
| NB-3 | 测试缺口：SETTLED/known=1 prior 行剩余、positive freight/line 0、幂等/冲突、known=0 amount-only、mixed known=0+1、并发同一行 | `t3_5_line_composition_integration_test.go` |
| NB-4 | `insertRefundCommitmentTx` 的 header 冲突未比较 `order_no/group_no/composition_source`；SETTLED 无 admission 的畸形行可被 resume 式处理 | `refund_composition.go:180-200` |
| NB-5 | known=1 但无 lines 的畸形组：有 lines 请求会 fail-closed，amount-only 请求不加载 composition，不发现 | `refund_composition.go:42-47`；`tcc_settlement.go` loader |
| NB-6 | 行超限测试只断言 `err != nil`，未断言错误来自行层而非 group/amount 层 | `TestAdmitRefundLineOverLimitWritesNothing` |

---

## 10. 裁决

> final：delta 后仍 SCOPED-PASS；见 §0A。

**SCOPED-PASS（Phase 1）。** Phase 1 行组成 + commitment lines 落库通过真实 MySQL 回归；S14/S15 都计入行剩余；`composition_known=0`/混合组拒绝行级；旧 amount-only 未变；边界无迁移/proto/契约；T3.4/T3.5 仍 `[ ]`，无越权。

**Phase 1 行组成可交人类合入；T3.4/T3.5 其余（responsibility/reason_code、168h、T3.6、post-departure partial 强制 lines）未做；AC-60/63 仍未验证；NOT RELEASE。**

---

## 11. 复现命令（只读）

```bash
git rev-parse HEAD   # 0740286...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestAdmitRefund|TestTccAdmission|TestLegacy' -count=1
go build ./internal/services/order/... && go vet ./internal/services/order
git diff 0740286 -- migrations api docs/specs | wc -l        # 0
git diff 0740286 --numstat -- internal/services/order/refund_admission.go  # 6/6
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 SCOPED-PASS 裁决。未改代码/测试/契约/迁移；未 commit、未 merge。
