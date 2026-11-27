# Architecture review tracking

This is the canonical running status ledger. Update it every round with evidence
and remaining gaps. `docs/handoff.md` was retired on 2026-11-27 (archived); this
ledger is the only running-status source.

> **唯一运行状态台账（2026-11-25 人类批准）**：裁决、冻结、PASS/FAIL、治理决策以本文件为准；`tasks.md`、`plan.md`、`handoff.md`（已停用）、协作线程均为视图，冲突时以本文件为准。

> 2026-11-27 D-2 归档：旧路线图、GOV-COST-01、collab-gates、pair/线程相关历史与已被取代的 11-23/11-24 条目已移至 `docs/review-tracking-archive.md`；主文件不再重复。只移动，不改内容；不重排。

Source: 2026-11-20 handoff plus follow-up implementation. `done` means the code
and the stated verification exist; `partial` means a safe subset shipped and a
named gap remains. This file was rewritten after a 2026-11-21 audit because the
old table still described P2#10 as rejected and Phase 7 as not started.

### 2026-11-27 过程纠正: PAY-SETTLE-01 T5 证据的 Review trailer

- **事实**：`0416bc6` 在 commit 时尚未复审（后来 `a9758c7` 才由 reviewer-syntax 补审 PASS）；
  `ec36339`、`9a80fd2` **从未复审**，却写了 `Reviewed-by: reviewer-syntax`。这违反 reviewer 独立、
  AC/任务状态归 reviewer 的规则，不得据此宣称 PASS。
- **处置**：`T5.2` 从 `[x]` 退回 `[ ]`（作者自跑证据，待复审）；`T5.3` 保持 `[ ]`；
  T5.2/T5.3 evidence 文档标记“author-run only, reviewer pending”；不伪造新的 Reviewed-by。
- **不改写历史 commit**（未 push）；由人类决定是否 rebase/重写这三个 commit 的 trailer，或补审后
  在台账记录“补审有效”。
- 继续 T5 前，T5.2/T5.3 证据必须经独立 reviewer 复审；不得用“goal 进度”替代审查。

### 2026-11-24 Phase 1 parallel authorization

Director authorized T1.2/T1.3 (TCC batch + permit Tx1 + resume) to proceed in
parallel with the AC-40 zero after-sales amendment. This is a scoped waiver of
the "any contract gap stops implementation" rule for this isolated gap. T1.0
Order-side work is complete; AC-40 end-to-end remains blocked until the PO/
Architect amendment is approved. No AC-VERIFIED or Release PASS.

### 2026-11-25 T1.4 re-decomposition (Option A accepted)

Director accepted re-decomposition instead of patching T1.4b2. The draft is
`docs/lead/PAY-SETTLE-01/decomposition-review.md`: WP-R1 Payment refund,
WP-R2 Order refund ledger, WP-R3 Zero, WP-I1 group reservation identity,
WP-I2 release, WP-I3 restock, WP-B1/B2 backfill, WP-E1..E4 compatibility,
WP-V1 evidence. Each WP owns one invariant, one branch, one scoped Reviewer.
T1.4b2 is frozen until WP-I1 fixes reservation identity. No code yet.

### 2026-11-25 T1.5 re-scope: TCC no longer uses refund_requested

- Verified `TccCancelMarkPaid` no longer emits `shop.order.refund_requested.v1`
  (T1.4 admission/batch path uses `refund_applications` + `RefundBatch`).
- The only remaining producer is `refundCancelledAfterConfirm` (payment-level
  cancel after inventory confirm). Making the consumer observe-only now would
  drop that refund. Therefore T1.5 is closed as "TCC bypass removed"; the
  remaining consumer stays until T1.6 adds the durable payment-level cancel
  path (`pay:<payment_no>:cancel`) and then becomes observe-only.
- No code change in T1.5.

### 2026-11-25 P0 WIP merged to main (human approved)

- Director approved merge; merge commit `d7c2f30` on `main` (18 commits ahead
  of origin, not pushed).
- Main now contains the PAY-SETTLE-01 P0 WIP: settlement/refund, RefundBatch,
  zero after-sale, single-group release, payment-level cancel, reconcile.
- **Not a Release PASS**: T1.7/T5 evidence, T2/T3/T4, AC-VERIFIED, release
  gates remain open. Multi-group physical release is deferred (reserved).
- `inventory.confirm.requested` warehouse_id remains a separate task.

### 2026-11-25 T1.6-R PASS (scoped)

- Reviewer PASS for `8dc1766..20df5b7`: warehouse_id, PENDING authoritative
  snapshot, durable compensation, legacy net. T1.6 scoped close; no merge.
- Residual: `inventory.confirm.requested` lacks warehouse_id (paid-confirm
  path); separate task, not T1.6 blocker.

### 2026-11-25 T1.6 FAIL — warehouse/captured/compensation

- B1: `order.cancelled`/compensation payload lacks `warehouse_id`; multi-warehouse
  W2 reservation is not released/restocked.
- B2: PENDING_PAYMENT with request PaidCent=0 does not read Payment snapshot;
  captured funds can be cancelled without a payment cancel command.
- B3: inventory nil/queue failure is silent; legacy cancel-after-confirm uses
  PaidCent, not captured-refunded net.
- T1.6 frozen; T1.6-R required; no merge.

### 2026-11-25 T1.4b2a P0-R2 PASS (scoped)

- Reviewer PASS for `60130e1..1de09e3`: group_count=1, single-group release,
  multi-group reserved/P1, no scan/backfill. T1.4b2a scoped close; T1.4 still
  WIP. No merge/Release; T1.5/T1.6 next with Amendments.

### 2026-11-25 P0 simplification: single-group physical release only (Tier L2)

Director accepted: for P0, only single-group orders emit an immediate full
Inventory release. Multi-group orders complete refund admission but do not emit
a physical release; stock stays reserved and is deferred to P1/manual. This
removes the TCC group REFUNDED projection, deferred scan, watermark/cursor,
manual queue and production backfill. No AC-VERIFIED/Release PASS; no merge.

### 2026-11-25 T1.4b2-R FAIL — deferred release still too complex

- B1: TCC groups stay REFUNDING; no TCC REFUNDED projection, so multi-group
  last-active never emits. B2: scan has no watermark/cursor; starves and would
  backfill production. B3: shared POST/unknown group has no durable manual
  marker; unknown group can fail-open release.
- Old group/reservation B1 and system_shortage B2 are closed.
- T1.4b2 frozen. Proposed simplification: P0 releases only single-group orders;
  multi-group physical release deferred to P1/manual. Awaiting human decision.

### 2026-11-25 T1.4b2 FAIL — group/reservation identity

- B1 [ARCHITECTURAL_FLAW]: release event carries group quantity, but Inventory
  reservations are order/sku/warehouse totals and Release/TccCancelConfirm
  require the full reservation. A partial group refund cannot release a shared
  reservation; DLQ, INV-44 not executed.
- B2: SYSTEM shortage source is sent as `aftersale`, not `system_shortage`.
- B3: existing PRE admissions/S19 have no release/backfill path.
- T1.4b2 frozen; Architect ruling required before code; no merge.

### 2026-11-25 T1.4b1-R2 PASS (scoped)

- B2 persistent reconcile worker/order projection and B3 selfcheck closed;
  Reviewer PASS for `198c623..6ac218e`.
- Continue WIP; no merge. T1.4b2 next: INV-44 release/restock + zero
  AfterSales task; C-04 release event no separate human approval.

### 2026-11-25 T1.4b1-R FAIL — B2/B3

- B1 external refund/atomicity closed; B2 lazy reconcile is not persistent and
  does not update order amount projection; B3 Plan/selfcheck not bound to
  `198c623`.
- T1.4b1-R frozen; T1.4b1-R2 required: persistent reconcile worker + order
  projection in same tx + canonical selfcheck/Plan status. No merge.

### 2026-11-24 C-04 RefundBatch accepted

Director accepted the C-04 `RefundBatch` contract (expected_total/new-only
remaining, first-reply idempotency, batch ledger schema, error mapping). Payment
WIP `fc295e5` remains unapproved; Order preflight and Reviewer are next. No
merge/AC-VERIFIED.

### 2026-11-24 T1.4b1-R scope approved

Director approved the atomic `Payment.RefundBatch` + T1.4b1-R scope. Contract
and plan still require Architect/QC review; no code until then.

### 2026-11-24 T1.4b1 Reviewer FAIL — stop-the-line

- Scope `dff267d..1f4f2d1`; FAIL. B1: pre-payment does not re-read Payment
  remaining vs batch outstanding, so an external partial refund can allow G1
  payment then G2 rejection (partial payment). Must preflight all groups before
  any Payment.Refund; distinguish own accepted refunds via GetRefundByKey from
  external refunds; mismatch -> MANUAL_HOLD with zero payments.
- T1.4b1 frozen; no merge; T1.4b2 not started.
- Requires Architect/Plan ruling and Reviewer re-review.

### 2026-11-24 T1.4a Addendum approved

Director approved the T1.4a contract rulings Addendum (R-A INV-44 owner,
R-B POST commitment timing, R-C zero cancel). T1.4a code `6be5c5a` remains
WIP; T1.4b/T3 and Reviewer are still required. No AC-VERIFIED/merge.

### 2026-11-24 T1.2-R2 PASS (scoped)

- G1/g1/g2/g3 closed; Reviewer PASS for `e7e29e5..72b984b`.
- T1.2 scoped close; still WIP, no merge, no T1.4 yet.
- No AC-VERIFIED; AC-51/AC-65 remain DEFERRED.

### 2026-11-24 T1.2-R Reviewer PASS (scoped)

- Scope `a3f080f..e7e29e5`; PASS for T1.2-R B1/B2/M1/M2/m1. Not merge/Release.
- G1 remains MAJOR: legacy orders with no fulfillment group and status
  SHIPPED/COMPLETED can be misclassified as not shipped when ListShipments has
  no facts; fix before T1.4. g1 mixed composition/g2 no-active dependency are
  minor follow-ups.
- T1.2 is still WIP; T1.4/T1.6/T1.7/T5 not done.

### 2026-11-24 T1.2-R approved

Director approved the T1.2-R remediation plan (B1/B2/M1/M2/m1); implementation
of R-1..R-5 is authorized on the WIP branch. T1.2 remains unmerged; Reviewer
re-review and human merge decision are still required.

### 2026-11-24 T1.2/T1.2b Reviewer FAIL — stop-the-line

- Review scope `998e896..4e5d07f`; verdict FAIL:
  B1 real lifecycle states (ALLOCATED/SHIPPED/COMPLETED) are silently ignored
  by TCC cancel (no batch/permit/refund); B2 missing canonical Developer
  self-check/test-scope artifact; M1 historical commitment composition can
  block whole TCC; M2 no-active-group case still creates FROZEN batch;
  M3 Plan approval state stale.
- `PAY-SETTLE-01` is frozen: no further managed code, no merge, no T1.4.
- Required: T1.2 remediation plan, regression tests for real lifecycle states,
  canonical self-check, Reviewer re-review, then human merge decision.
- Zero after-sale amendment remains approved; T1.2 code is not approved.

### 2026-11-25 human decisions: WIP continuation and amendment waiver

- T1.4b1-R2: if Reviewer PASS, continue on the WIP branch; do not merge main.
- T1.4b2 C-04 release event: no separate human approval required.
- T1.5/T1.6: if they touch the `order.refund_requested` consumer or
  payment-level refund semantics, issue an Amendment first.
- Merge after P0 closure still requires explicit human approval.
- **Temporary waiver (scoped)**: during the PAY-SETTLE-01 WIP, ADR/SPEC
  Amendments do not require separate human approval. QC/Architect review still
  applies; merge, Release PASS, and AC-VERIFIED are NOT waived. Scope ends at
  P0 closure/merge decision (or 2026-12-31, whichever is earlier). Recorded per
  constitution Article Governance 6.

### 2026-11-25 R0.4b/c WIP (not approved)

- `90b502c`: Fulfillment startup switch gate + `order/0022` checks. Addendum is
  DRAFT; no QC/Architect/human/Reviewer. Shadow runtime rejected until T4.4;
  range deploy wiring and `verify-r01-migrations.sh` 0022 entry remain open.
- R0.3 still FAIL: AC-27 AC-REF, INV-45..50 DEFERRED/description gaps,
  regression AC-14/21/25/27/31/32 not run, 12 orphan open.

### 2026-11-25 R0.3 progress: AC-27 branches, regression, Architect ruling

- Developer `fe1600b` + selfcheck `34d0466`: AC-27 EARS-2
  (`REFUNDED`+`REVOKED` via real row read-back), `CANCELLED` group, §7.5
  all-refunded-unshipped, post-ship `REFUNDING/REFUNDED` reverse — PASS.
  Regression AC-14/21/25/27/31/32 real MySQL PASS (no SKIP); T1.7 + zero
  evidence paths verified. No AC-VERIFIED.
- **3 branches BLOCKED** with time-sequence counterexamples: waybill
  `CANCELLED` absent from Order read model; refund release does not write
  `reservation_state`; pre-ship `REFUNDING` closure semantics. Developer did
  not add `if-else` or change production semantics.
- Lead withdrew the unsupported "refunding must not project CANCELLED" line
  (not in AC-27); recorded in `tasks.md`.
- Architect DRAFT `111516f` (R-1..R-5): closure predicate =
  Order-owned `order_refund_admissions.result=PRE_SHIP_REVOKED` (not
  `refund_state`); R-2 requires re-aggregating order lifecycle inside the
  existing `aftersale.completed` projection (single tx, CAS, outbox unchanged);
  R-3 forbids Order writing `reservation_state` and records the multi-group
  release gap (`refund_release.go:182-186`, P1/manual); R-4 keeps current
  pre-ship `REFUNDING` closure; R-5 is SPEC §7.5 drift for PO next version.
  Pending QC; no code until accepted.

### 2026-11-26 GOV-SDD-HYGIENE-01: P1 governance text cleanup (L2 light path)

- **Human decision (2026-11-26, Director)**: approved the P1 cleanup **without**
  a Spec Kit path-boundary declaration. Project-structure alignment with Spec
  Kit is a separate later evaluation (`SDD-STRUCT-ALIGN-01`, not started).
- **Change scope**: `AGENTS.md:15` (§0.5) and `AGENTS.md:77` (§2 item 4) retire
  live pair-thread wording (Lead routing + historical archive);
  `docs/collab/CONVENTIONS.md` removes the §4 original protocol summary,
  replaces the G-1 "thread exists" check with Tier-applicable §3 fields, and
  removes the §7.2 executor-limitation history. No new rule/gate/state; no Spec
  Kit boundary text.
- **Change input**: base `472e64f`; head = reviewed working tree; allowed_paths:
  `AGENTS.md`, `docs/collab/CONVENTIONS.md`,
  `docs/collab/qc-reports/QC-GOV-SDD-HYGIENE-01.md`,
  `docs/collab/qc-reports/REV-GOV-SDD-HYGIENE-01.md`, `docs/review-tracking.md`;
  forbidden: code/contracts/migrations/`scripts/**`/CI/`.specify/**`/
  `docs/specs/**`; tests: docs-only, none claimed.
- **Benefit (grep-checkable)**: `线程只承载` / `线程是否存在` live checks gone;
  §4 protocol summary and §7.2 limitation list gone; G-1 references §3 fields.
- **Stop condition**: only the two governance files in this increment; if the
  cleanup needs constitution/ADR/scripts/CI or a Spec Kit boundary, stop and
  split. Revert if the next governance review finds the removed history still
  needed or new ambiguity appears.
- **Delta audit (constitution :96, 2026-11-26)**: `AGENTS.md` blob `e7759cd`,
  `CONVENTIONS.md` v3 blob `d465d50`; 5 hunks; all roles must reread both;
  `prompt/**` unchanged; no MEMORY running-state changes. Known MINOR accepted:
  constitution `:96/:75` remains broader (delta audit may still cite threads);
  future PATCH, not a blocker.
- **Actors**: Architect `arch-gov`, QC `qc-gov` (READY, 0 BLOCKER/0 MAJOR),
  Reviewer `reviewer-gov` (PASS, `REV-GOV-SDD-HYGIENE-01`, base `472e64f`,
  5 hunks, `NOT RELEASE`); human approval 2026-11-26 (Director).
- **Tier**: L2 (governance text is always L2, `AGENTS.md:85`; only two
  governance files → light path, `AGENTS.md:84`/constitution `:77`/
  `CONVENTIONS.md:94`). `baseline_impact`: 不涉及 INV (no
  behavior/contract/status change).

### 2026-11-26 GOV-SDD-CLOSURE-01: PO approval write-back (L2 light path)

- **Human decision (2026-11-26)**: approved the one-line PO rule. The Spec Kit
  draft clause was removed after QC C-1 to respect the 2026-11-26 "no Spec Kit
  path-boundary" decision.
- **Change scope**: `prompt/Product-Owner.md:18` only: on `SPEC_APPROVED`, sync
  `Spec.md` + `docs/specs/README.md` in the same round; new ACs are EARS-
  rewritten, numbered, and handed to Reviewer for `ac-baseline.txt` +
  `ac-coverage.tsv` (ledger owner remains Reviewer).
- **Change input**: base `f1946e74`; allowed_paths: `prompt/Product-Owner.md`,
  `docs/collab/qc-reports/QC-GOV-SDD-CLOSURE-01.md`,
  `docs/collab/qc-reports/REV-GOV-SDD-CLOSURE-01.md`, `docs/review-tracking.md`;
  forbidden: code/contracts/migrations/`scripts/**`/CI/`.specify/**`/
  `docs/specs/**`; tests: docs-only, none claimed.
