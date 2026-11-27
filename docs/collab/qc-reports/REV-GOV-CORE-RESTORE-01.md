# REV-GOV-CORE-RESTORE-01 — 宪法 v2.2.0 / AGENTS 的 L2+L3 治理条款恢复审查

| 项 | 值 |
|---|---|
| change_id | **`GOV-CORE-RESTORE-01`** |
| 级别 | **治理变更**（宪法 3 变更分类；AGENTS §4 级别含该值） |
| 阶段 | **Spec** |
| review_scope | 只审 v2.2.0 的治理文本增量：宪法 **12 / 15 / 23 / 26**、版本 2.2.0；AGENTS **§2 integration owner**、**§3 输出第 5 项 + Spec 阶段 bullet**。对照旧宪法 v1.3.0 的 **I.2 / II.1 / II.5 / III.2 / III.4**（`git show 9e81497:.specify/memory/constitution.md`）与旧 `docs/archive/2026-11-27-governance/CONVENTIONS.md` **§2.2 / §3 / §4.1**，并核对与当前宪法 8/9/11/12/13/14/16-19/27-31、AGENTS §1–§6 的冲突。**不审**代码、SPEC-0001/0002 的 AC 语义、C-2/C-3/C-4/D-2、13 个 addendum、任何实现证据。 |
| base | **`9e81497`**（`git rev-parse HEAD` = `9e81497214bad5b4b92cca6063d1ae8edab0d0c1`） |
| head | **工作树 @ `9e81497`（未提交）**。评审输入的字节绑定：`git diff 9e81497` sha256 = `682be3ca70ff76ead2ec2eeffc852ad1e01902b648b73536b226f37097bd7b53`；`.specify/memory/constitution.md` sha256 = `32ae52cfa1b5d05699f3bf12d886314f4ea8e12742e30df2bd3a09c3c97bf81b`；`AGENTS.md` sha256 = `baf82ba092e02ffbb5d6154cf44fe1d91f87ae81416482a60ebd70b99ccfe409`。工作树若在上述字节之外变化，本裁决失效。 |
| final head | **2026-11-27 第二次补入后（最终确认输入）**：宪法 sha256 `d6f21e2d9dd5f7f42942c91d8e8e75f1af4570a4e5bf7f3bc627fe3d5329ae94`、AGENTS sha256 `fb2f7a6e7e82d8d28e1cbf6e876a73903f9f163d83d87577373fc2903b375c19`、`git diff 9e81497` sha256 `d44e0812128230923265f04266a40cdb7a12333c2409ef8a8962c337197a28dd`（评审输入；本轮落盘后见 §0A）。实例名仍 0 命中；final confirmation 结论见 §0A。 |
| NB-11 final | **2026-11-27 NB-11 逐字化后**：宪法 sha256 `2121158a023214b8d4a5bf8e06cf8595f312004aaf963431201de1b211bdb887`、AGENTS sha256 `597fb0f700727a99c596f561b5699daffa97474f1abdf58a3c4e91571cee874e`、`git diff 9e81497` sha256 `38c24b6c315cc87679fc5bf842d74ab43b7a9f2727ab55456f09b841a96d87d0`；**NB-11 逐字化引入新窄化 → FAIL**（见 §0B）。 |
| NB-11 restored | **2026-11-27 NB-11 两层结构恢复后**：宪法 sha256 `beffa845af65d32bec3de24c031d726e7beee4ed26414fb7e70ba01f873c629d`、AGENTS sha256 `597fb0f700727a99c596f561b5699daffa97474f1abdf58a3c4e91571cee874e`、`git diff 9e81497` sha256 `3ff0c5a95038ddc4dd61faa51863fdd450b2e38678310ca4cb952abbfe6f2e19`；**B1 消解，final PASS 成立**（见 §0C）。 |
| 术语更正 | **2026-11-27 人类更正**：本项目没有“外部复审”；审查者 = AGENTS §1 的独立 subagent，审查者之外的其他输入（外部 agent/顾问/自动扫描/对话建议）只作问题提案（宪法 12）。本报告历史段落的旧称已按此更正；旧 CONVENTIONS §2.2 的标题保留为历史引用。**术语更正不影响 PASS**（B1/NB 不受影响）。评审字节：宪法 sha256 `414b94a6b9419a01abc7573036de9eceda4c0f75120f9d6bd9cf9b2163b8ba4f`、AGENTS sha256 `05d948fbac7f466d187bb054143f9fff3d44d310295d62e90a06fc7bc804021b`、`git diff 9e81497` sha256 `7bd92ccd5268ee6637060e0d0dafd2c5d078eedff35e7aeaba7b171fabd38d0d`（术语更正输入；落盘后见 §8）。 |
| 契约输入 | 宪法 v2.2.0（工作树）；根 `AGENTS.md`（工作树）；base `9e81497` 的宪法 v1.3.0 与 `AGENTS.md`；旧 `docs/archive/2026-11-27-governance/CONVENTIONS.md`（只作对照）；`docs/specs/0001-*.md`、`docs/specs/0002-*.md`、`docs/specs/README.md`；`docs/review-tracking.md`；Spec Kit 1.4.3 的 `.dsh/skills/**`、`.specify/scripts/bash/common.sh`（仅在需要核对工具行为时）。`docs/convert/DECISIONS.md`、`A1-SELFCHECK.md` 只作线索，未据其结论判定。 |
| INV | 不涉及领域不变量（无资金/库存/履约/权限/迁移行为变更；INV-01…52 文本未动）。受影响的治理不变量：宪法 **8/9/11/12/13/14/16-19/27-31**；本次恢复的旧条款：v1.3.0 **I.2/II.1/II.5/III.2/III.4**。 |
| verification | **静态审查**（治理文本，无代码可复跑）。实测：`git diff --check 9e81497` exit 0；`shasum -a 256` 绑定宪法/AGENTS；`grep` 核对实例名（0 命中）；`git show 9e81497` 逐条对照旧 I.2/II.1/II.5/III.2/III.4；`sed -n` 对照旧 CONVENTIONS §2.2/§3/§4.1。未运行 Go/MySQL/NATS/Redis/`-race`（不适用）。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge；不批准 v2.2.0 文本进入生效版本 |
| overall verdict | **第一次（2026-11-27）：SCOPED-PASS**（列出的 L2/L3 通过；未覆盖的旧限定见 §5）。**final confirmation（2026-11-27）：PASS**——§5 的未覆盖项与 NB-1…NB-9 已补入并逐项复核消解；NB-10 确认不阻断；剩余为 NB-8 措辞/NB-11/NB-12/NB-13 的非阻断观察（见 §0A）。本报告 §0–§8 保留第一次审查原文，§0A 为第二次 final confirmation。**NB-11 逐字化后（§0B）：FAIL（1 阻断，宪法 23 兼容期范围窄化）**；**NB-11 两层结构恢复后（§0C）：PASS**（B1 消解，历史保留）。 |

