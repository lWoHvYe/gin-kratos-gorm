# ADR 0006: 协作闸门可执行化（G-1 所有权 / G-2 版本 / G-3 AC↔测试）

- Status: **superseded / retired（2026-11-25）**（原 accepted：Architect 方案；Developer 实现，Reviewer 跑 A1…A5。Agent 工具链已整体删除，见下方 Retirement）
- Date: 2026-11-22
- Revision: **1.1 / Amendment 1**（2026-11-23；T-003 闸门有效性补强）
- Thread: [T-002](../collab/po-architect/threads/T-002.md)；Amendment: [T-003](../collab/po-architect/threads/T-003.md)
- Related: [`docs/collab/CONVENTIONS.md`](../collab/CONVENTIONS.md) §3/§4/§6、[ADR 0004](0004-p3-0-contract-and-production-scope.md)

## Retirement（2026-11-25）

Director 于 2026-11-25 决定**整体退役协作闸门工具链**（非仅 P0a）。此后本 ADR 的机器化设计（含 Amendment 1）**只作设计历史**，不再是执行契约。

- **工具已移除**：`scripts/collab_gates.py`、`scripts/check-collab-gates.sh`、`scripts/test-collab-gates.sh`（由 Lead 删除）。
- **`make verify` 不再运行 `gates`**；Makefile 已无 `gates` / `gates-strict` / `gates-test` 目标。
- **CI 已无 `collab-gates` job**。
- **G-1…G-4 仅作为人工检查清单**保留在 `docs/collab/CONVENTIONS.md` §7（§7.1 边界表）；**无替代工具**，不新增门禁，不恢复脚本/CI/Makefile 集成。
- 人工审查与常规 test/build 门禁不变。

以下原文（Context / Decision / 实现清单 / Reviewer 验收 / Consequences / Amendment 1）保留为历史。

## Context

`CONVENTIONS.md` §4 已定义三条闸门（G-1 所有权、G-2 版本、G-3 AC↔测试），但只是文字约定。真实漂移已经发生：PO 在 20:57 裁决 P-01…P-18 后，ADR §19.2 仍写着“待 PO 裁决”。

T-002 要求：本地可执行、可接 `make verify`/CI、失败必须指名道姓、两档模式、覆盖台账、无网络/Docker、<5 秒、不得用删 AC 或豁免名单绕过。

## Decision

### 1. 单一入口与两档模式

新增 `scripts/check-collab-gates.sh`（bash + git + grep + python3 标准库；不依赖网络/Docker/外部服务）。

```bash
make gates          # warn：打印违规与覆盖率，始终 exit 0；并入 make verify
make gates-strict   # strict：任一违规/未覆盖即 exit 1；CI 与合并前使用
make gates-test     # 自测：临时 git 仓库跑 A1…A5，不碰真实历史
```

- 默认检查范围 `HEAD^..HEAD`（最后一次提交）；无父提交时用 `HEAD`。
- `--range=<base>..<head>` 覆盖范围；CI 用 `origin/main..HEAD`。
- `--staged --role=<Role>` 供 pre-commit：检查暂存路径与给定角色；不依赖 commit trailer。
- `GATES_MODE=warn|strict` 只切换报告/退出码，**不能跳过检查**；不提供 `SKIP_GATES`。
- `make verify` 只挂 warn；`make gates-strict` 是 CI/发布门禁。

失败输出必须可复制：

```text
G-1 FAIL docs/specs/0001-...md  role=Developer  expected=Product Owner
G-2 FAIL docs/specs/0001-...md  version 1.2 -> 1.2 (no bump)
G-2 FAIL docs/specs/README.md   index=1.1  yaml=1.2
G-3 FAIL AC-14                  no test, no valid deferral
G-3 FAIL AC-07                  deferral expired 2026-11-30
SUMMARY G-1=1 G-2=2 G-3=2 mode=strict exit=1
```

### 2. G-1 所有权

**路径 → 唯一所有者**

| 路径 | 所有者 |
|---|---|
| `docs/specs/**` | Product Owner |
| `docs/adr/**` | System Architect |
| `docs/implementation/**` | Developer |
| `docs/po/**` | Product Owner |
| `docs/architect/**` | System Architect |

