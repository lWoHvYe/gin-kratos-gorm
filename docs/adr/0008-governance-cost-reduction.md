# ADR 0008: 治理成本收敛的实现设计（SPEC-0003 / GOV-TIER-01 / GOV-HUM-01 / GOV-QC-01 / GOV-CODE-01 / GOV-SCAF-01）

| 项 | 值 |
|---|---|
| ADR ID | 0008（`docs/adr/0008-governance-cost-reduction.md`） |
| Revision | **Rev 1.1（approved，2026-11-24）**：按 QC-05（`docs/collab/qc-reports/ADR-0008-QC-01.md`，BLOCKED 4 BLOCKER / 8 MAJOR / 8 MINOR）整改 Q-01…Q-20，并绑定**已批准的 SPEC-0003 v1.4**（2026-11-24，sha1 `58679f03…`）；逐项映射与关闭状态见 **§21**。Rev 1.1 经 **QC-02 复审 READY（0 BLOCKER / 0 MAJOR / 7 MINOR；Q-01…Q-20 = 20/20 CLOSED）**，7 项 MINOR（N-1…N-7 = M-1…M-7）作为**编辑性澄清**并入后，由 **Director 于 2026-11-24 批准**（`Status: approved`，批准记录见 **§22**）。Rev 1.0 = QC-05 被审稿（967 行，sha1 `0c67a163…`，HEAD `dff267d`） |
| Status | **withdrawn（2026-11-25）**（原 **approved**：Director 于 2026-11-24 批准 Rev 1.1；随协作闸门工具链整体退役而撤回，见下方 Withdrawal 注记） |
| Date | 2026-11-24 |
| Owner | System Architect（设计 owner） |
| Integration owner | System Architect（治理/工具链落点集成）；**Independent Verifier = Director 本人或其每次运行前书面指定者**（GV-S21 `designated_by`/`independence_evidence`，§6.10；已由 `docs/review-tracking.md` 的 “2026-11-24 human decisions: GOV-COST-01 bootstrap, IV and commit timing” 裁决）；release owner 为 Director 或其书面指定者（GV-S22） |
| Spec | **SPEC-0003 v1.4（approved 2026-11-24；697 行；sha1 `58679f03e417b98a591545c84854cda3aaa6b898`；`approved_by=Director`/`approved_at=2026-11-24`）**，`docs/specs/0003-platform-governance-cost-reduction.md`。**本 ADR 的 bootstrap 条款（§7）只绑定已批准的 v1.4**；QC-05 所审稿为 696 行 sha1 `79db9323…`，批准稿并入 QC-05 M-1…M-7 编辑性澄清（含 GV-S21 `g4_status`）后摘要为 `58679f03…`；v1.3（approved 2026-11-24，689 行，sha1 `75eb8f7d…`）仅作历史背景。**摘要沿革（明确区分，终止漂移）**：①**批准内容摘要 = `58679f03…`（697 行）**；②批准后仅发生一次**元数据回填**（YAML 头 `adr: [] → [0008]`，工作区现为 698 行、sha1 `1371eafc…`），**不含任何 AC/INV 文本变更**，因此不构成 AC-102 的“同版本内容变化”；③**review/Release 的 canonical 绑定 = P0a 提交内的 SPEC-0003 blob**（`git rev-parse <P0a-head>:docs/specs/0003-platform-governance-cost-reduction.md`），**不是**未跟踪工作区文件的 sha1（§6.7 同口径：未跟踪文件不算证据） |
| Review | QC-04 对 SPEC-0003 v1.3：READY（0/0/4）；**QC-05 对 SPEC-0003 v1.4：READY（0 BLOCKER/0 MAJOR/7 MINOR），人类已批准**；QC-05 对本 ADR Rev 1.0：**BLOCKED（4 BLOCKER / 8 MAJOR / 8 MINOR；Q-01…Q-20）**；**QC-02 对本 ADR Rev 1.1：READY FOR HUMAN ADR APPROVAL（0 BLOCKER / 0 MAJOR / 7 MINOR；Q-01…Q-20 = 20/20 CLOSED）**，7 MINOR 已编辑性并入；**Director 已于 2026-11-24 批准本 Rev 1.1**（§22；宪法 `:75-76`） |
| Baseline impact | 业务 AC-01…68 / INV-01…50 **零语义变更**；治理产物、CI、脚本、模板、台账受 §17 影响 |
| Change ID | `GOV-COST-01`（`docs/lead/GOV-COST-01/` 已存在）；最终分配/注册归 Lead（GV-S12/GV-S19），本 ADR 不写注册表 |
| Supersedes | 无。本 ADR 是 SPEC-0003 `adr: []` 的**首个** ADR（Spec §11 “ADR 关系”） |

> **被审内容绑定（AC-102）**：本 ADR 的规范绑定 = **已批准的 SPEC-0003 v1.4**（697 行，sha1 `58679f03e417b98a591545c84854cda3aaa6b898`，`approved_by=Director`/`approved_at=2026-11-24`）的 AC-69…AC-112 与 INV-51…INV-81。QC-05 审稿 sha1 `79db9323…`（696 行）→ 批准稿 `58679f03…`（697 行）的差异 = QC-05 M-1…M-7 编辑性澄清（M-3 新增 GV-S21 `g4_status`；M-2 把 AC-69 的 G-1…G-4 四项输出要求一并纳入 bootstrap 豁免；其余为字段/引用/记录一致性），AC 编号与语义不变。**本 ADR 已按批准稿重新对齐**（Rev 1.1 交付前完成）。
>
> **v1.4 的绑定边界（必须显式声明，替代 Rev 1.0 对 v1.3 的绑定）**：
> 1. **v1.4 已批准**（approved 2026-11-24；`approved_by=Director`）。本 ADR Rev 1.1 **亦已由 Director 于 2026-11-24 批准**（`Status: approved`，§22）；**Spec 批准不构成 ADR 批准**，故批准前的 P0a/实现禁令现仅按 §20 的其余前置（Plan 书面批准、IV 指定）继续生效。
> 2. **bootstrap 口径只以 v1.4 为准**：AC-106/AC-112 = `BASE` 树 legacy **G-1…G-3 PASS**（scoped 到 AC-01…AC-68 + 已批准 Spec 版本，显式 `--range=<BASE^>..<BASE>`）+ 显式 **`G-4 NOT-RUN(not-implemented-at-BASE)`** 记录（不阻断 P0a/P0b、永不计 PASS、不授权 Release/AC-VERIFIED）+ IV 逐行 diff + 人类书面批准/`expiry`；**完整 G-4 在 P0b 落地并自 P0b 起必需**；AC-69 `GATE_INCOMPLETE` 对“bootstrap 期 G-4 未实现”的豁免**随 P0b 终止**。
> 3. 本 ADR **不解释、不改写、不放宽** v1.4 的 AC 文本；凡 ADR 表述与 v1.4 冲突，以 v1.4 为准并停线（`EVIDENCE_UNBOUND`），ADR 侧不得给出“QC/人类认可口径”式替代。
> 4. **重对齐条款**：若 v1.4 在**本 ADR 批准前**再次发生实质变化（AC-106/AC-112 的证据组合、G-4 落地时点、scoped AC 集、GV-S15/GV-S20/GV-S21 字段），或批准后升版（v1.5+），本 ADR 必须重新对齐并升 Rev（Rev 1.2+），QC 重审不得省略；若 v1.4 被否/回滚，本 ADR 的 §7/§15-1/§21 失效并按新文本重写。
> 5. 与 v1.3 的差异仅限 bootstrap 证据口径、§6/§12 交叉引用与行号校正（v1.4 §13 自述）；其余 AC/INV 文本不变，故本 ADR 的非 bootstrap 设计不受影响。
>
> **版本格式现状**：G-2 当前只解析两段式版本（`scripts/collab_gates.py:38` 的 `VERSION_RE` 与索引版本正则 `:196`），补丁级版本会被判 `G-2 FAIL ... missing YAML version`（`:212-214`）。v1.4 为两段式，不阻塞本 ADR；P0b 把解析扩为 `MAJOR.MINOR[.PATCH]`，并**新增同版本内容摘要比较**（§6.11 G-2，Q-07），以免“版本号不变、内容已变”被机器放行。

> **Withdrawal（2026-11-25）**：Director 决定**整体退役协作闸门工具链**，本 ADR 随 `Status: withdrawn` 撤回。P0a 已落地的执行器代码及配套 wrapper/自测（`scripts/collab_gates.py`、`scripts/check-collab-gates.sh`、`scripts/test-collab-gates.sh`）、Makefile `gates*` 目标与 CI `collab-gates` job 已随闸门一并删除；P0b 及之后（G-4、`--release`、`scope`、GV-S18/GV-S20/GV-S21 载体、`collab-gates.yml`）**从未实现**。本 ADR 与 SPEC-0003 仅保留为**设计历史**，不是现行契约：其 AC-69…AC-112 / INV-51…INV-81、载体 schema、`MANAGED` 扩展与实现清单均不再有执行者或验收义务；人工审查与常规 test/build 门禁不受影响。

---

## 0. 决策摘要（D-08-01…D-08-20）

| ID | 决策 | 依据 |
|---|---|---|
| D-08-01 | **单一执行器 + 既有四闸门**：只补齐 G-1…G-4，不新增 G-5/G-6，不引入第二执行器或新门禁类型 | AC-69；`docs/collab/CONVENTIONS.md:188`；ADR-0006 §6 |
| D-08-02 | **三态 scope**：`local`（默认，可 warn）/`review`/`release`；`review`/`release` 只接受 strict，拒绝 `--mode=warn` | AC-69/70；Spec `:139`、`:461` |
| D-08-03 | **Review/Release 必须显式 `base..head`**；未提供即 `RANGE_UNBOUND`，默认范围只用于 `local` 提示 | AC-71；`scripts/collab_gates.py:61-69` 现状 |
| D-08-04 | **治理证据模型**：8 列台账（`test_type/evidence_path/status`）+ 证据工件 + GV-S20 审计行（Reviewer 签名 + 摘要），不改台账 schema | AC-99/101/95；GV-S8；`docs/collab/ac-coverage.tsv:1` |
| D-08-05 | **内容摘要**：`sha1:<40hex>` 内容摘要 + `diff_digest`（`git diff --binary --full-index base..head` 的 sha1），写入 QC/Review/Release/门禁记录 | AC-102/95；INV-71 |
| D-08-06 | **tier registry 是数据表而非新台账**：判据规范源仍是已批准 Spec/AGENTS；执行器只做“路径下限 + 未知⇒L2 + per-change 继承”的客观判定 | AC-75/97/111；INV-56/57/79/80 |
| D-08-07 | **L1 联签 = Reviewer verdict 之后、merge 之前**；QC 与 Reviewer 各写一个签署块，`SIGNED` 为派生值；G-1 机械核验字段/顺序/摘要 | AC-82/84/98；GV-S16 |
| D-08-08 | **bootstrap P0a→P0b（绑定 v1.4）**：`BASE` 投影树内跑 legacy 执行器（显式 `--range=<BASE^>..<BASE>`）+ 变更后可复现的 scoped projection + IV 双摘要 + 人类书面批准/`expiry`；`BASE` 上 G-4 缺失以 `G-4 NOT-RUN(not-implemented-at-BASE)` 记录（不阻断、不计 PASS），P0b 落地完整 G-4；不用未上线的新 G-3 自证 | AC-106/112（v1.4）；INV-75/81 |
| D-08-09 | **治理门禁独立 workflow**：新增 `.github/workflows/collab-gates.yml` 只跑 `collab-gates`；`go.yml` 的 `verify`（含 MySQL service）不被 `docs/**`、`prompt/**`、`.specify/**` 触发；`GATE_NOT_RUN` 由 IV 发出 | AC-73/100/107；D-CR-5 |
| D-08-10 | **自保护**：门禁/CI/Makefile 变更必须基线 + 变更后各跑一次 + IV 逐行 diff；被审 diff 内执行器不得自证；P0b 退出必须含一次 **review-scope** 运行或等价负向夹具 | AC-100；INV-70/76 |
| D-08-11 | **canonical 载体先登记后写入**：G-1 `MANAGED` 支持 owner + append-only co-writer；`docs/lead/releases/**` owner=release owner、`docs/lead/<id>/gate-runs/**` 内容产出者=`gates-executor`（提交者=Lead/Reviewer）；未登记即 `EVIDENCE_UNBOUND` | AC-109；INV-78 |
| D-08-12 | **scaffold 生成结构 + 强制主权声明**，内容仍由 owner 写；幂等键 `change_id+artifact+template_version`；不产生第二台账 | AC-90/91/92；GOV-SCAF-01 |
| D-08-13 | **追加格式固化**：GV-S10 错分台账、GV-S19 注册表、GV-S20 审计日志（15 列 + 人类独占 mode）、GV-S21 复核记录的 TSV/Markdown 表头与枚举 | AC-81/93/95/103/107 |
| D-08-14 | **业务基线零变更**；本 ADR 批准后回填 SPEC-0003 `adr: [0008]`（Architect 职责），不在本 ADR 内改 Spec | AC-96/109；Spec §11 |
| D-08-15 | **per-change tier 判定改为记录内容规则**：不再按文件名命名空间；判据 = diff 只追加 + 记录携带 `parent_change_id`（+ `change_id`）；`ac-coverage.tsv`/`ac-baseline.txt`/QC 报告/GV-S19/GV-S20 行同一张表（§4.5/§4.8），`--print-tier-rules` 输出 | AC-111；INV-80 |
| D-08-16 | **人类写入字段统一落 GV-S20**：GV-S9 豁免、GV-S15 批准、GV-S17 裁决分别复用 `mode=waiver` / `approval\|bootstrap\|release` / `tier` 行，补齐 `expiry/scope/affected_AC/affected_INV/撤销`；AI 只读，非人类 actor 写入即拒绝 | AC-94/108/112/95；INV-65/77 |
| D-08-17 | **GV-S18 载体定稿**：gate record TSV（`docs/lead/<change_id>/gate-runs/<base7>-<head7>-<scope>.tsv`）；执行器只写该目录与 `--audit-row-out` 工件，不写 `review-tracking.md`、不改 git 历史 | AC-70/74；INV-66 |
| D-08-18 | **GV-S20 追加在 review/release 强制**：CI 产“待追加行”工件，Lead/Reviewer 提交，G-1 机器核对行存在且摘要匹配；缺行即 `EVIDENCE_UNBOUND` | AC-95；INV-66 |
| D-08-19 | **IV 人选已裁决**：Director 本人持有，或每次运行前书面指定并写入 GV-S21（`designated_by`+`independence_evidence`），指定者不得是变更作者 | AC-107；`docs/review-tracking.md` “GOV-COST-01 bootstrap, IV and commit timing” |
| D-08-20 | **`--release-scope` 机器校验**：存在性、必填字段、对象版本/摘要与当前 Spec/ADR 一致；缺失/字段缺/摘要不符/陈旧 → 阻断（§5.4/§6.3） | AC-110；INV-74 |

---

## 1. Context

### 1.1 契约现状

SPEC-0003 的 **v1.4（approved 2026-11-24）** 是本 ADR 的规范绑定（v1.3 已于 2026-11-24 批准、QC-04 判 READY，仅作背景；v1.4 仅修订 bootstrap 证据口径）。它把治理规则从“文字约定”升级为可审计契约：L0/L1 封闭枚举 + 其余 L2（AC-75…78/97）、错分停线（AC-80/81）、L1 联签（AC-82…85/98）、QC delta/full 身份（AC-86…89/105）、门禁执行器补全（AC-69…74/99…103）、bootstrap（AC-106/112）、载体与审计（AC-93/95/109/110/111）。

本 ADR 只做**实现设计**：把这些 AC 映射为可执行、可复核、单写者、fail-closed 的机制；不重写 Spec、不新增业务规则、不实现业务代码。

### 1.2 仓库现状事实（审计日 2026-11-24）

| # | 事实（实际值） | 证据 | 对本设计的含义 |
|---|---|---|---|
| 1 | 执行器自述只实现 G-1/G-2/G-3，无 G-4、无 `--release` | `scripts/collab_gates.py:2` | AC-69 要求补齐 G-4/Release，不能以“未实现”当 PASS |
| 2 | `MANAGED` 只覆盖 8 个前缀；无 `scripts/**`、`.github/**`、`Makefile`、`docs/review-tracking.md`、`docs/collab` 台账、`docs/lead/**`、`prompt/**` | `scripts/collab_gates.py:17-28` | G-1 的 canonical 覆盖不足（AC-109/INV-78） |
| 3 | 版本正则只接受 `\d+\.\d+`（YAML 与 README 索引各一处） | `scripts/collab_gates.py:38`、`:190-198`、`:212-214` | 补丁级版本会 FAIL；必须先扩展解析（§6.11） |
| 4 | 未传范围时默认 `HEAD^..HEAD`；无父提交退化为 `HEAD..HEAD` | `scripts/collab_gates.py:61-69` | Review/Release 不得复用默认范围（AC-71） |
| 5 | 台账解析只取 `owner/expiry/reason/thread`，**忽略** `test_type/evidence_path/status` | `scripts/collab_gates.py:275-296`（存储见 `:292-295`） | AC-99 要求读取三列并校验，否则治理 AC 无法验证 |
| 6 | G-3 从工作区读**全部** `docs/specs/*.md` 的 AC 集 | `scripts/collab_gates.py:306` | 治理 AC 一旦入 Spec 即进入 G-3 范围；scope 运行必须可复现 |
| 7 | 输出 `AC coverage: n/m` | `scripts/collab_gates.py:362` | AC-72 明令改用 `AC-REF`/`AC-VERIFIED` |
| 8 | warn/strict 仅由退出码区分，失败信息照常打印 | `scripts/collab_gates.py:390-397` | `mode=warn` 零退出不构成 PASS；Review/Release 必须拒绝 warn |
| 9 | wrapper 只剥离 `--mode`，其余参数透传；无 `RANGE/SCOPE/RELEASE` 透传 | `scripts/check-collab-gates.sh:5-16` | 需要在 Makefile/CI 显式传范围与 scope |
| 10 | Makefile 门禁入口无 `RANGE/SCOPE` 透传；`verify` 只挂 warn | `Makefile:159-169` | 需要新增透传，不改变 `verify` 语义 |
| 11 | CI 触发路径：`**/*.go`、`go.mod`、`go.sum`、`api/**`、`migrations/**`、`scripts/**`、`Makefile`、workflow 自身；**无** `docs/**`、`prompt/**`、`.specify/**`、根 `AGENTS.md` | `.github/workflows/go.yml:4-23` | 治理文档变更可绕过 CI（AC-73/D-CR-5） |
| 12 | `collab-gates` job 只跑 `make gates-strict`（无显式 range） | `.github/workflows/go.yml:26-33` | 不满足 AC-71；CI 绿不能证明全分支 |
| 13 | 自测 A1…A5 断言字符串 `AC coverage` | `scripts/test-collab-gates.sh:92` | 措辞改造后自测必须同步，否则假绿 |
| 14 | 业务台账 8 列；业务行 `test_type=deferred`、`status=DEFERRED` | `docs/collab/ac-coverage.tsv:1-2`（37 行） | 新枚举**只作用于 AC-69…112**，不得追溯改业务行 |
| 15 | `ac-baseline.txt` 止于 AC-68（68 行） | `docs/collab/ac-baseline.txt:68` | 治理 AC 尚未登记；批准轮由 Reviewer 登记 |
| 16 | `review-tracking.md` 已有三个 human decisions 区（区段标题见下），**无** `Change freeze` / `Governance audit log` 区 | `docs/review-tracking.md` 的区段 “2026-11-24 human decisions: governance cost reduction (D-CR-1…D-CR-5)”、“…GOV-COST-01 bootstrap, IV and commit timing”、“2026-11-24 SPEC/ADR approvals”；**行号会漂移，引用只带区段标题** | AC-109 要求先由 Lead 建立区并登记 |
| 17 | `.specify/templates/` 有 plan/tasks/checklist/spec/constitution 模板，**无** `evidence-matrix-template.md` | `ls .specify/templates/` | AC-90 的第三个模板缺失，owner=System Architect |
| 18 | D-CR-1…D-CR-5 canonical 批准记录 | `docs/review-tracking.md` 区段 “2026-11-24 human decisions: governance cost reduction (D-CR-1…D-CR-5)”（当前文件约 `:94-105`，以区段标题为准） | 本 ADR 的上游依据；不是 Release PASS |
| 19 | CONVENTIONS §7.2 自认 G-4、`--release`、`AC coverage`、默认范围、CI 覆盖均未落地 | `docs/collab/CONVENTIONS.md` 的 §7.2 债务清单 | 本 ADR 是这份债务清单的收口设计 |
| 20 | QC 报告实际命名为 `<主题>-QC-NN.md`（如 `SPEC-0003-QC-04.md`、`ADR-0007-QC-01.md`），**从不是** `<change_id>*` 前缀 | `docs/collab/qc-reports/` 目录清单 | per-change tier 判定**不得**按文件名命名空间（Q-03），改按记录内容 + append-only（§4.5/§4.8） |

### 1.3 为什么需要本 ADR

- AC-69…74 要求“机器可执行输出”，但当前执行器连 G-4 与 Release 语义都不存在；没有实现设计，Developer 会各自发明字段/退出码，Reviewer 无法复核。
- AC-99/100/107/112 要求“治理证据 + 自保护 + IV + bootstrap”，涉及**执行器输出契约、canonical 载体、CI 触发、独立复核**四个边界；这些是架构决策，不能留给实现即兴。
- 本 ADR 不引入架构/接口复杂度：**无新服务、无 DB、无网络依赖、无 Kafka/Temporal/新 ORM**；只使用 python3 标准库、bash、git、GitHub Actions 与 Markdown/TSV/YAML。

---

## 2. 范围、非目标与红线

### 2.1 设计范围

1. tier registry（路径下限/未知⇒L2/per-change 继承）与 effective_tier 派生。
2. canonical 载体清单、owner、格式、写入时机、G-1 `MANAGED` 注册。
3. G-1…G-4 的输出契约、scope/mode、显式 range、退出码、治理证据模型、摘要绑定、自保护、IV。
4. bootstrap P0a/P0b 的 `BASE`、scoped legacy 运行、证据、批准/到期/解除。
5. CI 路径覆盖与 `GATE_NOT_RUN` 外部检测。
6. L1 QC+Reviewer 联签机制与失效/重签。
7. QC delta/full 身份（version+digest）与 Release 对象集。
8. scaffold 生成器、主权声明、幂等与边界。
9. 错分台账/注册表/审计日志/IV 记录的行政格式。
10. baseline 影响、AC 覆盖矩阵、上线顺序与回滚。

