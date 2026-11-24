# GOV-COST-01 Plan — 治理成本收敛实施计划

| 项 | 值 |
|---|---|
| change_id | `GOV-COST-01` |
| Status | **FROZEN at P0a — Director 2026-11-25**（门禁大幅放宽，不继续复杂化）。P0a DONE（`0d57398`/`3cac054`，gates 绿）；**P0b/P1/P2 全部 DEFERRED**；未提交的 P0b/CI 改动已回退；Amendment 1 不执行 |
| Drafted | 2026-11-24 |
| Plan owner | Lead |
| Integration owner | System Architect（治理/工具链落点）|
| Spec | SPEC-0003 **v1.4 approved**（2026-11-24；AC-69…AC-112，INV-51…INV-81；`docs/specs/0003-platform-governance-cost-reduction.md`）|
| ADR | ADR-0008 **Rev 1.1 approved**（2026-11-24；`docs/adr/0008-governance-cost-reduction.md`；§18.1 R0…R6 执行顺序）|
| Base | 起草时 HEAD `c8fe518`；P0a 基线另行绑定**显式** `BASE`（见 §4）|
| Allowed paths | `docs/specs/0003*`、`docs/specs/README.md`、`docs/adr/0008*`、`docs/collab/{qc-reports,lead,ac-*}`、`docs/lead/GOV-COST-01/**`、`scripts/collab_gates.py`、`scripts/check-collab-gates.sh`、`scripts/test-collab-gates.sh`、`Makefile`、`.github/workflows/**`、`docs/review-tracking.md` |
| 禁止路径 | 业务服务代码/迁移/proto（除非另开 L2 change）；`internal/platform` 以外的新跨服务协议；任何 G-5/G-6 新门禁类型 |
| Baseline impact | 业务 AC-01…68 / INV-01…50 **零变更**；治理门禁、CI、台账、模板、脚本受本 Plan 影响 |
| 人类批准点 | Plan（本文件）→ P0a bootstrap 证据 → Release（任何级别）|

> 本 Plan 只编排**已批准契约**（SPEC-0003 v1.4 + ADR-0008 Rev 1.1）的实现，不新增业务规则。契约与实现冲突时停止并由 owner 裁决；不得以“代码现状”静默废止契（AGENTS §7、CONVENTIONS §2.1）。

---

## 1. 结论与范围

本变更把治理流程的边际成本从“每变更人肉重复支付”转为“一次性机器化 + 分级”。范围严格对应 SPEC-0003 的五个改造项：

| 改造项 | 契约 | 执行阶段 |
|---|---|---|
| GOV-CODE-01 门禁机器化补全 | AC-69…74、99、100、102、107 | P0a + P0b |
| GOV-TIER-01 三级分级与错分停线 | AC-75…81、97、98、103、108、109、111 | P1 |
| GOV-HUM-01 L1 联签与人类批准域 | AC-82…85、101 | P1 |
| GOV-QC-01 QC 差分审查 | AC-86…89、105、110 | P2 |
| GOV-SCAF-01 治理产物骨架 | AC-90…92 | P2 |

**Non-goals**（Spec §1.2、Proposal §5）：不取消 subagent 自读、不动契约先行、不合并台账、不新增门禁类型、不批量延期治理 AC、不以本变更绕过分级或人类批准。

---

## 2. 已裁决事项（输入）

| ID | 裁决 | 证据 |
|---|---|---|
| D-CR-1…5 | 全采纳（L0/L1/L2、QC+Reviewer 联签、QC delta、错分停线、CI 三路径） | `docs/review-tracking.md` §D-CR；SPEC-0003 §10 |
| QC-05 Q-02 | bootstrap 证据 = `BASE` 树 G-1…G-3 PASS + `G-4 NOT-RUN(not-implemented-at-BASE)` | 同上 §GOV-COST-01 裁决；SPEC v1.4 AC-106/112 |
| IV | Director 持有或逐次书面指定（GV-S21 `designated_by`） | 同上 |
| 提交时机 | 批准稿 untracked 至 **P0a 原子提交**（spec + G-3 扩展 + 台账播种） | 同上；Spec §12/§14 |

**未关闭（不阻断本 Plan，按保守默认）**：D-CR-6（根 `AGENTS.md` CI 覆盖）、D-CR-7（L1 联签 QC 范围）。

### 2.1 P0a/P0b bootstrap 授权（GV-S20 `mode=bootstrap`）

| 字段 | 值 |
|---|---|
| approver | Director |
| approved_at | 2026-11-24 |
| expiry | 2026-12-31 |
| scope | P0a/P0b（SPEC-0003 v1.4 AC-106/AC-112） |
| change_id | `GOV-COST-01` |
| 替代控制 | IV 独立复核 + 人类书面批准；`G-4 NOT-RUN(not-implemented-at-BASE)` 永不计 PASS、不授权 Release/AC-VERIFIED；P0b 落地 G-4 后失效 |
| 关联 | 人类裁决记录 `docs/review-tracking.md` §"2026-11-24 human decisions: GOV-COST-01 bootstrap, IV and commit timing"；Plan 批准（本文件 Status） |

