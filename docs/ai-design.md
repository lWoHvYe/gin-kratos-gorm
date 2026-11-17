# AI assistant design

Status: Phase 0 implementation. This document is the contract for the `ai`
service; business services stay unchanged unless a tool needs a new read API.

## Goals and non-goals

Goal: a conversational assistant that can answer shopping questions and explain
a user's orders using **live business data**, without moving AI into the
transaction path.

Non-goals for Phase 0:

- no writes (no ordering, cancelling, refunding, cart mutation);
- no vector database or embeddings; catalog search uses the existing
  `CatalogService.ListProducts` filters;
- no review summarisation (the schema has no reviews yet);
- no admin analytics (the admin API has lists, not aggregates);
- no Python runtime.

The assistant is an optional surface: `AI_ENABLED=false`, a provider error, a
rate limit or a timeout must leave browsing, checkout and payment untouched.

## Architecture

```
Storefront
   │ POST /api/v1/ai/chat (SSE)
   ▼
Gateway BFF ──gRPC stream──▶ ai service (Kratos, Eino ReAct agent)
   │                              │
   │                              ├── DeepSeek ChatModel (eino-ext)
   │                              │      or mock provider (AI_PROVIDER=mock)
   │                              ├── tools: catalog.search_products,
   │                              │          catalog.get_product
   │                              └── Redis: session history, rate limit
   │
   └── existing gRPC clients ──▶ catalog / order / ... (unchanged)
```

The AI service has no MySQL access. Tools call the owning service with the
caller's context; the service is only reachable from the BFFs through the mesh
AuthorizationPolicy.

## API

Internal gRPC (`AIService.Chat`, server-streaming):

```proto
message AIChatRequest {
  string session_id = 1;   // client-generated or server-assigned
  string message = 2;      // trimmed, max 2000 chars
  int64 user_id = 3;       // injected by the BFF from the JWT
}
message AIChatEvent {
  string type = 1;         // delta | tool | done | error
  string delta = 2;        // text increment for "delta"
  string tool = 3;         // tool name for "tool"
  string error = 4;        // safe message for "error"
}
```

Public BFF: `POST /api/v1/ai/chat` with `Accept: text/event-stream`. Events use
the same `type` values. The BFF derives `user_id` from the access token and
ignores any body field claiming an identity.

SSE is exempt from the normal 15s BFF budget: the handler runs with a 120s
context and the Kratos HTTP server timeout is raised accordingly. Normal routes
keep `bff.Timeout(15s)`.

## Tools (Phase 0)

| Tool | Service call | Input | Rules |
|---|---|---|---|
| `search_products` | `CatalogService.ListProducts` | keyword, category_id, brand_id, min/max price, sort | `page_size<=10`, only `ON_SALE` results |
| `get_product` | `CatalogService.GetProduct` | product id | returns name, subtitle, description, price range, attributes; no internal ids beyond product/SKU |
| `list_my_orders` | `OrderService.ListOrders` | status, limit | `user_id` from the request context; `page_size<=10` |
| `get_my_order` | `OrderService.GetOrder` | order no | rejects an order whose `user_id` differs |
| `list_my_after_sales` | `AfterSalesService.ListAfterSales` | status, limit | `user_id` from the request context |
| `get_my_after_sale` | `AfterSalesService.GetAfterSale` | after-sale no | `GetAfterSale` has no filter, so ownership is checked in the tool |
| `search_policy` | in-code FAQ | question | curated returns/shipping/cancel/payment/seckill/address text, max 2 entries |
| `search_knowledge` | Redis Search (vector KNN) | open-ended question | top 5 passages from indexed products/policies; requires `AI_RAG_ENABLED` |
| `sales_summary` (admin) | `OrderService.GetOrderStats` | days | aggregates only: order counts and paid amount |
| `refund_summary` (admin) | `AfterSalesService.GetAfterSaleStats` | days | aggregates only: after-sale counts and refunded amount |
| `low_stock` (admin) | `InventoryService.ListLowStock` | threshold, limit | rows with available <= threshold; limit 20 |
| `draft_campaign` (admin) | none (pure validation) | name/type/threshold/discount/days/budget | returns a DRAFT campaign + rule JSON; never writes |
| `preview_campaign` (admin) | `PromotionService.EvaluateDraft` | draft values + sample item | runs the real promotion engine without saving; returns discount |

Tool results are compact JSON; the model may not set `user_id`. Writes will be
two-phase `draft -> confirm` and always call the existing business RPC, never
SQL.

## Conversation and limits

- Redis key `ai:sess:<user_id>:<session_id>`, list of user/assistant turns,
  TTL 30 minutes, max 20 messages. Only the last 20 are sent to the model.