### 2.2 非目标（明确不做）

- 不修改 SPEC-0003、宪法、AGENTS、CONVENTIONS 的规则文本；治理落点文本由对应 owner 在批准后按 Spec §11 修订。
- 不实现业务代码、迁移、服务；不触碰 `internal/services/**`、`cmd/**`、`api/**`、`migrations/**`、`frontend/**`。
- **不新增 G-5/G-6**，不把语义判断（架构好坏、断言深度、是否可发布）交给机器。
- 不引入新依赖/新平台（无 Kafka、Temporal、新 ORM、外部 SaaS、向量库）；不联网即可跑门禁。
- **不提供任何本地补丁式“安全网”**：无 `SKIP_GATES`、无 `--force`、无路径白名单、无恒 PASS 分支；修门禁只能改协议/载体并经 L2 流程。
- 不做“治理 AC 批量延期”；不把 NOT-RUN 记为 PASS。
- 不在本 ADR 内创建任何新文件（除本 ADR）；模板/注册表/审计区由后续 change 按 §11/§12 建立。

### 2.3 红线映射

| 红线 | 本设计的承载 |
|---|---|
| R-1 自读宪法 | 不变；IV/Reviewer 自读，`delta audit` 见 §12.5 |
| R-2 契约先行 | 不变；本 ADR 批准前不得写实现（宪法 `:76`） |
| R-3 唯一台账 | `ac-coverage.tsv` 仍是唯一 AC 状态台账；plan/evidence-matrix 只是视图（AC-91） |
| R-4 测试不可伪造 | G-3 只认 `VERIFIED` + 证据工件 + Reviewer 签名；G-3 PASS ≠ 执行证据（AC-99/101） |
| R-5 fail-closed | 全链路 NOT-RUN/ERROR/TIMEOUT ≠ PASS；无默认放行（§6.2/§6.6） |
| R-6 判据不可篡改 | tier 判据只存在于已批准 Spec；执行器数据表变更须 Spec 升版 |

---

## 3. 设计总览

```text
                 ┌──────────────────────────────────────────────────────┐
                 │ 变更输入：change-package（GV-S3，Lead；Reviewer 追加 head）│
                 │ base/head/tier/effective_tier/spec/adr/AC/allowed_paths │
                 └───────────────┬──────────────────────────────────────┘
                                 │
   ┌─────────────────────────────▼──────────────────────────────────────┐
   │ scripts/collab_gates.py  scope=local|review|release                │
   │  G-0 仓库/参数校验 → GATE_ERROR（退出 2；不是第五闸门，§6.2）        │
   │  G-1 范围/所有权/CI 配置/联签/自保护引用/tier 派生/审计行           │
   │  G-2 契约版本（三段语义版本 + 同版本内容摘要 + README 索引/变更记录）│
   │  G-3 AC-REF / AC-VERIFIED（一律从被审 head 读取；§6.7）             │
   │  G-4 到期/债务/豁免/bootstrap 有效期（只在 P0b 起存在）             │
   └───────┬───────────────────────┬───────────────────────┬────────────┘
           │ stdout（人读）         │ --record-out（GV-S4/GV-S18）│ 退出码
           ▼                       ▼                       ▼
   Reviewer / 人类          docs/lead/<id>/gate-runs/*.tsv    0 PASS / 1 FAIL
                                    │                     2 GATE_ERROR
                                    │                     3 GATE_INCOMPLETE
   ┌────────────────────────────────▼───────────────────┐ 4 NOT-RUN（治理证据未 VERIFIED）
   │ --audit-row-out：待追加 GV-S20 行（CI artifact）    │ 124 GATE_TIMEOUT
   │  → Lead/Reviewer 提交（review/release 强制，§6.12） │
   └────────────────────────────────┬───────────────────┘
                                    │
   IV（GV-S21，Director 或书面指定）← BASE 投影 vs 变更后 / P0b review-scope 自检
                                    │
                                    ▼
   docs/review-tracking.md：Change freeze（GV-S10）/ Governance audit log（GV-S20）
   CI：.github/workflows/collab-gates.yml 只跑治理门禁（不触发 verify/MySQL）
```

**唯一执行器、唯一 AC 台账、唯一审计载体**；新增的是字段/格式/记录，不是第二套状态。**G-0 是既存的前置校验标签，归 `GATE_ERROR`（不是第五个闸门）**；写入者边界：执行器只写 `docs/lead/<change_id>/gate-runs/**` 与显式指定的 `--audit-row-out` 工件（§6.12）。

---

## 4. 分级注册表与全域总函数

### 4.1 总函数（AC-75/97）

对每个 `change_id` 恰好产生一个标签：

```text
tier(c) =
  L0    if 仅命中 L0 封闭枚举（AC-76）且未命中任何 L2 判据/路径
  L1    if 仅命中 L1 封闭枚举（AC-77）且未命中任何 L2 判据/路径
  L2    otherwise（默认）
effective_tier(c) = (GV-S17 == UPGRADE_L2) ? L2 : tier(c)
```

- **判据不足、文件/行为冲突、未列举路径/工件、新建 canonical 载体**一律 L2（AC-97 `:351`）。
- **最高子改动级别**决定 overall tier；per-change 记录不参与（AC-111，§4.8）。
- 排期标签 P0/P1/P2 不改变级别（AC-97 `:354`）。
- 执行器只做**客观子集**：路径下限、未知⇒L2、per-change 继承、`effective_tier` 派生；语义判据（是否行为变更/是否契约变更）由 Lead 初判 + Reviewer `tier_review` 承担。

### 4.2 L0（AC-76）

封闭枚举仅三项：(a) 非治理文档错别字/链接/格式；(b) 注释/空白/格式；(c) 测试名称/注释/格式。**机器可判的只有“路径是否命中 L2 路径表”**；语义部分由 Lead 记录判据、Reviewer `CONFIRMED`，争议交人类。执行器对 L0 声明做两项硬检查：

1. 变更路径不得命中 §4.5 的 L2 路径下限；命中即 `TIER_MISMATCH`（G-1 FAIL）。
2. L0 仍须 `change_id`/`Actor`/`Reviewed-by`/range（AC-76 `:239`）。

### 4.3 L1（AC-77）

封闭枚举：(a) 单服务内既有已批准 Spec 条款下的行为修正；(b) 既有契约下的校验/映射/本地化文案；(c) 既有契约下的查询/排序/读模型修正；(d) 既有契约下的日志/指标/链路或无语义性能调优。

- L1 不要求 Plan/新 Spec/ADR（AC-77 `:244`、AC-98 `:357`）。
- L1 影响既有 AC 时，受影响 AC 必须有真实执行证据（AC-101，§9.2）。
- 命中任一 L2 判据 → 拒绝 L1（`TIER_MISMATCH`），按 AC-80 停线。

### 4.4 L2（AC-78）

命中以下任一即 L2：不属于 L0/L1 封闭枚举；P0/跨服务/资金/库存/权限/状态机/契约新增修改；触及宪法/AGENTS/CONVENTIONS/任何 Spec/ADR/`prompt/**`/治理文档/`docs/specs/**`/`docs/adr/**`/`docs/specs/README.md`/`scripts/**`/`.github/**`/`Makefile`/`internal/platform/**`/canonical 载体/未知路径。

L2 全流程：Spec→QC→人类书面批准→（需要架构/接口设计时）ADR→QC→人类书面批准→Plan/Tasks→人类书面 Plan 批准→实现→Reviewer→Release 人类书面放行。L2 治理/工具链/canonical 无架构/接口设计时**免 ADR 但不得免** Spec/QC/人类书面/Plan/Independent Verifier（AC-78 `:247`、AC-98 `:358`）。

### 4.5 tier registry、路径下限与 effective_tier（AC-79/97/108）

**registry 形态**：执行器内一张只读数据表（随代码版本化），规范来源是已批准 Spec/AGENTS；`--print-tier-rules` 打印，便于 QC/Reviewer 对表。

| 字段 | 含义 |
|---|---|
| `rule_id` | 稳定 ID（如 `L2-PATH-SCRIPTS`），测试断言其与 AC-76/77/97 的对应关系 |
| `kind` | `path` / `meta`（行为/契约/排期标签不由机器判） |
| `pattern` | 路径前缀/glob（仅 `kind=path`） |
| `tier_floor` | `L2`（本 ADR 只编码下限，不编码 L0/L1 语义） |
| `spec_ref` | 对应 AC/条款（审计用） |
| `registry_version` | 与 Spec 版本/摘要绑定；变更 registry 本身是 L2 |

**L2 路径下限（不可降级）**：`AGENTS.md`、`docs/collab/CONVENTIONS.md`、`.specify/memory/**`、`docs/specs/**`（含新建）、`docs/adr/**`、`prompt/**`、`scripts/**`、`.github/**`、`Makefile`、`internal/platform/**`、`docs/review-tracking.md`、`docs/collab/ac-baseline.txt`、`docs/collab/ac-coverage.tsv`、`.specify/templates/**`、`docs/lead/**`（change-package/release scope）、`docs/collab/change-id-registry.tsv`、`docs/collab/gate-runs/**`、`docs/collab/qc-reports/**`、其他已登记 canonical 载体。

**per-change 记录判据（AC-111；Rev 1.1 重写，Q-03）**：**不再使用文件名命名空间**（Rev 1.0 的 `docs/collab/qc-reports/<change_id>*` 与实际命名 `SPEC-0003-QC-04.md`/`ADR-0007-QC-01.md` 不符，且会让任何 L1 的台账行/QC 报告被判 L2）。一条记录只要**同时**满足下列两个条件，即为本次变更的 per-change 记录，继承父 `effective_tier`，不触发 L2 路径下限：

1. **diff 只追加**：该载体在本 range 内 `git diff -U0 <base>..<head> -- <path>` 只有 `+` 行，没有 `^-` 删除/改写行，也没有表头/schema/枚举变化；
2. **记录内容自带归属**：记录本身（或该变更的 change-package / GV-S20 行）携带 `change_id` 与 `parent_change_id`，且 `parent_change_id` = 本变更的 `change_id`；行内无法携带字段的载体（`ac-baseline.txt` 单行 `AC-NN`、`docs/specs/README.md` 索引行）由其 change-package/GV-S20 行的 `inherited_files` 显式声明覆盖该路径（`inherited_files` 是 change-package 的**可选**声明键，不改变 Spec 的必填字段集；未声明即按“无法声明”处理，不继承、走 L2 下限）。

机器读法（一律从被审 head 取，与 §6.11 G-1/G-3 同口径）：`git show <head>:<path>` 取内容；`git diff -U0 <base>..<head> -- <path>` 判追加；change-package 表头取 `parent_change_id`/`inherited_files`；GV-S20 取该行 `parent_change_id`。任一条件不满足 → 按路径下限 L2；若变更自称 L0/L1 ⇒ `TIER_MISMATCH`。

**同一张表（§4.5 与 §4.8 共用；`--print-tier-rules` 原样输出）**：

| rule_id | 载体（路径） | per-change 判定输入（机器读） | 继承条件 | 不满足时 |
|---|---|---|---|---|
| `PC-CHANGE-PACKAGE` | `docs/lead/<id>/change-package.md` | 本变更 change-package 的 `parent_change_id`/`tier` | 目录归属本 `change_id`；`parent_change_id` 存在（首次变更写自身） | 按 AC-97 未知⇒L2 |
| `PC-QC-REPORT` | `docs/collab/qc-reports/<任意名>.md`（实例 `SPEC-0003-QC-04.md`、`ADR-0007-QC-01.md`） | 报告头 `change_id`/`parent_change_id`/`effective_tier`/`reviewed_digest` | 报告头 `parent_change_id` = 本 change_id；range 内只追加（新文件视为只追加） | `EVIDENCE_UNBOUND`；按 L2 复审 |
| `PC-COVERAGE-ROW` | `docs/collab/ac-coverage.tsv` 新增行 | 行内 `AC-0NN`；change-package `inherited_files`；diff 只增行 | 只追加 + `inherited_files` 声明 + 父 tier 一致 | L2（改表头/改既有行 = carrier L2） |
| `PC-BASELINE-LINE` | `docs/collab/ac-baseline.txt` 新增行 | `AC-NN` 升序连续；change-package `inherited_files`；diff 只增行 | 只追加 + `inherited_files` + 未删除既有 AC | L2；删除/改写 ⇒ `TIER_MISMATCH` |
| `PC-REGISTRY-ROW` | `docs/collab/change-id-registry.tsv` 追加行 | 行内 `change_id`/`tier`；`refs` 中的 `parent=<id>`（登记派生 change 时必填） | 只追加 + `change_id` 唯一 + `tier` = 该 change 的 effective_tier；派生行 `refs` 含 `parent=` | `EVIDENCE_UNBOUND`；改既有行 = L2 |
| `PC-AUDIT-ROW` | `docs/review-tracking.md` `Governance audit log` 追加行 | GV-S20 列 `change_id`/`parent_change_id`/`effective_tier`/`digest` | 只追加 + `parent_change_id` = 本 change_id + 引用本 range | `EVIDENCE_UNBOUND` |
| `PC-IV-FILE` | `docs/collab/gate-runs/<id>/<base>-<head>.md` | GV-S21 头字段 + change-package `inherited_files` | 新文件/只追加 + 父 tier 声明 | `GATE_NOT_RUN`/`EVIDENCE_UNBOUND` |
| `PC-GATE-RECORD` | `docs/lead/<id>/gate-runs/<base7>-<head7>-<scope>.tsv` | 记录头 `change_id`/`parent_change_id`/`scope` | 新文件/只追加 + 本 range | `EVIDENCE_UNBOUND` |
| `PC-RELEASE-SCOPE` | `docs/lead/releases/<release_id>/scope.md` | GV-S22 字段 + `change_id`/`parent_change_id` | 由 release owner 写；对象集变化允许**同一 owner 原地重写本文件**（§5.4 的显式 append-only 例外） | `QC_FULL_REVIEW_REQUIRED` |
| `PC-README-ROW` | `docs/specs/README.md` 索引行与 `## 13. 变更记录` 行 | change-package `inherited_files`；diff 只增行 | 只追加 + `inherited_files` 声明 | L2（改索引结构 = L2） |
| `PC-EVIDENCE` | `docs/lead/<id>/evidence/**` | 目录归属 `change_id` | 新文件/只追加 | `EVIDENCE_UNBOUND` |

`--print-tier-rules` 必须输出上表（`TIER-RULE <rule_id> carrier=<path> inherit=append-only+parent_change_id source=<change-package|row|header>`），使 QC/Reviewer 可对表；`registry_version` 与 SPEC-0003 v1.4 摘要绑定。

**写入者分离与 effective_tier 读取（AC-79/108；Q-04）**：`change_tier` 只由 Lead 写入；`tier_review` 只由 Reviewer 写入；二者不得同一 actor（AC-79）；`GV-S17` 只由人类写入，载体是 GV-S20 的 `mode=tier` 行（§5.1/§12.2）。**effective_tier 是派生值，不落盘为第二字段**（避免第二写入者）：

- G-1 读取顺序：(1) change-package 的 `tier`（GV-S1）；(2) 被审 head 的 `docs/review-tracking.md` `Governance audit log` 中该 `change_id` 的最新非 `supersedes` `mode=tier` 行（GV-S17）；(3) `effective_tier = (GV-S17 = UPGRADE_L2) ? L2 : GV-S1`，输出 `tier_source=GV-S1|GV-S17` 与所用行的 `timestamp`。
- **缺失** GV-S17 ⇒ 视为未升级（effective = GV-S1），不是错误；**行存在但缺 `reason/date/actor/evidence`** ⇒ `EVIDENCE_UNBOUND` 且 fail-closed 取 L2；**多行冲突且无 `supersedes` 链** ⇒ `EVIDENCE_UNBOUND` + L2；**非人类 actor** 写 `mode=tier` ⇒ 拒绝（§12.2 的 HUMAN_ONLY_MODES）。
- G-4/Release 读 GV-S15（`mode=approval|bootstrap|release`）与 GV-S9（`mode=waiver`）同样从被审 head 读；缺失/过期/撤销的行为见 §6.11 G-4 与 §6.7。

### 4.6 错分停线（AC-80/81/85）

- 检测到 L2 被按 L0/L1 执行（执行器路径下限、Reviewer `MISMATCH`、任何人举报）→ **立即冻结 `change_id`**：停止受管提交，Reviewer 在 GV-S10 追加错分行。
- 已提交/已合入不免除：人类写 GV-S17 `UPGRADE_L2`，按 L2 重走（补 Spec/ADR/QC/人类批准/Plan 重审/Reviewer 重审）。
- 新 `change_id`、`hotfix:`、拆提交、改判据一律 `TIER_MISMATCH` 拒绝；`hotfix:` 仅限人类批准的独立安全修复且不得扩展原 `allowed_paths`（宪法 `:83`）。
- L1 实施中产生 L2 影响（跨服务/资金/库存/权限/状态机/治理/canonical）→ 停线 + 作废 GV-S16 联签（AC-85）。

### 4.7 L1/L2 产物 × 批准者矩阵（AC-98）

| 阶段 | L1（仅封闭业务枚举） | L2（默认，含治理/工具链/canonical） |
|---|---|---|
| 变更输入包 | Lead 写；Reviewer 追加 head | Lead 写；Reviewer 追加 head |
| Spec/ADR | 不新开，引用既有条款 | 人类书面批准 Spec/ADR |
| Plan/Tasks | **不要求** | **要求**；人类书面 Plan 批准后才能写代码 |
| QC | QC+Reviewer 联签（见 §9） | 全量/delta（§10）；人类批准 |
| 实现/Review | Reviewer verdict + scope + digest | Developer 实现；Reviewer verdict；IV（如触及门禁） |
| Merge | 人类知会/裁决 | 人类知会/裁决 |
| Release | 人类书面批准 | 人类书面批准（GV-S15） |
| 自保护 | 不要求 | 触及门禁/CI/`Makefile`/canonical 时 AC-100/AC-107 |

**“任何级别 Plan 批准”** 一律解释为“仅在存在/要求 Plan 时”（AC-98 `:361`），不得给 L1 增加 Plan 门。

### 4.8 tier scoping（AC-111）

- 以**被变更的对象/行为**定级，不以本次产生的记录定级。
- per-change 记录（change-package、QC 报告、review verdict、GV-S20/GV-S21 行、本次的 baseline/ledger 行、release_scope、README 索引行、证据工件、gate record）必须写 `parent_change_id` 与父 `effective_tier`，**继承**父 tier；**判定一律用 §4.5 的同一张表**（`PC-*` 规则），不以文件名/目录命名空间判定——QC 报告实际命名是 `SPEC-0003-QC-04.md` 这类主题名，按内容里的 `parent_change_id` 判定。缺字段 → `EVIDENCE_UNBOUND`。
- 既有载体的**结构/规则**变更（schema、表头、模板、G-1…G-4 逻辑、gate paths、CI triggers、ledger/audit format、canonical 路径/owner）→ 该载体变更本身 L2。
- 追加新行或 append-only 的状态/关闭行（不改既有行内容/表头/schema）→ per-change 记录，继承父 tier；**原地改写既有行 = L2**（AC-111 的 append-only 边界，QC-04 M-4 已写入 Spec）。**唯一例外**：`release_scope` 由 release owner 在对象集变化时原地重写（`PC-RELEASE-SCOPE`），因为 Spec 要求“对象集变化必须重写并重审”。
- 台账/注册表的“更新”只允许追加状态行；`status=CLOSED` 以新行表达，旧行原样保留。
- `--print-tier-rules` 的同表输出（`TIER-RULE …`）是 QC/Reviewer 复核本节的唯一机器口径；表与实现不一致 ⇒ `TIER_MISMATCH` 停线。

---

## 5. Canonical 载体、schema 与注册（AC-109/INV-78）

### 5.1 载体清单（唯一 owner / 路径 / 格式 / 写入时机）

> Rev 1.1 起，**人类写入的三个状态字段（GV-S9/GV-S15/GV-S17）不再各自新建文件**：它们统一落在 GV-S20 审计日志的专用 `mode` 行上（单载体、append-only、可机器解析），AI 只读（Q-04）。GV-S18 与 GV-S4 共用执行器 gate record TSV（§6.12），二者字段/列不同（Q-12）。

