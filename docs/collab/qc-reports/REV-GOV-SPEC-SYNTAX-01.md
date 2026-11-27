# REV-GOV-SPEC-SYNTAX-01 — 宪法 v2.1.0 / 根 `AGENTS.md` 治理调整审查

| 项 | 值 |
|---|---|
| change_id | **`GOV-SPEC-SYNTAX-01`** |
| 级别 | **治理变更**（宪法 3 变更分类；AGENTS §3/§4 已含该值；第一次审查的 NB-14 已在 re-review 消解） |
| 阶段 | **Spec** |
| review_scope | 只审 v2.1.0 的治理文本增量：`.specify/memory/constitution.md`（v2.0.0→v2.1.0）与根 `AGENTS.md`（v2.0→v2.1）相对本增量前文本的改动，连同判断这些改动所必需的 Spec Kit 1.4.3 实际行为、本仓实际路径与台账结构。**不审**代码、SPEC-0001/0002 的 AC 语义、C-2/C-3/C-4/D-2 的完成度、13 个 ADR addendum 的合并、任何实现/测试证据。 |
| base | **`9e81497`**（`git rev-parse HEAD` = `9e81497214bad5b4b92cca6063d1ae8edab0d0c1`） |
| head | **工作树 @ `9e81497`（未提交）**。第一次审查（FAIL）对象：`git diff 9e81497` sha256 `21b4a9941e6d9d8b59305529591dea51b3e0a7739c8f7ca11757f07523849868`；宪法 sha256 `411b15f46c594c635258b2ffce218d9a3a076d98e6658f3fec3384458f2e829b`；`AGENTS.md` sha256 `444e75ae69ead387a4c75ab6cba1404f3389fae281fdce99a1a4867280c2d5bf`。**re-review（PASS）对象见 §0A**：diff sha256 `23cbdb0b8a9ece434dbf2890419ef83031cf8325866d2fdf3ec9873389c68800`、宪法 `1314c3a169848bc10351bb228241ef53623d15c6a63579b473733013ead3e95a`、AGENTS `cd6d4394ac286bc23ae1221377e00dfed09a8cceb95324fb885d013af842f3cb`。工作树若在上述字节之外变化，裁决失效。 |
| final head | **2026-11-27 实例名移除后的最终确认输入**：宪法 sha256 `f844060ee4b51d1d9c01d666b577b1bc874595a40a4f7d487b7c9af08be374b6`、AGENTS sha256 `e6e05c3dde851a7528e627a672ff133707ce92cd366b1a4a0abd8c62a6456d6f`、`git diff 9e81497` sha256 `0d3f4abc0caaaa64dba57521f7f985dffe8bdaadb9c80c4ea1314a774cf57535`（确认输入）；当前 `git diff 9e81497` sha256 `841c98bb2b68ca2340da15662db9cfdb563f49aafed432f15cd8bc400ff6a311`（含 reviewer 落盘与台账新增人类指示条目，见 §0A）。**实例名移除不影响 B1**：新 30 的通用表述仍锁定原语义；宪法/AGENTS 已无 `PAY-SETTLE-01`/`PAY-AC50-REVIEW-01`/`SPEC-0002`（grep 0 命中）。 |
| 契约输入 | 宪法 v2.1.0（工作树）；根 `AGENTS.md`（工作树）；base `9e81497` 的宪法 v1.3.0 与 `AGENTS.md`（`git show 9e81497:...`）；Spec Kit 1.4.3 的 `.dsh/skills/speckit-{specify,implement}/SKILL.md`、`.specify/templates/spec-template.md`、`.specify/scripts/bash/common.sh`；`docs/specs/0001-*.md`、`docs/specs/0002-*.md`、`docs/specs/README.md`；`docs/review-tracking.md`；旧 `docs/archive/2026-11-27-governance/CONVENTIONS.md`（只作对照）。`docs/convert/DECISIONS.md`、`A1-SELFCHECK.md` 只作定位/线索，未据其任何结论判定。 |
| INV | 不涉及领域不变量（无资金/库存/履约/权限/迁移行为变更；INV-01…52 文本未动）。受影响的是治理不变量：宪法 **1/3/8/9/12/18/27/30** 与 **Governance**（版本、最小增量）。 |
| verification | **静态审查**（治理文本，无代码可复跑）。实测：`git diff --check 9e81497` exit 0；`bash .specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks` → exit 1 “Feature directory not found”（证明本仓当前无 Spec Kit feature 上下文）；`common.sh:642-650` 逐行核对 override=整体替换；`grep` 核对台账最新块、change_id、tasks 勾选、旧规则残留。未运行任何 Go 测试/MySQL/NATS/Redis/`-race`（不适用）。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（本次调整的人类指示人；**尚未批准本裁决或 v2.1.0 文本**；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge；不批准 v2.1.0 文本进入生效版本 |
| overall verdict | **第一次（2026-11-27）：FAIL**（1 条阻断 B1；14 条 NB；5 条继承遗留；3 处 `[TODO]`）。**re-review（2026-11-27）：PASS**（B1 与已声明的 NB/L1/L4/L5 消解；见 §0A）。本报告 §0–§7 保留第一次审查原文，§0A 为 re-review。**final confirmation（2026-11-27）：PASS 仍成立**（实例名移除为 Governance 合规修正，B1 语义未变；见 §0A Final confirmation）。 |

---

## 0A. Re-review（2026-11-27，第二次）

| 项 | 值 |
|---|---|
| 裁决 | **PASS**（范围：B1、NB-1…NB-10、NB-14、L1、L4、L5；L2/L3/NB-13 本次不阻断，另行修订；NB-12 已由台账 2026-11-27 人类指示条目记录“收益/停止条件”） |
| head | 工作树 @ `9e81497`（未提交）。**被审状态**（作者修复后、reviewer 落 L5/台账前）：`git diff 9e81497` sha256 `23cbdb0b8a9ece434dbf2890419ef83031cf8325866d2fdf3ec9873389c68800`；宪法 sha256 `1314c3a169848bc10351bb228241ef53623d15c6a63579b473733013ead3e95a`；AGENTS sha256 `cd6d4394ac286bc23ae1221377e00dfed09a8cceb95324fb885d013af842f3cb`。**落盘后**（含 reviewer 的 L5/台账/本报告）：`git diff 9e81497` sha256 `ed2f51f9ecf264648cc3aa43ed78a8c0089c984596ac1a8ccd1d2a7582dc7a91`。 |
| 方法 | 重新逐行核对新 30、附录 A 行 1/7/8、兼容注、AGENTS §1–§6；不沿用第一次的结论。L5 用归档 `ac-coverage.tsv:2-7` 与 SPEC-0002 §15.3 核对后在 §15.2 补行；未改宪法/AGENTS/代码/tsv/归档。 |
| release_status | **NOT RELEASE**；本 PASS 不批准 merge/Release；人类批准 v2.1.0 文本后方生效 |

### B1 三项特别核对

| 问题 | 结论 | 证据 |
|---|---|---|
| SPEC-0002 的 `change_id` 是否仍被文件名主干覆盖 | **不再** | 宪法 30（`:55`）把“Spec 身份”定义为文件/目录本身，change_id 单列：“在途或长期 Spec 的 change_id 在台账登记，登记后不得更换”。**旧文本的实例示例已移除（Governance 合规）**；SPEC-0002 的已登记 change_id 仍在台账（`PAY-AC50-REVIEW-01`/父 `PAY-SETTLE-01`），文件名主干只作 Spec 身份，不再作 change_id。 |
| SPEC-0001 无 `change_id` 是否被强制回填 | **不强制** | 30（`:55`）“历史变更不回溯补登”；扁平文件分支只定义 Spec 身份；新变更在台账登记 change_id 即可。[SPEC-0001](docs/specs/0001-fulfillment-quantity-split.md) 头注仍无 `change_id`（`:1-14`），未被要求补登。 |
| AC 编号语义是否不再冲突 | **不再冲突**（R-1 已由 AGENTS §4 `<Spec>` 位移格式消解） | 附录 A 验收表行（`:87`）：“新 Spec 的 AC 可用 `<change_id>-AC-<序号>`；长期 Spec 沿用原 `AC-<序号>`；退役的编号不得复用。AC 编号标识 AC，不要求等于 change_id”。SPEC-0002 的 `AC-39…72` 与 `PAY-AC50-REVIEW-01` 不再互相推导。 |

### 逐项复核

