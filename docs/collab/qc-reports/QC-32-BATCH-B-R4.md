# QC-32 — Batch B 终审增量（ADR Rev 2.2 + SPEC v1.10 K-8）

| 项 | 值 |
|---|---|
| review_scope | QC-31 后的三处增量：① ADR-0007 Rev 2.2 启动校验 fail-closed 修复；② SPEC-0002 v1.10（in-review）K-8 amendment；③ Batch B 本体未回归。只读；不替代人类批准/Reviewer verdict |
| 被审对象 | 工作区 34 项 = **31 ` M` + 3 `??`**（QC-29/30/31）；本轮新增改动仅 3 个已跟踪文件：`docs/adr/0007-payment-settlement-admission.md`、`docs/specs/0002-payment-settlement-admission.md`、`docs/specs/README.md` |
| 版本绑定 | 仍未提交：HEAD `104b9276fa2eafd5ac54ad6d9a55a2d701a0dddd`；sha1（工作区 blob）：ADR `1b853f6cc34b`（1606 行，Rev 2.2）、SPEC `184fbcb01159`（787 行，v1.10 in-review）、specs README `a4fbde836cd8` |
| Batch B 未回归 | constitution `f9c4f1fdbfe6`、`AGENTS.md` `a1b962eec421`、`CONVENTIONS.md` `0f2745f0db21`、`review-tracking.md` `1f53c33b2bf5`、`docs/architect/README.md` `6ec66ef5ead3`、`developer-reviewer/README.md` `e096c588420c`、`developer-reviewer/threads.md` `698e103dd49f`、`T-002.md` `677079cb144f` —— **与 QC-31 逐项相同** |
| 审查类型 | 终审增量（静态、红队）；未运行测试（文档变更） |
| 结论 | **`READY FOR HUMAN BATCH B APPROVAL`（with 12 MINOR）** —— **0 BLOCKER / 0 MAJOR / 12 MINOR**（7 项承 QC-31、5 项本轮新增，均非阻断） |

## 1. BLOCKER 关闭验证（启动校验 fail-closed）

| # | 断言 | 结果 | 证据 |
|---|---|---|---|
| 1 | 请求 `settled` 且批准行缺失/过期/非 `APPROVED`/range 不匹配 → **非零退出，绝不回退 `paid`** | **成立** | `ADR:829`（settled：`expires_at` 非空且未过期、`release_range` 匹配；“不得降级 `paid`、不得告警后继续”）；`ADR:830`（shadow 同规则）；`ADR:828`（未知/空 mode → 启动失败）；`ADR:833`（旧布尔 `true` 视为请求 settled，同样校验，“不得按 `paid` 运行”；新旧配置冲突 → fail-fast） |
| 2 | 批准行过期后重启 → 启动失败；运行期不轮询/不热切换 | **成立** | `ADR:835`：“到期只使下一次启动失败，不会自动改模式”；`ADR:847`（§6.3 同规则） |
| 3 | `settled→paid` 只经 §6.4 Director 批准回滚 | **成立** | `ADR:835`（“只能走 §6.4”、“不存在批准行缺失/过期 → 自动回 paid”）；`ADR:863`（回滚须写批准/事件单）；`ADR:115`（A-10“不得由启动校验自动回 paid”）；`ADR:865` 的 `shadow→paid` 直接回退不构成风险（shadow 无准入权威，INV-49） |
| 4 | `settlement_switch_approvals.expires_at` 对 `settled` NOT NULL | **成立** | `ADR:1268-1269` `chk_switch_settled_expiry`；`ADR:1259`（列注释）；`ADR:1280-1281`（存量 NULL 行须 Director 重签，否则请求 `settled` 启动失败） |
| 5 | §12.3 step 0 只记录、不再写 task | **成立** | `ADR:1060`：“只双消费并记录 `settlement_shadow_diffs`/告警；**不建单、不推进组、不放行、不写 settlement task/refund application**” |
| 6 | 无残留“回退/降级 paid”表述 | **成立** | `grep "按 \`paid\` 运行\|降级 \`paid\`\|回退 \`paid\`"` 全部出现在禁止句；`ADR:1606` §18.4 记录原缺陷（Rev 2.1 §6.2“按 `paid` 运行并告警”）与五点修复 |

> 唯一边界见 MINOR-5（仅设 approval id、未请求 mode → 走 `paid` 默认，不失败），不构成“请求 settled 却跑 paid”。

