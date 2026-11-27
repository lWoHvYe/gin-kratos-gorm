# REV-SDD-STRUCT-ALIGN-01-SPEC — Spec Kit 双布局试点 Spec 阶段审查

| 项 | 值 |
|---|---|
| change_id | **`SDD-STRUCT-ALIGN-01`** |
| 级别 | **契约变更（工具链）**（宪法 3 兜底；非业务行为） |
| 阶段 | **Spec** |
| review_scope | `specs/sdd-struct-align-01/spec.md`、`docs/adr/0009-spec-kit-dual-layout.md`、`specs/sdd-struct-align-01/{plan.md,tasks.md}`；附录 A 八项、AC-1…6、ADR/边界/命令实测。**不审**实现（`specs/README.md`、`Spec.md` 指针是批准后实现）、`docs/specs/` 迁移、业务 AC。 |
| base | **`6a66ea5`**（`git rev-parse HEAD` = `6a66ea5a5dee499f23f94b935310a923874be9ef`，C-2 preset 提交） |
| head | **工作树 @ `6a66ea5`（未提交；新文件全部未跟踪）**：`spec.md` sha256 `eb29fe1a1b518743a47e661eb97d4a2181295255ef011edea6027960a18d1520`（reviewer 已更新验收表）；`plan.md` `c9f0797e20addc06c7196a7ac18697687df28dd3bd0d1236b16bb899e4aa6258`；`tasks.md` `aef4a686f61958975af4209bcf8388bc85ae33f08bf48d3add48fab44c0a9155`；ADR `a489df1c24c9cf42d5232b6807447f240ec195f62d8d79ece7ce34bbc59f4d8a`；`git diff 6a66ea5`（tracked）= 0。 |
| final head | **2026-11-27 B1 修正后（最终确认输入）**：`spec.md` sha256 `b2303b5cdc295d7e27461adf9b1261ce2c469784072891748db639a4b66890d3`（reviewer 已把 AC-1 标已验证）；`plan.md` `239d054b5ac4e593f9dd827835e3b139372935e59d9f1cc969e1fca224c99322`；`tasks.md` `aef4a686f61958975af4209bcf8388bc85ae33f08bf48d3add48fab44c0a9155`；ADR `87bc3688b35f90ddc0e0992ba15f84fbbd893092f6d842681490c112ceed7e09`。**final：PASS（Spec 阶段）**；AC-4/5 未验证（实现阶段）。 |
| impl final | **2026-11-27 实现阶段（AC-4/AC-5）**：`spec.md` sha256 `d8384b192fdd1462ebfe4de2480366bcde1b32d9636615b504fbb70a6c274bb7`；`specs/README.md` `4c9a7f06e46ce483b7a58045ef75acd7ab400019a15351f4bff4b1ea3dd36b57`（28 行）；`Spec.md` `5866ee3ee29ddfd56c9f88d79311aac8ba41e2ad816fee2f59d3dc82b5731a40`（1 added/0 deleted）；ADR `87bc3688b35f90ddc0e0992ba15f84fbbd893092f6d842681490c112ceed7e09`。**实现 PASS；可交人类合入；NOT RELEASE。** |
| 契约输入 | Spec/ADR/plan/tasks；宪法 v2.2.0（附录 A/23/30）；AGENTS（§1/§3/§4）；`.specify/scripts/bash/{common.sh,check-prerequisites.sh,resolve-template.sh}`；`.specify/presets/gin-kratos-spec/**`；`Spec.md`/`docs/specs/README.md`（只读对照）。`docs/convert/SDD-STRUCT-ALIGN-01-PLAN.md` 是作者侧线索，**未作证据**。 |
| INV | 不涉及领域不变量（无业务行为/状态/资金/库存/权限/迁移）。工具链约束：Spec Kit 1.4.3 路径解析、`docs/specs/` 原地不动、核心模板/脚本不改、一个 Spec 一个验收表。 |
| verification | **静态 + 命令实测**：AC-1 `--json --require-spec` exit 0 但 JSON 无 `FEATURE_SPEC`（**B1**）；AC-2 `--require-tasks --include-tasks` exit 0 且 `AVAILABLE_DOCS:["tasks.md"]`；AC-3 缺 plan/tasks/spec/目录均 exit 1；AC-6 resolver 输出含 `## 附录 A 对照`、`specify preset list` enabled/Templates 1；无 env/feature.json exit 1；`git diff` 对 `docs/specs/0001/0002`、`Spec.md`、核心模板/脚本均为 0；实例名 grep 0。AC-4/5 未验证（实现阶段）。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge；本裁决是 Spec 阶段 |
| overall verdict | **FAIL**：1 阻断（**B1**：FR-001/AC-1 断言 `check-prerequisites.sh --json --require-spec` 的 `FEATURE_SPEC`，但该脚本 JSON 模式只输出 `FEATURE_DIR`/`AVAILABLE_DOCS`；`FEATURE_SPEC` 只在 `--paths-only`）。AC-2/3/6 已验证，AC-4/AC-5 未验证（实现阶段）；Spec 八项落点/ADR/边界均通过；NB-1…NB-4。**final（B1 修正后）：PASS（Spec 阶段）**——AC-1/2/3/6 已验证，AC-4/5 未验证（实现阶段）；见 §0A。**implementation（2026-11-27）：PASS**——AC-4/AC-5 已验证；可交人类合入；**NOT RELEASE**；见 §0B。 |