| 项 | 结果 | 证据 |
|---|---|---|
| B1 | **消解** | 宪法 30 `:55`；附录 A 验收表行 `:87` |
| NB-1 | **消解** | 兼容注 `:90`：“每条被实现或验收依赖的 scenario 必须映射到验收表的一行 AC，不能只引用 `FR`/`SC` 而遗漏 scenario” |
| NB-2 | **消解** | 行 1 `:81`：`SHALL`/`THEN`/`MUST` 均纳入形容词禁令 |
| NB-3 | **消解** | 兼容注 `:90`：Spec Kit 默认布局可用目录名作 change_id；移入 `docs/specs/` 保留台账已登记 change_id，不按文件名重算 |
| NB-4 | **消解** | AGENTS §1 `:8`：必须写原因、未覆盖范围并取得人类书面豁免，否则不构成代码阶段 PASS；§3 `:58` 已在最终改动中同步该条件（R-3 消解） |
| NB-5 | **消解** | AGENTS §6 `:88`：checklist 默认作者自检；作为审查产物时 `[x]` 是审查者判定 |
| NB-6 | **消解** | AGENTS §6 `:88`：`speckit-implement` 没有阶段参数，分批靠操作者中止/重启；本仓仍不可运行（R-4） |
| NB-7 | **消解** | AGENTS §4 `:62`：同日条目按插入位置（块顶最新）判定，日期只用于分组 |
| NB-8 | **消解** | 宪法 30 `:55`：“提交尾注写在合入提交”；与 AGENTS §4 `:74` 一致 |
| NB-9 | **消解** | AGENTS §5 `:80`：治理变更报告放 `docs/collab/qc-reports/`；新扁平 Spec 的 evidence 路径已在最终改动中改为 `docs/specs/evidence/<change_id>/`（R-2 消解） |
| NB-10 | **消解** | 附录 A 行 8 `:88`：“延期是待完成工作，不等于豁免；豁免按第 18 条另行登记（范围、期限、不变量）” |
| NB-14 | **消解** | 宪法 3 `:13` 新增治理变更；AGENTS §3 `:43`、§4 `:66` 级别含治理变更；任务书写“治理变更写无” |
| L1 | **消解** | AGENTS `:30`：“实现变更做第 4-8 步；plan 可以一页以内，但不省略”；`:35`“简单变更的 plan 可以很短”与之不再矛盾 |
| L4 | **消解** | 宪法 30 `:55`：“同一时间只推进一个 change_id，并行须人类批准”；台账并行批准记录由人类随后补（不阻断） |
| L5 | **消解** | SPEC-0002 §15.2 新增 AC-22/28/29/30/35/37 六行（状态“未验证”，检查类型“测试”，证据“-（DEFERRED；见 §15.3 延期清单，expiry 2026-12-15）”，审查者“reviewer-syntax（本次补行；未验收）”）；§15.3 的“附录 A 尚未定义延期清单格式与到期规则”已改为“附录 A v2.1.0 已定义延期清单格式与到期规则；本表是迁移清单”；状态未变（DEFERRED/未验证） |

### 剩余非阻断项（本次不阻断）

| 项 | 说明 |
|---|---|
| NB-12 | **已记录**：2026-11-27 台账人类指示条目写明“收益”与“停止条件”；本次不再作为缺口（人类批准时可再确认）。 |
| NB-13 | 仍为 2.1.0；人类指示条目写“MINOR…非 MAJOR”，但“最终文本待确认”；`[TODO]` 以人类最终批准时的版本分类为准，不阻断。 |
| R-1/R-2/R-3 | **已消解（最终改动）**：AGENTS §4 `:71` 的 AC 位移格式改为 `<Spec> <AC 编号>`；§5 `:80` 的扁平 Spec evidence 改为 `docs/specs/evidence/<change_id>/`；§3 `:58` 已同步 §1 `:8` 的静态审查条件。 |
| R-4 | `speckit-implement` 在本仓仍不可运行（无 `specs/`/`.specify/feature.json`）；属 SDD-STRUCT-ALIGN-01，另案。 |
| R-5 | 新增的台账人类指示条目标题用“人类指示”，不在 AGENTS §4 的 `<审查裁决\|人类决定\|状态变化>` 集合内；属台账格式观察（作者侧记录），本次不阻断。 |
| L2 | 旧 `CONVENTIONS.md` §2.2/§3/§4.1 未迁入；**本次不阻断**，另行修订。 |
| L3 | v1.3.0 的 permit/commitment/Σ、测试替身/启动 fail-fast、`order.paid` 兼容期细节等仍缺；**本次不阻断**，另行修订。 |

### Final confirmation（2026-11-27，实例名移除后）

| 项 | 值 |
|---|---|
| final head | 宪法 sha256 `f844060ee4b51d1d9c01d666b577b1bc874595a40a4f7d487b7c9af08be374b6`；AGENTS sha256 `e6e05c3dde851a7528e627a672ff133707ce92cd366b1a4a0abd8c62a6456d6f`；`git diff 9e81497` sha256 `0d3f4abc0caaaa64dba57521f7f985dffe8bdaadb9c80c4ea1314a774cf57535`（本次确认输入） |
| 实例名检查 | `grep -n 'PAY-SETTLE-01\|PAY-AC50-REVIEW-01\|SPEC-0002\|SPEC-0001' .specify/memory/constitution.md AGENTS.md` → **0 命中**；符合 Governance“实例名不得写进宪法”。 |
| B1 是否仍消解 | **是**。新 30 的规范句未变：Spec 身份 = 文件/目录；“每次变更只有一个 change_id”；“在途或长期 Spec 的 change_id 在台账登记，登记后不得更换”；“历史变更不回溯补登”。删掉的只是示例，不是规则；SPEC-0002 的 change_id 仍以台账登记为准，SPEC-0001 仍不被强制回填。 |
| 附带复核 | 同一次最终改动还消解了 R-1（AGENTS §4 `:71` `<Spec>`）、R-2（§5 `:80` `docs/specs/evidence/<change_id>/`）、R-3（§3 `:58` 同步 §1 `:8`）。 |
| 台账新条目 | 台账随后新增 `2026-11-27 人类指示: GOV-SPEC-SYNTAX-01`（Actor: lead，最终文本待确认），记录收益/停止条件与首轮 FAIL→re-review PASS 链；**不改宪法/AGENTS 文本，不改变 B1/PASS**；NB-12 因此有书面记录，R-5 为其标题类型观察。 |
| 结论 | **PASS 仍成立**。剩余仅 NB-13、R-4、R-5、L2/L3，均**本次不阻断**。 |
| 落盘后 | 当前 `git diff 9e81497` sha256 = `841c98bb2b68ca2340da15662db9cfdb563f49aafed432f15cd8bc400ff6a311`（含 reviewer 的 L5/台账/报告输出与台账新增人类指示条目；被审状态的规范文本以宪法/AGENTS 的 sha256 为准）。 |

> re-review 只推翻第一次的 **B1 阻断**与已声明 NB/L1/L4/L5 的状态；第一次审查的事实、证据绑定和 L2/L3 判断保留在 §0–§7。台账新增 re-review 条目，原 FAIL 条目保留为历史（仅结论被取代）。

---

## 0. 第一次审查结论（2026-11-27，保留原文）