## 2. SPEC v1.10 漂移关闭验证

| # | 断言 | 结果 | 证据 |
|---|---|---|---|
| 1 | 规范文本无 `G-4` / `--release --range` | **成立** | `grep` 仅 3 处，全为历史描述：`SPEC:20`（版本行“移除已退役 G-4…”）、`SPEC:763`（v1.10 变更记录）、`SPEC:784`（K-8 审批日志）；规范条款 0 命中 |
| 2 | SPEC 与 ADR 用同一 K-6 门禁串 | **成立** | SPEC `:394`/`:572`/`:577`/`:678`/`:704`/`:744` = “SPEC-0002 §7 矩阵 + 五方终态 + Reviewer 人工核对 + `review-tracking` 记录（协作闸门已退役，无命令可跑）”；ADR `:115`/`:856`/`:1061`/`:1067` 同串；仅反引号排版差异 |
| 3 | AC-66 与 D-27 / ADR 映射一致 | **成立** | `SPEC:393-394`（OFF→SHADOW 仅影子批准；SHADOW→ON 需 §7 矩阵 + 五方终态 + Reviewer 人工核对 + review-tracking + Director/人类批准）；`SPEC:704`（D-27 同步改引 ADR Rev 2.1，保留移除日期与回滚）；ADR AC-66 行 `:1464` 补“请求缺/过期/不匹配 → 启动失败（非零），不回退 paid” |
| 4 | 仅 AC-66 变更，无其他 AC/INV | **成立** | `git diff` 只改 5 处规范点（AC-66 `:393-394`；§5.9 `:572`/`:576-577`；D-27 `:704`；§9 `:678`；§12 `:744`）+ YAML/头部/变更记录/审批日志；INV 段（含 INV-49 `:133`）未改 |
| 5 | EARS 保留 | **成立** | `:393-394` 均 `WHEN … THE SYSTEM SHALL …`；`IF 开关被误置…fail-closed`、`WHEN 切换完成…order.paid 不得再作为准入依据` 原样保留 |
| 6 | 索引同步 | **成立** | `docs/specs/README.md:82` = `in-review / 1.10`；YAML `status: in-review`、`approved_by/at: null`（`SPEC:22`）——**尚未批准，不得当规范源** |

## 3. Batch B 本体

- 29 个 Batch B 文件 sha1 与 QC-31 **全部相同**（见头部）；轻路径/K-7 L0 归档/线程退役/T-002 归档均未回退。
- 无 code/CI/state/脚本改动：`scripts/`、`.github/`、`Makefile`、`internal/`、`api/`、`migrations/` 在工作区无变更；新增文件仅 3 份 QC 报告。
- 新增改动 **不属于** Batch B 轻路径：ADR 是独立 ADR 修订（K-4 轨道），SPEC 是 PO-owned 契约修订（K-8，in-review）。

## 4. QC-31 的 7 项 MINOR（全部仍 open，均非阻断）

M1 变更输入（`change_id`/`Tier`/base/head/`allowed_paths`）未记录（`review-tracking.md:324-343` 无，`AGENTS.md:86`）；M2 四份 MEMORY owner 作者性 `[不确定]`；M3 “台账格式”未限定（`AGENTS.md:84`、宪法 `:77`）；M4 宪法 `:96` 仍列 threads 为 delta-audit 地、`:75` “线程协议”措辞；M5 `archived: true` 未在 `CONVENTIONS.md:171`/§4 说明；M6 K-7 未点名 `docs/architect/README.md`（`review-tracking.md:341-343`）；M7 `developer-reviewer/threads.md:6` 仍指 §3（另两个索引已 §4）。上述文件 sha1 未变，故状态承 QC-31。

## 5. 本轮新增 MINOR（5）