- **Benefit (grep/check)**: closes the index-drift recurrence point; new AC
  registration now names both ledgers; no new tool/gate/state.
- **Stop condition**: only this prompt line; if the rule needs a Spec Kit path
  or toolchain change, stop and split. Revert if the next two approvals show no
  improvement in index/AC registration.
- **Delta audit (constitution :96, 2026-11-26)**: `prompt/**` changed; PO must
  reread its prompt; Lead/QC/Reviewer aware; no MEMORY running-state changes.
- **Actors**: Architect `arch-closure`; QC `qc-closure` (READY, 0 BLOCKER/0
  MAJOR, 5 MINOR); Reviewer `reviewer-closure` PASS (`REV-GOV-SDD-CLOSURE-01`;
  base `f1946e74`; prompt blob `453eb3d`; 1-line diff; `NOT RELEASE`). Human
  approval 2026-11-26 covers the reviewed blob.
- **Reviewer MINOR (non-blocking)**: A-3 numbering continuation, B-1 explicit
  Gherkin ban, D-2 Spec YAML/change log, F-1 second-source pointer, R-1 approval
  identity/blob — accepted as TODOs for a later increment.
- **Tier**: L2 (governance text light path; `prompt/**`). `baseline_impact`:
  不涉及 INV (no behavior/contract/status change).

### 2026-11-26 R0.4e-R2 PASS (scoped): F-1/F-2 closed

- Reviewer `49d030b` (base `da0cdd1`, head `4549ccb`): **R0.4e-R2 PASS
  (scoped)**. F-1 closed with real MySQL 8.4 `ADD CHECK` evidence
  (`INPLACE/LOCK=NONE` rejected, COPY + `SHARED_NO_WRITE` MDL 43.7s/16.0s,
  write blocking, `DROP CHECK` online); F-2 closed with branch A
  (0022 byte-identical `90b502c`, 0023 `_v2` add-then-drop, fresh
  `0017→0022→0023`, ledger checksums, staging/prod unreachable -> branch A).
- Exact HEAD tests: `TestR04Checks` PASS 4.452s, `-race` PASS 7.232s,
  `verify-r01-migrations.sh` EXIT=0.
- Residual MINOR (not blockers): F-1 raw log header is `da0cdd1`; 0022's
  frozen header still contains the superseded "re-run safe" sentence;
  Lead Plan/evidence matrix references were updated to 0021/0022/0023.
- **NOT RELEASE**; R0.4 scoped PASS only. AC-27 still AC-REF; AC-50-FIX-01
  blocked.

### 2026-11-26 governance delta audit: GOV-SDD-HYGIENE-01

- `AGENTS.md` §0.5: cross-role info only via Lead; review-tracking only
  structured decisions/canonical pointers. §2.4: thread CLOSED marked
  historical. `CONVENTIONS.md` §4 protocol summary removed; §7.1 G-1 now
  checks Tier-applicable §3 fields.
- No role/Tier/phase change; `PAY-AC50-REVIEW-01` Plan/Tasks unaffected.
  SDD structural alignment deferred to `SDD-STRUCT-ALIGN-01`.
- All roles re-read AGENTS/CONVENTIONS at next step; no old-summary reliance.

### 2026-11-26 integrity: unowned housekeeping edits reverted

- At 21:56 (after the T0 verdict commit), five files had uncommitted edits
  outside the T0 scope and not attributable to the T0 verdict:
  `Spec.md`, `docs/collab/ac-baseline.txt`, `docs/collab/ac-coverage.tsv`,
  `docs/specs/0001-fulfillment-quantity-split.md`, `docs/specs/README.md`.
- The diff (74 lines) was saved to `.tmp/unexpected-housekeeping-2026-11-26.patch`
  and the files were reverted to HEAD to keep the T0 checkpoint exact.
- Owner routing: PO must apply the SPEC-0001 v1.5 / SPEC-0002 v1.11 index
  sync (`Spec.md`, `docs/specs/README.md`, the `未 commit` note in SPEC-0001);
  Reviewer must add `ac-baseline` AC-69…72 and the AC-72 three-part rows in
  **T7**, not during T0.
- T0 checkpoint remains `e374384` (code) / `c796580` (selfcheck) / `84f5acd`
  (verdict); no status change.

### 2026-11-27 人类决定: PAY-SETTLE-01 T2.5/T2.6/T2.7 - 回填工具 + seq 取号 + 测试

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（实现 PASS）;
  `级别: 实现变更`; `阶段: 合入前`.
- **T2.5**：`cmd/order-legacy-backfill` + `legacy_backfill.go`；默认 dry-run，`-apply` 唯一写开关；
  只选 `PAID AND settlement_payment_no=''`；dry-run 分类 unique/zero/ambiguous，apply 复用
  `ensureLegacyResolution`；重入不重复退款/队列；部分失败报错 exit 1。
- **T2.6**：`nextLegacyRefundSeq` + `legacyRefundID`；人类裁决 A=独立短事务提交（ADR-0010），
  调用方持 orders 行锁；rollback 不复用，允许 seq gap；20 并发唯一。
- **T2.7**：`TestLegacy*` 全量 PASS（含 dry-run/apply/幂等/并发/rollback/部分失败）。
- 证据：`go test ./internal/services/order -run 'TestLegacy'` → ok 24.361s；
  `go build ./cmd/order-legacy-backfill`、`go vet` exit 0；边界 diff 0。
- AC-60/AC-63 **仍未验证**（T2.5/T2.6/T2.7 为子集；T2.8/T3/T5 未做）；`NOT RELEASE`；
  合入由人类执行。本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T3.1/T3.3 - 不变量验证；T3 缺口审计

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（SCOPED-PASS）;
  `级别: 实现变更`; `阶段: 合入前`.
- **T3.1**：现有 6 处 permit 写路径无 `REVOKED→NONE/GRANTED`；新增
  `TestRevokedPermitCannotBeRestored`（AdmitRefund/BlockShipmentForRefund/AuthorizeShipment
  后仍 REVOKED；AuthorizeShipment 断言 revoked 分支）。
- **T3.3**：现有 `AdmitRefund` 在写 admission/commitment/outbox 前完成 over-amount/partial
  校验；新增 `TestRejectedAdmissionWritesNoCommitment`（零行、permit 仍 NONE）。
- **T3.2/T3.4/T3.5/T3.6 缺口**：T3.4 proto/`0018` 列已存在但 `AdmitRefund` 未使用
  `lines/responsibility/freight_refundable_cent`；T3.2 无 `RELEASED` 写路径/ release 命令；
  T3.6 未接线；均需 Plan/人类批准。详见 `docs/lead/PAY-SETTLE-01/t3-audit.md`。
- 证据：`go test ./internal/services/order -run 'TestRevokedPermitCannotBeRestored|TestRejectedAdmissionWritesNoCommitment'`
  → PASS；无代码/契约/迁移改动；AC-60/63 仍未验证；`NOT RELEASE`。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T3.4/T3.5 Phase 1 - 行组成 + commitment lines

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（SCOPED-PASS）;
  `级别: 实现变更`; `阶段: 合入前`; base `0740286`.
- **范围**：`AdmitRefund` 读取 `lines`，行剩余 = `line_paid - RESERVED/SETTLED commitment lines`
  （S14/S15 由 commitments 表达）；`Σ goods + freight = amount`；freight ≤ 组剩余；known=0 拒绝行级；
  header + `order_refund_commitment_lines` 落库并回读校验；无 lines 的旧 amount-only 路径不变。
- **测试**：正常落库/幂等、行超限、Σ 不匹配、known=0 拒绝、正运费 line 0、SETTLED 行剩余；
  `TestAdmitRefund|TestTccAdmission|TestLegacy` full **51.969s PASS**（reviewer 复跑）。
- **未做**：responsibility/reason_code 封闭枚举、168h 运费挂起、T3.6 AfterSales 接线、
  post-departure partial 强制 lines、T3.2 release。T3.4/T3.5 仍 `[ ]`。
- **NB**：quantity=0 的部分金额语义需产品/架构确认（NB-7）；known=1 无 lines 的 amount-only 不发现（NB-5）；
  并发/known=0 amount-only/mixed 测试缺口。AC-60/63 仍未验证；`NOT RELEASE`。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T3.5 Phase 2 - responsibility/reason + quantity=0

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（SCOPED-PASS）;
  `级别: 实现变更`; `阶段: 合入前`; base `5db0f0b`.
- **共享枚举**：新增 `internal/platform/refundpolicy`，AfterSales 的 party 常量改为 alias、
  `validParty/validReason` 改共享包；行为等价。
- **Order 校验**：空 responsibility 兼容旧调用；非空必须合法且 reason 配对；reason 无 responsibility 拒绝；
  `UNDETERMINED` + `freight_refundable_cent>0` 拒绝（商品照退、运费由 AfterSales hold）；
  `quantity=0` 表示部分金额、不消耗整件，`quantity>0` 仍校验数量剩余。
- **测试**：非法 party/跨 party/orphan reason/UNDETERMINED 运费/正运费通过；quantity=0 落库；
  quantity 超退；full `TestAdmitRefund|TestTccAdmission|TestLegacy` **44.274s PASS**。
- **未做**：T3.6 AfterSales 实际传 lines/responsibility/freight；168h hold worker；AC-60/63 仍未验证。
- NB-1/NB-5 测试已在 reviewer 之后补强（仅测试）；NB-2 产品边界仍建议书面确认；`NOT RELEASE`。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T3.6 - AfterSales 退款行/责任映射

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（SCOPED-PASS）;
  `级别: 实现变更`; `阶段: 合入前`; base `07291e6`.
- **映射**：`processRefundTask` 用 `after_sale_items.line_no/pending_amount_cent` 组装
  `RefundLine`；`freight = effective_amount - Σ goods`；传 `responsibility/reason`；
  负数 pending fail-closed；responsibility/reason 归一化；`UNDETERMINED + freight>0` 拒绝。
- **兼容**：pending 全 0 的 legacy 行保持 amount-only；zero 路径未改；shortage/TCC 回归 PASS。
- **测试**：`TestBuildRefundAdmissionRequest*`（含 lower-case 归一化）；AfterSales 定向
  `TestAC14|TestAC30|TestAC33|TestFreightResponsibilityDisabled` **2.564s PASS**；`go vet` 0。
- **未做/NB**：纯运费（所有行 goods=0）仍需产品确认（NB-1）；缺真实 `processRefundTask`
  断言 `AdmitRefundRequest` 的集成测试（NB-3）；T3.4/T3.5 仍 `[ ]`；AC-60/63 仍未验证。
- **说明**：工作树 `.dsh/.specify` 的 Spec Kit 1.4.5 自动升级未纳入本提交。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T3.2 - 人工释放 ops CLI

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（SCOPED-PASS）;
  `级别: 实现变更`; `阶段: 合入前`; base `4f7793c`.
- **服务**：`ReleaseRefundCommitment` 只 `RESERVED→RELEASED`；D-24 字段/阈值/双人复核/过期/
  金额一致；reason 封闭枚举 + `refund_applications` 状态；写 `refund_commitment_releases` +
  CAS；同 release_id 幂等（含 authorized_at/expires_at/reviewed_at），同 refund/authorization
  冲突；`DryRun` 零写入；不碰 permit/订单/组/Payment。
- **CLI**：`cmd/refund-commitment-release`，默认 dry-run，`-apply` 唯一写开关；exit 2 配置/1 业务。
- **测试**：9 个 `TestReleaseCommitment*` PASS（成功+幂等+reviewed_at、SETTLED、双人复核、
  过期/金额/order/group、trigger 变体、授权复用、dry-run、并发）；full 41.502s PASS；vet 0。
- **AC-62**：有引用 → **已验证**（reviewer 更新 `docs/specs/0002` 验收表）；AC-60/63 仍未验证。
- **NB**：仅授权角色依赖 ops 主机/DB 凭证/授权流程（部署控制，人类批准 A）；Payment 对账窗口
  由 ops runbook 在 `-apply` 前核对；`reviewed_at` 幂等已补；permit/订单/组不变与 trigger 变体
  已补测试。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T3 收口 - T3.4/T3.5/T3.7

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（PASS）;
  `级别: 实现变更`; `阶段: 合入前`; base `4a743ee`.
- **新增测试**（仅测试）：并发同一行只一个 admission/commitment/line；prior known=0 组仍允许
  整组 amount-only；mixed known=0+1 带 lines fail-closed。
- **已有证据**：Order full `TestAdmitRefund|TestTccAdmission|TestLegacy|TestReleaseCommitment`
  **49.365s PASS**；AfterSales `TestAC14|TestAC31|TestAC32|TestAC33|TestFreightResponsibilityDisabled`
  **1.965s PASS**（含 168h hold timeout）；T3.6 映射/T3.2 release 已 PASS。
- **任务**：T3.1/T3.2/T3.3/T3.4/T3.5/T3.6/T3.7 全部 `[x]`；T3 收口。
- **AC**：AC-62 已验证；**AC-60/63 仍未验证**（等 T5/TestAC60/63）；`NOT RELEASE`。
- **NB-2 校正**：T3.2 `TestReleaseCommitment*` 实际 9 个（reviewer 早期按 7 个计），tasks 的
  “9 tests” 正确。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T5 - ADR-0011 + 真实迁移/AC60/63 证据

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（PASS）;
  `级别: 实现变更（Spec/代码混合）`; `阶段: 合入前`; base `a92c53a`..`0416bc6`.
- **ADR-0011**：人类裁决 A；`legacy_24h` + `LEGACY_24H_EXPIRY`；Inventory ack 同事务；
  Order `refund_release_receipts`；Payment 前必须 RELEASED；失败保持 COMPENSATING；多组/无组人工，
  有离仓组优先 EXPIRED_FINANCE；consumer-first。
- **T5 证据**：`TestT5RealMigrationSchemaAndAC60Backfill`（22 个真实迁移 + AC-60 回填）；
  `TestAC60`（读取/回填/有售后退款/发货）；`TestAC63`（unique/ambiguous/zero/COMPENSATING 前缀）；
  migration runner 支持 `DELIMITER $$`。T5 tests **7.984s PASS**；vet 0。
- **AC**：AC-60/63 仍未验证（AC-63 全链路 ack/Payment/EXPIRED_REFUND 未实现；AC-60 无售后自动分配/
  Inventory HELD/缺货未接线）；T5.2/3/5/6/7 未做；`NOT RELEASE`。
- **NB 修复**：多组 shipped carve-out、legacy_expired 后缀说明、旧 consumer DLQ、planned tests、
  迁移归属、t5-status 已提交。下一步 Phase 1（events source + ack 事件）。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T5 Phase 1 - legacy_24h + ack 契约

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（PASS）;
  `级别: 实现变更`; `阶段: 合入前`; base `a9758c7`.
- **events**：新增 `legacy_24h` + `LEGACY_24H_EXPIRY`；旧 source 仍封闭；
  ack `inventory.refund_release_completed`（RELEASED/FAILED、release_id/refund_id/order/released_at）。
- **Inventory**：consumer 接受 `legacy_24h`（consumer-first）；未发 producer。
- **未做**：ack producer、Order receipt、Payment-last、migration；AC-63 仍未验证。
- 测试：events source/ack 正反例 + inventory 接受；`go test ./internal/platform/events -run TestRefundRelease`
  0.100s、inventory 0.117s PASS；vet 0；boundary 0。
- **NB**：ack `items` 未 required/validated；FAILED `error` 可选；FAILED `released_at` 语义待 Phase 2。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T5 Phase 2 - Inventory release + ack 同事务

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（PASS）;
  `级别: 实现变更`; `阶段: 合入前`; base `9853e01`.
- **实现**：`source=legacy_24h` 路由到 ack 路径；一个 `svc.write` 事务内逐项
  `releaseTx`/`cancelReservationTx`，成功写 Inventory outbox
  `inventory.refund_release_completed`（RELEASED）；永久失败回滚后写 FAILED ack；
  可重试失败不写 ack；ack 去重。
- **测试**：同事务释放+ack、重复消息只 1 ack、永久失败 FAILED ack、部分释放回滚
  （新增）；Inventory `2.412s`/`0.720s` PASS；events PASS；vet 0。
- **未做**：Order `refund_release_receipts`/ack consumer/Payment last/EXPIRED_REFUND（Phase 3）；
  AC-63 仍未验证；T5.2/3/5/6/7 未做；`NOT RELEASE`。
- **NB**：ack 去重无 `(event_type,aggregate_id)` 唯一键（并发 no-row 可重复）；FAILED ack
  阻断后续 RELEASED 的语义待 Phase 3；FAILED ack 写失败仅 DLQ。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T5 Phase 3a - Order release receipt + ack consumer

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（PASS）;
  `级别: 实现变更`; `阶段: 合入前`; base `f27c17c`.
- **迁移 0024**：`refund_release_receipts`（PENDING/RELEASED/FAILED、lease、version、refund UNIQUE）。
- **ack consumer**：订阅 Inventory ack；只 `PENDING→RELEASED/FAILED`；重复 no-op；终态不降级；
  FAILED 记 last_error；order 不匹配/未知 fail-closed；`cmd/order` 启动。
- **未做**：legacy worker 写 release/receipt、Payment-last、EXPIRED_REFUND（Phase 3b）；
  AC-63 仍未验证；T5.2/3/5/6/7 未做；`NOT RELEASE`。
- **测试**：真实迁移 23 个文件 + ack 2 例 **4.146s PASS**；vet 0。
- **NB**：`insert` 同 refund 不同 release 静默 no-op（Phase 3b 校验）；未知 receipt/终态/order mismatch
  测试缺口；启动接线无集成测试。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T5 Phase 3b - AC-63 全链路

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（PASS）;
  `级别: 实现变更`; `阶段: 合入前`; base `098e7fe`.
