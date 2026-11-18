# Architecture review tracking

Source: external review received 2026-11-18. Status is updated as items land.
`done` means code + verification; `partial` means a safe subset shipped and the
remaining risk is documented.

## P1

| # | Item | Status | Notes |
|---|---|---|---|
| 1 | Seckill TCC cancel bucket/mapping compensation | partial | HELD releases the bucket; CONFIRMED now reverses into the open bucket or normal on_hand when closed. Missing DB-level race test. |
| 2 | Late release after activity RELEASED writes available | done | `ReleaseSeckillHold` and `releaseSeckillMapping` now return held units to normal `reserved` when the bucket is closed. |
| 3 | Qualification vs order release double-credit | partial | `ReleaseSeckillHold` skips when an order mapping exists; expiry skips qualifications with `order_no`. Needs DB-level race tests. |
| 4 | MarkPaid/cancel race leaves CONFIRMED stock | done | cancel-after-confirm calls inventory `TccCancelConfirm` (non-seckill). Race injection test still desirable. |
| 5 | Coupon lock lost on crash | done | `locked_at`, TTL sweeper, pricing/0008 migration; live sweep verified. |

## P2

| # | Item | Status |
|---|---|---|
| 6 | Outbox dead-letter archive | done | terminal dead letters copied to outbox_dead_letters before prune; MySQL integration test passes |
| 7 | Outbox serial relay / lease timeout | done | batch 20, 4m lease (worst case 200s), backlog/oldest gauges; MySQL lease test |
| 8 | BFF 15s vs saga 30s timeout | done | checkout/place gets 35s; other routes 15s |
| 9 | quotes/inventory_ledger lifecycle | partial | quotes cleaned after 30d; inventory_ledger retention still pending a policy/archive decision |
| 10 | Catalog SQL fallback vs GORM divergence/N+1 | partial | fallback batches products/SKUs/images/attrs and matches SKU code; admin predicate alignment still to audit |
| 11 | Deep OFFSET pagination | done (page > 1000 rejected; keyset still future) |
| 12 | LIKE '%x%' cannot use index | pending |
| 13 | Per-SKU GetStock in checkout/cart | pending |
| 14 | AI frontend entry when disabled | done | public /ai/status drives the storefront entry |
| 15 | Base chart mock payment defaults | done | base mock=false; single-node overlay mock=true; payment secret required when mock off |
| 16 | P1 compensation/race tests | partial (released-bucket logic added; no DB race suite) |
| 17 | Demo seed inside production migration | pending (cannot edit applied 0001; needs forward split) |
| 18 | CORS reflects any origin when unset | done (fail closed; compose dev allowlist, chart value) |
| 19 | Admin identity/ban surface | pending |

## P3

- Alertmanager/zero-alert rules: pending.
- Single-proto regeneration noise: pending.
- Compose etcd default: pending.
- BFF skeleton duplication / 1300-line admin: pending.
- Duplicate migration scripts: pending.
- Docs drift: AI MaxStep fixed; other counts still to audit.
- Frontend lint/test placeholders: pending.
- Missing version/updated_at on some tables: pending.
- Proto reserved/deprecated: pending.
- preview_campaign fixed user_id=1: pending verification.
- Duplicate frontend types / pinia version mismatch: pending.

## AI suggestions

- Real-model eval harness: done (13 cases).
- Feedback capture/quality dashboard: pending.
- RAG hybrid/rerank/semantic cache: pending.
- Eino interrupt-resume HITL: pending (current flow is hand-rolled).
- Cart task assistant / proactive admin brief / long-term memory / multi-agent: pending.