1. **裁决 FAIL，范围只有 v2.1.0 的治理文本增量。** 唯一阻断是 **B1**：宪法 30 新增的“扁平文件取文件名主干”分支与本仓已登记事实冲突——SPEC-0002 头注已登记 `change_id: PAY-AC50-REVIEW-01`、`parent_change_id: PAY-SETTLE-01`（`docs/specs/0002-payment-settlement-admission.md:8-9`），SPEC-0001 头注没有 `change_id`，而 `docs/specs/0001-*.md` 本身也不是任何已登记 `change_id`；按新 30，两个 Spec 都会得到第三个 id。这使“一个 Spec 一个 change_id”在文本内部自相矛盾，并删掉了已登记 id 的不可更换性。**B1 不是笔误，是“Spec 身份”与“变更身份”被合并的结构问题。**
2. **A/C/D/F/G/H 的主要判断成立。** A：行 1 与兼容注保留了可判定性、拒绝错误、证据要求，且与 AGENTS §2 第 1 步、§3 任务书一致；C：宪法 27 明文承认 `[X]` 只是执行进度，形式上解决了与 `speckit-implement` 的冲突；D：与记录中的人类裁决 1-A 一致，独立性未被削弱；F：台账确实不是全局“最新在上”，新条款与“最新块顶部插入”的实际结构相符；G：三项事实核对里模板覆盖层 replace/append 完全正确，合入尾注与人类合入一致；H：延期清单覆盖了 owner/expiry/reason/到期阻断/不得批量续期。
3. **但 A/G/H 各留了可被利用的口径洞**：A 的 `FR/SC` “来源”与 `MUST` 未纳入形容词禁令（NB-1/NB-2），且 Spec Kit 生成目录与扁平文件布局没有对齐（NB-3）；G 的审查报告路径规则与本仓实际 `docs/collab/qc-reports/` 冲突，且对“治理变更无 Spec”没有落点（NB-9），尾注规则与宪法 30 的“提交”范围不清（NB-8）；H 的延期清单与宪法 18 的“豁免/受影响不变量/逐台账”边界未定义（NB-10）。
4. **本次调整没有解决的继承问题**见 §5：plan 与“只做 5-8 步”的口径冲突（L1）、旧 CONVENTIONS 的外部复审/integration owner/reviewer 元数据未迁入（L2）、v1.3.0 压缩掉的 permit/commitment/Σ 与测试替身/启动 fail-fast 等（L3）、“同一时间只推进一个 change_id”被删（L4）、SPEC-0002 §15.2 只列 6 条基线 AC、另 6 条 DEFERRED 基线 AC 没有任何验收状态表落点（L5）。

> 本报告把“**本次 v2.1.0 引入**”与“**继承自 v2.0.0 / v1.3.0**”分开标注。阻断只针对前者；继承项只在 §5 列出，不改变本次 FAIL 的依据，也不据其给作者加新阻断。

---

## 1. 审查对象、口径与证据绑定

### 1.1 本次增量的隔离方式

base `9e81497` 是 v1.3.0；工作树是 v2.1.0。C-1 的 v2.0.0 文本已落在工作树且未提交，git 中没有 v2.0.0 的提交。因此 v2.1.0 的增量用 **`docs/convert/constitution.draft.md` / `AGENTS.draft.md` → 工作树** 的逐行 `diff` 隔离（draft 只作定位线索，其结论不作为证据）；最终判断一律回到工作树文本、base 文本、Spec Kit 1.4.3 脚本与 SKILL、本仓实际文件。

### 1.2 本次 v2.1.0 实际改了什么（逐项）

| # | 文件/位置 | 改动 |
|---|---|---|
| 1 | 宪法 1 | “必须包含七个必备章节” → “必须回答附录 A 的必备问题；章节标题可按 Spec Kit 模板组织，但每个问题都要有落点” |
| 2 | 宪法 27 | 追加“tasks 的勾选只是执行进度标记，Spec Kit `implement` 写 `[X]` 不改变本条” |
| 3 | 宪法 30 | 追加扁平文件 `docs/specs/<NNNN>-<name>.md` 取文件名主干；重写为“一个 Spec 一个 change_id。” |
| 4 | 宪法 Governance/版本 | `2.0.0` → `2.1.0` |
| 5 | 宪法 附录 A 标题 | “必备章节” → “必备内容” |
| 6 | 宪法 附录 A 行 1 | EARS 强制 → “语法不限：EARS 或 Spec Kit 原生语法”；保留可判定、拒绝错误、`SHALL`/`THEN` 形容词禁令 |
| 7 | 宪法 附录 A 行 8 | 新增“延期清单”（AC/工作项、owner、expiry、reason；到期阻断；不得删行/批量续期/新 change_id 规避） |
| 8 | 宪法 附录 A 兼容注 | 新增：允许保留 Spec Kit 四章节、必须给“附录 A 对照”、稳定 AC 编号、`FR/SC` 作为来源、工具语法不豁免可判定性/异常分支/错误码/证据 |
| 9 | AGENTS §1 | 审查者从“subagent 或无运行时静态会话”改为“独立 subagent（有运行时…；不能复跑时声明静态审查）” |
| 10 | AGENTS §2 第 1 步 | “必备章节” → “必备问题；用 Spec Kit 时可用其原生章节…并加附录 A 对照” |
| 11 | AGENTS §2 实现工具条 | 追加“用 `speckit-implement` 时，按 tasks 阶段分批调用或在阶段边界主动中止；工具默认会连续跑完全部任务，不能作为豁免” |
| 12 | AGENTS §3 | 两处“必备章节” → “必备问题” |
| 13 | AGENTS §4 | 台账追加“历史未重排时以标题日期判定新旧，新条目插入当前最新块顶部”；尾注追加“作者不执行合入，由人类在合入提交上补这三个尾注” |
| 14 | AGENTS §5 | 追加“在途变更可沿用 `docs/implementation/**`，但台账必须写精确路径和 `git rev-parse HEAD`” |
| 15 | AGENTS §6 | 追加“项目覆盖层是整体替换；只追加章节用预设/扩展层 `append`，不要用 override 冒充 append” |

### 1.3 不依据作者自述

- `docs/convert/DECISIONS.md`、`A1-SELFCHECK.md` 只用于找位置（如台账行号、裁决编号），**没有**把其中的“已落地/SPD/SCOPED-PASS”等结论当证据；本报告所有事实均用 `git show 9e81497`、`grep`、`sed`、脚本实跑、文件原文重新核对。
- 人类裁决 1-A/2-B 的**原始文本不在仓库内**；只能与 `DECISIONS.md` 的转写比对。这使 D 的一部分只能记为 `[TODO]`（见 §6）。
- v2.1.0 的**人类指示原文也不在仓库内**；本报告只能依据“工作树文本 + 宪法/AGENTS + 工具行为”判断，不能核对该指示的范围与边界。

---

## 2. A–H 逐项判断

### A. 附录 A 行 1 允许 EARS 或 Spec Kit 原生语法

**结论：部分成立。** 可判定性、错误码、证据要求没有丢；但“工具语法成为逃逸口”只被部分堵住。

**A.1 没丢的部分（逐条核对）**

- 行 1 仍要求“每条 AC 必须可判定，并能落到一个测试、一个契约检查或一个可观测指标上”（`.specify/memory/constitution.md:80`）——这是 AC 可判定性的主约束，不依赖句式。
- 金额/状态/并发规则仍必须写“拒绝时返回的错误”（`:80`）；`SHALL`/`THEN` 后仍禁止“快速/及时/合理”等无法判定的形容词（`:80`）。
- 兼容注末句明确“工具语法不构成对可判定性、异常分支、错误码和证据要求的豁免”（`:89`）。
- AGENTS §2 第 1 步（`AGENTS.md:21`）与 §3 的 Spec 阶段审查（`AGENTS.md:54`）都改成“必备问题是否都有落点”，与宪法 1/附录 A 的“问题”口径一致，没有出现“章节齐全即可”的旧口径。
- `speckit-specify` 的模板确实有 `User Scenarios & Testing`（Given/When/Then）、`Functional Requirements`（`MUST`）、`Success Criteria`（`SC-001`）、`Assumptions`、`Edge Cases`（`.specify/templates/spec-template.md:11-131`）；兼容注允许保留的四个章节名与实际模板一致（`Edge Cases` 被归在 `User Scenarios` 下，未单列，见 §2.G 旁注）。

**A.2 仍需堵的洞**

