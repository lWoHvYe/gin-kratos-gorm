```yaml
id: SPEC-0002
title: 支付结算与履约准入
status: approved         # draft | in-review | approved | superseded
version: 1.8
updated_at: 2026-11-24
owner: Product Owner
approved_by: Director
approved_at: 2026-11-24
domain: payment
supersedes: []
superseded_by: null
adr: [0007]
```

# SPEC-0002 — 支付结算与履约准入（P0，v1.8 approved）

| 项 | 值 |
|---|---|
| 版本 | v1.8（已批准：P-04/P-05/P-06、TCC Σ、`legacy_payment_resolution`/`COMPENSATING`、`order.settled` 影子、AC-40 收窄） |
| 日期 | 2026-11-24 |
| 状态 | **已批准**：Director 2026-11-24 批准；ADR-0007 Amendment 1 同日 accepted；QC-14 `READY FOR HUMAN SPEC APPROVAL` |
| 位置 | `docs/specs/`：AC-40 收窄、AC-62…66 新增；按 Human 指示登记 AC-62…66 延期行；Reviewer 复核；AC-39…66 先以延期行登记 |
| 作者 | Product Owner |
| 下游 | System Architect（只接收本文件与 QC 报告，不接收口头补充） |
| 依据 | `AGENTS.md` §4；`docs/architecture-baseline.md` INV-01…16；`docs/specs/0001` v1.3/v1.4；`docs/adr/0007-payment-settlement-admission.md` Rev 1.0 + `0007-payment-settlement-admission-amd1.md` Amendment 1；`docs/lead/PAY-SETTLE-01/plan.md` C-02；`docs/review-tracking.md` 2026-11-24 裁决；QC-10…QC-14 |
| 范围 | H1/H2 批准的业务口径：首次支付、结算、取消竞态、库存就绪、发货准入、退款统一准入、缺货、收货；v1.8 增加 P-04/P-05/P-06、TCC Σ、历史队列 `COMPENSATING`、`order.settled` shadow |

> 本文件只定义业务边界、状态所有权、业务不变量与验收标准（EARS）。**不定义**表结构、事件名、RPC 形态、事务边界、重试算法或开关实现；这些由 Architect 在 ADR 中设计并接受本文件验收。

---

## 0. 一页决策摘要

1. **首次支付必须足额**：生效支付金额严格等于 `pay_cent`；同一订单只有一个生效支付；重复/超额/少额捕获一律不生效并全额退款；零元单（无任何已捕获 intent）免捕获。**本期不做部分支付**。
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
| **S15 退款承诺额度** | **Order** | 每个 `refund_id` 一条承诺，记录商品行/运费组成；`RESERVED`（占用可退额度）/`SETTLED`（Payment 已退款）/`RELEASED`（仅释放额度） | 退款准入、额度计算、对账、退款申请 | 不是打款事实；`RELEASED` 不等于退款完成或失败已处置；不得由 worker 自动写；不得把 `SETTLED` 回退 |
| **S16 历史支付解析队列** | **Order 记录事实；运营/Director 作裁决** | `legacy_payment_resolution`，按 `order_no` 唯一；状态 `OPEN/RESOLVING/COMPENSATING/RESOLVED/EXPIRED_REFUND/EXPIRED_ZERO/EXPIRED_FINANCE/REJECTED`（顺序与 ADR A-04 对齐）；24h 到期按 D-22 已批准分支收敛；有捕获且无离仓组必须先经 `COMPENSATING` | 发货准入、退款准入、对账、运营 | 不是生效支付事实，不能替代 Payment `captured/refunded`；不得猜 `payment_no`；不得长期停留在 `OPEN` |
| **S17 退款金额组成与责任** | **AfterSales（售后责任与行已退事实）+ 缺货事实（SYSTEM 缺货责任）**；Order 只拥有准入计算与在途承诺 | 行剩余、商品/运费组成、责任枚举与证据引用；`freight_refundable_cent` 由责任方产出 | Order 准入、Payment 执行、对账、用户展示 | Order 不得改责任、不得从自由文本推断责任；AfterSales 不得改准入结果；不得把商品金额与运费混算 |
| **S18 `order.settled` 影子事实** | **Order 产出；Fulfillment 影子消费** | 影子消费记录/比对结果；**不是**发货准入事实 | 告警、对账、切换前比对 | 不建运单；不推进组；不放行发货；不得替代 `order.paid` 兼容路径或 permit/库存事实 |

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
- S15 的 `RESERVED→RELEASED` 只有 `AC-62` 列出的合法触发；释放只改变额度占用，不恢复许可、不重开退款、不改变订单生命周期。
- S16 的 `RESOLVED` 必须回写生效支付并以幂等恢复命令继续被阻断的动作；`EXPIRED_*` 与 `REJECTED` 是终态，不得回退到 `OPEN`。
- S17 的行级“已退”事实沿用 SPEC-0001 S14（AfterSales 唯一）；Order 的在途承诺是 S15，二者不得互相替代。订单/组剩余可退额 = 行实付 − S14 已退 − S15 在途，且不超过订单 Payment captured 剩余（INV-24）。
- S18 的影子消费失败只影响告警/对账，不影响结算、发货准入和用户展示；默认关闭时不得以任何方式放行发货。

---

## 3. 业务不变量（INV-30…INV-49）

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
| INV-43 | **先准入后打款（组级）**：组级退款（售后/TCC 补偿/缺货）未得到 `PRE_SHIP_REVOKED` 或 `POST_SHIP_ALLOWED` 前不得调用支付退款；支付级退款（非生效捕获/超少付/取消）是 capture correction，不经组许可，只受 Payment captured 上限约束 | 组级未准入先退款；支付级退款被错误要求组许可 |
| INV-44 | **组退款后的库存处置有主**：未离仓组的预留由 Inventory 释放（`HELD` 直接释放，`CONFIRMED` 走基线 `CancelConfirm` 的补偿释放）；已离仓组的退货回库按 SPEC-0001 INV-03；释放/回库失败必须重试并告警；释放后 `available ≤ on_hand` 且行数量守恒（SPEC-0001 INV-17），不得超卖或重复退款 | 退款后预留无人处理；释放失败静默；释放后超卖 |
| **INV-45** | **准入不可逆与承诺释放纪律**：组级准入一旦给出 `PRE_SHIP_REVOKED/POST_SHIP_ALLOWED`，该组 permit 终态不可恢复；退款承诺只能因“打款前 `REJECTED/CANCELLED`”或“`FAILED_PERMANENT`/人工放弃且取得人工授权”而 `RESERVED→RELEASED`；释放只释放额度，不恢复 permit；禁止 worker/重试/脚本静默释放 | 把 `REVOKED` 改回 `CONSUMED`；后台自动释放承诺导致额度虚增；把释放当作退款完成 |
| **INV-46** | **历史支付解析收敛**：`PAID` 且生效支付为空的订单，必须先以 Payment 事实解析；唯一候选=`SUCCEEDED` 且 `captured == pay_cent`；零元单（**无任何已捕获 intent**）=`zero:<order_no>`；`pay_cent=0` 但有捕获时不得 zero，按 AC-40 保留的第二、三条/AC-39 支付级+人工队列；0/多候选进 `legacy_payment_resolution`，24h 到期必须离开 `OPEN/RESOLVING`（进入 `COMPENSATING` 或终态）；有捕获无离仓→先 `COMPENSATING`（撤 permit/CancelIfCreated/INV-44 释放，先持久化 `refund_applications(PENDING)`，Payment.Refund 最后）全部成功后→`EXPIRED_REFUND`（不写 `CANCELLED`）、有离仓→`EXPIRED_FINANCE`；补偿失败保持 `COMPENSATING`；不得猜测、不得按 0 额度、不得无限期 `OPEN` | 猜 `payment_no`；把无生效支付当 0 额度；把零元有捕获当零元单；队列长期 `OPEN`；用 `EXPIRED_FINANCE` 冒充已退款 |
| **INV-47** | **行级额度、商品/运费组成与责任唯一**：每笔组级退款同时满足“行级 ≤ 行剩余可退额”和“合计 ≤ 订单 captured 剩余”（INV-24 双层）；金额拆为商品 + 可退运费；责任与可退运费只由 AfterSales / 缺货事实产出，Order 只校验与计算；`UNDETERMINED` 商品照退、运费挂起 ≤168h；后置补退同组、累计退运费 ≤ 用户实付运费 | 只查组总额不查行；Order 自行判责；商品退款被运费判定阻塞；补退新建案件或超退运费 |
| **INV-48** | **TCC Σ 预检与逐组扇出**：改任何状态前，`Σ 活动组系统责任上限 == 订单剩余 captured`；不等则阻断+告警+人工，不得部分打款；每个活动组独立准入/打款；部分失败只补未完成组，崩溃续跑不得重退已完成组 | Σ 不等仍打款；用整单退款事件替代逐组准入；重跑时重复退款 |
| **INV-49** | **`order.settled` 影子不具准入权威**：默认关闭；影子只记录/比对，不建单、不放行；影子不一致/失败只告警；切换必须经独立 release 门禁与人类批准；切换后 `order.paid` 不得再作准入依据 | 用 shadow 结果发货；shadow 失败阻断结算；默认 false 长期保留旧准入路径；未批准就切换 |

