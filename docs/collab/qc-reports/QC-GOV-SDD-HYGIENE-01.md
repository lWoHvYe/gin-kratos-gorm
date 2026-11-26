# QC-GOV-SDD-HYGIENE-01 — P1 治理文本修订草案红队审查

> **结论性质**：本文件是 QC 红队报告。`READY` 只表示草案可进入人类批准/Reviewer 复核，**不构成人类批准、不构成 merge/Release、不替代 Reviewer verdict**。
> 本报告只读审查；未修改 `AGENTS.md`、`docs/collab/CONVENTIONS.md` 或任何其他文件，未 commit。

| 项 | 值 |
|---|---|
| change_id | `GOV-SDD-HYGIENE-01` |
| Tier | `L2`（治理文本轻路径；命中 `AGENTS.md:84` / 宪法 `:77`；无代码/契约/CI/`scripts/**` 改动） |
| base | `472e64fec33cbbf91592e2ffc74cbb31abc541db`（HEAD）；`AGENTS.md` blob `a1b962eec421`、`CONVENTIONS.md` blob `42fba2173d65` |
| head | 初始审查 `AGENTS.md` `e7759cda8999` / `CONVENTIONS.md` `1b16ced9032d`；中间 Delta `CONVENTIONS.md` `f922b32c57bd`；**最终（当前工作树，未提交）**：`AGENTS.md` `e7759cda8999`（未变）、`CONVENTIONS.md` `d465d501254e`；另有 Lead 的 `docs/review-tracking.md` 记录（`+23` 行，`:350-371`），不属两文件治理文本 diff |
| review_scope | 仅审上述两文件的工作树 diff（治理文本轻路径）。对照：宪法 v1.3.0、`prompt/Quality-Checker.md`、`docs/review-tracking.md`（2026-11-25 legacy cleanup / gate retirement / `66331aa` integrity 条目）、`docs/phase-legacy-cleanup-and-optimization-plan.md`、`docs/cost-reduction-proposal.md`、`docs/adr/0006-collab-gates.md`、三个 pair 存档、`docs/collab/ac-coverage.tsv`、`.specify` 脚本/模板/技能。**不审** PAY-SETTLE-01/ADR-0007/SPEC 契约正文与代码；纯文档变更，未运行测试，不构成 AC 验收或 Release 证据 |
| Actor | `qc-gov` |
| verdict | **`READY`** — **Final scope 最终：0 BLOCKER / 0 MAJOR；M3/D-1 N/A，D-2/M4/M5 CLOSED，open MINOR 5（M1/M2/M6/M7/M8）**。初始 8 MINOR 与中间 Delta（head `f922b32`；M3/M4/M5 CLOSED、D-1/D-2 新 MINOR、open 7）均为历史，见文末 `Final scope` |
| release_status | **`NOT RELEASE`** |

---

## 0. 审查对象、版本绑定与范围证据

### 0.1 变更范围（只改 2 个文件）

- `git diff --name-only` = `AGENTS.md`、`docs/collab/CONVENTIONS.md`；`git diff --cached --name-only` 为空；`git status --porcelain --untracked-files=all` 无其他文件/新增文件。
- `AGENTS.md`：2 行（`:15`、`:77`），4 增 4 删。
- `CONVENTIONS.md`：新增 §2.4（`:104-110`）；删除 §4 的“原协议摘要”块（`§4` 末、`:155` 与 `:157` 之间）；G-1 单元格（`:225`）；§7.2（`:234-236`）。

### 0.2 与 `66331aa` 的范围纪律一致性

`66331aa` 记录：五个 T0 范围外的未提交文件被保存到 `.tmp/unexpected-housekeeping-2026-11-26.patch` 后回退，以保持 T0 checkpoint 精确（`docs/review-tracking.md:366-379`）。本轮审查遵循同一纪律：只写本 QC 报告，只读其余文件；若提交，必须只包含 `allowed_paths` 内的两个文件。

### 0.3 被审草案原文的可考性

Architect 的草案正文未出现在工作树（`grep -rn "GOV-SDD\|REMOVE" docs/ .tmp/` 0 命中）；本报告按任务 brief 所列 5 项改动逐项核对**实际工作树 diff**，不依赖草案摘要。

---

## A. 宪法冲突：宪法 `:96` 与 `AGENTS.md:15`“pair 线程已退役，仅存档”

### 结论

**不构成 BLOCKER；属可接受的既有漂移（MINOR M1），且本草案使 `AGENTS.md` 与 `CONVENTIONS.md` 更同源，而不是新增冲突。** 宪法本身的措辞未在本草案中修改（不在 `AGENTS.md:84` 轻路径允许清单内），需另走 `/speckit-constitution`。

### 证据

- 宪法 `:96`：“……并在 `docs/review-tracking.md`（或对应 `docs/collab/<pair>/threads/`）记录 delta audit（日期、版本、差异摘要）……”。**主记录地是 review-tracking，threads 只是并列可选项**。
- `AGENTS.md:15`：“……`docs/review-tracking.md` 只承载结构化裁决与 canonical 指针（pair 线程已退役，仅存档）。”
- `CONVENTIONS.md:30`：协作线程“已退役 2026-11-25，只读存档”“不得新开或删除；历史指针保留”；`:153`：“不再新开线程”；`:155`：三个 pair 目录保留为存档，**不得删除**。
- 宪法 `:75`：“执行细则（角色矩阵、产物所有权、线程协议）以 `AGENTS.md` 与 `docs/collab/CONVENTIONS.md` 为准”——该条本身把线程协议委托给下层文件，故下层将其标为历史说明并不直接违宪。
- 宪法 `:79`：“线程 CLOSED、实现 PASS、代码合入、Release PASS 是四种不同状态”；`AGENTS.md:77` 改为“历史线程 CLOSED（仅存档）”，语义仍可对齐（都是“不得互相替代”）。

### 分析与判定

1. 宪法 `:96` 的 **primary path（review-tracking）完整可用**；threads 只是“or”分支。草案并未禁止把 delta audit 写入 review-tracking，因此不存在“无法合规记录”的死结。
2. 窄口径冲突客观存在：宪法允许把 delta audit 记入 threads，而 `CONVENTIONS.md:30` 禁止新开线程。但该冲突**先于本草案存在**：pair 退役为 2026-11-25（`CONVENTIONS.md:153`），宪法 v1.3.0（同日，`:109`）未同步 `:96`/`:75`；且 QC-33 已把同一问题登记为 MINOR-7。
3. 本草案把旧 `AGENTS.md:15` 的“线程只承载结构化问题、裁决和 canonical 指针”（把线程描述为现行通道）改为“已退役，仅存档”，**减少**了与 `CONVENTIONS.md` 的漂移，没有新增禁止性规则。
4. 线程路径没有被删除（`CONVENTIONS.md:155`），所以宪法 `:96` 的物理落点仍在；只是不再新开。

**判定：可接受的既有漂移（MINOR），不是 BLOCKER。**

### 最小修复建议（只描述，不执行）