- **实现**：`compensateLegacyResolution` 写 `refund_applications(legacy_expired)` + release
  (`legacy_24h`) + PENDING receipt（同事务）；PENDING/FAILED 零 Payment；RELEASED 后
  `Payment.Refund`（幂等键 `pay:<payment_no>:legacy_expired`、non-effective）→ app DONE →
  `EXPIRED_REFUND`；Payment 失败保持 COMPENSATING。
- **worker**：scan/claim 扩展 COMPENSATING；shipped 优先 `EXPIRED_FINANCE`；多组/无组人工。
- **测试**：`TestLegacy24hAckThenPaymentAndExpiredRefund`（ack 前 0 Payment、ack 后 1、
  EXPIRED_REFUND、app DONE）、`TestLegacy24hFailedAckKeepsCompensatingAndPaysNothing`；
  full `TestLegacy|TestT5RealMigration|TestAC60|TestAC63|TestRefundReleaseAck|TestReleaseCommitment`
  **41.224s PASS**；vet 0。
- **AC-63**：未验证 → **已验证**（单组自动 + 多组人工 ADR-0011 A）；**AC-60 仍部分**；
  T5.2/3/5/6/7 未做；`NOT RELEASE`。
- **NB**：release/receipt 只对应第一个 refund（其余在 applications/Payment）；app DONE 与
  EXPIRED_REFUND 非同一 tx；Payment 永久错误仍按 Unavailable 重试；FAILED/PENDING 无专用告警；
  重试重调所有 refund（依赖 Payment 幂等）。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T5 - AC-60 收口

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（PASS）;
  `级别: 实现变更`; `阶段: 合入前`; base `4202747`.
- **proto**：`AdmitRefundRequest.client_request_id=11`（additive；只重生成 `mall.pb.go`）。
- **迁移 0025**：`legacy_refund_allocations(order_no, request_id, refund_id)`；
  `AdmitRefund` 空 refund_id 时按 request_id 幂等分配 `legacy:<order_no>:<seq>`。
- **发货**：`checkOrderReady` 读行状态；HELD→Confirm；CONFIRMED→放行；MISSING/确认失败→
  shortage/refund 错误，不消费 permit。
- **测试**：自动分配幂等、HELD/MISSING 发货；full **61.813s/41.902s PASS**；proto-guard OK；
  vet 0；`internal/services/inventory` 0 改动。
- **AC-60**：未验证 → **已验证**；**AC-63 保持已验证**；T5.2/3/5/6/7 未做；`NOT RELEASE`。
- **NB**：无并发 loser/空 request_id/CONFIRMED/Confirm 失败专门测试；D-11 确认失败上限未实现
  （ops 可 `ReportGroupShortage`，非永久阻断）；分配在 order 状态检查前会消耗 seq。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 状态变化: PAY-SETTLE-01 T5.2/T5.3 - race + 故障证据

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: -（T5.6 复审待）`;
  `级别: 实现变更（测试/证据）`; `阶段: T5`; base `f98f662`.
- **T5.2**：`go test -race ./internal/services/order` → **195.753s PASS**（MySQL；order 全包）。
  可标 `[x]`（order 范围）；payment/inventory/aftersales 未跑。
- **T5.3**：AC-50 crash replay、Ship/Cancel race、T17 crash、AC-63 Payment 失败/FAILED ack
  → **3.895s PASS**；AC-43/47/55/56/59/61、TCC crash/lazy reconcile、DLQ 未跑；保持 `[ ]`。
- **AC-60/63** 保持已验证；T5.1/T5.5/T5.6/T5.7 未做；`NOT RELEASE`。
- 证据：`docs/lead/PAY-SETTLE-01/t5.2-t5.3-evidence.md`；新增
  `TestLegacy24hPaymentFailureKeepsCompensating`。

### 2026-11-27 人类决定: PAY-SETTLE-01 T5.3 - AC-59 退款投影先于捕获拒绝

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（PASS，部分）;
  `级别: 实现变更`; `阶段: 合入前`; base `ec36339`.
- **实现**：`applyPaymentRefunded` 读取 `paid_cent`；`RefundedCent>0 && paid_cent<=0` →
  `EVENT_OUT_OF_ORDER`，在 ledger/金额/commitment 写入前返回；消费者可有限重试。
- **测试**：`TestPaymentRefundedBeforeCaptureRejected`（PENDING_PAYMENT/paid=0 → 错误、
  refunded_cent 仍 0）；既有累计较老事件仍 SETTLED；full **100.678s PASS**；vet 0。
- **AC-59**：只完成 1/4（退款投影先于捕获）；收货先于发货、库存失败先于支付成功、缺货解决先于
  上报未做；**AC-59 仍未验证**；T5.3 仍部分；AC-60/63 不变；`NOT RELEASE`。
- **NB**：历史 PAID paid=0 会 fail-closed（需回填/告警）；未断言 ledger/commitment 零行；
  未测 consumer 重试/DLQ 分类。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T5.3 - AC-59 3/4 + AC-57 CREATED

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（PASS，部分）;
  `级别: 实现变更`; `阶段: 合入前`; base `54013a6`.
- **缺货解决先于上报**：无 case → `EVENT_OUT_OF_ORDER` + Warn，不建案/不写 outbox。
- **收货先于发货 / AC-57 CREATED**：`confirmReceiptTx` 仅 `SHIPPED/RECEIVED`，其余
  （含 CREATED）→ `EVENT_OUT_OF_ORDER`，在 receipt/evidence/status 写入前拒绝。
- **测试**：`TestAC59ShortageResolutionBeforeReportRejected`、
  `TestAC59ReceiptBeforeShipmentRejected`；order **40.648s**、fulfillment **5.660s PASS**；vet 0。
- **AC-59 3/4**（库存失败先于支付成功未做）；**AC-57 部分**（仅 CREATED）；AC-60/63 不变；
  T5.3 仍部分；`NOT RELEASE`。
- **NB**：无 case 告警仅 Warn（无指标）；未来新 shipment 状态会被守卫判乱序；无 CANCELLED/RECEIVED
  新专门测试。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 人类决定: PAY-SETTLE-01 T5.3 - AC-43 settlement crash/resume

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（PASS，部分）;
  `级别: 实现变更`; `阶段: 合入前`; base `aad98a1`.
- **修复**：`claimSettlementTask` 原来只认 `PENDING`，崩溃后 `PROCESSING` 任务永不恢复；
  改为 `state IN (PENDING,PROCESSING)` 且 `lease_until` 过期可 claim，UPDATE 同条件 CAS。
- **测试**：`TestSettlementTaskCrashResume`（过期后新 owner 恢复并 DONE）、
  `TestSettlementTaskMaxAttemptsFailsPermanent`（10 次 → FAILED_PERMANENT）；3 PASS 2.155s；
  `TestSettlement` 2.149s PASS；vet 0。
- **AC-43 部分**：crash/resume + max attempts；CANCELLED 全额退款/HELD-CONFIRMED 未做；
  AC-59 3/4（第 4 条库存失败事件契约缺）；AC-60/63 不变；T5.3 仍部分；`NOT RELEASE`。
- **NB**：无活跃 lease 第二 claim 断言；UPDATE RowsAffected 未查；max attempt 未断言无 Payment；
  sleep 1.2s 有时序余量。
- 本条落盘后由本提交记录，**未 merge**。

### 2026-11-27 状态变化: PAY-SETTLE-01 T5.3 - AC-47/55/56 + TCC 对账证据

- `change_id: PAY-SETTLE-01`; `Actor: lead`; `Reviewed-by: -（T5.6 复审待）`;
  `级别: 实现变更（测试/证据）`; `阶段: T5`; base `ef867f4`.
- **AC-47**：新增 `TestAC47MarkPaidConfirmFailureCompensates`（Confirm 失败→TccCancelConfirm、
  保持 PENDING_PAYMENT、无 paid outbox）。
- **AC-55/56/TCC**：`TestAC25RefundProjection...`、`TestRefundProjection...`、
  `TestGroupShortageChoiceAndDefaultRefund`、`TestTccRefundBatchLazyReconcile*`、
  `TestTccReconcileWorker*` → **16.449s PASS**。
- **AC-61**：仍只有 `docs/ops.md` runbook，无告警/入口/每日对账代码证据。
- **T5.3 仍部分**：AC-59 第 4 条、AC-56 4h/48h、AC-61、AC-43 CANCELLED/HELD-CONFIRMED 未完成；
  AC-60/63 不变；`NOT RELEASE`。
- 证据：`docs/lead/PAY-SETTLE-01/t5.2-t5.3-evidence.md`。

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T5.3 AC-43 Payment crash/resume PASS（部分）

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `aad98a1`..工作树（未提交；HEAD=`aad98a1`；payment settlement/test；无 api/migration；2.155s/2.149s、vet 0、boundary 0）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-43；AC-59/AC-60/63 不变）；`docs/adr/0007-payment-settlement-admission.md`
- 结论: PASS（AC-43 部分（crash/resume + max attempts）；T5.3 仍部分；AC-60/63 不变；NOT RELEASE）
- 取代: 无
- 阻断项: 无; 待办: active-lease/RowsAffected；CANCELLED/HELD-CONFIRMED；AC-59 第 4 条；T5.3
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-43 未验证 -> 未验证（部分）；AC-60/63 不变

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T5.3 AC-59 第 2/3 分支 + AC-57 CREATED PASS（部分）

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `54013a6`..工作树（未提交；HEAD=`54013a6`；shortage/delivery/tests；无 api/migration；order 40.648s、fulfillment 5.660s、vet 0、boundary 0）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-59/AC-57；AC-60/63 不变）；`docs/adr/0007-payment-settlement-admission.md`
- 结论: PASS（AC-59 3/4；AC-57 部分（仅 CREATED）；AC-60/63 不变；NOT RELEASE）
- 取代: 无
- 阻断项: 无; 待办: shortage 告警/metric；AC-57 其余条款；AC-59 库存失败分支；T5.3
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-59 未验证 -> 未验证（3/4）；AC-57 未验证 -> 未验证（部分 CREATED）；AC-60/63 不变

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T5.3 AC-59 第一条 PASS（部分）

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `ec36339`..工作树（未提交；HEAD=`ec36339`；refund_consumer.go/test；无 api/migration；full 100.678s、targeted 2.597s、vet 0、boundary 0）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-59；AC-60/63 不变）；`docs/adr/0007-payment-settlement-admission.md`
- 结论: PASS（AC-59 部分（仅第一条）；T5.3 仍部分；AC-60/63 不变；NOT RELEASE）
- 取代: 无
- 阻断项: 无; 待办: 历史 PAID paid=0 fail-closed risk；consumer retry test；ledger/commitment zero-write assertion；AC-59 其余 3 分支；T5.3
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-59 未验证 -> 未验证（仅 1/4 分支；AC-60/63 不变）

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T5 AC-60 收口 PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `4202747`..工作树（未提交；HEAD=`4202747`；proto/migration 0025/allocation/shipping/tests；无 inventory；full 41.902s、proto-guard OK 1002、vet 0、boundary 0）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-60/AC-63/INV-46）；`docs/adr/0011-legacy-24h-release-ack.md`
- 结论: PASS（AC-60 已验证；AC-63 保持已验证；T5.2/3/5/6/7 未做；NOT RELEASE）
- 取代: 无
- 阻断项: 无; 待办: 并发 allocation/空 request/CONFIRMED/confirm-failure 测试；D-11 上限；分配先于状态检查；无 lines 兼容测试；AC-63；T5.2/3/5/6/7
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-60 未验证 -> 已验证（无售后自动分配/发货 HELD/MISSING + T5；CONFIRMED 代码路径；D-11 NB）；AC-63 已验证 -> 已验证

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T5 ADR-0011 Phase 3b PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `098e7fe`..工作树（未提交；HEAD=`098e7fe`；worker/test；无 migration/api/inventory；full 41.224s、vet 0、boundary 0）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-63/INV-46）；`docs/adr/0011-legacy-24h-release-ack.md`
- 结论: PASS（Phase 3b 可合入；AC-63 已验证（单组自动 + 多组人工 ADR-0011 A）；AC-60 仍部分；T5.2/3/5/6/7 未做；NOT RELEASE）
- 取代: 无
- 阻断项: 无; 待办: Payment failure/GetRefundByKey/permanent；app DONE+terminal 非原子；FAILED/PENDING scan 无告警；多组人工文档；Payment failure/multi-group/shipped 测试缺口；AC-60/T5.2/3/5/6/7
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-63 未验证 -> 已验证（单组自动 + 多组人工 ADR-0011 A；`TestLegacy24hAckThenPaymentAndExpiredRefund`/`...FailedAck` + T5 AC-63 prefix）；AC-60 未验证 -> 未验证（部分）

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T5 ADR-0011 Phase 3a PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `f27c17c`..工作树（未提交；HEAD=`f27c17c`；migration `0024`、ack consumer、`cmd/order`、2 tests；无 inventory/api；tests 4.146s、vet 0、boundary 0）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-63/INV-46）；`docs/adr/0011-legacy-24h-release-ack.md`
- 结论: PASS（Phase 3a 可合入；Phase 3b Payment-last 未做；AC-63 仍未验证；NOT RELEASE）
- 取代: 无
- 阻断项: 无; 待办: insert `ON DUPLICATE` one-to-one；unknown/terminal/order-mismatch/startup tests；direct apply 绕过 validator；Phase 3b；AC-63/T5.2/3/5/6/7
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-63 未验证 -> 未验证（Phase 3a 回执，不是 Payment/EXPIRED_REFUND；报告 §7）

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T5 ADR-0011 Phase 2 PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `9853e01`..工作树（未提交；HEAD=`9853e01`；`refund_release_consumer.go` `34b0e8d8…`、integration test `eb9b2d78…`；无 order/migration/api；inventory 2.312s、events 0.103s、vet 0、boundary 0）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-63/INV-46）；`docs/adr/0011-legacy-24h-release-ack.md`
- 结论: PASS（Phase 2 可合入；Phase 3 Order receipt/Payment 未做；AC-63 仍未验证；NOT RELEASE）
- 取代: 无
- 阻断项: 无; 待办: ack dedup 无 unique/并发；FAILED ack 阻塞 later RELEASED；retryable/partial rollback/CONFIRMED/no-row 测试；decode/group_count 错误无 FAILED ack；Phase 3；AC-63/T5.2/3/5/6/7
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-63 未验证 -> 未验证（Phase 2 release+ack，不是 Payment/EXPIRED_REFUND；报告 §8）

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T5 ADR-0011 Phase 1 PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `a9758c7`..工作树（未提交；HEAD=`a9758c7`；`events.go` `eaa8dcb0…`、`registry.go` `fdfc9728…`、tests `da6bdb34…`/`8bd4e100…`；无 order/migration/api；events 0.100s、inventory 0.117s、vet 0、boundary 0）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-63/INV-46）；`docs/adr/0011-legacy-24h-release-ack.md`
- 结论: PASS（Phase 1 可合入；Phase 2 Inventory ack 未做；AC-63 仍未验证；NOT RELEASE）
- 取代: 无
- 阻断项: 无; 待办: ack `items` 未 required/validated；FAILED `error` 可选；`released_at` 对 FAILED 语义；Phase 2/3；AC-63/T5.2/3/5/6/7
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-63 未验证 -> 未验证（Phase 1 不是全链路；报告 §7）

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T5 ADR-0011 + 证据 PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更（Spec/代码混合）; 阶段: Spec/代码; base..head: `a92c53a..0416bc6`；新增 ADR-0011、T5 plan/status、T5 tests、migration runner；`migrations/api` 0；T5 tests 7.984s、vet 0、boundary 0
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-60/AC-63/INV-46）；`docs/adr/0011-legacy-24h-release-ack.md` Accepted
- 结论: PASS（ADR-0011 可进入 Phase 1 实现；AC-60/63 仍未验证；T5.2/3/5/6/7 未做；NOT RELEASE）
- 取代: 无
- 阻断项: 无; 待办: NB-1 多组 shipped EXPIRED_FINANCE carve-out；NB-2 legacy_expired source/refund_id 与旧 addendum source 取代；NB-3 旧 consumer DLQ；NB-4 ADR 核对 future tests；NB-5 t5-status “未提交”；NB-6 ADR 新迁移属 Phase 3；AC-60/63/T5.1/2/3/5/6/7
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-60/AC-63 未验证 -> 未验证（T5.4 真实迁移/前缀不是 AC 全条；报告 §7）

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T3.4/T3.5/T3.7 收口 PASS（仅测试）

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `4a743ee`..工作树（未提交；HEAD=`4a743ee`；仅新增 `t3_7_refund_admission_integration_test.go`；无生产代码/迁移/契约；3 tests 1.996s、full 69.961s、AfterSales 3.623s、vet 0、boundary 0）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-60/63 不升级；T3.4/T3.5/T3.7 无独立 AC）；`docs/adr/0007-payment-settlement-admission.md` Rev 2.3 / Amd1
- 结论: PASS（T3.4/T3.5/T3.7 可标 [x]；AC-60/63 仍未验证；NOT RELEASE）
- 取代: 无
- 阻断项: 无; 待办: AC-60/63 等 T5/TestAC60/TestAC63；NB-1 并发错误文本未断言；NB-2 T3.2 tasks “9 tests” 与 7 不符；AC-60/63 未验证
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-60/AC-63 未验证 -> 未验证（T3.4/T3.5/T3.7 收口，不是 AC 验证；报告 §0/§9）

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T3.2 人工释放 ops CLI SCOPED-PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `4f7793c`..工作树（未提交；HEAD=`4f7793c`；`refund_commitment_release.go` `2ba93949…`、CLI `d3ccfc6d…`、test `b9a4ce24…`、plan `86d714cb…`；`.gitignore`/drop list 测试卫生；7 tests 6.122s、full 63.232s、vet/build 0、boundary 0）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-62/INV-45/D-24）；`docs/adr/0007-payment-settlement-admission.md` Rev 2.3 / Amd1 §4.3/§6.4
- 结论: SCOPED-PASS（T3.2 ops CLI；“仅授权角色”依赖 ops 部署控制；AC-60/63 仍未验证）；release_status: NOT RELEASE
- 取代: 无
- 阻断项: 无; 待办: ops runbook 在 -apply 前核对 Payment 对账窗口（AC-62）；reviewed_at 未参与幂等比较；测试未显式断言 permit/order/group 不变；USER_WITHDRAW/CANCELLED/同 authorization 不同 release 测试缺口；AC-60/63 未验证
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-62 有引用 -> 已验证（T3.2 ops CLI；`TestReleaseCommitment*` 7 PASS；角色/对账窗口部署控制 NB）；AC-60/AC-63 未验证 -> 未验证

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T3.6 AfterSales→Order 映射 SCOPED-PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `07291e6`..工作树（未提交；HEAD=`07291e6`；`workers.go` `3ea335a4…`、`refund_admission_request_test.go` `cc7260b6…`、plan `2bf321aa…`；`.dsh/.specify` 自动升级排除；`internal/services/order`/api/migrations/docs/specs 未改；pure 0.089s、aftersales 2.074s、order 52.293s、vet 0、boundary 0）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-60/AC-63）；`docs/adr/0007-payment-settlement-admission.md` Rev 2.3 / Amd1 §4.4
- 结论: SCOPED-PASS（T3.6 AfterSales→Order 映射；168h hold 由既有 AfterSales worker；T3.6 可标 [x] 映射范围；T3.4/T3.5 仍 [ ]）；release_status: NOT RELEASE
- 取代: 无
- 阻断项: 无; 待办: 无真实 processRefundTask 集成断言（NB-3）；freight-only goods0/freight>0 归入 legacy amount-only（NB-1）；negative pending 被跳过（NB-2）；helper UNDETERMINED 未 normalize（NB-4）；no-lines 早返回绕过 UNDETERMINED 检查（NB-5）；AC-60/63 未验证
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-60/AC-63 未验证 -> 未验证（T3.6 映射子集；报告 §0/§9）

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T3.4/T3.5 Phase 2 SCOPED-PASS（Order 侧）

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `5db0f0b`..工作树（未提交；HEAD=`5db0f0b`；`refund_composition.go` `8aa0d0ef…`、`aftersales/responsibility.go` `87237f8d…`、plan `4758182f…`、settled test `7110890c…`；新增 `refundpolicy` `61106c25…`/`31dbba21…`、`t3_5_phase2_integration_test.go` `722a1f75…`；`refund_admission.go` 未改；`tasks.md` T3.4/T3.5 仍 `[ ]`；order 42.730s、policy 0.063s、AfterSales 定向 1.302s、vet 0、boundary 0）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-60/AC-63）；`docs/adr/0007-payment-settlement-admission.md` Rev 2.3 / Amd1 §4.4
- 结论: SCOPED-PASS（Phase 2 Order 侧；AfterSales 枚举源等价；T3.6/168h 未做）；release_status: NOT RELEASE
- 取代: 无
- 阻断项: 无; 待办: T3.6 AfterSales 接线/168h 未做；quantity>0 数量超退测试缺口；quantity=0 部分金额边界产品确认；AfterSales 不 normalize 一致性；无 lines UNDETERMINED 错误文案；AC-60/63 未验证
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-60/AC-63 未验证 -> 未验证（Phase 2 子集；报告 §0/§9）

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T3.4/T3.5 Phase 1 delta SCOPED-PASS（NB 加固）

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `0740286`..工作树（未提交；HEAD=`0740286`；`refund_admission.go` 6/6；`refund_composition.go` `2a2468d4…`、`t3_5_line_composition_integration_test.go` `f1a7086d…`、plan `3abc51c7…`；6 个 Phase 1 测试 PASS；full 51.969s；`git diff 0740286 -- migrations api docs/specs` = 0）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-60/AC-63）；`docs/adr/0007-payment-settlement-admission.md` Rev 2.3 / Amd1 §4.4
- 结论: SCOPED-PASS（Phase 1 delta；NB-1/2/4/6 已修；新增 NB-7 quantity=0 产品语义）；release_status: NOT RELEASE
- 取代: 无（原 2026-11-27 Phase 1 SCOPED-PASS 仍有效；本条为 NB 加固后的 delta 确认）
- 阻断项: 无; 待办: T3.4/T3.5 其余未做；NB-7 quantity=0 部分金额语义需产品裁决（不阻断 Phase 1）；NB-5 known=1 无 lines 的 amount-only 未发现；并发/known=0 amount-only/mixed 测试缺口；AC-60/63 未验证
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-60/AC-63 未验证 -> 未验证（Phase 1 子集；报告 §0A/§10）

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T3.4/T3.5 Phase 1 行组成 SCOPED-PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `0740286`..工作树（未提交；HEAD=`0740286`；`refund_admission.go` 6/6；新增 `refund_composition.go` `6115b48f…`、`t3_5_line_composition_integration_test.go` `c5d4b9d1…`、plan `316b1dce…`；`tasks.md` T3.4/T3.5 仍 `[ ]`）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-60/AC-63）；`docs/adr/0007-payment-settlement-admission.md` Rev 2.3 / Amd1 §4.4
- 结论: SCOPED-PASS（Phase 1 行组成 + commitment lines；T3.4/T3.5 其余未做）；release_status: NOT RELEASE
- 取代: 无
- 阻断项: 无; 待办: responsibility/reason_code/168h/T3.6 未做；SETTLED/known=1 prior/positive freight/idempotency/concurrency 测试缺口；plan Draft 状态与“唯一键兜底”措辞；header 冲突未比较 order/group/source；AC-60/63 未验证
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-60/AC-63 未验证 -> 未验证（Phase 1 子集；报告 §0/§10）

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T3 审计 + T3.1/T3.3 不变量测试 SCOPED-PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `7d103a5`..工作树（未提交；HEAD=`7d103a5`；新增 `t3_invariants_integration_test.go` `e167630e…`、`t3-audit.md` `44236a31…`；`tasks.md` `7796b327…`；`git diff` 无内部非测试代码/迁移/proto/契约）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md` v1.11（AC-60/AC-63）；`docs/adr/0007-payment-settlement-admission.md` Rev 2.3（§4.4/Amd1 §6.5）
- 结论: SCOPED-PASS; 范围: T3.1/T3.3（公共路径测试 + 全包 permit 写路径 grep）；T3.2/T3.4/T3.5/T3.6 缺口审计；release_status: NOT RELEASE
- 取代: 无
- 阻断项: 无; 待办: tcc/worker permit 路径未测试（grep 审计）；CONSUMED partial 分支未测试；outbox 断言按 aggregate_id；审计“Tx1 前”应改为“Tx1 内写前”；审计行号/GRANTED→REVOKED/zero UPDATE 829/EOF 修正；T3.2/T3.4/T3.5/T3.6 需 Plan/人类批准；AC-60/63 未验证
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-60/AC-63 未验证 -> 未验证（T3.1/T3.3 子集；报告 §5）

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T2.5/T2.6/T2.7 复审 PASS（B1 消解；ADR-0010）

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待合入；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `45fe958`..工作树（未提交；HEAD=`45fe958`；`refund_seq.go` `8c02a7c2…`、`refund_seq_integration_test.go` `77d4d54e…`、`legacy_backfill_integration_test.go` `dd205dbb…`、`ADR-0010` `c5b6643a…`、plan `77ef33ed…`、tasks `75abf7a1…`；tracked diff 仅 `tasks.md` 3/3 行）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md`（AC-60/AC-63）；`docs/adr/0007-payment-settlement-admission-amd1.md` §6.5（事务范围由 **ADR-0010** 取代）；`docs/adr/0010-legacy-refund-seq-autonomous.md`（Accepted）
- 结论: PASS（实现阶段；T2.5/T2.6/T2.7；B1 消解；ADR-0010）；**可交人类合入**；T2.8/T3 未做；AC-60/AC-63 仍未验证（子集）；release_status: NOT RELEASE（PASS 只表示该范围证据满足，不是 Release）
- 取代: 2026-11-27 `审查裁决: PAY-SETTLE-01 - T2.5/T2.6/T2.7 实现复审 FAIL（1 阻断）`（仅结论 FAIL -> PASS；原条目保留）
- 阻断项: 无（B1 消解：`nextLegacyRefundSeq` 独立短事务 commit、rollback 不复用；`TestLegacyRefundSeqRollbackDoesNotReuse` PASS；ADR-0010 Accepted 取代 Amd1 §6.5 事务范围）；待办: AC-60/63 等 T3.5/T3/T5；T2.8/T3 未做；NB: 0019 注释 stale、ADR-0010 标题/Status、plan“不新开”、测试未持 orders 行锁、CLI exit 2 文档
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-60/AC-63 未验证 -> 未验证（T2.5/T2.6/T2.7 子集；报告 §0A/§6）

### 2026-11-27 审查裁决: PAY-SETTLE-01 - T2.5/T2.6/T2.7 实现复审 FAIL（1 阻断）

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待裁决；作者 `lead` 不作 Reviewed-by）; 级别: 实现变更; 阶段: 代码; base..head: `45fe958`..工作树（未提交；HEAD=`45fe958`；新文件未跟踪：`legacy_backfill.go` `f686acee…`、`refund_seq.go` `7b5a340a…`、两个测试 `72fd8179…`/`a22bdb41…`、`cmd/.../main.go` `f888d860…`、`t2.5-plan.md` `7721b90e…`；tracked diff 仅 `tasks.md` 3/3 行）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md`（AC-60/AC-63）；`docs/adr/0007-payment-settlement-admission.md` §4.4；`docs/adr/0007-payment-settlement-admission-amd1.md` §6.5/§12.5.5
- 结论: FAIL; 范围: T2.5/T2.6/T2.7；T2.5/T2.7 测试通过，T2.6 B1（同 tx rollback 复用 seq）；AC-60/AC-63 未验证（T2.5/T2.6/T2.7 子集）；release_status: NOT RELEASE
- 取代: 无
- 阻断项: **B1** T2.6 `nextLegacyRefundSeq` 在调用方事务内取号；rollback 回滚 `next_seq`，下一次复用同一 seq（实测两次 `seq=1`），与 ADR Amendment 1 §6.5“不得复用/回退”、迁移 `0019:55-58`“never reused or rolled back”冲突；代码注释自行弱化为“committed allocations”（宪法 4/7/20/22）。; 待办: 人类/架构裁决“独立提交取号”vs“修正‘不得复用/回退’为已提交 id 不复用”；加 rollback 测试；NB-1 部分失败、NB-2 零元+已捕获、NB-3 计数、NB-4 exit 2、NB-5 引用；T2.8/T3 未做
- AC 位移: `docs/specs/0002-payment-settlement-admission.md` AC-60/AC-63 未验证 -> 未验证（T2.5/T2.6/T2.7 子集；T2.6 B1；报告 §6）

