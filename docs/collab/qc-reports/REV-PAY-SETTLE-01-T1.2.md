# REV-PAY-SETTLE-01-T1.2 — T1.2 / T1.2b / T1.3 实现有范围审查

| 项 | 值 |
|---|---|
| review_scope | **实现**审查：`998e896..4e5d07f` 中 T1.2（`12858d4`）、T1.2b（`4e5d07f`）的 Tx1 / Σ 预检接线 / permit / batch / application / resume，以及 T1.3 的 `REFUNDING` 续跑；对齐 SPEC-0002 v1.9 AC-51/AC-62/AC-65/AC-66/AC-67/AC-68 中与本范围相关部分、INV-48/INV-45/INV-36/INV-06，以及已 accepted 的 T1.2 Contract Rulings Addendum R-1/R-2/R-3。**不审查** T1.4/T1.5/T1.6/T1.7/T2/T3/T5 的实现，不评审 Spec/ADR 本身，不构成 Release PASS。 |
| base | `998e896`（Phase 1a checkpoint docs；T1.0/T1.1 代码在更早的 `053896b`） |
| head | `4e5d07f`（T1.2b）；工作区 HEAD `f972cb2`，`4e5d07f..HEAD` 仅 docs/契约接受（含本 Addendum accepted），`git diff 4e5d07f..HEAD -- internal/` 为空，因此测试绑定到 `4e5d07f` 的代码。 |
| spec_version | SPEC-0002 **v1.9**（approved 2026-11-24） |
| adr_revision | ADR-0007 Amendment 1（accepted，Rev 1.0）+ 零元 Addendum（accepted）+ **T1.2 Contract Rulings Addendum（accepted，Rev 1.0，R-1/R-2/R-3）** |
| change_id | `PAY-SETTLE-01` |
| integration_owner | `[不确定]`：交付物未显式声明；Plan T5.7 仍为待办（见 M3） |
| INV | INV-06、INV-24、INV-36、INV-44、INV-45、INV-46、INV-48、INV-50（本范围主要涉及 INV-06/36/48/45） |
| affected_AC | AC-51、AC-65（主）；AC-62/AC-64/AC-66/AC-67/AC-68 仅边界核对；**不宣称任何 AC-VERIFIED** |
| verification | 见 §4：`go build ./...`、定向 order/TCC 测试、完整 order 包、TCC `-race`、`go vet` 均通过；MySQL 真实实例；测试使用测试文件内联 DDL（非真实 `migrations/`） |
| Actor / Reviewed-by | `reviewer-phase1`（Reviewer）；代码作者 `dev-phase1b`（Developer），不同 actor |
| release_status | **FAIL（实现级）**；不是 Release PASS，不批准合并 |

> 本报告只裁决指定 `base..head` 的 T1.2/T1.2b/T1.3 实现；不修改代码/契约，不代替人类 merge/Release 决策。FAIL 按 AGENTS §1.5/宪法 Workflow §9 触发 Stop-the-line：`PAY-SETTLE-01` 不得继续在该 change_id 提交受管代码，修复须挂重审 Plan。

---

## 1. 结论

**FAIL（有范围）。**

R-1/R-2/R-3 的三个核心机制在“订单状态仍为 `PAID` / `PENDING_PAYMENT`”的测试路径上实现得较完整：Σ 预检接线、Tx1 订单锁内同一快照复核、仅审计 batch、`PRECHECK_FAILED` 不 resume、`ListShipments` 事务外读取、`EXCEPTION+shipped_at` 判已离仓、PENDING+captured fail-closed、`NONE→REVOKED`、`CONSUMED` 不 REVOKED、batch/application 幂等、G1、T1.3 续跑与并发收敛均有真实 MySQL 测试通过。

但存在一个**资金链路 fail-open 的生命周期入口缺陷**（B1），以及一个**历史组成语义偏离**（M1）和一个**无活动组语义缺失**（M2）。B1 使 T1.2/T1.3 最重要的“已离仓 / 多组部分发货 / REFUNDING 不早退”在真实业务状态（`ALLOCATED`/`SHIPPED`/`COMPLETED`）下**静默成功且不退款**，且现有测试用不可达的 `PAID + group SHIPPED` 组合掩盖了该缺陷。按 AC-51/AC-65/INV-48 与 Addendum R-2/T1.3 的要求，不能 PASS。

**不是 `[ARCHITECTURAL_FLAW]`**：AC-51/AC-65 与 Addendum R-2 已明确“已离仓组按 `POST_SHIP_ALLOWED` 退款”“补偿按组扇出”，契约本身完整；缺陷是 Order 生命周期入口没有覆盖契约要求的状态，属于实现偏离。若 Architect 认为 TCC 补偿只应覆盖 `PENDING_PAYMENT/PAID`，那是对 SPEC/ADR 的收窄，必须走版本化修订并经 Director 批准，不能在实现里静默 `return nil`。

---

## 2. 阻断项 / 主要问题

