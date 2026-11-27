# REV-PAY-SETTLE-01-T3-AUDIT — T3 审计 + T3.1/T3.3 不变量测试审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（在途；SPEC-0002 AC-60/AC-63 范围内） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | `internal/services/order/t3_invariants_integration_test.go`（T3.1/T3.3 不变量测试）；`docs/lead/PAY-SETTLE-01/t3-audit.md`（T3.1–T3.7 现状/缺口）；`tasks.md` T3 进度。**不审** T3.2/T3.4/T3.5/T3.6 的实现（未交付）、T3.7 测试矩阵、release。 |
| base | **`7d103a5`**（`git rev-parse HEAD` = `7d103a51eb4b36018eaf483192f5406f78392267`，T2.5/T2.6/T2.7 已提交） |
| head | **工作树 @ `7d103a5`（未提交）**：`t3_invariants_integration_test.go` sha256 `e167630e0a173316aa2136a6e342c435178373a090fffaf667bac67446159f66`（95 行）；`t3-audit.md` `44236a319e066a948f9c42ab1e4272c336c7332244d66e383ac19d1cb844f4c6`（61 行，末尾有字面 `EOF`）；`tasks.md` `7796b327a06cddd1b5040ea0b55bbc16facc63d7c6b0cacd5828eb227e5e192d`（T3.1/T3.3 `[x]`，其余 `[ ]`）。 |
| spec_version | **SPEC-0002 v1.11**（approved 2026-11-26；AC-60/AC-63） |
| adr_revision | **ADR-0007 Rev 2.3 / Amendment 1 Rev 1.0**（T3.1/T3.3 相关） |
| baseline_impact | **不涉及**：仅新增测试/审计文档，无生产代码/迁移/契约改动；不改既有 permit/refund 行为；T3.1/T3.3 是既有行为的不变量测试。 |
| Spec/ADR | `docs/specs/0002-payment-settlement-admission.md` AC-60/AC-63；`docs/adr/0007-payment-settlement-admission.md` §4.4；ADR-0007 Amd1 §6.5（permit/refund）；`migrations/order/0018_refund_commitment_composition.sql`、`0019_legacy_payment_resolution.sql`（只读对照）。 |
| INV | 不涉及新业务行为；受影响：T3.1 permit 不可恢复、T3.3 准入前拒绝不建 commitment/outbox、AC-60/63（仍未验证）、宪法 20/22/23。 |
| verification | **静态 + 命令实测**：`go test ./internal/services/order -run 'TestRevokedPermitCannotBeRestored|TestRejectedAdmissionWritesNoCommitment' -count=1 -v` → 2 个测试 PASS（1.692s）；全包 `grep` 6 处 `UPDATE order_shipping_permits` + 1 处 `INSERT`，逐条核对无 `REVOKED→NONE/GRANTED`；`git diff 7d103a5 -- 'internal/services/order/*.go'` 无内部非测试改动；`git diff 7d103a5 -- migrations api docs/specs` = 0；`tasks.md` 只 T3.1/T3.3 改 `[x]`。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **SCOPED-PASS**：T3.1/T3.3 不变量由公共路径测试 + 全包 permit 写路径 grep 证实；T3.2/T3.4/T3.5/T3.6 缺口判断准确、需 Plan/人类批准；**AC-60/63 仍未验证**。0 阻断；NB-1…NB-6（审计精度/测试覆盖面）。 |

---

## 0. 结论

