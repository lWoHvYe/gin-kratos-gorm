```yaml
id: SPEC-0002
title: 支付结算与履约准入
status: approved         # draft | in-review | approved | superseded
version: 1.6
updated_at: 2026-11-23
owner: Product Owner
approved_by: Director
approved_at: 2026-11-23
domain: payment
supersedes: []
superseded_by: null
adr: []
```

# SPEC-0002 — 支付结算与履约准入（P0，工作草案）

| 项 | 值 |
|---|---|
| 版本 | v1.6（已批准；QC-08 `PASS WITH CONDITIONS`，条件 C-1/C-8/C-9 已由甲方裁决） |
| 日期 | 2026-11-23 |
| 状态 | **已批准**：Director 2026-11-23 批准；可交 Architect 出 ADR-0007 |
| 位置 | `docs/specs/`：批准时已由 Reviewer 同步 `ac-baseline.txt`/`ac-coverage.tsv`；AC-39…61 先以延期行登记 |
| 作者 | Product Owner |
| 下游 | System Architect（只接收本文件与 QC 报告，不接收口头补充） |
| 依据 | `AGENTS.md` §4；`docs/architecture-baseline.md` INV-01…16；`docs/specs/0001` v1.2；`docs/ext-review-1123-triage.md` §7.4；`docs/review-tracking.md` 2026-11-23 裁决；`docs/collab/qc-reports/SPEC-0002-QC-01.md`、`SPEC-0002-QC-02.md`、`SPEC-0002-QC-03.md`、`SPEC-0002-QC-04.md`、`SPEC-0002-QC-05.md`、`SPEC-0002-QC-06.md`、`SPEC-0002-QC-07.md` |
| 范围 | H1/H2 批准的业务口径：首次支付、结算、取消竞态、库存就绪、发货准入、退款统一准入、缺货、收货 |

> 本文件只定义业务边界、状态所有权、业务不变量与验收标准（EARS）。**不定义**表结构、事件名、RPC 形态、事务边界、重试算法或开关实现；这些由 Architect 在 ADR 中设计并接受本文件验收。

---

## 0. 一页决策摘要

1. **首次支付必须足额**：生效支付金额严格等于 `pay_cent`；同一订单只有一个生效支付；重复/超额/少额捕获一律不生效并全额退款；零元单免捕获。**本期不做部分支付**。
2. **钱一旦被捕获，系统自己负责把它送到终态**：要么结算并允许履约，要么自动退款。不得把“支付渠道会重放”“运维会手工补”当作唯一恢复手段。
3. **`PAID` 只声明进入时刻的确认事实**：订单已接受支付、分配快照已冻结、所有行预留已确认；它不声明当前可履约、不表示可发货、不承担退款态，也不是新状态字段。发货仍要求该组实时可履约、无未解决缺货、许可为 `NONE`。
4. **所有退款走同一个许可行裁决**：普通售后、TCC 补偿、SYSTEM 缺货退款共用同一准入；未发货组撤销未用许可，已授权未离仓组取消运单，已离仓组只退款。
5. **竞态有唯一胜者**：支付确认 vs 取消（仅 `PENDING_PAYMENT` 窗口）、发货 vs 退款（许可/运单行），必须能回答“谁赢了、输的一方怎么补偿”。
6. **用户看到的支付/退款状态以权威事实为准**：捕获但未 `PAID` 只显示“支付处理中”；退款完成以 Payment 退款事实为准，不以 Order 投影先到后到改变结论。

---

## 1. 范围与非目标

### 1.1 本期范围

- 首次支付足额、重复支付、零元单、超额/少额捕获。
- 捕获后的结算与进程崩溃恢复（含 DTM 未受理、回调重复、渠道不再重放）。
- 支付确认与订单取消的竞态裁决（仅未支付订单）。
- 结算前库存预留确认、`PAID` 前置条件、发货前的组级准入。
- 三类退款来源（普通售后、TCC 补偿、SYSTEM 缺货）的统一准入与恢复，含组退款后的预留释放/回库边界。
- 缺货 4h 通知、48h 默认退款、等待补货到期、补货成功、案件终态与案件序号。
- 收货事实来源与“未发货不可确认收货”。
- 历史 `PAID` 订单的展示兼容与收敛路径（不要求四事实回填）。

### 1.2 明确不做（Non-goals）

- 部分支付、多次付款、跨渠道分账；未来要做另立 Spec/ADR，禁止用现有字段隐式支持。
- 多币种、跨境、支付渠道新增。
- 多仓读模型/Catalog 投影（P1，另立 ADR）。
- 售后行台账、回库授权、worker fencing 的细节（归 SPEC-0001 与 ADR-0005）。
- AI 工具与 Mesh 权限（另立 AI ADR；本 Spec 不新增任何 AI 写权限）。

---

## 2. 状态所有权（State Ownership）

| 事实/状态 | 唯一事实来源 | 取值/含义 | 谁消费 | 禁止承担的隐含语义 |
|---|---|---|---|---|
| S1 支付捕获事实 | Payment | 订单级 `captured`（只计生效支付）；单 intent 的 `captured` | Order、AfterSales、对账 | 不表示订单已结算；不表示库存已确认；不表示可发货 |
| S2 支付退款事实 | Payment | 订单级 `refunded`（只计针对生效支付的退款）；单 intent 的 `refunded`；非生效捕获退款单独记账 | Order、AfterSales、用户展示、对账 | 不表示商品已回库；不表示缺货案件已解决 |
| S3 订单结算事实 | Order | `orders.status` 的 `PENDING_PAYMENT → PAID` 迁移；`PAID` 表示：生效支付已接受 + 分配快照冻结 + 所有行预留 `CONFIRMED`。**不新增状态字段** | Fulfillment、用户展示 | 不表示可发货；不声明当前可履约；不表示售后额度已占用；不表示缺货案件已解决 |
| S4 库存可履约事实 | Inventory | 按订单行的预留状态：`HELD / CONFIRMED / RELEASED`；`CONFIRMED` 才可履约；`RELEASED` 后该行不可发 | Order、Fulfillment | 不由订单状态或 Catalog 投影推导；`RELEASED` 不等于已退款 |
| S5 发货许可 | Order | 按履约组的许可行：`NONE / CONSUMED / REVOKED`（沿用 SPEC-0001 S5 与基线 §3.4） | Fulfillment | 不表示库存就绪；不表示运单已离仓；不得由 `orders.status` 单独推导 |
| S6 运单与签收证据 | 证据来源=承运商事件或用户确认；Fulfillment 落库并维护运单状态；Order 只消费投影 | 运单状态（`CREATED/SHIPPED/RECEIVED/CANCELLED`）、`shipped_at`（离仓时间）、`EXCEPTION` 标记；签收证据；receipt | Order、用户展示 | 不表示已回库；不表示已退款；Order 不得自行制造签收事实 |
| S7 缺货案件 | Order | `NOTIFIED / RESOLVED_WAIT / RESTOCKED / RESOLVED_REFUND`；`RESOLVED_WAIT` 带补货日期；`RESTOCKED` 表示补货到货可继续履约；`RESOLVED_REFUND` 为退款终态 | AfterSales、通知、用户展示 | 不直接打款；不代表退款已完成；`RESTOCKED` 不是退款 |
| S8 退款申请与任务 | 该申请的唯一发起方（AfterSales 或 Order；每个 `refund_id` 只有一个所有者） | 每条申请有全局唯一 `refund_id`、金额、组、任务状态；`refund_id` 在创建时确定并终身不变；Payment 只负责执行 | Order 准入、Payment 执行、运营 | 不是打款事实；不拥有准入结果（准入结果归 Order）；不得因重复申请生成第二条记录 |
| S9 组退款进度 | Order | `NONE → REFUNDING → REFUNDED`（沿用 SPEC-0001 S4）；已离仓组允许同组多次部分退款；未离仓组的退款受 AC-50/D-19 限制 | 用户展示、Fulfillment 读模型 | **不是禁发裁决点**；禁发看 S5 与 S4/S6/S7 |

