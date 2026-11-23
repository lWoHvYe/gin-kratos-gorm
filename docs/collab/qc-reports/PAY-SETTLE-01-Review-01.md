# PAY-SETTLE-01 Review-01 — 实现有范围审查（P0-A / P0-B）

| 项 | 值 |
|---|---|
| review_scope | **实现**审查（非 Spec/ADR 验收、非 Release）：ADR-0007（accepted, Rev 1.0）D-01…D-14；SPEC-0002 v1.7；SPEC-0001 v1.3；`git log d38f282..HEAD` 中 f3c9ac0…03a0717 的代码/迁移/测试；当前工作区 |
| base | `d38f282`（已批准 Spec 之前的基线） |
| head | `03a0717`（`fix(order): reject a second payment identity on PAID`）；工作区干净（`git status` 无输出） |
| 契约基线 | `docs/adr/0007-payment-settlement-admission.md`；`docs/specs/0002-payment-settlement-admission.md` v1.7；`docs/specs/0001-fulfillment-quantity-split.md` v1.3 |
| INV | 受影响：INV-01/04/05/06/07/14/24/26/30…44；本报告发现 INV-30/34/36/37/40/43 与 INV-24 的实现缺口 |
| affected_AC | AC-39…61；重点 AC-40/41/43/45/47/48/50/51/53/54/56/60/61 |
| verification | **引用 lead 提供的执行日志，未重跑**：order 7.586s、payment 0.672s、aftersales 10.973s、fulfillment 5.653s；定向测试 CheckOrderReady / CancelIfCreated / ListOrderIntents / SettlementTask / PricingTask / SettleZero / AdmitRefund / Commitment / AC45。测试使用各自测试文件内的内联 DDL（非 `migrations/`），集成测试在无 `TEST_MYSQL_DSN` 时 Skip；AC-39…61 中仅 `TestAC45TccConfirmRejectsSecondPayment`（`duplicate_payment_integration_test.go:12`）有 AC 命名，其余为未命名流程测试或未覆盖；AC-01…38 的命名测试属 0001/0005 基线 |
| release_status | **FAIL**（实现级；不等于 Release PASS） |

> 本报告仅裁决本次实现是否满足契约；不等于 Spec 全量验收，不构成 Release PASS，不替代 AC-VERIFIED。未运行测试、未改代码/契约。

## 1. 结论

**FAIL。** 主要能力（CheckOrderReady、CancelIfCreated、settlement/refund task、SettleZero、AdmitRefund、commitment、事件/迁移）已落位，且两阶段退款准入的 happy path 与幂等测试通过；但存在 **6 个契约级阻断项**，集中在“退款后仍可发货/发货准入、零元与历史单、承诺释放、组额度口径”上，均会让 AC-40/50/51/53/56/60 无法验收。按 AGENTS §2.2，这些必须在当前分支前修复；其中不存在需要打回 Architect 的 `[ARCHITECTURAL_FLAW]`（ADR 的 D-08/D-10/D-09/§6.1 要求明确，是实现偏离）。

## 2. 阻断项

