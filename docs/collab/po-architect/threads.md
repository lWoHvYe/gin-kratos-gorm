# PO ↔ Architect 线程索引

> **【历史存档 · 已退役】** pair 线程已退役（2026-11-25）；角色交互经 Lead 路由；本目录仅作历史存档，不得新增线程。任何历史线程不得删除或改写，仅保留原样（`CONVENTIONS.md` §4）。
>
> 只放索引，正文在 `threads/T-NNN.md`（一文件一线程，避免并发冲突）。
> 协议见 [`../CONVENTIONS.md`](../CONVENTIONS.md) §4（已退役，历史说明）；目录说明见 [`README.md`](README.md)。

| 编号 | 主题 | 发起 | 阻塞 | 状态 | 裁决/落点 | 文件 |
|---|---|---|---|---|---|---|
| T-001 | 多仓拆单 P-01…P-18 业务歧义 | Architect 2026-11-22 | 是 | **CLOSED** | SPEC-0001 v1.2 §15（`c8b9a2d`）+ ADR-0005 v1.2 §19.2/§20（`5f421a5`） | [T-001.md](threads/T-001.md) |
| T-002 | 三条协作闸门可执行化（所有权/版本/AC↔测试） | PO 2026-11-22 | 否 | **CLOSED** | ADR-0006 + CONVENTIONS 附录 A；A1…A5 通过（`0212efa`） | [T-002.md](threads/T-002.md) |
| T-003 | 闸门有效性补强：覆盖率≠断言深度、到期阻断、扫描范围可见 | PO 2026-11-22 | 否 | **CLOSED** | 2026-11-25 闸门退役，ADR-0006 v1.1 / Amendment 1 未实现即 superseded（`docs/review-tracking.md` gate retirement） | [T-003.md](threads/T-003.md) |

## 状态统计

- `OPEN`：0
- `ANSWERED`：0
- `CLOSED`：3（T-001、T-002、T-003）

## 维护规则（已冻结，2026-11-25 起）

1. **不再新增线程**：新的分歧、提问或需他人修改产物，一律**上报 Lead 路由至 owner**，不得在本目录建 `T-NNN.md` 或加行。
2. 历史线程的 `status` 与上表单元格**保持原状**，不再变更（T-001…T-003 全部 `CLOSED`，无需再推进）。
3. 关闭记录（`closed_by`、commit）只作历史阅读，不再补写或回改。
4. 任何人不得删除或改写历史线程，只读存档。