### 2026-11-27 人类批准: SDD-STRUCT-ALIGN-01 - Spec Kit 双布局试点（Spec + 实现 PASS）

- `change_id: SDD-STRUCT-ALIGN-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（Spec PASS + 实现 PASS）;
  `级别: 契约变更（工具链）`; `阶段: 合入前`.
- 人类 2026-11-27 批准方案 A（双布局 + 自举试点）+ Spec.md 一行指针；Spec 链接的
  `docs/adr/0009-spec-kit-dual-layout.md` 同批接受。
- Spec: `specs/sdd-struct-align-01/spec.md` **Approved**；ADR-0009 **Accepted**；
  AC-1…AC-6 **已验证**（reviewer 独立实测；AC-1 修正为 `FEATURE_DIR` + `--paths-only`
  的 `FEATURE_SPEC` 两条断言）。
- 实现：新增 `specs/README.md`、`Spec.md` 末行一行指针；`docs/specs/0001/0002` 未迁移/未改写；
  核心模板/脚本/代码未改；无业务行为/AC。
- 结果：实现 **PASS**；`NOT RELEASE`；合入由人类执行。本条落盘，**未 commit/未 merge**。

### 2026-11-27 审查裁决: SDD-STRUCT-ALIGN-01 - 实现复审 PASS（AC-4/AC-5 已验证）

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待合入；作者 `lead` 不作 Reviewed-by）; 级别: 契约变更（工具链）; 阶段: 代码; base..head: `6a66ea5`..工作树（未提交；HEAD=`6a66ea5`；spec sha256 `d8384b192fdd1462ebfe4de2480366bcde1b32d9636615b504fbb70a6c274bb7`、`specs/README.md` `4c9a7f06e46ce483b7a58045ef75acd7ab400019a15351f4bff4b1ea3dd36b57`、`Spec.md` `5866ee3ee29ddfd56c9f88d79311aac8ba41e2ad816fee2f59d3dc82b5731a40`、ADR `87bc3688b35f90ddc0e0992ba15f84fbbd893092f6d842681490c112ceed7e09`；`docs/specs/`/核心模板/脚本 diff 0）
- Spec 和 ADR: `specs/sdd-struct-align-01/spec.md`；`docs/adr/0009-spec-kit-dual-layout.md`（Proposed）
- 结论: PASS（实现阶段；AC-4/5 已验证，AC-1/2/3/6 保持已验证）；**可交人类合入**；release_status: NOT RELEASE（PASS 只表示该范围证据满足，不是 Release）
- 取代: 无
- 阻断项: 无; 待办: 人类合入；Release 由人类；NOT RELEASE；合入前作者把 Spec header `Status` 从 Draft 更新为 Approved（人类已批准，非阻断）
- AC 位移: `specs/sdd-struct-align-01/spec.md` AC-4 未验证 -> 已验证（`docs/specs/0001/0002` diff 0；`Spec.md` 仅 +1 行、无删改；报告 §0B）；AC-5 未验证 -> 已验证（`specs/README.md` 28 行覆盖 AC-5；报告 §0B）；AC-1/2/3/6 保持已验证

### 2026-11-27 审查裁决: SDD-STRUCT-ALIGN-01 - Spec 复审 PASS（B1 消解）

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 契约变更（工具链）; 阶段: Spec; base..head: `6a66ea5`..工作树（未提交；HEAD=`6a66ea5`；spec sha256 `b2303b5cdc295d7e27461adf9b1261ce2c469784072891748db639a4b66890d3`、ADR `87bc3688b35f90ddc0e0992ba15f84fbbd893092f6d842681490c112ceed7e09`、plan `239d054b5ac4e593f9dd827835e3b139372935e59d9f1cc969e1fca224c99322`、tasks `aef4a686f61958975af4209bcf8388bc85ae33f08bf48d3add48fab44c0a9155`；新文件未跟踪，final sha 以文件 sha256 为准）
- Spec 和 ADR: `specs/sdd-struct-align-01/spec.md`；`docs/adr/0009-spec-kit-dual-layout.md`（Proposed）
- 结论: PASS（Spec 阶段；AC-1/2/3/6 已验证，AC-4/5 未验证、实现阶段复审）；**Spec 可交人类批准**；release_status: NOT RELEASE（PASS 只表示该范围证据满足，不是 Release）
- 取代: 2026-11-27 `审查裁决: SDD-STRUCT-ALIGN-01 - Spec Kit 双布局试点 Spec FAIL（1 阻断）`（仅结论 FAIL -> PASS；原条目保留）
- 阻断项: 无（B1 消解：FR-001/AC-1 改为 `--json --require-spec` 的 `FEATURE_DIR` + `--paths-only --json` 的 `FEATURE_SPEC`，两条实测 exit 0；NB-1 行号 `check-prerequisites.sh:20/:110-125/:139-169`、`create-new-feature.sh:280/:424-425` 已核；NB-2 AC↔FR 已加；NB-3/NB-4 ADR 已改）；待办: 人类批准后实现 AC-4（`Spec.md` 指针）与 AC-5（`specs/README.md`），代码阶段复审；AC-4/5 不得在 Spec 阶段标已验证
- AC 位移: `specs/sdd-struct-align-01/spec.md` AC-1 未验证 -> 已验证（两条命令 exit 0；报告 §0A）；AC-2/3/6 保持已验证；AC-4/5 保持未验证（实现阶段）

### 2026-11-27 审查裁决: SDD-STRUCT-ALIGN-01 - Spec Kit 双布局试点 Spec FAIL（1 阻断）

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 契约变更（工具链）; 阶段: Spec; base..head: `6a66ea5`..工作树（未提交；HEAD=`6a66ea5`；新文件全部未跟踪：spec sha256 `eb29fe1a1b518743a47e661eb97d4a2181295255ef011edea6027960a18d1520`、ADR `a489df1c24c9cf42d5232b6807447f240ec195f62d8d79ece7ce34bbc59f4d8a`、plan `c9f0797e20addc06c7196a7ac18697687df28dd3bd0d1236b16bb899e4aa6258`、tasks `aef4a686f61958975af4209bcf8388bc85ae33f08bf48d3add48fab44c0a9155`；tracked diff vs base = 0）
- Spec 和 ADR: `specs/sdd-struct-align-01/spec.md`；`docs/adr/0009-spec-kit-dual-layout.md`（Proposed）
- 结论: FAIL; 范围: Spec 完整性/ADR/命令实测/边界/AC 状态；AC-2/3/6 已验证，AC-1 未验证（FR-001 `FEATURE_SPEC` 不在 `--json` 输出），AC-4/5 未验证（实现阶段）；release_status: NOT RELEASE
- 取代: 无
- 阻断项: **B1** FR-001/AC-1 断言 `check-prerequisites.sh --json --require-spec` 的 `FEATURE_SPEC`，但 JSON 模式只输出 `FEATURE_DIR`/`AVAILABLE_DOCS`（`FEATURE_SPEC` 只在 `--paths-only`）；Spec Assumptions 禁改脚本，AC-1 不可达；修正 FR-001/AC-1 后重审。; 待办: NB-1 可达性只给 `common.sh:163-230`（check-prerequisites/create-new-feature 无行号）；NB-2 AC 表原无 AC↔FR 映射（reviewer 已补）；NB-3 ADR 核对仍写“本仓待验证”；NB-4 ADR 背景把 tasks 说成总是要求；人类批准后实现 AC-4/5 并复审
- AC 位移: `specs/sdd-struct-align-01/spec.md` AC-1 未验证 -> 未验证（FR-001/`--json` 无 `FEATURE_SPEC`；报告 §2）；AC-2 未验证 -> 已验证（`--require-tasks` exit 0，`AVAILABLE_DOCS:["tasks.md"]`；报告 §2）；AC-3 未验证 -> 已验证（缺 plan/tasks/spec/目录 exit 1；报告 §2）；AC-4 未验证 -> 未验证（`Spec.md` 指针待实现；报告 §3）；AC-5 未验证 -> 未验证（`specs/README.md` 未创建；报告 §3）；AC-6 未验证 -> 已验证（resolver 含附录 A；报告 §2）

### 2026-11-27 人类决定: GOV-C2-TEMPLATE-01 - C-2 以 preset append 加附录 A

- `change_id: GOV-C2-TEMPLATE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（PASS）;
  `级别: 治理变更`; `阶段: 落地`.