1. **T3.1 满足**：全包 6 处 permit 写路径（`settlement.go` Authorize/Block、`refund_admission.go` 两处、`tcc_settlement.go`、`legacy_resolution_worker.go`）逐条核对，只有 `NONE→REVOKED`、`NONE/GRANTED→CONSUMED`、`CONSUMED→CONSUMED`、`GRANTED→REVOKED`；**没有 `REVOKED→NONE/GRANTED`**。`TestRevokedPermitCannotBeRestored` 在 AdmitRefund/BlockShipmentForRefund/AuthorizeShipment 三条公共路径后都断言 permit 仍 `REVOKED`。
2. **T3.3 满足（invariant 层面）**：`AdmitRefund` 的 over-amount / 组剩余 / partial unshipped 校验在 admission/commitment/outbox **写入之前**返回错误；`TestRejectedAdmissionWritesNoCommitment` 断言 admissions/commitments/outbox 零行、permit 仍 `NONE`。**注意**：校验发生在 `db.WithTx` 内、`ensureGroupExistsTx`/`ensurePermitTx` 之后，不是“Tx1 之前”；`ensurePermitTx` 在 permit 缺失时可能先插一行 `NONE`（错误时回滚）。审计的“Tx1 前”措辞不准确（NB-1）。
3. **T3.2/T3.4/T3.5/T3.6 缺口判断准确**：proto 已有 `RefundLine lines/responsibility/reason_code/freight_refundable_cent`（`mall.proto:531-542`），`0018` 已有 `goods_cent/freight_cent/composition_known` 与 `order_refund_commitment_lines`（`:63`），但 `AdmitRefund` **不读** `GetLines/GetResponsibility/GetFreightRefundableCent`（grep 零命中）；T3.2 无 `order_refund_commitments` 的 `RELEASED` 生产写入路径、无退款 release RPC；T3.6 未接线。审计结论可信。
4. **AC-60/63 仍未验证**：T3.1/T3.3 是子集；T3.2/T3.4/T3.5/T3.6/T3.7/T5 未完成；不升级。
5. **SCOPED-PASS**：T3.1/T3.3 通过；未覆盖的测试路径（tcc/worker permit 写、CONSUMED partial 分支）已在 NB 列出；T3.2/T3.4/T3.5/T3.6 需 Plan/人类批准；**NOT RELEASE**。

---

## 1. 审查输入与方法

- 读：两个测试、`t3-audit.md`、`tasks.md` T3 段、`settlement.go`/`refund_admission.go`/`tcc_settlement.go`/`legacy_resolution_worker.go` 的 permit 写路径、proto `AdmitRefundRequest`、`0018`/`0019` 迁移。
- 跑：两个 T3 测试（MySQL `mall_order_test`）→ PASS；`grep` 全 order 包 `UPDATE/INSERT/DELETE order_shipping_permits`。
- 边界：`git diff 7d103a5 -- 'internal/services/order/*.go'` = 0（新增 `_test.go` 未跟踪）；`git diff 7d103a5 -- migrations api docs/specs` = 0。
- 未改代码/测试/契约/迁移；测试产生的 `order-legacy-backfill` 二进制已删除；未 commit/merge。

---

## 2. 两个测试是否真的证明不变量

### 2.1 `TestRevokedPermitCannotBeRestored`

```text
=== RUN   TestRevokedPermitCannotBeRestored
--- PASS: TestRevokedPermitCannotBeRestored (0.83s)
```

- Seed `REVOKED` permit；`AdmitRefund` 返回 `PRE`，`assertPermitState` 仍 `REVOKED`；`BlockShipmentForRefund` 后仍 `REVOKED`；`AuthorizeShipment` 返回 error 后仍 `REVOKED`。
- **不是只断言“返回错误”**：每次都读回 DB permit state（`assertPermitState`），证明三条公共路径没有改回 `NONE/GRANTED`。
- **局限**：`AuthorizeShipment` 只断言 error 非 nil，未断言 error 是“revoked permit”分支（可能是别的失败）；`tcc_settlement`/`legacy_resolution_worker` 的 permit 写路径未被本测试执行（由 §3 grep 覆盖）。NB-2/NB-3。

### 2.2 `TestRejectedAdmissionWritesNoCommitment`

```text
=== RUN   TestRejectedAdmissionWritesNoCommitment
--- PASS: TestRejectedAdmissionWritesNoCommitment (0.76s)
```

- over-amount 20000 + partial unshipped 5000 都返回 error；`order_refund_admissions`/`order_refund_commitments`（按 order）0 行；`outbox_events`（按 `aggregate_id='ORD-T33'`）0 行；permit 仍 `NONE`。
- **不是只断言错误**：断言 admissions/commitments/outbox 零行 + permit state。
- **局限**：outbox 只按 `aggregate_id='ORD-T33'` 过滤；若拒绝路径用别的 aggregate（如 refund_id/group）写事件，本断言会漏（NB-4）。partial 只覆盖 `permit NONE` 分支；`permit CONSUMED + partial unshipped` 的另一条 `partialRefundError` 分支未测（NB-5）。

