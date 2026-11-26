# REV-PAY-AC50-REVIEW-01-FINAL — T0–T8 最终 scoped 裁决

| 项 | 值 |
|---|---|
| change_id | `PAY-AC50-REVIEW-01`（Parent `PAY-SETTLE-01` 冻结） |
| Tier | **L2（确认）**：TCC/退款准入、跨服务 Order/AfterSales/Fulfillment、资金门禁、迁移/台账；不适用 L1 |
| review_scope | **T0–T8**：T0/T0b/T0c、T1/T2、T3–T6、T7 台账、T8 回归；不审 PAY-SETTLE-01 的 T2/T3/T4.4/T5，不声称 Release |
| base | **`3ce0d7d`**（T3–T6 re-review PASS；freeze lifted for T7/T8） |
| code head | **`3ce0d7d1cdc8bd36eb65db7cbe36e11736fedd62`**（代码在 T7/T8 未变；`git diff 3ce0d7d..HEAD -- ':(exclude)docs'` 为空） |
| head（selfcheck / T7-T8） | **`c8f6c9cbf26c23808a69fc81cfdb7a666b1e8d04`**（selfcheck `c8f6c9c`） |
| 契约输入 | SPEC-0002 v1.11 AC-50/51/61/62/65/67/69–72；ADR 0.6（ACCEPTED）；impact 0.4；Plan/Tasks P5 后；T8 证据 §13 |
| integration_owner | System Architect（契约）；Order（T0–T6）；AfterSales（T2/T4）；Fulfillment（T0/T6b）；ops（T5c/T7 告警/入口） |
| INV | **INV-45**（AC-REF；T3 release writer deferred）、**INV-51**（AC-VERIFIED，T0–T6）、**INV-52**（AC-VERIFIED，T3 partial）、**INV-44**（AC-REF；多组/missing metric deferred）、**INV-46**（DEFERRED T2）、**INV-50**（AC-REF；零元 AC-72 告警段 deferred） |
| affected_AC | AC-50/51/61/62/65/67（修订）、AC-69–72（新增）；AC-72 三段；**AC-50/69/70/71/72-backend AC-VERIFIED；其余 AC-REF/DEFERRED** |
| verification | §2：MySQL 8.4.13 + NATS 2.10.29、独立 schema；order 38/38 + `-race`、aftersales 16/16 + `-race`、fulfillment 4/4、metrics 1/1（selfcheck T8）；本报告重跑关键集合全 PASS；`migrations` diff 空 |
| 日期 / Actor / Reviewed-by | 2026-11-27 / `reviewer-r04` / `human (merge pending)` |
| release_status | **NOT RELEASE** |
| overall verdict | **PASS（T0–T8 scoped）**；无代码 blocker；T7 台账已更新；不声称 Release/merge/PAY-SETTLE-01 closure |

---

## 0. 结论

**PASS（T0–T8 scoped）。** T0–T6 的实现与 T3-R1 修复已在精确 base/head 上验证；T8 回归在真实 MySQL 8.4.13 + NATS 2.10.29、独立 schema 上 **0 FAIL / 0 SKIP**（order 38/38、order-inv 20/20、aftersales 16/16、fulfillment 4/4、metrics 1/1；`-race` order 38/38、aftersales 16/16）。本报告另重跑关键集合：order 34.500s、`-race` 40.685s；aftersales 9.400s、`-race` 10.181s；fulfillment 1.892s；metrics 0.087s；真实 JetStream 2.141s。`migrations` diff 为空；T3–T6 后代码未再改。

T7 台账已更新：`ac-baseline.txt` 增 AC-69…72；`ac-coverage.tsv` 更新 AC-50/51/61/62/65/67，新增 AC-69–72（AC-72 三段）。**AC-50/69/70/71/72-backend 为 AC-VERIFIED；AC-51/61/62/65/67 为 AC-REF；AC-72 告警/入口为 DEFERRED；AC-21/25/27/31/32 保持 AC-REF；AC-14 维持既有 AC-VERIFIED。** 这是实现/AC 证据的 scoped PASS，不是 Release PASS。

---

## 1. 关键证据与最终状态

