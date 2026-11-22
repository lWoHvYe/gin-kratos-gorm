# Architect ↔ Developer 协作通道

> 通用协议见 [`../CONVENTIONS.md`](../CONVENTIONS.md)（所有 Agent 必读）。
> 本文件只写**这一对角色**的补充规定；冲突时以 CONVENTIONS / AGENTS.md 为准。

## 目录结构

```text
docs/collab/architect-developer/
├── README.md       本文件：配对补充规则
├── threads.md      线程索引（只有表格）
└── threads/
    └── T-NNN.md    一文件一线程，正文 + YAML front-matter
```

## 配对补充规则

1. **谁写什么**：Architect 写 ADR 与本目录的“答复/裁决”；Developer 写代码、测试、`docs/implementation/**` 偏差登记，以及本目录的“进展/提问/验证”。
2. **线程只作指针**：业务规则以 `docs/specs/**` 为准，设计以 `docs/adr/**` 为准；线程里不新定规则。
3. **一问一线程**：Developer 遇到 ADR 未定义的异常，必须开 `T-NNN` 问，不得自行发明；提问必须带 `Spec 章节` 与 `ADR 章节`。
4. **编号与 trailer**：线程文件仍用 `T-NNN`；commit trailer 用 `Thread: architect-developer/T-NNN`，避免与 `po-architect/T-NNN` 混淆。
5. **关闭由 Architect 执行**：实现合并 + 测试证据 + 偏差 `owner/expiry` 齐全后，Architect 改 `status: CLOSED`、填 `closed_by`、同步索引。
6. **不越界**：Architect 不写业务代码；Developer 不改 Spec/ADR；Reviewer 只审不改。

## 与其他文档的关系

| 文档 | 角色 |
|---|---|
| `docs/specs/` | 业务结论，唯一事实源 |
| `docs/adr/` | 设计结论，唯一事实源 |
| `docs/implementation/*-deviation-register.md` | 实现与设计的差异，必须带 `owner` + `expiry` |
| `docs/collab/architect-developer/threads/` | 过程记录，**不作验收依据** |
