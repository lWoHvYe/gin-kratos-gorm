# PO ↔ Architect 线程索引

> 只放索引，正文在 `threads/T-NNN.md`（一文件一线程，避免并发冲突）。
> 协议见 [`../CONVENTIONS.md`](../CONVENTIONS.md) §3；目录说明见 [`README.md`](README.md)。

| 编号 | 主题 | 发起 | 阻塞 | 状态 | 裁决/落点 | 文件 |
|---|---|---|---|---|---|---|
| T-001 | 多仓拆单 P-01…P-18 业务歧义 | Architect 2026-11-22 | 是 | **CLOSED** | SPEC-0001 v1.2 §15（`c8b9a2d`）+ ADR-0005 v1.2 §19.2/§20（commit pending） | [T-001.md](threads/T-001.md) |
| T-002 | 三条协作闸门可执行化（所有权/版本/AC↔测试） | PO 2026-11-22 | 否 | **ANSWERED** | ADR-0006 + CONVENTIONS 附录 A；待 Developer 实现 + Reviewer A1…A5 | [T-002.md](threads/T-002.md) |

## 状态统计

- `OPEN`：0
- `ANSWERED`：1（T-002，等 Developer 实现 + Reviewer A1…A5）
- `CLOSED`：1（T-001）

## 维护规则

1. 新增线程：在 `threads/` 建 `T-NNN.md`（带 YAML front-matter），再在本表加一行。
2. 状态变化只改两处：线程文件的 `status`、本表对应单元格。
3. 关闭时填 `closed_by`（commit hash），并在本表"裁决/落点"列追加 commit。
4. 任何人不得删除历史线程，只追加。
