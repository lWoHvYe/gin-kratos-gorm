# REV-PAY-SETTLE-01-R0.2 — C-01..C-05 日期化 verdict

| 项 | 值 |
|---|---|
| review_scope | Phase 0-R/R0.2：C-01 SPEC-0001 v1.4；C-02 SPEC-0002 v1.10；C-03 ADR-0007 Rev 2.3；C-04 proto（RefundBatch/增量字段/release RPC）；C-05 migrations（order/0017-0021、fulfillment/0010-0011）与 R0.1 真实迁移证据（code `42a8db5`、evidence `aaef9f9`）。对照 AC-27/INV-45..50 台账。只读；不修改代码/契约；不批准 merge。 |
| **verdict date** | **2026-11-25**（所有 verdict 均本日；**不倒签 2026-11-24**） |
| R0.2 review input | 当前 `aaef9f9`；C-05 evidence 绑定 `42a8db5`；C-01 `44a9ae9`；C-02 `f121a1e`；C-03 `7a4f968`；C-04 `7421f4d`/当前 proto |
| Tier | 全部 **L2**（C-01…C-05；R0.1 migrations L2）；R0.2 为 L2 复审 |
| Actor / Reviewed-by | R0.2 Reviewer `reviewer-phase1`；契约批准人分别为 PO/Director（C-01/C-02）、Architect/Director（C-03/C-04）、Developer/QC（C-05）；**Actor ≠ Reviewed-by** |
| **overall verdict** | **FAIL（R0.2 有范围）**；非 Release PASS |
| release_status | **NOT RELEASE**；不批准合并 |

> **日期声明**：C-01 的原始批准是 2026-11-24（历史事实）；本 R0.2 对 C-01 的**复核结论日期为 2026-11-25**，不是把 2026-11-24 重新写成今天。C-02/C-03/C-04/C-05 的本轮 verdict 均为 2026-11-25。

---

## 1. 逐契约 verdict（2026-11-25）

| 契约 | 对象 / 状态 | **verdict（2026-11-25）** | 依据 |
|---|---|---|---|
| **C-01** | SPEC-0001 **v1.4**（status frozen；approved 2026-11-24；AC-27 EARS 基线迁移） | **PASS（contract）** | `docs/specs/0001-fulfillment-quantity-split.md:20-21`、`:695-696`、`:710-713`；AC-27 语义仅 EARS 迁移，不改变 INV-01/06/26/38。**AC-27 台账缺口见 B2**；PASS 不代表 AC-VERIFIED。 |
| **C-02** | SPEC-0002 **v1.10**（approved 2026-11-25；K-8；INV-45..50） | **PASS（contract）** | `docs/specs/0002-payment-settlement-admission.md:20`、`:108-134`、`:781`；v1.10 仅 K-8 门禁口径，不新增业务规则。**INV-45..50 evidence 缺口见 B3**。 |
| **C-03** | ADR-0007 **Rev 2.3**（accepted 2026-11-25；N5 修复） | **PASS** | `docs/adr/0007-payment-settlement-admission.md:3-5`、`:1513-1515`；Rev 2.3 只修 N5 approval-id/mode 配对 fail-closed，不改变业务规则。 |
| **C-04** | proto：`RefundBatch`/`GetRefundByKey`/T1.6 optional event fields；**release RPC** | **FAIL（2026-11-25）** | `api/proto/mall/v1/mall.proto:782-785` 有 `GetRefundByKey`/`RefundBatch`；T1.6 optional 字段；但 **C-04 release RPC 仍缺**（R0.4，`tasks.md:21`）；C-04 目前只有 2026-11-24 RefundBatch accepted，不能作为完整 C-04 PASS。**B1**。 |
| **C-05** | migrations order/0017-0021、fulfillment/0010-0011；R0.1 真实 apply | **PASS** | R0.1 `42a8db5`、evidence `aaef9f9`：MySQL 8.4.13 + `mall/migrate:dev` + 真实文件；0017-0021/0010-0011 apply；ledger sha256 全匹配；0018 全量重入/部分失败收敛；0011 `chk_switch_settled_expiry` 正反例；fail-closed 存量违规行；runner replay `already applied`。**证据充分**（见 §4）。 |

**R0.2 总体：FAIL**（C-01/C-02/C-03/C-05 PASS；C-04 FAIL；AC-27/INV-45..50 台账缺口未关闭）。

---

## 2. BLOCKER（3 条）

### B1 — C-04 release RPC 仍缺（R0.4）

- **事实**：`api/proto/mall/v1/mall.proto` 的 C-04 增量只有 `GetRefundByKey`（`:782`）和 `RefundBatch`（`:783-785`）；现有 `Inventory.Release`（`:242`）/`TccCancelConfirm`（`:254`）不是 C-04 新增 release RPC。`docs/lead/PAY-SETTLE-01/tasks.md:21` 明确 R0.4 仍待“C-04 release RPC”。
- **影响**：C-04 的 RefundBatch 部分 accepted（2026-11-24），但 **release RPC 未闭合**，C-04 不能给出完整 PASS；T1.4b2 P0 使用 release event 不改变该契约缺口。C-04 verdict **FAIL**；由 Architect/Dev 在 R0.4 补契约与实现/测试，不能由 Reviewer 倒签 2026-11-24。
- **不涉及**：C-05 migrations 独立 PASS；T1.4b2 event 使用不豁免 C-04 release RPC。