| ID | 严重级 | 区域 | 证据 | 问题 | 必须怎么改 |
|---|---|---|---|---|---|
| P-01 | **BLOCKER** | TCC 补偿 / 履约准入 / D-08/D-10/AC-51 | `internal/services/order/tcc.go:172-260`（`TccCancelMarkPaid` 只置 `refund_state=REFUNDING` 并 Outbox `order.refund_requested`）；`internal/services/order/settlement.go:289-330`（`AuthorizeShipment` 只拦 `refund_state=FULL`）；`internal/services/order/settlement.go:146-180`（`groupActions` 的 `ship` 也只判 `FULL`） | TCC 补偿走**整单** `order.refund_requested`，没有逐组 `AdmitRefund`/许可 CAS；未发货组的 permit 仍为 `NONE`，Order 处于 `REFUNDING` 但 `AuthorizeShipment` 不拦，`groupActions` 仍给出 `ship` → **退款与发货可同时发生**（已退款又发出），AC-51 的逐组系统责任/Σ 也未实现 | TCC 补偿必须按活动组逐组走 `AdmitRefund`（`tcc:<payment_no>:<group_no>`），由许可行决定 PRE/POST；或等价地在整单补偿前撤销所有未发货组 permit；`AuthorizeShipment` 增加 `REFUNDING` 组级拦截；补 AC-51 的 Σ 等额/阻断/人工与“退款后发货”竞态测试 |
| P-02 | **BLOCKER** | 缺货禁发 / FulfillmentAllowed / INV-40/AC-56/§6.1 | `internal/services/order/shortage.go`（`ReportGroupShortage` 不撤 permit）；`settlement.go:289-330`（`AuthorizeShipment` 不查活动缺货、不查行 `CONFIRMED`）；`settlement.go:150-166`（`groupActions` 的 `ship` 同样不查缺货） | 支付后缺货案件处于 `NOTIFIED/RESOLVED_WAIT` 时，组 permit 仍 `NONE`、`ship` 动作仍在，`AuthorizeShipment` 允许消费 permit → **缺货组可被发货**（应“预留保持 CONFIRMED、禁发”） | 在 `AuthorizeShipment` 与 `groupActions` 的交集实现 FulfillmentAllowed：`PAID ∧ 行 CONFIRMED ∧ 无活动缺货 ∧ permit=NONE`；缺货上报时冻结该组发货（permit 或显式 case 查询）；补 AC-56 的“缺货组不可 Ship”用例 |
| P-03 | **BLOCKER** | 零元单 / AC-40 | `internal/services/order/refund_admission.go:80-88`（`GetIntent(settlement_payment_no)` 且 `CapturedCent<=0` 直接失败）；`internal/services/order/zero.go:34`（零元 `settlement_payment_no=zero:<order_no>`，无 Payment intent） | 零元单的售后/缺货退款走 `AdmitRefund` 时 `Payment.GetIntent("zero:…")` 找不到 intent → 准入失败；零元单“只释放/回库、不调 Payment”的 AC-40 路径实际不可执行，退款/缺货无法撤销许可 | 零元（`settlement_payment_no` 以 `zero:` 开头或无 Payment intent）在准入中走专用分支：金额 0、跳过 Payment 权威额度、仍做许可/组额度/两阶段；补零元售后的许可撤销/运单取消测试 |
| P-04 | **BLOCKER** | 历史单 / AC-60/D-05/§4.4 | `internal/services/order/refund_admission.go:80-82`（空 `settlement_payment_no` 直接 `Failed`）；`internal/services/payment/service.go:225`（`ListOrderIntents` 实现，但 `grep` 仅测试调用）；`migrations/order/0016_settlement_admission.sql` 无回填 | 历史 `PAID` 单无 `settlement_payment_no` 时，`AdmitRefund` 一律失败；`ListOrderIntents` 无生产调用方，也没有回填脚本/迁移 → 历史退款被永久阻断，与 ADR §4.4“解析/回填、无法解析入人工队列”不符 | 在准入前调用 `ListOrderIntents`/`GetPaymentByOrder` 解析唯一候选并回填（幂等）；0/多候选入人工队列并告警；补回填 dry-run/计数/回滚；补 AC-60 历史退款用例 |
| P-05 | **BLOCKER** | 承诺释放 / D-09/AC-53 | `refund_admission.go:166-170`（只插入 `RESERVED`）；`refund_consumer.go:83`（只 `RESERVED→SETTLED`）；全仓 `grep RELEASED` 仅 schema/ADR，无写入；`internal/services/aftersales/workers.go` 的 `releaseRefundTask` 只释放本地 `after_sale_amount_ledger`，不调 Order 释放 commitment | 退款申请被拒绝/取消、或 `FAILED_PERMANENT` 后，Order 的 commitment 永久 `RESERVED`：订单级 `SUM(RESERVED)` 与组 `groupConsumed` 永久扣减 → 后续合法退款被误拒；`RELEASED` 只有状态机没有调用方 | 在拒绝/取消/永久失败路径调用 Order 释放（`RESERVED→RELEASED`，幂等、带 refund_id）；补“失败后释放、下一笔可退”的用例；对账修复已 `RESERVED` 但 Payment 已退/申请已终态的承诺 |
| P-06 | **BLOCKER** | 组额度口径 / AC-50/0001 §6.5 | `refund_admission.go:128-152`：`groupRemaining = Σ line_paid + Σ freight_share − consumed`，无“本次责任是否可退运费”过滤，`Source` 未参与；未离仓组要求 `amount == groupRemaining` | 普通售后（用户责任、运费不可退）的整组商品退货，aftersales 计算出 `goodsCent`（运费 0），而准入把整组运费算进 `groupRemaining` → `amount < groupRemaining` → 误报 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`，整组退款被拒。ADR §4.3 的三层（组商品/组运费按责任/TCC 全部未退/整组退款金额）未实现 | 准入接收并校验责任/来源（普通售后按 0001 §6.5 本次可退运费，TCC/SYSTEM 按全部未退）；`整组退款金额` 用责任口径，`组剩余可退上限` 用最大口径；补“用户责任整组退货（运费 0）可退”与“TCC 全运费”两用例 |
| P-07 | MAJOR | 金额精确 / INV-30/D-06 | `internal/services/order/service.go:1327-1338`（`paidAmount`：`paidCent > pay_cent` 才拒）；`tcc.go:43/83`（`paid > pay_cent` 才拒，允许 `paid < pay_cent`） | Order 侧允许**少付**进入 `PAID`；新建 intent 由 `CreateIntent` 保证足额，但历史 intent 或直接 RPC 的 `paid_cent` 小于 `pay_cent` 时仍可结算，违反 INV-30“严格等于” | Order 的最终校验改为 `paid_cent == pay_cent`（零元 0），否则 `PAYMENT_AMOUNT_MISMATCH`；TCC Try/Confirm 同改；补少付/超付不可 PAID 用例 |
| P-08 | MAJOR | D-04 单一结算路径 | `service.go:591`（`MarkPaid`）与 `tcc.go:54`（`TccConfirmMarkPaid`）各自复制快照/冻结/事件/pricing task；TCC 分支缺 `ensureAllocationSnapshotTx` | D-04 要求直接/TCC 共用 `settleOrderTx`；现状是两套函数，TCC 对无快照的历史单可能只冻结订单不冻结快照，后续路径漂移 | 抽出 `settleOrderTx`（事务外就绪、事务内快照/冻结/CAS/事件/pricing task），`MarkPaid`/TCC Confirm 只做各自的入口校验并调用它；补 TCC 历史单快照用例 |
| P-09 | MAJOR | Fulfillment 消费者 / D-12/§6.2/§12.3 | `cmd/fulfillment/main.go:65-68`：`FulfillmentConsumeOrderSettled=false` 时仍订阅 `shop.order.paid.v1` 并建 `CREATED`；`consumer.go` 的 `autoShipAllowed` 用 BFF `groupActions`，非 permit/行/缺货事实 | 默认仍从 `order.paid` 建运单；`autoShipAllowed` 只代理动作，不能替代 FulfillmentAllowed；灰度默认未切换，存在新旧消费者双建/重复风险（`CreateShipment` 唯一键兜底但会重试） | 灰度步骤先影子后停旧（§12.3）；`autoShipAllowed` 改查 Order 的 FulfillmentAllowed（或 `AuthorizeShipment` 前置）；切换前验证 `order.paid` 不再建可发货占位 |
| P-10 | MAJOR | 迁移/表结构 / §5.1/§12.1 | `migrations/payment/0011_*.sql`、`migrations/order/0016_*.sql` 无 `refund_applications`；`grep refund_applications` 全仓无 | ADR 的“两类退款统一记录 `refund_applications`”未落地：支付级退款只有 `payment_duplicate_captures`/`refunds`，组级只有 `after_sales`；退款申请事实分散，无法按 ADR 对账/恢复 | 新增 `refund_applications`（或明确以 aftersales/payment 既有表映射并回写 ADR 偏差），让两类退款统一可查；补迁移与 backfill 方案 |
| P-11 | MAJOR | 行级 INV-24 / AC-53 | `refund_admission.go:128-141` 只做组/订单层，无行级剩余校验；行级只在 `internal/services/aftersales` 的 `after_sale_line_ledger` 本地执行 | Order 准入不校验行级额度；越权调用 `AdmitRefund` 或 aftersales 计算偏差时，组内某行可超退（组总额仍在内），违反 INV-24 的“行 + 订单两层” | `AdmitRefund` 增行级入参（line_no/quantity/amount）与行剩余校验，或强制只接受 aftersales 已签名的行级结果并回查；补行级超退用例 |
| P-12 | MAJOR | 事务内远程调用 / §14 | `internal/services/aftersales/workers.go` 的 `processRefundTask`：`BeginTx`→`lockAfterSale`→`computeLineRefunds`→`s.payments.RefundOrder`（远程）→ 更新，均在同一个 `tx` 内 | ADR §14 明确“禁止在 DB 事务内做跨服务调用”；当前在持有行锁时调 Payment，超时会阻塞并发售后与锁等待 | 拆为“本地预占（tx 提交）→ 远程 Payment.Refund（幂等）→ 本地完成（tx）”或 outbox 驱动；保持 `after_sale_no` 幂等；补超时/重试用例 |
| P-13 | MAJOR | 测试/AC / 发布证据 | AC-39…61 中仅 `TestAC45TccConfirmRejectsSecondPayment`（`duplicate_payment_integration_test.go:12`）有 AC 命名；其余定向测试（AdmitRefund/Commitment/SettlementTask/SettleZero/ListOrderIntents/Ready/CancelIfCreated）未标 AC；测试内联 DDL 建表（`settlement_integration_test.go:21-...`）而非 `migrations/`；`review-tracking` 记 AC-39…61 为 deferral（developer-reviewer/T-002） | 当前无“AC-39…61 逐条可验收”的证据：AC-39/40/41/42/43/44/46/47/48/49/50/51/52/53/54/55/56/59/60/61 无 AC 命名测试，且零元退款、历史退款、TCC Σ/补偿、缺货禁发、RELEASED、组运费责任、退款 vs 发货竞态、Fulfillment 切换均无覆盖；AC-57/58 复用 ADR-0005（其 receipt/争议测试在 fulfillment 的 AC-17…37，本次未重跑）；测试未跑真实迁移 | 按 AC 建立可执行用例（故障注入/竞态），测试使用 `migrations/` 起库；在 `ac-coverage.tsv` 标注 AC-REF 与实测；在补齐前不得宣称实现 PASS 为 Release 证据 |
| P-14 | MINOR | 任务字段 / D-13 | `internal/services/payment/settlement.go:36-43` 插入 task 不含 `dtm_gid`/`dtm_status`；`tcc.go` 调用时用 `payment-confirm:<payment_no>` | ADR §12.1 的 `dtm_gid/dtm_status` 未写入，启动前“查既有 gid”无落点；gid 只在 TCC 调用处 | 在 task 行写入确定性 gid，worker 启动/领取时查询并复用；补 gid 幂等断言 |
| P-15 | MINOR | 准入 Tx2 幂等 | `refund_admission.go:221-225`：Tx2 `UPDATE … WHERE refund_id=?` 无 `state='PENDING_REMOTE'`/version 守卫；并发重复会重复写/重复 outbox | 同 `refund_id` 并发时结果一致但会重复事件/版本跳变 | Tx2 加 `state=PENDING_REMOTE`/version 条件，影响 0 行则读回既有结果 |
| P-16 | MINOR | 零元恢复 / §2.1 | `cmd/order/main.go` 无零元结算 worker；`grep zero_settlement` 无实现 | 零元 `SettleZero` 仅同步入口，调用方崩溃后订单停在 `PENDING_PAYMENT`，可能被过期取消，W1 无恢复 | 增加 `zero_settlement` 任务或 Order worker 扫描 `pay_cent=0` 的 PENDING 单，幂等调用 SettleZero |

## 3. ADR D-01…D-14 落点核对

| 决策 | 现状 | 备注 |
|---|---|---|
| D-01 四事实分离 | **部分** | PaymentCaptured/OrderSettled/InventoryReady 有落点；FulfillmentAllowed 只在文档/动作层，`AuthorizeShipment` 未实现行 CONFIRMED/缺货（P-02/P-09） |
| D-02 捕获后持久结算 | **是** | `payment_settlement_tasks` + worker；零元无任务（P-16） |
| D-03 CreateIntent 足额 | **是** | 新 intent 校验；Order 末端允许少付（P-07） |
| D-04 settleOrderTx 共用 | **否** | 两个函数（P-08） |
| D-05 CheckOrderReady 强制 | **是** | 生产 `RequireInventory:true`（`cmd/order/providers.go:126`）；两路径都调用；nil 仅测试 |
| D-06 CAS=0 重读 | **是** | `MarkPaid`/TCC 的 affected==0 重读；金额校验不严（P-07） |
| D-07 单一生效支付 | **是** | `settlement_payment_no`、重复捕获在 CAS 后由 worker 退款；`payment_duplicate_captures` |
| D-08 两类退款 | **部分** | 组级 `AdmitRefund`、支付级 duplicate/cancel 有；TCC 补偿整单绕开组准入（P-01）；零元（P-03）；`refund_applications` 缺（P-10） |
| D-09 额度双层 + Payment 权威 | **部分** | `GetIntent` 权威 + `order_refund_commitments`；`RELEASED` 无写入（P-05）；组运费责任（P-06）；行级（P-11） |
| D-10 未离仓整组 + TCC Σ | **部分** | 未离仓整组校验有；TCC 逐组/Σ 未实现（P-01） |
| D-11 已授权未离仓两阶段 | **是** | `PENDING_REMOTE→CancelIfCreated→Tx2`；`CONSUMED+CANCELLED→PRE`、`ALREADY_SHIPPED→POST` 有测试 |
| D-12 事件 subject/双发 | **是** | `shop.order.settled.v1`/`shop.order.refund_admitted.v1` 注册；`order.paid` 同事务双发；Fulfillment 切换默认未开（P-09） |
| D-13 唯一 TCC gid | **部分** | 调用用 `payment-confirm:`；task 的 `dtm_gid` 未写（P-14） |
| D-14 零元不经过 Payment | **部分** | `SettleZero` 有；零元退款/售后被准入阻断（P-03）；无恢复 worker（P-16） |

**“只有能力、没有调用方/消费者”清单**：`ListOrderIntents`（P-04，无生产调用）、`order_refund_commitments.RELEASED`（P-05，无写入）、TCC 逐组/Σ（P-01，无实现）、`refund_applications`（P-10，无表）、行级准入（P-11，只在 AfterSales 本地）、零元恢复（P-16，无 worker）、`order.settled` 消费者（P-09，默认关闭）。

## 4. 重点核对

1. **AdmitRefund（AC-50/51/53/54）**：许可/两阶段/幂等有；**缺**行级 INV-24（P-11）、组运费责任（P-06）、TCC Σ（P-01）、RELEASED（P-05）；未离仓整组校验有（`amount==groupRemaining`）。
2. **settleOrderTx（D-04/D-05/D-06）**：`CheckOrderReady` 事务前、生产 fail-closed ✓；`settlement_payment_no`/`order.settled` 双发/CAS=0 重读 ✓；但两套函数（P-08）、金额允许少付（P-07）；`history PAID` 走 `MarkPaid` 时无 `settleOrderTx` 的快照补齐（P-08）。
3. **CreateIntent/历史/支付级退款**：新 intent 足额 ✓；`ListOrderIntents` 无调用（P-04）；duplicate/cancel 由 `handleSettlementFailure` 走 Payment.Refund（`pay:<payment_no>:…`），未进入 `refund_applications`（P-10）；“已退款又 PAID” 的 TCC/整单退款+发货见 P-01。
4. **事件/消费者**：`order.refund_admitted` 的 Fulfillment 消费者已实现（PRE→CancelIfCreated，ALREADY_SHIPPED 告警）✓；`order.settled` 切换默认关闭（P-09），灰度步骤未在代码中强制“影子/停旧”。
5. **迁移/回填**：0011/0016 与 ADR 主要表一致；缺 `refund_applications`（P-10）、无历史回填（P-04）；`settlement_payment_no` 用 `NOT NULL DEFAULT ''` 而非 ADR 的 NULL（语义等价，记为偏差，不单独阻断）。
6. **测试/AC**：见 `verification` 与 P-13；当前 **不可 Release**：AC-39…61 仍是 deferral，定向测试只覆盖 happy path，未覆盖上述阻断项。
7. **反模式**：CAS=0 未当成功 ✓；读投影做准入：`AdmitRefund` 用 Payment 权威 ✓（aftersales 的 `order_snapshot` 仅本地额度兜底，记为 MINOR）；事务内远程调用：aftersales 的 `processRefundTask`（P-12）；未加 fence 的释放：inventory 的 `RunExpiry` 只释放 HELD（与 ADR 收窄后的规则一致，✓）；按 worker 分支：未见新的按 worker 特殊恢复分支（P-09 的消费者切换除外）。

## 5. 声明

**【测试范围】** 引用 lead 的日志（包级耗时与定向测试），未重跑；未执行 `make gates`/`make verify`；未启动集群或迁移。

**【未覆盖范围】** 未重跑测试、未验证真实 MySQL 迁移/回填、未做并发/故障注入、未审 AC-57/58（ADR-0005 的收货/争议，属既有基线）、未审计价/促销的实现细节。

**【发布语义】** `release_status=FAIL` 仅表示本次实现审查不通过，不等于 Release PASS，也不能被引用为 `AC-39…61` 已验收；`AC-VERIFIED` 须由补齐 AC 用例后另行裁决（`review-tracking` 的 deferral 机制不变）。
