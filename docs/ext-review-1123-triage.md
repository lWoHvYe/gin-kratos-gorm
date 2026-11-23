# 外部复审（2026-11-23）问题分类与规则路由

> **性质**：分析/路由建议，**不是契约**。任何业务口径变更须走 `docs/specs/**`（PO），设计变更须走 `docs/adr/**`（Architect），并按 `docs/collab/CONVENTIONS.md` §2 经 QC 与人类批准。
> **输入**：[`docs/ext-review-1123.md`](ext-review-1123.md)
> **核对基线**：`HEAD=38c221b`（AGENTS.md V2），工作区干净。
> **方法**：抽取问题 → 静态复核代码/文档 → 定性 → 定责 → 映射规则条款 → 给出下一步与门禁。
> **声明**：本轮只做静态核对，**未运行** Go/前端/MySQL/NATS/Redis/DTM/Helm/集群/浏览器。下表「复核」列：✅ = 本轮亲自读到代码或文档事实；📄 = 方向成立但未逐行复核；❓ = 仓库内无法验证。

---

## 0. 结论（给人类指挥官）

1. **这是一份高质量的「症状清单」，但不是「处理清单」。** 我复核的 15 个关键断言全部成立；但它把 Dev 可直接修的 bug、必须先改 ADR 的协议缺口、需要 PO/人类裁决的业务口径、以及 Reviewer 的验收语义问题混在了一张表里。按编号逐个补丁，正是 `AGENTS.md` §0.1 和 review 第十节都禁止的做法。
2. **根因不是「没有规则」，而是 `AGENTS.md` §4 的目标协议还没落到 ADR 和代码。** 四事实分离、持久恢复、统一退款准入、多仓身份、渐进兼容，已经是本仓库最高铁律（§4 标题即「违反即错」）；review 的 §2/§3/§4 基本是在逐条证明这些铁律被违反。因此正确路由是 **Architect 先出 ADR → Dev 一次实现 → Reviewer 用故障矩阵验收**，而不是 Dev 按条打补丁。
3. **真正需要补的治理规则只有两条，且都不新增机器门禁**：G1 把 `baseline_impact` 提升到 INV 级（解决「非 SDD 基线没人保证持续成立」）；G2 给 QC 加崩溃窗口清单（解决连续漏审同类窗口）。其余问题要么已有规则，要么是规则执行缺失。
4. **review 有 6 处需要修正后再采信**（详见 §2），其中 3 处是引用/事实缺陷：`trae/specs/...` 在本仓库及 git 历史中不存在；`ADR-0086` 实为 `ADR-0006`；`Makefile:174-202` 引用错位。另 3 处是把「已由 Spec 裁决的漂移」或「已实现的部分」误判为开放缺口。

---

## 1. 我复核了什么（关键 15 条）

| # | review 断言 | 我读到的事实 | 结论 |
|---|---|---|---|
| 1 | 列表读取列数不匹配 | `orderColumns` 24 列（`internal/services/order/repo.go:137-139`），`listOrders` 只 Scan 20 个目标（`:294-300`，缺 `allocation_frozen_at/allocation_hash/package_count/warehouse_count`） | ✅ 成立，列表必失败 |
| 2 | 迁移可跑断 | `migrations/aftersales/0012_line_ledger.sql:4-14` 同一 ALTER 里 `ADD COLUMN line_no ... DEFAULT 0` + `ADD UNIQUE KEY (after_sale_id, line_no)`；历史多明细行全变 0 | ✅ 成立 |
| 3 | Helm 开关不闭合 | `deploy/k8s/charts/mall/templates/deployments.yaml:145-160` 只给 gateway/order 注入 `WAREHOUSE_SPLIT_ENABLED`；**五个 SPEC-0001 开关（quantity/auto-confirm/carrier/shortage/freight）在 Helm 中完全没有注入点**（chart 无 `extraEnv`），`QUANTITY_SPLIT_ENABLED` 仅被 `config.go:519-527` 读取 | ✅ 成立，且比 review 描述更严重：K8s 无法从仓库资产打开这些特性 |
| 4 | Payment 创建 intent 不反查订单 | `payment/service.go:94-121` 只校验 order_no 非空、amount>0 | ✅ 成立 |
| 5 | 少付被接受 | `order/tcc.go:39-45,75-81` 与直接路径 `paidAmount`（`order/service.go:1272-1279`）均只拒 `paid > pay_cent` | ✅ 成立 |
| 6 | TCC Confirm 把 CAS=0 当成功并继续副作用 | `order/tcc.go:91-98` `affected==0 → return nil`；事务后 `:117-119` 无论是否迁移都把记录置 PAID 并 `afterPaid` | ✅ 成立 |
| 7 | TCC Confirm 无 freeze/快照 | `order/tcc.go` 全文无 `ensureAllocationSnapshotTx/freezeAllocationTx`；对比直接路径 `order/service.go:649-685` | ✅ 成立 |
| 8 | 退款事件口径不一致 | `payment/service.go:366-384` 发 `RefundedCent=本 intent 累计`、`OrderRefundedCent=整单 SUM`、`CapturedCent=本 intent captured`；`order/refund_consumer.go:47-69` 用 `OrderRefundedCent` 对比 `CapturedCent` 判 FULL | ✅ 成立 |
| 9 | `order.paid` 被当履约就绪 | `fulfillment/consumer.go:25-85` 直接消费 paid 建运单；`order/fulfillment_groups.go:69-83` 只置 status=ALLOCATED，`reservation_state` 始终 HELD（全仓无更新点）；`order/settlement.go:146-177` 不看 reservation；`fulfillment/service.go:238-249` Ship 只查 permit。**注**：直接 MarkPaid 会先 Confirm 库存（`order/service.go:606-635`），INV-06 只在默认 TCC 路径被破坏（Order 分支置 PAID 时 Inventory 分支仍可 Retryable/Failed） | ✅ 成立（TCC 路径违反 INV-06） |
| 10 | 三条退款入口不一致 | 普通售后先 RPC Block 再本地事务（`aftersales/service.go:598-620`）；TCC Cancel 只写 `order.refund_requested`（`order/tcc.go:200-220`）；SYSTEM 缺货只发 `aftersale.created`（`aftersales/shortage_consumer.go:180-205`），Fulfillment 的 `refund_consumer.go` 只订阅 `aftersale.refund_requested` | ✅ 成立 |
| 11 | permit 不查组级退款态 | `order/settlement.go:289-337` 只拒整单 FULL 与 permit REVOKED | ✅ 成立 |
| 12 | 恢复链丢仓 | `order/service.go:712-745`（compensation/confirm payload 仅 sku+quantity）；`inventory/operations.go:686-693` `item(sku,0)` 取同 SKU 首仓 | ✅ 成立 |
| 13 | 收货事件缺组号 | `fulfillment/delivery.go:160-170` 结构体有 `GroupNo`，但 `:428-435,620-627,757-765` 三个 producer 均未赋值；`events/registry.go:183-184` 只要求 `shipment_no,order_no,source` | ✅ 成立 |
| 14 | Catalog 单仓投影 | `catalog/stock_consumer.go:39-43` 且 `cmd/catalog-stock-rebuild/main.go:50-52` 同样跳过非仓 1 | ✅ 成立 |
| 15 | 门禁能力与 CI 范围 | `scripts/collab_gates.py` 只有 G-1/G-2/G-3；G-1 对 range 内 Role 做并集（`:110-155`）；默认 `HEAD^..HEAD`；输出仍写 `AC coverage`（`:339`）；`.github/workflows/go.yml:3-33` 的 paths 不含 `docs/**`、`prompt/**` | ✅ 成立 |