| ID | 严重级 | 区域 / 契约 | 证据（精确位置） | 事实 | 必须满足的契约行为 |
|---|---|---|---|---|---|
| **B1** | **BLOCKER** | TCC 补偿入口 / 订单生命周期 / AC-51、AC-65、INV-06、INV-48、Addendum R-2、Plan T1.3 | `internal/services/order/tcc.go:190-196`（入口 `from != PENDING_PAYMENT && from != PAID` → `return Empty, nil`）；`internal/services/order/tcc_settlement.go:235-237`（Tx1 同样 `locked.Status != PENDING/PAID` → `return nil`）；`tcc.go:375-389`（`tccCancelDecision` 对 `ALLOCATED/SHIPPED/COMPLETED` 返回 `tccCancelNone`）；真实状态转移见 `internal/services/order/consumer.go:58`（`shipment.created.v1` → `PAID→ALLOCATED`）与 `consumer.go:163-200`；调用方 `internal/services/payment/tcc.go:42-47` 对 `ALLOCATED/SHIPPED` 直接 `return nil`，`:73-84` 只在 `CANCELLED/CLOSED` 时 `refundUnpayable`，因此 TCC cancel 的静默成功不会被 Payment 侧补偿；多组部分发货的既有测试 `internal/services/order/settlement_integration_test.go:630-634`（order 保持 `ALLOCATED`，组可 `SHIPPED`） | **当订单已推进到 `ALLOCATED`/`SHIPPED`/`COMPLETED` 时，TCC 补偿静默返回成功，不建 batch、不写 permit/application/commitment、不退款。** 具体时序：`TccConfirmMarkPaid` 在订单分支事务内写 `PAID` 并 append `order.paid`（`tcc.go:97-132`）→ outbox 投递 → Fulfillment 创建运单 → `shop.shipment.created.v1` 消费把 `PAID→ALLOCATED`（`consumer.go:58`）→ payment-confirm saga 的后续分支失败，DTM 调 `TccCancelMarkPaid` → `tcc.go:190-196` 命中 `ALLOCATED` 直接返回成功；或 entry 读到 `PAID`、Tx1 加锁时已是 `ALLOCATED`，`tcc_settlement.go:235-237` 直接 `return nil`。结果：**captured 的钱不退，运单继续走，DTM 认为补偿已完成，Payment 回调重试又因订单已 `ALLOCATED` 直接 `return nil`（`payment/tcc.go:42-47`）**。这不只是“少发货组被撤 permit”问题，而是 AC-51 已离仓组 `POST_SHIP_ALLOWED` 分支在真实状态不可达。 | AC-51 明确：`IF 某组已离仓（SHIPPED/RECEIVED，或 EXCEPTION 且 shipped_at 非空）THEN 按 POST_SHIP_ALLOWED 退款，不得取消运单；其他未离仓组按 PRE_SHIP_REVOKED`。AC-65 的组级扇出同样不限制订单只能处于 `PENDING/PAID`。补偿入口必须覆盖所有可能已捕获的订单生命周期状态；无法安全判定时 MUST 返回可重试/可识别的 fail-closed 错误，**不得 `return nil` 静默成功**。已发货/已完成的组按 POST 退款；未离仓组按 PRE/撤 permit；`REFUNDING` 订单即使生命周期已推进也必须续跑（T1.3“REFUNDING 不早退”）。T1.7 必须增加真实事件序列用例（`shipment.created→ALLOCATED`、多组部分 `SHIPPED`、`REFUNDING` 后 `ALLOCATED`），不能再用 `PAID+group SHIPPED` 这种原子投影不会产生的组合。 |
| **B2** | **BLOCKER（交付治理，非代码缺陷）** | 交付物完整性 / AGENTS §1.4、§6、CONVENTIONS §8、Reviewer prompt | `prompt/Reviewer.md:9`（“如果 Developer 没有提交自检清单，直接 FAIL 打回”）；`AGENTS.md:56`（强制三问：并发/超时、绕过架构的 if-else、向现实的妥协）；`docs/collab/CONVENTIONS.md:220`（必须含测试日志/【测试范围】/【未覆盖范围】）；仓库内 `docs/collab/developer-reviewer/threads/` 只有 `T-001/T-002`，无本 T1.2/T1.2b 交付物；`docs/lead/PAY-SETTLE-01/tasks.md` 仍是 `DRAFT — not approved`（见 M3） | 仓库 canonical 交付物中**没有** Developer 的《反妥协自检清单》三问回答、`【测试范围】`、`【未覆盖范围】`、返工影响声明。Lead 口述的测试耗时（order 38.806s / TCC 15.376s / -race 16.370s）是日志声明，不是自检清单。 | 按角色 prompt，缺自检清单直接 FAIL。重审包必须补：①并发/超时/部分成功处理；②是否绕过架构加 if-else；③向现实做了哪些妥协（重大妥协高亮）；④`【测试范围】`/`【未覆盖范围】`；⑤测试日志绑定精确 head。若该交付物实际由 Lead 另行持有，应在重审材料中附上并落 canonical 指针，不能以口述替代。 |
| **M1** | **MAJOR** | 历史 commitment 组成 / ADR Amd1 §4.3、AC-64、AC-65“已部分退款组只退剩余” | `internal/services/order/tcc_settlement.go:910-1021`（`loadTccGroupComposition`）：`928-935` 要求 `amount == goods+freight`，`1011-1018` 要求 commitment 行合计等于头；`tcc_settlement.go:522-529` 在按组循环**之前**加载整单组成；ADR `docs/adr/0007-payment-settlement-admission-amd1.md:268-269`（历史 `composition_known=0` 行 `goods=amount, freight=0`，**只允许整组退款**）；`migrations/order/0018_refund_commitment_composition.sql:23-25` 的迁移把历史行写成 `goods=amount`、无行 | 迁移后的历史 commitment 是 `goods_cent=amount_cent, freight_cent=0, composition_known=0`，且**没有 `order_refund_commitment_lines` 行**。该行能通过 `928-935` 的头校验，但会在 `1011-1018` 因“头 goods=amount、行 goods=0”失败；`writeTccBatchGroupsTx` 在循环前就加载整单组成，所以只要订单上有**任意**历史 commitment（即使该组已离仓、本应走 POST 不建 commitment），整个 TCC 补偿都会失败。这与 ADR“含 composition_known=0 的组只允许整组退款”相反——整组 TCC 退款被完全阻断。测试 `internal/services/order/tcc_compensation_integration_test.go:724-771` 用的是 0/0（旧 writer）形状，不是迁移后的 `goods=amount`/无行形状；`tcc_precheck_integration_test.go:174-196` 的部分退款只有预检，没有端到端 TCC 用例。 | 对 `composition_known=0` 的组，整组退款必须可按 ADR §4.3 执行（例如新 commitment 以 `composition_known=0`/仅额度形式表达，或先由人工/T3.4 补齐行）；行级部分退款才应 fail-closed `REFUND_ADMISSION_CONFLICT`。当前“任何不完整组成都整单阻断”需要 Architect/Developer 按 ADR 收口，并补迁移后真实历史行形状的端到端测试（AC-65 的部分退款分支）。若 T3.4 才拥有组成补齐，应在 Plan 中显式声明 T1.2 对历史 commitment 的依赖/延期，不能默认所有订单无 commitment。 |
| **M2** | **MAJOR** | 无活动组 / AC-51、AC-65、ADR Amd1 §4.1.3(7) | `internal/services/order/tcc_settlement.go:262-290`（`createTccCompensationTx` 在 Σ=0=remaining 后直接 `markOrderRefundingTx`、建 batch、写组；没有 `ActiveGroupCount==0` 的成功分支）；AC-65 `docs/specs/0002-payment-settlement-admission.md:378`（AC-51 的“全组已退完→幂等成功”在 `:243-244`）；ADR `:250`；测试仅预检 `tcc_precheck_integration_test.go:197-219` | 当所有组都已全额退款/剩余为 0（Σ 活动组=0，remaining captured=0）时，契约要求“补偿幂等成功，不再调用 Payment”。实现仍会把订单置 `REFUNDING` 并创建一条**无 application 的 FROZEN batch**，没有 T1.4 worker 时会永久悬挂。一致数据下 `refund_state=FULL` 会在入口早退，因此该问题主要在订单退款投影滞后/历史不一致时触发，但它仍是 AC-51/AC-65 的明确分支，且无测试。 | 在 Tx1 入口显式处理“无活动组”：幂等成功返回；不得写 `REFUNDING`/空 batch/任何 permit/commitment；不得调用 Payment。补“全组已退完”的端到端 TCC 用例（不是只测 `precheckTccSum`）。 |
| **m1** | MINOR | R-1 审计完整性 | `internal/services/order/tcc_settlement.go:306-331`（resume 的 `recheckTccSumTxExcluding` 失败返回普通 `apperr.Failed`）；`:212-218` 只对 `tccSumMismatch` 写审计；Addendum R-1 §2.2.1 | 初次预检/初次 Tx1 重检的 Σ 不等会写 `PRECHECK_FAILED`，但**续跑路径的 Σ/快照变化**（`remaining` 或 `sum_group_cap` 与冻结 batch 不等）不写审计 batch，只返回 FAILED_PRECONDITION。仍然是 fail-closed，但审计/人工入口不完整，与 R-1“Σ 不等（含 validateTccSum 失败）必须写审计”存在字面差距。 | 明确续跑快照不一致的所有权：要么写 `PRECHECK_FAILED`/`MANUAL_HOLD` 审计事实，要么在 ADR/Plan 中明确该分支只由 T1.4 的 `MANUAL_HOLD` 承担；补测试断言。 |
| **m2** | MINOR | 批次续跑状态集 | `internal/services/order/tcc_settlement.go:446-458`（`MANUAL_HOLD/FAILED_PERMANENT` 被列为 resumable）；ADR §6.1“重试时复用同一 batch 与同一 refund_id（state CAS 回 PAYING/ADMITTING）”；AC-62 | T1.2 的 resume 把 `FAILED_PERMANENT/MANUAL_HOLD` 当作普通可续跑 batch，只补缺失行、不做状态 CAS；T1.2 目前不会产生这些状态，但未来 T1.4 产生后，DTM 自动重试可能绕过人工处置语义。 | 续跑只接受 `FROZEN/ADMITTING/ADMITTED/PAYING`（或按 ADR 显式 CAS 回 `PAYING/ADMITTING` 后继续）；`MANUAL_HOLD/FAILED_PERMANENT` 必须 fail-closed/人工，不自动续跑。 |
| **m3** | MINOR | `EXCEPTION` 判定的 fail-open 边角 | `internal/services/order/tcc_settlement.go:172`：`exception := TrimSpace(...); if exception != "" && exception != "NONE"`；迁移 `migrations/fulfillment/0004_carrier_delivery.sql` 默认 `NONE` | 空 `exception_state`（proto 缺省/旧行）会被当作 `NONE`；若该行 `shipped_at_unix>0`，会被判“未离仓”并进入撤 permit/commitment 的 PRE 路径。当前 DB 默认 `NONE`，生产风险低，但 R-2 要求“不明确 fail-closed”。 | 空/未知 `exception_state` 且有 `shipped_at` 时应 fail-closed 或按 `EXCEPTION` 处理，不得默认 `NONE`；补一个空值用例。 |
| **m4** | MINOR | PENDING+captured guard 的输入权威性 | `internal/services/order/tcc.go:199-210`（`paid := req.GetPaidCent(); if <=0 用 record.PaidCent`；guard 基于此值）；真实调用方 `internal/services/payment/tcc.go:48-50` 传 `PaidCent: in.AmountCent` | 真实调用方总会带 captured 金额，因此生产 guard 行为符合 R-3；但若请求 `paid_cent=0`（伪造/旧 replay）而订单实际 captured>0，`tccCancelDecision` 会走 PENDING 本地取消，释放预留并成功返回，不退款。Addendum 明确“请求的 paid_cent 即 captured”，所以这不是契约违背，但仍是 fail-open 的信任边界。 | 若要保持 R-3 的强 fail-closed，在不调用 Payment 的前提下至少拒绝“PENDING + 请求未提供 captured”的补偿请求，或由调用链保证 `paid_cent` 必填并在协议层校验；记录为残余风险。 |
| **m5** | MINOR | 审计 batch 竞争 | `internal/services/order/tcc_settlement.go:368-395`（`MAX(attempt_seq)+1` 非事务分配，重复键最多重试 3 次） | 高并发 Σ 失败时第 4 个请求可能拿不到 attempt_seq，只记录日志、丢失审计行；补偿本身仍 fail-closed，不发生错误退款。 | 审计分配应可重试/可观测（或使用序列/重试上限 + 告警），确保 R-1 审计不丢；补并发失败用例。 |
| **m6** | MINOR | 测试真实性/迁移 | `internal/services/order/settlement_integration_test.go:46-210` 使用内联 DDL 创建 `refund_batches/refund_applications/order_refund_commitment_lines`；这些表没有真实 migration 的 CHECK/枚举/索引；T5.4 仍待办 | 逻辑测试是真实 MySQL，SQL 真实执行；但没有跑 `migrations/order/0017/0018`，也没有验证状态枚举/金额 CHECK/真实列长度。测试中构造的 `PAID+SHIPPED`、`0/0 legacy` 也偏离真实状态（见 B1/M1）。 | T5.4 必须用真实迁移跑；T1.2 至少补一条“按迁移 DDL 建表”的 smoke（或明确由 T5.4 承担并在 Plan 中登记）。Mock 必须符合 Article IV.3 的真实业务边界。 |
| **m7** | MINOR | G-1 commit 元数据 | `git log 4e5d07f` 的受管提交有 `change_id/Actor/Role`，但**无 `Reviewed-by`**；`scripts/collab_gates.py:135-138` 要求 HEAD 三者齐全且 `Actor != Reviewed-by` | WIP 提交未带 `Reviewed-by`，G-1 在 HEAD 上会 FAIL。Reviewer 的 actor 独立性（reviewer-phase1 ≠ dev-phase1b）本身满足。 | 重审提交必须补 `Reviewed-by`（不得等于 `Actor`），或按流程把 Review trailer 放到复审后的提交；不能以“首次复审后再补”绕过 G-1 的 HEAD 检查。 |
| **m8** | MINOR | 证据索引/台账漂移 | `docs/lead/PAY-SETTLE-01/evidence-matrix.md` 仍把 AC-51/AC-65 标 `NOT STARTED`；`docs/collab/ac-coverage.tsv:20/34` 仍 `DEFERRED`；`docs/lead/PAY-SETTLE-01/tasks.md:20-31` 的 T1.2/T1.2b/T1.3 仍未勾选 | **没有 AC-VERIFIED 误标**（这是好的）；但 Lead 的 evidence matrix/tasks 没有同步 T1.2/T1.2b 的实现状态，Reviewer 台账仍是 deferral。 | Lead 更新 Plan/Tasks/evidence 指针；Reviewer 台账在 T1.4/T1.7 补齐前继续 DEFERRED，不得写成 AC-VERIFIED。 |
| **M3** | **MAJOR（治理）** | Plan 批准状态 / 宪法 Workflow §8、CONVENTIONS §3 | `docs/lead/PAY-SETTLE-01/tasks.md:3` 仍写 `Status: DRAFT — not approved. ... 未批准前不得写业务代码`；`docs/review-tracking.md` 只记录了 “Phase 1 parallel authorization” 的 scoped waiver；Addendum §5 要求 Plan 更新 T1.2/T1.2b/T1.7；“integration_owner” 未声明 | 受管代码已在 `12858d4/4e5d07f` 提交，但 Plan/Tasks 自身仍声明未批准；目前唯一可引用的授权是 review-tracking 中的 Director 并行授权（豁免 stop condition），不是 Plan 批准记录。若该授权即批准，Plan/Tasks 必须回写；否则违反“Plan 未批准不得写业务代码”。 | Lead 必须把 Director 的授权/Plan 批准落到 Plan/Tasks（含 integration_owner、allowed_paths、affected_AC），或由人类明确该提交的 Plan 基线；Reviewer 不能替 Lead 认定批准。 |

