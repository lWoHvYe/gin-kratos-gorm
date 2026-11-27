# 核心治理文件复核（2026-11-27）

| 项 | 值 |
|---|---|
| 范围 | `.specify/memory/constitution.md`（v2.2.0）、根 `AGENTS.md`、已归档 `docs/archive/2026-11-27-governance/CONVENTIONS.md`（现行 `docs/collab/CONVENTIONS.md` 已是 stub） |
| 触发 | 人类指示：系统复核；Spec Kit 生成的 Spec 应允许其原生语法，不再强制 EARS |
| base | `9e81497`；head = 当前工作树（未提交） |
| 审查 | `reviewer-syntax` 首轮 **FAIL**（1 阻断）→ 作者修复 → **re-review PASS**（SCOPED，`NOT RELEASE`） |
| 报告 | [REV-GOV-SPEC-SYNTAX-01.md](../../../collab/qc-reports/REV-GOV-SPEC-SYNTAX-01.md) |
| 状态 | 宪法 v2.2.0（MINOR）；L2/L3 已在 v2.2.0 恢复；**人类已批准 v2.2.0（2026-11-27）；未 commit/未 merge** |

---

## 1. 结论

1. **EARS 不再是强制语法**。附录 A 行 1 允许 EARS 或 Spec Kit 原生语法（`Given/When/Then` + `FR` + `SC`）；新增“Spec Kit 兼容”注：Spec Kit 的 Acceptance Scenarios 必须逐条映射到验收表 AC，不能只引用 `FR`/`SC`；可判定性、异常分支、错误码、证据要求不变。
2. **Spec 身份与 change_id 已分开**。首轮审查的阻断项是：原第 30 条按文件名主干推 change_id，而 SPEC-0002 已登记 `PAY-AC50-REVIEW-01`，两者冲突。现改为：Spec 身份 = 文件/目录；一个 Spec 可多次变更；每次变更一个 change_id；长期 Spec 的 change_id 在台账登记，历史不回溯补登；AC 编号不要求等于 change_id。
3. **直接相关的一致性冲突已修**：tasks `[X]` 只是执行进度；`speckit-implement` 必须分批且工具无阶段参数；模板覆盖层 replace vs append；台账同日按插入位置；合入尾注由人类补；evidence 路径分流；实现变更也写一页以内 plan；治理变更加入级别表。
4. **L2/L3 已在 v2.2.0 恢复**（`GOV-CORE-RESTORE-01`，2026-11-27 第二批；见 §7）：
   - **L2**：旧 CONVENTIONS §2.2 的其他输入规则（审查者之外）、§3 integration owner、§4.1 reviewer 元数据（`review_scope`/`release_status`/`INV`/`baseline_impact`）。
   - **L3**：v1.3.0 压缩丢失的 permit/commitment/Σ、事务核心 `database/sql`+CAS、测试替身/启动 fail-fast/请求路径不得 panic、`order.paid` 兼容期收口细节。

---

## 2. 本轮已调整（宪法 v2.1.0 + AGENTS）

| # | 位置 | 调整 | 理由 |
|---|---|---|---|
| 1 | 宪法附录 A 行 1 | EARS **或** Spec Kit 原生语法均可 | 人类指示；Spec Kit `specify` 原生产出是 Gherkin/FR/SC |
| 2 | 宪法附录 A 注 | 新增“Spec Kit 兼容”：Acceptance Scenarios 逐条映射到 AC；默认布局目录名可作 change_id；迁入扁平布局保留已登记 id | 防止工具语法成为逃逸口，同时不强制改写 |
| 3 | 宪法 1 | “七个必备章节”改为“附录 A 的必备问题”；章节标题可按 Spec Kit 模板组织 | 避免固定章节数与工具模板冲突 |
| 4 | 宪法 30 | Spec 身份与 change_id 分开；长期 Spec 登记 change_id；历史不回溯补登；同一时间一个 change_id | 修复 B1；与 SPEC-0002 的 `PAY-AC50-REVIEW-01` / `PAY-SETTLE-01` 对齐 |
| 5 | 宪法附录 A 行 86 | 新 Spec 可用 `<change_id>-AC-<n>`；长期 Spec 沿用原 `AC-<n>`；AC 编号不要求等于 change_id | 修复 AC 编号语义冲突 |
| 6 | 宪法 27 | tasks 的 `[X]` 只是执行进度，不保存验收状态 | 解决 `speckit-implement` 标 `[X]` 与“tasks 不保存状态”的冲突 |
| 7 | 宪法附录 A | 新增“延期清单”行：`AC/工作项、owner、expiry、reason`；到期阻断；不得批量续期；延期≠豁免 | 补审查者 NB-1；32 条 DEFERRED 有合法落点 |
| 8 | 宪法 3 | 新增“治理变更”类型（按 Governance，不走 Spec/ADR） | 修复“治理变更不是合法级别”的 NB-14 |
| 9 | AGENTS §1 | 审查者改为有运行时的外部 subagent；静态回退须写原因/未覆盖/人类书面豁免 | 对齐人类裁决 1-A；补 NB-4 |
| 10 | AGENTS §2 | 用 Spec Kit 时可用原生章节 + “附录 A 对照”；实现变更写第 4-8 步、plan 一页以内；`speckit-implement` 分批且无阶段参数 | 对齐 Spec Kit；解决 L1 矛盾 |
| 11 | AGENTS §3 | 级别加“治理变更”；静态审查条件对齐 §1；AC 位移点名 Spec | 修复 NB-14/R-1/R-3 |
| 12 | AGENTS §4 | 同日条目按插入位置判定；尾注写在合入提交；AC 位移带 Spec | 修复 NB-7/NB-8/R-1 |
| 13 | AGENTS §5 | 新 Spec 用 `evidence/`（扁平用 `docs/specs/evidence/<change_id>/`）；在途用 `docs/implementation/**`；治理变更报告用 `docs/collab/qc-reports/` | 修复 NB-9/R-2 |
| 14 | AGENTS §6 | checklist 默认作者自检；作为审查产物时 `[x]` 是审查者判定；模板覆盖层 replace vs append | 修复 NB-5；纠正 C-2 的实现方式 |

