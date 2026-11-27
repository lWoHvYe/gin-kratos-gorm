# REV-GOV-C2-TEMPLATE-01 — Spec 模板 preset append（附录 A 对照；原 override 已删除）审查

| 项 | 值 |
|---|---|
| change_id | **`GOV-C2-TEMPLATE-01`** |
| 级别 | **治理变更**（工具链/模板；不走 Spec/ADR） |
| 阶段 | **Spec** |
| review_scope | **方案更新（人类选择 B）**：override 已删除，改为 `.specify/presets/gin-kratos-spec/` 的 `append`（preset.yml + spec-template.append.md + .registry）。审：① preset schema 是否完整、`preset list` 是否 enabled/Templates 1；② resolver 输出是否 = 核心模板 + 附录段、无重复/冲突；③ 8 项与现行宪法 v2.2.0 附录 A 是否一致（含 NB-1/2/3）；④ 是否写实例名/运行状态；⑤ 是否还依赖 override；⑥ 人类 B 选择的维护风险是否由 preset 自动叠加 + “附录 A 变化时同步”覆盖。**原 override 审查保留为历史（§0–§4）**；不审代码、业务 AC、其他模板。 |
| base | **`a0ec6ba`**（`git rev-parse HEAD` = `a0ec6ba3ca03624745a3e2d377697abd19b1f9db`，上一个治理提交） |
| head | **工作树 @ `a0ec6ba`（未提交；override 未跟踪）**：`.specify/templates/overrides/spec-template.md` sha256 `a546438a75131c57d6e65229e64064eed9c0cad0b8063b4002e5a1bc409c1d8a`（146 行）；核心模板 sha256 `3945437fc35cd30a5b2bf7beea680337c3516826d3efa5a6b92c4a7eca1ba28e`（131 行）；resolver sha256 `829e227096abc8bf0889889ec9f792f503ca5d395b7836a8a7eb739ee75214e7`；宪法 `414b94a6b9419a01abc7573036de9eceda4c0f75120f9d6bd9cf9b2163b8ba4f`、`AGENTS.md` `05d948fbac7f466d187bb054143f9fff3d44d310295d62e90a06fc7bc804021b`（与 base 相同）。 |
| preset final | **2026-11-27 preset append（人类选择 B）**：`preset.yml` sha256 `74d7ceb1bea644daf62c4e97c174b0899b98c0bdf46a93ea22691224d6225828`；`spec-template.append.md` sha256 `31b56a1066085be7c557923bbc1f5642e055215e4fbf8f315d653b7e629854e9`；`.registry` sha256 `b945dd764b47180c9af7eae6800d51ec93d864d9b7e140722bf2882eb5ec780a`；核心模板 `3945437f…`、resolver `829e2270…`、宪法 `414b94a6…`、AGENTS `05d948fb…` 与 base 相同；override 目录已不存在。**preset append 不影响 PASS**。落盘后 `git diff a0ec6ba` sha256 `d8cbf50f92bb555fede382621a1c3ff4c830ffee8623c72af592837cb2718ec8`（含台账条目；preset/报告未跟踪，final sha 以 preset 文件 sha256 为准）。 |
| 契约输入 | `.specify/templates/overrides/spec-template.md`（历史）；`.specify/presets/gin-kratos-spec/{preset.yml,spec-template.append.md}`；`.specify/presets/.registry`；核心 `.specify/templates/spec-template.md`；`.specify/memory/constitution.md` 附录 A；`AGENTS.md` §6（覆盖层 replace/append）；`.specify/scripts/bash/common.sh` 的 `resolve_template_content`；`.specify/scripts/bash/resolve-template.sh`；`.dsh/skills/speckit-specify/SKILL.md`（`--json`/`TEMPLATE_CONTENT`）。 |
| INV | 不涉及领域不变量（无业务行为/AC/代码变更）。工具链/治理约束：宪法附录 A、AGENTS §6“项目覆盖层整体替换、append 走预设/扩展层”、核心模板不得直接改。 |
| verification | **静态审查 + resolver 实跑**：`diff -u` 核心 vs override（只多 2 行文件头注释 + 13 行附录 A 段（合计 +15 行），无删除）；`resolve-template.sh spec-template` exit 0（146 行，含 `User Scenarios & Testing` 与“附录 A 对照”）；`resolve-template.sh spec-template --json` exit 0，`TEMPLATE_NAME=spec-template`、`TEMPLATE_CONTENT` 5206 字符、两段均含；`python3 -c 'import yaml'` 失败（PyYAML 缺失）但 override 路径优先返回；`git diff a0ec6ba -- .specify/templates/spec-template.md .specify/memory/constitution.md AGENTS.md` 为空；`grep` 实例名/运行状态 0 命中。未运行代码测试（不适用）。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge；不批准模板生效 |
| overall verdict | **第一次（override）：PASS**（0 阻断；NB-1…NB-5）。**final（preset append）：PASS**——schema 完整、`preset list` enabled/Templates 1、resolver 输出 = 核心 + 附录段（1 处，无重复）、8 项含 NB-1/2/3 修正、无实例名、无 override 依赖；NB-4/5 消解；**preset append 不影响 PASS**。 |