---

## 0C. NB-11 两层结构恢复后的 final confirmation（2026-11-27，第四次）— PASS

| 项 | 值 |
|---|---|
| 裁决 | **PASS**（B1 消解；范围：仅 NB-11 两层结构恢复；NB-8/NB-12/NB-13 维持已确认；NB-10 不阻断） |
| final head | 宪法 sha256 `beffa845af65d32bec3de24c031d726e7beee4ed26414fb7e70ba01f873c629d`；AGENTS sha256 `597fb0f700727a99c596f561b5699daffa97474f1abdf58a3c4e91571cee874e`；`git diff 9e81497` sha256 `3ff0c5a95038ddc4dd61faa51863fdd450b2e38678310ca4cb952abbfe6f2e19`；落盘后 hash 见 §8 |
| B1 消解 | 新 23（`:45`）恢复旧 II.1 两层结构：“**兼容期**必须写明默认消费者切换、收口条件、移除日期与回滚路径，不得以‘flag 默认 false’长期保留旧准入路径；**灰度影子期还必须**由 ADR 批准切换步骤、owner 与收口日期，切换完成后的生产默认必须消费 `OrderSettled` 或经 ADR 批准的等价事实。”四项 + flag 禁止回到“兼容期（含 BFF/旧消费者双发）”；ADR 切换步骤/owner/收口日期与生产默认 `OrderSettled` 只对“灰度影子期”追加。**与旧 II.1 两段一致，B1 消解。** |
| 其余逐字化 | NB-8（§3 第 5 项后写入审查报告/台账记 review_scope 与 release_status）、NB-12（§2 金额/库存/权限/履约准入不得只写“不涉及”）、NB-13（§2 plan 写 `baseline_impact`/`stop_conditions`）已复核，无新冲突。 |
| 术语更正 | “外部复审”改为“审查者之外的其他输入”（人类 2026-11-27 更正）；规则语义未变，**术语更正不影响 PASS**（B1/NB 不受影响）。术语更正 final head：宪法 sha256 `414b94a6b9419a01abc7573036de9eceda4c0f75120f9d6bd9cf9b2163b8ba4f`、AGENTS sha256 `05d948fbac7f466d187bb054143f9fff3d44d310295d62e90a06fc7bc804021b`、`git diff 9e81497` sha256 `7bd92ccd5268ee6637060e0d0dafd2c5d078eedff35e7aeaba7b171fabd38d0d`。 |
| 残留 | “ADR 批准的版本化兼容期”仍未逐字出现；附录 B（事件/协议变更须 ADR）与“灰度影子期必须由 ADR 批准切换步骤”可推出，且旧 II.1 的四项/影子期两层已恢复，**不阻断**。NB-10 段落过载仍不阻断。 |
| 结论 | **PASS。** B1 消解；人类批准前补收益/停止条件与版本 2.2.0 的 MINOR 理由即可。 |

---

## 0B. NB-11 逐字化后的 final confirmation（2026-11-27，第三次）— FAIL

| 项 | 值 |
|---|---|
| 裁决 | **FAIL**（范围：仅 NB-11 逐字化后的宪法 23 兼容期范围；NB-8/NB-12/NB-13 逐字化已复核，无新问题） |
| final head | 宪法 sha256 `2121158a023214b8d4a5bf8e06cf8595f312004aaf963431201de1b211bdb887`；AGENTS sha256 `597fb0f700727a99c596f561b5699daffa97474f1abdf58a3c4e91571cee874e`；`git diff 9e81497` sha256 `38c24b6c315cc87679fc5bf842d74ab43b7a9f2727ab55456f09b841a96d87d0`；实例名 0 命中；`git diff --check` exit 0 |
| 阻断 | **B1（NB-11 逐字化）**：新 23 把“默认消费者切换、收口条件、移除日期与回滚路径”和“不得以 flag 默认 false 长期保留”全部挂在“**灰度影子期的兼容期**”下。旧 II.1 第一段把这些要求挂在“版本化兼容期”（含 BFF/旧消费者双发）；只有“ADR 批准的切换步骤/owner/收口日期”和“生产默认必须消费 `OrderSettled`/等价事实”属于灰度影子期。结果：纯 BFF/旧消费者兼容期不再需要移除日期/回滚/收口条件，也不再被禁止以 flag 默认 false 长期保留 → 旧准入路径可无限期保留，违反旧 II.1 的兼容期控制与宪法 6 的渐进式兼容。 |
| 条款 | 旧 v1.3.0 宪法 **II.1** 第一段（“兼容期 MUST 写明默认消费者切换、收口条件、移除日期与回滚路径；MUST NOT 以‘flag 默认 false’长期保留旧准入路径”）；现行宪法 **6**（渐进式兼容）、**23**（`order.paid` 兼容期）。 |
| 需要的语义 | 四项 + flag 禁止应保持在“兼容期”（含 BFF/旧消费者）；ADR 切换步骤/owner/收口日期与生产默认 `OrderSettled` 才限定在“灰度影子期”。这是恢复旧 II.1 的两段结构，不是新增条款。 |
| 其余逐字化 | NB-8（§3 第 5 项后“第 5 项的字段写入审查报告；台账至少记 review_scope 与 release_status”）、NB-12（§2“金额、库存、权限、履约准入变更不得只写‘不涉及’”）、NB-13（§2“plan 必须写 `baseline_impact`/`stop_conditions`”）已复核，未引入新冲突。 |
| 结论 | **FAIL**；B1 是唯一阻断。作者需把四项 + flag 禁止的作用域从“灰度影子期的兼容期”恢复到“兼容期”（含 BFF），ADR 切换步骤/owner/收口日期留在影子期；重新审查后即可 PASS。 |

---

## 0A. Final confirmation（2026-11-27，第二次）

