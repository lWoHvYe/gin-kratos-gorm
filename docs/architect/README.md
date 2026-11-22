# Architect 记忆区说明

- 所有者：System Architect（其他角色只读）。
- `MEMORY.md`：跨会话状态索引（当前 Spec/ADR 版本、未完成项、易漂移点、闸门）。
- 这里**不放业务规则**；业务以 `docs/specs/**` 为准，设计以 `docs/adr/**` 为准。
- 需要改设计结论：开线程 → 改 ADR（Amendment）→ 更新 `MEMORY.md` 指针。
- 需要改业务口径：开线程给 PO，不在本目录做裁决。
