# AI roadmap

Order agreed with the product owner. Each phase must pass real-model evaluation
and keep the assistant optional (failures must not affect browse/checkout).

| Phase | Scope | Status |
|---|---|---|
| 1 | Feedback quality dashboard over `ai:feedback:*` | done |
| 2 | ADK HITL pilot for admin campaign drafts | done (deterministic review agent; bound + atomic save) |
| 3 | Redis hybrid retrieval (BM25 + KNN + RRF), rerank, semantic cache | done (hybrid + rerank + versioned semantic cache) |
| 4 | Replace customer/admin ReAct with ADK; keep SSE, tool events, budgets, breaker, eval | done for chat; HITL remains deterministic |
| 5 | Correctness and recovery gate | done (identity/eval 15/15, HITL FAILED/Abort/state UI) |
| 6 | Versioned RAG and measurable quality | partial: hit@5 5/5, grounded/refusal, lease/rollback done; no Job, thin cache/rerank comparison |
| 7 | Customer task assistant: comparison, budget list, cart draft | done: real-model E2E + expired/price/stock/unclosed tests |
| 8 | Controlled personalization / preference profile | partial: A/B 3/3, attribution key/isolation test; no UI attribution/regression suite |
| 9 | Proactive admin operations copilot | partial: Redis snapshot + optional webhook/links/feedback; no webhook auth/rich anomaly explanations |
| 10 | Evidence-triggered advanced capabilities | deferred by evidence gate (no adoption) |

## Audit note correction (2026-11-20)

The earlier audit note listed four runtime boundaries. Current code has moved:

- `preview_campaign` no longer falls back to user 1; it uses
  `userIDFromContext` and errors when identity is absent.
- `cmd/ai-eval` executes `contains_any` and rejects cases without an executable
  assertion.
- HITL now checks draft binding before saving, saves campaign+rule in one
  transaction, and closes the checkpoint only after that write.
- RAG no longer drops the live index or caps collection at 500 products; it
  builds a versioned index, switches `_current`, and paginates fully.

So the roadmap should not repeat those as open defects. What remains is below.

## Phase 5: correctness and recovery gate

Done:
- server-injected identity is the only identity source
- `contains_any` and assertion-less-case rejection in eval
- HITL binding check, atomic campaign+rule transaction, close-after-save
- draft/preview/HITL share one CampaignSpec builder

Remaining before calling the gate done:
1. ~~Add a test that captures `EvaluateDraftRequest.UserId`~~ (done).
2. ~~Run eval repeatedly~~: `-repeat` + test-only rate/budget resets produce
   26/26 over two runs; tool-selection accuracy and raw text are in the report.
3. ~~HITL recovery/idempotency~~: duplicate confirm returns the existing
   campaign/rule via unique draft codes; failed save leaves the checkpoint open
   and retry succeeds. Formal state records are optional; live tests pass.
4. Keep the plain apply path available; HITL must not be a single point of
   failure for admins.

Exit gate: repeated real-model eval meets the agreed threshold; identity,
save-failure and duplicate-confirm tests pass; AI remains outside browse and
checkout.

## Phase 6: versioned RAG and measurable quality

Done:
- independent versioned index/prefix, `_current` alias switch, full pagination
- dual-write registry, post-build reconcile, 15-minute state reconciliation,
  7-day old-version retention
- version-bound exact + semantic cache
- document/build/version metrics and search-mode/empty metrics

Remaining:
1. ~~Move the full rebuild out of every Pod startup into an elected leader or a
   Job~~ (Redis leader lock + non-leader loads `ai:rag:current`; a dedicated Job
   remains optional). Quality eval, consumer lag/truncation/rebuild-failure
   metrics and a tested rollback runbook remain.
2. Metrics: expected vs indexed document count, truncation/empty-page signal,
   consumer lag, rebuild failures and version age.
3. Quality eval beyond string matching: retrieval hit@5 is 5/5 via
   `make ai-rag-eval`; groundedness/refusal and cache/rerank comparisons remain.
4. A tested rollback runbook: switch alias back to the previous version and
   verify catch-up; retained versions alone are not a rollback guarantee.

