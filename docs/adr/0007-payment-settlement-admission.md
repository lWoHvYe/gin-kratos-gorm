# ADR 0007: 支付结算与履约准入（P0-A / P0-B）

- Status: **accepted（Revision 2.3）**——Rev 2.2 的增量：修复 N5 配置错误——`SETTLE_RELEASE_APPROVAL_ID` 非空但未请求 `settled` 时 **启动失败（非零退出）**，不得默认 `paid`；同步 SPEC 指针至 v1.10。**不新增业务规则**。
- Approved: **2026-11-23**（Rev 1.0，Director）；**2026-11-24**（Amendment 1、Addendum 1 零元售后、Addendum 2 T1.2 rulings、Addendum 3 T1.4a rulings、C-04 RefundBatch 契约）；**2026-11-25**（T1.6 payment-level cancel Addendum；T1.4b2 Addendum 撤回；Rev 2.0 合并版；Rev 2.1；Rev 2.2 启动校验 BLOCKER 修复；Rev 2.3 N5 修复）。逐条批准事实见 §17.1。
- Revision: **2.3**（增量：Rev 2.2 + N5 approval-id/mode 配对失败 + SPEC v1.10 指针；Rev 2.0/2.1/2.2 历史见 §17.3）
- Date: **2026-11-25**
- Supersedes（作为规范文本）：
  - 本文 Rev 1.0 正文（历史版本，条款已并入本版）；
  - [0007-payment-settlement-admission-amd1.md](0007-payment-settlement-admission-amd1.md)（Amendment 1，accepted 2026-11-24）；
  - [0007-amd1-addendum-zero-after-sale.md](0007-amd1-addendum-zero-after-sale.md)（Addendum 1，accepted 2026-11-24）；
  - [0007-amd1-addendum-t12-contract-rulings.md](0007-amd1-addendum-t12-contract-rulings.md)（Addendum 2，accepted 2026-11-24）；
  - [0007-amd1-addendum-t14a-contract-rulings.md](0007-amd1-addendum-t14a-contract-rulings.md)（Addendum 3，accepted 2026-11-24）；
  - [0007-amd1-addendum-t16-payment-cancel.md](0007-amd1-addendum-t16-payment-cancel.md)（T1.6，accepted 2026-11-25）。
  - [0007-amd1-refundbatch-c04-rulings.md](0007-amd1-refundbatch-c04-rulings.md)（C-04 语义裁决 R-1…R-6；内容随 C-04 accepted 2026-11-24 并入）；
  - [0007-amd1-refundbatch-c04-draft.md](0007-amd1-refundbatch-c04-draft.md)（C-04 契约，accepted 2026-11-24）。
  - 上列文件按 **superseded by ADR-0007 Rev 2.0/2.1/2.2/2.3 (2026-11-25)** 标记，仅保留为历史与审计证据。
- 未并入（历史保留；**不是** Rev 2.3 的规范来源）：
  - [0007-amd1-addendum-t14b1-payment-sigma.md](0007-amd1-addendum-t14b1-payment-sigma.md)——**DRAFT（未批准、未生效）**，其 R-1…R-5 未被任何批准记录接受；不得作为实现/验收依据（C-04 的原子性以本文 §5.9 为准，不由该文件授权）。
  - [0007-amd1-addendum-t14b2-inv44-group.md](0007-amd1-addendum-t14b2-inv44-group.md)——**superseded / withdrawn（2026-11-25）**，已由 `docs/lead/PAY-SETTLE-01/t1.4b2-design.md` Rev 1.3 取代；其 B1/B2/B3 不并入本 ADR。
  - [0007-payment-settlement-admission-amd1-c04-c05-handoff.md](0007-payment-settlement-admission-amd1-c04-c05-handoff.md)——**handoff（契约输入清单，非批准）**；其引用的 D-27/D-28/D-29 输入已在正文实现，文件本身不构成契约。
- Spec: [SPEC-0002 v1.10](../specs/0002-payment-settlement-admission.md)（v1.10；本 ADR 未修改 SPEC；SPEC 的 `依据` 行已指向本 ADR **Rev 2.3**（PO 2026-11-25 修复））。
- Integration owner: **Director**（人类）；设计 owner/秘书：System Architect；deputy：待指派
- Related: [architecture-baseline.md](../architecture-baseline.md)（INV-01…16）、[ADR 0005](0005-fulfillment-quantity-split.md)（履约/售后基线）、[ADR 0006](0006-collab-gates.md)（retired）、[ext-review-1123-triage.md](../ext-review-1123-triage.md)
- Change ID: `PAY-SETTLE-01`
- Baseline impact: `INV-01/04/05/06/07/14/21/24/26/30…50`；`AC-39…68`
- 阅读约定：正文中的 `〔来源〕` 标记指出该条款合并自哪个已批准文件/章节；**裁决 ID 原样保留**（`D-01…D-14`、`A-01…A-14`、`AZ-01…AZ-06`、`D-22…D-29`、`T1.2/R-1…R-3`、`T1.4a/R-A…R-C`、`T1.6/CB-1…CB-5`、`C-04/R-1…R-6`）。命名空间前缀仅用于消除跨文件同名 ID 的歧义（例如 Rev 1.0 §13 的 `R-2/R-3/R-4` 记为 `QC08/R-2…`）。
- 未裁决事项**不进入**正文规范条款；Rev 2.0 记录的两项冲突（C-1/C-2）已于 2026-11-25 裁决并在 §18 关闭，正文按裁决结果收敛，不得再由实现自行选择。

---

## Merge map（Rev 2.3 § → 来源）

| Rev 2.3 § | 内容 | 来源（文件 §） |
|---|---|---|
| 头部/状态 | 版本、批准、supersede 清单 | Rev 1.0 头 + Amd1 头 + Addenda 头 + `docs/review-tracking.md` 2026-11-24/2026-11-25 |
| §0.1 | `D-01…D-14` 决策摘要 | Rev 1.0 §0 |
| §0.2 | `A-01…A-14` 决策摘要 | Amd1 §1 |
| §0.3 | `AZ-01…AZ-06` 零元售后决策 | Zero Addendum §1（+ §1.1 A-Z1…A-Z8、§1.3 QC-17 B-1/B-2/B-3） |
| §0.4 | `T1.2/R-1…R-3` 契约裁决 | Addendum 2 §1（+ §2/§3/§4） |
| §0.5 | `T1.4a/R-A…R-C` 契约裁决 | Addendum 3 §1（+ §2/§3/§4） |
| §0.6 | `T1.6/CB-1…CB-5` 实现契约基线 | T1.6 §4.2（+ §1/§2/§3） |
| §0.7 | `C-04/R-1…R-6` RefundBatch 契约裁决摘要 | C-04 §4 + C-04 rulings §1（accepted 2026-11-24） |
| §1.1 | 事实所有权（含 S15…S19 事实） | Rev 1.0 §1.1 + Amd1 §4.2/§4.3 + Zero §2 |
| §1.2 | 事件契约（含 `order.settled` v1 字段） | Rev 1.0 §1.2 + Amd1 §3.1 + T1.6 §3.1 |
| §1.3 | 事件版本、影子消费者、退役顺序 | Amd1 §3.1/§4.6 + T1.6 §3.3（C-2 已关闭：A-08 观察语义） |
| §2.1–§2.4 | 支付意图、回调事务、金额裁决、结算 worker | Rev 1.0 §2 |
| §2.5 | `Payment.GetRefundByKey` 与退款事实查询 | Amd1 §4.7、A-07、A-12 + handoff §1.2 |
| §2.6 | 单笔退款上限 vs 批级 Σ 边界（C-04 batch 为规范） | Amd1 §4.1.3(5) + C-04 §2/§3（C-1 已关闭） |
| §3.1–§3.6 | 订单结算、就绪校验、CAS 重读、取消、零元、核销 | Rev 1.0 §3 |
| §3.4（扩） | `PENDING_PAYMENT+已捕获` 支付级取消 | Amd1 §4.8 + T1.6 §2/§3 + Addendum 2 `T1.2/R-3` |
| §3.7 | `refundCancelledAfterConfirm` 持久命令 | T1.6 §3.2 |
| §4.1–§4.3 | 资金口径、`payment.refunded` 字段、退款额度 | Rev 1.0 §4 + Amd1 §4.7（惰性对账/唯一定稿人） |
| §4.4 | 历史 `PAID` 生效支付解析（D-28：队列/状态机/schema） | Rev 1.0 §4.4 + Amd1 §5 |
| §5.1 | 退款分类与账本所有权 | Rev 1.0 §5.1 + Amd1 §4.2 + Zero `AZ-01` |
| §5.2 | `AdmitRefund`（增量字段、行级额度、准入持久化、零元短路） | Rev 1.0 §5.2 + Amd1 §4.4 + Zero §3.1 + T1.2/R-2 + T1.4a/R-B |
| §5.3 | `CancelIfCreated` 与已离仓判定 | Rev 1.0 §5.3 + T1.2/R-2 §3.2.5 |
| §5.4 | 支付级退款 | Rev 1.0 §5.4 + Amd1 §4.5/§4.8 + T1.6 §3.1 |
| §5.5 | 三个组级来源 + TCC 补偿编排（AC-65） | Rev 1.0 §5.5 + Amd1 §4.1 + T1.2/R-1 + T1.4a/R-B |
| §5.6 | commitment 组成、释放与人工授权（AC-62/D-24） | Rev 1.0 §5.6 + Amd1 §4.3 |
| §5.7 | 组退款后的库存与 INV-44 owner | Rev 1.0 §5.7 + T1.4a/R-A + Zero `AZ-04` |
| §5.8 | `CONSUMED+CANCELLED` 与零元先取消后事务 | Rev 1.0 §5.2 + Zero §3 + T1.4a/R-C |
| §5.9 | `Payment.RefundBatch` 原子批退款（C-04：proto、Σ、幂等、错误、Order 预检边界） | C-04 §1–§3（accepted 2026-11-24；K-1） |
| §6.2–§6.4 | `order.settled` 影子（D-27）：消费模式、比对、切换/回滚 | Rev 1.0 §6.2/§12.3 + Amd1 §3 |
| §7 | 库存就绪、TOCTOU、缺货 | Rev 1.0 §7 + Zero `AZ-02` + T1.4a/R-A |
| §8 | 状态机与幂等键汇总（含 Payment 批退款账本态） | Rev 1.0 §8 + Amd1 §8 + Zero §4 + T1.6 §2.1 + C-04 §2.6 |
| §9 | 恢复所有者（W1–W5 + W6–W13） | Rev 1.0 §9 + Amd1 §9 + T1.4a/R-A |
| §10 | 错误语义 | Rev 1.0 §10 + Amd1 §2.3 + T1.6 §3.4 |
| §11 | 可观测性与对账 | Rev 1.0 §11 + Amd1 §3.3 + T1.6 §3.4 |
| §12.1–§12.4 | 结构清单、回填、灰度与消费者切换、回滚 | Rev 1.0 §12 + Amd1 §6.1/§10 + Zero §7 + T1.2/R-1 |
| §12.5 | schema 约束设计（C-05 输入） | Amd1 §5.4/§6 + Zero §7 + C-04 §2.8（§12.5.8） |
| §12.5.8 | `refunds` / `payment_refund_batches`（C-04） | C-04 §2.8 + 裁决 R-3/R-5 |
| §13 | 【假设与不确定性】 | Rev 1.0 §13 + Amd1 §11 + Zero §9 + Addendum 2 §3.2 + T1.6 §3.4 |
| §14 | 反模式自省 | Rev 1.0 §14 + Amd1 §12 + Zero §5 + T1.6 §3 |
| §15 | Baseline 影响与 AC 映射 | Rev 1.0 §15 + Amd1 §2.3/§13 + Zero §5 |
| §16 | Consequences | Rev 1.0 §16 + Rev 2.0 合并影响 |
| §17 | 批准、前置条件与版本历史 | Rev 1.0 §17 + 各 Amendment/Addendum 批准记录 + Rev 2.3 批准行 |
| §18 | **冲突裁决与关闭记录**（C-1/C-2 已于 2026-11-25 关闭） | Rev 2.0 冲突证据 + review-tracking K-1/K-2/K-4/K-6 |

---

## 0. 决策摘要

### 0.1 Rev 1.0 决策（`D-01…D-14`）〔来源：Rev 1.0 §0〕

| ID | 决策 | 理由 |
|---|---|---|
| D-01 | **四事实分离**：`PaymentCaptured`（Payment）、`OrderSettled`（Order 的 `PAID`）、`InventoryReady`（Inventory）、`FulfillmentAllowed`（Order 的组级许可谓词）各有唯一所有者；任何一方不得用单一 `order.paid` 推导另外三者 | SPEC-0002 INV-35 |
| D-02 | **捕获后持久结算**：Payment 在写 `SUCCEEDED` 的同一本地事务写 `payment_settlement_tasks`，worker 驱动 DTM/直接结算；W1 由该任务恢复 | SPEC-0002 INV-32/AC-43 |
| D-03 | **首次支付足额**：`CreateIntent` 反查 Order，校验订单存在、用户匹配、`PENDING_PAYMENT`、`amount_cent == pay_cent`；异常捕获不生效并走**支付级退款** | SPEC-0002 INV-30/AC-39 |
| D-04 | **直接/TCC 共用 `settleOrderTx`**：它是唯一把订单置 `PAID` 的写路径；直接路径与 Order TCC 分支都调用它 | SPEC-0002 INV-34 |
| D-05 | **同步就绪校验是 PAID 强制前置**：Order 在结算前调用 `Inventory.CheckOrderReady`（新只读 RPC）；不可用/超时 → fail-closed，不得回退到异步投影；TCC 分支顺序不作为正确性依赖 | INV-06/INV-34；QC-ADR Q-02 |
| D-06 | **CAS=0 必须重读**：只有 `PAID` 且 `settlement_payment_no=本 payment_no` 才幂等成功；`PENDING_PAYMENT` 可重试；`CANCELLED` 返回 `ORDER_CANCELLED` 并退款；其他冲突不触发副作用 | SPEC-0002 INV-33/AC-45 |
| D-07 | **单一生效支付**：`orders.settlement_payment_no` 是唯一生效支付；重复捕获在 CAS 裁决**之后**写 `payment_duplicate_captures` 并走**支付级退款**，不计入订单 captured/refunded | SPEC-0002 INV-31/INV-37/AC-41/AC-53 |
| D-08 | **退款分两类**：**组级退款**（售后/TCC 补偿/缺货）走 `AdmitRefund` + 许可行 CAS；**支付级退款**（非生效捕获/超少付/取消）由 Payment 直接幂等退款，不经组许可，只受 Payment captured 上限约束 | SPEC-0002 INV-36/INV-42/INV-43；QC-ADR Q-06 |
| D-09 | **额度双层 + Payment 权威**：组级退款准入读 Payment `GetPaymentByOrder` 的订单级 captured/refunded；Order 用 `order_refund_commitments` 串行化在途额度；Order 自身退款投影只用于展示 | SPEC-0002 AC-53/AC-55；QC-ADR Q-03 |
| D-10 | **未离仓整组退款**：未离仓组金额必须等于整组退款金额；部分金额返回 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；TCC 按“系统责任、每组全额、不做分摊”，`Σ 活动组系统责任上限 == 订单剩余 captured`，不等则阻断+人工裁决 | SPEC-0002 AC-50/AC-51、D-19/D-21 |
| D-11 | **已授权未离仓窗口**：`permit=CONSUMED` 且运单 `CREATED` 时，退款准入调用 `Fulfillment.CancelIfCreated`；取消成功→`PRE_SHIP_REVOKED`，已离仓→`POST_SHIP_ALLOWED`；Ship 与取消在运单行 CAS | SPEC-0002 §5.2/AC-50 |
| D-12 | **事件契约复用现有 subject**：`shop.payment.succeeded.v1`、`shop.payment.refunded.v1`、`shop.order.paid.v1`、`shop.order.allocation_frozen.v1`、`shop.inventory.confirmed.v1` 继续使用；仅新增 `shop.order.settled.v1`、`shop.order.refund_admitted.v1`；灰度期 `order.paid` 与 `order.settled` 同事务双发 | QC-ADR Q-07；AGENTS §4 渐进兼容 |
| D-13 | **唯一 TCC gid**：复用 `payment-confirm:<payment_no>`；不新增 `payment-settle` gid；worker 启动前查询既有 gid，禁止同一支付两个 TCC 事务 | baseline §2.4；QC-ADR Q-08 |
| D-14 | **零元单不经过 Payment**：Order 提供 `SettleZero(order_no)` 入口，以 `settlement_payment_no=zero:<order_no>` 直接结算；零元退款申请 `DONE(zero_amount)`，不调 Payment | SPEC-0002 AC-40；QC-ADR Q-09 |

### 0.2 Amendment 1 决策（`A-01…A-14`）〔来源：Amd1 §1〕

| ID | 决策 | 映射 |
|---|---|---|
| A-01 | **准入不可逆**：permit 终态不可恢复；commitment 只能按 AC-62 的合法触发 `RESERVED→RELEASED`，且必须人工授权、审计、幂等、CAS/lease；禁止 worker 静默释放 | AC-62 / INV-45 / D-1 |
| A-02 | **退款执行账本**：`refund_applications` 由 **Order** 拥有，记录组级退款申请与 Order 发起的支付级退款意图；AfterSales 是业务发起方但通过 `AdmitRefund` 写入；Payment 是执行方，不写 Order 账本 | C-03 / AC-62 / AC-64 / S8/S15 |
| A-03 | **行级与组成**：commitment 必须有行/商品/运费组成；新承诺 `composition_known=1`，行合计必须等于头金额；历史 `composition_known=0` 只允许整组退款，禁止行级部分退款直到人工补齐 | AC-64 / INV-24 / INV-47 |
| A-04 | **历史解析队列**：`legacy_payment_resolution` 由 Order 唯一写；`order_no` 唯一；状态 `OPEN/RESOLVING/COMPENSATING/RESOLVED/EXPIRED_REFUND/EXPIRED_ZERO/EXPIRED_FINANCE/REJECTED`；24h 到期先 `COMPENSATING`（非 OPEN），补偿后持久化退款意图并 `Payment.Refund`，**Payment 成功后才 `EXPIRED_REFUND`**；不得无限期 `OPEN` | AC-63 / S16 / D-28 |
| A-05 | **TCC 先预检后改状态**：事务外取 Payment 权威快照；Tx1 订单锁下复核 Σ；不等不写任何 permit/commitment/订单退款态；Tx1 同事务写 REFUNDING + batch + 分状态 permit | AC-65 / INV-48 / D-27（Plan） |
| A-06 | **分状态 permit 冻结**：`NONE→REVOKED`（CAS）；`CONSUMED` 不得写 `REVOKED`，只写 `PENDING_REMOTE` admission+commitment，Tx1 后 `CancelIfCreated`；Ship 胜出→`POST_SHIP_ALLOWED`；已离仓组只退款 | AC-65 / AC-50 / §5.2 |
| A-07 | **单一定稿人**：Order 的 `payment.refunded` 消费者是 commitment `RESERVED→SETTLED` 的唯一写入方；`AdmitRefund` 前的惰性对账是补偿路径；Payment 提供按 `refund_id` 查询退款事实的只读 API | AC-62 / AC-55 / §4.3 |
| A-08 | **`order.refund_requested` 观察语义**：保留 subject 兼容；除 T1.6 的支付级取消命令载体外，该 subject 的消费只允许观察/告警，不得据 TCC/售后驱动的退款语义执行 `Refund`；支付级取消由权威命令 `pay:<payment_no>:cancel` 执行（§3.4.2/§3.7），T1.6 是唯一例外；新 TCC 不依赖该事件；移除日期在 ON/消费方迁移后由 Director 定（C-2 已关闭，见 §18.2） | D-5 / AC-62 / INV-36/43 / T1.6 |
| A-09 | **`order.settled` 影子**：R1 只实现 `paid\|shadow` 两态，生产默认 `paid`；shadow 双消费、只记录/比对、不建单；`settled` 需 release 门禁+人类批准 | AC-66 / D-27 / INV-49 |
| A-10 | **切换/回滚/移除**：OFF→SHADOW 需 Director 影子批准；SHADOW→ON 需 `SPEC-0002 §7 矩阵 + 五方终态 + Reviewer 人工核对 + review-tracking 记录（协作闸门已退役，无命令可跑）` + Director；ON 必须记录 `order.paid` 移除日期（建议 ≤ON+30d，超出需 Director 书面批准）；回滚 ON→SHADOW/paid 仅经 §6.4 Director 批准，不重复建单，不得由启动校验自动回 paid | AC-66 / D-27 / INV-49 |
| A-11 | **`GRANTED` 收敛**：v1.7 许可枚举仅 `NONE/CONSUMED/REVOKED`；代码 `GRANTED` 既非持久合法值也不得作为第二状态；预检有行则阻断部署，手工按证据分类；未知状态 fail-closed | D-29 / v1.7 §2/§5.2 |
| A-12 | **API/配置增量**：`AdmitRefund` 增加 `repeated RefundLine`、责任/理由/运费字段、`batch_id`；新增 `Payment.GetRefundByKey`（或 `Refund.refund_id`）；新增 Order 历史队列管理/恢复命令；`FULFILLMENT_SETTLE_CONSUMER_MODE`（paid/shadow/settled）与 `SETTLE_RELEASE_APPROVAL_ID`；全部 additive、不重编号 | C-04 / AC-62/63/64/66 |
| A-13 | **新增结构**：`refund_batches`、`refund_applications`、`order_refund_commitment_lines`、`refund_commitment_releases`、`order_refund_sequences`、`legacy_payment_resolution`、`settlement_shadow_diffs`、`settlement_switch_approvals`；`order_refund_commitments` 增组成列 | C-05 |
| A-14 | **迁移范围**：D-27 的 shadow 表在 Fulfillment DB，C-05 的 `allowed_paths` 必须扩展到 `migrations/fulfillment/**`；否则 D-27 无落点 | D-27 / C-05 |

### 0.3 零元售后决策（`AZ-01…AZ-06`）〔来源：Zero Addendum §1〕

| ID | 决策 |
|---|---|
| **AZ-01** | 零元售后的执行账本放在 **Order**：`refund_applications(source=system_zero, amount=0, state=DONE)` 为**最终**事实；`refund_id=after_sale_no`，`goods_cent=0`、`freight_cent=0`、`non_effective_capture=0`。**`AdmitRefund` 增加零元短路**（不新增 RPC）：`source=system_zero && amount=0 && 订单 pay_cent=0 && 无任何已捕获 intent` 时跳过 Payment 额度/commitment，仍执行 permit/运单/INV-44 准入，同事务返回 DONE；正金额路径不变。**需要回库的退货，回库完成后才调用 `AdmitRefund` 并写 DONE**；REFUND_ONLY/缺货在许可/运单/释放完成后写 DONE。 |
| **AZ-02** | 零元售后**不调用 Payment**，不产生 `refund_no`、不写订单级 `captured/refunded`、不进入 `legacy_payment_resolution`/`EXPIRED_*`/`COMPENSATING`。 |
| **AZ-03** | AfterSales 不新增 `DONE_ZERO` 状态；沿用现有生命周期终态 **`REFUNDED` + `refunded_cent=0`**，`amount_state=CONSUMED`，`refund_no=''`（空字符串，不伪造 Payment 退款号）。 |
| **AZ-04** | 零元售后仍走 `AdmitRefund` 的**零元短路**做组级准入（撤许可/取消未离仓运单/按 INV-44 释放）；短路跳过 Payment 额度与 **0 元 `order_refund_commitments`**；`order_refund_admissions` 照常持久化 `PRE_SHIP_REVOKED/POST_SHIP_ALLOWED`。 |
| **AZ-05** | 这不是新状态机分支：零元短路在 `AdmitRefund` 同一事务直接写 `refund_applications DONE`，**不保留无 worker 的 PENDING 悬挂**；AfterSales 复用 `REQUESTED→APPROVED→REFUNDING→REFUNDED`；`source=system_zero + amount=0 + state=DONE` 是既有状态的值组合，不是新状态。 |
| **AZ-06** | AC-55 的“以 Payment 退款事实为准”仅适用于**有资金退款**；AC-68 已于 **2026-11-24 随零元售后 amendment 批准**，明确完成事实是 Order `refund_applications(DONE, system_zero, 0)` + AfterSales `REFUNDED/0`，不适用 AC-55 的 Payment 优先；正金额退款仍按 AC-55。 |

