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

## Snapshot (2026-11-26)

- **HEAD**: `fd208b2` (Reviewer verdicts) on `main`; then Lead/ledger updates.
  Worktree clean after the Lead checkpoint commit.
- `main` contains the PAY-SETTLE-01 P0 WIP merge (`d7c2f30`).
  **Not P0 closure; not Release PASS.**
- Canonical contracts: SPEC-0002 **v1.10**, SPEC-0001 **v1.5**
  (AC-27/AC-27A, approved 2026-11-26), ADR-0007 **Rev 2.3**,
  R0.4 Addendum **ACCEPTED** (`c432ed6`).
- Production split remains **off**; no Release approval.

## State after this session

- **R0.4 Addendum**: accepted by human 2026-11-26 (QC R3 READY; 9 MINOR TODO).
- **R0.4 implementation**: `c38c54f` + selfcheck `9322e75` (branch B:
  0022 edited after preflight; 24h equality; exit 3/4; compose/Helm/ops;
  `verify-r01-migrations.sh` 0021/0022; real MySQL 8.4.13 EXIT=0).
- **Reviewer R0.4e = FAIL (scoped)**: F-1 missing `ADD CHECK` lock/MDL/
  maintenance-window evidence; F-2 preflight did not cover staging/prod, so
  branch B is unsafe under §2.3.3 fail-closed. Gate behavior otherwise passes.
- **Reviewer R0.3-R2 = PASS (scoped)**: AC-27 assertions real; **AC-27 stays
  AC-REF** (AC-27A steps 5/6 not implemented; `AC-50-FIX-01`,
  `AC-27-NOWB-01`, `AC-27-LIVE-01` open). AC-14 now AC-VERIFIED; AC-21/25/27/
  31/32 AC-REF; 12 orphan mapped to T5.x/T3.
- **Phase 0-R is NOT closable**: R0.3-R1c, R0.4e F-1/F-2, T2/T3/T4.4 and the
  ops alarm drill remain.

## Open — next session first

1. **R0.4e F-2**: switch to branch A (add `order/0023`, do not edit 0022) or
   obtain real staging/prod `schema_migrations` evidence; then F-1: capture
   `ADD CHECK` MDL/lock/maintenance-window evidence; Reviewer re-review.
2. **R0.3-R1c**: close `AC-50-FIX-01` (needs ADR §5.3 additive amendment:
   `CancelIfCreatedReply` no-waybill result; Order maps to
   `REFUND_ADMISSION_CONFLICT`, never PRE), `AC-27-NOWB-01`,
   `AC-27-LIVE-01`; then implement AC-27A steps 5/6 and re-review.
3. **T5/AC-39…68** evidence, `-race`/fault, real migrations; **no
   AC-VERIFIED except AC-14**.
4. Only after all of the above: P0 closure, then T2/T3/T4, then release
   decision. No P3/0012 in this thread.

## Canonical pointers

| Need | File |
|---|---|
| Running ledger / gaps | `docs/review-tracking.md` |
| Post-merge backlog | `docs/lead/PAY-SETTLE-01/backlog.md` |
| Tasks / Phase 0-R | `docs/lead/PAY-SETTLE-01/tasks.md` |
| Evidence matrix | `docs/lead/PAY-SETTLE-01/evidence-matrix.md` |
| R0.4 design (ACCEPTED) | `docs/adr/0007-amd1-addendum-r0.4-c04-release.md` |
| AC-27 ruling / SPEC v1.5 | `docs/specs/0001-fulfillment-quantity-split.md`, `docs/adr/0007-amd1-addendum-r03-ac27-readmodel.md` |
| T1.7 report | `docs/collab/qc-reports/REV-PAY-SETTLE-01-T1.7.md` |
| R0.2/R0.3 reports | `docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.2-R2.md`, `...-R0.3.md`, `...-R0.3-R2.md` |
| R0.4 verdict/QC | `docs/collab/qc-reports/REV-PAY-SETTLE-01-R0.4.md`, `QC-PAY-SETTLE-01-R0.4-ADDENDUM-R3.md` |
| Developer env/test | `docs/development.md` |

## Do not say

- “P0 closed” / “Release PASS” / “AC-39…68 verified”.
- “multi-group stock released” — P0 deferred, reserved/P1.
- “R0.4 approved” — the Addendum is accepted, but **R0.4e is FAIL** (F-1/F-2)
  and `90b502c` was superseded by WIP `c38c54f`.
- “AC-27 verified” — it is **AC-REF**; AC-27A steps 5/6 and
  `AC-50-FIX-01`/`AC-27-NOWB-01`/`AC-27-LIVE-01` remain open.