Exit gate: rolling restart and embedding failure leave the last good index
serving; >500 products fully indexed; quality is measured against the Phase 5
baseline.

## Phase 7: customer task assistant (draft, never direct write)

Current slice: `draft_cart` is a read-only tool that resolves SKU/price/stock,
persists the draft in Redis bound to the server-derived user, and returns a
10-minute draft id. The BFF confirm endpoint revalidates price/status/stock and
calls Cart. It is idempotent after completion (duplicate confirm returns
`confirmed=true` without adding again). An SSE `draft` event and the storefront
confirmation card are implemented. Real-model E2E passed (draft SSE then
confirm 200); duplicate draft frames are tolerated by the client.

Start with product comparison and budget shopping-list generation. The
assistant uses read tools to produce a cart draft containing SKU, quantity,
price quote and stock timestamp. The UI renders the draft; only explicit user
confirmation calls existing cart/checkout RPCs with server-injected identity.

Acceptance: price/stock/status/quote expiry revalidated at confirmation;
duplicate confirmation idempotent; expired/changed drafts require re-preview;
eval covers comparison, budget compliance, price/stock changes and rejection;
ordinary chat remains the fallback.

## Phase 8: controlled personalization

Current slice: server-side Redis profile keyed by user id, 180-day retention,
GET/PUT/DELETE BFF endpoints, storefront edit/clear panel, and a system-message
injection for customer chat. Live isolation check: demo preferences are empty
for the admin user. A/B benchmark passes: without preferences the first recommendation is 极光
Note 12; with `云舟` saved it becomes 云舟 M3. The profile is user-scoped,
180-day retained and clearable.

Start with a small server-side preference profile (brands/categories, budget
range, size where relevant), not unrestricted transcript memory. Preferences are
user-scoped, visible, editable and clearable; retention/deletion is explicit; do
not infer sensitive attributes. Inject only relevant preferences and record
which influenced a recommendation. Eval must show improvement over no-memory
and prove no cross-user leakage.

## Phase 9: proactive operations copilot

Current slice: admin-only deterministic brief from order/after-sale/low-stock
aggregates, with source window and generated_at, plus an admin UI button. An AI
scheduler writes one brief per day under a Redis date lock and stores
`ai:ops:latest`; failures release the date for retry. Exception explanation
links remain; latest-brief read and admin feedback are implemented.

Use a scheduled, controlled workflow rather than free-running multi-agent
loops. Daily brief and exception explanations come only from existing aggregate
admin RPCs; every number must be traceable to a tool response. Delivery stays
outside transaction paths, is idempotent, records source timestamps, and AI
failure only skips a brief. Eval checks numeric fidelity and that anomalies are
not invented.

## Phase 10: evidence-triggered advanced capabilities

Decision: deferred. Phase 7-9 evaluation has not demonstrated that a single
ChatModelAgent/Workflow is insufficient, and there is no multimodal data
governance or per-intent routing benchmark yet. Do not adopt multi-agent,
multimodal or routing without that evidence.

- Supervisor / Plan-Execute only if Phase 7-9 eval shows a single
  ChatModelAgent/Workflow cannot decompose the task; set step/token/time/sub-agent
  budgets.
- Multimodal search only after a multimodal model, image governance and a
  measurable user scenario exist.
- Model routing/reasoning models only after per-intent quality/latency/cost
  measurements show a benefit; retain a deterministic fallback and breaker.

## Roadmap decisions

Accepted into the roadmap: Phase 5 recovery/identity/eval gate, Phase 6
leader/Job rebuild + quality metrics + eval + rollback runbook, Phase 7 cart
draft, Phase 8 preference profile, Phase 9 scheduled ops brief.

Not adopted now: reintroducing an `AI_ADK_HITL` flag (ADK is already the only
chat runtime); free-running multi-agent; multimodal; routing before measurement;
moving the transaction core to an ORM.

Every phase keeps the invariants: AI optional; read-only tools by default;
writes are draft -> human confirmation -> original business RPC; identity only
from server context; model/embedding keys only from Secrets.