| 项 | 值 |
|---|---|
| 裁决 | **PASS**（范围：§5 的未覆盖旧限定 + NB-1…NB-9 + §3 复审纪律；NB-10 确认不阻断） |
| final head | 宪法 sha256 `d6f21e2d9dd5f7f42942c91d8e8e75f1af4570a4e5bf7f3bc627fe3d5329ae94`；AGENTS sha256 `fb2f7a6e7e82d8d28e1cbf6e876a73903f9f163d83d87577373fc2903b375c19`；评审输入 `git diff 9e81497` sha256 `d44e0812128230923265f04266a40cdb7a12333c2409ef8a8962c337197a28dd`；落盘后 hash 见 §8 |
| 方法 | 重新逐条读当前宪法 12/15/23/26 与 AGENTS §2/§3/§4；对照第一次的 §5 未覆盖项与 NB-1…NB-10；`grep` 实例名 0 命中；`git diff --check` exit 0。未改宪法/AGENTS/代码/tsv/归档。 |

### 逐项确认

| 项 | 补入/现状 | 结论 |
|---|---|---|
| II.5 两条 IF/THEN | 宪法 23（`:45`）：“IF 某履约组存在未完成退款且尚未授权发货，THEN THE SYSTEM SHALL 撤销或拒绝该组的发货 permit；IF 已离仓后发生退款，THEN THE SYSTEM SHALL 保留已发货事实并只退款，不得回滚履约。” | **消解** |
| II.1 第二段 | 23（`:45`）：“兼容期必须写明……必须有 ADR 批准的切换步骤、owner 与收口日期；切换完成后的生产默认必须消费 `OrderSettled` 或经 ADR 批准的等价事实。” | **消解**（“版本化兼容期”字眼未逐字；ADR 批准体现在“ADR 批准的切换步骤”，语义可推出） |
| I.2 限定 | 15（`:31`）首句改为“资金、库存、订单核心链路”；并加“测试替身、开发默认值或 nil 依赖不得弱化生产守卫” | **消解** |
| III.4 apperr | 26（`:48`）：“错误用 `apperr`（内部上下文包装可用 `fmt.Errorf("%w")`，对外错误按 `docs/contracts.md` 的错误契约映射）”；GORM 恢复为“仅 `catalog/gorm_repo.go` 与 `cart/gorm_repo.go`，且仅限读侧” | **消解** |
| §2.2 后果 | 12（`:28`）：“引用找不到的标 `[不确定]`，不得作为需求引用；已被既有 Spec/ADR/INV 裁决的只能按文档漂移处理。本条不削弱第 9 条：权限、安全、数据完整性、迁移风险仍必须阻断。” | **消解** |
| NB-1 | 12（`:28`）末尾的“本条不削弱第 9 条……” | **消解** |
| NB-2 | 23（`:45`）：“任何 commitment 的创建/释放**规则**必须由版本化 Spec/ADR 规定；改变该规则须经人类批准，范围内的单次创建/释放由作者按已批准规则执行。” | **消解**（规则 vs 单次执行已分开） |
| NB-3 | AGENTS §2（`:35`）：“由既有角色兼任，作者或人类指定，不新增角色” | **消解** |
| NB-4/NB-9 | AGENTS §3 第 5 项（`:53`）：`spec_version`/`adr_revision`、`verification`、`INV: 不涉及（依据）`；所有变更至少写不涉及+依据或列 INV；资金/库存/权限/跨服务/数据迁移必须逐条列 INV | **消解**（reviewer 侧；作者侧“金额/库存/权限/履约准入不得只写不涉及”未逐字恢复 → NB-12） |
| NB-6 | §3 第 5 项 `release_status` 定义 + §4 结论行（`:71`）“仅表示该范围证据满足，不是 Release” | **消解** |
| NB-7 | 任务书（`:47`）：“审查者之外的其他输入只作问题提案（宪法 12），不作为证据” | **消解** |
| NB-8 | §4 结论行已有 `release_status` 摘要；第 5 项字段在任务书“输出”清单中，但**未逐字写“写入审查报告”** | **基本消解**（按任务书上下文即报告；若要求逐字，需补一句） |
| §3 复审纪律 | AGENTS §3（`:57`）：“审查期间 HEAD 变化时，重新确认差异和受影响验证；作者的 plan 必须写 `baseline_impact`（‘不涉及’也要写依据）与 `stop_conditions`。” | **消解**（作者 plan 义务放在 §3 而非 §2 第 4 步 → NB-13 观察） |
| NB-10 | 宪法 23 仍是一整段（`:45`），未拆 | **不阻断**。内容正确；若人类要求可读性，可在 23 内用换行/分号分组（事实分离 / 退款-发货 IF/THEN / permit/commitment / Σ / `order.paid` 兼容期），不新增条款编号。 |

### 新残余/观察（均不阻断）

- **NB-8（基本消解）**：“第 5 项字段写入审查报告”未逐字出现；任务书的“输出”清单已包含第 5 项，按上下文即报告内容。若人类要求逐字，补一句即可。
- **NB-11（§0A 时观察，§0B 升级为阻断，§0C 消解）**：旧 II.1 的“ADR 批准的切换步骤/owner/收口日期”原属“灰度影子期”；§0A 时新 23 把它挂在“兼容期”上，BFF-only 兼容也会被要求写切换步骤/owner。**NB-11 逐字化后**，四项兼容期要求也被一起限定到“灰度影子期的兼容期”，范围窄化 → §0B 的 B1；**§0C 两层结构恢复后消解**。
- **NB-12（新观察，不阻断）**：旧 §3 的“金额/库存/权限/履约准入相关变更不得只写‘不涉及’”未逐字恢复；新 §3 只要求计划写“不涉及也要写依据”，reviewer 第 5 项的 INV 强制覆盖了大部分。
- **NB-13（新观察，不阻断）**：作者 plan 的 `baseline_impact`/`stop_conditions` 义务放在 §3（审查者任务书）而不是 §2 第 4 步的 plan 描述；规则可读，但位置与 §2 分离。
- **人类批准前补**：收益/停止条件（Governance 71）；版本 2.2.0 的 MINOR 理由（新增/恢复规则，非删除/重定义原则）。这两项不影响文本 PASS。

### 结论

**PASS。** §5 的未覆盖项与 NB-1…NB-9 已补入并消解；NB-10 确认不阻断；剩余 NB-8 措辞、NB-11/NB-12/NB-13 为非阻断观察。人类批准前补收益/停止条件与版本理由即可。

---

## 0. 第一次审查结论（2026-11-27，保留原文）

