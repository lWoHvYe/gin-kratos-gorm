# AI hardening review (2026-11-20)

Long-term choices for the Phase 5-10 implementation.

| Topic | Better long-term choice | Current state |
|---|---|---|
| Phase 10 | Keep the evidence gate; defer multi-agent/multimodal/routing | deferred |
| ADK streaming | Tool-aware streaming adapter that buffers until tool_calls are known, then streams the final answer | non-streaming; stable tools, no token deltas |
| HITL recovery | Explicit `INTERRUPTED -> CONFIRMING -> SAVED/FAILED` record plus business idempotency key | unique draft-code idempotency; duplicate/save-failure tests pass |
| Cart confirm | Atomic/bulk Cart RPC or a per-draft idempotency key in the Cart service | done: `Cart.ConfirmCartDraft` transaction + `(user_id,draft_id)` unique key |
| RAG rebuild | Dedicated Job or elected leader with lease renewal | Redis leader lock; non-leader loads `ai:rag:current` |
| RAG failure | Keep the last good alias serving and retry the build with backoff | startup reports RAG disabled if embedding fails |
| Ops delivery | Outbox/durable delivery with retry | Redis date lock + `ai:ops:latest` snapshot |
| CoreDNS | Keep the host's original forwarder when it works | restored `forward . /etc/resolv.conf`; DNS verified |

## Fixed in this pass

- `ai:rag:current` had a 24h TTL and would silently disappear; now persistent.
- Cart done markers were user-agnostic; now store the owner id and reject other
  users.
- Preference values reject line breaks and are framed as data, not instructions.
- Rollback runbook requires a non-empty target index and cache invalidation
  (`ai:rag:q:*`, `ai:rag:sem:*`).

## Remaining known risks

1. Cart confirmation is now atomic and idempotent in the Cart service. The AI
   `CompleteCartDraft` call after the Cart update is best-effort; if it fails, a
   retry is safe because Cart returns the existing confirmed cart.
2. The RAG lock has a 10-minute TTL without renewal. A crashed builder blocks
   rebuilds until expiry. Followers now fall back to a valid alias if the
   `ai:rag:current` pointer is missing, but a first build longer than the 60s
   follower wait can still leave followers without an alias. A Job/lease renewal
   remains the long-term fix.
3. `RunOpsBriefScheduler` sets the date key before generating. A crash in that
   window skips the day; the outbox/job design is stronger.
4. Admin low-stock links point at `/admin/inventory?sku_id=...`; confirm that
   route exists in the admin SPA before relying on the link.
5. Phase 8's benchmark passed 3/3 repeated A/B runs.
6. Non-streaming ADK is a deliberate correctness tradeoff, not the final UX.

## Verification at the time of this review

- `make verify`
- `ai-eval`: 15/15; tool-selection 3/3
- `ai-rag-eval`: hit@5 5/5
- `ai-pref-eval`: PASS
- Phase 7 real-model draft SSE -> confirm 200
- RAG alias rollback 64 docs and restore
- CoreDNS original forward: `api.deepseek.com` resolves

## Additional verification (follow-up)

- Phase 8 benchmark rerun after the discriminating-baseline change: 3/3 PASS.
- Strict revocation: warmed the 5s cache, disabled demo, checkout preview
  returned 401 immediately; re-enable returned 200.
- BatchGetStock deduplicates IDs and omits missing rows (MySQL test).

## P2#12 seed cleanup

The broad cleanup files are retained only as `.sql.disabled` for history. New
`0014` migrations match exact ids/codes/names/prices/status and inventory
`version = 1`; a fresh-DB SQL check removed all seed rows, while an adapted
price/version row survived.
