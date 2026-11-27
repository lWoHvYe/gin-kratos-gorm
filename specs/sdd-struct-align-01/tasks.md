# Tasks: SDD-STRUCT-ALIGN-01

> 勾选是执行进度，不是验收状态；AC 状态见 `spec.md` 验收表。

- [x] T1 写 ADR-0009（双布局决策，Proposed）
- [x] T2 写 `specs/README.md`：双布局规则、`docs/specs/` 边界、一个 Spec 一个验收表
- [x] T3 `Spec.md` 加一行指向 `specs/README.md`；现有索引行不动
- [x] T4 验证 AC-1：`SPECIFY_FEATURE_DIRECTORY=specs/sdd-struct-align-01 ./.specify/scripts/bash/check-prerequisites.sh --json --require-spec`
- [x] T5 验证 AC-2：同目录 `--json --require-tasks --include-tasks`
- [x] T6 验证 AC-3：临时目录缺 plan / 缺 tasks / 缺目录 → exit 非零
- [x] T7 验证 AC-4：`git diff` 对 `docs/specs/0001`/`0002` 为空；`Spec.md` 只多一行
- [x] T8 验证 AC-5：`specs/README.md` 存在且含双布局/一个 Spec 一个验收表
- [x] T9 验证 AC-6：`resolve-template.sh spec-template` 输出含 `## 附录 A 对照`
- [x] T10 独立 reviewer 复核 Spec/plan/tasks + 命令证据；人类批准