| ID | 内容 | 证据 | 最小修复 |
|---|---|---|---|
| **N1** | **K-8 未进入 canonical 台账**：`review-tracking.md` 仍只到 K-7（sha1 未变），而 SPEC 以 K-8 为依据（`SPEC:20/:22/:784`），ADR Rev 2.2 的批准只在 ADR 内（`:1504`、`:1541`） | `review-tracking.md:1-6` 声明其为唯一运行状态台账；`review-tracking.md:341-343` 只到 K-7 | 在 2026-11-25 legacy cleanup decisions 增 K-8（授权 SPEC amendment；Rev 2.2 批准；Reviewer BLOCKER 修复），并引用 `SPEC:784` |
| **N2** | 版本指针互指落后一版：`ADR:21` 写 “SPEC-0002 **v1.9**（approved）”，`SPEC:26` 写 “ADR **Rev 2.1**”，`ADR:1590` §18.4 列头仍 “**Rev 2.1** 处理”，`ADR:1383` §13.16 仍 “以 SPEC v1.9 为靶” | 当前实为 ADR Rev 2.2 / SPEC v1.10(in-review) | 统一为“Rev 2.2 / SPEC v1.9 approved + v1.10 in-review”；SPEC 批准后把 `ADR:21`/`:1383` 改为 v1.10 |
| **N3** | 提交分离：`docs/specs/README.md:89` “一个 commit 一份 Spec：不得把 Spec 与代码、ADR 或其他业务文档混提”；当前 SPEC v1.10 与 ADR Rev 2.2、Batch B 治理/L0 归档在同一工作区 | `docs/specs/README.md:89`；`git status` 31 ` M` | SPEC 获批后**单独** `docs(spec)` commit；ADR 修订单独 `docs(adr)`；Batch B 治理+L0 归档单独 commit（各自 trailer 写 Tier/allowed_paths） |
| **N4** | Reviewer BLOCKER 无归属工件：ADR 三处引 “Reviewer BLOCKER [ARCHITECTURAL_FLAW]”（`:3`、`:1508`、`:1606`），但 `docs/collab/qc-reports/` 无对应 REV/QC 文件（最新仅 QC-31），`review-tracking` 无 BLOCKER/frozen 条目 | `ls -lt docs/collab/qc-reports/` 最新 = QC-31；`grep ARCHITECTURAL_FLAW review-tracking` 无相关条目 | 补该 REV 报告（`Actor`/`Reviewed-by`/精确 base/head/`review_scope`）或至少把 verdict 记入 `review-tracking` frozen/BLOCKER 区 |
| **N5** | 配置边界：仅设 `SETTLE_RELEASE_APPROVAL_ID`、而 `FULFILLMENT_SETTLE_CONSUMER_MODE` 未设且旧布尔 false → 按 `ADR:827` 走 `paid` 默认，不失败、无提示（已 provision 的批准行被静默忽略） | `ADR:827`（唯一允许的默认路径）、`:829`（仅“请求 settled”时校验 approval id） | `§6.2` 增一条：“approval id 非空但未请求 mode → fail-fast，或显式 WARN 并要求显式 `mode=paid`”；不改主路径语义 |

> 测试可测性提示（不计 finding）：AC-66 正文未明写“批准行过期重启”的期望结果，该行为由 INV-49（`SPEC:133`）+ ADR §6.2 承载；Reviewer 的 AC-66 用例应至少含“过期批准重启 → 启动失败、不恢复 paid 准入”，否则只测到 happy path。

## 6. 人类/Reviewer 剩余动作

| 角色 | 动作 |
|---|---|
| 人类 | ① 批准 **Batch B**（治理 L2 轻路径 + K-7 L0 归档）；② **单独**批准 **SPEC-0002 v1.10**（现 `in-review`/`approved_by: null`；批准后 PO 改 YAML 为 approved、同步 `docs/specs/README.md:82`、单独 commit）；③ 可选一并采纳 N1（K-8 入台账）与 N2（指针）。 |
| Reviewer | 在 verdict 记 `base=104b927`/`head`/`allowed_paths`（治理、ADR、SPEC 三组分开）、确认 `Tier`、补 N4 的 BLOCKER 工件；对 Rev 2.2 的 `§6.2/§6.3/§6.4/§12.5.6/§13.20` 写明 `review_scope`；AC-66 测试矩阵覆盖启动校验失败分支。 |
| 阻断项 | **无**：1 BLOCKER（启动静默回退）与 SPEC 漂移均已关闭；C-1/C-2 已于 2026-11-25 关闭（`ADR:1547/:1559`）。 |

## 7. 限制与确认

- ADR 与 HEAD 之间隔着 Rev 1.0→2.2 的整批改动，本轮以 **Rev 2.2 现行文本逐项核对** + QC-31 基线（Rev 2.1 sha1 `e7e72023a21c`）比对，未重做整文件逐句 diff。
- 未运行测试（纯文档）；`ac-coverage.tsv` 36 行、三个 pair 目录 7 个历史线程未动。
- 本报告只读：仅新增 `docs/collab/qc-reports/QC-32-BATCH-B-R4.md`；未修改任何被审文件。