- 从 commit trailer 解析 `Role:`，支持多值：`Role: Architect, Developer`。
- 角色归一化：`PO`/`Product Owner` → PO；`Architect`/`System Architect` → Architect；`Developer` → Developer；`Reviewer` → Reviewer（不拥有写路径）。
- 一个提交触及的每个受管路径，其所有者必须出现在 `Role:` 中；否则 G-1 FAIL。
- 受管路径变更但**没有** `Role:` trailer：warn 模式打印 `G-1 WARN missing Role`；strict 模式 FAIL（不可验证即不放行）。
- 只读检查 `git diff-tree` 与 commit message；**不修改 git 历史**。
- `docs/collab/**`、`AGENTS.md`、`CONVENTIONS.md`、`scripts/**` 不属于 G-1 的四个业务产物路径；仍受 Reviewer 人工审查。

**唯一豁免：commit trailer `Exception: T-NNN`**

- 只豁免 G-1，不豁免 G-2/G-3。
- `T-NNN` 必须存在 `docs/collab/*/threads/T-NNN.md`；线程 `CLOSED` 时 strict FAIL（过期豁免）。
- 输出必须打印 `G-1 EXEMPT T-NNN <path>`，不得静默。
- **禁止**静态路径豁免名单；禁止把路径挪到不受管目录来绕过。

### 3. G-2 版本

对范围内每个变更的 `docs/specs/NNNN-*.md`：

1. 解析 YAML `version`（旧版本从 `git show <base>:<file>`，新版本从工作区/HEAD）。
2. 新文件：必须有 README 索引行、变更记录含该版本号。
3. 已有文件：
   - `version` 未升 → 除非 commit 带 `Non-normative: true`，否则 G-2 FAIL。
   - `version` 升 → 必须严格大于旧版本；`docs/specs/README.md` 索引版本必须与 YAML 一致；根 `Spec.md` 索引（若存在该行）也必须一致；`## 13. 变更记录` 必须出现新版本号。
4. `Non-normative: true` 只用于链接/错别字修正；warn 模式也必须打印 `G-2 EXEMPT non-normative`；A6 由 Reviewer 人工判断是否滥用。
5. `Exception:` 不适用于 G-2（Spec 版本不能靠线程豁免）。
6. ADR 的版本/Amendment 规则仍按 CONVENTIONS §4.2 人工执行；本 ADR 不把 ADR 语义检查机器化（避免误判已接受的 Amendment）。

### 4. G-3 AC↔测试

**AC 基线（防删除）**

- `docs/collab/ac-baseline.txt`：一行一个 `AC-NN`，升序，只增不减。
- 当前 Spec 的 AC 集合必须 **等于** baseline；缺失 → `G-3 FAIL AC-NN deleted`；新增未登记 → `G-3 FAIL AC-NN not in baseline`。
- 与上一版 baseline 比较：旧 baseline 必须是新 baseline 的子集；baseline 缩短 → FAIL。
- AC 编号必须连续 `1..max`，不得跳号/复用；废弃只能保留 ID 并标 `[DEPRECATED]`，不得删除。

**测试引用**

- 在 `*_test.go` 中查：`TestAC<NN>`（允许 `TestAC0*14`）或注释/字符串 `AC-<NN>`。
- 同时覆盖 tracked（`git grep`）与 untracked（`git ls-files --others`）；不扫 `.tmp`/`node_modules`。
- 一个 AC 至少一个引用；否则看覆盖台账。

**覆盖台账（Reviewer 维护）**

`docs/collab/ac-coverage.tsv`，Tab 分隔：

```text
# ac	owner	expiry	reason	thread
AC-07	Developer	2026-12-15	Playwright 未接 CI	T-004
```

- `expiry` 必须为 `YYYY-MM-DD` 且 ≥ 今天；`owner`/`reason` 非空。
- 未覆盖的 AC 有有效行 → warn 通过；strict 也只警告并打印剩余天数。
- 未覆盖且无行 / 行过期 / 行格式错 → strict FAIL。
- 台账里的 AC 不在 Spec / 已有测试 → warn stale。
- **禁止**用台账长期挂起：到期未清即 FAIL；不得静默续期（续期需改 expiry 并留 Reviewer 记录）。

**Warn vs Strict**

- warn：打印 `AC coverage: N/M`、未覆盖清单、无效台账；exit 0。
- strict：上述任一违规 exit 1；A3 必须列出 `AC-NN`。