1. **SCOPED-PASS，范围是任务书列出的 L2/L3 项 + 版本 2.2.0。** 这些项本身没有发现阻断：12 关于审查者之外的其他输入的规则、§2 的 integration owner、§3 输出第 5 项、15 的 fail-closed/依赖/启动、23 的 permit/commitment/Σ/`order.paid`、26 的 `database/sql`+CAS/GORM 读侧，方向与旧 v1.3.0 一致；2.2.0 是 MINOR（新增规则，不是删除/重定义原则）。
2. **但“恢复”不完整。** 旧 II.5 的两条 IF/THEN（未完成退款→撤销/拒绝 permit；已离仓→保留发货事实、只退款、不回滚）没有写回；旧 II.1 的“ADR 批准的版本化兼容期”、灰度影子期的 ADR 切换步骤/owner/收口日期、切换后生产默认必须消费 `OrderSettled`/等价事实没有写回；旧 I.2 的“开发默认值”和“订单”核心链路范围没有写回；旧 III.4 的 `fmt.Errorf("%w")` 与 `docs/contracts.md` 错误契约映射没有写回。这些是 SCOPED-PASS 的“未覆盖”项，不是本报告新加的阻断。
3. **新引入的措辞问题（NB，不阻断）**：宪法 12 的“外部结论不能直接成为阻断依据”与 9 的“权限、安全、数据完整性、迁移风险必须阻断”没有交叉引用，可能被读成削弱 9 的例外；宪法 23 把旧的“任何 commitment **释放**须版本化 Spec/ADR + 人类批准”扩成“**创建**/释放”，可能被读成每次创建都要人类批准（与 17 的“范围内缺陷修复由作者直接做”冲突）；AGENTS §2 的 integration owner 没有说明它是否为新角色（§1 只有作者/审查者/人类三个角色）；§3 第 5 项的 `release_status=PASS` 没有定义（旧 §5 明确它不是 Release PASS）。
4. **没有实例名/运行状态**：`grep 'PAY-SETTLE-01\|PAY-AC50-REVIEW-01\|SPEC-0002\|SPEC-0001'` 在宪法/AGENTS **0 命中**。`cmd/*/main.go` 是旧条文里的路径模式，不是实例名。
5. **人类批准前还需补**：收益/停止条件（Governance 71）、版本 2.2.0 的 MINOR 理由、§5 的未覆盖旧限定（至少 II.5 两条 IF/THEN 与 II.1 生产默认/ADR 批准）、以及 12/9 与 commitment“创建”的澄清。详见 §6/§7。

---

## 1. 审查对象与方法

### 1.1 本次增量（v2.2.0 相对 v2.1.0）

| # | 位置 | 改动 |
|---|---|---|
| 1 | 宪法 12 | 追加审查者之外的其他输入结论只能作问题提案、必须引用当前 HEAD 文件与行号、不能直接成为契约/AC/阻断依据、先查既有 Spec/ADR 裁决 |
| 2 | AGENTS §2 | 追加跨服务变更在 plan 时指定 integration owner（作者或人类指定），负责基线与组合验证；无 owner 不得进入第 5 步 |
| 3 | AGENTS §3 | 输出加第 5 项 review_scope / release_status / INV / baseline_impact；Spec 阶段 bullet 加 integration owner、baseline_impact、INV |
| 4 | 宪法 15 | 追加必需依赖构造期必填/启动 fail-fast、可选能力显式 Options/禁止 nil、测试替身 `_test.go`/build tag、请求路径不得 panic、启动配置/依赖缺失 fail-fast 非零退出（`cmd/*/main.go` 启动 panic 例外） |
| 5 | 宪法 23 | 追加不得自行恢复 `REVOKED` permit、commitment 创建/释放须版本化 Spec/ADR + 人类批准、准入前不建 commitment、准入后失败进持久重试/人工队列、TCC/SYSTEM 逐组准入 + Σ 等式、`order.paid` 仅 BFF/旧消费者兼容、兼容期四要素 |
| 6 | 宪法 26 | 追加事务核心显式 `database/sql` + 条件 `UPDATE`（CAS）、GORM 仅 `catalog`/`cart` 读侧 |
| 7 | 宪法版本 | 2.1.0 → **2.2.0**（MINOR） |

### 1.2 方法

- 以 `git show 9e81497:.specify/memory/constitution.md` 为旧宪法 v1.3.0 基线，逐条对照 I.2/II.1/II.5/III.2/III.4；以归档 `CONVENTIONS.md` 对照 §2.2/§3/§4.1。
- 以当前宪法 8/9/11/12/13/14/16-19/27-31 与 AGENTS §1–§6 为“现行模型”，检查恢复条款是否与它们冲突或把模型重新加重。
- `docs/convert/DECISIONS.md`、`A1-SELFCHECK.md` 只用于定位（如旧行号/旧条款清单），不作为证据；所有事实用 `git show`/`sed`/`grep` 重新核对。
- 未修改宪法/AGENTS/代码/tsv/归档；本报告与台账条目是 reviewer 落盘。

---

## 2. 与旧宪法 v1.3.0 的逐条对照

### 2.1 I.2（Fail-Closed）→ 宪法 15

| 旧 I.2 的限定 | v2.2.0 现状 | 结论 |
|---|---|---|
| 资金、库存、**订单**的核心链路，未预期异常/超时/配置缺失 Fail-Closed 或持久补偿；不得 try-catch 吞异常/默认放行 | 15（`:31`）覆盖资金/库存链路的异常/超时/配置缺失、不得静默放行/吞异常/砍逻辑；**“订单”未写** | **部分恢复**：范围收窄；“订单”核心链路是 v2.0.0 起就缺失的，本次未补。 |
| 测试替身、**开发默认值**或 nil 依赖不得弱化生产守卫 | 15 有测试替身 `_test.go`/build tag、必需依赖构造期必填、可选能力显式 Options/禁止 nil；**“开发默认值”未写** | **部分恢复**：“开发默认值”是旧 I.2 的显式禁止项，本次未恢复。 |
| 必需依赖（Inventory/Payment/Fulfillment）构造期必填、启动期 fail-fast | 15 有 | **已恢复** |
| 可选能力（pricing/promotion/AI）显式 Options，禁止 nil 表示关闭 | 15 有 | **已恢复** |
| 测试替身只允许 `_test.go`/build tag | 15 有 | **已恢复** |
| 请求路径不得 panic；启动期配置/依赖缺失 fail-fast 非零退出；`cmd/*/main.go` 启动失败 panic 不视为违规（III.4） | 15 有 | **已恢复** |
| 跨服务与对外错误统一 `apperr`（内部 `fmt.Errorf("%w")`），按 `docs/contracts.md` 错误契约映射（III.4） | 26（`:48`）只有“错误用 `apperr`”；无 `fmt.Errorf("%w")`、无 `docs/contracts.md` 映射 | **未恢复**（III.4 的一半）；见 §5。 |

