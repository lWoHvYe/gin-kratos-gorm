# REV-ADR-0007-R2.2 — Batch B BLOCKER

| 项 | 值 |
|---|---|
| Object | ADR-0007 Rev 2.1/2.2 delta（startup switch；Rev 2.3 为 N5 follow-up） |
| base | `104b927` |
| head | working tree（uncommitted） |
| Actor | Reviewer |
| Reviewed-by | human |
| review_scope | ADR §6.2/§6.4 startup switch |
| verdict | **FAIL**（initial）→ 重审 **PASS**（Rev 2.2/2.3） |

## Finding — `[ARCHITECTURAL_FLAW]` (BLOCKER)

`ADR §6.2`（Rev 2.1）规定：请求 `settled` 但 `settlement_switch_approvals` 行缺失/过期时，按 `paid` 运行（"按 `paid` 运行并告警"）。ON 成功后，这会静默恢复 `order.paid` 准入、绕过 §6.4 的 Director 批准回滚，并违反 SPEC-0002 INV-49（"切换后 `order.paid` 不得再作准入依据"）；对已 settled 订单存在重复/订单级运单风险。

## Required fix

- 请求 `settled`/`shadow` 且批准行缺失/过期/非 `APPROVED`/range 不匹配 → **非零启动失败**，绝不 `paid`。
- `settled→paid` 只经 §6.4 Director 批准回滚。
- `settlement_switch_approvals.expires_at` 对 `settled` NOT NULL（CHECK），且启动时未过期。
- N5：`SETTLE_RELEASE_APPROVAL_ID` 非空但未请求 `settled` → 启动失败。
- §12.3 step 0 仅记录，不写 settlement task/refund application。

## Resolution

Rev 2.2 落地 fail-closed 启动校验；Rev 2.3 补 N5 approval-id/mode 配对。§6.2/§6.3/§6.4/§7/§12.3 与 SPEC v1.10 AC-66/§5.9/D-27 已复核一致。verdict FAIL → 重审 PASS。

**State**: not AC-VERIFIED; not Release PASS; human merge/Release still required.
