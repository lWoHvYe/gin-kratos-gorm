# Spec Kit 使用指南（非规范）

> 操作辅助，冲突以宪法/AGENTS 为准。Spec Kit 1.4.5；官方流程是 SDD，但**不是所有变更都必须走全套**。

## 1. 先选流程

| 场景 | 用法 |
|---|---|
| 契约/feature 变更 | `specify → clarify（仅必要时）→ plan → tasks → implement → converge` |
| 实现变更（已有 Spec 覆盖） | 现有 Spec + plan/tasks + tests；不重新 specify |
| bugfix | 官方 `bug` extension：assess → fix → test；不新建 Spec |
| idea/assessment | 官方 assess 流程；不进入实现 |

- 只有 `specify` 是 `plan` 前严格必需的；`clarify`/`checklist`/`analyze` 是可选质量门。
- 不要求每个变更生成全套 `spec.md/plan.md/tasks.md/research/contracts/quickstart`。

## 2. Spec 持久化

- 既有 `docs/specs/` flat Spec：按 `docs/specs/README.md` 原地升版，不迁移、不复制。
- 新 capability：`specs/<feature>/spec.md` 是该 feature 的权威 Spec；后续变更原地更新。
- 对既有 Spec 的变更：`specs/<change_id>/` 只放 plan/tasks/evidence；不要再生成第二份权威 spec.md。
- 一个业务能力一个权威位置；`docs/specs/README.md` 的 spec index 记录权威位置。
- 不用 Spec Kit 的 per-change branch 去复制长期 Spec。

## 3. 命令注意事项

- `clarify`：最多 5 个问题；解决最高歧义，其余进 Spec 的业务裁决题。
- `checklist`：作者自检/需求质量清单，不是验收。
- `analyze`：跨 artifact 一致性 lint；不产生 PASS。
- `implement`：默认连续执行；必须按 tasks 阶段分批，阶段边界主动中止。
- `converge`：append-only；只在 implement 后使用，最多 2 轮；仍 append 就停，回 plan/Spec。
- `speckit-implement` 没有阶段参数；分批靠操作者。

## 4. 模板与扩展

- 不直接改 `.specify/templates/**`、`.specify/scripts/**` 核心文件。
- 追加章节用 preset/extension 的 `append`；整体替换用 `overrides/`；不要用 override 冒充 append。
- 已安装官方 `bugfix` bundle（`bug` v1.0.2 + workflow v1.0.1）；本仓只用 `bug` 的 assess/fix/test，
  不使用 workflow 的 human gate。
- 社区 preset/extension 未审计；安装前审源码、版本、artifact 预算。
- `test-first-governance` 等重型 preset 不默认安装（10 模板/65KB/90-85 coverage/BDD-ATDD）。

## 5. 官方参考

- SDD 命令：https://github.github.io/spec-kit/reference/agentic-sdd.html
- Spec 持久化：https://github.github.io/spec-kit/how-to/persisting-specs.html
- Evolving specs：https://github.github.io/spec-kit/how-to/evolving-specs.html
- Contract-driven：https://github.github.io/spec-kit/how-to/contract-driven-development.html
- Bug fixing：https://github.github.io/spec-kit/guides/bugfix.html