| 载体 | 唯一写入者 | 路径 | 格式 | 写入时机 |
|---|---|---|---|---|
| GV-S3 变更输入包 | Lead（base/tier/...）；Reviewer 只追加 `head` | `docs/lead/<change_id>/change-package.md` | YAML 头 + 表格；字段 `change_id/base/head/tier/effective_tier/parent_change_id/spec_version/adr_revision/integration_owner/baseline_impact/affected_AC/allowed_paths/inherited_files/rollback/approvals` | 实施前；Review 时追加 head |
| GV-S19 `change_id` 注册表 | Lead | `docs/collab/change-id-registry.tsv` | TSV：`change_id repo base head tier status refs owner opened_at closed_at`；追加式；`refs` 可带 `parent=<id>` | 分配前；关闭追加状态行 |
| GV-S10 错分台账 | Reviewer | `docs/review-tracking.md` 的 `## Change freeze` 区 | Markdown 表：`change_id frozen_at actual_tier discovered_at discoverer action re_review_plan evidence unfrozen_at` | 发现错分/FAIL 冻结时 |
| GV-S20 治理审计日志 | 每行由该结论 owner 写（append-only） | `docs/review-tracking.md` 的 `## Governance audit log` 区 | TSV 15 列（§12.2）；更正只追加 `supersedes` | 结论产生时 |
| **GV-S15 人类批准（含 bootstrap）** | 人类指挥官（Director） | **GV-S20 行，`mode=approval\|bootstrap\|release`** | 固定 `evidence` 键：`approver=<actor>;date=<YYYY-MM-DD>;expiry=<YYYY-MM-DD\|->;scope=<范围>;affected_ac=<AC 列表\|->;affected_inv=<INV 列表\|->`；`digest=sha1:<被批准文件>` | 书面批准产生时；bootstrap 的 `expiry` 必填（AC-112） |
| **GV-S17 分级裁决** | 人类指挥官（Director） | **GV-S20 行，`mode=tier`** | `verdict=CONFIRM\|UPGRADE_L2`；`evidence` 固定键：`reason=<理由>;date=<YYYY-MM-DD>;gv_s1_tier=<L0\|L1\|L2>;decided_for=<change_id>` | MISMATCH/NEEDS_HUMAN 裁决时；无裁决即无此行 |
| **GV-S9 豁免** | 人类指挥官（Director） | **GV-S20 行，`mode=waiver`** | `verdict=GRANTED\|REVOKED`；`evidence` 固定键：`waiver_id=;approver=;date=;expiry=;scope=;affected_inv=;affected_ac=;alternative_control=;revoked_at=<-\|date>;status=<GRANTED\|REVOKED>`；撤销/到期用新行 + `supersedes` | 书面豁免时；撤销/到期时追加 |
| GV-S21 独立复核记录 | Independent Verifier | `docs/collab/gate-runs/<change_id>/<base>-<head>.md` | Markdown/YAML 头：`verifier/designated_by/independence_evidence/timestamp/base/head/baseline_result/post_change_result/baseline_digest/post_change_digest/diff_conclusion/GATE_NOT_RUN?`（两个 digest = `gate_self_digest`，§6.8）；`g4_status ∈ {PASS, NOT-RUN(not-implemented-at-BASE), NA}`（v1.4 M-3）；bootstrap 另加 `ac_scope_evaluated/ac_scope_excluded/scope_filter_command/base_tree_path/base_tree_digest/base_gate_summary/range`（§7.3） | 门禁/CI 变更复核时 |
| GV-S22 Release 对象集 | release owner | `docs/lead/releases/<release_id>/scope.md` | YAML/表格：`release_id/release_owner/designated_by/designation_evidence/change_id/parent_change_id/specs/adrs/versions/digests/baseline_impact` | 全量 QC 前；对象集变化由同一 owner 原地重写 |
| **GV-S4/GV-S18 门禁运行记录** | 门禁执行器（内容）；Lead/Reviewer（提交） | `docs/lead/<change_id>/gate-runs/<base7>-<head7>-<scope>.tsv` | TSV，表头与列见 §6.12；GV-S18 = `release_block_reason` 列 | 每次 review/release 运行结束时（原子写） |
| **GV-S20 待追加行工件** | 门禁执行器（内容）；CI 上传；Lead/Reviewer 提交 | `--audit-row-out=<path>` 指向的 CI artifact（仓库外或 run 临时目录） | GV-S20 同一行格式（§12.2），单行 TSV | review/release 运行结束时；提交后成为审计行（§6.12） |
| GV-S8 AC 台账 | Reviewer | `docs/collab/ac-coverage.tsv` | 8 列（现有 schema）：`ac owner expiry reason thread test_type evidence_path status` | 状态变化/证据登记时 |
| AC 基线 | Reviewer | `docs/collab/ac-baseline.txt` | 一行一个 `AC-NN`，升序、连续、只增不减 | 新 AC 登记时 |
| Spec 索引 | Product Owner | `docs/specs/README.md` | 索引表 + `## 13. 变更记录` | Spec 批准/升版时 |
| plan/tasks/evidence-matrix 模板 | System Architect | `.specify/templates/plan-template.md`、`tasks-template.md`、`evidence-matrix-template.md` | Markdown；`evidence-matrix-template.md` 当前缺失 | P0.5 前 |

### 5.2 G-1 `MANAGED` 扩展（AC-109）

`MANAGED` 从 `str -> str` 扩展为 `str -> {owner, append_only_writers, content_producer, tier}`，并改**最长前缀匹配**（现状 `scripts/collab_gates.py:161` 按插入序取首个匹配，新增条目后会产生误判；Q-13 校正行号）。新增条目至少：

| 前缀 | owner（可提交者/角色） | append-only co-writers | 备注 |
|---|---|---|---|
| `docs/lead/releases/` | **release owner**（Director 或其书面指定者） | — | **必须长于** `docs/lead/` 才能被最长前缀选中；`scope.md` 允许同一 owner 原地重写（Q-06） |
| `docs/lead/` | Lead | — | change-package/plan/tasks/evidence；其中 `docs/lead/<change_id>/gate-runs/**` 的内容产出者是 `gates-executor`，提交者仍是 Lead（Reviewer 可追加复跑记录），执行器不提交（§6.12）。**MANAGED 的 key 必须是字面前缀**，故不为 `<change_id>` 另设条目 |
| `docs/collab/qc-reports/` | Quality Checker | — | 主题命名（`SPEC-0003-QC-04.md`）不受限制 |
| `docs/collab/gate-runs/` | Independent Verifier | — | GV-S21；Lead 只登记/指针 |
| `docs/collab/ac-baseline.txt` / `ac-coverage.tsv` | Reviewer | — | 只追加行 |
| `docs/collab/change-id-registry.tsv` | Lead | — | 只追加行 |
| `docs/review-tracking.md` | Lead（建区/指针） | Reviewer（GV-S10 行）；Quality Checker / System Architect / Independent Verifier / Lead / Developer（各自 GV-S20 行）；**人类（Director）写 `mode ∈ {tier,waiver,approval,bootstrap,release}` 的行** | 共享 append-only 区；按行归属 |
| `.github/workflows/collab-gates.yml` | Developer | — | 治理门禁独立 workflow（§8.1） |
| `prompt/` | System Architect | — | |
| `.specify/templates/` | System Architect | — | |
| `scripts/` | Developer | — | |
| `.github/` | Developer | — | |
| `Makefile` | Developer | — | |

- **per-change 继承**：§4.5 的 `PC-*` 表是唯一 tier 判定口径；`MANAGED` 只回答“谁可写/是否 append-only”，不回答 tier。
- **append-only co-writer** 仅在 diff 只增加行、不删除/改写既有行时被接受；删除/改写要求 owner 角色（机器可判：`git diff -U0` 无 `^-` 行）。
- **角色名**：`ROLE_ALIASES` 增加 `Independent Verifier`/`IV`、`Release owner`、`Director`、`Quality Checker`/`QC`、**`gates-executor`（映射 `Gate Executor`，机器角色）**（Q-14）。
- **HUMAN_ONLY_MODES = {tier, waiver, approval, bootstrap, release}**：这些 mode 的 GV-S20 行 `actor` 必须是人类（`HUMAN_ACTORS` 数据表：Director 及其书面授权的具名人类），**`actor=gates-executor` 或其他非人类 actor 出现在这些 mode ⇒ 拒绝（`EVIDENCE_UNBOUND`；waiver 另记 `WAIVER_INVALID`）**。执行器只产出 `mode=gate` 行的**工件**，提交行的 `actor` 是 Reviewer/release owner，且 `evidence` 必须含 `produced_by=gates-executor`（§12.2）。
- 未登记路径首次写入 → `EVIDENCE_UNBOUND`（AC-109）；新建载体按 AC-97 定 L2。
- `MANAGED` 的**数据**是 canonical registry；其**代码**由 Developer 按本 ADR 实现，不得由单人顺手改绿（ADR-0006 §6）。

### 5.3 `change_id` 注册表与全 ref 唯一（AC-93/103）

- 分配前 Lead 用 `git for-each-ref` + `git reflog --all` + `git log --all --grep=<id>` 检索全 ref（含已删除分支 reflog），再把检索证据写入 GV-S20，然后追加注册表行。
- 注册表缺失、写入者不是 Lead、或缺 `repo/base/head/tier/status/refs` → `CHANGE_ID_REUSED`/`EVIDENCE_UNBOUND`。
- G-1 在 G-1 扩展上线前做人工核对；上线后读注册表 + 全 ref（§6.11）。
- 关闭：追加 `status=CLOSED`、`closed_at` 非空的新行；**不得覆盖历史行**。

### 5.4 `release_scope`（AC-105/110；Q-06）

- release owner = Director 本人或 Director 书面指定的 integration owner；GV-S22 记录 `designated_by/designation_evidence`（Director 本人：`Director` / `direct`）。
- 对象集 = 本次 Release 涉及的全部 Spec/ADR 的当前版本 + digest + 受影响基线。
- **`--release-scope=<path>` 的机器校验**（release scope 的**必需**参数，不是可选）：

| 校验 | 判据（一律从被审 head 读） | 不满足时 |
|---|---|---|
| 存在性 | `git cat-file -e <head>:<path>` 成功，且路径在 `docs/lead/releases/` 前缀下 | `QC_FULL_REVIEW_REQUIRED`，GV-S18 记 `RELEASE_BLOCKED` |
| 必填字段 | `release_id/release_owner/designated_by/designation_evidence/change_id/parent_change_id/specs/adrs/versions/digests/baseline_impact` 全部非空 | `EVIDENCE_UNBOUND` + 阻断 |
| 授权链 | `designated_by` 与 `designation_evidence` 同时存在；`release_owner` ≠ QC/Reviewer/执行器 | `QC_FULL_REVIEW_REQUIRED` |
| 版本一致 | `versions` 中每个 Spec/ADR 的版本 = 该对象在被审 head 的 version 字段 | `DIGEST_MISMATCH`（Spec 的“对象版本/摘要与当前不一致”口径）+ 阻断 |
| 摘要一致 | `digests` 中每个 `sha1` = `sha1(git show <head>:<object>)`（原始字节，§6.9） | `DIGEST_MISMATCH` + 阻断 |
| 陈旧判定 | 对象集任一 version/digest 与当前 head 不符，或 `change_id` 不在 GV-S19 的 OPEN 行 | `QC_FULL_REVIEW_REQUIRED`；旧全量 QC 结论作废 |
| 写入者 | 提交者 = `release_owner`（MANAGED owner 校验，§5.2） | `EVIDENCE_UNBOUND` |

- **release scope 缺 `--release-scope` ⇒ 直接阻断**（退出 2 的 usage 错误，GV-S18 记 `RELEASE_BLOCKED`）；review scope 不要求该参数，但若提供则执行同样校验（避免“预检通过、release 换文件”）。
- 对象集变化 → release owner 重写同一份 `scope.md` 并重跑全量 QC；旧结论作废。允许该 owner 原地重写本文件（§4.8 的 `PC-RELEASE-SCOPE` 例外）。
- QC/Reviewer 不得代写；无 `designated_by/designation_evidence` 不得开工。
- 校验失败一律在 GV-S18 落**一个**码（优先级 `DIGEST_MISMATCH` > `QC_FULL_REVIEW_REQUIRED` > `EVIDENCE_UNBOUND`），`RELEASE_ELIGIBLE=false`，退出 1（缺参为 2）。

### 5.5 证据工件（AC-99/101/95）

- 证据工件是**可见、可复核的仓库文件**（命令日志、静态检查输出、人工检查单），放在该变更自己的证据目录（如 `docs/lead/<change_id>/evidence/`），属 per-change 记录，继承父 tier。
- 台账行 `evidence_path` 必须指向该工件的 repo 相对路径且存在于被审 head；`-`/空值不算。
- **Reviewer 签名与摘要不进台账列**：由 GV-S20 审计行承载（`actor=<Reviewer>`、`evidence` 含 `ac=AC-0NN;test_type=...;base=...;head=...`、`digest=sha1:<证据工件>`），避免修改 8 列 schema 或制造第二台账。
- 这是本 ADR 对 Spec 未明确处的**设计选择**（§16 的 Q-02 已 CLOSED），不新增第二写入者：台账状态归 Reviewer，签名归 Reviewer，摘要归 Reviewer（同 actor、不同字段/载体）。

---

## 6. 门禁执行器 G-1…G-4 扩展（AC-69…74/99/100/102）

### 6.1 调用接口

```text
scripts/check-collab-gates.sh [--mode=warn|strict] \
  [--scope=local|review|release] \
  [--range=<base>..<head>] \
  [--change-id=<id>] \
  [--release-scope=<path>] \
  [--record-out=<path>] \
  [--audit-row-out=<path>] \
  [--repo=<path>] [--staged --role=...]
```

- `--scope=local`（默认）：允许默认范围、允许 warn；只作本地提示，**不得**用于 Review/Release 结论。
- `--scope=review|release`：必须 `--mode=strict`（warn 一律拒绝，退出 2）；必须显式 `--range`、`--change-id`、`--record-out`、`--audit-row-out`；`release` 另必须 `--release-scope`（§5.4）。
- **删除 `--audit-append`**（Rev 1.0 的可选追加开关）：改为 `--audit-row-out=<path>` 只产“待追加行”工件，执行器**从不写** `docs/review-tracking.md`，提交由 Lead/Reviewer 完成（§6.12；Q-05/Q-12）。
- **`--repo` 必须绑定被审工作树**（Q-20）：review/release 下 `--repo` 指向的工作树必须包含 range 的 head（`git -C <repo> cat-file -e <head>^{commit}` 成功且 head 是该工作树 HEAD 或其祖先，祖先规则见 §6.8），执行器回显 `REPO <toplevel>` 并把 `git rev-parse --show-toplevel` 的绝对路径与 head 写入 gate record 与 GV-S4；`--repo` 指向不含该 head 的仓库 ⇒ `GATE_ERROR`（退出 2）。
- 保留 ADR-0006 Amd1 的 `--release` 作为 `--scope=release` 的别名；新增 `--review` 别名。
- wrapper（`scripts/check-collab-gates.sh`）只剥离 `--mode` 的行为保留；其余参数透传。
- Makefile 新增 `RANGE`/`SCOPE`/`RELEASE_SCOPE`/`RECORD_OUT`/`AUDIT_ROW_OUT` 透传；`make verify` 仍只挂 `gates`（warn）；`gates-strict` 默认 `--scope=local --mode=strict`，CI 显式传 `SCOPE=review` 与记录参数（§8.1；Q-19）。

### 6.2 输出契约与退出码（AC-69/70/71/72）

首屏固定结构（`--record-out` 写同内容 TSV）：

```text
SCAN RANGE <base>..<head> mode=<local|review|release>
RANGE_BOUND <explicit|RANGE_UNBOUND>
REPO <worktree-toplevel> head=<head7> worktree_clean=<true|false>
CHANGE <change_id> tier=<effective_tier> source=<GV-S1|GV-S17> row=<timestamp|->
G-1 PASS|FAIL|NOT-RUN <evidence>
G-2 PASS|FAIL|NOT-RUN <evidence>
G-3 PASS|FAIL|NOT-RUN <evidence>
G-4 PASS|FAIL|NOT-RUN <evidence>
AC-REF <n>/<m> (references only)
AC-VERIFIED <governance_n>/44 (evidence+signature+digest)
RELEASE_ELIGIBLE <true|false>
RELEASE_BLOCK_REASON <code|none>
AUDIT_ROW <pending|committed> <row-key|path>
SUMMARY G-0=<ok|GATE_ERROR> G-1=<n> G-2=<n> G-3=<n> G-4=<n> scope=<scope> range=<base7>..<head7> exit=<code>
```

- **G-0 的归属（Q-17）**：既存执行器在非 git 仓库打印 `G-0 FAIL <repo> is not a git repository` 并退出 2（`scripts/collab_gates.py:380-381`）。G-0 是**前置校验标签**，映射为 `GATE_ERROR`（退出 2），**不是第五个闸门**，不计入 G-1…G-4 计数；`SUMMARY` 以 `G-0=<ok|GATE_ERROR>` 单列。
- **P0a 的 G-4 输出**：P0a 执行器（legacy + G-3 治理证据扩展）只输出 G-1…G-3，**不输出也不伪造 G-4 行**；`g4_status=NOT-RUN(not-implemented-at-BASE)` 是 **IV 在 GV-S21 记录的事实**（§7.3），不是执行器输出、不是退出码。**v1.4（M-2）的 bootstrap 例外同时覆盖 AC-69 的“G-1…G-4 四项完整输出要求”与 `GATE_INCOMPLETE` 规则**，且只豁免“G-4 尚未实现”这一条事实；其余缺口仍按 AC-69 处理。四闸门完整性（缺一 ⇒ `GATE_INCOMPLETE`）自 **P0b** 起对完整 AC 集执行。

| 退出码 | 语义 | 是否阻断 |
|---|---|---|
| 0 | 本次 scope 要求的闸门全部 PASS，且无阻断码 | 否（**0 不等于 bootstrap PASS、不等于 Release PASS**） |
| 1 | 任一闸门 FAIL（违规）或存在阻断码 | 是 |
| 2 | `GATE_ERROR`（G-0、usage、git/IO、`--repo` 不匹配、HEAD 漂移重试失败） | 是 |
| 3 | `GATE_INCOMPLETE`（闸门缺失/未执行/未输出、CI 路径缺失；**P0b 之后**含 G-4） | 是 |
| 4 | `NOT-RUN`（闸门已实现但治理证据未 VERIFIED 等） | 阻断 Release；不得当 PASS |
| 124 | wrapper `timeout` 命中 → `GATE_TIMEOUT` | 是 |

**同一事实只允许一个码/记录（Q-10；与 v1.4 的 bootstrap carve-out 对齐）**：

| 事实 | 码/记录 | 退出 |
|---|---|---|
| G-1/G-2/G-3/G-4 未实现、未输出、未执行（**P0b 之后**） | `GATE_INCOMPLETE` | 3 |
| G-4 未实现，**且处于 bootstrap 窗口**（P0a/P0b，v1.4 例外） | 执行器不打码（不输出 G-4 行）；IV 在 GV-S21 记 `G-4 NOT-RUN(not-implemented-at-BASE)`；`RELEASE_ELIGIBLE=false` | 运行按实际结果（scoped 0/1）；bootstrap 结论永不计 PASS |
| 闸门已实现、运行成功，但治理 AC 无 `VERIFIED` 证据 | `NOT-RUN` | 4 |
| 执行器崩溃/用法错误/`--repo` 不匹配 | `GATE_ERROR` | 2 |
| 运行超时 | `GATE_TIMEOUT` | 124 |
| CI 未触发/被停用/路径被删 | `GATE_NOT_RUN`（IV 发，GV-S21） | — |

- **四闸门缺一即 `GATE_INCOMPLETE`**（P0b 起）；空输出、`mode=warn` 零退出都不能算 PASS。
- 任何 NOT-RUN 使 `RELEASE_ELIGIBLE=false`；Review/Release scope 要求四闸门全 PASS，否则不得出 Review 结论。
- 输出自检：出现 `AC coverage`/`覆盖率`/`验收率` 即 `AC_SEMANTICS_INVALID`（退出 1）。
- 本 ADR 不新增门禁类型；Release 的额外校验是 **G-1/G-3/G-4 内的检查项**，不是 G-5。

### 6.3 Review vs Release（AC-70）

| 项 | review | release |
|---|---|---|
| 显式 range | 必需 | 必需 |
| `change-package` base/head 一致 | 必需 | 必需 |
| `--repo` 绑定被审工作树（§6.4） | 必需 | 必需 |
| 四闸门 PASS | 必需 | 必需 |
| 治理 AC 全 `VERIFIED` | 不要求（但不得自称 AC-VERIFIED） | **要求 44/44** |
| `release_scope` + 全量 QC 对象集 | 不要求（提供即校验） | **必需**，机器校验（§5.4） |
| GV-S20 追加行（`--audit-row-out` 产出且已提交） | 必需（缺失 ⇒ `EVIDENCE_UNBOUND`） | 必需（缺失 ⇒ `EVIDENCE_UNBOUND`） |
| 延期项 | 只报告 | 受影响范围必须 `closed` 或人类豁免 |
| 输出 | 只出 gate 结论 + `RELEASE_ELIGIBLE=false` | 只出阻断码；**永不写 `release_verdict`** |

- 门禁只拥有 GV-S4/GV-S18；Reviewer 的 `review_verdict`（GV-S5）与人类 `release_verdict`（GV-S6）单独写，机器不得代写。
- scope 未声明/无法判定 → fail-closed 为 `RELEASE_BLOCKED`（AC-70）。
- **release 模式缺 `--release-scope`** ⇒ usage 错误（退出 2，GV-S18 记 `RELEASE_BLOCKED`）；提供但校验失败 ⇒ 按 §5.4 的码阻断（退出 1）。
- **GV-S20 追加不可选**（Q-05）：review/release 的 gate 结论只有在“本次运行的待追加行已被 Lead/Reviewer 提交进 head 的 `Governance audit log`”后才可被引用；执行器自己从不写审计日志（§6.12）。

### 6.4 显式 range、repo 绑定与 HEAD 漂移（AC-71）

- `--range` 回显并写入记录；与 change-package `base/head` 不一致 → `RANGE_UNBOUND`（退出 1，不用于结论）。
- 未提供 range 的 `review`/`release` → `RANGE_UNBOUND` + 退出 1；`local` 默认范围打印 `RANGE_BOUND RANGE_UNBOUND (local default; not for review/release)`。
- **`--repo` 绑定**（Q-20）：`repo = Path(args.repo).resolve()` 必须是包含被审 head 的工作树；`git -C <repo> rev-parse --show-toplevel` 与 `git -C <repo> rev-parse HEAD` 写入 gate record 的 `repo_path`/`repo_head` 列与 stdout `REPO` 行。`--repo` 指向不含 head 的仓库、或 `--repo` 不是工作树根 ⇒ `GATE_ERROR`（退出 2），不得静默以该仓库的 head 生成记录。
- **head 祖先规则（审计行/IV 记录的提交窗口）**：GV-S20 的 gate 行或 GV-S21 记录的 `head` 可以是被审 head 的祖先，**当且仅当** `git diff --name-only <row_head>..<head>` ⊆ §4.5 的 per-change 载体、且该区间 `git diff -U0` 无 `^-` 行、且 `gate_self_digest` 不变。这样“记录提交后再复跑”不会自锁，也不会让记录提交改写被审内容摘要。
- 运行前后各取一次 `git rev-parse HEAD`；变化则自动重绑重跑一次（`range_rebound=true`），再次变化 → `GATE_ERROR`。旧结论不覆盖新范围，也不被后续提交继承。
- **`--range` 必须透传给 G-3 的删除/连续性检查**（Q-08）：G-3 不得回退到 `HEAD^..HEAD`（现状 `scripts/collab_gates.py:317` 的 `split_range(repo, None)` 是缺陷）。
- CI 必须传显式范围（§8.1）。

### 6.5 AC-REF 与 AC-VERIFIED（AC-72）

- 术语表只有 `AC-REF`（测试代码存在 `TestAC<NN>`/`AC-<NN>` 引用）与 `AC-VERIFIED`（指定 head 的真实执行证据 + Reviewer 语义核对 + 摘要）。
- 业务 AC-01…68 保留既有 `AC-REF`/延期语义；治理 AC-69…112 走 §6.7。
- 仅引用、SKIP、FAIL、未执行、空 `evidence_path`、`status!=VERIFIED` 一律不得 `AC-VERIFIED`。
- 输出不得出现“覆盖率/验收率/AC coverage”；违反即 `AC_SEMANTICS_INVALID`。

### 6.6 失败、错误、超时（AC-74）

| 情况 | 记录 | 退出 |
|---|---|---|
| 门禁判定 FAIL | `G-1..G-4 FAIL` | 1 |
| 执行器崩溃/git/台账不可读/HEAD 漂移 | `GATE_ERROR` + 证据 | 2 |
| `timeout` 命中 | `GATE_TIMEOUT` | 124 |
| 闸门缺失/CI 路径缺失 | `GATE_INCOMPLETE` | 3（**例外**：bootstrap 期 G-4 未实现按 §6.2 映射表记 IV 事实，不打码） |