补充约束：

- `orders.status` 只表达订单生命周期（`PENDING_PAYMENT/PAID/ALLOCATED/SHIPPED/COMPLETED/CANCELLED`）；退款事实只写 `refund_state`/`refund_status` 投影，不得写回生命周期（基线 INV-01、SPEC-0001 AC-25）。
- **新发货的禁发唯一裁决点是 S5 许可行**：退款准入在许可行上 CAS；S9 只是进度投影，不能单独作为禁发依据。
- **许可与运单回答不同问题**：S5 回答“还能不能发新货”，S6 的运单状态回答“这票货是否已经离仓”。因此 `CONSUMED` 不等于 `SHIPPED`；两者都必须在发货前/退款时被读取。
- `FulfillmentAllowed` 不是新状态字段，而是发货时刻的业务谓词：`PAID` ∧ 组内所有行 `CONFIRMED` ∧ 缺货案件不在 `NOTIFIED/RESOLVED_WAIT` ∧ S5=`NONE`。
- `OrderSettled` 与 `PAID` 同义；本文件不使用 `SETTLED` 作为独立字段或新状态值。
- **已授权未离仓窗口**（`S5=CONSUMED` 且运单仍 `CREATED`）：退款准入 SHALL 取消该运单并按“未发货”处理；Ship 与取消在运单行上 CAS，唯一胜者。若 Ship 先胜出，退款按已发货处理。
- **“离仓”判定**：已离仓 = 运单状态 `SHIPPED/RECEIVED`，或 `EXCEPTION` 且 `shipped_at` 非空；未离仓 = 无运单、`CREATED` 或 `CANCELLED`。该判定只用于退款准入，不改变许可状态机。
- **退款准入结果持久化**：Order 在许可行/准入记录上按 `refund_id` 持久化准入结果（`PRE_SHIP_REVOKED` 或 `POST_SHIP_ALLOWED`）；发起方的退款申请只保存自己的任务状态，通过 `refund_id` 读取 Order 的准入结果，不得双写同一事实。重试先读 Order 的结果；结果缺失才重跑准入。重试时若已处于目标状态（许可 `REVOKED`、运单 `CANCELLED`、结果已持久化），视为成功并返回同一结果，不得再改状态。
- **缺货案件期间预留**：`NOTIFIED/RESOLVED_WAIT` 期间该组预留保持 `CONFIRMED`，不因通知释放；`RESOLVED_REFUND` 后释放；`RESTOCKED` 后保持 `CONFIRMED`。

---

## 3. 业务不变量（INV-30…INV-44）

> 基线 INV-01…16 与 SPEC-0001 INV-17…29 继续有效；本节新增或收窄，冲突时以更严格者为准，并在 §11 说明。

| ID | 不变量 | 反面即违规 |
|---|---|---|
| INV-30 | **首次支付足额**：生效支付金额严格等于 `pay_cent`；零元单 `pay_cent=0` 免捕获；不生效的捕获必须全额退款 | 少付/超付被接受并推进订单 |
| INV-31 | **单一生效支付**：同一订单最多一个生效支付；`PAID` 的 CAS 唯一胜者即生效支付；其他成功捕获一律退款 | 两笔支付都计入订单 |
| INV-32 | **捕获后系统自恢复**：捕获事实一旦提交，系统必须自行推进到终态（`PAID` 或自动退款），不得依赖渠道/人工重放作为唯一恢复 | 钱已扣、订单取消、无人退款 |
| INV-33 | **结算幂等与冲突裁决**：同一 `payment_no` 的重复确认幂等；订单已不是 `PENDING_PAYMENT` 或已被其他支付身份结算时，不得触发任何支付后副作用 | CAS 失败仍确认库存/核销优惠 |
| INV-34 | **`PAID` 前置条件**：只有“生效支付（或零元）+ 分配冻结 + 所有行预留 `CONFIRMED`”才能进入 `PAID`；明确不足或超过 D-11 等待上限仍无法确认的，不得 `PAID`，已捕获资金全额退款、订单 `CANCELLED` | `PAID` 仍存在 `HELD` 预留；确认失败仍展示已支付 |
| INV-35 | **四事实分离**：`PaymentCaptured / OrderSettled(=PAID) / InventoryReady / FulfillmentAllowed` 是不同事实；任何 API/页面/事件不得用单一状态同时表达三者 | 用 `PAID` 推导可发货 |
| INV-36 | **退款统一准入**：普通售后、TCC 补偿、SYSTEM 缺货退款必须先在 S5 许可行上裁决；未发货组 `NONE→REVOKED`，已授权未离仓组取消运单，已离仓组保持 `CONSUMED`；同一 `refund_id` 重复准入幂等 | 某入口绕过许可行；两个字段各自裁决禁发 |
| INV-37 | **资金口径唯一**：订单级 `captured` 只计生效支付；订单级 `refunded` 只计针对生效支付的退款；`FULL/REFUNDED` 判定只用这两个订单级值；非生效捕获退款单独记账 | 用单 intent `captured` 对比整单 `refunded`；把重复支付退款计入订单退款 |
| INV-38 | **组级隔离**：单组退款/缺货只阻断该组并撤销该组未用许可/未离仓运单；其他组维持原许可；整单退款才阻断所有未发货组 | 一组退款锁死整单 |
| INV-39 | **取消/确认唯一胜者**：仅 `PENDING_PAYMENT` 订单可取消；取消与支付确认在 `orders.status` 上 CAS，唯一胜者；败者必须退款或释放，不得留下支付后副作用 | 已支付订单被“取消”；两边都成功 |
| INV-40 | **缺货不静默与终态**：支付后实物缺货必须在 4h 内通知、48h 未选默认退款；等待补货必须给日期，到期未补货转退款；补货到货（含用户未选择前）转 `RESTOCKED` 并恢复履约；`RESTOCKED/RESOLVED_REFUND` 为终态，同一案件不得被重复上报重开；同一 `(order_no, group_no)` 同时只有一个活动案件，新的缺货事件使用新 `case_seq`；已退款后的迟到补货不得恢复发货 | 缺货静默、案件被重开、等待补货无到期动作、补货到货仍禁发、退款后补货又发货 |
| INV-41 | **收货来源与守卫**：签收/收货事实只能来自承运商签收事件或用户确认；`CREATED/CANCELLED` 运单不得确认收货；已确认后的争议不回滚收货事实（SPEC-0001 P-07） | `CREATED` 可确认收货；争议回滚收货 |
| INV-42 | **退款申请不丢失**：任何退款来源必须先留下以全局唯一 `refund_id` 标识的可恢复申请/任务；执行失败可重试；同一 `refund_id` 重复提交幂等；具体驱动机制由 ADR 定义 | 先打款后记录；失败后申请消失；同一申请出现两条记录 |
| INV-43 | **先准入后打款**：退款准入未得到 `PRE_SHIP_REVOKED` 或 `POST_SHIP_ALLOWED` 前，不得调用支付退款；准入冲突必须阻断并告警 | 未准入先退款；冲突被静默忽略 |
| INV-44 | **组退款后的库存处置有主**：未离仓组的预留由 Inventory 释放（`HELD` 直接释放，`CONFIRMED` 走基线 `CancelConfirm` 的补偿释放）；已离仓组的退货回库按 SPEC-0001 INV-03；释放/回库失败必须重试并告警；释放后 `available ≤ on_hand` 且行数量守恒（SPEC-0001 INV-17），不得超卖或重复退款 | 退款后预留无人处理；释放失败静默；释放后超卖 |

