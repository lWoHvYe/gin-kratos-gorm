# Architecture review tracking

This is the canonical running status ledger. Update it every round with evidence
and remaining gaps. `docs/handoff.md` is a session-boundary snapshot and is only
refreshed when a new conversation is started.

> **唯一运行状态台账（2026-11-25 人类批准）**：裁决、冻结、PASS/FAIL、治理决策以本文件为准；`tasks.md`、`plan.md`、`handoff.md`、协作线程均为视图，冲突时以本文件为准。

Source: 2026-11-20 handoff plus follow-up implementation. `done` means the code
and the stated verification exist; `partial` means a safe subset shipped and a
named gap remains. This file was rewritten after a 2026-11-21 audit because the
old table still described P2#10 as rejected and Phase 7 as not started.

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

## 2026-11-24 human decisions: constitution v1.2.0

- Director approved constitution v1.2.0 and set the hierarchy:
  **Constitution > AGENTS.md > docs/collab/CONVENTIONS.md**.
- PAY-SETTLE-01 human decision B: the existing WIP stays on `main` but is
  **frozen**: no merge to release, no publication, no expansion. Open items:
  P-01 safety net only (TCC per-group admission/Σ still missing), P-04, P-05,
  P-06; AC-39…61 remain deferred debts.
- Ratification gate work: G1 AGENTS/CONVENTIONS hierarchy aligned; G2 G-1
  change_id/Actor/Reviewed-by + G-1 check added; G3 ac-coverage columns added;
  G4 this record. G5 human approval recorded here.
- 2026-11-24: Director approved the PAY-SETTLE-01 remediation Plan and
  decisions D-1..D-5 (audited release, 24h legacy terminal states, AfterSales
  freight-responsibility owner, `order.settled` shadow/default false,
  `order.refund_requested` observe-only). Phase 0 contract authoring only; no
  implementation approval.
- Contract debt: SPEC-0001 AC-27 使用 Given/Then/And，下一次 SPEC-0001 版本升级 MUST
  改为 EARS 并登记基线迁移；在此之前不得把 AC-27 标为 AC-VERIFIED。
- 2026-11-24: C-04 proto/config and C-05 migrations passed QC-16 and merged to main
  (`merge: C-04/C-05 contract and migrations`). Phase 0 exit; Phase 1 implementation
  may start on a WIP branch. No service code, AC-VERIFIED, or Release PASS.
- 2026-11-24: Director approved D-22..D-26 defaults:
  - D-22: 24h terminal — no capture/zero -> `EXPIRED_ZERO`; captured and no shipped group ->
    `EXPIRED_REFUND` (does not write `CANCELLED`); shipped group -> `EXPIRED_FINANCE`;
  - D-23: `EXPIRED_FINANCE` only finance/authorized role; no reopen; max 7 days;
  - D-24: authorized role; >=1000 CNY dual review; full audit;
  - D-25: `RELEASED` quota reusable for the same order/group; new refund still needs a new
    application + admission;
  - D-26: 168h starts at the group's first `UNDETERMINED` for that after-sale; other groups/
    later applications do not reset.
  These are business decisions only; SPEC-0002 v1.8, SPEC-0001 v1.4 and
  ADR-0007 Amendment 1 were approved on 2026-11-24 (see the next section).
  D-27..D-29 are closed by ADR-0007 Amendment 1.

## 2026-11-24 human decisions: governance cost reduction (D-CR-1…D-CR-5)
- Director approved **all recommendations** D-CR-1…D-CR-5 from
  `docs/cost-reduction-proposal.md` on 2026-11-24 (session directive to Lead:
  "已经批准了 D-CR-1 至 D-CR-5 的所有建议"). Recorded here by Lead; the approval
  is the Director's, not the Lead's, and this entry is the canonical approval
  evidence referenced by SPEC-0003.
  - D-CR-1: adopt the three-tier change classification L0/L1/L2 with the proposal
    §3 objective criteria; criteria changes require human-approved Spec versions.
  - D-CR-2: L1 stage approval delegated to **QC + Reviewer joint sign-off**
    (conservative option; not Reviewer-only).
  - D-CR-3: approve QC delta review for small versions, keeping the two mandatory
    full-QC nodes (major version bump, before Release gate).
  - D-CR-4: mis-tiering = stop-the-line + redo at L2 + Reviewer ledger record.
  - D-CR-5: CI gate-triggering paths must include `docs/**`, `prompt/**`,
    `.specify/**`.
- Scope: governance/process only; no business AC/INV change; no waiver of EARS,
  fail-closed, stop-the-line, or red lines R-1…R-6.
- Open: D-CR-6 (root `AGENTS.md` / full managed-path CI coverage) and D-CR-7
  (QC scope inside L1 joint sign-off) remain undecided.