- 未来单独走 `/speckit-constitution`（宪法 `:102`）做一次 PATCH：
  - `:96` 括注改为“`docs/review-tracking.md`（pair 线程仅存档，不作 delta audit 记录地）”；
  - `:75` 的“线程协议”改为“协作路由（线程协议为历史存档）”；
  - 可选：`:79` 的“线程 CLOSED”改为“历史线程 CLOSED”。
- 若本次不做宪法 PATCH：至少在本变更的 delta audit 中显式记“宪法 `:96`/`:75` 残留，已接受”，避免后续角色误以为 threads 仍是可写通道。**不要**用改 `CONVENTIONS.md` 的方式去覆盖宪法。

---

## B. 删除安全性：§4“原协议摘要”与 §7.2“执行器能力历史限制清单”

### 结论

**DELETE-SAFE。** 没有现行规则依赖被删内容；残留引用全部是**归档/已撤回文档的入站引用**，不是活引用。存在 1 项非阻断观察（M2：归档指针变旧、`archived: true` 无现行定义）。

grep 复核（排除 `.git`/`.tmp`/`node_modules`）：被删协议细节的**现行规则引用 0 命中**；`T-NNN`/`opened_by`/`ANSWERED` 只命中归档线程、归档索引与已退役 ADR；§7.2 债务清单只命中已撤回的 `cost-reduction-proposal.md` 与 2026-11-23 外部复审快照。详见下表。

### B.1 删除 §4“原协议摘要”

被删内容（见 `git diff`）：`docs/collab/<pair>/threads/T-NNN.md` 路径、一问一线程、编号递增不复用、`OPEN -> ANSWERED -> CLOSED`、YAML 字段 `id / title / opened_by / opened_at / blocking / status / spec / adr / closed_by`、结论回写 Spec/ADR。被删原文自称“只作历史参考，不具备现行效力”。

现行文本的自足性：

- §4 保留退役说明、目录不得删除、历史 36 行指针不回改（`CONVENTIONS.md:151-155`）；§4.1 保留（`:157-172`）。
- §5 的 `thread_status=CLOSED` 自带“历史通道/线程已退役”限定（`:182`），不依赖被删状态机定义。
- §6.1/§6.3 的 `thread` 列规则自带“历史 36 行保留原 pair 指针、新行写 `review-tracking/<change_id>`”（`:199`、`:211`），不依赖被删协议。

入站引用清点（全部为归档，不构成现行依赖）：

| 引用点 | 性质 | 判定 |
|---|---|---|
| `docs/collab/architect-developer/threads.md:5`、`docs/collab/po-architect/threads.md:6`：“协议见 `CONVENTIONS.md` §4（已退役，历史说明）” | 归档索引；§4 仍在，只是不再展开细节 | 归档引用，可接受 |
| `docs/collab/developer-reviewer/threads.md:6`：“协议见 …… §3” | 归档索引；**§3 是变更输入，不是线程协议**（QC-31 M7 已登记，未修） | 归档引用，非本次范围 |
| `docs/adr/0006-collab-gates.md:7/:23/:93/:257/:293`（§4/§4.2/§4.4） | ADR 已 `superseded / retired`（`:3`、`:9-19`），原文明确“只作设计历史，不再是执行契约”；`§4.2/§4.4` 更早已不在 CONVENTIONS v3 中 | 退役 ADR 的历史引用，不构成现行依赖 |
| 归档线程正文（如 `docs/collab/architect-developer/threads/T-001.md:75`“开 `T-NNN`”） | 历史正文，禁止改写（`CONVENTIONS.md:30/:155`） | 只能保留原样，不是现行通道 |

**唯一实质性观察**：`docs/collab/developer-reviewer/threads/T-002.md:11` 的 `archived: true` 字段在现行 `CONVENTIONS.md` 中没有定义（被删的 YAML 字段清单也不含 `archived`）。这是既有缺口（QC-33 MINOR-8 仍未关闭），删除 §4 摘要后更无就近说明；但它不产生现行行为/状态规则，**非阻断**。如需收口，在 §4 历史说明补一句“历史线程的 `archived: true` 表示随退役归档，不改变 `CLOSED=沟通闭环` 语义”。

### B.2 删除 §7.2 执行器能力历史限制清单

- 被删清单：仅实现 G-1/G-2/G-3、无 G-4/`--release`、输出 `AC coverage`、默认 `HEAD^..HEAD`、G-1 Role 并集只能提示、Makefile 未透传 `RANGE/RELEASE`、CI 未覆盖全部 docs/prompt。
- 所有**仍然有效**的事实都保留在 §7 头部与 G-1…G-4 人工清单：
  - `CONVENTIONS.md:215-217`：脚本/CI/Makefile 目标已删、`make verify` 不再运行 gates、**没有替代工具**、不得要求运行已删命令、不得宣称“已通过机器门禁”。
  - `CONVENTIONS.md:221-230`：G-1…G-4 只是人工清单，“原机器检查范围”只记录历史。
  - 旁证：`AGENTS.md:127`、`prompt/Reviewer.md:3`、`docs/review-tracking.md:293-299`、`docs/adr/0006-collab-gates.md:13-16`。
- 对被删“债务清单”的入站引用，均为已撤回/归档文档：
  - `docs/cost-reduction-proposal.md:20/:36/:43/:64-65`（该文件 `:3` 明写“已撤回/归档”，不再驱动实现）；
  - `docs/ext-review-1123.md:24/:240`、`docs/ext-review-1123-triage.md:129/:134/:160`（2026-11-23 外部复审快照，按 `CONVENTIONS.md:72-76` 只是问题提案/描述性输入）。
- 没有任何现行规范条款要求引用 §7.2 的某条限制作为证据或前置条件。

### 判定

- **无活引用、无悬空规范性依赖、无规范性信息丢失**；删除后 §7 的“目标规则不得被描述成已通过机器能力”仍完整保留（`:236`）。
- M2（MINOR，非阻断）：归档文档中的 §7.2/§4 指针会变旧；无需本次修。若追求整洁，可在归档文件头部加“§7.2 债务清单已随退役删除”的注释，但那是其他 owner 的产物，不能由本变更顺手改。

---

## C. 新增 §2.4“Spec Kit 路径边界”

### 结论

**基本正确、位置合理、未新增机器门禁/状态字段/第二套路径规则。** 存在 1 项非阻断缺口（M3）：`SPECIFY_FEATURE_DIRECTORY` 真实存在，但草案未写它的两个副作用/约束（持久化到 `.specify/feature.json`；同一变量同时服务 specify/plan/tasks，无法同时指向 `docs/specs/` 与 `docs/lead/`）。

### C.1 与上位规则逐条对照