---

## 4. 业务规则与验收标准（EARS）

> 语法遵循 `AGENTS.md` 附录 A。每条 AC 至少一个可执行测试；资金、库存、竞态必须有并发或故障注入用例，且至少一个异常分支。

### 4.1 首次支付、重复支付与零元单

**AC-39** 足额校验（金额维度）
- WHEN 订单创建支付意图或确认支付，THE SYSTEM SHALL 校验金额严格等于 `pay_cent`。
- IF 请求金额 ≠ `pay_cent`，THEN THE SYSTEM SHALL 拒绝发起渠道支付并返回 `PAYMENT_AMOUNT_MISMATCH`。
- IF 渠道侧仍成功捕获了 ≠ `pay_cent` 的金额，THEN THE SYSTEM SHALL 不把该捕获计入订单，并对其全额退款；向调用方返回 `PAYMENT_AMOUNT_MISMATCH`。

**AC-40** 零元单
- WHEN `pay_cent=0`，THE SYSTEM SHALL 不创建捕获事实，并按 AC-47 的预留确认后进入 `PAID`。
- IF 零元单收到任何正金额捕获，THEN THE SYSTEM SHALL 全额退款，不改变订单金额。
- WHEN 零元单发生取消/缺货/售后，THE SYSTEM SHALL 只释放或回库，不调用 Payment；退款申请以 0 元标记 `DONE(zero_amount)`，售后额度为 0。

**AC-41** 重复支付与生效支付（身份维度）
- WHEN 同一订单出现第二个成功捕获（不同 `payment_no`）且金额正确，THE SYSTEM SHALL 保持唯一生效支付，并对非生效捕获全额退款；返回 `DUPLICATE_PAYMENT`。
- WHEN 同一 `payment_no` 的回调重复到达，THE SYSTEM SHALL 幂等成功，不退款、不重复副作用。
- IF 多个捕获并发，THEN 由 `PENDING_PAYMENT → PAID` 的 CAS 唯一胜者决定生效支付；非胜者必须退款。
- 金额不正确的捕获按 AC-39 处理，不按本条处理。

**AC-42** 用户可见状态
- WHILE 捕获成功但订单仍为 `PENDING_PAYMENT`，THE SYSTEM SHALL 只展示“支付处理中”，不得展示“已支付”或“可发货”。
- WHEN 订单为 `PAID`，THE SYSTEM SHALL 展示“已支付”，但仍不得展示“可发货”，除非该组满足 AC-48。
- WHEN 某组已撤销许可、运单已取消或进入退款，THE SYSTEM SHALL 对该组展示“退款处理中/已退款”，其他组不受影响。

### 4.2 结算与恢复

**AC-43** 捕获后自恢复
- WHEN 捕获事实已提交，THE SYSTEM SHALL 持久化结算意图并自行推进结算。
- IF 进程在“捕获已提交、结算未完成”之间崩溃，THEN 重启后 SHALL 从持久意图继续，不得依赖渠道重放。
- IF 订单已 `CANCELLED`，THEN THE SYSTEM SHALL 全额退款；若预留为 `HELD` 则释放，若已 `CONFIRMED` 则发起补偿释放，并按 INV-44 收敛，订单保持取消终态。

**AC-44** 回调幂等与身份冲突
- WHEN 同一 `payment_no` 的回调重复到达，THE SYSTEM SHALL 只产生一次结算副作用。
- IF 回调的 `payment_no` 已存在但金额/订单不一致，THEN THE SYSTEM SHALL 不按该回调改写订单事实；若该回调对应渠道真实入账，则按 AC-39 全额退款，并告警。
- IF 本地持久化失败，THEN THE SYSTEM SHALL 返回可重试错误，不得向渠道 ack；只有本地事实与 outbox 同事务提交后才 ack。
- IF 订单已被其他 `payment_no` 结算，THEN 本回调 SHALL 按 AC-41 退款，不得推进订单。

**AC-45** 结算冲突
- WHEN 结算时订单已不是 `PENDING_PAYMENT`，THE SYSTEM SHALL 重读权威状态后裁决，不得把“更新失败”当作幂等成功。
- IF 订单已 `PAID` 且生效支付就是本 `payment_no`，THEN SHALL 幂等成功；否则 SHALL 不触发库存/优惠/发货副作用，并退款。

### 4.3 取消竞态

**AC-46** 取消窗口与唯一胜者
- WHEN 订单为 `PENDING_PAYMENT`，THE SYSTEM SHALL 允许取消；取消与支付确认在 `orders.status` 上 CAS，唯一胜者。
- IF 取消胜出，THEN 支付确认 SHALL 失败并返回 `ORDER_CANCELLED`，已捕获资金全额退款，库存与优惠不得确认。
- IF 支付胜出，THEN 取消 SHALL 失败并返回 `ORDER_ALREADY_PAID`；用户只能进入售后退款，不得把订单置回未支付。
- IF 订单已 `PAID`，THE SYSTEM SHALL 不提供取消动作。

### 4.4 结算前库存与发货准入

**AC-47** `PAID` 前预留确认与归因
- WHEN 结算执行 `PENDING_PAYMENT → PAID`，THE SYSTEM SHALL 保证所有订单行预留 `CONFIRMED` 且分配快照已冻结。
- IF Inventory 明确返回不可补足，THEN THE SYSTEM SHALL 立即终止结算：不得 `PAID`，全额退款、订单 `CANCELLED`，返回 `INVENTORY_NOT_READY`，并告知用户“库存不足，已退款”。
- IF Inventory 技术失败/超时未决，THEN THE SYSTEM SHALL 持续重试并保持“支付处理中”；超过 D-11 的等待上限仍无终态，THEN 按“明确失败”处理（不得 `PAID`、全额退款、`CANCELLED`、告警）。
- IF 确认成功后再发现实物缺货，THEN 按 AC-56 缺货流程处理，不影响已发生的 `PAID` 事实。
- AC-61 的 30 分钟只用于用户/运营可见性，不改变本条的终态判定。

**AC-48** 发货准入
- WHEN Fulfillment 为新运单发货前申请许可，THE SYSTEM SHALL 校验：订单 `PAID`、组内所有行预留 `CONFIRMED`、缺货案件不在 `NOTIFIED/RESOLVED_WAIT`、许可为 `NONE`。
- IF 任一条件不满足，THEN THE SYSTEM SHALL 返回 `FULFILLMENT_NOT_ALLOWED`。
- IF 许可为 `CONSUMED` 且同一 `shipment_no`，THEN Order SHALL 幂等返回已授权；IF 不同 `shipment_no`，THEN 返回 `FULFILLMENT_NOT_ALLOWED`。
- IF 许可为 `REVOKED`，THEN Order SHALL 返回 `FULFILLMENT_NOT_ALLOWED`。
- WHEN Fulfillment 执行 Ship，THE SYSTEM SHALL 先读运单状态：`CANCELLED` → `SHIPMENT_CANCELLED`；`SHIPPED/RECEIVED` → 幂等成功；`CREATED` → 继续。
- IF 运单已 `CANCELLED`，THEN THE SYSTEM SHALL 拒绝 Ship，不得复活运单。