---

## 0B. Implementation review（代码阶段，2026-11-27）— PASS

| 项 | 值 |
|---|---|
| 裁决 | **PASS（实现阶段）**；AC-1/2/3/6 保持已验证；**AC-4/AC-5 已验证**；**可交人类合入**；**NOT RELEASE** |
| impl final | `spec.md` sha256 `d8384b192fdd1462ebfe4de2480366bcde1b32d9636615b504fbb70a6c274bb7`；`specs/README.md` `4c9a7f06e46ce483b7a58045ef75acd7ab400019a15351f4bff4b1ea3dd36b57`（28 行）；`Spec.md` `5866ee3ee29ddfd56c9f88d79311aac8ba41e2ad816fee2f59d3dc82b5731a40`；ADR `87bc3688b35f90ddc0e0992ba15f84fbbd893092f6d842681490c112ceed7e09`；base `6a66ea5` |
| AC-4 | **已验证**：`git diff 6a66ea5 -- docs/specs/0001/0002` = **0 行**；`git diff 6a66ea5 -- Spec.md` = **1 added / 0 deleted**（仅末行指针，现有索引行未改）。 |
| AC-5 | **已验证**：`specs/README.md` 28 行，覆盖双布局、`docs/specs/` 边界、一个 Spec 一个验收表、`feature.json` 不提交、迁移另立变更、模板/preset 命令；链接目标 `specs/README.md`、`docs/specs/0001/0002`、`docs/review-tracking.md`、`.specify/scripts/bash/*` 均存在。 |
| 复跑 | `SPECIFY_FEATURE_DIRECTORY=specs/sdd-struct-align-01 ... --json --require-tasks --include-tasks` → exit 0，`AVAILABLE_DOCS:["tasks.md"]`；`resolve-template.sh spec-template` 输出含 `## 附录 A 对照`。 |
| 边界 | `git diff 6a66ea5 -- .specify/templates/spec-template.md .specify/scripts` = **0 行**；`docs/specs/0001/0002` = 0；无业务 AC/代码；宏观 `SPEC-0001/0002` 仅出现在 `Spec.md` **原有**索引行（实现只加一行指针，未改索引）。 |
| 观察 | Spec header 仍写 `**Status**: Draft（待独立审查 + 人类批准）`，与“人类已批准 Spec”不一致；人类批准已在台账，作者应在合入前把 header Status 更新为 Approved（reviewer 不改 Spec 正文）。**非阻断。** |
| 结论 | **实现 PASS。可交人类合入；NOT RELEASE。** Release 由人类执行；本裁决不等于 Release。 |

