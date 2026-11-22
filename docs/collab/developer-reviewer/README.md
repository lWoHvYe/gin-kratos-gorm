# Developer ↔ Reviewer 协作通道

> 通用协议见 [`../CONVENTIONS.md`](../CONVENTIONS.md)（所有 Agent 必读）。
> 本文件只写**这一对角色**的补充规定；冲突时以 CONVENTIONS / AGENTS.md 为准。

## 目录结构

```text
docs/collab/developer-reviewer/
├── README.md       本文件：配对补充规则
├── threads.md      线程索引（只有表格，不放正文）
└── threads/
    └── T-NNN.md    一文件一线程，正文 + YAML front-matter
```

## 配对补充规则

1. **谁写什么**：Reviewer 写审查结论、AC 逐条核对、红队问题、放行条件；Developer 写修复进展、commit、测试日志、自检清单回复。双方不改对方已写内容。
2. **一次只推进一个阻塞审查**：`T-NNN` 为 `OPEN + BLOCKING` 时不开新审查线程；Developer 完成修复并回写证据后，Reviewer 复跑再改 `ANSWERED/CLOSED`。
3. **Reviewer 提问必须给证据**：契约编号（Spec/ADR/AGENTS/CONVENTIONS 章节）+ `文件:行号` + 复现路径；禁止“大概/应该/可能”。
4. **Developer 回复必须给证据**：修复 commit（`fix(...)` / `feat(...)`）、测试命令与终端输出、`[测试范围]` / `[未覆盖范围]`、三问自检清单；仅口头“done”无效。
5. **Reviewer 不改代码**：只提议；Developer 实施。Developer 对结论有异议时，开线程给反证或升级 Architect/人类裁决，**不得静默豁免**。
6. **关闭由 Reviewer 执行**：所有 BLOCKING 项关闭、偏差有 `owner + expiry`、AC 覆盖缺口进台账或有测试、复跑绿后，改 `status: CLOSED`、填 `closed_by`、同步索引表。
7. **审查结论的效力**：线程只是过程记录，验收以 `docs/specs/**` + `docs/adr/**` + 代码/测试实际结果为准；`[TODO]` 非阻断项不得反向升级为 PASS 理由。

## 与其他文档的关系

| 文档 | 角色 |
|---|---|
| `docs/specs/` | 业务结论，唯一事实源；Reviewer 只读 |
| `docs/adr/` | 设计结论，唯一事实源；Reviewer 只读 |
| `docs/implementation/*-deviation-register.md` | 实现偏差，必须带 `owner` + `expiry`；Developer 写，Reviewer 核验 |
| `docs/reviewer/MEMORY.md` | Reviewer 长期记忆/闸门状态 |
| `docs/collab/ac-baseline.txt` / `ac-coverage.tsv` | 闸门台账；Reviewer 维护 |
| `docs/collab/developer-reviewer/threads/` | 审查过程，**不作验收依据** |
