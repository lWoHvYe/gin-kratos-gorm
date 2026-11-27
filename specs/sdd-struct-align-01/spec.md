# SDD-STRUCT-ALIGN-01 — Spec Kit 双布局试点

**Feature**: `specs/sdd-struct-align-01/`（historical）
**Status**: historical（原 ADR-0009 试点；单布局迁移后不再作为当前证据）
**Input**: 人类 2026-11-27 选择方案 A（双布局 + 自举试点）+ Spec.md 加一行指针。

## User Scenarios & Testing

### User Story 1 - 新变更在 `specs/<change_id>/` 走 Spec Kit（P1）

作者对一个新的变更创建 `specs/<change_id>/spec.md`、`plan.md`、`tasks.md`，设置 `SPECIFY_FEATURE_DIRECTORY`（或由 `specify` 写 `.specify/feature.json`），随后 `check-prerequisites.sh`、`/speckit-plan`、`/speckit-tasks`、`/speckit-implement` 能解析到该目录。

**Why this priority**: 这是试点要解决的核心问题；没有它，Spec Kit 的 CLI 路径在本仓不可用。

**Independent Test**: 在 `specs/sdd-struct-align-01/` 上运行 `check-prerequisites.sh --json --require-spec` 与 `--require-tasks --include-tasks`，两者 exit 0。

**Acceptance Scenarios**:

1. **Given** `SPECIFY_FEATURE_DIRECTORY` 指向含 `spec.md` + `plan.md` 的目录，**When** 运行 `check-prerequisites.sh --json --require-spec`，**Then** exit 0 且 `FEATURE_DIR` 为该目录；同一目录上 `--paths-only --json` 返回 `FEATURE_SPEC` = 该目录的 `spec.md`。
2. **Given** 同目录还有 `tasks.md`，**When** 运行 `--json --require-tasks --include-tasks`，**Then** exit 0 且 `AVAILABLE_DOCS` 含 `tasks.md`。

### User Story 2 - 现有 `docs/specs/` 不被迁移（P1）

现有扁平 Spec 继续作为已批准契约；`Spec.md` 只加一行指向 `specs/README.md`，不改现有索引行与文件路径。

**Why this priority**: 双布局的价值在于不破坏已批准契约；迁移必须是独立变更。

**Independent Test**: `git diff` 对 `docs/specs/0001-...md`、`docs/specs/0002-...md` 为空；`Spec.md` 只多一行指针。

**Acceptance Scenarios**:

1. **Given** 本变更实现完成，**When** 检查 `docs/specs/0001-...md` 与 `docs/specs/0002-...md`，**Then** 文件路径与内容均未变。
2. **Given** 本变更实现完成，**When** 检查 `Spec.md`，**Then** 现有索引行不变，只多一行指向 `specs/README.md`。

### Edge Cases

- `SPECIFY_FEATURE_DIRECTORY` 缺失且 `.specify/feature.json` 不存在：脚本必须报错，不得回退到 `docs/specs/`。
- 目录存在但缺 `plan.md`：`--require-spec` 必须报 “plan.md not found”，不得静默通过。
- 目录存在但缺 `tasks.md`：`--require-tasks` 必须报错。
- 同时存在 `docs/specs/` 与 `specs/`：以 `SPECIFY_FEATURE_DIRECTORY`/`feature.json` 指向的目录为准，不自动猜测。
- `SPECIFY_FEATURE_DIRECTORY` 是相对路径：按仓库根解析，与绝对路径等价。

## Requirements

### Functional Requirements

- **FR-001**: WHEN `SPECIFY_FEATURE_DIRECTORY` 指向含 `spec.md` + `plan.md` 的目录，`check-prerequisites.sh --json --require-spec` SHALL exit 0 且 `FEATURE_DIR` 为该目录；同一目录上 `--paths-only --json` SHALL 返回 `FEATURE_SPEC` = 该目录的 `spec.md`。
- **FR-002**: WHEN 同目录还有 `tasks.md`，`check-prerequisites.sh --json --require-tasks --include-tasks` SHALL exit 0，且 `AVAILABLE_DOCS` 含 `tasks.md`。
- **FR-003**: IF feature 目录、`spec.md` 或 `plan.md` 缺失，THEN 脚本 SHALL exit 非零并给出明确错误；不得回退到 `docs/specs/`。
- **FR-004**: 本变更 SHALL NOT 移动或改写 `docs/specs/0001-...md`、`docs/specs/0002-...md`；`Spec.md` 现有索引行 SHALL 不变，只允许新增一行指向 `specs/README.md`。
- **FR-005**: `specs/README.md` SHALL 说明：新变更用 `specs/<change_id>/`；`docs/specs/` 是 2026-11-27 前的扁平现行/历史契约；一个 Spec 一个验收表；迁移另立变更。
- **FR-006**: `resolve-template.sh spec-template` SHALL 含宪法附录 A 对照段（C-2 preset `gin-kratos-spec`，append）。

### Key Entities

无（工具链变更；不涉及数据模型/业务状态）。

## Success Criteria

### Measurable Outcomes

