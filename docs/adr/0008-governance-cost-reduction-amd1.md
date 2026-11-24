# ADR-0008 Amendment 1 — CI 对门禁退出码 4（NOT-RUN）的显式处理

| 项 | 值 |
|---|---|
| ADR ID | 0008 Amendment 1（`docs/adr/0008-governance-cost-reduction-amd1.md`） |
| Status | **withdrawn（2026-11-25）**：**从未实现**（never implemented）；协作闸门工具链已于 2026-11-25 整体退役（原 approved：Director 2026-11-25 批准 Amendment 1 Rev 1.1） |
| Base | **ADR-0008 Rev 1.1**（approved 2026-11-24）；本文件不重写其正文，仅按 §4.1 S-1…S-3 对其 §8.1 作定点修订 |
| Revision | **1.1**（approved，2026-11-25；Amendment 1 Rev 1.1） |
| Date | 2026-11-25 |
| Owner / Integration owner | System Architect（`docs/adr/**` 所有权见 `docs/collab/CONVENTIONS.md:25`） |
| Spec | **SPEC-0003 v1.4（approved 2026-11-24）**；本 Amendment 只解释 CI 对退出码的处理，不改任何 AC 文本 |
| Change ID | `GOV-COST-01` |
| 决策来源 | Director 2026-11-25“CI NOT-RUN policy”：`docs/review-tracking.md` 区段 **“2026-11-24 human decisions: GOV-COST-01 bootstrap, IV and commit timing”**（当前 `:175-182`）；该记录明确“Requires ADR-0008 **Amendment 1** + QC + Director approval before T-B7 wiring; until then P0b is not committed”（该前置已于 2026-11-25 满足：QC-02 READY + Director 批准） |
| Review | QC-01 对 Rev 1.0：**BLOCKED（1 MAJOR / 5 MINOR）**，Rev 1.1 整改后 6/6 CLOSED（§10）；**QC-02 对 Rev 1.1：READY（0 BLOCKER / 0 MAJOR / 5 MINOR）**，5 项 MINOR 与批准同轮编辑性并入（§11）；**Director 2026-11-25 批准** |
| 覆盖 | CI 退出码映射（§2）、Release/tag 门槛（§3）、workflow 契约与门禁 job 唯一归属（§4）、反绕过（§5）；含对 ADR-0008 §8.1 的定点修订（§4.1 S-1…S-3） |
| 生效状态 | ① QC-02 READY（0 BLOCKER / 0 MAJOR / 5 MINOR）——已满足；② Director 书面批准（GV-S15，2026-11-25）——已满足；③ T-B7 可接线，但仍受 AC-100/AC-107 与 Q-01…Q-04 默认处置约束 |
| 与 ADR-0008 Rev 1.1 的关系 | 增量 Amendment：**只解释** CI 记账，并**定点修订** §8.1（移除 `go.yml` 的 `collab-gates` job、扩展 `collab-gates.yml` 路径覆盖、同步 G-1 静态核对）。**§15-6 不受影响**（§6.2） |

- Revision: **1.1**（approved, 2026-11-25；Amendment 1 Rev 1.1；本行为 G-2 机器读取行，口径同 ADR-0008 §6.11 N-4）

> **Withdrawn（2026-11-25）**：本 Amendment 从未实现；协作闸门（含 `collab-gates` CI job 与执行器退出码记账）已于 2026-11-25 整体退役。

> **边界声明**：执行器退出码本身不变（4 仍是 4），四闸门与 AC 语义不变，Review/Release 的结论门槛不变，任何 merge/landing 政策不变（§15-6 原样）。改变的是 **CI job 成功/失败的记账方式**，且必须以“`RELEASE_ELIGIBLE=false` 被记录、工件被上传、运行被注解”为前提。

---

## 0. 阅读方式与规范边界

1. **只增不改**：不删除、不重编号 ADR-0008 Rev 1.1 的 §/D-08/AC/INV/GV-S；仅 §8.1 的三处定点修订（S-1…S-3）显式取代原文对应表述，未列出者继续有效。
2. **不改契约**：不改 SPEC-0003 v1.4 的任何 AC/INV；不改 `scripts/collab_gates.py` 的退出码定义；不新增 G-5/G-6 或第二执行器。
3. **不写实现**：本文件只给 workflow 契约；`.github/workflows/collab-gates.yml`、`go.yml` 的 job 移除与 Makefile 透传由 Developer 在 T-B7 实现，并受 AC-100/AC-107 自保护约束。
4. **不产生 PASS**：本文件的批准不产生实现 PASS、AC-VERIFIED、Review/Release PASS 或 merge 授权。
5. **唯一改动载体**：本文件是本次唯一新增/修改文件；ADR-0008、SPEC-0003、脚本、CI、Makefile、台账、QC 报告均未改动。

---

## 1. Context

### 1.1 事实基线（2026-11-25）

| 事实 | 实际值 | 证据 |
|---|---|---|
| 治理 AC 尚未验证 | AC-69…AC-112 共 **44** 行，`test_type=pending`、`evidence_path=-`、`status=PENDING`（非 DEFERRED） | `docs/collab/ac-coverage.tsv:1`（表头）、`:38-81`（AC-69…AC-112） |
| 退出码已定义 | `EXIT_PASS=0`、`EXIT_FAIL=1`、`EXIT_GATE_ERROR=2`、`EXIT_GATE_INCOMPLETE=3`、`EXIT_NOT_RUN=4` | `scripts/collab_gates.py:70-75` |
| 4 的产生条件 | 任一闸门 `NOT-RUN` ⇒ 返回 4，并打印 `RELEASE_ELIGIBLE false` / `RELEASE_BLOCK_REASON <code>` / `SUMMARY … exit=4` | `scripts/collab_gates.py:1315-1316`、`:1327-1328`、`:1341-1343`；ADR-0008 §6.2 `:433`、`:442` |
| 4 的契约语义 | 阻断 Release；不得当 PASS；`RELEASE_ELIGIBLE=false` | ADR-0008 §6.2 `:433`、`:447-448`；§6.3 `:459`、`:464` |
| 4 是正常工作态 | bootstrap 的 review-scope 自检**预期**整体退出 4，不得当自检失败或 bootstrap PASS | ADR-0008 §6.8 第 7 条 `:527-531`；§7.3 `:726-727` |
| 实测：执行器 4 / make 2 | `G-3 NOT-RUN 44 governance AC(s) not VERIFIED`；`RELEASE_ELIGIBLE false`；`RELEASE_BLOCK_REASON GOV_AC_EVIDENCE_MISSING`；`SUMMARY … exit=4`；**`make` 自身退出 2**（`make: *** [gates-strict] Error 4`） | 2026-11-25 运行 `make gates-strict`，HEAD `131c0e0` + 工作树 P0b 执行器改动；QC-01 §5 独立复现 |
| 既有第二个 gate job | `.github/workflows/go.yml` 的 `collab-gates` job（`:26-33`）只跑 `make gates-strict`，同样会拿到 4 ⇒ make 折叠 2 ⇒ 红 | `go.yml:26-33`；QC-01 AMD1-M1 |
| 治理 workflow 尚未创建 | `.github/workflows/` 仅有 `frontend.yml`、`go.yml` | `ls .github/workflows/`；QC-01 §5 |
| Makefile 透传/退出回显未落地 | `gates-strict:` 仅 `./scripts/check-collab-gates.sh --mode=strict` | `Makefile:162-169` |