| AC | 状态 | 证据 / 边界 |
|---|---|---|
| AC-50 | **AC-VERIFIED** | AC-50 全分支（NO_WAYBILL/partial/legacy/zero/REVIEW）在 `ac50_review_integration_test.go`、audit/fault 测试；T8 at 3ce0d7d |
| AC-51 | **AC-REF** | TCC Σ/逐组/REVIEW tests PASS；完整 AC-51 分支矩阵/T3 release 未穷尽 |
| AC-61 | **AC-REF** | runbook/query + 人工队列；5 分钟告警部署 deferred，非 AC-VERIFIED |
| AC-62 | **AC-REF** | REVIEW 保持 commitment RESERVED；D-24 人工释放/T3 未实现 |
| AC-65 | **AC-REF** | TCC Σ/MANUAL_HOLD/Tx2 tests PASS；完整终态/打款矩阵未穷尽 |
| AC-67 | **AC-REF** | 零元 NO_WAYBILL 不持久化、CANCELLED→PRE+DONE；AC-72 零元计数/告警段 deferred |
| AC-69 | **AC-VERIFIED（后端）** | REVIEW 持久化/幂等/审计/re-arm + TCC/AfterSales 镜像；O2/O2b/O2c/O19/O20/O23 |
| AC-70 | **AC-VERIFIED** | O4a–O4e/O5c/O10/O10b；fresh/preseed partial、legacy |
| AC-71 | **AC-VERIFIED** | O1b/W12/O16 + T3-R1 fresh gate；MANUAL_HOLD/0 打款/稳定 token |
| AC-72（backend） | **AC-VERIFIED（仅后端段）** | gauge/7d/零元不混入/runbook；**AC-72 整体不 VERIFIED** |
| `REVIEW-REQUIRED-ALERTS-01` | **DEFERRED** | 5 分钟告警规则/值班部署未交付；owner ops，2026-12-31 |
| `REVIEW-REQUIRED-ENTRY-01` | **DEFERRED** | 用户可见文案/人工入口未交付；owner Lead/BFF/ops，2026-12-31 |
| AC-21/25/27/31/32 | **AC-REF（保持）** | 既有通知/全场景/可见性限制；AC-27A 5/6、AC-27-NOWB/LIVE 仍开 |

`ac-baseline.txt`：AC-69/70/71/72 已追加（AC-72 单条；tsv 三段）。`ac-coverage.tsv`：既有行保留、未删/未重编号；更新/新增行 thread=`review-tracking/PAY-AC50-REVIEW-01`，owner/expiry 维持 Developer/2026-12-31（ALERTS/ENTRY 为 ops、Lead/BFF/ops）。

---

## 2. 验证（HEAD c8f6c9c / code 3ce0d7d）

```bash
# MySQL 8.4.13 独立 schema；NATS 2.10.29
TEST_MYSQL_DSN=$ORDER_DSN go test ./internal/services/order -run 'TestAC25|TestAC27|TestT17|TestTCCReview|TestAC50Review' -count=1
TEST_MYSQL_DSN=$ORDER_DSN go test -race ./internal/services/order -run 'TestAC25|TestAC27|TestT17|TestTCCReview|TestAC50Review' -count=1
TEST_MYSQL_DSN=$AS_DSN go test ./internal/services/aftersales -run 'TestAC14|TestAC31|TestAC32|TestZero|TestReview' -count=1
TEST_MYSQL_DSN=$AS_DSN go test -race ./internal/services/aftersales -run 'TestAC14|TestAC31|TestAC32|TestZero|TestReview' -count=1
TEST_MYSQL_DSN=$FF_DSN go test ./internal/services/fulfillment -run 'TestAC21|TestCancelIfCreated' -count=1
go test ./internal/platform/metrics -count=1
TEST_MYSQL_DSN=$FF_DSN TEST_NATS_URL=nats://127.0.0.1:4222 go test ./internal/services/fulfillment -run 'TestRefundAdmittedConsumerReviewAndDuplicate' -count=1 -v
git diff --stat 3ce0d7d..HEAD -- migrations
```

| 项 | 结果 |
|---|---|
| order 目标 | **PASS 34.500s**；`-race` **PASS 40.685s** |
| aftersales 目标 | **PASS 9.400s**；`-race` **PASS 10.181s** |
| fulfillment 目标 | **PASS 1.892s** |
| metrics | **PASS 0.087s** |
| 真实 JetStream | **PASS 2.141s** |
| selfcheck T8 | **order 38/38、order-inv 20/20、aftersales 16/16、fulfillment 4/4、metrics 1/1；0 FAIL/SKIP** |
| `migrations` diff | **空**；T3–T6 后代码无变化 |

原始日志：`.tmp/evidence-ac50-final/reviewer-final-regression.log`、`reviewer-final-nats.log`（本次审查生成，未入库）。

---

## 3. INV/AC-72 三段

