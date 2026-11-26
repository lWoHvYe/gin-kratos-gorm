# REV-PAY-SETTLE-01-R0.3-R1c — AC-27A 第 5/6 条读模型关闭谓词 scoped 复审

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| Tier | **L2（确认）**：Order 读模型/退款准入/组关闭谓词；跨 L2 handback；不适用 L1 |
| review_scope | **R0.3-R1c**：AC-27A 第 5/6 条 `groupIsClosed`/`PreShipAdmission`/3 组边界/late 离仓守卫/`orders.status` 不写；不重开 L2 T0–T8，不审 T2/T3/T4.4/T5 |
| base | **`ce0f6b4`**（Lead resume R0.3-R1c） |
| code head | **`9a57d012254fcad8c34d9af7c48b2aa398c36ccb`**（4 files，+503） |
| head（复审绑定） | **`36b747a93cbf441d2f7c2532b00844693b36f58f`**（selfcheck §9） |
| 契约输入 | SPEC-0001 **v1.5** AC-27/AC-27A + §16.2；Architect AC-27 裁决 DRAFT 0.1（R-1..R-5；R-2 与 v1.5 冲突）；L2 `PAY-AC50-REVIEW-01` final（AC-50 AC-VERIFIED） |
| integration_owner | Order（读模型）；Architect（R-1..R-5）；PO（AC-27/27A 正文） |
| INV | **INV-45**（permit/commitment）、**INV-44**（释放归 Inventory）、**INV-01/24/38/50**（生命周期/额度/零元边界）；INV-51/52 由 L2 维持 |
| verification | §1：MySQL 8.4.13 独立 schema；order `TestAC27|TestAC25|TestT17` plain 9.133s / `-race` 9.994s；selfcheck 全 order 116.705s / 121.013s；`migrations` diff 空 |
| 日期 / Actor / Reviewed-by | 2026-11-27 / `reviewer-r04` / `human (merge pending)` |
| release_status | **NOT RELEASE** |
| overall verdict | **PASS（R0.3-R1c scoped）**；AC-27 仍 **AC-REF**（NOWB/LIVE 未闭）；late SHIPPED/RECEIVED 无契约冲突；不声称 Release/merge |

---

## 0. 结论

**PASS（R0.3-R1c scoped）。** `9a57d01` 在真实 MySQL 8.4.13、独立 schema 上重跑 `TestAC27|TestAC25|TestT17` plain **9.133s**、`-race` **9.994s**，全部 PASS；selfcheck 的 order 全包 116.705s / `-race` 121.013s 亦 PASS；`migrations` diff 为空，生产 diff 仅 `settlement.go`/`fulfillment_groups.go` + 测试。

实现符合 SPEC-0001 v1.5：
- `groupIsClosed` 固定顺序（nil → CANCELLED → SHIPPED/RECEIVED 离仓守卫 → REVOKED → CONSUMED+PRE → open）；
- `PreShipAdmission` 由 `EXISTS(order_refund_admissions result='PRE_SHIP_REVOKED')` 派生，**只用 Order 自有事实**，非持久字段；
- 第 5 支按 AC-27A 第 5 条同时要求 `permit=CONSUMED`；
- `refund_state`/`reservation_state` 不参与；
- 无 `orders.status` 写入；
- 3 组边界与 late SHIPPED/RECEIVED 守卫有测试。

**AC-27 不能升级为 AC-VERIFIED**：AC-27-NOWB-01 / AC-27-LIVE-01 仍开；AC-50-FIX-01 的写路径已由 L2 交付（AC-50 AC-VERIFIED），但不足以关闭全部前置。AC-27 保持 **AC-REF**。

---

## 1. 实际执行（HEAD 36b747a / code 9a57d01）

```bash
# MySQL 8.4.13，独立 schema
TEST_MYSQL_DSN=.../mall_integration_r1c_order go test ./internal/services/order -run 'TestAC27|TestAC25|TestT17' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_r1c_order_race go test -race ./internal/services/order -run 'TestAC27|TestAC25|TestT17' -count=1
git diff --stat 9a57d01^..9a57d01 -- migrations
```