- Rate limit: 20 requests / 10 minutes per user (Redis sliding window, fail
  closed for the AI surface only).
- Input: max 2000 characters, trimmed; empty input rejected.
- Agent: Eino ReAct, `MaxStep=6`, model timeout 30s, stream timeout 120s.
- Output: max 4096 model tokens (configurable), no markdown HTML rendering in
  the BFF (frontend renders text/plain).

## Safety

- System prompt: mall assistant only; do not invent prices, stock or policies;
  always prefer tool data; say "不知道" when data is missing.
- Retrieved/product text is data, not instructions; tool names and arguments
  are validated by the server. Unknown tools are rejected.
- No PII in logs: log user id, session id, latency, token counts, tool names —
  not message bodies.
- Model key only from `DEEPSEEK_API_KEY`; `config.Load` fails closed in
  production when `AI_PROVIDER=deepseek` and the key is missing.
- Graceful degradation: provider errors return `type=error`; the frontend shows
  "助手暂不可用" without blocking the page.

## Configuration

| Env | Default | Meaning |
|---|---|---|
| `AI_ENABLED` | `false` | feature flag; false returns Unavailable |
| `AI_PROVIDER` | `mock` | `mock` or `deepseek` |
| `DEEPSEEK_API_KEY` | empty | required for `deepseek` in production |
| `DEEPSEEK_BASE_URL` | `https://api.deepseek.com` | OpenAI-compatible endpoint |
| `AI_MODEL` | `deepseek-chat` | model id |
| `AI_MAX_TOKENS` | `1024` | completion cap |
| `AI_TIMEOUT` | `30s` | per model call |
| `AI_RATE_LIMIT` | `20/10m` | per-user request quota |
| `AI_DAILY_TOKEN_BUDGET` | `0` (disabled) | prompt+completion tokens per user per UTC day |
| `AI_GLOBAL_DAILY_TOKEN_BUDGET` | `0` (disabled) | daily tokens across all users |
| `AI_CIRCUIT_FAILURES` / `AI_CIRCUIT_COOLDOWN` | `5` / `60s` | open the provider circuit after consecutive failures |
| `AI_PRICE_*_CENTS_PER_1K` | `0` | optional list prices for cost metrics, not billing |

`AI_PROVIDER=mock` is deterministic: it calls `search_products` and then answers
from the tool result, so local development and CI need no API key.

## Observability

Prometheus (ai service; tokens/tool calls come from Eino callbacks):

- `mall_ai_requests_total{result}`
- `mall_ai_request_duration_seconds{result}`
- `mall_ai_tool_calls_total{tool,result}`
- `mall_ai_tokens_total{kind}`
- `mall_ai_rate_limited_total`
- `mall_ai_budget_exceeded_total{scope}`
- `mall_ai_cost_cents_total{kind}` (when prices are configured)
- `mall_ai_circuit_open`
- `mall_ai_rag_updates_total{result}`

OTel: span per request and per tool call; LLM adapter spans come from Eino
callbacks (Phase 0.1).

## Deployment

- `cmd/ai` service, gRPC 9011, health 9111; image `mall/ai:dev`.
- Compose: `ai` depends on Redis and catalog; default `AI_ENABLED=true`,
  `AI_PROVIDER=mock`.
- K8s: deployment/service/PDB via the existing chart values; AuthorizationPolicy
  allows only `gateway` and `admin` service accounts to call the AI service
  port; NetworkPolicy mirrors that.
- Secret: `DEEPSEEK_API_KEY` in the environment's secret store; never in values.

## Phase plan

Phase 0 (this change): service skeleton, Eino ReAct + two catalog tools,
DeepSeek/mock providers, Redis session + rate limit, gRPC stream, gateway SSE,
metrics, config, compose/K8s wiring, unit tests.

Phase 1 (done): order/after-sales read tools scoped by the request context;
policy FAQ answered from a curated in-code document (no embeddings); assistant
widget on the storefront (`components/AiAssistant.vue`, SSE parser in
`utils/sse.ts`).

Phase 3 (Redis vector search): `AI_RAG_ENABLED=true` seeds policy entries and
on-sale products into Redis Search at startup, then a durable NATS consumer on
`shop.catalog.product.changed.v1` upserts/deletes product documents as the
catalog changes. `search_knowledge` embeds the question and runs a KNN query
(`FT.SEARCH ... =>[KNN k @embedding $vec]`). If NATS is unavailable the startup
snapshot remains in use and the consumer logs a warning.

Embeddings are pluggable:

- `EMBEDDING_PROVIDER=hash` is a deterministic token-hash fallback (dev/CI). It
  is lexical, not semantic; treat RAG quality as unproven until a real model is
  configured.
