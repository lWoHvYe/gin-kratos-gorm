# Spec Kit 使用指南（非规范）

> 操作辅助，冲突以宪法/AGENTS 为准。Spec Kit 1.4.5；官方流程是 SDD，但**不是所有变更都必须走全套**。

## 1. 先选流程

| 场景 | 用法 |
|---|---|
| 契约/feature 变更 | `specify → clarify（仅必要时）→ plan → tasks → implement → converge` |
| 实现变更（已有 Spec 覆盖） | 现有 Spec + plan/tasks + tests；不重新 specify |
| bugfix | 不设特殊路由：已批准 Spec 覆盖且无契约主题→实现变更；否则契约变更。`bug` extension 可作工具 |
| idea/assessment | 官方 assess 流程；不进入实现 |

- 只有 `specify` 是 `plan` 前严格必需的；`clarify`/`checklist`/`analyze` 是可选质量门。
- 不要求每个变更生成全套 `spec.md/plan.md/tasks.md/research/contracts/quickstart`。

## 2. Spec 持久化

- 所有 active Spec 在 `specs/<id>/spec.md`；`specs/README.md` 是唯一 index。
- `0001` 为 historical reference（复审状态见台账）；`0002` 为 active；`sdd-struct-align-01` 为 historical。
- `docs/specs/` 只做重定向；不保留副本。
- 新工作 `specs/<feature>`；变更原地更新；evidence 放 `specs/<id>/evidence/<change_id>/`。
- Spec Kit 模板中的 `[###-feature-name]` 只是占位；active Spec 以 `specs/<id>/spec.md` 为准，`<id>` 可为 `NNNN-slug` 或 change_id，不要求数字前缀。

## 3. 命令注意事项

- `clarify`：最多 5 个问题；解决最高歧义，其余进 Spec 的业务裁决题。
- `checklist`：作者自检/需求质量清单，不是验收。
- `analyze`：跨 artifact 一致性 lint；不产生 PASS。
- `implement`：默认连续执行；必须按 tasks 阶段分批，阶段边界主动中止。
- `converge`：append-only；只在 implement 后使用，最多 2 轮；仍 append 就停，回 plan/Spec。
- `speckit-implement` 没有阶段参数；分批靠操作者。

## 3.1 ADR 桥接（非规范）

- Spec Kit 不读 ADR；`plan` 必须输出 **ADR Constraints** 表，`tasks` 每条带 `ADR: §X`。
- `implement` 只读 tasks；不在 tasks 里的 ADR 约束等于没写。
- 本仓用 preset append 提供这两段；不要改核心模板，也不要把“obey ADR”只写在命令附言里。
- `research.md` / `data-model.md` / `contracts/` 是可选辅助，不是规范输入。

## 4. 模板与扩展

- 不直接改 `.specify/templates/**`、`.specify/scripts/**` 核心文件。
- 追加章节用 preset/extension 的 `append`；整体替换用 `overrides/`；不要用 override 冒充 append。
- 已安装官方 `bug` bundle（`bug` v1.0.2 + workflow v1.0.1）；`bug` 的 assess/fix/test 可作工具，但不构成 bugfix 特殊路由。
- 社区 preset/extension 未审计；安装前审源码、版本、artifact 预算。
- `test-first-governance` 等重型 preset 不默认安装（10 模板/65KB/90-85 coverage/BDD-ATDD）。

## 5. 官方参考

- SDD 命令：https://github.github.io/spec-kit/reference/agentic-sdd.html
- Spec 持久化：https://github.github.io/spec-kit/how-to/persisting-specs.html
- Evolving specs：https://github.github.io/spec-kit/how-to/evolving-specs.html
- Contract-driven：https://github.github.io/spec-kit/how-to/contract-driven-development.html
- Bug fixing：https://github.github.io/spec-kit/guides/bugfix.html
