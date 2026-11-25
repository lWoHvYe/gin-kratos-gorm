# Architect ↔ Developer 线程索引

> **【历史存档 · 已退役】** pair 线程已退役（2026-11-25）；角色交互经 Lead 路由；本目录仅作历史存档，不得新增线程。任何历史线程不得删除或改写，仅保留原样（`CONVENTIONS.md` §4）。
>
> 只放索引，正文在 `threads/T-NNN.md`。协议见 [`../CONVENTIONS.md`](../CONVENTIONS.md) §4（已退役，历史说明），配对规则见 [`README.md`](README.md)。

| 编号 | 主题 | 发起 | 阻塞 | 状态 | 落点 | 文件 |
|---|---|---|---|---|---|---|
| T-001 | SPEC-0001 v1.2 实施交接（B-01/B-05 + v1.2 修正 + T-002 闸门） | Architect 2026-11-22 | 是 | **CLOSED** | ADR-0005 v1.2 §18/§19.2、ADR-0006；实施交接（`c085006`） | [T-001.md](threads/T-001.md) |
| T-002 | 请求追认 Developer 提交 scope 词表 | Developer 2026-11-22 | 否 | **CLOSED** | CONVENTIONS §1/§5.2（`c085006`） | [T-002.md](threads/T-002.md) |

## 状态统计

- `OPEN`：0
- `ANSWERED`：0
- `CLOSED`：2（T-001 实施交接；T-002 scope 追认）

## 维护规则（已冻结，2026-11-25 起）

1. **不再新增线程**：新的分歧、提问或需他人修改产物，一律**上报 Lead 路由至 owner**，不得在本目录建 `T-NNN.md` 或加行。
2. 历史线程的 `status` 与上表单元格**保持原状**，不再变更（T-001、T-002 均 `CLOSED`）。
3. 关闭记录（`closed_by`、commit）只作历史阅读，不再补写或回改。
4. 任何人不得删除或改写历史线程，只读存档。