### 1.2 问题

P0b 执行器落地后，AC-69…AC-112 在 P1/P2 逐条验证前保持 `PENDING`，因此 review-scope 的 `make gates-strict …` 会输出 `G-3 NOT-RUN` 并退出 4。ADR-0008 §6.2/§8.1 只规定“4 阻断 Release、不得当 PASS”，**没有规定 CI job 应把 4 记为成功还是失败**。

若按“review scope 要求四闸门全 PASS”的字面读法（§6.2 `:448`、§6.3 `:459`）把 4 当 job 失败，则每个受管提交都会让治理 CI 检查变红——而该红色不表示任何真实违规。**问题不限于新 workflow**：`.github/workflows/go.yml` 的既有 `collab-gates` job（`:26-33`）同样执行 `make gates-strict`，P0b 之后任何代码/脚本类 PR（含 P0b 自身）也会得到 4 → make 折叠 2 → 红（QC-01 AMD1-M1）。因此必须**同时**处置两个 job 的归属，而不是只给新 workflow 写映射。

本 Amendment **不改变任何 merge/landing 政策**：ADR-0008 §15-6（`:1057`）原样有效，CI 绿不等于 merge 批准（§6.2）。

### 1.3 人类裁决（2026-11-25）

Director 决定（`docs/review-tracking.md` 上述区段 `:175-182`）：CI 门禁 job **接受执行器退出 4（NOT-RUN）作为成功 job**，同时记录 `RELEASE_ELIGIBLE=false`、上传 gate 工件、注解运行；**Release/tag 仍要求 exit 0**；`--mode=warn` 与 `continue-on-error` **继续禁止**。理由：main 必须在治理 AC 增量验证期间保持绿色，且**不得**把 NOT-RUN 当 PASS。

该裁决**只覆盖 CI job 记账**，不包含 merge/landing 时点裁决（QC-01 m2）；本 Amendment 不据此扩张。

### 1.4 为什么不能退化为 warn / `continue-on-error`

| 方案 | 为什么禁止 |
|---|---|
| `--mode=warn` | 执行器在 warn 下强制 `exit_code = EXIT_PASS`（`scripts/collab_gates.py:1323-1325`）；零退出抹掉 4，且 §6.2 `:447` 明文“`mode=warn` 零退出不能算 PASS” |
| `continue-on-error: true` | 把 1/2/3/124 **一并**变绿，无法区分“治理证据未齐”与“真实违规/崩溃”；§8.1 `:762` 明文禁止 |
| `\|\| true` / `exit 0` 兜底 | 与 `continue-on-error` 等价，且不留 `RELEASE_ELIGIBLE=false` 的机器记录 |

本 Amendment 的映射与上述方案的本质区别：**退出码原样保留（4 不被改写为 0）**，job 的成功只是“本次运行不构成 CI 失败”的记账，且必须以记录与工件为条件。

---

## 2. Decision：CI 退出码映射（A1-01…A1-08）

### 2.1 映射表（唯一权威；适用于**唯一**的治理 gate job）

本表适用于 `.github/workflows/collab-gates.yml` 的 `collab-gates` job。按 §4.1 S-1，`go.yml` **不再保留** gate job；因此不存在“两个 job 各有一套解释”的分叉。

| 执行器退出 | 语义（ADR-0008 §6.2） | CI job 结果 | 必须记录 | 绝对禁止 |
|---|---|---|---|---|
| **0** | 本次 scope 要求的闸门全部 PASS | **success** | stdout、gate record、audit row；`RELEASE_ELIGIBLE` 按执行器输出原样记录；`gate_exit=0`/`gate_status=PASS` | 当作 bootstrap PASS / Release PASS / merge 批准 |
| **4** | `NOT-RUN`（如治理 AC 未 VERIFIED） | **success + NOT-RUN 注解** | `gate_exit=4`/`gate_status=NOT-RUN`、`RELEASE_ELIGIBLE=false`、执行器原样 `RELEASE_BLOCK_REASON`、三工件、step summary 明示 release-blocked | 当 gate PASS / AC-VERIFIED / Review PASS / Release 证据；静默转 0 并隐藏 block reason |
| **1** | 任一闸门 FAIL 或存在阻断码（**含首跑 `EVIDENCE_UNBOUND`（GV-S20）与 `GATE_NOT_RUN`（GV-S21），见 A1-07**） | **failure** | stdout、gate record（如已产出）；按 A1-07 的标题注解来源与处置指引 | 降级为 success；用 `continue-on-error`/warn 消除首跑红 |
| **2** | `GATE_ERROR`（G-0/usage/git/IO/`--repo` 不匹配） | **failure** | stdout 等可得的工件 | 降级为 success |
| **3** | `GATE_INCOMPLETE`（闸门缺失/未执行/未输出、CI 路径缺失） | **failure** | 同上 | 降级为 success |
| **124** | `GATE_TIMEOUT`（wrapper timeout） | **failure** | 同上 | 降级为 success |
| 其他/缺失/不可解析 | 未知 | **failure（fail-closed）** | 原始输出 | 猜测为 0 或 4 |

### 2.2 决策条款

