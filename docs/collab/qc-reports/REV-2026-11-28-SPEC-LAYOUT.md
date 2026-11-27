# REV-2026-11-28 — v3 / Spec 单布局交付证据

Status: **author evidence; independent review pending; NOT RELEASE**.

| 项 | 值 |
|---|---|
| change_id | `LEAN-CORE-01` (parent) / `SPEC-LAYOUT-01` (child; human approved combined) |
| base..head | `6ee05df..5a8b80c`（后续收口 commit 见历史） |
| 级别 | 治理 + 契约 |
| 阶段 | Spec |
| Spec/ADR | `specs/README.md`; `docs/adr/0012-spec-single-layout.md` |
| 未覆盖 | 无业务代码/运行时测试；无 pilot 指标；0001 QC delta 未复审 |

## 交付

- 0001/0002 `git mv` 到 `specs/<id>/spec.md`；`git diff` R098，仅链接/路径/位置/历史批准行路径变化，业务/AC 文本未重写；
- `docs/specs` 只剩重定向 README；`Spec.md` 只指针；`specs/README` 唯一 active index；
- `sdd-struct-align-01` historical，不在 index；
- ADR-0009 header `Superseded by ADR-0012`；ADR-0012 Accepted；
- 0001 改标 historical reference（QC delta pending），补 §17 历史验收表；0002 active，状态以 §15 为准。

## 验证命令 / 结果

```text
git diff --summary 6ee05df..HEAD        # R098 x2; no code dirs
git ls-files docs/specs                 # only README
relative link check (27 changed md)     # 0 broken
grep '^| AC-' specs/0001.../spec.md     # 38 rows in §17
git grep docs/specs/0001|0002           # only historical segments/pointers
```

## 【测试范围】

- 文档/链接/index/命令边界静态检查；
- `check-prerequisites --paths-only` 0001/0002 exit 0；
- 0001 `--require-spec` exit 1（无 plan.md），与 `specs/README` 边界一致。

## 【未覆盖范围】

- 未跑 Go/集成测试（无代码改动）；
- 未跑 pilot 指标；
- 未完成 0001 QC delta 复审；未把 0001 冒充 frozen/已验证。

## 【取舍】

- 0001 用 historical reference + §17 历史表，避免伪造“frozen/已验证”；
- 历史台账/旧 ADR 段保留旧路径，不重写历史；live 指针已更新；
- 独立 reviewer 对最终 diff 的 FAIL 项已收口一部分；仍以 review pending 记录。

## 结论

`release_status: NOT RELEASE`。独立 review 通过前不得据此宣称 PASS/Release。
