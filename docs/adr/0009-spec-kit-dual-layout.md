# ADR-0009: Spec Kit 双布局试点（SDD-STRUCT-ALIGN-01）

| 项 | 值 |
|---|---|
| Status | **Accepted**（2026-11-27 人类批准 Spec 时同批接受；实现 PASS） |
| Change ID | `SDD-STRUCT-ALIGN-01` |
| 取代 | 无 |
| 关联 | `specs/sdd-struct-align-01/spec.md`、`docs/adr/0007-payment-settlement-admission.md`（无冲突） |

## 背景

本仓现有 Spec 是 `docs/specs/<NNNN>-<name>.md` 扁平文件（`docs/specs/README.md`：一份业务能力一个文件、approved 后原地升版）。Spec Kit 1.4.3 的 `check-prerequisites.sh` 通过 `SPECIFY_FEATURE_DIRECTORY`/`.specify/feature.json` 解析 feature 目录，`--require-spec` 要求目录内有 `spec.md` + `plan.md`，`--require-tasks` 另要求 `tasks.md`；`create-new-feature.sh` 默认创建 `specs/<NNN-name>/`（`SPECS_DIR=specs`，`:280`）。扁平文件不能直接作为 `FEATURE_SPEC`，因此 `speckit-specify/plan/tasks/implement` 在本仓当前不可用。

## 决策

采用**双布局**，不做全量迁移：

1. 新变更使用 `specs/<change_id>/spec.md` + `plan.md` + `tasks.md`；当前 feature 由 `SPECIFY_FEATURE_DIRECTORY` 或 `.specify/feature.json` 指定（后者是机器本地状态，不提交）。
2. 现有 `docs/specs/0001-...md`、`docs/specs/0002-...md` **原地不动**，继续作为已批准契约；`Spec.md` 现有索引行不改。
3. `Spec.md` 只加一行指针，指向 `specs/README.md`（新变更布局说明）。
4. `specs/README.md` 写明：新变更在 `specs/`；`docs/specs/` 是 2026-11-27 前的扁平现行/历史契约；一个 Spec 一个验收表；迁移另立变更。
5. 本变更的试点 Spec 放在 `specs/sdd-struct-align-01/`，用 C-2 preset 模板（Spec Kit 原生章节 + 附录 A 对照）。

## 备选与否决理由

- **全量迁移 `docs/specs/` → `specs/`**：引用面大（`Spec.md`、`docs/specs/README.md`、ADR、台账、测试），已批准契约路径变化，风险高于首个试点的收益；否决。
- **只用模板/语法，不接 CLI**：零结构变更，但 `check-prerequisites`/`implement` 仍不可用，试点退化；否决。

## 后果

- 两种布局并存；`specs/` 与 `docs/specs/` 的边界由 `specs/README.md` 和 `Spec.md` 指针说明。
- `feature.json` 不提交；作者需在会话内设置 `SPECIFY_FEATURE_DIRECTORY` 或运行 `specify`。
- 不新增业务行为/AC；不改核心模板/脚本；`docs/specs/` 不动。
- 后续若要单一布局，另立迁移变更（L2/契约变更），不在本试点内做。

## 核对

- 每条 AC 见 `specs/sdd-struct-align-01/spec.md` 验收状态表。
- 路径解析：`.specify/scripts/bash/common.sh:163-230`（`get_feature_paths`）。
- 命令实测：`check-prerequisites.sh --json --require-spec`（spec+plan）与 `--require-tasks --include-tasks`（+tasks）已在 `/tmp/spec-kit-pilot` 通过；本仓 `specs/sdd-struct-align-01/` 的 AC-2/3/6 已由 `REV-SDD-STRUCT-ALIGN-01-SPEC.md` 验证；AC-1 修正后重审。
