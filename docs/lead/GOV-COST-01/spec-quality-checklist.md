# SPEC-0003 规格质量检查单（Lead 证据视图）

| 项 | 值 |
|---|---|
| change_id | `GOV-COST-01` |
| 被检对象 | `docs/specs/0003-platform-governance-cost-reduction.md`（SPEC-0003 v1.0，in-review，460 行） |
| 审查时 HEAD | `6716929`（`docs(review): record T1.2-R PASS and G1 follow-up`） |
| 治理基线 | 宪法 v1.2.0（EFFECTIVE，109 行）；`AGENTS.md`（137 行）；`docs/collab/CONVENTIONS.md`（255 行） |
| 检查者 | Lead（只做客观合规检查，不代替 PO 写 Spec、不代替 QC 给语义裁决） |
| 结论 | 机械项全部 PASS；语义缺口候选已移交 QC，见本文件末节 |

> 本检查单是 **Lead 的证据索引**，不是契约、不是 QC 报告、不构成任何 AC 验收。仓库规范要求 PO Spec 位于 `docs/specs/NNNN-<domain>-<capability>.md`（`docs/specs/README.md:9-30`），故 Spec Kit 默认的 `specs/` 目录与 `checklists/` 位置在此**不适用**；本检查单放在 Lead 工作区，Spec 契约仍以单文件为唯一来源。

## 1. 内容质量

- [x] 无实现细节泄漏作为**要求**：§1.2 明确排除脚本算法/代码结构/CI YAML 细节；把门禁执行器、台账、CI 工作流作为可观察边界提及（合理）
- [x] 聚焦治理价值与风险：分级降本、门禁机器化、错分停线
- [x] 面向非实现读者：决策摘要把 L0/L1/L2、联签、delta 复审讲清
- [x] 必填章节完整：YAML 元数据 + §0–§14，与 SPEC-0002 结构对齐

## 2. 需求完备性

- [x] 无 `[NEEDS CLARIFICATION]` 标记；开放项收敛为 D-CR-6/D-CR-7（2 个，含选项与推荐）
- [x] 需求可测：AC-69…AC-96 各含 WHEN/IF/WHILE + `THE SYSTEM SHALL`（85 条 EARS 语句，28 个 AC 全部覆盖）
- [x] 成功标准可验证：以门禁输出、联签记录、台账、范围绑定为证据，未把提案“估算”当阈值（§1.3 明确）
- [x] 验收场景定义：治理 AC 以文档/命令输出/门禁记录验收，§9 规定 `test_type`/`evidence_path`
- [x] 边界情况：§7 E32…E43 异常矩阵；§5 状态机非法迁移
- [x] 范围有界：§1.2 non-goals 含提案 §5 anti-scope 与“不得用本 Spec 绕过门禁”
- [x] 依赖与假设：§1.3 现状复核 6 条漂移；§11 落点依赖；§12 顺序依赖

## 3. 可交付性

- [x] 每个 AC 有验收方与证据形态（§9 + §8 失败可见性）
- [x] 主流程覆盖：分级→复核→联签/停线→门禁→QC→Release
- [x] 与既有基线隔离：AC-01…AC-68、INV-01…INV-50 不变（AC-96）
- [x] 编号唯一：AC-69…96 连续；INV-51…66 连续；未复用既有编号
- [x] 引用可解析：47 处 `文件:行号` 全部在对应文件行数范围内
- [x] EARS 合规：无行首 Given/When/Then；无“应当/应该”模糊语

## 4. 已识别的待 QC 语义问题（Lead 假设，未裁决）

1. **治理变更自身如何定级**：AC-75/77 的 L1 判据写“单服务内行为变更”，`AGENTS.md`/`CONVENTIONS.md`/CI/`scripts/collab_gates.py` 的治理变更既非业务文档修正（L0），也非单服务行为（L1），可能落入分级空隙。
2. **L1 “无人类逐门批准” 与宪法 v1.2.0 Workflow 4/8 的边界**：AC-82/83 写 L1 免人类逐门批准；需确认未覆盖 FAIL、Plan、merge 等人类检查点。
3. **D-CR-6 未关闭**：根 `AGENTS.md` 不在 AC-73 的三项触发路径内，治理文件仍可绕过 CI。
4. **D-CR-7 未关闭**：AC-82 已按保守口径（delta + 受影响条款）编码，但未与 D-CR-3 的“主版本全量”显式交叉引用。
5. **GV-S1 落点未定义**：`change_tier` 写入“变更输入包”，其持久位置/格式未在 Spec 中定义（可能属 Architect/Plan 范畴，需 QC 判定是否应写入 Spec）。

## 5. QC 结果（4 轮，全部报告在 `docs/collab/qc-reports/`）

| 轮次 | 结论 | 计数 | 关键缺口 |
|---|---|---|---|
| QC-01 | BLOCKED | 0/10/11 | 治理/工具链无级别（Q-01）、L1 委托边界不闭合（Q-02）、治理 AC 无法通过 G-3（Q-03）、门禁无自保护（Q-04）、单写者缺口（Q-05）、D-CR 批准不可核验（Q-06） |
| QC-02 | BLOCKED | 0/3/8 | L1-G 越权新增分级（N-01）、AC-97 非法域（N-02）、G-3 引导期矛盾（N-03） |
| QC-03 | BLOCKED | 0/2/4 | tier 自升级使 L0/L1 不可执行（N-04）、首次部署自阻断（N-05） |
| QC-04 | **READY FOR HUMAN SPEC APPROVAL** | 0/0/4 | 仅 M-1…M-4 非阻断字段/复现证据澄清 |

- 复核对象：v1.3，688 行，AC-69…112（44 个，连续），INV-51…81（31 个），sha1 `15ad2979…`，HEAD `f64434a`。
- QC-04 明确：`READY` 不构成 AC 验收/实现 PASS/Release PASS（`docs/collab/qc-reports/SPEC-0003-QC-04.md:75`）。
- 待批准时应用的 4 项 MINOR：M-1 bootstrap expiry/scope 字段落点；M-2 AC-107 字段清单补 `designated_by/independence_evidence`；M-3 scoped legacy 运行证据（评估 AC 集/命令）；M-4 “更新记录行”边界示例（append-only status 行 vs 改写既有行）。

## 6. 未覆盖范围

- 未做语义红队审查（QC 职责）；未运行任何门禁脚本；未在 GitHub runner 验证 CI 路径行为（静态推断，spec §1.3 已标 `[不确定]`）。
- 未登记 `docs/specs/README.md` 索引、`ac-baseline.txt`/`ac-coverage.tsv`（PO 任务边界外，需批准后由 PO/Reviewer 执行）。
