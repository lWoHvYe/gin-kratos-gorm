# QC-31 — Batch B 终轮增量复评（QC-30 后）

| 项 | 值 |
|---|---|
| review_scope | 仅核对 QC-30 的 2 MAJOR 与 6 MINOR 关闭情况 + K-7 分类：B-1 归档措辞（3 个 pair README/索引 + `docs/architect/README.md`）、T-002 头部、ADR addendum 头部、`review-tracking.md:341` K-7。只读 |
| 被审对象 | 工作区未提交变更 **29 个已跟踪文件 + 2 份 QC 报告**（全为 `.md`）；本轮增量 = `docs/architect/README.md`、`docs/collab/developer-reviewer/{README.md,threads.md,threads/T-002.md}`、7 个 ADR addendum 头部、`docs/review-tracking.md`（+K-7） |
| 版本绑定 | 仍**未提交**：HEAD `104b9276fa2eafd5ac54ad6d9a55a2d701a0dddd`；sha1 = 工作区 blob（`git hash-object`） |
| 对照基线 | 宪法 v1.3.0 `f9c4f1fdbfe6`（未变）；`AGENTS.md` `a1b962eec421`（未变）；`CONVENTIONS.md` `0f2745f0db21`（未变）；ADR-0007 Rev 2.1 `e7e72023a21c`（未变）；`review-tracking.md` `1f53c33b2bf5`；`docs/architect/README.md` `6ec66ef5ead3`；`developer-reviewer/README.md` `e096c588420c`；`developer-reviewer/threads.md` `698e103dd49f`；`T-002.md` `677079cb144f` |
| 审查类型 | 终轮增量复评（静态、只读）；未运行测试（纯文档），不构成 AC 验收或 Release PASS |
| 结论 | **`READY FOR HUMAN BATCH B APPROVAL`（with 7 MINOR）** —— **0 BLOCKER / 0 MAJOR / 7 MINOR**（全部非阻断；见 §3） |

## 1. 遗留项关闭情况

| 来源 | 项 | 判定 | 证据 |
|---|---|---|---|
| QC-29 | BLK-1 轻路径 vs 宪法 | **CLOSED** | 宪法 `:77`（v1.3.0，`:109`）与 `AGENTS.md:84` 同源；ADR 修订显式排除；人类合入/Release 保留 |
| QC-29 | MAJ-1 B-2 走轻路径 | **CLOSED** | `review-tracking.md:324-329`（B-2 = 独立 ADR 修订）；ADR Rev 2.1 批准 `:1500-1501` + `:1533` |
| QC-29 | MAJ-2 线程退役未收口 | **CLOSED** | 见 §2 第 1 项：三处命令式文本已转“已退役/上报 Lead” |
| QC-29 | MAJ-3 死命令前置 | **CLOSED** | ADR 全文 `G-4`/`--release`/`--range` = 0 命中；替换措辞 `:115/:854/:1061/:1067` |
| QC-29 | MAJ-4 `thread` 格式 | **CLOSED** | `review-tracking.md:337` K-5；`CONVENTIONS.md:188/:200/:142` + `architect/MEMORY.md:46` 一致 |
| QC-30 | MAJ-1 归档残留命令 | **CLOSED** | `developer-reviewer/README.md:20`（“以下为…历史记录…不得新增线程”）、`:21`“（已退役）”、`:25`“（已退役）…状态字段只保留原值”；`threads.md:20-21`“维护规则（历史存档·只读）…不得新增线程”；`docs/architect/README.md:6-7`“上报 Lead 路由至 owner” |
| QC-30 | MAJ-2 允许清单 vs 归档改动 | **CLOSED（按 K-7）** | `review-tracking.md:341-343`：“pair README/threads/index archive edits are **L0 non-normative companion cleanup**, not light-path L2; the light-path L2 scope for B-1 is the `CONVENTIONS.md` rule change only. No allowlist expansion is needed.” |
| QC-30 | MIN-1 T-002 CLOSED 语义 | **CLOSED（残留 1 MINOR）** | `T-002.md:7/:10/:11` = `status: CLOSED` + `closed_by: Reviewer` + `archived: true`；`threads.md:11` 落点列“archived — pair threads retired 2026-11-25（历史记录，Review-01 FAIL 与 AC 指针保留原样）”、`:17` 状态统计区分 T-001（复核通过）与 T-002（退役归档）。语义可辨；`archived` 字段说明见 §3-M5 |
| QC-30 | MIN-2 归档头部 Rev 2.0/未裁决 | **CLOSED** | 5 个合并 addendum + `amd1` 头部均改为“**superseded by ADR-0007 Rev 2.1 (2026-11-25)**”（如 `t12:5`、`amd1:5`）；`c04-draft:5/:121`、`c04-rulings:5/:97` 同步；`t14b1:5-6`“未并入 Rev 2.1…C-1 CLOSED”；`t14b2:5-6`、handoff `:5-6` 改为 Rev 2.1。`grep 未裁决` 在 addenda = 0 命中（主 ADR 剩余 4 处均为“已关闭/0 项”的正确表述） |

## 2. 本轮三项验证