- 人类选择 **B（装 PyYAML + preset append）**；user-site `pyyaml 6.0.4` 已安装（经人类批准；
  只写用户目录，未改 Homebrew 系统包）。
- `.specify/presets/gin-kratos-spec/`：`preset.yml` 为 Spec Kit 1.4.3 schema 1.0；
  `spec-template.append.md` 含附录 A 8 项；`specify preset list` 显示 enabled；
  `resolve-template.sh spec-template` exit 0、145 行 = 核心模板 + 附录 A；override 已删；
  核心模板未改。
- 维护：preset append 自动叠加核心模板；宪法附录 A 变化时更新 append 文件
  （preset description 已注明）。
- 未 commit/未 merge（本条落盘后由 C-2 提交）。

### 2026-11-27 审查裁决: GOV-C2-TEMPLATE-01 - Spec 模板附录 A 对照 preset append PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 治理变更; 阶段: Spec; base..head: `a0ec6ba`..工作树（未提交；HEAD=`a0ec6ba`；preset.yml sha256 `74d7ceb1bea644daf62c4e97c174b0899b98c0bdf46a93ea22691224d6225828`，append sha256 `31b56a1066085be7c557923bbc1f5642e055215e4fbf8f315d653b7e629854e9`，.registry sha256 `b945dd764b47180c9af7eae6800d51ec93d864d9b7e140722bf2882eb5ec780a`，核心模板 sha256 `3945437fc35cd30a5b2bf7beea680337c3516826d3efa5a6b92c4a7eca1ba28e`，resolver sha256 `829e227096abc8bf0889889ec9f792f503ca5d395b7836a8a7eb739ee75214e7`；宪法/AGENTS 与 base 相同 `414b94a6b9419a01abc7573036de9eceda4c0f75120f9d6bd9cf9b2163b8ba4f`/`05d948fbac7f466d187bb054143f9fff3d44d310295d62e90a06fc7bc804021b`）
- Spec 和 ADR: 无（`.specify/presets/gin-kratos-spec/{preset.yml,spec-template.append.md}` + `.specify/presets/.registry`；override 已删除；核心模板/宪法/AGENTS 未改）
- 结论: PASS（**preset append 不影响 PASS**）; 范围: preset append（8 项含 NB-1/2/3 的 EARS/MUST、AC 编号、延期禁止项）；resolver 输出=核心模板+附录段（145 行，附录 A 对照 1 处；`--json` `TEMPLATE_CONTENT` 5324 字符）；无实例名/业务 AC 变更；release_status: NOT RELEASE（PASS 只表示该范围证据满足，不是 Release）
- 取代: 无
- 阻断项: 无; 待办: NB-1…NB-5 已消解（append 内容补 EARS/MUST、AC 编号、延期禁止项；override 删除后 NB-4 消失；description 明写“Appendix A 变化时同步本文件”）；preset append 自动叠加核心模板，人类 B 选择的维护风险已由该方式覆盖
- AC 位移: 无（工具链/治理变更，不涉及 AC 状态）；方案变更: 原 override 方案（sha256 `a546438a75131c57d6e65229e64064eed9c0cad0b8063b4002e5a1bc409c1d8a`）已删除，改为 preset append；final sha: `preset.yml` `74d7ceb1bea644daf62c4e97c174b0899b98c0bdf46a93ea22691224d6225828`、`spec-template.append.md` `31b56a1066085be7c557923bbc1f5642e055215e4fbf8f315d653b7e629854e9`、`.registry` `b945dd764b47180c9af7eae6800d51ec93d864d9b7e140722bf2882eb5ec780a`（preset 文件未跟踪；diff hash 会随条目文字变化，故不作为绑定）；证据: `docs/collab/qc-reports/REV-GOV-C2-TEMPLATE-01.md` §0A

### 2026-11-27 人类批准: GOV-SPEC-SYNTAX-01 / GOV-CORE-RESTORE-01 - 宪法 v2.2.0 生效

- `Actor: human (Director)`; `Reviewed-by: reviewer-syntax`（final PASS）；`级别: 治理变更`.
- 人类批准宪法 **v2.2.0** 最终文本（含 v2.1.0 的 Spec Kit 语法放宽与 v2.2.0 的 L2/L3 恢复）；
  `AGENTS.md` 同次批准。`Approved/Effective/Last Amended = 2026-11-27`。
- 批准前已确认：无“外部复审”概念，reviewer 即独立 subagent；术语更正不影响 PASS。
- 生效范围：治理文本；不改代码/AC 状态。未 commit/未 merge；合入由人类执行，并需一并提交
  `docs/archive/**`、`docs/review-tracking-archive.md`、审查报告与 `docs/convert/**` 等未跟踪文件。

### 2026-11-27 人类更正: GOV-CORE-RESTORE-01 - 不存在“外部复审”；reviewer 是独立 subagent

- 人类指出：本项目没有“外部复审”这一角色/概念；审查者就是独立 subagent（AGENTS §1）。
- 措辞更正（不改变规则语义）：宪法 12 与 AGENTS §3 改为“审查者（独立 subagent）的裁决是
  唯一审查结论；审查者之外的其他输入（外部 agent、顾问、自动扫描、对话建议）只作问题提案”。
- 旧 CONVENTIONS §2.2 的原意（非审查者输入不能直接成为契约/AC/阻断依据）保留，但不再使用
  “外部复审”这个名称。
- 请 reviewer 同步更正其报告与台账条目中的“外部复审”措辞；本更正不改 AC/状态，
  不 commit/merge。

### 2026-11-27 人类决定: GOV-CORE-RESTORE-01 - 恢复 L2 治理元数据与 L3 高风险领域条款；宪法 v2.2.0