其余断言（auto-confirm 未 Commit、restock 授权不校验数量、fencing 缺 owner、AI 无 turn_id、RAG 构建失败 exit 0 等）我抽读了对应文件，方向均成立，标为 📄；`5.3-a` 的「永久失败释放 pending/订单预算」未逐行复核。

---

## 2. 对 review 的修正（采信前必须处理）

| # | review 的问题 | 本仓库事实 | 处理 |
|---|---|---|---|
| R1 | 引用 `./trae/specs/harden-payment-inventory-seckill/spec.md:129-159,175-216` 作为「早期 Requirements」 | 该路径**不存在**，git 全历史也无此文件（`git log --all -- '*trae*'` 为空） | ❓ 不可验证。PO 要么提供原件，要么把主张重锚定到 `architecture-baseline.md` INV-01…16 与 `docs/specs/0001` INV-17…29；否则不能作为 AC 依据 |
| R2 | 多次称 `ADR-0086 Amendment 1` | 仓库内只有 `docs/adr/0006-collab-gates.md`，其 Revision 为 **1.1 / Amendment 1**（T-003） | 编号笔误，按 ADR-0006 处理 |
| R3 | 称 `Makefile:174-202` 证明浅层 AC 测试 | 该区间是 e2e/split 脚本 target；`gates` 在 `Makefile:159` 的 `verify` 里，`check-split-concurrency`/`fault-split-uat` 确未进入 | 引用错位，结论成立；改写为「verify 未包含拆单/故障脚本」 |
| R4 | 称 ADR-0005 §3.7 与 §7.4 的矛盾「需 Architect 裁决」 | `docs/specs/0001` P-07 已批准「已确认后 15 天内可争议，但**不回滚收货事实**」，ADR §7.4 与之一致；§3.7 的「用户胜，撤销收货」是**文字漂移**（缺「确认前」限定） | 归为文档漂移（Architect 修 §3.7），不是开放业务问题 |
| R5 | 把「退款状态非空不阻断运单或推进发货」当硬需求 | 与 `docs/specs/0001` INV-04、AC-27 的「permit 裁决唯一胜者」冲突 | 不采纳原表述。正确表述：**不得因整单退款态误伤其他已发货组的合法退款**；该组退款前不得发货。由 PO 在 Spec 中写清，而非引用外部文档 |
| R6 | 称缺货「不校验选择期限」 | 48h 默认退款已实现（`order/shortage.go:236-290` `RunShortageWorker`，且 CAS 检查 RowsAffected）；真正的缺口是 `ChooseShortageResolution` 忽略 RowsAffected（`:139-167`）与 `ReportShortage` 的 `ON DUPLICATE KEY` 会覆盖已解决案件（`:72-84`） | 降级：不是「状态机缺失」，是 CAS/终态实现缺陷（IMP + 少量 ARC 版本语义） |

---

## 3. 分类框架

