# REV-DEV-POINTER — `prompt/Developer.md` 单行指针

| 项 | 值 |
|---|---|
| review_scope | `3a30156..bf1af61`：`prompt/Developer.md` 的新增单行指针；目标 `docs/development.md`；`docs/handoff.md` 快照语义；L2 治理轻路径、Actor/Reviewed-by、边界。只读纯文档；不评审工作区其他未提交改动。不批准 Release/merge。 |
| base | `3a30156` |
| head | `bf1af61`（`docs(prompt): point Developer to canonical development rules`） |
| 文件 | `prompt/Developer.md` 1 insertion；无其他文件 |
| Tier / Actor | **L2**（治理文本轻路径）；Actor `architect-plan-review`（System Architect）≠ Reviewed-by `Director` |
| QC | QC-34 PASS（见非阻断第 1 条：当前为 untracked，未随本 commit 提交） |
| **verdict** | **PASS（有范围）** |
| release_status | **NOT RELEASE**；不批准合并 |

---

## 1. BLOCKER

**无。**

## 2. 变更范围

- `git show --stat bf1af61`：仅 `prompt/Developer.md` 1 个文件、1 insertion；`git diff 3a30156..bf1af61 --name-only` 仅该文件。
- 新增行（`prompt/Developer.md:4`）：
  > 本地环境/测试/迁移规则以 `docs/development.md` 为准；`docs/handoff.md` 仅是会话快照，不是规则源；若规则只出现在 handoff，停止并请 Lead 路由到 canonical。
- **未改**其他 prompt/角色边界、AGENTS、CONVENTIONS、代码、proto、migrations、CI、scripts；未改 Developer 的职责/绝对边界/自检清单。
- **纯文档**：无测试可跑（与用户“纯文档，不跑测试”一致）。

## 3. 指针目标与职责

| 检查 | 结果 |
|---|---|
| `docs/development.md` 存在且在本 commit 历史中 | **是**；标题 “Development environment and test commands”；自述 durable entry point，明确不是 session snapshot |
| 本地环境/测试规则 | 覆盖 toolchain/`.tmp` cache、`go build`、`go test`、`make verify`、`TEST_MYSQL_DSN`/`scripts/test-dsn.sh`、integration、frontend、evidence 规则 |
| 迁移规则 | 明确 applied 文件 immutable、必须新增 `migrations/<service>/NNNN_name.sql`、lease/attempt ledger、fresh/disposable MySQL、按 service 检查 |
| `docs/handoff.md` 语义 | 文件头明确 “snapshot, not a rule or contract source”“Do not use this file as a Developer rule source. If a rule exists only here, stop and ask the Lead to route it to its canonical owner.” 与新增指针一致 |
| 职责分层 | `development.md` 指向 `review-tracking.md`/`roadmap-status.md`（current truth）、`specs/`/`adr/`（contracts）、`handoff.md`（session boundary）、`ops.md`/UAT runbook；目标职责正确 |
| `Makefile` 引用 | `make verify`、`make test-integration` 均存在；指针不是死命令 |

## 4. Tier / Actor / 轻路径

- commit `bf1af61`：`Role: System Architect`、`change_id: PAY-SETTLE-01`、`Actor: architect-plan-review`、`Tier: L2`、`Reviewed-by: Director`。
- AGENTS §2.7 **治理文本 L2 轻路径**：仅改 `prompt/**`、无代码/契约/CI/scripts 时，Architect 起草 → QC → 人类批准 → Reviewer 复核；本变更仅 `prompt/Developer.md`，QC-34 PASS，Director 复核，Reviewer 本轮复核。**Tier L2 确认**，无 Spec/ADR/Plan 要求。
- **Actor ≠ Reviewed-by**：`architect-plan-review` ≠ `Director`；人类批准在 `Reviewed-by`。
- 新增行只指向 canonical 文档与 Lead 路由，不改变任何业务/角色边界；与 AGENTS §0.5/§1.0 的上下文最小化与 Lead 路由一致。

## 5. 非阻断 / 注意

1. **QC-34 当前为 untracked**：`docs/collab/qc-reports/QC-34-DEV-POINTER.md` 不在 `bf1af61` 的提交中（`git show bf1af61:...` 不存在）。QC 内容 PASS 且可读，但 canonical 交付建议随文档提交或另做 docs commit，避免 L2 轻路径的 QC 证据只存在于工作区。
2. commit message 只有 `Tier: L2`/`Reviewed-by`，未重复 `Tier-Rationale`/`Spec-Ref`/`allowed_paths`；轻路径下 selfcheck/QC 可解释，但属 [TODO] 交付输入完整性问题。
3. 工作区存在与本次指针无关的未提交代码/迁移/脚本改动（`settlement_integration_test.go`、`migrations/order/0018*`、`migrations/fulfillment/0011*`、`scripts/verify-r01-migrations.sh`）；**不在本 commit**，本报告不评价，也不作为指针 PASS 依据。
4. `docs/development.md` 的具体命令/缓存路径本次未执行验证（纯文档复审）；`make verify`/`make test-integration` target 存在，指针目标职责正确。

## 6. 声明

- 本报告未修改文件；纯文档复审，**未跑测试**。
- **PASS 仅覆盖 `3a30156..bf1af61` 的单行指针**：Tier L2 确认、Actor/Reviewed-by 合规、指针目标存在且职责正确、未越界其他角色边界/AGENTS/CONVENTIONS/代码/契约。
- **不是 Release PASS；不批准 merge；不替代人类 merge 决策。**
- 报告：[REV-DEV-POINTER.md](docs/collab/qc-reports/REV-DEV-POINTER.md)
