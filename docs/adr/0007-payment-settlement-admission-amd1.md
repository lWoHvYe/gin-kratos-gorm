# ADR 0007 Amendment 1 — 准入后生命周期、历史支付解析、TCC Σ 预检与 `order.settled` 影子

| 项 | 值 |
|---|---|
| Status | **superseded by ADR-0007 Rev 2.3 (2026-11-25)**——原 accepted（Director 2026-11-24 批准；Amendment 1）；条款已逐条并入 Rev 2.3，本文件仅保留历史/审计证据，不再作为实现依据；§3.2 已按 Rev 2.2/2.3 启动 fail-closed/N5 修复同步 |
| Superseded by | [ADR-0007 Rev 2.3](0007-payment-settlement-admission.md)（2026-11-25；merge map 见该文件 §Merge map；原 C-1/C-2 冲突已于 2026-11-25 关闭，见该文件 §18） |
| Approved | **2026-11-24** |
| Base | **ADR-0007 Rev 1.0**（accepted，2026-11-23）；本文件**不修改** Rev 1.0 正文 |
| Revision | **1.0**（Amendment 1；原 r5 定稿；QC-13/QC-14 收口：Payment 成功后才终态） |
| Addendum | [0007-amd1-addendum-zero-after-sale.md](0007-amd1-addendum-zero-after-sale.md)（accepted 2026-11-24） |
| T1.2 Addendum | [0007-amd1-addendum-t12-contract-rulings.md](0007-amd1-addendum-t12-contract-rulings.md)（accepted 2026-11-24） |
| T1.4a Addendum | [0007-amd1-addendum-t14a-contract-rulings.md](0007-amd1-addendum-t14a-contract-rulings.md)（accepted 2026-11-24） |
| C-04 RefundBatch | [0007-amd1-refundbatch-c04-draft.md](0007-amd1-refundbatch-c04-draft.md)（accepted 2026-11-24） |
| Date | 2026-11-24 |
| Target Spec | **SPEC-0002 v1.8（approved 2026-11-24）**；QC-13/QC-14 已复核；AC-63 零元精确条件、AC-66 三级开关门禁、`COMPENSATING` 已对齐；SPEC-0001 v1.4（approved 2026-11-24，C-01） |
| Change ID | `PAY-SETTLE-01` |
| Integration owner | **Director**（人类）；deputy: TBD |
| Design owner | System Architect |
| 触发 | Plan C-03；QC/Reviewer 对 P-04/P-05/P-06 的阻断；Plan D-1…D-5；本 Amendment 的 D-27…D-29 |
| 覆盖 | AC-62、AC-63、AC-64、AC-65、AC-66；INV-45…INV-49；S15…S18；C-03 全部条目 |
| 生效前置 | ① 本 Amendment **accepted 2026-11-24**；② SPEC-0002 v1.8 **approved 2026-11-24**；③ C-04/C-05 契约与迁移评审通过。未满足前不得实现。 |
| 与 Rev 1.0 的关系 | 本文件是**增量 Amendment**。凡本文件明确“替代/收窄/补充”的条款，以本文件为准；未列出的 Rev 1.0 条款继续有效。Rev 1.0 原文不删除、不重写。 |

> **生效声明**：本文件是 **ADR-0007 Amendment 1（accepted 2026-11-24）**，不是实现依据；SPEC-0002 v1.8 已 approved，实现前仍需 C-04/C-05 评审通过。Rev 1.0 的已批准正文不被本文件重写。

---

## 0. 阅读方式与规范边界

1. **只增不改**：不删除、不重编号 Rev 1.0 的 D-01…D-14、INV/AC、状态机与表；只新增本文件条款。
2. **Spec 优先**：本 Amendment 的业务语义以 SPEC-0002 v1.8（批准后）为唯一来源；ADR 只定义实现协议，不自行改变业务规则。
3. **代码现状不是契约**：若代码与 SPEC/ADR 冲突，以 SPEC/ADR 为准；本文件对 D-29 的 `GRANTED` 漂移即按此处理。
4. **不写代码**：本文的 DDL 是**schema 约束设计**，不是迁移文件；C-05 由 Developer 在获批后实现。
5. **可回滚**：新增表/列均为 additive；不做 down migration；回滚通过代码/开关与既有任务重放完成。

---

## 1. 决策摘要（A-01…A-14）

| ID | 决策 | 映射 |
|---|---|---|
| A-01 | **准入不可逆**：permit 终态不可恢复；commitment 只能按 AC-62 的合法触发 `RESERVED→RELEASED`，且必须人工授权、审计、幂等、CAS/lease；禁止 worker 静默释放 | AC-62 / INV-45 / D-1 |
| A-02 | **退款执行账本**：`refund_applications` 由 **Order** 拥有，记录组级退款申请与 Order 发起的支付级退款意图；AfterSales 是业务发起方但通过 `AdmitRefund` 写入；Payment 是执行方，不写 Order 账本 | C-03 / AC-62 / AC-64 / S8/S15 |
| A-03 | **行级与组成**：commitment 必须有行/商品/运费组成；新承诺 `composition_known=1`，行合计必须等于头金额；历史 `composition_known=0` 只允许整组退款，禁止行级部分退款直到人工补齐 | AC-64 / INV-24 / INV-47 |
| A-04 | **历史解析队列**：`legacy_payment_resolution` 由 Order 唯一写；`order_no` 唯一；状态 `OPEN/RESOLVING/COMPENSATING/RESOLVED/EXPIRED_REFUND/EXPIRED_ZERO/EXPIRED_FINANCE/REJECTED`；24h 到期先 `COMPENSATING`（非 OPEN），补偿后持久化退款意图并 `Payment.Refund`，**Payment 成功后才 `EXPIRED_REFUND`**；不得无限期 `OPEN` | AC-63 / S16 / D-28 |
| A-05 | **TCC 先预检后改状态**：事务外取 Payment 权威快照；Tx1 订单锁下复核 Σ；不等不写任何 permit/commitment/订单退款态；Tx1 同事务写 REFUNDING + batch + 分状态 permit | AC-65 / INV-48 / D-27（Plan） |
| A-06 | **分状态 permit 冻结**：`NONE→REVOKED`（CAS）；`CONSUMED` 不得写 `REVOKED`，只写 `PENDING_REMOTE` admission+commitment，Tx1 后 `CancelIfCreated`；Ship 胜出→`POST_SHIP_ALLOWED`；已离仓组只退款 | AC-65 / AC-50 / Rev1.0 §5.2 |
| A-07 | **单一定稿人**：Order 的 `payment.refunded` 消费者是 commitment `RESERVED→SETTLED` 的唯一写入方；`AdmitRefund` 前的惰性对账是补偿路径；Payment 提供按 `refund_id` 查询退款事实的只读 API | AC-62 / AC-55 / Rev1.0 §4.3 |
| A-08 | **`order.refund_requested` 退役为观察**：保留 subject 兼容；Payment 消费者改为只观察/告警，不再 `Refund`；新 TCC 不依赖该事件；移除日期在 ON/消费方迁移后由 Director 定 | D-5 / AC-62 / INV-36/43 |
| A-09 | **`order.settled` 影子**：R1 只实现 `paid|shadow` 两态，生产默认 `paid`；shadow 双消费、只记录/比对、不建单；`settled` 需 release 门禁+人类批准 | AC-66 / D-27 / INV-49 |
| A-10 | **切换/回滚/移除**：OFF→SHADOW 需 Director 影子批准；SHADOW→ON 需 SPEC §7/G-4/五方/`--release --range` + Director；ON 必须记录 `order.paid` 移除日期（建议 ≤ON+30d，超出需 Director 书面批准）；回滚 ON→SHADOW/paid，不重复建单 | AC-66 / D-27 |
| A-11 | **`GRANTED` 收敛**：v1.7 许可枚举仅 `NONE/CONSUMED/REVOKED`；代码 `GRANTED` 既非持久合法值也不得作为第二状态；预检有行则阻断部署，手工按证据分类；未知状态 fail-closed | D-29 / v1.7 §2/§5.2 |
| A-12 | **API/配置增量**：`AdmitRefund` 增加 `repeated RefundLine`、责任/理由/运费字段、`batch_id`；新增 `Payment.GetRefundByKey`（或 `Refund.refund_id`）；新增 Order 历史队列管理/恢复命令；`FULFILLMENT_SETTLE_CONSUMER_MODE`（paid/shadow/settled）与 `SETTLE_RELEASE_APPROVAL_ID`；全部 additive、不重编号 | C-04 / AC-62/63/64/66 |
| A-13 | **新增结构**：`refund_batches`、`refund_applications`、`order_refund_commitment_lines`、`refund_commitment_releases`、`order_refund_sequences`、`legacy_payment_resolution`、`settlement_shadow_diffs`、`settlement_switch_approvals`；`order_refund_commitments` 增组成列 | C-05 |
| A-14 | **迁移范围**：D-27 的 shadow 表在 Fulfillment DB，C-05 的 `allowed_paths` 必须扩展到 `migrations/fulfillment/**`；否则 D-27 无落点（Plan 的允许路径当前只含 payment/order） | D-27 / C-05 |

---

## 2. 映射表

### 2.1 Rev 1.0 条款 → 本 Amendment

