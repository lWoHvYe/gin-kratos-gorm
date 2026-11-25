# `docs/collab/qc-reports/` 文件索引

**本文件只是文件索引，不是状态台账。** 每份报告的结论（PASS / BLOCKED / CHANGES REQUIRED / READY FOR …）只写在报告自身，运行状态（裁决、冻结、PASS/FAIL、治理决策）以 `docs/review-tracking.md` 为唯一台账；本文件不复制、不推断、不背书任何结论。行号、base/head、审查范围以各报告头部为准。

索引对象：本目录下 44 份 `.md` 报告（不含本 README）。生成方式为扫描实际目录，未列入不存在的文件。

## 一、三套命名族

| 族 | 形态 | 语义 | 份数 |
|---|---|---|---|
| 1. 序号式 QC 报告 | `QC-nn-<对象>.md` | QC 按时间顺序编号的红队/复核报告，一文件一次审查 | 20（`QC-09`…`QC-28`） |
| 2. Reviewer 裁决 | `REV-<change_id>-<task>[-R[-n]].md` | Reviewer 的有范围 PASS/FAIL 裁决；`-R` 为修复轮、`-R2` 为修复轮的短复审 | 12（全部为 `PAY-SETTLE-01`） |
| 3. 主体锚定式单次审查 | `<SPEC|ADR|change_id>-QC-nn.md`、`<change_id>-Review-01.md` | 早期按被审对象锚定、从 `01` 起编号的单对象审查 | 12（`SPEC-0002`×8、`ADR-0007`×3、`PAY-SETTLE-01`×1） |

家族内部事实备注（非结论）：

- 家族 1 现存序号起点为 `QC-09`；`QC-01`…`QC-08` 不在本目录。
- `ADR-0007-QC-02.md` / `ADR-0007-QC-03.md` 的文件名按对象锚定，正文标题写作 `QC-ADR-02` / `QC-ADR-03`，同一文件两个称谓。
- 家族 3 的 `SPEC-0002-QC-01`…`-08` 对应 `SPEC-0002` 草案从 v0.1 到 v1.6 的连续收敛轮；`ADR-0007-QC-01`…`-03` 对应 `ADR-0007` 从 proposed 到 v0.3。

## 二、按 SPEC / ADR 分组

**SPEC-0002（支付结算与履约准入，含 SPEC-0001 联动）**

| 文件 | 审查对象 |
|---|---|
| `SPEC-0002-QC-01.md` … `SPEC-0002-QC-08.md` | SPEC-0002 草案 v0.1 → v1.6 逐版审查 |
| `QC-09-SPEC-0002-v1.8.md` | SPEC-0002 v1.8-draft-r2 / SPEC-0001 v1.4-draft |
| `QC-10-SPEC-0002-v1.8-final.md` | SPEC-0002 v1.8-draft-r3（FINAL CANDIDATE） |
| `QC-12-SPEC-0002-v1.8-ADR-Amd1.md` | SPEC-0002 v1.8-r4 + ADR-0007 Amd1 r2 联合 |
| `QC-13-SPEC-0002-v1.8-ADR-Amd1-final.md` | SPEC-0002 v1.8-r5 + ADR-0007 Amd1 r3 联合 |
| `QC-14-SPEC-0002-v1.8-ADR-Amd1-final.md` | SPEC-0002 v1.8-r5 + ADR-0007 Amd1 r4 联合 |
| `QC-17-zero-after-sale.md` | SPEC-0002 v1.9-draft 零元售后 + ADR Addendum r2 |
| `QC-18-zero-after-sale.md` | SPEC-0002 v1.9-r2 + 零元 Addendum r3 |

**ADR-0007（支付结算与履约准入）**

| 文件 | 审查对象 |
|---|---|
| `ADR-0007-QC-01.md` … `ADR-0007-QC-03.md` | ADR-0007 proposed 草案 → v0.2 → v0.3 |
| `QC-11-ADR-0007-Amd1.md` | ADR-0007 Amendment 1 Rev 0.1-draft |
| `QC-12` / `QC-13` / `QC-14` | 见上表（SPEC + ADR Amendment 联合复核） |

## 三、按任务号分组（`PAY-SETTLE-01`）

| 任务 | QC 报告 | Reviewer 裁决 |
|---|---|---|
| P0-A / P0-B（Phase 0 实现） | — | `PAY-SETTLE-01-Review-01.md` |
| C-04 / C-05（契约草案） | `QC-15-C04-C05.md`、`QC-16-C04-C05.md` | — |
| T1.2 / T1.2b / T1.3 | — | `REV-PAY-SETTLE-01-T1.2.md` |
| T1.2-R | `QC-19-T1.2-R.md`、`QC-20-T1.2-R.md` | `REV-PAY-SETTLE-01-T1.2-R.md`、`REV-PAY-SETTLE-01-T1.2-R2.md` |
| T1.4a（契约裁决） | `QC-21-T1.4a.md` | — |
| T1.4b1 | — | `REV-PAY-SETTLE-01-T1.4b1.md` |
| T1.4b1-R | `QC-22-T1.4b1-R.md`、`QC-23-T1.4b1-R.md`、`QC-24-T1.4b1-R.md` | `REV-PAY-SETTLE-01-T1.4b1-R.md`、`REV-PAY-SETTLE-01-T1.4b1-R2.md` |
| T1.4b1-R C-04（RefundBatch） | `QC-25-T1.4b1-R-C04.md`、`QC-26-T1.4b1-R-C04.md` | — |
| T1.4b2（design + a/b/b2 + P0） | `QC-27-T1.4b2.md` | `REV-PAY-SETTLE-01-T1.4b2.md`、`-T1.4b2-R.md`、`-T1.4b2-P0.md`、`-T1.4b2-P0-R2.md` |
| T1.6（Payment-Level Cancel） | `QC-28-T1.6.md` | `REV-PAY-SETTLE-01-T1.6.md`、`REV-PAY-SETTLE-01-T1.6-R.md` |

## 四、归属

- `docs/collab/qc-reports/**` 的目录所有者是 Quality Checker（`docs/collab/CONVENTIONS.md` §1）。
- `REV-*` 是由 Reviewer 出具的有范围裁决，与 Reviewer 协作产生/归档于本目录；部分报告（如 `QC-12`/`QC-13`/`QC-17`）为 SPEC + ADR 联合复核。**归属仍属原报告作者**（Actor/Reviewed-by 见各报告头部），QC 不因索引或归档而取得其内容的所有权，也不在本文件中改写其结论。
- 需要修改他角色已写内容时，走线程由所有者回写；本索引不构成任何授权或验收。
