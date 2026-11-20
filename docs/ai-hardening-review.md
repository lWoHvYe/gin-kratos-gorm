# AI hardening review (2026-11-21)

This supersedes the earlier “done” wording where it overstated coverage.

| Area | What is actually implemented | Status / gap |
|---|---|---|
| RAG rebuild | Redis lock with 3m TTL, 1m renewal, alias fallback, current pointer without TTL, lazy proxy + 2m background retry | AI starts before embedding is ready; still no independent Job; first build >3m can leave followers without alias |
| RAG cache | version-bound exact/semantic cache; increment/delete invalidates version cache; rollback runbook flushes cache | implemented; cache/rerank A/B quality not measured |
| HITL | `INTERRUPTED -> CONFIRMING -> SAVED`; `FAILED` via Abort; full draft hash; fail-closed meta; close failures surfaced | no admin UI for FAILED/state display; CONFIRMING declaration is not atomic CAS |
| ADK chat | non-streaming was replaced by a cumulative-chunk normalizer; tool-call fragments merge by Index; SSE deltas restored | eval 15/15 and 246 deltas; no multi-replica/provider-failure live soak |
| Cart assistant | draft tool, Redis draft, SSE draft, storefront confirm card, Cart transactional idempotent confirmation | expiry/price/stock-change rejection is not covered by an automated suite |
| Preferences | 180-day Redis profile, API/UI, system injection, A/B 3/3, per-session attribution record and Redis user-isolation test | attribution is Redis/log only; one prompt family |
| Ops brief | deterministic daily Redis snapshot, atomic latest/done, low-stock links, feedback | no external delivery; anomaly explanations limited |
| Types/admin | generated-schema derivation for major read models; admin router 263 lines; ESLint real | Shipment/Refund/ListReply remain hand-written; further split optional |

## Remaining concrete work

1. RAG Job/elected-leader + crash/restart drill; remove the 3m follower window (lazy retry already removes permanent RAG-disabled startup).
2. HITL `FAILED`/Abort + admin state display; atomic state transition.
3. Cart draft expiry/price/stock-change test suite and explicit UI recovery cases.
4. Phase 8 cross-user/attribution eval; Phase 9 external delivery/anomaly links.
5. Real GitHub CI run.