- 恢复后在同一 `change_id + base + head + mode` 上重跑并保留两次记录；只有最新 PASS 可作结论。
- Release 模式错误/超时只写 GV-S18 `GATE_ERROR`/`GATE_TIMEOUT`，绝不写 `release_verdict`；人工口头确认不得替代重跑。
- 不提供“跳过失败继续”的开关。

### 6.7 G-3 治理证据模型（AC-99/101/102）

对 `AC-69…112` 逐条（**所有读取一律来自被审 head，禁止读工作区、禁止未跟踪文件**，Q-08）：

1. `ac-coverage.tsv`：`git show <head>:docs/collab/ac-coverage.tsv` 中存在精确 `AC-0NN` 行且为 8 列（现状 `scripts/collab_gates.py:275-296` 忽略后三列，扩展后必须读取）。
2. `status ∈ {REF,PENDING,VERIFIED,DEFERRED}`；只有 `VERIFIED` 进入通过路径。
3. `test_type ∈ {unit,integration,race,fault-injection,manual,static,script}`；治理 AC 常用 `script`/`static`/`manual`。
4. `evidence_path` 非空、非 `-`，且 `git cat-file -e <head>:<path>` 成功。**未跟踪文件不得作为 AC-REF/AC-VERIFIED 证据**（现状 `scripts/collab_gates.py:263` 的 `--others` 必须移除）。
5. GV-S20 存在审计行（从 head 的 `Governance audit log` 读）：`actor=Reviewer`、`mode=ac-verify`、`spec_version`/`adr_revision` 列非空、`evidence` 含 `ac=AC-0NN;test_type=<该行>;base=<base7>;head=<head7>`、`digest=sha1:<证据工件>`，且摘要与 head 上的工件内容一致。缺行/缺版本列 ⇒ `EVIDENCE_UNBOUND`（Q-05/Q-18）。
6. 全满足 → `AC-VERIFIED`；否则 `GOV_AC_EVIDENCE_MISSING`（表示“未验证”，不是对 FAIL 的语义豁免）。
7. **未上线扩展前**（P0a 之前）：治理 AC 一律 `NOT-RUN (G3_BOOTSTRAP_REQUIRED)`，`RELEASE_ELIGIBLE=false`。
8. **业务 AC-01…68 不走此模型**：继续按引用/延期判定（AC-99），避免追溯改写历史台账。
9. **head 绑定与 `--range` 透传**：G-3 用 `git ls-tree -r --name-only <head>` 枚举 `docs/specs/*.md`、`docs/collab/ac-baseline.txt`、`docs/collab/ac-coverage.tsv`、测试引用文件，用 `git show <head>:<path>` 读内容；baseline 删除/连续性检查必须用**传入的 `--range`**，不得回退 `HEAD^..HEAD`（现状 `scripts/collab_gates.py:317` 的 `split_range(repo, None)` 是缺陷）。同一次运行对同一 head 必须可复现；工作区未提交修改不得改变结论。

`DEFERRED` 治理 AC：只允许单条、必须有 owner/expiry/reason/thread + 人类书面豁免（GV-S9，`mode=waiver` 行，§5.1），且 **Release 前必须处置**；不得批量登记（AC-99/INV-69）。G-3 在 release scope 中对治理 AC 的 `DEFERRED` 直接 `GOV_AC_EVIDENCE_MISSING`；`waiver` 缺失/过期/撤销 ⇒ `WAIVER_INVALID`。

### 6.8 自保护（AC-100；Q-01/Q-09/Q-15）

当 range 触及 `scripts/**`、`.github/**`、`Makefile`、GV-S4/GV-S18 生成逻辑或门禁/CI 配置：

1. **基线（`BASE`）运行必须在 `BASE` 的干净投影树内进行**（§7.2 的命令）：`git worktree add --detach <tmp-base> <BASE>`，然后在该树内以显式 `--range=<BASE^>..<BASE>` 运行 legacy 执行器。**G-3 不得读取 live working tree**；投影树只含 `<BASE>` 的已跟踪内容（无未跟踪 SPEC-0003），否则 `BASE` 无效（§7.1 前置断言）。
2. **变更后运行**在可复现的 head projection（§7.2）或同一被审 head 的隔离 worktree 中进行，显式 range 与 scope。
3. **无论两次是否 PASS**，IV 必须逐行复核门禁/CI diff，写 GV-S21（两次结果、双摘要、`diff_conclusion`、`GATE_NOT_RUN?`；bootstrap 另加 `base_tree_path/base_tree_digest/base_gate_summary/range`，§7.3）。
4. 结果不一致、任一失败、或 diff 发现未来路径/条件弱化（即使当前 range 未触发）→ 绑定双摘要并阻断（`GATE_NOT_RUN`/`EVIDENCE_UNBOUND`）。
5. **diff 判定只归 IV**：IV 在 GV-S21 写 `diff_conclusion=FAIL` 时，G-1 读该字段并阻断（`GATE_INCOMPLETE`/`GATE_NOT_RUN`）。Rev 1.0 的“执行器若发现自己的 diff 把自身改成恒 PASS…”属自证条款，**已删除**（Q-15）；执行器不判定自己的 diff。
6. 被审 diff 内的执行器不得自证；IV 的 actor 必须 ≠ 变更作者/Lead。
7. bootstrap 期间用 AC-112 的 scoped 评估替代“两次完整门禁”（§7），但 **P0b 出口必须另有一次 review-scope 运行或等价负向夹具**（Q-09）：
   - (a) 在 IV 记录提交后的同一 range 上跑一次 `--scope=review`，证明 G-1 的 GV-S21 校验真的执行（记录存在 ⇒ PASS；删除/篡改记录 ⇒ 失败）；
   - (b) 或由 IV 拥有的独立测试夹具复现 `GATE_NOT_RUN`（无 GV-S21）与 `EVIDENCE_UNBOUND`（摘要不符）两种失败。
   - 二者至少一项为 P0b 出口证据；缺此项则 AC-100/AC-107 不得标 VERIFIED（自保护机制不得在诞生当次从未被执行）。
   - **预期退出码与证据取值（N-7/M-7）**：bootstrap 期治理 AC 必然未 `VERIFIED`（G-3 = `NOT-RUN`），故 (a) 的 review-scope 自检**预期整体退出码 = 4**（§6.2 退出码表：`NOT-RUN` 阻断 Release）。该自检的证据 = stdout 中的 `G-1 PASS|FAIL` 行与 `GATE_NOT_RUN`/`EVIDENCE_UNBOUND` 负向结果，**不得**以其 `exit 0` 作为任何 PASS 证据（review-scope 在 bootstrap 期不可能 exit 0），也**不得**把 `exit 4` 本身当自检失败或当 bootstrap PASS；IV 必须把这两点写入 GV-S21 的 `post_change_result` 与说明。

**摘要口径**：`baseline_digest`/`post_change_digest` = `gate_self_digest`，即对门禁实现字节集（`scripts/collab_gates.py`、`scripts/check-collab-gates.sh`、`.github/workflows/**`、`Makefile` 中门禁接线）按固定顺序拼接 `sha1(file)` 再取 `sha1`；**不含整棵 diff、不含 GV-S21 记录/GV-S20 行/`review-tracking.md` 自身**。这样 IV/Reviewer 提交复核记录与审计行都不会改变摘要，避免“记录永远匹配不上”的死锁。`base_tree_digest` 另算：`git rev-parse <BASE>^{tree}`（§7.3）。

**检查范围与时序**：G-1 的 GV-S21 检查只在 `scope=review|release` 生效（`local` 不要求 IV；bootstrap 的 scoped 评估用 `scope=local`，§7.2）。判定：存在本 `change_id` 的 GV-S21 记录，`baseline_digest == sha1(BASE gate files)`、`post_change_digest == 当前 gate_self_digest`、`diff_conclusion=PASS`、`GATE_NOT_RUN?=false`；记录的 `head` 可以是当前 head 的祖先，条件见 §6.4 的 head 祖先规则（只追加 per-change 记录 + `gate_self_digest` 不变）。顺序：基线运行 → 变更后运行 → IV 写记录并提交 → 最终被审 head 上复跑 → G-1 PASS。无记录 → `GATE_NOT_RUN`；记录存在但摘要/结论不匹配 → `EVIDENCE_UNBOUND`。

### 6.9 内容摘要（AC-102/95）

- `content_digest`：对单个文件原始字节 `sha1`（与 QC-04 的 `15ad2979…` 口径一致），格式 `sha1:<40hex>`。
- `diff_digest`：`git diff --no-color --binary --full-index <base>..<head> | sha1sum`。
- `gate_self_digest`：门禁实现字节集的摘要（§6.8），只用于 GV-S21 的基线与变更后比较。
- QC 报告、Review verdict、Release verdict、gate record、GV-S20 行都记录 `spec_version + adr_revision + content_digest + diff_digest + base/head`（Q-18：`spec_version`/`adr_revision` 是 GV-S20 的正式列，见 §12.2）。
- 同版本不同摘要 → `DIGEST_MISMATCH`：旧 QC/Review/Release 全部失效，按新版本重审（G-2 的机器落点见 §6.11）。

### 6.10 Independent Verifier（AC-107；IV 人选已裁决）

- **IV = Director 本人，或 Director 在每次运行前书面指定的具名 actor**（`docs/review-tracking.md` 的 “2026-11-24 human decisions: GOV-COST-01 bootstrap, IV and commit timing”）。Director 本人：`designated_by=Director`、`independence_evidence=direct`；书面指定：`designated_by=<Director>`、`independence_evidence=<书面指定记录路径>@sha1:<digest>`。无指定/无独立性证据 = 无 IV。
- IV actor ≠ 变更作者/Lead；唯一证据载体 `docs/collab/gate-runs/<change_id>/<base>-<head>.md`。
- IV 记录字段按 GV-S21（含 `GATE_NOT_RUN?`）；bootstrap scoped 运行另记 `ac_scope_evaluated/ac_scope_excluded/scope_filter_command` 与 `base_tree_path/base_tree_digest/base_gate_summary/range`（§7.3）。
- 外部检测者发现门禁未运行，由同一 IV 角色发 `GATE_NOT_RUN`；不得由被审 diff 内执行器/作者充当。
- IV 写记录；Lead 只登记/指针（§5.2）；不得互相代写。
- **每次运行都必须有一次指定留痕**（逐次指定，不是一次性授权）：同一 change 的多次 IV 复核各自记录 `designated_by`/`independence_evidence`；缺任一次 ⇒ 该次复核无效（`GATE_NOT_RUN`）。

### 6.11 各闸门检查项（职责边界）

| 闸门 | 保留（ADR-0006） | 新增/扩展 | 不做的语义判断 |
|---|---|---|---|
| G-1 所有权/范围 | 受管路径 owner、Role/Exception、`change_id/Actor/Reviewed-by`、`Actor≠Reviewed-by` | 显式 range 绑定、`change-package` base/head 一致、`--repo` 工作树绑定（§6.4）、`effective_tier` 读取与回显（§4.5）、路径下限 `TIER_MISMATCH`、per-change 记录内容判定（§4.5 表）、L1 联签字段/顺序/摘要（含 `verdict_at`，§9）、L2 批准摘要存在性、`change_id` 注册表、CI 治理 workflow 的三目录触发（§8.1）、门禁/CI 变更的 GV-S21 引用（`diff_conclusion`）、GV-S20 gate 行存在性（review/release）、最长前缀 MANAGED、`HUMAN_ONLY_MODES` 的 actor 校验、评审报告/GV-S20 的 `parent_change_id` | 架构是否合理、级别是否“真的”L0/L1（Reviewer/人类） |
| G-2 契约版本 | Spec version/README 索引/`## 13. 变更记录`、baseline 删除保护 | 版本解析扩为 `MAJOR.MINOR[.PATCH]` 并按元组比较；**同版本内容摘要比较**（base/head **两端都存在**该路径且 `SPEC`/`ADR` blob `sha1` 不同即 `DIGEST_MISMATCH`；base 无该 blob ⇒ 走“新增契约”分支，见下方 N-4）；`Non-normative: true` 豁免**只允许 L0 非治理文档**，治理契约路径出现该 trailer ⇒ G-2 FAIL（§6.11 下方细化）；索引同格式、`adr` 回填只报告不阻断；**ADR 的版本基准字段 = `Revision`（`Rev X.Y`）**，Spec 的版本基准字段 = YAML `version:` | ADR Amendment 语义、业务规则正确性 |
| G-3 AC | AC 编号/引用/延期字段、baseline 连续 | 治理证据模型（§6.7）、`AC-REF`/`AC-VERIFIED` 措辞、`GOV_AC_EVIDENCE_MISSING`、输出自检、治理 AC 不参与业务引用检查；**全部读取走 `git show <head>:`/`git ls-tree <head>`，`--range` 透传，未跟踪文件不算证据** | 测试是否执行、断言是否充分 |
| G-4 到期/债务 | 台账/偏差登记 expiry、续期理由、批量续期 >3、`status=closed` | 实现 ADR-0006 Amd1 §2（当前完全缺失）；**输入 = GV-S20 的 `mode=waiver` 行（GV-S9：`expiry/scope/affected_inv/affected_ac/revoked_at/status`）与 `mode=approval\|bootstrap\|release` 行（GV-S15：`expiry`）**；bootstrap 有效期、Release 受影响延期处置检查 | 延期是否合理、残余风险是否可接受 |

**G-2 的同版本摘要与 `Non-normative` 边界（Q-07）**：

- 对 `docs/specs/**`、`docs/adr/**`、`AGENTS.md`、`docs/collab/CONVENTIONS.md`、`.specify/memory/**` 这些**治理契约**：同一版本号下、且 **base 与 head 均存在该路径**时，`git rev-parse <base>:<path>` 与 `<head>:<path>` 的 blob 不同 ⇒ `DIGEST_MISMATCH`（G-2 FAIL，退出 1），不得以版本号未变放行；版本递增时按元组比较并检查 README 索引/`## 13. 变更记录`。base 无该路径（新增文件）不适用本句，见下方 N-4。
- `Non-normative: true` 豁免**只允许**在 range 内全部变更文件都属于 AC-76 (a) 的**非治理文档**（不含 `docs/specs/**`、`docs/adr/**`、`AGENTS.md`、`docs/collab/CONVENTIONS.md`、`docs/review-tracking.md`、`docs/collab/*.tsv|txt`、`.specify/**`、`prompt/**`、`scripts/**`、`.github/**`、`Makefile`）时生效；任一治理契约路径命中该 trailer ⇒ G-2 FAIL（消息含 `non-normative trailer on governance path`），不新增阻塞码。
- 该豁免的判据与 AC-76 (a) 共用一个 L0 路径集合，避免“声明 non-normative 即免版本递增”的绕过面。

**G-2 的新增文件与 ADR `Revision` 字段（N-4/M-4）**：

- **新增文件（base 无该 blob）**：同版本摘要比较只适用于 base 与 head **都存在**的路径。`git rev-parse <base>:<path>` 失败（base 无该 blob）⇒ 走“**新增契约**”分支：README 索引 / `## 13. 变更记录` / 版本链校验（与 `scripts/collab_gates.py:212-224` 对新增 Spec 的既有分支同口径），**不做**同版本 blob 比较、**不报** `DIGEST_MISMATCH`。本变更自身就新增 `docs/adr/0008-governance-cost-reduction.md`，P0.5 还会新增注册表/模板；按字面比较会把“新增治理契约”误判为 `DIGEST_MISMATCH`。
- **ADR 的版本基准字段**：`docs/adr/**` **没有** `version:` 字段，其版本基准是头部的 **`Revision: Rev X.Y`**（本文件为 `Rev 1.1`）。G-2 的字段表、§6.9 的摘要绑定与 §12.2 的 `adr_revision` 列一律以 `Revision` 为准；Spec 仍用 YAML `version:`（两段/三段）。新增 ADR 的版本链校验 = 头部 `Revision` 非空 + README/`## 13. 变更记录` 引用一致（只报告不阻断的部分保持原样）。
- **已有路径的增删边界**：base 有、head 无（删除治理契约）不属于本节豁免，按 G-1 受管路径与 AC-102（版本/摘要绑定）处置；不得借“新增文件”分支规避删除保护。

**G-4 的输入与失效行为（Q-04）**：

| 输入（从被审 head 的 GV-S20 读） | 缺失 | 过期/撤销 |
|---|---|---|
| `mode=waiver` 行（GV-S9） | 引用该豁免的结论 ⇒ `WAIVER_INVALID` | `WAIVER_INVALID`，按更严格规则重走 |
| `mode=approval\|bootstrap\|release` 行（GV-S15） | bootstrap 期 ⇒ `G3_BOOTSTRAP_REQUIRED`；Release 期 ⇒ `RELEASE_BLOCKED` | 到期/缺失 `expiry` ⇒ 同左，且恢复严格门禁 |
| `mode=tier` 行（GV-S17，G-1 用） | 视为未升级（非错误） | 不适用（无 expiry；多行冲突 ⇒ `EVIDENCE_UNBOUND` + L2） |

> **不新增 G-5/G-6**：Release 的额外校验落在 G-1/G-3/G-4；语义判断仍归 Reviewer/人类（CONVENTIONS §7.2 的机器能力边界）。

### 6.12 记录、GV-S4/GV-S18 与 GV-S20 的落盘（单写者）

- **执行器的写入边界（修正 Rev 1.0 的“只读、从不改仓库”自述，Q-12）**：执行器**不是**全只读——它只写两类路径，其余一律不写：
  1. `--record-out=<path>`：非 CI 运行时必须位于 `docs/lead/<change_id>/gate-runs/` 下；CI 运行（环境 `CI=true`）允许写入 `$RUNNER_TEMP/gate-runs/<change_id>/…`（仓库外），由 Lead/Reviewer 以**同一字节**提交到 canonical 路径，摘要写入 gate record；两种情况都原子写（temp+rename）；
  2. `--audit-row-out=<path>`：必须在仓库工作树之外（CI artifact 目录或临时目录）；若指到仓库内 ⇒ usage 错误（退出 2）。
  执行器**不写** `docs/review-tracking.md`、不改 git 历史/索引/分支、不写业务文件、不写台账。违反 ⇒ `GATE_ERROR`（退出 2），该次运行不得作为结论。
- **gate record（GV-S4 + GV-S18）格式**（`docs/lead/<change_id>/gate-runs/<base7>-<head7>-<scope>.tsv`，TSV，表头固定）：

  ```text
  timestamp	change_id	parent_change_id	effective_tier	tier_source	scope	base	head	repo_path	repo_head	range_bound	gate	verdict	release_block_reason	evidence	digest	audit_row
  ```

  - 行：`G-1`…`G-4` 各一行（`gate=G-…`、`verdict=PASS|FAIL|NOT-RUN`、`evidence`/`digest` 对应）；末行 `gate=SUMMARY`，承载 `release_block_reason`（GV-S18）与 `audit_row=<row-key|pending>`。
  - **GV-S18 的语义 = `release_block_reason` 列**：取值只用 Spec §5.2 的码（`GATE_NOT_RUN`/`GATE_INCOMPLETE`/`GATE_ERROR`/`GATE_TIMEOUT`/`G3_BOOTSTRAP_REQUIRED`/`RELEASE_BLOCKED`）；写入时机 = 运行结束（与记录同一次原子写）；读取方 = Reviewer/Release 授权人/IV；写失败 ⇒ `GATE_ERROR` 且不得出结论。它与人类 `release_verdict`（GV-S6）不同载体、不同写入者。
- **GV-S20 追加在 review/release 强制（Q-05）**：每次 review/release 运行都必须产出 `--audit-row-out` 的一行（GV-S20 15 列，§12.2）；CI 上传该工件，Lead（gate 结论）或 Reviewer（review 结论）把它追加到 `docs/review-tracking.md` 的 `Governance audit log` 并提交。**G-1 在 review/release 核对**：被审 head 中存在该 `change_id + base + head + mode=gate` 的 audit 行、`digest` 与 gate record/`gate_self_digest` 一致、`evidence` 含 `produced_by=gates-executor`；缺失 ⇒ `EVIDENCE_UNBOUND`（AC-95 的“缺追加记录即 EVIDENCE_UNBOUND”）。**不存在“可选 append”模式**。
- **head 口径（N-6/M-6）**：上句的“head”是“**被审 head，或按 §6.4 祖先规则允许的祖先 head**”——行记录的 `head` 可以是祖先，当且仅当 `<row_head>..<被审 head>` 只追加 per-change 载体且 `gate_self_digest` 不变（§6.4）。按字面要求“精确等于被审 head”会死锁（每次提交行都会产生新 head，永远要求下一行）。
- **首次运行的预期 `EVIDENCE_UNBOUND`（N-6）**：首次 review/release 运行时审计行尚未提交，`EVIDENCE_UNBOUND` 是**预期**而非缺陷；处置固定为“提交 `--audit-row-out` 工件 → 在同一 `change_id+base+head+mode` 键上重跑”，第二次运行才可能 PASS（E59）。不得把首次失败当结论，也不得为绕开它而删除该项检查或改用可选 append。
- 单写者说明：GV-S4/GV-S18 的内容产出者是执行器；GV-S20 行的 `actor` 是提交该行的结论 owner（review/release 场景为 Reviewer/release owner），`evidence` 记 `produced_by=gates-executor`；Reviewer 写 GV-S5，人类写 GV-S6，三者字段不同、载体不同。`gates-executor` 只作为 `ROLE_ALIASES` 的机器角色与 `MANAGED` 的内容产出者登记，**不得**作为任何 GV-S20 行的 `actor`（§5.2）。

---

## 7. Bootstrap P0a/P0b（AC-106/112，绑定 SPEC-0003 v1.4）

> 本节是 **v1.4 的唯一实现口径**。AC-106/AC-112 的文本（`BASE` 树 legacy **G-1…G-3 PASS** + 显式 `G-4 NOT-RUN(not-implemented-at-BASE)` + IV + 人类批准；P0b 落地完整 G-4；AC-69 `GATE_INCOMPLETE` 的豁免随 P0b 终止）**不得由 ADR 解释或改写**（Q-02）。若 v1.4 在批准前实质变化，本节按头部“重对齐条款”失效并重写。

### 7.1 阶段、`BASE` 与前置断言

