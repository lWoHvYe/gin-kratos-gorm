# Architect ↔ Developer 协作通道

> **【历史存档 · 已退役】** pair 线程已退役（2026-11-25）；角色交互经 Lead 路由；本目录仅作历史存档，不得新增线程。新增分歧、提问或需他人修改产物一律**上报 Lead 路由至 owner**（`CONVENTIONS.md` §1、§4）。
>
> 通用协议见 [`../CONVENTIONS.md`](../CONVENTIONS.md)（所有 Agent 必读）。
> 本文件只写**这一对角色**的补充规定；冲突时以 CONVENTIONS / AGENTS.md 为准。以下规则均为 **2026-11-25 退役前**的历史协议，不再具现行效力，仅用于读懂 T-001/T-002。

## 目录结构

```text
docs/collab/architect-developer/
├── README.md       本文件：配对补充规则
├── threads.md      线程索引（只有表格）
└── threads/
    └── T-NNN.md    一文件一线程，正文 + YAML front-matter
```

## 配对补充规则（历史协议，2026-11-25 退役）

> 现行做法：不得开线程；ADR 未定义的异常、质疑或需他人修改产物，一律**上报 Lead 路由至 owner**。

1. **（历史）谁写什么**：Architect 写 ADR 与本目录的“答复/裁决”；Developer 写代码、测试、`docs/implementation/**` 偏差登记，以及本目录的“进展/提问/验证”。
2. **（历史）线程只作指针**：业务规则以 `docs/specs/**` 为准，设计以 `docs/adr/**` 为准；线程里不新定规则。
3. **（历史）一问一线程**：Developer 遇到 ADR 未定义的异常**不得自行发明**——退役前须开 `T-NNN` 问；现改为**上报 Lead 路由至 owner**（提问仍须带 `Spec 章节` 与 `ADR 章节`）。
4. **（历史）编号与 trailer**：线程文件仍用 `T-NNN`；commit trailer 用 `Thread: architect-developer/T-NNN` 以避免与 `po-architect/T-NNN` 混淆。2026-11-25 起不再新增线程，`Thread:` trailer 仅见于历史提交。
5. **（历史）关闭由 Architect 执行**：实现合并 + 测试证据 + 偏差 `owner/expiry` 齐全后，Architect 改 `status: CLOSED`、填 `closed_by`、同步索引。退役后历史线程状态冻结，不再改动。
6. **不越界**：Architect 不写业务代码；Developer 不改 Spec/ADR；Reviewer 只审不改。需要对方修改时**上报 Lead 路由至 owner**。

## 与其他文档的关系

| 文档 | 角色 |
|---|---|
| `docs/specs/` | 业务结论，唯一事实源 |
| `docs/adr/` | 设计结论，唯一事实源 |
| `docs/implementation/*-deviation-register.md` | 实现与设计的差异，必须带 `owner` + `expiry` |
| `docs/collab/architect-developer/threads/` | 历史存档（pair 线程 2026-11-25 退役，不得新增），**不作验收依据** |
