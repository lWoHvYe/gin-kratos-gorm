# Developer ↔ Reviewer 线程索引

> 只放索引，正文在 `threads/T-NNN.md`（一文件一线程，避免并发冲突）。
> 协议见 [`../CONVENTIONS.md`](../CONVENTIONS.md) §3；目录说明见 [`README.md`](README.md)。

| 编号 | 主题 | 发起 | 阻塞 | 状态 | 落点 | 文件 |
|---|---|---|---|---|---|---|
| T-001 | SPEC-0001 v1.2 实施审查（HEAD `8ea9236`）— **FAIL** | Reviewer 2026-11-22 | 是 | **OPEN** | P0-1…P0-6 已回证；**P0-7 运费 hold 结清待修**（不得开 `FREIGHT_RESPONSIBILITY_ENABLED`） | [T-001.md](threads/T-001.md) |

## 状态统计

- `OPEN`：1（T-001，等 Developer 修复 + 回证）
- `ANSWERED`：0
- `CLOSED`：0

## 维护规则

1. 新增审查线程：在 `threads/` 建 `T-NNN.md`（带 YAML front-matter），再在本表加一行。
2. 状态变化只改两处：线程文件的 `status`、本表对应单元格。
3. 关闭时填 `closed_by`（复跑 commit + 复跑结论），并在本表“落点”列追加 commit。
4. 任何人不得删除历史线程，只追加。