| 命令 | 结果 |
|---|---|
| order `TestAC27|TestAC25|TestT17` | **PASS 9.133s**；AC-27A 新 5 个用例全 PASS；T17 六场景 PASS |
| `-race` 同 | **PASS 9.994s** |
| selfcheck 全 order | **PASS 116.705s**；`-race` 121.013s（0 SKIP） |
| `migrations` diff | **空** |
| 生产 diff | 仅 `fulfillment_groups.go`、`settlement.go`；无 `UPDATE orders` |

原始日志：`.tmp/evidence-r1c/reviewer-r1c.log`（本次审查生成，未入库）。

---

## 2. AC-27A 谓词逐条

| # | 条件 | 实现/测试 | 判定 |
|---|---|---|---|
| 1 | 组不存在 | `groupIsClosed(nil)=false`；`TestAC27AClosurePredicateOrder/missing_group_is_open` | **PASS** |
| 2 | 持久化 `CANCELLED` | 第 2 顺序返回 true；`persisted_CANCELLED_closes` / `CANCELLED_wins_over_a_pre-ship_admission` | **PASS** |
| 3 | `SHIPPED/RECEIVED` 离仓守卫先于退款事实 | 第 3 顺序返回 false；`SHIPPED_guard...` / `RECEIVED_guard...`；late `TestAC27ALateShippedStatusWinsOverPreAdmission` | **PASS** |
| 4 | permit `REVOKED` | 第 4 顺序 true；`permit_REVOKED_closes_an_allocated_group` | **PASS** |
| 5 | permit `CONSUMED` 且 `PreShipAdmission` | 第 5 顺序 `permitConsumed && PreShipAdmission`；`CONSUMED_plus_pre-ship_admission_closes`；`pre-ship_admission_without_a_CONSUMED_permit_stays_open` | **PASS** |
| 6 | 其他（NONE/PENDING_REMOTE/REVIEW/POST/缺行） | 第 6 顺序 false；`CONSUMED_without...`/`permit_NONE`/`REVIEW`/`PENDING`/`POST`/`no_admission_row` 子用例 | **PASS** |
| — | `PreShipAdmission` 载体 | `fulfillment_groups.go` 同一 SELECT `EXISTS(... result='PRE_SHIP_REVOKED')`；`refund_result` 参数非字面量；无 JOIN 放大 | **PASS** |
| — | `refund_state`/`reservation_state` 不参与 | `TestAC27AClosureIgnoresRefundStateAndReservationState`（NONE/REFUNDING/REFUNDED × HELD/RELEASED） | **PASS** |
| — | 无 `orders.status` 写入 | 生产 diff 无 `UPDATE orders`；集成测试前后均 `ALLOCATED` | **PASS** |
| — | 3 组边界 | `TestAC27AThreeGroupBoundary`：G3 未关闭→`PARTIALLY_SHIPPED`；G3 关闭→`SHIPPED`，`orders.status` 仍 `ALLOCATED` | **PASS** |
| — | CANCELLED 生命周期共存 | `TestAC27AClosedGroupKeepsCancelledLifecycle` | **PASS** |

---

## 3. 迟到 SHIPPED/RECEIVED 的裁定

**无契约冲突，不需要 `[BLOCKED]`/PO 裁决。** AC-27A 的固定顺序与 late 子句已经给出条件规则：

- **第 3 条离仓守卫优先**：S3 组投影已是 `SHIPPED/RECEIVED` → 读模型 `open`（`TestAC27ALateShippedStatusWinsOverPreAdmission`、`TestAC27AAdmissionResultDerivation/late_shipped...`）。
- **无法判定/无事件证据/乱序**：S3 仍 `ALLOCATED` 且只有 PRE 准入 → 保持 **closed**（`TestAC27AClosurePredicateOrder/CONSUMED_plus_pre-ship_admission_closes`；`0001:626-631`）。这正是“不得把已关闭组重新打开”的适用场景；不冲突。
- 读模型不写回 PRE/运单/`orders.status`，不制造 S6 事实。
- 残余：confirmed late S3 事件打开读模型时，AC-27A 要求“并告警”；本次读路径未新增告警（selfcheck 说明由 AC-50 写路径 log/metric 承担）。标 **[MINOR]/T7 残余**，不阻断本谓词。

