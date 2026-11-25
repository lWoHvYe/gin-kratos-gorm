# PAY-SETTLE-01 任务重拆分审查

| 项 | 值 |
|---|---|
| Status | **APPROVED (simplified) — 2026-11-25；在当前分支继续** |
| 触发 | T1.4 越改越多；Reviewer B1 `[ARCHITECTURAL_FLAW]`（group 数量 vs order reservation） |
| 决策 | 人类接受方案 A：先重拆分，不再在 T1.4b2 上叠补丁 |
| 范围 | P0-A/P0-B + T1–T5 的重新分解；不写代码 |
| change_id | 仍为 `PAY-SETTLE-01`；子任务用 `work_package` 标识，不新开 change_id 绕过冻结 |

## 1. 当前任务盘点

| 旧任务 | 当前状态 | 实际问题 |
|---|---|---|
| T1.2/T1.3 TCC batch/permit/resume | WIP，T1.2-R2 有范围 PASS | 只到 `REFUNDING + batch/apps`，无打款 |
| T1.4a 准入/CancelIfCreated/零元 | WIP `6be5c5a` | 零元、PERMIT、POST 已做；INV-44 未做 |
| T1.4b1 打款 | WIP `fc295e5` | Payment 批退款/预检/worker；未最终 PASS |
| T1.4b1-R2 reconcile/order projection | PASS（`198c623..6ac218e`） | 只解决事件丢失与金额投影 |
| T1.4b2 release/zero | FAIL `401a67d..3597989` | B1 group/reservation 身份不一致；B2 source；B3 回填 |
| T2 历史 PAID | 未开始 | 依赖 Payment 解析/人工队列 |
| T3 行级/责任运费 | 未开始 | 依赖 commitment lines/责任 owner |
| T4 fail-closed/shadow | 未开始 | 与 T1 解耦，可独立 |
| T5 证据矩阵 | 未开始 | 最后集成，不应与功能混做 |

## 2. 为什么原来会“越改越多”

1. **按代码层拆，而不是按不变量拆**：准入、打款、worker、release 分别拆，但每个都跨 Order/Payment/Inventory/Fulfillment。
2. **四个不变量被打包进 T1.4**：
   - 资金原子性（不能部分打款）；
   - 事件恢复（丢失后 order 投影收敛）；
   - 库存身份（group vs order reservation）；
   - 历史兼容（已有 PRE/S19 回填）。
3. **契约前置缺失**：group reservation 在 ADR/Plan 中不存在，直到 T1.4b2 才暴露。
4. **单分支滚动**：所有 WIP 在同一分支，后续修复叠在前一层上；Reviewer 只能对混合范围裁决。
5. **“先做再发现”**：每次 Reviewer/架构发现缺口，就新增 Amendment 和修复切片，形成设计债分期付款。

## 3. 新拆分原则

1. **一个 WP 只拥有一个核心不变量**；跨服务必须显式契约，不得靠代码默契。
2. **先契约，后迁移，再代码**；迁移与消费者必须在同一 WP。
3. **禁止 WP 同时改 money + inventory identity**；必须拆。
4. **禁止在 Reviewer FAIL 后继续同一 WP 叠补丁**；先重拆分/重审 Plan。
5. **禁止“或等价/必要时”**；所有 schema、事件、状态必须定稿。
6. **回填永远在生产者修复之后**；不得先洗历史数据。
7. **一个 WP 一个分支**；集成分支只接收 Reviewer PASS 的 WP。
8. **测试矩阵按 WP 制定**；AC-VERIFIED 只能由 Reviewer 写唯一台账。

## 4. 新工作包（WP）

### P0 资金与订单