---

## 4. 业务规则与验收标准（EARS）

> 语法遵循 `AGENTS.md` 附录 A。每条 AC 至少一个可执行测试；资金、库存、竞态必须有并发或故障注入用例，且至少一个异常分支。

### 4.1 首次支付、重复支付与零元单

**AC-39** 足额校验（金额维度）
- WHEN 订单创建支付意图或确认支付，THE SYSTEM SHALL 校验金额严格等于 `pay_cent`。
- IF 请求金额 ≠ `pay_cent`，THEN THE SYSTEM SHALL 拒绝发起渠道支付并返回 `PAYMENT_AMOUNT_MISMATCH`。
- IF 渠道侧仍成功捕获了 ≠ `pay_cent` 的金额，THEN THE SYSTEM SHALL 不把该捕获计入订单，并对其全额退款；向调用方返回 `PAYMENT_AMOUNT_MISMATCH`。

**AC-40** 零元单
- WHEN 订单 `pay_cent=0` 且不存在任何已捕获 intent，THE SYSTEM SHALL 不创建捕获事实，并按 AC-47 的预留确认后进入 `PAID`。
- IF 订单 `pay_cent=0` 但存在任何已捕获 intent，THEN THE SYSTEM SHALL 不按零元单处理，不得解析为 `zero:<order_no>`、不得按零元单结算或发货；按 AC-40 保留的第二、三条与 AC-39 对该等捕获逐笔全额退款/支付级处理，并进入 `legacy_payment_resolution` 人工队列；IF 退款完成，THEN THE SYSTEM SHALL 将队列置 `RESOLVED` 并保留审计，不得重复退款；IF 退款未完成，THEN 24h 到期按有捕获分支处理（无离仓→`EXPIRED_REFUND`，有离仓→`EXPIRED_FINANCE`），不得置 `EXPIRED_ZERO`。
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

### 4.9 准入后生命周期、历史解析与行级退款

**AC-62** 准入不可逆与退款承诺的人工释放

- WHEN 组级退款已取得准入结果（`PRE_SHIP_REVOKED` 或 `POST_SHIP_ALLOWED`），THE SYSTEM SHALL 保持该组许可为终态；不得把 `REVOKED` 改回 `CONSUMED`，不得为同一组恢复可发货，也不得因退款失败/放弃而重建许可。
- WHEN 退款执行进入 `FAILED_PERMANENT` 或运营决定放弃该笔退款，THE SYSTEM SHALL 保持该笔退款承诺为 `RESERVED`、继续占用可退额度，并进入人工授权队列；在人工授权完成前不得释放额度，也不得把该退款标成已退款/已完成。
- WHEN 受理人工释放请求，THE SYSTEM SHALL 按 **D-24 已批准矩阵（HUMAN APPROVED 2026-11-24（默认））** 执行：仅授权角色可授权；金额 ≥1000 元必须双人复核；全量审计；并校验最小证据字段：授权单号、操作人/授权人、时间、金额、原因码、证据引用、有效期；IF 任一字段缺失、越权、超阈值未复核、授权过期/被撤销，THEN THE SYSTEM SHALL 拒绝 `RESERVED→RELEASED`，不得改变任何额度或状态，并返回 `REFUND_RELEASE_NOT_AUTHORIZED`。
- IF 释放请求的 `refund_id` 不存在、承诺不属于该订单/组、或授权金额与承诺金额不一致，THEN THE SYSTEM SHALL 拒绝并返回 `REFUND_COMMITMENT_CONFLICT`，不得按最近记录猜测，也不得部分释放。
- WHEN 人工授权释放同一 `refund_id`，THE SYSTEM SHALL 在满足版本/租约条件的前提下幂等执行**一次** `RESERVED→RELEASED`，只释放该笔承诺占用的额度，记录审计（授权人/时间/原因/释放前后额度/`refund_id`），并保持 permit 终态、退款申请 `FAILED_PERMANENT` 记录、组退款进度与订单生命周期不变。
- IF 同一 `refund_id` 重复提交释放，THEN THE SYSTEM SHALL 返回首次释放结果，不重复释放额度；IF 两个执行者并发释放同一承诺，THEN THE SYSTEM SHALL 以版本化条件迁移与执行租约（CAS/lease）裁决，只允许一个成功，另一个返回当前状态且不得写状态。
- IF 该承诺已 `SETTLED`（Payment 已退款）或已 `RELEASED`，THEN THE SYSTEM SHALL 拒绝再次释放并告警，不得双重扣减，也不得把已退金额退回额度；IF 释放金额与该承诺 `RESERVED` 金额不等，THEN THE SYSTEM SHALL 拒绝部分释放或超额释放，不得拆分承诺。
- IF Payment 已有该 `refund_id` 的退款成功事实但承诺仍为 `RESERVED`（对账窗口），THEN THE SYSTEM SHALL 先按 Payment 事实收敛为 `SETTLED`，不得释放，也不得把已退金额重新计入可退余额。
- WHILE 承诺仍为 `RESERVED`，THE SYSTEM SHALL 继续把它计入订单/组的在途占用，禁止同一额度再承诺给其他 `refund_id`；IF 同一额度被重复承诺，THEN THE SYSTEM SHALL 拒绝并告警。
- IF 后台任务、重试、运维脚本或任何非人工授权入口尝试把 `RESERVED` 改为 `RELEASED`，THEN THE SYSTEM SHALL 拒绝并告警；**禁止任何 worker 静默释放**。
- WHEN 释放完成，THE SYSTEM SHALL 只更新额度占用，不恢复发货、不把退款申请改为 `DONE`、不伪造组 `REFUNDED`；该退款仍需人工决定“重试/转其他处置”。
- WHEN 释放完成，THE SYSTEM SHALL 将释放额度回到**同一订单/同一组**的可退余额；不得跨订单或跨组复用；新退款仍需新申请并重新执行准入；释放动作不得自动触发退款（**D-25 HUMAN APPROVED 2026-11-24（默认）**）。
- WHEN 平台判定 `RESERVED→RELEASED` 是否合法，THE SYSTEM SHALL 只接受两类触发：①打款前退款申请被业务驳回或用户撤回（`REJECTED/CANCELLED`）；②`FAILED_PERMANENT` 或人工放弃且取得人工授权。IF Architect 认为还需要第三种触发，THEN 必须回 PO 升版并经人类批准，不得由实现自行新增。
- WHEN 释放触发被标记为 `CANCELLED` 或 `REJECTED`，THE SYSTEM SHALL 只接受“退款申请在执行前被用户撤回/业务驳回”的退款申请状态，并校验 `refund_id`、操作人/授权人、原因、时间与证据；IF 仅订单 `CANCELLED`、售后关闭、运单取消或其他非退款申请的终止，THEN 不得作为 `RESERVED→RELEASED` 的触发，必须拒绝并告警。