| 阶段 | 内容 | 退出条件 |
|---|---|---|
| P0a | G-3 接受治理证据（`test_type/evidence_path/status=VERIFIED` + digest + Reviewer 签名）；治理 AC 仍为 NOT-RUN | ⓪ **前置**：`## Governance audit log` 区段与 `mode=bootstrap` 批准行已在首个 P0a 提交之前建立并提交（N-1，见下方前置条款）① `BASE` 树 legacy **G-1…G-3 PASS**（显式 `--range=<BASE^>..<BASE>`；scoped AC-01…AC-68）② 显式 `G-4 NOT-RUN(not-implemented-at-BASE)` 记录（IV，永不阻断、永不计 PASS）③ IV 逐行 diff + 双摘要（`gate_self_digest` + `base_tree_digest`）④ 人类书面批准（GV-S15 `mode=bootstrap`：`approver/date/expiry/scope`；该行必须已在 `BASE` 树内，见⓪）；`RELEASE_ELIGIBLE=false` |
| P0b | Release mode、显式 range、CI paths、AC-100 自保护、AC-107 IV、**完整 G-4** | ① 同一 `BASE` 树 + P0b 门禁字节集的投影上 scoped 运行四闸门 PASS（G-4 已实现）② **review-scope 运行或等价负向夹具**（§6.8 第 7 条，Q-09；预期整体退出 4，见 §7.3）③ IV 双摘要 ④ 人类批准；P0b 完成即结束 bootstrap |
| P0.5 | GV-S19/GV-S3/GV-S10/GV-S21/GV-S22 载体与 `evidence-matrix-template.md`，登记 G-1 `MANAGED`（GV-S20 区段与 `mode=bootstrap` 行已在首个 P0a 提交前建立，不属本阶段） | 载体可写且 G-1 覆盖 |
| P1 | 分级/委托（GOV-TIER-01 + GOV-HUM-01）+ AC-101 接入 | 分级矩阵可执行 |
| P2 | QC delta + scaffold（GOV-QC-01 + GOV-SCAF-01） | 全量节点/对象集可执行 |

**`BASE` 定义**：`BASE` = **第一个 P0a 提交的前一提交**，由 Lead 写入 change-package 与 GV-S20；`BASE` 不得落在 AC-69…登记之后。**`BASE` 树内必须已含 `## Governance audit log` 区段与 `mode=bootstrap` 批准行**（N-1）：`BASE` 是首个 P0a 提交的父提交，故该区段/行必须由 Lead 在此之前提交，否则 P0a 无合法批准行，且 P0b 的 scoped 投影树（= `BASE` 树）缺 G-4 的 bootstrap 有效期输入 ⇒ `G3_BOOTSTRAP_REQUIRED`，四闸门 PASS 不可能达成。

**P0a 前置（N-1/M-1，不可后置）**：`docs/review-tracking.md` 的 `## Governance audit log` 区段、以及 GV-S15 的 `mode=bootstrap` 批准行（`approver/date/expiry/scope` + `digest`）由 **Lead 在首个 P0a 提交之前**建立并提交（该提交即 `BASE` 或其祖先，且自身必须满足下方不变量 6 的 trailer 要求）。**P0.5 只建其余载体**（GV-S19/GV-S3/GV-S10/GV-S21/GV-S22 与模板），不再承担 GV-S20 区段/批准行的建立；“建立前不得首次写入”（§12.1）的时点因此前移到 P0a 之前。

**`BASE` 树不变量（Q-01：可执行性前提）**：

1. `BASE` 树**不含**治理 Spec `docs/specs/0003-platform-governance-cost-reduction.md`——已批准的 v1.4 在 P0a 之前保持未跟踪（`docs/review-tracking.md` 的 “GOV-COST-01 bootstrap, IV and commit timing”：approved SPEC-0003 remains untracked until P0a；P0a 原子落地 = 已批准 Spec + G-3 扩展 + 台账/baseline seeding）；
2. `BASE:docs/collab/ac-baseline.txt` 的末行 = `AC-68`（AC-69…112 尚未登记）；
3. `BASE` 的门禁实现是 legacy（只有 G-1/G-2/G-3，无 G-4；`scripts/collab_gates.py:2`）；
4. `BASE^` 存在（`git cat-file -e <BASE>^^{commit}`）；
5. `BASE` 投影树干净（`git -C <tree> status --porcelain` 为空），无未跟踪治理 Spec；
6. **`BASE^..BASE` 的 G-1 PASS：`BASE` 提交自身携带 `change_id`/`Actor`/`Reviewed-by` trailer（且 `Actor ≠ Reviewed-by`）、Role/owner 合规**（N-2；`scripts/collab_gates.py:131-138` 只读 `HEAD` = `BASE` 的 message trailer）。**实测**：以无 trailer 的 WIP HEAD `1f4f2d1`（QC-02 测量时的 WIP HEAD）作 `BASE`，legacy 运行得 `G-1 FAIL HEAD missing change_id/Actor/Reviewed-by trailer`、`SUMMARY G-1=1 G-2=0 G-3=0 exit=1`；以 `main`（`0790b38`，含全套 trailer）为 `BASE` 得 `G-1=0 G-2=0 G-3=0 exit=0`。

任一条不成立 ⇒ `BASE` 无效，返回 `G3_BOOTSTRAP_REQUIRED`，**不得**用“在投影里删除已批准 Spec/改写 baseline”制造假 `BASE`（那会伪造证据）。若 `BASE` 上已含 SPEC-0003（例如批准稿被提前提交），唯一合法处置是：`BASE` 前移、P0a 重排（人类书面批准 + Spec 摘要绑定），或由人类按 INV-56 升版调整；ADR 不得自行放宽。

**不变量 6 的失败处置（N-2）**：改用合规提交作为 `BASE`——即插入一个只改**非受管路径**（如 `docs/review-tracking.md` 的 `Governance audit log` 区段 + `mode=bootstrap` 行，与 N-1 的载体建立合并为同一提交）、且带全套 trailer 的“准备提交”作为 `BASE`；或前移 `BASE` 到最近的合规提交。**不得**用 head 投影、工作区 hack 或“反正 legacy `MANAGED` 不含该路径”掩盖该 FAIL（§6.4 的祖先规则不适用于 `BASE` 选择）。

**为什么 `BASE` 能 PASS（与 QC-05 Q-01 的闭合）**：G-3 现状从**工作区**读 `docs/specs/*.md` 与 `ac-baseline.txt`。在干净 `BASE` 投影树内运行，工作区 = `BASE` 的已跟踪内容：没有治理 Spec、baseline 止于 AC-68，因此 G-3 只评估 AC-01…68（与 CI 在该提交上的绿态同构），不会产生 AC-69…112 的 `not in baseline`/`no test` 噪声。**G-3 不得读取 live working tree**：运行时 cwd 与 `--repo` 必须指向投影树，IV 用 `git -C <tree> status --porcelain` 证明树干净并在 GV-S21 记录。

**bootstrap 期限与批准**：GV-S15 记 `approved_by/approved_at/scope/expiry`（`expiry` 必填），以 GV-S20 `mode=bootstrap` 行的 `evidence`（`approver/date/expiry/scope`）承载；到期自动恢复严格门禁（`G3_BOOTSTRAP_REQUIRED` + `RELEASE_BLOCKED`）。

整个 `GOV-COST-01` 在**同一 WIP 分支**上按 P0a→P0b→P0.5→P1→P2 叠加提交，**中间不 merge 到 main**；最终 head 上治理 AC 全部 VERIFIED 后才进入 Review/Release。这样避免“NOT-RUN 阻断 merge 导致无法继续实现”的悖论，也不引入 scoped bypass 开关。

### 7.2 可复现运行：`BASE` 投影树 + head 投影（`BASE` 树 + 被审门禁字节集）

`BASE` 基线运行（legacy 执行器原样，**在投影树内**，显式 range）：

```bash
BASE=<第一个 P0a 提交的前一提交>; REPO=<被审仓库>; TMP=$(mktemp -d)

# 0) 前置断言（§7.1；任一失败 ⇒ BASE 无效，G3_BOOTSTRAP_REQUIRED）
git -C "$REPO" cat-file -e "$BASE^{commit}"
git -C "$REPO" cat-file -e "$BASE^^{commit}"
! git -C "$REPO" cat-file -e "$BASE:docs/specs/0003-platform-governance-cost-reduction.md"
test "$(git -C "$REPO" show "$BASE:docs/collab/ac-baseline.txt" | tail -1)" = "AC-68"
git -C "$REPO" log -1 --format=%B "$BASE" | grep -q '^change_id:'    # 不变量 6：BASE 自带三个 trailer
git -C "$REPO" log -1 --format=%B "$BASE" | grep -q '^Actor:'
git -C "$REPO" log -1 --format=%B "$BASE" | grep -q '^Reviewed-by:'

# 1) BASE 投影树（只含 BASE 已跟踪内容；无未跟踪 SPEC-0003）
git -C "$REPO" worktree add --detach "$TMP/base" "$BASE"
test -z "$(git -C "$TMP/base" status --porcelain)"        # 干净树

# 2) 在投影树内运行 legacy 执行器，显式 range=<BASE^>..<BASE>
( cd "$TMP/base" && ./scripts/check-collab-gates.sh --mode=strict \
    --range="$BASE^..$BASE" ) | tee "$TMP/base-gate.out"

# 3) 证据字段
BASE_TREE=$(git -C "$REPO" rev-parse "$BASE^{tree}")
grep '^SUMMARY' "$TMP/base-gate.out"                       # 期望 G-1=0 G-2=0 G-3=0（仅 G-1..G-3）
```

> **`git archive` 变体不可作为 canonical**：`git archive <BASE> | tar -x -C <tmp>` 得到非 git 目录，执行器会 `G-0 FAIL ... is not a git repository`（退出 2，`scripts/collab_gates.py:380-381`）；若再 `git init` 并提交则**伪造历史**。因此 archive 只用于“只读摘要/字节比对”，canonical 运行一律用 `git worktree add --detach`（或等价的 `git clone --no-local` + `checkout --detach`）。

变更后 scoped 运行（**head 投影 = `BASE` 树 + 被审 head 的门禁实现字节集**；在一次性 clone 中构造 projection commit，不污染被审仓库，也不需要“删 Spec/覆盖 baseline”这类工作区 hack）：

```bash
REVIEWED=<被审 head>
git clone --quiet --no-local "$REPO" "$TMP/proj"
git -C "$TMP/proj" checkout --detach "$BASE"
# 只取被审 head 的门禁实现字节集（= gate_self_digest 的输入集；P0a 无 collab-gates.yml 时跳过）
for f in scripts/collab_gates.py scripts/check-collab-gates.sh Makefile .github/workflows/go.yml .github/workflows/collab-gates.yml; do
  git -C "$TMP/proj" checkout "$REVIEWED" -- "$f" 2>/dev/null || true
done
git -C "$TMP/proj" add -A
git -C "$TMP/proj" -c user.name=governance-projection -c user.email=projection@local \
  commit --quiet -m "projection: BASE + gate_self_digest($REVIEWED)" \
  --trailer "change_id: GOV-COST-01" \
  --trailer "Actor: governance-projection" \
  --trailer "Reviewed-by: <IV>" \
  --trailer "Role: Developer"
PROJ=$(git -C "$TMP/proj" rev-parse HEAD)
PROJECTION_DIFF_DIGEST=$(git -C "$TMP/proj" diff "$BASE..$PROJ" | sha1sum)

# P0a（执行器只有 G-1..G-3；legacy 不接受 --change-id/--scope/--record-out，勿传）：
( cd "$TMP/proj" && ./scripts/check-collab-gates.sh --mode=strict \
    --range="$BASE..$PROJ" --repo="$TMP/proj" )

# P0b（执行器已有 scope/record/audit-row；record 写在一次性 clone 内）：
( cd "$TMP/proj" && ./scripts/check-collab-gates.sh --scope=local --mode=strict \
    --range="$BASE..$PROJ" --change-id=GOV-COST-01 --repo="$TMP/proj" \
    --record-out="$TMP/proj/docs/lead/GOV-COST-01/gate-runs/${BASE:0:7}-${PROJ:0:7}-local.tsv" \
    --audit-row-out="$TMP/audit-row.tsv" )
```

- **投影提交必须自带自己的 trailer（N-3/M-3）**：四个 `--trailer`（`change_id`/`Actor`/`Reviewed-by`/`Role`，且 `Actor ≠ Reviewed-by`）是**必需**的，不是装饰。当前即使不加也能过 G-1，只是因为 legacy `MANAGED`（`scripts/collab_gates.py:17-28`，8 个前缀）**不含** `scripts/**`/`Makefile`/`.github/**`，故 G-1 在“无受管文件”时提前返回（`scripts/collab_gates.py:126-128`）；一旦 P0.5 把门禁路径登记进 `MANAGED` 早于 P0b，同一命令会因投影 HEAD 缺 trailer/Role 而 `G-1 FAIL`，脚本对阶段顺序敏感。**不得依赖 `MANAGED` 尚未覆盖门禁路径这一时序巧合。** 投影提交只存在于一次性 clone（`$TMP/proj`）中，不进入被审仓库，故 `Actor` 可为机器名 `<actor>`；`Reviewed-by` 填 IV 的具名 actor。
- **为什么不是“删治理 Spec + 覆盖 baseline”**：那种工作区 hack（Rev 1.0 §7.2）在 P0b 之后与 G-3 的 head 绑定读取（§6.7）冲突，且会制造“已批准 Spec 不存在”的假树。head 投影只替换门禁实现字节集，业务 Spec/台账/测试保持 `BASE`（= 已批准基线），因此 scoped 评估的对象 = “被审门禁实现 × 已批准业务基线”，结论可复现且与被审文档解耦。
- **`gate_self_digest(PROJ) == gate_self_digest(REVIEWED)`**：投影只取门禁字节集，故 IV 可把两个摘要并列写进 GV-S21，证明“被审实现”与“被评实现”是同一字节集。
- **`scope=local` 的理由**：bootstrap 证据是“scoped 评估”，不是 Review/Release 结论；Review scope 的 G-1 要求 GV-S21 记录，而该记录正是本次运行的产物（先有鸡还是先有蛋）。IV 仍必须逐行 diff，并把完整命令写入 GV-S21。P0b 的 review-scope 自检（Q-09）在 IV 记录提交后的**同一被审 range** 上另跑一次，不属于本节 scoped 证据。
- **不修改 legacy 执行器、不新增 scope 开关、不伪造 PASS**；`$TMP/proj` 是一次性 clone，被审仓库与其 git 历史、工作树不被改动。若环境无 `git worktree` 权限，退化到 `git clone --no-local` + `checkout --detach`（等价、仍不改被审仓库）。
- **P0b 首次运行 G-4 可能暴露既有延期**：G-4 读 `ac-coverage.tsv` 的延期/到期字段（ADR-0006 Amd1 §2 的完整实现）。若基线存在过期延期，按 AC-74/AC-94 处置（补证据/关闭/人类书面豁免），**不得为让 P0b 变绿而放宽 G-4**。

### 7.3 证据字段与批准（Q-01/Q-02/Q-09）

`P0a`/`P0b` 的**规范证据组合（v1.4 唯一口径）**：

| # | 证据 | 载体/字段 |
|---|---|---|
| 1 | `BASE` 树 legacy **G-1…G-3 PASS**，显式 `--range=<BASE^>..<BASE>`，scoped AC-01…AC-68 | GV-S21：`baseline_result=PASS`、`range=<BASE^>..<BASE>`、`base_gate_summary=<SUMMARY 行原文>`、`base_gate_exit=0`、`ac_scope_evaluated=AC-01…AC-68`、`ac_scope_excluded=AC-69…AC-112`、`scope_filter_command=<完整命令>` |
| 2 | 显式 **`g4_status=NOT-RUN(not-implemented-at-BASE)`**（不阻断 P0a/P0b、永不计 PASS、不授权 Release/AC-VERIFIED） | GV-S21 的**独立字段** `g4_status`（枚举 `PASS\|NOT-RUN(not-implemented-at-BASE)\|NA`，v1.4 M-3；不与 `post_change_result`/`GATE_NOT_RUN?` 混用）；P0a 执行器**不输出**该行，由 IV 依据 `SUMMARY` 只有 G-1…G-3 的事实记录；`ac_scope_excluded` 只列 AC 编号（G-4 状态不入该字段） |
| 3 | IV 逐行 diff + 双摘要 + 树摘要 | GV-S21：`baseline_digest=sha1(BASE gate files)`、`post_change_digest=gate_self_digest(REVIEWED)`、`diff_conclusion=PASS`、`base_tree_path`、`base_tree_digest=<BASE^{tree}>`、`projection_head=<PROJ>`、`projection_diff_digest`、`verifier/designated_by/independence_evidence/timestamp` |
| 4 | 人类书面批准（含 `BASE`/diff 摘要） | GV-S15：`mode=bootstrap` 行的 `approver/date/expiry/scope` + `digest` |
| 5 | 治理 AC 仍 NOT-RUN、`RELEASE_ELIGIBLE=false` | gate record / GV-S20（`mode=gate`）与 GV-S18 |
| 6 | head 投影回归（ADR 追加的自保护证据，**不替代 1–5**） | GV-S21：`post_change_result=<SUMMARY>`、`projection_head=<PROJ>`、`projection_diff_digest`、`scope_filter_command=<完整命令>` |

- **head 投影运行是 ADR 追加的自保护证据（第 6 项，非 AC-112 的规范证据、不得替代 1–4）**：证明被审门禁实现作用在已批准基线上仍然 G-1…G-3 PASS（P0b 为四闸门 PASS）。其 `SUMMARY`/exit 写 GV-S21 的 `post_change_result`，`scope_filter_command` 含上面的完整命令与 `projection_diff_digest`。
- `P0a` 的 head 投影运行**不要求**退出 4：P0a 执行器只有 G-1…G-3，scoped 集内全部 PASS 时退出 0；**退出 0 不是 bootstrap PASS**（`RELEASE_ELIGIBLE=false`，且 `G-4 NOT-RUN(not-implemented-at-BASE)` 记录存在）。**不得**把 `G-4 NOT-RUN` 记为 `GATE_INCOMPLETE`（v1.4 的 bootstrap carve-out）或记为 PASS。
- **P0b 出口另需**：一次 review-scope 运行（IV 记录提交后的同一被审 range）或 IV 拥有的等价负向夹具（§6.8 第 7 条），证明 GV-S21 缺失/摘要不符会 FAIL；缺此项 ⇒ AC-100/AC-107 不得标 VERIFIED、P0b 不得退出。**该次 review-scope 自检的预期整体退出码 = 4**（bootstrap 期治理 AC 未 `VERIFIED`）；证据取 `G-1 PASS|FAIL` 行与 `GATE_NOT_RUN`/`EVIDENCE_UNBOUND` 负向结果，**不得**以 `exit 0` 或 `exit 4` 单独作为 PASS 证据（N-7）。
- 证据由 IV/Reviewer 逐项核对；任何一项缺失 ⇒ `G3_BOOTSTRAP_REQUIRED`/`EVIDENCE_UNBOUND`，不得以口语说明补齐。

### 7.4 禁止与解除（AC-106/112）

- bootstrap 不得用于 Release 或 AC-VERIFIED；不得用未上线的新 G-3 自证；不得批量延期；不得超出 P0a/P0b 范围。
- 缺人类批准/期限、`BASE` 不满足 §7.1 不变量、范围超限 → `G3_BOOTSTRAP_REQUIRED`/`EVIDENCE_UNBOUND`。
- P0b 完成 → 关闭 bootstrap（GV-S15 状态 + GV-S20 `bootstrap_closed` 行）；此后 AC-100 的两次门禁对完整 AC 集与 G-1…G-4 全部适用；G-4 缺失/未执行/未输出 ⇒ `GATE_INCOMPLETE`（AC-69，不再豁免）。
- 扩展回滚/失效 → 治理 AC 回到 NOT-RUN 并阻断 Release；已引用的 bootstrap 豁免按 `WAIVER_INVALID` 作废（§18.3）。

---

## 8. CI 路径覆盖与外部检测（AC-73/100；Q-16/Q-19）

### 8.1 治理门禁独立 workflow（不触发 `verify`/MySQL）

**决策**：新增 `.github/workflows/collab-gates.yml`（Developer 实现，L2 + IV），**只**运行治理门禁；`.github/workflows/go.yml` 的 `verify` job 保持只由代码路径触发，**其 `paths` 不再包含** `docs/**`、`prompt/**`、`.specify/**`。理由：GitHub Actions 的 `paths` 作用于整份 workflow；若把三目录加进 `go.yml`，任何治理文档提交都会启动带 `mysql:8.4` service 的 `verify`（`.github/workflows/go.yml` 的 `verify` job），成本与时延远高于治理门禁本身——这与“治理成本收敛”的目标相反（QC-05 Q-16）。

| workflow | 触发路径（`push`/`pull_request`） | job | 依赖 |
|---|---|---|---|
| `.github/workflows/collab-gates.yml`（**新增**） | `docs/**`、`prompt/**`、`.specify/**`、`.github/workflows/collab-gates.yml`、`scripts/collab_gates.py`、`scripts/check-collab-gates.sh`、`Makefile` | 仅 `collab-gates` | 只用 runner 自带的 `python3` + `git`；`fetch-depth: 0`；**无** MySQL service、**无** Go 构建 |
| `.github/workflows/go.yml`（**修改**） | 代码路径（`**/*.go`、`go.mod`、`go.sum`、`api/**`、`migrations/**`、`scripts/**`、`Makefile`、workflow 自身），**移除**三项目录 | `verify`、`collab-gates`（保留现有 job，但不再由治理文档触发） | 现状不变 |

**CI 命令（显式范围 + 记录 + 审计行工件，Q-19）**：

```text
make gates-strict SCOPE=review \
  RANGE="<base>..<head>" \
  RECORD_OUT="$RUNNER_TEMP/gate-runs/<change_id>/<base7>-<head7>-review.tsv" \
  AUDIT_ROW_OUT="$RUNNER_TEMP/gate-runs/<change_id>/<base7>-<head7>-review.audit.tsv"
```

- PR：`origin/${{ github.base_ref }}..HEAD`；push：`${{ github.event.before }}..${{ github.sha }}`；`before` 为全 0 时显式回退 `origin/main..HEAD`，再回退 `root..HEAD` 并打印 `RANGE_BOUND root`（显式回退，不静默）；`<change_id>` 取自 PR 标签/branch 映射，缺失时打印 `CHANGE_ID_UNBOUND` 并不出结论。
- **`RECORD_OUT`/`AUDIT_ROW_OUT` 是 review/release 的必需参数**（§6.1）；CI 显式传两者，`RECORD_OUT`/`AUDIT_ROW_OUT` 在 `$RUNNER_TEMP`（仓库外），执行器不写仓库；Lead/Reviewer 把 gate record 与审计行**同一字节**提交到 `docs/lead/<change_id>/gate-runs/` 与 `docs/review-tracking.md`，摘要记入 gate record（§6.12）。
- 上传 gate record、stdout、audit row 三个 artifact，供 Reviewer/IV 引用。
- 不把 `--mode=warn` 放进 CI；不在 CI 里 `continue-on-error`。
- G-1 静态核对（§6.11）：`collab-gates.yml` 存在且 `paths` 含三目录、job 仍执行 `make gates-strict`、`AUDIT_ROW_OUT`/`RECORD_OUT` 未被删；`go.yml` 的 `paths` 含三目录（即治理文档会触发 `verify`）⇒ `GATE_INCOMPLETE`。**“拆成独立 workflow”不是完整覆盖**，D-CR-6 仍 OPEN（§8.3）。