| 类别 | 定义 | 唯一所有者 | 主要规则 | 允许动作 | 禁止动作 |
|---|---|---|---|---|---|
| **BIZ** 业务契约 | 业务口径未定或需求未被 canonical 记录 | PO | AGENTS §1.1、附录 A；CONVENTIONS §2 | 升版 Spec、EARS AC、State Ownership | 指定技术实现；因难实现而简化 |
| **ARC** 架构协议 | 跨服务事务/恢复/事件身份/读模型缺协议，或协议自相矛盾 | Architect | AGENTS §1.3、§4；CONVENTIONS §2.1 | 新 ADR / Amendment，写恢复所有者与故障语义 | 写业务代码；让 Dev 在分支硬改 |
| **IMP** 实现缺陷 | 已有契约下的确定性 bug/漏实现 | Developer | AGENTS §1.4、§2.1、§6；CONVENTIONS §8 | 修代码 + 测试 + 自检清单 | 加 `if` 绕过协议；静默降级 |
| **GOV** 验收治理 | PASS/释放/AC 证据/门禁语义 | Reviewer（+ 流程所有者 Architect） | CONVENTIONS §4.1/§5/§6/§7；ADR-0006 | 有范围 verdict、延期台账、门禁实现 | 用 AC-REF/线程 CLOSED 冒充 Release PASS |
| **DOC** 文档漂移 | 描述性文档与代码/已批准契约不一致 | 各文档 owner | CONVENTIONS §2.1、§1 | 修正文字、更新快照 | 用旧文档描述冒充当前实现 |
| **SEC/AI** | 权限、Mesh、AI 可选面 | Architect（设计）/ Dev（实现） | AGENTS §5；§6.2 | 精确 ALLOW、版本化索引、Turn/Action 协议 | 扩 DENY 清单代替收敛默认授权 |
| **GAP** 规则缺口 | 现有规则无法覆盖的治理真空 | Architect 提案 + 人类批准 | CONVENTIONS §1/§7.1 | 最小修订 + 明确维护者 | 新增更多机器门禁 |

---

## 4. 逐项分类与规则路由

### 4.1 支付 / 结算（review §2）

| # | 问题 | 复核 | 类别/优先级 | 所有者 | 规则落点 | 下一步（产出 + 验收） |
|---|---|---|---|---|---|---|
| 2.1 | 首次支付足额、intent 反查订单未由末端保证 | ✅ | **BIZ+ARC** / P0 | PO → Architect | AGENTS §1.1、§4「事实所有权」；附录 A | PO 在新 Spec（建议 SPEC-0002）用 EARS 写明「IF 首次支付金额 ≠ 订单应付 THEN SHALL 拒绝」及「本期不做部分支付」；Architect 定 Payment→Order 身份校验契约；Dev 去掉 `paid <= pay_cent` 的隐式宽松。验收：少付/超付/重复 intent 三类负例 |
| 2.2 | Payment 已写 SUCCEEDED，DTM 未受理即崩溃无恢复所有者 | ✅ | **ARC** / P0 | Architect | AGENTS §4「分布式事务与持久恢复」；§1.3「假设与不确定性」 | ADR：SUCCEEDED 同事务写 settlement task，worker 按 payment_no 幂等驱动并记录 DTM 身份；崩溃窗口注入测试（本地提交后 kill、DTM 拒绝、CANCELLED） |
| 2.3 | TCC Confirm CAS=0 当幂等成功并执行支付后副作用 | ✅ | **ARC+IMP** / P0 | Architect → Dev | AGENTS §4「全局幂等性」；§1.5 `[ARCHITECTURAL_FLAW]` | ADR 定义「幂等成功」的判据＝同一 payment identity 已完成；CAS=0 必须重读权威状态，CANCELLED 不得触发 afterPaid。验收：预读与 CAS 之间提交取消 |
| 2.4-a | 默认 TCC 无冻结/快照，与直接路径不一致 | ✅ | **ARC+IMP** / P0 | Architect → Dev | AGENTS §4；基线 INV-06 | ADR：直接/TCC 共用同一 settlement/freeze 原语；禁止两条路径各写一套 |
| 2.4-b | 整单 refunded 与单 intent captured 混比 | ✅ | **ARC** / P0 | Architect | AGENTS §4「事实所有权」；Spec INV-24 双层校验 | ADR 明确 `order_captured_cent` / `order_refunded_cent` 与单 intent 字段，FULL 判定只用订单级口径；验收：双 intent 不同金额/channel 的部分退款序列 |
| 2.4-c | 第二个 captured intent 见 PAID 直接成功 | ✅ | **ARC** / P1 | Architect | AGENTS §4 | ADR：重复捕获唯一胜者 + 非胜者退款协议；验收：双回调并发 |
| 2.5 | `order.paid` 同时承担「已支付/库存已确认/可发货」 | ✅ | **ARC** / P0 | Architect → Dev | AGENTS §4「四事实分离」；基线 INV-06（默认 TCC 路径破坏；直接路径已先 Confirm） | ADR：`PaymentCaptured / OrderSettled / InventoryReady / FulfillmentAllowed` 分事实；Fulfillment 消费库存确认终态而非 paid；同时修 `reservation_state` 影子（当前无任何更新点）。**禁止**给 Fulfillment 再加 readiness `if` |

### 4.2 退款准入（review §3）