### 0.4 T1.2/T1.3 契约裁决（`T1.2/R-1…R-3`）〔来源：Addendum 2 §1〕

| ID | 裁决 |
|---|---|
| **T1.2/R-1** | **以 ADR 为准**：Σ 不等必须写 `refund_batches(state=PRECHECK_FAILED)` 审计 batch；**不写**订单/permit/commitment/application/outbox/状态日志。这是审计事实，不是退款状态变更。`PRECHECK_FAILED` 不进入可续跑集合；人工修正后创建新 attempt。 |
| **T1.2/R-2** | **契约缺口，成立**：事实 owner 是 **Fulfillment（S6）**，Order DB 无 `shipment/shipped_at`。T1.2b 必须在 Tx1 外读 `Fulfillment.ListShipments(order_no)`（现有 RPC；`GetShipmentByOrder` 多组会失败），把 `exception_state != 'NONE'`（当前值 `OPEN`）且 `shipped_at_unix>0` 判为已离仓；不可用/不明确 → fail-closed，不写 batch。T1.4 的 `CancelIfCreated` 必须把 `EXCEPTION+shipped_at` 返回 `ALREADY_SHIPPED`，不得默认 `TRANSIENT_ERROR` 无限重试。 |
| **T1.2/R-3** | **语义归 T1.6**：`PENDING_PAYMENT+已捕获` 由 T1.6 实现 CAS→`CANCELLED` + 支付级取消退款；T1.2/T1.3 不得保留旧 `PAID+REFUNDING` 行为，T1.2b 加 fail-closed guard。**该 guard 已被 T1.6 accepted Addendum 取代**（T1.6 §4：本 Addendum 替代其 guard；见 §3.4）。 |

### 0.5 T1.4a 契约裁决（`T1.4a/R-A…R-C`）〔来源：Addendum 3 §1〕

| ID | 裁决 |
|---|---|
| **T1.4a/R-A** | **INV-44 释放/回库 owner 成立，必须补 owner**：事实 owner/执行者 = **Inventory**；**Order** 负责在准入事务后产生**持久 release 请求**（outbox/lease task），不得在 `AdmitRefund` 事务内直接远程释放；**AfterSales** 只负责退货受理/回库授权，Inventory 执行 restock；**Fulfillment 不负责库存释放**。零元与正金额**同一 owner/同一执行路径**。 |
| **T1.4a/R-B** | **POST 组 commitment 时机（契约澄清）**：Tx1 只写 application；POST 组的 `RESERVED` commitment 在**准入阶段**与 POST admission 同一事务写入；打款前必须已 RESERVED；不得在 Tx1 写、不得在打款后补。 |
| **T1.4a/R-C** | **零元 CONSUMED 先 `CancelIfCreated`（有条件接受）**：零元无资金、取消幂等，允许“先取消、后单事务写 admission+S19 DONE”；**前提**是调用前已有**持久 AfterSales refund task**（lease/retry）；无持久任务时必须先持久意图再远程取消；不得留下无 worker 的 PENDING；不得在取消失败时写 S19。 |

### 0.6 T1.6 实现契约基线（`T1.6/CB-1…CB-5`）〔来源：T1.6 §4.2〕

| ID | 裁决 | 契约 |
|---|---|---|
| **T1.6/CB-1** | `CANCEL_CAS_LOST` 不返回业务错误，继续组扇出 | CAS 失败是正常竞态：不写 payment-level command；稳定 reason `CANCEL_CAS_LOST` + log + metric `payment_cancel_cas_lost_total`。 |
| **T1.6/CB-2** | amount 用 `captured-refunded` 净额 | `paymentCancelAmount = captured - refunded`；净额 0 不写命令；不得退超过 Payment cap；不得用订单 `pay_cent` 猜测。 |
| **T1.6/CB-3** | Inventory `TccCancelConfirm` 失败仍写退款命令 | Payment 退款不被回库失败阻塞；Inventory 补偿必须持久化（`queueInventoryCompensation`/等价）并告警；命令写入失败则回滚并返回错误，不得静默。 |
| **T1.6/CB-4** | 错误码仅 reason/log/metric，不扩 apperr/proto | 异步命令不新增 proto/domain code；必须使用稳定 `details.reason`（`PAYMENT_CANCEL_NO_CAPTURE`/`PAYMENT_CANCEL_REFUND_RETRY`/`PAYMENT_NOT_REFUNDABLE` 等）+ log + metric；Reviewer 不得接受裸字符串无观测。 |
| **T1.6/CB-5** | `refundCancelledAfterConfirm` Payment 不可用 fail-closed | Payment 权威快照不可用时不猜 amount、不写命令；返回 `UNAVAILABLE`/告警，由 caller/重试恢复；不得用订单金额降级。 |

### 0.7 C-04 RefundBatch 契约裁决（`C-04/R-1…R-6`）〔来源：C-04 §4 + C-04 rulings §1；accepted 2026-11-24〕

| ID | 裁决 |
|---|---|
| **C-04/R-1** | **首次持久化 Reply 固定**：`status` 描述首次执行；同 batch 重放原样返回首次 Reply，不重算/不降级。首次至少一个 NEW item → `REFUNDED`；首次全部 item 已存在 → `ALREADY_REFUNDED`；后续重放不因“现在全部已存在”改判。 |
| **C-04/R-2** | **mixed Σ**：`expected_total_cent == Σ(items)`；只校验 `Σ NEW items <= remaining`（`remaining = captured - refunded`）；既有 item 已在 `intent.refunded_cent` 中，不重复扣；全 NEW 时等价于 `Σ items <= remaining`。 |
| **C-04/R-3** | **schema 定稿**：`payment_refund_batches`（PK `(payment_no,batch_id)`、`items_fingerprint`、`status`、`refunded_cent`、`reason`、`item_results JSON`）+ `refunds.batch_id` + `uk_refunds_batch_item(payment_no,batch_id,idempotency_key)`；保留 `uk_refunds_idempotency(payment_no,idempotency_key)`；`batch_id=''` 用于既有 plain Refund。 |
| **C-04/R-4** | **错误码与 Order 映射**：apperr 标准 code + `details.reason`；新增 `PAYMENT_NOT_REFUNDABLE`/`BATCH_STATE_INVALID`；`BATCH_UNAVAILABLE` → 保持 `PAYING` 退避重试；其余确定性错误 → `MANUAL_HOLD`、零新 Payment、告警；不得把确定性错误重试成部分成功。 |
| **C-04/R-5** | **`refunds.batch_id` 审计**：`batch_id` 只标记本批**新写入**；mixed 的既有 item 结果记 `ALREADY_REFUNDED`（含原 `refund_no`），不重写 `refunds` 行、不重发 `payment.refunded`。 |
| **C-04/R-6** | C-04 已按 R-1…R-5 与 schema 定稿；**Director accepted 2026-11-24**；Rev 2.1 并入（§5.9/§12.5.8）。`fc295e5` 仍是 WIP，不构成实现 PASS。 |

---

## 1. 事实模型与事件契约

### 1.1 事实所有权

| 事实 | 所有者 | 持久化形态 | 消费者 | 禁止 |
|---|---|---|---|---|
| PaymentCaptured | Payment | `payment_intents(SUCCEEDED, captured_cent)` + `payment_callbacks` 去重 | Order、AfterSales、对账 | 不表示库存就绪/可发货 |
| OrderSettled | Order | `orders.status=PAID` + `settlement_payment_no` + `allocation_frozen_at/hash` | Fulfillment、BFF、AfterSales | 不由 Payment 事件顺序推导 |
| InventoryReady | Inventory | 行预留 `CONFIRMED`；以 `CheckOrderReady` 同步返回 | Order | 不由 `PAID`/Catalog 投影推导 |
| FulfillmentAllowed | Order | 组级谓词：`PAID ∧ 行 CONFIRMED ∧ 无活动缺货 ∧ permit=NONE` | Fulfillment | 不是新状态字段 |
| RefundAdmission | Order | `order_refund_admissions(refund_id 唯一)` + 许可行 CAS + `order_refund_commitments` | AfterSales/Payment/Fulfillment | 不是打款事实 |
| Shipment/Receipt | Fulfillment | `shipments` + `shipment_receipts` + `shipped_at` + `exception_state` | Order、BFF | Order 不得制造签收事实；Order DB 无该事实 |
| **RefundCommitment（S15）** | Order | `order_refund_commitments` + `order_refund_commitment_lines`（商品/运费组成） | 准入额度、对账、退款申请 | 不是打款事实；`RELEASED` 不等于退款完成；不得由 worker 自动写；`SETTLED` 不得回退 〔A-01/A-03/Amd1 §4.3〕 |
| **LegacyPaymentResolution（S16）** | Order 记录事实；运营/Director 裁决 | `legacy_payment_resolution`（`order_no` 唯一） | 发货准入、退款准入、对账、运营 | 不替代 Payment `captured/refunded`；不得猜 `payment_no`；不得长期 `OPEN` 〔A-04/Amd1 §5〕 |
| **RefundComposition/Responsibility（S17）** | AfterSales（售后责任与行已退）+ 缺货事实 | 行剩余、商品/运费组成、责任枚举与证据 | Order 准入、Payment 执行、对账 | Order 不得改责任、不得从自由文本推断责任；不得把商品与运费混算 〔A-03/Amd1 §4.4〕 |
| **SettlementShadow（S18）** | Order 产出；Fulfillment 影子消费 | `settlement_shadow_diffs` | 告警、对账、切换前比对 | 不建运单、不推进组、不放行发货、不阻断结算 〔A-09/Amd1 §3〕 |
| **ZeroRefundExecution（S19）** | Order | `refund_applications(source=system_zero, amount_cent=0, state=DONE)` | AfterSales、BFF、对账、运营 | 不是 Payment 退款事实；不产生 Payment `refunded`；不得用正金额；未完成不得标 `DONE` 〔AZ-01/AZ-05〕 |

- S15 的 `RESERVED→RELEASED` 仅 AC-62 的合法触发（§5.6）；S19 的 `DONE` 只表示零元退款执行闭环，**不表示 Payment 已退款**。
- S19 **没有 `PENDING` 中间态**：由 `AdmitRefund` 零元短路在同一事务直接建为 `DONE`；短路失败/回库未完成时不创建 S19，不得留下无 worker 的悬挂行。

### 1.2 事件（规范 subject）

| Subject | 状态 | 生产者 → 消费者 | 最小 payload | 备注 |
|---|---|---|---|---|
| `shop.payment.succeeded.v1` | 既有 | Payment → Order/BI | `payment_no, order_no, amount_cent, captured_at, channel` | 即 PaymentCaptured；`payment_no` 唯一 |
| `shop.payment.refunded.v1` | 既有，扩展字段 | Payment → Order/AfterSales/BI | 见 §4.2（**必须携带 `refund_id`**） | `refund_id` 幂等；Order 只消费订单级字段；**C-04 批退款只为新写入 item 发一次**，`ALREADY_REFUNDED` 的既有 item 不重发（§5.9） |
| `shop.order.paid.v1` | 既有，兼容双发 | Order → BFF/旧消费者 | 既有字段 | 不再作为履约准入依据；移除日期见 §6.4 |
| `shop.order.allocation_frozen.v1` | 既有 | Order → 审计/BI | 既有字段 | 与结算同事务 |
| `shop.order.settled.v1` | **新增** | Order → Fulfillment 影子/AfterSales/BI | 见 §1.3（必填字段不变，仅追加可选 `schema_version`） | `order_no+payment_no` 唯一；`schema_version>1` 影子拒绝 |
| `shop.order.refund_admitted.v1` | **新增** | Order → AfterSales/Payment/Fulfillment | `refund_id, order_no, group_no, result, amount_cent, version` | `refund_id` 唯一；字段不变 |
| `shop.order.refund_requested.v1` | 既有，**仅观察（退役中）** | Order（仅 `refundCancelledAfterConfirm`）→ Payment | 见 §3.7 / §5.4 | 保留兼容；**A-08 收窄**：除 `pay:<payment_no>:cancel` 命令载体外，消费只观察/告警，不得执行 `Refund`（C-2 已关闭，§18.2）；本版不移除 subject |
| `shop.inventory.confirmed.v1` | 既有 | Inventory → Order | 既有字段 | 仅投影加速；**不得**作为 PAID 依据 |
| **inventory 释放请求（组级）** | 既有事件（C-04 增量字段） | Order → Inventory | `refund_id, order_no, group_no, items[], warehouse_id`（+ `source`, `reason`） | 既有 `inventory.compensation_requested` 只有 `order_no/items`，不足以按组释放；C-04 必须补组字段，C-05 无迁移 〔T1.4a/R-A〕 |
| `shop.order.cancelled.v1` | 既有 | Order → Inventory/Pricing | 既有字段（支付级取消分支不新增组字段） | 单组释放不得复用本事件（§3.4.2）〔T1.6 §2.2〕 |

- registry 必须新增 `order.settled`/`order.refund_admitted` 的必填字段；未知事件 fail-closed。
- P0 只支持单仓 G1；`warehouse_id/group_no` 的多仓身份升级属 P1（另立 ADR-0005 Amendment），**不是本 ADR 的前置**。
- `order.paid` 与 `order.settled` 在同一结算事务内双发；旧消费者清单见 §12.4。
- **禁止**：改已发布 subject 的必填字段类型/语义或删除；破坏性变更必须新增 `.v2` + 新消费者/registry。〔Amd1 §3.1〕

### 1.3 `order.settled` v1 字段、版本与影子消费规则 〔来源：Amd1 §3.1〕

Subject 不变（`shop.order.settled.v1`）；R1 不新增 subject、不改 v1 必填字段，只在同一 v1 内增加可选字段。

| 字段 | 必填 | 类型/语义 |
|---|---|---|
| `order_no` | 是 | 订单号 |
| `payment_no` | 是 | 生效支付；零元为 `zero:<order_no>` |
| `settled_at` | 是 | Unix 秒；仅展示/延迟指标，**不参与严格等值比对** |
| `allocation_hash` | 是 | 冻结快照 hash（64 hex） |
| `package_count` | 是 | ≥1 |
| `warehouse_count` | 是 | ≥1（P0 单仓 G1 兼容值 1） |
| `groups` | 是 | `[]string` group_no；去重后非空，按字典序 |
| `schema_version` | 否 | 整数；缺省=1；当前只接受 1；>1 拒绝影子 |
| `order_status` | 否 | 可选快照；影子以 Order 权威读为准，不以此字段做准入 |
| `paid_cent` / `pay_cent` | 否 | 可选；仅用于差异定位，不替代 Payment 权威 |

**版本规则**：

1. 同一 subject v1 只允许**追加可选字段**；已发布必填字段不得改类型/语义/删除。
2. 破坏性变更必须新增 `shop.order.settled.v2` + 新消费者/新 registry，不得原地改 v1。
3. 影子消费者：未知 `schema_version`、缺必填、`groups` 为空/重复、`payment_no` 为空 → 拒绝该影子事实并告警；不得用默认值猜测，不得影响结算。
4. `event_id` 由 outbox 提供；重复 `(order_no, payment_no)` 事件只更新 `last_seen_at`，不重复记差异。
5. 事件注册表必填字段保持 `order_no, payment_no, settled_at, allocation_hash, package_count, warehouse_count, groups`；`schema_version` 为可选，未知版本由消费者拒绝。

**比对口径（仅告警）**：Order 读模型（`Order.GetOrder(Admin)`）是结算事实权威；`order.paid` 是兼容旁证；Fulfillment 本地 shipment 只作观察、影子不建单。差异类型：`MISMATCH_ORDER_STATUS/MISMATCH_PAYMENT_NO/MISMATCH_ALLOCATION_HASH/MISMATCH_COUNTS/MISMATCH_GROUP_SET/MISSING_PAID（窗口默认 5 分钟）/UNKNOWN_VERSION/MISSING_FIELD/ORDER_LOOKUP_FAILED`。不比较 `settled_at` 严格等值；差异只写 `settlement_shadow_diffs`、告警、对账，**不得阻断结算、不得放行/阻断发货、不得自动修复**；差异率阈值由 Director 在影子批准时写入批准行，不在代码硬编码。〔Amd1 §3.3〕

### 1.4 事件退役与消费者状态

- `order.refund_requested`：保留 subject 与 registry 字段以兼容已投递消息；新 TCC 不以该事件作为退款触发器（TCC 只走 §5.5.1 batch/准入）；**除 `pay:<payment_no>:cancel` 命令载体外，该 subject 的消费按 A-08 只观察/告警，不得执行 `Refund`**；支付级取消由 T1.6 命令载体执行（§3.4.2/§3.7），二者不冲突（C-2 已于 2026-11-25 关闭，§18.2）；移除日期在 BFF/BI/旧消费者迁移后由 Director 在 release 记录中设定，本版不移除。
- `order.refund_admitted`：现有字段不变；Order 的 admission 结果按 `refund_id` 幂等。
- `order.paid`：Fulfillment 不得据此允许发货；`settled` 模式切换与移除日期见 §6.4。

---

## 2. 支付意图与捕获（Payment）

### 2.1 `CreateIntent`

1. 校验 `order_no`、`amount_cent>0`。
2. 调 `Order.GetOrder(Admin)`（deadline 500ms）；校验订单存在、`user_id` 匹配、`status=PENDING_PAYMENT`、`amount_cent == pay_cent`。
3. 失败返回 `PAYMENT_AMOUNT_MISMATCH`/`ORDER_STATE_CONFLICT`，不创建 intent。
4. 按 `(order_no, amount_cent, channel)` 幂等创建 intent；`user_id` 由 Order 反查。

**零元单不走此入口**：`pay_cent=0` 由 BFF 调 `Order.SettleZero(order_no)`（§3.5），Payment 不创建 intent/任务。
Order 增加零元结算恢复：`zero_settlement` 任务（`order_no` 唯一）或由 Order worker 扫描 `PENDING_PAYMENT AND pay_cent=0`；BFF 调用幂等，崩溃后由 worker 继续。

### 2.2 Callback 本地事务

同一本地事务：

1. `payment_callbacks` 去重（`UNIQUE(channel,event_id)`，复用既有表）。
2. `intent.status=SUCCEEDED` + captured。
3. 写 `payment_settlement_tasks(payment_no PK, state=PENDING, attempts=0, dtm_gid='payment-confirm:<payment_no>', ...)`。
4. `outbox.Append(shop.payment.succeeded.v1)`。
5. 提交后才 ack；本地持久化失败返回可重试错误，不得 ack。

### 2.3 金额/身份裁决（在 Order CAS 之后）

| 输入 | 裁决 |
|---|---|
| 同一 `payment_no` 重放 | 幂等成功，不重复捕获/副作用 |
| 同一 `payment_no` 金额/订单不符 | 拒绝并告警；真实入账则**支付级退款**（`refund_id=pay:<payment_no>:mismatch`） |
| 第二个 `payment_no` 成功捕获 | 由 Order CAS 裁决生效支付；非生效捕获写 `payment_duplicate_captures` + **支付级退款**（`refund_id=pay:<payment_no>:dup`），返回 `DUPLICATE_PAYMENT` |
| 渠道捕获 ≠ pay_cent | 不生效；**支付级退款**；`PAYMENT_AMOUNT_MISMATCH` |
| `pay_cent=0` | 不经过 Payment；Order `SettleZero` |

**禁止在回调时预判“第二个=重复”**：必须等 Order 的 `PAID` CAS 结果；非生效方由 worker 在拿到 `DUPLICATE_PAYMENT` 后写记录/退款。

### 2.4 结算任务 worker

```text
PENDING -> PROCESSING -> DONE
              │
              ├-> PENDING（指数退避）
              └-> FAILED_PERMANENT（告警 + 人工重放）
```

- 认领：`FOR UPDATE SKIP LOCKED` + `lease_owner/lease_until`；同一 `payment_no` 只允许一个 owner。
- **TCC 启用**：`gid=payment-confirm:<payment_no>`（复用既有 gid）；启动前查询 DTM 是否已有该 gid/在途，若有则接管/查询，不得创建第二个；成功后记 `dtm_status=DONE`。
- **TCC 未启用**：调用直接 `MarkPaid`；同一 `payment_no` 幂等。
- `ORDER_CANCELLED`/`DUPLICATE_PAYMENT`/`PAYMENT_AMOUNT_MISMATCH`：任务转 DONE + 创建**支付级退款**申请（§5.4），不重试结算。
- 重启恢复：任务即入口；人工按 `payment_no` 重放；`cmd/payment/main.go` 必须注册 worker。

### 2.5 退款事实查询（`GetRefundByKey`）〔来源：Amd1 §4.7、A-07、A-12〕

- 新增 `Payment.GetRefundByKey(payment_no, refund_id)`（或给 `Refund` 增只读 `refund_id`/`idempotency_key` 字段）；**不得用 `refund_no` 猜测 `refund_id`**。
- 用途：`COMPENSATING` 打款超时/未知、`payment.refunded` 丢失/迟到时的惰性对账与归因；`payment.refunded` 必须携带 `refund_id`。
- 查询超时/不可用 → **fail-closed**：阻断该笔准入/继续重试，**不得把未知当未退或已退**。
- 该 API 是 C-04 的必填增量；未落地前，惰性对账只能降级为“按订单列表 + 严格匹配/人工”，不得猜测。〔Amd1 §11.2〕

### 2.6 单笔退款上限与批级 Σ 的边界

- **单笔 `Refund`**（支付级单笔，§5.4）只保证该笔不超 `captured - refunded`；Order 的预检与“每笔打款前重读 Payment 权威”（§5.5.1）仍是必要防线。
- **TCC 组级批打款 MUST 使用 C-04 `RefundBatch`**（§5.9，accepted 2026-11-24）：Payment 在 `payment_intents` 行锁内校验 `Σ items == expected_total_cent` 且 `Σ NEW items <= remaining`，批内 all-or-none。这关闭“预检后外部退款/取消退款插入导致先付一组、后组失败”的竞态窗口（C-04/R-2；C-1 已关闭）。
- Order 仍负责 `B = Σ 组责任上限`、`done`、`inFlight`、`outstanding = B - done - inFlight` 与 `R = captured - refunded` 的勾稽；Payment **不计算/不接收** Order 的 `B`，只校验调用方声明的 `expected_total_cent` 与 items。
- `BATCH_UNAVAILABLE` → 保持 `PAYING` 退避重试；确定性错误 → `MANUAL_HOLD`、零新 Payment（§5.9.3）。不得把“Payment 会拒绝某一组”当作 Σ 兜底，也不得把批内原子性扩展为跨批保证。

---

## 3. 订单结算（Order）

### 3.1 `settleOrderTx`（唯一 PAID 写路径）

**事务外**：调用 `Inventory.CheckOrderReady`（§3.2）并拿到逐行 `CONFIRMED` + `reservation_version`；不可用/超时 → `TRANSIENT_ERROR`，不进入事务。

**事务内**（短事务，禁止远程调用）：

1. `SELECT ... FOR UPDATE` 订单。
2. `status != PENDING_PAYMENT` → 进入 §3.3 裁决。
3. `paid_cent == pay_cent`（零元 `0`）；否则 `PAYMENT_AMOUNT_MISMATCH`。
4. `ensureAllocationSnapshotTx`（缺失则补建）。
5. `freezeAllocationTx`（冻结 `allocation_frozen_at/hash`）。
6. `UPDATE orders SET status='PAID', settlement_payment_no=?, paid_cent=?, inventory_ready_version=? WHERE order_no=? AND status='PENDING_PAYMENT'`。
7. 状态日志 + outbox：`shop.order.paid.v1`（兼容）+ `shop.order.settled.v1` + `shop.order.allocation_frozen.v1`。
8. 写 `pricing_confirmation_tasks`（§3.6，持久任务）。
9. 提交。