- `EMBEDDING_PROVIDER=openai` calls any OpenAI-compatible `/embeddings` endpoint
  (BGE, Qwen, OpenAI). Set `EMBEDDING_BASE_URL`, `EMBEDDING_MODEL`,
  `EMBEDDING_API_KEY`, `EMBEDDING_DIM`. DeepSeek's chat API does not provide
  embeddings, so this is a separate provider even when chat uses DeepSeek. The
  dev cluster uses Qwen `qwen3.7-text-embedding` (1024 dims).
  Docs take effect for new/restarted processes.
- See `deploy/k8s/charts/mall/values-deepseek.example.yaml` for the real-model
  overlay; keys live in `mall-ai-secrets`, never in values.

Redis is the store/searcher, not the embedding generator. `pgvector` remains an
alternative if a second database is acceptable, but it was not added.

Phase 2: admin read-only copilot (done for the analytics half): aggregate RPCs
`GetOrderStats`, `GetAfterSaleStats`, `ListLowStock` and the admin SSE endpoint
`POST /api/admin/v1/ai/chat` with an admin-only agent. Phase 2B (done): `draft_campaign` builds a validated DRAFT campaign/rule,
`preview_campaign` runs the pricing engine through
`PromotionService.EvaluateDraft` without persisting, and
`POST /api/admin/v1/promotions/apply-draft` saves both only when an admin sends
`confirm=true`. The AI never calls the write endpoint. Draft campaigns use the
valid `PRIORITY` stack mode (a bug found during the live save test). The admin
SPA exposes this flow at `/promotions/draft` (test → confirm → save DRAFT).

Phase 3: optional RAG. Add embeddings + Qdrant only if evaluation shows the
MySQL retrieval is insufficient; index catalog changes from NATS/outbox
asynchronously.

## Current maturity (2026-11)

The dev cluster now runs `deepseek-flash` for chat and `qwen3.7-text-embedding`
(1024 dimensions) through an OpenAI-compatible endpoint in Aliyun MaaS; Redis
Search indexes 66 product/policy documents. Live checks produced grounded
answers from the seeded catalog and real order/refund aggregates.

Still demo-grade:

- No automated real-model evaluation set or quality metrics; `TestGoldenAssistant`
  uses the mock model and fake services. A live prompt set is still manual.
- The model is `deepseek-flash` with a 12-step ReAct budget; latency/token cost
  per tool path have not been measured in aggregate.
- Customer-facing tools are read-only. Campaign drafts can be previewed and
  saved through the admin-only apply endpoint after explicit confirmation; the
  assistant itself has no write tool.
- Request quota, per-user/global daily token budgets and a provider circuit
  breaker are implemented; prices are optional and cost is not billed. There is
  no semantic cache or per-tool budget yet.
- SSE emits `delta`/`tool`/`done`/`error`; tool events come from Eino callbacks.
  Client cancellation is detected only when a write fails.
- Sessions store user text in Redis for 30 minutes in plaintext.
- `admin=true` is trusted from the admin BFF under the mesh trust ADR.
- No feedback capture, evaluation dashboard, or alerts on tool/provider errors.
- The k3s NetworkPolicy workaround allows AI pods all public egress (port-
  qualified ipBlock rules did not take effect); production needs an egress
  gateway with domain allow-listing.
- RAG seeds on startup and updates from catalog events; if the consumer is
  down, the index can lag until the next pod restart.

Hardening order: real-model evaluation harness, token/cost budgets, tool events
and cancellation, feedback capture, event-driven indexing, then Phase 2B writes.

## Acceptance for Phase 2A

1. An admin token receives sales/refund/low-stock aggregates and no user-level
   rows; a normal user gets 403 on `/api/admin/v1/ai/chat`.
2. The admin route is documented in `docs/openapi/admin.yaml`; the gateway AI
   route is documented too.
3. Aggregate RPCs are read-only and bounded (window <= 366 days, low-stock
   limit <= 50).

## Acceptance for Phase 1

1. Asking "我的订单" streams the caller's orders; another user's order number is
   rejected by the tool.
2. "退货政策" is answered from the curated FAQ and cites the entry title.
3. The storefront renders the assistant entry; an unauthenticated call returns
   401; a rate-limited call returns 429; message bodies never appear in logs.

## Acceptance for Phase 0

1. `make verify` passes; AI unit tests use the mock provider and need no
   network or Redis.
2. With `AI_PROVIDER=mock`, a local `ai` + Redis + catalog (or a fake catalog)
   streams `delta` events and a final `done`.
3. `AI_ENABLED=false` does not register `/api/v1/ai/chat` (the storefront hides
   the entry); the ai service itself returns Unavailable if called anyway.
4. No prompt, key or message body appears in logs.
5. A 429 is returned after the per-user quota; other users are unaffected.
