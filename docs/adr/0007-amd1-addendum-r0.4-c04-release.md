# ADR-0007 Amendment 1 Addendum — R0.4 C-04 Release / Schema Ruling

| 项 | 值 |
|---|---|
| Status | **DRAFT — 未批准；R0.4 不进入实现** |
| Approved | — |
| Revision | **0.1-draft（r1）** |
| Tier | **L2**（跨服务事件/库存/迁移/启动门禁） |
| change_id | `PAY-SETTLE-01` |
| Base | ADR-0007 **Rev 2.3（accepted 2026-11-25）** + C-04 RefundBatch（accepted）+ T1.4a Rulings Addendum |
| Spec-Ref | **SPEC-0002 v1.10** INV-44/INV-45…50、AC-27/AC-50/AC-62/AC-65/AC-66；ADR Rev 2.3 §2.4/§5.7/§6.2/§6.5/§12.5 |
| allowed_paths | `internal/services/fulfillment/`、`cmd/fulfillment/`、`internal/platform/config/`、`migrations/order/`、`migrations/fulfillment/`、对应 `_test.go` |
| baseline_impact | C-04 release event（无 RPC）、R0.4 启动门禁、group_no/24h CHECK；不改变 C-04 RefundBatch |
| Scope | C-04 release RPC 判定 + R0.4 设计；不写代码/迁移/测试 |

---

## 1. C-04 release RPC：不新增

### 1.1 裁决

**C-04 不需要新增 release RPC。** Reviewer B1 “C-04 release RPC 缺”以 `tasks.md:21` 的 Plan 文案为依据，**越过已 accepted 的 ADR Rev 2.3**；ADR 已明确 release 是 **event/outbox**，不是 gRPC。

### 1.2 条款证据

| 条款 | 证据 |
|---|---|
| ADR Rev 2.3 §2.4（Inventory 释放请求） | `inventory 释放请求（组级）` 为**既有事件（C-04 增量字段）**；`Order → Inventory`；`refund_id/order_no/group_no/items/warehouse_id + source/reason`；“既有 `inventory.compensation_requested` 只有 `order_no/items`，不足以按组释放；**C-04 必须补组字段，C-05 无迁移**”（`:204`） |
| ADR Rev 2.3 §5.7 | “C-04 必须扩展组字段（**不新增 RPC**），C-05 无迁移”（`:724`） |
| ADR Rev 2.3 §5.7 / §8 | release 的 owner：Order 产出 durable release 请求、Inventory 执行；幂等键 `release_id=rr:<refund_id>`；无 RPC 语义（`:712-716`、`:940`） |
| T1.4a Rulings Addendum R-A | Order 在准入事务后产生持久 release 请求（outbox/lease task）；Inventory 执行 `Release`/`TccCancelConfirm`；C-04 事件字段，不新增 RPC |
| C-04 RefundBatch（accepted） | 只覆盖 Payment `RefundBatch`；不覆盖 Inventory release；不能把“RefundBatch accepted”当成“release RPC 存在” |
| SPEC-0002 v1.10 INV-44 | 组退款后的库存处置有主；未离仓组预留由 Inventory 释放；已离仓回库按 0001 INV-03；未要求 gRPC |
| A-12（C-04 API 增量） | 列出的 C-04 API 为 `AdmitRefund` 字段、`GetRefundByKey`、历史队列 RPC、配置；**不含 Inventory release RPC**（`:117`） |

### 1.3 结论

- C-04 release 的契约是 **event/outbox + Inventory consumer**，现有 `Release`/`TccCancelConfirm` 是执行 API；不需要 service/method/messages/字段/授权矩阵，也没有新增错误码。
- 幂等：`release_id=rr:<refund_id>`；consumer 重复/DLQ；`group_no` 审计；P0 单组 `group_count=1`；这些在 T1.4b2/Release event 契约中。
- **Reviewer B1 越界**：`tasks.md:21` 是 Plan 任务，不是 Spec/ADR；Plan 不能覆盖 ADR Rev 2.3。R0.4 应把“C-04 release RPC”改成“C-04 release event/schema/consumer 验证”；Reviewer 应撤回/降级 B1，或先走 ADR/SPEC 修订，不能倒逼实现新增 RPC。
- 交 Reviewer 复核：请 Reviewer 在 R0.2 verdict 中撤回 B1 的“RPC 缺失”，改为“Plan 文案与 ADR 不一致”；不得据此 FAIL C-04。

---

## 2. R0.4 设计

### 2.1 Fulfillment 启动读取 `settlement_switch_approvals`

在 Fulfillment 进程构造/启动消费者前，按 ADR Rev 2.3 §6.2 校验（只读；应用无写权限）：

| 请求模式 | 校验 | 失败 |
|---|---|---|
| `paid`（默认） | `SETTLE_RELEASE_APPROVAL_ID` 必须为空（N5）；无需批准行 | 非零退出；不得以 `paid` 继续 |
| `shadow` | 存在 `target_mode='shadow' AND status='APPROVED'` 的批准行；`expires_at IS NULL OR expires_at > DB NOW(3)`；`release_range` 按部署范围校验 | 非零退出；不得回退 `paid` |
| `settled` | `SETTLE_RELEASE_APPROVAL_ID` 非空；按 approval_id 查行；`target_mode='settled'`、`status='APPROVED'`、`expires_at IS NOT NULL AND expires_at > DB NOW(3)`、`release_range` 与本部署 release range 一致 | 非零退出；不得回退 `paid` |