- **A1-01（只改记账）**：执行器退出码集合与语义不变；本文件只定义 CI job 的 success/failure 记账。`4 ⇒ job success` 的前提是 A1-03 的记录条件全部满足。
- **A1-02（退出码必须来自显式通道）**：job **不得**用 `make` 的退出状态推断执行器退出码——`make` 对任何 recipe 失败一律返回 2（§1.1 实测 `make: *** [gates-strict] Error 4`，`$?=2`）。workflow 必须读取 Makefile 回显的显式通道（§4.3 R-1）；通道缺失/不可解析 ⇒ job **failure**。
- **A1-03（4 的成功是带条件的）**：仅当 ① 退出码 = 4；② 执行器输出 `RELEASE_ELIGIBLE false`；③ gate record 与 audit row 工件非空；④ 注解与 step summary 已写入——四者同时成立，job 才可 success。任一缺失 ⇒ job **failure**（视为 `GATE_ERROR`）。
- **A1-04（失败仍上传工件）**：1/2/3/124/未知的 job 为 failure，但工件仍以 `if: always()` 上传，供 Reviewer/IV 排查；上传失败不改变 job 的 failure 结论。
- **A1-05（一致性断言，fail-closed）**：若 `exit=4` 但 `RELEASE_ELIGIBLE` 不是 `false`（执行器输出缺失或为 `true`），job 必须 failure 并打印 `::error::`，不得 success。
- **A1-06（不得复用结论）**：exit 4 的运行不得被引用为 Review PASS 或 Release 证据；Review 结论仍要求四闸门全 PASS 即 exit 0（§6.2 `:448`、§6.3 `:459`），本 Amendment 不放宽该门槛。
- **A1-07（首跑红的两种预期路径与重跑序列；QC-01 m1、QC-02 m2）**：P0b 之后 review-scope 首次运行可能返回 **1（G-1 FAIL）** 而非 4，来源有且仅有两种，处置相同：
  - **① GV-S20 审计行缺失**（`mode=gate` 行未提交；§6.3/§6.12）：任意 review 运行的 head 尚无本次 `change_id+base+head+mode` 行时，执行器返回 `EVIDENCE_UNBOUND` ⇒ exit 1；注解标题 `GOV-GATES-EVIDENCE-UNBOUND`，正文“预期首跑：提交 `--audit-row-out` 工件行 → 同键重跑”。
  - **② GV-S21/IV 记录缺失**（gate/CI 变更；§6.8/AC-107）：PR 触及 `scripts/**`、`.github/**`、`Makefile`、门禁/canonical，且被审 head 尚无本 change 的已提交 GV-S21 记录（**含 P0b/T-B7 自身**）时，G-1 以 `GATE_NOT_RUN` 阻断 ⇒ exit 1；注解标题 `GOV-GATES-NO-IV-RECORD`，正文“预期首跑：基线运行 → 变更后运行 → IV 写 GV-S21 并提交 → 最终 head 重跑”。
  - **共同处置**：① job **仍为 failure**（不得 success）；② 按上述标题注解，不得降级为 `::notice`；③ 提交对应载体（GV-S20 行或 GV-S21 记录；新 head 仅追加 per-change 载体且 `gate_self_digest` 不变，§6.4 祖先规则）；④ 重跑后应为 exit 4（NOT-RUN）或 exit 0；⑤ **禁止**用 `continue-on-error`/warn/4→0 消除该红；⑥ 不得把该红记为实现失败（Reviewer/IV 在记录中注明“首跑前置缺失”）。该预期在对应检查落地后生效（当前工作树执行器尚未实现 GV-S20 审计行检查与 GV-S21 校验，QC-01/QC-02 实测 review 仍为 exit 4）。
- **A1-08（单一 gate job，QC-01 M1）**：治理门禁只允许一个 job。T-B7 MUST 删除 `go.yml:26-33` 的 `collab-gates` job，由 `collab-gates.yml` 覆盖代码+治理路径（§4.1 S-1…S-3）；不得保留双 job，也不得只给新 workflow 写映射而让 `go.yml` 的 job 继续裸跑 `make gates-strict`。

---

## 3. Release / tag 门槛

1. 任何 Release/Publish/tag workflow **MUST** 以 **release-scope** 的 `exit=0`（且 `RELEASE_ELIGIBLE=true`）为前置；`exit=4` 一律记 `RELEASE_BLOCKED` 并阻断。
2. **机器可读门槛（QC-01 m3；QC-02 m1/m5）**：Release job **不得**以 prose、job conclusion 或“CI 绿”为放行条件。唯一机器条件 = **`gate_exit == '0'`**；但 GitHub Actions 的 `needs` **只在同一 workflow 内可见**，因此按落点二选一：
   - **① 同 workflow**：Release job 与 `collab-gates` 同处 `.github/workflows/collab-gates.yml`，用 `needs: collab-gates` 且 `if: needs.collab-gates.outputs.gate_exit == '0'`（tag 发布再合并 `github.ref_type == 'tag'`）；
   - **② 独立 workflow**：`on: workflow_run`（`workflows: [collab-gates]`、`types: [completed]`），用 `actions/download-artifact@v4`（`run-id: ${{ github.event.workflow_run.id }}` + token）下载该 run 的 artifact 并读取 `gate_exit`，要求 `gate_exit == '0'`；**MUST NOT** 只用 `workflow_run.conclusion == 'success'`（exit-4 的 job 也是 success）。
   无论 ①/②：同时读取**已提交**的 gate record（GV-S18 的 `release_block_reason`）与 GV-S20 行；任一缺失/不符 ⇒ 不得 Release。**`gate_exit != '0'`（含空值、`missing` 与 job 失败）时 Release 不得运行**（m5）。
3. **不得由 NOT-RUN 派生 Release PASS**：`exit=4` 的运行即使 job 为 success，也不得写 `release_verdict=PASS`；`release_verdict` 仍只由人类（GV-S6）写入（§6.3 `:466`）。
4. **review-scope exit 0 也不授权 Release**：review 输出恒为 `RELEASE_ELIGIBLE=false`（§6.3 `:464`）；Release 必须消费 release-scope 运行并满足 §5.4/§6.3。
5. **当前无 Release workflow 时**：Release 仍由人类 release owner + IV 按 GV-S6 放行；人类 MUST 以 release-scope `exit=0`（或 §5.4 的等价机器校验）为前置，不得以 collab-gates job success 代替（Q-02）。

---

## 4. Workflow 契约（T-B7）

### 4.1 门禁 job 的唯一归属与路径覆盖（A 方案；QC-01 M1）

**决策：A 方案。** T-B7 MUST 把治理门禁收敛为**唯一** job：删除 `.github/workflows/go.yml` 的既有 `collab-gates` job（`:26-33`），由 `.github/workflows/collab-gates.yml` 对**治理路径与代码路径**统一执行门禁。理由：只要 `go.yml` 保留 gate job，P0b 后代码/脚本 PR 同样拿到 exit 4 → make 折叠 2 → 红，“main 保持绿”不成立（QC-01 AMD1-M1）。

以下三处是对 ADR-0008 §8.1 的**定点修订（S-1…S-3）**，修订后 §8.1 的对应表述以本节为准：