**AC-49** 四事实展示
- WHEN API/页面输出订单状态，THE SYSTEM SHALL 分别输出支付、结算、库存可履约、发货许可四类事实；不得用单一状态替代。

### 4.5 退款统一准入

**AC-50** 普通售后退款
- WHEN 售后发起退款，THE SYSTEM SHALL 先以 `refund_id` 留下可恢复的退款申请，按下列额度口径计算该组剩余可退上限，再在许可行上申请准入。
- 额度口径：`组剩余商品额度 = Σ 行剩余可退额`（0001 INV-24 行层）；`组剩余运费额度 = 该组尚未退的 freight_share`（普通售后按 0001 §6.5 本次责任是否可退；TCC 系统补偿按“全部未退运费”计）；`组剩余可退上限 = 商品 + 运费`；`整组退款金额 = 商品剩余 + 本次可退运费`。
- IF 本次金额 > 组剩余可退上限，OR 该组已全额退款（存在至少一条该组 `DONE` 退款申请且组剩余可退上限=0），THEN THE SYSTEM SHALL 返回 `REFUND_ADMISSION_CONFLICT`；不得改动许可/运单/打款。零元单的首次 0 元申请不属于“已全额退款”。
- IF 该组未离仓 AND 本次金额 < 整组退款金额，THEN THE SYSTEM SHALL 返回 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`；不得改动许可/运单/打款，用户必须改为整组退款（D-19）。
- IF 该组未离仓 AND 本次金额 = 整组退款金额，THEN THE SYSTEM SHALL 按下列唯一分支处理：
  - 许可 `NONE` 且运单为无/`CREATED`/`CANCELLED`：置 `REVOKED`，返回 `PRE_SHIP_REVOKED`，并通知 Fulfillment 取消 `CREATED` 运单（无运单/已取消时幂等成功）。
  - 许可 `REVOKED`：幂等返回 `PRE_SHIP_REVOKED`。
  - 许可 `CONSUMED` 且运单 `CREATED`：取消该运单，返回 `PRE_SHIP_REVOKED`；Ship 与取消在运单行 CAS，若 Ship 先胜出（运单已 `SHIPPED`）则按已离仓返回 `POST_SHIP_ALLOWED`。
  - 许可 `CONSUMED` 且运单 `CANCELLED`：返回 `PRE_SHIP_REVOKED`（这是准入结果，不是许可状态；许可保持 `CONSUMED`，但该组无运单可发，AC-48 会拒绝新运单）。
  - 许可 `CONSUMED` 且无运单：返回 `REFUND_ADMISSION_CONFLICT` 并告警（异常状态，需人工核对）。
- IF 许可 `NONE→REVOKED` 的 CAS 败给并发 Ship（许可已 `CONSUMED`），THEN 重读并收敛：运单 `SHIPPED/RECEIVED` → `POST_SHIP_ALLOWED`；运单 `CREATED` → 取消运单并 `PRE_SHIP_REVOKED`；运单 `CANCELLED` → `PRE_SHIP_REVOKED`；其他 → `REFUND_ADMISSION_CONFLICT`。
- IF 该组已离仓（运单 `SHIPPED/RECEIVED`，或 `EXCEPTION` 且有离仓时间），THEN THE SYSTEM SHALL 返回 `POST_SHIP_ALLOWED`，退款继续，不得回滚发货/收货事实。
- IF 该组已离仓且存在新的 `refund_id`、仍有剩余可退额度，THEN THE SYSTEM SHALL 允许后续部分退款并返回 `POST_SHIP_ALLOWED`。
- IF 同一 `refund_id` 重复申请，THEN THE SYSTEM SHALL 先读取已持久化的准入结果并返回同一结果；结果缺失时重跑准入并收敛到同一结果。
- IF 准入未成功，THEN THE SYSTEM SHALL 不得调用支付退款，申请保持可重试。
- WHEN 计算退款金额，THE SYSTEM SHALL 沿用 SPEC-0001 §6.5.1/§6.5.2；行层沿用 0001 INV-24 的“行剩余 + 订单 captured 剩余”双层校验；组层使用上文“商品剩余 + 可退运费”上限；部分组只退该组 `freight_share`，运费责任按已批准判责口径。
- WHEN 该组未离仓且准入成功，THE SYSTEM SHALL 在许可 `REVOKED` 或运单 `CANCELLED` 生效后，才按 INV-44 释放该组预留；释放失败 SHALL 重试并告警，不得允许该组再发货。
- IF 重试时许可 CAS 已成功但运单取消失败/未完成，THEN THE SYSTEM SHALL 在运单行 CAS 重试取消（幂等），直到许可与运单一致；不得只重跑许可 CAS 就返回成功。

**AC-51** TCC 补偿退款（订单级 → 组级扇出）
- WHEN 支付确认失败/补偿需要退款，THE SYSTEM SHALL 为订单的每个履约组创建组级 `refund_id = tcc:<payment_no>:<group_no>`，并逐组申请 AC-50 的准入；无履约组的历史单按 G1 处理：金额=订单剩余可退，许可=G1，释放按 §6.6.1。
- **TCC 补偿视为系统责任**：每个活动组的退款金额 = 该组“系统责任上限”（商品剩余 + 全部未退 `freight_share`），运费不按用户责任扣减；不做最大余数分摊。
- IF 订单仍为 `PENDING_PAYMENT` 且已捕获，THEN SHALL 先通过 `orders.status` 的 CAS 进入 `CANCELLED`（与 AC-46 同一裁决点）；CAS 失败（已被支付确认推进）则订单为 `PAID`，继续按组准入。
- 定义“活动组”：组剩余可退上限 > 0 且未全额退款的组；活动组才需要本次准入与打款。
- IF 任一活动组未取得准入结果，THEN THE SYSTEM SHALL 不得打款，整个补偿保持可重试；不得只对部分组退款后放行其余组。已全额退款/剩余为 0 的组不阻塞整单。
- IF 某组已离仓（`SHIPPED/RECEIVED`，或 `EXCEPTION` 且 `shipped_at` 非空），THEN 该组按 `POST_SHIP_ALLOWED` 退款，不得取消运单；其他未离仓组按 `PRE_SHIP_REVOKED` 撤销/取消。
- IF 某组已全额退款或剩余可退上限=0，THEN 跳过该组；IF 所有组都已全额退款/剩余为 0，THEN 补偿幂等成功，不再调用 Payment。
- IF 历史订单不存在许可行，THEN Order SHALL 为 G1 初始化 `NONE` 许可行后再走本流程。
- WHEN 所有活动组准入完成，THE SYSTEM SHALL 按每组“系统责任上限”逐组打款；Σ 活动组系统责任上限必须等于订单剩余 `captured`。IF 两者不等，THEN THE SYSTEM SHALL 阻断本次补偿、告警并转人工裁决；不得部分打款，也不得把差额标成“待退款”继续放行。
- IF 某活动组已部分退款，THEN 本次只退该组剩余部分（商品剩余 + 未退运费）；不得按原始组份额重复退款。
- IF 准入未成功或打款失败，THEN THE SYSTEM SHALL 保留组级申请与任务，按 AC-50 重试；不得把订单标记为已退款。

**AC-52** SYSTEM 缺货退款
- WHEN 缺货决定退款，THE SYSTEM SHALL 以 `refund_id` 留下申请，走同一许可行准入，并通知 Fulfillment 取消该组未发货运单。
- IF 该组已离仓（`SHIPPED/RECEIVED`，或 `EXCEPTION` 且 `shipped_at` 非空），THEN THE SYSTEM SHALL 只退款，不得取消运单或回滚履约事实。
- WHEN 计算该组退款金额，THE SYSTEM SHALL 沿用 SPEC-0001 §6.5.1/§6.5.2：部分组只退该组 `freight_share`，支付后缺货责任落 `MERCHANT_WAREHOUSE`。

**AC-53** 资金口径
- WHEN 判定整单是否全退，THE SYSTEM SHALL 只比较订单级 `captured` 与订单级 `refunded`；订单级 `captured` 在 `PAID` 后才等于生效支付金额，`PAID` 前的捕获只用于退款判断，不计入订单额度。
- IF 存在非生效捕获，THEN 其退款 SHALL 单独记账，不计入订单 `captured/refunded`。
- IF 同一订单存在多个 intent，THEN THE SYSTEM SHALL 不得用单 intent 金额做整单判定。

**AC-54** 组级隔离
- WHEN 单组发生退款或缺货，THE SYSTEM SHALL 只阻断该组并取消该组未离仓运单；其他组维持原许可。
- IF 整单全额退款，THEN THE SYSTEM SHALL 阻断所有未发货组并取消其未离仓运单。

**AC-55** 退款完成口径
- WHEN 用户查看退款结果，THE SYSTEM SHALL 以 Payment 退款事实为准；Order 投影延迟只能显示“退款处理中”。
- IF 退款执行超时未知，THEN THE SYSTEM SHALL 保持可重试并展示处理中，不得显示“已退款”。

### 4.6 缺货

**AC-56** 缺货通知、默认、补货与终态
- WHEN 支付后实物缺货被确认，THE SYSTEM SHALL 在 4 小时内通知用户，并提供“等待补货（给日期）/退款”二选一。
- WHEN 通知发出后 48 小时用户未选择，THE SYSTEM SHALL 默认该组退款（fail-safe，不扣用户）。
- WHEN 用户选择等待补货，THE SYSTEM SHALL 记录补货日期；IF 到期仍未补货，THEN SHALL 再次通知并转退款。
- WHEN 仓库确认补货到货，THE SYSTEM SHALL 将案件置 `RESTOCKED`，解除该组禁发并恢复履约；不得退款。
- WHEN 仓库在用户选择前确认补货到货，THE SYSTEM SHALL 将 `NOTIFIED` 直接置 `RESTOCKED`，解除禁发并恢复履约，不再要求用户选择。
- IF 案件已 `RESTOCKED/RESOLVED_REFUND`，THEN 同一 `event_id` 的重复上报 SHALL 返回 `SHORTAGE_CASE_CLOSED`；新的缺货事件（新 `event_id`）SHALL 由 Order 分配新 `case_seq` 建新案件，不得覆盖旧案。
- IF 案件已 `RESOLVED_REFUND` 后收到补货到货，THEN THE SYSTEM SHALL 拒绝该补货事件并告警（`SHORTAGE_CASE_CLOSED`），不得恢复发货、不得新建案件。
- IF 用户选择、48h 定时器、补货到货并发，THEN THE SYSTEM SHALL 用案件版本 CAS 只接受一个迁移；相同选择重复提交 SHALL 幂等返回当前案件；活动案件内的重复 `event_id` SHALL 幂等返回当前案件；终态后重复 SHALL 返回 `SHORTAGE_CASE_CLOSED`。

### 4.7 收货

**AC-57** 未发货不可确认收货
- IF 运单状态为 `CREATED` 或 `CANCELLED`，THEN THE SYSTEM SHALL 拒绝用户确认收货，返回 `SHIPMENT_NOT_SHIPPED`。
- WHEN 用户已确认或承运商已签收，THE SYSTEM SHALL 只允许一次收货事实；重复请求 SHALL 幂等返回已收货结果。
- IF 运单有未解决争议，THEN 用户确认 SHALL 返回 `RECEIPT_DISPUTE_OPEN`。

**AC-58** 收货后争议
- WHEN 已确认收货后的争议成立，THE SYSTEM SHALL 保留已收货事实并进入售后/退款流程，不得回滚 receipt（SPEC-0001 P-07）。

### 4.8 乱序与历史兼容

**AC-59** 乱序收敛（逐对可判定）
- WHEN 退款投影先于捕获事实到达，THE SYSTEM SHALL 拒绝该投影并返回 `EVENT_OUT_OF_ORDER`，由消费者按有限重试策略重试；不得在捕获事实缺失时把订单标记为已退款。
- WHEN 收货事件先于发货事件到达，THE SYSTEM SHALL 拒绝并返回 `EVENT_OUT_OF_ORDER`，由消费者按有限重试策略重试；若运单从未发货，该收货事件 SHALL 被拒绝并告警。
- WHEN 库存失败事件先于支付成功事件到达，THE SYSTEM SHALL 在支付结算时立即失败并退款，不得先确认库存。
- WHEN 缺货解决事件先于缺货上报事件到达，THE SYSTEM SHALL 拒绝该解决事件并返回 `EVENT_OUT_OF_ORDER` 并告警，不得凭空创建已解决案件。
- IF 迟到事件会造成非法迁移，THEN THE SYSTEM SHALL 拒绝该迁移并告警，不得静默覆盖已达成终态。

**AC-60** 历史 `PAID` 订单
- WHEN 上线前已 `PAID` 的历史订单被读取，THE SYSTEM SHALL 按“已支付”展示，不要求四事实回填。
- WHEN 历史订单发起退款，THE SYSTEM SHALL 仍走本 Spec 的准入；有售后单时 `refund_id = after_sale_no`，无售后单时由 Order 按 `legacy:<order_no>:<refund_seq>` 分配并保持幂等。
- WHEN 历史订单发起发货，THE SYSTEM SHALL 以 Inventory 当前预留状态为准；若为 `HELD`，先按现有确认/补偿流程收敛；若为 `CONFIRMED` 则可申请许可；若“无法确认”（无预留行、Inventory 明确 not found、或确认失败超过 D-11 上限），则按缺货/退款流程处理。不得因缺少历史事实直接放行，也不得永久阻断。

**AC-61** 失败可见性与对账
- IF 捕获后 30 分钟仍未结算，THEN THE SYSTEM SHALL 向用户显示“支付处理中，已记录，无需重复支付”，并向运营告警；该 30 分钟不改变 AC-47 的终态判定。
- IF 退款任务永久失败、许可准入冲突或预留释放失败，THEN THE SYSTEM SHALL 告警并提供按 `payment_no/refund_id/after_sale_no` 的人工重放入口；用户侧显示“处理中”。
- WHEN 每日对账，THE SYSTEM SHALL 对平 Payment `captured/refunded`、intent 级“已收未 PAID”资金（含时间窗口）、Order 投影、非生效捕获退款与未完成退款任务；差异 SHALL 告警并保留可重放记录。

---

## 5. 状态机与非法迁移

### 5.1 订单生命周期（Order）

```text
PENDING_PAYMENT --支付确认(AC-47)--> PAID --首组发货--> ALLOCATED --全部发货--> SHIPPED --全部收货--> COMPLETED
PENDING_PAYMENT --取消(AC-46) / 捕获后结算失败退款(AC-47)--> CANCELLED
```

- **未捕获的支付失败不改变订单**；由过期扫描按现有规则取消。
- 任何回退到 `PENDING_PAYMENT` 不允许，返回 `ORDER_STATE_CONFLICT`。

| 迁移 | 前置条件 | 非法时返回 |
|---|---|---|
| `PENDING_PAYMENT → PAID` | 生效支付（或零元）+ 快照冻结 + 所有行预留 `CONFIRMED` | 订单已非 `PENDING_PAYMENT` → `SETTLEMENT_CONFLICT`；订单仍待支付但库存失败 → `INVENTORY_NOT_READY` |
| `PENDING_PAYMENT → CANCELLED` | 取消胜出或捕获后结算失败退款 | `ORDER_ALREADY_PAID` |
| `PAID → ALLOCATED → SHIPPED → COMPLETED` | 组级发货/收货投影 | `ORDER_STATE_CONFLICT` |

### 5.2 发货许可（Order）与运单（Fulfillment）

```text
许可: NONE --授权发货--> CONSUMED（终态）
      NONE --退款准入--> REVOKED（终态）