---

## 0A. Preset append 更新（2026-11-27，人类选择 B）— PASS

| 项 | 值 |
|---|---|
| 裁决 | **PASS**（B：装 PyYAML + preset append；override 已删除） |
| 变更 | `.specify/presets/gin-kratos-spec/preset.yml`（schema_version / preset{id,name,version,description} / requires.speckit_version / provides.templates{type,name,file,strategy: append}）；`spec-template.append.md`（附录 A 8 项）；`.specify/presets/.registry`（enabled, priority 10）；override 目录已删除；核心模板/宪法/AGENTS 未改（`git diff a0ec6ba` 0 行） |
| 检查点 1 | preset schema 完整；`specify preset list` → **enabled / Templates: 1**；`specify --version` 1.4.3，`requires.speckit_version: ">=1.4.0"` 兼容 | **通过** |
| 检查点 2 | resolver 输出 = 核心模板 + 附录段：`resolve-template.sh spec-template` exit 0，145 行；`# Feature Specification` 1、`## User Scenarios & Testing` 1、`## 附录 A 对照` 1；`--json` → `TEMPLATE_CONTENT` 5324 字符，两段均含；无重复/冲突 | **通过** |
| 检查点 3 | 8 项与宪法附录 A 一致；NB-1/2/3 已补：EARS `WHEN/IF/WHILE...SHALL`、`MUST`、`SHALL/THEN/MUST` 形容词；AC 编号 `<change_id>-AC-<n>`/长期 `AC-<n>`/退役不复用；延期“不得删行/批量续期/新 change_id”+“豁免按 18 条” | **通过** |
| 检查点 4 | 实例名/运行状态 0 命中（`grep PAY-SETTLE-01\|PAY-AC50-REVIEW-01\|SPEC-0002\|SPEC-0001 .specify/presets/` 无结果） | **通过** |
| 检查点 5 | 不再依赖 override：`.specify/templates/overrides/` 不存在；resolver 走 preset 层（PyYAML 6.0.4 已装） | **通过** |
| 检查点 6 | 维护风险：preset `append` 自动叠加核心模板；`description` 明写“update this file when Appendix A changes”；只有附录 A 变化时更新 append 文件；**人类 B 选择的维护风险已由该方式覆盖**；**preset append 不影响 PASS** | **通过** |
| NB 状态 | NB-1/2/3 由 append 内容补入；NB-4（override 头 HTML 注释）随 override 删除消失——resolver 输出仍有 5 处 HTML 注释，是核心模板自带的 instructional 注释，非本次新增；NB-5 由 preset description 覆盖 | **消解** |
| 结论 | **PASS**；0 阻断；NB-1…NB-5 消解；无实例名/业务 AC；无 override 依赖；**preset append 不影响 PASS** | — |

---

## 0. 第一次审查（override，已删除）结论

