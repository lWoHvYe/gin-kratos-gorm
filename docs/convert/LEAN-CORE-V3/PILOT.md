# v3.0 pilot

Status: design, not started.

## 目标

在两个**非 T5、非 legacy 迁移**的真实变更上验证；先记录 v2.2.0 下两个可比变更的 review 轮次/lead time 作为基线，没有基线就记录当前值：

- review 轮次是否 ≤2（超过由人类分类，不自动放行）；
- lead time 是否下降；
- escaped blocker 是否为 0；
- 文档新增/修改行数 ÷ 代码改动行数是否 ≤1（纯文档变更不适用，记 N/A）；
- human 分钟是否只花在决策/合入/Release。

## 采用步骤（批准后）

1. 应用 `CONSTITUTION.v3.md`、`AGENTS.v3.md` 的 process 部分；
2. **不改** Spec 布局/README/`sdd`/0001/0002；Spec 权威另案；
3. 按 `ADOPTION.md` 只记录 adoption 身份；
4. 跑两个变更，记录指标；
5. 有改善才保留；无改善 revert 整个 v3.0。

## 不做什么

- 不迁移 0001/0002；
- 不设 bugfix 特殊路由；
- 不强制 mutation/state model；
- 不新增角色/状态；
- 不把非规范 checklist/guide 当验收。
