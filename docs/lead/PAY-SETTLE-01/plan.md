# PAY-SETTLE-01 Remediation Plan (Revision 1)

| 项 | 值 |
|---|---|
| Status | **APPROVED (Plan) — implementation not started** |
| Approved | 2026-11-24（Human / Director） |
| change_id | `PAY-SETTLE-01`（**原 change_id**；不使用新 id 绕过 Review-01 冻结） |
| Review base/head | `d38f282..03a0717`（Review-01 FAIL）；补救 `6550a2c`；当前治理 head `3afdfe2` |
| Scope | P0-A/P0-B 实现修复；不含 0012 协议扩展，只读其已冻结的分配事实；不含 Release |
| Integration owner | **Director**（人类）；deputy: TBD |
| Plan owner | Lead |
| Approvals | Human **APPROVED** 2026-11-24；QC **RESOLVED**；Architect **RESOLVED**；PO/Developer/Reviewer Phase 0 sign-off tracked below |
| Allowed paths | `docs/specs/0001*`、`docs/specs/0002*`、`docs/adr/0007*`、`api/proto/mall/v1/**`、`api/gen/**`、`internal/services/{payment,order,aftersales,fulfillment}/**`、`migrations/{payment,order,fulfillment}/**`、`cmd/**`、`internal/platform/{events,config}/**`、对应测试 |
| Baseline impact | `INV-01/04/05/06/07/14/24/26/30…44`；`AC-39…61`；基线 `AC-14/21/25/27/31/32` 回归 |
| Current WIP | 按人类裁决 B：保留在 main、冻结、不合并 release、不发布 |

## 1. 结论与范围

Review-01 的实现级 FAIL 有 6 个阻断项（P-01…P-06）。本 Plan 的目标不是“再加几个 if”，而是把 **TCC/售后的统一退款准入、历史单解析、commitment 生命周期、行级/责任运费** 收敛到已批准契约，并通过 Reviewer 有范围复核。

本 Plan 明确：P-04/P-05/P-06 涉及**业务语义修订**，必须先由 PO/Architect 产出 SPEC-0002 v1.8 / ADR-0007 Amendment 1，不能由 Developer 自创。Plan 批准不等于实现批准；Phase 0 契约批准后才进入 Phase 1。

## 2. 批准前必须由人类/PO 裁决的事项

| ID | 决策 | 选项/建议 | Owner |
|---|---|---|---|
| D-1 | 准入后放弃/永久失败 | 建议：permit 不可恢复；commitment 仅允许**人工授权、审计、幂等**的 `RESERVED→RELEASED`（释放额度、不恢复 permit）；禁止 worker 静默释放 | PO + Architect + Human |
| D-2 | 历史 `PAID` 未解析时能否发货 | 建议：24h 内阻断该组并进人工队列；超时终态：有捕获 intent → `EXPIRED_REFUND` 逐笔支付级退款；无捕获且 `pay_cent=0` → `zero:<order_no>`；无捕获且 `pay_cent>0` → `EXPIRED_FINANCE` 财务人工关闭（不发货、留审计）。`RESOLVED` 后恢复发货/退款命令幂等；`REJECTED` 仅用于脏数据终态。不得无限期停留在 `OPEN` | PO + Human |
| D-3 | 责任→可退运费的唯一 Owner | 建议：AfterSales/缺货判定责任并产出 `UNDETERMINED/HOLD` 或可退金额；Order 只做确定性校验与额度计算；168h 默认商家责任按 SPEC-0001 | PO + Architect |
| D-4 | `order.settled` 默认切换 | 本 Plan **不切换**；只实现 shadow 第三态并由 Director 另开 release 门禁；生产 flags 保持 false | Director |
| D-5 | `order.refund_requested` 消费者 | 保留 subject 作兼容；Payment 消费者改为只观察/告警，不再直接 `Refund`；移除日期/回滚由 ADR Amendment 定义 | Architect |

## 3. 契约修正（Phase 0，先于代码）