---

## 3. 执行顺序总览（不可颠倒）

```text
Phase 0  契约批准（已完成）
  SPEC-0003 v1.4 approved · ADR-0008 Rev 1.1 approved
     ↓
P0a      G-3 治理证据扩展 + 原子落库（Spec/G-3/台账同提交）
     ↓
P0b      G-4 · Release 模式 · 显式 range · 自保护/IV · CI 路径 · G-2 摘要
     ↓
P1       Tier 注册表与总函数 · L1 联签 · GV-S9/S15/S17 载体 · 审计
     ↓
P2       QC delta/full 身份 · Release object set · Scaffold 生成器
     ↓
Release  人类放行（任何级别）
```

依赖理由（Spec §12、ADR §18.1）：P1/P2 的兜底都依赖机器门禁；QC delta 的收口依赖 Release 全量复审与 G-4；先放宽 QC 再机器化属颠倒顺序，禁止。

---

## 4. P0a — G-3 治理证据扩展与原子落库

**目标**：让治理 AC 具备可机器核验的证据路径，并让 SPEC-0003 首次进入仓库时不制造既成红门禁。

**前置（ADR §7.1 ⓪）**：
- [ ] bootstrap 人类批准行已存在（GV-S20 `mode=bootstrap`，approver/date/expiry/scope）；
- [ ] `BASE` 选定为**首个 P0a 提交的父提交**，该树**不含** SPEC-0003 的 AC-69…112；
- [ ] `BASE^..BASE` 满足 G-1 trailer（`change_id`/`Actor`/`Reviewed-by`）；
- [ ] IV 已指定（Director 或书面 designee）并记录 `independence_evidence`。

**任务**

| ID | 任务 | Owner | 依赖 | 证据 |
|---|---|---|---|---|
| T-A1 | G-3 扩展：读取 `ac-coverage.tsv` 的 `test_type/evidence_path/status`；治理 AC 需 `VERIFIED`+签名+摘要，否则 `GOV_AC_EVIDENCE_MISSING`；业务 AC 保持旧语义 | Developer | Plan 批准 | 单测 + 脚本运行日志 |
| T-A2 | G-3 读取改为 **head 绑定**（`git show`/`git ls-tree`），禁止工作区与未跟踪测试引用；`--range` 透传 | Developer | T-A1 | 单测 + 复现命令 |
| T-A3 | `BASE` 投影运行：干净 worktree 内执行 legacy G-1…G-4（G-4 不存在时记 `g4_status=NOT-RUN(not-implemented-at-BASE)`），显式 `--range=<BASE^>..<BASE>`，scoped AC-01…68；IV 记录 GV-S21 | Developer + IV | T-A2 | GV-S21 行 + 命令输出 |
| T-A4 | 台账播种：`ac-baseline.txt` 增至 AC-112；`ac-coverage.tsv` 为 AC-69…112 建 `PENDING` 行（**非 DEFERRED**；含 owner/thread 与 `test_type/evidence_path` 占位） | Reviewer | T-A1 | 台账 diff + G-3 运行 |
| T-A5 | 原子提交：approved SPEC-0003 + G-3 代码 + 台账 + 测试 + 证据，四位一体；带 `change_id/Actor/Reviewed-by` | Lead + Developer | T-A1…T-A4 | commit + `make gates-strict` |

**退出条件**：P0a 提交后 `make gates-strict` 绿；治理 AC 呈 `PENDING`/`NOT-RUN`（不冒充 VERIFIED），Release 仍被阻断；`BASE` 证据可复现。

**回滚**：revert 原子提交；台账回退；不得保留半套 G-3 扩展。

---

## 5. P0b — 四闸门、Release 模式与自保护

| ID | 任务 | Owner | 依赖 | 证据 |
|---|---|---|---|---|
| T-B1 | 实现 G-4（所有权/范围 + 到期 + 豁免有效性 + bootstrap 有效期），退出码与 `GATE_INCOMPLETE` 语义统一（ADR §6.2/§6.11） | Developer | P0a | 单测 + 运行 |
| T-B2 | `--release` 模式：显式 `base..head`、阻断项/延期处置/基线非回归/全量 QC 校验，输出 `release_verdict` 与 `release_block_reason` | Developer | T-B1 | 单测 + 样例 |
| T-B3 | 显式 range 与默认 `HEAD^..HEAD` 降级为提示；`RANGE_UNBOUND` | Developer | T-B2 | 单测 |
| T-B4 | AC-REF/AC-VERIFIED 语义与措辞（移除 `AC coverage`） | Developer | T-B1 | 单测 + 输出快照 |
| T-B5 | G-2 同版本摘要校验 + `Non-normative` 收窄 + ADR `Revision` | Developer | T-B2 | 单测 |
| T-B6 | 门禁自保护：执行器/CI 变更由基线版本 + IV 独立 diff 复核；外部检测 `GATE_NOT_RUN` | Developer + IV | T-B5 | GV-S21 + CI 记录 |
| T-B7 | CI：治理门禁独立 workflow（避免整 workflow 触发 MySQL verify）；`docs/**`、`prompt/**`、`.specify/**` + `AGENTS.md` 过渡控制 | Developer | T-B6 | workflow 文件 + 运行 |
| T-B8 | P0b 出口：**review-scope** 运行（不是 local），验证 GV-S21 缺失/摘要不符时 G-1 FAIL；预期退出码按 ADR N-7 | Developer + IV | T-B1…T-B7 | GV-S21 + 运行 |

