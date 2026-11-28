## ADR Traceability（本仓必填，preset append）

- 每条任务必须带 `ADR: <id> §X` 或 `ADR: 无`；契约变更无 ADR 引用不得开始。
- 第一条任务：读 linked ADR + Spec，列出本任务受约束项。
- `research.md` / `data-model.md` / `contracts/` 是可选辅助，不是规范输入；与 Spec/ADR 冲突以 Spec/ADR 为准。
- 实现完成前，逐条核对 ADR 的 MUST / MUST NOT 与验证方式；无法核对则停止。
