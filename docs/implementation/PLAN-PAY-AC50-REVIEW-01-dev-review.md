# PLAN-PAY-AC50-REVIEW-01 — Developer Plan 评审（L2-P3）

| 项 | 值 |
|---|---|
| change_id / Tier | `PAY-AC50-REVIEW-01` / **L2**（资金、跨服务、状态机；plan.md:7-8） |
| Role / Actor | Developer / `dev-r04-fix`（只读评审；未写业务代码、未改 Plan/SPEC/ADR/台账，未 commit） |
| 输入 | Plan `docs/lead/PAY-AC50-REVIEW-01/plan.md` + `tasks.md`、SPEC-0002 v1.11、ADR-0007 AC-50 Addendum 0.5、impact 0.3；代码只读核对 HEAD `752b3f5`（`go test`/迁移未执行）。ADR/SPEC 行号取评审时工作树版本（其他角色并发修改，结论仍以 Plan 行为为准） |
| 结论 | **BLOCKED**（Plan 层可修；非推翻 SPEC/ADR 语义） |

## 1. 可执行性 / 依赖顺序 / 原子性

1. **缺 R-1 任务（BLOCKER）**：T1–T8 无一交付 Fulfillment 的 `<NO_WAYBILL>`（现状 `cancel_if_created.go:50-55` 无行→`CANCELLED`）；它是 T1/T2/T3/T4 所有 REVIEW 映射的前提（ADR:24、§2.4:289-297、§15.1#1），Plan 的 allowed_paths 列了该文件却无 owner/验收/F1–F7b 断言（plan.md:14,41-48）。
2. **缺 proto/生成物任务（BLOCKER）**：plan.md:14 允许 proto 注释、plan.md:15 又 forbidden `api/gen/**`；`make proto` 必然产生生成 diff（`Makefile:153-157`；ADR:24 要求 Lead 在 Brief 确认提交方式）。无任务/无验收 → T5 无法交付。
3. **原子性/发布顺序缺失（BLOCKER）**：ADR:294 要求 Fulfillment 新值与 Order 映射同一原子提交（或先 Fulfillment 后 Order，旧 Order fail-closed）；Plan 无此约束（plan.md:41-48、§4:60-63）。
4. **T6 顺序倒置 + CAS 谓词不足（BLOCKER）**：Tx2 CAS 是 T1/T2/T3 的公共前置，却排最后（plan.md:46）。ADR:436 的 `WHERE refund_id=? AND state='PENDING_REMOTE'` 对 REVIEW 无效——REVIEW 的 `state` 仍是 `PENDING_REMOTE`（`0016:7-19`），当前 `refund_admission.go:590-594` 无守卫：Tx2 B 先探测得 `CANCELLED`、Tx2 A 先把 REVIEW 写入（state 不变），B 的 CAS 仍命中并把 REVIEW 洗成 PRE（INV-51/A1/A9）。需 T6 前置并把谓词改为 `result`+`state` 双条件（或 `AND result<>'REVIEW_REQUIRED'`），加 O11 并发断言。
5. **`order/worker.go` 不在 Plan allowed_paths（BLOCKER）**：ADR:24 明确含 `internal/services/order/worker.go`（仅 gauge 刷新）、ADR:647；plan.md:14 未列 → AC-72 gauge tick 越界或漏做。
6. **`docs/ops.md` 缺口（BLOCKER）**：AC-72 查询/SLA/对账、§8.3 re-arm、存量 PRE 审计（SPEC:492-499）需落 runbook；Plan 的 allowed/forbidden 均未列该路径，也无 owner。
7. **T5 越界（BLOCKER）**：plan.md:45 要 BFF 文案“退款处理中（需人工核对）”；SPEC:503 明确该文案/入口归 `REVIEW-REQUIRED-ENTRY-01`、本 L2 交付前“沿用既有‘退款处理中’”，且 `internal/services/gateway/**` 当前无任何 refund 文案面（仓库 grep 为空）→ 不可执行且与 SPEC 冲突。

建议顺序：**T0**（Fulfillment `<NO_WAYBILL>` + 唯一映射 + existing-first + Tx2 双谓词 CAS，同一原子提交）→ T1 → T3 → T4 → T5（拆审计/metrics，删除文案项）→ T7 → T8；T6 并入 T0。

## 2. 一任务一不变量与 allowed_paths

- T1 含 5 个断言面（合法值/`MANUAL_HOLD`/0 Payment/只阻塞本批/re-arm）、T3 含 3 分支、T4 含零元+legacy、T5 含审计+metrics+文案（plan.md:41,43,44,45），与 plan.md:37 自述“每任务独占一个不变量/分支”不符——建议拆分或每分支独立提交。
- allowed_paths 过宽/不精确：`internal/services/aftersales/` 整目录（plan.md:14）超出 ADR:24/§8.5 的“仅 last_error/PENDING/REVIEW 重试上限”（`workers.go` 同文件含 restock 路径 `:230-260`，误改即 FAIL）；`internal/services/gateway/**` 无 refund 面；`cmd/*` 无任务使用；`internal/services/order/` 的“refund_batch*/consumer/metrics”未点名（`refund_release.go`/`consumer.go` 是否在内，ADR:24 要求逐条论证）。
- 缺失路径：`internal/services/order/worker.go`、`docs/ops.md`、`docs/implementation/**`（自检/证据）；T7 未写 Developer 不得改 `ac-coverage.tsv`（CONVENTIONS:31）。