| # | 问题 | 复核 | 类别/优先级 | 所有者 | 规则落点 | 下一步 |
|---|---|---|---|---|---|---|
| 3.1 | 普通售后先 RPC Block 再本地事务，崩溃窗口丢任务 | ✅ | **ARC** / P1 | Architect | AGENTS §4「持久恢复」；§1.4 | ADR：先本地持久化退款意图/任务，再由 worker 调 Order 准入；验收：Block 前/后、task 提交前、Payment 超时后四处注入崩溃 |
| 3.2 | TCC Cancel 绕过 permit，直接写 `order.refund_requested` | ✅ | **ARC+IMP** / P0 | Architect → Dev | AGENTS §4「统一准入」；Spec INV-04 | 同上协议；TCC Cancel 只写意图，由同一 worker 走 permit 裁决 |
| 3.3 | SYSTEM 缺货退款无 Block、不发 `aftersale.refund_requested` | ✅ | **ARC+IMP** / P0 | Architect → Dev | AGENTS §4；Spec INV-26/AC-30 | 三入口共用状态与恢复语义；缺货决议仍归 Order 状态机。验收：缺货组退款后该组运单必须取消/不可发 |
| 3.4 | `AuthorizeShipment` 不查组级 REFUNDING/REFUNDED | ✅ | **ARC+IMP** / P0 | Architect → Dev | Spec INV-04；AGENTS §1.5 | 在 permit 行上裁决「未发货撤销 / 已授权后允许退款」；**禁止**「所有 REFUNDING 禁止发货」一刀切 |

### 4.3 多仓、事件身份、读模型（review §4）

| # | 问题 | 复核 | 类别/优先级 | 所有者 | 规则落点 | 下一步 |
|---|---|---|---|---|---|---|
| 4.1 | cancel/confirm/compensation/replay 丢 `warehouse_id` | ✅ | **ARC+IMP** / P1 | Architect（契约）→ Dev（全链路） | AGENTS §4「多仓身份」「渐进兼容」 | 版本化事件 v2，最小身份 `(order_no,line_no/sku_id,warehouse_id,quantity)`；producer/registry/consumer/DLQ/replay 一次升级；缺仓 fail-closed，仅版本化历史单仓协议可默认 W1 |
| 4.2 | 三类 received producer 未赋组号，Order 默认 G1 | ✅ | **ARC+IMP** / P1 | Architect → Dev | AGENTS §4；Spec INV-27 | 同一版本化升级；registry 增加业务非零与组合身份校验；用真实 Fulfillment outbox 重验历史“G2 receive”结论 |
| 4.3 | Catalog 仍是单仓投影，`stock_version` 单值 LWW | ✅ | **ARC+IMP** / P2 | Architect → Dev | AGENTS §4「读写隔离」；INV-09 | ADR：仓级投影或版本向量，再聚合 SKU 展示；在线 consumer 与 rebuild 共用口径。**禁止**只删过滤条件 |

### 4.4 SDD 增量的确定性缺陷（review §5）

| # | 问题 | 复核 | 类别/优先级 | 所有者 | 规则落点 | 下一步 |
|---|---|---|---|---|---|---|
| 5.1-a | 订单列表 24 列 / 20 Scan | ✅ | **IMP** / P0 | Developer | AGENTS §2.1、§6.1；CONVENTIONS §8 | 直接修 + 列表读取测试；独立提交，不与协议改造混提 |
| 5.1-b | 售后迁移唯一键先于回填 | ✅ | **IMP** / P0 | Developer | Spec §9.4 backfill；CONVENTIONS §6.2 | 调整迁移顺序或拆两步；用历史多明细售后副本演练 + 回滚（`make rehearse-c-migration` 类） |
| 5.1-c | Helm 开关不闭合 | ✅ | **IMP** / P1 | Developer | AGENTS §4「多仓」；§6.3 | 比 review 更严重：chart 没有五个 SPEC-0001 开关的任何注入点（quantity/carrier/auto-confirm/shortage/freight），也没有 `extraEnv`；K8s 上这些特性无法从仓库资产开启。补 values/模板 + `helm template` 断言；否则「UAT 验证过」不可复现 |
| 5.2-a | CREATED 可 ConfirmReceipt；争议驳回只写 receipt 不更新 shipment | ✅ | **BIZ+IMP** / P1 | PO（澄清）→ Dev | Spec §6.8/S7；AC-20/22 | PO 用 EARS 明确「IF 未发货 THEN 用户确认收货 SHALL 被拒绝」；Dev 加状态守卫并统一 receipt 迁移原语 |
| 5.2-b | auto-confirm 多分支更新 job 后直接 return，事务未 Commit；LOST/DAMAGED 仍可能 T7 | ✅ | **IMP** / P1 | Developer | AGENTS §0.3、§6.2 | 统一 `commit` 出口；异常轨迹阻断 T7；补故障注入（job 卡 pending、重复 DELIVERED） |
| 5.2-c | 无 `shipment.dispute_resolved` consumer；字段不满足 ADR §18.8 | ✅ | **ARC+IMP** / P2 | Architect → Dev | ADR-0005 §18.8；registry fail-closed | 先定 dispute_id/status/时间字段契约，再补 consumer 与 registry；验收：争议裁决后 AfterSales/运营动作可达 |
| 5.2-d | `ChooseShortageResolution` 忽略 RowsAffected；重复上报可重开已解决案件 | ✅ | **IMP**（+ARC 版本语义）/ P2 | Developer | Spec AC-30/D-05；AGENTS §4 | CAS 检查 RowsAffected；`ON DUPLICATE` 不得把 RESOLVED 拉回 NOTIFIED；补并发双选测试 |
| 5.3-a | 行台账 pending 生命周期/永久失败释放口径 | 📄 | **IMP** / P1 | Developer | ADR-0005 §6.4；Spec INV-24 | 统一「预占 + 已退 / 明确释放」迁移；人工重算必须恢复 pending；补台账重算测试 |
| 5.3-b | restock/refund worker 未用 lease owner fencing | 📄 | **IMP** / P1 | Developer | AGENTS §4「全局幂等性」 | 所有完成/释放 SQL 带 `lease_owner=owner`；失租 worker 不得写聚合与事件 |
| 5.3-c | Restock 授权不校验 quantity/line_no，`used_at` 早于库存成功 | ✅ | **IMP** / P1 | Developer | ADR §18.5；AGENTS §0.3 | 授权校验补数量与行号；`used_at` 在库存成功后条件写；失败可重试 |

