# QC-33 — Batch B 提交前终审（ADR Rev 2.3 + SPEC v1.10 approved）

| 项 | 值 |
|---|---|
| review_scope | QC-32 后的四项增量：① N5 修复（approval-id 无 `settled` 请求 → fail-closed）；② ADR 指针更新至 SPEC v1.10 / Rev 2.3；③ SPEC v1.10 批准元数据；④ Batch B 本体未回归。只读 |
| 被审对象 | 工作区 35 项 = **31 ` M` + 4 `??`**（QC-29…33）；本轮改动仍仅 3 个已跟踪文件：ADR-0007（1614 行，Rev 2.3）、SPEC-0002（786 行，v1.10 approved）、`docs/specs/README.md`；另 `review-tracking.md` 新增 K-8 |
| 版本绑定 | 仍未提交：HEAD `104b9276fa2eafd5ac54ad6d9a55a2d701a0dddd`；sha1：ADR `f8c464722c61`（Rev 2.3）、SPEC `e2c8b148b204`（v1.10 approved）、`docs/specs/README.md` `05d97db470ae`、`review-tracking.md` `1a6b629b3132` |
| Batch B 未回归 | 宪法 `f9c4f1fdbfe6`、`AGENTS.md` `a1b962eec421`、`CONVENTIONS.md` `0f2745f0db21`、`docs/architect/README.md` `6ec66ef5ead3`、`developer-reviewer/README.md` `e096c588420c`、`developer-reviewer/threads.md` `698e103dd49f`、`T-002.md` `677079cb144f` —— 与 QC-32 相同 |
| 结论 | **`READY FOR COMMIT`（with 10 MINOR）** —— **0 BLOCKER / 0 MAJOR / 10 MINOR**（4 项本轮新增/残留 + 6 项承 QC-31/32，均非阻断；提交须遵守 K-8 的“分开 commit”） |

## 1. N5 修复（fail-closed）

| # | 断言 | 结果 | 证据 |
|---|---|---|---|
| 1 | approval-id 非空但未请求 `settled` → 启动失败，不默认 `paid` | **成立** | `ADR:3`（Rev 2.3 状态行）；`ADR:827`（`paid` 默认要求“**且 `SETTLE_RELEASE_APPROVAL_ID` 为空**”）；`ADR:828`（N5 config error：mode 未设置/空/`paid`/`shadow`、旧布尔非 `true` → 非零退出，“不得默认 `paid`”）；`ADR:849`（§6.3）、`ADR:880`（§7 跨切面）、`ADR:1064`/`ADR:1072`（§12.3 装配）、`ADR:1614`（§18.4 修复记录） |
| 2 | 无 paid 回退路径 | **成立** | `ADR:821`（`paid` 行：approval-id 必须为空）；`ADR:830`（请求 settled 的批准行校验“不得降级 `paid`”）；`ADR:834`（旧布尔 → settled 请求同校验）；`ADR:835`（回退限制）——N5 只把“默认 paid”收窄为“显式未请求且无 approval-id” |
| 3 | 无残留 `paid` 兜底表述 | **成立** | `grep "按 \`paid\` 运行\|降级 \`paid\`\|回退 \`paid\`"` 全部为禁止句；`ADR:827-828` 与 `:834` 交叉一致（item 7 未重复“approval-id 为空”，由 §6.2 规则 1/N5 + `:1064` 覆盖，属可实现口径，不计 finding） |

## 2. 指针与元数据

| # | 断言 | 结果 | 证据 |
|---|---|---|---|
| 1 | ADR 指针指向 SPEC v1.10 | **成立（表述粗糙）** | `ADR:21` “Spec: [SPEC-0002 v1.10]（v1.10；本 ADR 未修改 SPEC；SPEC 的 `依据` 行仍指 Rev 1.0/Amendment 1，见 §17.4）”——URL/版本正确，但**括号内“SPEC 依据仍指 Rev 1.0/Amendment 1”已为假**（见 MINOR-1） |
| 2 | SPEC 指针指向 ADR Rev 2.3 | **成立** | `SPEC:26` “`docs/adr/0007-payment-settlement-admission.md` **Rev 2.3（latest；switch/gate 见 §12.3/§17.2）**” |
| 3 | SPEC 批准元数据一致 | **成立** | YAML `status: approved`（`SPEC:4`）、`approved_by: Director`（`:8`）、`approved_at: 2026-11-25`（`:9`）；标题 `v1.10 approved`（`:18`）；版本/状态行（`:20/:22`）；变更记录 `v1.10（approved）`（`:763`）；批准记录含 Director 批准 + PO 落库（`:785-786`）；README 索引 `approved / 1.10`（`README.md:82`） |
| 4 | K-8 已入 canonical 台账（QC-32 N1） | **已关闭** | `review-tracking.md:344-347`：K-8 = PO amendment（非 waiver）；QC-32 READY 0/0/12；**separate human approval + separate commit** |
| 5 | 无 code/CI/state 新改动 | **成立** | `scripts/`、`.github/`、`Makefile`、`internal/`、`api/`、`migrations/` 工作区无变更；新增文件仅 QC 报告 |