---

## 3. 审查者首轮 FAIL 的阻断项（已修）

**B1｜change_id 与已登记事实冲突**：

- 现象：原第 30 条“扁平文件取文件名主干”会让 SPEC-0002 同时拥有 `0002-payment-settlement-admission` 与 `PAY-AC50-REVIEW-01` 两个 id。
- 根因：把“Spec 身份”和“变更实例身份”合并；本仓扁平 Spec 是“一份业务能力一个文件、approved 后原地升版”，一个 Spec 一生有多个 change_id（`docs/specs/README.md:3,70`）。
- 修复：宪法 30 拆成两层；SPEC-0002 保留 `PAY-AC50-REVIEW-01`（父 `PAY-SETTLE-01`）；SPEC-0001 历史不补登；AC 编号独立。
- 复核：re-review 三项特别核对全部 PASS。

---

## 4. v2.1.0 时的遗留（v2.2.0 已处理，保留为历史）

> 以下为 v2.1.0 审查时列出的缺口；v2.2.0 已恢复，见 §7。

### 4.1 治理增项（L2）

| # | 旧规则 | 建议补到哪里 | 理由 |
|---|---|---|---|
| G-1 | CONVENTIONS §2.2 的其他输入规则（审查者之外）只能是问题提案，必须引用当前 HEAD 文件:行号，不能直接成为契约/AC/阻断依据 | 宪法 12 加一句 | 本次 ROLLOUT 本身就是审查者之外的分析产物；没有这条，外部结论可直接变契约 |
| G-2 | CONVENTIONS §3 跨服务变更必须有 integration owner，负责基线与组合验证 | AGENTS §2 或 §5 | 新模型只要求“列出受影响 AC/非回归证据”，没有责任人 |
| G-3 | CONVENTIONS §4.1 Reviewer 元数据 `review_scope`/`release_status`/`INV`/`baseline_impact` | AGENTS §3 任务书输出加第 5 项 | 资金/库存/权限/跨服务裁决需要 INV 与 release_status；当前任务书只要求“写明范围” |

### 4.2 高风险领域条款（L3）

| # | 旧宪法条款 | 建议 | 风险 |
|---|---|---|---|
| D-1 | II.5 permit/commitment/Σ：不得自行恢复 `REVOKED` permit；commitment 创建/释放须版本化 SPEC/ADR + 人类批准；TCC 补偿/SYSTEM 退款按活动组金额之和等于订单剩余可退金额逐组准入 | 宪法 23 补回 2-3 句 | **不可接受级别**：当前 23 只剩“同一组幂等准入 + permit 裁决竞态” |
| D-2 | III.2 事务核心用显式 `database/sql` + 条件 `UPDATE`（CAS）；GORM 仅读侧 | 宪法 26 补回 | 部分可接受（GORM 限制 + 禁新 ORM 兜底），但 CAS 语义不应只靠 20 条“幂等” |
| D-3 | I.2 测试替身只允许 `_test.go`/build tag；必需依赖构造期必填、启动期 fail-fast；可选能力用显式 Options，禁止 nil 表示关闭；III.4 请求路径不得 panic，启动失败 fail-fast 非零退出 | 宪法 15/26 补回 | **不可接受级别**：当前 15 只覆盖资金/库存运行时异常 |
| D-4 | II.1 `order.paid` 兼容期：双发仅作 BFF/旧消费者兼容，不得作为准入/结算/库存就绪依据；收口条件/移除日期/回滚路径必须写明 | 宪法 6 或 23 补回 | 部分丢失；当前项目仍在兼容期语义内 |