**退出条件**：四闸门完整；Release 模式可判 BLOCKED/PASS；自保护在诞生当次被执行；CI 只跑治理门禁时成本符合 ADR §8。

---

## 6. P1 — 分级、联签与载体

| ID | 任务 | Owner | 依赖 |
|---|---|---|---|
| T-C1 | Tier 注册表 + 总函数（L0/L1 封闭枚举 + 未列举 L2 + 人类裁决）；`--print-tier-rules` | Developer + Architect | P0b |
| T-C2 | 治理/工具链 L2 路径与 MANAGED 表；`PC-*` per-change 规则（`parent_change_id` + 仅追加） | Developer | T-C1 |
| T-C3 | GV-S19 注册表、GV-S20 审计行（`mode=waiver/approval/bootstrap/release/tier`）、GV-S22 release scope | Lead + Developer | T-C2 |
| T-C4 | L1 联签：`qc_signoff`/`reviewer_signoff` + `verdict_at` 顺序校验 | Developer | T-C2 |
| T-C5 | GV-S9/S15/S17 人类写载体与读取顺序、失败码 | Developer | T-C3 |

---

## 7. P2 — QC 差分与骨架

| ID | 任务 | Owner | 依赖 |
|---|---|---|---|
| T-D1 | QC delta/full 身份（spec_id + version + digest）、未复审清单、Release object set（`release_scope` 7 项校验） | Developer + Architect | P1 |
| T-D2 | Scaffold 生成器（plan/tasks/evidence-matrix 结构 + 主权声明行）；模板版本与摘要 | Lead + Developer | P1 |
| T-D3 | 审计载体与追加格式、`spec_version/adr_revision` 字段 | Developer | T-D1 |

---

## 8. AC → 阶段映射（证据矩阵见 `evidence-matrix.md`）

| AC 区间 | 主题 | 阶段 |
|---|---|---|
| AC-69…74 | 门禁执行器完整性 | P0a/P0b |
| AC-75…81、97、98、103、108、109、111 | 分级/错分/注册表 | P1 |
| AC-82…85、101 | 联签/人类批准域/L1 执行证据 | P1 |
| AC-86…89、105、110 | QC delta/full/Release 对象集 | P2 |
| AC-90…92 | Scaffold | P2 |
| AC-93…96、99、100、102、104、106、107、112 | 异常/证据/自保护/bootstrap | P0a/P0b/P1 |
| AC-01…68 / INV-01…50 | 业务基线 | **不变**（回归证据） |

---

## 9. 风险与 Stop-the-line

| 风险 | 处置 |
|---|---|
| P0a 原子提交前规格被提交 → G-3 红 | 以 `BASE` 投影 + 未跟踪约束执行（ADR §7.1）；违反即停 |
| 台账批量 `DEFERRED` 换绿 | 禁止（INV-69/AC-99）；P0a 只允许 `PENDING` 播种 |
| 执行器被自改恒 PASS | IV 独立 diff + 基线版本执行；`GATE_NOT_RUN` |
| 分级被用于绕过 L2 | 未列举一律 L2；错分 stop-the-line + Reviewer 台账 |
| CI 触发整 workflow（MySQL） | 治理门禁独立 workflow（ADR §8/Q-16） |
| 摘要漂移（untracked 稿） | 以 P0a 提交 blob 为 canonical；untracked 摘要仅评审时点 |

任何 Reviewer FAIL / QC BLOCKED 即冻结 `GOV-COST-01`，直到修复 Plan、契约裁决与重审完成。

---

## 10. 【测试范围】与【未覆盖范围】

- **测试范围**：`scripts/collab_gates.py` 单测与 `scripts/test-collab-gates.sh`；`make gates`/`gates-strict`；`BASE` 投影复现命令；CI 治理 workflow 触发；台账/证书记录格式校验。
- **未覆盖范围**：未在 GitHub runner 实跑；未做生产/预发部署；D-CR-6/D-CR-7 未决部分的默认值；成本数字仍为估算（本 Plan 不设阈值）。

---

## 11. 上线与回滚

- 按 R0…R6（ADR §18.1）分阶段合并；每阶段独立 `change_id` 子提交、独立证据。
- 回滚以 revert 分阶段提交表达，保留变更记录与错分台账；不得静默回滚治理规则；不得用“未来会放宽”提前跳过当前门禁。
- 无业务数据迁移、无服务发布；Release 仍由人类按 SPEC-0003 AC-110 放行。
