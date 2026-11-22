# ADR 0004: P3-0 contract gate and production backup scope

- Status: accepted
- Date: 2026-11-22
- Related: [ADR 0003](0003-c-production-rollout.md)

## Context

P3 adds AI Turn/Action, quantity-level warehouse allocation, carrier providers
and multi-warehouse seckill. Those features will add new actions, events and
money/inventory transitions. Before P3 implementation, the existing contract
surface must stop drifting.

The current P0/P1/P2 necessary slice has passed dev and UAT validation. Full
production backup/restore and A/B production rollback have not been performed
because there is no large-scale production rollout; production split remains
off.

## Decision

1. **Core outbox events have a typed schema registry** in
   `internal/platform/events`. `outbox.Append` validates every payload against
   the registered schema and unknown event types fail closed.
2. **Contract gate** is `make contract-check`:
   - proto evolution baseline;
   - typed event schema/contract tests;
   - admin/gateway OpenAPI route parity;
   - generated TypeScript must match the committed OpenAPI specs.
3. **Production backup/rollback scope:**
   - Production backup/restore and A/B/C rollback are **not** P3 design
     prerequisites while production split remains off.
   - They are a hard precondition only for a production enablement request, and
     the C runbook (ADR 0003) remains the source of truth.
   - P3 features must ship behind disabled-by-default flags and must not turn on
     production split implicitly.
4. **P3 implementation may start only after its own Spec/ADR**:
   - AI: Turn/Action protocol ADR, including turn identity, session concurrency,
     draft/confirm, cancellation, recovery and mesh read-only allowlist.
   - Warehouse: product Spec for quantity split, cost/priority model, returns
     and freight allocation.
   - Carrier: provider/webhook/idempotency contract.
   - Seckill: multi-warehouse semantics after split production gate.

## Consequences

- Adding a new event type or changing a required field without updating the
  registry fails the producing transaction (`outbox.Append`) and/or tests.
- Optional fields require `omitempty` and a schema update; incompatible changes
  require a new event version rather than silently mutating an existing payload.
- P3 can start with design work and isolated experiments, but not with a
  production rollout.
- Production backup/rollback remains an explicit future gate, not an implicit
  P3 deliverable.
