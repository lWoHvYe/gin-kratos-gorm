# Review & Verification Protocol（非规范试点）

> 性质：非规范操作协议，冲突以宪法/AGENTS 为准。用于一个非 T5、非 legacy 迁移的变更试点；
> 不新增角色/批准点，不替代 Spec/ADR/台账。
> 本协议是 review 试点，不用于 v3 pilot；§5 的风险分级仅为试点辅助，不替代宪法 12/AGENTS。

## 0. 过渡边界

- 所有 active Spec 在 `specs/<id>/spec.md`；`specs/README.md` 是唯一 index。
- `0001` historical reference（复审状态见台账）；`0002` active；`sdd` historical；`docs/specs/` 只重定向。
- 本协议只解决“review 轮次/证据/成本”。

## 1. 目标

在不降低 blocker 检出的前提下，把 review 从“多轮 prose 往返”变成“一次证据核对 + 有界升级”：

- 首次 review 前，作者交完整证据；
- reviewer 一次给全 blocking 问题；
- 作者一次修完 blocker；
- 不在 review 里做设计；设计问题回 Spec/ADR；
- 不设“2 轮就放行”的硬上限；超过 2 轮升级分类，而不是继续 prose。

## 2. 输入（缺一退回）

- 权威 Spec/ADR 与本次 AC；
- plan：`baseline_impact`、`stop_conditions`、AC→检查映射；
- diff（精确 base/head）；
- 测试日志（绑定 HEAD）；
- `【测试范围】【未覆盖范围】【取舍】`；
- 高风险：按宪法 12 的并发/故障证据；标“已验证”的 claim 必须有独立 oracle（宪法 11/13）；matrix/mutation 是可选辅助。

reviewer 不读作者自述当证据；不要求作者提供 2,000 行 markdown。

## 3. 审查顺序

1. **范围/分类**：契约变更/实现变更；是否越界；是否命中附录 B。
2. **Spec/ADR 一致性**：附录 A 全部问题；AC 可判定；5 窗口/恢复所有者；状态所有权。
3. **证据**：测试是否独立 oracle；是否只复述实现；高风险是否有 negative control/mutation/race/fault。
4. **diff**：只读变更段 + 受影响文件；200–400 行以上考虑拆分。
5. **状态**：AC 位移/台账由 reviewer 写；作者不得自标。

## 4. 一次给全 + 有界升级

- reviewer 同一轮列出所有 blocking 问题（可附 MAJOR/MINOR 备注），每条指向 AC/INV/ADR/必备章节/宪法条款；非阻断记 `[TODO]`。
- 作者用“问题→修复→证据”矩阵一次修完；reviewer 只复跑失败项+diff，不重读全文。
- 不设“2 轮自动 PASS/FAIL”。超过 2 轮仍不收敛时：
  - 分类为：实现错误 / 证据不足 / 设计问题 / 契约缺口 / 环境不可用；
  - 实现错误→继续修；证据不足→回补；设计/契约→回 Spec/ADR/冻结；环境→人类豁免；
  - 不用轮次上限覆盖权限/安全/数据完整性/迁移/资金/库存的阻断。
- reviewer 不给补丁、不替作者设计；设计担忧能指向 INV/ADR/AC 就阻断，不能指向的记 `[TODO]`。

## 5. 风险分级

- **High**（资金、库存、履约、权限、跨服务、迁移、状态机）：按宪法 12 的并发/故障证据；标“已验证”必须有独立 oracle（宪法 11/13）；mutation/state model 可选辅助；reviewer 跑证据；human merge。
- **Medium**：测试/契约检查 + reviewer 摘要；human 可抽样。
- **Low**：lint/tests + reviewer 摘要；human merge。
- 风险由路径/事件/迁移触发，不由作者自选；作者只能上调。

## 6. 成本指标（试点）

| 指标 | 目标/记录 |
|---|---|
| review 轮次 | 记录；>2 触发分类，不是自动放行 |
| 首次 review 的 blocker 数 | 记录 |
| 作者一次修完率 | 记录 |
| escaped blocker（到 human/生产才发现） | 0 |
| review 时间 / 实现+测试时间 | 记录；>1 触发检查 |
| artifact 行数 / 代码改动行数 | 记录；>1 触发检查 |
| human 分钟 | 只决策/merge/release |

不把 commit 数、报告数、测试数当进度。

## 7. 试点

- 选一个非 T5、非 legacy 迁移的契约/实现变更；作者按本协议交证据；reviewer 按本协议审。
- 记录 §6；两个变更后与旧流程对比。
- 若 review 轮次/时间没有下降，停止本协议，不写进 AGENTS/宪法。
- 若有效，再把最小规则写进 AGENTS/宪法（单独治理变更），不在试点中暗改。

## 8. 不做什么

- 不恢复六角色/QC/多 Agent 辩论。
- 不新增人类批准点；human 仍只批准 Spec/合入/Release。
- 不强制新工具；mutation/state model 只在高风险且可执行时使用。
- 不用“2 轮上限”放行未解决的风险。
- 不把 0001/0002 迁移混进本试点。