| 编号 | 产物 | 内容 | Owner | 依赖 |
|---|---|---|---|---|
| C-01 | SPEC-0001 v1.4 | AC-27 改 EARS（只禁行首 Gherkin；句内 AND/OR 允许）；登记基线迁移。**独立 PO 债务，不与 P0 修复混提** | PO | D-1..D-3 |
| C-02 | SPEC-0002 v1.8 | P-04/P-05/P-06、TCC Σ 预检、准入不可撤销/人工释放、行/运费/责任、历史人工队列；新增 AC-62+ 映射新语义 | PO | D-1..D-5 |
| C-03 | ADR-0007 Amendment 1 | TCC batch/扇出、Σ 预检、permit 冻结顺序、`refund_applications`、`legacy_payment_resolution`（含 `EXPIRED_*` 到订单/组终态的映射）、commitment 行/组成、消费者切换 | Architect | C-02 |
| C-04 | proto（增量字段，不重编号） | `RefundLine`、责任/运费字段、退款申请/完成 RPC、batch 查询；事件字段 | Architect owner；Developer 只消费 | C-03 |
| C-05 | migrations | `refund_applications`、`order_refund_commitment_lines`、`legacy_payment_resolution`、`refund_seq`；索引/唯一键；真实迁移测试 | Developer（schema 由 C-03 定） | C-03 |

Phase 0 退出标准：C-01..C-05 全部经 QC/Reviewer/人类批准；否则不得进入 Phase 1。

## 4. 关键设计（修订后）

### 4.1 P-01 TCC 补偿（顺序修正）

1. **Σ 预检（事务外 + 锁内复核）**：事务外调用 `Payment.GetIntent/ListOrderIntents` 取权威 captured/refunded 快照；然后在 Tx1 的订单锁下用**同一快照**复核 `Σ 活动组系统责任上限 == 订单剩余 captured`。禁止在 DB 事务内做跨服务调用；不等则回滚，**不改 permit/commitment/订单状态**。
2. **Tx1：REFUNDING + batch + 分状态 permit**：同一事务写 `orders.refund_state=REFUNDING`、组级 `refund_applications`/batch；对活动未离仓组按状态处理：`NONE→REVOKED`（CAS）；`CONSUMED` **不得写 REVOKED**，只写 `PENDING_REMOTE` admission + commitment，Tx1 提交后立即 `CancelIfCreated`；Ship 胜出则 `POST_SHIP_ALLOWED`。`AuthorizeShipment` 在 REFUNDING 提交后由 permit 状态/取消结果拒绝；不存在“退款打款后才发货”的路径。Ship 可先于取消胜出，此时按 `POST_SHIP_ALLOWED` 处理，不得回滚履约。
3. **持久续跑**：补偿入口在 `REFUNDING` 下不得直接早退；按 batch/task 幂等推进；崩溃后重跑只补未完成组。
4. **逐组准入**：对每个活动组调用 `AdmitRefund`（已 REVOKED 的组幂等返回 PRE；已离仓组 POST）；任一未成功不得打款。
5. **打款**：全部准入成功后，按组系统责任上限逐组调用 Payment；`payment.refunded` 的 `refund_id` 关闭对应 commitment。
6. **兼容**：`order.refund_requested` 保留 subject；Payment 消费者改为观察/告警；TCC 退款只走 batch/准入。

测试：Σ 不等不改状态、REFUNDING 与 Ship 并发、崩溃后重跑、部分组失败、已发货组、无组 G1、`PENDING_PAYMENT+已捕获` 分支。

### 4.2 P-04 历史 PAID

- 生产者先修：TCC/直接路径不得产生“PAID 但无 `settlement_payment_no`”。
- 解析：`settlement_payment_no` 为空（`''`/NULL）且 `PAID` 时调用 `Payment.ListOrderIntents`。
- 候选：`status=SUCCEEDED` + `captured == pay_cent` + order 匹配；`pay_cent=0` 解析为 `zero:<order_no>`。
- 唯一候选：CAS 写回 `WHERE settlement_payment_no = ''`（当前列 `NOT NULL DEFAULT ''`；若 C-05 改为可空，则用 `IS NULL`）。并发只有一个成功。
- 其他已捕获 intent：按支付级退款/`payment_duplicate_captures` 处理，不静默留存。
- 0/多候选：写 `legacy_payment_resolution`（`OPEN/RESOLVING/RESOLVED/EXPIRED_REFUND/EXPIRED_ZERO/EXPIRED_FINANCE/REJECTED`，唯一键 `order_no`，SLA 24h，Owner Director/ops）；`RESOLVED` 后写回 payment_no 并通过幂等恢复命令继续发货/退款；24h 超时按 D-2 进入 `EXPIRED_REFUND`/`EXPIRED_ZERO`/`EXPIRED_FINANCE` 终态，不得无限期 `OPEN`；不得猜 payment_no。
- 回填工具：dry-run、计数对账、幂等、只处理 `PAID`、先修生产者后运行。
- `refund_seq`：独立序列表/唯一键，禁止并发取号冲突。

