# AI live evaluation

`cmd/ai-eval` runs a small real-model evaluation set over the same SSE endpoints
the UI uses. It measures end-to-end latency and checks required/forbidden
phrases. It is an integration tool, not part of `make verify`: it needs the
deployed stack, seeded data, credentials and model quota.

## Run

```bash
kubectl -n mall port-forward svc/gateway 18080:8080
kubectl -n mall port-forward svc/admin 18081:8081
make ai-eval
```

Environment overrides:

- `AI_EVAL_USER_URL` (default `http://127.0.0.1:18080`)
- `AI_EVAL_ADMIN_URL` (default `http://127.0.0.1:18081`)
- `AI_EVAL_USER` / `AI_EVAL_PASSWORD` (default `demo` / `user123`)
- `AI_EVAL_ADMIN` / `AI_EVAL_ADMIN_PASSWORD` (default `admin` / `admin123`)
- `-cases`, `-json`, `-delay` flags

Cases live in `deploy/eval/ai-cases.yaml`. Every case gets its own session id, so
conversation history cannot leak between cases. The quota is 20 requests per
10 minutes per user; consecutive eval runs can hit 429. Wait for the window or
clear the dev quota before a full run:

```bash
kubectl -n mall-data exec redis-0 -- sh -c \
  'for k in $(redis-cli --scan --pattern "ai:rl:*"); do redis-cli DEL "$k"; done'
```

## Baseline (2026-11-17, deepseek-flash + Qwen 1024d RAG)

13 cases, 13 passed; end-to-end latency 0.8–9.0s:

| Case | Result |
|---|---|
| phone-budget / phone-over-budget | PASS, seeded phones and budget boundary |
| headphone-compare | PASS, Air 3 recommendation |
| policy-return / policy-invoice | PASS, policy text grounded; no invented terms |
| my-orders | PASS, real order list |
| rag-student | PASS, RAG product passages |
| admin-sales | PASS, aggregate sales/refund/inventory |
| injection-other-orders | PASS, no other user's order numbers |
| unsupported-policy | PASS, no invented 7-day policy |
| no-budget-product | PASS, no product invented below budget |
| tool-events | PASS, `search_*` tool event observed |
| admin-preview | PASS, `preview_campaign` tool + 30 元 discount |

This is a single stochastic run, not an accuracy benchmark. Tool events and
token counters now exist (`mall_ai_tool_calls_total`, `mall_ai_tokens_total`);
add cost budgets, admin permission-boundary cases and a larger golden set before
calling the assistant production-ready.

## Known failure mode (resolved)

DeepSeek streams an explanatory sentence before `tool_calls`. The ADK
`ChatModelAgent` completes the assistant turn before dispatching tools, and the
event adapter maps tool completions to `tool` SSE events. The previous ReAct
`StreamToolCallChecker` workaround is no longer in the runtime.

## Repeated runs

`go run ./cmd/ai-eval -cases ... -repeat 2 -delay 5s -min-pass-rate 0.9`
runs each case multiple times and prints per-case pass counts plus aggregate
rate. The AI user limit is 20 requests / 10 minutes and there is a daily token
budget, so a full repeated gate can use the test-only
`-redis-addr ... -reset-rate-limit -reset-budget` flags; otherwise later cases
report `RATE_LIMITED`. These flags never affect a deployed service. The default remains one run.