**禁止**：TCC 分支另写 PAID/冻结；`afterPaid` 在事务内做远程调用。

### 3.2 `Inventory.CheckOrderReady`（新只读 RPC）

```text
rpc CheckOrderReady(CheckOrderReadyRequest) returns (CheckOrderReadyReply)

CheckOrderReadyRequest {
  string order_no = 1;
  repeated LineRef lines = 2;   // line_no/sku_id/warehouse_id/quantity
}
CheckOrderReadyReply {
  bool ready = 1;
  repeated LineStatus lines = 2;
  int64 reservation_version = 3;
}
LineStatus { int32 line_no; int64 sku_id; int64 warehouse_id; int32 quantity;
             string status; int64 version; }
```

- 语义：只读、幂等；`ready=true` 当且仅当所有行 `CONFIRMED`；`HELD/RELEASED/缺失` → `ready=false`。
- 错误：`INVENTORY_NOT_READY`（明确未就绪）、`TRANSIENT_ERROR`（超时/不可用）；deadline 500ms。
- **不可用即阻断**：结算不得回退到 Order 本地就绪投影；本地投影只用于展示。
- owner：Inventory（实现）+ platform（proto/gen）；integration owner 组织契约评审。

### 3.3 CAS=0 / 重读裁决

| 重读状态 | 结论 |
|---|---|
| `PENDING_PAYMENT`（CAS 未生效，事务可重试） | 重试 `settleOrderTx`（有限次，指数退避）；超限 `TRANSIENT_ERROR` |
| `PAID` 且 `settlement_payment_no=本 payment_no` | 幂等成功；不重复副作用 |
| `PAID` 且 `settlement_payment_no≠本 payment_no` | 本笔为重复捕获；支付级退款；`DUPLICATE_PAYMENT` |
| `CANCELLED` | 不结算；支付级退款；`ORDER_CANCELLED` |
| 其他 | `SETTLEMENT_CONFLICT`；不触发副作用；告警 |

### 3.4 取消

#### 3.4.1 通用规则 〔来源：Rev 1.0 §3.4〕

- 仅 `PENDING_PAYMENT` 可取消；取消与结算在 `orders.status` 上 CAS，唯一胜者。
- 取消胜出：已捕获资金走**支付级退款**（§5.4）；`HELD` 释放，`CONFIRMED` 走 `CancelConfirm` 补偿释放；失败重试告警。
- 已 `PAID` 不提供取消；TCC 补偿/结算失败走组级退款。

#### 3.4.2 `PENDING_PAYMENT + 已捕获`：支付级取消 〔来源：Amd1 §4.8 + T1.6 §2/§3 + T1.2/R-3〕

> **替代关系（明确）**：本节取代 `T1.2/R-3` 的 fail-closed guard（T1.6 §4）；T1.2/T1.3 不得再保留 `PAID+REFUNDING` 旧行为，也不得在 T1.6 分支就绪前置空写入。

1. **入口**：TCC 补偿裁决点（`TccCancelMarkPaid`/等价），即 `T1.2/R-3` 的 guard 位置。先读 Payment 权威快照（`captured/refunded`）；仅 `status=PENDING_PAYMENT && captured>0` 适用本分支；Payment 不可用/超时 → fail-closed，不进入本分支、不猜 captured〔CB-5〕。
2. **事务内**（禁止远程调用）：
   1. `SELECT orders FOR UPDATE`；CAS `PENDING_PAYMENT→CANCELLED`（带 version/status）；
   2. 同 tx `outbox` append `order.cancelled`（库存/优惠补偿，既有协议）；
   3. 同 tx append **支付级取消命令**（`refund_id=pay:<payment_no>:cancel`、`source=payment`、`non_effective_capture=true`、`amount_cent=captured-refunded 净额`；净额 0 不写命令）；
   4. commit。
3. **不写** `refund_batches`/`refund_applications`/`order_refund_admissions`/commitment，**不改 permit**，**不进入** `AdmitRefund`/组扇出。
4. **CAS 0 行（已 PAID）** → rollback；稳定 reason `CANCEL_CAS_LOST` + log + metric `payment_cancel_cas_lost_total`；进入 §5.5.1 组扇出；**不返回业务错误**、不再发支付级取消〔CB-1〕。
5. **outbox append 失败** → 整个 tx rollback（CAS 不落），返回错误并重试；不得静默。
6. **无 captured/零元**：`captured=0` 或 `pay_cent=0` → 不发 payment command；按 AC-46 本地取消/零元路径处理。
7. **支付侧执行**：Payment 使用既有 `Refund`（`idempotency_key=refund_id`、`non_effective_capture=true`），**不使用** batch 契约（单笔支付级退款，无组 items/Σ）；重复事件 Payment 幂等，不重复退款。
8. **Inventory**：由 `order.cancelled` consumer 异步补偿——`HELD` → `Release`；`CONFIRMED` → `TccCancelConfirm` 回库补偿；失败持久化补偿 + 重试/告警，不静默〔CB-3〕；Order 不直接调 Inventory，本分支不使用 §5.7 的组级 release 事件。
9. **边界**：本分支仅适用于 `PENDING_PAYMENT` 且已捕获；历史 `PAID` 的 `EXPIRED_REFUND` 是另一分支（§4.4.3：不写 `CANCELLED`、保留订单生命周期、先撤许可/取消运单/释放后支付），**不得混用**。支付级取消与组级退款可能并发，`refund_id` 不同；Payment captured 上限与订单 `refunded` 投影保证不超退；Order 不得把支付级退款计入组额度。

### 3.5 零元结算 `SettleZero`

- 入口：BFF/Order 内部调用 `Order.SettleZero(order_no)`；无 Payment 参与。
- 校验 `pay_cent=0`、`status=PENDING_PAYMENT`；调用 `settleOrderTx`，`payment_no=zero:<order_no>`，`paid_cent=0`。
- 零元单的取消/缺货/售后：`refund_applications` 以 `source=system_zero`、金额 0；仍走 `AdmitRefund` 零元短路撤销许可/取消运单/按 INV-44 释放，终态 `DONE(zero_amount)`；不调 Payment；只释放/回库（§5.2.5）。

### 3.6 `pricing_confirmation_tasks`

- 与结算同事务写入（`order_no` 唯一，`state=PENDING`，payload：quote/coupon/items）。
- worker 调 Pricing 核销（`ConsumeCoupon`/`ConfirmPromotions`），幂等；失败重试，`FAILED_PERMANENT` 告警 + 人工入口。
- 业务终态：核销成功/人工关闭；不得因为核销失败把订单退回未支付或重复退款。

### 3.7 `refundCancelledAfterConfirm` 的持久命令 〔来源：T1.6 §3.2〕

- 现状只发 legacy event + 日志；Rev 2.0 起必须：
  1. Inventory `CONFIRMED` 预留先 `TccCancelConfirm`（失败 `queueInventoryCompensation`/等价持久补偿 + 告警）；
  2. 同事务/可重试地写入支付级取消命令（§3.4.2 第 2.3 步的同一命令形状）；
  3. append 失败 → **返回错误/告警**，不得只 log 后继续；
  4. 无 captured/`payment_no` → 不写命令，保持告警（`PAYMENT_CANCEL_NO_CAPTURE`）。
- 同一 order/payment 重复调用：`refund_id=pay:<payment_no>:cancel` 固定，Payment.Refund 幂等；不重复创建组级 S19。
- **不使用 `RefundBatch`**：本命令是单笔支付级退款（无组 items/Σ），由 `Refund` 幂等执行；C-04 `RefundBatch` 仅用于 TCC 组级批退款（§5.9）；C-05 无新增迁移。

---

## 4. 资金口径（Payment/Order）

### 4.1 生效支付与订单级金额

- `settlement_payment_no` = 唯一生效支付；`PAID` 后必填。**历史 `PAID` 单可为空**，按 §4.4 解析。列语义统一为 `NOT NULL DEFAULT ''`（用 `''` 表示空，避免 NULL 比较陷阱）〔Amd1 §11.3〕。
- 订单级 `captured` = 生效支付的 captured（零元 0）；**以 Payment 事实为准**。
- 订单级 `refunded` = 针对生效支付的退款累计；不含非生效捕获/支付级退款。
- `payment_duplicate_captures` 单独记账；不进入订单 captured/refunded。
- `FULL/REFUNDED`：`order_captured_cent >= pay_cent` 且 `order_refunded_cent >= order_captured_cent`；由 Order 投影计算，但准入用 Payment 权威值（§4.3）。

### 4.2 `shop.payment.refunded.v1` 字段

```text
refund_id, payment_no, order_no, amount_cent,
intent_captured_cent, intent_refunded_cent,
order_captured_cent, order_refunded_cent,
non_effective_capture(bool), source, reason, status
```

- Order 只消费 `order_captured_cent/order_refunded_cent` 且 `non_effective_capture=false`；用于展示/对账，不用于准入。
- `source=payment|group`；组级退款由 `refund_id` 关联 `refund_applications`。
- `refund_id` 必填且唯一；回放按 `refund_id` 幂等，重复事件不得二次投影；新增字段灰度期 nullable/optional，旧消费者忽略。
- **C-04 批退款**：`payment.refunded` 只为本批**新插入**的 item 发一次；mixed 中的既有 item 记 `ALREADY_REFUNDED`（含原 `refund_no`），不重写 `refunds` 行、不重发事件（§5.9；C-04/R-5）。

### 4.3 退款额度（准入）

三层：

1. **Payment 权威**：按 `orders.settlement_payment_no` 调 `Payment.GetIntent(payment_no)`（deadline 500ms）取该生效 intent 的 `captured_cent/refunded_cent`；订单级 captured/refunded = 该 intent 的值（非生效捕获不计入）。历史单先按 §4.4 解析生效支付。
2. **Order 在途账本**：`order_refund_commitments(order_no, group_no, refund_id, amount_cent, goods_cent, freight_cent, composition_known, state)`；`state ∈ RESERVED | SETTLED | RELEASED`。准入事务锁订单行，校验 `SUM(state=RESERVED) + 本次 <= Payment captured - Payment refunded`。`SETTLED` 已进入 Payment refunded，必须排除；`RELEASED` 也排除。
3. **组/行层**：`组剩余商品额度 = Σ行剩余`；`组剩余运费额度 = 普通售后按 0001 §6.5 本次责任是否可退、TCC 系统补偿按全部未退 freight_share`；`组剩余可退上限 = 商品 + 运费`；`整组退款金额 = 商品剩余 + 本次可退运费`。行层沿用 SPEC-0001 INV-24，且行剩余 = 行实付 − AfterSales 已退（S14）− 该行在途承诺（S15 行组成）〔Amd1 §4.4〕。

任一层不满足 → `REFUND_ADMISSION_CONFLICT`；不得打款。Payment 的 INV-05 仍是最终防线；Order 投影只展示。

**承诺状态迁移**：`RESERVED → SETTLED`（Payment 退款成功，由 Order 的 `payment.refunded` 消费者单写）/ `RESERVED → RELEASED`（仅 AC-62 的合法触发，§5.6）。完成与拒绝必须更新 commitment，否则下一笔会被双重扣减。

**惰性对账与唯一对账写入方**〔A-07〕：

- Order 的 `shop.payment.refunded.v1` 消费者是 `RESERVED→SETTLED` 的**唯一写入方**；其他路径只读。
- 若 worker 在 Payment 退款成功后、更新 commitment 前崩溃，准入在返回 `REFUND_ADMISSION_CONFLICT` 前必须对该订单的 `RESERVED` 承诺按 `refund_id` 查 Payment 退款事实做**惰性对账**：已退则置 `SETTLED` 后重算；仍未退才拒绝。日终/后台 worker 扫描陈旧 `RESERVED` 做同样收敛并告警计数。
- Payment 查询依赖 `GetRefundByKey`（§2.5）；超时/不可用 → fail-closed（阻断该笔准入/继续重试），不得猜“已退/未退”。

### 4.4 历史 `PAID` 单的生效支付（D-28）〔来源：Rev 1.0 §4.4 + Amd1 §5〕

#### 4.4.1 候选与写回

- 队列 owner：**Order 唯一写**；Payment 只读；运营/Director 通过 Order 管理命令裁决。触发：`PAID` 且 `settlement_payment_no=''` 时，由 Order 的准入/发货/解析入口调用 `EnsureLegacyResolution`；读接口只观察，不建行。
- 候选：`ListOrderIntents` **事务外**读取；`status=SUCCEEDED` + `captured == orders.pay_cent` + order 匹配；`orders.paid_cent` 若为 0 视为未回填、仅审计，若非 0 且 ≠ `pay_cent` 则转队列/告警（不自动采用）；**仅当 `pay_cent=0` 且无任何已捕获 intent 时**解析为 `zero:<order_no>`；**有捕获 intent 时不得 zero**：无离仓组按支付级退款，有离仓组/不确定进入人工队列或 `EXPIRED_FINANCE`，不得吞掉已捕获资金。
- 并发：先读 Payment（事务外）→ Tx `SELECT orders FOR UPDATE` → 重读 `settlement_payment_no`；唯一候选 CAS `UPDATE ... WHERE settlement_payment_no=''`；0/多候选 `INSERT ... ON DUPLICATE KEY UPDATE order_no=order_no` 后 `SELECT ... FOR UPDATE`；终态不重开。
- 无法解析的历史单**退款准入阻断并告警**，不得按 0 额度处理。**闸门口径（SPEC-0002 v1.8 AC-63 + Amd1 §5.3）**：`OPEN/RESOLVING/COMPENSATING` → `LEGACY_PAYMENT_UNRESOLVED`，未离仓组禁发、全部组退款阻断；`EXPIRED_FINANCE/REJECTED` → 禁发禁退；`EXPIRED_REFUND` → 禁发、不再新建组退款（仅允许已创建的支付级退款）；`RESOLVED/EXPIRED_ZERO` → 按正常路径，以许可/库存为准。Rev 1.0 §4.4 “无法解析仍可发货”的旧表述已被 AC-63 收窄，不在本版保留。
- 回填脚本 dry-run、计数对账、可回滚；不强制在业务高峰执行。

#### 4.4.2 队列状态机（`legacy_payment_resolution`）

```text
OPEN --运营开始--> RESOLVING --人工解析--> RESOLVED（终态）
OPEN/RESOLVING --24h 无捕获零元--> EXPIRED_ZERO（终态）
OPEN/RESOLVING --24h 无捕获有金额 / 有离仓 / 不确定--> EXPIRED_FINANCE（终态）
OPEN/RESOLVING --24h 有捕获且无离仓--> COMPENSATING
COMPENSATING --补偿完成且 Payment 全部成功--> EXPIRED_REFUND（终态）
COMPENSATING --Payment 失败 / 超时 / 未知--> COMPENSATING（租约内重试 + 告警）
COMPENSATING --Ship 胜出 / 未知 / 人工判定不可恢复--> EXPIRED_FINANCE（终态）
COMPENSATING --补偿失败 / 失租--> COMPENSATING（租约内重试 + 告警）
OPEN/RESOLVING --脏数据人工--> REJECTED（终态）

禁止：COMPENSATING -> OPEN/RESOLVING；COMPENSATING -> 提前 EXPIRED_REFUND；
      EXPIRED_* -> OPEN；REJECTED -> OPEN；RESOLVED -> EXPIRED_*；删除记录
```

- 唯一键：`order_no` 主键（一单一行）；`resolution_id` 终态生成 `lpr:<order_no>`，唯一。状态迁移一律 `UPDATE ... WHERE order_no=? AND version=? AND state IN (...)`；0 行即败者，读回当前状态。
- `COMPENSATING` 是 24h 到期后的**非终端中间态**：它已离开 `OPEN/RESOLVING`，因此不违反 AC-63“不得继续 OPEN”；它不是终态，**不得提前退款**，也不得回 OPEN。
- `resolution_kind`：`COMPENSATING` 期间为空；`RESOLVED` 为 `MANUAL`；`EXPIRED_ZERO/EXPIRED_REFUND/EXPIRED_FINANCE/REJECTED` 为同名值。`blocked_scope`：`OPEN/RESOLVING/COMPENSATING/EXPIRED_REFUND` 为 `UNSHIPPED_GROUPS_AND_REFUND`；`EXPIRED_FINANCE` 为 `FINANCE_HOLD`；`REJECTED` 为 `REJECTED_HOLD`；`RESOLVED/EXPIRED_ZERO` 为 `NONE`。
- 财务关闭：`EXPIRED_FINANCE` 仅财务/授权角色 7 天内写 `closed_at`；状态保持终态、不重开；逾期升级授权角色，仍保持终态（D-23）。

#### 4.4.3 `COMPENSATING` 与 `EXPIRED_REFUND` 的强制顺序

**仅在“有捕获且无任何已离仓组”时进入**；24h 到期**不得继续停留在 `OPEN/RESOLVING`**：

1. 24h 到期：队列锁内 CAS `OPEN/RESOLVING→COMPENSATING`（version/lease），从此刻起已离开 `OPEN/RESOLVING`；记录 `expires_at`，`resolution_kind` 暂空，`blocked_scope` 保持禁发禁退。
2. 在 `COMPENSATING` 持租约读取订单/组/运单；若任一已离仓或不确定 → CAS `COMPENSATING→EXPIRED_FINANCE`（保留审计，不取消运单、不退款）。
3. 对每个未离仓组：permit `NONE` → CAS `NONE→REVOKED`（已 `REVOKED` 幂等跳过）；permit `CONSUMED` 且运单 `CREATED` → 事务外 `CancelIfCreated`（幂等）；运单 `CANCELLED`/无运单 → 继续；返回 `ALREADY_SHIPPED/RECEIVED` → 该组已离仓 → **CAS `COMPENSATING→EXPIRED_FINANCE`**，停止，不取消运单、不退款。
4. 按 INV-44 释放未确认预留：`HELD` 直接释放，`CONFIRMED` 走 `CancelConfirm`；释放失败写持久任务 + 告警，**不阻塞退款完成**，但该组不得再发货（D-18）。
5. **先持久化退款意图（Payment 之前）**：对每笔未退完的 captured intent 幂等写/复用 `refund_applications(source=legacy_expired, group_no NULL, refund_id=pay:<payment_no>:legacy_expired, amount=captured−refunded, goods_cent=amount, freight_cent=0, non_effective_capture=true, state=PENDING)`；`captured−refunded<=0` 的 intent 跳过；已存在同 `refund_id` 的行直接复用，不重复创建。
6. **再调用 Payment.Refund（最后一步副作用）**：按 `refund_id` 幂等逐笔退款；每次成功后把对应 application 置 `DONE`；所有未退完 intent 均 `DONE`（或本来已退完）后，才 CAS **`COMPENSATING→EXPIRED_REFUND`**。`EXPIRED_REFUND` 表示 **Payment 退款已完成**，不是“仅补偿完成、打款未完成”。
7. **打款失败/超时/未知**：**保持 `COMPENSATING`**；application 保持 `PENDING/PROCESSING`，告警并重试；**不得提前 CAS `EXPIRED_REFUND`，不得回 `OPEN/RESOLVING`**。重试前用 `GetRefundByKey` 确认；已退但本地未提交 → 补写 `DONE` 后再 CAS 终态；未退 → 重试；不得重复退款。
8. **补偿/打款失败/失租**：保持 `COMPENSATING`，租约内重试 + 告警；人工判定不可恢复时，CAS `COMPENSATING→EXPIRED_FINANCE` + 审计，不得由 worker 自动改终态。
9. 已离仓组一律 `EXPIRED_FINANCE`，不得取消运单/不得回滚履约；permit/运单/INV-44 处置**先于** Payment；Payment 成功后终态；**不得出现“已退款又发货”**。
10. 已解析出唯一生效支付后，若历史订单仍有其他已捕获 intent：按支付级退款或 `payment_duplicate_captures` 登记，不得静默留存；若生效支付尚未解析，则不得自动退款，按队列分支处理。

该顺序对应 AC-63 与 E25；实现必须与 `AdmitRefund`/`BlockShipmentForRefund` 共用同一 permit/运单 CAS 语义，不得另写第二套裁决；测试须覆盖“`COMPENSATING` 不回到 OPEN、打款失败不提前 `EXPIRED_REFUND`、`EXPIRED_REFUND` 表示 Payment 已完成、`EXPIRED_REFUND` 后不可再发货/不可重复退款”。

#### 4.4.4 并发与禁用捷径

0. **准入/发货入口**：`AdmitRefund` 与 `AuthorizeShipment` 在读取订单后先调用 `EnsureLegacyResolution`：
   - 返回 `RESOLVED`/`EXPIRED_ZERO` → 继续原动作（`EXPIRED_REFUND` 只允许已创建的支付级退款，不再新建组退款）；
   - 返回 `OPEN/RESOLVING/COMPENSATING` → `LEGACY_PAYMENT_UNRESOLVED`，未离仓组禁发、退款阻断；`COMPENSATING` 不提前退款；
   - 返回 `EXPIRED_FINANCE/REJECTED` → `LEGACY_PAYMENT_UNRESOLVED`，禁发禁退；
   - 唯一候选在 `Ensure` 内 CAS 写回后直接继续；不得在只读 BFF 查询中创建队列。
1. **所有写**（创建、状态迁移、`settlement_payment_no` 写回、终态退款申请）在 Order 事务内完成；远程 Payment 调用在事务外。
2. 队列创建与唯一候选写回共用 `orders` 行锁：先 Payment 读，再锁订单重读；锁内不得调 Payment。
3. `INSERT` 队列用 `ON DUPLICATE KEY UPDATE order_no=order_no`（或 `INSERT IGNORE`）+ `SELECT ... FOR UPDATE`；若已是终态直接返回；若 `OPEN/RESOLVING` 只更新 `candidate_count/pay_cent/blocked_scope`，**不重置状态/expires_at**；若 `COMPENSATING` 幂等返回当前状态，不改状态/不重置租约。
4. 唯一候选 CAS `WHERE settlement_payment_no=''`；0 行读回：与本候选相同 → 幂等成功；不同 → `SETTLEMENT_CONFLICT` + 进队列/告警，不覆盖。
5. `RESOLVED`：校验人工证据 → 锁队列 → `OPEN/RESOLVING→RESOLVED`（version CAS）→ CAS 写 `orders.settlement_payment_no` → 生成 `resolution_id` → 提交；写回冲突则回滚队列状态并转 `EXPIRED_FINANCE`/告警。
6. 24h worker：`SELECT ... WHERE state IN ('OPEN','RESOLVING') AND expires_at<=NOW(3) FOR UPDATE SKIP LOCKED` → 写 `lease_owner/lease_until` → 提交；事务外 `ListOrderIntents`；再 Tx 锁行、校验 `lease/version/state` → 按 §4.4.3 分支；失租/状态已变 → 不写；补偿或打款失败 → 保持 `COMPENSATING`。
7. `EXPIRED_ZERO`：终态事务 CAS 写 `settlement_payment_no='zero:<order_no>'`；不调 Payment。`REJECTED`/`EXPIRED_FINANCE`：不写退款、不放行发货；保留审计。
8. 恢复：`RESOLVED` 写回 payment_no + 生成 `resolution_id`；`ResumeLegacyResolution(order_no, resolution_id)` 幂等，只继续被阻断动作，不重复发货/退款；`COMPENSATING` 只由持租约 worker 继续。

#### 4.4.5 管理命令（C-04 输入）〔来源：handoff §1.3〕

