# ADR-0007 Amendment 1 Addendum 4 — T1.4b1 Payment Σ / 外部退款竞态裁决

| 项 | 值 |
|---|---|
| Status | **DRAFT — 未批准、未生效；不批准 T1.4b1 实现**；**未并入 ADR-0007 Rev 2.1 (2026-11-25)** |
| Merge note | B-2 合并分类：draft / not normative。R-1…R-5 未被任何批准记录接受；Rev 2.1 只保留已批准条款；其 R-2 涉及的 C-04 状态冲突已由 Director 于 2026-11-25 关闭（Rev 2.1 §18.1 C-1 CLOSED），本文件仍为未批准 DRAFT。 |
| Base | ADR-0007 Amendment 1（accepted 1.0）+ 零元/T1.2/T1.4a Rulings Addenda（accepted） |
| WIP | `1f4f2d1`（T1.4b1：PAYING、逐组 Payment.Refund、payment.refunded 收敛）；Reviewer FAIL B1 |
| Target Spec | **SPEC-0002 v1.9（approved 2026-11-24）**；AC-51/AC-65、INV-48 |
| Change ID | `PAY-SETTLE-01` |
| Scope | 只裁决 T1.4b1 的 pre-payment Σ、外部退款竞态、resume 重算与测试要求；不写代码/迁移/测试 |
| 生效前置 | 本 Addendum QC + Human 批准；T1.4b1-R 实现与 Reviewer 复审；**不批准 `1f4f2d1`** |

---

## 1. 裁决表

| # | 问题 | 裁决 | 依据 | 影响 |
|---|---|---|---|---|
| **R-1** | 任意 `Payment.Refund` 前如何区分“本批已受理/已退但事件未投影”与“外部新退款” | **必须同时读 `Payment.GetIntent` 与 `Payment.GetRefundByKey(refund_id)`**；用 **batch frozen cap `B`** 与 **done/in-flight/remaining** 的不等式判定。不能只依赖 commitment/application 状态或 Payment per-call cap。 | Reviewer B1；ADR §4.1.3(5)；AC-65/INV-48；T1.4b1 `tcc_payment.go:189-288` 未读 Payment remaining；`tcc_settlement.go:401-418` resume 只查 sum==0；`payment/service.go:378-386` 只有单组 cap | T1.4b1-R 重写 preflight；T1.4b1 不得 PASS |
| **R-2** | `remaining != outstanding` 是否一律 MANUAL_HOLD + 零打款；perflight 与逐笔之间的竞态 | **是**：任何非零差额（`external = (B - done) - remaining`）→ **MANUAL_HOLD、零 Payment 调用、零新 SETTLED**；in-flight 不是 external，需先收敛 in-flight，不得开始新组。**仅 preflight + 每笔前重读不能消除竞态**；必须有 **Payment 端原子批量退款/预留**（推荐新 RPC `RefundBatch`），否则不得声称 AC-65 的“不得部分打款”。 | AC-65 `:382`、`:375`；ADR §4.1.3(5) `:249`；Reviewer B1 时序 | **C-04 新 RPC/契约**；T1.4b1-R；无 C-04 则只能保留残余风险并需人类豁免，不得 Release/AC-VERIFIED |
| **R-3** | `ADMITTED/PAYING` resume 如何重算；本批 SETTLED/已受理组如何扣除 | resume 必须重新读 `GetIntent` 与每个 `refund_id` 的 `GetRefundByKey`，计算 `done`/`inFlight`/`outstanding`；**`done` 只计 commitment SETTLED 或 Payment refund 成功**；application DONE 但无 Payment 成功 → **unknown/manual**；不得把 in-flight 当 outstanding 重新付，也不得把外部退款算作本批 done。 | ADR A-07；AC-65 崩溃续跑；T1.4b1 `tcc_payment.go:192-256` 只按 application/commitment 跳过 | T1.4b1-R 重写 resume；T1.4b1-R 测试 |
| **R-4** | 是否可保留逐笔记账 + 每笔前重读，但要求首笔前全量预检 | **可以保留作为 defense-in-depth，但不能单独作为 correctness 证明**：首笔前必须全量预检；每笔前重读用于发现变化并停止后续；外部退款竞态窗口仍由 `RefundBatch` 原子性关闭。无 `RefundBatch` 时，任何“preflight 后外部退款”仍可能导致先付一组、后组失败。 | AC-65；Reviewer B1；T1.4b1 `payTccBatch` 逐组调用 `payTccGroup` | T1.4b1-R + C-04；测试矩阵见 §4 |
| **R-5** | 是否需要 Addendum/Plan T1.4b1-R | **需要本 Addendum + Plan T1.4b1-R**；Plan 必须新增 C-04 `GetRefundByKey`/`RefundBatch`、T1.4b1-R 任务与测试矩阵。 | Plan stop condition；Reviewer FAIL | Addendum 本文件；Plan 更新；不批准代码 |