## 3. 测试环境可行性（真实 MySQL 8.4）

- **可行**：仓库已具备 disposable `mysql:8.4` + `mall/migrate:dev` 的验证先例（PAY-SETTLE-01 R0.4）；Order 包有 `fakePaymentPort`/fake fulfillment（`refund_admission_integration_test.go:21-110`），AfterSales 有 `fakeOrderService`（`settlement_integration_test.go:199`），NATS 2.10 镜像可用，`-race` 有先例。
- **Plan 未落**：未写 `TEST_MYSQL_DSN`/per-package schema、NATS（O14/F7b 需 `TEST_NATS_URL`）、`-race`/故障注入命令；O1b 必须走 `TccCancelMarkPaid`（ADR:679），O19/O22 走 AfterSales fake，O2c 走 Order fake 的 `getErr`；T1–T6 只有文字验收，未绑定 ADR §10 的 O/F case id → Reviewer 无法按路径核对。

## 4. 缺失的前置/测试/ops

- 无迁移核验未写：`git diff --stat -- migrations` 为空、0016/0017 frozen（ADR:642）。
- T4 缺零元终态/人工队列/AC-72 零元单列（SPEC:434,490；ADR O7/O7b/O8/O21）。
- T2 缺 REVIEW 的**类型化识别**（从 gRPC details 取 `result=REVIEW_REQUIRED`，禁止字符串嗅探）与非 REVIEW 8 次仍 `FAILED_PERMANENT` 的回归（`workers.go:740`；O22b 未绑）。
- ops：告警规则已声明另立（plan.md:45），但 runbook/存量 PRE 审计/对账无 owner/期限。
- 回滚口径（ADR §7.6：仅前向修复、禁无审计回滚）未进 Plan 验收/停止条件。
- T1 的 `TCC_BATCH_REQUIRES_MANUAL_HANDLING`/`MANUAL_HOLD` 短路已存在（`tcc_settlement.go:133-140`、`tcc_admission.go:87,404`；`tcc_payment_integration_test.go:268`），验收应写“复用+回归”，避免重复实现。

## 问题清单（`文件:行号`）

| # | 级别 | 位置 | 问题 / 最小修订 |
|---|---|---|---|
| 1 | BLOCKER | plan.md:14,41-48 | 缺 Fulfillment `<NO_WAYBILL>` 任务；新增 T0（或并入 T2）并绑 F1–F7b |
| 2 | BLOCKER | plan.md:14-15 | proto 注释与 `api/gen/**` forbidden 矛盾；明确生成物仅 `make proto` diff（Lead Brief） |
| 3 | BLOCKER | plan.md:41-48 | 无 Fulfillment+Order 原子提交/先 Fulfillment 后 Order 约束（ADR:294） |
| 4 | BLOCKER | plan.md:46；ADR:436；`refund_admission.go:590-594` | Tx2 CAS 对 REVIEW 无效（REVIEW 仍是 `state=PENDING_REMOTE`，`0016:7-19`）→ T6 前置 + 双谓词，需 Architect errata/Brief 固定 |
| 5 | BLOCKER | plan.md:14；ADR:24,647 | `order/worker.go`（gauge tick）不在 allowed_paths |
| 6 | BLOCKER | plan.md:14-15；SPEC:492-499 | AC-72/§8.3 runbook（`docs/ops.md`）无 owner/路径 |
| 7 | BLOCKER | plan.md:45；SPEC:503 | BFF 文案越界；删除文案项，仅保留“不展示已退款/已关闭”断言 |
| 8 | MAJOR | plan.md:37,41-45 | 任务粒度违反“一任务一不变量”；拆 T1/T3/T4/T5 |
| 9 | MAJOR | plan.md:14 | aftersales/gateway/cmd 过宽、order 子文件未点名；按文件白名单收窄 |
| 10 | MAJOR | plan.md:41-48 | 验收未映射 ADR §10 用例与环境/命令；补 case id 矩阵 |
| 11 | MAJOR | `workers.go:740`；SPEC:456 | REVIEW 识别/8 次不 `FAILED_PERMANENT`/O22b 未落；补 typed-details 方案 |
| 12 | MAJOR | SPEC:434,490；plan.md:44 | 零元终态/人工队列/AC-72 零元单列未落 |
| 13 | MAJOR | CONVENTIONS:31；plan.md:47 | T7 的 tsv 写入权与 Developer 证据边界未写 |
| 14 | MINOR | plan.md:45；ADR:642 | T5 owner 混 Architect；无迁移机器核验未列 |

## 结论：**BLOCKED**

Plan 的契约方向与 ADR 0.5 一致，但现稿无法在不越界/不返工的前提下开工：缺 R-1（Fulfillment）与 proto 任务、allowed_paths 与 ADR:24 不一致、T6 前置与 CAS 谓词不足（可把 REVIEW 洗成 PRE）、T5 文案与 SPEC:503 冲突、AC-72 ops runbook 无路径。修订 #1–#7 后可转 READY；#8–#14 可在 P5 前补齐。除 #4 需要 Architect 以 errata/Brief 固定 CAS 谓词外，其余只需 Lead 修 Plan，无需重开 SPEC/ADR 状态机设计。