---

## 3. 已核对通过项（按审查重点）

### 3.1 R-1：`PRECHECK_FAILED` 仅审计

- **初次 Σ 不等**：`tcc.go:237-256` 在外部预检失败后只调用 `recordTccPrecheckFailure`，然后返回原 `FAILED_PRECONDITION` 与 details；`recordTccPrecheckFailure`（`tcc_settlement.go:363-396`）只 INSERT `refund_batches`，字段为 `source=tcc`、`state=PRECHECK_FAILED`、`captured/refunded/sum_group_cap/remaining`、`version=1`。
- **Tx1 重检失败**：`freezeTccCompensationTx` 在订单锁内 recheck，失败返回 `tccSumMismatch`（`tcc_settlement.go:262-270`）；`applyTccCompensation`（`:206-219`）在 `db.WithTx` 回滚后才写审计，故订单/permit/commitment/application/outbox/status log 零写入。
- **重复失败**：`recordTccPrecheckFailure` 用 `MAX(attempt_seq)+1` 分配新 attempt；测试 `TestTccCancelSigmaMismatchWritesAuditBatchOnly`（`tcc_compensation_integration_test.go:131-185`）断言恰好 1 条审计、其他表/outbox/status log 0、订单 version 与 permit 未变；`TestTccCancelPrecheckFailureAuditDoesNotBlockNextAttempt`（`:190-248`）断言两次失败得到 2 条审计、修复后新 FROZEN attempt_seq=3、再跑复用同一 batch。
- **不 resume**：`findTccBatchForResume`（`tcc_settlement.go:446-458`）对 `PRECHECK_FAILED` 不加入 resumable，也不当作 completed；测试覆盖。
- **结论**：R-1 的初次/初次 Tx1 重检路径 **PASS**；续跑快照不等无审计是 m1（MINOR）。