- **SC-001**: 在本仓 `specs/sdd-struct-align-01/` 上，AC-1/AC-2 的两条命令 exit 0；AC-3 的缺 plan/tasks 用例 exit 非零。
- **SC-002**: `git diff -- docs/specs/0001-fulfillment-quantity-split.md docs/specs/0002-payment-settlement-admission.md` 为空；`Spec.md` 只多一行指针。
- **SC-003**: `resolve-template.sh spec-template` 输出含 `## 附录 A 对照`；`specify preset list` 显示 `gin-kratos-spec` enabled。

## Assumptions

- Spec Kit 1.4.3；`check-prerequisites.sh` 通过 `SPECIFY_FEATURE_DIRECTORY`/`feature.json` 解析目录（`.specify/scripts/bash/common.sh:163-230`）。
- PyYAML 6.0.4 已装 user-site，供 preset 解析；不依赖系统包。
- `.specify/feature.json` 是机器本地状态，不提交；作者在会话内设置 env 或运行 `specify`。
- 不修改 `.specify/templates/spec-template.md`、不修改 `.specify/scripts/**`。
- 不新增业务行为/AC；不迁移 `docs/specs/`。

## 附录 A 对照

| 必备问题 | 回答 |
|---|---|
| 业务规则与验收标准 | AC-1…AC-6 见验收状态表；工具链行为，无业务规则。金额/状态/并发不适用。 |
| 状态所有权 | 无业务状态；Spec/AC 状态只在验收表，运行状态在台账。 |
| 契约与异常恢复 | 路径解析通过 env/`feature.json`；缺 spec/plan/tasks fail-closed；不迁移旧路径；设计决策见 `docs/adr/0009-spec-kit-dual-layout.md`。 |
| 可达性检查 | `get_feature_paths`：`.specify/scripts/bash/common.sh:163-230`；JSON 输出：`.specify/scripts/bash/check-prerequisites.sh:20`；`--paths-only` 的 `FEATURE_SPEC`：`:110-125`；fail-closed：`:139-169`；`create-new-feature.sh` 默认 `SPECS_DIR=specs`：`:280`、`FEATURE_DIR/SPEC_FILE`：`:424-425`。 |
| 假设与不确定性 | 见 §Assumptions；Spec Kit 升级/核心模板变化时需复核路径与 preset。 |
| 业务裁决题 | D-1 布局：A 双布局（人类 2026-11-27）；D-2 Spec.md 指针：A 加一行（人类 2026-11-27）。无其他。 |
| 验收状态表 | 见下。 |
| 延期清单 | 无。 |

## 验收状态表

AC-1…AC-6 与 FR-001…FR-006 一一对应。

> 历史验收表：本表只记录 ADR-0009 试点结果；AC-4/5 不再作为当前证据。

| AC 编号 | 状态 | 检查类型 | 证据路径 | 审查者 |
|---|---|---|---|---|
| AC-1 | 已验证 | 契约检查 | FR-001；`--json --require-spec` exit 0，`FEATURE_DIR` = `specs/sdd-struct-align-01`；`--paths-only --json` exit 0，`FEATURE_SPEC` = 该目录 `spec.md`；见 `docs/collab/qc-reports/REV-SDD-STRUCT-ALIGN-01-SPEC.md` §2 | reviewer-syntax |
| AC-2 | 已验证 | 契约检查 | FR-002；`SPECIFY_FEATURE_DIRECTORY=specs/sdd-struct-align-01 ... --json --require-tasks --include-tasks` → exit 0，`AVAILABLE_DOCS:["tasks.md"]`；报告 §2 | reviewer-syntax |
| AC-3 | 已验证 | 契约检查 | FR-003；缺 plan/tasks/spec/目录 → exit 1（`/tmp/sdd-review/{noplan,notasks,nospec,nodir}`）；报告 §2 | reviewer-syntax |
| AC-4 | 已验证 | 契约检查 | FR-004；`git diff 6a66ea5 -- docs/specs/0001/0002` = 0 行；`git diff 6a66ea5 -- Spec.md` = 1 added/0 deleted（仅末行指针，现有索引行未改）；见报告 §0B | reviewer-syntax |
| AC-5 | 已验证 | 契约检查 | FR-005；`specs/README.md` 存在（28 行），覆盖双布局/`docs/specs/` 边界/一个 Spec 一个验收表/`feature.json` 不提交/迁移另立/模板命令；见报告 §0B | reviewer-syntax |
| AC-6 | 已验证 | 契约检查 | FR-006；`resolve-template.sh spec-template` 输出含 `## 附录 A 对照`；`specify preset list` enabled/Templates 1；报告 §2 | reviewer-syntax |

> AC↔FR 映射：AC-1=FR-001、AC-2=FR-002、AC-3=FR-003、AC-4=FR-004、AC-5=FR-005、AC-6=FR-006。AC-1 已验证（B1 消解：`--json --require-spec` 的 `FEATURE_DIR` + `--paths-only --json` 的 `FEATURE_SPEC`）；AC-4/AC-5 实现阶段已验证（见报告 §0B）。

## 延期清单

无。