新增 Order 管理 RPC（仅 ops/Director 授权；Payment 不写 Order 队列）：`GetLegacyResolution`（只读）、`ResolveLegacyResolution`（`order_no, resolution_id, payment_no, operator, authorized_by, evidence_ref`；`OPEN/RESOLVING→RESOLVED` CAS）、`ResumeLegacyResolution`（幂等恢复被阻断的 ship/refund）。reply 统一为 `LegacyResolution`：`order_no/state/candidate_count/resolution_id/pay_cent/blocked_scope/resolution_kind/expires_at_unix/resolved_payment_no/operator/authorized_by/version`；取值以 §4.4.2/§12.5.7 为准，不新增队列状态。

---

## 5. 退款准入与编排

### 5.1 退款分类与账本所有权

| 类别 | 来源 | `refund_id` | 是否经 `AdmitRefund` | 说明 |
|---|---|---|---|---|
| **组级退款** | 普通售后 / SYSTEM 缺货 / SYSTEM 零元 / TCC 补偿 | `after_sale_no`（售后/缺货/零元）、`tcc:<payment_no>:<group_no>`（TCC） | **是** | 受许可行/组额度/离仓状态约束；零元走短路（§5.2.5） |
| **支付级退款** | 非生效捕获 / 超少付 / 取消 / 历史 `EXPIRED_REFUND` | `pay:<payment_no>:<reason>`（`mismatch\|dup\|cancel\|legacy_expired`） | **否** | 只受 Payment captured 上限；不涉及履约 |

- `refund_applications` 统一记录两类；`source` 枚举 `aftersale|tcc|system_shortage|system_zero|payment|legacy_expired`；组级才有 `group_no`（支付级 `group_no NULL`）。
- **账本所有权**〔A-02/Amd1 §4.2〕：**Order 拥有** `refund_applications` 与 `refund_batches`（order DB）——Order 是准入与 commitment 所有者；AfterSales 是普通售后的业务发起方，通过 `AdmitRefund` 让 Order 幂等登记；Order 是 TCC/历史支付级退款的发起方。AfterSales 的 `after_sales` 仍是业务申请，Order 的 `refund_applications` 是**执行/准入账本**，二者以 `refund_id` 关联；SPEC S8“唯一发起方”仍成立。
- Payment 发起的支付级退款（重复捕获/取消由 Payment 结算 worker 处理）继续由 Payment `refunds` + `payment_duplicate_captures` 持久化，**Order 不复制该事实**；统一的是 `refund_id` 可追溯与幂等，不是单一物理表。
- 零元售后执行事实（S19）由 Order 的 `refund_applications(source=system_zero, amount=0, state=DONE)` 承载，不创建 Payment refund〔AZ-01/AZ-05〕。
- **边界**：INV-43“先准入后打款”只约束组级退款；支付级退款由 Payment captured 上限与 Order 的取消/重复捕获裁决保证，不经过组许可。

### 5.2 组级准入 `AdmitRefund`

#### 5.2.1 请求与前置校验 〔来源：Amd1 §4.4、A-12〕

`AdmitRefund` 追加（additive、不重编号）：`repeated RefundLine{line_no, quantity, goods_cent}`、`responsibility`（封闭枚举）、`reason_code`、`freight_refundable_cent`、`batch_id`、`source`。Order 计算：

- 行剩余 = 行实付 − AfterSales 已退（S14）− 该行在途承诺（S15 行组成）；
- 组商品剩余 = Σ 行剩余；组运费上限 = 已退运费台账之外、按责任可退的 `freight_share`；
- 组可退上限 = 商品剩余 + 可退运费；整组退款金额 = 商品剩余 + 本次可退运费；
- 订单级 `Payment captured − refunded − RESERVED` 为最终上限。

责任由 AfterSales/缺货事实产出；Order 只校验枚举/证据并计算，不推断、不改责任；`UNDETERMINED` 商品照退、运费挂起，组保持 REFUNDING；168h 起算按 D-26“该组该次售后首次进入 `UNDETERMINED`”，到期按 `MERCHANT_WAREHOUSE` 同组补退；补退幂等、累计不超实付运费。

**已离仓判定（事实在 Fulfillment）**〔T1.2/R-2〕：Order 组 `SHIPPED/RECEIVED` **或** Fulfillment shipment `SHIPPED/RECEIVED` **或** `exception_state != 'NONE'`（当前 `OPEN`）且 `shipped_at_unix>0`。Order 必须在 Tx1 外按 `order_no` 调 `ListShipments`（page_size=100，`total<=100`；超出或分页不完整 → fail-closed/人工）建 map；Fulfillment 超时、无 shipment 但 Order 组状态非 `PENDING/ALLOCATED`、多 shipment 冲突、`exception_state` 非 `NONE` 但无 `shipped_at` → **fail-closed，不写 batch/permit/admission/application**，告警+人工；不得猜“已离仓/未离仓”。

**历史队列闸门**〔Amd1 §5.3〕：读取订单后先调 `EnsureLegacyResolution`（§4.4.4）；非 `RESOLVED/EXPIRED_ZERO`（以及 `EXPIRED_REFUND` 的支付级分支约束）→ `LEGACY_PAYMENT_UNRESOLVED`，不得准入。

#### 5.2.2 两阶段执行

**事务外**：`Payment.GetIntent(effective_payment_no)` 取权威 captured/refunded（历史单先按 §4.4 解析）；`ListShipments` 取已离仓事实（§5.2.1）。

**Tx1（短事务，Order-owned，禁止远程调用）**：

1. `SELECT order FOR UPDATE`；校验订单 `status=PAID`、组存在且属于该订单、金额。
2. 额度校验（§4.3）；未离仓且金额 < 整组退款金额 → `PARTIAL_REFUND_REQUIRES_FULL_GROUP`，不改状态；含 `composition_known=0` 的组只允许整组退款（§5.6）。
3. 读取许可行：
   - `NONE` → CAS `REVOKED`；写 `order_refund_admissions(refund_id 唯一, result=PRE_SHIP_REVOKED)` + `order_refund_commitments(RESERVED)`（含行组成）；outbox `order.refund_admitted`；提交。
     - **CAS 失败（并发 Ship 先 `NONE→CONSUMED`）**：Tx1 回滚后重读许可/运单，按 `CONSUMED` 分支重新执行；不得直接把结果写成 PRE。
     - Fulfillment 消费 `order.refund_admitted` 的 PRE 结果，调用 `CancelIfCreated` 取消 `CREATED` 运单（幂等）；Order 崩溃不影响 outbox 投递（W2）。IF 消费时返回 `ALREADY_SHIPPED/RECEIVED`，THEN 不得静默忽略：写告警并发出 `order.refund_admission_recheck`，Order 若尚未打款则改为 `POST_SHIP_ALLOWED`；若已打款则停止后续退款并人工对账。
   - `REVOKED` → 幂等 `PRE_SHIP_REVOKED`（写/读 admission）。
   - `CONSUMED`：写 `admission(state=PENDING_REMOTE)` + commitment；提交后调用 `CancelIfCreated`（不依赖事务外预读运单状态）。
   - 已离仓组（§5.2.1 判定）：Tx1 只写 application，permit 保持 `CONSUMED`，**不写 commitment**。
4. `order_refund_commitments` 与许可 CAS 同事务。

**Tx1 后（远程，禁止在事务内）**：

- `CONSUMED`：调用 `Fulfillment.CancelIfCreated(order_no, group_no, shipment_no)`：
  - `CANCELLED`（含已取消）→ Tx2 写 `result=PRE_SHIP_REVOKED`（`PRE_SHIP_REVOKED` 由 `CONSUMED+CANCELLED` 收敛）。
  - `ALREADY_SHIPPED/RECEIVED` → Tx2 写 `result=POST_SHIP_ALLOWED`，并在**准入阶段**与 POST admission 同一事务写 `order_refund_commitments(RESERVED)`（T1.4a/R-B）。
  - `TRANSIENT_ERROR` → 保持 `PENDING_REMOTE`，worker 重试；**不调 Payment**。
- 已授权未离仓窗口的 `CANCELLED` 即 `CONSUMED+CANCELLED` 分支；Tx2 结果为 PRE（§5.8）。

**Tx2**：按 `refund_id` 更新 admission（唯一键），outbox `order.refund_admitted`；重复调用返回同一结果。

#### 5.2.3 `CONSUMED + CANCELLED` 分支

Tx2 结果 `PRE_SHIP_REVOKED`；许可保持 `CONSUMED`（终态），该组无运单可发；新运单因不同 `shipment_no` 被 `FULFILLMENT_NOT_ALLOWED`。W2 重试按运单状态收敛。**`CONSUMED` 不得写 `REVOKED`**〔A-06〕。

#### 5.2.4 准入结果持久化与重试

- `order_refund_admissions(refund_id PK, result, state, amount, version)`；结果只允许 `PRE_SHIP_REVOKED/POST_SHIP_ALLOWED`，中间态 `PENDING_REMOTE`；不改变主键/语义。
- 同 `refund_id`：读结果返回；缺失则重跑；`PENDING_REMOTE` 继续远程取消。不同 `refund_id`：组级按额度/离仓规则；支付级按 Payment captured。
- 所有 worker 带 lease owner；失租不得写 admission/commitment。
- 零元仍写 `PRE_SHIP_REVOKED/POST_SHIP_ALLOWED`，`amount_cent=0`〔AZ-04/Zero §7〕。

#### 5.2.5 零元短路（`source=system_zero`）〔来源：Zero §3.1 + AZ-01〕

**强制**：`AdmitRefund` 在 `source=system_zero && amount=0 && 订单 pay_cent=0 && 无任何已捕获 intent` 时，**跳过 Payment 权威额度与 commitment**，但仍执行 permit/运单/INV-44 准入，并在同一事务返回 `DONE`；正金额路径不变，不新增 RPC。

1. 参数：`refund_id=after_sale_no`、`source=system_zero`、`amount_cent=0`、`group_no`、可选 `lines`（零元商品 `goods_cent=0`）。
2. 订单校验：`pay_cent=0` 且**无任何已捕获 intent**（有捕获则不是零元售后，按 AC-40/AC-63 走支付级/人工队列）；`amount_cent=0`。
3. 走与正金额相同的组级准入：`NONE→REVOKED` 或 `CONSUMED+CREATED` 的 `CancelIfCreated`；已离仓组按 `POST_SHIP_ALLOWED`，不取消运单。
4. 写 `order_refund_admissions`；**不创建 commitment**（0 元无在途额度；含 0 元 commitment 也不创建）。
5. 写/复用 `refund_applications(refund_id=after_sale_no, source=system_zero, amount=0, goods=0, freight=0, state=DONE, admission_result=PRE/POST, non_effective_capture=0)`；返回 `state=DONE`。
6. 不调用 Payment；不写订单级 `captured/refunded`；不进入 `legacy_payment_resolution`。
7. 回库退款（RETURN_REFUND）必须**回库完成后**才调用本短路；REFUND_ONLY/缺货无回库，许可/运单/释放完成即可 DONE；不得在回库前标 DONE/REFUNDED〔A-Z6/AC-67〕。
8. 幂等：`refund_id`（`after_sale_no`）重复调用返回同一结果；不重复释放/回库。

### 5.3 `Fulfillment.CancelIfCreated`

```text
rpc CancelIfCreated(CancelIfCreatedRequest) returns (CancelIfCreatedReply)
CancelIfCreatedRequest { order_no; group_no; shipment_no; refund_id }
CancelIfCreatedReply { status: CANCELLED | ALREADY_SHIPPED | RECEIVED | TRANSIENT_ERROR }
```

- CAS `CREATED→CANCELLED`；已 `CANCELLED` 幂等；`SHIPPED/RECEIVED` 返回对应状态；不取消已离仓运单。
- **已离仓判定补充**〔T1.2/R-2 §3.2.5〕：`exception_state != 'NONE' && shipped_at` 必须返回 `ALREADY_SHIPPED`（不得落 `TRANSIENT_ERROR` 无限重试）；`exception_state` 非 `NONE` 但无 `shipped_at` → `TRANSIENT_ERROR` + 告警/人工，不取消。**无 proto 字段新增**。
- 与 Ship 的 `CREATED→SHIPPED` CAS 竞争，唯一胜者；返回码决定 admission 结果。
- deadline 1s；owner Fulfillment。T1.4 对 `CONSUMED` 组调本 RPC；EXCEPTION+shipped 期望 `ALREADY_SHIPPED`；TRANSIENT/unknown 不取消、不退款，保持 `PENDING_REMOTE`/人工。

### 5.4 支付级退款

- 触发：`PAYMENT_AMOUNT_MISMATCH`、`DUPLICATE_PAYMENT`、取消胜出（§3.4.2）、历史 `EXPIRED_REFUND`（§4.4.3）。**零元售后/缺货不属于支付级**，走组级 `system_zero`。
- 申请：`refund_id=pay:<payment_no>:<reason>`；`source=payment`（历史为 `legacy_expired`）；`non_effective_capture=true`；金额 = 权威 `captured − refunded` 净额（净额 0 不写命令）；无 group。
- 执行：Payment.Refund 直接幂等（`refund_id` 作为 `idempotency_key`）；Order 不写订单级 refunded（非生效）或仅写取消投影（取消）；不调用 `AdmitRefund`；**不使用 `RefundBatch`**：支付级单笔走 `Refund`，C-04 batch 仅用于 TCC 组级（§5.9）。
- 失败：`refund_applications`/Payment 持久重试；`FAILED_PERMANENT` + 人工；用户显示“退款处理中”。
- 支付级退款执行失败不得阻塞取消/回库（§3.7）；但不得静默丢弃命令。

### 5.5 三个组级来源与 TCC 补偿编排

| 来源 | 申请创建 | 准入 | 打款 | 恢复 |
|---|---|---|---|---|
| 普通售后 | AfterSales 本地事务写 `refund_applications` + task（Order 执行账本） | `AdmitRefund` | AfterSales worker → Payment | task 持久重试 |
| TCC 补偿 | Order 为每个**活动组**创建 `tcc:<payment_no>:<group_no>` | 逐组 `AdmitRefund` | 全部活动组准入完成后逐组打款 | 组级 task；只补剩余 |
| SYSTEM 缺货 | AfterSales 系统售后 + 申请 | `AdmitRefund` | 同普通售后 | 同普通售后 |
| SYSTEM 零元 | AfterSales 系统售后 + 申请（金额 0） | `AdmitRefund` 零元短路（§5.2.5） | 不调 Payment；`DONE(zero_amount)` | 同普通售后 |

#### 5.5.1 TCC 补偿（AC-65）〔来源：Amd1 §4.1 + A-05/A-06 + T1.2/R-1 + T1.4a/R-B〕

**对象**：`refund_batches`（`batch_id` 唯一；`(source, order_no, payment_no)` **非 UNIQUE**，允许多 attempt；同一时刻只允许一个 active batch，由 Tx1 订单锁 + state 检查保证）；每个活动组一条 `refund_applications`；`order_refund_admissions` 每 `refund_id` 唯一；`order_refund_commitments(+lines)` 承载额度占用与组成。

**强制顺序**：

```text
(1) 事务外：Payment.GetIntent(payment_no) 取 captured/refunded 快照；
    读取 Order 冻结的行/freight_share；计算订单剩余 captured 与 Σ 活动组系统责任上限。
(2) 预检：不等 → 不写 permit/commitment/订单退款态；写 refund_batches(state=PRECHECK_FAILED)
    仅作审计 + 告警 + 人工；结束。（T1.2/R-1：审计 batch 必须写；其余零写入）
(3) Tx1（短事务，禁止远程调用）：
    - SELECT orders FOR UPDATE；
    - 用同一 Payment 快照复核 Σ；不等 → rollback，不写任何状态；
    - UPDATE orders.refund_state=REFUNDING（CAS：非 FULL）；
    - INSERT refund_batches(state=FROZEN) + 活动组 refund_applications(PENDING)；
    - 分状态 permit：
        NONE      → CAS NONE→REVOKED；
        CONSUMED  → 不写 REVOKED；写 order_refund_admissions(state=PENDING_REMOTE)
                    + order_refund_commitments(RESERVED) + 组行组成（Tx1 内）；
        已离仓（SHIPPED/RECEIVED/EXCEPTION+shipped_at）→ 仅写 application，permit 保持 CONSUMED；
                    其 POST 组 RESERVED commitment 在准入阶段与 POST admission 同事务写（T1.4a/R-B），Tx1 不写；
    - 提交。
(4) Tx1 后：对 CONSUMED 组调用 Fulfillment.CancelIfCreated（幂等）；
    CANCELLED → Tx2 admission PRE_SHIP_REVOKED；
    ALREADY_SHIPPED/RECEIVED → Tx2 admission POST_SHIP_ALLOWED（POST commitment 在准入阶段写，T1.4a/R-B）；
    TRANSIENT_ERROR → 保持 PENDING_REMOTE，worker 重试；不得打款。
(5) 逐组 AdmitRefund（NONE 已 REVOKED 的组幂等返回 PRE；其余按 AC-50）；
    任一未 ADMITTED → batch 不进入 PAYING，不调用 Payment。
(6) batch=PAYING：按 C-04 RefundBatch 原子批打款（§5.9：expected_total_cent=outstanding、
    items 只含 outstanding 组、Σ NEW <= remaining、all-or-none；Order 预检门禁见 §5.9.4）；
    成功由 payment.refunded 收敛 commitment SETTLED；BATCH_UNAVAILABLE 保持 PAYING 退避重试；
    确定性错误 MANUAL_HOLD、零新 Payment；失败保留 application/commitment，只重试未完成组。
(7) 全部组 SETTLED/已退款/零元 → batch=DONE；否则 MANUAL_HOLD/FAILED_PERMANENT。
```

**关键规则**：

1. **Σ 预检在状态变更前**；Tx1 用同一 Payment 快照在订单锁内复核；Payment 调用禁止在事务/锁内。
2. **`CONSUMED` 不得写 `REVOKED`**；只走 `PENDING_REMOTE` + `CancelIfCreated`；Ship 胜出 → `POST_SHIP_ALLOWED`；已离仓组不取消运单。
3. **batch 持久化**：崩溃重跑按 batch/组 application 续跑；已 `SETTLED` 组跳过；不得创建第二套 `refund_id`。
4. **部分失败（跨批）**：C-04 批内 all-or-none；已由先前批次成功退款的组保留 `SETTLED`；失败/未完成组保留 `RESERVED` + application；batch `MANUAL_HOLD`/可重试；不得重退已完成组。
5. **快照变化**：PAYING 期间每次打款前重新读取 Payment 权威；若 remaining captured 变化导致 `Σ NEW` 超出 `remaining`，`RefundBatch` 返回 `INSUFFICIENT_REMAINING` → **停止后续打款**、batch=`MANUAL_HOLD`、告警；已打款组不回滚。这是 TCC 的批级正确性约束（§2.6/§5.9）。
6. **`PENDING_PAYMENT+已捕获`**：不进入组扇出，按 §3.4.2 CAS→`CANCELLED` + 支付级取消退款；CAS 失败（已 PAID）才进入组扇出。
7. **无活动组**：幂等成功，不调用 Payment；历史无组按 G1；零元不进入 Payment。
8. **Σ 不等**：写 `PRECHECK_FAILED` 审计 batch（含 `captured_snapshot_cent/refunded_snapshot_cent/sum_group_cap_cent/remaining_captured_cent`、`attempt_seq`），保留 Σ 数字 details + 告警；**不写**订单/permit/commitment/application/outbox/状态日志；`PRECHECK_FAILED` 不进入 `findTccBatchForResume` 可续跑集合，人工修正后新 attempt（`attempt_seq+1`）〔T1.2/R-1〕。
9. 测试要求：POST 组 admission + commitment 同事务；重复/并发不产生第二条；Tx1-only 断言无 commitment；`PRECHECK_FAILED` 只写审计 batch〔T1.4a/R-B〕。
10. **活动组定义**：系统责任上限（商品剩余 + 全部未退运费）> 0 且未全退的组。
11. **Σ 人工裁决**：`Σ 活动组系统责任上限` 必须 `== 订单剩余 captured`；不等 → 阻断、告警、人工裁决（owner：**Director + Architect**；动作：按 `payment_no` 对账/补录，**不部分打款**）。
12. **逐组全额**：不做最大余数分摊；全退组跳过；部分成功只补未 `DONE` 的组（不得重退已完成组）。

### 5.6 commitment 组成、释放与人工授权（AC-62）

**头表**：`order_refund_commitments` 增 `goods_cent`、`freight_cent`、`composition_known`、`composition_source`；`amount_cent = goods_cent + freight_cent`。**行表**：`order_refund_commitment_lines(refund_id, line_no, quantity, goods_cent, freight_cent)`——`line_no=0` 运费行（goods=0），`line_no>0` 商品行（freight=0）；`SUM(goods_cent)=头.goods_cent`、`SUM(freight_cent)=头.freight_cent`，事务内校验。

- 新承诺 `composition_known=1`；历史迁移行 `composition_known=0`，`goods_cent=amount_cent, freight_cent=0`（仅占额度、不归行）。
- **含 `composition_known=0` 承诺的组只允许整组退款；禁止行级部分退款**，直到人工补齐组成；否则 `REFUND_ADMISSION_CONFLICT`（A-03）。
- **释放**〔A-01 + D-24〕：
  - 只接受 AC-62 的合法触发（打款前 `REJECTED/CANCELLED`；`FAILED_PERMANENT`/人工放弃 + 授权）；每笔释放写 `refund_commitment_releases`（`refund_id` 唯一、`authorization_id` 唯一、金额、授权人/复核人/操作人、原因码、证据、有效期）。
  - **授权矩阵**：金额 ≥1000 元（`100000` 分）必须两位**不同**授权人（`authorized_by` 与 `reviewed_by` 均非空且不相等）；低于阈值至少一位授权人；`operator` 只执行、不得替代授权。任一缺失/越权/过期/金额不符 → `REFUND_RELEASE_NOT_AUTHORIZED`，不改额度/状态。
  - `UPDATE order_refund_commitments SET state='RELEASED' WHERE refund_id=? AND state='RESERVED' AND version=?`；0 行 → 读回当前状态，幂等或冲突。
  - 释放金额必须等于头金额；禁止部分释放；`SETTLED` 优先：若 Payment 已退，先 `RESERVED→SETTLED`，不得释放。
  - worker/脚本/重试入口无授权 → `REFUND_RELEASE_NOT_AUTHORIZED`；不提供内部自动释放开关；worker 不得静默释放。
- 释放额度回到**同一订单/同一组**的可退余额（D-25）；不得跨订单/跨组复用；释放不自动触发退款、不恢复 permit、不重开退款。

### 5.7 组退款后的库存与 INV-44 owner

**所有权**〔T1.4a/R-A〕：

| 事实/动作 | 唯一 owner | 说明 |
|---|---|---|
| 未离仓组预留释放（`HELD`/`CONFIRMED`） | **Inventory** | `Release` / `TccCancelConfirm` 的既有执行者；释放失败持久重试/告警 |
| 触发释放 | **Order** | `AdmitRefund`/TCC admission 只返回准入结果；由 Order 在准入事务后产生 **durable release 请求**（outbox/lease task），**禁止在 DB 事务内远程调用 Inventory** |
| 已离仓退货回库 | **AfterSales**（受理/授权）；**Inventory**（执行 Restock） | 既有 `after_sale_restock_tasks`；不得由 Fulfillment 直接改库存 |
| 运单取消 | **Fulfillment** | 只 `CancelIfCreated`，不释放库存 |
| 零元 | **同 R-A** | 不新增零元专用 release owner；同一 Inventory 执行、同一 Order/AfterSales 触发 |

**释放语义**（Rev 1.0 §5.7 保持）：

- 未离仓：许可 `REVOKED`/运单 `CANCELLED` 生效后释放（`HELD` 直接，`CONFIRMED` 走 `CancelConfirm`）；失败重试告警，不阻断退款完成，但禁止再发货。
- 已离仓：退货回库按 SPEC-0001 INV-03；回库成功后退款；已发货组**不写 release 事件**、不得 `Release`。
- 释放后 `available <= on_hand`、行数量守恒；违反告警并阻断该组。
- 零元与正金额走**同一** release 事件/consumer（`refund_id/order_no/group_no/items/warehouse_id`）；零元只跳过 Payment，不跳过 INV-44 释放/回库〔AZ-04/T1.4a §2.3〕。
- 现有 `inventory.compensation_requested` 只带 `order_no/items`，不足以按组释放；C-04 必须扩展组字段（不新增 RPC），C-05 无迁移。

