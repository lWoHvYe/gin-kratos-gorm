# EVID-2026-11-28 — DOC-CONSISTENCY-01（作者证据）

Status: **author evidence; independent review pending; NOT RELEASE**.

| 项 | 值 |
|---|---|
| change_id | `DOC-CONSISTENCY-01` |
| base..head | `db27510..工作树`（HEAD=`db27510`；提交后回填最终 head） |
| 级别 | 治理变更（**PATCH**；v3.0.1） |
| 阶段 | Spec |
| Spec/ADR | 无（治理变更） |
| 未覆盖 | 无代码/运行时测试；0001 状态与 `Spec.md` 的 QC 残留（另立 `SPEC-INDEX-01`）；pilot 指标 |

## 改动（6 个文档文件）

- `AGENTS.md`：§3 审查者任务书增加“先读 AGENTS.md”；§2.3 的 0001 状态改为“见台账”；v3.0.1 PATCH。
- `.specify/memory/constitution.md`：clause 4/5 的 “QC delta 待复审” 改为“复审状态见台账”；v3.0.1 PATCH。
- `docs/guides/spec-kit.md`：QC delta 术语改为“见台账”；补充模板 `[###-feature-name]` 只是占位、`specs/<id>` 可为 slug/change_id。
- `docs/guides/review-protocol.md`：QC delta 术语改为“见台账”；注明本协议不用于 v3 pilot，§5 风险分级不替代宪法 12/AGENTS。
- `docs/collab/qc-reports/REV-2026-11-28-SPEC-LAYOUT.md`：标题标注“作者证据，不是 Reviewer 裁决”；QC delta → reviewer delta。
- `docs/review-tracking.md`：保留独立 reviewer 的 FAIL 条目；作者已恢复被误改的 2026-11-28 旧条目原文，不改写 AC/状态行。

**未改**：`specs/0001/.../spec.md`、`specs/0002/.../spec.md`、`specs/README.md`。已批准 Spec 正文不静默改写，避免行号漂移（上一轮 FAIL 的 B1/B2/B5 根因）。

## 验证命令 / 结果

```text
git diff --check                         # exit 0
git diff --name-only                     # 6 个 tracked 文档，无代码/迁移/模板
grep -rn "QC delta" AGENTS.md .specify/memory/constitution.md \
  docs/guides/spec-kit.md docs/guides/review-protocol.md \
  docs/collab/qc-reports/REV-2026-11-28-SPEC-LAYOUT.md
                                         # 0（规范/流程文件）
grep -rn "QC delta" docs/review-tracking.md
                                         # 4 处：1 处为本轮 review 条目引用该字符串，
                                         # 2 处旧 2026-11-28 条目，1 处历史段；
                                         # 均不在本 change 的改写范围，显式列为例外
git diff --name-only db27510 -- specs/   # 空（0001/0002/README 未改）
git status --short                       # 6 个 M + 1 个 ??（本 EVID 未跟踪）
```

> 本 EVID 是**未跟踪文件**，提交时必须纳入，否则证据不随交付；最终 head 在提交后回填。
> `review-tracking` 的 4 处 `QC delta` 是历史/本轮 review 引用（旧 2026-11-28 条目、历史段、review 条目），不在本 change 的改写范围。

## 【测试范围】

- 静态文档检查：diff、术语 grep、文件范围、`git diff --check`；
- 不改代码/schema/RPC/事件/迁移，不跑 Go/集成测试。

## 【未覆盖范围】

- 0001 的 `spec.md` 头部与 `Spec.md` 仍保留历史 “QC delta” 字样（historical 正文，不重写）；其当前状态应在台账，另立 `SPEC-INDEX-01` 统一索引与指针；
- 0001 reviewer delta 复审；
- `SPEC-INDEX-01`（0001 状态与 `Spec.md` 的 QC 残留）尚未在台账登记；需按 AGENTS §4 立条目并指定 owner，非本 change 阻断；
- v3 pilot 指标与两个试点变更；
- 未重排 `docs/review-tracking.md` 的历史日期组（P1-2 另定）。

## 【取舍】

- 不改已批准 Spec 正文：避免契约改写与 `0001:622-625` / `0002:90` 等 ADR 锚点漂移；
- 不改 `.specify/templates/**`（AGENTS §6：核心模板/脚本不改），改由 guide 说明占位符；
- 台账不动旧条目的 AC/状态行；只保留 reviewer 自己的 FAIL 条目。

## 收益 / 停止条件（宪法 18）

- **收益**：消除 v3 三角色模型中的 QC 残留与“规范文件携带运行状态”的自相矛盾；首次 pilot 前减少一处审查者读不到 AGENTS 的歧义。
- **停止条件**：若独立 reviewer 仍判 FAIL，revert 本 diff，不进入 pilot；不把本 PATCH 作为 v3.0 已通过的证据。