**AC-63** 历史 `PAID` 订单的生效支付解析与人工队列

- WHEN 读取、发货、退款或对账遇到 `PAID` 且生效支付标识为空（`''`/`NULL`）的订单，THE SYSTEM SHALL 先解析生效支付；不得按 0 额度处理、不得猜测 `payment_no`、不得直接判定已退款，也不得因缺少历史事实直接放行。
- WHEN 解析，THE SYSTEM SHALL 以 Payment 的支付事实为唯一依据，取该订单的全部支付 intent；候选必须同时满足 `status=SUCCEEDED` 且 `captured == 订单 pay_cent`；IF 订单号不匹配或金额不等的 intent，THEN 不得作为候选。
- IF 恰好一个候选，THEN THE SYSTEM SHALL 仅当生效支付仍为空时条件写回该 `payment_no`，并继续原动作；IF 并发写回，THEN THE SYSTEM SHALL 只允许一个成功，其他读取同一结果，不得产生两个生效支付。
- IF 订单 `pay_cent=0` 且不存在任何已捕获 intent，THEN THE SYSTEM SHALL 解析为 `zero:<order_no>`，不调用 Payment、不要求捕获；不得为凑候选而选择任意 intent。
- IF 订单 `pay_cent=0` 但存在任何已捕获 intent，THEN THE SYSTEM SHALL 不得解析为 `zero:<order_no>`、不得按零元单结算或发货；按 AC-40 保留的第二、三条与 AC-39 对该等捕获逐笔全额退款/支付级处理，并进入 `legacy_payment_resolution` 人工队列；IF 退款完成，THEN THE SYSTEM SHALL 将队列置 `RESOLVED` 并保留审计，不得重复退款；IF 退款未完成，THEN 24h 到期按有捕获分支处理（无离仓→`EXPIRED_REFUND`，有离仓→`EXPIRED_FINANCE`），不得置 `EXPIRED_ZERO`。
- IF 候选数为 0 或多于 1，THEN THE SYSTEM SHALL 阻断该单的未离仓组发货与全部退款，进入 `legacy_payment_resolution` 人工队列，并向用户展示“支付核验中”、向运营告警；不得按 0 额度放行，也不得无限期停留在 `OPEN`。
- WHEN 进入队列，THE SYSTEM SHALL 以 `order_no` 唯一建立记录，初始 `OPEN`，记录进入时间、候选数量、`pay_cent`、阻断范围与证据；IF 同一订单重复触发，THEN THE SYSTEM SHALL 幂等复用同一条记录，不得重复建队列；IF 该订单已有终态队列记录，THEN THE SYSTEM SHALL 按既有终态处理，不得重开 `OPEN`。
- WHEN 运营开始处理，THE SYSTEM SHALL 置 `RESOLVING`；WHILE `OPEN/RESOLVING` 且未满 24h，THE SYSTEM SHALL 继续阻断该单的未离仓组发货与退款，等待人工证据，不得自动放行；已离仓的履约事实保持不回滚。
- WHEN 人工以唯一候选解析成功（含补录证据），THE SYSTEM SHALL 置 `RESOLVED`、写回生效支付、记录授权人与证据，并以幂等恢复命令继续此前被阻断的发货或退款；IF 恢复命令重复执行，THEN THE SYSTEM SHALL 返回同一结果，不得重复发货或重复退款。
- WHEN 队列自进入起满 24h 仍未 `RESOLVED`，THE SYSTEM SHALL 按 D-22 已批准规则收敛，不得继续 `OPEN`：
  - IF 无任何已捕获 intent 且 `pay_cent=0`，THEN THE SYSTEM SHALL 置 `EXPIRED_ZERO`，以 `zero:<order_no>` 收敛，不调用 Payment；恢复命令按零元单幂等继续。
  - IF 无任何已捕获 intent 且 `pay_cent>0`，THEN THE SYSTEM SHALL 置 `EXPIRED_FINANCE`，转财务人工关闭，不发货、不退款、保留审计；不得显示为已退款（沿用 D-2 的无捕获分支）。
  - IF 存在已捕获 intent 且**没有任何组已离仓**，THEN THE SYSTEM SHALL 先离开 `OPEN/RESOLVING` 进入 **`COMPENSATING`**，不得直接置 `EXPIRED_REFUND`；在 `COMPENSATING` 内完成：撤销未离仓组 permit → `CancelIfCreated` → 按 INV-44 释放预留；上述补偿完成后，**Payment.Refund 最后执行**；全部成功后才置 `EXPIRED_REFUND`，以支付级退款事实表达结果；**不写 `CANCELLED`，保留订单生命周期**；不得伪造订单级 `captured/refunded`（INV-37），不得发货。
  - WHILE 处于 `COMPENSATING`，THE SYSTEM SHALL 只重试未完成步骤；IF 补偿失败/超时，THEN THE SYSTEM SHALL 保持 `COMPENSATING` 并告警，不得回到 `OPEN`、不得提前置 `EXPIRED_REFUND`；IF 该等捕获已按 AC-40 完成支付级退款，THEN THE SYSTEM SHALL 不重复退款，仅完成剩余补偿后置 `EXPIRED_REFUND`。
- WHEN 调用 `Payment.Refund`，THE SYSTEM SHALL 必须先幂等持久化支付级 `refund_applications(PENDING)`；IF Payment 成功，THEN 才置该申请 `DONE`，且仅在全部未退完 intent 均成功后置队列 `EXPIRED_REFUND`；IF Payment 失败/超时/未知，THEN 保持 `COMPENSATING` 并告警，不得置 `DONE`/`EXPIRED_REFUND`；IF Payment 仅部分成功，THEN 已成功申请置 `DONE`，队列保持 `COMPENSATING` 继续未完成，不得提前 `EXPIRED_REFUND`。
  - IF 存在已捕获 intent 且**存在已离仓组**，THEN THE SYSTEM SHALL 置 `EXPIRED_FINANCE`，转财务人工核定，不得自动批量退款，不得回滚已离仓事实，未离仓组保持禁发。