1. **PASS。** 这是工具链/模板变更，不是业务契约变更；核心模板、宪法、AGENTS 均未改（`git diff a0ec6ba` 为空，唯一新增是未跟踪的 override）。
2. **override 是核心模板的完整副本 + 追加段**：`diff -u` 只有 `+2`（文件头注释）与 `+13`（附录 A 段；合计 +15），**没有 `-` 行**；核心的 `User Scenarios & Testing`/`Requirements`/`Success Criteria`/`Assumptions`，以及 `Edge Cases`/`Key Entities` 全部原样保留；追加段在 `Assumptions` 之后，标题唯一，无重复/冲突。
3. **8 项齐全**：业务规则与验收标准、状态所有权、契约与异常恢复、可达性、假设与不确定性、业务裁决题、验收状态表、延期清单。验收表列与附录 A 完全一致；延期清单的 owner/expiry/reason/到期阻断/延期≠豁免一致。
4. **无业务 AC/行为变更；无实例名/运行状态**（`grep 'PAY-SETTLE-01\|PAY-AC50-REVIEW-01\|SPEC-0002\|SPEC-0001'` 0 命中；override 只有“宪法附录 A”的通用引用）。
5. **维护风险已在文件头说明**：“核心模板更新时重新生成本文件”；但 override 是整体 replace，核心模板更新不会自动合并，**需人类确认接受**（审查者不代替人类接受风险）。
6. **resolver 可用**：`resolve-template.sh spec-template` 与 `--json` 均 exit 0；`--json` 输出的 `TEMPLATE_CONTENT` 就是 `speckit-specify` 所需的模板内容；override 优先级 1 在 preset 解析之前返回，**不依赖 PyYAML**（本机 `import yaml` 确实失败）。

---

## 1. 审查对象与方法

- 逐行 `diff -u .specify/templates/spec-template.md .specify/templates/overrides/spec-template.md`，确认只有追加、无删除/改写。
- 对照现行宪法 v2.2.0 附录 A（`.specify/memory/constitution.md:77-90`）逐项核对 8 个 bullet。
- `git diff a0ec6ba -- .specify/templates/spec-template.md .specify/memory/constitution.md AGENTS.md` 确认核心模板/宪法/AGENTS 未改；`git status` 确认唯一新增是未跟踪的 override。
- 实跑 resolver 两种模式；用 `python3 -c` 只解析 JSON（不依赖 PyYAML）核对 `TEMPLATE_CONTENT`。
- 未改 override/核心模板/宪法/AGENTS/代码；本报告与台账条目是 reviewer 落盘。

---

## 2. 逐条结论

### 2.1 override 是否 = 核心原文 + 追加段（无删改/重复/冲突）

**是。** `diff -u` 结果只有两处 `+`：

1. 文件头 `<!-- 项目覆盖层：Spec Kit 1.4.3 spec-template + 宪法附录 A 对照。核心模板更新时重新生成本文件。 -->`；
2. 文件尾 `## 附录 A 对照（宪法必备内容）` 的 13 行（含前置空行；heading + blockquote + 8 个 bullet）。

核心 131 行无 `-` 行；`Status: Draft`、`User Scenarios & Testing`、`Requirements`、`Functional Requirements`、`Success Criteria`、`Assumptions`、`Edge Cases`、`Key Entities` 均在。追加段标题唯一，未与核心的任何 `##` 重复；追加段在 `Assumptions` 之后，不打断核心章节顺序。**无重复/冲突。**

### 2.2 8 项与宪法附录 A 是否一致