### 4.3 小项

- `docs/collab/ac-baseline.txt` 仍未归档（C-4 清单未列）；新模型无 AC 基线概念，建议归档 + stub。
- `docs/convert/` 仍未跟踪且与已落地治理文本重复；建议归档或删除。
- C-2（Spec 模板覆盖层）未做；新语法允许后不再强制 EARS，但模板仍缺附录 A 八项，建议用预设/扩展层 append 一个“附录 A 对照”段落。
- `speckit-implement` 在本仓仍不可直接运行（无 `specs/`、无 `.specify/feature.json`）；属 `SDD-STRUCT-ALIGN-01`。
- 治理变更的“最小增量/可验证收益/停止条件”已写入台账 `GOV-SPEC-SYNTAX-01` 条目；连续两个变更后复查。

---

## 5. 建议的第二批修订（待人类决定）

如果要做，建议拆成两个独立治理变更，避免再次把“语法放宽”和“领域条款恢复”混在一起：

1. **`GOV-REVIEW-META-01`**：G-1/G-2/G-3（审查者之外的其他输入、integration owner、reviewer 元数据）。
2. **`GOV-INV-RESTORE-01`**：D-1…D-4（permit/commitment/Σ、database/sql+CAS、测试替身/fail-fast、order.paid 兼容期）。

两者都按 Governance 走：作者起草 → 独立审查 → 人类批准；不新增 Spec/ADR。

---

## 6. 当前状态

- 宪法 **v2.2.0**、AGENTS 已更新；L2/L3 第二批经审查者 SCOPED-PASS → NB-11 FAIL → 两层结构恢复 **final PASS**；`NOT RELEASE`。
- **人类已批准 v2.2.0（2026-11-27）**；当前未 commit/未 merge。
- 合入时仍需包含 `docs/archive/**`、`docs/review-tracking-archive.md`、`REV-GOV-CONVERT-01-C3.md`、`REV-GOV-SPEC-SYNTAX-01.md`、`docs/convert/**` 等未跟踪文件。

## 7. v2.2.0 第二批：L2 + L3 恢复（`GOV-CORE-RESTORE-01`）

| 项 | 恢复内容 |
|---|---|
| L2 审查者之外的其他输入 | 宪法 12：审查者（独立 subagent）的裁决是唯一审查结论；其他输入只作问题提案，必须引用当前 HEAD 文件:行号；找不到标 `[不确定]`、不得作需求引用；已裁决按文档漂移；不削弱第 9 条的安全/权限/数据完整性/迁移阻断 |
| L2 integration owner | AGENTS §2：跨服务变更写 plan 时指定 integration owner（既有角色兼任，不新增角色），负责基线与组合验证 |
| L2 reviewer 元数据 | AGENTS §3：`review_scope`、`release_status`、`spec_version`/`adr_revision`、`verification`、INV、`baseline_impact`；HEAD 变化重新确认；plan 写 `baseline_impact` 与 `stop_conditions` |
| L3 启动/依赖 | 宪法 15：订单/开发默认值、必需依赖构造期必填/启动 fail-fast、可选能力显式 Options、测试替身仅 `_test.go`/build tag、启动失败非零退出 |
| L3 退款/准入 | 宪法 23：II.5 两条 IF/THEN（未完成退款撤 permit；已离仓只退款不回滚）、`REVOKED` permit 不得自行恢复、commitment 规则、Σ 等式、`order.paid` 兼容期两层（兼容期四项+禁 flag；灰度影子期 ADR 切换/owner/收口+生产默认 `OrderSettled`） |
| L3 DB/错误 | 宪法 26：`database/sql`+条件 `UPDATE`（CAS）、GORM 仅 `catalog/gorm_repo.go`/`cart/gorm_repo.go` 读侧、`apperr`/`fmt.Errorf("%w")`/`docs/contracts.md` 错误映射 |

**审查结果**：SCOPED-PASS（首次）→ NB-11 逐字化 FAIL（1 阻断：把四项兼容要求错误限定到“灰度影子期”）→ 恢复两层结构 → **final PASS**。报告：[REV-GOV-CORE-RESTORE-01.md](../../../collab/qc-reports/REV-GOV-CORE-RESTORE-01.md)。

**仍非阻断**：宪法 23 仍是一整段（可读性，不是错误）；“ADR 批准的版本化兼容期”未逐字出现，可由附录 B + 影子期 ADR 要求推出。

> 人类更正（2026-11-27）：不存在“外部复审”；reviewer 就是独立 subagent；其他输入才作问题提案。
