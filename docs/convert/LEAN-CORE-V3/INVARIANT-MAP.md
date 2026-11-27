# Invariant Map — v3.0

> 旧宪法/AGENTS/README 是来源材料，不是兼容基线。保留不变量，替换机制；
> 单布局已迁移：active Spec 在 `specs/`；`docs/specs/` 只重定向；ADR-0009 被 ADR-0012 取代。

| 旧内容 | 处理 | v3.0 落点 |
|---|---|---|
| 契约先行；行为变更先 Spec | 保留 | 宪法 I / 1–2 |
| 独立审查；作者不审自己 | 保留 | 宪法 II / 6–8 |
| 证据绑定 HEAD；口头 done 无效 | 保留 | 宪法 III / 11 |
| AC 状态只由审查者写 | 保留 | 宪法 II / 10 |
| Fail-Closed；不吞异常；依赖 fail-fast | 保留 | 宪法 III / 14 + 附录 C |
| 状态所有权；四个事实分离 | 保留 | 宪法 I / 4–5 + 附录 C2 |
| permit/退款竞态；已发货只退款；`REVOKED`；commitment/Σ | 保留 | 附录 C2 |
| 幂等/唯一约束/CAS；Outbox/DTM/gid；持久恢复 | 保留 | 附录 C1 |
| 读写隔离；多仓 `warehouse_id`；静态守卫 | 保留 | 附录 C3 |
| 人类批准 Spec/ADR/合入/Release | 保留 | 宪法 IV / 16–17 |
| 不伪造 trailer；不用 hotfix/新 change_id 绕过 FAIL | 保留 | 宪法 III / 15 |
| 已批准 Spec/ADR 不静默改写；新 ADR 取代 | 保留 | 宪法 I / 3 |
| 38 条原则/6 角色/QC/PO/Architect/Reviewer | 替换 | 19 条原则 + 作者/审查者/人类 |
| pair 线程/MEMORY/gates/tsv/多份 QC 报告 | 退役 | 不进入 v3.0 |
| L0/L1/L2、QC 五窗口、审计清单 | 退役/非规范 | 仅 `docs/checklists/**` 参考，不作验收 |
| bugfix 特殊路由 | 不做 | bugfix 只是实现/契约分类（宪法 I / 2） |
| verification matrix / mutation / state model 强制 | 不进核心 | 非规范试点指南 |
| 0001/0002 迁移；Spec 双/单布局 | **已迁移为单一布局** | `specs/0001.../spec.md`、`specs/0002.../spec.md`；`docs/specs/` 重定向；见 `ADOPTION.md` |
| `Spec.md`/README 状态表/索引 | **改为指针/唯一 index** | `Spec.md` 指针；`specs/README` 唯一 index；`docs/specs/README` 重定向 |
| 以报告/commit 数量衡量 | 不保留 | 以 review 轮次/lead time/escaped blocker 衡量（`PILOT.md`） |

## 设计原则

1. **安全不变量优先于流程精简**：可以删角色/文档，不能删 Fail-Closed、permit/退款、证据、独立审查。
2. **机制可替换，教训不可丢**：旧角色是机制；旧事故教训是约束。
3. **旧文本不逐条兼容**：v3.0 是替代文本；冲突以 v3.0 为准，旧文本归档。
4. **live 状态已迁移**：0001/0002 在 `specs/`；ADR-0009 由 ADR-0012 取代；`sdd` historical。
5. **可试错**：v3.0 只跑 2 个 change；指标不改善就回滚。