| Rev 1.0 | 本 Amendment 处理 |
|---|---|
| §1.1 事实所有权（RefundAdmission/commitment） | §4.2、§4.3 补充 S15/行组成；不改变事实分离 |
| §1.2 `order.settled` 字段 | §3.1 定义 v1 必填/可选字段、版本与兼容 |
| §2.4 / §3.1 结算双发 | §3.1 只读；影子不改变双发与结算 |
| §3.3 / §3.4 取消 | §4.8 明确 `PENDING_PAYMENT+已捕获` 的支付级退款 |
| §4.3 退款额度/commitment | §4.2、§4.3、§4.7 补充行/组成、释放、惰性对账 |
| §4.4 历史 `PAID` | §5.2 以 SPEC-0002 AC-63 为准（候选 `captured == pay_cent`；零元；队列） |
| §5.1 退款分类/`refund_applications` | §4.2 定义 Order-owned 账本；§4.6 支付级例外 |
| §5.2 `AdmitRefund` 两阶段 | §4.1/§4.6 明确 CONSUMED 不写 REVOKED；NONE→REVOKED 分包冻结 |
| §5.5 TCC | §4.1 增加 batch/Σ 预检/冻结/续跑 |
| §5.4 支付级退款 | §4.6 维持 Payment 执行；§4.8 取消/历史例外 |
| §12.1 表结构 | §6 增补/变更 |
| §12.3 灰度与开关 | §3.2/§3.3 以 D-27 为准（R1 不切换） |
| §13 假设 | §9 更新 |

### 2.2 C-03 → 本 Amendment → AC

| C-03 条目 | 本 Amendment | AC |
|---|---|---|
| TCC batch/扇出 | §4.1 | AC-65 |
| Σ 预检（事务外快照 + 锁内复核） | §4.1.2 | AC-65 |
| permit 冻结顺序（NONE/CONSUMED 分状态） | §4.1.3 | AC-65 / AC-50 |
| `refund_applications` | §4.2 | AC-62 / AC-64 |
| `legacy_payment_resolution` | §5 | AC-63 |
| commitment 行/商品运费组成 | §4.3 | AC-64 |
| 消费者切换（`order.refund_requested`） | §4.6 | D-5 / AC-62 |
| `order.settled` shadow | §3 | AC-66 / D-27 |
| `GRANTED` 收敛 | §5.4 | D-29 / v1.7 §2/§5.2 |

### 2.3 AC-62…66 实现映射

| AC | 关键实现点 | 失败语义 |
|---|---|---|
| AC-62 | 释放命令只接受人工授权；`RESERVED→RELEASED` CAS；permit 不恢复；`SETTLED` 优先收敛 | `REFUND_RELEASE_NOT_AUTHORIZED` / `REFUND_COMMITMENT_CONFLICT` |
| AC-63 | 事务外解析；CAS 写回；0/多候选队列；24h 先 `COMPENSATING`（离开 OPEN）→ 先撤 permit/取消 CREATED 运单/INV-44 释放 → 持久化退款意图 → `Payment.Refund`；**Payment 成功后才 `EXPIRED_REFUND`**；恢复命令；零元精确条件（v1.8） | `LEGACY_PAYMENT_UNRESOLVED` |
| AC-64 | 行级三层额度；商品/运费组成；责任枚举；168h 挂起；同组补退 | `REFUND_ADMISSION_CONFLICT` / `PARTIAL_REFUND_REQUIRES_FULL_GROUP` / `REFUND_RESPONSIBILITY_INVALID` |
| AC-65 | 事务外快照；Tx1 锁内复核；分状态 permit；batch 续跑；部分打款只补未完成 | `TCC_REFUND_SUM_MISMATCH` / `REFUND_ADMISSION_CONFLICT` |
| AC-66 | 影子双消费、只记录；默认 paid；切换门禁；回滚 | 影子运行期失败只告警；请求 `shadow`/`settled` 缺/过期/不匹配批准 → 启动失败（非零），不回退 `paid`（Rev 2.2） |

---

## 3. D-27：`order.settled` 影子、版本、比对与切换

### 3.1 事件字段与版本

**Subject 不变**：`shop.order.settled.v1`。R1 不新增 subject、不改 v1 的必填字段；只在同一 v1 内增加可选字段。

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

### 3.2 影子消费模式与开关

用**三态枚举**替代原布尔开关：

```text
FULFILLMENT_SETTLE_CONSUMER_MODE = paid | shadow | settled
默认 = paid（仅当未请求模式；R1 生产默认；等价于旧 false）
```

| 模式 | 行为 | 允许条件 |
|---|---|---|
| `paid` | 只消费 `order.paid`（旧路径）；`order.settled` 仍双发但不消费 | 未请求模式时的默认（或显式 `paid`）；无需批准 |
| `shadow` | 同时消费 `order.paid` 与 `order.settled`；**都只记录/比对，不建运单、不推进组、不放行** | Director 影子批准（`target_mode='shadow'`、`status='APPROVED'`；`expires_at` 非 NULL 时未过期）+ 影子就绪清单 |
| `settled` | 以 `order.settled` 建单/推进；`order.paid` 只 BFF/旧消费者 | SHADOW→ON 全量 release 门禁 + Director 批准（`target_mode='settled'`、`status='APPROVED'`、`expires_at` 非空且未过期、`release_range` 匹配） |

**启动校验（fail-closed）**：

> **Rev 2.2/2.3 同步（2026-11-25）**：请求 `shadow`/`settled` 时批准行缺失/过期/非 `APPROVED`/`range` 不匹配 → **启动失败（非零退出）**，不得回退 `paid`；**N5**：`SETTLE_RELEASE_APPROVAL_ID` 非空但未请求 `settled` → 启动失败（config error），不得默认 `paid`；`settled→paid` 只经 §3.4 的 Director 批准回滚。

1. 未设置 mode 且旧布尔未设置/`false`，**且 `SETTLE_RELEASE_APPROVAL_ID` 为空** → **`paid` 默认**（唯一允许的默认路径）。
   - **N5**：`SETTLE_RELEASE_APPROVAL_ID` 非空但未请求 `settled`（mode 未设置/空/`paid`/`shadow`）→ **启动失败（非零退出）**；approval-id 不得单独生效。
2. 未知/空/大小写不符 → **启动失败**（非零退出），不得默认 `paid`。
3. 请求 `settled`：`SETTLE_RELEASE_APPROVAL_ID` 必须非空，且同一次启动校验必须存在 `settlement_switch_approvals` 行：
   - `target_mode='settled'`、`status='APPROVED'`、`expires_at` **非空且未过期**、`release_range` 与本部署 release range 一致；
   - 任一不满足 → **启动失败（非零退出）**；不得进入 `settled`、不得降级 `paid`、不得告警后继续。
4. 请求 `shadow`：必须存在 `target_mode='shadow'`、`status='APPROVED'` 的批准行；`expires_at` 非 NULL 时不得过期。任一不满足 → **启动失败（非零退出）**，不得降级 `paid`。
5. 批准行只能由人类运维/受控迁移写入；应用无写权限。表在 Fulfillment DB。
6. 任何运行期修改 `FULFILLMENT_SETTLE_CONSUMER_MODE` 不热生效；必须重启并重新校验。脚本/自动任务无法绕过启动校验。
7. **旧布尔开关兼容**：`FULFILLMENT_CONSUME_ORDER_SETTLED=false`/未设置 → 未请求模式（`paid` 默认）；`true` → 视为请求 `settled`，同样必须通过 `SETTLE_RELEASE_APPROVAL_ID` 校验；**无/过期/非 `APPROVED`/range 不匹配 → 启动失败（非零退出）**，不得按 `paid` 运行。新旧配置同时存在且冲突 → **启动失败**（fail-fast），不静默择一。
8. **回退限制**：`settled→paid` 只能走 §3.4 的 Director 批准回滚 + 重启；不存在“批准行缺失/过期 → 自动回 `paid`”的路径。

### 3.3 影子比对口径

**权威源**：

- Order 读模型（`Order.GetOrder(Admin)`）为结算事实权威：`status=PAID`、`settlement_payment_no`、`paid_cent/pay_cent`、`allocation_hash`、`package_count/warehouse_count`、group 集合。
- `order.paid` 事件为兼容旁证：`order_no/payment_no/paid_cent/status`。
- Fulfillment 本地 shipment 只作观察，不作为结算权威；影子不创建 shipment。

**比对字段与判定**：

| 检查 | 数据源 | 差异类型（仅告警） |
|---|---|---|
| order 存在且 `status=PAID` | Order | `MISMATCH_ORDER_STATUS` |
| `event.payment_no == Order.settlement_payment_no` | 双方 | `MISMATCH_PAYMENT_NO` |
| `event.allocation_hash == Order.allocation_hash` | 双方 | `MISMATCH_ALLOCATION_HASH` |
| `package_count/warehouse_count` 相等且 ≥1 | 双方 | `MISMATCH_COUNTS` |
| `event.groups` 集合 == Order group 集合 | 双方 | `MISMATCH_GROUP_SET` |
| `order.paid` 同 `(order_no,payment_no)` 在窗口内存在 | 事件流 | `MISSING_PAID`（超时窗口默认 5 分钟） |
| `schema_version/必填字段` | 事件 | `UNKNOWN_VERSION` / `MISSING_FIELD` |
| Order RPC 失败 | Order | `ORDER_LOOKUP_FAILED` |
| 重复事件 | 事件 | 仅更新 `last_seen_at`，不记差异 |

