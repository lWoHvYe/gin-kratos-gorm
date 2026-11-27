# specs/ — 新工作布局

- **新能力**：`specs/<feature>/spec.md` 是该能力的权威 Spec；plan/tasks/evidence 放同目录。
- **既有 `specs/<feature>`**：原地更新 spec；不新建 per-change 第二份 Spec。
- **既有 `docs/specs/0001/0002`**：冻结；不迁移、不复制；修改只做必要原地修改，迁移另案。
- **一个能力一个权威**：不得在 `docs/specs/` 与 `specs/` 同时维护同一能力的 active Spec。
- **命令**：有 `plan.md` 的 feature 目录可用 `check-prerequisites --require-spec`；legacy 无 plan 用 `--paths-only`。
- **非规范**：`docs/checklists/**`、`docs/guides/**` 是操作辅助，不是验收依据；冲突以宪法/AGENTS 为准。
