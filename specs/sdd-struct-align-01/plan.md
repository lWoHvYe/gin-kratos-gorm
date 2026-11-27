# Plan: SDD-STRUCT-ALIGN-01（Spec Kit 双布局试点）

## Technical Context

- 变更类型：契约变更（工具链/路径；宪法 3 兜底）。
- 技术栈：Bash + Spec Kit 1.4.3 + PyYAML 6.0.4（user-site）；**不改 Go 代码**。
- 设计决策：`docs/adr/0009-spec-kit-dual-layout.md`（Proposed）。
- 现有契约：`docs/specs/0001-...md`、`docs/specs/0002-...md` 原地不动。

## Constitution Check

- 契约先行：本 Spec + ADR-0009；人类批准前不实现。
- 独立审查：reviewer subagent 审 Spec/plan/tasks + 命令实测。
- 状态唯一：AC 状态只在 §验收状态表；运行状态只在 `docs/review-tracking.md`。
- 领域不变量：不涉及资金/库存/权限/跨服务/状态机；无 INV。
- 手工改生成物/核心模板：不涉及；C-2 preset 只 append。

## 实施顺序

1. **T1**：ADR-0009（已写，待审查）。
2. **T2**：`specs/README.md`（双布局规则；实现阶段）。
3. **T3**：`Spec.md` 加一行指针（实现阶段）。
4. **T4**：AC-1/AC-2 命令验证（本仓 `specs/sdd-struct-align-01/`）。
5. **T5**：AC-3 失败用例（临时目录：缺 plan、缺 tasks、缺目录）。
6. **T6**：AC-6 `resolve-template.sh` 含附录 A。
7. **T7**：独立 reviewer 复核 + 人类批准；实现后复审。

## 证据

- `SPECIFY_FEATURE_DIRECTORY=specs/sdd-struct-align-01 ./.specify/scripts/bash/check-prerequisites.sh --json --require-spec`（FEATURE_DIR）
- `... --paths-only --json`（FEATURE_SPEC）
- `SPECIFY_FEATURE_DIRECTORY=specs/sdd-struct-align-01 ./.specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks`
- `./.specify/scripts/bash/resolve-template.sh spec-template | grep -F '## 附录 A 对照'`
- `git diff -- docs/specs/0001-fulfillment-quantity-split.md docs/specs/0002-payment-settlement-admission.md`
- `git diff -- Spec.md`（实现后只应多一行指针）

## 风险与回退

- 双布局：`specs/README.md` 说明边界；不改 `docs/specs/`。
- `feature.json` 本地状态：不提交；用 env 或 `specify` 生成。
- 回退：删除 `specs/` 与 ADR-0009、撤销 `Spec.md` 指针即可；`docs/specs/` 无改动。

## 不做

- 不迁移 `docs/specs/`；不改现有 `Spec.md` 索引行；不改核心模板/脚本；不新增业务 AC。