### 4.5 安全 / AI（review §6、§8）

| # | 问题 | 复核 | 类别/优先级 | 所有者 | 规则落点 | 下一步 |
|---|---|---|---|---|---|---|
| 6 | broad namespace ALLOW + 不完整 DENY；未覆盖 Pricing 写方法；live 只验一个方法 | ✅ | **SEC** / P1 | Architect（策略）→ Dev（渲染）→ Reviewer（live UAT） | AGENTS §5「权限安全」；§6.2 | 收窄 ingress 来源，恢复显式服务间 ALLOW + AI 精确只读 ALLOW，DENY 仅作第二道；全服务写面静态矩阵 + live 反例。未跑 live 前只能记「未验收」 |
| 8.1 | AI 无稳定 `turn_id/action_id`、session 无 revision、取消/预算/反馈语义缺失 | 📄（已核实 session/recent-draft/feedback；取消与预算未逐行） | **SEC/AI** / P3 | Architect | AGENTS §5「协议优先」「可用性隔离」 | 单独 AI ADR；不阻塞交易项；BFF 取消传播与预算软/硬上限先定语义 |
| 8.2 | RAG 构建失败 exit 0、不完整扫描做 destructive reconcile、缓存/版本注册表缺陷 | ✅（构建/扫描）；📄（缓存/版本） | **SEC/AI + IMP** / P3 | Architect → Dev | AGENTS §5；§0.3 | 构建失败非零；扫描不完整禁止删除；缓存覆盖实际写版本；再补 base 隔离与回收 |

### 4.6 验收与治理（review §9）

| # | 问题 | 复核 | 类别/优先级 | 所有者 | 规则落点 | 下一步 |
|---|---|---|---|---|---|---|
| 9.1-a | 6 条 deferred AC 无执行者；G-4 未实现 | ✅ | **GOV** / P1 | Architect（已裁决 T-003）→ Dev（实现）→ Reviewer（A1…A5） | CONVENTIONS §6.3、§7.2；ADR-0006 v1.1 | 按 T-003 实现 G-4/`--release`/`SCAN RANGE`；到期未清阻断对应范围。**禁止**删行或批量续期 |
| 9.1-b | T-001 写「最终放行 / 不影响 PASS」 | ✅ | **GOV** / P1 | Reviewer | CONVENTIONS §5、§4.1 | 重新出具有范围 verdict；只保留「该范围实现审查 PASS」，撤回发布语义；不回写历史线程事实 |
| 9.1-c | AC-38 浅测；拆单并发/故障脚本未进 `verify` | ✅ | **GOV** / P1 | Reviewer + Dev | CONVENTIONS §6.1、§6.2；AGENTS §6.2 | Reviewer 判定断言不足并回退/延期；拆单并发脚本纳入组合门禁的证据链，而不是 `verify` 全跑 |
| 9.1-d | P0-7 金额断言被降级为 `[TODO] 请 Dev 忽略` | ✅ | **GOV** / P1 | Reviewer | AGENTS §1.5（不得把资金类降级）、§6.2 | 撤销「不影响 PASS」表述；资金相关断言不得与 Lint 同级，必须补多组金额序列或显式延期 |
| 9.1-e | MEMORY 同时写动态状态与「门禁已过」 | ✅ | **DOC** / P3 | 各角色 | CONVENTIONS §1 末段 | 删除动态状态，只留稳定经验 + canonical 链接 |
| 9.1-f | G-1 对 range 做 Role 并集；默认 `HEAD^..HEAD`；CI 不含 docs/prompt | ✅ | **GOV+IMP** / P2 | Architect（规则）→ Dev（脚本/CI） | CONVENTIONS §7.1/§7.2；ADR-0006 | 规则已定，按 T-003 实现；CI paths 至少加 `docs/collab/**`、`docs/specs/**`、`docs/adr/**`、`prompt/**`，否则文档漂移永远不过门禁 |

### 4.7 文档漂移（review §1）