- Tracking: SPEC-0003 (`docs/specs/0003-platform-governance-cost-reduction.md`)
  draft; QC-01 returned BLOCKED (0 BLOCKER / 10 MAJOR / 11 MINOR) and the spec is
  in revision. Not a Spec approval; SPEC-0003 `approved_by` stays null.

## 2026-11-24 human decisions: GOV-COST-01 bootstrap, IV and commit timing

Director decisions on ADR-0008 QC-01 (BLOCKED 4/8/8), recorded by Lead:

- **Bootstrap G-4 (QC-05 Q-02)**: choose **SPEC-0003 v1.4 amendment**. AC-106/112
  bootstrap evidence = legacy **G-1…G-3 PASS** (scoped AC-01…68) plus an explicit
  `G-4 NOT-RUN(not-implemented-at-BASE)` record that does **not** block P0a/P0b;
  G-4 is implemented in P0b and required from then on. NOT-RUN never counts as PASS;
  this is a bootstrap-only, time-boxed, human-approved evidence exception and does
  not authorize Release. Spec v1.4 was subsequently QC-passed and approved
  (see below).
- **Independent Verifier (AC-107/GV-S21)**: held by the **Director**, or a
  per-run **written designee** recorded in GV-S21 (`designated_by` +
  `independence_evidence`); the designee must not author the change.
- **Commit timing**: approved SPEC-0003 remains **untracked until P0a**. P0a lands
  atomically: approved spec + G-3 governance-evidence extension + ledger/base
  seeding. Until then the committed HEAD gate stays green; batch `DEFERRED`
  registration remains forbidden (INV-69/AC-99).
- ADR-0008 Rev 1.1 was approved on 2026-11-24 (QC-02 READY 0/0/7; Q-01…Q-20 20/20
  CLOSED; bound to SPEC-0003 v1.4 approved).
- **2026-11-24 approvals**: Director approved SPEC-0003 v1.4 (QC-05 READY 0/0/7,
  M-1…M-7 editorial; `adr: [0008]` backfilled) and ADR-0008 Rev 1.1 (QC-02 READY
  0/0/7, MINORs N-1…N-7 editorial).
- **2026-11-24 Plan approval + bootstrap authorization**: Director approved the
  `GOV-COST-01` Plan (`docs/lead/GOV-COST-01/plan.md`) and authorized the
  P0a/P0b bootstrap exception: GV-S20 `mode=bootstrap`, approver=Director,
  expiry=2026-12-31, scope=AC-106/AC-112. NOT-RUN never counts as PASS; the
  exception does not authorize Release or AC-VERIFIED and ends when P0b lands G-4.
- **2026-11-25 CI NOT-RUN policy**: Director decided the CI gate job **accepts
  executor exit 4 (NOT-RUN)** as a successful job while recording
  `RELEASE_ELIGIBLE=false`, uploading the gate artifacts and annotating the run;
  **Release/tag workflows still require exit 0**, and `--mode=warn` /
  `continue-on-error` remain forbidden in CI. Rationale: main must stay green
  while governance ACs are verified incrementally, without treating NOT-RUN as
  PASS. Requires ADR-0008 **Amendment 1** + QC + Director approval before T-B7
  wiring; until then P0b is not committed.
- **2026-11-25 scope reduction (Director)**: 门禁**大幅放宽**——「作为一环没必要做那么复杂」。
  `GOV-COST-01` **冻结在 P0a**（已提交 `0d57398`/`3cac054`）；P0b（G-4、Release
  模式、CI 退出码映射、自保护 IV）、P1（分级/联签）、P2（QC delta/scaffold）全部
  **deferred**。未提交的 P0b/CI/Makefile/执行器改动已**回退**；Amendment 1 保留为
  未实现的草案（`docs/adr/0008-governance-cost-reduction-amd1.md`），不作为契约执行。
  运行时门禁保持 G-1/G-2/G-3 + 人类 Release：治理 AC 的 `NOT-RUN` 仅提示、不阻断
  提交；Release 仍由人类放行。不新增 CI workflow，不改 `go.yml`。
- **2026-11-25 gate retirement (Director)**: 门禁工具**整体退役**，不再保留任何脚本/CI
  强制。已删除 `scripts/collab_gates.py`、`scripts/check-collab-gates.sh`、
  `scripts/test-collab-gates.sh`、`scripts/test_collab_gates_p0a.py`；`Makefile` 移除
  `gates`/`gates-strict`/`gates-test` 且 `verify` 不再依赖 `gates`；`.github/workflows/go.yml`
  移除 `collab-gates` job（`verify` 保留）。G-1…G-4 降级为 `CONVENTIONS §7` 的人工检查清单；
  `ADR-0006` 标记 retired，`ADR-0008`/Amendment 1 标记 withdrawn。GOV-COST-01 正式撤回，
  不再推进 P0a 之后的任何实现。测试/构建/迁移/proto 门禁不变；Release 仍由人类放行。
