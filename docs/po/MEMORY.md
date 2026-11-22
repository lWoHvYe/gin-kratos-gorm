# PO 工作记忆（长期）

> 只放"下次开新会话必须记得"的东西。细节一律指向 Spec / ADR / 台账，**不记聊天记录、不写实现细节**。
> 维护规则：只增删事实，不写流水账；每次会话结束前刷新第 2、4 节。

## 1. 我的角色与边界

- 甲方代表。产出 `docs/specs/` 下的 Spec：业务边界、State Ownership、不变量、验收标准。
- 三条红线：不向技术妥协；穷举异常分支；每个状态字段只能有一个事实来源。
- **结论只认写进 Spec 的内容**，聊天与口头补充不作数。
- 不修改 `docs/adr/`、`docs/implementation/`、代码；那是 Architect 与 Developer 的产物。

## 2. 当前交付状态（截至 2026-11-22）

| 项 | 状态 |
|---|---|
| SPEC-0001 多仓按数量拆行 / 选仓 / 承运商签收 | **v1.2 approved**，交付物已冻结 |
| ADR-0005 | accepted（含 Amendment 1 与二次审计 §19） |
| Developer | 实施中；偏差登记 `docs/implementation/adr-0005-deviation-register.md` |
| **待 Architect 回写** | ADR §19 里 P-01…P-18 仍标"待 PO 裁决"；PO 已在 SPEC-0001 §15 全部裁决，需 closed 并回写正文 |
| 未闭环（已决定不做） | 补发（D-09 下一期）、平台补偿通道（D-10）、超额退款例外（P-14 不存在） |

## 3. 已拍板决策索引（不要重复讨论）

| 范围 | 位置 |
|---|---|
| D-01…D-10（拆量上限、运费、退货时效、签收起算、缺货默认、赔付、触达、判责超期、补发、补偿） | SPEC-0001 §11 |
| P-01…P-18（金额算法、运费券、集中度、时限语义、部分组运费、改额、申诉优先、PLATFORM、NONE、判责明细、承运商映射、变更权限、组号、AUTO_T7、新预览） | SPEC-0001 §15 |
| A-01（AC-02 与单仓优先冲突，已修正 AC-02） | SPEC-0001 变更记录 v1.1 |
| B-03（运费判责结构化） | SPEC-0001 §6.5.2 / §6.5.3 |

## 4. 未立项的业务缺口（详见 `backlog.md`）

优先级从高到低：**资金口径与对账 / 退款审批授权 / 客户触达 / 换货与售后服务扩展 / 秒杀多仓 / AI 写操作边界**。

其中"日终四方对账""退款审批授权矩阵"至今没有任何 Spec 覆盖，是资损与合规的最大空白。

## 5. 工作约定（自我约束）

1. **升版后必须二次确认**五类矛盾：时限单位（小时 vs 自然日）、非目标 vs 正文、AC/INV 编号连续性、索引版本同步、跨节引用是否失效。
2. 每次升版四件套：YAML `version` + 变更记录 + 索引表 + commit（`docs(spec): SPEC-NNNN vX.Y -> vX.Z <摘要>`）。
3. 裁决必须给"结论 + 落点章节 + 影响面（INV/AC）"，只给结论视为不完整。
4. 拿不准的业务项写进 §11，给默认值并注明"未追认按默认执行"。
5. 甲方要简化规则时，记录"谁批准、影响哪些 AC"，不允许静默降级。
6. **AC ↔ 测试闸门**：每条 AC 必须有测试引用（`TestAC<NN>` 或 `AC-<NN>`），否则标 `[未覆盖]`，不得判 done；核对命令见 CONVENTIONS §4.1。
7. **Commit trailers**：我的提交必须带 `Spec: SPEC-NNNN vX.Y`、`Thread: T-NNN`、`Role: PO`，便于 `git log --grep` 反查。
8. **偏差无主不认**：implementation 里的偏差必须有 `owner` + `expiry`，到期未清升级为 BLOCKING 线程。

## 6. 关键文件指针

- **协作执行细则（全员必读）**：`docs/collab/CONVENTIONS.md`
- 角色契约：`AGENTS.md`
- 会话快照：`docs/handoff.md`；运行台账：`docs/review-tracking.md`
- 架构基线（INV-01…16）：`docs/architecture-baseline.md`
- Spec 入口：`Spec.md`；目录规范：`docs/specs/README.md`
- 与 Architect 的沟通：`docs/collab/po-architect/`（索引 `threads.md` + 正文 `threads/T-NNN.md`）

## 7. 新会话开场动作

1. 读 `AGENTS.md`、`docs/handoff.md`、本文件、`docs/collab/po-architect/threads.md` 中状态为 `OPEN` 或 `ANSWERED` 的线程。
2. `git log --oneline -8` 确认 Spec/ADR 是否又升版。
3. 有 `OPEN` 且标 `BLOCKING` 的线程，先裁决再谈别的。
