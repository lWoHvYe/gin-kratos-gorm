# 多 Agent 协作约定 v2（执行细则）

> 本文件是 [`AGENTS.md`](../../AGENTS.md) 的**执行细则**，不改变角色边界；两者冲突时以 `AGENTS.md` 为准。
> **所有 Agent 会话开始必须读本文件**，并与自己的角色段落对照执行。

---

## 0. 一句话

**业务结论只活在 Spec，设计结论只活在 ADR；线程只是过程指针；每条验收项必须能在测试里被找到；每次改动必须能按编号反查。**

---

## 1. 产物与所有权

| 产物 | 路径 | 唯一作者 | 其他角色可以做什么 |
|---|---|---|---|
| 业务规格 | `docs/specs/**` | Product Owner | 只读；要改 → 开线程 |
| 设计决策 | `docs/adr/**` | System Architect | 只读；要改 → 开线程 |
| 实现偏差 | `docs/implementation/**` | Developer | 只读；PO/Architect 通过线程要求改 |
| 协作线程 | `docs/collab/*/threads/**` | 双方各自追加 | 不改对方已写内容 |
| 代码与测试 | `internal/**`、`cmd/**`、`api/**`… | Developer | Reviewer 只提议，不直接改 |
| PO 记忆 | `docs/po/**` | Product Owner | 只读 |

**禁止**：任何角色直接编辑他人产物。需要对方改，只能开线程。

---

## 2. 单一事实源阶梯

```text
Spec（业务） > ADR（设计） > 代码
```

- 过程文档（`register`、`threads`、`implementation/`）**只放指针，不得承载新的业务规则**。
- 一条规则如果只存在于聊天或线程里，等于**没定**。
- 代码与 Spec 冲突时以 Spec 为准（Spec 与基线冲突时，看 Spec 是否显式废止该基线条目）。

---

## 3. 线程协议（一文件一线程）

**路径**：`docs/collab/<pair>/threads/T-NNN.md`；**索引**：同目录 `threads.md` 只放一张表，不放正文。

- 编号 `T-NNN` 三位递增，**永不复用**；一问一线程。
- 状态机：`OPEN`（等对方） → `ANSWERED`（已答复，等回写） → `CLOSED`（已回写并证实）。
- 文首 YAML 必填：`id / title / opened_by / opened_at / blocking / status / spec / adr / closed_by`。
- 正文必填：现象、Spec 原文引用（带章节）、ADR 现处理、影响（金额/用户承诺/契约/数据）、选项与建议、**裁决（结论 + 落点章节 + 影响 INV/AC）**、**回写（改了哪节 + commit）**。
- `BLOCKING: true`：PO **48 小时内**给口径；未裁决**不得开工**（fail-closed）。非阻塞项按 Spec 默认值执行。
- **关闭三条件**（缺一不可）：
  1. 裁决已写入 Spec，版本号已升，变更记录注明影响面；
  2. ADR 已回写，且删除所有"待裁决"标记；
  3. 偏差登记对应条目已 `closed` 或已有明确排期。

---

## 4. 三条闸门

### 4.1 验收闸门：AC ↔ 测试

- 每条 `AC-NN` 必须至少被一个测试引用：测试函数名含 `TestAC<NN>`，或测试文件内注释含 `AC-<NN>`。
- 命名示例：`func TestAC14_FreightRefundByGroup(t *testing.T)`。
- Reviewer 核对（在仓库根执行）：

  ```bash
  for n in $(seq -w 1 38); do
    c=$(grep -rl "AC-$n" --include='*_test.go' . | wc -l | tr -d ' ')
    [ "$c" = "0" ] && echo "MISSING AC-$n"
  done
  ```

- 没有测试的 AC 一律标 `[未覆盖]`，**不得判 done**。

### 4.2 版本闸门

- Spec 业务内容变更：`version` 升版 + 变更记录写影响 INV/AC + 索引表同步 + commit。
- ADR 变更：新增 `Amendment N` 或 supersede，**不原地改写已接受的结论**。
- 仅错别字/链接修正：可不升版，但 commit 必须注明 `non-normative`。

### 4.3 所有权闸门

- 提交前自查 `git diff --name-only`；出现不属于自己的产物路径 → 停止，改走线程。
- 有 CI 时，路径所有权检查失败即红（由 Architect 出方案、Developer 实现）。
- Reviewer 复核是否混提、是否越权。

---

## 5. Commit 约定

格式：`<type>(<scope>): <摘要>`，结尾追加 trailers：