### 8.2 `GATE_NOT_RUN` 外部检测（AC-73/AC-107）

- CI 未触发/被停用/触发路径被删 → IV 发 `GATE_NOT_RUN`，记 GV-S21；CI 绿或“无失败”不得当通过。
- IV 复核内容：`collab-gates.yml` 的 `on.push.paths`/`on.pull_request.paths` 是否包含三目录；`collab-gates` job 是否仍执行 `make gates-strict`（含 `RECORD_OUT`/`AUDIT_ROW_OUT`）；是否有 `paths` 排除/条件跳过；`go.yml` 是否被加回三目录（重复触发 `verify`）。
- 仓库内自测（`scripts/test-collab-gates.sh`）增加 workflow 静态断言，防止本地改动悄悄删路径；但**自测不能替代 IV**（同一 actor 不得自证）。

### 8.3 D-CR-6 的保守默认（保持 OPEN）

- 根 `AGENTS.md` **不加入** CI 触发路径（D-CR-6 未决）；AGENTS 变更仍按 L2 + 人类书面批准 + IV 独立 diff 复核 + QC/Reviewer 审核（AC-100）。
- 不得把“三项 CI 路径已覆盖”或“治理 workflow 已拆分”称为完整覆盖；`AGENTS.md`、其他未列举治理路径的 CI 缺口保持可见（`docs/review-tracking.md` 的相应 human decisions 区）。

### 8.4 成本口径（修正 Rev 1.0 的“<10s”声明）

- 治理 workflow 不启动 MySQL、不编译 Go，只跑治理门禁 + 上传 artifact；其耗时随仓库规模与 `fetch-depth: 0` 的 clone 线性增长，**本 ADR 不给未计时阈值**（提案的成本数字未被计量，Spec §0 明确不得当 AC 阈值，`docs/cost-reduction-proposal.md` §7 的估算同样不作为阈值）。
- Rev 1.0 §19 的“CI 增加治理路径触发（成本小、<10s）”**撤回**：拆分前的真实成本是“每次治理文档提交都触发带 MySQL 的 `verify`”，拆分后是“只跑治理门禁”；两者都不是 <10s 的保证，须在 P0b 实测并在 gate record 中记录耗时，供后续决策。

---

## 9. L1 QC + Reviewer 联签机制（AC-82/84/98/101）

### 9.1 状态机

```text
WAITING
  → REVIEW_VERDICT_PASS   (Reviewer GV-S5 PASS + verdict_at + review_scope + base/head + reviewed_digest)
  → QC_SIGNED             (QC 写 qc_signoff，signed_at ≥ verdict_at)
  → REVIEWER_COUNTERSIGNED(Reviewer 写 reviewer_signoff，signed_at ≥ qc_signoff)
  → SIGNED                (派生；仅 post_review_pre_merge)
失效：delta 超出签署范围 / HEAD 或 digest 变化 / effective_tier→L2 / AC-80 冻结 → WAITING
```

`stage` 必须为 `post_review_pre_merge`；单签、同一 actor、缺 `signed_at/delta_digest/verdict_at` → 无效并记 `SIGNOFF_MISSING`。

**顺序是可机检的硬条件（Q-11）**：`verdict_at ≤ qc.signed_at ≤ reviewer.signed_at`，且三者的 `delta_digest` 完全相同；任一字段缺失 ⇒ `SIGNOFF_MISSING`。`verdict_at` 是 Reviewer 出 verdict 的机器可读时间（GV-S5 / `reviewer_signoff` 的必填字段），不是 `signed_at` 的别名。

### 9.2 签署字段（GV-S16）

QC 在其 QC 报告中写：

```yaml
qc_signoff:
  actor: <qc-id>
  signed_at: 2026-11-24T23:10:00+08:00
  stage: post_review_pre_merge
  delta_digest: sha1:<diff_digest>
  review_scope: delta + affected clauses
  affected_clauses: [...]
  unreviewed: [...]
  execution_evidence: [AC-xx -> evidence_path]
  verdict: SIGNED
```

Reviewer 在其 review verdict/线程中写 `reviewer_signoff`（同字段，另加 `verdict_at`）：

```yaml
reviewer_signoff:
  actor: <reviewer-id>
  verdict_at: 2026-11-24T22:55:00+08:00   # GV-S5 的 verdict 时间，必填
  signed_at: 2026-11-24T23:20:00+08:00
  stage: post_review_pre_merge
  delta_digest: sha1:<diff_digest>
  ...
  verdict: SIGNED
```

- QC 的 `signed_at` 必须 ≥ Reviewer 的 `verdict_at`（QC 不得在 verdict 之前签署）；Reviewer 的 `signed_at` 必须 ≥ QC 的 `signed_at`。
- 三者 `delta_digest` 必须相同；G-1 逐字段比较（字段名固定），缺失/顺序错误/摘要不一致 ⇒ `SIGNOFF_MISSING`（AC-82/AC-84）。
- Reviewer 必须核对 QC 的 `unreviewed` 不含本次 delta/受影响条款、受影响 AC 有真实执行证据（AC-101）。`SIGNED` 不落盘为第三字段，由消费者派生。

### 9.3 失效与重签（AC-84）

- 任何新增提交/HEAD/内容摘要变化 → 作废两侧签署，重签。
- `effective_tier` 升 L2、AC-80 冻结、delta 超出签署范围 → 立即失效。
- QC 声明的未复审章节若属于本次 delta 或受影响条款 → 不得批准，须补齐。
- 旧签署不得沿用；重签必须重新绑定 `delta_digest`。

### 9.4 人类保留域（AC-83）

- 人类书面批准（GV-S15）：宪法修订、分级裁决、Spec、ADR、Plan（仅在要求时）、Release、bootstrap。
- 人类知会/裁决：每个 `change_id` 的首次/重审 FAIL、merge；知会**不得**替代书面批准。
- 机器/Agent 不得写 `approved_by`/`release_verdict`；不得把“已委托/自动批准”写入。

### 9.5 委托不得外溢（AC-85）

- L1 实施中发现 L2 判据/跨服务/资金/库存/权限/状态机/治理/canonical 影响 → 停止、按 AC-80 停线、作废联签、按 L2 重走。
- 用 L1 联签替代 L2 的 QC/人类批准/Plan → `TIER_MISMATCH`。
- L1 影响到 Release 证据/既有基线 → 重新评估并取得人类 Release 书面批准。

---

## 10. QC delta/full 身份与 Release 对象集（AC-86…89/105/110）

### 10.1 delta 的最小范围与版本判据（AC-86）

- QC 报告头必须含：`spec_id/from_version/from_digest/to_version/to_digest/reviewed_digest/review_type=delta|full/reviewed_sections/unreviewed_sections/affected_INV/affected_AC`。
- **另须按 AC-111 写 `change_id`/`parent_change_id`/`effective_tier`（N-5/M-5）**：这三项是 §4.5 `PC-QC-REPORT` 行做 per-change 继承判定的输入，缺任一项 ⇒ 该报告不得按 per-change 继承判定（按 L2 复审），且引用它的结论记 `EVIDENCE_UNBOUND`。**完整必填集 = AC-86 的清单 ∪ AC-111 的三字段**，两处规则指向同一份报告头，不得各自维护更短的子集。
- delta 至少覆盖：变更 delta + 直接关联章节 + 受影响 INV/AC + State Ownership + 异常分支 + `reviewed_digest`。
- 小版本 = 主版本号不变；主版本递增 → 全额复审（AC-88）。版本号不变但摘要变化 → 新版本重审（`DIGEST_MISMATCH`）。

### 10.2 未复审章节声明（AC-87）

- delta 报告必须列“本次未复审章节”（章节号、理由、风险）+ 已复审 delta/章节 + `reviewed_digest`。
- 缺失/为空但未声明全量/与 delta 矛盾/摘要不符 → `DIGEST_MISMATCH` / `QC_FULL_REVIEW_REQUIRED`。
- 声明归 QC（GV-S7）；Reviewer 不得代写或改写。

### 10.3 两个强制全量节点（AC-88/105）

1. Spec 主版本递增；
2. 进入 Release 门禁。

Release 全量 QC 覆盖 `release_scope` 的**全部对象**（每个对象当前 version + digest），产出一份 Release QC 记录；对象缺失/仅 delta/摘要不符 → `QC_FULL_REVIEW_REQUIRED`。全量节点不可由 L0/L1 联签或口头催促替代；取消/修改须人类批准 Spec 升版（INV-56）。

### 10.4 delta 不得扩张（AC-89）

delta 不得用于 L2 新契约、状态机拆分、资金/库存/权限、跨服务协议修订、治理/canonical 变更或降低宪法/AGENTS/CONVENTIONS 控制；不得把未复审章节标“无风险”而无证据；人类豁免按 AC-94 校验。

---

## 11. Scaffold 生成器（AC-90/91/92）

### 11.1 接口与输入

```text
python3 scripts/scaffold_governance.py \
  --change-id=<id> --artifact=plan,tasks,evidence-matrix \
  --tier=<effective_tier> --base=<sha> --spec=<path@version> --adr=<id@rev> \
  --owner=<lead> --allowed-paths=<glob,...> --rollback=<text>
```

- 只生成结构 + 主权声明；内容由对应 owner 写（Lead/PO/Architect/Developer/Reviewer）。
- `template_version` = 三个模板的 `sha1(内容)` + git blob，写入骨架头与幂等键。
- 输出：`docs/lead/<change_id>/plan.md`、`tasks.md`、`evidence-matrix.md`。
- 幂等：`change_id + artifact + template_version`；已存在同键 → 不改写；已存在不同键 → 拒绝并报 `SCAFFOLD_INCOMPLETE`（无 `--force`）。

### 11.2 最小字段与主权声明（AC-90/91）

- plan：`change_id/base/head/tier/effective_tier/parent_change_id/spec_version/integration_owner/baseline_impact/affected_AC/allowed_paths/rollback/approvals`。
- tasks：`task_id/owner/AC/evidence/status`。
- evidence-matrix：`AC/test_type/evidence_path/status/owner`（+ `change_id/parent_change_id/effective_tier`）。
- 主权声明（生成器写入，内容 owner 不得删除/改写）：

```text
> 主权声明：docs/collab/ac-coverage.tsv 是唯一 AC 台账，Owner=Reviewer；本文件只是 Plan 视图。
> 产物 owner：Spec=PO；ADR=System Architect；Plan/Tasks=Lead；QC=QC；Review=Reviewer；代码=Developer。
> canonical：GV-S3/GV-S19=Lead；GV-S10=Reviewer；GV-S20=各结论 owner；GV-S21=Independent Verifier；GV-S22=release owner。
> 生成不等于批准；不得用于 Spec/ADR/Plan/Release 批准证据。
```

- 缺声明/声称第二台账 → `SCAFFOLD_OWNERSHIP_MISSING`；缺模板/字段/`template_version` → `SCAFFOLD_INCOMPLETE`。
- `evidence-matrix-template.md` 由 System Architect 在 P0.5 建立；缺失不得手写替代。

### 11.3 生成边界（AC-92）

- 生成器不得代写业务结论、不得改变 owner、不得绕过审批门。
- 不得把“已生成/已用模板”作为批准证据。
- `api/gen` 禁止手工改生成物的规则只针对代码生成物；治理骨架不受该禁令约束，但也不得手工改代码生成物。

---

## 12. 审计/台账/注册表格式与 delta audit

### 12.1 载体与区段

- `docs/review-tracking.md` 由 Lead 按 AC-109 建立两个区：`## Change freeze`、`## Governance audit log`；建立前不得首次写入。**时点（N-1/M-1）**：`## Governance audit log` 区段与 `mode=bootstrap` 批准行必须在**首个 P0a 提交之前**建立并提交（该提交即 `BASE` 或其祖先，须带 `change_id`/`Actor`/`Reviewed-by` 全套 trailer，§7.1 前置/不变量 6）；因 P0a 首个提交的父提交即 `BASE`，该行必须落在 `BASE` 树内，供 P0b 投影的 G-4 读取有效期——否则 P0b 四闸门 PASS 不可能达成。P0.5 只建其余载体（GV-S19/GV-S3/GV-S10/GV-S21/GV-S22 与模板）。
- 两区之外的历史 human decisions 区不改写。**引用一律用区段标题 + 日期，不用行号**（行号漂移；QC-05 Q-13）：现行区段为
  1. “2026-11-24 human decisions: governance cost reduction (D-CR-1…D-CR-5)”（D-CR 批准证据，approver=Director）；
  2. “2026-11-24 human decisions: GOV-COST-01 bootstrap, IV and commit timing”（bootstrap/IV/commit timing 裁决）；
  3. “2026-11-24 SPEC/ADR approvals: SPEC-0002 v1.8 / SPEC-0001 v1.4 / ADR-0007 Amendment 1”。

### 12.2 Governance audit log（GV-S20，AC-95）

TSV，表头固定（**15 列**；`spec_version`/`adr_revision` 为 Rev 1.1 新增，Q-18），append-only：

```text
timestamp	change_id	parent_change_id	effective_tier	tier_source	actor	base	head	mode	verdict	spec_version	adr_revision	evidence	digest	supersedes
```

- `timestamp`：ISO-8601 带 `+08:00`；`base/head`：7–40 位 hex；`spec_version`：`vX.Y[.Z]`（本行结论所依据的 Spec 版本）；`adr_revision`：`Rev X.Y` 或 `-`（无 ADR 时）。
- `mode ∈ {tier,qc,review,release,gate,waiver,scaffold,bootstrap,ac-verify,approval}`；`verdict` 使用 Spec §5.2 的阻塞码/结论枚举（`waiver` 行另用 `GRANTED|REVOKED`，`tier` 行用 `CONFIRM|UPGRADE_L2`）。
- **HUMAN_ONLY_MODES = {tier, waiver, approval, bootstrap, release}**：这些行的 `actor` 必须是 `HUMAN_ACTORS` 中的具名人类（Director 或书面授权者）；`actor=gates-executor`/其他非人类 actor ⇒ 拒绝（`EVIDENCE_UNBOUND`；`waiver` 另记 `WAIVER_INVALID`）。
- `mode=gate` 行：`actor` = 提交该行的结论 owner（Reviewer 或 release owner），`evidence` 必须含 `produced_by=gates-executor;gate_record=<path>;gate_self_digest=sha1:<…>`。
- `mode=waiver|approval|bootstrap|release` 行的 `evidence` 固定键见 §5.1；缺任一固定键 ⇒ 该行无效（`EVIDENCE_UNBOUND`；豁免另记 `WAIVER_INVALID`）。
- 必须记录 `change_id`、`parent_change_id`、继承 `effective_tier`、`tier_source`、actor、时间、范围、结论、证据路径、`digest`（`reviewed_digest`/证据工件摘要）、`spec_version`/`adr_revision`（AC-95 的“引用版本”）。
- 更正：追加新行 + `supersedes=<原行时间戳/序号>`；不得删除/覆盖。
- 缺范围/摘要/证据路径/版本列/审计行 → `EVIDENCE_UNBOUND`，不得用于 Release。
- **机器解析口径**：G-1/G-3/G-4 只用“当前 head 的该区段 TSV + 固定列序 + 固定键值”解析；不解析自由文本。解析失败（列数不符/枚举非法/重复主键）⇒ `EVIDENCE_UNBOUND`。

### 12.3 Change freeze（GV-S10，AC-81）

Markdown 表：`change_id | frozen_at | actual_tier | discovered_at | discoverer | action | re_review_plan | evidence | unfrozen_at`。

- Reviewer 在发现时立即写；不得删除或静默关闭；与冻结登记同载体。
- 解冻只能追加 `unfrozen_at` 与证据，不得删除原行。

### 12.4 错分处置记录（AC-81）

错分 → 冻结（§12.3）+ 人类 GV-S17 `UPGRADE_L2` + 重审 Plan 指针 + GV-S20 行；重走期间再次低估 → 保持冻结并按更高级别重走。

### 12.5 delta audit（AC-104）

- 触发：`AGENTS.md`、CONVENTIONS、宪法、`docs/specs/**` 治理 Spec、`prompt/**`、`scripts/**`、`.github/**`、`Makefile` 发生治理性变更。
- 内容：日期、旧/新版本、变更文件与差异摘要、Lead 与所有受影响 subagent 的重读确认。
- 载体：`docs/review-tracking.md` 或 `docs/collab/<pair>/threads/`；CONVENTIONS/prompt 的流程/角色变化由 Reviewer 复核（不得 Lead 自证）。
- 缺失 → 不得进入下一交付/merge，记 `EVIDENCE_UNBOUND`。

---

## 13. 状态所有权：GV-S1…GV-S22 单写者映射

| 字段 | 唯一写入者 | 落点/字段 | 消费方 | 模糊点（本 ADR 的处置） |
|---|---|---|---|---|
| GV-S1 `change_tier` | Lead（change-package） | change-package `tier` | Reviewer/人类/门禁 | 历史初判；不表示批准 |
| GV-S2 `tier_review` | Reviewer（verdict） | review verdict | Lead/人类 | 不改写 GV-S1 |
| GV-S3 变更输入包 | Lead 写 base/tier/...；Reviewer **只追加** head | change-package | 全员 | 双字段分写，不是双写者；不得改写已写字段 |
| GV-S4 `gate_verdict` | 门禁执行器 | `--record-out` TSV 的 G-1…G-4 | Reviewer/Release 授权人 | 只证明机器结论，不证明语义/可发布 |
| GV-S5 `review_verdict` | Reviewer | review verdict | Lead/人类 | ≠ Spec 全量验收/merge/Release |
| GV-S6 `release_verdict` | 人类指挥官 | release 记录 | 全员 | 机器/Reviewer/Lead 不得代写 |
| GV-S7 `qc_coverage` | Quality Checker | QC 报告头 | Reviewer/人类 | 缺未复审清单不得视为全量 |
| GV-S8 `ac-coverage.tsv` | Reviewer | 8 列台账 | 全员 | 第二台账禁止；`VERIFIED` 需 Reviewer 签名+摘要 |
| GV-S9 `waiver` | 人类指挥官（AI 只引用） | **GV-S20 行 `mode=waiver`**（§5.1/§12.2：`waiver_id/approver/date/expiry/scope/affected_inv/affected_ac/alternative_control/revoked_at/status`） | 全员/G-4 | AI 不得创建/推断/扩大；无记录/过期/撤销即不存在（`WAIVER_INVALID`） |
| GV-S10 `mis_tier_ledger` | Reviewer | review-tracking `## Change freeze` | Lead/人类 | 区段由 Lead 建（AC-109），行由 Reviewer 写 |
| GV-S11 `gate_range` | 发起者（Review=Reviewer；Release=人类授权人） | 命令/S21/审计行 | 门禁 | 执行器只消费，不拥有 |
| GV-S12 `change_id` | Lead | change-package/registry | 全员 | 不复用、不绕冻结 |
| GV-S13 scaffold 结构 | Lead（生成器执行） | `docs/lead/<id>/*` | 全员 | 生成≠批准 |
| GV-S14 产物内容 | 各产物 owner | Spec/ADR/Plan/QC/Review | 全员 | 生成器/Lead 不得代写 |
| GV-S15 人类批准记录 | 人类指挥官 | **GV-S20 行 `mode=approval\|bootstrap\|release`**（`approver/date/expiry/scope/affected_ac/affected_inv`；`digest`=被批准文件） | 全员/G-4 | bootstrap `expiry` 必填（v1.4 已入 Spec）；非 bootstrap 可 `-`；非人类 actor 写入即拒绝 |
| GV-S16 L1 联签 | QC 写 `qc_signoff`；Reviewer 写 `reviewer_signoff` | QC 报告 / review verdict | 全员 | `SIGNED` 派生，不落盘、无第三写者 |
| GV-S17 `tier_ruling` | 人类指挥官 | **GV-S20 行 `mode=tier`**（`verdict=CONFIRM\|UPGRADE_L2`；`reason/date/gv_s1_tier/decided_for`） | Lead/Reviewer/门禁 | 唯一升级事实；不改写 GV-S1 历史；G-1 读最新非 supersedes 行派生 `effective_tier`（§4.5） |
| GV-S18 `release_block_reason` | 门禁执行器（内容）；Lead/Reviewer（提交） | gate record TSV 的 `release_block_reason` 列（`docs/lead/<change_id>/gate-runs/<base7>-<head7>-<scope>.tsv`，§6.12） | 人类/Reviewer/IV | 不是 `release_verdict`；不得写 PASS；写入时机=运行结束；写失败 ⇒ `GATE_ERROR` |
| GV-S19 注册表 | Lead | `change-id-registry.tsv` | 全员 | 追加式；关闭追加状态行，不覆盖 |
| GV-S20 审计日志 | 每行由结论 owner 写 | review-tracking `Governance audit log` | 全员 | 共享 append-only；按行归属，不是单字段写者；一个 canonical 指针 |
| GV-S21 IV 记录 | Independent Verifier（Director 本人或每次运行前书面指定） | `docs/collab/gate-runs/<change_id>/<base>-<head>.md` | 人类/Reviewer/门禁 | 无指定/独立性证据即不存在；bootstrap 另记 `base_tree_path/base_tree_digest/base_gate_summary/range` |
| GV-S22 `release_scope` | release owner（`docs/lead/releases/` 的 MANAGED owner） | `docs/lead/releases/<id>/scope.md` | QC/Reviewer/人类 | 无 `designated_by/designation_evidence` 不得开工；`--release-scope` 机器校验（§5.4） |

**明确无第二写入者的争议点**：

1. **GV-S3 的 base 与 head 由两人写**：字段分离 + 只追加，不是同一字段双写者。
2. **GV-S16 的 `SIGNED`**：派生值，不存储；任何“第三签署字段”都被禁止。
3. **GV-S20 的共享行**：行级 owner 制；执行器只产出 `mode=gate` 行的**工件**，提交者是其 `actor`（Reviewer/release owner，`evidence` 记 `produced_by=gates-executor`）；`gates-executor` 是 `ROLE_ALIASES` 的机器角色，**不得**作为任何审计行 `actor`（Q-14）。
4. **GV-S4/GV-S18 同属执行器**：不同字段（结论 vs 阻断原因），由同一次运行写入不同行/列。
5. **GV-S9 与 GV-S15 的 `expiry`**：两者都是人类写、都落在 GV-S20 专用 mode 行（`waiver` / `approval|bootstrap|release`），不新建文件、不新建第二台账（Q-04）；bootstrap 复用 `mode=bootstrap` 行承载 `approver/date/expiry/scope`。
6. **GV-S8 的签名/摘要不在台账列**：放在 GV-S20 行（§5.5），Reviewer 同 actor；台账 schema 不变。

