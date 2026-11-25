# Session handoff — snapshot

> **Purpose**: session-boundary snapshot, not a rule/contract source. Refresh
> only before a new session.
>
> Current truth: [`review-tracking.md`](review-tracking.md) and
> [`roadmap-status.md`](roadmap-status.md). Contracts: [`specs/`](specs/),
> [`adr/`](adr/). Environment/tests: [`development.md`](development.md).
> UAT: [`runbooks/uat-environment.md`](runbooks/uat-environment.md).
>
> Do not use this file as a Developer rule source. If a rule exists only here,
> stop and ask the Lead to route it to its canonical owner.

## Snapshot (2026-11-25)

- **HEAD**: `90b502c` on `main`; worktree clean.
- `main` contains the PAY-SETTLE-01 P0 WIP merge (`d7c2f30`).
  **Not P0 closure; not Release PASS.**
- Canonical contracts: SPEC-0002 **v1.10**, ADR-0007 **Rev 2.3**.
- Production split remains **off**; no Release approval.
- P0 WIP state: T1.7 test-only PASS; R0.1/C-05 evidence PASS; R0.2 C-01…C-05
  contract PASS after C-04 “release RPC” B1 was withdrawn (release is event).
  **R0.3 FAIL**; **R0.4b/c WIP `90b502c`**; R0.5 not done.

## Completed this session

- `docs/handoff.md` split: durable env/test rules moved to
  `docs/development.md`; handoff is snapshot only. README and
  `prompt/Developer.md` point to canonical; QC-34/Reviewer PASS.
- T1.7 test-only evidence (`93380d5..2a05c1d`): Σ mismatch, REFUNDING vs Ship,
  crash resume, partial group failure, legacy no-group G1, shipped POST.
- R0.1: real MySQL 8.4 migration apply; 0018 re-entry; 0011
  `chk_switch_settled_expiry`; C-05 evidence PASS.
- R0.2-R2: C-01/C-02/C-03/C-04/C-05 contract PASS; C-04 release is
  event/outbox, **no release RPC**.
- R0.4b/c WIP:
  - Fulfillment startup reads `settlement_switch_approvals`; paid/shadow/settled
    fail-closed; `SETTLE_RELEASE_RANGE`;
  - `order/0022` group_no/24h SLA CHECK;
  - **not approved**: R0.4 Addendum is DRAFT; no QC/Architect/human/Reviewer;
  - shadow runtime is deliberately rejected until T4.4; range wiring in
    Helm/compose and `verify-r01-migrations.sh` extension are not done.

## Open — next session first

1. **R0.4 governance/closeout**: accept Addendum via QC/Architect/human, then
   Reviewer; decide shadow runtime; wire `SETTLE_RELEASE_RANGE` in deploy/ops;
   add `order/0022` to the real-migration verification script; R0.4d/e.
2. **R0.3/R0.5**: AC-27 needs `refund_state=REFUNDED` + reservation/waybill
   CANCELLED branch assertions; INV-45..50 rows need branch descriptions;
   regression AC-14/21/25/27/31/32 and 12 orphan stay DEFERRED; refresh
   evidence-matrix to SPEC v1.10.
3. **T5/AC**: AC-39…68 evidence, real migrations, `-race`/fault; **no
   AC-VERIFIED yet**.
4. Only after P0 closure: T2/T3/T4, then release decision. Do not start P3/0012
   in this thread.

## Canonical pointers

| Need | File |
|---|---|
| Running ledger / gaps | `docs/review-tracking.md` |
| Post-merge backlog | `docs/lead/PAY-SETTLE-01/backlog.md` |
| Tasks / Phase 0-R | `docs/lead/PAY-SETTLE-01/tasks.md` |
| Evidence matrix | `docs/lead/PAY-SETTLE-01/evidence-matrix.md` |
| R0.4 design DRAFT | `docs/adr/0007-amd1-addendum-r0.4-c04-release.md` |
| T1.7 report | `docs/collab/qc-reports/REV-PAY-SETTLE-01-T1.7.md` |
| R0.2/R0.3 reports | `docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.2-R2.md`, `...-R0.3.md` |
| Developer env/test | `docs/development.md` |

## Do not say

- “P0 closed” / “Release PASS” / “AC-39…68 verified”.
- “multi-group stock released” — P0 deferred, reserved/P1.
- “R0.4 approved” — it is WIP `90b502c`, no Reviewed-by.