---

## 0A. Final confirmation（2026-11-27，B1 修正后）— PASS

| 项 | 值 |
|---|---|
| 裁决 | **PASS（Spec 阶段）**；B1 消解，NB-1…NB-4 消解；AC-1/2/3/6 **已验证**，AC-4/AC-5 **未验证（实现阶段）** |
| final head | `spec.md` sha256 `b2303b5cdc295d7e27461adf9b1261ce2c469784072891748db639a4b66890d3`；ADR `87bc3688b35f90ddc0e0992ba15f84fbbd893092f6d842681490c112ceed7e09`；plan `239d054b5ac4e593f9dd827835e3b139372935e59d9f1cc969e1fca224c99322`；tasks `aef4a686f61958975af4209bcf8388bc85ae33f08bf48d3add48fab44c0a9155`；base `6a66ea5`；新文件未跟踪，final sha 以文件 sha256 为准 |
| AC-1 重跑 | `--json --require-spec` → `{"FEATURE_DIR":"/Users/.../specs/sdd-struct-align-01","AVAILABLE_DOCS":[]}`，exit 0；同目录 `--json --paths-only` → `FEATURE_SPEC":"/Users/.../specs/sdd-struct-align-01/spec.md"`，exit 0。**FR-001/AC-1 的两条断言均满足。** |
| AC-2/3/6 | 复跑通过：AC-2 `AVAILABLE_DOCS:["tasks.md"]`；AC-3 缺 plan/tasks/spec/目录均 exit 1；AC-6 resolver 输出含 `## 附录 A 对照`、preset enabled。 |
| NB-1 | 行号已核：`check-prerequisites.sh:20`（JSON 输出）、`:110-125`（`--paths-only` 的 `FEATURE_SPEC`）、`:139-169`（fail-closed）；`create-new-feature.sh:280`（`SPECS_DIR=specs`）、`:424-425`（`FEATURE_DIR/SPEC_FILE`）。**消解。** |
| NB-2 | Spec 验收表前已加“AC-1…AC-6 与 FR-001…FR-006 一一对应”；reviewer 同步把表内 AC-1 标为已验证、AC-4/5 保持未验证。**消解。** |
| NB-3/NB-4 | ADR §核对 已引用 `REV-SDD-STRUCT-ALIGN-01-SPEC.md`（AC-2/3/6 已验证、AC-1 待重审）；背景已改为“`--require-spec` 要求 spec+plan，`--require-tasks` 另要求 tasks”。**消解。** |
| 边界 | `docs/specs/0001/0002`、`Spec.md`、核心模板/脚本 diff 0；实例名 0；无业务 AC。 |
| 结论 | **PASS（Spec 阶段）**。**Spec 可交人类批准**；批准后实现 AC-4（`Spec.md` 一行指针）与 AC-5（`specs/README.md`），代码阶段复审。**AC-4/AC-5 不得在 Spec 阶段标已验证。** |

---

## 0. 第一次审查结论（FAIL，保留原文）

1. **FAIL。** 唯一阻断是 **B1**：Spec 的 FR-001/AC-1 要求 `--json --require-spec` 输出 `FEATURE_SPEC`，但 `check-prerequisites.sh` 的 JSON 模式只输出 `{"FEATURE_DIR","AVAILABLE_DOCS"}`；`FEATURE_SPEC` 只在 `--paths-only --json`/text 模式出现。Spec 的 Assumptions 又写“不修改 `.specify/scripts/**`”，因此 AC-1 按当前文本不可达。**Spec 不能交人类批准，直到 FR-001/AC-1 与脚本实际输出对齐。**
2. **其余通过**：Spec 的附录 A 八项都有落点；ADR-0009 命中附录 B、Status Proposed、背景/决策/备选/后果/取代齐全、未改写已批准 ADR；无业务 AC/实例名；`docs/specs/0001/0002`、`Spec.md`、核心模板/脚本均未改。
3. **AC 状态**（我拥有验收表）：AC-2/AC-3/AC-6 **已验证**（命令实测）；AC-1 **未验证**（B1）；AC-4/AC-5 **未验证**（依赖实现：`Spec.md` 指针、`specs/README.md`）。没有为了全绿提前标 AC-4/5。
4. **人类批准后**：修正 FR-001/AC-1 → 复审 Spec → 人类批准 → 实现 AC-4/AC-5（`Spec.md` 一行指针 + `specs/README.md`）→ 代码阶段复审。AC-4/5 不得在 Spec 阶段标已验证。

