# GOV-COST-01 Tasks

> **2026-11-25 决定：GOV-COST-01 撤回；门禁工具整体退役。** T-B*/T-C*/T-D* 全部 DEFERRED（门禁大幅放宽，不再实施 P0b/P1/P2）。下方状态保留为历史记录。

> Owner: Lead。状态：`TODO | WIP | BLOCKED | DONE`。每个任务必须有 AC 引用与证据路径。
> 计划依据：`plan.md`、SPEC-0003 v1.4（approved）、ADR-0008 Rev 1.1（approved）。

## Phase 0 — 契约（已完成）

| ID | 任务 | Owner | AC | 状态 | 证据 |
|---|---|---|---|---|---|
| T0.1 | SPEC-0003 v1.4 批准（含 QC-05 M-1…M-7） | PO + Director | — | DONE | `docs/specs/0003-…md`、QC-05 |
| T0.2 | ADR-0008 Rev 1.1 批准（QC-02 0/0/7） | Architect + Director | — | DONE | `docs/adr/0008-…md`、QC-02 |
| T0.3 | adr 回填 `[0008]` + 索引 | PO | — | DONE | spec §13/§14、README §4 |

## P0a — G-3 治理证据扩展 + 原子落库

| ID | 任务 | Owner | AC | 依赖 | 状态 | 证据 |
|---|---|---|---|---|---|---|
| T-A1 | G-3 读取 `test_type/evidence_path/status`；治理 AC 需 VERIFIED+签名+摘要，否则 `GOV_AC_EVIDENCE_MISSING` | Developer | AC-99 | Plan 批准 | DONE | `scripts/collab_gates.py` 单测（0d57398） |
| T-A2 | G-3 head 绑定（`git show`/`git ls-tree`），禁用工作区/未跟踪引用；`--range` 透传 | Developer | AC-71/72/102 | T-A1 | DONE | 单测 + 复现命令（0d57398） |
| T-A3 | `BASE` 投影运行：干净 worktree + `--range=<BASE^>..<BASE>` + scoped AC-01…68 + `g4_status=NOT-RUN(...)`；IV 记录 GV-S21 | Developer + IV | AC-106/112/107 | T-A2 | DONE | `evidence/bootstrap/T-A3-base.md`（0d57398） |
| T-A4 | 台账播种：baseline 到 AC-112；tsv 为 AC-69…112 建 `PENDING`（非 DEFERRED） | Reviewer | AC-95/99 | T-A1 | DONE | 台账 diff（0d57398） |
| T-A5 | 原子提交（spec + G-3 + 台账 + 测试 + 证据）带 trailer | Lead + Developer | AC-95/103 | T-A1…T-A4 | DONE | commit `0d57398` + `make gates-strict` exit 0 |

**P0a 前置**：bootstrap 人类批准行（GV-S20 `mode=bootstrap`，approver/date/expiry/scope）先于首个 P0a 提交；`BASE` 树不含 AC-69…112；`BASE^..BASE` 有 trailer；IV 已指定。

## P0b — 四闸门 / Release / 自保护

| ID | 任务 | Owner | AC | 状态 | 证据 |
|---|---|---|---|---|---|
| T-B1 | 实现 G-4（所有权/范围 + 到期 + 豁免 + bootstrap 有效期）与退出码统一 | Developer | AC-69/94/112 | TODO | 单测 |
| T-B2 | `--release` 模式与 `release_verdict`/`release_block_reason` | Developer | AC-70/110 | TODO | 单测 |
| T-B3 | 显式 range；默认范围降级提示；`RANGE_UNBOUND` | Developer | AC-71 | TODO | 单测 |
| T-B4 | AC-REF/AC-VERIFIED 措辞 | Developer | AC-72 | TODO | 输出快照 |
| T-B5 | G-2 同版本摘要 + `Non-normative` 收窄 + ADR `Revision` | Developer | AC-102 | TODO | 单测 |
| T-B6 | 自保护：基线版本 + IV diff + `GATE_NOT_RUN` | Developer + IV | AC-100/107 | TODO | GV-S21 + CI |
| T-B7 | 治理门禁独立 workflow + 三路径 + `AGENTS.md` 过渡控制 | Developer | AC-73/100 | TODO | workflow |
| T-B8 | P0b 出口 **review-scope** 运行（非 local） | Developer + IV | AC-100/107 | TODO | GV-S21 |

## P1 — 分级 / 联签 / 载体

| ID | 任务 | Owner | AC | 状态 | 证据 |
|---|---|---|---|---|---|
| T-C1 | Tier 注册表 + 总函数 + `--print-tier-rules` | Developer + Architect | AC-75…81/97/109 | TODO | 单测 |
| T-C2 | 治理 L2 路径 + MANAGED + `PC-*` per-change 规则 | Developer | AC-97/111/103 | TODO | 单测 |
| T-C3 | GV-S19 注册表 / GV-S20 审计行 / GV-S22 release scope | Lead + Developer | AC-95/104/110 | TODO | schema + 样例 |
| T-C4 | L1 联签 + `verdict_at` 顺序 | Developer | AC-82/84/101 | TODO | 单测 |
| T-C5 | GV-S9/S15/S17 载体与失败码 | Developer | AC-94/108/112 | TODO | 单测 |

## P2 — QC delta / Scaffold

| ID | 任务 | Owner | AC | 状态 | 证据 |
|---|---|---|---|---|---|
| T-D1 | QC delta/full 身份 + 未复审清单 + Release object set | Developer + Architect | AC-86…89/105/110 | TODO | 单测 |
| T-D2 | Scaffold 生成器 + 主权声明 | Lead + Developer | AC-90…92 | TODO | 生成样例 |
| T-D3 | 审计载体/追加格式 + `spec_version/adr_revision` | Developer | AC-95 | TODO | 单测 |

## 回归（业务基线）

| ID | 任务 | Owner | 状态 |
|---|---|---|---|
| T-R1 | AC-01…68 / INV-01…50 非回归：现有业务测试与基线不变 | Developer/Reviewer | TODO |
| T-R2 | 每次 P0a/P0b 提交后 `make gates-strict` 绿（P0a 时 Release 仍阻断，属预期） | Lead | TODO |