### 3.2 R-2：`ListShipments` 与已离仓判定

- **事务外读取**：`TccCancelMarkPaid` 在 `applyTccCompensation` 之前调用 `loadTccShipmentFacts`（`tcc.go:275-281`）；`loadTccShipmentFacts`（`tcc_settlement.go:117-158`）用 `ListShipments(order_no, page=1, size=100)`，`total>100` 或返回数≠total、nil shipment、同组多个 live shipment 均 fail-closed；Tx1 只接收 `tccShipmentFacts` 快照，事务内无远程调用。
- **已离仓判定**：`tccShipmentFacts.shipped`（`:163-192`）按 Order 组 `SHIPPED/RECEIVED`、Fulfillment shipment `SHIPPED/RECEIVED`、`exception_state != NONE && shipped_at_unix > 0` 判已离仓；`exception_state` 非 NONE 但无 `shipped_at`、Order 组状态非 PENDING/ALLOCATED 且无 shipment 均 fail-closed。
- **POST 路径**：`writeTccBatchGroupsTx`（`:555-568`）对 shipped 组设 `POST_SHIP_ALLOWED`，跳过 `decideTccPermitTx`，只写 application，不撤 permit、不写 admission/commitment；测试 `TestTccCancelExceptionShippedGroupIsPostShipAllowed`（`:944-983`）、`TestTccCancelShippedGroupIsPostShipAllowed`（`:424-459`）通过。
- **不可用/不明确**：`TestTccCancelExceptionWithoutShippedAtFailsClosed`（`:987-1016`）、`TestTccCancelShipmentFactsUnavailableFailsClosed`（`:1020-1044`）通过，断言零写入。
- **结论**：R-2 机制 **PASS**；但 B1 使“订单已推进后的 shipped 组”无法进入该机制，且“多 live/incomplete”没有测试（T1.7 补）。`exception_state` 空值见 m3。