```text
docs(spec): SPEC-0001 v1.1 -> v1.2 裁决 P-01…P-18

<正文：为什么改、影响什么>

Spec: SPEC-0001 v1.2
ADR: ADR-0005
Thread: T-001
AC: AC-14, AC-31
Role: PO
```

- trailer 键：`Spec` / `ADR` / `Thread` / `AC` / `Role`。
- 一笔 commit 只做一件事；**Spec 不与代码同笔提交**（目录首次建立除外）。
- 反查：`git log --grep 'Thread: T-001'`、`git log --grep 'AC: AC-14'`。

---

## 6. 偏差与兼容（防止 shim 变永久债）

- 每条偏差**必须**具备：`owner`、原因、`expiry`（到期日）、复审节奏。
- 到期未清 → 自动升级为 `BLOCKING` 线程；**不得静默续期**。
- 允许"临时兼容"，不允许"无主兼容"。

---

## 7. 各角色最小动作

| 角色 | 开场必读 | 交付物 | 绝对不做 |
|---|---|---|---|
| Product Owner | `AGENTS.md`、本文件、`docs/po/MEMORY.md`、线程索引 | 升版 Spec（YAML + 变更记录 + 索引） | 写代码、改 ADR/代码、在聊天里定规则 |
| System Architect | `AGENTS.md`、本文件、Spec 当前版本、线程索引 | ADR / Amendment | 写业务代码、改 Spec |
| Developer | `AGENTS.md`、本文件、Spec + ADR + 偏差登记 | 代码、测试（带 AC 编号）、偏差登记（带 owner/expiry） | 改 Spec/ADR、无主 shim、`// TODO` 掩盖冲突 |
| Reviewer | `AGENTS.md`、本文件、Spec、ADR | 审查结论（按 AC 逐条 + 红队问题） | 微观完美主义阻断交付、放过未覆盖 AC |

---

## 8. 给人类的启动指令（可直接粘贴）

**通用开场（每个 Agent 都先说这句）**

```text
先读 AGENTS.md 和 docs/collab/CONVENTIONS.md，按你的角色执行；
不要修改别人的产物，需要改动就开线程。
```

**给 Product Owner**

```text
你是 PO。读 AGENTS.md、docs/collab/CONVENTIONS.md、docs/po/MEMORY.md、
docs/collab/po-architect/threads.md。有 OPEN 且 BLOCKING 的线程先裁决。
产出/升版 Spec 必须：YAML 版本 + 变更记录（写明影响 INV/AC）+ 索引同步 + commit（带 trailers），
升版后做二次确认（时限单位、非目标 vs 正文、编号连续性、索引同步、跨节引用）。
```

**给 System Architect**

```text
你是 Architect。读 AGENTS.md、docs/collab/CONVENTIONS.md、当前 Spec 版本、
docs/collab/po-architect/threads.md。OPEN 线程按"提问四要素"回复；已 ANSWERED 的
线程负责回写 ADR 并标 CLOSED。ADR 变更用 Amendment，不许原地改写已接受结论；
不许改 docs/specs 与 docs/po。commit 带 Spec:/ADR:/Thread: trailers。
```

**给 Developer**

```text
你是 Developer。读 AGENTS.md、docs/collab/CONVENTIONS.md、当前 Spec、对应 ADR、
docs/implementation/ 偏差登记。每条 AC 至少一个测试且测试名含 TestAC<NN>；
实现与 ADR 不符时不许自己发明规则，开线程问。偏差必须登记 owner + expiry；
不许改 Spec/ADR。commit 带 Spec:/ADR:/AC: trailers。
```

**给 Reviewer**

```text
你是 Reviewer。读 AGENTS.md、docs/collab/CONVENTIONS.md、Spec、ADR。
逐条核对 AC 是否有测试（用 CONVENTIONS §4.1 的命令），检查偏差是否有 owner/expiry，
检查是否混提/越权，然后按红队思维直接问 Developer："我要是极端用户怎么打穿你这版？"
```

---

## 9. 反模式清单（出现即打回）

- 在聊天里定业务规则，Spec 不更新。
- 线程里写了裁决，Spec/ADR 没回写。
- 一个字段承载多个业务含义（已收货 / 已回库 / 可退款压成一个状态）。
- 用 `try-catch` 吞异常、用 `// TODO` 掩盖冲突。
- 偏差无 owner、无到期日。
- Spec 与代码同一笔提交。
- 用"技术上难"作为简化业务规则的理由。