- WHEN 队列进入 `EXPIRED_FINANCE`，THE SYSTEM SHALL 只允许财务/授权角色在 **7 天**最长处理时限内关闭；不得重开；IF 7 天内未关闭，THEN THE SYSTEM SHALL 升级至更高级授权角色并保持 `EXPIRED_FINANCE`，不得自动退款、自动结算、自动关闭或重开。
- WHEN 关闭 `EXPIRED_FINANCE`，THE SYSTEM SHALL 记录操作人/授权人、时间、原因、金额/证据并保留审计；终态不可变。
- IF 已解析出唯一生效支付且历史订单存在其他已捕获 intent（非生效捕获/重复捕获），THEN THE SYSTEM SHALL 逐笔按支付级退款或登记重复捕获事实，不得静默留存；其退款不计入订单级 `captured/refunded`（AC-53/INV-37）；IF 生效支付尚未解析，THEN 不得按本条自动退款，按队列分支处理。
- IF 人工判定数据脏且无法收敛（订单与支付身份不可信），THEN THE SYSTEM SHALL 置 `REJECTED` 终态，禁止发货与退款，保留审计并告警；`REJECTED` 不得用于“暂时没查到”或“等待补证”。
- IF 24h 到期定时器与人工 `RESOLVED` 并发，THEN THE SYSTEM SHALL 以队列状态版本条件迁移只接受一个终态；败者返回当前终态，不得把 `RESOLVED` 覆盖为 `EXPIRED_*`，也不得把 `EXPIRED_*` 改回 `OPEN`。
- WHEN 队列进入任一终态，THE SYSTEM SHALL 记录终态、时间、操作者/依据，并进入对账；不得删除队列记录。

> **D-22 已批准（HUMAN APPROVED 2026-11-24（默认））**：24h 到期按上述已批准分支执行——无捕获且零元→`EXPIRED_ZERO`；无捕获且 `pay_cent>0`→`EXPIRED_FINANCE`；有捕获且无离仓组→先 `COMPENSATING`（撤 permit/CancelIfCreated/INV-44 释放，Payment.Refund 最后）全部成功后→`EXPIRED_REFUND`（不写 `CANCELLED`，保留订单生命周期）；补偿失败保持 `COMPENSATING`；有离仓组→`EXPIRED_FINANCE`。**D-23 同时已批准**：`EXPIRED_FINANCE` 仅财务/授权角色可关闭，不得重开，最长 7 天，逾期升级授权角色并保持 `EXPIRED_FINANCE`，审计保留。

**AC-64** 行级额度、商品/运费组成与责任唯一 Owner

