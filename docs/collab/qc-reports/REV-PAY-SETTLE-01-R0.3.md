# REV-PAY-SETTLE-01-R0.3 — AC-27 / INV-45..50 台账收口

| 项 | 值 |
|---|---|
| review_scope | R0.3：AC-27 台账行与真实测试；INV-45..50 evidence-matrix 核对；回归 AC-14/21/25/27/31/32 与 12 orphan 台账。只读代码/契约；仅更新 Reviewer-owned `docs/collab/ac-coverage.tsv`。不批准 merge/Release。 |
| HEAD | `ba435dd`（当前 clean；R0.1 code `42a8db5`、evidence `aaef9f9`） |
| **verdict date** | **2026-11-25** |
| Actor / Reviewed-by | Reviewer `reviewer-phase1`；台账 Owner=Reviewer；与 Developer/Architect/PO 不同 |
| **overall verdict** | **FAIL（R0.3 有范围）**：AC-27 行已补为 **AC-REF**（非 AC-VERIFIED）；B3 INV-45..50 台账/evidence 仍有缺口；回归与 12 orphan 未全部关闭。 |
| release_status | **NOT RELEASE**；不批准合并 |

---

## 1. AC-27（B2）

### 1.1 测试执行（绑定 HEAD）

```bash
go test -race ./internal/services/order -run '^TestAC27RevokedGroupIsClosedInProjection$' -count=1 -v
```

- HEAD：`ba435dd`
- 结果：**PASS**，1.218s（`TestAC27RevokedGroupIsClosedInProjection` 0.00s + 编译）
- 测试文件：`internal/services/order/warehouse_quote_test.go:123`
- 断言：G1 SHIPPED、G2 ALLOCATED+permit REVOKED → `shipmentStatusForGroups=SHIPPED`、`aggregateOrderStatus=SHIPPED`、`effectiveGroupStatus(G2)=CANCELLED`。

### 1.2 AC-27 覆盖判定：AC-REF，不是 AC-VERIFIED

AC-27 v1.4 有三段 EARS（`docs/specs/0001-fulfillment-quantity-split.md:571-579`）：

1. **G2 已退款完成 + G1 正常发货 → 按 G1 推进、不阻塞**：测试覆盖投影函数，部分覆盖。
2. **WHILE G2 `refund_state=REFUNDED` 且许可 `REVOKED` 或运单 `CANCELLED`**：测试只有 `PermitState=REVOKED`，**未设置/断言 `refund_state=REFUNDED`**。
3. **G2 预留已释放或运单已取消 → §6.6/§6.6.1 判定合法**：测试没有 reservation/waybill 状态，只验证 permit REVOKED 的读模型。

因此测试虽然 PASS，但 **AC-27 的 `refund_state=REFUNDED` 与 reservation/waybill 分支未被断言**；按“PASS 才可 AC-VERIFIED”的严格口径，本轮标 **AC-REF**，不把投影单测当完整 AC-27。

### 1.3 台账行（已更新 `ac-coverage.tsv`）

```
AC-27	Developer	2026-12-31	AC-27 projection test passes; EARS refund_state=REFUNDED and reservation/waybill branches not covered	developer-reviewer/T-003	unit	internal/services/order/warehouse_quote_test.go:123 TestAC27RevokedGroupIsClosedInProjection	AC-REF
```

- Owner **Developer**（测试 owner），Reviewer 保留台账所有权。
- Expiry **2026-12-31**；status **AC-REF**，不伪造 AC-VERIFIED。
- 剩余动作：补 `refund_state=REFUNDED`、reservation/waybill CANCELLED 分支的真实断言（Owner Developer，expiry 2026-12-31）；补齐后可重审为 AC-VERIFIED。

---

## 2. B3 — INV-45..50 evidence-matrix

`docs/lead/PAY-SETTLE-01/evidence-matrix.md` 现有行均含 **owner/evidence/status**；DEFERRED/AC-REF 未被写成 VERIFIED。但存在以下具体缺口：

