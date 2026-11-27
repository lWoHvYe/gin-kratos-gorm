> 2026-11-27 人类批准方案 A；Spec/ADR 已批准，实现 PASS（见 `REV-SDD-STRUCT-ALIGN-01-SPEC.md`）；待人类合入。

# SDD-STRUCT-ALIGN-01 试点方案（Phase 0）

| 项 | 值 |
|---|---|
| 变更 | `SDD-STRUCT-ALIGN-01`（工具链/结构；按宪法 3 兜底作为契约变更） |
| 状态 | 提案，待人类选方案；未创建 `specs/`、未改 `docs/specs/` |
| 目标 | 让 Spec Kit 的 `specify`/`plan`/`tasks`/`implement` 路径在本仓可用，同时不破坏现有 `docs/specs/` 扁平 Spec |
| 基线 | `6a66ea5`（C-2 preset append 已提交）；宪法 v2.2.0 |

---

## 1. 侦察结论（可执行事实）

1. **`check-prerequisites.sh` 不硬编码 `specs/`**。它通过 `get_feature_paths` 从
   `SPECIFY_FEATURE_DIRECTORY`（优先）或 `.specify/feature.json` 的 `feature_directory`
   解析目录；目录内约定 `spec.md`、`plan.md`、`tasks.md`（`common.sh:163-230`）。
2. **实测**（`/tmp/spec-kit-pilot`，不改仓库）：
   - `spec.md + plan.md` → `--json --require-spec` **exit 0**，返回 `FEATURE_DIR`。
   - 再加 `tasks.md` → `--json --require-tasks --include-tasks` **exit 0**，返回 `tasks.md`。
   - 缺 `plan.md` → exit 1，提示 “Run /speckit-plan first”。
3. **`create-new-feature.sh` 默认创建 `specs/<NNN-name>/`**，并写 `.specify/feature.json`；
   `feature.json` 被 `.specify/.gitignore` 忽略（机器本地状态）。
4. **现有布局**：`docs/specs/0001-...md`、`docs/specs/0002-...md` 是扁平文件；
   `docs/specs/README.md` 规定“一份业务能力一个文件、编号终身唯一、approved 后原地升版”。
   `check-prerequisites` 要求 `spec.md` 在 feature 目录内，扁平文件不能直接作为 `FEATURE_SPEC`。
5. **C-2 已解决模板**：`resolve-template.sh spec-template` = 核心模板 + 附录 A 8 项
   （preset `gin-kratos-spec`，append）；`specify` 会用这份模板。

---

## 2. 方案

### A. 双布局（推荐）

- **新变更**用 Spec Kit 布局：`specs/<change_id>/spec.md`、`plan.md`、`tasks.md`；
  `SPECIFY_FEATURE_DIRECTORY` 或 `feature.json` 指向当前目录。
- **现有 `docs/specs/*.md` 原地不动**，继续作为已批准契约；`Spec.md` 索引不改。
- 新增 `specs/README.md` 说明：新变更用 `specs/<change_id>/`；`docs/specs/` 是 2026-11-27
  前的历史/现行扁平契约，迁移另立变更；一个 Spec 一个验收表。
- 自举试点：本变更自己的 Spec 放 `specs/sdd-struct-align-01/`，用它验证
  `check-prerequisites`、`resolve-template`、plan/tasks 流程。
- 优点：不搬 100+ 引用、不破坏已批准 Spec；`specify` 立即可用；符合宪法 30
  “Spec 有目录用目录名 / 扁平文件用文件名主干”。
- 缺点：两种布局并存；需要 `SPECIFY_FEATURE_DIRECTORY`/`feature.json` 指定当前变更；
  `Spec.md` 需要一段说明指向 `specs/README.md`。

### B. 全量迁移

- 把 `docs/specs/0001/0002` 移到 `specs/0001-.../spec.md`，更新 `Spec.md`、`docs/specs/README.md`、
  ADR、台账、脚本、测试中的所有引用。
- 优点：单一布局。缺点：引用面大、已批准契约路径变化、风险高；不适合作为第一个试点。

### C. 只用模板/语法，不接 CLI

- 保持 `docs/specs/` 扁平；不创建 `specs/`；`speckit-specify/plan/tasks/implement` 不使用。
- 优点：零结构变更。缺点：`check-prerequisites`/`implement` 仍不可用；
  “试点”退化为“只用模板”。

---

## 3. 推荐的最小增量（方案 A）

1. `specs/README.md`：双布局规则 + 迁移边界。
2. `specs/sdd-struct-align-01/spec.md`：用 C-2 模板（Spec Kit 原生章节 + 附录 A 对照），
   AC 只覆盖工具链行为（路径解析、失败模式、不迁移、文档、模板）。
3. `specs/sdd-struct-align-01/plan.md`、`tasks.md`：一页以内。
4. 验证：
   - `SPECIFY_FEATURE_DIRECTORY=specs/sdd-struct-align-01 ./.specify/scripts/bash/check-prerequisites.sh --json --require-spec` → exit 0；
   - `--json --require-tasks --include-tasks` → exit 0；
   - `resolve-template.sh spec-template` 含附录 A；
   - `git diff -- docs/specs Spec.md` 为空；核心模板未改。
5. `Spec.md` 加一行指向 `specs/README.md`（新变更布局），不改现有索引行。
6. 审查：独立 reviewer 审 Spec（Spec 阶段）；人类批准后实现并复审。

**不做**：不移动 `docs/specs/`、不改 `Spec.md` 现有条目、不建 `specs/` 以外的路径迁移、
不新增业务行为/AC、不修改核心模板/脚本。

---

## 4. 待确认

- 选 A（双布局 + 自举试点）、B（全量迁移）还是 C（只用模板）。
- 若选 A，确认 `specs/sdd-struct-align-01/` 的 Spec 作为本变更的试点 Spec（含工具链 AC）。
- 若选 A，`Spec.md` 是只加一行指针，还是暂时不动（由 `specs/README.md` 自解释）。

---

## 5. 风险

| 风险 | 处置 |
|---|---|
| 双布局造成“两个真源” | `specs/README.md` 明确：新变更在 `specs/`；`docs/specs/` 是历史/现行扁平契约；一个 Spec 一个验收表；不改旧文件 |
| `feature.json` 是机器本地状态 | 不提交；用 `SPECIFY_FEATURE_DIRECTORY` 或在会话内 `specify` 生成 |
| `specs/` 与 `docs/specs/` 命名混淆 | 目录名用 `<change_id>`；`docs/specs/` 保留 `SPEC-NNNN`；README 说明 |
| 试点 Spec 自举（描述自己） | AC 只覆盖工具链行为，不写业务规则；验收表用未验证起始 |
| 分类 | 宪法 3 无法归入时按契约变更；本变更按“契约变更（工具链）”试点 |
