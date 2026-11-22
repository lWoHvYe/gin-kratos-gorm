# PO ↔ Architect 协作通道

> 通用协议见 [`../CONVENTIONS.md`](../CONVENTIONS.md)（所有 Agent 必读）。
> 本文件只写**这一对角色**的补充规定；冲突时以 CONVENTIONS 为准。

## 目录结构

```text
docs/collab/po-architect/
├── README.md       本文件：配对补充规则
├── threads.md      线程索引（只有表格）
└── threads/
    └── T-NNN.md    一文件一线程，正文 + YAML front-matter
```

## 配对补充规则

1. **谁写什么**：PO 写线程的"裁决"段与 `docs/specs/`；Architect 写"回写"段与 `docs/adr/`。双方不改对方文件。
2. **一次只推进一个阻塞线程**：`BLOCKING` 未 `ANSWERED` 时不开新线程，避免并行发散。
3. **Architect 提问必须带 ADR 章节号**；PO 裁决必须带 Spec 章节号。缺章节号直接退回。
4. **ADR 的"待 PO 裁决"标记**：PO 答复后必须在同一轮删除或改为已决，不允许长期共存（历史已出现过一次漂移）。
5. **关闭由 Architect 执行**：满足 CONVENTIONS §3 三条件后改 `status: CLOSED`、填 `closed_by`、同步索引表。

## 与其他文档的关系

| 文档 | 角色 |
|---|---|
| `docs/specs/` | 业务结论，唯一事实源 |
| `docs/adr/` | 设计结论，唯一事实源 |
| `docs/implementation/*-deviation-register.md` | 实现与设计的差异，必须带 `owner` + `expiry` |
| `docs/collab/po-architect/threads/` | 过程记录，**不作验收依据** |