- **2026-11-25 artifact cleanup**: 已删除 GOV-COST-01 的全部产物——
  SPEC-0003、ADR-0008、Amendment 1、9 份相关 QC 报告、`docs/lead/GOV-COST-01/`；
  AC-69…AC-112 从 `ac-baseline.txt`/`ac-coverage.tsv` 移除（台账回到 AC-01…68 / 36 行），
  `docs/specs/README.md` 的 SPEC-0003 索引行移除。仅保留本决策记录与
  `docs/cost-reduction-proposal.md` 作为历史；不再有任何门禁/脚本引用它们。
- **2026-11-25 tiering decision (Director)**: 分级以人工约定写入 `AGENTS.md` §2.7 /
  `CONVENTIONS.md` §2.3，无脚本、无 CI、无新状态字段。**L1 审批 = Reviewer PASS + 人类合入**，
  取代已撤回 GOV-COST-01 的 D-CR-2（QC+Reviewer 联签，从未实施）；**L0 同样保留人类合入**
  （宪法 Governance 4：merge 是人类检查点，Lead/subagent 不得代行）；L2 不减免任何批准点。
  判据变更须人类批准；误判即 stop-the-line、按 L2 重做并记录在此。
- **2026-11-25 delta audit (Governance 5)**: 治理基线变更——AGENTS.md §2.7 与
  CONVENTIONS.md §2.3 新增 L0/L1/L2 分级；宪法 **v1.2.3**（v1.2.2 Workflow 1 L0/L1 路由、Governance 4 适用时检查点；
  v1.2.1 门禁退役；v1.2.3 历史记录与人工清单口径）；5 份角色 prompt 增加分级口径并新增
  `prompt/Lead.md`；协作闸门脚本/CI 已删除。所有 Lead/subagent 必须重新读取 AGENTS、
  宪法、CONVENTIONS 与自身 prompt，不得依赖旧摘要。
- **2026-11-25 delta audit (AGENTS 角色矩阵 + §2.8)**: §1.0 Lead 限定 L2 Plan 并提议 Tier；
  §1.4/§1.5 补 L1 口径与全级别 Tier 确认；新增分级角色动作总括；§2.8 治理变更最小化与撤回。
  **收益**：角色动作与分级对齐、防止再次无界扩张；**停止条件**：本轮为最后编辑，不再新增治理机制。
- **2026-11-25 §2.8 收口**: 撤回清理由各产物 owner 执行、仅删本次新增行；多文件同步限于首个可验证增量；安全/合规/数据完整性可豁免可测收益；收益与停止条件记录于 review-tracking 或变更输入。

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

### 2026-11-25 docs IA: handoff / development split

- `docs/handoff.md` is now a session-boundary snapshot only; durable local
  environment/test commands moved to `docs/development.md`.
- `README.md` new-session entry points to `review-tracking` / `roadmap-status`
  and `development.md`, with handoff explicitly marked snapshot.
- References checked: `review-tracking` header still valid; the ADR
  `...-c04-c05-handoff.md` is a different contract-input file and untouched.
- Remaining pointer: `prompt/Developer.md` should link `development.md` and say
  handoff is not a rule source; this is an L2 light-path prompt edit for
  Architect → QC → human → Reviewer, not done in this L0 commit.

## 2026-11-25 legacy cleanup decisions (Batch A/B/C)

- **D-LC-1**: approved Batch A (L0), including A-2a's sole-running-ledger declaration.
- **D-LC-4**: C-1.1 — the six ACs expiring 2026-12-15 are decided by 2026-12-08.
- **D-LC-2 / D-LC-3**: Batch B 的治理文本项（B-1/B-3/B-4/B-5）走**轻路径**
  （Architect -> QC -> 人类 -> Reviewer；不建 Spec/ADR/Plan）；**B-2 的 ADR-0007
  合并是独立 ADR 修订**（Rev 2.0 作为合并版批准，Rev 2.1 为其增量），不属轻路径。
- **D-LC-5**: Phase 0 sign-off gap — **retrospective review 2026-11-25**: Developer
  `RETRO PASS WITH CONDITIONS`; Reviewer `RETRO FAIL`. Phase 0 exit unsupported:
  T0.7 five-party sign-off false (no Reviewer dated verdict), C-05 real migration
  tests absent, missing `chk_switch_settled_expiry`, fail-closed config
  unimplemented, no C-04 release RPC. No back-dating; remediation is `tasks.md`
  Phase 0-R (R0.1..R0.5) before P0 closure/Release. PAY-SETTLE-01 remains frozen.
  **Closed 2026-11-25**: Director accepted the historical Phase 0 deviation; no
  back-fill; Phase 0-R stays as a precondition for P0 closure/Release.
- **K-1 (C-1)**: C-04 RefundBatch is **accepted (2026-11-24)**; merge into
  ADR-0007 Rev 2.1; fix the DRAFT wording in the c04 rulings/draft headers.
