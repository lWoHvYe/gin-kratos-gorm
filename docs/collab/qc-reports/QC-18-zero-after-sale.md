# QC-18 — 零元售后 Amendment 短复核（v1.9-r2 / Addendum r3）

| 项 | 值 |
|---|---|
| review_scope | `docs/po/drafts/0002-v1.9-zero-after-sale.md`（v1.9-r2：S19 同事务 DONE、AC-55 例外、AC-67/68）；`docs/adr/0007-amd1-addendum-zero-after-sale.md`（r3：`AdmitRefund` 零元短路、C-05 0021、AC-68 收敛） |
| 审查类型 | QC-17 B-1/B-2/B-3 闭合复核；不替代 Human `ZERO AMENDMENT APPROVED` |
| 结论 | **`READY FOR HUMAN ZERO AMENDMENT APPROVAL`** |

> 只读复核；未改文件、未跑测试。

## 1. QC-17 闭合情况

| 项 | 状态 | 证据 |
|---|---|---|
| **B-1** `AdmitRefund(amount=0)` 不可复用 / `[ARCHITECTURAL_FLAW]` | **闭合（契约层）** | Addendum r3 AZ-01/A-Z1/AZ-04（`:22-27,35,56`）明确“`AdmitRefund` 零元短路（不新增 RPC）：`source=system_zero && amount=0 && pay_cent=0 && 无任何已捕获 intent` 时跳过 Payment 额度/commitment，仍执行 permit/运单/INV-44，同事务 DONE；正金额路径不变”；PO v1.9-r2 S19（`:55-62`）已写“零元短路同事务直接 DONE、不创建 PENDING”；C-04 proto 无新增、Order 实现需零元分支 |
| **B-2** C-05 “零影响”不成立 | **闭合（契约层）** | Addendum r3 §7/§1.1（`:64,153-161`）明确“C-05 新增 1 个迁移，只加 `refund_applications` 的 `system_zero` 0 金额/non_effective=0 CHECK；不改已合入的 `0017`”；不再声称零影响 |
| **B-3** AC-55 例外未正式成立 | **闭合** | PO v1.9-r2 AC-68（`:94-103`）已收敛：零元例外仅适用于 `pay_cent=0` 且无任何已捕获 intent；有捕获按 AC-40/AC-63；正金额退款继续按 AC-55 以 Payment 事实为准；AC-68 不再是“A-Z3 若另选”的开放条款 |

## 2. BLOCKER

**无。**

## 3. 非阻断提示（批准/实现前置）

1. **C-05 的 `0021_zero_application_check.sql` 尚未出现在当前工作区**：Addendum r3 已定义“新增 1 个迁移、建议名 0021、不改 0017”，但 `migrations/order/` 当前只到 `0020`。这是 C-05 的待实现项，不是零元 Amendment 的文本缺陷；应在 C-05 评审/实现前补齐，并在 Addendum 的“生效前置”中保持“C-04/C-05 输入评审通过”的门禁。
2. **`AdmitRefund` 的零元短路代码尚未实现**：Addendum 的 AZ-01/A-Z1 已把契约定死；Order 的 `refund_admission.go` 需按此分支实现（`source=system_zero + amount=0 + pay_cent=0 + 无捕获` 时跳过 `GetIntent`/captured 校验/commitment，仍走 permit/运单/INV-44），这是 T1.2 的实现项，不是文本阻塞。
3. 沿用 QC-17：S19 是 `refund_applications(source=system_zero)` 的语义子集；`amount_state=CONSUMED + REFUNDED/0 + refund_no=''` 由 AC-68 例外覆盖；`AdmitRefund` 零元分支不创建 0 元 commitment，与 S15/INV-45/AC-62 相容（0 元无额度可占）。

## 4. 声明

- 我**未修改任何文件**（PO 草案/ADR Addendum/SPEC/ADR/C-04/C-05 均只读），**未运行测试/门禁**。
- 本次只复核 QC-17 B-1/B-2/B-3 的闭合；`READY FOR HUMAN ZERO AMENDMENT APPROVAL` 只针对零元修订文本，不构成 C-04/C-05 批准、实现验证或 Release PASS。