---

## 4. AC-27 台账与前置缺陷

- **AC-50-FIX-01**：L2 `PAY-AC50-REVIEW-01` final/AC-50 行已交付写路径（AC-50 **AC-VERIFIED**）；本行不再重复。
- **AC-27-NOWB-01**：仍开（存量 PRE 一次性审计/`CONSUMED+无运单` 人工核对终态）；保持 **AC-REF**。
- **AC-27-LIVE-01**：仍开（`PENDING_REMOTE` 永久悬挂/DLQ/AC-61 人工重放）；保持 **AC-REF**。
- **AC-27**：不升级 **AC-VERIFIED**。`ac-coverage.tsv` AC-27 仍 **AC-REF**，reason/evidence 更新为 R0.3-R1c 已完成但 NOWB/LIVE 未闭；owner/expiry/thread 不变（Developer/2026-12-31/review-tracking/PAY-SETTLE-01）。
- **AC-27A 第 6 条告警**：读路径不新增；由 AC-50 写路径 Error/metric 承担；`AC-27-NOWB-01`/`AC-27-LIVE-01` 仍是人工队列/审计项。

---

## 5. INV / 宪法 / Governance

| 项 | 结论 | 依据 |
|---|---|---|
| INV-45 | **PASS** | 关闭用 permit 终态/PRE，不改 permit、不放 commitment |
| INV-44 | **PASS（边界）** | 不要求 Order 观测 `RELEASED`；多组释放缺口仍 DEFERRED |
| INV-01/24/38/50 | **PASS（读模型）** | 无 `orders.status` 写；生命周期/BFF 投影不变；零元不参与 |
| I.1 拒绝“屎山” | **PASS** | 单一 `groupIsClosed` 固定顺序；无 call-site 散落分支 |
| I.2 Fail-Closed | **PASS** | PENDING/REVIEW/POST/缺行保持 open；无提前关闭 |
| I.3 契约优先 | **PASS** | 按 SPEC v1.5；R-2 DRAFT 与 v1.5 冲突，以 SPEC 为准，未实现 `orders.status` 写入 |
| I.4 拒绝妥协 | **PASS** | R-2 未实现被明确标注，不用代码静默迁就 DRAFT |
| II.1 事实分离 | **PASS** | 只用 S5/RefundAdmission/S3；不引入 S6 运单事实 |
| II.3 幂等/最终一致 | **PASS** | 纯读派生、每次重算；读与 Tx2 只可能滞后关闭 |
| III.2/III.4 | **PASS** | database/sql；无迁移/新字段/worker |
| IV.1/IV.2 | **PASS** | 真实 MySQL 8.4.13；plain/`-race`；T17 回归 |
| IV.4 | **PASS** | `NOT RELEASE`；不声称 Release |
| Workflow 9 / Governance | **PASS** | L2 handback 后只做 R0.3-R1c；人类 merge pending |

**`[ARCHITECTURAL_FLAW]`：无。** Architect R-2（在退款完成 consumer 重聚合 `orders.status`）与 SPEC v1.5 AC-27 EARS-1（只读模型、不写 `orders.status`）冲突；以已批准 SPEC 为准，R-2 **不得实现**，建议 Architect 撤回/errata DRAFT 0.1 R-2。此非代码缺陷。

---

## 6. 结论 / 残余

- **R0.3-R1c scoped PASS**：AC-27A 第 5/6 条、3 组边界、late 守卫、无 `orders.status` 写均真实执行并通过。
- **AC-27 保持 AC-REF**，不升级；`AC-27-NOWB-01`/`AC-27-LIVE-01` 未闭；`AC-50-FIX-01` 写路径由 L2 交付。
- 残余：late confirmed 告警未在读取路径新增（AC-27A late 子句的告警由写路径/AC-27-NOWB 承担，标 MINOR）；Architect DRAFT R-2 需撤回/errata；多组 INV-44 释放缺口不变；Phase 0-R 未闭合（T2/T3/T4.4/T5）。
- **NOT RELEASE / 不 merge / 不声称 P0 closure**；`PAY-SETTLE-01` 仍冻结。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-27
**报告**：`docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.3-R1c.md`