- **不比较** `settled_at` 的严格等值（Order 当前用进程时钟；比较窗口仅用于延迟指标）。
- 差异只写 `settlement_shadow_diffs`、告警、对账；**不得阻断结算、不得放行/阻断发货、不得自动修复**。
- 影子差异率（按小时/订单量）是 SHADOW→ON 门禁输入；具体阈值由 Director 在影子批准时写入批准行，不在代码中硬编码。
- `settlement_shadow_diffs` 唯一键 `(order_no, payment_no)`；`state ∈ MATCH|MISMATCH|ERROR`；`diff_fields JSON` 记录差异字段集合；重复事件累加 `version`、更新 `last_seen_at`。

### 3.4 切换步骤、移除日期、回滚

**步骤（R1 不执行 1–4；只交付代码与默认 paid）**

0. **R1 部署**：`mode=paid`；`shadow`/`settled` 代码存在但未启用；行为不变。
1. **OFF→SHADOW**：Director 批准影子（`settlement_switch_approvals` 行，`target_mode='shadow'`）；双消费、只记录/比对；不建单。观察期由 Director 指定（建议 ≥7 天且覆盖多组/零元/历史单样本）。该步骤使用**影子批准 + 影子就绪清单**，不套用 SHADOW→ON 的全量 release 门禁；PO 对 AC-66 的修订将同样区分这两级。
2. **影子验收**：无 `UNKNOWN_VERSION/MISSING_FIELD/MISMATCH_*` 阻断；差异率低于批准阈值；`order.paid` 与 `order.settled` 对平记录可查。
3. **SHADOW→ON**：必须同时满足 SPEC-0002 §7 矩阵、G-4、五方终态核对、`--release --range`、Reviewer/QC 证据、Director 批准；`mode=settled`；`order.paid` 停止建单/推进组，只保留 BFF/旧消费者兼容。
4. **ON 记录**：`target_mode='settled'` 的同一批准行写入逻辑字段 `order.paid_admission_retired_at`（SQL 列 `order_paid_admission_retired_at`）与 `order.paid_subject_removal_date`（SQL 列 `order_paid_subject_removal_date`，移除日期，定义见 §6.6），**建议 ≤ ON+30 天**；未填写不得 ON；超过 30 天必须 Director 书面批准并记录理由。
5. **移除**：移除日期到达且 BFF/旧消费者迁移完成后，单独立项停止 `order.paid` 生产/消费；本 R1 不移除 subject。

**回滚**：

- `ON→SHADOW/OFF` 需 Director 批准 + 回滚演练/事件单；该回滚不套用 SHADOW→ON 的全量 release 门禁，但必须记录批准与原因；保留任务、退款申请、影子差异记录，可重放；不做 down migration。
- 回滚后若恢复 `order.paid` 建单，必须经过许可、库存、缺货判定，并以 `(order_no, group_no)` 唯一运单防重；已存在运单不得重复创建。
- `shadow→paid` 直接回退，不改变数据；`settled→shadow` 后 shadow 只记录，不建单。
- 任何回滚不得把已由 `settled` 建立的运单/许可回滚；只前向兼容。

---

## 4. C-03 设计：退款申请、commitment、TCC 与支付完成

### 4.1 TCC 补偿（AC-65）

#### 4.1.1 对象

- `refund_batches`：一次 TCC 补偿批次；`batch_id` 唯一；`(source, order_no, payment_no)` **允许多次 attempt，不是 UNIQUE**；同一时刻只允许一个 active batch（由 Tx1 订单锁 + state 检查保证）。
- `refund_applications`：每个活动组一条 `tcc:<payment_no>:<group_no>`；支付级取消/历史为 `group_no NULL`。
- `order_refund_admissions`：沿用 Rev 1.0；每 `refund_id` 唯一。
- `order_refund_commitments` + `order_refund_commitment_lines`：额度占用与组成。

#### 4.1.2 顺序（强制）

```text
(1) 事务外：Payment.GetIntent(payment_no) 取 captured/refunded 快照；
    读取 Order 冻结的行/freight_share；计算订单剩余 captured 与 Σ 活动组系统责任上限。
(2) 预检：不等 → 不写 permit/commitment/订单退款态；写 refund_batches(state=PRECHECK_FAILED)
    仅作审计 + 告警 + 人工；结束。
(3) Tx1（短事务，禁止远程调用）：
    - SELECT orders FOR UPDATE；
    - 用同一 Payment 快照复核 Σ；不等 → rollback，不写任何状态；
    - UPDATE orders.refund_state=REFUNDING（CAS：非 FULL）；
    - INSERT refund_batches(state=FROZEN) + 活动组 refund_applications(PENDING)；
    - 分状态 permit：
        NONE      → CAS NONE→REVOKED；
        CONSUMED  → 不写 REVOKED；写 order_refund_admissions(state=PENDING_REMOTE)
                    + order_refund_commitments(RESERVED) + 组行组成；
        已离仓（SHIPPED/RECEIVED）→ 仅写 application，permit 保持 CONSUMED；
    - 提交。
(4) Tx1 后：对 CONSUMED 组调用 Fulfillment.CancelIfCreated（幂等）；
    CANCELLED → Tx2 admission PRE_SHIP_REVOKED；
    ALREADY_SHIPPED/RECEIVED → Tx2 admission POST_SHIP_ALLOWED；
    TRANSIENT_ERROR → 保持 PENDING_REMOTE，worker 重试；不得打款。
(5) 逐组 AdmitRefund（NONE 已 REVOKED 的组幂等返回 PRE；其余按 AC-50）；
    任一未 ADMITTED → batch 不进入 PAYING，不调用 Payment。
(6) batch=PAYING：逐组 Payment.Refund(refund_id)；成功由 payment.refunded 收敛
    commitment SETTLED；失败保留 application/commitment，只重试未完成组。
(7) 全部组 SETTLED/已退款/零元 → batch=DONE；否则 MANUAL_HOLD/FAILED_PERMANENT。
```

#### 4.1.3 关键规则

1. **Σ 预检在状态变更前**；Tx1 用同一 Payment 快照在订单锁内复核；Payment 调用禁止在事务/锁内。
2. **`CONSUMED` 不得写 `REVOKED`**；只走 `PENDING_REMOTE` + `CancelIfCreated`；Ship 胜出 → `POST_SHIP_ALLOWED`；已离仓组不取消运单。
3. **batch 持久化**：崩溃重跑按 batch/组 application 续跑；已 `SETTLED` 组跳过；不得创建第二套 `refund_id`。
4. **部分失败**：已成功组保留 `SETTLED`；失败组保留 `RESERVED` + application；batch `MANUAL_HOLD`/可重试；不得重退已完成组。
5. **快照变化**：PAYING 期间每次打款前重新读取 Payment 权威；若 remaining captured 变化导致 Σ 不成立，停止后续打款、batch=MANUAL_HOLD、告警；已打款组不回滚。
6. **`PENDING_PAYMENT+已捕获`**：按 SPEC AC-65/AC-51 先 CAS→`CANCELLED`（与取消同一裁决点）；成功后**不进入组扇出**，改走支付级取消退款 `pay:<payment_no>:cancel`（见 §4.8）；CAS 失败进入 PAID 组扇出。
7. **无活动组**：幂等成功，不调用 Payment；历史无组按 G1；零元不进入 Payment。

### 4.2 `refund_applications` / `refund_batches` 所有权

- **Order 拥有** `refund_applications` 与 `refund_batches`（order DB）：Order 是准入与 commitment 的所有者；AfterSales 是普通售后的业务发起方，通过 `AdmitRefund` 让 Order 幂等登记；Order 是 TCC/历史支付级退款的发起方。
- 这不是为同一 `refund_id` 造第二条业务申请：AfterSales 的 `after_sales` 仍是业务申请；Order 的 `refund_applications` 是**执行/准入账本**，二者以 `refund_id` 关联。SPEC S8 的“唯一发起方”仍成立。
- **Payment 发起**的支付级退款（重复捕获/取消由 Payment 结算 worker 处理）继续由 Payment 的 `refunds` + `payment_duplicate_captures` 持久化；Order 不复制该事实。这收窄 Rev 1.0 §5.1 的“统一表”表述：统一的是 `refund_id` 可追溯与幂等，不是单一物理表。
- `refund_applications.state`：`PENDING/PROCESSING/DONE/FAILED_PERMANENT/CANCELLED/REJECTED`；`admission_result` 仅 `PRE_SHIP_REVOKED/POST_SHIP_ALLOWED/PENDING_REMOTE`；`source`：`aftersale/tcc/system_shortage/system_zero/payment/legacy_expired`。
- 同一 `refund_id` 只有一条；`(batch_id, group_no)` 对组级唯一；`group_no NULL` 允许支付级；唯一键见 §6。
- `refund_batches.state`：`PRECHECKED/FROZEN/ADMITTING/ADMITTED/PAYING/DONE/PRECHECK_FAILED/MANUAL_HOLD/FAILED_PERMANENT`；`PRECHECK_FAILED` 仅审计，不产生 permit/commitment。

### 4.3 commitment 组成与释放（AC-62/AC-64）

**头表新增**：`order_refund_commitments` 增 `goods_cent`、`freight_cent`、`composition_known`、`composition_source`；`amount_cent = goods_cent + freight_cent`。