---

## 2. 可执行算法（pre-payment Σ）

### 2.1 输入与定义

在 **任何 `Payment.Refund` 之前**（首笔前全量；每笔前再执行一次）：

```text
B            = batch.SumGroupCapCent                 // 冻结的组上限之和（不随外部退款变）
intent       = Payment.GetIntent(payment_no)         // captured/refunded 权威
R            = intent.captured_cent - intent.refunded_cent
done         = 0
inFlight     = 0
unknown      = 0
outstanding  = 0
for each application in batch:
    refund = Payment.GetRefundByKey(payment_no, application.refund_id)
    commitment = order_refund_commitments[application.refund_id]
    if refund.status ∈ {REFUNDED, SUCCEEDED}:
        done += application.amount_cent
    else if refund.status ∈ {PENDING, PROCESSING}:
        inFlight += application.amount_cent
    else if commitment.state == SETTLED:
        done += application.amount_cent           // 事件可能晚到，但 Payment 事实已 SETTLED
    else if application.state == DONE or commitment.state == RESERVED:
        // DONE 但无 Payment 成功：不可信；RESERVED 且无 refund：待付
        if application.state == DONE:
            unknown += application.amount_cent
        else:
            outstanding += application.amount_cent
    else:
        unknown += application.amount_cent
```

- `done` 是本批**已经退款成功**的组；`inFlight` 是本批已有 Payment refund 但未完成；`outstanding` 是本批尚未发起退款的组。
- `B = done + inFlight + outstanding` 必须成立；否则 `unknown/manual`。
- `external = (B - done) - R`：
  - `external == 0`：没有外部退款/额外退款，`R == inFlight + outstanding`。
  - `external > 0`：存在外部新退款（或未投影的本批退款），`R < B - done` → **MANUAL_HOLD、零打款**。
  - `external < 0`：remaining 大于本批未完成额度（额外捕获/账实不符）→ **MANUAL_HOLD、零打款**。
- `inFlight > 0`：**不得开始任何新组 Payment**；只能对同一 `refund_id` 幂等重试/查询收敛；in-flight 归零后才允许新组。
- `unknown > 0`：MANUAL_HOLD/告警；不得猜 DONE 或再次付款。

### 2.2 门禁

```text
if B != done + inFlight + outstanding:      MANUAL_HOLD
if unknown > 0:                              MANUAL_HOLD
batchUnsettled = B - done                  // 含 inFlight + 尚未发起的组
if R != batchUnsettled:                    MANUAL_HOLD（零 Payment；外部退款/账实不符）
if inFlight > 0:                           等待/重试 in-flight；不得新组
if inFlight == 0:
    require R == outstanding                 // 等价于 Σ 活动组 == remaining
    然后才允许逐组 Payment.Refund
```

- `external != 0` 的判定必须在首笔前；任何一笔之后重新执行；若中途变化，立即停止后续 Payment，batch `MANUAL_HOLD`；已成功组保留 `SETTLED`，不得回滚。
- `B` 是冻结值；不得用当前组 cap/Commitment 重新定义 B。
- `GetRefundByKey` 超时/不可用：fail-closed，不 Payment；重试/人工。
- `GetIntent` 的 `refunded` 包含外部与本批退款；`GetRefundByKey` 用于归因，不能只看 `R`。

---

## 3. 外部退款竞态与 Payment 原子性（C-04）

### 3.1 结论

- **仅靠 `GetIntent` + `GetRefundByKey` + 每笔前重读，不能消除“preflight 之后、Payment.Refund 之前”的外部退款窗口。** 例：`B=10000`（G1=6000/G2=4000），preflight `R=10000`；外部退款 1000 后 G1 6000 可过、G2 4000 失败 → **G1 已付、G2 拒绝 = 部分打款**。
- Payment 当前 per-call cap 只保证单组不超 `captured-refunded`，不是 batch Σ。
- **需要 Payment 端 batch 级原子契约**（C-04）：
  - 推荐新增 `Payment.RefundBatch`：`payment_no`、`batch_id`、`items[{refund_id, amount_cent}]`；Payment 在 `payment_intents` 行锁内：
    1. 校验 `Σ items.amount_cent <= captured - refunded`；
    2. 按 `refund_id` 幂等；
    3. **全成功或全失败**（不产生部分 refund/outbox）；
    4. 返回逐 `refund_id` 结果；`INSUFFICIENT_REMAINING`/`BATCH_SUM_MISMATCH` 零写入。
  - 备选 `ReserveRefundBatch` + per-group `Refund`（需 reservation token/release），复杂度更高；本 Addendum 推荐原子 `RefundBatch`。
