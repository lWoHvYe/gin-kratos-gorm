# Lean Core v3.0 设计包

Status: **design package**；v3 core 已应用到 trial 分支 `reform/lean-core`（`d90d850`），未合并 main、未生效。

| 文件 | 内容 |
|---|---|
| `INVARIANT-MAP.md` | 旧规则 → 保留/替换/退役/延后 |
| `CONSTITUTION.v3.md` | 目标宪法：19 条原则 + 附录 A/B/C |
| `AGENTS.v3.md` | 目标流程：3 角色、契约/实现、审查、证据 |
| `ADOPTION.md` | process-only adoption；不改 Spec 布局/0001/0002 |
| `PILOT.md` | 两个变更的试点与回滚 |

## 评审标准

只看新设计本身：自洽、完整、可执行、达到目的。**不要**用旧 v2.2.0 条款逐条兼容作为阻断理由；旧文本只作来源。若发现新设计删掉了安全不变量、留下第二权威/死路径、术语不可判定、成本与目的不匹配，才阻断。

## 本轮范围

- 目标：减少 review 轮次/协调成本；防自证/伪造 AC/trailer；高风险证据可执行；流程重量与风险匹配。
- 不做：0001/0002 迁移、ADR-0009 迁移、bugfix 特殊路由、mutation/state model 强制、新增角色。
- 本轮只改流程；0001/0002、Spec 布局、sdd、ADR-0009 原样保留；Spec 权威另案。

## 采用前必须

1. 独立 reviewer PASS（新设计自身）；
2. 人类批准；
3. 两个变更 pilot 指标达标；
4. 否则 revert，不合并。