**行表**：`order_refund_commitment_lines(refund_id, line_no, quantity, goods_cent, freight_cent)`：
- `line_no=0` 表示运费行（goods=0）；`line_no>0` 表示商品行（freight=0）。
- `SUM(goods_cent) = 头.goods_cent`，`SUM(freight_cent) = 头.freight_cent`；事务内校验。
- 新承诺 `composition_known=1`；历史迁移行 `composition_known=0`，`goods_cent=amount_cent, freight_cent=0`（仅占额度，不归行）。
- **含 `composition_known=0` 承诺的组只允许整组退款；禁止行级部分退款**，直到人工补齐组成；否则 `REFUND_ADMISSION_CONFLICT`。

**释放**：
- 只接受 AC-62 的两类触发（打款前 `REJECTED/CANCELLED`；`FAILED_PERMANENT`/人工放弃 + 授权）；每笔释放写 `refund_commitment_releases`（唯一 `refund_id`、`authorization_id`、金额、授权人/复核人/操作人、原因码、证据、有效期）。
- **D-24 授权矩阵**：金额 ≥1000 元（`100000` 分）必须两位**不同**授权人（`authorized_by` 与 `reviewed_by` 均非空且不相等）；低于阈值至少一位授权人；`operator` 只执行、不得替代授权。任一缺失/越权/过期/金额不符 → `REFUND_RELEASE_NOT_AUTHORIZED`，不改额度/状态。
- `UPDATE order_refund_commitments SET state='RELEASED' WHERE refund_id=? AND state='RESERVED' AND version=?`；0 行 → 读回当前状态，幂等或冲突。
- 释放金额必须等于头金额；禁止部分释放；`SETTLED` 优先：若 Payment 已退，先 `RESERVED→SETTLED`，不得释放。
- worker/脚本/重试入口无授权 → `REFUND_RELEASE_NOT_AUTHORIZED`；不提供内部自动释放开关。

### 4.4 行级、责任与 168h（AC-64）

- `AdmitRefund` 请求增 `repeated RefundLine{line_no, quantity, goods_cent}`、`responsibility`（封闭枚举）、`reason_code`、`freight_refundable_cent`、`batch_id`、`source`。
- Order 计算：
  - 行剩余 = 行实付 − AfterSales 已退（S14）− 该行在途承诺（S15 行组成）；
  - 组商品剩余 = Σ 行剩余；
  - 组运费上限 = 已退运费台账之外、按责任可退的 `freight_share`；
  - 组可退上限 = 商品剩余 + 可退运费；整组退款金额 = 商品剩余 + 本次可退运费；
  - 订单级 Payment `captured - refunded - RESERVED` 为最终上限。
- 责任由 AfterSales/缺货事实产出；Order 只校验枚举/证据并计算，不推断、不改责任；`UNDETERMINED` 商品照退、运费挂起，组保持 REFUNDING；168h 起算按 D-26“该组该次售后首次进入 UNDETERMINED”；到期按 `MERCHANT_WAREHOUSE` 同组补退；补退幂等、累计不超实付运费。
- 已离仓组允许部分；未离仓组仍要求整组（AC-50/D-19）；有 `composition_known=0` 的组例外按 §4.3 只允许整组。

### 4.5 支付级退款与历史 `EXPIRED_REFUND`

- 支付级退款（非生效捕获/超少付/取消）不经 `AdmitRefund`，只受 Payment captured 上限；`refund_id=pay:<payment_no>:<reason>`；`non_effective_capture=true` 时 Order 投影不累加订单级 refunded（INV-37）。
- **`EXPIRED_REFUND`（本 Amendment r4 强制顺序：24h `COMPENSATING`；先补偿、再持久化退款意图、再 Payment，Payment 成功后才终态）**：仅在“有捕获且无任何已离仓组”时进入；24h 到期**不得继续停留在 `OPEN/RESOLVING`**：
  1. 24h 到期：队列锁内 CAS `OPEN/RESOLVING→COMPENSATING`（version/lease），从此刻起**已离开 `OPEN/RESOLVING`**；记录 `expires_at`、`resolution_kind` 暂空、`blocked_scope` 保持禁发禁退。
  2. 在 `COMPENSATING` 持租约读取订单/组/运单；若任一已离仓或不确定 → CAS `COMPENSATING→EXPIRED_FINANCE`（保留审计，不取消运单、不退款）。
  3. 对每个未离仓组：permit `NONE` → CAS `NONE→REVOKED`（已 `REVOKED` 幂等跳过）；permit `CONSUMED` 且运单 `CREATED` → 事务外 `CancelIfCreated`（幂等）；运单 `CANCELLED`/无运单 → 继续；返回 `ALREADY_SHIPPED/RECEIVED` → 该组已离仓 → **CAS `COMPENSATING→EXPIRED_FINANCE`**，停止，不取消运单、不退款。
  4. 按 INV-44 释放未确认预留：`HELD` 直接释放，`CONFIRMED` 走 `CancelConfirm`；释放失败写持久任务 + 告警，**不阻塞退款完成**，但该组不得再发货（D-18）。
  5. **先持久化退款意图（Payment 之前）**：对每笔未退完的 captured intent 幂等写/复用 `refund_applications(source=legacy_expired, group_no NULL, refund_id=pay:<payment_no>:legacy_expired, amount=captured−refunded, goods_cent=amount, freight_cent=0, non_effective_capture=true, state=PENDING)`；`captured−refunded<=0` 的 intent 跳过；已存在同 `refund_id` 的行直接复用，不重复创建。
  6. **再调用 Payment.Refund（最后一步副作用）**：按 `refund_id` 幂等逐笔退款；每次成功后把对应 application 置 `DONE`；所有未退完 intent 均 `DONE`（或本来已退完）后，才 CAS **`COMPENSATING→EXPIRED_REFUND`**。`EXPIRED_REFUND` 表示 **Payment 退款已完成**，不是“仅补偿完成、打款未完成”。
  7. **打款失败/超时/未知**：**保持 `COMPENSATING`**；application 保持 `PENDING/PROCESSING`，告警并重试；**不得提前 CAS `EXPIRED_REFUND`，不得回 `OPEN/RESOLVING`**。重试前用 `GetRefundByKey` 确认；已退但本地未提交 → 补写 `DONE` 后再 CAS 终态；未退 → 重试；不得重复退款。
  8. **补偿/打款失败/失租**：保持 `COMPENSATING`，租约内重试 + 告警；不得回 `OPEN/RESOLVING`，不得提前退款；人工判定不可恢复时，CAS `COMPENSATING→EXPIRED_FINANCE` + 审计，不得由 worker 自动改终态。
  9. 已离仓组一律 `EXPIRED_FINANCE`，不得取消运单/不得回滚履约；permit/运单/INV-44 处置**先于** Payment；Payment 成功后终态；**不得出现“已退款又发货”**。
- 已解析出唯一生效支付后，若历史订单仍有其他已捕获 intent：按支付级退款或 `payment_duplicate_captures` 登记，不得静默留存；若生效支付尚未解析，则不得按本条自动退款，按 §5 队列分支处理（AC-63）。
- 该顺序对应 AC-63 与 E25；实现必须与 `AdmitRefund`/`BlockShipmentForRefund` 共用同一 permit/运单 CAS 语义，不得另写第二套裁决；测试须覆盖“`COMPENSATING` 不回到 OPEN、打款失败不提前 `EXPIRED_REFUND`、`EXPIRED_REFUND` 表示 Payment 已完成、`EXPIRED_REFUND` 后不可再发货/不可重复退款”。
- 支付级退款执行失败：application 持久重试/`FAILED_PERMANENT` + 人工；用户显示处理中。

### 4.6 `order.refund_requested` 消费者

- 保留 subject 与 registry 字段以兼容已投递消息；**Payment 消费者改为只观察/告警**，禁止调用 `Refund`；指标保留 `zero_amount/invalid/observed`。
- 新 TCC 路径不再以该事件作为退款触发器；TCC 只走 §4.1 batch/准入。
- 移除日期：在所有 BFF/BI/旧消费者迁移后，由 Director 在 release 记录中设定；不得早于 ON/退款迁移完成；本 R1 不移除 subject。
- 旧队列消息仍可被观察消费；不得因消费者变更丢失消息（ack 后仅记录）。

### 4.7 惰性对账与支付完成（A-07）

- **唯一写入方**：Order 的 `shop.payment.refunded.v1` 消费者按 `refund_id` CAS `RESERVED→SETTLED`；其他路径只读。
- **补偿**：`AdmitRefund` 在事务外对涉及订单的 `RESERVED` 承诺按 `refund_id` 查询 Payment 退款事实；已退 → 事务内 `SETTLED` 后重算；未退 → 继续准入判断。日终 worker 扫描陈旧 `RESERVED` 做同样收敛，告警。
- **Payment API**：新增 `GetRefundByKey(payment_no, refund_id)`（或 `Refund` 增 `refund_id`/`idempotency_key` 只读字段）；不得用 `refund_no` 猜测 `refund_id`。这是 C-04 的必填增量。
- 若 Payment 查询超时/不可用：按 fail-closed 阻断该笔准入/继续重试；不得把未知当未退或已退。

### 4.8 `PENDING_PAYMENT+已捕获` 的取消分支