## 3. 剩余 MINOR（10 项，均非阻断）

| ID | 内容 | 证据 | 最小修复 |
|---|---|---|---|
| **1（新）** | **ADR 自述的“指针未同步”已过期**：`:21` 与 §17.4（`ADR:1549-1553`）仍写“SPEC `依据` 行仍指 Rev 1.0/Amendment 1/零元 Addendum”“README 索引仍链 Amendment 1”——SPEC:26 已改指 Rev 2.3。读者会以为仍在漂移 | `ADR:21`、`ADR:1549-1553` vs `SPEC:26` | §17.4 改为“已同步：SPEC:26 → Rev 2.3；README:82 保留历史 addendum 链接（非规范）”，`:21` 括号同步 |
| **2（新）** | SPEC 批准记录缺 QC 行：v1.10 的 QC delta（QC-32/QC-33）未入批准记录表；v1.8/v1.9 均有 QC-14/QC-18 行 | `SPEC:776-786` 无 v1.10 QC 行 | 增“Quality Checker \| QC-32/QC-33 READY（0 BLOCKER）\| 2026-11-25” |
| **3（承 QC-32 N4）** | Reviewer BLOCKER `[ARCHITECTURAL_FLAW]` 无归属性工件：ADR `:3`/`:1508`/`:1612` 引用，但 `docs/collab/qc-reports/` 无对应 REV 文件（最新 QC-32），`review-tracking` 无 BLOCKER/frozen 条目 | `ls -lt docs/collab/qc-reports/` | 补 REV 报告（Actor/Reviewed-by/base/head/review_scope）或记入 review-tracking |
| **4（承 M1）** | 变更输入/提交元数据未记录：无 `change_id`/`Tier`/base/head/`allowed_paths`；三条轨（Batch B 治理、ADR 修订、SPEC amendment）混在同一工作区 | `AGENTS.md:86`；`review-tracking.md:344-347` 已要求 separate commit | 提交时三笔分开（K-8）：`docs(spec)` SPEC 单独、`docs(adr)` ADR 单独、Batch B 治理+L0 归档单独；trailer 写 `change_id`/`Tier`/`allowed_paths` |
| **5（承 M2）** | 四份 MEMORY 的 owner 作者性 `[不确定]` | 无提交/Actor | 变更输入记各 owner 回写确认 |
| **6（承 M3）** | 轻路径“台账格式”未限定 | `AGENTS.md:84`、宪法 `:77` | 下次 PATCH 限定“不得删/改义既有列” |
| **7（承 M4）** | 宪法 `:96` 仍列 `threads/` 为 delta audit 记录地；`:75` 仍称“线程协议” | 宪法 `:75/:96` | 下次宪法 PATCH |
| **8（承 M5）** | `archived: true` 未在 `CONVENTIONS.md:171`/§4 说明 | `T-002.md:11` | 加一句“archived=随协议退役归档，不改变 CLOSED 语义” |
| **9（承 M6）** | K-7 未点名 `docs/architect/README.md` | `review-tracking.md:341-343` | K-7 补一句 |
| **10（承 M7）** | `developer-reviewer/threads.md:6` 仍指 §3（另两个索引已 §4） | 同文件 `:6` | 一处改为 §4 |

## 4. 提交前清单（人类/Reviewer）

- **人类**：批准 Batch B 提交；按 K-8 要求 **SPEC 单独 commit**（`docs/specs/README.md:89` 亦禁止混提），ADR 规范修订单独 commit，治理+L0 归档单独 commit。
- **Reviewer**：verdict 记 `base=104b927` / `head` / 三组 `allowed_paths` / `Tier`；补 QC-32/33 的 `review_scope`（ADR §6.2/§6.4/§12.5.6/§13.20、SPEC AC-66）；AC-66 用例须覆盖“批准行过期重启 → 启动失败、不恢复 paid”。
- 阻断项：**无**。N5 已 fail-closed；SPEC/ADR 指针与批准元数据一致；1 BLOCKER（启动静默回退）与 SPEC 漂移均已关闭。

## 5. 限制与确认

- 本轮只做增量：未重读 ADR 全文（以 QC-32 基线 + Rev 2.3 差异点核对）；未运行测试（纯文档）。
- 本报告只读：仅新增 `docs/collab/qc-reports/QC-33-BATCH-B-R5.md`；未修改任何被审文件。