| INV | 行现状 | 缺口 / 与 SPEC v1.10 不一致 |
|---|---|---|
| **INV-45** | Developer / `0018` 表约束 + T3 planned / **DEFERRED（T3）** | 描述“释放仅人工授权/审计”**漏掉** SPEC 允许的“打款前 REJECTED/CANCELLED”释放；`0018` 是 schema 证据，不是行为测试；无 release 行为 evidence。 |
| **INV-46** | Developer / T2 planned / **DEFERRED（T2）** | 描述漏掉“有捕获无离仓→COMPENSATING、有离仓→EXPIRED_FINANCE、补偿失败保持”；无 backfill/fault evidence。 |
| **INV-47** | Developer / T3 planned / **DEFERRED（T3）** | 描述漏掉“后置补退同组、累计退运费 ≤ 用户实付运费”；无行级/运费测试。 |
| **INV-48** | Developer / `t17_evidence_integration_test.go` test-only PASS / **AC-REF（未 AC-VERIFIED）** | T17 六场景 PASS，但无 AC-VERIFIED、无 T5 `-race`/fault 全矩阵；状态保守，正确。 |
| **INV-49** | Developer / T4.4 planned / **DEFERRED（T4）** | 描述漏掉“shadow 失败只告警、`order.paid` 切换后不得再作准入依据”；无 shadow 测试。 |
| **INV-50** | Developer / “T1.4b2b zero tests；`tcc_admission_integration_test.go`” / **AC-REF** | evidence path 指向 `tcc_admission_integration_test.go`，不是 zero 专用 `aftersales/zero_refund_integration_test.go`；描述漏掉“不得写 `orders.status=REFUNDED`、不得正金额”。 |

- **B3 判定**：台账行存在且状态诚实（DEFERRED/AC-REF，无 VERIFIED），但 **INV-45/46/47/49 仍是 DEFERRED**，**INV-48/50 AC-REF**，且上述 evidence/描述缺口未闭合；**B3 未收口**。不得把 DEFERRED 写成 VERIFIED；不批量续期。
- 剩余 owner/expiry：INV-45/47 T3（Developer，**2026-12-31**）；INV-46 T2（Developer，**2026-12-31**）；INV-49 T4.4（Developer，**2026-12-31**）；INV-48/50 T5（Developer/Reviewer，**2026-12-31**）；evidence path 修正与描述补齐后再 review。

---

## 3. 回归 AC 与 12 orphan

- 本轮已向 `ac-coverage.tsv` 添加 **AC-14/21/25/31/32** 行，均 **DEFERRED**，owner Developer，expiry **2026-12-31**，reason “R0.3 baseline regression not run; owner/expiry retained”，thread `developer-reviewer/T-003`，evidence `-`。**未跑**这些基线回归，不能写 AC-VERIFIED。
- **AC-27** 行 AC-REF（见 §1）。
- **12 orphan**：`AC-39/42/43/47/48/49/56/57/58/59/62/64` 现有行保持原 owner/expiry（Developer/2026-12-31，reason “orphan (no WP)”）；**未删除、未批量续期、未关闭**。本轮没有足够证据决定去留，保留为剩余项。
- **不得**把上述 DEFERRED 行写成 VERIFIED；`ac-coverage.tsv` 只记录真实状态。

---

## 4. 测试 / 台账证据

| 项目 | 结果 |
|---|---|
| AC-27 test at HEAD `ba435dd` | **PASS**（`-race`，1.218s） |
| AC-27 台账 | 已新增 **AC-REF** row（owner/expiry/test_type/evidence_path/status 完整） |
| AC-14/21/25/31/32 | 已新增 **DEFERRED** rows（未跑回归） |
| 12 orphan | 原样保留，未批量续期；owner/expiry 已存在 |
| INV-45..50 | 行有 owner/evidence/status；**未 VERIFIED**；缺口见 §2 |
| AC-VERIFIED | **未新增任何 AC-VERIFIED** |

---

## 5. 声明

- 本报告只更新 Reviewer-owned `docs/collab/ac-coverage.tsv`；未改代码/契约/其他台账。
- **R0.3 overall FAIL**：AC-27 仍 AC-REF（测试 PASS 但 EARS 分支未全覆盖）；B3 INV-45/46/47/49 DEFERRED、INV-48/50 AC-REF 且 evidence/描述有缺口；回归 AC-14/21/25/31/32 DEFERRED；12 orphan 未关闭。剩余 owner/expiry 已列。
- **不是 Release PASS；不批准 merge；不替代人类 merge 决策。**
- 日期 **2026-11-25**；未把 2026-11-24 的批准倒签为 VERIFIED。

报告：[REV-PAY-SETTLE-01-R0.3.md](docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.3.md)