### 5. 豁免键语义（只此三个）

| 键 | 位置 | 作用域 | 必须打印 |
|---|---|---|---|
| `Role:` | commit trailer | G-1 角色声明（多值） | 是 |
| `Exception: T-NNN` | commit trailer | 仅 G-1；线程必须存在且未 CLOSED | 是 |
| `Non-normative: true` | commit trailer | 仅 G-2 的链接/错别字 | 是 |

不提供其他键；不接受环境变量跳过；不接受路径白名单。

### 6. 绕过禁令（违反即打回）

以下行为一律视为绕过闸门，Reviewer 直接拒绝：

1. 删除/重编号 Spec 中的 AC，或改 `AC-NN` 编号来“避开”未覆盖。
2. 删除唯一测试、改测试名去掉 `TestAC<NN>`、或把 `AC-<NN>` 注释删掉。
3. 编辑 `ac-baseline.txt` 删除条目、跳号、或把新 AC 不登记。
4. 在 commit 里堆 `Exception: T-NNN` 豁免所有权，或复用一个已关闭线程。
5. 把 `docs/specs/**` 文件挪到不受管目录、改名绕过路径匹配。
6. 用 `Non-normative: true` 提交业务口径变化（A6 人工兜底；发现即回滚）。
7. 改 `CONVENTIONS.md` 删除/弱化 G-1/G-2/G-3，或改闸门脚本让检查恒真。
8. 在 CI 里把 strict 改成 warn、或跳过 `make gates-strict`。

闸门脚本属于**流程基础设施**：Developer 实现，改动需 Architect + Reviewer 双方复核；不得由单人顺手改绿。

### 7. 性能与离线

- 目标：`make gates-strict` < 5 秒（单机，38 个 AC 量级）。
- 只用 `git`/`grep`/`awk`/`python3` 标准库；不联网、不拉镜像、不启动 Docker/DB。
- 结果只读；不写业务库、不改 git 历史；临时文件用 `mktemp` 并清理。

## Developer 实现清单

1. `scripts/check-collab-gates.sh`：`--mode=warn|strict`、`--range`、`--staged --role`、G-1/G-2/G-3、输出格式、退出码。
2. `Makefile`：`gates`、`gates-strict`、`gates-test`；`verify` 增加 `gates`（warn）。
3. `docs/collab/ac-baseline.txt`：从 SPEC-0001 v1.2 生成 AC-01…AC-38。
4. `docs/collab/ac-coverage.tsv`：表头 + 注释；初始未覆盖项由 Reviewer 填 owner/expiry。
5. `scripts/test-collab-gates.sh`：临时 git 仓库合成 A1…A5，断言输出/退出码；不依赖真实历史。
6. CI：新增 `make gates-strict` job（若已有 GitHub workflow），失败上传文本输出。
7. 不改 Spec/ADR；不引入网络/Docker 依赖。

## Reviewer 验收（A1…A5 / A6）

| 编号 | 操作 | 期望 |
|---|---|---|
| A1 | 在临时分支用 `Role: Developer` 改一个 `docs/specs/*.md` 并提交 | `make gates-strict` FAIL，输出点名该文件与 `expected=Product Owner` |
| A2 | 改 Spec 正文不升 `version`；再升 version 但不同步 README | 两次都 FAIL；分别点名 `no bump` 与 `index=... yaml=...` |
| A3 | 删掉某 AC 的唯一测试引用 | strict FAIL，列出 `AC-NN` |
| A4 | 正常提交（正确 Role/版本/测试） | 三项通过，`make verify` 绿 |
| A5 | `time make gates-strict` | < 5 秒 |
| A6 | 提交 `Non-normative: true` 但实际改业务规则 | 机器无法判断语义；Reviewer 人工打回（接受此兜底） |

A1–A5 的终端输出贴进 T-002 的“验证”段，作为关闭证据。

## Consequences

**正面**

- 所有权/版本/AC 覆盖从“口头约定”变成可复制、可阻断的本地检查。
- 漂移在提交时暴露，而不是在 Reviewer 或上线时才发现。
- 豁免必须可见、有线程、有到期日，不能静默。

**代价**

- 新 AC 必须同步 baseline/ledger；多一步机械操作。
- 早期 AC 未覆盖时 warn 会持续提示；strict 只在合并/发布跑。
- `Non-normative` 仍需人工判断；机器只保证它被打印。