---

## 1. 审查对象与方法

- 逐行读 Spec/ADR/plan/tasks；`git status` 确认新文件全部未跟踪；`git diff 6a66ea5`（tracked）为 0，说明没有改动已跟踪的宪法/AGENTS/脚本/模板/旧 Spec。
- 实跑：AC-1/AC-2/AC-3/AC-6；无 env + 无 `feature.json`；`--paths-only --json`；临时目录 `/tmp/sdd-review/{noplan,notasks,nospec,nodir}`；`git diff` 边界检查；`grep` 实例名。
- 测试副作用：`check-prerequisites.sh` 在设置 `SPECIFY_FEATURE_DIRECTORY` 时会写 `.specify/feature.json`（机器本地、被 `.specify/.gitignore` 忽略）；reviewer 测试后已删除该文件，未提交。
- `docs/convert/SDD-STRUCT-ALIGN-01-PLAN.md` 只作定位线索（例如作者 `/tmp` 侦察结论），**未据其判定**；判定只用 Spec/ADR/宪法/脚本实测。

---

## 2. 独立实测

### 2.1 AC-1（FR-001）

```text
$ SPECIFY_FEATURE_DIRECTORY=specs/sdd-struct-align-01 \
    ./.specify/scripts/bash/check-prerequisites.sh --json --require-spec
{"FEATURE_DIR":"/Users/.../specs/sdd-struct-align-01","AVAILABLE_DOCS":[]}
exit=0
```

- exit 0，`FEATURE_DIR` 正确；**JSON 中没有 `FEATURE_SPEC`**。
- `check-prerequisites.sh` 头注释（`:20`）明确 JSON 模式是 `{"FEATURE_DIR","AVAILABLE_DOCS"}`；`FEATURE_SPEC` 只在 `--paths-only`（`:118-131`）出现。
- `--paths-only --json` 实测输出含 `FEATURE_SPEC":"/Users/.../specs/sdd-struct-align-01/spec.md"`，但 `--paths-only` 不做 `--require-spec` 校验（`:124-135`）。
- Spec 的 Assumptions（`:71`）写“不修改 `.specify/scripts/**`”，因此不能靠改脚本补 `FEATURE_SPEC`。**B1 成立。**

### 2.2 AC-2（FR-002）

```text
$ SPECIFY_FEATURE_DIRECTORY=specs/sdd-struct-align-01 \
    ./.specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks
{"FEATURE_DIR":"/Users/.../specs/sdd-struct-align-01","AVAILABLE_DOCS":["tasks.md"]}
exit=0
```

**AC-2 已验证。** `AVAILABLE_DOCS` 含 `tasks.md`。

### 2.3 AC-3（FR-003）

| 用例 | 结果 |
|---|---|
| `/tmp/sdd-review/noplan`（只有 spec.md） | `--require-spec` **exit 1**：“plan.md not found” |
| `/tmp/sdd-review/notasks`（spec.md+plan.md） | `--require-tasks` **exit 1**：“tasks.md not found” |
| `/tmp/sdd-review/nospec`（只有 plan.md） | `--require-spec` **exit 1**：“spec.md not found” |
| `/tmp/sdd-review/nodir`（不存在） | **exit 1**：“Feature directory not found” |
| 无 env + 无 `.specify/feature.json` | **exit 1**：“Feature directory not found. Set SPECIFY_FEATURE_DIRECTORY…” |