| # | 附录 A 必备问题 | override 的 bullet | 结论 |
|---|---|---|---|
| 1 | 业务规则与验收标准：可判定、落测试/契约检查/可观测指标；EARS 或 Spec Kit 原生语法；金额/状态/并发写拒绝错误；`SHALL`/`THEN`/`MUST` 后禁止不可判定形容词 | “每条 AC 可判定，能落到测试、契约检查或可观测指标；EARS 或 Given/When/Then + FR/SC 均可；金额、状态、并发写出拒绝错误；禁止不可判定形容词。” | **一致（摘要）**；省略 EARS 的 `WHEN/IF/WHILE...SHALL` 句式、`MUST` 关键字、`SHALL/THEN/MUST` 形容词关键字 → NB-1 |
| 2 | 状态所有权 | “每个核心状态字段的唯一事实来源；不得把多个状态揉在一个字段里。” | **一致** |
| 3 | 契约与异常恢复：幂等、最终一致性、持久恢复所有者；并发/超时/崩溃/部分成功/脑裂；状态机无死胡同；协议/数据模型/状态机链接 ADR | 同左 | **一致** |
| 4 | 可达性：`文件:行号`；到达不了标 `[不确定]`；行号过期更新不算契约变更 | “给 `文件:行号`；到达不了标 `[不确定]`。” | **一致（摘要）**；省略“行号过期时更新，不算契约变更” |
| 5 | 假设与不确定性：所有底层依赖风险 | 同左 | **一致** |
| 6 | 业务裁决题：每题带编号选项；没有写“无” | 同左 | **一致** |
| 7 | 验收状态表：AC 编号、状态（未验证/有引用/已验证）、检查类型（测试/契约检查/可观测指标）、证据路径、审查者；AC 编号规则 | “AC 编号、状态（未验证/有引用/已验证）、检查类型（测试/契约检查/可观测指标）、证据路径、审查者。” | **列完全一致**；省略 AC 编号规则（新 Spec `<change_id>-AC-<n>`、长期 `AC-<n>`、退役不复用） → NB-2 |
| 8 | 延期清单：AC/工作项、owner、expiry、reason；到期未关闭阻断对应范围；不得删行/批量续期/新 change_id 规避；延期≠豁免；豁免按 18 条 | “AC/工作项、owner、expiry、reason；到期未关闭阻断对应范围；延期≠豁免。” | **核心一致**；省略“不得删行/批量续期/新 change_id”“豁免按第 18 条另行登记” → NB-3 |

**判断**：8 项齐全、无矛盾；override 的 blockquote 明写“本节对照 `.specify/memory/constitution.md` 附录 A”，宪法是权威文本。摘要省略属于**非阻断完整性缺口**，不是错误。

### 2.3 是否引入业务行为/AC 变更

**无。** 这是模板文件，不是 Spec；没有新增/修改任何 AC、业务规则或代码。`git diff a0ec6ba -- .specify/templates/spec-template.md .specify/memory/constitution.md AGENTS.md` 为空；`git status` 唯一新增是未跟踪的 `.specify/templates/overrides/`。

### 2.4 是否写入实例名/运行状态

**0 命中。** `grep 'PAY-SETTLE-01\|PAY-AC50-REVIEW-01\|SPEC-0002\|SPEC-0001'` 在 override 中无结果；override 只引用“宪法附录 A”这一通用文档，不写 change_id/状态/日期/实例名。文件头注释是维护说明，不是运行状态。

### 2.5 覆盖层维护风险是否已在文件头说明

**已说明。** 文件头第 1 行：“项目覆盖层：Spec Kit 1.4.3 spec-template + 宪法附录 A 对照。**核心模板更新时重新生成本文件。**” override 是整体 replace（`common.sh:642-650`：命中 override 直接 `cat` 并 `return 0`），核心模板更新不会自动合并；文件头的“重新生成”是正确提示。

**需人类确认**：审查者不建议由自己代替人类接受“覆盖层 replace 的维护风险”。建议人类在批准时明确“接受/不接受该维护方式”；若接受，后续每次 Spec Kit 升级/核心模板更新都要手工重生成 override。

### 2.6 resolver 输出能否作为 `TEMPLATE_CONTENT`、是否依赖 PyYAML

**可以，且不依赖 PyYAML。** 实测：