运单: CREATED --Ship--> SHIPPED --签收/确认--> RECEIVED
      CREATED --退款准入/取消--> CANCELLED
```

- **`CONSUMED` 不等于 `SHIPPED`**：`CONSUMED` 只表示已授权；运单是否离仓看 S6。
- 已授权未离仓窗口：退款准入取消 `CREATED` 运单，按 `PRE_SHIP_REVOKED` 处理；Ship 与取消在运单行 CAS，唯一胜者。
- `CONSUMED → REVOKED` 不允许；`REVOKED → CONSUMED` 返回 `FULFILLMENT_NOT_ALLOWED`。
- Ship 在运单已 `CANCELLED` 后到达：返回 `SHIPMENT_CANCELLED`；Ship 败给退款取消：返回 `SHIPMENT_CANCELLED`。
- 重复 `NONE→CONSUMED`：同一 `shipment_no` 幂等；不同 `shipment_no` 返回 `FULFILLMENT_NOT_ALLOWED`。
- 重复 `NONE→REVOKED`：幂等返回 `PRE_SHIP_REVOKED`。

### 5.3 组退款进度（Order）

```text
NONE --> REFUNDING --> REFUNDED
```

- **已离仓**组允许多个 `refund_id` 的部分退款；**未离仓**组不允许部分退款（AC-50/D-19）。
- `REFUNDING` 表示仍有未完成申请；`REFUNDED` 的充分条件是：商品剩余=0，且运费责任已定并无可退运费（或已退完）；该组所有未被 `REJECTED/CANCELLED` 的 `refund_id` 均为 `DONE`，Payment 已退累计覆盖该组剩余可退额。
- IF 运费责任未定/挂起（0001 168h 内），THEN 该组 SHALL 保持 `REFUNDING`，不得置 `REFUNDED`；后续判责产生的运费退款属于同一组退款进度，不是新案件/新生命周期。
- `REJECTED/CANCELLED` 释放其预占额度，不计入 `REFUNDED`。
- 同一 `refund_id` 重复申请幂等；**已离仓**组的不同 `refund_id` 且仍有余额则允许；**未离仓**组不允许部分退款，必须整组退款（AC-50/D-19）；超出余额返回 `REFUND_ADMISSION_CONFLICT`。
- 不允许回退；退款完成以 Payment 事实为准。

### 5.4 缺货案件（Order）

```text
NOTIFIED --用户选等待--> RESOLVED_WAIT --补货到货--> RESTOCKED（终态，恢复履约）
NOTIFIED --补货到货（用户未选前）--> RESTOCKED（终态，恢复履约）
NOTIFIED --用户选退款 / 48h 未选--> RESOLVED_REFUND（终态）
RESOLVED_WAIT --补货日到期未补货--> RESOLVED_REFUND（终态）
```

- `RESOLVED_WAIT` 不是终态；`RESTOCKED/RESOLVED_REFUND` 为终态。
- 活动案件内的同一 `event_id` 重复上报幂等返回当前案件；终态后同一 `event_id` 返回 `SHORTAGE_CASE_CLOSED`；新 `event_id` 由 Order 分配新 `case_seq`。
- `RESOLVED_REFUND` 后的补货到货拒绝并告警，不得恢复发货。
- 用户选择、定时器、补货到货并发时由案件版本 CAS 裁决。

### 5.5 退款申请/任务（发起方）

```text
PENDING --> PROCESSING --> DONE
                 │
                 ├--> PENDING（可重试）
                 ├--> FAILED_PERMANENT（运营告警 + 人工重放）
                 ├--> CANCELLED（用户撤回；释放预占额度）
                 └--> REJECTED（业务驳回；释放预占额度）

