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

8 cases, 8 passed; end-to-end latency 1.8–4.9s:

| Case | Result |
|---|---|
| phone-budget / phone-over-budget | PASS, seeded phones and budget boundary |
| headphone-compare | PASS, Air 3 recommendation |
| policy-return / policy-invoice | PASS, policy text grounded; no invented terms |
| my-orders | PASS, real order list |
| rag-student | PASS, RAG product passages |
| admin-sales | PASS, aggregate sales/refund/inventory |

This is a single stochastic run, not an accuracy benchmark. Add cases for
groundedness (numbers must come from tools), refusal of unsupported policy,
prompt injection attempts and admin permission boundaries before calling the
assistant production-ready.

## Known failure mode (fixed)

DeepSeek streams an explanatory sentence before `tool_calls`. Eino's default
`StreamToolCallChecker` stops at the first content chunk, so the ReAct graph
ended before using a tool. `anyChunkToolCallChecker` scans the branch's stream
copy to EOF or the first tool call; the user-visible copy is unaffected.