### 3.3 R-3：PENDING+captured fail-closed 与无捕获本地取消

- **入口 guard**：`tcc.go:199-210` 对 `PENDING_PAYMENT && paid>0` 返回 `TCC_PENDING_CAPTURED_NOT_IMPLEMENTED`（`apperr.Failed`），在 precheck/写之前；Fulfillment/Payment/订单/许可/批次零写入。测试 `TestTccCancelPendingCapturedGuardWritesNothing`（`:865-906`）断言 `getCalls==0`、订单 version/refund_state 不变、无 batch/permit。
- **Tx1 guard**：`tcc_settlement.go:242-244` 对 `locked.Status==PENDING_PAYMENT && snapshot.CapturedCent>0` 返回同一错误；虽该分支在正常入口下被前置 guard 覆盖，代码层面仍是 fail-closed（无测试直接命中，见 m4）。
- **不误伤无捕获 PENDING**：`TestTccCancelPendingWithoutCaptureStillCancelsLocally`（`:910-939`）通过：`PAID` 前本地取消、`order.cancelled` 1 条、无 batch、无 Payment。
- **结论**：R-3 **PASS**（真实调用方路径）；残余信任边界 m4。

### 3.4 T1.2 / T1.3

- **Σ 接线**：`TccCancelMarkPaid` 对非 REFUNDING 先 `precheckTccSumWithSnapshot`，再 `applyTccCompensation`（`tcc.go:232-284`）；预检读 Payment 权威快照在事务外（`tcc_precheck.go:88-151`）。
- **订单锁内同一快照复核**：`freezeTccCompensationTx` → `findOrderForUpdate` → `createTccCompensationTx` → `recheckTccSumLocked`（`tcc_settlement.go:224-289`）；`TestTccSumTxRecheckUsesSameSnapshot`（`tcc_precheck_integration_test.go:342-371`）断言锁内无第二次 Payment 调用、stale snapshot fail-closed。
- **REFUNDING 不早退**：入口只对 `FULL/CANCELLED` 早退（`tcc.go:190`），`REFUNDING` 进入 resume；`TestTccCancelRefundingOrderWithoutBatchCreatesBatch`（`tcc_compensation_integration_test.go:630-657`）通过（PAID 状态）。**但 B1：REFUNDING + ALLOCATED/SHIPPED 仍在 `tcc.go:190-196` 早退。**
- **batch/application 幂等**：`insertTccBatchTx`（`:482-510`）`MAX+1` + PK；`ensureTccApplicationTx`（`:659-711`）`ON DUPLICATE KEY` + 读回校验；`ensureTccRemoteAdmissionTx`/`ensureTccCommitmentTx` 同样幂等并校验组成。
- **NONE→REVOKED / CONSUMED 不 REVOKED**：`decideTccPermitTx`（`:607-649`）CAS NONE→REVOKED；CONSUMED 返回 `PENDING_REMOTE`，只写 admission + commitment；`TestTccCancelCreatesBatchApplicationsAndRevokesPermits`（`tcc_compensation_integration_test.go:253-325`）、`TestTccCancelConsumedPermitWritesRemoteAdmissionWithoutRevoking`（`:330-388`）通过。
- **G1**：`computeTccPrecheck` 无组时按 G1（`tcc_precheck.go:223-245`）；`ensurePermitTx` 创建 G1 后 revoke；`TestTccCancelLegacyOrderInitializesG1Permit`（`tcc_compensation_integration_test.go:392-420`）通过。
- **并发/重复幂等**：`TestTccCancelConcurrentAndRepeatedIsIdempotent`（`tcc_compensation_integration_test.go:463-523`）、`TestTccCancelConcurrentPrecheckConvergesOnFrozenBatch`（`:802-846`）通过（-race 下复跑通过）。
- **resume 补缺**：`TestTccCancelResumeFillsMissingApplicationsAndPermits`（`tcc_compensation_integration_test.go:527-626`）删除 application/permit/admission/commitment 后重启，断言同一 batch、1 条 application、permit 正确；`TestTccCancelResumeBlocksWhenPaymentSnapshotChanges`（`:661-693`）断言快照变化时 batch/application 不变。
- **越界检查**：`TccCancelMarkPaid` 不调用 `AdmitRefund`/`Payment.Refund`/`Fulfillment.CancelIfCreated`，不 append `order.refund_requested`（`grep` 与 `tcc_compensation_integration_test.go:322-324` 均验证）；`git diff 998e896..4e5d07f --name-only` 无 `api/proto`、`migrations/`、consumer 文件；只改 `ports.go` 增加 existing `ListShipments` 方法（Addendum R-2 允许，无 proto 变更）。
- **T1.4/T1.6 标注**：`tcc.go:168-176`、`tcc_settlement.go:22-30`、`tcc_settlement.go:603-606` 注释明确 AdmitRefund/Payment/CancelIfCreated/T1.4 与 PENDING+captured→T1.6；`tasks.md` T1.4/T1.6 仍为未完成。**标注正确**，但这也意味着 T1.2 返回成功后没有任何 worker 继续驱动 batch（T1.4 未实现）；在 T1.4 缺失期间，捕获资金不会自动退款。当前 WIP 不合并可接受，但重审/合入前必须把“批次无消费者/无 worker”作为显式依赖，不得把 T1.2 的 `return nil` 当成退款完成。