### 5.8 `CONSUMED+CANCELLED` 与零元“先取消后事务”

- **零元 `CONSUMED` 组**〔T1.4a/R-C〕：允许先调 `Fulfillment.CancelIfCreated`，成功后再开事务写 `order_refund_admissions` + `refund_applications(source=system_zero, amount=0, state=DONE)`；取消幂等，事务失败不写 S19，重试幂等。**前提**：调用前已有持久 AfterSales refund task（或等价 lease/retry intent），保证进程崩溃/事务失败后可重试；仅同步调用、无持久任务时，必须先持久意图再远程取消。**禁止**：取消失败写 S19、无 worker 的 PENDING、事务内远程调用、把取消成功当退款完成而不写 S19。
- **正金额 `CONSUMED` 组**：保持原两阶段（§5.2.2），Tx1 只写 `PENDING_REMOTE` admission + application，Tx1 后 `CancelIfCreated`，Tx2 收敛；不得先取消后补写准入。

### 5.9 `Payment.RefundBatch` 原子批退款（C-04）〔来源：C-04 §1–§3；accepted 2026-11-24；K-1〕

> C-04 已由 Director 于 **2026-11-24** accepted，Rev 2.1 将其规范内容并入本文；C-1 于 **2026-11-25** 关闭（§18.1）。TCC 组级批打款 MUST 走本契约；支付级单笔仍走 §5.4 的 `Refund`。`fc295e5` 是 WIP，**不因 C-04 accepted 成为实现 PASS**。

#### 5.9.1 Proto

```proto
rpc RefundBatch (RefundBatchRequest) returns (RefundBatchReply);
rpc GetRefundByKey (GetRefundByKeyRequest) returns (.mall.v1.Refund); // 见 §2.5

message RefundBatchItem { string refund_id = 1; int64 amount_cent = 2; } // amount_cent > 0
message RefundBatchRequest {
  string payment_no = 1;
  string batch_id = 2;             // 幂等批标识；同一 payment_no 内唯一
  int64 expected_total_cent = 3;   // 必须 == Σ items；Order 的 outstanding
  repeated RefundBatchItem items = 4;
  string reason = 5;
}
message RefundBatchItemResult { string refund_id = 1; string refund_no = 2; string status = 3; int64 amount_cent = 4; } // status: REFUNDED/ALREADY_REFUNDED/CONFLICT
message RefundBatchReply { string batch_id = 1; string status = 2; int64 refunded_cent = 3; repeated RefundBatchItemResult item_results = 4; }
```

#### 5.9.2 语义（强制）

1. **输入校验**：`payment_no/batch_id` 非空；`items` 非空、`refund_id` 唯一、`amount_cent>0`；`expected_total_cent == Σ items.amount_cent`；否则 `INVALID_BATCH`，零写入。
2. **行锁**：在 `payment_intents` 行锁内计算 `remaining = captured - refunded`；外部退款/取消退款必须锁同一 intent 行。
3. **Σ（C-04/R-2）**：`Σ items != expected_total_cent` → `BATCH_SUM_MISMATCH`；`existingSum = Σ(已存在同 refund_id/amount 的 item)`、`newSum = Σ items - existingSum`、`remaining = captured - refunded`；**`newSum > remaining` → `INSUFFICIENT_REMAINING`**。既有 item 已计入 `intent.refunded_cent`，不得重复扣；全 NEW 时 `newSum == Σ items`。两个错误均零写入（不插 refund/outbox、不改 intent）。**不校验 `Σ items == B`**：`B` 不是 Payment 的输入或事实。
4. **幂等（C-04/R-1）**：`(payment_no, batch_id)` 的重复请求必须返回同一 `RefundBatchReply`；若 items/expected_total 与首次不同 → `IDEMPOTENCY_CONFLICT`，零写入。同一 `refund_id` 已存在且金额一致 → `ALREADY_REFUNDED`，可包含在成功批次中，不重复插行/事件；金额或 `payment_no` 不一致 → `ITEM_CONFLICT`，整批零写入。同 `batch_id` 不得混入新 `refund_id`（否则 `IDEMPOTENCY_CONFLICT`）。
5. **全成功或全失败**：所有 items 的 refund 行/outbox 在同一事务内写；任一项冲突/超限/失败 → 整个事务 rollback，不产生部分退款；不允许逐项成功后再报错。**批内原子性不扩展为跨批保证**；跨批重试只补未完成组（§5.5.1 规则 4）。
6. **Reply 固定**：`status` 在首次执行时确定并持久化到 `payment_refund_batches`，重复请求原样返回，不得重算/降级。首次有至少一个 NEW item → `REFUNDED`；即使后续重放时全部 item 已存在，仍返回 `REFUNDED`。首次全部 item 已存在（`newSum=0`）→ `ALREADY_REFUNDED`；重放仍返回 `ALREADY_REFUNDED`。`refunded_cent = Σ items`（本批全部 item 金额，含既有），不是 intent 总 `refunded_cent`；`item_results` 每项带 `refund_no/status/amount`。
7. **事件**：`payment.refunded` 只为**新插入**的 item 发一次；既有 item 不重复发事件，`item_results.status=ALREADY_REFUNDED`（§4.2、§5.9.5）。

#### 5.9.3 错误码与 Order 映射（C-04/R-4）

- 使用 apperr 标准 code + `details.reason`；reason 至少：`INVALID_BATCH`、`BATCH_SUM_MISMATCH`、`INSUFFICIENT_REMAINING`、`IDEMPOTENCY_CONFLICT`、`ITEM_CONFLICT`、`BATCH_UNAVAILABLE`、`PAYMENT_NOT_REFUNDABLE`（intent 从未 captured）、`BATCH_STATE_INVALID`（ledger/refund 事实不一致）。所有错误零写入，不得返回半成功。

| Payment reason | Order 行为 |
|---|---|
| `BATCH_UNAVAILABLE` | 保持 `PAYING`，退避重试；不得打款/部分写 |
| `INSUFFICIENT_REMAINING` | `MANUAL_HOLD`；零新 Payment |
| `BATCH_SUM_MISMATCH` | `MANUAL_HOLD`；调用方 bug/参数错误 |
| `IDEMPOTENCY_CONFLICT` | `MANUAL_HOLD`；同 batch 不同 items |
| `ITEM_CONFLICT` | `MANUAL_HOLD`；同 refund_id 不同 amount |
| `INVALID_BATCH` | `MANUAL_HOLD`（或调用方修正后新 batch） |
| `PAYMENT_NOT_REFUNDABLE` | `MANUAL_HOLD`/终态；不重试 |
| `BATCH_STATE_INVALID` | `MANUAL_HOLD`/终态；不重试 |

#### 5.9.4 Order 预检边界（C-04 §3）

Order 负责并 MUST 在调用 `RefundBatch` 前完成：

- `B = Σ batch.sum_group_cap_cent`；`done`（commitment `SETTLED` 或 `GetRefundByKey` 成功）；`inFlight`（`PENDING/PROCESSING`）；`outstanding = B - done - inFlight`；`unknown`（application `DONE` 但无 Payment 事实）；`R = captured - refunded`；
- 门禁：`B == done + inFlight + outstanding`、`unknown == 0`、`R == B - done`；否则 `MANUAL_HOLD`、零 Payment；
- `inFlight > 0` 不调用 `RefundBatch`；调用时 `expected_total_cent = outstanding`（**不是 B**），items 只含 outstanding 组。

Payment 不重复 Order 的 `B` 计算；只保证 `Σ items == expected_total_cent`、`Σ NEW items <= remaining` 且 all-or-none。

#### 5.9.5 存储与事件（C-04/R-3、R-5）

- Schema 定稿见 §12.5.8；C-05 迁移路径 `migrations/payment/**`（C-04 记录为 `0012_refund_batch.sql`）。**不允许**逐项写入/部分写入，避免“先插部分 refunds 再回滚”的审计歧义。
- `refunds.batch_id` 只对**新写入**行赋值；mixed 的既有行保持原值（通常 `''`）；`item_results` 保存每项 `refund_id/refund_no/status/amount`，既有项 `refund_no` 必须来自既有 refund 事实。
- `payment_refund_batches.status` 只允许 `REFUNDED/ALREADY_REFUNDED`（应用层校验；可加 CHECK 但非阻塞）。
- 事件：仅新写入 item 发 `payment.refunded`（§4.2）；既有 item 不重发；审计以 `payment_refund_batches.item_results` + `refunds.batch_id` 区分“既有未改写/本批新写”。

---

## 6. 履约准入（Order/Fulfillment）

### 6.1 Ship 顺序

1. Fulfillment 读运单：`CANCELLED` → `SHIPMENT_CANCELLED`；`SHIPPED/RECEIVED` → 幂等；`CREATED` 继续。
2. 调 `Order.AuthorizeShipment`：
   - `PAID` + 行 `CONFIRMED` + 无活动缺货 + permit `NONE` → `NONE→CONSUMED`。
   - `CONSUMED` 同一 `shipment_no` → 幂等；不同/`REVOKED` → `FULFILLMENT_NOT_ALLOWED`。
   - 未知 permit 状态 → fail-closed，不返回 ship（A-11/D-29）。
3. Fulfillment CAS `CREATED→SHIPPED`；与取消竞争，唯一胜者。

### 6.2 自动建单与 `order.settled` 消费模式（D-27）〔来源：Amd1 §3.2 + A-09〕

> **取代关系**：Rev 1.0 §6.2/§12.3 的“灰度期 Fulfillment 直接消费 `order.settled` 建单”表述**已被 D-27 取代**（Amd1 §2.1 映射表）。本版以本节为准。

用**三态枚举**替代原布尔开关：

```text
FULFILLMENT_SETTLE_CONSUMER_MODE = paid | shadow | settled
默认 = paid（仅当未请求模式；R1 生产默认；等价于旧 false）
```

| 模式 | 行为 | 允许条件 |
|---|---|---|
| `paid` | 只消费 `order.paid`（旧路径）；`order.settled` 仍双发但不消费 | 未请求模式时的默认（或显式 `paid`）；`SETTLE_RELEASE_APPROVAL_ID` 必须为空（N5）；无需批准 |
| `shadow` | 同时消费 `order.paid` 与 `order.settled`；**都只记录/比对，不建运单、不推进组、不放行** | Director 影子批准（`target_mode='shadow'`、`status='APPROVED'`；`expires_at` 非 NULL 时未过期）+ 影子就绪清单 |
| `settled` | 以 `order.settled` 建单/推进；`order.paid` 只 BFF/旧消费者 | SHADOW→ON 全量 release 门禁 + Director 批准（`target_mode='settled'`、`status='APPROVED'`、`expires_at` 非空且未过期、`release_range` 匹配） |

**启动校验（fail-closed）**：

1. 未设置 mode 且旧布尔未设置/`false`，**且 `SETTLE_RELEASE_APPROVAL_ID` 为空** → **`paid` 默认**（唯一允许的默认路径）。
   - **N5（config error）**：`SETTLE_RELEASE_APPROVAL_ID` 非空但未请求 `settled`（mode 未设置/空/`paid`/`shadow`，旧布尔非 `true`）→ **启动失败（非零退出）**；approval-id 不得单独生效，也不得默认 `paid`。
2. 未知/空/大小写不符 → **启动失败**（非零退出），不得默认 `paid`。
3. 请求 `settled`：`SETTLE_RELEASE_APPROVAL_ID` 必须非空，且同一次启动校验必须存在 `settlement_switch_approvals` 行：`target_mode='settled'`、`status='APPROVED'`、`expires_at` **非空且未过期**、`release_range` 与本部署 release range 一致。**任一不满足 → 启动失败（非零退出）**；不得进入 `settled`、不得降级 `paid`、不得告警后继续。
4. 请求 `shadow`：必须存在 `target_mode='shadow'`、`status='APPROVED'` 的批准行；`expires_at` 非 NULL 时不得过期。**任一不满足 → 启动失败（非零退出）**，不得降级 `paid`。
5. 批准行只能由人类运维/受控迁移写入；应用无写权限。表在 Fulfillment DB。
6. 任何运行期修改 `FULFILLMENT_SETTLE_CONSUMER_MODE` 不热生效；必须重启并重新校验。脚本/自动任务无法绕过启动校验。
7. **旧布尔开关兼容**：`FULFILLMENT_CONSUME_ORDER_SETTLED=false`/未设置 → 未请求模式（`paid` 默认）；`true` → 视为请求 `settled`，同样必须通过 `SETTLE_RELEASE_APPROVAL_ID` 校验；**无/过期/非 `APPROVED`/range 不匹配 → 启动失败（非零退出）**，不得按 `paid` 运行。新旧配置同时存在且冲突 → **启动失败**（fail-fast），不静默择一。
8. `shadow`/`settled` 的批准行内容（`target_mode/status/expires_at/release_range`）由 Fulfillment 启动校验；config 包不得猜测或替代。
9. **回退限制**：`settled→paid` 只能走 §6.4 的 Director 批准回滚 + 重启；不存在“批准行缺失/过期 → 自动回 `paid`”的路径。运行期不轮询 `expires_at`、不热切换：到期只使下一次启动失败，不会自动改模式。启动失败必须非零退出并记录原因，不得自动写入/修改批准行。

- 旧 `order.paid` 消费者在 `paid`/`shadow` 期继续兼容；`settled` 模式后 Fulfillment 不得从 `paid` 推进组或建可发货运单（§12.4）。
- 影子只记录/比对，**不建运单**；因此 `shadow` 期间自动建单暂停，需 Director 在影子批准时确认运营可接受手工/既有运单（§13.20）。

### 6.3 影子比对〔来源：Amd1 §3.3〕

字段、版本与差异类型见 §1.3。补充约束：

- 差异只写 `settlement_shadow_diffs`、告警、对账；**不得阻断结算、不得放行/阻断发货、不得自动修复**。
- 影子差异率（按小时/订单量）是 SHADOW→ON 门禁输入；具体阈值由 Director 在影子批准时写入批准行，不在代码硬编码。
- `settlement_shadow_diffs` 唯一键 `(order_no, payment_no)`；`state ∈ MATCH|MISMATCH|ERROR`；`diff_fields JSON`；重复事件累加 `version`、更新 `last_seen_at`；重复 `(order_no, payment_no)` 事件只更新 `last_seen_at`，不重复记差异。
- 影子运行期失败只告警，不阻断结算；**请求 `shadow`/`settled` 但批准行缺失/过期/非 `APPROVED`/range 不匹配 → 启动失败（非零退出，§6.2）**，不得回退 `paid`。
- **N5**：`SETTLE_RELEASE_APPROVAL_ID` 非空但未请求 `settled`（未设置/空/`paid`/`shadow`）→ **启动失败（非零退出）**；这是配置错误，不得静默默认为 `paid`。

### 6.4 切换步骤、移除日期与回滚〔来源：Amd1 §3.4 + A-10〕

**步骤（R1 不执行 1–4；只交付代码与默认 `paid`）**

0. **R1 部署**：`mode=paid`；`shadow`/`settled` 代码存在但未启用；行为不变。
1. **OFF→SHADOW**：Director 批准影子（`settlement_switch_approvals` 行，`target_mode='shadow'`）；双消费、只记录/比对；不建单。观察期由 Director 指定（建议 ≥7 天且覆盖多组/零元/历史单样本）。该步骤使用**影子批准 + 影子就绪清单**，**不套用** SHADOW→ON 的全量 release 门禁。
2. **影子验收**：无 `UNKNOWN_VERSION/MISSING_FIELD/MISMATCH_*` 阻断；差异率低于批准阈值；`order.paid` 与 `order.settled` 对平记录可查。
3. **SHADOW→ON**：必须同时满足 `SPEC-0002 §7 矩阵 + 五方终态 + Reviewer 人工核对 + review-tracking 记录（协作闸门已退役，无命令可跑）` + Director 批准；`mode=settled`；**该模式启动时必须通过 §6.2 的批准行校验（含 `expires_at` 非空/未过期、`release_range` 匹配），任一不满足即启动失败，不得降级 `paid`**；`order.paid` 停止建单/推进组，只保留 BFF/旧消费者兼容。
4. **ON 记录**：`target_mode='settled'` 的同一批准行写入逻辑字段 `order.paid_admission_retired_at`（SQL 列 `order_paid_admission_retired_at`）与 `order.paid_subject_removal_date`（SQL 列 `order_paid_subject_removal_date`），**建议 ≤ ON+30 天**；未填写不得 ON；超过 30 天必须 Director 书面批准并记录理由。
5. **移除**：移除日期到达且 BFF/旧消费者迁移完成后，单独立项停止 `order.paid` 生产/消费；**R1 不移除 subject**。

**回滚**：

- `ON→SHADOW/OFF` 需 Director 批准 + 回滚演练/事件单；该回滚不套用 SHADOW→ON 全量门禁，但必须记录批准与原因；保留任务、退款申请、影子差异记录，可重放；不做 down migration。
- **`settled→paid`/`settled→shadow` 只能由本节的 Director 批准回滚执行**：写入批准/回滚事件单并记录原因，再以目标模式重启；不存在“批准行缺失/过期/不匹配 → 自动回 paid”的路径，也不得由启动校验自行改写批准行。
- 回滚后若恢复 `order.paid` 建单，必须经过许可、库存、缺货判定，并以 `(order_no, group_no)` 唯一运单防重；已存在运单不得重复创建。
- `shadow→paid` 可直接回退，不改变数据；`settled→shadow` 视同 `settled→paid`，必须 Director 批准并记录；回滚不得把已由 `settled` 建立的运单/许可回滚，只前向兼容。

---

## 7. 库存就绪与缺货

- 结算前：`CheckOrderReady` 强制同步；不可用 fail-closed，不 PAID。
- **TOCTOU 规则（收窄）**：Inventory **不得自行释放 `CONFIRMED` 预留**；`HELD` 的 TTL 过期释放保留（它是未支付订单的安全网，PAID 要求全行 `CONFIRMED`，因此不会产生 `PAID + RELEASED`）。`CONFIRMED` 的释放只能由 Order 的取消/退款命令触发，且 Order 先在 `orders.status` 上 CAS（PAID/CANCELLED）成功后才发命令。若现有 Inventory 对 `CONFIRMED` 也有过期释放，必须在 P0 关闭并列入前置核查。
- **释放执行 owner**：Inventory（§5.7）；Order 只产出持久请求；禁止在 DB 事务内远程调 Inventory。
- `inventory_ready_version` 只作审计/展示，不作为 PAID 的 CAS 条件；正确性由“释放命令后置 + Order CAS 唯一胜者”保证。
- 支付后缺货：案件 `NOTIFIED/RESOLVED_WAIT`；预留保持 `CONFIRMED`；禁发；`RESTOCKED` 恢复；`RESOLVED_REFUND` 释放 + 组退款。
- 零元缺货：按 AC-52/AC-56 组级流程，金额 0、不调 Payment，仍执行 INV-44 释放/回库〔AZ-04/AC-67〕。
- 4h/48h/ETA/迟到补货：按 SPEC-0002 AC-56 与 SPEC-0001 §6.6。
- **启动配置一致性（N5，跨切面）**：`SETTLE_RELEASE_APPROVAL_ID` 非空但未请求 `settled` → 进程 **启动失败（非零退出）**，不得默认 `paid`；库存就绪/结算行为不因此放宽，详见 §6.2。

---

## 8. 状态机与幂等键

```text
Order:  PENDING_PAYMENT -> PAID -> ALLOCATED -> SHIPPED -> COMPLETED
        PENDING_PAYMENT -> CANCELLED
        （PENDING_PAYMENT+已捕获：CAS -> CANCELLED + 支付级取消命令，§3.4.2）
Permit: NONE -> CONSUMED | REVOKED（终态；三值枚举，无 GRANTED）
        CONSUMED -X-> REVOKED；REVOKED -X-> CONSUMED
Refund app: PENDING -> PROCESSING -> DONE | FAILED_PERMANENT | CANCELLED | REJECTED
            （零元短路：同事务直接 DONE，无 PENDING 悬挂）
Admission: PENDING_REMOTE -> PRE_SHIP_REVOKED | POST_SHIP_ALLOWED
Commitment: RESERVED -> SETTLED（Payment 退款成功）
            RESERVED -> RELEASED（打款前 REJECTED/CANCELLED，或 FAILED_PERMANENT/放弃 + 人工授权）
            禁止：SETTLED->RELEASED；RELEASED->RESERVED；worker 自动释放
Batch: PRECHECKED -> FROZEN -> ADMITTING -> ADMITTED -> PAYING -> DONE
                  |-> PRECHECK_FAILED（仅审计）
                  |-> MANUAL_HOLD / FAILED_PERMANENT
PaymentRefundBatch: 首次有 NEW item -> REFUNDED
                    首次全部既有 -> ALREADY_REFUNDED
                    （持久化后重放原样返回；不重算/不降级；无其他状态）
Legacy: OPEN -> RESOLVING -> RESOLVED
        OPEN/RESOLVING --24h 无捕获--> EXPIRED_ZERO | EXPIRED_FINANCE
        OPEN/RESOLVING --24h 有捕获无离仓--> COMPENSATING
        COMPENSATING --补偿完成且 Payment 全部成功--> EXPIRED_REFUND
        COMPENSATING --Payment 失败/超时/未知--> COMPENSATING（重试+告警）
        COMPENSATING --Ship胜出/未知/人工不可恢复--> EXPIRED_FINANCE
        COMPENSATING --补偿失败/失租--> COMPENSATING（重试+告警）
        OPEN/RESOLVING -> REJECTED
        禁止：COMPENSATING -> OPEN/RESOLVING；COMPENSATING -> 提前 EXPIRED_REFUND；终态 -> OPEN
Case: NOTIFIED -> RESOLVED_WAIT -> RESTOCKED | RESOLVED_REFUND
      NOTIFIED -> RESTOCKED | RESOLVED_REFUND
Task: PENDING -> PROCESSING -> DONE | PENDING(retry) | FAILED_PERMANENT
Shadow: MATCH | MISMATCH | ERROR（只用于告警/对账，不参与准入）
AfterSales(零元): REQUESTED -> APPROVED -> REFUNDING -> REFUNDED（refunded_cent=0）
```

| 对象 | 键 | 生成方 |
|---|---|---|
| 支付意图 | `(order_no, amount_cent, channel)` | Payment |
| 结算 | `payment_no`；TCC gid `payment-confirm:<payment_no>` | Payment |
| 生效支付 | `orders.settlement_payment_no`（`''` 表示空） | Order |
| 发货许可 | `(order_no, group_no)` | Order |
| 组级退款申请 | `refund_id`（after_sale_no / tcc:<payment_no>:<group_no>） | 发起方 |
| 支付级退款 | `refund_id=pay:<payment_no>:<reason>` | Payment/Order |
| 退款执行 | `refund_id` + `payment_no`（`GetRefundByKey`） | Payment |
| 零元退款执行（S19） | `refund_id=after_sale_no`；`source=system_zero` | Order |
| Payment 批退款 | `(payment_no, batch_id)`；`refund_id` 即 `idempotency_key` | Payment |
| Payment 批退款 item | `(payment_no, batch_id, refund_id)` 唯一 | Payment |
| TCC 批次 | `batch_id=tcc:<payment_no>:<attempt_seq>`；active batch 由 Tx1 订单锁 | Order |
| 承诺组成 | `(refund_id, line_no)`；`line_no=0` 为运费行 | Order |
| 承诺释放 | `refund_id` 唯一 + `authorization_id` 唯一 | Order（人工授权） |
| 历史解析队列 | `order_no` PK；`resolution_id=lpr:<order_no>` 唯一（终态） | Order |
| 历史退款号 | `legacy:<order_no>:<refund_seq>`（`order_refund_sequences`） | Order |
| 影子差异 | `(order_no, payment_no)` | Fulfillment |
| 切换批准 | `approval_id`；`(target_mode, status)` 索引 | 人类运维/受控迁移 |
| 缺货案件 | `(order_no, group_no, case_seq)` + `event_id` | Order |
| 组级库存释放 | `release_id=rr:<refund_id>` | Order 产出、Inventory 执行 |

