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
- **R0.4 implementation**: branch A fix `aec9ac8` + selfcheck `4549ccb`:
  `0022` restored byte-identically to `90b502c`; `0023_r04_checks_v2.sql`
  installs `_v2` checks (24h equality, discriminator) then drops the old
  names; F-1 real MySQL 8.4 evidence (`ADD CHECK` rejects
  `INPLACE/LOCK=NONE`, COPY + MDL, maintenance window); fresh
  `0017→…→0022→0023`, `verify-r01-migrations.sh` EXIT=0.
- **Reviewer R0.4e-R2 = PASS (scoped)**: F-1/F-2 CLOSED (`49d030b`).
  Residual MINOR: F-1 raw log header `da0cdd1`; 0022 frozen header keeps the
  superseded "re-run safe" sentence. NOT RELEASE.
- **Reviewer R0.3-R2 = PASS (scoped)**: AC-27 assertions real; **AC-27 stays
  AC-REF** (AC-27A steps 5/6 not implemented; `AC-50-FIX-01`,
  `AC-27-NOWB-01`, `AC-27-LIVE-01` open). AC-14 now AC-VERIFIED; other
  regression ACs AC-REF; 12 orphan mapped to T5.x/T3.
- **AC-50-FIX-01 FROZEN**: amendment DRAFT 0.3 (`b5129b5`) not accepted; QC
  round 3 BLOCKED — `result=CONFLICT` is not a legal TCC admission result
  (`tcc_settlement.go:847-876`), partial `CONSUMED+CANCELLED` cancels the
  `CREATED` waybill before returning PARTIAL (violates AC-50 `0002:217`),
  route A did not authorize persisting CONFLICT / no-re-probe. Needs a
  PO/SPEC-0002 ruling (bounded CONFLICT/PENDING outcome + TCC authorization,
  or keep the PRE deviation). No code written.
- **Phase 0-R is NOT closable**: AC-27 prerequisites, T2/T3/T4.4 and the ops
  alarm drill remain.

## Open — next session first

1. **Decide AC-50-FIX-01**: amend SPEC-0002/ADR to add a bounded persisted
   CONFLICT/PENDING admission outcome (and authorize TCC handling), or keep
   the current PRE mapping as an explicit deviation to 2026-12-31. Then re-QC
   and implement; no Lead/Developer patch.
2. **R0.3-R1c**: close `AC-50-FIX-01`, `AC-27-NOWB-01`, `AC-27-LIVE-01`;
   then implement AC-27A steps 5/6 and re-review.
3. **T5/AC-39…68** evidence, `-race`/fault, real migrations; no AC-VERIFIED
   except AC-14.
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