| WP | 内容 | Owner | 契约/迁移 | 依赖 | 退出标准 |
|---|---|---|---|---|---|
| **WP-R1 Payment Refund Authority** | `RefundBatch` 原子、`GetRefundByKey`、重复/非生效、`0012` | Payment Dev | C-04 accepted；C-05 payment/0012 | 无 | Payment Reviewer PASS |
| **WP-R2 Order Refund Ledger** | `refund_applications`/batch/commitment、TCC 准入、预检、`RefundBatch` 调用、lazy/worker、order 投影 | Order Dev | C-05 order/0017/0018/0021；无新契约 | R1 | Order Reviewer PASS；不接 Inventory |
| **WP-R3 Zero After-Sales** | 零元短路 + AfterSales task/入口 | AfterSales Dev | AC-67/68；C-05 0021 | R2 | Zero Reviewer PASS；不调 Payment |

### P0 库存身份与物理释放

| WP | 内容 | Owner | 契约/迁移 | 依赖 | 退出标准 |
|---|---|---|---|---|---|
| **WP-I1 Single-group Release** | **P0 只对单组订单发整笔 release**；多组退款准入成功但不释放库存，保持 reserved，P1/人工；不做 scan/last-active | Order + Inventory | 无新迁移；`release_scope=order_reservation_full` | 无 | Order/Inventory Reviewer PASS |
| **WP-I2 Refund Release (INV-44)** | 单组 Order outbox、Inventory consumer；整笔释放；PRE-only；幂等/DLQ；多组不写 | Order + Inventory | release event accepted；无新迁移 | I1 + R2 | Reviewer PASS |
| **WP-I3 Restock/Return** | 退货回库、restock 授权/执行 | AfterSales + Inventory | 现有 restock | I1 | Reviewer PASS；可与 I2 并行 |

### P0/P1 历史与兼容

| WP | 内容 | Owner | 依赖 | 退出标准 |
|---|---|---|---|---|
| **WP-B1 Historical Release Backfill** | **DEFERRED**：当前 flags false、未上线、生产无 PRE/S19 数据；不做 drift worker；如 dev/UAT 有数据只做一次性清理 | Order Dev | I2 | 人类书面确认“无生产数据”即可关闭 |
| **WP-B2 Historical PAID Resolution** | `settlement_payment_no` 空、`ListOrderIntents`、人工队列、回填 | Order + Payment | R1 | Reviewer PASS |
| **WP-E1 `refund_requested` observe-only** | 消费者停止直接 Refund，Amendment 先行 | Payment Dev | R2 | Reviewer PASS |
| **WP-E2 PENDING/CANCELLED+captured** | CAS→CANCELLED + 支付级 cancel 退款，Amendment 先行 | Order + Payment | R1/R2/E1 | Reviewer PASS |
| **WP-E3 Fail-closed clients** | Inventory/Payment/Fulfillment 构造期必填/启动 fail-fast | Platform Dev | 无 | build/启动测试 |
| **WP-E4 `order.settled` shadow** | 三态开关、默认 paid、不切换 | Fulfillment Dev | E3 | Reviewer PASS，不改默认 |

### 交叉验证

| WP | 内容 | 依赖 |
|---|---|---|
| **WP-V1 Evidence & Release** | AC-39…68 矩阵、真实 migrations、`-race`/故障注入、五方终态 | 所有 P0 WP |
| **WP-X1 0012 split chain** | 独立轨道，不并入 PAY-SETTLE-01 | Architect/人类 |

## 5. 依赖与顺序

```text
R1 Payment ──> R2 Order Ledger ──> R3 Zero
                  │
                  ├──> E1 observe-only ──> E2 PENDING/captured
                  │
I1 Group Reservation ──> I2 Refund Release ──> B1 Backfill
                  │
                  └──> I3 Restock
E3 fail-closed ──> E4 shadow
B2 Historical PAID (parallel, after R1)
V1 after P0
```

- R1 与 I1 可并行，但 Order 文件必须切分（reservation/confirm vs refund ledger）。
- I2 必须等 I1 + R2；不得用旧 order-level reservation 先发 release。
- B1 永远在 I2 之后；B2 不依赖 I2。
- E3/E4 与资金/库存解耦，可独立。

## 6. 分支与集成分支策略

