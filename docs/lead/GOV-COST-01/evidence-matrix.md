# GOV-COST-01 Evidence Matrix

> 本文件是 **Plan 视图**，不是唯一 AC 台账。唯一台账：`docs/collab/ac-coverage.tsv`（Owner=Reviewer）。AC-REF ≠ AC-VERIFIED。
> 状态：`PENDING`（未执行）| `REF`（仅引用）| `VERIFIED`（有执行证据 + Reviewer 签名 + 摘要）| `DEFERRED`（仅限有书面豁免的单条，禁止批量）。

## 1. 治理 AC（SPEC-0003）

| AC 组 | AC | test_type | evidence_path（计划） | status |
|---|---|---|---|---|
| 门禁执行器 | AC-69…74、99、100、102、107 | script/unit/static | `docs/lead/GOV-COST-01/evidence/gates/*.log` | PENDING |
| 分级/注册表 | AC-75…81、97、98、103、108、109、111 | unit/static | `docs/lead/GOV-COST-01/evidence/tier/*.log` | PENDING |
| 联签/委托 | AC-82…85、101 | unit/manual | `docs/lead/GOV-COST-01/evidence/signoff/*.md` | PENDING |
| QC delta/full | AC-86…89、105、110 | manual/static | `docs/collab/qc-reports/*-QC-*.md`、`docs/lead/GOV-COST-01/evidence/qc/*` | PENDING |
| Scaffold | AC-90…92 | script/static | `docs/lead/GOV-COST-01/evidence/scaffold/*` | PENDING |
| 异常/证据/bootstrap | AC-93…96、104、106、112 | script/manual | `docs/lead/GOV-COST-01/evidence/bootstrap/*` | PENDING |

## 2. 业务基线回归（不得因本变更下降）

| AC/INV | test_type | evidence_path | status |
|---|---|---|---|
| AC-01…68 / INV-01…50 | existing suite（`make test`/`make verify`） | 现有 CI 日志 + 本 change 的 base/head 回归记录 | PENDING |

## 3. 阶段 ↔ 证据门

| 阶段 | 必需证据 |
|---|---|
| P0a | `BASE` 投影运行输出（`--range=<BASE^>..<BASE>`）+ GV-S21（IV、`g4_status`、scoped AC 集）+ 台账播种 diff + `make gates-strict` |
| P0b | G-4/Release/自保护单测 + review-scope GV-S21 + 独立 workflow 运行 + G-2 摘要单测 |
| P1 | tier 总函数与 `PC-*` 单测 + 联签顺序单测 + GV-S20 载体样例 |
| P2 | QC delta/full 单测 + `release_scope` 校验 + scaffold 生成样例 |
| Release | 人类 `release_verdict` + 精确 base/head + 受影响基线非回归 + 全量 QC |

## 4. 未覆盖/已知限制

- 本矩阵为计划视图，`PENDING` 不代表任何验收。
- untracked 稿的 sha1 仅评审时点；canonical 绑定为 P0a 提交中的 blob。
- 未在 GitHub runner 实跑 CI；未做生产/预发部署。