| ID | 修订点 | 精确契约 |
|---|---|---|
| **S-1** | `go.yml` 不再有 gate job | 删除 `.github/workflows/go.yml:26-33` 的 `collab-gates` job；`go.yml` 只保留 `verify`（含 MySQL service），其 `paths` 仍为代码路径、**不含** `docs/**`/`prompt/**`/`.specify/**`。ADR-0008 §8.1 `:748` 的“`go.yml`（修改）… `collab-gates`（保留现有 job）”由本行取代 |
| **S-2** | `collab-gates.yml` 覆盖两类路径 | `on.push.paths` 与 `on.pull_request.paths` = 治理集 ∪ 代码集：`docs/**`、`prompt/**`、`.specify/**`、`.github/workflows/collab-gates.yml`、`.github/workflows/go.yml`、`scripts/**`、`Makefile`、`**/*.go`、`go.mod`、`go.sum`、`api/**`、`migrations/**`。`scripts/**` 覆盖 `scripts/collab_gates.py`/`scripts/check-collab-gates.sh`；代码 PR 因此仍被门禁，exit-4 映射同 §2 |
| **S-3** | G-1 静态核对同步 | 在 ADR-0008 §8.1 `:763` 的静态核对中追加：a) `go.yml` 的 `jobs:` 块**不含** `collab-gates:` 且不含 `make gates`/`check-collab-gates` 调用；b) `collab-gates.yml` 的 `paths` 同时含治理集与代码集（沿用 §15-14 的文本扫描口径，不引入 PyYAML）；c) job 仍执行 `make gates-strict` 且带 `RECORD_OUT`/`AUDIT_ROW_OUT`。违反 a/b/c 任一 ⇒ `GATE_INCOMPLETE` |

- **实现边界**：S-1…S-3 触及 `.github/**` 与 `scripts/**`（G-1 静态核对），属 **L2 + AC-100/AC-107**：基线版本与变更后各跑一次 + IV 逐行 diff + GV-S21；不得由被审 diff 内执行器自证。
- **不得留给 Developer 选择**：T-B7 出口必须能证明 A 已落地（`go.yml` 无 gate job、`collab-gates.yml` 覆盖两类路径、一次代码路径 PR 的 exit-4 运行记录）。若未来要恢复双 job（QC-01 选项 B），必须**新 Amendment**，并定义单一共享退出码定义 + anti-divergence 检查；本 Rev 1.1 **不采用 B**。
- **S-2 的流程后果（QC-02 m4）**：代码 PR 也进入 review-scope，因而**同样**需要 `change_id`、change-package base/head 一致（§6.3）、显式 range（§6.4）与 GV-S20 行；缺失时按 §7 的注解 failure，不得当“轻量检查”。
- **成本**：代码 PR 会额外跑 1 个轻量治理 job（无 MySQL/Go）；`verify` 的触发模型不变（§8.1 `:743`）。成本按 §8.4 不设阈值。

### 4.2 不变部分（引用 ADR-0008 §8.1）

- `collab-gates.yml`：`fetch-depth: 0`；只用 runner 自带 `python3`+`git`；**无** MySQL service、**无** Go 构建；触发路径按 S-2。
- 命令：`make gates-strict SCOPE=review RANGE="<base>..<head>" RECORD_OUT=… AUDIT_ROW_OUT=…`（§8.1 `:750-757`）；三个 artifact：gate record、stdout、audit row（§8.1 `:761`）。
- G-1 其余静态核对项（workflow 存在、job 仍执行 `make gates-strict`、`RECORD_OUT`/`AUDIT_ROW_OUT` 未被删）**不变**（§8.1 `:763`）；新增核对见 S-3。

### 4.3 新增部分（本 Amendment）

**R-0（R-2 的前置 step）range/change 解析，必须在 gate step 之前失败关闭（QC-02 m3/m4）**：workflow 先解析 `RANGE`、`CHANGE_ID`、`base7`、`head7` 并写入 `steps.range.outputs`。`CHANGE_ID` 为空（PR 标签/branch 映射缺失）⇒ 打印 `CHANGE_ID_UNBOUND`、`::error::`、`gate_exit=missing`、`gate_status=UNKNOWN` 并 **`exit 1`，不进入 gate step、不写 gate record**；`RANGE` 无法按 §8.1 `:759` 绑定 ⇒ 同样 failure。R-2 的 `*` 分支只是兜底，**不得**作为 `change_id` 缺失的唯一落点（否则 `RECORD_OUT` 会出现 `gate-runs//…` 空段并被误判 success）。

**R-1 退出码通道**：`gates-strict` recipe **MUST** 在执行器结束后回显其真实退出码，且 `make` 自身仍失败（本地语义不变）。唯一约定通道：stdout 中的独立行

```text
GATE_EXIT_CODE <0|1|2|3|4|124>
```

（可选等价实现：同时写 `$GATE_EXIT_OUT` 文件，内容同上；如两处不一致 ⇒ fail-closed。）执行器 stdout 与 gate record 已作为 artifact 上传，故该行可被第三方复核。此通道**不是**新退出码，也不改变执行器行为。

**R-2 分支逻辑**（workflow step 伪代码；示意，不代替实现，省略 checkout 与 range 解析步骤）：

