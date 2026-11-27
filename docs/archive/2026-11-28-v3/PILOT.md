# v3.0 pilot

Status: process trial; Spec migration already applied.

## 目标

在两个非 T5 的真实变更上验证；先记录 v2.2.0 下可比变更的 review 轮次/lead time 作为基线：

- review 轮次 ≤2（超过由人类分类，不自动放行）；
- lead time 下降；
- escaped blocker = 0；
- 文档改动行数 ÷ 代码改动行数 ≤1（纯文档 N/A）；
- human 分钟只花在决策/合入/Release。

## 采用

1. v3 宪法/AGENTS 已应用；
2. Spec 单布局/0001/0002 迁移已应用；
3. 跑两个变更，记录指标；
4. 有改善才保留；无改善 revert v3 commit（含迁移）。

## 不做什么

- 不迁移 T5/不碰 PAY-SETTLE 代码；
- 不设 bugfix 特殊路由；
- 不强制 mutation/state model；
- 不新增角色/状态。