---

## 14. 异常与恢复（E32…E62）

| # | 场景 | 系统行为 | 恢复/所有者 |
|---|---|---|---|
| E32 | L2 当 L1 执行 | G-1 `TIER_MISMATCH`；冻结；GV-S10 行 | 人类 GV-S17；Lead 重走 |
| E33 | 执行器崩溃/超时 | `GATE_ERROR`/`GATE_TIMEOUT`，GV-S18 | 同键重跑；执行器/发起者 |
| E34 | CI 静默跳过治理路径 | IV 发 `GATE_NOT_RUN`；阻断 | IV；Lead 补路径 |
| E35 | delta QC 漏清单/摘要 | `DIGEST_MISMATCH`/`QC_FULL_REVIEW_REQUIRED` | QC 补清单/全量；QC |
| E36 | Release 无全量 QC/对象集缺失 | `RELEASE_BLOCKED` | QC 补全量；release owner |
| E37 | 豁免缺失/过期/撤销 | `WAIVER_INVALID`；不推进 | 人类补书面；人类 |
| E38 | scaffold 缺主权行/字段 | `SCAFFOLD_OWNERSHIP_MISSING`/`SCAFFOLD_INCOMPLETE` | Lead 重新生成 |
| E39 | 复用 `change_id` | `CHANGE_ID_REUSED` | Lead 换新 id/人类批准 hotfix |
| E40 | 仅 Reviewer 签 L1 | `SIGNOFF_MISSING` | Reviewer/QC 补签 |
| E41 | L1 中发现 L2 | 停线；作废 GV-S16 | 人类裁决；按 L2 重走 |
| E42 | AC-REF 当 AC-VERIFIED | `AC_SEMANTICS_INVALID` | 补真实证据；Reviewer |
| E43 | Release 用默认范围 | `RANGE_UNBOUND` | 显式范围重跑；发起者 |
| E44 | 门禁被同 diff 改恒 PASS | `GATE_INCOMPLETE`/`GATE_NOT_RUN` | 基线重跑 + IV；Lead |
| E45 | 基线/变更后不一致或无 IV | 双摘要阻断 | 修复/书面豁免；IV |
| E46 | 治理 AC 无 G-3 证据 | `GOV_AC_EVIDENCE_MISSING` | 补 `VERIFIED` 行；Reviewer |
| E47 | 同版本不同摘要 | `DIGEST_MISMATCH`；旧结论失效 | 按新版本重审；QC/Reviewer |
| E48 | 豁免被撤销 | `WAIVER_INVALID`；按更严格规则重走 | 人类记录撤销 |
| E49 | 未列举/新建 canonical 当 L0/L1 | 路径下限/未知⇒L2；`TIER_MISMATCH` | 按 L2；Lead/人类 |
| E50 | Release 对象集缺 Spec/ADR | `QC_FULL_REVIEW_REQUIRED` | release owner 补对象集 |
| E51 | 治理文件变更缺 delta audit | `EVIDENCE_UNBOUND` | Lead 补 delta audit |
| E52 | L1 联签无真实执行证据 | `EVIDENCE_UNBOUND` | 补执行证据/豁免 |
| E53 | G-3 未上线即宣称 VERIFIED | `G3_BOOTSTRAP_REQUIRED` | 先 bootstrap；Reviewer |
| E54 | IV 缺失/actor=作者或 Lead | `GATE_NOT_RUN`/`EVIDENCE_UNBOUND` | Director 指定；IV |
| E55 | 基线/变更后结果不一致 | 双摘要阻断 | 修复/书面豁免 |
| E56 | release owner/scope 缺失 | `RELEASE_BLOCKED` | Director 指定；release owner |
| E57 | 新载体未登记/无 owner | `SCAFFOLD_INCOMPLETE`/`EVIDENCE_UNBOUND` | 按 AC-109 登记；Lead |
| E58 | P0b 被自身 NOT-RUN 治理 AC 阻断 | 用 §7.2 的 `BASE` 投影评估（scoped AC-01…68） | IV/人类；不阻断 P0b |
| E59 | 结论缺少已提交的 GV-S20 审计行 | `EVIDENCE_UNBOUND`（review/release 均强制） | Lead/Reviewer 提交 `--audit-row-out` 工件后重跑 |
| E60 | GV-S18 记录写失败/路径越界 | `GATE_ERROR`（退出 2），不得出结论 | 修权限/path 后同键重跑 |
| E61 | GV-S20 人类专用 mode 被非人类 actor 写入 | `EVIDENCE_UNBOUND`（waiver 另记 `WAIVER_INVALID`） | 人类重写；AI 不得补写 |
| E62 | `BASE` 不满足 §7.1 不变量（含已跟踪治理 Spec/基线越界/树不干净） | `G3_BOOTSTRAP_REQUIRED` | Lead 前移 `BASE` 或重排 P0a；人类批准 |

**幂等键**（沿用 Spec §6，不新增第二套）：门禁运行 `change_id+base+head+mode+digest`；QC delta `spec_id+from/to version+digest`；L1 联签 `change_id+artifact+delta_digest+stage+signed_at`；注册表 `change_id+repo`；审计行 `timestamp+change_id+actor+base/head+mode`；IV `change_id+base+head+verifier+双摘要`；G-3 引导 `extension_version+extension_digest+legacy_gate_digest+reviewer`。

---

## 15. 【假设与不确定性】

1. **Spec 版本/摘要绑定**：本 ADR 绑定**已批准的 SPEC-0003 v1.4**（697 行，sha1 `58679f03…`，`approved_by=Director`/`approved_at=2026-11-24`；QC-05 审稿 696 行 sha1 `79db9323…`）。QC-05 M-1…M-7 在批准时作为编辑性澄清并入（M-3 新增 GV-S21 `g4_status`；M-2 扩 AC-69 豁免到四项输出要求），本 Rev 1.1 已按批准稿对齐；若批准稿再变或升版，按 AC-102 以新摘要重新绑定并升 Rev。v1.3 仅作背景。
2. **GV-S18 持久化（已定稿，不再是 open question，Q-12）**：载体 = gate record TSV 的 `release_block_reason` 列（`docs/lead/<change_id>/gate-runs/<base7>-<head7>-<scope>.tsv`，§6.12），由执行器在运行结束时原子写；CI 在仓库外 `$RUNNER_TEMP` 写并上传，Lead/Reviewer 以同一字节提交。执行器的“只读”自述**已修正**为“只写 record-out/audit-row-out 两类路径，不写 `review-tracking.md`、不改 git 历史”。
3. **治理证据的签名/摘要载体**：Spec 只说“Reviewer 签名 + 摘要”，未说存哪；本 ADR 放 GV-S20 行而不改 8 列台账（§5.5）。若要求进台账列，则是 carrier schema 变更（L2），需另立 change。
4. **`BASE` 上无 G-4（已由 v1.4 尘埃落定，Q-02）**：`BASE` 的 legacy 执行器只有 G-1/G-2/G-3（`scripts/collab_gates.py:2`），v1.4 的 AC-106/AC-112 明文规定记 `G-4 NOT-RUN(not-implemented-at-BASE)`（不阻断 P0a/P0b、永不计 PASS），并规定 P0b 落地 G-4。**本 ADR 只实现 v1.4，不做任何解释性放宽**。
5. **GitHub Actions 行为未本地实跑**：`github.event.before` 全 0、`base_ref` 解析、`paths` 触发均为 [不确定]；CI runner 结果不作为本地证据，Review 结论必须由 Reviewer 在本地/CI 记录上复核。
6. **CI 红与 merge**：P0a…P2 在同一 WIP 分支叠加，中间提交在治理 AC 未 VERIFIED 时门禁为 NOT-RUN（退出 4），**不 merge 到 main**；最终 head 全绿后才 Review/Release。若组织要求中间 merge，需要 Spec 升版授权，不属本 ADR 权限。
7. **业务台账枚举**：业务行 `test_type=deferred` 不在 AC-99 枚举内；本设计只对 AC-69…112 新校验，业务行保持原样。若要求全局枚举，需要数据迁移 + L2。
8. **`docs/review-tracking.md` 的引用方式**：行号会漂移（QC-05 Q-13 实测 `:60`/`:94`/`:117`），**canonical 引用只带区段标题 + 日期**（§12.1），本 ADR 全文已改为区段引用；个别历史行号只作“当前文件位置”的辅助说明。
9. **`ac-baseline.txt` 登记时点**：批准轮 Reviewer 追加 AC-69…112 后，G-3 立即把它们纳入范围 → NOT-RUN；这是预期（Release 阻断），不得先登记再宣称 VERIFIED。
10. **摘要算法口径**：QC 用 `shasum -a 1`（原始字节），git blob 是另一口径；本 ADR 统一 `sha1:<原始字节>` + `diff_digest`，并在记录中同时保留 base/head 以便复核。
11. **并发写审计日志**：GV-S20 是 append-only 文本；两个 actor 同时追加可能冲突。假设同一 `change_id` 串行推进（AGENTS §1.0 “同一时间只推进一个 change_id”），冲突时以 git 合并解决并保留双方行。
12. **IV 人选（已裁决，不再是 open question）**：Director 本人持有 IV，或每次运行前书面指定（GV-S21 `designated_by`/`independence_evidence`；Director 本人 `Director/direct`）。该裁决已写入 `docs/review-tracking.md` 的 “2026-11-24 human decisions: GOV-COST-01 bootstrap, IV and commit timing”；缺记录才 `GATE_NOT_RUN`（§6.10）。
13. **第三方可复现性（Q-01 修正）**：§7.2 依赖 `git worktree`（或 `git clone --no-local`）构造投影树；**`git archive` 不是 canonical**（非 git 目录会触发 `G-0 GATE_ERROR`，若 `git init` 再提交则伪造历史），只用于只读摘要比对。投影内容（`BASE` 树 + 被审门禁字节集）是 `BASE`/`REVIEWED` 的纯函数，`projection_diff_digest` 可复算。
14. **不引入 PyYAML**：CI paths 的机械核对用文本扫描（`push:`/`pull_request:` 块内查找三目录），不做 YAML 解析；误判由 IV 人工兜底，不号称完备。
15. **`docs/specs/README.md` 的 SPEC-0003 索引**：由 PO 维护；本 ADR 不写索引，也不把索引存在当作 Spec 批准证据。
16. **人类身份不可由机器认证**：GV-S20 的人类专用 mode 只能靠 `HUMAN_ACTORS` 名单 + commit trailer + 人类 merge 检查点约束（`scripts/collab_gates.py:132-137` 只读 trailer 字符串）。本 ADR 明确：**人类 merge 检查点是最终控制**；`HUMAN_ACTORS` 是机器侧的辅助拒绝，不构成身份认证。该信任前提写入本节，不隐藏。

---

## 16. 决议状态与剩余 open questions（Rev 1.1；Q-04、IV 已裁决）

| ID | 问题 | 状态 | 决议/落点 |
|---|---|---|---|
| **D-CR-6** | CI 是否也纳入根 `AGENTS.md`（及全部 MANAGED 路径） | **保持 OPEN**（人类未决） | 保守默认：三项目录 + `AGENTS.md` 按 L2 + 人类书面批准 + IV 独立复核（AC-100）；不把三项称为完整覆盖（§8.3） |
| **D-CR-7** | L1 联签中 QC 的审查范围 | **保持 OPEN**（人类未决） | 按 Spec 推荐 (b)：delta + 受影响既有条款 + 未复审声明 + 执行证据（§9.2）；不得自行扩张/缩减 |
| Q-01 | GV-S18 落盘方式与执行器只读边界 | **CLOSED** | gate record TSV 的 `release_block_reason` 列 + 执行器只写两类路径（§6.12/§15-2） |
| Q-02 | 治理 AC 的签名/摘要放 GV-S20 行还是台账新列 | **CLOSED**（设计选择） | GV-S20 行（不改 8 列）；新增列需 L2 schema 变更（§5.5/§6.7） |
| Q-03 | bootstrap scoped 运行方式 | **CLOSED**（Rev 1.1 重写） | `BASE` 投影树 + `BASE..PROJ` head 投影；不用“删 Spec/覆盖 baseline”的工作区 hack（§7.2） |
| Q-04 | `BASE` 上 G-4 不存在 | **CLOSED（人类已裁决）** | SPEC-0003 v1.4：`G-4 NOT-RUN(not-implemented-at-BASE)`、P0b 落地 G-4、AC-69 豁免随 P0b 终止（§7；`docs/review-tracking.md` 的 “GOV-COST-01 bootstrap, IV and commit timing”） |
| Q-05 | `MANAGED` 的 append-only co-writer 语义 | **CLOSED** | 无 `^-` 行才接受 co-writer；per-change tier 另按 §4.5 表（§5.2） |
| Q-06 | 版本解析扩为三段式是否属于 P0b | **CLOSED** | 归 P0b；同版本内容摘要校验同轮落地（§6.11） |
| Q-07 | `docs/review-tracking.md` 在 G-1 的 owner 集 | **CLOSED** | `{Lead(建区/指针), Reviewer(freeze), 结论 owner(audit), 人类(专用 mode)}` + append-only co-writer（§5.2） |
| Q-08 | 基线/变更后两次门禁的 IV 记录时点 | **CLOSED** | 先跑两次 → IV 写记录 → 记录提交 → 最终复跑；head 祖先规则见 §6.4 |
| Q-09 | 首次 Release 的 release owner | **CLOSED** | Director 本人或书面指定；未指定前不得启动全量 QC（§5.4） |
| Q-10 | IV 人选/独立性证据 | **CLOSED（人类已裁决）** | Director 本人持有或每次运行前书面指定；GV-S21 记 `designated_by`/`independence_evidence`（§6.10） |
| Q-11 | GV-S20 追加是否可选 | **CLOSED** | review/release 强制；CI 产工件、Lead/Reviewer 提交、G-1 核对（§6.12/Q-05） |
| Q-12 | `--release-scope` 无机器校验 | **CLOSED** | §5.4 校验表 + 阻断码；`docs/lead/releases/` 入 MANAGED（release owner） |
| Q-13 | 行号漂移 | **CLOSED** | 全部改为区段标题/当前定位说明；MANAGED 引用校正为 `:161`（§1.2/§12.1/§5.2） |

> 说明：本节的 Q 编号是 ADR-0008 自身的 open-question 编号（Rev 1.0 遗留），与 QC-05 报告的问题编号 `Q-01…Q-20` **不是同一套**；两套编号的映射见 §21。

---

## 17. Baseline 影响与 AC 覆盖

### 17.1 业务基线（AC-96）

- **AC-01…68 / INV-01…50**：语义、编号、验收要求不变；本 ADR 不触碰业务代码、迁移、服务与前端。
- 回归验证：最终 head 上运行 `make gates-strict`（完整 AC 集）+ 既有 `make verify`（若本次只改治理/脚本，业务测试不受影响）；任何业务 AC 变化必须停止并按对应业务 Spec 升版处理（AC-96 `:342`）。
- 既有治理基线不降低：`ac-baseline.txt` 只增不减；`ac-coverage.tsv` 业务行不改写；`docs/review-tracking.md` 的 D-CR 批准区等历史证据不改写（引用用区段标题）。
- 回归验证以 §7.2 的 `BASE` 投影为准（AC-01…68 全绿）；业务 AC 的最终 Release 证据仍在完整 head 上取得，二者不得互相替代。

### 17.2 治理产物影响

| 产物 | 变更 | 证据/落点 |
|---|---|---|
| `AGENTS.md` | §2.5（`:76`）增补分级矩阵/错分停线；§1.2（`:38-42`）QC 联签职责；§1.5（`:58-64`）Reviewer 不变；§6（`:109-116`）DoD 增补 G-3 治理证据；§7 不变 | 批准后由 System Architect 修订；L2 + IV |
| `docs/collab/CONVENTIONS.md` | §2/§2.1 分级与 QC delta；§5/§6.1 语义保持；§7.2（`:192-210`）债务清单更新；§9（`:225-234`）角色动作 | L2 + Reviewer 复核 |
| `.github/workflows/collab-gates.yml` | **新建**：治理门禁独立 workflow，`paths` = `docs/**`/`prompt/**`/`.specify/**` + 门禁文件；只跑 `collab-gates`（无 MySQL/Go） | AC-73/100；IV（§8.1） |
| `.github/workflows/go.yml` | **移除**三项目录（避免治理文档触发 `verify`/MySQL）；`:26-33` 的 `collab-gates` job 保留但改由独立 workflow 承载 | AC-73/100；Q-16 |
| `scripts/collab_gates.py` | G-4、`--release`/scope、显式 range、`AC-REF`、治理证据、tier 检查、CI 路径检查、摘要、记录输出 | AC-69…74/99/100/102 |
| `scripts/check-collab-gates.sh` | 参数透传（`RANGE/SCOPE`），不改变 warn/strict | `:5-16` |
| `Makefile` | 增加 `RANGE/SCOPE/RELEASE_SCOPE/RECORD_OUT/AUDIT_ROW_OUT` 透传；`verify` 仍 warn | AC-71；Q-19 |
| `scripts/test-collab-gates.sh` | A1…A5 同步新措辞；新增 G-4/range/release/证据/tier/CI 路径场景 | `:92` 的 `AC coverage` 断言必须改 |
| `.specify/templates/evidence-matrix-template.md` | 新建（Architect，P0.5） | AC-90 |
| `docs/collab/ac-baseline.txt` | 追加 AC-69…112（Reviewer） | AC-99；G-3 |
| `docs/collab/ac-coverage.tsv` | 追加治理 AC 行；业务行不动 | AC-99/101 |
| `docs/collab/change-id-registry.tsv` | 新建（Lead，P0.5） | AC-103 |
| `docs/collab/gate-runs/**` | 新建（IV 写；Lead 登记/指针） | AC-107 |
| `docs/lead/<id>/gate-runs/**` | 新建（执行器产内容；Lead/Reviewer 提交）：GV-S4/GV-S18 gate record | AC-70/74；§6.12 |
| `docs/lead/releases/**` | 新建（release owner）：GV-S22 `scope.md`；MANAGED owner=release owner | AC-110；§5.4 |
| `docs/review-tracking.md` 的 GV-S20 专用 mode 行 | `mode=waiver`（GV-S9）、`mode=tier`（GV-S17）、`mode=approval\|bootstrap\|release`（GV-S15） | AC-94/108/112；§5.1/§12.2 |
| `docs/review-tracking.md` | 新建 `## Change freeze`、`## Governance audit log` 区（Lead） | AC-95/109 |
| `docs/specs/README.md` | SPEC-0003 索引与 ADR 列（PO） | G-2 |

### 17.3 AC 覆盖矩阵（AC-69…AC-112，44/44）

| AC | 设计落点 | 机制 | 预期证据 |
|---|---|---|---|
| AC-69 | §6.2/6.11/7.1 | 四闸门强制输出（P0b 起缺一 `GATE_INCOMPLETE`）；bootstrap 期 G-4 缺失按 v1.4 记 `NOT-RUN(not-implemented-at-BASE)`、不打双码；NOT-RUN 阻断 Release | script + unit |
| AC-70 | §6.3/6.12 | review/release 分离；release 只写 GV-S18（gate record 的 `release_block_reason` 列） | script |
| AC-71 | §6.4 | 显式 `base..head` 透传到 G-3；`RANGE_UNBOUND`；`--repo` 工作树绑定；HEAD 漂移重跑 | script + fault-injection |
| AC-72 | §6.5 | 仅 `AC-REF`/`AC-VERIFIED`；输出自检 | script + static |
| AC-73 | §8.1/8.2、§6.11 | 独立治理 workflow 含三目录 + G-1 静态核对 + IV `GATE_NOT_RUN` | static + script |
| AC-74 | §6.6/6.12 | 退出码 2/124；GV-S18 写失败 ⇒ `GATE_ERROR`；同键重跑；Release 只写阻断码 | fault-injection |
| AC-75 | §4.1 | 唯一标签、L2 例外、最高子改动 | script + manual |
| AC-76 | §4.2 | L0 封闭枚举 + 禁止路径 | manual + script |
| AC-77 | §4.3 | L1 封闭枚举 + L2 判据拒绝 | manual + script |
| AC-78 | §4.4 | L2 默认全流程 | static + manual |
| AC-79 | §4.5 | tier/复核写者分离 + effective_tier | script + manual |
| AC-80 | §4.6 | 停线冻结；拒绝绕过 | manual + script |
| AC-81 | §4.6/§12.3/§12.4 | L2 重走 + 错分台账 | manual + static |
| AC-82 | §9.1/9.2 | `verdict_at ≤ qc.signed_at ≤ reviewer.signed_at` + 同一 `delta_digest` + Reviewer 联签 | script + manual |
| AC-83 | §9.4/§13 | 人类书面批准域；知会≠批准 | manual |
| AC-84 | §9.3 | 失效/重签；摘要绑定 | script + manual |
| AC-85 | §9.5/§4.6 | 委托外溢 → 停线 | script + manual |
| AC-86 | §10.1 | delta 最小范围 + 版本判据 | static + manual |
| AC-87 | §10.2 | 未复审清单 + `reviewed_digest` | static + manual |
| AC-88 | §10.3 | 两个强制全量节点 | manual + static |
| AC-89 | §10.4 | delta 不扩张/不替代 | manual |
| AC-90 | §11.1/11.2 | 模板、最小字段、`template_version`、幂等 | unit + static |
| AC-91 | §11.2 | 主权声明强制 | unit + static |
| AC-92 | §11.3 | 生成边界；不得当批准证据 | manual + unit |
| AC-93 | §5.3 | 全 ref + 注册表 + 人工兜底 | script + manual |
| AC-94 | §5.1/6.11/§13 | 豁免 = GV-S20 `mode=waiver` 行（expiry/scope/affected/revoked）；G-4 读并判 `WAIVER_INVALID` | manual + static |
| AC-95 | §12.2/6.12 | GV-S20 15 列（含 `spec_version`/`adr_revision`）+ review/release 强制追加 + append-only + supersedes | static + manual |
| AC-96 | §17.1 | 业务 AC/INV 零变更证明 | manual |
| AC-97 | §4.1/4.5 | 全域总函数；未列举⇒L2 | script + manual |
| AC-98 | §4.7/§9 | L1/L2 产物×批准者矩阵 | manual + script |
| AC-99 | §6.7 | G-3 治理证据模型（head 绑定、未跟踪不算证据）+ GV-S20 行必备 + 不批量延期 | script + static |
| AC-100 | §6.8/7.2/§8.2 | `BASE` 投影 + head 投影 + IV 逐行 diff + 双摘要 + P0b review-scope/负向夹具 | script + manual + fault-injection |
| AC-101 | §9.2 | L1 受影响 AC 的真实执行证据 | script + manual |
| AC-102 | §6.9/§10/6.11 G-2 | version + digest 绑定；**同版本内容变化 ⇒ `DIGEST_MISMATCH`**；失效重审 | script + static |
| AC-103 | §5.3 | 注册表追加/关闭 + 人工兜底 | script |
| AC-104 | §12.5 | delta audit 触发/内容/载体 | static + manual |
| AC-105 | §10.3/§5.4 | Release 全量 QC 对象集 + `--release-scope` 机器校验 | script + manual |
| AC-106 | §7.1/7.2/7.3 | v1.4 口径：`BASE` 树 G-1…G-3 PASS（显式 `--range=<BASE^>..<BASE>`，scoped AC-01…68）+ `G-4 NOT-RUN` 记录 + IV + 人类批准 | script + manual |
| AC-107 | §6.10/6.8 | IV = Director/书面指定；GV-S21 字段 + 逐次 `designated_by`；P0b review-scope/负向夹具证明校验真的执行 | manual + static |
| AC-108 | §4.5/§5.1 | effective_tier 从 GV-S20 `mode=tier` 行派生（缺失/冲突/非人类 actor 的行为已定义） | script |
| AC-109 | §5.1/5.2 | 载体登记（含 GV-S9/15/17/18/20 modes）+ G-1 MANAGED 最长前缀（含 `docs/lead/releases/`） | script + static |
| AC-110 | §5.4/§6.3 | release owner 指定 + `--release-scope` 校验（存在/字段/版本/摘要/陈旧）+ 阻断码 | script + manual |
| AC-111 | §4.5/§4.8 | per-change 记录内容规则（`parent_change_id` + append-only）+ 同一张 `PC-*` 表 + carrier 结构 L2 | script + manual |
| AC-112 | §7.1/7.2/7.3/7.4 | 阶段、`BASE` 不变量、`BASE` 投影 + head 投影、`G-4 NOT-RUN` 记录、批准/到期/解除、P0b 落地 G-4 | script + manual |