- 在 C-04 `RefundBatch` 未合入前：T1.4b1 可保留 preflight/每笔前重读作为 defense-in-depth，但**不得声称 AC-65 “不得部分打款”**；需要人类书面豁免残余风险，且不得 Release/AC-VERIFIED。

### 3.2 与 `GetRefundByKey` 的关系

- `GetRefundByKey` 用于 resume/已受理/事件丢失归因，不替代 `RefundBatch` 的原子性。
- `GetRefundByKey` 必须返回 `refund_id`/status/amount；缺失时 C-04 阻塞 T1.4b1-R；不得用 `ListRefunds`/`refund_no` 猜。

---

## 4. 测试矩阵（必须）

| 场景 | 初始 | 期望 |
|---|---|---|
| 外部退款 1000 | B=10000, G1=6000/G2=4000, preflight R=9000 | `external=1000` → MANUAL_HOLD；**零 Payment.Refund**；G1/G2 无 SETTLED |
| 外部退款 6000 | R=4000 | MANUAL_HOLD；零 Payment；不得付 G2 |
| 外部退款 4000 | R=6000 | MANUAL_HOLD；即使 G1=6000 单组可过也不得付 G1 |
| 本批 G1 已退但事件未投影 | GetRefundByKey(G1)=REFUNDED；commitment RESERVED | done=6000；不再付 G1；若 R=4000 只付 G2；不得重复 |
| 事件丢失 | GetRefundByKey 成功；commitment RESERVED；app PROCESSING | 不重复 Payment；记录/触发 lazy reconcile；batch 不提前 DONE；不得当作 external |
| in-flight | GetRefundByKey=PENDING/PROCESSING | 不开始新组；只重试/查询同一 refund_id；in-flight 收敛后再计算 |
| ADMITTED/PAYING resume | G1 SETTLED，G2 RESERVED；R==G2 | 只付 G2；不重付 G1 |
| resume 外部退款 | G1 SETTLED，外部退款 1000 | external>0 → MANUAL_HOLD；不得付 G2 |
| `RefundBatch` 原子性 | 外部退款在 preflight 后、RefundBatch 前 | RefundBatch 全失败/零写入；不得部分成功 |
| GetRefundByKey 超时 | 任意 | fail-closed；零 Payment；告警/重试 |

---

## 5. Plan T1.4b1-R

| ID | Owner | 任务 |
|---|---|---|
| **R-Σ1** | Developer | 实现 §2 算法：B/done/inFlight/external/outstanding；首笔前全量、每笔前重读；`external!=0`/unknown → MANUAL_HOLD 零打款 |
| **R-Σ2** | Architect + Developer | C-04 新增 `GetRefundByKey` + `RefundBatch`（atomic all-or-none）；Order 改用 RefundBatch；无该 RPC 不得声称 AC-65 通过 |
| **R-Σ3** | Developer | ADMITTED/PAYING resume 重算；SETTLED/GetRefundByKey 成功扣除；in-flight 不新付 |
| **R-Σ4** | Developer | §4 全部测试；外部 1000/6000/4000、事件丢失、in-flight、resume、RefundBatch 原子性 |
| **R-Σ5** | Reviewer | 精确 base/head 复审；B1 关闭前不合并 |

**Plan 更新**：新增 T1.4b1-R；C-04 记录 `GetRefundByKey`/`RefundBatch`；T1.4b1 不得以“Payment 会拒绝某组”作为 Σ 兜底。

---

## 6. 状态与声明

- **Status: DRAFT**。本 Addendum 需 QC + Human 批准；不批准 `1f4f2d1` 实现。
- 不修改 ADR-0007 Amendment 1/Rev 1.0、既有 Addenda、SPEC、代码/迁移/测试。
- B1 关闭条件：§2 算法 + §3 C-04 原子 batch + §4 测试 + Reviewer 复审；在此之前 T1.4b1 保持冻结。
