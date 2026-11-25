# Session handoff — snapshot

> **Purpose**: this file is a session-boundary snapshot, not a rule or contract
> source. It is refreshed only when a new session is about to start.
>
> Current truth: [`review-tracking.md`](review-tracking.md) and
> [`roadmap-status.md`](roadmap-status.md). Contracts: [`specs/`](specs/),
> [`adr/`](adr/). Environment/tests: [`development.md`](development.md).
> UAT: [`runbooks/uat-environment.md`](runbooks/uat-environment.md).
>
> Do not use this file as a Developer rule source. If a rule exists only here,
> stop and ask the Lead to route it to its canonical owner.

## Snapshot (2026-11-25)

- Snapshot base: `93380d5`; this handoff commit follows it. Use `git log -1` for
  the exact HEAD.
- `main` contains the PAY-SETTLE-01 P0 WIP merge (`d7c2f30`) and later docs.
  **This is not P0 closure or Release PASS.**
- P0 is still open: **Phase 0-R (R0.1…R0.5)**, T1.7/T5 evidence, T1.4a/T-002,
  `inventory.confirm.requested` warehouse, NATS/DLQ, AC-39…68.
- Recent canonical versions: SPEC-0002 **v1.10**, ADR-0007 **Rev 2.3**.
- Production split remains **off**; no Release approval.

## What changed in this session

- PAY-SETTLE-01 P0 WIP merged to `main` with human approval.
- T1.6 payment-level cancel reached scoped PASS; T1.4b2a single-group release
  reached scoped PASS.
- T1.5 re-scoped: TCC no longer emits `order.refund_requested`; the consumer is
  retained only for cancel-after-confirm.
- T1.4b2 multi-group physical release was explicitly deferred (stock stays
  reserved; P1/manual).
- Phase 0-R was opened from the D-LC-5 retrospective FAIL.

## Next session first actions

1. Finish **Phase 0-R R0.1** (real migrations/CHECK/reentry) and **R0.4**
   (config fail-closed/C-04 release RPC/CHECK).
2. Close **R0.2/R0.3/R0.5** (dated Reviewer verdicts, AC/INV ledger, evidence
   matrix to SPEC v1.10).
3. Finish **T1.7** tests, then **T5** AC-39…68 evidence.
4. Do not start Release or T2/T3/T4 until P0 closure.

## Transient notes

- Local MySQL/NATS/Redis and test commands: see `development.md`; do not copy
  commands from this snapshot.
- `docs/ops.md` owns compose/Helm; `docs/runbooks/uat-environment.md` owns UAT.
- The old 2026-11-22 snapshot/P3/C sections were removed from this file; history
  remains in git.