```yaml
- name: Governance gates (scope=review, explicit exit handling)
  id: gates
  shell: bash
  env:
    RANGE_IN: ${{ steps.range.outputs.range }}      # PR: origin/<base_ref>..HEAD；push: before..sha
    BASE7: ${{ steps.range.outputs.base7 }}
    HEAD7: ${{ steps.range.outputs.head7 }}
    # canonical 命名（ADR-0008 §5.1/§6.12）：<base7>-<head7>-<scope>.tsv
    RECORD_OUT: ${{ runner.temp }}/gate-runs/${{ steps.range.outputs.change_id }}/${{ steps.range.outputs.base7 }}-${{ steps.range.outputs.head7 }}-review.tsv
    AUDIT_ROW_OUT: ${{ runner.temp }}/gate-runs/${{ steps.range.outputs.change_id }}/${{ steps.range.outputs.base7 }}-${{ steps.range.outputs.head7 }}-review.audit.tsv
  run: |
    set +e
    make gates-strict SCOPE=review RANGE="$RANGE_IN" \
      RECORD_OUT="$RECORD_OUT" AUDIT_ROW_OUT="$AUDIT_ROW_OUT" \
      > "$RUNNER_TEMP/gates.stdout" 2>&1
    set -e
    code="$(sed -n 's/^GATE_EXIT_CODE //p' "$RUNNER_TEMP/gates.stdout" | tail -n1)"
    eligible="$(sed -n 's/^RELEASE_ELIGIBLE //p' "$RUNNER_TEMP/gates.stdout" | tail -n1)"
    reason="$(sed -n 's/^RELEASE_BLOCK_REASON //p' "$RUNNER_TEMP/gates.stdout" | tail -n1)"
    case "$code" in
      0)
        echo "::notice title=GOV-GATES-PASS::exit 0; RELEASE_ELIGIBLE=$eligible"
        echo "gate_exit=0" >> "$GITHUB_OUTPUT"; echo "gate_status=PASS" >> "$GITHUB_OUTPUT"
        echo "release_eligible=$eligible" >> "$GITHUB_OUTPUT"
        ;;
      4)
        [ "$eligible" = "false" ] || { echo "::error::exit 4 with RELEASE_ELIGIBLE=$eligible"; exit 1; }
        [ -s "$RECORD_OUT" ] && [ -s "$AUDIT_ROW_OUT" ] || { echo "::error::exit 4 but gate artifacts missing"; exit 1; }
        echo "::warning title=GOV-GATES-NOT-RUN::exit 4 (NOT-RUN); RELEASE_ELIGIBLE=false; RELEASE_BLOCK_REASON=$reason"
        {
          echo "## Governance gates: NOT-RUN (exit 4)"
          echo ""
          echo "**RELEASE_ELIGIBLE=false** — release-blocked. NOT-RUN is not PASS."
          echo "- scope=review range=$RANGE_IN"
          echo "- RELEASE_BLOCK_REASON=$reason"
          echo "- artifacts: gate record / stdout / audit row"
        } >> "$GITHUB_STEP_SUMMARY"
        echo "gate_exit=4" >> "$GITHUB_OUTPUT"; echo "gate_status=NOT-RUN" >> "$GITHUB_OUTPUT"
        echo "release_eligible=false" >> "$GITHUB_OUTPUT"
        ;;
      *)
        echo "::error::governance gate exit=${code:-missing} (expected 0 or 4)"
        # 首跑 EVIDENCE_UNBOUND（A1-07）也落入此分支：failure 不变，注解给出重跑序列
        [ "$code" = "1" ] && echo "::warning title=GOV-GATES-EVIDENCE-UNBOUND::expected first-run? commit --audit-row-out row to the reviewed head, then re-run the same change_id+base+head+mode; do not add continue-on-error"
        case "$code" in
          1) status=FAIL ;; 2) status=GATE_ERROR ;; 3) status=GATE_INCOMPLETE ;;
          124) status=GATE_TIMEOUT ;; *) status=UNKNOWN ;;
        esac
        echo "gate_exit=${code:-missing}" >> "$GITHUB_OUTPUT"
        echo "gate_status=$status" >> "$GITHUB_OUTPUT"
        exit 1
        ;;
    esac
```

> R-2 的 `*` 分支 MUST 依 stdout 的失败码选择注解标题：`CHANGE_ID_UNBOUND`（R-0 已拦截）、change-package/`RANGE_UNBOUND`（§6.3/§6.4）→ `GOV-GATES-CHANGE-PACKAGE`、`EVIDENCE_UNBOUND`（GV-S20）→ `GOV-GATES-EVIDENCE-UNBOUND`、`GATE_NOT_RUN`（GV-S21）→ `GOV-GATES-NO-IV-RECORD`；未识别时用通用 `GOV-GATES-FAIL`。注解标题不得把首跑红改写成实现失败。

**R-3 工件上传**（`if: always()`，与 `continue-on-error` 无关）：

```yaml
- uses: actions/upload-artifact@v4
  if: always()
  with:
    name: collab-gates-${{ github.run_id }}-${{ github.run_attempt }}
    path: |
      ${{ runner.temp }}/gates.stdout
      ${{ runner.temp }}/gate-runs/
    if-no-files-found: error
    retention-days: 30          # 仅可见性/排障；canonical 见 R-4 与 m5
```

**R-4 记录与可见性（含 m3/m4/m5）**：

- exit 4 时 step summary **MUST** 含 `RELEASE_ELIGIBLE=false` 与执行器原样的 `RELEASE_BLOCK_REASON`；注解 **MUST** 用 `::warning`（non-failing）；**MUST NOT** 用自定义原因覆盖执行器打印的 block reason，也 **MUST NOT** 删改 stdout 中的 `G-3 NOT-RUN …` 行。
- **机器可读门槛（m3）**：job 必须暴露 outputs（step → job 逐层透传）：

```yaml
outputs:
  gate_exit: ${{ steps.gates.outputs.gate_exit }}          # "0" | "4" | "1" | ... | "missing"
  gate_status: ${{ steps.gates.outputs.gate_status }}      # PASS|NOT-RUN|FAIL|GATE_ERROR|GATE_INCOMPLETE|GATE_TIMEOUT|UNKNOWN
  release_eligible: ${{ steps.gates.outputs.release_eligible }}
```

  Release/Publish job 的机器条件 = `gate_exit == '0'`：同 workflow 用 `needs.collab-gates.outputs.gate_exit`；独立 workflow **必须**走 `workflow_run` + 下载 artifact 读取 `gate_exit`（`needs` 不跨 workflow；QC-02 m1）。**`gate_exit != '0'`（含空值、`missing`、job 失败）⇒ Release 不运行**（§3.2、m5）。这是 CI 层镜像；**canonical 机器字段**仍是提交进仓库的 gate record 的 `release_block_reason`（GV-S18）与 GV-S20 行。
- **canonical 命名（m4）**：CI 的 `RECORD_OUT` 直接使用 `<base7>-<head7>-review.tsv`（与 `docs/lead/<change_id>/gate-runs/` 的 canonical 文件名一致），提交时**无需改名**、字节不变；`audit row` 为 `<base7>-<head7>-review.audit.tsv`（§8.1 `:756` 口径）。
- **canonical 证据 vs artifact（m5）**：Release/审计以**提交进仓库**的 gate record 与 GV-S20 `mode=gate` 行为准（§6.12）；artifact 与 `retention-days: 30` 只承担可见性与排障，过期不影响 canonical 记录的存在性（但过期后无法再复核 CI 现场，Q-03 仅决定保留期）。

**R-5 一致性**：workflow 的成功不得被描述为“四闸门 PASS”；任何 `needs: collab-gates` 的 Release/Publish job 必须再加 `gate_exit == '0'` 条件（§3.2、R-4）。

---

## 5. 反绕过（Anti-bypass）

### 5.1 禁止清单（出现即该次运行无效；由 IV/Reviewer 复核）