- TCC 补偿入口先按 AC-46 同一裁决点 CAS `PENDING_PAYMENT→CANCELLED`；成功 → 发 `order.cancelled`（库存/优惠补偿）+ 写支付级取消申请 `pay:<payment_no>:cancel`（`source=payment`、`non_effective_capture=true`、`goods_cent=amount_cent`，Payment 执行，幂等）；**不创建组级 application、不进入组扇出、不改 permit**。
- CAS 失败 → 订单已 PAID；进入 §4.1 组扇出。
- **边界**：本分支仅适用于 `PENDING_PAYMENT` 且已捕获；历史 `PAID` 的 `EXPIRED_REFUND` 是另一分支（不写 `CANCELLED`、保留订单生命周期、先撤许可/取消运单/释放后支付），不得把两者混用。
- 取消退款与组退款可能并发时，`refund_id` 不同；Payment captured 上限与订单 `refunded` 投影保证不超退；Order 不得把支付级退款计入组额度。

---

## 5. D-28：`legacy_payment_resolution` 与“PAID 空 payment_no”

### 5.1 裁决

| 项 | 裁决 |
|---|---|
| 表 owner | **Order** 唯一写；Payment 只读；运营/Director 通过 Order 管理命令裁决 |
| 唯一键 | `order_no` 主键（一单一行）；`resolution_id` 终态时生成 `lpr:<order_no>`，唯一 |
| 触发 | `PAID` 且 `settlement_payment_no=''` 时，由 Order 的准入/发货/解析入口调用 `EnsureLegacyResolution`；读接口只观察，不建行 |
| 候选 | `ListOrderIntents` 事务外读取；`status=SUCCEEDED` + `captured == orders.pay_cent` + order 匹配；`orders.paid_cent` 若为 0 视为未回填、仅审计，若非 0 且 ≠ `pay_cent` 则转队列/告警（不自动采用）；**仅当 `pay_cent=0` 且无任何已捕获 intent 时**解析为 `zero:<order_no>`；**有捕获 intent 时不得 zero**：无离仓组按支付级退款，有离仓组/不确定进入人工队列或 `EXPIRED_FINANCE`，不得吞掉已捕获资金 |
| 并发 | 先读 Payment（事务外）→ Tx `SELECT orders FOR UPDATE` → 重读 `settlement_payment_no`；唯一候选 CAS `UPDATE ... WHERE settlement_payment_no=''`；0/多候选 `INSERT ... ON DUPLICATE KEY UPDATE order_no=order_no` 后 `SELECT ... FOR UPDATE`；终态不重开 |
| 24h | `expires_at = entered_at + 24h`（DB 时间）；worker 按 D-22 分支；**有捕获且无离仓组：先 `OPEN/RESOLVING→COMPENSATING`，不得继续 OPEN**；无捕获零元→`EXPIRED_ZERO`；有离仓/不确定→`EXPIRED_FINANCE`；过期/人工 `RESOLVED` 用 version CAS 只接受一个迁移；`COMPENSATING` 不是终态，`EXPIRED_REFUND` 仅在 Payment 成功后 |
| 阻断 | `OPEN/RESOLVING/COMPENSATING`：未离仓组禁发、全部组退款阻断（`LEGACY_PAYMENT_UNRESOLVED`）；`EXPIRED_FINANCE/REJECTED`：禁发禁退；`EXPIRED_REFUND`：禁发，不再新建组退款；`EXPIRED_ZERO/RESOLVED`：按正常路径 |
| `COMPENSATING` | **24h 后的补偿/打款中间态（非 OPEN/RESOLVING）**：持租约撤 `NONE` permit、`CONSUMED+CREATED` 走 `CancelIfCreated`、按 INV-44 释放；**先幂等持久化 `refund_applications(PENDING)`，再调用 `Payment.Refund`**；全部退款成功后 CAS→`EXPIRED_REFUND`；打款失败/超时保持 `COMPENSATING` + 告警/重试；Ship/未知/人工不可恢复→`EXPIRED_FINANCE`；不提前终态、不回 OPEN |
| `EXPIRED_REFUND` 顺序 | **Payment 完成后才进入**：此前的 permit 撤销/`CancelIfCreated`/INV-44 释放与 `refund_applications` 持久化均在 `COMPENSATING` 完成；`EXPIRED_REFUND` 表示所有未退完 captured intent 已由 Payment 退款成功；不是“补偿完成但打款未完成”；打款失败保持 `COMPENSATING` |
| 恢复 | `RESOLVED` 写回 payment_no + 生成 `resolution_id`；`ResumeLegacyResolution(order_no, resolution_id)` 幂等，只继续被阻断动作，不重复发货/退款；`COMPENSATING` 只由持租约 worker 继续 |
| 财务关闭 | `EXPIRED_FINANCE` 仅财务/授权角色 7 天内写 `closed_at`；状态保持终态、不重开；逾期升级授权角色，仍保持终态 |

### 5.2 状态机

```text
OPEN --运营开始--> RESOLVING --人工解析--> RESOLVED（终态）
OPEN/RESOLVING --24h 无捕获零元--> EXPIRED_ZERO（终态）
OPEN/RESOLVING --24h 无捕获有金额 / 有离仓 / 不确定--> EXPIRED_FINANCE（终态）
OPEN/RESOLVING --24h 有捕获且无离仓--> COMPENSATING
COMPENSATING --补偿完成且 Payment 全部成功--> EXPIRED_REFUND（终态）
COMPENSATING --Payment 失败 / 超时 / 未知--> COMPENSATING（重试 + 告警）
COMPENSATING --Ship 胜出 / 未知 / 人工判定不可恢复--> EXPIRED_FINANCE（终态）
COMPENSATING --补偿失败 / 失租--> COMPENSATING（租约内重试 + 告警）
OPEN/RESOLVING --脏数据人工--> REJECTED（终态）

禁止：COMPENSATING -> OPEN/RESOLVING；COMPENSATING -> 提前 EXPIRED_REFUND；
      EXPIRED_* -> OPEN；REJECTED -> OPEN；RESOLVED -> EXPIRED_*；删除记录
```

- `COMPENSATING` 是 24h 到期后的**非终端中间态**：它已离开 `OPEN/RESOLVING`，因此不违反 AC-63“不得继续 OPEN”；它不是终态，**不得提前退款**，也不得回 OPEN。补偿/打款失败保持该态并告警，由租约内重试；只有“补偿完成且 Payment 全部成功”才 `COMPENSATING→EXPIRED_REFUND`；`EXPIRED_REFUND` 表示 Payment 退款已完成，不是“仅补偿完成、打款未完成”。

### 5.3 并发与事务规则

0. **准入/发货入口**：`AdmitRefund` 与 `AuthorizeShipment` 在读取订单后先调用 `EnsureLegacyResolution`：
   - 返回 `RESOLVED`/`EXPIRED_ZERO` → 继续原动作（`EXPIRED_REFUND` 只允许已创建的支付级退款，不再新建组退款）；
   - 返回 `OPEN/RESOLVING/COMPENSATING` → 返回 `LEGACY_PAYMENT_UNRESOLVED`，未离仓组禁发、退款阻断；`COMPENSATING` 不提前退款。
   - 返回 `EXPIRED_FINANCE/REJECTED` → 返回 `LEGACY_PAYMENT_UNRESOLVED`，禁发禁退；
   - 唯一候选在 `Ensure` 内 CAS 写回后直接继续；不得在只读 BFF 查询中创建队列。
1. **所有写**（创建、状态迁移、`settlement_payment_no` 写回、终态退款申请）在 Order 事务内完成；远程 Payment 调用在事务外。
2. 队列创建与唯一候选写回共用 `orders` 行锁：先 Payment 读，再锁订单重读；锁内不得调 Payment。
3. `INSERT` 队列用 `ON DUPLICATE KEY UPDATE order_no=order_no`（或 `INSERT IGNORE`）+ `SELECT ... FOR UPDATE`；若已是终态，直接返回终态；若 `OPEN/RESOLVING`，只更新 `candidate_count/pay_cent/blocked_scope`，**不重置状态/expires_at**；若 `COMPENSATING`，幂等返回当前状态，不改状态/不重置租约。
4. 唯一候选：CAS `WHERE settlement_payment_no=''`；0 行 → 读回：
   - 与本候选相同 → 幂等成功；
   - 不同 → `SETTLEMENT_CONFLICT` + 进队列/告警，不覆盖。
5. `RESOLVED`：校验人工证据 → 锁队列 → `OPEN/RESOLVING→RESOLVED`（version CAS）→ CAS 写 `orders.settlement_payment_no` → 生成 `resolution_id` → 提交；写回冲突则回滚队列状态并转 `EXPIRED_FINANCE`/告警。
6. 24h worker：`SELECT ... WHERE state IN ('OPEN','RESOLVING') AND expires_at<=NOW(3) FOR UPDATE SKIP LOCKED` → 写 `lease_owner/lease_until` → 提交；事务外 `ListOrderIntents`；再 Tx 锁行、校验 `lease/version/state` → D-22 分支：
   - 有捕获且无离仓组 → **先 CAS `OPEN/RESOLVING→COMPENSATING`**（离开 OPEN）；在 `COMPENSATING` 持租约执行 §4.5 补偿（撤 permit/`CancelIfCreated`/INV-44 释放）；**再幂等持久化退款意图**；**再调用 `Payment.Refund`**；全部成功后 CAS `COMPENSATING→EXPIRED_REFUND`。
   - 无捕获零元 → `EXPIRED_ZERO`；无捕获有金额/有离仓/不确定 → `EXPIRED_FINANCE`。
   - 失租/状态已变 → 不写；补偿或打款失败 → 保持 `COMPENSATING`，不得回 OPEN/不得提前 `EXPIRED_REFUND`。
