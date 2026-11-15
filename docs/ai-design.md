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

Tool results are compact JSON; the model may not set `user_id`. Future tools
(order, aftersales, admin reads) follow the same pattern: read-only, scoped,
bounded. Writes will be two-phase `draft -> confirm` and always call the
existing business RPC, never SQL.

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
| `AI_RATE_LIMIT` | `20/10m` | per-user quota |

`AI_PROVIDER=mock` is deterministic: it calls `search_products` and then answers
from the tool result, so local development and CI need no API key.

## Observability

Prometheus (ai service):

- `ai_requests_total{result}`
- `ai_request_duration_seconds`
- `ai_tool_calls_total{tool,result}`
- `ai_tokens_total{kind}`
- `ai_rate_limited_total`

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

Phase 1: order/aftersales read tools; policy FAQ answered from a static document
(no embeddings); assistant widget on the storefront.

Phase 2: admin read-only copilot after adding aggregate read APIs; campaign
draft with `EvaluatePromotions` preview and explicit confirmation.

Phase 3: optional RAG. Add embeddings + Qdrant only if evaluation shows the
MySQL retrieval is insufficient; index catalog changes from NATS/outbox
asynchronously.

## Acceptance for Phase 0

1. `make verify` passes; AI unit tests use the mock provider and need no
   network or Redis.
2. With `AI_PROVIDER=mock`, a local `ai` + Redis + catalog (or a fake catalog)
   streams `delta` events and a final `done`.
3. `AI_ENABLED=false` does not register `/api/v1/ai/chat` (the storefront hides
   the entry); the ai service itself returns Unavailable if called anyway.
4. No prompt, key or message body appears in logs.
5. A 429 is returned after the per-user quota; other users are unaffected.
