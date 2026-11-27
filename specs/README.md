# specs/ — Spec Kit 新变更布局

2026-11-27 起，新变更使用 `specs/<change_id>/`；现有 `docs/specs/` 的扁平契约原地不动。

## 规则

- **新变更**：`specs/<change_id>/spec.md`、`plan.md`、`tasks.md`。当前 feature 由 `SPECIFY_FEATURE_DIRECTORY` 或 `.specify/feature.json` 指定；后者是机器本地状态，不提交。
- **现有 `docs/specs/`**：`0001-...md`、`0002-...md` 是 2026-11-27 前的扁平现行/历史契约；本布局不迁移、不改写它们。迁移到单一布局是独立契约变更。
- **一个 Spec 一个验收表**：AC 状态只在 Spec 的验收状态表；运行状态只在 `docs/review-tracking.md`；plan/tasks/MEMORY/汇总文档不保存状态。
- **Spec 模板**：`./.specify/scripts/bash/resolve-template.sh spec-template` = 核心模板 + 宪法附录 A（preset `gin-kratos-spec`，`append`）。
- **feature 切换**：设置 `export SPECIFY_FEATURE_DIRECTORY=specs/<change_id>`，或运行 `specify` 让它写 `.specify/feature.json`；不要提交 `feature.json`。

## 快速检查

```bash
export SPECIFY_FEATURE_DIRECTORY=specs/<change_id>
./.specify/scripts/bash/check-prerequisites.sh --json --require-spec
./.specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks
./.specify/scripts/bash/resolve-template.sh spec-template | grep -F '## 附录 A 对照'
```

`check-prerequisites.sh --json --require-spec` 返回 `FEATURE_DIR`；`--paths-only --json` 返回 `FEATURE_SPEC`。

## 边界

- 本目录不复制 `docs/specs/` 内容；不产生第二个真源。
- 不修改 `.specify/templates/spec-template.md`、`.specify/scripts/**`；模板扩展走 preset。
- 新变更的 Spec 使用 Spec Kit 原生章节或 EARS 均可，但必须回答宪法附录 A 的八项。