---

## 4. 测试与环境

- **代码 head**：`4e5d07f`；工作区 HEAD `f972cb2`（仅 docs commits），`internal/` 与 `4e5d07f` 完全一致。
- **MySQL**：`mysql-unicorn`，`127.0.0.1:3306`；DSN 经 `./scripts/test-dsn.sh` 生成：
  - `root:root@tcp(127.0.0.1:3306)/mall_integration_order?parseTime=true`
  - `root:root@tcp(127.0.0.1:3306)/mall_integration_order_race?parseTime=true`
- **Go**：`go1.27.1 darwin/amd64`；`GOCACHE=$PWD/.tmp/gocache`；`GOPROXY=off`；模块缓存使用已预填充的 `/Users/lWoHvYe/go/pkg/mod`。说明：`.tmp/gopath` 的 module cache 为空，首次 `go build ./...` 在 `goproxy.cn` 拉取超过 10 分钟仍未完成（未用于任何结论），改用已存在的模块缓存 + `.tmp/gocache` 离线构建；源码无差异。
- **工作区完整性**：构建时 `-mod=mod` 曾把 `google.golang.org/genproto/googleapis/rpc` 从 indirect 提升为 direct（与本次审查无关）；已用 `git restore` 恢复 `go.mod`。最终工作区除本报告外无代码/契约改动。
- **命令与结果**：