| 禁止项 | 违反点 |
|---|---|
| gate step/job 上的 `continue-on-error: true` | 把 1/2/3/124 一并变绿；§8.1 `:762` |
| `make gates` / `--mode=warn` 进入 CI | warn 强制 0（`scripts/collab_gates.py:1323-1325`）；§6.2 `:447` |
| `\|\| true`、`\|\| :`、`set +e` 后不判断、脚底 `exit 0` | 等价于 `continue-on-error` |
| 把 4 记为 0、删除/覆盖 `GATE_EXIT_CODE` 行、删除退出码通道 | 隐藏 `RELEASE_ELIGIBLE=false` |
| `go.yml` 保留 gate job，或双 job 各自解释退出码（QC-01 B 未批准） | 代码类 PR 仍会红；§4.1 S-1、A1-08 |
| exit 4 时省略 artifact 或跳过注解仍 success | 无记录=无证据（A1-03） |
| 用 exit 4 的运行声称 AC-VERIFIED / gate PASS / Review PASS / Release / merge 批准 | 违反 AC-69/70/72/112 与 §6.2 |
| 在 step summary 把 `G-3 NOT-RUN` 改写为 PASS/通过 | 输出自检与 AC-69 语义 |
| 为消红而降级为 `::notice` 或删除注解（含首跑 `EVIDENCE_UNBOUND` 的注解） | NOT-RUN/未绑定必须可见 |

### 5.2 允许的唯一形态

```text
捕获退出码（不改写） → 白名单 {0,4} → 0 → success；4 → success+记录+工件+注解；其余 → exit 1
```

`4` 与 `1/2/3/124` 的分支必须**显式列出**，不得用“非 0 一律失败”或“非 0 一律放过”的模糊逻辑。

### 5.3 IV 与 Reviewer 的职责

- **IV（每项复核）**：① CI 未触发/被停用/路径被删 ⇒ 仍发 `GATE_NOT_RUN`（§8.2 `:765-769`；AC-73/AC-107）；② exit-4 的绿色 job **不构成 IV 证据**；GV-S21 必须记录 run id/URL、`base/head`、`gate_exit`、`RELEASE_ELIGIBLE`、`g4_status`、`GATE_NOT_RUN?`；③ **若存在 Release/Publish workflow**：同 workflow 必须含 `needs.collab-gates.outputs.gate_exit == '0'`；独立 workflow 必须含 `workflow_run` + artifact 读取 `gate_exit == '0'`；只凭 `workflow_run.conclusion` 或 prose 一律视为无效（m3 + QC-02 m1）；④ 确认 `go.yml` 无 gate job、`collab-gates.yml` 覆盖两类路径（S-1/S-2）。
- **Reviewer**：引用 exit-4 运行时必须写 `review_scope` 与 `RELEASE_ELIGIBLE=false`，并明确“非全 PASS”；不得据此出 Review PASS（§6.3 `:459`）。首次运行 `EVIDENCE_UNBOUND`（A1-07）不得被写成实现 FAIL，也不得用 `continue-on-error` 消除。
- **放宽唯一途径**：未来若要接受 1/2/3/124，或把 4 记为 0，**必须**新 Amendment + QC + Director 书面批准；不得以改 workflow、加 `if`、改 Makefile 回显实现。

---

## 6. Scope / 边界

### 6.1 不改变 / 定点修订

| 项 | 状态 |
|---|---|
| SPEC-0003 v1.4 的 AC-69/70/72/100/107/112 文本与语义 | 不变；本文件不解释为放宽 |
| 执行器退出码 `0/1/2/3/4/124` 的定义与产生条件 | 不变（`scripts/collab_gates.py:70-75`） |
| 四闸门 G-1…G-4、`GATE_INCOMPLETE`、无 G-5/G-6 | 不变（§6.2 `:450`） |
| §6.2 输出契约（`RELEASE_ELIGIBLE`/`RELEASE_BLOCK_REASON`/`SUMMARY`） | 不变；workflow 只读不改 |
| §6.3 Review/Release 门槛（Review 要求四闸门 PASS；Release 要求 44/44 + release-scope） | 不变 |
| §8.1 的触发模型、CI 命令、三 artifact、其余 G-1 静态核对 | 不变；仅按 §4.1 S-1…S-3 定点修订 job 归属、路径覆盖与静态核对 |
| §8.2 `GATE_NOT_RUN` 外部检测、§6.10 IV、GV-S21、GV-S20 | 不变 |
| §15-6 merge/landing 政策 | **完全不变**（§6.2） |
| AC-01…AC-68 / INV-01…INV-50、业务代码与迁移 | 零变更 |

### 6.2 merge 政策不变：ADR-0008 §15-6 保持原样（QC-01 m2）

- 本 Amendment **不**收窄、不解释、不取代 ADR-0008 §15-6（`:1057`）：“中间提交不 merge 到 main；若组织要求中间 merge，需要 Spec 升版授权”继续有效。
- **CI 绿 ≠ merge 批准**：exit-4 的 job success 只表示“本次运行不构成 CI 失败”，不产生 merge 授权，也不改变任何 landing 时点/审批链；不得引用本文件作为合并依据。
- Director 2026-11-25 裁决（§1.3）只覆盖 CI job 记账；merge/landing 时点若需调整，由 Director 在同一 human decisions 区段单独裁决，并按 §15-6 的授权链执行。
- 本 Amendment 对 §15-6 的影响 = **无**。Rev 1.0 的收窄表述已删除（§10 m2）。

### 6.3 不做（非目标）

- 不改 `go.yml` 的 `verify` job / MySQL service / 代码路径触发。
- 不改 gate 结论、不新增门禁类型、不新增台账/列、不改 `ac-coverage.tsv`。
- 不引入 `continue-on-error`/warn 的“受控变体”，不引入自动重试掩盖 4。
- 不写业务代码、不生成第二台账、不代写 Reviewer/人类结论。

---

## 7. Baseline 影响