- **NB-1：`FR-<n>`/`SC-<n>` 的“来源”与“编号”边界不清。** 兼容注说“确保每条 AC 在验收状态表里有稳定编号：…新 Spec 可用 `<change_id>-AC-<n>`，也可引用 `FR-<n>`/`SC-<n>` 作为来源”（`:89`）。后半句若被读成“AC 编号可以是 `FR-001`”，就绕开附录 A 验收状态表的“AC 编号用 `<change_id>-AC-<序号>`”（`:86`）与宪法 30 的 change_id 归属，并使“退役的编号不得复用”无法在跨 Spec 层执行；若被读成“来源文本可引用 FR/SC”，则文本没有把这两种读法分开。另：`SC-<n>` 是业务指标（模板 `:115-118` 的例子是“2 分钟/90% 用户/降低 50% 工单”），不是带拒绝语义的验收条件；把它当 AC 来源时，金额/状态/并发规则的拒绝错误仍必须落在 FR 或 Given/When/Then 里，文本没有明说这一点。
- **NB-1（续）：原生 `Acceptance Scenarios` 没有要求逐条成为验收表里的 AC。** 兼容注要求“每条 AC”有稳定编号，但没有定义在 Spec Kit 原生语法下“哪些陈述是 AC”。作者可以写 10 条 Given/When/Then 的 “Acceptance Scenarios”，只在验收表登记 3 条 `FR`，其余 7 条既不是 AC 也不被验收。宪法 9 要求问题可溯源，附录 A 要求“每条 AC”可落证据；这个口径洞主要削弱“AC 完整性”，不是“单条 AC 可判定性”，因此列为非阻断，但正是“工具语法成为逃逸口”的最短路径。
- **NB-2：形容词禁令只点名 `SHALL`/`THEN`，未覆盖 `MUST`。** 行 1 允许 `Functional Requirements` 的 `MUST`（`:80`），但禁令写的是“`SHALL`/`THEN` 后不得接无法判定的形容词”。`System MUST respond quickly` 不在这句话的字面射程内；虽然“每条 AC 必须可判定”仍是总约束，但具体句式守卫出现了语法缝隙。
- **NB-3：Spec Kit 的目录布局与“扁平文件”布局没有对齐。** `speckit-specify` 的固定行为是：在 `specs/` 下建 `<NNN>-<short-name>/` 目录，写 `spec.md`，并把 `feature_directory` 持久化到 `.specify/feature.json`（`.dsh/skills/speckit-specify/SKILL.md:80-106`），还会生成 `checklists/requirements.md`（`:146`）。它**不会**生成 `docs/specs/<NNNN>-<name>.md` 这种扁平文件。兼容注只解决“章节保留”，没有说明生成后的 Spec 如何落到本仓扁平路径；宪法 30 的“扁平文件取文件名主干”分支与“独立目录取目录名”分支因此服务于两套不同的产物布局，作者需要手工移动/压平，文本没有写这个动作的归属。

**A.3 继承的风险（不是 v2.1.0 引入）**

- v1.3.0 的“附加协议”要求 EARS **穷举异常分支**，并明令禁止用行首 `Given/When/Then/And` 规避 EARS（`git show 9e81497:.specify/memory/constitution.md`，附加协议段）。v2.0.0 已经删掉“穷举异常分支”，v2.1.0 进一步允许 Given/When/Then。按人类指示放开语法本身不构成违规；但替代它的只有附录 A 行 2 的“并发、超时、崩溃、部分成功、网络脑裂下系统必须表现成什么”（`:82`）和审查者的人工判断，**没有一条可判定的“异常分支必须逐条落 AC”的机器可核对要求**。这是风险，不是本次 FAIL 的依据。

### B. `change_id` 支持扁平文件

**结论：不成立 → 阻断 B1。**

**B.1 本仓实际路径**

- 扁平文件确实存在：`docs/specs/0001-fulfillment-quantity-split.md`、`docs/specs/0002-payment-settlement-admission.md`（`ls docs/specs/`）。所以“本仓扁平文件”这个前提是真的。
- 但两个文件的**已登记 change_id 不等于文件名主干**：
  - SPEC-0002 头注：`change_id: PAY-AC50-REVIEW-01`、`parent_change_id: PAY-SETTLE-01`（`docs/specs/0002-payment-settlement-admission.md:8-9`）；正文 `:24` 也写 `change_id | PAY-AC50-REVIEW-01（…Parent PAY-SETTLE-01…）`。
  - SPEC-0001 头注**没有 `change_id` 字段**（`:1-14`）；其历史工作项用 `AC-50-FIX-01`、`AC-27-NOWB-01`、`AC-27-LIVE-01` 等 change_id（`docs/specs/0001-fulfillment-quantity-split.md:847,872,879`）。
  - git 提交尾注使用 `change_id: PAY-SETTLE-01`（如 `9e81497`、`d428dfd`、`4016c15`）；审查提交 `bda09a7` 也用 `PAY-SETTLE-01`。
- `docs/specs/README.md` 的硬规则是“一份业务能力一个文件，编号终身唯一”（`:3`）、“`approved` 之后业务口径变更必须原地升版，不新建文件”（`:70`）。也就是说：**一个扁平 Spec 是长期存在的业务能力文件，它一生会有多个 change_id**（SPEC-0002 至少经历 `PAY-SETTLE-01` 与 `PAY-AC50-REVIEW-01`）。新 30 把“Spec 的文件名”直接当 change_id，等于把“Spec 身份”和“变更实例身份”合并。

**B.2 与宪法 30 自身的冲突**

新 30 全文：“一个 Spec 一个 change_id。Spec 有独立目录时取目录名；本仓扁平文件 `docs/specs/<NNNN>-<name>.md` 取文件名主干；两者都没有时在台账登记，登记后不得更换。”（`:54`）

- “登记后不得更换”只挂在**第三分支**（两者都没有）上；对扁平文件分支，文本给出的是**强制取名规则**，没有“已登记的沿用”或“新 Spec 才适用”的限定。
- 于是按字面：SPEC-0002 的 change_id 应改为 `0002-payment-settlement-admission`，而它已登记的是 `PAY-AC50-REVIEW-01`。一个 Spec 同时有两个 change_id，违反新 30 第一句，也违反“登记后不得更换”的精神（虽然该短语不在这个分支）。
- 连带影响：附录 A 验收状态表要求 AC 编号用 `<change_id>-AC-<序号>`（`:86`）。在途 AC-39…72 可用“在途沿用原编号”豁免；但今后给 SPEC-0002 新增的 AC 会落到 `0002-payment-settlement-admission-AC-73`，与在途 `AC-73` 的编号体系并存，进一步加深“一个 Spec 两个 id”的混乱。

**B.3 为什么这是阻断而不是 NB**

- 它直接决定每个新变更的 `change_id`，进而决定提交尾注 `change_id`、台账条目的 change_id、AC 前缀；错了会污染所有后续账。
- 它不是“历史文本没写清”，而是新规则**主动把一个已登记的 id 改名**，且改名对象正是本仓唯一的两个 Spec。作者无法用“新 Spec 才适用”自辩，因为文本没有这个限定。
- 它是**本次 v2.1.0 引入**的（v2.0.0 的 30 只要求“取 Spec 所在目录的名称；没有目录时在台账登记”），不是继承问题。定为 **B1 阻断**。

### C. tasks `[X]` 只是执行进度

**结论：成立（宪法 27 与 `speckit-implement` 的形式冲突已解决），但有残留矛盾。**

**C.1 已解决**

- 宪法 27 现在明文：“plan、tasks 的勾选…都不保存状态；tasks 的勾选只是执行进度标记，Spec Kit `implement` 写 `[X]` 不改变本条。”（`:51`）
- `speckit-implement` 的行为是：完成一个任务就标 `[X]`（`.dsh/skills/speckit-implement/SKILL.md:173`），Done When 要求“All tasks…completed and marked `[X]`”（`:224`）。新 27 没有否认这个行为，只是否认它保存**验收状态**；两者可以并存。按治理条款“任何工具自带的检查与本文件冲突时，以本文件为准”（宪法 `:70`），即使工具的 Done When 把 `[X]` 当完成条件，也不改变 27 的验收语义。

**C.2 残留矛盾（NB-5）**

- `AGENTS.md` §6 说“`checklist` 只是作者自检”（`AGENTS.md:88`）；但 `speckit-implement` 明说 `checklists/requirements.md` 是内建质量清单，而**自定义 checklist 是 reviewer-owned requirements-quality review artifacts，`[x]` 表示 reviewer 判定该质量准则满足**（`.dsh/skills/speckit-implement/SKILL.md:62-63`）。这是第二个“勾选承载判定”的产物，宪法 27 的列举（plan、tasks、MEMORY、任何汇总文档）没有覆盖 `checklists/**`。本次没有解决。
- `speckit-implement` 的 Done When 仍是“全部 tasks `[X]`”（`:224`）；这与“一个阶段一批、批后停下”（`AGENTS.md:33`）不冲突（前者是工具完成定义，后者是人类流程），但工具的默认连续执行会推动作者越过阶段边界；33 的“主动中止”是人工覆盖，不是工具能力（见 E）。
- 台账/plan/tasks 的视图漂移仍存在（例如 `docs/lead/PAY-SETTLE-01/tasks.md:14` 的 `[retro FAIL]`、`:60` 的“Phase 0-R 未闭合”与台账/重排不一致），但 27 已明确视图不保存状态，故不构成新的规则冲突，只是旧漂移未清理。

### D. 审查者改为“有运行时的外部 subagent”

**结论：与记录中的人类裁决 1-A 一致；独立性没有被静态回退条款削弱（宪法 8 的独立性是“不同 actor”，不是“有运行时”）。但回退条款没有可判定的触发条件（NB-4）。**