### 4.3 P-05 准入后生命周期

- permit 终态不可恢复；不允许 `REVOKED→CONSUMED`。
- 准入前拒绝/取消不创建 commitment；准入后执行失败进入持久任务/人工。
- `FAILED_PERMANENT`/人工放弃：ADR Amendment 定义**人工授权、审计、幂等、带 version/lease fencing** 的 `RESERVED→RELEASED`，只释放额度，不恢复 permit；禁止 worker 静默释放。
- `RELEASED` 的合法触发由 C-03 定义；如果找不到合法触发，删除该状态并同步 SPEC，不能保留死代码。

### 4.4 P-06 行级/责任运费

- proto：`repeated RefundLine { line_no, quantity, goods_cent }`；`responsibility` 封闭枚举 + `reason_code`；`freight_refundable_cent`（AfterSales 判定）；`batch_id`/`refund_seq`。
- 存储：`order_refund_commitment_lines`记录 line/goods/freight 组成；组退款台账记录已退运费。
- 额度：行剩余 = `line_paid - 已承诺/已退`；组剩余 = `Σ 行剩余 + 可退 freight_share`；订单级 Payment captured 仍为最终上限。
- 未离仓整组；已离仓允许部分；`UNDETERMINED/168h` 时运费挂起，组保持 REFUNDING，超期按 SPEC-0001 默认商家责任；后置补退同组。
- 责任→可退运费映射只由 AfterSales 产出；Order 只校验，不猜测。

### 4.5 Fail-closed / 事实切换

- Inventory/Payment/Fulfillment 为必需依赖：构造期必填、启动 fail-fast；删除生产代码 nil 跳过；测试替身仅 `_test.go`/build tag。
- 逐 `cmd/*` 列依赖矩阵与启动测试；依赖故障时“不可启动”写进运维说明与回滚。
- `order.settled` 只实现 shadow 第三态（双消费、只记录、不建单）；默认值保持 false；切换是 Director 独立 release 门禁，附 §7 矩阵/五方终态/移除日期/回滚。
- `AuthorizeShipment` 为唯一发货许可权威；组 `refund_state` 仅防御/展示，不构成第二权威；permit + 行确认 + 缺货是禁发裁决点。

## 5. 任务与 Owner

详见 `tasks.md`。Phase 1–5 每条任务必须有 Owner、依赖、验收、证据；无 Owner 不得开始。

## 6. AC 证据矩阵

详见 `evidence-matrix.md`。它只是 Plan 视图，**不替代** `ac-coverage.tsv`（Owner=Reviewer）；AC-VERIFIED 只能由 Reviewer 依据真实执行日志写入唯一台账。

新语义（P-05 人工释放、P-04 人工队列、P-06 行/运费、Σ 预检、shadow）在 C-02 中新增 AC-62+ 或明确映射到 AC-50/53 的 EARS 条款；没有 AC 编号的语义不得进入实现。

## 7. 验收与停止条件

- Phase 0 契约未批准 → 停止。
- Σ 预检不等 → 任何状态变更前停止 + 人工。
- Plan/ADR/SPEC 冲突 → 停止并回所有者，不按 Plan 硬写。
- Reviewer FAIL → 冻结 `PAY-SETTLE-01`；不得用新 id/hotfix 绕过。
- AC 矩阵出现无 AC 编号/无 owner/无 evidence 行 → 不得进入 Phase 5。
- 默认开关未过 Director release 门禁 → 保持 false。
- AC-39…61 未全部有证据/显式延期 → 不声称实现 PASS/Release PASS。
- SPEC-0002 §7 的 E1…E21 与五方终态矩阵是 **Release 门禁**；R1 只列受影响基线 AC 与回归证据，不声称通过该矩阵。

## 8. 审批记录

| 角色 | 结论 | 日期 | 备注 |
|---|---|---|---|
| PO | Human approved D-1..D-5; SPEC authoring pending | 2026-11-24 | C-01/C-02 |
| Architect | Review RESOLVED; Amendment authoring pending | 2026-11-24 | C-03/C-04 |
| QC | Review RESOLVED | 2026-11-24 | 异常分支/测试矩阵 |
| Developer | Phase 0 sign-off pending | | 可执行性/风险 |
| Reviewer | Phase 0 sign-off pending | | 可验证性 |
| Human | **APPROVED** | 2026-11-24 | D-1..D-5、Plan |