| 对照点 | 结论 | 证据 |
|---|---|---|
| 宪法 `:102`（修订 MUST 走 `/speckit-constitution`） | **一致**：§2.4 明写“`/speckit-constitution` 仍是宪法 `:102` 规定的修订路径，不在此豁免” | `CONVENTIONS.md:110` |
| AGENTS §2.3（阶梯门禁：新 Spec/ADR 须 QC + 人类批准） | **不冲突**：§2.4 只规定“vendor 默认不是 canonical 写作路径”，未改 QC/人类批准链 | `AGENTS.md:76`；`CONVENTIONS.md:104-110` |
| `CONVENTIONS.md` §1 所有权表 | **一致**：`docs/specs/**` = PO、`docs/lead/**` = Lead；§2.4 的路径与 owner 与表相同 | `CONVENTIONS.md:20-34`；新 `:108` |
| §7 门禁退役声明 | **一致**：§2.4 末尾“不新增机器门禁”；§7 明写无替代工具、不得新增 G 系列 | `CONVENTIONS.md:110`；`:215-217`、`:230` |
| 新增机器门禁/状态字段 | **无**：没有脚本、CI、状态字段、台账列 | `git diff` 仅 `.md`；`CONVENTIONS.md:102` 范围守卫仍在 |
| 第二套路径规则 | **无**：`docs/specs/NNNN-*.md`、`docs/lead/<change_id>/` 已是 §1/既有实践；§2.4 反而明确 `specs/` 是“非规范产物路径”，是在**消除**第二套路径 | `CONVENTIONS.md:22-23`、`:30-34`；新 `:106` |
| 位置（CONVENTIONS 而非 AGENTS） | **合理**：宪法 `:75` 把执行细则委托给 AGENTS/CONVENTIONS；这是 vendor 工具的路径执行细则，不是宏观角色/质量规则。AGENTS 放宏观战略，CONVENTIONS 放执行细则；同文件还能与 §1 所有权表交叉引用 | 宪法 `:75`；`AGENTS.md:1-3` |

### C.2 `SPECIFY_FEATURE_DIRECTORY` 是否真实、语义是否正确

**真实存在，语义基本正确：**

- `.specify/scripts/bash/common.sh:187-200`：解析优先级 1) `SPECIFY_FEATURE_DIRECTORY` 环境变量（显式 override）→ 2) `.specify/feature.json` → 3) 报错；相对路径按 repo root 归一化为绝对路径；设置后会 `_persist_feature_json`（除非 no-persist）。
- `.dsh/skills/speckit-specify/SKILL.md:82-91`：默认 `specs/`；用户显式提供 `SPECIFY_FEATURE_DIRECTORY` 时“use it as-is”。
- `.dsh/skills/speckit-constitution/SKILL.md:127`：写回 `.specify/memory/constitution.md`；`:146`：不创建/修改模板源文件——与 §2.4 “不修改 vendor 模板”一致。
- `.specify/templates/**` 与 `/speckit-*` 确为 vendor 初始化产物（`.specify/init-options.json`：`speckit_version: 1.4.3`、`ai: dsh`；模板与 9 个技能均被 git 跟踪）。

**草案的表述缺口（M3，非阻断）：**

1. **持久化**：显式设置 `SPECIFY_FEATURE_DIRECTORY` 会写入 `.specify/feature.json`（`common.sh:196-200`），即使该目录是临时目录，也会留下机器本地指针；`.specify/.gitignore:5-7` 已忽略 `feature.json`，所以不是仓库产物，但会影响后续会话。草稿只写“不得留下 `specs/` 副本”，未写“配合 `SPECIFY_FEATURE_NO_PERSIST=1`/`--no-persist` 运行”（该开关真实存在：`common.sh:172-178`）。
2. **一变量服务三条命令**：`/speckit-plan`、`/speckit-tasks` 通过 `setup-plan.sh`/`setup-tasks.sh` 复用同一 resolver（`.dsh/skills/speckit-plan/SKILL.md:58`、`speckit-tasks/SKILL.md:58`）。若把 `SPECIFY_FEATURE_DIRECTORY` 设为 `docs/specs/NNNN-*`，`plan.md`/`tasks.md` 会被写进 PO 的 spec 目录，**与 §2.4 自己的“Plan/Tasks 走 `docs/lead/<change_id>/`”冲突**。草稿的“由 owner 回写 canonical”只解决了最终落点，没有说明三条命令应使用一次性目录/分别回写。
3. **审批链未复述**：§2.4 未明写“运行命令不改变 QC/人类批准/Reviewer 复核”。虽然上位规则仍在，但治理文本读者可能把“写回 canonical”误解为“可直接成为规范”。建议加一句“不改变 §2 批准点”。

### 判定

- **无 BLOCKER/MAJOR**；§2.4 的方向正确，且与宪法 `:102` 有显式衔接。
- **M3（MINOR，建议提交前补 1 句）**：补“`SPECIFY_FEATURE_NO_PERSIST=1`/一次性目录；plan/tasks 不得写入 `docs/specs/`；不改变 §2 批准点”。

---

## D. G-1 改动：“change input / change_id 是否存在”

### 结论

**方向正确（删除已退役线程证据要求），但新措辞有 2 个非阻断问题（M4）：语义含混、与同单元格后文的 `change_id` 检查重复；不会强推 Reviewer 索取新文件，但仍有被误读为“存在某份 change input 文件”的风险。**

### 证据

- 新单元格（`CONVENTIONS.md:225`）：“受管路径、Role/Exception 是否存在、change input / `change_id` 是否存在、HEAD 的 `change_id`/`Actor`/`Reviewed-by` 是否存在且 `Actor != Reviewed-by`”。
- §3 的 change input 是**字段集合**，不是单文件：“随 commit/PR 记录，不新增文件格式”（`CONVENTIONS.md:100`）；字段清单见 `:114-134`（`change_id`、`Tier`/`Tier-Rationale`、base、`spec_version`/`adr_revision`、`integration_owner`、`baseline_impact`、`affected_AC`、`allowed_paths`、`forbidden_paths`、`stop_conditions`、`evidence_and_output`）。
- `AGENTS.md:86` 与 `CONVENTIONS.md:242` 也把 `change_id`/`Tier`/`Actor`/`Reviewed-by` 作为 commit trailer 的受管字段。

### 分析

1. **是否重复 §3**：G-1 是“人工核对要点”，引用 §3 的字段并不构成第二套 schema；它没有重新定义字段。但当前写法没有引用 §3，读者不知道“change input”指哪份对象。
2. **是否可人工核对**：`change_id` 可以（trailer、review-tracking、Plan 路径）。`change input` 作为“文件是否存在”不可核对，因为设计上就不新增文件格式（`CONVENTIONS.md:100`）；作为“§3 字段是否齐备”可核对。
3. **是否会让 Reviewer 重新要求不存在的证据**：有中等风险。若 Reviewer 把斜杠读成“存在一份 change input 文档”，会复现旧的“线程是否存在”式空证据要求；但 §3 已给出字段口径，且 `change_id` 在单元格后文还会再查一次，熟练 Reviewer 不会真的索要文件。
4. **冗余**：同单元格后文已要求“HEAD 的 `change_id`……是否存在”，前半句再加 `change_id` 属重复。

### 判定

- **M4（MINOR，非阻断）**。建议改为：“§3 变更输入字段是否齐备（`change_id`/`Tier`/base/head/`allowed_paths`，commit/PR 可核对）”，并删掉后半句重复的 `change_id` 检查（保留 `Actor`/`Reviewed-by` 检查）。
- 这不是 §3 的重复定义，也不是新 schema；只是措辞/可核对性问题。

---

## E. 范围与完整性

### 结论