### 2.2 II.1（事实分离与 `order.paid` 兼容期）→ 宪法 23

| 旧 II.1 的限定 | v2.2.0 现状 | 结论 |
|---|---|---|
| 四个事实独立；不得用统一 `status`/单一 `order.paid` 混合表达 | 23（`:45`）首句有 | **已恢复** |
| 唯一例外是 **ADR 批准的版本化兼容期** | 23 只说“`order.paid` 仅可作 BFF/旧消费者兼容”，未写“ADR 批准/版本化兼容期” | **未完全恢复**：附录 B 可间接推出事件变更需 ADR，但旧 II.1 的显式批准要求没有写回。 |
| `order.paid` 不得作为履约准入/结算/库存就绪依据；收口后停止准入用途 | 23 有“不得作为……依据”（绝对禁止），覆盖“收口后停止” | **已恢复**（“收口后停止”被绝对禁止覆盖） |
| 兼容期必须写明默认消费者切换/收口条件/移除日期/回滚路径 | 23 有四项 | **已恢复** |
| 不得以“flag 默认 false”长期保留旧准入路径 | 23 有 | **已恢复** |
| 灰度影子期可保留旧默认，但必须有 **ADR 批准的切换步骤、owner、收口日期**；切换完成后生产默认必须消费 `OrderSettled` 或经 ADR 批准的等价事实 | 23 **未写**影子期、未写 owner/收口日期、未写“生产默认必须消费 `OrderSettled`/等价事实” | **未恢复**：这是旧 II.1 的第二段，属于关键限定；见 §5。 |

### 2.3 II.5（退款与履约准入）→ 宪法 23

| 旧 II.5 的限定 | v2.2.0 现状 | 结论 |
|---|---|---|
| 普通售后、TCC 补偿、SYSTEM 退款进入同一组幂等准入协议，由 permit 裁决退款与发货竞态 | 23 有 | **已恢复（一般原则）** |
| **IF 某履约组存在未完成退款且尚未授权发货，THEN 撤销或拒绝该组的发货 permit** | 23 **未写**具体结果 | **未恢复**：一般原则不能替代这条具体不变量；见 §5。 |
| **IF 已离仓后发生退款，THEN 保留已发货事实并只退款，不得回滚履约** | 23 **未写** | **未恢复**：这是事实所有权/履约不可回滚的关键不变量；见 §5。 |
| TCC/SYSTEM 逐组准入，Σ 等式，不得用整单退款事件替代逐组准入 | 23 有（逐组、Σ、不得整单替代） | **已恢复** |
| 不得自行恢复 `REVOKED` permit；恢复须版本化 Spec/ADR + 人类批准 | 23 有 | **已恢复** |
| 准入前拒绝/取消不得创建 commitment；准入后失败进持久重试/人工队列，不得静默释放 | 23 有 | **已恢复** |
| **任何 commitment 释放**须版本化 Spec/ADR + 人类批准 | 23 写成“任何 commitment 的**创建/释放**必须由版本化 Spec/ADR 规定并经人类批准” | **部分/新增**：旧文只要求“释放”；新增“创建”可能被读成每次创建都要人类批准，与 17 的“范围内缺陷修复由作者直接做”冲突；见 NB-2。 |

### 2.4 III.2（数据库访问）→ 宪法 26

| 旧 III.2 的限定 | v2.2.0 现状 | 结论 |
|---|---|---|
| 事务核心必须显式 `database/sql` + 条件 `UPDATE`（CAS） | 26 有 | **已恢复** |
| GORM 仅允许出现在 `catalog/gorm_repo.go` 与 `cart/gorm_repo.go`，且仅限读侧 | 26 说“GORM 仅限 `catalog`/`cart` 的读侧” | **部分**：读侧保留，文件级限定放宽为包级；见 NB-5。 |

### 2.5 III.4（静态守卫）→ 宪法 26/15

| 旧 III.4 的限定 | v2.2.0 现状 | 结论 |
|---|---|---|
| `internal/platform` 不依赖 `internal/services` | 26 有 | **已恢复** |
| 跨服务/对外错误统一 `apperr`，内部 `fmt.Errorf("%w")`，按 `docs/contracts.md` 错误契约映射 | 26 只有“错误用 `apperr`” | **未恢复**（`%w`/契约映射）；见 §5。 |
| 请求路径不得 panic；启动期配置/依赖缺失 fail-fast 非零退出；`cmd/*/main.go` 启动 panic 例外 | 15 有 | **已恢复** |

**小结**：旧 I.2/II.1/II.5/III.2/III.4 的“骨架”恢复了，但 **II.5 的两条 IF/THEN、II.1 的影子期/生产默认、I.2 的开发默认值/订单范围、III.4 的 apperr 契约映射**没有恢复。这些是本报告 §5 的未覆盖项。

---

## 3. 与旧 `CONVENTIONS.md` §2.2 / §3 / §4.1 的对照

### 3.1 旧 §2.2“外部复审/审计输入”（现称“审查者之外的其他输入”） → 宪法 12

| 旧 §2.2 | v2.2.0 现状 | 结论 |
|---|---|---|
| 审查者之外的其他输入（外部 agent/顾问/自动审计）结论只能作问题提案，不得直接成为契约/AC 依据 | 12（`:28`）有；“不能直接成为契约、AC 或阻断依据” | **已恢复**（“阻断依据”是新增措辞，见 NB-1） |
| 引用必须指向当前 HEAD 存在的文件与行号；找不到的标 `[不确定]`，不得作为需求引用 | 12 有“必须引用当前 HEAD 的文件与行号”；**未写**“找不到→`[不确定]`、不得作为需求引用” | **部分**；见 §5。 |
| 每条结论按所有者分流（业务→PO、设计→Architect、实现→Developer、验收→Reviewer） | 新模型无 PO/Architect/Developer/Reviewer 角色；未恢复 | **不适用**（新模型只有作者/审查者/人类）；不构成缺口。 |
| 分流前核对是否已被既有 Spec/ADR/INV 裁决；已裁决的只能按文档漂移处理 | 12 有“先查是否已被既有 Spec/ADR 裁决”；**未写 INV、未写“已裁决→文档漂移”** | **部分**；见 §5。 |