| # | 问题 | 复核 | 类别/优先级 | 所有者 | 规则落点 | 下一步 |
|---|---|---|---|---|---|---|
| D1 | ADR-0002 状态行仍写 phase 5/6 未完成 | ✅ | **DOC** / P3 | Architect | CONVENTIONS §2.1 | 用 Amendment 更新状态行，指向 review-tracking 的 phase 5/6 交付 |
| D2 | `c-warehouse-review.md:100-106` 的 “functionally complete” | ✅ | **DOC** / P2 | 文档 owner（Architect） | CONVENTIONS §2.1 | 改为「dev/UAT 功能可用；支付/事件/恢复阻断项未清，不具备发布条件」 |
| D3 | ADR-0005 §3.7「用户胜，撤销收货」与 §7.4 冲突 | ✅ | **DOC** / P2 | Architect | CONVENTIONS §2.1；Spec P-07 | 修 §3.7 加「确认前」限定；Spec 无需改 |
| D4 | ADR-0005 §18.8 `shipment.received` 必填字段无组号 | ✅ | **ARC** / P1 | Architect | ADR §18.8 + registry | 与 4.2 同一 Amendment 处理，不单独补字段 |
| D5 | `architecture-baseline.md` 快照 HEAD=4e0b8b4 已过期 | ✅ | **DOC** / P3 | Architect | CONVENTIONS §2.1 | 更新快照或显式标注「历史快照」 |

---

## 5. 规则矩阵：哪条规则处理哪类问题

| 规则条款 | 覆盖项 | 说明 |
|---|---|---|
| `AGENTS.md` §4「分布式事务与持久恢复」 | 2.2、3.1、3.2、3.3 | 已有铁律；缺的是 ADR 与实现，不是新规则 |
| `AGENTS.md` §4「事实所有权与履约准入」 | 2.3、2.4、2.5、3.4 | 四事实分离 + permit 唯一裁决已写明；review §2.5 的「加 readiness `if`」被此条禁止 |
| `AGENTS.md` §4「多仓身份」「渐进兼容」 | 4.1、4.2、4.3、5.1-c | 缺仓 fail-closed、版本化事件、双写/影子消费 |
| `AGENTS.md` §4「全局幂等性」 | 2.3、5.2-b、5.3-b | DB 唯一约束/CAS 兜底；lease fencing 属实现 |
| `AGENTS.md` §1.3 + §1.4 | 全部 ARC | Architect 必须写恢复所有者/假设；Dev 遇协议缺口必须停手反证 |
| `AGENTS.md` §1.5 + §6.2 | 3.4、9.1-b/d、全部重灾区 | Reviewer 不得把资金类降级为 `[TODO]`；必须有并发/故障证据 |
| `CONVENTIONS.md` §5 + §4.1 | 9.1-b、9.1-d | 四种状态不可混用；verdict 必须带范围与 `release_status` |
| `CONVENTIONS.md` §6.1/§6.3 | 9.1-a/c、全部 AC | AC-REF ≠ AC-VERIFIED；延期是债务不是证书 |
| `CONVENTIONS.md` §7.1/§7.2 + ADR-0006 | 9.1-a/f | 只保留少量客观门禁；机器不能判断言深度 |
| `CONVENTIONS.md` §2.1 + §1 | D1–D5、9.1-e | 描述性文档以代码为准；MEMORY 不放动态状态 |
| `SPEC-0001` INV-17…29 + AC-01…38 | 2.4-b、3.3、3.4、4.2、5.2-a/d、5.3-a | 已有业务契约；缺实现或需 PO 澄清边界 |
| `architecture-baseline.md` INV-01…16 | 2.5（INV-06，仅 TCC 路径）、3（INV-04）、4.3（INV-09） | 基线不变量被违反，属「回归」而非「新需求」 |
| `AGENTS.md` §5 | 6、8.1、8.2 | AI 可选面；权限/Mesh 与 Turn/Action 协议 |

---

## 6. 规则缺口（仅 2 条建议）

### G1（建议）把 baseline_impact 提升到 INV 级
- **真空**：`CONVENTIONS §3` 要求跨服务变更写 `baseline_impact`，但没有要求列出受影响的 **INV-01…29**；也没有任何机制保证「早期 hardening 要求」持续成立。review 的 §2.5/§3 就是这样漏过整个 SDD 周期的。
- **最小修订**：Reviewer 线程元数据（§4.1）增加 `INV:` 字段；`baseline_impact` 必须写「INV-xx 受影响 / 不受影响 + 证据」。不新增机器门禁（符合 §7.1）。
- **所有者**：Architect 提案，人类批准（流程原则变更，CONVENTIONS §1）。

### G2（建议）QC 报告增加「崩溃窗口清单」
- **真空**：`AGENTS §1.2` 要求 QC 检查并发/超时/崩溃/幂等，但没有可执行的窗口枚举，导致「本地提交后崩溃」「跨服务 RPC 成功后崩溃」「外部渠道不再重放」三类窗口连续漏审（review §2.2/§2.3/§3.1）。
- **最小修订**：在 `prompt/Quality-Checker.md` 加 5 个必答窗口（本地提交后崩溃、RPC 成功后崩溃、重复消费、取消竞态、外部重放缺失）。属角色边界文件，需人类批准（CONVENTIONS §1）。
- **不新增** G-5/G-6 机器门禁：这些是语义判断，机器只能制造虚假安全感（§7.1）。

---

## 7. 执行顺序与分发

### 7.1 需要人类先裁决（H1–H6）