- **K-2 (C-2)**: T1.6 payment-level cancel **supersedes A-08's absolute wording**;
  A-08 governs only `order.refund_requested` observation.
- **K-3**: constitution **v1.3.0** adds the governance-text L2 light path
  (ADR revisions excluded).
- **K-4**: ADR-0007 Rev 2.0 approved as the merged version; Rev 2.1 is its delta.
- **K-5**: new deferral `thread` format = `review-tracking/<change_id>`.
- **K-6**: retired `G-4` / deleted `--release --range` prerequisites in ADRs are
  replaced by "SPEC §7 + five-party terminal states + Reviewer manual check +
  review-tracking record（协作闸门已退役）".
- **K-7**: pair README/threads/index archive edits are **L0 non-normative
  companion cleanup**, not light-path L2; the light-path L2 scope for B-1 is the
  `CONVENTIONS.md` rule change only. No allowlist expansion is needed.
- **2026-11-25 Lead Brief 收口**: `prompt/Lead.md` 用 6 要素清单替代 9 行模板；`CONVENTIONS §3` 增 `forbidden_paths`/`stop_conditions`/`evidence_and_output` 并声明 Brief 是其 View；收益=消除三处重复，停止条件=不再新增模板；Reviewer PASS。
- **K-8**: Director chose a **PO amendment** (not a waiver) for SPEC-0002:
  v1.10 replaces `G-4` / `--release --range` in AC-66/§5.9/D-27/§9/§12 with the
  K-6 human gate; QC-32 READY 0/0/12; separate human approval + separate commit.

## 2026-11-24 SPEC/ADR approvals: SPEC-0002 v1.8 / SPEC-0001 v1.4 / ADR-0007 Amendment 1

Director approved on 2026-11-24, and PO applied the approved drafts to canonical
`docs/specs/`:

- **SPEC-0002 v1.8** — payment settlement & fulfillment admission. Approved
  `docs/specs/0002-payment-settlement-admission.md`; adds S15…S18, INV-45…49,
  AC-40 narrowing, AC-62…66, E22…E31, D-22…D-29. Historical v1.7 text is retained
  in the change log.
- **SPEC-0001 v1.4** — independent approval for the AC-27 EARS baseline migration
  only (`refund_state=REFUNDED`; in-progress `REFUNDING` is not included).
- **ADR-0007 Amendment 1** — accepted 2026-11-24 (A-01…A-14); it maps AC-62…66
  and closes D-27…D-29. The ADR file's own status header is Architect-owned and
  may lag this human-approval record; Architect must sync it.

**Scope of this record**: it is a **contract approval only**, not AC-VERIFIED,
not implementation PASS, and not Release PASS. No code/test evidence is claimed.
AC-39…61 remain deferred; AC-62…66 are newly registered as deferred
(`Developer`, expiry `2026-12-31`, `developer-reviewer/T-002`). Any
implementation/release still requires Reviewer verdict, AC evidence, affected
baseline regression (AC-14/21/25/27/31/32), and human `RELEASE_APPROVED`.

### 2026-11-24 zero after-sale amendment: SPEC-0002 v1.9

Director approved the zero after-sale amendment on 2026-11-24; PO applied it to
`docs/specs/0002-payment-settlement-admission.md` **v1.9 approved**.

- Formalized **S19** (zero refund execution, `refund_applications(source=system_zero)`),
  **INV-50**, and **AC-67/AC-68**; AC-55 gains a formal zero-only exception
  (`pay_cent=0` and no captured intent) while positive refunds keep Payment-fact
  priority unchanged.
- Zero after-sale uses `AdmitRefund` zero short-circuit: same-transaction S19
  `DONE`, no `PENDING`; AfterSales final `REFUNDED/0/CONSUMED/refund_no=''`.
- QC-18: `READY FOR HUMAN ZERO AMENDMENT APPROVAL` (0 BLOCKER).

**Scope**: contract approval only, not AC-VERIFIED, not implementation PASS, not
Release PASS. AC-67/68 are registered as deferred for Developer
(expiry 2026-12-31, `developer-reviewer/T-002`); implementation/C-05 remain
separate gates.

## P1

| ID | Status | Evidence / remaining gap |
|---|---|---|
| P1#1 | done | Closed-bucket release writes one ledger/event; `TestReleaseTxClosedSeckillWritesOneLedger` (MySQL) proves exactly one row. |
| P1#2 | done | `order.cancelled` uses `TccCancelConfirm`; partial MarkPaid compensation includes the failing SKU; `afterPaid` failure emits a durable confirm event consumed by `RunConfirmConsumer`; NATS confirm/order.cancelled tests pass. |
| P1#3 | done | `Stock.version`; `cmd/catalog-stock-rebuild` dry-run found 3 mismatches and repaired 3. |
| P1#4 | done | Full draft SHA binding, fail-closed meta, close errors surfaced, Lua CAS state transitions; live tamper 409, correct/duplicate 200, save-failure -> FAILED -> retry test. |
| P1#5 | done | `contains_any` executes; eval 15/15. |