**覆盖计数：44/44（AC-69…AC-112）**；每一行对应至少一个可复核证据类型；治理 AC 不使用伪造 Go 测试。

---

## 18. Rollout / Rollback

### 18.1 上线顺序（不可颠倒）

| 步骤 | 动作 | 入口条件 | 出口证据 | 批准 |
|---|---|---|---|---|
| R0 | ADR 批准 + Lead 分配 `GOV-COST-01`/`BASE` | QC-02 对 ADR Rev 1.1（0 BLOCKER/0 MAJOR；原计划编号 QC-06）**已满足** + 人类 ADR 批准（2026-11-24，§22）**已满足** + SPEC-0003 v1.4 已批准 **已满足**；Plan 书面批准与 IV 指定待满足 | GV-S15（`mode=approval`）、change-package、GV-S20 | Director |
| R1 | P0a G-3 治理证据 + 摘要 + 治理 Spec/baseline 原子落地 | v1.4 批准 + Plan 批准（L2）+ `BASE` 满足 §7.1 不变量 | `BASE` 投影树 G-1…G-3 PASS（`--range=<BASE^>..<BASE>`）+ `G-4 NOT-RUN(not-implemented-at-BASE)` 记录 + head 投影回归 + IV 双摘要 + 人类批准（`mode=bootstrap`，含 expiry） | Director |
| R2 | P0b Release/range/CI/自保护/G-4/IV | P0a 出口 | 同一 `BASE` 投影上四闸门 PASS + **review-scope 运行或负向夹具** + IV + 人类批准 | Director |
| R3 | P0.5 canonical 载体 + 模板 | P0b 出口 | G-1 `MANAGED` 覆盖、载体可写 | Lead/Architect |
| R4 | P1 分级/委托 + AC-101 | P0.5 出口、G-4/range/措辞可用 | L0/L1/L2 矩阵执行证据 | Director |
| R5 | P2 QC delta + scaffold | Release 对象集/G-4 可执行 | 全量节点 + scaffold 自测 | Director |
| R6 | 最终 Review/Release | 44/44 治理 AC VERIFIED、全量 QC、四闸门 PASS | Review verdict + Release verdict | Reviewer + Director |

- 中间提交在 WIP 分支；**不 merge 到 main**，直到最终 head 全绿（§15-6）。
- 每个治理落点变更后，Lead 与所有受影响 subagent 必须做 delta audit（AC-104）。

### 18.2 Halt 条件（立即停线）

- bootstrap 缺人类书面批准/期限，或 `BASE` 位置错误。
- IV 缺失/actor 冲突，或两次结果不一致未绑定双摘要。
- 任一闸门被改成恒 PASS、跳过检查、改退出码，或 CI 路径被删。
- 发现 L2 当 L0/L1 执行（AC-80）。
- 治理 AC 被批量登记为 DEFERRED，或用 NOT-RUN 自称 VERIFIED。
- 同版本不同摘要（AC-102）或旧 QC/Review 结论被复用。

### 18.3 回滚

- 治理规则不得由 Agent 静默回滚；回滚须人类书面批准 + Spec 升版。
- 文档/CI/脚本变更以 `git revert` 表达；保留原变更记录、错分台账、豁免撤销、GV-S21 记录与审计行；**不得删除历史**。
- 回滚后治理 AC 回到 NOT-RUN，Release 阻断；P0b 后若扩展回滚，恢复 AC-99/AC-100 的完整集并重新 bootstrap（新批准/新 `BASE`）。
- **回滚 = 载体 + 规则一起回退，且回退后不得出现“无主路径”**（QC-05 §10 遗留）：回滚 commit 必须同时（a）把 GV-S9/GV-S15/GV-S17 的专用 `mode` 行标记为作废（追加 `supersedes`，不删除），（b）把已引用的 bootstrap 豁免置 `WAIVER_INVALID` 并写明影响面，（c）把 `MANAGED` 中随本 ADR 新增的条目（`docs/lead/releases/`、`docs/lead/<id>/gate-runs/`、`.github/workflows/collab-gates.yml`）要么保留并继续登记 owner，要么在同一 commit 内把已存在文件迁移/归档到仍受管路径；**不得让回滚后的 `docs/lead/releases/**` 等文件变成未登记路径**（否则既有证据变 `EVIDENCE_UNBOUND`）。
- 无业务数据回滚（本 ADR 不触碰业务数据）。

---

## 19. Consequences

- **正面**：治理 AC 从文字变成可执行输出；Review/Release 语义分离；门禁自保护与 IV 让“改脚本变绿”可见；L1 联签与 QC delta 有明确字段/失效规则；载体先登记后写入。
- **代价**：执行器新增约 4 类检查与记录输出（含 G-4、`--release-scope` 校验、per-change tier、head 绑定读取）；G-2 版本解析扩展 + 同版本摘要比较；**治理门禁拆为独立 workflow**（不再触发 `verify`/MySQL，但新增一个 workflow 文件与其路径维护）；审计/注册表/IV 记录需要持续维护；bootstrap 期间 CI 可能长时间为 NOT-RUN（预期，Release 阻断）。
- **成本口径已修正（Q-16）**：不声称“<10s”或“成本小”；治理 workflow 的实际耗时须在 P0b 实测并写入 gate record（§8.4）；提案的成本数字仍是估算，不作为 AC 阈值。
- **风险**：G-4 在 `BASE` 缺失（v1.4 已定 `NOT-RUN(not-implemented-at-BASE)`，非“待确认”）；GV-S18 载体已定稿（gate record 列）；版本格式缺口（§15-1）；同版本摘要比较依赖 G-2 实现（P0b）；CI runner 行为未实跑（§15-5）。这些均以 NOT-RUN/阻断处理，不放行。
- **明确不做**（重申）：不新增 G-5/G-6、不引入新依赖/平台、不做本地补丁安全网、不批量延期、不用 AC-REF 冒充 AC-VERIFIED、不由机器写 Release PASS。

---

## 20. 批准与回填

- **批准前置**：
  1. **SPEC-0003 v1.4 已批准**（2026-11-24，sha1 `58679f03…`）——**已满足**；若 v1.4 在本 ADR 批准前再变或升版，本 ADR 按头部“重对齐条款”升 Rev 并重审。
  2. **QC-02 对本 ADR（Rev 1.1）出结论**（原计划编号 “QC-06”；实际报告为 `docs/collab/qc-reports/ADR-0008-QC-02.md`）：**READY（0 BLOCKER / 0 MAJOR / 7 MINOR）**，7 项 MINOR 已编辑性应用，并确认 Q-01…Q-20 = 20/20 CLOSED——**已满足**。
  3. 人类 ADR 批准（GV-S15 `mode=approval`：`approved_by/approved_at/scope/evidence/digest`）——**已满足**：Director 于 2026-11-24 批准 Rev 1.1（§22）。
  4. Plan/Tasks（`GOV-COST-01`）经人类书面批准（L2，宪法 `:82`）——**仍待满足**（P0a 前）。
  5. **Independent Verifier 已按每次运行书面指定并写入 GV-S21**（Director 本人 `Director/direct`，或 `designated_by=<Director>` + `independence_evidence=<书面记录>@sha1`）；P0a 前至少一次——**仍待满足**（P0a 前）。
- **Architect 回填职责**：ADR 批准后，SPEC-0003 的 `adr: []` 必须回填为 `adr: [0008]`（Spec YAML 头的 `adr` 字段）。回填由 Architect 发起并提供精确值/摘要；因 Spec 文件 Owner=PO（`docs/collab/CONVENTIONS.md:23`），实际写入按所有权由 PO 执行（或由人类书面授权 Architect 写入并记录），不得由 Architect 静默改 Spec。`docs/specs/README.md` 的 ADR 列同样由 PO 登记。回填本身按 AC-102 绑定批准摘要；不得在 ADR 批准前回填。
- **未批准前的禁令（已于 2026-11-24 解除）**：批准前不得实现门禁/CI/脚本/模板/台账变更；不得创建注册表/审计区/IV 记录；不得以本 ADR 作为 Spec 批准或 Release 证据。**批准后**实现仍受 §18.1 R0…R6、Plan 书面批准（前置 4）与 IV 逐次指定（前置 5）约束；N-1 的 `## Governance audit log` 区段与 `mode=bootstrap` 批准行须在首个 P0a 提交前建立（§7.1/§12.1）。
- **状态**：本 ADR `Status: approved`（Director 于 2026-11-24 批准 Rev 1.1；§22），`Revision: Rev 1.1（approved，2026-11-24）`。ADR-0006/0007 头部使用 `accepted`，与本文件的 `approved` 同义（本文件按 Director 指令使用 `approved`）；后续任何实质修改必须升 Rev（Rev 1.2+）并依头部“重对齐条款”重审，不得原地改已批准文本。

---

## 21. Rev 1.1 变更记录（QC-05 Q-01…Q-20 处置）

> Rev 1.0（QC-05 被审稿）= 967 行、sha1 `0c67a163…`、HEAD `dff267d`；本 Rev 1.1 只改本 ADR，不改 SPEC-0003、不改 QC 报告、不写实现代码/脚本/CI YAML。

| QC-05 ID | 严重级 | 问题摘要 | 状态 | 修复落点 |
|---|---|---|---|---|
| Q-01 | BLOCKER | `BASE` 不可执行（G-3 读工作区 + 无显式 range） | **CLOSED** | §7.1 `BASE` 不变量（未跟踪 Spec、baseline 止于 AC-68、干净树）+ §7.2 `BASE` 投影树命令（`git worktree add --detach` + `--range=<BASE^>..<BASE>`）+ §7.3 GV-S21 证据字段（`base_tree_path/base_tree_digest/range/base_gate_summary`）；`git archive` 明确非 canonical |
| Q-02 | BLOCKER [CONTRACT_GAP] | ADR 改写 AC-112 的 G-4 口径 | **CLOSED** | 头部绑定**已批准** SPEC-0003 v1.4（697 行，sha1 `58679f03…`；QC-05 审稿 `79db9323…` + M-1…M-7）且 §7 全节以 v1.4 为唯一口径；删除“QC/人类认可口径”表述；§15-4/§16 的 ADR 编号 Q-04 改为“人类已裁决 + v1.4 明文”；批准稿新增的 GV-S21 `g4_status` 已并入 §5.1/§7.3 |
| Q-03 | BLOCKER [CONTRACT_GAP] | 文件名命名空间 vs L2 路径下限自相矛盾（误判 L1） | **CLOSED** | §4.5 per-change 记录内容规则 + 11 行 `PC-*` 同一张表（含 `ac-coverage.tsv`/`ac-baseline.txt`/QC 报告真实命名/GV-S19/GV-S20 行/README 行）；§4.8 引用同一张表；`--print-tier-rules` 输出 `TIER-RULE …` |
| Q-04 | BLOCKER | GV-S9/GV-S15/GV-S17 无 canonical 载体 | **CLOSED** | §5.1 三字段统一落 GV-S20 专用 `mode`（`waiver` / `approval\|bootstrap\|release` / `tier`）+ 固定 `evidence` 键；§4.5 定义 `effective_tier` 读取顺序与缺失/冲突/非人类 actor 行为；§6.11 G-4 输入/失效表 |
| Q-05 | MAJOR [CONTRACT_GAP] | GV-S20 追加被降为可选 | **CLOSED** | §6.3/§6.12：review/release 强制 `--audit-row-out`；CI 产工件、Lead/Reviewer 提交、G-1 机器核对；删除 `--audit-append` |
| Q-06 | MAJOR | `docs/lead/releases/**` 未入 MANAGED；`--release-scope` 无校验 | **CLOSED** | §5.2 MANAGED 增 `docs/lead/releases/`（owner=release owner，最长前缀）+ §5.4 七项校验表与阻断码 + §6.3 release 必需参数 |
| Q-07 | MAJOR [CONTRACT_GAP] | G-2 无同版本摘要校验；`Non-normative` 豁免过宽 | **CLOSED** | §6.11 G-2：治理契约同版本 blob 比较 ⇒ `DIGEST_MISMATCH`；`Non-normative` 仅限 L0 非治理文档，治理路径命中即 FAIL |
| Q-08 | MAJOR | G-3 读工作区/未跟踪文件，忽略 `--range` | **CLOSED** | §6.7 第 4/9 条（`git show <head>:`/`git ls-tree <head>`、移除 `--others`、`--range` 透传）+ §6.4 末条 |
| Q-09 | MAJOR | P0b 自保护机制从未被执行（local scope 不校验 GV-S21） | **CLOSED** | §6.8 第 7 条 + §7.1 P0b 出口 + §7.3 第 6 项：review-scope 运行或 IV 负向夹具二者至少一项 |
| Q-10 | MAJOR [CONTRACT_GAP] | `GATE_INCOMPLETE`(3) 与 `NOT-RUN`(4) 对同一事实双码 | **CLOSED** | §6.2 退出码表 + “同一事实只允许一个码/记录”映射表：P0b 后缺闸门=`GATE_INCOMPLETE`；bootstrap 期 G-4 未实现=IV 记录（不打码）；治理证据未 VERIFIED=`NOT-RUN` |
| Q-11 | MAJOR | Reviewer verdict 时间无机器字段，联签顺序不可机检 | **CLOSED** | §9.1/§9.2 `verdict_at` + `verdict_at ≤ qc.signed_at ≤ reviewer.signed_at` + 同一 `delta_digest`；缺失 ⇒ `SIGNOFF_MISSING` |
| Q-12 | MAJOR | GV-S18 载体未定；执行器“只读”声明与 `--record-out` 冲突 | **CLOSED** | §6.12 gate record TSV 列 + GV-S18=`release_block_reason` 列 + 写入边界（只写 record-out/audit-row-out；CI 写 `$RUNNER_TEMP`）；§15-2 |
| Q-13 | MINOR | 行号漂移（review-tracking、MANAGED） | **CLOSED** | 全文改区段标题引用；§1.2 行 16/18、§12.1、§15-8；MANAGED 引用校为 `:161` |
| Q-14 | MINOR | `gates-executor` 未登记 | **CLOSED** | §5.2 `ROLE_ALIASES` 增 `gates-executor` + `HUMAN_ONLY_MODES` + `docs/review-tracking.md` co-writer 列；提交行 `actor`=Reviewer/release owner、`evidence` 含 `produced_by=gates-executor` |
| Q-15 | MINOR | “执行器自检自身 diff”属自证 | **CLOSED** | §6.8 第 5 条删除自证，改为 IV 的 `diff_conclusion=FAIL` 由 G-1 读取阻断 |
| Q-16 | MINOR | `docs/**` 入 `go.yml` 会触发整个 workflow（含 MySQL verify）；成本声明失真 | **CLOSED** | §8.1 拆分为 `.github/workflows/collab-gates.yml`（只跑治理门禁，无 MySQL/Go）+ `go.yml` 移除三目录；§8.4 撤回“<10s”并改为 P0b 实测 |
| Q-17 | MINOR | G-0 未纳入输出契约 | **CLOSED** | §6.2 明确 `G-0 FAIL … is not a git repository` → `GATE_ERROR`（退出 2），非第五闸门；`SUMMARY G-0=…` |
| Q-18 | MINOR | GV-S20 缺 `spec_version`/`adr_revision` | **CLOSED** | §12.2 15 列表头 + 枚举/主键 + 机器解析口径；§6.9 记录字段 |
| Q-19 | MINOR | CI 命令未传 `RECORD_OUT` | **CLOSED** | §6.1/§8.1 `RECORD_OUT`/`AUDIT_ROW_OUT` 必需参数与完整命令；Makefile 透传 |
| Q-20 | MINOR | `--repo` 未绑定被审工作树 | **CLOSED** | §6.1/§6.4：`--repo` 必须含被审 head，回显 `REPO <toplevel>` 并写入 gate record `repo_path/repo_head`；不匹配 ⇒ `GATE_ERROR` |

**关闭统计**：Q-01…Q-20 **全部 CLOSED（20/20）**；无 deferred 项。实现落地时点见 §18.1（P0a/P0b/P0.5）。
**绑定沿革**：QC-05 审稿绑定 SPEC-0003 v1.4（696 行，sha1 `79db9323…`，in-review）；本 Rev 1.1 交付前该稿经 QC-05 READY（0/0/7）与 Director 批准，并入 M-1…M-7 编辑性澄清（含 M-3 `g4_status`、M-2 四项输出豁免）后为 697 行、sha1 `58679f03…`。本 ADR 已按**批准稿**重新对齐；若批准稿再变，按头部“重对齐条款”升 Rev（Rev 1.2+）并重审。
**AC 覆盖**：§17.3 仍为 **44/44（AC-69…AC-112）**，其中 AC-69/82/94/95/99/100/102/106/107/108/109/110/111/112 的机制描述已按 v1.4 与本节修订更新；AC 编号与数量未变。
**QC-02 编辑性澄清（N-1…N-7 = M-1…M-7）**：QC-02 复审 Rev 1.1 为 **READY（0 BLOCKER / 0 MAJOR / 7 MINOR）**、Q-01…Q-20 = 20/20 CLOSED；7 项 MINOR 已在**同一批准轮**作为编辑性澄清并入（落点见 §22），仅涉记录/时序/字段一致性，**不改变**本表任何设计决策、AC 覆盖（44/44）或 SPEC-0003 v1.4 绑定。
**未改动的文件**：本 Rev 1.1（含批准轮 `Status: approved` 与 N-1…N-7）只修改 `docs/adr/0008-governance-cost-reduction.md`；SPEC-0003、QC 报告、脚本、CI YAML、台账均未改动。

---

## 22. Approval（批准记录）

| 项 | 值 |
|---|---|
| 批准对象 | ADR-0008 `docs/adr/0008-governance-cost-reduction.md` **Rev 1.1** |
| 决定 | **`Status: approved`** |
| approved_by / approved_at | **Director** / **2026-11-24** |
| QC 依据 | `docs/collab/qc-reports/ADR-0008-QC-02.md`：**READY FOR HUMAN ADR APPROVAL（0 BLOCKER / 0 MAJOR / 7 MINOR；Q-01…Q-20 = 20/20 CLOSED）**；前序 QC-05 对本 ADR Rev 1.0 为 BLOCKED（4/8/8） |
| MINOR 处置 | QC-02 的 7 项 MINOR（N-1…N-7 = 该报告 §6 的 M-1…M-7）**全部作为编辑性澄清并入本轮**：N-1 §7.1 P0a 前置 + `BASE` 定义 + §12.1 时点；N-2 §7.1 不变量第 6 条 + 失败处置；N-3 §7.2 投影提交四 trailer + 说明；N-4 §6.11 G-2 新增文件分支 + ADR 版本字段统一为 `Revision`；N-5 §10.1 并入 AC-111 三字段；N-6 §6.12 “head 或 §6.4 允许的祖先” + 首次运行预期；N-7 §6.8 第 7 条 + §7.1 P0b + §7.3 自检预期退出 4 |
| 上游绑定 | SPEC-0003 **v1.4（approved 2026-11-24）**：**批准内容摘要 = `58679f03e417b98a591545c84854cda3aaa6b898`（697 行）**；批准后仅一次元数据回填（`adr: [] → [0008]`；工作区 698 行、sha1 `1371eafc…`，无 AC/INV 变更）；**review/Release 的 canonical 绑定 = P0a 提交内的 SPEC-0003 blob**（未跟踪工作区文件不算证据）；本 ADR 仍只绑定 v1.4，不解释/不改写其 AC 文本 |
| 覆盖 | **AC-69…AC-112 = 44/44**（§17.3 表未改）；AC-01…68 / INV-01…50 零语义变更；D-CR-6 / D-CR-7 仍 OPEN 且保持保守默认（§8.3/§16） |
| 语义边界 | ADR 批准 **≠** 实现通过 **≠** AC-VERIFIED **≠** Release PASS；`RELEASE_ELIGIBLE=false` 直到 44/44 治理 AC VERIFIED；批准不改变任何现行门禁行为 |
| 批准后义务 | Lead 在首个 P0a 提交前建 `## Governance audit log` 与 `mode=bootstrap` 批准行（N-1）；Plan/Tasks 人类书面批准 + IV 逐次书面指定（§20 前置 4/5）；SPEC-0003 的 `adr: [] → [0008]` 回填由 PO 执行、Architect 提供精确值/摘要（§20） |