- WHEN 组级退款计算金额，THE SYSTEM SHALL 对每一退货行校验 `本次行退款 ≤ 行剩余可退额`，并对本次合计校验 `≤ 订单 captured 剩余`；行剩余 = 行实付 − AfterSales 已退（SPEC-0001 S14）− 该行 Order 在途承诺（S15，承诺必须带行组成）；两层都通过才可打款（INV-24 无例外）。
- IF 本次退货数量超过行剩余数量，OR 任一行退款超过行剩余可退额，OR 本次合计超过订单 captured 剩余，THEN THE SYSTEM SHALL 返回 `REFUND_ADMISSION_CONFLICT`，不得改动许可/运单/承诺/打款。
- IF 本次退货行不属于该订单/该履约组，OR 行数量/金额缺失或为负，THEN THE SYSTEM SHALL 拒绝本次申请，不得用其他行的剩余额度替代。
- WHEN 组级退款金额组成，THE SYSTEM SHALL 拆分为“商品金额 + 可退运费”：商品金额 = Σ 本次退货行按 SPEC-0001 §6.5 R1–R4 计算的本次退款额（行实付单价 × 本次退货数量，封顶行剩余可退额）；可退运费 = 责任方按 SPEC-0001 §6.5.1/§6.5.2 判定的本次可退运费；不得把商品金额计入运费，也不得把不可退运费摊回商品。
- IF 该组未离仓 AND 本次金额 < 整组退款金额（商品剩余 + 本次可退运费），THEN THE SYSTEM SHALL 返回 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`，不得改动许可/运单/打款（沿用 AC-50/D-19）。
- WHEN 责任为 `UNDETERMINED`，THE SYSTEM SHALL 允许商品部分按本次退货数量、在行剩余额度内照常退款，运费部分挂起、不进入本次打款；该组保持 `REFUNDING`，不得置 `REFUNDED`；用户展示“运费待判定”及 168h 期限，不得显示“已结清”。
- IF 该组未离仓且运费为 `UNDETERMINED`，THEN THE SYSTEM SHALL 按“商品剩余 + 0 运费”计算本次整组退款金额，允许商品部分整组退；运费挂起并在判定后/168h 到期后于同一组补退，不得因运费未判定而拒绝商品整组退款。
- WHEN 责任判定完成，THE SYSTEM SHALL 按结构化责任枚举、`reason_code` 与证据引用决定可退运费；IF 责任枚举缺失/非法/证据不足，THEN THE SYSTEM SHALL 拒绝该判定并保持 `UNDETERMINED`，不得默认商家责任（168h 到期除外），不得从自由文本/关键字推断责任（AC-33/INV-29）。
- WHEN 计算 168h 挂起期限，THE SYSTEM SHALL 以**该组该次售后首次进入 `UNDETERMINED` 的时间**起算（**D-26 HUMAN APPROVED 2026-11-24（默认）**），并记录该时间、售后/组标识与证据引用；其他组或后续申请不得重置该计时，也不得按订单创建时间起算。
- WHEN 运费挂起满 168h 仍未判定，THE SYSTEM SHALL 默认按 `MERCHANT_WAREHOUSE` 补退该组可退运费；补退必须在**同一组退款进度**内完成，使用可追溯的补退申请标识，不得新建案件/新生命周期。
- WHEN 后置补退，THE SYSTEM SHALL 重新校验行/组/订单三层额度与“累计已退运费 ≤ 该组实付运费 ≤ 用户实付运费”；IF 超限，THEN THE SYSTEM SHALL 拒绝并告警，不得为补退突破 INV-24/INV-21。
- IF 责任判定为 `CUSTOMER` 或其他不可退运费的责任，THEN THE SYSTEM SHALL 不把运费计入本次退款，也不得为“安抚”自行退运费；用户可见“运费不退”的依据。
- IF 同一运费补退重复触发，THEN THE SYSTEM SHALL 幂等返回同一结果，不重复退款；IF 该组可退运费已全部退完，THEN THE SYSTEM SHALL 返回已完成，不重复补退。
- WHEN 计算可退运费的责任，THE SYSTEM SHALL 只接受三类唯一来源：普通售后 = AfterSales 的结构化判定（按 0001 §6.5 本次责任是否可退）；支付后 SYSTEM 缺货 = 缺货事实按 0001 §6.5.2 自动落 `MERCHANT_WAREHOUSE`；TCC 系统补偿 = D-21 的系统责任（退该组全部未退运费，不做用户责任扣减）。Order 只做枚举/证据引用校验与额度计算，不得自行改责任，也不得从自由文本/关键字推断责任。
- WHEN 已离仓组发生部分退款，THE SYSTEM SHALL 允许按本次退货数量在行剩余额度内部分退款，且只退该组已判定可退的 `freight_share`；累计退运费不得超过该组实付运费。
- IF 该组已全额退款或组剩余为 0，THEN 新的普通组级退款请求 SHALL 返回 `REFUND_ADMISSION_CONFLICT`；运费后置补退按本条前述补退条款处理；零元单首次 0 元申请不视为“已全额退款”（沿用 AC-50）。

**AC-65** TCC 补偿的 Σ 预检、逐组扇出与续跑

- WHEN TCC 补偿退款开始，THE SYSTEM SHALL 在改变任何订单/许可/承诺/退款申请状态之前，以**同一份** Payment 权威 `captured/refunded` 快照计算“订单剩余 captured”与“Σ 活动组系统责任上限”；系统责任上限 = 该组商品剩余 + 该组全部未退 `freight_share`。
- IF 订单无任何已捕获金额（零元或未捕获），THEN THE SYSTEM SHALL 不进入逐组退款、不调用 Payment，按 AC-46 取消或零元收敛；不得把 0 元组当作活动组。
- IF `Σ 活动组系统责任上限 ≠ 订单剩余 captured`，THEN THE SYSTEM SHALL 在状态变更前阻断本次补偿，告警并转人工裁决；不得部分打款，不得把差额标记为“待退款”，不得改动 permit/commitment/订单退款态。
- WHEN Σ 相等，THE SYSTEM SHALL 为每个活动组使用组级退款标识 `tcc:<payment_no>:<group_no>` 逐组申请 AC-50 的准入；不得用整单退款事件替代逐组准入。
- IF 订单仍为 `PENDING_PAYMENT` 且已捕获，THEN THE SYSTEM SHALL 先在与取消相同的订单状态裁决点进入 `CANCELLED`；IF 该裁决失败（支付确认胜出），THEN THE SYSTEM SHALL 按 `PAID` 继续逐组准入，不得把已支付订单回退为未支付。
- WHEN 定义活动组，THE SYSTEM SHALL 只纳入“系统责任上限 > 0 且未全额退款”的组；IF 无活动组，THEN 补偿幂等成功，不调用 Payment；IF 历史订单无履约组，THEN 按 G1 处理（金额=订单剩余 captured，必要时初始化 G1 许可），不得跳过。
- WHEN 所有活动组均取得准入结果，THE SYSTEM SHALL 才允许逐组打款；IF 任一活动组未取得准入结果，THEN 不得打任何款，保留任务可重试，不得只对部分组退款后放行其余组。
- WHEN 逐组打款，THE SYSTEM SHALL 按每组系统责任上限打款；IF 某组打款失败，THEN 已成功组的承诺置 `SETTLED`，失败组保留承诺与任务，只重试未完成组，不得重退已完成组。
- WHEN 崩溃/超时/消息重放后重跑，THE SYSTEM SHALL 复用同一 `payment_no` 的 TCC 事务与组级 `refund_id`，跳过已 `SETTLED`/已退款组，只补未完成组；不得创建第二套退款标识导致重复退款。
- IF 准入或打款期间订单级 captured 快照发生变化（新增捕获/退款事实），THEN THE SYSTEM SHALL 重新执行 Σ 预检；IF 仍不等，THEN 阻断并转人工，不得沿用旧快照继续打款。
- IF 某组已离仓，THEN THE SYSTEM SHALL 按 `POST_SHIP_ALLOWED` 退款，不取消运单；IF 某组未离仓，THEN 按 `PRE_SHIP_REVOKED` 撤销/取消；不得对已离仓组回滚履约事实。
- IF 某活动组已部分退款，THEN 本次只退该组剩余（商品剩余 + 未退运费），不得按原始组份额重复退款。

**AC-66** `order.settled` 影子消费（默认不切换）

- WHILE `order.settled` 影子消费开关处于关闭（生产默认），THE SYSTEM SHALL 只记录影子事实与比对结果，不创建运单、不推进履约组、不放行发货、不改变订单生命周期；发货准入仍只由许可、库存与缺货事实裁决。
- WHEN 结算提交 `order.settled`，THE SYSTEM SHALL 在兼容期与 `order.paid` 保持同一结算事实下的双发；IF `order.settled` 缺失或延迟，THEN THE SYSTEM SHALL 不得因此阻断结算主链路、用户展示或 `order.paid` 的既有兼容消费。
- WHEN 影子消费发现 `order.settled` 与 `order.paid`/订单权威事实不一致，THE SYSTEM SHALL 告警并记录差异（`order_no`、`payment_no`、差异字段），不得自动切换消费者、不得据此放行或阻断发货、不得修改结算事实。
- IF 影子消费者自身失败、超时或崩溃，THEN THE SYSTEM SHALL 不影响结算、发货准入与用户展示，只在告警与对账中体现；不得把影子失败当作结算失败。
- IF 收到未知版本或缺少必填字段的 `order.settled` 事件，THEN THE SYSTEM SHALL 拒绝该影子事实并告警，不得按默认值猜测。
- WHEN 从 `OFF→SHADOW` 开启影子消费，THE SYSTEM SHALL 仅需 Director 影子批准，不要求 SPEC-0002 §7/G-4/五方终态/`--release --range` 的全量 release 门禁；`SHADOW` 仍只记录/比对，不建单、不放行。
- WHEN 从 `SHADOW→ON` 切换为实际消费，THE SYSTEM SHALL 必须通过 SPEC-0002 §7 矩阵、G-4、五方终态核对、`--release --range`，并经 Director/人类批准；否则不得切换。
- WHEN 从 `ON→SHADOW/OFF` 回滚，THE SYSTEM SHALL 经人类批准后执行；IF 非批准的开关变更（配置、运维、脚本、自动任务），THEN THE SYSTEM SHALL fail-closed 并告警，变更不生效，当前状态保持。
- WHEN 切换完成，THE SYSTEM SHALL 以 `order.settled`（或经 ADR 批准的等价事实）作为履约建单/准入依据；`order.paid` 只保留 BFF/旧消费者兼容，不得再作为准入依据；IF 无移除日期或回滚路径，THEN 不得宣称切换完成。
- IF 开关被误置为开启而门禁未通过，THEN THE SYSTEM SHALL fail-closed 并按关闭处理，告警；不得在门禁未通过时走新准入路径。
- WHEN 已切换后回滚，THE SYSTEM SHALL 在人类批准后恢复 `order.paid` 的 BFF/旧消费者兼容消费并保留既有任务/退款申请可重放，不得重复建单、重复发货或重复退款；IF 回滚需要恢复建单，THEN 只允许经许可/库存/缺货判定的路径，不得把 `order.paid` 直接作为准入依据。

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
- 准入后的 `FAILED_PERMANENT`/人工放弃不改变许可终态；不得因退款失败而恢复可发货。
- `RELEASED` 是**退款承诺**的状态，不是许可状态；不得把它画进许可机。

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
- 运费责任为 `UNDETERMINED` 时，组保持 `REFUNDING`，即使商品部分已退；不得置 `REFUNDED`。
- 后置补退属于同一组退款进度，不新建组、不新建案件、不改订单生命周期；补退完成后才允许组 `REFUNDED`。
- `REFUNDED` 的充分条件沿用本节，并增加：该组可退运费已判定且已退完，或责任判定确认无可退运费。

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
| `REFUND_RELEASE_NOT_AUTHORIZED` | Order | 缺少可核验人工授权，拒绝 `RESERVED→RELEASED` |
| `REFUND_COMMITMENT_CONFLICT` | Order | 承诺已 `SETTLED/RELEASED`、金额不符或并发释放失败 |
| `LEGACY_PAYMENT_UNRESOLVED` | Order | 历史 `PAID` 无生效支付，队列未 `RESOLVED`/`COMPENSATING` 未完成，阻断发货/退款 |
| `TCC_REFUND_SUM_MISMATCH` | Order | TCC Σ 活动组系统责任上限 ≠ 订单剩余 captured，阻断+人工 |
| `REFUND_RESPONSIBILITY_INVALID` | Order | 责任枚举/证据/`reason_code` 非法，拒绝按该判定计算运费 |

映射：AC-45 → `SETTLEMENT_CONFLICT`；AC-47 超时/失败 → `INVENTORY_NOT_READY`；AC-59 乱序 → `EVENT_OUT_OF_ORDER`；AC-44 已退款分支 → 幂等 ack，不再发起退款。

### 5.7 退款承诺（Order）

```text
RESERVED --Payment 退款成功--> SETTLED（终态）
RESERVED --打款前 REJECTED/CANCELLED--> RELEASED（终态）
RESERVED --FAILED_PERMANENT/人工放弃 + 人工授权--> RELEASED（终态）

