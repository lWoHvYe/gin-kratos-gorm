# Architect ↔ Developer 线程索引

> 只放索引，正文在 `threads/T-NNN.md`。协议见 [`../CONVENTIONS.md`](../CONVENTIONS.md) §3，配对规则见 [`README.md`](README.md)。

| 编号 | 主题 | 发起 | 阻塞 | 状态 | 落点 | 文件 |
|---|---|---|---|---|---|---|
| T-001 | SPEC-0001 v1.2 实施交接（B-01/B-05 + v1.2 修正 + T-002 闸门） | Architect 2026-11-22 | 是 | **CLOSED** | ADR-0005 v1.2 §18/§19.2、ADR-0006；实施交接（`c085006`） | [T-001.md](threads/T-001.md) |
| T-002 | 请求追认 Developer 提交 scope 词表 | Developer 2026-11-22 | 否 | **CLOSED** | CONVENTIONS §1/§5.2（`c085006`） | [T-002.md](threads/T-002.md) |

## 状态统计

- `OPEN`：0
- `ANSWERED`：0
- `CLOSED`：2（T-001 实施交接；T-002 scope 追认）

## 维护规则

1. 新增线程：在 `threads/` 建 `T-NNN.md`（带 YAML front-matter），再在本表加一行。
2. 状态变化只改两处：线程文件的 `status`、本表对应单元格。
3. 关闭时填 `closed_by`（commit hash），并在本表“落点”列追加 commit。
4. 任何人不得删除历史线程，只追加。