---

## 3. T3.1 写路径审计（grep 全 order 包）

`grep` 结果（生产代码，排除 `_test.go`）：

| 位置 | 写入 | 状态迁移 | 恢复风险 |
|---|---|---|---|
| `settlement.go:289` | `INSERT order_shipping_permits` | 新建 `NONE` | 无（新行） |
| `settlement.go:398` | `UPDATE ... SET state = CONSUMED` | `NONE/GRANTED → CONSUMED`（`REVOKED` 在 switch 提前 error） | **无** |
| `settlement.go:443` | `UPDATE ... SET state = nextState` | `NONE/GRANTED → REVOKED`；`CONSUMED → CONSUMED` | **无**（注意审计写“仅 NONE→REVOKED”，漏了 GRANTED→REVOKED，NB-1） |
| `refund_admission.go:567` | `UPDATE ... SET state = REVOKED WHERE state = NONE` | `NONE → REVOKED`（仅 PRE） | **无** |
| `refund_admission.go:829` | `UPDATE ... SET state = REVOKED WHERE state = NONE` | `NONE → REVOKED`（zero 路径；审计行号未列，NB-1） | **无** |
| `tcc_settlement.go:748` | `UPDATE ... SET state = REVOKED WHERE state = NONE` | `NONE → REVOKED` | **无** |
| `legacy_resolution_worker.go:248` | `UPDATE ... SET state = REVOKED WHERE state = NONE` | `NONE → REVOKED` | **无** |

- 生产代码无 `DELETE FROM order_shipping_permits`（仅 `tcc_compensation_integration_test.go` 测试有）；无 `SET state = NONE/GRANTED` from `REVOKED`。
- **结论：T3.1 无恢复路径。** 审计的“没有代码路径把 REVOKED 写回 NONE/GRANTED”正确；但审计的枚举/行号有偏差（NB-1）。

---

## 4. T3.3 拒绝路径是否在 Tx1 前

`AdmitRefund`（`refund_admission.go:450-545`）的实际顺序：

1. `db.WithTx` 打开 Tx1 → `findRefundAdmissionLocked`（锁 admission）→ `ensureGroupExistsTx` → `ensurePermitTx`（**permit 缺失时会 INSERT 一行 NONE**）。
2. 计算 `reserved`、`captured-refunded`、`groupRemaining`；`amount+reserved > captured-refunded` → error；`amount > groupRemaining` → error；`permit != CONSUMED && amount != groupRemaining` → `partialRefundError`。
3. 只有通过后才 `INSERT order_refund_admissions` / `order_refund_commitments` / `outbox` / permit CAS。

因此：

- **不是“Tx1 之前”**；是 **Tx1 内、admission/commitment/outbox 写入之前**。`ensurePermitTx` 可能先插 `NONE` permit（错误时随 Tx1 回滚）；本测试 seed 了 permit，所以没有触发该 insert。审计的“在写 Tx1 前”措辞不准确（NB-1）。
- **对 invariant 而言满足**：拒绝路径不写 admission/commitment/outbox；测试实测零行；permit 仍 NONE。
- **partial 分支**：`permit NONE` 的 partial 由第 2 步 `amount != groupRemaining` 拦截；`permit CONSUMED + partial unshipped` 由后续 `case partial && exitClass == exitUnshipped` 拦截，也在写入前；后者未被测试（NB-5）。

---

## 5. T3.2/T3.4/T3.5/T3.6 缺口判断