7. **打款失败/超时/未知**：保持 `COMPENSATING`；application 保持 `PENDING/PROCESSING`，告警并重试；不得提前 CAS `EXPIRED_REFUND`，不得回 `OPEN/RESOLVING`。重试前用 `GetRefundByKey` 确认；已退但本地未提交 → 补写 `DONE` 后再 CAS 终态；未退 → 重试；不得重复退款。`COMPENSATING` 期间发现 Ship 胜出/运单未知 → CAS `COMPENSATING→EXPIRED_FINANCE`，不得取消运单、不得继续退款。
8. `refund_applications` 必须**先于 Payment.Refund** 持久化（幂等、PENDING），Payment 成功后才允许 `COMPENSATING→EXPIRED_REFUND`；补偿前/打款前崩溃不得出现终态；`COMPENSATING` 不得提前终态；不得出现“已退款又发货”。
9. `EXPIRED_ZERO`：终态事务 CAS 写 `settlement_payment_no='zero:<order_no>'`；不调 Payment。
10. `REJECTED`/`EXPIRED_FINANCE`：不写退款、不放行发货；保留审计。

### 5.4 schema（D-28 必填约束）

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

- `resolution_kind`：`COMPENSATING` 期间为空；`RESOLVED` 为 `MANUAL`；`EXPIRED_ZERO/EXPIRED_REFUND/EXPIRED_FINANCE/REJECTED` 为同名值。
- `blocked_scope`：`OPEN/RESOLVING/COMPENSATING/EXPIRED_REFUND` 为 `UNSHIPPED_GROUPS_AND_REFUND`；`EXPIRED_FINANCE` 为 `FINANCE_HOLD`；`REJECTED` 为 `REJECTED_HOLD`；`RESOLVED/EXPIRED_ZERO` 为 `NONE`。
- `COMPENSATING` 不是终态，`resolution_id` 仍为 NULL；只有 `COMPENSATING→EXPIRED_REFUND`/`EXPIRED_FINANCE` 终态事务才写 `resolution_id`。

- `resolution_id` 仅终态写入；MySQL 唯一索引允许多个 NULL，故 `OPEN/RESOLVING/COMPENSATING` 不冲突。
- 状态迁移一律 `UPDATE ... WHERE order_no=? AND version=? AND state IN (...)`；0 行即败者，读回当前状态。
- `settlement_payment_no` 维持 `NOT NULL DEFAULT ''`；写回只用 `= ''`（不引入 NULL 语义），如需兼容 NULL 用 `COALESCE(...,'')=''`。

---

## 6. 新增/变更结构（C-05 设计约束）

以下为 schema 设计，不是迁移文件；C-05 由 Developer 落地并跑真实迁移。

### 6.1 `refund_batches`

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
- `PRECHECK_FAILED` 无 `refund_applications`，人工修正后创建新 attempt（`attempt_seq+1`，新 `batch_id`）。
- `MANUAL_HOLD/FAILED_PERMANENT` 已有 applications，重试时**复用同一 batch 与同一 `refund_id`**（state CAS 回 `PAYING/ADMITTING`），不得新建第二套退款标识。

### 6.2 `refund_applications`

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
  created_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
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
- 支付级行（`group_no NULL`）必须 `goods_cent = amount_cent`、`freight_cent = 0`，以满足 `goods+freight=amount` 的约束；支付级不参与组/行额度，只受 Payment captured 上限约束。
- `non_effective_capture`：支付级取消/历史 `EXPIRED_REFUND` 必须为 1，组级必须为 0；为 1 时 Order 投影不累加订单级 `captured/refunded`（INV-37），与 §4.5/§4.8 一致。

### 6.3 `order_refund_commitments` 变更 + 行表

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
- 新承诺 `composition_known=1`；缺行/合计不符 → 拒绝。

### 6.4 `refund_commitment_releases`

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

- `uk_release_refund` 保证每笔承诺最多一次释放；`uk_release_authorization` 防授权复用。
- 金额必须等于承诺头金额；不存在部分释放。

### 6.5 `order_refund_sequences`

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

