# PO ↔ Architect 协作通道

> **【历史存档 · 已退役】** pair 线程已退役（2026-11-25）；角色交互经 Lead 路由；本目录仅作历史存档，不得新增线程。新增分歧、提问或需他人修改产物一律**上报 Lead 路由至 owner**（`CONVENTIONS.md` §1、§4）。
>
> 通用协议见 [`../CONVENTIONS.md`](../CONVENTIONS.md)（所有 Agent 必读）。
> 本文件只写**这一对角色**的补充规定；冲突时以 CONVENTIONS 为准。以下规则均为 **2026-11-25 退役前**的历史协议，不再具现行效力，仅用于读懂 T-001…T-003。

## 目录结构

```text
docs/collab/po-architect/
├── README.md       本文件：配对补充规则
├── threads.md      线程索引（只有表格）
└── threads/
    └── T-NNN.md    一文件一线程，正文 + YAML front-matter
```

## 配对补充规则（历史协议，2026-11-25 退役）

> 现行做法：不得开线程；任何分歧、提问或需他人修改产物，一律**上报 Lead 路由至 owner**。

1. **（历史）谁写什么**：PO 写线程的"裁决"段与 `docs/specs/`；Architect 写"回写"段与 `docs/adr/`。双方不改对方文件。
2. **（历史）一次只推进一个阻塞线程**：`BLOCKING` 未 `ANSWERED` 时不并行推进。线程协议退役后不再新开线程，需要裁决一律上报 Lead 路由至 owner。
3. **（历史）Architect 提问必须带 ADR 章节号**；PO 裁决必须带 Spec 章节号。缺章节号直接退回。现行提问同样须带章节号，但经 Lead 路由。
4. **（历史）ADR 的"待 PO 裁决"标记**：PO 答复后必须在同一轮删除或改为已决，不允许长期共存（历史已出现过一次漂移）。
5. **（历史）关闭由 Architect 执行**：满足当时线程协议的三条件（现见 `CONVENTIONS.md` §4 历史说明）后改 `status: CLOSED`、填 `closed_by`、同步索引表。退役后历史线程状态冻结，不再改动。

## 与其他文档的关系

| 文档 | 角色 |
|---|---|
| `docs/specs/` | 业务结论，唯一事实源 |
| `docs/adr/` | 设计结论，唯一事实源 |
| `docs/implementation/*-deviation-register.md` | 实现与设计的差异，必须带 `owner` + `expiry` |
| `docs/collab/po-architect/threads/` | 历史存档（pair 线程 2026-11-25 退役，不得新增），**不作验收依据** |