### 3.2 §3 变更输入包与 integration owner → AGENTS §2

| 旧 §3 | v2.2.0 现状 | 结论 |
|---|---|---|
| 每个跨服务变更指定一名 integration owner | AGENTS §2（`:35`）有“跨服务变更在写 plan 时指定一名 integration owner” | **已恢复** |
| 由现有服务 Owner 或 Architect 兼任，**不新增角色** | AGENTS §2 说“作者或人类指定”；**未写 owner 的身份/不新增角色**；§1 只有作者/审查者/人类 | **部分**：owner 是否为新角色未定义；见 NB-3。 |
| 实现前确定 change_id/Tier/base/spec_version/adr_revision/integration_owner/baseline_impact/affected_AC/allowed_paths/forbidden_paths/stop_conditions/evidence_and_output | 新模型无 Tier/L0-L1-L2/allowed_paths；只恢复了 integration owner（§2）与 reviewer 的 baseline_impact/INV（§3） | **部分（最小化）**：Tier 等已废弃，不必搬回；但 `spec_version/adr_revision`、作者侧 `baseline_impact`、`stop_conditions` 未明确恢复。见 §5。 |
| Reviewer 开始前记录精确 head；审查期间 HEAD 变化必须重新确认差异/受影响验证 | AGENTS §3 的任务书只有 base/head 字段 | **未恢复**：没有“HEAD 变化→重新确认”的显式规则；见 §5。 |
| `baseline_impact` 只能“不涉及（依据）”或列受影响链路/INV/AC+回归证据；`不涉及` 必须说明为什么不影响任何 INV；金额/库存/权限/履约准入不得只写“不涉及” | AGENTS §3 第 5 项要求 INV/baseline_impact；**未写**“不涉及+依据”、未写“金额/库存/权限/履约准入不得只写不涉及” | **部分**；见 §5。 |
| 后续 SDD 不得只验证新增 AC 而忽略旧基线链路 | 宪法 12 有非回归义务；AGENTS §3 Spec bullet 有 baseline_impact | **部分覆盖** |

### 3.3 §4.1 Reviewer 元数据 → AGENTS §3 第 5 项

| 旧 §4.1 字段 | v2.2.0 现状 | 结论 |
|---|---|---|
| change_id / base / head | 任务书头部有 | **覆盖** |
| Tier | 新模型无 Tier | **不适用** |
| spec_version / adr_revision | 任务书无 | **未恢复**；见 §5。 |
| integration_owner | 第 5 项无；Spec bullet 要求“核对 integration owner” | **部分**：要求核对，但不在第 5 项的输出清单里。 |
| review_scope | 第 5 项有 | **覆盖** |
| baseline_impact | 第 5 项有 | **覆盖** |
| INV（或“不涉及+依据”） | 第 5 项要求 INV，但只对资金/库存/权限/跨服务“逐条列”；**未写“不涉及+依据”**，未覆盖数据迁移 | **部分**；见 NB-4/§5。 |
| affected_AC | 输出第 3 项“AC 位移”覆盖 | **覆盖** |
| verification | 第 5 项无 | **未恢复**；见 §5。 |
| release_status | 第 5 项有“NOT RELEASE / PASS” | **部分**：值有了，但 PASS 的条件/含义未定义（旧 §5 定义过）；见 NB-6。 |
| Actor / Reviewed-by | AGENTS §4 格式有 | **覆盖** |

---

## 4. 与当前宪法/AGENTS 的冲突检查

### 4.1 与宪法 8/9/11/12/13/14/16-19/27-31

- **8/11/13/14/16-19/27-31：未发现新冲突。** 12 关于审查者之外的其他输入的规则不改变“审查者必须是另一个实例”（8）；13 的抗压证据、14 的三件事、16 的人类合入/Release、17 的三类裁决、18 的豁免逐台账、19 的异议服从、27 的状态家、29 的谁写谁落账、30 的 change_id、31 的冻结均未被新条款改写。
- **9 vs 12（NB-1）**：9 规定“权限、安全、数据完整性、迁移风险不受可溯源限制，必须阻断”；12 新增“审查者之外的其他输入……不能直接成为……阻断依据”。两者可以调和（外部报告只是提案，审查者独立核实后再按 9 阻断），但 12 没有引用 9 的例外，存在被读成“外部安全审计不能阻断”的风险。**建议人类在批准前明确 12 不削弱 9 的例外**；本次记为 NB，不阻断。
- **17 vs 23（NB-2）**：23 的“任何 commitment 的创建/释放必须由版本化 Spec/ADR 规定并经人类批准”若读成“每次创建都要人类批准”，会与 17 的“范围内的缺陷修复由作者直接做”冲突，也不符合 commitment 在准入协议运行期创建的机制。按“规则须由 Spec/ADR 规定”的读法可调和；文本未消歧。
- **16 vs AGENTS §3 `release_status=PASS`（NB-6）**：16 说 Release 由人类做；旧 §5 明确 `release_status=PASS` 是审查者的发布证据结论，不等于 Release PASS。新 §3 第 5 项只给值域，没有定义 PASS 的条件/边界，容易被误读为 Release 批准。
- **15/23/26 与 8/11/13/14/16-19/27-31**：未发现冲突；15/26 是实现约束，23 是领域不变量，均不改变审查/批准/状态所有权。
- **12 与 AGENTS §3 任务书“只依据 Spec、ADR、宪法、base/head 代码/plan/日志”**：不冲突——外部结论只能是“问题提案”，不是判断依据；审查者仍须回到允许的来源。任务书没有提到审查者之外的其他输入，属表述缺口（NB-7），不是冲突。

### 4.2 与 AGENTS §1–§6

