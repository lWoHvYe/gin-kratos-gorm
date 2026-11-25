# QC-34 — Developer Prompt 指针（bf1af61）审查

| 项 | 值 |
|---|---|
| review_scope | `bf1af61`（base `3a30156`）：`prompt/Developer.md` 单行指针；目标 `docs/development.md`；对照 `docs/handoff.md`、AGENTS §2.7 治理轻路径、CONVENTIONS |
| 审查类型 | 治理文本 L2 轻路径 QC；不替代 Reviewer / 人类批准 |
| 结论 | **`PASS`** |

> 纯文档审查；未改文件、未跑测试。

## 1. 变更范围

- `git show bf1af61 --stat`：仅 `prompt/Developer.md` 1 个文件、1 insertion；无其他 prompt、无代码/契约/CI/`scripts/**` 改动。**未越界**。
- 新增行（`prompt/Developer.md:4`）：
  > 本地环境/测试/迁移规则以 `docs/development.md` 为准；`docs/handoff.md` 仅是会话快照，不是规则源；若规则只出现在 handoff，停止并请 Lead 路由到 canonical。

## 2. 指针目标

| 检查 | 结果 |
|---|---|
| 文件存在 | `docs/development.md` 存在（3985 bytes，2026-11-25） |
| 职责正确 | 标题即“Development environment and test commands”；自述“durable entry point … not a session snapshot”；覆盖 toolchain/`go build`/`go test`/`make verify`/integration（`TEST_MYSQL_DSN`、`scripts/test-dsn.sh`）/frontend/migrations（immutable、new migration、lease/attempt ledger、fresh MySQL） |
| 状态/契约指针 | 指向 `review-tracking.md`/`roadmap-status.md`（current truth）、`specs/`/`adr/`（contracts）、`handoff.md`（session boundary）、`ops.md`（deploy）——职责分层正确 |
| handoff 语义 | `docs/handoff.md:3-9` 自身写明“snapshot, not a rule or contract source”“Do not use this file as a Developer rule source. If a rule exists only here, stop and ask the Lead to route it to its canonical owner.” 与新增指针**完全一致** |

## 3. 与 AGENTS/CONVENTIONS 的一致性

- AGENTS §0.5（上下文最小化/版本化交付物）与 §2.7 治理轻路径（`prompt/**` L2 轻路径：Architect 起草 → QC → 人类批准 → Reviewer 复核）均被遵守：`Actor: architect-plan-review`、`Tier: L2`、`Reviewed-by: Director`（人类）。
- 未改角色边界：`prompt/Developer.md` 的职责/绝对边界/自检清单原样保留；新增行只把“本地环境/测试/迁移规则”指向 canonical 文档，不改变 Developer 的契约边界。
- 未发现与 AGENTS/CONVENTIONS 冲突的表述；“规则只在 handoff 时停止并请 Lead 路由”与 handoff 自身头部的停损语义一致，也不削弱 AGENTS §1.0 的 Lead 路由职责。

## 4. 非阻断提示

1. 提交 `Reviewed-by: Director` 是**人类批准**；按 AGENTS §2.7 轻路径仍需 Reviewer 复核（本次 QC 不等于 Reviewer 通过）。
2. `docs/development.md` 是 durable 文档；后续若工具链/迁移命令变化，应更新该文件而非回写 `prompt/Developer.md`，保持指针单向。

## 5. 声明

- 我**未修改任何文件**，**未运行测试**（纯文档变更）。
- `PASS` 只针对 `bf1af61` 的单行指针；不构成 Reviewer PASS、人类批准或 Release PASS。