| 命令 | 结果 |
|---|---|
| `go build ./...` | exit 0，2m19s |
| `go test ./internal/services/order -run 'TestTcc' -count=1 -v` | **PASS**，25.968s（wall 44.148s）；覆盖 R-1/R-2/R-3、T1.2/T1.3、Σ、G1、CONSUMED、并发、resume |
| `go test ./internal/services/order -count=1` | **ok**，48.369s（完整 order 包，含既有基线用例） |
| `go test -race ./internal/services/order -run 'TestTcc' -count=1` | **ok**，27.796s（wall 1m44s） |
| `go vet ./internal/services/order` | exit 0 |

- 与 Developer 声明耗时（order 38.806s / TCC 15.376s / -race 16.370s）存在机器/缓存差异；本 Reviewer 的复跑全部通过，但**不接受口述耗时替代自检清单**（B2）。

---

## 5. 未覆盖 / 不适用范围

- **未覆盖**：T1.4（`AdmitRefund`/逐组打款/`CancelIfCreated`）、T1.5（`order.refund_requested` 消费者观察化）、T1.6（PENDING+captured→CANCELLED + `pay:<payment_no>:cancel`）、T1.7（完整 AC 用例矩阵）、T2/T3（历史 PAID/组成/责任/168h）、T5.2/T5.3/T5.4（真实 `-race` 全量、故障注入、真实迁移）；本报告不评价这些范围内的实现。
- **未执行**：真实 `migrations/` 迁移/回填；DTM/Outbox/NATS 端到端；Fulfillment 真实 `ListShipments` / `CancelIfCreated`；Payment 真实 `GetIntent`/`Refund`；浏览器/BFF/运营展示；`make gates`/`make verify`/`make test-race` 全量。
- **测试局限**：TCC 测试使用内联 DDL（非 `migrations/`）；`PAID+SHIPPED`、`0/0` legacy 与真实状态/迁移形状不一致（见 B1/M1）；崩溃续跑用人工 DELETE 模拟，不是真实故障注入；没有“真实 Ship vs TCC 并发”的 `-race` 用例（T1.7/T5.2 待办）。
- **AC 状态**：`ac-coverage.tsv` 中 AC-51/AC-65 仍为 `DEFERRED`；本报告**不**标记任何 AC-VERIFIED。T1.2 的测试是 AC-REF/实现证据，不是 Spec 全量验收。AC-66/67/68 不在本实现范围；AC-62（人工释放）不在本实现范围。

---

## 6. 宪法 / 治理逐条核对

