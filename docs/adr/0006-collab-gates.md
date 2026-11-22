# ADR 0006: 协作闸门可执行化（G-1 所有权 / G-2 版本 / G-3 AC↔测试）

- Status: **accepted**（Architect 方案；Developer 实现，Reviewer 跑 A1…A5）
- Date: 2026-11-22
- Thread: [T-002](../collab/po-architect/threads/T-002.md)
- Related: [`docs/collab/CONVENTIONS.md`](../collab/CONVENTIONS.md) §3/§4/§6、[ADR 0004](0004-p3-0-contract-and-production-scope.md)

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