- **§1 角色表 vs §2 integration owner（NB-3）**：§1 只有作者/审查者/人类；§2 新增“integration owner”，没有说明它是既有角色兼任还是新角色。旧 §3 明说“由现有服务 Owner 或 Architect 兼任，不新增角色”。新模型没有服务 Owner/Architect，若 integration owner 是第四种角色，会与 §1 的角色矩阵冲突；若是作者/审查者/人类之一，应写明。**本次 NB，不阻断。**
- **§1 “作者不做合入” vs §2 integration owner**：integration owner 只“负责基线与组合验证”，不包含合入；与 16/§1 的“人类合入”不冲突。若 integration owner 被指派给作者，作者做基线/组合验证属于自检，不等于独立审查；审查者仍独立。**无冲突。**
- **§3 第 5 项 vs §1 “审查者不改 Spec/ADR 正文（验收状态表除外）”**：review_scope/release_status/INV/baseline_impact 写在审查报告/台账，不写 Spec 正文；不冲突。**但** §4 的台账条目格式没有这些字段，只有“范围/阻断项/AC 位移”；审查者若只在台账写 §4 字段，第 5 项就落在报告里——旧 §4.1 明确要求审查报告包含它们，新 §3 第 5 项没有说“写在哪里”。**表述缺口（NB-8）**，不是冲突。
- **§3 第 5 项 INV 范围（NB-4）**：只对“资金、库存、权限、跨服务”强制逐条列 INV；旧 §4.1 对**所有**变更要求“至少列出可能影响的业务不变量”，且“不涉及”也要给依据。数据迁移不在新清单里，而 9 说迁移风险必须阻断。**NB。**
- **§3 第 5 项缺 `spec_version/adr_revision`、`verification`（NB-9）**：旧 §4.1 要求；新任务书没有。**NB。**
- **§4 台账格式 vs §3 第 5 项**：见 NB-8。
- **§5 交付要求 vs §3 第 5 项**：§5 说审查报告放 `docs/collab/qc-reports/`（治理变更）；第 5 项的元数据随报告走。**无冲突。**
- **§6 推荐工具**：与本次恢复条款无关；未改动。

### 4.3 与“压缩过度/恢复过头”

- **压缩过度（有）**：见 §2/§3 的“未恢复”项，最重的是 II.5 两条 IF/THEN、II.1 影子期/生产默认、I.2 开发默认值/订单范围、III.4 apperr 契约映射。
- **恢复过头（有限）**：23 把 II.1 与 II.5 合并成一段，句子很长，事实分离/准入/commitment/Σ/`order.paid` 挤在一起，可读性下降；23 新增“commitment 创建”超出旧文（NB-2）；其余恢复是旧条款的骨架，没有把旧 §3 的完整输入包/Tier/L0-L1-L2、§4.1 的 Tier/spec_version/adr_revision 全搬回来——这一点是合理的**最小化**，不算“恢复过头”。

---

## 5. 未覆盖的旧条款限定（SCOPED-PASS 的 scope）

> 以下不是“本次改动写错了”，而是**第一次审查时尚未恢复**的旧限定。**final confirmation（§0A）已逐项补入**；本节保留第一次审查的状态描述。

1. **II.5 两条 IF/THEN**：①某履约组存在未完成退款且尚未授权发货 → 撤销或拒绝该组发货 permit；②已离仓后发生退款 → 保留已发货事实、只退款、不得回滚履约。新 23 只有“由 permit 裁决退款与发货竞态”的一般原则，没有这两条具体结果。**这是最重要的未覆盖项**（资金/履约竞态）。
2. **II.1 的第二段**：ADR 批准的版本化兼容期；灰度影子期的 ADR 批准切换步骤/owner/收口日期；切换完成后生产默认必须消费 `OrderSettled` 或经 ADR 批准的等价事实。新 23 覆盖了 `order.paid` 的禁止与兼容期四要素，但没有这段。
3. **I.2 的“开发默认值”**：旧文“测试替身、开发默认值或 nil 依赖不得弱化生产守卫”；新 15 有测试替身/nil/依赖，但没有“开发默认值”。
4. **I.2 的“订单”核心链路**：旧文“资金、库存、订单的核心链路”；新 15 第一句只有“资金、库存链路”（v2.0.0 起如此，本次未补）。
5. **III.4 的 apperr 细节**：内部 `fmt.Errorf("%w")` 与 `docs/contracts.md` 错误契约映射；新 26 只有“错误用 apperr”。
6. **§2.2 的两个后果**：引用找不到时标 `[不确定]`、不得作为需求引用；已由既有 Spec/ADR/INV 裁决的只能按文档漂移处理。新 12 有“必须引用当前 HEAD 文件与行号”“先查是否已被 Spec/ADR 裁决”，但没有这两个后果，也没有 INV。
7. **§3 的变更输入与复审纪律**：`spec_version/adr_revision`、作者侧 `baseline_impact`/`stop_conditions`；审查期间 HEAD 变化必须重新确认差异/受影响验证；`baseline_impact` 的“不涉及+依据”与“金额/库存/权限/履约准入不得只写不涉及”。新 AGENTS 只恢复了 integration owner 与 reviewer 的 INV/baseline_impact。
8. **§4.1 的字段**：`spec_version/adr_revision`、`verification`、`INV: 不涉及（依据）`；以及数据迁移也应列 INV（新 §3 第 5 项只列资金/库存/权限/跨服务）。
9. **`release_status=PASS` 的定义**：旧 §5 的条件（精确 base/head、阻断清零、延期处置、受影响基线非回归、要求的测试/环境证据）与“不等于 Release”的边界；新 §3 第 5 项只有值域。
10. **收益/停止条件与版本理由**：Governance 71 要求治理变更写明可验证收益和停止条件；当前 diff 没有 GOV-CORE-RESTORE-01 的记录。2.2.0 作为 MINOR 合理（新增规则），但人类批准记录应写明“为什么不是 MAJOR”（本次是恢复旧规则/新增规则，不是删除或重定义原则）。

---

## 6. 实例名、版本与“恢复过头”核对

- **实例名/运行状态**：宪法/AGENTS 对 `PAY-SETTLE-01`/`PAY-AC50-REVIEW-01`/`SPEC-0002`/`SPEC-0001` 的 grep **0 命中**；本次没有把台账里的 change_id/实例名写回宪法。`cmd/*/main.go` 是旧 III.4 的路径模式，不是实例名。
- **版本 2.2.0**：合理。Governance 版本规则“新增原则或规则为 MINOR”；本次是新增/恢复规则，没有删除或重定义原则，MINOR 成立。建议人类批准记录像 v2.1.0 那样写“MINOR（非 MAJOR）+ 理由”，避免下一轮再争。
- **收益/停止条件**：可写。收益可写“恢复 v1.3.0 的资金/履约/启动失败硬约束，使 L2/L3 审查有可引用的宪法条款，而不是只靠 Spec/ADR”；停止条件可写“若恢复条款导致与现行 8/9/17 冲突、或同一变更因恢复条款被第二次判为设计问题，则回退对应条款并记入台账；连续两个变更后复查”。这是**人类批准记录**要写的，不是作者在本 diff 里必须写的（但 Governance 71 要求“写明”，建议批准前补）。
- **恢复过头**：见 §4.3；主要是 23 的合并段与“commitment 创建”措辞。**压缩过度**：见 §5；主要是 II.5/II.1/I.2/III.4 的未覆盖限定。