| 维度 | 本 Amendment 批准并接线后的状态 |
|---|---|
| 治理 CI（`collab-gates.yml`） | exit 4 时 **job success**（NOT-RUN 注解 + `RELEASE_ELIGIBLE=false` + 三工件 + step summary）；exit 0 时 success；1/2/3/124/未知 failure |
| 代码类 PR | 不再有 `go.yml` 的第二 gate job；由 `collab-gates.yml`（S-2 覆盖代码路径）以同一 exit-4 映射门禁，`verify` 照旧 |
| 代码 PR 的流程前置（QC-02 m4） | S-2 使代码 PR 进入 review-scope 门禁，因此**同样**需要 `change_id`、按 §6.3/§6.4 的 change-package base/head 一致与显式 range、以及 GV-S20 `mode=gate` 行；缺失时分别注解 `CHANGE_ID_UNBOUND` / `GOV-GATES-CHANGE-PACKAGE` / `EVIDENCE_UNBOUND` 并 failure，不得按“轻量检查”放过 |
| merge / landing 政策 | **不变**（§15-6、§6.2）；CI 绿不构成 merge 授权 |
| Release/tag | **阻断**：`exit=4` 不得 Release；必须 release-scope `exit=0` + `gate_exit == '0'` 机器条件 + 人类 GV-S6 |
| Review 结论 | 仍要求 exit 0（四闸门 PASS）；exit 4 不得出 Review PASS |
| AC-VERIFIED | 不变：0/44（治理）直到 Reviewer 逐条签名；本文件不标任何 AC VERIFIED |
| 业务基线 AC-01…68 | 零语义变更、零回归要求新增（本文件不触碰业务路径） |
| 生产运行时代码/迁移 | 无变更 |
| CI 成本 | 每次代码或治理路径提交 1 个轻量治理 job（无 MySQL/Go）；实际分钟数未计量，按 §8.4 不给阈值 |
| artifact vs canonical | artifact（30 天）仅可见性/排障；**canonical = 提交进仓库的 gate record + GV-S20 行**，仓库记录优先（m5） |

**误读防护**：main 绿 ≠ 治理完成 ≠ merge 批准；`RELEASE_ELIGIBLE=false` 期间任何 Release/tag 都属违规。若业务方在 P1/P2 期间需要 Release，唯一路径是把 44 条治理 AC 补齐为 VERIFIED 并取得 release-scope exit 0，而不是依赖 CI 绿。

---

## 8. 【假设与不确定性】

1. **GitHub Actions 行为未本地实跑**：step 级 `$?`、`if: always()`、job outputs、`::warning` 注解、`upload-artifact@v4` 的 `if-no-files-found: error` 均未在真实 runner 验证（[不确定]）。T-B7 接线后必须有一次真实运行证据并写入 GV-S21。
2. **`make` 折叠退出码（已实测，非假设）**：`make` 对 recipe 失败返回 2（`make: *** [gates-strict] Error 4`，`$?=2`），因此**不能**用 make 状态分支；本文件假设 Makefile 能回显执行器真实退出码（R-1）。若目标名或入口改变，workflow 必须 fail-closed，不得退回用 make 状态。
3. **工件依赖**：`RECORD_OUT`/`AUDIT_ROW_OUT` 由 P0b 按 §6.1/§6.12 实现；当前工作树执行器的参数表（`scripts/collab_gates.py:1128-1145`）**尚未含**这两项（[不确定：P0b 提交时点]，QC-01 §5 独立复核）。未落地前 T-B7 **不得**接线；已接线但工件缺失时 exit-4 job 按 A1-03 失败，不会静默成功。
4. **Artifact retention**：GitHub 默认保留期由仓库设置决定（1–90 天），`retention-days: 30` 是建议值；它只影响可见性，不影响 canonical 记录（m5）；具体天数由 Director 定（Q-03）。
5. **Runner 成本**：一次治理 job 的分钟数取决于仓库规模与 `fetch-depth: 0`；本文件不承诺具体时长，沿用 ADR-0008 §8.4 的“不设阈值、实测记录”口径。
6. **分支保护迁移**：`go.yml` 的 `collab-gates` job 删除后，若它曾被设为 required status check，需由 repo admin 把 required check 切到 `collab-gates.yml` 的 job，否则可能留下悬空 check（[不确定]，属 repo admin / Director，Q-04）。
7. **注解限制**：GitHub 单 step 的注解数量/长度有上限（[不确定]）；step summary 与 artifact 是可见性载体，canonical 仍是仓库记录。
8. **Fork PR**：来自 fork 的 `pull_request` 使用只读 token，artifact 上传与注解是否可用（[不确定]）；若 fork 场景受限，按“未运行”由 IV 记 `GATE_NOT_RUN`。
9. **44 条 PENDING 是预期状态**：P1/P2 逐条验证；本文件不把它们标 VERIFIED，也不允许批量 `DEFERRED`（INV-69/AC-99）。
10. **两条首跑红路径（A1-07）均依赖 P0b 落地**：① §6.12 的 GV-S20 审计行检查与 `## Governance audit log` 区段（N-1）；② §6.8 的 GV-S21/IV 校验。当前工作树执行器尚未实现这两项检查（QC-01/QC-02 实测 review 仍 exit 4）；两条路径的“首跑 1 → 提交载体（审计行/IV 记录）→ 重跑 4/0”需在 T-B8/真实运行中确认。
11. **代码路径 PR 的 `<change_id>` 解析**：沿用 §8.1 `:759` 的 PR 标签/branch 映射；缺失 ⇒ `CHANGE_ID_UNBOUND`，由 **R-0** 在 gate step 前 `exit 1`（不写 gate record、不进入 R-2），不得以 exit-4 成功掩盖。这是 S-2 的显式后果（QC-02 m3/m4），不属放宽范围。

---

## 9. Open questions 与批准

### 9.1 Open questions

| ID | 问题 | 状态 | 默认处置 |
|---|---|---|---|
| Q-01 | 退出码回显通道用 stdout `GATE_EXIT_CODE <n>` 还是 `$GATE_EXIT_OUT` 文件 | **OPEN**（设计选择） | 以 stdout 行为主、文件为可选等价；两者不一致即 fail-closed |
| Q-02 | Release/tag 的机器落点：T-B7 是否同期新增 release workflow，或继续由人类 GV-S6 放行 | **OPEN**（Director） | 两种机制均已定义：① 同 workflow 用 `needs.collab-gates.outputs.gate_exit == '0'`；② 独立 workflow 用 `workflow_run` + artifact 读取；未定前保持人类 GV-S6 + IV |
| Q-03 | artifact retention 30 天还是 90 天 | **OPEN**（Director / Lead） | 暂用 30；仅影响可见性，canonical 记录不受影响（m5） |
| Q-04 | 移除 `go.yml` 的 gate job 后，required status check 如何迁移 | **OPEN**（repo admin / Director） | 默认把 required check 切到 `collab-gates.yml` 的 job；缺省不阻断本 Amendment |

> 说明：Q-01…Q-04 均**不阻塞**本 Amendment 的批准（已批准）；未决前按“默认处置”执行，且默认一律 fail-closed。

### 9.2 Approval（批准块）

