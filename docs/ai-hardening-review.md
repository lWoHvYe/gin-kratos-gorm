# AI hardening review (2026-11-21)

This supersedes the earlier “done” wording where it overstated coverage.

| Area | What is actually implemented | Status / gap |
|---|---|---|
| RAG rebuild | build Job/CronJob with Redis lease + renewal; AI Pods are load-only followers of `ai:rag:current` with `RAG_ALIAS_REFRESH`; build-due freshness skip | Job deploy/crash-restart drill not run on the live cluster |
| RAG cache | version-bound exact/semantic cache; increment/delete invalidates version cache; rollback runbook flushes cache | implemented; cache/rerank A/B quality not measured |
| HITL | `INTERRUPTED -> CONFIRMING -> SAVED`; `FAILED` via Abort; admin UI shows checkpoint state and exposes abort; full draft hash; fail-closed meta; Lua CAS on every transition | no browser E2E for the save-failure/retry path (Redis + BFF tests exist) |
| ADK chat | non-streaming was replaced by a cumulative-chunk normalizer; tool-call fragments merge by Index; SSE deltas restored | eval 15/15 and 246 deltas; no multi-replica/provider-failure live soak |
| Cart assistant | draft tool, Redis draft, SSE draft, storefront confirm card, Cart transactional idempotent confirmation | expired/price/stock/unclosed handler tests added; no browser E2E for each branch |
| Preferences | 180-day Redis profile, API/UI, system injection, A/B 3/3, per-session attribution record, Redis user-isolation test, `attribution` SSE event + storefront display | benchmark still one prompt family; no cross-user/no-memory suite |
| Ops brief | deterministic daily Redis snapshot, atomic latest/done, signed webhook (HMAC-SHA256 over timestamp.body, mandatory secret, delivery id, only 2xx marks done), low-stock links, feedback | cancellation/refund anomaly explanations still limited |
| Types/admin | generated-schema derivation for major read models; admin router 263 lines; ESLint real | Shipment/Refund/ListReply remain hand-written; further split optional |

## Remaining concrete work

1. RAG Job/CronJob rollout + crash/restart drill on the live cluster.
2. Browser E2E for HITL save-failure/retry and cart expiry/price-change/Complete-failure recovery (handler and Redis tests exist).
3. Phase 8 cross-user/no-memory eval across more than one prompt family.
4. Phase 9 richer cancellation/refund anomaly explanations.
5. Real GitHub CI run.