---

## 7. 裁决、阻断项与 NB

### 7.1 裁决

**SCOPED-PASS。** 范围：任务书列出的 L2（审查者之外的其他输入/integration owner/reviewer 元数据）+ L3（15/23/26）+ 版本 2.2.0。列出的项已按旧语义恢复，未发现违反宪法 8/11/13/14/16-19/27-31 或 AGENTS §1–§6 的阻断；未覆盖的旧限定见 §5，需人类批准前补入或明确接受。

### 7.2 阻断项

**无。** 本次没有发现“列出的恢复项写错/与现行宪法直接冲突”的阻断。12/9 与 commitment“创建”按 NB 处理，因为存在可调和的读法；如果人类认为 12 必须明确保留 9 的例外、或 23 的“创建”必须按每次实例理解，则应转为阻断（见 NB-1/NB-2）。

### 7.3 NB（本次引入/未消解）

| # | 位置 | 问题 | 依据 |
|---|---|---|---|
| NB-1 | 宪法 12 `:28` vs 9 `:22` | “外部结论不能直接成为阻断依据”未引用 9 的“权限/安全/数据完整性/迁移风险必须阻断”例外；可被读成削弱 9。 | 宪法 9/12 |
| NB-2 | 宪法 23 `:45` | 旧文只要求“任何 commitment **释放**须版本化 Spec/ADR + 人类批准”；新文扩成“**创建**/释放”，可被读成每次创建都要人类批准，与 17 冲突。 | 宪法 17/23；旧 II.5 |
| NB-3 | AGENTS §2 `:35` / §1 `:7-9` | integration owner 未说明是否为既有角色兼任；§1 只有作者/审查者/人类，旧 §3 明说“不新增角色”。 | AGENTS §1/§2；旧 §3 |
| NB-4 | AGENTS §3 `:53` | INV 只对资金/库存/权限/跨服务强制；旧 §4.1 对所有变更要求“至少列可能影响的 INV”，且“不涉及”要给依据；数据迁移不在新清单。 | 宪法 9；旧 §4.1 |
| NB-5 | 宪法 26 `:48` | GORM 从旧 III.2 的“仅 `catalog/gorm_repo.go`、`cart/gorm_repo.go` 读侧”放宽为“`catalog`/`cart` 读侧”；读侧保留，文件级限定丢失。 | 旧 III.2 |
| NB-6 | AGENTS §3 `:53` | `release_status=PASS` 只给值域，未定义条件/边界；旧 §5 明确它不等于 Release PASS。 | 宪法 16；旧 §5 |
| NB-7 | AGENTS §3 `:47` | 任务书“只依据 Spec/ADR/宪法/代码/plan/日志”没有提到 12 的审查者之外的其他输入；12 说这些输入只能作问题提案，二者可调和但未交叉引用。 | 宪法 12；AGENTS §3 |
| NB-8 | AGENTS §3 `:53`/§4 `:67-73` | 第 5 项要求 review_scope/release_status/INV/baseline_impact，但 §4 台账格式没有这些字段，也没说写进审查报告；旧 §4.1 明确要求报告包含。 | AGENTS §3/§4；旧 §4.1 |
| NB-9 | AGENTS §3 `:53` | 缺 `spec_version/adr_revision` 与 `verification`；旧 §4.1 要求。 | 旧 §4.1 |
| NB-10 | 宪法 23 `:45` | 23 把 II.1 与 II.5 合并成一段，事实分离/准入/commitment/Σ/`order.paid` 混在一起；不是错误，但可读性/可引用性下降，容易漏读。 | 结构/可读性 |

### 7.4 `[TODO]`

- `[TODO]`：人类对“L3”的完整范围原文不在仓库；本报告只能以任务书列出的 L3 项（4-7）为本次范围，无法核对“L3 是否包含 II.5 两条 IF/THEN、II.1 影子期”的人类原意。若人类认为包含，§5 第 1/2 项应升为阻断。

---

## 8. 复现命令（只读）

```bash
git rev-parse HEAD                         # 9e81497...
shasum -a 256 .specify/memory/constitution.md AGENTS.md
git diff 9e81497 | shasum -a 256           # 682be3ca...（评审输入）
git diff --check 9e81497                   # exit 0
# final confirmation 落盘后（含本报告与 PASS 条目）：0653ab07450032557a909e9667a84dcd608f35192fce1e916494a843bb794d13
# 第一次审查落盘后：32cbfaf288c4b8cfa8acf7a2ea8f4fd1bba2b7bf6c4a2dd3b350d3f5fef931a6
# NB-11 逐字化 FAIL 落盘后（含本报告与 FAIL 条目）：e7f4a6d6c9170bd62b26779140f3b4076cf61e7230d7ac520ecb70cc3c91073b
# NB-11 两层结构恢复 final PASS 落盘后（含本报告与 PASS 条目）：efa2004612d54df09922b3ccd698e73ae5956d0452704965ee343907e9e757f3
# 术语更正（外部复审 -> 审查者之外的其他输入）落盘后：571374ec3b66b036546b917b9eae95a5d8b177a453e3b9203446a4decdf0ddda
grep -n 'PAY-SETTLE-01\|PAY-AC50-REVIEW-01\|SPEC-0002\|SPEC-0001' .specify/memory/constitution.md AGENTS.md   # 0
git show 9e81497:.specify/memory/constitution.md | sed -n '/防御性架构/,/契约优先/p'
sed -n '28p;31p;45p;48p' .specify/memory/constitution.md
sed -n '35p;53p;56p;57p' AGENTS.md
sed -n '70,76p;106,140p;149,166p' docs/archive/2026-11-27-governance/CONVENTIONS.md
```

**本次落盘**：本报告（含 §0A final confirmation、§0B NB-11 FAIL、§0C NB-11 两层结构恢复 PASS）+ `docs/review-tracking.md` 最新块顶部四条裁决（SCOPED-PASS、final confirmation PASS、NB-11 逐字化 FAIL、NB-11 两层结构恢复 PASS）。并在 final PASS 条目加术语更正说明（“外部复审” → “审查者之外的其他输入”，不影响 PASS）。未改宪法、`AGENTS.md`、代码、tsv、归档；未 commit、未 merge。