| # | 决策 | 影响 |
|---|---|---|
| H1 | 是否把「首次支付必须足额、本期不做部分支付」定为硬不变量 | 决定 SPEC-0002 的 EARS 与 2.1/2.3 的实现方向 |
| H2 | 是否同意「支付结算与履约准入」作为 P0 单独立项，期间冻结 split/quantity/auto-confirm 开关 | 决定发布口径与并行度 |
| H3 | 指定 payment×order×inventory×fulfillment 的 integration owner | CONVENTIONS §3 硬要求 |
| H4 | 是否撤回/限定 `c-warehouse-review` 的 “functionally complete” 表述 | 发布叙事与对外口径 |
| H5 | AI/Mesh 是否让位于交易收敛 | 决定 AI-0…AI-5 排期 |
| H6 | 是否批准 G1/G2 两条规则修订 | 流程变更 |

### 7.2 分发（不按 review 编号，按契约边界）

1. **Dev 立即并行（不动协议）**：5.1-a 列表、5.1-b 迁移、5.1-c Helm；各自独立提交 + 测试日志 + 自检清单。→ `developer-reviewer/T-002`。
2. **PO → QC → Architect**：H1 裁决后，PO 出 SPEC-0002（支付结算与履约准入的业务不变量 + EARS AC，含四事实、足额支付、退款准入、缺货 48h 默认、未发货不可确认收货）；QC 按 G2 清单红队；人类批准。
3. **Architect**：ADR-0007（支付结算与履约准入：settlement task、CAS 幂等判据、统一 freeze、订单级资金口径、permit 裁决、三入口统一退款、四事实事件）；同时出 ADR-0005 Amendment（事件 v2 身份、dispute 字段、receipt 原语）与 ADR-0008（多仓读模型/Catalog 投影）。
4. **Dev**：按 ADR 一次实现，禁止按条补丁；故障矩阵见 §7.3。
5. **Reviewer**：先跑 T-003 A1…A5 与 G-4，再对 T-001 重出有范围 verdict；Release 结论必须绑定显式 `--release --range` 与真实执行证据。
6. **Security/AI**：Mesh 收敛与 live UAT 由 Architect+Dev 在交易项之后排；AI Turn/Action 单独 ADR。

### 7.3 验收矩阵（跨 review §2/§3 的组合，不是单点）

| 场景 | 必测 |
|---|---|
| Callback 提交后崩溃 / DTM 未受理 | Payment SUCCEEDED 但 Order 未结算，重启后由 settlement worker 收敛 |
| Confirm 预读后取消 | 唯一胜者；CANCELLED 不得触发 afterPaid；已捕获资金必须退款 |
| Inventory Confirm 持续失败 | Order 不得进入可履约；不得留下 HELD 却可 ship |
| 多 intent 不同金额/channel | 订单级 captured/refunded 口径一致；FULL 判定正确 |
| 退款三入口 × 崩溃点 | Block 前/后、task 提交前、Payment 超时后，均可从持久任务恢复 |
| 单组/多组 × CREATED/SHIPPED × 并发 Ship | permit 唯一胜者；缺货组退款撤销该组运单；已发货组退款不被误伤 |

---

### 7.4 H1–H6 建议（待人类批准，2026-11-23）

