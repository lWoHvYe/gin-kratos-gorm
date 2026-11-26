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

---

## P3b focused re-review（Plan/Tasks 重写后；HEAD `a1cc809`，ADR errata 0.6 工作树）

只核上轮 7 BLOCKER + 7 MAJOR；未重复全量评审，未执行测试（只读）。

| 上轮 # | 状态 | 证据 / 残余 |
|---|---|---|
| 1 Fulfillment `<NO_WAYBILL>` 任务 | **CLOSED** | T0：plan.md:41、tasks.md:25-27（F1/F1b/F2–F6/F8 + 文件/命令/expected） |
| 2 proto / `api/gen` 矛盾 | **CLOSED** | T0 含 proto+`make proto`（plan.md:41）；forbidden 仅“`api/gen/**` 手改”（plan.md:15）；ADR 0.6:24 列为生成物 |
| 3 原子性/发布顺序 | **CLOSED** | 发布单元（plan.md:13）：Fulfillment+Order 同一原子提交；拆分先 Fulfillment 后 Order |
| 4 T6 CAS 倒置/谓词不足 | **CLOSED（含 C-5 门）** | T0c 前置并写死 `state='PENDING_REMOTE' AND result <> 'REVIEW_REQUIRED'`（plan.md:43,86；ADR 0.6 A8:447、W13:506、O11d:730）；**C-5 人类 errata 批准仍待办**（plan.md:33） |
| 5 `order/worker.go` 不在 allowed | **CLOSED** | plan.md:14 含 `worker.go`（仅 gauge） |
| 6 `docs/ops.md` runbook | **PARTIAL（BLOCKER 未闭）** | 路径/owner/C-5 errata 已补（plan.md:14、:12），但 §2/tasks **无任务、无 acceptance、无期限**；`docs/ops.md` 在 tasks 仅出现在 C-5 行（tasks.md:13）→ AC-72 后端段“查询/SLA/对账”与 §8.3 re-arm/存量 PRE 审计无证据路径 |
| 7 BFF/gateway 文案越界 | **CLOSED** | plan.md:15 将 gateway 移出本 L2；无文案任务 |
| 8 任务粒度 | **CLOSED** | T1a–e/T2a–c/T4a–b/T5a–b/T6a–d 拆分（plan.md:44-60） |
| 9 allowed 过宽 | **CLOSED** | AfterSales 收窄到 `workers.go`、gateway forbidden、order 文件点名（plan.md:14-15）；残余 minor：`order/consumer.go` 未写“只读不改/需 Brief 论证” |
| 10 test file/命令/expected + 环境 | **PARTIAL（BLOCKER 未闭）** | 每任务有文件/expected ✓，但命令多处不可执行：`...` 占位（tasks.md:35-43,45,50,53,56,59,61,65,71）；T5b 未加 `TEST_MYSQL_DSN=$ORDER_DSN`（tasks.md:63）；T6b 只有 `TEST_NATS_URL=...` 且缺 MySQL DSN（tasks.md:67）；T4a 包归属不明（tasks.md:56）；T8 “见 DoD” 无命令（tasks.md:73）；`-race` 仅覆盖 T0c（plan.md:68） |
| 11 REVIEW 识别/8 次不 FAILED | **CLOSED（实现细节残留）** | T2c 绑 O22/O22b（tasks.md:49-51）；未写 typed gRPC details 识别方案（禁止字符串嗅探）→ T2b/T2c 实现时须固定 |
| 12 零元终态/队列/AC-72 单列 | **PARTIAL** | O21 计数已绑（tasks.md:55-57）；零元冲突“有限重试→人工队列”终态无 acceptance（SPEC:434；ADR §4.1） |
| 13 tsv 写入权 | **CLOSED** | T7 “Reviewer 写 tsv”（plan.md:61,75） |
| 14 无迁移核验/owner | **CLOSED** | DoD `git diff --stat -- migrations` 为空（plan.md:80）；T5 owner 列已去掉，ops/Developer 分工由 plan.md:12 定义 |

**新增残余（不在上轮清单，但影响 P5）**

- **T0b/T0c 发布单元**：T0b 写 REVIEW、T0c 才加双谓词；若分两次部署，中间窗口仍可被迟到探测洗成 PRE。需声明 **T0/T0b/T0c 同一发布单元**（或 T0c 先行），否则违反 A12 的“原子”。
- **F7b 未绑定**：ADR §10 的 F7b（迟到 `order.paid`/部分金额不转 PRE）未在任何任务 expected 中列出（T6b 仅 O14/O14b）。
- **T1c expected 缺同事务断言**：标题写“Tx2 同事务写 batch/app/admission”，expected 只有 `MANUAL_HOLD`/`refundCalls==0`（plan.md:46、tasks.md:38-39）；应断言 `refund_batches.last_error` canonical 与“仅已有 `refund_applications` 行才镜像”。

