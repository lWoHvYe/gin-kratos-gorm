# Developer ↔ Reviewer 线程索引

> 只放索引，正文在 `threads/T-NNN.md`（一文件一线程，避免并发冲突）。
> 协议见 [`../CONVENTIONS.md`](../CONVENTIONS.md) §3；目录说明见 [`README.md`](README.md)。

| 编号 | 主题 | 发起 | 阻塞 | 状态 | 落点 | 文件 |
|---|---|---|---|---|---|---|
| T-001 | SPEC-0001 v1.2 实施审查（P0-1…P0-7） | Reviewer 2026-11-22 | 是 | **CLOSED** | P0-7 修复 `f162ecf`；Reviewer 复跑 **PASS**；P0-6 到期项按 `ac-coverage.tsv` 跟踪 | [T-001.md](threads/T-001.md) |
| T-002 | SPEC-0002 v1.7 实施审查与 AC-39…61 延期跟踪 | Reviewer 2026-11-23 | 否 | **OPEN** | Review-01 **FAIL**（P-01…P-06）；AC-39…61 延期跟踪 | [T-002.md](threads/T-002.md) |

## 状态统计

- `OPEN`：1（T-002，待 ADR-0007）
- `ANSWERED`：0
- `CLOSED`：1（T-001，复核通过）

## 维护规则

1. 新增审查线程：在 `threads/` 建 `T-NNN.md`（带 YAML front-matter），再在本表加一行。
2. 状态变化只改两处：线程文件的 `status`、本表对应单元格。
3. 关闭时填 `closed_by`（复跑 commit + 复跑结论），并在本表“落点”列追加 commit。
4. 任何人不得删除历史线程，只追加。