**范围正确：只改 2 个文件，无代码/契约/CI/`scripts/**`/迁移。** 仍存在 2 处值得本次处理的活引用（M5、M6），其余为归档/他人产物，不需要本次修。

### E.1 范围证据

- `git diff --name-only` = `AGENTS.md`、`docs/collab/CONVENTIONS.md`（见 §0.1）。
- 未触碰：`internal/**`、`cmd/**`、`api/**`、`migrations/**`、`scripts/**`、`.github/**`、`Makefile`、`docs/specs/**`、`docs/adr/**`、`ac-coverage.tsv`。
- 与 `AGENTS.md:84` / 宪法 `:77` 的治理文本轻路径范围一致（详见 G）。

### E.2 “把退役线程当现行通道”的剩余引用清单

| # | 位置 | 内容 | 性质/判定 | 是否本次修 |
|---|---|---|---|---|
| 1 | `CONVENTIONS.md:225` | G-1 仍写“Role/**Exception** 是否存在” | `Exception:` 的唯一现行定义在已退役 ADR-0006 `:75-79/:135`，要求 `Exception: T-NNN` 且线程必须存在；线程已不能新开（`:30/:153`）。这是同一行里**未清干净的线程依赖** | **建议本次修**（M5）：改为“Role（及既有例外记录）是否存在”或直接删“/Exception” |
| 2 | `docs/collab/ac-coverage.tsv:38-43` | 6 行（AC-14/21/25/27/31/32）的 `thread` = `developer-reviewer/T-003` | 该文件不存在（`git log --all -- docs/collab/developer-reviewer/threads/T-003.md` 0 命中；目录仅 T-001/T-002）。不是“被删除”，是台账指向了从未存在的线程 | **Reviewer 修**（M6，`[TODO]`，不在本草案 2 文件范围） |
| 3 | `docs/review-tracking.md:7` | “……`tasks.md`、`plan.md`、`handoff.md`、**协作线程均为视图**……” | 线程已退役/仅存档；此处仍称其为“视图”，属措辞残留。不宣称线程是活通道，只声明冲突优先级 | 可不修（Lead-owned，`[TODO]` 可选） |
| 4 | `docs/collab/developer-reviewer/threads.md:6` | “协议见 `../CONVENTIONS.md` §3” | 归档索引指错节（§3 是变更输入；§4 才是退役说明）。QC-31 M7 已登记，仍未修 | 归档内容，非本次范围（可选修） |
| 5 | `docs/collab/architect-developer/threads/T-001.md:75` | “遇到 ADR 未定义的异常，开 `T-NNN`（本目录）……” | 历史线程正文；`CONVENTIONS.md:30/:155` 要求保留原样、不回改 | **不得修** |
| 6 | `docs/adr/0006-collab-gates.md:75-79/:135/:147` | `Exception: T-NNN` 必须存在 `docs/collab/*/threads/T-NNN.md`，线程 CLOSED 则 strict FAIL | ADR 已 `superseded / retired`（`:3`、`:9-19`），原文自称只作设计历史；不是现行规则 | 不修（ADR 修订不属轻路径；状态已正确标 retired） |

### E.3 判断

- 只有 **#1（Exception）** 与 **#2（T-003 悬空指针）** 是实质性的；#1 在同一被编辑行、零成本、建议本次修；#2 的 owner 是 Reviewer，且 `ac-coverage.tsv` 不在 `allowed_paths`，只能 `[TODO]` 交 Reviewer。
- #3–#6 均为归档/Lead-owned/ADR 历史，不构成本次阻断；若强行在本变更中改，会超出“只改 2 文件”的范围与 K-7 的 L0 companion 边界。

---

## F. 留档删除：独立复核 Architect “REMOVE: 0”

### 结论

**`REMOVE: 0` 成立；`[REMOVABLE]` = 无。** 四个候选都不是“只留档、无引用”的可删除项；其中线程目录还承担 36 行 AC 延期指针，成本方案被台账显式保留为历史，清理方案仍有未完成的 Batch C，ADR-0006 是被多处的历史引用锚点。

> 说明：Architect 的草案原文不在工作树（见 §0.3），本结论按仓库事实独立复核。

| 候选 | 现状/入站引用 | 删除风险 | 判定 |
|---|---|---|---|
| `docs/cost-reduction-proposal.md` | 自称“已撤回/归档”（`:3`）；`docs/review-tracking.md:303-304` 明写“**仅保留**本决策记录与 `docs/cost-reduction-proposal.md` 作为历史”；仓内无其他规范性入站链接（仅 review-tracking `:229` 的历史提及） | 删除会抹掉 GOV-COST-01 撤回的唯一完整背景；且属他人产物，`AGENTS.md:87` 明令“他人产物不得删除” | **NOT REMOVABLE** |
| `docs/phase-legacy-cleanup-and-optimization-plan.md` | 仍是**执行中**方案：Batch C 未完成（`:79-81`，`:99`“12-08 前做 C-1.1”）；含人类裁决 D-LC-1…D-LC-5（`:85-93`）；`review-tracking.md:632-664` 按 Batch A/B/C 与 D-LC 编号引用其决策 | 删除会丢失 pending 任务、owner 与裁决 provenance；且属他人产物 | **NOT REMOVABLE** |
| `docs/collab/*/threads/**`（3 目录 / 7 文件） | `ac-coverage.tsv:2-37` 的 **36 行历史 deferral** 全部指向 `developer-reviewer/T-001`（6）与 `T-002`（30）；`CONVENTIONS.md:155` 明写“**不得删除**”；宪法 `:96` 保留该路径；归档索引逐文件链接 | 删除会造成 36 行台账悬空，违反 `CONVENTIONS.md:155` 与 `AGENTS.md:87` | **NOT REMOVABLE** |
| `docs/adr/0006-collab-gates.md` | 状态 `superseded / retired`（`:3`）；被 `docs/adr/0007-payment-settlement-admission.md:23`、`docs/architect/MEMORY.md:39`、`po-architect/threads.md:11-12`、`architect-developer/threads/T-001.md:9/:20`、`ext-review-1123-triage.md:64` 引用；其 Retirement 节 `:9-19` 记录闸门退役 | ADR 是设计历史/审计链，删除会造成多处悬空；且属他人产物 | **NOT REMOVABLE** |

### F.1 额外核对：线程目录的引用比“36 行”更多，且出现 1 处悬空

- `ac-coverage.tsv` 现有 **42 数据行**（`wc -l` = 43，含表头）：
  - `:2-37` = **36 行历史 deferral**，thread 分布 `developer-reviewer/T-001` ×6 + `T-002` ×30，与 `CONVENTIONS.md:155` 的“历史 36 行”一致；
  - `:38-43` = 6 行 AC-VERIFIED/AC-REF（AC-14/21/25/27/31/32），thread 均为 `developer-reviewer/T-003`，**该文件不存在**（`ls docs/collab/developer-reviewer/threads/` 仅 T-001/T-002；`git log --all -- .../T-003.md` 0 命中）。
- 结论：`REMOVE: 0` 不受影响（反而说明线程目录不能删）；但 `:38-43` 的 `developer-reviewer/T-003` 是**台账悬空指针**（M6，`[TODO]` 交 Reviewer）。它不是“被删除的线程”，是从未存在的线程，故不构成 `CONVENTIONS.md:155` 的删除违规。

### F.2 是否有 [REMOVABLE]

**无。** 按“无引用 + 无 pending 工作 + 非他人产物 + 不违反 `CONVENTIONS.md:155`/`AGENTS.md:87`”四条同时满足才可列 `[REMOVABLE]`；四个候选均至少违反一条。

---

## G. 轻路径合规：AGENTS `:84` / 宪法 `:77`

### 结论

**合规：治理文本类 L2 轻路径成立；未建 Spec/ADR/Plan，未改代码/契约/CI/`scripts/**`。** 但“停止条件与可验证收益”尚未写入任何 canonical/变更输入（M7），delta audit 与 change input trailer 也尚未落（M8）——均为 MINOR/`[TODO]`，不阻断 READY。

### G.1 逐条核对

| 轻路径要求 | 结果 | 证据 |
|---|---|---|
| 仅治理文本（`AGENTS.md`/`CONVENTIONS.md`/MEMORY/`prompt/**`/台账格式） | **满足**：只改 `AGENTS.md`、`CONVENTIONS.md` | `git diff --name-only`；`AGENTS.md:84`；宪法 `:77` |
| 不改代码/契约/CI/`scripts/**` | **满足**：无 `internal/**`、`api/**`、`migrations/**`、`.github/**`、`scripts/**` 改动 | `git status`；§0.1 |
| 不建 Spec/ADR/Plan | **满足**：工作树无新增 Spec/ADR/Plan 文件 | `git status --untracked-files=all` |
| 流程：Architect 起草 → QC → 人类批准 → Reviewer 复核 | **进行中**：本报告是 QC；人类批准与 Reviewer 复核尚未发生 | `AGENTS.md:84`；本报告头部 |
| 人类 Release / 不越级 | **保留**：本报告 `NOT RELEASE`；未声称批准 | 宪法 `:77`；`CONVENTIONS.md:98` |

### G.2 停止条件与可验证收益（未写明 → M7）

- `AGENTS.md:87`：“治理/工具链变更 MUST 从最小可行增量开始，**写明停止条件与可验证收益**”；“收益与停止条件 MUST 记录在 `docs/review-tracking.md` 或变更输入。”
- 现状：两文件 diff 中没有“收益/停止条件”段；`docs/review-tracking.md` 无 `GOV-SDD-HYGIENE-01` 条目（`grep -rn "GOV-SDD" . --exclude-dir=.git ...` 0 命中）；变更尚未提交，故也没有 commit/PR 变更输入。
- **判定：MINOR M7（`[TODO]`，人类批准前补齐）**。建议的最小内容：
  - **可验证收益**：消除 §2.4 的 vendor 路径歧义；删除 §4/§7.2 的退役残留；把 G-1 从“线程是否存在”改为可核对字段——三项均可用 grep（无活引用/无死命令/无 `specs/` 副本）复核。
  - **停止条件**：若发现新的活引用、或需要改宪法/ADR/脚本/CI 才能闭环，则停止并拆分独立变更；首增量只允许本次 2 文件；若收益无法用 grep/人工核对验证，则回退而非继续加层。
- 参考先例：`docs/review-tracking.md:310-317` 的 2026-11-25 delta audit 已按“收益/停止条件”格式记录，可直接沿用。

### G.3 delta audit 与 change input（未落 → M8）

- 宪法 `:96` / `AGENTS.md:27`：治理文件变更后，Lead 与所有 subagent MUST 重新读取，并在 `review-tracking.md` 记录 delta audit（日期、版本、差异摘要）。
- 现状：`review-tracking.md` 无本条变更记录；提交后需由 Lead 补记（不是 Architect/QC 的产物）。
- 变更输入（`AGENTS.md:86`）：`change_id`/`Tier`/`Tier-Rationale`/`allowed_paths`/base/head/`baseline_impact`/测试证据/Reviewer verdict/人类批准人与日期，应写入 commit trailer/PR；当前仅 brief 有 `change_id`/`Tier`。
- **判定：MINOR M8（`[TODO]`，不阻断 READY；提交/合入前必须补）**。

---

## 分级汇总

| 级别 | 数量 | 明细 |
|---|---|---|
| **BLOCKER** | **0** | — |
| **MAJOR** | **0** | — |
| **MINOR** | **8** | M1 宪法 `:96`/`:75` 残留（既有漂移）；M2 §4/§7.2 归档入站引用变旧（无需修）；M3 §2.4 `SPECIFY_FEATURE_DIRECTORY` 副作用未写；M4 G-1 `change input / change_id` 含混/重复；M5 G-1 `Role/Exception` 仍是线程依赖；M6 `ac-coverage.tsv:38-43` 指向不存在的 `developer-reviewer/T-003`；M7 收益/停止条件未写；M8 delta audit/change input 未落 |
| `[TODO]`（不影响 READY） | M2、M3、M6、M7、M8 | M1 属未来宪法 PATCH；M4/M5 建议提交前顺手修 |

### 逐项 ID

| ID | 级别 | 位置 | 一句话 | Owner | 阻断? |
|---|---|---|---|---|---|
| M1 | MINOR | 宪法 `:96`/`:75` vs `AGENTS.md:15`/`CONVENTIONS.md:30` | 可接受的既有漂移；本草案减少而非新增漂移 | Architect（未来 `/speckit-constitution` PATCH） | 否 |
| M2 | MINOR | `CONVENTIONS.md` §4/§7.2 删除处 | 归档文档（cost proposal/ext-review/pair 索引）指针变旧；无现行依赖 | 无需本次修（可选归档注记） | 否 |
| M3 | MINOR | `CONVENTIONS.md:109` | 未写 `feature.json` 持久化与“一变量服务 specify/plan/tasks”的约束 | Architect（提交前 1 句） | 否 |
| M4 | MINOR | `CONVENTIONS.md:225` | “change input / `change_id`”含混且与后文重复 | Architect（提交前） | 否 |
| M5 | MINOR | `CONVENTIONS.md:225` | “Role/Exception”中的 `Exception` 仍依赖已退役的 `T-NNN` 线程 | Architect（本次同修最佳） | 否 |
| M6 | MINOR | `ac-coverage.tsv:38-43` | 6 行指向不存在的 `developer-reviewer/T-003` | Reviewer | 否 |
| M7 | MINOR | 两文件/变更输入 | 收益与停止条件未写明（`AGENTS.md:87`） | Lead/Architect | 否 |
| M8 | MINOR | `review-tracking.md`/commit | delta audit 与 change input trailer 未落 | Lead | 否 |

---

## 最小修复清单（只描述，不执行）

1. **`CONVENTIONS.md:225`（最高性价比，同时解 M4/M5）**：把
   “受管路径、Role/Exception 是否存在、change input / `change_id` 是否存在、HEAD 的 `change_id`/`Actor`/`Reviewed-by` 是否存在且 `Actor != Reviewed-by`”
   改为
   “受管路径、Role 是否存在、§3 变更输入字段是否齐备（`change_id`/`Tier`/base/head/`allowed_paths`，commit/PR 可核对）、HEAD 的 `Actor`/`Reviewed-by` 是否存在且 `Actor != Reviewed-by`”。
   若保留“Exception”，必须同时说明它是历史例外记录、不得再要求新建 `T-NNN` 线程。
2. **`CONVENTIONS.md:109`（M3）**：补一句“运行 `/speckit-*` 时用一次性特征目录并配合 `SPECIFY_FEATURE_NO_PERSIST=1`/`--no-persist`，避免 `.specify/feature.json` 残留；`plan.md`/`tasks.md` 不得落在 `docs/specs/`；不改变 §2 的 QC/人类批准/Reviewer 复核”。
3. **`CONVENTIONS.md:155`（M2 的可选收口）**：在 §4 历史说明加一句“历史线程的 `archived: true` 表示随退役归档，不改变 `CLOSED=沟通闭环`；原协议字段见各线程 front-matter/正文”，同时关闭 QC-33 MINOR-8 的残留。
4. **变更输入 + delta audit（M7/M8，Lead）**：提交时写 `change_id: GOV-SDD-HYGIENE-01`、`Tier: L2`、`base 472e64f`、`head`、`allowed_paths: AGENTS.md,docs/collab/CONVENTIONS.md`、`Tier-Rationale`、`Reviewer`/人类批准字段；在 `review-tracking.md` 补 delta audit（日期、版本、差异摘要）与收益/停止条件。
5. **`ac-coverage.tsv:38-43`（M6，Reviewer `[TODO]`）**：这 6 行不是历史 deferral，不应指向不存在的 `developer-reviewer/T-003`；改为 `review-tracking/PAY-SETTLE-01`（或对应 change_id）并保留原证据列。**不要**改 `:2-37` 的 36 行历史指针。
6. **宪法 `:96`/`:75`（M1，独立变更）**：未来经 `/speckit-constitution` 做 PATCH，不在本轻路径内；本变更只需在 delta audit 记录“接受残留”。
7. **可选、非本次必需**：`docs/collab/developer-reviewer/threads.md:6` 的 §3→§4（归档指针）；`docs/review-tracking.md:7` 的“协作线程均为视图”措辞（Lead-owned）。

### 非阻断建议（READY，最多 3 条）

1. **同修 G-1 单元格**：一次编辑同时消除 M4/M5——把“Role/Exception 是否存在、change input / `change_id` 是否存在”改为“Role 是否存在、§3 变更输入字段是否齐备”，不保留无法新开的 `T-NNN` 例外口径。
2. **§2.4 补 1 句运行约束**：`SPECIFY_FEATURE_NO_PERSIST=1`/一次性目录；`plan.md`/`tasks.md` 不得写入 `docs/specs/`；不改变 §2 的 QC/人类批准/Reviewer 复核（M3）。
3. **人类批准前补记录**：在 change input/commit trailer 与 `docs/review-tracking.md` 写明可验证收益、停止条件与 delta audit（M7/M8）。

---

## 明确声明

- **本报告不构成人类批准**：`READY` 仅是 QC 对草案的意见，人类批准点仍然存在（`AGENTS.md:84`、宪法 `:77`）。
- **本报告不构成 merge/Release**：`release_status = NOT RELEASE`；未产生 AC-VERIFIED，未运行测试。
- **本报告不替代 Reviewer verdict**：Reviewer 仍需按精确 head、`review_scope` 独立复核并确认 `Tier`（`AGENTS.md:59/:63`、宪法 `:94`）。
- **未修改任何文件/未 commit**：本报告是唯一写入；`AGENTS.md`/`CONVENTIONS.md` 及其他文件保持审查前状态。
- **版本绑定**：本报告只对 §0 所列 blob 有效；若提交产生新 head，必须重新绑定并复核 diff。

---

## Delta review (2026-11-26)

> 本节是**中间 delta**（head `f922b32c57bd`，§2.4 仍在）。人类随后决定删除 §2.4；**最终结论见文末 `Final scope after human decision (2026-11-26)`**，本节不再作为最终 verdict；上方 §0–§G 保留首轮审查绑定（`CONVENTIONS.md` blob `1b16ced9032d`）。

| 项 | 值 |
|---|---|
| 触发 | Architect 按 QC M3/M4/M5 修订：G-1 去 Exception、§2.4 补 NO_PERSIST/feature.json、M7/M8 由 Lead 记录 |
| base | `472e64fec33cbbf91592e2ffc74cbb31abc541db`（HEAD 未变） |
| new head | **当前工作树（未提交）**：`AGENTS.md` blob `e7759cda8999`（未变）、`CONVENTIONS.md` blob `f922b32c57bd`（首轮 `1b16ced9032d`） |
| 复核范围 | `docs/collab/CONVENTIONS.md:110`（§2.4 新句）、`:226`（G-1）；`AGENTS.md` 未变；`git diff --name-only` 仍仅两文件；`docs/review-tracking.md` 未变；无 commit |
| 方法 | 只读读取 + grep；未运行测试；未改 `AGENTS.md`/`CONVENTIONS.md`；未 commit |
| 中间 verdict（head `f922b32`） | **`READY`** — 0 BLOCKER / 0 MAJOR；M3/M4/M5 CLOSED；D-1/D-2 新 MINOR；open MINOR 7（历史，最终见文末） |
| release_status | **`NOT RELEASE`** |

### DR.1 M4：G-1 的 change input / change_id

**判定：CLOSED（主体）。**

- 新 G-1（`CONVENTIONS.md:226`）：“受管路径、Role 是否存在、§3 变更输入字段是否齐备（`change_id`/`Tier`/base/head/`allowed_paths`，commit/PR 可核对）、HEAD 的 `Actor`/`Reviewed-by` 是否存在且 `Actor != Reviewed-by`”。
- 不再出现“change input / `change_id`”这一“存在某份文档”的口径；§3 是字段集合而非文件格式（`CONVENTIONS.md:100`、`:115-135`）。
- 不再在同一单元格重复查 `change_id`：后文只查 `Actor`/`Reviewed-by`，`change_id` 由 §3 字段清单覆盖。
- **残余 D-2（新 MINOR，非阻断）**：`§3 变更输入字段是否齐备` 未限定 Tier；括号只列 5 个字段，而 §3 对 L2 另需 `spec_version`/`adr_revision`（`:139`）、`baseline_impact`/INV（`:141-146`）、`affected_AC`/`forbidden_paths`/`stop_conditions`/`evidence_and_output`（`:121-133`）；L0/L1 又不要求全部 §3 字段（`:84-86`、`:139`）。可能被读成“L0/L1 也要全量”或“L2 只查这 5 个”。建议改为“§3 中按 Tier 适用字段是否齐备（至少 `change_id`/`Tier`/base/head/`allowed_paths`；L2 另见 §3）”。

### DR.2 M5：删除 Exception 后的悬空引用

**判定：CLOSED。**

- `CONVENTIONS.md:226` 已无 `Exception`；现行 `AGENTS.md`/`CONVENTIONS.md`/`prompt/**`/宪法对 `Exception` 的 grep = **0 命中**。
- 现行规范文本（`AGENTS.md`/`CONVENTIONS.md`/`prompt/**`/宪法）外，`Exception` 仅出现在：已退役 `docs/adr/0006-collab-gates.md`（`:3` retired、`:11` 只作设计历史；`:75-79`/`:92`/`:135`/`:147` 历史设计）与归档线程（`docs/collab/po-architect/threads/T-002.md:23/:50-54`、`docs/collab/architect-developer/threads/T-001.md:63`）；均为历史/只读，不构成现行要求。
- `T-NNN` 在现行规范文本仅 `CONVENTIONS.md:200` 的“历史 36 行 `pair/T-NNN` 指针”（有意保留、不回改）；`docs/reviewer/MEMORY.md:14/:73`、`docs/architect/MEMORY.md:46` 也是历史指针，`docs/collab/*/README.md`/`threads.md`/归档线程为归档。**没有丢失现行要求**：`CONVENTIONS.md:243` 的 trailer 清单只有 `change_id`/`Actor`/`Reviewed-by`/`Tier`/`Role`，本来就不含 `Exception`。
- 结论：删除 `Exception` 不产生悬空现行引用；`Role` 仍有现行定义（`CONVENTIONS.md:243`），故 G-1 的“Role 是否存在”可核对。

### DR.3 M3：NO_PERSIST / 共用 resolver

**判定：CLOSED（M3 要求的 caveat 已写入）；同时引入 D-1（新 MINOR，非阻断）。**

已成立的事实：

- `SPECIFY_FEATURE_NO_PERSIST` 真实存在，是 `--no-persist` 的环境变量等价（`.specify/scripts/bash/common.sh:172-178`）；设置后 `get_feature_paths` 跳过 `_persist_feature_json`（`:187-200`）。
- `create-new-feature.sh:465-467` 同样尊重该变量。
- `/speckit-plan` / `/speckit-tasks` 分别调用 `setup-plan.sh` / `setup-tasks.sh`（`.dsh/skills/speckit-plan/SKILL.md:58`、`speckit-tasks/SKILL.md:58`），二者都 `source common.sh` 并调用 `get_feature_paths`（`setup-plan.sh:28/31`、`setup-tasks.sh:23/26`），因此 plan/tasks 的 `NO_PERSIST` 生效。

未成立/不精确的部分：

- `/speckit-specify` **不调用** `create-new-feature.sh`，而是由技能自身在 `SKILL.md:82-106` 完成 feature dir 解析，并在 `:99-106` **无条件要求**把路径写入 `.specify/feature.json`；该技能全文未提 `SPECIFY_FEATURE_NO_PERSIST`（grep 0 命中）。因此对 `/speckit-specify` 而言，`NO_PERSIST=1` **不能保证**不持久化；只有“事后清理”能兜底。
- 因此 `CONVENTIONS.md:110` 的“这三条命令共用同一 resolver”不精确：plan/tasks 共用 `common.sh:get_feature_paths`；specify 是技能级解析（模板部分才用 `resolve-template.sh`，其 `:6` 也 source common.sh，但 feature dir 不是同一代码路径）。更准确的说法是“共用同一 `SPECIFY_FEATURE_DIRECTORY` 解析规则”，不是同一 resolver。
- `CONVENTIONS.md:110` 已写“`plan.md`/`tasks.md` MUST NOT 写入 `docs/specs/`”，但未显式写“命令先写临时特征目录、再由 owner 回写 canonical”；结合“MUST 显式提供 `SPECIFY_FEATURE_DIRECTORY`”，读者仍可能把变量设到 `docs/specs/`。建议补“用一次性临时目录 + owner 回写 `docs/specs/` 与 `docs/lead/<change_id>/`”。

**D-1（MINOR）最小修复**：`CONVENTIONS.md:110` 改为“三条命令共用同一 `SPECIFY_FEATURE_DIRECTORY` 解析规则；`NO_PERSIST` 仅对 `common.sh`/`create-new-feature.sh` 路径生效，`/speckit-specify` 技能会手写 `feature.json`，运行后须清理/确认；plan/tasks 用临时目录，由 owner 回写”。本轮不阻断。

### DR.4 M7/M8：收益/停止条件与 delta audit

**判定：仍 OPEN（`[TODO]`，不阻断 READY）。**

- Lead 说明 M7/M8 由其在 change input/`review-tracking.md` 记录，**不写入治理文本**（正确分工：`AGENTS.md:87` 允许“review-tracking 或变更输入”；宪法 `:96` 要求 delta audit 记入 `review-tracking.md`）。
- 本次复核时仓库内**尚未**落地：`git status` 无 `docs/review-tracking.md` 变更；无 commit/PR 文本；`grep -rn "GOV-SDD"` 仍 0 命中。故 M7/M8 不能在本 delta 关闭。
- 要求：人类批准前应能核对 M7（收益/停止条件）；合入前必须落 M8（delta audit）。否则 `AGENTS.md:87`/宪法 `:96` 未满足，届时不得视为收口。

### DR.5 新门禁/新状态/新歧义

- **新机器门禁**：无。`git diff --name-only` 仍只有 `AGENTS.md`、`docs/collab/CONVENTIONS.md`；`scripts/**`、`.github/**`、`Makefile`、CI 无改动；§2.4 的 `MUST NOT 写入 docs/specs/` 是文本路径约束，不是机器门禁。
- **新状态字段**：无。`SPECIFY_FEATURE_NO_PERSIST` 是现有环境变量（`common.sh:172-178`），不是仓库状态字段；`.specify/feature.json` 是已存在的机器本地文件且被 `.specify/.gitignore` 忽略。
- **新歧义**：有 2 处，均 MINOR 非阻断：**D-1**（NO_PERSIST 覆盖范围/“同一 resolver”）与 **D-2**（G-1 Tier 适用字段）。
- **M3/M4/M5 关闭后的净变化**：初始 8 MINOR 中 3 项关闭（M3/M4/M5）；新增 D-1/D-2 后，open MINOR = **7**（M1、M2、M6、M7、M8、D-1、D-2）。

### DR.6 中间结论（历史，head `f922b32`）

- **中间 verdict（head `f922b32`）：`READY`**（0 BLOCKER / 0 MAJOR）；**不需要调整首轮 verdict**（历史；最终见文末 `Final scope`）。
- **M3/M4/M5：CLOSED**；**D-1/D-2：新 MINOR**；open MINOR **7**（M1、M2、M6、M7、M8、D-1、D-2）。
- **release_status：`NOT RELEASE`**。
- **后续变化**：人类随后决定完整删除 §2.4；本节 M3/D-1/D-2 的结论由文末 `Final scope after human decision (2026-11-26)` 取代。
- **建议的最小修复（只描述，不执行）**：
  1. `CONVENTIONS.md:110`：按 D-1 精确化 NO_PERSIST 覆盖范围与“共用解析规则（非同一 resolver）”，并写明临时目录→owner 回写。
  2. `CONVENTIONS.md:226`：按 D-2 加“按 Tier 适用字段（至少 …；L2 另见 §3）”。
  3. M7/M8：Lead 在 change input/`review-tracking.md` 落收益/停止条件与 delta audit。
  4. M6：Reviewer 修 `ac-coverage.tsv:38-43` 的 `developer-reviewer/T-003`（仍 `[TODO]`）。
- **声明**：本节仍不构成人类批准、不构成 merge/Release、不替代 Reviewer verdict；本节只追加到本 QC 报告，未修改 `AGENTS.md`/`CONVENTIONS.md` 或其他文件，未 commit。

---

## Final scope after human decision (2026-11-26)

> 本节是**最终结论**，取代 `Delta review`（中间 head `f922b32`）的 head/计数：§2.4 已被人类决定完整删除。只读复核；未改 `AGENTS.md`/`CONVENTIONS.md` 或其他文件，未 commit。

| 项 | 值 |
|---|---|
| change_id | `GOV-SDD-HYGIENE-01` |
| Tier | `L2`（治理文本轻路径；`AGENTS.md:84` / 宪法 `:77`） |
| base | `472e64fec33cbbf91592e2ffc74cbb31abc541db` |
| final head | **当前工作树（未提交）**：`AGENTS.md` blob `e7759cda8999`（未变）、`CONVENTIONS.md` blob `d465d501254e`；`docs/review-tracking.md` 另有 Lead 的 `+23` 行记录（`:350-371`），不是两文件治理文本的一部分 |
| human decision | 2026-11-26 Director 批准 P1，明确**不声明 Spec Kit 路径边界**；项目结构与 Spec Kit 对齐后续另议（`SDD-STRUCT-ALIGN-01` 未开始）——`docs/review-tracking.md:352-354` |
| final scope | 5 hunks：`AGENTS.md:15`、`:77`；`CONVENTIONS.md` §4 摘要删除（`:143-147`）、G-1（`:217`）、§7.2（`:226-228`）。§2.4 已不存在；`grep -iE "speckit|SPECIFY_FEATURE|原协议摘要"` 两文件 **0 命中** |
| diff-check | `git diff --check` = **exit 0**；`git diff --numstat` = AGENTS `2/2`、CONVENTIONS `2/6`、`review-tracking.md` `23/0`（Lead 台账，不属治理文本） |
| verdict | **`READY`** — 0 BLOCKER / 0 MAJOR；M3/D-1 N/A；D-2/M4/M5 CLOSED；open MINOR **5**（M1/M2/M6/M7/M8，按 brief 保持） |
| release_status | **`NOT RELEASE`** |

### FS.1 最终 scope 是首轮审查面的真子集

- 首轮审查面：AGENTS `:15`/`:77`、CONVENTIONS §4 摘要、G-1、§7.2、§2.4。
- 最终只剩前四项的 **5 个 hunk**：AGENTS 2 + CONVENTIONS 3；§2.4 完整删除；G-1 使用 D-2 建议的“§3 中按 Tier 适用字段是否齐备（至少 …；L2 另见 §3）”（`:217`）。
- 无新增文件/路径/hunk；G-1 文本是已建议的收窄，不是新 scope。相对首轮审查面为**真子集**。

### FS.2 计数更新

| 项 | 最终状态 | 证据 |
|---|---|---|
| M3 | **N/A** | §2.4 已完整删除；`grep -iE "speckit|SPECIFY_FEATURE"` 两文件 0 命中；不再有 feature.json/resolver 主题 |
| D-1 | **N/A** | 同上（D-1 是 §2.4 的 NO_PERSIST/共用 resolver 残余） |
| D-2 | **CLOSED** | G-1 改为“§3 中按 Tier 适用字段是否齐备（至少 `change_id`/`Tier`/base/head/`allowed_paths`；L2 另见 §3）”（`CONVENTIONS.md:217`），同时消除 L0/L1 过严与 L2 漏项两种读法 |
| M4 | **CLOSED** | G-1 不再要求“change input 文档”、不再重复 `change_id`；§3 字段集被显式引用（`:217`、`:106-126`） |
| M5 | **CLOSED** | G-1 无 `Exception`；现行规范文本 `Exception` 0 命中；`T-NNN` 仅历史指针（`CONVENTIONS.md:191` + reviewer/architect MEMORY/归档）；退役 ADR-0006 不构成现行要求 |
| M1 | **保持（MINOR）** | 宪法 `:96`/`:75` 与线程退役的既有漂移；本次不改宪法（宪法修订另走 `/speckit-constitution`） |
| M2 | **保持（MINOR，无需本次修）** | §4/§7.2 删除后的归档入站引用变旧；无现行依赖 |
| M6 | **保持（`[TODO]`，Reviewer）** | `ac-coverage.tsv:38-43` 的 `developer-reviewer/T-003` 不存在；不在两文件范围 |
| M7 | **保持（`[TODO]`，提交包装）** | Lead 已把 Benefit/Stop condition 写入 `docs/review-tracking.md:360-365`；正文已落，按 brief 仍列为提交前 TODO（剩余为最终 change-input/commit） |
| M8 | **保持（`[TODO]`，提交包装）** | Lead 已把 Delta audit 写入 `docs/review-tracking.md:366-367`；剩余 `base`/`head`/`allowed_paths`/Reviewer verdict 的 commit/PR 字段 |

> M7/M8 证据说明：若按“正文是否写明”判定，M7 可视为 CLOSED、M8 的 delta-audit 部分已 CLOSED；本报告按 brief 的“保持”清单仍列入提交前 TODO，不提前宣称收口。最终 open MINOR = **5**（M1/M2/M6/M7/M8）。

### FS.3 悬空引用/新歧义/新门禁

- **悬空引用**：无新增。最终 diff 无 `speckit`/`SPECIFY_FEATURE`/`原协议摘要`；`Exception` 仅退役 ADR-0006 与归档线程；`T-NNN` 仅历史指针（`CONVENTIONS.md:191` 的 36 行历史指针，以及 reviewer/architect MEMORY、归档索引/正文）。与首轮 B/F 结论一致：§4/§7.2 删除只影响归档入站引用，无现行规则依赖。
- **新歧义**：无。G-1 的“按 Tier 适用字段/至少/L2 另见 §3”消除了 D-2；§2.4 删除消除了 D-1。M1 的宪法漂移是既有问题，不是本 diff 新增。
- **新门禁/新状态**：无。最终治理文本 diff 仅两文件 5 hunks；无 `scripts/**`、`.github/**`、`Makefile`、CI、状态字段。`review-tracking.md` 的 `+23` 行是 Lead 的台账记录（`AGENTS.md:87`/宪法 `:96` 要求），不是规则/门禁。
- **`git diff --check`**：exit 0（无空白错误）。

### FS.4 最终结论

- **最终 verdict：`READY`**（0 BLOCKER / 0 MAJOR）；不调整。
- **open MINOR：5**（M1、M2、M6、M7、M8；M3/D-1 N/A；D-2/M4/M5 CLOSED）。
- **release_status：`NOT RELEASE`**。
- **声明**：人类已批准 P1，但本 QC final delta **不构成人类批准、不构成 merge/Release、不替代 Reviewer verdict**；`docs/review-tracking.md:368-369` 仍记 Reviewer verdict “pending”。M7/M8 的最终 change input/commit、M6 的台账修复、M1 的宪法 PATCH 仍分别由 Lead/Reviewer/Architect 跟踪。