- 1-A 的转写是“A 外部 subagent，有运行时”（`docs/convert/DECISIONS.md:14`）；AGENTS §1 的审查者行现在写“独立的审查 subagent（有运行时，能复跑测试；不能复跑时在裁决里声明‘静态审查’）”（`AGENTS.md:8`），方向一致。`[TODO]`：人类裁决原文不在仓库，无法核对措辞。
- 宪法 8 要求“审查者必须是另一个 agent 实例，不参与被审变更的写作”（`:20`）；一个没有运行时的独立实例仍然是“另一个实例”。所以“静态审查”不违反独立性；它削弱的是**复核强度**，不是独立性。
- 宪法 12 把“实际执行日志、绑定精确 HEAD”的义务放在**作者**；宪法 13 对资金/库存/权限/跨服务恢复要求并发/故障注入证据。即使审查者静态，作者的证据义务不变；宪法 9 也要求权限、安全、数据完整性、迁移风险“必须阻断”，不因审查者静态而降级。所以静态回退不是宪法违反。
- **NB-4**：但“不能复跑时”没有任何可判定的条件、没有要求写明原因/范围、没有人类豁免或降级声明；而 1-A 的目的正是“能复跑”。一个“有运行时”的审查者可以在不做任何说明的情况下声明静态审查，使 1-A 在资金/库存变更上自我抵消。文本没有给出“什么算不能复跑”的门槛，也没有说静态审查时哪些 AC 不能标已验证。列为非阻断（宪法 12/13 仍兜底），但这是 D 项的实质缺口。

### E. `speckit-implement` 必须分批

**结论：与工具默认行为一致；作为“人类流程规则”可执行，作为“工具模式”不可执行（NB-6）。**

**E.1 与工具默认行为一致**

- 工具“按阶段执行：完成一个阶段再进入下一个”（`.dsh/skills/speckit-implement/SKILL.md:154`），**没有阶段边界停点**；它只在 checklist 有未勾选项时停下来问（`:83-88`）。Done When 要求全部 tasks `[X]`（`:224`）。所以 AGENTS 说的“工具默认会连续跑完全部任务，不能作为豁免”（`AGENTS.md:33`）是**事实**，不是作者猜测。
- 这条规则是对工具默认行为的**人工覆盖**，按宪法 Governance“任何工具自带的检查与本文件冲突时，以本文件为准”（`:70`）有效；不需要工具配合也能成立。

**E.2 为什么“作为工具模式不可执行”（NB-6）**

- `speckit-implement` 没有“阶段范围”参数：它只要求 `$ARGUMENTS` 被“考虑”（`SKILL.md:18`），实际流程是 `check-prerequisites` → 读 `tasks.md`/`plan.md` → 全量执行（`:58-181`）；Done When 仍是“全部 tasks `[X]`”（`:224`）。所以“按 tasks 阶段分批调用”不是工具的原生调用方式，只能靠操作者在提示里限定并在阶段边界**主动中止**。
- **本仓当前根本起不来**：`check-prerequisites.sh --json --require-tasks --include-tasks` 返回 exit 1 “Feature directory not found”；仓库没有 `specs/` 目录、没有 `.specify/feature.json`，分支是 `main`。`common.sh:188-216` 的 feature 解析只认 `SPECIFY_FEATURE_DIRECTORY` 或 `.specify/feature.json`，没有分支名兜底（分支为空时只把 FEATURE_DIR 的 basename 当 `CURRENT_BRANCH`，不解决 feature 不存在）。所以 AGENTS §6 的“第 5 步可以用 implement”（`AGENTS.md:88`）对当前扁平 Spec 布局**不可操作**，除非先造一个 Spec Kit feature 目录；这正是 `SDD-STRUCT-ALIGN-01` 未决问题。分批条款本身没错，但它约束的是一个当前跑不起来的工具。

### F. 台账“最新在上”+“历史未重排以标题日期为准”

**结论：与当前台账实际结构一致；表述有可判定的边界问题（NB-7）。**

**F.1 实际结构核对**

- `docs/review-tracking.md` 顶部（`:14` 起）是 2026-11-24/25 的混合条目，**不是**最新；2026-11-27 的块从 `:333` 开始，块内最新在上：`:333`（human decision: freeze lift extended）→ `:342`（GOV-CONVERT-01 审查裁决）→ `:351`（v2.0 人类决定）→ `:374`（11 项裁决）……到 `:651`。任务要求把新条目插在 `:333` 之前，正是“当前最新块顶部”。
- 所以新条文“历史台账尚未重排时，以条目标题日期判定新旧，新条目插入当前最新块顶部”是对现状的**如实描述**，不是空想；D-2 也只归档不重排（`docs/review-tracking.md:6`）。

**F.2 问题（NB-7）**

- “以条目标题日期判定新旧”只能分辨不同日期；当前 11-27 块内有 30+ 条同日条目，日期无法排序。真正的顺序信息只在“块内位置”，条文没有说同日条目按位置而非日期。插入新条目时“最新块顶部”是确定的，但读旧条目时“哪条更新”仍只能靠位置。
- 回填/补记的风险：如果一条条目的**标题日期**早于最新块（例如补记 11-26 的裁决），按规定仍插在“当前最新块顶部”，就会出现“11-26 条目在 11-27 条目之上”，与“以标题日期判定新旧”冲突。条文没有处理补记。
- “未完成不改变‘最新在上’的语义”是定义性声明，不是事实：文件顶部实际是 11-24/25，全局并非“最新在上”。把它说成语义不变，容易让只读文件顶部的读者误判。非阻断，但建议在阅读顺序上不要依赖该句。

### G. 合入尾注 / evidence 路径 / 模板覆盖层

**结论：三项事实核对中，模板覆盖层完全正确；合入尾注与人类合入一致但埋了范围歧义（NB-8）；evidence 路径与本仓实际冲突（NB-9）。**

**G.1 合入尾注由人类补**

- 与人类裁决 2-B“保留人类合入”（`DECISIONS.md:15`）和宪法 16“合入在审查通过后由人类执行”（`:34`）一致；AGENTS §4 也把作者列为“不做合入”（`AGENTS.md:7`）。
- 但宪法 30 说“**提交尾注**写 `change_id`、`Actor`、`Reviewed-by`，Actor 与 Reviewed-by 不得相同”（`:54`），没有限定“合入提交”；AGENTS §4 现在说“作者不执行合入，由人类在合入提交上补这三个尾注”（`AGENTS.md:74`）。按字面，作者的普通提交也是“提交”，也应带三个尾注；而 git 事实是作者提交只有 `change_id`/`Actor`（有时有 `Tier`/`Status`/`Role`），没有 `Reviewed-by`（如 `d428dfd`、`4016c15`、`9e81497`），审查提交 `bda09a7` 写的是 `Reviewed-by: human (merge pending)`。新句把“补尾注”集中到合入提交，**加剧**了宪法 30 与实际的差距：如果严格按 30，历史上所有作者提交都不合规；如果按 AGENTS §4，30 的“提交”就必须解释成“合入提交”，而文本没有这么说。列为 NB-8。
- 另一个未被说明的点：人类在合入提交上写 `Actor: lead` 时，该提交的 git author 是 human，`Actor` 是“变更作者元数据”还是“git 作者”没有定义。非阻断，但会影响 `Actor != Reviewed-by` 的核对。

**G.2 evidence 路径过渡**

- 事实：仓库中**没有**任何 `evidence/` 目录（`find docs -type d -name evidence` 零命中）；在途证据确实在 `docs/implementation/**`（如 `PAY-SETTLE-01-*selfcheck.md`）；C-3 审查报告在 `docs/collab/qc-reports/REV-GOV-CONVERT-01-C3.md`，本报告也在同一目录。所以“在途变更可沿用 `docs/implementation/**`”与事实一致。
- 但 AGENTS §5 说“测试日志和审查报告放在 **Spec 同目录的 `evidence/`** 下”（`AGENTS.md:80`）。本仓 Spec 是扁平文件，Spec 同目录 = `docs/specs/`，于是所有 Spec 共享 `docs/specs/evidence/`；多个 change_id 会写进同一个目录，条文没有给文件命名/change_id 归属规则。更直接的是：**本次审查报告被要求写在 `docs/collab/qc-reports/`，不在 `docs/specs/evidence/`**——这本身证明 §5 与本仓实际/本次任务不一致。治理变更没有 Spec，§5 对“没有 Spec 的审查报告放哪”没有落点。列为 NB-9。