### 6.6 `settlement_shadow_diffs` / `settlement_switch_approvals`（Fulfillment DB）

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
  expires_at     DATETIME(3)  NULL,
  order_paid_admission_retired_at DATETIME(3) NULL, -- settled/ON 时写；shadow 行 NULL
  order_paid_subject_removal_date DATE        NULL, -- settled/ON 时必填；移除日期
  status         VARCHAR(16)  NOT NULL DEFAULT 'APPROVED',
  created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (approval_id),
  KEY idx_switch_target (target_mode, status),
  CONSTRAINT chk_switch_mode CHECK (target_mode IN ('shadow','settled')),
  CONSTRAINT chk_switch_status CHECK (status IN ('APPROVED','REVOKED','EXPIRED')),
  CONSTRAINT chk_switch_removal CHECK (
    target_mode <> 'settled' OR (
      order_paid_admission_retired_at IS NOT NULL AND order_paid_subject_removal_date IS NOT NULL
    )
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- `settled` 行在 ON 批准时必须写 `order.paid_admission_retired_at`（SQL 列 `order_paid_admission_retired_at`）与 `order.paid_subject_removal_date`（SQL 列 `order_paid_subject_removal_date`）；`shadow` 行两列为 NULL。`order.paid_subject_removal_date` 为 `DATE`（日粒度），与 §3.4 的“建议 ≤ ON+30d”一致；超 30 天需 Director 书面批准。
- §3.4 的 ON 记录写入这两列；启动校验只读取 `target_mode='settled'` 且 `status='APPROVED'` 的行。

> **C-05 路径**：这两张表属 Fulfillment DB；Plan 的 `allowed_paths` 当前只含 `migrations/{payment,order}/**`，**必须扩展 `migrations/fulfillment/**`**，否则 D-27 无法落地。

### 6.7 其他既有结构

- `order_refund_admissions`：不改变主键/语义；`PENDING_REMOTE` 仍为中间态。
- `payment_settlement_tasks` / `payment_duplicate_captures` / `refunds`：不改变；支付级退款仍由 Payment 持久化。
- `order_shipping_permits`：D-29 收敛后增加状态 CHECK（见 §5.4），不新增列。

---

## 7. D-29：`GRANTED` 与 v1.7 许可枚举漂移

### 7.1 裁决

**以 SPEC-0002 v1.7 §2/§5.2 为准**：持久许可状态只有 `NONE / CONSUMED / REVOKED`。`GRANTED` **不是**合法状态，**不得**成为第二许可状态、不得作为“已授权/可发货”的别名。

当前代码事实：`permitGranted = "GRANTED"` 仅用于 `groupActions` 的 `permitOpen` 读判断（`internal/services/order/settlement.go:24,161`），无写入方；`AuthorizeShipment` 不接受 `GRANTED`。因此这是**读侧兼容容忍**，不是持久状态。

### 7.2 收敛规则

1. **迁移前预检（阻断部署）**：
   ```sql
   SELECT order_no, fulfillment_group_no, state, shipment_no, after_sale_no
   FROM order_shipping_permits
   WHERE state NOT IN ('NONE','CONSUMED','REVOKED')
   LIMIT 100;
   ```
   返回任何行 → **停止部署**，进入人工分类；不得自动改成 `NONE`（会允许二次发货）或 `CONSUMED`（会伪造已授权）。
2. **人工分类（有证据、可审计）**：
   - 若 `shipments` 存在该 `(order_no, group_no)` 且状态 `SHIPPED/RECEIVED`（唯一）→ 置 `CONSUMED` + `shipment_no`；
   - 若组 `refund_state ∈ {REFUNDING,REFUNDED}` 或 `after_sale_no` 非空 → 置 `REVOKED`；
   - 无 shipment 且无退款证据 → 才可置 `NONE`；
   - 任一不确定 → 保持 `REVOKED`（fail-closed）并人工处理；不得默认放行。
   - 每次分类写审计（操作人/时间/证据/旧值/新值）；不提供自动映射脚本。
3. **schema 约束**：分类完成后增加
   ```sql
   ALTER TABLE order_shipping_permits
     ADD CONSTRAINT chk_permit_state CHECK (state IN ('NONE','CONSUMED','REVOKED'));
   ```
   迁移在 CHECK 之前必须完成预检；有脏行则禁止加约束。
4. **代码收敛**：
   - 删除 `permitGranted` 常量与 `permitOpen` 中的 `GRANTED` 分支；
   - `permitOpen` 只把 `NONE`（以及“无行时刚初始化的 NONE”）视为可授权；
   - `ensurePermitTx` 遇到未知状态 → 返回 `SETTLEMENT_CONFLICT`/`FULFILLMENT_NOT_ALLOWED` 并告警，不得当作 `NONE`；
   - `AuthorizeShipment` / `groupActions` 对未知状态 fail-closed（不返回 ship）。
5. **回滚**：无 down migration；回滚代码会恢复旧读容忍，但 CHECK 仍阻止写入 `GRANTED`；若人工分类错误，按审计记录人工修正，不批量回滚。
6. **契约漂移记录**：在 ADR/实现偏差中记录“代码曾有 `GRANTED` 读容忍，v1.7 不承认该值；已按 Spec 收敛”，不以代码现状修改 Spec。

---

## 8. 状态机汇总

```text
Permit:        NONE --授权--> CONSUMED（终态）
               NONE --退款准入--> REVOKED（终态）
               CONSUMED -X-> REVOKED；REVOKED -X-> CONSUMED

Commitment:    RESERVED --Payment 退款成功--> SETTLED（终态）
               RESERVED --打款前 REJECTED/CANCELLED--> RELEASED（终态）
               RESERVED --FAILED_PERMANENT/放弃 + 人工授权--> RELEASED（终态）
               禁止：SETTLED->RELEASED；RELEASED->RESERVED；worker 自动释放

Refund app:    PENDING -> PROCESSING -> DONE
                          |-> PENDING（重试）
                          |-> FAILED_PERMANENT（人工/释放）
                          |-> CANCELLED / REJECTED（打款前）
Batch:         PRECHECKED -> FROZEN -> ADMITTING -> ADMITTED -> PAYING -> DONE
                         |-> PRECHECK_FAILED（仅审计）
                         |-> MANUAL_HOLD / FAILED_PERMANENT

Legacy queue:  OPEN -> RESOLVING -> RESOLVED
               OPEN/RESOLVING --24h 无捕获--> EXPIRED_ZERO | EXPIRED_FINANCE
               OPEN/RESOLVING --24h 有捕获无离仓--> COMPENSATING
               COMPENSATING --补偿完成且 Payment 全部成功--> EXPIRED_REFUND
               COMPENSATING --Payment 失败/超时/未知--> COMPENSATING（重试+告警）
               COMPENSATING --Ship胜出/未知/人工不可恢复--> EXPIRED_FINANCE
               COMPENSATING --补偿失败/失租--> COMPENSATING（重试+告警）
               OPEN/RESOLVING -> REJECTED
               禁止：COMPENSATING -> OPEN/RESOLVING；COMPENSATING -> 提前 EXPIRED_REFUND；终态 -> OPEN
```

---

## 9. 恢复窗口与 owner

| 窗口 | 持久意图 | 驱动方 | 人工入口 |
|---|---|---|---|
| TCC 预检后/Tx1 前后崩溃 | `refund_batches` + `refund_applications` | Order worker | batch_id / payment_no |
| CONSUMED 取消运单超时 | `order_refund_admissions(PENDING_REMOTE)` | Order worker + Fulfillment `CancelIfCreated` | refund_id |
| TCC 打款部分成功 | `order_refund_commitments(RESERVED)` + application | Order worker 只补未完成组 | refund_id / payment_no |
| commitment 已退未记 | Payment `refunds` + `refund_id` | Order `payment.refunded` 消费者 + 惰性对账 worker | refund_id |
| 历史解析 24h | `legacy_payment_resolution(COMPENSATING)` | Order expiry/compensation worker（补偿 + Payment 重试） | order_no / resolution_id |
| 队列终态退款 | `refund_applications(source=legacy_expired)` | Order worker → Payment | payment_no / refund_id |
| 影子差异 | `settlement_shadow_diffs` | Fulfillment shadow consumer | order_no / payment_no |

- 所有 worker 必须 `lease_owner/lease_until`（或 `FOR UPDATE SKIP LOCKED`）并在失租时拒绝写聚合/事件。
- 任务有最大尝试与 `FAILED_PERMANENT` 告警；禁止把消息重放/进程存活作为唯一恢复。

---

## 10. 迁移、兼容与回滚

- 新表/列全部 additive；金额 `BIGINT _cent`；状态用 `VARCHAR + CHECK`；唯一键如上。
- 历史 `order_refund_commitments` 按 §6.3 回填；`composition_known=0` 的组只允许整组退款，行级部分退款阻断（fail-closed），等待人工补齐。
- `order_shipping_permits` 的 `GRANTED` 预检与人工分类必须先于 CHECK；有脏行不得部署。
- 迁移使用 hardened runner（attempt/checksum 账本 + 备份）；MySQL 8 DDL 非事务，失败可重跑；不做 down migration。
- 回滚：
  - 代码回滚：保留新表/列；旧代码忽略新列/表；
  - 数据回滚：不允许回退状态机；人工审计修正；
  - 开关回滚：`settled→shadow/paid` 只改消费模式，不丢任务/退款申请；不重复建单/退款。
- `order.settled` 双发保持不变；影子/切换不改变结算主链路。

---

## 11. 【假设与不确定性】（强制）

1. **SPEC 基线**：本 Amendment 以 **SPEC-0002 v1.8（approved 2026-11-24）** 为靶；若后续 SPEC 修订改变 AC-62…66/INV-45…49，本 Amendment 必须重新映射，不得直接实现。
2. **Payment 退款查询 API 缺口**：当前 proto 只有 `ListRefunds`（按 payment_no/order_no，返回 `refund_no`），没有按 `refund_id` 或 `idempotency_key` 的查询；C-04 必须新增 `GetRefundByKey`（或给 `Refund` 增 `refund_id`）。在 API 落地前，惰性对账只能降级为“按订单列表 + 严格匹配/人工”，不得猜测。
3. **`settlement_payment_no` 语义**：当前列 `NOT NULL DEFAULT ''`，ADR Rev 1.0 写 NULL 兼容历史；本 Amendment 统一用 `''` 表示空，避免 `NULL` 比较陷阱。
4. **`order.settled` 负载**：当前只发送 `groups []string`；本 Amendment 不要求新增必填字段；影子比对依赖 `Order.GetOrder`，不是事件快照。若未来要事件自包含，必须 additive/版本化。
5. **影子期间不自动建单**：`shadow` 模式双消费、只记录，意味着自动建单暂停；需 Director 在影子批准时确认运营可接受手工/既有运单。
6. **`GRANTED` 数据**：代码无写入方；若生产存在历史行，按 §7.2 人工分类，无法自动推断。
7. **`EXPIRED_FINANCE` 终态禁发**：与 AC-60“不得永久阻断”按“不得无限期 `OPEN`”解释；若人类要求最终可发货，必须回 PO 升版，不得由实现自行放行。
8. **D-22/D-23 已批准**：24h 分支、7 天财务关闭、`EXPIRED_REFUND` 不写 `CANCELLED` 是既定业务裁决；ADR 不重新裁决。
9. **多仓**：P0 单仓 G1 兼容；`warehouse_id` 仍由既有字段携带；本 Amendment 不开启多仓。
10. **DTM/渠道**：不假设补偿顺序、渠道无限重放；以持久 batch/application/admission 为恢复入口。
11. **`allowed_paths` 缺口**：D-27 表在 Fulfillment DB；Plan 的 C-05 允许路径需扩展 `migrations/fulfillment/**`，否则 C-05 无法落地。
12. **`order.paid` 移除日期**：R1 不切换、不填具体日期；ON 批准时必须写入移除日期（建议 ≤ON+30d，超出需 Director 书面批准）；未写不得 ON。
13. **AC-63 零元精确条件**：SPEC-0002 **v1.8 approved 2026-11-24** 已含“`pay_cent=0` 且无任何已捕获 intent 才 zero；有捕获则退款/人工队列”；本 Amendment §5.1/§4.5 与其一致。后续修订若删改该条件，本 Amendment 须回 PO 重映射，不得作为实现依据。

---

## 12. 反模式自省（强制）

- **不新增 `if-else` 状态补丁**：许可只有三值；`GRANTED` 直接移除，未知状态 fail-closed，不做多状态猜测。
- **不把投影当准入**：commitment 只用于在途额度；最终上限始终 Payment `captured/refunded`；影子只告警。
- **不在 DB 事务内做远程调用**：TCC 预检 Payment 在事务外；Tx1 锁内只读本地快照；`CancelIfCreated`/Payment 在事务后。
- **不让 worker 静默释放**：释放必须人工授权 + 审计 + CAS/lease；`payment.refunded` 只做 `SETTLED`。
- **不重复退款**：`refund_id` 是唯一键；TCC batch 重跑跳过 `SETTLED`/已退组；历史 `EXPIRED_REFUND` 跳过已退 intent。
- **不用 `order.paid` 作准入**：R1 默认 paid 只是旧路径兼容；shadow/ON 不建单；切换必须 release 门禁。
- **不引入新基础设施**：无 Kafka/Temporal/新 ORM；沿用 MySQL/outbox/DTM/NATS。
- **不做 down migration**：前向兼容；回滚靠代码/开关/重放。

---

## 13. 影响面与 AC 映射

| 对象 | 影响 |
|---|---|
| INV-01/04/05/06/07/14/21/24/26/30…44 | 不改变；继续有效 |
| INV-45…INV-49 | 新增，按 SPEC v1.8 |
| S15/S16/S17/S18 | 新增；不合并 `orders.status` |
| AC-39…61 | 不重编号、不改文本；受影响 AC-50/51/52/53/60/61 回归 |
| AC-62…66 | 本 Amendment 实现映射 |
| 基线 AC-14/21/25/27/31/32 | 回归证据由 Reviewer 按 HEAD 登记；PO 不写台账 |
| 事件 | `order.settled` 同 v1 additive；`order.refund_admitted` 不变；`order.refund_requested` 保留但 Payment 只观察 |
| 开关 | `FULFILLMENT_SETTLE_CONSUMER_MODE` 三态；R1 默认 `paid`；`settled` 需批准 |
| 迁移 | `migrations/{order,payment,fulfillment}`；真实迁移测试；无 down migration |

---

## 14. D-27…D-29 裁决摘要（供 QC/Reviewer 关闭）

| ID | 裁决 | 关闭条件 |
|---|---|---|
| **D-27** | `order.settled` v1 必填/可选字段如上；`paid|shadow|settled` 三态，R1 默认 `paid`；shadow 只记录/比对；**OFF→SHADOW = Director 影子批准 + 清单；SHADOW→ON = 全量 release 门禁 + 人类批准**；ON 必须写移除日期（建议 ≤ON+30d，超出需书面批准）；回滚 ON→shadow/paid，不重复建单；启动用 `settlement_switch_approvals` 校验，非批准变更 fail-closed | AC-66 测试覆盖 shadow 不建单/默认 false/未批准回退/版本拒绝；Director 批准影子与 ON 记录 |
| **D-28** | Order 唯一写 `legacy_payment_resolution`，`order_no` 主键、`resolution_id` 唯一；`OPEN/RESOLVING/COMPENSATING/RESOLVED/EXPIRED_*/REJECTED`；24h 先 `COMPENSATING`（非 OPEN）→ 先撤 permit/`CancelIfCreated`/INV-44 释放 → 持久化 `refund_applications` → `Payment.Refund`；**Payment 全部成功后才 `EXPIRED_REFUND`**，打款失败/超时保持 `COMPENSATING` 告警/重试；已离仓/不确定→`EXPIRED_FINANCE`，不取消运单；`EXPIRED_REFUND` 表示 Payment 退款完成；ops/Director 裁决；Payment 只读 | schema 约束 + 并发/到期/打款重试测试；AC-63 证据 |
| **D-29** | 以 v1.7 为准，`GRANTED` 非合法状态；迁移预检有行即阻断；人工按证据分类；代码移除读容忍；未知状态 fail-closed；加 CHECK；无 down migration | 预检脚本 + 分类 runbook + AC-48/AC-50 回归；Reviewer 确认 |

---

## 15. 对 SPEC-0002 v1.8（approved 2026-11-24）的架构判断

**未发现需要打回 Architect 的 `[ARCHITECTURAL_FLAW]`。** v1.8 的 S15…S18、INV-45…INV-49、AC-62…AC-66 可被本 Amendment 实现；D-22…D-26 是已批准的 business 裁决，本 Amendment 不重新裁决。

但以下 4 个边界由 QC/PO 在实现前确认；它们不是本 Amendment 自行改变的业务规则：

1. **AC-60“不得永久阻断” vs `EXPIRED_FINANCE` 终态禁发/禁退**：本 Amendment 按“不得无限期 `OPEN`”解释；`EXPIRED_FINANCE` 是财务终态，但订单仍可能长期 PAID/不可发。若 QC/PO 认为这违反 AC-60，必须回 PO 修订 AC-60/AC-63，本 Amendment 随之重映射。
2. **零元 + 已捕获**：AC-63 的零元规则与 AC-40 的“零元单收到正金额捕获须全额退款”可能冲突。**SPEC v1.8（approved 2026-11-24）已含“`pay_cent=0` 且无任何已捕获 intent 才 zero；有捕获则按 AC-40/AC-39 退款 + 人工队列”；本 Amendment §5.1 与其一致**，不再保留宽解释。
3. **`refund_applications` 所有权**：本 Amendment 将 Order 作为组级/Order 发起退款的执行账本 owner，收窄 Rev 1.0 §5.1 的“统一表”；AfterSales 的业务申请仍归 AfterSales。若 QC/Reviewer 认为违反 S8“唯一发起方”，需回 PO 澄清“申请”与“执行账本”的边界。
4. **OFF→SHADOW / SHADOW→ON 门禁**：SPEC v1.8（approved 2026-11-24）已区分——`OFF→SHADOW` = Director 影子批准 + 影子就绪清单；`SHADOW→ON` = 全量 §7/G-4/五方/`--release --range` + Director；`ON→SHADOW/OFF` = Director + 回滚计划/事件单。本 Amendment 按同一映射，**不保留“任何开关都全量门禁”的宽解释**；若 PO 批准文本不同，以 PO 文本为准并重映射，不由实现选择。

上述任一项被 QC 判定为契约缺陷 → 回 PO 修订，本 Amendment 不得由实现自行选择；在修订前不得进入 C-04/C-05/实现。

## 16. 批准

- **Status: accepted**（Director 2026-11-24 批准；Revision 1.0 / Amendment 1）。
- **评审状态**：QC-13 已关闭 Payment/终态顺序；QC-14 结论为 **`READY FOR HUMAN SPEC+ADR APPROVAL`**（2026-11-24）；ADR 已获批准，SPEC-0002 v1.8 已 approved 2026-11-24；C-04/C-05 仍待各自流程。
- **SPEC 对齐**：SPEC-0002 **v1.8 approved 2026-11-24** 已显式写“先持久化 `refund_applications(PENDING)` 再 `Payment.Refund`”，与本 Amendment §4.5/§5.3 一致；后续修订必须保持该顺序。
- 实现前：不得写代码/迁移/测试，不得把本文件作为实现依据，不得切换 `order.settled`。
- 后续：C-04（proto）与 C-05（迁移）仍需各自评审；Developer 按 C-03/C-04/C-05 实现，Reviewer 按精确 base/head 复审。
- 本 Amendment 不改变 Rev 1.0 的已批准正文；所有变更以本文件增补条款为准。

## 17. 变更记录

| Revision | 日期 | 变更 | 影响 |
|---|---|---|---|
| 0.1-draft（r1） | 2026-11-24 | 初始 ADR Amendment 1 草案：D-27/D-28/D-29 裁决；C-03 映射；AC-62…66 实现协议；schema 设计；假设与反模式自省 | 新建，未生效 |
| **0.2-draft（r2）** | 2026-11-24 | 按 QC-11 整改：**B-1** `EXPIRED_REFUND` 增加“先撤未离仓 permit / `CancelIfCreated` / INV-44 释放，最后支付；已离仓→`EXPIRED_FINANCE`”顺序；**B-2** `refund_applications` 增 `non_effective_capture`，`settlement_switch_approvals` 增 `order.paid_admission_retired_at`/`order.paid_subject_removal_date`（SQL 列 `order_paid_*`），对齐 §3.4/§4.5/§6.2/§6.6；**B-3** §5.1 采用 AC-63 精确零元条件（无捕获才 zero；有捕获退款/人工队列；r4 已完成该修订）；**MAJOR#4** 明确 OFF→SHADOW（影子批准）与 SHADOW→ON（全量门禁）分级，移除“任何开关都全量门禁”的宽解释；并补 §4.8 PENDING/历史 PAID 边界 | Status 仍 DRAFT；未改 Rev 1.0/Spec/代码 |
| **0.3-draft（r3）** | 2026-11-24 | 按 QC-12 B-1 整改：`legacy_payment_resolution` 增 `COMPENSATING`；24h 先 `OPEN/RESOLVING→COMPENSATING`（离开 OPEN），补偿完成才 `COMPENSATING→EXPIRED_REFUND`，补偿失败保持 `COMPENSATING` 告警、不回 OPEN/不提前退款；`EXPIRED_REFUND` 仍先撤 permit/`CancelIfCreated`/INV-44 释放，Payment.Refund 最后。Target Spec 升为 **v1.8-draft-r4**，删除 B-3 待修订措辞；补 `resolution_kind/blocked_scope` CHECK 与 `refund_batches` 唯一性说明。Status 仍 DRAFT | 未改 Rev 1.0/Spec/代码（注：r4 已改为 Payment 成功后才 `EXPIRED_REFUND`） |
| **0.4-draft（r4）** | 2026-11-24 | 按 QC-13 B-1 整改：`COMPENSATING` 内先幂等持久化 `refund_applications(PENDING)`，再调用 `Payment.Refund`；**Payment 全部成功后才 CAS `COMPENSATING→EXPIRED_REFUND`**；打款失败/超时保持 `COMPENSATING` 告警/重试，不得提前终态、不得回 OPEN；撤 permit/`CancelIfCreated`/INV-44 释放仍在打款前。Target Spec 升为 **v1.8-draft-r5**；同步 §4.5/§5.1/§5.2/§5.3/§8/D-28 | Status 仍 DRAFT；未改 Rev 1.0/Spec/代码 |
| **0.5-draft（r5）** | 2026-11-24 | 按 QC-14 收口：Revision 升 r5；Target Spec 确认 r5；§16 引用 QC-13/QC-14 `READY FOR HUMAN SPEC+ADR APPROVAL`；记录“SPEC r6 将显式写先持久化 `refund_applications(PENDING)` 再 Payment”，与 §4.5/§5.3 一致；Status 仍 DRAFT | 未改语义/Rev1.0/Spec/代码 |
| **1.0（accepted）** | 2026-11-24 | Director 批准 ADR-0007 Amendment 1：Status accepted、Approved 2026-11-24、Revision 1.0；§16 引用 QC-13/QC-14；Target Spec 确认；记录“SPEC r6 将显式写先持久化 `refund_applications(PENDING)` 再 Payment”，与 §4.5/§5.3 一致 | 生效前置：SPEC v1.8 approved 2026-11-24 + C-04/C-05 评审；保留 r1–r5 变更记录 |