| 条款 | 核对结果 |
|---|---|
| Article I.1 状态机/拒绝 if-else 雕花 | 实现有结构化的 Tx1/batch/permit 分支，未新增“默认放行”式补丁；但 `tcc.go:190-196`/`tcc_settlement.go:235-237` 的**静默 return nil** 实质是“默认放行”，B1 违反 fail-closed 精神。 |
| Article I.2 防御性架构（资金/库存/准入） | **B1 违反**：已捕获订单在 ALLOCATED/SHIPPED/COMPLETED 静默成功、不退款。其余 Payment/Fulfillment 不可用均 fail-closed（R-1/R-2/R-3 测试通过）。 |
| Article I.3 契约优先 | 未改 proto/事件；Order port 复用 existing `ListShipments`；符合契约。 |
| Article I.4 拒绝自我妥协 | T1.4/T1.6/T1.5 明确留给后续任务，未假装完成；但 B2 缺自检清单、M3 Plan 仍 DRAFT，交付治理不完整。 |
| Article II.1 事实分离 | TCC 退款走 `refund_applications`/commitment，不再以 `order.refund_requested` 表达；未合并事实。 |
| Article II.2 状态唯一所有权 | Order 写 order refund_state/permit/commitment；Fulfillment shipment 事实通过 `ListShipments` 只读；符合。 |
| Article II.3 幂等与最终一致性 | batch/application/admission/commitment 用 PK/唯一键 + 读回校验；并发测试通过。 |
| Article II.4 渐进式兼容 | 仍是 WIP，未合 main；无 proto/consumer 兼容破坏。 |
| Article II.5 退款与履约准入 / permit 不可逆 | `CONSUMED` 不写 `REVOKED`、POST 不撤 permit 的测试通过；但 B1 让“已离仓 POST”在真实生命周期下不可达，间接违反 permit/退款裁决的覆盖。 |
| Article II.6 读写隔离 | Σ 用 Payment 权威快照，Tx1 不远程调用；测试 `TestTccSumTxRecheckUsesSameSnapshot` 通过。 |
| Article III.1 DTM/Outbox | Tx1 无远程调用，未写 outbox；batch 持久化供续跑；但 T1.4 前的“无 worker 驱动”是已知缺口（§3.4）。 |
| Article III.2 DB 访问 | 显式 `database/sql` + 条件 UPDATE/CAS；未引入 GORM。 |
| Article III.3 多仓/租身份 | 未新增缺省 W1/G1 的跨仓协议；G1 仅用于历史无组回退（AC-65 允许）。 |
| Article III.4 静态守卫 | `apperr` 使用正确；无跨服务包引用；`go build ./...`/`go vet` 通过。 |
| Article III.5 技术栈 | 未引入新依赖/栈。 |
| Article IV.1 测试证据 | 真实 MySQL 测试在 `4e5d07f` 代码上执行；未把 SKIP/FAIL 当 PASS；未标 AC-VERIFIED；但 B2 缺自检清单与测试范围声明。 |
| Article IV.2 重灾区抗压 | TCC 并发/重复用 `-race` 复跑通过；**真实 Ship vs TCC 竞态、故障注入未覆盖**（T1.7/T5.2/T5.3）。 |
| Article IV.3 Mock 真实性 | **B1/M1 违反**：`PAID+SHIPPED` 不是原子投影会产生的状态；`0/0` legacy 不是迁移后的 `goods=amount` 形状。 |
| Article IV.4 组合与 Release 隔离 | 本报告明确实现 FAIL ≠ Release PASS；不批准合并。 |
| Article V AI 安全 | 不涉及（未改 AI/AI 工具/权限）。 |
| Workflow §1/§8 Plan/批准 | **M3**：Plan/Tasks 仍 DRAFT；仅 review-tracking 的并行授权，未回写 Plan/Tasks/integration_owner。 |
| Workflow §9 Stop-the-line | FAIL 后冻结 `PAY-SETTLE-01`；不得继续提交受管代码，修复挂重审 Plan。 |
| Workflow §10 Actor 独立性 | 审查者 `reviewer-phase1` ≠ 代码作者 `dev-phase1b`；报告记录 Actor/Reviewed-by/精确 base/head/review_scope/release_status。 |

---

## 7. 声明

- **代码/契约**：本报告未修改任何代码、proto、migrations、Spec/ADR；只写本报告。
- **结论语义**：`FAIL` 仅表示指定 `998e896..4e5d07f` 范围内 T1.2/T1.2b/T1.3 实现未通过契约审查；**不是** Spec 全量验收、**不是** Release PASS、**不批准合并**。merge/Release 由人类决定。
- **重审前置**：修复 B1/M1/M2 与 M3/B2 后，Developer 需提交自检清单 + 精确 head 的测试日志 + 【测试范围】/【未覆盖范围】；Reviewer 按新 head 重审；`PAY-SETTLE-01` 的冻结状态在重审 Plan 登记后解除。
- **AC 台账**：`ac-coverage.tsv` 的 AC-51/AC-65 继续 `DEFERRED`，未标 `AC-VERIFIED`；T1.4/T1.6/T1.7 完成前不得改写为 PASS。