**G.3 模板覆盖层 replace vs append**

- 事实核对**正确**：`.specify/scripts/bash/common.sh` 的 `resolve_template_content` 对项目覆盖层 `.specify/templates/overrides/<name>.md` 是 `cat` 后直接 `return 0`（`:642-650`）——即**整体替换**；预设/扩展层支持 `replace/prepend/append/wrap/replace-section`（`:740`，组合逻辑 `:865+`）。仓库当前既无 `.specify/templates/overrides/`，也无 `.specify/presets/`，所以“追加用预设/扩展层 append”是可执行但需要新建预设/扩展，不是现成开关；AGENTS 的表述（`AGENTS.md:89`）没有错。
- 旁注：兼容注列的四个章节不含 `Edge Cases`（它在 `User Scenarios` 块内）与 `Key Entities`；不影响“必备问题有落点”的判断，但说明“保留其章节”不是穷举清单。

### H. 新增“延期清单”行

**结论：5 项要素全部覆盖（owner/expiry/reason/到期阻断/不得批量续期）；与宪法 18 没有直接冲突，但“延期 vs 豁免”的边界未定义，形成可被利用的口径缺口（NB-10）。**

**H.1 覆盖核对**

| 要求 | 新行（宪法 `:87`） | 结论 |
|---|---|---|
| owner | “每条延期写 `AC/工作项、owner、expiry、reason`” | 覆盖 |
| expiry | 同上 | 覆盖 |
| reason | 同上 | 覆盖 |
| 到期阻断 | “到期未关闭阻断对应范围” | 覆盖（“对应范围”未定义，见 NB-10） |
| 不得批量续期 | “不得删行、批量续期或用新 change_id 规避” | 覆盖 |
| 受影响的不变量 | **未写** | 缺口 |
| 逐台账 / status / thread 指针 | **未写** | 缺口 |

**H.2 与宪法 18 的关系**

- 宪法 18：“豁免必须逐台账。写明范围、期限、受影响的不变量；没有期限的豁免无效。”（`:36`）
- 新行不是“豁免”，是“延期”；旧模型里两者也是分开的（旧 `CONVENTIONS.md` §5.2 豁免 vs §6.3 延期）。所以**不构成直接冲突**。
- 但新文本**没有定义**“延期”与“豁免”的边界：一条 AC 的延期如果实质上是对某条不变量的验证豁免（例如延期一个资金 permit/Σ 相关 AC），只按附录 A 写 `AC/owner/expiry/reason`，就可以不写 18 要求的“受影响的不变量”，也可以不“逐台账”。这正是“用一个格式绕过另一个格式”的路径。旧 `CONVENTIONS.md:201-203` 的延期行还有 `thread`/`status` 字段；新行没有，导致延期与台账/验收表的指针断开。列为 NB-10。
- 与 27 不冲突：延期清单没有状态列，当前状态仍只在验收状态表（`:51`）。这也意味着“到期未关闭”只能靠验收状态表判断，清单本身不能独立判定。
- 与 SPEC-0002 §15.3 的格式兼容：§15.3 的列正是 `AC | owner | expiry | reason`（`docs/specs/0002-payment-settlement-admission.md:1112-1146`），所以新行与 C-3 的过渡表**一致**；反过来说，C-3 的临时格式现在被宪法追认了，但 §15.3 开头仍写“附录 A 尚未定义延期清单格式与到期规则”（`:1110`），在 v2.1.0 后已成失效陈述 → **NB-11**。

---

## 3. 阻断项

### B1 [BLOCKER] 宪法 30 的扁平文件 `change_id` 规则与已登记事实/“一个 Spec 一个 change_id”冲突

- **违反条款**：宪法 **30**（“一个 Spec 一个 change_id”；“登记后不得更换”）、宪法 **附录 A 验收状态表**（AC 编号用 `<change_id>-AC-<序号>`，`:86`）、宪法 **9**（问题可溯源到变更）。
- **事实**：
  1. `docs/specs/0002-payment-settlement-admission.md:8-9` 已登记 `change_id: PAY-AC50-REVIEW-01`、`parent_change_id: PAY-SETTLE-01`；`:24` 重复该登记；`internal` 提交尾注使用 `change_id: PAY-SETTLE-01`（`git log`：`9e81497`、`d428dfd`、`4016c15`、`bda09a7`）。
  2. `docs/specs/0001-fulfillment-quantity-split.md:1-14` 没有 `change_id`；其历史工作项用 `AC-50-FIX-01`/`AC-27-NOWB-01`/`AC-27-LIVE-01`（`:847,872,879`）。
  3. 新 30（`.specify/memory/constitution.md:54`）规定扁平文件“取文件名主干”，即 `0002-payment-settlement-admission`、`0001-fulfillment-quantity-split`；文本没有“新 Spec 才适用”或“已登记沿用”的限定，“登记后不得更换”只挂在“两者都没有”分支。
  4. `docs/specs/README.md:3` 与 `:59-76`（关键句 `:70`） 明确扁平 Spec 是“一份业务能力一个文件、编号终身唯一、approved 后原地升版不新建文件”——一个 Spec 一生有多个 change_id，文件名不能等于 change 实例 id。
- **影响**：SPEC-0002/0001 在文本上各有两个及以上 change_id；今后新增 AC 的前缀与在途 `AC-39…72` 体系分叉；提交尾注/台账条目/审查任务书的 change_id 全部随之不确定。这是“Spec 身份”与“变更身份”被合并的结构问题，不是措辞瑕疵。
- **性质**：**本次 v2.1.0 引入**（v2.0.0 的 30 没有扁平文件分支）。
- **处置**：不属于审查者可自行修的范围（宪法/AGENTS 只能作者起草、人类批准）；本次 FAIL 退回第 1 步。

---

## 4. 非阻断问题（NB）

> 每条都给违反/依据；给不出的记 `[TODO]`。NB 不阻断本次，但应在下一次修订或后续契约变更中处置。

### NB-1 [A] `FR-<n>`/`SC-<n>` 的“来源”可被读成 AC 编号，且原生 Acceptance Scenarios 未要求逐条入验收表
- **依据**：宪法附录 A 验收状态表 `:86`（AC 编号用 `<change_id>-AC-<序号>`）、宪法 9（可溯源）。
- **事实**：兼容注 `:89` 把“稳定编号”与“引用 `FR/SC` 作为来源”放在同一句；`SC` 在模板 `:115-118` 是业务指标（2 分钟/90% 用户/降低 50% 工单），不是带拒绝语义的验收条件；模板 `:34-38` 的 `Acceptance Scenarios` 没有 AC 编号。
- **影响**：若读成编号，可绕开 change_id 前缀与“退役编号不得复用”；若读成来源，则“每条 AC”的判定仍由作者自定，10 条场景只登记 3 条 FR 也能过。非阻断，但这是 A 项“逃逸口”的主要残余。

### NB-2 [A] `MUST` 未纳入 `SHALL`/`THEN` 的形容词禁令
- **依据**：附录 A 行 1 `:80`（允许 `MUST`；禁令只点名 `SHALL`/`THEN`）。
- **影响**：“`System MUST respond quickly`”不在具体禁令的字面内；总约束“必须可判定”仍可兜底，但句式守卫出现缝隙。

### NB-3 [A/B] Spec Kit 生成目录与扁平文件布局未对齐
- **依据**：宪法 30 `:54` 的两种分支；`speckit-specify` SKILL `:80-106`、`:146`。
- **事实**：工具固定生成 `specs/<NNN>-<name>/spec.md` + `.specify/feature.json` + `checklists/`，不生成 `docs/specs/<NNNN>-<name>.md`；本仓没有 `specs/`、没有 `feature.json`。
- **影响**：兼容注只覆盖“章节”，没覆盖“产物落点/压平动作”；A 与 B 的路径假设不是同一个布局。

### NB-4 [D] 静态回退没有可判定的触发条件
- **依据**：AGENTS §1 `:8`、§3 `:58`；宪法 12/13。
- **事实**：条文只说“不能复跑时”声明，没有原因/范围/人类豁免/哪些 AC 不得标已验证的门槛；1-A 的目的（有运行时、可复跑）在资金/库存变更上可被无说明的静态声明抵消。宪法 8 的独立性不受影响（不同 actor 即可）。
- **影响**：复核强度可降级而无需记录；非阻断（作者证据义务与 9 条阻断义务仍在）。