- `release_range` 来源：部署配置 `SETTLE_RELEASE_RANGE`（新增 config；与批准行比较）；不得由代码默认空串绕过。
- 启动失败必须非零退出、记录 approval_id/mode/reason；不得自动写/改批准行；无运行期热切换；到期只使下次启动失败。
- `approval_id` 非空但未请求 `settled`（含 `shadow`）→ config error 启动失败（N5）。
- 读取不到表/连接失败 → 启动失败，不得猜 `paid`。
- 校验发生在消费者注册/建单前；未通过不得启动 worker。

### 2.2 `group_no` 判别 CHECK

新增 **`migrations/order/0022_r04_checks.sql`**（R0.4 专号；不改已合入 0017/0018/0019）：

```sql
ALTER TABLE refund_applications
  ADD CONSTRAINT chk_refund_app_group_discriminator CHECK (
    (source IN ('payment','legacy_expired') AND group_no IS NULL)
    OR
    (source IN ('aftersale','tcc','system_shortage','system_zero')
      AND group_no IS NOT NULL AND group_no <> '')
  );
```

- `group_no` 是 `refund_applications` 的判别字段；支付级/legacy 为 NULL，组级必须非空。
- 与 `0011`：`order_fulfillment_groups.group_no` 是 group 身份来源；CHECK 只约束 application 行，不复制 group 生命周期。
- 与 `0018`：0018 是 commitment composition；不改 0018，也不把 group 归属塞进 commitment 行。
- 与 R0.1：`0022` 在 0017/0019 后执行；R0.1 的 fresh apply 顺序必须为 `0011`→`0017`→`0018`→`0019`→`0022`；真实迁移测试覆盖“payment 带 group”/“组级无 group”失败。
- 0017 现有 `source`/`non_effective_capture` CHECK 保留，0022 只加判别。

### 2.3 `legacy_payment_resolution` 24h SLA

新增 `0022` 上的 CHECK（0019 已建表；不改 0019）：

```sql
ALTER TABLE legacy_payment_resolution
  ADD CONSTRAINT chk_lpr_expiry_24h CHECK (
    expires_at >= DATE_ADD(entered_at, INTERVAL 24 HOUR)
  );
```

- 语义：`expires_at` 不得早于 `entered_at + 24h`；writer 必须在 DB 时间上写入 24h 到期；不得用默认值/客户端时间。
- 过期状态机由 Order expiry/compensation worker 执行，不能由 DB CHECK 读 `NOW()`：
  - `state IN ('OPEN','RESOLVING') AND expires_at <= NOW(3)` → CAS `COMPENSATING` / `EXPIRED_ZERO` / `EXPIRED_FINANCE`；
  - 不允许继续 `OPEN/RESOLVING`；`COMPENSATING` 非终态，Payment 成功后才 `EXPIRED_REFUND`；
  - lease/version 保护并发。
- 0019 的 `chk_lpr_expiry (expires_at >= entered_at)` 保留；0022 追加 24h 下限。
- R0.1 真实迁移测试需覆盖：`expires_at = entered_at + 24h` 通过；`<24h` 失败；`OPEN` 过期后 worker 离开 OPEN；不新增 down migration。

### 2.4 R0.4 任务拆分

| ID | 任务 | Owner | 验收 |
|---|---|---|---|
| **R0.4a** | C-04 release RPC 判定/Reviewer B1 复核 | Architect + Reviewer | 依 ADR §2.4/§5.7 撤回 RPC 要求；Plan R0.4 文案修正 |
| **R0.4b** | Fulfillment 启动读取 `settlement_switch_approvals`（paid/shadow/settled/range/expiry/approval-id） | Developer + Architect | 失败非零退出；无 paid fallback；config/N5 测试 |
| **R0.4c** | `0022` group_no/24h CHECK + real migration | Developer + Architect | 真实迁移 PASS；错误行失败；0017/0018/0019 不改 |
| **R0.4d** | R0.1 顺序/重入/证据 | Lead + Developer | fresh DB 0011→0017→0018→0019→0022；日志 |
| **R0.4e** | Reviewer re-review | Reviewer | 精确 base/head；C-04 release event only；不要求 RPC |

## 3. C-04 release 与 T3 commitment release 边界

| 项 | C-04 release | T3 commitment release |
|---|---|---|
| 对象 | Inventory 预留/运单/回库（INV-44 物理库存） | `order_refund_commitments` 额度（RESERVED→RELEASED，INV-45/AC-62） |
| 触发 | Order 准入后 outbox event；Inventory consumer | 打款前 REJECTED/CANCELLED 或 FAILED_PERMANENT/人工放弃 + D-24 授权 |
| 执行 | `Release`/`TccCancelConfirm` | `refund_commitment_releases` 行 + 授权金额/复核 |
| 幂等 | `release_id=rr:<refund_id>` | `UNIQUE(refund_id)`/`UNIQUE(authorization_id)` |
| RPC | **不新增** | 人工/管理命令（T3），不是 release RPC |
| Phase 0-R | R0.4 不实现 T3 业务；不得把 T3 的额度释放/授权塞进 R0.4 | T3 独立任务，Phase 0-R 只做契约/校验 |

- R0.4 的 `group_no` CHECK 针对 `refund_applications`，不是 T3 commitment release；24h CHECK 针对 legacy queue，不是 T3。
- 不得以 C-04 release event 替代 T3 的 commitment release；两者 owner/触发/幂等键不同。

## 4. 状态

- **Status: DRAFT**；R0.4 不进入实现；需 QC/Architect + Reviewer 复核；C-04 release RPC 判定交 Reviewer 撤回 B1。
- 不写代码/迁移/测试；不修改已 accepted ADR Rev 2.3；C-05 0022 为 R0.4 草案。