| # | 建议 | 理由（证据） | 配套条件 / 退出条件 | 落点 |
|---|---|---|---|---|
| **H1** | **批准，但把语义钉死**：首次支付 `captured == pay_cent`；同一订单只允许一个生效支付，重复支付唯一胜者、非胜者退款；`pay_cent=0` 免捕获。本期**不做部分支付**；将来要做必须另立 Spec/ADR，禁止用 `paid <= pay_cent` 隐式支持 | BFF 当前传 `pay_cent`（`gateway/checkout_handlers.go:165-170`），所以不是「HTTP 可少付」，而是权威末端缺少 fail-closed（`order/service.go:1272-1279`、`payment/service.go:94-121`），与 `AGENTS §0.3` 冲突 | 少付/超付/零额/重复 intent/并发双回调 5 类负例 + 五方终态核对 | SPEC-0002 的 INV + EARS AC；ADR-0007 定义 Payment→Order 身份校验与重复支付协议 |
| **H2** | **批准 P0 立项，但把「冻结」拆成三层**：①生产开关保持 false（现状）；②UAT 可保留 `warehouseMode=split` 做测试，但**不得作为发布证据**；③P0 分 A/B/C——A 资金正确性（足额、settlement task、CAS 幂等、统一 freeze、订单级资金口径）、B 履约/退款准入（四事实、三入口、permit）、C 三个确定性发布错误（列表/迁移/Helm，可立即修） | 默认结算路径就是 TCC（`payment/tcc.go:145-148` 未设置时返回 true；Helm 默认 `"true"`），而 TCC 正是 CAS/freeze 缺陷所在；同时五个 SPEC-0001 开关在 Helm 中根本没有注入点，「UAT 验证过」无法从仓库资产复现 | 解除冻结必须同时满足：§7.3 矩阵全绿 + G-4 通过 + 显式 `--release --range` + 五方终态；否则不写 Release PASS | 人类发布冻结通知；`change_id=PAY-SETTLE-01`；ADR-0007；Helm 补全五开关 + `helm template` 断言 |
| **H3** | **Architect 任 integration owner，指定一名支付/订单 Dev 作 deputy**；不按四个服务拆 owner | `CONVENTIONS §3` 允许现有服务 Owner 或 Architect 兼任；本轮漂移正是四个服务各改各的 | owner 需在每次变更写 `change_id/base/spec_version/adr_revision/baseline_impact/affected_AC/allowed_paths`；负责五方终态核对表；不写业务代码 | 线程元数据与 deviation register；需人类提供实际姓名（无姓名时记 `Architect (acting)`） |
| **H4** | **撤回表述，但保留历史**：改为「phase 1–6 happy path 已在 dev/UAT 验证；支付结算、事件身份、恢复、Catalog 投影阻断项未清，不具备发布条件；生产保持关闭」，并链接本 triage §4 | `CONVENTIONS §2.1`：描述性文档与代码冲突以代码为准；原句在「功能面」上不算错，但被用于发布叙事即误导 | 不删原句，加 Amendment/脚注；与 D2 一次改完 | `docs/c-warehouse-review.md`（Architect） |
| **H5** | **分层让位**：①Mesh 安全收窄**现在就做**（不是 AI 功能，是权限漏洞）；②生产 `ai.enabled` 默认改 **false**，直到 Mesh live UAT 通过；③AI-0 可在 P0 后做设计但不得新增写权限，AI-1…5 暂缓；④RAG 的 exit code/destructive reconcile 随运维小修走 | `values.yaml:133-135` 默认 `ai.enabled=true`；`authorizationpolicy.yaml:12-29` 是 namespace-wide ALLOW，`values.yaml:303-377` 无 Pricing 写方法 DENY；`AGENTS §5` 把权限安全列为硬约束，AI 本身是可选面 | Mesh：先补 Pricing 等写方法到 DENY 作临时第二道 + 静态收窄；live 反例 UAT 排下一个集群窗口；未跑前只记「未验收」 | `values.yaml` / `authorizationpolicy.yaml` / `check-ai-mesh.sh` / AI ADR |
| **H6** | **两条都批，最小实现，不加 G-5/G-6**：G1 在 §4.1 元数据加 `INV:`，`baseline_impact` 必须列 INV + 证据（「不涉及」必须写依据），复用 deviation register 不新建台账；G2 在 QC prompt 加 5 个崩溃窗口必答（允许「N/A + 理由」，不得留空）。另把「外部复审引用必须可考」并入 G1 证据规则，不单列 | 本轮 3 个同类崩溃窗口连续漏审；review 的 `trae/specs`、`ADR-0086`、`Makefile` 引用缺陷说明外部输入也需要证据门槛；`CONVENTIONS §7.1` 明确不新增机器门禁 | G1 需 Architect 改 CONVENTIONS §4.1/§3；G2 需改 `prompt/Quality-Checker.md`；均属角色/流程边界，按 §1 需人类批准 | `CONVENTIONS.md`、`prompt/Quality-Checker.md` |

> **裁决（2026-11-23）**：人类批准 H1–H6 全部建议；H3 integration owner = Director，Architect 作设计 owner/秘书。后续 C-1（SPEC-0001 v1.3）、C-8（D-11…D-21）、C-9（落库）同轮批准，SPEC-0002 v1.6 已入库。记录见 `docs/review-tracking.md` 的「2026-11-23 human decisions」段。

> **不建议做的事**：在 P0 修复前把 `PAYMENT_TCC_ENABLED` 切成 false 当作「修好了」——它只缓解 2.3/2.4，不解决 2.2（callback 提交后崩溃仍无 settlement worker），且会让两条路径的故障矩阵只验证一条。若确有出货压力，应作为**显式风险接受**记录，而不是默认动作。

---

## 8. 附：本轮实际读取的证据（抽样）

- 支付/订单：`internal/services/payment/{service.go,tcc.go}`、`cmd/payment/main.go:31-35`、`internal/services/order/{repo.go,tcc.go,service.go,worker.go,settlement.go,fulfillment_groups.go,consumer.go,refund_consumer.go,shortage.go}`。
- 库存/履约：`internal/services/inventory/{operations.go,restock.go}`、`internal/services/fulfillment/{consumer.go,service.go,delivery.go,refund_consumer.go}`、`internal/services/aftersales/{service.go,shortage_consumer.go,workers.go,refund_lines.go,restock_auth.go}`。
- 投影/事件：`internal/services/catalog/stock_consumer.go`、`cmd/catalog-stock-rebuild/main.go`、`internal/platform/events/registry.go`。
- AI/部署/治理：`internal/services/ai/{session.go,service.go,feedback.go,rag.go}`、`cmd/ai/main.go`、`deploy/k8s/charts/mall/{templates/authorizationpolicy.yaml,templates/deployments.yaml,values.yaml}`、`scripts/{check-ai-mesh.sh,collab_gates.py}`、`.github/workflows/go.yml`、`Makefile`。
- 契约/流程：`AGENTS.md`、`docs/collab/CONVENTIONS.md`、`docs/adr/{0002,0005,0006}`、`docs/specs/0001`、`docs/architecture-baseline.md`、`docs/review-tracking.md`、`docs/collab/ac-coverage.tsv`、`docs/collab/*/threads/T-00{1,2,3}.md`、`docs/{architect,developer}/MEMORY.md`、`prompt/*.md`。

**【测试范围】** 静态代码/文档核对与交叉引用；未执行任何测试、迁移、Helm 渲染或 live 验证。
**【未覆盖范围】** 真实测试结果、历史数据分布、线上开关、Istio 实际转发、外部支付渠道重试、模型/承运商行为。