禁止：SETTLED -> RELEASED；RELEASED -> RESERVED；任何 worker 自动 RELEASED
```

- 同一 `refund_id` 只有一条承诺；`SETTLED`/`RELEASED` 为终态。
- `RELEASED` 只释放额度；不恢复许可、不重开退款、不改变订单/组已达成事实。
- 释放必须记录授权与审计；无授权的迁移一律拒绝（`REFUND_RELEASE_NOT_AUTHORIZED`）。

### 5.8 历史支付解析队列（Order）

```text
OPEN --运营开始处理--> RESOLVING --人工解析成功--> RESOLVED（终态）
OPEN/RESOLVING --24h 到期（有捕获且无离仓组）--> COMPENSATING
COMPENSATING --撤 permit + CancelIfCreated + INV-44 释放 + Payment.Refund 成功--> EXPIRED_REFUND（终态）
OPEN/RESOLVING --24h 到期（无捕获且零元）--> EXPIRED_ZERO（终态）
OPEN/RESOLVING --24h 到期（无捕获有金额/有离仓组）--> EXPIRED_FINANCE（终态，人工）
OPEN/RESOLVING --人工判定脏数据--> REJECTED（终态）
COMPENSATING --补偿失败/超时--> COMPENSATING（告警，重试未完成步骤）

禁止：COMPENSATING -> OPEN；COMPENSATING -> EXPIRED_REFUND（补偿/退款未完成）；
      EXPIRED_* -> OPEN；REJECTED -> OPEN；RESOLVED -> EXPIRED_*；终态删除记录