注：零元单的 0 元申请以 DONE(zero_amount) 终结，不调用 Payment；只走释放/回库。
```

### 5.6 业务错误码（唯一语义）

| 错误码 | 返回方 | 含义 |
|---|---|---|
| `PAYMENT_AMOUNT_MISMATCH` | Payment | 金额不等于 `pay_cent`；不生效 |
| `DUPLICATE_PAYMENT` | Payment/Order | 第二个成功的非生效捕获；退款 |
| `ORDER_CANCELLED` | Order | 取消已胜出，支付确认失败 |
| `ORDER_ALREADY_PAID` | Order | 支付已胜出，取消失败 |
| `SETTLEMENT_CONFLICT` | Order | 结算时订单已非 `PENDING_PAYMENT` 且不是同一支付身份 |
| `INVENTORY_NOT_READY` | Order | 结算前预留确认失败/超时终态 |
| `FULFILLMENT_NOT_ALLOWED` | Order | 发货许可被拒（库存/许可/缺货/退款） |
| `SHIPMENT_CANCELLED` | Fulfillment | 运单已被退款准入取消，拒绝 Ship |
| `REFUND_ADMISSION_CONFLICT` | Order | 超出剩余额度/已全额退款后的退款申请 |
| `PARTIAL_REFUND_REQUIRES_FULL_GROUP` | Order | 未离仓组的部分退款被拒，必须整组退款 |
| `SHORTAGE_CASE_CLOSED` | Order | 案件终态后重复上报或迟到补货 |
| `SHIPMENT_NOT_SHIPPED` | Fulfillment | 未发货/已取消运单不能确认收货 |
| `RECEIPT_DISPUTE_OPEN` | Fulfillment | 存在未解决争议，不能确认收货 |
| `EVENT_OUT_OF_ORDER` | 消费者 | 乱序事件被拒绝，有限重试后死信告警 |
| `TRANSIENT_ERROR` | Payment/Order | 本地持久化失败等可重试错误；不得 ack |
| `ORDER_STATE_CONFLICT` | Order | 非法生命周期迁移 |

映射：AC-45 → `SETTLEMENT_CONFLICT`；AC-47 超时/失败 → `INVENTORY_NOT_READY`；AC-59 乱序 → `EVENT_OUT_OF_ORDER`；AC-44 已退款分支 → 幂等 ack，不再发起退款。

---

## 6. 幂等键（谁生成、作用域、重试语义）

| 对象 | 业务键 | 生成方 | 重试语义 |
|---|---|---|---|
| 支付意图 | `(order_no, amount_cent, channel)` | Payment | 同键返回同一意图 |
| 结算 | `payment_no` + `order_no` | Payment/Order | 同键幂等；不同 `payment_no` 按 AC-41 |
| 发货许可 | `(order_no, group_no)` | Order | 重复 `CONSUMED/REVOKED` 按 §5.2 |
| 退款申请 | 全局唯一 `refund_id`（AfterSales 用 `after_sale_no`；TCC 每组 `tcc:<payment_no>:<group_no>`；SYSTEM 用 `after_sale_no`；历史无售后单用 `legacy:<order_no>:<refund_seq>`） | 发起方 | 同键幂等；不同键按 AC-50 余额规则 |
| 退款执行 | `refund_id` + `payment_no` | Payment | 同键幂等；失败可重试 |
| 缺货案件 | `(order_no, group_no, case_seq)`；`case_seq` 由 Order 按 `order+group` 从 1 递增分配；上报输入必须带唯一 `event_id` | Order | 同 `event_id` 幂等/`CLOSED`；新 `event_id` 新 `case_seq`；终态后返回 `SHORTAGE_CASE_CLOSED` |

具体存储与唯一约束由 ADR 设计；业务要求是：**重复投递/重试不得产生第二条资金或库存事实**。

---

## 7. 异常与期望终态（规范性）

> 本矩阵与第 4 节 AC 具有同等规范性；冲突时以更严格的条款为准。五方 = Payment / Order / Inventory / Pricing / Fulfillment。

| # | 场景 | Payment | Order | Inventory | Pricing | Fulfillment |
|---|---|---|---|---|---|---|
| E1 | 同一 `payment_no` 回调重复 | 一笔 captured | 只结算一次 | 只确认一次 | 只核销一次 | 只建一个运单 |
| E2 | 捕获提交后进程崩溃 | captured | 继续至 PAID 或退款 | 按结算结果确认/释放 | 同左 | 仅 PAID + 该组行 `CONFIRMED` + 无活动缺货后建单 |
| E3 | 本地提交、DTM 未受理 | captured | 重启后由系统继续 | 同上 | 同上 | 同上 |
| E4 | 结算 RPC 超时未知 | captured | 幂等重试，不重复副作用 | 幂等确认 | 幂等核销 | 不重复建单 |
| E5 | 预留确认明确不足 | 全额退款 | `CANCELLED` | `RELEASED` | 释放优惠 | 不建运单 |
| E6 | 取消胜出 | 全额退款 | `CANCELLED` | 释放/补偿释放 | 释放 | 不建运单 |
| E7 | 支付胜出 | captured | `PAID` | `CONFIRMED` | 已核销 | 可申请许可 |
| E8 | 第二个 `payment_no` 捕获（金额正确） | 非生效捕获退款 | 保持原生效支付 | 不重复确认 | 不重复核销 | 不重复建单 |
| E9 | 退款准入成功、本地未提交 | 未打款 | 许可已变更，可重试 | — | — | 撤单幂等 |
| E10 | 退款执行超时 | 未知→重试 | `REFUNDING` | — | — | — |
| E11 | 单组退款 vs 另一组发货 | 只退该组 | 该组 `REVOKED`，其他组 `NONE` | 其他组不受影响 | — | 其他组正常发货 |
| E12 | 缺货重复上报 | — | 同一案件保持终态 | — | — | 运单保持已取消/未建 |
| E13 | `CREATED` 确认收货 | — | 不推进 | — | — | 拒绝 |
| E14 | 已确认后争议 | — | 保留收货事实 | — | — | receipt 不回滚 |
| E15 | 外部渠道不再重放 | 由系统继续 | 自行结算或退款 | 同左 | 同左 | 同左 |
| E16 | 事件乱序 | 以 captured 为准 | 禁止非法迁移 | 以预留状态为准 | 禁止重复核销 | 禁止重复建单 |
| E17 | 零元单 | 无 captured | `PAID` | `CONFIRMED` | 按免费单处理 | 可申请许可；退款金额 0 |
| E18 | 历史 `PAID` 订单 | 按已有 captured | 按已支付展示 | 按已有预留收敛 | 按已有核销 | 仍走许可守卫 |
| E19 | 已授权未离仓退款 | 未打款 | 取消 `CREATED` 运单，按 `PRE_SHIP_REVOKED` | 释放 | — | 运单 `CANCELLED`，不再发 |
| E20 | 补货到货（用户未选前） | — | `NOTIFIED→RESTOCKED` | 保持 `CONFIRMED` | — | 可申请许可 |
| E21 | 退款后的迟到补货 | — | 拒绝并告警，保持 `RESOLVED_REFUND` | 不恢复 | — | 不建运单 |

---

## 8. 失败可见性（用户 / 运营）与对账

| 对象 | 用户看到 | 运营看到 |
|---|---|---|
| 捕获未结算 | “支付处理中，已记录，无需重复支付”（30 分钟后） | 告警：`payment_no`、订单、持续时长；可按 `payment_no` 重放 |
| 结算失败退款 | “库存不足/支付失败，已退款” | 告警：原因、`payment_no`、库存失败项；可重放 |
| 结算冲突 | 捕获存在且未 `PAID` 且无全额退款 → “支付处理中”；捕获存在且已全额退款 → “支付失败，已退款” | 告警：`payment_no`、冲突原因；人工裁决入口 |
| 退款准入冲突 | “退款处理中” | 告警：`refund_id`、组、冲突原因 |
| 退款永久失败 | “退款处理中” | 告警 + 人工重放入口；不得静默关闭 |
| 预留释放/回库失败 | “退款处理中” | 告警：组、预留、失败原因；重试/人工补偿 |
| 缺货 | 二选一页面 + 补货日期 | 4h/48h 超时告警、案件终态 |
| 发货被拒 | “备货中/退款处理中” | `FULFILLMENT_NOT_ALLOWED` 原因（库存/许可/缺货/退款） |

对账口径：Payment 订单级 captured/refunded、intent 级“已收未 PAID”资金（含时间窗口）、非生效捕获退款、Order 投影、未完成退款任务、预留释放结果每日对平；差异必须告警并保留可重放记录。

---

## 9. 验收证据要求

- 每条 AC 必须有 `TestAC<NN>`；资金、库存、竞态必须有并发或故障注入用例，至少一个异常分支。
- 直接路径与 TCC 路径必须接受**同等**故障注入。
- 验收绑定精确 HEAD 与显式范围（`--release --range`），输出五方终态核对表。
- Mock 数据必须符合真实业务边界（零元单、多 intent、多组订单、历史 `PAID`、补货到货）。
- 未运行、SKIP、仅 AC-REF 均不算 AC-VERIFIED（`CONVENTIONS §6.1`）。

---

## 10. 待裁决问题（D）

| ID | 问题 | 建议 | 影响 |
|---|---|---|---|
| D-11 | 库存确认等待上限（技术失败/超时） | **2 小时**（可配）；到时仍未终态按明确失败处理：整单退款 + `CANCELLED` + 告警 | AC-47/E5；需甲方确认 |
| D-12 | 重复支付退款是否收费、是否退运费 | **不收费、退全部非生效捕获**（用户无过错） | AC-41；需甲方确认 |
| D-13 | 终态后同一组再次缺货 | **允许新案件**（新 `case_seq`），不得覆盖/重开旧案 | AC-56；需甲方确认 |
| D-14 | 用户可见等待阈值 | **捕获后 30 分钟**显示处理中并告警；不改变终态判定 | AC-61；需甲方确认 |
| D-15 | 等待补货到期仍未补货 | **再次通知并转退款**；补货到货则 `RESTOCKED` | AC-56；需甲方确认 |
| D-16 | 历史 `PAID` 订单是否回填四事实 | **不回填**；按已有事实展示，新动作走收敛路径 | AC-60；需甲方确认 |
| D-17 | INV-06/SPEC-0001 的组退款释放口径 | **需 SPEC-0001 升版 v1.3**：把“禁止预留已释放但订单仍已支付”限定为“仍需履约的组”；明确组退款（含普通售后）的释放/回库边界。本 Spec 不单方面改写 | §11/INV-44；需甲方批准升版 |
| D-18 | 组退款后的预留释放是否阻断退款完成 | **不阻断**：退款按 Payment 事实完成；释放失败告警重试，不得超卖 | INV-44/AC-50；需甲方确认 |
| D-19 | 未离仓组的部分退款 | **不允许部分退款；必须整组退款**（避免“已退款但余量永不发”）；后续若要保留余量履约，另立 ADR 支持运单重建/许可恢复 | AC-50/§5.3；需甲方确认 |
| D-20 | 历史单“无法确认”或缺货路径 | 按 AC-60 收敛；历史单缺货属新业务口径，需甲方确认是否沿用 §6.6 | AC-60；需甲方确认 |
| D-21 | TCC 系统补偿是否退全部未退运费 | **退全部未退运费**（系统/平台原因，用户无过错） | AC-51；需甲方确认 |

---

## 11. 与既有契约的关系及需要的升版

- 基线 INV-01…16、SPEC-0001 INV-17…29 继续有效；本 Spec 不废止任何 INV/AC，但依赖 SPEC-0001 v1.3 对 0001:287 的限定（D-17）。
- 明确沿用的口径：
  1. **`PAID` 前置条件**：沿用基线 INV-06“确认在标记支付之前”，收窄为“所有订单行预留 `CONFIRMED` 才能 `PAID`”。
  2. **许可枚举**：沿用 SPEC-0001 S5 与基线 §3.4 的 `NONE/CONSUMED/REVOKED`；不新增 `ALLOWED`，`FulfillmentAllowed` 只是发货时刻谓词。
  3. **签收事实**：沿用 SPEC-0001 S7；Fulfillment 落签收证据，Order 只消费与投影。
  4. **取消**：沿用 SPEC-0001 §6.7；已支付订单不提供取消，只能走售后/退款。
- **需要 SPEC-0001 升版的冲突（D-17，不能由本 Spec 静默改写）**：
  - `docs/specs/0001` 第 287 行明文禁止“预留已释放但订单仍为已支付”。本 Spec 的组退款（普通售后/缺货）在未离仓时必然释放该组预留，同时订单生命周期仍为 `PAID`（退款态由 `refund_state` 投影表达，不存在 `PARTIALLY_REFUNDED` 生命周期值）。二者只能通过升版 0001 解决：把该禁止限定为“仍计划履约的组”，并明确组退款释放库存的合法性；否则只能改为“任何组退款都整单退款”，但这会与 AC-27/INV-38 冲突。
  - 该升版必须由甲方批准，并在 0001 变更记录写明影响的 INV-06/INV-26/AC-27；未批准前本 Spec 不得进入 `approved`。
- 本 Spec 如与上述条款冲突，以更严格且更晚批准的条款为准，并在 §13 记录影响的 INV/AC。

---

## 12. 数据兼容、上线与回滚

- 本 Spec 不规定存储结构；ADR 决定是否新增持久意图/任务/案件序号。若新增，必须同时提供历史数据回填、回滚与旧镜像 smoke 方案。
- 历史 `PAID` 订单按 AC-60 展示与收敛；不得因缺少四事实而直接放行，也不得永久阻断。
- 上线顺序建议：先实现只读事实与守卫（不改变现有写入路径）→ 再切换写入协议 → 最后开启相关开关；每个阶段必须有回滚条件。
- 生产开关保持关闭，直到 §7 矩阵、五方终态核对与 G-4 通过；UAT 结果不得作为 Release PASS 证据。

---

## 13. 变更记录

| 版本 | 日期 | 变更 | 影响 |
|---|---|---|---|
| v1.0-draft | 2026-11-23 | 初稿；QC-01 打回 | 首次提出 |
| v1.1-draft | 2026-11-23 | 按 QC-02 整改：补 `RESTOCKED`、许可/运单窗口、组退款预留释放、多部分退款、`refund_id` 唯一、乱序逐对、历史收敛、对账；把 INV-06 冲突改为 SPEC-0001 升版请求（D-17） | 新增 INV-44；AC-39…61 语义收紧；D-11…D-18 待甲方裁决 |
| v1.2-draft | 2026-11-23 | 按 QC-03 整改：离仓/未离仓判定、准入结果持久化、缺货期间预留、`NOTIFIED→RESTOCKED`、`case_seq`/`event_id`、未离仓部分退款禁止、错误码与 ack、对账补 pre-PAID、历史“无法确认”、文案/术语修正 | AC-48/50/51/56/59/60/61 收紧；新增 D-19/D-20；待 QC-04 确认 |
| v1.3-draft | 2026-11-23 | 按 QC-04 整改：AC-50 把“额度校验”前置到许可/运单动作之前；AC-48 拆分许可与运单状态，消除同输入两码；AC-51 取消走 `orders.status` CAS；准入结果归 Order、申请/任务归发起方；补 `shipped_at`、`REFUNDED` 充分条件、释放顺序、并发集合、对账摘要、`TRANSIENT_ERROR` | AC-48/50/51/56/§2/§5.3/§8/§5.6 收紧；待 QC-05 确认 |
| v1.4-draft | 2026-11-23 | 按 QC-05 整改：TCC 补偿改为订单级→组级扇出（每组 `tcc:<payment_no>:<group_no>`）；零元单“已全额退款”定义；AC-50 无运单/矛盾态/许可 CAS 败者分支；退款任务补 `CANCELLED/REJECTED`；错误码映射；重试“已是目标状态即成功” | AC-50/51/§2/§5.3/§5.5/§5.6 收紧；待 QC-06 确认 |
| v1.5-draft | 2026-11-23 | 按 QC-06 整改：额度口径纳入 `freight_share`；未离仓整组退款金额与上限；TCC 按组剩余（非原始份额）拆分并跳过已全退组；§6 TCC 键补组号；错误码表修复；零元单不调用 Payment；历史无组按 G1 | AC-50/51/§6/§5.6 收紧；待 QC-07 确认 |
| v1.6-draft | 2026-11-23 | 按 QC-07 整改：TCC 补偿改为系统责任、每组全额（商品剩余 + 全部未退运费）、不做分摊；Σ组上限≠订单剩余 captured 时阻断告警；REFUNDED 要求运费责任已定；零元单 `DONE(zero_amount)`；历史无许可行初始化 G1；新增 D-21 | AC-50/51/§5.3/§5.5 收紧；待 QC-08 确认 |
| **v1.6（冻结）** | 2026-11-23 | 甲方批准 v1.6；C-1（SPEC-0001 v1.3）已先行批准；D-11…D-21 全部按建议默认生效；C-9 落库（台账/索引/提交） | 新增 INV-30…44、AC-39…61；ADR-0007 启动 |


---

## 14. 批准记录

| 角色 | 结论 | 日期 |
|---|---|---|
| 甲方（业务） | 批准 SPEC-0001 v1.3（组退款释放边界）；解除本 Spec 的 C-1 | 2026-11-23 |
| 甲方（业务） | 批准 D-11…D-21 全部按建议默认值生效 | 2026-11-23 |
| 甲方（业务） | 批准本 Spec v1.6 与 C-9 落库（台账/索引/提交） | 2026-11-23 |
| Product Owner | 移入 `docs/specs/`、更新索引与 AC 台账指针 | 2026-11-23 |
| Quality Checker | QC-08 `PASS WITH CONDITIONS`；C-1/C-8/C-9 关闭后无代码级 BLOCKER | 2026-11-23 |