**明确不做**

- 不做语义级“是否业务变更”判断（A6 人工）。
- 不把 ADR Amendment 语义机器化（避免误判）。
- 不引入外部 SaaS、Docker、网络依赖。


---

## Amendment 1 — 闸门有效性补强（T-003，2026-11-23）

### Context

T-002 关闭后 `make gates-strict` 输出 `AC coverage: 38/38`，但证明力被高估：

1. **G-3 只证明“存在引用”，不证明“断言充分”**。AC-14 曾缺累计封顶断言，闸门仍是绿的。
2. **deferred/到期项没有执行者**。`ac-coverage.tsv` 的 6 条 deferred 与偏差登记的 expiry 没有机器拦截，CONVENTIONS §6 的“到期升级 BLOCKING”只活在纸上。
3. **扫描范围不可见**。默认 `HEAD^..HEAD`，输出看起来像“全分支已验”；历史 `[mixed]` 混提不会被提示。

本 Amendment 不改变 G-1/G-2 的结论，只补强 G-3 表述、新增 G-4、显式化扫描范围，并写清闸门能力边界。

### 1. G-3 措辞与计数

- 输出改为 `AC-REF <covered>/<total> (references only)`；**禁止**再用 `AC coverage` 这类暗示“达标”的词。
- `strict` 额外输出：
  - `DEFERRED <n>`：`docs/collab/ac-coverage.tsv` 中 `status` 非 `closed` 的行数；
  - `OVERDUE <n>`：同上且 `expiry <= today` 的行数。
- `DEFERRED`/`OVERDUE` 是**计数/债务提示**，不是通过证书；G-3 的失败条件仍是“无引用且无有效 deferral / deferral 过期/缺字段”。
- `status` 列可选（第 6 列，`open|closed`，缺省 `open`）；`closed` 行不计数，但若该 AC 仍无引用，G-3 仍失败。

### 2. 新增 G-4 到期闸门（strict 阻断）

**输入**

1. `docs/collab/ac-coverage.tsv`（列：`ac owner expiry reason thread [status]`）。
2. `docs/implementation/*-deviation-register.md` 的 Markdown 表：表头同时含 `owner` 与 `expiry`（可选 `status`）的表；数据行按表头取列。缺 `status` 视为 `open`；ID 取首列，缺失时用 `<file>#<row>`。

**判定**

- 行 `status != closed` 且 `expiry <= today`（Asia/Shanghai）→ `G-4 FAIL <id> owner=<owner> expiry=<date>`；`owner` 为空时输出 `owner=missing`。
- strict 有任一 FAIL 即 exit 1；warn 只打印。
- 日期必须 ISO `YYYY-MM-DD`；非法日期 → `G-4 FAIL <id> invalid expiry`。

**续期（不得静默变绿）**

- 相对 base 版本，若某行 `expiry` 后移：
  - commit range 的 message 必须含 trailer `Expiry-Renewal: <非空理由>`；否则 `G-4 FAIL <id> renewed without Expiry-Renewal`。
  - 单次运行后移 **超过 3 条** → `G-4 FAIL bulk renewal <n> rows`（必须拆成逐条续期或关闭）。
- `--staged` 无 commit message：跳过续期校验并打印 `G-4 WARN staged: renewal check skipped`。
- 关闭项必须把 `status` 置 `closed`（或从 deferred 表移除）；删除行/批量延后不算关闭。

### 3. 扫描范围可见

- 输出**第一行**：`SCAN RANGE <base>..<head>`；默认范围追加 `(default; release use --range origin/main..HEAD)`；`--staged` 输出 `SCAN RANGE STAGED`。
- 支持 `--range=<base>..<head>`；`origin/main` 不存在时 `G-0 FAIL range base not found`。
- 新增 `--release`：未显式传 `--range` → `G-0 FAIL release requires explicit --range`；发布/合并必须 `--release --range origin/main..HEAD`。
- 若范围内包含偏差登记 D.1 列出的 `[mixed]` 提交：输出 `G-1 WARN [mixed] <short>`；**至少提示，不静默**，但不因历史混提阻断 strict。
- 摘要行增加 G-4：`SUMMARY G-1=<n> G-2=<n> G-3=<n> G-4=<n> mode=<mode> exit=<0|1>`。