### NB-5 [C] `checklists` 的 `[x]` 语义与 AGENTS §6 冲突
- **依据**：宪法 27 `:51`；AGENTS §6 `:88`；`speckit-implement` SKILL `:62-63`。
- **事实**：工具说自定义 checklist 是 reviewer-owned、`[x]` = reviewer 判定满足；AGENTS 说 checklist 只是作者自检。27 的“任何汇总文档”未点名 `checklists/**`。
- **影响**：出现第二个“勾选承载判定”的产物，且作者/审查者归属相反；非阻断，但 C 项“残留矛盾”的实项。

### NB-6 [E] `speckit-implement` 在本仓不可运行且无阶段参数
- **依据**：AGENTS §2 `:33`、§6 `:88`；`speckit-implement` SKILL `:58`、`:154-158`、`:224`；`common.sh:188-216`。
- **事实**：`check-prerequisites.sh --json --require-tasks --include-tasks` 实测 exit 1；无 `specs/`、无 `.specify/feature.json`、分支 `main`；工具无阶段范围参数，Done When 要求全部 `[X]`。
- **影响**：“分批调用”只能靠操作者提示 + 主动中止，不能作为工具模式验证；AGENTS §6 的“可以用 implement”对当前扁平布局不成立（SDD-STRUCT-ALIGN-01 未决）。非阻断。

### NB-7 [F] 同日条目/补记与“按标题日期判定新旧”不兼容
- **依据**：AGENTS §4 `:62`。
- **事实**：11-27 块内 30+ 条同日条目，日期无法排序；补记早于最新块的日期仍要插在“最新块顶部”；文件顶部实际是 11-24/25，“最新在上”在全局不成立。
- **影响**：插入点确定，阅读顺序仍有歧义；非阻断。

### NB-8 [G] 尾注范围（宪法 30 的“提交” vs AGENTS §4 的“合入提交”）
- **依据**：宪法 30 `:54`；AGENTS §4 `:74`。
- **事实**：作者提交 `9e81497`/`d428dfd`/`4016c15` 只有 `change_id`/`Actor`（无 `Reviewed-by`）；审查提交 `bda09a7` 写 `Reviewed-by: human (merge pending)`。新句把“补三个尾注”集中到合入提交，没有说明作者提交是否豁免，也没定义 `Actor` 是变更作者还是 git author。
- **影响**：严格按 30，历史作者提交不合规；按 AGENTS §4，30 的“提交”必须解释为合入提交。非阻断，但影响 `Actor != Reviewed-by` 的核对口径。

### NB-9 [G] 审查报告路径规则与本仓实际/本次任务冲突
- **依据**：AGENTS §5 `:80`；宪法 12（证据绑定）。
- **事实**：仓库无 `evidence/` 目录；在途证据在 `docs/implementation/**`；C-3 报告与本报告在 `docs/collab/qc-reports/`；扁平 Spec 的“同目录 evidence/”是共享的 `docs/specs/evidence/`，无命名/change_id 归属规则；治理变更无 Spec，路径未定义。
- **影响**：本次报告按任务书写在 `docs/collab/qc-reports/`，与 §5 字面不一致；多 change_id 共用目录会碰撞。非阻断。

### NB-10 [H] “延期”与“豁免”的边界未定义，延期清单缺 INV/逐台账/status/thread
- **依据**：宪法 18 `:36`、附录 A 行 8 `:87`、宪法 27 `:51`。
- **事实**：新行覆盖 owner/expiry/reason/到期阻断/不得批量续期；不要求“受影响的不变量”，不要求逐台账，无 `status`/`thread`。旧 `CONVENTIONS.md:201-203` 的延期行有 `thread`/`status`。
- **影响**：资金/库存相关 AC 可用“延期”格式记录而不触发 18 的 INV/逐台账要求；“到期未关闭”依赖验收状态表判断。非阻断（18 仍适用于“豁免”），但边界未定义就是可绕路径。

### NB-11 [H/L5] SPEC-0002 §15.3 的“附录 A 尚未定义延期清单”在 v2.1.0 后失效
- **依据**：附录 A 行 8 `:87`；`docs/specs/0002-payment-settlement-admission.md:1110`。
- **事实**：§15.3 原文写“附录 A 尚未定义延期清单格式与到期规则（见审查报告 NB-1），本表格式为 C-3 迁移用的临时约定”；v2.1.0 已定义该行，陈述过时。SPEC-0002 是已批准 Spec，本次治理变更未（也不应擅自）改它。
- **影响**：已批准契约与新宪法文本出现一处可核对的漂移；应在 SPEC-0002 的下一次修订或 C-3 后续中处理。非阻断。

### NB-12 [Governance] 最小增量与“写明可验证收益/停止条件”
- **依据**：宪法 Governance `:71`（“治理文本和工具链的变更必须从最小可行增量开始，写明可验证收益和停止条件”）。
- **事实**：本次 v2.1.0 一次打包 15 处改动（§1.2），覆盖语法、change_id、tasks、审查者、implement、台账、尾注、evidence、模板、延期清单；被审 diff 中没有任何“可验证收益/停止条件”的记录（DECISIONS 只记到 C-1）。
- **影响**：无法按 Governance 判断这是不是一个“最小可行增量”、收益出现没有、何时撤回。非阻断（人类指示本身可视为授权），但治理元规则未被遵守/记录。

### NB-13 [Governance] `2.1.0` 的版本语义可争
- **依据**：宪法 Governance `:72`（“删除或重定义**原则**为 MAJOR，新增原则或规则为 MINOR，澄清为 PATCH”）。
- **事实**：v2.1.0 删除了“AC 必须用 EARS”的强制（v2.0.0 附录 A 行 1），并新增延期清单/兼容注等规则。若 EARS 强制被视为“原则”或“硬约束”，删除它应为 MAJOR；若视为“规则”，MINOR 可辩护。Governance 没有定义“删除规则”的版本级别。
- **影响**：版本号与变更性质的对应关系不确定。`[TODO]`：该分类无法从现有文本唯一判定，需人类确认“EARS 强制是否属于原则”。

### NB-14 [Governance/AGENTS] “治理变更”不是宪法 3 / AGENTS §4 定义的一级
- **依据**：宪法 3 `:9-12`（只有契约/实现两级；“无法归入的，按契约变更”）；宪法 Governance `:69`（治理变更由作者起草、审查者审查、人类批准，不需要 Spec）；AGENTS §3 `:43`（“级别: 契约变更 | 实现变更”）与 §4 `:66`（同）。
- **事实**：本次任务书写“级别: 治理变更”，C-1 台账条目也写“级别: 治理变更（宪法 Governance；非契约/实现变更）”；AGENTS §3 的任务书没有“治理变更”值，也没有“Spec 和 ADR: 无”的模板；§4 的台账格式同样只有两级。
- **影响**：治理变更的级别/任务书/台账字段没有合法落点，只能由人类临时增补；这不是本次 v2.1.0 引入（v2.0.0 的 Governance 与 §3/§4 已如此），但本次就是治理变更，缺口直接暴露。非阻断；本报告头部与台账条目按任务书写“治理变更”并注明本 NB。

---

## 5. 本次调整没有解决的继承问题（只列，不改）

> 以下不是 v2.1.0 引入；不改变本次 B1 阻断。逐项给出核对结果与我的判断。
>
> **re-review 更新（2026-11-27）**：L1/L4/L5 已在作者修复与本次 L5 补行中解决（见 §0A）；L2/L3 仍然存在，**本次不阻断**，另行修订。本节保留第一次审查时的状态描述。

### L1 AGENTS §2“实现变更只做第 5-8 步”与“简单变更的 plan 可以很短，但不省略”

- **事实**：`AGENTS.md:30`“实现变更只做第 5-8 步”；`:35`“简单变更的 plan 可以很短，但不省略”。第 4 步是写 plan/tasks；实现变更跳过第 4 步。
- **判断**：如果“简单变更”指**简单的契约变更**（走 1-8），两句不矛盾；如果指**任何简单变更**（含实现变更），就与“只做 5-8 步”直接矛盾。宪法/AGENTS 没有定义“简单变更”，也没有定义实现变更是否需要 plan。按最自然读法，存在冲突；按窄读法，是术语未定义。属于**继承**（v2.0.0 文本；base v1.3.0 的 Plan 门禁只覆盖 P0/跨服务/资金/库存/权限/状态机）。建议人类在下一版明确“简单变更”的范围，或删掉其中一句（本报告不建议具体补丁）。

### L2 旧 CONVENTIONS §2.2 / §3 / §4.1 是否应迁入新模型

