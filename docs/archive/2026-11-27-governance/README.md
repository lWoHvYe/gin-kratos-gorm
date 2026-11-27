# 2026-11-27 治理模型 v2.0 归档

本目录保存 `GOV-CONVERT-01`（2026-11-27 人类批准）停用的治理文件。原路径保留一行 stub，链接不回改，历史审查报告与已批准契约中的旧路径仍可导航。

| 原路径 | 归档文件 | 现行替代 |
|---|---|---|
| `docs/collab/CONVENTIONS.md` | [`CONVENTIONS.md`](CONVENTIONS.md) | [`AGENTS.md`](../../../AGENTS.md) + [`.specify/memory/constitution.md`](../../../.specify/memory/constitution.md) |
| `prompt/*.md`（6 个角色） | [`prompt/`](prompt/) | 角色模型改为作者 / 审查者 / 人类，见 `AGENTS.md` §1 |
| `docs/collab/ac-coverage.tsv` | [`ac-coverage.tsv`](ac-coverage.tsv) | `specs/0002-payment-settlement-admission/spec.md` §15 + `docs/review-tracking.md` |
| `docs/handoff.md` | [`handoff.md`](handoff.md) | `docs/review-tracking.md`（唯一运行状态） |
| `docs/lead/PAY-SETTLE-01/backlog.md` | [`PAY-SETTLE-01-backlog.md`](PAY-SETTLE-01-backlog.md) | `docs/review-tracking.md` + SPEC-0002 §15 |

## v2.2.0 快照

| 原路径 | 归档文件 | 说明 |
|---|---|---|
| `AGENTS.md` | [`AGENTS_archive.md`](AGENTS_archive.md) | base `6ee05df` 的 v2.2.0 |
| `.specify/memory/constitution.md` | [`constitution_archive.md`](constitution_archive.md) | base `6ee05df` 的 v2.2.0 |

## 保留未归档

- `docs/collab/ac-baseline.txt`：C-4 清单未列；新模型没有 AC 基线概念，待人类决定是否归档。
- `docs/po/backlog.md`：产品需求池，不属治理文本；当前入口为 `specs/README.md`。
- `docs/roadmap-status.md`：2026-11-22 路线图快照，非治理文本。

## 规则

- 本目录只读；不得把这里的规则当作现行规则。
- 历史审查报告、已批准 Spec/ADR 中的旧路径保留原样，不回溯改写。