| # | 验证项 | 结果 | 证据 |
|---|---|---|---|
| 1 | QC-29/QC-30 全部 BLOCKER/MAJOR 关闭 | **通过** | §1：BLK-1 + 6 MAJOR 全 CLOSED；`grep "必须开\|新增线程"` 仅剩“**不得**新增线程”类禁止句与历史正文（`architect-developer/README.md:25` 明确“2026-11-25 起不再新增线程”） |
| 2 | K-7 与 §2.3 L0 一致 | **通过（附 1 MINOR）** | `CONVENTIONS.md:84` L0 = “仅非规范性文本（文档/注释/格式）；不改变行为、契约、状态、权限或生成物语义；无 Spec/ADR/QC/Plan；人类合入”。pair `README.md`/`threads.md`/index 与 `docs/architect/README.md` 的改动只涉及归档横幅、命令式→历史式措辞、交叉引用，属非规范性文本，不改契约/行为/权限；`CONVENTIONS.md` 规则变更仍由 B-1 的 L2 轻路径承载（`review-tracking.md:324-329`）。**未扩张任何门禁**。边界项见 §3-M5（T-002 的 `status` 字段变更）与 §3-M6（`docs/architect/README.md` 未在 K-7 中点名） |
| 3 | 无新 BLOCKER/MAJOR；无新 gate/CI/state/spec | **通过** | `git status` 29 个已跟踪改动全为 `.md`；`docs/specs/`、`scripts/`、`.github/`、`Makefile`、`internal/`、`api/`、`migrations/` 均无改动（`git status --porcelain` 为空）；ADR 与宪法本轮**未再修改**（sha1 同 QC-30） |

## 3. MINOR（7 项，全部非阻断）

| ID | 内容 | 证据 | 最小修复 |
|---|---|---|---|
| **M1** | 变更输入仍缺：无 `change_id`/`Tier`/base/head/`allowed_paths`/`Tier-Rationale` | `AGENTS.md:86`；`review-tracking.md:324-343` 只有路径与 K 编号；`plan:38` 要求“一个 change_id” | 提交时写 trailer `change_id: LC-BATCH-B`、`Tier: L2`（含 L0 companion 说明）、base `104b927`、head；Reviewer verdict 确认 Tier |
| **M2** | 四份 MEMORY 的 owner 作者性不可验 `[不确定]` | 无提交、无 Actor trailer | 变更输入记各 owner 回写确认，或人类接受“Architect 代拟 + owner 追认”（`Tier: L2` 下可后补） |
| **M3** | 轻路径“台账格式”仍为无限制词 | `AGENTS.md:84`、宪法 `:77` | 下次 PATCH 限定“不得删除或改义既有列；列定义变更仍由 Reviewer 执行并人类批准” |
| **M4** | 宪法仍留退役前口径：`:96` 把 `docs/collab/<pair>/threads/` 列为 delta audit 记录地（现只读存档）；`:75` 仍称“线程协议”为执行细则 | 宪法 `:75/:96`；`CONVENTIONS.md:30/:142` | 下次宪法 PATCH：`:96` 括注改为“（`docs/review-tracking.md`）”、`:75`“线程协议”→“协作路由” |
| **M5** | `archived: true` 是新 YAML 字段，`CONVENTIONS.md:171` 仍定义 `CLOSED` 唯一含义为“沟通闭环”，§4 历史协议摘要也未列 `archived`；T-002 的 `status` 字段变更落在 L0“不改变状态”边界 | `T-002.md:7/:10/:11`；`CONVENTIONS.md:171`、`:142` | 在 `CONVENTIONS.md` §4 历史说明加一句：“历史线程可带 `archived: true`，表示随协议退役归档，不改变 `CLOSED` 的‘沟通闭环’含义”；并在 K-7 注明 T-002 的三字段属归档标注 |
| **M6** | K-7 只点名“pair README/threads/index”，未点名 `docs/architect/README.md` 的归档措辞改动 | `review-tracking.md:341-343`；`docs/architect/README.md:6-7` | K-7 补一句“含 `docs/architect/README.md` 的线程指针措辞（非规范性）”，避免下次被质疑范围 |
| **M7** | `developer-reviewer/threads.md:6` 仍写“协议见 CONVENTIONS §3”（另两个索引已改 §4；§3 现为产物所有权表，线程协议在 §4 历史节） | `developer-reviewer/threads.md:6`；`architect-developer/threads.md:5`、`po-architect/threads.md:6` 均为 §4（已退役，历史说明） | 一处改 `§3`→`§4（已退役，历史说明）` |

## 4. 人类/Reviewer 剩余动作

- **人类**：批准 Batch B 合并（L2 轻路径 + L0 companion）；可选同时采纳 M5/M6 的两句措辞（一次性消除 §3 的语义/范围歧义）。
- **Reviewer**：在 verdict 中确认 `Tier`（L2，含 K-7 的 L0 companion）、绑定 base/head 与 `allowed_paths`；按 QC-29 §6 建议对 Rev 2.1 的 §4.4/§6.2–§6.4/§12.1–§12.4 补一次逐条对照（本轮未复核这些段）。
- 无阻断项：C-1、C-2 已关闭（`ADR:1547/:1559`），K-1…K-7 已记录（`review-tracking.md:330-343`）。

## 5. 限制与确认

- 本轮仅做增量复评：未重跑 QC-30 的 Rev 2.1 抽查；ADR 与宪法 sha1 与 QC-30 相同，故其结论沿用（QC-30 §1/§2）。
- 未运行任何测试（纯文档变更，按 `AGENTS.md:127` 与宪法 `:81` 不得伪造测试声明）；三个 pair 目录 7 个历史线程文件、`ac-coverage.tsv` 36 行均未动。
- 本报告只读：仅新增 `docs/collab/qc-reports/QC-31-BATCH-B-R3.md`；未修改任何被审文件（工作区仍为 29 M + QC-29/QC-30 两个未跟踪文件）。
