# Specs 目录规范

本目录存放 Product Owner 产出的规格说明书（Spec）。**一份业务能力一个文件**，编号终身唯一。

入口：仓库根目录 [`Spec.md`](../../Spec.md) 只是索引，业务内容一律在本目录。

---

## 1. 命名规则（强制）

```text
NNNN-<domain>-<capability>.md
```

| 段 | 规则 | 示例 |
|---|---|---|
| `NNNN` | 四位序号，从 `0001` 起全局递增；**永不复用**，废弃/取代也保留编号 | `0001` |
| `<domain>` | 限定词表，见下表 | `fulfillment` |
| `<capability>` | kebab-case 英文短名，≤ 4 个词，描述"做什么" | `quantity-split` |

### 1.1 domain 词表（新增需在此登记）

`order`、`fulfillment`、`payment`、`aftersale`、`inventory`、`pricing`、`identity`、`ai`、`platform`

### 1.2 禁止事项

- 文件名**不得**出现版本号、日期、状态、作者：`Spec.md`、`Spec_v2_final.md`、`spec-20261122.md`、`拆单规格.md` 均为反例。
- 不得用中文文件名（跨平台与工具链风险）。
- 不得在一个文件里写两个互不相关的业务能力；跨域能力拆成多份并互相引用。
- 不得复用已废弃的编号。

---

## 2. 元数据（文首 YAML，必填）

```yaml
id: SPEC-0001                     # 与文件名前四位一致
title: 多仓按数量拆行、选仓优先级与承运商签收
status: approved                  # draft | in-review | approved | superseded
version: 1.0
owner: Product Owner
approved_by: 甲方
approved_at: 2026-11-22
domain: fulfillment
supersedes: []                    # 被本 Spec 取代的编号
superseded_by: null               # 本 Spec 被谁取代
adr: []                          # Architect 出稿后回填 ADR 编号
```

| 字段 | 说明 |
|---|---|
| `status` | `draft` 草稿；`in-review` 待甲方确认；`approved` 已批准可交接；`superseded` 已被取代 |
| `version` | 语义化主次版本；业务口径任何变更都要升版 |
| `supersedes` / `superseded_by` | 取代关系必须双向记录，禁止单向覆盖 |
| `adr` | 一个 Spec 可对应多个 ADR；回填后 Architect 与 Reviewer 据此追溯 |

---

## 3. 生命周期

```text
draft --提交--> in-review --甲方批准--> approved --被新版取代--> superseded
                     │                              │
                     └──打回修改（version+1）        └──口径变更（version+1，原地升版，不新建文件）
```

**硬规则**

1. 只有 `approved` 的 Spec 才能交给 Architect 出 ADR；`draft`/`in-review` 不得进入开发。
2. `approved` 之后，业务口径变更必须**原地升版**并更新 `version` 与变更记录，**不得新建文件**。
3. 只有"整块业务能力被另一份 Spec 取代"时才新建文件，并双向登记 `supersedes`/`superseded_by`。
4. 任何对已批准内容的修改都必须在「变更记录」中写明影响的不变量（INV）与验收项（AC），否则视为静默改写，Reviewer 可直接打回。
5. Spec 编号与 ADR 编号**相互独立**，不要为了对齐而跳号。

---

## 4. 索引（新增 Spec 必须在此登记）

| 编号 | 标题 | 状态 | 版本 | 关联 ADR |
|---|---|---|---|---|
| [SPEC-0001](0001-fulfillment-quantity-split.md) | 多仓按数量拆行、选仓优先级与承运商签收 | approved | 1.4 | [ADR-0005](../adr/0005-fulfillment-quantity-split.md) |
| [SPEC-0002](0002-payment-settlement-admission.md) | 支付结算与履约准入 | approved | 1.9 | [ADR-0007](../adr/0007-payment-settlement-admission.md) / [Amendment 1](../adr/0007-payment-settlement-admission-amd1.md) / [Zero after-sale addendum](../adr/0007-amd1-addendum-zero-after-sale.md) |
| [SPEC-0003](0003-platform-governance-cost-reduction.md) | 协作治理分级、门禁机器化与成本收敛 | approved | 1.4 | [ADR-0008](../adr/0008-governance-cost-reduction.md) |

---

## 5. 提交规则（批准即入库，保证改动可追踪）

1. **approved 立即提交**：Spec 一旦转为 `approved`，必须在同一轮工作内 commit，禁止长期留在工作区或只存在于本地未跟踪文件。
2. **一个 commit 一份 Spec**：不得把 Spec 与代码、ADR 或其他业务文档混提。唯一例外是本目录规范首次建立时，可与首份 Spec 同提。
3. **commit message 格式**：

   ```text
   docs(spec): SPEC-NNNN vX.Y <中文摘要>
   docs(spec): SPEC-NNNN vX.Y -> vX.Y+1 <变更摘要>   # 升版
   ```

   例：`docs(spec): SPEC-0001 v1.0 多仓按数量拆行与承运商签收`
4. **变更记录必须与 commit 一致**：commit 涉及的口径变化，必须已写进该 Spec 的「变更记录」并注明影响的 INV / AC。
5. **回滚以 commit 为单位**：Spec 的撤销/取代通过新 commit 表达，禁止 `git commit --amend` 已批准的历史。
6. **追溯方式**：`git log --follow --stat docs/specs/<文件>`；文件内**不写** commit hash（会过期），以 git 历史为准。

---

## 6. 交接链

```text
SPEC-NNNN（本目录，approved）
   └──> ADR-NNNN（docs/adr/）
          └──> 代码与测试
                 └──> Reviewer 对照 Spec 第 4/5/8/9 节审查
```

顺序不可颠倒；Developer 在 ADR 获批前不得开工。