**P3b 结论：BLOCKED（窄口径，2 个剩余 BLOCKER + 3 个残余）**

1. `docs/ops.md` runbook 有路径/owner 但无任务/验收/期限 → 补 **T5c**（owner ops，expiry 2026-12-31）：AC-72 查询/SLA/对账、§8.3 re-arm、存量 PRE 审计 SQL + expected 0 行/查询输出。
2. 命令可执行性：删除全部 `...` 占位，逐任务写全 `TEST_MYSQL_DSN=$ORDER_DSN|$FF_DSN|$AS_DSN`（T5b 补 `ORDER_DSN`，T6b/T6c 补 MySQL DSN 与 `TEST_NATS_URL=nats://127.0.0.1:4222`），T8 给全量命令；`-race` 至少覆盖 T0c/T6a/T6b。
3. 残余：T0/T0b/T0c 同一发布单元；F7b 绑定；T1c 同事务断言。

C-5（ADR 0.6 人类 errata 批准）仍是 P5 前置门，不属于 Developer 评审阻断；上述 1–3 修订后可转 READY。

---

## P3c focused re-review（只核 P3b #6/#10 + 新残余）

| P3b 项 | 状态 | 证据 / 残余 |
|---|---|---|
| #6 `docs/ops.md` runbook 无任务 | **CLOSED** | T5c：plan.md:57、tasks.md:67（owner ops、expiry 2026-12-31、证据=runbook 链接+演练记录，人工审阅）；allowed_paths 含 `docs/ops.md`（plan.md:14） |
| #10 命令/环境 | **PARTIAL** | **Tasks 已补齐**（tasks.md:26-81：T1a–T6d/T8 全 env/包路径，T4a 拆两条 `:58-59`，T5b 补 `$ORDER_DSN` `:66`，T6b/c 补 MySQL+NATS `:71,73`，T8 按包拆三条 `:79-81`）；**Plan 表未同步**：plan.md:53 仍单条 T4a、plan.md:59 仍 `TEST_NATS_URL=...` 占位、plan.md:60 “同上”、plan.md:63 仍单 `$ORDER_DSN` 跑 `{order,aftersales,fulfillment}` 三包（违反 per-package schema，`docs/development.md:52-54`）；**`-race` 未落 T6a/T6b**（plan.md:69 要求，tasks.md:69,71 命令无 `-race`） |
| T0/T0b/T0c 同一发布单元 | **CLOSED** | plan.md:13、tasks.md:5（分批顺序 Fulfillment → T0c → T0b） |
| T1c 同事务断言 | **CLOSED** | plan.md:46、tasks.md:38-40（“同事务失败整体回滚”） |
| T2a/T2b typed-details | **CLOSED** | plan.md:49-50、tasks.md:45-50（结构化 `result/reason/ticket`、禁字符串嗅探） |
| F7b 绑定 | **CLOSED** | plan.md:59、tasks.md:70（T6b 含 F7b/W8/W9/W13） |
| 零元终态/人工队列 | **未闭（新矛盾）** | plan.md:53、tasks.md:60 写“**零元不增加人工队列**”，与 SPEC:434（零元冲突“并告警、**进入人工队列**”）、ADR:412（有限重试后进人工队列）、ADR:581（仅“不进入 `REVIEW_REQUIRED` 计数/单列”）冲突；应为“不计入 REVIEW backlog（单列），仍进入零元人工队列”。否则实现会漏掉 AC-67 的人工队列 |

**P3c 结论：BLOCKED（窄口径，2 项）**

1. **零元人工队列表述**（plan.md:53、tasks.md:60）与 SPEC:434/ADR:412/581 冲突，需改为“不计入 `REVIEW_REQUIRED` backlog（单列），仍进入零元人工队列（AC-67/AC-72）”。
2. **Plan 表与 Tasks 命令不一致**（plan.md:53,59-60,63）：同步为 tasks 的完整 env/按包命令；T6a/T6b 命令补 `-race`（或把 plan.md:69 的 `-race` 口径收窄为 T0c+显式列出）。

其余 P3b 残余（T0 发布单元、T1c、T2 typed、F7b、#6 T5c）均已关闭；C-5（ADR 0.6 人类 errata 批准）仍为 P5 前置门。上述 1–2 修订后可转 READY。

P3d: READY — T4a 双命令/零元 backlog 文案（plan.md:53、tasks.md:58-60）、T6a/T6b/T6c `-race`+DSN+NATS（plan.md:58-60、tasks.md:69,71,73）、T8 三包命令（plan.md:63、tasks.md:79-81）均已核对一致；C-5 仍为 P5 前置门。
