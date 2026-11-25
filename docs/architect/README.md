# Architect 记忆区说明

- 所有者：System Architect（其他角色只读）。
- `MEMORY.md`：跨会话状态索引（当前 Spec/ADR 版本、未完成项、易漂移点、闸门）。
- 这里**不放业务规则**；业务以 `docs/specs/**` 为准，设计以 `docs/adr/**` 为准。
- pair 线程已退役（2026-11-25），本目录不新开线程，角色交互经 Lead 路由。
- 需要改设计结论：**上报 Lead 路由至 owner**（Architect）→ 改 ADR（Amendment/Rev）→ 更新 `MEMORY.md` 指针。
- 需要改业务口径：**上报 Lead 路由至 owner/人类**（PO 拥有 Spec），不在本目录做裁决。