人类决定 **2026-11-25：不新拆分支，当前 `wip/pay-settle-01-phase1` 继续**。以下原分支方案作废：

- 当前分支继续；每个 WP 用 `work_package: WP-xx` trailer 与独立 commit 范围隔离。
- Stop-the-line 仍按 WP：Reviewer FAIL 后该 WP 冻结，不得在同一 WP 继续叠代码。
- 不新开分支；`main` 只在 **P0 收尾 + 人类 merge approve** 后合入。

## 7. 当前 WIP 处置

| WIP | 处置 |
|---|---|
| Payment `RefundBatch`/`GetRefundByKey` | 归 WP-R1；需最终 Reviewer；不直接合并 |
| Order T1.2/T1.4a/T1.4b1-R2 | 归 WP-R2；已有 scoped PASS，但需在 WP 分支重审 |
| Zero `2a8bc17/3597989` | 归 WP-R3；需在 WP 分支重审 |
| T1.4b2a release `d5c4a7f/f58757a` | **冻结**；I1 完成后适配，不得按旧 reservation 继续 |
| T1.4b2b/b2 | 归 WP-R3，保留 |
| `0007-amd1-addendum-t14b2-inv44-group.md` | 作为 WP-I1 设计输入；QC/Architect 后 accepted |
| 当前混合分支 | 快照/tag；不再直接加功能 |

## 8. 门禁与人类检查点

- 每个 WP：契约/迁移 -> QC/Architect -> Dev -> Reviewer；**WP 内不得绕过**。
- Amendment 临时豁免：仍记录在 review-tracking；QC/Architect 仍需要。
- `main`/Release：P0 收尾后人类批准；不得自动。
- AC-VERIFIED：只由 Reviewer 在唯一台账写；本拆分不改变。
- 每个 WP 的 Reviewer PASS 只覆盖该 WP，不是整体 PASS。

## 9. 防止再次膨胀的检查表

在开一个新 WP 前必须回答：

1. 它只拥有一个核心不变量吗？跨服务契约是否已定？
2. 是否同时改 money + inventory identity？若是，拆。
3. 是否需要先改 schema/事件/消费者？若是，先契约 WP。
4. 是否有历史数据回填？回填是否在生产者修复之后？
5. 预计 Reviewer 可能因几个独立不变量 FAIL？>1 个就拆。
6. 是否能用一句话说清退出标准？不能就拆。
7. 是否在 FAIL 后继续同一 WP 叠补丁？若是，停止并重拆分。

## 10. 建议的下一步

1. 人类确认本重拆分（不改代码）。
2. 将 `0007-amd1-addendum-t14b2-inv44-group.md` 交 QC/Architect，定稿 WP-I1。
3. 开 `wip/ps-i1-group-reservation`（只做身份/迁移/Reserve/Confirm/Cancel，不接 release）。
4. WP-R1/R2/R3 各自独立 Reviewer；不再混在 T1.4b2 上。
5. I1/I2 完成后再做 B1；B2/E3/E4 可并行。
6. P0 全部 PASS 后，人类决定 merge。

## 11. 2026-11-25 简化裁决（Human）

- **不做 group-scoped reservation**，不改 Inventory schema。
- 多组订单部分组退款：库存延迟到**最后一个共享该 reservation 的活动组结束**才整笔释放；单组立即释放。
- **不做 B1 历史 PRE/S19 回填**：flags false、未上线、生产无此类数据；dev/UAT 如有数据只做一次性清理。
- **不新拆分支**：当前 `wip/pay-settle-01-phase1` 继续；用 `work_package` 隔离 commit。
- 保留 RefundBatch、reconcile worker、零元 task；不追求“每组立即释放库存”的完美语义。

## 12. 2026-11-25 进一步简化（Human）

- P0 **只对单组订单**发 physical release；多组订单退款准入成功但不释放库存，保持 reserved，P1/人工。
- 移除 deferred scan、last-active、TCC group REFUNDED、watermark/cursor、manual queue、生产回填。
- 单组立即 full release；多组记日志/告警，不写 outbox。