- **INV-51**：REVIEW 唯一载体/分路径镜像、自动不可收敛、replay 幂等、审计/re-arm；T0–T6 测试。**AC-VERIFIED**。
- **INV-52**：partial 不调用 `CancelIfCreated`；T3 O4a–O4e/O5c/O10b、`cancelCalls==0`。**AC-VERIFIED**。
- **INV-45**：REVIEW 保持 permit/commitment；T3 人工 release writer 未实现，**AC-REF**。
- **INV-44**：单组 release 既有；多组/missing-reservation metric **DEFERRED** → **AC-REF**。
- **INV-46**：24h CHECK 仅 DB 兜底；writer/worker/source 仍 T2 → **DEFERRED**。
- **INV-50**：零元不调 Payment/REVIEW、CANCELLED→PRE+DONE 已测；AC-72 零元告警段 deferred → **AC-REF**。
- **AC-72 三段**：backend `AC-72`（AC-VERIFIED，仅后端段）；`REVIEW-REQUIRED-ALERTS-01` DEFERRED；`REVIEW-REQUIRED-ENTRY-01` DEFERRED；**不得把 AC-72 整体标 VERIFIED/声称 5 分钟告警已上线**。

---

## 4. 宪法 Article I–V / Governance 核对

| 条款 | 结论 | 依据 |
|---|---|---|
| I.1 拒绝“屎山雕花” | **PASS** | T3-R1 修复抽 `persistTCCReviewTx` 单一入口，消除 Tx1/Tx2 两套实现；无散落放行分支 |
| I.2 Fail-Closed | **PASS** | batch/app `RowsAffected==1`，0 行回滚；`batch_id` 缺失 fail-closed；未知 probe → Unavailable |
| I.3 契约优先 | **PASS** | 按 SPEC v1.11/ADR 0.6；无新字段/RPC/迁移；0022/0023 迁移不改 |
| I.4 拒绝自我妥协 | **PASS** | T3–T6 FAIL 后停线修复并重审；未用局部 if/假 runbook 掩盖 |
| II.1 事实分离 | **PASS** | REVIEW 非准入终态；permit/commitment/refund/outbox 分离；不把 REVIEW 当 PRE/已退款 |
| II.3 幂等/最终一致 | **PASS** | Tx2 双谓词 CAS；replay 同错；outbox/release 同事务；T6 NATS/DLQ |
| II.5 退款/履约准入 | **PASS（本 L2 范围）** | permit 保持、无 release、partial 不取消；T3 release 仍 deferred |
| III.2 数据库访问 | **PASS** | database/sql + CAS；无 GORM/新表 |
| III.4 静态/fail-fast | **PASS** | 无新 worker；metrics 挂既有 tick；无跨服务包引用 |
| IV.1 证据不可妥协 | **PASS** | T8 真实 MySQL/NATS；本报告重跑；AC-VERIFIED 仅对真实断言 |
| IV.2 重灾区抗压 | **PASS（有范围）** | `-race`、崩溃/竞态/NATS/DLQ；UAT/prod 未跑，已声明 |
| IV.4 实现/发布隔离 | **PASS** | `NOT RELEASE`；不声称 merge/Release/PAY-SETTLE-01 closure |
| Workflow 9 stop-the-line | **PASS** | 上一轮 FAIL 后冻结、修复、重审；未新开 change_id |
| Governance 4/6 | **PASS** | 人类 merge pending；仅引用既有 waiver，未扩大 |

**`[ARCHITECTURAL_FLAW]`：无。**

---

## 5. 残余 / 交回 PAY-SETTLE-01

1. **`REVIEW-REQUIRED-ALERTS-01` / `REVIEW-REQUIRED-ENTRY-01` DEFERRED**：5 分钟告警规则/值班部署、用户可见文案/人工入口未交付；tsv 三段落 DEFERRED，不得声称已上线。
2. **F7b**（迟到 `order.paid` 真实 placeholder）**未构造**：本 L2 未改 Fulfillment settlement consumer；Order 侧 no-auto-PRE 3 次 replay 已覆盖；不得声称 F7b 全链路验证。
3. **W8**（`PENDING_REMOTE` 永久悬挂）归 **AC-27-LIVE-01**，本 L2 不实现；接受为残余。
4. **AC-21/25/31/32/AC-27 既有 AC-REF 限制**：通知/全场景/可见性/AC-27A 5/6 仍开；本次回归不升级。
5. **T3-R2 Plan/Tasks 指针**：T6b 仍写 `order/review_consumer`/`TestReviewConsumer`（0 tests）；实际测试为 `internal/services/fulfillment/review_consumer_integration_test.go::TestRefundAdmittedConsumerReviewAndDuplicate`。Lead 需在 T7 记录/Plan 修正；不影响 T8 证据。
6. **交回 `PAY-SETTLE-01` 边界**：AC-27 仍 **AC-REF**；Phase 0-R **未闭合**（T2/T3/T4.4/T5 仍 deferred）；本 L2 已交付 AC-50 写路径（`AC-50-FIX-01` 的实现范围），但 **AC-27-NOWB-01 / AC-27-LIVE-01 / AC-27A 5/6** 仍归 Parent/T7 登记；**本 L2 PASS 不构成 P0 closure/Release/merge**。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-27
**报告**：`docs/collab/qc-reports/REV-PAY-AC50-REVIEW-01-FINAL.md`
