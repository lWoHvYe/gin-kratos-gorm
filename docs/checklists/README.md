# 按需清单（非规范）

> **性质：操作辅助，不是新的治理层。** 与宪法/AGENTS 冲突时，以宪法/AGENTS 为准。
> 这些文件不保存状态、不新增角色/批准点、不替代 Spec/ADR/审查裁决。

## 为什么单独放

宪法和 AGENTS 只保留**每次都必须遵守**的硬规则；角色细节、五窗口、反模式、审查题库、
工具用法按任务读取，避免每次会话加载 30k 字符。

## 加载规则

| 场景 | 读什么 |
|---|---|
| 每次 | `.specify/memory/constitution.md`、`AGENTS.md`、当前 Spec/ADR |
| 写 Spec/plan/code 前 | `docs/checklists/author.md` |
| 写 ADR/状态机/并发设计 | `docs/checklists/design.md`、`docs/adr/template.md` |
| 审查 | `docs/checklists/reviewer.md` |
| 用 Spec Kit | `docs/guides/spec-kit.md` |
| 历史 | `docs/archive/**`、`docs/convert/**`、旧 QC 报告：**不默认加载** |

## 权威

- 宪法 > AGENTS > 本目录。
- 本目录是“提醒/题库/模板”；不构成新的禁止项或批准条件。
- 如果要把某条清单升级为硬规则，必须走治理变更，写进宪法/AGENTS，不在清单里暗改。