| 任务 | 审计说法 | 复核 |
|---|---|---|
| T3.2 | 未实现；无 `RELEASED` 写入路径；无退款 release RPC | **准确**：`grep RELEASED` 只命中测试/其他 projection；proto 的 `Release` RPC 属 inventory，不是退款 release。 |
| T3.4 | proto/`0018` 列已存在；`AdmitRefund` 未用 lines/responsibility/freight | **准确**：`mall.proto:531-542` 有 `RefundLine lines/responsibility/reason_code/freight_refundable_cent`；`0018:63` 有 `order_refund_commitment_lines`；`refund_admission.go` 对 `GetLines/GetResponsibility/GetFreightRefundableCent` **零命中**。 |
| T3.5 | 当前只有 amount/group 级；无行层 INV-24、Σ、Payment 三层 | **准确**：`AdmitRefund` 只按 `amount_cent/reserved/groupRemaining` 校验；无 line composition / responsibility / freight 落库。 |
| T3.6 | AfterSales 责任未接线；Order 不读 responsibility | **准确**：`AdmitRefund` 不读 `responsibility`；无 AfterSales 调用。 |
| T3.7 | T3.1/T3.3 已补；T3.2/T3.4/T3.5/T3.6 无测试 | **准确**；T3.7 仍 `[ ]`，见 tasks。 |

审计的下一刀建议（T3.4+T3.5 先 Plan、T3.2 独立契约、T3.6 后接）与缺口/依赖一致；**需 Plan/人类批准**，不在本轮实现。

---

## 6. 边界

- `git diff 7d103a5 -- 'internal/services/order/*.go'` = **0 行**（新增测试未跟踪，无内部生产代码改动）。
- `git diff 7d103a5 -- migrations api docs/specs` = **0 行**；无 proto/migration/契约改动。
- `tasks.md` 只把 T3.1/T3.3 改 `[x]`（带测试证据）；T3.2/T3.4/T3.5/T3.6/T3.7 保持 `[ ]`。
- 新增 `t3-audit.md` 末尾有字面 `EOF`（heredoc 残留，NB-6）；无代码/生成物改动。

---

## 7. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | 审计写路径枚举/行号不精确：`BlockShipmentForRefund` 还有 `GRANTED→REVOKED`；`refund_admission.go:829` 的 zero UPDATE 未列；行号 `:742-753` 与 `:829` 不符；“Tx1 前”应改为“Tx1 内、admission/commitment/outbox 写入前” | `settlement.go:429-443`；`refund_admission.go:829`；`t3-audit.md:12-29` |
| NB-2 | `AuthorizeShipment` 只断言 error 非 nil，未断言“revoked permit”分支/错误码；若因其他原因失败，测试仍 PASS | `t3_invariants_integration_test.go:43-49` |
| NB-3 | tcc/worker 的 permit 写路径未被测试执行，仅 grep/代码审计 | `tcc_settlement.go:744-758`；`legacy_resolution_worker.go:246-252` |
| NB-4 | T3.3 outbox 断言只按 `aggregate_id='ORD-T33'`；若拒绝路径写其他 aggregate，测试漏 | `t3_invariants_integration_test.go:79-81` |
| NB-5 | `permit CONSUMED + partial unshipped` 分支未测试；当前只覆盖 `permit NONE` 的 partial | `refund_admission.go:507-514` |
| NB-6 | `t3-audit.md` 末尾字面 `EOF`；审计 line refs 需校正 | `t3-audit.md:61` |

---

## 8. 裁决

**SCOPED-PASS。** T3.1/T3.3 不变量满足并测试（公共路径）；全包 permit 写路径 grep 无恢复路径；T3.3 拒绝路径在 Tx1 内写前、零 admission/commitment/outbox；T3.2/T3.4/T3.5/T3.6 缺口判断准确，需 **Plan/人类批准**；T3.7/T5 未完成；**AC-60/AC-63 仍未验证；NOT RELEASE**。

未覆盖/待办：tcc/worker permit 路径未测试（grep 审计）、CONSUMED partial 分支未测试、outbox 断言范围、审计措辞/行号/`EOF`；这些是 NB，不阻断 T3.1/T3.3 的 invariant 结论。

---

## 9. 复现命令（只读）

```bash
git rev-parse HEAD   # 7d103a5...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestRevokedPermitCannotBeRestored|TestRejectedAdmissionWritesNoCommitment' -count=1 -v
grep -rn 'UPDATE order_shipping_permits\|INSERT INTO order_shipping_permits\|DELETE FROM order_shipping_permits' internal/services/order/*.go
git diff 7d103a5 -- 'internal/services/order/*.go' | wc -l          # 0
git diff 7d103a5 -- migrations api docs/specs | wc -l               # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 SCOPED-PASS 裁决。未改代码/测试/契约/迁移；未 commit、未 merge。
