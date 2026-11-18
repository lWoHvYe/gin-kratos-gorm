# AI roadmap

Order agreed with the product owner. Each phase must pass real-model evaluation
and keep the assistant optional (failures must not affect browse/checkout).

| Phase | Scope | Status |
|---|---|---|
| 1 | Feedback quality dashboard over `ai:feedback:*` | done |
| 2 | ADK HITL pilot for admin campaign drafts | done |
| 3 | Redis hybrid retrieval (BM25 + KNN + RRF), rerank, semantic cache | done (hybrid + rerank; semantic cache deferred) |
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

## Phase 2 acceptance (done)

- `adk.Runner` with a Redis `CheckPointStore`; start interrupts with a draft,
  confirm resumes once and then calls the existing `apply-draft` path.
- Checkpoint is one-shot, 30-minute TTL, bound to the starting admin id.
- Admin UI exposes "ADK 生成草案" / "ADK 确认保存"; the plain save path remains.
- Live test: checkpoint `1-...` created, confirm created DRAFT campaign 62 /
  rule 17, replay returned NOT_FOUND.
- The deterministic draft-review agent isolates checkpoint mechanics; the
  conversational `ChatModelAgent` migration remains phase 4.

## Phase 3 acceptance (done)

- A pre-tokenized `terms` TEXT field makes CJK lexical retrieval work with
  Redis' default tokenizer; `SCORER BM25` ranks it.
- Search fetches KNN and BM25 candidates (up to 40), merges with RRF (k=60),
  then applies a deterministic query-token coverage rerank.
- Lexical failure falls back to KNN only; the assistant never fails because the
  lexical branch is unavailable.
- Real-model eval after deployment: 13/13 passed, including `rag-student`.
- A semantic cache is deferred (exact query caching would go stale on catalog
  events; revisit with an index version).

## Phase 4 shape

- Adopt `adk.ChatModelAgent` for both prompts, one `adk.Runner` per surface.
- Preserve `anyChunkToolCallChecker` behavior, `tool` SSE events, token usage
  callbacks, budgets/breaker, and the 13-case eval. Roll out behind a flag, then
  remove the ReAct path only after parity.