---

## 9. 恢复所有者（崩溃窗口）

### 9.1 Rev 1.0 窗口（W1–W5）

| # | 窗口 | 持久意图 | 唯一驱动方 | 人工入口 |
|---|---|---|---|---|
| W1 | 捕获本地提交后崩溃（DTM 未受理） | `payment_settlement_tasks(PENDING)` | Payment settlement worker | 按 `payment_no` 重放 |
| W2 | 跨服务 RPC 成功后本地未记录 | `refund_applications` + `order_refund_admissions(refund_id)` | 发起方 worker；Order 按 `refund_id` 幂等 | 按 `refund_id` 重放；`CancelIfCreated` 幂等 |
| W3 | 消息重复/乱序 | `event_id` 唯一约束 + 状态 CAS | 各消费者 | 死信 + `event_id` 重放 |
| W4 | 取消/确认竞态 | `orders.status` CAS + 支付级退款申请 | Order | 按 `order_no` 状态日志 |
| W5 | 外部渠道/承运商不再重放 | settlement/refund/短缺失效定时器 | Payment/AfterSales/Order worker | 按 `payment_no/refund_id/order_no` |

### 9.2 Rev 2.0 新增窗口（W6–W13）〔来源：Amd1 §9 + T1.4a/R-A〕

| # | 窗口 | 持久意图 | 驱动方 | 人工入口 |
|---|---|---|---|---|
| W6 | TCC 预检后/Tx1 前后崩溃 | `refund_batches` + `refund_applications` | Order worker | `batch_id` / `payment_no` |
| W7 | `CONSUMED` 取消运单超时 | `order_refund_admissions(PENDING_REMOTE)` | Order worker + Fulfillment `CancelIfCreated` | `refund_id` |
| W8 | TCC 打款部分成功 | `order_refund_commitments(RESERVED)` + application | Order worker 只补未完成组 | `refund_id` / `payment_no` |
| W9 | commitment 已退未记 | Payment `refunds` + `refund_id` | Order `payment.refunded` 消费者 + 惰性对账 worker | `refund_id` |
| W10 | 历史解析 24h | `legacy_payment_resolution(COMPENSATING)` | Order expiry/compensation worker（补偿 + Payment 重试） | `order_no` / `resolution_id` |
| W11 | 队列终态退款 | `refund_applications(source=legacy_expired)` | Order worker → Payment | `payment_no` / `refund_id` |
| W12 | 影子差异 | `settlement_shadow_diffs` | Fulfillment shadow consumer | `order_no` / `payment_no` |
| W13 | INV-44 释放请求未送达/失败 | Order durable release 请求（outbox/lease task）；Inventory 组级幂等 | Order producer → Inventory executor | `refund_id` / `group_no` |

- 每个 worker 有 lease/owner fencing；失租不得写聚合/事件；所有 worker 必须 `lease_owner/lease_until`（或 `FOR UPDATE SKIP LOCKED`）。
- 任务有最大尝试与告警；永久失败 `FAILED_PERMANENT` + 人工重放。
- 禁止把回调重放/进程存活/消息重复作为 W1/W2 等窗口的唯一恢复。

---

## 10. 错误语义

| 错误码 | 返回方 | 语义 |
|---|---|---|
| `PAYMENT_AMOUNT_MISMATCH` | Payment | 金额 ≠ pay_cent，不生效 |
| `DUPLICATE_PAYMENT` | Order/Payment | 非生效捕获 |
| `ORDER_CANCELLED` | Order | 取消胜出 |
| `ORDER_ALREADY_PAID` | Order | 支付胜出 |
| `ORDER_STATE_CONFLICT` | Order | 非法生命周期迁移 |
| `SETTLEMENT_CONFLICT` | Order | 结算状态冲突；含未知 permit 状态（A-11） |
| `INVENTORY_NOT_READY` | Order/Inventory | 预留未确认 |
| `FULFILLMENT_NOT_ALLOWED` | Order | 许可被拒 |
| `SHIPMENT_CANCELLED` | Fulfillment | 运单已取消 |
| `REFUND_ADMISSION_CONFLICT` | Order | 额度/状态冲突；含 `composition_known=0` 组行级退款、Fulfillment 事实不可用 |
| `PARTIAL_REFUND_REQUIRES_FULL_GROUP` | Order | 未离仓部分退款被拒 |
| **`REFUND_RESPONSIBILITY_INVALID`** | Order | 责任枚举/证据非法（A-03/§5.2.1） |
| **`REFUND_RELEASE_NOT_AUTHORIZED`** | Order | 释放缺授权/越权/过期/金额不符（A-01/D-24） |
| **`REFUND_COMMITMENT_CONFLICT`** | Order | 释放 CAS 冲突/状态不允许（A-01） |
| **`TCC_REFUND_SUM_MISMATCH`** | Order | TCC Σ 预检不等（AC-65；审计 batch 已写） |
| **`LEGACY_PAYMENT_UNRESOLVED`** | Order | 历史生效支付未解析，禁发/禁退（Amd1 §5.3） |
| **`PAYMENT_CANCEL_NO_CAPTURE`** | Order | 无 captured/无 payment_no，未写支付级取消命令（T1.6/CB-4） |
| **`PAYMENT_CANCEL_REFUND_RETRY`** | Payment/Order | 支付级取消失败/超时，退避重试（T1.6/CB-4） |
| **`PAYMENT_NOT_REFUNDABLE`** | Payment | 不可退（从未 captured）（T1.6/CB-4） |
| **`ALREADY_REFUNDED`** | Payment | 已全额退，不重复（T1.6/CB-4） |
| `SHORTAGE_CASE_CLOSED` | Order | 案件终态重复/迟到补货 |
| `SHIPMENT_NOT_SHIPPED` | Fulfillment | 未发货不能确认收货 |
| `RECEIPT_DISPUTE_OPEN` | Fulfillment | 争议未决 |
| `EVENT_OUT_OF_ORDER` | 消费者 | 乱序拒绝 |
| `TRANSIENT_ERROR` | 各服务 | 可重试；不得 ack/标成功 |

- 异步命令/事件路径不扩 apperr/proto；稳定 `details.reason` + log + metric 是唯一可观测口径（T1.6/CB-4）。

---

## 11. 可观测性与对账

- 指标：捕获未结算、settlement task 失败、退款申请状态、准入冲突/`PENDING_REMOTE` 时长、预留释放失败、缺货超时、`PAID` 与 Payment 不一致数；**新增**：TCC `PRECHECK_FAILED`/`MANUAL_HOLD` 计数、commitment `RESERVED` 陈旧数、历史队列各状态时长、影子差异率（`settlement_shadow_diffs`）、`payment_cancel_cas_lost_total`、支付级取消重试数。
- 告警：30 分钟未结算；退款 `FAILED_PERMANENT`；准入冲突；释放失败；4h/48h 超时；历史队列 `COMPENSATING` 超时/打款重试；`EXPIRED_REFUND` 后仍尝试发货；影子 `UNKNOWN_VERSION/ORDER_LOOKUP_FAILED`。
- 日终对账：Payment 订单级/intent 级已收未 PAID、非生效/支付级退款、Order 投影、未完成任务、释放结果；**新增**：行级/运费组成（行已退 S14 + 在途承诺 S15 + 行剩余 = 行实付；累计退运费 ≤ 实付运费）、零元 S19 `DONE` 计数与 AfterSales `REFUNDED/0` 对平（无 Payment 退款）、`legacy_payment_resolution` 终态与退款事实对平。
- 五方终态核对：Payment/Order/Inventory/Pricing/Fulfillment。

---

## 12. 迁移、上线与回滚

### 12.1 新增/复用结构

| 结构 | 决策 |
|---|---|
| `payment_callbacks` | **复用**（已有 `UNIQUE(channel,event_id)`） |
| `payment_settlement_tasks` | 新增：`payment_no PK, state, attempts, lease_owner, lease_until, dtm_gid, dtm_status, last_error` |
| `payment_duplicate_captures` | 新增：`payment_no PK, order_no, amount_cent, refund_id, state` |
| `orders.settlement_payment_no` | 新增列（`NOT NULL DEFAULT ''` 兼容历史；唯一候选 CAS `WHERE =''`）；`inventory_ready_version BIGINT` |
| `order_refund_admissions` | 新增：`refund_id PK, order_no, group_no, result, state, amount_cent, version` |
| `order_refund_commitments` | 新增：`(order_no, group_no, refund_id) UNIQUE, amount_cent, state(RESERVED/SETTLED/RELEASED), version`；**增列** `goods_cent/freight_cent/composition_known/composition_source`（§12.5.3） |
| `order_refund_commitment_lines` | **新增**（§12.5.3）：`(refund_id, line_no)` PK；商品/运费组成 |
| `refund_applications` | 新增：`refund_id PK, batch_id NULL, source, order_no, group_no NULL, non_effective_capture, amount_cent, goods_cent, freight_cent, state, admission_result, attempts, last_error, version`（§12.5.2） |
| `refund_batches` | **新增**（§12.5.1）：`batch_id PK`、state 枚举、Σ 快照、lease/version |
| `refund_commitment_releases` | **新增**（§12.5.4）：`release_id PK`、`UNIQUE(refund_id)`、`UNIQUE(authorization_id)`、≥1000 元双人复核 CHECK |
| `order_refund_sequences` | **新增**（§12.5.5）：`order_no PK`、`next_seq >= 1`；`legacy:<order_no>:<seq>` 取号 |
| `legacy_payment_resolution` | **新增**（§12.5.7）：`order_no PK`、`UNIQUE(resolution_id)`、八态 CHECK、lease/version |
| `settlement_shadow_diffs` | **新增**（§12.5.6，Fulfillment DB）：`(order_no, payment_no)` PK、`state MATCH/MISMATCH/ERROR` |
| `settlement_switch_approvals` | **新增**（§12.5.6，Fulfillment DB）：`approval_id PK`、`target_mode=shadow\|settled`；`settled` 行必须有 `order_paid_admission_retired_at` + `order_paid_subject_removal_date`，且 `expires_at NOT NULL`（CHECK） |
| `pricing_confirmation_tasks` | 新增：`order_no PK, state, attempts, last_error` |
| `order_shipping_permits` | 复用；D-29 预检通过后加 `CHECK(state IN ('NONE','CONSUMED','REVOKED'))`，不新增列 |
| 既有 `after_sale_*`/`shipments` | 复用，不重命名 |
| `refunds` / `payment_refund_batches` | **新增/变更（C-04，§12.5.8）**：`refunds.batch_id` + `uk_refunds_batch_item`；删除 legacy `uk_refunds_idem`；新增 `payment_refund_batches` 批账本（首次 Reply 固定） |

金额一律 `_cent BIGINT`；可变聚合有 `version`；所有新表有 `created_at/updated_at`。新表/列全部 additive；状态用 `VARCHAR + CHECK`；不做 down migration。

### 12.2 回填

- 历史 `PAID`：按 §4.4 解析/回填 `settlement_payment_no`；0/多候选入 `legacy_payment_resolution`；无法解析入人工队列；**不按 0 额度处理**。
- 历史在途退款：`refund_id=legacy:<order_no>:<refund_seq>`（`order_refund_sequences` 取号，不重复/不回退），映射既有 after-sales task；dry-run 计数 + 人工队列。
- 历史 commitment：`composition_known=0`、`goods_cent=amount_cent`、`freight_cent=0`；只允许整组退款，行级部分退款阻断，等待人工补齐（A-03）。
- `order_shipping_permits` 的 `GRANTED` 预检（D-29）：迁移前执行
  ```sql
  SELECT order_no, fulfillment_group_no, state, shipment_no, after_sale_no
  FROM order_shipping_permits
  WHERE state NOT IN ('NONE','CONSUMED','REVOKED')
  LIMIT 100;
  ```
  返回任何行 → **停止部署**，人工分类（有 `SHIPPED/RECEIVED` shipment → `CONSUMED` + `shipment_no`；组 `refund_state ∈ {REFUNDING,REFUNDED}` 或 `after_sale_no` 非空 → `REVOKED`；无 shipment 且无退款证据 → 才可 `NONE`；任一不确定 → `REVOKED` fail-closed）；每次分类写审计（操作人/时间/证据/旧值/新值）；不提供自动映射脚本。
- `refund_applications` 的 zero CHECK：C-05 新增迁移（建议 `migrations/order/0021_zero_application_check.sql`，下一个可用号）：`source='system_zero'` 时 `amount_cent=0 AND goods_cent=0 AND freight_cent=0 AND non_effective_capture=0`；**不得修改已合入的 `0017`**；无新表/新列/新枚举〔Zero §7〕。
- 所有回填可重跑、可回滚；不在高峰执行；dry-run 计数对账。

### 12.3 灰度与消费者切换〔来源：Amd1 §3.2/§3.4；取代 Rev 1.0 §12.3 的旧开关步骤〕

0. **影子（D-27）**：Fulfillment 只双消费并记录 `settlement_shadow_diffs`/告警；**不建单、不推进组、不放行、不写 settlement task/refund application**；行为不变。
1. **`mode=paid`（R1 默认）**：`order.paid` 为唯一消费路径；`order.settled` 双发不消费；代码可部署 `shadow`/`settled` 但未启用；**`SETTLE_RELEASE_APPROVAL_ID` 必须为空（非空即 N5 启动失败）**。
2. **OFF→SHADOW**：Director 影子批准（`settlement_switch_approvals(target_mode='shadow')`）+ 影子就绪清单；双消费只记录/比对；**不建单、不推进组、不放行**；观察期由 Director 指定。
3. **SHADOW→ON（`mode=settled`）**：同时满足 `SPEC-0002 §7 矩阵 + 五方终态 + Reviewer 人工核对 + review-tracking 记录（协作闸门已退役，无命令可跑）` + Director 批准；`order.paid` 停止建单/推进组，只保留 BFF/旧消费者兼容；必须写 `order_paid_admission_retired_at` + `order_paid_subject_removal_date`（建议 ≤ ON+30d）。
4. **结算 worker**：`PAYMENT_SETTLEMENT_WORKER_ENABLED=true`；旧同步路径保留为人工兜底。
5. **退款准入**：`REFUND_ADMISSION_ENABLED=true`；旧退款路径只读/停写。
6. **`GRANTED` 收敛**：预检/人工分类完成 → 加 CHECK → 删除 `permitGranted` 常量与 `permitOpen` 的 `GRANTED` 分支；`permitOpen` 只把 `NONE` 视为可授权；`ensurePermitTx` 遇未知状态返回 `SETTLEMENT_CONFLICT`/`FULFILLMENT_NOT_ALLOWED` 并告警；`AuthorizeShipment`/`groupActions` 未知状态 fail-closed。
- 回滚：按 §6.4 的 Director 批准回滚执行（`settled→shadow/paid` 需批准与事件单），再关开关；任务保留可重放；不做 down migration；数据前向兼容；只改消费模式，不丢任务/退款申请，不重复建单/退款。
- 启动装配必须注册 settlement worker、refund worker、pricing-confirmation worker、zero-settlement worker、legacy expiry/compensation worker、release 请求 worker；漏注册视为未实现。
- **N5 启动配置**：装配时必须校验 `SETTLE_RELEASE_APPROVAL_ID` 与消费模式配对；approval-id 非空但未请求 `settled` → **启动失败（非零退出）**，不得默认 `paid`（§6.2）。
- 生产开关默认 false，直到 `SPEC-0002 §7 矩阵 + 五方终态 + Reviewer 人工核对 + review-tracking 记录（协作闸门已退役，无命令可跑）`。

### 12.4 旧消费者清单

`shop.order.paid.v1` 旧消费者需在灰度期核对：Fulfillment 自动建单、BFF/通知、BI。Fulfillment 必须按 D-27 切换到 `settled`（或保持 `paid` 兼容路径）；其他消费者可继续消费 `order.paid`，但**不得据此允许发货**。
`shop.order.refund_requested.v1`：TCC 生产者已移除（T1.5）；仅 `refundCancelledAfterConfirm` 仍产生；**除 `pay:<payment_no>:cancel` 命令载体外，消费只观察/告警（A-08）；支付级取消走该命令（T1.6）**；C-2 已关闭（§18.2）；移除日期由 Director release 记录设定。

### 12.5 schema 约束设计（C-05 输入，不是迁移文件）

> 本节是 **schema 约束设计**；C-05 由 Developer 在获批后落地并跑真实迁移。MySQL 8 DDL 非事务：迁移用 hardened runner 的 attempt/checksum 账本 + 备份。

#### 12.5.1 `refund_batches`