- `./.specify/scripts/bash/resolve-template.sh spec-template` → exit 0，146 行，含 `User Scenarios & Testing` 与“附录 A 对照”；
- `./.specify/scripts/bash/resolve-template.sh spec-template --json` → exit 0；`python3 -c 'import json'` 解析出 `TEMPLATE_NAME=spec-template`、`TEMPLATE_CONTENT` 5206 字符，两个字符串均在；
- 本机 `python3 -c 'import yaml'` **失败**（PyYAML 缺失），但 `resolve_template_content` 的 override 分支（`common.sh:642-650`）在 preset/扩展层解析之前就 `cat` + `return 0`，不进入需要 Python/PyYAML 的 preset 分支；`--json` 用 `jq`（本机有）或 `json_escape` 兜底，均不依赖 PyYAML；
- `speckit-specify` 的 SKILL 要求用 `resolve-template.sh spec-template --json` 并解析 `TEMPLATE_CONTENT`（`.dsh/skills/speckit-specify/SKILL.md:95`），实测输出满足该接口。

### 2.7 NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | 业务规则 bullet 省略 EARS 的 `WHEN/IF/WHILE...SHALL` 句式、`MUST` 关键字、`SHALL/THEN/MUST` 形容词关键字；摘要仍指向宪法。 | 宪法附录 A 行 1；override 业务规则 bullet |
| NB-2 | 验收状态表 bullet 只列列名，省略 AC 编号规则（新 Spec `<change_id>-AC-<n>`、长期 `AC-<n>`、退役不复用）。 | 宪法附录 A 行 7；override 验收表 bullet |
| NB-3 | 延期清单 bullet 省略“不得删行/批量续期/新 change_id 规避”与“豁免按第 18 条另行登记”。 | 宪法附录 A 行 8、宪法 18；override 延期 bullet |
| NB-4 | 文件头 HTML 注释会随 `TEMPLATE_CONTENT` 进入每个生成的 `spec.md`；属维护说明，HTML 注释不渲染，但会产生噪音。 | override 第 1 行；`speckit-specify` 把 `TEMPLATE_CONTENT` 写入 `spec.md` |
| NB-5 | 文件头只写“核心模板更新时重新生成本文件”，未写“宪法附录 A 更新时同步本节”；附录 A 摘要也会随宪法修订而漂移。 | override 第 1 行、附录 A 段 |

---

## 3. 裁决

**PASS。** 0 阻断；NB-1…NB-5 不阻断。范围：override 的内容完整性、8 项对照、无业务 AC/实例名、维护说明、resolver 可用性。**覆盖层 replace 的维护风险需人类确认接受**；这是批准条件，不是技术阻断。

---

## 4. 复现命令（只读）

```bash
git rev-parse HEAD                                    # a0ec6ba...
git status --porcelain                                # ?? .specify/templates/overrides/
diff -u .specify/templates/spec-template.md .specify/templates/overrides/spec-template.md
git diff a0ec6ba -- .specify/templates/spec-template.md .specify/memory/constitution.md AGENTS.md   # empty
grep -n 'PAY-SETTLE-01\|PAY-AC50-REVIEW-01\|SPEC-0002\|SPEC-0001' .specify/templates/overrides/spec-template.md   # 0
./.specify/scripts/bash/resolve-template.sh spec-template | wc -l                     # 146
./.specify/scripts/bash/resolve-template.sh spec-template --json | python3 -c "import json,sys;d=json.load(sys.stdin);print(d['TEMPLATE_NAME'],len(d['TEMPLATE_CONTENT']),'附录 A 对照' in d['TEMPLATE_CONTENT'])"
python3 -c 'import yaml; print(yaml.__version__)'        # 6.0.4（人类批准的 user-site）
specify preset list                                     # enabled / Templates: 1
./.specify/scripts/bash/resolve-template.sh spec-template | wc -l   # 145
./.specify/scripts/bash/resolve-template.sh spec-template --json | python3 -c "import json,sys;d=json.load(sys.stdin);c=d['TEMPLATE_CONTENT'];print(len(c),c.count('## 附录 A 对照'))"
ls .specify/templates/overrides 2>&1                    # No such file or directory
# preset append 落盘后（含本报告与台账条目；preset/报告未跟踪，final sha 以 preset 文件 sha256 为准）：
# git diff a0ec6ba | shasum -a 256 -> d8cbf50f92bb555fede382621a1c3ff4c830ffee8623c72af592837cb2718ec8
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 PASS 裁决。未改 override/核心模板/宪法/AGENTS/代码；未 commit、未 merge。