- `change_id: GOV-CORE-RESTORE-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（final PASS）;
  `级别: 治理变更`; `阶段: 落地`; `人类批准: 2026-11-27（v2.2.0 最终文本批准）`.
- 宪法 v2.1.0 -> **v2.2.0**（MINOR：新增/恢复规则，不是删除或重定义原则；非 MAJOR）。
- **L2**：宪法 12 明确审查者（独立 subagent）的裁决是唯一审查结论；审查者之外的其他输入
  只作问题提案、必须引用当前 HEAD、找不到标 `[不确定]`、已裁决按文档漂移处理、且不削弱
  第 9 条的安全/权限/数据完整性/迁移阻断；AGENTS §2 跨服务
  integration owner（既有角色兼任，不新增角色）；AGENTS §3 裁决补 `review_scope`/
  `release_status`/`spec_version`/`adr_revision`/`verification`/INV/`baseline_impact`，
  HEAD 变化重新确认，plan 写 `baseline_impact` 与 `stop_conditions`。
- **L3**：宪法 15 覆盖订单/开发默认值、必需依赖构造期必填/启动 fail-fast、可选能力显式
  Options、测试替身仅 `_test.go`/build tag、启动失败非零退出；宪法 23 恢复 II.5 两条
  IF/THEN、`REVOKED` permit 不得自行恢复、commitment 规则、Σ 等式、`order.paid` 兼容期
  两层（兼容期四项+禁 flag；灰度影子期 ADR 切换/owner/收口+生产默认 OrderSettled）；
  宪法 26 恢复 `database/sql`+CAS、GORM 文件级读侧、`apperr`/`fmt.Errorf("%w")`/
  `docs/contracts.md` 错误映射。
- 收益：恢复 v1.3.0 的资金/履约/启动失败硬约束，使 L2/L3 审查有可引用条款。
  停止条件：若恢复条款与宪法 8/9/17 冲突，或同一变更因恢复条款第二次判为设计问题，
  回退对应条款并记台账；连续两个变更后复查。
- 独立审查：SCOPED-PASS -> NB-11 逐字化 FAIL -> 两层结构恢复 **final PASS**；
  报告 `docs/collab/qc-reports/REV-GOV-CORE-RESTORE-01.md`。
- 人类更正（2026-11-27）：不存在“外部复审”；reviewer 就是独立 subagent；其他输入才作问题提案。
- 未 commit/未 merge；人类批准最终文本后生效。

### 2026-11-27 审查裁决: GOV-CORE-RESTORE-01 - NB-11 两层结构恢复 final PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 治理变更; 阶段: Spec; base..head: `9e81497`..工作树（未提交；HEAD=`9e81497`；final head：宪法 sha256 `beffa845af65d32bec3de24c031d726e7beee4ed26414fb7e70ba01f873c629d`、AGENTS sha256 `597fb0f700727a99c596f561b5699daffa97474f1abdf58a3c4e91571cee874e`、`git diff 9e81497` sha256 `3ff0c5a95038ddc4dd61faa51863fdd450b2e38678310ca4cb952abbfe6f2e19`；落盘后 hash 见报告 §8）
- Spec 和 ADR: 无（宪法 v2.2.0 / `AGENTS.md` 修改）
- 结论: PASS; 范围: NB-11 两层结构恢复（兼容期四项 + flag 禁止；灰度影子期 ADR 切换步骤/owner/收口日期 + 生产默认 `OrderSettled`/ADR 等价事实）；NB-8/NB-12/NB-13 已确认；release_status: NOT RELEASE（PASS 只表示该范围证据满足，不是 Release）
- 取代: 2026-11-27 `审查裁决: GOV-CORE-RESTORE-01 - NB-11 逐字化 FAIL（1 阻断）`（仅结论 FAIL -> PASS；原条目保留）
- 阻断项: 无（B1 已消解）; 待办: 人类批准前补收益/停止条件与 2.2.0 MINOR 理由；NB-10 段落过载不阻断；“ADR 批准的版本化兼容期”未逐字（附录 B + 影子期 ADR 要求可推出，不阻断）
- AC 位移: 无（治理变更，不涉及 AC 状态）；证据: `docs/collab/qc-reports/REV-GOV-CORE-RESTORE-01.md` §0C
- 术语更正: 2026-11-27 人类更正“外部复审” → “审查者之外的其他输入”（审查者 = AGENTS §1 独立 subagent）；规则语义未变，**术语更正不影响 PASS**（B1/NB 不受影响）。

### 2026-11-27 审查裁决: GOV-CORE-RESTORE-01 - NB-11 逐字化 FAIL（1 阻断）

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 治理变更; 阶段: Spec; base..head: `9e81497`..工作树（未提交；HEAD=`9e81497`；NB-11 final head：宪法 sha256 `2121158a023214b8d4a5bf8e06cf8595f312004aaf963431201de1b211bdb887`、AGENTS sha256 `597fb0f700727a99c596f561b5699daffa97474f1abdf58a3c4e91571cee874e`、`git diff 9e81497` sha256 `38c24b6c315cc87679fc5bf842d74ab43b7a9f2727ab55456f09b841a96d87d0`；落盘后 hash 见报告 §8）
- Spec 和 ADR: 无（宪法 v2.2.0 / `AGENTS.md` 修改）
- 结论: FAIL; 范围: NB-11 逐字化后的宪法 23 兼容期范围；NB-8/NB-12/NB-13 逐字化已复核无新问题；release_status: NOT RELEASE
- 取代: 2026-11-27 `审查裁决: GOV-CORE-RESTORE-01 - final confirmation PASS`（仅结论 PASS -> FAIL；原条目保留）
- 阻断项: **B1（NB-11 逐字化）** 新 23 把“默认消费者切换/收口条件/移除日期/回滚路径”和“不得以 flag 默认 false 长期保留”全部挂在“灰度影子期的兼容期”下；旧 II.1 第一段把这些要求挂在“版本化兼容期”（含 BFF/旧消费者双发），只有 ADR 切换步骤/owner/收口日期与生产默认 `OrderSettled` 属于灰度影子期。纯 BFF 兼容期因此不再需要移除日期/回滚/收口条件，可长期保留 → 违反旧 II.1 的兼容期控制与宪法 6 的渐进式兼容。; 待办: 把四项+flag 禁止恢复到“兼容期”（含 BFF），ADR 切换步骤/owner/收口日期留在“灰度影子期”；重新审查后 PASS；人类批准前补收益/停止条件与版本理由
- AC 位移: 无（治理变更，不涉及 AC 状态）；证据: `docs/collab/qc-reports/REV-GOV-CORE-RESTORE-01.md` §0B

### 2026-11-27 审查裁决: GOV-CORE-RESTORE-01 - final confirmation PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 治理变更; 阶段: Spec; base..head: `9e81497`..工作树（未提交；HEAD=`9e81497`；final head：宪法 sha256 `d6f21e2d9dd5f7f42942c91d8e8e75f1af4570a4e5bf7f3bc627fe3d5329ae94`、AGENTS sha256 `fb2f7a6e7e82d8d28e1cbf6e876a73903f9f163d83d87577373fc2903b375c19`、`git diff 9e81497` sha256 `d44e0812128230923265f04266a40cdb7a12333c2409ef8a8962c337197a28dd`（评审输入）；落盘后 hash 见报告 §8）
- Spec 和 ADR: 无（宪法 v2.2.0 / `AGENTS.md` 修改）
- 结论: PASS; 范围: 补入 §5 的 II.5 两条 IF/THEN、II.1 第二段、I.2 限定、III.4 apperr、§2.2 后果 + NB-1…NB-9 + §3 复审纪律；release_status: NOT RELEASE（PASS 只表示该范围证据满足，不是 Release）
- 取代: 2026-11-27 `审查裁决: GOV-CORE-RESTORE-01 - L2/L3 高风险条款恢复 SCOPED-PASS`（仅结论 SCOPED-PASS -> PASS；原条目保留）
- 阻断项: 无; 待办: NB-8 的“写入审查报告”措辞、NB-10 段落过载（不阻断）、NB-11/NB-12/NB-13 观察；人类批准前补收益/停止条件与 2.2.0 MINOR 理由
- AC 位移: 无（治理变更，不涉及 AC 状态）；证据: `docs/collab/qc-reports/REV-GOV-CORE-RESTORE-01.md` §0A

### 2026-11-27 审查裁决: GOV-CORE-RESTORE-01 - L2/L3 高风险条款恢复 SCOPED-PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待批准；作者 `lead` 不作 Reviewed-by）; 级别: 治理变更; 阶段: Spec; base..head: `9e81497`..工作树（未提交；HEAD=`9e81497`；评审输入 `git diff 9e81497` sha256 `682be3ca70ff76ead2ec2eeffc852ad1e01902b648b73536b226f37097bd7b53`，宪法 sha256 `32ae52cfa1b5d05699f3bf12d886314f4ea8e12742e30df2bd3a09c3c97bf81b`，AGENTS sha256 `baf82ba092e02ffbb5d6154cf44fe1d91f87ae81416482a60ebd70b99ccfe409`；落盘后 hash 见报告 §8）
- Spec 和 ADR: 无（宪法 v2.2.0 / `AGENTS.md` 修改）
- 结论: SCOPED-PASS; 范围: L2（审查者之外的其他输入、integration owner、reviewer 元数据）+ L3（15/23/26）+ 版本 2.2.0；未覆盖: II.5 两条 IF/THEN、II.1 ADR/影子期/生产默认、I.2 开发默认值/订单范围、III.4 apperr 契约映射、§2.2 `[不确定]`/文档漂移、§3 变更输入/HEAD 重确认/baseline_impact 依据、§4.1 spec_version/adr_revision/verification/INV 依据/数据迁移（详见报告 §5）
- 取代: 无
- 阻断项: 无（列出的恢复项无阻断；12/9 交互、23“commitment 创建”措辞、integration owner 角色、`release_status=PASS` 定义为 NB）; 待办: NB-1…NB-10 + 人类批准前补收益/停止条件、版本 2.2.0 MINOR 理由、§5 未覆盖旧限定
- AC 位移: 无（治理变更，不涉及 AC 状态）；证据: `docs/collab/qc-reports/REV-GOV-CORE-RESTORE-01.md`

### 2026-11-27 人类决定: GOV-SPEC-SYNTAX-01 - 允许 Spec Kit 原生语法；宪法 v2.1.0

- `change_id: GOV-SPEC-SYNTAX-01`; `Actor: lead`; `Reviewed-by: reviewer-syntax`（re-review PASS）;
  `级别: 治理变更`; `阶段: 落地`; `人类批准: 2026-11-27（v2.1.0 批准）`.
- 宪法 v2.0.0 -> **v2.1.0**（MINOR：放宽 EARS 强制 + 新增延期清单/治理变更级别；非 MAJOR）。
- 改动：附录 A 行 1 允许 EARS 或 Spec Kit 原生语法（Given/When/Then + FR/SC）；新增
  “Spec Kit 兼容”注（Acceptance Scenarios 必须映射到 AC；默认布局用目录名作 change_id，
  迁入扁平布局保留台账已登记 id）；Spec 必须回答附录 A 问题而非固定章节；新增延期清单
  （延期≠豁免）；change_id 与 Spec 身份分开（长期 Spec 在台账登记，历史不回溯补登；
  同一时间一个 change_id）；tasks `[X]` 只是执行进度；审查者改为有运行时的外部 subagent
  （静态回退需人类书面豁免）；`speckit-implement` 分批/无阶段参数；台账同日按插入位置；
  尾注在合入提交；evidence 路径分流；模板覆盖层 replace vs append；实现变更做第 4-8 步、
  plan 一页以内；AGENTS 级别加“治理变更”。
- 收益：`speckit-specify` 生成的 Spec 不再需要改写为 EARS；可判定性、异常分支、错误码、
  证据要求不降低。停止条件：若出现用 Spec Kit 语法规避可判定 AC、异常分支或证据，
  恢复 EARS 强制并记入台账；连续两个变更后复查。
- 独立审查：`reviewer-syntax` 首轮 **FAIL**（B1：change_id 与 SPEC-0002 已登记
  `PAY-AC50-REVIEW-01` 冲突）→ 作者修复 → **re-review PASS**；报告
  `docs/collab/qc-reports/REV-GOV-SPEC-SYNTAX-01.md`。
- 未 commit/未 merge；人类批准最终文本后生效。

### 2026-11-27 审查裁决: GOV-SPEC-SYNTAX-01 - 宪法 v2.1.0/AGENTS 治理调整 re-review PASS

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（待人类批准；作者 `lead` 不作 Reviewed-by）; 级别: 治理变更; 阶段: Spec; base..head: `9e81497`..工作树（未提交；HEAD=`9e81497`；re-review 被审状态 = 作者修复后、reviewer 落 L5/台账前的 `git diff 9e81497` sha256 `23cbdb0b8a9ece434dbf2890419ef83031cf8325866d2fdf3ec9873389c68800`，宪法 sha256 `1314c3a169848bc10351bb228241ef53623d15c6a63579b473733013ead3e95a`，`AGENTS.md` sha256 `cd6d4394ac286bc23ae1221377e00dfed09a8cceb95324fb885d013af842f3cb`；落盘后 hash 见报告 §0A。**final head（实例名移除后）**：宪法 sha256 `f844060ee4b51d1d9c01d666b577b1bc874595a40a4f7d487b7c9af08be374b6`、AGENTS sha256 `e6e05c3dde851a7528e627a672ff133707ce92cd366b1a4a0abd8c62a6456d6f`、`git diff 9e81497` sha256 `0d3f4abc0caaaa64dba57521f7f985dffe8bdaadb9c80c4ea1314a774cf57535`（含 reviewer 前一轮落盘；本轮落盘后见报告 §0A））
- Spec 和 ADR: 无（宪法/`AGENTS.md` 修改）；被审：`.specify/memory/constitution.md` v2.1.0、根 `AGENTS.md`、`docs/specs/0002-payment-settlement-admission.md` §15.2/§15.3（L5 补行与更新）
- 结论: PASS（**final confirmation：实例名移除不影响 B1**，PASS 仍成立）; 范围: re-review B1 + NB-1…NB-10 + NB-14 + L1 + L4 + L5；静态审查；不含代码、SPEC-0001/0002 其余 AC 语义、C-2/C-3/C-4/D-2、13 个 addendum
- 取代: 2026-11-27 `审查裁决: GOV-SPEC-SYNTAX-01 - 宪法 v2.1.0/AGENTS 治理调整 FAIL（1 阻断）`（仅结论 FAIL -> PASS；原条目保留为历史）
- 阻断项: 无（B1 已消解：宪法 30 分离 Spec 身份与 change_id，点名 SPEC-0002 `PAY-AC50-REVIEW-01`/父 `PAY-SETTLE-01`，历史不回溯补登；附录 A 验收表行分离 AC 编号与 change_id）; 待办: NB-13 + R-4 + R-5 **本次不阻断**（NB-12 已由 2026-11-27 人类指示条目记录收益/停止条件）；R-1/R-2/R-3 已由最终改动消解；L2/L3 **本次不阻断**、另行修订
- AC 位移: SPEC-0001 AC-22/28/29/30/35/37 未验证 -> 未验证（原仅 §15.3 延期清单、无验收表落点；本次在 SPEC-0002 §15.2 补当前值行，证据/备注指向 §15.3；状态未变）；其余 AC 无位移；证据: `docs/collab/qc-reports/REV-GOV-SPEC-SYNTAX-01.md` §0A

### 2026-11-27 审查裁决: GOV-SPEC-SYNTAX-01 - 宪法 v2.1.0/AGENTS 治理调整 FAIL（1 阻断）

- Actor / Reviewed-by: `reviewer-syntax`（独立 subagent，与作者 `lead` 不同 actor） / `human`（本次调整的人类指示人；**尚未批准本裁决或 v2.1.0 文本**；作者 `lead` 不作 Reviewed-by）; 级别: 治理变更（宪法 Governance；AGENTS §4 的“级别”只列契约/实现，本值不在格式内，见报告 NB-14）; 阶段: Spec; base..head: `9e81497`..工作树（未提交；HEAD=`9e81497`；评审字节绑定：`git diff 9e81497` sha256 `21b4a9941e6d9d8b59305529591dea51b3e0a7739c8f7ca11757f07523849868`，宪法 sha256 `411b15f46c594c635258b2ffce218d9a3a076d98e6658f3fec3384458f2e829b`，`AGENTS.md` sha256 `444e75ae69ead387a4c75ab6cba1404f3389fae281fdce99a1a4867280c2d5bf`）
- Spec 和 ADR: 无（宪法/`AGENTS.md` 修改；`.specify/memory/constitution.md` v2.1.0 与根 `AGENTS.md` 工作树）
- 结论: FAIL; 范围: 宪法 v2.1.0（1/27/30/附录 A/Governance 版本）与 `AGENTS.md` §1–§6 的 v2.1 调整；静态审查（仅工具行为实测：`check-prerequisites` exit 1、`common.sh:642-650` override=replace、`git diff --check` exit 0）；不含代码、SPEC-0001/0002 AC 语义、C-2/C-3/C-4/D-2、13 个 addendum
- 取代: 无
- 阻断项: **B1** 宪法 30 新增“扁平文件取文件名主干”分支，与 SPEC-0002 已登记 `change_id: PAY-AC50-REVIEW-01`/`parent_change_id: PAY-SETTLE-01`（`docs/specs/0002-payment-settlement-admission.md:8-9`）、SPEC-0001 无 change_id、以及“扁平 Spec 原地升版、一生多 change_id”（`docs/specs/README.md:3,59-76`）冲突；文本未限定“新 Spec”，“登记后不得更换”只挂在“两者都没有”分支，使同一 Spec 出现两个 change_id，违反宪法 30“一个 Spec 一个 change_id”与“登记后不得更换”; 待办: NB-1（`FR/SC` 来源 vs 编号、Acceptance Scenarios 未逐条入表）、NB-2（`MUST` 未纳入 `SHALL/THEN` 形容词禁令）、NB-3（Spec Kit 目录 vs 扁平文件布局）、NB-4（静态回退无可判定触发条件）、NB-5（`checklists` 的 `[x]` 语义与 AGENTS §6 冲突）、NB-6（`speckit-implement` 在本仓不可运行且无阶段参数）、NB-7（同日/补记台账）、NB-8（宪法 30“提交尾注”vs AGENTS §4“合入提交”）、NB-9（审查报告 evidence 路径与本仓实际/qc-reports 冲突）、NB-10（延期 vs 豁免边界、缺 INV/逐台账/status/thread）、NB-11（SPEC-0002 §15.3“附录 A 尚未定义”已过时）、NB-12（最小增量未写可验证收益/停止条件）、NB-13（2.1.0 版本语义可争，[TODO]）、NB-14（“治理变更”不在宪法 3/AGENTS §4 级别内）；继承项 L1–L5 只列不改
- AC 位移: 无（治理变更，不涉及 AC 状态）；证据: `docs/collab/qc-reports/REV-GOV-SPEC-SYNTAX-01.md`（头部 review_scope/release_status/INV/Actor/Reviewed-by 齐全）

### 2026-11-27 human decision: freeze lift extended to T2->T3->T4->T5->P0

- Human decision (2026-11-27, after `reviewer-c3` G-1): the ruling 9-A freeze lift is
  extended from "Phase 0-R closeout + T2.4 remaining" to the full order
  **T2->T3->T4->T5->P0 closure** (ruling 7-B). T2.5/T2.6/T2.7, T3, T4 and T5 may
  proceed under the parent; no merge/Release; all other freeze constraints remain.
- This resolves the G-1 gate in `REV-GOV-CONVERT-01-C3.md`; the reviewer's
  SCOPED-PASS stands. `tasks.md` remains a view; this entry is authoritative.

### 2026-11-27 审查裁决: GOV-CONVERT-01 - B-1…B-6 状态对齐 + SPEC-0002 验收状态表 SCOPED-PASS

- Actor / Reviewed-by: `reviewer-c3`（独立 subagent，与作者 `lead` 不同 actor） / `human`（11 项裁决与冻结范围；作者 `lead` 不作 Reviewed-by）; 级别: 契约变更（状态迁移，不是行为变更）; 阶段: Spec; base..head: `9e81497`..工作树（未提交；含 B 阶段状态对齐与 C-1 落地）
- Spec 和 ADR: `docs/specs/0002-payment-settlement-admission.md`（新增 §15 验收状态表/延期清单，未改 AC 正文）；`docs/adr/0007-amd1-addendum-ac50-nowaybill.md`（`:7/:8/:98` 状态注记）
- 结论: SCOPED-PASS; 范围: B-1…B-6 状态对齐 + SPEC-0002 §15 表/延期清单 + 本台账条目；不含实现/merge/Release、AC-22/28/29/30/35/37 迁回 SPEC-0001、13 个 addendum 合并（C-3 后续）
- 取代: 2026-11-27 `OPS-P5b PASS; AC-27 AC-VERIFIED` 与 `PAY-SETTLE-01 Phase 0-R CLOSED` 中的 AC-27 AC-VERIFIED 表述（→ AC-REF）；2026-11-27 `T2a`/`T2.4 scoped PASS` 中的 AC-60/63 AC-REF 表述（→ DEFERRED）；AC-72 无台账条目被取代（台账原已写 "backend only"；本次只把 `ac-coverage.tsv` 单行 `AC-72` 拆为 `AC-72-BACKEND` + 两个 DEFERRED 工作项）；历史条目原文不改写
- 阻断项: 无（本裁决范围内，0 BLOCKER）；待办: ①裁决 9 的冻结放开范围仅 `Phase 0-R 收口 + T2.4 剩余`，而裁决 7-B 的下一步 `T2.5/T2.6/T2.7` 与 T3/T4/T5 不在放开范围，需人类在启动前明确（下一阶段门禁，非本裁决 FAIL）；②`docs/lead/PAY-SETTLE-01/tasks.md:60` 仍写“Phase 0-R 未闭合”与 `:26` 矛盾（Plan 视图漂移，台账为准）；③ADR nowaybill `:99` 残留“冻结 Parent”；④宪法附录 A 未定义延期清单/owner/expiry 格式（MINOR，见审查报告）
- AC 位移: AC-60 有引用 -> 未验证（`ac-coverage.tsv:29`；无 `TestAC60`）；AC-63 有引用 -> 未验证（`ac-coverage.tsv:32`；无 `TestAC63`）；AC-72 整体已验证 -> `AC-72-BACKEND` 已验证 + `REVIEW-REQUIRED-ALERTS-01`/`ENTRY-01` 未验证（`ac-coverage.tsv:47-49`）；AC-27 已验证 -> 有引用（`ac-coverage.tsv:41`；人类裁决 6）；证据路径见 SPEC-0002 §15 与 `docs/collab/qc-reports/REV-GOV-CONVERT-01-C3.md`

### 2026-11-27 人类决定: GOV-CONVERT-01 - 治理模型 v2.0 落地

- `change_id: GOV-CONVERT-01`; `Actor: lead`; `Reviewed-by: human (Director 2026-11-27)`;
  `级别: 治理变更（宪法 Governance；非契约/实现变更）`; `阶段: 落地`.
- **人类批准**：宪法 v2.0.0 与根 `AGENTS.md` 替换稿于 2026-11-27 批准并生效；
  `.specify/memory/constitution.md` 已替换，`AGENTS.md` 已替换。批准依据=2026-11-27 的
  11 项裁决（见下一条台账条目）+ C-1 最终文本批准。
- **A-2 取代关系（只记人类确认过的内容，不补推）**：
  - 取代旧清理方案 `docs/phase-legacy-cleanup-and-optimization-plan.md` 的 A/B/C 动作；
    Batch B 未完成的 MINOR 随旧治理文件停用而终止，**不追认为已完成**。
  - 取代 `GOV-COST-01` / `collab-gates` / pair 线程 / L0-L1-L2 分级 / 六角色矩阵 /
    `CONVENTIONS.md` / `prompt/**` / `ac-coverage.tsv` / `docs/handoff.md` /
    `docs/lead/PAY-SETTLE-01/backlog.md` 的现行效力；这些文件按裁决 4-B 在 C-4 归档，
    旧路径留 stub，历史引用不回溯改写。
  - 台账 `:966-968`（2026-11-25）记的 "`prompt/Developer.md` 指针 not done" 与事实不符：
    `prompt/Developer.md:4` 已存在 `docs/development.md` 指针（QC-34-DEV-POINTER /
    REV-DEV-POINTER 已记录）；本条更正为**已完成**，不补推其他 Batch B 项。
  - `SDD-STRUCT-ALIGN-01` 仍无 owner/expiry；按裁决 10-A 作为新模型试点候选，
    不因本次落地自动关闭。
- 未决：C-3（SPEC-0002 验收状态表 + 开放 AC 迁移 + 13 个 addendum 合并为新 ADR）、
  C-4（归档/停用旧文件）、D-2（台账归档，不重排）。C-3 的新 ADR 仍需独立审查与人类批准。
- 本变更未 commit、未 merge；按裁决 2-B，合入由人类执行。

### 2026-11-27 human decisions: ROLLOUT 11 rulings; B-1..B-6 state alignment

- `Actor: lead`; `Reviewed-by: human (Director rulings 2026-11-27)`; `Tier: L2`.
- Rulings: 1=A (external subagent reviewer with runtime); 2=B (human merge retained);
  3=A (new flow + state alignment first); 4=B (archive + stubs); 5=A (AC-72 split);
  6=B (AC-27 back to AC-REF); 7=B (correct the 11-27 reorder; T2->T3->T4->T5->P0);
  8=B (archive only, no reorder); 9=A (lift freeze for Phase 0-R + T2.4 remaining;
  no merge/Release); 10=A (next contract change as pilot); 11=A (no emergency path).
- Drafts adjusted but **not landed**: constitution 16 = three human approval points
  (Spec, merge, Release); AGENTS section 1 / step 7 merge by human; AGENTS section 2
  = no emergency path. C-1 landing still needs explicit final approval.
- **B-1**: AC-60/63 are **DEFERRED**, not AC-REF: no `TestAC60/63` exists; only
  production comments (`tcc_precheck.go:97`, `legacy_resolution.go:14`,
  `refund_admission.go:679`). The AC-REF wording at `:465/:473/:495` is superseded;
  `ac-coverage.tsv:29,32` stays DEFERRED.
- **B-2**: AC-72 split per ruling 5: `AC-72-BACKEND` AC-VERIFIED;
  `REVIEW-REQUIRED-ALERTS-01` / `REVIEW-REQUIRED-ENTRY-01` stay DEFERRED and are
  now prefixed AC-72-ALERTS/ENTRY in `ac-coverage.tsv:47-49`. AC-72 is not overall
  verified.
- **B-3**: AC-27 reverted to **AC-REF** per ruling 6; `tasks.md:57` remains in force;
  the C+A waiver is not formalized (`ac-coverage.tsv:41` updated).
- **B-4**: **PAY-SETTLE-01 freeze lifted** for Phase 0-R closeout + T2.4 remaining
  scope only (ruling 9 A); retroactively ratifies the T2.4 completed work (scoped
  PASS); no merge/Release; other frozen scopes unchanged. This updates the freeze
  state recorded at `:175/:198/:741/:789/:982` only to that extent.
- **B-5**: **11-27 reorder corrected** (ruling 7 B): authoritative order is
  T2->T3->T4->T5->P0 closure (`:502-509`); the "resume Phase 0-R first; T2/T3/T4
  behind P0" entry at `:620-625` is superseded; `tasks.md:26` corrected.
- **B-6**: SPEC-0002 `:9/:24` and ADR `0007-amd1-addendum-ac50-nowaybill`
  `:7/:8/:98` parent-frozen notes synced to "historical parent; freeze/de-freeze
  state in ledger (2026-11-27 B-4)"; historical records `0002:1011/:1038` untouched.
- **取代**: OPS-P5b/P5 的 AC-27 AC-VERIFIED（-> AC-REF）；T2a/T2.4 的 AC-60/63
  AC-REF（-> DEFERRED）；resume-Phase-0-R-first 重排（-> T2->T3->T4->T5->P0）；
  2026-11-25 `:966-968` 的 Developer pointer "not done"（-> 已完成，见
  `prompt/Developer.md:4`）。历史条目原文不改写。
- A-2/C/D remain pending. C-1 has been approved and landed (see the
  `GOV-CONVERT-01` entry above); C-4 will move retired files to `docs/archive/`
  with stubs; D-2 archive only.
- Line refs in this entry are the pre-insertion snapshot; this insertion shifts
  later lines. `docs/convert/A1-SELFCHECK.md` line refs are bound to HEAD `9e81497`.

### 2026-11-27 T2.4 scoped PASS

- Reviewer `bda09a7` (base `d428dfd`, head `deeac4a`, code `4016c15`):
  T2.4 scoped PASS, `NOT RELEASE`.
- COMPENSATING owner/SLA/queue/expiry, backlog/oldest gauges, 4 stable alert
  keys, 7-day EXPIRED_FINANCE close/escalation, REJECTED no-reopen PASS;
  0 release/0 Payment/0 EXPIRED_REFUND; no new worker/migration/RPC.
- AC-63/INV-46 remain **AC-REF**: COMPENSATING→EXPIRED_REFUND needs T3
  release completion/receipt/multi-group. ADR legacy ruling DRAFT pending
  human. T2.5/T2.6 not done.

### 2026-11-27 T2.4 legacy-release ruling QC READY

- QC: no `legacy_expired` release source; T3 owns release completion/receipt;
  T2.4 stays COMPENSATING prefix (0 release/0 Payment/0 EXPIRED_REFUND);
  AC-63/INV-46 remain AC-REF.
- MAJOR: COMPENSATING long-stay needs owner/SLA/manual queue/expiry and a
  concrete alert key; 7-day EXPIRED_FINANCE and REJECTED can be done
  independently (must not impersonate AC-63).
- Next: monitoring/queue + 7-day/REJECTED, then T2.4 scoped review; no
  release/Payment before T3.

### 2026-11-27 T2.4 BLOCKED: legacy_expired release source

- Zero gate MAJOR fixed (system_zero resolves historical zero before the
  short-circuit; no direct CONFLICT).
- T2.4 worker/queue implemented (`e6d8bf9`), but `legacy_expired` is not a
  legal INV-44 release source (`refund_release.go:32-48` closed enum; events
  lack source/reason). Worker stops at `COMPENSATING` with permit REVOKED,
  0 release/0 refund; `EXPIRED_REFUND`/7-day close/`REJECTED` not delivered.
- Needs Architect ruling: add `legacy_expired` release source/reason
  (additive event value, no RPC/migration) or explicit deferral to T3.
- No AC-60/63 VERIFIED.

### 2026-11-27 T2a PASS scoped; zero gate MAJOR

- Reviewer `b907d26` (base `106e5ac`, head `b73ac51`, code `1e57c15`):
  T2.1–T2.3 PASS scoped, `NOT RELEASE`; AC-60/63 stay AC-REF.
- MAJOR: `source=system_zero` bypasses the legacy gate
  (`refund_admission.go:341`); historical zero with empty settlement is not
  resolved and zero refund hits `REFUND_ADMISSION_CONFLICT`. Fail-closed but
  AC-60 zero path not closed; fix before T2.4/T2.5.
- Next: zero gate fix + T2.4 queue/SLA/COMPENSATING/admin entry.

### 2026-11-27 human resequencing: T2 -> T3 -> T4 -> T5 -> P0 closure

- Director chose option B: implement T2 (historical PAID/legacy), T3
  (line/freight/commitment release), T4 (fail-closed/shadow) first; then T5
  full AC-39…68 (including the 4 P0 blockers AC-39/41/57/59 and 17 test gaps);
  then P0 closure.
- Phase 0-R scoped closeout is unchanged. AC-62..68 stay deferred until their
  phases land. No merge/Release.

### 2026-11-27 T5.0 audit verified; 4 P0 blockers

- Reviewer `528fe5d`: audit acceptable baseline with corrections; `NOT RELEASE`.
- Evidence-ready: AC-50/67/68. TEST-GAP 17. FEATURE-GAP corrected:
  T2 = AC-40/60/63; T3 = AC-62/64; AC-61 = T5.3/ops.
- **P0 blockers**: AC-39 (callback amount mismatch -> no full refund,
  `payment/service.go:297-300`/`order/service.go:1341`); AC-41 (MarkPaid
  Conflict, no non-effective refund, `order/service.go:604-612`); AC-57
  (`ConfirmReceipt` does not reject `CREATED`, `fulfillment/delivery.go:386`);
  AC-59 (no `EVENT_OUT_OF_ORDER`; audit citation too broad).
- These must be fixed/tested before T5/P0 closure. T5.0 is not AC-VERIFIED.
- Decision needed: fix blockers under parent + T5 test gaps vs T2/T3/T4 first.

### 2026-11-27 T5 start: evidence audit

- Human: continue after Phase 0-R closure.
- T5.0 audit AC-39…68: classify each as EVIDENCE-READY (test exists),
  TEST-GAP (code exists, test missing), or FEATURE-GAP (needs T2/T3/T4).
  AC-62…68 depend on unimplemented T2/T3/T4; P0 closure scope must account.
- No code yet.

### 2026-11-27 PAY-SETTLE-01 Phase 0-R CLOSED (scoped)

- Reviewer `67fd571`: Phase 0-R **CLOSED (scoped)**, `NOT RELEASE`; not P0
  closure/merge.
- R0.1 PASS; R0.2 C-01..C-05 PASS; R0.3 **AC-27 AC-VERIFIED** (waiver C+A),
  AC-27A via R0.3-R1c, regression/INV/12 orphan checked; R0.4 Addendum
  ACCEPTED, R0.4e-R2 F-1/F-2 CLOSED; R0.5 matrix SPEC v1.10/v1.5 +
  migration scope.
- Architect R-2 now SUPERSEDED/WITHDRAWN (docs-only commit).
- Residual: 9 MINOR, INV-44/46/47/49/50 AC-REF/DEFERRED, AC-61/62 AC-REF,
  AC-72 ALERTS/ENTRY DEFERRED, F7b BLOCKED, UAT/prod maintenance window not
  executed, tasks/evidence-matrix MINOR drift.
- **Not included**: T2/T3/T4.4/T5, P0-A/B/C, Release. Next: T5/AC-39…68.

### 2026-11-27 OPS-P5b PASS; AC-27 AC-VERIFIED; PAY-OPS-REPLAY-01 complete

- Reviewer `bbcf8f6` (base `7b7beb5`, head `a8fc1b3`): OPS-P5b PASS scoped;
  **AC-27 upgraded to AC-VERIFIED** under human waiver C+A.
- Evidence: real `RunAfterSaleConsumer` (REFUNDING→REFUNDED; admission stays
  REVIEW_REQUIRED/PENDING_REMOTE; no release; duplicate idempotent; invalid
  JSON DLQ) + TCC re-drive (CANCELLED→PRE / SHIPPED→POST; exactly one
  RefundBatch). Test-only; `migrations` empty.
- AC-27A via R0.3-R1c only; AC-61/62 remain AC-REF; AC-72/F7b not upgraded.
  NOT RELEASE.
- `PAY-OPS-REPLAY-01` complete (manual runbook + drill + convergence; tool
  design withdrawn). Parent Phase 0-R R0.3 now satisfied; next: Architect
  withdraws R-2, Phase 0-R closeout, then T5.

### 2026-11-27 OPS-P4 PASS scoped; AC-27 AC-REF (convergence gap)

- Reviewer `b1e8409`: OPS-P4 runbook/drill PASS scoped, `NOT RELEASE`.
  Drill `5ef1b35` at HEAD `e3c68a2`: MySQL 8.4.13 + NATS 2.10.29,
  EXIT=0 / 60 OK; D1–D11, wrong-group, re-Refund, forged/expired/same-person,
  concurrency, zero duplicate/release/permit change PASS.
- **AC-27 not upgraded**: D1 uses a synthetic DLQ wrapper (no real Order
  `aftersale.completed` consumer); D5 only verifies re-arm CAS, not
  worker/TCC re-drive to PRE/POST. Terminal ② abandon covered; ① and LIVE
  convergence lack real assertions.
- Minimal remaining: test-only convergence with existing consumer/worker
  (no production code); owner Developer/ops/AfterSales, expiry 2026-12-31.

### 2026-11-27 human waiver: manual residual risks accepted

- Scope: `PAY-OPS-REPLAY-01` / `AC-27-NOWB-01` / `AC-27-LIVE-01`; expiry
  2026-12-31.
- Director explicitly accepts the manual-procedure residuals:
  1. S6 group proof relies on the Fulfillment authorizer signing/archiving
     `order_no+group_no` evidence (shipments alone has no group_no).
  2. No code guard against a user `Refund` reopening an abandoned task; ops
     ticket/named-account procedure must intercept.
  3. `authorization_id`/`owner_approval_ref` may be ticket-reported.
  4. No technical non-repudiation; named accounts, dual control, tickets and
     DBA audit compensate.
- This waiver does not waive Release/merge or any other AC; runbook + drill +
  Reviewer may mark AC-27 AC-VERIFIED per C+A.

### 2026-11-27 human waiver: manual ops procedure replaces tool entry

- Scope: `PAY-OPS-REPLAY-01` / `AC-27-NOWB-01` / `AC-27-LIVE-01`.
- Decision (C+A): no `cmd/ops`, no `ops_action_authorizations`, no `0024`, no
  new RPC. A **manual ops runbook** (DBA/ops SQL + dual control + audit log)
  is the entry; a **disposable MySQL 8.4 drill** of DLQ replay and NOWB
  re-arm/abandon, bound to HEAD and verified by Reviewer, may mark **AC-27
  AC-VERIFIED** (acceptance-semantics waiver).
- Expiry 2026-12-31; does not waive Release/merge or any other AC; the
  withdrawn `cmd/ops`/0024 design is not a contract.
- Affected invariants: INV-45 (no release/permit restore), AC-61/62;
  manual procedure must preserve canonical `last_error` and audit.

### 2026-11-27 change freeze: PAY-OPS-REPLAY-01 opened

- Human chose option A: new bounded L2 `PAY-OPS-REPLAY-01` for real AC-61
  manual replay + NOWB manual abandon/re-arm. `PAY-SETTLE-01` paused again.
- Plan/Tasks: `docs/lead/PAY-OPS-REPLAY-01/`. No code until Architect design,
  QC, and human Plan approval. No merge/Release.

### 2026-11-27 Phase 0-R closeout FAIL: NOWB open, LIVE blocked

- Reviewer `e02677d`: `AC-27-NOWB-01` **OPEN**, `AC-27-LIVE-01` **BLOCKED**;
  AC-27 remains **AC-REF**; Phase 0-R not closable, `NOT RELEASE`.
- NOWB: O19/O22/O20 + runbook only prove retain/audit/re-arm simulation; no
  real S6 evidence registration or ops replay entry; no AC-62 manual-abandon
  entry (AC-62 still AC-REF).
- LIVE: T6c is a NATS consumer test, not an AC-61 manual replay entry; proto
  has only internal `AdmitRefund`; `ReplayDLQ` is seckill-only; runbook is text.
- Decision needed: new bounded L2 for ops replay/AC-62 manual abandon, or
  defer NOWB/LIVE to T3 and accept AC-27 AC-REF, or redefine Phase 0-R.

### 2026-11-27 human decision: resume PAY-SETTLE-01 Phase 0-R

- Director: resume the parent and continue **Phase 0-R closeout** first:
  `AC-27-NOWB-01` / `AC-27-LIVE-01`, then T5 evidence. T2/T3/T4/P3/Release
  remain gated behind P0 closure (original constraint).
- No merge/Release; WIP only.

### 2026-11-27 R0.3-R1c PASS (scoped); AC-27 still AC-REF

- Reviewer `ce10714` (base `ce0f6b4`, head `36b747a`, code `9a57d01`):
  R0.3-R1c PASS scoped, `NOT RELEASE`.
- `groupIsClosed`: fixed order; branch 5 = `CONSUMED` + EXISTS
  `order_refund_admissions.result='PRE_SHIP_REVOKED'`; `refund_state`/
  `reservation_state` do not participate; **no `orders.status` write**;
  late SHIPPED/RECEIVED has no contract conflict (S3 shipped -> open;
  no event -> keep closed, no PRE).
- AC-27 remains **AC-REF**: `AC-27-NOWB-01` / `AC-27-LIVE-01` still open.
  `AC-50-FIX-01` write path is delivered by the L2 (AC-50 AC-VERIFIED).
- Architect DRAFT AC-27 R-2 (writes `orders.status`) conflicts with SPEC
  v1.5 -> must be withdrawn/errata; Phase 0-R remains open.

### 2026-11-27 parent resume: PAY-SETTLE-01 R0.3-R1c

- Human direction: after L2 completion, resume the paused `PAY-SETTLE-01`
  **only for R0.3-R1c** (AC-27A steps 5/6). No other parent scope reopens.
- Scope: Order read model closes an unshipped group on
  `order_refund_admissions.result=PRE_SHIP_REVOKED`; shipped/received guard
  first; `refund_state`/`reservation_state` do not participate; **no
  `orders.status` write**, no migration/schema, no new RPC/worker.
- WIP only; no merge/Release. Reviewer scoped verdict required.

### 2026-11-27 L2 residual PASS; PAY-AC50-REVIEW-01 complete (scoped)

- Reviewer `91b7310` (base `b731a9f`, head `d119894`, code `d90daf0`):
  T7-R residual PASS; T0–T8 scoped PASS maintained; `NOT RELEASE`.
  T7-R2/R3 PASS; F7b full-chain `[BLOCKED]` accepted (no cross-service
  harness; static guard forbids single-test cross-service imports; no fixture
  substitution), residual owner Lead/QA expiry 2026-12-31.
- Ledger unchanged: AC-50/69/70/71/72-backend AC-VERIFIED; AC-51/61/62/65/67
  AC-REF; AC-72 ALERTS/ENTRY DEFERRED; AC-27 AC-REF; AC-21/25/31/32 AC-REF.
- **PAY-AC50-REVIEW-01 is complete (scoped)**: T0–T8 + T7-R; no deploy/merge/
  Release. Parent `PAY-SETTLE-01` resumes R0.3-R1c (AC-27A 5/6) next.

### 2026-11-27 human direction: continue L2 residual hardening

- Director: keep advancing `PAY-AC50-REVIEW-01`; return to paused
  `PAY-SETTLE-01` only after the L2 residuals are closed.
- Scope (test-only, no new behavior/contract): F7b late `order.paid`
  placeholder, TCC 1024 truncation, app 0-row/empty `batch_id`, unknown probe
  status, O19 `refund_applications` 0 rows / O1b other-batch isolation.
- Parent stays frozen; no deploy/merge/Release.

### 2026-11-27 PAY-AC50-REVIEW-01 final PASS (scoped) + handback

- Reviewer `f0420fc` (base `3ce0d7d`, head `c8f6c9c`): **T0–T8 PASS
  scoped**, `NOT RELEASE`, no merge/P0 closure.
- Ledger: AC-50/69/70/71/72-backend **AC-VERIFIED**; AC-51/61/62/65/67
  **AC-REF**; AC-72 ALERTS/ENTRY **DEFERRED**; AC-14 remains AC-VERIFIED.
- Residuals: `REVIEW-REQUIRED-ALERTS-01`/`ENTRY-01` deferred; F7b late
  placeholder not constructed; W8 = AC-27-LIVE-01; AC-21/25/31/32 limits.
- Handback (L2-R3): `PAY-SETTLE-01` may resume R0.3-R1c (AC-27A 5/6) and the
  rest of Phase 0-R; AC-27 remains **AC-REF**; no release/merge.

### 2026-11-27 T3-T6 re-review PASS; freeze lifted

- Reviewer `18933e9` (base `72575f6`, head `9d9f79d`, code `c097e98`):
  T3-R1 BLOCKER CLOSED; T3–T6 PASS scoped, `NOT RELEASE`.
- fresh Tx1/Tx2 share `persistTCCReviewTx`; batch/app mirror require
  `RowsAffected==1` (0-row rolls back); `source=tcc` partial/legacy produce
  `MANUAL_HOLD` + canonical `last_error` + app REVIEW + token, 0 probe/payment.
- Residual: T6b command package (fulfillment), app 0-row/empty batch_id lacks
  an independent test, F7b late placeholder `[未覆盖]`, W8 = AC-27-LIVE-01.
- change_id unfrozen for T7/T8 only; no deploy.

### 2026-11-27 T3-T6 scoped FAIL: TCC fresh REVIEW bypass

- Reviewer `e4e8a61` (base `30c6f16`, head `849cf78`, code `13a586a`+`f1946e7`):
  **T3/T4 FAIL**, T5/T6 PASS, `NOT RELEASE`.
- BLOCKER: fresh Tx1 REVIEW (`refund_admission.go:544-551`) bypasses the T1c
  hold gate (`writeAdmissionTx2:940-970`). For `source=tcc` partial/legacy
  unclassifiable, REVIEW admission+commitment is committed while the batch
  stays `ADMITTING`, the application is not mirrored, `last_error` is absent,
  and `admitTccBatch` returns `Unavailable` instead of
  `TCC_BATCH_REQUIRES_MANUAL_HANDLING` (AC-71/ADR W12). T3/T4 tests only cover
  `source=aftersale`.
- **change_id frozen** until T3-R1/T3-R2 + re-review. T5/T6 remain PASS.
- Residual: T6b test lives in `fulfillment`, not `order`; F7b/W8 not
  constructed; TCC 1024 truncation not triggered.

### 2026-11-26 T1/T2 PASS (scoped)

- Reviewer `30c6f16` (base `ff0d912`, head `7b28024`, code `ff7a847`):
  T1/T2 PASS scoped, NOT RELEASE. MySQL 8.4.13 order/aftersales tests +
  `-race` PASS; T0 regression PASS; no forbidden TCC files/migrations/RPC.
- Residuals (not FAIL): T1c batch/app mirror `RowsAffected` unchecked;
  O19 0-row / O1b other-batch / TCC 1024 truncation not asserted; O16 audit
  simulated. T3–T6 not landed -> no standalone deploy.
- Next: T3/T4 (partial, zero, legacy) + T1c hardening.

### 2026-11-26 T0 release unit PASS (scoped)

- Reviewer `84f5acd` (base `a34c845`, head `c796580`, code `e374384`):
  **T0/T0b/T0c PASS (scoped)**, `NOT RELEASE`, no AC-VERIFIED.
- Real MySQL 8.4.13 independent schemas: order `TestAC50Review` PASS
  (11.965s), `-race` PASS (13.370s); fulfillment `TestCancelIfCreated` PASS
  (2.011s); existing `TestAdmitRefund*` PASS; `make proto-guard` OK;
  `migrations` diff empty.
- `NO_WAYBILL` does not create a row; REVIEW persisted with typed details,
  existing-first; no PRE/POST outbox/release/`Payment.Refund`; Tx2 CAS
  `state='PENDING_REMOTE' AND result <> 'REVIEW_REQUIRED'`; REVIEW cannot be
  washed to PRE.
- Residual: unknown/future probe status lacks a T0 test (code defaults
  `Unavailable`); partial `CONSUMED+NO_WAYBILL` still writes REVIEW until T3;
  T1/T2/T3 not landed -- **must not deploy alone**. Wire token is
  `NO_WAYBILL` (ADR `<NO_WAYBILL>` is a placeholder); record in Brief.

### 2026-11-26 human approval C-5 + P5

- Director approved **ADR 0.6 errata** (`ADR_APPROVED`, C-5) and the
  `PAY-AC50-REVIEW-01` **Plan/Tasks** (`P5`) on 2026-11-26.
- Implementation authorized: T0+T0b+T0c release unit first, then T1–T8.
- No merge/Release/AC-VERIFIED implied; PAY-SETTLE-01 remains frozen.

### 2026-11-26 PAY-AC50-REVIEW-01 Plan review: four roles READY

- Plan/Tasks rewritten after the first review round: atomic tasks
  (T0/T0b/T0c/T1a-e/T2a-c/T3/T4a-b/T5a-c/T6a-d/T7/T8), ADR §10 F/O/W
  binding (incl. W13/O11d), per-task test file/command/expected/HEAD,
  AC-72 three-part ledger, T5c ops runbook.
- **Architect P1b READY** (R-1..R-6 closed; ADR 0.6 allows the selfcheck file,
  `api/gen` only via `make proto`).
- **QC P2c READY** (0 BLOCKER; 1 non-blocking TODO).
- **Developer P3d READY** (commands/env/ops runbook closed).
- **Reviewer P4b READY** (evidence paths/AC-72/tsv boundary; C-5 pending).
- **ADR 0.6 errata: QC R9 READY (0 BLOCKER)**; needs human errata
  `ADR_APPROVED` (C-5). Plan needs human **P5** approval. No code.

### 2026-11-26 human approval C-4: SPEC-0002 v1.11 + ADR AC-50 0.5

- Director **approved** SPEC-0002 v1.11 (`SPEC_APPROVED`) and ADR-0007 AC-50
  Addendum 0.5 (`ADR_APPROVED`) on 2026-11-26, and authorized the four-role
  Plan review (`PAY-AC50-REVIEW-01`, L2-P1..P4).
- This is **contract approval only**: no implementation/merge/Release/
  AC-VERIFIED. Plan P5 (human Plan approval) is still required before code.
- QC basis: `QC-PAY-SETTLE-01-AC50-NOWB.md` R8 READY (0 BLOCKER; 2 TODO).

### 2026-11-26 PAY-AC50-REVIEW-01 contract closure: QC R8 READY

- SPEC-0002 **v1.11** (`d159300`, in-review), ADR-0007 AC-50 Addendum
  **DRAFT 0.5** (`b2491cb`), impact analysis DRAFT 0.3 (`3ae8e79`), Plan
  (`e725a5c`) : QC round 8 **READY, 0 BLOCKER** (`QC-PAY-SETTLE-01-AC50-NOWB.md`).
  - `REVIEW_REQUIRED` (15 chars) fits existing columns; **no migration**.
  - Single authority `order_refund_admissions`; normal after-sales mirrors
    `after_sale_refund_tasks.last_error`, task stays `PENDING` (no auto
    `FAILED_PERMANENT`); TCC parks the batch in `MANUAL_HOLD`; partial amount
    never calls `CancelIfCreated`; replay returns the persisted decision before
    any Payment read; audit reuses the admission row (no new table).
  - Canonical: `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>`; no
    `arm`; overflow appends ` truncated=1`.
- 2 non-blocking TODOs: R8-T1 version stamps; R8-T2 AC-72 three-part ledger
  rows.
- **Next**: human `SPEC_APPROVED`/`ADR_APPROVED`, then Plan review
  (L2-P1..P4) and human Plan approval. **No code**; PAY-SETTLE-01 stays frozen.

### 2026-11-26 change freeze: PAY-SETTLE-01 paused; PAY-AC50-REVIEW-01 opens

- Director chose option (a): the AC-50 `CONSUMED+无运单` fix becomes an
  **independent L2 change**, `change_id=PAY-AC50-REVIEW-01`
  (Plan/Tasks: `docs/lead/PAY-AC50-REVIEW-01/`).
- **PAY-SETTLE-01 remains frozen** (D-LC-5): no further managed code or
  ledger state under the parent; AC-27 stays **AC-REF**; Phase 0-R stays open.
- New L2 starts at **contract closure**: QC R4-B1/B2/B3 + 3 MAJOR (PO/Architect
  -> QC round 5 -> human `SPEC_APPROVED`/`ADR_APPROVED`), then the five-party
  Plan review and human Plan approval; **no code before that**.
- One `change_id` at a time: PAY-SETTLE-01 resumes only after the L2 handback
  (`L2-R3`, AC-27A steps 5/6). No merge/Release/P0 closure.

### 2026-11-26 SPEC-0002 v1.11 / ADR 0.4: QC round 4 BLOCKED

- PO v1.11 (`c2bca33`, in-review) adds `REVIEW_REQUIRED`
  (`result=REVIEW_REQUIRED`, `state=PENDING_REMOTE`; application
  `admission_result=REVIEW_REQUIRED`, `PENDING`; TCC batch `MANUAL_HOLD`) plus
  S20/INV-51/52/AC-69…72 and an impact section.
- Architect DRAFT 0.4 + impact analysis (`43bbe5a`): **no schema migration**
  (15-char value fits `VARCHAR(32)`/`VARCHAR(24)`); TCC is the largest change
  (`tcc_settlement.go`/`tcc_admission.go` must accept REVIEW and park the batch
  in `MANUAL_HOLD`); partial amount never calls `CancelIfCreated` (three-way
  classification from Order-owned facts); idempotent replay before
  `Payment.GetIntent`; Tx2 needs a `state='PENDING_REMOTE'` CAS.
- QC round 4 (`6401cba`) BLOCKED: R4-B1 normal after-sales has no
  `refund_applications` row (conflicts with S20/AC-69); R4-B2 zero-amount
  no-persist exception missing (conflicts with AC-67/69); R4-B3 AC-69/72
  require retaining the original review record but ADR §8.3 keeps
  `REVIEW-AUDIT-01` deferred. Majors: stale ADR 0.3 reference; AC-72 user/ops
  entry separate; `tcc_*.go` PO authorization is not human implementation
  approval.
- Implementation **not approved**. Needs scope decision: fix the three
  blockers + human `ADR_APPROVED`, or narrow the amendment / keep the PRE
  deviation.

### 2026-11-26 human decision: PO authorized to upgrade SPEC-0002 (AC-50 outcome)

- Director authorized the PO to issue a **SPEC-0002 v1.11** amendment adding a
  **bounded, persisted CONFLICT/PENDING admission outcome** (and its TCC
  handling), so AC-50 `CONSUMED+no-waybill` no longer silently maps to PRE.
- Scope: AC-50/51/62/65, INV-45/46; impact analysis (business + technical)
  must be produced and QC-reviewed before implementation. No code until
  accepted; no merge/Release/AC-VERIFIED implied.

### 2026-11-26 AC-50-FIX-01 BLOCKED (QC round 2)

- Architect amendment `docs/adr/0007-amd1-addendum-ac50-nowaybill.md`
  DRAFT 0.2 (`c9be297`) is **not accepted**. QC round 1 BLOCKED (Tx1 already
  wrote admission/commitment/outbox; `GetIntent`; W6), round 2 BLOCKED
  (`QC-PAY-SETTLE-01-AC50-NOWB.md`, `3366e9c`): R-2a `ListShipments` pre-read
  conflicts with accepted ADR Rev 2.3 §5.2.2 ("CONSUMED does not depend on an
  out-of-tx pre-read"); same-`refund_id` replay with persisted POST can return
  PARTIAL/Unavailable on timeout (AC-50 `0002:227`); B-3 partial
  (`order.paid` fallback can still create a placeholder).
- No code was written. AC-27 remains **AC-REF**; AC-50-FIX-01 remains open.
- Options for human/Architect: (a) amend ADR §5.2.2 via a versioned
  ADR revision (bounded pre-read + deadline + persisted-result replay), then
  re-QC; (b) keep the current PRE mapping as an explicit deviation with
  expiry 2026-12-31; (c) redesign the admission protocol as a separate L2
  change_id. No option may be chosen by Lead/Developer alone.
- **2026-11-26 human decision: route A** — drop the `ListShipments` pre-read;
  accept the Tx1-written artifacts and route `NO_SHIPMENT` to
  `REFUND_ADMISSION_CONFLICT` + manual `AC-27-NOWB-01` queue (commitment
  stays RESERVED, outbox placeholder kept, no PRE/refund/release). No
  ADR §5.2.2 exception. Architect to revise DRAFT 0.3, then QC round 3.
- **2026-11-26 AC-50-FIX-01 FROZEN (QC round 3 BLOCKED)**: route A as drafted
  still cannot be implemented without a contract change —
  R3-B1 `result=CONFLICT` is not a legal TCC admission result
  (`tcc_settlement.go:847-876`; `tcc_*.go` outside the amendment's
  allowed_paths); R3-B2 partial `CONSUMED+CANCELLED` makes `CancelIfCreated`
  cancel the `CREATED` waybill before returning `PARTIAL`, violating
  AC-50 `0002:217`; R3-B3 route A did not authorize persisting `CONFLICT` or
  "same `refund_id` no re-probe" (SPEC-0001:645 only allows PRE/POST;
  `CONFLICT-SPEC-SYNC` was only a stop condition); R3-B4 "application stays
  retryable" vs "no re-probe" contradiction.
- **Decision needed**: PO/SPEC-0002 must either (a) add a bounded, persisted
  CONFLICT/PENDING admission outcome and authorize TCC handling of it, or
  (b) keep the current PRE mapping as an explicit deviation to 2026-12-31.
  No Lead/Developer patch. AC-27 stays AC-REF.

### 2026-11-26 human decisions: F-2 branch A, UAT as prod, AC-50-FIX-01 start

- Director chose **branch A** for the 0022/0023 decision: restore
  `migrations/order/0022_r04_checks.sql` byte-identically and add
  `order/0023_r04_checks_v2.sql` (replace both checks, `_v2` names, add then
  drop old; no missing-constraint window).
- Director directed that **UAT is treated as prod** for the R0.4 migration
  preflight/evidence and lock/maintenance-window scope until further notice.
- Director authorized starting **AC-50-FIX-01**: ADR §5.3 additive amendment
  first (no-waybill `CancelIfCreated` result; Order maps to
  `REFUND_ADMISSION_CONFLICT`, never PRE), then QC, then implementation.
- No merge/Release/AC-VERIFIED implied.

### 2026-11-26 Reviewer verdicts: R0.4e FAIL, R0.3-R2 scoped PASS

- Reviewer `fd208b2` (base `90b502c`, head `9322e75`, code `c38c54f`):
  - **R0.4e FAIL (scoped)**: F-1 missing `ADD CHECK` lock/MDL/maintenance-window
    evidence (Addendum §2.4/§2.6 R0.4d2); F-2 preflight did not cover
    staging/prod, so branch B is not safe under §2.3.3 fail-closed. Gate/code
    behavior, four-package `go test`/`-race`, `verify-r01-migrations.sh`
    (EXIT=0) and compose/Helm render otherwise pass.
  - **R0.3-R2 PASS (scoped)**: AC-27 assertions real and re-run at HEAD;
    AC-27 stays **AC-REF** because AC-27A steps 5/6 are not implemented and
    `AC-50-FIX-01`/`AC-27-NOWB-01`/`AC-27-LIVE-01` remain open. Regression
    AC-14/21/25/27/31/32 re-run; ledger now AC-14 **AC-VERIFIED**, others
    AC-REF; 12 orphan mapped to T5.x/T3.
- INV-44 evidence row missing the missing-reservation metric deferral
  (Lead-owned) is now recorded in `evidence-matrix.md`.
- **Phase 0-R is not closable**: R0.3-R1c, R0.4e F-1/F-2, T2/T3/T4.4, ops
  alarm drill remain. No merge/Release/P0 closure.

### 2026-11-26 R0.4 implementation + SPEC-0001 v1.5 (WIP)

- **R0.4 Addendum accepted** (`c432ed6`; human 2026-11-26; QC R3 READY
  `bd7c239`; 9 MINOR as TODO).
- Developer R0.4 `c38c54f` + selfcheck `9322e75`: 0022 **branch B** (preflight
  showed no apply in reachable envs), 24h equality CHECK, exit 3/4 +
  `mode/approval_id/reason`, compose/Helm/ops wiring (default `paid`),
  `verify-r01-migrations.sh` 0021/0022; real MySQL 8.4.13 + `mall/migrate:dev`
  EXIT=0. **Reviewer R0.4e pending**; not implementation PASS yet.
- **SPEC-0001 v1.5 approved** (`4215b3c`; Director authorized 2026-11-26; QC
  V15 BLOCKED -> R2 READY -> N-1..N-4 confirmed): AC-27 EARS-1 is the S2 read
  model (no `orders.status` write on refund); AC-27A closure predicate uses
  Order-owned `permit` + `order_refund_admissions.result`; §7.5 drift fixed;
  AC-50 `CONSUMED+no-waybill` stays `REFUND_ADMISSION_CONFLICT`.
- Prerequisites registered, **not closed**: `AC-50-FIX-01`,
  `AC-27-NOWB-01`, `AC-27-LIVE-01` (owner/expiry 2026-12-31). Until then
  AC-27 stays **AC-REF, not AC-VERIFIED**, and R0.3-R1c (AC-27A predicate
  implementation) must not be claimed converged.
- Phase 0-R remains open; no merge/Release.

### 2026-11-26 R0.4 Addendum human acceptance (R0.4a3)

- Director accepted `docs/adr/0007-amd1-addendum-r0.4-c04-release.md`
  (DRAFT 0.3 r3, `d5dda12`; ACCEPTED `c432ed6`) on **2026-11-26**, with the
  **9 MINORs tracked as TODO** (S-1/S-4/S-5/S-8 etc.).
- Basis: QC round 3 `READY` (0 BLOCKER / 9 MINOR), report `bd7c239`.
- Scope: Addendum only. Does **not** approve `90b502c`, does not waive
  Reviewer re-review (R0.4b/c), does not approve R0.4d implementation,
  merge, Release or AC-VERIFIED.

### 2026-11-26 human decision: PO SPEC-0001 v1.5 authorized

- Director authorized PO to issue **SPEC-0001 v1.5** to resolve the AC-27
  conflict with §6.6.1/AC-25 (persisted lifecycle vs read-model projection),
  the §7.5:405/:409 drift, and the AC-50 `CONSUMED`+no-waybill deviation.
- QC delta review is still required before implementation; no code until the
  PO/QC/Architect path completes. No merge/Release/AC-VERIFIED implied.

### 2026-11-25 human decision: R0.4 shadow runtime

- Director decision (scoped to R0.4): keep
  `FULFILLMENT_SETTLE_CONSUMER_MODE=shadow` **hard-rejected at startup until
  T4.4** delivers the observe-only dual consumer. R0.4 does not implement
  shadow consumption; the Addendum must record the runtime ruling, the
  fail-closed exit semantics and the T4.4 exit condition. Default stays `paid`.
- Scope: R0.4 only. Does not approve `90b502c`, does not waive Reviewer/
  Architect review, human merge, Release or AC-VERIFIED.

### 2026-11-25 R0.2 FAIL (dated 2026-11-25)

- C-01/C-02/C-03/C-05 contract/evidence PASS; **C-04 FAIL**: release RPC still
  missing (R0.4). B2 AC-27 ledger absent; B3 INV-45..50 evidence/ledger absent
  (R0.3/R0.5). No backdating; Phase 0-R remains open.

### 2026-11-25 T1.7 TEST PASS (scoped)

- Reviewer PASS for `93380d5..2a05c1d` test-only: Sigma mismatch, REFUNDING vs
  Ship, crash resume, partial group failure, legacy no-group G1, shipped POST.
- No production/contract/migration change; no AC-VERIFIED. Non-blocking note:
  test header says "real C-05 migrations" but uses inline mirrored schema; T5
  still needs real-migration rehearsal.