```sql
CREATE TABLE refund_batches (
  batch_id        VARCHAR(96) NOT NULL,             -- tcc:<payment_no>:<attempt_seq>
  source          VARCHAR(24) NOT NULL,             -- tcc | legacy | ...
  order_no        VARCHAR(64) NOT NULL,
  payment_no      VARCHAR(64) NOT NULL,
  attempt_seq     INT NOT NULL DEFAULT 1,
  state           VARCHAR(24) NOT NULL,             -- PRECHECKED/FROZEN/ADMITTING/ADMITTED/PAYING/DONE/PRECHECK_FAILED/MANUAL_HOLD/FAILED_PERMANENT
  captured_snapshot_cent BIGINT NOT NULL DEFAULT 0,
  refunded_snapshot_cent BIGINT NOT NULL DEFAULT 0,
  sum_group_cap_cent     BIGINT NOT NULL DEFAULT 0,
  remaining_captured_cent BIGINT NOT NULL DEFAULT 0,
  attempts        INT NOT NULL DEFAULT 0,
  lease_owner     VARCHAR(64) NOT NULL DEFAULT '',
  lease_until     DATETIME(3) NULL,
  last_error      VARCHAR(1024) NOT NULL DEFAULT '',
  version         BIGINT NOT NULL DEFAULT 1,
  created_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (batch_id),
  KEY idx_refund_batch_lookup (source, order_no, payment_no, state),
  KEY idx_refund_batch_state (state, updated_at),
  CONSTRAINT chk_refund_batch_state CHECK (state IN
    ('PRECHECKED','FROZEN','ADMITTING','ADMITTED','PAYING','DONE','PRECHECK_FAILED','MANUAL_HOLD','FAILED_PERMANENT'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- 同一 `(source, order_no, payment_no)` 允许存在多个 `PRECHECK_FAILED` 审计批次；**同一时刻只允许一个 active batch**（`FROZEN/ADMITTING/ADMITTED/PAYING`），由 Tx1 订单锁检查；`idx_refund_batch_lookup` **不是 UNIQUE**，唯一性不靠该索引。
- `PRECHECK_FAILED` 无 `refund_applications`，人工修正后创建新 attempt（`attempt_seq+1`，新 `batch_id`）；`MANUAL_HOLD/FAILED_PERMANENT` 已有 applications，重试时**复用同一 batch 与同一 `refund_id`**（state CAS 回 `PAYING/ADMITTING`），不得新建第二套退款标识。

#### 12.5.2 `refund_applications`

```sql
CREATE TABLE refund_applications (
  refund_id       VARCHAR(128) NOT NULL,
  batch_id        VARCHAR(96)  NULL,
  source          VARCHAR(24)  NOT NULL,            -- aftersale/tcc/system_shortage/system_zero/payment/legacy_expired
  order_no        VARCHAR(64)  NOT NULL,
  group_no        VARCHAR(64)  NULL,
  non_effective_capture TINYINT(1) NOT NULL DEFAULT 0, -- payment/legacy_expired=1；组级=0
  amount_cent     BIGINT       NOT NULL DEFAULT 0,
  goods_cent      BIGINT       NOT NULL DEFAULT 0,
  freight_cent    BIGINT       NOT NULL DEFAULT 0,
  state           VARCHAR(24)  NOT NULL DEFAULT 'PENDING',
  admission_result VARCHAR(24) NOT NULL DEFAULT '',  -- PRE_SHIP_REVOKED/POST_SHIP_ALLOWED/PENDING_REMOTE
  attempts        INT          NOT NULL DEFAULT 0,
  lease_owner     VARCHAR(64)  NOT NULL DEFAULT '',
  lease_until     DATETIME(3)  NULL,
  last_error      VARCHAR(1024) NOT NULL DEFAULT '',
  version         BIGINT       NOT NULL DEFAULT 1,
  created_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (refund_id),
  UNIQUE KEY uk_refund_app_batch_group (batch_id, group_no),
  KEY idx_refund_app_order_state (order_no, state),
  KEY idx_refund_app_source_state (source, state),
  CONSTRAINT chk_refund_app_state CHECK (state IN
    ('PENDING','PROCESSING','DONE','FAILED_PERMANENT','CANCELLED','REJECTED')),
  CONSTRAINT chk_refund_app_non_effective CHECK (non_effective_capture IN (0,1)),
  CONSTRAINT chk_refund_app_non_effective_source CHECK (
    (source IN ('payment','legacy_expired') AND non_effective_capture = 1)
    OR (source NOT IN ('payment','legacy_expired') AND non_effective_capture = 0)
  ),
  CONSTRAINT chk_refund_app_amount CHECK (amount_cent >= 0 AND goods_cent >= 0 AND freight_cent >= 0
    AND goods_cent + freight_cent = amount_cent)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- `group_no NULL` 只用于支付级；组级必须非空。`(batch_id, group_no)` 唯一在 `batch_id NULL` 时允许多行 NULL（支付级不受限），由 `refund_id` 唯一兜底。
- 支付级行（`group_no NULL`）必须 `goods_cent = amount_cent`、`freight_cent = 0`；支付级不参与组/行额度，只受 Payment captured 上限约束。
- `non_effective_capture`：支付级取消/历史 `EXPIRED_REFUND` 必须为 1，组级必须为 0；为 1 时 Order 投影不累加订单级 `captured/refunded`（INV-37）。
- 同一 `refund_id` 只有一条；`(batch_id, group_no)` 对组级唯一；`state`/`admission_result` 枚举见 §8。

#### 12.5.3 `order_refund_commitments` 变更 + 行表

```sql
ALTER TABLE order_refund_commitments
  ADD COLUMN goods_cent BIGINT NOT NULL DEFAULT 0 AFTER amount_cent,
  ADD COLUMN freight_cent BIGINT NOT NULL DEFAULT 0 AFTER goods_cent,
  ADD COLUMN composition_known TINYINT(1) NOT NULL DEFAULT 0 AFTER freight_cent,
  ADD COLUMN composition_source VARCHAR(16) NOT NULL DEFAULT 'legacy' AFTER composition_known;

-- 历史行：仅占额度、不归行；不得据此做行级部分退款
UPDATE order_refund_commitments
SET goods_cent = amount_cent, freight_cent = 0,
    composition_known = 0, composition_source = 'legacy'
WHERE composition_source = 'legacy';

CREATE TABLE order_refund_commitment_lines (
  refund_id     VARCHAR(128) NOT NULL,
  line_no       INT          NOT NULL,             -- 0=运费行；>0=商品行
  quantity      INT          NOT NULL DEFAULT 0,
  goods_cent    BIGINT       NOT NULL DEFAULT 0,
  freight_cent  BIGINT       NOT NULL DEFAULT 0,
  created_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (refund_id, line_no),
  CONSTRAINT chk_commitment_line_amount CHECK (goods_cent >= 0 AND freight_cent >= 0),
  CONSTRAINT chk_commitment_line_kind CHECK (
    (line_no = 0 AND goods_cent = 0) OR (line_no > 0 AND freight_cent = 0)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- 事务内校验：`SUM(goods_cent)=头.goods_cent`、`SUM(freight_cent)=头.freight_cent`、`头.amount=goods+freight`。
- 新承诺 `composition_known=1`；缺行/合计不符 → 拒绝（`REFUND_ADMISSION_CONFLICT`）。

#### 12.5.4 `refund_commitment_releases`

```sql
CREATE TABLE refund_commitment_releases (
  release_id      VARCHAR(96)  NOT NULL,
  refund_id       VARCHAR(128) NOT NULL,
  amount_cent     BIGINT       NOT NULL,
  authorization_id VARCHAR(96) NOT NULL,
  operator        VARCHAR(64)  NOT NULL,
  authorized_by   VARCHAR(64)  NOT NULL,
  reviewed_by     VARCHAR(64)  NOT NULL DEFAULT '',
  reason_code     VARCHAR(64)  NOT NULL,
  evidence_ref    VARCHAR(255) NOT NULL,
  authorized_at   DATETIME(3)  NOT NULL,
  reviewed_at     DATETIME(3)  NULL,
  expires_at      DATETIME(3)  NULL,
  released_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  version         BIGINT       NOT NULL DEFAULT 1,
  PRIMARY KEY (release_id),
  UNIQUE KEY uk_release_authorization (authorization_id),
  UNIQUE KEY uk_release_refund (refund_id),
  CONSTRAINT chk_release_amount CHECK (amount_cent > 0),
  CONSTRAINT chk_release_review CHECK (
    amount_cent < 100000 OR (reviewed_by <> '' AND reviewed_by <> authorized_by)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- `uk_release_refund` 保证每笔承诺最多一次释放；`uk_release_authorization` 防授权复用；金额必须等于承诺头金额，不存在部分释放。

#### 12.5.5 `order_refund_sequences`

```sql
CREATE TABLE order_refund_sequences (
  order_no   VARCHAR(64) NOT NULL,
  next_seq   BIGINT      NOT NULL DEFAULT 1,
  version    BIGINT      NOT NULL DEFAULT 1,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (order_no),
  CONSTRAINT chk_refund_seq CHECK (next_seq >= 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- 取号在 `orders` 行锁事务内：`INSERT ... ON DUPLICATE KEY UPDATE next_seq = LAST_INSERT_ID(next_seq + 1)`（或先 `SELECT ... FOR UPDATE` 再更新）；`refund_id=legacy:<order_no>:<seq>`；不得复用/回退。

#### 12.5.6 `settlement_shadow_diffs` / `settlement_switch_approvals`（Fulfillment DB）

```sql
CREATE TABLE settlement_shadow_diffs (
  order_no        VARCHAR(64) NOT NULL,
  payment_no      VARCHAR(64) NOT NULL,
  source_subject  VARCHAR(64) NOT NULL DEFAULT 'shop.order.settled.v1',
  schema_version  INT         NOT NULL DEFAULT 1,
  state           VARCHAR(16) NOT NULL DEFAULT 'MATCH',
  diff_fields     JSON        NULL,
  detail          JSON        NULL,
  first_seen_at   DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  last_seen_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  version         BIGINT      NOT NULL DEFAULT 1,
  PRIMARY KEY (order_no, payment_no),
  KEY idx_shadow_state (state, last_seen_at),
  CONSTRAINT chk_shadow_state CHECK (state IN ('MATCH','MISMATCH','ERROR'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE settlement_switch_approvals (
  approval_id    VARCHAR(96)  NOT NULL,
  target_mode    VARCHAR(16)  NOT NULL,             -- shadow | settled
  approved_by    VARCHAR(64)  NOT NULL,
  approved_at    DATETIME(3)  NOT NULL,
  release_range  VARCHAR(128) NOT NULL DEFAULT '',
  evidence_ref   VARCHAR(255) NOT NULL,
  expires_at     DATETIME(3)  NULL,                 -- settled 必填（CHECK）；shadow 可为 NULL
  order_paid_admission_retired_at DATETIME(3) NULL, -- settled/ON 时写；shadow 行 NULL
  order_paid_subject_removal_date DATE        NULL, -- settled/ON 时必填；移除日期
  status         VARCHAR(16)  NOT NULL DEFAULT 'APPROVED',
  created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (approval_id),
  KEY idx_switch_target (target_mode, status),
  CONSTRAINT chk_switch_mode CHECK (target_mode IN ('shadow','settled')),
  CONSTRAINT chk_switch_status CHECK (status IN ('APPROVED','REVOKED','EXPIRED')),
  CONSTRAINT chk_switch_settled_expiry CHECK (
    target_mode <> 'settled' OR expires_at IS NOT NULL
  ),
  CONSTRAINT chk_switch_removal CHECK (
    target_mode <> 'settled' OR (
      order_paid_admission_retired_at IS NOT NULL AND order_paid_subject_removal_date IS NOT NULL
    )
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- `settled` 行在 ON 批准时必须写 `order.paid_admission_retired_at` 与 `order.paid_subject_removal_date`（SQL 列 `order_paid_*`）；`shadow` 行两列为 NULL。`order.paid_subject_removal_date` 为 `DATE`（日粒度），与 §6.4 “建议 ≤ ON+30d”一致；超 30 天需 Director 书面批准。
- **`expires_at` 迁移/约束**：`settled` 行 **MUST NOT NULL**（`chk_switch_settled_expiry`），且启动时不得过期；`shadow` 行可为 NULL（表示不设到期；非 NULL 时启动校验须未过期）。C-05 迁移必须包含该 CHECK；缺 `expires_at` 的 `settled` 行不得写入。存量 `settled` 行若 `expires_at IS NULL`，必须由 Director 重新签发批准行（不得自动补默认值）；否则请求 `settled` 启动失败。
- 启动校验只读取 `target_mode IN ('shadow','settled')` 且 `status='APPROVED'` 的行，并按 §6.2 校验 `expires_at`/`release_range`；请求模式校验失败 → 启动失败（非零退出），不得回退 `paid`。
- **C-05 路径**：这两张表属 Fulfillment DB；`allowed_paths` 必须包含 `migrations/fulfillment/**`，否则 D-27 无落点（A-14）。

#### 12.5.7 `legacy_payment_resolution`

```sql
CREATE TABLE legacy_payment_resolution (
  order_no          VARCHAR(64)  NOT NULL,
  state             VARCHAR(24)  NOT NULL DEFAULT 'OPEN',
  candidate_count   INT          NOT NULL DEFAULT 0,
  pay_cent          BIGINT       NOT NULL DEFAULT 0,
  blocked_scope     VARCHAR(32)  NOT NULL DEFAULT 'UNSHIPPED_GROUPS_AND_REFUND',
  resolution_id     VARCHAR(96)  NULL,
  resolution_kind   VARCHAR(24)  NOT NULL DEFAULT '',
  resolved_payment_no VARCHAR(64) NOT NULL DEFAULT '',
  entered_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  expires_at        DATETIME(3)  NOT NULL,
  resolved_at       DATETIME(3)  NULL,
  closed_at         DATETIME(3)  NULL,
  lease_owner       VARCHAR(64)  NOT NULL DEFAULT '',
  lease_until       DATETIME(3)  NULL,
  operator          VARCHAR(64)  NOT NULL DEFAULT '',
  authorized_by     VARCHAR(64)  NOT NULL DEFAULT '',
  reason_code       VARCHAR(64)  NOT NULL DEFAULT '',
  evidence_ref      VARCHAR(255) NOT NULL DEFAULT '',
  version           BIGINT       NOT NULL DEFAULT 1,
  created_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (order_no),
  UNIQUE KEY uk_lpr_resolution (resolution_id),
  KEY idx_lpr_state_expiry (state, expires_at),
  KEY idx_lpr_lease (state, lease_until),
  CONSTRAINT chk_lpr_state CHECK (state IN
    ('OPEN','RESOLVING','COMPENSATING','RESOLVED','EXPIRED_REFUND','EXPIRED_ZERO','EXPIRED_FINANCE','REJECTED')),
  CONSTRAINT chk_lpr_kind CHECK (resolution_kind IN
    ('','MANUAL','EXPIRED_ZERO','EXPIRED_REFUND','EXPIRED_FINANCE','REJECTED')),
  CONSTRAINT chk_lpr_scope CHECK (blocked_scope IN
    ('UNSHIPPED_GROUPS_AND_REFUND','FINANCE_HOLD','REJECTED_HOLD','NONE')),
  CONSTRAINT chk_lpr_expiry CHECK (expires_at >= entered_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- `resolution_id` 仅终态写入；MySQL 唯一索引允许多个 NULL，故 `OPEN/RESOLVING/COMPENSATING` 不冲突。
- `settlement_payment_no` 维持 `NOT NULL DEFAULT ''`；写回只用 `= ''`（不引入 NULL 语义），如需兼容 NULL 用 `COALESCE(...,'')=''`。
- 状态迁移一律 `UPDATE ... WHERE order_no=? AND version=? AND state IN (...)`；0 行即败者，读回当前状态。

#### 12.5.8 `refunds` / `payment_refund_batches`（C-04）

```sql
-- 既有 refunds 表变更
ALTER TABLE refunds
  ADD COLUMN batch_id VARCHAR(96) NOT NULL DEFAULT '' AFTER payment_no;

-- 既有单列幂等键保留；删除仅按金额/reason 的 legacy 唯一键（允许同 payment 同金额/reason 的多个组）
ALTER TABLE refunds DROP INDEX uk_refunds_idem;
ALTER TABLE refunds
  ADD UNIQUE KEY uk_refunds_batch_item (payment_no, batch_id, idempotency_key);

CREATE TABLE payment_refund_batches (
  payment_no       VARCHAR(64) NOT NULL,
  batch_id         VARCHAR(96) NOT NULL,
  items_fingerprint CHAR(64)   NOT NULL,
  status           VARCHAR(32) NOT NULL,            -- 仅 REFUNDED / ALREADY_REFUNDED
  refunded_cent    BIGINT      NOT NULL,            -- Σ items（含既有），不是 intent 总退款额
  reason           VARCHAR(255) NOT NULL DEFAULT '',
  item_results     JSON        NOT NULL,            -- [{refund_id, refund_no, status, amount_cent}]
  created_at       DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at       DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (payment_no, batch_id),
  CONSTRAINT chk_prb_status CHECK (status IN ('REFUNDED','ALREADY_REFUNDED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- `refund_id` 即 `idempotency_key`，不另造列；既有 `uk_refunds_idempotency(payment_no, idempotency_key)` 保留不动；`refunds.batch_id=''` 表示 legacy/plain `Refund`，RefundBatch **不得改写既有行**，只有本批新写入行赋本批 `batch_id`。
- `item_results` 保存每项 `refund_id/refund_no/status/amount`；`ALREADY_REFUNDED` 项的 `refund_no` 必须来自既有 refund 事实；`status` 只允许 `REFUNDED/ALREADY_REFUNDED`（应用层校验；CHECK 非阻塞）。
- C-05 迁移路径：`migrations/payment/**`（C-04 记录为 `0012_refund_batch.sql`）；真实迁移测试使用 `migrations/`，不得内联 DDL 冒充。

#### 12.5.9 迁移执行与回滚

- 迁移路径：`migrations/order/**`（refund batch/application/commitment/release/sequence/legacy queue/permit CHECK/0021 zero CHECK）、`migrations/payment/**`（C-04 批退款 schema，§12.5.8）、`migrations/fulfillment/**`（shadow/approvals）。真实迁移测试使用 `migrations/`，不得内联 DDL 冒充。
- 回滚：代码回滚保留新表/列（旧代码忽略）；数据回滚不允许回退状态机，人工审计修正；开关回滚只改消费模式，不丢任务/退款申请，不重复建单/退款；不做 down migration。
- `order.settled` 双发保持不变；影子/切换不改变结算主链路。

---

## 13. 【假设与不确定性】（强制）

1. **DTM 确认/补偿顺序不保证**：Order 用同步 `CheckOrderReady` 兜底，不依赖分支顺序；若 DTM 出现非幂等补偿，停止灰度回 ADR。
2. **`CheckOrderReady` 是新 RPC**：需 proto/gen 与 Inventory 实现；未发布前 `PAYMENT_SETTLEMENT_WORKER_ENABLED=false`，不得用异步投影替代。
3. **P0 只支持单仓 G1**：多仓事件身份 v2 是 P1（另立 ADR-0005 Amendment）；多仓开关保持关闭。本 ADR 不依赖该 Amendment。
4. **Pricing 核销**：`pricing_confirmation_tasks` 持久重试；核销失败不退还支付、不重复退款；人工入口关闭。
5. **历史单**：按 §4.4；无法解析的退款阻断并人工处理；发货仍以许可/库存为准。
6. **渠道回调**：以 `(channel,event_id)` 去重；不假设无限重放；支付级退款可重试。
7. **MySQL 8 DDL 原子非事务**：迁移用 hardened runner 的 attempt/checksum 账本 + 备份。
8. **时钟**：业务时间以 MySQL `NOW(3)` 为准；定时器以 DB 时间；服务时钟偏差不影响 CAS。
9. **`QC08/R-2`（取消记账）**：取消仅 `PENDING_PAYMENT`；取消退款是**支付级**，不进入组额度/订单 refunded 投影；`order_captured=0`。T1.6 进一步把取消退款金额定为 `captured-refunded` 净额（§3.4.2）。
10. **`QC08/R-3`（历史无组单）**：按 G1；额度=订单剩余 captured；许可行无则初始化 `NONE`；释放按 SPEC-0001 v1.3 §6.6.1。
11. **`QC08/R-4`（TCC 运费与后续判责）**：TCC 系统补偿退全部未退运费；后续责任判责不得二次退同一运费，共享 `order_refund_commitments`/组台账。
12. **容量**：单节点 dev/UAT 不做 100k/1M RPS；不引入新队列/事务平台。
13. **`payment.refunded` 扩展字段兼容**：新增字段在灰度期 nullable/optional；旧消费者忽略；Order 只在 `non_effective_capture=false` 时使用订单级字段；回放按 `refund_id` 幂等，重复事件不得二次投影。
14. **历史人工队列**：无法解析生效支付/legacy 退款的历史单进入人工队列，目标 24h 内处理；期间退款阻断、发货仍以许可/库存为准。
15. **INV-43 适用范围（已确认）**：本 ADR 将 INV-43“先准入后打款”限定为**组级退款**；支付级退款（非生效捕获/超少付/取消）是 capture correction，不经过组许可。该范围已由 SPEC-0002 v1.7/§INV-43 确认（v1.9/v1.10 延续）；若人类要求所有退款都经组许可，需为支付级退款定义 synthetic admission，并回写 SPEC-0002。
16. **SPEC 基线**：本版以 **SPEC-0002 v1.10** 为靶；若后续 SPEC 修订改变 AC-62…68/INV-45…50，本 ADR 必须重新映射，不得直接实现。
17. **Payment 退款查询 API 缺口**：`GetRefundByKey` 落地前，惰性对账只能降级为“按订单列表 + 严格匹配 + 人工”，不得用 `refund_no` 猜 `refund_id`、不得猜“已退/未退”。
18. **`settlement_payment_no` 语义**：列 `NOT NULL DEFAULT ''`；Rev 1.0 曾写 NULL 兼容历史，本版统一用 `''`，避免 NULL 比较陷阱。
19. **`order.settled` 负载**：当前只发送 `groups []string`；本版不要求新增必填字段；影子比对依赖 `Order.GetOrder`，不是事件快照；若未来要事件自包含，必须 additive/版本化。
20. **影子期间不自动建单**：`shadow` 双消费只记录，意味着自动建单暂停；需 Director 在影子批准时确认运营可接受手工/既有运单。请求 `shadow`/`settled` 但批准行缺失/过期/非 `APPROVED`/`release_range` 不匹配 → **启动失败（非零退出）**，不得回退 `paid`；`paid` 仅为未请求模式时的默认/显式运行态；`settled→paid` 只经 §6.4 的 Director 批准回滚（SPEC-0002 INV-49）。
21. **`GRANTED` 数据**：代码无写入方；若生产存在历史行，按 §12.2 人工分类，无法自动推断；预检有行即阻断部署。
22. **`EXPIRED_FINANCE` 终态禁发**：与 AC-60“不得永久阻断”按“不得无限期 `OPEN`”解释；若人类要求最终可发货，必须回 PO 升版，不得由实现自行放行。
23. **D-22/D-23 已批准**：24h 分支、7 天财务关闭、`EXPIRED_REFUND` 不写 `CANCELLED` 是既定业务裁决；ADR 不重新裁决。
24. **多仓**：P0 单仓 G1 兼容；`warehouse_id` 仍由既有字段携带；本版不开启多仓。
25. **DTM/渠道**：不假设补偿顺序、渠道无限重放；以持久 batch/application/admission 为恢复入口。
26. **`allowed_paths` 缺口**：D-27 表在 Fulfillment DB；C-05 允许路径必须扩展 `migrations/fulfillment/**`，否则 C-05 无法落地。
27. **`order.paid` 移除日期**：R1 不切换、不填具体日期；ON 批准时必须写入移除日期（建议 ≤ ON+30d，超出需 Director 书面批准）；未写不得 ON。
28. **AC-63 零元精确条件**：SPEC-0002 v1.10 已含“`pay_cent=0` 且无任何已捕获 intent 才 zero；有捕获则退款/人工队列”；本版 §4.4.1/§5.2.5 与其一致；后续修订若删改该条件，必须回 PO 重映射。
29. **AfterSales 零元创建路径**：现有普通用户售后对“金额 0”有校验；零元订单售后必须由服务端识别 `pay_cent=0` 后走零元路径，不能用“用户手填 0 元”绕过正金额订单的售后额度；该识别属于实现，不改 Spec。
30. **`refund_no` 为空**：UI/对账不得把空 `refund_no` 当作“未退款”；以 `status=REFUNDED` + `refunded_cent=0` + Order `refund_applications(DONE)` 为准。
31. **0 元 commitment 例外**：零元不创建 commitment；S15“每个 refund_id 一条承诺”按“有金额占用”解释，0 元无额度可占；若 QC/PO 认为必须保留 0 元承诺，应新增明确状态/规则，不得由实现自行选择。
32. **已离仓事实依赖 Fulfillment**：Order 必须调 `ListShipments`（现有 RPC）；`GetShipmentByOrder` 多组会失败；`ListShipments` 分页/超时/`exception_state` 语义不明确 → fail-closed，不写 batch（T1.2/R-2）。
33. **T1.6 异步命令观测**：不扩 apperr/proto；稳定 `details.reason` + log + metric 是唯一可观测口径；Payment 不可用时 fail-closed，不猜 amount（T1.6/CB-4/CB-5）。
34. **C-04 RefundBatch（已裁决）**：C-04 于 2026-11-24 accepted，Rev 2.1 并入 §5.9/§12.5.8；TCC 批打款 MUST 用 `RefundBatch`（`Σ items == expected_total_cent`、`Σ NEW items <= remaining`、all-or-none），Order 预检 + 每笔前重读仍是必要防线；C-1 已于 2026-11-25 关闭（§18.1）。`fc295e5` 不构成实现 PASS。
35. **`order.refund_requested` 消费者（已裁决）**：A-08 收窄为只观察/告警；支付级取消以 T1.6 命令载体 `pay:<payment_no>:cancel` 为权威（§3.4.2/§3.7）；C-2 已于 2026-11-25 关闭（§18.2）。
36. **未并入的 Addenda**：T1.4b1（DRAFT）、T1.4b2（withdrawn）的内容不构成 Rev 2.3 规范；C-04 已于 2026-11-24 accepted 并随 Rev 2.1 并入（§0.7/§5.9/§12.5.8），其原文件仅作历史审计。

---

## 14. 反模式自省（强制）

- 禁止直接/TCC 各写一套 freeze/状态/取消补丁；必须走 `settleOrderTx`。
- 禁止用异步就绪投影决定 `PAID`；`CheckOrderReady` 不可用即阻断。
- 禁止用 Order 自己的退款投影做额度准入；Payment 权威 + Order 在途账本。
- 禁止把 `CAS=0/已存在/RPC 超时` 静默当成功。
- 禁止在 DB 事务内做跨服务调用；`CheckOrderReady`/`ListShipments`/Payment 快照在事务前，`CancelIfCreated`/Payment.Refund 在 Tx1 后。
- 禁止给每个 worker 加特殊恢复分支；统一任务 + lease/fencing + 人工重放。
- 禁止新增 Kafka/Temporal/Schema Registry；禁止改事件 subject 造成 Break Change。
- 禁止删除/重编号 AC 或把未执行测试写成 PASS。
- **不新增 `if-else` 状态补丁**：许可只有三值；`GRANTED` 直接移除，未知状态 fail-closed，不做多状态猜测〔Amd1 §12〕。
- **不把投影当准入**：commitment 只用于在途额度；最终上限始终 Payment `captured/refunded`；影子只告警。
- **不让 worker 静默释放**：释放必须人工授权 + 审计 + CAS/lease；`payment.refunded` 只做 `SETTLED`。
- **不重复退款**：`refund_id` 是唯一键；TCC batch 重跑跳过 `SETTLED`/已退组；历史 `EXPIRED_REFUND` 跳过已退 intent。
- **不用 `order.paid` 作准入**：`paid` 只是旧路径兼容；`shadow` 不建单；切换必须 release 门禁。
- **不引入新基础设施**：无 Kafka/Temporal/新 ORM；沿用 MySQL/outbox/DTM/NATS。
- **不做 down migration**：前向兼容；回滚靠代码/开关/重放。
- **零元不是新状态机分支**：用 `system_zero + amount=0 + DONE` 值组合表达；禁止新增 `DONE_ZERO`、伪造 `refund_no`、0 元 commitment 悬挂〔Zero §4〕。
- **不使用合成/伪造 Payment 退款事实**：零元不得有 Payment `refunded`；支付级退款不得进入组额度。
- **不得把未知当未退/已退**：`GetRefundByKey` 超时/不可用一律 fail-closed。
- **C-04/C-2 已裁决，不得回退**：TCC 组级批打款只用 §5.9 `RefundBatch`；支付级取消只用 `pay:<payment_no>:cancel` 命令；`order.refund_requested` 消费只观察/告警。不得以旧单方文本实现/验收。

---

## 15. Baseline 影响与 AC 映射

### 15.1 Baseline / 契约影响

| 基线/契约 | 影响 |
|---|---|
| INV-01 | 退款不写回 `orders.status`；`settlement_payment_no` 仅结算写 |
| INV-04 | 许可行仍是组级退款/发货唯一裁决点 |
| INV-05 | Payment 末端 `refunded+amount<=captured` 仍保留 |
| INV-06 | `CheckOrderReady` 强制前置 + `settleOrderTx`；组退款释放按 0001 v1.3 §6.6.1 |
| INV-07/14 | outbox 同事务；DTM barrier 不降级 |
| INV-21/24/26 | 行级额度=行实付−已退−在途承诺（额度双层：Payment 权威 + Order 在途账本，§4.3）；缺货 4h/48h 不变 |
| INV-30…44 | Rev 1.0 实现目标（不重编号、不改文本） |
| **INV-45…49** | **新增**（SPEC-0002 v1.8）：准入不可逆与释放纪律、历史解析收敛、行级/组成/责任、TCC Σ 预检、影子不具准入权威 |
| **INV-50** | **新增**（SPEC-0002 v1.9）：零元售后闭环不经过 Payment |
| **INV-43 范围** | 仅约束组级退款；支付级退款为 capture correction（§13.15，SPEC v1.7/v1.9 确认） |

### 15.2 AC 映射（Rev 1.0 + Amd1 + Zero）

AC 映射（Rev 1.0）：
- AC-39/40/41/44 → §2；AC-42/61 → §2/§11；AC-43/45 → §2.4/§3；AC-46 → §3.4；
- AC-47/48 → §3.1/§6；AC-49 → §1；AC-50/51/52/53/54/55 → §4/§5；
- AC-56 → §7；AC-57/58 → ADR-0005（不变）；AC-59 → §9 W3（四对乱序逐条落表）；AC-60 → §4.4/§12.2；
- AC-40 零元 → §3.5/§5.1/§5.2.5。

AC-62…66 实现映射〔Amd1 §2.3〕：

| AC | 关键实现点 | 失败语义 |
|---|---|---|
| AC-62 | 释放命令只接受人工授权；`RESERVED→RELEASED` CAS；permit 不恢复；`SETTLED` 优先收敛（§5.6） | `REFUND_RELEASE_NOT_AUTHORIZED` / `REFUND_COMMITMENT_CONFLICT` |
| AC-63 | 事务外解析；CAS 写回；0/多候选队列；24h 先 `COMPENSATING`（离开 OPEN）→ 先撤 permit/取消 CREATED 运单/INV-44 释放 → 持久化退款意图 → `Payment.Refund`；**Payment 成功后才 `EXPIRED_REFUND`**；恢复命令；零元精确条件（§4.4/§5.2.5） | `LEGACY_PAYMENT_UNRESOLVED` |
| AC-64 | 行级三层额度；商品/运费组成；责任枚举；168h 挂起；同组补退（§5.2.1/§5.6） | `REFUND_ADMISSION_CONFLICT` / `PARTIAL_REFUND_REQUIRES_FULL_GROUP` / `REFUND_RESPONSIBILITY_INVALID` |
| AC-65 | 事务外快照；Tx1 锁内复核；分状态 permit；batch 续跑；C-04 `RefundBatch` 原子批（`Σ NEW <= remaining`/all-or-none，§5.9）；部分打款只补未完成（§5.5.1） | `TCC_REFUND_SUM_MISMATCH` / `REFUND_ADMISSION_CONFLICT` / `INSUFFICIENT_REMAINING` / `MANUAL_HOLD` |
| AC-66 | 影子双消费、只记录；默认 `paid`；切换门禁；回滚（§6.2–§6.4） | 影子运行期失败只告警；请求 `shadow`/`settled` 缺/过期/不匹配批准 → **启动失败（非零），不回退 `paid`** |
| **AC-67** | 零元售后/取消/缺货不调用 Payment；`AdmitRefund` 零元短路同事务 `DONE`（§5.2.5） | 不创建 S19、不创建 commitment、不写订单级金额 |
| **AC-68** | AfterSales `REFUNDED/0/CONSUMED/refund_no=''`；展示 `DONE(zero_amount)`；仅 `pay_cent=0` 且无捕获（§5.2.5/§11） | 非 `DONE` 不得展示已完成；正金额仍走 AC-55 |

乱序逐对（AC-59）：

| 乱序 | 终态 |
|---|---|
| 退款投影先于捕获 | 拒绝/重试；捕获到达后按订单级字段投影 |
| 收货先于发货 | 拒绝/重试；运单从未发货则拒绝并告警 |
| 库存失败先于支付成功 | 结算时 `INVENTORY_NOT_READY`，退款 |
| 缺货解决先于上报 | 拒绝并告警；不得创建已解决案件 |

- 基线回归：既有 AC-14/21/25/27/31/32 的回归证据由 Reviewer 按 HEAD 登记；本 ADR 不写台账。
- 受影响 AC（AC-50/51/52/53/60/61）不重编号、不改文本，回归由 Reviewer 绑定。

---

## 16. Consequences

- **正面**：资金终态可恢复；四事实不混用；三类组退款统一；支付级退款独立；零元售后闭环无 Payment 伪造；影子上线可控；可按 AC 写故障注入。
- **代价**：新增表/worker/RPC；直接/TCC 双路径改造；灰度三态与批准行运维；行级组成/责任/168h 增加 AfterSales 与对账成本；历史单人工队列；D-27 影子期自动建单暂停。
- **运营**：新增告警/人工入口；对账口径变化；`GRANTED` 预检可能阻断部署；历史 commitment 需人工补齐组成；`order.paid` 移除需独立立项。
- **风险**：`CheckOrderReady`/`CancelIfCreated`/`ListShipments`/`GetRefundByKey` 未就绪时只能影子/关闭；不得跳过；C-04 已 accepted（§5.9）且 C-2 已关闭，但实现仍须按契约验收，不得据旧单方文本或 WIP 声称 PASS。
- **Rev 2.0 合并代价**：单文件变长；历史增量的“只增不改”边界由 merge map 维持；后续变更必须以 Rev 2.4 起的新修订落地，不得再写增量 addendum 叠加（除非另立新 ADR）。

---

## 17. 批准、前置条件与版本历史

### 17.1 批准记录（逐条事实）

| 日期 | 对象 | 批准/状态 | 证据 |
|---|---|---|---|
| 2026-11-23 | ADR-0007 Rev 1.0（`D-01…D-14`） | accepted（含 INV-43 组级适用范围；C-1…C-9 前置） | Director；`docs/review-tracking.md` 2026-11-23 节 |
| 2026-11-24 | Amendment 1（`A-01…A-14`，D-27…D-29） | accepted Rev 1.0 | Director；`docs/review-tracking.md:331-352`；QC-13/QC-14 |
| 2026-11-24 | Addendum 1 零元售后（`AZ-01…AZ-06`） | accepted Rev 1.0 | Director；QC-17/QC-18；SPEC-0002 v1.9；提交 `0f4fd2b` |
| 2026-11-24 | Addendum 2 T1.2/T1.3 rulings（`T1.2/R-1…R-3`） | accepted Rev 1.0（不批准 `12858d4` 实现） | Director；提交 `f972cb2` |
| 2026-11-24 | Addendum 3 T1.4a rulings（`T1.4a/R-A…R-C`） | accepted Rev 1.0（不批准 `6be5c5a` 实现） | Director；QC-21；提交 `d540ae1` |
| 2026-11-24 | C-04 RefundBatch 契约（`R-1…R-6`） | **accepted**（Director 2026-11-24） | `7421f4d` + `review-tracking.md:118-123`；K-1；并入 Rev 2.1 §0.7/§5.9/§12.5.8 |
| 2026-11-25 | T1.6 payment-level cancel Addendum | accepted Rev 1.0（T1.6 WIP 不等于实现 PASS） | QC-28 READY；提交 `cfa0b0d`；`review-tracking.md` 2026-11-25 节 |
| 2026-11-25 | T1.4b2 Addendum | **superseded / withdrawn** | 人类接受简化；`docs/lead/PAY-SETTLE-01/t1.4b2-design.md` Rev 1.3 |
| 2026-11-25 | Rev 2.0 合并版 | **accepted（merged version）** | Director；K-4；`docs/review-tracking.md` 2026-11-25 K-4 |
| 2026-11-25 | **Rev 2.1（C-04 合并 + C-2 + 退役命令前置替换）** | **accepted**（Director 2026-11-25） | Director；K-1/K-2/K-4/K-6；`docs/review-tracking.md` 2026-11-25；本文 §18.1–§18.2 |
| 2026-11-25 | **Rev 2.2（启动校验 BLOCKER 修复）** | **accepted**（Director 2026-11-25） | Reviewer BLOCKER [ARCHITECTURAL_FLAW]；SPEC-0002 INV-49（SPEC:133）；本文 §6.2/§6.4/§12.5.6/§18.4 |
| 2026-11-25 | **Rev 2.3（N5 修复 + SPEC v1.10 指针）** | **accepted**（Director 2026-11-25） | Director N5 裁决；`SETTLE_RELEASE_APPROVAL_ID` 无 `settled` 请求即 config error；本文 §6.2/§6.3/§7/§12.3/§18.4 |

- Status: **accepted（Rev 2.3，2026-11-25）**；Rev 2.3 只修复 N5 配置错误（approval-id 非空但无 `settled` 请求 → 启动失败，不默认 `paid`）并同步 SPEC 指针至 v1.10；**不改变**任何已批准业务规则。若正文与来源条款出现歧义，以来源条款的批准记录（本表）为准，由 Architect 以 Rev 2.4 修正。
- 批准前置（C-1…C-9，Rev 1.0 继承）：
  1. `CheckOrderReady` 的 proto/字段/owner 评审通过（Inventory + platform）。
  2. `CancelIfCreated` 的 proto/字段/owner 评审通过（Fulfillment）。
  3. `Payment.GetIntent`/`GetPaymentByOrder(intents[])`/`GetRefundByKey` 与 `payment.refunded` 扩展字段的契约评审通过（Payment + platform）。
  4. settlement/refund/pricing-confirmation/zero-settlement/legacy-expiry/release 请求 worker 的启动装配与开关清单确认。
  5. event registry 新增 `order.settled`/`order.refund_admitted` 的必填字段评审通过。
  6. 历史 `PAID` 解析与 legacy refund_id 的回填 dry-run 方案通过；人工队列 24h 目标。
  7. 迁移评审（新表/列/唯一键/金额类型/索引）通过。
  8. Inventory 核查：不得自行释放 `CONFIRMED` 预留；`HELD` TTL 释放保留。
  9. INV-43 适用范围人类确认（组级退款；支付级为 capture correction）——已由 SPEC-0002 v1.7/v1.9 确认。
- 批准（2026-11-23/24/25）：Developer 按本 ADR 实现；`AC-39…68` 逐条 AC-VERIFIED；Reviewer 出具有范围 verdict。未批准前：不得写支付/结算/退款/准入协议代码。

### 17.2 与 SPEC 的关系

- SPEC-0002 v1.10 的业务语义是唯一来源；ADR 只定义实现协议，不改变业务规则；两者冲突以 SPEC 为准并回 Architect 重映射。
- 本 ADR 未修改 SPEC（B-2 约束）；SPEC 的 `依据` 行仍列 Rev 1.0/Amendment 1/零元 Addendum，属 PO-owned 指针，需由 PO 在下次 SPEC 修订时同步。

### 17.3 版本历史

| Revision | 日期 | 变更 | 影响 |
|---|---|---|---|
| 1.0 | 2026-11-23 | 初始 accepted（D-01…D-14）；四事实分离、结算/准入/退款协议、五崩溃窗口 | Director 批准 |
| Amd1 1.0 | 2026-11-24 | Amendment 1：A-01…A-14、D-27…D-29、S15…S18、AC-62…66、schema/C-05 输入 | accepted；**已并入 Rev 2.0 并 superseded** |
| Addendum 1 1.0 | 2026-11-24 | 零元售后：AZ-01…AZ-06、S19、AdmitRefund 零元短路、C-05 `0021` | accepted；**已并入 Rev 2.0 并 superseded** |
| Addendum 2 1.0 | 2026-11-24 | T1.2/T1.3 裁决：R-1 审计 batch、R-2 EXCEPTION+shipped_at、R-3 PENDING 归 T1.6 | accepted；**已并入 Rev 2.0 并 superseded** |
| Addendum 3 1.0 | 2026-11-24 | T1.4a 裁决：R-A INV-44 owner、R-B POST commitment 时机、R-C 零元先取消条件 | accepted；**已并入 Rev 2.0 并 superseded** |
| T1.6 Addendum 1.0 | 2026-11-25 | 支付级取消分支、命令载体、`refundCancelledAfterConfirm` 持久命令、CB-1…CB-5 | accepted；**已并入 Rev 2.0 并 superseded**（A-08 收窄，C-2 已关闭） |
| C-04 rulings 1.0 / C-04 1.0 | 2026-11-24 | C-04 RefundBatch：`expected_total_cent`、`Σ NEW <= remaining`、首次 Reply 固定、batch ledger、错误映射、Order 预检边界 | C-04 accepted 2026-11-24（rulings 内容随 C-04 并入）；**已并入 Rev 2.1 并 superseded**（C-1 已关闭） |
| **2.0** | **2026-11-25** | **合并 Rev 1.0 + Amd1 + Addenda 1/2/3/T1.6 为单一正文**；新增 merge map、§18 未裁决冲突；supersede 5 个增量文件（Amd1 + Addendum 1/2/3 + T1.6）；T1.4b1/C-04 rulings/T1.4b2/C-04 文件/handoff 保留为非规范历史 | **不新增业务规则**；条款语义不变；已 accepted（K-4） |
| **2.1** | **2026-11-25** | **并入 C-04 RefundBatch（§0.7/§5.9/§12.5.8）；A-08 收窄为观察语义（C-2 关闭）；已退役协作闸门/已删发布命令的前置替换；§18 C-1/C-2 关闭** | **accepted 2026-11-25（Director）**；不新增业务规则；C-04 实现仍需 Reviewer/AC 证据 |
| **2.2** | **2026-11-25** | **修复 Reviewer BLOCKER：`shadow`/`settled` 启动校验失败即非零退出，禁止静默回 `paid`；`settled→paid` 仅 §6.4 Director 回滚；`settled` 批准行 `expires_at` NOT NULL（CHECK）；修正 §12.3 step 0 影子 task 冲突** | **accepted 2026-11-25（Director）**；满足 SPEC-0002 INV-49；不新增业务规则 |
| **2.3** | **2026-11-25** | **N5：`SETTLE_RELEASE_APPROVAL_ID` 非空但未请求 `settled` → 启动失败（非零），不得默认 `paid`；SPEC 指针更新为 v1.10** | **accepted 2026-11-25（Director）**；配置错误 fail-closed；不新增业务规则 |

### 17.4 仓库内指针同步状态（2026-11-25）

- **已同步（2026-11-25）**：SPEC-0002 的 `依据` 行指本 ADR **Rev 2.3**；`docs/specs/README.md` 的 SPEC-0002 索引指向 **v1.10 / ADR Rev 2.3**。
- 历史（修订前）：SPEC `依据` 行曾指 Rev 1.0/Amendment 1；README 曾链 Amendment 1/零元 Addendum。仅作审计记录，不再代表当前状态。
- 两者由 PO 维护；本 ADR 不代改 SPEC。

---

## 18. 冲突裁决与关闭记录（C-1/C-2 已于 2026-11-25 关闭）

> Rev 2.0 记录的 C-1/C-2 已由 Director 于 **2026-11-25** 裁决（记录于 `docs/review-tracking.md` 2026-11-25 legacy cleanup decisions 的 K-1/K-2/K-4/K-6），Rev 2.1 按裁决收敛正文并关闭两项。下列证据表保留原冲突记录供审计；关闭后不得再据任一方旧文本实施/验收。

### 18.1 C-1：C-04 `RefundBatch` 契约的批准状态互相矛盾 —— **CLOSED（2026-11-25，K-1）**

| 主张 | 证据（文件:行 / 提交 / 台账） |
|---|---|
| **C-04 已 accepted（2026-11-24）** | `0007-amd1-refundbatch-c04-draft.md:5`（Status accepted）、`:7`（Revision 1.0）、`:111`（R-6 accepted）、`:121`；提交 `7421f4d` "docs(adr): accept C-04 RefundBatch contract"；`docs/review-tracking.md:118-123`（Director accepted the C-04 RefundBatch contract）；T1.6 `:10` 与 T1.4b2 `:7` 的 Base 均写 "C-04 RefundBatch（accepted）" |
| **C-04 仍是 DRAFT / 未批准（历史记录，已作废）** | `0007-amd1-refundbatch-c04-rulings.md:5`（原 "Status: **DRAFT — 未批准；C-04 仍为 DRAFT**"；2026-11-25 已改为 superseded by Rev 2.1）、`:25`（R-6 要求 C-04 先纳入 R-1/R-2/R-4/R-5 + schema 再 accepted）、`:97`；提交 `f55afe3` |
| **B-2 合并输入（历史记录，已作废）** | 把 `0007-amd1-refundbatch-c04-draft.md` 归类为 "draft — do NOT merge as normative" |

**连带项（t14b1）**：`0007-amd1-addendum-t14b1-payment-sigma.md` 仍为 **DRAFT（未批准）**；其 R-1…R-5 不构成本 ADR 规范。C-04 的原子性由已 accepted 的 C-04 契约承载（§5.9），不由 t14b1 授权；t14b1 不得再被引用为依据。

**裁决与 Rev 2.1 处理（K-1）**：C-04 RefundBatch 为 **accepted（2026-11-24）**。Rev 2.1 将 R-1…R-6 并入 §0.7、§5.9、§12.5.8（state、Σ、首次 Reply 固定、错误映射、schema、事件），并在 §2.6/§5.5.1 落实 TCC 批打款；`c04-draft` 标记 accepted + superseded by Rev 2.1，`c04-rulings` 标记 superseded by Rev 2.1。C-1 关闭。

### 18.2 C-2：`order.refund_requested` 消费者能否调用 `Refund`（A-08 vs T1.6）—— **CLOSED（2026-11-25，K-2）**

| 主张 | 证据 |
|---|---|
| **Payment 消费者只观察、不再 `Refund`（A-08 原绝对措辞）** | Amendment 1 §4.6 / `A-08`：保留 subject 兼容；Payment 消费者改为只观察/告警，**不再 `Refund`**；新 TCC 不依赖该事件；移除日期由 Director 定 |
| **Payment 消费者执行 `pay:<payment_no>:cancel` 命令（即调用 Refund），其余 observe-only** | T1.6 §1.4/§3.3/§4.1：保留 subject 作为“当前命令载体”；`refund_id=pay:<payment_no>:cancel` → 执行命令；旧 cancel-after-confirm 映射执行；其他 TCC/未知 → observe-only；最终 observe-only/移除由 release 决定 |
| **人工裁决记录（支持 T1.6）** | `docs/review-tracking.md:31-40`（T1.5 re-scope）："Making the consumer observe-only now would drop that refund. Therefore … the remaining consumer stays until T1.6 adds the durable payment-level cancel path (`pay:<payment_no>:cancel`) and then becomes observe-only." |
| **原缺口（已裁决）** | 原 A-08 的绝对措辞（“不再 `Refund`”）与 T1.6 的命令执行语义在同一 subject、同一消费者上不一致；两份文件均为 accepted（2026-11-24 / 2026-11-25） |

**裁决与 Rev 2.1 处理（K-2）**：**T1.6 支付级取消命令载体 `pay:<payment_no>:cancel` 权威**，支付级取消由它执行（§3.4.2/§3.7/§5.4）；**A-08 收窄为只约束 `order.refund_requested` 的观察语义**（除该命令载体外，消费只观察/告警，不得据 TCC/售后退款语义执行 `Refund`），已在 §0.2 A-08、§1.2、§1.4、§3.7、§5.4、§12.4 收敛。C-2 关闭。

### 18.3 已按既有裁决处理、不构成冲突的事项

- **`T1.2/R-3` guard → T1.6**：T1.6 §4 明确“本 Addendum 替代其 PENDING+captured fail-closed guard”；属显式替代，不是冲突（正文 §3.4.2 已写明替代关系）。
- **`T1.4a/R-B` POST commitment 时机**：Amd1 §4.1.2 step 3 只描述 Tx1（“已离仓→仅写 application”）；Addendum 3 §3 明确 admission 阶段写 commitment，属**契约澄清**，两者可同时成立（正文 §5.5.1 已分列）。
- **`T1.2/R-1` `PRECHECK_FAILED` 审计 batch vs Plan “零写入”**：ADR 优先，属已裁决的 Plan 偏差（正文 §5.5.1 规则 8）；不涉及 ADR 内部冲突。
- **Rev 1.0 §4.4“无法解析的历史单仍可发货” vs SPEC-0002 v1.8 AC-63“0/多候选禁发”**：以 SPEC 为准（SPEC 优先；Amd1 §5.3 已映射为 `LEGACY_PAYMENT_UNRESOLVED` 禁发）；属已批准的 business 收窄，不是 ADR 内部冲突。正文 §4.4.1 已写明旧句被收窄。
- **T1.4b2（withdrawn）**：其 B1/B2/B3 不并入；B2（SYSTEM 缺货 `system_shortage`）在 `review-tracking.md:89` 记为 "closed"，但关闭依据不在任何 accepted ADR 文件中，本 ADR 不将其写成规范；P0 单组释放范围（`review-tracking.md:75-81`）属 Plan/设计决策，不在本 ADR 定义。
- **C-04 rulings（原 DRAFT，2026-11-25 随 C-04 accepted 并入 Rev 2.1）**：R-1…R-6 以 C-04 accepted 契约文本为准，已在 §0.7/§5.9/§12.5.8 落地（§18.1）。
- **handoff**：契约输入清单，非批准；其 §1.2/§1.3/§1.4 的输入已在 §2.5/§4.4.5/§6.2 体现。

### 18.4 合并清单与计数

| 文件 | 状态 | Rev 2.1 处理 |
|---|---|---|
| `0007-payment-settlement-admission.md` | 本文件 | **Rev 2.1** |
| `0007-payment-settlement-admission-amd1.md` | accepted（2026-11-24） | **merged → superseded** |
| `0007-amd1-addendum-zero-after-sale.md` | accepted（2026-11-24） | **merged → superseded** |
| `0007-amd1-addendum-t12-contract-rulings.md` | accepted（2026-11-24） | **merged → superseded** |
| `0007-amd1-addendum-t14a-contract-rulings.md` | accepted（2026-11-24） | **merged → superseded** |
| `0007-amd1-addendum-t16-payment-cancel.md` | accepted（2026-11-25） | **merged → superseded**（C-2 已关闭） |
| `0007-amd1-refundbatch-c04-rulings.md` | superseded（原 DRAFT；C-04 已 accepted 2026-11-24） | **merged → superseded by Rev 2.1** |
| `0007-amd1-refundbatch-c04-draft.md` | accepted（2026-11-24；原冲突已裁决） | **merged → superseded by Rev 2.1** |
| `0007-amd1-addendum-t14b1-payment-sigma.md` | DRAFT（未批准） | **not merged**；保留为历史 |
| `0007-amd1-addendum-t14b2-inv44-group.md` | superseded / withdrawn（2026-11-25） | **not merged**；保留为历史 |
| `0007-payment-settlement-admission-amd1-c04-c05-handoff.md` | handoff（非批准） | **not merged**；保留为历史 |

统计：**合并 7 个增量文件**（Amendment 1 + Addendum 1/2/3 + T1.6 + C-04 rulings/C-04）→ **7 个 superseded**；**未合并 2 个 ADR addendum**（T1.4b1 DRAFT、T1.4b2 withdrawn）+ 1 个 handoff（非规范）；**未裁决冲突 0 项**（C-1/C-2 已于 2026-11-25 关闭）。

**Rev 2.2 BLOCKER 修复记录（2026-11-25，Reviewer [ARCHITECTURAL_FLAW]）**：Rev 2.1 §6.2 规定请求 `settled` 时批准行缺失/过期/不匹配则“按 `paid` 运行并告警”。ON 之后若批准行 `expires_at` 到期，重启会静默回退 `order.paid` 准入，绕过 §6.4 的 Director 回滚并违反 **SPEC-0002 INV-49**（SPEC:133）。Rev 2.2 修复：① 请求 `settled`（含旧布尔 `true`）或 `shadow`，批准行缺失/过期/非 `APPROVED`/`range` 不匹配 → **启动失败（非零退出）**，绝不回退 `paid`；② `paid` 仅作为未请求模式时的默认/显式运行态；③ `settled→paid`/`settled→shadow` 只经 §6.4 的 Director 批准回滚，无自动路径；④ `settlement_switch_approvals.expires_at` 对 `target_mode='settled'` MUST NOT NULL（`chk_switch_settled_expiry`；shadow 可为 NULL，见 §12.5.6）；⑤ §12.3 step 0 的“写 settlement task/refund application”改为 D-27 记录-only。§6.2/§6.3/§6.4/§12.3/§12.5.6/§13.20 与 Addendum §3.2 已同步。

**Rev 2.3 N5 修复记录（2026-11-25，Director 批准）**：`SETTLE_RELEASE_APPROVAL_ID` 非空但未请求 `settled`（mode 未设置/空/`paid`/`shadow`，旧布尔非 `true`）是 **config error**；进程 MUST **启动失败（非零退出）**，不得静默默认 `paid`。approval-id 只能与显式 `settled` 请求配对；该规则已加入 §6.2 规则 1、§6.3、§7 跨切面注记、§12.3 启动装配与 Addendum §3.2。SPEC 指针已更新为 **SPEC-0002 v1.10**。