### 4. 能力边界（同时写入 CONVENTIONS §4.4）

- **绿 = 引用存在 + 规则未违反**；不等于断言充分，更不等于业务正确。
- G-3 只核对 `TestAC<NN>`/`AC-<NN>` 引用；`DEFERRED` 是债务清单，不是质量证书。
- G-4 只核对到期/状态；不判断测试深度。
- `[mixed]` 是历史提示；不证明整条分支干净。
- 断言深度由 Reviewer 按 ADR-0005 §14 人工判定，并在审查线程留证据；`ac-coverage.tsv` 只登记延期，不替代审查。

### 5. 禁止

- 删除 `ac-coverage.tsv` 的 deferred 行、把 `status` 直接改 `closed` 而实际未完成。
- 批量把 `expiry` 后移（>3 条）来变绿。
- 放宽 G-3 或把 `AC-REF` 改回 `coverage`。
- 用默认 `HEAD^..HEAD` 冒充全分支验证；发布必须显式 range。
- 删/改 `[mixed]` 历史登记来消除提示。

## Developer 实现清单（Amendment 1）

1. `scripts/collab_gates.py`：
   - G-3 输出 `AC-REF ... (references only)`；strict 输出 `DEFERRED/OVERDUE`；
   - G-4 解析 `ac-coverage.tsv` + 偏差登记表；到期/续期/批量续期判定；
   - `SCAN RANGE` 首行；`--range`、`--release`；`[mixed]` 提示；
   - 摘要加 `G-4`。
2. `docs/collab/ac-coverage.tsv`：表头补可选 `status`；现有 6 行保持 `open`。
3. `docs/implementation/adr-0005-deviation-register.md`：含 `owner`+`expiry` 的表补 `status` 列（`open|closed`），或确认缺省 `open` 可用。
4. `scripts/test-collab-gates.sh`：新增 A1…A5 场景（过期、正常计数、range、能力边界存在、<10s）。
5. `Makefile`：`gates-strict` 支持 `RANGE`/`RELEASE` 透传；`gates-test` 覆盖新场景。
6. 不联网、不依赖 Docker；strict 仍只读。

## Reviewer 验收（A1…A5，T-003）

| 编号 | 操作 | 期望 |
|---|---|---|
| A1 | 把 `ac-coverage.tsv` 某行 `expiry` 改为昨天 | strict FAIL，输出 `G-4 FAIL AC-NN owner=... expiry=...` |
| A2 | 正常状态 | 输出含 `SCAN RANGE ...`、`AC-REF 38/38 (references only)`、`DEFERRED 6`、`OVERDUE 0`、`SUMMARY ... G-4=0` |
| A3 | `--release --range origin/main..HEAD` | 输出实际范围；`[mixed]` 提交有 `G-1 WARN [mixed]`；缺 range 时 `G-0 FAIL` |
| A4 | 检查 CONVENTIONS §4.4 与 Reviewer 审查模板引用 | 能力边界段落存在，模板引用该段 |
| A5 | `time make gates-strict ...` | < 10s；无网络/Docker |

## 非阻断 follow-ups（R 建议，均 `[TODO]`，不阻塞 T-003/PASS）

1. `[TODO]` P0-7 回归测试目前只断言 Payment 调用次数与 breakdown；建议用有效多组订单断言“商品退款只含商品、hold 只付该组份额”的实际金额序列，防止“次数对、金额错”。
2. `[TODO]` 长期停在 `REQUESTED/APPROVED` 的售后 hold 不会被 worker 结算，但仍被 `freightCommitted` 计入；建议加到期 `WAIVED` 或排除未批准请求。
3. `[TODO]` AC-22 三工作日告警 worker、AC-28/29/30/35/37 浏览器断言仍按 `ac-coverage.tsv` 的 `expiry=2026-12-15` 跟踪；到期未清升级 BLOCKING。

以上不进入 T-003 的实现范围；Reviewer 的 PASS 不因它们回退。

### Consequences（Amendment 1）

- 绿的含义被降级为“引用/规则未违反”，减少“38/38 = 全好”的误读。
- 到期项有了执行者；续期必须留理由，批量续期被阻断。
- 代价：G-4 增加 Markdown 表解析；`status` 列需要维护；strict 变慢但目标 <10s。
