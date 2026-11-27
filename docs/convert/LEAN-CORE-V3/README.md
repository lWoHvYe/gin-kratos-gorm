# Lean Core v3.0 设计包

Status: merged on main (human direction); independent review pending; release_status: NOT RELEASE.

| 文件 | 内容 |
|---|---|
| `INVARIANT-MAP.md` | 旧规则 → 保留/替换/退役/延后 |
| `CONSTITUTION.v3.md` | 目标宪法：20 条原则 + 附录 A/B/C |
| `AGENTS.v3.md` | 目标流程：3 角色、契约/实现、审查、证据 |
| `ADOPTION.md` | 单布局迁移、唯一 index、sdd historical、回滚 |
| `PILOT.md` | 两个变更的试点与回滚 |
| `RETRO-2026-11-28.md` | 本轮复盘、根因、风险、防复发规则 |

## 评审标准

只看新设计本身：自洽、完整、可执行、达到目的。**不要**用旧 v2.2.0 条款逐条兼容作为阻断理由；旧文本只作来源。若发现新设计删掉了安全不变量、留下第二权威/死路径、术语不可判定、成本与目的不匹配，才阻断。

## 本轮范围

- 目标：减少 review 轮次/协调成本；防自证/伪造 AC/trailer；高风险证据可执行；流程重量与风险匹配。
- 不做：bugfix 特殊路由、mutation/state model 强制、新增角色。
- 单布局已迁移：0001/0002 在 `specs/`；sdd historical；ADR-0009 被 ADR-0012 取代；docs/specs 只重定向。

## 采用前必须

1. 独立 reviewer PASS（新设计自身）；
2. 人类批准；
3. 两个变更 pilot 指标达标；
4. 否则 revert，不合并。