**AC-3 已验证。** 缺 plan/tasks/spec/目录均 fail-closed，没有回退到 `docs/specs/`。

### 2.4 AC-6（FR-006）

```text
$ ./.specify/scripts/bash/resolve-template.sh spec-template | grep -F '## 附录 A 对照'
## 附录 A 对照（宪法必备内容）
$ specify preset list
  Gin Kratos Spec Standards (gin-kratos-spec) v0.1.0 — enabled — priority 10
    Templates: 1
```

**AC-6 已验证。** resolver 输出含附录 A；preset enabled/Templates 1（C-2 已在 `6a66ea5` 提交）。

### 2.5 AC-4 / AC-5（实现阶段，不标已验证）

- AC-4：`git diff -- docs/specs/0001-fulfillment-quantity-split.md docs/specs/0002-payment-settlement-admission.md` = **0 行**；`git diff -- Spec.md` = **0 行**。`Spec.md` 的一行指针是批准后实现，**AC-4 未验证**。
- AC-5：`specs/README.md` **不存在**；按用户要求不写它。**AC-5 未验证**，实现阶段验证。

### 2.6 边界

- `git diff 6a66ea5 -- .specify/templates/spec-template.md .specify/scripts` = **0 行**；核心模板/脚本未改。
- `docs/specs/0001/0002` 未改；`Spec.md` 未改。
- 实例名 `PAY-SETTLE-01` / `PAY-AC50-REVIEW-01` / `SPEC-0002` / `SPEC-0001` 在宪法/AGENTS/Spec/ADR **0 命中**（`grep` exit 1）。
- 无业务行为/AC：Spec 全部 AC 都是工具链路径/命令/文档边界；无资金/库存/权限/状态机。

---

## 3. Spec 完整性（附录 A 八项）

| 附录 A 项 | Spec 落点 | 判断 |
|---|---|---|
| 业务规则与验收标准 | 附录 A 对照行 1；AC-1…AC-6 + FR-001…006；金额/状态/并发不适用 | **有落点**；但 AC 表原本无 AC 文本，reviewer 已在验收表补 AC↔FR 映射（NB-2）；AC-1 断言错误（B1） |
| 状态所有权 | 附录 A 对照行 2；“无业务状态；Spec/AC 状态只在验收表，运行状态在台账” | **有落点** |
| 契约与异常恢复 | 行 3；env/feature.json 解析；缺 spec/plan/tasks fail-closed；不迁移旧路径；ADR-0009 | **有落点** |
| 可达性检查 | 行 4；`common.sh:163-230`；`check-prerequisites.sh` 主流程；`create-new-feature.sh` 默认 `specs/<NNN-name>/` | **部分**：`check-prerequisites.sh`/`create-new-feature.sh` 只给脚本名，未给行号（NB-1） |
| 假设与不确定性 | 行 5 + §Assumptions（Spec Kit 1.4.3、PyYAML 6.0.4、feature.json 本地、不改脚本/模板、无业务 AC） | **有落点** |
| 业务裁决题 | 行 6：D-1 布局 A、D-2 Spec.md 指针 A（人类 2026-11-27） | **有落点** |
| 验收状态表 | §验收状态表；reviewer 已更新 AC-1…6 状态/证据/审查者 + AC↔FR 映射 | **有落点** |
| 延期清单 | 无 | **有落点** |

- **AC 可判定性**：AC-2/3/6 可判定且实测通过；AC-1 因 `FEATURE_SPEC` 断言与脚本输出不符而不可判定为通过；AC-4/5 在 Spec 阶段可判定但依赖实现，保持未验证。
- **没有把工具链 AC 写成业务规则**：Spec 明确“工具链行为，无业务规则”；FR-001…006 都是命令/文档/路径约束。

---

## 4. ADR-0009

