# ADR-0012: Spec 单布局迁移

| 项 | 值 |
|---|---|
| Status | **Accepted**（2026-11-28 人类批准） |
| Date | 2026-11-28 |
| Supersedes | **ADR-0009**（Spec Kit 双布局试点） |
| Superseded by | 无 |
| Related Spec | `specs/0001-fulfillment-quantity-split/spec.md`、`specs/0002-payment-settlement-admission/spec.md` |
| Contract owner | human（AGENTS §1 既有角色） |

## 背景

ADR-0009 采用 `docs/specs/` 扁平 + `specs/<change_id>/` 双布局。实际后果：同一能力可能两份 Spec；`--require-spec` 与扁平原地升版冲突；`sdd-struct-align-01` 没有过渡；reviewer 无法仅凭 index 判断权威。

## 决策

1. 所有 active Spec 统一为 `specs/<id>/spec.md`；`docs/specs/` 只保留重定向 README。
2. `0001/0002` 用 `git mv` 迁入 `specs/0001-fulfillment-quantity-split/spec.md`、`specs/0002-payment-settlement-admission/spec.md`；只改相对链接/路径，不改业务正文/AC。
3. `specs/README.md` 是唯一 active-Spec index；一行一个能力；记 `id/title/status/version/authority_path/owner/supersedes/adr`。
4. `Spec.md` 只做指针；`docs/specs/README.md` 只做重定向。
5. `specs/sdd-struct-align-01` 标 historical；不进入当前 index；AC-4/5 不再作为当前证据。
6. AGENTS/宪法同步到新布局；0001/0002 不保留副本。

## 备选与否决

- 保留双布局：否决，第二权威。
- 只加 index 不迁移：否决，`--require-spec`/路径仍分裂。
- 保留 0001/0002 副本：否决，副本=第二真源。

## 后果

- 0001/0002 迁移后需更新内部相对链接与 live ADR/文档引用；历史台账不重写。
- 新工作 `specs/<feature>`；变更原地更新；evidence 放 `specs/<id>/evidence/<change_id>/`。
- `check-prerequisites --require-spec` 仅对含 `plan.md` 的 Spec 使用；legacy 无 plan 用 `--paths-only`。

## 验证

- AC-001：`docs/specs` 无 active `*.md`，只 README；
- AC-002：0001/0002 `git diff -M` 为 rename，业务/AC 未改，仅链接/路径；
- AC-003：`specs/README` 唯一 index，每能力一行；
- AC-004：`Spec.md` 指针；docs/specs README 重定向；
- AC-005：ADR-0009 header superseded；sdd historical；
- AC-006：live 引用无旧 `docs/specs/0001|0002`；历史台账保留；
- AC-007：`--paths-only`/`--require-spec` 边界符合；
- AC-008：回滚可 `mv` 回，且无副本。