| 项 | 值 |
|---|---|
| 批准对象 | 本文件 **ADR-0008 Amendment 1 Rev 1.1** |
| 决定 | **`Status: approved`** |
| approved_by / approved_at | **Director** / **2026-11-25** |
| QC | **QC-02：READY（0 BLOCKER / 0 MAJOR / 5 MINOR）**；5 项 MINOR 与批准同轮编辑性并入（§11）；QC-01 的 1 MAJOR + 5 MINOR = 6/6 CLOSED（§10） |
| 生效条件 | **已满足**：QC-02 READY + Director 书面批准（2026-11-25）；T-B7 可接线 |
| 批准后义务 | T-B7 实现 S-1…S-3 + 退出码通道（R-1）+ 两条首跑红路径注解（A1-07）+ 机器可读门槛（§3.2）；不得以本批准宣称实现 PASS/AC-VERIFIED/Review/Release PASS 或 merge 授权 |
| 绑定 | **SPEC-0003 v1.4**（approved 2026-11-24）+ **ADR-0008 Rev 1.1**（approved 2026-11-24）；本文件另定点修订其 §8.1（§4.1 S-1…S-3） |
| 变更边界 | 本次仅修改本文件；ADR-0008/SPEC-0003/`scripts/`/`.github/`/`Makefile`/台账/QC 报告均未改动 |

---

## 10. Rev 1.1 变更记录（QC-01 整改）

> 被审 Rev 1.0 = 307 行，sha1 `c69bb2b4e80fe436cf3f6e45623c509cb8109731`；本 Rev 1.1 只改本文件，不改 ADR-0008/SPEC-0003/脚本/CI/Makefile/台账/QC 报告。QC-01 结论 **BLOCKED（1 MAJOR / 5 MINOR）** 逐项处置如下。

| QC-01 ID | 严重级 | 问题摘要 | 处置 | 落点 |
|---|---|---|---|---|
| AMD1-M1 | MAJOR | exit-4 映射只覆盖新 `collab-gates.yml`，`go.yml` 既有 gate job 仍会红 | **选 A**：T-B7 删除 `go.yml:26-33` 的 `collab-gates` job；`collab-gates.yml` 覆盖代码+治理路径；G-1 静态核对同步（S-1…S-3）；不采用 B | §1.1、§1.2、§2.1、A1-08、§4.1、§5.1、§6.1、§9 Q-04 |
| AMD1-m1 | MINOR | 首跑可能 exit 1（GV-S20 行未提交）而非 4 | 补 A1-07 预期序列与注解；job 仍 failure；禁止 `continue-on-error`；依赖 P0b 审计行检查 | §2.1、A1-07、R-2、§5.3、§8-10 |
| AMD1-m2 | MINOR | 错误收窄 §15-6（merge 政策） | **删除收窄**；明确“CI 绿 ≠ merge 批准”、§15-6 完全不变、Director 裁决仅覆盖 CI 记账 | §1.2、§1.3、§6.2、§7 |
| AMD1-m3 | MINOR | `gate_exit==0` 仅散文、无可机读载体 | job/step outputs `gate_exit`/`gate_status`/`release_eligible`；Release 条件 `needs…outputs.gate_exit == '0'`；IV 清单加入 release workflow 检查 | §3.2、R-4、§5.3③ |
| AMD1-m4 | MINOR | CI 工件命名与 canonical `<base7>-<head7>-<scope>.tsv` 不一致 | `RECORD_OUT`/`AUDIT_ROW_OUT` 直接用 canonical 文件名；提交免改名、字节不变 | R-2、R-4 |
| AMD1-m5 | MINOR | artifact 30 天 vs canonical 仓库记录未分层 | 明确 artifact 仅可见性/排障，canonical = 提交进仓库的 gate record + GV-S20 行，仓库记录优先 | R-4、§7、§8-4、Q-03 |

**关闭统计**：M1 + m1…m5 **6/6 已处置**；无 deferred。QC-02 复审 READY（0/0/5，§11），**Director 2026-11-25 已批准**；T-B7 可接线（仍受 AC-100/AC-107 与 §11 澄清约束）。


---

## 11. QC-02 编辑性澄清（m1…m5；与批准同轮）

> QC-02（`docs/collab/qc-reports/ADR-0008-AMD1-QC-02.md`）对 Rev 1.1：**READY（0 BLOCKER / 0 MAJOR / 5 MINOR）**，QC-01 的 1 MAJOR + 5 MINOR = 6/6 CLOSED。5 项 MINOR 仅涉记录/契约精度，**不改变**任何设计决策、退出码、AC/INV 或 Release 门槛；按 Director 指令与 2026-11-25 批准同轮作为编辑性澄清并入。

| QC-02 ID | 问题摘要 | 澄清（编辑性） | 落点 |
|---|---|---|---|
| Q2-m1 | `needs` 只在同一 workflow 内可见，跨 workflow 失效 | Release 二选一：① 与 `collab-gates` 同 workflow，用 `needs.collab-gates.outputs.gate_exit == '0'`；② 独立 workflow 用 `workflow_run` + 下载 artifact 读取 `gate_exit`；禁止只凭 `workflow_run.conclusion` | §3.2、R-4、§5.3③、Q-02 |
| Q2-m2 | 首跑红只声明 GV-S20，漏了 GV-S21/IV | A1-07 增列两条路径：GV-S20 缺失（`EVIDENCE_UNBOUND`）与 GV-S21 缺失（`GATE_NOT_RUN`，含 P0b/T-B7 自身）；failure/注解/提交载体/重跑序列一致 | A1-07、§2.1、§5.3、§8-10 |
| Q2-m3 | `CHANGE_ID_UNBOUND` 只在散文；R-2 按退出码分支 | 新增 R-0：range/change 解析 step 在 gate 前失败关闭（`exit 1`、不写 record）；R-2 `*` 仅兜底 | R-0、§8-11 |
| Q2-m4 | 代码 PR 随 S-2 承担 review 前置，未声明 | §4.1/§7 明确代码 PR 同样需要 `change_id`、change-package/range 一致、GV-S20 行；缺失注解并 failure | §4.1、§7、§8-11 |
| Q2-m5 | `gate_exit` 空值判据与 R-2 的 `missing` 不一致 | 统一为 `gate_exit != '0'`（含空值、`missing`、job 失败）⇒ Release 不运行 | §3.2、R-4 |

**关闭统计**：QC-02 **5/5 已编辑性并入**；未改 ADR-0008/SPEC-0003/脚本/CI/Makefile/台账/QC 报告；本文件 `Status: approved`（Director 2026-11-25）。