| 检查 | 结论 |
|---|---|
| 是否命中附录 B | **是**：路径布局/技术选型，需 ADR；0009 存在，Status **Proposed** |
| 背景/决策/备选/后果/取代 | **齐全**：背景（现有扁平 Spec 与 Spec Kit 路径）、决策（双布局，不迁移）、备选（全量迁移/只用模板，均否决）、后果（双布局边界、feature.json 本地、不新增业务 AC）、取代“无” |
| 是否改写已批准 ADR | **否**：新建 0009；关联 0007 标注“无冲突”，未改写 |
| 核对 | ADR §核对 说“本仓 `specs/sdd-struct-align-01/` 待验证”；实测 AC-2/3/6 通过、AC-1 失败（NB-3：修正 AC-1 后更新/引用本报告） |
| 背景措辞 | “要求目录内有 spec.md、plan.md、tasks.md”是简化；`--require-spec` 只要求 spec+plan，tasks 由 `--require-tasks` 要求（NB-4，非阻断） |

---

## 5. 边界与实例名

- **无业务行为/AC**：Spec 只定义工具链路径、命令、文档边界；没有业务规则/状态机/资金/库存/权限。
- **未迁移/改写 `docs/specs/0001/0002`**：`git diff` 0 行。
- **未改 `Spec.md`**：`git diff` 0 行（指针是批准后实现，AC-4）。
- **未改核心模板/脚本**：`git diff 6a66ea5 -- .specify/templates/spec-template.md .specify/scripts` 0 行。
- **无实例名/运行状态**：宪法/AGENTS/Spec/ADR 对 `PAY-SETTLE-01`/`PAY-AC50-REVIEW-01`/`SPEC-0002`/`SPEC-0001` 0 命中；Spec 只写“人类 2026-11-27 选择方案 A”这类决策记录，不是运行状态。
- **`docs/convert/SDD-STRUCT-ALIGN-01-PLAN.md`** 是作者侧线索，不是本次契约；未据其判定。

---

## 6. 验收表更新（reviewer 落盘）

> final confirmation：AC-1 已验证；AC-4/AC-5 实现阶段已验证；见 §0A/§0B。本节是第一次审查时的表状态。

已更新 `specs/sdd-struct-align-01/spec.md` 的 §验收状态表（**只改验收表，未改 Spec 正文**）：

- **AC-1** 未验证：`--json --require-spec` exit 0，但 JSON 无 `FEATURE_SPEC`（B1）；`FEATURE_SPEC` 只在 `--paths-only`。审查者 `reviewer-syntax（未验收）`。
- **AC-2** 已验证：`--json --require-tasks --include-tasks` exit 0，`AVAILABLE_DOCS:["tasks.md"]`；`reviewer-syntax`。
- **AC-3** 已验证：缺 plan/tasks/spec/目录均 exit 1；`reviewer-syntax`。
- **AC-4** 未验证：`git diff docs/specs` 为空，但 `Spec.md` 指针待实现；`reviewer-syntax（未验收）`。
- **AC-5** 未验证：`specs/README.md` 未创建（实现阶段）；`reviewer-syntax（未验收）`。
- **AC-6** 已验证：resolver 输出含附录 A；preset enabled/Templates 1；`reviewer-syntax`。
- 表下加 AC↔FR 映射：AC-1=FR-001…AC-6=FR-006。

---

## 7. 问题

### B1（阻断）

> final confirmation：B1 已消解（FR-001/AC-1 改为 `FEATURE_DIR` + `--paths-only` 的 `FEATURE_SPEC`）；见 §0A。

**FR-001/AC-1 断言 `check-prerequisites.sh --json --require-spec` 输出 `FEATURE_SPEC`，但该脚本 JSON 模式不输出它。**