```

- 队列状态只表达“历史支付身份解析进度”，不是 Payment 的捕获事实，也不是退款完成事实。
- `COMPENSATING` 是 `EXPIRED_REFUND` 的**中间态**，只用于“有捕获且无离仓组”的 24h 到期分支；进入后先撤销未离仓组 permit、`CancelIfCreated`、按 INV-44 释放预留；再幂等持久化支付级 `refund_applications(PENDING)`；最后执行 Payment.Refund；全部成功后才置 `EXPIRED_REFUND`。
- `COMPENSATING` 内补偿失败/超时保持 `COMPENSATING` 并告警，只重试未完成步骤；不得回到 `OPEN` 或提前置 `EXPIRED_REFUND`。
- `EXPIRED_REFUND` 是“按支付级退款收敛”的终态（D-22 已批准：有捕获且无离仓组，经 `COMPENSATING` 全部成功后使用）；`EXPIRED_ZERO` 是“按零元身份收敛”的终态（D-22 已批准：无捕获且零元时使用）；`EXPIRED_FINANCE` 是“财务人工关闭/核定”的终态（D-22 已批准：无捕获有金额、或有离仓组、或无法自动收敛时使用；仅财务/授权角色 7 天内关闭，不重开，逾期升级）；`REJECTED` 只用于脏数据。
- 24h 按系统计时（小时），起点=进入 `OPEN`；有捕获且无离仓组必须先进入 `COMPENSATING`，不得直接 `EXPIRED_REFUND`；`EXPIRED_FINANCE` 由财务/授权角色在 7 天内关闭，逾期升级并保持该终态；并发到期/人工解析只允许一个终态。

### 5.9 `order.settled` 影子开关（业务状态）

```text
OFF（生产默认） --Director 影子批准--> SHADOW（只记录/比对）
SHADOW --SPEC-0002 §7/G-4/五方终态/--release --range + Director--> ON（以 order.settled 为履约准入依据）
ON --人类批准--> SHADOW/OFF（任务保留、可重放；不做 down migration）
```

- `OFF→SHADOW` 仅需 Director 影子批准，不要求全量 release 门禁；`SHADOW` 仍只记录/比对，不建单、不放行。
- `SHADOW→ON` 必须通过 SPEC-0002 §7 矩阵、G-4、五方终态核对、`--release --range` 与 Director/人类批准；`ON` 必须写明移除日期与回滚路径。
- `ON→SHADOW/OFF` 回滚须经人类批准；非批准变更 fail-closed 并告警。
- `OFF/SHADOW` 均不得以 `order.settled` 建单或放行发货；切换不在本 Spec 的默认批准范围，默认 false 不得改变。

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
| 人工释放承诺 | `refund_id + 授权单号` | Order（记录人工授权） | 同键幂等；不同键但同 `refund_id` 仍只允许一次有效释放 |
| 历史支付解析队列 | `order_no` | Order | 同键复用同一条；终态后不得重开 |
| 历史解析恢复命令 | `order_no + resolution_id` | Order/运营 | 同键幂等；只继续被阻断的动作，不重复发货/退款 |
| TCC 补偿 | `payment_no + group_no`（组级 `refund_id=tcc:<payment_no>:<group_no>`） | Order | 同键幂等；重跑跳过已 `SETTLED` 组 |
| 运费后置补退 | 原组 + 原售后/补退序号（可追溯的补退申请标识） | AfterSales/Order（按来源） | 同键幂等；不得复用已 `DONE` 的退款申请 |
| `order.settled` 影子比对 | `order_no + payment_no` | Fulfillment 影子消费者 | 重复事件只记录一次差异；不产生资金/库存事实 |

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
| E22 | 准入后 `FAILED_PERMANENT`/人工放弃 | 不重复退款 | permit 保持终态；承诺保持 `RESERVED` 或经人工授权 `RELEASED`；不恢复发货 | 不恢复预留 | — | 不重建运单 |
| E23 | 人工释放与重试并发 | 未打款 | 只有一个释放成功；另一个返回当前状态；额度只释放一次 | — | — | — |
| E24 | 历史 `PAID` 无生效支付（0/多候选） | 以已捕获 intent 为准 | 进 `legacy_payment_resolution`；未离仓组禁发、退款阻断；不猜、不按 0 额度 | 保持现状，不释放 | — | 不建新运单 |
| E25 | 历史队列 24h 到期 | 无捕获零元→`EXPIRED_ZERO`；无捕获有金额/有离仓组→`EXPIRED_FINANCE`；有捕获无离仓→先 `COMPENSATING`，完成撤 permit/CancelIfCreated/INV-44 释放后，先幂等持久化 `refund_applications(PENDING)`、再 Payment.Refund 最后执行，全部成功→`EXPIRED_REFUND`；失败/超时保持 `COMPENSATING` 告警 | `EXPIRED_REFUND` 只能由 `COMPENSATING` 全部成功进入；未离仓组保持禁发；不伪造已退款 | 仅在 `COMPENSATING` 完成撤 permit/取消运单后按 INV-44 释放 | — | `CancelIfCreated` 在 `COMPENSATING` 内完成；不建新运单 |
| E26 | TCC Σ 不等 | 以快照为准 | 任何状态变更前阻断；告警+人工；不部分打款 | — | — | 不建单 |
| E27 | TCC 部分组打款失败/崩溃续跑 | 已成功组已退 | 已成功组 `SETTLED`；失败组保留 `RESERVED`+任务；只补未完成组 | — | — | 已离仓组不回滚 |
| E28 | 行级/运费超限或责任非法 | 未打款 | 拒绝 `REFUND_ADMISSION_CONFLICT`/`REFUND_RESPONSIBILITY_INVALID`；不改许可/承诺 | — | — | 不取消运单 |
| E29 | 运费 168h 到期 | 补退运费 | 按 `MERCHANT_WAREHOUSE` 补退；同组进度；累计不超实付运费；重复触发幂等 | — | — | — |
| E30 | `order.settled` 影子不一致/失败 | — | 结算事实不变 | — | — | 只告警/记录；不建单、不放行 |
| E31 | 队列到期与人工 `RESOLVED` 并发 | — | 版本条件迁移只接受一个终态；败者返回当前状态 | — | — | — |

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
| 历史支付未解析 | “支付核验中”/“退款处理中” | 队列 `legacy_payment_resolution` 告警；24h 到期按 D-22 已批准分支；有捕获无离仓进入 `COMPENSATING`，失败/超时保持并告警；`EXPIRED_FINANCE` 财务 7 天时限；可按 `order_no` 人工重放 |
| 退款承诺待人工释放 | “退款处理中” | 人工授权队列、审计记录；按 `refund_id` 重放；禁止 worker 自动关闭 |
| 运费待判定 | “运费待判定”，最长 168h | 判责超时告警；到期按商家责任补退 |
| TCC Σ 不等 | “退款处理中” | 告警：`payment_no`、Σ 与订单剩余 captured、差额；人工裁决，不部分打款 |
| 影子比对差异 | 无用户可见变化 | 告警：`order_no/payment_no/差异字段`；只记录，不自动切换 |

对账口径：Payment 订单级 captured/refunded、intent 级“已收未 PAID”资金（含时间窗口）、非生效捕获退款、Order 投影、未完成退款任务、预留释放结果每日对平；差异必须告警并保留可重放记录。

**v1.8 对账增补**：

- 每日对平 `legacy_payment_resolution` 的进入/终态计数、`EXPIRED_*` 与 `REJECTED` 的审计、阻断订单清单；不得长期停留在 `OPEN/RESOLVING`。
- 每日对平 `RELEASED` 承诺的授权记录与额度变化；任何无授权释放视为差异并告警。
- 每批 TCC 对平 Σ 预检快照、逐组打款与 `SETTLED` 承诺；差异告警并保留可重放记录。
- 每日对平行级/运费组成：行已退（S14）+ 在途承诺（S15）+ 行剩余 = 行实付；累计退运费 ≤ 用户实付运费。
- 影子期对平 `order.settled` 与 `order.paid`/订单事实；差异只告警，不自动修复。

---

## 9. 验收证据要求

- 每条 AC 必须有 `TestAC<NN>`；资金、库存、竞态必须有并发或故障注入用例，至少一个异常分支。
- 直接路径与 TCC 路径必须接受**同等**故障注入。
- 验收绑定精确 HEAD 与显式范围（`--release --range`），输出五方终态核对表。
- Mock 数据必须符合真实业务边界（零元单、多 intent、多组订单、历史 `PAID`、补货到货）。
- 未运行、SKIP、仅 AC-REF 均不算 AC-VERIFIED（`CONVENTIONS §6.1`）。

---

## 10. 决策与待裁决问题（D）

| ID | 问题 | 决策/建议 | 影响 |
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
| D-22 | 历史队列 24h 到期后的终态分支、已离仓组处置、`EXPIRED_REFUND` 后订单生命周期终态；零元单有捕获的边界 | **HUMAN APPROVED 2026-11-24（默认）**：无捕获且零元→`EXPIRED_ZERO`；无捕获且 `pay_cent>0`→`EXPIRED_FINANCE`；有捕获且无离仓组→先 `COMPENSATING`（撤 permit/CancelIfCreated/INV-44 释放，先持久化 `refund_applications(PENDING)`，Payment.Refund 最后）全部成功后→`EXPIRED_REFUND`（**不写 `CANCELLED`**，保留订单生命周期）；补偿失败保持 `COMPENSATING`；有离仓组→`EXPIRED_FINANCE`；`pay_cent=0` 但存在已捕获 intent 时不得 `EXPIRED_ZERO`，按 AC-40 保留的第二、三条/AC-39 支付级+人工队列 | AC-63/E25/AC-60/AC-40/AC-39/AC-46/AC-25/INV-37 |
| D-23 | `EXPIRED_FINANCE` 的处置权限、是否可重开、最长关闭时限 | **HUMAN APPROVED 2026-11-24（默认）**：仅财务/授权角色可关闭；不得重开；最长 7 天；逾期升级至更高级授权角色并保持 `EXPIRED_FINANCE`；全量审计 | AC-63/S16 |
| D-24 | 人工释放权限矩阵（谁可授权、金额阈值、是否双人复核/审计） | **HUMAN APPROVED 2026-11-24（默认）**：仅授权角色可授权；金额 ≥1000 元必须双人复核；全量审计；按 AC-62 的最小证据字段校验 | AC-62/INV-45 |
| D-25 | `RELEASED` 后额度是否可立即被同一订单的新退款申请使用 | **HUMAN APPROVED 2026-11-24（默认）**：释放额度可回到**同一订单/同一组**的可退余额；不得跨订单/跨组复用；新退款仍需新申请并重新准入；释放动作不得自动触发退款 | AC-62/AC-64 |
| D-26 | 168h 运费挂起在多组/多次售后/后置补退场景的计时起点 | **HUMAN APPROVED 2026-11-24（默认）**：从该组该次售后**首次进入 `UNDETERMINED`** 的时间起算并记录证据；其他组/后续申请不重置；不按订单创建时间起算 | AC-64/AC-31/AC-32 |
| D-27 | `order.settled` shadow 的字段/版本/比对口径、切换步骤与移除日期 | **ADR-0007 Amendment 1（accepted 2026-11-24）A-09/A-10 已决策**：本 Spec 不切换；`OFF→SHADOW` 仅 Director 影子批准；`SHADOW→ON` 需 SPEC §7/G-4/五方/`--release --range` + Director；`ON→SHADOW/OFF` 回滚需人类批准；非批准变更 fail-closed | AC-66/D-4 |
| D-28 | `legacy_payment_resolution` 与“`PAID` 空 `settlement_payment_no`”的写入竞态、唯一键、队列 owner | **ADR-0007 Amendment 1（accepted 2026-11-24）A-04 已决策**：Order 唯一写队列；运营/Director 唯一裁决；不得由 Payment 写订单队列 | AC-63/S16/C-05 |
| D-29 | 代码中的 `GRANTED` 许可值与 v1.7 `<NONE/CONSUMED/REVOKED>` 枚举漂移如何收敛 | **ADR-0007 Amendment 1（accepted 2026-11-24）A-11 已决策**：以 v1.7 为准；`GRANTED` 若为历史兼容值必须显式映射，不得成为第二许可状态；不得改 Spec 迁就代码 | v1.7 §2/§5.2 |

**零元边界（B-3）**：`EXPIRED_ZERO` 只适用于 `pay_cent=0` 且**不存在任何已捕获 intent**；`pay_cent=0` 但存在已捕获 intent 时不得 `zero:<order_no>`/`EXPIRED_ZERO`，必须按 AC-40 保留的第二、三条与 AC-39 支付级处理并进 `legacy_payment_resolution` 人工队列。

**裁决状态**：D-22…D-26 已 **HUMAN APPROVED 2026-11-24（默认）**；D-27…D-29 已由 **ADR-0007 Amendment 1（accepted 2026-11-24）** 决策。

---

## 11. 与既有契约的关系

- 基线 INV-01…16、SPEC-0001 INV-17…29 继续有效；本 Spec 不废止任何 INV/AC，但依赖 SPEC-0001 v1.3 对 0001:287 的限定（D-17）与 v1.4 的 AC-27 EARS 迁移。
- 明确沿用的口径：
  1. **`PAID` 前置条件**：沿用基线 INV-06“确认在标记支付之前”，收窄为“所有订单行预留 `CONFIRMED` 才能 `PAID`”。
  2. **许可枚举**：沿用 SPEC-0001 S5 与基线 §3.4 的 `NONE/CONSUMED/REVOKED`；不新增 `ALLOWED`，`FulfillmentAllowed` 只是发货时刻谓词。
  3. **签收事实**：沿用 SPEC-0001 S7；Fulfillment 落签收证据，Order 只消费与投影。
  4. **取消**：沿用 SPEC-0001 §6.7；已支付订单不提供取消，只能走售后/退款。
- **SPEC-0001 升版冲突（D-17，已解决）**：
  - `docs/specs/0001` 第 287 行明文禁止“预留已释放但订单仍为已支付”。本 Spec 的组退款（普通售后/缺货）在未离仓时必然释放该组预留，同时订单生命周期仍为 `PAID`（退款态由 `refund_state` 投影表达，不存在 `PARTIALLY_REFUNDED` 生命周期值）。二者只能通过升版 0001 解决：把该禁止限定为“仍计划履约的组”，并明确组退款释放库存的合法性；否则只能改为“任何组退款都整单退款”，但这会与 AC-27/INV-38 冲突。
  - 该升版已由 SPEC-0001 v1.3（2026-11-23 批准）完成，并在 0001 变更记录写明影响的 INV-06/INV-26/AC-27；SPEC-0001 v1.4（2026-11-24）完成 AC-27 的 EARS 迁移。
- 本 Spec 如与上述条款冲突，以更严格且更晚批准的条款为准，并在 §13 记录影响的 INV/AC。

**v1.8 与既有契约的关系**：

1. 本 Spec v1.8 不废止 v1.7 的任何 INV/AC；新增 S15…S18、INV-45…49、AC-62…66，并收窄 AC-40（不重编号）。
2. SPEC-0001 v1.3 的 D-17 组退款释放口径继续有效；v1.4 完成 AC-27 的 EARS 基线迁移（C-01，独立批准）。P-06 的行级/运费/责任必须与 0001 §6.5.1/§6.5.2/§6.5.3/§6.6.1 一致。
3. ADR-0007 Rev 1.0 继续有效；ADR-0007 Amendment 1（accepted 2026-11-24）逐条映射 AC-62…66，并关闭 D-27…D-29；两者冲突时以 Amendment 1 为准。ADR 文件状态头由 Architect 同步；本 Spec 记录 Human 批准事实。
4. 宪法 II.5：permit 恢复与 commitment 释放由本 Spec + ADR Amendment 1 规定；实现不得自行恢复 `REVOKED` permit 或静默释放承诺。
5. D-4：本 Spec 不切换 `order.settled`，只做 shadow；默认 false；AC-66 不授权任何默认开启。
6. D-5：`order.refund_requested` 保留 subject 兼容；Payment 消费者改为观察/告警、不得直接 `Refund` 的移除日期与回滚由 ADR Amendment 1 定义。
7. 非目标不变：不引入多仓行为、部分支付、新支付渠道、Kafka/Temporal/新 ORM；不改变 0001 的拆量/选仓/签收；不改变 AI 权限。

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
| **v1.7（冻结）** | 2026-11-23 | 甲方批准 ADR-0007 v0.4 时同步澄清 INV-43 适用范围：组级退款需准入；支付级退款（非生效/超少付/取消）是 capture correction，不经组许可 | 仅澄清 INV-43 适用边界；不删除/重编号 AC；ADR-0007 引用 v1.7 |
| **v1.8（冻结）** | 2026-11-24 | 甲方批准 v1.8：P-04 历史 `PAID` 解析/`legacy_payment_resolution`/`COMPENSATING`；P-05 准入不可逆与人工释放；P-06 行级 INV-24/商品运费组成/责任/168h/后置补退；TCC Σ 预检与逐组扇出；`order.settled` shadow；AC-40 零元边界收窄；新增 S15…S18、INV-45…49、AC-62…66、E22…E31、D-22…D-29；ADR-0007 Amendment 1 accepted 同日 | 影响 INV-01/04/05/06/07/14/21/24/26/30…44 + 新增 INV-45…49；既有 AC-50/51/52/53/60/61 收窄；基线回归 AC-14/21/25/27/31/32；AC-62…66 以延期行登记；不删除/重编号 AC |


---

## 14. 批准记录

| 角色 | 结论 | 日期 |
|---|---|---|
| 甲方（业务） | 批准 SPEC-0001 v1.3（组退款释放边界）；解除本 Spec 的 C-1 | 2026-11-23 |
| 甲方（业务） | 批准 D-11…D-21 全部按建议默认值生效 | 2026-11-23 |
| 甲方（业务） | 批准本 Spec v1.6 与 C-9 落库（台账/索引/提交） | 2026-11-23 |
| Product Owner | 移入 `docs/specs/`、更新索引与 AC 台账指针 | 2026-11-23 |
| Quality Checker | QC-08 `PASS WITH CONDITIONS`；C-1/C-8/C-9 关闭后无代码级 BLOCKER | 2026-11-23 |
| 甲方（业务） | 批准 ADR-0007 v0.4；确认 INV-43 仅约束组级退款，支付级退款为 capture correction | 2026-11-23 |
| 甲方（Director） | 批准 SPEC-0002 v1.8；同时批准 SPEC-0001 v1.4 与 ADR-0007 Amendment 1 | 2026-11-24 |
| Quality Checker | QC-14 `READY FOR HUMAN SPEC APPROVAL`（0 BLOCKER；2 条非阻断待批准记录） | 2026-11-24 |
| Product Owner | 将已批准的 v1.8 变更应用为 `docs/specs/` v1.8 approved；更新变更记录/批准记录；按 Human 指示登记 AC-62…66 延期行，Reviewer 复核 | 2026-11-24 |
