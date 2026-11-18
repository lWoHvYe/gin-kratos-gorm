# AI roadmap

Order agreed with the product owner. Each phase must pass real-model evaluation
and keep the assistant optional (failures must not affect browse/checkout).

| Phase | Scope | Status |
|---|---|---|
| 1 | Feedback quality dashboard over `ai:feedback:*` | done |
| 2 | ADK HITL pilot for admin campaign drafts | pending |
| 3 | Redis hybrid retrieval (BM25 + KNN + RRF), rerank, semantic cache | pending |
| 4 | Replace the customer/admin ReAct runtime with ADK; keep SSE, tool events, budgets, breaker, eval | pending |

## Phase 1 acceptance

- Admin BFF `GET /api/admin/v1/ai/feedback/summary?days=1..30`
- AI `FeedbackSummary` aggregates daily helpful/not-helpful counts and recent
  not-helpful comments from Redis (per-day list capped at 5000)
- Admin SPA `/ai/quality` shows totals, helpful rate, daily table and comments
- Live check: summary returns the stored feedback entry; web build passes.

## Phase 2 shape (ADK HITL)

- New admin-only draft endpoint returns `checkpoint_id` + draft, then a confirm
  endpoint resumes via `adk.Runner.ResumeWithParams`; only the resume path calls
  the existing `apply-draft` with `confirm=true`.
- Checkpoints in Redis with TTL and admin identity binding; feature flag
  `AI_ADK_HITL`; fallback remains the current hand-rolled flow.
- No direct writes from the model; the BFF keeps the admin role check.

## Phase 3 shape

- Redis Search index keeps KNN; add BM25 over the same documents and combine
  with RRF; optional rerank; measure with the existing eval set before default.

## Phase 4 shape

- Adopt `adk.ChatModelAgent` for both prompts, one `adk.Runner` per surface.
- Preserve `anyChunkToolCallChecker` behavior, `tool` SSE events, token usage
  callbacks, budgets/breaker, and the 13-case eval. Roll out behind a flag, then
  remove the ReAct path only after parity.