- **违反**：宪法附录 A 行 1（每条 AC 必须可判定）与行 4（可达性）；Spec 自身 Assumptions 写“不修改 `.specify/scripts/**`”，所以不能靠改脚本补字段。
- **事实**：`check-prerequisites.sh:20` 明确 JSON 模式 `{"FEATURE_DIR","AVAILABLE_DOCS"}`；`FEATURE_SPEC` 只在 `--paths-only`（`:118-131`）。实测 `--json --require-spec` 输出无 `FEATURE_SPEC`。
- **影响**：AC-1 按当前文本不可达；Spec 不能交人类批准。
- **需要的语义**（不是新增业务规则）：让 FR-001/AC-1 与脚本实际输出一致——例如断言 `FEATURE_DIR` 指向该目录且 `spec.md` 存在，或用 `--paths-only --json` 取 `FEATURE_SPEC` 并另跑 `--require-spec` 校验；不要在 Spec 里要求改脚本（Assumptions 已禁止）。修正后重审 Spec 即可。

### NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | 可达性只给 `common.sh:163-230`；`check-prerequisites.sh`/`create-new-feature.sh` 只给脚本名，未给行号 | 附录 A 行 4；Spec 行 4 |
| NB-2 | AC 表原本没有 AC 文本/AC↔FR 映射；“附录 A 对照”行 1 说“AC-1…AC-6 见验收状态表”是循环引用；reviewer 已在验收表补映射，作者下一版应把映射写进正文/表 | 附录 A 行 1/7 |
| NB-3 | ADR §核对 仍写“本仓待验证”；实测 AC-2/3/6 通过、AC-1 失败；修正 AC-1 后应更新/引用本报告 | ADR-0009 §核对 |
| NB-4 | ADR 背景说“要求目录内有 spec.md、plan.md、tasks.md”是简化；`--require-spec` 只要求 spec+plan | ADR-0009 §背景；`check-prerequisites.sh` 行为 |

---

## 8. 裁决与下一步

> final confirmation：**PASS（Spec 阶段，B1 消解）**；见 §0A。

**FAIL。** 1 阻断（B1）；AC-2/3/6 已验证；AC-1 未验证；AC-4/5 未验证（实现阶段）；Spec 八项落点、ADR、边界均通过。

**Spec 不可交人类批准**，直到 FR-001/AC-1 与脚本实际输出对齐。修正后：
1. 重审 Spec（只审差异/受影响 AC），AC-1 可转“已验证”；
2. 人类批准 Spec/ADR；
3. 实现 AC-4（`Spec.md` 一行指针）与 AC-5（`specs/README.md`），代码阶段复审；AC-4/5 不得在 Spec 阶段标已验证。

---

## 9. 复现命令（只读）

```bash
git rev-parse HEAD                                    # 6a66ea5...
git status --porcelain                                # ?? docs/adr/0009..., ?? docs/convert/..., ?? specs/
SPECIFY_FEATURE_DIRECTORY=specs/sdd-struct-align-01 ./.specify/scripts/bash/check-prerequisites.sh --json --require-spec
SPECIFY_FEATURE_DIRECTORY=specs/sdd-struct-align-01 ./.specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks
SPECIFY_FEATURE_DIRECTORY=specs/sdd-struct-align-01 ./.specify/scripts/bash/check-prerequisites.sh --json --paths-only
./.specify/scripts/bash/resolve-template.sh spec-template | grep -F '## 附录 A 对照'
git diff -- docs/specs/0001-fulfillment-quantity-split.md docs/specs/0002-payment-settlement-admission.md
git diff -- Spec.md
git diff 6a66ea5 -- .specify/templates/spec-template.md .specify/scripts
grep -n 'PAY-SETTLE-01\|PAY-AC50-REVIEW-01\|SPEC-0002\|SPEC-0001' .specify/memory/constitution.md AGENTS.md specs/sdd-struct-align-01/spec.md docs/adr/0009-spec-kit-dual-layout.md
```

**本次落盘**：本报告（§0A Spec PASS、§0B 实现 PASS）+ `docs/review-tracking.md` 最新块顶部 Spec FAIL→PASS 与实现 PASS 条目 + `specs/sdd-struct-align-01/spec.md` 验收表（仅验收表）。未改 Spec 正文/ADR 正文/`Spec.md`/`specs/README.md`/核心模板/脚本/代码；未 commit、未 merge。