### B2 — AC-27 台账缺失（R0.3）

- **事实**：`docs/collab/ac-coverage.tsv` 共有 37 行，**没有 AC-27 行**；`docs/collab/ac-baseline.txt:27` 有 AC-27。C-01 v1.4 的 AC-27 只有正文/EARS 迁移，无 `test_type`/`evidence_path`/`status`。
- **影响**：C-01 **contract PASS**，但 AC-27 未进入唯一台账，不能 AC-VERIFIED；R0.2 不能声称 Phase 0-R 完成。R0.3 Owner=Reviewer，需为 AC-27 写覆盖/延期决策；不得用正文引用替代台账。

### B3 — INV-45..50 evidence/ledger 缺失（R0.3）

- **事实**：`docs/lead/PAY-SETTLE-01/evidence-matrix.md` 仍是 **v1.7**，无 INV-45..50 行；`ac-coverage.tsv` 是 AC 台账，本就没有 INV 行。SPEC-0002 v1.10 已定义 INV-45..50（`:129-134`），但无 **INV-45..50** 的 evidence/延期决策。
- **影响**：C-02 **contract PASS**，但 INV-45..50 的覆盖/延期决策缺失（R0.3 明确要求）；同时 R0.5 evidence-matrix 仍 v1.7，不能声称 Phase 0-R/R0.5 完成。R0.3 需补 INV-45..50 证据或明确延期；不能以 Spec 正文替代。

---

## 3. C-04 缺 release RPC 对 verdict 的影响

- C-04 的 **RefundBatch/GetRefundByKey** 已 accepted（2026-11-24）；本轮 **2026-11-25** 只能确认这两个 RPC 存在，不能确认 **release RPC**。
- **C-04 verdict = FAIL（2026-11-25）**，不是“C-04 accepted 所以 PASS”。R0.4 补 release RPC 后，Reviewer 按精确 base/head 重出 C-04 verdict；本报告不倒签、不预批。
- C-05 的 migration PASS 不依赖 release RPC；C-04 release RPC 缺失不阻塞 C-05 的迁移证据，但阻塞 Phase 0-R/C-04 完整收口。

---

## 4. C-05 真实迁移证据（R0.1）

- **绑定**：code `42a8db5`，evidence `aaef9f9`；evidence log 头固定 `HEAD 42a8db5`、`MYSQL_IMAGE mysql:8.4`、`MIGRATE_IMAGE mall/migrate:dev`、`WORKTREE clean`。
- **真实环境**：MySQL **8.4.13**，一次性容器；`mall/migrate:dev` runner 与 `deploy/k8s/images/migrate/run.sh` sha256 一致；挂载真实 `migrations/`，非内联 DDL。
- **apply**：order/0017-0021、fulfillment/0010-0011 全部 apply；ledger 的 sha256 逐一等于文件。
- **0018 重入**：全量重放 no-op；ALTER 后崩溃的 partial state 可收敛；单列部分状态只补缺失列；runner FAILED attempt 拒绝自动重放，人工删除 attempt 后同一文件收敛。
- **0011**：`chk_switch_settled_expiry` 对 settled NULL expires 拒绝（3819）、settled+expires/shadow NULL 接受；存量违规行 fail-closed 打印并保留，重签后同一文件成功；replay 不重复 CHECK。
- **审计**：持久环境 order/0018 未进入 `schema_migrations`，修改 0018 不会造成已应用环境 checksum 分叉；未审计环境会 fail-closed。
- **结论**：**C-05 证据充分，PASS**；不把 R0.1 test-only 的内联 schema 当真实迁移（R0.1 已用真实 runner 文件，非 T1.7 的内联 helper）。

---

## 5. 台账缺口（R0.3）

- **AC-27**：无 `ac-coverage.tsv` 行（B2）。
- **INV-45..50**：`evidence-matrix.md` v1.7 无行；无 INV 证据/延期决策（B3）。
- **AC-14/21/25/27/31/32 回归**：`ac-coverage.tsv` 无这些 AC 的回归行/decision；R0.3 任务要求覆盖/延期。
- **12 orphan**：R0.3 需去留决策；本报告不替 Reviewer 写台账。
- **AC-VERIFIED**：本报告**未标**任何 AC；C-01/C-02/C-05 的 PASS 仅合同/迁移证据，不是 AC-VERIFIED。

---

## 6. 声明

- 本报告未修改代码/契约/迁移；未跑新的迁移/测试；C-05 采用 R0.1 `aaef9f9` 绑定 `42a8db5` 的真实日志。
- **R0.2 总体 FAIL**：C-01/C-02/C-03/C-05 PASS（contract/evidence）；**C-04 FAIL**（release RPC 缺失）；AC-27/INV-45..50 台账缺口阻断 R0.3/Phase 0-R。
- **所有 verdict 日期 2026-11-25；未倒签 2026-11-24**。C-01 的 2026-11-24 仅作为原始批准历史引用。
- **不是 Release PASS；不批准 merge；不替代人类 merge 决策。**
- 报告：[REV-PAY-SETTLE-01-R0.2.md](docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.2.md)
