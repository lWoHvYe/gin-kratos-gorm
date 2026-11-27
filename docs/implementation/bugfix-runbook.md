# Bugfix Runbook（非规范）

> **性质：非规范操作指南。** 不修改宪法或 `AGENTS.md`，不新增角色/批准点；与宪法/AGENTS
> 冲突时以宪法/AGENTS 为准。它只说明如何按现行“实现变更”规则使用已安装的官方 `bug` 扩展。

适用 change_id：`BUGFIX-<slug>`（在 `docs/review-tracking.md` 登记；不复用该 Spec 的历史 change_id）。

---

## 1. 什么时候可以用

必须**同时**满足：

1. 被破坏的行为已被批准的 Spec 覆盖，能指出**具体 AC**（不是只有一句条款）；
2. 不触及契约主题：资金、库存、履约、权限、跨服务协议/事件、数据迁移、状态机、
   `api/proto/**`、`migrations/**`、`internal/platform/**`、共享包、`frontend/**`、
   `deploy/**`、`scripts/**`、CI 配置；
3. **不命中附录 B 的 ADR 触发条件**：数据模型/迁移、状态机、并发控制方案、技术选型等；
4. 不是意图变化（不是新增/修改行为/AC）；
5. 能写 failing regression test，并给出 plan（内容见 §2）。

任一条不满足 → 按契约变更走 Spec/ADR，不用本 runbook。

---

## 2. Plan 内容（不设页数上限，只要求不写机制）

在 `docs/implementation/bugfix/<slug>/plan.md` 写：

- `baseline_impact`：受影响的 Spec/AC/INV；不涉及也要写依据；
- `stop_conditions`；
- 受影响 AC → failing regression test；
- 实现范围（只修根因，不夹带重构/新行为）；
- 如果根因是架构/设计问题：停止，回 Spec/ADR，不在此 plan 里设计。

不要求新 Spec/ADR；plan 只记录本次修复，不写状态机/协议/数据模型设计。

---

## 3. 流程

```text
1. 作者登记 change_id=BUGFIX-<slug>、级别=实现变更、Spec/AC、base
2. 写 docs/implementation/bugfix/<slug>/plan.md
3. /speckit-bug-assess <report> slug=<slug>   # 只诊断，不改代码
4. 停下：作者核对 assessment 的根因/remediation、分类条件；不成立则回契约变更
5. /speckit-bug-fix slug=<slug>               # 只按 assessment 修复
6. /speckit-bug-test slug=<slug>              # 只验证，不为通过改代码
7. 作者写 review 证据：diff、regression 日志（绑定 HEAD）、【测试范围】【未覆盖范围】【取舍】
8. 审查者按 AGENTS §3 代码阶段审查；更新受影响 Spec 验收表/台账 AC 位移
9. human 按 AGENTS §2 第 7 步合入
```

- 第 3–6 步不是无人值守连续执行：assessment 后必须停下核对；fix 后、test 前也停下。
- 审查 FAIL 分三类：
  - 分类/范围不成立 → 回契约变更；
  - 证据不足（plan/regression/日志缺） → 回第 2/5 步补；
  - 架构/设计问题，或同一问题第二次不过且判为设计问题 → 冻结，回 Spec/ADR。
- `bug-test` 的 `verified/partial/failed` 只是证据，**不是 AC 状态**，也不等于 reviewer PASS/Release。
- 受影响 AC 的状态只在 Spec 验收状态表更新；台账写 AC 位移。

---

## 4. 证据路径

```text
docs/implementation/bugfix/<slug>/plan.md
docs/implementation/bugfix/<slug>/review.md        # 审查报告/verdict
docs/implementation/bugfix/<slug>/regression.log   # 绑定 git rev-parse HEAD
.specify/bugs/<slug>/assessment.md
.specify/bugs/<slug>/fix.md
.specify/bugs/<slug>/test.md
```

- 缺 plan/review/regression/assessment/fix/test 任一，审查者按证据不足退回。
- 回归日志必须绑定精确 HEAD；测试范围/未覆盖/取舍写在 review.md。
- `docs/implementation/**` 是在途变更的既有证据路径（AGENTS §5），不新增状态源。

---

## 5. 人类批准点

不变：人类仍只批准 Spec、合入、Release（宪法第 16 条）。bugfix 不新增人类批准点；
不使用官方 `bugfix` workflow 的 human gate，只使用 `bug` extension 的 assess/fix/test 命令。

---

## 6. 试点与回退

- 先选一个满足 §1 的真实 bug；记录是否新建 Spec/ADR、plan 内容、assessment 根因、
  test 是否复现、reviewer 轮次、lead time、report/代码比。
- 若需要新 Spec/ADR、改行为/AC、或触及 §1 任一条件：停止，改契约变更。
- 若 assessment 无根因、test 不复现、无 regression：FAIL，不进入 fix。
- 本 runbook 不改 live 宪法/AGENTS；若试点显示没有减少 lead time，删除本 runbook 即可，
  不需要治理回滚。

---

## 7. 与 v1/v2 治理草案的关系

- v1/v2 治理草案均被独立 reviewer FAIL；**不应用**。
- 本 runbook 不新增 Article 3 的 bugfix 分类，不引入 change_id/AC/第 4–8 步的例外；
  它只是把现行“实现变更”规则写成可操作步骤。
- 若未来要把 bugfix 变成宪法级路由，需另开治理变更，并按 reviewer 的 F1–F10 重写。