## P2 (current handoff IDs)

| ID | Status | Evidence / remaining gap |
|---|---|---|
| P2#8 | done | Public GetProduct/GetSku require ENABLED + price>0. |
| P2#9 | done | page>1000 INVALID; cursor errors unified; GORM/SQL keyset walk to last page tested. |
| P2#10 | done | `BatchGetStock`; checkout/cart-merge/AI confirm batch prefetch + local totals; dedupe/missing test. The older “rejected” note is obsolete. |
| P2#11 | done | Assistant root is hidden until `available=true`. |
| P2#12 | done | Old broad cleanup disabled; `0014` requires exact seed fields/version; SQL positive/negative checks. |
| P2#13 | done | Strict identity for checkout/payment/admin; normal routes keep 5s cache; live warmed-cache disable→401. |
| P2#14 | done | `make test-integration` covers inventory/catalog/pricing and CI invokes it; local run passed. Real GitHub CI not run. |
| P2#17 | partial | Versioned index, dual-write, lease renewal, alias/cache invalidation, metrics, hit@5 5/5 done. Independent `ai-rag-build` Job/CronJob + load-only follower Pods remove the first-build/follower window. Remaining gap: cache/rerank quality comparison still thin. |

## 2026-11-21 external review: transaction refactor B

| Item | Status | Evidence / remaining gap |
|---|---|---|
| B (#1/#2/#37) | done (dev cluster) | `inventory_operations`/`inventory_operation_items` (0015), deterministic operation id + scope hash, per-item progress, savepoint per item, terminal events, replay, DTM single-branch path. MySQL/NATS tests cover middle-item failure/resume, permanent per-item failure, mixed cancel, all-or-nothing branch tx, retry budget, concurrent confirm/cancel, replay. The inventory image now exposes `op-replay` as a one-shot recovery command. Live dev verification: Helm migration job complete; DTM smoke passed; multi-SKU live confirm/cancel reached 2/2 `SUCCEEDED`. Remaining: production rollout/rollback rehearsal; a gRPC/BFF replay surface is optional. |
| #22 payment config | done | Invalid `PAYMENT_MOCK_ENABLED`/AI booleans fail config loading; non-dev `PAYMENT_MOCK_ENABLED=true` is rejected; `PAYMENT_SECRET` remains required when mock is off. Config tests cover all three. |
| #38 AI mesh | done | explicit read ALLOW plus explicit write DENY. Live UAT: ai-sa ListProducts succeeds, Catalog CreateProduct returns PermissionDenied; `scripts/check-ai-mesh.sh`. |
| AI RAG release isolation | done (dev) | The post-upgrade RAG build was blocking upgrades when the AI image was stale; the one-shot Job is now opt-in (`ai.ragBuild.hook=false`), the CronJob is the normal path, and a manual `create job --from=cronjob` completed. Helm upgrade completed without `--no-hooks`. |
| Storefront HTTP session | done (dev) | The built cookie was `Secure` on plain HTTP, so a full page load dropped the session and the seckill browser test could not authenticate. Cookie security now follows the request protocol; Playwright 7/7 passed. |
| #3 catalog rebuild | done for version guard | `decideProjection` refuses a newer catalog projection and the SQL update is CAS-guarded; unit test covers equal/newer/stale/conflict. Missing-catalog/inventory rows are reported as counters, not a full reconciliation UI. |

## 2026-11-21 external review: transaction refactor A

| Item | Status | Evidence / remaining gap |
|---|---|---|
| A (#20/#21/#24/#36) | done (dev cluster) | Order lifecycle is separate from `refund_state`/projection; shipping permit CAS + refund admission; server-owned `available_actions`; after-sales lifetime ledger from Payment captured, immutable effective-amount versions, `RETURN_RECEIVED`, durable restock/refund tasks, no status scanner; Payment `GetPaymentByOrder`; Fulfillment `Ship` requires the permit. MySQL tests cover permit/refund race, projection/lifecycle separation, TCC cancel, concurrent amount admission, restock failure/retry, refund retry/idempotency. Live dev verification: real-data preflight clones then Helm migration job complete; 17 legacy restock tasks converged to `DONE`; refund-only after shipment kept `SHIPPED/FULL`; pre-ship refund blocked shipment (`422`, permit `REVOKED`); the admin CREATE shipment path now checks `Order.available_actions` before inserting a shipment; `make dtm-smoke` passed; E2E phase15 passed; Playwright 7/7. Remaining: production backup/restore and rollback rehearsal; order detail still reads after-sales from the separate endpoint. |
| #3 catalog rebuild | covered by B | Version CAS + stale/conflict reporting from the previous slice. |

## 2026-11-21 product decision: multi-warehouse C

| Item | Status | Evidence / remaining gap |
|---|---|---|
| C cross-warehouse split orders | done (dev+UAT); production split flag intentionally off (no large-scale rollout) | ADR [0002](adr/0002-cross-warehouse-split-orders.md) accepted. Phase 1 live: `order/0011` applied after real-data preflight; 94 orders -> 94 `G1`/warehouse-1 groups, 0 missing groups, 0 bad item allocations; a new smoke order got `G1/1/ALLOCATED` and line 1/warehouse 1; `TestFulfillmentGroupShadowTracksOrder` passes. This is shadow state only: no split allocation, quote/pricing warehouse input, per-group permit/shipment or inventory key change yet. Phase 2a adds read-only `Inventory.AllocateOrder` (lowest warehouse id with full line availability, no reservation writes) and a gateway `WAREHOUSE_ALLOCATION_SHADOW` dual-read (500ms cap, log-only). Live dev: shadow enabled, `dtm-smoke` preview logged "warehouse allocation shadow matched single warehouse" (1 line), and E2E phase15 passed. Phase 2b adds `QuoteRequest.allocations`, `QuoteLine.warehouse_id/group_no`, `quote_groups`, per-group freight, and an Order gate that rejects split quotes until phase 3. Phase 3a makes inventory reservation/operation identity warehouse-aware: unique `(order_no, sku_id, warehouse_id)`, operation items keyed by sku+warehouse with `inv:v2` ids, OrderItem carries warehouse/group, and saga/Confirm pick it up. Live: `inventory/0016` preflight + migration passed; E2E phase15 passed. Phase 3b implements multi-group `CreateOrderInner` groups, Order/Gateway `WAREHOUSE_SPLIT_ENABLED` (default false), per-group shadow allocation and a split-quote validator. Phase 4 makes shipments/permits group-keyed, creates one parcel per order group, and resolves partial shipment/receipt into `shipment_status`. Live dev validation used a temporary `WAREHOUSE_SPLIT_ENABLED=true`: two-warehouse quote (G1/1 + G2/2), two Auto shipments, G1 ship -> `ALLOCATED/PARTIALLY_SHIPPED`, G2 ship -> `SHIPPED/SHIPPED`, G1 receive -> `SHIPPED/PARTIALLY_RECEIVED`, G2 receive -> `COMPLETED/RECEIVED`; phase15 E2E passed. Phase 5 makes after-sale items and restock tasks group/warehouse-aware, requires one group per after-sale, validates a return against the group's SHIPPED/RECEIVED state (not the overall partial order status), blocks only that group's permit, and restocks to the origin warehouse. Live dev: split order with G2 shipped; RETURN_REFUND for G2 restocked to warehouse 2 and refunded while G1 still shipped later to SHIPPED; permit G1 remained absent/unblocked. Phase 6 exposes group actions (`ship_group:`, `receive_group:`, `apply_after_sale_group:`), a storefront group/logistics/receive/after-sale UI, a group selector on the after-sale apply page, admin per-group shipment creation, and a shipment-list BFF. Live browser check: split order showed G1/G2, G2 receive moved the projection to PARTIALLY_RECEIVED, and the apply page carried the G2 group. Dev single-node now enables split; production defaults remain false. System review: [c-warehouse-review.md](c-warehouse-review.md). Live dev: `pricing/0010` applied after real-data preflight (88 quotes -> 88 groups, 0 missing/bad lines); E2E phase15 passed; the new quote is `G1/warehouse 1` with per-group freight. Pricing unit tests cover group freight/discount and the single-group gate. Defaults approved: flat per-warehouse freight, stock/priority allocator, line-level split only, origin returns, paid-unshipped via refund. |

## P3-0 contract prerequisites

| Item | Status | Evidence |
|---|---|---|
| Event registry / outbox validation | done | `internal/platform/events/registry.go`; unknown/malformed payloads fail closed in `outbox.Append`; `TestRegistryCoversKnownEventTypes` |
| Contract gate | done | `make contract-check`: proto guard, event tests, admin/gateway route parity, generated TS freshness |
| Production backup scope | decided | [ADR 0004](adr/0004-p3-0-contract-and-production-scope.md) |

## P3

- Version increments: done for catalog category/brand/SKU, identity address,
  seckill qualification.
- CampaignSpec: done.
- etcd: optional/disabled by default in chart, README corrected.
- Frontend ESLint: done; admin 0 error/11 warning, storefront 0 error.
- Admin router: split to 263 lines; promotion/catalog/orders/seckill/catalog-write
  handlers moved. Further business-boundary split remains optional.
- Generated types: read models Sku/Product/Order/AfterSale/Campaign/PromotionRule/
  Stock/Category/Brand derive from generated schemas; Shipment/Refund/ListReply
  still hand-written because no generated schema exists.
- proto guard: RPC signatures + reserved; baseline 685; CI invokes guard.
- K8s observability: OTLP collector egress added.

## AI roadmap truth table

| Phase | Code status | Honest status / gap |
|---|---|---|
| 5 correctness | identity, eval, HITL binding/idempotency, explicit CONFIRMING/SAVED, FAILED via Abort, admin state UI, Lua CAS | done for the gate |
| 6 versioned RAG | versioned index/alias, dual-write, lease renewal, reconciliation, cache, metrics, hit@5, build Job/CronJob + load-only followers | partial: cache/rerank quality comparison thin |
| 7 cart assistant | draft tool, Redis draft, SSE draft, storefront confirm, Cart idempotent confirm | done: real-model E2E + expired/price/stock/unclosed Handler tests |
| 8 preferences | Redis profile, 180d retention, API/UI, system-message injection, A/B 3/3, per-session attribution key, Redis isolation test, attribution SSE event + storefront UI | partial: benchmark still one prompt family; add a cross-user/no-memory regression set |
| 9 ops copilot | deterministic daily brief, latest snapshot, signed webhook (HMAC, fail-closed, delivery id), low-stock links, feedback | partial: cancellation/refund anomaly explanations not built |
| 10 advanced | not adopted | deferred by evidence gate |

## Verification evidence

- `make verify` passes (proto guard included).
- `make test-integration` passed locally with MySQL/NATS/Redis across outbox,
  consumer, AI RAG, seckill, inventory, catalog and pricing.
- `ai-eval` 15/15; `ai-rag-eval` hit@5 5/5; `ai-pref-eval` 3/3.
- Phase 7 real-model draft SSE → confirm 200; normal answer emitted 246 SSE deltas.
- RAG lease renewal + follower pointer swap + build-due Redis tests.
- HITL Redis CAS tests: save-failure -> FAILED -> retry, SAVED terminal, illegal
  transition, missing/corrupt meta; admin BFF save-failure retry test.
- Attribution: owner-scoped SSE event (cross-user has none) and BFF proxy test.
- Webhook: signature/timestamp/delivery headers, fail-closed without secret,
  non-2xx rejection; frontend typecheck + build pass. `make verify` passes with
  proto guard at 685 baseline entries (the refresh also added previously missing
  HITL abort fields).
- Inventory B: `go test -race ./internal/services/inventory` with MySQL/NATS
  passed; `make test-integration` passed including the new operation tests;
  migration `0015` applied cleanly to a scratch MySQL 8.4 database.
- Transaction A: `go test -race ./internal/services/order
  ./internal/services/aftersales ./internal/services/payment
  ./internal/services/fulfillment` passed with MySQL; `make test-integration`
  includes the new order/payment/after-sales/fulfillment tests. Migrations
  `order/0010` and `aftersales/0010` were applied to scratch MySQL 8.4 data;
  the legacy status/amount/restock backfills produced the expected rows.

## Explicit unverified / drift

- Real GitHub Actions was not executed.
- The new AI image/Job/CronJob was not rolled out to the dev cluster in this
  session; the RAG crash/restart live drill is still pending.
- Phase 8 has owner-scoped attribution, but no automated cross-user/no-memory
  benchmark beyond the one prompt family.
- Phase 9 has a signed external webhook, but no rich cancellation/refund anomaly
  explanations.
- Inventory B has no live DTM barrier smoke (`make dtm-smoke`) or cluster
  rollout; `ReplayInventoryOperation` is an internal method without an admin
  RPC/BFF surface yet.
- Transaction A has no cluster rollout, live DTM smoke, or browser E2E; the
  new migrations were validated on scratch data, not against the deployed
  `mall_order`/`mall_aftersales` databases.

## 2026-11-24 PAY-SETTLE-01 implementation review: FAIL

Reviewer scoped review of `d38f282..03a0717` (report
`docs/collab/qc-reports/PAY-SETTLE-01-Review-01.md`) returned **FAIL**,
release_status FAIL. Six contract-level blockers:

1. TCC compensation bypasses group refund admission; refund and shipping can
   both proceed.
2. Shortage groups can still ship (no permit revoke / no shortage or readiness
   check in AuthorizeShipment).
3. Zero-amount refund/shortage path is blocked by `captured > 0`.
4. Historical PAID orders without `settlement_payment_no` cannot refund.
5. Commitments have no RELEASED path; quota is permanently reserved.
6. Group quota does not filter freight by responsibility.

No `[ARCHITECTURAL_FLAW]`: ADR-0007 D-08/D-09/D-10 and §6.1 already specify the
required behavior; these are implementation deviations. The goal remains active;
P0-A/P0-B is **not complete** and no Release PASS may be claimed.

### 2026-11-24 remediation after Review-01

At head `6550a2c`: P-01 safety net (TCC cancel blocks all groups; full fan-out
admission still missing), P-02 fixed (AuthorizeShipment checks readiness, group
refund state and open shortage), P-03 fixed (zero-amount admission). P-04
(historical PAID resolution), P-05 (RELEASED) and P-06 (freight responsibility)
remain open. Review-01 remains FAIL; no Release PASS until a fresh scoped review
at a new head.

## 2026-11-23 human decisions: payment settlement & fulfillment admission (P0)

Source: external review `docs/ext-review-1123.md`; classification and evidence in
`docs/ext-review-1123-triage.md`. Human approved the H1–H6 recommendations on
2026-11-23. This section records status and pointers; normative text lives in
Spec/ADR/CONVENTIONS.

| # | Decision | Status |
|---|---|---|
| H1 | First payment must be exact (`captured == pay_cent`), one active payment per order, duplicate capture refunded, `pay_cent=0` exempt, **no partial payment this phase** | approved; SPEC-0002 v1.7 approved; ADR-0007 accepted 2026-11-23; implementation next |
| H2 | Payment-settlement + fulfillment-admission is a P0 project (`change_id=PAY-SETTLE-01`), split P0-A funds / P0-B admission / P0-C three deterministic release errors; production switches stay false; UAT split may stay but is **not** release evidence | approved; freeze in effect |
| H3 | Integration owner: **Director** (name provided 2026-11-23); Architect acts as design owner/secretary; one payment/order Developer as deputy; no per-service split ownership | approved; Director named, deputy TBD |
| H4 | Withdraw the `c-warehouse-review` “functionally complete” release implication, keep history | approved; doc amended 2026-11-23 |
| H5 | Mesh narrowing now (security, not an AI feature); production `ai.enabled=false` until live Mesh UAT; AI-0 design may follow P0, AI-1…5 deferred | approved; static config changed, live UAT pending |
| H6 | Approve `INV:` in reviewer metadata / `baseline_impact` (G1) and the QC crash-window checklist (G2); no new machine gates | approved; CONVENTIONS §2.2/§3/§4.1 and `prompt/Quality-Checker.md` updated |

**Freeze / release hold**: no Release PASS may cite `make gates-strict`, thread
CLOSED, AC-REF counts, or the historical C/UAT results. Unfreeze only when the
§7.3 matrix in the triage doc, G-4, explicit `--release --range`, and the
five-party final-state check all pass.

**Next gate**: ADR-0007 v0.4 is **accepted** (Director 2026-11-23); SPEC-0002 is
v1.7 (INV-43 scope clarified). P0-A/P0-B implementation starts with the C-1…C-9
preconditions: `CheckOrderReady`/`CancelIfCreated`/`GetIntent(intents[])` proto
contracts, event registry, worker assembly, migration review, and the Inventory
CONFIRMED-release check. Until those contracts land, keep the production flags
false and do not claim AC-VERIFIED.

**C-1/C-8/C-9 closed**: Director approved 2026-11-23; SPEC-0001 v1.3 and
SPEC-0002 v1.6 approved; AC-39…61 registered as deferrals
(developer-reviewer/T-002, expiry 2026-12-31); commits `9dc38b6`, `6aa12f3`.

**ADR-0007 handoff (R-1…R-4, from QC-08)**: (R-1) Σ group caps ≠ order remaining
captured needs an owner/action for manual adjudication; (R-2) choose accounting
for order-level cancellation refunds vs group caps; (R-3) historical no-group
orders use the whole remaining amount for G1; (R-4) TCC refunds freight first, so
a later responsibility ruling must not double-refund (shared group ledger).

**2026-11-23 approvals and landing**: Director approved C-1 (SPEC-0001 v1.3),
C-8 (D-11…D-21 all defaults), and C-9 (ledger + move + commit). SPEC-0001 v1.3
and SPEC-0002 v1.6 are approved; AC-39…61 are registered in
`ac-baseline.txt`/`ac-coverage.tsv` as deferrals (developer-reviewer/T-002,
expiry 2026-12-31). ADR-0007 is the next gate.

**2026-11-23 0012 decision**: long-term split chain (not the operator bridge).
Audit: dev `mall-data` and UAT `mall-uat-data` applied only up to
`aftersales/0011_group_items.sql`; no 0012 row/attempt. Local compose has no
`schema_migrations` and no `line_no`. So 0012 can be replaced before any
persistent environment applies it; implementation still needs Architect sign-off,
a backfill tool and a disposable-DB rehearsal.

**Gate note (historical, 2026-11-23; superseded by the 2026-11-24 approvals
above)**: while the Spec was still unapproved, the draft lived in
`docs/po/drafts/` and was not indexed in `docs/specs/README.md` or `Spec.md`. On `SPEC_APPROVED`,
Reviewer adds AC-39…61 to `docs/collab/ac-baseline.txt` and `ac-coverage.tsv`
(owner + expiry + thread), the PO moves the file into `docs/specs/`, updates both
indexes, and the approval is committed as `docs(spec): SPEC-0002 v1.x ...` in the
same round. Do not index or commit the draft before that sync.

P0-C (order list scan, aftersales migration ordering, Helm flag plumbing) may
start independently because it implements already-approved contracts.