| 旧条款 | 新模型现状 | 判断 |
|---|---|---|
| §2.2 外部复审/审计输入只能作问题提案；引用必须指向当前 HEAD 文件/行；分流前核对既有 Spec/ADR/INV | 无对应；宪法 8/9 只覆盖“审查者”的独立与可溯源，不覆盖非审查者的外部审计输入 | **应迁入**。本次审查本身就是外部 subagent 的输入，若无此规则，外部结论可被作者直接写成 AC 依据。 |
| §3 变更输入包 / integration owner / baseline_impact / INV | 宪法 12 仍要求跨 Spec 非回归证据，但新模型没有 integration owner，也没有 `baseline_impact` 字段 | **至少应明确责任人**。新模型的作者（主会话）可以承担，但文本没写“谁组织组合验证”，宪法 12 的义务有、责任人无。 |
| §4.1 reviewer 元数据（review_scope/release_status/INV/baseline_impact/Actor/Reviewed-by） | AGENTS §3 只要求“写明范围”；§4 的台账格式没有 release_status/INV/baseline_impact 字段 | **应迁入**。宪法 12 要求非回归证据、9 要求可溯源；没有 `INV`/`baseline_impact` 字段，资金/库存变更的审查报告可以只写“范围”。本次任务书头部额外要求了 review_scope/release_status/INV，正是对 §4 缺字段的补丁式绕过。 |

### L3 v1.3.0 压缩后丢失的硬约束

| 旧宪法（`git show 9e81497:.specify/memory/constitution.md`） | v2.1.0 现状 | 判断 |
|---|---|---|
| II.5 permit/commitment：未完成退款→撤销/拒绝发货 permit；已离仓→保留发货事实只退款；Σ 等式；不得自行恢复 `REVOKED` permit；准入前拒绝/取消不创建 commitment；准入后失败→持久重试/人工队列，不得静默释放；commitment 释放须版本化 SPEC/ADR + 人类批准 | 宪法 23 只保留“同一组幂等准入协议，由 permit 裁决退款与发货竞态” | **丢失到不可接受**。这些正是资金链路的“不得就地修补”禁令；新 23 的概括不足以阻止后续实现用“释放 commitment/恢复 permit”解决卡单。 |
| III.2 事务核心必须 `database/sql` + 条件 `UPDATE`（CAS） | 宪法 20 保留“唯一约束或状态机 CAS”；26 保留“GORM 仅 catalog/cart”；35 禁新 ORM | **部分丢失，可接受**。GORM 限制 + 禁新 ORM 事实上把事务核心推到 `database/sql`；CAS 仍在 20。缺的是“显式 `database/sql`”这一句，不是核心语义。 |
| I.2 测试替身只能在 `_test.go`/build tag；必需依赖构造期必填、启动期 fail-fast；可选能力用显式 Options，禁止 nil 表示关闭；III.4 启动配置/依赖缺失 fail-fast + 非零退出；请求路径不得 panic | 宪法 15 只覆盖“资金/库存链路未预期异常、超时、配置缺失”；无测试替身、nil 依赖、启动失败、请求路径 panic 的条款 | **丢失到不可接受（其中测试替身/启动 fail-fast 最便宜、最可执行）**。15 的“配置缺失必须报错阻断”只覆盖资金/库存，且不区分“必需依赖”与“可选能力”。 |
| II.1 `order.paid` 兼容期：灰度双发只作 BFF/旧消费者兼容；不得作为履约准入/结算/库存就绪依据；收口后停止准入用途；必须写明默认消费者切换/收口条件/移除日期/回滚；不得以 flag 默认 false 长期保留；影子消费切换 | 宪法 23 只保留“四个事实不得由单一状态或 `order.paid` 合并替代”；宪法 6 保留“双写/版本化/影子消费” | **部分丢失**。事实分离保留，但“不得作为准入依据”“收口/移除/回滚”被删；在既有 PAY-SETTLE-01 兼容期仍在的仓库里，这是可执行的约束缺口。 |

### L4 “同一时间只推进一个 change_id”是否还需要

- **事实**：base `AGENTS.md` §1.0 明列“同一时间只推进一个 `change_id`；不得以 round/commit 数量作为进度”；v2.0.0/v2.1.0 删除了这句（新 AGENTS 只在 §2 的流程里隐含单作者）。现仅存于归档 `docs/archive/2026-11-27-governance/prompt/Lead.md:6` 与 `docs/lead/PAY-AC50-REVIEW-01/plan.md:22`。
- **判断**：新模型用“精确 base/head + 证据绑定”可以支持多个 change 并行；但单一工作树上并行多个 change 会造成 HEAD 漂移、证据串线（本次审查的 head 就不是 commit）。删掉这条没有给出替代的调度约束。**尚需要**，至少需要人类明确“并行是否允许、以什么为边界”。属于继承（v2.0.0 删除）。

### L5 SPEC-0002 §15.2 是否漏了 6 条 DEFERRED 基线 AC（AC-22/28/29/30/35/37）

- **事实**：`docs/specs/0002-payment-settlement-admission.md` §15.2 只有 6 行：AC-14/21/25/27/31/32（`:1101-1106`）；AC-22/28/29/30/35/37 只在 §15.3 延期清单（`:1114-1119`），标注“属 SPEC-0001，SPEC-0001 建表时迁回”。SPEC-0001 **没有**验收状态表（其 §15 是“裁决记录”，`:792`；全仓 `grep '验收状态表' docs/specs/*.md` 只命中 SPEC-0002）。
- **判断**：按宪法 27“验收状态的当前值只在 Spec 的验收状态表”，这 6 条 AC 的当前值（应为“未验证”）**没有任何验收状态表落点**；§15.3 不是状态表。§15.2 的标题限定“本次变更受影响的基线 AC”，6 条 DEFERRED 是否算“受影响”可以争，但“唯一当前值应在 SPEC-0001 的表”这句本身承认了缺口。属于**继承**（C-3 已标 MINOR，未阻断）；v2.1.0 新增延期清单使 §15.3 的格式被追认，但没有补上状态表落点，且 §15.3:1110 的“附录 A 尚未定义”已过时（NB-11）。**本次调整没有解决**。

---

## 6. 限制与 `[TODO]`

1. **静态审查**：治理文本，无代码可复跑；未运行 Go/MySQL/NATS/Redis/`-race`。已声明的工具行为实测见 §1.1 与 §7。
2. **head 不是 commit**：评审对象用字节 sha256 绑定（见头部）。若工作树在合入前再改，本裁决失效；合入前应重新取精确 commit 并复核。
3. `[TODO]`：人类裁决 1-A/2-B 的**原文**不在仓库，只能与 `DECISIONS.md` 转写比对；D 与 G 的“与人类裁决一致”是“与转写一致”，不是与原始决议一致。
4. `[TODO]`：v2.1.0 的**人类指示原文**不在仓库，无法核对本次 15 处改动是否都在指示范围内；本报告只能按文本判断。
5. `[TODO]`（NB-13）：`2.1.0` 是否应为 MAJOR，取决于“EARS 强制”是否属于宪法 Governance 的“原则”，现有文本不能唯一判定。
6. 未审：代码、SPEC-0001/0002 的 AC 语义、C-2 模板覆盖层落地、C-3/C-4/D-2 完成度、13 个 addendum、任何实现证据。未更新 `docs/collab/qc-reports/README.md` 索引（不在本次可改范围）。

---

## 7. 复现命令（只读）

```bash
git rev-parse HEAD                         # 9e81497...
git diff 9e81497 | shasum -a 256           # 21b4a9...
shasum -a 256 .specify/memory/constitution.md AGENTS.md
git diff --check 9e81497                   # exit 0
grep -n '^### 2026-11-27' docs/review-tracking.md | head   # 最新块从 :333 起
sed -n '1054,1125p' docs/specs/0002-payment-settlement-admission.md
grep -rn '验收状态表' docs/specs/*.md       # 只命中 SPEC-0002
git show 9e81497:.specify/memory/constitution.md | sed -n '1,140p'
bash .specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks   # exit 1
sed -n '642,650p;740p' .specify/scripts/bash/common.sh
```

**本次落盘**：本报告 `docs/collab/qc-reports/REV-GOV-SPEC-SYNTAX-01.md`（新文件）；`docs/review-tracking.md` 在 `:333` 前新增一条审查裁决。未改宪法、`AGENTS.md`、`docs/convert/**`、代码、tsv、归档；未 commit、未 merge。
